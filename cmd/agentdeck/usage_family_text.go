package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/kitdine/agent-deck/internal/usage"
)

// usageFamilyRenderer keeps the older usage report commands on the same
// terminal primitives as usage stats without changing their JSON envelopes.
type usageFamilyRenderer struct {
	primitives usageTextPrimitives
	width      int
}

func newUsageFamilyRenderer(w io.Writer, options usageTextRenderOptions) usageFamilyRenderer {
	primitives := newUsageTextPrimitives(w, false)
	if options.width > 0 {
		primitives.width = min(max(options.width, statsMinWidth), statsMaxWidth)
	}
	primitives.color = options.color
	return usageFamilyRenderer{primitives: primitives, width: primitives.width}
}

func (r usageFamilyRenderer) section(label, color string) string {
	return r.primitives.sectionTitle(label, r.width, color)
}

func writeUsageFamilyLines(w io.Writer, lines []string) error {
	_, err := io.WriteString(w, strings.Join(lines, "\n")+"\n")
	return err
}

func renderUsageFamilyMetricTable(w io.Writer, title string, rows [][]string, options usageTextRenderOptions) error {
	renderer := newUsageFamilyRenderer(w, options)
	columns := make([][]usageAlignedColumn, len(rows))
	for index, row := range rows {
		columns[index] = []usageAlignedColumn{{label: strings.ToUpper(row[0]), value: row[1]}}
	}
	usageAlignColumnRows(columns)
	lines := []string{renderer.section(title, "1;33")}
	for _, row := range columns {
		lines = append(lines, usageAlignedColumns(renderer.width, row...)...)
	}
	return writeUsageFamilyLines(w, lines)
}

func renderUsageFamilySummary(w io.Writer, value usage.Summary, options usageTextRenderOptions) error {
	renderer := newUsageFamilyRenderer(w, options)
	metricRows := [][]usageAlignedColumn{
		{{label: "EVENTS", value: strconv.FormatInt(value.Counts["events"], 10)}},
		{{label: "EXACT ATTRIBUTION", value: strconv.FormatInt(value.Counts["exact"], 10)}},
		{{label: "ESTIMATED ATTRIBUTION", value: strconv.FormatInt(value.Counts["estimated"], 10)}},
		{{label: "UNATTRIBUTED ATTRIBUTION", value: strconv.FormatInt(value.Counts["unattributed"], 10)}},
		{{label: "PRICED EVENTS", value: strconv.FormatInt(value.Counts["priced"], 10)}},
		{{label: "UNPRICED EVENTS", value: strconv.FormatInt(value.Counts["unpriced"], 10)}},
		{{label: "CATALOG BASE TOTAL", value: optionalCost(value.CatalogBaseCost)}},
		{{label: "PROVIDER TOTAL", value: optionalCost(value.ProviderCost)}},
		{{label: "KNOWN CATALOG SUBTOTAL", value: optionalCost(value.KnownCatalogBaseCost)}},
		{{label: "KNOWN PROVIDER SUBTOTAL", value: optionalCost(value.KnownProviderCost)}},
		{{label: "WARNINGS", value: textList(value.Warnings)}},
		{{label: "UNPRICED", value: textList(value.Unpriced)}},
	}
	usageAlignColumnRows(metricRows)
	lines := []string{renderer.section("📊 USAGE SUMMARY", "1;32")}
	for _, row := range metricRows {
		lines = append(lines, usageAlignedColumns(renderer.width, row...)...)
	}

	tokenRows := make([][]usageAlignedColumn, 0, len(usageTokenNames))
	for _, token := range usageTokenNames {
		tokenRows = append(tokenRows, []usageAlignedColumn{{label: strings.ToUpper(token.label), value: strconv.FormatInt(value.Tokens[token.key], 10)}})
	}
	usageAlignColumnRows(tokenRows)
	lines = append(lines, "", renderer.section("🪙 TOKEN TOTALS", "1;36"))
	for _, row := range tokenRows {
		lines = append(lines, usageAlignedColumns(renderer.width, row...)...)
	}

	lines = append(lines, "", renderer.section("🧾 MODEL COVERAGE", "1;35"))
	if len(value.Models) == 0 {
		return writeUsageFamilyLines(w, append(lines, renderer.primitives.style("No model coverage.", "2")))
	}
	modelDetails := make([][]usageAlignedColumn, len(value.Models))
	for index, model := range value.Models {
		modelDetails[index] = []usageAlignedColumn{
			{label: "EVENTS", value: strconv.FormatInt(model.Events, 10)},
			{label: "PRICED", value: strconv.FormatInt(model.PricedEvents, 10)},
			{label: "UNPRICED", value: strconv.FormatInt(model.UnpricedEvents, 10)},
			{label: "STATUS", value: modelCoverageStatus(model)},
		}
	}
	usageAlignColumnRows(modelDetails)
	for index, model := range value.Models {
		name := statsTitle(model.Client) + " / " + statsTitle(model.Model)
		coverage := float64(0)
		if model.Events > 0 {
			coverage = float64(model.PricedEvents) / float64(model.Events) * 100
		}
		percent := fmt.Sprintf("%.0f%%", coverage)
		nameWidth := min(max(12, renderer.width/3), 32)
		barWidth := min(28, max(6, renderer.width-nameWidth-statsVisibleWidth(percent)-3))
		lines = append(lines, statsPad(statsFit(name, nameWidth), nameWidth)+" "+renderer.primitives.barTrack(scaledBar(coverage, 100, barWidth), barWidth, "35")+" "+percent)
		for _, line := range usageAlignedColumns(renderer.width, modelDetails[index]...) {
			lines = append(lines, renderer.primitives.style(line, "2"))
		}
	}
	return writeUsageFamilyLines(w, lines)
}

func renderUsageFamilySessions(w io.Writer, values []usage.SessionSummary, options usageTextRenderOptions) error {
	renderer := newUsageFamilyRenderer(w, options)
	lines := []string{renderer.section("📚 USAGE SESSIONS", "1;36")}
	if len(values) == 0 {
		return writeUsageFamilyLines(w, append(lines, renderer.primitives.style("No usage sessions.", "2")))
	}
	primary := make([][]usageAlignedColumn, len(values))
	secondary := make([][]usageAlignedColumn, len(values))
	for index, value := range values {
		primary[index] = usageSessionPrimaryColumns(value)
		secondary[index] = usageSessionSecondaryColumns(value)
	}
	usageAlignColumnRows(primary)
	usageAlignColumnRows(secondary)
	for index := range values {
		if renderer.width >= 140 {
			all := append(append([]usageAlignedColumn{}, primary[index]...), secondary[index]...)
			lines = append(lines, usageAlignedColumns(renderer.width, all...)...)
			continue
		}
		lines = append(lines, usageAlignedColumns(renderer.width, primary[index]...)...)
		for detailIndex, detail := range usageAlignedColumns(max(statsMinWidth-2, renderer.width-2), secondary[index]...) {
			prefix := "  "
			if detailIndex == 0 {
				prefix = "↳ "
			}
			lines = append(lines, renderer.primitives.style(prefix+detail, "2"))
		}
	}
	return writeUsageFamilyLines(w, lines)
}

func usageSessionPrimaryColumns(value usage.SessionSummary) []usageAlignedColumn {
	zone := displayZoneName()
	return []usageAlignedColumn{
		{label: "CLIENT", value: value.Client},
		{label: "SESSION", value: value.SessionID},
		{label: "FIRST (" + zone + ")", value: renderDisplayTime(value.FirstAt)},
		{label: "LAST (" + zone + ")", value: renderDisplayTime(value.LastAt)},
		{label: "INPUT", value: strconv.FormatInt(value.Tokens["input_tokens"], 10)},
		{label: "CACHED", value: strconv.FormatInt(value.Tokens["cached_input_tokens"], 10)},
		{label: "OUTPUT", value: strconv.FormatInt(value.Tokens["output_tokens"], 10)},
		{label: "BASE COST", value: sessionCostText(value.CatalogBaseCost, value.KnownCatalogBaseCost)},
		{label: "PROVIDER COST", value: sessionCostText(value.ProviderCost, value.KnownProviderCost)},
		{label: "STATUS", value: usageSessionStatus(value)},
	}
}

func usageSessionSecondaryColumns(value usage.SessionSummary) []usageAlignedColumn {
	return []usageAlignedColumn{
		{label: "CACHE READ", value: strconv.FormatInt(value.Tokens["cache_read_tokens"], 10)},
		{label: "CACHE CREATE", value: strconv.FormatInt(value.Tokens["cache_creation_tokens"], 10)},
		{label: "WRITE 5M", value: strconv.FormatInt(value.Tokens["cache_write_5m_tokens"], 10)},
		{label: "WRITE 1H", value: strconv.FormatInt(value.Tokens["cache_write_1h_tokens"], 10)},
		{label: "CACHE WRITE", value: strconv.FormatInt(value.Tokens["cache_write_tokens"], 10)},
	}
}
