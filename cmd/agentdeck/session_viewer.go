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
)

var sessionViewerSections = []string{"OVERVIEW", "DOCUMENTS", "ACTIVITY", "TOKENS"}

type sessionViewerPage struct {
	Lines   []string
	Page    int
	Total   int
	Warning string
	Stale   bool
	Partial bool
}

type sessionViewerLoad func(context.Context, sessionViewerSection, int, int) (sessionViewerPage, error)

// sessionViewerState is terminal-independent so controls and independent
// section pages are testable without a pseudo-terminal dependency.
type sessionViewerState struct {
	section  sessionViewerSection
	selected int
	pages    [4]int
	current  sessionViewerPage
	load     sessionViewerLoad
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
	s.current = page
	if s.selected >= len(page.Lines) {
		s.selected = max(0, len(page.Lines)-1)
	}
	return nil
}

// apply updates only viewer state. The caller reloads only when it returns
// true, preserving lazy independent page acquisition.
func (s *sessionViewerState) apply(key string) (reload, exit bool) {
	switch key {
	case "q", "escape":
		return false, true
	case "left", "shift-tab":
		s.section = sessionViewerSection((int(s.section) + len(sessionViewerSections) - 1) % len(sessionViewerSections))
		s.selected = 0
		return true, false
	case "right", "tab":
		s.section = sessionViewerSection((int(s.section) + 1) % len(sessionViewerSections))
		s.selected = 0
		return true, false
	case "up":
		s.selected = max(0, s.selected-1)
	case "down":
		s.selected = min(max(0, len(s.current.Lines)-1), s.selected+1)
	case "home":
		s.selected = 0
	case "end":
		s.selected = max(0, len(s.current.Lines)-1)
	case "page-up":
		if s.pages[s.section] > 1 {
			s.pages[s.section]--
			s.selected = 0
			return true, false
		}
	case "page-down":
		if s.current.Page*sessionViewerPageLimit < s.current.Total {
			s.pages[s.section]++
			s.selected = 0
			return true, false
		}
	}
	return false, false
}

func renderSessionViewer(w io.Writer, width, height int, state *sessionViewerState) error {
	if _, err := io.WriteString(w, "\x1b[H\x1b[2J"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "SESSION VIEWER  %s\n\n", strings.Join(sessionViewerSections, " | ")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "SECTION: %s\n\n", sessionViewerSections[state.section]); err != nil {
		return err
	}
	viewport := max(1, height-7)
	start := max(0, min(state.selected-viewport/2, len(state.current.Lines)-viewport))
	end := min(len(state.current.Lines), start+viewport)
	for index := start; index < end; index++ {
		line := state.current.Lines[index]
		prefix := "  "
		if index == state.selected {
			prefix = "> "
		}
		if _, err := fmt.Fprintf(w, "%s%s\n", prefix, sessionShowFit(line, max(1, width-2))); err != nil {
			return err
		}
	}
	if len(state.current.Lines) == 0 {
		if _, err := fmt.Fprintln(w, "No rows."); err != nil {
			return err
		}
	}
	status := fmt.Sprintf("page %d · %d rows · row %d/%d", state.current.Page, state.current.Total, state.selected+1, len(state.current.Lines))
	if state.current.Stale {
		status += " · stale"
	}
	if state.current.Partial {
		status += " · partial"
	}
	if state.current.Warning != "" {
		status += " · " + state.current.Warning
	}
	_, err := fmt.Fprintf(w, "\n%s\n←/→ tab shift-tab section · ↑/↓ select · pgup/pgdn page · home/end · q/esc quit\n", sessionShowFit(status, width))
	return err
}
