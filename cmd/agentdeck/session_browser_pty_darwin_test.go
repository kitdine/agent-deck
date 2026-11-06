//go:build darwin

package main

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
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
	core, err := store.Open(context.Background(), stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	index, err := store.OpenSessions(context.Background(), stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	command := newRootCommand(slave, slave)
	command.SetArgs([]string{"--state-dir", stateDir, "session", "--interactive"})
	done, stop := startSessionBrowserPTYOwner(command.ExecuteContext)
	defer stop()
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
				return sessionViewerPage{Rows: []sessionViewerRow{{Label: "MODEL", Value: "claude-opus", Detail: terminalDetailModel{notes: []terminalDetailNote{{text: "Selected model detail"}}}}}, Page: page, Total: 1}, nil
			case viewerTokens:
				return sessionViewerPage{Rows: []sessionViewerRow{{Label: "#1 · claude-opus", Value: "IN 10 · OUT 2", Detail: terminalDetailModel{fields: []terminalDetailField{{label: "INPUT TOKENS", value: "10", role: terminalDetailRoleToken}, {label: "OUTPUT TOKENS", value: "2", role: terminalDetailRoleToken}}, notes: []terminalDetailNote{{status: "complete", role: terminalDetailRoleSuccess}}}}}, Page: page, Total: 1}, nil
			default:
				return sessionViewerPage{Empty: "No rows in fixture.", Page: page}, nil
			}
		}
	}

	done, stop := startSessionBrowserPTYOwner(func(ctx context.Context) error {
		return runSessionBrowser(ctx, slave, slave, sessionBrowserPage{items: items}, nil, load)
	})
	defer stop()
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
	tokens := readSessionBrowserPTYUntil(t, master, "COMPLETE")
	for _, want := range []string{"[TOKENS]", "INPUT TOKENS", "10", "OUTPUT TOKENS", "2"} {
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

func TestSessionBrowserPTYOwnerCanBeStoppedBeforeTheFirstFrame(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	done, stop := startSessionBrowserPTYOwner(func(ctx context.Context) error {
		return runSessionBrowser(ctx, slave, slave, sessionBrowserPage{}, nil, nil)
	})
	defer stop()
	stop()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("early browser cancellation = %v", err)
	}
	// Closing is safe only after the browser's terminal operations have ended.
	if err := slave.Close(); err != nil {
		t.Fatal(err)
	}
}

func startSessionBrowserPTYOwner(run func(context.Context) error) (<-chan error, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		done <- run(ctx)
	}()
	return done, func() {
		cancel()
		<-stopped
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

func TestSessionBrowserPTYSwitchesScopeWithoutWritingAndRestoresTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}
	stateDir := seedOriginSessions(t)
	if result := runOriginCLI(t, stateDir, "--format", "json", "session", "classify", "aux-b", "--client", "claude", "--set", "auxiliary"); result.exit != 0 {
		t.Fatalf("set = %#v", result)
	}
	before := stateFileBytes(t, stateDir)
	command := newRootCommand(slave, slave)
	command.SetArgs([]string{"--state-dir", stateDir, "session", "--interactive"})
	done, stop := startSessionBrowserPTYOwner(command.ExecuteContext)
	defer stop()
	first := readSessionBrowserPTYUntil(t, master, "s scope")
	if !strings.Contains(first, "Scope: work") || strings.Contains(first, "aux-b") {
		t.Fatalf("initial work frame = %q", first)
	}
	if _, err := master.WriteString("s"); err != nil {
		t.Fatal(err)
	}
	readSessionBrowserPTYUntil(t, master, "esc cancel")
	if _, err := master.WriteString("\x1b[B\r"); err != nil {
		t.Fatal(err)
	}
	auxiliary := readSessionBrowserPTYUntil(t, master, "aux-b")
	if !strings.Contains(auxiliary, "Scope: auxiliary") {
		t.Fatalf("auxiliary frame = %q", auxiliary)
	}
	if _, err := master.WriteString("q"); err != nil {
		t.Fatal(err)
	}
	// The cursor and main screen are restored before the owner returns.
	readSessionBrowserPTYUntil(t, master, terminalLeaveScreen)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session browser did not exit after q")
	}
	if after := stateFileBytes(t, stateDir); !reflect.DeepEqual(before, after) {
		t.Fatal("interactive scope switching changed state")
	}
}

// startBlockedScopeLoad opens the scope menu of a work browser on an 80x24
// PTY, applies auxiliary and waits until the loading frame and the blocked
// loader are both observed.
func startBlockedScopeLoad(t *testing.T, load sessionBrowserScopeLoad, started <-chan struct{}) (master *os.File, done <-chan error, stop func()) {
	t.Helper()
	t.Setenv("TERM", "xterm-256color")
	master, slave := openSessionViewerPTY(t)
	t.Cleanup(func() { master.Close(); slave.Close() })
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}
	initial := sessionBrowserPage{scope: "work", items: []session.Metadata{{Client: "claude", SessionID: "old-row"}}, classes: []string{"unknown"}}
	done, stop = startSessionBrowserPTYOwner(func(ctx context.Context) error {
		return runSessionBrowser(ctx, slave, slave, initial, load, nil)
	})
	readSessionBrowserPTYUntil(t, master, "s scope")
	if _, err := master.WriteString("s"); err != nil {
		t.Fatal(err)
	}
	readSessionBrowserPTYUntil(t, master, "esc cancel")
	if _, err := master.WriteString("\x1b[B\r"); err != nil {
		t.Fatal(err)
	}
	readSessionBrowserPTYUntil(t, master, "Loading committed sessions")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("scope loader did not start")
	}
	return master, done, stop
}

// ASC-CLI-R1-F1: q must exit while a scope read is blocked, without waiting
// for the read to complete, even when the loader ignores cancellation.
func TestSessionBrowserPTYQuitsDuringABlockedScopeRead(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	load := func(context.Context, string) (sessionBrowserPage, error) {
		close(started)
		<-release
		return sessionBrowserPage{scope: "auxiliary"}, nil
	}
	master, done, stop := startBlockedScopeLoad(t, load, started)
	defer stop()
	if _, err := master.WriteString("q"); err != nil {
		t.Fatal(err)
	}
	readSessionBrowserPTYUntil(t, master, terminalLeaveScreen)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(sessionBrowserLoadDrain + 2*time.Second):
		t.Fatal("q was ignored while the scope read was blocked")
	}
}

// A loader that honors its context is cancelled and collected before the
// browser returns, so no read outlives the restored terminal.
func TestSessionBrowserPTYQuitCancelsAndCollectsTheScopeRead(t *testing.T) {
	started, returned := make(chan struct{}), make(chan struct{})
	load := func(ctx context.Context, _ string) (sessionBrowserPage, error) {
		close(started)
		<-ctx.Done()
		close(returned)
		return sessionBrowserPage{}, ctx.Err()
	}
	master, done, stop := startBlockedScopeLoad(t, load, started)
	defer stop()
	if _, err := master.WriteString("q"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-returned:
		default:
			t.Fatal("browser returned before the cancelled scope read was collected")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("browser did not exit after q during loading")
	}
}

// Escape cancels only the load: the previous list and scope stay, and the
// cancelled request's late result is never applied.
func TestSessionBrowserPTYEscapeCancelsLoadAndDiscardsTheLateResult(t *testing.T) {
	started, release, delivered := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	load := func(context.Context, string) (sessionBrowserPage, error) {
		close(started)
		<-release
		defer close(delivered)
		return sessionBrowserPage{scope: "auxiliary", items: []session.Metadata{{Client: "codex", SessionID: "late-row"}}, classes: []string{"auxiliary"}}, nil
	}
	master, done, stop := startBlockedScopeLoad(t, load, started)
	defer stop()
	if _, err := master.WriteString("\x1b"); err != nil {
		t.Fatal(err)
	}
	// Read the restored list frame through its last line in one call.
	back := readSessionBrowserPTYUntil(t, master, "s scope")
	if !strings.Contains(back, "Scope: work") || !strings.Contains(back, "old-row") {
		t.Fatalf("escape did not keep the previous scope: %q", back)
	}
	once.Do(func() { close(release) })
	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("late loader did not finish")
	}
	if _, err := master.WriteString("\x1b[B"); err != nil {
		t.Fatal(err)
	}
	redrawn := readSessionBrowserPTYUntil(t, master, "s scope")
	if strings.Contains(redrawn, "late-row") || !strings.Contains(redrawn, "Scope: work") {
		t.Fatalf("late result of a cancelled load was applied: %q", redrawn)
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
