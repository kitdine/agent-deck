package quota

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// SampleCompletion identifies the exact completed measurement returned by an
// explicit finish. Its generation is local and never appears in a response.
type SampleCompletion struct {
	generation string
}

func (r SampleResult) Completion() *SampleCompletion {
	return r.completion
}

// ReadSessionMeasurement reads only matching samples from the caller's
// transaction. The Tokens report prefers a matching active sample. Finish
// and its replay require the exact latest generation returned by the action.
func ReadSessionMeasurement(ctx context.Context, tx *sql.Tx, client Client, sessionID string, completion *SampleCompletion) (ValuationInput, error) {
	input := ValuationInput{Client: client, SessionID: sessionID}
	installed, err := sampleTableExists(ctx, tx)
	if err != nil {
		return input, err
	}
	if !installed {
		if completion != nil {
			return input, &SampleError{Code: SampleMissing}
		}
		return input, nil
	}

	query := `SELECT slot, start_json, finish_json FROM quota_session_samples WHERE client=? AND session_id=?`
	args := []any{client, sessionID}
	if completion == nil {
		query += ` ORDER BY CASE slot WHEN 'active' THEN 0 ELSE 1 END LIMIT 1`
	} else {
		query += ` AND slot='latest' AND generation=?`
		args = append(args, completion.generation)
	}
	var slot sampleSlot
	var start string
	var finish sql.NullString
	err = tx.QueryRowContext(ctx, query, args...).Scan(&slot, &start, &finish)
	if errors.Is(err, sql.ErrNoRows) {
		if completion != nil {
			return input, &SampleError{Code: SampleMissing}
		}
		return input, nil
	}
	if err != nil {
		return input, err
	}
	input.Before, err = decodeValuationEndpoint(start)
	if err == nil && slot == sampleLatest {
		input.After, err = decodeValuationEndpoint(finish.String)
	}
	return input, err
}

func decodeValuationEndpoint(raw string) (*ValuationEndpoint, error) {
	var saved sampleEndpoint
	if err := json.Unmarshal([]byte(raw), &saved); err != nil || saved.Version != 1 {
		return nil, errInvalidStoredSample
	}
	endpoint := &ValuationEndpoint{
		ProbeStartedAt: saved.ProbeStartedAt, ProbeCompletedAt: saved.ProbeCompletedAt,
		Source: saved.Source, AccountDigest: saved.AccountDigest,
		Windows: make([]SampleWindow, 0, len(saved.Windows)),
	}
	for _, value := range saved.Windows {
		window := SampleWindow{
			Window: Window{
				WindowKey: value.WindowKey, Label: sampleWindowLabel(value.WindowMinutes),
				Source: saved.Source, WindowMinutes: value.WindowMinutes, UsedPercent: value.UsedPercent,
			},
			Pool: value.Pool, Slot: value.Slot, PrecisionPP: value.PrecisionPP,
		}
		if value.ResetsAt != nil {
			window.ResetsAt = *value.ResetsAt
		}
		if value.ObservedResetAt != nil {
			window.ObservedResetAt, window.HasObservedResetAt = *value.ObservedResetAt, true
		}
		endpoint.Windows = append(endpoint.Windows, window)
	}
	return endpoint, nil
}

func sampleWindowLabel(minutes int) string {
	switch {
	case minutes <= 0:
		return "Quota window"
	case minutes == 7*24*60:
		return "Weekly"
	case minutes%(24*60) == 0:
		return fmt.Sprintf("%d-day", minutes/(24*60))
	case minutes%60 == 0:
		return fmt.Sprintf("%d-hour", minutes/60)
	default:
		return fmt.Sprintf("%d-minute", minutes)
	}
}
