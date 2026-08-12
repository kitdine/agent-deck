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
	Detail     terminalDetailModel
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
	section        sessionViewerSection
	selected       [sessionViewerSectionCount]int
	pages          [sessionViewerSectionCount]int
	viewports      [sessionViewerSectionCount]int
	limits         [sessionViewerSectionCount]int
	ordinals       [sessionViewerSectionCount]int
	anchors        [sessionViewerSectionCount]string
	current        sessionViewerPage
	currentSection sessionViewerSection
	loaded         bool
	load           sessionViewerLoad
}

func newSessionViewerState(load sessionViewerLoad) *sessionViewerState {
	state := &sessionViewerState{load: load}
	for index := range state.pages {
		state.pages[index] = 1
	}
	return state
}

func (s *sessionViewerState) refresh(ctx context.Context) error {
	section := s.section
	limit := s.limit(section)
	page, err := s.load(ctx, section, s.pages[section], limit)
	if err != nil {
		return err
	}
	s.installPage(page, limit, s.selected[section], "")
	return nil
}

// reflow reloads the active section at most once for a new acquisition
// capacity. The selected stable identity wins; its absolute ordinal is the
// deterministic fallback when the source changed between loads.
func (s *sessionViewerState) reflow(ctx context.Context, limit int) error {
	section := s.section
	limit = max(1, limit)
	if s.loaded && s.currentSection == section && s.pages[section] == s.current.Page {
		s.rememberSelection()
	}
	absolute := s.ordinals[section]
	anchor := s.anchors[section]
	pageNumber := absolute/limit + 1
	page, err := s.load(ctx, section, pageNumber, limit)
	if err != nil {
		return err
	}
	selected := max(0, absolute-(max(1, page.Page)-1)*limit)
	s.installPage(page, limit, selected, anchor)
	return nil
}

func (s *sessionViewerState) installPage(page sessionViewerPage, limit, selected int, anchor string) {
	section := s.section
	if page.Page < 1 {
		page.Page = max(1, s.pages[section])
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
	if anchor != "" {
		for index := range page.Rows {
			if page.Rows[index].Identity == anchor {
				selected = index
				break
			}
		}
	}
	s.current = page
	s.currentSection = section
	s.loaded = true
	s.limits[section] = max(1, limit)
	s.pages[section] = page.Page
	s.selected[section] = min(max(0, selected), max(0, len(page.Rows)-1))
	if s.viewports[section] > s.selected[section] {
		s.viewports[section] = s.selected[section]
	}
	s.rememberSelection()
}

func (s *sessionViewerState) limit(section sessionViewerSection) int {
	if s.limits[section] > 0 {
		return s.limits[section]
	}
	return sessionViewerPageLimit
}

func (s *sessionViewerState) rememberSelection() {
	section := s.section
	if !s.loaded || s.currentSection != section || len(s.current.Rows) == 0 {
		return
	}
	selected := min(max(0, s.selected[section]), len(s.current.Rows)-1)
	s.ordinals[section] = max(0, (max(1, s.current.Page)-1)*s.limit(section)+selected)
	s.anchors[section] = s.current.Rows[selected].Identity
}

// apply updates only viewer state. The caller reloads only when true is
// returned, preserving bounded lazy acquisition for each independent section.
func (s *sessionViewerState) apply(key string) (reload, exit bool) {
	switch key {
	case "q", "escape":
		return false, true
	case "left", "shift-tab":
		s.rememberSelection()
		s.section = sessionViewerSection((int(s.section) + len(sessionViewerSections) - 1) % len(sessionViewerSections))
		return true, false
	case "right", "tab":
		s.rememberSelection()
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
			s.ordinals[s.section] = (s.pages[s.section] - 1) * s.limit(s.section)
			s.anchors[s.section] = ""
			return true, false
		}
	case "page-down":
		if s.current.Page*s.limit(s.section) < s.current.Total {
			s.pages[s.section]++
			s.selected[s.section] = 0
			s.viewports[s.section] = 0
			s.ordinals[s.section] = (s.pages[s.section] - 1) * s.limit(s.section)
			s.anchors[s.section] = ""
			return true, false
		}
	}
	s.rememberSelection()
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
		sessionViewerTabLine(state.section, sessionViewerCanvasWidth(width), p),
	)
	canvasWidth := sessionViewerCanvasWidth(width)

	advisories := sessionViewerAdvisories(state.current, p)
	advisoryLimit := 1
	if height >= 16 {
		advisoryLimit = 2
	}
	for _, advisory := range advisories[:min(len(advisories), advisoryLimit)] {
		lines = append(lines, statsFit(advisory, canvasWidth))
	}

	// lines[0] is clear/home control with no visual row.
	bodyBudget := max(1, height-(len(lines)-1)-2)
	lines = append(lines, sessionViewerBody(state, canvasWidth, bodyBudget, p)...)
	lines = append(lines,
		sessionViewerStatus(state, canvasWidth, p),
		statsFit("←/→ tab · ↑/↓ select · pgup/pgdn page · home/end · q/esc back", canvasWidth),
	)
	if len(lines) > height+1 {
		lines = lines[:height+1]
	}
	return sessionViewerWriteFrame(w, lines, width)
}

func sessionViewerCanvasWidth(width int) int {
	return max(1, min(width, 120))
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

	detailBudget := min(len(detail), max(0, budget-1))
	listBudget := max(1, budget-detailBudget)
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
	detail := row.Detail
	detail.title = row.Label
	return renderTerminalDetailModel(detail, width, p)
}

func sessionViewerStatus(state *sessionViewerState, width int, p usageTextPrimitives) string {
	page := state.current
	limit := state.limit(state.section)
	pageCount := max(1, (page.Total+limit-1)/limit)
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
