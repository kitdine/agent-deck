package main

import (
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
	if os.Getenv("AGENTDECK_PERF_SIGNAL_CHILD") == "1" {
		if !perftrace.Start(perftrace.RoleHelper) {
			os.Exit(3)
		}
		flushTraceOnTermination()
		perftrace.Mark(perftrace.JobStart)
		os.Stdout.WriteString("ready\n")
		time.Sleep(time.Minute)
		os.Exit(4)
	}
	dir := filepath.Join(t.TempDir(), "perf")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestTracedHelperFlushesOnSIGTERM$")
	child.Env = append(os.Environ(), "AGENTDECK_PERF_SIGNAL_CHILD=1", perftrace.EnvOut+"="+dir)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 6)
	if _, err := stdout.Read(buffer); err != nil || string(buffer) != "ready\n" {
		t.Fatalf("child not ready: %q %v", buffer, err)
	}
	if err := child.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	err = child.Wait()
	status, ok := child.ProcessState.Sys().(syscall.WaitStatus)
	if err == nil || !ok || !status.Signaled() || status.Signal() != syscall.SIGTERM {
		t.Fatalf("child did not die by SIGTERM: %v", err)
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
