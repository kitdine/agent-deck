package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/kitdine/agent-deck/internal/hookrefusal"
	"github.com/kitdine/agent-deck/internal/platform"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// MigrateState installs or resumes the fenced session layout. Call at the
// command boundary before acquiring scan/state locks or opening any Store.
func MigrateState(ctx context.Context, root string) error {
	root, err := platform.GuardStateRoot(root)
	if err != nil {
		return err
	}
	if err := migrateCJSIManagedRoot(ctx, root, lockWait, cjsiStoreLegacyManager{wait: lockWait}); err != nil {
		return migrationOpenError(ctx, root, err)
	}
	_ = hookrefusal.Clear(root)
	return nil
}

// EnsureStateMigrated is the automatic writable-entry gate. Ready roots need
// no locks or drain. Competing installers recheck readiness under both locks
// before either proceeding with the same migration or admitting a new writer.
func EnsureStateMigrated(ctx context.Context, root string) error {
	root, err := platform.GuardStateRoot(root)
	if err != nil {
		return err
	}
	ready, err := cjsiStateInstalled(ctx, root)
	if ready || (err != nil && !errors.Is(err, ErrStateBusy)) {
		return err
	}
	err = migrateCJSIManagedRootWhenNeeded(ctx, root, lockWait, cjsiStoreLegacyManager{wait: lockWait}, cjsiStateInstalled, nil)
	return migrationOpenError(ctx, root, err)
}

func migrationOpenError(ctx context.Context, root string, err error) error {
	if errors.Is(err, ErrStateBusy) {
		if ahead := schemaAheadAtRest(ctx, root); ahead != nil {
			return ahead
		}
	}
	return err
}

func cjsiStateInstalled(ctx context.Context, root string) (ready bool, err error) {
	if err := checkCJSICorePath(root); err != nil {
		return false, err
	}
	path := filepath.Join(root, "agentdeck.sqlite3")
	pin, err := admitLegacyFile(ctx, path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, pin.close()) }()
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	// A competing CLI may be waiting for maintenance. An idle pooled
	// connection in that other process would pin the root through its wait.
	db, err := openLegacySQL(path, legacyURI(path, "mode=ro&_pragma=busy_timeout(5000)"), "", pin.root)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	version, err := cjsiCoreVersionForRouting(ctx, db)
	if err != nil || version < cjsiLayoutCoreSchemaVersion {
		return false, err
	}
	_, err = checkCoreLayoutForOpen(ctx, root, db, version)
	if errors.Is(err, errCJSILayoutPending) || errors.Is(err, errCJSIFenceInvalid) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// A process killed after committing ready may still owe snapshot retirement.
	_, err = os.Lstat(filepath.Join(root, cjsiSnapshotName))
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	return false, err
}

func cjsiCoreVersionForRouting(ctx context.Context, db *sql.DB) (int, error) {
	version, err := schemaVersion(ctx, db)
	if err != nil {
		if fresh, err := schemaMetadataIsFresh(ctx, db); err == nil && fresh {
			return 0, nil
		}
	}
	return version, err
}

// CJSILayoutOpenError identifies a core at the managed version whose layout
// could not be opened. Diagnostics distinguish incomplete installation from
// failures to access the database itself.
type CJSILayoutOpenError struct{ err error }

func (e *CJSILayoutOpenError) Error() string { return e.err.Error() }
func (e *CJSILayoutOpenError) Unwrap() error { return e.err }
func (e *CJSILayoutOpenError) Installing() bool {
	return errors.Is(e.err, errCJSILayoutPending)
}

func checkManagedCoreOpen(ctx context.Context, root string, db *sql.DB) (cjsiLayoutJournal, error) {
	j, err := readManagedCoreLayout(ctx, root, db)
	if err != nil {
		return cjsiLayoutJournal{}, &CJSILayoutOpenError{err}
	}
	return j, nil
}

func checkCoreLayoutForOpen(ctx context.Context, root string, db *sql.DB, version int) (cjsiLayoutJournal, error) {
	if err := checkSchemaAhead(version, CurrentSchemaVersion); err != nil {
		return cjsiLayoutJournal{}, err
	}
	if version < cjsiLayoutCoreSchemaVersion {
		return cjsiLayoutJournal{}, checkNoPrematureCJSIJournal(ctx, db, version)
	}
	return checkManagedCoreOpen(ctx, root, db)
}

// ErrStateRootIdentityMismatch reports a managed layout bound to another
// physical directory, as can happen after copying the state root.
var ErrStateRootIdentityMismatch = &Error{Code: "state_root_identity_mismatch"}

func checkManagedJournalRoot(root string, recorded cjsiFileIdentity) error {
	stat, err := privateServingPath(root, true)
	if err != nil {
		return err
	}
	if cjsiIdentity(stat) == recorded {
		return nil
	}
	// The portable archive deliberately writes device 0, inode 1 until
	// Restore can bind the journal to the target's physical identity.
	if recorded == (cjsiFileIdentity{inode: 1}) {
		return errors.Join(ErrStateRestoreInProgress, errCJSILayoutInvalid)
	}
	return errors.Join(ErrStateRootIdentityMismatch, errCJSILayoutInvalid)
}

func readManagedCoreLayout(ctx context.Context, root string, db *sql.DB) (cjsiLayoutJournal, error) {
	j, err := cjsiReadLayoutJournal(ctx, db)
	if err != nil {
		return cjsiLayoutJournal{}, errors.Join(ErrUnknownSchema, err)
	}
	if err := checkManagedJournalRoot(root, j.root); err != nil {
		return cjsiLayoutJournal{}, err
	}
	if j.phase != cjsiLayoutReady {
		return cjsiLayoutJournal{}, errCJSILayoutPending
	}
	if err := cjsiCheckRecordedFence(root, j.fence); err != nil {
		return cjsiLayoutJournal{}, err
	}
	head, err := cjsiReadIndexHead(ctx, db)
	if err != nil {
		return cjsiLayoutJournal{}, err
	}
	epoch := j.epoch
	// The managed index writer supplies the purge transition. Its journal
	// can retain the previous epoch for cleanup after the head is cleared.
	if j.selection().phase == cjsiLayoutPurged {
		epoch = 0
	}
	if epoch != head.IndexEpoch {
		return cjsiLayoutJournal{}, errCJSILayoutInvalid
	}
	return j, nil
}

// An unrebound portable core must be rejected before a writable opener
// can chmod it, reserve sidecars, or change its journal mode.
func checkManagedCoreBindingBeforeWrite(ctx context.Context, root, path string) (err error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return err
	}
	db, err := sql.Open("sqlite", cjsiSQLiteURI(path, "ro"))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	version, probeErr := schemaVersion(ctx, db)
	if probeErr != nil || version != cjsiLayoutCoreSchemaVersion {
		return ctx.Err() // Preserve normal legacy SQLite recovery.
	}
	j, err := cjsiReadLayoutJournal(ctx, db)
	if err != nil {
		return errors.Join(ErrUnknownSchema, err)
	}
	return checkManagedJournalRoot(root, j.root)
}

// The caller pins root admission across this check and the legacy open, so
// the schema/fence transition cannot race with creating or reading old files.
// A managed index is only exposed by WithCJSIIndexSnapshot, whose selection
// checks and protection overlays cannot be expressed by a raw Store handle.
func checkLegacySessionOpen(ctx context.Context, root string, readOnly bool) error {
	if err := checkCJSICorePath(root); err != nil {
		return err
	}
	db, release, err := acquireReadOnlySQL(ctx, filepath.Join(root, "agentdeck.sqlite3"), "read-only")
	if errors.Is(err, os.ErrNotExist) {
		return cjsiCheckLegacyRuntimeFiles(root)
	}
	if err != nil {
		return legacySessionCoreReadError(root, readOnly, err)
	}
	defer release()
	version, err := cjsiCoreVersionForRouting(ctx, db)
	if err != nil {
		return legacySessionCoreReadError(root, readOnly, err)
	}
	if readOnly && version > CurrentSchemaVersion && cjsiNoManagedSessionFiles(root) {
		var managed bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE lower(name) GLOB 'cjsi_*')`).Scan(&managed); err != nil {
			return err
		}
		if !managed {
			return nil
		}
	}
	j, err := checkCoreLayoutForOpen(ctx, root, db, version)
	if err != nil {
		return err
	}
	if version < cjsiLayoutCoreSchemaVersion {
		return cjsiCheckLegacyRuntimeFiles(root)
	}
	if j.selection().phase == cjsiLayoutNeverBuilt {
		return errors.Join(os.ErrNotExist, ErrCJSIIndexAbsent)
	}
	return ErrCJSIIndexChanged
}

func legacySessionCoreReadError(root string, readOnly bool, err error) error {
	// Preserve the legacy All view over an independent index when the core
	// isn't a SQLite database at all. Never infer legacy from contention,
	// a readable schema with invalid metadata, or any migration residue.
	var driverErr *sqlite.Error
	if !errors.As(err, &driverErr) || driverErr.Code()&0xff != sqlite3.SQLITE_NOTADB {
		return err
	}
	if !cjsiNoManagedSessionFiles(root) {
		return err
	}
	// Older callers use the finite writer even for independent All reads.
	// Preserve an existing index but do not create one around a broken core.
	if !readOnly {
		if _, statErr := privateServingPath(filepath.Join(root, cjsiLegacyIndexName), false); statErr != nil {
			return err
		}
	}
	return nil
}

func cjsiNoManagedSessionFiles(root string) bool {
	if cjsiCheckLegacyRuntimeFiles(root) != nil || cjsiRequireCleanIndexPath(filepath.Join(root, cjsiPrivateIndexName)) != nil {
		return false
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".cjsi-") {
			return false
		}
	}
	return true
}
