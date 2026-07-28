package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newClaudeAdvisoryFixture creates an isolated state directory with one custom
// Claude provider and a settings file holding both unowned credential sources
// plus unrelated keys AgentDeck must carry through untouched.
func newClaudeAdvisoryFixture(t *testing.T, settingsContents string) (state, settings string) {
	t.Helper()
	root := t.TempDir()
	state, settings = filepath.Join(root, "state"), filepath.Join(root, "settings.json")
	if err := os.WriteFile(settings, []byte(settingsContents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--state-dir", state, "provider", "add", "example", "--endpoint", "https://provider.example", "--clients", "claude"}, bytes.NewBufferString("synthetic-secret\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	return state, settings
}

const conflictingClaudeSettings = `{"env":{"ANTHROPIC_API_KEY":"unowned-secret","UNRELATED":true},"apiKeyHelper":"/bin/echo unowned-helper","unrelatedTop":1}`

// TestOfficialClaudeSwitchReportsUnownedCredentialSourcesWithoutRemovingThem is
// the task's acceptance criterion: the advisories reach stderr, and the switch
// neither writes nor deletes a field AgentDeck does not own.
func TestOfficialClaudeSwitchReportsUnownedCredentialSourcesWithoutRemovingThem(t *testing.T) {
	state, settings := newClaudeAdvisoryFixture(t, conflictingClaudeSettings)

	_, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "use", "official", "--client", "claude", "--config-path", settings)
	if exit != 0 {
		t.Fatalf("official claude switch exit = %d: %s", exit, stderr)
	}
	for _, want := range []string{
		"advisory: env.ANTHROPIC_API_KEY in " + settings + " overrides the official selection",
		"advisory: apiKeyHelper in " + settings + " overrides the official selection",
		"advisory: restart running Claude sessions",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	if strings.Contains(stderr, "unowned-secret") || strings.Contains(stderr, "unowned-helper") {
		t.Fatalf("advisory leaked a credential value: %q", stderr)
	}

	var document map[string]any
	contents, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	environment := document["env"].(map[string]any)
	if environment["ANTHROPIC_API_KEY"] != "unowned-secret" || document["apiKeyHelper"] != "/bin/echo unowned-helper" {
		t.Fatalf("switch changed an unowned credential source: %s", contents)
	}
	if environment["UNRELATED"] != true || document["unrelatedTop"] != float64(1) {
		t.Fatalf("switch changed an unrelated field: %s", contents)
	}
	if _, present := environment["ANTHROPIC_BASE_URL"]; present {
		t.Fatalf("official switch kept an owned endpoint: %s", contents)
	}
	if _, present := environment["ANTHROPIC_AUTH_TOKEN"]; present {
		t.Fatalf("official switch kept an owned credential: %s", contents)
	}
}

// TestSwitchAdvisoriesStayOutOfTheJSONEnvelope pins the second acceptance
// criterion: the envelope on stdout is identical, field for field, to the same
// switch made with no conflicting source present.
func TestSwitchAdvisoriesStayOutOfTheJSONEnvelope(t *testing.T) {
	conflicted, conflictedSettings := newClaudeAdvisoryFixture(t, conflictingClaudeSettings)
	clean, cleanSettings := newClaudeAdvisoryFixture(t, `{"env":{"UNRELATED":true},"unrelatedTop":1}`)

	envelopeFor := func(t *testing.T, state, settings string) map[string]any {
		t.Helper()
		stdout, stderr, exit := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "use", "official", "--client", "claude", "--config-path", settings)
		if exit != 0 {
			t.Fatalf("switch exit = %d: %s", exit, stderr)
		}
		if strings.Contains(stdout, "advisory") {
			t.Fatalf("advisory entered the JSON envelope: %q", stdout)
		}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
			t.Fatalf("envelope = %q: %v", stdout, err)
		}
		// generated_at is a timestamp, so it is the one field two separate
		// runs may legitimately differ on.
		delete(envelope, "generated_at")
		return envelope
	}

	withConflict := envelopeFor(t, conflicted, conflictedSettings)
	withoutConflict := envelopeFor(t, clean, cleanSettings)
	if !jsonEqual(t, withConflict, withoutConflict) {
		t.Fatalf("envelope with advisories = %#v, without = %#v", withConflict, withoutConflict)
	}
}

func jsonEqual(t *testing.T, left, right any) bool {
	t.Helper()
	encodedLeft, err := json.Marshal(left)
	if err != nil {
		t.Fatal(err)
	}
	encodedRight, err := json.Marshal(right)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Equal(encodedLeft, encodedRight)
}

// TestSwitchAdvisoryScopeAndQuietSuppression covers which client-specific
// advisory a switch carries and that --quiet silences it like every other
// stderr note this command prints.
func TestSwitchAdvisoryScopeAndQuietSuppression(t *testing.T) {
	state, settings := newClaudeAdvisoryFixture(t, conflictingClaudeSettings)

	_, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "use", "example", "--client", "claude", "--config-path", settings)
	if exit != 0 {
		t.Fatalf("custom claude switch exit = %d: %s", exit, stderr)
	}
	if !strings.Contains(stderr, "advisory: restart running Claude sessions") {
		t.Fatalf("custom claude switch missing the restart advisory: %q", stderr)
	}
	if strings.Contains(stderr, "overrides the official selection") {
		t.Fatalf("custom claude switch reported an official-only conflict: %q", stderr)
	}

	codexState, codexConfig := newRouteSurfaceFixture(t)
	_, codexStderr, exit := runRouteCommand(t, "--state-dir", codexState, "provider", "use", "example", "--config-path", codexConfig)
	if exit != 0 {
		t.Fatalf("codex switch exit = %d: %s", exit, codexStderr)
	}
	if !strings.Contains(codexStderr, "advisory: start a new or restart the running Codex session to ensure this switch is applied") {
		t.Fatalf("codex switch missing application-boundary advisory: %q", codexStderr)
	}
	if strings.Contains(codexStderr, "running client reads its settings file live") ||
		strings.Contains(codexStderr, "overrides the official selection") {
		t.Fatalf("codex switch carried a Claude-only advisory: %q", codexStderr)
	}

	_, quietStderr, exit := runRouteCommand(t, "--state-dir", state, "--quiet", "provider", "use", "official", "--client", "claude", "--config-path", settings)
	if exit != 0 {
		t.Fatalf("quiet switch exit = %d: %s", exit, quietStderr)
	}
	if quietStderr != "" {
		t.Fatalf("--quiet still printed an advisory: %q", quietStderr)
	}

	_, quietCodexStderr, exit := runRouteCommand(t, "--state-dir", codexState, "--quiet", "provider", "use", "example", "--config-path", codexConfig)
	if exit != 0 {
		t.Fatalf("quiet codex switch exit = %d: %s", exit, quietCodexStderr)
	}
	if quietCodexStderr != "" {
		t.Fatalf("--quiet codex switch still printed an advisory: %q", quietCodexStderr)
	}
}
