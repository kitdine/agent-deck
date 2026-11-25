package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"time"

	"github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// A header comes from the usage owner's independently selected Eligibility
// after CJSIUsageCandidate.Stage committed its complete typed material.
type cjsiStagedGroup struct {
	stamp                     cjsiledger.ProposalStamp
	state                     cjsiledger.State
	gaps                      cjsiledger.Gap
	pending, partial, overlap bool
}

// The session owner qualifies the sealed candidate against this independently
// pinned parent view outside core's transaction, under the shared fences.
type cjsiPrepareSession struct {
	parent          CJSIIndexStamp
	ticket, attempt int64
	seal            [32]byte
}

type cjsiPrepareProof struct {
	admission cjsiSlotAdmission
	session   cjsiPrepareSession
	groups    map[string]cjsiStagedGroup
}

// Resolve the current exclusive attempt under the coordinator's writer fences.
// input binds the original full scope, including preimages not stored as
// individual core columns. Reconstruct it from registered source/owner proof,
// never from the request or from an existing draft. The owner resolves displaced
// attempts and verifies their retirement before authorizing a new reservation.
//
// For freeze, return the complete, budgeted thread manifest from independent
// group selection and successful exact Stage calls. Keep their child tables,
// selected worksets and source preconditions fenced through freeze, or reprove
// them under the same admission. Supply the qualified same-attempt session
// seal and the canonical aggregate diff that commits to usageMaterial, planned
// operations, preconditions and postconditions. All expensive work precedes
// this callback; here use existing pins and bounded reads of the borrowed tx.
type cjsiSelectPrepareProof func(context.Context, cjsiRowQueryer) (cjsiPrepareProof, error)

func reserveCJSICombinedDraft(ctx context.Context, tx *sql.Tx, planned cjsiSlotAdmission, current cjsiSelectPrepareProof) error {
	return prepareCJSICombined(ctx, tx, planned, current, false)
}

func freezeCJSICombinedDraft(ctx context.Context, tx *sql.Tx, completed cjsiSlotAdmission, current cjsiSelectPrepareProof) error {
	return prepareCJSICombined(ctx, tx, completed, current, true)
}

// Each step borrows its own short, dedicated, already admitted core writer.
// Success leaves final deadline/cancellation check and commit to its owner.
// Failure, including panic, rolls back the whole transaction and cannot leave
// a partially reserved/frozen draft committable.
func prepareCJSICombined(ctx context.Context, tx *sql.Tx, request cjsiSlotAdmission, current cjsiSelectPrepareProof, freeze bool) (err error) {
	if tx == nil {
		return errCJSISlotAdmission
	}
	success := false
	defer func() {
		if !success {
			if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
				err = errors.Join(err, rb)
			}
		}
	}()
	p := request.proposal.utc()
	request.proposal = p
	zero := [32]byte{}
	if current == nil || !filepath.IsAbs(request.rootPath) || filepath.Clean(request.rootPath) != request.rootPath ||
		request.root.inode == 0 || request.fence.inode == 0 || p.ticket <= p.base.Ticket || p.attempt <= 0 || p.base.Ticket < 0 ||
		p.base.CoreEpoch <= 0 || p.base.IndexEpoch <= 0 || p.base.Inode == 0 || p.input == zero ||
		p.usageRevision < 0 || p.userRevision < 0 || p.routeRevision < 0 || p.priceRevision < 0 ||
		p.expected.identity == ([16]byte{}) || p.expected != (cjsiSlotReceipt{identity: p.expected.identity}) ||
		p.acceptUntil.Year() > 9999 || !time.Now().Before(p.acceptUntil) ||
		(p.diff != zero) != freeze || (p.sessionSeal != zero) != freeze || (p.usageMaterial != zero) != freeze {
		return errCJSISlotAdmission
	}
	if err := cjsiCheckJournalDurability(ctx, tx); err != nil {
		return err
	}
	// Also force a deferred stale WAL snapshot to fail on exact retries (no
	// row changes). The normal caller acquires immediate writer admission.
	if _, err := tx.ExecContext(ctx, `UPDATE main.cjsi_publication_head SET ticket=ticket WHERE 0`); err != nil {
		return err
	}
	proof, err := current(ctx, tx)
	if err != nil {
		return err
	}
	proof.admission.proposal = proof.admission.proposal.utc()
	var qualified cjsiPrepareSession
	if freeze {
		qualified = cjsiPrepareSession{p.base, p.ticket, p.attempt, p.sessionSeal}
	}
	if proof.admission != request || proof.session != qualified || (proof.groups != nil) != freeze {
		return errCJSISlotAdmission
	}
	if err := checkCJSIPrepareBase(ctx, tx, request); err != nil {
		return err
	}
	if freeze {
		err = freezeCJSIStagedGroups(ctx, tx, p, proof.groups)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO main.cjsi_publication_attempts
			(ticket,attempt,base_ticket,core_epoch,index_epoch,session_revision,usage_revision,user_revision,route_revision,price_revision,
			 protection_revision,coverage_revision,input_digest,slot_identity,accept_until,state)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'draft') ON CONFLICT(ticket,attempt) DO NOTHING`,
			p.ticket, p.attempt, p.base.Ticket, p.base.CoreEpoch, p.base.IndexEpoch, p.base.SessionRevision, p.usageRevision, p.userRevision, p.routeRevision,
			p.priceRevision, p.base.ProtectionRevision, p.base.CoverageRevision, p.input[:], p.expected.identity[:], p.acceptUntil.Format(time.RFC3339Nano))
	}
	if err != nil {
		return err
	}
	diff, session, err := readCJSIPrepareDraft(ctx, tx, p)
	if err != nil {
		return err
	}
	if (freeze && (!bytes.Equal(diff, p.diff[:]) || !bytes.Equal(session, p.sessionSeal[:]))) ||
		(!freeze && (diff != nil || session != nil)) || !time.Now().Before(p.acceptUntil) {
		return errCJSISlotAdmission
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	success = true
	return nil
}

const cjsiPrepareIdentityConflict = `SELECT EXISTS(SELECT 1 FROM main.cjsi_publication_attempts INDEXED BY cjsi_publication_attempts_slot_identity
	WHERE slot_identity=? AND (ticket,attempt)<>(?,?))`

func checkCJSIPrepareBase(ctx context.Context, tx *sql.Tx, a cjsiSlotAdmission) error {
	p, b := a.proposal, a.proposal.base
	j, err := cjsiReadLayoutJournal(ctx, tx)
	if err != nil {
		return err
	}
	if j.phase != cjsiLayoutReady || j.root != a.root || j.fence != a.fence || j.epoch != b.IndexEpoch ||
		(b.BaseTicket == 0 && b.BaseSeal != j.seal.digest) {
		return errCJSISlotAdmission
	}
	head, err := cjsiReadIndexHead(ctx, tx) // includes live core epoch
	if err != nil {
		return err
	}
	head.Device, head.Inode = j.prepared.identity.device, j.prepared.identity.inode
	if head != b {
		return errCJSISlotAdmission
	}
	var priorSeal any
	if p.baseSlotSeal != ([32]byte{}) {
		priorSeal = p.baseSlotSeal[:]
	}
	var one int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM main.cjsi_publication_head h JOIN main.serving_generation g USING(singleton)
		JOIN main.schema_metadata m USING(singleton) WHERE h.singleton=1 AND m.version=37 AND g.schema_version=? AND g.revision=?
		AND h.slot_ticket=? AND h.slot_attempt=? AND h.slot_seal IS ?
		AND (SELECT file FROM pragma_database_list WHERE name='main')=? AND (SELECT foreign_keys FROM pragma_foreign_keys)=1`,
		ServingStorageVersion, p.usageRevision, p.baseSlotTicket, p.baseSlotAttempt, priorSeal, filepath.Join(a.rootPath, "agentdeck.sqlite3")).Scan(&one)
	if err != nil {
		return err
	}
	var conflict bool
	if err := tx.QueryRowContext(ctx, cjsiPrepareIdentityConflict, p.expected.identity[:], p.ticket, p.attempt).Scan(&conflict); err != nil {
		return err
	}
	if conflict {
		return errCJSISlotAdmission
	}
	return nil
}

func readCJSIPrepareDraft(ctx context.Context, tx *sql.Tx, p cjsiSlotProposal) (diff, session []byte, err error) {
	b := p.base
	var deadline string
	err = tx.QueryRowContext(ctx, `SELECT diff_digest,session_seal,accept_until FROM main.cjsi_publication_attempts
		WHERE ticket=? AND attempt=? AND state='draft' AND base_ticket=? AND core_epoch=? AND index_epoch=? AND session_revision=?
		AND usage_revision=? AND user_revision=? AND route_revision=? AND price_revision=? AND protection_revision=? AND coverage_revision=?
		AND input_digest=? AND slot_identity=? AND slot_seal IS NULL AND slot_digest IS NULL AND slot_size IS NULL AND slot_format IS NULL
		AND built_at IS NULL AND valid_until IS NULL`, p.ticket, p.attempt, b.Ticket, b.CoreEpoch, b.IndexEpoch, b.SessionRevision,
		p.usageRevision, p.userRevision, p.routeRevision, p.priceRevision, b.ProtectionRevision, b.CoverageRevision, p.input[:], p.expected.identity[:]).Scan(&diff, &session, &deadline)
	if err != nil {
		return nil, nil, err
	}
	until, err := time.Parse(time.RFC3339Nano, deadline)
	if err != nil || !until.Equal(p.acceptUntil) {
		return nil, nil, errCJSISlotAdmission
	}
	return diff, session, nil
}

func freezeCJSIStagedGroups(ctx context.Context, tx *sql.Tx, p cjsiSlotProposal, groups map[string]cjsiStagedGroup) error {
	diff, session, err := readCJSIPrepareDraft(ctx, tx, p)
	if err != nil {
		return err
	}
	retry := bytes.Equal(diff, p.diff[:]) && bytes.Equal(session, p.sessionSeal[:])
	if !retry && (diff != nil || session != nil) {
		return errCJSISlotAdmission
	}
	if err := checkCJSIStagedGroups(ctx, tx, p, groups); err != nil {
		return err
	}
	if retry {
		return nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE main.cjsi_publication_attempts SET diff_digest=?,session_seal=?
		WHERE ticket=? AND attempt=? AND state='draft' AND diff_digest IS NULL AND session_seal IS NULL`, p.diff[:], p.sessionSeal[:], p.ticket, p.attempt)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errCJSISlotAdmission
	}
	return nil
}

// Read the primary-key prefix, limited to the authenticated group's budgeted
// cardinality plus one. This detects extra rows without counting history.
// An explicitly attested empty set is distinct from a missing manifest.
func checkCJSIStagedGroups(ctx context.Context, tx *sql.Tx, p cjsiSlotProposal, groups map[string]cjsiStagedGroup) error {
	rows, err := tx.QueryContext(ctx, `SELECT g.thread,g.input_digest,g.diff_digest,g.state,g.gaps,g.pending,g.partial,g.unresolved_overlap,
		EXISTS(SELECT 1 FROM main.cjsi_usage_selection s WHERE s.thread=g.thread AND s.ticket=g.ticket AND s.attempt=g.attempt)
		FROM main.cjsi_usage_groups g WHERE g.ticket=? AND g.attempt=? ORDER BY g.thread LIMIT ?`, p.ticket, p.attempt, len(groups)+1)
	if err != nil {
		return err
	}
	defer rows.Close()
	basis := cjsiledger.Basis{Ticket: p.ticket, Attempt: p.attempt, BaseTicket: p.base.Ticket, CoreEpoch: p.base.CoreEpoch, InputDigest: p.input}
	count := 0
	for rows.Next() {
		var thread string
		var input, diff []byte
		var actual cjsiStagedGroup
		var public bool
		if err := rows.Scan(&thread, &input, &diff, &actual.state, &actual.gaps, &actual.pending, &actual.partial, &actual.overlap, &public); err != nil {
			return err
		}
		want, ok := groups[thread]
		if !ok || thread == "" || public || !bytes.Equal(input, p.input[:]) || len(diff) != 32 || want.stamp.Diff == ([32]byte{}) {
			return errCJSISlotAdmission
		}
		actual.stamp = cjsiledger.ProposalStamp{Basis: basis, Diff: [32]byte(diff)}
		if actual != want {
			return errCJSISlotAdmission
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != len(groups) {
		return errCJSISlotAdmission
	}
	return nil
}
