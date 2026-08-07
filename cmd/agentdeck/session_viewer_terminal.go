package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func runSessionViewer(ctx context.Context, input, output *os.File, load sessionViewerLoad) error {
	if !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return errors.New("--interactive requires TTY stdin and stdout")
	}
	state, err := term.MakeRaw(int(input.Fd()))
	if err != nil {
		return fmt.Errorf("enable interactive terminal: %w", err)
	}
	defer term.Restore(int(input.Fd()), state)
	defer fmt.Fprint(output, "\x1b[?25h\n")
	if _, err := fmt.Fprint(output, "\x1b[?25l"); err != nil {
		return err
	}
	viewer := newSessionViewerState(load)
	if err := viewer.refresh(ctx); err != nil {
		return err
	}
	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)
	defer signal.Stop(resized)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		width, height := 100, 24
		if columns, rows, sizeErr := term.GetSize(int(output.Fd())); sizeErr == nil && columns > 0 && rows > 0 {
			width, height = columns, rows
		}
		if err := renderSessionViewer(output, width, height, viewer); err != nil {
			return err
		}
		key, resizedDuringRead, err := readSessionViewerKey(ctx, input, resized)
		if err != nil {
			return err
		}
		if sessionViewerShouldRedrawAfterRead(key, resizedDuringRead) {
			continue
		}
		reload, exit := viewer.apply(key)
		if exit {
			return nil
		}
		if reload {
			if err := viewer.refresh(ctx); err != nil {
				return err
			}
		}
	}
}

// sessionViewerShouldRedrawAfterRead preserves an already recognized Escape
// when a resize arrives during its ambiguity lookahead.
func sessionViewerShouldRedrawAfterRead(key string, resizedDuringRead bool) bool {
	return resizedDuringRead && key != "escape"
}

func readSessionViewerKey(ctx context.Context, input *os.File, resized <-chan os.Signal) (string, bool, error) {
	key, resizedDuringRead, err := readSessionViewerByte(ctx, input, resized, 0)
	if err != nil || resizedDuringRead {
		return "", resizedDuringRead, err
	}
	switch key {
	case 0x03:
		return "q", false, nil
	case 'q':
		return "q", false, nil
	case '\t':
		return "tab", false, nil
	case 0x1b:
		next, resizedDuringRead, err := readSessionViewerByte(ctx, input, resized, 35*time.Millisecond)
		if err != nil || resizedDuringRead {
			return "escape", resizedDuringRead, err
		}
		if next == 0 {
			return "escape", false, nil
		}
		if next != '[' {
			return "escape", false, nil
		}
		final, resizedDuringRead, err := readSessionViewerByte(ctx, input, resized, 35*time.Millisecond)
		if err != nil || resizedDuringRead {
			return "escape", resizedDuringRead, err
		}
		switch final {
		case 'A':
			return "up", false, nil
		case 'B':
			return "down", false, nil
		case 'C':
			return "right", false, nil
		case 'D':
			return "left", false, nil
		case 'H':
			return "home", false, nil
		case 'F':
			return "end", false, nil
		case 'Z':
			return "shift-tab", false, nil
		case '5':
			_, _, _ = readSessionViewerByte(ctx, input, resized, 35*time.Millisecond)
			return "page-up", false, nil
		case '6':
			_, _, _ = readSessionViewerByte(ctx, input, resized, 35*time.Millisecond)
			return "page-down", false, nil
		}
	}
	return "", false, nil
}

func readSessionViewerByte(ctx context.Context, input *os.File, resized <-chan os.Signal, timeout time.Duration) (byte, bool, error) {
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	for {
		if err := ctx.Err(); err != nil {
			return 0, false, err
		}
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			return 0, false, nil
		}
		select {
		case <-resized:
			return 0, true, nil
		default:
		}
		fds := []unix.PollFd{{Fd: int32(input.Fd()), Events: unix.POLLIN}}
		if _, err := unix.Poll(fds, 25); err != nil && !errors.Is(err, unix.EINTR) {
			return 0, false, err
		}
		if fds[0].Revents&(unix.POLLIN|unix.POLLHUP) == 0 {
			continue
		}
		var buffer [1]byte
		if _, err := input.Read(buffer[:]); err != nil {
			return 0, false, err
		}
		return buffer[0], false, nil
	}
}
