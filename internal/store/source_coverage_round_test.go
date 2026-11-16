package store

import (
	"context"
	"testing"
	"time"
)

func inventoryStates(t *testing.T, s *Store) map[string]string {
	t.Helper()
	rows, err := s.DB.Query(`SELECT inventory_id,state FROM inventory_runs`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	states := map[string]string{}
	for rows.Next() {
		var id, state string
		if err := rows.Scan(&id, &state); err != nil {
			t.Fatal(err)
		}
		states[id] = state
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return states
}

func coverageHead(t *testing.T, s *Store) string {
	t.Helper()
	var head string
	if err := s.DB.QueryRow(`SELECT inventory_id FROM domain_coverage_heads WHERE singleton=1`).Scan(&head); err != nil {
		t.Fatal(err)
	}
	return head
}

// Abandonment is scoped to one round's own acceptances: other rounds' pending
// staging, ids that merely share a prefix and finished coverage are untouched,
// and the paused attempts are reclaimed only after a newer inventory finishes.
func TestAbandonRoundInventoriesPausesOnlyTheRoundsUnfinishedAcceptances(t *testing.T) {
	ctx := context.Background()
	_, core, sessions := servingTestStores(t)
	sources := []InventorySource{{Path: "a", Identity: "i", End: 10}}
	at := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	for _, domain := range []*Store{core, sessions} {
		if err := domain.AcceptInventory(ctx, "settled", 1, sources, at); err != nil {
			t.Fatal(err)
		}
		if _, err := domain.DB.Exec(`UPDATE source_domain_checkpoints SET committed_end=captured_end,state='complete' WHERE inventory_id='settled'`); err != nil {
			t.Fatal(err)
		}
		if err := domain.FinishInventory(ctx, "settled", "complete"); err != nil {
			t.Fatal(err)
		}
		// Another acceptance still staging, an unrelated round sharing the
		// prefix, the round's first attempt and the head left by its replay.
		if err := domain.stageInventory(ctx, "staging", 1, sources, at.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"round2", "round", "round.replay"} {
			if err := domain.AcceptInventory(ctx, id, 1, sources, at.Add(2*time.Minute)); err != nil {
				t.Fatal(err)
			}
		}

		exists, err := domain.AbandonRoundInventories(ctx, "absent")
		if err != nil || exists {
			t.Fatalf("absent round exists=%t err=%v", exists, err)
		}
		if got := inventoryStates(t, domain); got["round"] != "pending" || got["round.replay"] != "pending" {
			t.Fatalf("absent round changed inventories: %v", got)
		}
		revision := servingRevision(t, domain)
		exists, err = domain.AbandonRoundInventories(ctx, "round")
		if err != nil || !exists {
			t.Fatalf("round exists=%t err=%v", exists, err)
		}
		want := map[string]string{"settled": "complete", "staging": "pending", "round2": "pending", "round": "paused", "round.replay": "paused"}
		if got := inventoryStates(t, domain); len(got) != len(want) {
			t.Fatalf("inventories=%v, want %v", got, want)
		} else {
			for id, state := range want {
				if got[id] != state {
					t.Fatalf("inventories=%v, want %v", got, want)
				}
			}
		}
		if after := servingRevision(t, domain); after != revision+1 {
			t.Fatalf("serving revision %d -> %d, want one terminal coverage change", revision, after)
		}
		// Finished acceptances still exist; a repeat changes nothing else.
		exists, err = domain.AbandonRoundInventories(ctx, "round")
		if err != nil || !exists || servingRevision(t, domain) != revision+1 {
			t.Fatalf("repeat exists=%t err=%v revision=%d", exists, err, servingRevision(t, domain))
		}
		if head := coverageHead(t, domain); head != "round.replay" {
			t.Fatalf("abandonment moved the coverage head to %q", head)
		}

		// The replay's new acceptance finishes; paused attempts and the
		// superseded settlement are reclaimed, the staging one is retained.
		if err := domain.AcceptInventory(ctx, "round.next", 1, sources, at.Add(3*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := domain.DB.Exec(`UPDATE source_domain_checkpoints SET committed_end=captured_end,state='complete' WHERE inventory_id='round.next'`); err != nil {
			t.Fatal(err)
		}
		if err := domain.FinishInventory(ctx, "round.next", "complete"); err != nil {
			t.Fatal(err)
		}
		want = map[string]string{"staging": "pending", "round2": "pending", "round.next": "complete"}
		if got := inventoryStates(t, domain); len(got) != len(want) || got["staging"] != "pending" || got["round2"] != "pending" || got["round.next"] != "complete" {
			t.Fatalf("after replay inventories=%v, want %v", got, want)
		}
		if head := coverageHead(t, domain); head != "round.next" {
			t.Fatalf("replayed coverage head=%q", head)
		}
	}
	if _, err := core.AbandonRoundInventories(ctx, ""); err == nil {
		t.Fatal("empty round id accepted")
	}
}
