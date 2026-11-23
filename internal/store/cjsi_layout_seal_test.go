package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

const cjsiFixtureIndexEpoch = int64(123)

var cjsiFixtureInitialSeal = cjsiIndexSeal{
	version: 7,
	digest:  sha256.Sum256([]byte("private index initial seal fixture")),
}

func TestCJSILayoutSQLiteSeal(t *testing.T) {
	t.Run("initial seal is transactional and bound to the supplied epoch", func(t *testing.T) {
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), "private.sqlite3")
		db := openCJSIFixtureDB(t, path, "rwc")
		defer db.Close()
		tx := beginCJSIFixtureTx(t, db)
		if err := createCJSISealFixtureSchema(ctx, tx); err != nil {
			t.Fatal(err)
		}
		if err := initializeCJSIIndexSeal(ctx, tx, cjsiFixtureIndexEpoch+1, cjsiFixtureInitialSeal); !errors.Is(err, errCJSILayoutInvalid) {
			t.Fatalf("initialized a seal for a different epoch: %v", err)
		}
		if err := initializeCJSIIndexSeal(ctx, tx, cjsiFixtureIndexEpoch, cjsiIndexSeal{}); !errors.Is(err, errCJSILayoutInvalid) {
			t.Fatalf("initialized invalid seal: %v", err)
		}
		if err := initializeCJSIIndexSeal(ctx, tx, cjsiFixtureIndexEpoch, cjsiFixtureInitialSeal); err != nil {
			t.Fatal(err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		var tables int
		if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='cjsi_layout_initial_seal'").Scan(&tables); err != nil || tables != 0 {
			t.Fatalf("seal schema survived rollback: %d, %v", tables, err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if _, _, err := prepareCJSISealFixture(ctx, path); err != nil {
			t.Fatal(err)
		}
		reopened := openCJSIFixtureDB(t, path, "rw")
		defer reopened.Close()
		tx = beginCJSIFixtureTx(t, reopened)
		other := cjsiFixtureInitialSeal
		other.digest[0] ^= 1
		if err := initializeCJSIIndexSeal(ctx, tx, cjsiFixtureIndexEpoch, other); err == nil {
			t.Fatal("initialization overwrote the existing seal")
		}
		if got, err := readCJSIIndexSeal(ctx, tx); err != nil || got != cjsiFixtureInitialSeal {
			t.Fatalf("reopened initial seal = %+v, %v", got, err)
		}
	})

	t.Run("malformed and unbound seals are rejected", func(t *testing.T) {
		for _, item := range []struct{ name, update string }{
			{"missing seal", "DELETE FROM cjsi_layout_initial_seal"},
			{"missing generation", "DELETE FROM session_index_generation"},
			{"different epoch", "UPDATE session_index_generation SET epoch=epoch+1"},
			{"invalid seal epoch", "UPDATE cjsi_layout_initial_seal SET index_epoch=0"},
			{"invalid version", "UPDATE cjsi_layout_initial_seal SET layout_version=-1"},
			{"short digest", "UPDATE cjsi_layout_initial_seal SET seal_digest=zeroblob(31)"},
			{"zero digest", "UPDATE cjsi_layout_initial_seal SET seal_digest=zeroblob(32)"},
		} {
			t.Run(item.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "private.sqlite3")
				initial := openCJSIFixtureDB(t, path, "rwc")
				cjsiFixtureExec(t, initial, "CREATE TABLE fixture_baseline(x INTEGER)")
				if err := initial.Close(); err != nil {
					t.Fatal(err)
				}
				if _, _, err := prepareCJSISealFixture(t.Context(), path); err != nil {
					t.Fatal(err)
				}
				db := openCJSIFixtureDB(t, path, "rw")
				defer db.Close()
				db.SetMaxOpenConns(1)
				cjsiFixtureExec(t, db, "PRAGMA ignore_check_constraints=ON")
				cjsiFixtureExec(t, db, item.update)
				tx := beginCJSIFixtureTx(t, db)
				if got, err := readCJSIIndexSeal(t.Context(), tx); !errors.Is(err, errCJSILayoutInvalid) || got != (cjsiIndexSeal{}) {
					t.Fatalf("invalid seal was accepted: %+v, %v", got, err)
				}
			})
		}
	})
}

func prepareCJSISealFixture(ctx context.Context, path string) (int64, cjsiIndexSeal, error) {
	db, err := sql.Open("sqlite", cjsiSQLiteURI(path, "rw"))
	if err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	defer tx.Rollback()
	if err := createCJSISealFixtureSchema(ctx, tx); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	if err := initializeCJSIIndexSeal(ctx, tx, cjsiFixtureIndexEpoch, cjsiFixtureInitialSeal); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	if err := tx.Commit(); err != nil {
		return 0, cjsiIndexSeal{}, err
	}
	return cjsiFixtureIndexEpoch, cjsiFixtureInitialSeal, nil
}

func createCJSISealFixtureSchema(ctx context.Context, tx *sql.Tx) error {
	if err := ensureSessionIndexGeneration(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE session_index_generation SET epoch=? WHERE singleton=1", cjsiFixtureIndexEpoch); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, cjsiLayoutSealSchema)
	return err
}
