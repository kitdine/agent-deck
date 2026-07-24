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
