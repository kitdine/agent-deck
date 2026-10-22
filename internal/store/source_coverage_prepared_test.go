package store

import (
	"context"
	"testing"
	"time"
)

func TestPreparedSourceCoverageKeepsIdentityOffsetsAndRollback(t *testing.T) {
	ctx := context.Background()
	_, core, _ := servingTestStores(t)
	generation, err := core.DerivedSnapshotGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = core.AcceptInventory(ctx, "prepared", generation.Epoch, []InventorySource{{Path: "source", Identity: "identity", End: 12}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	tx, err := core.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	w, err := PrepareSourceCoverage(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for _, test := range []struct {
		identity string
		end      int64
	}{{"wrong", 12}, {"identity", 13}, {"identity", -1}} {
		if err := w.Commit(ctx, "prepared", "source", test.identity, 12, test.end); err == nil {
			t.Fatal("invalid prepared coverage accepted")
		}
	}
	if err = w.Commit(ctx, "prepared", "source", "identity", 12, 12); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	coverage, err := core.ReadSourceCoverage(ctx)
	if err != nil || coverage.Committed != 0 {
		t.Fatalf("rollback advanced coverage: %+v %v", coverage, err)
	}
}

func TestPreparedInventoryInsertionKeepsWholeAcceptanceAtomic(t *testing.T) {
	ctx := context.Background()
	_, core, _ := servingTestStores(t)
	generation, err := core.DerivedSnapshotGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = core.AcceptInventory(ctx, "invalid", generation.Epoch, []InventorySource{{Path: "valid", Identity: "i", End: 1}, {Path: "", Identity: "bad", End: 2}}, time.Now())
	if err == nil {
		t.Fatal("invalid inventory accepted")
	}
	var count int
	if err = core.DB.QueryRow(`SELECT count(*) FROM inventory_runs WHERE inventory_id='invalid'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial inventory persisted: %d %v", count, err)
	}
}
