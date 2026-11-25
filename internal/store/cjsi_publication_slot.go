package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"path/filepath"
	"time"
)

var errCJSISlotAdmission = errors.New("CJSI private slot receipt does not match current admission")

type cjsiSlotReceipt struct {
	identity            [16]byte
	digest              [32]byte
	size, format        int64
	builtAt, validUntil time.Time
}

// Frozen before I/O. The aggregate diff binds final row operations, their
// preconditions and expected opened revisions/coverage. usageMaterial is the
// usage owner's authenticated commitment to the complete required staged set
// for this exact proposal; one group's diff or mere presence cannot supply it.
// sessionSeal comes from the independently sealed proposed-session view.
type cjsiSlotProposal struct {
	ticket, attempt                                           int64
	base                                                      CJSIIndexStamp
	baseSlotTicket, baseSlotAttempt                           int64
	baseSlotSeal                                              [32]byte
	usageRevision, userRevision, routeRevision, priceRevision int64
	input, diff, usageMaterial, sessionSeal                   [32]byte
	acceptUntil                                               time.Time
	expected                                                  cjsiSlotReceipt
}

// selectCurrent must resolve the currently authorized combined attempt from
// the coordinator under its writer fences, independently of the worker. Its
// callback uses only existing pins and bounded reads of the borrowed core tx.
// Its proof covers the still-valid session/usage bindings, user/route/price inputs,
// and the registered reservation and expected bytes/format/window. root and
// fence are identities from CURRENT managed admission; keep the physical root,
// core and actual authenticated slot pinned through this transaction.
type cjsiSlotAdmission struct {
	rootPath    string
	root, fence cjsiFileIdentity
	proposal    cjsiSlotProposal
}

type cjsiSelectSlotAdmission func(context.Context, cjsiRowQueryer) (cjsiSlotAdmission, error)

func (r cjsiSlotReceipt) utc() cjsiSlotReceipt {
	r.builtAt, r.validUntil = r.builtAt.UTC(), r.validUntil.UTC()
	return r
}

func (p cjsiSlotProposal) utc() cjsiSlotProposal {
	p.acceptUntil, p.expected = p.acceptUntil.UTC(), p.expected.utc()
	return p
}

func (p cjsiSlotProposal) live(now time.Time) bool {
	return !now.Before(p.expected.builtAt) && now.Before(p.expected.validUntil) && now.Before(p.acceptUntil)
}

func (p cjsiSlotProposal) valid() bool {
	r := p.expected
	zero := [32]byte{}
	return p.ticket > p.base.Ticket && p.base.Ticket >= 0 && p.attempt > 0 &&
		p.base.CoreEpoch > 0 && p.base.IndexEpoch > 0 && p.base.Inode != 0 &&
		p.usageRevision >= 0 && p.userRevision >= 0 && p.routeRevision >= 0 && p.priceRevision >= 0 &&
		p.input != zero && p.diff != zero && p.sessionSeal != zero && p.usageMaterial != zero &&
		r.identity != ([16]byte{}) && r.digest != zero && r.size > 0 && r.size <= 8<<20 && r.format > 0 &&
		!r.builtAt.IsZero() && r.builtAt.Year() >= 0 && r.validUntil.Year() <= 9999 &&
		p.acceptUntil.Year() <= 9999 && p.live(time.Now())
}

// Versioned domain, fixed-width big-endian integers/digests, UTC instants
// encoded as signed Unix seconds and nanoseconds. No Go struct layout, time
// zone spelling, SQL text or map iteration participates in the seal.
func (p cjsiSlotProposal) seal() [32]byte {
	b, r := p.base, p.expected
	data := []byte("agentdeck/cjsi/combined-slot-seal/v1\x00")
	for _, n := range []int64{p.ticket, p.attempt, b.Ticket, b.CoreEpoch, b.IndexEpoch,
		b.BaseTicket, b.BaseAttempt, b.BaseRevision, b.SessionTicket, b.SessionAttempt, b.SessionRevision,
		b.ProtectionRevision, b.CoverageRevision, p.baseSlotTicket, p.baseSlotAttempt,
		p.usageRevision, p.userRevision, p.routeRevision, p.priceRevision, r.size, r.format} {
		data = binary.BigEndian.AppendUint64(data, uint64(n))
	}
	data = binary.BigEndian.AppendUint64(data, b.Device)
	data = binary.BigEndian.AppendUint64(data, b.Inode)
	for _, d := range [][32]byte{b.BaseSeal, b.SessionSeal, p.baseSlotSeal, p.input, p.diff, p.usageMaterial, p.sessionSeal, r.digest} {
		data = append(data, d[:]...)
	}
	data = append(data, r.identity[:]...)
	for _, t := range []time.Time{p.acceptUntil, r.builtAt, r.validUntil} {
		data = binary.BigEndian.AppendUint64(data, uint64(t.Unix()))
		data = binary.BigEndian.AppendUint32(data, uint32(t.Nanosecond()))
	}
	return sha256.Sum256(data)
}

// Borrow a short, dedicated core write transaction after disk I/O. The caller
// rolls back every error (including cancellation or expiry before commit).
// A returned digest identifies a private seal only after that commit succeeds.
func registerCJSISlotReceipt(ctx context.Context, tx *sql.Tx, built cjsiSlotProposal, disk cjsiSlotReceipt, selectCurrent cjsiSelectSlotAdmission) ([32]byte, error) {
	built = built.utc()
	if tx == nil || selectCurrent == nil || !built.valid() || disk.utc() != built.expected {
		return [32]byte{}, errCJSISlotAdmission
	}
	admit, err := selectCurrent(ctx, tx)
	if err != nil {
		return [32]byte{}, err
	}
	if admit.proposal.utc() != built || !filepath.IsAbs(admit.rootPath) || filepath.Clean(admit.rootPath) != admit.rootPath {
		return [32]byte{}, errCJSISlotAdmission
	}
	if err := cjsiCheckJournalDurability(ctx, tx); err != nil {
		return [32]byte{}, err
	}
	j, err := cjsiReadLayoutJournal(ctx, tx)
	if err != nil {
		return [32]byte{}, err
	}
	if j.phase != cjsiLayoutReady || j.root != admit.root || j.fence != admit.fence || j.epoch != built.base.IndexEpoch {
		return [32]byte{}, errCJSISlotAdmission
	}
	head, err := cjsiReadIndexHead(ctx, tx)
	if err != nil {
		return [32]byte{}, err
	}
	head.Device, head.Inode = j.prepared.identity.device, j.prepared.identity.inode
	if head != built.base || (head.BaseTicket == 0 && head.BaseSeal != j.seal.digest) {
		return [32]byte{}, errCJSISlotAdmission
	}
	return built.register(ctx, tx, filepath.Join(admit.rootPath, "agentdeck.sqlite3"))
}

func (p cjsiSlotProposal) register(ctx context.Context, tx *sql.Tx, corePath string) ([32]byte, error) {
	r, b, seal := p.expected, p.base, p.seal()
	builtAt, validUntil := r.builtAt.Format(time.RFC3339Nano), r.validUntil.Format(time.RFC3339Nano)
	var priorSlotSeal any
	if p.baseSlotSeal != ([32]byte{}) {
		priorSlotSeal = p.baseSlotSeal[:]
	}
	var state, deadline string
	var empty, exact bool
	err := tx.QueryRowContext(ctx, `SELECT a.state, a.accept_until,
		(a.slot_seal IS NULL AND a.slot_digest IS NULL AND a.slot_size IS NULL AND a.slot_format IS NULL AND a.built_at IS NULL AND a.valid_until IS NULL),
		(a.slot_seal IS ? AND a.slot_digest IS ? AND a.slot_size IS ? AND a.slot_format IS ? AND a.built_at IS ? AND a.valid_until IS ?)
		FROM main.cjsi_publication_attempts a
		JOIN main.cjsi_publication_head h ON h.singleton=1
		JOIN main.serving_generation g ON g.singleton=1 AND g.schema_version=? AND g.revision=a.usage_revision
		JOIN main.schema_metadata m ON m.singleton=1 AND m.version=37
		WHERE a.ticket=? AND a.attempt=? AND a.state IN ('draft','sealed')
		AND a.base_ticket=? AND a.core_epoch=? AND a.index_epoch=? AND a.session_revision=?
		AND a.protection_revision=? AND a.coverage_revision=? AND a.usage_revision=? AND a.user_revision=? AND a.route_revision=? AND a.price_revision=?
		AND a.input_digest=? AND a.diff_digest=? AND a.session_seal=? AND a.slot_identity=?
		AND h.slot_ticket=? AND h.slot_attempt=? AND h.slot_seal IS ?
		AND (SELECT file FROM pragma_database_list WHERE name='main')=?
		AND (SELECT foreign_keys FROM pragma_foreign_keys)=1
		AND NOT EXISTS(SELECT 1 FROM main.cjsi_publication_attempts other
		    WHERE other.slot_identity=a.slot_identity AND (other.ticket,other.attempt)<>(a.ticket,a.attempt))`,
		seal[:], r.digest[:], r.size, r.format, builtAt, validUntil, ServingStorageVersion, p.ticket, p.attempt,
		b.Ticket, b.CoreEpoch, b.IndexEpoch, b.SessionRevision, b.ProtectionRevision, b.CoverageRevision,
		p.usageRevision, p.userRevision, p.routeRevision, p.priceRevision, p.input[:], p.diff[:], p.sessionSeal[:], r.identity[:],
		p.baseSlotTicket, p.baseSlotAttempt, priorSlotSeal, corePath).Scan(&state, &deadline, &empty, &exact)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return [32]byte{}, errCJSISlotAdmission
		}
		return [32]byte{}, err
	}
	until, err := time.Parse(time.RFC3339Nano, deadline)
	if err != nil || !until.Equal(p.acceptUntil) || !p.live(time.Now()) ||
		!(state == "draft" && empty || state == "sealed" && exact) {
		return [32]byte{}, errCJSISlotAdmission
	}
	// Also obtain SQLite write admission on an exact sealed retry. The draft
	// predicate then matches zero rows. A stale WAL snapshot cannot report a
	// successful read-only retry after retirement or a competing publication.
	result, err := tx.ExecContext(ctx, `UPDATE main.cjsi_publication_attempts
		SET slot_seal=?,slot_digest=?,slot_size=?,slot_format=?,built_at=?,valid_until=?,state='sealed'
		WHERE ticket=? AND attempt=? AND state='draft'`, seal[:], r.digest[:], r.size, r.format, builtAt, validUntil, p.ticket, p.attempt)
	if err != nil {
		return [32]byte{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return [32]byte{}, err
	}
	if (state == "draft" && n != 1) || (state == "sealed" && n != 0) || !p.live(time.Now()) {
		return [32]byte{}, errCJSISlotAdmission
	}
	return seal, nil
}
