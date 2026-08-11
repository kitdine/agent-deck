package main

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"

	"github.com/kitdine/agent-deck/internal/usage"
)

const (
	statsMinWidth     = 48
	statsDefaultWidth = 100
	statsMaxWidth     = 260

	statsModelsCap        = 8
	statsProvidersCap     = 8
	statsUnpricedCap      = 12
	statsModelCacheCap    = 8
	statsCacheSessionsCap = 10
	statsTrendCap         = 48

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
// trendLines and responsive zero-range folding both call this so they can
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
	primitives := newUsageTextPrimitives(w, noColor)
	return usageTextRenderOptions{width: primitives.width, color: primitives.color}
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
	_, err := io.WriteString(w, renderer.renderResponsive())
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

func (r statsTextRenderer) trendLines(width int) []string {
	metricLabel := strings.ToUpper(r.report.Metric)
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
	peakLabel := compactMetric(maximum, r.report.Metric)
	if r.report.Metric == "cost" && !r.hasKnownProviderCost() {
		peakLabel = "unavailable"
	}
	trendColor := usageMetricColor(r.report.Metric)
	if r.report.Metric == "cost" && (r.hasPartialCost() || !r.hasKnownProviderCost()) {
		trendColor = usageColorWarning
	}
	lines := []string{r.sectionTitle("🗓 TREND · "+metricLabel+" · PEAK "+peakLabel, width, trendColor)}
	labelWidth, valueWidth := r.statsTrendLabelValueWidths()
	labelWidth = min(labelWidth, max(statsTrendDefaultLabelWidth, width-valueWidth-12))
	barWidth := min(52, max(statsTrendMinBarWidth, width-labelWidth-valueWidth-4))
	for index := range buckets {
		label := labels[index]
		valueColor := usageMetricColor(r.report.Metric)
		if r.report.Metric == "cost" {
			valueColor = usageCostColor(buckets[index].MetricValue, knownCostAvailable(buckets[index].MetricValue, buckets[index].KnownMetricValue, buckets[index].Coverage))
		}
		value := statsPadLeft(r.style(valueLabels[index], valueColor), valueWidth)
		if maximum <= 0 {
			lines = append(lines, statsPad(label, labelWidth)+"  "+value)
			continue
		}
		filled := scaledBar(values[index], maximum, barWidth)
		bar := r.barTrack(filled, barWidth, trendColor)
		lines = append(lines, statsPad(label, labelWidth)+"  "+bar+"  "+value)
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
	lines := []string{r.sectionTitle(rankingLabel, width, usageColorSession)}
	shownModels := statsTopN(r.report.Models, r.capFor(statsModelsCap))
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
	}
	modelDetails := make([][]usageAlignedColumn, limit)
	for index, model := range shownModels {
		cost := compactCost(model.ProviderCost, model.KnownProviderCost, knownCostAvailable(model.ProviderCost, model.KnownProviderCost, model.Coverage))
		modelDetails[index] = dimensionDetailColumns(compactNumber(float64(model.Tokens)), cost, modelPricingStatus(model), groupedInt(model.Sessions))
	}
	usageAlignColumnRows(modelDetails)
	nameWidth := min(23, max(14, width/2))
	shareWidth := 6
	for _, label := range shareLabels {
		shareWidth = max(shareWidth, statsVisibleWidth(label))
	}
	barWidth := min(36, max(6, width-nameWidth-shareWidth-5))
	for index := 0; index < limit; index++ {
		model := shownModels[index]
		name := statsFit(model.Name, nameWidth)
		identityColor := usageClientColor(model.Client)
		line := statsPad(r.style(name, identityColor), nameWidth)
		if shareLabels[index] != "unavailable" {
			filled := scaledBar(shares[index], 100, barWidth)
			line += " " + r.barTrack(filled, barWidth, identityColor)
		}
		shareColor := usageMetricColor(r.report.Metric)
		if shareLabels[index] == "unavailable" {
			shareColor = usageColorWarning
		}
		lines = append(lines, line+" "+statsPadLeft(r.style(shareLabels[index], shareColor), shareWidth))
		continuation := []string{groupedInt(modelToolCalls(model)) + " tools"}
		if model.CacheHitRate != nil && (model.CachedReadTokens > 0 || model.CacheWriteTokens > 0) {
			continuation = append(continuation, *model.CacheHitRate+"% hit")
		}
		for _, detailLine := range r.dimensionDetailLines(width, modelDetails[index], continuation...) {
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
	lines = append(lines, "", r.sectionTitle(clientLabel, width, usageColorBrand))
	clientDetails := make([][]usageAlignedColumn, len(r.report.Clients))
	for index, client := range r.report.Clients {
		cost := compactCost(client.ProviderCost, client.KnownProviderCost, knownCostAvailable(client.ProviderCost, client.KnownProviderCost, client.Coverage))
		clientDetails[index] = dimensionDetailColumns(compactNumber(float64(client.Tokens)), cost, modelPricingStatus(client), groupedInt(client.Sessions))
	}
	usageAlignColumnRows(clientDetails)
	for index, client := range r.report.Clients {
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
		identityColor := usageClientColor(client.Name)
		line := statsPad(r.style(statsTitle(client.Name), identityColor), nameWidth)
		if shareLabel != "unavailable" {
			filled := scaledBar(share, 100, barWidth)
			line += " " + r.barTrack(filled, barWidth, identityColor)
		}
		shareColor := usageMetricColor(r.report.Metric)
		if shareLabel == "unavailable" {
			shareColor = usageColorWarning
		}
		lines = append(lines, line+" "+statsPadLeft(r.style(shareLabel, shareColor), shareWidth))
		var continuation []string
		if client.CacheHitRate != nil && (client.CachedReadTokens > 0 || client.CacheWriteTokens > 0) {
			continuation = append(continuation, *client.CacheHitRate+"% hit")
		}
		for _, detailLine := range r.dimensionDetailLines(width, clientDetails[index], continuation...) {
			lines = append(lines, r.style(detailLine, "2"))
		}
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
	lines = append(lines, "", r.sectionTitle(providerLabel, width, usageColorInfo))
	shownProviders := statsTopN(r.report.Providers, r.capFor(statsProvidersCap))
	providerDetails := make([][]usageAlignedColumn, len(shownProviders))
	for index, provider := range shownProviders {
		cost := compactCost(provider.ProviderCost, provider.KnownProviderCost, knownCostAvailable(provider.ProviderCost, provider.KnownProviderCost, provider.Coverage))
		providerDetails[index] = dimensionDetailColumns(compactNumber(float64(provider.Tokens)), cost, modelPricingStatus(provider), groupedInt(provider.Sessions))
	}
	usageAlignColumnRows(providerDetails)
	for index, provider := range shownProviders {
		share, _ := strconv.ParseFloat(provider.KnownShare, 64)
		shareLabel := formatPercent(share)
		if r.report.Metric == "cost" && !knownCostAvailable(provider.MetricValue, provider.KnownMetricValue, provider.Coverage) {
			share = 0
			shareLabel = "unavailable"
		}
		nameWidth := min(23, max(14, width/2))
		shareWidth := max(6, statsVisibleWidth(shareLabel))
		barWidth := min(36, max(6, width-nameWidth-shareWidth-3))
		name := statsTitle(provider.Client) + "/" + provider.Name
		identityColor := usageClientColor(provider.Client)
		line := statsPad(r.style(statsFit(name, nameWidth), identityColor), nameWidth)
		if shareLabel != "unavailable" {
			filled := scaledBar(share, 100, barWidth)
			line += " " + r.barTrack(filled, barWidth, identityColor)
		}
		shareColor := usageMetricColor(r.report.Metric)
		if shareLabel == "unavailable" {
			shareColor = usageColorWarning
		}
		lines = append(lines, line+" "+statsPadLeft(r.style(shareLabel, shareColor), shareWidth))
		var continuation []string
		if provider.CacheHitRate != nil && (provider.CachedReadTokens > 0 || provider.CacheWriteTokens > 0) {
			continuation = append(continuation, *provider.CacheHitRate+"% hit")
		}
		// The route is reported metadata, so it only ever appends to a row
		// that already exists, and only when a wrapper actually carried
		// events. A provider that was never selected through one renders
		// exactly as before.
		if provider.WrapperEvents > 0 {
			continuation = append(continuation, groupedInt(provider.WrapperEvents)+" via wrapper")
		}
		for _, detailLine := range r.dimensionDetailLines(width, providerDetails[index], continuation...) {
			lines = append(lines, r.style(detailLine, "2"))
		}
	}
	if len(r.report.Providers) == 0 {
		lines = append(lines, r.style("No providers in this range.", "2"))
	}
	lines = append(lines, r.topNFooterLine(len(r.report.Providers), len(shownProviders), "providers")...)
	cacheLines := r.cachePresentationLines(width)
	if len(cacheLines) > 0 {
		lines = append(lines, "", r.sectionTitle("CACHE HIT RATE", width, usageColorWarning))
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

func (r statsTextRenderer) cachePresentationLines(width int) []string {
	var lines []string
	var cacheModels []usage.StatsDimension
	for _, model := range r.report.Models {
		if model.CacheHitRate == nil || model.CachedReadTokens == 0 && model.CacheWriteTokens == 0 {
			continue
		}
		cacheModels = append(cacheModels, model)
	}
	shownCacheModels := statsTopN(cacheModels, r.capFor(statsModelCacheCap))
	if len(shownCacheModels) > 0 {
		lines = append(lines, r.style("CACHE MODELS", usageColorWarning))
		modelLabels := make([][]string, len(shownCacheModels))
		modelDetails := make([][]usageAlignedColumn, len(shownCacheModels))
		for index, model := range shownCacheModels {
			rate, _ := strconv.ParseFloat(*model.CacheHitRate, 64)
			barWidth := min(12, max(8, width/8))
			label := fmt.Sprintf("MODEL %s/%s %s %s%%", statsTitle(model.Client), model.Name, r.barTrack(scaledBar(rate, 100, barWidth), barWidth, usageColorSuccess), *model.CacheHitRate)
			modelLabels[index] = statsWrap(label, width)
			modelDetails[index] = []usageAlignedColumn{
				{label: "READ", value: compactNumber(float64(model.CachedReadTokens)), width: 6},
				{label: "WRITE", value: compactNumber(float64(model.CacheWriteTokens)), width: 6},
			}
		}
		usageAlignColumnRows(modelDetails)
		for index, model := range shownCacheModels {
			lines = append(lines, modelLabels[index]...)
			continuation := []string(nil)
			if model.LogicalInputTokens > 0 {
				continuation = append(continuation, "LOGICAL INPUT "+compactNumber(float64(model.LogicalInputTokens)))
			}
			for _, detailLine := range r.dimensionDetailLines(width, modelDetails[index], continuation...) {
				lines = append(lines, r.style(detailLine, "2"))
			}
		}
	}
	lines = append(lines, r.topNFooterLine(len(cacheModels), len(shownCacheModels), "cache models")...)

	shownSessions := statsTopN(r.report.CacheSessions, r.capFor(statsCacheSessionsCap))
	if len(shownSessions) > 0 {
		lines = append(lines, "", r.style("CACHE SESSIONS", usageColorWarning))
	}
	for index, session := range shownSessions {
		rate := "0.00"
		if session.CacheHitRate != nil {
			rate = *session.CacheHitRate
		}
		identifier := statsFit(session.SessionID, min(32, max(12, width/3)))
		detail := fmt.Sprintf("SESSION %s/%s [%d] %s%% hit · read %s · write %s · %s", statsTitle(session.Client), identifier, index+1, rate, compactNumber(float64(session.CachedReadTokens)), compactNumber(float64(session.CacheWriteTokens)), strings.Join(session.Models, ","))
		lines = append(lines, statsWrap(detail, width)...)
	}
	if len(shownSessions) > 0 {
		lines = append(lines, "", r.style("DETAIL COMMANDS", "2"))
		for index, session := range shownSessions {
			if session.DetailCommand == "" {
				continue
			}
			for _, commandLine := range statsWrapCommand(fmt.Sprintf("[%d] %s", index+1, session.DetailCommand), width) {
				lines = append(lines, r.style(commandLine, "2"))
			}
		}
	}
	lines = append(lines, r.topNFooterLine(len(r.report.CacheSessions), len(shownSessions), "cache sessions")...)
	return lines
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
		lines = append(lines, statsWrap("range "+statsActivityRange(activity.FirstAt, activity.LastAt), r.width)...)
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

// statsActivityRange localizes the model activity bounds and names the zone
// once for the pair. Both values must parse before either is rewritten, so a
// half-localized range can never claim a zone it does not describe.
func statsActivityRange(firstAt, lastAt string) string {
	_, firstErr := time.Parse(time.RFC3339Nano, firstAt)
	_, lastErr := time.Parse(time.RFC3339Nano, lastAt)
	if firstErr != nil || lastErr != nil {
		return firstAt + " - " + lastAt
	}
	return renderDisplayTime(firstAt) + " - " + renderDisplayTime(lastAt) + " " + displayZoneName()
}

func modelToolCalls(model usage.StatsDimension) int64 {
	if model.Activity == nil {
		return 0
	}
	return model.Activity.ToolCalls
}

func (r statsTextRenderer) activityLines() []string {
	metricLabel := strings.ToUpper(r.report.Metric)
	costUnavailable := false
	if r.report.Metric == "cost" {
		costUnavailable = !r.hasKnownProviderCost()
		if r.hasPartialCost() {
			metricLabel = "KNOWN COST"
		}
	}
	title := "▦ ACTIVITY BY WEEKDAY / HOUR · " + metricLabel
	if r.width >= 58 {
		title += " · 1H BUCKET"
	}
	lines := []string{r.sectionTitle(title, r.width, usageColorBrand)}
	if costUnavailable {
		for _, line := range statsWrap("unavailable: no priced events; heatmap retained with zero-value buckets", r.width) {
			lines = append(lines, r.style(line, usageColorWarning))
		}
	}

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
	primitives := r.textPrimitives()
	for weekday := 0; weekday < 7; weekday++ {
		line := weekdays[weekday] + "  "
		for hour := 0; hour < 24; hour++ {
			line += primitives.heatmapCell(heatLevel(values[weekday*24+hour], maximum)) + cellSeparator
		}
		lines = append(lines, strings.TrimRight(line, " "))
	}
	legend := "1H BUCKET · LESS  " + primitives.heatmapCell(0) + " " + primitives.heatmapCell(1) + " " + primitives.heatmapCell(2) + " " + primitives.heatmapCell(3) + " " + primitives.heatmapCell(4) + "  MORE"
	from, to := compactStatsDisplayRange(r.report.Range)
	rangeText := from + " - " + to
	gap := r.width - statsVisibleWidth(legend) - statsVisibleWidth(rangeText)
	if gap >= 1 {
		lines = append(lines, legend+strings.Repeat(" ", gap)+rangeText)
	} else {
		lines = append(lines, legend, statsPadLeft(rangeText, r.width))
	}
	return lines
}

func (r statsTextRenderer) sectionTitle(label string, width int, color string) string {
	return r.textPrimitives().sectionTitle(label, width, color)
}

func (r statsTextRenderer) textPrimitives() usageTextPrimitives {
	return usageTextPrimitives{width: r.width, color: r.color}
}

func (r statsTextRenderer) barTrack(filled, width int, color string) string {
	return r.textPrimitives().barTrack(filled, width, color)
}

func dimensionDetailColumns(tokens, cost, status, sessions string) []usageAlignedColumn {
	return []usageAlignedColumn{
		usageAlignedColumn{label: "TOKENS", value: tokens, width: 10},
		usageAlignedColumn{label: "COST", value: cost, width: 12},
		usageAlignedColumn{label: "STATUS", value: status, width: 9},
		usageAlignedColumn{label: "SESSIONS", value: sessions, width: 6},
	}
}

func (r statsTextRenderer) dimensionDetailLines(width int, columns []usageAlignedColumn, continuation ...string) []string {
	lines := usageAlignedColumns(width, columns...)
	for _, value := range continuation {
		for _, line := range statsWrap("↳ "+value, width) {
			lines = append(lines, line)
		}
	}
	return lines
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
	return r.textPrimitives().style(value, code)
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

// statsWrapCommand keeps every terminal line within width while preserving a
// command copyable by a shell: a long token continues with a backslash-newline.
func statsWrapCommand(value string, width int) []string {
	if width <= 2 {
		return statsWrap(value, width)
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(value) {
		if statsVisibleWidth(word) <= width {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if line != "" && statsVisibleWidth(candidate) > width {
				lines = append(lines, line)
				line = word
			} else {
				line = candidate
			}
			continue
		}

		for statsVisibleWidth(line) > width-2 {
			prefix := runewidth.Truncate(line, width-2, "")
			lines = append(lines, prefix+" \\")
			line = strings.TrimPrefix(line, prefix)
		}
		if line != "" {
			lines = append(lines, line+" \\")
			line = ""
		}
		remaining := stripStatsANSI(word)
		for statsVisibleWidth(remaining) > width-1 {
			prefix := runewidth.Truncate(remaining, width-1, "")
			lines = append(lines, prefix+"\\")
			remaining = strings.TrimPrefix(remaining, prefix)
		}
		line = remaining
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

// statsTitle upper-cases the first character of a known client-style label. It
// decodes a full rune rather than a byte, so a non-ASCII label keeps its first
// code point intact instead of being split into invalid UTF-8.
func statsTitle(value string) string {
	first, size := utf8.DecodeRuneInString(value)
	if size == 0 || first == utf8.RuneError && size == 1 {
		return value
	}
	return string(unicode.ToUpper(first)) + value[size:]
}
