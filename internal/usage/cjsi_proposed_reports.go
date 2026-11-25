package usage

import (
	"context"
	"database/sql"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// ProposedReportGroup is copied directly from the validated Build input.
// Receipts is required even when PrepareCJSIOTel found no ownership change.
type ProposedReportGroup struct {
	Usage       CJSIUsageCandidate
	Draft       CJSIUsageDraft
	OTel        *store.CJSIOTelChange
	Receipts    store.CJSIOTelReceiptStore
	RestoreOTel bool
}

type ReportProposal struct {
	Ticket, Attempt int64
	FrozenDiff      [32]byte
	Groups          []ProposedReportGroup
}

type proposedIntervals []ledger.Member

func (m proposedIntervals) intervalMembers(_ context.Context, _ *sql.Tx, session string) ([]ledger.Member, error) {
	if session == "" {
		return m, nil
	}
	out := []ledger.Member{}
	for _, v := range m {
		if v.Key.Thread == session {
			out = append(out, v)
		}
	}
	return out, nil
}

// WithProposedReports selects the unchanged parent groups and replaces whole
// affected groups before any report/window filtering. It also returns the
// exact immutable dirty-range plan for the coordinator's final transaction.
func WithProposedReports(ctx context.Context, tx *sql.Tx, s ReportSelection, p ReportProposal, fn func(*PublishedReports) error) (ReportIntervalImpact, error) {
	var zero ReportIntervalImpact
	parent, err := s.parent(ctx, tx)
	if err != nil {
		return zero, err
	}
	if p.Ticket <= s.Ticket || p.Ticket != s.Ticket+1 || p.Attempt <= 0 || p.FrozenDiff == ([32]byte{}) || p.Groups == nil {
		return zero, ledger.ErrProposalMismatch
	}
	if err := verifyUsageStagingSchema(ctx, tx); err != nil {
		return zero, err
	}
	err = requireUsageRow(tx.QueryRowContext(ctx, usageLiveAttemptSQL+`
	AND a.core_epoch=? AND a.base_ticket=? AND a.ticket=? AND a.attempt=? AND a.diff_digest=?
	AND a.state IN ('draft','sealed') AND length(a.session_seal)=32
	AND (SELECT count(*) FROM main.cjsi_usage_groups WHERE ticket=a.ticket AND attempt=a.attempt)=?`,
		s.CoreEpoch, s.Ticket, p.Ticket, p.Attempt, p.FrozenDiff[:], len(p.Groups)))
	if err != nil {
		return zero, err
	}

	members, err := parent.intervalMembers(ctx, tx, "")
	if err != nil {
		return zero, err
	}
	next := proposedIntervals{}
	before, after := []intervalImpactImage{}, []intervalImpactImage{}
	image := func(m ledger.Member) intervalImpactImage {
		return intervalImpactImage{owner: SessionKey{Client: "codex", SessionID: m.Key.Thread}, member: m}
	}
	selected := make(map[string]bool, len(s.Groups))
	for _, group := range s.Groups {
		selected[group.Thread] = true
	}
	changed := make(map[string]bool, len(p.Groups))
	facts := newProposedFacts(tx)
	for _, group := range p.Groups {
		d, b := group.Draft, group.Draft.Stamp.Basis
		if changed[d.Thread] || group.Receipts == nil || b.CoreEpoch != s.CoreEpoch || b.Ticket != p.Ticket || b.Attempt != p.Attempt || b.BaseTicket != s.Ticket {
			return zero, ledger.ErrProposalMismatch
		}
		changed[d.Thread] = true
		// An affected thread that was previously selected must contribute its
		// old intervals to the invalidation plan, even if it now has no members.
		if !selected[d.Thread] {
			if err := requireUsageRow(tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM main.cjsi_usage_selection WHERE thread=?)`, d.Thread)); err != nil {
				return zero, err
			}
		}
		if err := group.Usage.VerifyStaged(ctx, tx, d); err != nil {
			return zero, err
		}
		v, err := group.Usage.Proposal().Eligibility(d.Stamp, d.Thread)
		if err != nil {
			return zero, err
		}
		for _, m := range v.Members {
			if m.Key.Kind == ledger.TokenInterval {
				after = append(after, image(m))
				next = append(next, m)
			}
		}
		if err := facts.applyGroup(ctx, group, v); err != nil {
			return zero, err
		}
	}
	for _, m := range members {
		if changed[m.Key.Thread] {
			before = append(before, image(m))
		} else {
			next = append(next, m)
		}
	}
	plan, err := planIntervalImpact(before, after)
	if err != nil {
		return zero, err
	}
	impact := ReportIntervalImpact{s.CoreEpoch, p.Ticket, p.Attempt, s.Ticket, p.FrozenDiff, plan}
	q, err := facts.reader()
	if err != nil {
		return zero, err
	}
	if err = borrowReports(ctx, tx, q, s, next, &impact, fn); err != nil {
		return zero, err
	}
	return impact, nil
}

// ReportIntervalImpact is detached from SQL. SelectedRanges supplies the
// original plan to the existing store writer inside the matching opening CAS.
type ReportIntervalImpact struct {
	epoch, ticket, attempt, parent int64
	diff                           [32]byte
	plan                           intervalImpactPlan
}

func (i ReportIntervalImpact) SelectedRanges(ctx context.Context, tx *sql.Tx) ([]store.IntervalDirtyRange, error) {
	if tx == nil || i.plan == nil {
		return nil, ledger.ErrProposalMismatch
	}
	err := requireUsageRow(tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_publication_head h
	JOIN main.cjsi_publication_attempts a ON a.ticket=h.ticket AND a.core_epoch=h.core_epoch
	WHERE h.singleton=1 AND h.ticket=? AND h.core_epoch=? AND h.slot_attempt=a.attempt AND h.slot_ticket=h.ticket
	AND a.attempt=? AND a.base_ticket=? AND a.diff_digest=? AND a.state='sealed'`, i.ticket, i.epoch, i.attempt, i.parent, i.diff[:]))
	if err != nil {
		return nil, err
	}
	return intervalStampRanges(i.plan)
}

func (r *PublishedReports) AggregateCacheStamp(ctx context.Context, q StatsOptions, coverage [32]byte) ([32]byte, error) {
	if err := r.lease.Err(); err != nil {
		return [32]byte{}, err
	}
	return aggregateCacheStamp(r.head.coreEpoch, q, coverage, func(s intervalImpactScope) (int64, error) {
		// Bootstrap has no committed interval ticket. Do not use the public
		// positive-ticket reader or infer a clean base over stray future rows.
		if r.impact != nil && r.head.ticket == 0 {
			err := requireUsageRow(r.tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM main.cjsi_usage_interval_dirty_ranges WHERE core_epoch=?)`, r.head.coreEpoch))
			if err != nil {
				return 0, err
			}
			return r.impact.stamp(0, s, q.From, q.To)
		}
		old, err := readIntervalStamp(ctx, r.tx, r.head, s, q.From, q.To)
		if err != nil || r.impact == nil {
			return old, err
		}
		return r.impact.stamp(old, s, q.From, q.To)
	})
}

func (i *ReportIntervalImpact) stamp(old int64, s intervalImpactScope, from, to time.Time) (int64, error) {
	hit, err := i.plan.intersects(s, from, to)
	if hit {
		return i.ticket, nil
	}
	return old, err
}
