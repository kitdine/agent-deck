package main

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/mattn/go-runewidth"
)

func TestRenderSessionShowTextKeepsMetadataForEmptyPagedSections(t *testing.T) {
	var output strings.Builder
	result := session.Result{Metadata: session.Metadata{
		Client: "codex", SessionID: "session-1", Project: "project", Model: "gpt-safe",
		FirstAt: "2026-08-01T00:00:00Z", LastAt: "2026-08-01T00:00:03Z",
	}}
	err := renderSessionShowText(&output, result, map[string]session.Pagination{
		"documents": {Page: 3, Limit: 1, Total: 2},
		"activity":  {Page: 3, Limit: 1, Total: 2},
	}, "agentdeck session show --client codex session-1 --activity --page 4", nil, nil, true, "Activity source is unavailable.", false)
	if err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{
		"SESSION", "SESSION ID", "session-1", "SESSION SPAN", "3s",
		"DOCUMENTS", "No approved visible documents", "ACTIVITY", "Activity source is unavailable.", "No safe activity calls",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered text missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSessionShowTextWrapsDocumentsWithoutWideTables(t *testing.T) {
	for _, width := range []string{"48", "60", "80", "120", "140"} {
		t.Run(width, func(t *testing.T) {
			t.Setenv("COLUMNS", width)
			visible := strings.Repeat("approved visible 文本😀 ", 12) + "END-OF-APPROVED-TEXT"
			var output strings.Builder
			err := renderSessionShowText(&output, session.Result{
				Metadata:  session.Metadata{Client: "codex", SessionID: "session-1", FirstAt: "2026-08-01T00:00:00Z", LastAt: "2026-08-01T00:00:01Z"},
				Documents: []session.Document{{EventAt: "2026-08-01T00:00:00Z", Kind: "user_prompt", Text: visible}},
			}, nil, "", nil, nil, false, "", false)
			if err != nil {
				t.Fatal(err)
			}
			text := output.String()
			for _, want := range []string{"DOCUMENT 1", "EVENT AT", "KIND", "user_prompt", "TEXT", "approved visible", "END-OF-APPROVED-TEXT"} {
				if !strings.Contains(text, want) {
					t.Fatalf("%s-column document output missing %q:\n%s", width, want, text)
				}
			}
			if strings.Contains(text, "+---") || strings.Contains(text, "| CLIENT") {
				t.Fatalf("%s-column session show used an unbounded table:\n%s", width, text)
			}
		})
	}
}

func TestRenderSessionShowTextRespectsVisibleWidthsAndSanitizesControls(t *testing.T) {
	for _, width := range []string{"1", "2", "10", "16", "24", "32", "48", "60", "80", "100", "120", "140"} {
		t.Run(width, func(t *testing.T) {
			t.Setenv("COLUMNS", width)
			limit := 0
			for _, r := range width {
				limit = limit*10 + int(r-'0')
			}
			duration := int64(1250)
			cost := "0.125000000"
			result := session.Result{
				Metadata: session.Metadata{
					Client: "codex\x1b[31m", SessionID: strings.Repeat("会话-long-", 4), Project: "项目😀 e\u0301", Model: "gpt-safe-e\u0301",
					FirstAt: "2026-08-01T00:00:00Z", LastAt: "2026-08-01T00:00:01Z",
				},
				Documents:       []session.Document{{EventAt: "2026-08-01T00:00:00Z", Kind: "user_prompt", Text: strings.Repeat("可见😀文本e\u0301", 18) + "\x1b]0;unsafe\a"}},
				Activity:        []activity.Detail{{StartedAt: "2026-08-01T00:00:00Z", Tool: "Read", Model: "gpt-safe", Status: "completed", DurationMS: &duration}},
				ActivitySummary: &session.ActivitySummary{Total: 1, Completed: 1, TotalDurationMS: duration, AverageDurationMS: &duration},
			}
			summary := &usage.SessionSummary{Tokens: sessionShowTestTokens(), ProviderCost: &cost, CatalogBaseCost: &cost}
			invocations := []usage.SessionInvocation{{Sequence: 1, EventAt: "2026-08-01T00:00:00Z", Model: "gpt-safe", Tokens: sessionShowTestTokens(), ProviderCost: &cost, CatalogBaseCost: &cost}}
			var output strings.Builder
			if err := renderSessionShowText(&output, result, nil, "", summary, invocations, true, "", false); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output.String(), "\x1b") || strings.Contains(output.String(), "unsafe") {
				t.Fatalf("%s-column output retained terminal control content: %q", width, output.String())
			}
			for _, line := range strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n") {
				if got := runewidth.StringWidth(line); got > limit {
					t.Fatalf("line width %d exceeds %d: %q", got, limit, line)
				}
			}
		})
	}
}

func TestRenderSessionShowTextExplainsActivityAndInvocationDetails(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	duration := int64(1250)
	known := "0.125000000"
	result := session.Result{
		Metadata: session.Metadata{Client: "claude", SessionID: "session-1", Model: "claude-opus", FirstAt: "2026-08-01T00:00:00Z", LastAt: "2026-08-01T00:00:02Z"},
		Activity: []activity.Detail{{StartedAt: "2026-08-01T00:00:00Z", CompletedAt: "2026-08-01T00:00:01.25Z", Tool: "Read", Model: "claude-opus", Status: "completed", DurationMS: &duration}},
		ActivitySummary: &session.ActivitySummary{
			Total: 1, Completed: 1, TotalDurationMS: duration, AverageDurationMS: &duration,
			ByTool: []session.ToolCount{{Tool: "Read", Count: 1}},
		},
	}
	summary := &usage.SessionSummary{
		Client: "claude", SessionID: "session-1", Tokens: sessionShowTestTokens(), KnownProviderCost: &known, KnownCatalogBaseCost: &known,
		Unpriced: []string{"output_tokens"}, Warnings: []string{"historical attribution"}, FirstAt: "2026-08-01T00:00:00Z",
	}
	invocations := []usage.SessionInvocation{{
		Sequence: 1, EventAt: "2026-08-01T00:00:00Z", Model: "claude-opus", Tokens: sessionShowTestTokens(),
		KnownProviderCost: known, KnownCatalogBaseCost: known, Unpriced: []string{"output_tokens"}, Warnings: []string{"historical attribution"},
	}}
	var output strings.Builder
	if err := renderSessionShowText(&output, result, nil, "", summary, invocations, true, "", false); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{
		"SUMMARY", "1 calls", "1 completed", "BY TOOL", "Read 1", "CALL 1", "1.25s (1,250 ms)",
		"TOKENS", "PRIMARY TOKENS", "input 1,200", "cached input 200", "CACHE TOKENS", "read 70", "write 5m 13", "write 1h 17",
		"PROVIDER COST", "0.125000000 (partial)", "PRICING", "partial", "UNPRICED", "output_tokens", "WARNINGS", "historical attribution",
		"INVOCATION #1", "Sequence numbers are chronological usage positions, not conversation turns.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("structured Session text missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "+---") || strings.Contains(text, "| EVENT AT") {
		t.Fatalf("structured Session text fell back to invocation table:\n%s", text)
	}
}

func TestRenderSessionShowTextEmptyTokensAreNotApplicable(t *testing.T) {
	var output strings.Builder
	if err := renderSessionShowText(&output, session.Result{Metadata: session.Metadata{Client: "codex", SessionID: "session-1"}}, nil, "", &usage.SessionSummary{Tokens: map[string]int64{}}, nil, false, "", false); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "not applicable") || !strings.Contains(text, "No normalized invocations") || strings.Contains(text, "PROVIDER COST     unpriced") {
		t.Fatalf("empty Tokens state is ambiguous:\n%s", text)
	}
}

func TestRenderSessionShowTextKeepsActivityDetailsWithoutSummary(t *testing.T) {
	var output strings.Builder
	result := session.Result{
		Metadata: session.Metadata{Client: "codex", SessionID: "session-1"},
		Activity: []activity.Detail{{StartedAt: "2026-08-01T00:00:00Z", Tool: "Read", Model: "gpt-safe", Status: "completed"}},
	}
	if err := renderSessionShowText(&output, result, nil, "", nil, nil, false, "", false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ACTIVITY", "CALL 1", "Read", "gpt-safe", "completed"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("activity without summary missing %q:\n%s", want, output.String())
		}
	}
}

func TestSessionShowActivityLinesKeepLargePagesCompact(t *testing.T) {
	duration := int64(1250)
	values := make([]activity.Detail, 20)
	for index := range values {
		values[index] = activity.Detail{
			StartedAt:  "2026-08-01T00:00:00Z",
			Tool:       "Read",
			Model:      "gpt-5.6-sol",
			Status:     "completed",
			DurationMS: &duration,
		}
	}
	lines := sessionShowActivityLines(values, 240)
	if len(lines) != len(values)+1 {
		t.Fatalf("activity lines = %d, want one labeled header plus one row per call", len(lines))
	}
	joined := strings.Join(lines, "\n")
	for _, redundant := range []string{"\nSTARTED", "\nTOOL", "\nMODEL", "\nSTATUS", "\nDURATION", "\nCOMPLETED", "\n\n"} {
		if strings.Contains(joined, redundant) {
			t.Fatalf("compact activity rows retained %q:\n%s", redundant, joined)
		}
	}
}

func TestSessionShowPricingTreatsCatalogOnlyCostAsPartial(t *testing.T) {
	cost := "0.125000000"
	if got := sessionShowSummaryPricing(usage.SessionSummary{CatalogBaseCost: &cost}); got != "partial" {
		t.Fatalf("summary pricing = %q, want partial", got)
	}
	if got := sessionShowInvocationPricing(usage.SessionInvocation{CatalogBaseCost: &cost}); got != "partial" {
		t.Fatalf("invocation pricing = %q, want partial", got)
	}
}

func TestRenderSessionShowTextPaginationUsesBoundedContinuation(t *testing.T) {
	t.Setenv("COLUMNS", "48")
	var output strings.Builder
	next := "agentdeck --state-dir '/a very long state directory' session show 'session-1' --client codex --tokens --page 2 --limit 20"
	if err := renderSessionShowText(&output, session.Result{Metadata: session.Metadata{Client: "codex", SessionID: "session-1"}}, map[string]session.Pagination{
		"documents": {Page: 1, Limit: 20, Total: 21, Shown: 20, HasMore: true, NextPage: 2},
	}, next, nil, nil, false, "", false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"SHOWING", "1-20 of 21", "NEXT PAGE", "agentdeck --state-dir"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("pagination missing %q:\n%s", want, output.String())
		}
	}
	for _, line := range strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n") {
		if got := runewidth.StringWidth(line); got > 48 {
			t.Fatalf("pagination line width %d exceeds 48: %q", got, line)
		}
	}
}

func TestRenderSessionShowTextNamesDisplayZoneForRecordTimestamps(t *testing.T) {
	usePinnedDisplayZone(t, time.FixedZone("UTC+8", 8*60*60))
	const stored = "2026-07-20T16:00:00Z"
	result := session.Result{
		Metadata:  session.Metadata{Client: "codex", SessionID: "session-1"},
		Documents: []session.Document{{EventAt: stored, Kind: "user_prompt", Text: "visible"}},
		Activity: []activity.Detail{{
			StartedAt: stored, CompletedAt: stored, Tool: "Read", Status: "completed",
		}},
	}
	summary := &usage.SessionSummary{Tokens: sessionShowTestTokens()}
	invocations := []usage.SessionInvocation{{Sequence: 1, EventAt: stored, Tokens: sessionShowTestTokens()}}
	var output strings.Builder
	if err := renderSessionShowText(&output, result, nil, "", summary, invocations, true, "", false); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"EVENT AT", "STARTED", "COMPLETED", "2026-07-21 00:00:00 UTC+8"} {
		if !strings.Contains(text, want) {
			t.Fatalf("record timestamp missing %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "EVENT AT") != 2 || strings.Count(text, "2026-07-21 00:00:00 UTC+8") != 4 || strings.Contains(text, stored) {
		t.Fatalf("document/invocation timestamps did not share the display-zone contract:\n%s", text)
	}

	invalid := result
	invalid.Documents = []session.Document{{EventAt: "not-a-timestamp", Kind: "user_prompt", Text: "visible"}}
	invalid.Activity = []activity.Detail{{
		StartedAt: "not-a-timestamp", CompletedAt: "not-a-timestamp", Tool: "Read", Status: "completed",
	}}
	invocations[0].EventAt = "not-a-timestamp"
	output.Reset()
	if err := renderSessionShowText(&output, invalid, nil, "", summary, invocations, true, "", false); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), "—") != 3 || strings.Contains(output.String(), "UTC+8") {
		t.Fatalf("invalid record timestamps fabricated a display zone:\n%s", output.String())
	}
}

func TestSessionShowActivityLinesUseResponsiveLabeledGrammar(t *testing.T) {
	duration := int64(1240)
	values := []activity.Detail{{
		StartedAt:  "2026-08-11T17:30:00Z",
		Tool:       "Read",
		Model:      "claude-opus",
		Status:     "completed",
		DurationMS: &duration,
	}}

	for _, width := range []int{48, 80, 100, 120, 180} {
		t.Run(fmt.Sprintf("width-%d", width), func(t *testing.T) {
			lines := sessionShowActivityLines(values, width)
			text := strings.Join(lines, "\n")
			for _, want := range []string{"CALL", "1", "STARTED", "2026-08-11", "TOOL", "Read", "MODEL", "claude-opus", "STATUS", "completed", "DURATION", "1.24s"} {
				if !strings.Contains(text, want) {
					t.Fatalf("activity at width %d missing %q:\n%s", width, want, text)
				}
			}
			for _, line := range lines {
				if got := statsVisibleWidth(line); got > width {
					t.Fatalf("activity line width = %d, want <= %d: %q", got, width, line)
				}
			}
		})
	}

	standardLines := sessionShowActivityLines(values, 80)
	if len(standardLines) != 2 || !strings.Contains(standardLines[0], "TOOL Read") || !strings.Contains(standardLines[0], "STATUS completed") || strings.Contains(standardLines[0], "STARTED") {
		t.Fatalf("standard activity primary line order = %#v", standardLines)
	}
	for _, want := range []string{"STARTED", "MODEL claude-opus", "DURATION 1.24s"} {
		if !strings.Contains(standardLines[1], want) {
			t.Fatalf("standard activity detail line missing %q: %#v", want, standardLines)
		}
	}

	wide := strings.Join(sessionShowActivityLines(values, 120), "\n")
	veryWide := strings.Join(sessionShowActivityLines(values, 180), "\n")
	if wide != veryWide {
		t.Fatalf("wide activity should remain content-bounded:\n120:\n%s\n180:\n%s", wide, veryWide)
	}
	compact := strings.Join(sessionShowActivityLines(values, 48), "\n")
	if !strings.Contains(compact, "CALL 1\n") || !strings.Contains(compact, "TOOL") {
		t.Fatalf("compact activity should stack a complete labeled record:\n%s", compact)
	}
	previous := -1
	for _, label := range []string{"TOOL", "STATUS", "STARTED", "MODEL", "DURATION"} {
		position := strings.Index(compact, label)
		if position <= previous {
			t.Fatalf("compact activity field order invalid at %q:\n%s", label, compact)
		}
		previous = position
	}
}

func TestSessionShowActivityLinesOmitUnknownAndRedundantOptionalFields(t *testing.T) {
	duration := int64(2000)
	values := []activity.Detail{
		{
			Tool:        "unknown",
			Model:       "unavailable",
			Status:      "completed",
			CompletedAt: "2026-08-11T17:30:02Z",
			DurationMS:  &duration,
		},
		{
			Tool:        "Write",
			CompletedAt: "2026-08-11T17:31:00Z",
		},
		{
			StartedAt:   "not-a-time",
			CompletedAt: "not-a-time",
		},
		{},
	}

	text := strings.Join(sessionShowActivityLines(values, 80), "\n")
	for _, omitted := range []string{"unknown", "unavailable", "not-a-time"} {
		if strings.Contains(text, omitted) {
			t.Fatalf("activity should omit optional %q value:\n%s", omitted, text)
		}
	}
	if strings.Count(text, "COMPLETED") != 1 {
		t.Fatalf("activity should show valid COMPLETED only without DURATION:\n%s", text)
	}
	if !strings.Contains(text, "DURATION") || !strings.Contains(text, "STARTED") || !strings.Contains(text, "—") || !strings.Contains(text, "NO SAFE ACTIVITY METADATA") {
		t.Fatalf("activity should preserve duration, invalid-time marker, and explicit safe-empty state:\n%s", text)
	}
	wideLines := sessionShowActivityLines(values, 180)
	if len(wideLines) != len(values)+1 || !strings.Contains(wideLines[0], "STATE") || !strings.Contains(wideLines[len(wideLines)-1], "NO SAFE ACTIVITY METADATA") {
		t.Fatalf("wide activity should keep table mode for safe-empty rows: %#v", wideLines)
	}
}

func TestSessionShowActivityLinesUseAbsoluteOrdinalsAcrossResponsiveLayouts(t *testing.T) {
	values := []activity.Detail{{Tool: "Read", Status: "completed"}, {Tool: "Write", Status: "failed"}}
	for _, width := range []int{48, 80, 180} {
		lines := sessionShowActivityLinesFrom(values, width, 21)
		seen := map[string]bool{}
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "CALL" {
				seen[fields[1]] = true
			} else if len(fields) > 0 && (fields[0] == "21" || fields[0] == "22" || fields[0] == "1" || fields[0] == "2") {
				seen[fields[0]] = true
			}
		}
		if !seen["21"] || !seen["22"] || seen["1"] || seen["2"] {
			t.Fatalf("activity width %d ordinals = %#v, want 21 and 22 only:\n%s", width, seen, strings.Join(lines, "\n"))
		}
	}
}

func TestRenderSessionShowTextUsesActivityPageOffset(t *testing.T) {
	for _, width := range []string{"48", "80", "180"} {
		t.Run(width, func(t *testing.T) {
			t.Setenv("COLUMNS", width)
			var output strings.Builder
			result := session.Result{
				Metadata: session.Metadata{Client: "codex", SessionID: "session-1"},
				Activity: []activity.Detail{{Tool: "Read", Status: "completed"}},
			}
			pagination := map[string]session.Pagination{"activity": {Page: 3, Limit: 10, Total: 21, Shown: 1}}
			if err := renderSessionShowText(&output, result, pagination, "", nil, nil, true, "", false); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(output.String(), "\n")
			seen21, seen1 := false, false
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) >= 2 && fields[0] == "CALL" {
					seen21 = seen21 || fields[1] == "21"
					seen1 = seen1 || fields[1] == "1"
				} else if len(fields) > 0 {
					seen21 = seen21 || fields[0] == "21"
					seen1 = seen1 || fields[0] == "1"
				}
			}
			if !seen21 || seen1 {
				t.Fatalf("activity page offset at width %s (seen21=%t seen1=%t):\n%s", width, seen21, seen1, output.String())
			}
		})
	}
}

func TestSessionShowPageFirstOrdinalSaturatesHugePages(t *testing.T) {
	if got := sessionShowPageFirstOrdinal(session.Pagination{Page: math.MaxInt, Limit: 2}); got != math.MaxInt {
		t.Fatalf("huge activity page first ordinal = %d, want %d", got, math.MaxInt)
	}
}

func TestSessionShowActivityLinesOmitUnavailableStartedAt(t *testing.T) {
	for _, startedAt := range []string{"unknown", "unavailable", " UNKNOWN "} {
		for _, width := range []int{48, 80, 180} {
			text := strings.Join(sessionShowActivityLines([]activity.Detail{{StartedAt: startedAt, Tool: "Read"}}, width), "\n")
			if strings.Contains(text, "STARTED") || strings.Contains(text, "—") {
				t.Fatalf("activity width %d retained unavailable StartedAt %q:\n%s", width, startedAt, text)
			}
		}
	}
}

func TestSessionShowActivityLinesSanitizeHostileCells(t *testing.T) {
	values := []activity.Detail{{
		Tool:   "Read\x1b[31m\nspoof",
		Model:  "模型🙂e\u0301",
		Status: "completed\rfailed",
	}}
	for _, width := range []int{48, 80, 120} {
		lines := sessionShowActivityLines(values, width)
		text := strings.Join(lines, "\n")
		if strings.Contains(text, "\x1b") || strings.Contains(text, "\r") {
			t.Fatalf("activity exposed terminal control at width %d: %q", width, text)
		}
		for _, line := range lines {
			if got := statsVisibleWidth(line); got > width {
				t.Fatalf("hostile activity line width = %d, want <= %d: %q", got, width, line)
			}
		}
	}
}

func TestRenderSessionShowTextReportsWriteFailure(t *testing.T) {
	want := errors.New("write failed")
	err := renderSessionShowText(sessionShowErrorWriter{err: want}, session.Result{Metadata: session.Metadata{Client: "codex", SessionID: "session-1"}}, nil, "", nil, nil, false, "", false)
	if !errors.Is(err, want) {
		t.Fatalf("render error = %v, want %v", err, want)
	}
}

type sessionShowErrorWriter struct{ err error }

func (w sessionShowErrorWriter) Write([]byte) (int, error) { return 0, w.err }

func sessionShowTestTokens() map[string]int64 {
	return map[string]int64{
		"input_tokens":          1200,
		"cached_input_tokens":   200,
		"output_tokens":         300,
		"cache_read_tokens":     70,
		"cache_creation_tokens": 11,
		"cache_write_5m_tokens": 13,
		"cache_write_1h_tokens": 17,
	}
}
