package telemetry

import (
	"context"
	"errors"
	"time"

	"github.com/kitdine/agent-deck/internal/perftrace"
	"github.com/kitdine/agent-deck/internal/store"
)

// Capability is the metering state of the OTel source.
type Capability string

const (
	CapabilityDisabled    Capability = "disabled"
	CapabilityCaptureOnly Capability = "capture_only"
	CapabilityReconciled  Capability = "reconciled"
)

// Capture-only reasons, as status reason_code values.
const (
	ReasonProducerUnsupported = "producer_unsupported"
	ReasonNoProducerObserved  = "no_verified_producer_observed"
	ReasonConversationMissing = "conversation_missing"
	ReasonOwnershipNotSettled = "ownership_not_settled"
	ReasonNoJSONLEvidence     = "no_jsonl_evidence"
)

// SettleAfter is how long a supported observation waits, together with a
// later complete inventory, before its conversation can be OTel-owned.
const SettleAfter = 10 * time.Minute

// SupportEntry identifies the admitted producer family, the version the
// producer actually reported, and the transport of one observation.
type SupportEntry struct {
	Producer   string
	Version    string
	Originator string
	Transport  string
}

// Name identifies the entry in observations and session markers.
func (e SupportEntry) Name() string {
	return e.Producer + "/" + e.Version + "/" + e.Originator + "/" + e.Transport
}

// supported applies store.TelemetryAdmitted, the single admission definition.
// The families and event shape were verified by isolated producer experiments
// (2026-10-29, Bug ad-bug-codex-subagent-usage-attribution): usage events
// carry conversation.id, counts equal the server's usage, and failed attempts
// emit no usage event. The version is kept as provenance and never compared.
func supported(record Record) (SupportEntry, bool) {
	if !store.TelemetryAdmitted(record.Producer, record.ProducerVersion, record.Originator, record.Transport, record.EventName, record.EventKind) {
		return SupportEntry{}, false
	}
	return SupportEntry{Producer: record.Producer, Version: record.ProducerVersion, Originator: record.Originator, Transport: record.Transport}, true
}

// MeteringCapability is the reconciler's verdict: disabled when the receiver
// is off, reconciled when the latest metered event was admitted,
// capture_only otherwise.
func MeteringCapability(enabled bool, lastSupportEntry string, anyReceived bool) (Capability, string) {
	switch {
	case !enabled:
		return CapabilityDisabled, ""
	case !anyReceived:
		return CapabilityCaptureOnly, ReasonNoProducerObserved
	case lastSupportEntry == "":
		return CapabilityCaptureOnly, ReasonProducerUnsupported
	default:
		return CapabilityReconciled, ""
	}
}

// Observation turns an allowlisted record into its stored form. A supported
// metered record starts pending; ownership is decided per conversation by
// store.SettleTelemetry, never by time, model or token similarity.
func Observation(record Record, source string, received time.Time) store.TelemetryObservation {
	observation := store.TelemetryObservation{
		Key:              record.Key(),
		Source:           source,
		Producer:         record.Producer,
		ProducerVersion:  record.ProducerVersion,
		Originator:       record.Originator,
		EventName:        record.EventName,
		EventKind:        record.EventKind,
		Transport:        record.Transport,
		RecordAt:         record.RecordAt,
		ObservedAt:       record.ObservedAt,
		ReceivedAt:       received.UTC().Format(time.RFC3339Nano),
		Model:            record.Model,
		ResponseID:       record.ResponseID,
		RequestID:        record.RequestID,
		AttemptID:        record.AttemptID,
		ThreadID:         record.ThreadID,
		SourceClass:      record.SourceClass,
		InputTokens:      record.InputTokens,
		OutputTokens:     record.OutputTokens,
		CachedTokens:     record.CachedTokens,
		CacheWriteTokens: record.CacheWriteTokens,
		ReasoningTokens:  record.ReasoningTokens,
		TotalTokens:      record.TotalTokens,
		Metered:          record.Metered(),
	}
	entry, ok := supported(record)
	switch {
	case record.Invalid != "":
		observation.State, observation.Reason = "invalid", record.Invalid
		observation.Metered = false
	case !observation.Metered:
		observation.State, observation.Reason = "unmetered", "no_completed_usage"
		if record.EventKind == "response.completed" && (record.InputTokens != nil || record.OutputTokens != nil || record.TotalTokens != nil || record.CachedTokens != nil || record.CacheWriteTokens != nil || record.ReasoningTokens != nil) {
			observation.State, observation.Reason = "capture_only", store.TelemetryIncompleteCountsReason
		}
	case !ok:
		observation.State, observation.Reason = "capture_only", ReasonProducerUnsupported
	case record.ThreadID == "":
		observation.State, observation.Reason = "capture_only", ReasonConversationMissing
	default:
		observation.SupportEntry = entry.Name()
		observation.State, observation.Reason = "pending", ReasonOwnershipNotSettled
	}
	return observation
}

// Reconciler drains the journal into the observation table and settles
// conversation ownership. The cursor and the observations advance in one
// transaction, so a crash replays at most the uncommitted batch, and a replay
// is idempotent by observation key.
type Reconciler struct {
	Store   *store.Store
	Journal *Journal
	Now     func() time.Time
	Batch   int
	// SessionsChanged, when set, runs after a settle pass changed OTel-owned
	// sessions, to refresh the session-index projection.
	SessionsChanged func(context.Context) error
	// RefreshSessions requests initial projection/classification even without a numerical change.
	RefreshSessions bool
	// PrepareSources, when set, runs before every settle pass. The caller
	// injects the usage layer's source identity preparation, which this
	// package cannot import. The store checks the outcome inside the settle
	// transaction, so a missing or failed preparation defers promotion.
	PrepareSources    func(context.Context) (bool, error)
	projectionPending bool
}

// Drain normalizes every complete journal record after the cursor.
func (r *Reconciler) Drain(ctx context.Context) (store.TelemetryApplyResult, error) {
	var total store.TelemetryApplyResult
	for {
		cursor, err := r.Store.TelemetryCursorPosition(ctx)
		if err != nil {
			return total, err
		}
		records, next, err := r.Journal.ReadFrom(cursor, r.Batch)
		if err != nil {
			return total, err
		}
		if next == cursor {
			return total, nil
		}
		received := r.Now()
		batch := make([]store.TelemetryObservation, 0, len(records))
		for _, record := range records {
			batch = append(batch, Observation(record, "receiver", received))
		}
		result, err := r.Store.ApplyTelemetryObservations(ctx, batch, &cursor, &next)
		if errors.Is(err, store.ErrTelemetryCursorMoved) {
			continue
		}
		if err != nil {
			return total, err
		}
		perftrace.Mark(perftrace.OTelNormalized, perftrace.Count{Key: "records", Value: int64(len(records))}, perftrace.Count{Key: "inserted", Value: int64(result.Inserted)})
		total.Inserted += result.Inserted
		total.Duplicate += result.Duplicate
		total.Pending += result.Pending
		total.CaptureOnly += result.CaptureOnly
		total.Conflict += result.Conflict
		total.Invalid += result.Invalid
		total.Unmetered += result.Unmetered
		total.Changed = total.Changed || result.Changed
		r.projectionPending = r.projectionPending || result.Changed
	}
}

// Settle re-evaluates conversation ownership and, when anything changed,
// refreshes the session-index projection. A pass that could not promote
// because source identities are incomplete still commits and projects its
// retractions, and returns store.ErrTelemetrySourceIdentityPending: it is not
// a successful settlement.
func (r *Reconciler) Settle(ctx context.Context) (store.TelemetrySettleResult, error) {
	var prepareErr error
	var preparedChanged bool
	if r.PrepareSources != nil {
		preparedChanged, prepareErr = r.PrepareSources(ctx)
	}
	result, err := r.Store.SettleTelemetry(ctx, r.Now(), SettleAfter)
	// Preparation may have committed a retraction before settlement fails.
	// Keep that work pending until its projection succeeds on a later call.
	r.projectionPending = r.projectionPending || r.RefreshSessions || preparedChanged || result.Changed
	result.Changed = r.projectionPending
	pending := errors.Is(err, store.ErrTelemetrySourceIdentityPending)
	if err != nil && !pending {
		return result, err
	}
	if result.Changed && r.SessionsChanged != nil {
		if err := r.SessionsChanged(ctx); err != nil {
			r.projectionPending = true
			return result, err
		}
	}
	r.RefreshSessions = false
	if pending {
		r.projectionPending = false
		return result, err
	}
	// Callers publish completed and identity-pending passes. An ordinary
	// preparation error suppresses that notification, so a later successful
	// pass must still report the committed changes even after projection.
	r.projectionPending = result.Changed && prepareErr != nil && !errors.Is(prepareErr, store.ErrTelemetrySourceIdentityPending)
	return result, prepareErr
}

// Import normalizes records from an explicitly named OTLP JSON file through
// the same allowlist, validation and idempotent keys as the receiver.
func Import(ctx context.Context, database *store.Store, records []Record, now time.Time) (store.TelemetryApplyResult, error) {
	batch := make([]store.TelemetryObservation, 0, len(records))
	for _, record := range records {
		batch = append(batch, Observation(record, "import", now))
	}
	return database.ApplyTelemetryObservations(ctx, batch, nil, nil)
}

// Import normalizes records like the package Import and carries a retraction
// made while normalizing a conflict to the next Settle's projection.
func (r *Reconciler) Import(ctx context.Context, records []Record, now time.Time) (store.TelemetryApplyResult, error) {
	result, err := Import(ctx, r.Store, records, now)
	r.projectionPending = r.projectionPending || result.Changed
	return result, err
}
