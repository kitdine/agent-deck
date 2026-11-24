package usage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func (c CJSIUsageCandidate) Thread() string { return c.thread }

// ReadCJSICommittedCall joins a stored event to an explicitly selected
// stable-response member. It is not an ownership or source discovery query.
func ReadCJSICommittedCall(ctx context.Context, q CJSIUsageDraftReader, st ledger.ProposalStamp, key ledger.Key) (ledger.CommittedCall, error) {
	var out ledger.CommittedCall
	if key.Kind != ledger.Response || key.ID == "" || key.Thread == "" {
		return out, ledger.ErrProposalMismatch
	}
	b := st.Basis
	image, err := ReadCJSIOldUsage(ctx, q, key.EventKey())
	if err != nil {
		return out, err
	}
	if image.Client != "codex" || image.EventID != key.ID || image.SessionID != key.Thread {
		return out, ledger.ErrProposalMismatch
	}
	// Follow only opened successors in this epoch up to the thread's actual
	// selection. A newer assessment of this thread must explicitly carry this
	// exact row image. A replacement, withdrawal, or missing receipt ends it.
	prior := image.Precondition()
	var cached, write, reason sql.NullInt64
	err = q.QueryRowContext(ctx, `WITH RECURSIVE
 selected AS (SELECT s.ticket,s.attempt FROM main.cjsi_usage_selection s
 JOIN main.cjsi_publication_head h ON h.singleton=1 AND h.core_epoch=:epoch AND h.ticket>=s.ticket
 JOIN main.derived_snapshot_generation gen ON gen.singleton=1 AND gen.epoch=h.core_epoch WHERE s.thread=:thread),
 lineage(ticket,attempt) AS (
 SELECT a.ticket,a.attempt FROM main.cjsi_publication_attempts a
 WHERE a.ticket=:ticket AND a.attempt=:attempt AND a.state='opened' AND a.core_epoch=:epoch AND a.base_ticket=:base
 UNION ALL
 SELECT a.ticket,a.attempt FROM lineage l JOIN main.cjsi_publication_attempts a ON a.base_ticket=l.ticket
 WHERE a.state='opened' AND a.core_epoch=:epoch AND a.ticket<=(SELECT ticket FROM selected)
 AND NOT EXISTS(SELECT 1 FROM main.cjsi_publication_attempts sibling WHERE sibling.ticket=l.ticket AND sibling.attempt<>l.attempt AND sibling.state='opened' AND sibling.core_epoch=:epoch)
 AND NOT EXISTS(SELECT 1 FROM main.cjsi_usage_groups g
   WHERE g.ticket=a.ticket AND g.attempt=a.attempt AND g.thread=:thread
   AND ((g.gaps & :held)<>0 OR NOT EXISTS(SELECT 1 FROM main.cjsi_old_preconditions p
    WHERE p.ticket=g.ticket AND p.attempt=g.attempt AND p.thread=g.thread AND p.event_key=:key AND p.row_hash=:rowhash AND p.disposition='hold'))))
 SELECT m.cached,m.cache_write,m.reasoning FROM main.cjsi_usage_groups g JOIN main.cjsi_usage_members m USING(ticket,attempt,thread)
 WHERE g.thread=:thread AND g.ticket=:ticket AND g.attempt=:attempt AND g.input_digest=:input AND g.diff_digest=:diff
 AND EXISTS(SELECT 1 FROM lineage JOIN selected USING(ticket,attempt))
 AND m.kind=:kind AND m.member_id=:id AND m.input_tokens=:ninput AND m.output_tokens=:noutput AND m.event_at=:at AND m.model=:model
 AND COALESCE(m.cached,0)=:cached AND COALESCE(m.cache_write,0)=:write`,
		sql.Named("thread", key.Thread), sql.Named("ticket", b.Ticket), sql.Named("attempt", b.Attempt), sql.Named("epoch", b.CoreEpoch), sql.Named("base", b.BaseTicket),
		sql.Named("held", ledger.UnsafeOldSide), sql.Named("key", prior.EventKey), sql.Named("rowhash", prior.RowHash[:]), sql.Named("input", b.InputDigest[:]), sql.Named("diff", st.Diff[:]),
		sql.Named("kind", key.Kind), sql.Named("id", key.ID), sql.Named("ninput", image.Input), sql.Named("noutput", image.Output), sql.Named("at", image.At), sql.Named("model", image.Model), sql.Named("cached", image.Cached), sql.Named("write", image.Write)).Scan(&cached, &write, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ledger.ErrProposalMismatch
	}
	if err != nil {
		return out, err
	}
	n := ledger.Counts{Input: image.Input, Output: image.Output, Cached: cached.Int64, Write: write.Int64, Reasoning: reason.Int64}
	if cached.Valid {
		n.Known |= ledger.CachedKnown
	}
	if write.Valid {
		n.Known |= ledger.WriteKnown
	}
	if reason.Valid {
		n.Known |= ledger.ReasoningKnown
	}
	return ledger.CommittedCall{Selection: st, Key: key, Prior: image.Precondition(), Tokens: n}, nil
}

// ApplySelectedRequests is called only by the final writer after its CAS
// selects this group and telemetry has settled its side. The caller owns
// rollback on any error, checkpoint/coverage and the other affected domains.
// Token-only members stay in the interval ledger.
func (c CJSIUsageCandidate) ApplySelectedRequests(ctx context.Context, tx *sql.Tx, stamp ledger.ProposalStamp, registered []ledger.OldRow) error {
	v, err := c.proposal.Eligibility(stamp, c.thread)
	if err != nil {
		return err
	}
	d, err := c.proposal.Changes(stamp, c.thread)
	if err != nil {
		return err
	}
	if err = requireUsageRow(tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_usage_selection s JOIN main.cjsi_usage_groups g USING(ticket,attempt,thread)
 JOIN main.cjsi_publication_head h ON h.singleton=1 JOIN main.derived_snapshot_generation gen ON gen.singleton=1 AND h.core_epoch=gen.epoch
 WHERE s.ticket=? AND s.attempt=? AND s.thread=? AND h.ticket=s.ticket AND h.core_epoch=? AND g.input_digest=? AND g.diff_digest=?`, stamp.Basis.Ticket, stamp.Basis.Attempt, c.thread, stamp.Basis.CoreEpoch, stamp.Basis.InputDigest[:], stamp.Diff[:])); err != nil {
		return err
	}
	old, err := c.readOldRows(ctx, tx, registered)
	if err != nil {
		return err
	}
	if v.Gaps&ledger.OldSideHeld != 0 {
		return nil
	}
	affected := map[string][2]string{}
	for _, r := range d.Retract {
		image, ok := old[r.EventKey]
		if !ok || image.Client != "codex" {
			return ledger.ErrProposalMismatch
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM main.usage_events WHERE event_key=?", r.EventKey); err != nil {
			return err
		}
		affected[image.SessionID] = [2]string{"codex", image.SessionID}
	}
	if len(v.Members) > 0 {
		if err = requireUsageRow(tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM main.usage_events WHERE client='codex' AND session_id=? AND source_path LIKE 'otel:%')`, c.thread)); err != nil {
			return err
		}
	}
	for _, m := range v.Members {
		if m.Key.Kind == ledger.TokenInterval {
			continue
		}
		if err = c.writeRequest(ctx, tx, m, old); err != nil {
			return err
		}
		affected[c.thread] = [2]string{"codex", c.thread}
	}
	if err = c.writeCheckpoints(ctx, tx); err != nil {
		return err
	}
	return rebuildSessions(ctx, tx, affected)
}

func (c CJSIUsageCandidate) writeRequest(ctx context.Context, tx *sql.Tx, m ledger.Member, old map[string]ledger.OldImage) error {
	if m.Key.Thread != c.thread || m.At.IsZero() || len(m.Proof) == 0 {
		return ledger.ErrProposalMismatch
	}
	key := m.Key.EventKey()
	eventID := m.Key.ID
	if m.Key.Kind == ledger.LegacyCall {
		eventID = m.TurnID
	}
	// A source range is an alias for this proved call. Use the same
	// path preference as the original event writer; all aliases support exact.
	path := ""
	var offset int64
	for _, ref := range m.Proof {
		for _, w := range c.worksets {
			if ledger.SelectionStamp(w.Selected) != ref.Source {
				continue
			}
			name := w.Selected.Path
			if path == "" || name > path {
				path = name
				offset = ref.Span.Start
			}
		}
	}
	if path == "" {
		return ledger.ErrProposalMismatch
	}
	// Only the independently selected old set can authorize modifying a row.
	_, err := ReadCJSIOldUsage(ctx, tx, key)
	if err == nil {
		prior, ok := old[key]
		if !ok || prior.Client != "codex" || prior.SessionID != c.thread || prior.EventID != eventID {
			return ledger.ErrProposalMismatch
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, upsertUsageEventSQL, key, "codex", c.thread, eventID, m.At.UTC().Format(time.RFC3339Nano), m.Model, nullableInt(m.TurnIndex), m.Tokens.Input, m.Tokens.Cached, m.Tokens.Output, 0, 0, 0, 0, m.Tokens.Write, path, offset)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM main.usage_run_bindings WHERE event_key=?", key); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE main.usage_events SET run_id=NULL WHERE event_key=? AND run_id IS NOT NULL", key); err != nil {
		return err
	}
	if m.ExactRun != 0 {
		return c.bindVerifiedRun(ctx, tx, key, m)
	}
	return err
}

func (c CJSIUsageCandidate) bindVerifiedRun(ctx context.Context, tx *sql.Tx, key string, m ledger.Member) error {
	// Recheck the entire set of aliases while holding the publication
	// transaction. A late run on a sibling or a retained pre-move path
	// disqualifies the binding before the first covering witness is used.
	type sourceRange struct {
		path       string
		start, end int64
	}
	witnesses := make(map[sourceRange]bool)
	selectedPaths := make(map[ledger.SourceStamp]string, len(c.worksets))
	for _, w := range c.worksets {
		selectedPaths[ledger.SelectionStamp(w.Selected)] = w.Selected.Path
	}
	for _, ref := range m.Proof {
		if path := selectedPaths[ref.Source]; path != "" {
			witnesses[sourceRange{path, ref.Span.Start, ref.Span.End}] = true
		}
		for _, r := range c.runs {
			if r.Source != ref.Source || r.OriginalPath == "" || r.RunID <= 0 || !r.Completed || !r.Exact || r.Bytes.Start < 0 {
				continue
			}
			start, end := max(ref.Span.Start, r.Bytes.Start), min(ref.Span.End, r.Bytes.End)
			if start < end {
				witnesses[sourceRange{r.OriginalPath, start, end}] = true
			}
		}
	}
	for witness := range witnesses {
		var conflict bool
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM main.usage_run_sources x JOIN main.usage_runs y ON y.id=x.run_id
    WHERE x.run_id<>? AND x.path=? AND y.client='codex' AND y.exact=1 AND y.ended_at IS NOT NULL
    AND x.end_offset>x.start_offset AND x.start_offset<? AND x.end_offset>?)`, m.ExactRun, witness.path, witness.end, witness.start).Scan(&conflict)
		if err != nil || conflict {
			return err
		}
	}
	for _, r := range c.runs {
		if r.RunID != m.ExactRun || !r.Completed || !r.Exact || r.EndHash == "" {
			continue
		}
		for _, ref := range m.Proof {
			if ref.Source != r.Source || r.Bytes.Start < 0 || r.Bytes.End > ref.Source.Prefix.Progress.CompleteEnd || ref.Span.Start < r.Bytes.Start || ref.Span.End > r.Bytes.End {
				continue
			}
			path := r.OriginalPath
			if path == "" {
				for _, w := range c.worksets {
					if ledger.SelectionStamp(w.Selected) == ref.Source {
						path = w.Selected.Path
						break
					}
				}
			}
			res, err := tx.ExecContext(ctx, `INSERT INTO main.usage_run_bindings(event_key,run_id)
    SELECT ?,u.id FROM main.usage_runs u JOIN main.usage_run_sources s ON s.run_id=u.id
    WHERE u.id=? AND u.client='codex' AND u.ended_at IS NOT NULL AND u.exact=1 AND u.ambiguity_reason=''
    AND s.path=? AND s.start_offset=? AND s.end_offset=? AND s.start_hash=? AND s.end_hash=?`, key, r.RunID, path, r.Bytes.Start, r.Bytes.End, r.StartHash, r.EndHash)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil || n == 1 {
				return err
			}
		}
	}
	// The old witness ceased to qualify. Normal event-time attribution applies.
	return nil
}
