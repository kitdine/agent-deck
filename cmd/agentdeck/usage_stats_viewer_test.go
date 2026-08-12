package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/usage"
)

func TestUsageStatsInteractivePreflightRejectsBeforeStateCreation(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	command := newUsageCommand(&commandOptions{stateDir: stateDir, format: "text", stdin: strings.NewReader(""), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}})
	command.SetArgs([]string{"stats", "--interactive"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "requires TTY stdin and stdout") {
		t.Fatalf("interactive preflight error = %v", err)
	}
	if _, err := os.Stat(stateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preflight created usage state: %v", err)
	}
}

func TestUsageStatsInteractiveRejectsJSONFormatBeforeStateCreation(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	command := newUsageCommand(&commandOptions{stateDir: stateDir, format: "json", stdin: strings.NewReader(""), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}})
	command.SetArgs([]string{"stats", "--interactive"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "requires text output") {
		t.Fatalf("interactive json rejection error = %v", err)
	}
	if _, err := os.Stat(stateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("json rejection created usage state: %v", err)
	}
}

// failingUsageViewerWriter fails every write after failAt, so a render can be
// interrupted at each of its own write boundaries.
type failingUsageViewerWriter struct {
	failAt  int
	written int
	err     error
}

func (w *failingUsageViewerWriter) Write(p []byte) (int, error) {
	w.written++
	if w.written > w.failAt {
		return 0, w.err
	}
	return len(p), nil
}

func TestUsageViewerRenderReportsEveryWriteFailure(t *testing.T) {
	report := usage.StatsReport{Metric: "tokens", Range: usage.StatsRange{From: "2026-08-01T00:00:00Z", To: "2026-08-02T00:00:00Z"}, Totals: usage.StatsTotals{Tokens: 100}, Coverage: usage.StatsCoverage{Percent: "50"}, Models: []usage.StatsDimension{{Name: "model", KnownShare: "100", Tokens: 100, Sessions: 1, Coverage: "50"}}}
	broken := errors.New("terminal write failed")
	for _, size := range [][2]int{{48, 10}, {140, 32}, {40, 9}} {
		state := newUsageViewerState(report, []string{"partial pricing"}, nil)
		state.section = usageViewerModels
		state.refresh()
		counter := &failingUsageViewerWriter{failAt: 1 << 30}
		if err := renderUsageStatsViewer(counter, size[0], size[1], state, usageTextPrimitives{}); err != nil {
			t.Fatalf("%dx%d baseline render: %v", size[0], size[1], err)
		}
		if counter.written == 0 {
			t.Fatalf("%dx%d render performed no writes", size[0], size[1])
		}
		for failAt := 0; failAt < counter.written; failAt++ {
			writer := &failingUsageViewerWriter{failAt: failAt, err: broken}
			if err := renderUsageStatsViewer(writer, size[0], size[1], state, usageTextPrimitives{}); !errors.Is(err, broken) {
				t.Fatalf("%dx%d write %d error = %v", size[0], size[1], failAt, err)
			}
		}
	}
}

func TestUsageViewerStateRetainsSectionLocalPageAndSelection(t *testing.T) {
	report := usage.StatsReport{Metric: "tokens", Models: make([]usage.StatsDimension, 21), Clients: []usage.StatsDimension{{Name: "codex"}}}
	for i := range report.Models {
		report.Models[i] = usage.StatsDimension{Name: string(rune('a' + i%26)), KnownShare: "1"}
	}
	state := newUsageViewerState(report, nil, nil)
	state.section = usageViewerModels
	state.refresh()
	if reload, _ := state.apply("page-down"); !reload {
		t.Fatal("page down did not reload")
	}
	state.refresh()
	if state.pages[usageViewerModels] != 2 {
		t.Fatalf("models page = %d, want 2", state.pages[usageViewerModels])
	}
	if reload, _ := state.apply("right"); !reload {
		t.Fatal("section change did not reload")
	}
	state.refresh()
	if reload, _ := state.apply("left"); !reload {
		t.Fatal("section return did not reload")
	}
	state.refresh()
	if state.pages[usageViewerModels] != 2 || state.selected[usageViewerModels] != 0 {
		t.Fatalf("models state = page %d selected %d", state.pages[usageViewerModels], state.selected[usageViewerModels])
	}
}

func TestUsageViewerStateKeymapChangesSelectedDetailAndExits(t *testing.T) {
	report := usage.StatsReport{Metric: "tokens", Models: make([]usage.StatsDimension, 21)}
	for i := range report.Models {
		report.Models[i] = usage.StatsDimension{Name: "model-" + string(rune('a'+i%26)), KnownShare: "1", Tokens: int64(i + 1)}
	}
	state := newUsageViewerState(report, nil, nil)
	state.section = usageViewerModels
	state.refresh()
	first := strings.Join(usageViewerDetail(state.rows[state.selected[state.section]], 80, usageTextPrimitives{}), "\n")
	state.apply("down")
	if strings.Join(usageViewerDetail(state.rows[state.selected[state.section]], 80, usageTextPrimitives{}), "\n") == first {
		t.Fatal("down did not change selected detail")
	}
	state.apply("end")
	if state.selected[state.section] != len(state.rows)-1 {
		t.Fatal("end did not select last row")
	}
	if reload, _ := state.apply("page-down"); !reload {
		t.Fatal("page down did not reload")
	}
	state.refresh()
	if state.pages[state.section] != 2 || state.selected[state.section] != 0 {
		t.Fatalf("page state = %d/%d", state.pages[state.section], state.selected[state.section])
	}
	if _, exit := state.apply("escape"); !exit {
		t.Fatal("escape did not exit")
	}
}

func TestStatsTitleKeepsFirstNonASCIICodePoint(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{in: "codex", want: "Codex"},
		{in: "模型甲", want: "模型甲"},
		{in: "😀 model", want: "😀 model"},
		{in: "étude", want: "Étude"},
		{in: "", want: ""},
		{in: "\xff raw", want: "\xff raw"},
	} {
		if got := statsTitle(tc.in); got != tc.want {
			t.Fatalf("statsTitle(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUsageViewerViewportRetainsWindowUntilSelectionLeavesIt(t *testing.T) {
	report := usage.StatsReport{Metric: "tokens", Models: make([]usage.StatsDimension, 20)}
	for i := range report.Models {
		report.Models[i] = usage.StatsDimension{Name: "model-" + string(rune('a'+i)), KnownShare: "1"}
	}
	state := newUsageViewerState(report, nil, nil)
	state.section = usageViewerModels
	state.refresh()

	// A window that already shows the selection does not move.
	state.viewports[usageViewerModels], state.selected[usageViewerModels] = 8, 10
	if got := state.viewportOffset(5); got != 8 {
		t.Fatalf("visible selection moved viewport to %d, want 8", got)
	}
	// Moving down past the last visible row shifts by the minimum amount.
	state.selected[usageViewerModels] = 13
	if got := state.viewportOffset(5); got != 9 {
		t.Fatalf("downward shift = %d, want 9", got)
	}
	// Moving above the window anchors the window on the selection.
	state.selected[usageViewerModels] = 4
	if got := state.viewportOffset(5); got != 4 {
		t.Fatalf("upward shift = %d, want 4", got)
	}
	// A viewport taller than the page collapses to the top.
	if got := state.viewportOffset(40); got != 0 {
		t.Fatalf("oversized viewport = %d, want 0", got)
	}
	// Shrinking must keep the selection visible; recovering returns to the
	// retained context rather than recentering.
	state.viewports[usageViewerModels], state.selected[usageViewerModels] = 12, 16
	if got := state.viewportOffset(3); got != 14 {
		t.Fatalf("shrunk viewport = %d, want 14", got)
	}
	if got := state.viewportOffset(6); got != 14 {
		t.Fatalf("recovered viewport = %d, want 14", got)
	}
	// An empty section has no window to retain.
	state.section = usageViewerClients
	state.refresh()
	if got := state.viewportOffset(5); got != 0 {
		t.Fatalf("empty section viewport = %d, want 0", got)
	}
}

func TestUsageViewerRenderRetainsViewportAcrossSectionsAndResize(t *testing.T) {
	report := usage.StatsReport{Metric: "tokens", Range: usage.StatsRange{From: "2026-08-01T00:00:00Z", To: "2026-08-02T00:00:00Z"}, Coverage: usage.StatsCoverage{Percent: "50"}, Models: make([]usage.StatsDimension, 21), Clients: []usage.StatsDimension{{Name: "codex", KnownShare: "100"}}}
	for i := range report.Models {
		report.Models[i] = usage.StatsDimension{Name: "model-" + string(rune('a'+i%26)), KnownShare: "1", Tokens: int64(i + 1)}
	}
	state := newUsageViewerState(report, nil, nil)
	state.section = usageViewerModels
	state.refresh()
	render := func(width, height int) {
		t.Helper()
		if err := renderUsageStatsViewer(&strings.Builder{}, width, height, state, usageTextPrimitives{}); err != nil {
			t.Fatal(err)
		}
	}
	render(140, 12)
	state.apply("end")
	render(140, 12)
	if state.viewports[usageViewerModels] != 14 {
		t.Fatalf("end viewport = %d, want 14", state.viewports[usageViewerModels])
	}
	// Selection up inside the visible window leaves the window in place.
	state.apply("up")
	render(140, 12)
	if state.viewports[usageViewerModels] != 14 || state.selected[usageViewerModels] != 18 {
		t.Fatalf("in-window up = viewport %d selected %d", state.viewports[usageViewerModels], state.selected[usageViewerModels])
	}
	// A section round trip returns to the same window.
	state.apply("right")
	state.refresh()
	render(140, 12)
	state.apply("left")
	state.refresh()
	render(140, 12)
	if state.viewports[usageViewerModels] != 14 || state.selected[usageViewerModels] != 18 {
		t.Fatalf("section round trip = viewport %d selected %d", state.viewports[usageViewerModels], state.selected[usageViewerModels])
	}
	// Shrinking shifts only as far as the selection requires; recovering grows
	// the retained window instead of recentering it on the selection.
	render(140, 10)
	if got := state.viewports[usageViewerModels]; got != 15 {
		t.Fatalf("shrunk render viewport = %d, want 15", got)
	}
	render(140, 12)
	if got := state.viewports[usageViewerModels]; got != 14 {
		t.Fatalf("recovered render viewport = %d, want 14", got)
	}
	if state.selected[usageViewerModels] != 18 {
		t.Fatalf("resize changed selection to %d", state.selected[usageViewerModels])
	}
	// A new page starts at the top of its own window.
	if reload, _ := state.apply("page-down"); !reload {
		t.Fatal("page down did not reload")
	}
	state.refresh()
	render(140, 12)
	if state.viewports[usageViewerModels] != 0 || state.selected[usageViewerModels] != 0 {
		t.Fatalf("page change = viewport %d selected %d", state.viewports[usageViewerModels], state.selected[usageViewerModels])
	}
}

func TestUsageViewerRenderNoColorKeepsSelectionAndWarnings(t *testing.T) {
	report := usage.StatsReport{Metric: "tokens", Range: usage.StatsRange{From: "2026-08-01T00:00:00Z", To: "2026-08-02T00:00:00Z"}, Totals: usage.StatsTotals{Tokens: 100, Sessions: 1}, Coverage: usage.StatsCoverage{Percent: "50"}, Models: []usage.StatsDimension{{Name: "模型😀", KnownShare: "100", Tokens: 100, Sessions: 1, Coverage: "50"}}}
	state := newUsageViewerState(report, []string{"partial pricing"}, nil)
	state.section = usageViewerModels
	state.refresh()
	var out strings.Builder
	if err := renderUsageStatsViewer(&out, 48, 10, state, usageTextPrimitives{}); err != nil {
		t.Fatal(err)
	}
	plain := stripStatsANSI(out.String())
	for _, want := range []string{"MODELS", ">", "模型😀", "DETAIL", "warning", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("render missing %q:\n%s", want, plain)
		}
	}
	if regexp.MustCompile(`\x1b\[[0-9;]+m`).MatchString(out.String()) {
		t.Fatalf("no-color render contains ANSI color: %q", out.String())
	}
}

func TestUsageViewerDetailUsesSemanticColorAndOmitsEmptyContent(t *testing.T) {
	known := "1.250000000"
	catalog := "1.500000000"
	average := "0.625000000"
	report := usage.StatsReport{
		Metric: "tokens",
		Range:  usage.StatsRange{From: "2026-08-01T00:00:00Z", To: "2026-08-02T00:00:00Z"},
		Totals: usage.StatsTotals{
			Tokens:               100,
			InputTokens:          80,
			OutputTokens:         20,
			Sessions:             2,
			Events:               4,
			KnownProviderCost:    known,
			KnownCatalogBaseCost: catalog,
			KnownAverageCost:     average,
			AverageTokens:        "50",
		},
		Coverage: usage.StatsCoverage{PricedEvents: 4, UnpricedEvents: 1, TotalEvents: 5, Percent: "80"},
	}
	state := newUsageViewerState(report, nil, nil)
	var colorful strings.Builder
	if err := renderUsageStatsViewer(&colorful, 80, 24, state, usageTextPrimitives{color: true}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DETAIL · TOKENS", "INPUT TOKENS", "AVERAGE TOKENS / SESSION", "\x1b[1;94m", "\x1b[1;96m80\x1b[0m"} {
		if !strings.Contains(colorful.String(), want) {
			t.Fatalf("color detail missing %q:\n%s", want, colorful.String())
		}
	}
	plainState := newUsageViewerState(report, nil, nil)
	var plainFrame strings.Builder
	if err := renderUsageStatsViewer(&plainFrame, 80, 24, plainState, usageTextPrimitives{}); err != nil {
		t.Fatal(err)
	}
	if stripped, plain := stripStatsANSI(colorful.String()), stripStatsANSI(plainFrame.String()); stripped != plain {
		t.Fatalf("color and no-color Usage frames differ:\ncolor stripped:\n%s\nplain:\n%s", stripped, plain)
	}

	state.selected[usageViewerOverview] = 1
	colorful.Reset()
	if err := renderUsageStatsViewer(&colorful, 80, 24, state, usageTextPrimitives{color: true}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DETAIL · COST", "PARTIAL", "UNPRICED EVENTS", "\x1b[1;93m$1.500000000 KNOWN\x1b[0m"} {
		if !strings.Contains(colorful.String(), want) {
			t.Fatalf("cost detail missing %q:\n%s", want, colorful.String())
		}
	}
	if strings.Contains(colorful.String(), "$1.250000000 KNOWN") {
		t.Fatalf("cost detail repeated the selected provider-cost value:\n%s", colorful.String())
	}

	var short strings.Builder
	if err := renderUsageStatsViewer(&short, 48, 10, state, usageTextPrimitives{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DETAIL · COST", "PARTIAL", "Provider cost includes priced events"} {
		if !strings.Contains(short.String(), want) {
			t.Fatalf("48x10 cost detail lost %q:\n%s", want, short.String())
		}
	}

	state.section = usageViewerCoverage
	state.refresh()
	if len(state.rows) != 2 {
		t.Fatalf("coverage rows = %d, want priced and unpriced only when there are no warnings", len(state.rows))
	}
	warningState := newUsageViewerState(report, []string{"Catalog refresh is stale."}, nil)
	warningState.section = usageViewerCoverage
	warningState.refresh()
	if len(warningState.rows) != 3 {
		t.Fatalf("coverage rows = %d, want required warning row", len(warningState.rows))
	}
	warningState.selected[usageViewerCoverage] = 2
	warningFrame := strings.Join(usageViewerDetail(warningState.rows[2], 80, usageTextPrimitives{color: true}), "\n")
	if !strings.Contains(warningFrame, "\x1b[1;93mWARNING · Catalog refresh is stale.\x1b[0m") {
		t.Fatalf("warning detail lost explicit warning role:\n%s", warningFrame)
	}
	unavailableCost := newUsageViewerState(usage.StatsReport{Metric: "cost", Range: report.Range, Coverage: usage.StatsCoverage{Percent: "0"}}, nil, nil)
	unavailableCost.selected[usageViewerOverview] = 1
	unavailableFrame := strings.Join(usageViewerDetail(unavailableCost.rows[1], 80, usageTextPrimitives{color: true}), "\n")
	if !strings.Contains(unavailableFrame, "\x1b[1;93mUNAVAILABLE · No priced events.\x1b[0m") {
		t.Fatalf("unavailable cost detail lost required pricing state:\n%s", unavailableFrame)
	}

	empty := newUsageViewerState(usage.StatsReport{
		Metric:   "tokens",
		Range:    report.Range,
		Totals:   usage.StatsTotals{Tokens: 100},
		Coverage: usage.StatsCoverage{Percent: "0"},
	}, nil, nil)
	var plain strings.Builder
	if err := renderUsageStatsViewer(&plain, 80, 24, empty, usageTextPrimitives{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain.String(), "DETAIL · TOKENS") {
		t.Fatalf("empty supplementary content rendered a detail block:\n%s", plain.String())
	}
}

func TestUsageViewerDimensionDetailExposesCacheAccounting(t *testing.T) {
	rate := "62.5"
	cached := usage.StatsDimension{Name: "cached", KnownShare: "60", Tokens: 1000, Sessions: 2, Coverage: "100", CachedReadTokens: 800, CacheWriteTokens: 200, LogicalInputTokens: 1200, CacheHitRate: &rate}
	plain := usage.StatsDimension{Name: "plain", KnownShare: "40", Tokens: 500, Sessions: 1, Coverage: "100"}
	report := usage.StatsReport{
		Metric:    "tokens",
		Range:     usage.StatsRange{From: "2026-08-01T00:00:00Z", To: "2026-08-02T00:00:00Z"},
		Totals:    usage.StatsTotals{Tokens: 1500, Sessions: 3},
		Coverage:  usage.StatsCoverage{Percent: "100"},
		Models:    []usage.StatsDimension{cached, plain},
		Clients:   []usage.StatsDimension{cached, plain},
		Providers: []usage.StatsDimension{{Name: "cached", Client: "codex", KnownShare: "60", Tokens: 1000, Sessions: 2, Coverage: "100", CachedReadTokens: 800, CacheWriteTokens: 200, LogicalInputTokens: 1200, CacheHitRate: &rate}, plain},
	}
	wantFields := map[string]struct {
		value string
		role  terminalDetailRole
	}{
		"TOKENS":           {value: "1.0K", role: terminalDetailRoleToken},
		"SESSIONS":         {value: "2", role: terminalDetailRoleSession},
		"PRICING COVERAGE": {value: "100%", role: terminalDetailRoleSuccess},
		"CACHE HIT RATE":   {value: "62.5%", role: terminalDetailRoleSuccess},
		"CACHE READ":       {value: "800", role: terminalDetailRoleToken},
		"CACHE WRITE":      {value: "200", role: terminalDetailRoleToken},
		"LOGICAL INPUT":    {value: "1.2K", role: terminalDetailRoleToken},
	}
	cacheLabels := []string{"CACHE HIT RATE", "CACHE READ", "CACHE WRITE", "LOGICAL INPUT"}
	for name, detail := range map[string]terminalDetailModel{
		"dimension": usageViewerDimensionDetail(usage.StatsDimension{CachedReadTokens: 800}),
		"cache":     usageViewerCacheDetail(usage.StatsCacheSession{CachedReadTokens: 800}),
	} {
		labels := make(map[string]bool, len(detail.fields))
		for _, field := range detail.fields {
			labels[field.label] = true
		}
		if !labels["CACHE READ"] || labels["CACHE WRITE"] {
			t.Fatalf("%s one-sided cache fields = %#v, want read without zero write", name, detail.fields)
		}
	}
	for _, section := range []usageViewerSection{usageViewerModels, usageViewerClients, usageViewerProviders} {
		state := newUsageViewerState(report, nil, nil)
		state.section = section
		state.refresh()
		row := state.rows[state.selected[section]]
		fields := make(map[string]terminalDetailField, len(row.detail.fields))
		for _, field := range row.detail.fields {
			fields[field.label] = field
		}
		for label, want := range wantFields {
			got, ok := fields[label]
			if !ok || got.value != want.value || got.role != want.role {
				t.Fatalf("section %d field %q = %#v, want value %q role %d", section, label, got, want.value, want.role)
			}
		}
		joined := strings.Join(usageViewerDetail(row, 80, usageTextPrimitives{}), "\n")
		// Selection drives detail, and a dimension without cache accounting
		// exposes no fabricated cache field.
		state.apply("down")
		otherRow := state.rows[state.selected[section]]
		other := strings.Join(usageViewerDetail(otherRow, 80, usageTextPrimitives{}), "\n")
		if other == joined {
			t.Fatalf("section %d selection did not change detail", section)
		}
		for _, unwanted := range cacheLabels {
			for _, field := range otherRow.detail.fields {
				if field.label == unwanted {
					t.Fatalf("section %d uncached detail contains %q: %#v", section, unwanted, otherRow.detail.fields)
				}
			}
		}
		// The wide frame renders the selected detail beside the list.
		var out strings.Builder
		if err := renderUsageStatsViewer(&out, 140, 32, state, usageTextPrimitives{}); err != nil {
			t.Fatal(err)
		}
		plainFrame := stripStatsANSI(out.String())
		if !strings.Contains(plainFrame, "TOKENS") || !strings.Contains(plainFrame, "500") {
			t.Fatalf("section %d frame lost selected detail:\n%s", section, out.String())
		}
	}
}

func TestUsageViewerRenderFitsRequiredGeometriesAndEmptySections(t *testing.T) {
	report := usage.StatsReport{Metric: "tokens", Range: usage.StatsRange{From: "2026-08-01T00:00:00Z", To: "2026-08-02T00:00:00Z"}, Totals: usage.StatsTotals{Tokens: 100}, Coverage: usage.StatsCoverage{Percent: "50"}, Models: []usage.StatsDimension{{Name: "模型😀", KnownShare: "100", Tokens: 100, Sessions: 1, Coverage: "50"}}}
	for _, size := range [][2]int{{48, 10}, {60, 18}, {80, 24}, {100, 24}, {120, 32}, {140, 32}, {180, 40}} {
		t.Run("geometry", func(t *testing.T) {
			state := newUsageViewerState(report, []string{"partial pricing"}, nil)
			state.section = usageViewerModels
			state.refresh()
			var out strings.Builder
			if err := renderUsageStatsViewer(&out, size[0], size[1], state, usageTextPrimitives{}); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
			if len(lines) > size[1] {
				t.Fatalf("%dx%d emitted %d lines", size[0], size[1], len(lines))
			}
			for _, line := range lines {
				if statsVisibleWidth(line) > size[0] {
					t.Fatalf("%dx%d overflow %q", size[0], size[1], line)
				}
			}
			plain := stripStatsANSI(out.String())
			for _, want := range []string{">", "warning", "page"} {
				if !strings.Contains(plain, want) {
					t.Fatalf("%dx%d missing %q", size[0], size[1], want)
				}
			}
		})
	}
	state := newUsageViewerState(usage.StatsReport{Metric: "tokens"}, nil, nil)
	state.section = usageViewerModels
	state.refresh()
	var out strings.Builder
	if err := renderUsageStatsViewer(&out, 48, 10, state, usageTextPrimitives{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stripStatsANSI(out.String()), "row 1/0") || !strings.Contains(stripStatsANSI(out.String()), "no selection") {
		t.Fatalf("empty section footer = %q", out.String())
	}
}

func TestUsageViewerRowsOnlyShowBarsWithDefinedBasis(t *testing.T) {
	report := usage.StatsReport{
		Metric:   "tokens",
		Coverage: usage.StatsCoverage{Percent: "0", TotalEvents: 0},
		Totals:   usage.StatsTotals{Tokens: 100, Sessions: 1},
		Buckets: []usage.StatsBucket{
			{Start: "zero", KnownMetricValue: "0"},
			{Start: "invalid", KnownMetricValue: "not-a-number"},
		},
		Models:        []usage.StatsDimension{{Name: "known", Client: "codex", KnownShare: "100"}, {Name: "unknown", Client: "claude", KnownShare: ""}},
		CacheSessions: []usage.StatsCacheSession{{Client: "codex", SessionID: "unknown"}},
	}
	state := newUsageViewerState(report, nil, nil)
	for _, row := range state.rows {
		if row.showBar {
			t.Fatalf("overview row %q exposes a bar without a comparison basis", row.label)
		}
	}

	state.section = usageViewerTrend
	state.refresh()
	for _, row := range state.rows {
		if row.showBar {
			t.Fatalf("zero/invalid trend row %q exposes a bar without a peak", row.label)
		}
	}
	report.Buckets[0].KnownMetricValue = "10"
	state = newUsageViewerState(report, nil, nil)
	state.section = usageViewerTrend
	state.refresh()
	if !state.rows[0].showBar {
		t.Fatal("trend row with a positive peak lost its comparison bar")
	}

	state.section = usageViewerModels
	state.refresh()
	if !state.rows[0].showBar || state.rows[1].showBar {
		t.Fatalf("model share bars = %v/%v, want true/false", state.rows[0].showBar, state.rows[1].showBar)
	}
	state.section = usageViewerCache
	state.refresh()
	if state.rows[0].showBar {
		t.Fatal("cache row without a hit-rate denominator exposes a bar")
	}
	state.section = usageViewerCoverage
	state.refresh()
	if state.rows[0].showBar || state.rows[1].showBar {
		t.Fatal("empty coverage rows expose bars without total events")
	}
}

func TestUsageViewerActivityRendersEveryHourWithSemanticHeatmapPalette(t *testing.T) {
	report := usage.StatsReport{
		Metric: "tokens",
		Range:  usage.StatsRange{From: "2026-08-01T00:00:00Z", To: "2026-08-08T00:00:00Z"},
		Activity: []usage.StatsActivity{
			{Weekday: 0, Hour: 0, KnownMetricValue: "1"},
			{Weekday: 0, Hour: 1, KnownMetricValue: "2"},
			{Weekday: 0, Hour: 2, KnownMetricValue: "3"},
			{Weekday: 0, Hour: 3, KnownMetricValue: "4"},
		},
	}
	state := newUsageViewerState(report, nil, nil)
	state.section = usageViewerActivity
	state.refresh()
	if len(state.rows) != 7 {
		t.Fatalf("activity rows = %d, want 7", len(state.rows))
	}
	for _, row := range state.rows {
		if len(row.heatmap) != 24 {
			t.Fatalf("activity row %q buckets = %d, want 24", row.label, len(row.heatmap))
		}
	}

	var colored strings.Builder
	if err := renderUsageStatsViewer(&colored, 80, 24, state, usageTextPrimitives{color: true}); err != nil {
		t.Fatal(err)
	}
	plain := stripStatsANSI(colored.String())
	for _, want := range []string{"[ACTIVITY]", "1H BUCKET", "LESS", "MORE", "Mon", "Sun", "·", "░", "▒", "▓", "█"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("activity frame missing %q:\n%s", want, plain)
		}
	}
	for _, code := range []string{"\x1b[1;94m", "\x1b[1;96m", "\x1b[1;92m", "\x1b[1;93m"} {
		if !strings.Contains(colored.String(), code) {
			t.Fatalf("activity frame missing palette code %q: %q", code, colored.String())
		}
	}
	if strings.Contains(colored.String(), "\x1b[1;91m") {
		t.Fatalf("activity intensity used error red: %q", colored.String())
	}

	var noColor strings.Builder
	if err := renderUsageStatsViewer(&noColor, 48, 10, state, usageTextPrimitives{}); err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`\x1b\[[0-9;]+m`).MatchString(noColor.String()) {
		t.Fatalf("no-color activity frame contains SGR: %q", noColor.String())
	}
	for _, want := range []string{"1H BUCKET", "·", "░", "▒", "▓", "█"} {
		if !strings.Contains(noColor.String(), want) {
			t.Fatalf("no-color activity frame missing %q: %q", want, noColor.String())
		}
	}
}

func TestUsageViewerWideOverviewUsesContentHeightAndKeepsActiveTabVisible(t *testing.T) {
	complete := "1.000000000"
	report := usage.StatsReport{
		Metric:   "tokens",
		Range:    usage.StatsRange{From: "2026-08-01T00:00:00Z", To: "2026-08-02T00:00:00Z"},
		Totals:   usage.StatsTotals{Tokens: 100, InputTokens: 80, OutputTokens: 20, Sessions: 1, ProviderCost: &complete, KnownProviderCost: complete},
		Coverage: usage.StatsCoverage{PricedEvents: 1, TotalEvents: 1, Percent: "100"},
	}
	state := newUsageViewerState(report, nil, nil)
	var out strings.Builder
	if err := renderUsageStatsViewer(&out, 140, 32, state, usageTextPrimitives{color: true}); err != nil {
		t.Fatal(err)
	}
	plain := stripStatsANSI(out.String())
	selected := usageViewerSelectedRow(t, out.String())
	if strings.ContainsAny(selected, "█·") {
		t.Fatalf("absolute overview KPI rendered a meaningless track: %q", selected)
	}
	if !strings.Contains(plain, "\nDETAIL · TOKENS\n") {
		t.Fatalf("short wide overview did not use sequential full-width layout:\n%s", plain)
	}

	state.section = usageViewerCoverage
	state.refresh()
	if tabs := stripStatsANSI(usageViewerTabLine(state.section, 48, usageTextPrimitives{color: true})); !strings.Contains(tabs, "[COVERAGE]") {
		t.Fatalf("narrow tab window hid active Coverage tab: %q", tabs)
	}
}

// usageViewerSelectedRow returns the selected-row line of a rendered frame with
// the viewer's own style sequences removed.
func usageViewerSelectedRow(t *testing.T, frame string) string {
	t.Helper()
	for _, line := range strings.Split(stripStatsANSI(frame), "\n") {
		if strings.HasPrefix(line, "> ") {
			return line
		}
	}
	t.Fatalf("frame has no selected row:\n%s", frame)
	return ""
}

func TestUsageViewerSanitizesUntrustedLabelsAndKeepsVisibleIdentity(t *testing.T) {
	cases := []struct {
		name  string
		label string
		want  string
	}{
		{name: "cjk", label: "模型甲", want: "模型甲"},
		{name: "emoji", label: "😀 model", want: "😀 model"},
		{name: "combining", label: "e\u0301tude", want: "e\u0301tude"},
		{name: "csi", label: "mo\x1b[31mdel", want: "mo del"},
		{name: "osc", label: "mo\x1b]0;pwn\x07del", want: "mo del"},
		{name: "c1-csi", label: "mo\u009b31mdel", want: "mo del"},
		{name: "c0", label: "mo\x01\ndel", want: "mo  del"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := usage.StatsReport{Metric: "tokens", Range: usage.StatsRange{From: "2026-08-01T00:00:00Z", To: "2026-08-02T00:00:00Z"}, Totals: usage.StatsTotals{Tokens: 100, Sessions: 1}, Coverage: usage.StatsCoverage{Percent: "50"}, Models: []usage.StatsDimension{{Name: tc.label, KnownShare: "100", Tokens: 100, Sessions: 1, Coverage: "50"}}}
			state := newUsageViewerState(report, nil, nil)
			state.section = usageViewerModels
			state.refresh()
			var out strings.Builder
			if err := renderUsageStatsViewer(&out, 60, 18, state, usageTextPrimitives{color: true}); err != nil {
				t.Fatal(err)
			}
			frame := out.String()
			if row := usageViewerSelectedRow(t, frame); !strings.HasPrefix(row, "> "+tc.want+" ") {
				t.Fatalf("selected row = %q, want label %q verbatim", row, tc.want)
			}
		plain := stripStatsANSI(frame)
		detailTitle := ""
		for _, line := range strings.Split(plain, "\n") {
			if strings.HasPrefix(line, "DETAIL · ") {
				detailTitle = line
				break
			}
		}
		gotTitle := strings.Join(strings.Fields(detailTitle), " ")
		wantTitle := strings.Join(strings.Fields("DETAIL · "+tc.want), " ")
		if gotTitle != wantTitle {
			t.Fatalf("detail title missing %q:\n%s", tc.want, plain)
		}
			for _, r := range plain {
				if r != '\n' && (r < 0x20 || r >= 0x7f && r <= 0x9f) {
					t.Fatalf("frame retains untrusted control %#U:\n%q", r, plain)
				}
			}
			// The viewer's own screen and style control stays intact.
			for _, want := range []string{"\x1b[H\x1b[2J", "\x1b[1;96m"} {
				if !strings.Contains(frame, want) {
					t.Fatalf("frame lost viewer control %q: %q", want, frame)
				}
			}
		})
	}
}

func TestUsageViewerTooSmallFrameFitsAndPreservesState(t *testing.T) {
	state := newUsageViewerState(usage.StatsReport{Metric: "tokens", Models: []usage.StatsDimension{{Name: "model", KnownShare: "100"}}}, nil, nil)
	state.section = usageViewerModels
	state.refresh()
	beforeSection, beforePage, beforeSelected := state.section, state.pages[state.section], state.selected[state.section]
	var out strings.Builder
	if err := renderUsageStatsViewer(&out, 40, 9, state, usageTextPrimitives{}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) > 9 {
		t.Fatalf("too-small frame emitted %d lines", len(lines))
	}
	for _, line := range lines {
		if statsVisibleWidth(line) > 40 {
			t.Fatalf("too-small frame overflow %q", line)
		}
	}
	if !strings.Contains(stripStatsANSI(out.String()), "Terminal too small") {
		t.Fatalf("missing too-small frame: %q", out.String())
	}
	if state.section != beforeSection || state.pages[state.section] != beforePage || state.selected[state.section] != beforeSelected {
		t.Fatal("too-small render mutated viewer state")
	}
}
