package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// shiftResult describes one working copy; it carries no paths or content.
type shiftResult struct {
	Corpus          string `json:"corpus_identity"`
	Identity        string `json:"working_copy_identity"`
	AnchorDate      string `json:"anchor_date"`
	Timezone        string `json:"timezone"`
	Days            int    `json:"shift_days"`
	Files           int    `json:"files"`
	Rewritten       int    `json:"timestamps_rewritten"`
	Unparsed        int    `json:"timestamps_unparsed"`
	FallbackFiles   int    `json:"utc_fallback_files"`
	FallbackDayMove int    `json:"utc_fallback_day_changes"`
	LastRecord      string `json:"last_record_local"`
}

func shiftCorpusCommand(corpus, out, anchor, tz string) (shiftResult, error) {
	location, err := time.LoadLocation(tz)
	if err != nil {
		return shiftResult{}, err
	}
	day, err := time.ParseInLocation("2006-01-02", anchor, location)
	if err != nil {
		return shiftResult{}, fmt.Errorf("--anchor: %w", err)
	}
	raw, err := os.ReadFile(strings.TrimSuffix(corpus, "/") + ".manifest.json")
	if err != nil {
		return shiftResult{}, fmt.Errorf("corpus manifest: %w", err)
	}
	var manifest corpusManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return shiftResult{}, err
	}
	if !manifest.Representative {
		return shiftResult{}, errors.New("corpus is below the representativeness floor; re-freeze with approval")
	}
	last, err := time.ParseInLocation("2006-01-02", manifest.LastDate, location)
	if err != nil {
		return shiftResult{}, err
	}
	days := int(day.Sub(last).Hours()/24 + 0.5)
	if day.Sub(last) < 0 {
		return shiftResult{}, errors.New("anchor date precedes the corpus's last date")
	}
	result, err := shiftCorpus(corpus, out, days, location)
	result.Corpus, result.AnchorDate = manifest.Identity, anchor
	return result, err
}

// instantsForWall returns every instant whose wall clock in location is the
// given one: one normally, two in an autumn repeated hour, none in a spring
// gap. It never relies on time.Date's unspecified choice at transitions.
func instantsForWall(year int, month time.Month, day, hour, minute, second, nanosecond int, location *time.Location) []time.Time {
	wall := time.Date(year, month, day, hour, minute, second, nanosecond, time.UTC)
	var out []time.Time
	seen := map[int]bool{}
	for _, probe := range []time.Time{wall.Add(-36 * time.Hour), wall.Add(36 * time.Hour)} {
		_, offset := probe.In(location).Zone()
		if seen[offset] {
			continue
		}
		seen[offset] = true
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(location)
		if local.Year() == year && local.Month() == month && local.Day() == day && local.Hour() == hour &&
			local.Minute() == minute && local.Second() == second && local.Nanosecond() == nanosecond {
			out = append(out, candidate)
		}
	}
	if len(out) == 2 && out[1].Before(out[0]) {
		out[0], out[1] = out[1], out[0]
	}
	return out
}

// shiftLocal moves an instant by days calendar days of its wall clock in
// location: a unique result is taken; a skipped spring time uses the offset
// in effect before the transition; a repeated autumn time takes the earlier.
func shiftLocal(at time.Time, days int, location *time.Location) time.Time {
	local := at.In(location)
	target := time.Date(local.Year(), local.Month(), local.Day()+days, 12, 0, 0, 0, time.UTC)
	candidates := instantsForWall(target.Year(), target.Month(), target.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), location)
	if len(candidates) > 0 {
		return candidates[0]
	}
	wall := time.Date(target.Year(), target.Month(), target.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), time.UTC)
	_, before := wall.Add(-36 * time.Hour).In(location).Zone()
	return wall.Add(-time.Duration(before) * time.Second)
}

// formatLike writes at in the same shape as original: UTC for "Z", the same
// numeric offset otherwise, with the same count of fractional digits, so the
// rewritten field has the original length.
func formatLike(original string, at time.Time) (string, bool) {
	fraction := 0
	if dot := strings.IndexByte(original, '.'); dot >= 0 {
		end := dot + 1
		for end < len(original) && original[end] >= '0' && original[end] <= '9' {
			end++
		}
		fraction = end - dot - 1
	}
	layout := "2006-01-02T15:04:05"
	if fraction > 0 {
		layout += "." + strings.Repeat("0", fraction)
	}
	var text string
	if strings.HasSuffix(original, "Z") {
		text = at.UTC().Format(layout) + "Z"
	} else {
		parsed, err := time.Parse(time.RFC3339Nano, original)
		if err != nil {
			return "", false
		}
		_, offset := parsed.Zone()
		text = at.In(time.FixedZone("", offset)).Format(layout + "-07:00")
	}
	return text, len(text) == len(original)
}

var (
	codexDayDir  = regexp.MustCompile(`^(\.codex/sessions/)(\d{4})/(\d{2})/(\d{2})/`)
	rolloutStamp = regexp.MustCompile(`rollout-(\d{4}-\d{2}-\d{2})T`)
)

// shiftPath moves the dates in Codex session directories and rollout names.
func shiftPath(rel string, days int) string {
	addDays := func(date string) string {
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			return date
		}
		return parsed.AddDate(0, 0, days).Format("2006-01-02")
	}
	if match := codexDayDir.FindStringSubmatch(rel); match != nil {
		moved := strings.ReplaceAll(addDays(match[2]+"-"+match[3]+"-"+match[4]), "-", "/")
		rel = match[1] + moved + "/" + rel[len(match[0]):]
	}
	return rolloutStamp.ReplaceAllStringFunc(rel, func(m string) string {
		return "rollout-" + addDays(m[len("rollout-"):len(m)-1]) + "T"
	})
}

// shiftFile rewrites one file's time fields. Per-file order protection: if the
// local-calendar shift would reverse any two parseable timestamps, the whole
// file is shifted by days*24h in UTC instead.
func shiftFile(raw []byte, days int, location *time.Location) (out []byte, rewritten, unparsed int, fallback bool, dayMoves int) {
	type stamp struct {
		field
		at time.Time
	}
	lines := bytes.SplitAfter(raw, []byte("\n"))
	stamps := make([][]stamp, len(lines))
	for i, line := range lines {
		for _, f := range timestampFields(line) {
			at, err := time.Parse(time.RFC3339Nano, string(line[f.start:f.end]))
			if err != nil {
				unparsed++
				continue
			}
			stamps[i] = append(stamps[i], stamp{f, at})
		}
	}
	shiftWith := func(local bool) ([]time.Time, bool) {
		var shifted []time.Time
		var previousOriginal, previousShifted time.Time
		ordered := true
		for _, lineStamps := range stamps {
			for _, s := range lineStamps {
				moved := s.at.Add(time.Duration(days) * 24 * time.Hour)
				if local {
					moved = shiftLocal(s.at, days, location)
				}
				if len(shifted) > 0 && !s.at.Before(previousOriginal) && moved.Before(previousShifted) {
					ordered = false
				}
				previousOriginal, previousShifted = s.at, moved
				shifted = append(shifted, moved)
			}
		}
		return shifted, ordered
	}
	shifted, ordered := shiftWith(true)
	if !ordered {
		fallback = true
		shifted, _ = shiftWith(false)
	}
	out = make([]byte, 0, len(raw))
	k := 0
	for i, line := range lines {
		cursor := 0
		for _, s := range stamps[i] {
			original := string(line[s.start:s.end])
			text, ok := formatLike(original, shifted[k])
			if fallback && shifted[k].In(location).Format("2006-01-02") != s.at.In(location).AddDate(0, 0, days).Format("2006-01-02") {
				dayMoves++
			}
			k++
			if !ok {
				unparsed++
				continue
			}
			out = append(out, line[cursor:s.start]...)
			out = append(out, text...)
			cursor = s.end
			rewritten++
		}
		out = append(out, line[cursor:]...)
	}
	return out, rewritten, unparsed, fallback, dayMoves
}

// shiftCorpus writes the working copy at out: same files, shifted times,
// shifted Codex path dates and shifted mtimes.
func shiftCorpus(corpus, out string, days int, location *time.Location) (shiftResult, error) {
	if !filepath.IsAbs(out) {
		return shiftResult{}, errors.New("--out must be absolute")
	}
	if _, err := os.Lstat(out); err == nil {
		return shiftResult{}, errors.New("--out already exists; the batch removes the previous working copy first")
	}
	result := shiftResult{Timezone: location.String(), Days: days}
	var lastRecord time.Time
	err := filepath.WalkDir(corpus, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return walkErr
		}
		rel, err := filepath.Rel(corpus, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		shifted, rewritten, unparsed, fallback, moves := shiftFile(raw, days, location)
		for _, line := range bytes.Split(shifted, []byte("\n")) {
			for _, f := range timestampFields(line) {
				if at, err := time.Parse(time.RFC3339Nano, string(line[f.start:f.end])); err == nil && at.After(lastRecord) {
					lastRecord = at
				}
			}
		}
		target := filepath.Join(out, filepath.FromSlash(shiftPath(filepath.ToSlash(rel), days)))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(target, shifted, 0o600); err != nil {
			return err
		}
		mtime := shiftLocal(info.ModTime(), days, location)
		if err := os.Chtimes(target, mtime, mtime); err != nil {
			return err
		}
		result.Files++
		result.Rewritten += rewritten
		result.Unparsed += unparsed
		if fallback {
			result.FallbackFiles++
			result.FallbackDayMove += moves
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	if err := os.WriteFile(filepath.Join(out, ".metadata_never_index"), nil, 0o600); err != nil {
		return result, err
	}
	if result.Identity, err = fixtureIdentity(out); err != nil {
		return result, err
	}
	result.LastRecord = lastRecord.In(location).Format(time.RFC3339)
	return result, nil
}
