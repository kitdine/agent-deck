package usage

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// The fixture supplies only the final coordinator decision. The actual
// selected request writer, run bindings, checkpoints and sessions run in tx.
type writerDecision struct {
	CJSIUsagePublication
	rejectAfterWrite bool
}

func (f writerDecision) Publish(_ context.Context, _ *sql.Tx, _ InventoryEntry, _ CJSIUsageCandidate, apply func([]ledger.OldRow) error) (bool, error) {
	if err := apply([]ledger.OldRow{}); err != nil {
		return false, err
	}
	if f.rejectAfterWrite {
		return false, ledger.ErrProposalMismatch
	}
	return false, nil
}

func testSelectedWriter(t *testing.T) {
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
	body := ledgerHead + ledgerTurn + oldNotice("09:00:00", -1, 0) + oldNotice("10:10:00", 20, 20) + ledgerNative("R", counter(10), `,"thread_token_usage":`+counter(10))
	path := t.TempDir() + "/source.jsonl"
	src, st, err := ledgerStage(t, path, body, nil, 20)
	if err != nil {
		t.Fatal(err)
	}
	in := ledgerInput(t)
	if err = db.DB.QueryRow("SELECT epoch FROM derived_snapshot_generation").Scan(&in.Basis.CoreEpoch); err != nil {
		t.Fatal(err)
	}
	in.Expected = []ledger.SourceStamp{st}
	e := InventoryEntry{Client: "codex", Path: path, Identity: st.Prefix.Identity, Size: int64(len(body))}
	w := CJSIUsageWorkset{Round: 1, Attempt: 1, Selected: ingest.CodexSelectedSource{Path: path, ThreadID: ledgerThread, Epoch: st.Epoch, Evidence: st.Prefix}}
	c, err := PrepareCJSIUsageCandidate(ctx, in, []CJSIUsagePreparedSource{{entry: e, binding: w, source: src}})
	if err != nil {
		t.Fatal(err)
	}
	stamp := c.Proposal().Stamp()
	b := stamp.Basis
	exec("INSERT INTO cjsi_publication_head VALUES(1,?,?)", b.Ticket, b.CoreEpoch)
	exec("INSERT INTO cjsi_usage_groups VALUES(?,?,?,?,?)", b.Ticket, b.Attempt, ledgerThread, b.InputDigest[:], stamp.Diff[:])
	run := func(reject bool) error {
		svc := New(db, t.TempDir())
		svc.cjsiManaged = true
		svc.CJSIPublication = writerDecision{rejectAfterWrite: reject}
		tx, err := db.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		p := preparedSource{candidate: &c, entry: e, r: newSourceStats()}
		if err := svc.publishPrepared(ctx, tx, &p); err != nil {
			return err
		}
		return tx.Commit()
	}
	if err = run(false); !errors.Is(err, ledger.ErrProposalMismatch) {
		t.Fatalf("unselected group: %v", err)
	}
	exec("INSERT INTO cjsi_usage_selection VALUES(?,?,?)", b.Ticket, b.Attempt, ledgerThread)
	if err = run(true); !errors.Is(err, ledger.ErrProposalMismatch) {
		t.Fatalf("late CAS failure: %v", err)
	}
	var n int
	for _, table := range []string{"usage_events", "usage_sessions", "usage_source_files"} {
		if err = db.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("leaked %s %d: %v", table, n, err)
		}
	}
	if err = run(false); err != nil {
		t.Fatal(err)
	}
	var tokens, version int64
	var session, eventID string
	if err = db.DB.QueryRow(`SELECT count(*),sum(input_tokens),session_id,event_id FROM usage_events`).Scan(&n, &tokens, &session, &eventID); err != nil || n != 1 || tokens != 10 || session != ledgerThread || eventID != "R" {
		t.Fatalf("public calls=%d tokens=%d session=%s event=%s err=%v", n, tokens, session, eventID, err)
	}
	if err = db.DB.QueryRow("SELECT parser_version FROM usage_source_files WHERE path=? AND cursor=size AND session_id=? AND thread_id=?", path, ledgerThread, ledgerThread).Scan(&version); err != nil || version != ledger.Parser {
		t.Fatalf("checkpoint=%d: %v", version, err)
	}
	if err = db.DB.QueryRow("SELECT count(*) FROM usage_sessions WHERE client='codex' AND session_id=?", ledgerThread).Scan(&n); err != nil || n != 1 {
		t.Fatalf("sessions=%d: %v", n, err)
	}
}

type missingDecision struct{ CJSIUsagePublication }

func (missingDecision) ProtectMissing(ctx context.Context, tx *sql.Tx, path string) (bool, error) {
	if path == "gone" || path == "unregistered" {
		_, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO held VALUES(?)", path)
		return true, err
	}
	return false, nil
}

func testLedgerMissing(t *testing.T) {
	ctx := t.Context()
	st, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := New(st, t.TempDir())
	svc.cjsiManaged = true
	svc.CJSIPublication = missingDecision{}
	_, err = st.DB.ExecContext(ctx, `CREATE TABLE held(path PRIMARY KEY);
 INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('gone','old',12,12,'hash');
 INSERT INTO usage_runs(id,client,provider,multiplier,started_at) VALUES(1,'codex','p','2','2026-01-01');
 INSERT INTO usage_run_sources(run_id,path,start_offset,start_hash) VALUES(1,'gone',0,'');
 INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset)
 VALUES('p','codex','p','r','2026-01-02T00:00:00Z','m','gone',1),
 ('u','codex','u','r','2026-01-02T00:00:00Z','m','unregistered',2),
 ('other','claude','x','r','2026-01-02T00:00:00Z','m','ordinary-cleanup',1)`)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.detachSource(ctx, "gone"); !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
		t.Fatalf("missing source status: %v", err)
	}
	n, err := svc.cleanupOrphanedEvents(ctx)
	if n != 1 || !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
		t.Fatalf("removed=%d: %v", n, err)
	}
	for q, want := range map[string]int{"SELECT count(*) FROM held": 2, "SELECT count(*) FROM usage_source_files WHERE path='gone'": 1, "SELECT count(*) FROM usage_run_sources WHERE path='gone'": 1, "SELECT count(*) FROM usage_events WHERE event_key IN ('p','u')": 2, "SELECT count(*) FROM usage_events WHERE event_key='other'": 0} {
		var got int
		if err := st.DB.QueryRow(q).Scan(&got); err != nil || got != want {
			t.Fatalf("%s = %d want %d: %v", q, got, want, err)
		}
	}
}
