//go:build darwin

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func TestRunSessionViewerPTYExitResizeAndRestore(t *testing.T) {
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
	<-ready
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
}

func TestRunSessionViewerPTYCancellationRestoresTerminal(t *testing.T) {
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
	<-ready
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
