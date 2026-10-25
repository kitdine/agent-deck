package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newYork(t *testing.T) *time.Location {
	t.Helper()
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	return location
}

func TestTimestampFieldsSelectOnlyContractFields(t *testing.T) {
	line := []byte(`{"timestamp":"2026-10-20T12:00:00.000Z","type":"session_meta","payload":{"id":"s","timestamp":"2026-10-20T11:59:00.000Z","note":"timestamp"},"message":{"timestamp":"2026-01-01T00:00:00Z"}}`)
	got := timestampFields(line)
	if len(got) != 2 {
		t.Fatalf("fields = %d, want top-level and session_meta payload", len(got))
	}
	for _, f := range got {
		if !strings.HasPrefix(string(line[f.start:f.end]), "2026-10-20T1") {
			t.Fatalf("selected %q", line[f.start:f.end])
		}
	}
	other := []byte(`{"timestamp":"2026-10-20T12:00:00.000Z","type":"event_msg","payload":{"timestamp":"2026-10-20T11:59:00.000Z"}}`)
	if got := timestampFields(other); len(got) != 1 {
		t.Fatalf("non-session_meta payload timestamp must not be shifted: %d fields", len(got))
	}
}

func TestShiftLocalResolvesDSTEdgesDeterministically(t *testing.T) {
	ny := newYork(t)
	if got := len(instantsForWall(2026, time.November, 1, 1, 30, 0, 0, ny)); got != 2 {
		t.Fatalf("autumn repeated hour has %d instants", got)
	}
	if got := len(instantsForWall(2026, time.March, 8, 2, 30, 0, 0, ny)); got != 0 {
		t.Fatalf("spring gap has %d instants", got)
	}
	// 02:30 EST on 2026-03-07 shifted one day falls in the spring gap: read
	// with the pre-transition offset, i.e. 03:30 EDT.
	gap := shiftLocal(time.Date(2026, time.March, 7, 7, 30, 0, 0, time.UTC), 1, ny)
	if want := time.Date(2026, time.March, 8, 7, 30, 0, 0, time.UTC); !gap.Equal(want) {
		t.Fatalf("gap shift = %v, want %v", gap.UTC(), want)
	}
	// 01:30 EDT on 2026-10-31 shifted one day is repeated: take the earlier.
	overlap := shiftLocal(time.Date(2026, time.October, 31, 5, 30, 0, 0, time.UTC), 1, ny)
	if want := time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC); !overlap.Equal(want) {
		t.Fatalf("overlap shift = %v, want %v", overlap.UTC(), want)
	}
	// An ordinary shift keeps the local wall clock across an offset change.
	ordinary := shiftLocal(time.Date(2026, time.October, 20, 3, 30, 0, 0, time.UTC), 30, ny)
	if local := ordinary.In(ny); local.Format("2006-01-02 15:04") != "2026-11-18 23:30" {
		t.Fatalf("ordinary shift = %v", local)
	}
}

func codexLine(stamp string) string {
	return `{"timestamp":"` + stamp + `","type":"event_msg","payload":{"type":"agent_message","message":"x"}}` + "\n"
}

func TestShiftFileFallsBackToUTCWhenTheCalendarShiftWouldReorder(t *testing.T) {
	ny := newYork(t)
	for name, tc := range map[string]struct {
		stamps []string
		days   int
	}{
		// 01:50 EDT then 01:10 EST on the autumn transition day.
		"original overlap": {[]string{"2026-11-01T05:50:00.000Z", "2026-11-01T06:10:00.000Z"}, 1},
		// 02:30 and 03:10 EST shifted onto the spring-forward day.
		"gap overshoot": {[]string{"2026-03-07T07:30:00.000Z", "2026-03-07T08:10:00.000Z"}, 1},
	} {
		raw := []byte(codexLine(tc.stamps[0]) + codexLine(tc.stamps[1]))
		out, rewritten, unparsed, fallback, _ := shiftFile(raw, tc.days, ny)
		if !fallback || rewritten != 2 || unparsed != 0 || len(out) != len(raw) {
			t.Fatalf("%s: fallback=%v rewritten=%d unparsed=%d len %d/%d", name, fallback, rewritten, unparsed, len(out), len(raw))
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var times []time.Time
		for _, line := range lines {
			f := timestampFields([]byte(line))[0]
			at, err := time.Parse(time.RFC3339Nano, line[f.start:f.end])
			if err != nil {
				t.Fatal(err)
			}
			times = append(times, at)
		}
		if times[1].Before(times[0]) {
			t.Fatalf("%s: order reversed after shift", name)
		}
		original, _ := time.Parse(time.RFC3339Nano, tc.stamps[0])
		if !times[0].Equal(original.Add(24 * time.Hour)) {
			t.Fatalf("%s: fallback must shift by d*24h in UTC, got %v", name, times[0])
		}
	}
}

func TestShiftFileChangesOnlyTimestampBytes(t *testing.T) {
	ny := newYork(t)
	raw := []byte(`{"parentUuid":null,"sessionId":"s","type":"user","message":{"role":"user","content":"keep 2026-10-20T00:00:00Z text"},"timestamp":"2026-10-20T14:05:06.789Z"}` + "\n" +
		`{"type":"summary","summary":"no time"}` + "\n" + `not json` + "\n")
	out, rewritten, _, fallback, _ := shiftFile(raw, 3, ny)
	if rewritten != 1 || fallback || len(out) != len(raw) {
		t.Fatalf("rewritten=%d fallback=%v len %d/%d", rewritten, fallback, len(out), len(raw))
	}
	if !bytes.Contains(out, []byte(`"timestamp":"2026-10-23T14:05:06.789Z"`)) || !bytes.Contains(out, []byte("keep 2026-10-20T00:00:00Z text")) {
		t.Fatalf("unexpected output:\n%s", out)
	}
	diff := 0
	for i := range raw {
		if raw[i] != out[i] {
			diff++
		}
	}
	if diff > 2 {
		t.Fatalf("%d bytes changed outside the date digits", diff)
	}
}

func TestShiftPathMovesCodexDates(t *testing.T) {
	got := shiftPath(".codex/sessions/2026/10/30/rollout-2026-10-30T23-10-00-abc.jsonl", 3)
	if want := ".codex/sessions/2026/11/02/rollout-2026-11-02T23-10-00-abc.jsonl"; got != want {
		t.Fatalf("path = %s, want %s", got, want)
	}
	if got := shiftPath(".claude/projects/p/s.jsonl", 3); got != ".claude/projects/p/s.jsonl" {
		t.Fatalf("claude path changed: %s", got)
	}
}

// A synthetic HOME stands in for the real one: freeze drops a half-written
// tail, ignores other directories, and two shifts give one identity.
func TestFreezeAndShiftAreDeterministic(t *testing.T) {
	home := t.TempDir()
	write := func(rel, body string, modified time.Time) {
		path := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	last := time.Date(2026, time.October, 22, 20, 0, 0, 0, time.UTC)
	for i := 0; i < minTodayFiles; i++ {
		// The last line is dated the freeze day: the midnight cutoff drops it.
		body := codexLine("2026-10-20T10:00:00.000Z") + codexLine(last.Format("2006-01-02T15:04:05.000Z")) + strings.Repeat("x", minTodayBytes/minTodayFiles) + "\n" + codexLine("2026-10-23T05:00:00.000Z")
		write(filepath.Join(".codex/sessions/2026/10/20", "rollout-2026-10-20T06-00-00-"+string(rune('a'+i%26))+string(rune('a'+i/26))+".jsonl"), body, last)
	}
	write(".codex/sessions/2026/10/23/rollout-2026-10-23T01-00-00-today.jsonl", codexLine("2026-10-23T05:30:00.000Z"), last)
	write(".agentdeck/agentdeck.sqlite3", "never copied", last)
	write(".claude/settings.json", "never copied", last)

	store := filepath.Join(t.TempDir(), "store")
	manifest, err := freezeCorpus(home, store, "America/New_York", time.Date(2026, time.October, 23, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.Representative || manifest.Files != minTodayFiles || manifest.LastDate != "2026-10-22" || manifest.TruncatedBytes != int64((minTodayFiles+1)*len(codexLine("2026-10-23T05:00:00.000Z"))) {
		t.Fatalf("manifest = %+v", manifest)
	}
	corpus := filepath.Join(store, manifest.Identity)
	if info, err := os.Stat(corpus); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("corpus dir: %v %v", info, err)
	}
	if _, err := os.Stat(filepath.Join(corpus, ".agentdeck")); err == nil {
		t.Fatal("freeze copied state outside the three source roots")
	}
	shift := func() shiftResult {
		out := filepath.Join(t.TempDir(), "home")
		result, err := shiftCorpusCommand(corpus, out, "2026-10-25", "America/New_York")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(out, ".codex/sessions/2026/10/23")); err != nil {
			t.Fatalf("path dates not shifted: %v", err)
		}
		return result
	}
	first, second := shift(), shift()
	if first.Identity != second.Identity || first.Days != 3 || first.Rewritten != 2*minTodayFiles {
		t.Fatalf("shift results %+v / %+v", first, second)
	}
	if !strings.HasPrefix(first.LastRecord, "2026-10-25T16:00:00") {
		t.Fatalf("last record = %s", first.LastRecord)
	}
	raw, _ := json.Marshal(first)
	if bytes.Contains(raw, []byte(home)) {
		t.Fatal("shift result leaked a path")
	}
}
