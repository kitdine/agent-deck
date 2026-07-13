package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/jobshen/agentdeck/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestGlobals(t *testing.T) {
	state, format, args, err := globals([]string{"--state-dir", "/tmp/state", "--format", "json", "provider", "list"})
	if err != nil || state != "/tmp/state" || format != "json" || len(args) != 2 {
		t.Fatalf("globals = %q, %q, %#v, %v", state, format, args, err)
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
	if !bytes.Contains(text.Bytes(), []byte(`"events":0`)) {
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

func TestReadCredentialFromPipe(t *testing.T) {
	value, err := readCredential(bytes.NewBufferString("synthetic-secret\n"))
	if err != nil || value != "synthetic-secret" {
		t.Fatalf("readCredential = %q, %v", value, err)
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
	if output := runSession("list"); !bytes.Contains([]byte(output), []byte(`"SessionID":"s"`)) {
		t.Fatalf("session list omitted metadata: %s", output)
	}
	for _, args := range [][]string{{"search", "approved"}, {"show", "codex", "s"}} {
		if output := runSession(args...); !bytes.Contains([]byte(output), []byte("approved")) {
			t.Fatalf("session %v omitted approved content: %s", args, output)
		}
	}
	runSession("search", `"forbidden-secret"`)
	runSession("exclude", "session", "s")
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
