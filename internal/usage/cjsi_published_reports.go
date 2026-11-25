package usage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// ReportSelection is the independently admitted, unmasked group selection.
// A retained group may have been last selected before Ticket.
type ReportSelection struct {
	CoreEpoch, Ticket int64
	Groups            []ReportGroup
	Clock             time.Time
}

type ReportGroup struct {
	Thread string
	Stamp  ledger.ProposalStamp
}

// PublishedReports is borrowed only inside WithPublishedReports or WithProposedReports.
// It neither owns the transaction nor obtains another connection.
type PublishedReports struct {
	lease    context.Context
	tx       *sql.Tx
	head     intervalStampHead
	selected intervalMemberSelection
	clock    time.Time
	read     readScopedRequests
	facts    reportQueryer
	price    *readPriceResolver
	impact   *ReportIntervalImpact
}

func (s ReportSelection) parent(ctx context.Context, tx *sql.Tx) (cjsiIntervalSelection, error) {
	if tx == nil || s.Clock.IsZero() || s.CoreEpoch <= 0 || s.Ticket < 0 || s.Groups == nil {
		return cjsiIntervalSelection{}, ledger.ErrProposalMismatch
	}
	if err := requireUsageRow(tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_publication_head WHERE singleton=1 AND core_epoch=? AND ticket=?`, s.CoreEpoch, s.Ticket)); err != nil {
		return cjsiIntervalSelection{}, err
	}
	out := cjsiIntervalSelection{s.CoreEpoch, s.Ticket, make([]cjsiIntervalGroup, 0, len(s.Groups))}
	seen := map[string]bool{}
	for _, g := range s.Groups {
		if g.Thread == "" || seen[g.Thread] || g.Stamp.Basis.CoreEpoch != s.CoreEpoch || g.Stamp.Basis.Ticket > s.Ticket {
			return cjsiIntervalSelection{}, ledger.ErrProposalMismatch
		}
		seen[g.Thread] = true
		out.groups = append(out.groups, cjsiIntervalGroup{g.Thread, g.Stamp})
	}
	return out, nil
}

func WithPublishedReports(ctx context.Context, tx *sql.Tx, s ReportSelection, fn func(*PublishedReports) error) error {
	parent, err := s.parent(ctx, tx)
	if err != nil {
		return err
	}
	return borrowReports(ctx, tx, tx, s, parent, nil, fn)
}

func borrowReports(ctx context.Context, tx *sql.Tx, facts reportQueryer, s ReportSelection, selected intervalMemberSelection, impact *ReportIntervalImpact, fn func(*PublishedReports) error) error {
	if fn == nil {
		return errors.New("missing report callback")
	}
	price, err := loadReportPriceResolver(ctx, tx, s.Clock)
	if err != nil {
		return err
	}
	lease, end := context.WithCancel(ctx)
	defer end()
	r := &PublishedReports{lease: lease, tx: tx, clock: s.Clock, head: intervalStampHead{s.CoreEpoch, s.Ticket}, selected: selected, impact: impact, facts: facts, price: &price}
	r.read = func(ctx context.Context, _ *sql.Tx, w scopedRequestWindow) ([]storedEvent, *readPriceResolver, error) {
		events, err := readReportEvents(ctx, facts, w, "", false)
		return events, &price, err
	}
	return fn(r)
}

func (r *PublishedReports) toolReader(q StatsOptions) readScopedToolCalls {
	if r.facts == nil {
		return nil
	}
	return func(ctx context.Context, _ *sql.Tx, w scopedRequestWindow) ([]scopedToolCall, error) {
		query := `SELECT t.client,t.session_id,t.model,t.tool_name,t.started_at,t.status,t.duration_ms,us.first_at FROM usage_tool_calls t LEFT JOIN usage_sessions us ON us.client=t.client AND us.session_id=t.session_id WHERE t.started_at>=? AND t.started_at<?`
		args := []any{w.from.UTC().Format(time.RFC3339Nano), w.to.UTC().Format(time.RFC3339Nano)}
		if w.client != "" {
			query += " AND t.client=?"
			args = append(args, w.client)
		}
		if q.Model != "" {
			query += " AND t.model=?"
			args = append(args, q.Model)
		}
		rows, err := r.facts.QueryContext(ctx, query+" ORDER BY t.started_at,t.activity_key", args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []scopedToolCall{}
		for rows.Next() {
			var t scopedToolCall
			var start sql.NullString
			if err := rows.Scan(&t.Client, &t.SessionID, &t.Model, &t.ToolName, &t.StartedAt, &t.Status, &t.DurationMS, &start); err != nil {
				return nil, err
			}
			if t.Model == "" {
				continue
			}
			if q.Provider != "" {
				t.Provider, err = runtimeProviderAt(r.price.timeline, t.Client, start)
				if err != nil {
					return nil, err
				}
			}
			out = append(out, t)
		}
		return out, rows.Err()
	}
}

// ReportSummary keeps allocation coverage and known subtotals out of the public JSON contract.
type ReportSummary struct {
	Summary         Summary
	Totals          PresentationTotals
	ShareBase       int64
	IntervalCounts  ledger.Counts
	IntervalMissing ledger.DetailMask
	Partial         bool
}

func (r *PublishedReports) Summary(ctx context.Context, q StatsOptions, session string) (ReportSummary, error) {
	if err := r.lease.Err(); err != nil {
		return ReportSummary{}, err
	}
	j, err := readScopedUsageSummary(ctx, r.tx, r.selected, q, session, r.read)
	return ReportSummary{j.summary, j.totals, j.shareBase, j.intervalCounts, j.intervalMissing, j.partial}, err
}

func (r *PublishedReports) Stats(ctx context.Context, q StatsOptions) (StatsReport, bool, error) {
	if err := r.lease.Err(); err != nil {
		return StatsReport{}, false, err
	}
	return readScopedStats(ctx, r.tx, r.selected, q, r.read, r.toolReader(q))
}

func (r *PublishedReports) Presentation(ctx context.Context, loc *time.Location, todayOnly bool) (PresentationReport, error) {
	if err := r.lease.Err(); err != nil {
		return PresentationReport{}, err
	}
	return readScopedPresentation(ctx, r.tx, r.selected, r.clock, loc, todayOnly, r.read)
}

func (r *PublishedReports) SessionUsage(ctx context.Context, client, id string) (SessionSummary, error) {
	if err := r.lease.Err(); err != nil {
		return SessionSummary{}, err
	}
	return readScopedSessionUsage(ctx, r.tx, r.selected, client, id, r.read)
}

func (r *PublishedReports) SignalsBatch(ctx context.Context, opts []SignalOptions) ([]SignalReport, error) {
	if err := r.lease.Err(); err != nil {
		return nil, err
	}
	return readScopedSignalsBatch(ctx, r.tx, opts, r.read)
}

func (r *PublishedReports) SessionSignals(ctx context.Context, client, id string) (SessionSignalSummary, bool, error) {
	if err := r.lease.Err(); err != nil {
		return SessionSignalSummary{}, false, err
	}
	return readScopedSessionSignals(ctx, r.tx, client, id, r.read)
}

func (r *PublishedReports) Earliest(ctx context.Context) (*time.Time, error) {
	if err := r.lease.Err(); err != nil {
		return nil, err
	}
	return readEarliestSignalAt(ctx, r.tx)
}

func (r *PublishedReports) Publication() (epoch, ticket int64, clock time.Time) {
	ticket = r.head.ticket
	if r.impact != nil {
		ticket = r.impact.ticket
	}
	return r.head.coreEpoch, ticket, r.clock
}
