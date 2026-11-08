//go:build agentdeck_acceptance && darwin

package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/ownershiptest"
)

// Client-configuration refusal is proved by byte-identical client files, not
// by the state access barrier: the isolated default state lives inside the
// injected account, so only the real state root is under the barrier here.
// The barrier and its controls still prove these commands never touched it.

var clientConfigFixtureFiles = map[string]string{
	".codex/config.toml":    "model = 'gpt-5.4'\n",
	".claude/settings.json": "{\"statusLine\":{\"type\":\"command\",\"command\":\"prior-statusline\"}}\n",
	".zshrc":                "# keep\n",
}

// clientConfigTree snapshots every client-side path under the account except
// AgentDeck's own state roots, so a refusal leaving a temporary file, backup
// or new directory is caught alongside changed bytes.
func clientConfigTree(t *testing.T, account string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(account, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".agentdeck" || entry.Name() == ".agentdeck-dev" || entry.Name() == ".agentdeck-installation") {
			return filepath.SkipDir
		}
		relative, _ := filepath.Rel(account, path)
		if entry.IsDir() {
			tree[relative+"/"] = ""
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".agentdeck-installation") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tree[relative] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func seedClientConfigAccount(t *testing.T, f ownershiptest.Fixture) {
	t.Helper()
	for name, body := range clientConfigFixtureFiles {
		path := filepath.Join(f.Account, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// clientConfigChild runs the acceptance executable uninstalled with HOME set
// to the injected real account. Shell location overrides are cleared so the
// startup files resolve inside that account.
func clientConfigChild(f ownershiptest.Fixture, home string, args []string) *exec.Cmd {
	command := ownershipChild(f, args, false)
	command.Env = append(command.Env, "HOME="+home, "ZDOTDIR=", "XDG_CONFIG_HOME=", "SHELL=/bin/zsh")
	return command
}

func runClientConfigChild(t *testing.T, f ownershiptest.Fixture, home string, args []string) (string, string, int) {
	t.Helper()
	command := clientConfigChild(f, home, args)
	f.Sandbox(command)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	_ = command.Run()
	if command.ProcessState == nil {
		t.Fatalf("%v did not run: %s", args, stderr.String())
	}
	return stdout.String(), stderr.String(), command.ProcessState.ExitCode()
}

func mustClientConfigChild(t *testing.T, f ownershiptest.Fixture, home string, args ...string) {
	t.Helper()
	if stdout, stderr, code := runClientConfigChild(t, f, home, args); code != 0 {
		t.Fatalf("seed %v: exit %d\n%s%s", args, code, stdout, stderr)
	}
}

type clientConfigCase struct {
	name string
	args []string
	text bool
	seed func(t *testing.T, f ownershiptest.Fixture, state []string)
}

func clientConfigCases() []clientConfigCase {
	return []clientConfigCase{
		{name: "provider.use/codex", args: []string{"provider", "use", "official", "--client", "codex", "--no-shell-setup"}},
		{name: "provider.use/claude", args: []string{"provider", "use", "official", "--client", "claude", "--no-shell-setup"}},
		{name: "telemetry.enable", args: []string{"telemetry", "enable", "--write-codex-config"}},
		{name: "telemetry.disable", args: []string{"telemetry", "disable"}, seed: func(t *testing.T, f ownershiptest.Fixture, state []string) {
			// The managed block and its digest record are produced with a
			// temporary HOME, then the block is placed in the account so the
			// refused removal has something it would otherwise remove.
			mustClientConfigChild(t, f, f.Home, append(append([]string{}, state...), "--format", "json", "telemetry", "enable", "--write-codex-config")...)
			written, err := os.ReadFile(filepath.Join(f.Home, ".codex", "config.toml"))
			if err != nil || !bytes.Contains(written, []byte("[otel]")) {
				t.Fatalf("telemetry seed did not write a block: %q %v", written, err)
			}
			if err := os.WriteFile(filepath.Join(f.Account, ".codex", "config.toml"), written, 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "usage.hook.setup", args: []string{"usage", "hook", "setup", "--client", "all"}},
		{name: "usage.hook.remove", args: []string{"usage", "hook", "remove", "--client", "all"}},
		{name: "desktop.quota-statusline.enable", args: []string{"desktop", "quota-statusline", "enable"}, seed: func(t *testing.T, f ownershiptest.Fixture, state []string) {
			mustClientConfigChild(t, f, f.Account, append(append([]string{}, state...), "--format", "json", "desktop", "quota-settings", "--reading", "on")...)
		}},
		{name: "desktop.quota-statusline.disable", args: []string{"desktop", "quota-statusline", "disable"}},
		{name: "shell.setup", args: []string{"shell", "setup", "zsh"}, text: true},
		{name: "shell.remove", args: []string{"shell", "remove", "zsh"}, text: true},
	}
}

func TestOwnershipClientConfigRefusedUnderDefaultAndExplicitState(t *testing.T) {
	for _, group := range []string{"default-isolated", "explicit-non-real"} {
		for _, tc := range clientConfigCases() {
			t.Run(group+"/"+tc.name, func(t *testing.T) {
				f := ownershiptest.New(t)
				seedClientConfigAccount(t, f)
				var state []string
				if group == "explicit-non-real" {
					state = []string{"--state-dir", filepath.Join(f.Base, "explicit-state")}
				} else {
					// Seeds that must run under another HOME still target the
					// default isolated root of the account.
					state = []string{"--state-dir", filepath.Join(f.Account, ".agentdeck-dev")}
				}
				if tc.seed != nil {
					tc.seed(t, f, state)
				}
				before := clientConfigTree(t, f.Account)
				args := append([]string{}, tc.args...)
				if !tc.text {
					args = append([]string{"--format", "json"}, args...)
				}
				if group == "explicit-non-real" {
					args = append(append([]string{}, state...), args...)
				}
				f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
				stdout, stderr, code := runClientConfigChild(t, f, f.Account, args)
				if code != 1 {
					t.Fatalf("exit = %d, want 1\nstdout=%s\nstderr=%s", code, stdout, stderr)
				}
				if tc.text {
					if !strings.HasPrefix(stderr, "state_ownership_refused: ") || !strings.Contains(stderr, "\n  resource: client_config\n") {
						t.Fatalf("text refusal = %q", stderr)
					}
				} else {
					var envelope struct {
						Error struct {
							Code    string            `json:"code"`
							Details map[string]string `json:"details"`
						} `json:"error"`
					}
					if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
						t.Fatalf("stderr is not an error envelope: %v %q", err, stderr)
					}
					if envelope.Error.Code != "state_ownership_refused" || envelope.Error.Details["resource"] != "client_config" || envelope.Error.Details["reason"] != "receipt_missing" {
						t.Fatalf("refusal envelope = %+v", envelope.Error)
					}
				}
				if after := clientConfigTree(t, f.Account); !reflect.DeepEqual(after, before) {
					t.Fatalf("client configuration changed:\nbefore=%v\nafter=%v", before, after)
				}
			})
		}
	}
}

func TestOwnershipClientConfigGoldenAndTemporaryHomeAllowed(t *testing.T) {
	golden := ownershipGoldens(t)["refused_client_config"]
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			f := ownershiptest.New(t)
			seedClientConfigAccount(t, f)
			before := clientConfigTree(t, f.Account)
			f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
			stdout, stderr, code := runClientConfigChild(t, f, f.Account, []string{"--format", format, "telemetry", "enable", "--write-codex-config"})
			if code != golden.Exit {
				t.Fatalf("exit = %d\n%s%s", code, stdout, stderr)
			}
			if format == "text" {
				if stdout != golden.Stdout || stderr != golden.Stderr {
					t.Fatalf("prototype mismatch\nstdout=%q\nwant=%q\nstderr=%q\nwant=%q", stdout, golden.Stdout, stderr, golden.Stderr)
				}
			} else {
				var got, want map[string]any
				if err := json.Unmarshal([]byte(stderr), &got); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(golden.JSON), &want); err != nil {
					t.Fatal(err)
				}
				delete(got, "generated_at")
				delete(want, "generated_at")
				if !reflect.DeepEqual(got, want) || stdout != "" {
					t.Fatalf("prototype JSON mismatch: got=%s want=%s", stderr, golden.JSON)
				}
			}
			if after := clientConfigTree(t, f.Account); !reflect.DeepEqual(after, before) {
				t.Fatalf("client configuration changed: %v", after)
			}
		})
	}
	// The same uninstalled build may write a temporary HOME outside the account.
	f := ownershiptest.New(t)
	for name, body := range clientConfigFixtureFiles {
		path := filepath.Join(f.Home, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	mustClientConfigChild(t, f, f.Home, "--format", "json", "telemetry", "enable", "--write-codex-config")
	mustClientConfigChild(t, f, f.Home, "--format", "json", "usage", "hook", "setup", "--client", "all")
	mustClientConfigChild(t, f, f.Home, "--format", "json", "provider", "use", "official", "--client", "codex", "--no-shell-setup")
	mustClientConfigChild(t, f, f.Home, "shell", "setup", "zsh")
	if written, err := os.ReadFile(filepath.Join(f.Home, ".codex", "config.toml")); err != nil || !bytes.Contains(written, []byte("[otel]")) {
		t.Fatalf("temporary HOME config not written: %q %v", written, err)
	}
}
