package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/usage"
)

// heroSummary is what a data frame's hero shows, as the App records it on
// frame.vsync (PerfDataFrame): totals, client and period codes, the day its
// label names and a digest of the cost fields its amount is formatted from.
type heroSummary struct {
	Client   int64 `json:"client"`
	Period   int64 `json:"period"`
	Tokens   int64 `json:"tokens"`
	Events   int64 `json:"events"`
	Sessions int64 `json:"sessions"`
	Day      int64 `json:"day"`
	Cost     int64 `json:"cost"`
}

func heroFromCounts(n map[string]int64) heroSummary {
	return heroSummary{Client: n["client"], Period: n["period"], Tokens: n["tokens"], Events: n["events"],
		Sessions: n["sessions"], Day: n["day"], Cost: n["cost"]}
}

// oracle is the expected content of one sample, built outside the timed
// window: Heroes for every client and period from an independent full scan of
// the sample's sources; for P10, Writes[n-1] is the default hero after the
// n-th append.
type oracle struct {
	Heroes []heroSummary `json:"heroes"`
	Writes []heroSummary `json:"writes,omitempty"`
	// Batches are the publications of an App-owned bootstrap from empty
	// state, by usage coverage (P11-first).
	Batches []coverageHeroes `json:"batches,omitempty"`
}

// coverageHeroes is one bootstrap publication: its usage coverage (counts
// and whether the state is complete) and the heroes it presents. Partial is
// the marker a surface must show for it.
type coverageHeroes struct {
	Committed int64         `json:"committed"`
	Sources   int64         `json:"sources"`
	Complete  bool          `json:"complete"`
	Partial   bool          `json:"partial"`
	Heroes    []heroSummary `json:"heroes"`
}

// expectBatch finds the publication with the given coverage. The final count
// is published twice, before and after the bootstrap finalizes, so the
// coverage state is part of its identity.
func (o oracle) expectBatch(committed, sources int64, complete bool) (coverageHeroes, bool) {
	for _, batch := range o.Batches {
		if batch.Committed == committed && batch.Sources == sources && batch.Complete == complete {
			return batch, true
		}
	}
	return coverageHeroes{}, false
}

const oracleFile = "oracle.json"

// Default selection of the App: all clients, today.
const (
	clientAll   = 1
	periodToday = 1
)

func (o oracle) expect(client, period int64) (heroSummary, bool) {
	for _, hero := range o.Heroes {
		if hero.Client == client && hero.Period == period {
			return hero, true
		}
	}
	return heroSummary{}, false
}

func readOracle(dir string) (oracle, error) {
	raw, err := os.ReadFile(filepath.Join(dir, oracleFile))
	if err != nil {
		return oracle{}, fmt.Errorf("oracle: %w", err)
	}
	var out oracle
	if err := json.Unmarshal(raw, &out); err != nil {
		return oracle{}, fmt.Errorf("oracle: %w", err)
	}
	if len(out.Heroes) == 0 {
		return oracle{}, errors.New("oracle: no expected heroes")
	}
	return out, nil
}

// fnv1a matches PerfDataFrame.fnv1a: FNV-1a over NUL-joined fields.
func fnv1a(fields ...string) int64 {
	hash := uint64(0xcbf29ce484222325)
	for _, b := range []byte(strings.Join(fields, "\x00")) {
		hash = (hash ^ uint64(b)) * 0x100000001b3
	}
	return int64(hash)
}

var (
	clientCodes = map[string]int64{"all": 1, "codex": 2, "claude": 3}
	periodCodes = map[string]int64{"today": 1, "7d": 2, "30d": 3}
)

// costDigest matches PerfDataFrame.cost; a missing value is U+0001.
func costDigest(totals usage.PresentationTotals) int64 {
	optional := func(value *string) string {
		if value == nil {
			return "\x01"
		}
		return *value
	}
	complete := "0"
	if totals.PricingComplete {
		complete = "1"
	}
	return fnv1a(optional(totals.CatalogBaseCost), optional(totals.ProviderCost), totals.KnownCatalogBaseCost,
		totals.KnownProviderCost, complete, strconv.Itoa(totals.UnpricedComponents))
}

// presentationHeroes converts a desktop snapshot's presentation into the
// heroes the App can show; day is the date its label names.
func presentationHeroes(report usage.PresentationReport, day int64) []heroSummary {
	var out []heroSummary
	for _, scope := range report.Scopes {
		client, ok := clientCodes[scope.Client]
		if !ok || !scope.Periods.Available {
			continue
		}
		for _, item := range scope.Periods.Items {
			period, ok := periodCodes[item.Period]
			if !ok {
				continue
			}
			out = append(out, heroSummary{Client: client, Period: period, Tokens: item.Totals.Tokens,
				Events: item.Totals.Events, Sessions: item.Totals.Sessions, Day: day, Cost: costDigest(item.Totals)})
		}
	}
	return out
}

// computeOracle scans home's sources into a fresh state root with the
// product CLI and reads the presentation every hero is drawn from. It runs
// outside every timed window; any failure is an oracle failure, which makes
// the samples it serves tool-invalid.
func computeOracle(agentdeck, home, tz, anchor, scratch string) ([]heroSummary, error) {
	state, err := os.MkdirTemp(scratch, "oracle-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(state)
	env := []string{"HOME=" + home, "TZ=" + tz, "PATH=/usr/bin:/bin", "LANG=en_US_POSIX", "LC_ALL=en_US_POSIX"}
	scan := exec.Command(agentdeck, "--state-dir", state, "scan")
	scan.Env = env
	addTestCLIEnvironment(scan, scan.Path)
	if output, err := scan.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("oracle scan: %v: %.200s", err, output)
	}
	if err := waitForWorkers(state, 60*time.Second); err != nil {
		return nil, fmt.Errorf("oracle scan: %w", err)
	}
	read := exec.Command(agentdeck, "--state-dir", state, "--format", "json", "desktop", "snapshot",
		"--wire-version", "1", "--recent-limit", "1")
	read.Env = env
	addTestCLIEnvironment(read, read.Path)
	raw, err := read.Output()
	if err != nil {
		return nil, fmt.Errorf("oracle snapshot: %w", err)
	}
	var envelope struct {
		Data struct {
			GeneratedAt string `json:"generated_at"`
			Usage       struct {
				Presentation usage.PresentationReport `json:"presentation"`
			} `json:"usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("oracle snapshot: %w", err)
	}
	day, err := dayCode(envelope.Data.GeneratedAt, tz)
	if err != nil {
		return nil, err
	}
	if want := anchorCode(anchor); day != want {
		return nil, fmt.Errorf("oracle day %d is not the anchor date %d (crossed midnight)", day, want)
	}
	heroes := presentationHeroes(envelope.Data.Usage.Presentation, day)
	if len(heroes) == 0 {
		return nil, errors.New("oracle snapshot has no usage presentation")
	}
	return heroes, nil
}

// computeBootstrapOracle runs the App-owned bootstrap with the product CLI
// into a fresh state root, outside every timed window, and records each
// publication's coverage and heroes. Its order is deterministic (today's
// sources first, 64 per batch), so the App's first-batch frame must show the
// heroes of the publication with the same coverage. The last publication
// must be complete and agree with the full-scan heroes, or the oracle failed.
func computeBootstrapOracle(agentdeck, home, tz, anchor, scratch string, full []heroSummary) ([]coverageHeroes, error) {
	state, err := os.MkdirTemp(scratch, "bootstrap-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(state)
	command := exec.Command(agentdeck, "--state-dir", state, "--format", "ndjson", "desktop", "bootstrap")
	command.Env = []string{"HOME=" + home, "TZ=" + tz, "PATH=/usr/bin:/bin", "LANG=en_US_POSIX", "LC_ALL=en_US_POSIX"}
	addTestCLIEnvironment(command, command.Path)
	raw, err := command.Output()
	if waitErr := waitForWorkers(state, 60*time.Second); err == nil {
		err = waitErr
	}
	if err != nil {
		return nil, fmt.Errorf("oracle bootstrap: %w", err)
	}
	want := anchorCode(anchor)
	var batches []coverageHeroes
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var event struct {
			Type string `json:"type"`
			Data struct {
				Header struct {
					UsageCoverage struct {
						State     string `json:"state"`
						Sources   int64  `json:"sources"`
						Committed int64  `json:"committed"`
					} `json:"usage_coverage"`
				} `json:"header"`
				Payload struct {
					Partial bool `json:"partial"`
					Data    struct {
						GeneratedAt string `json:"generated_at"`
						Usage       struct {
							Presentation usage.PresentationReport `json:"presentation"`
						} `json:"usage"`
					} `json:"data"`
				} `json:"payload"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("oracle bootstrap: %w", err)
		}
		if event.Type != "publication" {
			continue
		}
		day, err := dayCode(event.Data.Payload.Data.GeneratedAt, tz)
		if err != nil {
			return nil, err
		}
		if day != want {
			return nil, fmt.Errorf("oracle bootstrap day %d is not the anchor date %d (crossed midnight)", day, want)
		}
		coverage := event.Data.Header.UsageCoverage
		batches = append(batches, coverageHeroes{Committed: coverage.Committed, Sources: coverage.Sources,
			Complete: coverage.State == "complete", Partial: coverage.State != "complete" || event.Data.Payload.Partial,
			Heroes: presentationHeroes(event.Data.Payload.Data.Usage.Presentation, day)})
	}
	if len(batches) == 0 {
		return nil, errors.New("oracle bootstrap published nothing")
	}
	last := batches[len(batches)-1]
	if last.Partial || fmt.Sprint(last.Heroes) != fmt.Sprint(full) {
		return nil, errors.New("oracle bootstrap's last publication differs from the full scan")
	}
	return batches, nil
}

func dayCode(timestamp, tz string) (int64, error) {
	location, err := time.LoadLocation(tz)
	if err != nil {
		return 0, err
	}
	at, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return 0, fmt.Errorf("oracle generated_at: %w", err)
	}
	local := at.In(location)
	return int64(local.Year()*10000 + int(local.Month())*100 + local.Day()), nil
}

func anchorCode(anchor string) int64 {
	value, _ := strconv.ParseInt(anchor[:4]+anchor[5:7]+anchor[8:10], 10, 64)
	return value
}

// mirrorTodaySources copies the today-related source files of home, the only
// files a today hero depends on, to a new private directory with their
// modification times. P10 computes each write's expected hero there before the
// measured run, instead of rescanning the whole working copy per write.
func mirrorTodaySources(home string, midnight time.Time, scratch string) (string, error) {
	files, err := todayFiles(home, midnight)
	if err != nil {
		return "", err
	}
	mirror, err := os.MkdirTemp(scratch, "mirror-")
	if err != nil {
		return "", err
	}
	for _, path := range files {
		relative, err := filepath.Rel(home, path)
		if err != nil {
			return "", err
		}
		target := filepath.Join(mirror, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return "", err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(target, raw, 0o600); err != nil {
			return "", err
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if err := os.Chtimes(target, info.ModTime(), info.ModTime()); err != nil {
			return "", err
		}
	}
	return mirror, nil
}

// writeOracle saves an oracle into a sample directory.
func writeOracle(dir string, o oracle) error {
	return writeJSON(filepath.Join(dir, oracleFile), o)
}
