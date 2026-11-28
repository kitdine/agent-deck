package usage

import (
	"context"
	"database/sql"
	"time"

	"github.com/kitdine/agent-deck/internal/provider"
)

// ObservedNonOfficialSinceTx detects a Hook-observed excursion that may
// already have been followed by another official observation. Hook timestamps
// and the probe start are UTC. Removing the Z preserves fractional ordering.
func ObservedNonOfficialSinceTx(ctx context.Context, tx *sql.Tx, client string, since time.Time) (bool, error) {
	var changed bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usage_session_observations
		WHERE client=? AND observed_provider<>'' AND lower(observed_provider)<>?
		AND observed_at LIKE '%Z' AND unixepoch(substr(observed_at,1,19)) IS NOT NULL
		AND rtrim(observed_at,'Z')>=rtrim(?,'Z'))`,
		client, provider.OfficialProviderName, since.UTC().Format(time.RFC3339Nano)).Scan(&changed)
	return changed, err
}

// QuotaSampleUsagePending reads positive evidence for the selected session
// only. A pending turn older than an already admitted call can be an abandoned
// message and does not show that the current usage is still settling.
func QuotaSampleUsagePending(ctx context.Context, tx *sql.Tx, client, sessionID string, lastCallAt time.Time) (bool, error) {
	if client == "codex" {
		var pending bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM telemetry_observations
			WHERE thread_id=? AND state='pending' AND metered=1)`, sessionID).Scan(&pending); err != nil || pending {
			return pending, err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT started_at FROM usage_work_signals
		WHERE client=? AND session_id=? AND state=?`, client, sessionID, signalStatePending)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var started string
		if err := rows.Scan(&started); err != nil {
			return false, err
		}
		at, err := time.Parse(time.RFC3339Nano, started)
		if err != nil {
			return false, err
		}
		if !at.Before(lastCallAt) {
			return true, nil
		}
	}
	return false, rows.Err()
}
