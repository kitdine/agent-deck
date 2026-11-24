package usage

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func counter(x int) string {
	return fmt.Sprintf(`{"input_tokens":%d,"output_tokens":0,"cached_input_tokens":0}`, x)
}
func oldNotice(at string, last, total int) string {
	fields := `"total_token_usage":` + counter(total)
	if last >= 0 {
		fields += `,"last_token_usage":` + counter(last)
	}
	return fmt.Sprintf(`{"timestamp":"2026-01-02T%sZ","type":"event_msg","payload":{"type":"token_count","thread_id":"%s","turn_id":"t","info":{%s}}}`+"\n", at, ledgerThread, fields)
}
func testMixedLedger(t *testing.T) {
	base := ledgerHead + ledgerTurn + oldNotice("09:00:00", -1, 0) + oldNotice("10:10:00", 20, 20)
	for _, tc := range []struct {
		name                        string
		hi, calls, intervals, total int
	}{
		{"possible overlap", -1, 1, 0, 10}, {"contained", 10, 1, 1, 20}, {"independent", 30, 2, 0, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			extra := ""
			if tc.hi >= 0 {
				extra = `,"thread_token_usage":` + counter(tc.hi)
			}
			body := base + ledgerNative("r", counter(10), extra)
			in := ledgerInput(t, body, body)
			_, v := ledgerView(t, in)
			calls, intervals, total := 0, 0, 0
			for _, m := range v.Members {
				total += int(m.Tokens.Input)
				if m.Key.Kind != ledger.TokenInterval {
					calls++
					continue
				}
				intervals++
				r := m.Interval
				lo := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
				hi := lo.Add(70 * time.Minute)
				if r.LowerAt != lo || r.UpperAt != hi || r.Lower.Input != 0 || r.Upper.Input != 20 {
					t.Fatalf("invalid original interval %+v", r)
				}
				all, over := r.InWindow(lo, hi)
				if all || !over {
					t.Fatalf("crossing right-exclusive window %v %v", all, over)
				}
				all, over = r.InWindow(lo, hi.Add(time.Nanosecond))
				if !all || !over {
					t.Fatal("whole interval unavailable")
				}
			}
			if calls != tc.calls || intervals != tc.intervals || total != tc.total {
				t.Fatalf("calls=%d intervals=%d tokens=%d gaps=%x", calls, intervals, total, v.Gaps)
			}
		})
	}
}
func testLedgerCopies(t *testing.T) {
	head := ledgerHead + ledgerTurn
	a := ledgerNative("stable", counter(6), "")
	b := ledgerNative("stable", counter(9), "")
	c := ledgerNative("independent", counter(6), "")
	in := ledgerInput(t, head+a+c, head+b+c)
	p, v := ledgerView(t, in)
	if v.Gaps&ledger.ConflictingResponse == 0 || len(v.Members) != 1 || v.Members[0].Key.ID != "independent" || len(v.Members[0].Proof) != 2 {
		t.Fatal(v)
	}
	slices.Reverse(in.Sources)
	again, _ := ledgerView(t, in)
	if p.Stamp() != again.Stamp() {
		t.Fatal("source enumeration changed proposal")
	}
	invalids := []string{`{"output_tokens":1}`, `{"input_tokens":1,"output_tokens":0,"reasoning_output_tokens":1}`, `{"input_tokens":9223372036854775807,"output_tokens":1}`, `{"input_tokens":-1,"output_tokens":0}`}
	for _, bad := range invalids {
		_, v := ledgerView(t, ledgerInput(t, head+ledgerNative("bad", bad, "")))
		if v.Gaps&ledger.InvalidMeter == 0 || len(v.Members) != 0 {
			t.Fatal(v)
		}
	}
	_, v = ledgerView(t, ledgerInput(t, head+ledgerNative("r", counter(0), "")))
	if len(v.Members) != 1 || v.Members[0].Tokens.Known&ledger.CachedKnown == 0 {
		t.Fatalf("explicit zero discarded: %+v", v)
	}
}

func testLedgerDraftSelection(t *testing.T) {
	ctx := t.Context()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.DB.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Isolate the selector from the rest of the managed schema. The full
	// managed migration and typed staging are exercised with the store fixture.
	exec(`CREATE TABLE cjsi_layout_journal(phase INTEGER);INSERT INTO cjsi_layout_journal VALUES(5);
 CREATE TABLE cjsi_publication_head(singleton,ticket,core_epoch,index_epoch,session_revision,coverage_revision,protection_revision);
 CREATE TABLE cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,input_digest,diff_digest,state,accept_until,index_epoch,session_revision,coverage_revision,protection_revision)`)
	body := ledgerHead + ledgerTurn + ledgerNative("r", counter(3), "")
	in := ledgerInput(t, body)
	var live int64
	if err := db.DB.QueryRow("SELECT epoch FROM derived_snapshot_generation").Scan(&live); err != nil {
		t.Fatal(err)
	}
	in.Basis.CoreEpoch = live
	for _, key := range []string{"retained", "outside"} {
		exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES(?,'codex','s','t','2026-01-02T00:00:00Z','m','old',0)`, key)
	}
	image, err := ReadCJSIOldUsage(ctx, db.DB, "retained")
	if err != nil {
		t.Fatal(err)
	}
	row := image.Precondition()
	b := in.Basis
	p, _ := ledgerView(t, in)
	exec("INSERT INTO cjsi_publication_head VALUES(1,?,?,0,0,0,0)", b.BaseTicket, b.CoreEpoch)
	exec("INSERT INTO cjsi_publication_attempts VALUES(?,?,?,?,?,NULL,'draft','2099-01-01T00:00:00Z',0,0,0,0)", b.Ticket, b.Attempt, b.BaseTicket, b.CoreEpoch, b.InputDigest[:])
	c := CJSIUsageCandidate{thread: in.Thread, proposal: p, old: []ledger.OldRow{row}}
	d := CJSIUsageDraft{Thread: in.Thread, Stamp: p.Stamp(), ExpectedOldRows: []ledger.OldRow{row}}
	check := func(c CJSIUsageCandidate, d CJSIUsageDraft, want bool) {
		t.Helper()
		tx, err := db.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		err = c.checkDraft(ctx, tx, d)
		if (err == nil) != want || err != nil && !errors.Is(err, ledger.ErrProposalMismatch) {
			t.Fatalf("checkDraft=%v want accepted=%t", err, want)
		}
	}
	check(c, d, true)
	omitted := c
	omitted.old = nil
	check(omitted, d, false)
	bad := d
	bad.ExpectedOldRows = nil
	check(omitted, bad, false)
	exec("UPDATE usage_events SET output_tokens=1 WHERE event_key='outside'")
	check(c, d, true)
	exec("UPDATE usage_events SET output_tokens=1 WHERE event_key='retained'")
	check(c, d, false)
	exec("UPDATE usage_events SET output_tokens=0 WHERE event_key='retained'")
	exec("UPDATE derived_snapshot_generation SET epoch=epoch+1")
	check(c, d, false)
	exec("UPDATE derived_snapshot_generation SET epoch=?", live)
	check(c, d, true)
}

func testIdlessLedger(t *testing.T) {
	head := ledgerHead + ledgerTurn
	known := ledgerNative("known", counter(10), `,"thread_token_usage":`+counter(10))
	extra := ledgerNative("", counter(5), `,"thread_token_usage":`+counter(15))
	_, v := ledgerView(t, ledgerInput(t, head+known+extra))
	if len(v.Members) != 2 || v.Members[0].Key.Kind != ledger.Response || v.Members[1].Key.Kind != ledger.TokenInterval || v.Members[1].Tokens.Input != 5 {
		t.Fatal(v)
	}
	_, v = ledgerView(t, ledgerInput(t, head+extra))
	if len(v.Members) != 0 || !v.Pending {
		t.Fatal("isolated idless notification claimed ownership", v)
	}
	legacy := oldNotice("09:50:00", -1, 0) + oldNotice("10:10:00", 10, 10)
	_, v = ledgerView(t, ledgerInput(t, head+legacy+ledgerNative("", counter(10), `,"thread_token_usage":`+counter(10))))
	if len(v.Members) != 1 || v.Members[0].Key.Kind != ledger.LegacyCall || v.Members[0].Tokens.Input != 10 {
		t.Fatal("unique legacy alias", v)
	}
}
