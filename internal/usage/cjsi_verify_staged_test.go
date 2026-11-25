package usage

import (
	"context"
	"database/sql"
	"errors"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
	"testing"
)

func TestCJSIVerifyStaged(t *testing.T) {
	r := newReplayDatabase(t)
	body := ledgerHead + ledgerTurn + ledgerNative("verified", counter(9), "")
	c, d := r.candidate(t, 1, body)
	if _, err := r.db.Exec("INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES('old','codex','s','t','2026-01-02T00:00:00Z','m','old',0)"); err != nil {
		t.Fatal(err)
	}
	row, err := ReadCJSIOldUsage(t.Context(), r.db, "old")
	if err != nil {
		t.Fatal(err)
	}
	c.old = []ledger.OldRow{row.Precondition()}
	d.ExpectedOldRows = c.old
	selectDraft := func(context.Context, CJSIUsageDraftReader, string) (CJSIUsageDraft, error) { return d, nil }
	if err = c.Stage(t.Context(), r.db, selectDraft); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec("ALTER TABLE cjsi_publication_attempts ADD COLUMN session_seal; UPDATE cjsi_publication_attempts SET state='sealed',diff_digest=randomblob(32),session_seal=randomblob(32)"); err != nil {
		t.Fatal(err)
	}
	if err = c.Stage(t.Context(), r.db, selectDraft); !errors.Is(err, ledger.ErrProposalMismatch) {
		t.Fatalf("frozen stage: %v", err)
	}
	for _, tc := range []struct{ name, mutation string }{
		{"sealed", ""}, {"member", "UPDATE cjsi_usage_members SET input_tokens=input_tokens+1"},
		{"workset", "UPDATE cjsi_checkpoint_attempts SET complete_end=complete_end-1"},
		{"old-image", "UPDATE usage_events SET output_tokens=output_tokens+1 WHERE event_key='old'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := r.db.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: tc.mutation == ""})
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if tc.mutation != "" {
				if _, err = tx.Exec(tc.mutation); err != nil {
					t.Fatal(err)
				}
			}
			var before, after int
			if err = tx.QueryRow("SELECT total_changes()").Scan(&before); err != nil {
				t.Fatal(err)
			}
			err = c.VerifyStaged(t.Context(), tx, d)
			if tc.mutation == "" && err != nil || tc.mutation != "" && !errors.Is(err, ledger.ErrProposalMismatch) {
				t.Fatalf("verify: %v", err)
			}
			if err = tx.QueryRow("SELECT total_changes()").Scan(&after); err != nil || before != after {
				t.Fatal("verifier wrote a row", err)
			}
		})
	}
}
