package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/activity"
	terminaloutput "github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// renderSessionShowText is the copyable, screen-reader-friendly Session
// fallback. Every section uses bounded labeled continuation lines; ordinary
// output never changes into a data-dependent table at wider geometries.
func renderSessionShowText(
	w io.Writer,
	value session.Result,
	pagination map[string]session.Pagination,
	nextCommand string,
	usageSummary *usage.SessionSummary,
	invocations []usage.SessionInvocation,
	activityRequested bool,
	activityWarning string,
	sourceStale bool,
) error {
	width := sessionShowTextWidth(w)
	lines := []string{sessionShowFit("SESSION", width)}
	metadata := []struct{ label, value string }{
		{label: "CLIENT", value: sessionShowKnown(value.Client)},
		{label: "SESSION ID", value: sessionShowKnown(value.SessionID)},
		{label: "PROJECT", value: sessionShowKnown(value.Project)},
		{label: "MODEL", value: sessionShowKnown(value.Model)},
		{label: "FIRST ACTIVITY", value: sessionShowKnown(renderDisplayTimeWithZone(value.FirstAt))},
		{label: "LAST ACTIVITY", value: sessionShowKnown(renderDisplayTimeWithZone(value.LastAt))},
		{label: "SESSION SPAN", value: sessionShowSpan(value.FirstAt, value.LastAt)},
	}
	for _, field := range metadata {
		lines = append(lines, sessionShowFieldLines(field.label, field.value, width)...)
	}

	lines = sessionShowSection(lines, "DOCUMENTS", width)
	if sourceStale {
		lines = append(lines, sessionShowFieldLines("WARNING", "Indexed documents may be stale because the selected source is unavailable.", width)...)
	}
	lines = append(lines, sessionShowDocumentLines(value.Documents, width)...)
	if page, found := pagination["documents"]; found {
		lines = append(lines, sessionShowPaginationLines(page, nextCommand, width)...)
	}

	if value.ActivitySummary != nil || activityRequested || len(value.Activity) > 0 {
		lines = sessionShowSection(lines, "ACTIVITY", width)
		if activityWarning != "" {
			lines = append(lines, sessionShowFieldLines("WARNING", activityWarning, width)...)
		}
		lines = append(lines, sessionShowActivitySummaryLines(value.ActivitySummary, width)...)
		lines = append(lines, sessionShowActivityLines(value.Activity, width)...)
		if page, found := pagination["activity"]; found {
			lines = append(lines, sessionShowPaginationLines(page, nextCommand, width)...)
		}
	}

	if usageSummary != nil {
		lines = sessionShowSection(lines, "TOKENS", width)
		hasInvocations := len(invocations) > 0 || usageSummary.FirstAt != "" || usageSummary.LastAt != ""
		if page, found := pagination["invocations"]; found && page.Total > 0 {
			hasInvocations = true
		}
		lines = append(lines, sessionShowUsageLines(*usageSummary, hasInvocations, width)...)
		lines = sessionShowSection(lines, "INVOCATIONS", width)
		lines = append(lines, sessionShowInvocationLines(invocations, width)...)
		if page, found := pagination["invocations"]; found {
			lines = append(lines, sessionShowPaginationLines(page, nextCommand, width)...)
		}
	}
	return sessionShowWriteLines(w, lines)
}

func sessionShowTextWidth(w io.Writer) int {
	width := 100
	if file, ok := w.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		if columns, _, err := term.GetSize(int(file.Fd())); err == nil && columns > 0 {
			width = columns
		}
	}
	if columns, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && columns > 0 {
		width = columns
	}
	return max(1, width)
}

func sessionShowSection(lines []string, title string, width int) []string {
	if len(lines) > 0 && lines[len(lines)-1] != "" {
		lines = append(lines, "")
	}
	return append(lines, sessionShowFit(title, width))
}

func sessionShowDocumentLines(values []session.Document, width int) []string {
	if len(values) == 0 {
		return []string{sessionShowFit("No approved visible documents on this page.", width)}
	}
	lines := make([]string, 0, len(values)*5)
	for index, value := range values {
		if index > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sessionShowFit(fmt.Sprintf("DOCUMENT %d", index+1), width))
		lines = append(lines, sessionShowFieldLines("EVENT AT", sessionShowKnown(renderSessionDocumentTime(value.EventAt)), width)...)
		lines = append(lines, sessionShowFieldLines("KIND", sessionShowKnown(value.Kind), width)...)
		lines = append(lines, sessionShowFieldLines("TEXT", sessionShowKnown(value.Text), width)...)
	}
	return lines
}

func sessionShowActivitySummaryLines(summary *session.ActivitySummary, width int) []string {
	if summary == nil {
		return nil
	}
	average := "unavailable"
	if summary.AverageDurationMS != nil {
		average = sessionShowMilliseconds(*summary.AverageDurationMS)
	}
	lines := sessionShowFieldLines("SUMMARY", fmt.Sprintf(
		"%s calls · %s completed · %s failed · %s incomplete",
		groupedInt(int64(summary.Total)),
		groupedInt(int64(summary.Completed)),
		groupedInt(int64(summary.Failed)),
		groupedInt(int64(summary.Incomplete)),
	), width)
	lines = append(lines, sessionShowFieldLines("DURATION", "total "+sessionShowMilliseconds(summary.TotalDurationMS)+" · average "+average, width)...)
	if len(summary.ByTool) > 0 {
		tools := make([]string, 0, len(summary.ByTool))
		for _, item := range summary.ByTool {
			tools = append(tools, sessionShowKnown(item.Tool)+" "+groupedInt(int64(item.Count)))
		}
		lines = append(lines, sessionShowFieldLines("BY TOOL", strings.Join(tools, " · "), width)...)
	}
	return lines
}

func sessionShowActivityLines(values []activity.Detail, width int) []string {
	if len(values) == 0 {
		return []string{sessionShowFit("No safe activity calls on this page.", width)}
	}
	lines := make([]string, 0, len(values)*7)
	for index, value := range values {
		if index > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sessionShowFit(fmt.Sprintf("CALL %d", index+1), width))
		lines = append(lines, sessionShowFieldLines("STARTED", sessionShowKnown(renderDisplayTime(value.StartedAt)), width)...)
		lines = append(lines, sessionShowFieldLines("TOOL", sessionShowKnown(value.Tool), width)...)
		lines = append(lines, sessionShowFieldLines("MODEL", sessionShowKnown(value.Model), width)...)
		lines = append(lines, sessionShowFieldLines("STATUS", sessionShowKnown(value.Status), width)...)
		duration := "unavailable"
		if value.DurationMS != nil {
			duration = sessionShowMilliseconds(*value.DurationMS)
		}
		lines = append(lines, sessionShowFieldLines("DURATION", duration, width)...)
		if value.CompletedAt != "" {
			lines = append(lines, sessionShowFieldLines("COMPLETED", sessionShowKnown(renderDisplayTime(value.CompletedAt)), width)...)
		}
	}
	return lines
}

func sessionShowUsageLines(value usage.SessionSummary, hasInvocations bool, width int) []string {
	primary := fmt.Sprintf("input %s · cached input %s · output %s",
		groupedInt(value.Tokens["input_tokens"]),
		groupedInt(value.Tokens["cached_input_tokens"]),
		groupedInt(value.Tokens["output_tokens"]),
	)
	cache := fmt.Sprintf("read %s · create %s · write 5m %s · write 1h %s",
		groupedInt(value.Tokens["cache_read_tokens"]),
		groupedInt(value.Tokens["cache_creation_tokens"]),
		groupedInt(value.Tokens["cache_write_5m_tokens"]),
		groupedInt(value.Tokens["cache_write_1h_tokens"]),
	)
	catalogCost := sessionCostText(value.CatalogBaseCost, value.KnownCatalogBaseCost)
	providerCost := sessionCostText(value.ProviderCost, value.KnownProviderCost)
	pricing := sessionShowSummaryPricing(value)
	if !hasInvocations {
		catalogCost, providerCost, pricing = "not applicable", "not applicable", "not applicable"
	}
	lines := sessionShowFieldLines("PRIMARY TOKENS", primary, width)
	lines = append(lines, sessionShowFieldLines("CACHE TOKENS", cache, width)...)
	lines = append(lines, sessionShowFieldLines("CATALOG COST", catalogCost, width)...)
	lines = append(lines, sessionShowFieldLines("PROVIDER COST", providerCost, width)...)
	lines = append(lines, sessionShowFieldLines("PRICING", pricing, width)...)
	lines = append(lines, sessionShowFieldLines("UNPRICED", sessionShowList(value.Unpriced), width)...)
	lines = append(lines, sessionShowFieldLines("WARNINGS", sessionShowList(value.Warnings), width)...)
	return lines
}

func sessionShowInvocationLines(values []usage.SessionInvocation, width int) []string {
	if len(values) == 0 {
		return []string{
			sessionShowFit("No normalized invocations on this page.", width),
			sessionShowFit("Sequence numbers are chronological usage positions, not conversation turns.", width),
		}
	}
	lines := []string{sessionShowFit("Sequence numbers are chronological usage positions, not conversation turns.", width)}
	for _, value := range values {
		lines = append(lines, "", sessionShowFit("INVOCATION #"+strconv.Itoa(value.Sequence), width))
		lines = append(lines, sessionShowFieldLines("EVENT AT", sessionShowKnown(renderDisplayTime(value.EventAt)), width)...)
		lines = append(lines, sessionShowFieldLines("MODEL", sessionShowKnown(value.Model), width)...)
		lines = append(lines, sessionShowFieldLines("PRIMARY TOKENS", fmt.Sprintf(
			"input %s · cached input %s · output %s",
			groupedInt(value.Tokens["input_tokens"]),
			groupedInt(value.Tokens["cached_input_tokens"]),
			groupedInt(value.Tokens["output_tokens"]),
		), width)...)
		lines = append(lines, sessionShowFieldLines("CACHE TOKENS", fmt.Sprintf(
			"read %s · create %s · write 5m %s · write 1h %s",
			groupedInt(value.Tokens["cache_read_tokens"]),
			groupedInt(value.Tokens["cache_creation_tokens"]),
			groupedInt(value.Tokens["cache_write_5m_tokens"]),
			groupedInt(value.Tokens["cache_write_1h_tokens"]),
		), width)...)
		lines = append(lines, sessionShowFieldLines("CATALOG COST", sessionShowInvocationCost(value.CatalogBaseCost, value.KnownCatalogBaseCost, value.Unpriced), width)...)
		lines = append(lines, sessionShowFieldLines("PROVIDER COST", sessionShowInvocationCost(value.ProviderCost, value.KnownProviderCost, value.Unpriced), width)...)
		lines = append(lines, sessionShowFieldLines("PRICING", sessionShowInvocationPricing(value), width)...)
		lines = append(lines, sessionShowFieldLines("UNPRICED", sessionShowList(value.Unpriced), width)...)
		lines = append(lines, sessionShowFieldLines("WARNINGS", sessionShowList(value.Warnings), width)...)
	}
	return lines
}

func sessionShowPaginationLines(page session.Pagination, nextCommand string, width int) []string {
	first, last := 0, 0
	if page.Shown > 0 {
		first = (page.Page-1)*page.Limit + 1
		last = first + page.Shown - 1
	}
	lines := sessionShowFieldLines("SHOWING", fmt.Sprintf("%d-%d of %d", first, last, page.Total), width)
	if page.HasMore && nextCommand != "" {
		lines = append(lines, sessionShowFieldLines("NEXT PAGE", nextCommand, width)...)
	}
	return lines
}

func sessionShowFieldLines(label, value string, width int) []string {
	width = max(1, width)
	label = strings.ToUpper(sessionShowSafe(label))
	value = sessionShowKnown(value)
	if width < 40 {
		lines := []string{sessionShowFit(label+":", width)}
		indent := min(2, max(0, width-1))
		budget := max(1, width-indent)
		for _, wrapped := range sessionShowWrap(value, budget) {
			lines = append(lines, strings.Repeat(" ", indent)+sessionShowFit(wrapped, budget))
		}
		return lines
	}
	labelWidth := min(16, max(10, width/5))
	prefix := "  " + statsPad(sessionShowFit(label, labelWidth), labelWidth) + "  "
	budget := max(1, width-statsVisibleWidth(prefix))
	wrapped := sessionShowWrap(value, budget)
	lines := make([]string, 0, len(wrapped))
	for index, line := range wrapped {
		if index == 0 {
			lines = append(lines, prefix+line)
		} else {
			lines = append(lines, strings.Repeat(" ", statsVisibleWidth(prefix))+line)
		}
	}
	return lines
}

func sessionShowWrap(value string, width int) []string {
	value = sessionShowKnown(value)
	width = max(1, width)
	lines := make([]string, 0, 2)
	current := ""
	flush := func() {
		if current != "" {
			lines = append(lines, current)
			current = ""
		}
	}
	for _, word := range strings.Fields(value) {
		parts := sessionShowHardWrapWord(word, width)
		if len(parts) == 1 && current != "" && runewidth.StringWidth(current)+1+runewidth.StringWidth(parts[0]) <= width {
			current += " " + parts[0]
			continue
		}
		flush()
		for index, part := range parts {
			if index < len(parts)-1 || runewidth.StringWidth(part) == width {
				lines = append(lines, part)
			} else {
				current = part
			}
		}
	}
	flush()
	if len(lines) == 0 {
		return []string{"unknown"}
	}
	return lines
}

func sessionShowHardWrapWord(word string, width int) []string {
	parts := make([]string, 0, 2)
	var part strings.Builder
	used := 0
	flush := func() {
		if part.Len() > 0 {
			parts = append(parts, part.String())
			part.Reset()
			used = 0
		}
	}
	for _, r := range word {
		size := max(0, runewidth.RuneWidth(r))
		if size > width {
			flush()
			parts = append(parts, sessionShowFit(string(r), width))
			continue
		}
		if used+size > width {
			flush()
		}
		part.WriteRune(r)
		used += size
	}
	flush()
	if len(parts) == 0 {
		return []string{"unknown"}
	}
	return parts
}

func sessionShowKnown(value string) string {
	value = strings.TrimSpace(sessionShowSafe(value))
	if value == "" {
		return "unknown"
	}
	return value
}

func sessionShowSafe(value string) string {
	return terminaloutput.SanitizeTerminalCell(value)
}

func sessionShowList(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	safe := make([]string, 0, len(values))
	for _, value := range values {
		safe = append(safe, sessionShowKnown(value))
	}
	return strings.Join(safe, " · ")
}

func sessionShowSpan(firstAt, lastAt string) string {
	first, firstErr := time.Parse(time.RFC3339Nano, firstAt)
	last, lastErr := time.Parse(time.RFC3339Nano, lastAt)
	if firstErr != nil || lastErr != nil || last.Before(first) {
		return "unknown"
	}
	return last.Sub(first).Round(time.Millisecond).String()
}

func sessionShowMilliseconds(value int64) string {
	return (time.Duration(value) * time.Millisecond).String() + " (" + groupedInt(value) + " ms)"
}

func sessionShowSummaryPricing(value usage.SessionSummary) string {
	switch {
	case value.ProviderCost != nil && len(value.Unpriced) == 0:
		return "complete"
	case value.KnownProviderCost != nil || value.KnownCatalogBaseCost != nil || value.ProviderCost != nil || value.CatalogBaseCost != nil:
		return "partial"
	default:
		return "unpriced"
	}
}

func sessionShowInvocationCost(total *string, known string, unpriced []string) string {
	if total != nil && len(unpriced) == 0 {
		return *total
	}
	if known != "" {
		return known + " (partial)"
	}
	if total != nil {
		return *total + " (partial)"
	}
	return "unpriced"
}

func sessionShowInvocationPricing(value usage.SessionInvocation) string {
	switch {
	case value.ProviderCost != nil && len(value.Unpriced) == 0:
		return "complete"
	case value.KnownProviderCost != "" || value.KnownCatalogBaseCost != "" || value.ProviderCost != nil || value.CatalogBaseCost != nil:
		return "partial"
	default:
		return "unpriced"
	}
}

func sessionShowWriteLines(w io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, strings.TrimRight(line, " ")); err != nil {
			return err
		}
	}
	return nil
}

func sessionShowFit(value string, width int) string {
	value = sessionShowSafe(value)
	width = max(1, width)
	if runewidth.StringWidth(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	var result strings.Builder
	used := 0
	for _, r := range value {
		size := runewidth.RuneWidth(r)
		if used+size > width-1 {
			break
		}
		result.WriteRune(r)
		used += size
	}
	return result.String() + "…"
}
