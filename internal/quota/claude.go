package quota

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultClaudeProseTimeout bounds the `claude -p "/usage"` probe.
const DefaultClaudeProseTimeout = 10 * time.Second

// claudeProseCommandArgs names the executable and arguments ProbeClaudeProse
// runs. Package-level so tests can substitute a fake process (CLA-R1-F4) —
// no test in this package points it at the real "claude" binary.
var claudeProseCommandArgs = []string{"claude", "-p", "/usage"}

// ClaudeProseResult is `claude -p "/usage"` mapped onto the domain (C4).
type ClaudeProseResult struct {
	Windows []Observation
}

// ClaudeFailureKind distinguishes why a Claude prose probe did not produce a
// result, mirroring CodexFailureKind's classification for C2/C4/C6's closed
// reason set: spawn failure, non-zero exit, and deadline are all
// probe_failed; a shape change ParseClaudeProse rejects is parse_failed —
// distinct outcomes a caller must be able to tell apart (C6), not one
// generic wrapped error a caller can only classify by matching text.
type ClaudeFailureKind int

const (
	ClaudeFailureSpawn ClaudeFailureKind = iota + 1
	ClaudeFailureExit
	ClaudeFailureDeadline
	ClaudeFailureParse
)

func (k ClaudeFailureKind) String() string {
	switch k {
	case ClaudeFailureSpawn:
		return "spawn"
	case ClaudeFailureExit:
		return "exit"
	case ClaudeFailureDeadline:
		return "deadline"
	case ClaudeFailureParse:
		return "parse"
	default:
		return "unknown"
	}
}

// Reason maps a failure kind to C6's closed reason set. A parse failure
// (C4's all-or-nothing shape check) is parse_failed; every other kind — the
// process never started, exited before producing output, or the deadline
// expired — is probe_failed.
func (k ClaudeFailureKind) Reason() Reason {
	if k == ClaudeFailureParse {
		return ReasonParseFailed
	}
	return ReasonProbeFailed
}

// ClaudeFailureError reports a classified Claude prose probe failure. Err is
// the underlying cause and remains matchable via errors.Unwrap.
type ClaudeFailureError struct {
	Kind ClaudeFailureKind
	Err  error
}

func (e *ClaudeFailureError) Error() string {
	return fmt.Sprintf("claude prose probe failed (%s): %v", e.Kind, e.Err)
}

func (e *ClaudeFailureError) Unwrap() error { return e.Err }

// ProbeClaudeProse drives `claude -p "/usage"` as a child process and parses
// its stdout (C4). --output-format json adds no structured keys — the same
// prose arrives in "result" — so this reads plain stdout rather than adding
// unnecessary JSON handling.
//
// Start and Wait are classified separately, matching codex.go's ProbeCodex:
// a Start failure is unambiguously a spawn failure (the process never ran at
// all), while a Wait failure is an ordinary exit unless the context deadline
// had already expired. This avoids sniffing cmd.Output()'s combined error by
// type, which does not reliably distinguish "executable not found via
// LookPath" (*exec.Error) from "executable not found at an absolute path"
// (a plain fs.ErrNotExist-wrapping error) — both are spawn failures, but
// only the former satisfies errors.As(err, *exec.Error).
func ProbeClaudeProse(ctx context.Context, observedAt time.Time, timeout time.Duration) (ClaudeProseResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, claudeProseCommandArgs[0], claudeProseCommandArgs[1:]...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Start(); err != nil {
		return ClaudeProseResult{}, &ClaudeFailureError{Kind: ClaudeFailureSpawn, Err: err}
	}
	if err := cmd.Wait(); err != nil {
		kind := ClaudeFailureExit
		if ctx.Err() != nil {
			kind = ClaudeFailureDeadline
		}
		return ClaudeProseResult{}, &ClaudeFailureError{Kind: kind, Err: err}
	}
	result, parseErr := ParseClaudeProse(stdout.String(), observedAt)
	if parseErr != nil {
		return ClaudeProseResult{}, &ClaudeFailureError{Kind: ClaudeFailureParse, Err: parseErr}
	}
	return result, nil
}

// claudeProseLinePattern matches one of the two known lines, e.g.:
//
//	Current session: 22% used · resets Sep 9 at 1:50am (America/Los_Angeles)
//	Current week (all models): 3% used · resets Sep 15 at 8pm (America/Los_Angeles)
//
// The parser is strict (C4): a shape change is a reported parse failure,
// never degraded to a looser pattern that would match future copy by
// accident with no signal that the source changed.
var claudeProseLinePattern = regexp.MustCompile(
	`^(Current session|Current week \(all models\)): (\d+(?:\.\d+)?)% used · resets ([A-Za-z]+ \d{1,2}) at (\d{1,2}(?::\d{2})?(?:am|pm)) \(([^)]+)\)$`,
)

// ParseClaudeProse extracts the two known lines from `claude -p "/usage"`
// output (C4). All-or-nothing: if either line is absent or fails to match,
// the whole result is an error — a surface showing one window while
// silently dropping the other is worse than reporting that parsing failed.
// Every other line — including the "what's contributing" section, which
// states on its face that it is a local derivation AgentDeck already
// computes itself — is discarded.
func ParseClaudeProse(output string, observedAt time.Time) (ClaudeProseResult, error) {
	var sessionLine, weekLine string
	var haveSession, haveWeek bool
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "Current session:"):
			sessionLine, haveSession = line, true
		case strings.HasPrefix(line, "Current week (all models):"):
			weekLine, haveWeek = line, true
		}
	}
	if !haveSession || !haveWeek {
		return ClaudeProseResult{}, fmt.Errorf("expected both usage lines in claude -p /usage output, got session=%v week=%v", haveSession, haveWeek)
	}

	session, err := parseClaudeProseLine(sessionLine, observedAt)
	if err != nil {
		return ClaudeProseResult{}, fmt.Errorf("session line: %w", err)
	}
	week, err := parseClaudeProseLine(weekLine, observedAt)
	if err != nil {
		return ClaudeProseResult{}, fmt.Errorf("week line: %w", err)
	}

	return ClaudeProseResult{Windows: []Observation{
		mapClaudeProseWindow(session, observedAt, 0),
		mapClaudeProseWindow(week, observedAt, 1),
	}}, nil
}

type claudeProseLine struct {
	windowKey   string
	usedPercent float64
	resetsAt    time.Time
}

func parseClaudeProseLine(line string, now time.Time) (claudeProseLine, error) {
	matches := claudeProseLinePattern.FindStringSubmatch(line)
	if matches == nil {
		return claudeProseLine{}, fmt.Errorf("line does not match the expected shape: %q", line)
	}
	label, percentText, dateText, timeText, zoneName := matches[1], matches[2], matches[3], matches[4], matches[5]

	var windowKey string
	switch label {
	case "Current session":
		windowKey = ClaudeWindowFiveHour
	case "Current week (all models)":
		windowKey = ClaudeWindowSevenDay
	default:
		return claudeProseLine{}, fmt.Errorf("unrecognized line label %q", label)
	}

	percent, err := strconv.ParseFloat(percentText, 64)
	if err != nil {
		return claudeProseLine{}, fmt.Errorf("invalid used percent %q: %w", percentText, err)
	}

	location, err := time.LoadLocation(zoneName)
	if err != nil {
		return claudeProseLine{}, fmt.Errorf("unknown time zone %q: %w", zoneName, err)
	}
	resetsAt, err := resolveClaudeProseInstant(dateText, timeText, location, now)
	if err != nil {
		return claudeProseLine{}, err
	}

	return claudeProseLine{windowKey: windowKey, usedPercent: percent, resetsAt: resetsAt}, nil
}

// resolveClaudeProseInstant resolves a year-less "Sep 9 at 1:50am" against
// the named zone (never the local one — the two differ for any user whose
// machine zone is not their account zone, per C4). The prose carries no
// year, so one is inferred from now, in that zone; an instant that would
// otherwise land more than a day in the past is rolled forward a year,
// handling the case where the reset date is observed just before a
// year boundary (e.g. "resets Jan 2" seen in late December).
// claudeProseTimeLayouts covers both observed shapes: minutes present
// ("1:50am") and the on-the-hour form with no minutes ("8pm").
var claudeProseTimeLayouts = []string{"Jan 2 3:04pm 2006", "Jan 2 3pm 2006"}

func resolveClaudeProseInstant(dateText, timeText string, location *time.Location, now time.Time) (time.Time, error) {
	nowInZone := now.In(location)
	candidate := dateText + " " + timeText + " " + strconv.Itoa(nowInZone.Year())
	var parsed time.Time
	var err error
	for _, layout := range claudeProseTimeLayouts {
		parsed, err = time.ParseInLocation(layout, candidate, location)
		if err == nil {
			break
		}
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("unparseable reset instant %q %q: %w", dateText, timeText, err)
	}
	if parsed.Before(nowInZone.Add(-24 * time.Hour)) {
		parsed = parsed.AddDate(1, 0, 0)
	}
	return parsed.UTC(), nil
}

// vendorOrder is a stable, assigned order (0=session/five_hour,
// 1=week/seven_day), not vendor data: the prose route has no vendor order of
// its own, but Store.Windows sorts by vendor_order, and both Claude routes
// must agree on that order so a mixed-source client never shows week before
// session merely because of insertion order (QD-R1-F3's sibling gap on the
// status-line route).
func mapClaudeProseWindow(line claudeProseLine, observedAt time.Time, vendorOrder int) Observation {
	minutes, reason := ClaudeWindowMinutes(line.windowKey)
	return Observation{
		Client: ClientClaude, WindowKey: line.windowKey, Source: SourceClaudeProse,
		ObservedAt: observedAt, WindowMinutes: minutes, WindowMinutesReason: reason,
		VendorOrder: vendorOrder, UsedPercent: line.usedPercent, ResetsAt: line.resetsAt,
	}
}
