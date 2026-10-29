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

// SupportEntry is one producer/version/transport/event shape whose identity
// and ownership behavior an isolated producer experiment verified.
type SupportEntry struct {
	Producer   string
	Version    string
	Originator string
	Transport  string
	EventName  string
	EventKind  string
}

// Name identifies the entry in observations and session markers.
func (e SupportEntry) Name() string {
	return e.Producer + "/" + e.Version + "/" + e.Originator + "/" + e.Transport
}

// supportTable lists the combinations verified by isolated producer
// experiments on 2026-10-29 (Bug ad-bug-codex-subagent-usage-attribution):
// usage events carry conversation.id; main requests are in the conversation's
// rollout; background requests (Memories consolidation, TUI title generation)
// run under conversations without a rollout; counts equal the server's usage;
// failed attempts emit no usage event. Combinations not listed, such as the
// Desktop originator or WebSocket, stay capture_only.
var supportTable = []SupportEntry{
	{Producer: "codex_exec", Version: "0.165.0", Originator: "codex_exec", Transport: "sse", EventName: "codex.sse_event", EventKind: "response.completed"},
	{Producer: "codex-app-server", Version: "0.165.0", Originator: "codex-tui", Transport: "sse", EventName: "codex.sse_event", EventKind: "response.completed"},
}

func supported(record Record) (SupportEntry, bool) {
	for _, entry := range supportTable {
		if record.Producer == entry.Producer && record.ProducerVersion == entry.Version && record.Originator == entry.Originator && record.Transport == entry.Transport &&
			record.EventName == entry.EventName && record.EventKind == entry.EventKind {
			return entry, true
		}
	}
	return SupportEntry{}, false
}

// MeteringCapability is the reconciler's verdict: disabled when the receiver
// is off, reconciled when the latest valid producer is in the support table,
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
	}
}

// Settle re-evaluates conversation ownership and, when anything changed,
// refreshes the session-index projection.
func (r *Reconciler) Settle(ctx context.Context) (store.TelemetrySettleResult, error) {
	result, err := r.Store.SettleTelemetry(ctx, r.Now(), SettleAfter)
	if err != nil || !result.Changed || r.SessionsChanged == nil {
		return result, err
	}
	return result, r.SessionsChanged(ctx)
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
