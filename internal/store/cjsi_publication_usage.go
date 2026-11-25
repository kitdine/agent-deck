package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

func (m CJSIRuntimeRoot) checkUsageLocation(j cjsiLayoutJournal, h CJSIIndexStamp) error {
	if j.phase != cjsiLayoutReady || j.root != m.root || j.fence != m.fence || j.epoch != h.IndexEpoch {
		return ErrTelemetrySourceIdentityPending
	}
	path := filepath.Join(m.path, cjsiPrivateIndexName)
	switch j.selection().phase {
	case cjsiLayoutNeverBuilt:
		if h.IndexEpoch != 0 {
			return ErrTelemetrySourceIdentityPending
		}
		if err := cjsiRequireCleanIndexPath(path); err != nil {
			return errors.Join(ErrTelemetrySourceIdentityPending, err)
		}
	case cjsiLayoutSelected:
		st, err := privateServingPath(path, false)
		if err != nil || st.Nlink != 1 || cjsiIdentity(st) != j.prepared.identity {
			return errors.Join(ErrTelemetrySourceIdentityPending, err)
		}
	default:
		return ErrTelemetrySourceIdentityPending
	}
	return nil
}

// CompleteUsagePublication carries the session and slot selections through
// unchanged after the independently registered non-intersection proof has
// been rechecked under the writer. A failed proof cannot update a cursor.
func (d CJSIPublicationDraft) CompleteUsagePublication(ctx context.Context, c CJSICombinedChanges) error {
	if !d.usageOnly || c.Digest == ([32]byte{}) || c.Groups == nil || c.Validate == nil || c.ApplySelected == nil || c.RecheckSources == nil || c.VerifyOpened == nil {
		return ErrTelemetrySourceIdentityPending
	}
	if err := c.RecheckSources(ctx); err != nil {
		return err
	}
	p := d.admission.proposal
	groups := map[string]cjsiStagedGroup{}
	for _, g := range c.Groups {
		if _, ok := groups[g.Thread]; ok || g.Thread == "" || g.Stamp.Basis != d.Basis() {
			return ErrTelemetrySourceIdentityPending
		}
		groups[g.Thread] = cjsiStagedGroup{g.Stamp, g.State, g.Gaps, g.Pending, g.Partial, g.UnresolvedOverlap}
	}
	return d.root.withPublicationLocks(ctx, d.election, func() (err error) {
		if c.CheckSourceWalls != nil {
			if err := c.CheckSourceWalls(ctx); err != nil {
				return err
			}
		}
		db, err := d.root.runner().core(ctx, true)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, db.Close()) }()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err = d.checkDraft(ctx, tx); err != nil {
			return err
		}
		j, err := cjsiReadLayoutJournal(ctx, tx)
		if err != nil {
			return err
		}
		if err = d.root.checkUsageLocation(j, p.base); err != nil {
			return err
		}
		if err = c.Validate(ctx, tx); err != nil {
			return err
		}
		if err = checkCJSIStagedGroups(ctx, tx, p, groups); err != nil {
			return err
		}
		if c.CoverageRevision < p.base.CoverageRevision || c.UsageRevision < p.usageRevision || c.CoverageRevision == p.base.CoverageRevision && c.UsageRevision == p.usageRevision {
			return ErrTelemetrySourceIdentityPending
		}
		if err = cjsiOpenOne(tx.ExecContext(ctx, `UPDATE cjsi_publication_head SET ticket=?,coverage_revision=? WHERE singleton=1 AND ticket=? AND session_ticket=? AND slot_ticket=? AND slot_attempt=? AND (SELECT revision FROM serving_generation WHERE singleton=1)=?`, p.ticket, c.CoverageRevision, p.base.Ticket, p.base.SessionTicket, p.baseSlotTicket, p.baseSlotAttempt, p.usageRevision)); err != nil {
			return err
		}
		if err = c.ApplySelected(ctx, tx); err != nil {
			return err
		}
		if err = cjsiOpenOne(tx.ExecContext(ctx, "UPDATE cjsi_publication_attempts SET diff_digest=?,state='opened' WHERE ticket=? AND attempt=? AND state='draft' AND diff_digest IS NULL AND slot_identity IS NULL AND slot_seal IS NULL AND session_seal IS NULL", c.Digest[:], p.ticket, p.attempt)); err != nil {
			return err
		}
		var revision int64
		if err = tx.QueryRowContext(ctx, "SELECT revision FROM serving_generation WHERE singleton=1").Scan(&revision); err != nil {
			return err
		}
		if revision != c.UsageRevision {
			return ErrCJSIIndexChanged
		}
		if err = c.VerifyOpened(ctx, tx); err != nil {
			return err
		}
		if !time.Now().Before(p.acceptUntil) {
			return context.DeadlineExceeded
		}
		if err = tx.Commit(); err != nil {
			check, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			observed := d.root.WithPublishedCore(check, func(r *sql.Tx, h CJSIIndexStamp) error {
				if h.Ticket != p.ticket {
					return ErrCJSIIndexChanged
				}
				var one int
				if e := r.QueryRowContext(check, `SELECT 1 FROM cjsi_publication_attempts WHERE ticket=? AND attempt=? AND state='opened' AND diff_digest=? AND input_digest=?`, p.ticket, p.attempt, c.Digest[:], p.input[:]).Scan(&one); e != nil {
					return e
				}
				return c.VerifyOpened(check, r)
			})
			if observed != nil {
				return errors.Join(errCJSIOpenUnconfirmed, err, observed)
			}
		}
		return nil
	})
}

// SetUsagePending records only this verified group's update
// marker. Repeating the operation does not invalidate another cache revision.
func (m CJSIRuntimeRoot) SetUsagePending(ctx context.Context, group [32]byte, election CJSIRuntimeElection) error {
	if group == ([32]byte{}) {
		return ErrTelemetrySourceIdentityPending
	}
	return m.withPublicationLocks(ctx, election, func() (err error) {
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
		head, e := cjsiReadIndexHead(ctx, tx)
		if e != nil {
			return e
		}
		if head.CoreEpoch != m.epoch {
			return ErrCJSIIndexChanged
		}
		var window [32]byte
		r, err := tx.ExecContext(ctx, `INSERT INTO cjsi_scope_overlays(domain,scope_kind,scope_key,window_key,pending,partial,masked,coverage_revision,protection_revision)
  SELECT 'usage','group',?,?,1,0,0,coverage_revision+1,protection_revision FROM cjsi_publication_head WHERE singleton=1 AND core_epoch=?
  ON CONFLICT(domain,scope_kind,scope_key,window_key) DO UPDATE SET pending=1,coverage_revision=excluded.coverage_revision WHERE pending=0`, group[:], window[:], m.epoch)
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		if err != nil {
			return err
		}
		if n != 0 {
			if err = cjsiOpenOne(tx.ExecContext(ctx, "UPDATE cjsi_publication_head SET coverage_revision=coverage_revision+1 WHERE singleton=1 AND core_epoch=?", m.epoch)); err != nil {
				return err
			}
		}
		return tx.Commit()
	})
}

// ClearUsagePendingTx participates in the same opening transaction as the
// chosen facts. The coordinator selects the already-computed coverage revision.
func ClearUsagePendingTx(ctx context.Context, tx *sql.Tx, group [32]byte, before, revision int64) error {
	if tx == nil || group == ([32]byte{}) || before < 0 || revision < before {
		return ErrTelemetrySourceIdentityPending
	}
	var window [32]byte
	if before == revision {
		var pending bool
		err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM cjsi_scope_overlays WHERE domain='usage' AND scope_kind='group' AND scope_key=? AND window_key=? AND pending=1)", group[:], window[:]).Scan(&pending)
		if err != nil {
			return err
		}
		if pending {
			return ErrCJSIIndexChanged
		}
		return nil
	}
	_, err := tx.ExecContext(ctx, `UPDATE cjsi_scope_overlays SET pending=0,coverage_revision=? WHERE domain='usage' AND scope_kind='group' AND scope_key=? AND window_key=? AND pending=1`, revision, group[:], window[:])
	return err
}

// PublicationOutcome preserves the underlying database or source error and
// presents existing retryable admission failures to watch as pending.
func PublicationOutcome(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var sqlite interface{ Code() int }
	if errors.Is(err, ErrCJSIIndexChanged) || errors.Is(err, errCJSIOpenChanged) || errors.Is(err, errCJSISlotAdmission) || errors.Is(err, errCJSIOTelPrecondition) || errors.Is(err, ErrOriginProjectionStale) || errors.Is(err, ErrStateBusy) || errors.Is(err, sql.ErrNoRows) || errors.Is(err, unix.EWOULDBLOCK) || errors.As(err, &sqlite) && sqlite.Code()&255 == 5 {
		return errors.Join(ErrTelemetrySourceIdentityPending, err)
	}
	return err
}
