package main

import (
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/usage"
)

func TestUsageStatsMaximumColumnsFollowApprovedWidthBands(t *testing.T) {
	for _, test := range []struct {
		width int
		want  int
	}{
		{48, 1}, {119, 1}, {120, 2}, {179, 2},
		{180, 3}, {239, 3}, {240, 4}, {260, 4},
	} {
		if got := statsMaximumColumns(test.width); got != test.want {
			t.Fatalf("width %d maximum columns = %d, want %d", test.width, got, test.want)
		}
	}
}

func TestUsageStatsPreferredPanelMappings(t *testing.T) {
	wants := [][][]string{
		nil,
		{{"trend", "models", "clients", "providers", "cache", "coverage"}},
		{{"trend", "clients", "coverage"}, {"models", "providers", "cache"}},
		{{"trend", "coverage"}, {"models", "clients"}, {"providers", "cache"}},
		{{"trend"}, {"models"}, {"clients", "providers"}, {"cache", "coverage"}},
	}
	for columns := 1; columns <= 4; columns++ {
		got := statsPanelGroups(columns)
		if strings.Join(flattenStatsPanelGroups(got), ",") != strings.Join(flattenStatsPanelGroups(wants[columns]), ",") {
			t.Fatalf("%d-column mapping = %#v, want %#v", columns, got, wants[columns])
		}
	}
}

func flattenStatsPanelGroups(groups [][]string) []string {
	var flattened []string
	for index, group := range groups {
		flattened = append(flattened, strings.Join(group, "+"))
		if index < len(groups)-1 {
			flattened = append(flattened, "|")
		}
	}
	return flattened
}

func TestUsageStatsResponsiveLayoutHonorsBalanceAndValueThresholds(t *testing.T) {
	report := usageStatsTextFixture()
	for _, width := range []int{48, 119, 120, 179, 180, 239, 240, 260} {
		renderer := statsTextRenderer{report: report, width: width}
		layout := renderer.responsiveLayout()
		if got, maximum := len(layout.columns), statsMaximumColumns(width); got > maximum || got < 1 {
			t.Fatalf("width %d columns = %d, want 1..%d", width, got, maximum)
		}
		if len(layout.columns) > 1 {
			if layout.shortest*100 < layout.tallest*60 {
				t.Fatalf("width %d heights %d/%d violate 60%% balance", width, layout.shortest, layout.tallest)
			}
			previous := renderer.responsiveLayoutFor(len(layout.columns) - 1)
			if layout.tallest*100 > previous.tallest*85 {
				t.Fatalf("width %d tallest %d did not improve at least 15%% over %d", width, layout.tallest, previous.tallest)
			}
		}
		for _, line := range joinStatsColumns(layout.columns, layout.widths, statsResponsiveColumnGap) {
			if statsVisibleWidth(line) > width {
				t.Fatalf("width %d grid line is %d cells: %q", width, statsVisibleWidth(line), line)
			}
		}
	}
}

func TestUsageStatsResponsiveKPIRowsFollowWidthBands(t *testing.T) {
	for _, test := range []struct {
		width int
		lines int
	}{
		{100, 10},
		{140, 7},
		{180, 4},
	} {
		renderer := statsTextRenderer{report: usageStatsTextFixture(), width: test.width}
		lines := renderer.responsiveKPILines()
		if len(lines) != test.lines {
			t.Fatalf("width %d KPI lines = %d, want %d", test.width, len(lines), test.lines)
		}
		for _, line := range lines {
			if statsVisibleWidth(line) != test.width {
				t.Fatalf("width %d KPI line = %d cells: %q", test.width, statsVisibleWidth(line), line)
			}
		}
	}
}

func TestUsageStatsTrendFoldsThreeOrMoreConsecutiveZeroBuckets(t *testing.T) {
	report := usageStatsTextFixture()
	report.Buckets = []usage.StatsBucket{
		{Start: "2026-08-01T00:00:00Z", Tokens: 10},
		{Start: "2026-08-02T00:00:00Z"},
		{Start: "2026-08-03T00:00:00Z"},
		{Start: "2026-08-04T00:00:00Z"},
		{Start: "2026-08-05T00:00:00Z", Tokens: 20},
	}
	renderer := statsTextRenderer{report: report, width: 100}
	text := strings.Join(renderer.responsiveTrendLines(100), "\n")
	if !strings.Contains(text, "Aug 02…Aug 04") {
		t.Fatalf("folded zero range missing:\n%s", text)
	}
	if strings.Contains(text, "Aug 03 ") {
		t.Fatalf("middle zero bucket was not folded:\n%s", text)
	}
	if !strings.Contains(text, "Aug 01") || !strings.Contains(text, "Aug 05") {
		t.Fatalf("non-zero edge buckets were lost:\n%s", text)
	}
}
