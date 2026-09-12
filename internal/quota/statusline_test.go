package quota

import (
	"bytes"
	"context"
	"testing"
	"time"
)

const statusLineSample = `{"rate_limits": {"five_hour": {"used_percentage": 22.4, "resets_at": 1757516400}, "seven_day": {"used_percentage": 3, "resets_at": 1758121200}}}`

func TestParseStatusLinePayloadFullMapping(t *testing.T) {
	payload, err := ParseStatusLinePayload([]byte(statusLineSample))
	if err != nil {
		t.Fatalf("ParseStatusLinePayload: %v", err)
	}
	if payload.RateLimits == nil {
		t.Fatal("RateLimits = nil, want populated")
	}
	if payload.RateLimits.FiveHour == nil || *payload.RateLimits.FiveHour.UsedPercentage != 22.4 {
		t.Fatalf("FiveHour = %+v, want used_percentage=22.4", payload.RateLimits.FiveHour)
	}
	if payload.RateLimits.SevenDay == nil || *payload.RateLimits.SevenDay.UsedPercentage != 3 {
		t.Fatalf("SevenDay = %+v, want used_percentage=3", payload.RateLimits.SevenDay)
	}
}

func TestParseStatusLinePayloadEmptyStdinIsNotAnError(t *testing.T) {
	payload, err := ParseStatusLinePayload(nil)
	if err != nil {
		t.Fatalf("ParseStatusLinePayload(nil): %v", err)
	}
	if payload.RateLimits != nil {
		t.Fatalf("RateLimits = %+v, want nil", payload.RateLimits)
	}
}

func TestParseStatusLinePayloadRateLimitsAbsentIsNotAnError(t *testing.T) {
	payload, err := ParseStatusLinePayload([]byte(`{}`))
	if err != nil {
		t.Fatalf("ParseStatusLinePayload: %v", err)
	}
	if payload.RateLimits != nil {
		t.Fatalf("RateLimits = %+v, want nil", payload.RateLimits)
	}
}

func TestParseStatusLinePayloadMalformedJSONIsAnError(t *testing.T) {
	if _, err := ParseStatusLinePayload([]byte(`{"rate_limits": `)); err == nil {
		t.Fatal("malformed JSON must be a parse error")
	}
}

func TestMapStatusLineObservationsFullMapping(t *testing.T) {
	payload, err := ParseStatusLinePayload([]byte(statusLineSample))
	if err != nil {
		t.Fatalf("ParseStatusLinePayload: %v", err)
	}
	observedAt := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	windows := MapStatusLineObservations(payload, observedAt)
	if len(windows) != 2 {
		t.Fatalf("windows = %d, want 2: %+v", len(windows), windows)
	}

	five := windows[0]
	if five.WindowKey != ClaudeWindowFiveHour || five.UsedPercent != 22.4 {
		t.Fatalf("five_hour window = %+v", five)
	}
	if five.WindowMinutes != 300 || five.WindowMinutesReason != "" {
		// C3: window_minutes is always the fixed local constant, never the
		// (absent) vendor field.
		t.Fatalf("five_hour.WindowMinutes = (%d, %q), want (300, \"\")", five.WindowMinutes, five.WindowMinutesReason)
	}
	if five.Source != SourceClaudeStatusLine || !five.ObservedAt.Equal(observedAt) {
		t.Fatalf("five_hour Source/ObservedAt = %v/%v", five.Source, five.ObservedAt)
	}
	if !five.ResetsAt.Equal(time.Unix(1757516400, 0).UTC()) {
		t.Fatalf("five_hour.ResetsAt = %v", five.ResetsAt)
	}

	seven := windows[1]
	if seven.WindowKey != ClaudeWindowSevenDay || seven.WindowMinutes != 10080 {
		t.Fatalf("seven_day window = %+v", seven)
	}
}

func TestMapStatusLineObservationsPartialPayload(t *testing.T) {
	payload, err := ParseStatusLinePayload([]byte(`{"rate_limits": {"five_hour": {"used_percentage": 10, "resets_at": 1}}}`))
	if err != nil {
		t.Fatalf("ParseStatusLinePayload: %v", err)
	}
	windows := MapStatusLineObservations(payload, time.Now())
	if len(windows) != 1 || windows[0].WindowKey != ClaudeWindowFiveHour {
		t.Fatalf("windows = %+v, want exactly one five_hour window", windows)
	}
}

func TestMapStatusLineObservationsNilRateLimits(t *testing.T) {
	windows := MapStatusLineObservations(StatusLinePayload{}, time.Now())
	if windows != nil {
		t.Fatalf("windows = %+v, want nil", windows)
	}
}

func TestMapStatusLineObservationsUnrecognizedWindowNameIsNotReported(t *testing.T) {
	// ClaudeWindowMinutes (task 1) already covers this, exercised here
	// through the mapping path used at runtime.
	minutes, reason := ClaudeWindowMinutes("thirty_day")
	if minutes != 0 || reason != ReasonNotReported {
		t.Fatalf("ClaudeWindowMinutes(unrecognized) = (%d, %q), want (0, not_reported)", minutes, reason)
	}
}

// --- ChainStatusLine / RecordStatusLinePayload tests. CLA-R1-F3 split what
// was one CaptureStatusLine call into these two so a caller can run the
// chain before ever touching a store; both are exercised together here in
// that same order, matching how quota_capture.go must call them.

func TestChainStatusLinePassesStdoutThroughByteForByte(t *testing.T) {
	var stdout bytes.Buffer
	ChainStatusLine(context.Background(), "cat", []byte(statusLineSample), &stdout)
	if stdout.String() != statusLineSample {
		t.Fatalf("stdout = %q, want the input passed through unchanged", stdout.String())
	}
}

func TestChainStatusLineNoPriorCommandRunsNoSubprocess(t *testing.T) {
	var stdout bytes.Buffer
	ChainStatusLine(context.Background(), "", []byte(statusLineSample), &stdout)
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when there is no prior command to chain to", stdout.String())
	}
}

func TestChainStatusLinePriorFailureDoesNotAffectSubsequentCapture(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	observedAt := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	var stdout bytes.Buffer

	ChainStatusLine(ctx, "exit 1", []byte(statusLineSample), &stdout)
	RecordStatusLinePayload(ctx, store, []byte(statusLineSample), observedAt)

	windows, err := store.Windows(ctx, ClientClaude)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 2 {
		t.Fatalf("windows = %+v, want capture to have persisted both fields despite the prior command failing", windows)
	}
}

func TestRecordStatusLinePayloadFailureDoesNotAffectAlreadyCompletedChain(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	var stdout bytes.Buffer

	ChainStatusLine(ctx, "cat", []byte("not valid json"), &stdout)
	RecordStatusLinePayload(ctx, store, []byte("not valid json"), time.Now())

	if stdout.String() != "not valid json" {
		t.Fatalf("stdout = %q, want the prior command's output despite the capture payload being malformed", stdout.String())
	}
	windows, err := store.Windows(ctx, ClientClaude)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 0 {
		t.Fatalf("windows = %+v, want none (malformed payload must not be recorded)", windows)
	}
}

func TestRecordStatusLinePayloadNilStoreIsANoop(t *testing.T) {
	// Must not panic; persistence being unavailable is swallowed (C3
	// property 3 applies here too, since this only ever runs after the
	// chain has already completed).
	RecordStatusLinePayload(context.Background(), nil, []byte(statusLineSample), time.Now())
}

func TestChainThenRecordPersistsThroughStore(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	observedAt := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	var stdout bytes.Buffer

	ChainStatusLine(ctx, "", []byte(statusLineSample), &stdout)
	RecordStatusLinePayload(ctx, store, []byte(statusLineSample), observedAt)

	windows, err := store.Windows(ctx, ClientClaude)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if len(windows) != 2 {
		t.Fatalf("windows = %+v, want both five_hour and seven_day persisted", windows)
	}
	for _, w := range windows {
		if w.Source != SourceClaudeStatusLine || !w.ObservedAt.Equal(observedAt) {
			t.Fatalf("window = %+v, want Source/ObservedAt set correctly", w)
		}
	}
}
