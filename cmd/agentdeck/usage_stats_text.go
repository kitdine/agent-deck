package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"

	"github.com/kitdine/agent-deck/internal/usage"
)

const (
	statsMinWidth     = 48
	statsDefaultWidth = 100
	statsMaxWidth     = 160
)

type usageTextRenderOptions struct {
	width int
	color bool
}

func newUsageTextRenderOptions(w io.Writer, noColor bool) usageTextRenderOptions {
	width := statsDefaultWidth
	terminal := false
	if file, ok := w.(*os.File); ok {
		terminal = term.IsTerminal(int(file.Fd()))
		if terminal {
			if columns, _, err := term.GetSize(int(file.Fd())); err == nil && columns > 0 {
				width = columns
			}
		}
	}
	if raw := os.Getenv("COLUMNS"); raw != "" {
		if columns, err := strconv.Atoi(raw); err == nil && columns > 0 {
			width = columns
		}
	}
	width = min(max(width, statsMinWidth), statsMaxWidth)
	color := terminal && !noColor && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	return usageTextRenderOptions{width: width, color: color}
}

func renderUsageStats(w io.Writer, report usage.StatsReport) error {
	return renderUsageStatsWithOptions(w, report, usageTextRenderOptions{})
}

func renderUsageStatsWithOptions(w io.Writer, report usage.StatsReport, options usageTextRenderOptions) error {
	if options.width == 0 {
		options.width = statsDefaultWidth
	}
	options.width = min(max(options.width, statsMinWidth), statsMaxWidth)
	renderer := statsTextRenderer{report: report, width: options.width, color: options.color}
	_, err := io.WriteString(w, renderer.render())
	return err
}

type statsTextRenderer struct {
	report usage.StatsReport
	width  int
	color  bool
}

func (r statsTextRenderer) render() string {
	var out strings.Builder
	title := "📊 USAGE STATS · " + r.rangeLabel()
	out.WriteString(r.style(title, "1;32"))
	out.WriteByte('\n')
	out.WriteString(r.metaLine())
	out.WriteString("\n\n")
	for _, line := range r.kpiLines() {
		out.WriteString(line)
		out.WriteByte('\n')
	}
	out.WriteByte('\n')
	if r.width >= 104 {
		leftWidth := (r.width - 4) * 3 / 5
		rightWidth := r.width - 4 - leftWidth
		for _, line := range joinStatsColumns(r.trendLines(leftWidth), leftWidth, r.rankingLines(rightWidth), rightWidth, 4) {
			out.WriteString(line)
			out.WriteByte('\n')
		}
	} else {
		blocks := [][]string{r.trendLines(r.width), r.rankingLines(r.width)}
		for blockIndex, block := range blocks {
			for _, line := range block {
				out.WriteString(line)
				out.WriteByte('\n')
			}
			if blockIndex < len(blocks)-1 {
				out.WriteByte('\n')
			}
		}
	}
	out.WriteByte('\n')
	for _, line := range r.summaryLines() {
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if r.report.ShowModelActivity && len(r.report.Models) == 1 {
		out.WriteByte('\n')
		for _, line := range r.modelActivityLines(r.report.Models[0]) {
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	if len(r.report.Activity) > 0 {
		out.WriteByte('\n')
		for _, line := range r.activityLines() {
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	if len(r.report.UnpricedModels) > 0 {
		out.WriteByte('\n')
		for _, line := range r.unpricedLines() {
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func (r statsTextRenderer) rangeLabel() string {
	from, fromErr := time.Parse(time.RFC3339Nano, r.report.Range.From)
	to, toErr := time.Parse(time.RFC3339Nano, r.report.Range.To)
	if fromErr != nil || toErr != nil {
		return "CUSTOM RANGE"
	}
	days := 1
	date := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	last := to.Add(-time.Nanosecond)
	lastDate := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, time.UTC)
	for date.Before(lastDate) && days < 10000 {
		date = date.AddDate(0, 0, 1)
		days++
	}
	if days == 1 {
		return "TODAY"
	}
	return fmt.Sprintf("LAST %d DAYS", days)
}

func (r statsTextRenderer) metaLine() string {
	from, to := compactStatsDisplayRange(r.report.Range)
	metadata := fmt.Sprintf("%s - %s · %s · %s · %s · %s events", from, to, r.report.Timezone, r.report.GroupBy, r.report.Metric, groupedInt(r.report.Totals.Events))
	return statsFit(metadata, r.width)
}

func (r statsTextRenderer) kpiLines() []string {
	values := []struct{ label, value string }{
		{label: "TOKENS", value: compactNumber(float64(r.report.Totals.Tokens))},
		{label: "COST", value: compactCost(r.report.Totals.ProviderCost, r.report.Totals.KnownProviderCost, r.hasKnownProviderCost())},
		{label: "SESSIONS", value: groupedInt(r.report.Totals.Sessions)},
	}
	inner := r.width - 4
	base := inner / 3
	widths := []int{base, base, inner - base*2}
	border := func(left, middle, right string) string {
		return left + strings.Repeat("─", widths[0]) + middle + strings.Repeat("─", widths[1]) + middle + strings.Repeat("─", widths[2]) + right
	}
	labels := "│"
	numbers := "│"
	for index, value := range values {
		labels += " " + statsPad(value.label, widths[index]-2) + " │"
		numbers += " " + statsPad(r.style(value.value, "1;37"), widths[index]-2) + " │"
	}
	return []string{border("┌", "┬", "┐"), labels, numbers, border("└", "┴", "┘")}
}

func (r statsTextRenderer) trendLines(width int) []string {
	metricLabel := strings.ToUpper(r.report.Metric)
	lines := []string{r.sectionTitle("🗓 TREND · "+metricLabel, width, "1;34")}
	labels := compactBucketLabels(r.report.Buckets, r.report.GroupBy)
	maximum := float64(0)
	values := make([]float64, len(r.report.Buckets))
	valueLabels := make([]string, len(r.report.Buckets))
	for index, bucket := range r.report.Buckets {
		values[index] = statsBucketMetric(bucket, r.report.Metric)
		valueLabels[index] = compactMetric(values[index], r.report.Metric)
		if r.report.Metric == "cost" {
			valueLabels[index] = compactCost(bucket.MetricValue, bucket.KnownMetricValue, knownCostAvailable(bucket.MetricValue, bucket.KnownMetricValue, bucket.Coverage))
		}
		maximum = math.Max(maximum, values[index])
	}
	labelWidth := 7
	for _, label := range labels {
		labelWidth = max(labelWidth, statsVisibleWidth(label))
	}
	valueWidth := 9
	for _, value := range valueLabels {
		valueWidth = max(valueWidth, statsVisibleWidth(value))
	}
	labelWidth = min(labelWidth, max(7, width-valueWidth-12))
	barWidth := min(52, max(8, width-labelWidth-valueWidth-4))
	for index := range r.report.Buckets {
		label := labels[index]
		filled := scaledBar(values[index], maximum, barWidth)
		bar := r.style(strings.Repeat("█", filled), "34") + strings.Repeat("░", barWidth-filled)
		lines = append(lines, statsPad(label, labelWidth)+"  "+bar+"  "+statsPadLeft(valueLabels[index], valueWidth))
	}
	if len(r.report.Buckets) == 0 {
		lines = append(lines, r.style("No activity in this range.", "2"))
	}
	return lines
}

func (r statsTextRenderer) rankingLines(width int) []string {
	rankingLabel := "🤖 MODELS"
	if r.report.Metric == "cost" {
		switch {
		case r.hasPartialCost():
			rankingLabel += " · KNOWN COST"
		case !r.hasKnownProviderCost():
			rankingLabel += " · COST UNAVAILABLE"
		}
	}
	lines := []string{r.sectionTitle(rankingLabel, width, "1;35")}
	maximum := float64(0)
	limit := len(r.report.Models)
	shares := make([]float64, limit)
	shareLabels := make([]string, limit)
	for index := 0; index < limit; index++ {
		model := r.report.Models[index]
		shares[index], _ = strconv.ParseFloat(model.KnownShare, 64)
		shareLabels[index] = formatPercent(shares[index])
		if r.report.Metric == "cost" {
			if !knownCostAvailable(model.MetricValue, model.KnownMetricValue, model.Coverage) {
				shares[index] = 0
				shareLabels[index] = "unavailable"
			}
		}
		maximum = math.Max(maximum, shares[index])
	}
	nameWidth := min(23, max(14, width/2))
	shareWidth := 6
	for _, label := range shareLabels {
		shareWidth = max(shareWidth, statsVisibleWidth(label))
	}
	barWidth := min(36, max(6, width-nameWidth-shareWidth-5))
	for index := 0; index < limit; index++ {
		model := r.report.Models[index]
		name := statsFit(model.Name, nameWidth)
		filled := scaledBar(shares[index], maximum, barWidth)
		bar := r.style(strings.Repeat("█", filled), "35") + strings.Repeat("░", barWidth-filled)
		lines = append(lines, statsPad(name, nameWidth)+" "+bar+" "+statsPadLeft(shareLabels[index], shareWidth))
		cost := compactCost(model.ProviderCost, model.KnownProviderCost, knownCostAvailable(model.ProviderCost, model.KnownProviderCost, model.Coverage))
		detail := fmt.Sprintf("%s tokens · %s · %s · %s · %s sessions · %s tools", compactNumber(float64(model.Tokens)), shareLabels[index], cost, modelPricingStatus(model), groupedInt(model.Sessions), groupedInt(modelToolCalls(model)))
		if model.CacheHitRate != nil && (model.CachedReadTokens > 0 || model.CacheWriteTokens > 0) {
			detail += " · " + *model.CacheHitRate + "% hit"
		}
		for _, detailLine := range statsWrap(detail, width) {
			lines = append(lines, r.style(detailLine, "2"))
		}
	}
	if limit == 0 {
		lines = append(lines, r.style("No models in this range.", "2"))
	}
	clientLabel := "CLIENTS"
	if r.report.Metric == "cost" {
		switch {
		case r.hasPartialCost():
			clientLabel += " · KNOWN COST"
		case !r.hasKnownProviderCost():
			clientLabel += " · COST UNAVAILABLE"
		}
	}
	lines = append(lines, "", r.sectionTitle(clientLabel, width, "1;36"))
	for _, client := range r.report.Clients {
		share, _ := strconv.ParseFloat(client.KnownShare, 64)
		shareLabel := formatPercent(share)
		if r.report.Metric == "cost" {
			if !knownCostAvailable(client.MetricValue, client.KnownMetricValue, client.Coverage) {
				share = 0
				shareLabel = "unavailable"
			}
		}
		nameWidth := min(10, max(6, width/5))
		shareWidth := max(6, statsVisibleWidth(shareLabel))
		barWidth := min(40, max(8, width-nameWidth-shareWidth-3))
		filled := scaledBar(share, 100, barWidth)
		bar := r.style(strings.Repeat("█", filled), "36") + strings.Repeat("░", barWidth-filled)
		lines = append(lines, statsPad(statsTitle(client.Name), nameWidth)+" "+bar+" "+statsPadLeft(shareLabel, shareWidth))
	}
	cacheLines := r.cacheLines(width)
	if len(cacheLines) > 0 {
		lines = append(lines, "", r.sectionTitle("CACHE HIT RATE", width, "1;33"))
		lines = append(lines, cacheLines...)
	}
	return lines
}

func modelPricingStatus(model usage.StatsDimension) string {
	if model.ProviderCost != nil {
		return "PRICED"
	}
	if knownCostAvailable(model.ProviderCost, model.KnownProviderCost, model.Coverage) {
		return "PARTIAL"
	}
	return "UNPRICED"
}

func (r statsTextRenderer) cacheLines(width int) []string {
	var lines []string
	for _, model := range r.report.Models {
		if model.CacheHitRate == nil || model.CachedReadTokens == 0 && model.CacheWriteTokens == 0 {
			continue
		}
		detail := fmt.Sprintf("MODEL %s/%s  %s%% hit · read %s · write %s", statsTitle(model.Client), model.Name, *model.CacheHitRate, compactNumber(float64(model.CachedReadTokens)), compactNumber(float64(model.CacheWriteTokens)))
		if model.Client == "claude" {
			detail += " · logical input " + compactNumber(float64(model.LogicalInputTokens))
		}
		lines = append(lines, statsWrap(detail, width)...)
	}
	limit := min(10, len(r.report.CacheSessions))
	for index := 0; index < limit; index++ {
		session := r.report.CacheSessions[index]
		rate := "0.00"
		if session.CacheHitRate != nil {
			rate = *session.CacheHitRate
		}
		models := strings.Join(session.Models, ",")
		detail := fmt.Sprintf("SESSION %s/%s  %s%% hit · read %s · write %s · %s", statsTitle(session.Client), session.SessionID, rate, compactNumber(float64(session.CachedReadTokens)), compactNumber(float64(session.CacheWriteTokens)), models)
		lines = append(lines, statsWrap(detail, width)...)
		for _, commandLine := range statsWrap(session.DetailCommand, width) {
			lines = append(lines, r.style(commandLine, "2"))
		}
	}
	if omitted := len(r.report.CacheSessions) - limit; omitted > 0 {
		lines = append(lines, r.style(fmt.Sprintf("+%d more cache sessions in JSON", omitted), "2"))
	}
	return lines
}

func (r statsTextRenderer) modelActivityLines(model usage.StatsDimension) []string {
	activity := usage.StatsModelActivity{}
	if model.Activity != nil {
		activity = *model.Activity
	}
	lines := []string{r.sectionTitle("MODEL ACTIVITY · "+model.Name, r.width, "1;33")}
	summary := fmt.Sprintf("%s sessions · %s active days · %s tools · %s completed · %s failed", groupedInt(activity.ActiveSessions), groupedInt(activity.ActiveDays), groupedInt(activity.ToolCalls), groupedInt(activity.CompletedCalls), groupedInt(activity.FailedCalls))
	lines = append(lines, statsWrap(summary, r.width)...)
	if activity.FirstAt != "" {
		lines = append(lines, statsWrap("range "+activity.FirstAt+" - "+activity.LastAt, r.width)...)
	}
	if activity.AverageDuration != nil {
		lines = append(lines, fmt.Sprintf("timed duration %s ms total · %s ms average", groupedInt(activity.TotalDurationMS), groupedInt(*activity.AverageDuration)))
	}
	for _, tool := range activity.Tools {
		lines = append(lines, statsWrap(fmt.Sprintf("%s  %s calls", tool.Name, groupedInt(tool.Calls)), r.width)...)
	}
	if len(activity.Tools) == 0 {
		lines = append(lines, r.style("No tool activity in this range.", "2"))
	}
	return lines
}

func modelToolCalls(model usage.StatsDimension) int64 {
	if model.Activity == nil {
		return 0
	}
	return model.Activity.ToolCalls
}

func (r statsTextRenderer) unpricedLines() []string {
	lines := []string{r.sectionTitle("UNPRICED MODELS", r.width, "1;33")}
	for _, model := range r.report.UnpricedModels {
		entry := fmt.Sprintf("%s/%s · %s", statsTitle(model.Client), model.Model, strings.Join(model.Components, ", "))
		lines = append(lines, statsWrap(entry, r.width)...)
	}
	return lines
}

func (r statsTextRenderer) summaryLines() []string {
	average := compactCost(r.report.Totals.AverageCost, r.report.Totals.KnownAverageCost, r.hasKnownProviderCost())
	peakValue, _ := strconv.ParseFloat(r.report.Peak.KnownValue, 64)
	peak := compactMetric(peakValue, r.report.Metric)
	if r.report.Metric == "cost" {
		peak = compactCost(r.report.Peak.Value, r.report.Peak.KnownValue, knownCostAvailable(r.report.Peak.Value, r.report.Peak.KnownValue, r.report.Peak.Coverage))
	}
	items := []string{"AVG COST  " + average, "PEAK  " + peak, "PRICED  " + r.report.Coverage.Percent + "%"}
	inner := r.width - 2
	base := inner / 3
	widths := []int{base, base, inner - base*2}
	for index, item := range items {
		if statsVisibleWidth(item) > widths[index] {
			return append([]string{strings.Repeat("─", r.width)}, items...)
		}
	}
	line := ""
	for index, item := range items {
		line += statsPad(item, widths[index])
	}
	return []string{strings.Repeat("─", r.width), strings.TrimRight(line, " ")}
}

func (r statsTextRenderer) activityLines() []string {
	metricLabel := strings.ToUpper(r.report.Metric)
	if r.report.Metric == "cost" {
		if !r.hasKnownProviderCost() {
			return []string{
				r.sectionTitle("▦ ACTIVITY BY WEEKDAY / HOUR · COST", r.width, "1;32"),
				"unavailable: no priced events",
			}
		}
		if r.hasPartialCost() {
			metricLabel = "KNOWN COST"
		}
	}
	lines := []string{r.sectionTitle("▦ ACTIVITY BY WEEKDAY / HOUR · "+metricLabel, r.width, "1;32")}
	values := make([]float64, 7*24)
	maximum := float64(0)
	for _, activity := range r.report.Activity {
		if activity.Weekday < 0 || activity.Weekday >= 7 || activity.Hour < 0 || activity.Hour >= 24 {
			continue
		}
		value, _ := strconv.ParseFloat(activity.KnownMetricValue, 64)
		values[activity.Weekday*24+activity.Hour] = value
		maximum = math.Max(maximum, value)
	}
	cellSeparator := " "
	if r.width < 58 {
		cellSeparator = ""
		lines = append(lines, "     00 03 06 09 12 15 18 21")
	} else {
		lines = append(lines, "     00    03    06    09    12    15    18    21")
	}
	weekdays := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	for weekday := 0; weekday < 7; weekday++ {
		line := weekdays[weekday] + "  "
		for hour := 0; hour < 24; hour++ {
			level := heatLevel(values[weekday*24+hour], maximum)
			cell := []string{"·", "░", "▒", "▓", "█"}[level]
			if r.color && level > 0 {
				cell = r.style(cell, []string{"", "32", "1;32", "1;92", "1;97;42"}[level])
			}
			line += cell + cellSeparator
		}
		lines = append(lines, strings.TrimRight(line, " "))
	}
	legend := "LESS  · ░ ▒ ▓ █  MORE"
	from, to := compactStatsDisplayRange(r.report.Range)
	rangeText := from + " - " + to
	gap := r.width - runewidth.StringWidth(legend) - runewidth.StringWidth(rangeText)
	if gap >= 1 {
		lines = append(lines, legend+strings.Repeat(" ", gap)+rangeText)
	} else {
		lines = append(lines, legend, statsPadLeft(rangeText, r.width))
	}
	return lines
}

func (r statsTextRenderer) sectionTitle(label string, width int, color string) string {
	plain := label + " "
	return r.style(label, color) + " " + strings.Repeat("─", max(0, width-runewidth.StringWidth(plain)))
}

func (r statsTextRenderer) hasPartialCost() bool {
	return r.report.Totals.ProviderCost == nil && r.hasKnownProviderCost()
}

func (r statsTextRenderer) hasKnownProviderCost() bool {
	if r.report.Totals.ProviderCost != nil || r.report.Coverage.PricedEvents > 0 {
		return true
	}
	value, err := strconv.ParseFloat(r.report.Totals.KnownProviderCost, 64)
	return err == nil && value != 0
}

func (r statsTextRenderer) style(value, code string) string {
	if !r.color || value == "" {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func joinStatsColumns(left []string, leftWidth int, right []string, rightWidth, gap int) []string {
	count := max(len(left), len(right))
	lines := make([]string, 0, count)
	for index := 0; index < count; index++ {
		leftLine, rightLine := "", ""
		if index < len(left) {
			leftLine = left[index]
		}
		if index < len(right) {
			rightLine = right[index]
		}
		lines = append(lines, strings.TrimRight(statsPad(leftLine, leftWidth)+strings.Repeat(" ", gap)+statsFit(rightLine, rightWidth), " "))
	}
	return lines
}

func compactStatsDate(value string) string {
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return at.Format("Jan 02, 2006")
}

func compactStatsDisplayRange(value usage.StatsRange) (string, string) {
	from, fromErr := time.Parse(time.RFC3339Nano, value.From)
	to, toErr := time.Parse(time.RFC3339Nano, value.To)
	if fromErr != nil || toErr != nil {
		return compactStatsDate(value.From), compactStatsDate(value.To)
	}
	if to.After(from) {
		to = to.Add(-time.Nanosecond)
	}
	return from.Format("Jan 02, 2006"), to.Format("Jan 02, 2006")
}

func compactBucketLabels(buckets []usage.StatsBucket, group string) []string {
	labels := make([]string, len(buckets))
	if group != "hour" {
		for index, bucket := range buckets {
			labels[index] = compactBucketLabel(bucket.Start, group)
		}
		return labels
	}
	parsed := make([]time.Time, len(buckets))
	dates := map[string]struct{}{}
	for index, bucket := range buckets {
		at, err := time.Parse(time.RFC3339Nano, bucket.Start)
		if err != nil {
			labels[index] = bucket.Start
			continue
		}
		parsed[index] = at
		dates[at.Format("2006-01-02")] = struct{}{}
	}
	includeDate := len(dates) > 1
	counts := map[string]int{}
	for index, at := range parsed {
		if at.IsZero() {
			continue
		}
		format := "15:04"
		if includeDate {
			format = "Jan 02 15:04"
		}
		labels[index] = at.Format(format)
		counts[labels[index]]++
	}
	for index, at := range parsed {
		if !at.IsZero() && counts[labels[index]] > 1 {
			labels[index] += " " + at.Format("-07:00")
		}
	}
	return labels
}

func compactBucketLabel(value, group string) string {
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	switch group {
	case "hour":
		return at.Format("15:04")
	case "month":
		return at.Format("Jan 06")
	default:
		return at.Format("Jan 02")
	}
}

func statsBucketMetric(bucket usage.StatsBucket, metric string) float64 {
	switch metric {
	case "cost":
		value, _ := strconv.ParseFloat(bucket.KnownMetricValue, 64)
		return value
	case "sessions":
		return float64(bucket.Sessions)
	default:
		return float64(bucket.Tokens)
	}
}

func compactMetric(value float64, metric string) string {
	switch metric {
	case "cost":
		return "$" + compactDecimal(value)
	case "sessions":
		return groupedInt(int64(value))
	default:
		return compactNumber(value)
	}
}

func compactCost(complete *string, known string, knownAvailable bool) string {
	value := known
	partial := complete == nil
	if complete != nil {
		value = *complete
	}
	if !knownAvailable {
		return "unavailable"
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return "unavailable"
	}
	formatted := "$" + compactDecimal(parsed)
	if partial {
		formatted += " KNOWN"
	}
	return formatted
}

func knownCostAvailable(complete *string, known, coverage string) bool {
	if complete != nil {
		return true
	}
	if value, err := strconv.ParseFloat(known, 64); err == nil && value != 0 {
		return true
	}
	value, err := strconv.ParseFloat(coverage, 64)
	return err == nil && value > 0
}

func compactDecimal(value float64) string {
	absolute := math.Abs(value)
	if absolute >= 1_000_000 {
		return compactNumber(value)
	}
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func compactNumber(value float64) string {
	absolute := math.Abs(value)
	for _, unit := range []struct {
		threshold float64
		suffix    string
	}{{1_000_000_000_000, "T"}, {1_000_000_000, "B"}, {1_000_000, "M"}, {1_000, "K"}} {
		if absolute >= unit.threshold {
			return strconv.FormatFloat(value/unit.threshold, 'f', 1, 64) + unit.suffix
		}
	}
	return groupedInt(int64(math.Round(value)))
}

func groupedInt(value int64) string {
	text := strconv.FormatInt(value, 10)
	sign := ""
	if strings.HasPrefix(text, "-") {
		sign, text = "-", strings.TrimPrefix(text, "-")
	}
	for index := len(text) - 3; index > 0; index -= 3 {
		text = text[:index] + "," + text[index:]
	}
	return sign + text
}

func formatPercent(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64) + "%"
}

func scaledBar(value, maximum float64, width int) int {
	if value <= 0 || maximum <= 0 || width <= 0 {
		return 0
	}
	return min(width, max(1, int(math.Round(value/maximum*float64(width)))))
}

func heatLevel(value, maximum float64) int {
	if value <= 0 || maximum <= 0 {
		return 0
	}
	ratio := value / maximum
	switch {
	case ratio <= 0.25:
		return 1
	case ratio <= 0.5:
		return 2
	case ratio <= 0.75:
		return 3
	default:
		return 4
	}
}

func statsFit(value string, width int) string {
	if statsVisibleWidth(value) <= width {
		return value
	}
	return runewidth.Truncate(stripStatsANSI(value), width, "…")
}

func statsWrap(value string, width int) []string {
	if statsVisibleWidth(value) <= width {
		return []string{value}
	}
	words := strings.Fields(value)
	lines := make([]string, 0, 2)
	line := ""
	for _, word := range words {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if line != "" && statsVisibleWidth(candidate) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		line = candidate
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func statsPad(value string, width int) string {
	value = statsFit(value, width)
	return value + strings.Repeat(" ", max(0, width-statsVisibleWidth(value)))
}

func statsPadLeft(value string, width int) string {
	value = statsFit(value, width)
	return strings.Repeat(" ", max(0, width-statsVisibleWidth(value))) + value
}

func statsVisibleWidth(value string) int {
	return runewidth.StringWidth(stripStatsANSI(value))
}

func stripStatsANSI(value string) string {
	var plain strings.Builder
	for index := 0; index < len(value); {
		if value[index] == 0x1b && index+1 < len(value) && value[index+1] == '[' {
			index += 2
			for index < len(value) {
				character := value[index]
				index++
				if character >= 0x40 && character <= 0x7e {
					break
				}
			}
			continue
		}
		plain.WriteByte(value[index])
		index++
	}
	return plain.String()
}

func statsTitle(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
