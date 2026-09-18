package quota

import (
	"testing"
	"time"
)

func TestReasonSetIsExhaustiveAndClosed(t *testing.T) {
	want := []Reason{
		ReasonNotReported, ReasonNotOfficial, ReasonNeverProbed, ReasonProbeFailed,
		ReasonParseFailed, ReasonNotConsented, ReasonProbeDisabled,
	}
	if len(Reasons) != len(want) {
		t.Fatalf("Reasons has %d members, want %d", len(Reasons), len(want))
	}
	for _, r := range want {
		if !r.Valid() {
			t.Errorf("declared member %q reports invalid", r)
		}
	}
	if Reason("probe_disabled") != ReasonProbeDisabled {
		t.Fatal("probe_disabled must be a declared member")
	}
	if Reason("something_else").Valid() {
		t.Fatal("an undeclared reason must not validate")
	}
}

func TestAttributionConfirmed(t *testing.T) {
	if AttributionConfirmed(ClientClaude) {
		t.Fatal("Claude must never be attribution_confirmed=true (C8)")
	}
	if !AttributionConfirmed(ClientCodex) {
		t.Fatal("Codex must always be attribution_confirmed=true (C8)")
	}
}

func TestEnvelopeAttributionConfirmedIsDerivedNotSettable(t *testing.T) {
	// QD-R1-F7: AttributionConfirmed is a method derived from Client, so an
	// Envelope literal cannot carry an independently wrong value.
	claude := Envelope{Client: ClientClaude}
	if claude.AttributionConfirmed() {
		t.Fatal("a Claude envelope must report AttributionConfirmed() == false")
	}
	codex := Envelope{Client: ClientCodex}
	if !codex.AttributionConfirmed() {
		t.Fatal("a Codex envelope must report AttributionConfirmed() == true")
	}
}

func TestCodexWindowKey(t *testing.T) {
	cases := []struct {
		limitID   string
		secondary bool
		want      string
	}{
		{"codex", false, "codex"},
		{"codex", true, "codex_secondary"},
		{"bengalfox", false, "bengalfox"},
		{"bengalfox", true, "bengalfox_secondary"},
	}
	for _, c := range cases {
		if got := CodexWindowKey(c.limitID, c.secondary); got != c.want {
			t.Errorf("CodexWindowKey(%q, %v) = %q, want %q", c.limitID, c.secondary, got, c.want)
		}
	}
}

func TestClaudeWindowKeysAreFixed(t *testing.T) {
	if ClaudeWindowFiveHour != "five_hour" || ClaudeWindowSevenDay != "seven_day" {
		t.Fatal("Claude window keys must be the vendor's own two payload names")
	}
}

func TestClaudeWindowMinutes(t *testing.T) {
	if minutes, reason := ClaudeWindowMinutes(ClaudeWindowFiveHour); minutes != 300 || reason != "" {
		t.Fatalf("five_hour = (%d, %q), want (300, \"\")", minutes, reason)
	}
	if minutes, reason := ClaudeWindowMinutes(ClaudeWindowSevenDay); minutes != 10080 || reason != "" {
		t.Fatalf("seven_day = (%d, %q), want (10080, \"\")", minutes, reason)
	}
	if minutes, reason := ClaudeWindowMinutes("thirty_day"); minutes != 0 || reason != ReasonNotReported {
		t.Fatalf("unrecognized window = (%d, %q), want (0, not_reported)", minutes, reason)
	}
}

func TestAllowedAgeBothWindowLengthsAtEachInterval(t *testing.T) {
	cases := []struct {
		name          string
		windowMinutes int
		interval      time.Duration
		want          time.Duration
	}{
		{"5h window at 5m interval", 300, 5 * time.Minute, 30 * time.Minute},
		{"5h window at 15m interval", 300, 15 * time.Minute, 30 * time.Minute},
		{"5h window at 30m interval, floor takes over", 300, 30 * time.Minute, 60 * time.Minute},
		{"7d window at 5m interval", 10080, 5 * time.Minute, 1008 * time.Minute},
		{"7d window at 15m interval", 10080, 15 * time.Minute, 1008 * time.Minute},
		{"7d window at 30m interval", 10080, 30 * time.Minute, 1008 * time.Minute},
	}
	for _, c := range cases {
		if got := AllowedAge(c.windowMinutes, c.interval); got != c.want {
			t.Errorf("%s: AllowedAge = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestStaleBoundaryOneMinuteEitherSide(t *testing.T) {
	// 5-hour window at a 30m interval: allowed_age = 60m (the floor takes over).
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	justUnder := []Window{{WindowKey: "codex", WindowMinutes: 300, ObservedAt: now.Add(-59 * time.Minute)}}
	if Stale(now, justUnder, 30*time.Minute) {
		t.Error("age one minute under the 60m threshold must not be stale")
	}

	justOver := []Window{{WindowKey: "codex", WindowMinutes: 300, ObservedAt: now.Add(-61 * time.Minute)}}
	if !Stale(now, justOver, 30*time.Minute) {
		t.Error("age one minute over the 60m threshold must be stale")
	}
}

func TestStaleFollowsShortestWindowsOwnObservationNotTheNewestAcrossWindows(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	// Codex PR #5 P1: a fresh weekly figure must not mask a stale five-hour
	// figure -- staleness is the shortest window's *own* age, never the
	// newest timestamp across every window.
	staleShort := []Window{
		{WindowKey: "seven_day", WindowMinutes: 10080, ObservedAt: now},                      // fresh
		{WindowKey: "five_hour", WindowMinutes: 300, ObservedAt: now.Add(-90 * time.Minute)}, // stale for the 30m allowed_age at a 5m interval
	}
	if !Stale(now, staleShort, 5*time.Minute) {
		t.Fatal("a stale shortest window must flag the card stale even when a longer window is fresh")
	}

	// The inverse: a stale (long-untouched) weekly figure must not flag the
	// card when the determinative five-hour figure is actually fresh.
	freshShort := []Window{
		{WindowKey: "seven_day", WindowMinutes: 10080, ObservedAt: now.Add(-1000 * time.Hour)}, // very stale
		{WindowKey: "five_hour", WindowMinutes: 300, ObservedAt: now.Add(-time.Minute)},        // fresh
	}
	if Stale(now, freshShort, 5*time.Minute) {
		t.Fatal("a fresh shortest window must not be flagged stale by a stale longer window")
	}
}

// Codex PR #5 third review, P2: Codex can expose several windows tied for
// the shortest duration, and a partial persistence failure can leave their
// observation times different. A fresh first-encountered tied-shortest
// window must not mask a stale sibling tied at the same duration.
func TestStaleAmongTiedShortestWindowsUsesTheOldestObservation(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	freshFirst := []Window{
		{WindowKey: "codex_primary", WindowMinutes: 300, ObservedAt: now},                          // fresh
		{WindowKey: "codex_secondary", WindowMinutes: 300, ObservedAt: now.Add(-90 * time.Minute)}, // stale for the 30m allowed_age at a 5m interval
	}
	if !Stale(now, freshFirst, 5*time.Minute) {
		t.Fatal("a stale tied-shortest window must flag the card stale even when the first-encountered tied window is fresh")
	}

	staleFirst := []Window{
		{WindowKey: "codex_primary", WindowMinutes: 300, ObservedAt: now.Add(-90 * time.Minute)}, // stale
		{WindowKey: "codex_secondary", WindowMinutes: 300, ObservedAt: now},                      // fresh
	}
	if !Stale(now, staleFirst, 5*time.Minute) {
		t.Fatal("a stale tied-shortest window must flag the card stale regardless of slice order")
	}
}

func TestStaleWithNoPresentWindowLength(t *testing.T) {
	windows := []Window{{WindowKey: "seven_day", WindowMinutesReason: ReasonNotReported, ObservedAt: time.Now().Add(-1000 * time.Hour)}}
	now := time.Now()
	if Stale(now, windows, 5*time.Minute) {
		t.Fatal("staleness cannot be computed with no present window length")
	}
}
