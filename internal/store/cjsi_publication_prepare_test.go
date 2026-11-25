package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

type cjsiPreparationFixture struct {
	db            *sql.DB
	draft, frozen cjsiPrepareProof
}

func (f cjsiPrepareProof) selectCurrent(context.Context, cjsiRowQueryer) (cjsiPrepareProof, error) {
	return f, nil
}
func (f cjsiPreparationFixture) reserve(t *testing.T, tx *sql.Tx) error {
	return reserveCJSICombinedDraft(t.Context(), tx, f.draft.admission, f.draft.selectCurrent)
}
func (f cjsiPreparationFixture) freeze(t *testing.T, tx *sql.Tx) error {
	return freezeCJSICombinedDraft(t.Context(), tx, f.frozen.admission, f.frozen.selectCurrent)
}

func newCJSIPreparationFixture(t *testing.T) cjsiPreparationFixture {
	t.Helper()
	c := newCJSIMigrationCapsule(t)
	c.seed35(t)
	old, err := OpenSessions(t.Context(), c.root)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, name := range []string{"agentdeck.sqlite3", cjsiPrivateIndexName} {
			_ = closeCJSIInstallReadPool(filepath.Join(c.root, name))
		}
	})
	db := openCJSISlotCore(t, c.root)
	tx := beginCJSIFixtureTx(t, db)
	p := cjsiSlotProposal{ticket: 11, attempt: 3, userRevision: 7, routeRevision: 8, priceRevision: 9,
		input: sha256.Sum256([]byte("fixture planned combined scope / original view / two groups")), acceptUntil: time.Now().UTC().Add(time.Hour),
		expected: cjsiSlotReceipt{identity: [16]byte{19}}}
	err = WithCJSIIndexSnapshot(t.Context(), c.root, tx, func(v CJSIIndexSnapshot) error { p.base = v.Selection(); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow("SELECT revision FROM main.serving_generation").Scan(&p.usageRevision); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	st, err := privateServingPath(c.root, true)
	if err != nil {
		t.Fatal(err)
	}
	fence, err := validateCJSIFence(c.root)
	if err != nil {
		t.Fatal(err)
	}
	f := cjsiPreparationFixture{db: db, draft: cjsiPrepareProof{admission: cjsiSlotAdmission{c.root, cjsiIdentity(st), fence, p}}}
	f.frozen = f.draft
	f.frozen.groups = map[string]cjsiStagedGroup{}
	basis := cjsiledger.Basis{Ticket: p.ticket, Attempt: p.attempt, CoreEpoch: p.base.CoreEpoch, BaseTicket: p.base.Ticket, InputDigest: p.input}
	var material []byte
	for _, thread := range []string{"fixture-a", "fixture-b"} {
		proposal, err := cjsiledger.Replay(t.Context(), cjsiledger.ReplayInput{Basis: basis, Thread: thread})
		if err != nil {
			t.Fatal(err)
		}
		v, err := proposal.Eligibility(proposal.Stamp(), thread)
		if err != nil {
			t.Fatal(err)
		}
		f.frozen.groups[thread] = cjsiStagedGroup{v.Stamp, v.State, v.Gaps, v.Pending, v.Partial, v.UnresolvedOverlap}
		material = append(material, v.Stamp.Diff[:]...)
	}
	p.usageMaterial = sha256.Sum256(material)
	p.sessionSeal = sha256.Sum256([]byte("fixture qualified session"))
	p.diff = sha256.Sum256(append(append([]byte("fixture aggregate: group operations and postconditions:"), p.usageMaterial[:]...), p.sessionSeal[:]...))
	f.frozen.admission.proposal = p
	// Synthetic session-owner qualification. This is actual sealed private
	// index metadata; core's preparation step never opens that database.
	index, err := sql.Open("sqlite", cjsiSQLiteURI(filepath.Join(c.root, cjsiPrivateIndexName), "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	b := p.base
	cjsiFixtureExec(t, index, `INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,parent_ticket,protection_revision,coverage_revision,session_revision)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, p.ticket, p.attempt, b.CoreEpoch, b.IndexEpoch, b.BaseTicket, b.BaseAttempt, b.BaseSeal[:], b.BaseRevision, b.Ticket, b.ProtectionRevision, b.CoverageRevision, b.SessionRevision+1)
	cjsiFixtureExec(t, index, `UPDATE cjsi_session_candidates SET seal_digest=? WHERE ticket=? AND attempt=?`, p.sessionSeal[:], p.ticket, p.attempt)
	session := cjsiPrepareSession{parent: b}
	var sum []byte
	if err := index.QueryRow(`SELECT ticket,attempt,seal_digest FROM cjsi_session_candidates WHERE ticket=? AND attempt=? AND parent_ticket=? AND base_seal=? AND index_epoch=?`,
		p.ticket, p.attempt, b.Ticket, b.BaseSeal[:], b.IndexEpoch).Scan(&session.ticket, &session.attempt, &sum); err != nil {
		t.Fatal(err)
	}
	session.seal = [32]byte(sum)
	f.frozen.session = session
	return f
}

// Exercise the usage staging row contract on the same actual database. Only
// the usage owner supplies staged source/child material; these two synthetic
// ledger proposals have no sources, members or old rows.
func stageCJSIPrepareFixture(ctx context.Context, db *sql.DB, thread string, g cjsiStagedGroup) error {
	b := g.stamp.Basis
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var allowed int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_publication_attempts a
		JOIN main.cjsi_publication_head h ON h.singleton=1 AND h.ticket=a.base_ticket AND h.core_epoch=a.core_epoch AND h.index_epoch=a.index_epoch
		  AND h.session_revision=a.session_revision AND h.protection_revision=a.protection_revision AND h.coverage_revision=a.coverage_revision
		JOIN main.derived_snapshot_generation d ON d.singleton=1 AND d.epoch=h.core_epoch
		JOIN main.cjsi_layout_journal j ON j.singleton=1 AND j.phase=5 AND j.fence_identity IS NOT NULL
		WHERE a.ticket=? AND a.attempt=? AND a.base_ticket=? AND a.core_epoch=? AND a.input_digest=? AND a.state='draft' AND a.diff_digest IS NULL
		  AND NOT EXISTS(SELECT 1 FROM main.cjsi_usage_selection WHERE thread=? AND ticket=a.ticket AND attempt=a.attempt)`,
		b.Ticket, b.Attempt, b.BaseTicket, b.CoreEpoch, b.InputDigest[:], thread).Scan(&allowed)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO main.cjsi_usage_groups VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(ticket,attempt,thread) DO NOTHING`, b.Ticket, b.Attempt, thread, b.InputDigest[:], g.stamp.Diff[:], g.state, g.gaps, g.pending, g.partial, g.overlap)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (f cjsiPreparationFixture) stage(t *testing.T) {
	t.Helper()
	for thread, g := range f.frozen.groups {
		if err := stageCJSIPrepareFixture(t.Context(), f.db, thread, g); err != nil {
			t.Fatal(err)
		}
	}
}
func commitCJSIPrepare(t *testing.T, db *sql.DB, step func(*testing.T, *sql.Tx) error, wantChanges int64) {
	t.Helper()
	tx := beginCJSIFixtureTx(t, db)
	before := cjsiSlotChanges(t, tx)
	if err := step(t, tx); err != nil {
		t.Fatal(err)
	}
	if n := cjsiSlotChanges(t, tx) - before; n != wantChanges {
		t.Fatalf("changed %d rows, want %d", n, wantChanges)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
func refuseCJSIPrepare(t *testing.T, tx *sql.Tx, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("accepted invalid preparation")
	}
	if err := tx.Commit(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("refusal left transaction committable: %v", err)
	}
}

func TestCJSIPublicationPrepare(t *testing.T) {
	f := newCJSIPreparationFixture(t)
	before := cjsiSlotCoreImage(t, f.db)
	tx := beginCJSIFixtureTx(t, f.db)
	if err := f.reserve(t, tx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, cjsiSlotCoreImage(t, f.db)) {
		t.Fatal("reservation survived rollback")
	}
	commitCJSIPrepare(t, f.db, f.reserve, 1)
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	f.db = openCJSISlotCore(t, f.draft.admission.rootPath)
	commitCJSIPrepare(t, f.db, f.reserve, 0)
	var ok bool
	err := f.db.QueryRow(`SELECT state='draft' AND diff_digest IS NULL AND session_seal IS NULL AND slot_seal IS NULL AND
		slot_digest IS NULL AND slot_size IS NULL AND slot_format IS NULL AND built_at IS NULL AND valid_until IS NULL AND slot_identity=?
		FROM cjsi_publication_attempts WHERE ticket=? AND attempt=?`, f.draft.admission.proposal.expected.identity[:], f.draft.admission.proposal.ticket, f.draft.admission.proposal.attempt).Scan(&ok)
	if err != nil || !ok {
		t.Fatalf("unexpected draft metadata: %v", err)
	}
	tx = beginCJSIFixtureTx(t, f.db)
	refuseCJSIPrepare(t, tx, f.freeze(t, tx))
	f.stage(t)
	f.stage(t) // The same staged groups are still eligible for exact retry.
	staged := cjsiSlotCoreImage(t, f.db)
	tx = beginCJSIFixtureTx(t, f.db)
	if err := f.freeze(t, tx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(staged, cjsiSlotCoreImage(t, f.db)) {
		t.Fatal("interrupted freeze lost the staged draft")
	}
	commitCJSIPrepare(t, f.db, f.freeze, 1)
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	f.db = openCJSISlotCore(t, f.draft.admission.rootPath)
	commitCJSIPrepare(t, f.db, f.freeze, 0)
	tx = beginCJSIFixtureTx(t, f.db)
	refuseCJSIPrepare(t, tx, f.reserve(t, tx))
	for thread, g := range f.frozen.groups {
		if err := stageCJSIPrepareFixture(t.Context(), f.db, thread, g); err == nil {
			t.Fatal("staging remained eligible after freeze")
		}
	}
	// Only now can an authenticated build supply the missing receipt fields.
	a := f.frozen.admission
	now := time.Now().UTC()
	r := cjsiSlotReceipt{identity: a.proposal.expected.identity, digest: sha256.Sum256([]byte("prepared")), size: 8, format: 1, builtAt: now, validUntil: now.Add(time.Hour)}
	a.proposal.expected = r
	tx = beginCJSIFixtureTx(t, f.db)
	seal, err := registerCJSISlotReceipt(t.Context(), tx, a.proposal, r, func(context.Context, cjsiRowQueryer) (cjsiSlotAdmission, error) { return a, nil })
	if err != nil || seal != a.proposal.seal() {
		t.Fatalf("receipt did not follow frozen material: %x %v", seal, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var durable bool
	err = f.db.QueryRow(`SELECT state='sealed' AND slot_seal=? AND diff_digest=? AND session_seal=? FROM main.cjsi_publication_attempts`,
		seal[:], a.proposal.diff[:], a.proposal.sessionSeal[:]).Scan(&durable)
	if err != nil || !durable {
		t.Fatalf("private handoff failed: %v", err)
	}
	after := cjsiSlotCoreImage(t, f.db)
	for _, private := range []string{"cjsi_publication_attempts", "cjsi_usage_groups"} {
		delete(before, private)
		delete(after, private)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("preparation/staging/receipt changed unrelated core state")
	}
}

func TestCJSIPrepareIdentityAndOwner(t *testing.T) {
	f := newCJSIPreparationFixture(t)
	commitCJSIPrepare(t, f.db, f.reserve, 1)
	t.Run("indexed-across-retired", func(t *testing.T) {
		tx := beginCJSIFixtureTx(t, f.db)
		p := f.draft.admission.proposal
		rows, err := tx.Query("EXPLAIN QUERY PLAN "+cjsiPrepareIdentityConflict, p.expected.identity[:], p.ticket, p.attempt)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for rows.Next() {
			var id, par, unused int
			var detail string
			if err := rows.Scan(&id, &par, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			found = found || strings.Contains(detail, "SEARCH") && strings.Contains(detail, "cjsi_publication_attempts_slot_identity (slot_identity=?)")
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		if !found {
			t.Fatal("reservation lacks bounded identity lookup")
		}
		cjsiSlotExec(t, tx, "UPDATE cjsi_publication_attempts SET state='retired'")
		other := f.draft
		other.admission.proposal.ticket++
		other.admission.proposal.attempt++
		refuseCJSIPrepare(t, tx, reserveCJSICombinedDraft(t.Context(), tx, other.admission, other.selectCurrent))
	})
	for _, change := range []func(*cjsiSlotProposal){
		func(p *cjsiSlotProposal) { p.expected.identity[0]++ }, func(p *cjsiSlotProposal) { p.input[0]++ },
		func(p *cjsiSlotProposal) { p.userRevision++ }, func(p *cjsiSlotProposal) { p.routeRevision++ }, func(p *cjsiSlotProposal) { p.priceRevision++ },
		func(p *cjsiSlotProposal) { p.acceptUntil = p.acceptUntil.Add(time.Second) }, func(p *cjsiSlotProposal) { p.expected.identity = [16]byte{} },
		func(p *cjsiSlotProposal) { p.expected.size = 1 }, func(p *cjsiSlotProposal) { p.diff = [32]byte{1} },
		func(p *cjsiSlotProposal) { p.ticket = p.base.Ticket }, func(p *cjsiSlotProposal) { p.acceptUntil = time.Now().Add(-time.Hour) },
	} {
		other := f.draft
		change(&other.admission.proposal)
		tx := beginCJSIFixtureTx(t, f.db)
		refuseCJSIPrepare(t, tx, reserveCJSICombinedDraft(t.Context(), tx, other.admission, other.selectCurrent))
	}
	t.Run("owner-moved-to-fresh-key", func(t *testing.T) {
		current := f.draft
		current.admission.proposal.ticket++
		current.admission.proposal.expected.identity[0]++
		tx := beginCJSIFixtureTx(t, f.db)
		refuseCJSIPrepare(t, tx, reserveCJSICombinedDraft(t.Context(), tx, f.draft.admission, current.selectCurrent))
	})
	t.Run("stale-reader-cannot-promote", func(t *testing.T) {
		tx := beginCJSIFixtureTx(t, f.db)
		var state string
		if err := tx.QueryRow("SELECT state FROM cjsi_publication_attempts").Scan(&state); err != nil || state != "draft" {
			t.Fatal(err)
		}
		cjsiFixtureExec(t, f.db, "UPDATE cjsi_publication_attempts SET state='retired'")
		refuseCJSIPrepare(t, tx, f.reserve(t, tx))
		assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE state='retired'", 1)
	})
}

func TestCJSIPrepareFreezeGuards(t *testing.T) {
	f := newCJSIPreparationFixture(t)
	commitCJSIPrepare(t, f.db, f.reserve, 1)
	f.stage(t)
	for _, tc := range []struct{ name, sql string }{
		{"missing", "DELETE FROM cjsi_usage_groups WHERE thread='fixture-a'"},
		{"extra", `INSERT INTO cjsi_usage_groups SELECT ticket,attempt,'extra',input_digest,diff_digest,state,gaps,pending,partial,unresolved_overlap FROM cjsi_usage_groups WHERE thread='fixture-a'`},
		{"same-count-wrong-thread", "UPDATE cjsi_usage_groups SET thread='other' WHERE thread='fixture-a'"},
		{"group-input", "UPDATE cjsi_usage_groups SET input_digest=randomblob(32)"},
		{"group-diff", "UPDATE cjsi_usage_groups SET diff_digest=randomblob(32)"},
		{"group-flags", "UPDATE cjsi_usage_groups SET partial=1-partial"},
		{"group-selected", "INSERT INTO cjsi_usage_selection SELECT thread,ticket,attempt FROM cjsi_usage_groups"},
		{"wrong-root", "UPDATE cjsi_layout_journal SET root_identity=randomblob(16)"},
		{"wrong-fence", "UPDATE cjsi_layout_journal SET fence_identity=randomblob(16)"},
		{"head", "UPDATE cjsi_publication_head SET ticket=ticket+1"},
		{"core-epoch", "UPDATE derived_snapshot_generation SET epoch=epoch+1"},
		{"index-epoch", "UPDATE cjsi_publication_head SET index_epoch=index_epoch+1"},
		{"session-revision", "UPDATE cjsi_publication_head SET base_revision=base_revision+1,session_revision=session_revision+1"},
		{"usage-revision", "UPDATE serving_generation SET revision=revision+1"},
		{"protection", "UPDATE cjsi_publication_head SET protection_revision=protection_revision+1"},
		{"coverage", "UPDATE cjsi_publication_head SET coverage_revision=coverage_revision+1"},
		{"other-owner", "UPDATE cjsi_publication_attempts SET state='retired'"},
		{"receipt-present", "UPDATE cjsi_publication_attempts SET built_at='premature'"},
		{"different-slot", "UPDATE cjsi_publication_attempts SET slot_identity=randomblob(16)"},
		{"partial-freeze", "UPDATE cjsi_publication_attempts SET session_seal=randomblob(32)"},
		{"changed-freeze", "UPDATE cjsi_publication_attempts SET session_seal=randomblob(32),diff_digest=randomblob(32)"},
		{"malformed-expiry", "UPDATE cjsi_publication_attempts SET accept_until='late'"},
		{"suppressed-final-write", `CREATE TEMP TRIGGER suppress BEFORE UPDATE OF diff_digest ON main.cjsi_publication_attempts BEGIN SELECT RAISE(IGNORE); END`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := beginCJSIFixtureTx(t, f.db)
			cjsiSlotExec(t, tx, tc.sql)
			refuseCJSIPrepare(t, tx, f.freeze(t, tx))
		})
	}
	for _, change := range []func(*cjsiPrepareProof){
		func(x *cjsiPrepareProof) { x.groups = nil }, func(x *cjsiPrepareProof) { x.session.ticket++ }, func(x *cjsiPrepareProof) { x.session.parent.SessionRevision++ },
		func(x *cjsiPrepareProof) { x.session.seal[0]++ }, func(x *cjsiPrepareProof) { x.admission.proposal.usageMaterial = [32]byte{} },
		func(x *cjsiPrepareProof) { x.admission.proposal.diff = [32]byte{} }, func(x *cjsiPrepareProof) { x.admission.proposal.sessionSeal = [32]byte{} },
	} {
		x := f.frozen
		change(&x)
		tx := beginCJSIFixtureTx(t, f.db)
		refuseCJSIPrepare(t, tx, freezeCJSICombinedDraft(t.Context(), tx, x.admission, x.selectCurrent))
	}
	t.Run("independent-owner-material", func(t *testing.T) {
		x := f.frozen
		x.admission.proposal.usageMaterial[0]++
		tx := beginCJSIFixtureTx(t, f.db)
		refuseCJSIPrepare(t, tx, freezeCJSICombinedDraft(t.Context(), tx, x.admission, f.frozen.selectCurrent))
	})
	t.Run("frozen-retry-rechecks-staging", func(t *testing.T) {
		commitCJSIPrepare(t, f.db, f.freeze, 1)
		tx := beginCJSIFixtureTx(t, f.db)
		cjsiSlotExec(t, tx, "DELETE FROM cjsi_usage_groups")
		refuseCJSIPrepare(t, tx, f.freeze(t, tx))
	})
}

func TestCJSIPrepareConcurrentReservation(t *testing.T) {
	f := newCJSIPreparationFixture(t)
	first := beginCJSIFixtureTx(t, f.db)
	if err := f.reserve(t, first); err != nil {
		t.Fatal(err)
	}
	other, err := sql.Open("sqlite", cjsiSQLiteURI(filepath.Join(f.draft.admission.rootPath, "agentdeck.sqlite3"), "rw")+
		"&_pragma=foreign_keys(1)&_pragma=synchronous(FULL)&_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	assertCJSIMigrationNumber(t, other, "SELECT count(*) FROM cjsi_publication_attempts", 0)
	candidate := f.draft
	candidate.admission.proposal.attempt++
	second := beginCJSIFixtureTx(t, other)
	consulted := false
	refuseCJSIPrepare(t, second, reserveCJSICombinedDraft(t.Context(), second, candidate.admission, func(context.Context, cjsiRowQueryer) (cjsiPrepareProof, error) {
		consulted = true
		return candidate, nil
	}))
	if consulted {
		t.Fatal("competing connection did not acquire writer admission but selected an owner")
	}
	if err := first.Commit(); err != nil {
		t.Fatal(err)
	}
	second = beginCJSIFixtureTx(t, other)
	refuseCJSIPrepare(t, second, reserveCJSICombinedDraft(t.Context(), second, candidate.admission, candidate.selectCurrent))
	assertCJSIMigrationNumber(t, other, "SELECT count(*) FROM cjsi_publication_attempts", 1)
	commitCJSIPrepare(t, other, f.reserve, 0)
}
