package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newRouteSurfaceFixture creates an isolated state directory holding one
// custom Codex provider plus a client config the switch commands may rewrite.
func newRouteSurfaceFixture(t *testing.T) (state, config string) {
	t.Helper()
	root := t.TempDir()
	state, config = filepath.Join(root, "state"), filepath.Join(root, "config.toml")
	if err := os.WriteFile(config, []byte("model = 'keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--state-dir", state, "provider", "add", "example", "--endpoint", "https://provider.example", "--clients", "codex"}, bytes.NewBufferString("synthetic-secret\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	return state, config
}

func runRouteCommand(t *testing.T, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	var out, errOut bytes.Buffer
	exit = execute(args, bytes.NewReader(nil), &out, &errOut)
	return out.String(), errOut.String(), exit
}

func routeEnvelopeData(t *testing.T, encoded string) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(encoded), &envelope); err != nil {
		t.Fatalf("decode envelope %q: %v", encoded, err)
	}
	return envelope.Data
}

func routeEnvelopeList(t *testing.T, encoded string) []map[string]any {
	t.Helper()
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(encoded), &envelope); err != nil {
		t.Fatalf("decode envelope %q: %v", encoded, err)
	}
	return envelope.Data
}

// TestProviderSetWrapperNormalizesStoresAndClears pins the storage half of the
// surface: --url normalizes like a Codex-bound endpoint before any write, and
// --clear is the one path that writes an empty value straight through.
func TestProviderSetWrapperNormalizesStoresAndClears(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)

	stdout, _, exit := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "set-wrapper", "example", "--url", "https://wrapper.example/v1/")
	if exit != 0 {
		t.Fatalf("set-wrapper exit = %d: %s", exit, stdout)
	}
	if data := routeEnvelopeData(t, stdout); data["wrapper_url"] != "https://wrapper.example" {
		t.Fatalf("stored wrapper_url = %#v, want the normalized base", data["wrapper_url"])
	}

	shown, _, exit := runRouteCommand(t, "--state-dir", state, "provider", "show", "example")
	if exit != 0 || !strings.Contains(shown, "wrapper: https://wrapper.example\n") {
		t.Fatalf("provider show = %q (exit %d)", shown, exit)
	}
	listed, _, exit := runRouteCommand(t, "--state-dir", state, "provider", "list")
	if exit != 0 || !strings.Contains(listed, "WRAPPER") || !strings.Contains(listed, "https://wrapper.example") {
		t.Fatalf("provider list = %q (exit %d)", listed, exit)
	}

	cleared, _, exit := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "set-wrapper", "example", "--clear")
	if exit != 0 {
		t.Fatalf("set-wrapper --clear exit = %d: %s", exit, cleared)
	}
	if _, present := routeEnvelopeData(t, cleared)["wrapper_url"]; present {
		t.Fatalf("cleared wrapper still reported: %s", cleared)
	}
	shown, _, exit = runRouteCommand(t, "--state-dir", state, "provider", "show", "example")
	if exit != 0 || strings.Contains(shown, "wrapper:") {
		t.Fatalf("provider show after --clear = %q (exit %d)", shown, exit)
	}
}

// TestProviderDefinitionJSONCarriesWrapperURLForBothProviderKinds pins the
// additive JSON field on every command that reports a definition, for the
// stored-row provider and for the built-in one whose wrapper lives outside
// the providers table. The field is omitted, not empty, when no wrapper is
// configured.
func TestProviderDefinitionJSONCarriesWrapperURLForBothProviderKinds(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)
	for _, name := range []string{"example", "official"} {
		if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", name, "--url", "https://"+name+"-wrapper.example"); exit != 0 {
			t.Fatalf("set-wrapper %s exit = %d: %s", name, exit, stderr)
		}
	}

	shown, _, _ := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "show", "example")
	if definition := routeEnvelopeData(t, shown)["definition"].(map[string]any); definition["wrapper_url"] != "https://example-wrapper.example" {
		t.Fatalf("provider show definition = %#v", definition)
	}
	listed, _, _ := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "list")
	wrappers := map[string]any{}
	for _, item := range routeEnvelopeList(t, listed) {
		definition := item["definition"].(map[string]any)
		wrappers[definition["name"].(string)] = definition["wrapper_url"]
	}
	if wrappers["example"] != "https://example-wrapper.example" || wrappers["official"] != "https://official-wrapper.example" {
		t.Fatalf("provider list wrappers = %#v", wrappers)
	}
	status, _, _ := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "status")
	for _, item := range routeEnvelopeList(t, status) {
		definition := item["definition"].(map[string]any)
		if definition["wrapper_url"] != wrappers[definition["name"].(string)] {
			t.Fatalf("provider status definition = %#v", definition)
		}
	}

	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--clear"); exit != 0 {
		t.Fatalf("clear exit = %d: %s", exit, stderr)
	}
	shown, _, _ = runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "show", "example")
	if definition := routeEnvelopeData(t, shown)["definition"].(map[string]any); definition["wrapper_url"] != nil {
		t.Fatalf("cleared wrapper still present in JSON: %#v", definition)
	}
	shown, _, _ = runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "show", "official")
	if definition := routeEnvelopeData(t, shown)["definition"].(map[string]any); definition["wrapper_url"] != "https://official-wrapper.example" {
		t.Fatalf("clearing a custom wrapper changed the built-in one: %#v", definition)
	}
}

// TestProviderStatusJSONReportsSelectionRoute covers the active-selection
// half of the route contract, which travels through its own struct rather
// than the one provider current uses.
func TestProviderStatusJSONReportsSelectionRoute(t *testing.T) {
	state, config := newRouteSurfaceFixture(t)
	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--url", "https://wrapper.example"); exit != 0 {
		t.Fatalf("set-wrapper exit = %d: %s", exit, stderr)
	}

	activeSelection := func(t *testing.T) map[string]any {
		t.Helper()
		status, _, _ := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "status", "example")
		active, ok := routeEnvelopeData(t, status)["active"].([]any)
		if !ok || len(active) != 1 {
			t.Fatalf("active selections = %#v", routeEnvelopeData(t, status)["active"])
		}
		return active[0].(map[string]any)
	}

	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "use", "example", "--config-path", config, "--via"); exit != 0 {
		t.Fatalf("use --via exit = %d: %s", exit, stderr)
	}
	if selection := activeSelection(t); selection["via_wrapper"] != true || selection["endpoint"] != "https://wrapper.example" {
		t.Fatalf("active selection after --via = %#v", selection)
	}

	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "use", "example", "--config-path", config); exit != 0 {
		t.Fatalf("direct use exit = %d: %s", exit, stderr)
	}
	if selection := activeSelection(t); selection["via_wrapper"] != false || selection["endpoint"] != "https://provider.example" {
		t.Fatalf("active selection after direct switch = %#v", selection)
	}
}

// TestProviderSetWrapperRequiresExactlyOneIntent keeps the two intents
// explicit: neither flag cannot be read as "clear", and both together cannot
// be read as either one.
func TestProviderSetWrapperRequiresExactlyOneIntent(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "neither", args: []string{"provider", "set-wrapper", "example"}},
		{name: "both", args: []string{"provider", "set-wrapper", "example", "--url", "https://wrapper.example", "--clear"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, stderr, exit := runRouteCommand(t, append([]string{"--state-dir", state, "--format", "json"}, test.args...)...)
			if exit != 2 {
				t.Fatalf("exit = %d, want 2: %s", exit, stderr)
			}
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Error.Code != "invalid_argument" {
				t.Fatalf("error code = %q", envelope.Error.Code)
			}
		})
	}
	shown, _, _ := runRouteCommand(t, "--state-dir", state, "provider", "show", "example")
	if strings.Contains(shown, "wrapper:") {
		t.Fatalf("rejected invocation still stored a wrapper: %q", shown)
	}
}

// TestProviderSetWrapperOnOfficialUsesItsOwnStorage covers the built-in
// provider, whose wrapper URL is its only stored state and lives outside the
// providers table.
func TestProviderSetWrapperOnOfficialUsesItsOwnStorage(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)
	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "official", "--url", "https://official-wrapper.example/"); exit != 0 {
		t.Fatalf("official set-wrapper exit = %d: %s", exit, stderr)
	}
	shown, _, _ := runRouteCommand(t, "--state-dir", state, "provider", "show", "official")
	if !strings.Contains(shown, "wrapper: https://official-wrapper.example\n") {
		t.Fatalf("official show = %q", shown)
	}
	custom, _, _ := runRouteCommand(t, "--state-dir", state, "provider", "show", "example")
	if strings.Contains(custom, "wrapper:") {
		t.Fatalf("official wrapper leaked onto a custom provider: %q", custom)
	}
}

// TestProviderUseViaWrapperWritesWrapperEndpointAndReportsRoute covers the
// switch half: --via changes only the endpoint written, the recorded route is
// readable afterwards, and the effective route is reported on stderr because
// the client file alone cannot distinguish the two routes.
func TestProviderUseViaWrapperWritesWrapperEndpointAndReportsRoute(t *testing.T) {
	state, config := newRouteSurfaceFixture(t)
	if _, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "set-wrapper", "example", "--url", "https://wrapper.example"); exit != 0 {
		t.Fatalf("set-wrapper exit = %d: %s", exit, stderr)
	}

	_, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "use", "example", "--config-path", config, "--via")
	if exit != 0 {
		t.Fatalf("use --via exit = %d: %s", exit, stderr)
	}
	if want := "effective route: codex via wrapper, endpoint https://wrapper.example\n"; !strings.Contains(stderr, want) {
		t.Fatalf("use --via stderr = %q, want %q", stderr, want)
	}
	contents, err := os.ReadFile(config)
	if err != nil || !strings.Contains(string(contents), `base_url = 'https://wrapper.example/v1'`) {
		t.Fatalf("config after --via = %q, %v", contents, err)
	}
	if !strings.Contains(string(contents), "synthetic-secret") {
		t.Fatalf("--via dropped the provider credential: %q", contents)
	}
	current, _, _ := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "current")
	selections := routeEnvelopeList(t, current)
	if len(selections) != 1 || selections[0]["via_wrapper"] != true || selections[0]["endpoint"] != "https://wrapper.example" {
		t.Fatalf("current after --via = %#v", selections)
	}
	currentText, _, _ := runRouteCommand(t, "--state-dir", state, "provider", "current")
	if !strings.Contains(currentText, "ROUTE") || !strings.Contains(currentText, "via wrapper") {
		t.Fatalf("current text after --via = %q", currentText)
	}
	statusText, _, _ := runRouteCommand(t, "--state-dir", state, "provider", "status", "example")
	if !strings.Contains(statusText, "ROUTE") || !strings.Contains(statusText, "via wrapper") {
		t.Fatalf("status text after --via = %q", statusText)
	}

	_, stderr, exit = runRouteCommand(t, "--state-dir", state, "provider", "use", "example", "--config-path", config)
	if exit != 0 {
		t.Fatalf("direct use exit = %d: %s", exit, stderr)
	}
	if want := "effective route: codex direct, endpoint https://provider.example\n"; !strings.Contains(stderr, want) {
		t.Fatalf("direct use stderr = %q, want %q", stderr, want)
	}
	contents, err = os.ReadFile(config)
	if err != nil || !strings.Contains(string(contents), `base_url = 'https://provider.example/v1'`) {
		t.Fatalf("config after direct switch = %q, %v", contents, err)
	}
	current, _, _ = runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "current")
	selections = routeEnvelopeList(t, current)
	if len(selections) != 1 || selections[0]["via_wrapper"] != false || selections[0]["endpoint"] != "https://provider.example" {
		t.Fatalf("current after direct switch = %#v", selections)
	}
}

// TestProviderUseViaWithoutConfiguredWrapperFailsBeforeTouchingClientFile is
// the task's named acceptance criterion.
func TestProviderUseViaWithoutConfiguredWrapperFailsBeforeTouchingClientFile(t *testing.T) {
	state, config := newRouteSurfaceFixture(t)
	before, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "use", "example", "--config-path", config, "--via")
	if exit == 0 {
		t.Fatalf("--via without a wrapper succeeded: %s%s", stdout, stderr)
	}
	if strings.Contains(stderr, "effective route") {
		t.Fatalf("failed switch reported a route: %q", stderr)
	}
	after, err := os.ReadFile(config)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("client file changed on a rejected --via switch: %q, %v", after, err)
	}
	current, _, _ := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "current")
	if selections := routeEnvelopeList(t, current); len(selections) != 0 {
		t.Fatalf("rejected --via switch recorded a selection: %#v", selections)
	}
}

// TestProviderUseEffectiveRouteStaysOutOfStdoutAndRespectsQuiet pins the
// advisory's boundaries: it is stderr-only in both formats and suppressed by
// --quiet, so no automation reading stdout can see it.
func TestProviderUseEffectiveRouteStaysOutOfStdoutAndRespectsQuiet(t *testing.T) {
	state, config := newRouteSurfaceFixture(t)

	stdout, stderr, exit := runRouteCommand(t, "--state-dir", state, "--format", "json", "provider", "use", "example", "--config-path", config)
	if exit != 0 {
		t.Fatalf("json use exit = %d: %s", exit, stderr)
	}
	if strings.Contains(stdout, "effective route") {
		t.Fatalf("effective route entered the JSON envelope: %q", stdout)
	}
	if !strings.Contains(stderr, "effective route: codex direct, endpoint https://provider.example") {
		t.Fatalf("json run stderr = %q", stderr)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("json envelope = %q: %v", stdout, err)
	}

	_, quietStderr, exit := runRouteCommand(t, "--state-dir", state, "--quiet", "provider", "use", "example", "--config-path", config)
	if exit != 0 {
		t.Fatalf("quiet use exit = %d: %s", exit, quietStderr)
	}
	if quietStderr != "" {
		t.Fatalf("--quiet still reported a route: %q", quietStderr)
	}
}

// TestProviderUseOfficialClaudeReportsNoEndpointWritten covers the one route
// that writes no endpoint at all, which the route line must not describe as
// an empty endpoint.
func TestProviderUseOfficialClaudeReportsNoEndpointWritten(t *testing.T) {
	state, _ := newRouteSurfaceFixture(t)
	settings := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(settings, []byte(`{"env":{"UNRELATED":true}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, exit := runRouteCommand(t, "--state-dir", state, "provider", "use", "official", "--client", "claude", "--config-path", settings)
	if exit != 0 {
		t.Fatalf("official claude use exit = %d: %s", exit, stderr)
	}
	if want := "effective route: claude direct, no endpoint written\n"; !strings.Contains(stderr, want) {
		t.Fatalf("official claude stderr = %q, want %q", stderr, want)
	}
}
