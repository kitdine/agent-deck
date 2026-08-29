package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/usage"
)

func signalTestInt(value int) *int           { return &value }
func signalTestFloat(value float64) *float64 { return &value }
func signalTestString(value string) *string  { return &value }

func signalTextFixture() usage.SignalReport {
	return usage.SignalReport{
		Period: "7d",
		Client: "all",
		Activity: &usage.SignalActivity{
			Available: true,
			CostBasis: usage.CostBasisTurn,
			Kinds: []usage.SignalActivityKind{{
				Kind: "coding", Share: 52, Cost: 2.74, Events: 21,
				Sub: []usage.SignalActivitySub{{Kind: "feature", Share: 24, Cost: 1.26, Events: 9}},
			}},
		},
		Workflow: &usage.SignalWorkflow{
			Available: true, FirstEditSeconds: signalTestInt(132), FilesTouched: signalTestInt(7),
			Retries: signalTestInt(0), EditsPerSession: signalTestFloat(4),
			TopFile: signalTestString("tasks.md"), TopFileEdits: signalTestInt(4),
		},
		Tooling: &usage.SignalTooling{
			Available: true, Calls: 82, Groups: 2,
			Rows:         []usage.SignalToolRow{{Kind: "edit", Calls: 50, Share: 61}, {Kind: "bash", Calls: 32, Share: 39}},
			TopMCPServer: "codegraph", TopMCPCalls: 5,
		},
	}
}

func TestRenderUsageSignalsTextUsesContractSectionsAndValues(t *testing.T) {
	var output bytes.Buffer
	if err := renderUsageSignalsWithOptions(&output, signalTextFixture(), usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{
		"🧭 ACTIVITY", "Coding", "52%", "$2.74", "21 events", "└ Feature", "24%",
		"🧱 WORKFLOW", "FIRST EDIT", "2m12s (median)", "FILES TOUCHED", "7", "REWORK", "0", "MOST TOUCHED", "tasks.md ×4",
		"🔧 TOOLING", "Edit", "50 calls", "61%", "TOP MCP", "codegraph · 5 calls",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("signals text missing %q:\n%s", want, text)
		}
	}
}

func TestRenderUsageSignalsTextKeepsUnavailableDistinctFromZero(t *testing.T) {
	report := usage.SignalReport{
		Period:   "today",
		Client:   "all",
		Activity: &usage.SignalActivity{Available: false},
		Workflow: &usage.SignalWorkflow{Available: false},
		Tooling:  &usage.SignalTooling{Available: false},
	}
	var output bytes.Buffer
	if err := renderUsageSignalsWithOptions(&output, report, usageTextRenderOptions{width: 80}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"No turn in the selected scope.", "No tool call in the selected scope.", "FIRST EDIT", "FILES TOUCHED", "REWORK", "EDITS / SESSION", "MOST TOUCHED", "—"} {
		if !strings.Contains(text, want) {
			t.Fatalf("empty signals text missing %q:\n%s", want, text)
		}
	}
}

func TestUsageStatsPlacesSignalsAfterHeatmapBeforeCoverage(t *testing.T) {
	report := usage.StatsReport{
		Range:    usage.StatsRange{From: "2026-08-27T00:00:00Z", To: "2026-08-28T00:00:00Z"},
		Timezone: "UTC", GroupBy: "hour", Metric: "tokens",
		Signals: func() *usage.SignalReport { value := signalTextFixture(); return &value }(),
	}
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	positions := []int{
		strings.Index(text, "▦ ACTIVITY BY WEEKDAY / HOUR"),
		strings.Index(text, "🧭 WORK KIND"),
		strings.Index(text, "🧱 WORKFLOW"),
		strings.Index(text, "🔧 TOOLING"),
		strings.Index(text, "COVERAGE"),
	}
	for index, position := range positions {
		if position < 0 || index > 0 && position <= positions[index-1] {
			t.Fatalf("section order %v is invalid:\n%s", positions, text)
		}
	}
}

func TestRenderSessionShowTextAddsSignalsLineOnlyWhenPresent(t *testing.T) {
	result := session.Result{
		Metadata: session.Metadata{Client: "claude", SessionID: "s", FirstAt: "2026-08-27T00:00:00Z", LastAt: "2026-08-27T00:04:00Z"},
		Signals:  &session.WorkSignals{Kind: "coding", CostBasis: usage.CostBasisTurn, ToolCalls: 12, FilesTouched: signalTestInt(3), FirstEditSeconds: signalTestInt(240)},
	}
	var output bytes.Buffer
	if err := renderSessionShowText(&output, result, nil, "", nil, nil, true, "", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "SIGNALS") || !strings.Contains(output.String(), "Coding · 12 tool calls · 3 files · first edit 4m") {
		t.Fatalf("session output has no signal summary:\n%s", output.String())
	}

	result.Signals = nil
	output.Reset()
	if err := renderSessionShowText(&output, result, nil, "", nil, nil, false, "", false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "SIGNALS") {
		t.Fatalf("session output rendered a missing signal row:\n%s", output.String())
	}
}
