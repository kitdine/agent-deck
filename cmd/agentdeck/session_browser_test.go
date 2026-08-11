package main

import (
	"bytes"
	"regexp"
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
		t.Fatal("escape did not exit list")
	}
	if _, exit := state.apply("q", 10); !exit {
		t.Fatal("q did not exit list")
	}
}

func TestRenderSessionBrowserShowsHeadersUnknownModelProjectAndPrivacy(t *testing.T) {
	state := newSessionBrowserState([]session.Metadata{{
		Client:     "claude",
		SessionID:  "session-1",
		Project:    "/private/project/agent-deck",
		LastAt:     "2026-08-10T12:00:00Z",
		SourcePath: "/private/secret/session.jsonl",
	}})
	var rendered bytes.Buffer
	if err := renderSessionBrowser(&rendered, 140, 24, state, usageTextPrimitives{color: true}); err != nil {
		t.Fatal(err)
	}
	text := rendered.String()
	for _, want := range []string{"CLIENT", "SESSION", "MODEL", "PROJECT", "LAST ACTIVITY", "session-1", "unknown", "agent-deck", "SELECTED", "\x1b[1;96m", "\x1b[1;95m"} {
		if !strings.Contains(text, want) {
			t.Fatalf("browser missing %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{state.items[0].SourcePath, state.items[0].Project, "/private/"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("browser exposed private path %q:\n%s", forbidden, text)
		}
	}
}

func TestRenderSessionBrowserEmptyStateAndNoColorRecovery(t *testing.T) {
	var empty bytes.Buffer
	if err := renderSessionBrowser(&empty, 80, 24, newSessionBrowserState(nil), usageTextPrimitives{}); err != nil {
		t.Fatal(err)
	}
	text := empty.String()
	if !strings.Contains(text, "No indexed sessions.") || !strings.Contains(text, "agentdeck session scan") {
		t.Fatalf("empty browser did not include recovery hint: %q", text)
	}
	if regexp.MustCompile(`\x1b\[[0-9;]+m`).MatchString(text) {
		t.Fatalf("no-color empty browser contains SGR: %q", text)
	}
}

func TestRenderSessionBrowserCompactPreviewBudgetsModelAndProject(t *testing.T) {
	state := newSessionBrowserState([]session.Metadata{{
		Client: "claude", SessionID: "session-1", Model: strings.Repeat("claude-very-long-model-", 3), Project: "/private/work/agent-deck",
		LastAt: "2026-08-10T12:00:00Z",
	}})
	var rendered bytes.Buffer
	if err := renderSessionBrowser(&rendered, 48, 10, state); err != nil {
		t.Fatal(err)
	}
	plain := stripSessionViewerANSI(rendered.String())
	if !strings.Contains(plain, "MODEL claude-") || !strings.Contains(plain, "PROJECT agent-deck") {
		t.Fatalf("compact preview did not preserve model/project budgets:\n%s", plain)
	}
}

func TestRenderSessionBrowserFitsResponsiveGeometries(t *testing.T) {
	items := make([]session.Metadata, 16)
	for index := range items {
		items[index] = session.Metadata{
			Client:    "codex",
			SessionID: strings.Repeat("会话-long-", 5),
			Model:     "gpt-5.6-sol",
			Project:   "/Users/example/项目-agent-deck",
			LastAt:    "2026-08-10T12:00:00Z",
		}
	}
	state := newSessionBrowserState(items)
	for _, size := range [][2]int{{48, 10}, {60, 12}, {80, 24}, {120, 24}, {140, 32}} {
		var rendered bytes.Buffer
		if err := renderSessionBrowser(&rendered, size[0], size[1], state, usageTextPrimitives{color: true}); err != nil {
			t.Fatalf("%dx%d render: %v", size[0], size[1], err)
		}
		plain := stripSessionViewerANSI(rendered.String())
		lines := strings.Split(strings.TrimSuffix(plain, "\n"), "\n")
		if len(lines) > size[1] {
			t.Fatalf("%dx%d emitted %d visual lines", size[0], size[1], len(lines))
		}
		for lineIndex, line := range lines {
			if visible := statsVisibleWidth(line); visible > size[0] {
				t.Fatalf("%dx%d line %d width %d: %q", size[0], size[1], lineIndex, visible, line)
			}
		}
		for _, want := range []string{"MODEL", "PROJECT", "LAST"} {
			if !strings.Contains(plain, want) {
				t.Fatalf("%dx%d controlled layout lost %q:\n%s", size[0], size[1], want, plain)
			}
		}
	}
}

func TestRenderSessionBrowserTooSmallFrameFits(t *testing.T) {
	state := newSessionBrowserState([]session.Metadata{{SessionID: "session-1"}})
	var rendered bytes.Buffer
	if err := renderSessionBrowser(&rendered, 20, 4, state); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stripSessionViewerANSI(rendered.String()), "\n"), "\n")
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
