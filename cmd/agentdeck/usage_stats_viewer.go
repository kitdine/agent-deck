package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/usage"
	"golang.org/x/term"
)

const usageViewerPageLimit = 20

type usageViewerSection int

const (
	usageViewerOverview usageViewerSection = iota
	usageViewerTrend
	usageViewerActivity
	usageViewerModels
	usageViewerClients
	usageViewerProviders
	usageViewerCache
	usageViewerCoverage
	usageViewerSectionCount
)

var usageViewerSections = [usageViewerSectionCount]string{"OVERVIEW", "TREND", "ACTIVITY", "MODELS", "CLIENTS", "PROVIDERS", "CACHE", "COVERAGE"}

type usageViewerRow struct {
	identity   string
	label      string
	value      string
	detail     terminalDetailModel
	bar        float64
	showBar    bool
	labelColor string
	valueColor string
	barColor   string
	heatmap    []int
}

// usageViewerState is terminal-independent, keeping navigation and selection
// invariants testable without a pseudo-terminal.
type usageViewerState struct {
	report    usage.StatsReport
	top       *int
	warnings  []string
	section   usageViewerSection
	pages     [usageViewerSectionCount]int
	selected  [usageViewerSectionCount]int
	viewports [usageViewerSectionCount]int
	rows      []usageViewerRow
	help      bool
}

func newUsageViewerState(report usage.StatsReport, warnings []string, top *int) *usageViewerState {
	s := &usageViewerState{report: report, warnings: append([]string(nil), warnings...), top: top}
	for i := range s.pages {
		s.pages[i] = 1
	}
	s.refresh()
	return s
}

func (s *usageViewerState) refresh() {
	rows := s.sectionRowsCapped()
	page := s.pages[s.section]
	start := (page - 1) * usageViewerPageLimit
	if start >= len(rows) && page > 1 {
		page = max(1, (len(rows)+usageViewerPageLimit-1)/usageViewerPageLimit)
		s.pages[s.section] = page
		start = (page - 1) * usageViewerPageLimit
	}
	start = min(start, len(rows))
	end := min(len(rows), start+usageViewerPageLimit)
	s.rows = rows[start:end]
	if s.selected[s.section] >= len(s.rows) {
		s.selected[s.section] = max(0, len(s.rows)-1)
	}
	if len(s.rows) == 0 {
		s.selected[s.section], s.viewports[s.section] = 0, 0
	}
}

// viewportOffset returns the section's retained viewport offset, clamped to the
// current page and the supplied visible height, and shifted by the minimum
// amount required to keep the selected row visible. Selection movement inside
// an already visible window therefore leaves the window where the user left it,
// and a shrink/recover resize returns to the same context.
func (s *usageViewerState) viewportOffset(height int) int {
	height = max(1, height)
	offset := min(max(0, s.viewports[s.section]), max(0, len(s.rows)-height))
	selected := s.selected[s.section]
	if selected < offset {
		offset = selected
	} else if selected >= offset+height {
		offset = selected - height + 1
	}
	offset = max(0, offset)
	s.viewports[s.section] = offset
	return offset
}

func (s *usageViewerState) apply(key string) (reload, exit bool) {
	current := int(s.section)
	switch key {
	case "q", "escape":
		return false, true
	case "?":
		s.help = !s.help
	case "left", "shift-tab":
		s.section = usageViewerSection((current + len(usageViewerSections) - 1) % len(usageViewerSections))
		return true, false
	case "right", "tab":
		s.section = usageViewerSection((current + 1) % len(usageViewerSections))
		return true, false
	case "up":
		s.selected[current] = max(0, s.selected[current]-1)
	case "down":
		s.selected[current] = min(max(0, len(s.rows)-1), s.selected[current]+1)
	case "home":
		s.selected[current] = 0
	case "end":
		s.selected[current] = max(0, len(s.rows)-1)
	case "page-up":
		if s.pages[current] > 1 {
			s.pages[current]--
			s.selected[current], s.viewports[current] = 0, 0
			return true, false
		}
	case "page-down":
		if s.pages[current]*usageViewerPageLimit < len(s.sectionRowsCapped()) {
			s.pages[current]++
			s.selected[current], s.viewports[current] = 0, 0
			return true, false
		}
	}
	return false, false
}

func (s *usageViewerState) sectionRowsCapped() []usageViewerRow {
	rows := s.sectionRows()
	if s.top != nil && *s.top > 0 && (s.section == usageViewerModels || s.section == usageViewerProviders || s.section == usageViewerCache) {
		return rows[:min(len(rows), *s.top)]
	}
	return rows
}

// sectionRows is the report-adapter boundary. Report labels and detail text
// carry provider, model, client, and session identifiers that originate in
// session metadata, so every visible field is sanitized here: no untrusted
// escape or control byte can reach the alternate screen, while the viewer's own
// screen-control and style sequences are added afterwards by the renderer.
func (s *usageViewerState) sectionRows() []usageViewerRow {
	rows := s.reportRows()
	for i := range rows {
		rows[i].label = output.SanitizeTerminalCell(rows[i].label)
		rows[i].value = output.SanitizeTerminalCell(rows[i].value)
		rows[i].detail = normalizeTerminalDetail(rows[i].detail)
	}
	return rows
}

func (s *usageViewerState) reportRows() []usageViewerRow {
	metric := strings.ToUpper(s.report.Metric)
	switch s.section {
	case usageViewerOverview:
		costAvailable := knownCostAvailable(s.report.Totals.ProviderCost, s.report.Totals.KnownProviderCost, s.report.Coverage.Percent)
		return []usageViewerRow{
			{identity: "tokens", label: "TOKENS", value: compactNumber(float64(s.report.Totals.Tokens)), valueColor: usageColorToken, detail: usageViewerTokenDetail(s.report.Totals)},
			{identity: "cost", label: "COST", value: compactCost(s.report.Totals.ProviderCost, s.report.Totals.KnownProviderCost, costAvailable), valueColor: usageCostColor(s.report.Totals.ProviderCost, costAvailable), detail: usageViewerCostDetail(s.report.Totals, s.report.Coverage)},
			{identity: "sessions", label: "SESSIONS", value: groupedInt(s.report.Totals.Sessions), valueColor: usageColorSession, detail: usageViewerSessionDetail(s.report.Totals)},
			{identity: "priced", label: "PRICED", value: s.report.Coverage.Percent + "%", valueColor: usageCoverageColor(s.report.Coverage.Percent), detail: usageViewerCoverageDetail(s.report.Coverage)},
		}
	case usageViewerTrend:
		rows := make([]usageViewerRow, 0, len(s.report.Buckets))
		peak := 0.0
		for _, b := range s.report.Buckets {
			v, err := strconv.ParseFloat(b.KnownMetricValue, 64)
			available := err == nil && (metric != "COST" || knownCostAvailable(b.MetricValue, b.KnownMetricValue, b.Coverage))
			if available {
				peak = max(peak, v)
			}
		}
		for _, b := range s.report.Buckets {
			v, err := strconv.ParseFloat(b.KnownMetricValue, 64)
			available := err == nil && (metric != "COST" || knownCostAvailable(b.MetricValue, b.KnownMetricValue, b.Coverage))
			valueColor := usageMetricColor(metric)
			if metric == "COST" {
				valueColor = usageCostColor(b.MetricValue, available)
			}
			rows = append(rows, usageViewerRow{
				identity:   b.Start,
				label:      b.Start,
				value:      usageViewerMetric(b.MetricValue, b.KnownMetricValue, b.Coverage, metric),
				bar:        v / max(1, peak),
				showBar:    available && peak > 0,
				valueColor: valueColor,
				barColor:   valueColor,
				detail:     usageViewerTrendDetail(b, metric),
			})
		}
		return rows
	case usageViewerActivity:
		return s.activityRows()
	case usageViewerModels, usageViewerClients, usageViewerProviders:
		var dimensions []usage.StatsDimension
		switch s.section {
		case usageViewerModels:
			dimensions = s.report.Models
		case usageViewerClients:
			dimensions = s.report.Clients
		default:
			dimensions = s.report.Providers
		}
		rows := make([]usageViewerRow, 0, len(dimensions))
		for _, d := range dimensions {
			// Only the client component is a known short enum, so it is the
			// only part that carries ordinary text's title casing. Model and
			// provider names stay verbatim, exactly as the ordinary renderer
			// prints them.
			name := d.Name
			client := d.Client
			switch {
			case s.section == usageViewerClients:
				name = statsTitle(d.Name)
				client = d.Name
			case s.section == usageViewerProviders && d.Client != "":
				name = statsTitle(d.Client) + "/" + d.Name
			}
			share, err := strconv.ParseFloat(d.KnownShare, 64)
			available := err == nil && (metric != "COST" || knownCostAvailable(d.MetricValue, d.KnownMetricValue, d.Coverage))
			shareLabel := "unavailable"
			if available {
				shareLabel = formatPercent(share)
			}
			identityColor := usageClientColor(client)
			valueColor := usageMetricColor(metric)
			if !available {
				valueColor = usageColorWarning
			}
			rows = append(rows, usageViewerRow{
				identity:   d.Client + "\x00" + d.Name,
				label:      name,
				value:      shareLabel,
				bar:        share / 100,
				showBar:    available,
				labelColor: identityColor,
				valueColor: valueColor,
				barColor:   identityColor,
				detail:     usageViewerDimensionDetail(d),
			})
		}
		return rows
	case usageViewerCache:
		rows := make([]usageViewerRow, 0, len(s.report.CacheSessions))
		for _, c := range s.report.CacheSessions {
			rate, value := "unavailable", 0.0
			available := false
			if c.CacheHitRate != nil {
				parsed, err := strconv.ParseFloat(*c.CacheHitRate, 64)
				if err == nil {
					rate = *c.CacheHitRate + "%"
					value = parsed / 100
					available = true
				}
			}
			valueColor := usageColorWarning
			if available {
				valueColor = usageColorSuccess
			}
			rows = append(rows, usageViewerRow{
				identity:   c.Client + "\x00" + c.SessionID,
				label:      c.Client + "/" + c.SessionID,
				value:      rate,
				bar:        value,
				showBar:    available,
				labelColor: usageClientColor(c.Client),
				valueColor: valueColor,
				barColor:   usageColorSuccess,
				detail:     usageViewerCacheDetail(c),
			})
		}
		return rows
	case usageViewerCoverage:
		total := s.report.Coverage.TotalEvents
		pricedRatio, unpricedRatio := 0.0, 0.0
		showCoverageBars := total > 0
		if showCoverageBars {
			pricedRatio = float64(s.report.Coverage.PricedEvents) / float64(total)
			unpricedRatio = float64(s.report.Coverage.UnpricedEvents) / float64(total)
		}
		rows := []usageViewerRow{
			{identity: "priced", label: "PRICED EVENTS", value: groupedInt(s.report.Coverage.PricedEvents), bar: pricedRatio, showBar: showCoverageBars, valueColor: usageCoverageColor(s.report.Coverage.Percent), barColor: usageColorSuccess, detail: usageViewerCoverageRowDetail(s.report.Coverage, "priced")},
			{identity: "unpriced", label: "UNPRICED EVENTS", value: groupedInt(s.report.Coverage.UnpricedEvents), bar: unpricedRatio, showBar: showCoverageBars, valueColor: usageColorWarning, barColor: usageColorWarning, detail: usageViewerCoverageRowDetail(s.report.Coverage, "unpriced")},
		}
		if len(s.warnings) > 0 {
			rows = append(rows, usageViewerRow{identity: "warnings", label: "WARNINGS", value: groupedInt(int64(len(s.warnings))), valueColor: usageColorWarning, detail: usageViewerWarningsDetail(s.warnings)})
		}
		return rows
	default:
		return nil
	}
}

func (s *usageViewerState) activityRows() []usageViewerRow {
	values := make([]float64, 7*24)
	maximum := 0.0
	for _, activity := range s.report.Activity {
		if activity.Weekday < 0 || activity.Weekday >= 7 || activity.Hour < 0 || activity.Hour >= 24 {
			continue
		}
		value, err := strconv.ParseFloat(activity.KnownMetricValue, 64)
		if err != nil {
			continue
		}
		values[activity.Weekday*24+activity.Hour] = value
		maximum = max(maximum, value)
	}

	shortDays := [...]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	rows := make([]usageViewerRow, 0, len(shortDays))
	for weekday := range shortDays {
		levels := make([]int, 24)
		peakHour, peakValue := 0, 0.0
		for hour := range levels {
			value := values[weekday*24+hour]
			levels[hour] = heatLevel(value, maximum)
			if value > peakValue {
				peakHour, peakValue = hour, value
			}
		}
		detail := terminalDetailModel{}
		if peakValue > 0 {
			detail.fields = append(detail.fields,
				terminalDetailField{label: "PEAK WINDOW", value: fmt.Sprintf("%02d:00–%02d:00", peakHour, (peakHour+1)%24), role: terminalDetailRoleNeutral, priority: terminalDetailPriorityPrimary},
				terminalDetailField{label: "PEAK " + strings.ToUpper(s.report.Metric), value: s.activityMetricValue(peakValue), role: usageViewerMetricDetailRole(s.report.Metric), priority: terminalDetailPriorityPrimary},
			)
			detail.notes = append(detail.notes, terminalDetailNote{text: "1H BUCKETS · " + strings.ToUpper(s.report.Metric), role: terminalDetailRoleNeutral, priority: terminalDetailPrioritySecondary})
		} else if strings.EqualFold(s.report.Metric, "cost") && !knownCostAvailable(s.report.Totals.ProviderCost, s.report.Totals.KnownProviderCost, s.report.Coverage.Percent) {
			detail.notes = append(detail.notes, terminalDetailNote{text: "No priced events.", status: "UNAVAILABLE", role: terminalDetailRoleWarning, priority: terminalDetailPriorityPrimary})
		}
		rows = append(rows, usageViewerRow{identity: shortDays[weekday], label: shortDays[weekday], labelColor: usageColorInfo, heatmap: levels, detail: detail})
	}
	return rows
}

func (s *usageViewerState) activityMetricValue(value float64) string {
	switch strings.ToLower(s.report.Metric) {
	case "cost":
		if !knownCostAvailable(s.report.Totals.ProviderCost, s.report.Totals.KnownProviderCost, s.report.Coverage.Percent) {
			return "unavailable"
		}
		label := "$" + compactDecimal(value)
		if s.report.Totals.ProviderCost == nil {
			label += " KNOWN"
		}
		return label
	case "sessions":
		return groupedInt(int64(value))
	default:
		return compactNumber(value)
	}
}

// usageViewerDimensionDetail keeps the four primary fields first so that a
// short frame degrades by dropping cache accounting rather than the tokens,
// cost, sessions, and coverage a dimension always has. Cache fields appear only
// when the dimension actually carries cache accounting.
func usageViewerCacheDetail(cache usage.StatsCacheSession) terminalDetailModel {
	detail := terminalDetailModel{}
	usageViewerAppendDetailField(&detail, "CACHE READ", compactNumber(float64(cache.CachedReadTokens)), terminalDetailRoleToken, terminalDetailPriorityPrimary, cache.CachedReadTokens > 0)
	usageViewerAppendDetailField(&detail, "CACHE WRITE", compactNumber(float64(cache.CacheWriteTokens)), terminalDetailRoleToken, terminalDetailPriorityPrimary, cache.CacheWriteTokens > 0)
	usageViewerAppendDetailField(&detail, "LOGICAL INPUT", compactNumber(float64(cache.LogicalInputTokens)), terminalDetailRoleToken, terminalDetailPrioritySecondary, cache.LogicalInputTokens > 0)
	if command := strings.TrimSpace(cache.DetailCommand); command != "" {
		detail.notes = append(detail.notes, terminalDetailNote{text: command, role: terminalDetailRoleNeutral, priority: terminalDetailPriorityTertiary})
	}
	return detail
}

func usageViewerTokenDetail(totals usage.StatsTotals) terminalDetailModel {
	detail := terminalDetailModel{}
	usageViewerAppendDetailField(&detail, "INPUT TOKENS", compactNumber(float64(totals.InputTokens)), terminalDetailRoleToken, terminalDetailPriorityPrimary, totals.InputTokens > 0)
	usageViewerAppendDetailField(&detail, "OUTPUT TOKENS", compactNumber(float64(totals.OutputTokens)), terminalDetailRoleToken, terminalDetailPriorityPrimary, totals.OutputTokens > 0)
	usageViewerAppendDetailField(&detail, "CACHE READ TOKENS", compactNumber(float64(totals.CachedReadTokens)), terminalDetailRoleToken, terminalDetailPrioritySecondary, totals.CachedReadTokens > 0)
	usageViewerAppendDetailField(&detail, "CACHE WRITE TOKENS", compactNumber(float64(totals.CacheWriteTokens)), terminalDetailRoleToken, terminalDetailPrioritySecondary, totals.CacheWriteTokens > 0)
	usageViewerAppendDetailField(&detail, "AVERAGE TOKENS / SESSION", totals.AverageTokens, terminalDetailRoleToken, terminalDetailPrioritySecondary, totals.Tokens > 0 && totals.Sessions > 0)
	return detail
}

func usageViewerCostDetail(totals usage.StatsTotals, coverage usage.StatsCoverage) terminalDetailModel {
	detail := terminalDetailModel{}
	usageViewerAppendDetailField(&detail, "CATALOG BASE COST", usageViewerExactCost(totals.CatalogBaseCost, totals.KnownCatalogBaseCost), usageViewerCostDetailRole(totals.CatalogBaseCost), terminalDetailPriorityPrimary, usageViewerNonZeroCost(totals.CatalogBaseCost, totals.KnownCatalogBaseCost))
	usageViewerAppendDetailField(&detail, "AVERAGE COST / SESSION", usageViewerExactCost(totals.AverageCost, totals.KnownAverageCost), usageViewerCostDetailRole(totals.AverageCost), terminalDetailPrioritySecondary, usageViewerNonZeroCost(totals.AverageCost, totals.KnownAverageCost))
	usageViewerAppendDetailField(&detail, "PRICED EVENTS", groupedInt(coverage.PricedEvents), terminalDetailRoleSuccess, terminalDetailPrioritySecondary, coverage.PricedEvents > 0)
	usageViewerAppendDetailField(&detail, "UNPRICED EVENTS", groupedInt(coverage.UnpricedEvents), terminalDetailRoleWarning, terminalDetailPriorityPrimary, coverage.UnpricedEvents > 0)
	if totals.ProviderCost == nil {
		if knownCostAvailable(totals.ProviderCost, totals.KnownProviderCost, coverage.Percent) {
			detail.notes = append(detail.notes, terminalDetailNote{text: "Provider cost includes priced events only.", status: "PARTIAL", role: terminalDetailRoleWarning, priority: terminalDetailPriorityPrimary})
		} else {
			detail.notes = append(detail.notes, terminalDetailNote{text: "No priced events.", status: "UNAVAILABLE", role: terminalDetailRoleWarning, priority: terminalDetailPriorityPrimary})
		}
	}
	return detail
}

func usageViewerSessionDetail(totals usage.StatsTotals) terminalDetailModel {
	detail := terminalDetailModel{}
	usageViewerAppendDetailField(&detail, "EVENTS", groupedInt(totals.Events), terminalDetailRoleSession, terminalDetailPriorityPrimary, totals.Events > 0)
	usageViewerAppendDetailField(&detail, "AVERAGE TOKENS / SESSION", totals.AverageTokens, terminalDetailRoleToken, terminalDetailPrioritySecondary, totals.Tokens > 0 && totals.Sessions > 0)
	usageViewerAppendDetailField(&detail, "AVERAGE COST / SESSION", usageViewerExactCost(totals.AverageCost, totals.KnownAverageCost), usageViewerCostDetailRole(totals.AverageCost), terminalDetailPrioritySecondary, usageViewerNonZeroCost(totals.AverageCost, totals.KnownAverageCost))
	return detail
}

func usageViewerCoverageDetail(coverage usage.StatsCoverage) terminalDetailModel {
	detail := terminalDetailModel{}
	usageViewerAppendDetailField(&detail, "PRICED EVENTS", groupedInt(coverage.PricedEvents), terminalDetailRoleSuccess, terminalDetailPriorityPrimary, coverage.PricedEvents > 0)
	usageViewerAppendDetailField(&detail, "UNPRICED EVENTS", groupedInt(coverage.UnpricedEvents), terminalDetailRoleWarning, terminalDetailPriorityPrimary, coverage.UnpricedEvents > 0)
	usageViewerAppendDetailField(&detail, "TOTAL EVENTS", groupedInt(coverage.TotalEvents), terminalDetailRoleSession, terminalDetailPrioritySecondary, coverage.TotalEvents > 0)
	return detail
}

func usageViewerTrendDetail(bucket usage.StatsBucket, metric string) terminalDetailModel {
	detail := terminalDetailModel{}
	usageViewerAppendDetailField(&detail, "ENDS", bucket.End, terminalDetailRoleNeutral, terminalDetailPriorityPrimary, strings.TrimSpace(bucket.End) != "" && bucket.End != bucket.Start)
	usageViewerAppendDetailField(&detail, "PRICING COVERAGE", strings.TrimSpace(bucket.Coverage)+"%", usageViewerCoverageDetailRole(bucket.Coverage), terminalDetailPriorityPrimary, strings.TrimSpace(bucket.Coverage) != "")
	usageViewerAppendDetailField(&detail, "SESSIONS", groupedInt(bucket.Sessions), terminalDetailRoleSession, terminalDetailPrioritySecondary, !strings.EqualFold(metric, "sessions") && bucket.Sessions > 0)
	if strings.EqualFold(metric, "tokens") {
		usageViewerAppendDetailField(&detail, "INPUT TOKENS", compactNumber(float64(bucket.InputTokens)), terminalDetailRoleToken, terminalDetailPrioritySecondary, bucket.InputTokens > 0)
		usageViewerAppendDetailField(&detail, "OUTPUT TOKENS", compactNumber(float64(bucket.OutputTokens)), terminalDetailRoleToken, terminalDetailPrioritySecondary, bucket.OutputTokens > 0)
		usageViewerAppendDetailField(&detail, "CACHE READ TOKENS", compactNumber(float64(bucket.CachedReadTokens)), terminalDetailRoleToken, terminalDetailPriorityTertiary, bucket.CachedReadTokens > 0)
		usageViewerAppendDetailField(&detail, "CACHE WRITE TOKENS", compactNumber(float64(bucket.CacheWriteTokens)), terminalDetailRoleToken, terminalDetailPriorityTertiary, bucket.CacheWriteTokens > 0)
	}
	return detail
}

func usageViewerCoverageRowDetail(coverage usage.StatsCoverage, selected string) terminalDetailModel {
	detail := terminalDetailModel{}
	usageViewerAppendDetailField(&detail, "PRICING COVERAGE", strings.TrimSpace(coverage.Percent)+"%", usageViewerCoverageDetailRole(coverage.Percent), terminalDetailPriorityPrimary, strings.TrimSpace(coverage.Percent) != "")
	usageViewerAppendDetailField(&detail, "PRICED EVENTS", groupedInt(coverage.PricedEvents), terminalDetailRoleSuccess, terminalDetailPriorityPrimary, selected != "priced" && coverage.PricedEvents > 0)
	usageViewerAppendDetailField(&detail, "UNPRICED EVENTS", groupedInt(coverage.UnpricedEvents), terminalDetailRoleWarning, terminalDetailPriorityPrimary, selected != "unpriced" && coverage.UnpricedEvents > 0)
	usageViewerAppendDetailField(&detail, "TOTAL EVENTS", groupedInt(coverage.TotalEvents), terminalDetailRoleSession, terminalDetailPrioritySecondary, coverage.TotalEvents > 0)
	return detail
}

func usageViewerWarningsDetail(warnings []string) terminalDetailModel {
	detail := terminalDetailModel{notes: make([]terminalDetailNote, 0, len(warnings))}
	for _, warning := range warnings {
		if strings.TrimSpace(warning) == "" {
			continue
		}
		detail.notes = append(detail.notes, terminalDetailNote{text: warning, status: "WARNING", role: terminalDetailRoleWarning, priority: terminalDetailPriorityPrimary})
	}
	return detail
}

func usageViewerAppendDetailField(detail *terminalDetailModel, label, value string, role terminalDetailRole, priority int, include bool) {
	value = strings.TrimSpace(value)
	if !include || value == "" || strings.EqualFold(value, "unknown") || strings.EqualFold(value, "unavailable") {
		return
	}
	detail.fields = append(detail.fields, terminalDetailField{label: label, value: value, role: role, priority: priority})
}

func usageViewerExactCost(complete *string, known string) string {
	value, suffix := strings.TrimSpace(known), " KNOWN"
	if complete != nil && strings.TrimSpace(*complete) != "" {
		value, suffix = strings.TrimSpace(*complete), ""
	}
	if value == "" {
		return ""
	}
	return "$" + strings.TrimPrefix(value, "$") + suffix
}

func usageViewerNonZeroCost(complete *string, known string) bool {
	value := strings.TrimSpace(known)
	if complete != nil {
		value = strings.TrimSpace(*complete)
	}
	parsed, err := strconv.ParseFloat(strings.TrimPrefix(value, "$"), 64)
	return err == nil && parsed != 0
}

func usageViewerCostDetailRole(complete *string) terminalDetailRole {
	if complete == nil {
		return terminalDetailRoleWarning
	}
	return terminalDetailRoleCost
}

func usageViewerCoverageDetailRole(percent string) terminalDetailRole {
	value, err := strconv.ParseFloat(strings.TrimSpace(percent), 64)
	if err != nil || value < 100 {
		return terminalDetailRoleWarning
	}
	return terminalDetailRoleSuccess
}

func usageViewerMetricDetailRole(metric string) terminalDetailRole {
	switch strings.ToUpper(strings.TrimSpace(metric)) {
	case "TOKENS":
		return terminalDetailRoleToken
	case "COST":
		return terminalDetailRoleCost
	case "SESSIONS":
		return terminalDetailRoleSession
	default:
		return terminalDetailRoleNeutral
	}
}

func usageViewerDimensionDetail(d usage.StatsDimension) terminalDetailModel {
	detail := terminalDetailModel{}
	usageViewerAppendDetailField(&detail, "TOKENS", compactNumber(float64(d.Tokens)), terminalDetailRoleToken, terminalDetailPriorityPrimary, d.Tokens > 0)
	usageViewerAppendDetailField(&detail, "PROVIDER COST", usageViewerExactCost(d.ProviderCost, d.KnownProviderCost), usageViewerCostDetailRole(d.ProviderCost), terminalDetailPriorityPrimary, usageViewerNonZeroCost(d.ProviderCost, d.KnownProviderCost))
	usageViewerAppendDetailField(&detail, "SESSIONS", groupedInt(d.Sessions), terminalDetailRoleSession, terminalDetailPriorityPrimary, d.Sessions > 0)
	usageViewerAppendDetailField(&detail, "PRICING COVERAGE", strings.TrimSpace(d.Coverage)+"%", usageViewerCoverageDetailRole(d.Coverage), terminalDetailPriorityPrimary, strings.TrimSpace(d.Coverage) != "")
	if d.CacheHitRate != nil {
		rate := strings.TrimSpace(*d.CacheHitRate)
		role := terminalDetailRoleSuccess
		if _, err := strconv.ParseFloat(rate, 64); err != nil {
			role = terminalDetailRoleWarning
		}
		usageViewerAppendDetailField(&detail, "CACHE HIT RATE", rate+"%", role, terminalDetailPrioritySecondary, rate != "")
	}
	usageViewerAppendDetailField(&detail, "CACHE READ", compactNumber(float64(d.CachedReadTokens)), terminalDetailRoleToken, terminalDetailPrioritySecondary, d.CachedReadTokens > 0)
	usageViewerAppendDetailField(&detail, "CACHE WRITE", compactNumber(float64(d.CacheWriteTokens)), terminalDetailRoleToken, terminalDetailPrioritySecondary, d.CacheWriteTokens > 0)
	usageViewerAppendDetailField(&detail, "LOGICAL INPUT", compactNumber(float64(d.LogicalInputTokens)), terminalDetailRoleToken, terminalDetailPrioritySecondary, d.LogicalInputTokens > 0)
	usageViewerAppendDetailField(&detail, "WRAPPER EVENTS", groupedInt(d.WrapperEvents), terminalDetailRoleSession, terminalDetailPrioritySecondary, d.WrapperEvents > 0)
	return detail
}

func usageViewerMetric(complete *string, known, coverage, metric string) string {
	if metric == "COST" {
		return compactCost(complete, known, knownCostAvailable(complete, known, coverage))
	}
	value, _ := strconv.ParseFloat(known, 64)
	if metric == "SESSIONS" {
		return groupedInt(int64(value))
	}
	return compactNumber(value)
}

func usageStatsInteractiveTerminal(stdin io.Reader, stdout io.Writer) (*os.File, *os.File, error) {
	input, ok := stdin.(*os.File)
	if !ok {
		return nil, nil, errors.New("usage stats --interactive requires TTY stdin and stdout")
	}
	output, ok := stdout.(*os.File)
	if !ok || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return nil, nil, errors.New("usage stats --interactive requires TTY stdin and stdout")
	}
	if os.Getenv("TERM") == "dumb" {
		return nil, nil, errors.New("usage stats --interactive requires a usable terminal")
	}
	width, height, err := term.GetSize(int(output.Fd()))
	if err != nil || width < 48 || height < 10 {
		return nil, nil, errors.New("usage stats --interactive requires a terminal at least 48x10")
	}
	return input, output, nil
}

func runUsageStatsViewer(ctx context.Context, input, output *os.File, report usage.StatsReport, warnings []string, noColor bool, top *int) error {
	if _, _, err := usageStatsInteractiveTerminal(input, output); err != nil {
		return err
	}
	width, height, _ := term.GetSize(int(output.Fd()))
	viewer := newUsageViewerState(report, warnings, top)
	terminal, err := startInteractiveTerminal(input, output)
	if err != nil {
		return err
	}
	defer terminal.Close()
	p := newUsageTextPrimitives(output, noColor)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		width, height, _ = term.GetSize(int(output.Fd()))
		if err := renderUsageStatsViewer(terminal.frameWriter(), max(1, width), max(1, height), viewer, p); err != nil {
			return err
		}
		key, resizedDuringRead, err := readSessionViewerKey(ctx, input, terminal.resized)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if sessionViewerShouldRedrawAfterRead(key, resizedDuringRead) {
			continue
		}
		reload, exit := viewer.apply(key)
		if exit {
			return nil
		}
		if reload {
			viewer.refresh()
		}
	}
}

func renderUsageStatsViewer(w io.Writer, width, height int, s *usageViewerState, p usageTextPrimitives) error {
	if _, err := io.WriteString(w, "\x1b[H\x1b[2J"); err != nil {
		return err
	}
	if width < 48 || height < 10 {
		return renderUsageStatsViewerTooSmall(w, width, height)
	}
	if _, err := fmt.Fprintln(w, statsFit(p.style("AGENTDECK · USAGE", usageColorBrand)+" "+p.style("INTERACTIVE · READ ONLY", usageColorInfo), width)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, usageViewerTabLine(s.section, width, p)); err != nil {
		return err
	}
	from, to := compactStatsDisplayRange(s.report.Range)
	meta := output.SanitizeTerminalCell(fmt.Sprintf("%s–%s · %s · %s · %s", from, to, s.report.Timezone, strings.ToUpper(s.report.Metric), s.report.GroupBy))
	if _, err := fmt.Fprintln(w, statsFit(meta, width)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, usageViewerKPIline(s.report, width, p)); err != nil {
		return err
	}

	selected := s.selected[s.section]
	bodyBudget := max(2, height-6) // title, tabs, meta, KPIs, status, and help.
	guide := []string(nil)
	if s.section == usageViewerActivity {
		guide = usageViewerActivityGuide(width, bodyBudget, p)
	}
	contentBudget := max(1, bodyBudget-len(guide))
	detail := []string(nil)
	if len(s.rows) > 0 {
		detail = usageViewerDetail(s.rows[selected], width, p)
	}
	sideBySide := width >= 120 && len(s.rows) > 0 && len(detail) > 0 && len(s.rows)+len(detail) > contentBudget
	leftWidth, rightWidth := width, width
	if sideBySide {
		leftWidth = width*2/3 - 2
		rightWidth = width - width*2/3
		detail = usageViewerDetailWithin(s.rows[selected], rightWidth, contentBudget, p)
	} else if len(detail) > 0 {
		detail = usageViewerDetailWithin(s.rows[selected], width, max(0, contentBudget-1), p)
	}

	viewport := max(1, contentBudget-len(detail))
	if sideBySide {
		viewport = contentBudget
	}
	start := s.viewportOffset(viewport)
	end := min(len(s.rows), start+viewport)
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		lines = append(lines, usageViewerRowLine(s.rows[i], i == selected, leftWidth, p))
	}
	if len(lines) == 0 {
		lines = append(lines, p.style("No rows in this section.", usageColorWarning))
	}
	for _, line := range guide {
		if _, err := fmt.Fprintln(w, statsFit(line, width)); err != nil {
			return err
		}
	}
	if sideBySide {
		for _, line := range usageJoinColumns(lines, leftWidth, detail, rightWidth, 2) {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
	} else {
		for _, line := range lines {
			if _, err := fmt.Fprintln(w, statsFit(line, width)); err != nil {
				return err
			}
		}
		if len(s.rows) > 0 {
			for _, line := range detail {
				if _, err := fmt.Fprintln(w, statsFit(line, width)); err != nil {
					return err
				}
			}
		}
	}

	status := fmt.Sprintf("page %d · %d rows", s.pages[s.section], len(s.sectionRowsCapped()))
	if len(s.rows) == 0 {
		status += " · no selection"
	} else {
		status += fmt.Sprintf(" · row %d/%d", selected+1, len(s.rows))
	}
	statusColor := usageColorInfo
	if len(s.warnings) > 0 {
		status += " · warning"
		statusColor = usageColorWarning
	}
	if _, err := fmt.Fprintln(w, statsFit(p.style(status, statusColor), width)); err != nil {
		return err
	}
	help := "←/→ tabs · ↑/↓ select · pgup/pgdn page · home/end · ? help · q/esc quit"
	if width < 80 && !s.help {
		help = "? help · q quit"
	}
	_, err := fmt.Fprintln(w, statsFit(help, width))
	return err
}

func usageViewerKPIline(report usage.StatsReport, width int, p usageTextPrimitives) string {
	costAvailable := knownCostAvailable(report.Totals.ProviderCost, report.Totals.KnownProviderCost, report.Coverage.Percent)
	parts := []string{
		p.style("TOKENS "+compactNumber(float64(report.Totals.Tokens)), usageColorToken),
		p.style("COST "+compactCost(report.Totals.ProviderCost, report.Totals.KnownProviderCost, costAvailable), usageCostColor(report.Totals.ProviderCost, costAvailable)),
		p.style("SESSIONS "+groupedInt(report.Totals.Sessions), usageColorSession),
		p.style("PRICED "+report.Coverage.Percent+"%", usageCoverageColor(report.Coverage.Percent)),
	}
	return statsFit(strings.Join(parts, "   "), width)
}

func usageViewerTabLine(section usageViewerSection, width int, p usageTextPrimitives) string {
	tokens := make([]string, len(usageViewerSections))
	for i, name := range usageViewerSections {
		if usageViewerSection(i) == section {
			tokens[i] = p.style("["+name+"]", usageColorBrand)
		} else {
			tokens[i] = name
		}
	}
	build := func(start, end int) string {
		visible := append([]string(nil), tokens[start:end]...)
		if start > 0 {
			visible = append([]string{"‹"}, visible...)
		}
		if end < len(tokens) {
			visible = append(visible, "›")
		}
		return strings.Join(visible, " ")
	}
	if full := build(0, len(tokens)); statsVisibleWidth(full) <= width {
		return full
	}
	start, end := int(section), int(section)+1
	for {
		expanded := false
		if start > 0 {
			candidate := build(start-1, end)
			if statsVisibleWidth(candidate) <= width {
				start--
				expanded = true
			}
		}
		if end < len(tokens) {
			candidate := build(start, end+1)
			if statsVisibleWidth(candidate) <= width {
				end++
				expanded = true
			}
		}
		if !expanded {
			return statsFit(build(start, end), width)
		}
	}
}

func usageViewerActivityGuide(width, bodyBudget int, p usageTextPrimitives) []string {
	legend := "1H BUCKET · LESS " + p.heatmapCell(0) + " " + p.heatmapCell(1) + " " + p.heatmapCell(2) + " " + p.heatmapCell(3) + " " + p.heatmapCell(4) + " MORE"
	if bodyBudget < 6 {
		return []string{statsFit(legend, width)}
	}
	axis := "     00 03 06 09 12 15 18 21"
	if width >= 58 {
		axis = "     00    03    06    09    12    15    18    21"
	}
	return []string{statsFit(legend, width), statsFit(axis, width)}
}

func usageViewerRowLine(row usageViewerRow, selected bool, width int, p usageTextPrimitives) string {
	prefix := "  "
	if selected {
		prefix = p.style("> ", usageColorBrand)
	}
	if len(row.heatmap) > 0 {
		separator := ""
		if width >= 58 {
			separator = " "
		}
		cells := make([]string, 0, len(row.heatmap))
		for _, level := range row.heatmap {
			cells = append(cells, p.heatmapCell(level))
		}
		label := p.style(statsPad(row.label, 3), row.labelColor)
		return statsFit(prefix+label+" "+strings.Join(cells, separator), width)
	}

	labelWidth := min(28, max(10, width/3))
	label := statsPad(p.style(statsFit(row.label, labelWidth), row.labelColor), labelWidth)
	value := p.style(row.value, row.valueColor)
	line := prefix + label
	if row.showBar {
		barWidth := min(20, max(0, width-statsVisibleWidth(prefix)-labelWidth-statsVisibleWidth(row.value)-3))
		if barWidth > 0 {
			line += " " + p.barTrack(scaledBar(row.bar, 1, barWidth), barWidth, row.barColor)
		}
	}
	if row.value != "" {
		line += " " + value
	}
	return statsFit(line, width)
}

// renderUsageStatsViewerTooSmall deliberately has no state mutation: a resize
// below the entry minimum is transient, so the previous section/page/selection
// is available unchanged as soon as the terminal recovers.
func renderUsageStatsViewerTooSmall(w io.Writer, width, height int) error {
	lines := []string{
		"AGENTDECK · USAGE",
		"Terminal too small for interactive usage.",
		"Resize to at least 48x10.",
		"q/esc quit",
	}
	for _, line := range lines[:min(len(lines), max(1, height))] {
		if _, err := fmt.Fprintln(w, statsFit(line, max(1, width))); err != nil {
			return err
		}
	}
	return nil
}

func usageViewerDetail(row usageViewerRow, width int, p usageTextPrimitives) []string {
	detail := row.detail
	detail.title = row.label
	return renderTerminalDetailModel(detail, width, p)
}

// usageViewerDetailWithin preserves Detail identity and semantic warning state
// when a short terminal cannot fit the full card. Frames that fit retain the
// shared renderer's complete ordering.
func usageViewerDetailWithin(row usageViewerRow, width, limit int, p usageTextPrimitives) []string {
	if limit <= 0 {
		return nil
	}
	full := usageViewerDetail(row, width, p)
	if len(full) <= limit {
		return full
	}

	detail := row.detail
	detail.title = row.label
	priorityNotes := make([]terminalDetailNote, 0, len(detail.notes))
	for _, note := range detail.notes {
		if note.priority == terminalDetailPriorityPrimary ||
			note.role == terminalDetailRoleWarning || note.role == terminalDetailRoleError {
			priorityNotes = append(priorityNotes, note)
		}
	}
	if len(priorityNotes) == 0 {
		return full[:limit]
	}

	priority := renderTerminalDetailModel(terminalDetailModel{
		title: detail.title,
		notes: priorityNotes,
	}, width, p)
	if len(priority) >= limit {
		return priority[:limit]
	}

	result := append([]string(nil), priority...)
	for _, line := range full[1:] {
		if len(result) == limit {
			break
		}
		result = append(result, line)
	}
	return result
}
