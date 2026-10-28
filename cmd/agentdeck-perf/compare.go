package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"path/filepath"
	"sort"
)

// Before/after comparison (requirements "performance measurement and report",
// performance evaluation "comparison report"). There is no target and no
// verdict: R = q(candidate) / q(baseline) from one ABBA batch, with a
// deterministic paired bootstrap over ABBA groups for its 95% interval.
const (
	bootstrapRounds = 10_000
	bootstrapSeed   = 1
)

// ratioReport is one statistic of an ABBA comparison.
type ratioReport struct {
	Statistic string      `json:"statistic"`
	Groups    int         `json:"groups"`
	Baseline  reportFloat `json:"baseline_ms"`
	Candidate reportFloat `json:"candidate_ms"`
	Ratio     reportFloat `json:"ratio"`
	Low       reportFloat `json:"ratio_low"`
	High      reportFloat `json:"ratio_high"`
}

// reportFloat encodes a product failure's +Inf (a statistic that reached an
// unanswered sample) as the string "inf"; JSON has no infinity.
type reportFloat float64

func (f reportFloat) MarshalJSON() ([]byte, error) {
	if math.IsInf(float64(f), 0) || math.IsNaN(float64(f)) {
		return json.Marshal(fmt.Sprint(float64(f)))
	}
	return json.Marshal(float64(f))
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

// ratioInterval compares the arms' quantile q over complete ABBA groups.
func ratioInterval(groups []group, q float64) (ratioReport, error) {
	if len(groups) == 0 {
		return ratioReport{}, errors.New("no complete ABBA groups")
	}
	ratio := func(sample []group) float64 {
		b := statistic(pooled(sample, func(g group) []interval { return g.baseline }), q)
		c := statistic(pooled(sample, func(g group) []interval { return g.candidate }), q)
		if b == 0 || math.IsInf(b, 1) {
			return math.Inf(1)
		}
		return c / b
	}
	result := ratioReport{Statistic: statisticName(q), Groups: len(groups), Ratio: reportFloat(ratio(groups)),
		Baseline:  reportFloat(statistic(pooled(groups, func(g group) []interval { return g.baseline }), q)),
		Candidate: reportFloat(statistic(pooled(groups, func(g group) []interval { return g.candidate }), q))}
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
	result.Low = reportFloat(ratios[int(0.025*float64(bootstrapRounds))])
	result.High = reportFloat(ratios[int(math.Ceil(0.975*float64(bootstrapRounds)))-1])
	return result, nil
}

func statisticName(q float64) string {
	if q == 0.5 {
		return "median"
	}
	return fmt.Sprintf("p%.0f", q*100)
}

// abbaQuantiles are the reported statistics: latency conditions report
// median, p95 and p99; P09 reports the median wall time.
func abbaQuantiles(condition string) []float64 {
	if condition == "P09" {
		return []float64{0.5}
	}
	return []float64{0.5, 0.95, 0.99}
}

func ratios(groups []group, condition string) ([]ratioReport, error) {
	var out []ratioReport
	for _, q := range abbaQuantiles(condition) {
		r, err := ratioInterval(groups, q)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// armValues extracts one run unit's values for a latency condition, with the
// same oracle and schedule checks as stats.
func armValues(dir, condition string, s schedule) ([]interval, error) {
	if conditionIntervals[condition].end == "" && condition != "P10" {
		return nil, fmt.Errorf("compare: %s is not an ABBA latency condition", condition)
	}
	files, err := readSample(dir)
	if err != nil {
		return nil, err
	}
	return sampleIntervals(dir, condition, files, s)
}

// compareSchedule admits only a formal batch run with the condition's approved
// formal schedule, whose run units form complete A B B A groups (an even
// number of units, two per group). A tool pilot produces no comparison.
func compareSchedule(condition string, schedules ...schedule) (schedule, error) {
	plan, ok := conditionPlans()[condition]
	if !ok {
		return schedule{}, fmt.Errorf("compare: unknown condition %s", condition)
	}
	approved := plan.formal
	approved.Mode = "formal"
	for _, s := range schedules {
		switch {
		case s.Mode != "formal":
			return schedule{}, fmt.Errorf("compare: a %q batch produces no comparison; run the formal schedule", s.Mode)
		case s != approved:
			return schedule{}, fmt.Errorf("compare: schedule %+v is not the approved formal schedule %+v", s, approved)
		}
	}
	if approved.Units < 2 || approved.Units%2 != 0 {
		return schedule{}, fmt.Errorf("compare: %d run units per arm do not form complete A B B A groups", approved.Units)
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
	s, err := compareSchedule(condition, baselineSchedule, candidateSchedule)
	if err != nil {
		return nil, err
	}
	if len(baseline) != s.Units || len(candidate) != s.Units {
		return nil, fmt.Errorf("compare needs %d run units in both arms, got %d and %d", s.Units, len(baseline), len(candidate))
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
	s, err := compareSchedule("P09", recorded)
	if err != nil {
		return nil, err
	}
	if len(dirs) != s.Units {
		return nil, fmt.Errorf("compare needs %d B0/S0 pairs, got %d", s.Units, len(dirs))
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

// abbaComparison reports every statistic of an ABBA condition; P03 is
// reported per dimension, each with its own resampled groups.
type abbaComparison struct {
	Condition  string                   `json:"condition"`
	Method     string                   `json:"method"`
	Ratios     []ratioReport            `json:"ratios,omitempty"`
	Dimensions map[string][]ratioReport `json:"dimensions,omitempty"`
}

func compareABBA(root, condition string) (abbaComparison, error) {
	result := abbaComparison{Condition: condition, Method: "abba_ratio"}
	groups, err := abbaGroups(root, condition)
	if err != nil {
		return result, err
	}
	if condition != "P03" {
		result.Ratios, err = ratios(groups, condition)
		return result, err
	}
	result.Dimensions = map[string][]ratioReport{}
	for _, dimension := range dimensions {
		var own []group
		for _, g := range groups {
			own = append(own, group{baseline: byDimension(g.baseline, dimension.code), candidate: byDimension(g.candidate, dimension.code)})
		}
		for _, g := range own {
			if len(g.baseline) == 0 || len(g.candidate) == 0 {
				return result, fmt.Errorf("compare: a group has no %s selections", dimension.name)
			}
		}
		if result.Dimensions[dimension.name], err = ratios(own, condition); err != nil {
			return result, err
		}
	}
	return result, nil
}

// armComparison places the two arms' own reports side by side for the
// conditions whose run units cannot form A B B A groups: P06 adds per-pair
// differences, P07 and P11-full the ratio of medians, P08 its counts.
type armComparison struct {
	Condition string         `json:"condition"`
	Method    string         `json:"method"`
	Baseline  batchReport    `json:"baseline"`
	Candidate batchReport    `json:"candidate"`
	Derived   map[string]any `json:"derived,omitempty"`
}

var armMethods = map[string]string{"P06": "paired_difference", "P07": "side_by_side", "P11-full": "side_by_side", "P08": "counts"}

func compareArms(root, condition string) (armComparison, error) {
	result := armComparison{Condition: condition, Method: armMethods[condition], Derived: map[string]any{}}
	for _, arm := range []struct {
		dir string
		out *batchReport
	}{{"baseline", &result.Baseline}, {"candidate", &result.Candidate}} {
		dir := filepath.Join(root, arm.dir)
		s, err := readSchedule(dir)
		if err == nil {
			_, err = formalSchedule(condition, s)
		}
		if err != nil {
			return result, fmt.Errorf("%s: %w", arm.dir, err)
		}
		var buf bytes.Buffer
		if err := runStats(dir, condition, &buf); err != nil {
			return result, fmt.Errorf("%s: %w", arm.dir, err)
		}
		if err := json.Unmarshal(buf.Bytes(), arm.out); err != nil {
			return result, err
		}
		if arm.out.ToolInvalid != 0 {
			return result, fmt.Errorf("%s: %d tool-invalid run units", arm.dir, arm.out.ToolInvalid)
		}
	}
	switch condition {
	case "P06":
		result.Derived["pairs"] = pairedDifferences(result.Baseline.Values, result.Candidate.Values)
	case "P07", "P11-full":
		if b, c := result.Baseline.Summary, result.Candidate.Summary; b != nil && c != nil && b.MedianMS > 0 {
			result.Derived["median_ratio"] = c.MedianMS / b.MedianMS
		}
	}
	return result, nil
}

// formalSchedule admits only the condition's approved formal schedule.
func formalSchedule(condition string, s schedule) (schedule, error) {
	plan, ok := conditionPlans()[condition]
	if !ok {
		return schedule{}, fmt.Errorf("compare: unknown condition %s", condition)
	}
	approved := plan.formal
	approved.Mode = "formal"
	if s != approved {
		return schedule{}, fmt.Errorf("compare: schedule %+v is not the approved formal schedule %+v", s, approved)
	}
	return approved, nil
}

// pairedDifferences subtracts each numeric field of the baseline's window from
// the candidate's window of the same run unit (P06: "candidate - baseline").
func pairedDifferences(baseline, candidate map[string]any) map[string]map[string]float64 {
	out := map[string]map[string]float64{}
	for unit, raw := range candidate {
		c, ok := raw.(map[string]any)
		b, found := baseline[unit].(map[string]any)
		if !ok || !found {
			continue
		}
		diff := map[string]float64{}
		for key, value := range c {
			cv, cok := value.(float64)
			bv, bok := b[key].(float64)
			if cok && bok {
				diff[key] = cv - bv
			}
		}
		out[unit] = diff
	}
	return out
}

func runCompare(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("batch-out", "", "condition output: baseline/ and candidate/ arms, or a P09 batch with B0/S0 pairs")
	condition := flags.String("condition", "", "condition, e.g. P01-first")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	switch {
	case *condition == "P05":
		return report(stderr, "compare", errors.New("P05 has no baseline arm; report it with stats"))
	case armMethods[*condition] != "":
		result, err := compareArms(*root, *condition)
		if err == nil {
			err = printJSON(stdout, result)
		}
		return report(stderr, "compare", err)
	}
	result, err := compareABBA(*root, *condition)
	if err == nil {
		err = printJSON(stdout, result)
	}
	return report(stderr, "compare", err)
}
