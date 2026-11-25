package usage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// Task 8 supplies the effective, unmasked groups for this query from the
// borrowed core transaction. Nil means unavailable; an explicit empty slice
// asserts a completed selection. Ticket is the admitted head, not the ticket
// at which every retained thread was last changed.
type cjsiIntervalSelection struct {
	coreEpoch, ticket int64
	groups            []cjsiIntervalGroup
}

type cjsiIntervalGroup struct {
	thread string
	stamp  ledger.ProposalStamp
}

func intervalStageError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ledger.ErrProposalMismatch
	}
	return err
}

// readSelectedIntervalMembers validates the pinned head and each selected
// group, returning all decoded TokenInterval rows (with whole-interval proved
// model when present). Disjoint non-Codex clients short-circuit before reading.
func readSelectedIntervalMembers(ctx context.Context, tx *sql.Tx, selected intervalMemberSelection, scope StatsOptions, session string) ([]ledger.Member, error) {
	if !scope.From.Before(scope.To) {
		return nil, errors.New("interval aggregation requires a nonempty window")
	}
	if scope.Client != "" && scope.Client != "codex" {
		return []ledger.Member{}, nil
	}
	return selected.intervalMembers(ctx, tx, session)
}

type intervalMemberSelection interface {
	intervalMembers(context.Context, *sql.Tx, string) ([]ledger.Member, error)
}

func (selected cjsiIntervalSelection) intervalMembers(ctx context.Context, tx *sql.Tx, session string) ([]ledger.Member, error) {
	if tx == nil || selected.groups == nil || selected.coreEpoch <= 0 || selected.ticket < 0 {
		return nil, ledger.ErrProposalMismatch
	}
	var one int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_publication_head
		WHERE singleton=1 AND core_epoch=? AND ticket=?`, selected.coreEpoch, selected.ticket).Scan(&one); err != nil {
		return nil, intervalStageError(err)
	}

	members := []ledger.Member{}
	seen := make(map[string]bool)
	for _, group := range selected.groups {
		if group.thread == "" || seen[group.thread] {
			return nil, ledger.ErrProposalMismatch
		}
		seen[group.thread] = true
		if session != "" && group.thread != session {
			continue
		}
		b := group.stamp.Basis
		if b.CoreEpoch != selected.coreEpoch || b.Ticket > selected.ticket || b.InputDigest == ([32]byte{}) || group.stamp.Diff == ([32]byte{}) {
			return nil, ledger.ErrProposalMismatch
		}
		rows, err := readIntervalGroup(ctx, tx, group)
		if err != nil {
			return nil, err
		}
		members = append(members, rows...)
	}
	return members, nil
}

// Only the interval contribution. The caller owns group/protection coverage,
// the request side, and final admission; partial here is the reducer's slice
// allocation result, not a verdict that the whole ledger is complete.
func readScopedIntervals(ctx context.Context, tx *sql.Tx, selected intervalMemberSelection, scope StatsOptions, session string) (scopedIntervalSum, error) {
	if !scope.From.Before(scope.To) || (scope.Client != "" && scope.Client != "codex") {
		return reduceScopedIntervals(nil, scope, session)
	}
	members, err := readSelectedIntervalMembers(ctx, tx, selected, scope, session)
	if err != nil {
		return scopedIntervalSum{}, err
	}
	return reduceScopedIntervals(members, scope, session)
}

func readIntervalGroup(ctx context.Context, tx *sql.Tx, group cjsiIntervalGroup) ([]ledger.Member, error) {
	b := group.stamp.Basis
	// A selected, authenticated group with no intervals is a valid result.
	// Check it separately from its children. Reject an interval-kind member
	// with no endpoints even if a different kind reuses the same member ID.
	var one int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_usage_selection s
		JOIN main.cjsi_usage_groups g USING(ticket,attempt,thread)
		JOIN main.cjsi_publication_attempts a USING(ticket,attempt)
		WHERE s.thread=? AND s.ticket=? AND s.attempt=? AND g.input_digest=? AND g.diff_digest=?
		AND a.core_epoch=? AND a.base_ticket=? AND a.input_digest=g.input_digest AND a.state='opened'
		AND NOT EXISTS (
			SELECT 1 FROM main.cjsi_usage_members m
			LEFT JOIN main.cjsi_usage_intervals i USING(ticket,attempt,thread,kind,member_id)
			WHERE m.ticket=s.ticket AND m.attempt=s.attempt AND m.thread=s.thread AND m.kind=?
			AND i.member_id IS NULL)`, group.thread, b.Ticket, b.Attempt, b.InputDigest[:], group.stamp.Diff[:],
		b.CoreEpoch, b.BaseTicket, ledger.TokenInterval).Scan(&one)
	if err != nil {
		return nil, intervalStageError(err)
	}
	// The converse orphan (interval without its exact member) fails Scan on
	// the required counts. Never use a time predicate that hides NULL bounds.
	rows, err := tx.QueryContext(ctx, `SELECT i.member_id, m.model, i.counter_epoch, i.lower_at, i.upper_at,
		m.input_tokens,m.output_tokens,m.cached,m.cache_write,m.reasoning,
		i.lower_input,i.lower_output,i.lower_cached,i.lower_write,i.lower_reasoning,
		i.upper_input,i.upper_output,i.upper_cached,i.upper_write,i.upper_reasoning
		FROM main.cjsi_usage_intervals i
		LEFT JOIN main.cjsi_usage_members m USING(ticket,attempt,thread,kind,member_id)
		WHERE i.ticket=? AND i.attempt=? AND i.thread=? AND i.kind=?`,
		b.Ticket, b.Attempt, group.thread, ledger.TokenInterval)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []ledger.Member
	for rows.Next() {
		member, err := decodeIntervalRow(rows, group.thread)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

type intervalSQLCounts struct {
	input, output            int64
	cached, write, reasoning sql.NullInt64
}

func (n intervalSQLCounts) counts() (ledger.Counts, bool) {
	v := ledger.Counts{Input: n.input, Output: n.output, Cached: n.cached.Int64, Write: n.write.Int64, Reasoning: n.reasoning.Int64}
	if n.cached.Valid {
		v.Known |= ledger.CachedKnown
	}
	if n.write.Valid {
		v.Known |= ledger.WriteKnown
	}
	if n.reasoning.Valid {
		v.Known |= ledger.ReasoningKnown
	}
	return v, validIntervalCounts(v)
}

func decodeIntervalRow(rows *sql.Rows, thread string) (ledger.Member, error) {
	m := ledger.Member{Key: ledger.Key{Kind: ledger.TokenInterval, Thread: thread}}
	var epoch []byte
	var start, end string
	var n, lo, hi intervalSQLCounts
	err := rows.Scan(&m.Key.ID, &m.Model, &epoch, &start, &end,
		&n.input, &n.output, &n.cached, &n.write, &n.reasoning,
		&lo.input, &lo.output, &lo.cached, &lo.write, &lo.reasoning,
		&hi.input, &hi.output, &hi.cached, &hi.write, &hi.reasoning)
	if err != nil {
		return ledger.Member{}, err
	}
	r := &m.Interval
	r.LowerAt, err = time.Parse(time.RFC3339Nano, start)
	if err != nil {
		return ledger.Member{}, errors.New("invalid lower interval timestamp")
	}
	r.UpperAt, err = time.Parse(time.RFC3339Nano, end)
	if err != nil || r.LowerAt.IsZero() || r.UpperAt.IsZero() || r.UpperAt.Before(r.LowerAt) {
		return ledger.Member{}, errors.New("invalid interval time bounds")
	}
	if m.Key.ID == "" || len(epoch) != len(r.Epoch) || [32]byte(epoch) == ([32]byte{}) {
		return ledger.Member{}, errors.New("invalid stored interval identity")
	}
	copy(r.Epoch[:], epoch)
	var residualOK, lowerOK, upperOK bool
	m.Tokens, residualOK = n.counts()
	r.Lower, lowerOK = lo.counts()
	r.Upper, upperOK = hi.counts()
	if !residualOK || !lowerOK || !upperOK || !residualFitsRange(m.Tokens, *r) {
		return ledger.Member{}, errors.New("inconsistent stored interval counts")
	}
	return m, nil
}
