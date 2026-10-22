package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestInventoryBatchesPreserveEverySourceAndRevision(t *testing.T) {
	ctx := context.Background()
	_, core, sessions := servingTestStores(t)
	for _, domain := range []struct {
		name string
		s    *Store
	}{{"core", core}, {"session", sessions}} {
		t.Run(domain.name, func(t *testing.T) {
			for _, count := range []int{0, 1, 127, 128, 129, 4717} {
				id := fmt.Sprintf("inventory-%d", count)
				sources := make([]InventorySource, count)
				for i := range sources {
					sources[i] = InventorySource{Path: fmt.Sprintf("source-%05d", i), Identity: fmt.Sprintf("identity-%d", i), End: int64(i * 123)}
				}
				var before, after int64
				if err := domain.s.DB.QueryRow(`SELECT revision FROM serving_generation`).Scan(&before); err != nil {
					t.Fatal(err)
				}
				started := time.Now()
				if err := domain.s.AcceptInventory(ctx, id, 1, sources, started); err != nil {
					t.Fatal(err)
				}
				t.Logf("synthetic %s sources=%d AcceptInventory wall=%s", domain.name, count, time.Since(started))
				rows, err := domain.s.DB.Query(`SELECT source_path,identity,captured_end,committed_end,state FROM source_domain_checkpoints WHERE inventory_id=? ORDER BY source_path`, id)
				if err != nil {
					t.Fatal(err)
				}
				index := 0
				for rows.Next() {
					var got InventorySource
					var committed int64
					var state string
					if err := rows.Scan(&got.Path, &got.Identity, &got.End, &committed, &state); err != nil {
						t.Fatal(err)
					}
					if index >= count || got != sources[index] || committed != 0 || state != "pending" {
						t.Fatalf("checkpoint %d changed: %+v committed=%d state=%s", index, got, committed, state)
					}
					index++
				}
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				rows.Close()
				if index != count {
					t.Fatalf("checkpoints=%d want=%d", index, count)
				}
				var head string
				var declared int
				if err := domain.s.DB.QueryRow(`SELECT h.inventory_id,i.source_count FROM domain_coverage_heads h JOIN inventory_runs i USING(inventory_id)`).Scan(&head, &declared); err != nil || head != id || declared != count {
					t.Fatalf("head=%s count=%d error=%v", head, declared, err)
				}
				if err := domain.s.DB.QueryRow(`SELECT revision FROM serving_generation`).Scan(&after); err != nil || after != before+int64(count)+1 {
					t.Fatalf("revision %d -> %d count=%d error=%v", before, after, count, err)
				}
			}
		})
	}
}

func TestInventoryLaterBatchFailureRollsBackAcceptanceAndHead(t *testing.T) {
	ctx := context.Background()
	_, core, sessions := servingTestStores(t)
	for _, s := range []*Store{core, sessions} {
		if err := s.AcceptInventory(ctx, "previous", 1, nil, time.Now()); err != nil {
			t.Fatal(err)
		}
		var before int64
		if err := s.DB.QueryRow(`SELECT revision FROM serving_generation`).Scan(&before); err != nil {
			t.Fatal(err)
		}
		for _, failure := range []string{"duplicate", "trigger", "invalid", "head"} {
			sources := make([]InventorySource, 300)
			for i := range sources {
				sources[i] = InventorySource{Path: fmt.Sprintf("source-%05d", i), Identity: "identity", End: int64(i)}
			}
			switch failure {
			case "duplicate":
				sources[299] = sources[0]
			case "invalid":
				sources[299].End = -1
			case "trigger":
				if _, err := s.DB.Exec(`CREATE TRIGGER reject_checkpoint BEFORE INSERT ON source_domain_checkpoints WHEN NEW.source_path='source-00299' BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`); err != nil {
					t.Fatal(err)
				}
			case "head":
				if _, err := s.DB.Exec(`CREATE TRIGGER reject_head BEFORE UPDATE ON domain_coverage_heads BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.AcceptInventory(ctx, failure, 1, sources, time.Now()); err == nil {
				t.Fatalf("%s failure accepted", failure)
			}
			for _, table := range []string{"inventory_runs", "source_domain_checkpoints"} {
				var count int
				if err := s.DB.QueryRow(`SELECT count(*) FROM `+table+` WHERE inventory_id=?`, failure).Scan(&count); err != nil || count != 0 {
					t.Fatalf("%s leaked %s rows=%d error=%v", failure, table, count, err)
				}
			}
			var head string
			var after int64
			if err := s.DB.QueryRow(`SELECT inventory_id FROM domain_coverage_heads`).Scan(&head); err != nil || head != "previous" {
				t.Fatalf("failed acceptance moved head to %s: %v", head, err)
			}
			if err := s.DB.QueryRow(`SELECT revision FROM serving_generation`).Scan(&after); err != nil || after != before {
				t.Fatalf("rollback advanced revision %d -> %d: %v", before, after, err)
			}
			if _, err := s.DB.Exec(`DROP TRIGGER IF EXISTS reject_checkpoint; DROP TRIGGER IF EXISTS reject_head`); err != nil {
				t.Fatal(err)
			}
		}
	}
}
