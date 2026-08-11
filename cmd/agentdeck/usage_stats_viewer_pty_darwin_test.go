//go:build darwin

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/usage"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func TestRunUsageStatsViewerPTYExitRestoresScreen(t *testing.T) {
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- runUsageStatsViewer(context.Background(), slave, slave, usage.StatsReport{Metric: "tokens"}, nil, true, nil)
	}()
	time.Sleep(25 * time.Millisecond)
	if _, err := master.WriteString("q"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("usage viewer did not exit after q")
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("raw mode was not restored")
	}
	if err := master.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 8192)
	n, err := master.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	output := string(buf[:n])
	for _, want := range []string{"\x1b[?1049h", "\x1b[?25l", "\x1b[?25h", "\x1b[?1049l"} {
		if !strings.Contains(output, want) {
			t.Fatalf("terminal output missing %q: %q", want, output)
		}
	}
	if strings.Contains(strings.ReplaceAll(output, "\r\n", ""), "\n") {
		t.Fatalf("raw-mode usage frame contains a bare LF: %q", output)
	}
}

func TestRunUsageStatsViewerPTYCancellationRestoresTerminal(t *testing.T) {
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runUsageStatsViewer(ctx, slave, slave, usage.StatsReport{Metric: "tokens"}, nil, true, nil)
	}()
	time.Sleep(25 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("usage viewer did not exit after cancellation")
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("raw mode was not restored after cancellation")
	}
}

func TestRunUsageStatsViewerPTYInterruptExitsAndReleasesInput(t *testing.T) {
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _ = io.Copy(io.Discard, master)
	}()
	done := make(chan error, 1)
	go func() {
		done <- runUsageStatsViewer(context.Background(), slave, slave, usage.StatsReport{Metric: "tokens"}, nil, true, nil)
	}()
	time.Sleep(25 * time.Millisecond)
	if _, err := master.WriteString("\x03"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("usage viewer did not exit after ctrl-c")
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("raw mode was not restored after ctrl-c")
	}
	// A surviving reader would consume this line instead of leaving it for the
	// terminal owner.
	if _, err := master.WriteString("released\n"); err != nil {
		t.Fatal(err)
	}
	if err := slave.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	n, err := slave.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(buf[:n]), "released") {
		t.Fatalf("terminal input after exit = %q", string(buf[:n]))
	}
}

func TestRunUsageStatsViewerPTYInputEOFRestoresTerminal(t *testing.T) {
	master, slave := openSessionViewerPTY(t)
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- runUsageStatsViewer(context.Background(), slave, slave, usage.StatsReport{Metric: "tokens"}, nil, true, nil)
	}()
	time.Sleep(50 * time.Millisecond)
	// Drain the first frame from the same goroutine that closes the master, so
	// the viewer is blocked on input rather than on a full terminal buffer.
	if err := master.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := master.Read(make([]byte, 8192)); err != nil {
		t.Fatal(err)
	}
	master.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("input EOF error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("usage viewer did not exit after input EOF")
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("raw mode was not restored after input EOF")
	}
}

func TestRunUsageStatsViewerPTYWriteFailureRestoresTerminal(t *testing.T) {
	master, slave := openSessionViewerPTY(t)
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	master.Close()
	err = runUsageStatsViewer(context.Background(), slave, slave, usage.StatsReport{Metric: "tokens"}, nil, true, nil)
	if !errors.Is(err, syscall.EIO) {
		t.Fatalf("terminal write failure error = %v", err)
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("raw mode was not restored after a terminal write failure")
	}
}

func TestRunUsageStatsViewerPTYResizeTooSmallAndRecover(t *testing.T) {
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	resize := func(columns, rows uint16) {
		t.Helper()
		if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: columns, Row: rows}); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
			t.Fatal(err)
		}
	}
	resize(80, 24)
	go func() {
		_, _ = io.Copy(io.Discard, master)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runUsageStatsViewer(ctx, slave, slave, usage.StatsReport{Metric: "tokens"}, nil, true, nil)
	}()
	time.Sleep(50 * time.Millisecond)
	resize(40, 9)
	time.Sleep(50 * time.Millisecond)
	resize(80, 24)
	time.Sleep(150 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("resize cancellation error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("usage viewer did not exit after resize recovery")
	}
}
