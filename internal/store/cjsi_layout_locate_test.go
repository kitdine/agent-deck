package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var errCJSILocateStop = errors.New("stop after a durable layout phase")

// Every root here comes from the real numbered migrations, the production
// layout adapters and a managed drain. Readers supply their own core
// transaction, as a future opener does.
func TestCJSIFencedStateLayoutLocation(t *testing.T) {
	t.Run("legacy core version keeps the legacy name without touching files", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			seed    func(cjsiMigrationCapsule)
			version int
		}{
			{"schema 35", func(c cjsiMigrationCapsule) { c.seed35(t) }, 35},
			{"schema 36", func(c cjsiMigrationCapsule) { c.seed36(t) }, cjsiBaseCoreSchemaVersion},
		} {
			t.Run(tc.name, func(t *testing.T) {
				c := newCJSIMigrationCapsule(t)
				tc.seed(c)
				seedCJSILegacySessions(t, c.root)
				core := c.openCore(t, "ro")
				defer core.Close()
				assertCJSIMigrationNumber(t, core, "SELECT version FROM schema_metadata", tc.version)
				before := cjsiRootEntries(t, c.root)
				got, err := locateCJSIWithCore(t, c.root, core)
				want := cjsiSessionLocation{route: cjsiRouteLegacy, path: filepath.Join(c.root, cjsiLegacyIndexName)}
				if err != nil || got != want {
					t.Fatalf("legacy location = %+v, %v; want %+v", got, err, want)
				}
				if after := cjsiRootEntries(t, c.root); !maps.Equal(before, after) {
					t.Fatalf("legacy location touched the root:\nbefore %v\nafter  %v", before, after)
				}
				// The legacy route leaves the existing opener and data intact.
				legacy, err := OpenSessionsReadOnly(t.Context(), c.root)
				if err != nil {
					t.Fatal(err)
				}
				assertCJSIMigrationNumber(t, legacy.DB, "SELECT count(*) FROM session_exclusions WHERE value='retained'", 1)
				if err := errors.Join(legacy.Close(), closeCJSIInstallReadPool(legacy.path)); err != nil {
					t.Fatal(err)
				}
			})
		}
	})

	t.Run("version and journal disagreements fail closed before any file", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		core := c.openCore(t, "rw")
		defer core.Close()
		cases := []struct {
			name      string
			statement string
			check     func(error) bool
		}{
			{"journal below the layout version", fmt.Sprintf("UPDATE schema_metadata SET version=%d", cjsiBaseCoreSchemaVersion),
				func(err error) bool { return errors.Is(err, errCJSILayoutInvalid) }},
			{"layout version without a journal", "DROP TABLE cjsi_layout_journal",
				func(err error) bool { return errors.Is(err, errCJSILayoutInvalid) }},
			{"missing journal record", "DELETE FROM cjsi_layout_journal",
				func(err error) bool { return errors.Is(err, errCJSILayoutInvalid) && errors.Is(err, sql.ErrNoRows) }},
			{"later core version", fmt.Sprintf("UPDATE schema_metadata SET version=%d", cjsiLayoutCoreSchemaVersion+1),
				func(err error) bool {
					var ahead *SchemaAhead
					return errors.As(err, &ahead) && ahead.Stored == cjsiLayoutCoreSchemaVersion+1 && ahead.Supported == cjsiLayoutCoreSchemaVersion
				}},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				// The reader's own transaction sees the change; rollback restores it.
				tx := beginCJSIFixtureTx(t, core)
				defer tx.Rollback()
				if _, err := tx.ExecContext(t.Context(), test.statement); err != nil {
					t.Fatal(err)
				}
				before := cjsiRootEntries(t, c.root)
				got, err := locateCJSISessionIndex(t.Context(), c.root, tx)
				if !test.check(err) || got != (cjsiSessionLocation{}) {
					t.Fatalf("location = %+v, %v", got, err)
				}
				if after := cjsiRootEntries(t, c.root); !maps.Equal(before, after) {
					t.Fatalf("rejected location touched the root:\nbefore %v\nafter  %v", before, after)
				}
			})
		}
		if got, err := locateCJSIWithCore(t, c.root, core); err != nil || got != (cjsiSessionLocation{route: cjsiRouteAbsent}) {
			t.Fatalf("rolled-back cases changed the layout: %+v, %v", got, err)
		}
	})

	t.Run("unfinished durable phases stay closed until the drained resume", func(t *testing.T) {
		for _, stop := range []cjsiInstallPhase{cjsiFenceInstalling, cjsiLegacyCaptured, cjsiFenceDurable, cjsiPrivatePrepared} {
			name := map[cjsiInstallPhase]string{cjsiFenceInstalling: "fence installing", cjsiLegacyCaptured: "legacy captured",
				cjsiFenceDurable: "fence durable", cjsiPrivatePrepared: "private prepared"}[stop]
			t.Run(name, func(t *testing.T) {
				ctx := t.Context()
				c := newCJSIMigrationCapsule(t)
				c.seed35(t)
				seedCJSILegacySessions(t, c.root)
				err := withCJSIManagedCore(ctx, c.root, 0, c, func(physical string, db *sql.DB) error {
					if err := migrateCJSICore(ctx, physical, db); err != nil || stop == cjsiFenceInstalling {
						return err
					}
					journal := cjsiSQLiteLayoutJournal{db}
					err := resumeCJSILayoutInstall(ctx, physical, cjsiLayoutInstall{
						load: journal.load,
						advance: func(ctx context.Context, prior, next cjsiLayoutJournal) error {
							if err := journal.advance(ctx, prior, next); err != nil {
								return err
							}
							if next.phase == stop {
								return errCJSILocateStop
							}
							return nil
						},
						prepare:  prepareCJSIProductionIndex,
						readSeal: readCJSIProductionSeal,
					})
					if !errors.Is(err, errCJSILocateStop) {
						return fmt.Errorf("install did not stop at phase %d: %w", stop, err)
					}
					return closeCJSIInstallReadPool(filepath.Join(physical, cjsiPrivateIndexName))
				})
				if err != nil {
					t.Fatal(err)
				}
				core := c.openCore(t, "ro")
				j, err := (cjsiSQLiteLayoutJournal{core}).load(ctx)
				if err != nil || j.phase != stop {
					t.Fatalf("durable phase %d, want %d: %v", j.phase, stop, err)
				}
				// Before the swap the legacy file is still a readable SQLite
				// database. The located route never falls back to it.
				if _, err := validateCJSIFence(c.root); (err == nil) != (stop >= cjsiFenceDurable) {
					t.Fatalf("fence state at phase %d: %v", stop, err)
				}
				before := cjsiRootEntries(t, c.root)
				got, err := locateCJSIWithCore(t, c.root, core)
				if !errors.Is(err, errCJSILayoutPending) || !errors.Is(err, ErrStateBusy) || got != (cjsiSessionLocation{}) {
					t.Fatalf("unfinished phase %d located %+v, %v", stop, got, err)
				}
				if after := cjsiRootEntries(t, c.root); !maps.Equal(before, after) {
					t.Fatalf("pending location touched the root:\nbefore %v\nafter  %v", before, after)
				}
				if err := core.Close(); err != nil {
					t.Fatal(err)
				}
				// The same unfinished record under another root is foreign.
				moved := cjsiMovedRoot(t)
				moveCJSIRootEntries(t, c.root, moved)
				movedCore := openCJSIFixtureDB(t, filepath.Join(moved, "agentdeck.sqlite3"), "ro")
				if got, err := locateCJSIWithCore(t, moved, movedCore); !errors.Is(err, errCJSILayoutInvalid) || errors.Is(err, ErrStateBusy) || got != (cjsiSessionLocation{}) {
					t.Fatalf("foreign unfinished record located %+v, %v", got, err)
				}
				if err := movedCore.Close(); err != nil {
					t.Fatal(err)
				}
				moveCJSIRootEntries(t, moved, c.root)

				if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
					t.Fatal(err)
				}
				core = c.openCore(t, "ro")
				defer core.Close()
				private := filepath.Join(c.root, cjsiPrivateIndexName)
				t.Cleanup(func() { _ = closeCJSIInstallReadPool(private) })
				ready, err := (cjsiSQLiteLayoutJournal{core}).load(ctx)
				if err != nil {
					t.Fatal(err)
				}
				want := cjsiSessionLocation{route: cjsiRoutePrivate, path: private, selection: ready.selection()}
				if got, err := locateCJSIWithCore(t, c.root, core); err != nil || got != want || want.selection.phase != cjsiLayoutSelected {
					t.Fatalf("resumed location = %+v, %v; want %+v", got, err, want)
				}
				index := openCJSIFixtureDB(t, private, "ro")
				defer index.Close()
				assertCJSIMigrationNumber(t, index, "SELECT count(*) FROM session_exclusions WHERE value='retained'", 1)
			})
		}
	})

	t.Run("never-built layout stays absent and creates nothing", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		core := c.openCore(t, "ro")
		defer core.Close()
		private := filepath.Join(c.root, cjsiPrivateIndexName)
		absent := cjsiSessionLocation{route: cjsiRouteAbsent}
		before := cjsiRootEntries(t, c.root)
		if got, err := locateCJSIWithCore(t, c.root, core); err != nil || got != absent {
			t.Fatalf("never-built location = %+v, %v", got, err)
		}
		if after := cjsiRootEntries(t, c.root); !maps.Equal(before, after) {
			t.Fatalf("never-built location touched the root:\nbefore %v\nafter  %v", before, after)
		}
		if err := cjsiRequireCleanIndexPath(private); err != nil {
			t.Fatal(err)
		}

		// A residual database at the private name is neither selected nor opened.
		residual := openCJSIFixtureDB(t, private, "rwc")
		cjsiFixtureExec(t, residual, "CREATE TABLE residual(value TEXT)")
		if err := errors.Join(residual.Close(), os.Chmod(private, 0600)); err != nil {
			t.Fatal(err)
		}
		before = cjsiRootEntries(t, c.root)
		if got, err := locateCJSIWithCore(t, c.root, core); err != nil || got != absent {
			t.Fatalf("residual index was located: %+v, %v", got, err)
		}
		if after := cjsiRootEntries(t, c.root); !maps.Equal(before, after) {
			t.Fatalf("residual index was opened:\nbefore %v\nafter  %v", before, after)
		}

		legacy := filepath.Join(c.root, cjsiLegacyIndexName)
		prior := filepath.Join(c.root, "prior-fence")
		if err := os.Rename(legacy, prior); err != nil {
			t.Fatal(err)
		}
		replaceCJSIFence(t, c.root)
		if got, err := locateCJSIWithCore(t, c.root, core); !errors.Is(err, errCJSIFenceInvalid) || got != (cjsiSessionLocation{}) {
			t.Fatalf("unrecorded fence accepted: %+v, %v", got, err)
		}
		if err := os.Remove(legacy); err != nil {
			t.Fatal(err)
		}
		recreated := openCJSIFixtureDB(t, legacy, "rwc")
		cjsiFixtureExec(t, recreated, "CREATE TABLE session_sources(source_path TEXT)")
		if err := errors.Join(recreated.Close(), os.Chmod(legacy, 0600)); err != nil {
			t.Fatal(err)
		}
		if got, err := locateCJSIWithCore(t, c.root, core); !errors.Is(err, errCJSIFenceInvalid) || got != (cjsiSessionLocation{}) {
			t.Fatalf("missing fence fell back to a legacy database: %+v, %v", got, err)
		}
		if err := errors.Join(os.Remove(legacy), os.Rename(prior, legacy)); err != nil {
			t.Fatal(err)
		}
		if got, err := locateCJSIWithCore(t, c.root, core); err != nil || got != absent {
			t.Fatalf("restored fence: %+v, %v", got, err)
		}
	})

	t.Run("selected index binds identity epoch seal fence and root", func(t *testing.T) {
		ctx := t.Context()
		c := newCJSIMigrationCapsule(t)
		c.seed35(t)
		seedCJSILegacySessions(t, c.root)
		if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		private := filepath.Join(c.root, cjsiPrivateIndexName)
		t.Cleanup(func() { _ = closeCJSIInstallReadPool(private) })
		core := c.openCore(t, "rw")
		ready, err := (cjsiSQLiteLayoutJournal{core}).load(ctx)
		if err != nil {
			t.Fatal(err)
		}
		want := cjsiSessionLocation{route: cjsiRoutePrivate, path: private, selection: ready.selection()}
		before := cjsiRootEntries(t, c.root, private+"-wal", private+"-shm")
		if got, err := locateCJSIWithCore(t, c.root, core); err != nil || got != want {
			t.Fatalf("selected location = %+v, %v; want %+v", got, err, want)
		}
		if after := cjsiRootEntries(t, c.root, private+"-wal", private+"-shm"); !maps.Equal(before, after) {
			t.Fatalf("selected location changed layout files:\nbefore %v\nafter  %v", before, after)
		}

		// A private layout version this binary does not know is never opened.
		if err := closeCJSIInstallReadPool(private); err != nil {
			t.Fatal(err)
		}
		tx := beginCJSIFixtureTx(t, core)
		if _, err := tx.ExecContext(ctx, "UPDATE cjsi_layout_journal SET seal_version=2"); err != nil {
			t.Fatal(err)
		}
		if got, err := locateCJSISessionIndex(ctx, c.root, tx); !errors.Is(err, errCJSILayoutInvalid) || got != (cjsiSessionLocation{}) {
			t.Fatalf("unsupported private layout version located %+v, %v", got, err)
		}
		if processReadPools.contains(private) {
			t.Fatal("unsupported private layout version opened the index")
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}

		aside := filepath.Join(c.root, "prior-index")
		if err := os.Rename(private, aside); err != nil {
			t.Fatal(err)
		}
		copyCJSIFile(t, aside, private)
		if got, err := locateCJSIWithCore(t, c.root, core); !errors.Is(err, errCJSILayoutInvalid) || got != (cjsiSessionLocation{}) {
			t.Fatalf("replacement index located %+v, %v", got, err)
		}
		if err := errors.Join(os.Remove(private), os.Rename(aside, private)); err != nil {
			t.Fatal(err)
		}

		// A byte-identical fence at a new inode passes content validation.
		// Only the journal's recorded identity rejects it.
		legacy := filepath.Join(c.root, cjsiLegacyIndexName)
		prior := filepath.Join(c.root, "prior-fence")
		if err := os.Rename(legacy, prior); err != nil {
			t.Fatal(err)
		}
		replaceCJSIFence(t, c.root)
		if got, err := locateCJSIWithCore(t, c.root, core); !errors.Is(err, errCJSIFenceInvalid) || got != (cjsiSessionLocation{}) {
			t.Fatalf("unrecorded fence accepted: %+v, %v", got, err)
		}
		if err := os.Remove(legacy); err != nil {
			t.Fatal(err)
		}
		recreated := openCJSIFixtureDB(t, legacy, "rwc")
		cjsiFixtureExec(t, recreated, "CREATE TABLE session_sources(source_path TEXT)")
		if err := errors.Join(recreated.Close(), os.Chmod(legacy, 0600)); err != nil {
			t.Fatal(err)
		}
		if got, err := locateCJSIWithCore(t, c.root, core); !errors.Is(err, errCJSIFenceInvalid) || got != (cjsiSessionLocation{}) {
			t.Fatalf("missing fence fell back to a legacy database: %+v, %v", got, err)
		}
		if err := errors.Join(os.Remove(legacy), os.Rename(prior, legacy)); err != nil {
			t.Fatal(err)
		}
		if got, err := locateCJSIWithCore(t, c.root, core); err != nil || got != want {
			t.Fatalf("restored layout = %+v, %v", got, err)
		}

		// Every child keeps its inode. Only the root identity changes.
		if err := errors.Join(core.Close(), closeCJSIInstallReadPool(private)); err != nil {
			t.Fatal(err)
		}
		moved := cjsiMovedRoot(t)
		moveCJSIRootEntries(t, c.root, moved)
		movedCore := openCJSIFixtureDB(t, filepath.Join(moved, "agentdeck.sqlite3"), "ro")
		defer movedCore.Close()
		if got, err := locateCJSIWithCore(t, moved, movedCore); !errors.Is(err, errCJSILayoutInvalid) || got != (cjsiSessionLocation{}) {
			t.Fatalf("journal of another root located %+v, %v", got, err)
		}
		if processReadPools.contains(filepath.Join(moved, cjsiPrivateIndexName)) {
			t.Fatal("moved root opened its private index")
		}
	})

	t.Run("unrecorded fence is rejected before the private index opens", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		c.seed35(t)
		seedCJSILegacySessions(t, c.root)
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		private := filepath.Join(c.root, cjsiPrivateIndexName)
		t.Cleanup(func() { _ = closeCJSIInstallReadPool(private) })
		core := c.openCore(t, "ro")
		defer core.Close()
		// Start cold. The install retired its readers and nothing has located.
		if err := closeCJSIInstallReadPool(private); err != nil || processReadPools.contains(private) {
			t.Fatalf("private reader before location: %v", err)
		}
		// Identical fence bytes at a new inode pass content validation alone.
		replaceCJSIFence(t, c.root)
		before := cjsiRootEntries(t, c.root)
		if got, err := locateCJSIWithCore(t, c.root, core); !errors.Is(err, errCJSIFenceInvalid) || got != (cjsiSessionLocation{}) {
			t.Fatalf("unrecorded fence located %+v, %v", got, err)
		}
		if processReadPools.contains(private) {
			t.Fatal("rejected location opened and retained a private reader")
		}
		if after := cjsiRootEntries(t, c.root); !maps.Equal(before, after) {
			t.Fatalf("rejected location touched the root:\nbefore %v\nafter  %v", before, after)
		}
	})

	t.Run("one core snapshot spans a concurrent managed migration", func(t *testing.T) {
		ctx := t.Context()
		c := newCJSIMigrationCapsule(t)
		seeded, err := Open(ctx, c.root)
		if err != nil {
			t.Fatal(err)
		}
		if err := errors.Join(seeded.Close(), closeCJSIInstallReadPool(seeded.path)); err != nil {
			t.Fatal(err)
		}
		seedCJSILegacySessions(t, c.root)
		private := filepath.Join(c.root, cjsiPrivateIndexName)
		t.Cleanup(func() {
			_ = errors.Join(closeCJSIInstallReadPool(private), closeCJSIInstallReadPool(filepath.Join(c.root, cjsiLegacyIndexName)))
		})

		reader := c.openCore(t, "ro")
		defer reader.Close()
		tx, err := reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		var pinned int
		if err := tx.QueryRowContext(ctx, "SELECT version FROM schema_metadata").Scan(&pinned); err != nil || pinned != cjsiBaseCoreSchemaVersion {
			t.Fatalf("snapshot version %d: %v", pinned, err)
		}
		if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		legacy := cjsiSessionLocation{route: cjsiRouteLegacy, path: filepath.Join(c.root, cjsiLegacyIndexName)}
		if got, err := locateCJSISessionIndex(ctx, c.root, tx); err != nil || got != legacy {
			t.Fatalf("pinned snapshot located %+v, %v; want %+v", got, err, legacy)
		}
		// That older answer reaches only the fence, which the legacy opener refuses.
		if s, err := OpenSessionsReadOnly(ctx, c.root); !cjsiIsNotADB(err) {
			if s != nil {
				_ = s.Close()
			}
			t.Fatalf("stale legacy route opened data: %v", err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		got, err := locateCJSIWithCore(t, c.root, reader)
		if err != nil || got.route != cjsiRoutePrivate || got.path != private {
			t.Fatalf("fresh snapshot located %+v, %v", got, err)
		}
	})

	t.Run("a drained root refuses verification and the resume retires its reader", func(t *testing.T) {
		ctx := t.Context()
		root := legacyTestRoot(t)
		legacy, err := OpenSessions(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		cjsiFixtureExec(t, legacy.DB, "INSERT INTO session_exclusions VALUES('kind','retained')")
		if err := errors.Join(legacy.Close(), closeCJSIInstallReadPool(legacy.path)); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(ctx, root, time.Second, cjsiStoreLegacyManager{time.Second}); err != nil {
			t.Fatal(err)
		}
		physical, err := platform.GuardStateRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		private := filepath.Join(physical, cjsiPrivateIndexName)
		t.Cleanup(func() { _ = closeCJSIInstallReadPool(private) })
		if err := closeCJSIInstallReadPool(private); err != nil || processReadPools.contains(private) {
			t.Fatalf("install left a private reader: %v", err)
		}
		core := openCJSIFixtureDB(t, filepath.Join(physical, "agentdeck.sqlite3"), "ro")
		defer core.Close()

		release := mustDrainLegacy(t, physical)
		if got, err := locateCJSIWithCore(t, physical, core); !errors.Is(err, ErrStateBusy) || got != (cjsiSessionLocation{}) {
			t.Fatalf("drained root located %+v, %v", got, err)
		}
		if processReadPools.contains(private) {
			t.Fatal("verification opened the index while the root was drained")
		}
		if err := release(); err != nil {
			t.Fatal(err)
		}

		got, err := locateCJSIWithCore(t, physical, core)
		if err != nil || got.route != cjsiRoutePrivate || got.path != private {
			t.Fatalf("released root located %+v, %v", got, err)
		}
		if !processReadPools.contains(private) {
			t.Fatal("verification reader was not pooled")
		}
		// No admission outlives the call, and the idle reader holds none.
		release, err = (cjsiStoreLegacyManager{}).drainLegacy(ctx, physical)
		if err != nil {
			t.Fatalf("location left root admission held: %v", err)
		}
		if err := release(); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(ctx, root, time.Second, cjsiStoreLegacyManager{time.Second}); err != nil {
			t.Fatal(err)
		}
		if processReadPools.contains(private) {
			t.Fatal("drained resume kept the idle verification reader")
		}
		index := openCJSIFixtureDB(t, private, "ro")
		defer index.Close()
		assertCJSIMigrationNumber(t, index, "SELECT count(*) FROM session_exclusions WHERE value='retained'", 1)
	})

	t.Run("caller arguments", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		core := c.openCore(t, "ro")
		defer core.Close()
		tx, err := core.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := locateCJSISessionIndex(canceled, c.root, tx); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled location: %v", err)
		}
		if _, err := locateCJSISessionIndex(t.Context(), c.root, nil); !errors.Is(err, errCJSILayoutInvalid) {
			t.Fatalf("missing core transaction: %v", err)
		}
		if _, err := locateCJSISessionIndex(t.Context(), "relative", tx); !errors.Is(err, errCJSILayoutInvalid) {
			t.Fatalf("relative root: %v", err)
		}
	})
}

func locateCJSIWithCore(t *testing.T, root string, core *sql.DB) (cjsiSessionLocation, error) {
	t.Helper()
	tx, err := core.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	return locateCJSISessionIndex(t.Context(), root, tx)
}

func seedCJSILegacySessions(t *testing.T, root string) {
	t.Helper()
	legacy, err := OpenSessions(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	cjsiFixtureExec(t, legacy.DB, "INSERT INTO session_exclusions VALUES('kind','retained')")
	if err := errors.Join(legacy.Close(), closeCJSIInstallReadPool(legacy.path)); err != nil {
		t.Fatal(err)
	}
}

func replaceCJSIFence(t *testing.T, root string) {
	t.Helper()
	staged := filepath.Join(root, "next-fence")
	data := cjsiFenceBytes()
	if err := os.WriteFile(staged, data[:], 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staged, filepath.Join(root, cjsiLegacyIndexName)); err != nil {
		t.Fatal(err)
	}
	if _, err := validateCJSIFence(root); err != nil {
		t.Fatal(err)
	}
}

func cjsiMovedRoot(t *testing.T) string {
	t.Helper()
	moved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return moved
}

// Renaming keeps every child inode, so only the root identity changes.
func moveCJSIRootEntries(t *testing.T, from, to string) {
	t.Helper()
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.Rename(filepath.Join(from, entry.Name()), filepath.Join(to, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
}

func copyCJSIFile(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func cjsiIsNotADB(err error) bool {
	var driverErr *sqlite.Error
	return errors.As(err, &driverErr) && driverErr.Code() == sqlite3.SQLITE_NOTADB
}

type cjsiRootEntry struct {
	mode  os.FileMode
	size  int64
	mtime int64
	inode uint64
}

// The caller's core connection owns the core WAL sidecars.
func cjsiRootEntries(t *testing.T, root string, ignore ...string) map[string]cjsiRootEntry {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	skip := map[string]bool{"agentdeck.sqlite3-wal": true, "agentdeck.sqlite3-shm": true}
	for _, path := range ignore {
		skip[filepath.Base(path)] = true
	}
	state := make(map[string]cjsiRootEntry)
	for _, entry := range entries {
		if skip[entry.Name()] {
			continue
		}
		info, err := os.Lstat(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatal("missing file identity")
		}
		state[entry.Name()] = cjsiRootEntry{info.Mode(), info.Size(), info.ModTime().UnixNano(), uint64(stat.Ino)}
	}
	return state
}
