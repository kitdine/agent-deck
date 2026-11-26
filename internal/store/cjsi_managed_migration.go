package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
)

// A root manager may issue this lease only after excluding new legacy opens
// and draining every managed writer, reader and idle SQLite connection. The
// exclusion lasts through release.
type cjsiLegacyManager interface {
	drainLegacy(context.Context, string) (release func() error, err error)
}

func migrateCJSIManagedRoot(ctx context.Context, root string, wait time.Duration, manager cjsiLegacyManager) error {
	return migrateCJSIManagedRootWithReady(ctx, root, wait, manager, nil)
}

// beforeReady runs after verifying the installed fence and index, immediately
// before the ready journal and initial head commit. Production passes nil.
func migrateCJSIManagedRootWithReady(ctx context.Context, root string, wait time.Duration, manager cjsiLegacyManager, beforeReady func(context.Context) error) error {
	return migrateCJSIManagedRootWhenNeeded(ctx, root, wait, manager, nil, beforeReady)
}

func migrateCJSIManagedRootWhenNeeded(ctx context.Context, root string, wait time.Duration, manager cjsiLegacyManager, installed func(context.Context, string) (bool, error), beforeReady func(context.Context) error) error {
	return withCJSIManagedCoreIfNeeded(ctx, root, wait, manager, installed, func(root string, core *sql.DB) error {
		return installCJSIState(ctx, root, core, beforeReady)
	})
}

func installCJSIState(ctx context.Context, root string, core *sql.DB, beforeReady func(context.Context) error) error {
	if err := migrateCJSICore(ctx, root, core); err != nil {
		return err
	}
	// A version stamp from an incomplete pre-release layout is not a C2
	// selection. Never retire a recovery snapshot on its authority.
	if _, err := cjsiReadIndexHead(ctx, core); err != nil {
		return err
	}
	journal, err := cjsiReadLayoutJournal(ctx, core)
	if err != nil {
		return err
	}
	if journal.phase == cjsiFenceInstalling {
		// Capture may resume in a later invocation than schema preflight.
		if err := normalizeLegacySQLitePermissions(filepath.Join(root, cjsiLegacyIndexName)); err != nil {
			return err
		}
	}
	return resumeCJSILayoutWithReady(ctx, root, core, func(ctx context.Context, prior, next cjsiLayoutJournal) error {
		if beforeReady != nil {
			if err := beforeReady(ctx); err != nil {
				return err
			}
		}
		return cjsiCommitInitialHead(ctx, root, core, prior, next)
	})
}

func validateCJSIMigrationRegistry(currentVersion int, registry []migration) error {
	_, err := validatedCJSISchemaReferences(context.Background(), currentVersion, registry)
	return err
}

func migrateCJSICore(ctx context.Context, physicalRoot string, core *sql.DB) error {
	// Validate the registry and generate the authoritative schema-36 reference
	// from an empty in-memory SQLite database before touching the target core.
	refs, err := validatedCJSISchemaReferences(ctx, CurrentSchemaVersion, migrations)
	if err != nil {
		return err
	}
	step37 := cjsiCoreMigration37(func(ctx context.Context, tx *sql.Tx) error {
		if err := verifyCJSICoreSchema(ctx, tx, refs.quota36); err != nil {
			return err
		}
		if err := prepareLegacySessionIndexForMigration(ctx, physicalRoot); err != nil {
			return err
		}
		return initializeCJSILayoutCore(ctx, tx, physicalRoot)
	})
	if err := migrateThrough(ctx, core, append(slices.Clip(migrations), step37), cjsiLayoutCoreSchemaVersion); err != nil {
		return err
	}
	return verifyCJSICoreSchema(ctx, core, refs.layout37)
}

// The in-memory reference uses the same migration with a schema-only journal.
// Only the managed, on-disk journal needs root identity and durability checks.
func cjsiCoreMigration37(installJournal func(context.Context, *sql.Tx) error) migration {
	return migration{
		version: cjsiLayoutCoreSchemaVersion,
		apply: func(ctx context.Context, tx *sql.Tx) error {
			if err := installJournal(ctx, tx); err != nil {
				return err
			}
			// Usage-only replay stages here without the session index, which a
			// never-built root lacks. The workset, publication head, core facts,
			// interval dirty stamps, receipts, and legacy trigger removal commit
			// atomically with this schema version.
			_, err := tx.ExecContext(ctx, cjsiCheckpointWorksetSchema+cjsiPublicationHeadSchema+cjsiCoreFactsSchema)
			return err
		},
	}
}

func withCJSIManagedCore(ctx context.Context, root string, wait time.Duration, manager cjsiLegacyManager, run func(string, *sql.DB) error) error {
	return withCJSIManagedCoreIfNeeded(ctx, root, wait, manager, nil, run)
}

func withCJSIManagedCoreIfNeeded(ctx context.Context, root string, wait time.Duration, manager cjsiLegacyManager, installed func(context.Context, string) (bool, error), run func(string, *sql.DB) error) (err error) {
	if manager == nil {
		return ErrStateBusy
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err = platform.GuardStateRoot(root)
	if err != nil {
		return err
	}
	if err := checkCJSICorePath(root); err != nil {
		return err
	}
	if err := platform.EnsureStateRoot(root); err != nil {
		return err
	}
	stat, err := privateServingPath(root, true)
	if err != nil {
		return err
	}
	// AcquireScanLock is scanruntime.AcquireMaintenance's actual lock.
	maintenance, err := AcquireScanLock(ctx, root, wait)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, maintenance.Release()) }()
	state, err := AcquireLock(ctx, root, wait)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, state.Release()) }()
	if err := checkCJSICorePath(root); err != nil {
		return err
	}
	if installed != nil {
		// Another process may have finished while we were waiting. It is
		// safe for its first writer to retain the ready core handle.
		ready, err := installed(ctx, root)
		if ready || err != nil {
			return err
		}
	}
	release, err := manager.drainLegacy(ctx, root)
	if err != nil {
		return err
	}
	if release == nil {
		return ErrStateBusy
	}
	defer func() { err = errors.Join(err, release()) }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := cjsiCheckInstallRoot(root, cjsiIdentity(stat)); err != nil {
		return err
	}

	path := filepath.Join(root, "agentdeck.sqlite3")
	if err := checkManagedCoreBindingBeforeWrite(ctx, root, path); err != nil {
		return err
	}
	if err := prepareCJSIManagedCore(path); err != nil {
		return err
	}
	// Use a single busy_timeout pragma (writablePragmas = 5000ms) plus FULL durability.
	db, err := openWritableSQL(legacyURI(path, "mode=rw&"+writablePragmas+"&_pragma=synchronous(FULL)"), "core")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
		return err
	}
	if journalMode != "wal" {
		return errCJSILayoutInvalid
	}
	return run(root, db)
}

// Validate surviving SQLite files before opening them. A missing main with
// surviving sidecars is an unfinished recovery, not a fresh database.
func prepareCJSIManagedCore(path string) error {
	_, err := ownedLegacySQLiteInfo(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := cjsiRequireAbsentSidecars(path); err != nil {
			return err
		}
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, platform.FileMode)
		if err != nil {
			return err
		}
		return file.Close()
	}
	if err != nil {
		return err
	}
	return normalizeLegacySQLitePermissions(path)
}
