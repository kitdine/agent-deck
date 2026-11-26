package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCJSIProductionLayoutSchemaIntent(t *testing.T) {
	root := t.TempDir()
	core := newCJSIProductionCore(t, root)
	ctx := t.Context()
	cjsiFixtureExec(t, core, "UPDATE schema_metadata SET version=?", cjsiBaseCoreSchemaVersion-1)
	tx := beginCJSIFixtureTx(t, core)
	if err := initializeCJSILayoutCore(ctx, tx, root); !errors.Is(err, ErrUnknownSchema) {
		t.Fatalf("older schema bypassed base prerequisite: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := resumeCJSIProductionLayout(ctx, root, core); !errors.Is(err, ErrUnknownSchema) {
		t.Fatalf("older core activated installation: %v", err)
	}
	if err := cjsiRequireAbsent(filepath.Join(root, cjsiLegacyIndexName)); err != nil {
		t.Fatal(err)
	}

	cjsiFixtureExec(t, core, "UPDATE schema_metadata SET version=?", cjsiBaseCoreSchemaVersion)
	for _, commit := range []bool{false, true} {
		tx := beginCJSIFixtureTx(t, core)
		if err := initializeCJSILayoutCore(ctx, tx, root); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE schema_metadata SET version=?", cjsiLayoutCoreSchemaVersion); err != nil {
			t.Fatal(err)
		}
		if commit {
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		} else if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}

		version, err := schemaVersion(ctx, core)
		wantVersion, wantTables := cjsiBaseCoreSchemaVersion, 0
		if commit {
			wantVersion, wantTables = cjsiLayoutCoreSchemaVersion, 1
		}
		var tables int
		if err := core.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'").Scan(&tables); err != nil {
			t.Fatal(err)
		}
		if err != nil || version != wantVersion || tables != wantTables {
			t.Fatalf("version=%d journal tables=%d, commit=%v: %v", version, tables, commit, err)
		}
	}
	j, err := (cjsiSQLiteLayoutJournal{core}).load(ctx)
	if err != nil || j.phase != cjsiFenceInstalling || j.selection().phase != cjsiLayoutStaging {
		t.Fatalf("invalid committed intent: %+v, %v", j, err)
	}
}

func TestCJSIProductionLayoutPrepare(t *testing.T) {
	for _, rebuild := range []bool{false, true} {
		name := "compatible epoch preserved"
		if rebuild {
			name = "legacy rebuild mints epoch"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			path := filepath.Join(t.TempDir(), cjsiPreparedName)
			createCJSIPrivateFixture(t, path)
			db := openCJSIFixtureDB(t, path, "rw")
			schema := cjsiSessionBaselineSchema
			if rebuild {
				schema = `CREATE TABLE session_sources(identity TEXT);
					CREATE TABLE session_metadata(identity TEXT);
					CREATE VIRTUAL TABLE session_documents USING fts5(text)`
			}
			cjsiFixtureExec(t, db, schema+`; CREATE TABLE session_index_generation(singleton INTEGER PRIMARY KEY, epoch INTEGER NOT NULL);
				INSERT INTO session_index_generation VALUES(1,123)`)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			epoch, seal, err := prepareCJSIProductionIndex(ctx, path)
			if err != nil || epoch <= 0 || (epoch != 123) != rebuild || seal.version != 1 {
				t.Fatalf("prepared epoch=%d version=%d, rebuild=%v: %v", epoch, seal.version, rebuild, err)
			}
			if epoch2, seal2, err := prepareCJSIProductionIndex(ctx, path); err != nil || epoch2 != epoch || seal2 != seal {
				t.Fatalf("repeat prepare changed index identity: %d %+v, %v", epoch2, seal2, err)
			}
			if err := cjsiRequireAbsentSidecars(path); err != nil {
				t.Fatalf("prepare left SQLite sidecars: %v", err)
			}

			db = openCJSIFixtureDB(t, path, "rw")
			cjsiFixtureExec(t, db, "UPDATE cjsi_layout_initial_seal SET layout_version=2")
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if _, _, err := prepareCJSIProductionIndex(ctx, path); !errors.Is(err, errCJSILayoutInvalid) {
				t.Fatalf("retry accepted a different layout version: %v", err)
			}
			db = openCJSIFixtureDB(t, path, "rw")
			cjsiFixtureExec(t, db, "UPDATE cjsi_layout_initial_seal SET layout_version=1, index_epoch=index_epoch+1")
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if _, _, err := prepareCJSIProductionIndex(ctx, path); !errors.Is(err, errCJSILayoutInvalid) {
				t.Fatalf("retry accepted an unbound epoch: %v", err)
			}
		})
	}
}

func TestCJSIProductionLayoutResumeAndRetire(t *testing.T) {
	root := t.TempDir()
	ctx := t.Context()
	core := newCJSIProductionCore(t, root)
	t.Cleanup(func() { _ = core.Close() })
	legacy, err := OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	cjsiFixtureExec(t, legacy.DB, "INSERT INTO session_exclusions VALUES('kind','retained')")
	epoch, err := legacy.SessionIndexEpoch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	if err := closeCJSIInstallReadPool(filepath.Join(root, cjsiLegacyIndexName)); err != nil {
		t.Fatal(err)
	}
	seedCJSIProductionIntent(t, root, core)

	journal := cjsiSQLiteLayoutJournal{core}
	install := cjsiLayoutInstall{
		load: journal.load, advance: journal.advance, readSeal: readCJSIProductionSeal,
		prepare: func(ctx context.Context, path string) (int64, cjsiIndexSeal, error) {
			if _, _, err := prepareCJSIProductionIndex(ctx, path); err != nil {
				return 0, cjsiIndexSeal{}, err
			}
			return 0, cjsiIndexSeal{}, errCJSIFixtureInterrupted
		},
	}
	if err := runCJSIInstallFixture(t, root, install); !errors.Is(err, errCJSIFixtureInterrupted) {
		t.Fatalf("interrupt after prepare: %v", err)
	}
	fenced, err := journal.load(ctx)
	if err != nil || fenced.phase != cjsiFenceDurable {
		t.Fatalf("failed prepare advanced the layout: %+v, %v", fenced, err)
	}
	snapshot := filepath.Join(root, cjsiSnapshotName)
	if err := retireCJSILegacySnapshot(ctx, root, fenced); !errors.Is(err, errCJSILayoutInvalid) {
		t.Fatalf("retirement ran while recovery still needed its snapshot: %v", err)
	}
	if err := cjsiVerifyLayoutFile(ctx, snapshot, fenced.snapshot); err != nil {
		t.Fatal(err)
	}

	install.prepare = prepareCJSIProductionIndex
	install.advance = func(ctx context.Context, prior, next cjsiLayoutJournal) error {
		if next.phase == cjsiLayoutReady {
			return errCJSIFixtureInterrupted
		}
		return journal.advance(ctx, prior, next)
	}
	if err := runCJSIInstallFixture(t, root, install); !errors.Is(err, errCJSIFixtureInterrupted) {
		t.Fatalf("interrupt at ready commit: %v", err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	core = openCJSIFixtureDB(t, filepath.Join(root, "agentdeck.sqlite3"), "rw")
	journal = cjsiSQLiteLayoutJournal{core}
	prepared, err := journal.load(ctx)
	if err != nil || prepared.phase != cjsiPrivatePrepared || prepared.epoch != epoch {
		t.Fatalf("reopened wrong preparation state: %+v, %v", prepared, err)
	}
	assertCJSIStagingClosed(t, root, prepared)
	final := filepath.Join(root, "session-index-v1.sqlite3")
	if err := cjsiVerifyLayoutFile(ctx, final, prepared.prepared); err != nil {
		t.Fatal(err)
	}

	// A leased verification pool must block ready commit. The production
	// path may proceed once the caller has drained it.
	_, release, err := acquirePrivateVerifySQL(ctx, final, "identity")
	if err != nil {
		t.Fatal(err)
	}
	if err := runCJSIProductionFixture(t, root, core); !errors.Is(err, ErrStateBusy) {
		t.Fatalf("ready committed with an open private read lease: %v", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	assertCJSIJournal(t, journal, prepared)

	// A foreign sidecar prevents retirement without preventing the
	// verified readiness commit. Remove it and retry the same ready row.
	sidecar := snapshot + "-wal"
	if err := os.WriteFile(sidecar, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := runCJSIProductionFixture(t, root, core); err == nil {
		t.Fatal("retirement ignored a snapshot sidecar")
	}
	ready := prepared
	ready.phase = cjsiLayoutReady
	assertCJSIJournal(t, journal, ready)
	if _, err := os.Stat(snapshot); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(sidecar); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(root, "fixture-original-snapshot")
	if err := os.Rename(snapshot, original); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshot, content, 0600); err != nil {
		t.Fatal(err)
	}
	if err := runCJSIProductionFixture(t, root, core); !errors.Is(err, errCJSILayoutInvalid) {
		t.Fatalf("retirement accepted identical bytes at an unrecorded identity: %v", err)
	}
	if _, err := os.Stat(snapshot); err != nil {
		t.Fatalf("failed retirement removed replacement: %v", err)
	}
	if err := os.Rename(original, snapshot); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := runCJSIProductionFixture(t, root, core); err != nil {
			t.Fatal(err)
		}
		if processReadPools.contains(final) {
			t.Fatal("installation retained a private verification pool")
		}
		if err := cjsiRequireCleanIndexPath(snapshot); err != nil {
			t.Fatal(err)
		}
		assertCJSIJournal(t, journal, ready)
	}
	db := openCJSIFixtureDB(t, final, "ro")
	defer db.Close()
	var exclusion string
	if err := db.QueryRowContext(ctx, "SELECT value FROM session_exclusions").Scan(&exclusion); err != nil || exclusion != "retained" {
		t.Fatalf("legacy contents were not retained: %q, %v", exclusion, err)
	}
}

func TestCJSIProductionLayoutNeverBuilt(t *testing.T) {
	root := t.TempDir()
	core := newCJSIProductionCore(t, root)
	seedCJSIProductionIntent(t, root, core)
	for range 2 {
		if err := runCJSIProductionFixture(t, root, core); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{cjsiSnapshotName, cjsiPreparedName, cjsiPrivateIndexName} {
			if err := cjsiRequireCleanIndexPath(filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
		}
		j, err := (cjsiSQLiteLayoutJournal{core}).load(t.Context())
		if err != nil || j.phase != cjsiLayoutReady || j.selection().phase != cjsiLayoutNeverBuilt {
			t.Fatalf("never-built root got unexpected readiness: %+v, %v", j, err)
		}
	}
}

func newCJSIProductionCore(t *testing.T, root string) *sql.DB {
	t.Helper()
	path := filepath.Join(root, "agentdeck.sqlite3")
	createCJSIPrivateFixture(t, path)
	db := openCJSIFixtureDB(t, path, "rw")
	t.Cleanup(func() { _ = db.Close() })
	cjsiFixtureExec(t, db, `PRAGMA journal_mode=WAL;
		CREATE TABLE schema_metadata(singleton INTEGER PRIMARY KEY CHECK(singleton=1), version INTEGER NOT NULL);
		INSERT INTO schema_metadata VALUES(1,36)`)
	return db
}

func seedCJSIProductionIntent(t *testing.T, root string, core *sql.DB) {
	t.Helper()
	cjsiFixtureExec(t, core, "UPDATE schema_metadata SET version=?", cjsiBaseCoreSchemaVersion)
	tx := beginCJSIFixtureTx(t, core)
	if err := initializeCJSILayoutCore(t.Context(), tx, root); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(t.Context(), "UPDATE schema_metadata SET version=?", cjsiLayoutCoreSchemaVersion); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func createCJSIPrivateFixture(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func runCJSIProductionFixture(t *testing.T, root string, core *sql.DB) error {
	t.Helper()
	scan, err := AcquireScanLock(t.Context(), root, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer scan.Release()
	state, err := AcquireLock(t.Context(), root, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer state.Release()
	return resumeCJSIProductionLayout(t.Context(), root, core)
}
