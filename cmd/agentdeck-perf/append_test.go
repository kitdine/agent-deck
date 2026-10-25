package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/perftrace"
	"github.com/kitdine/agent-deck/internal/scanruntime"
)

func privateTraceDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "perf")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// scanCounts runs one product scan round in-process with tracing and returns
// its scan.end counters.
func scanCounts(t *testing.T, client scanruntime.Client) map[string]int64 {
	t.Helper()
	dir := privateTraceDir(t)
	t.Setenv(perftrace.EnvOut, dir)
	if !perftrace.Start(perftrace.RoleCollector) {
		t.Fatal("tracing did not start")
	}
	if _, err := client.Request(context.Background(), scanruntime.ScopeBoth); err != nil {
		t.Fatal(err)
	}
	if err := perftrace.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv(perftrace.EnvOut, "")
	file, err := os.Open(filepath.Join(dir, "collector-000001.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record traceRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if record.Stage == "scan.end" {
			return record.N
		}
	}
	t.Fatal("no scan.end marker")
	return nil
}

// P08 relies on exact counters: an unchanged rescan reads nothing, and an
// append of one record to every today-related file decodes exactly those
// records, imports their usage, and does so identically on a replay.
func TestAppendDrivesExactIngestCounters(t *testing.T) {
	if testing.Short() {
		t.Skip("generates and scans a 64 MiB fixture")
	}
	opts, manifest := smallFixture(t, 11)
	stateRoot := filepath.Join(t.TempDir(), "state")
	client := scanruntime.Client{StateRoot: stateRoot, Home: opts.out, ForceLocal: true}
	if _, err := client.Request(context.Background(), scanruntime.ScopeBoth); err != nil {
		t.Fatal(err)
	}
	if counts := scanCounts(t, client); counts["read_bytes"] != 0 || counts["decoded_records"] != 0 {
		t.Fatalf("no-op rescan read sources: %v", counts)
	}

	result, err := appendRecords(opts.out, opts.anchor, 11, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != manifest.TodayFiles || result.Records != manifest.TodayFiles {
		t.Fatalf("append touched %d files, want %d today-related files", result.Files, manifest.TodayFiles)
	}
	counts := scanCounts(t, client)
	if counts["decoded_records"] != int64(result.Records) || counts["usage_imported"] != int64(result.Records) {
		t.Fatalf("append counters = %v, want %d decoded and imported records", counts, result.Records)
	}
	if counts["read_bytes"] <= 0 || counts["read_bytes"] > 64<<10 {
		t.Fatalf("append reread more than the appended bytes: %v", counts)
	}
	if again := scanCounts(t, client); again["read_bytes"] != 0 || again["decoded_records"] != 0 {
		t.Fatalf("rescan after append is not a no-op: %v", again)
	}

	single, err := appendRecords(opts.out, opts.anchor, 11, 2, false)
	if err != nil || single.Records != 1 {
		t.Fatalf("single append = %+v %v", single, err)
	}
	// Appends are written now, after the anchor date's fixture records.
	if info, err := os.Stat(opts.out); err != nil || info.ModTime().After(time.Now().Add(time.Minute)) {
		t.Fatalf("fixture root stat: %v", err)
	}
}
