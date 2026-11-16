package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/watch"
)

func watchOriginFixture(t *testing.T) (string, string) {
	t.Helper()
	home, state := t.TempDir(), t.TempDir()
	path := filepath.Join(home, ".claude", "projects", "project", "readable.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"sessionId":"watch-work","timestamp":"2026-11-15T12:00:00Z","type":"user","message":{"role":"user","content":"hello"}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = previous })
	return home, state
}

func runOneSessionWatchEvent(t *testing.T, state string) watch.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	writer := &cancelAfterLineWriter{cancel: cancel}
	command := newRootCommand(bytes.NewReader(nil), writer)
	command.SetContext(ctx)
	command.SetArgs([]string{"--state-dir", state, "--format", "ndjson", "watch", "--domains", "session", "--interval", "10ms"})
	if err := command.Execute(); err != nil {
		t.Fatalf("watch: %v", err)
	}
	var event watch.Event
	if err := json.Unmarshal(bytes.TrimSpace(writer.Bytes()), &event); err != nil {
		t.Fatalf("watch emitted no valid event: %q %v", writer.String(), err)
	}
	return event
}

func TestSessionOnlyWatchClassifiesWhileIdentityIsPending(t *testing.T) {
	home, state := watchOriginFixture(t)
	core, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	missing := filepath.Join(home, ".codex", "sessions", "unreadable-copy.jsonl")
	if _, err := core.DB.Exec(`INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES(?,'old',0,0,'')`, missing); err != nil {
		t.Fatal(err)
	}
	if _, err := core.DB.Exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset)
 VALUES('pending','codex','parent','pending','2026-11-15T12:00:00Z','m',10,1,?,0)`, missing); err != nil {
		t.Fatal(err)
	}
	event := runOneSessionWatchEvent(t, state)
	if event.Type != "scan_pending" || event.Reason != "source_identity_pending" || event.Domain != "session" {
		t.Fatalf("pending event: %+v", event)
	}
	var automatic, reason string
	if err := core.DB.QueryRow(`SELECT automatic,reason_code FROM session_origins WHERE client='claude' AND session_id='watch-work'`).Scan(&automatic, &reason); err != nil || automatic != "work" || reason != "default_work" {
		t.Fatalf("classification: %s %s %v", automatic, reason, err)
	}
	if _, found, err := core.Setting(context.Background(), "watch.fingerprint.session"); err != nil || found {
		t.Fatalf("pending checkpoint acknowledged: %v %v", found, err)
	}
}

func TestSessionWatchRefreshesLegacyRuleCheckpoint(t *testing.T) {
	home, state := watchOriginFixture(t)
	ctx := context.Background()
	core, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	index, err := store.OpenSessions(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	if _, err := session.ScanWithOptions(ctx, index.DB, home, session.ScanOptions{}); err != nil {
		t.Fatal(err)
	}
	raw, err := sessionInventoryFingerprint(home)
	if err != nil {
		t.Fatal(err)
	}
	epoch, err := index.SessionIndexEpoch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.SetSetting(ctx, "watch.fingerprint.session", fmt.Sprintf("v1:%d:%s", epoch, raw)); err != nil {
		t.Fatal(err)
	}
	if event := runOneSessionWatchEvent(t, state); event.Type != "scan_completed" {
		t.Fatalf("rule refresh event: %+v", event)
	}
	var automatic, reason string
	if err := core.DB.QueryRow(`SELECT automatic,reason_code FROM session_origins WHERE client='claude' AND session_id='watch-work'`).Scan(&automatic, &reason); err != nil || automatic != "work" || reason != "default_work" {
		t.Fatalf("rule refresh: %s %s %v", automatic, reason, err)
	}
}

func TestWatchPendingTextReportsAutomaticRetry(t *testing.T) {
	var output bytes.Buffer
	err := renderWatchText(&output, watch.Event{Type: "scan_pending", Domain: "session", Reason: "source_identity_pending", GeneratedAt: time.Unix(1, 0)})
	if err != nil || !bytes.Contains(output.Bytes(), []byte("scan pending")) || !bytes.Contains(output.Bytes(), []byte("Retrying automatically")) || bytes.Contains(output.Bytes(), []byte("scan completed")) {
		t.Fatalf("pending text: %q %v", output.String(), err)
	}
}
