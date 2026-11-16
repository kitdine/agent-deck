package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A worker hidden from the process listing still leaves its trace file empty
// until it flushes; the run must wait for the lifetime record instead of
// handing an empty trace to the reader.
func TestWaitForTracesWaitsForTheLifetimeRecord(t *testing.T) {
	lifetime := traceRecord{Stage: "resource.lifetime", T: 9}
	complete := processTrace("cli", lifetime)

	dir := t.TempDir()
	if err := waitForTraces(dir, time.Second); err != nil {
		t.Fatalf("a run without traces must not wait: %v", err)
	}

	writeFile(t, filepath.Join(dir, "cli-000001.jsonl"), complete)
	worker := filepath.Join(dir, "collector-000001.jsonl")
	writeFile(t, worker, "")
	flushed := make(chan error, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		flushed <- os.WriteFile(worker, []byte(processTrace("collector", lifetime)), 0o600)
	}()
	started := time.Now()
	if err := waitForTraces(dir, 10*time.Second); err != nil {
		t.Fatalf("flushed trace not accepted: %v", err)
	}
	if err := <-flushed; err != nil {
		t.Fatal(err)
	}
	if time.Since(started) < 300*time.Millisecond {
		t.Fatal("returned before the worker flushed its trace")
	}
	counts, err := scanEndCounts(dir)
	if err != nil {
		t.Fatalf("reader rejected the awaited traces: %v %v", counts, err)
	}

	// A header without the lifetime record is a partially flushed trace.
	partial := strings.SplitAfter(complete, "\n")[0]
	stuck := t.TempDir()
	writeFile(t, filepath.Join(stuck, "collector-000001.jsonl"), partial)
	err = waitForTraces(stuck, 300*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "collector-000001.jsonl: trace not flushed") {
		t.Fatalf("an unflushed trace must fail the run, got %v", err)
	}
}

// ASC-OS-R3-F1: Close writes resource.lifetime before its trace.invalid tail,
// and a 64 KiB buffer boundary can flush a prefix that ends exactly with the
// lifetime record. While the producer still holds the trace, such a prefix
// must not be accepted, although it already reads as a valid sample.
func TestWaitForTracesRejectsALifetimePrefixBeforeTheTail(t *testing.T) {
	const bufferSize = 64 << 10
	// The producer's record encoding (internal/perftrace record).
	type record struct {
		Stage string           `json:"stage"`
		T     int64            `json:"t"`
		Dur   *int64           `json:"dur,omitempty"`
		N     map[string]int64 `json:"n,omitempty"`
		Err   string           `json:"err,omitempty"`
	}
	encode := func(record record) string {
		raw, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw) + "\n"
	}
	header := `{"v":1,"role":"collector","seq":1,"clock":"CLOCK_UPTIME_RAW","t":0}` + "\n"
	lifetime := encode(record{Stage: "resource.lifetime", T: 9, N: map[string]int64{"maxrss_bytes": 1}})
	scanEnd := func(digits int) string {
		value := int64(1)
		for i := 1; i < digits; i++ {
			value *= 10
		}
		return encode(record{Stage: "scan.end", T: 1, N: map[string]int64{"read_bytes": value}})
	}
	var prefix strings.Builder
	prefix.WriteString(header)
	// Fill with minimal lines, then let three final lines absorb the
	// remainder with up to 17 extra digits each, so the lifetime record ends
	// exactly at the buffer boundary.
	short, long := len(scanEnd(1)), len(scanEnd(18))
	for bufferSize-prefix.Len()-len(lifetime) > 3*long {
		prefix.WriteString(scanEnd(1))
	}
	for left := 3; left > 0; left-- {
		extra := min(bufferSize-prefix.Len()-len(lifetime)-left*short, long-short)
		prefix.WriteString(scanEnd(1 + extra))
	}
	prefix.WriteString(lifetime)
	if prefix.Len() != bufferSize {
		t.Fatalf("prefix = %d bytes, want %d", prefix.Len(), bufferSize)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "collector-000001.jsonl")
	producer, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()
	if err := syscall.Flock(int(producer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if _, err := producer.WriteString(prefix.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := readSample(dir); err != nil {
		t.Fatalf("fixture is not the counterexample: the prefix must read as a valid sample, got %v", err)
	}
	err = waitForTraces(dir, 300*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "collector-000001.jsonl: trace not flushed") {
		t.Fatalf("a held trace with a visible lifetime record was accepted: %v", err)
	}

	tail := encode(record{Stage: "trace.invalid", T: 10, N: map[string]int64{"rejected": 1}})
	if _, err := producer.WriteString(tail); err != nil {
		t.Fatal(err)
	}
	if err := producer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitForTraces(dir, 10*time.Second); err != nil {
		t.Fatalf("closed trace not accepted: %v", err)
	}
	if _, err := readSample(dir); err == nil || !strings.Contains(err.Error(), "trace.invalid") {
		t.Fatalf("completed trace must be rejected by its tail, got %v", err)
	}
}
