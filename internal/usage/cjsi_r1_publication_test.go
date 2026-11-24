package usage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func testLegacyTurnReplacement(t *testing.T) {
	ctx := t.Context()
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "sessions", "mismatch.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	start := ledgerHead + `{"type":"turn_context","payload":{"turn_id":"t1","model":"m"}}` + "\n"
	line := fmt.Sprintf(`{"timestamp":"2026-01-02T10:00:00Z","type":"event_msg","payload":{"type":"token_count","thread_id":"%s","turn_id":"t2","info":{"last_token_usage":%s}}}`+"\n", ledgerThread, counter(10))
	body := start + line

	// Run real v8 scanner to see exact v8 row produced before migration.
	v8, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer v8.Close()
	src, st, err := ledgerStage(t, path, body, nil, 16)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(v8, home)
	svc.MachineIdentity = func(context.Context) (string, error) { return "repro", nil }
	if _, err := svc.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var key string
	if err := v8.DB.QueryRow(`SELECT event_key FROM usage_events WHERE source_path=?`, path).Scan(&key); err != nil {
		t.Fatal(err)
	}
	prior, err := ReadCJSIOldUsage(ctx, v8.DB, key)
	if err != nil {
		t.Fatal(err)
	}

	// Build candidate with genuine v8 history.
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := v8.DB.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE cjsi_usage_selection(ticket,attempt,thread);
 CREATE TABLE cjsi_usage_groups(ticket,attempt,thread,input_digest,diff_digest);
 CREATE TABLE cjsi_publication_head(singleton,ticket,core_epoch)`)

	in := ledgerInput(t)
	if err := v8.DB.QueryRow("SELECT epoch FROM derived_snapshot_generation").Scan(&in.Basis.CoreEpoch); err != nil {
		t.Fatal(err)
	}
	in.Expected = []ledger.SourceStamp{st}
	in.History = ledger.History{
		Assessed: true,
		Rows:     []ledger.OldRow{prior.Precondition()},
		Evidence: []ledger.Prior{{
			Image: prior,
			Anchor: ledger.Anchor{
				Original: ledger.Reference{Source: st, Span: ingest.CodexSpan{Start: int64(len(start)), End: int64(len(body))}},
				Through:  sha256.Sum256([]byte(body)),
			},
		}},
	}
	e := InventoryEntry{Client: "codex", Path: path, Identity: st.Prefix.Identity, Size: int64(len(body))}
	w := CJSIUsageWorkset{Round: 1, Attempt: 1, Selected: ingest.CodexSelectedSource{Path: path, ThreadID: ledgerThread, Epoch: st.Epoch, Evidence: st.Prefix}}
	c, err := PrepareCJSIUsageCandidate(ctx, in, []CJSIUsagePreparedSource{{entry: e, binding: w, source: src}})
	if err != nil {
		t.Fatal(err)
	}
	stamp := c.Proposal().Stamp()
	delta, err := c.Proposal().Changes(stamp, ledgerThread)
	if err != nil {
		t.Fatal(err)
	}
	exec("INSERT INTO cjsi_publication_head VALUES(1,?,?)", stamp.Basis.Ticket, stamp.Basis.CoreEpoch)
	exec("INSERT INTO cjsi_usage_groups VALUES(?,?,?,?,?)", stamp.Basis.Ticket, stamp.Basis.Attempt, ledgerThread, stamp.Basis.InputDigest[:], stamp.Diff[:])
	exec("INSERT INTO cjsi_usage_selection VALUES(?,?,?)", stamp.Basis.Ticket, stamp.Basis.Attempt, ledgerThread)

	tx, err := v8.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	applyErr := c.ApplySelectedRequests(ctx, tx, stamp, []ledger.OldRow{prior.Precondition()})
	if applyErr != nil || len(delta.Retract) != 1 || delta.Retract[0] != prior.Precondition() {
		t.Fatalf("turn replacement: prior=%s delta=%+v err=%v", prior.EventID, delta, applyErr)
	}
	var turn string
	if err := tx.QueryRow("SELECT event_id FROM usage_events WHERE event_key=?", key).Scan(&turn); err != nil || turn != "t2" {
		t.Fatalf("selected turn=%q: %v", turn, err)
	}
}

func testPartialRunOverlap(t *testing.T) {
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
	exec(`CREATE TABLE cjsi_usage_selection(ticket,attempt,thread);
 CREATE TABLE cjsi_usage_groups(ticket,attempt,thread,input_digest,diff_digest);
 CREATE TABLE cjsi_publication_head(singleton,ticket,core_epoch)`)

	prefix := ledgerHead + ledgerTurn
	req := ledgerNative("R", counter(10), "")
	body := prefix + req
	path := filepath.Join(t.TempDir(), "runs.jsonl")
	src, st, err := ledgerStage(t, path, body, nil, 16)
	if err != nil {
		t.Fatal(err)
	}
	spanStart, spanEnd := int64(len(prefix)), int64(len(body))
	// Run 1 covers [0, spanEnd] (contains span). Run 2 is a conflicting completed
	// exact run covering [spanStart+5, spanEnd+50] (partially overlaps span!).
	exec(`INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact,ambiguity_reason)
 VALUES(1,'codex','p1','1','2026-01-02T09:00:00Z','2026-01-02T11:00:00Z',1,''),
       (2,'codex','p2','2','2026-01-02T09:30:00Z','2026-01-02T11:30:00Z',1,'')`)
	exec(`INSERT INTO usage_run_sources(run_id,path,start_offset,end_offset,start_hash,end_hash)
 VALUES(1,?,0,?,'sh1','eh1'),
       (2,?,?,?,'sh2','eh2')`, path, spanEnd, path, spanStart+5, spanEnd+50)

	in := ledgerInput(t)
	if err := db.DB.QueryRow("SELECT epoch FROM derived_snapshot_generation").Scan(&in.Basis.CoreEpoch); err != nil {
		t.Fatal(err)
	}
	in.Expected = []ledger.SourceStamp{st}
	in.History = ledger.History{
		Assessed: true,
		Runs: []ledger.RunRange{{
			OriginalPath: path, StartHash: "sh1", EndHash: "eh1",
			RunID: 1, Completed: true, Exact: true, Source: st,
			Bytes: ingest.CodexSpan{Start: 0, End: spanEnd},
		}},
	}
	e := InventoryEntry{Client: "codex", Path: path, Identity: st.Prefix.Identity, Size: int64(len(body))}
	w := CJSIUsageWorkset{Round: 1, Attempt: 1, Selected: ingest.CodexSelectedSource{Path: path, ThreadID: ledgerThread, Epoch: st.Epoch, Evidence: st.Prefix}}
	c, err := PrepareCJSIUsageCandidate(ctx, in, []CJSIUsagePreparedSource{{entry: e, binding: w, source: src}})
	if err != nil {
		t.Fatal(err)
	}
	stamp := c.Proposal().Stamp()
	exec("INSERT INTO cjsi_publication_head VALUES(1,?,?)", stamp.Basis.Ticket, stamp.Basis.CoreEpoch)
	exec("INSERT INTO cjsi_usage_groups VALUES(?,?,?,?,?)", stamp.Basis.Ticket, stamp.Basis.Attempt, ledgerThread, stamp.Basis.InputDigest[:], stamp.Diff[:])
	exec("INSERT INTO cjsi_usage_selection VALUES(?,?,?)", stamp.Basis.Ticket, stamp.Basis.Attempt, ledgerThread)

	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := c.ApplySelectedRequests(ctx, tx, stamp, []ledger.OldRow{}); err != nil {
		t.Fatal(err)
	}
	eventKey := (ledger.Key{Kind: ledger.Response, Thread: ledgerThread, ID: "R"}).EventKey()
	countBindings := func(want int) {
		t.Helper()
		var n int
		if err := tx.QueryRow("SELECT count(*) FROM usage_run_bindings WHERE event_key=?", eventKey).Scan(&n); err != nil || n != want {
			t.Fatalf("bindings=%d want=%d: %v", n, want, err)
		}
	}
	countBindings(0)
	// The adjacent half-open range does not conflict.
	if _, err := tx.Exec("UPDATE usage_run_sources SET start_offset=? WHERE run_id=2", spanEnd); err != nil {
		t.Fatal(err)
	}
	view, err := c.Proposal().Eligibility(stamp, ledgerThread)
	if err != nil || len(view.Members) != 1 {
		t.Fatal(view, err)
	}
	if err := c.bindVerifiedRun(ctx, tx, eventKey, view.Members[0]); err != nil {
		t.Fatal(err)
	}
	countBindings(1)
}
