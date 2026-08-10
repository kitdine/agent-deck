package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/usage"
	"golang.org/x/term"
)

const usageViewerPageLimit = 20

type usageViewerSection int

const (
	usageViewerOverview usageViewerSection = iota
	usageViewerTrend
	usageViewerModels
	usageViewerClients
	usageViewerProviders
	usageViewerCache
	usageViewerCoverage
)

var usageViewerSections = []string{"OVERVIEW", "TREND", "MODELS", "CLIENTS", "PROVIDERS", "CACHE", "COVERAGE"}

type usageViewerRow struct {
	identity string
	label    string
	value    string
	detail   []string
	bar      float64
}

// usageViewerState is terminal-independent, keeping navigation and selection
// invariants testable without a pseudo-terminal.
type usageViewerState struct {
	report    usage.StatsReport
	top       *int
	warnings  []string
	section   usageViewerSection
	pages     [7]int
	selected  [7]int
	viewports [7]int
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
		for j := range rows[i].detail {
			rows[i].detail[j] = output.SanitizeTerminalCell(rows[i].detail[j])
		}
	}
	return rows
}

func (s *usageViewerState) reportRows() []usageViewerRow {
	metric := strings.ToUpper(s.report.Metric)
	switch s.section {
	case usageViewerOverview:
		return []usageViewerRow{
			{identity: "tokens", label: "TOKENS", value: compactNumber(float64(s.report.Totals.Tokens)), detail: []string{"Input " + compactNumber(float64(s.report.Totals.InputTokens)), "Output " + compactNumber(float64(s.report.Totals.OutputTokens))}},
			{identity: "cost", label: "COST", value: compactCost(s.report.Totals.ProviderCost, s.report.Totals.KnownProviderCost, true), detail: []string{"Known provider cost " + s.report.Totals.KnownProviderCost}},
			{identity: "sessions", label: "SESSIONS", value: groupedInt(s.report.Totals.Sessions), detail: []string{"Events " + groupedInt(s.report.Totals.Events)}},
			{identity: "priced", label: "PRICED", value: s.report.Coverage.Percent + "%", detail: []string{fmt.Sprintf("%d priced · %d unpriced", s.report.Coverage.PricedEvents, s.report.Coverage.UnpricedEvents)}},
		}
	case usageViewerTrend:
		rows := make([]usageViewerRow, 0, len(s.report.Buckets))
		peak := 0.0
		for _, b := range s.report.Buckets {
			v, _ := strconv.ParseFloat(b.KnownMetricValue, 64)
			peak = max(peak, v)
		}
		for _, b := range s.report.Buckets {
			v, _ := strconv.ParseFloat(b.KnownMetricValue, 64)
			rows = append(rows, usageViewerRow{identity: b.Start, label: b.Start, value: usageViewerMetric(b.MetricValue, b.KnownMetricValue, b.Coverage, metric), bar: v / max(1, peak), detail: []string{"Range " + b.Start + " to " + b.End, "Coverage " + b.Coverage + "%", "Sessions " + groupedInt(b.Sessions)}})
		}
		return rows
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
			switch {
			case s.section == usageViewerClients:
				name = statsTitle(d.Name)
			case s.section == usageViewerProviders && d.Client != "":
				name = statsTitle(d.Client) + "/" + d.Name
			}
			share, _ := strconv.ParseFloat(d.KnownShare, 64)
			rows = append(rows, usageViewerRow{identity: d.Client + "\x00" + d.Name, label: name, value: formatPercent(share), bar: share / 100, detail: usageViewerDimensionDetail(d)})
		}
		return rows
	case usageViewerCache:
		rows := make([]usageViewerRow, 0, len(s.report.CacheSessions))
		for _, c := range s.report.CacheSessions {
			rate, value := "unavailable", 0.0
			if c.CacheHitRate != nil {
				rate = *c.CacheHitRate + "%"
				value, _ = strconv.ParseFloat(*c.CacheHitRate, 64)
				value /= 100
			}
			rows = append(rows, usageViewerRow{identity: c.Client + "\x00" + c.SessionID, label: c.Client + "/" + c.SessionID, value: rate, bar: value, detail: []string{"Read " + compactNumber(float64(c.CachedReadTokens)), "Write " + compactNumber(float64(c.CacheWriteTokens)), "Logical input " + compactNumber(float64(c.LogicalInputTokens)), c.DetailCommand}})
		}
		return rows
	default:
		return []usageViewerRow{
			{identity: "priced", label: "PRICED EVENTS", value: groupedInt(s.report.Coverage.PricedEvents), detail: []string{"Coverage " + s.report.Coverage.Percent + "%"}},
			{identity: "unpriced", label: "UNPRICED EVENTS", value: groupedInt(s.report.Coverage.UnpricedEvents), detail: []string{"Total events " + groupedInt(s.report.Coverage.TotalEvents)}},
			{identity: "warnings", label: "WARNINGS", value: groupedInt(int64(len(s.warnings))), detail: append([]string(nil), s.warnings...)},
		}
	}
}

// usageViewerDimensionDetail keeps the four primary fields first so that a
// short frame degrades by dropping cache accounting rather than the tokens,
// cost, sessions, and coverage a dimension always has. Cache fields appear only
// when the dimension actually carries cache accounting.
func usageViewerDimensionDetail(d usage.StatsDimension) []string {
	cost := compactCost(d.ProviderCost, d.KnownProviderCost, knownCostAvailable(d.ProviderCost, d.KnownProviderCost, d.Coverage))
	detail := []string{
		"Tokens " + compactNumber(float64(d.Tokens)),
		"Cost " + cost,
		"Sessions " + groupedInt(d.Sessions),
		"Coverage " + d.Coverage + "%",
	}
	if d.CacheHitRate != nil {
		detail = append(detail, "Cache hit rate "+*d.CacheHitRate+"%")
	}
	if d.CachedReadTokens > 0 || d.CacheWriteTokens > 0 {
		detail = append(detail, "Cache read "+compactNumber(float64(d.CachedReadTokens)), "Cache write "+compactNumber(float64(d.CacheWriteTokens)))
	}
	if d.LogicalInputTokens > 0 {
		detail = append(detail, "Logical input "+compactNumber(float64(d.LogicalInputTokens)))
	}
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
	raw, err := term.MakeRaw(int(input.Fd()))
	if err != nil {
		return fmt.Errorf("enable interactive terminal: %w", err)
	}
	defer term.Restore(int(input.Fd()), raw)
	if _, err := fmt.Fprint(output, "\x1b[?1049h\x1b[?25l"); err != nil {
		return err
	}
	defer fmt.Fprint(output, "\x1b[?25h\x1b[?1049l")
	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)
	defer signal.Stop(resized)
	p := newUsageTextPrimitives(output, noColor)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		width, height, _ = term.GetSize(int(output.Fd()))
		if err := renderUsageStatsViewer(output, max(1, width), max(1, height), viewer, p); err != nil {
			return err
		}
		key, resizedDuringRead, err := readSessionViewerKey(ctx, input, resized)
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
	if _, err := fmt.Fprintln(w, statsFit(p.style("AGENTDECK · USAGE", "1;36")+" "+p.style("INTERACTIVE · READ ONLY", "2;36"), width)); err != nil {
		return err
	}
	tabs := make([]string, len(usageViewerSections))
	for i, name := range usageViewerSections {
		if usageViewerSection(i) == s.section {
			tabs[i] = p.style("["+name+"]", "1;36")
		} else {
			tabs[i] = name
		}
	}
	if _, err := fmt.Fprintln(w, statsFit(strings.Join(tabs, " "), width)); err != nil {
		return err
	}
	from, to := compactStatsDisplayRange(s.report.Range)
	meta := output.SanitizeTerminalCell(fmt.Sprintf("%s–%s · %s · %s · %s", from, to, s.report.Timezone, strings.ToUpper(s.report.Metric), s.report.GroupBy))
	if _, err := fmt.Fprintln(w, statsFit(meta, width)); err != nil {
		return err
	}
	kpis := fmt.Sprintf("TOKENS %s   COST %s   SESSIONS %s   PRICED %s%%", compactNumber(float64(s.report.Totals.Tokens)), compactCost(s.report.Totals.ProviderCost, s.report.Totals.KnownProviderCost, true), groupedInt(s.report.Totals.Sessions), s.report.Coverage.Percent)
	if _, err := fmt.Fprintln(w, statsFit(kpis, width)); err != nil {
		return err
	}
	selected := s.selected[s.section]
	bodyBudget := max(2, height-6) // title, tabs, meta, KPIs, status, and help.
	detail := []string(nil)
	if len(s.rows) > 0 {
		detail = usageViewerDetail(s.rows[selected], width)
	}
	if width < 120 && len(detail) > bodyBudget-1 {
		detail = detail[:bodyBudget-1]
	}
	viewport := max(1, bodyBudget-len(detail))
	if width >= 120 {
		viewport = bodyBudget
		detail = detail[:min(len(detail), bodyBudget)]
	}
	start := s.viewportOffset(viewport)
	end := min(len(s.rows), start+viewport)
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		row := s.rows[i]
		prefix := "  "
		if i == selected {
			prefix = p.style("> ", "1;36")
		}
		barWidth := min(20, max(0, width-statsVisibleWidth(prefix)-statsVisibleWidth(row.label)-statsVisibleWidth(row.value)-6))
		lines = append(lines, statsFit(prefix+statsPad(row.label, min(28, max(10, width/3)))+" "+p.barTrack(scaledBar(row.bar, 1, barWidth), barWidth, "36")+" "+row.value, width))
	}
	if len(lines) == 0 {
		lines = append(lines, "No rows in this section.")
	}
	if width >= 120 && len(s.rows) > 0 {
		for _, line := range usageJoinColumns(lines, width*2/3-2, detail, width-width*2/3, 2) {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
	} else {
		for _, line := range lines {
			if _, err := fmt.Fprintln(w, line); err != nil {
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
	if len(s.warnings) > 0 {
		status += " · warning"
	}
	if _, err := fmt.Fprintln(w, statsFit(status, width)); err != nil {
		return err
	}
	help := "←/→ tabs · ↑/↓ select · pgup/pgdn page · home/end · ? help · q/esc quit"
	if width < 80 && !s.help {
		help = "? help · q quit"
	}
	_, err := fmt.Fprintln(w, statsFit(help, width))
	return err
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

func usageViewerDetail(row usageViewerRow, width int) []string {
	lines := []string{"DETAIL · " + row.label}
	for _, detail := range row.detail {
		lines = append(lines, statsWrap(detail, max(1, width))...)
	}
	return lines
}
