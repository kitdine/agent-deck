package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/session"
)

func TestSessionBrowserStateNavigationAndEntry(t *testing.T) {
	items := make([]session.Metadata, 30)
	state := newSessionBrowserState(items)
	if open, exit := state.apply("enter", 10); !open || exit {
		t.Fatalf("enter = open %v, exit %v", open, exit)
	}
	state.apply("page-down", 10)
	if state.selected != 10 {
		t.Fatalf("page down selection = %d, want 10", state.selected)
	}
	state.apply("end", 10)
	if state.selected != 29 {
		t.Fatalf("end selection = %d, want 29", state.selected)
	}
	state.apply("page-up", 10)
	if state.selected != 19 {
		t.Fatalf("page up selection = %d, want 19", state.selected)
	}
	state.apply("home", 10)
	if state.selected != 0 {
		t.Fatalf("home selection = %d, want 0", state.selected)
	}
	if _, exit := state.apply("escape", 10); !exit {
		t.Fatal("escape did not exit the list")
	}
	if _, exit := state.apply("q", 10); !exit {
		t.Fatal("q did not exit the list")
	}
}

func TestRenderSessionBrowserEmptyStateAndPrivacy(t *testing.T) {
	var empty bytes.Buffer
	if err := renderSessionBrowser(&empty, 80, 24, newSessionBrowserState(nil)); err != nil {
		t.Fatal(err)
	}
	if text := empty.String(); !strings.Contains(text, "No indexed sessions.") || !strings.Contains(text, "agentdeck session scan") {
		t.Fatalf("empty browser did not include recovery hint: %q", text)
	}

	state := newSessionBrowserState([]session.Metadata{{
		Client:     "codex",
		SessionID:  "session-1",
		Project:    "/private/project/path",
		Model:      "gpt-5",
		LastAt:     "2026-08-10T12:00:00Z",
		SourcePath: "/private/secret/session.jsonl",
	}})
	var rendered bytes.Buffer
	if err := renderSessionBrowser(&rendered, 140, 24, state); err != nil {
		t.Fatal(err)
	}
	text := rendered.String()
	if !strings.Contains(text, "session-1") || strings.Contains(text, state.items[0].SourcePath) || strings.Contains(text, state.items[0].Project) || strings.Contains(text, "/private/") {
		t.Fatalf("browser row identity/privacy mismatch: %q", text)
	}
	for _, line := range strings.Split(text, "\n") {
		if statsVisibleWidth(line) > 140 {
			t.Fatalf("browser line width = %d, want <= 140: %q", statsVisibleWidth(line), line)
		}
	}
}

func TestRenderSessionBrowserTooSmallFrameFits(t *testing.T) {
	state := newSessionBrowserState([]session.Metadata{{Client: "codex", SessionID: "session-1"}})
	var rendered bytes.Buffer
	if err := renderSessionBrowser(&rendered, 20, 4, state); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(rendered.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("too-small rows = %d, want 4", len(lines))
	}
	for _, line := range lines {
		if statsVisibleWidth(line) > 20 {
			t.Fatalf("too-small line width = %d, want <= 20: %q", statsVisibleWidth(line), line)
		}
	}
}

func TestSessionInteractiveRootRouteRejectsUnsupportedOutputBeforeViewer(t *testing.T) {
	stateDir := t.TempDir()
	if _, err := runSessionCommand(stateDir, "--format", "json", "session", "--interactive"); err == nil || !strings.Contains(err.Error(), "requires text format") {
		t.Fatalf("JSON route error = %v, want text-format rejection", err)
	}
	if _, err := runSessionCommand(stateDir, "session", "--interactive"); err == nil || !strings.Contains(err.Error(), "requires TTY stdin and stdout") {
		t.Fatalf("non-TTY route error = %v, want TTY rejection", err)
	}
}
