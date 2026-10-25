package main

import (
	"bytes"
	"encoding/json"
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

// A regression in one dimension fails P03 even when the others improve.
func TestP03GateJudgesEachDimension(t *testing.T) {
	root := t.TempDir()
	p03Arm(t, filepath.Join(root, "baseline"), "formal", 4, 25, [3]int64{1000, 1000, 100})
	p03Arm(t, filepath.Join(root, "candidate"), "formal", 4, 25, [3]int64{100, 100, 200})
	groups, err := abbaGroups(root, "P03")
	if err != nil {
		t.Fatal(err)
	}
	result, err := dimensionGate(groups, 0.95, 1)
	if err != nil || result.Verdict != "fail" || result.Dimensions["panel"].Verdict != "fail" ||
		result.Dimensions["client"].Verdict != "pass" || result.Dimensions["panel"].Ratio != 2 {
		t.Fatalf("P03 gate = %+v %v", result, err)
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
	if code := runGate([]string{"--batch-out", root, "--condition", "P09", "--k", "0.7"}, &out, &stderr); code != 0 {
		t.Fatalf("gate exit %d: %s", code, stderr.String())
	}
	var verdict gateVerdict
	if err := json.Unmarshal(out.Bytes(), &verdict); err != nil || verdict.Verdict != "pass" || verdict.Statistic != "p50" || verdict.Groups != 10 {
		t.Fatalf("P09 gate = %s %v", out.String(), err)
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

func TestRelativeGateVerdicts(t *testing.T) {
	pass, err := relativeGate(groupsOf(20, 500, 250, 1), 0.95, 0.7)
	if err != nil || pass.Verdict != "pass" || pass.High > 0.7 {
		t.Fatalf("50%% faster under k=0.7: %+v %v", pass, err)
	}
	fail, _ := relativeGate(groupsOf(20, 500, 500, 1), 0.95, 0.5)
	if fail.Verdict != "fail" {
		t.Fatalf("unchanged candidate under k=0.5: %+v", fail)
	}
	edge, _ := relativeGate(groupsOf(4, 500, 495, 3), 0.95, 1)
	if edge.Verdict == "fail" && edge.Ratio <= 1 {
		t.Fatalf("point estimate below k must not be a fail: %+v", edge)
	}
	again, _ := relativeGate(groupsOf(20, 500, 250, 1), 0.95, 0.7)
	if again != pass {
		t.Fatalf("bootstrap is not deterministic: %+v vs %+v", again, pass)
	}
}

func TestRelativeGateCountsProductFailuresAsInfinite(t *testing.T) {
	groups := groupsOf(10, 500, 100, 4)
	for i := range groups {
		for j := 0; j < 5; j++ { // half the candidate's samples never reached the frame
			groups[i].candidate[j] = interval{Failed: true}
		}
	}
	verdict, _ := relativeGate(groups, 0.95, 0.7)
	if verdict.Verdict != "fail" {
		t.Fatalf("failures in the tail must fail the gate: %+v", verdict)
	}
}

// A tool pilot, arms run with different schedules, or a schedule other than
// the approved formal one produce no gate result (review Round 2 F2: a
// one-unit pilot per arm used to pass with a point interval).
func TestGateAdmitsOnlyTheApprovedFormalSchedule(t *testing.T) {
	pilot := t.TempDir()
	p03Arm(t, filepath.Join(pilot, "baseline"), "pilot", 1, 20, [3]int64{100, 100, 100})
	p03Arm(t, filepath.Join(pilot, "candidate"), "pilot", 1, 20, [3]int64{50, 50, 50})
	var out, stderr bytes.Buffer
	if code := runGate([]string{"--batch-out", pilot, "--condition", "P03", "--k", "1"}, &out, &stderr); code == 0 {
		t.Fatalf("a pilot produced a gate result: %s", out.String())
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
	if _, err := gateSchedule("P06", schedule{Mode: "formal", Units: 3, Window: 1800}); err == nil {
		t.Fatal("an odd number of run units must not reach a gate result")
	}
	// P11-full's 5 completion runs are reported, not gated, so the gate
	// refusing them is correct; every gated latency condition must form groups.
	for condition, plan := range conditionPlans() {
		if _, ok := conditionIntervals[condition]; ok && condition != "P11-full" && plan.formal.Units%2 != 0 {
			t.Fatalf("%s formal schedule has %d units, not complete A B B A groups", condition, plan.formal.Units)
		}
	}
}
