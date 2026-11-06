package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
)

type originCLIResult struct {
	exit   int
	stdout string
	stderr string
}

func runOriginCLI(t *testing.T, state string, args ...string) originCLIResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exit := execute(append([]string{"--state-dir", state}, args...), bytes.NewReader(nil), &stdout, &stderr)
	return originCLIResult{exit: exit, stdout: stdout.String(), stderr: stderr.String()}
}

func decodeOriginJSON(t *testing.T, encoded string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(encoded), &value); err != nil {
		t.Fatalf("decode %q: %v", encoded, err)
	}
	return value
}

// seedOriginSessions builds an isolated state with a core and a session
// index: work-a and aux-b are claude sessions, and shared-id exists for both
// clients. Every session starts unknown/origin_metadata_missing.
func seedOriginSessions(t *testing.T) string {
	t.Helper()
	state := filepath.Join(t.TempDir(), "state")
	ctx := context.Background()
	core, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	index, err := store.OpenSessions(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	for i, item := range []struct{ client, id, text string }{
		{"claude", "work-a", "needle work"},
		{"claude", "aux-b", "needle maintenance"},
		{"claude", "shared-id", "needle shared claude"},
		{"codex", "shared-id", "needle shared codex"},
	} {
		at := fmt.Sprintf("2026-11-0%dT08:00:00Z", i+1)
		if err := session.ReplaceDocuments(ctx, index.DB, item.client, item.id, []session.Document{{Client: item.client, SessionID: item.id, EventAt: at, Kind: "user_prompt", Text: item.text}}); err != nil {
			t.Fatal(err)
		}
	}
	return state
}

func mustOriginOK(t *testing.T, result originCLIResult) map[string]any {
	t.Helper()
	if result.exit != 0 {
		t.Fatalf("exit %d: stdout=%q stderr=%q", result.exit, result.stdout, result.stderr)
	}
	return decodeOriginJSON(t, result.stdout)
}

func errorEnvelope(t *testing.T, result originCLIResult, exit int, code string) map[string]any {
	t.Helper()
	if result.exit != exit || result.stdout != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q, want exit %d and empty stdout", result.exit, result.stdout, result.stderr, exit)
	}
	envelope := decodeOriginJSON(t, result.stderr)
	errorValue, _ := envelope["error"].(map[string]any)
	if errorValue["code"] != code {
		t.Fatalf("error = %#v, want code %s", envelope["error"], code)
	}
	return errorValue
}

func sessionKeys(t *testing.T, data any) []string {
	t.Helper()
	items, ok := data.([]any)
	if !ok {
		t.Fatalf("data is not an array: %#v", data)
	}
	keys := make([]string, 0, len(items))
	for _, item := range items {
		value := item.(map[string]any)
		keys = append(keys, value["client"].(string)+"/"+value["session_id"].(string))
	}
	sort.Strings(keys)
	return keys
}

func TestSessionScopeFlagRejectsInvalidAndConflictingValuesBeforeReading(t *testing.T) {
	state := filepath.Join(t.TempDir(), "missing-state")
	for _, args := range [][]string{
		{"--format", "json", "session", "list", "--session-scope", "unknown"},
		{"--format", "json", "session", "list", "--session-scope", "work", "--session-scope", "all"},
		{"--format", "json", "usage", "signals", "--session-scope", "Work"},
		{"--format", "json", "session", "show", "id", "--session-scope", "all"},
		{"--format", "json", "usage", "summary", "--session-scope", "all"},
	} {
		result := runOriginCLI(t, state, args...)
		errorEnvelope(t, result, 2, "invalid_argument")
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("rejected input created state: %v", err)
	}
	var flag sessionScopeFlag
	if err := flag.Set("auxiliary"); err != nil {
		t.Fatal(err)
	}
	if err := flag.Set("auxiliary"); err != nil {
		t.Fatalf("repeating the same value was rejected: %v", err)
	}
}

func TestSessionListScopesDefaultsAndJSONCompatibility(t *testing.T) {
	state := seedOriginSessions(t)
	set := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "aux-b", "--client", "claude", "--set", "auxiliary"))
	if data := set["data"].(map[string]any); data["outcome"] != "saved" || data["revision"] != float64(1) || data["publication_pending"] != true {
		t.Fatalf("set receipt = %#v", data)
	}

	legacy := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "list"))
	if got := sessionKeys(t, legacy["data"]); !reflect.DeepEqual(got, []string{"claude/aux-b", "claude/shared-id", "claude/work-a", "codex/shared-id"}) {
		t.Fatalf("JSON default (all) = %v", got)
	}
	selection := legacy["session_selection"].(map[string]any)
	if selection["requested_scope"] != nil || selection["effective_scope"] != "all" || selection["source_default"] != "json_compat" || selection["availability"] != "available" || selection["unknown_count"] != float64(3) {
		t.Fatalf("JSON default session_selection = %#v", selection)
	}
	for _, field := range []string{"read_vector", "publication_id"} {
		if selection[field] == nil {
			t.Fatalf("session_selection missing %s: %#v", field, selection)
		}
	}
	for _, item := range legacy["data"].([]any) {
		value := item.(map[string]any)
		for _, field := range []string{"client", "session_id", "project", "source_path", "model", "first_at", "last_at", "origin"} {
			if _, ok := value[field]; !ok {
				t.Fatalf("list element lost %s: %#v", field, value)
			}
		}
	}

	work := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "list", "--session-scope", "work"))
	if got := sessionKeys(t, work["data"]); !reflect.DeepEqual(got, []string{"claude/shared-id", "claude/work-a", "codex/shared-id"}) {
		t.Fatalf("work = %v", got)
	}
	if selection := work["session_selection"].(map[string]any); selection["requested_scope"] != "work" || selection["source_default"] != "explicit" || selection["unknown_count"] != float64(3) {
		t.Fatalf("work selection = %#v", selection)
	}
	auxiliary := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "list", "--session-scope", "auxiliary", "--limit", "1"))
	page := auxiliary["data"].(map[string]any)
	pagination := page["pagination"].(map[string]any)["sessions"].(map[string]any)
	if pagination["total"] != float64(1) || len(page["sessions"].([]any)) != 1 || auxiliary["session_selection"].(map[string]any)["unknown_count"] != float64(0) {
		t.Fatalf("auxiliary page = %#v", auxiliary)
	}
	allPaged := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "list", "--session-scope", "work", "--all"))
	if total := allPaged["data"].(map[string]any)["pagination"].(map[string]any)["sessions"].(map[string]any)["total"]; total != float64(3) {
		t.Fatalf("--all must only cancel paging; work total = %v", total)
	}

	text := runOriginCLI(t, state, "--no-color", "session", "list")
	if text.exit != 0 || !strings.Contains(text.stdout, "Scope: work · 3 sessions · 3 unknown") || strings.Contains(text.stdout, "aux-b") || !strings.Contains(text.stdout, "UNKNOWN") || !strings.Contains(text.stdout, "CLASS") {
		t.Fatalf("text default work = %#v", text)
	}
	auxText := runOriginCLI(t, state, "--no-color", "session", "list", "--session-scope", "auxiliary")
	if !strings.Contains(auxText.stdout, "AUXILIARY") || !strings.Contains(auxText.stdout, "aux-b") || strings.Contains(auxText.stdout, "work-a") {
		t.Fatalf("auxiliary text = %q", auxText.stdout)
	}
	next := runOriginCLI(t, state, "--no-color", "session", "list", "--session-scope", "work", "--limit", "1")
	if !strings.Contains(next.stdout, "Next page: agentdeck --state-dir") || !strings.Contains(next.stdout, "--session-scope work --page 2 --limit 1") {
		t.Fatalf("next page lost the explicit scope: %q", next.stdout)
	}

	// show takes an exact id and has no scope: an auxiliary session stays visible.
	shown := runOriginCLI(t, state, "--format", "json", "session", "show", "aux-b", "--client", "claude")
	if shown.exit != 0 {
		t.Fatalf("show auxiliary session = %#v", shown)
	}
}

func TestSessionListOracleMatchesSelectionServiceForEveryScope(t *testing.T) {
	state := seedOriginSessions(t)
	mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude", "--set", "auxiliary"))
	ctx := context.Background()
	core, err := store.OpenReadOnly(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	index, err := store.OpenSessionsReadOnly(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	reader := sessionorigin.Reader{Core: core, Sessions: index, StateRoot: state}
	for _, scope := range []string{"work", "auxiliary", "all"} {
		for _, client := range []string{"", "claude", "codex"} {
			snapshot, err := reader.Select(ctx, sessionorigin.Selection{Scope: scope, Client: client}, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{}
			for _, item := range snapshot.Sessions {
				want = append(want, item.Client+"/"+item.SessionID)
			}
			sort.Strings(want)
			args := []string{"--format", "json", "session", "list", "--session-scope", scope}
			if client != "" {
				args = append(args, "--client", client)
			}
			envelope := mustOriginOK(t, runOriginCLI(t, state, args...))
			if got := sessionKeys(t, envelope["data"]); !reflect.DeepEqual(got, want) {
				t.Fatalf("scope %s client %q: CLI %v, service %v", scope, client, got, want)
			}
			if unknown := envelope["session_selection"].(map[string]any)["unknown_count"]; unknown != float64(snapshot.UnknownCount) {
				t.Fatalf("unknown_count %v, service %d", unknown, snapshot.UnknownCount)
			}
		}
	}
}

func TestSessionSearchSeparatesDocumentTotalFromSelectedSessions(t *testing.T) {
	state := seedOriginSessions(t)
	ctx := context.Background()
	index, err := store.OpenSessions(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.ReplaceDocuments(ctx, index.DB, "claude", "work-a", []session.Document{
		{Client: "claude", SessionID: "work-a", EventAt: "2026-11-01T08:00:00Z", Kind: "user_prompt", Text: "needle one"},
		{Client: "claude", SessionID: "work-a", EventAt: "2026-11-01T08:01:00Z", Kind: "user_prompt", Text: "needle two"},
	}); err != nil {
		t.Fatal(err)
	}
	index.Close()
	mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "aux-b", "--client", "claude", "--set", "auxiliary"))
	result := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "search", "needle", "--session-scope", "work", "--all"))
	page := result["data"].(map[string]any)
	total := page["pagination"].(map[string]any)["search"].(map[string]any)["total"]
	selected := result["session_selection"].(map[string]any)["selected_session_count"]
	if total != float64(4) || selected != float64(3) {
		t.Fatalf("documents total %v, selected sessions %v; want 4 and 3", total, selected)
	}
	legacy := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "search", "needle"))
	if items, ok := legacy["data"].([]any); !ok || len(items) != 5 {
		t.Fatalf("JSON default search keeps the all array: %#v", legacy["data"])
	}
	text := runOriginCLI(t, state, "--no-color", "session", "search", "needle")
	if !strings.Contains(text.stdout, "Scope: work · 4 matching documents in 3 sessions") || strings.Contains(text.stdout, "needle maintenance") {
		t.Fatalf("search text = %q", text.stdout)
	}
}

func TestSessionClassifyQuerySetAndConflictContracts(t *testing.T) {
	state := seedOriginSessions(t)
	query := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude"))
	data := query["data"].(map[string]any)
	origin := data["origin"].(map[string]any)
	if data["classification_available"] != true || origin["automatic"] != "unknown" || origin["effective"] != "unknown" || origin["reason_code"] != "origin_metadata_missing" || origin["revision"] != float64(0) || origin["override"] != nil {
		t.Fatalf("query = %#v", data)
	}
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "shared-id"), 2, "invalid_argument")
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "missing", "--client", "claude"), 1, "session_not_found")
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude", "--set", "unknown"), 2, "invalid_argument")
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude", "--set", "work", "--clear"), 2, "invalid_argument")
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--id", "work-a", "--client", "claude"), 2, "invalid_argument")
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "--request", "req-1", "--client", "claude"), 2, "invalid_argument")

	first := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude", "--set", "work", "--request-id", "req-set-1"))
	if receipt := first["data"].(map[string]any); receipt["outcome"] != "saved" || receipt["revision"] != float64(1) || receipt["request_id"] != "req-set-1" {
		t.Fatalf("first set = %#v", receipt)
	}
	replay := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude", "--set", "work", "--request-id", "req-set-1"))
	if receipt := replay["data"].(map[string]any); receipt["outcome"] != "saved" || receipt["revision"] != float64(1) || receipt["replayed"] != true {
		t.Fatalf("same request replay = %#v", receipt)
	}
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude", "--set", "auxiliary", "--request-id", "req-set-1"), 2, "invalid_request")
	same := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude", "--set", "work"))
	if receipt := same["data"].(map[string]any); receipt["outcome"] != "no_change" || receipt["revision"] != float64(1) || receipt["publication_pending"] != false {
		t.Fatalf("same value = %#v", receipt)
	}
	conflict := errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude", "--set", "auxiliary", "--expected-revision", "0"), 1, "origin_conflict")
	if details := conflict["details"].(map[string]any); details["current_revision"] != float64(1) || details["request_id"] == nil {
		t.Fatalf("conflict details = %#v", conflict)
	}
	after := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "work-a", "--client", "claude"))
	if origin := after["data"].(map[string]any)["origin"].(map[string]any); origin["effective"] != "work" || origin["automatic"] != "unknown" || origin["revision"] != float64(1) {
		t.Fatalf("override must keep automatic and apply effective: %#v", origin)
	}
	text := runOriginCLI(t, state, "--no-color", "session", "classify", "work-a", "--client", "claude", "--set", "auxiliary")
	for _, want := range []string{"CLASSIFICATION SAVED", "Request ID  req-", "Target      claude/work-a", "Outcome     saved", "Revision    2", "Views: pending publication", "Undo: agentdeck session classify --id 'work-a' --client 'claude' --clear"} {
		if !strings.Contains(text.stdout, want) {
			t.Fatalf("set text missing %q:\n%s", want, text.stdout)
		}
	}
	readable := runOriginCLI(t, state, "--no-color", "session", "classify", "work-a", "--client", "claude")
	for _, want := range []string{"Client     claude", "Effective  AUXILIARY", "Automatic  UNKNOWN", "Reason     origin_metadata_missing", "Rule       origin-reducer-v1", "Override   auxiliary (matched, binding limited)", "Revision   2"} {
		if !strings.Contains(readable.stdout, want) {
			t.Fatalf("query text missing %q:\n%s", want, readable.stdout)
		}
	}
}

func TestSessionClassifyClearReceiptListAndReadBack(t *testing.T) {
	state := seedOriginSessions(t)
	mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "aux-b", "--client", "claude", "--set", "auxiliary"))
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "aux-b", "--clear"), 2, "invalid_argument")

	// The index is gone: clear resolves only the core key and still withdraws it.
	if err := os.Remove(filepath.Join(state, "sessions.sqlite3")); err != nil {
		t.Fatal(err)
	}
	listed := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "list"))
	page := listed["data"].(map[string]any)
	if page["total"] != float64(1) || page["limit"] != float64(20) || len(page["items"].([]any)) != 1 {
		t.Fatalf("override list = %#v", page)
	}
	cleared := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "aux-b", "--client", "claude", "--clear", "--request-id", "req-clear-1"))
	receipt := cleared["data"].(map[string]any)
	if receipt["outcome"] != "saved" || receipt["revision"] != float64(2) || receipt["override"] != nil || receipt["automatic"] != nil || receipt["effective"] != nil ||
		receipt["classification_available"] != false || receipt["classification_reason"] != "not_queried" || receipt["publication_pending"] != true {
		t.Fatalf("clear receipt = %#v", receipt)
	}
	again := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "aux-b", "--client", "claude", "--clear"))
	if receipt := again["data"].(map[string]any); receipt["outcome"] != "no_change" || receipt["revision"] != float64(2) {
		t.Fatalf("tombstone clear = %#v", receipt)
	}
	never := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "never-seen", "--client", "codex", "--clear"))
	if receipt := never["data"].(map[string]any); receipt["outcome"] != "no_change" || receipt["revision"] != float64(0) {
		t.Fatalf("never-present clear = %#v", receipt)
	}
	if listed := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "list")); listed["data"].(map[string]any)["total"] != float64(0) {
		t.Fatalf("tombstone listed as an override: %#v", listed["data"])
	}
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "list", "--limit", "201"), 2, "invalid_argument")

	known := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "--request", "req-clear-1"))
	if receipt := known["data"].(map[string]any); receipt["outcome"] != "saved" || receipt["classification_reason"] != "not_queried" || receipt["automatic"] != nil || receipt["replayed"] != false {
		t.Fatalf("read back clear receipt = %#v", receipt)
	}
	unknown := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "--request", "req-never"))
	if data := unknown["data"].(map[string]any); data["outcome"] != "unknown" || data["request_id"] != "req-never" || len(data) != 2 {
		t.Fatalf("unknown read back = %#v", data)
	}
	text := runOriginCLI(t, state, "--no-color", "session", "classify", "--request", "req-clear-1")
	for _, want := range []string{"CLASSIFICATION CLEARED · RECORDED RECEIPT", "Automatic   unavailable (not queried)", "not necessarily the current state"} {
		if !strings.Contains(text.stdout, want) {
			t.Fatalf("read-back text missing %q:\n%s", want, text.stdout)
		}
	}
}

func TestSessionClassifyReservedIDUsesIDFlag(t *testing.T) {
	state := seedOriginSessions(t)
	ctx := context.Background()
	index, err := store.OpenSessions(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.ReplaceDocuments(ctx, index.DB, "codex", "list", []session.Document{{Client: "codex", SessionID: "list", EventAt: "2026-11-05T00:00:00Z", Kind: "user_prompt", Text: "x"}}); err != nil {
		t.Fatal(err)
	}
	index.Close()
	result := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "--id", "list", "--client", "codex", "--set", "work"))
	if target := result["data"].(map[string]any)["target"].(map[string]any); target["session_id"] != "list" {
		t.Fatalf("reserved id target = %#v", target)
	}
}

func TestSessionScopeDefaultGetSetAndReadFailure(t *testing.T) {
	state := seedOriginSessions(t)
	got := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "scope", "get"))
	if data := got["data"].(map[string]any); !reflect.DeepEqual(data, map[string]any{"confirmed": true, "value": "work", "revision": float64(0), "reason": nil}) {
		t.Fatalf("fresh default = %#v", data)
	}
	saved := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "scope", "set", "all"))
	if data := saved["data"].(map[string]any); data["outcome"] != "saved" || data["value"] != "all" || data["revision"] != float64(1) {
		t.Fatalf("scope set = %#v", data)
	}
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "scope", "set", "work", "--expected-revision", "0"), 1, "origin_conflict")
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "scope", "set", "everything"), 2, "invalid_argument")
	if data := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "scope", "get"))["data"].(map[string]any); data["value"] != "all" || data["revision"] != float64(1) {
		t.Fatalf("saved default = %#v", data)
	}
	// The CLI text default stays work regardless of the saved default.
	if text := runOriginCLI(t, state, "--no-color", "session", "list"); !strings.Contains(text.stdout, "Scope: work") {
		t.Fatalf("CLI default followed the desktop default: %q", text.stdout)
	}

	broken := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(broken, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "agentdeck.sqlite3"), []byte("not a database"), 0600); err != nil {
		t.Fatal(err)
	}
	failure := errorEnvelope(t, runOriginCLI(t, broken, "--format", "json", "session", "scope", "get"), 1, "runtime_error")
	if details := failure["details"].(map[string]any)["default_scope"].(map[string]any); !reflect.DeepEqual(details, map[string]any{"confirmed": false, "value": nil, "revision": nil, "reason": "read_failed"}) {
		t.Fatalf("read failure default = %#v", details)
	}
}

func originStatusSources(t *testing.T, result originCLIResult) ([]map[string]any, bool) {
	t.Helper()
	envelope := mustOriginOK(t, result)
	if envelope["command"] != "session.origin_source.status" {
		t.Fatalf("command = %v", envelope["command"])
	}
	items := envelope["data"].(map[string]any)["sources"].([]any)
	sources := make([]map[string]any, 0, len(items))
	for _, item := range items {
		sources = append(sources, item.(map[string]any))
	}
	if len(sources) != 2 || sources[0]["source"] != "claude-mem" || sources[1]["source"] != "codex-otel-background" {
		t.Fatalf("sources order = %#v", sources)
	}
	claudeFields := []string{"available", "configured", "enabled", "kind", "last_run_at", "reason_code", "rule_version", "source", "supported_version"}
	codexFields := []string{"available", "background_sessions", "configured", "enabled", "kind", "reason_code", "receiver", "rule_version", "source", "supported_version"}
	for index, want := range [][]string{claudeFields, codexFields} {
		got := make([]string, 0)
		for key := range sources[index] {
			got = append(got, key)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("source %d fields = %v, want %v", index, got, want)
		}
	}
	return sources, envelope["partial"].(bool)
}

func stateFileBytes(t *testing.T, state string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, name := range []string{"agentdeck.sqlite3", "sessions.sqlite3", filepath.Join("session-origin", "sources.json"), filepath.Join("telemetry", "receiver.json")} {
		data, err := os.ReadFile(filepath.Join(state, name))
		if err == nil {
			files[name] = string(data)
		}
	}
	return files
}

func TestSessionOriginSourceStatusStatesAndPrivacy(t *testing.T) {
	state := seedOriginSessions(t)
	dataDir := filepath.Join(t.TempDir(), "claude-mem-private-root")

	// Not configured; receiver.json absent reads as disabled; no records.
	sources, partial := originStatusSources(t, runOriginCLI(t, state, "--format", "json", "session", "origin-source", "status"))
	if partial || sources[0]["reason_code"] != "source_not_configured" || sources[0]["available"] != false || sources[0]["configured"] != false || sources[0]["enabled"] != false || sources[0]["last_run_at"] != nil {
		t.Fatalf("not configured = %#v partial=%v", sources[0], partial)
	}
	if sources[1]["available"] != true || sources[1]["receiver"] != "disabled" || sources[1]["reason_code"] != "otel_receiver_disabled" || sources[1]["background_sessions"] != float64(0) || sources[1]["configured"] != nil || sources[1]["enabled"] != nil {
		t.Fatalf("codex no records = %#v", sources[1])
	}

	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "origin-source", "set", "claude-mem", "--data-dir", "relative/root"), 2, "invalid_argument")
	before := stateFileBytes(t, state)
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "origin-source", "set", "codex-otel-background", "--data-dir", dataDir), 2, "invalid_argument")
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "origin-source", "disable", "codex-otel-background"), 2, "invalid_argument")
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "origin-source", "status", "extra"), 2, "invalid_argument")
	if after := stateFileBytes(t, state); !reflect.DeepEqual(before, after) {
		t.Fatal("rejected built-in source command changed state")
	}

	set := runOriginCLI(t, state, "--format", "json", "session", "origin-source", "set", "claude-mem", "--data-dir", dataDir)
	if data := mustOriginOK(t, set)["data"].(map[string]any); !reflect.DeepEqual(data, map[string]any{"source": "claude-mem", "configured": true, "enabled": true, "mapping_coverage": "not_verified"}) {
		t.Fatalf("set data = %#v", data)
	}
	setText := runOriginCLI(t, state, "--no-color", "session", "origin-source", "set", "claude-mem", "--data-dir", dataDir)
	for _, output := range []string{set.stdout, set.stderr, setText.stdout} {
		if strings.Contains(output, dataDir) || strings.Contains(output, "claude-mem-private-root") {
			t.Fatalf("set output leaked the data root: %q", output)
		}
	}
	sources, partial = originStatusSources(t, runOriginCLI(t, state, "--format", "json", "session", "origin-source", "status"))
	if partial || sources[0]["reason_code"] != "source_not_yet_read" || sources[0]["available"] != false || sources[0]["configured"] != true || sources[0]["enabled"] != true {
		t.Fatalf("not yet read = %#v", sources[0])
	}

	// A committed ok run makes the adapter available; disable makes it false
	// while keeping the summary for viewing.
	core, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	version := "15.2.1"
	if err := core.ReplaceOriginDerivation(context.Background(), store.OriginDerivation{Adapter: "claude-mem", Client: "claude",
		Run: &store.OriginAdapterRun{Adapter: "claude-mem", Outcome: "ok", ValidatedVersion: &version, FinishedAt: "2026-11-03T08:00:00Z"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.DB.Exec(`INSERT INTO telemetry_sessions(client,session_id,source_kind,model,first_at,last_at,support_entry,settled_at) VALUES('codex','bg-1',?, 'm','a','b','codex_exec/0.165.0/codex_exec/sse','s')`, store.TelemetrySessionSourceKind); err != nil {
		t.Fatal(err)
	}
	core.Close()
	before = stateFileBytes(t, state)
	statusRun := runOriginCLI(t, state, "--format", "json", "session", "origin-source", "status")
	sources, partial = originStatusSources(t, statusRun)
	if partial || sources[0]["available"] != true || sources[0]["reason_code"] != nil || sources[0]["supported_version"] != "15.2.1" || sources[0]["last_run_at"] != "2026-11-03T08:00:00Z" {
		t.Fatalf("ok run = %#v", sources[0])
	}
	if sources[1]["available"] != true || sources[1]["reason_code"] != nil || sources[1]["background_sessions"] != float64(1) || sources[1]["receiver"] != "disabled" {
		t.Fatalf("codex with records = %#v", sources[1])
	}
	if after := stateFileBytes(t, state); !reflect.DeepEqual(before, after) {
		t.Fatal("status changed state bytes")
	}
	text := runOriginCLI(t, state, "--no-color", "session", "origin-source", "status")
	if !strings.Contains(text.stdout, "claude-mem              adapter   configured  enabled  available  15.2.1  last read 2026-11-03T08:00:00Z") || !strings.Contains(text.stdout, "codex-otel-background   builtin   1 background sessions  receiver disabled") {
		t.Fatalf("status text = %q", text.stdout)
	}
	for _, output := range []string{statusRun.stdout, text.stdout} {
		if strings.Contains(output, dataDir) || strings.Contains(output, state) {
			t.Fatalf("status leaked a path: %q", output)
		}
	}

	disabled := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "origin-source", "disable", "claude-mem"))
	if data := disabled["data"].(map[string]any); data["enabled"] != false || data["mapping_coverage"] != "unavailable" || data["evidence_invalidated"] != true {
		t.Fatalf("disable = %#v", data)
	}
	sources, partial = originStatusSources(t, runOriginCLI(t, state, "--format", "json", "session", "origin-source", "status"))
	if partial || sources[0]["available"] != false || sources[0]["reason_code"] != "source_disabled" || sources[0]["last_run_at"] != "2026-11-03T08:00:00Z" {
		t.Fatalf("disabled after ok run = %#v", sources[0])
	}

	// receiver.json unreadable: partial, Codex still available.
	if err := os.MkdirAll(telemetry.Dir(state), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(telemetry.Dir(state), "receiver.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	sources, partial = originStatusSources(t, runOriginCLI(t, state, "--format", "json", "session", "origin-source", "status"))
	if !partial || sources[1]["receiver"] != "unknown" || sources[1]["available"] != true || sources[1]["reason_code"] != nil {
		t.Fatalf("receiver unknown with records = %#v partial=%v", sources[1], partial)
	}

	// Configuration unreadable with a run summary present: available false.
	if err := os.WriteFile(filepath.Join(state, "session-origin", "sources.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	sources, partial = originStatusSources(t, runOriginCLI(t, state, "--format", "json", "session", "origin-source", "status"))
	if !partial || sources[0]["reason_code"] != "source_config_unreadable" || sources[0]["available"] != false || sources[0]["configured"] != nil || sources[0]["enabled"] != nil {
		t.Fatalf("config unreadable = %#v", sources[0])
	}
}

func TestOriginSourceStatusDecisionTable(t *testing.T) {
	configured := sessionorigin.SourceConfig{ClaudeMem: sessionorigin.AdapterConfig{DataDir: "/root"}}
	cases := []struct {
		name      string
		in        originStatusInputs
		claude    string
		available bool
		codex     string
		partial   bool
	}{
		{"core unavailable", originStatusInputs{config: configured, coreErr: os.ErrPermission, receiver: "enabled"}, sourceReasonCoreUnavailable, false, sourceReasonCoreUnavailable, true},
		{"config error before core", originStatusInputs{configErr: os.ErrPermission, coreErr: os.ErrPermission, receiver: "enabled"}, sourceReasonConfigUnreadable, false, sourceReasonCoreUnavailable, true},
		{"run unavailable", originStatusInputs{config: configured, runFound: true, run: store.OriginAdapterRun{Outcome: "unavailable"}, receiver: "enabled"}, sourceReasonUnavailable, false, sourceReasonNoBackground, false},
		{"run unsupported", originStatusInputs{config: configured, runFound: true, run: store.OriginAdapterRun{Outcome: "unsupported"}, receiver: "unknown"}, sourceReasonVersionUnsupported, false, sourceReasonReceiverUnknown, true},
		{"run partial", originStatusInputs{config: configured, runFound: true, run: store.OriginAdapterRun{Outcome: "partial"}, receiver: "disabled", background: 2}, "", true, "", false},
	}
	for _, tc := range cases {
		sources, partial := originSourceStatuses(tc.in)
		reason := func(value *string) string {
			if value == nil {
				return ""
			}
			return *value
		}
		if reason(sources[0].ReasonCode) != tc.claude || sources[0].Available != tc.available || reason(sources[1].ReasonCode) != tc.codex || partial != tc.partial {
			t.Fatalf("%s: claude=%q/%v codex=%q partial=%v", tc.name, reason(sources[0].ReasonCode), sources[0].Available, reason(sources[1].ReasonCode), partial)
		}
		if tc.in.coreErr != nil && (sources[1].Available || sources[1].BackgroundSessions != nil) {
			t.Fatalf("%s: codex must be unavailable with null count", tc.name)
		}
	}
	unknown := "future_reason"
	if line := originSourceStatusLine(originSourceStatus{Source: "claude-mem", Kind: "adapter", ReasonCode: &unknown}); !strings.Contains(line, "reason unavailable") || strings.Contains(line, " available ") {
		t.Fatalf("unknown reason line = %q", line)
	}
}

func TestSessionListWithoutCoreKeepsEverySessionUnknownInWork(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	index, err := store.OpenSessions(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.ReplaceDocuments(context.Background(), index.DB, "claude", "only", []session.Document{{Client: "claude", SessionID: "only", EventAt: "2026-11-01T00:00:00Z", Kind: "user_prompt", Text: "x"}}); err != nil {
		t.Fatal(err)
	}
	index.Close()
	work := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "list", "--session-scope", "work"))
	if got := sessionKeys(t, work["data"]); !reflect.DeepEqual(got, []string{"claude/only"}) || work["session_selection"].(map[string]any)["unknown_count"] != float64(1) {
		t.Fatalf("absent core work = %#v", work)
	}
	if _, err := os.Stat(filepath.Join(state, "agentdeck.sqlite3")); !os.IsNotExist(err) {
		t.Fatalf("read created the core: %v", err)
	}
	// An existing but unreadable core is unavailable for work, never zero.
	if err := os.WriteFile(filepath.Join(state, "agentdeck.sqlite3"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	errorEnvelope(t, runOriginCLI(t, state, "--format", "json", "session", "list", "--session-scope", "work"), 1, "runtime_error")
	all := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "list"))
	if selection := all["session_selection"].(map[string]any); selection["availability"] != "unavailable" || selection["unknown_count"] != nil || len(all["data"].([]any)) != 1 {
		t.Fatalf("all with unreadable core = %#v", all)
	}
}

func TestSessionBrowserScopeMenuStateMachine(t *testing.T) {
	work := sessionBrowserPage{scope: "work", items: []session.Metadata{{Client: "claude", SessionID: "a"}, {Client: "claude", SessionID: "b"}}, classes: []string{"unknown", "work"}, unknown: 1}
	state := newScopedSessionBrowserState(work)
	state.apply("down", 10)
	if open, exit := state.apply("s", 10); open || exit || !state.menuOpen || state.menuIndex != 0 {
		t.Fatalf("s did not open the menu on the current scope: %+v", state)
	}
	state.apply("down", 10)
	state.apply("escape", 10)
	if state.menuOpen || state.pendingScope != "" || state.scope != "work" {
		t.Fatalf("escape must only cancel the menu: %+v", state)
	}
	state.apply("s", 10)
	state.apply("down", 10)
	state.apply("down", 10)
	state.apply("enter", 10)
	if state.pendingScope != "all" || state.menuOpen {
		t.Fatalf("enter did not request all: %+v", state)
	}
	state.pendingScope = ""
	state.replace(sessionBrowserPage{scope: "all", items: []session.Metadata{{Client: "codex", SessionID: "x"}, {Client: "claude", SessionID: "b"}}, classes: []string{"auxiliary", "work"}})
	if state.selected != 1 || state.scope != "all" {
		t.Fatalf("selection identity not kept across scopes: %+v", state)
	}
	state.replace(sessionBrowserPage{scope: "auxiliary", items: []session.Metadata{{Client: "codex", SessionID: "x"}}, classes: []string{"auxiliary"}})
	if state.selected != 0 {
		t.Fatalf("missing selection must move to the first row: %+v", state)
	}
	state.failedScope = "work"
	if _, exit := state.apply("escape", 10); exit || state.failedScope != "" {
		t.Fatalf("escape after a failed load must return to the list: %+v", state)
	}
	if _, exit := state.apply("escape", 10); !exit {
		t.Fatal("escape on the list must exit")
	}
	state.apply("s", 10)
	if _, exit := state.apply("q", 10); !exit {
		t.Fatal("q must exit from the menu")
	}

	var rendered bytes.Buffer
	failed := newScopedSessionBrowserState(work)
	failed.failedScope = "auxiliary"
	if err := renderSessionBrowser(&rendered, 80, 24, failed); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"2 SESSIONS · Scope: work · 1 unknown", "UNKNOWN claude", "Requested scope auxiliary failed; showing work.", "s scope"} {
		if !strings.Contains(rendered.String(), want) {
			t.Fatalf("browser frame missing %q:\n%s", want, rendered.String())
		}
	}
	rendered.Reset()
	empty := newScopedSessionBrowserState(sessionBrowserPage{scope: "auxiliary"})
	if err := renderSessionBrowser(&rendered, 48, 10, empty); err != nil || !strings.Contains(rendered.String(), "No auxiliary sessions in this selection.") {
		t.Fatalf("filtered empty = %q %v", rendered.String(), err)
	}
	rendered.Reset()
	menu := newScopedSessionBrowserState(work)
	menu.apply("s", 10)
	if err := renderSessionBrowser(&rendered, 48, 10, menu); err != nil || !strings.Contains(rendered.String(), "> Work (work and unknown)") || !strings.Contains(rendered.String(), "esc cancel") {
		t.Fatalf("menu frame = %q %v", rendered.String(), err)
	}
}

func writeScopedSignalSession(t *testing.T, home, id string) {
	t.Helper()
	source := filepath.Join(home, ".claude", "projects", id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		`{"type":"user","sessionId":"` + id + `","timestamp":"2026-08-29T08:00:00Z","cwd":"/project","message":{"role":"user","content":"change the cache"}}`,
		`{"type":"assistant","sessionId":"` + id + `","timestamp":"2026-08-29T08:00:04Z","cwd":"/project","message":{"role":"assistant","id":"m-` + id + `","model":"claude-test","usage":{"input_tokens":100,"output_tokens":10},"content":[{"type":"tool_use","id":"t-` + id + `","name":"Edit","input":{"file_path":"/project/a.go"}}]}}`,
		`{"type":"user","sessionId":"` + id + `","timestamp":"2026-08-29T08:00:05Z","cwd":"/project","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t-` + id + `","content":"ok"}]}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(source, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestUsageSignalsAndStatsApplySessionScopeOnlyToWorkAnalysis(t *testing.T) {
	home := withUsageSignalsHome(t)
	state := filepath.Join(t.TempDir(), "state")
	waitForDetachedScanCleanup(t, state)
	writeScopedSignalSession(t, home, "signal-work")
	writeScopedSignalSession(t, home, "signal-aux")
	if result := runOriginCLI(t, state, "session", "scan"); result.exit != 0 {
		t.Fatalf("scan = %#v", result)
	}
	mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "session", "classify", "signal-aux", "--client", "claude", "--set", "auxiliary"))
	calls := func(scope string) (float64, map[string]any) {
		t.Helper()
		args := []string{"--format", "json", "usage", "signals", "--period", "all", "--client", "claude", "--kind", "tooling"}
		if scope != "" {
			args = append(args, "--session-scope", scope)
		}
		envelope := mustOriginOK(t, runOriginCLI(t, state, args...))
		tooling := envelope["data"].(map[string]any)["tooling"].(map[string]any)
		return tooling["calls"].(float64), envelope["session_selection"].(map[string]any)
	}
	all, selection := calls("")
	if all != 2 || selection["effective_scope"] != "all" || selection["source_default"] != "json_compat" {
		t.Fatalf("default JSON signals = %v %#v", all, selection)
	}
	work, selection := calls("work")
	auxiliary, _ := calls("auxiliary")
	if work != 1 || auxiliary != 1 || selection["availability"] != "available" {
		t.Fatalf("work=%v auxiliary=%v all=%v selection=%#v", work, auxiliary, all, selection)
	}
	text := runOriginCLI(t, state, "--no-color", "usage", "signals", "--period", "all", "--client", "claude")
	if text.exit != 0 || !strings.HasPrefix(text.stdout, "Scope: work") {
		t.Fatalf("signals text = %#v", text)
	}

	totals := func(scope string) any {
		envelope := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "usage", "stats", "--no-scan", "--period", "all", "--session-scope", scope))
		if envelope["session_selection"].(map[string]any)["effective_scope"] != scope {
			t.Fatalf("stats selection = %#v", envelope["session_selection"])
		}
		return envelope["data"].(map[string]any)["totals"]
	}
	if !reflect.DeepEqual(totals("work"), totals("auxiliary")) || !reflect.DeepEqual(totals("work"), totals("all")) {
		t.Fatal("usage totals changed with the session scope")
	}
	stats := runOriginCLI(t, state, "--no-color", "usage", "stats", "--no-scan", "--period", "all")
	if !strings.Contains(stats.stdout, "Scope: work (sessions, activity, workflow, tooling) · Usage scope: all") || !strings.Contains(stats.stdout, "All usage and costs remain unchanged.") {
		t.Fatalf("stats text = %q", stats.stdout)
	}
}

func TestUsageSignalsWithoutSessionIndexKeepsEverythingInWork(t *testing.T) {
	withUsageSignalsHome(t)
	state := filepath.Join(t.TempDir(), "state")
	waitForDetachedScanCleanup(t, state)
	core, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	core.Close()
	envelope := mustOriginOK(t, runOriginCLI(t, state, "--format", "json", "usage", "stats", "--no-scan", "--period", "all", "--session-scope", "auxiliary"))
	if selection := envelope["session_selection"].(map[string]any); selection["availability"] != "available" || selection["unknown_count"] != float64(0) {
		t.Fatalf("absent index selection = %#v", selection)
	}
	if _, err := os.Stat(filepath.Join(state, "sessions.sqlite3")); !os.IsNotExist(err) {
		t.Fatalf("stored-only stats created the session index: %v", err)
	}
}
