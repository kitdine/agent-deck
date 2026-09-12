package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/scanruntime"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

var snapshotPerformanceNow = time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)

type snapshotPerformanceCorpus struct {
	Home, Codex, Claude string
}

func TestUnifiedScanRuntimeMatchesLegacyAcrossSourceMutations(t *testing.T) {
	corpus := writeSnapshotPerformanceCorpus(t)
	legacyState := newSnapshotPerformanceState(t)
	sharedState := newSnapshotPerformanceState(t)
	assertEquivalent := func(stage string) {
		t.Helper()
		if err := scanSnapshotPerformanceDomains(context.Background(), legacyState, corpus.Home, false); err != nil {
			t.Fatalf("%s legacy scan: %v", stage, err)
		}
		if err := scanSnapshotPerformanceDomains(context.Background(), sharedState, corpus.Home, true); err != nil {
			t.Fatalf("%s shared scan: %v", stage, err)
		}
		legacy := captureSnapshotPerformanceRows(t, legacyState)
		shared := captureSnapshotPerformanceRows(t, sharedState)
		if !reflect.DeepEqual(shared, legacy) {
			t.Fatalf("%s shared rows differ from legacy\nshared=%#v\nlegacy=%#v", stage, shared, legacy)
		}
	}

	assertEquivalent("initial")
	file, err := os.OpenFile(corpus.Codex, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString(`{"type":"visible_user_prompt","session_id":"partial","payload":{"text":"split`); err != nil {
		t.Fatal(err)
	}
	file.Close()
	assertEquivalent("partial append")
	file, err = os.OpenFile(corpus.Codex, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString(" record" + `"}}` + "\n"); err != nil {
		t.Fatal(err)
	}
	file.Close()
	assertEquivalent("completed append")

	contents, err := os.ReadFile(corpus.Codex)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := bytes.Replace(contents, []byte("synthetic answer"), []byte("synthetic reply!"), 1)
	if len(rewritten) != len(contents) {
		t.Fatal("same-size rewrite fixture changed length")
	}
	if err = os.WriteFile(corpus.Codex, rewritten, 0o600); err != nil {
		t.Fatal(err)
	}
	assertEquivalent("same-size rewrite")

	renamed := filepath.Join(filepath.Dir(corpus.Codex), "renamed.jsonl")
	if err = os.Rename(corpus.Codex, renamed); err != nil {
		t.Fatal(err)
	}
	corpus.Codex = renamed
	assertEquivalent("rename")

	duplicate := filepath.Join(corpus.Home, ".codex", "archived_sessions", "duplicate.jsonl")
	if err = os.MkdirAll(filepath.Dir(duplicate), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(duplicate, rewritten, 0o600); err != nil {
		t.Fatal(err)
	}
	assertEquivalent("duplicate")

	before := captureSnapshotPerformanceRows(t, sharedState)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err = scanSnapshotPerformanceDomains(cancelled, sharedState, corpus.Home, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("detached shared scan error=%v", err)
	}
	if err = scanSnapshotPerformanceDomains(context.Background(), sharedState, corpus.Home, true); err != nil {
		t.Fatalf("detached shared scan did not reach a terminal result: %v", err)
	}
	after := captureSnapshotPerformanceRows(t, sharedState)
	if !reflect.DeepEqual(after, before) {
		t.Fatal("detached shared scan did not preserve the completed domain state")
	}
}

func TestUnifiedScanRuntimeReleasesBudgetForChangedSourceAfterUnchangedSource(t *testing.T) {
	corpus := writeSnapshotPerformanceCorpus(t)
	file, err := os.OpenFile(corpus.Claude, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString(strings.Repeat(" ", 1<<20) + "{}\n"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}

	legacyState := newSnapshotPerformanceState(t)
	sharedState := newSnapshotPerformanceState(t)
	for name, state := range map[string]string{"legacy": legacyState, "shared": sharedState} {
		if err = scanSnapshotPerformanceDomains(context.Background(), state, corpus.Home, name == "shared"); err != nil {
			t.Fatalf("%s initial scan: %v", name, err)
		}
	}
	appendSnapshotPerformanceChange(t, corpus)
	if err = scanSnapshotPerformanceDomains(context.Background(), legacyState, corpus.Home, false); err != nil {
		t.Fatalf("legacy incremental scan: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err = scanSnapshotPerformanceDomains(ctx, sharedState, corpus.Home, true); err != nil {
		t.Fatalf("shared incremental scan with an unchanged leading source: %v", err)
	}
	legacy := captureSnapshotPerformanceRows(t, legacyState)
	shared := captureSnapshotPerformanceRows(t, sharedState)
	if !reflect.DeepEqual(shared, legacy) {
		t.Fatalf("shared incremental rows differ from legacy\nshared=%#v\nlegacy=%#v", shared, legacy)
	}

	beforeCancellation := captureSnapshotPerformanceRows(t, sharedState)
	cancelled, cancelImmediately := context.WithCancel(context.Background())
	cancelImmediately()
	if err = scanSnapshotPerformanceDomains(cancelled, sharedState, corpus.Home, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("detached shared scan error=%v", err)
	}
	if err = scanSnapshotPerformanceDomains(context.Background(), sharedState, corpus.Home, true); err != nil {
		t.Fatalf("detached shared scan did not reach a terminal result: %v", err)
	}
	afterCancellation := captureSnapshotPerformanceRows(t, sharedState)
	if !reflect.DeepEqual(afterCancellation, beforeCancellation) {
		t.Fatal("detached shared scan did not preserve the completed domain state")
	}
}

func TestUnifiedScanRuntimeRecordsStageProfile(t *testing.T) {
	corpus := writeSnapshotPerformanceCorpus(t)
	state := newSnapshotPerformanceState(t)
	started := time.Now()
	result, err := (scanruntime.Client{StateRoot: state, Home: corpus.Home, ForceLocal: true}).Request(context.Background(), scanruntime.ScopeBoth)
	if err != nil {
		t.Fatal(err)
	}
	if err = result.ErrorFor(scanruntime.ScopeBoth); err != nil {
		t.Fatal(err)
	}
	if result.Stages.DiscoveryMS < 0 || result.Stages.UsageMS < 0 || result.Stages.SessionMS < 0 || result.Stages.TotalMS < 0 {
		t.Fatalf("negative worker stage profile: %#v", result.Stages)
	}
	if result.Stages.TotalMS > time.Since(started).Milliseconds()+1000 {
		t.Fatalf("worker total stage profile exceeds observed wall time: %#v", result.Stages)
	}
	t.Logf("worker stage profile: discovery=%dms usage=%dms session=%dms total=%dms", result.Stages.DiscoveryMS, result.Stages.UsageMS, result.Stages.SessionMS, result.Stages.TotalMS)
}

func scanSnapshotPerformanceDomains(ctx context.Context, state, home string, shared bool) error {
	if shared {
		result, err := (scanruntime.Client{StateRoot: state, Home: home, ForceLocal: true}).Request(ctx, scanruntime.ScopeBoth)
		if err != nil {
			return err
		}
		return result.ErrorFor(scanruntime.ScopeBoth)
	}
	core, err := store.Open(ctx, state)
	if err != nil {
		return err
	}
	defer core.Close()
	sessions, err := store.OpenSessions(ctx, state)
	if err != nil {
		return err
	}
	defer sessions.Close()
	usageService := usage.New(core, home)
	if _, err = usageService.Scan(ctx); err != nil {
		return err
	}
	_, err = session.Scan(ctx, sessions.DB, home)
	return err
}

type snapshotPerformanceReference struct {
	Payload []byte
	Result  desktop.Result
	Rows    map[string][]string
}

type snapshotPerformanceReport struct {
	SchemaVersion int                          `json:"schema_version"`
	GeneratedAt   string                       `json:"generated_at"`
	Hardware      snapshotPerformanceHardware  `json:"hardware"`
	Toolchain     string                       `json:"toolchain"`
	Corpus        snapshotPerformanceCorpusID  `json:"corpus"`
	FixedTime     string                       `json:"fixed_time"`
	Timezone      string                       `json:"timezone"`
	DeadlineMS    int64                        `json:"deadline_ms"`
	Method        string                       `json:"method"`
	Samples       []snapshotPerformanceSample  `json:"samples"`
	Summaries     []snapshotPerformanceSummary `json:"summaries"`
}

type snapshotPerformanceHardware struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	LogicalCPUs  int    `json:"logical_cpus"`
	CPU          string `json:"cpu,omitempty"`
	MemoryBytes  int64  `json:"memory_bytes,omitempty"`
}

type snapshotPerformanceCorpusID struct {
	SHA256 string `json:"sha256"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
}

type snapshotPerformanceSample struct {
	Scenario            string `json:"scenario"`
	Index               int    `json:"index"`
	Outcome             string `json:"outcome"`
	Complete            bool   `json:"complete"`
	WithinTarget        bool   `json:"within_target"`
	TargetMS            int64  `json:"target_ms"`
	WallMS              int64  `json:"wall_ms"`
	CPUTimeMS           int64  `json:"cpu_time_ms"`
	PeakRSSBytes        int64  `json:"peak_rss_bytes"`
	ProcessStartupMS    int64  `json:"process_startup_ms"`
	UsageRefreshMS      int64  `json:"usage_refresh_ms"`
	SessionRefreshMS    int64  `json:"session_refresh_ms"`
	VerificationMS      int64  `json:"verification_ms"`
	SnapshotSHA256      string `json:"snapshot_sha256,omitempty"`
	LogicalRowsSHA256   string `json:"logical_rows_sha256,omitempty"`
	UsageErrorCode      string `json:"usage_error_code,omitempty"`
	SessionErrorCode    string `json:"session_error_code,omitempty"`
	UsageFailureStage   string `json:"usage_failure_stage,omitempty"`
	SessionFailureStage string `json:"session_failure_stage,omitempty"`
	OSCacheState        string `json:"os_cache_state"`
	ExternalLoad        string `json:"external_load"`
}

type snapshotPerformanceHelperResult struct {
	Action              string `json:"action"`
	ExitCode            int    `json:"exit_code"`
	CommandMS           int64  `json:"command_ms"`
	StdoutBase64        string `json:"stdout_base64,omitempty"`
	StderrSHA256        string `json:"stderr_sha256,omitempty"`
	UsageRefreshMS      int64  `json:"usage_refresh_ms,omitempty"`
	SessionRefreshMS    int64  `json:"session_refresh_ms,omitempty"`
	UsageErrorCode      string `json:"usage_error_code,omitempty"`
	SessionErrorCode    string `json:"session_error_code,omitempty"`
	UsageFailureStage   string `json:"usage_failure_stage,omitempty"`
	SessionFailureStage string `json:"session_failure_stage,omitempty"`
}

type snapshotPerformanceProcessResult struct {
	Helper    snapshotPerformanceHelperResult
	Outcome   string
	CPUTimeMS int64
	PeakRSS   int64
	StartupMS int64
}

type snapshotPerformanceSummary struct {
	Scenario            string `json:"scenario"`
	Samples             int    `json:"samples"`
	MedianMS            int64  `json:"median_ms"`
	P95MS               int64  `json:"p95_ms"`
	MaximumMS           int64  `json:"maximum_ms"`
	MaximumCPUTimeMS    int64  `json:"maximum_cpu_time_ms"`
	MaximumPeakRSSBytes int64  `json:"maximum_peak_rss_bytes"`
	AllComplete         bool   `json:"all_complete"`
	AllWithinTarget     bool   `json:"all_within_target"`
}

func TestSnapshotPerformanceContractSyntheticCorpus(t *testing.T) {
	corpus := writeSnapshotPerformanceCorpus(t)
	state := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}

	cold, partial, warnings, err := refreshDesktopIndexes(context.Background(), state, corpus.Home)
	if err != nil {
		t.Fatalf("cold refresh: %v", err)
	}
	assertCompleteDesktopRefresh(t, "cold", cold, partial, warnings)
	reference := captureSnapshotPerformanceReference(t, state, corpus.Home)
	for name, rows := range reference.Rows {
		if len(rows) == 0 {
			t.Fatalf("fixed corpus did not exercise required logical table %s", name)
		}
	}
	assertSnapshotPerformanceGolden(t, reference.Payload)

	unchanged, partial, warnings, err := refreshDesktopIndexes(context.Background(), state, corpus.Home)
	if err != nil {
		t.Fatalf("unchanged refresh: %v", err)
	}
	assertCompleteDesktopRefresh(t, "unchanged", unchanged, partial, warnings)
	got := captureSnapshotPerformanceReference(t, state, corpus.Home)
	if !bytes.Equal(got.Payload, reference.Payload) {
		t.Fatalf("unchanged complete serialized snapshot differs at %s", firstByteDifference(reference.Payload, got.Payload))
	}
	if !reflect.DeepEqual(got.Result, reference.Result) {
		t.Fatal("unchanged snapshot differs in internal fields omitted from JSON")
	}
	if !reflect.DeepEqual(got.Rows, reference.Rows) {
		t.Fatalf("unchanged refresh changed logical rows\nbefore=%#v\nafter=%#v", reference.Rows, got.Rows)
	}

	appendSnapshotPerformanceChange(t, corpus)
	changed, partial, warnings, err := refreshDesktopIndexes(context.Background(), state, corpus.Home)
	if err != nil {
		t.Fatalf("changed refresh: %v", err)
	}
	assertCompleteDesktopRefresh(t, "changed", changed, partial, warnings)
	afterChange := captureSnapshotPerformanceReference(t, state, corpus.Home)
	if bytes.Equal(afterChange.Payload, reference.Payload) {
		t.Fatal("changed input left the complete serialized snapshot unchanged")
	}
	if reflect.DeepEqual(afterChange.Rows, reference.Rows) {
		t.Fatal("changed input left every relevant logical table unchanged")
	}
	if len(afterChange.Result.Snapshot.Sessions.Items) <= len(reference.Result.Snapshot.Sessions.Items) {
		t.Fatalf("changed session input was not published: before=%d after=%d", len(reference.Result.Snapshot.Sessions.Items), len(afterChange.Result.Snapshot.Sessions.Items))
	}
}

func TestIngestionStageProfile(t *testing.T) {
	corpus := os.Getenv("AGENTDECK_INGEST_STAGE_CORPUS")
	out := os.Getenv("AGENTDECK_INGEST_STAGE_REPORT")
	if corpus == "" || out == "" {
		t.Skip("isolated corpus and report not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	started := time.Now()
	result, err := (scanruntime.Client{StateRoot: newSnapshotPerformanceState(t), Home: corpus, ForceLocal: true}).Request(ctx, scanruntime.ScopeBoth)
	if err != nil {
		t.Fatal(err)
	}
	if err = result.ErrorFor(scanruntime.ScopeBoth); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	report := map[string]float64{
		"discovery_ms":   float64(result.Stages.DiscoveryMS),
		"usage_ms":       float64(result.Stages.UsageMS),
		"session_ms":     float64(result.Stages.SessionMS),
		"worker_wall_ms": float64(result.Stages.TotalMS),
		"wall_ms":        float64(elapsed) / float64(time.Millisecond),
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(out, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotPerformanceRepresentativeCorpus(t *testing.T) {
	if os.Getenv("AGENTDECK_SNAPSHOT_PERFORMANCE_WORKER") == "1" {
		runSnapshotPerformanceWorker(t)
		return
	}
	corpus := os.Getenv("AGENTDECK_SNAPSHOT_PERFORMANCE_CORPUS")
	reportPath := os.Getenv("AGENTDECK_SNAPSHOT_PERFORMANCE_REPORT")
	if corpus == "" || reportPath == "" {
		t.Skip("set AGENTDECK_SNAPSHOT_PERFORMANCE_CORPUS and AGENTDECK_SNAPSHOT_PERFORMANCE_REPORT")
	}
	info, err := os.Stat(corpus)
	if err != nil || !info.IsDir() {
		t.Fatalf("representative corpus: %v", err)
	}
	sampleCount := 1
	if raw := os.Getenv("AGENTDECK_SNAPSHOT_PERFORMANCE_SAMPLES"); raw != "" {
		sampleCount, err = strconv.Atoi(raw)
		if err != nil || sampleCount < 1 {
			t.Fatalf("invalid sample count %q", raw)
		}
	}
	deadline := 5 * time.Minute
	if raw := os.Getenv("AGENTDECK_SNAPSHOT_PERFORMANCE_DEADLINE"); raw != "" {
		deadline, err = time.ParseDuration(raw)
		if err != nil || deadline <= 0 {
			t.Fatalf("invalid deadline %q", raw)
		}
	}
	report := snapshotPerformanceReport{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		Hardware:      snapshotPerformanceHardwareInfo(t),
		Toolchain:     runtime.Version(),
		Corpus:        snapshotPerformanceCorpusDigest(t, corpus),
		FixedTime:     snapshotPerformanceNow.Format(time.RFC3339Nano),
		Timezone:      "UTC",
		DeadlineMS:    deadline.Milliseconds(),
		Method:        "two fresh CLI helper subprocesses per sample; end-to-end wall includes both command initializations, refresh-indexes, streamed snapshot and parent decode; logical-row verification is timed separately",
	}
	var completedColdState string
	for _, scenario := range []string{"cold_import", "unchanged_refresh"} {
		var reusableState string
		var setupErr error
		if scenario == "unchanged_refresh" {
			reusableState = completedColdState
			if reusableState == "" {
				setupErr = errors.New("no completed cold import available for unchanged measurement")
			}
		}
		for index := 1; index <= sampleCount; index++ {
			state := reusableState
			if scenario == "cold_import" {
				state = newSnapshotPerformanceState(t)
			}
			if setupErr != nil {
				report.Samples = append(report.Samples, snapshotPerformanceSample{Scenario: scenario, Index: index, Outcome: "setup_failure", ExternalLoad: "uncontrolled", OSCacheState: "uncontrolled"})
				continue
			}
			sample := runSnapshotPerformanceSample(scenario, index, corpus, state, deadline)
			report.Samples = append(report.Samples, sample)
			if scenario == "cold_import" && sample.Complete {
				completedColdState = state
			}
			report.Summaries = summarizeSnapshotPerformance(report.Samples)
			if err = writeSnapshotPerformanceReport(reportPath, report); err != nil {
				t.Fatalf("persist sample: %v", err)
			}
		}
	}
	report.Summaries = summarizeSnapshotPerformance(report.Samples)
	if err = writeSnapshotPerformanceReport(reportPath, report); err != nil {
		t.Fatalf("write private report: %v", err)
	}
	t.Logf("snapshot performance report: %s", reportPath)
	for _, sample := range report.Samples {
		if !sample.Complete {
			t.Errorf("%s sample %d incomplete: %s", sample.Scenario, sample.Index, sample.Outcome)
		}
	}
}

func writeSnapshotPerformanceReport(path string, report snapshotPerformanceReport) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o600)
}

func TestSnapshotPerformanceSummaryKeepsEverySampleAndWorstResources(t *testing.T) {
	summaries := summarizeSnapshotPerformance([]snapshotPerformanceSample{
		{Scenario: "cold_import", WallMS: 30, CPUTimeMS: 50, PeakRSSBytes: 300, Complete: true, WithinTarget: true},
		{Scenario: "cold_import", WallMS: 10, CPUTimeMS: 20, PeakRSSBytes: 100, Complete: true, WithinTarget: true},
		{Scenario: "cold_import", WallMS: 20, CPUTimeMS: 40, PeakRSSBytes: 200, Complete: false, WithinTarget: false},
	})
	if len(summaries) != 1 {
		t.Fatalf("summaries = %#v", summaries)
	}
	got := summaries[0]
	if got.Samples != 3 || got.MedianMS != 20 || got.P95MS != 30 || got.MaximumMS != 30 ||
		got.MaximumCPUTimeMS != 50 || got.MaximumPeakRSSBytes != 300 || got.AllComplete || got.AllWithinTarget {
		t.Fatalf("summary = %#v", got)
	}
}

func TestSnapshotPerformanceTargetUsesEndToEndWallAndExcludesVerification(t *testing.T) {
	sample := snapshotPerformanceSample{
		Scenario: "unchanged_refresh", Complete: true, TargetMS: 1000,
		WallMS: 1200, CPUTimeMS: 100, PeakRSSBytes: 20 * 1024 * 1024,
		VerificationMS: 500,
	}
	applySnapshotPerformanceTarget(&sample)
	if sample.WithinTarget {
		t.Fatal("900 ms internal work inside a 1200 ms helper cycle was incorrectly accepted against the 1000 ms end-to-end target")
	}
	sample.WallMS = 900
	applySnapshotPerformanceTarget(&sample)
	if !sample.WithinTarget {
		t.Fatal("verification outside the measured helper cycle incorrectly affected target status")
	}
}

func TestSnapshotPerformanceFailuresAreBoundedAndReportable(t *testing.T) {
	corpus := writeSnapshotPerformanceCorpus(t)
	state := newSnapshotPerformanceState(t)
	result, partial, warnings, err := refreshDesktopIndexes(context.Background(), state, corpus.Home)
	if err != nil {
		t.Fatal(err)
	}
	assertCompleteDesktopRefresh(t, "failure fixture setup", result, partial, warnings)

	t.Setenv("AGENTDECK_SNAPSHOT_PERFORMANCE_TEST_BEHAVIOR", "hang_snapshot")
	timedOut := runSnapshotPerformanceSample("unchanged_refresh", 1, corpus.Home, state, 500*time.Millisecond)
	if timedOut.Complete || timedOut.Outcome != "measurement_deadline" || timedOut.WallMS > 2000 {
		t.Fatalf("timeout sample = %#v", timedOut)
	}

	t.Setenv("AGENTDECK_SNAPSHOT_PERFORMANCE_TEST_BEHAVIOR", "missing_snapshot")
	missing := runSnapshotPerformanceSample("unchanged_refresh", 2, corpus.Home, state, 5*time.Second)
	if missing.Complete || missing.Outcome != "missing_result" {
		t.Fatalf("missing-result sample = %#v", missing)
	}

	reportPath := filepath.Join(t.TempDir(), "failed-report.json")
	report := snapshotPerformanceReport{SchemaVersion: 1, Samples: []snapshotPerformanceSample{
		{Scenario: "unchanged_refresh", Index: 0, Outcome: "complete", Complete: true},
		timedOut,
		missing,
	}}
	if err = writeSnapshotPerformanceReport(reportPath, report); err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved snapshotPerformanceReport
	if err = json.Unmarshal(encoded, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Samples) != 3 || saved.Samples[0].Outcome != "complete" || saved.Samples[1].Outcome != "measurement_deadline" || saved.Samples[2].Outcome != "missing_result" {
		t.Fatalf("saved failure report = %#v", saved.Samples)
	}
}

func runSnapshotPerformanceWorker(t *testing.T) {
	action := os.Getenv("AGENTDECK_SNAPSHOT_PERFORMANCE_ACTION")
	behavior := os.Getenv("AGENTDECK_SNAPSHOT_PERFORMANCE_TEST_BEHAVIOR")
	if behavior == "hang_snapshot" && action == "snapshot" {
		time.Sleep(10 * time.Second)
	}
	if behavior == "missing_snapshot" && action == "snapshot" {
		return
	}
	home := os.Getenv("AGENTDECK_SNAPSHOT_PERFORMANCE_CORPUS")
	state := os.Getenv("AGENTDECK_SNAPSHOT_PERFORMANCE_STATE")
	oldHome, oldNow, oldObserver := userHomeDir, desktopNow, desktopIndexRefreshObserver
	userHomeDir = func() (string, error) { return home, nil }
	desktopNow = func() time.Time { return snapshotPerformanceNow }
	var observed desktopIndexRefreshResult
	desktopIndexRefreshObserver = func(result desktopIndexRefreshResult) { observed = result }
	t.Cleanup(func() {
		userHomeDir, desktopNow, desktopIndexRefreshObserver = oldHome, oldNow, oldObserver
	})
	args := []string{"--state-dir", state, "--format", "json", "desktop"}
	switch action {
	case "refresh-indexes":
		args = append(args, "refresh-indexes")
	case "snapshot":
		args = append(args, "snapshot", "--wire-version", "1", "--recent-limit", "20", "--stream")
	default:
		t.Fatalf("unknown helper action %q", action)
	}
	var stdout, stderr bytes.Buffer
	started := time.Now()
	exit := execute(args, bytes.NewReader(nil), &stdout, &stderr)
	result := snapshotPerformanceHelperResult{
		Action: action, ExitCode: exit, CommandMS: time.Since(started).Milliseconds(),
		StdoutBase64:        base64.StdEncoding.EncodeToString(stdout.Bytes()),
		UsageRefreshMS:      observed.Usage.DurationMilliseconds,
		SessionRefreshMS:    observed.Sessions.DurationMilliseconds,
		UsageErrorCode:      observed.Usage.ErrorCode,
		SessionErrorCode:    observed.Sessions.ErrorCode,
		UsageFailureStage:   observed.Usage.failureStage,
		SessionFailureStage: observed.Sessions.failureStage,
	}
	if stderr.Len() > 0 {
		digest := sha256.Sum256(stderr.Bytes())
		result.StderrSHA256 = hex.EncodeToString(digest[:])
	}
	line, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("SNAPSHOT_PERFORMANCE_HELPER=%s\n", line)
}

func runSnapshotPerformanceSample(scenario string, index int, corpus, state string, deadline time.Duration) snapshotPerformanceSample {
	sample := snapshotPerformanceSample{Scenario: scenario, Index: index, Outcome: "complete", OSCacheState: "uncontrolled", ExternalLoad: "uncontrolled"}
	if scenario == "unchanged_refresh" {
		sample.TargetMS = 1000
	} else {
		sample.TargetMS = 10000
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	cycleStarted := time.Now()
	refresh := runSnapshotPerformanceHelper(ctx, "refresh-indexes", corpus, state)
	mergeSnapshotPerformanceProcess(&sample, refresh)
	if refresh.Outcome != "complete" || refresh.Helper.ExitCode != 0 {
		sample.Outcome = refresh.Outcome
		if sample.Outcome == "complete" {
			sample.Outcome = "refresh_error"
		}
		sample.WallMS = time.Since(cycleStarted).Milliseconds()
		applySnapshotPerformanceTarget(&sample)
		return sample
	}
	sample.UsageRefreshMS = refresh.Helper.UsageRefreshMS
	sample.SessionRefreshMS = refresh.Helper.SessionRefreshMS
	sample.UsageErrorCode = refresh.Helper.UsageErrorCode
	sample.SessionErrorCode = refresh.Helper.SessionErrorCode
	sample.UsageFailureStage = refresh.Helper.UsageFailureStage
	sample.SessionFailureStage = refresh.Helper.SessionFailureStage
	if refresh.Helper.UsageFailureStage == "checkpoint_persistence" || refresh.Helper.SessionFailureStage == "checkpoint_persistence" {
		sample.Outcome = "post_scan_persistence_failure"
	} else if refresh.Helper.UsageFailureStage == "deadline" || refresh.Helper.SessionFailureStage == "deadline" {
		sample.Outcome = "measurement_deadline"
	} else if refresh.Helper.UsageFailureStage == "scan" || refresh.Helper.SessionFailureStage == "scan" {
		sample.Outcome = "scan_or_parser_failure"
	}
	if sample.Outcome != "complete" {
		sample.WallMS = time.Since(cycleStarted).Milliseconds()
		applySnapshotPerformanceTarget(&sample)
		return sample
	}
	snapshot := runSnapshotPerformanceHelper(ctx, "snapshot", corpus, state)
	mergeSnapshotPerformanceProcess(&sample, snapshot)
	if snapshot.Outcome != "complete" || snapshot.Helper.ExitCode != 0 {
		sample.Outcome = snapshot.Outcome
		if sample.Outcome == "complete" {
			sample.Outcome = "snapshot_error"
		}
		sample.WallMS = time.Since(cycleStarted).Milliseconds()
		applySnapshotPerformanceTarget(&sample)
		return sample
	}
	stream, err := base64.StdEncoding.DecodeString(snapshot.Helper.StdoutBase64)
	if err != nil {
		sample.Outcome = "snapshot_output_invalid"
	} else {
		payload, partial, decodeErr := decodeSnapshotPerformanceStreamValue(stream)
		if decodeErr != nil {
			sample.Outcome = "snapshot_output_invalid"
		} else if partial {
			sample.Outcome = "snapshot_partial"
		} else {
			digest := sha256.Sum256(payload)
			sample.SnapshotSHA256 = hex.EncodeToString(digest[:])
		}
	}
	sample.WallMS = time.Since(cycleStarted).Milliseconds()
	if sample.Outcome == "complete" {
		verificationStarted := time.Now()
		rowsDigest, rowsErr := snapshotPerformanceRowsDigest(ctx, state)
		sample.VerificationMS = time.Since(verificationStarted).Milliseconds()
		if rowsErr != nil {
			sample.Outcome = "logical_rows_verification_failure"
			if ctx.Err() != nil {
				sample.Outcome = "verification_deadline"
			}
		} else {
			sample.LogicalRowsSHA256 = rowsDigest
			sample.Complete = true
		}
	}
	applySnapshotPerformanceTarget(&sample)
	return sample
}

func runSnapshotPerformanceHelper(ctx context.Context, action, corpus, state string) snapshotPerformanceProcessResult {
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSnapshotPerformanceRepresentativeCorpus$", "-test.v")
	command.Env = append(os.Environ(),
		"AGENTDECK_SNAPSHOT_PERFORMANCE_WORKER=1",
		"AGENTDECK_SNAPSHOT_PERFORMANCE_CORPUS="+corpus,
		"AGENTDECK_SNAPSHOT_PERFORMANCE_STATE="+state,
		"AGENTDECK_SNAPSHOT_PERFORMANCE_ACTION="+action,
		"TZ=UTC",
	)
	started := time.Now()
	output, err := command.CombinedOutput()
	total := time.Since(started)
	result := snapshotPerformanceProcessResult{Outcome: "complete"}
	if command.ProcessState != nil {
		result.CPUTimeMS = (command.ProcessState.UserTime() + command.ProcessState.SystemTime()).Milliseconds()
		result.PeakRSS = snapshotPerformancePeakRSS(command.ProcessState)
	}
	if ctx.Err() != nil {
		result.Outcome = "measurement_deadline"
		return result
	}
	if err != nil {
		result.Outcome = "helper_nonzero_exit"
		return result
	}
	const prefix = "SNAPSHOT_PERFORMANCE_HELPER="
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, prefix) {
			if decodeErr := json.Unmarshal([]byte(strings.TrimPrefix(line, prefix)), &result.Helper); decodeErr != nil {
				result.Outcome = "invalid_result"
				return result
			}
			result.StartupMS = max(0, total.Milliseconds()-result.Helper.CommandMS)
			return result
		}
	}
	result.Outcome = "missing_result"
	return result
}

func mergeSnapshotPerformanceProcess(sample *snapshotPerformanceSample, process snapshotPerformanceProcessResult) {
	sample.CPUTimeMS += process.CPUTimeMS
	sample.PeakRSSBytes = max(sample.PeakRSSBytes, process.PeakRSS)
	sample.ProcessStartupMS += process.StartupMS
}

func applySnapshotPerformanceTarget(sample *snapshotPerformanceSample) {
	sample.WithinTarget = sample.Complete && sample.WallMS <= sample.TargetMS
	if sample.Scenario == "unchanged_refresh" {
		sample.WithinTarget = sample.WithinTarget && sample.CPUTimeMS <= 500 && sample.PeakRSSBytes <= 100*1024*1024
	}
}

func newSnapshotPerformanceState(t *testing.T) string {
	t.Helper()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestSnapshotPerformanceCorpusDigestIncludesArchives(t *testing.T) {
	corpus := writeSnapshotPerformanceCorpus(t)
	before := snapshotPerformanceCorpusDigest(t, corpus.Home)
	archive := filepath.Join(corpus.Home, ".codex", "archived_sessions", "archived.jsonl")
	if err := os.MkdirAll(filepath.Dir(archive), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	after := snapshotPerformanceCorpusDigest(t, corpus.Home)
	if after.Files != before.Files+1 || after.Bytes != before.Bytes+3 || after.SHA256 == before.SHA256 {
		t.Fatalf("archive missing from corpus identity: before=%+v after=%+v", before, after)
	}
}

func snapshotPerformanceCorpusDigest(t *testing.T, root string) snapshotPerformanceCorpusID {
	t.Helper()
	hash := sha256.New()
	result := snapshotPerformanceCorpusID{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if !strings.Contains(relative, string(filepath.Separator)+"sessions"+string(filepath.Separator)) &&
			!strings.Contains(relative, string(filepath.Separator)+"archived_sessions"+string(filepath.Separator)) &&
			!strings.Contains(relative, string(filepath.Separator)+"projects"+string(filepath.Separator)) {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		fmt.Fprintf(hash, "%s\x00%d\x00", filepath.ToSlash(relative), info.Size())
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		result.Files++
		result.Bytes += info.Size()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	result.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return result
}

func snapshotPerformanceHardwareInfo(t *testing.T) snapshotPerformanceHardware {
	t.Helper()
	hardware := snapshotPerformanceHardware{OS: runtime.GOOS, Architecture: runtime.GOARCH, LogicalCPUs: runtime.NumCPU()}
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			hardware.CPU = strings.TrimSpace(string(output))
		}
		if output, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
			hardware.MemoryBytes, _ = strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
		}
	}
	return hardware
}

func summarizeSnapshotPerformance(samples []snapshotPerformanceSample) []snapshotPerformanceSummary {
	byScenario := map[string][]snapshotPerformanceSample{}
	for _, sample := range samples {
		byScenario[sample.Scenario] = append(byScenario[sample.Scenario], sample)
	}
	keys := make([]string, 0, len(byScenario))
	for scenario := range byScenario {
		keys = append(keys, scenario)
	}
	sort.Strings(keys)
	out := make([]snapshotPerformanceSummary, 0, len(keys))
	for _, scenario := range keys {
		values := byScenario[scenario]
		wall := make([]int64, len(values))
		summary := snapshotPerformanceSummary{Scenario: scenario, Samples: len(values), AllComplete: true, AllWithinTarget: true}
		for i, sample := range values {
			wall[i] = sample.WallMS
			summary.MaximumCPUTimeMS = max(summary.MaximumCPUTimeMS, sample.CPUTimeMS)
			summary.MaximumPeakRSSBytes = max(summary.MaximumPeakRSSBytes, sample.PeakRSSBytes)
			summary.AllComplete = summary.AllComplete && sample.Complete
			summary.AllWithinTarget = summary.AllWithinTarget && sample.WithinTarget
		}
		sort.Slice(wall, func(i, j int) bool { return wall[i] < wall[j] })
		summary.MedianMS = wall[len(wall)/2]
		summary.P95MS = wall[(95*len(wall)-1)/100]
		summary.MaximumMS = wall[len(wall)-1]
		out = append(out, summary)
	}
	return out
}

func writeSnapshotPerformanceCorpus(t *testing.T) snapshotPerformanceCorpus {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	codex := filepath.Join(home, ".codex", "sessions", "2026", "08", "13", "contract.jsonl")
	claude := filepath.Join(home, ".claude", "projects", "-synthetic-contract", "contract.jsonl")
	for _, path := range []string{codex, claude} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	privateFile := filepath.Join(home, "work", "synthetic.go")
	codexLines := []string{
		`{"type":"session_meta","payload":{"id":"codex-contract"}}`,
		`{"type":"turn_context","payload":{"turn_id":"codex-turn","model":"gpt-5.6"}}`,
		`{"type":"visible_user_prompt","session_id":"codex-contract","timestamp":"2026-08-13T08:00:00Z","payload":{"text":"synthetic prompt"}}`,
		`{"type":"event_msg","timestamp":"2026-08-13T08:00:01Z","payload":{"type":"user_message","message":"implement synthetic contract"}}`,
		fmt.Sprintf(`{"type":"response_item","timestamp":"2026-08-13T08:00:02Z","payload":{"item":{"type":"function_call","call_id":"call-1","name":"exec_command","arguments":{"cmd":"apply patch %s","workdir":"%s"}}}}`, privateFile, filepath.Dir(privateFile)),
		`{"type":"response_item","timestamp":"2026-08-13T08:00:03Z","payload":{"item":{"type":"function_call_output","call_id":"call-1","output":"synthetic result"}}}`,
		`{"type":"event_msg","timestamp":"2026-08-13T08:00:04Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":10}}}}`,
		`{"type":"visible_assistant_final","session_id":"codex-contract","timestamp":"2026-08-13T08:00:05Z","payload":{"text":"synthetic answer"}}`,
	}
	claudeLines := []string{
		`{"type":"user","sessionId":"claude-contract","timestamp":"2026-08-10T09:00:00Z","cwd":"/synthetic/project","message":{"role":"user","content":"synthetic claude prompt"}}`,
		`{"type":"assistant","sessionId":"claude-contract","timestamp":"2026-08-10T09:00:01Z","message":{"role":"assistant","id":"message-1","model":"claude-sonnet-5","usage":{"input_tokens":50,"output_tokens":5},"content":[{"type":"tool_use","id":"tool-1","name":"Read","input":{"file_path":"/synthetic/project/README.md"}},{"type":"text","text":"synthetic claude answer"}]}}`,
	}
	if err := os.WriteFile(codex, []byte(strings.Join(codexLines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claude, []byte(strings.Join(claudeLines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return snapshotPerformanceCorpus{Home: home, Codex: codex, Claude: claude}
}

func appendSnapshotPerformanceChange(t *testing.T, corpus snapshotPerformanceCorpus) {
	t.Helper()
	file, err := os.OpenFile(corpus.Codex, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	lines := []string{
		`{"type":"turn_context","payload":{"turn_id":"codex-turn-2","model":"gpt-5.6"}}`,
		`{"type":"visible_user_prompt","session_id":"codex-changed","timestamp":"2026-08-13T09:00:00Z","payload":{"text":"changed synthetic prompt"}}`,
		`{"type":"event_msg","timestamp":"2026-08-13T09:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":140,"cached_input_tokens":20,"output_tokens":15}}}}`,
		`{"type":"visible_assistant_final","session_id":"codex-changed","timestamp":"2026-08-13T09:00:02Z","payload":{"text":"changed synthetic answer"}}`,
	}
	if _, err = file.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		t.Fatal(err)
	}
}

func assertCompleteDesktopRefresh(t *testing.T, scenario string, result desktopIndexRefreshResult, partial bool, warnings []string) {
	t.Helper()
	if partial || len(warnings) != 0 || !result.Usage.Success || !result.Sessions.Success {
		t.Fatalf("%s refresh incomplete: result=%#v partial=%t warnings=%v", scenario, result, partial, warnings)
	}
	if result.Usage.failureStage != "" || result.Sessions.failureStage != "" {
		t.Fatalf("%s refresh retained failure stage: %#v", scenario, result)
	}
}

func captureSnapshotPerformanceReference(t *testing.T, state, home string) snapshotPerformanceReference {
	t.Helper()
	oldNow := desktopNow
	desktopNow = func() time.Time { return snapshotPerformanceNow }
	t.Cleanup(func() { desktopNow = oldNow })
	result, err := (desktop.Service{
		StateRoot: state,
		Home:      home,
		Workdir:   filepath.Join(home, "work"),
		Now:       desktopNow,
		Location:  time.UTC,
	}).Build(context.Background(), desktop.Request{WireVersion: desktop.WireVersion, RecentLimit: 20})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if result.Partial {
		t.Fatalf("snapshot partial: warnings=%v snapshot=%#v", result.Warnings, result.Snapshot)
	}
	var stream bytes.Buffer
	if err = writeDesktopSnapshotStream(&stream, result); err != nil {
		t.Fatalf("write stream: %v", err)
	}
	payload := decodeSnapshotPerformanceStream(t, stream.Bytes())
	return snapshotPerformanceReference{Payload: payload, Result: result, Rows: captureSnapshotPerformanceRows(t, state)}
}

func decodeSnapshotPerformanceStream(t *testing.T, stream []byte) []byte {
	t.Helper()
	payload, _, err := decodeSnapshotPerformanceStreamValue(stream)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func decodeSnapshotPerformanceStreamValue(stream []byte) ([]byte, bool, error) {
	scanner := bufio.NewScanner(bytes.NewReader(stream))
	scanner.Buffer(make([]byte, 0, 64*1024), base64.StdEncoding.EncodedLen(desktopSnapshotChunkBytes)+16*1024)
	frames := []desktopSnapshotChunkEnvelope{}
	for scanner.Scan() {
		var frame desktopSnapshotChunkEnvelope
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			return nil, false, fmt.Errorf("decode stream frame: %w", err)
		}
		frames = append(frames, frame)
	}
	if err := scanner.Err(); err != nil {
		return nil, false, err
	}
	if len(frames) == 0 || len(frames) != frames[0].Data.Count {
		return nil, false, fmt.Errorf("stream frame count=%d", len(frames))
	}
	var payload []byte
	for index, frame := range frames {
		if frame.Data.Index != index || frame.Data.Count != len(frames) || frame.Data.TotalBytes != frames[0].Data.TotalBytes || frame.Data.SHA256 != frames[0].Data.SHA256 {
			return nil, false, fmt.Errorf("stream metadata mismatch at frame %d", index)
		}
		chunk, err := base64.StdEncoding.DecodeString(frame.Data.Payload)
		if err != nil {
			return nil, false, fmt.Errorf("decode stream payload: %w", err)
		}
		payload = append(payload, chunk...)
	}
	digest := sha256.Sum256(payload)
	if len(payload) != frames[0].Data.TotalBytes || hex.EncodeToString(digest[:]) != frames[0].Data.SHA256 {
		return nil, false, fmt.Errorf("stream integrity mismatch: bytes=%d/%d digest=%x/%s", len(payload), frames[0].Data.TotalBytes, digest, frames[0].Data.SHA256)
	}
	var envelope struct {
		Data    desktop.Snapshot `json:"data"`
		Partial bool             `json:"partial"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, false, fmt.Errorf("decode complete payload: %w", err)
	}
	return payload, envelope.Partial, nil
}

type snapshotQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func captureSnapshotPerformanceRows(t *testing.T, state string) map[string][]string {
	t.Helper()
	ctx := context.Background()
	core, err := store.OpenReadOnly(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	sessions, err := store.OpenSessionsReadOnly(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer sessions.Close()
	queries := map[string]struct {
		db    snapshotQueryer
		query string
	}{
		"usage_events":       {core.DB, `SELECT event_key,client,session_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,source_path,source_offset,COALESCE(turn_index,0) FROM usage_events ORDER BY event_key`},
		"usage_sources":      {core.DB, `SELECT path,identity,size,cursor,prefix_hash,session_id,turn_id,model,parser_version,codex_cumulative_json,imported,replaced,malformed,unsupported,modified_at FROM usage_source_files ORDER BY path`},
		"usage_tool_calls":   {core.DB, `SELECT activity_key,client,session_id,tool_name,status,source_path,source_offset,COALESCE(turn_index,0),tool_kind,COALESCE(mcp_server,'') FROM usage_tool_calls ORDER BY activity_key`},
		"usage_tool_files":   {core.DB, `SELECT activity_key,path_digest,base_name,wrote FROM usage_tool_files ORDER BY activity_key,path_digest`},
		"usage_work_signals": {core.DB, `SELECT client,session_id,turn_index,started_at,state,message_class,intent_sub,activity_kind,activity_sub,source_path FROM usage_work_signals ORDER BY client,session_id,turn_index`},
		"session_metadata":   {sessions.DB, `SELECT client,session_id,project,model,first_at,last_at FROM session_metadata ORDER BY client,session_id,source_path`},
		"session_documents":  {sessions.DB, `SELECT client,session_id,event_at,kind,text FROM session_documents ORDER BY client,session_id,event_at,kind,text`},
		"session_sources":    {sessions.DB, `SELECT source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version FROM session_sources ORDER BY source_path`},
	}
	out := make(map[string][]string, len(queries))
	for name, query := range queries {
		rows, queryErr := query.db.QueryContext(ctx, query.query)
		if queryErr != nil {
			t.Fatalf("query %s: %v", name, queryErr)
		}
		columns, columnsErr := rows.Columns()
		if columnsErr != nil {
			rows.Close()
			t.Fatal(columnsErr)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if scanErr := rows.Scan(pointers...); scanErr != nil {
				rows.Close()
				t.Fatal(scanErr)
			}
			parts := make([]string, len(values))
			for i, value := range values {
				parts[i] = fmt.Sprintf("%v", value)
			}
			out[name] = append(out[name], strings.Join(parts, "\x00"))
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			t.Fatal(rowsErr)
		}
		rows.Close()
		sort.Strings(out[name])
	}
	return out
}

func snapshotPerformanceRowsDigest(ctx context.Context, state string) (string, error) {
	core, err := store.OpenReadOnly(ctx, state)
	if err != nil {
		return "", err
	}
	defer core.Close()
	sessions, err := store.OpenSessionsReadOnly(ctx, state)
	if err != nil {
		return "", err
	}
	defer sessions.Close()
	queries := map[string]struct {
		db    snapshotQueryer
		query string
	}{
		"usage_events":       {core.DB, `SELECT event_key,client,session_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,source_path,source_offset,COALESCE(turn_index,0) FROM usage_events ORDER BY event_key`},
		"usage_sources":      {core.DB, `SELECT path,identity,size,cursor,prefix_hash,session_id,turn_id,model,parser_version,codex_cumulative_json,imported,replaced,malformed,unsupported,modified_at FROM usage_source_files ORDER BY path`},
		"usage_tool_calls":   {core.DB, `SELECT activity_key,client,session_id,tool_name,status,source_path,source_offset,COALESCE(turn_index,0),tool_kind,COALESCE(mcp_server,'') FROM usage_tool_calls ORDER BY activity_key`},
		"usage_tool_files":   {core.DB, `SELECT activity_key,path_digest,base_name,wrote FROM usage_tool_files ORDER BY activity_key,path_digest`},
		"usage_work_signals": {core.DB, `SELECT client,session_id,turn_index,started_at,state,message_class,intent_sub,activity_kind,activity_sub,source_path FROM usage_work_signals ORDER BY client,session_id,turn_index`},
		"session_metadata":   {sessions.DB, `SELECT client,session_id,project,model,first_at,last_at FROM session_metadata ORDER BY client,session_id,source_path`},
		"session_documents":  {sessions.DB, `SELECT client,session_id,event_at,kind,text FROM session_documents ORDER BY client,session_id,event_at,kind,text`},
		"session_sources":    {sessions.DB, `SELECT source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version FROM session_sources ORDER BY source_path`},
	}
	valuesByTable := make(map[string][]string, len(queries))
	for name, query := range queries {
		rows, queryErr := query.db.QueryContext(ctx, query.query)
		if queryErr != nil {
			return "", fmt.Errorf("query %s: %w", name, queryErr)
		}
		columns, columnsErr := rows.Columns()
		if columnsErr != nil {
			rows.Close()
			return "", columnsErr
		}
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for index := range values {
				pointers[index] = &values[index]
			}
			if scanErr := rows.Scan(pointers...); scanErr != nil {
				rows.Close()
				return "", scanErr
			}
			parts := make([]string, len(values))
			for index, value := range values {
				parts[index] = fmt.Sprintf("%v", value)
			}
			valuesByTable[name] = append(valuesByTable[name], strings.Join(parts, "\x00"))
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			return "", rowsErr
		}
		rows.Close()
		sort.Strings(valuesByTable[name])
	}
	encoded, err := json.Marshal(valuesByTable)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func assertSnapshotPerformanceGolden(t *testing.T, payload []byte) {
	t.Helper()
	path := filepath.Join("testdata", "snapshot-performance", "synthetic-snapshot.json")
	if os.Getenv("AGENTDECK_UPDATE_SNAPSHOT_PERFORMANCE_FIXTURE") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, payload, "", "  "); err != nil {
			t.Fatal(err)
		}
		pretty.WriteByte('\n')
		if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err = json.Compact(&compact, want); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compact.Bytes(), payload) {
		t.Fatalf("snapshot reference differs at %s; regenerate only after approving the contract change", firstByteDifference(compact.Bytes(), payload))
	}
}

func firstByteDifference(want, got []byte) string {
	limit := min(len(want), len(got))
	for i := 0; i < limit; i++ {
		if want[i] != got[i] {
			return fmt.Sprintf("byte %d (want %q, got %q)", i, want[i], got[i])
		}
	}
	return fmt.Sprintf("byte %d (length %d vs %d)", limit, len(want), len(got))
}
