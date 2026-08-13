package main

import (
	"context"
	"fmt"
	"os"
	pathpkg "path"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/activity"
	terminaloutput "github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

const sessionViewerDocumentPreviewRunes = 4000

func newSessionViewerLoad(ctx context.Context, database *store.Store, metadata session.Metadata, service *usage.Service) sessionViewerLoad {
	return func(_ context.Context, section sessionViewerSection, page, limit int) (sessionViewerPage, error) {
		switch section {
		case viewerOverview:
			return sessionViewerOverviewPage(metadata), nil
		case viewerDocuments:
			documents, pagination, err := session.DocumentsPage(ctx, database.DB, metadata, page, limit, false)
			if err != nil {
				return sessionViewerPage{}, err
			}
			_, sourceErr := os.Stat(metadata.SourcePath)
			return sessionViewerPage{
				Rows:  sessionViewerDocumentRows(metadata, documents, pagination.Page, pagination.Limit),
				Empty: "No approved visible documents are indexed for this session.",
				Page:  pagination.Page,
				Total: pagination.Total,
				Stale: os.IsNotExist(sourceErr),
			}, nil
		case viewerActivity:
			result, err := activity.ReadDetailsPage(metadata.SourcePath, metadata.Client, metadata.SessionID, page, limit, false)
			if err != nil {
				return sessionViewerPage{
					Empty:   "No safe activity details are available.",
					Page:    page,
					Warning: "Activity source is unavailable.",
					Stale:   os.IsNotExist(err),
				}, nil
			}
			return sessionViewerActivityPage(result), nil
		case viewerTokens:
			if service == nil {
				return sessionViewerPage{
					Empty:   "No normalized token invocations are available.",
					Page:    page,
					Warning: "Usage service is unavailable.",
					Partial: true,
				}, nil
			}
			summary, err := service.SessionUsageSummary(ctx, metadata.Client, metadata.SessionID)
			if err != nil {
				return sessionViewerPage{
					Empty:   "Normalized token summary is unavailable.",
					Page:    page,
					Warning: "Token summary could not be read.",
					Partial: true,
				}, nil
			}
			invocations, pagination, err := service.SessionInvocations(ctx, metadata.Client, metadata.SessionID, page, limit, false)
			if err != nil {
				return sessionViewerPage{
					Summary: sessionViewerTokenSummaryLines(summary, false),
					Empty:   "Normalized invocation details are unavailable.",
					Page:    page,
					Warning: "Token invocation details could not be read.",
					Partial: true,
				}, nil
			}
			return sessionViewerTokensPage(summary, invocations, pagination), nil
		default:
			return sessionViewerPage{}, fmt.Errorf("unknown viewer section")
		}
	}
}

func sessionViewerOverviewPage(metadata session.Metadata) sessionViewerPage {
	client := sessionViewerKnown(metadata.Client)
	model := sessionViewerKnown(metadata.Model)
	project := sessionProjectLabel(metadata.Project)
	first := sessionViewerKnown(renderDisplayTimeWithZone(metadata.FirstAt))
	last := sessionViewerKnown(renderDisplayTimeWithZone(metadata.LastAt))
	duration := sessionViewerSessionDuration(metadata.FirstAt, metadata.LastAt)
	rows := []sessionViewerRow{
		{Identity: "client", Label: "CLIENT", Value: client, LabelColor: sessionClientColor(client), ValueColor: sessionClientColor(client)},
		{Identity: "session", Label: "SESSION", Value: sessionViewerKnown(metadata.SessionID), LabelColor: usageColorBrand, ValueColor: usageColorBrand},
		{Identity: "model", Label: "MODEL", Value: model, LabelColor: sessionClientColor(client), ValueColor: sessionClientColor(client)},
		{Identity: "project", Label: "PROJECT", Value: project, LabelColor: usageColorSuccess, ValueColor: usageColorSuccess},
		{Identity: "first", Label: "FIRST ACTIVITY", Value: first, LabelColor: usageColorInfo, ValueColor: usageColorInfo},
		{Identity: "last", Label: "LAST ACTIVITY", Value: last, LabelColor: usageColorInfo, ValueColor: usageColorInfo},
		{Identity: "duration", Label: "SESSION SPAN", Value: duration, LabelColor: usageColorSession, ValueColor: usageColorSession},
	}
	return sessionViewerPage{
		Rows:    rows,
		Summary: []string{strings.ToUpper(client) + " · " + model + " · times in " + displayZoneName()},
		Page:    1,
		Total:   len(rows),
	}
}

func sessionViewerDocumentRows(metadata session.Metadata, documents []session.Document, page, limit int) []sessionViewerRow {
	rows := make([]sessionViewerRow, 0, len(documents))
	for index, document := range documents {
		kind := sessionViewerKnown(document.Kind)
		when := sessionViewerKnown(renderSessionDocumentTime(document.EventAt))
		text := terminaloutput.SanitizeTerminalCell(document.Text)
		preview := oneLine(text)
		if strings.TrimSpace(preview) == "" {
			preview = "empty approved text"
		}
		excerpt, truncated := sessionViewerExcerpt(text, sessionViewerDocumentPreviewRunes)
		detail := terminalDetailModel{
			fields: []terminalDetailField{
				{label: "KIND", value: kind, role: terminalDetailRoleNeutral, priority: terminalDetailPriorityPrimary},
				{label: "INDEXED", value: when, role: terminalDetailRoleSession, priority: terminalDetailPriorityPrimary},
			},
			notes: []terminalDetailNote{{text: excerpt, role: terminalDetailRoleNeutral, priority: terminalDetailPrioritySecondary}},
		}
		if truncated {
			detail.notes = append(detail.notes, terminalDetailNote{text: fmt.Sprintf("Preview capped at %d characters.", sessionViewerDocumentPreviewRunes), status: "TRUNCATED", role: terminalDetailRoleWarning, priority: terminalDetailPriorityTertiary})
		}
		detail.notes = append(detail.notes, terminalDetailNote{text: sessionViewerFullPageCommand(metadata, page, limit), role: terminalDetailRoleNeutral, priority: terminalDetailPriorityTertiary})
		rows = append(rows, sessionViewerRow{
			Identity:   fmt.Sprintf("document-%d", (page-1)*limit+index+1),
			Label:      when + " · " + kind,
			Value:      preview,
			Detail:     detail,
			LabelColor: usageColorInfo,
			ValueColor: usageColorSuccess,
		})
	}
	return rows
}

func sessionViewerFullPageCommand(metadata session.Metadata, page, limit int) string {
	return fmt.Sprintf(
		"FULL PAGE · agentdeck session show %s --client %s --page %d --limit %d",
		sessionViewerShellArg(metadata.SessionID),
		sessionViewerShellArg(metadata.Client),
		page,
		limit,
	)
}

func sessionViewerActivityPage(result activity.Page) sessionViewerPage {
	average := "unavailable"
	if result.Summary.AverageDurationMS != nil {
		average = groupedInt(*result.Summary.AverageDurationMS) + "ms"
	}
	rows := make([]sessionViewerRow, 0, len(result.Details))
	for index, detail := range result.Details {
		status := sessionViewerKnown(detail.Status)
		model := sessionViewerKnown(detail.Model)
		started := sessionViewerKnown(renderDisplayTime(detail.StartedAt))
		completed := sessionViewerKnown(renderDisplayTime(detail.CompletedAt))
		duration := "unavailable"
		if detail.DurationMS != nil {
			duration = groupedInt(*detail.DurationMS) + "ms"
		}
		rows = append(rows, sessionViewerRow{
			Identity:   fmt.Sprintf("activity-%d", (result.Page-1)*result.Limit+index+1),
			Label:      started + " · " + sessionViewerKnown(detail.Tool),
			Value:      status,
			Detail:     sessionViewerActivityDetail(model, duration, completed),
			LabelColor: sessionClientColor(detail.Client),
			ValueColor: sessionActivityStatusColor(status),
		})
	}
	return sessionViewerPage{
		Rows: rows,
		Summary: []string{fmt.Sprintf(
			"%d calls · %d completed · %d failed · %d incomplete · average %s",
			result.Summary.Total,
			result.Summary.Completed,
			result.Summary.Failed,
			result.Summary.Incomplete,
			average,
		)},
		Empty:   "No safe activity calls were found for this session.",
		Page:    max(1, result.Page),
		Total:   result.Total,
		Partial: result.Summary.Incomplete > 0,
	}
}

func sessionViewerActivityDetail(model, duration, completed string) terminalDetailModel {
	detail := terminalDetailModel{}
	if model != "unknown" {
		detail.fields = append(detail.fields, terminalDetailField{label: "MODEL", value: model, role: terminalDetailRoleNeutral, priority: terminalDetailPriorityPrimary})
	}
	if duration != "unavailable" {
		detail.fields = append(detail.fields, terminalDetailField{label: "DURATION", value: duration, role: terminalDetailRoleSession, priority: terminalDetailPriorityPrimary})
	}
	if completed != "unknown" {
		detail.fields = append(detail.fields, terminalDetailField{label: "COMPLETED", value: completed, role: terminalDetailRoleSuccess, priority: terminalDetailPrioritySecondary})
	}
	detail.notes = append(detail.notes, terminalDetailNote{text: "Arguments, results, commands, and hidden content are excluded.", status: "SAFE METADATA ONLY", role: terminalDetailRoleNeutral, priority: terminalDetailPriorityTertiary})
	return detail
}

func sessionViewerTokensPage(summary usage.SessionSummary, invocations []usage.SessionInvocation, pagination usage.InvocationPagination) sessionViewerPage {
	_, pricing := sessionViewerSummaryCost(summary)
	rows := make([]sessionViewerRow, 0, len(invocations))
	for _, invocation := range invocations {
		model := sessionViewerKnown(invocation.Model)
		cost, status := sessionViewerInvocationCost(invocation)
		detail := terminalDetailModel{}
		for _, component := range sessionViewerTokenComponents() {
			if invocation.Tokens[component.key] == 0 {
				continue
			}
			// Input, output, and cost are already carried by the selected row.
			if component.key == "input_tokens" || component.key == "output_tokens" {
				continue
			}
			detail.fields = append(detail.fields, terminalDetailField{label: component.label, value: groupedInt(invocation.Tokens[component.key]), role: terminalDetailRoleToken, priority: terminalDetailPriorityPrimary})
		}
		if catalogCost := sessionViewerInvocationCatalogCost(invocation); catalogCost != "unpriced" {
			detail.fields = append(detail.fields, terminalDetailField{label: "CATALOG BASE COST", value: catalogCost, role: sessionViewerPricingDetailRole(status), priority: terminalDetailPriorityPrimary})
		}
		detail.notes = append(detail.notes, terminalDetailNote{text: "Normalized invocation; no unreliable conversation-turn join is claimed.", status: strings.ToUpper(status), role: sessionViewerPricingDetailRole(status), priority: terminalDetailPriorityPrimary})
		if len(invocation.Unpriced) > 0 {
			detail.notes = append(detail.notes, terminalDetailNote{text: strings.Join(invocation.Unpriced, ", "), status: "UNPRICED COMPONENTS", role: terminalDetailRoleWarning, priority: terminalDetailPriorityPrimary})
		}
		if len(invocation.Warnings) > 0 {
			detail.notes = append(detail.notes, terminalDetailNote{text: strings.Join(invocation.Warnings, " · "), status: "WARNING", role: terminalDetailRoleWarning, priority: terminalDetailPriorityPrimary})
		}
		rows = append(rows, sessionViewerRow{
			Identity: fmt.Sprintf("invocation-%d", invocation.Sequence),
			Label: fmt.Sprintf("#%d · %s · %s",
				invocation.Sequence,
				sessionViewerKnown(renderDisplayTime(invocation.EventAt)),
				model,
			),
			Value: fmt.Sprintf("IN %s · OUT %s · COST %s",
				groupedInt(invocation.Tokens["input_tokens"]),
				groupedInt(invocation.Tokens["output_tokens"]),
				cost,
			),
			Detail:     detail,
			LabelColor: sessionClientColor(summary.Client),
			ValueColor: sessionPricingStatusColor(status),
		})
	}

	warnings := append([]string(nil), summary.Warnings...)
	if len(summary.Unpriced) > 0 {
		warnings = append(warnings, "unpriced components: "+strings.Join(summary.Unpriced, ", "))
	}
	warningText := ""
	if len(warnings) > 0 {
		warningText = strings.Join(warnings, " · ")
	}
	partial := pagination.Total > 0 && pricing != "complete"
	summaryLines := sessionViewerTokenSummaryLines(summary, pagination.Total == 0)
	return sessionViewerPage{
		Rows:    rows,
		Summary: summaryLines,
		Empty:   "No normalized token invocations are indexed for this session.",
		Page:    max(1, pagination.Page),
		Total:   pagination.Total,
		Warning: warningText,
		Partial: partial,
	}
}

func sessionViewerPricingDetailRole(status string) terminalDetailRole {
	if status == "complete" {
		return terminalDetailRoleCost
	}
	return terminalDetailRoleWarning
}

func sessionViewerTokenSummaryLines(summary usage.SessionSummary, empty bool) []string {
	cost, pricing := sessionViewerSummaryCost(summary)
	if empty {
		cost, pricing = "not applicable", "not applicable"
	}
	return []string{
		fmt.Sprintf("INPUT %s · CACHED INPUT %s · OUTPUT %s", groupedInt(summary.Tokens["input_tokens"]), groupedInt(summary.Tokens["cached_input_tokens"]), groupedInt(summary.Tokens["output_tokens"])),
		"PROVIDER COST " + cost + " · PRICING " + pricing,
	}
}

type sessionViewerTokenComponent struct {
	key   string
	label string
}

func sessionViewerTokenComponents() []sessionViewerTokenComponent {
	return []sessionViewerTokenComponent{
		{key: "input_tokens", label: "INPUT TOKENS"},
		{key: "cached_input_tokens", label: "CACHED INPUT TOKENS"},
		{key: "output_tokens", label: "OUTPUT TOKENS"},
		{key: "cache_read_tokens", label: "CACHE READ TOKENS"},
		{key: "cache_creation_tokens", label: "CACHE CREATION TOKENS"},
		{key: "cache_write_5m_tokens", label: "CACHE WRITE 5M TOKENS"},
		{key: "cache_write_1h_tokens", label: "CACHE WRITE 1H TOKENS"},
		{key: "cache_write_tokens", label: "CACHE WRITE TOKENS"},
	}
}

func sessionViewerSummaryCost(summary usage.SessionSummary) (string, string) {
	if summary.ProviderCost != nil && len(summary.Unpriced) == 0 {
		return *summary.ProviderCost, "complete"
	}
	if summary.KnownProviderCost != nil {
		return *summary.KnownProviderCost + " (partial)", "partial"
	}
	return "unpriced", "unpriced"
}

func sessionViewerInvocationCost(invocation usage.SessionInvocation) (string, string) {
	if invocation.ProviderCost != nil && len(invocation.Unpriced) == 0 {
		return *invocation.ProviderCost, "complete"
	}
	if invocation.KnownProviderCost != "" {
		return invocation.KnownProviderCost + " (partial)", "partial"
	}
	return "unpriced", "unpriced"
}

func sessionViewerInvocationCatalogCost(invocation usage.SessionInvocation) string {
	if invocation.CatalogBaseCost != nil && len(invocation.Unpriced) == 0 {
		return *invocation.CatalogBaseCost
	}
	if invocation.KnownCatalogBaseCost != "" {
		return invocation.KnownCatalogBaseCost + " (partial)"
	}
	return "unpriced"
}

func sessionActivityStatusColor(status string) string {
	switch strings.ToLower(status) {
	case "completed", "success", "succeeded":
		return usageColorSuccess
	case "failed", "error":
		return usageColorError
	default:
		return usageColorWarning
	}
}

func sessionPricingStatusColor(status string) string {
	if status == "complete" {
		return usageColorCost
	}
	return usageColorWarning
}

func sessionClientColor(client string) string {
	switch strings.ToLower(strings.TrimSpace(client)) {
	case "codex":
		return usageColorToken
	case "claude":
		return usageColorSession
	default:
		return usageColorWarning
	}
}

func sessionProjectLabel(project string) string {
	value := strings.TrimSpace(terminaloutput.SanitizeTerminalCell(project))
	if value == "" {
		return "unknown"
	}
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimRight(value, "/")
	if base := pathpkg.Base(value); base != "" && base != "." && base != "/" {
		return base
	}
	return "unknown"
}

func sessionViewerKnown(value string) string {
	value = strings.TrimSpace(terminaloutput.SanitizeTerminalCell(value))
	if value == "" {
		return "unknown"
	}
	return value
}

func sessionViewerSessionDuration(firstAt, lastAt string) string {
	first, firstErr := time.Parse(time.RFC3339Nano, firstAt)
	last, lastErr := time.Parse(time.RFC3339Nano, lastAt)
	if firstErr != nil || lastErr != nil || last.Before(first) {
		return "unknown"
	}
	return last.Sub(first).Round(time.Millisecond).String()
}

func sessionViewerExcerpt(value string, limit int) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "No visible text.", false
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value, false
	}
	return string(runes[:limit]) + "…", true
}

func sessionViewerShellArg(value string) string {
	value = terminaloutput.SanitizeTerminalCell(value)
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
