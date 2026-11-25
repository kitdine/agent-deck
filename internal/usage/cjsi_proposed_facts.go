package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// Only the bounded changed keys are materialized. An absent row in a keyed
// replacement is a deletion. CASE/union and the window predicate execute on
// the caller's connection; no TEMP schema, writes or second snapshot.
type proposedFacts struct {
	tx       *sql.Tx
	events   map[string][]any
	sessions map[string][]any // Codex only; keep original UUID spelling
	files    map[string][]any
}

func newProposedFacts(tx *sql.Tx) *proposedFacts {
	return &proposedFacts{tx, map[string][]any{}, map[string][]any{}, map[string][]any{}}
}

const reportEventColumns = "event_key,client,session_id,event_id,event_at,model,turn_index,input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,cache_write_5m_tokens,cache_write_1h_tokens,cache_write_tokens,source_path,source_offset,run_id"

type projectedQuery struct {
	tx     *sql.Tx
	prefix string
	args   []any
}

func (q projectedQuery) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return q.tx.QueryContext(ctx, q.prefix+query, append(q.args[:len(q.args):len(q.args)], args...)...)
}

func (q projectedQuery) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return q.tx.QueryRowContext(ctx, q.prefix+query, append(q.args[:len(q.args):len(q.args)], args...)...)
}

// Emit integers as JSON integers (never via a float64 intermediate).
func jsonTable(table, columns, key, condition string, rows map[string][]any) (string, any, error) {
	// encoding/json replaces ill-formed UTF-8. Never silently change a
	// registered physical path, identifier or other SQLite TEXT input.
	for k, row := range rows {
		if !utf8.ValidString(k) {
			return "", nil, ledger.ErrProposalMismatch
		}
		for _, v := range row {
			if s, ok := v.(string); ok && !utf8.ValidString(s) {
				return "", nil, ledger.ErrProposalMismatch
			}
		}
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return "", nil, err
	}
	cols := strings.Split(columns, ",")
	extract := make([]string, len(cols))
	for i := range cols {
		extract[i] = fmt.Sprintf("json_extract(value,'$[%d]')", i)
	}
	changes := table + "_delta"
	return changes + " AS MATERIALIZED (SELECT key,value FROM json_each(?)), " + table + "(" + columns + ") AS (" +
		"SELECT " + columns + " FROM main." + table + " AS base WHERE NOT (" + condition + " AND base." + key + " IN (SELECT key FROM " + changes + ")) UNION ALL SELECT " +
		strings.Join(extract, ",") + " FROM " + changes + " WHERE value IS NOT NULL)", string(data), nil
}

func (f *proposedFacts) reader() (projectedQuery, error) {
	q := projectedQuery{tx: f.tx}
	var ctes []string
	for _, t := range []struct {
		name, columns, key, predicate string
		rows                          map[string][]any
	}{
		{"usage_events", reportEventColumns, "event_key", "1", f.events},
		{"usage_sessions", "client,session_id,first_at,last_at,started_at", "session_id", "base.client='codex'", f.sessions},
		{"usage_source_files", "path,session_id,session_started_at", "path", "1", f.files},
	} {
		if len(t.rows) == 0 {
			continue
		}
		sql, arg, err := jsonTable(t.name, t.columns, t.key, t.predicate, t.rows)
		if err != nil {
			return q, err
		}
		ctes = append(ctes, sql)
		q.args = append(q.args, arg)
	}
	if len(f.events) > 0 {
		// Changed events already carry the effective binding in run_id.
		ctes = append(ctes, `usage_run_bindings AS (SELECT event_key,run_id FROM main.usage_run_bindings WHERE event_key NOT IN (SELECT key FROM usage_events_delta))`)
	}
	if len(ctes) > 0 {
		q.prefix = "WITH " + strings.Join(ctes, ",") + " "
	}
	return q, nil
}

func reportNullable(n sql.NullInt64) any {
	if n.Valid {
		return n.Int64
	}
	return nil
}

func (f *proposedFacts) otel(e store.CJSIOTelReportEffects) {
	for _, r := range e.Remove.Events {
		f.events[r.Key] = nil
	}
	for _, s := range e.Remove.Sessions {
		f.sessions[s.Thread] = nil
	}
	bindings := map[string]int64{}
	for _, r := range e.Insert.Bindings {
		bindings[r.Event] = r.Run
	}
	for _, r := range e.Insert.Events {
		run := reportNullable(r.Run)
		if b, ok := bindings[r.Key]; ok {
			run = b
		}
		f.events[r.Key] = []any{r.Key, r.Client, r.Thread, r.ID, r.At, r.Model, reportNullable(r.Turn), r.Input, r.Cached, r.Output, r.CacheRead, r.CacheCreation, r.Write5m, r.Write1h, r.CacheWrite, r.Path, r.Offset, run}
	}
	for _, s := range e.Insert.Sessions {
		f.sessions[s.Thread] = []any{s.Client, s.Thread, s.First, s.Last, s.Started}
	}
}

func (f *proposedFacts) applyGroup(ctx context.Context, g ProposedReportGroup, v ledger.Eligibility) error {
	if g.RestoreOTel && g.OTel == nil {
		return store.ErrTelemetrySourceIdentityPending
	}
	var e store.CJSIOTelReportEffects
	if g.OTel != nil {
		var err error
		e, err = g.OTel.ReportEffects(ctx, f.tx, g.Receipts, g.Draft.Thread, g.Draft.Stamp)
		if err != nil {
			return err
		}
	} else {
		// A missing owner object is not evidence that there were no effects.
		// Confirm the original no-op against these same receipt inputs.
		check, err := store.PrepareCJSIOTel(ctx, f.tx, g.Draft.Thread, g.Usage.Proposal(), g.Receipts)
		if err != nil {
			return err
		}
		if check != nil {
			return store.ErrCJSIOTelPrecondition
		}
	}
	if g.RestoreOTel != e.Restoring {
		return store.ErrCJSIOTelPrecondition
	}
	if !g.RestoreOTel {
		f.otel(e)
	}
	if v.Gaps&ledger.OldSideHeld == 0 {
		if err := f.requests(ctx, g, v); err != nil {
			return err
		}
	}
	if g.RestoreOTel {
		q, err := f.reader()
		if err != nil {
			return err
		}
		if err := g.OTel.VerifyReportVacated(ctx, q); err != nil {
			return err
		}
		f.otel(e)
	}
	return nil
}

func (f *proposedFacts) requests(ctx context.Context, g ProposedReportGroup, v ledger.Eligibility) error {
	c := g.Usage
	delta, err := c.proposal.Changes(g.Draft.Stamp, c.thread)
	if err != nil {
		return err
	}
	old, err := c.readOldRows(ctx, f.tx, g.Draft.ExpectedOldRows)
	if err != nil {
		return err
	}
	for key := range old {
		if _, changed := f.events[key]; changed {
			return ledger.ErrProposalMismatch
		}
	}
	affected := map[string]bool{}
	for _, row := range delta.Retract {
		image, ok := old[row.EventKey]
		if !ok || image.Client != "codex" {
			return ledger.ErrProposalMismatch
		}
		f.events[row.EventKey] = nil
		affected[image.SessionID] = true
	}
	if len(v.Members) > 0 {
		q, err := f.reader()
		if err != nil {
			return err
		}
		if err := requireUsageRow(q.QueryRowContext(ctx, requestWithoutOTelSQL("usage_events"), c.thread)); err != nil {
			return err
		}
	}
	for _, m := range v.Members {
		if m.Key.Kind == ledger.TokenInterval {
			continue
		}
		request, err := c.requestIdentity(m)
		if err != nil {
			return err
		}
		var lookupErr error
		if row, changed := f.events[request.key]; !changed {
			_, lookupErr = ReadCJSIOldUsage(ctx, f.tx, request.key)
		} else if row == nil {
			lookupErr = sql.ErrNoRows
		}
		if err := request.checkReplacement(c.thread, old, lookupErr); err != nil {
			return err
		}
		run, err := proposedRun(ctx, f.tx, c, m)
		if err != nil {
			return err
		}
		f.events[request.key] = []any{request.key, "codex", c.thread, request.id, m.At.UTC().Format(time.RFC3339Nano), m.Model, nullableInt(m.TurnIndex), m.Tokens.Input, m.Tokens.Cached, m.Tokens.Output, 0, 0, 0, 0, m.Tokens.Write, request.path, request.offset, run}
		affected[c.thread] = true
	}
	start, err := c.checkpointStart()
	if err != nil {
		return err
	}
	for _, p := range c.checkpoints {
		f.files[p.Entry.Path] = []any{p.Entry.Path, c.thread, start}
	}
	if len(affected) == 0 {
		return nil
	}
	q, err := f.reader()
	if err != nil {
		return err
	}
	for session := range affected {
		var first, last sql.NullString
		var started string
		err := q.QueryRowContext(ctx, `SELECT MIN(event_at),MAX(event_at),COALESCE((SELECT MIN(session_started_at) FROM usage_source_files WHERE session_id=? AND session_started_at<>''),'') FROM usage_events WHERE client='codex' AND session_id=?`, session, session).Scan(&first, &last, &started)
		if err != nil {
			return err
		}
		f.sessions[session] = nil
		if first.Valid {
			f.sessions[session] = []any{"codex", session, first.String, last.String, started}
		}
	}
	return nil
}

// Exact-run association is optional: a conflict anywhere in the selected
// aliases (including pre-move ranges) leaves the new event unbound.
func proposedRun(ctx context.Context, tx *sql.Tx, c CJSIUsageCandidate, m ledger.Member) (any, error) {
	if m.ExactRun == 0 {
		return nil, nil
	}
	paths := map[ledger.SourceStamp]string{}
	for _, w := range c.worksets {
		paths[ledger.SelectionStamp(w.Selected)] = w.Selected.Path
	}
	covered := false
	for _, ref := range m.Proof {
		type span struct {
			path       string
			start, end int64
		}
		spans := []span{}
		if path := paths[ref.Source]; path != "" {
			spans = append(spans, span{path, ref.Span.Start, ref.Span.End})
		}
		for _, r := range c.runs {
			if r.Source != ref.Source || !r.Completed || !r.Exact || r.RunID <= 0 || r.Bytes.Start < 0 {
				continue
			}
			lo, hi := max(r.Bytes.Start, ref.Span.Start), min(r.Bytes.End, ref.Span.End)
			if lo < hi && r.OriginalPath != "" {
				spans = append(spans, span{r.OriginalPath, lo, hi})
			}
			if r.RunID != m.ExactRun || r.EndHash == "" || r.Bytes.End > ref.Source.Prefix.Progress.CompleteEnd || r.Bytes.Start > ref.Span.Start || r.Bytes.End < ref.Span.End {
				continue
			}
			path := r.OriginalPath
			if path == "" {
				path = paths[ref.Source]
			}
			var one int
			err := tx.QueryRowContext(ctx, `SELECT 1 FROM usage_runs u JOIN usage_run_sources s ON u.id=s.run_id
			 WHERE u.id=? AND u.client='codex' AND u.ended_at IS NOT NULL AND u.exact=1 AND u.ambiguity_reason=''
			 AND s.path=? AND s.start_offset=? AND s.end_offset=? AND s.start_hash=? AND s.end_hash=?`, m.ExactRun, path, r.Bytes.Start, r.Bytes.End, r.StartHash, r.EndHash).Scan(&one)
			if err == nil {
				covered = true
			} else if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}
		for _, s := range spans {
			var conflict bool
			err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usage_run_sources x JOIN usage_runs y ON y.id=x.run_id
			 WHERE x.run_id<>? AND x.path=? AND y.client='codex' AND y.exact=1 AND y.ended_at IS NOT NULL
			 AND x.end_offset>x.start_offset AND x.start_offset<? AND x.end_offset>?)`, m.ExactRun, s.path, s.end, s.start).Scan(&conflict)
			if err != nil || conflict {
				return nil, err
			}
		}
	}
	if covered {
		return m.ExactRun, nil
	}
	return nil, nil
}
