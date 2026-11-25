package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"time"
)

var errCJSIOpenChanged = errors.New("CJSI combined publication proof changed")

// The target is part of the aggregate diff. session is the qualified proposed
// component paired with the expected public coverage stamp. Its physical base
// may be the current base or the exactly compacted previous session component.
type cjsiOpenTarget struct {
	session       CJSIIndexStamp
	usageRevision int64
}

func (n cjsiOpenTarget) validFor(p cjsiSlotProposal) bool {
	s, b := n.session, p.base
	sameBase := s.BaseTicket == b.BaseTicket && s.BaseAttempt == b.BaseAttempt && s.BaseSeal == b.BaseSeal && s.BaseRevision == b.BaseRevision
	compacted := s.BaseTicket == b.SessionTicket && s.BaseAttempt == b.SessionAttempt && s.BaseSeal == b.SessionSeal && s.BaseRevision == b.SessionRevision
	return (sameBase || compacted) && s.Ticket == p.ticket && s.SessionTicket == p.ticket && s.SessionAttempt == p.attempt &&
		s.SessionSeal == p.sessionSeal && s.CoreEpoch == b.CoreEpoch && s.IndexEpoch == b.IndexEpoch && s.Device == b.Device && s.Inode == b.Inode &&
		s.ProtectionRevision == b.ProtectionRevision && s.CoverageRevision >= b.CoverageRevision &&
		s.SessionRevision >= b.SessionRevision && n.usageRevision >= p.usageRevision &&
		(s.SessionRevision != b.SessionRevision || n.usageRevision != p.usageRevision || s.CoverageRevision != b.CoverageRevision)
}

// The adapter owns an immutable, already prepared combined-domain plan.
// binding returns the ORIGINAL stamp and postconditions from that plan. Its
// diff commits to the complete affected keys, old images, operations and
// target revisions/coverage. Never label a worker's plan with the newly
// selected stamp. All methods are bounded to those keys.
//
// apply must recheck the still-current domain revisions and old images under
// the shared writer fences, save takeover receipts before withdrawing facts,
// and apply exactly the bound usage/OTel, selection, binding and scoped
// coverage changes with their real triggers. stampSelected writes only the
// plan's invalidations that require the newly selected ticket. verify reads
// the actual postimages, receipts, affected scope and stamp rows, including
// required absences. An acknowledgment or the global revision alone is
// insufficient. None of these methods owns commit, the attempt or the head.
type cjsiOpenDelta interface {
	binding() (cjsiSlotProposal, cjsiOpenTarget)
	apply(context.Context, *sql.Tx) error
	stampSelected(context.Context, *sql.Tx) error
	verify(context.Context, cjsiRowQueryer) error
}

type cjsiCombinedOpen struct {
	admission cjsiSlotAdmission
	next      cjsiOpenTarget
	slotSeal  [32]byte
	delta     cjsiOpenDelta
}

// Bind BEFORE the write transaction. The independent coordinator has checked
// the actual source capture and all writer fences, uniqueness/lifecycle of
// the reserved slot, the pinned file/bytes and the proposed session candidate
// (including parent, physical base and seal). It holds these pins/fences until
// commit. The target and actual receipt come from that qualification.
func bindCJSICombinedOpen(a cjsiSlotAdmission, disk cjsiSlotReceipt, target cjsiOpenTarget, delta cjsiOpenDelta) (cjsiCombinedOpen, error) {
	if delta == nil || !filepath.IsAbs(a.rootPath) || filepath.Clean(a.rootPath) != a.rootPath || a.root.inode == 0 || a.fence.inode == 0 {
		return cjsiCombinedOpen{}, errCJSIOpenChanged
	}
	p, expected := delta.binding()
	a.proposal = a.proposal.utc()
	if p.utc() != a.proposal || disk.utc() != a.proposal.expected || expected != target || !target.validFor(a.proposal) {
		return cjsiCombinedOpen{}, errCJSIOpenChanged
	}
	return cjsiCombinedOpen{a, target, a.proposal.seal(), delta}, nil
}

// Borrow a fresh dedicated core write transaction. On success the coordinator
// checks cancellation/liveness and commits immediately while still fenced.
// An error or panic terminates the ENTIRE transaction, even after domain rows
// have changed. An unknown commit outcome goes to confirmCommitted on a fresh
// committed read snapshot; it must never cause a blind replay.
func (o cjsiCombinedOpen) open(ctx context.Context, tx *sql.Tx) (err error) {
	if tx == nil {
		return errCJSIOpenChanged
	}
	ready := false
	defer func() {
		if !ready {
			if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
				err = errors.Join(err, rb)
			}
		}
	}()
	p := o.admission.proposal
	if o.delta == nil || !p.valid() {
		return errCJSIOpenChanged
	}
	if err := cjsiCheckJournalDurability(ctx, tx); err != nil {
		return err
	}
	// Promote a deferred transaction before admitting any owner writes. This
	// touches no rows and a stale WAL snapshot fails its write upgrade.
	if _, err := tx.ExecContext(ctx, `UPDATE main.cjsi_publication_head SET ticket=ticket WHERE 0`); err != nil {
		return err
	}
	if err := o.checkRoot(ctx, tx); err != nil {
		return err
	}
	if err := o.checkAttempt(ctx, tx, "sealed"); err != nil {
		return err
	}
	if err := o.checkHead(ctx, tx, p.base, p.baseSlotTicket, p.baseSlotAttempt, p.baseSlotSeal, p.usageRevision); err != nil {
		return err
	}
	if !p.live(time.Now()) {
		return errCJSIOpenChanged
	}
	if err := o.delta.apply(ctx, tx); err != nil {
		return err
	}
	// Domain writes and their triggers may advance the usage revision, but
	// may not replace the original public head or its reservation.
	priorRevision := o.next.usageRevision
	if deferred, ok := o.delta.(interface{ beforeSelectionRevision() int64 }); ok {
		priorRevision = deferred.beforeSelectionRevision()
	}
	if err := o.checkHead(ctx, tx, p.base, p.baseSlotTicket, p.baseSlotAttempt, p.baseSlotSeal, priorRevision); err != nil {
		return err
	}
	if !p.live(time.Now()) {
		return errCJSIOpenChanged
	}
	s := o.next.session
	// checkHead just compared the entire original head under this writer.
	// Only this one statement replaces both components and the physical base.
	if err := cjsiOpenOne(tx.ExecContext(ctx, `UPDATE main.cjsi_publication_head SET ticket=?,session_ticket=?,session_attempt=?,session_seal=?,session_revision=?,
		base_ticket=?,base_attempt=?,base_seal=?,base_revision=?,slot_ticket=?,slot_attempt=?,slot_seal=?,coverage_revision=?
		WHERE singleton=1 AND ticket=? AND core_epoch=? AND index_epoch=? AND protection_revision=? AND coverage_revision=?`,
		p.ticket, p.ticket, p.attempt, p.sessionSeal[:], s.SessionRevision, s.BaseTicket, s.BaseAttempt, s.BaseSeal[:], s.BaseRevision,
		p.ticket, p.attempt, o.slotSeal[:], s.CoverageRevision, p.base.Ticket, p.base.CoreEpoch, p.base.IndexEpoch, p.base.ProtectionRevision, p.base.CoverageRevision)); err != nil {
		return err
	}
	if err := o.delta.stampSelected(ctx, tx); err != nil {
		return err
	}
	if err := cjsiOpenOne(tx.ExecContext(ctx, `UPDATE main.cjsi_publication_attempts SET state='opened'
		WHERE ticket=? AND attempt=? AND state='sealed' AND slot_seal=? AND session_seal=? AND diff_digest=?`,
		p.ticket, p.attempt, o.slotSeal[:], p.sessionSeal[:], p.diff[:])); err != nil {
		return err
	}
	// Includes independent readback of the final writes and trigger effects.
	if err := o.verifyOpened(ctx, tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !p.live(time.Now()) {
		return errCJSIOpenChanged
	}
	ready = true
	return nil
}

func cjsiOpenOne(r sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errCJSIOpenChanged
	}
	return nil
}

// After resolving the writer's commit, supply an independently admitted
// pool for the pinned core. Always start a fresh read snapshot here: an
// uncommitted writer's self-check cannot certify a committed publication.
// A mismatch (including a superseding head) requires receipt reconciliation.
// Confirmation can succeed after expiry and never re-applies the delta.
func (o cjsiCombinedOpen) confirmCommitted(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errCJSIOpenChanged
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return o.verifyOpened(ctx, tx)
}

func (o cjsiCombinedOpen) verifyOpened(ctx context.Context, tx *sql.Tx) error {
	if tx == nil || o.delta == nil {
		return errCJSIOpenChanged
	}
	if err := o.checkRoot(ctx, tx); err != nil {
		return err
	}
	if err := o.checkAttempt(ctx, tx, "opened"); err != nil {
		return err
	}
	p := o.admission.proposal
	if err := o.checkHead(ctx, tx, o.next.session, p.ticket, p.attempt, o.slotSeal, o.next.usageRevision); err != nil {
		return err
	}
	return o.delta.verify(ctx, tx)
}

func (o cjsiCombinedOpen) checkRoot(ctx context.Context, tx *sql.Tx) error {
	a := o.admission
	j, err := cjsiReadLayoutJournal(ctx, tx)
	if err != nil {
		return err
	}
	b := a.proposal.base
	if j.phase != cjsiLayoutReady || j.root != a.root || j.fence != a.fence || j.epoch != b.IndexEpoch ||
		j.prepared.identity != (cjsiFileIdentity{b.Device, b.Inode}) || (b.BaseTicket == 0 && b.BaseSeal != j.seal.digest) {
		return errCJSIOpenChanged
	}
	var ok int
	return tx.QueryRowContext(ctx, `SELECT 1 FROM main.schema_metadata WHERE singleton=1 AND version=37
		AND (SELECT file FROM pragma_database_list WHERE name='main')=?
		AND (SELECT foreign_keys FROM pragma_foreign_keys)=1`, filepath.Join(a.rootPath, "agentdeck.sqlite3")).Scan(&ok)
}

func (o cjsiCombinedOpen) checkHead(ctx context.Context, tx *sql.Tx, want CJSIIndexStamp, slotTicket, slotAttempt int64, slotSeal [32]byte, usage int64) error {
	head, err := cjsiReadIndexHead(ctx, tx) // also checks the live core generation epoch
	if err != nil {
		return err
	}
	// cjsiReadIndexHead supplies the logical fields; checkRoot binds the
	// physical fields from the pinned view to the managed layout journal.
	head.Device, head.Inode = o.admission.proposal.base.Device, o.admission.proposal.base.Inode
	if head != want {
		return errCJSIOpenChanged
	}
	var seal any
	if slotSeal != ([32]byte{}) {
		seal = slotSeal[:]
	}
	var one int
	return tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_publication_head h JOIN main.serving_generation g USING(singleton)
		WHERE h.singleton=1 AND h.slot_ticket=? AND h.slot_attempt=? AND h.slot_seal IS ? AND g.schema_version=? AND g.revision=?`,
		slotTicket, slotAttempt, seal, ServingStorageVersion, usage).Scan(&one)
}

func (o cjsiCombinedOpen) checkAttempt(ctx context.Context, tx *sql.Tx, state string) error {
	p := o.admission.proposal
	r, b := p.expected, p.base
	var deadline string
	err := tx.QueryRowContext(ctx, `SELECT accept_until FROM main.cjsi_publication_attempts
		WHERE ticket=? AND attempt=? AND state=? AND base_ticket=? AND core_epoch=? AND index_epoch=?
		AND session_revision=? AND usage_revision=? AND user_revision=? AND route_revision=? AND price_revision=?
		AND protection_revision=? AND coverage_revision=? AND input_digest=? AND diff_digest=? AND session_seal=?
		AND slot_seal=? AND slot_identity=? AND slot_digest=? AND slot_size=? AND slot_format=? AND built_at=? AND valid_until=?`,
		p.ticket, p.attempt, state, b.Ticket, b.CoreEpoch, b.IndexEpoch, b.SessionRevision, p.usageRevision, p.userRevision, p.routeRevision, p.priceRevision,
		b.ProtectionRevision, b.CoverageRevision, p.input[:], p.diff[:], p.sessionSeal[:], o.slotSeal[:], r.identity[:], r.digest[:], r.size, r.format,
		r.builtAt.Format(time.RFC3339Nano), r.validUntil.Format(time.RFC3339Nano)).Scan(&deadline)
	if err != nil {
		return err
	}
	until, err := time.Parse(time.RFC3339Nano, deadline)
	if err != nil || !until.Equal(p.acceptUntil) {
		return errCJSIOpenChanged
	}
	return nil
}
