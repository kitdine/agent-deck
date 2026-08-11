//go:build darwin

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"golang.org/x/sys/unix"
)

func TestSessionBrowserCommandReleasesStateLockWhileWaiting(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}
	stateDir := t.TempDir()
	command := newRootCommand(slave, slave)
	command.SetArgs([]string{"--state-dir", stateDir, "session", "--interactive"})
	done := make(chan error, 1)
	go func() {
		done <- command.ExecuteContext(context.Background())
	}()
	output := readSessionBrowserPTYUntil(t, master, "agentdeck session scan")
	if !strings.Contains(output, terminalEnterScreen) || !strings.Contains(output, "No indexed sessions.") {
		t.Fatalf("browser startup output incomplete: %q", output)
	}

	lock, err := store.AcquireLock(context.Background(), stateDir, 250*time.Millisecond)
	if err != nil {
		t.Fatalf("state lock remained held during browser think time: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := master.WriteString("q"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session browser did not exit after q")
	}
}

func TestSessionBrowserPTYOpensNavigatesAndReturnsFromStructuredDetail(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 120, Row: 24}); err != nil {
		t.Fatal(err)
	}

	items := []session.Metadata{{
		Client: "claude", SessionID: "root-session", Model: "claude-opus", Project: "/private/work/agent-deck",
		LastAt: "2026-08-10T12:00:00Z",
	}}
	load := func(session.Metadata) sessionViewerLoad {
		return func(_ context.Context, section sessionViewerSection, page, _ int) (sessionViewerPage, error) {
			switch section {
			case viewerOverview:
				return sessionViewerPage{Rows: []sessionViewerRow{{Label: "MODEL", Value: "claude-opus", Detail: []string{"Selected model detail"}}}, Page: page, Total: 1}, nil
			case viewerTokens:
				return sessionViewerPage{Rows: []sessionViewerRow{{Label: "#1 · claude-opus", Value: "IN 10 · OUT 2", Detail: []string{"INPUT TOKENS 10", "OUTPUT TOKENS 2", "PRICING STATUS complete"}}}, Page: page, Total: 1}, nil
			default:
				return sessionViewerPage{Empty: "No rows in fixture.", Page: page}, nil
			}
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- runSessionBrowser(context.Background(), slave, slave, items, load)
	}()
	root := readSessionBrowserPTYUntil(t, master, "SELECTED")
	for _, want := range []string{"MODEL", "PROJECT", "claude-opus", "agent-deck"} {
		if !strings.Contains(root, want) {
			t.Fatalf("root browser missing %q: %q", want, root)
		}
	}
	if strings.Contains(root, "/private/") {
		t.Fatalf("root browser exposed project path: %q", root)
	}

	if _, err := master.WriteString("\r"); err != nil {
		t.Fatal(err)
	}
	detail := readSessionBrowserPTYUntil(t, master, "DETAIL · MODEL")
	if !strings.Contains(detail, "[OVERVIEW]") {
		t.Fatalf("detail did not open on Overview: %q", detail)
	}
	if _, err := master.WriteString("\t\t\t"); err != nil {
		t.Fatal(err)
	}
	tokens := readSessionBrowserPTYUntil(t, master, "PRICING STATUS complete")
	for _, want := range []string{"[TOKENS]", "INPUT TOKENS 10", "OUTPUT TOKENS 2"} {
		if !strings.Contains(tokens, want) {
			t.Fatalf("Tokens detail missing %q: %q", want, tokens)
		}
	}

	if _, err := master.WriteString("\x1b"); err != nil {
		t.Fatal(err)
	}
	returned := readSessionBrowserPTYUntil(t, master, "SELECTED")
	if !strings.Contains(returned, "root-session") {
		t.Fatalf("Escape did not return to selected root session: %q", returned)
	}
	if _, err := master.WriteString("q"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session browser did not exit after q")
	}
}

func readSessionBrowserPTYUntil(t *testing.T, master interface {
	Read([]byte) (int, error)
	SetReadDeadline(time.Time) error
}, want string) string {
	t.Helper()
	if err := master.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	buffer := make([]byte, 4096)
	for !strings.Contains(output.String(), want) {
		read, err := master.Read(buffer)
		if err != nil {
			t.Fatalf("read browser PTY before %q: %v; output=%q", want, err, output.String())
		}
		output.Write(buffer[:read])
	}
	return output.String()
}
