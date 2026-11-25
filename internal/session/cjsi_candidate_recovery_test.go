//go:build cjsi_adapter_fixture

package session

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestCJSIRecoverSealedIndex(t *testing.T) {
	for _, fault := range []string{"none", "source", "parent", "id"} {
		t.Run(fault, func(t *testing.T) {
			ctx := t.Context()
			f := newPhysicalFixture(t, true, false)
			path := filepath.Join(t.TempDir(), "recover.jsonl")
			body := `{"type":"session_meta","payload":{"id":"` + adapterThread + `"}}
{"timestamp":"2026-01-02T10:00:00Z","type":"response_item","payload":{"type":"message","role":"user","content":"verified fixture"}}
`
			source := fixturePreparedNative(t, path, adapterThread, body)
			const ticket, attempt int64 = 9, 2
			var parent store.CJSIIndexStamp
			var seed *CJSIIndexCandidate
			f.snapshot(t, func(v store.CJSIIndexSnapshot) error {
				parent = v.Selection()
				if c, found, err := RecoverCJSIIndex(ctx, v, ticket, attempt, parent.SessionRevision+1, []CJSIPreparedSource{source}, nil, nil); err != nil || found || c != nil {
					t.Fatalf("absent recovery: found=%v err=%v", found, err)
				}
				var err error
				seed, err = PlanCJSIIndex(ctx, v, ticket, attempt, parent.SessionRevision+1, []CJSIPreparedSource{source}, nil, nil)
				return err
			})
			original := seed.Seal()
			ids := seed.plan.Replacements[0].DocumentIDs
			if len(ids) != 1 {
				t.Fatal("fixture requires one newly allocated document")
			}
			tx, err := f.private.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if err = seed.Stage(ctx, tx, parent, ticket, attempt, parent.SessionRevision+1, original); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
			oldID := ids[0]
			seed = nil // the allocator and committed membership outlive the plan
			switch fault {
			case "source":
				source.replacement.PrefixHash = "independent-different-source"
			case "parent":
				if _, err = f.private.Exec("DROP TRIGGER cjsi_sealed_update; UPDATE cjsi_session_candidates SET parent_ticket=1"); err != nil {
					t.Fatal(err)
				}
			case "id":
				replaceSealedFixtureID(t, ctx, f.private, oldID)
			}
			f.snapshot(t, func(v store.CJSIIndexSnapshot) error {
				before := recoveryChanges(t, v.Tx)
				c, found, err := RecoverCJSIIndex(ctx, v, ticket, attempt, parent.SessionRevision+1, []CJSIPreparedSource{source}, nil, nil)
				if fault == "none" {
					if !found || err != nil || c == nil || c.Seal() != original || c.plan.Replacements[0].DocumentIDs[0] != oldID {
						t.Fatalf("lost original ID or seal: found=%v err=%v", found, err)
					}
					fresh, e := PlanCJSIIndex(ctx, v, ticket, attempt, parent.SessionRevision+1, []CJSIPreparedSource{source}, nil, nil)
					if e != nil || fresh == nil || fresh.Seal() == original || fresh.plan.Replacements[0].DocumentIDs[0] <= oldID {
						t.Fatalf("fresh allocation changed: %v", e)
					}
				} else if !found || c != nil || !errors.Is(err, errIndexStage) {
					t.Fatalf("%s mismatch accepted: found=%v err=%v", fault, found, err)
				}
				if recoveryChanges(t, v.Tx) != before {
					t.Fatal("recovery wrote private rows")
				}
				return nil
			})
		})
	}
}

func recoveryChanges(t *testing.T, tx *sql.Tx) int64 {
	t.Helper()
	var n int64
	if err := tx.QueryRow("SELECT total_changes()").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func replaceSealedFixtureID(t *testing.T, ctx context.Context, db *sql.DB, old int64) {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	fresh, _, err := store.ReserveCJSIDocumentIDs(ctx, tx, 1)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := indexDigest([]int64{fresh})
	if err != nil {
		t.Fatal(err)
	}
	// Local corruption fixture. Make the new ID, payload, allocator and group
	// sequence mutually consistent so that only the original full seal rejects it.
	if _, err = tx.Exec("DROP TRIGGER cjsi_lock_order_UPDATE; DROP TRIGGER cjsi_lock_groups_UPDATE"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("INSERT INTO cjsi_candidate_payloads(doc_id,event_at,kind,text) SELECT ?,event_at,kind,text FROM cjsi_candidate_payloads WHERE doc_id=?", fresh, old); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("UPDATE cjsi_candidate_order SET doc_id=? WHERE doc_id=?", fresh, old); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("UPDATE cjsi_candidate_groups SET sequence_id=?", seq[:]); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestCJSIRecoveredWithdrawalCopy(t *testing.T) {
	f := newPhysicalFixture(t, true, true)
	dest := filepath.Join(t.TempDir(), filepath.Base(f.path))
	if err := os.Rename(f.path, dest); err != nil {
		t.Fatal(err)
	}
	replacement := stageFixturePlan(t, f, dest, true).Replacements[0]
	replacement.From = f.path
	replacement.DocumentIDs = nil
	replacement.DeletedDocumentIDs = nil
	replacement.Sequence = [32]byte{}
	f.snapshot(t, func(view store.CJSIIndexSnapshot) error {
		h := view.Selection()
		c, err := PlanCJSIIndex(t.Context(), view, h.Ticket+1, 1, h.SessionRevision+1, []CJSIPreparedSource{{replacement}}, []CJSISourceRemoval{{"replica", "second"}}, nil)
		if err != nil {
			return err
		}
		want := []CJSISourceRemoval{{f.path, f.identity}, {"replica", "second"}}
		if c == nil || !slices.Equal(c.Withdrawals(), want) {
			t.Fatal("the internally discovered move was absent")
		}
		original := c.Seal()
		got := c.Withdrawals()
		got[0] = CJSISourceRemoval{}
		if !slices.Equal(c.Withdrawals(), want) || c.Seal() != original {
			t.Fatal("caller mutated sealed withdrawals")
		}
		return nil
	})
}
