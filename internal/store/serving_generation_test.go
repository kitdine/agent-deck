package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func servingTestStores(t *testing.T) (string, *Store, *Store) {
	t.Helper()
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "state")
	core, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { core.Close() })
	sessions, err := OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sessions.Close() })
	return root, core, sessions
}

func TestServingGenerationReadOnlyAndDirtyRevision(t *testing.T) {
	ctx := context.Background()
	root, core, _ := servingTestStores(t)
	before, err := ReadServingGeneration(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"first", "second"} {
		if err = core.SetSetting(ctx, "serving-test", value); err != nil {
			t.Fatal(err)
		}
	}
	after, err := ReadServingGeneration(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if after.StateIdentity != before.StateIdentity || after.CoreEpoch != before.CoreEpoch || after.UsageRevision <= before.UsageRevision+1 {
		t.Fatalf("revision did not advance for each dirty write: before=%+v after=%+v", before, after)
	}
	again, err := ReadServingGeneration(ctx, root)
	if err != nil || again != after {
		t.Fatalf("read mutated generation: %+v %v", again, err)
	}
	if err = core.MintDerivedSnapshotEpoch(ctx); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadServingGeneration(ctx, root)
	if err != nil || restored.CoreEpoch == after.CoreEpoch {
		t.Fatalf("restore identity did not change: %+v %v", restored, err)
	}
}

func TestServingGenerationRejectsMissingAndUnsafeState(t *testing.T) {
	ctx := context.Background()
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := ReadServingGeneration(ctx, missing); err == nil {
		t.Fatal("missing state accepted")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("read created state: %v", err)
	}
	root, _, _ := servingTestStores(t)
	path := filepath.Join(root, "agentdeck.sqlite3")
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadServingGeneration(ctx, root); err == nil {
		t.Fatal("public database accepted")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "linked-state")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadServingGeneration(ctx, link); err == nil {
		t.Fatal("symlink state accepted")
	}
}

func TestSourceCoverageCommitRollbackAndFiniteInventory(t *testing.T) {
	ctx := context.Background()
	_, core, _ := servingTestStores(t)
	coverage, err := core.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "unknown" {
		t.Fatalf("new DB fabricated coverage: %+v %v", coverage, err)
	}
	generation, err := core.DerivedSnapshotGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = core.AcceptInventory(ctx, "finite", generation.Epoch, []InventorySource{{Path: "source", Identity: "identity", End: 123}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	coverage, err = core.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "update_pending" {
		t.Fatalf("pending acceptance lost: %+v %v", coverage, err)
	}
	tx, err := core.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = CommitSourceCoverage(ctx, tx, "finite", "source", "identity", 123, 123); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	if err = core.FinishInventory(ctx, "finite", "complete"); err == nil {
		t.Fatal("rolled back source claimed complete")
	}
	tx, err = core.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = CommitSourceCoverage(ctx, tx, "finite", "source", "wrong", 123, 123); err == nil {
		t.Fatal("wrong source identity accepted")
	}
	if err = CommitSourceCoverage(ctx, tx, "finite", "source", "identity", 123, 123); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	coverage, err = core.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "partial" || coverage.Committed != 1 {
		t.Fatalf("source commit prematurely completed finite scan: %+v %v", coverage, err)
	}
	if err = core.FinishInventory(ctx, "finite", "complete"); err != nil {
		t.Fatal(err)
	}
	coverage, err = core.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "complete" {
		t.Fatalf("complete finite inventory lost: %+v %v", coverage, err)
	}
	if err = core.AcceptInventory(ctx, "followup", generation.Epoch, []InventorySource{{Path: "source", Identity: "identity", End: 456}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = core.FinishInventory(ctx, "followup", "paused"); err != nil {
		t.Fatal(err)
	}
	coverage, err = core.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "partial" || coverage.InventoryID != "followup" {
		t.Fatalf("paused history erased or fabricated coverage: %+v %v", coverage, err)
	}
}

func TestServingGenerationReaderRechecksValuesAndFileIdentity(t *testing.T) {
	root, core, _ := servingTestStores(t)
	ctx := context.Background()
	reader, err := OpenServingGenerationReader(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	before, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := core.SetSetting(ctx, "reader-recheck", "changed"); err != nil {
		t.Fatal(err)
	}
	after, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if after.UsageRevision <= before.UsageRevision {
		t.Fatal("reader cached generation values")
	}
	if err := core.MintDerivedSnapshotEpoch(ctx); err != nil {
		t.Fatal(err)
	}
	epoch, err := reader.Read()
	if err != nil || epoch.CoreEpoch == after.CoreEpoch {
		t.Fatalf("cached epoch: %+v %v", epoch, err)
	}
	path := filepath.Join(root, "sessions.sqlite3")
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(path+".old", path)
	if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Read(); err == nil {
		t.Fatal("open connection hid replaced database")
	}
}

func TestServingGenerationReaderBudgetAndCancellation(t *testing.T) {
	root, _, _ := servingTestStores(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	start := time.Now()
	reader, err := OpenServingGenerationReader(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	deadline, ok := reader.ctx.Deadline()
	if !ok || deadline.Before(start.Add(200*time.Millisecond)) || deadline.After(time.Now().Add(200*time.Millisecond)) {
		t.Fatal("identity budget increased")
	}
	if _, err := reader.Read(); err != nil {
		t.Fatal(err)
	}
	next, _ := reader.ctx.Deadline()
	if !next.Equal(deadline) {
		t.Fatal("read renewed deadline")
	}
	cancel()
	if _, err := reader.Read(); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel ignored: %v", err)
	}
}
