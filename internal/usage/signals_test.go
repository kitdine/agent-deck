package usage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func newSignalService(t *testing.T, root string) (*Service, *store.Store, string) {
	t.Helper()
	home := filepath.Join(root, "home")
	database, err := store.Open(context.Background(), filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	service := New(database, home)
	service.MachineIdentity = func(context.Context) (string, error) { return "machine-a", nil }
	return service, database, home
}

func writeSource(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := ""
	for _, line := range lines {
		body += line + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func appendSource(t *testing.T, path string, lines ...string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		if _, err = file.WriteString(line + "\n"); err != nil {
			file.Close()
			t.Fatal(err)
		}
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
}

func signalRow(t *testing.T, database *store.Store, client, session string, turn int) (state, kind, sub, messageClass, source string) {
	t.Helper()
	err := database.DB.QueryRowContext(context.Background(),
		`SELECT state,activity_kind,activity_sub,message_class,source_path FROM usage_work_signals WHERE client=? AND session_id=? AND turn_index=?`,
		client, session, turn).Scan(&state, &kind, &sub, &messageClass, &source)
	if err != nil {
		t.Fatalf("signal row %s/%s/%d: %v", client, session, turn, err)
	}
	return
}

// Decision 11's reason for existing. The turn-opening message lands in one scan
// and the assistant entry lands in the next, so the message text is gone by the
// time the turn can be classified. Classifying from tool shape alone would make
// `debugging` unreachable at any scan boundary, which is the defect this asserts
// against.
func TestClassificationSurvivesAScanBoundary(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".claude", "projects", "boundary.jsonl")
	editPath := filepath.Join(root, "broken.go")

	writeSource(t, source, `{"type":"user","sessionId":"s","timestamp":"2026-08-27T00:00:00Z","message":{"role":"user","content":"fix the crash in the parser"}}`)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if state, kind, _, class, _ := signalRow(t, database, "claude", "s", 1); state != signalStatePending || kind != "" || class != "fault" {
		t.Fatalf("after first scan state=%q kind=%q class=%q, want pending with no verdict and a fault reduction", state, kind, class)
	}

	appendSource(t, source, `{"type":"assistant","timestamp":"2026-08-27T00:00:05Z","sessionId":"s","message":{"role":"assistant","id":"m1","model":"claude-opus","usage":{"input_tokens":10,"output_tokens":1},"content":[{"type":"tool_use","id":"t1","name":"Edit","input":{"file_path":"`+editPath+`"}}]}}`)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	state, kind, sub, class, _ := signalRow(t, database, "claude", "s", 1)
	if state != signalStateClassified || kind != "debugging" || sub != "repair" || class != "fault" {
		t.Fatalf("after boundary state=%q kind=%q sub=%q class=%q, want classified debugging/repair", state, kind, sub, class)
	}
}

// R1-F1: a command-shaped subcategory hint has no home of its own — it must be
// recoverable from usage_tool_calls, not carried in memory across scans. The
// message is scanned alone in the first scan (neutral, names nothing), and the
// edit plus the command that carries the hint arrive only in the second scan.
func TestClassificationRecoversTestingCommandHintAcrossAScanBoundary(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".claude", "projects", "boundary-testing.jsonl")
	editPath := filepath.Join(root, "parser.go")

	writeSource(t, source, claudeUser("s", "2026-08-27T00:00:00Z", "look at the parser"))
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if state, _, _, class, _ := signalRow(t, database, "claude", "s", 1); state != signalStatePending || class != "none" {
		t.Fatalf("after first scan state=%q class=%q, want pending with no message reduction", state, class)
	}

	appendSource(t, source, claudeAssistant("s", "m1", "2026-08-27T00:00:05Z",
		claudeEdit("t1", editPath)+","+claudeBash("t2", "go test ./...")))
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	state, kind, sub, _, _ := signalRow(t, database, "claude", "s", 1)
	if state != signalStateClassified || kind != "coding" || sub != "testing" {
		t.Fatalf("after boundary state=%q kind=%q sub=%q, want classified coding/testing", state, kind, sub)
	}
}

// Same boundary as above, for the chore-shaped command hint.
func TestClassificationRecoversChoreCommandHintAcrossAScanBoundary(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".claude", "projects", "boundary-chore.jsonl")
	editPath := filepath.Join(root, "config.go")

	writeSource(t, source, claudeUser("s", "2026-08-27T00:00:00Z", "look at the config"))
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if state, _, _, class, _ := signalRow(t, database, "claude", "s", 1); state != signalStatePending || class != "none" {
		t.Fatalf("after first scan state=%q class=%q, want pending with no message reduction", state, class)
	}

	appendSource(t, source, claudeAssistant("s", "m1", "2026-08-27T00:00:05Z",
		claudeEdit("t1", editPath)+","+claudeBash("t2", "git commit -am wip")))
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	state, kind, sub, _, _ := signalRow(t, database, "claude", "s", 1)
	if state != signalStateClassified || kind != "coding" || sub != "maintenance" {
		t.Fatalf("after boundary state=%q kind=%q sub=%q, want classified coding/maintenance", state, kind, sub)
	}
}

// Schema v21 replaces the reserved signal table. Version 5 sources must be
// re-read once after upgrade or an existing installation would expose no
// activity classifications until each source happened to change.
func TestParserVersionSixBackfillsActivityClassification(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".claude", "projects", "backfill.jsonl")
	target := filepath.Join(root, "feature.go")
	writeSource(t, source,
		claudeUser("s", "2026-08-27T00:00:00Z", "add a feature"),
		claudeAssistant("s", "m1", "2026-08-27T00:00:01Z", claudeEdit("t1", target)),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.ExecContext(ctx, `DELETE FROM usage_work_signals; UPDATE usage_source_files SET parser_version=5 WHERE path=?`, source); err != nil {
		t.Fatal(err)
	}
	result, err := service.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state, kind, sub, _, _ := signalRow(t, database, "claude", "s", 1)
	var parserVersion int
	if err = database.DB.QueryRowContext(ctx, `SELECT parser_version FROM usage_source_files WHERE path=?`, source).Scan(&parserVersion); err != nil {
		t.Fatal(err)
	}
	if state != signalStateClassified || kind != "coding" || sub != "feature" || parserVersion != usageParserVersion || result["replaced"] == 0 {
		t.Fatalf("state=%q kind=%q sub=%q parser=%d result=%v", state, kind, sub, parserVersion, result)
	}
}

// The pending row is keyed to the index the turn will commit to, so promotion is
// an in-place update. Consecutive user messages with no assistant between them
// target that one key and replace one another: those entries have produced no
// turn yet, and the one that finally draws a reply is the one whose intent the
// turn carries.
func TestConsecutiveUserMessagesReplaceRatherThanAccumulate(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".claude", "projects", "consecutive.jsonl")
	editPath := filepath.Join(root, "main.go")

	writeSource(t, source,
		`{"type":"user","sessionId":"s","timestamp":"2026-08-27T00:00:00Z","message":{"role":"user","content":"fix the crash"}}`,
		`{"type":"user","sessionId":"s","timestamp":"2026-08-27T00:00:01Z","message":{"role":"user","content":"actually, add a new endpoint"}}`,
		`{"type":"assistant","timestamp":"2026-08-27T00:00:05Z","sessionId":"s","message":{"role":"assistant","id":"m1","model":"claude-opus","usage":{"input_tokens":10,"output_tokens":1},"content":[{"type":"tool_use","id":"t1","name":"Write","input":{"file_path":"`+editPath+`"}}]}}`,
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_work_signals WHERE client='claude' AND session_id='s'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("signal rows = %d, want 1 — consecutive messages must replace, not accumulate", rows)
	}
	state, kind, sub, class, _ := signalRow(t, database, "claude", "s", 1)
	if state != signalStateClassified || kind != "coding" || sub != "feature" || class != "build" {
		t.Fatalf("state=%q kind=%q sub=%q class=%q, want the last message's intent to win", state, kind, sub, class)
	}
}

// Re-scanning unchanged content must re-derive the same index, the same
// reduction, and the same row. The classification is a pure function of the
// stored reduction and the turn's tool rows, so idempotence is a property of the
// design rather than of the order rows happen to be written in.
func TestClassifierReplayIsIdempotent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".codex", "sessions", "replay.jsonl")
	editPath := filepath.Join(root, "replay.go")

	writeSource(t, source,
		`{"type":"session_meta","payload":{"session_id":"s"}}`,
		`{"type":"turn_context","payload":{"turn_id":"t","model":"gpt-5.6"}}`,
		`{"type":"event_msg","timestamp":"2026-08-27T00:00:00Z","payload":{"type":"user_message","message":"implement the cache"}}`,
		`{"type":"response_item","timestamp":"2026-08-27T00:00:01Z","payload":{"item":{"type":"function_call","call_id":"c","name":"apply_patch","arguments":{"patch":"*** Update File: `+editPath+`"}}}}`,
		`{"type":"event_msg","timestamp":"2026-08-27T00:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":1}}}}`,
	)
	snapshot := func() string {
		t.Helper()
		var value string
		if err := database.DB.QueryRowContext(ctx,
			`SELECT group_concat(client||'|'||session_id||'|'||turn_index||'|'||state||'|'||message_class||'|'||intent_sub||'|'||activity_kind||'|'||activity_sub||'|'||source_path, ';') FROM (SELECT * FROM usage_work_signals ORDER BY client,session_id,turn_index)`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	first := snapshot()
	if first == "" {
		t.Fatal("no signal rows after the first scan")
	}
	for round := 0; round < 2; round++ {
		if _, err := service.Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if again := snapshot(); again != first {
			t.Fatalf("replay round %d changed rows:\n first=%q\n again=%q", round+1, first, again)
		}
	}
	if _, kind, sub, _, _ := signalRow(t, database, "codex", "s", 1); kind != "coding" || sub != "feature" {
		t.Fatalf("codex turn = %q/%q, want coding/feature", kind, sub)
	}
}

// A chat-only turn calls no tool at all, which is exactly the shape a sweep keyed
// on "no usage_tool_calls row" would delete. Decision 11 forbids that sweep, so a
// conversation turn must survive a reset and a re-scan.
func TestChatOnlyConversationTurnSurvivesReset(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".claude", "projects", "chat.jsonl")

	writeSource(t, source,
		`{"type":"user","sessionId":"s","timestamp":"2026-08-27T00:00:00Z","message":{"role":"user","content":"what do you think of this approach"}}`,
		`{"type":"assistant","timestamp":"2026-08-27T00:00:02Z","sessionId":"s","message":{"role":"assistant","id":"m1","model":"claude-opus","usage":{"input_tokens":5,"output_tokens":2},"content":[{"type":"text","text":"I think it holds."}]}}`,
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	state, kind, sub, _, _ := signalRow(t, database, "claude", "s", 1)
	if state != signalStateClassified || kind != "conversation" {
		t.Fatalf("chat-only turn = %q %q/%q, want a classified conversation", state, kind, sub)
	}
	var toolRows int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_tool_calls WHERE session_id='s'`).Scan(&toolRows); err != nil {
		t.Fatal(err)
	}
	if toolRows != 0 {
		t.Fatalf("tool rows = %d, want 0 — this test is only meaningful without them", toolRows)
	}
	// Force the reset path by rewriting the source with the same content plus a
	// trailing turn, then confirm the conversation row is rebuilt rather than
	// swept away.
	writeSource(t, source,
		`{"type":"user","sessionId":"s","timestamp":"2026-08-27T00:00:00Z","message":{"role":"user","content":"what do you think of this approach"}}`,
		`{"type":"assistant","timestamp":"2026-08-27T00:00:02Z","sessionId":"s","message":{"role":"assistant","id":"m1","model":"claude-opus","usage":{"input_tokens":5,"output_tokens":2},"content":[{"type":"text","text":"I think it holds."}]}}`,
		`{"type":"user","sessionId":"s","timestamp":"2026-08-27T00:00:03Z","message":{"role":"user","content":"ok"}}`,
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if state, kind, _, _, _ = signalRow(t, database, "claude", "s", 1); state != signalStateClassified || kind != "conversation" {
		t.Fatalf("after reset turn 1 = %q %q, want the conversation turn rebuilt", state, kind)
	}
}

// Re-scanning a rewritten source must leave no row from the old content.
func TestRewrittenSourceLeavesNoStaleSignalRow(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".claude", "projects", "rewritten.jsonl")
	editPath := filepath.Join(root, "x.go")

	writeSource(t, source,
		`{"type":"user","sessionId":"s","timestamp":"2026-08-27T00:00:00Z","message":{"role":"user","content":"fix the crash"}}`,
		`{"type":"assistant","timestamp":"2026-08-27T00:00:01Z","sessionId":"s","message":{"role":"assistant","id":"m1","model":"claude-opus","usage":{"input_tokens":5,"output_tokens":1},"content":[{"type":"tool_use","id":"t1","name":"Edit","input":{"file_path":"`+editPath+`"}}]}}`,
		`{"type":"user","sessionId":"s","timestamp":"2026-08-27T00:00:02Z","message":{"role":"user","content":"add a test"}}`,
		`{"type":"assistant","timestamp":"2026-08-27T00:00:03Z","sessionId":"s","message":{"role":"assistant","id":"m2","model":"claude-opus","usage":{"input_tokens":5,"output_tokens":1},"content":[{"type":"tool_use","id":"t2","name":"Write","input":{"file_path":"`+editPath+`"}}]}}`,
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var before int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_work_signals WHERE source_path=?`, source).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if before != 2 {
		t.Fatalf("rows before rewrite = %d, want 2", before)
	}
	writeSource(t, source,
		`{"type":"user","sessionId":"s","timestamp":"2026-08-27T00:00:00Z","message":{"role":"user","content":"fix the crash"}}`,
		`{"type":"assistant","timestamp":"2026-08-27T00:00:01Z","sessionId":"s","message":{"role":"assistant","id":"m1","model":"claude-opus","usage":{"input_tokens":5,"output_tokens":1},"content":[{"type":"tool_use","id":"t1","name":"Edit","input":{"file_path":"`+editPath+`"}}]}}`,
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var after int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_work_signals WHERE source_path=?`, source).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != 1 {
		t.Fatalf("rows after truncating rewrite = %d, want 1 — the second turn is gone from the source", after)
	}
}
