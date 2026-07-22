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

	statsModelsCap        = 8
	statsProvidersCap     = 8
	statsUnpricedCap      = 12
	statsModelCacheCap    = 8
	statsCacheSessionsCap = 10
	statsTrendCap         = 48

	// statsRankingMinWidth matches detail-compaction's single-line guarantee:
	// statsCompactDetail keeps a model/provider detail on one line for
	// realistic field values once its column is at least this wide. The
	// two-column layout must never hand rankingLines less than this, or the
	// "single line at width >= 80" contract silently breaks for wide
	// terminals even though the terminal itself is far past 80.
	statsRankingMinWidth = 80

	// statsTrendDefaultLabelWidth and statsTrendDefaultValueWidth are
	// trendLines' own starting column widths before it widens either to fit
	// the report's actual labels/values. statsTrendLabelValueWidths reuses
	// these same starting points so the two never disagree about what a
	// "default" bucket row needs.
	statsTrendDefaultLabelWidth = 7
	statsTrendDefaultValueWidth = 9

	// statsTrendMinBarWidth is trendLines' own floor on bar width
	// (`max(8, ...)`); a column narrower than label+2+8+2+value cannot show a
	// bucket row without truncating something even before the bar shrinks
	// further.
	statsTrendMinBarWidth = 8
)

// statsTrendLabelValueWidths computes the widest bucket label and value width
// trendLines would need to render every (cap-windowed) bucket in this report
// without truncating either, using the report's actual labels and values —
// not a static per-format guess. A prior fix used a fixed 7/9-column
// assumption and was reopened because compact formats can be wider than that
// (a known-but-partial cost value like "$13.4M KNOWN" is 12 columns, and so
// is a DST-disambiguated hour label like "15:04 +08:00"); computing the real
// widths from this report's own data, the same way trendLines itself will,
// closes that class of mismatch instead of chasing one more format.
// trendLines and the two-column layout decision both call this so they can
// never disagree about what trend actually needs.
func (r statsTextRenderer) statsTrendLabelValueWidths() (labelWidth, valueWidth int) {
	total := len(r.report.Buckets)
	buckets := r.report.Buckets
	if total > statsTrendCap {
		buckets = r.report.Buckets[total-statsTrendCap:]
	}
	labelWidth = statsTrendDefaultLabelWidth
	for _, label := range compactBucketLabels(buckets, r.report.GroupBy) {
		labelWidth = max(labelWidth, statsVisibleWidth(label))
	}
	valueWidth = statsTrendDefaultValueWidth
	for _, bucket := range buckets {
		valueLabel := compactMetric(statsBucketMetric(bucket, r.report.Metric), r.report.Metric)
		if r.report.Metric == "cost" {
			valueLabel = compactCost(bucket.MetricValue, bucket.KnownMetricValue, knownCostAvailable(bucket.MetricValue, bucket.KnownMetricValue, bucket.Coverage))
		}
		valueWidth = max(valueWidth, statsVisibleWidth(valueLabel))
	}
	return labelWidth, valueWidth
}

// statsTwoColumnFits reports whether the terminal is wide enough for both the
// ranking column's single-line detail guarantee and this report's actual
// trend label/value width, with the 4-column gap joinStatsColumns uses
// between them. render falls back to the stacked single-column layout when
// this is false, rather than splitting into a two-column layout that would
// have to truncate one side below what it needs to stay meaningful.
func (r statsTextRenderer) statsTwoColumnFits() bool {
	labelWidth, valueWidth := r.statsTrendLabelValueWidths()
	trendMinWidth := labelWidth + 2 + statsTrendMinBarWidth + 2 + valueWidth
	return r.width-4 >= statsRankingMinWidth+trendMinWidth
}

// statsTwoColumnWidths splits the two-column layout's available inner width
// (terminal width minus the 4-column gap used by joinStatsColumns) between
// the trend column and the ranking column. The ranking column is floored at
// statsRankingMinWidth so MODELS/PROVIDERS detail keeps its single-line
// guarantee regardless of how the split ratio would otherwise divide a wide
// terminal; trend takes whatever remains. Callers must check
// statsTwoColumnFits first — this function assumes there is enough room for
// both minimums and does not itself protect trend from being squeezed below
// statsTrendMinWidth.
func statsTwoColumnWidths(width int) (leftWidth, rightWidth int) {
	inner := width - 4
	rightWidth = max(statsRankingMinWidth, inner*2/5)
	leftWidth = inner - rightWidth
	return leftWidth, rightWidth
}

// statsTopN returns the leading limit items of items, in their existing
// order, or items unchanged if limit is non-positive or not exceeded. Callers
// pair it with topNFooterLine so the text output stays a fixed size while
// --format json keeps every row.
func statsTopN[T any](items []T, limit int) []T {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

// topNFooterLine reports how many rows were left out of a capped text list,
// or nil if none were.
func (r statsTextRenderer) topNFooterLine(total, shown int, label string) []string {
	if omitted := total - shown; omitted > 0 {
		return []string{r.style(fmt.Sprintf("+%d more %s in JSON", omitted, label), "2")}
	}
	return nil
}

// statsCompactDetail appends secondary fields to base, in priority order,
// joined by " · ", keeping only as many as still let the line fit within
// width. It stops at the first secondary that would not fit, rather than
// skipping ahead to a later one, so omission is predictable. High-value
// fields belong in base and are never dropped; the full field set is always
// present in JSON regardless of what this trims from the text line.
func statsCompactDetail(base string, width int, secondaries ...string) string {
	detail := base
	for _, secondary := range secondaries {
		candidate := detail + " · " + secondary
		if statsVisibleWidth(candidate) > width {
			break
		}
		detail = candidate
	}
	return detail
}

type usageTextRenderOptions struct {
	width int
	color bool
	// top overrides the shared-topn text caps (MODELS, PROVIDERS, UNPRICED
	// MODELS, per-model CACHE, cache sessions) when non-nil: 0 shows every row
	// (matching JSON), a positive value uses that as the cap for all of them.
	// nil (the default) keeps each section's own default cap. TREND and
	// CLIENTS are never affected.
	top *int
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
	renderer := statsTextRenderer{report: report, width: options.width, color: options.color, top: options.top}
	_, err := io.WriteString(w, renderer.render())
	return err
}

type statsTextRenderer struct {
	report usage.StatsReport
	width  int
	color  bool
	top    *int
}

// capFor resolves a shared-topn section's effective cap: the section's own
// default unless --top was explicitly given, in which case --top's value
// wins outright (0 falls through to statsTopN's own "limit <= 0 means no
// cap" rule, so explicit --top 0 naturally restores the full list).
func (r statsTextRenderer) capFor(defaultCap int) int {
	if r.top == nil {
		return defaultCap
	}
	return *r.top
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
	if r.statsTwoColumnFits() {
		leftWidth, rightWidth := statsTwoColumnWidths(r.width)
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
	total := len(r.report.Buckets)
	omitted := 0
	buckets := r.report.Buckets
	if total > statsTrendCap {
		omitted = total - statsTrendCap
		buckets = r.report.Buckets[omitted:]
	}
	labels := compactBucketLabels(buckets, r.report.GroupBy)
	maximum := float64(0)
	values := make([]float64, len(buckets))
	valueLabels := make([]string, len(buckets))
	for index, bucket := range buckets {
		values[index] = statsBucketMetric(bucket, r.report.Metric)
		valueLabels[index] = compactMetric(values[index], r.report.Metric)
		if r.report.Metric == "cost" {
			valueLabels[index] = compactCost(bucket.MetricValue, bucket.KnownMetricValue, knownCostAvailable(bucket.MetricValue, bucket.KnownMetricValue, bucket.Coverage))
		}
		maximum = math.Max(maximum, values[index])
	}
	labelWidth, valueWidth := r.statsTrendLabelValueWidths()
	labelWidth = min(labelWidth, max(statsTrendDefaultLabelWidth, width-valueWidth-12))
	barWidth := min(52, max(statsTrendMinBarWidth, width-labelWidth-valueWidth-4))
	for index := range buckets {
		label := labels[index]
		filled := scaledBar(values[index], maximum, barWidth)
		bar := r.style(strings.Repeat("█", filled), "34") + strings.Repeat("░", barWidth-filled)
		lines = append(lines, statsPad(label, labelWidth)+"  "+bar+"  "+statsPadLeft(valueLabels[index], valueWidth))
	}
	if total == 0 {
		lines = append(lines, r.style("No activity in this range.", "2"))
	}
	if omitted > 0 {
		lines = append(lines, r.style(fmt.Sprintf("+%d earlier buckets in JSON", omitted), "2"))
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
	shownModels := statsTopN(r.report.Models, r.capFor(statsModelsCap))
	maximum := float64(0)
	limit := len(shownModels)
	shares := make([]float64, limit)
	shareLabels := make([]string, limit)
	for index := 0; index < limit; index++ {
		model := shownModels[index]
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
		model := shownModels[index]
		name := statsFit(model.Name, nameWidth)
		filled := scaledBar(shares[index], maximum, barWidth)
		bar := r.style(strings.Repeat("█", filled), "35") + strings.Repeat("░", barWidth-filled)
		lines = append(lines, statsPad(name, nameWidth)+" "+bar+" "+statsPadLeft(shareLabels[index], shareWidth))
		cost := compactCost(model.ProviderCost, model.KnownProviderCost, knownCostAvailable(model.ProviderCost, model.KnownProviderCost, model.Coverage))
		detail := fmt.Sprintf("%s tokens · %s · %s · %s · %s sessions", compactNumber(float64(model.Tokens)), shareLabels[index], cost, modelPricingStatus(model), groupedInt(model.Sessions))
		secondaries := []string{groupedInt(modelToolCalls(model)) + " tools"}
		if model.CacheHitRate != nil && (model.CachedReadTokens > 0 || model.CacheWriteTokens > 0) {
			secondaries = append(secondaries, *model.CacheHitRate+"% hit")
		}
		detail = statsCompactDetail(detail, width, secondaries...)
		for _, detailLine := range statsWrap(detail, width) {
			lines = append(lines, r.style(detailLine, "2"))
		}
	}
	if limit == 0 {
		lines = append(lines, r.style("No models in this range.", "2"))
	}
	lines = append(lines, r.topNFooterLine(len(r.report.Models), limit, "models")...)
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
	providerLabel := "PROVIDERS"
	if r.report.Metric == "cost" {
		switch {
		case r.hasPartialCost():
			providerLabel += " · KNOWN COST"
		case !r.hasKnownProviderCost():
			providerLabel += " · COST UNAVAILABLE"
		}
	}
	lines = append(lines, "", r.sectionTitle(providerLabel, width, "1;34"))
	shownProviders := statsTopN(r.report.Providers, r.capFor(statsProvidersCap))
	for _, provider := range shownProviders {
		share, _ := strconv.ParseFloat(provider.KnownShare, 64)
		shareLabel := formatPercent(share)
		if r.report.Metric == "cost" && !knownCostAvailable(provider.MetricValue, provider.KnownMetricValue, provider.Coverage) {
			share = 0
			shareLabel = "unavailable"
		}
		nameWidth := min(23, max(14, width/2))
		shareWidth := max(6, statsVisibleWidth(shareLabel))
		barWidth := min(36, max(6, width-nameWidth-shareWidth-3))
		filled := scaledBar(share, 100, barWidth)
		bar := r.style(strings.Repeat("█", filled), "34") + strings.Repeat("░", barWidth-filled)
		name := statsTitle(provider.Client) + "/" + provider.Name
		lines = append(lines, statsPad(statsFit(name, nameWidth), nameWidth)+" "+bar+" "+statsPadLeft(shareLabel, shareWidth))
		cost := compactCost(provider.ProviderCost, provider.KnownProviderCost, knownCostAvailable(provider.ProviderCost, provider.KnownProviderCost, provider.Coverage))
		detail := fmt.Sprintf("%s tokens · %s · %s · %s · %s sessions", compactNumber(float64(provider.Tokens)), shareLabel, cost, modelPricingStatus(provider), groupedInt(provider.Sessions))
		var secondaries []string
		if provider.CacheHitRate != nil && (provider.CachedReadTokens > 0 || provider.CacheWriteTokens > 0) {
			secondaries = append(secondaries, *provider.CacheHitRate+"% hit")
		}
		detail = statsCompactDetail(detail, width, secondaries...)
		for _, detailLine := range statsWrap(detail, width) {
			lines = append(lines, r.style(detailLine, "2"))
		}
	}
	if len(r.report.Providers) == 0 {
		lines = append(lines, r.style("No providers in this range.", "2"))
	}
	lines = append(lines, r.topNFooterLine(len(r.report.Providers), len(shownProviders), "providers")...)
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
	var cacheModels []usage.StatsDimension
	for _, model := range r.report.Models {
		if model.CacheHitRate == nil || model.CachedReadTokens == 0 && model.CacheWriteTokens == 0 {
			continue
		}
		cacheModels = append(cacheModels, model)
	}
	shownCacheModels := statsTopN(cacheModels, r.capFor(statsModelCacheCap))
	for _, model := range shownCacheModels {
		detail := fmt.Sprintf("MODEL %s/%s  %s%% hit · read %s · write %s", statsTitle(model.Client), model.Name, *model.CacheHitRate, compactNumber(float64(model.CachedReadTokens)), compactNumber(float64(model.CacheWriteTokens)))
		if model.Client == "claude" {
			detail += " · logical input " + compactNumber(float64(model.LogicalInputTokens))
		}
		lines = append(lines, statsWrap(detail, width)...)
	}
	lines = append(lines, r.topNFooterLine(len(cacheModels), len(shownCacheModels), "cache models")...)
	shownSessions := statsTopN(r.report.CacheSessions, r.capFor(statsCacheSessionsCap))
	for _, session := range shownSessions {
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
	lines = append(lines, r.topNFooterLine(len(r.report.CacheSessions), len(shownSessions), "cache sessions")...)
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
	shown := statsTopN(r.report.UnpricedModels, r.capFor(statsUnpricedCap))
	for _, model := range shown {
		entry := fmt.Sprintf("%s/%s · %s", statsTitle(model.Client), model.Model, strings.Join(model.Components, ", "))
		lines = append(lines, statsWrap(entry, r.width)...)
	}
	lines = append(lines, r.topNFooterLine(len(r.report.UnpricedModels), len(shown), "unpriced models")...)
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
