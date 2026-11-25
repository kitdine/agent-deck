package doctor

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"net/url"
	"path/filepath"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const cjsiCoreSchemaVersion = 37

// The legacy core opener rejects schema 37. A version number alone cannot
// distinguish a managed layout from a future or malformed legacy database.
func hasCJSILayout(ctx context.Context, root string) (bool, error) {
	uri := url.URL{Scheme: "file", Path: filepath.Join(root, "agentdeck.sqlite3"), RawQuery: "mode=ro&_pragma=busy_timeout(5000)"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return false, err
	}
	defer db.Close()
	var managed bool
	err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_schema
		WHERE name IN ('cjsi_layout_journal','cjsi_publication_head'))`).Scan(&managed)
	return managed, err
}

func (s Service) checkCJSIDatabase(ctx context.Context, full bool, report Report) (Report, error) {
	// The legacy provider/usage diagnostics need a managed core opener before
	// they can be included in this report.
	report.Partial = true
	root, err := store.ResolveCJSIRuntimeRoot(ctx, s.StateRoot)
	if err != nil || root == nil {
		if ctx.Err() != nil {
			return Report{}, ctx.Err()
		}
		report.add(cjsiDatabaseFailure(err))
		return report, nil
	}
	var integrityOK bool
	var index session.Health
	var schemaErr error
	err = root.WithPublishedCore(ctx, func(tx *sql.Tx, stamp store.CJSIIndexStamp) error {
		var indexErr error
		index, indexErr = session.CheckSelectedIndexHealth(ctx, root, tx, full)
		if indexErr != nil {
			if cjsiOperationalFailure(indexErr) != "" {
				return indexErr
			}
			index = session.Health{Present: true, Integrity: "unavailable"}
		}
		built, err := selectedCJSICoreSlot(ctx, tx, stamp)
		if err == nil && !built.IsZero() {
			// Check the selected physical receipt at its build time. Reaching
			// its service cutoff is normal and does not damage the seal.
			_, err = root.ReadServingSlot(ctx, tx, built)
		}
		if err == nil && full {
			var integrity string
			err = tx.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity)
			integrityOK = integrity == "ok"
		}
		schemaErr = err
		return nil
	})
	if ctx.Err() != nil {
		return Report{}, ctx.Err()
	}
	if err != nil {
		report.add(cjsiDatabaseFailure(err))
		return report, nil
	}
	if schemaErr != nil {
		report.add(cjsiDatabaseFailure(schemaErr))
	} else {
		report.add(Check{Name: "schema", Status: "ok", Count: cjsiCoreSchemaVersion})
		if full && !integrityOK {
			report.add(Check{Name: "database_integrity", Status: "error", Code: "integrity_failed"})
		} else if full {
			report.add(Check{Name: "database_integrity", Status: "ok"})
		}
	}

	addSessionHealth(&report, index, full)
	return report, nil
}

// Busy and access failures use the same read-failure diagnostics as the
// initial database open. The root admission lock has no doctor lock resource.
func cjsiOperationalFailure(err error) string {
	if errors.Is(err, store.ErrStateBusy) {
		return databaseCode(err)
	}
	if errors.Is(err, fs.ErrPermission) {
		return "database_unreadable"
	}
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() & 0xff {
		case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED, sqlite3.SQLITE_IOERR, sqlite3.SQLITE_CANTOPEN,
			sqlite3.SQLITE_PERM, sqlite3.SQLITE_AUTH, sqlite3.SQLITE_READONLY, sqlite3.SQLITE_INTERRUPT,
			sqlite3.SQLITE_NOMEM, sqlite3.SQLITE_FULL, sqlite3.SQLITE_PROTOCOL:
			return "database_unreadable"
		}
	}
	return ""
}

func cjsiDatabaseFailure(err error) Check {
	if code := cjsiOperationalFailure(err); code != "" {
		return Check{Name: "database", Status: "error", Code: code}
	}
	return Check{Name: "schema", Status: "error", Code: "schema_incompatible", Count: cjsiCoreSchemaVersion}
}

// selectedCJSICoreSlot checks references from the published head and ignores
// unselected attempts. A zero time means there is no readable slot to probe.
func selectedCJSICoreSlot(ctx context.Context, tx *sql.Tx, stamp store.CJSIIndexStamp) (time.Time, error) {
	var valid bool
	if err := tx.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM sqlite_schema WHERE type='table' AND name IN (
			'cjsi_layout_journal','cjsi_publication_head','cjsi_publication_attempts',
			'cjsi_scope_overlays','cjsi_usage_groups','cjsi_usage_selection','cjsi_usage_sources',
			'cjsi_usage_members','cjsi_usage_intervals','cjsi_usage_interval_dirty_ranges','cjsi_usage_references','cjsi_old_preconditions',
			'cjsi_checkpoint_attempts','cjsi_checkpoint_records','cjsi_takeovers',
			'cjsi_receipt_observations','cjsi_receipt_events','cjsi_receipt_sessions','cjsi_receipt_markers'
		))=19 AND NOT EXISTS(SELECT 1 FROM sqlite_schema WHERE name='telemetry_jsonl_retracts_otel' COLLATE NOCASE)
		AND (?=0 OR 1=(SELECT count(*) FROM cjsi_publication_attempts
			WHERE ticket=? AND core_epoch=? AND state='opened' AND diff_digest IS NOT NULL))
		AND NOT EXISTS(SELECT 1 FROM cjsi_usage_selection s
			LEFT JOIN cjsi_usage_groups g USING(ticket,attempt,thread)
			LEFT JOIN cjsi_publication_attempts a USING(ticket,attempt)
			WHERE g.ticket IS NULL OR a.state IS NOT 'opened' OR a.core_epoch IS NOT ? OR s.ticket>?)`,
		stamp.Ticket, stamp.Ticket, stamp.CoreEpoch, stamp.CoreEpoch, stamp.Ticket).Scan(&valid); err != nil || !valid {
		return time.Time{}, errors.Join(store.ErrCJSIIndexChanged, err)
	}

	var selected, readable bool
	var built, until sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT h.slot_ticket<>0,
		coalesce(a.protection_revision=h.protection_revision AND NOT EXISTS(SELECT 1 FROM cjsi_scope_overlays WHERE masked=1),0),
		a.built_at,a.valid_until FROM cjsi_publication_head h LEFT JOIN cjsi_publication_attempts a
		ON a.ticket=h.slot_ticket AND a.attempt=h.slot_attempt AND a.slot_seal=h.slot_seal
		AND a.ticket=h.session_ticket AND a.attempt=h.session_attempt AND a.session_seal=h.session_seal
		AND a.session_revision=h.session_revision AND a.core_epoch=h.core_epoch AND a.index_epoch=h.index_epoch
		AND a.state='opened' AND a.slot_size>0 AND a.slot_format=1 AND length(a.slot_identity)=16 AND length(a.slot_digest)=32
		AND a.protection_revision<=h.protection_revision AND a.coverage_revision<=h.coverage_revision
		WHERE h.singleton=1`).Scan(&selected, &readable, &built, &until)
	if err != nil || !selected {
		return time.Time{}, err
	}
	start, startErr := time.Parse(time.RFC3339Nano, built.String)
	end, endErr := time.Parse(time.RFC3339Nano, until.String)
	if startErr != nil || endErr != nil || !start.Before(end) {
		return time.Time{}, store.ErrCJSIIndexChanged
	}
	if !readable {
		return time.Time{}, nil
	}
	return start, nil
}
