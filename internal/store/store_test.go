package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	_ "modernc.org/sqlite"
)

func TestDriverFoundation(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "state")
	s, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.Exec(ctx, "CREATE VIRTUAL TABLE session_search USING fts5(text)"); err != nil {
		t.Fatalf("FTS5 unavailable: %v", err)
	}
	if _, err := s.Exec(ctx, "INSERT INTO session_search(text) VALUES ('approved visible response')"); err != nil {
		t.Fatal(err)
	}
	var matches int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM session_search WHERE session_search MATCH 'visible'").Scan(&matches); err != nil {
		t.Fatal(err)
	}
	if matches != 1 {
		t.Fatalf("FTS5 matches = %d, want 1", matches)
	}

	var journalMode string
	if err := s.DB.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	destination := filepath.Join(root, "snapshot.sqlite3")
	if err := s.Backup(ctx, destination); err != nil {
		t.Fatalf("online backup: %v", err)
	}
	backupDB, err := sql.Open("sqlite", destination)
	if err != nil {
		t.Fatal(err)
	}
	defer backupDB.Close()
	if err := backupDB.QueryRowContext(ctx, "SELECT count(*) FROM session_search WHERE session_search MATCH 'response'").Scan(&matches); err != nil {
		t.Fatalf("backup contents: %v", err)
	}
	if matches != 1 {
		t.Fatalf("backup FTS5 matches = %d, want 1", matches)
	}

	assertMode(t, root, platform.DirectoryMode)
	for _, path := range []string{filepath.Join(root, "agentdeck.sqlite3"), filepath.Join(root, "agentdeck.sqlite3-wal"), filepath.Join(root, "agentdeck.sqlite3-shm"), destination} {
		assertMode(t, path, platform.FileMode)
	}
}

func TestPreparePrivateSQLiteFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.sqlite3")
	if err := preparePrivateSQLiteFiles(path); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{path, path + "-wal", path + "-shm", path + "-journal"} {
		assertMode(t, candidate, platform.FileMode)
	}
}

func TestAdoptExtensionReturnsMatchingFingerprint(t *testing.T) {
	ctx := context.Background()
	state, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	value := Extension{
		ID:           "codex:skill:user:sample",
		Client:       "codex",
		Kind:         "skill",
		Scope:        "user",
		NativeID:     "sample",
		SourcePath:   "/synthetic/sample",
		Version:      "unknown",
		Enabled:      "unknown",
		Capabilities: []string{"read_only"},
		Diagnostics:  []string{},
		Fingerprint:  "synthetic-fingerprint",
	}
	if err = state.ReplaceExtensions(ctx, []Extension{value}); err != nil {
		t.Fatal(err)
	}
	adopted, err := state.AdoptExtension(ctx, value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !adopted.Managed || adopted.AdoptedFingerprint != adopted.Fingerprint {
		t.Fatalf("AdoptExtension = %#v", adopted)
	}
}

func TestReplaceExtensionsDoesNotRefreshUnchangedInventory(t *testing.T) {
	ctx := context.Background()
	state, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	value := Extension{ID: "codex:skill:user:sample", Client: "codex", Kind: "skill", Scope: "user", NativeID: "sample", SourcePath: "/synthetic/sample", Version: "unknown", Enabled: "unknown", Capabilities: []string{"read_only"}, Diagnostics: []string{}, Fingerprint: "stable"}
	if err = state.ReplaceExtensions(ctx, []Extension{value}); err != nil {
		t.Fatal(err)
	}
	var before string
	if err = state.DB.QueryRowContext(ctx, "SELECT updated_at FROM extensions WHERE id=?", value.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if err = state.ReplaceExtensions(ctx, []Extension{value}); err != nil {
		t.Fatal(err)
	}
	var after string
	if err = state.DB.QueryRowContext(ctx, "SELECT updated_at FROM extensions WHERE id=?", value.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("unchanged extension updated_at changed: %q -> %q", before, after)
	}
}

func TestOpenSessionsAddsSourceCursorColumnsToExistingIndex(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(root, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "sessions.sqlite3")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec("CREATE TABLE session_sources (path TEXT PRIMARY KEY, content_hash TEXT NOT NULL, parser_version INTEGER NOT NULL, scanned_at TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, column := range []string{"identity", "cursor", "modified_at", "partial_line"} {
		var count int
		if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('session_sources') WHERE name = ?", column).Scan(&count); err != nil || count != 1 {
			t.Fatalf("column %q count=%d err=%v", column, count, err)
		}
	}
}

func TestMigrationsRejectUnknownNewerSchema(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "CREATE TABLE schema_metadata (version INTEGER NOT NULL); INSERT INTO schema_metadata VALUES (?)", CurrentSchemaVersion+1); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migrations); !errors.Is(err, ErrUnknownSchema) {
		t.Fatalf("migrate error = %v, want unknown schema", err)
	}
}

func TestMigrationsRejectExistingDatabaseWithoutMetadata(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "CREATE TABLE unrecognized (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migrations); !errors.Is(err, ErrUnknownSchema) {
		t.Fatalf("migrate error = %v, want unknown schema", err)
	}
}

func TestMigrationFailurePreservesLastUsableSchema(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "CREATE TABLE schema_metadata (singleton INTEGER PRIMARY KEY CHECK(singleton = 1), version INTEGER NOT NULL); INSERT INTO schema_metadata VALUES (1, 1); CREATE TABLE preserved (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	broken := []migration{{version: 2, statements: []string{"CREATE TABLE should_not_exist (", "CREATE TABLE never_reached (id INTEGER)"}}}
	if err := migrate(ctx, db, broken); err == nil {
		t.Fatal("migrate succeeded with broken migration")
	}
	version, err := schemaVersion(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("schema version = %d, want 1", version)
	}
	for _, table := range []string{"preserved", "should_not_exist", "never_reached"} {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		want := 0
		if table == "preserved" {
			want = 1
		}
		if count != want {
			t.Fatalf("table %s count = %d, want %d", table, count, want)
		}
	}
}

func TestBootstrapMigrationFailureLeavesNoPartialSchema(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	broken := []migration{{version: 1, statements: []string{"CREATE TABLE incomplete ("}}}
	if err := migrate(ctx, db, broken); err == nil {
		t.Fatal("migrate succeeded with broken bootstrap migration")
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("bootstrap left %d tables, want 0", count)
	}
}

func TestSchemaVersionRejectsMultipleRows(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "CREATE TABLE schema_metadata (version INTEGER NOT NULL); INSERT INTO schema_metadata VALUES (1), (1)"); err != nil {
		t.Fatal(err)
	}
	if _, err := schemaVersion(ctx, db); !errors.Is(err, ErrUnknownSchema) {
		t.Fatalf("schemaVersion error = %v, want unknown schema", err)
	}
}

func TestLockIsExclusiveAndPrivate(t *testing.T) {
	root := t.TempDir()
	first, err := AcquireLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertMode(t, filepath.Join(root, "state.lock"), platform.FileMode)
	if _, err := AcquireLock(context.Background(), root, 0); !errors.Is(err, ErrStateBusy) {
		t.Fatalf("second lock error = %v, want state_busy", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestLockReleaseDoesNotDeleteNewOwner(t *testing.T) {
	root := t.TempDir()
	first, err := AcquireLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "state.lock")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	second, err := AcquireLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	if err := first.Release(); !errors.Is(err, ErrLockLost) {
		t.Fatalf("first release error = %v, want lock_lost", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("new owner's lock was removed: %v", err)
	}
}

func TestOpenRespectsMigrationLock(t *testing.T) {
	root := t.TempDir()
	if err := platform.EnsureStateRoot(root); err != nil {
		t.Fatal(err)
	}
	lock, err := AcquireLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	previousWait := lockWait
	lockWait = 0
	defer func() { lockWait = previousWait }()
	if _, err := Open(context.Background(), root); !errors.Is(err, ErrStateBusy) {
		t.Fatalf("Open error = %v, want state_busy", err)
	}
}

func TestOpenReturnsLockReleaseFailure(t *testing.T) {
	root := t.TempDir()
	_, err := open(context.Background(), root, func(context.Context, string, time.Duration) (stateLock, error) {
		return failingLock{err: ErrLockLost}, nil
	})
	if !errors.Is(err, ErrLockLost) {
		t.Fatalf("Open error = %v, want lock_lost", err)
	}
}

type failingLock struct{ err error }

func (l failingLock) Release() error { return l.err }

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want.Perm() {
		t.Fatalf("%s mode = %#o, want %#o", path, got, want)
	}
}
