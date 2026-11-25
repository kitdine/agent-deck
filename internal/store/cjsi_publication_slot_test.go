package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

type cjsiSlotFixture struct {
	admission cjsiSlotAdmission
	groupDiff [32]byte
}

func openCJSISlotCore(t *testing.T, root string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", cjsiSQLiteURI(filepath.Join(root, "agentdeck.sqlite3"), "rw")+
		"&_pragma=foreign_keys(1)&_pragma=synchronous(FULL)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newCJSISlotFixture(t *testing.T) (cjsiSlotFixture, *sql.DB) {
	t.Helper()
	ctx := t.Context()
	c := newCJSIMigrationCapsule(t)
	c.seed35(t)
	legacy, err := OpenSessions(ctx, c.root)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = closeCJSIInstallReadPool(filepath.Join(c.root, "agentdeck.sqlite3"))
		_ = closeCJSIInstallReadPool(filepath.Join(c.root, cjsiPrivateIndexName))
	})
	db := openCJSISlotCore(t, c.root)
	// A prior core-only publication carries the baseline session component.
	cjsiFixtureExec(t, db, "UPDATE cjsi_publication_head SET ticket=1")
	tx := beginCJSIFixtureTx(t, db)
	var head CJSIIndexStamp
	err = WithCJSIIndexSnapshot(ctx, c.root, tx, func(s CJSIIndexSnapshot) error { head = s.Selection(); return nil })
	if err != nil {
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
	now := time.Now().UTC()
	r := cjsiSlotReceipt{identity: [16]byte{7}, digest: sha256.Sum256([]byte("prepared")), size: 8, format: 1,
		builtAt: now.Add(-time.Minute), validUntil: now.Add(3 * time.Hour)}
	p := cjsiSlotProposal{ticket: 3, attempt: 2, base: head, userRevision: 11, routeRevision: 17, priceRevision: 23,
		input: sha256.Sum256([]byte("registered inputs")), diff: sha256.Sum256([]byte("complete aggregate with postconditions")),
		usageMaterial: sha256.Sum256([]byte("complete required usage material")), sessionSeal: sha256.Sum256([]byte("proposed sessions")),
		acceptUntil: now.Add(time.Hour), expected: r}
	if err := db.QueryRow("SELECT revision FROM main.serving_generation").Scan(&p.usageRevision); err != nil {
		t.Fatal(err)
	}
	// Stage the group while the aggregate diff is still NULL. Only the
	// upstream owner installs that digest and the proposed-session binding.
	cjsiFixtureExec(t, db, `INSERT INTO cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,index_epoch,
		session_revision,usage_revision,user_revision,route_revision,price_revision,protection_revision,coverage_revision,
		input_digest,slot_identity,accept_until,state) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'draft')`,
		p.ticket, p.attempt, head.Ticket, head.CoreEpoch, head.IndexEpoch, head.SessionRevision, p.usageRevision, p.userRevision,
		p.routeRevision, p.priceRevision, head.ProtectionRevision, head.CoverageRevision, p.input[:], r.identity[:], p.acceptUntil.Format(time.RFC3339Nano))
	group := sha256.Sum256([]byte("group-specific delta"))
	cjsiFixtureExec(t, db, `INSERT INTO cjsi_usage_groups VALUES(?,?,'fixture',?,?,1,0,0,0,0)`, p.ticket, p.attempt, p.input[:], group[:])
	cjsiFixtureExec(t, db, `UPDATE cjsi_publication_attempts SET diff_digest=?,session_seal=? WHERE ticket=? AND attempt=? AND diff_digest IS NULL`, p.diff[:], p.sessionSeal[:], p.ticket, p.attempt)
	return cjsiSlotFixture{cjsiSlotAdmission{c.root, cjsiIdentity(st), fence, p}, group}, db
}

// Synthetic stand-in for the owner, with an independent one-group manifest.
// In particular the aggregate diff is different from this group's digest.
func (f cjsiSlotFixture) selectCurrent(ctx context.Context, q cjsiRowQueryer) (cjsiSlotAdmission, error) {
	p := f.admission.proposal
	var ok bool
	err := q.QueryRowContext(ctx, `SELECT count(*)=1 AND count(*) FILTER (WHERE thread='fixture' AND input_digest=? AND diff_digest=?)=1
		FROM main.cjsi_usage_groups WHERE ticket=? AND attempt=?`, p.input[:], f.groupDiff[:], p.ticket, p.attempt).Scan(&ok)
	if err != nil {
		return cjsiSlotAdmission{}, err
	}
	if !ok {
		return cjsiSlotAdmission{}, errCJSISlotAdmission
	}
	return f.admission, nil
}

func cjsiSlotExec(t *testing.T, tx *sql.Tx, stmt string, args ...any) {
	t.Helper()
	if _, err := tx.ExecContext(t.Context(), stmt, args...); err != nil {
		t.Fatal(err)
	}
}

func cjsiSlotChanges(t *testing.T, tx *sql.Tx) int64 {
	t.Helper()
	var n int64
	if err := tx.QueryRowContext(t.Context(), "SELECT total_changes()").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Compare full row images of all ordinary core tables, including facts,
// selection, coverage, bindings, receipts, revisions and layout. Callers can
// exempt just the private attempt row for a successful committed admission.
func cjsiSlotCoreImage(t *testing.T, db *sql.DB) map[string][]string {
	t.Helper()
	tx := beginCJSIFixtureTx(t, db)
	defer tx.Rollback()
	tables, err := tx.Query("SELECT name FROM pragma_table_list WHERE schema='main' AND type='table' ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for tables.Next() {
		var name string
		if err := tables.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err := tables.Err(); err != nil {
		t.Fatal(err)
	}
	if err := tables.Close(); err != nil {
		t.Fatal(err)
	}
	image := map[string][]string{}
	for _, name := range names {
		rows, err := tx.Query(`SELECT * FROM main."` + strings.ReplaceAll(name, `"`, `""`) + `"`)
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range ptrs {
			ptrs[i] = &values[i]
		}
		var result []string
		for rows.Next() {
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			result = append(result, string(data))
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		slices.Sort(result)
		image[name] = result
	}
	return image
}

func TestCJSISlotSealAdmission(t *testing.T) {
	f, db := newCJSISlotFixture(t)
	p := f.admission.proposal
	before := cjsiSlotCoreImage(t, db)

	t.Run("refuse-before-any-write", func(t *testing.T) {
		for _, tc := range []struct{ name, stmt string }{
			{"schema", "UPDATE schema_metadata SET version=36"},
			{"epoch", "UPDATE derived_snapshot_generation SET epoch=epoch+1"},
			{"live-usage", "UPDATE serving_generation SET revision=revision+1"},
			{"head", "UPDATE cjsi_publication_head SET ticket=2"},
			{"prior-slot", "UPDATE cjsi_publication_head SET slot_ticket=1,slot_attempt=1,slot_seal=randomblob(32)"},
			{"protection", "UPDATE cjsi_publication_head SET protection_revision=protection_revision+1"},
			{"coverage", "UPDATE cjsi_publication_head SET coverage_revision=coverage_revision+1"},
			{"selected-base-seal", "UPDATE cjsi_publication_head SET base_seal=CAST(zeroblob(31)||x'01' AS BLOB), session_seal=CAST(zeroblob(31)||x'01' AS BLOB)"},
			{"attempt-index-epoch", "UPDATE cjsi_publication_attempts SET index_epoch=index_epoch+1"},
			{"attempt-usage", "UPDATE cjsi_publication_attempts SET usage_revision=usage_revision+1"},
			{"attempt-user", "UPDATE cjsi_publication_attempts SET user_revision=user_revision+1"},
			{"attempt-route", "UPDATE cjsi_publication_attempts SET route_revision=route_revision+1"},
			{"attempt-price", "UPDATE cjsi_publication_attempts SET price_revision=price_revision+1"},
			{"unbound-diff", "UPDATE cjsi_publication_attempts SET diff_digest=NULL"},
			{"wrong-diff", "UPDATE cjsi_publication_attempts SET diff_digest=randomblob(32)"},
			{"wrong-input", "UPDATE cjsi_publication_attempts SET input_digest=randomblob(32)"},
			{"unbound-session", "UPDATE cjsi_publication_attempts SET session_seal=NULL"},
			{"wrong-session", "UPDATE cjsi_publication_attempts SET session_seal=randomblob(32)"},
			{"unreserved", "UPDATE cjsi_publication_attempts SET slot_identity=NULL"},
			{"wrong-reservation", "UPDATE cjsi_publication_attempts SET slot_identity=randomblob(16)"},
			{"partial-private-receipt", "UPDATE cjsi_publication_attempts SET slot_digest=randomblob(32)"},
			{"malformed-deadline", "UPDATE cjsi_publication_attempts SET accept_until='tomorrow'"},
			{"replaced-deadline", "UPDATE cjsi_publication_attempts SET accept_until='2099-01-01T00:00:00Z'"},
			{"retired", "UPDATE cjsi_publication_attempts SET state='retired'"},
			{"opened", "UPDATE cjsi_publication_attempts SET state='opened'"},
			{"missing-required-group", "DELETE FROM cjsi_usage_groups"},
			{"different-required-group", "UPDATE cjsi_usage_groups SET diff_digest=randomblob(32)"},
			{"reused-by-retired-attempt", `INSERT INTO cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,index_epoch,session_revision,usage_revision,user_revision,route_revision,price_revision,protection_revision,coverage_revision,input_digest,accept_until,state,slot_identity)
				SELECT ticket,attempt+1,base_ticket,core_epoch,index_epoch,session_revision,usage_revision,user_revision,route_revision,price_revision,protection_revision,coverage_revision,input_digest,accept_until,'retired',slot_identity FROM cjsi_publication_attempts`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				tx := beginCJSIFixtureTx(t, db)
				cjsiSlotExec(t, tx, tc.stmt)
				n := cjsiSlotChanges(t, tx)
				got, err := registerCJSISlotReceipt(t.Context(), tx, p, p.expected, f.selectCurrent)
				if err == nil || got != ([32]byte{}) || cjsiSlotChanges(t, tx) != n {
					t.Fatalf("refusal changed rows or returned a seal: %x, %v", got, err)
				}
			})
		}
	})

	t.Run("independent-inputs", func(t *testing.T) {
		fail := func(f cjsiSlotFixture, built cjsiSlotProposal, r cjsiSlotReceipt) {
			t.Helper()
			tx := beginCJSIFixtureTx(t, db)
			defer tx.Rollback()
			n := cjsiSlotChanges(t, tx)
			got, err := registerCJSISlotReceipt(t.Context(), tx, built, r, f.selectCurrent)
			if err == nil || got != ([32]byte{}) || n != cjsiSlotChanges(t, tx) {
				t.Fatalf("unexpected acceptance: %x %v", got, err)
			}
		}
		for _, change := range []func(*cjsiSlotReceipt){
			func(r *cjsiSlotReceipt) { r.identity[2]++ }, func(r *cjsiSlotReceipt) { r.digest[2]++ },
			func(r *cjsiSlotReceipt) { r.size++ }, func(r *cjsiSlotReceipt) { r.format++ },
			func(r *cjsiSlotReceipt) { r.builtAt = r.builtAt.Add(time.Nanosecond) }, func(r *cjsiSlotReceipt) { r.validUntil = r.validUntil.Add(time.Nanosecond) },
		} {
			r := p.expected
			change(&r)
			fail(f, p, r)
		}
		for _, change := range []func(*cjsiSlotProposal){
			func(v *cjsiSlotProposal) { v.ticket++ }, func(v *cjsiSlotProposal) { v.attempt++ },
			func(v *cjsiSlotProposal) { v.usageMaterial[0]++ }, func(v *cjsiSlotProposal) { v.base.Inode++ },
		} {
			built := p
			change(&built)
			fail(f, built, p.expected)
		}
		for _, change := range []func(*cjsiSlotProposal){
			func(v *cjsiSlotProposal) { v.usageMaterial = [32]byte{} }, func(v *cjsiSlotProposal) { v.expected.digest = [32]byte{} },
			func(v *cjsiSlotProposal) { v.expected.format = 0 }, func(v *cjsiSlotProposal) { v.expected.size = 0 },
			func(v *cjsiSlotProposal) { v.expected.size = 8<<20 + 1 }, func(v *cjsiSlotProposal) { v.expected.identity = [16]byte{} },
			func(v *cjsiSlotProposal) { v.expected.validUntil = v.expected.builtAt }, func(v *cjsiSlotProposal) { v.expected.builtAt = v.acceptUntil },
		} {
			owner := f
			change(&owner.admission.proposal)
			built := owner.admission.proposal
			fail(owner, built, built.expected)
		}
		for _, change := range []func(*cjsiSlotAdmission){
			func(a *cjsiSlotAdmission) { a.root.inode++ }, func(a *cjsiSlotAdmission) { a.fence.inode++ },
			func(a *cjsiSlotAdmission) { a.rootPath = filepath.Join(a.rootPath, "other") },
		} {
			owner := f
			change(&owner.admission)
			fail(owner, p, p.expected)
		}
	})

	tx := beginCJSIFixtureTx(t, db)
	n := cjsiSlotChanges(t, tx)
	seal, err := registerCJSISlotReceipt(t.Context(), tx, p, p.expected, f.selectCurrent)
	if err != nil || seal == ([32]byte{}) || cjsiSlotChanges(t, tx) != n+1 {
		t.Fatalf("first admission: %x %v", seal, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, cjsiSlotCoreImage(t, db)) {
		t.Fatal("rollback did not restore the full core image")
	}

	tx = beginCJSIFixtureTx(t, db)
	got, err := registerCJSISlotReceipt(t.Context(), tx, p, p.expected, f.selectCurrent)
	if err != nil || got != seal {
		t.Fatalf("admission after rollback: %x %v", got, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = openCJSISlotCore(t, f.admission.rootPath)
	after := cjsiSlotCoreImage(t, db)
	const attempts = "cjsi_publication_attempts"
	if reflect.DeepEqual(before[attempts], after[attempts]) {
		t.Fatal("missing durable private receipt")
	}
	delete(before, attempts)
	delete(after, attempts)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("receipt changed other core metadata or public facts")
	}

	t.Run("restart-exact-retry", func(t *testing.T) {
		tx := beginCJSIFixtureTx(t, db)
		n := cjsiSlotChanges(t, tx)
		var ok bool
		err := tx.QueryRow(`SELECT state='sealed' AND slot_seal=? AND slot_digest=? AND slot_size=? AND slot_format=? AND slot_identity=?
			AND built_at=? AND valid_until=? AND input_digest=? AND diff_digest=? AND session_seal=? FROM cjsi_publication_attempts`, seal[:], p.expected.digest[:], p.expected.size, p.expected.format, p.expected.identity[:], p.expected.builtAt.Format(time.RFC3339Nano), p.expected.validUntil.Format(time.RFC3339Nano), p.input[:], p.diff[:], p.sessionSeal[:]).Scan(&ok)
		if err != nil || !ok {
			t.Fatalf("incorrect durably stored fields: %v", err)
		}
		r := p.expected
		r.builtAt = r.builtAt.In(time.FixedZone("fixture", 7200))
		got, err := registerCJSISlotReceipt(t.Context(), tx, p, r, f.selectCurrent)
		if err != nil || got != seal || cjsiSlotChanges(t, tx) != n {
			t.Fatalf("sealed retry rewrote or lost receipt: %x %v", got, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("conflicting-receipt-after-restart", func(t *testing.T) {
		tx := beginCJSIFixtureTx(t, db)
		owner := f
		owner.admission.proposal.expected.digest = sha256.Sum256([]byte("replacement"))
		different := owner.admission.proposal
		n := cjsiSlotChanges(t, tx)
		got, err := registerCJSISlotReceipt(t.Context(), tx, different, different.expected, owner.selectCurrent)
		if !errors.Is(err, errCJSISlotAdmission) || got != ([32]byte{}) || cjsiSlotChanges(t, tx) != n {
			t.Fatalf("replaced a sealed receipt: %x %v", got, err)
		}
	})
	t.Run("sealed-conflict", func(t *testing.T) {
		for _, stmt := range []string{"slot_seal=randomblob(32)", "slot_digest=randomblob(32)", "slot_format=slot_format+1", "slot_size=slot_size+1", "built_at=NULL", "valid_until='bad'"} {
			tx := beginCJSIFixtureTx(t, db)
			cjsiSlotExec(t, tx, "UPDATE cjsi_publication_attempts SET "+stmt)
			n := cjsiSlotChanges(t, tx)
			got, err := registerCJSISlotReceipt(t.Context(), tx, p, p.expected, f.selectCurrent)
			if !errors.Is(err, errCJSISlotAdmission) || got != ([32]byte{}) || cjsiSlotChanges(t, tx) != n {
				t.Fatalf("rewrote %s: %x %v", stmt, got, err)
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("stale-retry-snapshot", func(t *testing.T) {
		tx := beginCJSIFixtureTx(t, db)
		var state string
		if err := tx.QueryRow("SELECT state FROM cjsi_publication_attempts").Scan(&state); err != nil || state != "sealed" {
			t.Fatalf("pin %q %v", state, err)
		}
		cjsiFixtureExec(t, db, "UPDATE cjsi_publication_attempts SET state='retired'")
		got, err := registerCJSISlotReceipt(t.Context(), tx, p, p.expected, f.selectCurrent)
		if err == nil || got != ([32]byte{}) {
			t.Fatalf("accepted stale sealed snapshot: %x %v", got, err)
		}
	})
	t.Run("expired-draft-after-restart", func(t *testing.T) {
		past := time.Now().UTC().Add(-time.Hour)
		cjsiFixtureExec(t, db, `UPDATE cjsi_publication_attempts SET state='draft',slot_seal=NULL,slot_digest=NULL,slot_size=NULL,slot_format=NULL,built_at=NULL,valid_until=NULL,accept_until=?`, past.Format(time.RFC3339Nano))
		reopened := openCJSISlotCore(t, f.admission.rootPath)
		tx := beginCJSIFixtureTx(t, reopened)
		owner := f
		owner.admission.proposal.acceptUntil = past
		owner.admission.proposal.expected.builtAt = past.Add(-time.Hour)
		built := owner.admission.proposal
		got, err := registerCJSISlotReceipt(t.Context(), tx, built, built.expected, owner.selectCurrent)
		if !errors.Is(err, errCJSISlotAdmission) || got != ([32]byte{}) {
			t.Fatalf("admitted late completion: %x %v", got, err)
		}
	})
}

func TestCJSISlotSealEncoding(t *testing.T) {
	sum := func(b byte) (d [32]byte) {
		for i := range d {
			d[i] = b
		}
		return d
	}
	instant := func(n int64) time.Time { return time.Unix(1700000000+n, n).UTC() }
	p := cjsiSlotProposal{
		ticket: 101, attempt: 7,
		base: CJSIIndexStamp{Ticket: 100, CoreEpoch: 10, IndexEpoch: 20, BaseTicket: 80, BaseAttempt: 8, BaseRevision: 301,
			SessionTicket: 90, SessionAttempt: 9, SessionRevision: 305, ProtectionRevision: 32, CoverageRevision: 33,
			Device: 15, Inode: 16, BaseSeal: sum(1), SessionSeal: sum(2)},
		baseSlotTicket: 97, baseSlotAttempt: 4, baseSlotSeal: sum(3),
		usageRevision: 27, userRevision: 28, routeRevision: 29, priceRevision: 30,
		input: sum(4), diff: sum(5), usageMaterial: sum(6), sessionSeal: sum(7), acceptUntil: instant(12),
		expected: cjsiSlotReceipt{digest: sum(8), size: 31, format: 2, builtAt: instant(10), validUntil: instant(11)},
	}
	for i := range p.expected.identity {
		p.expected.identity[i] = 9
	}
	digest := p.seal()
	if hex.EncodeToString(digest[:]) != "23049c0ab71593af34ddb2e3842772bde9ba2140303d7eb7d093ae65ce9aeb0d" {
		t.Fatalf("combined-slot seal encoding changed: %x", digest)
	}
}
