package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/kitdine/agent-deck/internal/watch"
)

type guiJSONContract struct {
	SchemaVersion int                           `json:"schema_version"`
	SuccessFields []string                      `json:"success_fields"`
	ErrorFields   []string                      `json:"error_fields"`
	LeafCommands  []string                      `json:"leaf_commands"`
	Contracts     map[string]guiCommandContract `json:"contracts"`
}

type guiCommandContract struct {
	Success      any    `json:"success_schema"`
	Error        any    `json:"error_schema"`
	ErrorCode    string `json:"error_code"`
	ErrorCommand string `json:"error_command"`
}

type contractRoundTrip func(*http.Request) (*http.Response, error)

func (f contractRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func loadGUIContract(t *testing.T) guiJSONContract {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("testdata", "phase7", "gui-json-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture guiJSONContract
	if err = json.Unmarshal(contents, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != output.SchemaVersion {
		t.Fatalf("fixture schema version = %d", fixture.SchemaVersion)
	}
	return fixture
}

func leafCommands(root *cobra.Command) []*cobra.Command {
	var leaves []*cobra.Command
	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		if (command.RunE != nil || command.Run != nil) && command.Name() != "completion" {
			leaves = append(leaves, command)
		}
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	for _, child := range root.Commands() {
		visit(child)
	}
	return leaves
}

func leafCommandPaths(root *cobra.Command) []string {
	paths := make([]string, 0)
	for _, command := range leafCommands(root) {
		paths = append(paths, commandOutputName(command))
	}
	return sortedStrings(paths)
}

func invalidArgsForLeaf(command *cobra.Command) []string {
	args := strings.Fields(command.CommandPath())[1:]
	if strings.Contains(command.Use, "[") {
		return append(args, "one", "two")
	}
	if strings.Contains(command.Use, "<") {
		return args
	}
	return append(args, "unexpected")
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func assertJSONFields(t *testing.T, encoded []byte, want []string) {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatalf("decode JSON: %q: %v", encoded, err)
	}
	fields := make([]string, 0, len(value))
	for field := range value {
		fields = append(fields, field)
	}
	if !reflect.DeepEqual(sortedStrings(fields), sortedStrings(want)) {
		t.Fatalf("JSON fields = %v, want %v", fields, want)
	}
}

func TestGUIJSONContractFixture(t *testing.T) {
	fixture := loadGUIContract(t)
	if !reflect.DeepEqual(sortedStrings(fixture.LeafCommands), sortedStrings(leafCommandPaths(newRootCommand(bytes.NewReader(nil), &bytes.Buffer{})))) {
		t.Fatalf("fixture leaf commands = %v, actual = %v", fixture.LeafCommands, leafCommandPaths(newRootCommand(bytes.NewReader(nil), &bytes.Buffer{})))
	}
	wantOutputs := make([]string, 0, len(fixture.LeafCommands)+1)
	for _, command := range fixture.LeafCommands {
		if command == "run" {
			wantOutputs = append(wantOutputs, "run.codex", "run.claude")
			continue
		}
		wantOutputs = append(wantOutputs, command)
	}
	contractCommands := make([]string, 0, len(fixture.Contracts))
	for command := range fixture.Contracts {
		contractCommands = append(contractCommands, command)
	}
	if !reflect.DeepEqual(sortedStrings(contractCommands), sortedStrings(wantOutputs)) {
		t.Fatalf("fixture contract commands = %v, want %v", contractCommands, wantOutputs)
	}
	for _, contract := range []struct {
		encoded []byte
		fields  []string
	}{
		{mustJSON(t, output.New("provider.list", []any{}, time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC))), fixture.SuccessFields},
		{mustJSON(t, output.NewError("provider.list", "invalid_argument", "synthetic", time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC))), fixture.ErrorFields},
	} {
		assertJSONFields(t, contract.encoded, contract.fields)
	}
}

func TestWatchNDJSONFixtureIsVersionedAndPrivate(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("testdata", "phase7", "watch-events.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.Split(strings.TrimSpace(string(contents)), "\n")
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	completed := watch.Service{
		Sources: watch.SourceSet{{Domain: "usage", Snapshot: func(context.Context) (string, error) { return "changed", nil }, Scan: func(context.Context) (int, error) { return 2, nil }}},
		Lock:    func(context.Context) (func() error, error) { return func() error { return nil }, nil },
		Now:     func() time.Time { return now },
	}
	busy := watch.Service{
		Sources: watch.SourceSet{{Domain: "usage", Snapshot: func(context.Context) (string, error) { return "changed", nil }, Scan: func(context.Context) (int, error) { t.Fatal("busy scan ran"); return 0, nil }}},
		Lock:    func(context.Context) (func() error, error) { return nil, store.ErrStateBusy },
		Now:     func() time.Time { return now },
	}
	actualEvents, err := completed.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	busyEvents, err := busy.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	actualEvents = append(actualEvents, busyEvents...)
	if len(actualEvents) != len(expected) {
		t.Fatalf("watch events = %#v", actualEvents)
	}
	for index, line := range expected {
		encoded := mustJSON(t, actualEvents[index])
		if string(encoded) != line {
			t.Fatalf("watch golden line %d = %s, want %s", index, encoded, line)
		}
		var event watch.Event
		if err = json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.SchemaVersion != watch.EventSchemaVersion || event.Type == "" || event.Domain == "" || event.GeneratedAt.IsZero() {
			t.Fatalf("invalid watch fixture event: %+v", event)
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatal(err)
		}
		for field := range fields {
			if field != "schema_version" && field != "type" && field != "domain" && field != "generated_at" && field != "changes" && field != "skipped" && field != "reason" {
				t.Fatalf("watch fixture exposes non-contract field %q", field)
			}
		}
	}
}

func TestEveryLeafSyntaxErrorUsesStableJSON(t *testing.T) {
	fixture := loadGUIContract(t)
	root := newRootCommand(bytes.NewReader(nil), &bytes.Buffer{})
	for _, leaf := range leafCommands(root) {
		args := append([]string{"--format", "json"}, invalidArgsForLeaf(leaf)...)
		var stdout, stderr bytes.Buffer
		if exit := execute(args, bytes.NewReader(nil), &stdout, &stderr); exit != 2 {
			t.Fatalf("%s exit = %d, want 2", commandOutputName(leaf), exit)
		}
		if stdout.Len() != 0 {
			t.Fatalf("%s wrote stdout: %s", commandOutputName(leaf), stdout.String())
		}
		assertJSONFields(t, stderr.Bytes(), fixture.ErrorFields)
		var envelope map[string]any
		if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope["command"] != commandOutputName(leaf) || envelope["error"].(map[string]any)["code"] != "invalid_argument" {
			t.Fatalf("%s syntax envelope = %#v", commandOutputName(leaf), envelope)
		}
	}
}

func TestJSONCommandsUseSyntheticStateAndDoNotExposeSecrets(t *testing.T) {
	state, home := filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("[mcp_servers.synthetic]\ncommand = 'synthetic'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	if err := run([]string{"--state-dir", state, "provider", "add", "synthetic", "--endpoint", "https://example.invalid", "--clients", "codex"}, bytes.NewBufferString("synthetic-secret\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args    []string
		command string
	}{
		{[]string{"provider", "list"}, "provider.list"},
		{[]string{"provider", "status"}, "provider.status"},
		{[]string{"provider", "show", "synthetic"}, "provider.show"},
		{[]string{"credential", "list"}, "credential.list"},
		{[]string{"usage", "scan"}, "usage.scan"},
		{[]string{"usage", "summary"}, "usage.summary"},
		{[]string{"usage", "sessions"}, "usage.sessions"},
		{[]string{"usage", "diagnose"}, "usage.diagnose"},
		{[]string{"price", "history"}, "price.history"},
		{[]string{"price", "status"}, "price.status"},
		{[]string{"session", "scan"}, "session.scan"},
		{[]string{"session", "list"}, "session.list"},
		{[]string{"session", "search", "synthetic"}, "session.search"},
		{[]string{"extension", "scan"}, "extension.scan"},
		{[]string{"extension", "list"}, "extension.list"},
		{[]string{"extension", "doctor"}, "extension.doctor"},
		{[]string{"backup", "list"}, "backup.list"},
		{[]string{"doctor", "--full"}, "doctor"},
		{[]string{"version"}, "version"},
	}
	for _, test := range cases {
		t.Run(test.command, func(t *testing.T) {
			var stdout bytes.Buffer
			args := append([]string{"--state-dir", state, "--format", "json"}, test.args...)
			if err := run(args, bytes.NewReader(nil), &stdout); err != nil {
				t.Fatal(err)
			}
			assertNonNullJSONEnvelope(t, stdout.Bytes(), test.command)
			assertNoExportedJSONFields(t, stdout.Bytes())
			if bytes.Contains(stdout.Bytes(), []byte("synthetic-secret")) {
				t.Fatalf("%s exposed a credential", test.command)
			}
		})
	}
	for _, path := range []string{filepath.Join(state, "agentdeck.sqlite3"), filepath.Join(state, "sessions.sqlite3")} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(contents, []byte("synthetic-secret")) {
			t.Fatalf("state database contains a credential: %s", path)
		}
	}
}

func assertNoExportedJSONFields(t *testing.T, encoded []byte) {
	t.Helper()
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	var check func(any)
	check = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if len(key) > 0 && key[0] >= 'A' && key[0] <= 'Z' {
					t.Fatalf("exported Go JSON field %q in %s", key, encoded)
				}
				check(child)
			}
		case []any:
			for _, child := range typed {
				check(child)
			}
		}
	}
	check(value)
}

func TestBackupOutputAndArchiveKeepCredentialEncrypted(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	var stdout bytes.Buffer
	if err := run([]string{"--state-dir", state, "provider", "add", "synthetic", "--endpoint", "https://example.invalid", "--clients", "codex"}, bytes.NewBufferString("phase7-secret\n"), &stdout); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "phase7.adb")
	stdout.Reset()
	if err := run([]string{"--state-dir", state, "--format", "json", "backup", "create", archive}, bytes.NewBufferString("passphrase\n"), &stdout); err != nil {
		t.Fatal(err)
	}
	assertNonNullJSONEnvelope(t, stdout.Bytes(), "backup.create")
	archiveBytes, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	for _, prohibited := range []string{"phase7-secret", "passphrase"} {
		if bytes.Contains(archiveBytes, []byte(prohibited)) || bytes.Contains(stdout.Bytes(), []byte(prohibited)) {
			t.Fatalf("backup output or archive exposes %q", prohibited)
		}
	}
	database, err := store.OpenReadOnly(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := database.ProviderCredential(ctx, "synthetic", "default")
	if closeErr := database.Close(); err == nil {
		err = closeErr
	}
	if err != nil || !credential.SecretPresent {
		t.Fatalf("source credential = %#v, %v", credential, err)
	}
}

func TestPriceOverrideGoldenUsesSnakeCaseInput(t *testing.T) {
	state, home := filepath.Join(t.TempDir(), "state"), t.TempDir()
	fixture := filepath.Join(t.TempDir(), "official-overrides.json")
	contents := []byte(`[{"model":"gpt-5.4","provider":"openai","source_url":"https://example.invalid/pricing","effective_from":"2026-07-14T00:00:00Z","prices":{"output":"9"}}]`)
	if err := os.WriteFile(fixture, contents, 0600); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	var stdout bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "price", "override", "--file", fixture}, bytes.NewReader(nil), &stdout); err != nil {
		t.Fatal(err)
	}
	assertNonNullJSONEnvelope(t, stdout.Bytes(), "price.override")
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || envelope.Data["overrides"] != float64(1) {
		t.Fatalf("override golden = %#v, %v", envelope, err)
	}
	var ignored, stderr bytes.Buffer
	if exit := execute([]string{"--state-dir", state, "--format", "json", "price", "override", "--file", filepath.Join(t.TempDir(), "missing.json")}, bytes.NewReader(nil), &ignored, &stderr); exit != 1 {
		t.Fatalf("missing override exit = %d", exit)
	}
	assertJSONFields(t, stderr.Bytes(), loadGUIContract(t).ErrorFields)
}

func TestProviderRecoverGoldenUsesEmptyArrayAndStableOperationDTO(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	runRecover := func() map[string]any {
		t.Helper()
		var stdout bytes.Buffer
		if err := run([]string{"--state-dir", state, "--format", "json", "provider", "recover"}, bytes.NewReader(nil), &stdout); err != nil {
			t.Fatal(err)
		}
		assertNonNullJSONEnvelope(t, stdout.Bytes(), "provider.recover")
		var envelope map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope
	}
	if data, ok := runRecover()["data"].([]any); !ok || len(data) != 0 {
		t.Fatalf("empty recover data = %#v", data)
	}
	database, err = store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.CreateOperation(ctx, store.Operation{ID: "phase7-pending", Kind: "provider.use", State: "prepared", Client: "codex"}); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	data, ok := runRecover()["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("pending recover data = %#v", data)
	}
	operation, ok := data[0].(map[string]any)
	if !ok || operation["id"] != "phase7-pending" || operation["state"] != "failed" || operation["error_code"] != "interrupted_before_external_write" {
		t.Fatalf("recover operation = %#v", operation)
	}
	assertNoExportedJSONFields(t, mustJSON(t, operation))
}

func TestPriceUpdateGoldenUsesInjectedTransport(t *testing.T) {
	commit := "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
	body := `{"gpt":{"litellm_provider":"openai","input_cost_per_token":0.000002,"output_cost_per_token":0.00001,"cache_read_input_token_cost":0.0000002}}`
	previous := usage.PriceHTTPClient
	usage.PriceHTTPClient = func() *http.Client {
		return &http.Client{Transport: contractRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}
	}
	t.Cleanup(func() { usage.PriceHTTPClient = previous })
	var stdout bytes.Buffer
	if err := run([]string{"--state-dir", filepath.Join(t.TempDir(), "state"), "--format", "json", "price", "update", "--commit", commit}, bytes.NewReader(nil), &stdout); err != nil {
		t.Fatal(err)
	}
	assertNonNullJSONEnvelope(t, stdout.Bytes(), "price.update")
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || envelope.Data["models"] != float64(1) || envelope.Data["commit_sha"] != commit {
		t.Fatalf("price update golden = %#v, %v", envelope, err)
	}
}

func TestRunClaudeGolden(t *testing.T) {
	previousProcesses := runClientProcesses
	runClientProcesses = func(string) ([]int, error) { return nil, nil }
	t.Cleanup(func() { runClientProcesses = previousProcesses })
	root := t.TempDir()
	state, home, bin := filepath.Join(root, "state"), filepath.Join(root, "home"), filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(home, ".claude", "projects", "phase7", "session.jsonl")
	script := "#!/bin/sh\nmkdir -p \"$(dirname \"$AGENTDECK_PHASE7_CLAUDE_LOG\")\"\nprintf '%s\\n' '{\"type\":\"assistant\",\"timestamp\":\"2026-07-14T00:00:01Z\",\"sessionId\":\"phase7-claude\",\"message\":{\"id\":\"phase7-message\",\"model\":\"claude-sonnet-4-6\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"cache_read_input_tokens\":0,\"cache_creation_input_tokens\":0}}}' > \"$AGENTDECK_PHASE7_CLAUDE_LOG\"\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AGENTDECK_PHASE7_CLAUDE_LOG", log)
	t.Cleanup(func() { userHomeDir = oldHome })
	invoke := func(stdin string, args ...string) []byte {
		t.Helper()
		var stdout bytes.Buffer
		if err := run(append([]string{"--state-dir", state, "--format", "json"}, args...), bytes.NewBufferString(stdin), &stdout); err != nil {
			t.Fatal(err)
		}
		return stdout.Bytes()
	}
	invoke("phase7-claude-secret\n", "provider", "add", "phase7-claude", "--endpoint", "https://example.invalid", "--clients", "claude")
	invoke("", "provider", "use", "phase7-claude")
	result := invoke("", "run", "claude", "--", "phase7")
	assertNonNullJSONEnvelope(t, result, "run.claude")
	var envelope struct {
		Data struct {
			Exact       bool   `json:"exact"`
			Attribution string `json:"attribution"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result, &envelope); err != nil || !envelope.Data.Exact || envelope.Data.Attribution != "exact" {
		t.Fatalf("claude run = %#v, %v", envelope.Data, err)
	}
}
