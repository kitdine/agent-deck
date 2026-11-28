package quota

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type sampleSlot string

const (
	sampleActive sampleSlot = "active"
	sampleLatest sampleSlot = "latest"
)

// sampleEndpoint is the on-disk whitelist. Probe result metadata and account
// IDs are kept out of it even when supplied by the adapter.
type sampleEndpoint struct {
	Version          int            `json:"version"`
	ProbeStartedAt   time.Time      `json:"probe_started_at"`
	ProbeCompletedAt time.Time      `json:"probe_completed_at"`
	Source           Source         `json:"source"`
	AccountDigest    string         `json:"account_digest,omitempty"`
	Windows          []sampleWindow `json:"windows"`
}

type sampleWindow struct {
	WindowKey       string     `json:"window_key"`
	Pool            string     `json:"pool"`
	Slot            string     `json:"slot"`
	WindowMinutes   int        `json:"window_minutes"`
	UsedPercent     float64    `json:"used_percent"`
	ResetsAt        *time.Time `json:"resets_at"`
	ObservedResetAt *time.Time `json:"observed_reset_at,omitempty"`
	PrecisionPP     *float64   `json:"precision_pp,omitempty"`
}

type sampleRow struct {
	generation string
	sessionID  string
	startedAt  time.Time
	finishedAt *time.Time
	start      sampleEndpoint
}

var errInvalidStoredSample = errors.New("stored quota sample is invalid")

func readSample(ctx context.Context, tx *sql.Tx, client Client, slot sampleSlot) (*sampleRow, error) {
	var row sampleRow
	var started, startJSON string
	var finished sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT generation, session_id, started_at, finished_at, start_json
		FROM quota_session_samples WHERE client=? AND slot=?`, client, slot).
		Scan(&row.generation, &row.sessionID, &started, &finished, &startJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	row.startedAt, err = time.Parse(timeLayout, started)
	if err != nil || json.Unmarshal([]byte(startJSON), &row.start) != nil || row.start.Version != 1 {
		return nil, errInvalidStoredSample
	}
	if finished.Valid {
		at, err := time.Parse(timeLayout, finished.String)
		if err != nil {
			return nil, errInvalidStoredSample
		}
		row.finishedAt = &at
	}
	return &row, nil
}

func deleteActiveSample(ctx context.Context, tx *sql.Tx, client Client, selected *sampleRow) error {
	result, err := tx.ExecContext(ctx, `DELETE FROM quota_session_samples
		WHERE client=? AND slot='active' AND generation=? AND session_id=?`,
		client, selected.generation, selected.sessionID)
	return requireSampleChange(result, err)
}

func saveSample(ctx context.Context, tx *sql.Tx, client Client, sessionID, generation string, active *sampleRow, endpoint sampleEndpoint) error {
	payload, err := json.Marshal(endpoint)
	if err != nil {
		return &SampleError{Code: SampleProbeFailed}
	}
	if active == nil {
		_, err := tx.ExecContext(ctx, `INSERT INTO quota_session_samples(client, slot, generation, session_id, started_at, start_json)
			VALUES (?, 'active', ?, ?, ?, ?)`, client, generation, sessionID, endpoint.ProbeStartedAt.Format(timeLayout), string(payload))
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM quota_session_samples WHERE client=? AND slot='latest'`, client); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE quota_session_samples SET slot='latest', finished_at=?, finish_json=?
		WHERE client=? AND slot='active' AND generation=? AND session_id=?`,
		endpoint.ProbeCompletedAt.Format(timeLayout), string(payload), client, active.generation, active.sessionID)
	return requireSampleChange(result, err)
}

func requireSampleChange(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return &SampleError{Code: SampleConflict}
	}
	return nil
}

func deleteClientSamples(ctx context.Context, tx quotaSQL, client Client) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM quota_session_samples WHERE client=?`, client)
	return err
}

func sampleTableExists(ctx context.Context, tx quotaSQL) (bool, error) {
	var installed bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_schema
		WHERE type='table' AND name='quota_session_samples')`).Scan(&installed)
	return installed, err
}

func (s *Store) invalidateSampleCycle(ctx context.Context, cycle RefreshCycle, client Client, expected Generation) error {
	_, err := s.cycleWrite(ctx, cycle, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		if client == ClientClaude {
			state, seq, err := readClaudeContext(ctx, conn)
			if err != nil || state.Generation != expected || seq >= cycle.seq {
				return false, err
			}
		}
		err := deleteClientSamples(ctx, conn, client)
		return err == nil, err
	})
	return err
}

func invalidateSamplesIfInstalled(ctx context.Context, tx quotaSQL, client Client) error {
	installed, err := sampleTableExists(ctx, tx)
	if err != nil || !installed {
		return err
	}
	return deleteClientSamples(ctx, tx, client)
}

// Compared with checking only the most recent cache account, comparing the
// saved digest still detects a change after an intervening anonymous probe.
// The optional table check keeps the ordinary cache usable on pre-sample cores.
func invalidateSamplesForAccount(ctx context.Context, tx quotaSQL, client Client, digest string) (bool, error) {
	if digest == "" {
		return false, nil
	}
	installed, err := sampleTableExists(ctx, tx)
	if err != nil || !installed {
		return false, err
	}
	var changed bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM quota_session_samples WHERE client=?
		AND (CASE WHEN json_valid(start_json) THEN json_extract(start_json, '$.account_digest') END NOT IN ('', ?)
			OR CASE WHEN json_valid(finish_json) THEN json_extract(finish_json, '$.account_digest') END NOT IN ('', ?))
	)`, client, digest, digest).Scan(&changed)
	if err != nil || !changed {
		return false, err
	}
	return true, deleteClientSamples(ctx, tx, client)
}
