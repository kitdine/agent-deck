package scanruntime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIngestionProfileIsOptInAndMeasuresActualRereads(t *testing.T) {
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
	t.Setenv("AGENTDECK_SCAN_PROFILE", "")
	first, err := client.Request(context.Background(), ScopeBoth)
	if err != nil {
		t.Fatal(err)
	}
	if first.Stages.Ingestion != nil {
		t.Fatal("default scan exposed opt-in metrics")
	}
	if first.Stages.UsageDetails != nil {
		t.Fatal("default scan exposed usage diagnostics")
	}
	if first.Usage.Changes["imported"] != 1 {
		t.Fatalf("fixture did not import usage: %#v", first.Usage.Changes)
	}
	t.Setenv("AGENTDECK_SCAN_PROFILE", "1")
	unchanged, err := client.Request(context.Background(), ScopeBoth)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Stages.Ingestion == nil || unchanged.Stages.Ingestion["read_bytes"] != 0 {
		t.Fatalf("unchanged scan reread sources: %#v", unchanged.Stages.Ingestion)
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
	updated, err := client.Request(context.Background(), ScopeBoth)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Stages.Ingestion["read_bytes"] <= 0 || updated.Stages.Ingestion["read_bytes"] > float64(len(contents)+len(delta))*2 {
		t.Fatalf("append read accounting is not bounded by the source: %#v", updated.Stages.Ingestion)
	}
	if updated.Usage.State != "completed" || updated.Session.State != "completed" {
		t.Fatalf("profiling changed outcomes: %#v", updated)
	}
	if updated.Usage.Changes["imported"] != 1 {
		t.Fatalf("append did not import a new usage event: %#v", updated.Usage.Changes)
	}
	for _, stage := range []string{"source_total", "source_publication", "publication_lock_wait", "session_rebuild", "source_commit"} {
		value := updated.Stages.UsageDetails[stage]
		if value.Calls != 1 || value.TotalMS < 0 || value.MaxMS != value.TotalMS {
			t.Fatalf("invalid single-source %s timing: %#v", stage, value)
		}
	}
	if updated.Stages.UsageDetails["orphan_candidates"].Calls != 2 {
		t.Fatal("planning and import orphan checks were not both measured")
	}
	if updated.Stages.UsageDetails["source_publication"].TotalMS < updated.Stages.UsageDetails["source_commit"].TotalMS {
		t.Fatal("publication duration excluded commit")
	}
}

func TestServingProfileAggregatesAcrossPriorityAndHistoryBatches(t *testing.T) {
	t.Setenv("AGENTDECK_SCAN_PROFILE", "1")
	home := t.TempDir()
	now := time.Now()
	writeServingSource(t, home, "today", now.Add(-time.Minute), 17)
	history := writeServingSource(t, home, "history", now.AddDate(0, 0, -20), 23)
	old := now.AddDate(-1, 0, 0)
	if err := os.Chtimes(history, old, old); err != nil {
		t.Fatal(err)
	}
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
		t.Fatalf("profiled serving round failed: %+v %v", result, err)
	}
	if result.Stages.UsageDetails["source_commit"].Calls != 2 || result.Stages.UsageDetails["orphan_cleanup"].Calls != 1 {
		t.Fatalf("batch aggregation or final reconciliation missing: %+v", result.Stages.UsageDetails)
	}
}
