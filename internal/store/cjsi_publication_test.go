package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/usage/cjsiledger"
	"golang.org/x/sys/unix"
)

func TestCJSIPublicationSlotIdentityMigration(t *testing.T) {
	for _, version := range []string{"new", "36"} {
		t.Run(version, func(t *testing.T) {
			c := newCJSIMigrationCapsule(t)
			if version == "36" {
				c.seed36(t)
			}
			if err := migrateCJSIManagedRoot(t.Context(), c.root, 0, c); err != nil {
				t.Fatal(err)
			}
			db := openCJSISlotCore(t, c.root)
			assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM pragma_index_list('cjsi_publication_attempts')
   WHERE name='cjsi_publication_attempts_slot_identity' AND partial=1`, 1)
			var column, ddl string
			if err := db.QueryRow("SELECT name FROM pragma_index_info('cjsi_publication_attempts_slot_identity')").Scan(&column); err != nil || column != "slot_identity" {
				t.Fatal(column, err)
			}
			if err := db.QueryRow("SELECT sql FROM sqlite_schema WHERE name='cjsi_publication_attempts_slot_identity'").Scan(&ddl); err != nil || ddl != "CREATE INDEX cjsi_publication_attempts_slot_identity ON cjsi_publication_attempts(slot_identity) WHERE slot_identity IS NOT NULL" {
				t.Fatal(ddl, err)
			}
		})
	}
}

// The fixture stages the physical shape Task 5 emits for a move whose old
// pathname is immediately reused: there is no separate deletion at that key.
type publicationIndexFixture struct{ seal [32]byte }

func (p publicationIndexFixture) Seal() [32]byte { return p.seal }
func (p publicationIndexFixture) Stage(ctx context.Context, tx *sql.Tx, h CJSIIndexStamp, ticket, attempt, rev int64, seal [32]byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,parent_ticket,protection_revision,coverage_revision,session_revision)
 VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, ticket, attempt, h.CoreEpoch, h.IndexEpoch, h.SessionTicket, h.SessionAttempt, h.SessionSeal[:], h.SessionRevision, h.Ticket, h.ProtectionRevision, h.CoverageRevision, rev)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cjsi_candidate_sources(ticket,attempt,source_path,op,source_kind,identity,priority,parser_version,complete_end,size,modified_at,changed_at,prefix_hash,scanned_at)
 VALUES(?,?,'reused','upsert','codex-jsonl','new-id',0,1,0,0,0,0,'','')`, ticket, attempt)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE cjsi_session_candidates SET seal_digest=? WHERE ticket=? AND attempt=?", seal[:], ticket, attempt)
	return err
}

func TestCJSIReaderConsistency(t *testing.T) {
	ctx := t.Context()
	f := newCJSIPreparationFixture(t)
	root := f.draft.admission.rootPath
	m, err := ResolveCJSIRuntimeRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	// Populate the old source before pinning the physical revision.
	private, err := m.openPublicationIndex(ctx, f.draft.admission.proposal.base, true)
	if err != nil {
		t.Fatal(err)
	}
	defer private.Close()
	_, err = private.Exec(`INSERT INTO session_sources(source_path,modified_at,identity,size,cursor,prefix_hash,priority,parser_version,scanned_at)
 VALUES('reused',0,'old-id',0,0,'',0,1,'')`)
	if err != nil {
		t.Fatal(err)
	}
	// This fixture directly seeds an already-compacted base. Carry its real
	// trigger revision into the two logical baseline markers.
	var rev int64
	if err = private.QueryRow("SELECT revision FROM serving_generation").Scan(&rev); err != nil {
		t.Fatal(err)
	}
	if _, err = private.Exec("UPDATE cjsi_index_state SET base_revision=?", rev); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec("UPDATE cjsi_publication_head SET session_revision=?,base_revision=?", rev, rev); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	validate := func(context.Context, *sql.Tx) error { return nil }
	in := CJSIPublicationInputs{Digest: sha256.Sum256([]byte("registered capture")), Validate: validate}
	d, err := m.BeginPublication(ctx, in, now.Add(time.Minute), nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := publicationIndexFixture{sha256.Sum256([]byte("move and replace authenticated fixture"))}
	removed := []CJSISourceWithdrawal{{"reused", "old-id"}}
	if err = d.StageIndex(ctx, plan, rev+1, removed); err != nil {
		t.Fatal(err)
	}
	d, err = m.BeginPublication(ctx, in, now.Add(time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = d.StageIndex(ctx, plan, rev+1, []CJSISourceWithdrawal{{"reused", "different-old"}}); !errors.Is(err, ErrCJSIIndexChanged) {
		t.Fatalf("wrong old identity accepted: %v", err)
	}
	if err = d.StageIndex(ctx, plan, rev+1, removed); err != nil {
		t.Fatal(err)
	}
	old := beginCJSIFixtureTx(t, f.db)
	defer old.Rollback()
	var oldTicket int64
	if err = old.QueryRow("SELECT ticket FROM cjsi_publication_head").Scan(&oldTicket); err != nil {
		t.Fatal(err)
	}
	data := []byte("detached computed slot")
	var n int
	changes := CJSICombinedChanges{
		Digest: sha256.Sum256([]byte("set receipt and open revision")), Groups: []cjsiledger.Eligibility{}, Validate: validate,
		RecheckSources:  func(context.Context) error { return nil },
		SessionRevision: rev + 1, UsageRevision: d.admission.proposal.usageRevision, CoverageRevision: d.Parent().CoverageRevision,
		ApplySelected: func(ctx context.Context, tx *sql.Tx) error {
			// The real private DB currently has a writer reservation. A separate
			// protection cannot slip between final qualification and core COMMIT.
			if _, e := private.ExecContext(ctx, "BEGIN IMMEDIATE"); e == nil {
				private.Exec("ROLLBACK")
				return errors.New("private writer crossed the publication barrier")
			} else if !isCJSIBusy(e) {
				return e
			}
			n++
			return nil
		}, VerifyOpened: func(ctx context.Context, tx *sql.Tx) error {
			var one int
			return tx.QueryRowContext(ctx, "SELECT 1 FROM cjsi_publication_head WHERE ticket=?", d.Basis().Ticket).Scan(&one)
		},
	}
	frozen, err := d.FreezePublication(ctx, plan.Seal(), removed, changes)
	if err != nil {
		t.Fatal(err)
	}
	if err = frozen.Open(ctx, data, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var ticket int64
	if err = old.QueryRow("SELECT ticket FROM cjsi_publication_head").Scan(&ticket); err != nil || ticket != oldTicket || n != 1 {
		t.Fatal("old reader changed", ticket, n, err)
	}
	if _, err = private.Exec("BEGIN IMMEDIATE;ROLLBACK"); err != nil {
		t.Fatal("publication retained private writer", err)
	}
	read := func() error {
		return m.WithPublishedCore(ctx, func(tx *sql.Tx, h CJSIIndexStamp) error {
			if h.Ticket != d.Basis().Ticket || h.SessionSeal != plan.Seal() {
				return ErrCJSIIndexChanged
			}
			content, e := m.ReadServingSlot(ctx, tx, now)
			if e == nil && string(content) != string(data) {
				t.Fatal("wrong slot")
			}
			return e
		})
	}
	if err = read(); err != nil {
		t.Fatal(err)
	}
	generation, err := VerifyServingGeneration(ctx, root)
	if err != nil || generation.CoreEpoch != m.epoch || generation.SessionRevision != rev+1 || generation.StateIdentity == "" {
		t.Fatalf("managed serving generation: %+v %v", generation, err)
	}
	var selectedSlot []byte
	if err = f.db.QueryRow("SELECT slot_seal FROM cjsi_publication_head").Scan(&selectedSlot); err != nil {
		t.Fatal(err)
	}
	receipt := CJSIOpenReceipt{CoreEpoch: m.epoch, Ticket: d.Basis().Ticket, Attempt: d.Basis().Attempt, Input: in.Digest, Diff: frozen.proposal.diff, SessionSeal: plan.Seal(), SlotSeal: [32]byte(selectedSlot)}
	confirm := func(tx *sql.Tx) error { return changes.VerifyOpened(ctx, tx) }
	if err = m.ConfirmOpen(ctx, receipt, now, confirm); err != nil {
		t.Fatal("could not recover the opened ticket", err)
	}
	receipt.Diff[0] ^= 1
	if err = m.ConfirmOpen(ctx, receipt, now, confirm); err == nil {
		t.Fatal("confirmed different domain operations")
	}
	// A later private-first protection commits before the next read. Its
	// stale core mirror cannot make that slot visible again.
	if _, err = private.Exec("UPDATE cjsi_index_state SET protection_revision=protection_revision+1"); err != nil {
		t.Fatal(err)
	}
	if err = read(); err == nil {
		t.Fatal("slot remained visible after private-first protection")
	}
}

func isCJSIBusy(e error) bool {
	var sqlite interface{ Code() int }
	return errors.As(e, &sqlite) && sqlite.Code()&255 == 5
}

func TestCJSIUsageOnlyRouting(t *testing.T) {
	f := newCJSIPreparationFixture(t)
	root := f.draft.admission.rootPath
	// Replace the already-migrated fixture's private bytes with an unreadable
	// SQLite image. A core-only route must not run the index seal reader.
	path := filepath.Join(root, cjsiPrivateIndexName)
	if err := closeCJSIInstallReadPool(path); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteAt([]byte("invalid-fixture!"), 0); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	m, err := ResolveCJSIRuntimeRoot(t.Context(), root)
	if err != nil || m == nil {
		t.Fatalf("core-only route opened private index: %v", err)
	}
	// An operation that actually requests the selected index must reject it.
	if err = m.WithPublishedCore(t.Context(), func(tx *sql.Tx, _ CJSIIndexStamp) error {
		return WithCJSIIndexSnapshot(t.Context(), root, tx, func(CJSIIndexSnapshot) error { return nil })
	}); err == nil {
		t.Fatal("fixture did not distinguish a real index open")
	}
}

func TestCJSIPublicationFsyncFences(t *testing.T) {
	f := newCJSIRunnerFixture(t)
	original := f.r.slotIO
	f.r.slotIO = func(ctx context.Context, root *os.File, info os.FileInfo, id [16]byte, data []byte, sealed bool) (cjsiRunnerSlot, error) {
		if sealed {
			return original(ctx, root, info, id, data, sealed)
		}
		calls := 0
		var witnessErr error
		disk, err := writeCJSIServingSlot(ctx, root, info, id, data, func(fd int, identity [16]byte, b []byte) (cjsiDurableSlot, error) {
			return persistCJSIServingSlotAt(fd, identity, b, func(toSync int) error {
				calls++
				// Both actual fsync points admit the maintenance and state writers
				// and a separate SQLite commit; none depend on a wall-time threshold.
				locks := []func(context.Context, string, time.Duration) (*Lock, error){AcquireScanLock, AcquireLock}
				for _, acquire := range locks {
					l, e := acquire(ctx, f.r.admission.rootPath, 0)
					if e != nil {
						return e
					}
					if e = l.Release(); e != nil {
						return e
					}
				}
				pin, e := admitLegacy(ctx, f.r.admission.rootPath)
				if e != nil {
					return e
				}
				if e = pin.close(); e != nil {
					return e
				}
				_, e = f.db.ExecContext(ctx, `BEGIN IMMEDIATE;INSERT INTO inventory_runs VALUES(hex(randomblob(16)),1,'t',0,'complete');COMMIT`)
				if e != nil {
					witnessErr = e
					return e
				}
				return unix.Fsync(toSync)
			})
		})
		if err != nil || calls != 2 || witnessErr != nil {
			return cjsiRunnerSlot{}, errors.Join(errors.New("fsync writer witness failed"), err, witnessErr)
		}
		return publicationSlotIO(ctx, root, info, disk.Identity, data, true)
	}
	if err := f.r.run(t.Context(), f.data); err != nil {
		t.Fatal(err)
	}
}

func TestCJSIUsageOnlyPublication(t *testing.T) {
	ctx := t.Context()
	f := newCJSIMigrationCapsule(t)
	if err := migrateCJSIManagedRoot(ctx, f.root, 0, f); err != nil {
		t.Fatal(err)
	}
	m, err := ResolveCJSIRuntimeRoot(ctx, f.root)
	if err != nil {
		t.Fatal(err)
	}
	id := sha256.Sum256([]byte("isolated scope"))
	neverBuilt := func(ctx context.Context, tx *sql.Tx) error {
		var one int
		return tx.QueryRowContext(ctx, `SELECT 1 FROM cjsi_publication_head WHERE index_epoch=0 AND slot_ticket=0 AND NOT EXISTS(SELECT 1 FROM origin_overrides) AND NOT EXISTS(SELECT 1 FROM session_origins)`).Scan(&one)
	}
	if err = m.SetUsagePending(ctx, id, nil); err != nil {
		t.Fatal(err)
	}
	d, err := m.BeginUsagePublication(ctx, CJSIPublicationInputs{Digest: id, Validate: neverBuilt, UsageIndependent: neverBuilt}, time.Now().Add(time.Minute), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.SetUsagePending(ctx, id, nil); err != nil {
		t.Fatal(err)
	} // idempotent
	err = m.WithPublishedCore(ctx, func(tx *sql.Tx, h CJSIIndexStamp) error {
		c, e := ReadPublishedSourceCoverage(ctx, tx)
		if e != nil {
			return e
		}
		if c.State != "update_pending" || h.Ticket != 0 || h.CoverageRevision != 1 {
			t.Fatalf("held %+v %+v", c, h)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	c := CJSICombinedChanges{Digest: id, Groups: []cjsiledger.Eligibility{}, Validate: neverBuilt, RecheckSources: func(context.Context) error { return nil }, CoverageRevision: d.Parent().CoverageRevision + 1, UsageRevision: d.admission.proposal.usageRevision + 1,
		ApplySelected: func(ctx context.Context, tx *sql.Tx) error {
			if _, e := tx.ExecContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES('standalone','codex','s','r','t','m',13,'fixture',0)`); e != nil {
				return e
			}
			return ClearUsagePendingTx(ctx, tx, id, 1, 2)
		}, VerifyOpened: func(ctx context.Context, tx *sql.Tx) error {
			var n int
			return tx.QueryRowContext(ctx, `SELECT 1 FROM usage_events, cjsi_publication_head h WHERE event_key='standalone' AND input_tokens=13 AND h.ticket=1 AND h.index_epoch=0 AND h.slot_seal IS NULL AND h.coverage_revision=2`).Scan(&n)
		}}
	if err = d.CompleteUsagePublication(ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Lstat(filepath.Join(f.root, cjsiPrivateIndexName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("created or opened never-built index", err)
	}
}

func TestCJSIOriginCommitBarrier(t *testing.T) {
	ctx := t.Context()
	f := newCJSIPreparationFixture(t)
	m, err := ResolveCJSIRuntimeRoot(ctx, f.draft.admission.rootPath)
	if err != nil {
		t.Fatal(err)
	}
	h := f.draft.admission.proposal.base
	write := func(tx *sql.Tx, _ CJSIIndexStamp) error {
		_, e := tx.Exec("UPDATE cjsi_index_state SET protection_revision=protection_revision+1")
		return e
	}
	mirror := func(tx *sql.Tx, _ CJSIIndexStamp) error {
		_, e := tx.Exec("UPDATE cjsi_publication_head SET protection_revision=protection_revision+1")
		return e
	}
	// Run the actual last Task 6 private check under the same BEGIN IMMEDIATE
	// as the user mutation. The competing publisher has independent handles.
	conn, err := f.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(context.Background(), "ROLLBACK")
	if err = verifyOriginSetSelection(ctx, conn, &h); err != nil {
		t.Fatal(err)
	}
	changed := false
	err = m.WithProtectionWrite(ctx, func(tx *sql.Tx, s CJSIIndexStamp) error { changed = true; return write(tx, s) }, mirror)
	if !isCJSIBusy(err) || changed {
		t.Fatal("private protection passed the validated origin writer", changed, err)
	}
	if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
		t.Fatal(err)
	}
	// Reverse order: after a durable private-first protection, a failed core
	// mirror leaves the old selection in core but cannot authorize the old set.
	fail := errors.New("mirror failed")
	if err = m.WithProtectionWrite(ctx, write, func(*sql.Tx, CJSIIndexStamp) error { return fail }); !errors.Is(err, fail) {
		t.Fatal(err)
	}
	mutation := OriginMutation{RequestID: "private-first", PayloadHash: "hash", Kind: OriginMutationOverrideSet, Client: "codex", SessionID: "fixture", Value: "work", Binding: OriginBinding{Selection: &h, Digest: "digest"}, Now: time.Now()}
	if _, err = m.ApplyOriginMutation(ctx, mutation); !errors.Is(err, ErrOriginProjectionStale) {
		t.Fatalf("set after revocation: %v", err)
	}
	assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM origin_mutations WHERE request_id='private-first'", 0)
}

func TestCJSIAbandonedPublicationRetirement(t *testing.T) {
	f := newCJSIPreparationFixture(t)
	ctx := t.Context()
	root := f.draft.admission.rootPath
	m, err := ResolveCJSIRuntimeRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	commitCJSIPrepare(t, f.db, f.reserve, 1)
	p := f.draft.admission.proposal
	if err = m.RetireAbandonedPublication(ctx, p.ticket, p.attempt, nil); !errors.Is(err, ErrTelemetrySourceIdentityPending) {
		t.Fatalf("retired live draft: %v", err)
	}
	assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE state='draft'", 1)
	if _, err = f.db.Exec("UPDATE cjsi_publication_attempts SET accept_until='2000-01-01T00:00:00Z' WHERE ticket=? AND attempt=?", p.ticket, p.attempt); err != nil {
		t.Fatal(err)
	}
	if err = m.RetireAbandonedPublication(ctx, p.ticket, p.attempt, nil); err != nil {
		t.Fatal(err)
	}
	if err = m.RetireAbandonedPublication(ctx, p.ticket, p.attempt, nil); err != nil {
		t.Fatal(err)
	}
	assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE state='retired'", 1)
}
