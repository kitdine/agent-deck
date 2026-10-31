package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/perftrace"
)

// The App terminates lifetime-bound helpers at exit. A measured helper must
// still flush its trace, and must still terminate by SIGTERM.
func TestTracedHelperFlushesOnSIGTERM(t *testing.T) {
	if mode := os.Getenv("AGENTDECK_PERF_SIGNAL_CHILD"); mode != "" {
		ready := make(chan struct{})
		flushTraceOnTermination(ready)
		stdout, _ := tracePipeOutputs()
		if mode == "startup" {
			os.Stdout.WriteString("ready\n")
			if _, err := io.ReadFull(os.Stdin, make([]byte, 1)); err != nil {
				os.Exit(5)
			}
		}
		if !perftrace.Start(perftrace.RoleHelper) {
			os.Exit(3)
		}
		perftrace.Mark(perftrace.JobStart)
		close(ready)
		if mode != "startup" {
			os.Stdout.WriteString("ready\n")
		}
		if mode == "pipe" {
			if _, err := io.ReadFull(os.Stdin, make([]byte, 1)); err != nil {
				os.Exit(5)
			}
			stdout.Write([]byte("output after owner exit\n"))
			_ = perftrace.Close()
			os.Exit(4)
		}
		time.Sleep(time.Minute)
		os.Exit(4)
	}
	for _, mode := range []string{"initialized", "startup", "pipe"} {
		t.Run(mode, func(t *testing.T) { testTracedHelperTermination(t, mode) })
	}
}

func testTracedHelperTermination(t *testing.T, mode string) {
	dir := filepath.Join(t.TempDir(), "perf")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestTracedHelperFlushesOnSIGTERM$")
	child.Env = append(os.Environ(), "AGENTDECK_PERF_SIGNAL_CHILD="+mode, perftrace.EnvOut+"="+dir)
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill() })
	buffer := make([]byte, 6)
	if _, err := io.ReadFull(stdout, buffer); err != nil || string(buffer) != "ready\n" {
		t.Fatalf("child not ready: %q %v", buffer, err)
	}
	wantSignal := syscall.SIGTERM
	if mode == "pipe" {
		wantSignal = syscall.SIGPIPE
		if err := stdout.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := stdin.Write([]byte{1}); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := child.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
	}
	wait := make(chan error, 1)
	go func() { wait <- child.Wait() }()
	if mode == "startup" {
		select {
		case err := <-wait:
			t.Fatalf("termination bypassed trace initialization: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
		if _, err := stdin.Write([]byte{1}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err = <-wait:
	case <-time.After(5 * time.Second):
		t.Fatal("terminated helper did not exit")
	}
	status, ok := child.ProcessState.Sys().(syscall.WaitStatus)
	if err == nil || !ok || !status.Signaled() || status.Signal() != wantSignal {
		t.Fatalf("child did not die by %v: %v", wantSignal, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "helper-000001.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{`"stage":"job.start"`, `"stage":"resource.lifetime"`} {
		if !strings.Contains(string(raw), stage) {
			t.Fatalf("trace lost %s:\n%s", stage, raw)
		}
	}
}
