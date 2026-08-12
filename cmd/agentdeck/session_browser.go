package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	terminaloutput "github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/session"
	"golang.org/x/term"
)

type sessionBrowserState struct {
	items    []session.Metadata
	selected int
}

func newSessionBrowserState(items []session.Metadata) *sessionBrowserState {
	return &sessionBrowserState{items: items}
}

func (s *sessionBrowserState) apply(key string, viewport int) (open, exit bool) {
	last := max(0, len(s.items)-1)
	page := max(1, viewport)
	switch key {
	case "q", "escape":
		return false, true
	case "enter":
		return len(s.items) > 0, false
	case "up":
		s.selected = max(0, s.selected-1)
	case "down":
		s.selected = min(last, s.selected+1)
	case "home":
		s.selected = 0
	case "end":
		s.selected = last
	case "page-up":
		s.selected = max(0, s.selected-page)
	case "page-down":
		s.selected = min(last, s.selected+page)
	}
	return false, false
}

func (s *sessionBrowserState) viewport(width, height int) (start, end, size int) {
	compact := sessionBrowserLayoutFor(sessionBrowserCanvasWidth(width)).compact
	fixedRows, recordRows := 6, 1
	if compact {
		// The compact header and every compact record use two complete lines.
		fixedRows, recordRows = 7, 2
	}
	size = max(1, (height-fixedRows)/recordRows)
	start = max(0, min(s.selected-size/2, len(s.items)-size))
	end = min(len(s.items), start+size)
	return start, end, size
}

type sessionBrowserDetailLoad func(session.Metadata) sessionViewerLoad

func runSessionBrowser(
	ctx context.Context,
	input, output *os.File,
	items []session.Metadata,
	detailLoad sessionBrowserDetailLoad,
	noColor ...bool,
) error {
	if !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return errors.New("session --interactive requires TTY stdin and stdout")
	}
	if os.Getenv("TERM") == "dumb" {
		return errors.New("session --interactive requires a usable terminal")
	}

	disableColor := len(noColor) > 0 && noColor[0]
	p := newUsageTextPrimitives(output, disableColor)
	terminal, err := startInteractiveTerminal(input, output)
	if err != nil {
		return err
	}
	defer terminal.Close()

	frame := terminal.frameWriter()
	state := newSessionBrowserState(items)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		width, height := 100, 24
		if columns, rows, sizeErr := term.GetSize(int(output.Fd())); sizeErr == nil && columns > 0 && rows > 0 {
			width, height = columns, rows
		}
		if err := renderSessionBrowser(frame, width, height, state, p); err != nil {
			return err
		}
		_, _, viewport := state.viewport(width, height)
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
		open, exit := state.apply(key, viewport)
		if exit {
			return nil
		}
		if !open {
			continue
		}
		exitKey, err := runSessionViewerScreen(ctx, input, output, frame, terminal.resized, detailLoad(state.items[state.selected]), p)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if exitKey == "q" {
			return nil
		}
	}
}

func renderSessionBrowser(w io.Writer, width, height int, state *sessionBrowserState, primitives ...usageTextPrimitives) error {
	p := usageTextPrimitives{}
	if len(primitives) > 0 {
		p = primitives[0]
	}
	lines := []string{"\x1b[H\x1b[2J"}
	if width < 48 || height < 10 {
		lines = append(lines,
			p.style("AGENTDECK · SESSIONS", usageColorBrand),
			"Terminal too small for session browser.",
			"Resize to at least 48x10.",
			"q/esc quit",
		)
		return sessionViewerWriteFrame(w, lines[:min(len(lines), max(1, height+1))], width)
	}

	lines = append(lines,
		p.style("AGENTDECK · SESSIONS", usageColorBrand)+" · "+p.style("INTERACTIVE", usageColorSession)+" · "+p.style("READ ONLY", usageColorSuccess),
		p.style(fmt.Sprintf("%d INDEXED SESSIONS", len(state.items)), usageColorSession)+" · times in "+displayZoneName(),
	)
	if len(state.items) == 0 {
		lines = append(lines,
			p.style("No indexed sessions.", usageColorWarning),
			"Run: agentdeck session scan",
			"q/esc quit",
		)
		return sessionViewerWriteFrame(w, lines, width)
	}

	canvasWidth := sessionBrowserCanvasWidth(width)
	lines = append(lines, sessionBrowserHeader(canvasWidth, p)...)
	start, end, _ := state.viewport(width, height)
	for index := start; index < end; index++ {
		lines = append(lines, sessionBrowserStyledRows(state.items[index], index == state.selected, canvasWidth, p)...)
	}
	selected := state.items[state.selected]
	lines = append(lines,
		sessionBrowserPreview(selected, canvasWidth, p),
		statsFit(p.style(fmt.Sprintf("row %d/%d", state.selected+1, len(state.items)), usageColorInfo), canvasWidth),
		statsFit("↑/↓ select · pgup/pgdn page · home/end · enter open · q/esc quit", canvasWidth),
	)
	if len(lines) > height+1 {
		lines = lines[:height+1]
	}
	return sessionViewerWriteFrame(w, lines, width)
}

func sessionBrowserCanvasWidth(width int) int {
	return max(1, min(width, 120))
}

type sessionBrowserLayout struct {
	client  int
	session int
	model   int
	project int
	last    int
	compact bool
}

func sessionBrowserLayoutFor(width int) sessionBrowserLayout {
	available := max(1, width-2)
	if width >= 118 {
		layout := sessionBrowserLayout{client: 7, session: 34, model: 24, last: 18}
		layout.project = max(14, available-layout.client-layout.session-layout.model-layout.last-4)
		return layout
	}
	if width >= 80 {
		layout := sessionBrowserLayout{client: 7, session: 18, model: 13, last: 16}
		layout.project = max(10, available-layout.client-layout.session-layout.model-layout.last-4)
		return layout
	}
	return sessionBrowserLayout{compact: true}
}

func sessionBrowserHeader(width int, p usageTextPrimitives) []string {
	layout := sessionBrowserLayoutFor(width)
	if layout.compact {
		return []string{
			"  " + p.style("CLIENT / SESSION", usageColorBrand),
			"  " + p.style("MODEL · PROJECT · LAST ACTIVITY", usageColorBrand),
		}
	}
	return []string{"  " + sessionBrowserColumns(
		[]string{"CLIENT", "SESSION", "MODEL", "PROJECT", "LAST ACTIVITY"},
		layout,
		[]string{usageColorBrand, usageColorBrand, usageColorBrand, usageColorBrand, usageColorBrand},
		p,
	)}
}

func sessionBrowserStyledRows(item session.Metadata, selected bool, width int, p usageTextPrimitives) []string {
	prefix := "  "
	if selected {
		prefix = p.style("> ", usageColorBrand)
	}
	layout := sessionBrowserLayoutFor(width)
	client := sessionViewerKnown(item.Client)
	id := sessionViewerKnown(item.SessionID)
	model := sessionViewerKnown(item.Model)
	project := sessionProjectLabel(item.Project)
	last := sessionViewerKnown(renderSessionDocumentTime(item.LastAt))
	if layout.compact {
		available := max(1, width-2)
		identity := p.style(statsFit(client+"/"+id, available), sessionClientColor(client))
		second := sessionBrowserCompactMetadata(client, model, project, last, available, p)
		return []string{prefix + statsFit(identity, available), "  " + second}
	}
	return []string{prefix + sessionBrowserColumns(
		[]string{client, id, model, project, last},
		layout,
		[]string{sessionClientColor(client), usageColorBrand, sessionClientColor(client), usageColorSuccess, usageColorInfo},
		p,
	)}
}

func sessionBrowserCompactMetadata(client, model, project, last string, width int, p usageTextPrimitives) string {
	separator := " · "
	fixed := statsVisibleWidth("MODEL ") + statsVisibleWidth("PROJECT ") + statsVisibleWidth("LAST ") + 2*statsVisibleWidth(separator)
	values := max(3, width-fixed)
	modelWidth := max(1, values/4)
	projectWidth := max(1, values/3)
	lastWidth := max(1, values-modelWidth-projectWidth)
	line := p.style("MODEL", usageColorBrand) + " " + p.style(statsFit(model, modelWidth), sessionClientColor(client)) + separator +
		p.style("PROJECT", usageColorBrand) + " " + p.style(statsFit(project, projectWidth), usageColorSuccess) + separator +
		p.style("LAST", usageColorBrand) + " " + p.style(statsFit(last, lastWidth), usageColorInfo)
	return statsFit(line, width)
}

func sessionBrowserColumns(values []string, layout sessionBrowserLayout, colors []string, p usageTextPrimitives) string {
	widths := []int{layout.client, layout.session, layout.model, layout.project, layout.last}
	columns := make([]string, 0, len(values))
	for index, value := range values {
		columns = append(columns, statsPad(p.style(statsFit(value, widths[index]), colors[index]), widths[index]))
	}
	return strings.TrimRight(strings.Join(columns, " "), " ")
}

func sessionBrowserPreview(item session.Metadata, width int, p usageTextPrimitives) string {
	client := sessionViewerKnown(item.Client)
	model := sessionViewerKnown(item.Model)
	project := sessionProjectLabel(item.Project)
	last := sessionViewerKnown(renderSessionDocumentTime(item.LastAt))
	if sessionBrowserLayoutFor(width).compact {
		separator := " · "
		fixed := statsVisibleWidth("MODEL ") + statsVisibleWidth(separator) + statsVisibleWidth("PROJECT ")
		valueBudget := max(2, width-fixed)
		projectBudget := max(1, valueBudget*3/5)
		modelBudget := max(1, valueBudget-projectBudget)
		line := p.style("MODEL", usageColorBrand) + " " + p.style(statsFit(model, modelBudget), sessionClientColor(client)) + separator +
			p.style("PROJECT", usageColorBrand) + " " + p.style(statsFit(project, projectBudget), usageColorSuccess)
		return statsFit(line, width)
	}
	line := p.style("SELECTED", usageColorBrand) + " · " +
		p.style(client, sessionClientColor(client)) + " · MODEL " + p.style(model, sessionClientColor(client)) +
		" · PROJECT " + p.style(project, usageColorSuccess) + " · LAST " + p.style(last, usageColorInfo)
	return statsFit(line, width)
}

// sessionBrowserRow remains the plain, deterministic row adapter used by unit
// tests and non-color geometry checks.
func sessionBrowserRow(item session.Metadata, width int) string {
	layout := sessionBrowserLayoutFor(width + 2)
	client := sessionViewerKnown(terminaloutput.SanitizeTerminalCell(item.Client))
	id := sessionViewerKnown(terminaloutput.SanitizeTerminalCell(item.SessionID))
	model := sessionViewerKnown(terminaloutput.SanitizeTerminalCell(item.Model))
	project := sessionProjectLabel(item.Project)
	last := sessionViewerKnown(renderSessionDocumentTime(item.LastAt))
	if layout.compact {
		identity := strings.TrimSpace(client + "/" + id)
		lastWidth := min(18, max(12, width/3))
		return statsFit(statsPad(statsFit(identity, max(1, width-lastWidth-1)), max(1, width-lastWidth-1))+" "+statsPadLeft(statsFit(last, lastWidth), lastWidth), width)
	}
	return sessionBrowserColumns([]string{client, id, model, project, last}, layout, []string{"", "", "", "", ""}, usageTextPrimitives{})
}
