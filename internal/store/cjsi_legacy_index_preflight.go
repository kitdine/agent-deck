package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/kitdine/agent-deck/internal/platform"
	"golang.org/x/sys/unix"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// ErrLegacySessionIndexInvalid reports a legacy index that migration cannot
// read safely. The original index is retained for inspection or recovery.
var ErrLegacySessionIndexInvalid = &Error{Code: "legacy_session_index_invalid"}

// Reserved for disposable verification only. This directory is distinct from
// the durable layout snapshots and from every Restore staging name.
const cjsiPreflightDirName = ".cjsi-legacy-index-preflight-v1"
const cjsiPreflightSnapshotName = "verify.sqlite3"

// CheckLegacySessionIndexForMigration verifies the legacy source without
// editing it. A disposable copy, when needed, stays in the state root.
func CheckLegacySessionIndexForMigration(ctx context.Context, root string) (err error) {
	root, err = platform.GuardStateRoot(root)
	if err != nil {
		return err
	}
	path := filepath.Join(root, cjsiLegacyIndexName)
	pin, err := admitLegacy(ctx, root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, pin.close()) }()
	_, err = inspectLegacySessionIndex(ctx, path)
	return err
}

// The caller owns scan/state locks and has drained SQLite admission. A zero
// length main with empty sidecars is the legacy opener's initial empty index.
func prepareLegacySessionIndexForMigration(ctx context.Context, root string) error {
	path := filepath.Join(root, cjsiLegacyIndexName)
	if err := normalizeLegacySQLitePermissions(path); err != nil {
		return err
	}
	empty, err := inspectLegacySessionIndex(ctx, path)
	if err != nil || !empty {
		return err
	}
	db, err := sql.Open("sqlite", cjsiSQLiteURI(path, "rw"))
	if err != nil {
		return legacyIndexPreflightError(ctx, err)
	}
	initErr := func() error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, cjsiSessionBaselineSchema); err != nil {
			return err
		}
		return tx.Commit()
	}()
	return legacyIndexPreflightError(ctx, errors.Join(initErr, db.Close()))
}

func inspectLegacySessionIndex(ctx context.Context, path string) (empty bool, err error) {
	err = withLegacyIndexPreflightDir(filepath.Dir(path), func(snapshot string) error {
		empty, err = inspectLegacySessionIndexCopy(ctx, path, snapshot)
		return legacyIndexPreflightError(ctx, err)
	})
	return empty, err
}

func inspectLegacySessionIndexCopy(ctx context.Context, path, snapshot string) (bool, error) {
	stat, err := ownedLegacySQLiteInfo(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, inspectLegacySQLiteSidecars(path, false)
	}
	if err != nil {
		return false, err
	}
	if stat.Size() == 0 {
		return true, inspectLegacySQLiteSidecars(path, false)
	}
	if err := inspectLegacySQLiteSidecars(path, true); err != nil {
		return false, err
	}
	// SQLite can omit CHECK validation on a read-only handle. Use the same
	// standalone verification as capture, with a disposable writable copy.
	if err := cjsiBackupLayoutIndex(ctx, path, snapshot); err != nil {
		return false, err
	}
	return false, verifyLegacyPreflightContents(ctx, snapshot)
}

func ownedLegacySQLiteInfo(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err := checkLegacySQLiteOwner(path, info); err != nil {
		return nil, err
	}
	return info, nil
}

func checkLegacySQLiteOwner(path string, info os.FileInfo) error {
	st := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || st.Nlink != 1 || st.Uid != uint32(os.Getuid()) {
		return &os.PathError{Op: "validate legacy SQLite file", Path: path, Err: os.ErrPermission}
	}
	return nil
}

// Run only under drained admission. Unlike a diagnostic, a migration can
// secure owned legacy files before capture. Existing strict SQLite readers
// must keep rejecting aliases and avoid relaxing the private layout policy.
func normalizeLegacySQLitePermissions(path string) error {
	for _, suffix := range [...]string{"", "-wal", "-shm", "-journal"} {
		name := path + suffix
		info, err := ownedLegacySQLiteInfo(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		mode := info.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
		if mode == platform.FileMode {
			continue
		}
		fd, err := unix.Open(name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil {
			return &os.PathError{Op: "open", Path: name, Err: err}
		}
		file := os.NewFile(uintptr(fd), name)
		opened, err := file.Stat()
		if err == nil {
			err = checkLegacySQLiteOwner(name, opened)
		}
		if err == nil && !os.SameFile(info, opened) {
			err = &os.PathError{Op: "bind legacy SQLite file", Path: name, Err: os.ErrPermission}
		}
		if err == nil {
			err = file.Chmod(platform.FileMode)
		}
		if err := errors.Join(err, file.Close()); err != nil {
			return err
		}
	}
	return nil
}

// Diagnostic reads accept mode drift without chmod. A rollback journal or
// an orphaned nonempty sidecar needs recovery before any content verdict.
func inspectLegacySQLiteSidecars(path string, hasMain bool) error {
	for _, suffix := range cjsiSidecarSuffixes {
		info, err := ownedLegacySQLiteInfo(path + suffix)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if (suffix == "-journal" || !hasMain) && info.Size() != 0 {
			return errCJSILayoutInvalid
		}
	}
	return nil
}

// Report confirmed content failures explicitly. The general layout verifier
// also uses its invalid sentinel for filesystem and publication errors.
func verifyLegacyPreflightContents(ctx context.Context, snapshot string) (err error) {
	db, err := sql.Open("sqlite", cjsiSQLiteURI(snapshot, "rw"))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("%w: SQLite integrity check failed", ErrLegacySessionIndexInvalid)
	}
	var tables int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM main.sqlite_schema
		WHERE type='table' AND name IN ('session_sources','session_metadata')`).Scan(&tables); err != nil {
		return err
	}
	if tables != 2 {
		return fmt.Errorf("%w: missing legacy session tables", ErrLegacySessionIndexInvalid)
	}
	return nil
}

// Use the directory itself as the restart-safe lock. A second preflight will
// leave an active copy alone. The unlink checks also refuse replacements and
// aliases, including entries left by an unrelated process at reserved names.
func withLegacyIndexPreflightDir(root string, run func(string) error) (err error) {
	parent, err := openLegacyRoot(root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, parent.close()) }()
	if err := unix.Mkdirat(parent.fd, cjsiPreflightDirName, 0700); err != nil && !errors.Is(err, unix.EEXIST) {
		return err
	}
	fd, err := unix.Openat(parent.fd, cjsiPreflightDirName, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	dirPath := filepath.Join(root, cjsiPreflightDirName)
	dir := os.NewFile(uintptr(fd), dirPath)
	defer func() { err = errors.Join(err, dir.Close()) }()
	if err := legacyFlock(fd, unix.LOCK_EX); err != nil {
		return err
	}
	var owned unix.Stat_t
	if err := unix.Fstat(fd, &owned); err != nil {
		return err
	}
	if owned.Mode&0777 != 0700 || owned.Uid != uint32(os.Geteuid()) {
		return errCJSILayoutInvalid
	}
	check := func() error {
		var current unix.Stat_t
		err := unix.Fstatat(parent.fd, cjsiPreflightDirName, &current, unix.AT_SYMLINK_NOFOLLOW)
		if err != nil || current.Dev != owned.Dev || current.Ino != owned.Ino || current.Mode != owned.Mode || current.Uid != owned.Uid {
			return errors.Join(errCJSILayoutInvalid, err)
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	if err := removeLegacyPreflightFiles(dir); err != nil {
		return err
	}
	defer func() {
		cleanupErr := removeLegacyPreflightFiles(dir)
		if cleanupErr == nil {
			cleanupErr = check()
		}
		if cleanupErr == nil {
			cleanupErr = unix.Unlinkat(parent.fd, cjsiPreflightDirName, unix.AT_REMOVEDIR)
		}
		err = errors.Join(err, cleanupErr)
	}()
	return run(filepath.Join(dirPath, cjsiPreflightSnapshotName))
}

func removeLegacyPreflightFiles(dir *os.File) error {
	for _, suffix := range [...]string{"-wal", "-shm", "-journal", ""} {
		name := cjsiPreflightSnapshotName + suffix
		file, info, err := openRegularLockEntry(dir, name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		var st unix.Stat_t
		err = unix.Fstat(int(file.Fd()), &st)
		if err == nil {
			if st.Mode&0777 != 0600 || st.Nlink != 1 || st.Uid != uint32(os.Geteuid()) {
				err = errCJSILayoutInvalid
			} else {
				err = unlinkLockEntry(dir, name, info)
			}
		}
		if err := errors.Join(err, file.Close()); err != nil {
			return err
		}
	}
	return nil
}

func legacyIndexPreflightError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if canceled := ctx.Err(); canceled != nil {
		return canceled
	}
	var driverErr *sqlite.Error
	if errors.As(err, &driverErr) {
		code := driverErr.Code()
		if code == sqlite3.SQLITE_CONSTRAINT_CHECK {
			return errors.Join(ErrLegacySessionIndexInvalid, err)
		}
		switch code & 0xff {
		case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
			return errors.Join(ErrStateBusy, err)
		case sqlite3.SQLITE_NOTADB, sqlite3.SQLITE_CORRUPT:
			return errors.Join(ErrLegacySessionIndexInvalid, err)
		}
	}
	return err
}
