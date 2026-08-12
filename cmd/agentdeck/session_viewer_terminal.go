package main

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func runSessionViewer(ctx context.Context, input, output *os.File, load sessionViewerLoad, noColor ...bool) error {
	if !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return errors.New("--interactive requires TTY stdin and stdout")
	}
	if os.Getenv("TERM") == "dumb" {
		return errors.New("--interactive requires a usable terminal")
	}
	_, height := sessionViewerTerminalSize(output)
	viewer, err := prepareSessionViewer(ctx, load, sessionViewerAcquisitionLimit(height))
	if err != nil {
		return err
	}
	disableColor := len(noColor) > 0 && noColor[0]
	p := newUsageTextPrimitives(output, disableColor)
	terminal, err := startInteractiveTerminal(input, output)
	if err != nil {
		return err
	}
	defer terminal.Close()
	_, err = runPreparedSessionViewerScreen(ctx, input, output, terminal.frameWriter(), terminal.resized, viewer, p)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

// runSessionViewerScreen renders one detail screen inside an already-owned
// terminal. The returned key lets the session browser distinguish Back from
// global Quit without nesting raw-mode or alternate-screen lifecycles.
func runSessionViewerScreen(
	ctx context.Context,
	input, output *os.File,
	frame io.Writer,
	resized <-chan os.Signal,
	load sessionViewerLoad,
	primitives ...usageTextPrimitives,
) (string, error) {
	_, height := sessionViewerTerminalSize(output)
	viewer, err := prepareSessionViewer(ctx, load, sessionViewerAcquisitionLimit(height))
	if err != nil {
		return "", err
	}
	return runPreparedSessionViewerScreen(ctx, input, output, frame, resized, viewer, primitives...)
}

func prepareSessionViewer(ctx context.Context, load sessionViewerLoad, limit int) (*sessionViewerState, error) {
	viewer := newSessionViewerState(load)
	if err := viewer.reflow(ctx, limit); err != nil {
		return nil, err
	}
	return viewer, nil
}

func runPreparedSessionViewerScreen(
	ctx context.Context,
	input, output *os.File,
	frame io.Writer,
	resized <-chan os.Signal,
	viewer *sessionViewerState,
	primitives ...usageTextPrimitives,
) (string, error) {
	p := usageTextPrimitives{}
	if len(primitives) > 0 {
		p = primitives[0]
	}
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		width, height := sessionViewerTerminalSize(output)
		limit := sessionViewerAcquisitionLimit(height)
		if viewer.currentSection != viewer.section || viewer.limit(viewer.section) != limit {
			if err := viewer.reflow(ctx, limit); err != nil {
				return "", err
			}
		}
		if err := renderSessionViewer(frame, width, height, viewer, p); err != nil {
			return "", err
		}
		key, resizedDuringRead, err := readSessionViewerKey(ctx, input, resized)
		if err != nil {
			return "", err
		}
		if sessionViewerShouldRedrawAfterRead(key, resizedDuringRead) {
			continue
		}
		reload, exit := viewer.apply(key)
		if exit {
			return key, nil
		}
		if reload {
			if err := viewer.reflow(ctx, limit); err != nil {
				return "", err
			}
		}
	}
}

func sessionViewerTerminalSize(output *os.File) (width, height int) {
	width, height = 100, 24
	if columns, rows, err := term.GetSize(int(output.Fd())); err == nil && columns > 0 && rows > 0 {
		width, height = columns, rows
	}
	return width, height
}

func sessionViewerAcquisitionLimit(height int) int {
	// Clear/home is control-only. Title, tabs, status, and help are fixed; the
	// remaining maximum body capacity is acquired independently of Detail.
	return max(1, height-4)
}

func sessionViewerShouldRedrawAfterRead(key string, resizedDuringRead bool) bool {
	return resizedDuringRead && key == ""
}

func readSessionViewerKey(ctx context.Context, input *os.File, resized <-chan os.Signal) (string, bool, error) {
	first, resizedDuringRead, err := readSessionViewerByte(ctx, input, resized, 0)
	if err != nil || resizedDuringRead {
		return "", resizedDuringRead, err
	}
	switch first {
	case 0x03:
		return "q", false, nil
	case 'q':
		return "q", false, nil
	case '\r', '\n':
		return "enter", false, nil
	case '\t':
		return "tab", false, nil
	case 0x1b:
		next, resizedDuringRead, err := readSessionViewerByte(ctx, input, resized, 35*time.Millisecond)
		if errors.Is(err, io.EOF) {
			return "escape", false, nil
		}
		if err != nil || resizedDuringRead {
			return "escape", resizedDuringRead, err
		}
		if next == 0 || next != '[' {
			return "escape", false, nil
		}
		final, resizedDuringRead, err := readSessionViewerByte(ctx, input, resized, 35*time.Millisecond)
		if errors.Is(err, io.EOF) {
			return "escape", false, nil
		}
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
		case '5', '6':
			if _, _, suffixErr := readSessionViewerByte(ctx, input, resized, 35*time.Millisecond); suffixErr != nil && !errors.Is(suffixErr, io.EOF) {
				return "", false, suffixErr
			}
			if final == '5' {
				return "page-up", false, nil
			}
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

		waitMS := 25
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return 0, false, nil
			}
			waitMS = max(1, min(waitMS, int(remaining.Milliseconds())))
		}
		fds := []unix.PollFd{{Fd: int32(input.Fd()), Events: unix.POLLIN | unix.POLLHUP}}
		ready, err := unix.Poll(fds, waitMS)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, false, err
		}
		if ready == 0 {
			continue
		}
		if fds[0].Revents&(unix.POLLIN|unix.POLLHUP) == 0 {
			continue
		}
		buffer := []byte{0}
		count, err := input.Read(buffer)
		if count > 0 {
			return buffer[0], false, nil
		}
		if errors.Is(err, unix.EIO) {
			return 0, false, io.EOF
		}
		if err != nil {
			return 0, false, err
		}
		return 0, false, io.EOF
	}
}
