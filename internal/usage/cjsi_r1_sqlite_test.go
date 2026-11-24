package usage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

type replayDatabase struct {
	db    *sql.DB
	root  string
	epoch int64
}

// The ordinary store supplies the real WAL, usage events and run schema. This
// test adds the subset of private staging tables exercised by Stage and Apply.
// The managed migration fixture runs the same concurrency and receipt checks.
func newReplayDatabase(t *testing.T) replayDatabase {
	t.Helper()
	root := t.TempDir()
	s, err := store.Open(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	db := s.DB
	_, err = db.Exec(`UPDATE schema_metadata SET version=37;
 CREATE TABLE cjsi_layout_journal(phase INTEGER); INSERT INTO cjsi_layout_journal VALUES(5);
 CREATE TABLE cjsi_publication_head(singleton INTEGER PRIMARY KEY,ticket,core_epoch,index_epoch,session_revision,coverage_revision,protection_revision);
 INSERT INTO cjsi_publication_head SELECT 1,0,epoch,0,0,0,0 FROM derived_snapshot_generation;
 CREATE TABLE cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,input_digest,diff_digest,state,accept_until,index_epoch,session_revision,coverage_revision,protection_revision,PRIMARY KEY(ticket,attempt));
 CREATE TABLE cjsi_checkpoint_attempts(round_id,attempt_id,state,identity,generation,parser,max_line_bytes,capture_end,complete_end);
 CREATE TABLE cjsi_usage_selection(thread PRIMARY KEY,ticket,attempt);
 CREATE TABLE cjsi_usage_groups(ticket,attempt,thread,input_digest,diff_digest,state,gaps,pending,partial,unresolved_overlap,PRIMARY KEY(ticket,attempt,thread));
 CREATE TABLE cjsi_usage_members(ticket,attempt,thread,kind,member_id,turn_id,model,event_at,exact_run,input_tokens,output_tokens,cached,cache_write,reasoning,PRIMARY KEY(ticket,attempt,thread,kind,member_id));
 CREATE TABLE cjsi_usage_sources(ticket,attempt,thread,name_digest,selection_epoch,workset_round,workset_attempt,identity,generation,replay_parser,max_line_bytes,read_end,complete_end,prefix_digest);
 CREATE TABLE cjsi_usage_references(ticket,attempt,thread,kind,member_id,ordinal,name_digest,start_offset,end_offset);
 CREATE TABLE cjsi_usage_intervals(ticket,attempt,thread);
 CREATE TABLE cjsi_old_preconditions(ticket,attempt,thread,event_key,row_hash,disposition,PRIMARY KEY(ticket,attempt,thread,event_key));`)
	if err != nil {
		t.Fatal(err)
	}
	r := replayDatabase{db: db, root: root}
	if err = db.QueryRow("SELECT epoch FROM derived_snapshot_generation").Scan(&r.epoch); err != nil {
		t.Fatal(err)
	}
	var journal string
	if err = db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil || journal != "wal" {
		t.Fatal(journal, err)
	}
	return r
}
func (r replayDatabase) candidate(t *testing.T, ticket int64, body string, history ...ledger.CommittedCall) (CJSIUsageCandidate, CJSIUsageDraft) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.jsonl")
	src, st, err := ledgerStage(t, path, ledgerHead+ledgerTurn+body, nil, 32)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := usageSourceWall(path)
	if err != nil {
		t.Fatal(err)
	}
	w := CJSIUsageWorkset{Round: ticket, Attempt: 1, Selected: ingest.CodexSelectedSource{Path: path, ThreadID: ledgerThread, Epoch: st.Epoch, Evidence: st.Prefix}}
	b := ledger.Basis{Ticket: ticket, Attempt: 1, BaseTicket: ticket - 1, CoreEpoch: r.epoch, InputDigest: sha256.Sum256([]byte(fmt.Sprint(ticket)))}
	h := ledger.History{Assessed: true, Committed: history, Rows: []ledger.OldRow{}}
	for _, call := range history {
		h.Rows = append(h.Rows, call.Prior)
	}
	c, err := PrepareCJSIUsageCandidate(t.Context(), ledger.ReplayInput{Basis: b, Thread: ledgerThread, Expected: []ledger.SourceStamp{st}, History: h}, []CJSIUsagePreparedSource{{entry: entry, binding: w, source: src}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.db.Exec("INSERT INTO cjsi_publication_attempts VALUES(?,?,?,?,?,NULL,'draft','2099-01-01T00:00:00Z',0,0,0,0)", ticket, 1, ticket-1, r.epoch, b.InputDigest[:])
	if err != nil {
		t.Fatal(err)
	}
	p := st.Prefix
	_, err = r.db.Exec("INSERT INTO cjsi_checkpoint_attempts VALUES(?,1,'sealed',?,?,?,?,?,?)", ticket, p.Identity, p.Version.Generation, p.Version.Parser, p.MaxLineBytes, p.Progress.ReadEnd, p.Progress.CompleteEnd)
	if err != nil {
		t.Fatal(err)
	}
	return c, CJSIUsageDraft{Thread: ledgerThread, Stamp: c.Proposal().Stamp(), Worksets: []CJSIUsageWorkset{w}, ExpectedOldRows: h.Rows}
}
func (r replayDatabase) publish(t *testing.T, ticket int64, body string, history ...ledger.CommittedCall) ledger.ProposalStamp {
	t.Helper()
	ctx := t.Context()
	c, d := r.candidate(t, ticket, body, history...)
	if err := c.Stage(ctx, r.db, func(context.Context, CJSIUsageDraftReader, string) (CJSIUsageDraft, error) { return d, nil }); err != nil {
		t.Fatal(err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, q := range []string{"UPDATE cjsi_publication_attempts SET state='opened' WHERE ticket=?", "INSERT OR REPLACE INTO cjsi_usage_selection SELECT thread,ticket,attempt FROM cjsi_usage_groups WHERE ticket=?", "UPDATE cjsi_publication_head SET ticket=?"} {
		if _, err = tx.Exec(q, ticket); err != nil {
			t.Fatal(err)
		}
	}
	if err = c.ApplySelectedRequests(ctx, tx, d.Stamp, d.ExpectedOldRows); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return d.Stamp
}
func testOpenedHistory(t *testing.T) {
	r := newReplayDatabase(t)
	ctx := t.Context()
	read := func(st ledger.ProposalStamp, id string) (ledger.CommittedCall, error) {
		return ReadCJSICommittedCall(ctx, r.db, st, ledger.Key{Kind: ledger.Response, Thread: ledgerThread, ID: id})
	}
	mustRead := func(st ledger.ProposalStamp, id string) ledger.CommittedCall {
		t.Helper()
		c, err := read(st, id)
		if err != nil {
			t.Fatal(id, err)
		}
		return c
	}
	event := func(id string, n int) string { return ledgerNative(id, counter(n), "") }
	one := r.publish(t, 1, event("R1", 10))
	r1 := mustRead(one, "R1")
	two := r.publish(t, 2, event("R2", 20), r1)
	r1 = mustRead(one, "R1")
	r2 := mustRead(two, "R2")
	// The former source has no representative in ticket 2. Removing its
	// explicit carry receipt must break the chain even with the same live row.
	if _, err := r.db.Exec("UPDATE cjsi_old_preconditions SET disposition='retract' WHERE ticket=2"); err != nil {
		t.Fatal(err)
	}
	if _, err := read(one, "R1"); !errors.Is(err, ledger.ErrProposalMismatch) {
		t.Fatal("lost receipt", err)
	}
	if _, err := r.db.Exec("UPDATE cjsi_old_preconditions SET disposition='hold' WHERE ticket=2"); err != nil {
		t.Fatal(err)
	}
	three := r.publish(t, 3, event("R3", 3), r1, r2)
	r1 = mustRead(one, "R1")
	r2 = mustRead(two, "R2")
	r3 := mustRead(three, "R3")
	four := r.publish(t, 4, event("R1", 10)+event("R4", 4), r1, r2, r3)
	if _, err := read(one, "R1"); !errors.Is(err, ledger.ErrProposalMismatch) {
		t.Fatal("superseded ticket still authorizes replacement", err)
	}
	r1 = mustRead(four, "R1")
	r.publish(t, 5, event("R1", 11)+event("R5", 5), r1, mustRead(two, "R2"), mustRead(three, "R3"), mustRead(four, "R4"))
	if _, err := read(four, "R1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("conflict did not retract", err)
	}
	var n, tokens int
	if err := r.db.QueryRow("SELECT count(*),sum(input_tokens) FROM usage_events").Scan(&n, &tokens); err != nil || n != 4 || tokens != 32 {
		t.Fatalf("live calls=%d tokens=%d: %v", n, tokens, err)
	}
}
func testStageWriterLock(t *testing.T) {
	r := newReplayDatabase(t)
	c, d := r.candidate(t, 1, ledgerNative("R", counter(10), ""))
	exerciseStageWriterLock(t, r.db, r.root, c, d)
}

func exerciseStageWriterLock(t *testing.T, db *sql.DB, root string, c CJSIUsageCandidate, d CJSIUsageDraft) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	openPeer := func(timeout int) *sql.DB {
		t.Helper()
		p, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=rw&_pragma=foreign_keys(ON)&_pragma=busy_timeout(%d)", filepath.Join(root, "agentdeck.sqlite3"), timeout))
		if err != nil {
			t.Fatal(err)
		}
		p.SetMaxOpenConns(1)
		t.Cleanup(func() { p.Close() })
		return p
	}
	peer, probe := openPeer(5000), openPeer(0)
	selected := func(context.Context, CJSIUsageDraftReader, string) (CJSIUsageDraft, error) { return d, nil }
	busy := func(err error) bool { var e interface{ Code() int }; return errors.As(err, &e) && e.Code() == 5 }
	// A real busy_timeout waiter checks out its own connection. Observe that
	// it cannot pass the first read (or write) before explicitly releasing
	// the first writer. The database result after release checks which image
	// the waiter actually used.
	waitForBlocked := func(t *testing.T, pool *sql.DB, early <-chan struct{}) {
		t.Helper()
		for pool.Stats().InUse == 0 {
			select {
			case <-early:
				t.Fatal("waiter passed the locked operation")
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			default:
				runtime.Gosched()
			}
		}
		select {
		case <-early:
			t.Fatal("waiter passed the locked operation")
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(40 * time.Millisecond):
		}
	}
	holdStage := func(t *testing.T) (release func(), done <-chan error) {
		t.Helper()
		gate, entered := make(chan struct{}), make(chan struct{})
		release = sync.OnceFunc(func() { close(gate) })
		t.Cleanup(release)
		result := make(chan error, 1)
		go func() {
			result <- c.Stage(ctx, db, func(context.Context, CJSIUsageDraftReader, string) (CJSIUsageDraft, error) {
				close(entered)
				select {
				case <-gate:
				case <-ctx.Done():
				}
				return d, ctx.Err()
			})
		}()
		select {
		case <-entered:
		case err := <-result:
			t.Fatalf("Stage stopped before draft selection: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		return release, result
	}
	t.Run("concurrent first inserts serialize", func(t *testing.T) {
		release, first := holdStage(t)
		if _, err := probe.ExecContext(ctx, "UPDATE cjsi_publication_attempts SET state=state WHERE 0"); !busy(err) {
			t.Fatalf("first draft read did not hold writer: %v", err)
		}
		entered, second := make(chan struct{}), make(chan error, 1)
		go func() {
			second <- c.Stage(ctx, peer, func(context.Context, CJSIUsageDraftReader, string) (CJSIUsageDraft, error) {
				close(entered)
				return d, nil
			})
		}()
		waitForBlocked(t, peer, entered)
		release()
		if err := <-first; err != nil {
			t.Fatal(err)
		}
		if err := <-second; err != nil {
			t.Fatal(err)
		}
		var n int
		if err := db.QueryRow("SELECT count(*) FROM cjsi_usage_groups").Scan(&n); err != nil || n != 1 {
			t.Fatal(n, err)
		}
	})
	t.Run("reuse waits for earlier mutator", func(t *testing.T) {
		for _, tc := range []struct{ name, statement string }{{"epoch", "UPDATE derived_snapshot_generation SET epoch=epoch+1"}, {"retirement", "UPDATE cjsi_publication_attempts SET state='retired'"}} {
			t.Run(tc.name, func(t *testing.T) {
				conn, err := db.Conn(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
					t.Fatal(err)
				}
				defer conn.ExecContext(context.Background(), "ROLLBACK")
				if _, err = conn.ExecContext(ctx, tc.statement); err != nil {
					t.Fatal(err)
				}
				entered, done := make(chan struct{}), make(chan error, 1)
				go func() {
					done <- c.Stage(ctx, peer, func(context.Context, CJSIUsageDraftReader, string) (CJSIUsageDraft, error) {
						close(entered)
						return d, nil
					})
				}()
				waitForBlocked(t, peer, entered)
				if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
					t.Fatal(err)
				}
				if err := <-done; !errors.Is(err, ledger.ErrProposalMismatch) {
					t.Fatalf("reuse read stale state: %v", err)
				}
				if _, err = conn.ExecContext(ctx, "UPDATE derived_snapshot_generation SET epoch=?; UPDATE cjsi_publication_attempts SET state='draft'", d.Stamp.Basis.CoreEpoch); err != nil {
					t.Fatal(err)
				}
			})
		}
	})
	t.Run("later mutator waits for reuse", func(t *testing.T) {
		release, stage := holdStage(t)
		completed, result := make(chan struct{}), make(chan error, 1)
		go func() {
			_, err := peer.ExecContext(ctx, "UPDATE derived_snapshot_generation SET epoch=epoch+1")
			result <- err
			close(completed)
		}()
		waitForBlocked(t, peer, completed)
		release()
		if err := <-stage; err != nil {
			t.Fatal(err)
		}
		if err := <-result; err != nil {
			t.Fatal(err)
		}
		if err := c.Stage(ctx, db, selected); !errors.Is(err, ledger.ErrProposalMismatch) {
			t.Fatalf("reuse after mutation: %v", err)
		}
	})
}
