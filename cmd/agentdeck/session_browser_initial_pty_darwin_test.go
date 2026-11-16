//go:build darwin

package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	"golang.org/x/sys/unix"
)

var sessionBrowserExitKeys = []struct{ name, key string }{{"q", "q"}, {"escape", "\x1b"}}

// sessionBrowserColorModes runs a frame assertion with styled and plain
// output, so a check on visible text cannot pass only because ANSI styling
// happens to split the text it looks for.
var sessionBrowserColorModes = []struct{ name, noColor string }{{"color", ""}, {"no-color", "1"}}

func openSessionBrowserPTY80x24(t *testing.T) (master, slave *os.File) {
	t.Helper()
	t.Setenv("TERM", "xterm-256color")
	master, slave = openSessionViewerPTY(t)
	t.Cleanup(func() { master.Close(); slave.Close() })
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}
	return master, slave
}

// sessionBrowserInventedResult matches what only a loaded selection may
// show: a session count ("12 SESSIONS", "0 INDEXED SESSIONS"), the row
// position, or an empty-result state. The title "AGENTDECK · SESSIONS" is
// not a result and does not match.
var sessionBrowserInventedResult = regexp.MustCompile(`\b\d+ (INDEXED )?SESSIONS\b|\brow \d+/\d+|No indexed sessions\.|sessions in this selection\.`)

// assertNoSessionBrowserResult fails when a frame drawn without any loaded
// selection shows a count, list position or empty state.
func assertNoSessionBrowserResult(t *testing.T, frame string) {
	t.Helper()
	if invented := sessionBrowserInventedResult.FindString(stripStatsANSI(frame)); invented != "" {
		t.Fatalf("frame without a loaded selection shows %q: %q", invented, frame)
	}
}

// The invented-result check itself: it must reject real results in both
// color modes and accept the loading and error frames' title.
func TestSessionBrowserInventedResultCheckSeparatesTitleFromResults(t *testing.T) {
	p := usageTextPrimitives{color: true}
	for _, frame := range []string{"0 SESSIONS · Scope: work", p.style("12 SESSIONS", usageColorSession) + " · Scope: work", "3 INDEXED SESSIONS", "row 1/3", "No indexed sessions.", "No work sessions in this selection."} {
		if sessionBrowserInventedResult.FindString(stripStatsANSI(frame)) == "" {
			t.Fatalf("result %q not recognized", frame)
		}
	}
	for _, frame := range []string{"AGENTDECK · SESSIONS · READ ONLY", p.style("AGENTDECK · SESSIONS", usageColorBrand) + " · READ ONLY", "Sessions for scope work could not be read."} {
		if invented := sessionBrowserInventedResult.FindString(stripStatsANSI(frame)); invented != "" {
			t.Fatalf("title %q reported as result %q", frame, invented)
		}
	}
}

// ASC-CLI-R3-F4: with no previous result, an unreadable core or session
// index opens an exitable error page inside the terminal owner instead of
// failing before it; q and Escape both leave it and restore the terminal.
func TestSessionBrowserPTYInitialReadFailureShowsAnExitableErrorPage(t *testing.T) {
	for _, unreadable := range []struct {
		name, database string
		open           func(context.Context, string) (*store.Store, error)
	}{
		{"core", "agentdeck.sqlite3", store.OpenReadOnly},
		{"sessions", "sessions.sqlite3", store.OpenSessionsReadOnly},
	} {
		for _, exit := range sessionBrowserExitKeys {
			t.Run(unreadable.name+"/"+exit.name, func(t *testing.T) {
				master, slave := openSessionBrowserPTY80x24(t)
				stateDir := seedOriginSessions(t)
				for _, suffix := range []string{"", "-wal", "-shm"} {
					if err := os.Remove(filepath.Join(stateDir, unreadable.database+suffix)); err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
				}
				if err := os.Mkdir(filepath.Join(stateDir, unreadable.database), 0700); err != nil {
					t.Fatal(err)
				}
				if database, err := unreadable.open(context.Background(), stateDir); err == nil {
					database.Close()
					t.Fatalf("precondition: %s must be unreadable", unreadable.database)
				}
				command := newRootCommand(slave, slave)
				command.SetArgs([]string{"--state-dir", stateDir, "session", "--interactive"})
				done, stop := startSessionBrowserPTYOwner(command.ExecuteContext)
				defer stop()
				// Checked before reading: when the command fails before the
				// owner starts, nothing is drawn, and the PTY master read
				// blocks in the read syscall without honoring its deadline.
				select {
				case err := <-done:
					t.Fatalf("browser returned before an exit key: %v", err)
				case <-time.After(250 * time.Millisecond):
				}
				page := readSessionBrowserPTYUntil(t, master, "q/esc quit")
				if !strings.Contains(page, terminalEnterScreen) || !strings.Contains(page, "Sessions for scope work could not be read.") || !strings.Contains(page, "agentdeck doctor") {
					t.Fatalf("error page = %q", page)
				}
				assertNoSessionBrowserResult(t, page)
				if _, err := master.WriteString(exit.key); err != nil {
					t.Fatal(err)
				}
				readSessionBrowserPTYUntil(t, master, terminalLeaveScreen)
				select {
				case err := <-done:
					// The diagnosis reaches stderr after the terminal is restored.
					if err == nil || !strings.Contains(err.Error(), "session scope work is unavailable") {
						t.Fatalf("leaving the error page returned %v, want the unavailable selection", err)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("browser did not exit from the error page")
				}
			})
		}
	}
}

// The first read runs inside the owner: q or Escape leaves while it is
// blocked, and the read is cancelled and collected before the browser
// returns.
func TestSessionBrowserPTYInitialBlockedReadCanQuit(t *testing.T) {
	for _, mode := range sessionBrowserColorModes {
		for _, exit := range sessionBrowserExitKeys {
			t.Run(mode.name+"/"+exit.name, func(t *testing.T) {
				t.Setenv("NO_COLOR", mode.noColor)
				master, slave := openSessionBrowserPTY80x24(t)
				started, returned := make(chan struct{}), make(chan struct{})
				load := func(ctx context.Context, _ string) (sessionBrowserPage, error) {
					close(started)
					<-ctx.Done()
					close(returned)
					return sessionBrowserPage{}, ctx.Err()
				}
				done, stop := startSessionBrowserPTYOwner(func(ctx context.Context) error {
					return runSessionBrowserLoadingFirst(ctx, slave, slave, "work", load, nil)
				})
				defer stop()
				frame := readSessionBrowserPTYUntil(t, master, "q quit · esc cancel")
				if !strings.Contains(frame, "Requested scope: work") || !strings.Contains(frame, "Loading committed sessions") {
					t.Fatalf("initial loading frame = %q", frame)
				}
				assertNoSessionBrowserResult(t, frame)
				select {
				case <-started:
				case <-time.After(time.Second):
					t.Fatal("initial loader did not start")
				}
				if _, err := master.WriteString(exit.key); err != nil {
					t.Fatal(err)
				}
				readSessionBrowserPTYUntil(t, master, terminalLeaveScreen)
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
					select {
					case <-returned:
					default:
						t.Fatal("browser returned before the cancelled first read was collected")
					}
				case <-time.After(2 * time.Second):
					t.Fatal("browser did not exit during the first read")
				}
			})
		}
	}
}
