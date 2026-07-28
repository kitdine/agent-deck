package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeClaudeSettingsForTest(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestClaudeCredentialConflictsNamesSourcesWithoutRevealingValues covers the
// detector AgentDeck uses instead of deleting a field it does not own. One of
// the two keys holds a credential, so the result must name keys only.
func TestClaudeCredentialConflictsNamesSourcesWithoutRevealingValues(t *testing.T) {
	path := writeClaudeSettingsForTest(t, `{"env":{"ANTHROPIC_API_KEY":"synthetic-secret","UNRELATED":true},"apiKeyHelper":"/bin/echo synthetic-helper-secret"}`)
	conflicts, err := ClaudeCredentialConflicts(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 2 || conflicts[0] != ClaudeConflictAPIKey || conflicts[1] != ClaudeConflictAPIKeyHelper {
		t.Fatalf("conflicts = %#v", conflicts)
	}
	for _, conflict := range conflicts {
		if strings.Contains(conflict, "synthetic-secret") || strings.Contains(conflict, "synthetic-helper-secret") {
			t.Fatalf("conflict leaked a credential value: %q", conflict)
		}
	}
}

// TestClaudeCredentialConflictsIgnoresSourcesThatConfigureNothing keeps the
// advisory from firing on a key that overrides nothing, which would train
// users to ignore it. Both keys are string-valued to Claude, so every
// non-string shape is either empty or malformed for the key it sits on and
// cannot supply a credential.
func TestClaudeCredentialConflictsIgnoresSourcesThatConfigureNothing(t *testing.T) {
	for _, test := range []struct {
		name     string
		contents string
	}{
		{name: "absent", contents: `{"env":{"ANTHROPIC_BASE_URL":"https://example.invalid"}}`},
		{name: "empty string", contents: `{"env":{"ANTHROPIC_API_KEY":""},"apiKeyHelper":""}`},
		{name: "null", contents: `{"env":{"ANTHROPIC_API_KEY":null},"apiKeyHelper":null}`},
		{name: "no env object", contents: `{"apiKeyHelper":""}`},
		{name: "env is not an object", contents: `{"env":"unexpected"}`},
		{name: "false", contents: `{"env":{"ANTHROPIC_API_KEY":false},"apiKeyHelper":false}`},
		{name: "true", contents: `{"env":{"ANTHROPIC_API_KEY":true},"apiKeyHelper":true}`},
		{name: "number", contents: `{"env":{"ANTHROPIC_API_KEY":0},"apiKeyHelper":12345}`},
		{name: "empty object", contents: `{"env":{"ANTHROPIC_API_KEY":{}},"apiKeyHelper":{}}`},
		{name: "empty array", contents: `{"env":{"ANTHROPIC_API_KEY":[]},"apiKeyHelper":[]}`},
		{name: "non-empty object", contents: `{"env":{"ANTHROPIC_API_KEY":{"value":"synthetic-secret"}},"apiKeyHelper":{"command":"/bin/echo"}}`},
		{name: "non-empty array", contents: `{"env":{"ANTHROPIC_API_KEY":["synthetic-secret"]},"apiKeyHelper":["/bin/echo"]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			conflicts, err := ClaudeCredentialConflicts(writeClaudeSettingsForTest(t, test.contents))
			if err != nil || len(conflicts) != 0 {
				t.Fatalf("conflicts = %#v, %v", conflicts, err)
			}
		})
	}
}

// TestClaudeCredentialConflictsReportsBlankButNonEmptyValues covers the one
// string shape that looks empty and is not: Claude receives it, uses it, and
// fails to authenticate, which is exactly the confusing state the advisory
// explains. Trimming it away would be a false negative.
func TestClaudeCredentialConflictsReportsBlankButNonEmptyValues(t *testing.T) {
	conflicts, err := ClaudeCredentialConflicts(writeClaudeSettingsForTest(t, `{"env":{"ANTHROPIC_API_KEY":" "},"apiKeyHelper":"\t"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 2 || conflicts[0] != ClaudeConflictAPIKey || conflicts[1] != ClaudeConflictAPIKeyHelper {
		t.Fatalf("conflicts = %#v, want both blank-but-present sources", conflicts)
	}
}

// TestSwitchAdvisoriesAreClientSpecificAndScopeConflictsToClaudeOfficial pins
// advisories a completed switch carries. Codex gets its application-boundary
// note. The Claude restart note applies to every Claude switch; the conflict
// note only to a built-in-provider selection,
// which is the selection an unowned credential source overrides.
func TestSwitchAdvisoriesAreClientSpecificAndScopeConflictsToClaudeOfficial(t *testing.T) {
	settings := writeClaudeSettingsForTest(t, `{"env":{"ANTHROPIC_API_KEY":"synthetic-secret"},"apiKeyHelper":"/bin/echo helper"}`)
	service := Service{}

	if advisories := service.SwitchAdvisories(ClientCodex, OfficialProviderName, settings); len(advisories) != 1 ||
		!strings.Contains(advisories[0], "start a new or restart the running Codex session") {
		t.Fatalf("codex switch advisories = %#v", advisories)
	}

	custom := service.SwitchAdvisories(ClientClaude, "example", settings)
	if len(custom) != 1 || !strings.Contains(custom[0], "restart running Claude sessions") {
		t.Fatalf("custom claude advisories = %#v", custom)
	}

	official := service.SwitchAdvisories(ClientClaude, OfficialProviderName, settings)
	if len(official) != 3 {
		t.Fatalf("official claude advisories = %#v", official)
	}
	if !strings.Contains(official[0], ClaudeConflictAPIKey) || !strings.Contains(official[1], ClaudeConflictAPIKeyHelper) {
		t.Fatalf("official claude conflict advisories = %#v", official)
	}
	if !strings.Contains(official[2], "restart running Claude sessions") {
		t.Fatalf("restart advisory missing or misordered: %#v", official)
	}
	for _, advisory := range official {
		if strings.Contains(advisory, "synthetic-secret") {
			t.Fatalf("advisory leaked a credential value: %q", advisory)
		}
	}
}

// TestSwitchAdvisoriesSurviveAnUnreadableSettingsFile keeps an informational
// note from turning into a failure on a switch that already completed.
func TestSwitchAdvisoriesSurviveAnUnreadableSettingsFile(t *testing.T) {
	service := Service{}
	missing := filepath.Join(t.TempDir(), "absent.json")
	for name, path := range map[string]string{
		"missing file":  missing,
		"invalid json":  writeClaudeSettingsForTest(t, `{"env":`),
		"unresolvable":  "",
		"empty content": writeClaudeSettingsForTest(t, ``),
	} {
		t.Run(name, func(t *testing.T) {
			advisories := service.SwitchAdvisories(ClientClaude, OfficialProviderName, path)
			if len(advisories) != 1 || !strings.Contains(advisories[0], "restart running Claude sessions") {
				t.Fatalf("advisories = %#v, want only the restart note", advisories)
			}
		})
	}
}

// TestSwitchAdvisoriesResolveTheDefaultPathWhenNoOverrideIsGiven covers the
// invocation shape every normal user takes, where the CLI passes no
// --config-path and the settings file is the managed one under Home.
func TestSwitchAdvisoriesResolveTheDefaultPathWhenNoOverrideIsGiven(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"apiKeyHelper":"/bin/echo helper"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	advisories := Service{Home: home}.SwitchAdvisories(ClientClaude, OfficialProviderName, "")
	if len(advisories) != 2 || !strings.Contains(advisories[0], ClaudeConflictAPIKeyHelper) {
		t.Fatalf("advisories = %#v", advisories)
	}
}
