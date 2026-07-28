package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// usePinnedReportingZone fixes the zone usage reports resolve their local
// dates in, for one test.
//
// `usage stats --from/--to` take local dates by design (cli-manual.md: 本机
// 日期), so the window they resolve to depends on where the machine is. Tests
// whose fixtures carry UTC timestamps near the start of a day are otherwise
// silently sensitive to that: on a host west of UTC the local-day window opens
// hours later and drops those events, and the test fails for a reason that has
// nothing to do with what it asserts.
//
// It swaps the reportLocation seam rather than time.Local, matching how this
// package already pins userHomeDir and runClientProcesses. The seam exists
// because there is no other in-process way to pin the zone: TZ is read once,
// the first time the process resolves time.Local, so setting the environment
// variable from inside a test is too late.
func usePinnedReportingZone(t *testing.T, location *time.Location) {
	t.Helper()
	previous := reportLocation
	reportLocation = func() *time.Location { return location }
	t.Cleanup(func() { reportLocation = previous })
}

func useUTCReportingClock(t *testing.T) {
	t.Helper()
	usePinnedReportingZone(t, time.UTC)
}

// TestUsageStatsResolvesRangeDatesInTheMachineZone pins the behavior that made
// the two tests above zone-sensitive, so it stays a deliberate contract rather
// than a trap: --from/--to name local dates, not UTC dates.
//
// The sample instant is what makes this a real guard. 2026-07-20T16:00:00Z is
// 2026-07-21 00:00 in UTC+8 but still 2026-07-20 in UTC, so the two readings
// disagree about which day owns it: only local-date semantics put it in the
// 21st's window and leave the 20th's empty. An instant that landed on the same
// date in both frames — 15:00Z, say — would pass under either reading and
// guard nothing.
func TestUsageStatsResolvesRangeDatesInTheMachineZone(t *testing.T) {
	usePinnedReportingZone(t, time.FixedZone("UTC+8", 8*60*60))

	state := filepath.Join(t.TempDir(), "state")
	if err := run([]string{"--state-dir", state, "state", "migrate"}, bytes.NewReader(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	seedUsageEventForRangeTest(t, state, "2026-07-20T16:00:00Z")

	localDay := usageStatsEventCount(t, state, "2026-07-21", "2026-07-21")
	if localDay != 1 {
		t.Fatalf("events in the local day (2026-07-21 in UTC+8) = %d, want 1", localDay)
	}
	utcDay := usageStatsEventCount(t, state, "2026-07-20", "2026-07-20")
	if utcDay != 0 {
		t.Fatalf("events counted in the UTC day (2026-07-20) = %d, want 0", utcDay)
	}
}

// TestReportLocationDefaultsToTheMachineZone covers the half of the contract
// the test above cannot: that one pins the seam, so it proves dates are read
// in whatever zone is configured, not that the configured zone is the
// machine's. Reports are local by design, so the default is part of the
// contract and not an implementation detail.
func TestReportLocationDefaultsToTheMachineZone(t *testing.T) {
	if reportLocation() != time.Local {
		t.Fatalf("reportLocation() = %v, want the machine zone", reportLocation())
	}
}

func seedUsageEventForRangeTest(t *testing.T, state, at string) {
	t.Helper()
	database, err := store.Open(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec(context.Background(), `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES ('codex','zone-session',?,?);
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,source_path,source_offset) VALUES
 ('zone','codex','zone-session','zone',?,'gpt-test',10,0,'fixture',0)`, at, at, at); err != nil {
		t.Fatal(err)
	}
}

func usageStatsEventCount(t *testing.T, state, from, to string) int64 {
	t.Helper()
	var stdout bytes.Buffer
	if err := run([]string{"--state-dir", state, "--format", "json", "usage", "stats", "--from", from, "--to", to, "--no-scan"}, bytes.NewReader(nil), &stdout); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data struct {
			Totals struct {
				Events int64 `json:"events"`
			} `json:"totals"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stats envelope = %q: %v", stdout.String(), err)
	}
	if strings.Contains(stdout.String(), "\"error\"") {
		t.Fatalf("stats reported an error: %s", stdout.String())
	}
	return envelope.Data.Totals.Events
}
