package usage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/store"
	"reflect"
	"testing"
)

func seedClassificationBenchmark(tb testing.TB) (*sql.Tx, string) {
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
	exec := func(q string, a ...any) {
		tb.Helper()
		if _, err := tx.ExecContext(ctx, q, a...); err != nil {
			tb.Fatal(err)
		}
	}
	for i := 0; i < 120; i++ {
		owner := "selected"
		if i >= 100 {
			owner = "other"
		}
		exec(`INSERT INTO usage_work_signals(client,session_id,turn_index,started_at,state,message_class,intent_sub,activity_kind,activity_sub,source_path)
 VALUES('codex','s',?,'2026-08-13T01:00:00Z','pending',?,'','','',?)`, i, []string{"", "fault", "build"}[i%3], owner)
		if i%9 != 0 {
			exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset,turn_index) VALUES(?,'codex','s',?,'2026-08-13T01:00:00Z','gpt-5','foreign-owner',0,?)`, fmt.Sprint(i), fmt.Sprint(i), i)
		}
		if i%7 != 0 {
			names := []string{"spawn_agent", "Task", "Agent", "Skill", "Workflow", "update_plan", "TodoWrite", "exec_command"}
			kinds := []string{"edit", "read", "other"}
			hints := []string{"testing", "chore", ""}
			for j := 0; j < 10; j++ {
				exec(`INSERT INTO usage_tool_calls(activity_key,client,session_id,model,tool_name,started_at,status,source_path,source_offset,turn_index,tool_kind,command_hint) VALUES(?,'codex','s','gpt-5',?,'2026-08-13T01:00:00Z','started','foreign-owner',0,?,?,?)`, fmt.Sprintf("%d-%d", i, j), names[(i+j)%len(names)], i, kinds[(i+j)%3], hints[i%3])
			}
		}
	}
	return tx, "selected"
}
func TestSourceClassificationMatchesPerTurnReference(t *testing.T) {
	tx, path := seedClassificationBenchmark(t)
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
			out = append(out, fmt.Sprint(c, s, turn, state, k, sub, p))
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if err := classifySourceTurns(ctx, tx, path); err != nil {
		t.Fatal(err)
	}
	got := snapshot()
	if _, err := tx.ExecContext(ctx, "UPDATE usage_work_signals SET state='pending',activity_kind='',activity_sub=''"); err != nil {
		t.Fatal(err)
	}
	if err := legacyClassifySourceTurns(ctx, tx, path); err != nil {
		t.Fatal(err)
	}
	if want := snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("batched classification differs\ngot=%v\nwant=%v", got, want)
	}
	rows, err := tx.QueryContext(ctx, "EXPLAIN QUERY PLAN "+sourceTurnShapesSQL, path)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		t.Log(detail)
	}
}
func BenchmarkSourceClassification(b *testing.B) {
	for _, method := range []struct {
		name string
		fn   func(context.Context, *sql.Tx, string) error
	}{{"per_turn", legacyClassifySourceTurns}, {"batched", classifySourceTurns}} {
		b.Run(method.name, func(b *testing.B) {
			tx, path := seedClassificationBenchmark(b)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := method.fn(context.Background(), tx, path); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
func legacyClassifySourceTurns(ctx context.Context, tx *sql.Tx, path string) error {
	rows, err := tx.QueryContext(ctx, `SELECT client,session_id,turn_index,message_class,intent_sub FROM usage_work_signals WHERE source_path=?`, path)
	if err != nil {
		return err
	}
	type pending struct {
		client, session         string
		turnIndex               int
		messageClass, intentSub string
	}
	var candidates []pending
	for rows.Next() {
		var candidate pending
		if err = rows.Scan(&candidate.client, &candidate.session, &candidate.turnIndex, &candidate.messageClass, &candidate.intentSub); err != nil {
			rows.Close()
			return err
		}
		candidates = append(candidates, candidate)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, candidate := range candidates {
		var assistantCalls int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_events WHERE client=? AND session_id=? AND turn_index=?`,
			candidate.client, candidate.session, candidate.turnIndex).Scan(&assistantCalls); err != nil {
			return err
		}
		if assistantCalls == 0 {
			continue
		}
		shape, shapeErr := turnShape(ctx, tx, candidate.client, candidate.session, candidate.turnIndex)
		if shapeErr != nil {
			return shapeErr
		}
		// brainstorming is false here by construction: it is answered from the
		// message in hand, and Decision 2's persisted set carries message_class
		// and intent_sub and nothing else. A tool-less turn whose message and
		// reply fall in different scans takes the visible `exploration`
		// fallback. Widening the persisted set is a design change.
		kind, sub := activity.Classify(candidate.messageClass, candidate.intentSub, shape, false)
		if _, err = tx.ExecContext(ctx, `UPDATE usage_work_signals SET state=?,activity_kind=?,activity_sub=? WHERE client=? AND session_id=? AND turn_index=?`,
			signalStateClassified, kind, sub, candidate.client, candidate.session, candidate.turnIndex); err != nil {
			return err
		}
	}
	return nil
}
