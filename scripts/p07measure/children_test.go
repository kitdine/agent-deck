package p07measure

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestObservedCommandRetainsOutputErrorAndBoundedStderr(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS lifecycle")
	}
	dir := t.TempDir()
	t.Setenv("AGENTDECK_P07_BARRIER_DIR", dir)
	command := func() *exec.Cmd {
		c := exec.Command(os.Args[0], "-test.run=^TestChildCommandHelper$")
		c.Env = append(os.Environ(), "P07_CHILD_HELPER=1")
		return c
	}
	want, normalErr := command().Output()
	got, observedErr := CommandOutput("synthetic", command())
	normal, ok := normalErr.(*exec.ExitError)
	if !ok {
		t.Fatalf("normal error=%v", normalErr)
	}
	observed, ok := observedErr.(*exec.ExitError)
	if !ok {
		t.Fatalf("observed error=%v", observedErr)
	}
	if !bytes.Equal(got, want) || !bytes.Equal(normal.Stderr, observed.Stderr) || observed.ExitCode() != normal.ExitCode() {
		t.Fatal("observed Output changed stdout/stderr/exit semantics")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.child_*.json"))
	if err != nil || len(paths) != 2 {
		t.Fatalf("child records=%d %v", len(paths), err)
	}
	var start, end childRecord
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var value childRecord
		if err = json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		if value.Event == "child_start" {
			start = value
		} else {
			end = value
		}
	}
	if start.Birth == "" || start.Birth != end.Birth || start.PID != end.PID || start.ParentPID != os.Getpid() || start.BeforeStartNS > start.ObservedNS || start.ObservedNS > end.ObservedNS || end.ObservedNS > childClock() {
		t.Fatalf("unbound lifecycle: %+v %+v", start, end)
	}
	if len(observed.Stderr) > 66<<10 {
		t.Fatal("stderr saver unbounded")
	}
}

func TestObservedCommandCombinedAndStartWait(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS lifecycle")
	}
	t.Setenv("AGENTDECK_P07_BARRIER_DIR", t.TempDir())
	output, err := CommandCombinedOutput("synthetic", exec.Command("/bin/sh", "-c", "printf out;printf err >&2"))
	if err != nil || string(output) != "outerr" {
		t.Fatalf("combined=%q %v", output, err)
	}
	c := exec.Command("/usr/bin/true")
	if err = CommandStart("synthetic", c); err != nil {
		t.Fatal(err)
	}
	if err = CommandWait("synthetic", c); err != nil {
		t.Fatal(err)
	}
}

func TestChildCommandHelper(t *testing.T) {
	if os.Getenv("P07_CHILD_HELPER") != "1" {
		return
	}
	_, _ = os.Stdout.WriteString("content-free-child-output")
	_, _ = os.Stderr.WriteString(strings.Repeat("a", 40<<10) + strings.Repeat("b", 40<<10) + strings.Repeat("c", 40<<10))
	os.Exit(7)
}
