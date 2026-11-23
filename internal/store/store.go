// Package store owns AgentDeck's SQLite lifecycle and schema boundaries.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kitdine/agent-deck/internal/hookrefusal"
	"github.com/kitdine/agent-deck/internal/platform"
	"golang.org/x/sys/unix"
	"modernc.org/sqlite"
)

const (
	CurrentSchemaVersion   = 36
	CodeProviderNotFound   = "provider_not_found"
	CodeCredentialNotFound = "credential_not_found"
)

// writablePragmas also keeps SQLite statement journals in memory. A file-backed
// statement journal buffers its first 64 KiB as one allocation, which the
// pure-Go allocator maps and unmaps again for every multi-row write statement.
const writablePragmas = "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=temp_store(2)"

// OpenSessions opens the separately purgeable session-search database. It is
// deliberately not part of the core schema so deleting the index can never
// delete providers, credentials, or usage data.
func OpenSessions(ctx context.Context, stateRoot string) (*Store, error) {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return nil, err
	}
	if err := platform.EnsureStateRoot(stateRoot); err != nil {
		return nil, err
	}
	path := filepath.Join(stateRoot, "sessions.sqlite3")
	if err := preparePrivateSQLiteFiles(path); err != nil {
		return nil, err
	}
	db, err := openWritableSQL(path+"?"+writablePragmas, "session")
	if err != nil {
		return nil, err
	}
	if _, err = db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	rebuilt, err := migrateSessionSchema(ctx, db)
	if err != nil {
		db.Close()
		return nil, err
	}
	for _, statement := range []string{
		"CREATE TABLE IF NOT EXISTS session_exclusions (kind TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(kind, value))",
		"CREATE TABLE IF NOT EXISTS session_sources (source_path TEXT PRIMARY KEY, identity TEXT NOT NULL, cursor INTEGER NOT NULL, partial_line BLOB NOT NULL DEFAULT X'', size INTEGER NOT NULL, modified_at INTEGER NOT NULL, changed_at INTEGER NOT NULL DEFAULT 0, prefix_hash TEXT NOT NULL, priority INTEGER NOT NULL, parser_version INTEGER NOT NULL, scanned_at TEXT NOT NULL, parser_context TEXT NOT NULL DEFAULT '')",
		// Source preparation and publication both resolve renames by identity.
		// Keep this non-unique: distinct discovered paths may share an inode.
		"CREATE INDEX IF NOT EXISTS session_sources_identity ON session_sources(identity)",
		"CREATE TABLE IF NOT EXISTS session_metadata (source_path TEXT NOT NULL REFERENCES session_sources(source_path) ON DELETE CASCADE, client TEXT NOT NULL, session_id TEXT NOT NULL, project TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '', parser_version INTEGER NOT NULL, first_at TEXT NOT NULL, last_at TEXT NOT NULL, PRIMARY KEY(source_path, client, session_id))",
		"CREATE VIRTUAL TABLE IF NOT EXISTS session_documents USING fts5(source_path UNINDEXED, client UNINDEXED, session_id UNINDEXED, event_at UNINDEXED, kind UNINDEXED, text)",
	} {
		if _, err = db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	if rebuilt {
		_, err = MintSessionIndexEpoch(ctx, db)
	} else {
		err = ensureSessionIndexGeneration(ctx, db)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	if err = ensureSessionServingStorage(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{DB: db, path: path}
	if err = s.secureFiles(); err != nil {
		db.Close()
		return nil, err
	}
	primeServingReadPool(ctx, path)
	return s, nil
}

// OpenSessionsReadOnly opens an existing session-search index without creating
// state, applying migrations, changing permissions, or enabling WAL.
//
// busy_timeout mirrors OpenReadOnly's own (see its doc): a detached scanner
// can still hold a brief write lock on this index while this reads it.
func OpenSessionsReadOnly(ctx context.Context, stateRoot string) (*Store, error) {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(stateRoot, "sessions.sqlite3")
	db, release, err := acquireReadOnlySQL(ctx, path, "read-only")
	if err != nil {
		return nil, err
	}
	for _, table := range []string{"session_sources", "session_metadata"} {
		var present int
		if err = db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&present); err != nil {
			_ = release()
			return nil, err
		}
		if present != 1 {
			_ = release()
			return nil, fmt.Errorf("session index missing table %s", table)
		}
	}
	return &Store{DB: db, path: path, readOnly: true, releaseReadPool: release}, nil
}

// Session indexes are rebuildable. The first source-level schema upgrade
// deliberately discards the old client/session view instead of trying to
// invent source ownership for rows that never recorded it.
func migrateSessionSchema(ctx context.Context, db *sql.DB) (bool, error) {
	var hasDocuments, hasSources, hasSourcePath, hasChangedAt, hasEventAt int
	rebuilt := false
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='session_documents'").Scan(&hasDocuments); err != nil {
		return false, err
	}
	if hasDocuments != 0 {
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('session_documents') WHERE name='source_path'").Scan(&hasSourcePath); err != nil {
			return false, err
		}
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('session_documents') WHERE name='event_at'").Scan(&hasEventAt); err != nil {
			return false, err
		}
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='session_sources'").Scan(&hasSources); err != nil {
		return false, err
	}
	if hasSources != 0 {
		var sourceColumn int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('session_sources') WHERE name='source_path'").Scan(&sourceColumn); err != nil {
			return false, err
		}
		if sourceColumn == 0 {
			hasSourcePath = 0
		} else if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('session_sources') WHERE name='changed_at'").Scan(&hasChangedAt); err != nil {
			return false, err
		}
	}
	if (hasDocuments != 0 || hasSources != 0) && hasSourcePath == 0 {
		_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS session_documents; DROP TABLE IF EXISTS session_metadata; DROP TABLE IF EXISTS session_sources")
		if err != nil {
			return false, err
		}
		rebuilt = true
	}
	if hasDocuments != 0 && hasEventAt == 0 {
		_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS session_documents")
		if err != nil {
			return false, err
		}
		rebuilt = true
	}
	if hasSources != 0 && hasSourcePath != 0 && hasChangedAt == 0 {
		_, err := db.ExecContext(ctx, "ALTER TABLE session_sources ADD COLUMN changed_at INTEGER NOT NULL DEFAULT 0")
		if err != nil {
			return false, err
		}
		rebuilt = true
	}
	if hasSources != 0 && hasSourcePath != 0 {
		var hasContext int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('session_sources') WHERE name='parser_context'").Scan(&hasContext); err != nil {
			return false, err
		}
		if hasContext == 0 {
			if _, err := db.ExecContext(ctx, "ALTER TABLE session_sources ADD COLUMN parser_context TEXT NOT NULL DEFAULT ''"); err != nil {
				return false, err
			}
		}
	}
	return rebuilt, nil
}

var (
	ErrStateBusy     = &Error{Code: "state_busy"}
	ErrSchemaAhead   = &Error{Code: "schema_ahead"}
	ErrUnknownSchema = &Error{Code: "unknown_schema"}
	ErrLockLost      = &Error{Code: "lock_lost"}
)

// LockReason describes the classified state of a lock file.
type LockReason string

const (
	LockReasonLive         LockReason = "lock_live"
	LockReasonLegacy       LockReason = "lock_legacy"
	LockReasonOwnerUnknown LockReason = "lock_owner_unknown"
	LockReasonReclaimable  LockReason = "lock_reclaimable"
)

// ErrLockContention wraps ErrStateBusy when a named lock acquisition times out,
// carrying typed metadata about the contended resource and lock liveness.
type ErrLockContention struct {
	Resource        string
	Reason          string
	ActionKind      *string
	RecoveryCommand *string
}

func (e *ErrLockContention) Error() string {
	return fmt.Sprintf("%s: timed out waiting for %s lock", ErrStateBusy.Code, e.Resource)
}

func (e *ErrLockContention) Unwrap() error {
	return ErrStateBusy
}

func lockActionKind(reason string) *string {
	switch reason {
	case string(LockReasonLive):
		s := "retry"
		return &s
	case string(LockReasonLegacy):
		s := "manual_prerequisite"
		return &s
	default:
		return nil
	}
}

var lockWait = 5 * time.Second

type Error struct {
	Code string
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return e.Code
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// SchemaAhead reports a database whose schema version exceeds the version this
// binary supports. It is permanent until the binary is upgraded.
type SchemaAhead struct {
	Stored    int
	Supported int
}

func (e *SchemaAhead) Error() string {
	return fmt.Sprintf("%s: database version %d exceeds supported version %d; upgrade AgentDeck to open it", ErrSchemaAhead.Code, e.Stored, e.Supported)
}

func (e *SchemaAhead) Unwrap() error { return ErrSchemaAhead }

type Store struct {
	DB              *sql.DB
	path            string
	readOnly        bool
	releaseReadPool func() error
	closeOnce       sync.Once
	closeErr        error
}

type stateLock interface {
	Release() error
}

type lockAcquirer func(context.Context, string, time.Duration) (stateLock, error)

// Open creates or opens a private core database and applies explicit ordered
// migrations before returning it.
func Open(ctx context.Context, stateRoot string) (*Store, error) {
	return open(ctx, stateRoot, func(ctx context.Context, stateRoot string, timeout time.Duration) (stateLock, error) {
		return AcquireLock(ctx, stateRoot, timeout)
	})
}

// OpenExisting opens an already-created private core state without creating a
// missing root or database. Background derived-cache publication uses it so a
// detached worker can never resurrect state that its owner has removed.
func OpenExisting(ctx context.Context, stateRoot string) (*Store, error) {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(stateRoot)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm() != platform.DirectoryMode {
		return nil, errors.New("invalid existing state root")
	}
	path := filepath.Join(stateRoot, "agentdeck.sqlite3")
	file, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !file.Mode().IsRegular() || file.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("existing core database is not a regular private file")
	}
	return openAtExistingRoot(ctx, stateRoot, func(ctx context.Context, stateRoot string, timeout time.Duration) (stateLock, error) {
		return AcquireLock(ctx, stateRoot, timeout)
	}, false)
}

type alreadyHeldLock struct{}

func (alreadyHeldLock) Release() error { return nil }

// OpenWithLockHeld opens and migrates the core store when the caller already
// owns the state lock. It exists for compound operations that must update the
// core database and another state file under one lock.
func OpenWithLockHeld(ctx context.Context, stateRoot string) (*Store, error) {
	return open(ctx, stateRoot, func(context.Context, string, time.Duration) (stateLock, error) {
		return alreadyHeldLock{}, nil
	})
}

// OpenReadOnly opens an existing core database without creating state,
// applying migrations, changing permissions, or enabling WAL.
//
// busy_timeout matches every write path's connection (Open, above): a
// detached scan or watcher can still hold a brief write lock while this
// reads, and without a timeout SQLite fails that read immediately with
// SQLITE_BUSY instead of retrying, rather than blocking indefinitely --
// mode=ro already refuses to create or migrate anything.
func OpenReadOnly(ctx context.Context, stateRoot string) (*Store, error) {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(stateRoot, "agentdeck.sqlite3")
	db, release, err := acquireReadOnlySQL(ctx, path, "read-only")
	if err != nil {
		return nil, err
	}
	var version int
	if err = db.QueryRowContext(ctx, "SELECT version FROM schema_metadata").Scan(&version); err != nil {
		_ = release()
		return nil, err
	}
	if version > CurrentSchemaVersion {
		_ = release()
		return nil, &SchemaAhead{Stored: version, Supported: CurrentSchemaVersion}
	}
	return &Store{DB: db, path: path, readOnly: true, releaseReadPool: release}, nil
}

// schemaAheadAtRest only probes after lock acquisition fails. It never creates
// state, migrates the database, or changes its permissions.
func schemaAheadAtRest(ctx context.Context, stateRoot string) *SchemaAhead {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return nil
	}
	path, cleanup, err := schemaProbeSnapshot(ctx, stateRoot)
	if err != nil {
		return nil
	}
	defer cleanup()
	db, err := sql.Open("sqlite", "file:"+path+"?mode=rw")
	if err != nil {
		return nil
	}
	defer db.Close()
	version, err := schemaVersion(ctx, db)
	if err != nil || version <= CurrentSchemaVersion {
		return nil
	}
	return &SchemaAhead{Stored: version, Supported: CurrentSchemaVersion}
}

func open(ctx context.Context, stateRoot string, acquire lockAcquirer) (store *Store, err error) {
	stateRoot, err = platform.GuardStateRoot(stateRoot)
	if err != nil {
		return nil, err
	}
	if err := platform.EnsureStateRoot(stateRoot); err != nil {
		return nil, err
	}
	return openAtExistingRoot(ctx, stateRoot, acquire, true)
}

func openAtExistingRoot(ctx context.Context, stateRoot string, acquire lockAcquirer, create bool) (store *Store, err error) {
	lock, err := acquire(ctx, stateRoot, lockWait)
	if err != nil {
		if errors.Is(err, ErrStateBusy) {
			if ahead := schemaAheadAtRest(ctx, stateRoot); ahead != nil {
				return nil, ahead
			}
		}
		return nil, err
	}
	defer func() {
		releaseErr := lock.Release()
		if err == nil && releaseErr == nil {
			_ = hookrefusal.Clear(stateRoot)
		}
		if err != nil || releaseErr == nil {
			return
		}
		if store != nil {
			_ = store.Close()
			store = nil
		}
		err = releaseErr
	}()

	path := filepath.Join(stateRoot, "agentdeck.sqlite3")
	if create {
		err = preparePrivateSQLiteFiles(path)
	} else {
		err = prepareExistingPrivateSQLiteFile(path)
	}
	if err != nil {
		return nil, err
	}
	databasePath := path + "?" + writablePragmas
	if !create {
		databasePath = "file:" + path + "?mode=rw&" + writablePragmas
	}
	db, err := openWritableSQL(databasePath, "core")
	if err != nil {
		return nil, err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(ctx, db, migrations); err != nil {
		db.Close()
		return nil, err
	}
	store = &Store{DB: db, path: path}
	if err := store.secureFiles(); err != nil {
		db.Close()
		return nil, err
	}
	primeServingReadPool(ctx, path)
	return store, nil
}

func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		if s.releaseReadPool != nil {
			s.closeErr = s.releaseReadPool()
			return
		}
		if !s.readOnly && processReadPools.contains(s.path) {
			// Idle pooled readers keep SQLite from performing its usual last-
			// connection checkpoint. Preserve writer-close durability without
			// waiting for active readers or destroying their connection pools.
			_, s.closeErr = s.DB.ExecContext(context.Background(), "PRAGMA wal_checkpoint(PASSIVE)")
		}
		s.closeErr = errors.Join(s.closeErr, s.DB.Close())
		if !s.readOnly {
			s.closeErr = errors.Join(s.closeErr, s.secureFiles())
		}
	})
	return s.closeErr
}

// Exec is the write path used by early foundation code. It re-applies the
// owner-only mode because SQLite creates WAL sidecars lazily.
func (s *Store) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result, err := s.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return result, s.secureFiles()
}

func (s *Store) Setting(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.DB.QueryRowContext(ctx, "SELECT value FROM settings WHERE key=?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.Exec(ctx, `INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value WHERE settings.value IS NOT excluded.value`, key, value)
	return err
}

// ErrSettingsSecureFilesFailed wraps a secureFiles failure that happens
// after SetSettings' own transaction has already committed: the settings
// themselves are durably persisted, and only the defense-in-depth
// permission-hardening step failed. A caller that would otherwise roll back
// a compound write on any SetSettings error (Codex PR #5 ninth review, P2)
// must check for this first -- undoing another already-committed side
// effect (for example, an installed status-line route) here would make it
// disagree with what SetSettings durably wrote, which is strictly worse
// than a bare permissions warning.
var ErrSettingsSecureFilesFailed = errors.New("settings persisted but securing their file permissions failed")

// SetSettings writes every key in values in a single transaction, so a
// caller that groups several settings into one logical change (quota's
// SaveSettings, for example) cannot leave the settings table with only some
// of them written when a later key in the batch fails to persist.
func (s *Store) SetSettings(ctx context.Context, values map[string]string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, value := range values {
		if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value WHERE settings.value IS NOT excluded.value`, key, value); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := s.secureFiles(); err != nil {
		return fmt.Errorf("%w: %v", ErrSettingsSecureFilesFailed, err)
	}
	return nil
}

func (s *Store) DeleteSetting(ctx context.Context, key string) error {
	_, err := s.Exec(ctx, "DELETE FROM settings WHERE key=?", key)
	return err
}

func (s *Store) Backup(ctx context.Context, destination string) error {
	return backupDatabase(ctx, s.DB, destination)
}

// BackupSQLiteFile snapshots an existing SQLite database through the online
// backup API. It never copies a live database or its WAL sidecars directly.
func BackupSQLiteFile(ctx context.Context, source, destination string) error {
	db, err := sql.Open("sqlite", "file:"+source+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	return backupDatabase(ctx, db, destination)
}

func backupDatabase(ctx context.Context, db *sql.DB, destination string) error {
	if err := preparePrivateSQLiteFiles(destination); err != nil {
		return err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	err = conn.Raw(func(driverConn any) error {
		type backuper interface {
			NewBackup(string) (*sqlite.Backup, error)
		}
		backup, err := driverConn.(backuper).NewBackup(destination)
		if err != nil {
			return err
		}
		if _, err := backup.Step(-1); err != nil {
			_ = backup.Finish()
			return err
		}
		destinationConn, err := backup.Commit()
		if err != nil {
			return err
		}
		return destinationConn.Close()
	})
	if err != nil {
		return err
	}
	return os.Chmod(destination, platform.FileMode)
}

// SchemaVersion returns the on-disk core schema version without changing it.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var version int
	err := s.DB.QueryRowContext(ctx, "SELECT version FROM schema_metadata").Scan(&version)
	return version, err
}

// IntegrityCheck runs SQLite's read-only integrity check.
func (s *Store) IntegrityCheck(ctx context.Context) (string, error) {
	var result string
	err := s.DB.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result)
	return result, err
}

// preparePrivateSQLiteFiles makes the database and its sidecars private
// without ever opening and closing a file that already exists. SQLite guards a
// database with POSIX locks on the database file and on its -shm file, and
// closing any descriptor of a file drops every POSIX lock the process holds on
// it. A second open of the same database in one process must therefore not
// touch the existing files through a descriptor: doing so released the first
// connection's locks, let another process reinitialize the shared memory, and
// crashed the first one on its next WAL access.
func preparePrivateSQLiteFiles(path string) error {
	for _, candidate := range []string{path, path + "-wal", path + "-shm", path + "-journal"} {
		err := os.Chmod(candidate, platform.FileMode)
		if err == nil {
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		file, err := os.OpenFile(candidate, os.O_RDWR|os.O_CREATE|os.O_EXCL, platform.FileMode)
		if errors.Is(err, fs.ErrExist) {
			// Created by another opener since the chmod above.
			if err = os.Chmod(candidate, platform.FileMode); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

// prepareExistingPrivateSQLiteFile checks the existing database by name, for
// the reason given on preparePrivateSQLiteFiles.
func prepareExistingPrivateSQLiteFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != platform.FileMode {
		return errors.New("existing core database is not a regular private file")
	}
	return nil
}

func (s *Store) secureFiles() error {
	for _, path := range []string{s.path, s.path + "-wal", s.path + "-shm", s.path + "-journal"} {
		if err := os.Chmod(path, platform.FileMode); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

type Lock struct {
	mu       sync.Mutex
	path     string
	dir      *os.File // acquisition directory, kept open through the last release attempt
	token    string
	info     os.FileInfo
	released bool
}

// AcquiredIn compares the open directory handles. It does not prove that
// the acquired lock entry is still present.
func (l *Lock) AcquiredIn(dir *os.File) bool {
	if l == nil || l.dir == nil || dir == nil {
		return false
	}
	acquired, err := l.dir.Stat()
	if err != nil {
		return false
	}
	want, err := dir.Stat()
	return err == nil && os.SameFile(acquired, want)
}

// ownedLockRecord identifies the directory entry a lock was acquired at: the
// directory holding it, the name it was created under and the file it names.
type ownedLockRecord struct {
	dir  os.FileInfo
	name string
	info os.FileInfo
}

var ownedLocks = struct {
	mu      sync.Mutex
	records map[string]ownedLockRecord
}{
	records: make(map[string]ownedLockRecord),
}

func registerOwnedLock(token string, record ownedLockRecord) {
	ownedLocks.mu.Lock()
	ownedLocks.records[token] = record
	ownedLocks.mu.Unlock()
}

func unregisterOwnedLock(token string) {
	ownedLocks.mu.Lock()
	delete(ownedLocks.records, token)
	ownedLocks.mu.Unlock()
}

// isOwnedLockAt reports whether one observation of the entry name in dir is
// the entry this process acquired with the observed token. Entries are
// compared as the file system resolves them, not as strings: the same
// directory, the same file, and the same directory entry even if the caller
// uses another spelling on a case-insensitive volume.
func isOwnedLockAt(dir *os.File, name string, entry lockEntry) bool {
	if dir == nil || entry.dir == nil || entry.info == nil {
		return false
	}
	pid, ok := lockOwnerPID(entry.token)
	if !ok || pid != os.Getpid() {
		return false
	}
	ownedLocks.mu.Lock()
	rec, exists := ownedLocks.records[entry.token]
	ownedLocks.mu.Unlock()
	if !exists || rec.dir == nil || !os.SameFile(rec.dir, entry.dir) || !os.SameFile(rec.info, entry.info) {
		return false
	}
	if name == rec.name {
		return true
	}
	if !strings.EqualFold(name, rec.name) {
		return false
	}
	created, err := statLockEntryAt(dir, rec.name)
	if err != nil || !sameLockFile(created, entry.info) {
		return false
	}
	if lockLinkCount(entry.info) == 1 {
		return true
	}
	// All acquired lock names are ASCII. An equally long EqualFold match can
	// therefore differ only in ASCII case. With multiple links this proves
	// one entry only if this directory's lookup ignores those differences.
	return len(name) == len(rec.name) && platform.DirectoryIgnoresASCIICase(dir)
}

// statLockEntryAt describes the entry name in dir without following a
// symlink, resolved from that directory descriptor rather than a path.
func statLockEntryAt(dir *os.File, name string) (unix.Stat_t, error) {
	var st unix.Stat_t
	err := atLockDir(dir, func(dirFD int) error {
		for {
			err := unix.Fstatat(dirFD, name, &st, unix.AT_SYMLINK_NOFOLLOW)
			if !errors.Is(err, unix.EINTR) {
				return err
			}
		}
	})
	return st, err
}

func sameLockFile(st unix.Stat_t, info os.FileInfo) bool {
	other, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Mode&unix.S_IFMT == unix.S_IFREG &&
		uint64(st.Dev) == uint64(other.Dev) && uint64(st.Ino) == uint64(other.Ino)
}

// atLockDir runs op with the raw descriptor of the open directory dir.
func atLockDir(dir *os.File, op func(dirFD int) error) error {
	conn, err := dir.SyscallConn()
	if err != nil {
		return err
	}
	var opErr error
	if err := conn.Control(func(fd uintptr) { opErr = op(int(fd)) }); err != nil {
		return err
	}
	return opErr
}

// openLockDir opens the directory holding the lock entry at path, so creating,
// observing and identifying the entry all resolve from one directory.
func openLockDir(path string) (*os.File, os.FileInfo, error) {
	dir, err := os.OpenFile(filepath.Dir(path), os.O_RDONLY|unix.O_DIRECTORY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := dir.Stat()
	if err != nil {
		_ = dir.Close()
		return nil, nil, err
	}
	return dir, info, nil
}

// openLockEntryAt opens name in dir, retrying an interrupted open.
func openLockEntryAt(dir *os.File, name string, flags int, mode uint32) (int, error) {
	fd := -1
	err := atLockDir(dir, func(dirFD int) error {
		for {
			var err error
			fd, err = unix.Openat(dirFD, name, flags|unix.O_CLOEXEC, mode)
			if !errors.Is(err, unix.EINTR) {
				return err
			}
		}
	})
	return fd, err
}

func lockLinkCount(info os.FileInfo) uint64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(stat.Nlink)
	}
	return 0
}

func AcquireLock(ctx context.Context, stateRoot string, timeout time.Duration) (*Lock, error) {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return nil, err
	}
	lock, err := acquireNamedLock(ctx, stateRoot, "state.lock", timeout)
	if err != nil && errors.Is(err, ErrStateBusy) {
		return nil, wrapLockContention(stateRoot, "state.lock", "state", err)
	}
	return lock, err
}

// AcquireScanLock serializes foreground scans without blocking state mutations
// or read commands that use the short-lived state lock.
func AcquireScanLock(ctx context.Context, stateRoot string, timeout time.Duration) (*Lock, error) {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return nil, err
	}
	lock, err := acquireNamedLock(ctx, stateRoot, "scan.lock", timeout)
	if err != nil && errors.Is(err, ErrStateBusy) {
		return nil, wrapLockContention(stateRoot, "scan.lock", "scan", err)
	}
	return lock, err
}

func wrapLockContention(stateRoot, name, resource string, baseErr error) error {
	path := filepath.Join(stateRoot, name)
	reason, err := ClassifyLock(path, lockProcessAlive)
	reasonStr := string(reason)
	if err != nil || reasonStr == "" {
		reasonStr = string(LockReasonOwnerUnknown)
	}
	return &ErrLockContention{
		Resource:   resource,
		Reason:     reasonStr,
		ActionKind: lockActionKind(reasonStr),
	}
}

// InspectScanLockContention reports a contended scan lock after a detached
// worker failed to expose its socket. It does not acquire or remove the lock.
func InspectScanLockContention(stateRoot string) *ErrLockContention {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return nil
	}
	reason, err := ClassifyLock(filepath.Join(stateRoot, "scan.lock"), lockProcessAlive)
	if errors.Is(err, fs.ErrNotExist) || reason == LockReasonReclaimable {
		return nil
	}
	if err != nil || reason == "" {
		reason = LockReasonOwnerUnknown
	}
	reasonStr := string(reason)
	return &ErrLockContention{
		Resource:   "scan",
		Reason:     reasonStr,
		ActionKind: lockActionKind(reasonStr),
	}
}

// AcquireQuotaRefreshLock serializes the complete subscription-quota refresh
// cycle across helper processes. The cycle includes external client probes and
// the subsequent window replacement, so it deliberately has its own lock
// domain rather than holding state.lock across network/process I/O.
func AcquireQuotaRefreshLock(ctx context.Context, stateRoot string, timeout time.Duration) (*Lock, error) {
	return acquireNamedLock(ctx, stateRoot, "quota-refresh.lock", timeout)
}

// AcquireQuotaRefreshLockBefore keeps the quota command's original cutoff
// through directory validation, stale-owner checks and polling wakeups.
func AcquireQuotaRefreshLockBefore(ctx context.Context, stateRoot string, cutoff time.Time) (*Lock, error) {
	if cutoff.IsZero() || !time.Now().Before(cutoff) {
		return nil, ErrStateBusy
	}
	return acquireNamedLockBefore(ctx, stateRoot, "quota-refresh.lock", time.Until(cutoff), lockProcessAlive, tryLockReclaimFile, cutoff, time.After)
}

// CheckQuotaRefreshLock verifies the lock is still held for this core.
func (s *Store) CheckQuotaRefreshLock(lock *Lock) error {
	if s.readOnly || lock == nil || lock.token == "" ||
		lock.path != filepath.Join(filepath.Dir(s.path), "quota-refresh.lock") {
		return ErrLockLost
	}
	entry, _, err := readLockFile(lock.path)
	if err != nil {
		return errors.Join(ErrLockLost, err)
	}
	if !entry.owned || entry.token != lock.token {
		return ErrLockLost
	}
	return nil
}

// AcquireDerivedSnapshotCacheLock serializes cache publishers without sharing
// the scan or state-mutation lock domains.
func AcquireDerivedSnapshotCacheLock(ctx context.Context, stateRoot string, timeout time.Duration) (*Lock, error) {
	return acquireNamedLock(ctx, stateRoot, "derived-snapshot-cache.lock", timeout)
}

func AcquireServingSnapshotLock(ctx context.Context, stateRoot string, timeout time.Duration) (*Lock, error) {
	return acquireNamedLock(ctx, stateRoot, "serving-snapshot.lock", timeout)
}

func acquireNamedLock(ctx context.Context, stateRoot, name string, timeout time.Duration) (*Lock, error) {
	return acquireNamedLockWithProcessCheck(ctx, stateRoot, name, timeout, lockProcessAlive)
}

// LockProcessCheck determines whether the process with the given PID is alive and known.
type LockProcessCheck func(int) (alive, known bool)

type lockProcessCheck = LockProcessCheck

func acquireNamedLockWithProcessCheck(ctx context.Context, stateRoot, name string, timeout time.Duration, processAlive lockProcessCheck) (*Lock, error) {
	return acquireNamedLockWithChecks(ctx, stateRoot, name, timeout, processAlive, tryLockReclaimFile)
}

type reclaimFileLock func(*os.File) (bool, error)

// lockCreatedHook, when set by a test, runs after a lock entry has been
// created and before its identity is recorded.
var lockCreatedHook func(path string)

// createLockEntry exclusively creates the lock entry at path holding token.
// The directory is opened once; the entry is created relative to it and both
// identities recorded for ownership come from that one resolution, so a
// directory moved or replaced meanwhile cannot mix two resolutions. An
// existing entry fails with an error matching fs.ErrExist.
func createLockEntry(path, token string) (*Lock, error) {
	dir, dirInfo, err := openLockDir(path)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(path)
	fd, err := openLockEntryAt(dir, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL, uint32(platform.FileMode.Perm()))
	if err != nil {
		_ = dir.Close()
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	_, writeErr := file.WriteString(token)
	info, statErr := file.Stat()
	if err := errors.Join(writeErr, statErr, file.Close()); err != nil {
		if info != nil {
			_ = unlinkLockEntry(dir, name, info)
		}
		_ = dir.Close()
		return nil, err
	}
	if lockCreatedHook != nil {
		lockCreatedHook(path)
	}
	registerOwnedLock(token, ownedLockRecord{dir: dirInfo, name: name, info: info})
	return &Lock{path: path, dir: dir, token: token, info: info}, nil
}

func acquireNamedLockWithChecks(ctx context.Context, stateRoot, name string, timeout time.Duration, processAlive lockProcessCheck, tryReclaimLock reclaimFileLock) (*Lock, error) {
	return acquireNamedLockBefore(ctx, stateRoot, name, timeout, processAlive, tryReclaimLock, time.Time{}, time.After)
}

func acquireNamedLockBefore(ctx context.Context, stateRoot, name string, timeout time.Duration, processAlive lockProcessCheck, tryReclaimLock reclaimFileLock, cutoff time.Time, after func(time.Duration) <-chan time.Time) (*Lock, error) {
	stateRoot, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(stateRoot, name)
	token, err := newLockToken()
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	if !cutoff.IsZero() {
		deadline = cutoff
	}
	for {
		if !cutoff.IsZero() {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if time.Now().After(cutoff) {
				return nil, ErrStateBusy
			}
		}
		lock, err := createLockEntry(path, token)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		reclaimed, reclaimErr := reclaimLockFromDeadProcess(path, processAlive, tryReclaimLock)
		if reclaimErr != nil {
			return nil, reclaimErr
		}
		if reclaimed {
			continue
		}
		if timeout <= 0 || !time.Now().Before(deadline) {
			return nil, fmt.Errorf("%w: timed out waiting for state lock", ErrStateBusy)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-after(min(25*time.Millisecond, time.Until(deadline))):
		}
	}
}

const maxLockTokenBytes = 64

type lockFileState int

const (
	lockFileOK lockFileState = iota
	lockFileLegacy
	lockFileOwnerUnknown
)

// lockEntry is one observation of a lock's directory entry: the directory
// holding it, the regular file it names, and the token read from that file.
type lockEntry struct {
	dir   os.FileInfo
	info  os.FileInfo
	token string
	owned bool
}

// errLockEntryIrregular reports an entry that is not a regular file, such as a
// symlink, FIFO or directory: it cannot be a lock this protocol wrote.
var errLockEntryIrregular = errors.New("lock entry is not a regular file")

// openLockEntry opens the entry at path for reading without blocking. The
// directory is opened first and the entry relative to it, so both identities
// come from the same resolution; the final component is not followed, and
// O_NONBLOCK keeps a FIFO swapped in from waiting for a writer. Only a regular
// file is returned, open, together with its open directory; the caller closes
// both. An entry that cannot be opened because it is not a regular file, such
// as a Unix socket Darwin refuses to open, is reported as irregular after a
// no-follow look at the same directory; any other failure stays as it is.
func openLockEntry(path string) (*os.File, *os.File, lockEntry, error) {
	dir, dirInfo, err := openLockDir(path)
	if err != nil {
		if errors.Is(err, unix.ENOTDIR) {
			return nil, nil, lockEntry{}, errLockEntryIrregular
		}
		return nil, nil, lockEntry{}, err
	}
	file, info, err := openRegularLockEntry(dir, path)
	if err != nil {
		_ = dir.Close()
		return nil, nil, lockEntry{}, err
	}
	return dir, file, lockEntry{dir: dirInfo, info: info}, nil
}

func openRegularLockEntry(dir *os.File, path string) (*os.File, os.FileInfo, error) {
	name := filepath.Base(path)
	fd, openErr := openLockEntryAt(dir, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if openErr != nil {
		if errors.Is(openErr, unix.ELOOP) {
			return nil, nil, errLockEntryIrregular
		}
		if !errors.Is(openErr, unix.ENOENT) {
			if st, err := statLockEntryAt(dir, name); err == nil && st.Mode&unix.S_IFMT != unix.S_IFREG {
				return nil, nil, errLockEntryIrregular
			}
		}
		return nil, nil, &fs.PathError{Op: "open", Path: path, Err: openErr}
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, errLockEntryIrregular
	}
	return file, info, nil
}

// readLockFile makes one bounded observation of the lock entry at path,
// including whether it is the entry this process acquired, decided while the
// directory of that same observation is still open.
func readLockFile(path string) (lockEntry, lockFileState, error) {
	dir, file, entry, err := openLockEntry(path)
	if errors.Is(err, fs.ErrNotExist) {
		return lockEntry{}, lockFileOwnerUnknown, fs.ErrNotExist
	}
	if err != nil {
		return lockEntry{}, lockFileOwnerUnknown, nil
	}
	defer dir.Close()
	defer file.Close()
	if entry.info.Size() > maxLockTokenBytes {
		return lockEntry{}, lockFileLegacy, nil
	}
	contents, err := io.ReadAll(io.LimitReader(file, maxLockTokenBytes+1))
	if err != nil {
		return lockEntry{}, lockFileOwnerUnknown, nil
	}
	if len(contents) > maxLockTokenBytes {
		return lockEntry{}, lockFileLegacy, nil
	}
	entry.token = string(contents)
	entry.owned = isOwnedLockAt(dir, filepath.Base(path), entry)
	return entry, lockFileOK, nil
}

// ClassifyLock inspects the lock file at path without mutating it or holding locks.
func ClassifyLock(path string, check LockProcessCheck) (LockReason, error) {
	reason, _, err := classifyLock(path, check, false)
	return reason, err
}

// ClassifyLockOwnership inspects the lock file at path in a single read and
// reports both its liveness reason and whether it is genuinely held by this process.
func ClassifyLockOwnership(path string, check LockProcessCheck) (LockReason, bool, error) {
	return classifyLock(path, check, true)
}

func classifyLock(path string, check LockProcessCheck, checkOwnership bool) (LockReason, bool, error) {
	entry, state, err := readLockFile(path)
	if err != nil {
		return "", false, err
	}
	switch state {
	case lockFileLegacy:
		return LockReasonLegacy, false, nil
	case lockFileOwnerUnknown:
		return LockReasonOwnerUnknown, false, nil
	}
	pid, ok := lockOwnerPID(entry.token)
	if !ok {
		return LockReasonLegacy, false, nil
	}
	if check == nil {
		check = lockProcessAlive
	}
	alive, known := check(pid)
	if !known {
		return LockReasonOwnerUnknown, false, nil
	}
	if !alive {
		return LockReasonReclaimable, false, nil
	}
	if checkOwnership {
		return LockReasonLive, entry.owned, nil
	}
	return LockReasonLive, false, nil
}

func reclaimLockFromDeadProcess(path string, processAlive lockProcessCheck, tryReclaimLock reclaimFileLock) (bool, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, nil
	}
	defer file.Close()
	guarded, err := tryReclaimLock(file)
	if err != nil {
		return false, nil
	}
	if !guarded {
		return false, nil
	}
	openedInfo, err := file.Stat()
	if err != nil {
		return false, nil
	}
	pathInfo, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil || !os.SameFile(openedInfo, pathInfo) {
		return false, nil
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return false, nil
	}
	pid, ok := lockOwnerPID(string(contents))
	if !ok {
		// Legacy and malformed tokens carry no trustworthy owner identity.
		return false, nil
	}
	alive, known := processAlive(pid)
	if !known || alive {
		return false, nil
	}
	pathInfo, err = os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil || !os.SameFile(openedInfo, pathInfo) {
		return false, nil
	}
	latest, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil || string(latest) != string(contents) {
		return false, nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	return true, nil
}

func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.released {
		return nil
	}
	err := l.releaseOnce()
	if err == nil || errors.Is(err, ErrLockLost) {
		l.released = true
		unregisterOwnedLock(l.token)
		if l.dir != nil {
			_ = l.dir.Close()
		}
	}
	return err
}

func (l *Lock) releaseOnce() error {
	if l.dir == nil {
		return ErrLockLost
	}
	file, info, err := openRegularLockEntry(l.dir, l.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if errors.Is(err, errLockEntryIrregular) {
		// Not a regular file, however it failed to open: it cannot be the
		// lock this handle wrote, so the lock is lost and forgotten.
		return ErrLockLost
	}
	if err != nil {
		// Permission and transient I/O failures prove nothing about the
		// entry; the release stays retryable and the lock registered.
		return err
	}
	defer file.Close()
	if !os.SameFile(l.info, info) {
		return ErrLockLost
	}
	contents, readErr := io.ReadAll(io.LimitReader(file, maxLockTokenBytes+1))
	if readErr != nil {
		return readErr
	}
	if string(contents) != l.token {
		return ErrLockLost
	}
	return unlinkLockEntry(l.dir, filepath.Base(l.path), info)
}

// Recheck the name against the opened file before unlinking it. Both lookups
// stay relative to the acquisition directory even if its old path is reused.
func unlinkLockEntry(dir *os.File, name string, info os.FileInfo) error {
	st, err := statLockEntryAt(dir, name)
	if err == nil {
		if !sameLockFile(st, info) {
			return ErrLockLost
		}
		err = atLockDir(dir, func(fd int) error { return unix.Unlinkat(fd, name, 0) })
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func newLockToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("v1:%d:%s", os.Getpid(), hex.EncodeToString(bytes)), nil
}

// LockOwnedByCurrentProcess reports whether path names the lock entry this
// process acquired and still holds: the same directory entry and file
// identity, carrying the full token it wrote.
func LockOwnedByCurrentProcess(path string) bool {
	entry, state, err := readLockFile(path)
	if err != nil || state != lockFileOK {
		return false
	}
	return entry.owned
}

func lockOwnerPID(token string) (int, bool) {
	parts := strings.Split(token, ":")
	if len(parts) != 3 || parts[0] != "v1" || len(parts[2]) != 32 {
		return 0, false
	}
	if decoded, err := hex.DecodeString(parts[2]); err != nil || len(decoded) != 16 {
		return 0, false
	}
	pid, err := strconv.Atoi(parts[1])
	return pid, err == nil && pid > 0
}
