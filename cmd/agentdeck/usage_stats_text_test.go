package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/usage"
)

func TestUsageStatsBalancedTextLayout(t *testing.T) {
	report := usageStatsTextFixture()
	for _, width := range []int{48, 72, 100, 140} {
		t.Run(groupedInt(int64(width))+" columns", func(t *testing.T) {
			var output bytes.Buffer
			if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: width}); err != nil {
				t.Fatal(err)
			}
			text := output.String()
			for _, want := range []string{
				"📊 USAGE STATS · LAST 7 DAYS",
				"TOKENS", "366.9M",
				"COST", "$316.83", "KNOWN",
				"SESSIONS", "34",
				"🗓 TREND · TOKENS",
				"🤖 MODELS", "claude-opus-4-8", "codex-auto-review",
				"CLIENTS", "Claude", "Codex",
				"PROVIDERS", "Claude/relay", "Codex/official",
				"CACHE HIT RATE", "MODEL Claude/claude-opus-4-8", "SESSION Codex/codex-session", "--activity",
				"AVG COST", "PEAK", "PRICED  87.69%",
				"▦ ACTIVITY BY WEEKDAY / HOUR · TOKENS",
				"LESS  · ░ ▒ ▓ █  MORE",
				"UNPRICED MODELS", "Claude/claude-fable-5", "output",
			} {
				if !strings.Contains(text, want) {
					t.Fatalf("%d-column stats missing %q:\n%s", width, want, text)
				}
			}
			for _, noisy := range []string{"366924859", "316.832730700", "(partial)"} {
				if strings.Contains(text, noisy) {
					t.Fatalf("%d-column stats retained noisy value %q:\n%s", width, noisy, text)
				}
			}
			if strings.Contains(text, "Known priced subtotal") {
				t.Fatalf("%d-column retained generic partial-cost footnote:\n%s", width, text)
			}
			for lineNumber, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
				if got := statsVisibleWidth(line); got > width {
					t.Fatalf("%d-column line %d width = %d:\n%s", width, lineNumber+1, got, line)
				}
			}
		})
	}
}

func TestUsageStatsWideLayoutAndColorAreDeterministic(t *testing.T) {
	report := usageStatsTextFixture()
	var plain, colored bytes.Buffer
	if err := renderUsageStatsWithOptions(&plain, report, usageTextRenderOptions{width: 140}); err != nil {
		t.Fatal(err)
	}
	if err := renderUsageStatsWithOptions(&colored, report, usageTextRenderOptions{width: 140, color: true}); err != nil {
		t.Fatal(err)
	}
	plainText, coloredText := plain.String(), colored.String()
	if strings.Contains(plainText, "\x1b[") || !strings.Contains(coloredText, "\x1b[") {
		t.Fatalf("ANSI color plain=%q colored=%q", plainText, coloredText)
	}
	if stripStatsANSI(coloredText) != plainText {
		t.Fatal("colored stats changed layout or content")
	}
	var paired bool
	for _, line := range strings.Split(plainText, "\n") {
		if strings.Contains(line, "TREND · TOKENS") && strings.Contains(line, "🤖 MODELS") {
			paired = true
		}
		if statsVisibleWidth(line) > 140 {
			t.Fatalf("wide line overflowed: %q", line)
		}
	}
	if !paired {
		t.Fatalf("wide layout did not pair trend and ranking:\n%s", plainText)
	}
}

func TestUsageStatsCacheSessionsAreCappedOnlyInText(t *testing.T) {
	report := usageStatsTextFixture()
	report.CacheSessions = make([]usage.StatsCacheSession, 0, 11)
	rate := "50.00"
	for index := 0; index < 11; index++ {
		report.CacheSessions = append(report.CacheSessions, usage.StatsCacheSession{Client: "codex", SessionID: fmt.Sprintf("session-%02d", index), Models: []string{"gpt-5.6-sol"}, CachedReadTokens: int64(100 - index), LogicalInputTokens: 200, CacheHitRate: &rate, DetailCommand: fmt.Sprintf("agentdeck session show session-%02d --client codex --activity", index)})
	}
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 48}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "SESSION Codex/") != 10 || !strings.Contains(text, "+1 more cache sessions in JSON") || len(report.CacheSessions) != 11 {
		t.Fatalf("cache session text cap =\n%s", text)
	}
	assertUsageStatsWidth(t, text, 48)
}

func TestUsageStatsModelActivityDetailIsOptIn(t *testing.T) {
	report := usageStatsTextFixture()
	report.Models = report.Models[:1]
	report.ShowModelActivity = true
	average := int64(250)
	report.Models[0].Activity = &usage.StatsModelActivity{ActiveSessions: 3, ActiveDays: 2, FirstAt: "2026-07-19T00:00:00Z", LastAt: "2026-07-20T00:00:00Z", ToolCalls: 7, CompletedCalls: 5, FailedCalls: 1, TotalDurationMS: 1500, AverageDuration: &average, Tools: []usage.StatsToolCount{{Name: "exec_command", Calls: 5}, {Name: "apply_patch", Calls: 2}}}
	var output bytes.Buffer
	if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 72}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"MODEL ACTIVITY · claude-opus-4-8", "3 sessions", "2 active days", "exec_command", "apply_patch", "250 ms average"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("model activity missing %q:\n%s", want, output.String())
		}
	}
	assertUsageStatsWidth(t, output.String(), 72)
}

func TestUsageStatsBalancedGolden(t *testing.T) {
	expected, err := os.ReadFile("testdata/usage-stats-balanced.txt")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = renderUsageStatsWithOptions(&output, usageStatsTextFixture(), usageTextRenderOptions{width: 100}); err != nil {
		t.Fatal(err)
	}
	if output.String() != string(expected) {
		t.Fatalf("balanced stats output changed\n--- want ---\n%s\n--- got ---\n%s", expected, output.String())
	}
}

func TestUsageTextRenderOptionsHonorColumnsWithoutColoringRedirects(t *testing.T) {
	t.Setenv("COLUMNS", "140")
	t.Setenv("TERM", "xterm-256color")
	options := newUsageTextRenderOptions(&bytes.Buffer{}, false)
	if options.width != 140 || options.color {
		t.Fatalf("redirected render options = %#v", options)
	}
}

func TestUsageStatsTextUsesInclusiveDisplayRange(t *testing.T) {
	tests := []struct {
		name  string
		from  string
		to    string
		want  string
		not   string
		label string
	}{
		{name: "custom exclusive midnight", from: "2026-07-01T00:00:00+08:00", to: "2026-07-08T00:00:00+08:00", want: "Jul 01, 2026 - Jul 07, 2026", not: "Jul 08, 2026", label: "LAST 7 DAYS"},
		{name: "range in progress", from: "2026-07-14T00:00:00+08:00", to: "2026-07-20T15:43:01+08:00", want: "Jul 14, 2026 - Jul 20, 2026", not: "Jul 21, 2026", label: "LAST 7 DAYS"},
		{name: "DST boundary", from: "2026-03-07T00:00:00-05:00", to: "2026-03-10T00:00:00-04:00", want: "Mar 07, 2026 - Mar 09, 2026", not: "Mar 10, 2026", label: "LAST 3 DAYS"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := usageStatsTextFixture()
			report.Range = usage.StatsRange{From: test.from, To: test.to}
			var output bytes.Buffer
			if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: 100}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), test.want) || !strings.Contains(output.String(), test.label) || strings.Contains(output.String(), test.not) {
				t.Fatalf("display range =\n%s\nwant %q and %q without %q", output.String(), test.want, test.label, test.not)
			}
		})
	}
}

func TestUsageStatsActivityLegendShowsInclusiveRangeAtSupportedWidths(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want string
		not  string
	}{
		{
			name: "custom range",
			from: "2026-07-01T00:00:00+08:00",
			to:   "2026-07-08T00:00:00+08:00",
			want: "Jul 01, 2026 - Jul 07, 2026",
			not:  "Jul 08, 2026",
		},
		{
			name: "DST boundary",
			from: "2026-03-07T00:00:00-05:00",
			to:   "2026-03-10T00:00:00-04:00",
			want: "Mar 07, 2026 - Mar 09, 2026",
			not:  "Mar 10, 2026",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, width := range []int{48, 72, 100, 140} {
				t.Run(strconv.Itoa(width)+" columns", func(t *testing.T) {
					report := usageStatsTextFixture()
					report.Range = usage.StatsRange{From: test.from, To: test.to}
					var output bytes.Buffer
					if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: width}); err != nil {
						t.Fatal(err)
					}
					text := output.String()
					activity := usageStatsActivitySection(t, text)
					if !strings.Contains(activity, test.want) || strings.Contains(activity, test.not) {
						t.Fatalf("%d-column Activity range =\n%s\nwant %q without %q", width, activity, test.want, test.not)
					}
					if strings.Contains(activity, "…") {
						t.Fatalf("%d-column Activity range was truncated:\n%s", width, activity)
					}
					if width == 48 && strings.Contains(usageStatsLineContaining(activity, "LESS"), test.want) {
						t.Fatalf("%d-column Activity legend and range were not split:\n%s", width, activity)
					}
					if strings.Contains(text, "\x1b[") {
						t.Fatalf("%d-column redirected output contains ANSI:\n%q", width, text)
					}
					assertUsageStatsWidth(t, text, width)
				})
			}
		})
	}
}

func TestUsageStatsHourLabelsDistinguishDatesAndDSTFold(t *testing.T) {
	report := usage.StatsReport{
		Range:    usage.StatsRange{From: "2026-10-31T23:00:00-04:00", To: "2026-11-01T03:00:00-05:00"},
		Timezone: "America/New_York", GroupBy: "hour", Metric: "tokens",
		Totals: usage.StatsTotals{Tokens: 100, Sessions: 1, Events: 4},
		Buckets: []usage.StatsBucket{
			{Start: "2026-10-31T23:00:00-04:00", Tokens: 10, KnownMetricValue: "10"},
			{Start: "2026-11-01T00:00:00-04:00", Tokens: 20, KnownMetricValue: "20"},
			{Start: "2026-11-01T01:00:00-04:00", Tokens: 30, KnownMetricValue: "30"},
			{Start: "2026-11-01T01:00:00-05:00", Tokens: 40, KnownMetricValue: "40"},
		},
		Models: []usage.StatsDimension{}, Clients: []usage.StatsDimension{}, Activity: []usage.StatsActivity{},
		Peak: usage.StatsPeak{KnownValue: "40"}, Coverage: usage.StatsCoverage{Percent: "0.00"},
	}
	for _, width := range []int{48, 72, 100, 140} {
		var output bytes.Buffer
		if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: width}); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		if !strings.Contains(text, "USAGE STATS · LAST 2 DAYS") {
			t.Fatalf("%d-column DST range label =\n%s", width, text)
		}
		for _, want := range []string{"Oct 31 23:00", "Nov 01 00:00", "Nov 01 01:00 -04:00", "Nov 01 01:00 -05:00"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%d-column hour trend missing %q:\n%s", width, want, text)
			}
		}
		assertUsageStatsWidth(t, text, width)
	}
}

func TestUsageStatsCostTextDistinguishesCompletePartialAndUnavailable(t *testing.T) {
	complete := "1.000000000"
	report := usage.StatsReport{
		Range:    usage.StatsRange{From: "2026-07-01T00:00:00Z", To: "2026-07-04T00:00:00Z"},
		Timezone: "UTC", GroupBy: "day", Metric: "cost",
		Totals: usage.StatsTotals{Sessions: 3, Events: 4, KnownProviderCost: "3.000000000", KnownAverageCost: "1.000000000"},
		Buckets: []usage.StatsBucket{
			{Start: "2026-07-01T00:00:00Z", Events: 1, ProviderCost: &complete, MetricValue: &complete, KnownMetricValue: complete, Coverage: "100.00"},
			{Start: "2026-07-02T00:00:00Z", Events: 2, KnownProviderCost: "2.000000000", KnownMetricValue: "2.000000000", Coverage: "50.00"},
			{Start: "2026-07-03T00:00:00Z", Events: 1, KnownProviderCost: "0.000000000", KnownMetricValue: "0.000000000", Coverage: "0.00"},
		},
		Models: []usage.StatsDimension{
			{Name: "complete-model", Events: 1, MetricValue: &complete, KnownMetricValue: complete, KnownShare: "33.33", Coverage: "100.00"},
			{Name: "partial-model", Events: 2, KnownMetricValue: "2.000000000", KnownShare: "66.67", Coverage: "50.00"},
			{Name: "unpriced-model", Events: 1, KnownMetricValue: "0.000000000", KnownShare: "0.00", Coverage: "0.00"},
		},
		Clients:  []usage.StatsDimension{{Name: "codex", Events: 4, KnownMetricValue: "3.000000000", KnownShare: "100.00", Coverage: "50.00"}},
		Activity: []usage.StatsActivity{{Weekday: 0, Hour: 9, KnownMetricValue: "2.000000000"}},
		Peak:     usage.StatsPeak{KnownValue: "2.000000000", Coverage: "50.00"},
		Coverage: usage.StatsCoverage{PricedEvents: 2, UnpricedEvents: 2, TotalEvents: 4, Percent: "50.00"},
		UnpricedModels: []usage.StatsUnpricedModel{
			{Client: "codex", Model: "partial-model", Components: []string{"output"}},
			{Client: "codex", Model: "unpriced-model", Components: []string{"unknown_model"}},
		},
	}
	for _, width := range []int{48, 72, 100, 140} {
		var output bytes.Buffer
		if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: width}); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		for _, want := range []string{"$1.00", "$2.00 KNOWN", "unavailable", "33.3%", "66.7%", "100.0%", "ACTIVITY BY WEEKDAY / HOUR · KNOWN COST", "UNPRICED MODELS", "output", "unknown_model"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%d-column cost report missing %q:\n%s", width, want, text)
			}
		}
		for marker, want := range map[string]string{
			"complete-model": "33.3%",
			"partial-model":  "66.7%",
			"unpriced-model": "unavailable",
			"Codex":          "100.0%",
		} {
			line := usageStatsLineContaining(text, marker)
			if !strings.Contains(line, want) {
				t.Fatalf("%d-column line containing %q = %q, want %q", width, marker, line, want)
			}
		}
		for marker, want := range map[string]string{"Jul 01": "$1.00", "Jul 02": "$2.00 KNOWN", "Jul 03": "unavailable"} {
			line := usageStatsTrendLine(text, marker)
			if !strings.Contains(line, want) {
				t.Fatalf("%d-column line starting with %q = %q, want %q", width, marker, line, want)
			}
		}
		if strings.Contains(usageStatsTrendLine(text, "Jul 01"), "KNOWN") {
			t.Fatalf("%d-column complete bucket marked partial:\n%s", width, text)
		}
		if strings.Contains(text, "Known priced subtotal") || strings.Contains(text, "\x1b[") {
			t.Fatalf("%d-column partial annotation/ANSI =\n%s", width, text)
		}
		assertUsageStatsWidth(t, text, width)
	}
}

func TestUsageStatsCostTextUsesUnavailableWhenNothingIsPriced(t *testing.T) {
	report := usage.StatsReport{
		Range:    usage.StatsRange{From: "2026-07-01T00:00:00Z", To: "2026-07-02T00:00:00Z"},
		Timezone: "UTC", GroupBy: "day", Metric: "cost",
		Totals:   usage.StatsTotals{Sessions: 1, Events: 1, KnownProviderCost: "0.000000000", KnownAverageCost: "0.000000000"},
		Buckets:  []usage.StatsBucket{{Start: "2026-07-01T00:00:00Z", Events: 1, KnownMetricValue: "0.000000000", Coverage: "0.00"}},
		Models:   []usage.StatsDimension{{Name: "unpriced-model", Events: 1, KnownShare: "0.00", Coverage: "0.00"}},
		Clients:  []usage.StatsDimension{{Name: "codex", Events: 1, KnownShare: "0.00", Coverage: "0.00"}},
		Activity: []usage.StatsActivity{{Weekday: 0, Hour: 9, KnownMetricValue: "0.000000000"}},
		Peak:     usage.StatsPeak{KnownValue: "0.000000000", Coverage: "0.00"},
		Coverage: usage.StatsCoverage{UnpricedEvents: 1, TotalEvents: 1, Percent: "0.00"},
	}
	for _, width := range []int{48, 72, 100, 140} {
		var output bytes.Buffer
		if err := renderUsageStatsWithOptions(&output, report, usageTextRenderOptions{width: width}); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		if strings.Count(text, "unavailable") < 5 || strings.Contains(text, "$0.00") || strings.Contains(text, "Known priced subtotal") || strings.Contains(text, "\x1b[") {
			t.Fatalf("%d-column unpriced cost report =\n%s", width, text)
		}
		if line := usageStatsTrendLine(text, "Jul 01"); !strings.Contains(line, "unavailable") {
			t.Fatalf("%d-column line starting with Jul 01 = %q, want unavailable", width, line)
		}
		for _, marker := range []string{"unpriced-model", "Codex", "PEAK", "ACTIVITY BY WEEKDAY / HOUR · COST"} {
			if line := usageStatsLineContaining(text, marker); !strings.Contains(line, "unavailable") && marker != "ACTIVITY BY WEEKDAY / HOUR · COST" {
				t.Fatalf("%d-column line containing %q = %q, want unavailable", width, marker, line)
			}
		}
		assertUsageStatsWidth(t, text, width)
	}
}

func usageStatsLineContaining(text, marker string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, marker) {
			return line
		}
	}
	return ""
}

func usageStatsTrendLine(text, marker string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, marker) && (strings.Contains(line, "█") || strings.Contains(line, "░")) {
			return line
		}
	}
	return ""
}

func usageStatsActivitySection(t *testing.T, text string) string {
	t.Helper()
	start := strings.Index(text, "▦ ACTIVITY BY WEEKDAY / HOUR")
	if start < 0 {
		t.Fatalf("stats output has no Activity section:\n%s", text)
	}
	section := text[start:]
	if end := strings.Index(section, "\n\n"); end >= 0 {
		section = section[:end]
	}
	return section
}

func assertUsageStatsWidth(t *testing.T, text string, width int) {
	t.Helper()
	for lineNumber, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if got := statsVisibleWidth(line); got > width {
			t.Fatalf("%d-column line %d width = %d:\n%s", width, lineNumber+1, got, line)
		}
	}
}

func usageStatsTextFixture() usage.StatsReport {
	knownCost := "316.832730700"
	knownAverage := "9.318609726"
	opusCost := "240.000000000"
	gptCost := "50.000000000"
	read, codexRead := "60.00", "40.00"
	buckets := []usage.StatsBucket{
		{Start: "2026-07-14T00:00:00+08:00", Tokens: 13402755, Sessions: 11, KnownMetricValue: "13402755"},
		{Start: "2026-07-15T00:00:00+08:00", Tokens: 86444030, Sessions: 8, KnownMetricValue: "86444030"},
		{Start: "2026-07-16T00:00:00+08:00", Tokens: 201278072, Sessions: 7, KnownMetricValue: "201278072"},
		{Start: "2026-07-17T00:00:00+08:00", Tokens: 2532096, Sessions: 4, KnownMetricValue: "2532096"},
		{Start: "2026-07-18T00:00:00+08:00", Tokens: 0, Sessions: 0, KnownMetricValue: "0"},
		{Start: "2026-07-19T00:00:00+08:00", Tokens: 379243, Sessions: 2, KnownMetricValue: "379243"},
		{Start: "2026-07-20T00:00:00+08:00", Tokens: 62888663, Sessions: 8, KnownMetricValue: "62888663"},
	}
	models := []usage.StatsDimension{
		{Name: "claude-opus-4-8", Client: "claude", Tokens: 266116000, CachedReadTokens: 600000, CacheWriteTokens: 200000, LogicalInputTokens: 1000000, CacheHitRate: &read, Sessions: 20, KnownShare: "72.53", ProviderCost: &opusCost, KnownProviderCost: opusCost, Coverage: "100.00", Activity: &usage.StatsModelActivity{ToolCalls: 80}},
		{Name: "gpt-5.6-sol", Client: "codex", Tokens: 43480000, CachedReadTokens: 200000, LogicalInputTokens: 500000, CacheHitRate: &codexRead, Sessions: 8, KnownShare: "11.85", ProviderCost: &gptCost, KnownProviderCost: gptCost, Coverage: "100.00", Activity: &usage.StatsModelActivity{ToolCalls: 30}},
		{Name: "claude-fable-5", Client: "claude", Tokens: 32544000, Sessions: 4, KnownShare: "8.87", KnownProviderCost: "26.832730700", Coverage: "50.00", Activity: &usage.StatsModelActivity{ToolCalls: 12}},
		{Name: "codex-auto-review", Client: "codex", Sessions: 2, KnownShare: "2.62", Activity: &usage.StatsModelActivity{ToolCalls: 3}},
	}
	clients := []usage.StatsDimension{
		{Name: "claude", KnownShare: "82.89", LogicalInputTokens: 1000000, CacheHitRate: &read},
		{Name: "codex", KnownShare: "17.11", CacheHitRate: &codexRead},
	}
	providers := []usage.StatsDimension{
		{Name: "relay", Client: "claude", Tokens: 304000000, Sessions: 24, KnownShare: "82.89", KnownProviderCost: "266.832730700", Coverage: "87.00", LogicalInputTokens: 1000000, CacheHitRate: &read},
		{Name: "official", Client: "codex", Tokens: 62924859, Sessions: 10, KnownShare: "17.11", ProviderCost: &gptCost, KnownProviderCost: gptCost, Coverage: "100.00", CacheHitRate: &codexRead},
	}
	cacheSessions := []usage.StatsCacheSession{
		{Client: "claude", SessionID: "claude-session", Models: []string{"claude-opus-4-8"}, CachedReadTokens: 600000, CacheWriteTokens: 200000, LogicalInputTokens: 1000000, CacheHitRate: &read, DetailCommand: "agentdeck session show claude-session --client claude --activity"},
		{Client: "codex", SessionID: "codex-session", Models: []string{"gpt-5.6-sol"}, CachedReadTokens: 200000, LogicalInputTokens: 500000, CacheHitRate: &codexRead, DetailCommand: "agentdeck session show codex-session --client codex --activity"},
	}
	activity := make([]usage.StatsActivity, 0, 168)
	for weekday := 0; weekday < 7; weekday++ {
		for hour := 0; hour < 24; hour++ {
			value := int64(0)
			if (weekday+hour)%5 == 0 {
				value = int64((weekday + 1) * (hour + 1) * 1000)
			}
			activity = append(activity, usage.StatsActivity{Weekday: weekday, Hour: hour, KnownMetricValue: strconv.FormatInt(value, 10)})
		}
	}
	return usage.StatsReport{
		Range:    usage.StatsRange{From: "2026-07-14T00:00:00+08:00", To: "2026-07-20T15:43:01+08:00"},
		Timezone: "Asia/Shanghai", GroupBy: "day", Metric: "tokens",
		Totals:         usage.StatsTotals{Tokens: 366924859, Sessions: 34, Events: 2007, KnownProviderCost: knownCost, KnownAverageCost: knownAverage},
		Buckets:        buckets,
		Models:         models,
		Clients:        clients,
		Providers:      providers,
		CacheSessions:  cacheSessions,
		Activity:       activity,
		Peak:           usage.StatsPeak{KnownValue: "201278072"},
		Coverage:       usage.StatsCoverage{Percent: "87.69"},
		UnpricedModels: []usage.StatsUnpricedModel{{Client: "claude", Model: "claude-fable-5", Components: []string{"output"}}},
	}
}
