package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func publicationRetryFixture(t *testing.T) (cjsiPreparationFixture, *CJSIRuntimeRoot) {
	t.Helper()
	f := newCJSIPreparationFixture(t)
	root, err := ResolveCJSIRuntimeRoot(t.Context(), f.draft.admission.rootPath)
	if err != nil || root == nil {
		t.Fatal("managed root admission", err)
	}
	return f, root
}

func freezeRetryFixture(t *testing.T, d CJSIPublicationDraft, failFirstApply error) *CJSIFrozenPublication {
	t.Helper()
	ctx := t.Context()
	plan := publicationIndexFixture{sha256.Sum256([]byte("retry-index"))}
	h := d.Parent()
	if err := d.StageIndex(ctx, plan, h.SessionRevision+1, []CJSISourceWithdrawal{}); err != nil {
		t.Fatal(err)
	}
	noChange := func(context.Context, *sql.Tx) error { return nil }
	c := CJSICombinedChanges{Digest: sha256.Sum256([]byte("retry-delta")), Groups: []cjsiledger.Eligibility{},
		SessionRevision: h.SessionRevision + 1, UsageRevision: d.admission.proposal.usageRevision, CoverageRevision: h.CoverageRevision,
		Validate: noChange, RecheckSources: func(context.Context) error { return nil }, ApplySelected: func(context.Context, *sql.Tx) error {
			err := failFirstApply
			failFirstApply = nil
			return err
		}, VerifyOpened: noChange}
	f, err := d.FreezePublication(ctx, plan.Seal(), []CJSISourceWithdrawal{}, c)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestCJSIPublicationInvalidatedRetry(t *testing.T) {
	for _, tc := range []struct {
		name     string
		explicit bool
	}{
		{"coverage", false}, {"protection", false}, {"usage", false}, {"input", false},
		{"coverage", true}, {"protection", true}, {"usage", true},
	} {
		name := tc.name + "/begin"
		if tc.explicit {
			name = tc.name + "/retire"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			f, m := publicationRetryFixture(t)
			until := time.Now().Add(30 * time.Minute)
			input := func(changed bool) CJSIPublicationInputs {
				digest := sha256.Sum256([]byte("old source"))
				if changed {
					digest = sha256.Sum256([]byte("new source"))
				}
				return CJSIPublicationInputs{Digest: digest, Validate: func(ctx context.Context, tx *sql.Tx) error {
					var current bool
					if e := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM inventory_runs WHERE inventory_id='source-changed')").Scan(&current); e != nil {
						return e
					}
					if current != changed {
						return ErrTelemetrySourceIdentityPending
					}
					return nil
				}}
			}
			first, err := m.BeginPublication(ctx, input(false), until, nil)
			if err != nil {
				t.Fatal(err)
			}
			var failFirstApply error
			if !tc.explicit {
				failFirstApply = errors.New("stop after durable slot seal")
			}
			frozen := freezeRetryFixture(t, first, failFirstApply)
			b := first.Basis()
			id := first.SlotIdentity()
			var slotFile os.FileInfo
			slotPath := filepath.Join(m.path, cjsiServingSlotName(id))
			if failFirstApply != nil {
				if err = frozen.Open(ctx, []byte("old payload"), first.BuiltAt(), until); !errors.Is(err, failFirstApply) {
					t.Fatalf("failed to reach the durable seal: %v", err)
				}
				var oldState string
				if err = f.db.QueryRow("SELECT state FROM cjsi_publication_attempts WHERE ticket=? AND attempt=?", b.Ticket, b.Attempt).Scan(&oldState); err != nil || oldState != "sealed" {
					t.Fatal("old slot not sealed", oldState, err)
				}
				slotFile, err = os.Lstat(slotPath)
				if err != nil {
					t.Fatal(err)
				}
			}

			switch tc.name {
			case "coverage":
				err = m.SetUsagePending(ctx, sha256.Sum256([]byte("other scope")), nil)
			case "protection":
				err = m.WithProtectionWrite(ctx, func(tx *sql.Tx, _ CJSIIndexStamp) error {
					_, e := tx.Exec("UPDATE cjsi_index_state SET protection_revision=protection_revision+1")
					return e
				}, func(tx *sql.Tx, _ CJSIIndexStamp) error {
					_, e := tx.Exec("UPDATE cjsi_publication_head SET protection_revision=protection_revision+1")
					return e
				})
			case "usage":
				_, err = f.db.Exec("UPDATE serving_generation SET revision=revision+1 WHERE singleton=1")
			case "input":
				_, err = f.db.Exec("INSERT INTO inventory_runs VALUES('source-changed',1,'t',0,'complete')")
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = frozen.Open(ctx, []byte("old payload"), first.BuiltAt(), until); err == nil {
				t.Fatal("obsolete attempt opened")
			}
			assertCJSIMigrationNumber(t, f.db, "SELECT ticket FROM cjsi_publication_head", int(first.Parent().Ticket))
			if tc.explicit {
				if err = m.RetireAbandonedPublication(ctx, b.Ticket, b.Attempt, nil); err != nil {
					t.Fatalf("invalidated attempt could not retire: %v", err)
				}
			}
			next, err := m.BeginPublication(ctx, input(tc.name == "input"), until, nil)
			if err != nil {
				t.Fatalf("invalidated attempt blocked sequential retry: %v", err)
			}
			if next.Basis().Ticket != b.Ticket || next.Basis().Attempt <= b.Attempt || next.SlotIdentity() == id {
				t.Fatal("successor reused the retired reservation")
			}
			var state string
			var kept []byte
			if err = f.db.QueryRow("SELECT state,slot_identity FROM cjsi_publication_attempts WHERE ticket=? AND attempt=?", b.Ticket, b.Attempt).Scan(&state, &kept); err != nil || state != "retired" || !bytes.Equal(kept, id[:]) {
				t.Fatal("lost retired reservation", state, err)
			}
			if err = freezeRetryFixture(t, next, nil).Open(ctx, []byte("successor"), next.BuiltAt(), until); err != nil {
				t.Fatalf("successor failed to open: %v", err)
			}
			if slotFile != nil {
				after, e := os.Lstat(slotPath)
				if e != nil || !os.SameFile(slotFile, after) {
					t.Fatalf("retirement removed or replaced old slot: %v", e)
				}
				data, e := os.ReadFile(slotPath)
				if e != nil || string(data) != "old payload" {
					t.Fatalf("retired slot bytes changed: %v", e)
				}
			}
			var active int64
			if err = f.db.QueryRow("SELECT session_attempt FROM cjsi_publication_head WHERE ticket=?", b.Ticket).Scan(&active); err != nil || active != next.Basis().Attempt {
				t.Fatal("wrong selected successor", active, err)
			}
		})
	}
}

func TestCJSIPublicationScopeAndOwnerRevisions(t *testing.T) {
	for _, step := range []string{"exact", "user", "route", "price", "combined_to_usage", "usage_to_combined", "invalid_selector"} {
		t.Run(step, func(t *testing.T) {
			ctx := t.Context()
			f, m := publicationRetryFixture(t)
			valid := func(context.Context, *sql.Tx) error { return nil }
			in := CJSIPublicationInputs{Digest: sha256.Sum256([]byte("scope")), Validate: valid, UsageIndependent: valid}
			until := time.Now().Add(time.Hour)
			first, second := m.BeginPublication, m.BeginPublication
			switch step {
			case "usage_to_combined":
				first = m.BeginUsagePublication
			case "combined_to_usage":
				second = m.BeginUsagePublication
			}
			old, err := first(ctx, in, until, nil)
			if err != nil {
				t.Fatal(err)
			}
			switch step {
			case "user":
				in.UserRevision++
			case "route":
				in.RouteRevision++
			case "price":
				in.PriceRevision++
			case "invalid_selector":
				in.Digest = sha256.Sum256([]byte("unvalidated"))
				in.Validate = func(context.Context, *sql.Tx) error { return ErrTelemetrySourceIdentityPending }
			}
			next, err := second(ctx, in, until, nil)
			var state string
			if e := f.db.QueryRow("SELECT state FROM cjsi_publication_attempts WHERE ticket=? AND attempt=?", old.Basis().Ticket, old.Basis().Attempt).Scan(&state); e != nil {
				t.Fatal(e)
			}
			if step == "invalid_selector" {
				if !errors.Is(err, ErrTelemetrySourceIdentityPending) || state != "draft" {
					t.Fatalf("invalid selector displaced reservation: %s %v", state, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s blocked eligible retry: %v", step, err)
			}
			if step == "exact" {
				if next.Basis() != old.Basis() || next.SlotIdentity() != old.SlotIdentity() || state != "draft" {
					t.Fatal("live equivalent attempt was retired")
				}
				return
			}
			if state != "retired" || next.Basis().Attempt <= old.Basis().Attempt {
				t.Fatalf("%s did not replace the original reservation: %s", step, state)
			}
		})
	}
}

func TestCJSIPublicationCoveragePrecedence(t *testing.T) {
	ctx := t.Context()
	f, m := publicationRetryFixture(t)
	pending := sha256.Sum256([]byte("pending"))
	if err := m.SetUsagePending(ctx, pending, nil); err != nil {
		t.Fatal(err)
	}
	key := sha256.Sum256([]byte("missing"))
	var window [32]byte
	for _, tc := range []struct {
		name, want                 string
		partial, masked, inventory bool
	}{
		{"pending", "update_pending", false, false, false}, {"partial", "partial", true, false, false}, {"masked", "partial", false, true, false}, {"inventory", "partial", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := f.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if tc.partial || tc.masked {
				_, err = tx.Exec("INSERT INTO cjsi_scope_overlays(domain,scope_kind,scope_key,window_key,pending,partial,masked,coverage_revision,protection_revision) VALUES('usage','group',?,?,0,?,?,1,0)", key[:], window[:], tc.partial, tc.masked)
			}
			if tc.inventory {
				_, err = tx.Exec("INSERT INTO inventory_runs SELECT 'i',epoch,'t',1,'partial' FROM derived_snapshot_generation WHERE singleton=1; INSERT INTO domain_coverage_heads VALUES(1,'i')")
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := ReadPublishedSourceCoverage(ctx, tx)
			if err != nil || got.State != tc.want {
				t.Fatalf("coverage precedence %s: got %q, want %q, err %v", tc.name, got.State, tc.want, err)
			}
		})
	}
}

func TestCJSIPublicationAttemptTimeBounds(t *testing.T) {
	for _, tc := range []struct {
		name           string
		last, deadline time.Duration
		accept         bool
	}{
		{"allocation_after_deadline", 4 * time.Hour, time.Hour, false}, {"allocation_at_deadline", time.Hour - time.Microsecond, time.Hour, false}, {"allocation_after_now", time.Hour, 2 * time.Hour, false}, {"past_allocation", -time.Minute, time.Hour, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			f, m := publicationRetryFixture(t)
			now := time.Now().UTC().Truncate(time.Microsecond)
			p := f.draft.admission.proposal
			last := now.Add(tc.last).UnixMicro()
			_, err := f.db.Exec(`INSERT INTO cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,index_epoch,session_revision,usage_revision,user_revision,route_revision,price_revision,protection_revision,coverage_revision,input_digest,slot_identity,accept_until,state)
 VALUES(1,?,0,?,?,?,?,0,0,0,?,?,zeroblob(32),randomblob(16),?,'retired')`, last, p.base.CoreEpoch, p.base.IndexEpoch, p.base.SessionRevision, p.usageRevision, p.base.ProtectionRevision, p.base.CoverageRevision, now.Format(time.RFC3339Nano))
			if err != nil {
				t.Fatal(err)
			}
			d, err := m.BeginPublication(ctx, CJSIPublicationInputs{BuiltAt: now.Add(-2 * time.Hour), Digest: sha256.Sum256([]byte("clock")), Validate: func(context.Context, *sql.Tx) error { return nil }}, now.Add(tc.deadline), nil)
			if tc.accept {
				if err != nil || d.Basis().Attempt != last+1 {
					t.Fatalf("valid monotonic allocation: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrTelemetrySourceIdentityPending) {
				t.Fatalf("invalid allocated clock admitted: built=%s until=%s err=%v", d.BuiltAt(), d.AcceptUntil(), err)
			}
			assertCJSIMigrationNumber(t, f.db, "SELECT count(*) FROM cjsi_publication_attempts WHERE state='draft'", 0)
		})
	}
}

func TestCJSIPublicationWithdrawalSet(t *testing.T) {
	ctx := t.Context()
	f, m := publicationRetryFixture(t)
	private, err := m.openPublicationIndex(ctx, f.draft.admission.proposal.base, true)
	if err != nil {
		t.Fatal(err)
	}
	defer private.Close()
	tx, err := private.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, p := range []string{"replaced", "deleted", "same"} {
		if _, err = tx.Exec("INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,'old',0,0,0,'',0,1,'')", p); err != nil {
			t.Fatal(err)
		}
	}
	h := f.draft.admission.proposal.base
	for _, attempt := range []int{1, 2} {
		if _, err = tx.Exec(`INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,parent_ticket,protection_revision,coverage_revision,session_revision)
 VALUES(31,?,?,?,0,0,?,?,0,?,?,?)`, attempt, h.CoreEpoch, h.IndexEpoch, h.SessionSeal[:], h.SessionRevision, h.ProtectionRevision, h.CoverageRevision, h.SessionRevision+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = tx.Exec("INSERT INTO cjsi_candidate_sources(ticket,attempt,source_path,op) VALUES(31,1,'deleted','delete')"); err != nil {
		t.Fatal(err)
	}
	for _, row := range [][2]string{{"replaced", "new"}, {"same", "old"}, {"new-path", "new"}} {
		path, id := row[0], row[1]
		if _, err = tx.Exec("INSERT INTO cjsi_candidate_sources(ticket,attempt,source_path,op,source_kind,identity,priority,parser_version,complete_end,size,modified_at,changed_at,prefix_hash,scanned_at) VALUES(31,1,?,'upsert','codex-jsonl',?,0,1,0,0,0,0,'','')", path, id); err != nil {
			t.Fatal(err)
		}
	}
	a, b := CJSISourceWithdrawal{"replaced", "old"}, CJSISourceWithdrawal{"deleted", "old"}
	for _, tc := range []struct {
		name   string
		list   []CJSISourceWithdrawal
		accept bool
	}{
		{"complete", []CJSISourceWithdrawal{b, a}, true}, {"empty", []CJSISourceWithdrawal{}, false}, {"truncated", []CJSISourceWithdrawal{b}, false}, {"duplicate", []CJSISourceWithdrawal{a, a}, false}, {"complete_plus_duplicate", []CJSISourceWithdrawal{a, b, a}, false}, {"extra", []CJSISourceWithdrawal{a, b, {"same", "old"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyCJSIWithdrawals(ctx, tx, 31, 1, tc.list)
			if tc.accept && err != nil || !tc.accept && !errors.Is(err, ErrCJSIIndexChanged) {
				t.Fatalf("withdrawal set %s: accepted=%t error=%v", tc.name, err == nil, err)
			}
		})
	}
	// Attempt 2 has only an orphan delete. An inner join would silently
	// discard it, so even a manifest claiming no withdrawals must fail.
	if _, err = tx.Exec("INSERT INTO cjsi_candidate_sources(ticket,attempt,source_path,op) VALUES(31,2,'orphan','delete')"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		list []CJSISourceWithdrawal
	}{
		{"orphan_empty", []CJSISourceWithdrawal{}},
		{"orphan_fabricated", []CJSISourceWithdrawal{{Path: "orphan", Identity: "fabricated"}}},
		{"orphan_sentinel_identity", []CJSISourceWithdrawal{{Path: "orphan", Identity: ""}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := verifyCJSIWithdrawals(ctx, tx, 31, 2, tc.list); !errors.Is(err, ErrCJSIIndexChanged) {
				t.Fatalf("orphan delete with %s manifest: got %v, want ErrCJSIIndexChanged", tc.name, err)
			}
		})
	}

}

func TestCJSIPublicationDisplacedWorksetRoot(t *testing.T) {
	ctx := t.Context()
	f := newCJSIPreparationFixture(t)
	root := f.draft.admission.rootPath
	m, err := ResolveCJSIRuntimeRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	in := CJSIPublicationInputs{BuiltAt: now, Digest: sha256.Sum256([]byte("repro-3")), Validate: func(context.Context, *sql.Tx) error { return nil }}
	d, err := m.BeginPublication(ctx, in, now.Add(time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Prepare a valid source file
	srcPath := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(srcPath, []byte(`{"type":"session_meta","payload":{"id":"00000000-0000-4000-8000-000000000001"}}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	id, chg, ok := ingest.FileGeneration(st)
	if !ok {
		t.Fatal("unstable")
	}
	src := ingest.Source{Client: "codex", Path: srcPath, Identity: id, Size: st.Size(), ModifiedAt: st.ModTime().UnixNano(), ChangedAt: chg, Stable: true}
	fh, err := os.Open(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	red, err := ingest.CaptureCodexReduction(ctx, fh, srcPath, st.Size(), 1<<16)
	fh.Close()
	if err != nil {
		t.Fatal(err)
	}
	ev := red.Capture().Evidence()
	ev.Version = ingest.CodexCheckpointVersion{Generation: 1, Parser: cjsiledger.Parser, MaxLineBytes: 1 << 16}
	sel := ingest.CodexSelectedSource{Path: srcPath, ThreadID: "00000000-0000-4000-8000-000000000001", Epoch: 1, Evidence: ev}
	// Displace root with an empty 0700 successor directory
	moved := filepath.Join(filepath.Dir(root), "moved-repro3")
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(root); _ = os.Rename(moved, root) }()
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	err = d.WithSourceWorkset(ctx, src, sel, 256, func(CJSICapturedWorkset) error { return nil })
	if !errors.Is(err, errCJSISlotAdmission) {
		t.Errorf("displaced workset root: expected slot refusal, got %v", err)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 0 {
		t.Errorf("successor root was modified, entries=%v", names)
	}
}

// Exercise the registered receipt, not just the input to Open. The expired
// cases reconstruct a historical seal with its matching checksum and bounds,
// without testing a filesystem operation against a wall-clock threshold.
func TestCJSIPublicationSealedWindowRetry(t *testing.T) {
	for _, path := range []string{"begin", "retire", "live"} {
		t.Run(path, func(t *testing.T) {
			ctx := t.Context()
			f, m := publicationRetryFixture(t)
			now := time.Now().UTC()
			until := now.Add(time.Hour)
			oldEnd := now.Add(30 * time.Minute)
			in := CJSIPublicationInputs{BuiltAt: now.Add(-2 * time.Hour), Digest: sha256.Sum256([]byte("window-input")), Validate: func(context.Context, *sql.Tx) error { return nil }}
			first, err := m.BeginPublication(ctx, in, until, nil)
			if err != nil {
				t.Fatal(err)
			}
			stop := errors.New("stop before core commit")
			frozen := freezeRetryFixture(t, first, stop)
			data := []byte("old window")
			built := first.BuiltAt()
			if err = frozen.Open(ctx, data, built, oldEnd); !errors.Is(err, stop) {
				t.Fatalf("durable seal boundary: %v", err)
			}
			b := first.Basis()
			id := first.SlotIdentity()
			file := filepath.Join(m.path, cjsiServingSlotName(id))
			st, err := os.Lstat(file)
			if err != nil {
				t.Fatal(err)
			}
			if path == "live" {
				if err = m.RetireAbandonedPublication(ctx, b.Ticket, b.Attempt, nil); !errors.Is(err, ErrTelemetrySourceIdentityPending) {
					t.Fatalf("retired live sealed window: %v", err)
				}
				same, e := m.BeginPublication(ctx, in, until.Add(time.Hour), nil)
				if e != nil || same.Basis() != b || same.SlotIdentity() != id || !same.AcceptUntil().Equal(until) {
					t.Fatalf("lost live sealed reservation: %v", e)
				}
				if e = freezeRetryFixture(t, same, nil).Open(ctx, data, built, oldEnd); e != nil {
					t.Fatal(e)
				}
				return
			}
			p := frozen.proposal
			p.expected.digest = sha256.Sum256(data)
			p.expected.size = int64(len(data))
			p.expected.format = 1
			p.expected.builtAt, p.expected.validUntil = built, oldEnd
			prior := p.seal()
			p.expected.validUntil = now.Add(-time.Hour)
			expired := p.seal()
			if err = cjsiOpenOne(f.db.Exec(`UPDATE cjsi_publication_attempts SET valid_until=?,slot_seal=?
   WHERE ticket=? AND attempt=? AND state='sealed' AND slot_seal=? AND built_at=? AND valid_until=?`,
				p.expected.validUntil.Format(time.RFC3339Nano), expired[:], b.Ticket, b.Attempt, prior[:], built.Format(time.RFC3339Nano), oldEnd.Format(time.RFC3339Nano))); err != nil {
				t.Fatal(err)
			}
			if err = frozen.Open(ctx, data, built, p.expected.validUntil); err == nil {
				t.Fatal("expired slot opened")
			}
			assertCJSIMigrationNumber(t, f.db, "SELECT ticket FROM cjsi_publication_head", int(b.BaseTicket))
			if path == "retire" {
				if err = m.RetireAbandonedPublication(ctx, b.Ticket, b.Attempt, nil); err != nil {
					t.Fatalf("expired sealed window blocked retirement: %v", err)
				}
			}
			in.BuiltAt = time.Now().UTC()
			next, err := m.BeginPublication(ctx, in, until, nil)
			if err != nil {
				t.Fatalf("begin after slot expiry: %v", err)
			}
			if next.Basis().Ticket != b.Ticket || next.Basis().Attempt <= b.Attempt || next.SlotIdentity() == id {
				t.Fatal("expired sealed window reused the old reservation")
			}
			if err = freezeRetryFixture(t, next, nil).Open(ctx, []byte("new window"), next.BuiltAt(), until); err != nil {
				t.Fatalf("successor open after slot expiry: %v", err)
			}
			var retained int
			err = f.db.QueryRow("SELECT 1 FROM cjsi_publication_attempts WHERE ticket=? AND attempt=? AND state='retired' AND slot_identity=? AND slot_seal=?", b.Ticket, b.Attempt, id[:], expired[:]).Scan(&retained)
			if err != nil {
				t.Fatalf("lost retired receipt: %v", err)
			}
			after, err := os.Lstat(file)
			if err != nil || !os.SameFile(st, after) {
				t.Fatalf("lost retired slot inode: %v", err)
			}
			saved, err := os.ReadFile(file)
			if err != nil || !bytes.Equal(saved, data) {
				t.Fatalf("retired slot changed: %v", err)
			}
			var selected int64
			if err = f.db.QueryRow("SELECT slot_attempt FROM cjsi_publication_head WHERE ticket=?", b.Ticket).Scan(&selected); err != nil || selected != next.Basis().Attempt {
				t.Fatalf("wrong successor %d: %v", selected, err)
			}
		})
	}
}

func TestCJSIPublicationStoredWindowAdmission(t *testing.T) {
	ctx := t.Context()
	f, m := publicationRetryFixture(t)
	now := time.Now().UTC()
	until := now.Add(time.Hour)
	cutover := now.Add(15 * time.Minute).Truncate(time.Microsecond)
	in := CJSIPublicationInputs{BuiltAt: now.Add(-time.Hour), Digest: sha256.Sum256([]byte("stored-window")), Validate: func(context.Context, *sql.Tx) error { return nil }}
	for _, tc := range []struct {
		name, state string
		built, end  any
		live        bool
	}{
		{"draft", "draft", nil, nil, true}, {"draft_partial_window", "draft", nil, until, false},
		{"missing_end", "sealed", in.BuiltAt, nil, false}, {"bad_end", "sealed", in.BuiltAt, "not-an-instant", false},
		{"bad_build", "sealed", "not-an-instant", until, false}, {"missing_build", "sealed", nil, until, false},
		{"equal_end", "sealed", cutover, cutover, false}, {"reversed", "sealed", cutover.Add(time.Minute), cutover, false},
		{"allocation_at_end", "sealed", in.BuiltAt, cutover, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := m.BeginPublication(ctx, in, until, nil)
			if err != nil {
				t.Fatal(err)
			}
			stamp := func(v any) any {
				if at, ok := v.(time.Time); ok {
					return at.Format(time.RFC3339Nano)
				}
				return v
			}
			b := d.Basis()
			attempt := b.Attempt
			if tc.name == "allocation_at_end" {
				attempt = cutover.UnixMicro()
			}
			if _, err = f.db.Exec("UPDATE cjsi_publication_attempts SET attempt=?,state=?,built_at=?,valid_until=? WHERE ticket=? AND attempt=?", attempt, tc.state, stamp(tc.built), stamp(tc.end), b.Ticket, b.Attempt); err != nil {
				t.Fatal(err)
			}
			err = m.RetireAbandonedPublication(ctx, b.Ticket, attempt, nil)
			if tc.live {
				if !errors.Is(err, ErrTelemetrySourceIdentityPending) {
					t.Fatalf("retired live draft: %v", err)
				}
				again, e := m.BeginPublication(ctx, in, until, nil)
				if e != nil || again.Basis() != b {
					t.Fatalf("did not reuse live draft: %v", e)
				}
			} else if err != nil {
				t.Fatalf("unusable %s window cannot retire: %v", tc.name, err)
			}
		})
	}
}

func TestCJSIPublicationNarrowerSealedWindow(t *testing.T) {
	ctx := t.Context()
	f, m := publicationRetryFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	accept := now.Add(time.Hour)
	in := CJSIPublicationInputs{
		BuiltAt: now.Add(-time.Minute), Digest: sha256.Sum256([]byte("narrower pricing window")),
		Validate:       func(context.Context, *sql.Tx) error { return nil },
		SlotValidUntil: func(built time.Time) time.Time { return built.Add(30 * time.Minute) },
	}
	first, err := m.BeginPublication(ctx, in, accept, nil)
	if err != nil {
		t.Fatal(err)
	}
	stop := errors.New("stop after slot seal")
	if err := freezeRetryFixture(t, first, stop).Open(ctx, []byte("wide slot"), first.BuiltAt(), first.SlotValidUntil()); !errors.Is(err, stop) {
		t.Fatalf("did not reach the durable seal: %v", err)
	}

	basis, id := first.Basis(), first.SlotIdentity()
	path := filepath.Join(m.path, cjsiServingSlotName(id))
	inode, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	var originalSeal []byte
	if err := f.db.QueryRow("SELECT slot_seal FROM cjsi_publication_attempts WHERE ticket=? AND attempt=? AND slot_identity=? AND state='sealed'", basis.Ticket, basis.Attempt, id[:]).Scan(&originalSeal); err != nil || len(originalSeal) != sha256.Size {
		t.Fatalf("missing original sealed receipt: %v", err)
	}

	in.SlotValidUntil = func(built time.Time) time.Time { return built.Add(10 * time.Minute) }
	next, err := m.BeginPublication(ctx, in, accept, nil)
	if err != nil {
		t.Fatal(err)
	}
	nb := next.Basis()
	if nb.Ticket != basis.Ticket || nb.Attempt <= basis.Attempt || next.SlotIdentity() == id || next.SlotIdentity() == ([16]byte{}) {
		t.Fatal("narrower policy did not replace the sealed reservation")
	}
	if !next.SlotValidUntil().Equal(in.SlotValidUntil(next.BuiltAt())) {
		t.Fatal("successor has the wrong admitted cutoff")
	}
	if err := freezeRetryFixture(t, next, nil).Open(ctx, []byte("narrow slot"), next.BuiltAt(), next.SlotValidUntil()); err != nil {
		t.Fatalf("open at the successor's admitted cutoff: %v", err)
	}
	var selected int64
	if err := f.db.QueryRow("SELECT slot_attempt FROM cjsi_publication_head WHERE ticket=? AND slot_ticket=?", nb.Ticket, nb.Ticket).Scan(&selected); err != nil || selected != nb.Attempt {
		t.Fatalf("wrong opened successor: %d, %v", selected, err)
	}
	var state string
	var retainedID, retainedSeal []byte
	if err := f.db.QueryRow("SELECT state,slot_identity,slot_seal FROM cjsi_publication_attempts WHERE ticket=? AND attempt=?", basis.Ticket, basis.Attempt).Scan(&state, &retainedID, &retainedSeal); err != nil || state != "retired" || !bytes.Equal(retainedID, id[:]) || !bytes.Equal(retainedSeal, originalSeal) {
		t.Fatalf("original sealed receipt was not retained: state=%q err=%v", state, err)
	}
	if after, err := os.Lstat(path); err != nil || !os.SameFile(inode, after) {
		t.Fatalf("retired slot inode changed: %v", err)
	}
}

func TestCJSIPublicationOpenRequiresAdmittedCutoff(t *testing.T) {
	ctx := t.Context()
	f, m := publicationRetryFixture(t)
	now := time.Now().UTC()
	in := CJSIPublicationInputs{
		BuiltAt: now.Add(-time.Minute), Digest: sha256.Sum256([]byte("admitted slot cutoff")),
		Validate:       func(context.Context, *sql.Tx) error { return nil },
		SlotValidUntil: func(built time.Time) time.Time { return built.Add(20 * time.Minute) },
	}
	d, err := m.BeginPublication(ctx, in, now.Add(time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	frozen := freezeRetryFixture(t, d, nil)
	head := cjsiSlotCoreImage(t, f.db)["cjsi_publication_head"]
	if len(head) != 1 {
		t.Fatal("missing initial publication head")
	}
	payload := []byte("admitted payload")
	// Both alternatives are live and inside accept_until; only equality with
	// the admitted cutoff can distinguish either one from the correct window.
	for _, offset := range []time.Duration{-time.Minute, time.Minute} {
		if err := frozen.Open(ctx, payload, d.BuiltAt(), d.SlotValidUntil().Add(offset)); !errors.Is(err, errCJSISlotAdmission) {
			t.Errorf("Open with offset %s: got %v, want slot admission error", offset, err)
		}
		if got := cjsiSlotCoreImage(t, f.db)["cjsi_publication_head"]; !slices.Equal(head, got) {
			t.Fatal("mismatched cutoff changed the publication head")
		}
	}
	if err := frozen.Open(ctx, payload, d.BuiltAt(), d.SlotValidUntil()); err != nil {
		t.Fatalf("admitted cutoff did not open: %v", err)
	}
}

func TestCJSIPublicationClockMissRevalidatesInput(t *testing.T) {
	ctx := t.Context()
	f, m := publicationRetryFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	accept := now.Add(time.Hour)
	old := CJSIPublicationInputs{BuiltAt: now.Add(-5 * time.Minute), Digest: sha256.Sum256([]byte("prior selection")), Validate: func(ctx context.Context, tx *sql.Tx) error {
		var replaced bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM inventory_runs WHERE inventory_id='clock-peer')").Scan(&replaced); err != nil {
			return err
		}
		if replaced {
			return ErrTelemetrySourceIdentityPending
		}
		return nil
	}}
	initial, err := m.BeginPublication(ctx, old, accept, nil)
	if err != nil {
		t.Fatal(err)
	}
	var peer CJSIPublicationDraft
	var peerErr error
	changed := false
	old.BuiltAt = now
	old.SlotValidUntil = func(built time.Time) time.Time {
		if built.Equal(initial.BuiltAt()) && !changed {
			changed = true
			// This is the old clock discovered during writer admission. Change
			// the registered input and let a real peer win before readmission.
			_, peerErr = f.db.ExecContext(ctx, "INSERT INTO inventory_runs VALUES('clock-peer',1,'t',0,'complete')")
			if peerErr != nil {
				return time.Time{}
			}
			peer, peerErr = m.BeginPublication(ctx, CJSIPublicationInputs{
				BuiltAt: now, Digest: sha256.Sum256([]byte("peer selection")),
				Validate: func(ctx context.Context, tx *sql.Tx) error {
					var one int
					return tx.QueryRowContext(ctx, "SELECT 1 FROM inventory_runs WHERE inventory_id='clock-peer'").Scan(&one)
				},
			}, accept, nil)
		}
		return built.Add(30 * time.Minute)
	}
	_, err = m.BeginPublication(ctx, old, accept, nil)
	if !changed || peerErr != nil || peer.Basis().Attempt <= initial.Basis().Attempt {
		t.Fatalf("peer did not win at the clock miss: changed=%t err=%v", changed, peerErr)
	}
	if !errors.Is(err, ErrTelemetrySourceIdentityPending) {
		t.Errorf("stale input was readmitted after the clock miss: %v", err)
	}
	var state string
	peerID := peer.SlotIdentity()
	if err := f.db.QueryRow("SELECT state FROM cjsi_publication_attempts WHERE ticket=? AND attempt=? AND slot_identity=?", peer.Basis().Ticket, peer.Basis().Attempt, peerID[:]).Scan(&state); err != nil || state != "draft" {
		t.Fatalf("peer reservation lost: state=%q err=%v", state, err)
	}
}

func TestCJSIPublicationClosedSuccessorKeepsDraft(t *testing.T) {
	ctx := t.Context()
	f, m := publicationRetryFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	accept := now.Add(time.Hour)
	in := CJSIPublicationInputs{BuiltAt: now.Add(-3 * time.Minute), Digest: sha256.Sum256([]byte("live reservation")), Validate: func(context.Context, *sql.Tx) error { return nil }}
	prior, err := m.BeginPublication(ctx, in, accept, nil)
	if err != nil {
		t.Fatal(err)
	}
	in.BuiltAt = now.Add(-2 * time.Minute)
	in.Digest = sha256.Sum256([]byte("successor input"))
	// After the proposed build time, but already expired at admission.
	in.SlotValidUntil = func(built time.Time) time.Time { return built.Add(time.Minute) }
	_, err = m.BeginPublication(ctx, in, accept, nil)
	if !errors.Is(err, ErrTelemetrySourceIdentityPending) {
		t.Errorf("expired successor cutoff was admitted: %v", err)
	}
	var state string
	id := prior.SlotIdentity()
	if err := f.db.QueryRow("SELECT state FROM cjsi_publication_attempts WHERE ticket=? AND attempt=? AND slot_identity=?", prior.Basis().Ticket, prior.Basis().Attempt, id[:]).Scan(&state); err != nil || state != "draft" {
		t.Fatalf("expired successor displaced the prior reservation: state=%q err=%v", state, err)
	}
	var later int
	if err := f.db.QueryRow("SELECT count(*) FROM cjsi_publication_attempts WHERE ticket=? AND attempt>?", prior.Basis().Ticket, prior.Basis().Attempt).Scan(&later); err != nil || later != 0 {
		t.Fatalf("expired successor left %d newer attempts: %v", later, err)
	}
}
