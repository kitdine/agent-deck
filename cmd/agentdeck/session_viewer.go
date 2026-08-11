package main

import (
	"context"
	"fmt"
	"io"
	"strings"
)

const sessionViewerPageLimit = 20

type sessionViewerSection int

const (
	viewerOverview sessionViewerSection = iota
	viewerDocuments
	viewerActivity
	viewerTokens
	sessionViewerSectionCount
)

var sessionViewerSections = [sessionViewerSectionCount]string{"OVERVIEW", "DOCUMENTS", "ACTIVITY", "TOKENS"}

// sessionViewerRow keeps list identity separate from the selected-row detail.
// Detail is built only from the approved, bounded page loaded for this section.
type sessionViewerRow struct {
	Identity   string
	Label      string
	Value      string
	Detail     []string
	Footer     string
	LabelColor string
	ValueColor string
}

type sessionViewerPage struct {
	Rows    []sessionViewerRow
	Summary []string
	Empty   string

	// Lines is retained as an internal compatibility adapter for terminal tests
	// and callers that provide synthetic pages. Production loaders use Rows.
	Lines []string

	Page    int
	Total   int
	Warning string
	Stale   bool
	Partial bool
}

type sessionViewerLoad func(context.Context, sessionViewerSection, int, int) (sessionViewerPage, error)

// sessionViewerState is terminal-independent. Page, selection, and viewport
// are all section-local so switching tabs never destroys the user's place.
type sessionViewerState struct {
	section   sessionViewerSection
	selected  [sessionViewerSectionCount]int
	pages     [sessionViewerSectionCount]int
	viewports [sessionViewerSectionCount]int
	current   sessionViewerPage
	load      sessionViewerLoad
}

func newSessionViewerState(load sessionViewerLoad) *sessionViewerState {
	state := &sessionViewerState{load: load}
	for index := range state.pages {
		state.pages[index] = 1
	}
	return state
}

func (s *sessionViewerState) refresh(ctx context.Context) error {
	page, err := s.load(ctx, s.section, s.pages[s.section], sessionViewerPageLimit)
	if err != nil {
		return err
	}
	if page.Page < 1 {
		page.Page = s.pages[s.section]
	}
	if len(page.Rows) == 0 && len(page.Lines) > 0 {
		page.Rows = make([]sessionViewerRow, 0, len(page.Lines))
		for index, line := range page.Lines {
			page.Rows = append(page.Rows, sessionViewerRow{
				Identity: fmt.Sprintf("row-%d", index+1),
				Label:    line,
			})
		}
	}
	s.current = page
	section := s.section
	if s.selected[section] >= len(page.Rows) {
		s.selected[section] = max(0, len(page.Rows)-1)
	}
	if s.viewports[section] > s.selected[section] {
		s.viewports[section] = s.selected[section]
	}
	return nil
}

// apply updates only viewer state. The caller reloads only when true is
// returned, preserving bounded lazy acquisition for each independent section.
func (s *sessionViewerState) apply(key string) (reload, exit bool) {
	switch key {
	case "q", "escape":
		return false, true
	case "left", "shift-tab":
		s.section = sessionViewerSection((int(s.section) + len(sessionViewerSections) - 1) % len(sessionViewerSections))
		return true, false
	case "right", "tab":
		s.section = sessionViewerSection((int(s.section) + 1) % len(sessionViewerSections))
		return true, false
	case "up":
		s.selected[s.section] = max(0, s.selected[s.section]-1)
	case "down":
		s.selected[s.section] = min(max(0, len(s.current.Rows)-1), s.selected[s.section]+1)
	case "home":
		s.selected[s.section] = 0
	case "end":
		s.selected[s.section] = max(0, len(s.current.Rows)-1)
	case "page-up":
		if s.pages[s.section] > 1 {
			s.pages[s.section]--
			s.selected[s.section] = 0
			s.viewports[s.section] = 0
			return true, false
		}
	case "page-down":
		if s.current.Page*sessionViewerPageLimit < s.current.Total {
			s.pages[s.section]++
			s.selected[s.section] = 0
			s.viewports[s.section] = 0
			return true, false
		}
	}
	return false, false
}

func (s *sessionViewerState) viewport(size int) (start, end int) {
	size = max(1, size)
	section := s.section
	selected := s.selected[section]
	start = min(s.viewports[section], max(0, len(s.current.Rows)-size))
	if selected < start {
		start = selected
	}
	if selected >= start+size {
		start = selected - size + 1
	}
	start = max(0, min(start, max(0, len(s.current.Rows)-size)))
	end = min(len(s.current.Rows), start+size)
	s.viewports[section] = start
	return start, end
}

func renderSessionViewer(w io.Writer, width, height int, state *sessionViewerState, primitives ...usageTextPrimitives) error {
	p := usageTextPrimitives{}
	if len(primitives) > 0 {
		p = primitives[0]
	}

	lines := []string{"\x1b[H\x1b[2J"}
	if width < 48 || height < 10 {
		lines = append(lines,
			p.style("AGENTDECK · SESSION", usageColorBrand),
			"Terminal too small for session detail.",
			"Resize to at least 48x10.",
			"q/esc back",
		)
		return sessionViewerWriteFrame(w, lines[:min(len(lines), max(1, height+1))], width)
	}

	lines = append(lines,
		p.style("AGENTDECK · SESSION", usageColorBrand)+" · "+p.style("READ ONLY", usageColorSuccess),
		sessionViewerTabLine(state.section, width, p),
	)

	advisories := sessionViewerAdvisories(state.current, p)
	advisoryLimit := 1
	if height >= 16 {
		advisoryLimit = 2
	}
	for _, advisory := range advisories[:min(len(advisories), advisoryLimit)] {
		lines = append(lines, statsFit(advisory, width))
	}

	bodyBudget := max(1, height-len(lines)-2)
	lines = append(lines, sessionViewerBody(state, width, bodyBudget, p)...)
	lines = append(lines,
		sessionViewerStatus(state, width, p),
		statsFit("←/→ tab · ↑/↓ select · pgup/pgdn page · home/end · q/esc back", width),
	)
	if len(lines) > height+1 {
		lines = lines[:height+1]
	}
	return sessionViewerWriteFrame(w, lines, width)
}

func sessionViewerWriteFrame(w io.Writer, lines []string, width int) error {
	for index, line := range lines {
		if index == 0 && strings.HasPrefix(line, "\x1b[") {
			if _, err := io.WriteString(w, line); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintln(w, statsFit(line, max(1, width))); err != nil {
			return err
		}
	}
	return nil
}

func sessionViewerTabLine(active sessionViewerSection, width int, p usageTextPrimitives) string {
	tabs := make([]string, len(sessionViewerSections))
	for index, name := range sessionViewerSections {
		if sessionViewerSection(index) == active {
			tabs[index] = p.style("["+name+"]", usageColorBrand)
		} else {
			tabs[index] = name
		}
	}
	return statsFit(strings.Join(tabs, "  "), width)
}

func sessionViewerAdvisories(page sessionViewerPage, p usageTextPrimitives) []string {
	lines := make([]string, 0, 2+len(page.Summary))
	if page.Warning != "" {
		lines = append(lines, p.style("WARNING · "+page.Warning, usageColorWarning))
	}
	for _, summary := range page.Summary {
		if strings.TrimSpace(summary) != "" {
			lines = append(lines, p.style(summary, usageColorInfo))
		}
	}
	return lines
}

func sessionViewerBody(state *sessionViewerState, width, budget int, p usageTextPrimitives) []string {
	page := state.current
	if len(page.Rows) == 0 {
		empty := page.Empty
		if empty == "" {
			empty = "No rows in this section."
		}
		return []string{p.style(empty, usageColorWarning)}
	}

	selected := min(state.selected[state.section], len(page.Rows)-1)
	row := page.Rows[selected]
	detailWidth := width
	detail := sessionViewerDetail(row, detailWidth, p)
	useSplit := width >= 112 && budget >= 12 && len(detail) > max(4, budget/2)
	if useSplit {
		gap := 3
		leftWidth := max(38, width*46/100)
		rightWidth := max(1, width-leftWidth-gap)
		detail = sessionViewerDetail(row, rightWidth, p)
		start, end := state.viewport(budget)
		left := make([]string, 0, end-start)
		for index := start; index < end; index++ {
			left = append(left, sessionViewerRowLine(page.Rows[index], index == selected, leftWidth, p))
		}
		return usageJoinColumns(left, leftWidth, detail[:min(len(detail), budget)], rightWidth, gap)
	}

	if budget < 4 || len(detail) == 0 {
		start, end := state.viewport(budget)
		lines := make([]string, 0, end-start)
		for index := start; index < end; index++ {
			lines = append(lines, sessionViewerRowLine(page.Rows[index], index == selected, width, p))
		}
		return lines
	}

	listBudget := max(1, budget/2)
	start, end := state.viewport(listBudget)
	lines := make([]string, 0, budget)
	for index := start; index < end; index++ {
		lines = append(lines, sessionViewerRowLine(page.Rows[index], index == selected, width, p))
	}
	remaining := max(0, budget-len(lines))
	lines = append(lines, detail[:min(len(detail), remaining)]...)
	return lines
}

func sessionViewerRowLine(row sessionViewerRow, selected bool, width int, p usageTextPrimitives) string {
	prefix := "  "
	if selected {
		prefix = p.style("> ", usageColorBrand)
	}
	labelColor := row.LabelColor
	if labelColor == "" {
		labelColor = usageColorInfo
	}
	valueColor := row.ValueColor
	if valueColor == "" {
		valueColor = usageColorSuccess
	}
	label := p.style(row.Label, labelColor)
	value := p.style(row.Value, valueColor)
	available := max(1, width-2)
	if row.Value == "" || width < 58 {
		line := label
		if row.Value != "" {
			line += " · " + value
		}
		return prefix + statsFit(line, available)
	}
	valueWidth := min(max(12, statsVisibleWidth(row.Value)), max(12, available/2))
	labelWidth := max(1, available-valueWidth-1)
	line := statsPad(statsFit(label, labelWidth), labelWidth) + " " + statsPadLeft(statsFit(value, valueWidth), valueWidth)
	return prefix + statsFit(line, available)
}

func sessionViewerDetail(row sessionViewerRow, width int, p usageTextPrimitives) []string {
	if len(row.Detail) == 0 && row.Footer == "" {
		return nil
	}
	lines := []string{p.style("DETAIL · "+row.Label, usageColorBrand)}
	for _, value := range row.Detail {
		wrapped := statsWrap(value, max(1, width))
		lines = append(lines, wrapped...)
	}
	if row.Footer != "" {
		lines = append(lines, p.style(row.Footer, usageColorInfo))
	}
	return lines
}

func sessionViewerStatus(state *sessionViewerState, width int, p usageTextPrimitives) string {
	page := state.current
	pageCount := max(1, (page.Total+sessionViewerPageLimit-1)/sessionViewerPageLimit)
	selected := 0
	if len(page.Rows) > 0 {
		selected = state.selected[state.section] + 1
	}
	status := fmt.Sprintf("page %d/%d · %d total · selected %d/%d", page.Page, pageCount, page.Total, selected, len(page.Rows))
	statusColor := usageColorInfo
	states := make([]string, 0, 3)
	if len(page.Rows) == 0 {
		states = append(states, "empty")
	}
	if page.Stale {
		states = append(states, "stale")
		statusColor = usageColorWarning
	}
	if page.Partial {
		states = append(states, "partial")
		statusColor = usageColorWarning
	}
	if page.Warning != "" {
		states = append(states, "warning")
		statusColor = usageColorWarning
	}
	if len(states) == 0 {
		states = append(states, "complete")
	}
	status += " · " + strings.Join(states, "/")
	return statsFit(p.style(status, statusColor), width)
}
