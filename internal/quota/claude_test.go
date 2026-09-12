package quota

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const claudeProseSample = `Current session: 22% used · resets Sep 9 at 1:50am (America/Los_Angeles)
Current week (all models): 3% used · resets Sep 15 at 8pm (America/Los_Angeles)

Based on local sessions on this machine:
  - claude-opus-5: 40%
  - claude-sonnet-5: 60%
`

func TestParseClaudeProseFullMapping(t *testing.T) {
	observedAt := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

	result, err := ParseClaudeProse(claudeProseSample, observedAt)
	if err != nil {
		t.Fatalf("ParseClaudeProse: %v", err)
	}
	if len(result.Windows) != 2 {
		t.Fatalf("Windows = %d, want 2: %+v", len(result.Windows), result.Windows)
	}

	session := result.Windows[0]
	if session.WindowKey != ClaudeWindowFiveHour || session.UsedPercent != 22 {
		t.Fatalf("session window = %+v, want WindowKey=five_hour used=22", session)
	}
	if session.WindowMinutes != 300 || session.WindowMinutesReason != "" {
		t.Fatalf("session.WindowMinutes = (%d, %q), want (300, \"\")", session.WindowMinutes, session.WindowMinutesReason)
	}
	if session.Source != SourceClaudeProse || !session.ObservedAt.Equal(observedAt) {
		t.Fatalf("session Source/ObservedAt = %v/%v", session.Source, session.ObservedAt)
	}
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	wantSessionReset := time.Date(2026, 9, 9, 1, 50, 0, 0, loc).UTC()
	if !session.ResetsAt.Equal(wantSessionReset) {
		t.Fatalf("session.ResetsAt = %v, want %v", session.ResetsAt, wantSessionReset)
	}

	week := result.Windows[1]
	if week.WindowKey != ClaudeWindowSevenDay || week.UsedPercent != 3 {
		t.Fatalf("week window = %+v, want WindowKey=seven_day used=3", week)
	}
	if week.WindowMinutes != 10080 || week.WindowMinutesReason != "" {
		t.Fatalf("week.WindowMinutes = (%d, %q), want (10080, \"\")", week.WindowMinutes, week.WindowMinutesReason)
	}
	wantWeekReset := time.Date(2026, 9, 15, 20, 0, 0, 0, loc).UTC()
	if !week.ResetsAt.Equal(wantWeekReset) {
		t.Fatalf("week.ResetsAt = %v, want %v", week.ResetsAt, wantWeekReset)
	}
}

func TestParseClaudeProseDiscardsContributingSection(t *testing.T) {
	// The "what's contributing" lines must not produce windows or otherwise
	// affect the two known lines' parse (C4).
	withNoise := claudeProseSample + "\nCurrent something else: 99% used · resets Jan 1 at 1:00am (UTC)\n"
	result, err := ParseClaudeProse(withNoise, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ParseClaudeProse: %v", err)
	}
	if len(result.Windows) != 2 {
		t.Fatalf("Windows = %d, want exactly 2 (noise must be discarded): %+v", len(result.Windows), result.Windows)
	}
}

func TestParseClaudeProseAllOrNothingOnMissingLine(t *testing.T) {
	onlySession := "Current session: 22% used · resets Sep 9 at 1:50am (America/Los_Angeles)\n"
	if _, err := ParseClaudeProse(onlySession, time.Now()); err == nil {
		t.Fatal("a missing week line must fail the whole result, not emit a partial one")
	}
}

func TestParseClaudeProseAllOrNothingOnShapeChange(t *testing.T) {
	// The session line's shape changed (no "· resets" separator); the whole
	// result must fail even though the week line is well-formed — a partial
	// figure with a silently dropped window is the failure mode C4 forbids.
	changed := "Current session: 22% used, resets Sep 9 at 1:50am (America/Los_Angeles)\n" +
		"Current week (all models): 3% used · resets Sep 15 at 8pm (America/Los_Angeles)\n"
	if _, err := ParseClaudeProse(changed, time.Now()); err == nil {
		t.Fatal("a shape change on one line must fail the whole result")
	}
}

func TestParseClaudeProseUnknownTimeZone(t *testing.T) {
	bad := "Current session: 22% used · resets Sep 9 at 1:50am (Nowhere/Fictional)\n" +
		"Current week (all models): 3% used · resets Sep 15 at 8pm (America/Los_Angeles)\n"
	if _, err := ParseClaudeProse(bad, time.Now()); err == nil {
		t.Fatal("an unrecognized time zone name must be a parse failure")
	}
}

func TestResolveClaudeProseInstantResolvesAgainstNamedZoneNotLocal(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	// "now" is in UTC; the reset instant must be computed in Asia/Tokyo,
	// which is hours ahead — a local-zone bug would shift the result.
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	got, err := resolveClaudeProseInstant("Sep 2", "9:00am", tokyo, now)
	if err != nil {
		t.Fatalf("resolveClaudeProseInstant: %v", err)
	}
	want := time.Date(2026, 9, 2, 9, 0, 0, 0, tokyo)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResolveClaudeProseInstantRollsOverAtYearBoundary(t *testing.T) {
	loc := time.UTC
	// "now" is late December; a year-less "Jan 2" must resolve to next
	// year, not a Jan 2 that already passed months ago.
	now := time.Date(2026, 12, 30, 0, 0, 0, 0, loc)
	got, err := resolveClaudeProseInstant("Jan 2", "3:04pm", loc, now)
	if err != nil {
		t.Fatalf("resolveClaudeProseInstant: %v", err)
	}
	if got.Year() != 2027 {
		t.Fatalf("got %v, want year 2027", got)
	}
}

func TestParseClaudeProseFractionalPercent(t *testing.T) {
	sample := "Current session: 22.4% used · resets Sep 9 at 1:50am (America/Los_Angeles)\n" +
		"Current week (all models): 3% used · resets Sep 15 at 8pm (America/Los_Angeles)\n"
	result, err := ParseClaudeProse(sample, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ParseClaudeProse: %v", err)
	}
	if result.Windows[0].UsedPercent != 22.4 {
		t.Fatalf("UsedPercent = %v, want 22.4", result.Windows[0].UsedPercent)
	}
}

func TestParseClaudeProseEmptyOutput(t *testing.T) {
	if _, err := ParseClaudeProse("", time.Now()); err == nil {
		t.Fatal("empty output must be a parse failure, not zero windows silently")
	}
}

func TestClaudeProseLinePatternRejectsMalformedPercent(t *testing.T) {
	bad := "Current session: abc% used · resets Sep 9 at 1:50am (America/Los_Angeles)\n" +
		"Current week (all models): 3% used · resets Sep 15 at 8pm (America/Los_Angeles)\n"
	if _, err := ParseClaudeProse(bad, time.Now()); err == nil {
		t.Fatal("a non-numeric percent must fail to match the line pattern")
	}
}

func TestClaudeFailureKindReason(t *testing.T) {
	cases := []struct {
		kind ClaudeFailureKind
		want Reason
	}{
		{ClaudeFailureSpawn, ReasonProbeFailed},
		{ClaudeFailureExit, ReasonProbeFailed},
		{ClaudeFailureDeadline, ReasonProbeFailed},
		{ClaudeFailureParse, ReasonParseFailed},
	}
	for _, c := range cases {
		if got := c.kind.Reason(); got != c.want {
			t.Errorf("%v.Reason() = %q, want %q", c.kind, got, c.want)
		}
	}
}

func TestClaudeFailureErrorUnwraps(t *testing.T) {
	cause := os.ErrNotExist
	err := &ClaudeFailureError{Kind: ClaudeFailureSpawn, Err: cause}
	if got := err.Unwrap(); got != cause {
		t.Fatalf("Unwrap() = %v, want %v", got, cause)
	}
}

// --- ProbeClaudeProse transport tests, against an injected fake process.
// claudeProseCommandArgs never points at the real "claude" binary in this
// file (CLA-R1-F4).

func withFakeClaude(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-claude.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	previous := claudeProseCommandArgs
	claudeProseCommandArgs = []string{"/bin/sh", path}
	t.Cleanup(func() { claudeProseCommandArgs = previous })
}

func TestProbeClaudeProseNonZeroExitIsProbeFailed(t *testing.T) {
	withFakeClaude(t, `exit 1`)
	_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
	var failure *ClaudeFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *ClaudeFailureError", err)
	}
	if failure.Kind != ClaudeFailureExit || failure.Kind.Reason() != ReasonProbeFailed {
		t.Fatalf("Kind = %v, want exit mapping to probe_failed", failure.Kind)
	}
}

func TestProbeClaudeProseShapeChangeIsParseFailed(t *testing.T) {
	withFakeClaude(t, `printf 'the output format changed completely\n'`)
	_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
	var failure *ClaudeFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *ClaudeFailureError", err)
	}
	if failure.Kind != ClaudeFailureParse || failure.Kind.Reason() != ReasonParseFailed {
		t.Fatalf("Kind = %v, want parse mapping to parse_failed — distinguishable from a probe_failed exit (CLA-R1-F4)", failure.Kind)
	}
}

func TestProbeClaudeProseDeadlineIsProbeFailed(t *testing.T) {
	// A busy-wait loop, not `sleep`, so SIGKILL against this shell's own PID
	// reliably interrupts it promptly regardless of whether the shell would
	// otherwise exec-replace itself into a single trailing command.
	withFakeClaude(t, `while :; do :; done`)
	_, err := ProbeClaudeProse(context.Background(), time.Now(), 200*time.Millisecond)
	var failure *ClaudeFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *ClaudeFailureError", err)
	}
	if failure.Kind != ClaudeFailureDeadline || failure.Kind.Reason() != ReasonProbeFailed {
		t.Fatalf("Kind = %v, want deadline mapping to probe_failed", failure.Kind)
	}
}

func TestProbeClaudeProseSpawnFailureIsProbeFailed(t *testing.T) {
	previous := claudeProseCommandArgs
	claudeProseCommandArgs = []string{filepath.Join(t.TempDir(), "does-not-exist"), "-p", "/usage"}
	t.Cleanup(func() { claudeProseCommandArgs = previous })

	_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
	var failure *ClaudeFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *ClaudeFailureError", err)
	}
	if failure.Kind != ClaudeFailureSpawn || failure.Kind.Reason() != ReasonProbeFailed {
		t.Fatalf("Kind = %v, want spawn mapping to probe_failed", failure.Kind)
	}
}

func TestProbeClaudeProseSuccess(t *testing.T) {
	withFakeClaude(t, `printf 'Current session: 22%% used \xc2\xb7 resets Sep 9 at 1:50am (America/Los_Angeles)\nCurrent week (all models): 3%% used \xc2\xb7 resets Sep 15 at 8pm (America/Los_Angeles)\n'`)
	result, err := ProbeClaudeProse(context.Background(), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), 5*time.Second)
	if err != nil {
		t.Fatalf("ProbeClaudeProse: %v", err)
	}
	if len(result.Windows) != 2 {
		t.Fatalf("Windows = %+v, want 2", result.Windows)
	}
}
