package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// corpusRoots are the three source roots the product discovers, relative to
// HOME. Nothing else is ever copied from the real HOME.
var corpusRoots = []string{".codex/sessions", ".codex/archived_sessions", ".claude/projects"}

// Representativeness floor: half the recorded aggregate of today's share.
const (
	minTodayFiles = 56
	minTodayBytes = 19_000_000
)

// corpusManifest is the frozen corpus identity and its aggregates. It is the
// only description of the corpus that may enter records.
type corpusManifest struct {
	Identity       string              `json:"identity"`
	FrozenAt       string              `json:"frozen_at"`
	Timezone       string              `json:"timezone"`
	Files          int                 `json:"files"`
	Bytes          int64               `json:"bytes"`
	ByClient       map[string][2]int64 `json:"by_client"` // files, bytes
	LargeFiles     int                 `json:"large_files"`
	FirstDate      string              `json:"first_date"`
	LastDate       string              `json:"last_date"`
	LastDayFiles   int                 `json:"last_day_files"`
	LastDayBytes   int64               `json:"last_day_bytes"`
	OverlapRecords int                 `json:"overlap_records"`
	TruncatedBytes int64               `json:"truncated_bytes"`
	Representative bool                `json:"representative"`
}

func runCorpus(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: agentdeck-perf corpus <freeze|shift> [flags]")
		return 2
	}
	switch args[0] {
	case "freeze":
		flags := flag.NewFlagSet("corpus freeze", flag.ContinueOnError)
		flags.SetOutput(stderr)
		home := flags.String("home", "", "real HOME to copy the three source roots from")
		store := flags.String("store", "", "private corpus store directory")
		tz := flags.String("tz", "America/New_York", "batch timezone")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		manifest, err := freezeCorpus(*home, *store, *tz, time.Now())
		if err == nil {
			err = printJSON(stdout, manifest)
		}
		return report(stderr, "corpus freeze", err)
	case "shift":
		flags := flag.NewFlagSet("corpus shift", flag.ContinueOnError)
		flags.SetOutput(stderr)
		corpus := flags.String("corpus", "", "frozen corpus directory")
		out := flags.String("out", "", "working copy HOME (fixed batch path, must not exist)")
		anchor := flags.String("anchor", "", "anchor date YYYY-MM-DD")
		tz := flags.String("tz", "America/New_York", "batch timezone")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		result, err := shiftCorpusCommand(*corpus, *out, *anchor, *tz)
		if err == nil {
			err = printJSON(stdout, result)
		}
		return report(stderr, "corpus shift", err)
	}
	fmt.Fprintf(stderr, "agentdeck-perf corpus: unknown command %q\n", args[0])
	return 2
}

func printJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func report(stderr io.Writer, name string, err error) int {
	if err != nil {
		fmt.Fprintf(stderr, "agentdeck-perf %s: %v\n", name, err)
		return 1
	}
	return 0
}

// freezeCorpus copies the three source roots into a private staging
// directory, truncates each file to its last newline, keeps mtimes, and
// renames the copy to its identity. Every call reads real logs, so each one
// needs the user's prior approval.
func freezeCorpus(home, store, tz string, now time.Time) (corpusManifest, error) {
	if !filepath.IsAbs(home) || !filepath.IsAbs(store) {
		return corpusManifest{}, errors.New("--home and --store must be absolute")
	}
	location, err := time.LoadLocation(tz)
	if err != nil {
		return corpusManifest{}, err
	}
	if err := os.MkdirAll(store, 0o700); err != nil {
		return corpusManifest{}, err
	}
	if err := os.Chmod(store, 0o700); err != nil {
		return corpusManifest{}, err
	}
	staging, err := os.MkdirTemp(store, ".freeze-")
	if err != nil {
		return corpusManifest{}, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(staging)
		}
	}()
	// Only records before the freeze day's local midnight are kept, so the
	// last day of the corpus is a complete day.
	local := now.In(location)
	cutoff := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	var truncated int64
	for _, root := range corpusRoots {
		source := filepath.Join(home, root)
		err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
			if errors.Is(walkErr, fs.ErrNotExist) && path == source {
				return fs.SkipDir
			}
			if walkErr != nil || entry.IsDir() || !entry.Type().IsRegular() || !strings.HasSuffix(path, ".jsonl") {
				return walkErr
			}
			rel, err := filepath.Rel(home, path)
			if err != nil {
				return err
			}
			cut, err := copyBeforeCutoff(path, filepath.Join(staging, rel), cutoff)
			truncated += cut
			return err
		})
		if err != nil {
			return corpusManifest{}, err
		}
	}
	manifest, err := describeCorpus(staging, location)
	if err != nil {
		return corpusManifest{}, err
	}
	manifest.FrozenAt = now.In(location).Format(time.RFC3339)
	manifest.TruncatedBytes = truncated
	target := filepath.Join(store, manifest.Identity)
	if _, err := os.Stat(target); err == nil {
		return corpusManifest{}, fmt.Errorf("corpus %s already exists", manifest.Identity)
	}
	if err := os.Rename(staging, target); err != nil {
		return corpusManifest{}, err
	}
	keep = true
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(target+".manifest.json", append(raw, '\n'), 0o600); err != nil {
		return corpusManifest{}, err
	}
	return manifest, nil
}

// copyBeforeCutoff copies a source up to its first record at or after cutoff
// and up to its last newline (dropping a line still being written). A file
// whose records all start at or after cutoff is not copied. The copy's mtime
// is its last kept record time, or the source mtime if it has none.
func copyBeforeCutoff(source, target string, cutoff time.Time) (int64, error) {
	info, err := os.Stat(source)
	if err != nil {
		return 0, err
	}
	raw, err := os.ReadFile(source)
	if err != nil {
		return 0, err
	}
	end := bytes.LastIndexByte(raw, '\n') + 1
	kept := raw[:end]
	var last time.Time
	offset := 0
	for offset < len(kept) {
		next := bytes.IndexByte(kept[offset:], '\n') + offset + 1
		line := kept[offset:next]
		stop := false
		for _, f := range timestampFields(line) {
			at, err := time.Parse(time.RFC3339Nano, string(line[f.start:f.end]))
			if err != nil {
				continue
			}
			if !at.Before(cutoff) {
				stop = true
				break
			}
			if at.After(last) {
				last = at
			}
		}
		if stop {
			kept = kept[:offset]
			break
		}
		offset = next
	}
	dropped := int64(len(raw) - len(kept))
	if len(kept) == 0 {
		return dropped, nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return 0, err
	}
	if err := os.WriteFile(target, kept, 0o600); err != nil {
		return 0, err
	}
	mtime := info.ModTime()
	if !last.IsZero() {
		mtime = last
	}
	return dropped, os.Chtimes(target, mtime, mtime)
}

// describeCorpus computes identity and aggregates; record dates are local
// dates in the batch timezone.
func describeCorpus(root string, location *time.Location) (corpusManifest, error) {
	identity, err := fixtureIdentity(root)
	if err != nil {
		return corpusManifest{}, err
	}
	manifest := corpusManifest{Identity: identity, Timezone: location.String(), ByClient: map[string][2]int64{}}
	type fileDates struct {
		size int64
		last time.Time
	}
	var files []fileDates
	var first, last time.Time
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return walkErr
		}
		rel, _ := filepath.Rel(root, path)
		client := "claude"
		if strings.HasPrefix(rel, ".codex") {
			client = "codex"
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		size := int64(len(raw))
		manifest.Files++
		manifest.Bytes += size
		counts := manifest.ByClient[client]
		manifest.ByClient[client] = [2]int64{counts[0] + 1, counts[1] + size}
		if size >= 32*mib {
			manifest.LargeFiles++
		}
		var fileLast time.Time
		for _, line := range bytes.Split(raw, []byte("\n")) {
			for _, field := range timestampFields(line) {
				at, err := time.Parse(time.RFC3339Nano, string(line[field.start:field.end]))
				if err != nil {
					continue
				}
				if first.IsZero() || at.Before(first) {
					first = at
				}
				if at.After(last) {
					last = at
				}
				if at.After(fileLast) {
					fileLast = at
				}
				if inOverlap(at, location) {
					manifest.OverlapRecords++
				}
			}
		}
		files = append(files, fileDates{size, fileLast})
		return nil
	})
	if err != nil {
		return corpusManifest{}, err
	}
	if last.IsZero() {
		return corpusManifest{}, errors.New("corpus has no parseable record timestamps")
	}
	manifest.FirstDate = first.In(location).Format("2006-01-02")
	manifest.LastDate = last.In(location).Format("2006-01-02")
	for _, file := range files {
		if !file.last.IsZero() && file.last.In(location).Format("2006-01-02") == manifest.LastDate {
			manifest.LastDayFiles++
			manifest.LastDayBytes += file.size
		}
	}
	manifest.Representative = manifest.LastDayFiles >= minTodayFiles && manifest.LastDayBytes >= minTodayBytes
	return manifest, nil
}

// inOverlap reports whether an instant's wall clock occurs twice in location
// (the autumn repeated hour).
func inOverlap(at time.Time, location *time.Location) bool {
	local := at.In(location)
	return len(instantsForWall(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), location)) == 2
}

// field is the byte span of one timestamp string value inside a line.
type field struct{ start, end int }

// timestampFields finds the contract's time fields by byte position: the
// top-level "timestamp", and payload.timestamp when the record is a Codex
// session_meta. Positions come from the JSON tokenizer, so text that merely
// contains the word "timestamp" is never touched.
func timestampFields(line []byte) []field {
	if len(bytes.TrimSpace(line)) == 0 || !bytes.Contains(line, []byte(`"timestamp"`)) {
		return nil
	}
	type frame struct {
		object  bool
		wantKey bool
		key     string // current key inside an object
		name    string // key of this container in its parent
	}
	var stack []frame
	var out []field
	var payloadTimestamp *field
	recordType := ""
	decoder := json.NewDecoder(bytes.NewReader(line))
	consumed := func() {
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].wantKey = true
		}
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{', '[':
				name := ""
				if n := len(stack); n > 0 && stack[n-1].object {
					name = stack[n-1].key
				}
				stack = append(stack, frame{object: delim == '{', wantKey: delim == '{', name: name})
			default:
				stack = stack[:len(stack)-1]
				consumed()
			}
			continue
		}
		n := len(stack)
		if n == 0 {
			break
		}
		top := &stack[n-1]
		if top.object && top.wantKey {
			top.key, _ = token.(string)
			top.wantKey = false
			continue
		}
		if value, ok := token.(string); ok && top.object {
			end := int(decoder.InputOffset())
			start := end - len(value) - 2
			if start >= 0 && line[start] == '"' && line[end-1] == '"' {
				span := field{start + 1, end - 1}
				switch {
				case top.key == "timestamp" && n == 1:
					out = append(out, span)
				case top.key == "timestamp" && n == 2 && top.name == "payload":
					payloadTimestamp = &span
				case top.key == "type" && n == 1:
					recordType = value
				}
			}
		}
		consumed()
	}
	if payloadTimestamp != nil && recordType == "session_meta" {
		out = append(out, *payloadTimestamp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}
