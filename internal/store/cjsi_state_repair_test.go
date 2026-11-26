package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/hookrefusal"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func TestCJSILegacyIndexMigrationPreflight(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed func(*testing.T, string)
		bad  bool
	}{
		{"zero-byte opener residue", func(t *testing.T, path string) {
			if err := preparePrivateSQLiteFiles(path); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"garbage content", func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("synthetic non-SQLite index bytes"), 0600); err != nil {
				t.Fatal(err)
			}
		}, true},
		{"SQLite integrity failure", func(t *testing.T, path string) {
			seedCJSILegacySessions(t, filepath.Dir(path))
			db := openCJSIFixtureDB(t, path, "rw")
			cjsiFixtureExec(t, db, `CREATE TABLE checked_fixture(n INTEGER CHECK(n > 0));
				PRAGMA ignore_check_constraints=ON; INSERT INTO checked_fixture VALUES(0); PRAGMA ignore_check_constraints=OFF;`)
			var integrity string
			if err := db.QueryRowContext(t.Context(), "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity == "ok" {
				t.Fatalf("fixture needs a readable corrupt database: %q, %v", integrity, err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			c := newCJSIMigrationCapsule(t)
			c.seed36(t)
			path := filepath.Join(c.root, cjsiLegacyIndexName)
			tc.seed(t, path)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.bad {
				if err := EnsureStateMigrated(ctx, c.root); err != nil {
					t.Fatal(err)
				}
				core, err := OpenReadOnly(ctx, c.root)
				if err != nil {
					t.Fatal(err)
				}
				defer core.Close()
				assertCJSIMigrationNumber(t, core.DB, "SELECT version FROM schema_metadata", CurrentSchemaVersion)
				assertCJSIMigrationNumber(t, core.DB, "SELECT phase FROM cjsi_layout_journal", int(cjsiLayoutReady))
				index := openCJSIFixtureDB(t, filepath.Join(c.root, cjsiPrivateIndexName), "ro")
				defer index.Close()
				assertCJSIMigrationNumber(t, index, "SELECT count(*) FROM session_metadata", 0)
				return
			}
			for _, migrate := range []func(context.Context, string) error{EnsureStateMigrated, MigrateState} {
				if err := migrate(ctx, c.root); !errors.Is(err, ErrLegacySessionIndexInvalid) || errors.Is(err, ErrStateBusy) {
					t.Fatalf("corrupt index migration = %v", err)
				}
			}
			core, err := OpenReadOnly(ctx, c.root)
			if err != nil {
				t.Fatal(err)
			}
			defer core.Close()
			assertCJSIMigrationNumber(t, core.DB, "SELECT version FROM schema_metadata", cjsiBaseCoreSchemaVersion)
			assertCJSIMigrationNumber(t, core.DB, "SELECT count(*) FROM sqlite_schema WHERE name='cjsi_layout_journal'", 0)
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("migration changed corrupt source bytes: %v", err)
			}
			for _, name := range []string{cjsiSnapshotName, cjsiPreparedName, cjsiPrivateIndexName} {
				if err := cjsiRequireCleanIndexPath(filepath.Join(c.root, name)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestCJSILegacySQLitePermissionsAreRepairedDuringMigration(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		name := "preflight"
		if interrupted {
			name = "resumed capture"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			c := newCJSIMigrationCapsule(t)
			c.seed36(t)
			seedCJSILegacySessions(t, c.root)
			legacy := filepath.Join(c.root, cjsiLegacyIndexName)
			core := filepath.Join(c.root, "agentdeck.sqlite3")
			if interrupted {
				if err := withCJSIManagedCore(ctx, c.root, time.Second, cjsiStoreLegacyManager{wait: time.Second}, func(root string, db *sql.DB) error {
					return migrateCJSICore(ctx, root, db)
				}); err != nil {
					t.Fatal(err)
				}
			}
			if err := CloseLegacyReadPools(c.root); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{legacy + "-journal", core + "-journal"} {
				if err := os.WriteFile(name, nil, 0644); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{legacy, core, legacy + "-journal", core + "-journal"} {
				if err := os.Chmod(name, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if !interrupted {
				if err := CheckLegacySessionIndexForMigration(ctx, c.root); err != nil {
					t.Fatalf("read-only verification refused healthy mode-drifted index: %v", err)
				}
				if info, err := os.Stat(legacy); err != nil || info.Mode().Perm() != 0644 {
					t.Fatalf("verification changed original mode: %v", err)
				}
			}
			if err := EnsureStateMigrated(ctx, c.root); err != nil {
				t.Fatal(err)
			}
			assertCJSIInstalledFence(t, c.root)
			for _, name := range []string{core, legacy, filepath.Join(c.root, cjsiPrivateIndexName)} {
				if info, err := os.Stat(name); err != nil || info.Mode().Perm() != 0600 {
					t.Fatalf("%s is not private after migration: %v", name, err)
				}
			}
		})
	}
}

func TestCJSILegacyPreflightPreservesOperationalErrors(t *testing.T) {
	ctx := t.Context()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cjsiFixtureExec(t, db, "CREATE TABLE quota_fixture(payload BLOB); PRAGMA max_page_count=2")
	_, full := db.ExecContext(ctx, "INSERT INTO quota_fixture VALUES(zeroblob(1048576))")
	var driverErr *sqlite.Error
	if !errors.As(full, &driverErr) || driverErr.Code() != sqlite3.SQLITE_FULL {
		t.Fatalf("fixture did not exhaust SQLite pages: %v", full)
	}
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"sqlite full", errors.Join(errCJSILayoutInvalid, full)},
		{"I/O error", &os.PathError{Op: "read", Path: "synthetic snapshot", Err: syscall.EIO}},
		{"permission error", &os.PathError{Op: "open", Path: "synthetic snapshot", Err: os.ErrPermission}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := legacyIndexPreflightError(ctx, tc.err); got != tc.err || errors.Is(got, ErrLegacySessionIndexInvalid) {
				t.Fatalf("operational error remapped: %v", got)
			}
		})
	}
}

func TestCJSIRestoreRecoveryMarker(t *testing.T) {
	for _, corePresent := range []bool{true, false} {
		name := "publish gap"
		if corePresent {
			name = "zero-byte core"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			root := newCJSIMigrationCapsule(t).root
			scan, err := AcquireScanLock(ctx, root, 0)
			if err != nil {
				t.Fatal(err)
			}
			state, err := AcquireLock(ctx, root, 0)
			if err != nil {
				scan.Release()
				t.Fatal(err)
			}
			defer scan.Release()
			defer state.Release()
			if err := MarkStateRestoreInProgress(ctx, root); err != nil {
				t.Fatal(err)
			}
			if err := MarkStateRestoreInProgress(ctx, root); !errors.Is(err, ErrStateRestoreInProgress) {
				t.Fatalf("second restore reserved target: %v", err)
			}
			if pin, err := admitLegacy(ctx, root); !errors.Is(err, ErrStateRestoreInProgress) {
				if err == nil {
					_ = pin.close()
				}
				t.Fatalf("marked root admitted a shared SQLite opener: %v", err)
			}
			if s, err := OpenWithLockHeld(ctx, root); s != nil || !errors.Is(err, ErrStateRestoreInProgress) {
				t.Fatalf("marker alone did not prevent opening the target: %v, %v", s, err)
			}
			residue := filepath.Join(root, "agentdeck.sqlite3")
			if !corePresent {
				residue = filepath.Join(root, cjsiLegacyIndexName)
			}
			var data []byte
			if !corePresent {
				marker := cjsiFenceBytes()
				data = marker[:]
			}
			if err := os.WriteFile(residue, data, 0600); err != nil {
				t.Fatal(err)
			}
			before := cjsiRootEntries(t, root)
			for _, open := range []func(context.Context, string) (*Store, error){Open, OpenExisting, OpenWithLockHeld, OpenReadOnly, OpenSessions, OpenSessionsReadOnly, OpenSessionDiagnosticsReadOnly} {
				s, err := open(ctx, root)
				if s != nil || !errors.Is(err, ErrStateRestoreInProgress) {
					t.Fatalf("restore target opener = %v, %v", s, err)
				}
			}
			for _, migrate := range []func(context.Context, string) error{EnsureStateMigrated, MigrateState} {
				if err := migrate(ctx, root); !errors.Is(err, ErrStateRestoreInProgress) {
					t.Fatalf("restore target migration = %v", err)
				}
			}
			if !maps.Equal(before, cjsiRootEntries(t, root)) {
				t.Fatal("rejection mutated interrupted Restore")
			}
			if err := os.Remove(residue); err != nil {
				t.Fatal(err)
			}
			if err := ClearStateRestoreInProgress(root); err != nil {
				t.Fatal(err)
			}
			if err := CheckStateRestoreInProgress(root); err != nil {
				t.Fatal(err)
			}
			core, err := OpenWithLockHeld(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			defer core.Close()
			assertCJSIMigrationNumber(t, core.DB, "SELECT version FROM schema_metadata", cjsiBaseCoreSchemaVersion)
		})
	}
}

func TestCJSIReadyHeadEpochMismatch(t *testing.T) {
	ctx := t.Context()
	c := newCJSIMigrationCapsule(t)
	seedCJSILegacySessions(t, c.root)
	if err := EnsureStateMigrated(ctx, c.root); err != nil {
		t.Fatal(err)
	}
	db := c.openCore(t, "rw")
	cjsiFixtureExec(t, db, "UPDATE cjsi_publication_head SET index_epoch=index_epoch+1")
	if _, err := cjsiReadIndexHead(ctx, db); err != nil {
		t.Fatalf("fixture head should be individually valid: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := EnsureStateMigrated(ctx, c.root); !errors.Is(err, errCJSILayoutInvalid) {
		t.Fatalf("ensure accepted a mismatched epoch: %v", err)
	}
	for _, open := range []func(context.Context, string) (*Store, error){Open, OpenReadOnly} {
		if s, err := open(ctx, c.root); s != nil || !errors.Is(err, errCJSILayoutInvalid) {
			t.Fatalf("open accepted a mismatched epoch: %v, %v", s, err)
		}
	}
}

func TestCJSIMigrateStateClearsOldHookRefusal(t *testing.T) {
	c := newCJSIMigrationCapsule(t)
	c.seed36(t)
	if err := hookrefusal.Write(c.root, 99, CurrentSchemaVersion); err != nil {
		t.Fatal(err)
	}
	if err := MigrateState(t.Context(), c.root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(c.root, hookrefusal.Filename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful migration retained old refusal: %v", err)
	}
}

func TestCJSIEnsureRetiresSnapshotAfterReady(t *testing.T) {
	ctx := t.Context()
	c := newCJSIMigrationCapsule(t)
	c.seed36(t)
	seedCJSILegacySessions(t, c.root)
	stopped := errors.New("interrupted after durable ready")
	err := withCJSIManagedCore(ctx, c.root, time.Second, cjsiStoreLegacyManager{wait: time.Second}, func(root string, db *sql.DB) error {
		if err := migrateCJSICore(ctx, root, db); err != nil {
			return err
		}
		return resumeCJSILayoutWithReady(ctx, root, db, func(ctx context.Context, prior, next cjsiLayoutJournal) error {
			if err := cjsiCommitInitialHead(ctx, root, db, prior, next); err != nil {
				return err
			}
			return stopped
		})
	})
	if !errors.Is(err, stopped) {
		t.Fatalf("missed ready-before-retirement boundary: %v", err)
	}
	db := c.openCore(t, "ro")
	ready, err := cjsiReadLayoutJournal(ctx, db)
	if err = errors.Join(err, db.Close()); err != nil || ready.phase != cjsiLayoutReady || !ready.snapshot.present() {
		t.Fatalf("ready journal with retained snapshot: %+v, %v", ready, err)
	}
	snapshot := filepath.Join(c.root, cjsiSnapshotName)
	if err := cjsiVerifyLayoutFile(ctx, snapshot, ready.snapshot); err != nil {
		t.Fatal(err)
	}
	if err := EnsureStateMigrated(ctx, c.root); err != nil {
		t.Fatal(err)
	}
	if err := cjsiRequireCleanIndexPath(snapshot); err != nil {
		t.Fatalf("ready fast path retained a retired snapshot: %v", err)
	}
	if fence, err := validateCJSIFence(c.root); err != nil || fence != ready.fence {
		t.Fatalf("snapshot retirement changed the fence: %v, %v", fence, err)
	}
}

func TestCJSIPreflightReclaimsKilledCopy(t *testing.T) {
	if root := os.Getenv("AGENTDECK_TEST_CJSI_PREFLIGHT_ROOT"); root != "" {
		err := withLegacyIndexPreflightDir(root, func(snapshot string) error {
			if err := cjsiBackupLayoutIndex(t.Context(), filepath.Join(root, cjsiLegacyIndexName), snapshot); err != nil {
				return err
			}
			return killCJSIFixtureProcess(t.Context())
		})
		t.Fatalf("preflight crash fixture returned: %v", err)
	}
	ctx := t.Context()
	c := newCJSIMigrationCapsule(t)
	c.seed36(t)
	seedCJSILegacySessions(t, c.root)
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCJSIPreflightReclaimsKilledCopy$")
	command.Env = append(os.Environ(), "AGENTDECK_TEST_CJSI_PREFLIGHT_ROOT="+c.root)
	output, err := command.CombinedOutput()
	var killed *exec.ExitError
	if !errors.As(err, &killed) || killed.ProcessState.String() != "signal: killed" {
		t.Fatalf("preflight process did not die after snapshot: %v\n%s", err, output)
	}
	dir := filepath.Join(c.root, cjsiPreflightDirName)
	copy := filepath.Join(dir, cjsiPreflightSnapshotName)
	for path, mode := range map[string]os.FileMode{dir: 0700, copy: 0600} {
		if st, err := os.Lstat(path); err != nil || st.Mode().Perm() != mode {
			t.Fatalf("unprotected or misplaced preflight residue %q: %v", path, err)
		}
	}
	for _, entry := range mustReadPreflightDir(t, os.TempDir()) {
		if bytes.HasPrefix([]byte(entry.Name()), []byte("agentdeck-index-preflight-")) {
			t.Fatal("preflight left an out-of-root snapshot")
		}
	}
	if err := CheckLegacySessionIndexForMigration(ctx, c.root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("restarted preflight retained crash residue: %v", err)
	}
	core := c.openCore(t, "ro")
	defer core.Close()
	assertCJSIMigrationNumber(t, core, "SELECT version FROM schema_metadata", cjsiBaseCoreSchemaVersion)
}

func mustReadPreflightDir(t *testing.T, root string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestCJSIPreflightPreservesLiveAndAliasedCopies(t *testing.T) {
	ctx := t.Context()
	c := newCJSIMigrationCapsule(t)
	c.seed36(t)
	seedCJSILegacySessions(t, c.root)
	if err := withLegacyIndexPreflightDir(c.root, func(snapshot string) error {
		if err := os.WriteFile(snapshot, []byte("active fixture"), 0600); err != nil {
			return err
		}
		if err := CheckLegacySessionIndexForMigration(ctx, c.root); !errors.Is(err, ErrStateBusy) {
			t.Fatalf("second preflight admitted while first active: %v", err)
		}
		if data, err := os.ReadFile(snapshot); err != nil || string(data) != "active fixture" {
			t.Fatalf("active snapshot removed or changed: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(c.root, cjsiPreflightDirName)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(dir, cjsiPreflightSnapshotName)
	if err := os.Link(filepath.Join(c.root, cjsiLegacyIndexName), linked); err != nil {
		t.Fatal(err)
	}
	if err := CheckLegacySessionIndexForMigration(ctx, c.root); err == nil {
		t.Fatal("preflight accepted an aliased residue")
	}
	if _, err := os.Lstat(linked); err != nil {
		t.Fatalf("preflight deleted an aliased residue: %v", err)
	}
}

func TestCJSIRestoreOwnerVerifiesBeforeClear(t *testing.T) {
	ctx := t.Context()
	c := newCJSIMigrationCapsule(t)
	if err := EnsureStateMigrated(ctx, c.root); err != nil {
		t.Fatal(err)
	}
	startLocks := func() (*Lock, *Lock) {
		t.Helper()
		scan, err := AcquireScanLock(ctx, c.root, 0)
		if err != nil {
			t.Fatal(err)
		}
		state, err := AcquireLock(ctx, c.root, 0)
		if err != nil {
			_ = scan.Release()
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = errors.Join(state.Release(), scan.Release()) })
		return scan, state
	}
	scan, state := startLocks()
	if err := MarkStateRestoreInProgress(ctx, c.root); err != nil {
		t.Fatal(err)
	}
	if managed, err := ResolveCJSIRuntimeRoot(ctx, c.root); managed != nil || !errors.Is(err, ErrStateRestoreInProgress) {
		t.Fatalf("ordinary resolver bypassed marker: %v, %v", managed, err)
	}
	if managed, err := ResolveCJSIRuntimeRootForRestore(ctx, c.root, nil, state); managed != nil || !errors.Is(err, ErrLockLost) {
		t.Fatalf("owner resolver allowed absent scan proof: %v, %v", managed, err)
	}
	if managed, err := ResolveCJSIRuntimeRootForRestore(ctx, c.root, scan, state); managed == nil || err != nil {
		t.Fatalf("owner could not verify published layout: %v, %v", managed, err)
	}
	if err := CheckStateRestoreInProgress(c.root); !errors.Is(err, ErrStateRestoreInProgress) {
		t.Fatalf("verification cleared marker: %v", err)
	}
	fencePath := filepath.Join(c.root, cjsiLegacyIndexName)
	if err := os.WriteFile(fencePath, []byte("invalid fence"), 0600); err != nil {
		t.Fatal(err)
	}
	if managed, err := ResolveCJSIRuntimeRootForRestore(ctx, c.root, scan, state); managed != nil || !errors.Is(err, errCJSIFenceInvalid) {
		t.Fatalf("owner verification bypassed the normal fence check: %v, %v", managed, err)
	}
	fence := cjsiFenceBytes()
	if err := os.WriteFile(fencePath, fence[:], 0600); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(state.Release(), scan.Release()); err != nil {
		t.Fatal(err)
	}
	if managed, err := ResolveCJSIRuntimeRootForRestore(ctx, c.root, scan, state); managed != nil || !errors.Is(err, ErrLockLost) {
		t.Fatalf("owner resolver allowed released lock handles: %v, %v", managed, err)
	}
	scan, state = startLocks()
	if managed, err := ResolveCJSIRuntimeRootForRestore(ctx, c.root, scan, state); managed != nil || !errors.Is(err, ErrLockLost) {
		t.Fatalf("new lock generation bypassed another owner's marker: %v, %v", managed, err)
	}
}

func TestCJSIInterruptedColdCore(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, marked := range []bool{false, true} {
			for _, managed := range []bool{false, true} {
				name := map[bool]string{false: "single-file", true: "four-file"}[legacy] +
					map[bool]string{false: "/unmarked", true: "/marked"}[marked] +
					map[bool]string{false: "/open", true: "/migrate"}[managed]
				t.Run(name, func(t *testing.T) {
					ctx := t.Context()
					root := newCJSIMigrationCapsule(t).root
					if marked {
						scan, err := AcquireScanLock(ctx, root, 0)
						if err != nil {
							t.Fatal(err)
						}
						state, err := AcquireLock(ctx, root, 0)
						if err != nil {
							_ = scan.Release()
							t.Fatal(err)
						}
						if err := errors.Join(MarkStateRestoreInProgress(ctx, root), state.Release(), scan.Release()); err != nil {
							t.Fatal(err)
						}
					}
					path := filepath.Join(root, "agentdeck.sqlite3")
					if legacy {
						if err := preparePrivateSQLiteFiles(path); err != nil {
							t.Fatal(err)
						}
					} else if err := os.WriteFile(path, nil, 0600); err != nil {
						t.Fatal(err)
					}
					before := cjsiRootEntries(t, root)
					readErr := error(ErrStateCoreUninitialized)
					if marked {
						readErr = ErrStateRestoreInProgress
					}
					if s, err := OpenReadOnly(ctx, root); s != nil || !errors.Is(err, readErr) {
						t.Fatalf("cold core read = %v, %v", s, err)
					}
					var err error
					version := cjsiBaseCoreSchemaVersion
					if managed {
						err = EnsureStateMigrated(ctx, root)
						version = CurrentSchemaVersion
					} else {
						var s *Store
						s, err = Open(ctx, root)
						if s != nil {
							err = errors.Join(err, s.Close())
						}
					}
					if marked {
						if !errors.Is(err, ErrStateRestoreInProgress) || !maps.Equal(before, cjsiRootEntries(t, root)) {
							t.Fatalf("marked cold core altered or admitted: %v", err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					s, err := OpenReadOnly(ctx, root)
					if err != nil {
						t.Fatal(err)
					}
					defer s.Close()
					assertCJSIMigrationNumber(t, s.DB, "SELECT version FROM schema_metadata", version)
				})
			}
		}
	}
}

func TestCJSIEmptyCorePreservesNonemptySidecar(t *testing.T) {
	ctx := t.Context()
	root := newCJSIMigrationCapsule(t).root
	core := filepath.Join(root, "agentdeck.sqlite3")
	if err := os.WriteFile(core, nil, 0600); err != nil {
		t.Fatal(err)
	}
	wal := core + "-wal"
	want := []byte("synthetic unfinished WAL")
	if err := os.WriteFile(wal, want, 0600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureStateMigrated(ctx, root); !errors.Is(err, ErrStateRestoreInProgress) {
		t.Fatalf("ambiguous zero-byte core admitted: %v", err)
	}
	if got, err := os.ReadFile(wal); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("ambiguous zero-byte core changed WAL: %v", err)
	}
}
