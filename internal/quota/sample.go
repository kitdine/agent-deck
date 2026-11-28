package quota

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

type SampleAction string

const (
	SampleStart  SampleAction = "start"
	SampleFinish SampleAction = "finish"
	SampleCancel SampleAction = "cancel"
)

type SampleErrorCode string

const (
	SampleConflict        SampleErrorCode = "measurement_conflict"
	SampleMissing         SampleErrorCode = "measurement_missing"
	SampleUnsettled       SampleErrorCode = "unsettled"
	SampleProbeDisabled   SampleErrorCode = "probe_disabled"
	SampleNotOfficial     SampleErrorCode = "not_official"
	SampleProbeFailed     SampleErrorCode = "probe_failed"
	SampleAccountChanged  SampleErrorCode = "account_changed"
	SampleSessionNotFound SampleErrorCode = "session_not_found"
)

type SampleError struct {
	Code SampleErrorCode
}

func (e *SampleError) Error() string {
	switch e.Code {
	case SampleConflict:
		return "Another measurement is active"
	case SampleMissing:
		return "Start a measurement first"
	case SampleUnsettled:
		return "Usage is still settling"
	case SampleProbeDisabled:
		return "Quota reading is off"
	case SampleNotOfficial:
		return "The official provider is required for quota sampling"
	case SampleProbeFailed:
		return "Quota probe failed"
	case SampleAccountChanged:
		return "The sampled account changed"
	case SampleSessionNotFound:
		return "The selected usage session is unknown"
	default:
		return "Quota sample failed"
	}
}

// SampleResult publishes the outcome and keeps the completed sample identity local.
type SampleResult struct {
	Action     SampleAction      `json:"action"`
	Status     MeasurementStatus `json:"status"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
	Valuation  *ValuationResult  `json:"quota_valuation,omitempty"`

	completion *SampleCompletion
}

// SampleGate supplies the current quota gate facts for the selected client.
// RouteChanged requires positive evidence that the sampled official route has
// been superseded. A missing or unreadable provider selection alone is insufficient.
// AuthenticationFailed likewise requires a positive authentication result,
// independently of the probe adapter's generic exit or parse errors.
type SampleGate struct {
	ProbeEnabled         bool
	RecordedOfficial     bool
	SelectedAt           time.Time
	ObservedKnown        bool
	ObservedOfficial     bool
	RouteChanged         bool
	AuthenticationFailed bool
}

// Sampler runs explicit probes under the same state-root lock as quota refresh.
// Gate rechecks routing inside the transaction, including changes since the
// nonzero probe start on publication. UsagePending reads outstanding usage in
// the same transaction.
type Sampler struct {
	Scheduler    Scheduler
	StateRoot    string
	LockTimeout  time.Duration
	Gate         func(context.Context, *sql.Tx, Client, time.Time) (SampleGate, error)
	UsagePending func(context.Context, *sql.Tx, Client, string, time.Time) (bool, error)
}

func (s Sampler) Run(ctx context.Context, action SampleAction, client Client, sessionID string) (result SampleResult, err error) {
	if (action != SampleStart && action != SampleFinish && action != SampleCancel) ||
		(client != ClientCodex && client != ClientClaude) || sessionID == "" {
		return result, errors.New("invalid quota sample selection")
	}
	lock, err := store.AcquireQuotaRefreshLock(ctx, s.StateRoot, s.LockTimeout)
	if err != nil {
		return result, err
	}
	defer func() {
		if releaseErr := lock.Release(); err == nil {
			err = releaseErr
		}
	}()

	result = SampleResult{Action: action, Status: MeasurementMissing}
	active, beforeUsage, done, err := s.prepare(ctx, action, client, sessionID, &result)
	if err != nil || done {
		return result, err
	}

	started := s.Scheduler.now().UTC()
	if action == SampleFinish && !beforeUsage.lastCallAt.Before(started) {
		return result, &SampleError{Code: SampleUnsettled}
	}
	if _, busy := quotaProbeInFlight.LoadOrStore(client, struct{}{}); busy {
		return result, store.ErrStateBusy
	}
	defer quotaProbeInFlight.Delete(client)
	readings, err := s.probe(ctx, client, started)
	if err != nil {
		if probeAuthenticationFailed(err) {
			if invalidateErr := s.Scheduler.Store.invalidateSamples(ctx, client); invalidateErr != nil {
				return result, invalidateErr
			}
		}
		return result, &SampleError{Code: SampleProbeFailed}
	}
	return s.publish(ctx, action, client, sessionID, active, beforeUsage, readings)
}

func (s Sampler) prepare(ctx context.Context, action SampleAction, client Client, sessionID string, result *SampleResult) (*sampleRow, sampleUsageState, bool, error) {
	tx, err := s.Scheduler.Store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, sampleUsageState{}, false, err
	}
	defer tx.Rollback()
	lastCallAt, err := selectedSession(ctx, tx, client, sessionID)
	if err != nil {
		return nil, sampleUsageState{}, false, err
	}
	active, err := readSample(ctx, tx, client, sampleActive)
	if err != nil {
		return nil, sampleUsageState{}, false, err
	}
	sameSession := active != nil && active.sessionID == sessionID
	switch action {
	case SampleCancel:
		if sameSession {
			if err := deleteActiveSample(ctx, tx, client, active); err != nil {
				return nil, sampleUsageState{}, false, err
			}
			return nil, sampleUsageState{}, true, tx.Commit()
		}
		return nil, sampleUsageState{}, true, nil
	case SampleStart:
		if sameSession {
			if err := s.checkGate(ctx, tx, client, true, time.Time{}); err != nil {
				return nil, sampleUsageState{}, false, err
			}
			result.Status, result.StartedAt = MeasurementActive, &active.startedAt
			return nil, sampleUsageState{}, true, nil
		}
	case SampleFinish:
		if !sameSession {
			latest, err := readSample(ctx, tx, client, sampleLatest)
			if err != nil {
				return nil, sampleUsageState{}, false, err
			}
			if latest != nil && latest.sessionID == sessionID {
				if err := s.checkGate(ctx, tx, client, true, time.Time{}); err != nil {
					return nil, sampleUsageState{}, false, err
				}
				result.Status, result.StartedAt, result.FinishedAt = MeasurementComplete, &latest.startedAt, latest.finishedAt
				result.completion = &SampleCompletion{generation: latest.generation}
				return nil, sampleUsageState{}, true, nil
			}
			if active == nil {
				return nil, sampleUsageState{}, false, &SampleError{Code: SampleMissing}
			}
		}
	}
	if active != nil && !sameSession {
		return nil, sampleUsageState{}, false, &SampleError{Code: SampleConflict}
	}
	if err := s.checkGate(ctx, tx, client, false, time.Time{}); err != nil {
		return nil, sampleUsageState{}, false, err
	}
	var usageState sampleUsageState
	if action == SampleFinish {
		usageState, err = readSampleUsage(ctx, tx, client, sessionID, lastCallAt)
		if err != nil {
			return nil, sampleUsageState{}, false, err
		}
		if err := s.checkPending(ctx, tx, client, sessionID, usageState.lastCallAt); err != nil {
			return nil, sampleUsageState{}, false, err
		}
	}
	return active, usageState, false, nil
}

func selectedSession(ctx context.Context, tx *sql.Tx, client Client, sessionID string) (time.Time, error) {
	var last string
	err := tx.QueryRowContext(ctx, `SELECT last_at FROM usage_sessions WHERE client=? AND session_id=?`, client, sessionID).Scan(&last)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return time.Time{}, &SampleError{Code: SampleSessionNotFound}
	case err != nil:
		return time.Time{}, err
	}
	at, err := time.Parse(timeLayout, last)
	if err != nil {
		return time.Time{}, errors.New("selected session usage time is invalid")
	}
	return at, nil
}

type sampleUsageState struct {
	lastCallAt  time.Time
	fingerprint [sha256.Size]byte
}

// Read the admitted calls under the same transaction as the sample and gate.
// The session aggregate alone cannot detect an older call arriving while the
// probe runs, and RFC3339Nano text ordering can pick the wrong last call.
func readSampleUsage(ctx context.Context, tx *sql.Tx, client Client, sessionID string, lastCallAt time.Time) (sampleUsageState, error) {
	state := sampleUsageState{lastCallAt: lastCallAt}
	rows, err := tx.QueryContext(ctx, `SELECT event_at, json_array(event_key,event_at,model,input_tokens,cached_input_tokens,
		output_tokens,cache_read_tokens,cache_creation_tokens,cache_write_5m_tokens,cache_write_1h_tokens,cache_write_tokens)
		FROM usage_events WHERE client=? AND session_id=? ORDER BY event_key`, client, sessionID)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	hash := sha256.New()
	for rows.Next() {
		var atText, event string
		if err := rows.Scan(&atText, &event); err != nil {
			return state, err
		}
		at, err := time.Parse(timeLayout, atText)
		if err != nil {
			return state, errors.New("selected session event time is invalid")
		}
		if at.After(state.lastCallAt) {
			state.lastCallAt = at
		}
		hash.Write([]byte(event))
		hash.Write([]byte{'\n'})
	}
	hash.Sum(state.fingerprint[:0])
	return state, rows.Err()
}

func (s Sampler) checkGate(ctx context.Context, tx *sql.Tx, client Client, replay bool, probeStartedAt time.Time) error {
	var gate SampleGate
	if s.Gate != nil {
		var err error
		gate, err = s.Gate(ctx, tx, client, probeStartedAt)
		if err != nil {
			if replay {
				return nil
			}
			return &SampleError{Code: SampleProbeFailed}
		}
	}
	allowed, reason := Allowed(gate.ProbeEnabled, gate.RecordedOfficial, gate.ObservedKnown, gate.ObservedOfficial)
	routeChanged := gate.RouteChanged || (gate.ObservedKnown && !gate.ObservedOfficial)
	if routeChanged || gate.AuthenticationFailed {
		if err := deleteClientSamples(ctx, tx, client); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if allowed || replay {
			reason = ReasonProbeFailed
			if routeChanged {
				reason = ReasonNotOfficial
			}
		}
		return &SampleError{Code: SampleErrorCode(reason)}
	}
	if !replay && !allowed {
		return &SampleError{Code: SampleErrorCode(reason)}
	}
	if !probeStartedAt.IsZero() && gate.SelectedAt.After(probeStartedAt) {
		return &SampleError{Code: SampleProbeFailed}
	}
	return nil
}

func (s Sampler) checkPending(ctx context.Context, tx *sql.Tx, client Client, sessionID string, lastCallAt time.Time) error {
	if s.UsagePending == nil {
		return nil
	}
	pending, err := s.UsagePending(ctx, tx, client, sessionID, lastCallAt)
	if err != nil {
		return err
	}
	if pending {
		return &SampleError{Code: SampleUnsettled}
	}
	return nil
}

type sampleProbe struct {
	started   time.Time
	completed time.Time
	envelope  EnvelopeRecord
	windows   []Observation
}

func (s Sampler) probe(ctx context.Context, client Client, started time.Time) (sampleProbe, error) {
	probe := sampleProbe{started: started, envelope: EnvelopeRecord{Client: client, Applicable: true, ObservedAt: started}}
	switch client {
	case ClientCodex:
		result, err := s.Scheduler.probeCodex(ctx, started)
		if err != nil {
			return sampleProbe{}, err
		}
		probe.envelope.Source, probe.envelope.AccountID = SourceCodex, result.AccountID
		probe.envelope.Plan, probe.envelope.PlanReason = result.Plan, result.PlanReason
		probe.envelope.ResetAllowance, probe.envelope.Billing = result.ResetAllowance, result.Billing
		probe.windows = result.Windows
	case ClientClaude:
		result, err := s.Scheduler.probeClaudeProse(ctx, started)
		if err != nil {
			return sampleProbe{}, err
		}
		probe.envelope.Source, probe.windows = SourceClaudeProse, result.Windows
	}
	probe.completed = s.Scheduler.now().UTC()
	if probe.completed.Before(probe.started) {
		return sampleProbe{}, &SampleError{Code: SampleProbeFailed}
	}
	return probe, nil
}

func (s Sampler) publish(ctx context.Context, action SampleAction, client Client, sessionID string, active *sampleRow, beforeUsage sampleUsageState, probe sampleProbe) (SampleResult, error) {
	result := SampleResult{Action: action, Status: MeasurementActive, StartedAt: &probe.started}
	tx, err := s.Scheduler.Store.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	lastCallAt, err := selectedSession(ctx, tx, client, sessionID)
	if err != nil {
		return result, err
	}
	current, err := readSample(ctx, tx, client, sampleActive)
	if err != nil {
		return result, err
	}
	if err := s.checkGate(ctx, tx, client, false, probe.started); err != nil {
		return result, err
	}
	if !sameSampleGeneration(active, current) {
		return result, &SampleError{Code: SampleConflict}
	}

	digest := accountDigest(client, probe.envelope.AccountID)
	endpoint, err := recordSampleEndpoint(ctx, tx, probe, digest)
	if err != nil {
		return result, err
	}
	invalidated, err := invalidateSamplesForAccount(ctx, tx, client, digest)
	if err != nil {
		return result, err
	}
	if active != nil && (invalidated || active.start.Source != endpoint.Source) {
		if err := deleteClientSamples(ctx, tx, client); err != nil {
			return result, err
		}
		if err := tx.Commit(); err != nil {
			return result, err
		}
		return result, &SampleError{Code: SampleAccountChanged}
	}

	var generation string
	if action == SampleFinish {
		usageState, err := readSampleUsage(ctx, tx, client, sessionID, lastCallAt)
		if err != nil {
			return result, err
		}
		if !usageState.lastCallAt.Before(probe.started) || !usageState.lastCallAt.Equal(beforeUsage.lastCallAt) ||
			usageState.fingerprint != beforeUsage.fingerprint || probe.started.Before(active.start.ProbeCompletedAt) {
			return result, &SampleError{Code: SampleUnsettled}
		}
		if err := s.checkPending(ctx, tx, client, sessionID, usageState.lastCallAt); err != nil {
			return result, err
		}
		result.Status, result.StartedAt, result.FinishedAt = MeasurementComplete, &active.startedAt, &probe.completed
		result.completion = &SampleCompletion{generation: active.generation}
	} else {
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return result, err
		}
		generation = hex.EncodeToString(nonce[:])
	}

	if err := saveSample(ctx, tx, client, sessionID, generation, active, endpoint); err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func sameSampleGeneration(before, after *sampleRow) bool {
	if before == nil || after == nil {
		return before == after
	}
	return before.generation == after.generation && before.sessionID == after.sessionID
}

func recordSampleEndpoint(ctx context.Context, tx *sql.Tx, probe sampleProbe, digest string) (sampleEndpoint, error) {
	client := probe.envelope.Client
	endpoint := sampleEndpoint{
		Version: 1, ProbeStartedAt: probe.started, ProbeCompletedAt: probe.completed,
		Source: probe.envelope.Source, AccountDigest: digest, Windows: make([]sampleWindow, 0, len(probe.windows)),
	}
	var previousAt string
	var previousSource Source
	err := tx.QueryRowContext(ctx, `SELECT observed_at, source FROM quota_envelopes WHERE client=?`, client).Scan(&previousAt, &previousSource)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return sampleEndpoint{}, err
	}
	cacheCurrent := true
	if previousAt != "" {
		at, err := time.Parse(timeLayout, previousAt)
		if err != nil {
			return sampleEndpoint{}, errors.New("stored quota observation time is invalid")
		}
		cacheCurrent = !at.After(probe.started) && !(at.Equal(probe.started) && previousSource == SourceClaudeStatusLine)
	}
	for _, obs := range probe.windows {
		if obs.Client != client || obs.Source != endpoint.Source || obs.WindowKey == "" || !validPercent(obs.UsedPercent) {
			return sampleEndpoint{}, &SampleError{Code: SampleProbeFailed}
		}
		window := sampleWindow{
			WindowKey: obs.WindowKey, Pool: obs.Pool, Slot: obs.Slot,
			WindowMinutes: obs.WindowMinutes, UsedPercent: obs.UsedPercent, ResetsAt: observedTime(obs.ResetsAt),
		}
		if cacheCurrent {
			obs.AccountID, obs.ObservedAt = probe.envelope.AccountID, probe.started
			_, _, resetAt, err := recordObservation(ctx, tx, obs)
			if err != nil {
				return sampleEndpoint{}, err
			}
			window.ObservedResetAt = observedTime(resetAt)
		} else {
			_, resetAt, err := queryStored(ctx, tx, client, digest, obs.WindowKey)
			if err != nil {
				return sampleEndpoint{}, err
			}
			window.ObservedResetAt = observedTime(resetAt)
		}
		endpoint.Windows = append(endpoint.Windows, window)
	}
	if cacheCurrent {
		if err := pruneWindows(ctx, tx, client, probe.envelope.AccountID, observedWindowKeys(probe.windows)); err != nil {
			return sampleEndpoint{}, err
		}
		if err := putEnvelope(ctx, tx, probe.envelope); err != nil {
			return sampleEndpoint{}, err
		}
	}
	return endpoint, nil
}
