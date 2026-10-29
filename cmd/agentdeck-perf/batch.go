package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// batchState is one batch's working copy and template. The working copy
// lives at a fixed HOME so the template, whose source rows hold absolute
// paths, inodes and ctimes, stays valid for every run unit of the batch.
type batchState struct {
	Root        string           `json:"root"`
	Home        string           `json:"home"`
	Corpus      string           `json:"corpus"`
	Anchor      string           `json:"anchor"`
	TZ          string           `json:"tz"`
	Agentdeck   string           `json:"agentdeck"`
	WorkingCopy shiftResult      `json:"working_copy"`
	Template    templateManifest `json:"template"`
	Binding     string           `json:"source_binding"`
	Dirty       bool             `json:"dirty"`
	Generations int              `json:"generations"`
	// UnitDir is the run unit attempt being prepared; startRunUnit records
	// there what it actually restored. Never persisted.
	UnitDir string `json:"-"`
}

func (b *batchState) save() error { return writeJSON(filepath.Join(b.Root, "batch.json"), b) }

func loadBatch(root string) (*batchState, error) {
	raw, err := os.ReadFile(filepath.Join(root, "batch.json"))
	if err != nil {
		return nil, err
	}
	var batch batchState
	return &batch, json.Unmarshal(raw, &batch)
}

func runBatchPrepare(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("batch prepare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	corpus := flags.String("corpus", "", "frozen corpus directory")
	anchor := flags.String("anchor", "", "anchor date YYYY-MM-DD")
	tz := flags.String("tz", "America/New_York", "batch timezone")
	agentdeck := flags.String("agentdeck", "", "agentdeck CLI under test, used to build the template")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !filepath.IsAbs(*corpus) || !filepath.IsAbs(*agentdeck) {
		return report(stderr, "batch prepare", errors.New("--corpus and --agentdeck must be absolute"))
	}
	// The App's isolation accepts only this prefix for a test HOME.
	root, err := os.MkdirTemp("/private/tmp", "agentdeck-perf.")
	if err != nil {
		return report(stderr, "batch prepare", err)
	}
	root = appPathForm(root)
	batch := &batchState{Root: root, Home: filepath.Join(root, "home"), Corpus: *corpus, Anchor: *anchor, TZ: *tz, Agentdeck: *agentdeck}
	if err := batch.regenerate(); err != nil {
		return report(stderr, "batch prepare", err)
	}
	return report(stderr, "batch prepare", printJSON(stdout, batch))
}

// appPathForm writes a path the way the App sees it. The App standardizes its
// test HOME, which drops the /private prefix of /private/tmp, and its helpers
// scan sources under that HOME. The product binds imported state to the HOME
// string, so a template built under /private/tmp/... fails every App scan of
// the same files under /tmp/.... Template, CLI runs and App therefore all use
// the App's form.
func appPathForm(path string) string {
	if rest, ok := strings.CutPrefix(path, "/private/tmp/"); ok {
		return "/tmp/" + rest
	}
	return path
}

// regenerate replaces the working copy at the fixed HOME with a fresh shift
// of the corpus and lets the product rebuild the template from it.
func (b *batchState) regenerate() error {
	for _, path := range []string{b.Home, filepath.Join(b.Root, "template")} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	shifted, err := shiftCorpusCommand(b.Corpus, b.Home, b.Anchor, b.TZ)
	if err != nil {
		return err
	}
	template, err := buildTemplate(b.Agentdeck, b.Home, b.TZ, filepath.Join(b.Root, "template"))
	if err != nil {
		return err
	}
	binding, err := sourceBinding(b.Home)
	if err != nil {
		return err
	}
	b.WorkingCopy, b.Template, b.Binding, b.Dirty = shifted, template, binding, false
	b.Generations++
	return b.save()
}

// ensureClean regenerates a working copy that a writing condition changed.
func (b *batchState) ensureClean() error {
	if !b.Dirty {
		return nil
	}
	return b.regenerate()
}

// sourceBinding fingerprints the source tree as the product's "unchanged"
// check sees it: path, device, inode, size, mtime and ctime of every source.
func sourceBinding(home string) (string, error) {
	var lines []string
	for _, root := range corpusRoots {
		err := filepath.WalkDir(filepath.Join(home, root), func(path string, entry fs.DirEntry, walkErr error) error {
			if errors.Is(walkErr, fs.ErrNotExist) {
				return fs.SkipDir
			}
			if walkErr != nil || entry.IsDir() {
				return walkErr
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return errors.New("source binding needs stat information")
			}
			rel, _ := filepath.Rel(home, path)
			lines = append(lines, fmt.Sprintf("%s:%d:%d:%d:%d:%d", filepath.ToSlash(rel), stat.Dev, stat.Ino, info.Size(),
				info.ModTime().UnixNano(), stat.Ctimespec.Sec*1_000_000_000+stat.Ctimespec.Nsec))
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// startRunUnit checks the source binding and resets only state: the state
// root, the App's isolated App Group directory and its defaults. Source
// files are never copied or touched.
func (b *batchState) startRunUnit(mode stateMode) error {
	binding, err := sourceBinding(b.Home)
	if err != nil {
		return err
	}
	if binding != b.Binding {
		return errors.New("source tree no longer matches the template binding")
	}
	state := filepath.Join(b.Home, ".agentdeck")
	for _, path := range []string{state, filepath.Join(b.Home, "app-group"), filepath.Join(b.Home, "Library")} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	if err := os.Mkdir(state, 0o700); err != nil {
		return err
	}
	restored := map[string]string{}
	if mode != stateEmpty {
		entries, err := os.ReadDir(filepath.Join(b.Root, "template"))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			name := entry.Name()
			if mode == stateNoCache && (name == "desktop-serving-snapshot.json" || name == "desktop-derived-cache.json") {
				continue
			}
			if out, err := exec.Command("/bin/cp", "-c", filepath.Join(b.Root, "template", name), state+"/").CombinedOutput(); err != nil {
				return fmt.Errorf("copy template: %v: %s", err, out)
			}
			if restored[name], err = fileSHA256(filepath.Join(state, name)); err != nil {
				return err
			}
		}
	}
	return b.recordUnitStart(mode, restored)
}

// unitIdentityFile lists, one entry per startRunUnit of the attempt, the data
// state the attempt actually started from (performance evaluation, records
// and evidence). It travels with the attempt directory, so an interrupted
// attempt keeps the identity it ran on. It is a JSON array, not .jsonl: stats
// reads every .jsonl in a sample as a trace.
const unitIdentityFile = "unit-identity.json"

type unitIdentity struct {
	Mode        string            `json:"state_mode"`
	Generation  int               `json:"working_copy_generation"`
	AnchorDate  string            `json:"anchor_date"`
	Corpus      string            `json:"corpus_identity"`
	WorkingCopy string            `json:"working_copy_identity"`
	ShiftDays   int               `json:"shift_days"`
	Unparsed    int               `json:"timestamps_unparsed"`
	Binding     string            `json:"source_binding"`
	Template    templateManifest  `json:"template"`
	Restored    map[string]string `json:"restored_sha256"`
}

// recordUnitStart refuses a restore whose bytes are not the template the
// batch names, then appends the identity to the attempt's record.
func (b *batchState) recordUnitStart(mode stateMode, restored map[string]string) error {
	want := map[string]string{"agentdeck.sqlite3": b.Template.CoreSHA256, "sessions.sqlite3": b.Template.SessionSHA256,
		"desktop-serving-snapshot.json": b.Template.ServingSHA256}
	for name, sum := range restored {
		if expected, ok := want[name]; ok && expected != sum {
			return fmt.Errorf("restored %s does not match the batch template", name)
		}
	}
	if b.UnitDir == "" {
		return nil
	}
	path := filepath.Join(b.UnitDir, unitIdentityFile)
	var records []unitIdentity
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &records); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	records = append(records, unitIdentity{Mode: [...]string{"full", "no_cache", "empty"}[mode], Generation: b.Generations,
		AnchorDate: b.Anchor, Corpus: b.WorkingCopy.Corpus, WorkingCopy: b.WorkingCopy.Identity, ShiftDays: b.WorkingCopy.Days,
		Unparsed: b.WorkingCopy.Unparsed, Binding: b.Binding, Template: b.Template, Restored: restored})
	return writeJSON(path, records)
}

// noOpCheck is the template start invariant: a scan right after a reset reads
// no source bytes and decodes no records. It runs as its own CLI run unit.
func (b *batchState) noOpCheck(traceDir string) (map[string]int64, error) {
	if err := b.startRunUnit(stateFull); err != nil {
		return nil, err
	}
	if _, err := runCLI(b.Agentdeck, b.Home, filepath.Join(b.Home, ".agentdeck"), traceDir, b.TZ, "noop", "scan"); err != nil {
		return nil, err
	}
	return scanEndCounts(traceDir)
}

// scanEndCounts sums scan.end counters over the traces in dir.
func scanEndCounts(dir string) (map[string]int64, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	counts := map[string]int64{}
	for _, path := range paths {
		file, err := readTraceFile(path)
		if err != nil {
			return nil, err
		}
		for _, record := range file.Records {
			if record.Stage == "scan.end" {
				for _, key := range []string{"read_bytes", "decoded_records", "usage_imported", "session_documents"} {
					counts[key] += record.N[key]
				}
			}
		}
	}
	return counts, nil
}

// batchWindowOpen reports whether a run may start now: the anchor is today
// and both midnights are at least 45 minutes away. The corpus ends with a
// complete day, so later records of the shifted day may lie after the start;
// the product files records by date.
func batchWindowOpen(b *batchState, now time.Time) error {
	location, err := time.LoadLocation(b.TZ)
	if err != nil {
		return err
	}
	local := now.In(location)
	if local.Format("2006-01-02") != b.Anchor {
		return fmt.Errorf("anchor %s is not today's date %s", b.Anchor, local.Format("2006-01-02"))
	}
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	if next := today.AddDate(0, 0, 1); next.Sub(local) < 45*time.Minute || local.Sub(today) < 45*time.Minute {
		return errors.New("batch must start at least 45 minutes from local midnight")
	}
	return nil
}
