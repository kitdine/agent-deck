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

func TestThermalNominalReadsTheSpeedLimitValue(t *testing.T) {
	nominal := "Note: No thermal warning level has been recorded\n\tCPU_Scheduler_Limit \t= 100\n\tCPU_Available_CPUs \t= 12\n\tCPU_Speed_Limit \t= 100\n"
	if !thermalNominal(nominal) {
		t.Fatal("CPU_Speed_Limit = 100 is nominal")
	}
	if thermalNominal("\tCPU_Speed_Limit \t= 72\n") {
		t.Fatal("a reduced speed limit is not nominal")
	}
	if !thermalNominal("Note: No thermal warning level has been recorded\n") {
		t.Fatal("no limit reported is nominal")
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
