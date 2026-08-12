//go:build darwin

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func TestRunSessionViewerPTYExitResizeAndRestore(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- runSessionViewer(context.Background(), slave, slave, ptyViewerLoad(ready))
	}()
	waitForSessionViewerPTYReady(t, ready, done)
	if err := unix.IoctlSetWinsize(int(master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 10, Col: 60}); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatal(err)
	}
	if _, err := master.Write([]byte("\x1b")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("viewer did not exit after standalone PTY Escape")
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("viewer did not restore terminal state")
	}
	if err := master.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 8192)
	read, err := master.Read(buffer)
	if err != nil {
		t.Fatal(err)
	}
	output := string(buffer[:read])
	for _, want := range []string{"\x1b[?1049h", "\x1b[?25l", "\x1b[?25h", "\x1b[?1049l"} {
		if !strings.Contains(output, want) {
			t.Fatalf("terminal output missing %q: %q", want, output)
		}
	}
	if strings.Contains(strings.ReplaceAll(output, "\r\n", ""), "\n") {
		t.Fatalf("raw-mode session frame contains a bare LF: %q", output)
	}
}

func TestRunSessionViewerPTYResizeReflowsOncePerGeometryAndKeepsIdentity(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 18, Col: 60}); err != nil {
		t.Fatal(err)
	}
	frames := make(chan string, 64)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		var output strings.Builder
		buffer := make([]byte, 4096)
		for {
			count, err := master.Read(buffer)
			if count > 0 {
				output.Write(buffer[:count])
				current := output.String()
				if start := strings.LastIndex(current, "\x1b[H\x1b[2J"); start >= 0 {
					current = current[start:]
				}
				frames <- current
			}
			if err != nil {
				return
			}
		}
	}()
	loads := make(chan int, 8)
	load := func(_ context.Context, _ sessionViewerSection, page, limit int) (sessionViewerPage, error) {
		loads <- limit
		rows := make([]sessionViewerRow, limit)
		for index := range rows {
			rows[index] = sessionViewerRow{Identity: fmt.Sprintf("stable-%02d", index), Label: fmt.Sprintf("stable-%02d", index)}
		}
		return sessionViewerPage{Rows: rows, Summary: []string{fmt.Sprintf("LIMIT %d", limit)}, Page: page, Total: 80}, nil
	}
	done := make(chan error, 1)
	go func() { done <- runSessionViewer(context.Background(), slave, slave, load, true) }()
	wantLoad := func(want int) {
		t.Helper()
		select {
		case got := <-loads:
			if got != want {
				t.Fatalf("resize load limit = %d, want %d", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("viewer did not reload limit %d", want)
		}
	}
	wantFrame := func(limit int, identity string) {
		t.Helper()
		deadline := time.NewTimer(2 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case frame := <-frames:
				if strings.Contains(frame, fmt.Sprintf("LIMIT %d", limit)) &&
					strings.Contains(frame, "> "+identity) {
					return
				}
			case <-deadline.C:
				t.Fatalf("viewer did not render limit %d with selected identity %q", limit, identity)
			}
		}
	}
	wantLoad(14)
	wantFrame(14, "stable-00")
	for range 6 {
		if _, err := master.WriteString("\x1b[B"); err != nil {
			t.Fatal(err)
		}
	}
	wantFrame(14, "stable-06")
	for _, size := range []struct {
		rows, cols uint16
		limit      int
	}{{40, 180, 36}, {24, 80, 20}} {
		if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: size.rows, Col: size.cols}); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
			t.Fatal(err)
		}
		wantLoad(size.limit)
		wantFrame(size.limit, "stable-06")
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
		t.Fatal("viewer did not exit after resize acceptance")
	}
	if err := slave.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-readerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("resize frame reader did not exit")
	}
	select {
	case extra := <-loads:
		t.Fatalf("resize performed an extra reload with limit %d", extra)
	default:
	}
}

func TestRunSessionViewerPTYCancellationRestoresTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	master, slave := openSessionViewerPTY(t)
	defer master.Close()
	defer slave.Close()
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() { done <- runSessionViewer(ctx, slave, slave, ptyViewerLoad(ready)) }()
	waitForSessionViewerPTYReady(t, ready, done)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("viewer did not exit after cancellation")
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("viewer did not restore terminal state after cancellation")
	}
}

func ptyViewerLoad(ready chan<- struct{}) sessionViewerLoad {
	return func(_ context.Context, _ sessionViewerSection, page, _ int) (sessionViewerPage, error) {
		select {
		case ready <- struct{}{}:
		default:
		}
		return sessionViewerPage{Lines: []string{"row"}, Page: page, Total: 1}, nil
	}
}

func waitForSessionViewerPTYReady(t *testing.T, ready <-chan struct{}, done <-chan error) {
	t.Helper()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("viewer exited before PTY loader readiness: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("viewer did not reach PTY loader readiness")
	}
}

func TestRunSessionViewerPTYCtrlCAndEOFCleanup(t *testing.T) {
	load := func(_ context.Context, _ sessionViewerSection, page, _ int) (sessionViewerPage, error) {
		return sessionViewerPage{Rows: []sessionViewerRow{{Label: "MODEL", Value: "gpt-5.6", Detail: terminalDetailModel{notes: []terminalDetailNote{{text: "selected detail"}}}}}, Page: page, Total: 1}, nil
	}
	for _, test := range []struct {
		name   string
		exit   func(*os.File) error
		hangup bool
	}{
		{name: "ctrl-c", exit: func(master *os.File) error { _, err := master.Write([]byte{0x03}); return err }},
		{name: "eof", hangup: true, exit: func(master *os.File) error { return master.Close() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TERM", "xterm-256color")
			master, slave := openSessionViewerPTY(t)
			if !test.hangup {
				defer master.Close()
			}
			defer slave.Close()
			if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 24, Col: 100}); err != nil {
				t.Fatal(err)
			}
			before, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				done <- runSessionViewer(context.Background(), slave, slave, load)
			}()
			startup := readSessionBrowserPTYUntil(t, master, "q/esc back")
			if !strings.Contains(startup, terminalEnterScreen) {
				t.Fatalf("viewer did not enter alternate screen: %q", startup)
			}
			if err := test.exit(master); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("viewer exit error: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("viewer did not exit and clean up")
			}
			after, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("viewer did not restore terminal state")
			}
		})
	}
}

func openSessionViewerPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	fail := func(err error) (*os.File, *os.File) { master.Close(); t.Fatal(err); return nil, nil }
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), uintptr(unix.TIOCPTYGRANT), 0); errno != 0 {
		return fail(errno)
	}
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), uintptr(unix.TIOCPTYUNLK), 0); errno != 0 {
		return fail(errno)
	}
	var name [128]byte
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), uintptr(unix.TIOCPTYGNAME), uintptr(unsafe.Pointer(&name[0]))); errno != 0 {
		return fail(errno)
	}
	slave, err := os.OpenFile(string(name[:bytes.IndexByte(name[:], 0)]), os.O_RDWR, 0)
	if err != nil {
		return fail(err)
	}
	return master, slave
}
