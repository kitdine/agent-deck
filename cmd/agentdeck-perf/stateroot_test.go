package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolatedBatch is a batch HOME under the App's accepted test prefix with a
// one-file source tree and a template of tiny state files, enough for run
// unit resets without a corpus or a product scan.
func isolatedBatch(t *testing.T) *batchState {
	t.Helper()
	root, err := os.MkdirTemp("/private/tmp", "agentdeck-perf.")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	root = appPathForm(root)
	b := &batchState{Root: root, Home: filepath.Join(root, "home"), TZ: "America/New_York"}
	source := filepath.Join(b.Home, ".codex", "sessions", "source.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	template := filepath.Join(root, "template")
	if err := os.Mkdir(template, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"agentdeck.sqlite3": "core", "sessions.sqlite3": "sessions", "desktop-serving-snapshot.json": "{}"} {
		if err := os.WriteFile(filepath.Join(template, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, field := range map[string]*string{"agentdeck.sqlite3": &b.Template.CoreSHA256, "sessions.sqlite3": &b.Template.SessionSHA256, "desktop-serving-snapshot.json": &b.Template.ServingSHA256} {
		if *field, err = fileSHA256(filepath.Join(template, name)); err != nil {
			t.Fatal(err)
		}
	}
	if b.Binding, err = sourceBinding(b.Home); err != nil {
		t.Fatal(err)
	}
	return b
}

// productStateRoot asks the measured binary itself, as the App's helper is
// asked, where its default state for this HOME lives.
func productStateRoot(t *testing.T, binary, home string) string {
	t.Helper()
	command := exec.Command(binary, "desktop", "state-location", "--format", "json")
	command.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
	addTestCLIEnvironment(command, binary)
	out, err := command.Output()
	if err != nil {
		t.Fatalf("state-location: %v", err)
	}
	var envelope struct {
		Data struct {
			StateRoot string `json:"state_root"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil || envelope.Data.StateRoot == "" {
		t.Fatalf("state-location output %q: %v", out, err)
	}
	return envelope.Data.StateRoot
}

// The template must be restored where the measured build reads its state,
// not at a fixed name: a build that is not the installed program keeps its
// default state in an isolated root, and an App started on the batch HOME
// would otherwise start from empty state while the unit records a restore.
func TestRunUnitRestoresWhereTheMeasuredBuildKeepsItsState(t *testing.T) {
	if testing.Short() {
		t.Skip("builds agentdeck")
	}
	binary := buildAgentdeck(t)
	b := isolatedBatch(t)
	cfg := runConfig{Agentdeck: binary, TZ: b.TZ, Batch: b}
	if err := bindStateRoot(&cfg, false); err != nil {
		t.Fatal(err)
	}
	if err := b.startRunUnit(stateFull); err != nil {
		t.Fatal(err)
	}
	want := productStateRoot(t, binary, b.Home)
	for _, name := range []string{"agentdeck.sqlite3", "sessions.sqlite3", "desktop-serving-snapshot.json"} {
		if _, err := os.Stat(filepath.Join(want, name)); err != nil {
			t.Fatalf("template %s not restored into the measured state root %s: %v", name, want, err)
		}
	}
	if b.State != want {
		t.Fatalf("bound state root %q, measured build uses %q", b.State, want)
	}
	if strings.HasSuffix(want, string(filepath.Separator)+".agentdeck") {
		t.Fatalf("an uninstalled build reported the installed state root %q", want)
	}
}

// reportIsolatedRoot answers like an uninstalled build: an isolated root
// beside the sources in the physical HOME.
const reportIsolatedRoot = `printf '{"schema_version":1,"command":"desktop.state-location","data":{"installed":false,"reason":"receipt_missing","state_root":"%s/.agentdeck-dev"},"warnings":[],"partial":false}\n' "$(cd "$HOME" && pwd -P)"`

// fakeStateLocation writes a helper whose desktop state-location runs body;
// any other command fails.
func fakeStateLocation(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agentdeck")
	script := "#!/bin/sh\nif [ \"$1 $2\" != \"desktop state-location\" ]; then exit 64; fi\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// fakeApp is a bundle whose embedded helper is binary.
func fakeApp(t *testing.T, binary string) string {
	t.Helper()
	app := filepath.Join(t.TempDir(), "AgentDeck.app")
	helpers := filepath.Join(app, "Contents", "Helpers")
	if err := os.MkdirAll(helpers, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(helpers, "agentdeck"), raw, 0o700); err != nil {
		t.Fatal(err)
	}
	return app
}

func envelope(root string, installed bool) string {
	return `printf '{"schema_version":1,"command":"desktop.state-location","data":{"installed":` +
		map[bool]string{true: "true", false: "false"}[installed] + `,"reason":"receipt_missing","state_root":"%s"},"warnings":[],"partial":false}\n' "` + root + `"`
}

// The root comes only from the measured binary's own answer and must stay
// inside the batch HOME; every unsupported, refused or malformed answer is an
// error, never a guessed default. An older build without the command keeps
// its documented legacy root.
func TestStateRootFollowsTheMeasuredBinaryAndStaysInTheFixture(t *testing.T) {
	physical := `"$(cd "$HOME" && pwd -P)"`
	for _, test := range []struct {
		name, body, wantBase, source string
		setup                        func(t *testing.T, home string)
	}{
		{name: "isolated", body: reportIsolatedRoot, wantBase: ".agentdeck-dev", source: stateRootReported},
		{name: "legacy build", body: `printf 'Read desktop integration data\n\nUsage:\n  agentdeck desktop [command]\n'`, wantBase: ".agentdeck", source: stateRootLegacy},
		{name: "alias inside home", body: `printf '{"schema_version":1,"command":"desktop.state-location","data":{"installed":false,"state_root":"%s/.agentdeck"}}\n' ` + physical,
			wantBase: ".agentdeck", source: stateRootReported,
			setup: func(t *testing.T, home string) {
				if err := os.Mkdir(filepath.Join(home, ".agentdeck"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(".agentdeck", filepath.Join(home, ".agentdeck-dev")); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "installed program", body: envelope(`$(cd "$HOME" && pwd -P)/.agentdeck`, true)},
		{name: "outside home", body: envelope("/private/tmp/elsewhere/.agentdeck-dev", false)},
		{name: "home itself", body: envelope(`$(cd "$HOME" && pwd -P)`, false)},
		{name: "source root", body: envelope(`$(cd "$HOME" && pwd -P)/.codex`, false)},
		{name: "relative", body: envelope(".agentdeck-dev", false)},
		{name: "unclean", body: envelope(`$(cd "$HOME" && pwd -P)/x/../.agentdeck-dev`, false)},
		{name: "root is a link", body: reportIsolatedRoot, setup: func(t *testing.T, home string) {
			if err := os.Symlink(t.TempDir(), filepath.Join(home, ".agentdeck-dev")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "refused", body: `printf '{"schema_version":1,"command":"desktop.state-location","data":null,"error":{"code":"state_ownership_refused"}}\n'; exit 1`},
		{name: "malformed", body: `printf 'not json\n'`},
		{name: "wrong command", body: `printf '{"schema_version":1,"command":"desktop.snapshot","data":{"state_root":"/x"}}\n'`},
		{name: "empty", body: `:`},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "home")
			if err := os.Mkdir(home, 0o700); err != nil {
				t.Fatal(err)
			}
			if test.setup != nil {
				test.setup(t, home)
			}
			got, err := queryStateLocation(fakeStateLocation(t, test.body), home, "UTC")
			if test.wantBase == "" {
				if err == nil {
					t.Fatalf("accepted %+v", got)
				}
				return
			}
			physicalHome, _ := filepath.EvalSymlinks(home)
			if err != nil || got.Root != filepath.Join(physicalHome, test.wantBase) || got.Source != test.source {
				t.Fatalf("location = %+v, err = %v", got, err)
			}
		})
	}
}

// The real account's state stays protected even when the batch HOME is the
// account home: the shared state-path guard refuses it before any reset.
func TestStateRootRefusesTheProtectedAccountRoot(t *testing.T) {
	if _, err := queryStateLocation(fakeStateLocation(t, envelope(`$(cd "$HOME" && pwd -P)/.agentdeck`, false)), isolatedPerfAccount, "UTC"); err == nil {
		t.Fatal("the account's real state root was accepted as a batch state root")
	}
}

// App conditions bind the root the App's helper chooses and require the CLI
// under test to agree; every run unit re-resolves before its reset, so a
// rebuilt HOME is accepted and a changed answer stops the batch.
func TestBoundStateRootIsSharedAndReResolvedEveryRunUnit(t *testing.T) {
	b := isolatedBatch(t)
	cli := fakeStateLocation(t, reportIsolatedRoot)
	cfg := runConfig{App: fakeApp(t, cli), Agentdeck: cli, TZ: "UTC", Batch: b}
	if err := bindStateRoot(&cfg, true); err != nil {
		t.Fatal(err)
	}
	physicalHome, _ := filepath.EvalSymlinks(b.Home)
	want := filepath.Join(physicalHome, ".agentdeck-dev")
	if b.State != want || b.StateSource != stateRootReported {
		t.Fatalf("bound %q (%s), want %q", b.State, b.StateSource, want)
	}
	stale := filepath.Join(want, "stale")
	if err := os.MkdirAll(stale, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := b.startRunUnit(stateNoCache); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("reset left the previous unit's state: %v", err)
	}
	if _, err := os.Stat(filepath.Join(want, "agentdeck.sqlite3")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(want, "desktop-serving-snapshot.json")); !os.IsNotExist(err) {
		t.Fatalf("no-cache unit restored the serving cache: %v", err)
	}
	if err := b.startRunUnit(stateEmpty); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(want); err != nil || len(entries) != 0 {
		t.Fatalf("empty unit state = %v, %v", entries, err)
	}

	// A writing condition rebuilds the whole HOME; the next unit re-resolves
	// the same root on the new tree.
	if err := os.RemoveAll(b.Home); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(b.Home, ".codex", "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.Home, ".codex", "sessions", "source.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var err error
	if b.Binding, err = sourceBinding(b.Home); err != nil {
		t.Fatal(err)
	}
	if err := b.startRunUnit(stateFull); err != nil || b.State != want {
		t.Fatalf("rebuilt HOME: state %q err %v", b.State, err)
	}

	// The helper's answer changes mid-batch: the unit is refused.
	changing := fakeStateLocation(t, envelope(`$(cd "$HOME" && pwd -P)/.agentdeck-other`, false))
	b.resolveState = func() (stateLocation, error) { return queryStateLocation(changing, b.Home, "UTC") }
	if err := b.startRunUnit(stateFull); err == nil {
		t.Fatal("a changed state root was used")
	}

	// The App's helper and the CLI under test must name one root.
	other := isolatedBatch(t)
	cfg = runConfig{App: fakeApp(t, cli), Agentdeck: fakeStateLocation(t, `printf 'Read desktop integration data\n\nUsage:\n  agentdeck desktop [command]\n'`), TZ: "UTC", Batch: other}
	if err := bindStateRoot(&cfg, true); err == nil {
		t.Fatal("App helper and CLI disagreeing on the state root were accepted")
	}
	// Without a binding no reset runs at all.
	if err := isolatedBatch(t).startRunUnit(stateFull); err == nil {
		t.Fatal("a run unit reset an unbound state root")
	}
}

// Worker waits match the root under every spelling the process list shows.
func TestWorkerWaitMatchesEveryStateRootSpelling(t *testing.T) {
	got := pathAliases("/tmp/agentdeck-perf.1/home/.agentdeck-dev")
	for _, want := range []string{"/tmp/agentdeck-perf.1/home/.agentdeck-dev", "/private/tmp/agentdeck-perf.1/home/.agentdeck-dev"} {
		found := false
		for _, alias := range got {
			found = found || alias == want
		}
		if !found {
			t.Fatalf("aliases %v miss %q", got, want)
		}
	}
	if got := pathAliases("/private/tmp/agentdeck-perf.1/x"); len(got) < 2 || got[1] != "/tmp/agentdeck-perf.1/x" {
		t.Fatalf("aliases = %v", got)
	}
	if workerRunning("") {
		t.Fatal("an unbound root matched a worker")
	}
}
