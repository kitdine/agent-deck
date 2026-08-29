package usage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// The five metrics are always read over a window wider than the fixtures, so a
// test that means to exercise the scope bound says so by narrowing it itself.
func wideScope() SignalScope {
	return SignalScope{
		From: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}
}

func claudeUser(session, at, text string) string {
	return `{"type":"user","sessionId":"` + session + `","timestamp":"` + at + `","message":{"role":"user","content":"` + text + `"}}`
}

func claudeAssistant(session, id, at, tools string) string {
	return `{"type":"assistant","timestamp":"` + at + `","sessionId":"` + session + `","message":{"role":"assistant","id":"` + id +
		`","model":"claude-opus","usage":{"input_tokens":10,"output_tokens":1},"content":[` + tools + `]}}`
}

func claudeEdit(id, path string) string {
	return `{"type":"tool_use","id":"` + id + `","name":"Edit","input":{"file_path":"` + path + `"}}`
}

func claudeBash(id, command string) string {
	return `{"type":"tool_use","id":"` + id + `","name":"Bash","input":{"command":"` + command + `"}}`
}

func codexPatch(callID, at, path string) string {
	return `{"type":"response_item","timestamp":"` + at + `","payload":{"item":{"type":"function_call","call_id":"` + callID +
		`","name":"apply_patch","arguments":{"patch":"*** Update File: ` + path + `"}}}}`
}

func codexShell(callID, at, command string) string {
	return `{"type":"response_item","timestamp":"` + at + `","payload":{"item":{"type":"function_call","call_id":"` + callID +
		`","name":"exec_command","arguments":{"cmd":"` + command + `"}}}}`
}

func codexTokens(at string) string {
	return `{"type":"event_msg","timestamp":"` + at + `","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"output_tokens":1}}}}`
}

func codexTurn(turnID, at, message string) []string {
	return []string{
		`{"type":"turn_context","payload":{"turn_id":"` + turnID + `","model":"gpt-5.6"}}`,
		`{"type":"event_msg","timestamp":"` + at + `","payload":{"type":"user_message","message":"` + message + `"}}`,
	}
}

func metricsOf(t *testing.T, service *Service, scope SignalScope) WorkflowMetrics {
	t.Helper()
	metrics, err := service.WorkflowMetrics(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	return metrics
}

func wantInt(t *testing.T, name string, got *int, want int) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s is unavailable, want %d", name, want)
	}
	if *got != want {
		t.Fatalf("%s = %d, want %d", name, *got, want)
	}
}

// Decision 6 gives every metric an unavailable condition, and none of them is
// zero. A scope with sessions that never edited reports `edits_per_session` 0
// and the four edit-derived metrics unavailable; an empty scope reports all five
// unavailable. A surface cannot tell the two apart unless the derivation does.
func TestWorkflowMetricsSeparateUnavailableFromZero(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, _, home := newSignalService(t, root)

	empty := metricsOf(t, service, wideScope())
	if empty.FirstEditSeconds != nil || empty.FilesTouched != nil || empty.Retries != nil ||
		empty.EditsPerSession != nil || empty.TopFile != nil || empty.TopFileEdits != nil {
		t.Fatalf("empty scope reported a value: %+v", empty)
	}

	writeSource(t, filepath.Join(home, ".claude", "projects", "chat.jsonl"),
		claudeUser("s", "2026-08-27T00:00:00Z", "what do you think of this approach"),
		claudeAssistant("s", "m1", "2026-08-27T00:00:02Z", `{"type":"text","text":"it holds"}`),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	metrics := metricsOf(t, service, wideScope())
	if metrics.EditsPerSession == nil || *metrics.EditsPerSession != 0 {
		t.Fatalf("edits_per_session = %v, want an available 0 — the session exists and wrote nothing", metrics.EditsPerSession)
	}
	if metrics.FilesTouched != nil || metrics.TopFile != nil || metrics.TopFileEdits != nil {
		t.Fatalf("no written file in scope, yet a file metric reported a value: %+v", metrics)
	}
	if metrics.FirstEditSeconds != nil || metrics.Retries != nil {
		t.Fatalf("no session reached an edit, yet first_edit_seconds/retries reported a value: %+v", metrics)
	}
}

// Decision 6's own example: a Codex call that reads two files and writes a third
// counts as one file touched, not three. The metric reads `usage_tool_files`
// rows with `wrote = 1`, so the two read rows on the same call contribute
// nothing even though the parent call is edit-shaped.
func TestWorkflowMetricsCountOnlyWrittenFileRows(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	read1 := filepath.Join(root, "read-one.go")
	read2 := filepath.Join(root, "read-two.go")
	written := filepath.Join(root, "written.go")

	lines := codexTurn("t1", "2026-08-27T00:00:00Z", "implement the merge")
	lines = append(lines,
		codexShell("c1", "2026-08-27T00:00:01Z", "cat "+read1+" "+read2+" > "+written),
		codexTokens("2026-08-27T00:00:02Z"),
	)
	writeSource(t, filepath.Join(home, ".codex", "sessions", "files.jsonl"),
		append([]string{`{"type":"session_meta","payload":{"session_id":"s"}}`}, lines...)...)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var totalRows, writtenRows int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*),SUM(wrote) FROM usage_tool_files`).Scan(&totalRows, &writtenRows); err != nil {
		t.Fatal(err)
	}
	if totalRows != 3 || writtenRows != 1 {
		t.Fatalf("tool file rows = %d (%d written), want 3 rows of which 1 is written — the fixture no longer exercises the split", totalRows, writtenRows)
	}
	metrics := metricsOf(t, service, wideScope())
	wantInt(t, "files_touched", metrics.FilesTouched, 1)
	wantInt(t, "top_file_edits", metrics.TopFileEdits, 1)
	if metrics.TopFile == nil || *metrics.TopFile != "written.go" {
		t.Fatalf("top_file = %v, want the written file", metrics.TopFile)
	}
	if metrics.EditsPerSession == nil || *metrics.EditsPerSession != 1 {
		t.Fatalf("edits_per_session = %v, want 1 — one written row over one session", metrics.EditsPerSession)
	}
}

// The rework pattern of Decision 6: an edit, a non-read-shaped shell command,
// and the same file edited again.
func TestReworkCountsAnEditVerifyEditCycle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, _, home := newSignalService(t, root)
	target := filepath.Join(root, "target.go")

	lines := codexTurn("t1", "2026-08-27T00:00:00Z", "implement the parser")
	lines = append(lines,
		codexPatch("c1", "2026-08-27T00:00:01Z", target),
		codexShell("c2", "2026-08-27T00:00:02Z", "go test ./..."),
		codexPatch("c3", "2026-08-27T00:00:03Z", target),
		codexTokens("2026-08-27T00:00:04Z"),
	)
	writeSource(t, filepath.Join(home, ".codex", "sessions", "rework.jsonl"),
		append([]string{`{"type":"session_meta","payload":{"session_id":"s"}}`}, lines...)...)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	metrics := metricsOf(t, service, wideScope())
	wantInt(t, "retries", metrics.Retries, 1)
	wantInt(t, "files_touched", metrics.FilesTouched, 1)
	wantInt(t, "top_file_edits", metrics.TopFileEdits, 2)
}

// `edit → rg → edit` is research, not rework. The read-shaped exclusion is what
// keeps a shell-first workflow from being penalized, and this asserts the
// resulting 0 is an available 0 rather than the unavailable that the same scope
// would report if no session had reached an edit at all.
func TestReworkExcludesReadShapedCommandsBetweenEdits(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	target := filepath.Join(root, "target.go")

	lines := codexTurn("t1", "2026-08-27T00:00:00Z", "implement the parser")
	lines = append(lines,
		codexPatch("c1", "2026-08-27T00:00:01Z", target),
		codexShell("c2", "2026-08-27T00:00:02Z", "rg needle"),
		codexPatch("c3", "2026-08-27T00:00:03Z", target),
		codexTokens("2026-08-27T00:00:04Z"),
	)
	writeSource(t, filepath.Join(home, ".codex", "sessions", "research.jsonl"),
		append([]string{`{"type":"session_meta","payload":{"session_id":"s"}}`}, lines...)...)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var kind string
	if err := database.DB.QueryRowContext(ctx, `SELECT tool_kind FROM usage_tool_calls WHERE activity_key LIKE '%c2'`).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if kind != "read" {
		t.Fatalf("the middle call is %q, want read — this test only means something while the allowlist recognizes it", kind)
	}
	metrics := metricsOf(t, service, wideScope())
	wantInt(t, "retries", metrics.Retries, 0)
}

// CodeBurn counts rework within one turn; Decision 6 counts it within a session,
// so a fix that spans three turns is counted here and would not be counted
// there. This is the one place the two definitions differ, and asserting it is
// what keeps the narrower window from being reintroduced as an optimization.
func TestReworkWindowIsTheSessionNotTheTurn(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	target := filepath.Join(root, "target.go")

	lines := []string{`{"type":"session_meta","payload":{"session_id":"s"}}`}
	lines = append(lines, codexTurn("t1", "2026-08-27T00:00:00Z", "implement the parser")...)
	lines = append(lines, codexPatch("c1", "2026-08-27T00:00:01Z", target), codexTokens("2026-08-27T00:00:02Z"))
	lines = append(lines, codexTurn("t2", "2026-08-27T00:00:03Z", "run the suite")...)
	lines = append(lines, codexShell("c2", "2026-08-27T00:00:04Z", "go test ./..."), codexTokens("2026-08-27T00:00:05Z"))
	lines = append(lines, codexTurn("t3", "2026-08-27T00:00:06Z", "implement the fallback")...)
	lines = append(lines, codexPatch("c3", "2026-08-27T00:00:07Z", target), codexTokens("2026-08-27T00:00:08Z"))
	writeSource(t, filepath.Join(home, ".codex", "sessions", "spanning.jsonl"), lines...)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var turns int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(DISTINCT turn_index) FROM usage_tool_calls WHERE session_id='s'`).Scan(&turns); err != nil {
		t.Fatal(err)
	}
	if turns != 3 {
		t.Fatalf("tool calls span %d turns, want 3 — a same-turn fixture would not distinguish the two windows", turns)
	}
	metrics := metricsOf(t, service, wideScope())
	wantInt(t, "retries", metrics.Retries, 1)
}

// A pending row is a turn that does not exist yet. It must not be counted as a
// session, which would deflate `edits_per_session` by widening its denominator
// with turns that have produced nothing.
func TestPendingTurnsAreInvisibleToWorkflowMetrics(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	source := filepath.Join(home, ".claude", "projects", "pending.jsonl")
	target := filepath.Join(root, "target.go")

	writeSource(t, source, claudeUser("s", "2026-08-27T00:00:00Z", "add the retry loop"))
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	state, _, _, _, _ := signalRow(t, database, "claude", "s", 1)
	if state != signalStatePending {
		t.Fatalf("state = %q, want pending — the fixture must stop before the assistant entry", state)
	}
	pending := metricsOf(t, service, wideScope())
	if pending.EditsPerSession != nil {
		t.Fatalf("edits_per_session = %v over a pending-only scope, want unavailable — no session exists yet", pending.EditsPerSession)
	}

	appendSource(t, source, claudeAssistant("s", "m1", "2026-08-27T00:00:04Z", claudeEdit("t1", target)))
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	promoted := metricsOf(t, service, wideScope())
	if promoted.EditsPerSession == nil || *promoted.EditsPerSession != 1 {
		t.Fatalf("edits_per_session = %v after promotion, want 1", promoted.EditsPerSession)
	}
	wantInt(t, "first_edit_seconds", promoted.FirstEditSeconds, 4)
}

// The median is taken across sessions, and a session that never edited
// contributes to the denominator of `edits_per_session` without contributing a
// first-edit delay at all.
func TestFirstEditSecondsIsTheMedianAcrossSessions(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, _, home := newSignalService(t, root)
	target := filepath.Join(root, "target.go")

	for _, session := range []struct {
		name  string
		edits string
	}{
		{"fast", "2026-08-27T00:00:02Z"},
		{"middle", "2026-08-27T00:00:10Z"},
		{"slow", "2026-08-27T00:00:30Z"},
	} {
		writeSource(t, filepath.Join(home, ".claude", "projects", session.name+".jsonl"),
			claudeUser(session.name, "2026-08-27T00:00:00Z", "add the retry loop"),
			claudeAssistant(session.name, "m-"+session.name, session.edits, claudeEdit("t-"+session.name, target)),
		)
	}
	writeSource(t, filepath.Join(home, ".claude", "projects", "chat.jsonl"),
		claudeUser("chat", "2026-08-27T00:00:00Z", "what do you think of this approach"),
		claudeAssistant("chat", "m-chat", "2026-08-27T00:00:01Z", `{"type":"text","text":"it holds"}`),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	metrics := metricsOf(t, service, wideScope())
	wantInt(t, "first_edit_seconds", metrics.FirstEditSeconds, 10)
	if metrics.EditsPerSession == nil || *metrics.EditsPerSession != 0.75 {
		t.Fatalf("edits_per_session = %v, want 0.75 — three written rows over four sessions", metrics.EditsPerSession)
	}

	// A fourth editing session makes the count even, where the median falls
	// between two samples. 10 and 11 average to 10.5, and the metric is a whole
	// number of seconds, so it rounds up rather than truncating toward the
	// faster session.
	writeSource(t, filepath.Join(home, ".claude", "projects", "fourth.jsonl"),
		claudeUser("fourth", "2026-08-27T00:00:00Z", "add the retry loop"),
		claudeAssistant("fourth", "m-fourth", "2026-08-27T00:00:11Z", claudeEdit("t-fourth", target)),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	wantInt(t, "first_edit_seconds", metricsOf(t, service, wideScope()).FirstEditSeconds, 11)
}

// The read-shaped exclusion is not only about shell commands: a Claude `Read`
// between two edits is research by the same reasoning, and it lands as the same
// `read` kind the ported allowlist assigns a read-only Codex command.
func TestReworkTreatsAClaudeReadAsResearch(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, _, home := newSignalService(t, root)
	target := filepath.Join(root, "target.go")

	writeSource(t, filepath.Join(home, ".claude", "projects", "research.jsonl"),
		claudeUser("s", "2026-08-27T00:00:00Z", "add the retry loop"),
		claudeAssistant("s", "m1", "2026-08-27T00:00:01Z", claudeEdit("t1", target)),
		claudeAssistant("s", "m2", "2026-08-27T00:00:02Z", `{"type":"tool_use","id":"t2","name":"Read","input":{"file_path":"`+target+`"}}`),
		claudeAssistant("s", "m3", "2026-08-27T00:00:03Z", claudeEdit("t3", target)),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	metrics := metricsOf(t, service, wideScope())
	wantInt(t, "retries", metrics.Retries, 0)
	wantInt(t, "top_file_edits", metrics.TopFileEdits, 2)
}

// Claude keeps Bash in the Tooling `bash` bucket, but Decision 6 still excludes
// a read-shaped Bash command from the rework barrier. This must exercise the
// parser reduction, not a synthetic usage_tool_calls row already labelled read.
func TestReworkExcludesReadShapedClaudeBashCommand(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	target := filepath.Join(root, "target.go")

	writeSource(t, filepath.Join(home, ".claude", "projects", "bash-research.jsonl"),
		claudeUser("s", "2026-08-27T00:00:00Z", "add the retry loop"),
		claudeAssistant("s", "m1", "2026-08-27T00:00:01Z", claudeEdit("t1", target)),
		claudeAssistant("s", "m2", "2026-08-27T00:00:02Z", claudeBash("t2", "rg TODO "+target)),
		claudeAssistant("s", "m3", "2026-08-27T00:00:03Z", claudeEdit("t3", target)),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	metrics := metricsOf(t, service, wideScope())
	wantInt(t, "retries", metrics.Retries, 0)
	var commandRead int
	if err := database.DB.QueryRowContext(ctx, `SELECT command_read FROM usage_tool_calls WHERE tool_name='Bash'`).Scan(&commandRead); err != nil {
		t.Fatal(err)
	}
	if commandRead != 1 {
		t.Fatalf("command_read = %d, want 1", commandRead)
	}
}

func TestFirstEditRecognizesAClaudeBashWrite(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, _, home := newSignalService(t, root)
	target := filepath.Join(root, "generated.go")

	writeSource(t, filepath.Join(home, ".claude", "projects", "bash-write.jsonl"),
		claudeUser("s", "2026-08-27T00:00:00Z", "add generated output"),
		claudeAssistant("s", "m1", "2026-08-27T00:00:05Z", claudeBash("t1", "printf x > "+target)),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	metrics := metricsOf(t, service, wideScope())
	wantInt(t, "first_edit_seconds", metrics.FirstEditSeconds, 5)
	wantInt(t, "files_touched", metrics.FilesTouched, 1)
}

// Two files edited the same number of times must resolve to the same answer on
// every run. Without a total order the winner would follow Go's map iteration
// and the displayed file would change between two reads of an unchanged store.
func TestTopFileBreaksTiesDeterministically(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, _, home := newSignalService(t, root)
	alpha := filepath.Join(root, "alpha.go")
	beta := filepath.Join(root, "beta.go")

	writeSource(t, filepath.Join(home, ".claude", "projects", "tie.jsonl"),
		claudeUser("s", "2026-08-27T00:00:00Z", "add the retry loop"),
		claudeAssistant("s", "m1", "2026-08-27T00:00:01Z", claudeEdit("t1", alpha)),
		claudeAssistant("s", "m2", "2026-08-27T00:00:02Z", claudeEdit("t2", beta)),
		claudeAssistant("s", "m3", "2026-08-27T00:00:03Z", claudeEdit("t3", alpha)),
		claudeAssistant("s", "m4", "2026-08-27T00:00:04Z", claudeEdit("t4", beta)),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 20; round++ {
		metrics := metricsOf(t, service, wideScope())
		wantInt(t, "top_file_edits", metrics.TopFileEdits, 2)
		if metrics.TopFile == nil || *metrics.TopFile != "alpha.go" {
			t.Fatalf("round %d: top_file = %v, want alpha.go — the tie must break on the base name", round, metrics.TopFile)
		}
		wantInt(t, "files_touched", metrics.FilesTouched, 2)
	}
}

// The scope is a client and a period, and a turn outside either one contributes
// nothing. Both bounds are asserted because they are applied by the same query
// and a missing one would be invisible in a fixture that exercised only the
// other.
func TestWorkflowMetricsScopeIsBoundedByPeriodAndClient(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, _, home := newSignalService(t, root)
	claudeTarget := filepath.Join(root, "claude-target.go")
	codexTarget := filepath.Join(root, "codex-target.go")

	writeSource(t, filepath.Join(home, ".claude", "projects", "claude.jsonl"),
		claudeUser("s", "2026-08-27T00:00:00Z", "add the retry loop"),
		claudeAssistant("s", "m1", "2026-08-27T00:00:01Z", claudeEdit("t1", claudeTarget)),
	)
	codex := []string{`{"type":"session_meta","payload":{"session_id":"cx"}}`}
	codex = append(codex, codexTurn("t1", "2026-08-27T01:00:00Z", "implement the parser")...)
	codex = append(codex, codexPatch("c1", "2026-08-27T01:00:01Z", codexTarget), codexTokens("2026-08-27T01:00:02Z"))
	writeSource(t, filepath.Join(home, ".codex", "sessions", "codex.jsonl"), codex...)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	both := metricsOf(t, service, wideScope())
	wantInt(t, "files_touched over both clients", both.FilesTouched, 2)

	claudeOnly := wideScope()
	claudeOnly.Client = "claude"
	claudeMetrics := metricsOf(t, service, claudeOnly)
	wantInt(t, "claude files_touched", claudeMetrics.FilesTouched, 1)
	if claudeMetrics.TopFile == nil || *claudeMetrics.TopFile != "claude-target.go" {
		t.Fatalf("claude top_file = %v, want the file only that client wrote", claudeMetrics.TopFile)
	}

	before := SignalScope{
		From: time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC),
	}
	if metrics := metricsOf(t, service, before); metrics.EditsPerSession != nil || metrics.FilesTouched != nil {
		t.Fatalf("a period ending before every turn reported values: %+v", metrics)
	}
}
