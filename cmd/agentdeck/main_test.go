package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/buildinfo"
	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/extension"
	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/kitdine/agent-deck/internal/watch"
)

func TestMain(m *testing.M) {
	machineIdentity = func(context.Context) (string, error) { return "synthetic-machine", nil }
	os.Exit(m.Run())
}

func TestRootCommandRegistersGlobalFlags(t *testing.T) {
	root := newRootCommand(bytes.NewReader(nil), &bytes.Buffer{})
	for _, name := range []string{"state-dir", "format", "no-color", "quiet"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Fatalf("missing global flag %q", name)
		}
	}
	commands := map[string]bool{}
	for _, command := range root.Commands() {
		commands[command.Name()] = true
	}
	for _, name := range []string{"extension", "watch", "backup", "doctor"} {
		if !commands[name] {
			t.Fatalf("command %q missing", name)
		}
	}
	if root.Flags().Lookup("version") == nil {
		t.Fatal("missing root-only version flag")
	}
}

type accessCountingCredentialVault struct{ calls int }

func (s *accessCountingCredentialVault) called() error {
	s.calls++
	return errors.New("unexpected credential vault access")
}
func (s *accessCountingCredentialVault) Seal(context.Context, string, string) (credentialvault.Sealed, error) {
	return credentialvault.Sealed{}, s.called()
}
func (s *accessCountingCredentialVault) SealExisting(context.Context, string, string) (credentialvault.Sealed, error) {
	return credentialvault.Sealed{}, s.called()
}
func (s *accessCountingCredentialVault) Open(context.Context, string, credentialvault.Sealed) (string, error) {
	return "", s.called()
}
func (s *accessCountingCredentialVault) InspectKey(context.Context) (string, error) {
	return "", s.called()
}

func TestProviderListAndShowOfficialDoNotAccessSecretsOrLeakCredentials(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	vault := &accessCountingCredentialVault{}
	oldFactory := newCredentialVault
	newCredentialVault = func(string) provider.CredentialVault { return vault }
	t.Cleanup(func() { newCredentialVault = oldFactory })

	for _, format := range []string{"text", "json"} {
		for _, args := range [][]string{{"provider", "list"}, {"provider", "show", "official"}} {
			var stdout bytes.Buffer
			commandArgs := append([]string{"--state-dir", state, "--format", format}, args...)
			if err := run(commandArgs, bytes.NewReader(nil), &stdout); err != nil {
				t.Fatalf("%s %v: %v", format, args, err)
			}
			if !strings.Contains(stdout.String(), "official") || strings.Contains(stdout.String(), "experimental_bearer_token") || strings.Contains(stdout.String(), "synthetic-secret") {
				t.Fatalf("%s %v output = %s", format, args, stdout.String())
			}
		}
	}
	if vault.calls != 0 {
		t.Fatalf("provider list/show accessed credential vault %d times", vault.calls)
	}
}

func TestSessionCLIListAndShowPaginationContracts(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state with quote ' and space")
	home := t.TempDir()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	database, err := store.OpenSessions(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		client, id string
		docs       []session.Document
	}{
		{"codex", "a", []session.Document{{Client: "codex", SessionID: "a", Kind: "user_prompt", Text: "a1"}, {Client: "codex", SessionID: "a", Kind: "assistant_final", Text: "a2"}, {Client: "codex", SessionID: "a", Kind: "user_prompt", Text: "a3"}}},
		{"codex", "b", []session.Document{{Client: "codex", SessionID: "b", Kind: "user_prompt", Text: "b"}}},
		{"codex", "c", []session.Document{{Client: "codex", SessionID: "c", Kind: "user_prompt", Text: "c"}}},
		{"claude", "d", []session.Document{{Client: "claude", SessionID: "d", Kind: "user_prompt", Text: "d"}}},
	} {
		if err = session.ReplaceDocuments(context.Background(), database.DB, item.client, item.id, item.docs); err != nil {
			database.Close()
			t.Fatal(err)
		}
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}

	var listJSON bytes.Buffer
	if err = run([]string{"--state-dir", state, "--format", "json", "session", "list", "--client", "codex", "--page", "1", "--limit", "2"}, bytes.NewReader(nil), &listJSON); err != nil {
		t.Fatal(err)
	}
	var listEnvelope struct {
		Data struct {
			Sessions   []session.Metadata            `json:"sessions"`
			Pagination map[string]session.Pagination `json:"pagination"`
		} `json:"data"`
	}
	if err = json.Unmarshal(listJSON.Bytes(), &listEnvelope); err != nil {
		t.Fatal(err)
	}
	listPage := listEnvelope.Data.Pagination["sessions"]
	if len(listEnvelope.Data.Sessions) != 2 || listPage.Total != 3 || listPage.Shown != 2 || !listPage.HasMore || listPage.NextPage != 2 {
		t.Fatalf("filtered list pagination = %s", listJSON.String())
	}

	var listText bytes.Buffer
	if err = run([]string{"--state-dir", state, "session", "list", "--client", "codex", "--page", "1", "--limit", "2"}, bytes.NewReader(nil), &listText); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Showing 1-2 of 3", "--state-dir " + shellQuote(state), "--client 'codex'", "--page 2", "--limit 2"} {
		if !strings.Contains(listText.String(), want) {
			t.Fatalf("list text missing %q: %s", want, listText.String())
		}
	}

	var legacy bytes.Buffer
	if err = run([]string{"--state-dir", state, "--format", "json", "session", "list", "--client", "codex"}, bytes.NewReader(nil), &legacy); err != nil {
		t.Fatal(err)
	}
	var legacyEnvelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(legacy.Bytes(), &legacyEnvelope); err != nil || len(legacyEnvelope.Data) == 0 || legacyEnvelope.Data[0] != '[' || bytes.Contains(legacyEnvelope.Data, []byte(`"pagination"`)) {
		t.Fatalf("legacy list JSON = %s, %v", legacy.String(), err)
	}

	var showJSON bytes.Buffer
	if err = run([]string{"--state-dir", state, "--format", "json", "session", "show", "a", "--page", "1", "--limit", "2"}, bytes.NewReader(nil), &showJSON); err != nil {
		t.Fatal(err)
	}
	var showEnvelope struct {
		Data struct {
			Client     string                        `json:"client"`
			Documents  []session.Document            `json:"documents"`
			Pagination map[string]session.Pagination `json:"pagination"`
		} `json:"data"`
	}
	if err = json.Unmarshal(showJSON.Bytes(), &showEnvelope); err != nil {
		t.Fatal(err)
	}
	showPage := showEnvelope.Data.Pagination["documents"]
	if showEnvelope.Data.Client != "codex" || len(showEnvelope.Data.Documents) != 2 || showPage.Total != 3 || showPage.NextPage != 2 {
		t.Fatalf("show pagination = %s", showJSON.String())
	}
	var showText bytes.Buffer
	if err = run([]string{"--state-dir", state, "session", "show", "a", "--page", "1", "--limit", "2"}, bytes.NewReader(nil), &showText); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Showing 1-2 of 3", "--client 'codex'", "--page 2", "--limit 2"} {
		if !strings.Contains(showText.String(), want) {
			t.Fatalf("show text missing %q: %s", want, showText.String())
		}
	}

	for _, args := range [][]string{
		{"--state-dir", state, "session", "list", "--all", "--limit", "2"},
		{"--state-dir", state, "session", "show", "a", "--all", "--page", "2"},
		{"--state-dir", state, "session", "list", "--limit", "1001"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := execute(args, bytes.NewReader(nil), &stdout, &stderr); exit != 2 {
			t.Fatalf("args=%v exit=%d stderr=%s", args, exit, stderr.String())
		}
	}
	var hugePage bytes.Buffer
	if err = run([]string{"--state-dir", state, "--format", "json", "session", "list", "--page", strconv.FormatInt(math.MaxInt64, 10), "--limit", "1"}, bytes.NewReader(nil), &hugePage); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hugePage.String(), `"shown":0`) || !strings.Contains(hugePage.String(), `"total":4`) {
		t.Fatalf("huge page JSON = %s", hugePage.String())
	}
}

func TestProviderStatusReportsZeroCredentialsNotReadyInTextAndJSON(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	service := provider.Service{Store: database, Vault: newCredentialVault(state)}
	if _, err = service.Add(ctx, provider.Definition{Name: "empty", Endpoint: "https://example.invalid", CredentialRef: "empty-ref", Clients: []provider.Client{provider.ClientCodex}}, "synthetic-secret"); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = service.RemoveNamedCredential(ctx, "empty", "default"); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}

	var textOutput bytes.Buffer
	if err = run([]string{"--state-dir", state, "provider", "status", "empty"}, bytes.NewReader(nil), &textOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOutput.String(), "credentials: none") || !strings.Contains(textOutput.String(), "ready: false") {
		t.Fatalf("text status = %s", textOutput.String())
	}
	var jsonOutput bytes.Buffer
	if err = run([]string{"--state-dir", state, "--format", "json", "provider", "status", "empty"}, bytes.NewReader(nil), &jsonOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonOutput.String(), `"ready":false`) || strings.Contains(jsonOutput.String(), `"present":true`) {
		t.Fatalf("json status = %s", jsonOutput.String())
	}
}

func TestProviderStatusCollectionUsesIndependentActivationColumns(t *testing.T) {
	values := []provider.Status{
		{Definition: provider.Provider{Name: "official", BuiltIn: true}, Ready: true, Active: []provider.ActiveSelection{{Client: "codex"}}},
		{Definition: provider.Provider{Name: "custom"}, Credentials: []provider.Credential{{}, {}}, Active: []provider.ActiveSelection{{Client: "claude", Credential: "personal"}}},
	}
	var output bytes.Buffer
	if err := renderProviderStatuses(&output, values); err != nil {
		t.Fatal(err)
	}
	want := "" +
		"+----------+----------+-------------+-------+--------------+---------------+\n" +
		"| NAME     | TYPE     | CREDENTIALS | READY | CODEX ACTIVE | CLAUDE ACTIVE |\n" +
		"+----------+----------+-------------+-------+--------------+---------------+\n" +
		"| official | built-in | 0           | true  | -            | -             |\n" +
		"+----------+----------+-------------+-------+--------------+---------------+\n" +
		"| custom   | custom   | 2           | false | -            | personal      |\n" +
		"+----------+----------+-------------+-------+--------------+---------------+\n"
	if output.String() != want {
		t.Fatalf("provider status table =\n%s", output.String())
	}
}

func TestDoctorDiagnosesProviderUseExternalStateInTextAndJSON(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	state, home := filepath.Join(root, "state"), filepath.Join(root, "home")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config.toml")
	if err = os.WriteFile(config, []byte("before\n"), 0o600); err != nil {
		database.Close()
		t.Fatal(err)
	}
	fingerprint, err := provider.ConfigFingerprint(config)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = os.WriteFile(config, []byte("after\n"), 0o600); err != nil {
		database.Close()
		t.Fatal(err)
	}
	operations := []store.Operation{
		{ID: "external-transition", Kind: "provider.use", State: "failed", ErrorCode: "external_written_transition_failed", Client: "codex"},
		{ID: "completed-transition", Kind: "provider.use", State: "failed", ErrorCode: "selection_commit_failed", Client: "codex"},
		{ID: "failure-recording", Kind: "provider.use", State: "prepared", Client: "codex", ConfigFingerprint: fingerprint, DetailsJSON: fmt.Sprintf(`{"config_path":%q}`, config)},
	}
	for _, operation := range operations {
		if err = database.CreateOperation(ctx, operation); err != nil {
			database.Close()
			t.Fatal(err)
		}
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	for _, format := range []string{"text", "json"} {
		var output bytes.Buffer
		if err = run([]string{"--state-dir", state, "--format", format, "doctor"}, bytes.NewReader(nil), &output); err != nil {
			t.Fatalf("doctor %s: %v", format, err)
		}
		for _, code := range []string{"external_written_transition_failed", "selection_commit_failed", "interrupted_after_external_write"} {
			if !strings.Contains(output.String(), code) {
				t.Fatalf("doctor %s missing %s: %s", format, code, output.String())
			}
		}
	}
}

func TestEveryLeafCommandHasActionableHelp(t *testing.T) {
	root := newRootCommand(bytes.NewReader(nil), &bytes.Buffer{})
	for _, command := range leafCommands(root) {
		if strings.TrimSpace(command.Short) == "" {
			t.Errorf("%s has no short help", command.CommandPath())
		}
		if strings.Contains(command.Use, "<") || strings.Contains(command.Use, "[") {
			if !strings.Contains(command.Long, "Arguments:") {
				t.Errorf("%s has no argument help", command.CommandPath())
			}
			if strings.TrimSpace(command.Example) == "" {
				t.Errorf("%s has no example", command.CommandPath())
			}
		}
	}

	use, _, err := root.Find([]string{"provider", "use"})
	if err != nil {
		t.Fatal(err)
	}
	var help bytes.Buffer
	use.SetOut(&help)
	if err := use.Help(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"use <name>",
		"inferring unique choices",
		"--config-path",
		"agentdeck provider use aigocode --client codex",
	} {
		if !strings.Contains(help.String(), expected) {
			t.Errorf("provider use help missing %q:\n%s", expected, help.String())
		}
	}
}

func TestHelpOmitsLegacyProviderCredentialAndSessionExamples(t *testing.T) {
	root := newRootCommand(bytes.NewReader(nil), &bytes.Buffer{})
	var rendered strings.Builder
	for _, command := range append([]*cobra.Command{root}, leafCommands(root)...) {
		var help bytes.Buffer
		command.SetOut(&help)
		if err := command.Help(); err != nil {
			t.Fatal(err)
		}
		rendered.Write(help.Bytes())
	}
	for _, legacy := range []string{
		"agentdeck provider credential",
		"keep-keychain",
		"Keychain",
		"agentdeck session show codex ",
		"agentdeck session show claude ",
		"agentdeck session exclude project ",
		"agentdeck session exclude session ",
	} {
		if strings.Contains(rendered.String(), legacy) {
			t.Fatalf("legacy help example %q remains", legacy)
		}
	}
	for _, current := range []string{
		"agentdeck credential add aigocode",
		"agentdeck session show 019abc123 --client codex",
		"agentdeck session exclude --kind client --value claude",
	} {
		if !strings.Contains(rendered.String(), current) {
			t.Fatalf("current help example %q missing", current)
		}
	}
}

func TestSessionShowActivityReadsOnlySafeMetadataOnDemand(t *testing.T) {
	home := t.TempDir()
	state := filepath.Join(t.TempDir(), "state")
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	source := filepath.Join(home, ".codex", "sessions", "2026", "07", "20", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := strings.Join([]string{
		`{"type":"session_meta","timestamp":"2026-07-20T00:00:00Z","payload":{"session_id":"activity-session"}}`,
		`{"type":"turn_context","payload":{"turn_id":"turn-1","model":"gpt-5.4"}}`,
		`{"type":"response_item","timestamp":"2026-07-20T00:00:00Z","payload":{"item":{"type":"message","role":"user","content":[{"type":"input_text","text":"visible prompt"}]}}}`,
		`{"type":"response_item","timestamp":"2026-07-20T00:00:01Z","payload":{"item":{"type":"function_call","call_id":"call-1","name":"exec_command","arguments":"credential=secret"}}}`,
		`{"type":"response_item","timestamp":"2026-07-20T00:00:03Z","payload":{"item":{"type":"function_call_output","call_id":"call-1","output":"private result"}}}`,
		`{"type":"response_item","timestamp":"2026-07-20T00:00:04Z","payload":{"item":{"type":"function_call","call_id":"call-2","name":"exec_command","arguments":"credential=second-secret"}}}`,
		`{"type":"response_item","timestamp":"2026-07-20T00:00:05Z","payload":{"item":{"type":"function_call_output","call_id":"call-2","output":"second private result"}}}`,
		`{"type":"event_msg","timestamp":"2026-07-20T00:00:06Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":1}}}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(source, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--state-dir", state, "session", "scan"}, bytes.NewReader(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"text", "json"} {
		var output bytes.Buffer
		args := []string{"--state-dir", state}
		if format == "json" {
			args = append(args, "--format", "json")
		}
		args = append(args, "session", "show", "activity-session", "--client", "codex", "--activity")
		if err := run(args, bytes.NewReader(nil), &output); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		for _, want := range []string{"exec_command", "completed", "2000"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s activity missing %q: %s", format, want, text)
			}
		}
		for _, secret := range []string{"credential=secret", "credential=second-secret", "private result", "second private result", "arguments", "output"} {
			if strings.Contains(text, secret) {
				t.Fatalf("%s activity leaked %q: %s", format, secret, text)
			}
		}
	}
	var pagedText bytes.Buffer
	if err := run([]string{"--state-dir", state, "session", "show", "activity-session", "--activity", "--page", "1", "--limit", "1"}, bytes.NewReader(nil), &pagedText); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"activity: total=2 completed=2 failed=0 incomplete=0 total_duration_ms=3000 average_duration_ms=1500",
		"Showing 1-1 of 2",
		fmt.Sprintf("Next page: agentdeck --state-dir '%s' session show 'activity-session' --client 'codex' --activity --page 2 --limit 1", state),
	} {
		if !strings.Contains(pagedText.String(), want) {
			t.Fatalf("paged activity text missing %q: %s", want, pagedText.String())
		}
	}
	var paged bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "session", "show", "activity-session", "--activity", "--page", "1", "--limit", "1"}, bytes.NewReader(nil), &paged); err != nil {
		t.Fatal(err)
	}
	var pagedEnvelope struct {
		Data struct {
			Activity        []json.RawMessage             `json:"activity"`
			ActivitySummary *session.ActivitySummary      `json:"activity_summary"`
			Pagination      map[string]session.Pagination `json:"pagination"`
		} `json:"data"`
	}
	if err := json.Unmarshal(paged.Bytes(), &pagedEnvelope); err != nil {
		t.Fatal(err)
	}
	activityPage := pagedEnvelope.Data.Pagination["activity"]
	if _, ok := pagedEnvelope.Data.Pagination["documents"]; !ok || len(pagedEnvelope.Data.Activity) != 1 || activityPage.Total != 2 || activityPage.Shown != 1 || !activityPage.HasMore || activityPage.NextPage != 2 || pagedEnvelope.Data.ActivitySummary == nil || pagedEnvelope.Data.ActivitySummary.Total != 2 {
		t.Fatalf("show activity pagination = %s", paged.String())
	}
	var stats bytes.Buffer
	if err := run([]string{"--state-dir", state, "usage", "stats", "--from", "2026-07-20", "--to", "2026-07-20", "--model", "gpt-5.4", "--activity"}, bytes.NewReader(nil), &stats); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"MODEL ACTIVITY · gpt-5.4", "exec_command", "2 tools", "2 completed"} {
		if !strings.Contains(stats.String(), want) {
			t.Fatalf("model activity missing %q: %s", want, stats.String())
		}
	}
	for _, secret := range []string{"credential=secret", "credential=second-secret", "private result", "second private result"} {
		if strings.Contains(stats.String(), secret) {
			t.Fatalf("model activity leaked %q: %s", secret, stats.String())
		}
	}
}

func TestPhase9TextAndJSONGoldenContracts(t *testing.T) {
	baseCost, knownProvider := "1.250000000", "1.500000000"
	summary := usage.Summary{
		Counts:               map[string]int64{"events": 2, "exact": 1, "estimated": 1, "priced": 1, "unpriced": 1},
		Tokens:               map[string]int64{"input_tokens": 3, "output_tokens": 4},
		CatalogBaseCost:      &baseCost,
		KnownCatalogBaseCost: &baseCost,
		KnownProviderCost:    &knownProvider,
		Warnings:             []string{"scan_incomplete"},
		Unpriced:             []string{"unknown_model"},
		Models:               []usage.ModelCoverage{{Client: "codex", Model: "gpt-5.4", Events: 1, PricedEvents: 1}, {Client: "codex", Model: "model-x", Events: 1, UnpricedEvents: 1}},
	}
	var textOutput bytes.Buffer
	if err := writeEnvelope(&textOutput, "text", "usage.summary", summary, true, summary.Warnings); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"📊 USAGE SUMMARY", "🪙 TOKEN TOTALS", "🧾 MODEL COVERAGE", "| known catalog subtotal", "| input", "| codex  | model-x"} {
		if !strings.Contains(textOutput.String(), want) {
			t.Fatalf("usage text golden missing %q:\n%s", want, textOutput.String())
		}
	}
	if strings.Contains(textOutput.String(), "0x") || strings.Contains(textOutput.String(), "input_tokens=") {
		t.Fatalf("usage text golden contains internal formatting:\n%s", textOutput.String())
	}

	providerData := provider.DefinitionResult{Definition: provider.Provider{
		Name:            "example",
		Clients:         []store.ClientMapping{{Client: "codex"}},
		CredentialCount: 1,
	}}
	textOutput.Reset()
	if err := writeResult(&textOutput, "text", "provider.show", providerData); err != nil {
		t.Fatal(err)
	}
	wantProvider := "name: example\ntype: custom\nclients: codex\ncredentials: 1\n"
	if textOutput.String() != wantProvider {
		t.Fatalf("provider detail golden = %q, want %q", textOutput.String(), wantProvider)
	}

	report := doctor.Report{Mode: "quick", Status: "degraded", Warnings: 1, Checks: []doctor.Check{{Name: "credential", Status: "warning", Code: "credential_missing"}}}
	textOutput.Reset()
	if err := writeResult(&textOutput, "text", "doctor", report); err != nil {
		t.Fatal(err)
	}
	wantDoctor := "status: degraded\nmode: quick\nwarnings: 1\nerrors: 0\ncredential: warning (credential_missing)\n"
	if textOutput.String() != wantDoctor {
		t.Fatalf("doctor text golden = %q, want %q", textOutput.String(), wantDoctor)
	}

	var jsonOutput bytes.Buffer
	if err := writeEnvelope(&jsonOutput, "json", "usage.summary", summary, true, summary.Warnings, true); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(jsonOutput.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["schema_version"] != float64(output.SchemaVersion) || envelope["command"] != "usage.summary" || envelope["partial"] != true {
		t.Fatalf("usage JSON golden = %#v", envelope)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok || data["catalog_base_cost"] != baseCost || data["provider_cost"] != nil || strings.Contains(jsonOutput.String(), "synthetic-secret") {
		t.Fatalf("usage JSON data = %#v", envelope["data"])
	}
}

func TestUsageRebuildPartialWarningsAreVisible(t *testing.T) {
	stats := map[string]int{"files": 2, "imported": 1, "updated": 0, "ignored_non_usage": 0, "unsupported_usage": 0, "malformed": 0, "source_resets": 1}
	warnings := []string{"usage_source_rebuild_failed"}
	var textOutput bytes.Buffer
	if err := writeEnvelope(&textOutput, "text", "usage.rebuild", stats, true, warnings); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOutput.String(), "warnings: usage_source_rebuild_failed\n") {
		t.Fatalf("usage rebuild text = %q", textOutput.String())
	}
	var jsonOutput bytes.Buffer
	if err := writeEnvelope(&jsonOutput, "json", "usage.rebuild", stats, true, warnings); err != nil {
		t.Fatal(err)
	}
	var envelope output.Envelope
	if err := json.Unmarshal(jsonOutput.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Partial || len(envelope.Warnings) != 1 || envelope.Warnings[0] != warnings[0] {
		t.Fatalf("usage rebuild envelope = %#v", envelope)
	}
	var quietOutput bytes.Buffer
	if err := writeEnvelope(&quietOutput, "text", "usage.rebuild", stats, true, warnings, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(quietOutput.String(), "warnings: usage_source_rebuild_failed\n") {
		t.Fatalf("quiet partial rebuild text = %q", quietOutput.String())
	}
}

func TestQuietSuppressesOnlySuccessfulTextMutations(t *testing.T) {
	textState := filepath.Join(t.TempDir(), "text-state")
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"--state-dir", textState, "--quiet", "provider", "add", "quiet-provider", "--endpoint", "https://example.invalid", "--clients", "codex"}, bytes.NewBufferString("quiet-secret\n"), &stdout, &stderr); exit != 0 {
		t.Fatalf("quiet text exit=%d stderr=%s", exit, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("quiet text output = %q", stdout.String())
	}

	jsonState := filepath.Join(t.TempDir(), "json-state")
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"--state-dir", jsonState, "--format", "json", "--quiet", "provider", "add", "json-provider", "--endpoint", "https://example.invalid", "--clients", "codex"}, bytes.NewBufferString("json-secret\n"), &stdout, &stderr); exit != 0 {
		t.Fatalf("quiet JSON exit=%d stderr=%s", exit, stderr.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || envelope["command"] != "provider.add" || envelope["schema_version"] != float64(output.SchemaVersion) {
		t.Fatalf("quiet JSON envelope = %#v err=%v", envelope, err)
	}
	if strings.Contains(stdout.String(), "json-secret") {
		t.Fatalf("quiet JSON leaked credential: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"--state-dir", textState, "--quiet", "provider", "add"}, bytes.NewReader(nil), &stdout, &stderr); exit != 2 || stderr.Len() == 0 {
		t.Fatalf("quiet error exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
}

func TestProviderUseAutomaticallyResolvesClientAndBackupPaths(t *testing.T) {
	home, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	config := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("model = 'keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	if err := run([]string{"--state-dir", state, "provider", "add", "aigocode", "--endpoint", "https://example.invalid", "--clients", "codex"}, bytes.NewBufferString("synthetic-secret\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--state-dir", state, "provider", "use", "aigocode"}, bytes.NewReader(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(config)
	if err != nil || !strings.Contains(string(contents), "https://example.invalid/v1") {
		t.Fatalf("auto-resolved config = %q, %v", contents, err)
	}
	backups, err := filepath.Glob(filepath.Join(state, "client-backups", "codex", "*.redacted.toml"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("auto-managed backups = %v, %v", backups, err)
	}
	customConfig := filepath.Join(t.TempDir(), "custom-config.toml")
	if err := os.WriteFile(customConfig, []byte("model = 'custom'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--state-dir", state, "provider", "use", "aigocode", "--client", "codex", "--config-path", customConfig}, bytes.NewReader(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	customContents, err := os.ReadFile(customConfig)
	if err != nil || !strings.Contains(string(customContents), "https://example.invalid/v1") {
		t.Fatalf("overridden config = %q, %v", customContents, err)
	}
}

func TestVersionCommandAndFlagShareTextAndJSONContract(t *testing.T) {
	oldVersion, oldCommit, oldBranch, oldBuildTime := buildinfo.Version, buildinfo.Commit, buildinfo.Branch, buildinfo.BuildTime
	buildinfo.Version, buildinfo.Commit, buildinfo.Branch, buildinfo.BuildTime = "v1.2.3", "0123456789abcdef", "main", "2026-07-15 00:00:00"
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit, buildinfo.Branch, buildinfo.BuildTime = oldVersion, oldCommit, oldBranch, oldBuildTime
	})

	var commandText, flagText bytes.Buffer
	if err := run([]string{"version"}, bytes.NewReader(nil), &commandText); err != nil {
		t.Fatal(err)
	}
	wantText := fmt.Sprintf("Release Version: v1.2.3\nGit Commit Hash: 0123456789abcdef\nGit Branch: main\nGo Version: %s\nUTC Build Time: 2026-07-15 00:00:00\n", buildinfo.Current().GoVersion)
	if commandText.String() != wantText {
		t.Fatalf("version text = %q, want %q", commandText.String(), wantText)
	}
	for _, args := range [][]string{
		{"--version"},
		{"--version=true"},
		{"--no-color=true", "--quiet=false", "--version=true"},
	} {
		flagText.Reset()
		if err := run(args, bytes.NewReader(nil), &flagText); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
		if commandText.String() != flagText.String() {
			t.Fatalf("version text command=%q flag %v=%q", commandText.String(), args, flagText.String())
		}
	}

	for _, args := range [][]string{{"--format", "json", "version"}, {"--format", "json", "--version"}, {"--format=json", "--no-color=true", "--version=true"}} {
		var encoded bytes.Buffer
		if err := run(args, bytes.NewReader(nil), &encoded); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
		var envelope struct {
			SchemaVersion int                `json:"schema_version"`
			Command       string             `json:"command"`
			Data          buildinfo.Identity `json:"data"`
		}
		if err := json.Unmarshal(encoded.Bytes(), &envelope); err != nil {
			t.Fatalf("decode %v: %q: %v", args, encoded.String(), err)
		}
		if envelope.SchemaVersion != 1 || envelope.Command != "version" || envelope.Data.Version != "v1.2.3" || envelope.Data.Commit != "0123456789abcdef" || envelope.Data.Branch != "main" || envelope.Data.BuildTime != "2026-07-15 00:00:00" || envelope.Data.GoVersion == "" {
			t.Fatalf("version envelope for %v = %#v", args, envelope)
		}
	}
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"provider", "--version"}, bytes.NewReader(nil), &stdout, &stderr); exit != 2 {
		t.Fatalf("subcommand --version exit = %d, stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	for _, args := range [][]string{{"--format", "yaml", "version"}, {"--format=yaml", "--version"}} {
		stdout.Reset()
		stderr.Reset()
		if exit := execute(args, bytes.NewReader(nil), &stdout, &stderr); exit != 2 {
			t.Fatalf("invalid format %v exit = %d, stdout=%q stderr=%q", args, exit, stdout.String(), stderr.String())
		}
		if stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte("invalid format")) {
			t.Fatalf("invalid format %v stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		}
	}
}

func TestRunJSONPropagatesChildFailureAndClosesRun(t *testing.T) {
	for _, client := range []string{"codex", "claude"} {
		t.Run(client, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			state, home, bin := filepath.Join(root, "state"), filepath.Join(root, "home"), filepath.Join(root, "bin")
			if err := os.MkdirAll(bin, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, client), []byte("#!/bin/sh\nexit 7\n"), 0700); err != nil {
				t.Fatal(err)
			}
			database, err := store.Open(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			created, err := database.CreateProvider(ctx, store.Provider{Name: "synthetic", Endpoint: "https://example.invalid", CredentialRef: "agentdeck:test", Multiplier: "1", Clients: []store.ClientMapping{{Client: client}}})
			if err != nil {
				database.Close()
				t.Fatal(err)
			}
			if err = database.RecordSelection(ctx, store.Selection{ProviderID: created.ID, Client: client, MultiplierSnapshot: "1", SelectedAt: time.Now()}); err != nil {
				database.Close()
				t.Fatal(err)
			}
			if err = database.Close(); err != nil {
				t.Fatal(err)
			}

			oldHome := userHomeDir
			userHomeDir = func() (string, error) { return home, nil }
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Cleanup(func() { userHomeDir = oldHome })
			var stdout, stderr bytes.Buffer
			if exit := execute([]string{"--state-dir", state, "--format", "json", "run", client, "--", "synthetic"}, bytes.NewReader(nil), &stdout, &stderr); exit != 1 {
				t.Fatalf("run exit = %d, stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("failed run wrote success output: %s", stdout.String())
			}
			var envelope map[string]any
			if err = json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
				t.Fatalf("decode run error: %q: %v", stderr.String(), err)
			}
			if envelope["command"] != "run" || envelope["error"].(map[string]any)["code"] != "runtime_error" {
				t.Fatalf("run error envelope = %#v", envelope)
			}

			database, err = store.OpenReadOnly(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			var openRuns int
			if err = database.DB.QueryRowContext(ctx, "SELECT count(*) FROM usage_runs WHERE ended_at IS NULL").Scan(&openRuns); err != nil || openRuns != 0 {
				t.Fatalf("open runs = %d, err=%v", openRuns, err)
			}
			if err = database.Close(); err != nil {
				t.Fatal(err)
			}

			if err = os.WriteFile(filepath.Join(bin, client), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
				t.Fatal(err)
			}
			userHomeDir = func() (string, error) { return "", errors.New("synthetic home failure") }
			stdout.Reset()
			stderr.Reset()
			if exit := execute([]string{"--state-dir", state, "--format", "json", "run", client, "--", "synthetic"}, bytes.NewReader(nil), &stdout, &stderr); exit != 1 {
				t.Fatalf("cleanup failure exit = %d, stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
			}
			database, err = store.OpenReadOnly(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			var endedAt, reason string
			var exact int
			if err = database.DB.QueryRowContext(ctx, "SELECT ended_at,exact,ambiguity_reason FROM usage_runs ORDER BY id DESC LIMIT 1").Scan(&endedAt, &exact, &reason); err != nil {
				t.Fatal(err)
			}
			if endedAt == "" || exact != 0 || reason != "wrapper_cleanup_failed" {
				t.Fatalf("failed cleanup run = ended:%q exact:%d reason:%q", endedAt, exact, reason)
			}
		})
	}
}

func TestUsageCommandTextAndJSONContracts(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	var text bytes.Buffer
	if err := run([]string{"--state-dir", state, "usage", "diagnose"}, bytes.NewReader(nil), &text); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(text.Bytes(), []byte("USAGE DIAGNOSTICS")) || !bytes.Contains(text.Bytes(), []byte("| events")) || bytes.HasPrefix(bytes.TrimSpace(text.Bytes()), []byte("{")) {
		t.Fatalf("text output = %s", text.String())
	}
	var encoded bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "usage", "summary"}, bytes.NewReader(nil), &encoded); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(encoded.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["command"] != "usage.summary" || envelope["schema_version"] != float64(1) {
		t.Fatalf("envelope = %#v", envelope)
	}
}

type recordingUsageProgress struct {
	starts, stops int
	updates       []usage.ScanProgress
	stderr        io.Writer
}

func (r *recordingUsageProgress) Start() { r.starts++ }
func (r *recordingUsageProgress) Update(progress usage.ScanProgress) {
	r.updates = append(r.updates, progress)
}
func (r *recordingUsageProgress) Stop() { r.stops++ }

func TestUsageCommandsUseProgressForExplicitAndImplicitScans(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	oldFactory := newUsageProgress
	var reporters []*recordingUsageProgress
	var quietValues []bool
	newUsageProgress = func(stderr io.Writer, quiet bool) usage.ScanProgressReporter {
		reporter := &recordingUsageProgress{stderr: stderr}
		reporters = append(reporters, reporter)
		quietValues = append(quietValues, quiet)
		return reporter
	}
	t.Cleanup(func() { newUsageProgress = oldFactory })
	for _, args := range [][]string{
		{"--state-dir", state, "usage", "scan"},
		{"--state-dir", state, "usage", "rebuild"},
		{"--state-dir", state, "usage", "stats"},
		{"--state-dir", state, "usage", "summary"},
		{"--state-dir", state, "usage", "stats", "--no-scan"},
		{"--state-dir", state, "usage", "summary", "--no-scan"},
		{"--state-dir", state, "--quiet", "usage", "scan"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := execute(args, bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q", args, exit, stdout.String(), stderr.String())
		}
	}
	if len(reporters) != 7 {
		t.Fatalf("reporters=%d want 7", len(reporters))
	}
	for _, index := range []int{0, 1, 2, 3, 6} {
		reporter := reporters[index]
		if reporter.starts != 1 || reporter.stops != 1 || len(reporter.updates) == 0 {
			t.Fatalf("command %d progress starts=%d stops=%d updates=%#v", index, reporter.starts, reporter.stops, reporter.updates)
		}
	}
	if got := reporters[2].updates[0]; got != (usage.ScanProgress{}) {
		t.Fatalf("usage stats did not run its pre-scan: first progress=%#v", got)
	}
	if got := reporters[3].updates[0]; got != (usage.ScanProgress{}) {
		t.Fatalf("usage summary did not run its pre-scan: first progress=%#v", got)
	}
	for _, index := range []int{4, 5} {
		reporter := reporters[index]
		if reporter.starts != 0 || reporter.stops != 0 || len(reporter.updates) != 0 {
			t.Fatalf("command %d --no-scan progress starts=%d stops=%d updates=%#v", index, reporter.starts, reporter.stops, reporter.updates)
		}
	}
	if quietValues[6] != true {
		t.Fatalf("--quiet did not reach progress output: %v", quietValues)
	}
}

func TestUsageNoScanUsesStoredAggregateUntilDefaultScan(t *testing.T) {
	initial := []byte(`{"type":"session_meta","payload":{"session_id":"no-scan"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"first","model":"gpt-5.4"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-07-20T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10}}}}` + "\n")
	appended := []byte(`{"type":"turn_context","payload":{"turn_id":"second","model":"gpt-5.4"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-07-20T00:01:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20}}}}` + "\n")
	tests := []struct {
		name   string
		args   func(state string, noScan bool) []string
		decode func(t *testing.T, data []byte) (int64, int64, bool, []string)
	}{
		{
			name: "stats",
			args: func(state string, noScan bool) []string {
				args := []string{"--state-dir", state, "--format", "json", "usage", "stats", "--period", "all"}
				if noScan {
					args = append(args, "--no-scan")
				}
				return args
			},
			decode: func(t *testing.T, data []byte) (int64, int64, bool, []string) {
				t.Helper()
				var envelope struct {
					Data     usage.StatsReport `json:"data"`
					Partial  bool              `json:"partial"`
					Warnings []string          `json:"warnings"`
				}
				if err := json.Unmarshal(data, &envelope); err != nil {
					t.Fatal(err)
				}
				return envelope.Data.Totals.Events, envelope.Data.Totals.InputTokens, envelope.Partial, envelope.Warnings
			},
		},
		{
			name: "summary",
			args: func(state string, noScan bool) []string {
				args := []string{"--state-dir", state, "--format", "json", "usage", "summary"}
				if noScan {
					args = append(args, "--no-scan")
				}
				return args
			},
			decode: func(t *testing.T, data []byte) (int64, int64, bool, []string) {
				t.Helper()
				var envelope struct {
					Data     usage.Summary `json:"data"`
					Partial  bool          `json:"partial"`
					Warnings []string      `json:"warnings"`
				}
				if err := json.Unmarshal(data, &envelope); err != nil {
					t.Fatal(err)
				}
				return envelope.Data.Counts["events"], envelope.Data.Tokens["input_tokens"], envelope.Partial, envelope.Warnings
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			state := filepath.Join(root, "state")
			home := filepath.Join(root, "home")
			source := filepath.Join(home, ".codex", "sessions", "fixture.jsonl")
			if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(source, initial, 0o600); err != nil {
				t.Fatal(err)
			}
			database, err := store.Open(context.Background(), state)
			if err != nil {
				t.Fatal(err)
			}
			service := usage.New(database, home)
			if result, err := service.Scan(context.Background()); err != nil || result["imported"] != 1 {
				database.Close()
				t.Fatalf("initial scan=%#v err=%v", result, err)
			}
			if err := database.Close(); err != nil {
				t.Fatal(err)
			}
			file, err := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.Write(appended); err != nil {
				file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			oldHome := userHomeDir
			userHomeDir = func() (string, error) { return home, nil }
			t.Cleanup(func() { userHomeDir = oldHome })
			runJSON := func(noScan bool) (int64, int64, bool, []string) {
				t.Helper()
				var stdout, stderr bytes.Buffer
				if exit := execute(test.args(state, noScan), bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
					t.Fatalf("args=%v exit=%d stdout=%q stderr=%q", test.args(state, noScan), exit, stdout.String(), stderr.String())
				}
				return test.decode(t, stdout.Bytes())
			}
			assertReport := func(label string, wantEvents, wantTokens int64) {
				t.Helper()
				events, tokens, partial, warnings := runJSON(label == "no-scan")
				if events != wantEvents || tokens != wantTokens || partial || len(warnings) != 0 {
					t.Fatalf("%s events=%d tokens=%d partial=%t warnings=%#v; want events=%d tokens=%d partial=false warnings=[]", label, events, tokens, partial, warnings, wantEvents, wantTokens)
				}
			}
			assertReport("no-scan", 1, 10)
			assertReport("default", 2, 30)
		})
	}
}

type stderrProbeProgress struct{ stderr io.Writer }

func (p *stderrProbeProgress) Start()                    { _, _ = io.WriteString(p.stderr, "progress-probe\n") }
func (p *stderrProbeProgress) Update(usage.ScanProgress) {}
func (p *stderrProbeProgress) Stop()                     {}

func TestUsageProgressReporterUsesStderrAndPreservesJSONStdout(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	oldFactory := newUsageProgress
	var reporter *stderrProbeProgress
	newUsageProgress = func(stderr io.Writer, quiet bool) usage.ScanProgressReporter {
		if quiet {
			t.Fatal("JSON usage scan unexpectedly requested quiet progress")
		}
		reporter = &stderrProbeProgress{stderr: stderr}
		return reporter
	}
	t.Cleanup(func() { newUsageProgress = oldFactory })
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"--state-dir", state, "--format", "json", "usage", "scan"}, bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	if reporter == nil || reporter.stderr != io.Writer(&stderr) {
		t.Fatalf("progress reporter stderr=%#v want command stderr=%#v", reporter, &stderr)
	}
	if got := stderr.String(); got != "progress-probe\n" {
		t.Fatalf("stderr=%q", got)
	}
	if strings.Contains(stdout.String(), "progress-probe") {
		t.Fatalf("JSON stdout was polluted by progress: %q", stdout.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || envelope["command"] != "usage.scan" {
		t.Fatalf("JSON stdout=%q envelope=%#v err=%v", stdout.String(), envelope, err)
	}
}

type manualUsageProgressTimer struct{ channel chan time.Time }

func (timer *manualUsageProgressTimer) Chan() <-chan time.Time { return timer.channel }
func (timer *manualUsageProgressTimer) Stop()                  {}

type manualUsageProgressTicker struct{ channel chan time.Time }

func (ticker *manualUsageProgressTicker) Chan() <-chan time.Time { return ticker.channel }
func (ticker *manualUsageProgressTicker) Stop()                  {}

type manualUsageProgressClock struct {
	mu            sync.Mutex
	timer         *manualUsageProgressTimer
	ticker        *manualUsageProgressTicker
	tickerCreated chan struct{}
}

func newManualUsageProgressClock() *manualUsageProgressClock {
	return &manualUsageProgressClock{tickerCreated: make(chan struct{})}
}

func (clock *manualUsageProgressClock) NewTimer(time.Duration) usageProgressTimer {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.timer = &manualUsageProgressTimer{channel: make(chan time.Time, 1)}
	return clock.timer
}

func (clock *manualUsageProgressClock) NewTicker(time.Duration) usageProgressTicker {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.ticker = &manualUsageProgressTicker{channel: make(chan time.Time, 1)}
	close(clock.tickerCreated)
	return clock.ticker
}

func (clock *manualUsageProgressClock) fireTimer(t *testing.T) {
	t.Helper()
	clock.mu.Lock()
	timer := clock.timer
	clock.mu.Unlock()
	if timer == nil {
		t.Fatal("progress timer was not created")
	}
	timer.channel <- time.Time{}
}

func (clock *manualUsageProgressClock) waitForTicker(t *testing.T) {
	t.Helper()
	select {
	case <-clock.tickerCreated:
	case <-time.After(time.Second):
		t.Fatal("progress ticker was not created")
	}
}

func (clock *manualUsageProgressClock) fireTicker(t *testing.T) {
	t.Helper()
	clock.mu.Lock()
	ticker := clock.ticker
	clock.mu.Unlock()
	if ticker == nil {
		t.Fatal("progress ticker was not created")
	}
	ticker.channel <- time.Time{}
}

type synchronizedProgressWriter struct {
	mu       sync.Mutex
	contents string
	writes   chan struct{}
}

func newSynchronizedProgressWriter() *synchronizedProgressWriter {
	return &synchronizedProgressWriter{writes: make(chan struct{}, 8)}
}

func (writer *synchronizedProgressWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	writer.contents += string(data)
	writer.mu.Unlock()
	select {
	case writer.writes <- struct{}{}:
	default:
	}
	return len(data), nil
}

func (writer *synchronizedProgressWriter) String() string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.contents
}

func (writer *synchronizedProgressWriter) waitForWrite(t *testing.T) {
	t.Helper()
	select {
	case <-writer.writes:
	case <-time.After(time.Second):
		t.Fatal("progress output was not written")
	}
}

func TestUsageProgressOutputUsesControllableTimerAndTicker(t *testing.T) {
	clock := newManualUsageProgressClock()
	writer := newSynchronizedProgressWriter()
	progress := newUsageProgressOutputWithClock(writer, false, false, clock)
	progress.Start()
	progress.Update(usage.ScanProgress{Processed: 1, Total: 3})
	if got := writer.String(); got != "" {
		t.Fatalf("progress emitted before timer: %q", got)
	}
	clock.fireTimer(t)
	writer.waitForWrite(t)
	clock.waitForTicker(t)
	progress.Update(usage.ScanProgress{Processed: 2, Total: 3})
	clock.fireTicker(t)
	writer.waitForWrite(t)
	progress.Update(usage.ScanProgress{Processed: 3, Total: 3})
	progress.Stop()
	if got, want := writer.String(), "usage scan: 1/3 source files\nusage scan: 2/3 source files\nusage scan: 3/3 source files\n"; got != want || strings.Contains(got, "\x1b[") {
		t.Fatalf("non-TTY progress=%q want=%q", got, want)
	}
}

func TestUsageProgressOutputTTYFinalRedrawAndQuiet(t *testing.T) {
	clock := newManualUsageProgressClock()
	writer := newSynchronizedProgressWriter()
	progress := newUsageProgressOutputWithClock(writer, false, true, clock)
	progress.Start()
	progress.Update(usage.ScanProgress{Processed: 1, Total: 2})
	clock.fireTimer(t)
	writer.waitForWrite(t)
	progress.Update(usage.ScanProgress{Processed: 2, Total: 2})
	progress.Stop()
	if got, want := writer.String(), "\r\x1b[2Kusage scan: 1/2 source files\r\x1b[2Kusage scan: 2/2 source files\n"; got != want {
		t.Fatalf("TTY progress=%q want=%q", got, want)
	}
	quietClock := newManualUsageProgressClock()
	quietWriter := newSynchronizedProgressWriter()
	quiet := newUsageProgressOutputWithClock(quietWriter, true, false, quietClock)
	quiet.Start()
	quiet.Update(usage.ScanProgress{Processed: 1, Total: 2})
	quiet.Stop()
	if got := quietWriter.String(); got != "" {
		t.Fatalf("quiet progress=%q", got)
	}
	quietClock.mu.Lock()
	timerCreated := quietClock.timer != nil
	quietClock.mu.Unlock()
	if timerCreated {
		t.Fatal("quiet progress created a timer")
	}
}

func TestUsageProgressOutputFastStopBeforeInitialTimerStaysSilent(t *testing.T) {
	clock := newManualUsageProgressClock()
	writer := newSynchronizedProgressWriter()
	progress := newUsageProgressOutputWithClock(writer, false, false, clock)
	progress.Start()
	progress.Update(usage.ScanProgress{Processed: 1, Total: 2})
	progress.Stop()
	if got := writer.String(); got != "" {
		t.Fatalf("fast-stop progress=%q", got)
	}
	clock.mu.Lock()
	tickerCreated := clock.ticker != nil
	clock.mu.Unlock()
	if tickerCreated {
		t.Fatal("fast-stop progress created a refresh ticker")
	}
}

func TestUsageProgressOutputShowsParserVersionRereadReason(t *testing.T) {
	clock := newManualUsageProgressClock()
	writer := newSynchronizedProgressWriter()
	progress := newUsageProgressOutputWithClock(writer, false, false, clock)
	progress.Start()
	progress.Update(usage.ScanProgress{Processed: 1, Total: 1, Reason: usage.ParserVersionRereadReason})
	clock.fireTimer(t)
	writer.waitForWrite(t)
	progress.Stop()
	want := "usage scan: 1/1 source files (re-reading after parser update)\nusage scan: 1/1 source files (re-reading after parser update)\n"
	if got := writer.String(); got != want {
		t.Fatalf("parser-version progress=%q want=%q", got, want)
	}
}

func TestUsageSummaryAndSessionsUseReadableASCIITables(t *testing.T) {
	baseCost, providerCost := "0.100000000", "0.200000000"
	summary := usage.Summary{
		Tokens:               map[string]int64{"input_tokens": 10, "cached_input_tokens": 3, "output_tokens": 2},
		Counts:               map[string]int64{"events": 2, "exact": 1, "estimated": 1, "historical": 0, "priced": 1, "unpriced": 1},
		KnownCatalogBaseCost: &baseCost,
		KnownProviderCost:    &providerCost,
		Warnings:             []string{"estimated attribution"},
		Unpriced:             []string{"unknown_model"},
		Models:               []usage.ModelCoverage{{Client: "codex", Model: "gpt-5.4", Events: 1, PricedEvents: 1}, {Client: "codex", Model: "codex-auto-review", Events: 1, UnpricedEvents: 1}},
	}
	var rendered bytes.Buffer
	if err := renderUsageText(&rendered, "usage.summary", summary); err != nil {
		t.Fatal(err)
	}
	text := rendered.String()
	for _, want := range []string{"📊 USAGE SUMMARY", "🪙 TOKEN TOTALS", "🧾 MODEL COVERAGE", "| METRIC", "| TOKEN", "| CLIENT | MODEL", "known catalog subtotal", "codex-auto-review"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage summary missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "input_tokens=") {
		t.Fatalf("usage summary retained packed token text:\n%s", text)
	}

	values := []usage.SessionSummary{{
		Client:               "codex",
		SessionID:            "session-1",
		FirstAt:              "2026-07-16T00:00:00Z",
		LastAt:               "2026-07-16T00:01:00Z",
		Tokens:               map[string]int64{"input_tokens": 10, "cached_input_tokens": 3, "output_tokens": 2},
		KnownCatalogBaseCost: &baseCost,
		KnownProviderCost:    &providerCost,
		Warnings:             []string{"estimated"},
		Unpriced:             []string{"unknown_model"},
	}}
	rendered.Reset()
	if err := renderUsageText(&rendered, "usage.sessions", values); err != nil {
		t.Fatal(err)
	}
	text = rendered.String()
	if !strings.HasPrefix(text, "📚 USAGE SESSIONS\n+") || !strings.Contains(text, "| CLIENT | SESSION") || !strings.Contains(text, "| INPUT") || !strings.Contains(text, "| CACHED") || !strings.Contains(text, "| OUTPUT") || strings.Contains(text, "input_tokens=") {
		t.Fatalf("usage sessions are not rendered as the shared ASCII table:\n%s", text)
	}
}

func TestPhase6BackupAndDoctorCLIContracts(t *testing.T) {
	oldVersion := buildinfo.Version
	buildinfo.Version = "v1.2.3-backup"
	t.Cleanup(func() { buildinfo.Version = oldVersion })

	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "source")
	database, err := store.Open(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	sourceVault := credentialvault.New(source, machineIdentity)
	providerService := provider.Service{Store: database, Vault: sourceVault}
	if _, err = providerService.Add(ctx, provider.Definition{Name: "synthetic", Endpoint: "https://example.invalid", Clients: []provider.Client{provider.ClientCodex}}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	archive := filepath.Join(source, "backups", "portable", "phase6.adb")
	var output bytes.Buffer
	if err = run([]string{"--state-dir", source, "--format", "json", "backup", "create", archive}, bytes.NewBufferString("passphrase\n"), &output); err != nil {
		t.Fatal(err)
	}
	assertCommandEnvelope(t, output.Bytes(), "backup.create")
	var created struct {
		Data struct {
			Manifest struct {
				AgentDeckVersion string `json:"agentdeck_version"`
			} `json:"manifest"`
		} `json:"data"`
	}
	if err = json.Unmarshal(output.Bytes(), &created); err != nil || created.Data.Manifest.AgentDeckVersion != "v1.2.3-backup" {
		t.Fatalf("backup build provenance = %#v, %v", created, err)
	}
	var failedOutput, failedError bytes.Buffer
	if exit := execute([]string{"--state-dir", source, "--format", "json", "backup", "create", archive}, bytes.NewBufferString("passphrase\n"), &failedOutput, &failedError); exit != 1 {
		t.Fatalf("existing backup exit = %d, stdout=%s stderr=%s", exit, failedOutput.String(), failedError.String())
	}
	if failedOutput.Len() != 0 {
		t.Fatalf("existing backup wrote success output: %s", failedOutput.String())
	}
	var errorEnvelope map[string]any
	if err = json.Unmarshal(failedError.Bytes(), &errorEnvelope); err != nil || errorEnvelope["error"].(map[string]any)["code"] != "backup_exists" {
		t.Fatalf("existing backup error = %#v, %v", errorEnvelope, err)
	}
	output.Reset()
	if err = run([]string{"--state-dir", source, "--format", "json", "backup", "list"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	assertCommandEnvelope(t, output.Bytes(), "backup.list")
	output.Reset()
	if err = run([]string{"--format", "json", "backup", "inspect", archive}, bytes.NewBufferString("passphrase\n"), &output); err != nil {
		t.Fatal(err)
	}
	assertCommandEnvelope(t, output.Bytes(), "backup.inspect")

	target := filepath.Join(t.TempDir(), "target")
	output.Reset()
	if err = run([]string{"--state-dir", target, "--format", "json", "backup", "restore", archive}, bytes.NewBufferString("passphrase\n"), &output); err != nil {
		t.Fatal(err)
	}
	assertCommandEnvelope(t, output.Bytes(), "backup.restore")
	restored, err := store.OpenReadOnly(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := restored.ProviderCredential(ctx, "synthetic", "default")
	if err != nil {
		restored.Close()
		t.Fatal(err)
	}
	secret, err := restored.CredentialSecret(ctx, credential.ID)
	if err != nil {
		restored.Close()
		t.Fatal(err)
	}
	value, err := credentialvault.New(target, machineIdentity).Open(ctx, credential.CredentialRef, credentialvault.Sealed{Algorithm: secret.Algorithm, KeyVersion: secret.KeyVersion, KeyID: secret.KeyID, Nonce: secret.Nonce, Ciphertext: secret.Ciphertext})
	if closeErr := restored.Close(); err == nil {
		err = closeErr
	}
	if err != nil || value != "synthetic-secret" {
		t.Fatalf("restored secret = %q, %v", value, err)
	}

	output.Reset()
	if err = run([]string{"--state-dir", source, "--format", "json", "doctor", "--full"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	assertCommandEnvelope(t, output.Bytes(), "doctor")
	assertExtensionCLIErrorArgs(t, []string{"--format", "json", "backup", "inspect"}, 2, "backup.inspect", "invalid_argument")
}

func TestReadPassphraseFromOneLine(t *testing.T) {
	value, err := readPassphrase(bytes.NewBufferString("correct horse battery staple\nignored\n"))
	if err != nil || value != "correct horse battery staple" {
		t.Fatalf("readPassphrase = %q, %v", value, err)
	}
	if _, err = readPassphrase(bytes.NewReader(nil)); !isInputError(err) {
		t.Fatalf("empty passphrase error = %v", err)
	}
}

func TestPhase6RejectsNDJSONBeforeAnyBackupOrDoctorSideEffect(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	archive := filepath.Join(t.TempDir(), "portable.adb")
	for _, test := range []struct {
		args    []string
		command string
	}{
		{[]string{"--state-dir", state, "--format", "ndjson", "backup", "create", archive}, "backup.create"},
		{[]string{"--state-dir", state, "--format", "ndjson", "doctor"}, "doctor"},
	} {
		assertExtensionCLIErrorArgs(t, test.args, 2, test.command, "invalid_argument")
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("ndjson rejection created state: %v", err)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("ndjson rejection created archive: %v", err)
	}
}

func TestDoctorCLIReportsExactSchemaMatrixWithoutSQLLeakage(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name, checkName, status, code, recovery string
		version, count                          int
		drop                                    bool
	}{
		{"schema12", "schema", "warning", "schema_outdated", "agentdeck state migrate", 12, 12, true},
		{"schema_current", "schema", "ok", "", "", store.CurrentSchemaVersion, store.CurrentSchemaVersion, false},
		{"schema_current_missing_tool_calls", "schema", "error", "schema_incompatible", "", store.CurrentSchemaVersion, store.CurrentSchemaVersion, true},
		{"future", "database", "error", "unknown_schema", "", 99, 0, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, full := range []bool{false, true} {
				state := filepath.Join(t.TempDir(), "state")
				database, err := store.Open(ctx, state)
				if err != nil {
					t.Fatal(err)
				}
				if test.drop {
					if _, err = database.Exec(ctx, "DROP TABLE usage_tool_calls"); err != nil {
						database.Close()
						t.Fatal(err)
					}
				}
				if test.version != store.CurrentSchemaVersion {
					if _, err = database.Exec(ctx, "UPDATE schema_metadata SET version=?", test.version); err != nil {
						database.Close()
						t.Fatal(err)
					}
				}
				if err = database.Close(); err != nil {
					t.Fatal(err)
				}
				args := []string{"--state-dir", state}
				if full {
					args = append(args, "doctor", "--full")
				} else {
					args = append(args, "doctor")
				}
				var textOutput bytes.Buffer
				if err = run(args, bytes.NewReader(nil), &textOutput); err != nil {
					t.Fatal(err)
				}
				text := strings.ToLower(textOutput.String())
				for _, forbidden := range []string{"select ", "no such table", "sqlite", "driver error"} {
					if strings.Contains(text, forbidden) {
						t.Fatalf("full=%t text leaked %q: %s", full, forbidden, textOutput.String())
					}
				}
				if !strings.Contains(textOutput.String(), test.checkName+": "+test.status) || !strings.Contains(textOutput.String(), test.code) || (test.count != 0 && !strings.Contains(textOutput.String(), fmt.Sprintf("count=%d", test.count))) || !strings.Contains(textOutput.String(), test.recovery) {
					t.Fatalf("full=%t text schema output = %s", full, textOutput.String())
				}
				jsonArgs := append([]string{"--format", "json"}, args...)
				var jsonOutput bytes.Buffer
				if err = run(jsonArgs, bytes.NewReader(nil), &jsonOutput); err != nil {
					t.Fatal(err)
				}
				jsonText := strings.ToLower(jsonOutput.String())
				for _, forbidden := range []string{"select ", "no such table", "sqlite", "driver error"} {
					if strings.Contains(jsonText, forbidden) {
						t.Fatalf("full=%t JSON leaked %q: %s", full, forbidden, jsonOutput.String())
					}
				}
				var envelope struct {
					Data doctor.Report `json:"data"`
				}
				if err = json.Unmarshal(jsonOutput.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				var matched *doctor.Check
				for i := range envelope.Data.Checks {
					check := &envelope.Data.Checks[i]
					if check.Name == "usage_tool_calls" {
						t.Fatalf("full=%t contradictory JSON check: %#v", full, envelope.Data.Checks)
					}
					if check.Name == test.checkName {
						matched = check
					}
				}
				if matched == nil || matched.Status != test.status || matched.Code != test.code || matched.Count != test.count || matched.Recovery != test.recovery {
					t.Fatalf("full=%t JSON check=%#v", full, matched)
				}
			}
		})
	}
}

func TestStateMigrateTextAndJSONUpgradeSchema12(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, "DROP TABLE usage_tool_calls; DROP INDEX usage_events_client_session; UPDATE schema_metadata SET version=12"); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	var textOutput bytes.Buffer
	if err = run([]string{"--state-dir", state, "state", "migrate"}, bytes.NewReader(nil), &textOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOutput.String(), "Completed state.migrate.") {
		t.Fatalf("state migrate text = %s", textOutput.String())
	}
	database, err = store.OpenReadOnly(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	version, err := database.SchemaVersion(ctx)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	var tableCount int
	if err = database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='usage_tool_calls'").Scan(&tableCount); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	if version != store.CurrentSchemaVersion || tableCount != 1 {
		t.Fatalf("migrated schema version=%d tool table count=%d", version, tableCount)
	}
	var jsonOutput bytes.Buffer
	if err = run([]string{"--state-dir", state, "--format", "json", "state", "migrate"}, bytes.NewReader(nil), &jsonOutput); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data map[string]bool `json:"data"`
	}
	if err = json.Unmarshal(jsonOutput.Bytes(), &envelope); err != nil || !envelope.Data["migrated"] {
		t.Fatalf("state migrate JSON = %s, %v", jsonOutput.String(), err)
	}
}

func TestCobraSyntaxErrorsUseJSONInvalidArgumentExitCode(t *testing.T) {
	assertExtensionCLIErrorArgs(t, []string{"--format", "json", "--bogus", "doctor"}, 2, "agentdeck", "invalid_argument")
	assertExtensionCLIErrorArgs(t, []string{"--format", "json", "unknown-command"}, 2, "agentdeck", "invalid_argument")
	assertExtensionCLIErrorArgs(t, []string{"--format", "json", "run", "codex", "phase7"}, 2, "run", "invalid_argument")
}

func TestLoadWatchFingerprintsDoesNotWriteExistingState(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.SetSetting(ctx, "watch.fingerprint.extension", "stable"); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, "agentdeck.sqlite3")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fingerprints, err := loadWatchFingerprints(ctx, state)
	if err != nil || fingerprints["extension"] != "stable" {
		t.Fatalf("loadWatchFingerprints = %#v, %v", fingerprints, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("loadWatchFingerprints wrote database: %v", err)
	}
}

func assertCommandEnvelope(t *testing.T, contents []byte, command string) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(contents, &envelope); err != nil {
		t.Fatalf("decode %s: %q: %v", command, contents, err)
	}
	if envelope["command"] != command || envelope["schema_version"] != float64(1) {
		t.Fatalf("%s envelope = %#v", command, envelope)
	}
}

func TestExtensionScanCommandUsesSyntheticHomeAndStableJSON(t *testing.T) {
	state, home := filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "home")
	config := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("[mcp_servers.github]\ncommand = 'synthetic'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	var output bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "extension", "scan"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["command"] != "extension.scan" || envelope["schema_version"] != float64(1) {
		t.Fatalf("envelope = %#v", envelope)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok || data["diagnostics"] == nil {
		t.Fatalf("scan data = %#v", envelope["data"])
	}
	for _, args := range [][]string{{"extension", "list"}, {"extension", "show", "codex:mcp:user:github"}, {"extension", "doctor"}} {
		output.Reset()
		if err := run(append([]string{"--state-dir", state, "--format", "json"}, args...), bytes.NewReader(nil), &output); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if bytes.Contains(output.Bytes(), []byte(`"ID"`)) || !bytes.Contains(output.Bytes(), []byte(`"id"`)) && args[1] != "doctor" {
			t.Fatalf("unstable DTO %v: %s", args, output.String())
		}
	}
	output.Reset()
	if err := run([]string{"--state-dir", state, "--format", "json", "extension", "adopt", "codex:mcp:user:github"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	adopted := envelope["data"].(map[string]any)
	if adopted["managed"] != true || adopted["drift"] != false {
		t.Fatalf("adopt data = %#v", adopted)
	}

	assertExtensionCLIError(t, state, []string{"extension", "show", "missing"}, "extension.show", "extension_not_found")
	for _, args := range [][]string{
		{"--state-dir", state, "--format", "json", "extension", "show", "missing"},
		{"--state-dir", state, "extension", "--format", "json", "show", "missing"},
		{"--state-dir", state, "extension", "show", "missing", "--format=json"},
	} {
		assertExtensionCLIErrorArgs(t, args, 1, "extension.show", "extension_not_found")
	}
	for _, test := range []struct {
		args    []string
		command string
	}{
		{[]string{"extension", "show"}, "extension.show"},
		{[]string{"extension", "show", "one", "two"}, "extension.show"},
		{[]string{"extension", "adopt"}, "extension.adopt"},
		{[]string{"extension", "enable", "one", "two"}, "extension.enable"},
		{[]string{"extension", "disable"}, "extension.disable"},
	} {
		args := append([]string{"--state-dir", state, "--format", "json"}, test.args...)
		assertExtensionCLIErrorArgs(t, args, 2, test.command, "invalid_argument")
	}
	before, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	assertExtensionCLIError(t, state, []string{"extension", "disable", "codex:mcp:user:github"}, "extension.disable", extension.ErrReadOnly.Error())
	after, _ := os.ReadFile(config)
	if !bytes.Equal(before, after) {
		t.Fatal("disable changed native config")
	}
}

func assertExtensionCLIError(t *testing.T, state string, args []string, command, code string) {
	t.Helper()
	fullArgs := append([]string{"--state-dir", state, "--format", "json"}, args...)
	assertExtensionCLIErrorArgs(t, fullArgs, 1, command, code)
}

func assertExtensionCLIErrorArgs(t *testing.T, args []string, wantExit int, command, code string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if exitCode := execute(args, bytes.NewReader(nil), &stdout, &stderr); exitCode != wantExit {
		t.Fatalf("execute(%v) exit code = %d, want %d", args, exitCode, wantExit)
	}
	if stdout.Len() != 0 {
		t.Fatalf("execute(%v) stdout = %s", args, stdout.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("execute(%v) error JSON = %q: %v", args, stderr.String(), err)
	}
	errorData, ok := envelope["error"].(map[string]any)
	if !ok || envelope["command"] != command || errorData["code"] != code {
		t.Fatalf("execute(%v) envelope = %#v", args, envelope)
	}
}

func TestReadCredentialFromPipe(t *testing.T) {
	var prompt bytes.Buffer
	value, err := readCredential(bytes.NewBufferString("synthetic-secret\nignored\n"), &prompt, "agentdeck:test")
	if err != nil || value != "synthetic-secret" {
		t.Fatalf("readCredential = %q, %v", value, err)
	}
	if prompt.Len() != 0 {
		t.Fatalf("non-interactive prompt = %q", prompt.String())
	}
	if _, err = readCredential(bytes.NewReader(nil), &prompt, "agentdeck:test"); !isInputError(err) {
		t.Fatalf("empty credential error = %v", err)
	}
}

func TestReadCredentialFromTerminalWithoutEcho(t *testing.T) {
	terminal, err := os.CreateTemp(t.TempDir(), "terminal")
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()

	oldIsTerminal, oldReadPassword := credentialIsTerminal, credentialReadPassword
	credentialIsTerminal = func(*os.File) bool { return true }
	credentialReadPassword = func(fd int) ([]byte, error) {
		if fd != int(terminal.Fd()) {
			t.Fatalf("read password fd = %d", fd)
		}
		return []byte("terminal-secret"), nil
	}
	t.Cleanup(func() {
		credentialIsTerminal, credentialReadPassword = oldIsTerminal, oldReadPassword
	})

	var prompt bytes.Buffer
	value, err := readCredential(terminal, &prompt, "codex-pro")
	if err != nil || value != "terminal-secret" {
		t.Fatalf("readCredential = %q, %v", value, err)
	}
	if prompt.String() != "Credential for codex-pro: \n" || strings.Contains(prompt.String(), value) {
		t.Fatalf("terminal prompt = %q", prompt.String())
	}
}

func TestProviderAddReadsTerminalCredentialWithoutDisclosure(t *testing.T) {
	terminal, err := os.CreateTemp(t.TempDir(), "terminal")
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()

	oldIsTerminal, oldReadPassword := credentialIsTerminal, credentialReadPassword
	credentialIsTerminal = func(*os.File) bool { return true }
	credentialReadPassword = func(int) ([]byte, error) { return []byte("terminal-secret"), nil }
	t.Cleanup(func() {
		credentialIsTerminal, credentialReadPassword = oldIsTerminal, oldReadPassword
	})

	state := filepath.Join(t.TempDir(), "state")
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"--state-dir", state, "provider", "add", "work", "--endpoint", "https://example.invalid", "--clients", "codex", "--credential", "pro"}, terminal, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("provider add exit = %d, stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "terminal-secret") || strings.Contains(stderr.String(), "terminal-secret") {
		t.Fatalf("credential disclosed: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if stderr.String() != "Credential for work-pro-ref: \n" {
		t.Fatalf("credential prompt = %q", stderr.String())
	}
	database, getErr := store.OpenReadOnly(context.Background(), state)
	if getErr != nil {
		t.Fatal(getErr)
	}
	credential, getErr := database.ProviderCredential(context.Background(), "work", "pro")
	if getErr != nil {
		database.Close()
		t.Fatal(getErr)
	}
	secret, getErr := database.CredentialSecret(context.Background(), credential.ID)
	if getErr != nil {
		database.Close()
		t.Fatal(getErr)
	}
	value, getErr := credentialvault.New(state, machineIdentity).Open(context.Background(), credential.CredentialRef, credentialvault.Sealed{Algorithm: secret.Algorithm, KeyVersion: secret.KeyVersion, KeyID: secret.KeyID, Nonce: secret.Nonce, Ciphertext: secret.Ciphertext})
	if closeErr := database.Close(); getErr == nil {
		getErr = closeErr
	}
	if getErr != nil || value != "terminal-secret" {
		t.Fatalf("stored credential = %q, %v", value, getErr)
	}
}

func TestProviderAddExistingProviderAddsCredentialAndIdenticalRetryDoesNotPrompt(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")

	runAdd := func(stdin string, args ...string) (string, string, int) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		exit := execute(append([]string{"--state-dir", state, "provider", "add", "sssaicode"}, args...), bytes.NewBufferString(stdin), &stdout, &stderr)
		return stdout.String(), stderr.String(), exit
	}
	if _, stderr, exit := runAdd("claude-secret\n", "--credential", "claude", "--endpoint", "https://claude.example/v1", "--clients", "claude"); exit != 0 {
		t.Fatalf("initial add exit=%d stderr=%s", exit, stderr)
	}
	if _, stderr, exit := runAdd("codex-secret\n", "--credential", "codex", "--endpoint", "https://codex.example/api/v1", "--clients", "codex", "--multiplier", "0.4"); exit != 0 {
		t.Fatalf("credential add exit=%d stderr=%s", exit, stderr)
	}
	if _, stderr, exit := runAdd("", "--credential", "codex", "--endpoint", "https://codex.example/api", "--clients", "codex", "--multiplier", "0.4"); exit != 0 || stderr != "" {
		t.Fatalf("idempotent add exit=%d stderr=%q", exit, stderr)
	}

	var list bytes.Buffer
	if err := run([]string{"--state-dir", state, "credential", "list"}, bytes.NewReader(nil), &list); err != nil {
		t.Fatal(err)
	}
	text := list.String()
	for _, want := range []string{"PROVIDER", "ENDPOINT", "MULTIPLIER", "sssaicode", "sssaicode-codex-ref", "https://codex.example/api", "0.400000000000"} {
		if !strings.Contains(text, want) {
			t.Fatalf("credential list missing %q:\n%s", want, text)
		}
	}
	root := newRootCommand(bytes.NewReader(nil), &bytes.Buffer{})
	providerAdd, _, err := root.Find([]string{"provider", "add"})
	if err != nil {
		t.Fatal(err)
	}
	if providerAdd.Flags().Lookup("credential-ref") != nil || providerAdd.Flags().Lookup("credential-clients") != nil {
		t.Fatalf("legacy credential flags remain in provider add")
	}
	credentialFlag := providerAdd.Flags().Lookup("credential")
	if credentialFlag == nil || !strings.Contains(credentialFlag.Usage, "shorthand, not a reference") {
		t.Fatalf("provider add --credential help does not identify the shorthand: %#v", credentialFlag)
	}
	if !strings.Contains(providerAdd.Long, "--credential is the short name") || !strings.Contains(providerAdd.Long, "<provider>-<credential>-ref") {
		t.Fatalf("provider add help does not explain shorthand/reference generation: %q", providerAdd.Long)
	}
}

func TestSessionCommandUsesOnlyTheSeparateSessionDatabase(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	if err := run([]string{"--state-dir", state, "session", "list"}, bytes.NewReader(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, "sessions.sqlite3")); err != nil {
		t.Fatalf("sessions database missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "agentdeck.sqlite3")); !os.IsNotExist(err) {
		t.Fatalf("session command created core database: %v", err)
	}
}

func TestSessionPurgeRespectsStateLock(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	lock, err := store.AcquireLock(t.Context(), state, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	err = run([]string{"--state-dir", state, "session", "purge-index"}, bytes.NewReader(nil), &bytes.Buffer{})
	if !errors.Is(err, store.ErrStateBusy) {
		t.Fatalf("purge while state is locked = %v, want state_busy", err)
	}
}

func TestSessionCommandsPreserveSourcesAndDoNotExposeProhibitedContent(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	home := filepath.Join(t.TempDir(), "home")
	source := filepath.Join(home, ".codex", "sessions", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
		t.Fatal(err)
	}
	contents := []byte("{\"type\":\"visible_user_prompt\",\"session_id\":\"s\",\"payload\":{\"text\":\"approved prompt\"}}\n" +
		"{\"type\":\"developer\",\"session_id\":\"s\",\"payload\":{\"text\":\"forbidden-secret\"}}\n" +
		"{\"type\":\"visible_assistant_final\",\"session_id\":\"s\",\"payload\":{\"text\":\"approved reply\"}}\n")
	if err := os.WriteFile(source, contents, 0600); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	runSession := func(args ...string) string {
		t.Helper()
		var output bytes.Buffer
		if err := run(append([]string{"--state-dir", state, "--format", "json", "session"}, args...), bytes.NewReader(nil), &output); err != nil {
			t.Fatalf("session %v: %v", args, err)
		}
		if bytes.Contains(output.Bytes(), []byte("forbidden-secret")) {
			t.Fatalf("session %v exposed prohibited content: %s", args, output.String())
		}
		return output.String()
	}
	runSession("scan")
	if output := runSession("list"); !bytes.Contains([]byte(output), []byte(`"session_id":"s"`)) {
		t.Fatalf("session list omitted metadata: %s", output)
	}
	for _, args := range [][]string{{"search", "approved"}, {"show", "s", "--client", "codex"}} {
		if output := runSession(args...); !bytes.Contains([]byte(output), []byte("approved")) {
			t.Fatalf("session %v omitted approved content: %s", args, output)
		}
	}
	runSession("search", `"forbidden-secret"`)
	runSession("exclude", "--kind", "session", "--value", "s")
	if output := runSession("search", "approved"); bytes.Contains([]byte(output), []byte("approved")) {
		t.Fatalf("excluded session remained visible: %s", output)
	}
	runSession("rebuild")
	if output := runSession("search", "approved"); bytes.Contains([]byte(output), []byte("approved")) {
		t.Fatalf("rebuild restored excluded session: %s", output)
	}
	if after, err := os.ReadFile(source); err != nil || !bytes.Equal(after, contents) {
		t.Fatalf("source changed: %q err=%v", after, err)
	}
	runSession("purge-index")
	if _, err := os.Stat(filepath.Join(state, "sessions.sqlite3")); !os.IsNotExist(err) {
		t.Fatalf("purge-index left database: %v", err)
	}
}

func TestSessionShowReportsCrossClientAmbiguityAndClientFlagsValidate(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	sessions, err := store.OpenSessions(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{"codex", "claude"} {
		doc, docErr := session.ApprovedDocument(client, "shared", "user_prompt", client+" visible")
		if docErr != nil {
			t.Fatal(docErr)
		}
		if docErr = session.ReplaceDocuments(ctx, sessions.DB, client, "shared", []session.Document{doc}); docErr != nil {
			t.Fatal(docErr)
		}
	}
	if err = sessions.Close(); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return t.TempDir(), nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"--state-dir", state, "--format", "json", "session", "show", "shared"}, bytes.NewReader(nil), &stdout, &stderr); exit != 2 {
		t.Fatalf("ambiguous show exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "ambiguous") || !strings.Contains(stderr.String(), `"code":"invalid_argument"`) {
		t.Fatalf("ambiguous error = %s", stderr.String())
	}
	for _, args := range [][]string{
		{"session", "list", "--client", "invalid"},
		{"session", "search", "visible", "--client", "invalid"},
		{"session", "show", "shared", "--client", "invalid"},
		{"credential", "list", "--client", "invalid"},
	} {
		stdout.Reset()
		stderr.Reset()
		full := append([]string{"--state-dir", state, "--format", "json"}, args...)
		if exit := execute(full, bytes.NewReader(nil), &stdout, &stderr); exit != 2 || !strings.Contains(stderr.String(), `"code":"invalid_argument"`) {
			t.Fatalf("invalid client %v exit=%d stderr=%s", args, exit, stderr.String())
		}
	}
}

func TestSessionPurgeClearsOnlySessionCheckpointAndWatchBootstraps(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	state, home := filepath.Join(root, "state"), filepath.Join(root, "home")
	core, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	for domain, value := range map[string]string{"usage": "usage-stable", "session": "session-stale", "extension": "extension-stable"} {
		if err = core.SetSetting(ctx, "watch.fingerprint."+domain, value); err != nil {
			core.Close()
			t.Fatal(err)
		}
	}
	if err = core.Close(); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(home, ".codex", "sessions", "bootstrap.jsonl")
	if err = os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(source, []byte("{\"type\":\"visible_user_prompt\",\"session_id\":\"bootstrap\",\"payload\":{\"text\":\"rebuilt\"}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	if err = run([]string{"--state-dir", state, "session", "purge-index"}, bytes.NewReader(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	core, err = store.OpenReadOnly(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, settingErr := core.Setting(ctx, "watch.fingerprint.session"); settingErr != nil || found {
		core.Close()
		t.Fatalf("session checkpoint found=%t err=%v", found, settingErr)
	}
	for _, domain := range []string{"usage", "extension"} {
		if _, found, settingErr := core.Setting(ctx, "watch.fingerprint."+domain); settingErr != nil || !found {
			core.Close()
			t.Fatalf("%s checkpoint found=%t err=%v", domain, found, settingErr)
		}
	}
	if err = core.Close(); err != nil {
		t.Fatal(err)
	}
	watchCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	writer := &cancelAfterLineWriter{cancel: cancel}
	command := newRootCommand(bytes.NewReader(nil), writer)
	command.SetContext(watchCtx)
	command.SetArgs([]string{"--state-dir", state, "--format", "ndjson", "watch", "--domains", "session", "--interval", "10ms"})
	if err = command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(writer.String(), `"domain":"session"`) {
		t.Fatalf("watch output = %s", writer.String())
	}
	var output bytes.Buffer
	if err = run([]string{"--state-dir", state, "--format", "json", "session", "show", "bootstrap"}, bytes.NewReader(nil), &output); err != nil || !strings.Contains(output.String(), "rebuilt") {
		t.Fatalf("bootstrapped session = %s, %v", output.String(), err)
	}
}

func TestSessionPurgeFailureOrdering(t *testing.T) {
	for _, stage := range []string{"core open", "checkpoint delete", "index delete"} {
		t.Run(stage, func(t *testing.T) {
			ctx := context.Background()
			state := filepath.Join(t.TempDir(), "state")
			core, err := store.Open(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			if err = core.SetSetting(ctx, "watch.fingerprint.session", "stale"); err != nil {
				core.Close()
				t.Fatal(err)
			}
			if err = core.SetSetting(ctx, "watch.fingerprint.usage", "stable"); err != nil {
				core.Close()
				t.Fatal(err)
			}
			if err = core.Close(); err != nil {
				t.Fatal(err)
			}
			index := filepath.Join(state, "sessions.sqlite3")
			if err = os.WriteFile(index, []byte("rebuildable"), 0o600); err != nil {
				t.Fatal(err)
			}

			openCore := store.Open
			remove := os.Remove
			switch stage {
			case "core open":
				openCore = func(context.Context, string) (*store.Store, error) {
					return nil, errors.New("injected core open failure")
				}
			case "checkpoint delete":
				openCore = func(ctx context.Context, state string) (*store.Store, error) {
					database, openErr := store.Open(ctx, state)
					if openErr != nil {
						return nil, openErr
					}
					if _, triggerErr := database.Exec(ctx, `CREATE TRIGGER fail_checkpoint_delete BEFORE DELETE ON settings WHEN OLD.key='watch.fingerprint.session' BEGIN SELECT RAISE(FAIL,'injected checkpoint delete failure'); END`); triggerErr != nil {
						database.Close()
						return nil, triggerErr
					}
					return database, nil
				}
			case "index delete":
				remove = func(path string) error {
					if path == index {
						return errors.New("injected index delete failure")
					}
					return os.Remove(path)
				}
			}
			if err = purgeSessionIndex(ctx, state, openCore, remove); err == nil {
				t.Fatal("purge unexpectedly succeeded")
			}
			if _, statErr := os.Stat(index); statErr != nil {
				t.Fatalf("index should remain after %s failure: %v", stage, statErr)
			}
			core, err = store.OpenReadOnly(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			_, sessionFound, sessionErr := core.Setting(ctx, "watch.fingerprint.session")
			_, usageFound, usageErr := core.Setting(ctx, "watch.fingerprint.usage")
			core.Close()
			if sessionErr != nil || usageErr != nil || !usageFound {
				t.Fatalf("checkpoint state session=%t usage=%t errors=%v/%v", sessionFound, usageFound, sessionErr, usageErr)
			}
			if stage == "index delete" && sessionFound {
				t.Fatal("session checkpoint survived index deletion failure")
			}
			if stage != "index delete" && !sessionFound {
				t.Fatal("session checkpoint changed before index deletion stage")
			}
		})
	}
}

func TestUsageOnlyWatchNeverCreatesSessionStore(t *testing.T) {
	root := t.TempDir()
	state, home := filepath.Join(root, "state"), filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	writer := &cancelAfterLineWriter{cancel: cancel}
	command := newRootCommand(bytes.NewReader(nil), writer)
	command.SetContext(ctx)
	command.SetArgs([]string{"--state-dir", state, "--format", "ndjson", "watch", "--domains", "usage", "--interval", "10ms"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, "sessions.sqlite3")); !os.IsNotExist(err) {
		t.Fatalf("usage-only watch created session store: %v", err)
	}
}

func TestDeleteOnlyWatchTextAndNDJSONUseLogicalUnitsFromRealScans(t *testing.T) {
	at := time.Date(2026, 7, 21, 1, 2, 3, 0, time.UTC)
	assertOutput := func(t *testing.T, event watch.Event, label string, wantChanges int) {
		t.Helper()
		if event.Type != "scan_completed" || event.SchemaVersion != watch.EventSchemaVersion || event.Changes != wantChanges {
			t.Fatalf("watch event = %#v, want %d changes", event, wantChanges)
		}
		var textOutput bytes.Buffer
		if err := renderWatchText(&textOutput, event); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(textOutput.String(), fmt.Sprintf("%d %s changed", wantChanges, label)) {
			t.Fatalf("%s watch text = %s", event.Domain, textOutput.String())
		}
		var ndjson bytes.Buffer
		if err := json.NewEncoder(&ndjson).Encode(event); err != nil {
			t.Fatal(err)
		}
		var decoded watch.Event
		if err := json.Unmarshal(ndjson.Bytes(), &decoded); err != nil || decoded.Domain != event.Domain || decoded.Changes != event.Changes || decoded.Type != event.Type || decoded.SchemaVersion != event.SchemaVersion {
			t.Fatalf("%s watch NDJSON = %s, %#v, %v", event.Domain, ndjson.String(), decoded, err)
		}
	}

	t.Run("session middle deletion", func(t *testing.T) {
		ctx := context.Background()
		root := t.TempDir()
		home := filepath.Join(root, "home")
		path := filepath.Join(home, ".codex", "sessions", "delete.jsonl")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		writeDocuments := func(values ...string) {
			t.Helper()
			var contents strings.Builder
			for _, value := range values {
				fmt.Fprintf(&contents, "{\"type\":\"visible_user_prompt\",\"session_id\":\"delete\",\"payload\":{\"text\":%q}}\n", value)
			}
			if err := os.WriteFile(path, []byte(contents.String()), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		writeDocuments("one", "two", "three")
		database, err := store.OpenSessions(ctx, filepath.Join(root, "state"))
		if err != nil {
			t.Fatal(err)
		}
		defer database.Close()
		if result, scanErr := session.Scan(ctx, database.DB, home); scanErr != nil || result.Documents != 3 {
			t.Fatalf("initial session scan = %#v, %v", result, scanErr)
		}
		initialFingerprint, err := watch.FingerprintRoots(sessionWatchRoots(home)...)
		if err != nil {
			t.Fatal(err)
		}
		writeDocuments("one", "three")
		var scanResult session.ScanResult
		service := watch.Service{
			InitialFingerprints: map[string]string{"session": initialFingerprint},
			Sources: watch.SourceSet{{
				Domain:   "session",
				Snapshot: func(context.Context) (string, error) { return watch.FingerprintRoots(sessionWatchRoots(home)...) },
				Scan: func(ctx context.Context) (int, error) {
					var scanErr error
					scanResult, scanErr = session.Scan(ctx, database.DB, home)
					return scanResult.Documents + scanResult.Removed, scanErr
				},
			}},
			Lock: func(context.Context) (func() error, error) { return func() error { return nil }, nil },
			Now:  func() time.Time { return at },
		}
		events, err := service.Poll(ctx)
		if err != nil || len(events) != 1 {
			t.Fatalf("session watch Poll = %#v, %v", events, err)
		}
		if scanResult.Documents != 0 || scanResult.Removed != 1 {
			t.Fatalf("middle deletion scan result = %#v", scanResult)
		}
		assertOutput(t, events[0], "session documents", 1)
	})

	t.Run("usage source deletion", func(t *testing.T) {
		ctx := context.Background()
		root := t.TempDir()
		home := filepath.Join(root, "home")
		path := filepath.Join(home, ".codex", "sessions", "usage.jsonl")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		contents := `{"type":"session_meta","payload":{"session_id":"usage-delete"}}` + "\n" +
			`{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" +
			`{"type":"event_msg","timestamp":"2026-07-20T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10}}}}` + "\n"
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		database, err := store.Open(ctx, filepath.Join(root, "state"))
		if err != nil {
			t.Fatal(err)
		}
		defer database.Close()
		usageService := usage.New(database, home)
		if result, scanErr := usageService.Scan(ctx); scanErr != nil || result["imported"] != 1 {
			t.Fatalf("initial usage scan = %#v, %v", result, scanErr)
		}
		initialInventory, err := usageService.Inventory(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.Remove(path); err != nil {
			t.Fatal(err)
		}
		var inventory usage.Inventory
		var scanResult map[string]int
		service := watch.Service{
			InitialFingerprints: map[string]string{"usage": initialInventory.Fingerprint},
			Sources: watch.SourceSet{{
				Domain: "usage",
				Snapshot: func(ctx context.Context) (string, error) {
					var inventoryErr error
					inventory, inventoryErr = usageService.Inventory(ctx)
					return inventory.Fingerprint, inventoryErr
				},
				Scan: func(ctx context.Context) (int, error) {
					var scanErr error
					scanResult, scanErr = usageService.ScanInventory(ctx, inventory)
					return scanResult["imported"] + scanResult["updated"] + scanResult["removed"], scanErr
				},
			}},
			Lock: func(context.Context) (func() error, error) { return func() error { return nil }, nil },
			Now:  func() time.Time { return at },
		}
		events, err := service.Poll(ctx)
		if err != nil || len(events) != 1 {
			t.Fatalf("usage watch Poll = %#v, %v", events, err)
		}
		if scanResult["imported"] != 0 || scanResult["updated"] != 0 || scanResult["removed"] != 1 {
			t.Fatalf("usage deletion scan result = %#v", scanResult)
		}
		assertOutput(t, events[0], "usage records", 1)
	})
}

func TestProviderCurrentAndStatusRenderCredentialShorthand(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	vault := credentialvault.New(state, func(context.Context) (string, error) { return "synthetic-machine", nil })
	service := provider.Service{Store: database, Vault: vault}
	created, err := service.AddProvider(ctx, provider.Definition{Name: "example", Endpoint: "https://provider.example", Clients: []provider.Client{provider.ClientCodex}, Multiplier: "1"}, "work", "synthetic-secret")
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	selectedAt := time.Date(2026, 7, 20, 1, 2, 3, 0, time.UTC)
	if err = database.RecordSelection(ctx, store.Selection{ProviderID: created.ID, Client: "codex", ProviderName: "example", EndpointSnapshot: "https://provider.example/v1", MultiplierSnapshot: "1", CredentialName: "work", SelectedAt: selectedAt}); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		args []string
		want []string
	}{
		{args: []string{"provider", "current"}, want: []string{"| CLIENT | PROVIDER | CREDENTIAL | SELECTED AT", "| codex", "| example", "| work"}},
		{args: []string{"provider", "status"}, want: []string{"CODEX ACTIVE", "| example", "| work"}},
		{args: []string{"provider", "status", "example"}, want: []string{"| CLIENT | ACTIVE | CREDENTIAL | SELECTED AT", "| codex", "| true", "| work"}},
	} {
		var output bytes.Buffer
		args := append([]string{"--state-dir", state}, test.args...)
		if err = run(args, bytes.NewReader(nil), &output); err != nil {
			t.Fatalf("%v: %v", test.args, err)
		}
		for _, want := range test.want {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("%v missing %q:\n%s", test.args, want, output.String())
			}
		}
		if strings.Contains(output.String(), "synthetic-secret") {
			t.Fatalf("%v exposed credential value", test.args)
		}
	}
	var encoded bytes.Buffer
	if err = run([]string{"--state-dir", state, "--format", "json", "provider", "current"}, bytes.NewReader(nil), &encoded); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data []provider.CurrentSelection `json:"data"`
	}
	if err = json.Unmarshal(encoded.Bytes(), &envelope); err != nil || len(envelope.Data) != 1 || envelope.Data[0].Credential != "work" {
		t.Fatalf("provider current JSON = %#v, %v", envelope, err)
	}
}

func TestPriceCommandsUseReadableDedicatedRenderers(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, err = database.Exec(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES
('fixture','official','https://example.invalid/pricing','`+digest+`','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1),
('future','official','https://example.invalid/future','`+digest+`','2099-01-01T00:00:00Z','2099-01-01T00:00:00Z','USD',1);
INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES
('fixture','gpt-fixture','openai','2026-01-01T00:00:00Z','{"input":"1","cached_input":"0.5","output":"2"}','[]'),
('future','gpt-future','openai','2099-01-01T00:00:00Z','{"input":"9","cached_input":"9","output":"9"}','[]')`)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	for _, command := range [][]string{{"price", "status"}, {"price", "history"}, {"price", "list"}, {"price", "list", "gpt-fixture"}, {"price", "list", "--provider", "openai"}} {
		var output bytes.Buffer
		args := append([]string{"--state-dir", state}, command...)
		if err = run(args, bytes.NewReader(nil), &output); err != nil {
			t.Fatalf("%v: %v", command, err)
		}
		if strings.Contains(output.String(), "no usage text renderer") || strings.Contains(output.String(), digest) || strings.Contains(output.String(), "https://example.invalid/pricing") {
			t.Fatalf("default price output for %v leaked technical provenance or used usage renderer:\n%s", command, output.String())
		}
	}
	var verbose bytes.Buffer
	if err = run([]string{"--state-dir", state, "--verbose", "price", "list", "gpt-fixture"}, bytes.NewReader(nil), &verbose); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(verbose.String(), digest) || !strings.Contains(verbose.String(), "https://example.invalid/pricing") || !strings.Contains(verbose.String(), "USD / 1M tokens") {
		t.Fatalf("verbose price provenance = %s", verbose.String())
	}
	var mutation bytes.Buffer
	if err = writePriceEnvelope(&mutation, "text", "price.update", map[string]any{"version": "fixture", "models": 1, "commit_sha": digest[:40], "content_sha256": digest}, false, nil, false, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mutation.String(), "| RESULT") || strings.Contains(mutation.String(), digest) {
		t.Fatalf("price update result = %s", mutation.String())
	}
	var currentJSON bytes.Buffer
	if err = run([]string{"--state-dir", state, "--format", "json", "price", "status"}, bytes.NewReader(nil), &currentJSON); err != nil {
		t.Fatal(err)
	}
	var currentEnvelope struct {
		Data struct {
			Available bool                 `json:"available"`
			Version   string               `json:"version"`
			Catalogs  []usage.PriceCatalog `json:"catalogs"`
			Models    int                  `json:"models"`
		} `json:"data"`
	}
	if err = json.Unmarshal(currentJSON.Bytes(), &currentEnvelope); err != nil || !currentEnvelope.Data.Available || currentEnvelope.Data.Version != "fixture" || currentEnvelope.Data.Models != 1 || len(currentEnvelope.Data.Catalogs) != 1 {
		t.Fatalf("current plus future price status = %#v, %v", currentEnvelope, err)
	}

	futureOnlyState := filepath.Join(t.TempDir(), "future-only")
	futureOnly, err := store.Open(ctx, futureOnlyState)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = futureOnly.Exec(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES('future','official','https://example.invalid/future','`+digest+`','2099-01-01T00:00:00Z','2099-01-01T00:00:00Z','USD',1); INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES('future','gpt-future','openai','2099-01-01T00:00:00Z','{"input":"9"}','[]')`); err != nil {
		futureOnly.Close()
		t.Fatal(err)
	}
	if err = futureOnly.Close(); err != nil {
		t.Fatal(err)
	}
	var futureText, futureJSON bytes.Buffer
	if err = run([]string{"--state-dir", futureOnlyState, "price", "status"}, bytes.NewReader(nil), &futureText); err != nil {
		t.Fatal(err)
	}
	if err = run([]string{"--state-dir", futureOnlyState, "--format", "json", "price", "status"}, bytes.NewReader(nil), &futureJSON); err != nil {
		t.Fatal(err)
	}
	var futureEnvelope struct {
		Data struct {
			Available bool `json:"available"`
		} `json:"data"`
	}
	if err = json.Unmarshal(futureJSON.Bytes(), &futureEnvelope); err != nil || futureEnvelope.Data.Available || !strings.Contains(futureText.String(), "No price catalog is available.") {
		t.Fatalf("future-only price status text=%q json=%#v err=%v", futureText.String(), futureEnvelope, err)
	}
}

func TestUsageSummaryShortcutsAndStatsJSONContract(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	for _, period := range []string{"daily", "weekly", "monthly"} {
		var output bytes.Buffer
		if err := run([]string{"--state-dir", state, "usage", "summary", period}, bytes.NewReader(nil), &output); err != nil {
			t.Fatalf("summary %s: %v", period, err)
		}
		if !strings.Contains(output.String(), "USAGE SUMMARY") {
			t.Fatalf("summary %s = %s", period, output.String())
		}
	}
	var encoded bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "usage", "stats", "--from", "2026-07-01", "--to", "2026-07-07"}, bytes.NewReader(nil), &encoded); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(encoded.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok || envelope["command"] != "usage.stats" {
		t.Fatalf("stats envelope = %#v", envelope)
	}
	if timezone, _ := data["timezone"].(string); timezone == "" || timezone == "Local" {
		t.Fatalf("stats timezone = %q", timezone)
	}
	for _, key := range []string{"range", "timezone", "totals", "buckets", "models", "clients", "providers", "cache_sessions", "activity", "peak", "coverage", "unpriced_models"} {
		if _, exists := data[key]; !exists {
			t.Fatalf("stats JSON missing %s: %#v", key, data)
		}
	}
	var providerFiltered bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "usage", "stats", "--from", "2026-07-01", "--to", "2026-07-07", "--provider", "not-enumerated"}, bytes.NewReader(nil), &providerFiltered); err != nil {
		t.Fatalf("open-set provider filter: %v", err)
	}
	totals, ok := data["totals"].(map[string]any)
	if !ok {
		t.Fatalf("stats totals = %#v", data["totals"])
	}
	for _, key := range []string{"input_tokens", "output_tokens", "cached_read_tokens", "cache_write_tokens", "catalog_base_cost", "provider_cost", "known_catalog_base_cost", "known_provider_cost"} {
		if _, exists := totals[key]; !exists {
			t.Fatalf("stats totals JSON missing %s: %#v", key, totals)
		}
	}
	if activity, ok := data["activity"].([]any); !ok || len(activity) != 168 {
		t.Fatalf("stats activity = %#v", data["activity"])
	}
	models, ok := data["models"].([]any)
	if !ok {
		t.Fatalf("stats models = %#v", data["models"])
	}
	if len(models) > 0 {
		firstModel, _ := models[0].(map[string]any)
		if _, exists := firstModel["cache_hit_rate"]; !exists {
			t.Fatalf("model cache hit rate missing: %#v", firstModel)
		}
		if _, exists := firstModel["cache_read_rate"]; exists {
			t.Fatalf("legacy cache read rate remains: %#v", firstModel)
		}
	}
	var textOutput bytes.Buffer
	if err := run([]string{"--state-dir", state, "usage", "stats", "--from", "2026-07-01", "--to", "2026-07-07"}, bytes.NewReader(nil), &textOutput); err != nil {
		t.Fatal(err)
	}
	for _, section := range []string{"USAGE STATS", "TREND", "MODELS", "CLIENTS", "PROVIDERS", "No providers in this range.", "ACTIVITY BY WEEKDAY / HOUR"} {
		if !strings.Contains(textOutput.String(), section) {
			t.Fatalf("stats text missing %q:\n%s", section, textOutput.String())
		}
	}
	if !strings.Contains(textOutput.String(), "Jul 01, 2026 - Jul 07, 2026") || strings.Contains(textOutput.String(), "Jul 08, 2026") {
		t.Fatalf("stats text range is not inclusive:\n%s", textOutput.String())
	}
}

// TestUsageStatsTopFlagOverridesTextCapsButNotJSON exercises `--top` through
// the real CLI flag-parsing path (cobra Changed/GetInt in the `usage stats`
// withUsage wrapper), not just the renderer's capFor logic directly, so it
// proves the flag is actually wired end to end.
func TestUsageStatsTopFlagOverridesTextCapsButNotJSON(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	const modelCount = 9
	var sessionRows, eventRows []string
	for index := 0; index < modelCount; index++ {
		model := fmt.Sprintf("model-%02d", index)
		session := fmt.Sprintf("s%02d", index)
		at := fmt.Sprintf("2026-07-%02dT00:00:00Z", index+1)
		sessionRows = append(sessionRows, fmt.Sprintf("('codex','%s','%s','%s')", session, at, at))
		eventRows = append(eventRows, fmt.Sprintf("('e%d','codex','%s','e%d','%s','%s',%d,0,0,'fixture',%d)", index, session, index, at, model, 1000-index, index))
	}
	insert := "INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES " + strings.Join(sessionRows, ",") + ";" +
		"INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,source_path,source_offset) VALUES " + strings.Join(eventRows, ",")
	if _, err := database.Exec(ctx, insert); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	runStats := func(extra ...string) string {
		t.Helper()
		args := append([]string{"--state-dir", state, "usage", "stats", "--no-scan", "--period", "all"}, extra...)
		var out bytes.Buffer
		if err := run(args, bytes.NewReader(nil), &out); err != nil {
			t.Fatalf("run(%v) = %v", args, err)
		}
		return out.String()
	}
	// modelsSection isolates the MODELS block: the seeded models are also all
	// unpriced, so they additionally appear (uncapped, unaffected by --top)
	// in the UNPRICED MODELS section, which would otherwise double-count.
	modelsSection := func(t *testing.T, text string) string {
		t.Helper()
		start := strings.Index(text, "🤖 MODELS")
		if start < 0 {
			t.Fatalf("stats text missing MODELS section:\n%s", text)
		}
		end := strings.Index(text[start:], "\nCLIENTS")
		if end < 0 {
			t.Fatalf("stats text missing CLIENTS section after MODELS:\n%s", text)
		}
		return text[start : start+end]
	}
	// omitted/default: no --top given, shared-topn's default cap (8) applies.
	if text := runStats(); strings.Count(modelsSection(t, text), "model-") != 8 || !strings.Contains(modelsSection(t, text), "+1 more models in JSON") {
		t.Fatalf("default --top text cap =\n%s", text)
	}
	// positive N: overrides the default cap to exactly N.
	if text := runStats("--top", "3"); strings.Count(modelsSection(t, text), "model-") != 3 || !strings.Contains(modelsSection(t, text), "+6 more models in JSON") {
		t.Fatalf("--top 3 text cap =\n%s", text)
	}
	// explicit 0: restores the full, uncapped list.
	if text := runStats("--top", "0"); strings.Count(modelsSection(t, text), "model-") != modelCount || strings.Contains(modelsSection(t, text), "more models in JSON") {
		t.Fatalf("--top 0 text cap =\n%s", text)
	}
	// negative: rejected as invalid input.
	if err := run([]string{"--state-dir", state, "usage", "stats", "--no-scan", "--top", "-1"}, bytes.NewReader(nil), io.Discard); err == nil {
		t.Fatal("--top -1 did not error")
	}
	// JSON always carries every row, regardless of --top.
	for _, top := range []string{"", "3", "0"} {
		args := []string{"--state-dir", state, "--format", "json", "usage", "stats", "--no-scan", "--period", "all"}
		if top != "" {
			args = append(args, "--top", top)
		}
		var encoded bytes.Buffer
		if err := run(args, bytes.NewReader(nil), &encoded); err != nil {
			t.Fatalf("run(%v) = %v", args, err)
		}
		var envelope struct {
			Data struct {
				Models []any `json:"models"`
			} `json:"data"`
		}
		if err := json.Unmarshal(encoded.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if len(envelope.Data.Models) != modelCount {
			t.Fatalf("--top %q JSON models = %d, want %d\n%s", top, len(envelope.Data.Models), modelCount, encoded.String())
		}
	}
}

func TestResolveUsageRangeWeekIsCurrentLocalWeekAcrossBoundaries(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name             string
		now              time.Time
		from             time.Time
		crossesDSTOffset bool
	}{
		{name: "monday", now: time.Date(2026, 3, 2, 0, 15, 0, 0, location), from: time.Date(2026, 3, 2, 0, 0, 0, 0, location)},
		{name: "midweek", now: time.Date(2026, 3, 4, 12, 0, 0, 0, location), from: time.Date(2026, 3, 2, 0, 0, 0, 0, location)},
		{name: "sunday across DST transition", now: time.Date(2026, 3, 8, 23, 59, 0, 0, location), from: time.Date(2026, 3, 2, 0, 0, 0, 0, location), crossesDSTOffset: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			from, to, rangeErr := resolveUsageRange(context.Background(), nil, "week", "", "", test.now, location)
			if rangeErr != nil {
				t.Fatal(rangeErr)
			}
			if !from.Equal(test.from) || !to.Equal(test.now) {
				t.Fatalf("range = [%s,%s), want current week [%s,%s)", from, to, test.from, test.now)
			}
			if from.Weekday() != time.Monday || from.Hour() != 0 || from.Location() != location {
				t.Fatalf("week start = %s", from)
			}
			if test.crossesDSTOffset {
				_, fromOffset := from.Zone()
				_, toOffset := to.Zone()
				if fromOffset == toOffset {
					t.Fatalf("range offsets = %d and %d, want DST transition", fromOffset, toOffset)
				}
			}
		})
	}
}

func TestUsageTimezoneNameHonorsExplicitTZ(t *testing.T) {
	t.Setenv("TZ", "America/New_York")
	local := time.FixedZone("Local", -5*60*60)
	if got := usageTimezoneName(local, time.Date(2026, 1, 1, 0, 0, 0, 0, local)); got != "America/New_York" {
		t.Fatalf("explicit timezone = %q", got)
	}
	if got := timezoneNameFromPath("/var/db/timezone/zoneinfo/Asia/Shanghai"); got != "Asia/Shanghai" {
		t.Fatalf("macOS localtime path = %q", got)
	}
}

func TestUsageStatsTextMarksPartialAverageAndPeakCost(t *testing.T) {
	var output bytes.Buffer
	known := "1.250000000"
	report := usage.StatsReport{
		Range: usage.StatsRange{From: "2026-07-01T00:00:00Z", To: "2026-07-02T00:00:00Z"}, Timezone: "UTC", GroupBy: "day", Metric: "cost",
		Totals:  usage.StatsTotals{KnownProviderCost: known, KnownAverageCost: known},
		Buckets: []usage.StatsBucket{}, Models: []usage.StatsDimension{}, Clients: []usage.StatsDimension{}, Activity: []usage.StatsActivity{},
		Peak: usage.StatsPeak{KnownValue: known}, Coverage: usage.StatsCoverage{Percent: "50.00"},
	}
	if err := renderUsageStats(&output, report); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), "$1.25 KNOWN") < 2 || strings.Contains(output.String(), "Known priced subtotal") || strings.Contains(output.String(), "(partial)") {
		t.Fatalf("partial stats text = %s", output.String())
	}
}
