package scanruntime

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/perftrace"
)

type traceRecord struct {
	Stage string           `json:"stage"`
	T     int64            `json:"t"`
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
	for _, stage := range []perftrace.Stage{perftrace.UsagePrepare, perftrace.UsagePublication, perftrace.UsagePublicationWait, perftrace.UsageSessionRebuild, perftrace.UsageCommit, perftrace.JobStart, perftrace.IngestRead,
		perftrace.ScanUsage, perftrace.ScanSession, perftrace.ScanDerive, perftrace.PublicationCommit} {
		if got := len(stageRecords(records, string(stage))); got != 1 {
			t.Fatalf("%s markers = %d, want 1 for a single-source append", stage, got)
		}
	}
	// One reduce and one write transaction per domain.
	for _, stage := range []perftrace.Stage{perftrace.IngestReduce, perftrace.IngestWrite} {
		if got := len(stageRecords(records, string(stage))); got != 2 {
			t.Fatalf("%s markers = %d, want one per domain", stage, got)
		}
	}
	if got := len(stageRecords(records, string(perftrace.UsageOrphanScan))); got != 2 {
		t.Fatalf("planning and import orphan checks were not both measured: %d", got)
	}
	served := stageRecords(records, string(perftrace.PublicationCommit))[0]
	if served.N["coverage_complete"] != 1 || served.N["committed"] != served.N["sources"] || served.N["sources"] < 1 {
		t.Fatalf("publication.commit coverage counts = %v", served.N)
	}
	if derive := stageRecords(records, string(perftrace.ScanDerive))[0]; served.T < derive.T || served.T > derive.T+derive.Dur {
		t.Fatal("derive span does not enclose the serving publication")
	}
	publication := stageRecords(records, string(perftrace.UsagePublication))[0]
	commit := stageRecords(records, string(perftrace.UsageCommit))[0]
	enclosed := false
	for _, write := range stageRecords(records, string(perftrace.IngestWrite)) {
		end := write.T + write.Dur
		enclosed = enclosed || (write.T <= publication.T && publication.T+publication.Dur <= end && write.T <= commit.T && commit.T+commit.Dur <= end)
	}
	if !enclosed {
		t.Fatal("no ingest.write transaction encloses the usage publication and its commit")
	}
	if len(txStages(records)) == 0 {
		t.Fatal("no write transaction was recorded")
	}
}

// A caller may stop tracing as soon as the round's result is published, so
// scan.end must already be recorded when it is.
func TestScanEndIsRecordedBeforeTheResultIsPublished(t *testing.T) {
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
	r := newRound(t.TempDir())
	r.onTerminal = func(Result) error { return perftrace.Close() }
	r.executeWith(context.Background(), t.TempDir(), false, func(context.Context, string, string, string, bool) Result {
		return Result{Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "completed"}}
	})
	t.Setenv(perftrace.EnvOut, "")
	raw, err := os.ReadFile(filepath.Join(dir, "collector-000001.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var stages []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		var record traceRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		stages = append(stages, record.Stage)
	}
	if !slices.Contains(stages, string(perftrace.ScanEnd)) {
		t.Fatalf("scan.end was not recorded before publication: %v", stages)
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
	// Two batches, each with its own usage, session and publication, then the
	// final reconciliation, session pass and full publication.
	for stage, want := range map[perftrace.Stage]int{perftrace.ScanUsage: 3, perftrace.ScanSession: 3, perftrace.ScanDerive: 3,
		perftrace.PublicationCommit: 3, perftrace.IngestRead: 1, perftrace.ProjectionDone: 1} {
		if got := len(stageRecords(records, string(stage))); got != want {
			t.Fatalf("%s markers = %d, want %d", stage, got, want)
		}
	}
	commits := stageRecords(records, string(perftrace.PublicationCommit))
	if first, last := commits[0], commits[len(commits)-1]; first.N["coverage_complete"] != 0 || last.N["coverage_complete"] != 1 {
		t.Fatalf("serving publications did not progress from partial to complete: %v %v", first.N, last.N)
	}
	if counts := scanEnd(t, records); counts["read_bytes"] <= 0 || counts["decoded_records"] <= 0 {
		t.Fatalf("serving batches did not share ingest counters: %v", counts)
	}
}
