package session

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
	_ "modernc.org/sqlite"
)

func fileDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(contents)
}

func TestCheckHealthMissingIndexReportsNotRequestedWithoutCreatingState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	health, err := CheckHealth(context.Background(), root, false)
	if err != nil {
		t.Fatal(err)
	}
	if health.Present || health.FTSAvailable || health.Integrity != "not_requested" || health.UnreadableSources != 0 {
		t.Fatalf("health = %+v, want Present=false FTSAvailable=false Integrity=not_requested UnreadableSources=0", health)
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("state root exists after missing-index check: err=%v", err)
	}
}

func TestCheckHealthCompatibleIndexReportsOKWithoutMutatingTheDatabaseFile(t *testing.T) {
	root := t.TempDir()
	s, err := store.OpenSessions(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(root, "sessions.sqlite3")
	before, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeMode := before.Mode().Perm()
	beforeDigest := fileDigest(t, dbPath)

	health, err := CheckHealth(context.Background(), root, false)
	if err != nil {
		t.Fatal(err)
	}
	if !health.Present || !health.FTSAvailable || health.Integrity != "ok" || health.UnreadableSources != 0 {
		t.Fatalf("health = %+v, want Present=true FTSAvailable=true Integrity=ok UnreadableSources=0", health)
	}

	after, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.Mode().Perm(); got != beforeMode {
		t.Fatalf("%s mode changed: %#o, want %#o", dbPath, got, beforeMode)
	}
	if afterDigest := fileDigest(t, dbPath); afterDigest != beforeDigest {
		t.Fatal("non-full health check changed the session database contents")
	}

	// CheckHealth opens the index read-only, but SQLite still materializes
	// -shm/-wal sidecars for a WAL-mode database, and a read-only connection
	// cannot clean up its own sidecars on close. This pins the OBSERVED
	// behavior; doctor.go's "without creating" doc comment overstates the
	// contract. Handed off as a backlog item rather than fixed here, because
	// this queue is test-only. The sidecars are created 0600 inside the 0700
	// state root, so the privacy boundary that does hold is asserted too; the
	// main-database digest above proves the committed data is untouched.
	for _, suffix := range []string{"-wal", "-shm"} {
		info, err := os.Stat(dbPath + suffix)
		if err != nil {
			t.Fatalf("expected observed %s sidecar after CheckHealth: %v", suffix, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s sidecar mode = %#o, want 0600", suffix, got)
		}
	}
}

func TestCheckHealthWithoutSessionDocumentsTableReportsFTSUnavailable(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "sessions.sqlite3")
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(context.Background(), "CREATE TABLE session_sources (source_path TEXT PRIMARY KEY)"); err != nil {
		raw.Close()
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	health, err := CheckHealth(context.Background(), root, false)
	if err != nil {
		t.Fatal(err)
	}
	if !health.Present || health.FTSAvailable || health.Integrity != "not_requested" {
		t.Fatalf("health = %+v, want Present=true FTSAvailable=false Integrity=not_requested", health)
	}
}

func TestFullCheckHealthCountsUnreadableSourcesWithoutLeakingSourceDetails(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing.jsonl")
	denied := filepath.Join(t.TempDir(), "denied.jsonl")
	readable := filepath.Join(t.TempDir(), "readable.jsonl")
	if err := os.WriteFile(denied, []byte("synthetic denied source text"), 0000); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readable, []byte("synthetic readable source text"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Open(denied); err == nil {
		t.Skip("permission-denied fixture is not portable on this test runner (0000 file is still readable)")
	}
	s, err := store.OpenSessions(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for i, sourcePath := range []string{missing, denied, readable} {
		if _, err := s.DB.ExecContext(context.Background(), "INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,?,?,?,?,?,?,?)", sourcePath, "synthetic", 0, 0, 0, "", i, 1, "2026-01-01T00:00:00Z"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	health, err := CheckHealth(context.Background(), root, true)
	if err != nil {
		t.Fatal(err)
	}
	if !health.Present || !health.FTSAvailable || health.Integrity != "ok" || health.UnreadableSources != 2 {
		t.Fatalf("health = %+v, want Present=true FTSAvailable=true Integrity=ok UnreadableSources=2", health)
	}

	// The count is the only thing this diagnostic may reveal about sources.
	// Today Health carries only scalar fields, so leakage is structurally
	// impossible; this guard fails if a future field ever renders a source
	// path or basename, catching that regression before it can leak. It
	// reports which field leaked, never the leaked value, so the failure
	// message itself stays clean.
	value := reflect.ValueOf(health)
	for i := 0; i < value.NumField(); i++ {
		rendered := fmt.Sprintf("%v", value.Field(i).Interface())
		for _, source := range []string{missing, denied, readable} {
			if strings.Contains(rendered, filepath.Dir(source)) || strings.Contains(rendered, filepath.Base(source)) {
				t.Fatalf("Health field %q leaks source path detail", value.Type().Field(i).Name)
			}
		}
	}
}
