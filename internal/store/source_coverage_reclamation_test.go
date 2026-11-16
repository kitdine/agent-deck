package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestCompletedInventoriesReclaimCheckpointsAndRetainSettlementEvidence(t *testing.T) {
	ctx := context.Background()
	_, core, sessions := servingTestStores(t)
	for _, domain := range []*Store{core, sessions} {
		sources := make([]InventorySource, 2*inventoryInsertBatch+1)
		for i := range sources {
			sources[i] = InventorySource{Path: fmt.Sprint(i), Identity: "synthetic", End: 10}
		}
		for round := 0; round < 5; round++ {
			id := fmt.Sprintf("round-%d", round)
			if err := domain.AcceptInventory(ctx, id, 1, sources, time.Date(2026, 10, 29, 12, round, 0, 0, time.UTC)); err != nil {
				t.Fatal(err)
			}
			if round > 0 {
				var complete int
				if err := domain.DB.QueryRow(`SELECT count(*) FROM inventory_runs WHERE state='complete'`).Scan(&complete); err != nil || complete != 1 {
					t.Fatalf("pending head lost settlement evidence: complete=%d err=%v", complete, err)
				}
			}
			if _, err := domain.DB.Exec(`UPDATE source_domain_checkpoints SET committed_end=captured_end,state='complete' WHERE inventory_id=?`, id); err != nil {
				t.Fatal(err)
			}
			if err := domain.FinishInventory(ctx, id, "complete"); err != nil {
				t.Fatal(err)
			}
			var runs, checkpoints int
			if err := domain.DB.QueryRow(`SELECT count(*) FROM inventory_runs`).Scan(&runs); err != nil {
				t.Fatal(err)
			}
			if err := domain.DB.QueryRow(`SELECT count(*) FROM source_domain_checkpoints`).Scan(&checkpoints); err != nil {
				t.Fatal(err)
			}
			if runs != 1 || checkpoints != len(sources) {
				t.Fatalf("unbounded completed history: runs=%d checkpoints=%d", runs, checkpoints)
			}
		}
		// An in-flight acceptance has no head yet and must survive reclamation.
		if err := domain.stageInventory(ctx, "staging", 1, sources, time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := domain.AcceptInventory(ctx, "paused-head", 1, nil, time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := domain.FinishInventory(ctx, "paused-head", "paused"); err != nil {
			t.Fatal(err)
		}
		var retained int
		if err := domain.DB.QueryRow(`SELECT count(*) FROM inventory_runs WHERE inventory_id IN ('round-4','staging','paused-head')`).Scan(&retained); err != nil || retained != 3 {
			t.Fatalf("head, settlement or in-flight inventory removed: count=%d err=%v", retained, err)
		}
	}
}
