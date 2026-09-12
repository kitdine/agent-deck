package quota

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"time"
)

// MaxStatusLinePayloadBytes bounds what RecordStatusLinePayload will attempt
// to parse. It has no bearing on ChainStatusLine or on how much of stdin a
// caller reads: the chain must always receive the complete original stdin
// regardless of size (CLA-R1-F5) — this limit exists only so a pathological
// or oversized payload cannot make capture's own parsing spend unbounded
// work. Status-line payloads are tiny in practice; a caller should skip
// calling RecordStatusLinePayload at all once stdin exceeds this, per
// cmd/agentdeck/quota_capture.go's own size check (CLA-R2-F1).
const MaxStatusLinePayloadBytes = 64 * 1024

// StatusLinePayload is the JSON Claude Code passes on stdin to a configured
// statusLine command on each refresh of a live interactive session (C3):
//
//	{"rate_limits": {
//	   "five_hour": {"used_percentage": 0.0, "resets_at": 0},
//	   "seven_day": {"used_percentage": 0.0, "resets_at": 0}}}
//
// This is observed behavior (Anthropic issue #27915), not a declared
// contract: RateLimits being nil — the key absent, or present but null — is
// not_reported, never a failure.
type StatusLinePayload struct {
	RateLimits *StatusLineRateLimits `json:"rate_limits"`
}

type StatusLineRateLimits struct {
	FiveHour *StatusLineWindow `json:"five_hour"`
	SevenDay *StatusLineWindow `json:"seven_day"`
}

// StatusLineWindow's UsedPercentage is assumed to already be on a 0-100
// scale, matching Codex's own usedPercent field (C2), rather than a 0-1
// fraction the field's name could also suggest — unverified against a real
// payload, since this project declined to read Claude's own credentials to
// call an endpoint that could confirm it (C0), and no captured transcript
// exists in this topic's documents. Revisit if a real payload disagrees.
type StatusLineWindow struct {
	UsedPercentage *float64 `json:"used_percentage"`
	ResetsAt       *int64   `json:"resets_at"`
}

// ParseStatusLinePayload decodes the stdin payload. A malformed payload is
// an error; empty stdin or RateLimits being absent is not — both decode to
// a StatusLinePayload with RateLimits == nil.
func ParseStatusLinePayload(raw []byte) (StatusLinePayload, error) {
	var payload StatusLinePayload
	if len(bytes.TrimSpace(raw)) == 0 {
		return payload, nil
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return StatusLinePayload{}, err
	}
	return payload, nil
}

// MapStatusLineObservations maps the payload's five_hour/seven_day windows
// onto the domain. window_minutes is always the fixed local constant from
// ClaudeWindowMinutes (C3) — the payload carries no window length, and this
// never presents the constant as a vendor field. A window absent from the
// payload produces no Observation for it: not_reported, never a failure.
func MapStatusLineObservations(payload StatusLinePayload, observedAt time.Time) []Observation {
	if payload.RateLimits == nil {
		return nil
	}
	var windows []Observation
	if w := payload.RateLimits.FiveHour; w != nil {
		windows = append(windows, mapStatusLineWindow(ClaudeWindowFiveHour, *w, observedAt))
	}
	if w := payload.RateLimits.SevenDay; w != nil {
		windows = append(windows, mapStatusLineWindow(ClaudeWindowSevenDay, *w, observedAt))
	}
	return windows
}

func mapStatusLineWindow(windowKey string, w StatusLineWindow, observedAt time.Time) Observation {
	minutes, reason := ClaudeWindowMinutes(windowKey)
	obs := Observation{
		Client: ClientClaude, WindowKey: windowKey, Source: SourceClaudeStatusLine,
		ObservedAt: observedAt, WindowMinutes: minutes, WindowMinutesReason: reason,
	}
	if w.UsedPercentage != nil {
		obs.UsedPercent = *w.UsedPercentage
	}
	if w.ResetsAt != nil {
		obs.ResetsAt = time.Unix(*w.ResetsAt, 0).UTC()
	}
	return obs
}

// ChainStatusLine implements C3's mandatory chaining, property 1 and half of
// property 2: the prior command's stdout reaches Claude Code byte for byte
// — done by wiring cmd.Stdout directly to stdout, not by buffering and
// re-emitting — and its error is not even inspected, so a failure in it
// cannot affect anything the caller does afterward. priorCommand empty means
// nothing was registered before AgentDeck (or there was nothing to chain
// to); no subprocess runs in that case.
//
// Callers must run this before opening any store or doing any other
// capture work (CLA-R1-F3): a status-line command runs on every refresh of
// a live session, so nothing capture-related may sit in front of the chain
// — not even opening a database, whose lock wait or first-run migration
// could otherwise delay the user's own status line by seconds.
func ChainStatusLine(ctx context.Context, priorCommand string, stdin []byte, stdout io.Writer) {
	prior := strings.TrimSpace(priorCommand)
	if prior == "" {
		return
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", prior)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = stdout
	_ = cmd.Run()
}

// RecordStatusLinePayload implements C3's mandatory chaining, the other half
// of property 2 and all of property 3: a malformed payload or a nil store
// (persistence unavailable) is swallowed rather than returned, since by the
// time this runs the chain — ChainStatusLine — has already completed and
// nothing here can still affect it. Callers must call this only after
// ChainStatusLine, never before or concurrently with it.
func RecordStatusLinePayload(ctx context.Context, store *Store, stdin []byte, observedAt time.Time) {
	if store == nil {
		return
	}
	payload, err := ParseStatusLinePayload(stdin)
	if err != nil {
		return
	}
	for _, obs := range MapStatusLineObservations(payload, observedAt) {
		_, _, _, _ = store.Record(ctx, obs)
	}
}
