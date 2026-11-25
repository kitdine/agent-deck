package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// Synthetic domain plan. All facts, the scoped preimage, selection, historical
// receipt and range stamp use real migrated core tables and production triggers.
type cjsiOpenFixtureDelta struct {
	p                    cjsiSlotProposal
	next                 cjsiOpenTarget
	group, scope, window [32]byte
	calls                int
	afterApply           func(*sql.Tx) error
	afterStamp           func(*sql.Tx) error
}

func (d *cjsiOpenFixtureDelta) binding() (cjsiSlotProposal, cjsiOpenTarget) { return d.p, d.next }

func (d *cjsiOpenFixtureDelta) apply(ctx context.Context, tx *sql.Tx) error {
	d.calls++
	p := d.p
	var one int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_usage_groups WHERE ticket=? AND attempt=? AND thread='fixture' AND input_digest=? AND diff_digest=?`,
		p.ticket, p.attempt, p.input[:], d.group[:]).Scan(&one); err != nil {
		return err
	}
	// All three inserts have an absent-key precondition enforced by their PK.
	if err := cjsiOpenOne(tx.ExecContext(ctx, `INSERT INTO main.cjsi_takeovers(client,thread,period,opened_ticket,attempt,receipt_version,safe_old_side,proof_digest)
		VALUES('codex','fixture',1,?,?,1,1,?)`, p.ticket, p.attempt, p.input[:])); err != nil {
		return err
	}
	if err := cjsiOpenOne(tx.ExecContext(ctx, `INSERT INTO main.usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset)
		VALUES('open-event','codex','fixture','ev','2030-01-01','unit',17,'fixture.jsonl',0)`)); err != nil {
		return err
	}
	if err := cjsiOpenOne(tx.ExecContext(ctx, `INSERT INTO main.cjsi_usage_selection(thread,ticket,attempt) VALUES('fixture',?,?)`, p.ticket, p.attempt)); err != nil {
		return err
	}
	if err := cjsiOpenOne(tx.ExecContext(ctx, `UPDATE main.cjsi_scope_overlays SET pending=0,coverage_revision=?
		WHERE domain='usage' AND scope_kind='thread' AND scope_key=? AND window_key=? AND lower_at IS NULL AND upper_at IS NULL
		AND pending=1 AND partial=0 AND masked=0 AND protection_revision=? AND coverage_revision=?`,
		d.next.session.CoverageRevision, d.scope[:], d.window[:], p.base.ProtectionRevision, p.base.CoverageRevision)); err != nil {
		return err
	}
	if d.afterApply != nil {
		return d.afterApply(tx)
	}
	return nil
}

func (d *cjsiOpenFixtureDelta) stampSelected(ctx context.Context, tx *sql.Tx) error {
	if err := cjsiOpenOne(tx.ExecContext(ctx, `INSERT INTO main.cjsi_usage_interval_dirty_ranges
		SELECT core_epoch,'client-summary','codex','',1,2,ticket FROM main.cjsi_publication_head WHERE singleton=1 AND ticket=?`, d.p.ticket)); err != nil {
		return err
	}
	if d.afterStamp != nil {
		return d.afterStamp(tx)
	}
	return nil
}

func (d *cjsiOpenFixtureDelta) verify(ctx context.Context, q cjsiRowQueryer) error {
	p := d.p
	var one int
	return q.QueryRowContext(ctx, `SELECT 1 FROM main.usage_events e,main.cjsi_scope_overlays c,main.cjsi_usage_selection s,
		main.cjsi_takeovers t,main.cjsi_usage_interval_dirty_ranges r
		WHERE e.event_key='open-event' AND e.client='codex' AND e.session_id='fixture' AND e.input_tokens=17 AND e.model='unit'
		AND c.domain='usage' AND c.scope_kind='thread' AND c.scope_key=? AND c.window_key=? AND c.masked=0 AND c.partial=0 AND c.pending=0
		AND c.protection_revision=? AND c.coverage_revision=? AND s.thread='fixture' AND s.ticket=? AND s.attempt=?
		AND t.client='codex' AND t.thread=s.thread AND t.period=1 AND t.opened_ticket=s.ticket AND t.attempt=s.attempt AND t.proof_digest=?
		AND r.core_epoch=? AND r.scope_kind='client-summary' AND r.client='codex' AND r.model='' AND r.from_day=1 AND r.to_day=2 AND r.ticket=s.ticket`,
		d.scope[:], d.window[:], p.base.ProtectionRevision, d.next.session.CoverageRevision, p.ticket, p.attempt, p.input[:], p.base.CoreEpoch).Scan(&one)
}

// Pin a synthetic file with previously prepared bytes. The slot writer and
// worker-budget machinery have separate tests. This fixture supplies the
// authenticated readback to both admission and final qualification.
func cjsiOpenFixtureDisk(t *testing.T, f cjsiSlotFixture) cjsiSlotReceipt {
	t.Helper()
	r := f.admission.proposal.expected
	path := filepath.Join(f.admission.rootPath, fmt.Sprintf(".cjsi-serving-slot-%x", r.identity))
	prepared, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = prepared.Write([]byte("prepared")); err == nil {
		err = prepared.Sync()
	}
	if err = errors.Join(err, prepared.Close()); err != nil {
		t.Fatal(err)
	}
	dir, err := os.Open(f.admission.rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = errors.Join(dir.Sync(), dir.Close()); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	r.digest, r.size = sha256.Sum256(data), int64(len(data))
	return r
}

func newCJSIOpenFixture(t *testing.T, seal bool) (*sql.DB, cjsiCombinedOpen, *cjsiOpenFixtureDelta) {
	t.Helper()
	f, db := newCJSISlotFixture(t)
	p := f.admission.proposal
	next := cjsiOpenTarget{p.base, p.usageRevision + 1} // one real usage_events insert
	s := &next.session
	s.Ticket, s.SessionTicket, s.SessionAttempt, s.SessionSeal = p.ticket, p.ticket, p.attempt, p.sessionSeal
	s.CoverageRevision++
	s.SessionRevision++
	d := &cjsiOpenFixtureDelta{next: next, group: f.groupDiff, scope: sha256.Sum256([]byte("fixture thread")), window: sha256.Sum256([]byte("fixture window"))}
	// Commit this fixed synthetic edit list, preimages and expected stamps to
	// the fixture's aggregate diff BEFORE registering the private slot seal.
	p.diff = sha256.Sum256([]byte(fmt.Sprintf("fixture/v1:absent-event=17/selection/receipt/range;pending=1->0;%x/%x/%x/%+v/%d", d.group, d.scope, d.window, next.session, next.usageRevision)))
	d.p = p
	f.admission.proposal = p
	cjsiFixtureExec(t, db, `UPDATE cjsi_publication_attempts SET diff_digest=? WHERE ticket=? AND attempt=?`, p.diff[:], p.ticket, p.attempt)
	cjsiFixtureExec(t, db, `INSERT INTO cjsi_scope_overlays(domain,scope_kind,scope_key,window_key,pending,partial,masked,coverage_revision,protection_revision)
		VALUES('usage','thread',?,?,1,0,0,?,?)`, d.scope[:], d.window[:], p.base.CoverageRevision, p.base.ProtectionRevision)

	// A real private index candidate. The session owner normally constructs
	// and verifies its seal; the digest and source material here are synthetic.
	index, err := sql.Open("sqlite", cjsiSQLiteURI(filepath.Join(f.admission.rootPath, cjsiPrivateIndexName), "rw")+"&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = index.Close() })
	cjsiFixtureExec(t, index, `INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,parent_ticket,protection_revision,coverage_revision,session_revision)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, p.ticket, p.attempt, s.CoreEpoch, s.IndexEpoch, s.BaseTicket, s.BaseAttempt, s.BaseSeal[:], s.BaseRevision, p.base.Ticket, s.ProtectionRevision, s.CoverageRevision, s.SessionRevision)
	cjsiFixtureExec(t, index, `INSERT INTO cjsi_candidate_sources(ticket,attempt,source_path,op,source_kind,identity,priority,parser_version,complete_end,size,modified_at,changed_at,prefix_hash,scanned_at)
		VALUES(?,?,'fixture.jsonl','upsert','synthetic','fixture',1,7,0,0,1,1,'p','t')`, p.ticket, p.attempt)
	cjsiFixtureExec(t, index, `INSERT INTO cjsi_candidate_groups(ticket,attempt,source_path,client,session_id,op,project,model,parser_version,first_at,last_at,order_verified,sequence_id)
		VALUES(?,?,'fixture.jsonl','codex','fixture','replace','fixture','unit',7,'','',1,?)`, p.ticket, p.attempt, d.group[:])
	cjsiFixtureExec(t, index, `UPDATE cjsi_session_candidates SET seal_digest=? WHERE ticket=? AND attempt=?`, p.sessionSeal[:], p.ticket, p.attempt)
	disk := cjsiOpenFixtureDisk(t, f)
	pin := beginCJSIFixtureTx(t, index)
	var one int
	if err := pin.QueryRow(`SELECT 1 FROM cjsi_session_candidates WHERE ticket=? AND attempt=? AND parent_ticket=? AND base_seal=? AND session_revision=? AND coverage_revision=? AND seal_digest=?`,
		p.ticket, p.attempt, p.base.Ticket, s.BaseSeal[:], s.SessionRevision, s.CoverageRevision, p.sessionSeal[:]).Scan(&one); err != nil {
		t.Fatal(err)
	}
	if seal {
		tx := beginCJSIFixtureTx(t, db)
		if _, err := registerCJSISlotReceipt(t.Context(), tx, p, disk, f.selectCurrent); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	o, err := bindCJSICombinedOpen(f.admission, disk, next, d)
	if err != nil {
		t.Fatal(err)
	}
	return db, o, d
}

func cjsiOpenExpectClosed(t *testing.T, tx *sql.Tx, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("unexpected public opening")
	}
	if got := tx.Commit(); !errors.Is(got, sql.ErrTxDone) {
		t.Fatalf("failed transaction remained committable: %v (open error %v)", got, err)
	}
}

func checkCJSIOpenView(t *testing.T, tx *sql.Tx, o cjsiCombinedOpen, opened bool) {
	t.Helper()
	want := o.admission.proposal.base
	var slotTicket, slotAttempt int64
	var seal []byte
	var event, selection, pending, revision int
	err := tx.QueryRow(`SELECT slot_ticket,slot_attempt,slot_seal,(SELECT coalesce(sum(input_tokens),0) FROM usage_events WHERE event_key='open-event'),
		(SELECT count(*) FROM cjsi_usage_selection WHERE thread='fixture'),(SELECT pending FROM cjsi_scope_overlays LIMIT 1),
		(SELECT revision FROM serving_generation) FROM cjsi_publication_head`).Scan(&slotTicket, &slotAttempt, &seal, &event, &selection, &pending, &revision)
	if err != nil {
		t.Fatal(err)
	}
	if opened {
		want = o.next.session
		if slotTicket != want.Ticket || slotAttempt != want.SessionAttempt || len(seal) != 32 || [32]byte(seal) != o.slotSeal || event != 17 || selection != 1 || pending != 0 || int64(revision) != o.next.usageRevision {
			t.Fatal("mixed new slot, usage, selection or coverage")
		}
	} else if slotTicket != 0 || slotAttempt != 0 || seal != nil || event != 0 || selection != 0 || pending != 1 || int64(revision) != o.admission.proposal.usageRevision {
		t.Fatal("private candidate escaped into old view")
	}
	head, err := cjsiReadIndexHead(t.Context(), tx)
	want.Device, want.Inode = 0, 0
	if err != nil || head != want {
		t.Fatalf("head %+v, want %+v: %v", head, want, err)
	}
}

func TestCJSIPublicationLifecycle(t *testing.T) {
	db, o, d := newCJSIOpenFixture(t, true)
	reader := openCJSISlotCore(t, o.admission.rootPath)
	old := beginCJSIFixtureTx(t, reader)
	checkCJSIOpenView(t, old, o, false)
	writer := beginCJSIFixtureTx(t, db)
	if err := o.open(t.Context(), writer); err != nil {
		t.Fatal(err)
	}
	if err := o.confirmCommitted(t.Context(), reader); err == nil {
		t.Fatal("recovery accepted an uncommitted opening")
	}
	during := beginCJSIFixtureTx(t, reader)
	checkCJSIOpenView(t, during, o, false)
	if err := during.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	checkCJSIOpenView(t, old, o, false)
	if err := old.Rollback(); err != nil {
		t.Fatal(err)
	}
	// A fresh post-restart snapshot resolves both components and the actual
	// effective session group, not just the two numeric ticket fields.
	reopened := openCJSISlotCore(t, o.admission.rootPath)
	fresh := beginCJSIFixtureTx(t, reopened)
	checkCJSIOpenView(t, fresh, o, true)
	if err := WithCJSIIndexSnapshot(t.Context(), o.admission.rootPath, fresh, func(s CJSIIndexSnapshot) error {
		if s.Selection() != o.next.session {
			t.Fatalf("unexpected component: %+v", s.Selection())
		}
		var one int
		return s.Tx.QueryRow("SELECT 1 FROM (" + s.CopiesSQL() + ") WHERE client='codex' AND session_id='fixture' AND model='unit'").Scan(&one)
	}); err != nil {
		t.Fatal(err)
	}
	if err := o.confirmCommitted(t.Context(), reopened); err != nil {
		t.Fatal(err)
	}
	if err := fresh.Rollback(); err != nil {
		t.Fatal(err)
	}
	retry := beginCJSIFixtureTx(t, db)
	cjsiOpenExpectClosed(t, retry, o.open(t.Context(), retry))
	if d.calls != 1 {
		t.Fatalf("applied %d times", d.calls)
	}
	// A matching head alone cannot certify the original domain receipt.
	cjsiFixtureExec(t, db, "UPDATE cjsi_takeovers SET proof_digest=randomblob(32)")
	if err := o.confirmCommitted(t.Context(), reopened); err == nil {
		t.Fatal("confirmed altered receipt with unchanged head")
	}
	cjsiFixtureExec(t, db, "UPDATE cjsi_takeovers SET proof_digest=?", d.p.input[:])
	cjsiFixtureExec(t, db, "UPDATE cjsi_publication_head SET coverage_revision=coverage_revision+1")
	if err := o.confirmCommitted(t.Context(), reader); err == nil {
		t.Fatal("confirmed a superseded result")
	}
}

func TestCJSIPublicationOpenRollback(t *testing.T) {
	db, o, d := newCJSIOpenFixture(t, true)
	before := cjsiSlotCoreImage(t, db)
	for _, tc := range []struct {
		name  string
		setup func(*sql.Tx)
	}{
		{"after-domain-writes", func(*sql.Tx) { d.afterApply = func(*sql.Tx) error { return errors.New("domain failure") } }},
		{"post-apply-protection-drift", func(*sql.Tx) {
			d.afterApply = func(tx *sql.Tx) error {
				_, err := tx.Exec("UPDATE cjsi_publication_head SET protection_revision=protection_revision+1")
				return err
			}
		}},
		{"wrong-applied-usage", func(*sql.Tx) {
			d.afterApply = func(tx *sql.Tx) error {
				_, err := tx.Exec("DELETE FROM usage_events WHERE event_key='open-event'")
				return err
			}
		}},
		{"after-head-and-stamps", func(*sql.Tx) { d.afterStamp = func(*sql.Tx) error { return errors.New("late stamp failure") } }},
		{"unacknowledged-scope", func(*sql.Tx) {
			d.afterStamp = func(tx *sql.Tx) error { _, err := tx.Exec("UPDATE cjsi_scope_overlays SET masked=1"); return err }
		}},
		{"late-attempt-drift", func(*sql.Tx) {
			d.afterStamp = func(tx *sql.Tx) error {
				_, err := tx.Exec("UPDATE cjsi_publication_attempts SET accept_until='2000-01-01T00:00:00Z'")
				return err
			}
		}},
		{"denied-final-write", func(tx *sql.Tx) {
			cjsiSlotExec(t, tx, `CREATE TEMP TRIGGER deny_open BEFORE UPDATE OF state ON main.cjsi_publication_attempts WHEN NEW.state='opened' BEGIN SELECT RAISE(IGNORE); END`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d.afterApply, d.afterStamp = nil, nil
			calls := d.calls
			tx := beginCJSIFixtureTx(t, db)
			tc.setup(tx)
			cjsiOpenExpectClosed(t, tx, o.open(t.Context(), tx))
			if d.calls != calls+1 {
				t.Fatal("fault did not reach the apply stage")
			}
			if !reflect.DeepEqual(before, cjsiSlotCoreImage(t, db)) {
				t.Fatal("partial publication survived rollback")
			}
		})
	}
	t.Run("panic-aborts-writer", func(t *testing.T) {
		d.afterApply = nil
		fault := errors.New("stamp panic")
		d.afterStamp = func(*sql.Tx) error { panic(fault) }
		tx := beginCJSIFixtureTx(t, db)
		func() {
			defer func() {
				if recover() != fault {
					t.Fatal("missing injected panic")
				}
			}()
			_ = o.open(t.Context(), tx)
		}()
		if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
			t.Fatalf("writer survived panic: %v", err)
		}
		if !reflect.DeepEqual(before, cjsiSlotCoreImage(t, db)) {
			t.Fatal("partial opening survived panic")
		}
	})

}

func TestCJSIPublicationOpenAdmission(t *testing.T) {
	db, o, d := newCJSIOpenFixture(t, true)
	for _, tc := range []struct{ name, stmt string }{
		{"draft-with-seal", "UPDATE cjsi_publication_attempts SET state='draft'"},
		{"retired", "UPDATE cjsi_publication_attempts SET state='retired'"},
		{"no-slot-seal", "UPDATE cjsi_publication_attempts SET slot_seal=NULL"},
		{"wrong-identity", "UPDATE cjsi_publication_attempts SET slot_identity=randomblob(16)"},
		{"wrong-size", "UPDATE cjsi_publication_attempts SET slot_size=slot_size+1"},
		{"wrong-session", "UPDATE cjsi_publication_attempts SET session_seal=randomblob(32)"},
		{"wrong-diff", "UPDATE cjsi_publication_attempts SET diff_digest=randomblob(32)"},
		{"wrong-user-revision", "UPDATE cjsi_publication_attempts SET user_revision=user_revision+1"},
		{"wrong-accept-window", "UPDATE cjsi_publication_attempts SET accept_until='2000-01-01T00:00:00Z'"},
		{"changed-fence", "UPDATE cjsi_layout_journal SET fence_identity=randomblob(16)"},
		{"changed-epoch", "UPDATE derived_snapshot_generation SET epoch=epoch+1"},
		{"changed-prepared-usage", "UPDATE serving_generation SET revision=revision+1"},
		{"changed-session-base", "UPDATE cjsi_publication_head SET base_revision=base_revision+1,session_revision=session_revision+1"},
		{"new-protection", "UPDATE cjsi_publication_head SET protection_revision=protection_revision+1"},
		{"new-coverage", "UPDATE cjsi_publication_head SET coverage_revision=coverage_revision+1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := beginCJSIFixtureTx(t, db)
			cjsiSlotExec(t, tx, tc.stmt)
			cjsiOpenExpectClosed(t, tx, o.open(t.Context(), tx))
			if d.calls != 0 {
				t.Fatal("unqualified attempt reached domain writes")
			}
		})
	}
	t.Run("orphan-file-without-registered-receipt", func(t *testing.T) {
		db, o, d := newCJSIOpenFixture(t, false)
		tx := beginCJSIFixtureTx(t, db)
		cjsiOpenExpectClosed(t, tx, o.open(t.Context(), tx))
		if d.calls != 0 {
			t.Fatal("adopted unsealed draft")
		}
	})
	t.Run("stale-writer-and-independent-scope", func(t *testing.T) {
		tx := beginCJSIFixtureTx(t, db)
		checkCJSIOpenView(t, tx, o, false)
		cjsiFixtureExec(t, db, "UPDATE cjsi_scope_overlays SET masked=1,coverage_revision=coverage_revision+1")
		before := cjsiSlotCoreImage(t, db)
		cjsiOpenExpectClosed(t, tx, o.open(t.Context(), tx))
		if d.calls != 0 {
			t.Fatal("stale snapshot entered domain writes")
		}
		// With a fresh writer the exact scoped precondition must still fail.
		fresh := beginCJSIFixtureTx(t, db)
		cjsiOpenExpectClosed(t, fresh, o.open(t.Context(), fresh))
		if !reflect.DeepEqual(before, cjsiSlotCoreImage(t, db)) {
			t.Fatal("overwrote newer scope/coverage or kept partial edits")
		}
	})
}

func TestCJSIPublicationOpenBinding(t *testing.T) {
	db, o, d := newCJSIOpenFixture(t, true)
	a := o.admission
	wrong := a.proposal.expected
	wrong.digest[0]++
	if _, err := bindCJSICombinedOpen(a, wrong, o.next, d); err == nil {
		t.Fatal("accepted a different pinned file")
	}
	a.proposal.diff[0]++
	if _, err := bindCJSICombinedOpen(a, a.proposal.expected, o.next, d); err == nil {
		t.Fatal("accepted a relabeled delta")
	}
	a = o.admission
	for _, change := range []func(*cjsiOpenTarget){
		func(n *cjsiOpenTarget) { n.session.SessionAttempt++ }, func(n *cjsiOpenTarget) { n.session.SessionSeal[0]++ },
		func(n *cjsiOpenTarget) { n.session.BaseSeal[0]++ }, func(n *cjsiOpenTarget) { n.session.Inode++ },
		func(n *cjsiOpenTarget) { n.usageRevision = a.proposal.usageRevision - 1 }, func(n *cjsiOpenTarget) { n.session.CoverageRevision = a.proposal.base.CoverageRevision - 1 },
		func(n *cjsiOpenTarget) { n.session.ProtectionRevision++ }, func(n *cjsiOpenTarget) { n.session.SessionRevision = a.proposal.base.SessionRevision - 1 },
		func(n *cjsiOpenTarget) {
			n.usageRevision = a.proposal.usageRevision
			n.session.SessionRevision = a.proposal.base.SessionRevision
			n.session.CoverageRevision = a.proposal.base.CoverageRevision
		},
	} {
		bad := *d
		change(&bad.next)
		if _, err := bindCJSICombinedOpen(a, a.proposal.expected, bad.next, &bad); err == nil {
			t.Fatalf("accepted invalid target %+v", bad.next)
		}
	}
	bad := *d
	bad.next.usageRevision++
	if _, err := bindCJSICombinedOpen(a, a.proposal.expected, o.next, &bad); err == nil {
		t.Fatal("accepted different independently qualified postconditions")
	}
	// Retained preparation can be reconstructed for commit reconciliation
	// after expiry, but even a matching receipt cannot initiate a new open.
	expired := *d
	expired.p.acceptUntil = time.Now().Add(-time.Minute)
	a.proposal = expired.p
	past, err := bindCJSICombinedOpen(a, a.proposal.expected, expired.next, &expired)
	if err != nil {
		t.Fatal(err)
	}
	tx := beginCJSIFixtureTx(t, db)
	cjsiOpenExpectClosed(t, tx, past.open(t.Context(), tx))
	if expired.calls != 0 {
		t.Fatal("applied expired plan")
	}
}
