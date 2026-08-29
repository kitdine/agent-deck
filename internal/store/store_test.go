package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
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

func TestOpenReadOnlyCompatibleDBSkipsMigrationAndDoesNotCreateWriteSidecars(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(state, "agentdeck.sqlite3"))
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
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(state, "agentdeck.sqlite3")
	if err = os.Chmod(dbPath, 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	originalMode := before.Mode().Perm()

	store, err := OpenReadOnly(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	version, err := store.SchemaVersion(ctx)
	if err != nil || version != 6 {
		t.Fatalf("schema version = %d, %v", version, err)
	}
	var hasToolCalls int
	if err = store.DB.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='usage_tool_calls'").Scan(&hasToolCalls); err != nil {
		t.Fatal(err)
	}
	if hasToolCalls != 0 {
		t.Fatalf("usage_tool_calls table = %d, want 0 (no migration expected)", hasToolCalls)
	}
	for _, path := range []string{
		filepath.Join(state, "agentdeck.sqlite3-wal"),
		filepath.Join(state, "agentdeck.sqlite3-shm"),
		filepath.Join(state, "agentdeck.sqlite3-journal"),
	} {
		assertNotExist(t, path)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.Mode().Perm(); got != originalMode {
		t.Fatalf("%s mode changed: %#o, want %#o", dbPath, got, originalMode)
	}
}

func TestOpenReadOnlyDoesNotCreateMissingDatabase(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenReadOnly(ctx, state); err == nil {
		t.Fatal("OpenReadOnly unexpectedly succeeded")
	}
	for _, path := range []string{
		filepath.Join(state, "agentdeck.sqlite3"),
		filepath.Join(state, "agentdeck.sqlite3-wal"),
		filepath.Join(state, "agentdeck.sqlite3-shm"),
		filepath.Join(state, "agentdeck.sqlite3-journal"),
	} {
		assertNotExist(t, path)
	}
}

func TestOpenSessionsReadOnlyDoesNotCreateMissingDatabase(t *testing.T) {
	root := t.TempDir()
	if _, err := OpenSessionsReadOnly(context.Background(), root); err == nil {
		t.Fatal("OpenSessionsReadOnly unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(root, "sessions.sqlite3")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sessions.sqlite3 after read-only open: %v", err)
	}
}

func TestOpenSessionsReadOnlyReadsExistingIndex(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writable, err := OpenSessions(ctx, root)
	if err != nil {
		t.Fatalf("OpenSessions: %v", err)
	}
	defer writable.Close()

	readOnly, err := OpenSessionsReadOnly(ctx, root)
	if err != nil {
		t.Fatalf("OpenSessionsReadOnly: %v", err)
	}
	defer readOnly.Close()
	var sources int
	if err = readOnly.DB.QueryRowContext(ctx, "SELECT count(*) FROM session_sources").Scan(&sources); err != nil {
		t.Fatalf("read live WAL session index: %v", err)
	}
	if _, err = readOnly.DB.ExecContext(ctx, "DELETE FROM session_metadata"); err == nil {
		t.Fatal("read-only session index accepted a write")
	}
}

func TestOpenReadOnlyRejectsFutureSchema(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(state, "agentdeck.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE schema_metadata (singleton INTEGER PRIMARY KEY CHECK(singleton = 1), version INTEGER NOT NULL CHECK(version >= 0)); INSERT INTO schema_metadata(singleton, version) VALUES (1, ?)", CurrentSchemaVersion+1); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenReadOnly(ctx, state); !errors.Is(err, ErrUnknownSchema) {
		t.Fatalf("OpenReadOnly error = %v, want unknown_schema", err)
	}
	for _, path := range []string{
		filepath.Join(state, "agentdeck.sqlite3-wal"),
		filepath.Join(state, "agentdeck.sqlite3-shm"),
		filepath.Join(state, "agentdeck.sqlite3-journal"),
	} {
		assertNotExist(t, path)
	}
}

func TestBackupSQLiteFileCopiesWALCommittedDataAndCanReopen(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(state, "source.sqlite3")
	sourceDB, err := sql.Open("sqlite", source+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer sourceDB.Close()
	if _, err = sourceDB.ExecContext(ctx, "CREATE TABLE records(id INTEGER PRIMARY KEY, value TEXT);"); err != nil {
		t.Fatal(err)
	}
	if _, err = sourceDB.ExecContext(ctx, "INSERT INTO records(value) VALUES ('value')"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(source + "-wal"); err != nil {
		t.Fatalf("WAL file: %v", err)
	}
	if err = BackupSQLiteFile(ctx, source, filepath.Join(state, "snapshot.sqlite3")); err != nil {
		t.Fatal(err)
	}

	snapshot, err := sql.Open("sqlite", filepath.Join(state, "snapshot.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	var count int
	if err = snapshot.QueryRowContext(ctx, "SELECT count(*) FROM records").Scan(&count); err != nil {
		t.Fatalf("snapshot query: %v", err)
	}
	if count != 1 {
		t.Fatalf("snapshot count = %d, want 1", count)
	}
	assertMode(t, filepath.Join(state, "snapshot.sqlite3"), platform.FileMode)
}

func TestIntegrityCheckForHealthyAndCorruptDatabases(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Exec(ctx, "CREATE TABLE checks(id INTEGER PRIMARY KEY, value TEXT);"); err != nil {
		t.Fatal(err)
	}
	if result, err := store.IntegrityCheck(ctx); err != nil || result != "ok" {
		t.Fatalf("IntegrityCheck healthy = %q, %v", result, err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(state, "agentdeck.sqlite3")
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) <= 20 {
		t.Fatal("database file too small to corrupt")
	}
	corruptState := filepath.Join(t.TempDir(), "corrupt")
	if err := os.MkdirAll(corruptState, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	corruptPath := filepath.Join(corruptState, "agentdeck.sqlite3")
	corruptRaw := append([]byte(nil), raw...)
	corruptRaw[20] ^= 0xff
	if err := os.WriteFile(corruptPath, corruptRaw, platform.FileMode); err != nil {
		t.Fatal(err)
	}
	corrupt, openErr := OpenReadOnly(ctx, corruptState)
	if openErr != nil {
		t.Fatalf("OpenReadOnly on corrupted DB: %v", openErr)
	}
	result, checkErr := corrupt.IntegrityCheck(ctx)
	if closeErr := corrupt.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	if checkErr != nil {
		t.Fatalf("IntegrityCheck unexpectedly failed: %v", checkErr)
	}
	if result == "ok" {
		t.Fatalf("IntegrityCheck result on corrupted database = %q", result)
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

func TestOpenSessionsRebuildsDocumentsWithoutEventAt(t *testing.T) {
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
	if _, err := legacy.Exec(`CREATE VIRTUAL TABLE session_documents USING fts5(source_path UNINDEXED, client UNINDEXED, session_id UNINDEXED, kind UNINDEXED, text)`); err != nil {
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
	var columns int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('session_documents') WHERE name='event_at'").Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 1 {
		t.Fatalf("event_at columns = %d, want 1", columns)
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

// wrapper-schema: an existing pre-v15 database must open with every provider
// reading back with no wrapper and unchanged snapshot behavior, and the new
// provider/official wrapper storage must never create a credential,
// ciphertext row, or the vault key file.
func TestV15MigrationAddsProviderWrapperURLAndSelectionRouteWithoutSideEffects(t *testing.T) {
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
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	migrated, err := Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	if version, err := migrated.SchemaVersion(ctx); err != nil || version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, %v", version, err)
	}

	providers, err := migrated.ListProviders(ctx)
	if err != nil || len(providers) != 1 || providers[0].Name != "legacy" || providers[0].WrapperURL != "" {
		t.Fatalf("providers = %#v, %v", providers, err)
	}
	// The migration must add via_wrapper without disturbing any other
	// snapshot field already covered by the v6 fixture's completed selection.
	snapshot, err := migrated.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || snapshot.ViaWrapper || snapshot.Name != "legacy" || snapshot.Endpoint != "https://legacy.example" || snapshot.Multiplier != "1.5" || snapshot.Credential != "default" {
		t.Fatalf("pre-existing selection route = %#v, %v", snapshot, err)
	}

	// SetProviderWrapper is pure storage; normalization is a service-layer
	// concern (provider.NormalizeWrapperURL), so the stored value round-trips
	// exactly as given.
	updated, err := migrated.SetProviderWrapper(ctx, "legacy", "https://proxy.example", "")
	if err != nil || updated.WrapperURL != "https://proxy.example" {
		t.Fatalf("SetProviderWrapper = %#v, %v", updated, err)
	}
	reread, err := migrated.ProviderByName(ctx, "legacy")
	if err != nil || reread.WrapperURL != "https://proxy.example" {
		t.Fatalf("reread wrapper = %#v, %v", reread, err)
	}
	cleared, err := migrated.SetProviderWrapper(ctx, "legacy", "", "")
	if err != nil || cleared.WrapperURL != "" {
		t.Fatalf("cleared wrapper = %#v, %v", cleared, err)
	}
	if _, err = migrated.SetProviderWrapper(ctx, "does-not-exist", "https://proxy.example", ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetProviderWrapper on unknown provider error = %v, want sql.ErrNoRows", err)
	}

	if url, err := migrated.OfficialWrapperURL(ctx); err != nil || url != "" {
		t.Fatalf("official wrapper before set = %q, %v", url, err)
	}
	if err = migrated.SetOfficialWrapperURL(ctx, "https://proxy.example", ""); err != nil {
		t.Fatal(err)
	}
	if url, err := migrated.OfficialWrapperURL(ctx); err != nil || url != "https://proxy.example" {
		t.Fatalf("official wrapper after set = %q, %v", url, err)
	}
	if err = migrated.SetOfficialWrapperURL(ctx, "", ""); err != nil {
		t.Fatal(err)
	}
	if url, err := migrated.OfficialWrapperURL(ctx); err != nil || url != "" {
		t.Fatalf("official wrapper after clear = %q, %v", url, err)
	}

	var credentialCount, secretCount int
	if err = migrated.DB.QueryRowContext(ctx, `SELECT count(*) FROM provider_credentials`).Scan(&credentialCount); err != nil {
		t.Fatal(err)
	}
	if err = migrated.DB.QueryRowContext(ctx, `SELECT count(*) FROM credential_secrets`).Scan(&secretCount); err != nil {
		t.Fatal(err)
	}
	if credentialCount != 1 || secretCount != 0 {
		t.Fatalf("credential rows = %d, secret rows = %d, want unchanged 1 and 0", credentialCount, secretCount)
	}
	if _, err = os.Stat(filepath.Join(state, "credential.key")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("credential.key stat error = %v, want not-exist", err)
	}
}

func TestV19MigrationAddsHookDeliveryObservationLedger(t *testing.T) {
	ctx := context.Background()
	type columnSpec struct {
		columnType   string
		notNull      bool
		hasDefault   bool
		defaultValue string
		primaryKey   bool
	}
	// Mirrors the exact Stream 1 DDL in architecture.md, not just column
	// name/type: notnull, default, and primary-key must all match, or a
	// weakened constraint (e.g. a nullable route_effect) would still pass.
	observationColumns := map[string]columnSpec{
		"id":                   {"INTEGER", false, false, "", true},
		"client":               {"TEXT", true, false, "", false},
		"session_id":           {"TEXT", true, false, "", false},
		"observed_at":          {"TEXT", true, false, "", false},
		"hook_event":           {"TEXT", true, false, "", false},
		"source":               {"TEXT", true, true, "''", false},
		"config_matched":       {"INTEGER", false, false, "", false},
		"observed_provider":    {"TEXT", false, false, "", false},
		"observed_multiplier":  {"TEXT", false, false, "", false},
		"observed_via_wrapper": {"INTEGER", false, false, "", false},
		"prior_state":          {"TEXT", false, false, "", false},
		"conflict_scan":        {"TEXT", false, false, "", false},
		"conflict_sources":     {"TEXT", true, true, "''", false},
		"route_effect":         {"TEXT", true, false, "", false},
		"settings_changed_at":  {"TEXT", true, true, "''", false},
		"delivery_id":          {"TEXT", true, false, "", false},
	}
	type indexSpec struct {
		unique  bool
		columns []string
	}
	observationIndexes := map[string]indexSpec{
		"usage_session_observations_lookup":   {unique: false, columns: []string{"client", "session_id", "observed_at"}},
		"usage_session_observations_delivery": {unique: true, columns: []string{"delivery_id"}},
	}
	assertObservationShape := func(t *testing.T, db *sql.DB) {
		t.Helper()
		rows, err := db.QueryContext(ctx, `SELECT name,type,"notnull",dflt_value,pk FROM pragma_table_info('usage_session_observations')`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		got := map[string]columnSpec{}
		for rows.Next() {
			var name, columnType string
			var notNull, pk int
			var dfltValue sql.NullString
			if err := rows.Scan(&name, &columnType, &notNull, &dfltValue, &pk); err != nil {
				t.Fatal(err)
			}
			got[name] = columnSpec{columnType: columnType, notNull: notNull != 0, hasDefault: dfltValue.Valid, defaultValue: dfltValue.String, primaryKey: pk != 0}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if len(got) != len(observationColumns) {
			t.Fatalf("usage_session_observations columns = %#v, want %#v", got, observationColumns)
		}
		for name, want := range observationColumns {
			if got[name] != want {
				t.Fatalf("usage_session_observations.%s = %#v, want %#v", name, got[name], want)
			}
		}

		indexRows, err := db.QueryContext(ctx, `SELECT name,"unique" FROM pragma_index_list('usage_session_observations')`)
		if err != nil {
			t.Fatal(err)
		}
		defer indexRows.Close()
		foundIndexes := map[string]bool{}
		for indexRows.Next() {
			var name string
			var unique int
			if err := indexRows.Scan(&name, &unique); err != nil {
				t.Fatal(err)
			}
			want, ok := observationIndexes[name]
			if !ok {
				continue
			}
			foundIndexes[name] = true
			if (unique != 0) != want.unique {
				t.Fatalf("index %s unique = %v, want %v", name, unique != 0, want.unique)
			}
			columnRows, err := db.QueryContext(ctx, `SELECT name FROM pragma_index_info(?) ORDER BY seqno`, name)
			if err != nil {
				t.Fatal(err)
			}
			var columns []string
			for columnRows.Next() {
				var column string
				if err := columnRows.Scan(&column); err != nil {
					columnRows.Close()
					t.Fatal(err)
				}
				columns = append(columns, column)
			}
			if err := columnRows.Close(); err != nil {
				t.Fatal(err)
			}
			if len(columns) != len(want.columns) {
				t.Fatalf("index %s columns = %#v, want %#v", name, columns, want.columns)
			}
			for i, column := range want.columns {
				if columns[i] != column {
					t.Fatalf("index %s columns = %#v, want %#v", name, columns, want.columns)
				}
			}
		}
		if err := indexRows.Err(); err != nil {
			t.Fatal(err)
		}
		if len(foundIndexes) != len(observationIndexes) {
			t.Fatalf("usage_session_observations indexes found = %#v, want %#v", foundIndexes, observationIndexes)
		}
	}
	readRow := func(t *testing.T, db *sql.DB, query string, args ...any) map[string]any {
		t.Helper()
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		columns, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		if !rows.Next() {
			t.Fatalf("no row for query %q", query)
		}
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		result := map[string]any{}
		for i, column := range columns {
			if raw, ok := values[i].([]byte); ok {
				result[column] = string(raw)
			} else {
				result[column] = values[i]
			}
		}
		return result
	}

	t.Run("fresh database", func(t *testing.T) {
		db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.sqlite3"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if err := migrate(ctx, db, migrations[:19]); err != nil {
			t.Fatal(err)
		}
		if version, err := schemaVersion(ctx, db); err != nil || version != 19 {
			t.Fatalf("schema version = %d, %v, want 19", version, err)
		}
		assertObservationShape(t, db)
	})

	t.Run("upgrade from version 18", func(t *testing.T) {
		db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.sqlite3"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if err := migrate(ctx, db, migrations[:18]); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
			VALUES(1,'official','x','','1','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z');
			INSERT INTO usage_session_routes(client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,source,quality,semantic_key)
			VALUES('codex','session','2026-08-04T00:00:00Z','official','1',0,'SessionStart','resume','estimated','key-1');
			INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset)
			VALUES('event-1','codex','session','evt','2026-08-04T00:00:00Z','fixture','path',0);
		`); err != nil {
			t.Fatal(err)
		}
		beforeRoutes, err := db.QueryContext(ctx, `SELECT id,client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,source,quality,semantic_key FROM usage_session_routes ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		type routeRow struct {
			id                   int64
			client, sessionID    string
			observedAt           string
			provider, multiplier string
			viaWrapper           int
			hookEvent, source    string
			quality, semanticKey string
		}
		var wantRoutes []routeRow
		for beforeRoutes.Next() {
			var row routeRow
			if err := beforeRoutes.Scan(&row.id, &row.client, &row.sessionID, &row.observedAt, &row.provider, &row.multiplier, &row.viaWrapper, &row.hookEvent, &row.source, &row.quality, &row.semanticKey); err != nil {
				t.Fatal(err)
			}
			wantRoutes = append(wantRoutes, row)
		}
		if err := beforeRoutes.Close(); err != nil {
			t.Fatal(err)
		}
		wantEvent := readRow(t, db, `SELECT * FROM usage_events WHERE event_key='event-1'`)

		if err := migrate(ctx, db, migrations[18:19]); err != nil {
			t.Fatal(err)
		}
		if version, err := schemaVersion(ctx, db); err != nil || version != 19 {
			t.Fatalf("schema version = %d, %v, want 19", version, err)
		}
		assertObservationShape(t, db)

		routeColumns := map[string]string{}
		routeRows, err := db.QueryContext(ctx, `SELECT name,type FROM pragma_table_info('usage_session_routes')`)
		if err != nil {
			t.Fatal(err)
		}
		for routeRows.Next() {
			var name, columnType string
			if err := routeRows.Scan(&name, &columnType); err != nil {
				t.Fatal(err)
			}
			routeColumns[name] = columnType
		}
		if err := routeRows.Close(); err != nil {
			t.Fatal(err)
		}
		wantRouteColumns := map[string]string{"id": "INTEGER", "client": "TEXT", "session_id": "TEXT", "observed_at": "TEXT", "provider": "TEXT", "multiplier": "TEXT", "via_wrapper": "INTEGER", "hook_event": "TEXT", "source": "TEXT", "quality": "TEXT", "semantic_key": "TEXT"}
		if len(routeColumns) != len(wantRouteColumns) {
			t.Fatalf("usage_session_routes columns = %#v, want %#v", routeColumns, wantRouteColumns)
		}
		for name, columnType := range wantRouteColumns {
			if routeColumns[name] != columnType {
				t.Fatalf("usage_session_routes.%s type = %q, want %q", name, routeColumns[name], columnType)
			}
		}

		afterRoutes, err := db.QueryContext(ctx, `SELECT id,client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,source,quality,semantic_key FROM usage_session_routes ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		var gotRoutes []routeRow
		for afterRoutes.Next() {
			var row routeRow
			if err := afterRoutes.Scan(&row.id, &row.client, &row.sessionID, &row.observedAt, &row.provider, &row.multiplier, &row.viaWrapper, &row.hookEvent, &row.source, &row.quality, &row.semanticKey); err != nil {
				t.Fatal(err)
			}
			gotRoutes = append(gotRoutes, row)
		}
		if err := afterRoutes.Close(); err != nil {
			t.Fatal(err)
		}
		if len(gotRoutes) != len(wantRoutes) || len(gotRoutes) != 1 || gotRoutes[0] != wantRoutes[0] {
			t.Fatalf("usage_session_routes rows = %#v, want %#v", gotRoutes, wantRoutes)
		}

		gotEvent := readRow(t, db, `SELECT * FROM usage_events WHERE event_key='event-1'`)
		if !reflect.DeepEqual(gotEvent, wantEvent) {
			t.Fatalf("usage_events row = %#v, want %#v (byte-identical to the pre-migration row)", gotEvent, wantEvent)
		}

		var observationCount int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM usage_session_observations`).Scan(&observationCount); err != nil {
			t.Fatal(err)
		}
		if observationCount != 0 {
			t.Fatalf("usage_session_observations count = %d, want 0 after a purely additive migration", observationCount)
		}
	})
}

func TestV20AndV21MigrationsAddWorkSignalStorage(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(ctx, db, migrations[:19]); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `
		INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset)
		VALUES('event','codex','session','turn','2026-08-27T00:00:00Z','gpt','source',1);
		INSERT INTO usage_tool_calls(activity_key,client,session_id,model,tool_name,started_at,status,source_path,source_offset)
		VALUES('call','codex','session','gpt','apply_patch','2026-08-27T00:00:01Z','started','source',2);
	`); err != nil {
		t.Fatal(err)
	}
	if err = migrate(ctx, db, migrations[19:]); err != nil {
		t.Fatal(err)
	}
	if version, versionErr := schemaVersion(ctx, db); versionErr != nil || version != CurrentSchemaVersion || CurrentSchemaVersion < 20 {
		t.Fatalf("schema version = %d, %v, current=%d", version, versionErr, CurrentSchemaVersion)
	}
	var eventTurn, callTurn sql.NullInt64
	var toolKind string
	var mcpServer sql.NullString
	var commandRead int
	if err = db.QueryRowContext(ctx, `SELECT turn_index FROM usage_events WHERE event_key='event'`).Scan(&eventTurn); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT turn_index,tool_kind,mcp_server,command_read FROM usage_tool_calls WHERE activity_key='call'`).Scan(&callTurn, &toolKind, &mcpServer, &commandRead); err != nil {
		t.Fatal(err)
	}
	if eventTurn.Valid || callTurn.Valid || toolKind != "other" || mcpServer.Valid || commandRead != 0 {
		t.Fatalf("migrated defaults event_turn=%v call_turn=%v kind=%q mcp=%v command_read=%d", eventTurn, callTurn, toolKind, mcpServer, commandRead)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO usage_tool_files(activity_key,path_digest,base_name,wrote) VALUES('call','digest','main.go',1); INSERT INTO usage_work_signals(client,session_id,turn_index,started_at,state,message_class,intent_sub,activity_kind,activity_sub,source_path) VALUES('codex','session',1,'2026-08-27T00:00:00Z','classified','build','feature','coding','feature','/tmp/source.jsonl')`); err != nil {
		t.Fatal(err)
	}
	var fileRows, signalRows, digestIndexes int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM usage_tool_files`).Scan(&fileRows); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM usage_work_signals`).Scan(&signalRows); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM pragma_index_list('usage_tool_files') WHERE name='usage_tool_files_digest'`).Scan(&digestIndexes); err != nil {
		t.Fatal(err)
	}
	if fileRows != 1 || signalRows != 1 || digestIndexes != 1 {
		t.Fatalf("file_rows=%d signal_rows=%d digest_indexes=%d", fileRows, signalRows, digestIndexes)
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
	if _, err := db.ExecContext(ctx, "CREATE TABLE schema_metadata (singleton INTEGER PRIMARY KEY CHECK(singleton = 1), version INTEGER NOT NULL); INSERT INTO schema_metadata VALUES (1, 1); CREATE TABLE preserved (id INTEGER PRIMARY KEY, value TEXT NOT NULL); INSERT INTO preserved VALUES (1, 'committed')"); err != nil {
		t.Fatal(err)
	}
	broken := []migration{
		{version: 2, statements: []string{
			"CREATE TABLE committed_v2 (id INTEGER PRIMARY KEY, value TEXT NOT NULL)",
			"INSERT INTO committed_v2 VALUES (1, 'migration-2')",
		}},
		{version: 3, statements: []string{
			"CREATE TABLE should_not_exist (id INTEGER PRIMARY KEY)",
			"CREATE TABLE invalid (",
		}},
	}
	if err := migrate(ctx, db, broken); err == nil {
		t.Fatal("migrate succeeded with broken migration")
	}
	version, err := schemaVersion(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("schema version = %d, want 2", version)
	}
	for _, table := range []string{"preserved", "committed_v2", "should_not_exist"} {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		want := 0
		if table == "preserved" || table == "committed_v2" {
			want = 1
		}
		if count != want {
			t.Fatalf("table %s count = %d, want %d", table, count, want)
		}
	}
	var value string
	if err := db.QueryRowContext(ctx, "SELECT value FROM preserved WHERE id = 1").Scan(&value); err != nil || value != "committed" {
		t.Fatalf("preserved committed value = %q, %v", value, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT value FROM committed_v2 WHERE id = 1").Scan(&value); err != nil || value != "migration-2" {
		t.Fatalf("committed v2 value = %q, %v", value, err)
	}
}

func TestBootstrapMigrationFailureLeavesNoPartialSchema(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	broken := []migration{
		{version: 1, statements: []string{"CREATE TABLE first_bootstrap_table (id INTEGER PRIMARY KEY)"}},
		{version: 2, statements: []string{"CREATE TABLE second_bootstrap_table (id INTEGER PRIMARY KEY)", "CREATE TABLE invalid ("}},
	}
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

func TestMigrationApplyFailureRollsBackDDLAndPreservesCommittedSchema(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "CREATE TABLE schema_metadata (singleton INTEGER PRIMARY KEY CHECK(singleton = 1), version INTEGER NOT NULL); INSERT INTO schema_metadata VALUES (1, 1); CREATE TABLE preserved (id INTEGER PRIMARY KEY, value TEXT NOT NULL); INSERT INTO preserved VALUES (1, 'committed')"); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("apply failed")
	broken := []migration{{
		version:    2,
		statements: []string{"CREATE TABLE apply_rolled_back (id INTEGER PRIMARY KEY)"},
		apply: func(context.Context, *sql.Tx) error {
			return sentinel
		},
	}}
	if err := migrate(ctx, db, broken); !errors.Is(err, sentinel) {
		t.Fatalf("migrate error = %v, want apply sentinel", err)
	}
	version, err := schemaVersion(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("schema version = %d, want 1", version)
	}
	var tableCount int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'apply_rolled_back'").Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != 0 {
		t.Fatalf("apply failure left %d apply_rolled_back tables, want 0", tableCount)
	}
	var value string
	if err := db.QueryRowContext(ctx, "SELECT value FROM preserved WHERE id = 1").Scan(&value); err != nil || value != "committed" {
		t.Fatalf("preserved committed value = %q, %v", value, err)
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

func TestAcquireScanLockIsIndependentFromStateLock(t *testing.T) {
	root := t.TempDir()
	stateLock, err := AcquireLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer stateLock.Release()
	scanLock, err := AcquireScanLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := scanLock.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireScanLockBusyAndContextHandling(t *testing.T) {
	root := t.TempDir()
	owner, err := AcquireScanLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireScanLock(context.Background(), root, 0); !errors.Is(err, ErrStateBusy) {
		t.Fatalf("AcquireScanLock error = %v, want state_busy", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := AcquireScanLock(ctx, root, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("AcquireScanLock context cancel error = %v, want context.Canceled", err)
	}
	deadlineCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := AcquireScanLock(deadlineCtx, root, time.Second); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("AcquireScanLock context deadline error = %v, want context.DeadlineExceeded", err)
	}
	if err := owner.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireScanLockReleaseOwnsOnlyCurrentLock(t *testing.T) {
	root := t.TempDir()
	first, err := AcquireScanLock(context.Background(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "scan.lock")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	second, err := AcquireScanLock(context.Background(), root, 0)
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

func assertNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s to not exist", path)
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}
