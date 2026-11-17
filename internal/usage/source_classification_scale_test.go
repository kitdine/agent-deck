package usage

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

// baselineTurnShapesSQL is the query that shipped before the preaggregation
// fix (HEAD 4ac56db). Its assistant-call test is a correlated EXISTS over the
// whole session, so each owned turn re-walks the session's usage events. It is
// kept only as the behavioural oracle and the scaling baseline.
const baselineTurnShapesSQL = `
WITH shapes AS MATERIALIZED (
 SELECT a.client,a.session_id,a.turn_index,
        1 AS any_call,
        MAX(a.tool_kind='edit') AS edited,
        MAX(a.tool_kind='read') AS read_call,
        MAX(a.tool_name IN ('spawn_agent','Task','Agent')) AS delegated,
        MAX(a.tool_name IN ('Skill','Workflow')) AS workflow,
        MAX(a.tool_name IN ('update_plan','TodoWrite')) AS planned,
        MAX(a.command_hint='testing') AS testing,
        MAX(a.command_hint='chore') AS chore
 FROM usage_work_signals owned CROSS JOIN usage_tool_calls a
 ON owned.client=a.client AND owned.session_id=a.session_id AND owned.turn_index=a.turn_index
 WHERE owned.source_path=?1
 GROUP BY a.client,a.session_id,a.turn_index
)
SELECT w.client,w.session_id,w.turn_index,w.message_class,w.intent_sub,
       EXISTS(SELECT 1 FROM usage_events e
              WHERE e.client=w.client AND e.session_id=w.session_id AND e.turn_index=w.turn_index),
       COALESCE(a.any_call,0),COALESCE(a.edited,0),COALESCE(a.read_call,0),
       COALESCE(a.delegated,0),COALESCE(a.workflow,0),COALESCE(a.planned,0),
       COALESCE(a.testing,0),COALESCE(a.chore,0)
FROM usage_work_signals w
LEFT JOIN shapes a
 ON a.client=w.client AND a.session_id=w.session_id AND a.turn_index=w.turn_index
WHERE w.source_path=?1`

func openClassificationTx(tb testing.TB) *sql.Tx {
	tb.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { db.Close() })
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { tx.Rollback() })
	return tx
}

// seedLongSession writes one long Codex session of `turns` turns owned by
// "selected", with usage events and tool calls for most turns, plus the
// neighbours that must never leak into the answer: another client reusing the
// session id, a turn owned by another source, events with a NULL turn_index,
// turns without any usage event, and tool-call-only turns.
func seedLongSession(tb testing.TB, tx *sql.Tx, turns int) {
	tb.Helper()
	ctx := context.Background()
	exec := func(q string, a ...any) {
		tb.Helper()
		if _, err := tx.ExecContext(ctx, q, a...); err != nil {
			tb.Fatal(err)
		}
	}
	names := []string{"spawn_agent", "Task", "Skill", "update_plan", "exec_command", "apply_patch"}
	kinds := []string{"edit", "read", "other"}
	hints := []string{"testing", "chore", ""}
	for i := 0; i < turns; i++ {
		owner := "selected"
		if i%50 == 49 {
			owner = "other"
		}
		exec(`INSERT INTO usage_work_signals(client,session_id,turn_index,started_at,state,message_class,intent_sub,activity_kind,activity_sub,source_path)
 VALUES('codex','long',?,'2026-08-13T01:00:00Z','pending',?,'','','',?)`, i, []string{"", "fault", "build"}[i%3], owner)
		// every 11th turn has no assistant call yet; every 13th has events
		// that were persisted without a turn_index and must not count.
		switch {
		case i%11 == 0:
		case i%13 == 0:
			exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset,turn_index) VALUES(?,'codex','long',?,'2026-08-13T01:00:00Z','gpt-5','foreign-owner',0,NULL)`, fmt.Sprint("n", i), fmt.Sprint("n", i))
		default:
			for k := 0; k < 3; k++ { // several calls per turn: DISTINCT matters
				exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset,turn_index) VALUES(?,'codex','long',?,'2026-08-13T01:00:00Z','gpt-5','foreign-owner',?,?)`, fmt.Sprint(i, "-", k), fmt.Sprint(i, "-", k), k, i)
			}
		}
		if i%7 != 0 {
			for j := 0; j < 2; j++ {
				exec(`INSERT INTO usage_tool_calls(activity_key,client,session_id,model,tool_name,started_at,status,source_path,source_offset,turn_index,tool_kind,command_hint) VALUES(?,'codex','long','gpt-5',?,'2026-08-13T01:00:00Z','started','foreign-owner',0,?,?,?)`, fmt.Sprint(i, "-", j), names[(i+j)%len(names)], i, kinds[(i+j)%3], hints[i%3])
			}
		}
	}
	// A different client with the same session id has usage for turn 1 only.
	exec(`INSERT INTO usage_work_signals(client,session_id,turn_index,started_at,state,message_class,intent_sub,activity_kind,activity_sub,source_path)
 VALUES('claude','long',0,'2026-08-13T01:00:00Z','pending','build','','','','selected'),('claude','long',1,'2026-08-13T01:00:00Z','pending','build','','','','selected')`)
	exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset,turn_index) VALUES('c1','claude','long','c1','2026-08-13T01:00:00Z','m','foreign-owner',0,1)`)
}

func queryTurnShapes(tb testing.TB, tx *sql.Tx, query, path string) []string {
	tb.Helper()
	rows, err := tx.QueryContext(context.Background(), query, path)
	if err != nil {
		tb.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var client, session, message, intent string
		var turn int
		var v [9]int
		if err := rows.Scan(&client, &session, &turn, &message, &intent, &v[0], &v[1], &v[2], &v[3], &v[4], &v[5], &v[6], &v[7], &v[8]); err != nil {
			tb.Fatal(err)
		}
		out = append(out, fmt.Sprint(client, "|", session, "|", turn, "|", message, "|", intent, "|", v))
	}
	if err := rows.Err(); err != nil {
		tb.Fatal(err)
	}
	return out
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func explainPlan(tb testing.TB, tx *sql.Tx, query, path string) string {
	tb.Helper()
	rows, err := tx.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, path)
	if err != nil {
		tb.Fatal(err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			tb.Fatal(err)
		}
		lines = append(lines, detail)
	}
	return strings.Join(lines, "\n")
}

// The preaggregated query must classify exactly what the shipped per-turn
// EXISTS did, including turns without assistant usage, NULL turn_index events,
// turns owned by another source, and another client's identical session id.
func TestSourceTurnShapesMatchBaselineOnLongSession(t *testing.T) {
	tx := openClassificationTx(t)
	seedLongSession(t, tx, 600)
	got := queryTurnShapes(t, tx, sourceTurnShapesSQL, "selected")
	want := queryTurnShapes(t, tx, baselineTurnShapesSQL, "selected")
	if len(got) < 500 {
		t.Fatalf("fixture too small to be meaningful: %d rows", len(got))
	}
	if !reflect.DeepEqual(sortedCopy(got), sortedCopy(want)) {
		t.Fatalf("turn shapes differ from baseline\ngot=%d rows want=%d rows", len(got), len(want))
	}
	var withUsage, withoutUsage int
	for _, row := range got {
		if strings.Contains(row, "|[1 ") {
			withUsage++
		} else {
			withoutUsage++
		}
	}
	if withUsage == 0 || withoutUsage == 0 {
		t.Fatalf("fixture must contain both usage and no-usage turns: %d/%d", withUsage, withoutUsage)
	}
}

// Classification written through classifySourceTurns must equal the per-turn
// reference implementation and be stable when the same scan is repeated.
func TestClassifySourceTurnsLongSessionRepeatable(t *testing.T) {
	tx := openClassificationTx(t)
	seedLongSession(t, tx, 600)
	ctx := context.Background()
	snapshot := func() []string {
		t.Helper()
		rows, err := tx.QueryContext(ctx, "SELECT client,session_id,turn_index,state,activity_kind,activity_sub,source_path FROM usage_work_signals ORDER BY client,session_id,turn_index")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var c, s, state, k, sub, p string
			var turn int
			if err := rows.Scan(&c, &s, &turn, &state, &k, &sub, &p); err != nil {
				t.Fatal(err)
			}
			out = append(out, fmt.Sprint(c, "|", s, "|", turn, "|", state, "|", k, "|", sub, "|", p))
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if err := classifySourceTurns(ctx, tx, "selected"); err != nil {
		t.Fatal(err)
	}
	first := snapshot()
	if err := classifySourceTurns(ctx, tx, "selected"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, snapshot()) {
		t.Fatal("repeating the same scan changed classification")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE usage_work_signals SET state='pending',activity_kind='',activity_sub=''"); err != nil {
		t.Fatal(err)
	}
	if err := legacyClassifySourceTurns(ctx, tx, "selected"); err != nil {
		t.Fatal(err)
	}
	if want := snapshot(); !reflect.DeepEqual(first, want) {
		t.Fatal("classification differs from the per-turn reference")
	}
	var classified, pending, foreign int
	for _, row := range first {
		switch {
		case strings.Contains(row, "|other"):
			foreign++
			if !strings.Contains(row, "|pending|") {
				t.Fatalf("a turn owned by another source was classified: %s", row)
			}
		case strings.Contains(row, "|classified|"):
			classified++
		default:
			pending++
		}
	}
	if classified == 0 || pending == 0 || foreign == 0 {
		t.Fatalf("fixture lost a class: classified=%d pending=%d foreign=%d", classified, pending, foreign)
	}
}

// The plan must not probe usage_events once per owned turn. The baseline plan
// is asserted to do so, which keeps this check able to fail.
func TestSourceTurnShapesPlanHasNoPerTurnEventProbe(t *testing.T) {
	tx := openClassificationTx(t)
	seedLongSession(t, tx, 200)
	baseline := explainPlan(t, tx, baselineTurnShapesSQL, "selected")
	if !strings.Contains(baseline, "CORRELATED SCALAR SUBQUERY") || !strings.Contains(baseline, "usage_events_client_session") {
		t.Fatalf("baseline plan no longer shows the per-turn event probe; oracle is stale:\n%s", baseline)
	}
	final := explainPlan(t, tx, sourceTurnShapesSQL, "selected")
	if strings.Contains(final, "CORRELATED") {
		t.Fatalf("final plan still has a correlated per-turn subquery:\n%s", final)
	}
	for _, line := range strings.Split(final, "\n") {
		if strings.HasPrefix(line, "SCAN") && line != "SCAN s" {
			t.Fatalf("final plan full-scans per owned turn (%q):\n%s", line, final)
		}
	}
	if n := strings.Count(final, "usage_events"); n != 1 {
		t.Fatalf("usage_events should be read by exactly one plan step, got %d:\n%s", n, final)
	}
	if !strings.Contains(final, "usage_work_signals_source") || !strings.Contains(final, "usage_tool_calls_turn") {
		t.Fatalf("final plan lost its bounded indexes:\n%s", final)
	}
}

// BenchmarkLongSessionTurnShapes reports the growth of the shape query over one
// session as turns grow 4x per step: the baseline should grow ~16x per step,
// the preaggregated query ~4x. No time threshold is asserted.
func BenchmarkLongSessionTurnShapes(b *testing.B) {
	for _, variant := range []struct{ name, query string }{
		{"baseline", baselineTurnShapesSQL}, {"final", sourceTurnShapesSQL},
	} {
		for _, turns := range []int{500, 2000, 8000} {
			b.Run(fmt.Sprintf("%s/turns=%d", variant.name, turns), func(b *testing.B) {
				tx := openClassificationTx(b)
				seedLongSession(b, tx, turns)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					rows, err := tx.QueryContext(context.Background(), variant.query, "selected")
					if err != nil {
						b.Fatal(err)
					}
					n := 0
					for rows.Next() {
						n++
					}
					rows.Close()
					if n == 0 {
						b.Fatal("no rows")
					}
				}
			})
		}
	}
}
