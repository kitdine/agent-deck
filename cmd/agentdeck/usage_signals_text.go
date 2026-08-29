package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/usage"
)

func renderUsageSignalsWithOptions(w io.Writer, report usage.SignalReport, options usageTextRenderOptions) error {
	if options.width == 0 {
		options.width = statsDefaultWidth
	}
	options.width = min(max(options.width, statsMinWidth), statsMaxWidth)
	lines := usageSignalLines(report, options, "🧭 ACTIVITY")
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, strings.TrimRight(line, " ")); err != nil {
			return err
		}
	}
	return nil
}

func usageSignalLines(report usage.SignalReport, options usageTextRenderOptions, activityTitle string) []string {
	renderer := signalTextRenderer{
		report:     report,
		width:      options.width,
		primitives: usageTextPrimitives{width: options.width, color: options.color},
	}
	var lines []string
	appendSection := func(section []string) {
		if len(section) == 0 {
			return
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, section...)
	}
	if report.Activity != nil {
		appendSection(renderer.activityLines(activityTitle))
	}
	if report.Workflow != nil {
		appendSection(renderer.workflowLines())
	}
	if report.Tooling != nil {
		appendSection(renderer.toolingLines())
	}
	return lines
}

type signalTextRenderer struct {
	report     usage.SignalReport
	width      int
	primitives usageTextPrimitives
}

func (r signalTextRenderer) activityLines(title string) []string {
	lines := []string{r.primitives.sectionTitle(title, r.width, usageColorBrand)}
	activity := r.report.Activity
	if activity == nil || !activity.Available {
		return append(lines, r.primitives.style("No turn in the selected scope.", "2"))
	}
	labelWidth := 12
	for _, row := range activity.Kinds {
		labelWidth = max(labelWidth, statsVisibleWidth(statsTitle(row.Kind)))
		for _, sub := range row.Sub {
			labelWidth = max(labelWidth, statsVisibleWidth("└ "+statsTitle(sub.Kind)))
		}
	}
	labelWidth = min(labelWidth, 18)
	barWidth := min(10, max(4, r.width-labelWidth-36))
	for _, row := range activity.Kinds {
		share, cost := signalPercent(row.Share), signalMoney(row.Cost)
		if activity.CostBasis == usage.CostBasisNone {
			share, cost = "—", "—"
		}
		bar := r.primitives.barTrack(scaledBar(row.Share, 100, barWidth), barWidth, usageColorBrand)
		line := statsPad(statsTitle(row.Kind), labelWidth) + " " + bar + " " +
			statsPadLeft(share, 6) + " " + statsPadLeft(cost, 8) + "  " + signalCount(row.Events, "event")
		lines = append(lines, line)
		for _, sub := range row.Sub {
			subShare, subCost := signalPercent(sub.Share), signalMoney(sub.Cost)
			if activity.CostBasis == usage.CostBasisNone {
				subShare, subCost = "—", "—"
			}
			line = statsPad("└ "+statsTitle(sub.Kind), labelWidth+barWidth+1) + " " +
				statsPadLeft(subShare, 6) + " " + statsPadLeft(subCost, 8) + "  " + signalCount(sub.Events, "event")
			lines = append(lines, line)
		}
	}
	return lines
}

func (r signalTextRenderer) workflowLines() []string {
	lines := []string{r.primitives.sectionTitle("🧱 WORKFLOW", r.width, usageColorInfo)}
	workflow := r.report.Workflow
	if workflow == nil {
		return lines
	}
	firstEdit := signalIntValue(workflow.FirstEditSeconds, func(value int) string {
		return (time.Duration(value) * time.Second).String() + " (median)"
	})
	files := signalIntValue(workflow.FilesTouched, func(value int) string { return groupedInt(int64(value)) })
	rework := signalIntValue(workflow.Retries, func(value int) string {
		return groupedInt(int64(value)) + "  (edit, verify, edit again)"
	})
	edits := "—"
	if workflow.EditsPerSession != nil {
		edits = strconv.FormatFloat(*workflow.EditsPerSession, 'f', -1, 64)
	}
	top := "—"
	if workflow.TopFile != nil && workflow.TopFileEdits != nil {
		top = *workflow.TopFile + " ×" + strconv.Itoa(*workflow.TopFileEdits)
	}
	for _, row := range []struct{ label, value string }{
		{"FIRST EDIT", firstEdit},
		{"FILES TOUCHED", files},
		{"REWORK", rework},
		{"EDITS / SESSION", edits},
		{"MOST TOUCHED", top},
	} {
		lines = append(lines, statsPad(row.label, 16)+" "+row.value)
	}
	return lines
}

func (r signalTextRenderer) toolingLines() []string {
	lines := []string{r.primitives.sectionTitle("🔧 TOOLING", r.width, usageColorWarning)}
	tooling := r.report.Tooling
	if tooling == nil || !tooling.Available {
		return append(lines, r.primitives.style("No tool call in the selected scope.", "2"))
	}
	for _, row := range tooling.Rows {
		line := statsPad(statsTitle(row.Kind), 12) + " " + statsPadLeft(signalCount(row.Calls, "call"), 12) + "  " + statsPadLeft(signalPercent(row.Share), 6)
		lines = append(lines, line)
	}
	if tooling.TopMCPServer != "" {
		lines = append(lines, statsPad("TOP MCP", 12)+" "+tooling.TopMCPServer+" · "+signalCount(tooling.TopMCPCalls, "call"))
	}
	return lines
}

func signalIntValue(value *int, render func(int) string) string {
	if value == nil {
		return "—"
	}
	return render(*value)
}

func signalPercent(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64) + "%"
}

func signalMoney(value float64) string {
	return "$" + strconv.FormatFloat(value, 'f', 2, 64)
}

func signalCount(value int64, singular string) string {
	label := singular
	if value != 1 {
		label += "s"
	}
	return groupedInt(value) + " " + label
}
