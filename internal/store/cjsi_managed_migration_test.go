package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// This capsule owns every opener of its generated root. Controlled clients
// hold shared admission for their handle lifetime; kernel exit drops it.
type cjsiMigrationCapsule struct {
	root string
	gate string
}

func newCJSIMigrationCapsule(t *testing.T) cjsiMigrationCapsule {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return cjsiMigrationCapsule{root, filepath.Join(t.TempDir(), "admission")}
}

func (c cjsiMigrationCapsule) admit(op int) (*os.File, error) {
	f, err := os.OpenFile(c.gate, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), op|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrStateBusy
		}
		return nil, err
	}
	return f, nil
}

func (c cjsiMigrationCapsule) drainLegacy(ctx context.Context, root string) (func() error, error) {
	if root != c.root {
		return nil, errCJSILayoutInvalid
	}
	f, err := c.admit(unix.LOCK_EX)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"agentdeck.sqlite3", cjsiLegacyIndexName} {
		if err := closeCJSIInstallReadPool(filepath.Join(root, name)); err != nil {
			return nil, errors.Join(err, f.Close())
		}
	}
	return f.Close, nil
}

func (c cjsiMigrationCapsule) openCore(t *testing.T, mode string) *sql.DB {
	t.Helper()
	return openCJSIFixtureDB(t, filepath.Join(c.root, "agentdeck.sqlite3"), mode)
}

func (c cjsiMigrationCapsule) seed35(t *testing.T) {
	t.Helper()
	createCJSIPrivateFixture(t, filepath.Join(c.root, "agentdeck.sqlite3"))
	db := c.openCore(t, "rw")
	defer db.Close()
	if err := migrate(t.Context(), db, migrations[:35]); err != nil {
		t.Fatal(err)
	}
	assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", 35)
	cjsiFixtureExec(t, db, `INSERT INTO quota_alert_notices(client,window_key,kind,instance_unix,notified_at)
		VALUES('codex','window','test',123,'retained')`)
}

func (c cjsiMigrationCapsule) seed36(t *testing.T) {
	t.Helper()
	createCJSIPrivateFixture(t, filepath.Join(c.root, "agentdeck.sqlite3"))
	db := c.openCore(t, "rw")
	defer db.Close()
	if err := migrate(t.Context(), db, migrations[:36]); err != nil {
		t.Fatal(err)
	}
	assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", 36)
	cjsiFixtureExec(t, db, `INSERT INTO quota_alert_notices_v2(client,window_key,kind,instance_unix,notified_at,context_generation)
		VALUES('codex','window','test',123,'retained',0)`)
}

func assertCJSIMigrationNumber(t *testing.T, db *sql.DB, query string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRowContext(t.Context(), query).Scan(&got); err != nil || got != want {
		t.Fatalf("%s = %d, want %d: %v", query, got, want, err)
	}
}

// Abort the stamp write after all step-37 DDL. The connection-local trigger
// disappears on close and leaves the on-disk schema valid for retry.
func abortCJSIVersion37Write(t *testing.T, c cjsiMigrationCapsule) {
	t.Helper()
	ctx := t.Context()
	err := withCJSIManagedCore(ctx, c.root, 0, c, func(root string, core *sql.DB) error {
		core.SetMaxOpenConns(1)
		if _, err := core.ExecContext(ctx, fmt.Sprintf(`CREATE TEMP TRIGGER abort_cjsi
			BEFORE UPDATE OF version ON main.schema_metadata WHEN NEW.version=%d
			BEGIN SELECT RAISE(ABORT,'capsule rollback'); END`, cjsiLayoutCoreSchemaVersion)); err != nil {
			return err
		}
		return migrateCJSICore(ctx, root, core)
	})
	if err == nil || !strings.Contains(err.Error(), "capsule rollback") {
		t.Fatalf("expected injected failure writing version 37, got %v", err)
	}
}

func TestCJSIManagedMigration(t *testing.T) {
	t.Run("cold through physical alias", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		alias := filepath.Join(t.TempDir(), "state-alias")
		if err := os.Symlink(c.root, alias); err != nil {
			t.Fatal(err)
		}
		var observedBusy int
		if err := withCJSIManagedCore(t.Context(), alias, 0, c, func(root string, core *sql.DB) error {
			if err := core.QueryRowContext(t.Context(), "PRAGMA busy_timeout").Scan(&observedBusy); err != nil {
				return err
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if observedBusy != 5000 {
			t.Fatalf("managed core busy_timeout = %d, want 5000", observedBusy)
		}
		if err := migrateCJSIManagedRoot(t.Context(), alias, 0, c); err != nil {
			t.Fatal(err)
		}
		db := c.openCore(t, "ro")
		defer db.Close()
		var journalMode string
		if err := db.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&journalMode); err != nil || journalMode != "wal" {
			t.Fatalf("managed cold core journal mode = %q, want wal: %v", journalMode, err)
		}
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('quota_windows_v2','quota_envelopes_v2','quota_alert_notices_v2','quota_identities')", 4)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('quota_windows','quota_envelopes','quota_alert_notices')", 0)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='telemetry_jsonl_retracts_otel'", 0)
		j, err := (cjsiSQLiteLayoutJournal{db}).load(t.Context())
		stat, statErr := privateServingPath(c.root, true)
		if err != nil || statErr != nil || j.root != cjsiIdentity(stat) || j.selection().phase != cjsiLayoutNeverBuilt {
			t.Fatalf("cold layout %+v: %v / %v", j, err, statErr)
		}
		if _, err := validateCJSIFence(c.root); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{cjsiPrivateIndexName, cjsiSnapshotName, cjsiPreparedName} {
			if err := cjsiRequireCleanIndexPath(filepath.Join(c.root, name)); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("schema35 upgrade and atomic rollback", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		c.seed35(t)
		db := c.openCore(t, "rw")
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", 35)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices WHERE instance_unix=123 AND notified_at='retained'", 1)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='quota_alert_notices_v2'", 0)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		abortCJSIVersion37Write(t, c)
		db = c.openCore(t, "rw")
		// Step 36 committed its quota_v2 tables; step 37 rolled back atomically.
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiBaseCoreSchemaVersion)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices_v2 WHERE instance_unix=123 AND notified_at='retained'", 1)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='quota_alert_notices'", 0)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='telemetry_jsonl_retracts_otel'", 1)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 0)
		if err := cjsiRequireCleanIndexPath(filepath.Join(c.root, cjsiLegacyIndexName)); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		db = c.openCore(t, "ro")
		defer db.Close()
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices_v2 WHERE instance_unix=123 AND notified_at='retained'", 1)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='telemetry_jsonl_retracts_otel'", 0)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 1)
	})

	t.Run("schema36 upgrade and pre-release collision rejection", func(t *testing.T) {
		ctx := t.Context()
		// Real quota 36 root upgrades cleanly to 37 and preserves v2 data.
		c := newCJSIMigrationCapsule(t)
		c.seed36(t)
		if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		db := c.openCore(t, "ro")
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices_v2 WHERE instance_unix=123 AND notified_at='retained'", 1)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='telemetry_jsonl_retracts_otel'", 0)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}

		// Colliding pre-release 36 root: stamped version=36 with cjsi_layout_journal
		// but missing quota_windows_v2 (legacy quota tables still present).
		colliding := newCJSIMigrationCapsule(t)
		colliding.seed35(t)
		cdb := colliding.openCore(t, "rw")
		stat, err := privateServingPath(colliding.root, true)
		if err != nil {
			t.Fatal(err)
		}
		tx := beginCJSIFixtureTx(t, cdb)
		if _, err := tx.ExecContext(ctx, cjsiLayoutJournalSchema); err != nil {
			t.Fatal(err)
		}
		if err := initializeCJSILayoutJournal(ctx, tx, cjsiIdentity(stat)); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE schema_metadata SET version=36"); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if got, err := locateCJSIWithCore(t, colliding.root, cdb); !errors.Is(err, errCJSILayoutInvalid) || got != (cjsiSessionLocation{}) {
			t.Fatalf("locator accepted colliding pre-release 36 root: %+v, %v", got, err)
		}
		if err := cdb.Close(); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(ctx, colliding.root, 0, colliding); !errors.Is(err, ErrUnknownSchema) {
			t.Fatalf("managed migration accepted colliding pre-release 36 root: %v", err)
		}
		for _, tc := range []struct {
			name string
			open func(context.Context, string) (*Store, error)
		}{
			{"Open", Open},
			{"OpenExisting", OpenExisting},
			{"OpenWithLockHeld", OpenWithLockHeld},
			{"OpenReadOnly", OpenReadOnly},
		} {
			s, err := tc.open(ctx, colliding.root)
			if s != nil {
				_ = s.Close()
			}
			if !errors.Is(err, ErrUnknownSchema) {
				t.Fatalf("%s accepted colliding pre-release 36 root: %v", tc.name, err)
			}
		}
		cdb = colliding.openCore(t, "ro")
		assertCJSIMigrationNumber(t, cdb, "SELECT version FROM schema_metadata", 36)
		assertCJSIMigrationNumber(t, cdb, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='quota_windows_v2'", 0)
		if err := cdb.Close(); err != nil {
			t.Fatal(err)
		}
		if err := cjsiRequireCleanIndexPath(filepath.Join(colliding.root, cjsiLegacyIndexName)); err != nil {
			t.Fatal(err)
		}

		// Forged version=36 without journal AND without quota_windows_v2 must also fail.
		forged := newCJSIMigrationCapsule(t)
		forged.seed35(t)
		fdb := forged.openCore(t, "rw")
		cjsiFixtureExec(t, fdb, "UPDATE schema_metadata SET version=36")
		if err := fdb.Close(); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(ctx, forged.root, 0, forged); !errors.Is(err, ErrUnknownSchema) {
			t.Fatalf("managed migration accepted forged 36 root without quota_windows_v2: %v", err)
		}

		// Registry verification must reject mismatched ceiling, truncated sequence, or fake v36.
		if err := validateCJSIMigrationRegistry(35, migrations[:35]); !errors.Is(err, ErrUnknownSchema) {
			t.Fatalf("accepted mismatched ceiling 35: %v", err)
		}
		if err := validateCJSIMigrationRegistry(CurrentSchemaVersion, migrations[:35]); !errors.Is(err, ErrUnknownSchema) {
			t.Fatalf("accepted truncated registry: %v", err)
		}
		fake := append([]migration(nil), migrations...)
		fake[len(fake)-1].apply = nil
		if err := validateCJSIMigrationRegistry(CurrentSchemaVersion, fake); !errors.Is(err, ErrUnknownSchema) {
			t.Fatalf("accepted v36 without apply callback: %v", err)
		}
	})

	t.Run("ordinary open leaves installation to the managed entry", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		c.seed35(t)
		core, err := Open(t.Context(), c.root)
		if err != nil {
			t.Fatal(err)
		}
		assertCJSIMigrationNumber(t, core.DB, "SELECT version FROM schema_metadata", cjsiBaseCoreSchemaVersion)
		assertCJSIMigrationNumber(t, core.DB, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 0)
		if err := core.Close(); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		core, err = Open(t.Context(), c.root)
		if err != nil {
			t.Fatal(err)
		}
		assertCJSIMigrationNumber(t, core.DB, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion)
		if err := core.Close(); err != nil {
			t.Fatal(err)
		}
		state, err := AcquireLock(t.Context(), c.root, 0)
		if err != nil {
			t.Fatal(err)
		}
		store, openErr := OpenWithLockHeld(t.Context(), c.root)
		if err := state.Release(); err != nil {
			t.Fatal(err)
		}
		if openErr != nil {
			t.Fatalf("state-lock-only open: %v", openErr)
		}
		assertCJSIMigrationNumber(t, store.DB, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion)
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		db := c.openCore(t, "rw")
		cjsiFixtureExec(t, db, "UPDATE schema_metadata SET version=?", cjsiLayoutCoreSchemaVersion+1)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		err = migrateCJSIManagedRoot(t.Context(), c.root, 0, c)
		var ahead *SchemaAhead
		if !errors.As(err, &ahead) || ahead.Stored != cjsiLayoutCoreSchemaVersion+1 || ahead.Supported != cjsiLayoutCoreSchemaVersion {
			t.Fatalf("managed open: %v", err)
		}
		db = c.openCore(t, "ro")
		defer db.Close()
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion+1)
	})

	t.Run("exclusion spans resume and handle closure", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		ctx := t.Context()
		var observedReady bool
		manager := cjsiCapsuleHandoff(func(ctx context.Context, root string) (func() error, error) {
			release, err := c.drainLegacy(ctx, root)
			if err != nil {
				return nil, err
			}
			return func() error {
				defer release()
				// At this point the capsule has admitted no other holders. The
				// core WAL sidecars must be gone before validating Ready.
				for _, suffix := range []string{"-wal", "-shm"} {
					path := filepath.Join(root, "agentdeck.sqlite3"+suffix)
					if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
						t.Errorf("core sidecar %s remains at drain release: %v", suffix, err)
					}
				}
				for _, acquire := range []func(context.Context, string, time.Duration) (*Lock, error){AcquireScanLock, AcquireLock} {
					lock, err := acquire(ctx, root, 0)
					if err == nil {
						_ = lock.Release()
						t.Fatal("lock released before drain handoff")
					}
					if !errors.Is(err, ErrStateBusy) {
						t.Fatal(err)
					}
				}
				if lease, err := c.admit(unix.LOCK_SH); !errors.Is(err, ErrStateBusy) {
					if lease != nil {
						_ = lease.Close()
					}
					t.Fatalf("new legacy admission before handoff: %v", err)
				}
				db := c.openCore(t, "ro")
				defer db.Close()
				j, err := (cjsiSQLiteLayoutJournal{db}).load(ctx)
				observedReady = err == nil && j.selection().phase == cjsiLayoutNeverBuilt
				return err
			}, nil
		})
		if err := migrateCJSIManagedRoot(ctx, c.root, 0, manager); err != nil || !observedReady {
			t.Fatalf("release before layout-ready: ready=%v, err=%v", observedReady, err)
		}
	})

	t.Run("missing baseline prerequisite", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		core, err := Open(t.Context(), c.root)
		if err != nil {
			t.Fatal(err)
		}
		cjsiFixtureExec(t, core.DB, "DELETE FROM derived_snapshot_generation")
		if err := core.Close(); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); !errors.Is(err, ErrUnknownSchema) {
			t.Fatalf("incomplete baseline prerequisite: %v", err)
		}
		db := c.openCore(t, "ro")
		defer db.Close()
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiBaseCoreSchemaVersion)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 0)
	})
}

type cjsiCapsuleHandoff func(context.Context, string) (func() error, error)

func (h cjsiCapsuleHandoff) drainLegacy(ctx context.Context, root string) (func() error, error) {
	return h(ctx, root)
}

func TestCJSIManagedMigrationBarriers(t *testing.T) {
	for _, reason := range []string{"scan", "state", "live writer", "active reader", "no issuer", "unfinished", "cancel at handoff"} {
		t.Run(reason, func(t *testing.T) {
			c := newCJSIMigrationCapsule(t)
			c.seed35(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var manager cjsiLegacyManager = c
			switch reason {
			case "scan", "state":
				acquire := AcquireScanLock
				if reason == "state" {
					acquire = AcquireLock
				}
				lock, err := acquire(ctx, c.root, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer lock.Release()
			case "live writer":
				lease, err := c.admit(unix.LOCK_SH)
				if err != nil {
					t.Fatal(err)
				}
				defer lease.Close()
				legacy, err := OpenSessions(ctx, c.root)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = legacy.Close(); _ = closeCJSIInstallReadPool(legacy.path) }()
			case "active reader":
				reader, err := OpenReadOnly(ctx, c.root)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = reader.Close(); _ = closeCJSIInstallReadPool(reader.path) }()
			case "no issuer":
				manager = nil
			case "unfinished":
				manager = cjsiCapsuleHandoff(func(context.Context, string) (func() error, error) { return nil, nil })
			case "cancel at handoff":
				manager = cjsiCapsuleHandoff(func(ctx context.Context, root string) (func() error, error) {
					release, err := c.drainLegacy(ctx, root)
					cancel()
					return release, err
				})
			}
			err := migrateCJSIManagedRoot(ctx, c.root, 0, manager)
			if reason == "cancel at handoff" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("handoff: %v", err)
				}
			} else if !errors.Is(err, ErrStateBusy) {
				t.Fatalf("barrier: %v", err)
			}
			db := c.openCore(t, "ro")
			defer db.Close()
			assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", 35)
			assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices WHERE instance_unix=123 AND notified_at='retained'", 1)
			assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='quota_alert_notices_v2'", 0)
			assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 0)
			if err := cjsiRequireCleanIndexPath(filepath.Join(c.root, cjsiPrivateIndexName)); err != nil {
				t.Fatal(err)
			}
			if reason != "live writer" {
				if err := cjsiRequireCleanIndexPath(filepath.Join(c.root, cjsiLegacyIndexName)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// A process dies after the real migration runner commits and before the first
// resume step. No fixture synthesizes the journal.
func TestCJSIManagedMigrationCrashRetry(t *testing.T) {
	if root := os.Getenv("AGENTDECK_CJSI_CAPSULE_CRASH_ROOT"); root != "" {
		c := cjsiMigrationCapsule{root, os.Getenv("AGENTDECK_CJSI_CAPSULE_GATE")}
		err := withCJSIManagedCore(t.Context(), root, 0, c, func(physical string, db *sql.DB) error {
			if err := migrateCJSICore(t.Context(), physical, db); err != nil {
				return err
			}
			os.Exit(79)
			return nil
		})
		t.Fatalf("crash child returned: %v", err)
	}
	c := newCJSIMigrationCapsule(t)
	c.seed35(t)
	lease, err := c.admit(unix.LOCK_SH)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := OpenSessions(t.Context(), c.root)
	if err != nil {
		t.Fatal(err)
	}
	cjsiFixtureExec(t, legacy.DB, "INSERT INTO session_exclusions VALUES('kind','retained')")
	if err := errors.Join(legacy.Close(), closeCJSIInstallReadPool(legacy.path), lease.Close()); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCJSIManagedMigrationCrashRetry$")
	cmd.Env = append(os.Environ(), "AGENTDECK_CJSI_CAPSULE_CRASH_ROOT="+c.root, "AGENTDECK_CJSI_CAPSULE_GATE="+c.gate)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 79 {
		t.Fatalf("crash child: %v %s", err, out)
	}
	db := c.openCore(t, "ro")
	assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion)
	assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices_v2 WHERE instance_unix=123 AND notified_at='retained'", 1)
	assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='quota_alert_notices'", 0)
	stat, err := privateServingPath(c.root, true)
	if err != nil {
		t.Fatal(err)
	}
	j, err := (cjsiSQLiteLayoutJournal{db}).load(t.Context())
	if want := (cjsiLayoutJournal{phase: cjsiFenceInstalling, root: cjsiIdentity(stat)}); err != nil || j != want {
		t.Fatalf("committed intent %+v, want %+v: %v", j, want, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{cjsiPrivateIndexName, cjsiSnapshotName, cjsiPreparedName} {
		if err := cjsiRequireCleanIndexPath(filepath.Join(c.root, name)); err != nil {
			t.Fatal(err)
		}
	}
	old := openCJSIFixtureDB(t, filepath.Join(c.root, cjsiLegacyIndexName), "ro")
	assertCJSIMigrationNumber(t, old, "SELECT count(*) FROM session_exclusions WHERE value='retained'", 1)
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	var installed cjsiLayoutJournal
	for attempt := range 2 {
		if err := migrateCJSIManagedRoot(t.Context(), c.root, time.Second, c); err != nil {
			t.Fatal(err)
		}
		db = c.openCore(t, "ro")
		j, err = (cjsiSQLiteLayoutJournal{db}).load(t.Context())
		if err != nil || j.selection().phase != cjsiLayoutSelected || (attempt > 0 && installed != j) {
			t.Fatalf("resumed layout %+v: %v", j, err)
		}
		installed = j
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
	index := openCJSIFixtureDB(t, filepath.Join(c.root, cjsiPrivateIndexName), "ro")
	defer index.Close()
	assertCJSIMigrationNumber(t, index, "SELECT count(*) FROM session_exclusions WHERE value='retained'", 1)
}

func assertCJSIV36RejectedCleanly(t *testing.T, c cjsiMigrationCapsule, wantVersion int, err error) {
	t.Helper()
	if !errors.Is(err, ErrUnknownSchema) {
		t.Fatalf("managed migration accepted heterogeneous schema 36: err=%v, want ErrUnknownSchema", err)
	}
	if wantVersion > 0 {
		db := c.openCore(t, "ro")
		defer db.Close()
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", wantVersion)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 0)
	} else {
		db := c.openCore(t, "ro")
		defer db.Close()
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 0)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='schema_metadata'", 0)
	}
	if _, statErr := os.Lstat(filepath.Join(c.root, cjsiLegacyIndexName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("rejected root left legacy fence %s: %v", cjsiLegacyIndexName, statErr)
	}
	for _, name := range []string{cjsiLegacyIndexName, cjsiPrivateIndexName, cjsiSnapshotName, cjsiPreparedName} {
		if err := cjsiRequireCleanIndexPath(filepath.Join(c.root, name)); err != nil {
			t.Fatalf("rejected root left %s: %v", name, err)
		}
	}
}

// rebuildCJSITableWithDDL surgically rewrites one table's CREATE TABLE DDL while
// preserving existing rows and re-installing all triggers attached to the table,
// so each heterogeneous schema-36 fixture isolates its specific structural defect.
func rebuildCJSITableWithDDL(t *testing.T, db *sql.DB, table string, mutateDDL func(ddl string) string) {
	t.Helper()
	var ddl string
	if err := db.QueryRowContext(t.Context(), `SELECT sql FROM main.sqlite_master WHERE type='table' AND name=?`, table).Scan(&ddl); err != nil {
		t.Fatalf("read DDL for %s: %v", table, err)
	}
	mutated := mutateDDL(ddl)
	if mutated == ddl {
		t.Fatalf("DDL mutation for %s produced no change:\n%s", table, ddl)
	}

	rows, err := db.QueryContext(t.Context(), `SELECT sql FROM main.sqlite_master WHERE type='trigger' AND tbl_name=? ORDER BY name`, table)
	if err != nil {
		t.Fatalf("list triggers for %s: %v", table, err)
	}
	var triggers []string
	for rows.Next() {
		var trigSQL string
		if err := rows.Scan(&trigSQL); err != nil {
			rows.Close()
			t.Fatalf("scan trigger for %s: %v", table, err)
		}
		triggers = append(triggers, trigSQL)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		t.Fatalf("close trigger rows for %s: %v", table, err)
	}

	backup := "fixture_backup_" + table
	cjsiFixtureExec(t, db, "PRAGMA foreign_keys = OFF")
	defer cjsiFixtureExec(t, db, "PRAGMA foreign_keys = ON")
	cjsiFixtureExec(t, db, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", table, backup))
	cjsiFixtureExec(t, db, mutated)
	cjsiFixtureExec(t, db, fmt.Sprintf("INSERT INTO %s SELECT * FROM %s", table, backup))
	cjsiFixtureExec(t, db, fmt.Sprintf("DROP TABLE %s", backup))
	for _, trigSQL := range triggers {
		cjsiFixtureExec(t, db, trigSQL)
	}
}

func explainQuotaWindowUpsert(db *sql.DB) error {
	rows, err := db.Query(`EXPLAIN INSERT INTO quota_windows_v2(client, account_id, window_key, source, observed_at, used_percent, resets_at)
		VALUES('codex','','w','s','now',0,'later')
		ON CONFLICT(client,account_id,window_key) DO UPDATE SET source=excluded.source`)
	if err != nil {
		return err
	}
	return rows.Close()
}

func TestCJSIQuota36PrerequisiteValidation(t *testing.T) {
	ctx := t.Context()

	t.Run("extra schema objects", func(t *testing.T) {
		for _, tc := range []struct{ name, ddl string }{
			{"selection insert trigger", `CREATE TRIGGER fixture_selection_insert AFTER INSERT ON provider_selections
				BEGIN DELETE FROM provider_selections WHERE id=NEW.id; END`},
			{"version update trigger", `CREATE TRIGGER fixture_version_update BEFORE UPDATE OF version ON schema_metadata
				WHEN NEW.version>37 BEGIN SELECT RAISE(ABORT,'unexpected version'); END`},
			{"foreign key child", `CREATE TABLE fixture_selection_pin(selection_id INTEGER REFERENCES provider_selections(id))`},
			{"view", `CREATE VIEW fixture_quota AS SELECT client FROM quota_identities`},
			{"index", `CREATE INDEX fixture_version ON schema_metadata(version)`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				c := newCJSIMigrationCapsule(t)
				c.seed36(t)
				db := c.openCore(t, "rw")
				cjsiFixtureExec(t, db, tc.ddl)
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				err := migrateCJSIManagedRoot(ctx, c.root, 0, c)
				assertCJSIV36RejectedCleanly(t, c, cjsiBaseCoreSchemaVersion, err)
			})
		}
	})

	t.Run("recheck version 37 object set", func(t *testing.T) {
		for _, tc := range []struct{ name, ddl string }{
			{"extra view", `CREATE VIEW fixture_layout AS SELECT phase FROM cjsi_layout_journal`},
			{"restored v36 trigger", `CREATE TRIGGER telemetry_jsonl_retracts_otel AFTER INSERT ON schema_metadata BEGIN SELECT 1; END`},
			{"missing v37 table", `DROP TABLE cjsi_receipt_markers`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				c := newCJSIMigrationCapsule(t)
				if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
					t.Fatal(err)
				}
				db := c.openCore(t, "rw")
				cjsiFixtureExec(t, db, tc.ddl)
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); !errors.Is(err, ErrUnknownSchema) {
					t.Fatalf("recheck accepted altered schema 37: err=%v, want ErrUnknownSchema", err)
				}
			})
		}
	})

	t.Run("missing column", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			mutate string
		}{
			{
				name:   "quota_windows_v2 missing context_generation",
				mutate: "ALTER TABLE quota_windows_v2 DROP COLUMN context_generation",
			},
			{
				name:   "quota_envelopes_v2 missing context_generation",
				mutate: "ALTER TABLE quota_envelopes_v2 DROP COLUMN context_generation",
			},
			{
				name:   "quota_identities missing association_cycle_seq",
				mutate: "ALTER TABLE quota_identities DROP COLUMN association_cycle_seq",
			},
			{
				name: "quota_alert_notices_v2 missing context_generation",
				mutate: `CREATE TABLE quota_alert_notices_v2_bad (
					client TEXT NOT NULL,
					window_key TEXT NOT NULL,
					kind TEXT NOT NULL,
					threshold REAL NOT NULL DEFAULT 0,
					instance_unix INTEGER NOT NULL,
					notified_at TEXT NOT NULL,
					PRIMARY KEY (client, window_key, kind, threshold, instance_unix)
				);
				DROP TABLE quota_alert_notices_v2;
				ALTER TABLE quota_alert_notices_v2_bad RENAME TO quota_alert_notices_v2`,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				c := newCJSIMigrationCapsule(t)
				c.seed36(t)
				db := c.openCore(t, "rw")
				cjsiFixtureExec(t, db, tc.mutate)
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				err := migrateCJSIManagedRoot(ctx, c.root, 0, c)
				assertCJSIV36RejectedCleanly(t, c, cjsiBaseCoreSchemaVersion, err)
			})
		}
	})

	t.Run("missing composite primary key", func(t *testing.T) {
		t.Run("quota_windows_v2 missing composite PK", func(t *testing.T) {
			c := newCJSIMigrationCapsule(t)
			c.seed36(t)
			db := c.openCore(t, "rw")
			if err := explainQuotaWindowUpsert(db); err != nil {
				t.Fatalf("genuine seed36 UPSERT explain failed: %v", err)
			}
			rebuildCJSITableWithDDL(t, db, "quota_windows_v2", func(ddl string) string {
				key := "PRIMARY KEY (client, account_id, window_key)"
				idx := strings.Index(ddl, key)
				if idx < 0 {
					t.Fatalf("unexpected quota_windows_v2 DDL: %s", ddl)
				}
				comma := strings.LastIndex(ddl[:idx], ",")
				if comma < 0 {
					t.Fatalf("missing comma before PK in DDL: %s", ddl)
				}
				return ddl[:comma] + ddl[idx+len(key):]
			})
			if err := explainQuotaWindowUpsert(db); err == nil || !strings.Contains(err.Error(), "ON CONFLICT") {
				t.Fatalf("fixture did not break UPSERT conflict target: %v", err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			err := migrateCJSIManagedRoot(ctx, c.root, 0, c)
			assertCJSIV36RejectedCleanly(t, c, cjsiBaseCoreSchemaVersion, err)
		})

		t.Run("quota_alert_notices_v2 truncated composite PK", func(t *testing.T) {
			c := newCJSIMigrationCapsule(t)
			c.seed36(t)
			db := c.openCore(t, "rw")
			rebuildCJSITableWithDDL(t, db, "quota_alert_notices_v2", func(ddl string) string {
				return strings.Replace(
					ddl,
					"PRIMARY KEY (client, window_key, kind, threshold, instance_unix, context_generation)",
					"PRIMARY KEY (client, window_key, kind, threshold, instance_unix)",
					1,
				)
			})
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			err := migrateCJSIManagedRoot(ctx, c.root, 0, c)
			assertCJSIV36RejectedCleanly(t, c, cjsiBaseCoreSchemaVersion, err)
		})
	})

	t.Run("missing unique index", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			mutate func(t *testing.T, db *sql.DB)
		}{
			{
				name: "dropped usage_session_observations_delivery unique index",
				mutate: func(t *testing.T, db *sql.DB) {
					cjsiFixtureExec(t, db, "DROP INDEX usage_session_observations_delivery")
				},
			},
			{
				name: "downgraded provider_selections_completed_operation to non-unique index",
				mutate: func(t *testing.T, db *sql.DB) {
					cjsiFixtureExec(t, db, `DROP INDEX provider_selections_completed_operation;
						CREATE INDEX provider_selections_completed_operation ON provider_selections(operation_id) WHERE operation_id IS NOT NULL`)
				},
			},
			{
				name: "quota_envelopes_v2 missing client primary key unique autoindex",
				mutate: func(t *testing.T, db *sql.DB) {
					rebuildCJSITableWithDDL(t, db, "quota_envelopes_v2", func(ddl string) string {
						return strings.Replace(ddl, "client TEXT PRIMARY KEY", "client TEXT NOT NULL", 1)
					})
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				c := newCJSIMigrationCapsule(t)
				c.seed36(t)
				db := c.openCore(t, "rw")
				tc.mutate(t, db)
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				err := migrateCJSIManagedRoot(ctx, c.root, 0, c)
				assertCJSIV36RejectedCleanly(t, c, cjsiBaseCoreSchemaVersion, err)
			})
		}
	})

	t.Run("column type not null default check or trigger altered", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			mutate func(t *testing.T, db *sql.DB)
		}{
			{
				name: "quota_windows_v2 context_generation type INTEGER to TEXT",
				mutate: func(t *testing.T, db *sql.DB) {
					rebuildCJSITableWithDDL(t, db, "quota_windows_v2", func(ddl string) string {
						return strings.Replace(ddl, "context_generation INTEGER NOT NULL DEFAULT 0", "context_generation TEXT NOT NULL DEFAULT 0", 1)
					})
				},
			},
			{
				name: "quota_windows_v2 used_percent NOT NULL removed",
				mutate: func(t *testing.T, db *sql.DB) {
					rebuildCJSITableWithDDL(t, db, "quota_windows_v2", func(ddl string) string {
						return strings.Replace(ddl, "used_percent REAL NOT NULL", "used_percent REAL", 1)
					})
				},
			},
			{
				name: "quota_windows_v2 cycle_seq DEFAULT altered",
				mutate: func(t *testing.T, db *sql.DB) {
					rebuildCJSITableWithDDL(t, db, "quota_windows_v2", func(ddl string) string {
						return strings.Replace(ddl, "cycle_seq INTEGER NOT NULL DEFAULT 0", "cycle_seq INTEGER NOT NULL DEFAULT 1", 1)
					})
				},
			},
			{
				name: "quota_envelopes_v2 capture_gate CHECK removed",
				mutate: func(t *testing.T, db *sql.DB) {
					rebuildCJSITableWithDDL(t, db, "quota_envelopes_v2", func(ddl string) string {
						return strings.Replace(
							ddl,
							"capture_gate TEXT NOT NULL DEFAULT 'open_initial' CHECK(capture_gate IN ('open_initial','blocked','open'))",
							"capture_gate TEXT NOT NULL DEFAULT 'open_initial'",
							1,
						)
					})
				},
			},
			{
				name: "missing quota_revision_quota_windows_v2_INSERT trigger",
				mutate: func(t *testing.T, db *sql.DB) {
					cjsiFixtureExec(t, db, "DROP TRIGGER quota_revision_quota_windows_v2_INSERT")
				},
			},
			{
				name: "leftover legacy quota_windows table",
				mutate: func(t *testing.T, db *sql.DB) {
					cjsiFixtureExec(t, db, "CREATE TABLE quota_windows (client TEXT PRIMARY KEY)")
				},
			},
			{
				name: "leftover mixed-case legacy Quota_Windows table",
				mutate: func(t *testing.T, db *sql.DB) {
					cjsiFixtureExec(t, db, "CREATE TABLE Quota_Windows (client TEXT PRIMARY KEY)")
					assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_windows", 0)
				},
			},
			{
				name: "leftover uppercase legacy QUOTA_ENVELOPES table",
				mutate: func(t *testing.T, db *sql.DB) {
					cjsiFixtureExec(t, db, "CREATE TABLE QUOTA_ENVELOPES (client TEXT PRIMARY KEY)")
					assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_envelopes", 0)
				},
			},
			{
				name: "leftover mixed-case legacy Quota_Alert_Notices view",
				mutate: func(t *testing.T, db *sql.DB) {
					cjsiFixtureExec(t, db, "CREATE VIEW Quota_Alert_Notices AS SELECT 1 AS x")
					assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices", 1)
				},
			},
			{
				name: "unique index collation BINARY to NOCASE rejected",
				mutate: func(t *testing.T, db *sql.DB) {
					cjsiFixtureExec(t, db, `DROP INDEX usage_session_observations_delivery;
						CREATE UNIQUE INDEX usage_session_observations_delivery ON usage_session_observations(delivery_id COLLATE NOCASE)`)
				},
			},
			{
				name: "partial unique index predicate altered rejected",
				mutate: func(t *testing.T, db *sql.DB) {
					cjsiFixtureExec(t, db, `DROP INDEX provider_selections_completed_operation;
						CREATE UNIQUE INDEX provider_selections_completed_operation ON provider_selections(operation_id) WHERE operation_id IS NOT NULL AND operation_id <> ''`)
				},
			},
			{
				name: "double quoted NULL changes partial unique index predicate",
				mutate: func(t *testing.T, db *sql.DB) {
					assertCJSIMigrationNumber(t, db, `SELECT 'NULL' IS NOT NULL`, 1)
					assertCJSIMigrationNumber(t, db, `SELECT 'NULL' IS NOT "NULL"`, 0)
					cjsiFixtureExec(t, db, `DROP INDEX provider_selections_completed_operation;
						CREATE UNIQUE INDEX provider_selections_completed_operation ON provider_selections(operation_id)
						WHERE operation_id IS NOT "NULL"`)
				},
			},
			{
				name: "ordinary unique index replaced by expression index rejected",
				mutate: func(t *testing.T, db *sql.DB) {
					cjsiFixtureExec(t, db, `DROP INDEX usage_session_observations_delivery;
						CREATE UNIQUE INDEX usage_session_observations_delivery ON usage_session_observations(lower(delivery_id))`)
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				c := newCJSIMigrationCapsule(t)
				c.seed36(t)
				db := c.openCore(t, "rw")
				tc.mutate(t, db)
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				err := migrateCJSIManagedRoot(ctx, c.root, 0, c)
				assertCJSIV36RejectedCleanly(t, c, cjsiBaseCoreSchemaVersion, err)
			})
		}
	})

	t.Run("trigger distinguishes numeric separator from SELECT alias", func(t *testing.T) {
		trigger := func(expr string, want int) cjsiSchemaObjectDef {
			t.Helper()
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			cjsiFixtureExec(t, db, "CREATE TABLE items(id INTEGER); CREATE TABLE audit(value INTEGER)")
			cjsiFixtureExec(t, db, "CREATE TRIGGER record AFTER INSERT ON items BEGIN INSERT INTO audit SELECT "+expr+"; END")
			cjsiFixtureExec(t, db, "INSERT INTO items VALUES(1)")
			assertCJSIMigrationNumber(t, db, "SELECT value FROM audit", want)
			var d cjsiSchemaObjectDef
			if err := db.QueryRow("SELECT type, name, tbl_name, sql FROM sqlite_schema WHERE name='record'").
				Scan(&d.kind, &d.name, &d.table, &d.rawSQL); err != nil {
				t.Fatal(err)
			}
			return d
		}
		if equalCJSISchemaObjectDef(trigger("1_0", 10), trigger("1 _0", 1)) {
			t.Fatal("trigger writing 10 matched trigger writing 1")
		}
	})

	t.Run("missing singleton row", func(t *testing.T) {
		for _, table := range []string{
			"quota_cycle",
			"quota_projection_revision",
			"derived_snapshot_generation",
			"serving_generation",
		} {
			t.Run(table, func(t *testing.T) {
				c := newCJSIMigrationCapsule(t)
				c.seed36(t)
				db := c.openCore(t, "rw")
				cjsiFixtureExec(t, db, fmt.Sprintf("DELETE FROM %s WHERE singleton=1", table))
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				err := migrateCJSIManagedRoot(ctx, c.root, 0, c)
				assertCJSIV36RejectedCleanly(t, c, cjsiBaseCoreSchemaVersion, err)
			})
		}
	})

	t.Run("noop v36 migration callback", func(t *testing.T) {
		noop := append([]migration(nil), migrations...)
		noop[cjsiBaseCoreSchemaVersion-1].apply = func(context.Context, *sql.Tx) error { return nil }
		if err := validateCJSIMigrationRegistry(CurrentSchemaVersion, noop); !errors.Is(err, ErrUnknownSchema) {
			t.Fatalf("registry accepted non-nil noop v36 callback: %v", err)
		}

		extraStmt := append([]migration(nil), migrations...)
		extraStmt[cjsiBaseCoreSchemaVersion-1].statements = []string{"SELECT 1"}
		if err := validateCJSIMigrationRegistry(CurrentSchemaVersion, extraStmt); !errors.Is(err, ErrUnknownSchema) {
			t.Fatalf("registry accepted v36 with extra statements: %v", err)
		}

		// Semantically equivalent wrapper closure must succeed without reflect pointer equality.
		wrapped := append([]migration(nil), migrations...)
		wrapped[cjsiBaseCoreSchemaVersion-1].apply = func(ctx context.Context, tx *sql.Tx) error {
			return migrateQuotaIdentity(ctx, tx)
		}
		if err := validateCJSIMigrationRegistry(CurrentSchemaVersion, wrapped); err != nil {
			t.Fatalf("registry rejected semantically equivalent v36 wrapper: %v", err)
		}

		// Positive control: the real migrations registry passes.
		if err := validateCJSIMigrationRegistry(CurrentSchemaVersion, migrations); err != nil {
			t.Fatalf("real migrations registry rejected: %v", err)
		}

		// End-to-end: substituting a noop v36 callback in the live registry rejects
		// both v35 upgrade and cold bootstrap cleanly without writing 37, journal, fence, or private index.
		t.Run("managed migration rejects noop v36 callback from v35", func(t *testing.T) {
			c := newCJSIMigrationCapsule(t)
			c.seed35(t)
			orig := migrations[cjsiBaseCoreSchemaVersion-1]
			migrations[cjsiBaseCoreSchemaVersion-1].apply = func(context.Context, *sql.Tx) error { return nil }
			defer func() { migrations[cjsiBaseCoreSchemaVersion-1] = orig }()

			err := migrateCJSIManagedRoot(ctx, c.root, 0, c)
			assertCJSIV36RejectedCleanly(t, c, 35, err)
		})

		t.Run("managed migration rejects noop v36 callback on cold root", func(t *testing.T) {
			c := newCJSIMigrationCapsule(t)
			orig := migrations[cjsiBaseCoreSchemaVersion-1]
			migrations[cjsiBaseCoreSchemaVersion-1].apply = func(context.Context, *sql.Tx) error { return nil }
			defer func() { migrations[cjsiBaseCoreSchemaVersion-1] = orig }()

			err := migrateCJSIManagedRoot(ctx, c.root, 0, c)
			assertCJSIV36RejectedCleanly(t, c, 0, err)
		})
	})

	t.Run("positive upgrades and forged 37 rejection", func(t *testing.T) {
		t.Run("real quota 36 upgrades to 37", func(t *testing.T) {
			c := newCJSIMigrationCapsule(t)
			c.seed36(t)
			if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
				t.Fatalf("real quota 36 control upgrade failed: %v", err)
			}
			db := c.openCore(t, "ro")
			defer db.Close()
			assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion)
			assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 1)
			assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices_v2 WHERE instance_unix=123 AND notified_at='retained'", 1)
			assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_windows_v2 WHERE context_generation=0", 0)
			if err := explainQuotaWindowUpsert(db); err != nil {
				t.Fatalf("upgraded quota 37 UPSERT explain failed: %v", err)
			}
			if _, err := validateCJSIFence(c.root); err != nil {
				t.Fatalf("real quota 36 control fence invalid: %v", err)
			}
		})

		t.Run("schema 35 through 36 to 37 upgrades cleanly", func(t *testing.T) {
			c := newCJSIMigrationCapsule(t)
			c.seed35(t)
			if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
				t.Fatalf("schema 35->36->37 upgrade failed: %v", err)
			}
			db := c.openCore(t, "ro")
			defer db.Close()
			assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion)
			assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 1)
			assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM quota_alert_notices_v2 WHERE instance_unix=123 AND notified_at='retained'", 1)
			assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('quota_windows','quota_envelopes','quota_alert_notices')", 0)
			assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE type='trigger' AND name='telemetry_jsonl_retracts_otel'", 0)
			if err := explainQuotaWindowUpsert(db); err != nil {
				t.Fatalf("35->36->37 UPSERT explain failed: %v", err)
			}
			if _, err := validateCJSIFence(c.root); err != nil {
				t.Fatalf("35->36->37 fence invalid: %v", err)
			}
		})

		t.Run("equivalent index DDL whitespace variants upgrade cleanly", func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				mutate string
			}{
				{
					name: "ordinary unique index space before paren",
					mutate: `DROP INDEX usage_session_observations_delivery;
						CREATE UNIQUE INDEX usage_session_observations_delivery ON usage_session_observations (delivery_id)`,
				},
				{
					name: "partial unique index space around parens",
					mutate: `DROP INDEX provider_selections_completed_operation;
						CREATE UNIQUE INDEX provider_selections_completed_operation ON provider_selections ( operation_id ) WHERE operation_id IS NOT NULL`,
				},
				{
					name: "partial unique index quotes a real column",
					mutate: `DROP INDEX provider_selections_completed_operation;
						CREATE UNIQUE INDEX "Provider_Selections_Completed_Operation" ON [PROVIDER_SELECTIONS] ( [OPERATION_ID] )
						/* bound column */ WHERE "OPERATION_ID" IS NOT NULL`,
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					c := newCJSIMigrationCapsule(t)
					c.seed36(t)
					db := c.openCore(t, "rw")
					cjsiFixtureExec(t, db, tc.mutate)
					if err := db.Close(); err != nil {
						t.Fatal(err)
					}
					if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
						t.Fatalf("equivalent index DDL (%s) falsely rejected: %v", tc.name, err)
					}
					db = c.openCore(t, "ro")
					defer db.Close()
					assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", cjsiLayoutCoreSchemaVersion)
					assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'", 1)
					if _, err := validateCJSIFence(c.root); err != nil {
						t.Fatalf("equivalent index DDL (%s) fence invalid: %v", tc.name, err)
					}
				})
			}
		})

		t.Run("expression and partial index token semantics", func(t *testing.T) {
			openMem := func(t *testing.T, ddl string) cjsiTableSchema {
				t.Helper()
				db, err := sql.Open("sqlite", ":memory:")
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				db.SetMaxOpenConns(1)
				if _, err := db.ExecContext(ctx, ddl); err != nil {
					t.Fatal(err)
				}
				schema, err := readCJSITableSchema(ctx, db, "items")
				if err != nil {
					t.Fatal(err)
				}
				return schema
			}
			baseTable := `CREATE TABLE items(id INTEGER PRIMARY KEY, col TEXT NOT NULL, status TEXT NOT NULL); `
			t.Run("distinct SQLite expressions", func(t *testing.T) {
				db, err := sql.Open("sqlite", ":memory:")
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				for _, tc := range []struct {
					name, left, right string
				}{
					{"number and text", "typeof(1)", "typeof('1')"},
					{"parentheses change precedence", "(2 + 3) * 4", "2 + 3 * 4"},
					{"comparison collation", "'a' = 'A' COLLATE BINARY", "'a' = 'A' COLLATE NOCASE"},
					{"escaped single quote", "hex('it''s /*OPEN*/')", "hex('it''s /*open*/')"},
					{"escaped double quote fallback", `'O"Pen' IS "O""Pen"`, `'O"Pen' IS "O""pen"`},
					{"blob and string", "typeof(X'31')", "typeof('31')"},
				} {
					t.Run(tc.name, func(t *testing.T) {
						assertCJSIMigrationNumber(t, db, "SELECT ("+tc.left+") IS NOT ("+tc.right+")", 1)
						index := baseTable + "CREATE INDEX idx ON items("
						left := openMem(t, index+tc.left+")")
						right := openMem(t, index+tc.right+")")
						if err := compareCJSITableSchema("items", left, right); !errors.Is(err, ErrUnknownSchema) {
							t.Fatalf("different index expressions accepted: %v", err)
						}
					})
				}
			})
			t.Run("table CHECK distinguishes NULL from quoted text", func(t *testing.T) {
				table := "CREATE TABLE items(col TEXT CHECK(col IS NOT "
				left := openMem(t, table+"NULL))")
				right := openMem(t, table+`"NULL"))`)
				if err := compareCJSITableSchema("items", left, right); !errors.Is(err, ErrUnknownSchema) {
					t.Fatalf("different CHECK accepted: %v", err)
				}
			})
			ref := openMem(t, baseTable+`CREATE INDEX idx_expr ON items(lower(col)) WHERE status = 'open'; CREATE INDEX idx_ws ON items(col) WHERE status = 'a  b'`)
			same := openMem(t, baseTable+`CREATE INDEX "idx_expr" ON "items" ( lower ( col ) ) WHERE status='open'; CREATE INDEX idx_ws ON items ( col ) WHERE status = 'a  b'`)
			if err := compareCJSITableSchema("items", same, ref); err != nil {
				t.Fatalf("equivalent expression/partial index rejected: %v", err)
			}
			for _, tc := range []struct {
				name string
				ddl  string
			}{
				{
					name: "expression function changed",
					ddl:  baseTable + `CREATE INDEX idx_expr ON items(upper(col)) WHERE status = 'open'; CREATE INDEX idx_ws ON items(col) WHERE status = 'a  b'`,
				},
				{
					name: "partial literal case changed",
					ddl:  baseTable + `CREATE INDEX idx_expr ON items(lower(col)) WHERE status = 'OPEN'; CREATE INDEX idx_ws ON items(col) WHERE status = 'a  b'`,
				},
				{
					name: "partial literal internal whitespace changed",
					ddl:  baseTable + `CREATE INDEX idx_expr ON items(lower(col)) WHERE status = 'open'; CREATE INDEX idx_ws ON items(col) WHERE status = 'a b'`,
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					mutated := openMem(t, tc.ddl)
					if err := compareCJSITableSchema("items", mutated, ref); !errors.Is(err, ErrUnknownSchema) {
						t.Fatalf("divergent expression/partial index (%s) accepted: %v", tc.name, err)
					}
				})
			}
		})

		t.Run("forged version 37 missing composite PK rejected", func(t *testing.T) {
			c := newCJSIMigrationCapsule(t)
			c.seed36(t)
			db := c.openCore(t, "rw")
			rebuildCJSITableWithDDL(t, db, "quota_windows_v2", func(ddl string) string {
				key := "PRIMARY KEY (client, account_id, window_key)"
				idx := strings.Index(ddl, key)
				comma := strings.LastIndex(ddl[:idx], ",")
				return ddl[:comma] + ddl[idx+len(key):]
			})
			cjsiFixtureExec(t, db, "UPDATE schema_metadata SET version=?", cjsiLayoutCoreSchemaVersion)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			err := migrateCJSIManagedRoot(ctx, c.root, 0, c)
			assertCJSIV36RejectedCleanly(t, c, cjsiLayoutCoreSchemaVersion, err)
		})
	})
}

// Present an inert catalog row to the verifier without modifying SQLite's catalog.
type cjsiCatalogFixture struct {
	*sql.DB
	kind, name, ddl string
}

func (q cjsiCatalogFixture) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if strings.Contains(query, "main.sqlite_master") {
		query = strings.Replace(query, "main.sqlite_master", "(SELECT type,name,tbl_name,sql FROM main.sqlite_master UNION ALL SELECT ?,?,'quota_cycle',?)", 1)
		args = append([]any{q.kind, q.name, q.ddl}, args...)
	}
	return q.DB.QueryContext(ctx, query, args...)
}

func TestCJSIReservedCatalogObjects(t *testing.T) {
	ctx := t.Context()
	c := newCJSIMigrationCapsule(t)
	c.seed36(t)
	db := c.openCore(t, "rw")
	defer db.Close()
	refs, err := cjsiCanonicalSchemaReferences()
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []int{36, 37} {
		ref := refs.quota36
		if version == 37 {
			if err := migrateCJSICore(ctx, c.root, db); err != nil {
				t.Fatalf("migrate analyzed schema: %v", err)
			}
			ref = refs.layout37
		}
		cjsiFixtureExec(t, db, "ANALYZE")
		assertCJSIMigrationNumber(t, db, "SELECT version FROM schema_metadata", version)
		assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name='sqlite_stat1'", 1)
		if err := verifyCJSICoreSchema(ctx, db, ref); err != nil {
			t.Fatalf("analyzed schema %d: %v", version, err)
		}
		for _, row := range []cjsiCatalogFixture{
			{db, "trigger", "sqlite_fixture", ""},
			{db, "view", "SQLITE_Stat1", ""},
			{db, "index", "sqlite_autoindex_fixture_1", "explicit index"},
			{db, "table", "sqlite_stat5", ""},
		} {
			if err := verifyCJSICoreSchema(ctx, row, ref); !errors.Is(err, ErrUnknownSchema) {
				t.Errorf("schema %d accepted %s %s: %v", version, row.kind, row.name, err)
			}
		}
	}
}
