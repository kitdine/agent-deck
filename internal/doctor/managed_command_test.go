package doctor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/usagehook"
)

type notInstalledDoctorFixture struct{ account string }

func (r notInstalledDoctorFixture) AccountHome() (string, error) { return r.account, nil }
func (r notInstalledDoctorFixture) Resolve() platform.InstallationIdentity {
	return platform.InstallationIdentity{Reason: "receipt_other_executable"}
}

func writeClientFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func ownerFixture(t *testing.T) (home, self, other string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	self = filepath.Join(base, "self", "agentdeck")
	other = filepath.Join(base, "other", "agentdeck")
	writeClientFile(t, self, "")
	writeClientFile(t, other, "")
	old := currentExecutable
	currentExecutable = func() (string, error) { return self, nil }
	t.Cleanup(func() { currentExecutable = old })
	return filepath.Join(base, "home"), self, other
}

func claudeHook(command string) string {
	return `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":` + quoteJSON(command) + `}]}]}}`
}

func quoteJSON(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func TestManagedCommandOwnerCases(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		claude, codex            func(self, other string) string
		status, code, reason, rc string
	}{
		{name: "none", status: "ok"},
		{name: "owner", claude: func(self, _ string) string { return claudeHook("'" + self + "' usage hook event claude") }, status: "ok"},
		{name: "path-only", claude: func(_, _ string) string { return claudeHook("agentdeck usage hook event claude") }, status: "ok", reason: "path_resolved"},
		{name: "claude-mismatch", claude: func(_, other string) string { return claudeHook(other + " usage hook event claude") }, status: "warning", code: "managed_command_owner_mismatch", reason: "claude", rc: "agentdeck usage hook setup --client claude"},
		{name: "both-mismatch", claude: func(_, other string) string { return claudeHook(other + " usage hook event claude") }, codex: func(_, other string) string {
			return `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"` + other + ` usage hook event codex"}]}]}}`
		}, status: "warning", code: "managed_command_owner_mismatch", reason: "claude,codex", rc: "agentdeck usage hook setup --client all"},
		{name: "statusline-mismatch", claude: func(_, other string) string {
			return `{"statusLine":{"type":"command","command":"'` + other + `' quota capture"}}`
		}, status: "warning", code: "managed_command_owner_mismatch", reason: "claude", rc: "agentdeck desktop quota-statusline enable"},
		{name: "missing-program", claude: func(self, _ string) string { return claudeHook(self + "-gone/agentdeck usage hook event claude") }, status: "warning", code: "managed_command_owner_mismatch", reason: "claude", rc: "agentdeck usage hook setup --client claude"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, self, other := ownerFixture(t)
			if tc.claude != nil {
				writeClientFile(t, filepath.Join(home, ".claude", "settings.json"), tc.claude(self, other))
			}
			if tc.codex != nil {
				writeClientFile(t, filepath.Join(home, ".codex", "hooks.json"), tc.codex(self, other))
			}
			check, ok := managedCommandOwnerCheck(home)
			if !ok || check.Name != "managed_command_owner" || check.Status != tc.status || check.Code != tc.code || check.Reason != tc.reason || check.Recovery != tc.rc {
				t.Fatalf("check = %+v (ran=%v)", check, ok)
			}
			if strings.Contains(check.Reason+check.Recovery, "/") {
				t.Fatalf("check discloses a path: %+v", check)
			}
		})
	}
}

func TestManagedCommandOwnerOnlyForInstalledUnshortCircuitedReports(t *testing.T) {
	home, _, other := ownerFixture(t)
	writeClientFile(t, filepath.Join(home, ".claude", "settings.json"), claudeHook(other+" usage hook event claude"))
	ctx := context.Background()
	// Installed, but the state directory is missing: the short circuit wins.
	report, err := Service{StateRoot: filepath.Join(t.TempDir(), "missing"), Home: home}.Check(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range report.Checks {
		if check.Name == "managed_command_owner" {
			t.Fatalf("missing-state partial report ran managed_command_owner: %+v", report.Checks)
		}
	}
	// Installed with readable state: the check runs.
	state := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	report, err = Service{StateRoot: state, Home: home}.Check(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, check := range report.Checks {
		found = found || check.Name == "managed_command_owner" && check.Code == "managed_command_owner_mismatch"
	}
	if !found {
		t.Fatalf("installed report lacks managed_command_owner: %+v", report.Checks)
	}
	// Not installed: never runs.
	account := os.Getenv("AGENTDECK_ACCEPTANCE_ACCOUNT_HOME")
	platform.SetInstallationResolver(notInstalledDoctorFixture{account})
	t.Cleanup(func() { platform.SetInstallationResolver(installedDoctorFixture{account}) })
	if _, ran := managedCommandOwnerCheck(home); ran {
		t.Fatal("not-installed doctor ran managed_command_owner")
	}
}

// ISO-DIAG-R1-F1: the Hook recovery command doctor prints must actually clear
// the old absolute-path route it detected, keep unrelated hooks, and stay
// stable when repeated.
func TestManagedCommandOwnerHookRecoveryClearsMismatch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		quoted bool
		claude bool
		codex  bool
	}{
		{name: "claude", claude: true},
		{name: "codex", codex: true},
		{name: "both-quoted", quoted: true, claude: true, codex: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _, other := ownerFixture(t)
			program := other
			if tc.quoted {
				program = "'" + other + "'"
			}
			unrelated := `{"type":"command","command":"printf keep"}`
			if tc.claude {
				writeClientFile(t, filepath.Join(home, ".claude", "settings.json"), `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":`+quoteJSON(program+" usage hook event claude")+`},`+unrelated+`]}]}}`)
			}
			if tc.codex {
				writeClientFile(t, filepath.Join(home, ".codex", "hooks.json"), `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":`+quoteJSON(program+" usage hook event codex")+`},`+unrelated+`]}]}}`)
			}
			before, _ := managedCommandOwnerCheck(home)
			const prefix = "agentdeck usage hook setup --client "
			if before.Code != "managed_command_owner_mismatch" || !strings.HasPrefix(before.Recovery, prefix) {
				t.Fatalf("before = %+v", before)
			}
			client := usagehook.Client(strings.TrimPrefix(before.Recovery, prefix))
			for run := 0; run < 2; run++ {
				if _, err := usagehook.New(usagehook.Environment{Home: home}).Setup(usagehook.Request{Client: client}); err != nil {
					t.Fatalf("setup run %d: %v", run, err)
				}
				after, _ := managedCommandOwnerCheck(home)
				if after.Status != "ok" || after.Code != "" {
					t.Fatalf("after run %d = %+v", run, after)
				}
			}
			for _, path := range []string{filepath.Join(home, ".claude", "settings.json"), filepath.Join(home, ".codex", "hooks.json")} {
				data, err := os.ReadFile(path)
				if os.IsNotExist(err) {
					continue
				}
				if strings.Contains(string(data), other) {
					t.Fatalf("%s retained the old route: %s", path, data)
				}
				if (strings.Contains(path, ".claude") && tc.claude || strings.Contains(path, ".codex") && tc.codex) && strings.Count(string(data), "printf keep") != 1 {
					t.Fatalf("%s lost or duplicated the unrelated hook: %s", path, data)
				}
			}
		})
	}
}

// appendLegacyRoute adds an earlier owner's absolute-path hook and an unrelated
// hook to SessionStart, keeping every hook already present in the file.
func appendLegacyRoute(t *testing.T, path, program string, client usagehook.Client) {
	t.Helper()
	var document map[string]any
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if document == nil {
		document = map[string]any{}
	}
	hooks, _ := document["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		document["hooks"] = hooks
	}
	groups, _ := hooks["SessionStart"].([]any)
	hooks["SessionStart"] = append(groups, map[string]any{"hooks": []any{
		map[string]any{"type": "command", "command": program + " usage hook event " + string(client)},
		map[string]any{"type": "command", "command": "printf keep"},
	}})
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	writeClientFile(t, path, string(data))
}

func assertLegacyRouteCleared(t *testing.T, path, other string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), other) {
		t.Fatalf("%s retained the old route: %s", path, data)
	}
	if strings.Count(string(data), "printf keep") != 1 {
		t.Fatalf("%s lost or duplicated the unrelated hook: %s", path, data)
	}
}

func TestManagedCommandOwnerHookRecoveryClearsCoexistingAndRemovedRoutes(t *testing.T) {
	paths := map[usagehook.Client]func(home string) string{
		usagehook.ClientClaude: func(home string) string { return filepath.Join(home, ".claude", "settings.json") },
		usagehook.ClientCodex:  func(home string) string { return filepath.Join(home, ".codex", "hooks.json") },
	}
	for _, client := range []usagehook.Client{usagehook.ClientClaude, usagehook.ClientCodex} {
		for _, quoted := range []bool{false, true} {
			suffix := ""
			if quoted {
				suffix = "-quoted"
			}
			t.Run("setup-coexisting-"+string(client)+suffix, func(t *testing.T) {
				home, _, other := ownerFixture(t)
				program := other
				if quoted {
					program = "'" + other + "'"
				}
				manager := usagehook.New(usagehook.Environment{Home: home})
				if _, err := manager.Setup(usagehook.Request{Client: client}); err != nil {
					t.Fatalf("initial setup: %v", err)
				}
				path := paths[client](home)
				appendLegacyRoute(t, path, program, client)
				before, _ := managedCommandOwnerCheck(home)
				if before.Code != "managed_command_owner_mismatch" || before.Recovery != "agentdeck usage hook setup --client "+string(client) {
					t.Fatalf("before = %+v", before)
				}
				for run, want := range []usagehook.Outcome{usagehook.OutcomeConfigured, usagehook.OutcomeUnchanged} {
					summary, err := usagehook.New(usagehook.Environment{Home: home}).Setup(usagehook.Request{Client: client})
					if err != nil {
						t.Fatalf("setup run %d: %v", run, err)
					}
					if len(summary.Results) != 1 || summary.Results[0].Outcome != want {
						t.Fatalf("setup run %d = %+v, want %s", run, summary.Results, want)
					}
					after, _ := managedCommandOwnerCheck(home)
					if after.Status != "ok" || after.Code != "" {
						t.Fatalf("after run %d = %+v", run, after)
					}
					assertLegacyRouteCleared(t, path, other)
				}
			})
			t.Run("remove-legacy-only-"+string(client)+suffix, func(t *testing.T) {
				home, _, other := ownerFixture(t)
				program := other
				if quoted {
					program = "'" + other + "'"
				}
				path := paths[client](home)
				appendLegacyRoute(t, path, program, client)
				before, _ := managedCommandOwnerCheck(home)
				if before.Code != "managed_command_owner_mismatch" {
					t.Fatalf("before = %+v", before)
				}
				for run, want := range []usagehook.Outcome{usagehook.OutcomeRemoved, usagehook.OutcomeAbsent} {
					summary, err := usagehook.New(usagehook.Environment{Home: home}).Remove(usagehook.Request{Client: client})
					if err != nil {
						t.Fatalf("remove run %d: %v", run, err)
					}
					if len(summary.Results) != 1 || summary.Results[0].Outcome != want {
						t.Fatalf("remove run %d = %+v, want %s", run, summary.Results, want)
					}
					after, _ := managedCommandOwnerCheck(home)
					if after.Status != "ok" || after.Code != "" {
						t.Fatalf("after run %d = %+v", run, after)
					}
					assertLegacyRouteCleared(t, path, other)
				}
			})
		}
	}
}
