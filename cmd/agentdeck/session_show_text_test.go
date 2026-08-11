package main

import (
	"errors"
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
		"SUMMARY", "1 calls", "1 completed", "BY TOOL", "Read 1", "CALL 1", "DURATION", "1.25s (1,250 ms)",
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
	for _, want := range []string{"ACTIVITY", "CALL 1", "TOOL", "Read", "STATUS", "completed"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("activity without summary missing %q:\n%s", want, output.String())
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
	if strings.Count(output.String(), "—") != 4 || strings.Contains(output.String(), "UTC+8") {
		t.Fatalf("invalid record timestamps fabricated a display zone:\n%s", output.String())
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
