package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestTerminalFrameWriterUsesCRLFInRawMode(t *testing.T) {
	var output bytes.Buffer
	frame := &terminalFrameWriter{target: &output}
	if _, err := frame.Write([]byte("first\nsecond\r")); err != nil {
		t.Fatal(err)
	}
	if _, err := frame.Write([]byte("\nthird\n")); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "first\r\nsecond\r\nthird\r\n"; got != want {
		t.Fatalf("framed output = %q, want %q", got, want)
	}
	withoutCRLF := strings.ReplaceAll(output.String(), "\r\n", "")
	if strings.Contains(withoutCRLF, "\n") {
		t.Fatalf("frame contains a bare LF: %q", output.String())
	}
}

func TestTerminalFrameWriterPropagatesOutputFailure(t *testing.T) {
	want := errors.New("write failed")
	frame := &terminalFrameWriter{target: terminalFrameErrorWriter{err: want}}
	if _, err := frame.Write([]byte("row\n")); !errors.Is(err, want) {
		t.Fatalf("write error = %v, want %v", err, want)
	}
}

type terminalFrameErrorWriter struct {
	err error
}

func (w terminalFrameErrorWriter) Write([]byte) (int, error) {
	return 0, w.err
}
