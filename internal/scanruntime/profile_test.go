package scanruntime

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/perftrace"
)

type traceRecord struct {
	Stage string           `json:"stage"`
	Dur   int64            `json:"dur"`
	N     map[string]int64 `json:"n"`
}

// traced runs fn with measurement recording on and returns its records.
func traced(t *testing.T, fn func()) []traceRecord {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "perf")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(perftrace.EnvOut, dir)
	if !perftrace.Start(perftrace.RoleCollector) {
		t.Fatal("tracing did not start")
	}
	fn()
	if err := perftrace.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv(perftrace.EnvOut, "")
	file, err := os.Open(filepath.Join(dir, "collector-000001.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var records []traceRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record traceRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if record.Stage == "trace.invalid" {
			t.Fatalf("scan produced a rejected marker: %s", scanner.Text())
		}
		records = append(records, record)
	}
	return records
}

func stageRecords(records []traceRecord, stage string) []traceRecord {
	var out []traceRecord
	for _, record := range records {
		if record.Stage == stage {
			out = append(out, record)
		}
	}
	return out
}

func scanEnd(t *testing.T, records []traceRecord) map[string]int64 {
	t.Helper()
	ends := stageRecords(records, string(perftrace.ScanEnd))
	if len(ends) != 1 {
		t.Fatalf("scan.end markers = %d, want 1", len(ends))
	}
	return ends[0].N
}

func TestIngestionCountersMeasureActualRereads(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	directory := filepath.Join(home, ".codex", "sessions")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(directory, "profile.jsonl")
	contents := []byte(`{"type":"session_meta","payload":{"id":"profile","cwd":"/synthetic"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"profile-turn","model":"gpt-5"}}` + "\n" +
		`{"type":"event_msg","timestamp":"2026-10-09T12:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10},"total_token_usage":{"input_tokens":10}}}}` + "\n")
	if err := os.WriteFile(source, contents, 0600); err != nil {
		t.Fatal(err)
	}
	client := Client{StateRoot: state, Home: home, ForceLocal: true, Now: func() time.Time { return time.Date(2026, 10, 9, 13, 0, 0, 0, time.UTC) }}
	t.Setenv(perftrace.EnvOut, "")
	first, err := client.Request(context.Background(), ScopeBoth)
	if err != nil {
		t.Fatal(err)
	}
	if first.Usage.Changes["imported"] != 1 {
		t.Fatalf("fixture did not import usage: %#v", first.Usage.Changes)
	}

	records := traced(t, func() {
		if _, err := client.Request(context.Background(), ScopeBoth); err != nil {
			t.Fatal(err)
		}
	})
	if counts := scanEnd(t, records); counts["read_bytes"] != 0 || counts["decoded_records"] != 0 || counts["usage_imported"] != 0 {
		t.Fatalf("unchanged scan reread sources: %v", counts)
	}

	file, err := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	delta := []byte(`{"type":"event_msg","timestamp":"2026-10-09T12:01:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":5},"total_token_usage":{"input_tokens":15}}}}` + "\n")
	if _, err = file.Write(delta); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	var updated Result
	records = traced(t, func() {
		if updated, err = client.Request(context.Background(), ScopeBoth); err != nil {
			t.Fatal(err)
		}
	})
	counts := scanEnd(t, records)
	if counts["read_bytes"] <= 0 || counts["read_bytes"] > int64(len(contents)+len(delta))*2 {
		t.Fatalf("append read accounting is not bounded by the source: %v", counts)
	}
	if updated.Usage.State != "completed" || updated.Session.State != "completed" {
		t.Fatalf("tracing changed outcomes: %#v", updated)
	}
	if updated.Usage.Changes["imported"] != 1 || counts["usage_imported"] != 1 {
		t.Fatalf("append did not import exactly one usage event: %#v %v", updated.Usage.Changes, counts)
	}
	for _, stage := range []perftrace.Stage{perftrace.UsageSource, perftrace.UsagePublication, perftrace.UsagePublicationWait, perftrace.UsageSessionRebuild, perftrace.UsageCommit, perftrace.JobStart, perftrace.IngestRead} {
		if got := len(stageRecords(records, string(stage))); got != 1 {
			t.Fatalf("%s markers = %d, want 1 for a single-source append", stage, got)
		}
	}
	if got := len(stageRecords(records, string(perftrace.UsageOrphanScan))); got != 2 {
		t.Fatalf("planning and import orphan checks were not both measured: %d", got)
	}
	publication := stageRecords(records, string(perftrace.UsagePublication))[0]
	commit := stageRecords(records, string(perftrace.UsageCommit))[0]
	if publication.Dur < commit.Dur {
		t.Fatal("publication duration excluded commit")
	}
	if len(txStages(records)) == 0 {
		t.Fatal("no write transaction was recorded")
	}
}

func txStages(records []traceRecord) []string {
	var out []string
	for _, record := range records {
		if len(record.Stage) > 3 && record.Stage[:3] == "tx." {
			out = append(out, record.Stage)
		}
	}
	return out
}

func TestServingRoundRecordsEveryBatch(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	writeServingSource(t, home, "today", now.Add(-time.Minute), 17)
	history := writeServingSource(t, home, "history", now.AddDate(0, 0, -20), 23)
	old := now.AddDate(-1, 0, 0)
	if err := os.Chtimes(history, old, old); err != nil {
		t.Fatal(err)
	}
	records := traced(t, func() {
		r := newRound(home)
		r.serving = true
		r.cacheNow = func() time.Time { return now }
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		root, _, err := prepareStateRoot(filepath.Join(home, ".agentdeck"))
		if err != nil {
			t.Fatal(err)
		}
		r.execute(ctx, root, false)
		result, err := r.wait(ctx, ScopeBoth)
		if err != nil || result.Usage.State != "completed" || result.Session.State != "completed" {
			t.Fatalf("traced serving round failed: %+v %v", result, err)
		}
	})
	if got := len(stageRecords(records, string(perftrace.UsageCommit))); got != 2 {
		t.Fatalf("usage.commit markers = %d, want one per batch", got)
	}
	if got := len(stageRecords(records, string(perftrace.UsageOrphanCleanup))); got != 1 {
		t.Fatalf("usage.orphan_cleanup markers = %d, want 1 final reconciliation", got)
	}
	if counts := scanEnd(t, records); counts["read_bytes"] <= 0 || counts["decoded_records"] <= 0 {
		t.Fatalf("serving batches did not share ingest counters: %v", counts)
	}
}
