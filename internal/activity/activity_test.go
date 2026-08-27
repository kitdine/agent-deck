package activity

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParserExtractsOnlySafeCodexToolMetadata(t *testing.T) {
	parser := NewParser("codex", "fixture.jsonl")
	records := parseActivityFixture(t, parser, []string{
		`{"type":"session_meta","payload":{"session_id":"codex-session"}}`,
		`{"type":"turn_context","payload":{"turn_id":"turn-1","model":"gpt-5.4"}}`,
		`{"type":"response_item","timestamp":"2026-07-20T00:00:01Z","payload":{"item":{"type":"function_call","call_id":"call-1","name":"exec_command","arguments":"credential=secret"}}}`,
		`{"type":"response_item","timestamp":"2026-07-20T00:00:03Z","payload":{"item":{"type":"function_call_output","call_id":"call-1","output":"private file contents"}}}`,
	})
	details := Merge(records)
	if len(details) != 1 {
		t.Fatalf("details = %#v", details)
	}
	got := details[0]
	if got.Client != "codex" || got.SessionID != "codex-session" || got.Model != "gpt-5.4" || got.Tool != "exec_command" || got.Status != "completed" || got.DurationMS == nil || *got.DurationMS != 2000 {
		t.Fatalf("detail = %#v", got)
	}
	assertNoSensitiveActivityContent(t, details)
}

func TestParserExtractsOnlySafeClaudeToolMetadata(t *testing.T) {
	parser := NewParser("claude", "fixture.jsonl")
	records := parseActivityFixture(t, parser, []string{
		`{"type":"assistant","timestamp":"2026-07-20T00:00:01Z","sessionId":"claude-session","message":{"model":"claude-opus-4-1","content":[{"type":"tool_use","id":"tool-1","name":"Read","input":{"path":"/private/secret"}}]}}`,
		`{"type":"user","timestamp":"2026-07-20T00:00:04Z","sessionId":"claude-session","message":{"content":[{"type":"tool_result","tool_use_id":"tool-1","is_error":true,"content":"credential"}]}}`,
	})
	details := Merge(records)
	if len(details) != 1 {
		t.Fatalf("details = %#v", details)
	}
	got := details[0]
	if got.Client != "claude" || got.SessionID != "claude-session" || got.Model != "claude-opus-4-1" || got.Tool != "Read" || got.Status != "failed" || got.DurationMS == nil || *got.DurationMS != 3000 {
		t.Fatalf("detail = %#v", got)
	}
	assertNoSensitiveActivityContent(t, details)
}

func TestParserExtractsCodexWorkSignalMetadata(t *testing.T) {
	root := t.TempDir()
	patchPath := filepath.Join(root, "private", "patched.go")
	commandPath := filepath.Join(root, "private", "command.go")
	execPath := filepath.Join(root, "private", "exec.go")
	parser := NewParser("codex", "fixture.jsonl")
	parser.SetMachineIdentity("machine-a")
	values := []map[string]any{
		{"type": "session_meta", "payload": map[string]any{"session_id": "session"}},
		{"type": "turn_context", "payload": map[string]any{"turn_id": "turn-1", "model": "gpt-5.6"}},
		{"type": "response_item", "timestamp": "2026-08-27T00:00:01Z", "payload": map[string]any{"item": map[string]any{"type": "custom_tool_call", "call_id": "patch", "name": "apply_patch", "input": "*** Begin Patch\n*** Update File: " + patchPath + "\n*** End Patch"}}},
		{"type": "turn_context", "payload": map[string]any{"turn_id": "turn-2", "model": "gpt-5.6"}},
		{"type": "response_item", "timestamp": "2026-08-27T00:00:02Z", "payload": map[string]any{"item": map[string]any{"type": "function_call", "call_id": "command", "name": "exec_command", "arguments": `{"cmd":"cat ` + commandPath + ` && sed -i '' ` + commandPath + `","workdir":"` + root + `"}`}}},
		{"type": "turn_context", "payload": map[string]any{"turn_id": "turn-3", "model": "gpt-5.6"}},
		{"type": "response_item", "timestamp": "2026-08-27T00:00:03Z", "payload": map[string]any{"item": map[string]any{"type": "custom_tool_call", "call_id": "exec", "name": "exec", "input": "await Promise.all([tools.exec_command({cmd: 'cat " + execPath + " # )'}), tools.exec_command({cmd: `head -1 " + execPath + "`})])"}}},
		{"type": "turn_context", "payload": map[string]any{"turn_id": "turn-4", "model": "gpt-5.6"}},
		{"type": "response_item", "timestamp": "2026-08-27T00:00:04Z", "payload": map[string]any{"item": map[string]any{"type": "mcp_tool_call", "id": "mcp", "server": "codegraph", "tool": "explore"}}},
	}
	allRecords := parseActivityValues(parser, values)
	var records []Record
	for _, record := range allRecords {
		if record.StartedAt != "" {
			records = append(records, record)
		}
	}
	if len(records) != 4 {
		t.Fatalf("records = %#v", records)
	}
	for index, wantKind := range []string{"edit", "edit", "read", "mcp"} {
		if records[index].TurnIndex != index+1 || records[index].ToolKind != wantKind {
			t.Fatalf("record[%d] = %#v, want turn=%d kind=%s", index, records[index], index+1, wantKind)
		}
	}
	if records[3].MCPServer != "codegraph" {
		t.Fatalf("mcp record = %#v", records[3])
	}
	for index, wantBase := range []string{"patched.go", "command.go", "exec.go"} {
		if len(records[index].Files) != 1 || records[index].Files[0].BaseName != wantBase || records[index].Files[0].PathDigest == "" {
			t.Fatalf("record[%d] files = %#v, want %s", index, records[index].Files, wantBase)
		}
		if index < 2 && !records[index].Files[0].Wrote {
			t.Fatalf("record[%d] file = %#v, want write", index, records[index].Files[0])
		}
		if index == 2 && records[index].Files[0].Wrote {
			t.Fatalf("exec read file = %#v", records[index].Files[0])
		}
	}

	other := NewParser("codex", "fixture.jsonl")
	other.SetMachineIdentity("machine-b")
	otherRecords := parseActivityValues(other, values[:3])
	if len(otherRecords) != 1 || otherRecords[0].Files[0].PathDigest == records[0].Files[0].PathDigest {
		t.Fatalf("machine-salted digests = %q and %q", records[0].Files[0].PathDigest, otherRecords[0].Files[0].PathDigest)
	}
}

func TestParserSegmentsClaudeTurnsWithoutSyntheticOrToolResultBoundaries(t *testing.T) {
	root := t.TempDir()
	editPath := filepath.Join(root, "edit.go")
	readPath := filepath.Join(root, "read.go")
	parser := NewParser("claude", "fixture.jsonl")
	parser.SetMachineIdentity("machine-a")
	values := []map[string]any{
		{"type": "user", "sessionId": "session", "message": map[string]any{"role": "user", "content": "implement it"}},
		{"type": "assistant", "timestamp": "2026-08-27T00:00:01Z", "sessionId": "session", "message": map[string]any{"role": "assistant", "model": "claude-opus", "content": []any{map[string]any{"type": "tool_use", "id": "edit", "name": "Edit", "input": map[string]any{"file_path": editPath}}}}},
		{"type": "user", "timestamp": "2026-08-27T00:00:02Z", "sessionId": "session", "message": map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "edit", "content": "private-result"}}}},
		{"type": "user", "isMeta": true, "sessionId": "session", "message": map[string]any{"role": "user", "content": "Base directory for this skill: /private/skill"}},
		{"type": "assistant", "timestamp": "2026-08-27T00:00:03Z", "sessionId": "session", "message": map[string]any{"role": "assistant", "model": "claude-opus", "content": []any{map[string]any{"type": "tool_use", "id": "mcp", "name": "mcp__neo4j__read", "input": map[string]any{}}}}},
		{"type": "user", "sessionId": "session", "message": map[string]any{"role": "user", "content": "abandoned message"}},
		{"type": "user", "sessionId": "session", "message": map[string]any{"role": "user", "content": "inspect it"}},
		{"type": "assistant", "timestamp": "2026-08-27T00:00:04Z", "sessionId": "session", "message": map[string]any{"role": "assistant", "model": "claude-opus", "content": []any{map[string]any{"type": "tool_use", "id": "read", "name": "Read", "input": map[string]any{"file_path": readPath}}}}},
	}
	allRecords := parseActivityValues(parser, values)
	var records []Record
	for _, record := range allRecords {
		if record.StartedAt != "" {
			records = append(records, record)
		}
	}
	if len(records) != 3 {
		t.Fatalf("records = %#v", records)
	}
	if records[0].TurnIndex != 1 || records[0].ToolKind != "edit" || len(records[0].Files) != 1 || !records[0].Files[0].Wrote {
		t.Fatalf("edit record = %#v", records[0])
	}
	if records[1].TurnIndex != 1 || records[1].ToolKind != "mcp" || records[1].MCPServer != "neo4j" {
		t.Fatalf("synthetic-boundary MCP record = %#v", records[1])
	}
	if records[2].TurnIndex != 2 || records[2].ToolKind != "read" || len(records[2].Files) != 1 || records[2].Files[0].Wrote {
		t.Fatalf("read record = %#v", records[2])
	}
}

func TestReadDetailsToleratesMalformedAndTruncatedRecords(t *testing.T) {
	const (
		requestArguments = "requested-arguments-secret"
		requestContent   = "requested-content-secret"
		malformedSecret  = "malformed-record-secret"
		requestOutput    = "requested-output-secret"
		otherArguments   = "other-arguments-secret"
		otherOutput      = "other-output-secret"
		truncatedArgs    = "truncated-call-arguments-secret"
		truncatedOutput  = "truncated-completion-secret"
	)
	path := writeActivityJSONL(t, []string{
		`{"type":"session_meta","payload":{"session_id":"requested-session"}}`,
		`{"type":"turn_context","payload":{"turn_id":"requested-turn","model":"gpt-safe"}}`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:01Z","payload":{"item":{"type":"function_call","call_id":"requested-call","name":"exec_command","arguments":"` + requestArguments + `"}}}`,
		`{"type":"response_item","payload":{"arguments":"` + malformedSecret + `"`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:03Z","payload":{"item":{"type":"function_call_output","call_id":"requested-call","output":"` + requestOutput + `","content":"` + requestContent + `"}}}`,
		`{"type":"session_meta","payload":{"session_id":"other-session"}}`,
		`{"type":"turn_context","payload":{"turn_id":"other-turn","model":"gpt-other"}}`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:04Z","payload":{"item":{"type":"function_call","call_id":"other-call","name":"other_tool","arguments":"` + otherArguments + `"}}}`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:05Z","payload":{"item":{"type":"function_call_output","call_id":"other-call","output":"` + otherOutput + `"}}}`,
		`{"type":"session_meta","payload":{"session_id":"requested-session"}}`,
		`{"type":"turn_context","payload":{"turn_id":"requested-turn-2","model":"gpt-safe"}}`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:06Z","payload":{"item":{"type":"function_call","call_id":"truncated-call","name":"read_file","arguments":"` + truncatedArgs + `"}}}`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:09Z","payload":{"item":{"type":"function_call_output","call_id":"truncated-call","output":"` + truncatedOutput + `"}`,
	})

	details, err := ReadDetails(path, "codex", "requested-session")
	if err != nil {
		t.Fatal(err)
	}
	if len(details) != 2 {
		t.Fatalf("details = %#v, want completed and incomplete requested-session calls only", details)
	}
	completed := details[0]
	if completed.Client != "codex" || completed.SessionID != "requested-session" || completed.Model != "gpt-safe" || completed.Tool != "exec_command" || completed.StartedAt != "2026-07-23T00:00:01Z" || completed.CompletedAt != "2026-07-23T00:00:03Z" || completed.Status != "completed" || completed.DurationMS == nil || *completed.DurationMS != 2000 {
		t.Fatalf("completed detail = %#v", completed)
	}
	incomplete := details[1]
	if incomplete.Client != "codex" || incomplete.SessionID != "requested-session" || incomplete.Model != "gpt-safe" || incomplete.Tool != "read_file" || incomplete.StartedAt != "2026-07-23T00:00:06Z" || incomplete.CompletedAt != "" || incomplete.Status != "started" || incomplete.DurationMS != nil {
		t.Fatalf("incomplete detail = %#v, want no invented completion or duration", incomplete)
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{requestArguments, requestContent, malformedSecret, requestOutput, otherArguments, otherOutput, truncatedArgs, truncatedOutput, "arguments", "output", "content"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("ReadDetails leaked %q: %s", secret, encoded)
		}
	}
}

func TestReadDetailsWrapsOversizedScannerTokenWithoutPartialSuccess(t *testing.T) {
	const (
		prefixRecordSecret = "partial-arguments-secret"
		oversizedSecret    = "oversized-scanner-secret"
	)
	oversized := strings.Repeat(oversizedSecret, 1+(8<<20)/len(oversizedSecret))
	path := writeActivityJSONL(t, []string{
		`{"type":"session_meta","payload":{"session_id":"oversized-session"}}`,
		`{"type":"turn_context","payload":{"turn_id":"oversized-turn","model":"gpt-safe"}}`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:01Z","payload":{"item":{"type":"function_call","call_id":"partial-call","name":"exec_command","arguments":"` + prefixRecordSecret + `"}}}`,
		oversized,
	})

	details, err := ReadDetails(path, "codex", "oversized-session")
	if err == nil {
		t.Fatalf("ReadDetails returned partial success: %#v", details)
	}
	if details != nil {
		t.Fatalf("details = %#v, want nil on scanner failure", details)
	}
	if !strings.Contains(err.Error(), "read session activity:") {
		t.Fatalf("error = %q, want read session activity classification", err)
	}
	if errors.Unwrap(err) == nil {
		t.Fatalf("error = %T %v, want wrapped scanner cause", err, err)
	}
	for _, secret := range []string{prefixRecordSecret, oversizedSecret} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("scanner error leaked %q: %v", secret, err)
		}
	}
}

func TestReadDetailsPropagatesMissingSourceError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-session.jsonl")
	details, err := ReadDetails(path, "codex", "missing-session")
	if err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %T %v, want missing-source open error", err, err)
	}
	if details != nil {
		t.Fatalf("details = %#v, want nil for missing source", details)
	}
}

func TestReadDetailsPageKeepsOnlyRequestedPageAndCompleteSummary(t *testing.T) {
	path := writeActivityJSONL(t, []string{
		`{"type":"session_meta","payload":{"session_id":"paged-session"}}`,
		`{"type":"turn_context","payload":{"turn_id":"paged-turn","model":"gpt-safe"}}`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:01Z","payload":{"item":{"type":"function_call","call_id":"call-1","name":"first","arguments":"secret-1"}}}`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:02Z","payload":{"item":{"type":"function_call_output","call_id":"call-1","output":"secret-2"}}}`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:03Z","payload":{"item":{"type":"function_call","call_id":"call-2","name":"second","arguments":"secret-3"}}}`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:04Z","payload":{"item":{"type":"function_call_output","call_id":"call-2","output":"secret-4"}}}`,
		`{"type":"response_item","timestamp":"2026-07-23T00:00:05Z","payload":{"item":{"type":"function_call","call_id":"call-3","name":"third","arguments":"secret-5"}}}`,
	})

	result, err := ReadDetailsPage(path, "codex", "paged-session", 2, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 3 || result.Shown != 1 || !result.HasMore || result.NextPage != 3 {
		t.Fatalf("pagination = %#v", result)
	}
	if len(result.Details) != 1 || result.Details[0].Tool != "second" || result.Details[0].Status != "completed" {
		t.Fatalf("details = %#v", result.Details)
	}
	if result.Summary.Total != 3 || result.Summary.Completed != 2 || result.Summary.Incomplete != 1 || result.Summary.ByTool["first"] != 1 || result.Summary.ByTool["third"] != 1 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	assertNoSensitiveActivityContent(t, result.Details)
}

func TestReadDetailsPageRejectsDeepPageBeforeOpeningSource(t *testing.T) {
	_, err := ReadDetailsPage(filepath.Join(t.TempDir(), "missing.jsonl"), "codex", "session", maxPageCandidates+1, 1, false)
	if err == nil || !strings.Contains(err.Error(), "bounded window") {
		t.Fatalf("error = %v, want bounded page rejection", err)
	}
}

func writeActivityJSONL(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func parseActivityFixture(t *testing.T, parser *Parser, lines []string) []Record {
	t.Helper()
	var records []Record
	for index, line := range lines {
		var value map[string]any
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			t.Fatal(err)
		}
		records = append(records, parser.Parse(value, int64(index))...)
	}
	return records
}

func parseActivityValues(parser *Parser, values []map[string]any) []Record {
	var records []Record
	for index, value := range values {
		records = append(records, parser.Parse(value, int64(index))...)
	}
	return records
}

func assertNoSensitiveActivityContent(t *testing.T, details []Detail) {
	t.Helper()
	encoded, err := json.Marshal(details)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"credential", "private file contents", "/private/secret", "arguments", "output", "result", "input"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("activity metadata leaked %q: %s", secret, encoded)
		}
	}
}
