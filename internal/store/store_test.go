package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestV10MigrationCanonicalizesUsageEventAndSessionTimes(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, "agentdeck.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "agentdeck-v6.sql"))
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(fixture)); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES
('positive','codex','offsets','positive','2026-07-01T01:00:00+08:00','missing','fixture',1),
('negative','codex','offsets','negative','2026-06-30T20:00:00-05:00','missing','fixture',2);
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES('codex','offsets','2026-06-30T20:00:00-05:00','2026-07-01T01:00:00+08:00')`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	migrated, err := Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	var first, last string
	if err = migrated.DB.QueryRowContext(ctx, `SELECT first_at,last_at FROM usage_sessions WHERE client='codex' AND session_id='offsets'`).Scan(&first, &last); err != nil {
		t.Fatal(err)
	}
	if first != "2026-06-30T17:00:00Z" || last != "2026-07-01T01:00:00Z" {
		t.Fatalf("migrated session range = %q to %q", first, last)
	}
	var canonical int
	if err = migrated.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_events WHERE event_at IN ('2026-06-30T17:00:00Z','2026-07-01T01:00:00Z')`).Scan(&canonical); err != nil || canonical != 2 {
		t.Fatalf("canonical event times = %d, %v", canonical, err)
	}
}

func TestV13MigrationAddsSafeToolActivityStorage(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, "agentdeck.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "agentdeck-v6.sql"))
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(fixture)); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('fixture','identity',10,10,'hash')`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	migrated, err := Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	var parserVersion int
	var cumulativeJSON string
	if err = migrated.DB.QueryRowContext(ctx, `SELECT parser_version,codex_cumulative_json FROM usage_source_files WHERE path='fixture'`).Scan(&parserVersion, &cumulativeJSON); err != nil {
		t.Fatal(err)
	}
	if parserVersion != 0 || cumulativeJSON != "{}" {
		t.Fatalf("parser version = %d cumulative cursor = %q", parserVersion, cumulativeJSON)
	}
	if _, err = migrated.DB.ExecContext(ctx, `INSERT INTO usage_tool_calls(activity_key,client,session_id,model,tool_name,started_at,status,source_path,source_offset) VALUES('call','codex','session','gpt-5.4','exec_command','2026-07-20T00:00:00Z','started','fixture',1)`); err != nil {
		t.Fatal(err)
	}
	var toolName, status string
	if err = migrated.DB.QueryRowContext(ctx, `SELECT tool_name,status FROM usage_tool_calls WHERE activity_key='call'`).Scan(&toolName, &status); err != nil || toolName != "exec_command" || status != "started" {
		t.Fatalf("tool activity = %q %q, %v", toolName, status, err)
	}
	version, err := migrated.SchemaVersion(ctx)
	if err != nil || version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, %v", version, err)
	}
}

// The rebuildSessions query in internal/usage groups usage_events by
// (client, session_id) once per affected session per scanned file; without an
// index matching that pair, SQLite falls back to a full table scan that grows
// with the table's size on every scan. This confirms schema v14 adds the
// index and that the planner actually uses it.
func TestV14MigrationAddsUsageEventsClientSessionIndex(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	migrated, err := Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	version, err := migrated.SchemaVersion(ctx)
	if err != nil || version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, %v", version, err)
	}
	// Insert enough rows across enough distinct sessions that the planner's
	// choice of index is unambiguous — a two-row table could plausibly still
	// get scanned under some future SQLite cost-estimation heuristic even
	// with the index present, which would make this test fragile without
	// actually losing coverage of the index itself.
	insert := `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES(?,?,?,?,?,?,?,0)`
	for i := 0; i < 60; i++ {
		session := fmt.Sprintf("session-%02d", i)
		key := fmt.Sprintf("e%02d", i)
		if _, err = migrated.DB.ExecContext(ctx, insert, key, "codex", session, key, "2026-07-22T00:00:00Z", "gpt-5.4", "fixture"); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := migrated.DB.QueryContext(ctx, `EXPLAIN QUERY PLAN SELECT client,session_id,MIN(event_at),MAX(event_at) FROM usage_events WHERE client=? AND session_id=? GROUP BY client,session_id`, "codex", "session-00")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var usesIndex bool
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err = rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(detail, "SCAN usage_events") {
			t.Fatalf("rebuildSessions query plan still scans usage_events: %q", detail)
		}
		if strings.Contains(detail, "usage_events_client_session") {
			usesIndex = true
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !usesIndex {
		t.Fatal("rebuildSessions query plan does not report using usage_events_client_session")
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
