package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"slices"
	"time"

	"github.com/kitdine/agent-deck/internal/usage/cjsiledger"
	"golang.org/x/sys/unix"
)

// CJSIPublicationDraft is an allocated attempt at one checked physical root.
// Reopening the same input reuses the original deadline and reservation.
type CJSIPublicationDraft struct {
	root           CJSIRuntimeRoot
	election       CJSIRuntimeElection
	admission      cjsiSlotAdmission
	usageOnly      bool
	slotValidUntil time.Time
	validate       func(context.Context, *sql.Tx) error
}

func (d CJSIPublicationDraft) Basis() cjsiledger.Basis {
	p := d.admission.proposal
	return cjsiledger.Basis{Ticket: p.ticket, Attempt: p.attempt, BaseTicket: p.base.Ticket, CoreEpoch: p.base.CoreEpoch, InputDigest: p.input}
}
func (d CJSIPublicationDraft) Parent() CJSIIndexStamp { return d.admission.proposal.base }

// BuiltAt is recovered from the original allocation; a restarted planner
// does not give the same ticket a new clock or a different source timestamp.
func (d CJSIPublicationDraft) BuiltAt() time.Time     { return time.UnixMicro(d.Basis().Attempt).UTC() }
func (d CJSIPublicationDraft) AcceptUntil() time.Time { return d.admission.proposal.acceptUntil }

// SlotValidUntil is the admitted serving cutoff. It is zero for an unsealed
// low-level reservation with no cutoff policy, and for usage-only work.
func (d CJSIPublicationDraft) SlotValidUntil() time.Time { return d.slotValidUntil }

func (d CJSIPublicationDraft) SlotIdentity() [16]byte { return d.admission.proposal.expected.identity }

func (d CJSIPublicationDraft) ServingGenerationFor(h CJSIIndexStamp, usage int64) ServingGeneration {
	return d.root.ServingGenerationFor(h, usage)
}

type CJSIPublicationInputs struct {
	// SlotValidUntil derives the combined selector's cutoff from its captured
	// day/pricing boundaries. It runs outside all publication writer fences.
	// Usage-only admission has no slot. Low-level callers may leave it nil for
	// a draft whose slot validity has not been chosen yet.
	SlotValidUntil func(time.Time) time.Time
	// A separate scope proof is required for a usage-only writer. It must
	// establish that no indexed session, classification, marker or promised
	// derived component can change as a result of the registered delta.
	UsageIndependent func(context.Context, *sql.Tx) error
	// BuiltAt is the shared business clock, captured before planning.
	BuiltAt time.Time
	Digest  [32]byte
	// Validate checks the authenticated source inventory and dependencies in
	// this transaction. Re-run the same proof under writer admission.
	Validate                                   func(context.Context, *sql.Tx) error
	UserRevision, RouteRevision, PriceRevision int64
}

func (m CJSIRuntimeRoot) BeginPublication(ctx context.Context, inputs CJSIPublicationInputs, until time.Time, election CJSIRuntimeElection) (CJSIPublicationDraft, error) {
	return m.beginPublication(ctx, inputs, until, election, false)
}

// BeginUsagePublication allocates a single-domain attempt without opening or
// creating the index. Validate must establish independence for the complete
// affected group, including any previously promised serving dependencies.
func (m CJSIRuntimeRoot) BeginUsagePublication(ctx context.Context, inputs CJSIPublicationInputs, until time.Time, election CJSIRuntimeElection) (CJSIPublicationDraft, error) {
	if inputs.UsageIndependent == nil || inputs.Validate == nil {
		return CJSIPublicationDraft{}, ErrTelemetrySourceIdentityPending
	}
	validate := inputs.Validate
	inputs.Validate = func(ctx context.Context, tx *sql.Tx) error {
		if err := validate(ctx, tx); err != nil {
			return err
		}
		return inputs.UsageIndependent(ctx, tx)
	}
	return m.beginPublication(ctx, inputs, until, election, true)
}

// Only the current attempt and its possible successor matter to a writer.
// A miss releases the transaction and both fences before evaluating a clock;
// the next writer admission rechecks the actual head, election and inputs.
type cjsiAllocationWindows struct {
	known   map[int64]time.Time
	missing int64
}

func (w *cjsiAllocationWindows) lookup(built time.Time) time.Time {
	key := built.UnixMicro()
	end, ok := w.known[key]
	if !ok {
		w.missing = key
	}
	return end
}
func (m CJSIRuntimeRoot) beginPublication(ctx context.Context, inputs CJSIPublicationInputs, until time.Time, election CJSIRuntimeElection, usageOnly bool) (CJSIPublicationDraft, error) {
	if inputs.Validate == nil || inputs.Digest == ([32]byte{}) || !time.Now().Before(until) {
		return CJSIPublicationDraft{}, ErrTelemetrySourceIdentityPending
	}
	if usageOnly || inputs.SlotValidUntil == nil {
		return m.tryBeginPublication(ctx, inputs, until, election, usageOnly, nil)
	}
	if inputs.BuiltAt.IsZero() {
		inputs.BuiltAt = time.Now().UTC()
	}
	key := inputs.BuiltAt.UnixMicro()
	if key <= 0 || !inputs.BuiltAt.Before(until) {
		return CJSIPublicationDraft{}, ErrTelemetrySourceIdentityPending
	}
	windows := cjsiAllocationWindows{known: make(map[int64]time.Time, 2)}
	for {
		if err := ctx.Err(); err != nil {
			return CJSIPublicationDraft{}, err
		}
		if !time.Now().Before(until) {
			return CJSIPublicationDraft{}, ErrTelemetrySourceIdentityPending
		}
		if len(windows.known) == 2 {
			clear(windows.known)
		}
		windows.known[key] = inputs.SlotValidUntil(time.UnixMicro(key).UTC()).UTC()
		windows.missing = 0
		draft, err := m.tryBeginPublication(ctx, inputs, until, election, false, &windows)
		if err != nil || windows.missing == 0 {
			return draft, err
		}
		key = windows.missing
	}
}

func (m CJSIRuntimeRoot) tryBeginPublication(ctx context.Context, inputs CJSIPublicationInputs, until time.Time, election CJSIRuntimeElection, usageOnly bool, windows *cjsiAllocationWindows) (CJSIPublicationDraft, error) {
	var d CJSIPublicationDraft
	err := m.withPublicationLocks(ctx, election, func() (err error) {
		db, err := m.runner().core(ctx, true)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, db.Close()) }()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		h, err := cjsiReadIndexHead(ctx, tx)
		if err != nil {
			return err
		}
		j, err := cjsiReadLayoutJournal(ctx, tx)
		if err != nil {
			return err
		}
		if h.CoreEpoch != m.epoch || !usageOnly && h.IndexEpoch <= 0 || j.epoch != h.IndexEpoch || h.Ticket == math.MaxInt64 {
			return ErrTelemetrySourceIdentityPending
		}
		h.Device, h.Inode = j.prepared.identity.device, j.prepared.identity.inode
		clock := inputs.BuiltAt
		if clock.IsZero() {
			clock = time.Now().UTC()
		}
		allocation := clock.UnixMicro()
		if allocation <= 0 || !clock.Before(until) {
			return ErrTelemetrySourceIdentityPending
		}
		p := cjsiSlotProposal{ticket: h.Ticket + 1, attempt: allocation, base: h, input: inputs.Digest, userRevision: inputs.UserRevision, routeRevision: inputs.RouteRevision, priceRevision: inputs.PriceRevision, acceptUntil: until.UTC()}
		var prior []byte
		err = tx.QueryRowContext(ctx, `SELECT h.slot_ticket,h.slot_attempt,h.slot_seal,g.revision FROM cjsi_publication_head h JOIN serving_generation g USING(singleton) WHERE h.singleton=1`).Scan(&p.baseSlotTicket, &p.baseSlotAttempt, &prior, &p.usageRevision)
		if err != nil {
			return err
		}
		copy(p.baseSlotSeal[:], prior)
		if usageOnly {
			if err = m.checkUsageLocation(j, h); err != nil {
				return err
			}
		}
		if err = inputs.Validate(ctx, tx); err != nil {
			return err
		}
		// Only validated input may supersede a live reservation. The winner
		// retires and replaces the old attempt atomically under this writer.
		var identity []byte
		var state, deadline string
		var slotWindow cjsiStoredSlotWindow
		var last int64
		var matches bool
		err = tx.QueryRowContext(ctx, `SELECT a.attempt,a.state,a.slot_identity,a.accept_until,a.built_at,a.valid_until,
    (`+cjsiAttemptAtCurrentBase+` AND a.user_revision=? AND a.route_revision=? AND a.price_revision=?
     AND a.input_digest=? AND (a.slot_identity IS NULL)=?)
    FROM cjsi_publication_attempts a JOIN cjsi_publication_head h ON h.singleton=1
    JOIN serving_generation g ON g.singleton=h.singleton
    WHERE a.ticket=? ORDER BY a.attempt DESC LIMIT 1`,
			p.userRevision, p.routeRevision, p.priceRevision, p.input[:], usageOnly, p.ticket).Scan(&last, &state, &identity, &deadline, &slotWindow.builtAt, &slotWindow.validUntil, &matches)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		admittedAt := time.Now()
		var slotEnd time.Time
		reuse := false
		if err == nil {
			expires, e := time.Parse(time.RFC3339Nano, deadline)
			if e != nil {
				return e
			}
			if matches {
				slotEnd, reuse = slotWindow.openWindow(state, last, expires, admittedAt, windows)
				if windows != nil && windows.missing != 0 {
					return nil
				}
			}
			switch {
			case state == "opened":
				return ErrCJSIIndexChanged
			case reuse:
				p.attempt, p.acceptUntil = last, expires.UTC()
				copy(p.expected.identity[:], identity)
			default:
				if last == math.MaxInt64 {
					return ErrCJSIIndexChanged
				}
				p.attempt = max(allocation, last+1)
			}
		}
		// The sequence doubles as the recoverable build clock. Test the final
		// allocation, including monotonic bumps, against its actual admission window.
		built := time.UnixMicro(p.attempt)
		if built.After(admittedAt) || !built.Before(p.acceptUntil) || !admittedAt.Before(p.acceptUntil) {
			return ErrTelemetrySourceIdentityPending
		}
		if !reuse && windows != nil {
			slotEnd = windows.lookup(built)
			if windows.missing != 0 {
				return nil
			}
			if !built.Before(slotEnd) || !admittedAt.Before(slotEnd) {
				return ErrTelemetrySourceIdentityPending
			}
		}
		if !reuse && (state == "draft" || state == "sealed") {
			if err = cjsiOpenOne(tx.ExecContext(ctx, "UPDATE cjsi_publication_attempts SET state='retired' WHERE ticket=? AND attempt=? AND state=?", p.ticket, last, state)); err != nil {
				return err
			}
		}
		if !reuse && !usageOnly {
			if _, err = rand.Read(p.expected.identity[:]); err != nil {
				return err
			}
		}
		a := cjsiSlotAdmission{m.path, m.root, m.fence, p}
		if reuse {
			if !usageOnly {
				if err = checkCJSIPrepareBase(ctx, tx, a); err != nil {
					return err
				}
			}
		} else if usageOnly {
			_, err = tx.ExecContext(ctx, `INSERT INTO cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,index_epoch,session_revision,usage_revision,user_revision,route_revision,price_revision,protection_revision,coverage_revision,input_digest,slot_identity,accept_until,state)
   VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'draft')`, p.ticket, p.attempt, h.Ticket, h.CoreEpoch, h.IndexEpoch, h.SessionRevision, p.usageRevision, p.userRevision, p.routeRevision, p.priceRevision, h.ProtectionRevision, h.CoverageRevision, p.input[:], nil, p.acceptUntil.Format(time.RFC3339Nano))
		} else {
			err = reserveCJSICombinedDraft(ctx, tx, a, func(context.Context, cjsiRowQueryer) (cjsiPrepareProof, error) {
				return cjsiPrepareProof{admission: a}, nil
			})
		}
		if err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		d = CJSIPublicationDraft{root: m, election: election, admission: a, usageOnly: usageOnly, slotValidUntil: slotEnd, validate: inputs.Validate}
		return nil
	})
	return d, err
}

type CJSIIndexPlan interface {
	Seal() [32]byte
	Stage(context.Context, *sql.Tx, CJSIIndexStamp, int64, int64, int64, [32]byte) error
}

func (d CJSIPublicationDraft) next(seal [32]byte, rev, coverage int64) CJSIIndexStamp {
	p := d.admission.proposal
	h := p.base
	h.BaseTicket, h.BaseAttempt, h.BaseRevision, h.BaseSeal = h.SessionTicket, h.SessionAttempt, h.SessionRevision, h.SessionSeal
	h.Ticket, h.SessionTicket, h.SessionAttempt = p.ticket, p.ticket, p.attempt
	h.SessionRevision, h.SessionSeal, h.CoverageRevision = rev, seal, coverage
	return h
}

// Verify the authenticated withdrawal manifest against the same compacted base
// that supplied stable document IDs. Private edits invalidate the whole plan.
func (d CJSIPublicationDraft) checkIndex(ctx context.Context, tx *sql.Tx, h CJSIIndexStamp, removed []CJSISourceWithdrawal) error {
	b := d.Parent()
	var one int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM cjsi_session_candidates c JOIN cjsi_index_state s ON s.singleton=1
 JOIN serving_generation g USING(singleton) JOIN session_index_generation i USING(singleton)
 WHERE c.ticket=? AND c.attempt=? AND c.parent_ticket=? AND c.core_epoch=? AND c.index_epoch=i.epoch AND i.epoch=?
 AND c.base_ticket=s.compacted_ticket AND s.compacted_ticket=? AND c.base_attempt=s.compacted_attempt AND s.compacted_attempt=?
 AND c.base_seal=s.compacted_seal AND s.compacted_seal=? AND c.base_revision=s.base_revision AND s.base_revision=g.revision AND g.revision=?
 AND c.protection_revision=s.protection_revision AND s.protection_revision=? AND c.coverage_revision=? AND c.session_revision=? AND c.seal_digest=?`,
		h.Ticket, h.SessionAttempt, b.Ticket, b.CoreEpoch, b.IndexEpoch, b.SessionTicket, b.SessionAttempt, b.SessionSeal[:], b.SessionRevision, b.ProtectionRevision, b.CoverageRevision, h.SessionRevision, h.SessionSeal[:]).Scan(&one)
	if err != nil {
		return errors.Join(ErrCJSIIndexChanged, err)
	}
	return verifyCJSIWithdrawals(ctx, tx, h.Ticket, h.SessionAttempt, removed)
}

// StageIndex is idempotent for an independently reconstructed, identical
// candidate. A private seal on its own cannot authorize the public head.
func (d CJSIPublicationDraft) StageIndex(ctx context.Context, plan CJSIIndexPlan, revision int64, removed []CJSISourceWithdrawal) error {
	if d.usageOnly || plan == nil || plan.Seal() == ([32]byte{}) || removed == nil {
		return ErrCJSIIndexChanged
	}
	h := d.next(plan.Seal(), revision, d.Parent().CoverageRevision)
	return d.root.withPublicationLocks(ctx, d.election, func() (err error) {
		db, err := d.root.runner().core(ctx, false)
		if err != nil {
			return err
		}
		defer db.Close()
		if err = d.checkDraft(ctx, db); err != nil {
			return err
		}
		index, err := d.root.openPublicationIndex(ctx, d.Parent(), true)
		if err != nil {
			return err
		}
		defer index.Close()
		tx, err := index.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var prior []byte
		err = tx.QueryRowContext(ctx, "SELECT seal_digest FROM cjsi_session_candidates WHERE ticket=? AND attempt=?", h.Ticket, h.SessionAttempt).Scan(&prior)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			err = plan.Stage(ctx, tx, d.Parent(), h.Ticket, h.SessionAttempt, revision, h.SessionSeal)
		case err == nil && !bytes.Equal(prior, h.SessionSeal[:]):
			err = ErrCJSIIndexChanged
		}
		if err != nil {
			return err
		}
		if err = d.checkIndex(ctx, tx, h, removed); err != nil {
			return err
		}
		return tx.Commit()
	})
}
func (d CJSIPublicationDraft) checkDraft(ctx context.Context, q cjsiRowQueryer) error {
	p := d.admission.proposal
	if !time.Now().Before(p.acceptUntil) {
		return context.DeadlineExceeded
	}
	h, err := cjsiReadIndexHead(ctx, q)
	if err != nil {
		return err
	}
	h.Device, h.Inode = p.base.Device, p.base.Inode
	if h != p.base {
		return ErrCJSIIndexChanged
	}
	var identity any
	if !d.usageOnly {
		identity = p.expected.identity[:]
	}
	var one int
	if tx, ok := q.(*sql.Tx); ok && d.validate != nil {
		if err := d.validate(ctx, tx); err != nil {
			return err
		}
	}
	return q.QueryRowContext(ctx, `SELECT 1 FROM cjsi_publication_attempts WHERE ticket=? AND attempt=? AND input_digest=? AND slot_identity IS ? AND state IN ('draft','sealed')`, p.ticket, p.attempt, p.input[:], identity).Scan(&one)
}

// WithProposedView lends the selected private delta and the *same* core
// baseline to the reducer. The caller overlays the immutable domain delta
// in memory, then serializes only after this callback returns.
func (d CJSIPublicationDraft) WithProposedView(ctx context.Context, seal [32]byte, rev, coverage int64, removed []CJSISourceWithdrawal, read func(*sql.Tx, CJSIIndexSnapshot) error) error {
	if read == nil {
		return ErrCJSIIndexChanged
	}
	return d.WithStagedViews(ctx, seal, rev, coverage, removed, func(tx *sql.Tx, _, candidate CJSIIndexSnapshot) error { return read(tx, candidate) })
}
func (d CJSIPublicationDraft) WithStagedViews(ctx context.Context, seal [32]byte, rev, coverage int64, removed []CJSISourceWithdrawal, read func(*sql.Tx, CJSIIndexSnapshot, CJSIIndexSnapshot) error) error {
	if d.usageOnly || read == nil || removed == nil {
		return ErrCJSIIndexChanged
	}
	return d.root.WithPublishedCore(ctx, func(core *sql.Tx, _ CJSIIndexStamp) error {
		if err := d.checkDraft(ctx, core); err != nil {
			return err
		}
		db, err := d.root.openPublicationIndex(ctx, d.Parent(), false)
		if err != nil {
			return err
		}
		defer db.Close()
		tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return err
		}
		defer tx.Rollback()
		h := d.next(seal, rev, coverage)
		if err = d.checkIndex(ctx, tx, h, removed); err != nil {
			return err
		}
		return read(core, CJSIIndexSnapshot{Tx: tx, stamp: d.Parent()}, CJSIIndexSnapshot{Tx: tx, stamp: h, delta: true})
	})
}

type CJSICombinedChanges struct {
	CheckSourceWalls func(context.Context) error
	// Digest commits to all old images, row operations and target revisions.
	Digest                                           [32]byte
	Groups                                           []cjsiledger.Eligibility
	Validate                                         func(context.Context, *sql.Tx) error
	RecheckSources                                   func(context.Context) error
	ApplySelected                                    func(context.Context, *sql.Tx) error
	VerifyOpened                                     func(context.Context, *sql.Tx) error
	SessionRevision, UsageRevision, CoverageRevision int64
}
type preparedCombinedChanges struct {
	p      cjsiSlotProposal
	next   cjsiOpenTarget
	change CJSICombinedChanges
}

func (c preparedCombinedChanges) binding() (cjsiSlotProposal, cjsiOpenTarget) { return c.p, c.next }
func (c preparedCombinedChanges) apply(ctx context.Context, tx *sql.Tx) error {
	return c.change.Validate(ctx, tx)
}
func (c preparedCombinedChanges) stampSelected(ctx context.Context, tx *sql.Tx) error {
	return c.change.ApplySelected(ctx, tx)
}
func (c preparedCombinedChanges) verify(ctx context.Context, q cjsiRowQueryer) error {
	tx, ok := q.(*sql.Tx)
	if !ok {
		return ErrCJSIIndexChanged
	}
	return c.change.VerifyOpened(ctx, tx)
}
func (c preparedCombinedChanges) beforeSelectionRevision() int64 { return c.p.usageRevision }

// StageUsage admits the domain owner's real Stage transactions to this
// reservation and the current worker election. The owner releases these
// transactions before rendering or file I/O.
func (d CJSIPublicationDraft) StageUsage(ctx context.Context, stage func(*sql.DB) error) error {
	if stage == nil {
		return ErrTelemetrySourceIdentityPending
	}
	return d.root.withPublicationLocks(ctx, d.election, func() (err error) {
		db, err := d.root.runner().core(ctx, true)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, db.Close()) }()
		if err = d.checkDraft(ctx, db); err != nil {
			return err
		}
		var frozen bool
		if err = db.QueryRowContext(ctx, "SELECT diff_digest IS NOT NULL FROM cjsi_publication_attempts WHERE ticket=? AND attempt=?", d.Basis().Ticket, d.Basis().Attempt).Scan(&frozen); err != nil {
			return err
		}
		if frozen {
			return nil
		}
		return stage(db)
	})
}

func (d CJSIPublicationDraft) pinPrivate(ctx context.Context, h CJSIIndexStamp, removed []CJSISourceWithdrawal) (func() error, error) {
	index, err := d.root.openPublicationIndex(ctx, d.Parent(), true)
	if err != nil {
		return nil, err
	}
	it, err := index.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.Join(err, index.Close())
	}
	release := func() error { return errors.Join(it.Rollback(), index.Close()) }
	if err = d.checkIndex(ctx, it, h, removed); err != nil {
		return nil, errors.Join(err, release())
	}
	return release, nil
}

// CJSIFrozenPublication binds the private index, domain operations and
// expected postconditions before any reducer or encoder runs.
type CJSIFrozenPublication struct {
	draft    CJSIPublicationDraft
	proposal cjsiSlotProposal
	head     CJSIIndexStamp
	removed  []CJSISourceWithdrawal
	groups   map[string]cjsiStagedGroup
	changes  CJSICombinedChanges
}

func (d CJSIPublicationDraft) FreezePublication(ctx context.Context, seal [32]byte, removed []CJSISourceWithdrawal, c CJSICombinedChanges) (*CJSIFrozenPublication, error) {
	if d.usageOnly || c.Digest == ([32]byte{}) || seal == ([32]byte{}) || removed == nil || c.Groups == nil || c.Validate == nil || c.ApplySelected == nil || c.VerifyOpened == nil || c.RecheckSources == nil {
		return nil, ErrTelemetrySourceIdentityPending
	}
	p := d.admission.proposal
	groups := make(map[string]cjsiStagedGroup, len(c.Groups))
	stamps := make([]cjsiledger.ProposalStamp, 0, len(c.Groups))
	for _, g := range c.Groups {
		if _, dup := groups[g.Thread]; dup || g.Thread == "" || g.Stamp.Basis != d.Basis() {
			return nil, errCJSISlotAdmission
		}
		groups[g.Thread] = cjsiStagedGroup{g.Stamp, g.State, g.Gaps, g.Pending, g.Partial, g.UnresolvedOverlap}
		stamps = append(stamps, g.Stamp)
	}
	// The ordered group manifest is part of the prepared plan.
	encoded, err := json.Marshal(stamps)
	if err != nil {
		return nil, err
	}
	p.usageMaterial = sha256.Sum256(encoded)
	p.sessionSeal = seal
	aggregate, err := json.Marshal(struct {
		Domain                                           string
		Change, Session, Usage                           [32]byte
		Removed                                          []CJSISourceWithdrawal
		SessionRevision, UsageRevision, CoverageRevision int64
	}{"agentdeck/cjsi/publication-delta/v1", c.Digest, seal, p.usageMaterial, removed, c.SessionRevision, c.UsageRevision, c.CoverageRevision})
	if err != nil {
		return nil, err
	}
	p.diff = sha256.Sum256(aggregate)
	h := d.next(seal, c.SessionRevision, c.CoverageRevision)
	a := d.admission
	a.proposal = p
	if err = c.RecheckSources(ctx); err != nil {
		return nil, err
	}
	err = d.root.withPublicationLocks(ctx, d.election, func() error {
		release, err := d.pinPrivate(ctx, h, removed)
		if err != nil {
			return err
		}
		defer release()
		core, err := d.root.runner().core(ctx, true)
		if err != nil {
			return err
		}
		defer core.Close()
		tx, err := core.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err = d.checkDraft(ctx, tx); err != nil {
			return err
		}
		if err = c.Validate(ctx, tx); err != nil {
			return err
		}
		var state string
		err = tx.QueryRowContext(ctx, "SELECT state FROM cjsi_publication_attempts WHERE ticket=? AND attempt=?", p.ticket, p.attempt).Scan(&state)
		if err != nil {
			return err
		}
		if state == "sealed" {
			var one int
			if err := tx.QueryRowContext(ctx, "SELECT 1 FROM cjsi_publication_attempts WHERE ticket=? AND attempt=? AND diff_digest=? AND session_seal=?", p.ticket, p.attempt, p.diff[:], seal[:]).Scan(&one); err != nil {
				return err
			}
			return checkCJSIStagedGroups(ctx, tx, p, groups)
		}
		if err = freezeCJSICombinedDraft(ctx, tx, a, func(context.Context, cjsiRowQueryer) (cjsiPrepareProof, error) {
			return cjsiPrepareProof{admission: a, session: cjsiPrepareSession{p.base, p.ticket, p.attempt, p.sessionSeal}, groups: groups}, nil
		}); err != nil {
			return err
		}
		return tx.Commit()
	})
	if err != nil {
		return nil, err
	}
	return &CJSIFrozenPublication{d, p, h, slices.Clone(removed), groups, c}, nil
}

// DiffDigest names the frozen attempt including both component seals and the domain delta.
func (f *CJSIFrozenPublication) DiffDigest() [32]byte { return f.proposal.diff }

// Open revalidates or adopts a durable private slot and performs the final CAS.
func (f *CJSIFrozenPublication) Open(ctx context.Context, contents []byte, built, validUntil time.Time) error {
	if bound := f.draft.SlotValidUntil(); !bound.IsZero() && !validUntil.Equal(bound) {
		return errCJSISlotAdmission
	}
	d, p, h, removed, groups, c := f.draft, f.proposal, f.head, f.removed, f.groups, f.changes
	a := d.admission
	p.expected.digest = sha256.Sum256(contents)
	p.expected.size = int64(len(contents))
	p.expected.format = 1
	p.expected.builtAt = built.UTC()
	p.expected.validUntil = validUntil.UTC()
	a.proposal = p
	target := cjsiOpenTarget{h, c.UsageRevision}
	r := d.root.runner()
	r.admission = a
	r.slotIO = publicationSlotIO
	r.qualify = func(ctx context.Context, _, _ *os.File) (cjsiRunnerProof, error) {
		if d.election != nil {
			if err := d.election(); err != nil {
				return cjsiRunnerProof{}, err
			}
		}
		if c.CheckSourceWalls != nil {
			if e := c.CheckSourceWalls(ctx); e != nil {
				return cjsiRunnerProof{}, e
			}
		}
		release, err := d.pinPrivate(ctx, h, removed)
		if err != nil {
			return cjsiRunnerProof{}, err
		}
		return cjsiRunnerProof{release: release, target: target, delta: preparedCombinedChanges{p, target, c}, current: func(ctx context.Context, q cjsiRowQueryer) (cjsiSlotAdmission, error) {
			tx, ok := q.(*sql.Tx)
			if !ok {
				return a, errCJSISlotAdmission
			}
			if err := d.checkDraft(ctx, tx); err != nil {
				return a, err
			}
			if err := c.Validate(ctx, tx); err != nil {
				return a, err
			}
			return a, checkCJSIStagedGroups(ctx, tx, p, groups)
		}}, nil
	}
	// Source validation after fsync runs before either blocking writer fence.
	ioSlot := r.slotIO
	r.slotIO = func(ctx context.Context, root *os.File, info os.FileInfo, id [16]byte, b []byte, sealed bool) (cjsiRunnerSlot, error) {
		s, err := ioSlot(ctx, root, info, id, b, sealed)
		if err != nil {
			return s, err
		}
		if err = c.RecheckSources(ctx); err != nil {
			s.file.Close()
			return cjsiRunnerSlot{}, err
		}
		return s, nil
	}
	return r.run(ctx, contents)
}

func publicationSlotIO(ctx context.Context, root *os.File, info os.FileInfo, id [16]byte, data []byte, sealed bool) (cjsiRunnerSlot, error) {
	name := cjsiServingSlotName(id)
	s := cjsiRunnerSlot{name: name, identity: id, digest: sha256.Sum256(data), size: int64(len(data))}
	if !sealed {
		disk, err := persistReservedCJSISlot(ctx, root, info, id, data)
		if err != nil {
			return cjsiRunnerSlot{}, err
		}
		s.identity, s.digest, s.size = disk.Identity, disk.Digest, disk.Size
	}
	fd, err := unix.Openat(int(root.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return cjsiRunnerSlot{}, err
	}
	s.file = os.NewFile(uintptr(fd), name)
	return s, nil
}
