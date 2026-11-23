package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

var errCJSIFixtureInterrupted = errors.New("interrupted layout install")

func TestCJSIFencedStateLayoutRecovery(t *testing.T) {
	t.Run("committed WAL and restart at every journal boundary", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "state #?")
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		initCJSILegacyWAL(t, root)
		fixture := newCJSIJournalFixture(t, root)

		for _, step := range []struct {
			phase         cjsiInstallPhase
			commit        bool
			fenced, final bool
		}{
			{phase: cjsiLegacyCaptured},
			{phase: cjsiLegacyCaptured, commit: true},
			{phase: cjsiFenceDurable, fenced: true},
			{phase: cjsiFenceDurable, fenced: true, commit: true},
			{phase: cjsiPrivatePrepared, fenced: true},
			{phase: cjsiPrivatePrepared, fenced: true, commit: true},
			{phase: cjsiLayoutReady, fenced: true, final: true},
		} {
			install := fixture.install()
			advance := install.advance
			install.advance = func(ctx context.Context, old, next cjsiLayoutJournal) error {
				if next.phase != step.phase {
					t.Fatalf("unexpected transition %d -> %d, want %d", old.phase, next.phase, step.phase)
				}
				if step.commit {
					if err := advance(ctx, old, next); err != nil {
						return err
					}
				}
				return errCJSIFixtureInterrupted
			}
			if err := runCJSIInstallFixture(t, root, install); !errors.Is(err, errCJSIFixtureInterrupted) {
				t.Fatalf("interrupt before/after phase %d commit=%v: %v", step.phase, step.commit, err)
			}

			persisted := fixture.journal(t)
			if (persisted.phase == step.phase) != step.commit {
				t.Fatalf("journal phase=%d after interrupt at %d, commit=%v", persisted.phase, step.phase, step.commit)
			}
			legacy := filepath.Join(root, cjsiLegacyIndexName)
			if step.fenced {
				if _, err := validateCJSIFence(root); err != nil {
					t.Fatal(err)
				}
				if err := cjsiRequireAbsentSidecars(legacy); err != nil {
					t.Fatalf("legacy sidecars survived fence step: %v", err)
				}
			} else {
				assertCJSIFixtureRows(t, legacy)
				if err := cjsiCheckEmptyLayoutSidecar(legacy + "-wal"); err != nil {
					t.Fatalf("legacy WAL was not checkpointed before the fence: %v", err)
				}
			}
			snapshot := filepath.Join(root, cjsiSnapshotName)
			assertCJSIFixtureRows(t, snapshot)
			if err := cjsiRequireAbsentSidecars(snapshot); err != nil {
				t.Fatalf("snapshot depended on sidecars: %v", err)
			}

			final := filepath.Join(root, cjsiPrivateIndexName)
			if step.final {
				assertCJSIFixtureRows(t, final)
				if err := cjsiRequireCleanIndexPath(filepath.Join(root, cjsiPreparedName)); err != nil {
					t.Fatalf("staging name still exposed the installed index: %v", err)
				}
			} else if err := cjsiRequireCleanIndexPath(final); err != nil {
				t.Fatalf("premature private installation: %v", err)
			}
			assertCJSIStagingClosed(t, root, fixture.journal(t))
		}

		install := fixture.install()
		install.prepare = nil // Resume uses the verified installed image.
		if err := runCJSIInstallFixture(t, root, install); err != nil {
			t.Fatal(err)
		}
		j := fixture.journal(t)
		got, err := resolveCJSILayout(t.Context(), root, j.selection(), readCJSIFixtureSeal)
		if j.phase != cjsiLayoutReady || err != nil || got.path != filepath.Join(root, cjsiPrivateIndexName) {
			t.Fatalf("ready layout did not resolve: phase=%d, resolution=%+v, error=%v", j.phase, got, err)
		}
		journalBefore := j
		if err := runCJSIInstallFixture(t, root, install); err != nil || fixture.journal(t) != journalBefore {
			t.Fatalf("re-entering ready changed journal state: %v", err)
		}
	})

	t.Run("restart with a renamed fence and residual sidecars", func(t *testing.T) {
		root := t.TempDir()
		initCJSILegacyWAL(t, root)
		fixture := newCJSIJournalFixture(t, root)
		install := fixture.install()
		advance := install.advance
		install.advance = func(ctx context.Context, old, next cjsiLayoutJournal) error {
			if err := advance(ctx, old, next); err != nil {
				return err
			}
			return errCJSIFixtureInterrupted
		}
		if err := runCJSIInstallFixture(t, root, install); !errors.Is(err, errCJSIFixtureInterrupted) {
			t.Fatal(err)
		}

		// Recreate the filesystem boundary after rename but before the
		// legacy sidecar removals and directory sync.
		legacy := filepath.Join(root, cjsiLegacyIndexName)
		tmp := filepath.Join(root, cjsiFenceStage)
		if err := cjsiWriteFence(tmp); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(tmp, legacy); err != nil {
			t.Fatal(err)
		}
		fence, err := validateCJSIFence(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, suffix := range cjsiSidecarSuffixes {
			if err := os.WriteFile(legacy+suffix, nil, 0600); err != nil {
				t.Fatal(err)
			}
		}
		install = fixture.install()
		advance = install.advance
		install.advance = func(ctx context.Context, old, next cjsiLayoutJournal) error {
			if err := cjsiRequireAbsentSidecars(legacy); err != nil {
				t.Fatalf("sidecars survived through phase %d: %v", next.phase, err)
			}
			return advance(ctx, old, next)
		}
		if err := runCJSIInstallFixture(t, root, install); err != nil {
			t.Fatal(err)
		}
		j := fixture.journal(t)
		if j.phase != cjsiLayoutReady || j.fence != fence {
			t.Fatalf("resume replaced existing verified fence: phase=%d fence=%+v", j.phase, j.fence)
		}
	})

	t.Run("never-built stays without a private database", func(t *testing.T) {
		root := t.TempDir()
		legacy := filepath.Join(root, cjsiLegacyIndexName)
		if err := os.WriteFile(legacy+"-wal", nil, 0600); err != nil {
			t.Fatal(err)
		}
		fixture := newCJSIJournalFixture(t, root)
		install := fixture.install()
		install.prepare, install.readSeal = nil, nil
		if err := runCJSIInstallFixture(t, root, install); err != nil {
			t.Fatal(err)
		}
		j := fixture.journal(t)
		if j.phase != cjsiLayoutReady || j.selection().phase != cjsiLayoutNeverBuilt {
			t.Fatalf("wrong never-built state: %+v", j)
		}
		for _, name := range []string{cjsiSnapshotName, cjsiPreparedName, cjsiPrivateIndexName} {
			if err := cjsiRequireCleanIndexPath(filepath.Join(root, name)); err != nil {
				t.Fatalf("%s created for absent legacy index: %v", name, err)
			}
		}
		if err := cjsiRequireAbsentSidecars(legacy); err != nil {
			t.Fatal(err)
		}
		if err := cjsiProbeFence(t.Context(), legacy); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("failed verification leaves the last durable phase closed", func(t *testing.T) {
		for _, test := range []struct {
			name  string
			phase cjsiInstallPhase
			file  string
		}{
			{"changed legacy before fence", cjsiLegacyCaptured, cjsiLegacyIndexName},
			{"changed captured snapshot", cjsiLegacyCaptured, cjsiSnapshotName},
			{"changed durable fence", cjsiFenceDurable, cjsiLegacyIndexName},
			{"changed prepared copy", cjsiPrivatePrepared, cjsiPreparedName},
			{"unexpected final sidecar", cjsiPrivatePrepared, cjsiPrivateIndexName + "-wal"},
		} {
			t.Run(test.name, func(t *testing.T) {
				root := t.TempDir()
				initCJSILegacyWAL(t, root)
				fixture := newCJSIJournalFixture(t, root)
				install := fixture.install()
				advance := install.advance
				install.advance = func(ctx context.Context, old, next cjsiLayoutJournal) error {
					if err := advance(ctx, old, next); err != nil {
						return err
					}
					if next.phase == test.phase {
						return errCJSIFixtureInterrupted
					}
					return nil
				}
				if err := runCJSIInstallFixture(t, root, install); !errors.Is(err, errCJSIFixtureInterrupted) {
					t.Fatal(err)
				}

				path := filepath.Join(root, test.file)
				file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, err = file.WriteString("fixture mutation")
				if err := errors.Join(err, file.Close()); err != nil {
					t.Fatal(err)
				}
				if err := runCJSIInstallFixture(t, root, fixture.install()); err == nil {
					t.Fatal("corrupt layout became ready")
				}
				if j := fixture.journal(t); j.phase != test.phase {
					t.Fatalf("failed verification advanced journal: %d -> %d", test.phase, j.phase)
				}
				if err := cjsiRequireAbsent(filepath.Join(root, cjsiPrivateIndexName)); err != nil {
					t.Fatalf("failed verification installed a private index: %v", err)
				}
				assertCJSIStagingClosed(t, root, fixture.journal(t))
			})
		}
	})

	t.Run("busy legacy writer and uncertain absence fail closed", func(t *testing.T) {
		for _, scenario := range []string{"exclusive writer", "WAL writer", "orphan WAL", "rollback journal"} {
			t.Run(scenario, func(t *testing.T) {
				root := t.TempDir()
				legacy := filepath.Join(root, cjsiLegacyIndexName)
				if scenario == "orphan WAL" {
					if err := os.WriteFile(legacy+"-wal", []byte("unverified legacy WAL"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					db := openCJSIFixtureDB(t, legacy, "rwc")
					defer db.Close()
					if _, err := db.Exec(`CREATE TABLE session_sources(value TEXT); CREATE TABLE session_metadata(value TEXT)`); err != nil {
						t.Fatal(err)
					}
					if err := os.Chmod(legacy, 0600); err != nil {
						t.Fatal(err)
					}
					if scenario == "exclusive writer" {
						if _, err := db.Exec("BEGIN EXCLUSIVE"); err != nil {
							t.Fatal(err)
						}
						defer db.Exec("ROLLBACK")
					} else if scenario == "WAL writer" {
						if _, err := db.Exec("PRAGMA journal_mode=WAL; BEGIN IMMEDIATE; INSERT INTO session_sources VALUES('unfinished')"); err != nil {
							t.Fatal(err)
						}
						defer db.Exec("ROLLBACK")
					} else if err := os.WriteFile(legacy+"-journal", []byte("unverified rollback journal"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				fixture := newCJSIJournalFixture(t, root)
				err := runCJSIInstallFixture(t, root, fixture.install())
				if err == nil {
					t.Fatal("unverified source became ready")
				}
				if scenario == "WAL writer" && !errors.Is(err, ErrStateBusy) {
					t.Fatalf("legacy checkpoint did not refuse a live WAL writer: %v", err)
				}
				if j := fixture.journal(t); j.phase != cjsiFenceInstalling {
					t.Fatalf("unverified source advanced journal to %d", j.phase)
				}
				if _, err := validateCJSIFence(root); err == nil {
					t.Fatal("unverified source was fenced")
				}
			})
		}
	})

	t.Run("private seal mismatch remains fenced and retries from snapshot", func(t *testing.T) {
		root := t.TempDir()
		initCJSILegacyWAL(t, root)
		fixture := newCJSIJournalFixture(t, root)
		install := fixture.install()
		prepare := install.prepare
		install.prepare = func(ctx context.Context, path string) (int64, cjsiIndexSeal, error) {
			epoch, seal, err := prepare(ctx, path)
			seal.digest[0] ^= 1
			return epoch, seal, err
		}
		if err := runCJSIInstallFixture(t, root, install); !errors.Is(err, errCJSILayoutInvalid) {
			t.Fatalf("seal mismatch: %v", err)
		}
		if j := fixture.journal(t); j.phase != cjsiFenceDurable {
			t.Fatalf("seal mismatch advanced journal to %d", j.phase)
		}
		assertCJSIStagingClosed(t, root, fixture.journal(t))
		if err := runCJSIInstallFixture(t, root, fixture.install()); err != nil {
			t.Fatalf("verified capture could not rebuild failed preparation: %v", err)
		}
	})
}

func TestCJSIFencedStateLayoutWALFixture(t *testing.T) {
	path := os.Getenv("AGENTDECK_CJSI_TEST_WAL_PATH")
	if path == "" {
		t.Skip("subprocess fixture")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	db := openCJSIFixtureDB(t, path, "rw")
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0;
		CREATE TABLE session_sources(value TEXT);
		CREATE TABLE session_metadata(value TEXT);
		INSERT INTO session_sources VALUES('committed');
		BEGIN IMMEDIATE;
		INSERT INTO session_sources VALUES('uncommitted')`); err != nil {
		t.Fatal(err)
	}
	// Leave a real WAL and SHM after process exit, with no surviving SQLite
	// handle. A normal close could checkpoint away the test's committed WAL.
	os.Exit(0)
}

func initCJSILegacyWAL(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, cjsiLegacyIndexName)
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCJSIFencedStateLayoutWALFixture$")
	cmd.Env = append(os.Environ(), "AGENTDECK_CJSI_TEST_WAL_PATH="+path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create legacy WAL: %v\n%s", err, output)
	}
	if stat, err := os.Stat(path + "-wal"); err != nil || stat.Size() == 0 {
		t.Fatalf("fixture did not leave a committed WAL: %v", err)
	}
}

func openCJSIFixtureDB(t *testing.T, path, mode string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", cjsiSQLiteURI(path, mode))
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertCJSIFixtureRows(t *testing.T, path string) {
	t.Helper()
	db := openCJSIFixtureDB(t, path, "ro")
	defer db.Close()
	var n int
	var value string
	if err := db.QueryRow("SELECT count(*), max(value) FROM session_sources").Scan(&n, &value); err != nil || n != 1 || value != "committed" {
		t.Fatalf("committed legacy view: %d %q, %v", n, value, err)
	}
}

func assertCJSIStagingClosed(t *testing.T, root string, j cjsiLayoutJournal) {
	t.Helper()
	got, err := resolveCJSILayout(t.Context(), root, j.selection(), readCJSIFixtureSeal)
	if err != nil || got != (cjsiLayoutResolution{phase: cjsiLayoutStaging}) {
		t.Fatalf("closed layout exposed a path: %+v, %v", got, err)
	}
}

func runCJSIInstallFixture(t *testing.T, root string, install cjsiLayoutInstall) error {
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
	return resumeCJSILayoutInstall(t.Context(), root, install)
}

type cjsiJournalFixture struct{ path string }

func newCJSIJournalFixture(t *testing.T, root string) cjsiJournalFixture {
	t.Helper()
	stat, err := privateServingPath(root, true)
	if err != nil {
		t.Fatal(err)
	}
	fixture := cjsiJournalFixture{path: filepath.Join(t.TempDir(), "fixture-core.sqlite3")}
	db := openCJSIFixtureDB(t, fixture.path, "rwc")
	defer db.Close()
	raw, err := json.Marshal(cjsiJournalFixtureWire(cjsiLayoutJournal{
		phase: cjsiFenceInstalling, root: cjsiIdentity(stat),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE fixture_core_journal(record BLOB NOT NULL);
		INSERT INTO fixture_core_journal VALUES(?)`, raw); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f cjsiJournalFixture) install() cjsiLayoutInstall {
	return cjsiLayoutInstall{
		load: func(ctx context.Context) (cjsiLayoutJournal, error) {
			db, err := sql.Open("sqlite", cjsiSQLiteURI(f.path, "ro"))
			if err != nil {
				return cjsiLayoutJournal{}, err
			}
			defer db.Close()
			var raw []byte
			if err := db.QueryRowContext(ctx, "SELECT record FROM fixture_core_journal").Scan(&raw); err != nil {
				return cjsiLayoutJournal{}, err
			}
			var wire cjsiFixtureJournal
			if err := json.Unmarshal(raw, &wire); err != nil {
				return cjsiLayoutJournal{}, err
			}
			return wire.journal(), nil
		},
		advance: func(ctx context.Context, old, next cjsiLayoutJournal) error {
			db, err := sql.Open("sqlite", cjsiSQLiteURI(f.path, "rw"))
			if err != nil {
				return err
			}
			defer db.Close()
			prev, err := json.Marshal(cjsiJournalFixtureWire(old))
			if err != nil {
				return err
			}
			record, err := json.Marshal(cjsiJournalFixtureWire(next))
			if err != nil {
				return err
			}
			res, err := db.ExecContext(ctx, "UPDATE fixture_core_journal SET record=? WHERE record=?", record, prev)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil || n != 1 {
				return errors.Join(errCJSILayoutInvalid, err)
			}
			return nil
		},
		prepare: func(ctx context.Context, path string) (int64, cjsiIndexSeal, error) {
			db, err := sql.Open("sqlite", cjsiSQLiteURI(path, "rw"))
			if err != nil {
				return 0, cjsiIndexSeal{}, err
			}
			defer db.Close()
			if err := ensureSessionIndexGeneration(ctx, db); err != nil {
				return 0, cjsiIndexSeal{}, err
			}
			epoch, err := (&Store{DB: db}).SessionIndexEpoch(ctx)
			if err != nil {
				return 0, cjsiIndexSeal{}, err
			}
			seal := cjsiIndexSeal{version: 7, digest: sha256.Sum256([]byte("fixture migration seal"))}
			_, err = db.ExecContext(ctx, `CREATE TABLE fixture_index_seal(version INTEGER, digest BLOB);
				INSERT INTO fixture_index_seal VALUES(?,?)`, seal.version, seal.digest[:])
			return epoch, seal, err
		},
		readSeal: readCJSIFixtureSeal,
	}
}

func (f cjsiJournalFixture) journal(t *testing.T) cjsiLayoutJournal {
	t.Helper()
	j, err := f.install().load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return j
}

type cjsiFixtureJournal struct {
	Phase             cjsiInstallPhase
	Root, Fence       [2]uint64
	Main, Image, Next cjsiFixtureFile
	Epoch             int64
	Version           int
	Seal              [32]byte
}

type cjsiFixtureFile struct {
	Identity [2]uint64
	Size     int64
	Digest   [32]byte
}

func cjsiJournalFixtureWire(j cjsiLayoutJournal) cjsiFixtureJournal {
	file := func(f cjsiLayoutFile) cjsiFixtureFile {
		return cjsiFixtureFile{[2]uint64{f.identity.device, f.identity.inode}, f.size, f.digest}
	}
	return cjsiFixtureJournal{
		Phase: j.phase, Root: [2]uint64{j.root.device, j.root.inode}, Fence: [2]uint64{j.fence.device, j.fence.inode},
		Main: file(j.legacy), Image: file(j.snapshot), Next: file(j.prepared),
		Epoch: j.epoch, Version: j.seal.version, Seal: j.seal.digest,
	}
}

func (w cjsiFixtureJournal) journal() cjsiLayoutJournal {
	file := func(f cjsiFixtureFile) cjsiLayoutFile {
		return cjsiLayoutFile{cjsiFileIdentity{f.Identity[0], f.Identity[1]}, f.Size, f.Digest}
	}
	return cjsiLayoutJournal{
		phase: w.Phase, root: cjsiFileIdentity{w.Root[0], w.Root[1]}, fence: cjsiFileIdentity{w.Fence[0], w.Fence[1]},
		legacy: file(w.Main), snapshot: file(w.Image), prepared: file(w.Next),
		epoch: w.Epoch, seal: cjsiIndexSeal{version: w.Version, digest: w.Seal},
	}
}
