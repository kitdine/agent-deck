package activity

import (
	"encoding/json"
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
