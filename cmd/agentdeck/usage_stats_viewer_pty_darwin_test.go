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

func TestRunUsageStatsViewerPTYNavigatesToActivityHeatmap(t *testing.T) {
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}

	activitySeen := make(chan struct{}, 1)
	outputDone := make(chan string, 1)
	go func() {
		var output strings.Builder
		buffer := make([]byte, 4096)
		for {
			count, err := master.Read(buffer)
			if count > 0 {
				output.Write(buffer[:count])
				if strings.Contains(output.String(), "1H BUCKET") {
					select {
					case activitySeen <- struct{}{}:
					default:
					}
				}
			}
			if err != nil {
				outputDone <- output.String()
				return
			}
		}
	}()

	report := usage.StatsReport{
		Metric: "tokens",
		Range:  usage.StatsRange{From: "2026-08-01T00:00:00Z", To: "2026-08-08T00:00:00Z"},
		Activity: []usage.StatsActivity{
			{Weekday: 0, Hour: 9, KnownMetricValue: "10"},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runUsageStatsViewer(ctx, slave, slave, report, nil, true, nil)
	}()
	time.Sleep(25 * time.Millisecond)
	if _, err := master.WriteString("\x1b[C\x1b[C"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-activitySeen:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatal("usage viewer did not render Activity after two right-arrow keys")
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
		cancel()
		t.Fatal("usage viewer did not exit after Activity acceptance")
	}
	if err := slave.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case output := <-outputDone:
		for _, want := range []string{"[ACTIVITY]", "1H BUCKET", "Mon"} {
			if !strings.Contains(output, want) {
				t.Fatalf("PTY Activity output missing %q: %q", want, output)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("PTY Activity reader did not finish after viewer exit")
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
