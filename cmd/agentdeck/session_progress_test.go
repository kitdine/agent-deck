package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/session"
)

type recordingSessionProgress struct {
	starts  int
	stops   int
	updates []session.ScanProgress
	stderr  io.Writer
}

func (r *recordingSessionProgress) Start() { r.starts++ }

func (r *recordingSessionProgress) Update(progress session.ScanProgress) {
	r.updates = append(r.updates, progress)
}

func (r *recordingSessionProgress) Stop() {
	r.stops++
	if r.stderr != nil {
		_, _ = io.WriteString(r.stderr, "session-progress-finished\n")
	}
}

func TestSessionProgressOutputUsesTTYAndNonTTYContracts(t *testing.T) {
	nonTTYClock := newManualUsageProgressClock()
	nonTTYWriter := newSynchronizedProgressWriter()
	nonTTY := newSessionProgressOutputWithClock(nonTTYWriter, false, false, nonTTYClock)
	nonTTY.Start()
	nonTTY.Update(session.ScanProgress{Processed: 1, Total: 2, Documents: 1})
	nonTTYClock.fireTimer(t)
	nonTTYWriter.waitForWrite(t)
	nonTTYClock.waitForTicker(t)
	nonTTY.Update(session.ScanProgress{Processed: 2, Total: 2, Documents: 3})
	nonTTYClock.fireTicker(t)
	nonTTYWriter.waitForWrite(t)
	nonTTY.Stop()
	if got, want := nonTTYWriter.String(), "session scan: 1/2 source files, 1 documents, 0 skipped\nsession scan: 2/2 source files, 3 documents, 0 skipped\nsession scan: 2/2 source files, 3 documents, 0 skipped\n"; got != want || strings.Contains(got, "\x1b[") {
		t.Fatalf("non-TTY progress=%q want=%q", got, want)
	}

	ttyClock := newManualUsageProgressClock()
	ttyWriter := newSynchronizedProgressWriter()
	tty := newSessionProgressOutputWithClock(ttyWriter, false, true, ttyClock)
	tty.Start()
	tty.Update(session.ScanProgress{Processed: 1, Total: 2, Documents: 1})
	ttyClock.fireTimer(t)
	ttyWriter.waitForWrite(t)
	tty.Update(session.ScanProgress{Processed: 2, Total: 2, Documents: 3})
	tty.Stop()
	if got, want := ttyWriter.String(), "\r\x1b[2Ksession scan: 1/2 source files, 1 documents, 0 skipped\r\x1b[2Ksession scan: 2/2 source files, 3 documents, 0 skipped\n"; got != want {
		t.Fatalf("TTY progress=%q want=%q", got, want)
	}
}

func TestSessionProgressOutputSuppressesFastQuietAndZeroSourceScans(t *testing.T) {
	fastClock := newManualUsageProgressClock()
	fastWriter := newSynchronizedProgressWriter()
	fast := newSessionProgressOutputWithClock(fastWriter, false, false, fastClock)
	fast.Start()
	fast.Update(session.ScanProgress{Processed: 1, Total: 1, Documents: 1})
	fast.Stop()
	if got := fastWriter.String(); got != "" {
		t.Fatalf("fast progress=%q", got)
	}

	quietClock := newManualUsageProgressClock()
	quietWriter := newSynchronizedProgressWriter()
	quiet := newSessionProgressOutputWithClock(quietWriter, true, false, quietClock)
	quiet.Start()
	quiet.Update(session.ScanProgress{Processed: 1, Total: 1, Documents: 1})
	quiet.Stop()
	if got := quietWriter.String(); got != "" {
		t.Fatalf("quiet progress=%q", got)
	}
	quietClock.mu.Lock()
	quietTimer := quietClock.timer
	quietClock.mu.Unlock()
	if quietTimer != nil {
		t.Fatal("quiet progress created a timer")
	}

	zeroClock := newManualUsageProgressClock()
	zeroWriter := newSynchronizedProgressWriter()
	zero := newSessionProgressOutputWithClock(zeroWriter, false, false, zeroClock)
	zero.Start()
	zero.Update(session.ScanProgress{})
	zeroClock.fireTimer(t)
	zeroClock.waitForTicker(t)
	zero.Stop()
	if got := zeroWriter.String(); got != "" {
		t.Fatalf("zero-source progress=%q", got)
	}
}

func TestSessionCommandsUseProgressWithoutPollutingJSONOrCompletionOrder(t *testing.T) {
	const privateFixtureText = "private fixture text must never be rendered"
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	source := filepath.Join(home, ".codex", "sessions", "progress.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("{\"type\":\"session_meta\",\"payload\":{\"session_id\":\"progress-session\"}}\n{\"type\":\"visible_user_prompt\",\"payload\":{\"text\":\""+privateFixtureText+"\"}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertNoPrivateProgressData := func(label string, outputs ...string) {
		t.Helper()
		for _, value := range outputs {
			if strings.Contains(value, privateFixtureText) || strings.Contains(value, source) {
				t.Fatalf("%s leaked private scan data: %q", label, value)
			}
		}
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })
	oldProgress := newSessionProgress
	var reporters []*recordingSessionProgress
	var quietValues []bool
	newSessionProgress = func(stderr io.Writer, quiet bool) session.ScanProgressReporter {
		quietValues = append(quietValues, quiet)
		reporter := &recordingSessionProgress{stderr: stderr}
		reporters = append(reporters, reporter)
		return reporter
	}
	t.Cleanup(func() { newSessionProgress = oldProgress })

	var combined bytes.Buffer
	if exit := execute([]string{"--state-dir", state, "session", "scan"}, bytes.NewReader(nil), &combined, &combined); exit != 0 {
		t.Fatalf("scan exit=%d output=%q", exit, combined.String())
	}
	progressAt := strings.Index(combined.String(), "session-progress-finished\n")
	completedAt := strings.Index(combined.String(), "Completed session.scan.")
	if progressAt < 0 || completedAt < 0 || progressAt > completedAt {
		t.Fatalf("completion ordering=%q", combined.String())
	}
	assertNoPrivateProgressData("scan", combined.String())

	var rebuildOut, rebuildErr bytes.Buffer
	if exit := execute([]string{"--state-dir", state, "session", "rebuild"}, bytes.NewReader(nil), &rebuildOut, &rebuildErr); exit != 0 {
		t.Fatalf("rebuild exit=%d stdout=%q stderr=%q", exit, rebuildOut.String(), rebuildErr.String())
	}
	if !strings.Contains(rebuildOut.String(), "Completed session.rebuild.") || rebuildErr.String() != "session-progress-finished\n" {
		t.Fatalf("rebuild stdout=%q stderr=%q", rebuildOut.String(), rebuildErr.String())
	}
	assertNoPrivateProgressData("rebuild", rebuildOut.String(), rebuildErr.String())

	var jsonOut, jsonErr bytes.Buffer
	if exit := execute([]string{"--state-dir", state, "--format", "json", "--quiet", "session", "scan"}, bytes.NewReader(nil), &jsonOut, &jsonErr); exit != 0 {
		t.Fatalf("JSON scan exit=%d stdout=%q stderr=%q", exit, jsonOut.String(), jsonErr.String())
	}
	if strings.Contains(jsonOut.String(), "session-progress-finished") {
		t.Fatalf("JSON stdout leaked progress: %q", jsonOut.String())
	}
	assertNoPrivateProgressData("JSON scan", jsonOut.String(), jsonErr.String())
	var envelope map[string]any
	if err := json.Unmarshal(jsonOut.Bytes(), &envelope); err != nil || envelope["command"] != "session.scan" {
		t.Fatalf("JSON stdout=%q envelope=%#v err=%v", jsonOut.String(), envelope, err)
	}
	if jsonErr.String() != "session-progress-finished\n" || len(reporters) != 3 || len(quietValues) != 3 || quietValues[0] || quietValues[1] || !quietValues[2] {
		t.Fatalf("stderr=%q reporters=%d quiet=%v", jsonErr.String(), len(reporters), quietValues)
	}
	for index, reporter := range reporters {
		if reporter.starts != 1 || reporter.stops != 1 || len(reporter.updates) == 0 {
			t.Fatalf("command %d progress starts=%d stops=%d updates=%#v", index, reporter.starts, reporter.stops, reporter.updates)
		}
	}
}
