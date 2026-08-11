//go:build darwin

package main

import (
	"context"
	"strings"
	"testing"
	"time"

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
