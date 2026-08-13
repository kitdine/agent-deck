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
		activityStart := 1
		if page, found := pagination["activity"]; found {
			activityStart = sessionShowPageFirstOrdinal(page)
		}
		lines = append(lines, sessionShowActivityLinesFrom(value.Activity, width, activityStart)...)
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
		lines = append(lines, sessionShowTimestampFieldLines("EVENT AT", value.EventAt, width)...)
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
	return sessionShowActivityLinesFrom(values, width, 1)
}

func sessionShowPageFirstOrdinal(page session.Pagination) int {
	if page.Page <= 1 || page.Limit <= 0 {
		return 1
	}
	maxInt := int(^uint(0) >> 1)
	if page.Page-1 > (maxInt-1)/page.Limit {
		return maxInt
	}
	return (page.Page-1)*page.Limit + 1
}

func sessionShowActivityLinesFrom(values []activity.Detail, width, firstOrdinal int) []string {
	firstOrdinal = max(1, firstOrdinal)
	if len(values) > 0 && width >= 120 {
		if lines, ok := sessionShowActivityTableLines(values, width, firstOrdinal); ok {
			return lines
		}
	}
	if len(values) > 0 && width < 80 {
		return sessionShowActivityCompactLines(values, width, firstOrdinal)
	}
	if len(values) == 0 {
		return []string{sessionShowFit("No safe activity calls on this page.", width)}
	}
	lines := make([]string, 0, len(values)*3)
	for index, value := range values {
		fields := sessionShowActivityFields(value)
		if index > 0 {
			lines = append(lines, "")
		}
		primary := sessionShowActivityOrderedFields(fields, "TOOL", "STATUS")
		detail := sessionShowActivityOrderedFields(fields, "STARTED", "MODEL", "DURATION", "COMPLETED")
		lines = append(lines, sessionShowActivityWrappedFields(fmt.Sprintf("CALL %d", firstOrdinal+index), primary, width, 0)...)
		if len(detail) > 0 {
			lines = append(lines, sessionShowActivityWrappedFields("", detail, width, 2)...)
		} else if len(primary) == 0 {
			lines = append(lines, sessionShowActivityWrappedText("NO SAFE ACTIVITY METADATA", width, 2)...)
		}
	}
	return lines
}

type sessionShowActivityField struct {
	label string
	value string
}

func sessionShowActivityFields(value activity.Detail) []sessionShowActivityField {
	fields := make([]sessionShowActivityField, 0, 6)
	appendOptional := func(label, candidate string) {
		if visible, ok := sessionShowActivityOptional(candidate); ok {
			fields = append(fields, sessionShowActivityField{label: label, value: visible})
		}
	}
	if started, ok := sessionShowActivityOptional(value.StartedAt); ok {
		appendOptional("STARTED", sessionShowActivityTimestamp(started))
	}
	appendOptional("TOOL", value.Tool)
	appendOptional("MODEL", value.Model)
	appendOptional("STATUS", value.Status)
	if value.DurationMS != nil {
		fields = append(fields, sessionShowActivityField{label: "DURATION", value: sessionShowActivityDuration(*value.DurationMS)})
	} else if completed := strings.TrimSpace(value.CompletedAt); sessionShowActivityValidCompleted(completed) {
		appendOptional("COMPLETED", sessionShowActivityTimestamp(completed))
	}
	return fields
}

func sessionShowActivityTimestamp(value string) string {
	instant, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "—"
	}
	return instant.In(displayLocation()).Format("2006-01-02 15:04:05 MST")
}

func sessionShowActivityOptional(value string) (string, bool) {
	value = sessionShowKnown(value)
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unknown", "unavailable":
		return "", false
	default:
		return value, true
	}
}

func sessionShowActivityValidCompleted(value string) bool {
	if value == "" {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func sessionShowActivityDuration(value int64) string {
	return (time.Duration(value) * time.Millisecond).String()
}

func sessionShowActivityOrderedFields(fields []sessionShowActivityField, labels ...string) []sessionShowActivityField {
	ordered := make([]sessionShowActivityField, 0, len(labels))
	for _, label := range labels {
		if value, ok := sessionShowActivityFieldValue(fields, label); ok {
			ordered = append(ordered, sessionShowActivityField{label: label, value: value})
		}
	}
	return ordered
}

func sessionShowActivityWrappedFields(prefix string, fields []sessionShowActivityField, width, indent int) []string {
	parts := make([]string, 0, len(fields)+1)
	if prefix != "" {
		parts = append(parts, prefix)
	}
	for _, field := range fields {
		parts = append(parts, field.label+" "+field.value)
	}
	return sessionShowActivityWrappedText(strings.Join(parts, " · "), width, indent)
}

func sessionShowActivityWrappedText(value string, width, indent int) []string {
	indent = min(max(0, indent), max(0, width-1))
	wrapped := sessionShowWrap(value, max(1, width-indent))
	prefix := strings.Repeat(" ", indent)
	for index := range wrapped {
		wrapped[index] = prefix + wrapped[index]
	}
	return wrapped
}

func sessionShowActivityCompactLines(values []activity.Detail, width, firstOrdinal int) []string {
	lines := make([]string, 0, len(values)*7)
	for index, value := range values {
		if index > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sessionShowFit(fmt.Sprintf("CALL %d", firstOrdinal+index), width))
		fields := sessionShowActivityOrderedFields(sessionShowActivityFields(value), "TOOL", "STATUS", "STARTED", "MODEL", "DURATION", "COMPLETED")
		if len(fields) == 0 {
			lines = append(lines, sessionShowFieldLines("STATE", "NO SAFE ACTIVITY METADATA", width)...)
			continue
		}
		for _, field := range fields {
			lines = append(lines, sessionShowFieldLines(field.label, field.value, width)...)
		}
	}
	return lines
}

func sessionShowActivityTableLines(values []activity.Detail, width, firstOrdinal int) ([]string, bool) {
	records := make([][]sessionShowActivityField, len(values))
	labels := []string{"CALL"}
	for index, value := range values {
		records[index] = sessionShowActivityFields(value)
		if len(records[index]) == 0 {
			records[index] = []sessionShowActivityField{{label: "STATE", value: "NO SAFE ACTIVITY METADATA"}}
		}
	}
	for _, label := range []string{"STARTED", "TOOL", "MODEL", "STATUS", "DURATION", "COMPLETED", "STATE"} {
		for _, fields := range records {
			if _, ok := sessionShowActivityFieldValue(fields, label); ok {
				labels = append(labels, label)
				break
			}
		}
	}
	widths := make([]int, len(labels))
	for index, label := range labels {
		widths[index] = statsVisibleWidth(label)
	}
	for recordIndex, fields := range records {
		widths[0] = max(widths[0], statsVisibleWidth(strconv.Itoa(firstOrdinal+recordIndex)))
		for labelIndex, label := range labels[1:] {
			if value, ok := sessionShowActivityFieldValue(fields, label); ok {
				widths[labelIndex+1] = max(widths[labelIndex+1], statsVisibleWidth(value))
			}
		}
	}
	tableWidth := max(0, len(widths)-1)
	for _, columnWidth := range widths {
		tableWidth += columnWidth
	}
	if tableWidth > width {
		return nil, false
	}
	lines := []string{sessionShowActivityTableRow(labels, widths)}
	for recordIndex, fields := range records {
		row := make([]string, len(labels))
		row[0] = strconv.Itoa(firstOrdinal + recordIndex)
		for labelIndex, label := range labels[1:] {
			row[labelIndex+1], _ = sessionShowActivityFieldValue(fields, label)
		}
		lines = append(lines, sessionShowActivityTableRow(row, widths))
	}
	return lines, true
}

func sessionShowActivityFieldValue(fields []sessionShowActivityField, label string) (string, bool) {
	for _, field := range fields {
		if field.label == label {
			return field.value, true
		}
	}
	return "", false
}

func sessionShowActivityTableRow(values []string, widths []int) string {
	columns := make([]string, len(values))
	for index, value := range values {
		columns[index] = value + strings.Repeat(" ", max(0, widths[index]-statsVisibleWidth(value)))
	}
	return strings.TrimRight(strings.Join(columns, " "), " ")
}

func sessionShowUsageLines(value usage.SessionSummary, hasInvocations bool, width int) []string {
	primary := fmt.Sprintf("input %s · cached input %s · output %s",
		groupedInt(value.Tokens["input_tokens"]),
		groupedInt(value.Tokens["cached_input_tokens"]),
		groupedInt(value.Tokens["output_tokens"]),
	)
	cache := fmt.Sprintf("read %s · create %s · write 5m %s · write 1h %s · cache write %s",
		groupedInt(value.Tokens["cache_read_tokens"]),
		groupedInt(value.Tokens["cache_creation_tokens"]),
		groupedInt(value.Tokens["cache_write_5m_tokens"]),
		groupedInt(value.Tokens["cache_write_1h_tokens"]),
		groupedInt(value.Tokens["cache_write_tokens"]),
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
		lines = append(lines, sessionShowTimestampFieldLines("EVENT AT", value.EventAt, width)...)
		lines = append(lines, sessionShowFieldLines("MODEL", sessionShowKnown(value.Model), width)...)
		lines = append(lines, sessionShowFieldLines("PRIMARY TOKENS", fmt.Sprintf(
			"input %s · cached input %s · output %s",
			groupedInt(value.Tokens["input_tokens"]),
			groupedInt(value.Tokens["cached_input_tokens"]),
			groupedInt(value.Tokens["output_tokens"]),
		), width)...)
		lines = append(lines, sessionShowFieldLines("CACHE TOKENS", fmt.Sprintf(
			"read %s · create %s · write 5m %s · write 1h %s · cache write %s",
			groupedInt(value.Tokens["cache_read_tokens"]),
			groupedInt(value.Tokens["cache_creation_tokens"]),
			groupedInt(value.Tokens["cache_write_5m_tokens"]),
			groupedInt(value.Tokens["cache_write_1h_tokens"]),
			groupedInt(value.Tokens["cache_write_tokens"]),
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

func sessionShowTimestampFieldLines(label, value string, width int) []string {
	rendered := renderSessionDocumentTime(value)
	if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
		rendered = renderDisplayTimeWithZone(value)
	}
	return sessionShowFieldLines(label, rendered, width)
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
