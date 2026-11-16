package store

// Telemetry admission is the one definition of which producer events may be
// metered from OTel. It names producer families and one completed-usage event
// shape; the producer version is provenance only, so a Codex upgrade that
// keeps the shape needs no new entry.

// TelemetryProducerFamily is one service.name and originator pair whose
// completed-usage events an isolated producer experiment verified.
type TelemetryProducerFamily struct {
	Producer   string
	Originator string
}

// The admitted event shape and the producer-version bound, which is the
// telemetry decoder's field limit.
const (
	TelemetryAdmittedTransport      = "sse"
	TelemetryAdmittedEventName      = "codex.sse_event"
	TelemetryAdmittedEventKind      = "response.completed"
	TelemetryProducerVersionMaxSize = 256
)

// TelemetryAdmittedFamilies lists the admitted producer families. Others,
// such as the Desktop originator, stay capture_only.
func TelemetryAdmittedFamilies() []TelemetryProducerFamily {
	return []TelemetryProducerFamily{
		{Producer: "codex_exec", Originator: "codex_exec"},
		{Producer: "codex-app-server", Originator: "codex-tui"},
	}
}

// TelemetryAdmitted reports whether an event of this producer family, version
// and shape may be metered. The version must be present, within the field
// limit and free of the support-entry separator; it is never compared.
func TelemetryAdmitted(producer, version, originator, transport, eventName, eventKind string) bool {
	if transport != TelemetryAdmittedTransport || eventName != TelemetryAdmittedEventName || eventKind != TelemetryAdmittedEventKind {
		return false
	}
	if version == "" || len(version) > TelemetryProducerVersionMaxSize {
		return false
	}
	for i := 0; i < len(version); i++ {
		if version[i] == '/' {
			return false
		}
	}
	for _, family := range TelemetryAdmittedFamilies() {
		if producer == family.Producer && originator == family.Originator {
			return true
		}
	}
	return false
}
