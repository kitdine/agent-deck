package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/term"
)

const (
	terminalEnterScreen = "\x1b[?1049h\x1b[?25l"
	terminalLeaveScreen = "\x1b[?25h\x1b[?1049l"
)

// interactiveTerminal owns every process-global and terminal-local resource
// used by a full-screen viewer. Close is idempotent so every exit path can
// restore the same state without coordinating cleanup elsewhere.
type interactiveTerminal struct {
	input   *os.File
	output  *os.File
	raw     *term.State
	resized chan os.Signal
	once    sync.Once
}

func startInteractiveTerminal(input, output *os.File) (*interactiveTerminal, error) {
	raw, err := term.MakeRaw(int(input.Fd()))
	if err != nil {
		return nil, fmt.Errorf("enable interactive terminal: %w", err)
	}
	if _, err := io.WriteString(output, terminalEnterScreen); err != nil {
		_ = term.Restore(int(input.Fd()), raw)
		return nil, err
	}
	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)
	return &interactiveTerminal{input: input, output: output, raw: raw, resized: resized}, nil
}

func (t *interactiveTerminal) Close() {
	if t == nil {
		return
	}
	t.once.Do(func() {
		signal.Stop(t.resized)
		_, _ = io.WriteString(t.output, terminalLeaveScreen)
		_ = term.Restore(int(t.input.Fd()), t.raw)
	})
}

func (t *interactiveTerminal) frameWriter() io.Writer {
	return &terminalFrameWriter{target: t.output}
}

// terminalFrameWriter makes logical newlines independent of OPOST. MakeRaw
// disables output post-processing, so a bare LF would move down without
// returning to column zero. Every frame gets a fresh writer and therefore a
// fresh CRLF boundary state.
type terminalFrameWriter struct {
	target   io.Writer
	previous byte
}

func (w *terminalFrameWriter) Write(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, nil
	}
	framed := make([]byte, 0, len(value)+8)
	previous := w.previous
	for _, current := range value {
		if current == '\n' && previous != '\r' {
			framed = append(framed, '\r')
		}
		framed = append(framed, current)
		previous = current
	}
	w.previous = previous
	written, err := w.target.Write(framed)
	if err != nil {
		return 0, err
	}
	if written != len(framed) {
		return 0, io.ErrShortWrite
	}
	return len(value), nil
}
