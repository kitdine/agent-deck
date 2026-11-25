package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// These are the dependency revisions the coordinator can verify directly from
// the committed head. Selector-owned revisions, source digest and scope are
// checked against the independently validated inputs in BeginPublication.
const cjsiAttemptAtCurrentBase = `(a.base_ticket,a.core_epoch,a.index_epoch,a.session_revision,
 a.protection_revision,a.coverage_revision,a.usage_revision) =
 (h.ticket,h.core_epoch,h.index_epoch,h.session_revision,h.protection_revision,h.coverage_revision,g.revision)`

// A draft (including a frozen draft) has no registered slot clock. Both
// bounds are installed together with the durable seal. Reuse and retirement
// must agree about whether that immutable window can still authorize Open.
type cjsiStoredSlotWindow struct {
	builtAt, validUntil sql.NullString
}

func (w cjsiStoredSlotWindow) openWindow(state string, attempt int64, acceptUntil, now time.Time, windows *cjsiAllocationWindows) (time.Time, bool) {
	var end time.Time
	allocated := time.UnixMicro(attempt).UTC()
	if !now.Before(acceptUntil) || !allocated.Before(acceptUntil) {
		return end, false
	}
	switch state {
	case "draft":
		if w.builtAt.Valid || w.validUntil.Valid {
			return end, false
		}
	case "sealed":
		if !w.builtAt.Valid || !w.validUntil.Valid {
			return end, false
		}
		durableEnd, err := time.Parse(time.RFC3339Nano, w.validUntil.String)
		if err != nil || !now.Before(durableEnd) || !allocated.Before(durableEnd) {
			return end, false
		}
		built, err := time.Parse(time.RFC3339Nano, w.builtAt.String)
		if err != nil || !built.Before(durableEnd) || !built.Before(acceptUntil) {
			return end, false
		}
		end = durableEnd.UTC()
	default:
		return end, false
	}
	if windows != nil {
		// The selector can also prove expiry before slot registration. A sealed
		// receipt must fit the requested window and retains its original deadline.
		bound := windows.lookup(allocated)
		if !allocated.Before(bound) || !now.Before(bound) || end.After(bound) {
			return time.Time{}, false
		}
		if state == "draft" {
			end = bound
		}
	}
	return end, true
}

// RetireAbandonedPublication makes an expired or superseded private attempt
// terminal under the same writer fence as Open. Opened attempts are needed
// by ledger lineage. An immutable receipt is not a filesystem unlink lease.
func (m CJSIRuntimeRoot) RetireAbandonedPublication(ctx context.Context, ticket, attempt int64, election CJSIRuntimeElection) error {
	if ticket <= 0 || attempt <= 0 {
		return ErrCJSIIndexChanged
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
		var state, deadline string
		var sameBase bool
		var slotWindow cjsiStoredSlotWindow
		err = tx.QueryRowContext(ctx, `SELECT a.state,a.accept_until,a.built_at,a.valid_until,`+cjsiAttemptAtCurrentBase+`
    FROM cjsi_publication_attempts a JOIN cjsi_publication_head h ON h.singleton=1
    JOIN serving_generation g ON g.singleton=h.singleton
    JOIN derived_snapshot_generation d ON d.singleton=h.singleton AND d.epoch=h.core_epoch
    WHERE a.ticket=? AND a.attempt=? AND h.core_epoch=? AND (h.slot_ticket,h.slot_attempt)<>(a.ticket,a.attempt)
    AND (h.session_ticket,h.session_attempt)<>(a.ticket,a.attempt) AND a.state IN ('draft','sealed','retired')`,
			ticket, attempt, m.epoch).Scan(&state, &deadline, &slotWindow.builtAt, &slotWindow.validUntil, &sameBase)
		if err != nil {
			return err
		}
		if state != "retired" {
			until, err := time.Parse(time.RFC3339Nano, deadline)
			if err != nil {
				return err
			}
			_, live := slotWindow.openWindow(state, attempt, until, time.Now(), nil)
			if sameBase && live {
				return ErrTelemetrySourceIdentityPending
			}
			if err = cjsiOpenOne(tx.ExecContext(ctx, "UPDATE cjsi_publication_attempts SET state='retired' WHERE ticket=? AND attempt=? AND state=?", ticket, attempt, state)); err != nil {
				return err
			}
		}
		return tx.Commit()
	})
}
