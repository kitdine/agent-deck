package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func TestSessionViewerOverviewUsesExplicitUnknownAndCompactProject(t *testing.T) {
	page := sessionViewerOverviewPage(session.Metadata{
		Client:    "claude",
		SessionID: "session-1",
		Project:   "/private/work/agent-deck",
		FirstAt:   "2026-08-10T00:00:00Z",
		LastAt:    "2026-08-10T00:01:00Z",
	})
	values := make(map[string]string, len(page.Rows))
	for _, row := range page.Rows {
		values[row.Identity] = row.Value
	}
	if values["model"] != "unknown" || values["project"] != "agent-deck" || values["duration"] != "1m0s" {
		t.Fatalf("overview values = %#v", values)
	}
	for _, row := range page.Rows {
		if strings.Contains(strings.Join(append(row.Detail, row.Value), " "), "/private/") {
			t.Fatalf("overview exposed path in %#v", row)
		}
	}
}

func TestSessionViewerDocumentsExposeBoundedApprovedDetailAndRecoveryCommand(t *testing.T) {
	metadata := session.Metadata{Client: "codex", SessionID: "session-1", SourcePath: "/private/source.jsonl"}
	longText := strings.Repeat("approved visible text ", 300)
	rows := sessionViewerDocumentRows(metadata, []session.Document{{
		EventAt: "2026-08-10T12:00:00Z",
		Kind:    "assistant",
		Text:    longText + "\x1b[31munsafe escape",
	}}, 2, 20)
	if len(rows) != 1 || !strings.Contains(rows[0].Detail[0], "approved visible text") {
		t.Fatalf("document rows = %#v", rows)
	}
	joined := strings.Join(append(append([]string{}, rows[0].Detail...), rows[0].Footer), " ")
	for _, want := range []string{"Preview capped", "agentdeck session show", "--page 2", "--limit 20"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("document detail missing %q: %q", want, joined)
		}
	}
	for _, forbidden := range []string{"\x1b[31m", metadata.SourcePath} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("document detail exposed %q: %q", forbidden, joined)
		}
	}
}

func TestSessionViewerActivityRowsContainOnlySafeStructuredMetadata(t *testing.T) {
	duration := int64(1250)
	page := sessionViewerActivityPage(activity.Page{
		Details: []activity.Detail{{
			Client: "claude", Model: "claude-opus", Tool: "Read", Status: "completed",
			StartedAt: "2026-08-10T12:00:00Z", CompletedAt: "2026-08-10T12:00:01.25Z", DurationMS: &duration,
		}},
		Page: 1, Limit: 20, Total: 1,
		Summary: activity.Summary{Total: 1, Completed: 1, TotalDurationMS: duration, AverageDurationMS: &duration},
	})
	if len(page.Rows) != 1 || page.Rows[0].Value != "completed" {
		t.Fatalf("activity page = %#v", page)
	}
	joined := strings.Join(page.Rows[0].Detail, " ")
	for _, want := range []string{"MODEL claude-opus", "STATUS completed", "DURATION 1,250ms", "COMPLETED"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("activity detail missing %q: %q", want, joined)
		}
	}
	if !strings.Contains(page.Rows[0].Footer, "arguments, results, commands") {
		t.Fatalf("activity privacy footer = %q", page.Rows[0].Footer)
	}
}

func TestSessionViewerTokensUseNormalizedSummaryAndEveryComponent(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	service := usage.New(database, t.TempDir())
	service.Now = func() time.Time { return time.Date(2026, 7, 13, 2, 0, 0, 0, time.UTC) }
	if err := service.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at)
VALUES('codex','token-session','2026-07-13T00:00:00Z','2026-07-13T00:00:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,cache_write_5m_tokens,cache_write_1h_tokens,source_path,source_offset)
VALUES('token-event','codex','token-session','event','2026-07-13T00:00:00Z','gpt-5.4',120,20,30,7,11,13,17,'fixture',0);`); err != nil {
		t.Fatal(err)
	}

	load := newSessionViewerLoad(ctx, database, session.Metadata{Client: "codex", SessionID: "token-session"}, service)
	page, err := load(ctx, viewerTokens, 1, sessionViewerPageLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 1 || !strings.Contains(strings.Join(page.Summary, " "), "INPUT 120") || !strings.Contains(strings.Join(page.Summary, " "), "OUTPUT 30") {
		t.Fatalf("token page summary = %#v rows = %#v", page.Summary, page.Rows)
	}
	if !strings.Contains(page.Warning, "historical attribution") || page.Warning == "none" {
		t.Fatalf("token page warning = %q, want real attribution warning", page.Warning)
	}
	detail := strings.Join(page.Rows[0].Detail, " ")
	for _, want := range []string{
		"INPUT TOKENS 120", "CACHED INPUT TOKENS 20", "OUTPUT TOKENS 30",
		"CACHE READ TOKENS 7", "CACHE CREATION TOKENS 11", "CACHE WRITE 5M TOKENS 13", "CACHE WRITE 1H TOKENS 17",
		"CATALOG BASE COST", "PROVIDER COST", "PRICING STATUS complete",
	} {
		if !strings.Contains(detail, want) {
			t.Fatalf("token detail missing %q: %q", want, detail)
		}
	}
}

func TestSessionViewerTokensExposePartialPricingWarnings(t *testing.T) {
	known := "0.125000000"
	page := sessionViewerTokensPage(
		usage.SessionSummary{
			Client: "claude", Tokens: map[string]int64{"input_tokens": 10, "output_tokens": 2},
			KnownProviderCost: &known, Unpriced: []string{"output_tokens"}, Warnings: []string{"fallback attribution"},
		},
		[]usage.SessionInvocation{{
			Sequence: 1, EventAt: "2026-08-10T12:00:00Z", Model: "claude-opus",
			Tokens: map[string]int64{"input_tokens": 10, "output_tokens": 2}, KnownProviderCost: known,
			Unpriced: []string{"output_tokens"}, Warnings: []string{"fallback attribution"},
		}},
		usage.InvocationPagination{Page: 1, Limit: 20, Total: 1, Shown: 1},
	)
	if !page.Partial || !strings.Contains(page.Warning, "fallback attribution") || !strings.Contains(page.Warning, "unpriced components") {
		t.Fatalf("partial token page = %#v", page)
	}
	if !strings.Contains(page.Warning, "fallback attribution · unpriced components") {
		t.Fatalf("partial warning separator = %q", page.Warning)
	}
	detail := strings.Join(page.Rows[0].Detail, " ")
	for _, want := range []string{"PRICING STATUS partial", "UNPRICED COMPONENTS output_tokens", "WARNINGS fallback attribution"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("partial detail missing %q: %q", want, detail)
		}
	}
}

func TestSessionViewerTokensEmptyStateIsNotApplicable(t *testing.T) {
	page := sessionViewerTokensPage(
		usage.SessionSummary{Tokens: map[string]int64{}},
		nil,
		usage.InvocationPagination{Page: 1, Limit: 20},
	)
	joined := strings.Join(page.Summary, " ")
	if !strings.Contains(joined, "PROVIDER COST not applicable · PRICING not applicable") || strings.Contains(joined, "unpriced") {
		t.Fatalf("empty Tokens summary = %q", joined)
	}
	if page.Partial {
		t.Fatal("empty Tokens page must not claim partial pricing")
	}
}

func TestSessionViewerTokensReadFailureBecomesRecoverablePage(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	service := usage.New(database, t.TempDir())
	load := newSessionViewerLoad(ctx, database, session.Metadata{Client: "codex", SessionID: "session-1"}, service)
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	page, err := load(ctx, viewerTokens, 1, sessionViewerPageLimit)
	if err != nil {
		t.Fatalf("Tokens read failure escaped viewer: %v", err)
	}
	if !page.Partial || !strings.Contains(page.Warning, "could not be read") || page.Empty == "" {
		t.Fatalf("recoverable Tokens page = %#v", page)
	}
}
