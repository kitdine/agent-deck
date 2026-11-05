package sessionorigin

import (
	"strconv"

	"github.com/kitdine/agent-deck/internal/store"
)

// Built-in and adapter source names, as stored in origin_evidence.adapter.
const (
	SourceCodexOTelBackground = "codex-otel-background"
	SourceClaudeMem           = "claude-mem"
)

// codexOTelBackgroundEntries are the support entries, by SupportEntry.Name(),
// whose rollout-less conversations an isolated producer experiment
// (2026-10-29) showed carry only background requests: exec Memories
// consolidation, and TUI Memories consolidation and thread-title generation.
// An entry joins only with a new rule version after the same proof.
var codexOTelBackgroundEntries = map[string]bool{
	"codex_exec/0.165.0/codex_exec/sse":      true,
	"codex-app-server/0.165.0/codex-tui/sse": true,
}

// CodexOTelBackgroundEntries lists the entries rule codex-otel-background-v1 covers.
func CodexOTelBackgroundEntries() []string {
	return []string{"codex_exec/0.165.0/codex_exec/sse", "codex-app-server/0.165.0/codex-tui/sse"}
}

// AssessCodexMarker applies rule codex-otel-background-v1 to one marker of a
// codex session. A marker inside the rule's entries is maintenance evidence
// whose source token is the entry and its settle time; any other marker is
// unsupported. Originator, model, the otel: prefix, an empty project or time
// proximity are never used.
func AssessCodexMarker(marker store.TelemetrySessionOriginMarker) Assessment {
	if marker.SourceKind != store.TelemetrySessionSourceKind {
		return Assessment{}
	}
	if !codexOTelBackgroundEntries[marker.SupportEntry] {
		return Assessment{Unsupported: RuleCodexOTelBackgroundV1}
	}
	return Assessment{Evidence: []Evidence{{
		Adapter:          SourceCodexOTelBackground,
		RuleVersion:      RuleCodexOTelBackgroundV1,
		Purpose:          PurposeAuxiliary,
		SourceToken:      strconv.Quote(marker.SupportEntry) + "|" + strconv.Quote(marker.SettledAt),
		ValidatedVersion: marker.SupportEntry,
	}}}
}
