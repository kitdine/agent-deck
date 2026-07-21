// Package store owns AgentDeck's SQLite lifecycle and schema boundaries.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"modernc.org/sqlite"
)

const CurrentSchemaVersion = 13

// OpenSessions opens the separately purgeable session-search database. It is
// deliberately not part of the core schema so deleting the index can never
// delete providers, credentials, or usage data.
func OpenSessions(ctx context.Context, stateRoot string) (*Store, error) {
	if err := platform.EnsureStateRoot(stateRoot); err != nil {
		return nil, err
	}
	path := filepath.Join(stateRoot, "sessions.sqlite3")
	if err := preparePrivateSQLiteFiles(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if _, err = db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateSessionSchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	for _, statement := range []string{
		"CREATE TABLE IF NOT EXISTS session_exclusions (kind TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(kind, value))",
		"CREATE TABLE IF NOT EXISTS session_sources (source_path TEXT PRIMARY KEY, identity TEXT NOT NULL, cursor INTEGER NOT NULL, partial_line BLOB NOT NULL DEFAULT X'', size INTEGER NOT NULL, modified_at INTEGER NOT NULL, prefix_hash TEXT NOT NULL, priority INTEGER NOT NULL, parser_version INTEGER NOT NULL, scanned_at TEXT NOT NULL)",
		"CREATE TABLE IF NOT EXISTS session_metadata (source_path TEXT NOT NULL REFERENCES session_sources(source_path) ON DELETE CASCADE, client TEXT NOT NULL, session_id TEXT NOT NULL, project TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '', parser_version INTEGER NOT NULL, first_at TEXT NOT NULL, last_at TEXT NOT NULL, PRIMARY KEY(source_path, client, session_id))",
		"CREATE VIRTUAL TABLE IF NOT EXISTS session_documents USING fts5(source_path UNINDEXED, client UNINDEXED, session_id UNINDEXED, kind UNINDEXED, text)",
	} {
		if _, err = db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	s := &Store{DB: db, path: path}
	if err = s.secureFiles(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Session indexes are rebuildable.  The first source-level schema upgrade
// deliberately discards the old client/session view instead of trying to
// invent source ownership for rows that never recorded it.
func migrateSessionSchema(ctx context.Context, db *sql.DB) error {
	var hasDocuments, hasSources, hasSourcePath int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='session_documents'").Scan(&hasDocuments); err != nil {
		return err
	}
	if hasDocuments != 0 {
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('session_documents') WHERE name='source_path'").Scan(&hasSourcePath); err != nil {
			return err
		}
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='session_sources'").Scan(&hasSources); err != nil {
		return err
	}
	if hasSources != 0 {
		var sourceColumn int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('session_sources') WHERE name='source_path'").Scan(&sourceColumn); err != nil {
			return err
		}
		if sourceColumn == 0 {
			hasSourcePath = 0
		}
	}
	if (hasDocuments != 0 || hasSources != 0) && hasSourcePath == 0 {
		_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS session_documents; DROP TABLE IF EXISTS session_metadata; DROP TABLE IF EXISTS session_sources")
		return err
	}
	return nil
}

var (
	ErrStateBusy     = &Error{Code: "state_busy"}
	ErrUnknownSchema = &Error{Code: "unknown_schema"}
	ErrLockLost      = &Error{Code: "lock_lost"}
)

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

type Store struct {
	DB       *sql.DB
	path     string
	readOnly bool
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
func OpenReadOnly(ctx context.Context, stateRoot string) (*Store, error) {
	path := filepath.Join(stateRoot, "agentdeck.sqlite3")
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	var version int
	if err = db.QueryRowContext(ctx, "SELECT version FROM schema_metadata").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if version > CurrentSchemaVersion {
		db.Close()
		return nil, ErrUnknownSchema
	}
	return &Store{DB: db, path: path, readOnly: true}, nil
}

func open(ctx context.Context, stateRoot string, acquire lockAcquirer) (store *Store, err error) {
	if err := platform.EnsureStateRoot(stateRoot); err != nil {
		return nil, err
	}
	lock, err := acquire(ctx, stateRoot, lockWait)
	if err != nil {
		return nil, err
	}
	defer func() {
		releaseErr := lock.Release()
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
	if err := preparePrivateSQLiteFiles(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
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
	return store, nil
}

func (s *Store) Close() error {
	err := s.DB.Close()
	if s.readOnly {
		return err
	}
	if secureErr := s.secureFiles(); err == nil {
		err = secureErr
	}
	return err
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

func preparePrivateSQLiteFiles(path string) error {
	for _, candidate := range []string{path, path + "-wal", path + "-shm", path + "-journal"} {
		file, err := os.OpenFile(candidate, os.O_RDWR|os.O_CREATE, platform.FileMode)
		if err != nil {
			return err
		}
		if err := file.Chmod(platform.FileMode); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
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
	path  string
	token string
}

func AcquireLock(ctx context.Context, stateRoot string, timeout time.Duration) (*Lock, error) {
	return acquireNamedLock(ctx, stateRoot, "state.lock", timeout)
}

// AcquireScanLock serializes foreground scans without blocking state mutations
// or read commands that use the short-lived state lock.
func AcquireScanLock(ctx context.Context, stateRoot string, timeout time.Duration) (*Lock, error) {
	return acquireNamedLock(ctx, stateRoot, "scan.lock", timeout)
}

func acquireNamedLock(ctx context.Context, stateRoot, name string, timeout time.Duration) (*Lock, error) {
	path := filepath.Join(stateRoot, name)
	token, err := newLockToken()
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, platform.FileMode)
		if err == nil {
			if _, writeErr := file.WriteString(token); writeErr != nil {
				_ = file.Close()
				_ = os.Remove(path)
				return nil, writeErr
			}
			if err := file.Close(); err != nil {
				return nil, err
			}
			return &Lock{path: path, token: token}, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		if timeout <= 0 || !time.Now().Before(deadline) {
			return nil, fmt.Errorf("%w: timed out waiting for state lock", ErrStateBusy)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(min(25*time.Millisecond, time.Until(deadline))):
		}
	}
}

func (l *Lock) Release() error {
	contents, err := os.ReadFile(l.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if string(contents) != l.token {
		return ErrLockLost
	}
	return os.Remove(l.path)
}

func newLockToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
