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

func (s *sessionBrowserState) viewport(height int) (start, end, size int) {
	size = max(1, height-5)
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
) error {
	if !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return errors.New("session --interactive requires TTY stdin and stdout")
	}
	if os.Getenv("TERM") == "dumb" {
		return errors.New("session --interactive requires a usable terminal")
	}
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
		if err := renderSessionBrowser(frame, width, height, state); err != nil {
			return err
		}
		_, _, viewport := state.viewport(height)
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
		exitKey, err := runSessionViewerScreen(ctx, input, output, frame, terminal.resized, detailLoad(state.items[state.selected]))
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

func renderSessionBrowser(w io.Writer, width, height int, state *sessionBrowserState) error {
	if _, err := io.WriteString(w, "\x1b[H\x1b[2J"); err != nil {
		return err
	}
	if width < 48 || height < 10 {
		lines := []string{
			"AGENTDECK · SESSIONS",
			"Terminal too small for the session browser.",
			"Resize to at least 48x10.",
			"q/esc quit",
		}
		for _, line := range lines[:min(len(lines), max(1, height))] {
			if _, err := fmt.Fprintln(w, sessionShowFit(line, max(1, width))); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := fmt.Fprintln(w, "AGENTDECK · SESSIONS  INTERACTIVE · READ ONLY"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%d indexed sessions · times in %s\n", len(state.items), displayZoneName()); err != nil {
		return err
	}
	if len(state.items) == 0 {
		if _, err := fmt.Fprintln(w, "\nNo indexed sessions."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Run: agentdeck session scan"); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, "\nq/esc quit")
		return err
	}
	start, end, _ := state.viewport(height)
	for index := start; index < end; index++ {
		item := state.items[index]
		prefix := "  "
		if index == state.selected {
			prefix = "> "
		}
		line := sessionBrowserRow(item, width-2)
		if _, err := fmt.Fprintln(w, prefix+sessionShowFit(line, max(1, width-2))); err != nil {
			return err
		}
	}
	status := fmt.Sprintf("row %d/%d", state.selected+1, len(state.items))
	if _, err := fmt.Fprintln(w, sessionShowFit(status, width)); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, sessionShowFit("↑/↓ select · pgup/pgdn page · home/end · enter open · q/esc quit", width))
	return err
}

func sessionBrowserRow(item session.Metadata, width int) string {
	client := terminaloutput.SanitizeTerminalCell(item.Client)
	id := terminaloutput.SanitizeTerminalCell(item.SessionID)
	model := terminaloutput.SanitizeTerminalCell(item.Model)
	lastAt := renderSessionDocumentTime(item.LastAt)
	switch {
	case width >= 118:
		return fmt.Sprintf("%-7s %-40s %-28s %s", client, id, model, lastAt)
	case width >= 78:
		return fmt.Sprintf("%-7s %-30s %-14s %s", client, id, model, lastAt)
	default:
		identity := strings.TrimSpace(client + "/" + id)
		return fmt.Sprintf("%-34s %s", identity, lastAt)
	}
}
