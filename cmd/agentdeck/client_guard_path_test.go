package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderConfigPathPreservesSymlinkParentTraversal(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	a, b := filepath.Join(home, "a"), filepath.Join(home, "b")
	for _, dir := range []string{a, filepath.Join(b, "child")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(b, "child"), filepath.Join(a, "link")); err != nil {
		t.Fatal(err)
	}
	intended, alternate := filepath.Join(b, "config.toml"), filepath.Join(a, "config.toml")
	for path, body := range map[string]string{intended: "model = 'physical'\n", alternate: "model = 'alternate'\n"} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := run([]string{"--state-dir", state, "provider", "add", "path-test", "--endpoint", "https://example.invalid", "--clients", "codex"}, strings.NewReader("synthetic-secret\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	// filepath.Join would remove the very traversal this regression exercises.
	config := filepath.Join(a, "link") + "/../config.toml"
	if err := run([]string{"--state-dir", state, "provider", "use", "path-test", "--client", "codex", "--config-path", config, "--no-shell-setup"}, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(intended)
	if err != nil || !bytes.Contains(data, []byte("https://example.invalid/v1")) || !bytes.Contains(data, []byte("physical")) {
		t.Fatalf("intended config = %q, %v", data, err)
	}
	data, err = os.ReadFile(alternate)
	if err != nil || string(data) != "model = 'alternate'\n" {
		t.Fatalf("alternate changed: %q, %v", data, err)
	}
}
