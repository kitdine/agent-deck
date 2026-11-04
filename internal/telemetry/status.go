package telemetry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kitdine/agent-deck/internal/store"
)

// WarningMemoriesNotCounted is the status warning when Codex Memories is on
// but collection is not enabled.
const WarningMemoriesNotCounted = "codex_memories_usage_not_counted"

// Status is telemetry.status. Field names follow the reviewed CLI specimen.
// Preference, runtime fact, connection observation, metering capability and
// Codex facts are separate fields; none is inferred from another, and an
// unknown value stays unknown or null.
type Status struct {
	Receiver       string  `json:"receiver"`
	Listener       string  `json:"listener"`
	ListenerReason *string `json:"listener_reason"`
	EngineMode     string  `json:"engine_mode"`
	Port           int     `json:"port"`
	Endpoint       string  `json:"endpoint"`
	// ProducerConfigured is true/false only from the explicit readable check
	// of the Codex configuration, unknown otherwise.
	ProducerConfigured   string             `json:"producer_configured"`
	Reception            string             `json:"reception"`
	LastReceivedAt       *string            `json:"last_received_at"`
	LastRefusedAt        *string            `json:"last_refused_at"`
	LastRefusal          *string            `json:"last_refusal"`
	Metering             string             `json:"metering"`
	ReasonCode           *string            `json:"reason_code"`
	PendingObservations  int                `json:"pending_observations"`
	CountedObservations  int                `json:"counted_observations"`
	BackgroundSessions   int                `json:"background_sessions"`
	ConflictObservations int                `json:"conflict_observations"`
	InvalidObservations  int                `json:"invalid_observations"`
	HistoricalCoverage   string             `json:"historical_coverage"`
	CodexMemories        string             `json:"codex_memories"`
	CodexExporter        string             `json:"codex_exporter"`
	Coverage             []CoverageInterval `json:"coverage"`
	Gaps                 []CoverageInterval `json:"gaps"`
	// Warnings go to the envelope, not the data object.
	Warnings []string `json:"-"`
}

// CoverageInterval is one recorded listening interval or known gap. An open
// interval has a null end; a crashed listener's end is "unknown".
type CoverageInterval struct {
	StartedAt string  `json:"started_at"`
	EndedAt   *string `json:"ended_at"`
	Reason    *string `json:"reason"`
}

// ReadStatus assembles telemetry status from the receiver configuration, the
// listener's self-description, the Codex configuration facts and stored
// observations. It never starts a listener and never reads the token.
func ReadStatus(ctx context.Context, stateRoot, home string, open func() (*store.Store, error)) (Status, error) {
	config, err := LoadConfig(stateRoot)
	if err != nil {
		return Status{}, err
	}
	listener, err := ReadListenerState(stateRoot)
	if err != nil {
		return Status{}, err
	}
	facts := ReadCodexFacts(home, config.Port)
	status := Status{
		Receiver:           map[bool]string{true: "enabled", false: "disabled"}[config.Enabled],
		Listener:           listener.State,
		EngineMode:         "none",
		Port:               config.Port,
		Endpoint:           Endpoint(config.Port),
		ProducerConfigured: producerConfigured(facts.Exporter),
		Reception:          "unknown",
		HistoricalCoverage: "unknown",
		CodexMemories:      facts.Memories,
		CodexExporter:      facts.Exporter,
		Coverage:           []CoverageInterval{},
		Gaps:               []CoverageInterval{},
	}
	switch {
	case listener.State == "running" && listener.Owner == "app":
		status.EngineMode = "app"
	case listener.State == "running":
		status.EngineMode = "foreground"
	}
	if listener.Error != "" {
		status.ListenerReason = stringPointer(listener.Error)
	} else if config.Enabled && listener.State != "running" && facts.CollectionBlocked() {
		status.ListenerReason = stringPointer(ReasonMemoriesOff)
	}
	if !config.Enabled && facts.NeedsCollection() {
		status.Warnings = append(status.Warnings, WarningMemoriesNotCounted)
	}
	if listener.LastRefusedAt != "" {
		status.LastRefusedAt, status.LastRefusal = stringPointer(listener.LastRefusedAt), stringPointer(listener.LastRefusal)
	}
	summary := store.TelemetrySummary{}
	if _, err := os.Stat(filepath.Join(stateRoot, "agentdeck.sqlite3")); err == nil {
		database, err := open()
		if err != nil {
			return Status{}, err
		}
		defer database.Close()
		if summary, err = database.TelemetrySummary(ctx); err != nil {
			return Status{}, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Status{}, err
	}
	capability, reason := MeteringCapability(config.Enabled, summary.LastSupportEntry, summary.LastReceivedAt != "")
	status.Metering = string(capability)
	if reason != "" {
		status.ReasonCode = stringPointer(reason)
	}
	status.PendingObservations = summary.Pending + summary.CaptureOnly
	status.CountedObservations = summary.OTelOwned
	status.BackgroundSessions = summary.OwnedSessions
	status.ConflictObservations = summary.Conflict
	status.InvalidObservations = summary.Invalid
	if summary.LastReceivedAt != "" {
		status.LastReceivedAt = stringPointer(summary.LastReceivedAt)
		status.Reception = "received"
	}
	for _, interval := range summary.Coverage {
		item := CoverageInterval{StartedAt: interval.StartedAt}
		if interval.EndedAt != "" {
			item.EndedAt = stringPointer(interval.EndedAt)
		}
		if interval.Reason != "" {
			item.Reason = stringPointer(interval.Reason)
		}
		if interval.Kind == "gap" {
			status.Gaps = append(status.Gaps, item)
		} else {
			status.Coverage = append(status.Coverage, item)
		}
	}
	return status, nil
}

// ReasonMemoriesOff is the listener stop reason under the linkage.
const ReasonMemoriesOff = "codex_memories_off"

func producerConfigured(exporter string) string {
	switch exporter {
	case ExporterManaged, ExporterAgentDeck:
		return "true"
	case ExporterAbsent, ExporterOther:
		return "false"
	}
	return "unknown"
}

// Endpoint is the loopback logs endpoint for a port.
func Endpoint(port int) string {
	return "http://127.0.0.1:" + strconv.Itoa(port) + LogsPath
}

func stringPointer(s string) *string { return &s }
