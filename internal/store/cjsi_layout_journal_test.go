package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"math"
	"net/url"
	"path/filepath"
	"testing"
)

func TestCJSILayoutSQLiteJournal(t *testing.T) {
	t.Run("schema ahead and intent share the caller transaction", func(t *testing.T) {
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), "core.sqlite3")
		db := openCJSIFixtureDB(t, path, "rwc")
		defer func() { _ = db.Close() }()
		cjsiFixtureExec(t, db, "CREATE TABLE fixture_core_schema(ahead INTEGER); INSERT INTO fixture_core_schema VALUES(0)")
		root := cjsiFileIdentity{device: math.MaxUint64, inode: 1 << 63}
		for _, commit := range []bool{false, true} {
			tx := beginCJSIFixtureTx(t, db)
			if _, err := tx.ExecContext(ctx, "UPDATE fixture_core_schema SET ahead=1; "+cjsiLayoutJournalSchema); err != nil {
				t.Fatal(err)
			}
			if err := initializeCJSILayoutJournal(ctx, tx, root); err != nil {
				t.Fatal(err)
			}
			if err := initializeCJSILayoutJournal(ctx, tx, root); err == nil {
				t.Fatal("initialization replaced an existing intent")
			}
			finish := tx.Rollback
			if commit {
				finish = tx.Commit
			}
			if err := finish(); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db = openCJSIFixtureDB(t, path, "rw")
			var ahead, tables int
			if err := db.QueryRow("SELECT ahead FROM fixture_core_schema").Scan(&ahead); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_journal'").Scan(&tables); err != nil {
				t.Fatal(err)
			}
			if (ahead == 1) != commit || (tables == 1) != commit {
				t.Fatalf("schema-ahead=%d, journal table=%d after commit=%v", ahead, tables, commit)
			}
			if commit {
				assertCJSIJournal(t, cjsiSQLiteLayoutJournal{db}, cjsiLayoutJournal{phase: cjsiFenceInstalling, root: root})
			}
		}
	})

	t.Run("full-record CAS and monotonic transitions", func(t *testing.T) {
		ctx := t.Context()
		db := openCJSIFixtureDB(t, filepath.Join(t.TempDir(), "core.sqlite3"), "rwc")
		defer db.Close()
		rows := cjsiJournalTestRecords()
		store := createCJSIJournalFixture(t, db, rows[0].root)
		cjsiFixtureExec(t, db, "UPDATE cjsi_layout_journal SET legacy_file=?", cjsiEncodeLayoutFile(rows[1].legacy))
		if err := store.advance(ctx, rows[0], rows[1]); !errors.Is(err, errCJSILayoutJournalStale) {
			t.Fatalf("CAS ignored that an absent field was populated: %v", err)
		}
		cjsiFixtureExec(t, db, "UPDATE cjsi_layout_journal SET legacy_file=NULL")
		for i := 1; i < len(rows)-1; i++ {
			if err := store.advance(ctx, rows[i-1], rows[i]); err != nil {
				t.Fatal(err)
			}
		}
		prepared, ready := rows[3], rows[4]
		for _, item := range []struct {
			name string
			edit func(*cjsiLayoutJournal)
		}{
			{"root identity", func(j *cjsiLayoutJournal) { j.root.device++ }},
			{"legacy file", func(j *cjsiLayoutJournal) { j.legacy.identity.inode-- }},
			{"snapshot file", func(j *cjsiLayoutJournal) { j.snapshot.digest[0] ^= 1 }},
			{"fence identity", func(j *cjsiLayoutJournal) { j.fence.inode++ }},
			{"prepared file", func(j *cjsiLayoutJournal) { j.prepared.size-- }},
			{"epoch", func(j *cjsiLayoutJournal) { j.epoch-- }},
			{"seal version", func(j *cjsiLayoutJournal) { j.seal.version-- }},
			{"seal digest", func(j *cjsiLayoutJournal) { j.seal.digest[0] ^= 1 }},
		} {
			t.Run(item.name, func(t *testing.T) {
				stale := prepared
				item.edit(&stale)
				staleReady := stale
				staleReady.phase = cjsiLayoutReady
				if err := store.advance(ctx, stale, staleReady); !errors.Is(err, errCJSILayoutJournalStale) {
					t.Fatalf("full prior record mismatch was not stale: %v", err)
				}
				if err := store.advance(ctx, prepared, staleReady); !errors.Is(err, errCJSILayoutInvalid) {
					t.Fatalf("transition allowed changing recorded metadata: %v", err)
				}
				assertCJSIJournal(t, store, prepared)
			})
		}
		for _, update := range [][2]cjsiLayoutJournal{{rows[0], ready}, {prepared, rows[2]}, {prepared, prepared}} {
			if err := store.advance(ctx, update[0], update[1]); !errors.Is(err, errCJSILayoutInvalid) {
				t.Fatalf("illegal transition succeeded: %v", err)
			}
		}

		cjsiFixtureExec(t, db, `CREATE TRIGGER fixture_fail_update BEFORE UPDATE ON cjsi_layout_journal
			BEGIN SELECT RAISE(ABORT, 'fixture write failure'); END`)
		if err := store.advance(ctx, prepared, ready); err == nil || errors.Is(err, errCJSILayoutJournalStale) {
			t.Fatalf("SQLite write failure was not propagated: %v", err)
		}
		assertCJSIJournal(t, store, prepared)
		cjsiFixtureExec(t, db, "DROP TRIGGER fixture_fail_update")
		if err := store.advance(ctx, prepared, ready); err != nil {
			t.Fatal(err)
		}
		if err := store.advance(ctx, prepared, ready); !errors.Is(err, errCJSILayoutJournalStale) {
			t.Fatalf("repeated CAS should be stale: %v", err)
		}
		assertCJSIJournal(t, store, ready)
	})

	t.Run("durable journal configuration required on the writing connection", func(t *testing.T) {
		db := openCJSIFixtureDB(t, filepath.Join(t.TempDir(), "core.sqlite3"), "rwc")
		defer db.Close()
		db.SetMaxOpenConns(1)
		ctx := t.Context()
		rows := cjsiJournalTestRecords()
		cjsiFixtureExec(t, db, "PRAGMA synchronous=NORMAL")
		tx := beginCJSIFixtureTx(t, db)
		if _, err := tx.ExecContext(ctx, cjsiLayoutJournalSchema); err != nil {
			t.Fatal(err)
		}
		if err := initializeCJSILayoutJournal(ctx, tx, rows[0].root); !errors.Is(err, errCJSILayoutInvalid) {
			t.Fatalf("initialized asynchronous intent: %v", err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		cjsiFixtureExec(t, db, "PRAGMA synchronous=FULL")
		store := createCJSIJournalFixture(t, db, rows[0].root)
		cjsiFixtureExec(t, db, "PRAGMA journal_mode=MEMORY")
		if err := store.advance(ctx, rows[0], rows[1]); !errors.Is(err, errCJSILayoutInvalid) {
			t.Fatalf("advanced journal with nondurable rollback log: %v", err)
		}
		assertCJSIJournal(t, store, rows[0])
	})

	t.Run("corrupt journal evidence fails closed", func(t *testing.T) {
		rows := cjsiJournalTestRecords()
		overflowSize := append([]byte(nil), cjsiEncodeLayoutFile(rows[3].prepared).([]byte)...)
		binary.BigEndian.PutUint64(overflowSize[16:24], 1<<63)
		for _, item := range []struct {
			name, update string
			values       []any
		}{
			{"phase wraps to ready if narrowed", "UPDATE cjsi_layout_journal SET phase=261", nil},
			{"zero root inode", "UPDATE cjsi_layout_journal SET root_identity=zeroblob(16)", nil},
			{"truncated identity", "UPDATE cjsi_layout_journal SET fence_identity=zeroblob(15)", nil},
			{"overflow file size", "UPDATE cjsi_layout_journal SET prepared_file=?", []any{overflowSize}},
			{"absent snapshot", "UPDATE cjsi_layout_journal SET snapshot_file=NULL", nil},
			{"present field before its phase", "UPDATE cjsi_layout_journal SET phase=1", nil},
			{"partial seal", "UPDATE cjsi_layout_journal SET index_epoch=NULL", nil},
			{"invalid epoch", "UPDATE cjsi_layout_journal SET index_epoch=0", nil},
			{"zero digest", "UPDATE cjsi_layout_journal SET seal_digest=zeroblob(32)", nil},
			{"missing singleton", "DELETE FROM cjsi_layout_journal", nil},
		} {
			t.Run(item.name, func(t *testing.T) {
				db := openCJSIFixtureDB(t, filepath.Join(t.TempDir(), "core.sqlite3"), "rwc")
				defer db.Close()
				db.SetMaxOpenConns(1)
				store := createCJSIJournalFixture(t, db, rows[0].root)
				for i := 1; i < len(rows); i++ {
					if err := store.advance(t.Context(), rows[i-1], rows[i]); err != nil {
						t.Fatal(err)
					}
				}
				cjsiFixtureExec(t, db, "PRAGMA ignore_check_constraints=ON")
				cjsiFixtureExec(t, db, item.update, item.values...)
				if got, err := store.load(t.Context()); !errors.Is(err, errCJSILayoutInvalid) || got != (cjsiLayoutJournal{}) {
					t.Fatalf("corrupt journal returned evidence: %+v, %v", got, err)
				}
			})
		}
	})
}

func TestCJSILayoutSQLiteJournalConcurrentCAS(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "core.sqlite3")
	dsn := url.URL{
		Scheme:   "file",
		Path:     path,
		RawQuery: "mode=rwc&_pragma=busy_timeout(5000)&_pragma=synchronous(FULL)",
	}
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Contenders should observe the winning commit and become stale.
	var busy int
	if err := db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil || busy != 5000 {
		t.Fatalf("CAS fixture busy_timeout=%d, want 5000: %v", busy, err)
	}
	cjsiFixtureExec(t, db, "PRAGMA journal_mode=WAL")
	rows := cjsiJournalTestRecords()
	journal := createCJSIJournalFixture(t, db, rows[0].root)
	const contenders = 24
	start := make(chan struct{})
	results := make(chan error, contenders)
	for range contenders {
		go func() {
			<-start
			results <- journal.advance(ctx, rows[0], rows[1])
		}()
	}
	close(start)
	var winners, stale int
	for range contenders {
		switch err := <-results; {
		case err == nil:
			winners++
		case errors.Is(err, errCJSILayoutJournalStale):
			stale++
		default:
			t.Errorf("unexpected CAS outcome: %v", err)
		}
	}
	if winners != 1 || stale != contenders-1 {
		t.Fatalf("CAS committed %d winners and returned %d stale results", winners, stale)
	}
	assertCJSIJournal(t, journal, rows[1])
}

func TestCJSILayoutSQLiteAdaptersResume(t *testing.T) {
	for _, populated := range []bool{true, false} {
		name := "never-built"
		if populated {
			name = "committed WAL"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if populated {
				initCJSILegacyWAL(t, root)
			}
			stat, err := privateServingPath(root, true)
			if err != nil {
				t.Fatal(err)
			}
			core := filepath.Join(t.TempDir(), "core.sqlite3")
			db := openCJSIFixtureDB(t, core, "rwc")
			defer func() { _ = db.Close() }()
			cjsiFixtureExec(t, db, "PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL")
			store := createCJSIJournalFixture(t, db, cjsiIdentity(stat))
			install := cjsiLayoutInstall{
				load: store.load, readSeal: readCJSIIndexSeal,
				prepare: func(ctx context.Context, path string) (int64, cjsiIndexSeal, error) {
					if !populated {
						t.Fatal("never-built state attempted private preparation")
					}
					return prepareCJSISealFixture(ctx, path)
				},
				advance: func(ctx context.Context, old, next cjsiLayoutJournal) error {
					if next.phase == cjsiLayoutReady {
						return errCJSIFixtureInterrupted
					}
					return store.advance(ctx, old, next)
				},
			}
			if err := runCJSIInstallFixture(t, root, install); !errors.Is(err, errCJSIFixtureInterrupted) {
				t.Fatalf("interrupt before layout ready: %v", err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db = openCJSIFixtureDB(t, core, "rw")
			store = cjsiSQLiteLayoutJournal{db}
			persisted, err := store.load(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			wantPhase := cjsiFenceDurable
			if populated {
				wantPhase = cjsiPrivatePrepared
			}
			if persisted.phase != wantPhase {
				t.Fatalf("reopened phase=%d, want %d", persisted.phase, wantPhase)
			}
			assertCJSIStagingClosed(t, root, persisted)
			install.load, install.advance, install.prepare = store.load, store.advance, nil
			if err := runCJSIInstallFixture(t, root, install); err != nil {
				t.Fatal(err)
			}
			ready := persisted
			ready.phase = cjsiLayoutReady
			assertCJSIJournal(t, store, ready)
			got, err := resolveCJSILayout(t.Context(), root, ready.selection(), readCJSIIndexSeal)
			final := filepath.Join(root, cjsiPrivateIndexName)
			if populated {
				if err != nil || got.path != final {
					t.Fatalf("ready index did not resolve: %+v, %v", got, err)
				}
				assertCJSIFixtureRows(t, final)
			} else if err != nil || got.phase != cjsiLayoutNeverBuilt || got.path != "" {
				t.Fatalf("unexpected never-built resolution: %+v, %v", got, err)
			} else if err := cjsiRequireCleanIndexPath(final); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func cjsiJournalTestRecords() []cjsiLayoutJournal {
	file := cjsiLayoutFile{
		identity: cjsiFileIdentity{device: math.MaxUint64, inode: math.MaxUint64},
		size:     math.MaxInt64, digest: sha256.Sum256([]byte("layout file fixture")),
	}
	intent := cjsiLayoutJournal{phase: cjsiFenceInstalling, root: cjsiFileIdentity{device: 3, inode: 14}}
	captured := intent
	captured.phase, captured.legacy, captured.snapshot = cjsiLegacyCaptured, file, file
	fenced := captured
	fenced.phase, fenced.fence = cjsiFenceDurable, cjsiFileIdentity{device: 3, inode: 23}
	prepared := fenced
	prepared.phase, prepared.prepared, prepared.epoch = cjsiPrivatePrepared, file, math.MaxInt64
	prepared.seal = cjsiIndexSeal{version: math.MaxInt, digest: sha256.Sum256([]byte("layout seal fixture"))}
	ready := prepared
	ready.phase = cjsiLayoutReady
	return []cjsiLayoutJournal{intent, captured, fenced, prepared, ready}
}

func createCJSIJournalFixture(t *testing.T, db *sql.DB, root cjsiFileIdentity) cjsiSQLiteLayoutJournal {
	t.Helper()
	tx := beginCJSIFixtureTx(t, db)
	if _, err := tx.ExecContext(t.Context(), cjsiLayoutJournalSchema); err != nil {
		t.Fatal(err)
	}
	if err := initializeCJSILayoutJournal(t.Context(), tx, root); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return cjsiSQLiteLayoutJournal{db}
}

func assertCJSIJournal(t *testing.T, store cjsiSQLiteLayoutJournal, want cjsiLayoutJournal) {
	t.Helper()
	if got, err := store.load(t.Context()); err != nil || got != want {
		t.Fatalf("journal = %+v, want %+v, error=%v", got, want, err)
	}
}

func beginCJSIFixtureTx(t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

func cjsiFixtureExec(t *testing.T, db *sql.DB, statement string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), statement, args...); err != nil {
		t.Fatal(err)
	}
}
