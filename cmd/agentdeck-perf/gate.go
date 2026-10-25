package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"path/filepath"
	"sort"
	"strconv"
)

// Relative gate (requirements "numeric acceptance", performance evaluation
// "relative judgement"): R = q(candidate) / q(baseline) from one ABBA batch;
// a deterministic paired bootstrap over ABBA groups gives a 95% interval.
const (
	bootstrapRounds = 10_000
	bootstrapSeed   = 1
)

type gateVerdict struct {
	Condition string  `json:"condition"`
	Statistic string  `json:"statistic"`
	K         float64 `json:"k"`
	Groups    int     `json:"groups"`
	Baseline  float64 `json:"baseline_ms"`
	Candidate float64 `json:"candidate_ms"`
	Ratio     float64 `json:"ratio"`
	Low       float64 `json:"ratio_low"`
	High      float64 `json:"ratio_high"`
	Verdict   string  `json:"verdict"` // pass, uncertain, fail
}

// group holds one ABBA group's values per arm.
type group struct{ baseline, candidate []interval }

func statistic(values []interval, q float64) float64 {
	sorted := make([]float64, 0, len(values))
	for _, value := range values {
		if value.Failed {
			sorted = append(sorted, math.Inf(1))
		} else {
			sorted = append(sorted, float64(value.Value)/1e6)
		}
	}
	sort.Float64s(sorted)
	return nearestRank(sorted, q)
}

func pooled(groups []group, pick func(group) []interval) []interval {
	var out []interval
	for _, g := range groups {
		out = append(out, pick(g)...)
	}
	return out
}

// relativeGate judges candidate against baseline for quantile q and bound k.
func relativeGate(groups []group, q, k float64) (gateVerdict, error) {
	if len(groups) == 0 {
		return gateVerdict{}, errors.New("no complete ABBA groups")
	}
	ratio := func(sample []group) float64 {
		b := statistic(pooled(sample, func(g group) []interval { return g.baseline }), q)
		c := statistic(pooled(sample, func(g group) []interval { return g.candidate }), q)
		if b == 0 || math.IsInf(b, 1) {
			return math.Inf(1)
		}
		return c / b
	}
	verdict := gateVerdict{K: k, Groups: len(groups), Ratio: ratio(groups),
		Baseline:  statistic(pooled(groups, func(g group) []interval { return g.baseline }), q),
		Candidate: statistic(pooled(groups, func(g group) []interval { return g.candidate }), q)}
	rng := rand.New(rand.NewPCG(bootstrapSeed, 0))
	ratios := make([]float64, bootstrapRounds)
	sample := make([]group, len(groups))
	for round := range ratios {
		for i := range sample {
			sample[i] = groups[rng.IntN(len(groups))]
		}
		ratios[round] = ratio(sample)
	}
	sort.Float64s(ratios)
	verdict.Low = ratios[int(0.025*float64(bootstrapRounds))]
	verdict.High = ratios[int(math.Ceil(0.975*float64(bootstrapRounds)))-1]
	switch {
	case verdict.High <= k:
		verdict.Verdict = "pass"
	case verdict.Ratio <= k:
		verdict.Verdict = "uncertain"
	default:
		verdict.Verdict = "fail"
	}
	return verdict, nil
}

// armValues extracts one run unit's values for a latency condition, with the
// same oracle and schedule checks as stats.
func armValues(dir, condition string, s schedule) ([]interval, error) {
	if conditionIntervals[condition].end == "" && condition != "P10" {
		return nil, fmt.Errorf("gate: %s has no relative latency gate", condition)
	}
	files, err := readSample(dir)
	if err != nil {
		return nil, err
	}
	return sampleIntervals(dir, condition, files, s)
}

// gateSchedule admits only a formal batch run with the condition's approved
// formal schedule, whose run units form complete A B B A groups (an even
// number of units, two per group). A tool pilot produces no gate result.
func gateSchedule(condition string, schedules ...schedule) (schedule, error) {
	plan, ok := conditionPlans()[condition]
	if !ok {
		return schedule{}, fmt.Errorf("gate: unknown condition %s", condition)
	}
	approved := plan.formal
	approved.Mode = "formal"
	for _, s := range schedules {
		switch {
		case s.Mode != "formal":
			return schedule{}, fmt.Errorf("gate: a %q batch produces no gate result; run the formal schedule", s.Mode)
		case s != approved:
			return schedule{}, fmt.Errorf("gate: schedule %+v is not the approved formal schedule %+v", s, approved)
		}
	}
	if approved.Units < 2 || approved.Units%2 != 0 {
		return schedule{}, fmt.Errorf("gate: %d run units per arm do not form complete A B B A groups", approved.Units)
	}
	return approved, nil
}

// abbaGroups pairs run units 2g-1 and 2g of each arm into group g: runArms
// ran them A B, B A, so each group is the four run units A B B A. A
// tool-invalid unit in either arm invalidates the whole run, as for any batch.
func abbaGroups(root, condition string) ([]group, error) {
	if condition == "P09" {
		return p09Groups(root)
	}
	baseline, err := sampleDirs(filepath.Join(root, "baseline"))
	if err != nil {
		return nil, err
	}
	candidate, err := sampleDirs(filepath.Join(root, "candidate"))
	if err != nil {
		return nil, err
	}
	baselineSchedule, err := readSchedule(filepath.Join(root, "baseline"))
	if err != nil {
		return nil, err
	}
	candidateSchedule, err := readSchedule(filepath.Join(root, "candidate"))
	if err != nil {
		return nil, err
	}
	s, err := gateSchedule(condition, baselineSchedule, candidateSchedule)
	if err != nil {
		return nil, err
	}
	if len(baseline) != s.Units || len(candidate) != s.Units {
		return nil, fmt.Errorf("gate needs %d run units in both arms, got %d and %d", s.Units, len(baseline), len(candidate))
	}
	var groups []group
	for i := 0; i < len(baseline); i += 2 {
		var g group
		for j := i; j < i+2; j++ {
			b, err := armValues(baseline[j], condition, s)
			if err != nil {
				return nil, fmt.Errorf("baseline %s: %w", filepath.Base(baseline[j]), err)
			}
			c, err := armValues(candidate[j], condition, s)
			if err != nil {
				return nil, fmt.Errorf("candidate %s: %w", filepath.Base(candidate[j]), err)
			}
			g.baseline, g.candidate = append(g.baseline, b...), append(g.candidate, c...)
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// p09Groups reads P09's B0 (baseline) and S0 (candidate) wall times from one
// batch: units 2g-1 and 2g, ordered B0 S0 then S0 B0, are ABBA group g.
func p09Groups(root string) ([]group, error) {
	dirs, err := sampleDirs(root)
	if err != nil {
		return nil, err
	}
	recorded, err := readSchedule(root)
	if err != nil {
		return nil, err
	}
	s, err := gateSchedule("P09", recorded)
	if err != nil {
		return nil, err
	}
	if len(dirs) != s.Units {
		return nil, fmt.Errorf("gate needs %d B0/S0 pairs, got %d", s.Units, len(dirs))
	}
	var groups []group
	for i := 0; i < len(dirs); i += 2 {
		var g group
		for j := i; j < i+2; j++ {
			runs, err := p09Pair(dirs[j], j+1)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Base(dirs[j]), err)
			}
			for _, run := range runs {
				value := interval{Value: run.WallNS, Failed: run.ExitCode != 0}
				if run.Label == "B0" {
					g.baseline = append(g.baseline, value)
				} else {
					g.candidate = append(g.candidate, value)
				}
			}
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// gateReport is P03's verdict: each dimension is judged on its own, and the
// condition passes only when every dimension passes.
type gateReport struct {
	Condition  string                 `json:"condition"`
	Statistic  string                 `json:"statistic"`
	K          float64                `json:"k"`
	Verdict    string                 `json:"verdict"`
	Dimensions map[string]gateVerdict `json:"dimensions"`
}

// dimensionGate judges P03 per dimension with its own resampled groups.
func dimensionGate(groups []group, q, k float64) (gateReport, error) {
	report := gateReport{K: k, Verdict: "pass", Dimensions: map[string]gateVerdict{}}
	for _, dimension := range dimensions {
		var own []group
		for _, g := range groups {
			own = append(own, group{baseline: byDimension(g.baseline, dimension.code), candidate: byDimension(g.candidate, dimension.code)})
		}
		for _, g := range own {
			if len(g.baseline) == 0 || len(g.candidate) == 0 {
				return gateReport{}, fmt.Errorf("gate: a group has no %s selections", dimension.name)
			}
		}
		verdict, err := relativeGate(own, q, k)
		if err != nil {
			return gateReport{}, err
		}
		verdict.Condition = "P03-" + dimension.name
		report.Dimensions[dimension.name] = verdict
		switch {
		case verdict.Verdict == "fail":
			report.Verdict = "fail"
		case verdict.Verdict == "uncertain" && report.Verdict == "pass":
			report.Verdict = "uncertain"
		}
	}
	return report, nil
}

func runGate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("batch-out", "", "condition output: baseline/ and candidate/ arms, or a P09 batch with B0/S0 pairs")
	condition := flags.String("condition", "", "latency condition, e.g. P01-first")
	k := flags.String("k", "", "bound fixed by baseline-remeasure (1 = no regression)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	bound, err := strconv.ParseFloat(*k, 64)
	if err != nil || bound <= 0 {
		return report(stderr, "gate", errors.New("--k must be a positive number"))
	}
	// Latency gates use p95; P09's gate is the median wall time.
	q := 0.95
	if *condition == "P09" {
		q = 0.5
	}
	statisticName := fmt.Sprintf("p%.0f", q*100)
	groups, err := abbaGroups(*root, *condition)
	if err != nil {
		return report(stderr, "gate", err)
	}
	if *condition == "P03" {
		result, err := dimensionGate(groups, q, bound)
		if err == nil {
			result.Condition, result.Statistic = *condition, statisticName
			err = printJSON(stdout, result)
		}
		return report(stderr, "gate", err)
	}
	verdict, err := relativeGate(groups, q, bound)
	if err == nil {
		verdict.Condition, verdict.Statistic = *condition, statisticName
		err = printJSON(stdout, verdict)
	}
	return report(stderr, "gate", err)
}
