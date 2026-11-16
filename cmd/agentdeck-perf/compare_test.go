package main

import (
	"bytes"
	"encoding/json"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

// p03Arm writes one arm of a P03 relative run: units of inner selections
// per dimension with fixed latencies (ms) per dimension.
func p03Arm(t *testing.T, root, mode string, units, inner int, latency [3]int64) {
	t.Helper()
	writeSchedule(t, root, schedule{Mode: mode, Units: units, Inner: inner})
	o := oracleJSON(t, oracle{Heroes: []heroSummary{today}})
	for unit := 1; unit <= units; unit++ {
		var records []traceRecord
		at := int64(0)
		for d, value := range latency {
			for range inner {
				records = append(records, traceRecord{Stage: "select.intent", T: at, N: map[string]int64{"dimension": int64(d + 1)}},
					frame(at+value*ms, today))
				at += 2000 * ms
			}
		}
		sampleDir(t, root, unit, map[string]string{"app-000001.jsonl": trace("app", records...), oracleFile: o})
	}
}

// P03 is compared per dimension; a slower dimension is reported, not judged.
func TestP03ComparesEachDimension(t *testing.T) {
	root := t.TempDir()
	p03Arm(t, filepath.Join(root, "baseline"), "formal", 4, 25, [3]int64{1000, 1000, 100})
	p03Arm(t, filepath.Join(root, "candidate"), "formal", 4, 25, [3]int64{100, 100, 200})
	result, err := compareABBA(root, "P03")
	if err != nil {
		t.Fatal(err)
	}
	panel, client := result.Dimensions["panel"], result.Dimensions["client"]
	if len(panel) != 3 || panel[1].Statistic != "p95" || panel[1].Ratio != 2 || client[1].Ratio != 0.1 {
		t.Fatalf("P03 comparison = %+v", result)
	}
}

// P09's gate is the median wall of real ABBA CLI pairs in one batch.
func TestP09GateUsesThePairedCLIRuns(t *testing.T) {
	root := t.TempDir()
	writeSchedule(t, root, schedule{Mode: "formal", Units: 20})
	for unit := 1; unit <= 20; unit++ {
		var runs []cliRun
		for _, label := range p09Order(unit) {
			wall := 10_000 * ms
			if label == "S0" {
				wall = 6_000 * ms
			}
			runs = append(runs, cliRun{Label: label, WallNS: wall + int64(unit)*ms})
		}
		raw, _ := json.Marshal(runs)
		sampleDir(t, root, unit, map[string]string{"cli-runs.json": string(raw)})
	}
	var out, stderr bytes.Buffer
	if code := runCompare([]string{"--batch-out", root, "--condition", "P09"}, &out, &stderr); code != 0 {
		t.Fatalf("compare exit %d: %s", code, stderr.String())
	}
	var result abbaComparison
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || len(result.Ratios) != 1 ||
		result.Ratios[0].Statistic != "median" || result.Ratios[0].Groups != 10 || result.Ratios[0].Ratio >= 0.7 {
		t.Fatalf("P09 comparison = %s %v", out.String(), err)
	}
	if bytes.Contains(out.Bytes(), []byte("verdict")) || bytes.Contains(out.Bytes(), []byte(`"k"`)) {
		t.Fatalf("a comparison carries no k and no verdict: %s", out.String())
	}
	// Swapped order inside a pair is not an ABBA unit.
	raw, _ := json.Marshal([]cliRun{{Label: "S0", WallNS: 1}, {Label: "B0", WallNS: 1}})
	if err := os.WriteFile(filepath.Join(root, "sample-0001", "cli-runs.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := p09Groups(root); err == nil {
		t.Fatal("a pair out of ABBA order must be rejected")
	}
}

// groupsOf builds ABBA groups with per-arm values around the given means (ms),
// with the same multiplicative noise in both arms as a shared machine load.
func groupsOf(n int, baseMS, candMS float64, seed uint64) []group {
	rng := rand.New(rand.NewPCG(seed, 2))
	groups := make([]group, n)
	for i := range groups {
		load := 1 + rng.Float64() // shared by the group's run units
		for j := 0; j < 10; j++ {
			noise := 0.9 + 0.2*rng.Float64()
			groups[i].baseline = append(groups[i].baseline, interval{Value: int64(baseMS * load * noise * 1e6)})
			groups[i].candidate = append(groups[i].candidate, interval{Value: int64(candMS * load * noise * 1e6)})
		}
	}
	return groups
}

func TestRatioIntervalIsDeterministicAndBracketsTheRatio(t *testing.T) {
	faster, err := ratioInterval(groupsOf(20, 500, 250, 1), 0.95)
	if err != nil || faster.Ratio > 0.6 || faster.Low > faster.Ratio || faster.High < faster.Ratio || faster.Statistic != "p95" {
		t.Fatalf("50%% faster: %+v %v", faster, err)
	}
	again, _ := ratioInterval(groupsOf(20, 500, 250, 1), 0.95)
	if again != faster {
		t.Fatalf("bootstrap is not deterministic: %+v vs %+v", again, faster)
	}
	if _, err := ratioInterval(nil, 0.5); err == nil {
		t.Fatal("no groups must not produce a comparison")
	}
}

func TestRatioIntervalCountsProductFailuresAsInfinite(t *testing.T) {
	groups := groupsOf(10, 500, 100, 4)
	for i := range groups {
		for j := 0; j < 5; j++ { // half the candidate's samples never reached the frame
			groups[i].candidate[j] = interval{Failed: true}
		}
	}
	result, _ := ratioInterval(groups, 0.95)
	if !math.IsInf(float64(result.Ratio), 1) {
		t.Fatalf("failures in the tail must make the p95 infinite: %+v", result)
	}
	raw, err := json.Marshal(result)
	if err != nil || !bytes.Contains(raw, []byte(`"ratio":"+Inf"`)) {
		t.Fatalf("an infinite statistic must still encode: %s %v", raw, err)
	}
}

// P06's three paired windows are reported as per-unit differences.
func TestPairedDifferencesSubtractTheBaselineWindow(t *testing.T) {
	baseline := map[string]any{"sample-0001": map[string]any{"cpu": 0.004, "label": "x"}, "schedule": map[string]any{"units": 3.0}}
	candidate := map[string]any{"sample-0001": map[string]any{"cpu": 0.003, "label": "x"}, "sample-0002": map[string]any{"cpu": 1.0}, "schedule": map[string]any{"units": 3.0}}
	got := pairedDifferences(baseline, candidate)
	if len(got) != 1 || math.Abs(got["sample-0001"]["cpu"]+0.001) > 1e-12 {
		t.Fatalf("differences = %v", got)
	}
}

func TestPairedDifferenceMediansPreserveSignedChanges(t *testing.T) {
	got := pairedDifferenceMedians(map[string]map[string]float64{
		"sample-0001": {"cpu": 2, "rss": -10},
		"sample-0002": {"cpu": -4, "rss": 8},
		"sample-0003": {"cpu": 1, "rss": 3},
	})
	if got["cpu"] != 1 || got["rss"] != 3 {
		t.Fatalf("paired medians = %v", got)
	}
}

// P05 has no baseline arm, so compare refuses it.
func TestCompareRefusesP05(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := runCompare([]string{"--batch-out", t.TempDir(), "--condition", "P05"}, &out, &stderr); code == 0 {
		t.Fatalf("P05 produced a comparison: %s", out.String())
	}
}

// A tool pilot, arms run with different schedules, or a schedule other than
// the approved formal one produce no comparison (review Round 2 F2: a
// one-unit pilot per arm used to pass with a point interval).
func TestCompareAdmitsOnlyTheApprovedFormalSchedule(t *testing.T) {
	pilot := t.TempDir()
	p03Arm(t, filepath.Join(pilot, "baseline"), "pilot", 1, 20, [3]int64{100, 100, 100})
	p03Arm(t, filepath.Join(pilot, "candidate"), "pilot", 1, 20, [3]int64{50, 50, 50})
	var out, stderr bytes.Buffer
	if code := runCompare([]string{"--batch-out", pilot, "--condition", "P03"}, &out, &stderr); code == 0 {
		t.Fatalf("a pilot produced a comparison: %s", out.String())
	}
	mismatch := t.TempDir()
	p03Arm(t, filepath.Join(mismatch, "baseline"), "formal", 4, 25, [3]int64{100, 100, 100})
	p03Arm(t, filepath.Join(mismatch, "candidate"), "formal", 2, 25, [3]int64{50, 50, 50})
	if _, err := abbaGroups(mismatch, "P03"); err == nil {
		t.Fatal("arms with different schedules must be rejected")
	}
	unapproved := t.TempDir()
	p03Arm(t, filepath.Join(unapproved, "baseline"), "formal", 2, 25, [3]int64{100, 100, 100})
	p03Arm(t, filepath.Join(unapproved, "candidate"), "formal", 2, 25, [3]int64{50, 50, 50})
	if _, err := abbaGroups(unapproved, "P03"); err == nil {
		t.Fatal("a schedule other than the approved formal one must be rejected")
	}
	// A formal schedule whose units cannot form complete A B B A groups.
	if _, err := compareSchedule("P06", schedule{Mode: "formal", Units: 3, Window: 1800}); err == nil {
		t.Fatal("an odd number of run units must not reach an ABBA comparison")
	}
	// P11-full's 5 completion runs are compared side by side, so the ABBA
	// path refusing them is correct; every ABBA latency condition must form groups.
	for condition, plan := range conditionPlans() {
		if _, ok := conditionIntervals[condition]; ok && condition != "P11-full" && plan.formal.Units%2 != 0 {
			t.Fatalf("%s formal schedule has %d units, not complete A B B A groups", condition, plan.formal.Units)
		}
	}
}
