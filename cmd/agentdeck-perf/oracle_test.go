package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/usage"
)

const ms = int64(1e6)

// sampleDir creates root/sample-NNNN with the given files.
func sampleDir(t *testing.T, root string, index int, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(root, fmt.Sprintf("sample-%04d", index))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// trace renders one process trace: header, records, lifetime tail.
func trace(role string, records ...traceRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, `{"v":1,"role":%q,"seq":1,"clock":"CLOCK_UPTIME_RAW","t":0}`+"\n", role)
	last := int64(0)
	for _, record := range records {
		raw, _ := json.Marshal(record)
		b.Write(append(raw, '\n'))
		last = max(last, record.T)
	}
	fmt.Fprintf(&b, `{"stage":"resource.lifetime","t":%d}`+"\n", last+1)
	return b.String()
}

func writeSchedule(t *testing.T, root string, s schedule) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(root, scheduleFile), s); err != nil {
		t.Fatal(err)
	}
}

func oracleJSON(t *testing.T, o oracle) string {
	t.Helper()
	raw, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func (h heroSummary) counts() map[string]int64 {
	return map[string]int64{"client": h.Client, "period": h.Period, "tokens": h.Tokens, "events": h.Events,
		"sessions": h.Sessions, "day": h.Day, "cost": h.Cost}
}

func frame(at int64, hero heroSummary) traceRecord {
	return traceRecord{Stage: "frame.vsync", T: at, N: hero.counts()}
}

func statsReport(t *testing.T, root, condition string) batchReport {
	t.Helper()
	var out bytes.Buffer
	if err := runStats(root, condition, &out); err != nil {
		t.Fatal(err)
	}
	report := batchReport{Summary: &summary{}}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	return report
}

var today = heroSummary{Client: 1, Period: 1, Tokens: 100, Events: 10, Sessions: 2, Day: 20261024, Cost: 7}

// The Swift probe and this tool must hash identically (PerfFrameProbeTests
// pins the same value).
func TestFNVDigestMatchesTheAppProbe(t *testing.T) {
	if got := fnv1a("pub", "all", "today", "breakdown"); got != 5653972754369679809 {
		t.Fatalf("fnv1a = %d", got)
	}
}

func TestPresentationHeroesCoverEveryClientAndPeriodWithTheirCost(t *testing.T) {
	cost := "1.25"
	report := usage.PresentationReport{Scopes: []usage.PresentationScope{{
		Client: "codex",
		Periods: usage.PresentationPeriods{Available: true, Items: []usage.PresentationPeriod{
			{Period: "7d", Totals: usage.PresentationTotals{Tokens: 9, Events: 3, Sessions: 1, ProviderCost: &cost, KnownProviderCost: "1.25"}},
			{Period: "lifetime", Totals: usage.PresentationTotals{Tokens: 99}},
		}},
	}}}
	heroes := presentationHeroes(report, 20261024)
	if len(heroes) != 1 || heroes[0] != (heroSummary{Client: 2, Period: 2, Tokens: 9, Events: 3, Sessions: 1, Day: 20261024,
		Cost: costDigest(report.Scopes[0].Periods.Items[0].Totals)}) {
		t.Fatalf("heroes = %+v", heroes)
	}
	unpriced := report.Scopes[0].Periods.Items[0].Totals
	unpriced.ProviderCost = nil
	if costDigest(unpriced) == heroes[0].Cost {
		t.Fatal("a missing cost must digest differently from a present one")
	}
}

// P01: a frame whose hero differs from the oracle is a product failure; a
// missing oracle makes the unit tool-invalid.
func TestFrameHeroIsCheckedAgainstTheOracle(t *testing.T) {
	root := t.TempDir()
	writeSchedule(t, root, schedule{Units: 3})
	wrong := today
	wrong.Sessions = 3 // same tokens, another hero field wrong
	click := func(hero heroSummary) string {
		return trace("app", traceRecord{Stage: "input.action", T: 10 * ms}, frame(60*ms, hero))
	}
	o := oracleJSON(t, oracle{Heroes: []heroSummary{today}})
	sampleDir(t, root, 1, map[string]string{"app-000001.jsonl": click(today), oracleFile: o})
	sampleDir(t, root, 2, map[string]string{"app-000001.jsonl": click(wrong), oracleFile: o})
	sampleDir(t, root, 3, map[string]string{"app-000001.jsonl": click(today)})
	report := statsReport(t, root, "P01-first")
	if report.ToolInvalid != 1 || !strings.Contains(report.Reasons[0], "oracle") || report.Summary.N != 2 || report.Summary.Failures != 1 {
		t.Fatalf("report = %+v", report)
	}
}

// P10: each write is judged against its own expected hero, by sequence.
func TestWriteToFrameMatchesOnlyTheExpectedStateOfEachWrite(t *testing.T) {
	after := func(tokens, events int64) heroSummary {
		h := today
		h.Tokens, h.Events = tokens, events
		return h
	}
	expected := oracle{Heroes: []heroSummary{today}, Writes: []heroSummary{after(200, 11), after(300, 12)}}
	written := func(at, seq int64) traceRecord {
		return traceRecord{Stage: "source.written", T: at, N: map[string]int64{"seq": seq}}
	}
	cases := []struct {
		name   string
		frames []traceRecord
		want   []interval
	}{
		{"a drop below the expected total is wrong content", []traceRecord{frame(500*ms, after(99, 10))},
			[]interval{{Failed: true}, {Failed: true}}},
		{"a frame of the earlier state is waited through", []traceRecord{
			frame(200*ms, today), frame(900*ms, after(200, 11)), frame(6200*ms, after(200, 11)), frame(6500*ms, after(300, 12)),
		}, []interval{{Value: 900 * ms}, {Value: 500 * ms}}},
		{"equal tokens with other fields wrong", []traceRecord{frame(400*ms, after(200, 99))},
			[]interval{{Failed: true}, {Failed: true}}},
		{"a skipped state never ends its write", []traceRecord{frame(6100*ms, after(300, 12))},
			[]interval{{Failed: true}, {Value: 100 * ms}}},
	}
	for _, c := range cases {
		files := []traceFile{
			{Role: "driver", Records: []traceRecord{written(0, 1)}},
			{Role: "driver", Records: []traceRecord{written(6000*ms, 2)}},
			{Role: "app", Records: c.frames},
		}
		got, err := writeToFrame(files, expected)
		if err != nil || len(got) != len(c.want) {
			t.Fatalf("%s: %+v %v", c.name, got, err)
		}
		for i := range got {
			if got[i].Failed != c.want[i].Failed || (!got[i].Failed && got[i].Value != c.want[i].Value) {
				t.Fatalf("%s: write %d = %+v, want %+v", c.name, i+1, got[i], c.want[i])
			}
		}
	}
	// A write without its expected state is an oracle failure, not a success.
	if _, err := writeToFrame([]traceFile{{Role: "driver", Records: []traceRecord{written(0, 3)}}}, expected); err == nil {
		t.Fatal("a write beyond the oracle must be tool-invalid")
	}
}

// A unit that did not run its schedule is tool-invalid, whatever it measured.
func TestStatsRejectsUnitsThatDidNotRunTheirSchedule(t *testing.T) {
	root := t.TempDir()
	writeSchedule(t, root, schedule{Mode: "formal", Units: 1, Inner: 2})
	o := oracleJSON(t, oracle{Heroes: []heroSummary{today, {Client: 2, Period: 1, Day: 20261024}}})
	var records []traceRecord
	at := int64(0)
	for _, dimension := range []int64{1, 1, 2, 2, 3} { // one panel selection short
		records = append(records, traceRecord{Stage: "select.intent", T: at, N: map[string]int64{"dimension": dimension}}, frame(at+10*ms, today))
		at += 100 * ms
	}
	sampleDir(t, root, 1, map[string]string{"app-000001.jsonl": trace("app", records...), oracleFile: o})
	if report := statsReport(t, root, "P03"); report.ToolInvalid != 1 || !strings.Contains(report.Reasons[0], "schedule") {
		t.Fatalf("report = %+v", report)
	}
	missing := t.TempDir()
	writeSchedule(t, missing, schedule{Units: 2})
	sampleDir(t, missing, 1, map[string]string{"app-000001.jsonl": trace("app", traceRecord{Stage: "app.ready", T: 1}, traceRecord{Stage: "model.usable", T: 5})})
	if report := statsReport(t, missing, "P02"); report.ToolInvalid != 1 || !strings.Contains(report.Reasons[0], "run units") {
		t.Fatalf("missing unit: %+v", report)
	}
}

// P01-rebuild counts a unit only if a completed rebuild spans its clicks.
func TestRebuildLayerRequiresClicksInsideACompletedRebuild(t *testing.T) {
	root := t.TempDir()
	writeSchedule(t, root, schedule{Units: 3})
	o := oracleJSON(t, oracle{Heroes: []heroSummary{today}})
	clicks := trace("app", traceRecord{Stage: "input.action", T: 1000 * ms}, frame(1050*ms, today),
		traceRecord{Stage: "input.action", T: 2000 * ms}, frame(2040*ms, today))
	rebuild := func(r rebuildRecord) string {
		raw, _ := json.Marshal(r)
		return string(raw)
	}
	sampleDir(t, root, 1, map[string]string{"app-000001.jsonl": clicks, oracleFile: o, rebuildFile: rebuild(rebuildRecord{StartNS: 500 * ms, EndNS: 9000 * ms})})
	sampleDir(t, root, 2, map[string]string{"app-000001.jsonl": clicks, oracleFile: o, rebuildFile: rebuild(rebuildRecord{StartNS: 500 * ms, EndNS: 1500 * ms})})
	sampleDir(t, root, 3, map[string]string{"app-000001.jsonl": clicks, oracleFile: o, rebuildFile: rebuild(rebuildRecord{StartNS: 500 * ms, EndNS: 9000 * ms, ExitCode: 1, Error: "exit status 1"})})
	report := statsReport(t, root, "P01-rebuild")
	if report.ToolInvalid != 2 || report.Summary.N != 2 || report.Summary.Failures != 0 {
		t.Fatalf("report = %+v", report)
	}
}

// The routing segment is input.event → input.action of event clicks; an
// unrouted event click is counted, not tool-invalid, in this condition only.
func TestRoutingPairsReportTheExcludedSegment(t *testing.T) {
	records := []traceRecord{
		{Stage: "input.event", T: 0}, {Stage: "input.action", T: 3 * ms}, frame(40*ms, today),
		{Stage: "input.action", T: 1000 * ms}, frame(1030*ms, today),
		{Stage: "input.event", T: 2000 * ms}, {Stage: "input.unrouted", T: 7000 * ms},
		{Stage: "input.action", T: 8000 * ms}, frame(8020*ms, today),
	}
	routing, events, actions, unrouted := routingPairs(records)
	if len(routing) != 1 || routing[0].Value != 3*ms || len(events) != 1 || events[0].Value != 40*ms ||
		len(actions) != 2 || actions[0].Value != 30*ms || actions[1].Value != 20*ms || unrouted != 1 {
		t.Fatalf("routing=%+v events=%+v actions=%+v unrouted=%d", routing, events, actions, unrouted)
	}
}

// P06: a helper alive at the deadline is a product failure, not a window.
func TestIdleWindowHelperAliveIsAProductFailure(t *testing.T) {
	root := t.TempDir()
	writeSchedule(t, root, schedule{Units: 1, Window: 300})
	sampleDir(t, root, 1, map[string]string{"app-000001.jsonl": trace("app",
		traceRecord{Stage: "model.usable", T: 1}, traceRecord{Stage: "driver.precondition_timeout", T: 420000 * ms, N: map[string]int64{"helper_alive": 1}})})
	report := statsReport(t, root, "P06")
	failure, _ := report.Values["sample-0001"].(map[string]any)
	if report.ToolInvalid != 0 || failure["product_failure"] != "helper alive 7 min after model.usable" {
		t.Fatalf("report = %+v", report)
	}
}

// A planned append replayed into another copy writes the same bytes as
// writing it in place, so the mirror oracle sees the measured content.
func TestPlannedAppendReplaysIdentically(t *testing.T) {
	opts, _ := smallFixture(t, 5)
	plan, err := planAppend(opts.out, opts.anchor, 5, 3, false)
	if err != nil || len(plan.Records) != 1 {
		t.Fatalf("plan = %+v %v", plan, err)
	}
	target := filepath.Join(opts.out, plan.Records[0].Path)
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := writeJSON(path, plan); err != nil {
		t.Fatal(err)
	}
	result, err := replayAppend(opts.out, path)
	if err != nil || result.Records != 1 || result.TokensDelta != plan.Records[0].Tokens {
		t.Fatalf("replay = %+v %v", result, err)
	}
	after, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(after, append(before, []byte(plan.Records[0].Line+"\n")...)) {
		t.Fatalf("replay did not append exactly the planned line: %v", err)
	}
	plan.Records[0].Path = "../outside.jsonl"
	if err := writeJSON(path, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := replayAppend(opts.out, path); err == nil {
		t.Fatal("a planned path outside HOME must be rejected")
	}
}

func TestFormalSchedulesFollowTheContract(t *testing.T) {
	plans := conditionPlans()
	for condition, want := range map[string][2]schedule{
		"P01-first":   {{Units: 20}, {Units: 100}},
		"P01-repeat":  {{Units: 1, Inner: 20}, {Units: 10, Inner: 20}},
		"P01-rebuild": {{Units: 20}, {Units: 100}},
		"P02":         {{Units: 20}, {Units: 100}},
		"P03":         {{Units: 1, Inner: 20}, {Units: 4, Inner: 25}},
		"P04":         {{Units: 20}, {Units: 100}},
		"P06":         {{Units: 1, Window: 300}, {Units: 3, Window: 1800}},
		"P07":         {{Units: 1, Inner: 20}, {Units: 3, Inner: 100}},
		"P08":         {{Units: 2}, {Units: 3}},
		"P09":         {{Units: 3}, {Units: 20}},
		"P10":         {{Units: 1, Inner: 20}, {Units: 4, Inner: 25}},
		"P11-first":   {{Units: 20}, {Units: 100}},
		"P11-full":    {{Units: 1}, {Units: 5}},
		"P01-routing": {{Units: 1, Inner: 20}, {}},
	} {
		plan, ok := plans[condition]
		if !ok || plan.pilot != want[0] || plan.formal != want[1] {
			t.Fatalf("%s: pilot %+v formal %+v, want %+v", condition, plan.pilot, plan.formal, want)
		}
	}
	if len(plans) != 14 {
		t.Fatalf("%d plans", len(plans))
	}
}

// P11's first frame is checked against the bootstrap publication with its
// coverage: totals, day and selection of that partial batch, and the partial
// marker. A frame of another coverage, a wrong day, a missing marker or a
// wrong partial total is a product failure; no bootstrap oracle is a tool
// failure (review Round 2 F1: any first frame used to pass).
func TestFirstBatchFrameIsCheckedAgainstItsCoverage(t *testing.T) {
	first := heroSummary{Client: 1, Period: 1, Tokens: 1126911, Events: 59, Sessions: 57, Day: 20261024, Cost: 3}
	expected := oracle{Heroes: []heroSummary{today}, Batches: []coverageHeroes{
		{Committed: 64, Sources: 2232, Partial: true, Heroes: []heroSummary{first}},
		{Committed: 2232, Sources: 2232, Partial: true, Heroes: []heroSummary{first}},
		{Committed: 2232, Sources: 2232, Complete: true, Heroes: []heroSummary{today}},
	}}
	frameOf := func(hero heroSummary, partial, committed int64) string {
		counts := hero.counts()
		counts["partial"], counts["committed"], counts["sources"] = partial, committed, 2232
		if partial == 0 && committed == 2232 {
			counts["coverage_complete"] = 1
		}
		return trace("app", traceRecord{Stage: "app.ready", T: 10 * ms},
			traceRecord{Stage: "frame.vsync", T: 1010 * ms, N: counts})
	}
	wrongDay, wrongTotal := first, first
	wrongDay.Day = 19000101
	wrongTotal.Tokens = -999
	root := t.TempDir()
	writeSchedule(t, root, schedule{Units: 7})
	o := oracleJSON(t, expected)
	for index, app := range []string{
		frameOf(first, 1, 64),      // valid partial first batch
		frameOf(today, 0, 2232),    // valid complete publication
		frameOf(wrongDay, 1, 64),   // wrong day
		frameOf(first, 0, 64),      // partial batch without its marker
		frameOf(wrongTotal, 1, 64), // wrong partial total
		frameOf(first, 1, 128),     // a coverage the bootstrap never published
		frameOf(first, 1, 64),      // no bootstrap oracle: tool-invalid
	} {
		files := map[string]string{"app-000001.jsonl": app, oracleFile: o}
		if index == 6 {
			files[oracleFile] = oracleJSON(t, oracle{Heroes: []heroSummary{today}})
		}
		sampleDir(t, root, index+1, files)
	}
	report := statsReport(t, root, "P11-first")
	if report.ToolInvalid != 1 || !strings.Contains(report.Reasons[0], "bootstrap") || report.Summary.N != 6 || report.Summary.Failures != 4 {
		t.Fatalf("report = %+v", report)
	}
}
