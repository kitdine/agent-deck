package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/scanruntime"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestScanCommandReturnsOneCompletedSharedRound(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"--state-dir", state, "--format", "json", "scan"}, bytes.NewReader(nil), &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("scan exit = %d, stderr = %q", exit, stderr.String())
	}
	if bytes.Contains(stderr.Bytes(), []byte("\x1b")) || bytes.Contains(stderr.Bytes(), []byte(state)) {
		t.Fatalf("non-TTY progress is unsafe: %q", stderr.String())
	}
	var envelope struct {
		Command string          `json:"command"`
		Data    scanCommandData `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode scan envelope: %v\n%s", err, stdout.String())
	}
	if envelope.Command != "scan" || envelope.Data.Scope != scanruntime.ScopeBoth {
		t.Fatalf("scan envelope = %#v", envelope)
	}
	if envelope.Data.Usage.State != "completed" || envelope.Data.Session.State != "completed" {
		t.Fatalf("scan terminal states = usage=%#v session=%#v", envelope.Data.Usage, envelope.Data.Session)
	}
}

func TestScanCommandCompletesCombinedLegacySessionIndexMigration(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy, err := sql.Open("sqlite", filepath.Join(state, "sessions.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.ExecContext(ctx, `
CREATE TABLE session_sources (
  source_path TEXT PRIMARY KEY,
  identity TEXT NOT NULL,
  cursor INTEGER NOT NULL,
  partial_line BLOB NOT NULL DEFAULT X'',
  size INTEGER NOT NULL,
  modified_at INTEGER NOT NULL,
  prefix_hash TEXT NOT NULL,
  priority INTEGER NOT NULL,
  parser_version INTEGER NOT NULL,
  scanned_at TEXT NOT NULL
);
CREATE TABLE session_metadata (
  source_path TEXT NOT NULL,
  client TEXT NOT NULL,
  session_id TEXT NOT NULL,
  project TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  parser_version INTEGER NOT NULL,
  first_at TEXT NOT NULL,
  last_at TEXT NOT NULL,
  PRIMARY KEY(source_path, client, session_id)
);
CREATE VIRTUAL TABLE session_documents USING fts5(
  source_path UNINDEXED,
  client UNINDEXED,
  session_id UNINDEXED,
  kind UNINDEXED,
  text
);`)
	if err != nil {
		legacy.Close()
		t.Fatal(err)
	}
	if err = legacy.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"--state-dir", state, "--format", "json", "scan"}, bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
		t.Fatalf("first combined legacy scan exit=%d stderr=%q", exit, stderr.String())
	}

	sessions, err := store.OpenSessionsReadOnly(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer sessions.Close()
	for table, column := range map[string]string{
		"session_sources":   "changed_at",
		"session_documents": "event_at",
	} {
		var count int
		if err = sessions.DB.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info(?) WHERE name=?", table, column).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s.%s count=%d, want 1", table, column, count)
		}
	}
}

func TestScanCommandNDJSONStreamsProgressThenFrozenResult(t *testing.T) {
	state := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"--state-dir", state, "--format", "ndjson", "scan"}, bytes.NewReader(nil), &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 {
		t.Fatalf("scan exit=%d stderr=%q", exit, stderr.String())
	}
	lines := bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("scan stream=%q", stdout.String())
	}
	var lastSequence uint64
	for index, line := range lines {
		var envelope scanEventEnvelope
		if err := json.Unmarshal(line, &envelope); err != nil {
			t.Fatalf("line %d: %v\n%s", index, err, line)
		}
		if envelope.Command != "scan" || envelope.SchemaVersion != 1 || envelope.Scope != scanruntime.ScopeBoth {
			t.Fatalf("line %d envelope=%#v", index, envelope)
		}
		if index == len(lines)-1 {
			if envelope.Type != "result" || envelope.Partial {
				t.Fatalf("terminal envelope=%#v", envelope)
			}
			continue
		}
		if envelope.Type != "progress" {
			t.Fatalf("line %d type=%q", index, envelope.Type)
		}
		encoded, err := json.Marshal(envelope.Data)
		if err != nil {
			t.Fatal(err)
		}
		var progress scanruntime.Progress
		if err = json.Unmarshal(encoded, &progress); err != nil {
			t.Fatal(err)
		}
		if index > 0 && progress.Sequence <= lastSequence {
			t.Fatalf("non-increasing progress: previous=%d current=%d", lastSequence, progress.Sequence)
		}
		lastSequence = progress.Sequence
	}
	if bytes.Contains(stdout.Bytes(), []byte(home)) {
		t.Fatal("scan progress leaked a path")
	}
}

func TestScanCommandTextUsesSelectedScopeCompletionCopy(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"--state-dir", state, "--quiet", "scan", "--scope", "usage"}, bytes.NewReader(nil), &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 || stdout.String() != "Scan complete: usage.\n" {
		t.Fatalf("scan exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	waitForBackgroundScan(t, state)
}

func TestScanCommandRejectsUnsupportedScopeBeforeWorkerLaunch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"--state-dir", t.TempDir(), "scan", "--scope", "extension"}, bytes.NewReader(nil), &stdout, &stderr)
	if exit == 0 {
		t.Fatal("scan accepted unsupported scope")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("invalid scan scope")) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
