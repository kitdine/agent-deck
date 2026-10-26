package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNearestRankFollowsTheContractDefinition(t *testing.T) {
	values := make([]float64, 100)
	for i := range values {
		values[i] = float64(i + 1)
	}
	for q, want := range map[float64]float64{0.5: 50, 0.95: 95, 0.99: 99} {
		if got := nearestRank(values, q); got != want {
			t.Fatalf("P(%v) = %v, want %v", q, got, want)
		}
	}
	if got := nearestRank([]float64{7, 9, 11}, 0.5); got != 9 {
		t.Fatalf("odd median = %v", got)
	}
}

func TestPairIntervalsCountsMissingAndLateEndsAsFailures(t *testing.T) {
	ms := int64(1e6)
	records := []traceRecord{
		{Stage: "input.event", T: 0},
		{Stage: "frame.vsync", T: 120 * ms},
		{Stage: "input.event", T: 1000 * ms},
		// No end before the next start: product failure.
		{Stage: "input.event", T: 2000 * ms},
		{Stage: "frame.vsync", T: 9000 * ms}, // beyond the 5 s timeout
		{Stage: "frame.vsync", T: 9100 * ms}, // an end without a start is ignored
	}
	got := pairIntervals(records, "input.event", "frame.vsync", 5000*ms, nil)
	if len(got) != 3 || got[0].Value != 120*ms || got[0].Failed || !got[1].Failed || !got[2].Failed {
		t.Fatalf("intervals = %+v", got)
	}
	s := summarize(got)
	if s.N != 3 || s.Failures != 2 || s.MinMS != 120 || !math.IsInf(s.P95MS, 1) {
		t.Fatalf("summary = %+v", s)
	}
	raw, err := json.Marshal(s)
	if err != nil || !bytes.Contains(raw, []byte(`"p95_ms":null`)) {
		t.Fatalf("failures must serialise as null tail values: %s %v", raw, err)
	}
}

// P03 is judged per dimension: each interval keeps its start marker's counts.
func TestPairIntervalsKeepsTheStartDimension(t *testing.T) {
	ms := int64(1e6)
	records := []traceRecord{
		{Stage: "select.intent", T: 0, N: map[string]int64{"dimension": 1}},
		{Stage: "frame.vsync", T: 40 * ms},
		{Stage: "select.intent", T: 100 * ms, N: map[string]int64{"dimension": 3}},
	}
	got := pairIntervals(records, "select.intent", "frame.vsync", 5000*ms, nil)
	if len(got) != 2 || got[0].Start["dimension"] != 1 || got[0].Value != 40*ms ||
		got[1].Start["dimension"] != 3 || !got[1].Failed {
		t.Fatalf("intervals = %+v", got)
	}
}

func TestPeakRSSUpperBoundSumsOnlyOverlappingProcesses(t *testing.T) {
	life := func(role string, start, end, rss int64) traceFile {
		return traceFile{Role: role, Start: start, Records: []traceRecord{{Stage: "resource.lifetime", T: end, N: map[string]int64{"maxrss_bytes": rss}}}}
	}
	files := []traceFile{
		life("collector", 0, 100, 200),
		life("helper", 50, 60, 30),   // overlaps the collector
		life("helper", 100, 120, 40), // touches the collector's end
		life("helper", 200, 300, 90),
		life("app", 0, 400, 1000), // the App is reported separately, never summed
	}
	if got := peakRSSUpperBound(files); got != 240 {
		t.Fatalf("peak = %d, want 240", got)
	}
}

func TestStatsMarksIncompleteSamplesToolInvalid(t *testing.T) {
	root := t.TempDir()
	write := func(sample, name, body string) {
		dir := filepath.Join(root, sample)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeSchedule(t, root, schedule{Units: 3})
	header := `{"v":1,"role":"app","seq":1,"clock":"CLOCK_UPTIME_RAW","t":0}` + "\n"
	write("sample-0001", "app-000001.jsonl", header+`{"stage":"app.ready","t":10}`+"\n"+`{"stage":"model.usable","t":2000010}`+"\n"+`{"stage":"resource.lifetime","t":3000000}`+"\n")
	write("sample-0002", "app-000001.jsonl", header+`{"stage":"app.ready","t":10}`+"\n") // never flushed lifetime
	write("sample-0003", "app-000001.jsonl", header+`{"stage":"app.ready","t":10}`+"\n"+`{"stage":"trace.invalid","t":11,"n":{"rejected":1}}`+"\n"+`{"stage":"resource.lifetime","t":12}`+"\n")
	var out bytes.Buffer
	if err := runStats(root, "P02", &out); err != nil {
		t.Fatal(err)
	}
	var report struct {
		Samples     int     `json:"samples"`
		ToolInvalid int     `json:"tool_invalid"`
		Summary     summary `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Samples != 3 || report.ToolInvalid != 2 || report.Summary.N != 1 {
		t.Fatalf("report = %s", out.String())
	}
}

// A run whose App never became usable is a product failure for the
// condition's own stimulus, not a tool-invalid sample.
func TestPreconditionTimeoutCountsAsProductFailure(t *testing.T) {
	root := t.TempDir()
	writeSchedule(t, root, schedule{Units: 1})
	dir := filepath.Join(root, "sample-0001")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	trace := `{"v":1,"role":"app","seq":1,"clock":"CLOCK_UPTIME_RAW","t":0}` + "\n" +
		`{"stage":"app.ready","t":10}` + "\n" + `{"stage":"driver.precondition_timeout","t":30000000010,"n":{"step":0}}` + "\n" +
		`{"stage":"resource.lifetime","t":30000000020}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "app-000001.jsonl"), []byte(trace), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, condition := range []string{"P01-first", "P02"} {
		var out bytes.Buffer
		if err := runStats(root, condition, &out); err != nil {
			t.Fatal(err)
		}
		var report struct {
			ToolInvalid int     `json:"tool_invalid"`
			Summary     summary `json:"summary"`
		}
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.ToolInvalid != 0 || report.Summary.N != 1 || report.Summary.Failures != 1 {
			t.Fatalf("%s: %s", condition, out.String())
		}
	}
}

func TestScreenLockedReadsTheConsoleSession(t *testing.T) {
	locked := `+-o Root  <class IORegistryEntry>
    "IOConsoleUsers" = ({"kCGSSessionOnConsoleKey"=Yes,"CGSSessionScreenIsLocked"=Yes,"kCGSessionLoginDoneKey"=Yes})`
	if !screenLockedIn(locked) {
		t.Fatal("a locked console session must be detected")
	}
	if screenLockedIn(strings.Replace(locked, `"CGSSessionScreenIsLocked"=Yes,`, "", 1)) {
		t.Fatal("an unlocked session reports no lock key")
	}
}

// processTrace renders one process trace whose lifetime record carries counts.
func processTrace(role string, lifetime traceRecord, records ...traceRecord) string {
	var b strings.Builder
	b.WriteString(`{"v":1,"role":"` + role + `","seq":1,"clock":"CLOCK_UPTIME_RAW","t":0}` + "\n")
	for _, record := range append(records, lifetime) {
		raw, _ := json.Marshal(record)
		b.Write(append(raw, '\n'))
	}
	return b.String()
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// P08 compares the append scan's read bytes with what the append wrote plus
// the declared anchors, and reports an excess instead of hiding it.
func TestP08ReportsTheAppendBound(t *testing.T) {
	root := t.TempDir()
	writeSchedule(t, root, schedule{Mode: "formal", Units: 3})
	scan := func(read int64) string {
		return processTrace("collector", traceRecord{Stage: "resource.lifetime", T: 2},
			traceRecord{Stage: "scan.end", T: 1, N: map[string]int64{"read_bytes": read, "decoded_records": read / 100}})
	}
	appended, _ := json.Marshal(appendResult{Files: 2, Records: 2, DeltaBytes: 300, AnchorBytes: 8192})
	for unit := 1; unit <= 3; unit++ {
		dir := sampleDir(t, root, unit, nil)
		writeFile(t, filepath.Join(dir, "noop", "collector-000001.jsonl"), scan(0))
		writeFile(t, filepath.Join(dir, "append", "collector-000001.jsonl"), scan(9000))
		writeFile(t, filepath.Join(dir, "append", appendFile), string(appended))
	}
	report := statsReport(t, root, "P08")
	bound, _ := report.Values["append_bound"].(map[string]any)
	if report.ToolInvalid != 0 || bound["bound_bytes"] != float64(8492) || bound["excess_bytes"] != float64(508) ||
		report.Values["append_within_bound"] != false {
		t.Fatalf("P08 = %+v", report)
	}
	if err := os.Remove(filepath.Join(root, "sample-0002", "append", appendFile)); err != nil {
		t.Fatal(err)
	}
	if report := statsReport(t, root, "P08"); report.ToolInvalid != 1 {
		t.Fatalf("a missing append result must be tool-invalid: %+v", report)
	}
}

// P09 reports each arm's own rebuild cost: S0 from its worker's traces, B0
// from its original scan profile, never from the front process alone.
func TestP09ReportsEachArmsOwnRebuildCost(t *testing.T) {
	root := t.TempDir()
	writeSchedule(t, root, schedule{Mode: "formal", Units: 2})
	dur := func(v int64) *int64 { return &v }
	for unit := 1; unit <= 2; unit++ {
		var runs []cliRun
		for _, label := range p09Order(unit) {
			runs = append(runs, cliRun{Label: label, WallNS: 10_000 * ms, UserNS: 20 * ms, MaxRSS: 10 << 20})
		}
		raw, _ := json.Marshal(runs)
		dir := sampleDir(t, root, unit, map[string]string{"cli-runs.json": string(raw)})
		writeFile(t, filepath.Join(dir, "B0", b0ProfileFile), "unrelated line\n"+
			`{"type":"scan_performance","stages":{"discovery_ms":30,"usage_ms":12000,"session_ms":13000,"derived_cache_ms":1500,"total_ms":15000,"worker_cpu_time_ms":30000,"worker_peak_rss_bytes":209715200,"ingestion":{"read_bytes":500}}}`+"\n")
		writeFile(t, filepath.Join(dir, "S0", "collector-000001.jsonl"), processTrace("collector",
			traceRecord{Stage: "resource.lifetime", T: 20, N: map[string]int64{"user_ns": 25_000 * ms, "sys_ns": 1_000 * ms, "maxrss_bytes": 160 << 20}},
			traceRecord{Stage: "scan.usage", T: 1, Dur: dur(11_000 * ms)}, traceRecord{Stage: "scan.session", T: 1, Dur: dur(11_500 * ms)},
			traceRecord{Stage: "scan.derive", T: 12, Dur: dur(1_400 * ms)}, traceRecord{Stage: "scan.end", T: 19, N: map[string]int64{"read_bytes": 400}}))
		writeFile(t, filepath.Join(dir, "S0", "cli-000001.jsonl"), processTrace("cli",
			traceRecord{Stage: "resource.lifetime", T: 21, N: map[string]int64{"maxrss_bytes": 12 << 20}}))
	}
	report := statsReport(t, root, "P09")
	b0, _ := report.Values["B0_rebuild"].(map[string]any)
	s0, _ := report.Values["S0_rebuild"].(map[string]any)
	median := func(arm map[string]any, key string) any {
		value, _ := arm[key].(map[string]any)
		return value["median_ms"]
	}
	if report.ToolInvalid != 0 || median(b0, "worker_cpu") != float64(30000) || median(b0, "total_cpu") != float64(30020) ||
		median(b0, "stage_session") != float64(13000) || median(s0, "worker_cpu") != float64(26000) ||
		median(s0, "stage_session") != float64(11500) {
		t.Fatalf("P09 = %+v", report.Values)
	}
	if rss, _ := b0["rss_upper_bound_bytes"].([]any); len(rss) != 2 || rss[0] != float64(210<<20) {
		t.Fatalf("B0 RSS bound = %v, want worker peak plus front peak", b0["rss_upper_bound_bytes"])
	}
	if rss, _ := s0["rss_upper_bound_bytes"].([]any); len(rss) != 2 || rss[0] != float64(172<<20) {
		t.Fatalf("S0 RSS bound = %v, want overlapping collector plus cli", s0["rss_upper_bound_bytes"])
	}
	if err := os.Remove(filepath.Join(root, "sample-0001", "S0", "collector-000001.jsonl")); err != nil {
		t.Fatal(err)
	}
	if report := statsReport(t, root, "P09"); report.ToolInvalid != 1 {
		t.Fatalf("an arm without its own rebuild measurement must be tool-invalid: %+v", report)
	}
}

// S0's read bytes come only from a recorded scan.end counter: a trace missing
// the event or its read_bytes is tool-invalid, while a recorded zero is a
// valid measurement.
func TestP09S0ReadBytesMustBeRecorded(t *testing.T) {
	dur := func(v int64) *int64 { return &v }
	cases := []struct {
		name    string
		end     []traceRecord
		invalid int
		read    any
	}{
		{"missing scan.end", nil, 1, nil},
		{"scan.end without read_bytes", []traceRecord{{Stage: "scan.end", T: 19, N: map[string]int64{"decoded_records": 4}}}, 1, nil},
		{"recorded zero", []traceRecord{{Stage: "scan.end", T: 19, N: map[string]int64{"read_bytes": 0}}}, 0, float64(0)},
		{"complete", []traceRecord{{Stage: "scan.end", T: 19, N: map[string]int64{"read_bytes": 400}}}, 0, float64(400)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeSchedule(t, root, schedule{Mode: "formal", Units: 1})
			var runs []cliRun
			for _, label := range p09Order(1) {
				runs = append(runs, cliRun{Label: label, WallNS: 10_000 * ms, UserNS: 20 * ms, MaxRSS: 10 << 20})
			}
			raw, _ := json.Marshal(runs)
			dir := sampleDir(t, root, 1, map[string]string{"cli-runs.json": string(raw)})
			writeFile(t, filepath.Join(dir, "B0", b0ProfileFile),
				`{"type":"scan_performance","stages":{"usage_ms":12000,"session_ms":13000,"derived_cache_ms":1500,"total_ms":15000,"worker_cpu_time_ms":30000,"worker_peak_rss_bytes":209715200,"ingestion":{"read_bytes":500}}}`+"\n")
			records := append([]traceRecord{
				{Stage: "scan.usage", T: 1, Dur: dur(11_000 * ms)}, {Stage: "scan.session", T: 1, Dur: dur(11_500 * ms)},
				{Stage: "scan.derive", T: 12, Dur: dur(1_400 * ms)}}, tc.end...)
			writeFile(t, filepath.Join(dir, "S0", "collector-000001.jsonl"), processTrace("collector",
				traceRecord{Stage: "resource.lifetime", T: 20, N: map[string]int64{"user_ns": 25_000 * ms, "maxrss_bytes": 160 << 20}}, records...))
			report := statsReport(t, root, "P09")
			if report.ToolInvalid != tc.invalid {
				t.Fatalf("tool_invalid = %d, want %d: %+v", report.ToolInvalid, tc.invalid, report)
			}
			if tc.read == nil {
				return
			}
			s0, _ := report.Values["S0_rebuild"].(map[string]any)
			if reads, _ := s0["read_bytes"].([]any); len(reads) != 1 || reads[0] != tc.read {
				t.Fatalf("S0 read_bytes = %v, want [%v]", s0["read_bytes"], tc.read)
			}
		})
	}
}

// An attempt the driver marked interrupted from outside is rerun, not counted:
// it is recognised, CLI-only units never are, and stats reports the reruns
// kept under interrupted/ without reading them as samples.
func TestInterruptedAttemptsAreRerunNotCounted(t *testing.T) {
	if screenLocked() {
		t.Skip("the console session is locked; unitInterruption would report the lock")
	}
	root := t.TempDir()
	writeSchedule(t, root, schedule{Mode: "formal", Units: 1})
	interrupted := sampleDir(t, filepath.Join(root, interruptedDir), 1, map[string]string{
		"app-000001.jsonl": trace("app", traceRecord{Stage: "app.ready", T: 10}, traceRecord{Stage: "driver.interrupted", T: 20})})
	if reason := unitInterruption(interrupted); reason == "" {
		t.Fatal("a driver.interrupted attempt must be recognised")
	}
	clean := sampleDir(t, root, 1, map[string]string{
		"app-000001.jsonl": trace("app", traceRecord{Stage: "app.ready", T: 10}, traceRecord{Stage: "model.usable", T: 2_000_010})})
	if reason := unitInterruption(clean); reason != "" {
		t.Fatalf("an uninterrupted attempt reported %q", reason)
	}
	cli := sampleDir(t, filepath.Join(t.TempDir(), "cli"), 1, map[string]string{"cli-runs.json": "[]"})
	if reason := unitInterruption(cli); reason != "" {
		t.Fatalf("a CLI-only unit has no window to interrupt: %q", reason)
	}
	report := statsReport(t, root, "P02")
	if report.Samples != 1 || report.ToolInvalid != 0 || report.Interrupted != 1 || report.Summary.N != 1 {
		t.Fatalf("report = %+v", report)
	}
}
