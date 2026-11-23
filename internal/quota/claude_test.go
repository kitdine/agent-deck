package quota

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	// Codex PR #5 second review, P2: both windows previously left VendorOrder
	// at its zero value, so Store.Windows' vendor_order ordering could not
	// tell session from week once both had been written.
	if session.VendorOrder != 0 {
		t.Fatalf("session.VendorOrder = %d, want 0", session.VendorOrder)
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
	if week.VendorOrder != 1 {
		t.Fatalf("week.VendorOrder = %d, want 1", week.VendorOrder)
	}
	if week.WindowMinutes != 10080 || week.WindowMinutesReason != "" {
		t.Fatalf("week.WindowMinutes = (%d, %q), want (10080, \"\")", week.WindowMinutes, week.WindowMinutesReason)
	}
	wantWeekReset := time.Date(2026, 9, 15, 20, 0, 0, 0, loc).UTC()
	if !week.ResetsAt.Equal(wantWeekReset) {
		t.Fatalf("week.ResetsAt = %v, want %v", week.ResetsAt, wantWeekReset)
	}
}

const claudeSonnetWeekLine = "Current week (Sonnet): 99% used · resets Sep 18 at 6pm (UTC)\n"

func TestParseClaudeProseDiscardsContributingSection(t *testing.T) {
	observedAt := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	want, err := ParseClaudeProse(claudeProseSample, observedAt)
	if err != nil {
		t.Fatalf("ParseClaudeProse baseline: %v", err)
	}
	got, err := ParseClaudeProse(claudeProseSample+claudeSonnetWeekLine, observedAt)
	if err != nil {
		t.Fatalf("ParseClaudeProse with model-only week: %v", err)
	}
	if len(got.Windows) != 2 || !reflect.DeepEqual(got.Windows, want.Windows) {
		t.Fatalf("windows changed with a model-only week: got %+v, want %+v", got.Windows, want.Windows)
	}
}

func TestParseClaudeProseAllOrNothingOnMissingLine(t *testing.T) {
	onlySession := "Current session: 22% used · resets Sep 9 at 1:50am (America/Los_Angeles)\n"
	for _, tc := range []struct {
		name string
		week string
	}{
		{name: "no weekly line"},
		{name: "model-only weekly line", week: claudeSonnetWeekLine},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observedAt := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			result, err := ParseClaudeProse(onlySession+tc.week, observedAt)
			var parseErr *ProseParseError
			if !errors.As(err, &parseErr) || parseErr.Detail != DetailMissingLine || len(result.Windows) != 0 {
				t.Fatalf("without an all-models week: windows=%+v, error=%v; want no windows and missing_line", result.Windows, err)
			}
		})
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

// --- Executable discovery under a GUI/helper-restricted PATH.
//
// An app-launched helper gets a fixed PATH (Homebrew, /usr/local, /usr, /bin)
// that excludes the native installer's ~/.local/bin, so the bare name
// "claude" cannot be resolved there. These tests reproduce that environment
// with an isolated HOME and PATH and fake commands only: no real claude
// binary, real home directory, or manual symlink is consulted.

const fakeClaudeUsageScript = "#!/bin/sh\n" +
	`printf 'Current session: 22%% used \xc2\xb7 resets Sep 9 at 1:50am (America/Los_Angeles)\nCurrent week (all models): 3%% used \xc2\xb7 resets Sep 15 at 8pm (America/Los_Angeles)\n'` + "\n"

// isolateClaudeDiscovery points HOME at an empty temp home, PATH at an empty
// directory (the restricted-helper shape), and the probe at the bare name
// "claude". It returns the home and the PATH directory.
func isolateClaudeDiscovery(t *testing.T) (home, pathDir string) {
	t.Helper()
	home = t.TempDir()
	pathDir = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", pathDir)
	previous := claudeProseCommandArgs
	claudeProseCommandArgs = []string{"claude", "-p", "/usage"}
	t.Cleanup(func() { claudeProseCommandArgs = previous })
	return home, pathDir
}

func writeFakeClaude(t *testing.T, dir, script string, mode os.FileMode) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte(script), mode); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// WriteFile is subject to umask; pin the exact mode under test.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	return path
}

func requireClaudeFailureKind(t *testing.T, err error, want ClaudeFailureKind) {
	t.Helper()
	var failure *ClaudeFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *ClaudeFailureError", err)
	}
	if failure.Kind != want || failure.Kind.Reason() != ReasonProbeFailed {
		t.Fatalf("Kind = %v, want %v mapping to probe_failed", failure.Kind, want)
	}
}

func TestProbeClaudeProseFindsNativeInstallUnderRestrictedPATH(t *testing.T) {
	home, _ := isolateClaudeDiscovery(t)
	writeFakeClaude(t, filepath.Join(home, ".local", "bin"), fakeClaudeUsageScript, 0o755)

	result, err := ProbeClaudeProse(context.Background(), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), 5*time.Second)
	if err != nil {
		t.Fatalf("ProbeClaudeProse with claude only in $HOME/.local/bin: %v", err)
	}
	if len(result.Windows) != 2 {
		t.Fatalf("Windows = %+v, want 2", result.Windows)
	}
}

func TestProbeClaudeProseFollowsNativeInstallSymlink(t *testing.T) {
	// The native installer links ~/.local/bin/claude to a versioned binary.
	home, _ := isolateClaudeDiscovery(t)
	versions := filepath.Join(home, ".local", "share", "claude", "versions")
	target := writeFakeClaude(t, versions, fakeClaudeUsageScript, 0o755)
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(binDir, "claude")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	if _, err := ProbeClaudeProse(context.Background(), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), 5*time.Second); err != nil {
		t.Fatalf("ProbeClaudeProse through installer symlink: %v", err)
	}
}

func TestProbeClaudeProsePATHWinsOverHomeFallback(t *testing.T) {
	home, pathDir := isolateClaudeDiscovery(t)
	writeFakeClaude(t, pathDir, fakeClaudeUsageScript, 0o755)
	// A failing home candidate proves it was not the one executed.
	writeFakeClaude(t, filepath.Join(home, ".local", "bin"), "#!/bin/sh\nexit 1\n", 0o755)

	if _, err := ProbeClaudeProse(context.Background(), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), 5*time.Second); err != nil {
		t.Fatalf("PATH candidate should win over the home fallback: %v", err)
	}
}

func TestProbeClaudeProseExplicitCommandWinsOverHomeFallback(t *testing.T) {
	home, _ := isolateClaudeDiscovery(t)
	writeFakeClaude(t, filepath.Join(home, ".local", "bin"), fakeClaudeUsageScript, 0o755)
	explicit := writeFakeClaude(t, t.TempDir(), "#!/bin/sh\nexit 1\n", 0o755)
	claudeProseCommandArgs = []string{explicit, "-p", "/usage"}

	_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
	requireClaudeFailureKind(t, err, ClaudeFailureExit)
}

func TestProbeClaudeProseExplicitMissingCommandDoesNotFallBack(t *testing.T) {
	home, _ := isolateClaudeDiscovery(t)
	writeFakeClaude(t, filepath.Join(home, ".local", "bin"), fakeClaudeUsageScript, 0o755)
	claudeProseCommandArgs = []string{filepath.Join(t.TempDir(), "missing"), "-p", "/usage"}

	_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
	requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
}

func TestProbeClaudeProseMissingEverywhereIsBoundedSpawnFailure(t *testing.T) {
	isolateClaudeDiscovery(t)

	_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
	requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
}

func TestProbeClaudeProseRejectsUntrustedHomeCandidates(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, binDir string)
	}{
		{"not executable", func(t *testing.T, binDir string) {
			writeFakeClaude(t, binDir, fakeClaudeUsageScript, 0o644)
		}},
		{"group writable", func(t *testing.T, binDir string) {
			writeFakeClaude(t, binDir, fakeClaudeUsageScript, 0o775)
		}},
		{"world writable", func(t *testing.T, binDir string) {
			writeFakeClaude(t, binDir, fakeClaudeUsageScript, 0o757)
		}},
		{"directory", func(t *testing.T, binDir string) {
			if err := os.MkdirAll(filepath.Join(binDir, "claude"), 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
		}},
		{"dangling symlink", func(t *testing.T, binDir string) {
			if err := os.MkdirAll(binDir, 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), filepath.Join(binDir, "claude")); err != nil {
				t.Fatalf("Symlink: %v", err)
			}
		}},
		{"writable install directory", func(t *testing.T, binDir string) {
			writeFakeClaude(t, binDir, fakeClaudeUsageScript, 0o755)
			if err := os.Chmod(binDir, 0o777); err != nil {
				t.Fatalf("Chmod: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := isolateClaudeDiscovery(t)
			tc.setup(t, filepath.Join(home, ".local", "bin"))

			_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
			requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
		})
	}
}

func TestProbeClaudeProseIgnoresRelativePATHEntriesAndWorkingDirectory(t *testing.T) {
	isolateClaudeDiscovery(t)
	cwd := t.TempDir()
	writeFakeClaude(t, cwd, fakeClaudeUsageScript, 0o755)
	t.Chdir(cwd)
	t.Setenv("PATH", ".:"+os.Getenv("PATH"))

	_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
	requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
}

// CQP-R1-F2: only a genuine not-found may fall back to the home install.
// exec.ErrDot (the command exists only relative to the working directory)
// must reach the caller unchanged instead of being replaced by running a
// different, valid executable from $HOME.
func TestProbeClaudeProseErrDotIsNotBypassedByValidHomeFallback(t *testing.T) {
	home, _ := isolateClaudeDiscovery(t)
	writeFakeClaude(t, filepath.Join(home, ".local", "bin"), fakeClaudeUsageScript, 0o755)
	cwd := t.TempDir()
	writeFakeClaude(t, cwd, "#!/bin/sh\nexit 1\n", 0o755)
	t.Chdir(cwd)
	t.Setenv("PATH", ".:"+os.Getenv("PATH"))

	_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
	requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
	if !errors.Is(err, exec.ErrDot) {
		t.Fatalf("err = %v, want it to preserve exec.ErrDot", err)
	}
}

// CQP-R1-F1: every ancestor the fallback depends on must be unreplaceable by
// other users, not just the leaf and its install directory.
func TestProbeClaudeProseRejectsReplaceableAncestors(t *testing.T) {
	chmod := func(t *testing.T, path string, mode os.FileMode) {
		t.Helper()
		if err := os.Chmod(path, mode); err != nil {
			t.Fatalf("Chmod %s: %v", path, err)
		}
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, home string)
	}{
		{"world-writable .local ancestor", func(t *testing.T, home string) {
			writeFakeClaude(t, filepath.Join(home, ".local", "bin"), fakeClaudeUsageScript, 0o755)
			chmod(t, filepath.Join(home, ".local"), 0o777)
		}},
		{"world-writable home", func(t *testing.T, home string) {
			writeFakeClaude(t, filepath.Join(home, ".local", "bin"), fakeClaudeUsageScript, 0o755)
			chmod(t, home, 0o777)
		}},
		{"group-writable .local ancestor", func(t *testing.T, home string) {
			writeFakeClaude(t, filepath.Join(home, ".local", "bin"), fakeClaudeUsageScript, 0o755)
			chmod(t, filepath.Join(home, ".local"), 0o775)
		}},
		{"symlink target parent writable", func(t *testing.T, home string) {
			mid := t.TempDir()
			target := writeFakeClaude(t, filepath.Join(mid, "versions"), fakeClaudeUsageScript, 0o755)
			chmod(t, mid, 0o777)
			binDir := filepath.Join(home, ".local", "bin")
			if err := os.MkdirAll(binDir, 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if err := os.Symlink(target, filepath.Join(binDir, "claude")); err != nil {
				t.Fatalf("Symlink: %v", err)
			}
		}},
		{"symlinked .local whose target parent is writable", func(t *testing.T, home string) {
			mid := t.TempDir()
			writeFakeClaude(t, filepath.Join(mid, "real-local", "bin"), fakeClaudeUsageScript, 0o755)
			chmod(t, mid, 0o777)
			if err := os.Symlink(filepath.Join(mid, "real-local"), filepath.Join(home, ".local")); err != nil {
				t.Fatalf("Symlink: %v", err)
			}
		}},
		{"intermediate symlink hop in a writable directory", func(t *testing.T, home string) {
			versions := writeFakeClaude(t, t.TempDir(), fakeClaudeUsageScript, 0o755)
			hopDir := t.TempDir()
			chmod(t, hopDir, 0o777)
			hop := filepath.Join(hopDir, "hop")
			if err := os.Symlink(versions, hop); err != nil {
				t.Fatalf("Symlink: %v", err)
			}
			binDir := filepath.Join(home, ".local", "bin")
			if err := os.MkdirAll(binDir, 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if err := os.Symlink(hop, filepath.Join(binDir, "claude")); err != nil {
				t.Fatalf("Symlink: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := isolateClaudeDiscovery(t)
			tc.setup(t, home)

			_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
			requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
		})
	}
}

// A sticky world-writable ancestor (the /tmp boundary) is a deliberate,
// bounded exception: other users cannot rename or remove entries the current
// user owns, so a user-owned chain beneath it stays trusted.
func TestProbeClaudeProseTrustsOwnedChainBelowStickyAncestor(t *testing.T) {
	base := t.TempDir()
	if err := os.Chmod(base, 0o777|os.ModeSticky); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	home := filepath.Join(base, "home")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	previous := claudeProseCommandArgs
	claudeProseCommandArgs = []string{"claude", "-p", "/usage"}
	t.Cleanup(func() { claudeProseCommandArgs = previous })
	writeFakeClaude(t, filepath.Join(home, ".local", "bin"), fakeClaudeUsageScript, 0o755)

	if _, err := ProbeClaudeProse(context.Background(), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), 5*time.Second); err != nil {
		t.Fatalf("owned chain below a sticky ancestor should be trusted: %v", err)
	}
}

func TestProbeClaudeProseHomeCandidateStaysDeadlineBounded(t *testing.T) {
	home, _ := isolateClaudeDiscovery(t)
	writeFakeClaude(t, filepath.Join(home, ".local", "bin"), "#!/bin/sh\nwhile :; do :; done\n", 0o755)

	_, err := ProbeClaudeProse(context.Background(), time.Now(), 200*time.Millisecond)
	requireClaudeFailureKind(t, err, ClaudeFailureDeadline)
}

// --- Kernel-order resolution (CQP-R2-F1).
//
// A symlink target such as "safe/alias/../claude" means something different
// to the kernel than to filepath.Clean: the kernel resolves "alias" first and
// applies ".." to wherever alias really points. Trust must be judged on the
// object the kernel would actually reach, and that same object executed.

const exitNineteenScript = "#!/bin/sh\nexit 19\n"

func mkdirMode(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, mode); err != nil {
		t.Fatalf("MkdirAll %s: %v", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("Chmod %s: %v", path, err)
	}
}

func symlinkOrFatal(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink %s -> %s: %v", link, target, err)
	}
}

func realPathOrFatal(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks %s: %v", path, err)
	}
	return real
}

// Exact CQP-R2-F1 counterexample: lexical cleaning sees the trusted
// safe/claude sentinel, while the kernel reaches a world-writable binary in a
// world-writable directory. It must be refused, not executed.
func TestProbeClaudeProseSymlinkDotDotRejectsUntrustedKernelTarget(t *testing.T) {
	home, _ := isolateClaudeDiscovery(t)
	safe := t.TempDir()
	sentinel := writeFakeClaude(t, safe, exitNineteenScript, 0o755)
	untrusted := t.TempDir()
	mkdirMode(t, filepath.Join(untrusted, "subdir"), 0o755)
	untrustedClaude := writeFakeClaude(t, untrusted, fakeClaudeUsageScript, 0o777)
	mkdirMode(t, untrusted, 0o777)
	symlinkOrFatal(t, filepath.Join(untrusted, "subdir"), filepath.Join(safe, "alias"))
	rawTarget := safe + "/alias/../claude"
	entry := filepath.Join(home, ".local", "bin", "claude")
	symlinkOrFatal(t, rawTarget, entry)

	// Preconditions proving the scenario is the one under test.
	if got := filepath.Clean(rawTarget); got != sentinel {
		t.Fatalf("filepath.Clean(rawTarget) = %s, want lexical sentinel %s", got, sentinel)
	}
	if got, want := realPathOrFatal(t, entry), realPathOrFatal(t, untrustedClaude); got != want {
		t.Fatalf("kernel resolution of entry = %s, want untrusted target %s", got, want)
	}
	info, err := os.Lstat(untrustedClaude)
	if err != nil || info.Mode().Perm() != 0o777 {
		t.Fatalf("untrusted target mode = %v (err %v), want 0777", info, err)
	}

	_, err = ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
	requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
}

// The mirror case: the kernel target is trusted, the lexical one is a
// failing sentinel. Success proves the executed object is the kernel one,
// so normal links through directory aliases and ".." keep working.
func TestProbeClaudeProseSymlinkDotDotRunsTrustedKernelTarget(t *testing.T) {
	home, _ := isolateClaudeDiscovery(t)
	safe := t.TempDir()
	writeFakeClaude(t, safe, exitNineteenScript, 0o755)
	other := t.TempDir()
	mkdirMode(t, filepath.Join(other, "subdir"), 0o755)
	writeFakeClaude(t, other, fakeClaudeUsageScript, 0o755)
	symlinkOrFatal(t, filepath.Join(other, "subdir"), filepath.Join(safe, "alias"))
	symlinkOrFatal(t, safe+"/alias/../claude", filepath.Join(home, ".local", "bin", "claude"))

	result, err := ProbeClaudeProse(context.Background(), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), 5*time.Second)
	if err != nil {
		t.Fatalf("ProbeClaudeProse should run the trusted kernel target, not the lexical sentinel: %v", err)
	}
	if len(result.Windows) != 2 {
		t.Fatalf("Windows = %+v, want 2", result.Windows)
	}
}

// $HOME itself is interpreted the way a shell's "$HOME/.local/bin/claude"
// would be: a symlinked component followed by ".." is resolved, not cleaned.
func TestProbeClaudeProseHomeWithDotDotFollowsKernelOrder(t *testing.T) {
	isolateClaudeDiscovery(t)
	safe := t.TempDir()
	writeFakeClaude(t, filepath.Join(safe, ".local", "bin"), exitNineteenScript, 0o755)
	other := t.TempDir()
	mkdirMode(t, filepath.Join(other, "subdir"), 0o755)
	writeFakeClaude(t, filepath.Join(other, ".local", "bin"), fakeClaudeUsageScript, 0o755)
	symlinkOrFatal(t, filepath.Join(other, "subdir"), filepath.Join(safe, "alias"))
	t.Setenv("HOME", safe+"/alias/..")

	if _, err := ProbeClaudeProse(context.Background(), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), 5*time.Second); err != nil {
		t.Fatalf("HOME with alias/.. should resolve to the kernel location: %v", err)
	}
}

// Normal installer layouts with relative links, ".." and a symlinked install
// directory stay supported.
func TestProbeClaudeProseTrustsNormalRelativeLinks(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, home string)
	}{
		{"relative versioned link with dotdot", func(t *testing.T, home string) {
			writeFakeClaude(t, filepath.Join(home, ".local", "share", "claude", "versions", "2.0.0"), fakeClaudeUsageScript, 0o755)
			symlinkOrFatal(t, "../share/claude/versions/2.0.0/claude", filepath.Join(home, ".local", "bin", "claude"))
		}},
		{"relative symlinked install directory", func(t *testing.T, home string) {
			writeFakeClaude(t, filepath.Join(home, "dot-local", "bin"), fakeClaudeUsageScript, 0o755)
			symlinkOrFatal(t, "dot-local", filepath.Join(home, ".local"))
		}},
		{"two-hop chain", func(t *testing.T, home string) {
			target := writeFakeClaude(t, filepath.Join(home, "versions"), fakeClaudeUsageScript, 0o755)
			symlinkOrFatal(t, target, filepath.Join(home, "current"))
			symlinkOrFatal(t, "../../current", filepath.Join(home, ".local", "bin", "claude"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := isolateClaudeDiscovery(t)
			tc.setup(t, home)

			if _, err := ProbeClaudeProse(context.Background(), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), 5*time.Second); err != nil {
				t.Fatalf("ProbeClaudeProse: %v", err)
			}
		})
	}
}

// The fallback executes the exact symlink-free path it verified, so nothing
// is left for a later, separate resolution to reinterpret.
func TestResolveClaudeExecutableReturnsVerifiedRealPath(t *testing.T) {
	home, _ := isolateClaudeDiscovery(t)
	target := writeFakeClaude(t, filepath.Join(home, ".local", "share", "claude", "versions"), fakeClaudeUsageScript, 0o755)
	entry := filepath.Join(home, ".local", "bin", "claude")
	symlinkOrFatal(t, target, entry)

	got, err := resolveClaudeExecutable("claude")
	if err != nil {
		t.Fatalf("resolveClaudeExecutable: %v", err)
	}
	if want := realPathOrFatal(t, entry); got != want {
		t.Fatalf("resolved = %s, want the kernel real path %s", got, want)
	}
	if again := realPathOrFatal(t, got); again != got {
		t.Fatalf("resolved path %s still contains symlinks (real %s)", got, again)
	}
}

func TestProbeClaudeProseRejectsSymlinkLoopsAndOverlongChains(t *testing.T) {
	t.Run("loop", func(t *testing.T) {
		home, _ := isolateClaudeDiscovery(t)
		binDir := filepath.Join(home, ".local", "bin")
		symlinkOrFatal(t, "other", filepath.Join(binDir, "claude"))
		symlinkOrFatal(t, "claude", filepath.Join(binDir, "other"))

		_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
		requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
	})
	t.Run("longer than the hop limit", func(t *testing.T) {
		home, _ := isolateClaudeDiscovery(t)
		binDir := filepath.Join(home, ".local", "bin")
		writeFakeClaude(t, filepath.Join(home, "real"), fakeClaudeUsageScript, 0o755)
		previous := filepath.Join(home, "real", "claude")
		for i := 0; i <= maxClaudeSymlinkHops; i++ {
			link := filepath.Join(binDir, "hop"+string(rune('a'+i)))
			symlinkOrFatal(t, previous, link)
			previous = link
		}
		symlinkOrFatal(t, previous, filepath.Join(binDir, "claude"))

		_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
		requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
	})
}
