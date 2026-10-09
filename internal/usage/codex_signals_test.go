package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func codexSignalJSON(t *testing.T, timestamp, kind string, payload map[string]any) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"timestamp": timestamp, "type": kind, "payload": payload})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCodexInjectedContextDoesNotReplaceUserIntent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".codex", "sessions", "context.jsonl")
	lines := []string{
		`{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"s"}}`,
		`{"timestamp":"2026-10-08T12:00:00Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}`,
		codexSignalUser(t, "2026-10-08T12:00:00Z", "look at the parser"),
		codexSignalUser(t, "2026-10-08T12:00:00Z", "# AGENTS.md instructions for /fixture\n<INSTRUCTIONS>fix every crash</INSTRUCTIONS>"),
		codexSignalUser(t, "2026-10-08T12:00:00Z", "<environment_context>fix the failing environment</environment_context>"),
	}
	writeSource(t, source, append(lines, codexSignalOutput(t, filepath.Join(root, "parser.go"))...)...)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	state, kind, sub, class, _ := signalRow(t, database, "codex", "s", 1)
	if state != signalStateClassified || kind != "coding" || sub != "feature" || class != "none" {
		t.Fatalf("state=%q kind=%q sub=%q class=%q, want the real neutral message's coding/feature classification", state, kind, sub, class)
	}
}

func TestCodexSignalUpgradeReplaysOnlyCodex(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".codex", "sessions", "history.jsonl")
	claude := filepath.Join(home, ".claude", "projects", "unchanged.jsonl")
	lines := []string{
		`{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"s"}}`,
		`{"timestamp":"2026-10-08T12:00:00Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}`,
		codexSignalUser(t, "2026-10-08T12:00:00Z", "fix the crash"),
	}
	writeSource(t, source, append(lines, codexSignalOutput(t, filepath.Join(root, "parser.go"))...)...)
	writeSource(t, claude, claudeUser("other", "2026-10-08T12:00:00Z", "add the feature"), claudeAssistant("other", "m1", "2026-10-08T12:00:01Z", claudeEdit("other-edit", filepath.Join(root, "other.go"))))
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	bytesBefore, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	eventSnapshot := func() string {
		t.Helper()
		var value string
		if err := database.DB.QueryRowContext(ctx, `SELECT GROUP_CONCAT(event_key||':'||input_tokens||':'||output_tokens||':'||turn_index) FROM (SELECT * FROM usage_events ORDER BY event_key)`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	eventsBefore := eventSnapshot()
	if _, err := database.DB.ExecContext(ctx, `DELETE FROM usage_work_signals WHERE client='codex'; UPDATE usage_source_files SET parser_version=7; UPDATE usage_source_files SET turn_id='t1' WHERE path=?`, source); err != nil {
		t.Fatal(err)
	}
	inventory, err := service.Inventory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inventory.Mutated, []string{source}) || !inventory.ParserVersionReread {
		t.Fatalf("mutated=%v parser reread=%v, want only the old Codex source", inventory.Mutated, inventory.ParserVersionReread)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.ScanInventory(canceled, inventory); err == nil {
		t.Fatal("canceled history replay unexpectedly succeeded")
	}
	var version int
	if err := database.DB.QueryRowContext(ctx, `SELECT parser_version FROM usage_source_files WHERE path=?`, source).Scan(&version); err != nil || version != 7 {
		t.Fatalf("canceled replay advanced source version=%d err=%v", version, err)
	}
	if _, err := service.ScanInventory(ctx, inventory); err != nil {
		t.Fatal(err)
	}
	state, kind, _, _, _ := signalRow(t, database, "codex", "s", 1)
	if state != signalStateClassified || kind != "debugging" {
		t.Fatalf("history state=%q kind=%q", state, kind)
	}
	if err := database.DB.QueryRowContext(ctx, `SELECT parser_version FROM usage_source_files WHERE path=?`, claude).Scan(&version); err != nil || version != 7 {
		t.Fatalf("unrelated Claude source version=%d err=%v, want unchanged 7", version, err)
	}
	result, err := service.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bytesAfter, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if result["replaced"] != 0 || eventsBefore != eventSnapshot() || !reflect.DeepEqual(bytesBefore, bytesAfter) {
		t.Fatalf("replay was not idempotent or changed source/event identity: result=%v", result)
	}
}

func codexSignalUser(t *testing.T, timestamp, text string) string {
	t.Helper()
	return codexSignalJSON(t, timestamp, "response_item", map[string]any{
		"type": "message", "role": "user",
		"content": []any{map[string]any{"type": "input_text", "text": text}},
	})
}

func codexSignalOutput(t *testing.T, target string) []string {
	t.Helper()
	return []string{
		codexSignalJSON(t, "2026-10-08T12:00:02Z", "response_item", map[string]any{
			"type": "custom_tool_call", "name": "apply_patch", "call_id": "edit-1",
			"input": "*** Begin Patch\n*** Add File: " + target + "\n+package fixture\n*** End Patch",
		}),
		`{"timestamp":"2026-10-08T12:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100},"total_token_usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100}}}}`,
	}
}

func TestCodexResponseMessagePersistsWorkSignal(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".codex", "sessions", "signals.jsonl")
	lines := []string{
		`{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"s"}}`,
		`{"timestamp":"2026-10-08T12:00:00Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}`,
		codexSignalUser(t, "2026-10-08T12:00:00Z", "fix the crash in the parser"),
	}
	writeSource(t, source, append(lines, codexSignalOutput(t, filepath.Join(root, "parser.go"))...)...)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var events, calls int
	if err := database.DB.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM usage_events),(SELECT COUNT(*) FROM usage_tool_calls)`).Scan(&events, &calls); err != nil {
		t.Fatal(err)
	}
	if events != 1 || calls != 1 {
		t.Fatalf("fixture events=%d calls=%d, want one of each", events, calls)
	}
	state, kind, sub, class, owner := signalRow(t, database, "codex", "s", 1)
	if state != signalStateClassified || kind != "debugging" || sub != "repair" || class != "fault" || owner != source {
		t.Fatalf("state=%q kind=%q sub=%q class=%q owner=%q, want classified debugging/repair owned by %q", state, kind, sub, class, owner, source)
	}
	options := SignalOptions{Period: "today", Client: "codex", IncludeSub: true,
		From: time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}
	report, err := service.Signals(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if report.Activity == nil || !report.Activity.Available || report.Activity.CostBasis != CostBasisTurn ||
		report.Workflow == nil || !report.Workflow.Available || report.Workflow.FirstEditSeconds == nil || *report.Workflow.FirstEditSeconds != 2 ||
		report.Workflow.FilesTouched == nil || *report.Workflow.FilesTouched != 1 || report.Workflow.Retries == nil || *report.Workflow.Retries != 0 ||
		report.Tooling == nil || !report.Tooling.Available || report.Tooling.Calls != 1 {
		t.Fatalf("unexpected signal projection: %+v", report)
	}
	batch, err := service.SignalsBatch(ctx, []SignalOptions{options})
	if err != nil || !reflect.DeepEqual(batch, []SignalReport{report}) {
		t.Fatalf("individual and desktop batch projections differ: err=%v", err)
	}
}

func TestCodexSecondTurnAndDuplicateEnvelopeAcrossAppend(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".codex", "sessions", "second.jsonl")
	lines := []string{
		`{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"s"}}`,
		`{"timestamp":"2026-10-08T12:00:00Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}`,
		codexSignalUser(t, "2026-10-08T12:00:00Z", "fix the crash"),
	}
	writeSource(t, source, append(lines, codexSignalOutput(t, filepath.Join(root, "first.go"))...)...)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	appendSource(t, source, codexSignalUser(t, "2026-10-08T12:01:00Z", "add the next feature"))
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if state, _, _, class, _ := signalRow(t, database, "codex", "s", 2); state != signalStatePending || class != "build" {
		t.Fatalf("second message state=%q class=%q, want pending/build", state, class)
	}
	appendSource(t, source,
		`{"timestamp":"2026-10-08T12:01:00Z","type":"turn_context","payload":{"turn_id":"t2","model":"gpt-5"}}`,
		codexSignalJSON(t, "2026-10-08T12:01:00Z", "event_msg", map[string]any{"type": "user_message", "message": "add the next feature"}),
		codexSignalJSON(t, "2026-10-08T12:01:02Z", "response_item", map[string]any{"type": "custom_tool_call", "name": "apply_patch", "call_id": "edit-2", "input": "*** Begin Patch\n*** Add File: " + filepath.Join(root, "second.go") + "\n+package fixture\n*** End Patch"}),
		`{"timestamp":"2026-10-08T12:01:03Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"output_tokens":100},"total_token_usage":{"input_tokens":2000,"output_tokens":200}}}}`,
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if state, kind, sub, class, _ := signalRow(t, database, "codex", "s", 2); state != signalStateClassified || kind != "coding" || sub != "feature" || class != "build" {
		t.Fatalf("second turn state=%q kind=%q sub=%q class=%q", state, kind, sub, class)
	}
	if _, kind, sub, class, _ := signalRow(t, database, "codex", "s", 1); kind != "debugging" || sub != "repair" || class != "fault" {
		t.Fatal("next message contaminated the first turn")
	}
	var events, calls, signals, maxTurn int
	if err := database.DB.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM usage_events),(SELECT COUNT(*) FROM usage_tool_calls),(SELECT COUNT(*) FROM usage_work_signals),(SELECT MAX(turn_index) FROM usage_events)`).Scan(&events, &calls, &signals, &maxTurn); err != nil {
		t.Fatal(err)
	}
	if events != 2 || calls != 2 || signals != 2 || maxTurn != 2 {
		t.Fatalf("events=%d calls=%d signals=%d max turn=%d, want exactly two logical turns", events, calls, signals, maxTurn)
	}
}

func TestCodexMessageSignalsKeepDuplicateSourceOwnership(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	lower := filepath.Join(home, ".codex", "sessions", "a.jsonl")
	higher := filepath.Join(home, ".codex", "sessions", "z.jsonl")
	for _, fixture := range []struct{ path, message string }{{lower, "look around"}, {higher, "fix the crash"}} {
		lines := []string{
			`{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"s"}}`,
			`{"timestamp":"2026-10-08T12:00:00Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}`,
			codexSignalUser(t, "2026-10-08T12:00:00Z", fixture.message),
		}
		writeSource(t, fixture.path, append(lines, codexSignalOutput(t, filepath.Join(root, "parser.go"))...)...)
	}
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if _, kind, _, _, owner := signalRow(t, database, "codex", "s", 1); kind != "debugging" || owner != higher {
		t.Fatalf("kind=%q owner=%q, want the indexed last source", kind, owner)
	}
	if err := os.Remove(higher); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if _, kind, _, _, owner := signalRow(t, database, "codex", "s", 1); kind != "coding" || owner != lower {
		t.Fatalf("orphan recovery kind=%q owner=%q, want remaining source", kind, owner)
	}
}

func TestCodexMessageContextAndAppendBoundaries(t *testing.T) {
	for _, before := range []bool{false, true} {
		for _, split := range []bool{false, true} {
			t.Run(map[bool]string{false: "after", true: "before"}[before]+map[bool]string{false: "/one-scan", true: "/append"}[split], func(t *testing.T) {
				ctx := context.Background()
				root := t.TempDir()
				service, database, home := newSignalService(t, root)
				source := filepath.Join(home, ".codex", "sessions", "boundary.jsonl")
				meta := `{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"s"}}`
				turn := `{"timestamp":"2026-10-08T12:00:00Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}`
				user := codexSignalUser(t, "2026-10-08T12:00:00Z", "fix the crash")
				first, second := []string{meta, turn, user}, codexSignalOutput(t, filepath.Join(root, "parser.go"))
				if before {
					first, second = []string{meta, user}, append([]string{turn}, second...)
				}
				if !split {
					first, second = append(first, second...), nil
				}
				writeSource(t, source, first...)
				if _, err := service.Scan(ctx); err != nil {
					t.Fatal(err)
				}
				if split {
					state, kind, _, class, _ := signalRow(t, database, "codex", "s", 1)
					if state != signalStatePending || kind != "" || class != "fault" {
						t.Fatalf("before output: state=%q kind=%q class=%q", state, kind, class)
					}
					appendSource(t, source, second...)
					if _, err := service.Scan(ctx); err != nil {
						t.Fatal(err)
					}
				}
				state, kind, sub, _, _ := signalRow(t, database, "codex", "s", 1)
				if state != signalStateClassified || kind != "debugging" || sub != "repair" {
					t.Fatalf("after output: state=%q kind=%q sub=%q", state, kind, sub)
				}
				var eventTurn, callTurn, rows int
				if err := database.DB.QueryRowContext(ctx, `SELECT (SELECT turn_index FROM usage_events),(SELECT turn_index FROM usage_tool_calls),(SELECT COUNT(*) FROM usage_work_signals)`).Scan(&eventTurn, &callTurn, &rows); err != nil {
					t.Fatal(err)
				}
				if eventTurn != 1 || callTurn != 1 || rows != 1 {
					t.Fatalf("event turn=%d call turn=%d signals=%d, want one logical turn", eventTurn, callTurn, rows)
				}
			})
		}
	}
}

func TestCodexUsageNotificationsBeforeUserKeepCurrentTurn(t *testing.T) {
	notifications := []struct{ name, record string }{
		{"null-info", `{"timestamp":"2026-10-08T12:00:00Z","type":"event_msg","payload":{"type":"token_count","info":null,"rate_limits":{"primary":{"used_percent":1}}}}`},
		{"rate-only", `{"timestamp":"2026-10-08T12:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":1}}}}`},
		{"empty-info", `{"timestamp":"2026-10-08T12:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{}}}`},
		{"empty-usage", `{"timestamp":"2026-10-08T12:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{},"total_token_usage":{}}}}`},
		{"zero-usage", `{"timestamp":"2026-10-08T12:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":0,"output_tokens":0},"total_token_usage":{"input_tokens":0,"output_tokens":0}}}}`},
	}
	for _, notification := range notifications {
		for _, boundary := range []struct {
			name string
			end  int
		}{{"single", 6}, {"after-context", 2}, {"after-notification", 3}, {"after-user", 4}} {
			t.Run(notification.name+"/"+boundary.name, func(t *testing.T) {
				ctx := context.Background()
				root := t.TempDir()
				service, database, home := newSignalService(t, root)
				source := filepath.Join(home, ".codex", "sessions", "notifications.jsonl")
				lines := []string{
					`{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"s"}}`,
					`{"timestamp":"2026-10-08T12:00:00Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}`,
					notification.record,
					codexSignalUser(t, "2026-10-08T12:00:00Z", "fix the crash"),
				}
				lines = append(lines, codexSignalOutput(t, filepath.Join(root, "parser.go"))...)
				writeSource(t, source, lines[:boundary.end]...)
				if _, err := service.Scan(ctx); err != nil {
					t.Fatal(err)
				}
				if boundary.end < len(lines) {
					if boundary.end == 4 {
						var index int
						var state string
						if err := database.DB.QueryRowContext(ctx, `SELECT turn_index,state FROM usage_work_signals WHERE client='codex' AND session_id='s'`).Scan(&index, &state); err != nil {
							t.Fatal(err)
						}
						if index != 1 || state != signalStatePending {
							t.Fatalf("before assistant: signal_turn=%d state=%s, want 1/pending", index, state)
						}
					}
					appendSource(t, source, lines[boundary.end:]...)
					if _, err := service.Scan(ctx); err != nil {
						t.Fatal(err)
					}
				}
				var eventTurn, callTurn, signalTurn, signals int
				var state, kind, sub string
				if err := database.DB.QueryRowContext(ctx, `SELECT
				 COALESCE((SELECT turn_index FROM usage_events LIMIT 1),-1),
				 COALESCE((SELECT turn_index FROM usage_tool_calls LIMIT 1),-1),
				 COALESCE((SELECT turn_index FROM usage_work_signals LIMIT 1),-1),
				 (SELECT COUNT(*) FROM usage_work_signals),
				 COALESCE((SELECT state FROM usage_work_signals LIMIT 1),''),
				 COALESCE((SELECT activity_kind FROM usage_work_signals LIMIT 1),''),
				 COALESCE((SELECT activity_sub FROM usage_work_signals LIMIT 1),'')`).Scan(&eventTurn, &callTurn, &signalTurn, &signals, &state, &kind, &sub); err != nil {
					t.Fatal(err)
				}
				if eventTurn != 1 || callTurn != 1 || signalTurn != 1 || signals != 1 || state != signalStateClassified || kind != "debugging" || sub != "repair" {
					t.Fatalf("event_turn=%d call_turn=%d signal_turn=%d signals=%d state=%s kind=%s/%s; want 1/1/1, one classified debugging/repair signal", eventTurn, callTurn, signalTurn, signals, state, kind, sub)
				}
			})
		}
	}
}

func TestCodexRealTokenUsageStillOpensNextUserTurn(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".codex", "sessions", "real-output.jsonl")
	output := codexSignalOutput(t, filepath.Join(root, "unused.go"))
	writeSource(t, source,
		`{"timestamp":"2026-10-08T12:00:00Z","type":"session_meta","payload":{"id":"s"}}`,
		`{"timestamp":"2026-10-08T12:00:00Z","type":"turn_context","payload":{"turn_id":"t1","model":"gpt-5"}}`,
		codexSignalUser(t, "2026-10-08T12:00:00Z", "explain the parser"),
		output[1],
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	appendSource(t, source, codexSignalUser(t, "2026-10-08T12:01:00Z", "add the next feature"))
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if state, kind, _, class, _ := signalRow(t, database, "codex", "s", 2); state != signalStatePending || kind != "" || class != "build" {
		t.Fatalf("after real token output state=%s kind=%s class=%s, want next turn pending/build", state, kind, class)
	}
	var eventTurn, calls int
	if err := database.DB.QueryRowContext(ctx, `SELECT (SELECT turn_index FROM usage_events),(SELECT COUNT(*) FROM usage_tool_calls)`).Scan(&eventTurn, &calls); err != nil || eventTurn != 1 || calls != 0 {
		t.Fatalf("real-output control: event_turn=%d calls=%d err=%v", eventTurn, calls, err)
	}
}
