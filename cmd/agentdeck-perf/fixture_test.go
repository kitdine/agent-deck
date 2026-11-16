package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func smallFixture(t *testing.T, seed uint64) (fixtureOptions, fixtureManifest) {
	t.Helper()
	opts, err := parseFixtureOptions("small", seed, "2026-10-22", "America/New_York", filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := generateFixture(opts)
	if err != nil {
		t.Fatal(err)
	}
	return opts, manifest
}

// Identical parameters must give an identical identity; a different seed
// must change placement but not the size sets.
func TestFixtureIsDeterministic(t *testing.T) {
	_, first := smallFixture(t, 7)
	_, second := smallFixture(t, 7)
	if first != second {
		t.Fatalf("same parameters produced different fixtures:\n%+v\n%+v", first, second)
	}
	_, other := smallFixture(t, 8)
	if other.Identity == first.Identity {
		t.Fatal("a different seed produced the same fixture identity")
	}
	if other.TotalBytes != first.TotalBytes || other.TodayBytes != first.TodayBytes || other.Sigma != first.Sigma {
		t.Fatalf("seed changed a size set: %+v vs %+v", other, first)
	}
}

func TestFixtureHonoursProfileTotals(t *testing.T) {
	opts, manifest := smallFixture(t, 7)
	p := profiles["small"]
	if manifest.Files != p.files || manifest.TotalBytes != p.totalBytes || manifest.LargeFiles != len(p.fixed) {
		t.Fatalf("manifest = %+v", manifest)
	}
	plan, err := planSizes(p)
	if err != nil {
		t.Fatal(err)
	}
	specs := layoutFiles(opts, plan)

	midnight := opts.anchor
	var todayFiles, todayBytes, codex, archived, codexFiles, continued, overlong int64
	for _, spec := range specs {
		path := filepath.Join(opts.out, spec.rel)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() != spec.size {
			t.Fatalf("%s size = %d, want %d", spec.rel, info.Size(), spec.size)
		}
		isToday := !info.ModTime().Before(midnight)
		if isToday != spec.today {
			t.Fatalf("%s mtime %v disagrees with today=%v", spec.rel, info.ModTime(), spec.today)
		}
		if spec.today {
			todayFiles++
			todayBytes += spec.size
			if spec.archived {
				t.Fatalf("today file %s is archived", spec.rel)
			}
		}
		if spec.client == "codex" {
			codexFiles++
			if strings.Contains(spec.rel, "archived_sessions") {
				archived++
			}
		}
		if spec.continued {
			continued++
		}
		if spec.overlong {
			overlong++
			if spec.size != 8*mib {
				t.Fatalf("overlong line in a %d byte file", spec.size)
			}
		}
		codex += map[bool]int64{true: 1}[spec.client == "codex"]
	}
	if todayFiles != int64(p.todayFiles) || todayBytes != 790_846 || continued != int64(p.todayContinued) || overlong != 1 {
		t.Fatalf("today=%d bytes=%d continued=%d overlong=%d", todayFiles, todayBytes, continued, overlong)
	}
	if codex != 240 || archived != 36 {
		t.Fatalf("codex=%d archived=%d", codex, archived)
	}
}

// Every line is either a JSON record or one of the inserted malformed lines,
// and continued sessions hold records both before and on the anchor date.
func TestFixtureRecordsParse(t *testing.T) {
	opts, _ := smallFixture(t, 7)
	plan, _ := planSizes(profiles["small"])
	malformed, records, before, on := 0, 0, 0, 0
	for _, spec := range layoutFiles(opts, plan) {
		raw, err := os.ReadFile(filepath.Join(opts.out, spec.rel))
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		scanner.Buffer(nil, 16<<20)
		for scanner.Scan() {
			var record map[string]any
			if json.Unmarshal(scanner.Bytes(), &record) != nil {
				if scanner.Text() != `{"type":"event_msg","payload":{"type":` {
					t.Fatalf("%s: unexpected invalid line %.80q", spec.rel, scanner.Text())
				}
				malformed++
				continue
			}
			records++
			if !spec.continued {
				continue
			}
			stamp, _ := record["timestamp"].(string)
			if at, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
				if at.Before(opts.anchor) {
					before++
				} else {
					on++
				}
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
	}
	if malformed == 0 || records == 0 || before == 0 || on == 0 {
		t.Fatalf("malformed=%d records=%d continued before/on=%d/%d", malformed, records, before, on)
	}
	if rate := float64(malformed) / float64(records); rate > 0.003 {
		t.Fatalf("malformed rate %.4f", rate)
	}
}
