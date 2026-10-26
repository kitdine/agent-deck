package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// traceRecord is one line of a perftrace file (Go or Swift writer).
type traceRecord struct {
	Stage string           `json:"stage"`
	T     int64            `json:"t"`
	Dur   *int64           `json:"dur"`
	N     map[string]int64 `json:"n"`
	Err   string           `json:"err"`
}

// traceFile is one process's trace: header plus records in file order.
type traceFile struct {
	Role    string
	Seq     int
	Start   int64
	Records []traceRecord
}

func (f traceFile) end() (int64, bool) {
	for i := len(f.Records) - 1; i >= 0; i-- {
		if f.Records[i].Stage == "resource.lifetime" {
			return f.Records[i].T, true
		}
	}
	return 0, false
}

func readTraceFile(path string) (traceFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return traceFile{}, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(nil, 1<<20)
	var out traceFile
	first := true
	for scanner.Scan() {
		if first {
			var header struct {
				V     int    `json:"v"`
				Role  string `json:"role"`
				Seq   int    `json:"seq"`
				Clock string `json:"clock"`
				T     int64  `json:"t"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &header); err != nil || header.V != 1 || header.Clock != "CLOCK_UPTIME_RAW" {
				return traceFile{}, fmt.Errorf("%s: invalid trace header", filepath.Base(path))
			}
			out.Role, out.Seq, out.Start, first = header.Role, header.Seq, header.T, false
			continue
		}
		var record traceRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return traceFile{}, fmt.Errorf("%s: malformed record", filepath.Base(path))
		}
		out.Records = append(out.Records, record)
	}
	if err := scanner.Err(); err != nil {
		return traceFile{}, err
	}
	if first {
		return traceFile{}, fmt.Errorf("%s: empty trace", filepath.Base(path))
	}
	return out, nil
}

// readSample reads every trace of one sample directory. Any rejected marker,
// dropped record or missing lifetime record makes the sample tool-invalid.
func readSample(dir string) ([]traceFile, error) {
	if raw, err := os.ReadFile(filepath.Join(dir, "invalid.json")); err == nil {
		var invalid struct {
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(raw, &invalid)
		return nil, errors.New(invalid.Reason)
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var files []traceFile
	for _, path := range paths {
		file, err := readTraceFile(path)
		if err != nil {
			return nil, err
		}
		for _, record := range file.Records {
			if record.Stage == "trace.invalid" || record.Stage == "driver.invalid" {
				return nil, fmt.Errorf("%s: %s", filepath.Base(path), record.Stage)
			}
		}
		if _, ok := file.end(); !ok {
			return nil, fmt.Errorf("%s: missing resource.lifetime (process did not flush)", filepath.Base(path))
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return nil, errors.New("no trace files")
	}
	return files, nil
}

// interval is one measured value; Failed marks a product failure counted as +Inf.
type interval struct {
	Value  int64
	Failed bool
	// Start is the counts of the start marker (e.g. P03's dimension); End
	// the end marker's (a data frame's hero). StartT and EndT are their
	// times; EndT is 0 when no end arrived.
	Start, End   map[string]int64
	StartT, EndT int64
}

// pairIntervals measures each start marker to the first end marker after it
// and before the next start. A start with no end within timeout is a product
// failure; an end without a start is ignored.
func pairIntervals(records []traceRecord, start, end string, timeout int64, accept func(traceRecord) bool) []interval {
	var out []interval
	for i, record := range records {
		if record.Stage != start {
			continue
		}
		next := int64(math.MaxInt64)
		for _, later := range records[i+1:] {
			if later.Stage == start {
				next = later.T
				break
			}
		}
		found := false
		for _, later := range records[i+1:] {
			if later.T >= next {
				break
			}
			if later.Stage == end && later.T >= record.T && (accept == nil || accept(later)) {
				value := later.T - record.T
				out = append(out, interval{Value: value, Failed: value > timeout, Start: record.N, End: later.N,
					StartT: record.T, EndT: later.T})
				found = true
				break
			}
		}
		if !found {
			out = append(out, interval{Failed: true, Start: record.N, StartT: record.T})
		}
	}
	return out
}

// summary is the per-batch statistics block of the measurement contract.
type summary struct {
	N        int     `json:"n"`
	Failures int     `json:"failures"`
	MinMS    float64 `json:"min_ms"`
	MedianMS float64 `json:"median_ms"`
	P95MS    float64 `json:"p95_ms"`
	P99MS    float64 `json:"p99_ms"`
	MaxMS    float64 `json:"max_ms"`
}

// nearestRank returns sorted[ceil(q*n)-1]; failures sort as +Inf.
func nearestRank(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	index := int(math.Ceil(q*float64(len(sorted)))) - 1
	return sorted[max(0, min(index, len(sorted)-1))]
}

func summarize(values []interval) summary {
	sorted := make([]float64, 0, len(values))
	failures := 0
	for _, value := range values {
		if value.Failed {
			failures++
			sorted = append(sorted, math.Inf(1))
			continue
		}
		sorted = append(sorted, float64(value.Value)/1e6)
	}
	sort.Float64s(sorted)
	if len(sorted) == 0 {
		return summary{}
	}
	return summary{
		N: len(sorted), Failures: failures, MinMS: sorted[0], MedianMS: nearestRank(sorted, 0.5),
		P95MS: nearestRank(sorted, 0.95), P99MS: nearestRank(sorted, 0.99), MaxMS: sorted[len(sorted)-1],
	}
}

// MarshalJSON writes +Inf (product failures in the tail) as null.
func (s summary) MarshalJSON() ([]byte, error) {
	finite := func(v float64) *float64 {
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return nil
		}
		return &v
	}
	return json.Marshal(struct {
		N        int      `json:"n"`
		Failures int      `json:"failures"`
		MinMS    *float64 `json:"min_ms"`
		MedianMS *float64 `json:"median_ms"`
		P95MS    *float64 `json:"p95_ms"`
		P99MS    *float64 `json:"p99_ms"`
		MaxMS    *float64 `json:"max_ms"`
	}{s.N, s.Failures, finite(s.MinMS), finite(s.MedianMS), finite(s.P95MS), finite(s.P99MS), finite(s.MaxMS)})
}

// peakRSSUpperBound sums ru_maxrss of helper/collector/cli processes whose
// lifetimes overlap, maximised over time. Each process's peak bounds its RSS at
// every instant, so the sum bounds the true aggregate peak (P07).
func peakRSSUpperBound(files []traceFile) int64 {
	type event struct {
		t     int64
		delta int64
	}
	var events []event
	for _, file := range files {
		if file.Role == "app" {
			continue
		}
		end, ok := file.end()
		if !ok {
			continue
		}
		var rss int64
		for _, record := range file.Records {
			if record.Stage == "resource.lifetime" {
				rss = record.N["maxrss_bytes"]
			}
		}
		events = append(events, event{file.Start, rss}, event{end, -rss})
	}
	// Starts sort before ends at the same instant: touching lifetimes overlap.
	sort.Slice(events, func(i, j int) bool {
		if events[i].t != events[j].t {
			return events[i].t < events[j].t
		}
		return events[i].delta > events[j].delta
	})
	var current, peak int64
	for _, e := range events {
		current += e.delta
		peak = max(peak, current)
	}
	return peak
}

// conditionIntervals maps an interval condition to its markers and timeout.
// skipFirst drops each process's first pair (P01's repeated-click layer).
// P01 starts at input.action: the measurement-harness review adopted the
// pre-declared alternative stimulus, and P01-routing reports the excluded
// input.event → input.action segment.
var conditionIntervals = map[string]struct {
	start, end string
	timeoutMS  int64
	skipFirst  bool
}{
	"P01-first":   {"input.action", "frame.vsync", 5000, false},
	"P01-repeat":  {"input.action", "frame.vsync", 5000, true},
	"P01-rebuild": {"input.action", "frame.vsync", 5000, false},
	"P02":         {"app.ready", "model.usable", 30000, false},
	"P03":         {"select.intent", "frame.vsync", 5000, false},
	"P04":         {"app.ready", "frame.vsync", 30000, false},
	"P11-first":   {"app.ready", "frame.vsync", 60000, false},
	"P11-full":    {"app.ready", "publication.complete", 3600000, false},
}

// batchReport is the stats output: sample accounting plus condition results.
type batchReport struct {
	Condition   string   `json:"condition"`
	Samples     int      `json:"samples"`
	ToolInvalid int      `json:"tool_invalid"`
	Reasons     []string `json:"tool_invalid_reasons,omitempty"`
	// Interrupted counts run-unit attempts interrupted from outside and rerun;
	// they are kept under interrupted/ and are not samples.
	Interrupted int            `json:"interrupted_reruns"`
	Summary     *summary       `json:"summary,omitempty"`
	Values      map[string]any `json:"values,omitempty"`
}

// dimensions are P03's selection dimensions by select.intent code.
var dimensions = []struct {
	code int64
	name string
}{{1, "client"}, {2, "period"}, {3, "panel"}}

// scheduledPerUnit is the number of measured values one run unit of an
// interval condition yields under its schedule.
func scheduledPerUnit(condition string, s schedule) int {
	switch condition {
	case "P01-repeat", "P10":
		return s.Inner
	case "P01-rebuild":
		return 2
	case "P03":
		return 3 * s.Inner
	}
	return 1
}

// sampleIntervals measures one run unit of an interval condition (P10
// included). Data frames are checked against the unit's oracle: a frame
// whose hero differs is a product failure. A unit that did not yield its
// scheduled values, has no usable oracle, or (P01-rebuild) has a click
// outside its rebuild is tool-invalid.
func sampleIntervals(dir, condition string, files []traceFile, s schedule) ([]interval, error) {
	want := scheduledPerUnit(condition, s)
	if condition == "P10" {
		expected, err := readOracle(dir)
		if err != nil {
			return nil, err
		}
		values, err := writeToFrame(files, expected)
		if err == nil && len(values) != want {
			err = fmt.Errorf("schedule: %d writes, want %d", len(values), want)
		}
		return values, err
	}
	spec := conditionIntervals[condition]
	var values []interval
	for _, file := range files {
		if file.Role != "app" {
			continue
		}
		// The product never became usable, so none of the unit's own stimuli
		// ran: each is a product failure, counted as +Inf. P02's own interval
		// already records it.
		if spec.start != "app.ready" && hasStage(file.Records, "driver.precondition_timeout") {
			for i := range want {
				values = append(values, interval{Failed: true, Start: map[string]int64{"dimension": int64(i%3 + 1)}})
			}
			return values, nil
		}
		pairs := pairIntervals(file.Records, spec.start, spec.end, spec.timeoutMS*1e6, nil)
		if spec.skipFirst && len(pairs) > 0 {
			pairs = pairs[1:]
		}
		values = append(values, pairs...)
	}
	if len(values) != want {
		return nil, fmt.Errorf("schedule: %d values, want %d", len(values), want)
	}
	if condition == "P03" {
		for _, dimension := range dimensions {
			count := 0
			for _, value := range values {
				if value.Start["dimension"] == dimension.code {
					count++
				}
			}
			if count != s.Inner {
				return nil, fmt.Errorf("schedule: %d %s selections, want %d", count, dimension.name, s.Inner)
			}
		}
	}
	if plan := conditionPlans()[condition]; plan.oracle {
		expected, err := readOracle(dir)
		if err != nil {
			return nil, err
		}
		if plan.bootstrap && len(expected.Batches) == 0 {
			return nil, errors.New("oracle has no bootstrap publications")
		}
		for i := range values {
			if values[i].Failed {
				continue
			}
			end := heroFromCounts(values[i].End)
			heroes := oracle{Heroes: expected.Heroes}
			partialShown := values[i].End["partial"] == 1
			wantPartial := false
			if plan.bootstrap {
				// The frame shows one bootstrap publication: the one with its
				// coverage. Any other coverage, hero or marker is wrong content.
				batch, ok := expected.expectBatch(values[i].End["committed"], values[i].End["sources"],
					values[i].End["coverage_complete"] == 1)
				if !ok {
					values[i].Failed = true
					continue
				}
				heroes.Heroes, wantPartial = batch.Heroes, batch.Partial
			}
			hero, ok := heroes.expect(end.Client, end.Period)
			if !ok && !plan.bootstrap {
				return nil, fmt.Errorf("oracle has no hero for client %d period %d", end.Client, end.Period)
			}
			values[i].Failed = !ok || end != hero || (plan.bootstrap && partialShown != wantPartial)
		}
	}
	if condition == "P01-rebuild" {
		if err := insideRebuild(dir, values); err != nil {
			return nil, err
		}
	}
	return values, nil
}

// insideRebuild requires a successful rebuild that spans every click: from
// the stimulus to its frame, or to the timeout for a click with no frame.
func insideRebuild(dir string, values []interval) error {
	raw, err := os.ReadFile(filepath.Join(dir, rebuildFile))
	if err != nil {
		return fmt.Errorf("rebuild: %w", err)
	}
	var rebuild rebuildRecord
	if err := json.Unmarshal(raw, &rebuild); err != nil {
		return fmt.Errorf("rebuild: %w", err)
	}
	if rebuild.Error != "" || rebuild.ExitCode != 0 || rebuild.EndNS <= rebuild.StartNS {
		return fmt.Errorf("rebuild did not complete: exit %d %s", rebuild.ExitCode, rebuild.Error)
	}
	for _, value := range values {
		end := value.EndT
		if end == 0 {
			end = value.StartT + conditionIntervals["P01-rebuild"].timeoutMS*1e6
		}
		if value.StartT < rebuild.StartNS || end > rebuild.EndNS {
			return errors.New("rebuild: a click is outside the full rebuild")
		}
	}
	return nil
}

// runStats measures one condition over the sample directories under root,
// against the schedule the run recorded there.
func runStats(root, condition string, w io.Writer) error {
	dirs, err := sampleDirs(root)
	if err != nil {
		return err
	}
	s, err := readSchedule(root)
	if err != nil {
		return err
	}
	report := batchReport{Condition: condition, Samples: len(dirs), Values: map[string]any{"schedule": s}}
	if attempts, err := filepath.Glob(filepath.Join(root, interruptedDir, "sample-*")); err == nil {
		report.Interrupted = len(attempts)
	}
	invalid := func(dir string, err error) {
		report.ToolInvalid++
		report.Reasons = append(report.Reasons, filepath.Base(dir)+": "+err.Error())
	}
	if len(dirs) != s.Units {
		report.ToolInvalid++
		report.Reasons = append(report.Reasons, fmt.Sprintf("schedule: %d run units, want %d", len(dirs), s.Units))
	}
	switch {
	case conditionIntervals[condition].end != "" || condition == "P10":
		var values []interval
		var rebuildRSS []int64
		for _, dir := range dirs {
			files, err := readSample(dir)
			var unit []interval
			if err == nil {
				unit, err = sampleIntervals(dir, condition, files, s)
			}
			if err != nil {
				invalid(dir, err)
				continue
			}
			values = append(values, unit...)
			switch condition {
			case "P03":
				report.Values["background_markers"] = backgroundMarkers(files)
			case "P01-rebuild":
				rebuildRSS = append(rebuildRSS, peakRSSUpperBound(files))
			}
		}
		result := summarize(values)
		report.Summary = &result
		if condition == "P03" {
			// The gate is per dimension; the pooled summary is for reference.
			for _, dimension := range dimensions {
				report.Values[dimension.name] = summarize(byDimension(values, dimension.code))
			}
		}
		if condition == "P01-rebuild" {
			// The rebuild's helper and collector processes (P07's RSS bound).
			report.Values["rebuild_rss_upper_bound_bytes"] = rebuildRSS
		}
	case condition == "P01-routing":
		var routing, eventClicks, actionClicks []interval
		unrouted := 0
		for _, dir := range dirs {
			files, err := readSample(dir)
			if err != nil {
				invalid(dir, err)
				continue
			}
			var r, e, a []interval
			u := 0
			for _, file := range files {
				if file.Role == "app" {
					r, e, a, u = routingPairs(file.Records)
				}
			}
			if len(e)+u != s.Inner || len(a) != s.Inner {
				invalid(dir, fmt.Errorf("schedule: %d event and %d action clicks, want %d each", len(e)+u, len(a), s.Inner))
				continue
			}
			routing, eventClicks, actionClicks, unrouted = append(routing, r...), append(eventClicks, e...), append(actionClicks, a...), unrouted+u
		}
		// The segment the alternative stimulus excludes: input.event → input.action.
		result := summarize(routing)
		report.Summary = &result
		report.Values["unrouted"] = unrouted
		report.Values["event_click"], report.Values["action_click"] = summarize(eventClicks), summarize(actionClicks)
	case condition == "P06":
		for _, dir := range dirs {
			files, err := readSample(dir)
			if err == nil {
				if failure := preconditionFailure(files); failure != "" {
					// No idle window exists: a product failure for this run, not
					// a tool failure.
					report.Values[filepath.Base(dir)] = map[string]any{"product_failure": failure}
					continue
				}
				var window map[string]any
				if window, err = idleWindowStats(dir, files); err == nil {
					if seconds := window["window_s"].(float64); seconds < float64(s.Window) {
						err = fmt.Errorf("schedule: window %.0f s, want %d s", seconds, s.Window)
					} else {
						report.Values[filepath.Base(dir)] = window
					}
				}
			}
			if err != nil {
				invalid(dir, err)
			}
		}
	case condition == "P07":
		var tx []interval
		var peaks []int64
		for i, dir := range dirs {
			files, err := readSample(dir)
			if err == nil {
				// Every unit is one full rebuild; only the last one continues
				// with the incremental sequence.
				increments := 0
				if i == len(dirs)-1 {
					increments = s.Inner
				}
				err = cliRunCounts(dir, map[string]int{"full": 1, "incremental": increments})
			}
			if err != nil {
				invalid(dir, err)
				continue
			}
			peaks = append(peaks, peakRSSUpperBound(files))
			for _, file := range files {
				for _, record := range file.Records {
					if strings.HasPrefix(record.Stage, "tx.") && record.Dur != nil {
						tx = append(tx, interval{Value: *record.Dur})
					}
				}
			}
		}
		result := summarize(tx)
		report.Summary = &result
		report.Values["rss_upper_bound_bytes"] = peaks
	case condition == "P08":
		for _, dir := range dirs {
			for _, scenario := range []string{"noop", "append"} {
				files, err := readSample(filepath.Join(dir, scenario))
				if err != nil {
					invalid(dir, fmt.Errorf("%s: %w", scenario, err))
					continue
				}
				counts := map[string]int64{}
				for _, file := range files {
					for _, record := range file.Records {
						if record.Stage == "scan.end" {
							for _, key := range []string{"read_bytes", "decoded_records", "usage_imported", "session_documents"} {
								counts[key] += record.N[key]
							}
						}
					}
				}
				if scenario == "append" {
					bound, err := appendBound(filepath.Join(dir, scenario), counts["read_bytes"])
					if err != nil {
						invalid(dir, err)
						continue
					}
					previous, seen := report.Values["append_bound"].(map[string]int64)
					if seen && fmt.Sprint(previous) != fmt.Sprint(bound) {
						invalid(dir, fmt.Errorf("append bound differs from the first run: %v vs %v", bound, previous))
						continue
					}
					report.Values["append_bound"] = bound
				}
				key := scenario + "_counts"
				previous, seen := report.Values[key].(map[string]int64)
				if seen && fmt.Sprint(previous) != fmt.Sprint(counts) {
					invalid(dir, fmt.Errorf("%s counters differ from the first run: %v vs %v", scenario, counts, previous))
					continue
				}
				report.Values[key] = counts
			}
		}
		if bound, ok := report.Values["append_bound"].(map[string]int64); ok {
			report.Values["append_within_bound"] = bound["read_bytes"] <= bound["bound_bytes"]
		}
	case condition == "P09":
		arms := map[string][]interval{}
		cpu := map[string][]interval{}
		costs := map[string][]p09Arm{}
		for i, dir := range dirs {
			runs, err := p09Pair(dir, i+1)
			pair := make([]p09Arm, len(runs))
			for j := 0; err == nil && j < len(runs); j++ {
				pair[j], err = p09ArmCost(dir, runs[j])
			}
			if err != nil {
				invalid(dir, err)
				continue
			}
			for j, run := range runs {
				costs[run.Label] = append(costs[run.Label], pair[j])
				arms[run.Label] = append(arms[run.Label], interval{Value: run.WallNS, Failed: run.ExitCode != 0})
				cpu[run.Label] = append(cpu[run.Label], interval{Value: run.UserNS + run.SysNS})
			}
		}
		b0, s0 := summarize(arms["B0"]), summarize(arms["S0"])
		report.Values["B0_wall"], report.Values["S0_wall"] = b0, s0
		report.Values["B0_front_cpu"], report.Values["S0_front_cpu"] = summarize(cpu["B0"]), summarize(cpu["S0"])
		for label, list := range costs {
			report.Values[label+"_rebuild"] = p09ArmSummary(list)
		}
		if b0.MedianMS > 0 {
			report.Values["s0_over_b0_median"] = s0.MedianMS / b0.MedianMS
		}
	default:
		return fmt.Errorf("stats: unknown condition %s", condition)
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func byDimension(values []interval, code int64) []interval {
	var out []interval
	for _, value := range values {
		if value.Start["dimension"] == code {
			out = append(out, value)
		}
	}
	return out
}

func readCLIRuns(dir string) ([]cliRun, error) {
	if _, err := os.Stat(filepath.Join(dir, "invalid.json")); err == nil {
		_, err = readSample(dir)
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "cli-runs.json"))
	if err != nil {
		return nil, err
	}
	var runs []cliRun
	err = json.Unmarshal(raw, &runs)
	return runs, err
}

// cliRunCounts requires exactly the scheduled number of CLI runs per label.
func cliRunCounts(dir string, want map[string]int) error {
	runs, err := readCLIRuns(dir)
	if err != nil {
		return err
	}
	got := map[string]int{}
	for _, run := range runs {
		got[run.Label]++
	}
	for label, count := range got {
		if want[label] != count {
			return fmt.Errorf("schedule: %d %s runs, want %d", count, label, want[label])
		}
	}
	for label, count := range want {
		if got[label] != count {
			return fmt.Errorf("schedule: %d %s runs, want %d", got[label], label, count)
		}
	}
	return nil
}

// p09Pair reads one P09 unit: exactly one B0 and one S0 run in the order
// p09Order gives its index, so consecutive units are ABBA groups.
// appendBound reads P08's append result and compares the scan's read bytes
// with delta + declared anchors (requirements, P08).
func appendBound(dir string, readBytes int64) (map[string]int64, error) {
	raw, err := os.ReadFile(filepath.Join(dir, appendFile))
	if err != nil {
		return nil, fmt.Errorf("append result: %w", err)
	}
	var appended appendResult
	if err := json.Unmarshal(raw, &appended); err != nil {
		return nil, fmt.Errorf("append result: %w", err)
	}
	if appended.Files == 0 || appended.DeltaBytes <= 0 {
		return nil, errors.New("append result: nothing appended")
	}
	return map[string]int64{"files": int64(appended.Files), "delta_bytes": appended.DeltaBytes,
		"anchor_bytes": appended.AnchorBytes, "bound_bytes": appended.DeltaBytes + appended.AnchorBytes,
		"read_bytes": readBytes, "excess_bytes": readBytes - appended.DeltaBytes - appended.AnchorBytes}, nil
}

// p09Arm is one P09 arm's full-rebuild cost from its own measurement: S0's
// traces (worker lifetime, stage spans, ingest counters) or B0's original
// scan profile, plus the front process from cli-runs.
type p09Arm struct {
	WorkerCPUNS, FrontCPUNS, RSSBoundBytes, ReadBytes int64
	Stages                                            map[string]int64 // ns
}

func p09ArmCost(dir string, run cliRun) (p09Arm, error) {
	arm := p09Arm{FrontCPUNS: run.UserNS + run.SysNS, Stages: map[string]int64{}}
	if run.Label == "B0" {
		raw, err := os.ReadFile(filepath.Join(dir, "B0", b0ProfileFile))
		if err != nil {
			return p09Arm{}, fmt.Errorf("B0 profile: %w", err)
		}
		var profile struct {
			Type   string `json:"type"`
			Stages struct {
				DiscoveryMS     int64              `json:"discovery_ms"`
				UsageMS         int64              `json:"usage_ms"`
				SessionMS       int64              `json:"session_ms"`
				DerivedCacheMS  int64              `json:"derived_cache_ms"`
				TotalMS         int64              `json:"total_ms"`
				WorkerCPUTimeMS int64              `json:"worker_cpu_time_ms"`
				WorkerPeakRSS   int64              `json:"worker_peak_rss_bytes"`
				Ingestion       map[string]float64 `json:"ingestion"`
			} `json:"stages"`
		}
		found := false
		for _, line := range strings.Split(string(raw), "\n") {
			if json.Unmarshal([]byte(line), &profile) == nil && profile.Type == "scan_performance" {
				found = true
			}
		}
		read, counted := profile.Stages.Ingestion["read_bytes"]
		if !found || profile.Stages.WorkerPeakRSS <= 0 || !counted {
			return p09Arm{}, errors.New("B0 profile: no complete scan_performance record")
		}
		ms := int64(1e6)
		arm.WorkerCPUNS = profile.Stages.WorkerCPUTimeMS * ms
		// The front process waits for the worker's result: their lifetimes
		// overlap, so the sum of the two peaks bounds the aggregate (P07 method).
		arm.RSSBoundBytes = profile.Stages.WorkerPeakRSS + run.MaxRSS
		arm.ReadBytes = int64(read)
		arm.Stages["discovery"], arm.Stages["usage"], arm.Stages["session"], arm.Stages["derive"], arm.Stages["total"] =
			profile.Stages.DiscoveryMS*ms, profile.Stages.UsageMS*ms, profile.Stages.SessionMS*ms, profile.Stages.DerivedCacheMS*ms, profile.Stages.TotalMS*ms
		return arm, nil
	}
	files, err := readSample(filepath.Join(dir, "S0"))
	if err != nil {
		return p09Arm{}, fmt.Errorf("S0 traces: %w", err)
	}
	collectors, scanEnds := 0, 0
	for _, file := range files {
		for _, record := range file.Records {
			switch {
			case record.Stage == "resource.lifetime" && file.Role == "collector":
				collectors++
				arm.WorkerCPUNS += record.N["user_ns"] + record.N["sys_ns"]
			case record.Stage == "scan.end" && file.Role == "collector":
				// A missing counter is unknown, not a zero read: only a
				// recorded read_bytes (zero included) counts.
				read, counted := record.N["read_bytes"]
				if !counted {
					return p09Arm{}, errors.New("S0 traces: scan.end without read_bytes")
				}
				scanEnds++
				arm.ReadBytes += read
			case strings.HasPrefix(record.Stage, "scan.") && record.Dur != nil:
				arm.Stages[strings.TrimPrefix(record.Stage, "scan.")] += *record.Dur
			}
		}
	}
	if collectors == 0 || scanEnds == 0 || arm.Stages["usage"] == 0 || arm.Stages["session"] == 0 || arm.Stages["derive"] == 0 {
		return p09Arm{}, errors.New("S0 traces: no complete collector rebuild")
	}
	arm.RSSBoundBytes = peakRSSUpperBound(files)
	return arm, nil
}

// p09ArmSummary reports an arm's worker and total CPU, RSS upper bound, read
// bytes and stage durations as distributions over its runs.
func p09ArmSummary(list []p09Arm) map[string]any {
	series := map[string][]interval{}
	var reads []int64
	for _, arm := range list {
		series["worker_cpu"] = append(series["worker_cpu"], interval{Value: arm.WorkerCPUNS})
		series["total_cpu"] = append(series["total_cpu"], interval{Value: arm.WorkerCPUNS + arm.FrontCPUNS})
		for stage, ns := range arm.Stages {
			series["stage_"+stage] = append(series["stage_"+stage], interval{Value: ns})
		}
		reads = append(reads, arm.ReadBytes)
	}
	out := map[string]any{"runs": len(list), "read_bytes": reads}
	for key, values := range series {
		out[key] = summarize(values)
	}
	var rss []int64
	for _, arm := range list {
		rss = append(rss, arm.RSSBoundBytes)
	}
	out["rss_upper_bound_bytes"] = rss
	return out
}

func p09Pair(dir string, index int) ([]cliRun, error) {
	runs, err := readCLIRuns(dir)
	if err != nil {
		return nil, err
	}
	order := p09Order(index)
	if len(runs) != len(order) {
		return nil, fmt.Errorf("schedule: %d runs, want one B0/S0 pair", len(runs))
	}
	for i, run := range runs {
		if run.Label != order[i] {
			return nil, fmt.Errorf("schedule: run %d is %s, want %s", i+1, run.Label, order[i])
		}
	}
	return runs, nil
}

// preconditionFailure names the product failure that kept a P06 run from
// opening its window, or "" when the window could open.
func preconditionFailure(files []traceFile) string {
	for _, file := range files {
		if file.Role != "app" {
			continue
		}
		for _, record := range file.Records {
			if record.Stage == "driver.precondition_timeout" {
				if record.N["helper_alive"] == 1 {
					return "helper alive 7 min after model.usable"
				}
				return "not usable within 30 s"
			}
		}
	}
	return ""
}

func hasStage(records []traceRecord, stage string) bool {
	for _, record := range records {
		if record.Stage == stage {
			return true
		}
	}
	return false
}

// backgroundMarkers counts helper, scan and OTel work between the first
// selection intent and the last frame, which P03 requires to be zero.
func backgroundMarkers(files []traceFile) int {
	from, to := int64(math.MaxInt64), int64(0)
	for _, file := range files {
		for _, record := range file.Records {
			if file.Role == "app" && record.Stage == "select.intent" {
				from = min(from, record.T)
			}
			if file.Role == "app" && record.Stage == "frame.vsync" {
				to = max(to, record.T)
			}
		}
	}
	count := 0
	for _, file := range files {
		for _, record := range file.Records {
			busy := record.Stage == "helper.start" || record.Stage == "job.start" || strings.HasPrefix(record.Stage, "otel.")
			if busy && record.T >= from && record.T <= to {
				count++
			}
		}
	}
	return count
}

// routingPairs reads a P01-routing App trace. An event click is input.event
// followed by its input.action (the routing segment) or by input.unrouted;
// an action click is an input.action with no pending event. Each click's
// latency runs from its own start to the next frame.vsync.
func routingPairs(records []traceRecord) (routing, eventClicks, actionClicks []interval, unrouted int) {
	var event *traceRecord
	for i := range records {
		record := records[i]
		switch record.Stage {
		case "input.event":
			event = &records[i]
		case "input.unrouted":
			event = nil
			unrouted++
		case "input.action":
			start := record
			clicks := &actionClicks
			if event != nil {
				routing = append(routing, interval{Value: record.T - event.T})
				start, clicks, event = *event, &eventClicks, nil
			}
			value := interval{Failed: true}
			for _, later := range records[i+1:] {
				if later.Stage == "frame.vsync" {
					value = interval{Value: later.T - start.T, Failed: later.T-start.T > 5_000_000_000}
					break
				}
				if later.Stage == "input.event" || later.Stage == "input.action" {
					break
				}
			}
			*clicks = append(*clicks, value)
		}
	}
	return routing, eventClicks, actionClicks, unrouted
}

// writeToFrame measures P10: write n (source.written seq n) ends at the first
// App frame.vsync after it whose hero equals the oracle's expectation after n
// writes. Frames still showing an earlier expected state are waited through;
// any other content, or no match within 30 s, is a product failure.
func writeToFrame(files []traceFile, expected oracle) ([]interval, error) {
	var writes []traceRecord
	var frames []traceRecord
	for _, file := range files {
		for _, record := range file.Records {
			switch {
			case record.Stage == "source.written":
				writes = append(writes, record)
			case record.Stage == "frame.vsync" && file.Role == "app":
				frames = append(frames, record)
			}
		}
	}
	sort.Slice(writes, func(i, j int) bool { return writes[i].T < writes[j].T })
	sort.Slice(frames, func(i, j int) bool { return frames[i].T < frames[j].T })
	base, ok := expected.expect(clientAll, periodToday)
	if !ok {
		return nil, errors.New("oracle has no all/today hero")
	}
	states := append([]heroSummary{base}, expected.Writes...)
	var out []interval
	for i, written := range writes {
		n := int(written.N["seq"])
		if n != i+1 || n >= len(states) {
			return nil, fmt.Errorf("oracle: write %d has sequence %d of %d expected writes", i+1, n, len(expected.Writes))
		}
		value := interval{Failed: true, StartT: written.T}
	frames:
		for _, frame := range frames {
			if frame.T < written.T {
				continue
			}
			if frame.T-written.T > 30_000_000_000 {
				break
			}
			hero := heroFromCounts(frame.N)
			if hero == states[n] {
				value = interval{Value: frame.T - written.T, StartT: written.T, EndT: frame.T, End: frame.N}
				break
			}
			for _, earlier := range states[:n] {
				if hero == earlier {
					continue frames
				}
			}
			break // wrong content: neither this write's state nor an earlier one
		}
		out = append(out, value)
	}
	return out, nil
}

// idleWindowStats applies the P06 window definition to one sample.
func idleWindowStats(dir string, files []traceFile) (map[string]any, error) {
	var begin, end *traceRecord
	activations := 0
	var appFile traceFile
	for _, file := range files {
		if file.Role == "app" {
			appFile = file
		}
	}
	for i := range appFile.Records {
		record := &appFile.Records[i]
		switch record.Stage {
		case "idle.window.begin":
			begin = record
		case "idle.window.end":
			end = record
		}
	}
	if begin == nil || end == nil {
		return nil, errors.New("idle window markers missing")
	}
	window := end.T - begin.T
	cpu := (end.N["user_ns"] + end.N["sys_ns"]) - (begin.N["user_ns"] + begin.N["sys_ns"])
	for _, file := range files {
		for _, record := range file.Records {
			if record.T < begin.T || record.T > end.T {
				continue
			}
			if record.Stage == "job.enqueue" || record.Stage == "helper.start" {
				activations++
			}
		}
		if file.Role != "app" && file.Start >= begin.T && file.Start <= end.T {
			for _, record := range file.Records {
				if record.Stage == "resource.lifetime" {
					cpu += record.N["user_ns"] + record.N["sys_ns"]
				}
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "rss.json"))
	if err != nil {
		return nil, fmt.Errorf("external RSS samples: %w", err)
	}
	var samples []rssSample
	if err := json.Unmarshal(raw, &samples); err != nil {
		return nil, err
	}
	var totals []int64
	var appOnly *rssSample
	for i := range samples {
		sample := samples[i]
		if sample.AppOnly {
			appOnly = &samples[i]
			continue
		}
		if sample.T >= begin.T && sample.T <= end.T {
			totals = append(totals, sample.TotalKB)
		}
	}
	if appOnly == nil {
		return nil, errors.New("no App-only ps reading at the window end")
	}
	if len(totals) > 10 {
		totals = totals[len(totals)-10:]
	}
	if len(totals) == 0 {
		return nil, errors.New("no RSS samples inside the window")
	}
	sort.Slice(totals, func(i, j int) bool { return totals[i] < totals[j] })
	stable := totals[(len(totals)+1)/2-1]
	resident := end.N["resident_bytes"] / 1024
	difference := math.Abs(float64(resident-appOnly.AppKB)) / float64(max(appOnly.AppKB, 1))
	if difference > 0.10 {
		return nil, fmt.Errorf("App RSS cross-check differs by %.1f%%", difference*100)
	}
	return map[string]any{
		"window_s": float64(window) / 1e9, "cpu_core_fraction": float64(cpu) / float64(window),
		"stable_rss_kb": stable, "activations": activations,
		"os_wakeups": end.N["wakeups"] - begin.N["wakeups"], "rss_cross_check_diff": difference,
	}, nil
}

func runStatsCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || !strings.HasPrefix(args[1], "P") {
		fmt.Fprintln(stderr, "usage: agentdeck-perf stats <batch-dir> <condition>")
		return 2
	}
	if err := runStats(args[0], args[1], stdout); err != nil {
		fmt.Fprintln(stderr, "agentdeck-perf stats:", err)
		return 1
	}
	return 0
}
