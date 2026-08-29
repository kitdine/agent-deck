package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withUsageSignalsHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	previous := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = previous })
	return home
}

func TestUsageSignalsCommandUsesEnvelopeAndHasNoTopFlag(t *testing.T) {
	withUsageSignalsHome(t)
	state := filepath.Join(t.TempDir(), "state")
	var output bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "usage", "signals", "--period", "7d", "--client", "codex", "--kind", "workflow"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("decode usage signals envelope: %v\n%s", err, output.String())
	}
	if envelope["command"] != "usage.signals" {
		t.Fatalf("command = %#v, want usage.signals", envelope["command"])
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok || data["period"] != "7d" || data["client"] != "codex" || data["workflow"] == nil || data["activity"] != nil || data["tooling"] != nil {
		t.Fatalf("data = %#v, want one selected workflow family", envelope["data"])
	}
	if err := run([]string{"--state-dir", state, "usage", "signals", "--top", "1"}, bytes.NewReader(nil), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("--top error = %v, want unknown flag", err)
	}
}

func TestUsageSignalsAndSessionShowReadTheSameSafeDerivation(t *testing.T) {
	home := withUsageSignalsHome(t)
	state := filepath.Join(t.TempDir(), "state")
	source := filepath.Join(home, ".claude", "projects", "safe.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
		t.Fatal(err)
	}
	secretPath := filepath.Join(t.TempDir(), "private", "cache.go")
	content := strings.Join([]string{
		`{"type":"user","sessionId":"safe-session","timestamp":"2026-08-29T08:00:00Z","cwd":"/private/project","message":{"role":"user","content":"implement the cache"}}`,
		`{"type":"assistant","sessionId":"safe-session","timestamp":"2026-08-29T08:00:04Z","cwd":"/private/project","message":{"role":"assistant","id":"m1","model":"claude-test","usage":{"input_tokens":100,"output_tokens":10},"content":[{"type":"tool_use","id":"t1","name":"Edit","input":{"file_path":"` + secretPath + `"}}]}}`,
		`{"type":"user","sessionId":"safe-session","timestamp":"2026-08-29T08:00:05Z","cwd":"/private/project","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(source, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--state-dir", state, "session", "scan"}, bytes.NewReader(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var signals bytes.Buffer
	if err := run([]string{"--state-dir", state, "--no-color", "usage", "signals", "--period", "all", "--client", "claude", "--sub"}, bytes.NewReader(nil), &signals); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"🧭 ACTIVITY", "Coding", "🧱 WORKFLOW", "🔧 TOOLING", "Edit"} {
		if !strings.Contains(signals.String(), want) {
			t.Fatalf("usage signals missing %q:\n%s", want, signals.String())
		}
	}
	for _, forbidden := range []string{secretPath, filepath.Dir(secretPath), "implement the cache", `file_path`} {
		if strings.Contains(signals.String(), forbidden) {
			t.Fatalf("usage signals leaked %q:\n%s", forbidden, signals.String())
		}
	}

	var shown bytes.Buffer
	if err := run([]string{"--state-dir", state, "session", "show", "safe-session", "--client", "claude", "--activity"}, bytes.NewReader(nil), &shown); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shown.String(), "SIGNALS") || !strings.Contains(shown.String(), "1 tool call · 1 file · first edit 4s") {
		t.Fatalf("session show has no work-signal line:\n%s", shown.String())
	}
}
