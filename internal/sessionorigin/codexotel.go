package sessionorigin

import (
	"strconv"
	"strings"

	"github.com/kitdine/agent-deck/internal/store"
)

const (
	SourceCodexOTelBackground = "codex-otel-background"
	SourceClaudeMem           = "claude-mem"
)

// BuiltinRule describes an ordered, versioned classification decision.
// Defaults are results, never purpose evidence that conflicts with auxiliary.
type BuiltinRule struct {
	ID        string
	Priority  int
	Class     string
	Rationale string
	matches   func(Assessment) bool
}

var builtinRules = []BuiltinRule{
	{RuleClaudeMemSDKMapV1, 10, ClassAuxiliary, "Exact mapping from the enabled optional claude-mem adapter.", func(a Assessment) bool {
		for _, e := range a.Evidence {
			if e.RuleVersion == RuleClaudeMemSDKMapV1 && e.Purpose == PurposeAuxiliary {
				return true
			}
		}
		return false
	}},
	{RuleCodexOTelTUIBackgroundV1, 20, ClassAuxiliary, "Inferred from an OTel-owned Codex TUI conversation without JSONL; a rollout-less main conversation can also match.", func(a Assessment) bool {
		for _, e := range a.Evidence {
			if e.RuleVersion == RuleCodexOTelTUIBackgroundV1 && e.Purpose == PurposeAuxiliary {
				return true
			}
		}
		return false
	}},
	{RuleCodexOTelExecWorkV1, 30, ClassWork, "Work by default for an OTel-owned Codex exec conversation; background Memories work can also match.", func(a Assessment) bool { return a.FallbackRule == RuleCodexOTelExecWorkV1 }},
	{RuleDefaultWorkV1, 40, ClassWork, "Work by default when no other purpose rule applies.", func(Assessment) bool { return true }},
}

func BuiltinRules() []BuiltinRule { return append([]BuiltinRule(nil), builtinRules...) }
func BuiltinRuleIDs() []string {
	out := make([]string, 0, len(builtinRules))
	for _, r := range builtinRules {
		out = append(out, r.ID)
	}
	return out
}
func RuleRationale(id string) string {
	for _, r := range builtinRules {
		if r.ID == id {
			return r.Rationale
		}
	}
	return ""
}

// markerFamily uses the same family/version constraints as numerical admission.
func markerFamily(entry string) string {
	p := strings.Split(entry, "/")
	if len(p) != 4 || !store.TelemetryAdmitted(p[0], p[1], p[2], p[3], store.TelemetryAdmittedEventName, store.TelemetryAdmittedEventKind) {
		return ""
	}
	return p[0]
}
func CodexMarkerIsBackground(entry string) bool { return markerFamily(entry) == "codex-app-server" }

func AssessCodexMarker(marker store.TelemetrySessionOriginMarker) Assessment {
	if marker.SourceKind != store.TelemetrySessionSourceKind {
		return Assessment{}
	}
	switch markerFamily(marker.SupportEntry) {
	case "codex_exec":
		return Assessment{FallbackRule: RuleCodexOTelExecWorkV1}
	case "codex-app-server":
		return Assessment{Evidence: []Evidence{{Adapter: SourceCodexOTelBackground, RuleVersion: RuleCodexOTelTUIBackgroundV1, Purpose: PurposeAuxiliary, SourceToken: strconv.Quote(marker.SupportEntry) + "|" + strconv.Quote(marker.SettledAt), ValidatedVersion: marker.SupportEntry}}}
	default:
		return Assessment{Unsupported: RuleCodexOTelTUIBackgroundV1}
	}
}
