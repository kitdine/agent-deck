package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kitdine/agent-deck/internal/perftrace"
)

// runConfig names the inputs of one condition run against a prepared batch.
type runConfig struct {
	StopOnFailure     bool   // stop after a completed failing unit, preserving its evidence
	Condition         string // selected condition, propagated to both arms
	App               string // Perf build AgentDeck.app
	Agentdeck         string // S0 CLI (the same build's helper)
	B0                string // baseline CLI, P09 only
	P09BaselineTraced bool   // S0 baseline uses current traces instead of the legacy B0 profile
	Perf              string // this tool, for appends
	Seed              uint64
	Out               string
	TZ                string
	Batch             *batchState
	// Baseline arm for a relative gate: its own build and its own batch,
	// because the template depends on the binary that built it.
	Baseline *runConfig
}

// schedule is a condition's run-unit structure (performance evaluation,
// samples and statistics): Units run units, each with Inner samples inside
// it (repeated clicks, selections per dimension, writes or increments) and,
// for P06, a Window in seconds. Pilot and formal schedules are separate and
// fixed; a formal batch is never assembled from pilot runs.
type schedule struct {
	Mode   string `json:"mode"`
	Units  int    `json:"units"`
	Inner  int    `json:"inner,omitempty"`
	Window int    `json:"window_s,omitempty"`
}

const scheduleFile = "schedule.json"

func readSchedule(root string) (schedule, error) {
	raw, err := os.ReadFile(filepath.Join(root, scheduleFile))
	if err != nil {
		return schedule{}, fmt.Errorf("schedule: %w", err)
	}
	var s schedule
	if err := json.Unmarshal(raw, &s); err != nil {
		return schedule{}, fmt.Errorf("schedule: %w", err)
	}
	return s, nil
}

// conditionPlan says how one condition runs.
type conditionPlan struct {
	pilot, formal schedule // formal.Units == 0: the condition has no formal batch
	writes        bool     // appends to source files: regenerate before, dirty after
	// oracle: every sample's data frames are checked against the expected
	// heroes of an independent full scan, computed once per arm. bootstrap
	// adds the heroes of each publication of an independent bootstrap from
	// empty state, for a first frame that shows a partial first batch.
	oracle, bootstrap bool
	run               func(cfg runConfig, dir string, index int, s schedule) error
}

func runBatchCommand(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var cfg runConfig
	condition := flags.String("condition", "", "condition id, e.g. P01-first")
	formal := flags.Bool("formal", false, "run the formal schedule instead of the tool pilot")
	batchRoot := flags.String("batch", "", "prepared batch directory (agentdeck-perf batch prepare)")
	flags.StringVar(&cfg.App, "app", "", "Perf AgentDeck.app")
	flags.StringVar(&cfg.Agentdeck, "agentdeck", "", "agentdeck CLI under test (S0)")
	flags.StringVar(&cfg.B0, "b0", "", "baseline agentdeck CLI (P09)")
	flags.BoolVar(&cfg.P09BaselineTraced, "p09-baseline-traced", false, "P09 baseline is S0 with perftrace support, not legacy B0")
	flags.BoolVar(&cfg.StopOnFailure, "stop-on-failure", false, "stop after the first failed run unit; preserve its records")
	flags.Uint64Var(&cfg.Seed, "seed", 1, "append seed")
	baselineApp := flags.String("baseline-app", "", "baseline arm Perf AgentDeck.app (ABBA relative run)")
	baselineAgentdeck := flags.String("baseline-agentdeck", "", "baseline arm agentdeck CLI")
	baselineBatch := flags.String("baseline-batch", "", "baseline arm prepared batch")
	flags.StringVar(&cfg.Out, "out", "", "empty directory for this condition's samples")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	batch, err := loadBatch(*batchRoot)
	if err == nil {
		cfg.Batch, cfg.TZ = batch, batch.TZ
		cfg.Perf, err = os.Executable()
	}
	if err == nil && *baselineApp != "" {
		var base *batchState
		if base, err = loadBatch(*baselineBatch); err == nil {
			if base.WorkingCopy.Identity != batch.WorkingCopy.Identity {
				err = errors.New("baseline and candidate batches must use the same working copy identity")
			}
			arm := cfg
			arm.App, arm.Agentdeck, arm.Batch, arm.Out = *baselineApp, *baselineAgentdeck, base, filepath.Join(cfg.Out, "baseline")
			cfg.Baseline = &arm
		}
	}
	if err == nil {
		err = runBatch(cfg, *condition, *formal, stdout)
	}
	return report(stderr, "run", err)
}

func runBatch(cfg runConfig, condition string, formal bool, stdout io.Writer) error {
	cfg.Condition = condition
	if cfg.Baseline != nil {
		cfg.Baseline.Condition = condition
	}
	plan, ok := conditionPlans()[condition]
	if !ok {
		return fmt.Errorf("unknown condition %q", condition)
	}
	s := plan.pilot
	s.Mode = "pilot"
	if formal {
		if plan.formal.Units == 0 {
			return fmt.Errorf("%s has no formal batch", condition)
		}
		s = plan.formal
		s.Mode = "formal"
	}
	if err := requireEmptyDir(cfg.Out); err != nil {
		return err
	}
	environment, err := batchPreflight(cfg)
	if err != nil {
		return err
	}
	// Keep the machine awake for the whole run; caffeinate exits with us.
	caffeinate := exec.Command("/usr/bin/caffeinate", "-dimsu", "-w", strconv.Itoa(os.Getpid()))
	if err := caffeinate.Start(); err != nil {
		return err
	}
	defer caffeinate.Process.Kill()
	if err := writeJSON(filepath.Join(cfg.Out, "environment.json"), environment); err != nil {
		return err
	}
	// Writing conditions start from a fresh working copy and leave it dirty;
	// read-only conditions never see another condition's appends.
	if plan.writes {
		cfg.Batch.Dirty = true
	}
	if err := cfg.Batch.ensureClean(); err != nil {
		return err
	}
	if cfg.Baseline != nil {
		return runArms(cfg, plan, condition, s, stdout)
	}
	if err := writeJSON(filepath.Join(cfg.Out, scheduleFile), s); err != nil {
		return err
	}
	if err := writeArmIdentity(cfg); err != nil {
		return err
	}
	expected := armOracle(cfg, plan)
	for index := 1; index <= s.Units; index++ {
		if err := runUnit(cfg, plan, index, s, expected); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s %d/%d done\n", condition, index, s.Units)
	}
	if plan.writes {
		cfg.Batch.Dirty = true
		return cfg.Batch.save()
	}
	return nil
}

// armOracle computes an arm's read-only oracle once, on first use, outside
// every run unit; nil when the condition checks no data frame.
func armOracle(cfg runConfig, plan conditionPlan) func() (oracle, error) {
	if !plan.oracle {
		return nil
	}
	var result *oracle
	var failure error
	return func() (oracle, error) {
		if result == nil && failure == nil {
			heroes, err := computeOracle(cfg.Agentdeck, cfg.Batch.Home, cfg.TZ, cfg.Batch.Anchor, cfg.Batch.Root)
			result, failure = &oracle{Heroes: heroes}, err
			if err == nil && plan.bootstrap {
				result.Batches, failure = computeBootstrapOracle(cfg.Agentdeck, cfg.Batch.Home, cfg.TZ, cfg.Batch.Anchor,
					cfg.Batch.Root, heroes)
			}
		}
		return *result, failure
	}
}

// maxUnitAttempts bounds how often one run unit is rerun after an outside
// interruption before it is recorded tool-invalid.
const maxUnitAttempts = 5

// interruptedDir holds the attempts of run units that were interrupted from
// outside; stats reports them and never reads them as samples.
const interruptedDir = "interrupted"

// runUnit runs one run unit into <out>/sample-NNNN. An attempt interrupted
// from outside the product — another app took the front from the shown
// popover, or the screen was locked — measured nothing about the product: it
// is kept under <out>/interrupted and the same unit runs again (2026-10-25
// user decision). Environment, oracle and run errors make the unit
// tool-invalid; only I/O on the batch is returned.
func runUnit(cfg runConfig, plan conditionPlan, index int, s schedule, expected func() (oracle, error)) error {
	dir := filepath.Join(cfg.Out, fmt.Sprintf("sample-%04d", index))
	for attempt := 1; ; attempt++ {
		if err := runUnitOnce(cfg, plan, index, s, expected, dir); err != nil {
			return err
		}
		reason := unitInterruption(dir)
		if reason == "" {
			if cfg.StopOnFailure {
				return checkUnitOutcome(dir, cfg.Condition, s)
			}
			return nil
		}
		if attempt == maxUnitAttempts {
			if err := markInvalid(dir, fmt.Sprintf("interrupted %d times, last: %s", attempt, reason)); err != nil {
				return err
			}
			if cfg.StopOnFailure {
				return fmt.Errorf("%s: interrupted %d times; records retained", dir, attempt)
			}
			return nil
		}
		kept := filepath.Join(cfg.Out, interruptedDir)
		if err := os.MkdirAll(kept, 0o700); err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(dir, "interrupted.json"), map[string]string{"reason": reason}); err != nil {
			return err
		}
		if err := os.Rename(dir, filepath.Join(kept, fmt.Sprintf("sample-%04d-attempt-%d", index, attempt))); err != nil {
			return err
		}
		waitUnlocked()
		if plan.writes {
			cfg.Batch.Dirty = true
		}
	}
}

// Check only the completed unit, never treat the remaining schedule as missing
// samples. External interruptions have already been handled by runUnit.
func checkUnitOutcome(dir, condition string, s schedule) error {
	if raw, err := os.ReadFile(filepath.Join(dir, "invalid.json")); err == nil {
		return fmt.Errorf("%s: invalid unit: %s", dir, raw)
	}
	if condition == "P08" {
		for _, scenario := range []string{"noop", "append"} {
			if _, err := readSample(filepath.Join(dir, scenario)); err != nil {
				return fmt.Errorf("%s: %s: %w", dir, scenario, err)
			}
		}
		runs, err := readCLIRuns(dir)
		if err != nil {
			return err
		}
		if len(runs) == 0 {
			return fmt.Errorf("%s: no CLI runs", dir)
		}
		for _, run := range runs {
			if run.ExitCode != 0 {
				return fmt.Errorf("%s: product failure: %s exit %d", dir, run.Label, run.ExitCode)
			}
		}
		return nil
	}
	if condition == "P09" {
		var index int
		if _, err := fmt.Sscanf(filepath.Base(dir), "sample-%d", &index); err != nil {
			return err
		}
		runs, err := p09Pair(dir, index)
		if err != nil {
			return err
		}
		for _, run := range runs {
			if run.ExitCode != 0 {
				return fmt.Errorf("%s: product failure: %s exit %d", dir, run.Label, run.ExitCode)
			}
			if _, err := p09ArmCost(dir, run); err != nil {
				return err
			}
		}
		return nil
	}
	files, err := readSample(dir)
	if err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	if failure := preconditionFailure(files); failure != "" {
		return fmt.Errorf("%s: product failure: %s (records retained)", dir, failure)
	}
	if conditionIntervals[condition].end != "" || condition == "P10" {
		values, err := sampleIntervals(dir, condition, files, s)
		if err != nil {
			return fmt.Errorf("%s: %w", dir, err)
		}
		for _, value := range values {
			if value.Failed {
				return fmt.Errorf("%s: product failure (records retained)", dir)
			}
		}
	}
	if condition == "P03" && backgroundMarkers(files) != 0 {
		return fmt.Errorf("%s: background work during selection (records retained)", dir)
	}
	if condition == "P06" {
		_, err = idleWindowStats(dir, files)
	}
	return err
}

// unitInterruption names an outside interruption of the attempt in dir, or "".
func unitInterruption(dir string) string {
	if raw, err := os.ReadFile(filepath.Join(dir, "invalid.json")); err == nil {
		var invalid struct {
			Reason string `json:"reason"`
		}
		if json.Unmarshal(raw, &invalid) == nil && invalid.Reason == screenLockedReason {
			return screenLockedReason
		}
		return ""
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "app-*.jsonl"))
	if len(paths) == 0 {
		// CLI conditions have no window a locked screen or another app can take.
		return ""
	}
	if screenLocked() {
		return screenLockedReason
	}
	for _, path := range paths {
		file, err := readTraceFile(path)
		if err != nil {
			return ""
		}
		for _, record := range file.Records {
			if record.Stage == "driver.interrupted" {
				return "another app took the front from the shown popover"
			}
		}
	}
	return ""
}

const screenLockedReason = "screen locked"

// waitUnlocked waits until the console session is unlocked.
func waitUnlocked() {
	for screenLocked() {
		time.Sleep(5 * time.Second)
	}
}

func runUnitOnce(cfg runConfig, plan conditionPlan, index int, s schedule, expected func() (oracle, error), dir string) error {
	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	waitUnlocked()
	if reason := sampleEnvironment(cfg, dir); reason != "" {
		return markInvalid(dir, reason)
	}
	if expected != nil {
		o, err := expected()
		if err != nil {
			return markInvalid(dir, "oracle: "+err.Error())
		}
		if err := writeOracle(dir, o); err != nil {
			return err
		}
	}
	cfg.Batch.UnitDir = dir
	err := plan.run(cfg, dir, index, s)
	cfg.Batch.UnitDir = ""
	if err != nil {
		// An interrupted attempt is rerun, not recorded invalid.
		if unitInterruption(dir) != "" {
			return nil
		}
		return markInvalid(dir, err.Error())
	}
	return nil
}

// runArms interleaves baseline (A) and candidate (B) run units by pair:
// odd pairs run A B, even pairs B A, so consecutive pairs form the A B B A
// groups the gate resamples; the gate accepts only formal schedules with an
// even number of units. Arms write <out>/baseline and <out>/candidate.
func runArms(cfg runConfig, plan conditionPlan, condition string, s schedule, stdout io.Writer) error {
	candidate := cfg
	candidate.Baseline, candidate.Out = nil, filepath.Join(cfg.Out, "candidate")
	baseline := *cfg.Baseline
	for _, arm := range []runConfig{baseline, candidate} {
		if err := os.Mkdir(arm.Out, 0o700); err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(arm.Out, scheduleFile), s); err != nil {
			return err
		}
		if err := writeArmIdentity(arm); err != nil {
			return err
		}
		if err := arm.Batch.ensureClean(); err != nil {
			return err
		}
	}
	oracles := map[*batchState]func() (oracle, error){
		baseline.Batch: armOracle(baseline, plan), candidate.Batch: armOracle(candidate, plan),
	}
	for index := 1; index <= s.Units; index++ {
		order := []runConfig{baseline, candidate}
		if index%2 == 0 {
			order = []runConfig{candidate, baseline}
		}
		for _, arm := range order {
			if plan.writes {
				arm.Batch.Dirty = true
				if err := arm.Batch.ensureClean(); err != nil {
					return err
				}
			}
			if err := runUnit(arm, plan, index, s, oracles[arm.Batch]); err != nil {
				return err
			}
			if plan.writes {
				arm.Batch.Dirty = true
				if err := arm.Batch.save(); err != nil {
					return err
				}
			}
		}
		fmt.Fprintf(stdout, "%s pairs %d/%d done\n", condition, index, s.Units)
	}
	return nil
}

// markInvalid records why a sample is tool-invalid; stats reads it.
func markInvalid(dir, reason string) error {
	return writeJSON(filepath.Join(dir, "invalid.json"), map[string]string{"reason": reason})
}

func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

// armIdentityFile records what stays fixed for one arm: the executables it
// ran and the corpus, anchor and working copy identity (performance
// evaluation, records and evidence). The template a run unit restored can
// change between units, so each attempt records it in unitIdentityFile.
// Source digests are bound by the evidence, not by this tool.
const armIdentityFile = "identity.json"

type armIdentity struct {
	P09BaselineTraced bool              `json:"p09_baseline_traced,omitempty"`
	Tool              string            `json:"perf_tool_sha256"`
	Binaries          map[string]string `json:"binaries_sha256"`
	AnchorDate        string            `json:"anchor_date"`
	Corpus            string            `json:"corpus_identity"`
	WorkingCopy       string            `json:"working_copy_identity"`
	ShiftDays         int               `json:"shift_days"`
	Unparsed          int               `json:"timestamps_unparsed"`
}

// appBinaries are the measured App's executables, relative to the bundle.
var appBinaries = []string{
	"Contents/MacOS/AgentDeck",
	"Contents/Helpers/agentdeck",
	"Contents/Frameworks/AgentDeckShared.framework/Versions/A/AgentDeckShared",
}

func writeArmIdentity(cfg runConfig) error {
	identity, err := identifyArm(cfg)
	if err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Out, armIdentityFile), identity)
}

func identifyArm(cfg runConfig) (armIdentity, error) {
	b := cfg.Batch
	identity := armIdentity{Binaries: map[string]string{}, AnchorDate: b.Anchor, Corpus: b.WorkingCopy.Corpus,
		WorkingCopy: b.WorkingCopy.Identity, ShiftDays: b.WorkingCopy.Days, Unparsed: b.WorkingCopy.Unparsed}
	identity.P09BaselineTraced = cfg.P09BaselineTraced
	var err error
	if cfg.Perf != "" {
		if identity.Tool, err = fileSHA256(cfg.Perf); err != nil {
			return armIdentity{}, err
		}
	}
	paths := map[string]string{"agentdeck": cfg.Agentdeck}
	if cfg.B0 != "" {
		paths["p09_baseline"] = cfg.B0
	}
	if cfg.App != "" {
		for _, name := range appBinaries {
			paths[name] = filepath.Join(cfg.App, name)
		}
	}
	for name, path := range paths {
		if identity.Binaries[name], err = fileSHA256(path); err != nil {
			return armIdentity{}, err
		}
	}
	return identity, nil
}

// batchEnvironment is recorded with every condition run.
type batchEnvironment struct {
	CPU           string           `json:"cpu"`
	MemBytes      string           `json:"mem_bytes"`
	OS            string           `json:"os"`
	Timezone      string           `json:"timezone"`
	AnchorDate    string           `json:"anchor_date"`
	Corpus        string           `json:"corpus_identity"`
	WorkingCopy   string           `json:"working_copy_identity"`
	ShiftDays     int              `json:"shift_days"`
	FallbackFiles int              `json:"utc_fallback_files"`
	Unparsed      int              `json:"timestamps_unparsed"`
	Template      templateManifest `json:"template"`
	Generations   int              `json:"working_copy_generations"`
	StartedAt     int64            `json:"started_uptime_ns"`
}

func output(name string, args ...string) string {
	raw, _ := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(raw))
}

func batchPreflight(cfg runConfig) (batchEnvironment, error) {
	for _, path := range []string{cfg.App, cfg.Agentdeck, cfg.Out} {
		if !filepath.IsAbs(path) {
			return batchEnvironment{}, errors.New("--app, --agentdeck and --out must be absolute")
		}
	}
	b := cfg.Batch
	if err := batchWindowOpen(b, time.Now()); err != nil {
		return batchEnvironment{}, err
	}
	env := batchEnvironment{
		CPU: output("/usr/sbin/sysctl", "-n", "machdep.cpu.brand_string"), MemBytes: output("/usr/sbin/sysctl", "-n", "hw.memsize"),
		OS: output("/usr/bin/sw_vers", "-productVersion"), Timezone: b.TZ, AnchorDate: b.Anchor,
		Corpus: b.WorkingCopy.Corpus, WorkingCopy: b.WorkingCopy.Identity, ShiftDays: b.WorkingCopy.Days,
		FallbackFiles: b.WorkingCopy.FallbackFiles, Unparsed: b.WorkingCopy.Unparsed, Template: b.Template,
		Generations: b.Generations, StartedAt: perftrace.Now(),
	}
	if others := otherAgentDeckProcesses(cfg.App); len(others) > 0 {
		return env, fmt.Errorf("another AgentDeck instance is running (pid %s); quit it first", strings.Join(others, ","))
	}
	return env, nil
}

// otherAgentDeckProcesses lists AgentDeck App processes not started from the
// measured bundle. The batch never stops them; it refuses to start.
func otherAgentDeckProcesses(app string) []string {
	var out []string
	for _, line := range strings.Split(output("/bin/ps", "-Ao", "pid=,comm="), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		command := strings.Join(fields[1:], " ")
		if strings.HasSuffix(command, "AgentDeck.app/Contents/MacOS/AgentDeck") && !strings.HasPrefix(command, app+"/") {
			out = append(out, fields[0])
		}
	}
	return out
}

// sampleEnvironment records the machine state for one run unit. Load is
// recorded, never a reason to invalidate: ABBA interleaving cancels it. Only a
// locked screen or another AgentDeck instance invalidate the run unit; the
// power source and CPU speed limit are not checked (2026-10-25 user decision).
func sampleEnvironment(cfg runConfig, dir string) string {
	environment := map[string]any{
		"load_1m":           loadAverage(),
		"started_uptime_ns": perftrace.Now(),
		"screen_locked":     screenLocked(),
	}
	_ = writeJSON(filepath.Join(dir, "environment.json"), environment)
	switch {
	case environment["screen_locked"].(bool):
		return screenLockedReason
	case len(otherAgentDeckProcesses(cfg.App)) > 0:
		return "another AgentDeck instance is running"
	}
	return ""
}

// screenLocked reads the console session's lock state. The batch requires an
// unlocked screen: a locked one occludes every window, so no data frame can
// commit and a missing frame would read as a product failure.
func screenLocked() bool {
	return screenLockedIn(output("/usr/sbin/ioreg", "-n", "Root", "-d1"))
}

func screenLockedIn(registry string) bool {
	return strings.Contains(registry, `"CGSSessionScreenIsLocked"=Yes`)
}

func loadAverage() float64 {
	fields := strings.Fields(strings.Trim(output("/usr/sbin/sysctl", "-n", "vm.loadavg"), "{} "))
	if len(fields) == 0 {
		return 99
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 99
	}
	return value
}

// stateMode selects a run unit's starting state root.
type stateMode int

const (
	stateFull    stateMode = iota // template databases and serving cache
	stateNoCache                  // template databases, no serving/derived cache
	stateEmpty                    // no state: first install
)

// relaunchSettle separates one App run unit from the next. The system places
// a new status item only after the previous instance's item has gone; a launch
// that follows an exit immediately can leave the new item off screen, where no
// click can reach it.
const relaunchSettle = 2 * time.Second

var lastAppExit time.Time

// during is work alongside one App run (a sampler, writer or rebuild). It
// gets the run's timeout context and a channel closed when the App has
// exited; launchApp waits for it to return before the run unit ends.
type during func(ctx context.Context, exited <-chan struct{})

// launchApp starts the Perf App the way a user's launch does (open -n) on the
// batch HOME and waits for the driver to finish the script and quit.
func launchApp(cfg runConfig, home, traceDir string, steps []map[string]any, timeout time.Duration, alongside during) error {
	if wait := relaunchSettle - time.Since(lastAppExit); wait > 0 {
		time.Sleep(wait)
	}
	defer func() { lastAppExit = time.Now() }()
	script := filepath.Join(traceDir, "script.json")
	if err := writeJSON(script, map[string]any{"steps": steps}); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/bin/open", "-n", "-W", "-a", cfg.App,
		"--env", "AGENTDECK_TEST_HOME="+home, "--env", "HOME="+home, "--env", "CFFIXED_USER_HOME="+home,
		"--env", perftrace.EnvOut+"="+traceDir, "--env", "AGENTDECK_PERF_SCRIPT="+script, "--env", "TZ="+cfg.TZ)
	if err := command.Start(); err != nil {
		return err
	}
	exited, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		if alongside != nil {
			alongside(ctx, exited)
		}
	}()
	err := command.Wait()
	timedOut := ctx.Err() != nil
	close(exited)
	if timedOut {
		killMeasuredApp(cfg.App)
	}
	<-done
	if timedOut {
		return errors.New("app did not finish its script before the batch timeout")
	}
	// Scan workers the App started outlive it briefly; their traces are
	// complete, and the state root safe to reset, only once they exit.
	if waitErr := waitForWorkers(filepath.Join(home, ".agentdeck"), 60*time.Second); err == nil {
		err = waitErr
	}
	return err
}

// killMeasuredApp stops only processes started from the measured bundle.
func killMeasuredApp(app string) {
	for _, pid := range measuredAppPIDs(app) {
		if value, err := strconv.Atoi(pid); err == nil {
			_ = syscall.Kill(value, syscall.SIGKILL)
		}
	}
}

func measuredAppPIDs(app string) []string {
	var out []string
	for _, line := range strings.Split(output("/bin/ps", "-Ao", "pid=,comm="), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.Join(fields[1:], " ") == app+"/Contents/MacOS/AgentDeck" {
			out = append(out, fields[0])
		}
	}
	return out
}

// cliRun is one externally timed CLI process (P07/P08/P09).
type cliRun struct {
	CostSource string `json:"cost_source,omitempty"`
	Label      string `json:"label"`
	WallNS     int64  `json:"wall_ns"`
	UserNS     int64  `json:"user_ns"`
	SysNS      int64  `json:"sys_ns"`
	MaxRSS     int64  `json:"maxrss_bytes"`
	ExitCode   int    `json:"exit_code"`
	StartedNS  int64  `json:"started_ns"`
}

func runCLI(binary, home, stateDir, traceDir, tz, label string, args ...string) (cliRun, error) {
	return runCLIWith(binary, home, stateDir, traceDir, tz, label, nil, nil, args...)
}

// Nil in production. Package tests supply account injection only for their
// registered, acceptance-tagged CLI fixtures; the normal whitelist is unchanged.
var testCLIEnvironment func(string) []string

func addTestCLIEnvironment(command *exec.Cmd, binary string) {
	if testCLIEnvironment != nil {
		command.Env = append(command.Env, testCLIEnvironment(binary)...)
	}
}

// runCLIWith is runCLI with extra environment and the command's stderr, which
// P09's B0 arm needs for its original opt-in scan profile.
func runCLIWith(binary, home, stateDir, traceDir, tz, label string, env []string, stderr io.Writer, args ...string) (cliRun, error) {
	command := exec.Command(binary, append([]string{"--state-dir", stateDir}, args...)...)
	command.Env = append([]string{"HOME=" + home, "TZ=" + tz, "PATH=/usr/bin:/bin", "LANG=en_US_POSIX", "LC_ALL=en_US_POSIX"}, env...)
	addTestCLIEnvironment(command, binary)
	command.Stderr = stderr
	if traceDir != "" {
		command.Env = append(command.Env, perftrace.EnvOut+"="+traceDir)
	}
	started := perftrace.Now()
	err := command.Run()
	ended := perftrace.Now()
	run := cliRun{Label: label, WallNS: ended - started, StartedNS: started}
	if state := command.ProcessState; state != nil {
		usage := state.SysUsage().(*syscall.Rusage)
		run.UserNS, run.SysNS, run.MaxRSS, run.ExitCode = usage.Utime.Nano(), usage.Stime.Nano(), usage.Maxrss, state.ExitCode()
	}
	// The scan runs in a detached worker that exits after a short idle
	// period. Its trace is complete, and the state root safe to reset, only
	// after it has exited.
	if waitErr := waitForWorkers(stateDir, 60*time.Second); err == nil {
		err = waitErr
	}
	// The process listing can be blind to the worker (a sandbox may hide
	// other processes' arguments), so the trace itself is the authority:
	// see waitForTraces for its completion barrier.
	if traceDir != "" && err == nil {
		err = waitForTraces(traceDir, 60*time.Second)
	}
	if err != nil {
		return run, fmt.Errorf("%s: %w", label, err)
	}
	return run, nil
}

// waitForWorkers waits until no scan worker for stateDir is running. The
// runtime starts the worker with the symlink-resolved state root, which can
// differ from the App's spelling (/tmp versus /private/tmp); both are matched.
func waitForWorkers(stateDir string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for workerRunning(stateDir) {
		if time.Now().After(deadline) {
			return errors.New("scan worker did not exit after the run")
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// waitForTraces waits until every trace in dir is complete: its producer has
// released the trace lock, which happens only when Close has flushed every
// record and closed the file, and it holds a resource.lifetime record. The
// lifetime record alone is no barrier: Close writes it before its tail
// (trace.invalid), and a buffer boundary can expose it while the tail is
// still unwritten.
func waitForTraces(dir string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		pending, err := unflushedTrace(dir)
		if err != nil || pending == "" {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s: trace not flushed after the run", pending)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// unflushedTrace returns the first trace in dir that is not yet complete.
func unflushedTrace(dir string) (string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, path := range paths {
		released, err := perftrace.Released(path)
		if err != nil {
			return "", err
		}
		if !released {
			return filepath.Base(path), nil
		}
		file, err := readTraceFile(path)
		if err != nil {
			// An empty or partially flushed file is still being written.
			return filepath.Base(path), nil
		}
		if _, ok := file.end(); !ok {
			return filepath.Base(path), nil
		}
	}
	return "", nil
}

func workerRunning(stateDir string) bool {
	needles := []string{"--state-dir " + stateDir + " scan-worker"}
	if resolved, err := filepath.EvalSymlinks(stateDir); err == nil && resolved != stateDir {
		needles = append(needles, "--state-dir "+resolved+" scan-worker")
	}
	for _, line := range strings.Split(output("/bin/ps", "-Ao", "command="), "\n") {
		for _, needle := range needles {
			if strings.Contains(line, needle) {
				return true
			}
		}
	}
	return false
}

// helperAlive reports whether any process the measured App started is
// running: its child processes, or a detached scan worker on its state root.
func helperAlive(cfg runConfig) bool {
	for _, app := range measuredAppPIDs(cfg.App) {
		if output("/usr/bin/pgrep", "-P", app) != "" {
			return true
		}
	}
	return workerRunning(filepath.Join(cfg.Batch.Home, ".agentdeck"))
}

func appendCLIRun(dir string, run cliRun) error {
	path := filepath.Join(dir, "cli-runs.json")
	var runs []cliRun
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &runs); err != nil {
			return err
		}
	}
	return writeJSON(path, append(runs, run))
}

// rssSample is one external ps reading (P06).
type rssSample struct {
	T       int64 `json:"t"`
	AppKB   int64 `json:"app_kb"`
	TotalKB int64 `json:"total_kb"`
	AppOnly bool  `json:"app_only,omitempty"`
}

// sampleRSS reads App and live helper RSS every 60 s from outside the App,
// and takes one App-only reading as soon as the driver signals the window end.
// It also answers the driver's request to start the window: once no helper
// is alive, it creates idle-window-ready.
func sampleRSS(cfg runConfig, traceDir string) during {
	return func(ctx context.Context, exited <-chan struct{}) {
		var samples []rssSample
		save := func() { _ = writeJSON(filepath.Join(traceDir, "rss.json"), samples) }
		next := time.Now()
		signal := filepath.Join(traceDir, "idle-window-end")
		request, ready := filepath.Join(traceDir, "idle-window-request"), filepath.Join(traceDir, "idle-window-ready")
		answered := false
		for ctx.Err() == nil {
			select {
			case <-exited:
				save()
				return
			default:
			}
			if !answered {
				if _, err := os.Stat(request); err == nil && !helperAlive(cfg) {
					answered = os.WriteFile(ready, nil, 0o600) == nil
				}
			}
			if _, err := os.Stat(signal); err == nil {
				if app := measuredAppPIDs(cfg.App); len(app) == 1 {
					samples = append(samples, rssSample{T: perftrace.Now(), AppKB: rssKB(app[0]), AppOnly: true})
				}
				save()
				return
			}
			if time.Now().After(next) {
				if app := measuredAppPIDs(cfg.App); len(app) == 1 {
					sample := rssSample{T: perftrace.Now(), AppKB: rssKB(app[0])}
					sample.TotalKB = sample.AppKB
					for _, child := range strings.Fields(output("/usr/bin/pgrep", "-P", app[0])) {
						sample.TotalKB += rssKB(child)
					}
					samples = append(samples, sample)
					save()
				}
				next = next.Add(60 * time.Second)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func rssKB(pid string) int64 {
	value, _ := strconv.ParseInt(output("/bin/ps", "-o", "rss=", "-p", pid), 10, 64)
	return value
}

// click opens (and, for repeats, closes and reopens) the popover with the
// alternative stimulus adopted in the measurement-harness review: the
// button's action is sent directly, and P01 starts at input.action.
func click(timeoutMS int, repeat int) map[string]any {
	return map[string]any{"op": "click", "ms": timeoutMS, "repeat": repeat, "stimulus": "action"}
}

func step(op string, fields ...any) map[string]any {
	out := map[string]any{"op": op}
	for i := 0; i+1 < len(fields); i += 2 {
		out[fields[i].(string)] = fields[i+1]
	}
	return out
}

// P10 writes one record every writeInterval, the contract's minimum of 5 s
// plus margin, starting writeLead after launch, once the popover is open.
const (
	writeInterval = 6 * time.Second
	writeLead     = 15 * time.Second
)

// conditionPlans are the conditions' pilot and formal schedules (performance
// evaluation, tool pilot table and per-item formal samples). Each App launch,
// CLI process or write sequence is one run unit: it starts with a binding
// check and a state reset; samples inside it are not reset.
func conditionPlans() map[string]conditionPlan {
	app := func(mode stateMode, steps func(schedule) []map[string]any, timeout time.Duration) func(runConfig, string, int, schedule) error {
		return func(cfg runConfig, dir string, _ int, s schedule) error {
			if err := cfg.Batch.startRunUnit(mode); err != nil {
				return err
			}
			return launchApp(cfg, cfg.Batch.Home, dir, steps(s), timeout, nil)
		}
	}
	fixed := func(steps ...map[string]any) func(schedule) []map[string]any {
		return func(schedule) []map[string]any { return steps }
	}
	usable := step("wait_usable", "ms", 30000)
	settle := step("sleep", "ms", 2000)
	home := func(cfg runConfig) string { return cfg.Batch.Home }
	state := func(cfg runConfig) string { return filepath.Join(cfg.Batch.Home, ".agentdeck") }
	recordCLI := func(dir string, run cliRun, err error) error {
		if appendErr := appendCLIRun(dir, run); err == nil {
			err = appendErr
		}
		return err
	}
	return map[string]conditionPlan{
		"P01-first": {pilot: schedule{Units: 20}, formal: schedule{Units: 100}, oracle: true,
			run: app(stateFull, fixed(usable, settle, click(5000, 1)), 2*time.Minute)},
		// Inner open/close clicks per process after the first, which is skipped.
		"P01-repeat": {pilot: schedule{Units: 1, Inner: 20}, formal: schedule{Units: 10, Inner: 20}, oracle: true,
			run: app(stateFull, func(s schedule) []map[string]any {
				return []map[string]any{usable, settle, click(5000, s.Inner+1)}
			}, 5*time.Minute)},
		// A first and a repeated click while a full rebuild of the working copy
		// runs in a separate state root; stats counts only clicks inside it.
		"P01-rebuild": {pilot: schedule{Units: 20}, formal: schedule{Units: 100}, oracle: true,
			run: func(cfg runConfig, dir string, _ int, _ schedule) error {
				if err := cfg.Batch.startRunUnit(stateFull); err != nil {
					return err
				}
				var rebuild rebuildRecord
				err := launchApp(cfg, cfg.Batch.Home, dir, []map[string]any{usable, settle, step("wait_rebuild", "ms", 60000), click(5000, 2)},
					5*time.Minute, func(ctx context.Context, exited <-chan struct{}) {
						if err := waitRebuildRequest(ctx, dir, exited); err != nil {
							rebuild.Error = err.Error()
							return
						}
						rebuild = runRebuild(ctx, cfg, dir)
					})
				if writeErr := writeJSON(filepath.Join(dir, rebuildFile), rebuild); err == nil {
					err = writeErr
				}
				if err == nil && rebuild.Error != "" {
					err = fmt.Errorf("rebuild: %s", rebuild.Error)
				}
				return err
			}},
		// The excluded routing segment: Inner pairs of an event click (reported,
		// never invalidating, when unrouted) and an action click in one process.
		"P01-routing": {pilot: schedule{Units: 1, Inner: 20},
			run: app(stateFull, func(s schedule) []map[string]any {
				steps := []map[string]any{usable, settle}
				for range s.Inner {
					steps = append(steps, map[string]any{"op": "click", "ms": 5000, "repeat": 1, "stimulus": "event", "unrouted": "report"},
						click(5000, 1))
				}
				return steps
			}, 10*time.Minute)},
		"P02": {pilot: schedule{Units: 20}, formal: schedule{Units: 100},
			run: app(stateFull, fixed(usable, settle, click(5000, 1)), 2*time.Minute)},
		// Selections start once the publication is complete: P03 requires a
		// restored publication with no background work.
		// Formal: 4 run units of 25 per dimension, so the gate resamples whole
		// A B B A groups (design decision of 2026-10-24, MH-R1-F2).
		"P03": {pilot: schedule{Units: 1, Inner: 20}, formal: schedule{Units: 4, Inner: 25}, oracle: true,
			run: app(stateFull, func(s schedule) []map[string]any {
				return []map[string]any{usable, step("wait_complete", "ms", 600000), settle, click(5000, 1),
					step("select", "dimension", "client", "values", []string{"codex", "claude", "all"}, "count", s.Inner, "ms", 5000),
					step("select", "dimension", "period", "values", []string{"7d", "30d", "today"}, "count", s.Inner, "ms", 5000),
					// The isolated App has quota reading off, so the quota tab is hidden
					// and usage is presented; rotate through panels that render.
					step("select", "dimension", "panel", "values", []string{"breakdown", "attribution", "sessions", "usage"}, "count", s.Inner, "ms", 5000),
				}
			}, 30*time.Minute)},
		// Launch conditions start at app.ready, not at a click; they open the
		// popover with the alternative stimulus as soon as the item is on screen.
		"P04": {pilot: schedule{Units: 20}, formal: schedule{Units: 100}, oracle: true,
			run: app(stateNoCache, fixed(step("open_action", "ms", 30000)), 3*time.Minute)},
		// P11's first frame shows a partial first batch (performance evaluation,
		// P11 row): it is checked against the bootstrap publication with its
		// coverage, including the partial marker, not against the full hero.
		"P11-first": {pilot: schedule{Units: 20}, formal: schedule{Units: 100}, oracle: true, bootstrap: true,
			run: app(stateEmpty, fixed(step("open_action", "ms", 60000)), 5*time.Minute)},
		"P11-full": {pilot: schedule{Units: 1}, formal: schedule{Units: 5},
			run: app(stateEmpty, fixed(step("open_action", "ms", 60000), step("wait_complete", "ms", 3600000)), 70*time.Minute)},
		// The window opens 120 s after model.usable once no helper is alive; a
		// helper alive 7 min after model.usable is a product failure.
		"P06": {pilot: schedule{Units: 1, Window: 300}, formal: schedule{Units: 3, Window: 1800},
			run: func(cfg runConfig, dir string, _ int, s schedule) error {
				if err := cfg.Batch.startRunUnit(stateFull); err != nil {
					return err
				}
				steps := []map[string]any{usable, step("idle_window", "seconds", s.Window,
					"after_usable_ms", 120000, "helper_deadline_ms", 420000)}
				timeout := 7*time.Minute + time.Duration(s.Window)*time.Second + 3*time.Minute
				return launchApp(cfg, cfg.Batch.Home, dir, steps, timeout, sampleRSS(cfg, dir))
			}},
		// Each unit is a full rebuild from empty state; the last unit then runs
		// the Inner incremental appends and scans without any reset between.
		"P07": {pilot: schedule{Units: 1, Inner: 20}, formal: schedule{Units: 3, Inner: 100}, writes: true,
			run: func(cfg runConfig, dir string, index int, s schedule) error {
				if err := cfg.Batch.startRunUnit(stateEmpty); err != nil {
					return err
				}
				run, err := runCLI(cfg.Agentdeck, home(cfg), state(cfg), dir, cfg.TZ, "full", "scan")
				err = recordCLI(dir, run, err)
				for n := 1; err == nil && index == s.Units && n <= s.Inner; n++ {
					if err = runAppendCLI(cfg, dir, n, false); err == nil {
						run, err = runCLI(cfg.Agentdeck, home(cfg), state(cfg), dir, cfg.TZ, "incremental", "scan")
						err = recordCLI(dir, run, err)
					}
				}
				return err
			}},
		// noop runs on the clean copy; append regenerates the copy first, so
		// every run starts from the same sources.
		"P08": {pilot: schedule{Units: 2}, formal: schedule{Units: 3}, writes: true,
			run: func(cfg runConfig, dir string, _ int, _ schedule) error {
				for _, scenario := range []string{"noop", "append"} {
					traceDir := filepath.Join(dir, scenario)
					if err := os.Mkdir(traceDir, 0o700); err != nil {
						return err
					}
					if err := os.Chmod(traceDir, 0o700); err != nil {
						return err
					}
					if scenario == "append" {
						cfg.Batch.Dirty = true
					}
					if err := cfg.Batch.ensureClean(); err != nil {
						return err
					}
					if err := cfg.Batch.startRunUnit(stateNoCache); err != nil {
						return err
					}
					if scenario == "append" {
						appended, err := runAppendResult(cfg, "", 1, true)
						if err != nil {
							return err
						}
						cfg.Batch.Dirty = true
						if err := writeJSON(filepath.Join(traceDir, appendFile), appended); err != nil {
							return err
						}
					}
					run, err := runCLI(cfg.Agentdeck, home(cfg), state(cfg), traceDir, cfg.TZ, scenario, "scan")
					if err = recordCLI(dir, run, err); err != nil {
						return err
					}
				}
				return nil
			}},
		// One B0/S0 pair per unit, B0 first in odd pairs and S0 first in even
		// ones, so consecutive pairs are ABBA groups. Every arm is its own CLI
		// run unit from an empty state root on the same sources. The scan runs
		// in a detached worker, so the front process's rusage is not the
		// rebuild's cost: S0 traces into <dir>/S0 (the worker's own lifetime,
		// stage spans and ingest counters), and B0, which has no new markers,
		// writes its original opt-in scan profile to <dir>/B0/profile.jsonl.
		"P09": {pilot: schedule{Units: 3}, formal: schedule{Units: 20},
			run: func(cfg runConfig, dir string, index int, _ schedule) error {
				if cfg.B0 == "" {
					return errors.New("P09 needs --b0")
				}
				for _, arm := range p09Order(index) {
					if err := cfg.Batch.startRunUnit(stateEmpty); err != nil {
						return err
					}
					armDir := filepath.Join(dir, arm)
					if err := os.Mkdir(armDir, 0o700); err != nil {
						return err
					}
					if err := os.Chmod(armDir, 0o700); err != nil {
						return err
					}
					var run cliRun
					var err error
					if arm == "B0" && cfg.P09BaselineTraced {
						run, err = runCLI(cfg.B0, home(cfg), state(cfg), armDir, cfg.TZ, arm, "scan")
						run.CostSource = "perftrace"
					} else if arm == "B0" {
						var profile bytes.Buffer
						run, err = runCLIWith(cfg.B0, home(cfg), state(cfg), "", cfg.TZ, arm, []string{b0ProfileEnv + "=1"}, &profile, "scan")
						if writeErr := os.WriteFile(filepath.Join(armDir, b0ProfileFile), profile.Bytes(), 0o600); err == nil {
							err = writeErr
						}
					} else {
						run, err = runCLI(cfg.Agentdeck, home(cfg), state(cfg), armDir, cfg.TZ, arm, "scan")
					}
					if err = recordCLI(dir, run, err); err != nil {
						return err
					}
				}
				return nil
			}},
		// One App run unit with one write sequence of Inner appends: write n
		// starts from the template plus the previous n-1 appends, and its frame
		// must show the hero expected after exactly those n appends.
		// Formal: 4 write sequences of 25, each from a regenerated working
		// copy, so the gate resamples whole A B B A groups (design decision of
		// 2026-10-24, MH-R1-F2).
		"P10": {pilot: schedule{Units: 1, Inner: 20}, formal: schedule{Units: 4, Inner: 25}, writes: true,
			run: runP10},
	}
}

// B0 (14234ec) prints its stage profile, worker CPU and peak RSS and ingest
// counters to stderr as one scan_performance JSON line when this is set.
const (
	b0ProfileEnv  = "AGENTDECK_SCAN_PROFILE"
	b0ProfileFile = "profile.jsonl"
	// appendFile is P08's append result, next to the append scan's traces.
	appendFile = "append.json"
)

func p09Order(index int) []string {
	if index%2 == 0 {
		return []string{"S0", "B0"}
	}
	return []string{"B0", "S0"}
}

// rebuildRecord is the background full rebuild of one P01-rebuild unit; stats
// accepts only clicks between its start and end.
type rebuildRecord struct {
	StartNS  int64  `json:"start_ns"`
	EndNS    int64  `json:"end_ns"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

const rebuildFile = "rebuild.json"

// The Perf driver requests the rebuild only after model.usable and settling.
// A slow App launch must not consume the rebuild before either click occurs.
func waitRebuildRequest(ctx context.Context, dir string, exited <-chan struct{}) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(filepath.Join(dir, "rebuild-request")); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-exited:
			return errors.New("App exited before requesting the rebuild")
		case <-ticker.C:
		}
	}
}

// runRebuild runs a full rebuild of the working copy into a separate state
// root, tracing into the sample (its processes' resources are P07's), and
// returns only after its scan worker has exited and the root is removed.
func runRebuild(ctx context.Context, cfg runConfig, traceDir string) rebuildRecord {
	state, err := os.MkdirTemp(cfg.Batch.Root, "rebuild-")
	if err != nil {
		return rebuildRecord{Error: err.Error()}
	}
	defer os.RemoveAll(state)
	command := exec.CommandContext(ctx, cfg.Agentdeck, "--state-dir", state, "scan")
	command.Env = []string{"HOME=" + cfg.Batch.Home, "TZ=" + cfg.TZ, "PATH=/usr/bin:/bin", perftrace.EnvOut + "=" + traceDir}
	addTestCLIEnvironment(command, cfg.Agentdeck)
	record := rebuildRecord{StartNS: perftrace.Now()}
	err = command.Start()
	if err == nil {
		// Release the driver only after a real rebuild process has started.
		if readyErr := os.WriteFile(filepath.Join(traceDir, "rebuild-ready"), []byte("ready\n"), 0o600); readyErr != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
			err = readyErr
		} else {
			err = command.Wait()
		}
	}
	record.EndNS = perftrace.Now()
	if command.ProcessState != nil {
		record.ExitCode = command.ProcessState.ExitCode()
	}
	if err != nil {
		record.Error = err.Error()
	}
	if waitErr := waitForWorkers(state, 60*time.Second); waitErr != nil && record.Error == "" {
		record.Error = waitErr.Error()
	}
	return record
}

// runP10 builds the write sequence's oracle before the App starts, then
// replays the planned records into the working copy one every writeInterval.
// After the run, an independent full scan of the working copy must agree
// with the last expected hero, or the oracle failed.
func runP10(cfg runConfig, dir string, _ int, s schedule) error {
	// Each write sequence starts from a regenerated working copy; the
	// previous sequence's appends mark it dirty.
	if err := cfg.Batch.ensureClean(); err != nil {
		return err
	}
	defer func() { cfg.Batch.Dirty = true }()
	if err := cfg.Batch.startRunUnit(stateFull); err != nil {
		return err
	}
	plans, err := os.MkdirTemp(cfg.Batch.Root, "appends-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(plans)
	expected, err := writeSequenceOracle(cfg, plans, s.Inner)
	if err != nil {
		return fmt.Errorf("oracle: %w", err)
	}
	if err := writeOracle(dir, expected); err != nil {
		return err
	}
	var failure error
	appends := func(ctx context.Context, exited <-chan struct{}) {
		wait := func(d time.Duration) bool {
			select {
			case <-time.After(d):
				return true
			case <-exited:
				return false
			case <-ctx.Done():
				return false
			}
		}
		for n := 1; n <= s.Inner; n++ {
			gap := writeInterval
			if n == 1 {
				gap = writeLead
			}
			if !wait(gap) {
				failure = fmt.Errorf("app exited before write %d", n)
				return
			}
			if failure = replayAppendCLI(cfg, dir, planPath(plans, n)); failure != nil {
				return
			}
		}
	}
	sleep := writeLead + time.Duration(s.Inner)*writeInterval + 35*time.Second
	steps := []map[string]any{step("wait_usable", "ms", 30000), step("sleep", "ms", 2000), click(5000, 1),
		step("sleep", "ms", sleep.Milliseconds())}
	err = launchApp(cfg, cfg.Batch.Home, dir, steps, sleep+3*time.Minute, appends)
	if err == nil {
		err = failure
	}
	if err == nil {
		final, finalErr := computeOracle(cfg.Agentdeck, cfg.Batch.Home, cfg.TZ, cfg.Batch.Anchor, cfg.Batch.Root)
		hero, ok := oracle{Heroes: final}.expect(clientAll, periodToday)
		switch {
		case finalErr != nil:
			err = fmt.Errorf("oracle: final scan: %w", finalErr)
		case !ok || hero != expected.Writes[len(expected.Writes)-1]:
			err = errors.New("oracle: a full scan after the run differs from the last expected write")
		}
	}
	return err
}

func planPath(dir string, n int) string {
	return filepath.Join(dir, fmt.Sprintf("append-%04d.json", n))
}

// writeSequenceOracle plans every write of the sequence on a mirror of the
// today-related sources and scans the mirror after each one, outside the
// timed run. The mirror must first agree with a full scan of the working copy.
func writeSequenceOracle(cfg runConfig, plans string, writes int) (oracle, error) {
	heroes, err := computeOracle(cfg.Agentdeck, cfg.Batch.Home, cfg.TZ, cfg.Batch.Anchor, cfg.Batch.Root)
	if err != nil {
		return oracle{}, err
	}
	expected := oracle{Heroes: heroes}
	base, ok := expected.expect(clientAll, periodToday)
	if !ok {
		return oracle{}, errors.New("no all/today hero")
	}
	location, err := time.LoadLocation(cfg.TZ)
	if err != nil {
		return oracle{}, err
	}
	midnight, err := time.ParseInLocation("2006-01-02", cfg.Batch.Anchor, location)
	if err != nil {
		return oracle{}, err
	}
	mirror, err := mirrorTodaySources(cfg.Batch.Home, midnight, cfg.Batch.Root)
	if err != nil {
		return oracle{}, err
	}
	defer os.RemoveAll(mirror)
	mirrorHero := func() (heroSummary, error) {
		heroes, err := computeOracle(cfg.Agentdeck, mirror, cfg.TZ, cfg.Batch.Anchor, cfg.Batch.Root)
		if err != nil {
			return heroSummary{}, err
		}
		hero, ok := oracle{Heroes: heroes}.expect(clientAll, periodToday)
		if !ok {
			return heroSummary{}, errors.New("mirror has no all/today hero")
		}
		return hero, nil
	}
	if hero, err := mirrorHero(); err != nil || hero != base {
		return oracle{}, fmt.Errorf("today-only mirror differs from the working copy: %+v vs %+v %v", hero, base, err)
	}
	for n := 1; n <= writes; n++ {
		plan, err := planAppend(mirror, midnight, cfg.Seed, n, false)
		if err != nil {
			return oracle{}, err
		}
		if _, err := writeAppend(mirror, plan, false); err != nil {
			return oracle{}, err
		}
		if err := writeJSON(planPath(plans, n), plan); err != nil {
			return oracle{}, err
		}
		hero, err := mirrorHero()
		if err != nil {
			return oracle{}, err
		}
		expected.Writes = append(expected.Writes, hero)
	}
	return expected, nil
}

// replayAppendCLI writes a planned append as its own process so its trace
// (role driver, source.written) lands in the sample directory.
func replayAppendCLI(cfg runConfig, traceDir, plan string) error {
	command := exec.Command(cfg.Perf, "append", "--home", cfg.Batch.Home, "--replay", plan)
	command.Env = []string{"PATH=/usr/bin:/bin", "TZ=" + cfg.TZ, perftrace.EnvOut + "=" + traceDir}
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("append %s: %v: %s", filepath.Base(plan), err, output)
	}
	return nil
}

// runAppendCLI runs this tool's append as its own process so its trace (role
// driver, source.written) lands in the sample directory.
func runAppendCLI(cfg runConfig, traceDir string, n int, all bool) error {
	_, err := runAppendResult(cfg, traceDir, n, all)
	return err
}

// runAppendResult runs one append and returns what it wrote.
func runAppendResult(cfg runConfig, traceDir string, n int, all bool) (appendResult, error) {
	args := []string{"append", "--home", cfg.Batch.Home, "--anchor", cfg.Batch.Anchor, "--tz", cfg.TZ,
		"--seed", strconv.FormatUint(cfg.Seed, 10), "--n", strconv.Itoa(n)}
	if all {
		args = append(args, "--all-today")
	}
	command := exec.Command(cfg.Perf, args...)
	command.Env = []string{"PATH=/usr/bin:/bin", "TZ=" + cfg.TZ}
	if traceDir != "" {
		command.Env = append(command.Env, perftrace.EnvOut+"="+traceDir)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return appendResult{}, fmt.Errorf("append %d: %v: %s", n, err, stderr.Bytes())
	}
	var result appendResult
	if err := json.Unmarshal(output, &result); err != nil {
		return appendResult{}, fmt.Errorf("append %d: result: %w", n, err)
	}
	return result, nil
}

// sampleDirs lists a batch's sample directories in order.
func sampleDirs(root string) ([]string, error) {
	dirs, err := filepath.Glob(filepath.Join(root, "sample-*"))
	sort.Strings(dirs)
	return dirs, err
}
