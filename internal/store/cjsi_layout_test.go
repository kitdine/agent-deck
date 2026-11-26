package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCJSIFencedStateLayout(t *testing.T) {
	writeFence := func(t *testing.T, root string) {
		t.Helper()
		data := cjsiFenceBytes()
		if err := os.WriteFile(filepath.Join(root, cjsiLegacyIndexName), data[:], 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("absent phases never create or inspect an index", func(t *testing.T) {
		root := t.TempDir()
		writeFence(t, root)
		noRead := func(context.Context, *sql.Tx) (cjsiIndexSeal, error) {
			t.Fatal("nonselected phase inspected an index")
			return cjsiIndexSeal{}, nil
		}
		for _, phase := range []cjsiLayoutPhase{cjsiLayoutNeverBuilt, cjsiLayoutPurged} {
			got, err := resolveCJSILayout(t.Context(), root, cjsiLayoutSelection{phase: phase}, noRead)
			if err != nil || got != (cjsiLayoutResolution{phase: phase}) {
				t.Fatalf("phase %v: got %+v, %v", phase, got, err)
			}
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 1 || entries[0].Name() != cjsiLegacyIndexName {
			t.Fatal("absent state created files or changed the fence")
		}

		// A valid private database can already be installed at its final
		// name while the coordinator has yet to select it.
		cjsiIndexFixture(t, root)
		for _, phase := range []cjsiLayoutPhase{cjsiLayoutNeverBuilt, cjsiLayoutPurged} {
			want := cjsiLayoutResolution{phase: phase}
			got, err := resolveCJSILayout(t.Context(), root, cjsiLayoutSelection{phase: phase}, noRead)
			if err != nil || got != want {
				t.Fatalf("residual index was selected in phase %v: %+v, %v", phase, got, err)
			}
		}
		selection := cjsiLayoutSelection{phase: cjsiLayoutStaging}
		if err := os.Remove(filepath.Join(root, cjsiLegacyIndexName)); err != nil {
			t.Fatal(err)
		}
		got, err := resolveCJSILayout(t.Context(), root, selection, noRead)
		if err != nil || got != (cjsiLayoutResolution{phase: cjsiLayoutStaging}) {
			t.Fatalf("staging exposed a locator: %+v, %v", got, err)
		}
		if _, err := os.Lstat(filepath.Join(root, cjsiLegacyIndexName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("staging installed a fence")
		}
	})

	t.Run("fence replacement during seal read cannot select", func(t *testing.T) {
		root := t.TempDir()
		writeFence(t, root)
		selection := cjsiIndexFixture(t, root)
		got, err := resolveCJSILayout(t.Context(), root, selection, func(ctx context.Context, tx *sql.Tx) (cjsiIndexSeal, error) {
			seal, err := readCJSIFixtureSeal(ctx, tx)
			if err != nil {
				return cjsiIndexSeal{}, err
			}
			marker := filepath.Join(root, cjsiLegacyIndexName)
			if err := os.Rename(marker, filepath.Join(root, "prior-fence")); err != nil {
				t.Fatal(err)
			}
			writeFence(t, root)
			return seal, nil
		})
		if !errors.Is(err, errCJSIFenceInvalid) || got.path != "" {
			t.Fatalf("fence replacement was not detected: %+v, %v", got, err)
		}
	})

	t.Run("index replacement during seal read cannot select", func(t *testing.T) {
		root := t.TempDir()
		writeFence(t, root)
		selection := cjsiIndexFixture(t, root)
		got, err := resolveCJSILayout(t.Context(), root, selection, func(ctx context.Context, tx *sql.Tx) (cjsiIndexSeal, error) {
			seal, err := readCJSIFixtureSeal(ctx, tx)
			if err != nil {
				return cjsiIndexSeal{}, err
			}
			path := filepath.Join(root, cjsiPrivateIndexName)
			if err := os.Rename(path, filepath.Join(root, "prior-index")); err != nil {
				t.Fatal(err)
			}
			cjsiIndexFixture(t, root)
			return seal, nil
		})
		if !errors.Is(err, errCJSILayoutInvalid) || got.path != "" {
			t.Fatalf("index replacement was not detected: %+v, %v", got, err)
		}
	})

	t.Run("root replacement during seal read cannot select", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "root")
		next := filepath.Join(parent, "next")
		for _, path := range []string{root, next} {
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
		}
		writeFence(t, root)
		selection := cjsiIndexFixture(t, root)
		// Keep both child inodes identical across the replacement, so
		// only the root identity check can detect the change.
		for _, name := range []string{cjsiLegacyIndexName, cjsiPrivateIndexName} {
			if err := os.Link(filepath.Join(root, name), filepath.Join(next, name)); err != nil {
				t.Fatal(err)
			}
		}
		got, err := resolveCJSILayout(t.Context(), root, selection, func(ctx context.Context, tx *sql.Tx) (cjsiIndexSeal, error) {
			seal, err := readCJSIFixtureSeal(ctx, tx)
			if err != nil {
				return cjsiIndexSeal{}, err
			}
			if err := os.Rename(root, filepath.Join(parent, "prior-root")); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(next, root); err != nil {
				t.Fatal(err)
			}
			return seal, nil
		})
		if !errors.Is(err, errCJSILayoutInvalid) || got.path != "" {
			t.Fatalf("root replacement was not detected: %+v, %v", got, err)
		}
	})

	t.Run("busy selected index remains unavailable without a locator", func(t *testing.T) {
		root := t.TempDir()
		writeFence(t, root)
		selection := cjsiIndexFixture(t, root)
		db, err := sql.Open("sqlite", filepath.Join(root, cjsiPrivateIndexName))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if _, err := conn.ExecContext(t.Context(), "BEGIN EXCLUSIVE"); err != nil {
			t.Fatal(err)
		}
		defer conn.ExecContext(t.Context(), "ROLLBACK")
		got, err := resolveCJSILayout(t.Context(), root, selection, readCJSIFixtureSeal)
		if !errors.Is(err, ErrStateBusy) || got.path != "" {
			t.Fatalf("exclusive writer was not reported as busy: %+v, %v", got, err)
		}
	})

	t.Run("old fence must be exact private regular marker", func(t *testing.T) {
		for _, kind := range []string{"missing", "wrong content", "wrong size", "wrong mode", "directory", "symlink"} {
			t.Run(kind, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, cjsiLegacyIndexName)
				marker := cjsiFenceBytes()
				var err error
				switch kind {
				case "wrong content":
					marker[len(marker)-1] = 1
					err = os.WriteFile(path, marker[:], 0600)
				case "wrong size":
					err = os.WriteFile(path, marker[:len(marker)-1], 0600)
				case "wrong mode":
					if err = os.WriteFile(path, marker[:], 0600); err == nil {
						err = os.Chmod(path, 0644)
					}
				case "directory":
					err = os.Mkdir(path, 0700)
				case "symlink":
					target := filepath.Join(root, "marker")
					if err = os.WriteFile(target, marker[:], 0600); err == nil {
						err = os.Symlink(target, path)
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				got, err := resolveCJSILayout(t.Context(), root, cjsiLayoutSelection{phase: cjsiLayoutNeverBuilt}, nil)
				if !errors.Is(err, errCJSIFenceInvalid) || got != (cjsiLayoutResolution{}) {
					t.Fatalf("bad fence accepted: %+v, %v", got, err)
				}
			})
		}
	})

	t.Run("selected locator requires matching identity and seal", func(t *testing.T) {
		root := t.TempDir()
		writeFence(t, root)
		selection := cjsiIndexFixture(t, root)
		want := cjsiLayoutResolution{phase: cjsiLayoutSelected, path: filepath.Join(root, cjsiPrivateIndexName)}
		if got, err := resolveCJSILayout(t.Context(), root, selection, readCJSIFixtureSeal); err != nil || got != want {
			t.Fatalf("verified selected index: %+v, %v", got, err)
		}
		cases := []struct {
			name   string
			change func(*cjsiLayoutSelection)
		}{
			{"unknown phase", func(s *cjsiLayoutSelection) { s.phase = cjsiLayoutPhase(255) }},
			{"unapproved locator", func(s *cjsiLayoutSelection) { s.locator = cjsiIndexLocator(255) }},
			{"wrong identity", func(s *cjsiLayoutSelection) { s.identity.inode++ }},
			{"wrong version", func(s *cjsiLayoutSelection) { s.seal.version++ }},
			{"wrong epoch", func(s *cjsiLayoutSelection) { s.epoch++ }},
			{"wrong seal", func(s *cjsiLayoutSelection) { s.seal.digest[0] ^= 1 }},
			{"mixed phase and selection", func(s *cjsiLayoutSelection) { s.phase = cjsiLayoutStaging }},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				bad := selection
				test.change(&bad)
				got, err := resolveCJSILayout(t.Context(), root, bad, readCJSIFixtureSeal)
				if !errors.Is(err, errCJSILayoutInvalid) || got != (cjsiLayoutResolution{}) {
					t.Fatalf("invalid selection accepted: %+v, %v", got, err)
				}
			})
		}
		if err := os.Remove(filepath.Join(root, cjsiLegacyIndexName)); err != nil {
			t.Fatal(err)
		}
		if got, err := resolveCJSILayout(t.Context(), root, selection, readCJSIFixtureSeal); !errors.Is(err, errCJSIFenceInvalid) || got.path != "" {
			t.Fatalf("selected index bypassed missing fence: %+v, %v", got, err)
		}
	})

	t.Run("base schema open callers refuse old path before selected data", func(t *testing.T) {
		root := t.TempDir()
		core, err := Open(t.Context(), root)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = core.Close() })
		version, err := core.SchemaVersion(t.Context())
		if err != nil || version != cjsiBaseCoreSchemaVersion {
			t.Fatalf("legacy test requires the quota schema %d baseline: version=%d err=%v", cjsiBaseCoreSchemaVersion, version, err)
		}
		writeFence(t, root)
		selection := cjsiIndexFixture(t, root)

		for _, open := range []func(context.Context, string) (*Store, error){OpenSessionsReadOnly, OpenSessions} {
			legacy, err := open(t.Context(), root)
			if legacy != nil {
				_ = legacy.Close()
				t.Fatal("legacy opener returned a database past the fence")
			}
			if !errors.Is(err, errCJSILayoutInvalid) {
				t.Fatalf("legacy opener did not refuse the inconsistent layout: %v", err)
			}
			if _, err := validateCJSIFence(root); err != nil {
				t.Fatal("legacy opener changed the fence")
			}
		}
		if err := cjsiProbeFence(t.Context(), filepath.Join(root, cjsiLegacyIndexName)); err != nil {
			t.Fatalf("old SQLite read/write statements opened the fence: %v", err)
		}
		got, err := resolveCJSILayout(t.Context(), root, selection, readCJSIFixtureSeal)
		if err != nil || got.phase != cjsiLayoutSelected {
			t.Fatalf("legacy opener damaged private selection: %+v, %v", got, err)
		}
	})

	t.Run("private selection", testCJSIPrivateSelection)
	t.Run("c2 migration", testCJSIC2Migration)
}

func cjsiIndexFixture(t *testing.T, root string) cjsiLayoutSelection {
	t.Helper()
	path := filepath.Join(root, cjsiPrivateIndexName)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ensureSessionIndexGeneration(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	epoch, err := (&Store{DB: db}).SessionIndexEpoch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	seal := cjsiIndexSeal{version: 7, digest: sha256.Sum256([]byte("synthetic selected index seal"))}
	// A test-local adapter exercises this boundary without assigning a
	// production schema version or name.
	if _, err := db.Exec(`CREATE TABLE fixture_index_seal(version INTEGER, digest BLOB)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO fixture_index_seal VALUES(?,?)`, seal.version, seal.digest[:]); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	stat, err := privateServingPath(path, false)
	if err != nil {
		t.Fatal(err)
	}
	return cjsiLayoutSelection{phase: cjsiLayoutSelected, locator: cjsiIndexPrivate, identity: cjsiIdentity(stat), epoch: epoch, seal: seal}
}

func readCJSIFixtureSeal(ctx context.Context, tx *sql.Tx) (cjsiIndexSeal, error) {
	var seal cjsiIndexSeal
	var digest []byte
	if err := tx.QueryRowContext(ctx, `SELECT version, digest FROM fixture_index_seal`).Scan(&seal.version, &digest); err != nil {
		return cjsiIndexSeal{}, err
	}
	if len(digest) != len(seal.digest) {
		return cjsiIndexSeal{}, errCJSILayoutInvalid
	}
	copy(seal.digest[:], digest)
	return seal, nil
}
