package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kitdine/agent-deck/internal/usage"
)

const statsResponsiveColumnGap = 4

type statsResponsiveLayout struct {
	columns  [][]string
	widths   []int
	tallest  int
	shortest int
}

func (r statsTextRenderer) renderResponsive() string {
	var out strings.Builder
	out.WriteString(r.style("📊 USAGE STATS · "+r.rangeLabel(), "1;32"))
	out.WriteByte('\n')
	out.WriteString(r.metaLine())
	out.WriteString("\n\n")
	writeStatsLines(&out, r.responsiveKPILines())
	out.WriteByte('\n')

	layout := r.responsiveLayout()
	writeStatsLines(&out, joinStatsColumns(layout.columns, layout.widths, statsResponsiveColumnGap))

	if r.report.ShowModelActivity && len(r.report.Models) == 1 {
		out.WriteByte('\n')
		writeStatsLines(&out, r.modelActivityLines(r.report.Models[0]))
	}
	if len(r.report.Activity) > 0 {
		out.WriteByte('\n')
		writeStatsLines(&out, r.activityLines())
	}
	if detail := r.responsiveDetailCommandLines(r.width); len(detail) > 0 {
		out.WriteByte('\n')
		writeStatsLines(&out, detail)
	}
	if len(r.report.Warnings) > 0 {
		out.WriteByte('\n')
		out.WriteString(r.sectionTitle("⚠ WARNINGS", r.width, "1;33"))
		out.WriteByte('\n')
		for _, warning := range r.report.Warnings {
			writeStatsLines(&out, statsWrap("! "+warning, r.width))
		}
	}
	return out.String()
}

func writeStatsLines(out *strings.Builder, lines []string) {
	for _, line := range lines {
		out.WriteString(line)
		out.WriteByte('\n')
	}
}

func (r statsTextRenderer) responsiveKPILines() []string {
	average := compactCost(r.report.Totals.AverageCost, r.report.Totals.KnownAverageCost, r.hasKnownProviderCost())
	peakValue, _ := strconv.ParseFloat(r.report.Peak.KnownValue, 64)
	peak := compactMetric(peakValue, r.report.Metric)
	if r.report.Metric == "cost" {
		peak = compactCost(r.report.Peak.Value, r.report.Peak.KnownValue, knownCostAvailable(r.report.Peak.Value, r.report.Peak.KnownValue, r.report.Peak.Coverage))
	}
	values := []struct{ label, value string }{
		{label: "TOKENS", value: compactNumber(float64(r.report.Totals.Tokens))},
		{label: "COST", value: compactCost(r.report.Totals.ProviderCost, r.report.Totals.KnownProviderCost, r.hasKnownProviderCost())},
		{label: "SESSIONS", value: groupedInt(r.report.Totals.Sessions)},
		{label: "AVG COST / SESSION", value: average},
		{label: "PEAK " + strings.ToUpper(r.report.Metric), value: peak},
		{label: "PRICED EVENTS", value: r.report.Coverage.Percent + "%"},
	}
	columns := 2
	if r.width >= 180 {
		columns = 6
	} else if r.width >= 120 {
		columns = 3
	}
	widths := splitStatsWidths(r.width-(columns+1), columns, 0)
	border := func(left, middle, right string) string {
		var line strings.Builder
		line.WriteString(left)
		for index, width := range widths {
			if index > 0 {
				line.WriteString(middle)
			}
			line.WriteString(strings.Repeat("─", width))
		}
		line.WriteString(right)
		return line.String()
	}
	lines := []string{border("┌", "┬", "┐")}
	for row := 0; row < len(values); row += columns {
		labels, numbers := "│", "│"
		for column := 0; column < columns; column++ {
			value := values[row+column]
			labels += " " + statsPad(value.label, widths[column]-2) + " │"
			numbers += " " + statsPad(r.style(value.value, "1;37"), widths[column]-2) + " │"
		}
		lines = append(lines, labels, numbers)
		if row+columns < len(values) {
			lines = append(lines, border("├", "┼", "┤"))
		}
	}
	return append(lines, border("└", "┴", "┘"))
}

func (r statsTextRenderer) responsiveLayout() statsResponsiveLayout {
	maximum := statsMaximumColumns(r.width)
	for count := maximum; count >= 2; count-- {
		candidate := r.responsiveLayoutFor(count)
		previous := r.responsiveLayoutFor(count - 1)
		balanced := candidate.shortest*100 >= candidate.tallest*60
		useful := candidate.tallest*100 <= previous.tallest*85
		if balanced && useful {
			return candidate
		}
	}
	return r.responsiveLayoutFor(1)
}

func statsMaximumColumns(width int) int {
	switch {
	case width >= 240:
		return 4
	case width >= 180:
		return 3
	case width >= 120:
		return 2
	default:
		return 1
	}
}

func (r statsTextRenderer) responsiveLayoutFor(count int) statsResponsiveLayout {
	widths := splitStatsWidths(r.width, count, statsResponsiveColumnGap)
	panelMaps := make([]map[string][]string, count)
	for index := range panelMaps {
		panelMaps[index] = r.responsivePanelMap(widths[index])
	}
	assignment := balancedStatsPanelAssignment(panelMaps, count)
	columns := make([][]string, count)
	for panelIndex, key := range statsPanelOrder() {
		column := assignment[panelIndex]
		panel := trimStatsBlankLines(panelMaps[column][key])
		if len(panel) == 0 {
			continue
		}
		if len(columns[column]) > 0 {
			columns[column] = append(columns[column], "")
		}
		columns[column] = append(columns[column], panel...)
	}
	shortest, tallest := statsColumnHeightRange(columns)
	return statsResponsiveLayout{columns: columns, widths: widths, tallest: tallest, shortest: shortest}
}

func statsPanelOrder() []string {
	return []string{"trend", "models", "clients", "providers", "cache", "coverage"}
}

func balancedStatsPanelAssignment(panelMaps []map[string][]string, columns int) []int {
	keys := statsPanelOrder()
	preferred := make(map[string]int, len(keys))
	for column, group := range statsPanelGroups(columns) {
		for _, key := range group {
			preferred[key] = column
		}
	}
	assignment := make([]int, len(keys))
	best := make([]int, len(keys))
	bestTallest, bestSpread, bestPenalty := int(^uint(0)>>1), int(^uint(0)>>1), int(^uint(0)>>1)
	var visit func(int)
	visit = func(index int) {
		if index == len(keys) {
			used := make([]bool, columns)
			heights := make([]int, columns)
			panelCounts := make([]int, columns)
			penalty := 0
			for panelIndex, column := range assignment {
				used[column] = true
				heights[column] += len(trimStatsBlankLines(panelMaps[column][keys[panelIndex]]))
				panelCounts[column]++
				if preferred[keys[panelIndex]] != column {
					penalty++
				}
			}
			for column := range heights {
				if !used[column] {
					return
				}
				heights[column] += max(0, panelCounts[column]-1)
			}
			shortest, tallest := heights[0], heights[0]
			for _, height := range heights[1:] {
				shortest = min(shortest, height)
				tallest = max(tallest, height)
			}
			spread := tallest - shortest
			if tallest < bestTallest || tallest == bestTallest && (spread < bestSpread || spread == bestSpread && penalty < bestPenalty) {
				copy(best, assignment)
				bestTallest, bestSpread, bestPenalty = tallest, spread, penalty
			}
			return
		}
		start := 0
		end := columns
		if index == 0 {
			// TREND anchors the left edge; this also removes equivalent column
			// permutations from the small exhaustive search.
			end = 1
		}
		for column := start; column < end; column++ {
			assignment[index] = column
			visit(index + 1)
		}
	}
	visit(0)
	return best
}

func statsColumnHeightRange(columns [][]string) (shortest, tallest int) {
	if len(columns) == 0 {
		return 0, 0
	}
	shortest, tallest = len(columns[0]), len(columns[0])
	for _, column := range columns[1:] {
		shortest = min(shortest, len(column))
		tallest = max(tallest, len(column))
	}
	return shortest, tallest
}

func statsPanelGroups(columns int) [][]string {
	switch columns {
	case 4:
		return [][]string{{"trend"}, {"models"}, {"clients", "providers"}, {"cache", "coverage"}}
	case 3:
		return [][]string{{"trend", "coverage"}, {"models", "clients"}, {"providers", "cache"}}
	case 2:
		return [][]string{{"trend", "clients", "coverage"}, {"models", "providers", "cache"}}
	default:
		return [][]string{{"trend", "models", "clients", "providers", "cache", "coverage"}}
	}
}

func splitStatsWidths(total, columns, gap int) []int {
	inner := max(columns, total-gap*(columns-1))
	base := inner / columns
	widths := make([]int, columns)
	for index := range widths {
		widths[index] = base
	}
	widths[len(widths)-1] += inner - base*columns
	return widths
}

func joinStatsColumns(columns [][]string, widths []int, gap int) []string {
	height := 0
	for _, column := range columns {
		height = max(height, len(column))
	}
	lines := make([]string, 0, height)
	for row := 0; row < height; row++ {
		var line strings.Builder
		for column := range columns {
			if column > 0 {
				line.WriteString(strings.Repeat(" ", gap))
			}
			value := ""
			if row < len(columns[column]) {
				value = columns[column][row]
			}
			line.WriteString(statsPad(value, widths[column]))
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	return lines
}

func (r statsTextRenderer) responsivePanelMap(width int) map[string][]string {
	panels := splitStatsRankingPanels(r.rankingLines(width))
	panels["trend"] = r.responsiveTrendLines(width)
	panels["coverage"] = r.responsiveCoverageLines(width)
	if len(panels["cache"]) == 0 {
		panels["cache"] = []string{r.sectionTitle("CACHE HIT RATE", width, "1;33"), r.style("No cache data in this range.", "2")}
	}
	return panels
}

func splitStatsRankingPanels(lines []string) map[string][]string {
	panels := map[string][]string{}
	key := "models"
	for _, line := range lines {
		if next := statsRankingPanelKey(line); next != "" {
			key = next
		}
		if key == "cache" && strings.HasPrefix(strings.TrimSpace(stripStatsANSI(line)), "DETAIL COMMANDS") {
			break
		}
		panels[key] = append(panels[key], line)
	}
	for key, panel := range panels {
		panels[key] = trimStatsBlankLines(panel)
	}
	return panels
}

func statsRankingPanelKey(line string) string {
	plain := strings.TrimSpace(stripStatsANSI(line))
	switch {
	case strings.HasPrefix(plain, "🤖 MODELS"):
		return "models"
	case strings.HasPrefix(plain, "CLIENTS"):
		return "clients"
	case strings.HasPrefix(plain, "PROVIDERS"):
		return "providers"
	case strings.HasPrefix(plain, "CACHE HIT RATE"):
		return "cache"
	default:
		return ""
	}
}

func trimStatsBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(stripStatsANSI(lines[0])) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(stripStatsANSI(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (r statsTextRenderer) responsiveCoverageLines(width int) []string {
	lines := []string{r.sectionTitle("COVERAGE", width, "1;33")}
	priced := fmt.Sprintf("PRICED %s%% · %s/%s events", r.report.Coverage.Percent, groupedInt(r.report.Coverage.PricedEvents), groupedInt(r.report.Coverage.TotalEvents))
	lines = append(lines, statsWrap(priced, width)...)
	lines = append(lines, statsWrap("UNPRICED "+groupedInt(r.report.Coverage.UnpricedEvents)+" events", width)...)
	shown := statsTopN(r.report.UnpricedModels, r.capFor(statsUnpricedCap))
	if len(shown) == 0 {
		return append(lines, r.style("No unpriced models in this range.", "2"))
	}
	lines = append(lines, "", r.style("UNPRICED MODELS", "1;33"))
	for _, model := range shown {
		entry := fmt.Sprintf("%s/%s · %s", statsTitle(model.Client), model.Model, strings.Join(model.Components, ", "))
		lines = append(lines, statsWrap(entry, width)...)
	}
	return append(lines, r.topNFooterLine(len(r.report.UnpricedModels), len(shown), "unpriced models")...)
}

func (r statsTextRenderer) responsiveDetailCommandLines(width int) []string {
	shown := statsTopN(r.report.CacheSessions, r.capFor(statsCacheSessionsCap))
	lines := []string{}
	for index, item := range shown {
		if strings.TrimSpace(item.DetailCommand) == "" {
			continue
		}
		if len(lines) == 0 {
			lines = append(lines, r.sectionTitle("DETAIL COMMANDS", width, "1;33"))
		}
		lines = append(lines, statsWrapCommand(fmt.Sprintf("[%d] %s", index+1, item.DetailCommand), width)...)
	}
	if len(lines) > 0 {
		lines = append(lines, r.topNFooterLine(len(r.report.CacheSessions), len(shown), "cache sessions")...)
	}
	return lines
}

func (r statsTextRenderer) responsiveTrendLines(width int) []string {
	lines := r.trendLines(width)
	buckets := r.report.Buckets
	if len(buckets) > statsTrendCap {
		buckets = buckets[len(buckets)-statsTrendCap:]
	}
	if len(buckets) < 3 || len(lines) < len(buckets)+1 {
		return lines
	}
	labels := compactBucketLabels(buckets, r.report.GroupBy)
	rows := lines[1 : len(buckets)+1]
	result := append([]string{}, lines[0])
	for start := 0; start < len(buckets); {
		end := start
		for end < len(buckets) && statsTrendBucketIsZero(buckets[end], r.report.Metric) {
			end++
		}
		if end-start >= 3 {
			result = append(result, r.foldedTrendRow(width, labels[start], labels[end-1], buckets[start]))
			start = end
			continue
		}
		result = append(result, rows[start])
		start++
	}
	return append(result, lines[len(buckets)+1:]...)
}

func statsTrendBucketIsZero(bucket usage.StatsBucket, metric string) bool {
	if metric == "cost" && !knownCostAvailable(bucket.MetricValue, bucket.KnownMetricValue, bucket.Coverage) {
		return false
	}
	return statsBucketMetric(bucket, metric) == 0
}

func (r statsTextRenderer) foldedTrendRow(width int, first, last string, bucket usage.StatsBucket) string {
	value := compactMetric(0, r.report.Metric)
	if r.report.Metric == "cost" {
		value = compactCost(bucket.MetricValue, bucket.KnownMetricValue, true)
	}
	labelWidth, valueWidth := r.statsTrendLabelValueWidths()
	labelWidth = min(max(labelWidth, min(statsVisibleWidth(first+"…"+last), 18)), max(statsTrendDefaultLabelWidth, width-valueWidth-12))
	barWidth := min(52, max(statsTrendMinBarWidth, width-labelWidth-valueWidth-4))
	label := statsFit(first+"…"+last, labelWidth)
	return statsPad(label, labelWidth) + " " + r.barTrack(0, barWidth, "34") + " " + statsPadLeft(value, valueWidth)
}
