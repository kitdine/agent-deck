package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestDerivedSnapshotGenerationTracksRelevantWrites(t *testing.T) {
	original := generateGenerationEpoch
	generateGenerationEpoch = func() (int64, error) { return 424242, nil }
	t.Cleanup(func() { generateGenerationEpoch = original })
	ctx := context.Background()
	database, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	initial, err := database.DerivedSnapshotGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if initial.Epoch != 424242 || initial.Revision != 0 || !initial.Dirty || initial.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("initial generation=%#v", initial)
	}
	build, err := database.BeginDerivedSnapshotGenerationBuild(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if build.Epoch != initial.Epoch || build.Revision != initial.Revision+1 || build.Dirty {
		t.Fatalf("build generation=%#v initial=%#v", build, initial)
	}

	if _, err = database.DB.ExecContext(ctx, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('source','identity',0,0,'')`); err != nil {
		t.Fatal(err)
	}
	changed, err := database.DerivedSnapshotGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !changed.Dirty || changed.Revision != build.Revision+1 {
		t.Fatalf("changed generation=%#v build=%#v", changed, build)
	}
	if _, err = database.DB.ExecContext(ctx, `UPDATE usage_source_files SET size=1 WHERE path='source'`); err != nil {
		t.Fatal(err)
	}
	coalesced, err := database.DerivedSnapshotGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !coalesced.Dirty || coalesced.Revision != changed.Revision {
		t.Fatalf("dirty write was not coalesced: changed=%#v coalesced=%#v", changed, coalesced)
	}
	nextBuild, err := database.BeginDerivedSnapshotGenerationBuild(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if nextBuild.Dirty || nextBuild.Revision != coalesced.Revision+1 {
		t.Fatalf("next build=%#v coalesced=%#v", nextBuild, coalesced)
	}
}

func TestDerivedSnapshotGenerationInstallsEveryAllowlistedWriterTrigger(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for _, table := range derivedSnapshotGenerationTables {
		var count int
		if err = database.DB.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='trigger' AND tbl_name=? AND name LIKE ?`, table, fmt.Sprintf("derived_snapshot_generation_%s_%%", table)).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 3 {
			t.Fatalf("%s generation triggers=%d, want insert/update/delete", table, count)
		}
	}

	assertDirty := func(name string, write func() error) {
		t.Helper()
		before, err := database.DerivedSnapshotGeneration(ctx)
		if err != nil {
			t.Fatal(err)
		}
		before, err = database.BeginDerivedSnapshotGenerationBuild(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if before.Dirty {
			t.Fatalf("%s precondition=%#v err=%v", name, before, err)
		}
		if err = write(); err != nil {
			t.Fatalf("%s write: %v", name, err)
		}
		after, err := database.DerivedSnapshotGeneration(ctx)
		if err != nil || !after.Dirty || after.Revision != before.Revision+1 {
			t.Fatalf("%s generation before=%#v after=%#v err=%v", name, before, after, err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	assertDirty("provider", func() error {
		_, err := database.DB.ExecContext(ctx, `INSERT INTO providers(name,endpoint,credential_ref,multiplier,created_at,updated_at) VALUES('generation-provider','https://example.invalid','generation-ref','1',?,?)`, now, now)
		return err
	})
	assertDirty("price", func() error {
		_, err := database.DB.ExecContext(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES('generation-price','fixture','fixture://generation','digest',?,'2026-01-01','USD',1)`, now)
		return err
	})
	assertDirty("source", func() error {
		_, err := database.DB.ExecContext(ctx, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('generation-source','generation',0,0,'')`)
		return err
	})
}

func TestSessionIndexEpochPersistsAcrossOpenAndMintsOnDemand(t *testing.T) {
	values := []int64{101, 202}
	original := generateGenerationEpoch
	generateGenerationEpoch = func() (int64, error) {
		value := values[0]
		values = values[1:]
		return value, nil
	}
	t.Cleanup(func() { generateGenerationEpoch = original })

	ctx := context.Background()
	root := t.TempDir()
	sessions, err := OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	epoch, err := sessions.SessionIndexEpoch(ctx)
	if err != nil || epoch != 101 {
		t.Fatalf("created session epoch=%d err=%v", epoch, err)
	}
	if err = sessions.Close(); err != nil {
		t.Fatal(err)
	}
	sessions, err = OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer sessions.Close()
	if reopened, reopenErr := sessions.SessionIndexEpoch(ctx); reopenErr != nil || reopened != epoch {
		t.Fatalf("reopened session epoch=%d err=%v want=%d", reopened, reopenErr, epoch)
	}
	minted, err := MintSessionIndexEpoch(ctx, sessions.DB)
	if err != nil || minted != 202 {
		t.Fatalf("minted session epoch=%d err=%v", minted, err)
	}
}

func TestOpenExistingNeverCreatesMissingState(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir() + "/missing"
	if _, err := OpenExisting(ctx, root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing state error=%v", err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("OpenExisting created root: %v", err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenExisting(ctx, root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing database error=%v", err)
	}
	if _, err := os.Stat(root + "/agentdeck.sqlite3"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("OpenExisting created database: %v", err)
	}
	created, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = created.Close(); err != nil {
		t.Fatal(err)
	}
	existing, err := OpenExisting(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = existing.Close(); err != nil {
		t.Fatal(err)
	}
}
