// Package quota holds the subscription-quota domain model: the per-client
// envelope, its windows, and the closed reason set surfaces render. It owns
// no transport and no credential access (architecture.md C0); adapters,
// scheduling, alerts, wire, and CLI live in later packages/tasks.
package quota

import "time"

// Client identifies which vendor client an observation or envelope is about.
type Client string

const (
	ClientCodex  Client = "codex"
	ClientClaude Client = "claude"
)

// Source identifies which adapter route produced an Observation. Codex has
// one route; Claude has two, and C5's tiebreak distinguishes them.
type Source string

const (
	SourceCodex            Source = "codex"
	SourceClaudeStatusLine Source = "claude_status_line"
	SourceClaudeProse      Source = "claude_prose"
)

// Reason is architecture.md C6's closed set of causes for an absent or
// unusable value. It is closed so every surface can render every member; a
// free-text reason would arrive untranslated.
type Reason string

const (
	ReasonNotReported   Reason = "not_reported"
	ReasonNotOfficial   Reason = "not_official"
	ReasonNeverProbed   Reason = "never_probed"
	ReasonProbeFailed   Reason = "probe_failed"
	ReasonParseFailed   Reason = "parse_failed"
	ReasonNotConsented  Reason = "not_consented"
	ReasonProbeDisabled Reason = "probe_disabled"
)

// Reasons enumerates the closed set exhaustively, so a caller can validate
// membership or render every member without hand-copying the list.
var Reasons = []Reason{
	ReasonNotReported,
	ReasonNotOfficial,
	ReasonNeverProbed,
	ReasonProbeFailed,
	ReasonParseFailed,
	ReasonNotConsented,
	ReasonProbeDisabled,
}

// Valid reports whether r is a member of the closed Reason set.
func (r Reason) Valid() bool {
	for _, known := range Reasons {
		if r == known {
			return true
		}
	}
	return false
}

// AttributionConfirmed is C8: false for Claude, true for Codex, always by
// construction rather than a value a caller could set incorrectly.
func AttributionConfirmed(c Client) bool {
	return c == ClientCodex
}

// Codex window-key construction (C6): the vendor's limitId, with "_secondary"
// appended when the window came from the secondary slot. The primary slot
// takes the bare limitId so the common single-window case has the shortest
// key.
func CodexWindowKey(limitID string, secondary bool) string {
	if secondary {
		return limitID + "_secondary"
	}
	return limitID
}

// Claude's two window keys are fixed vendor names (C6); both routes parse
// into these same two values so one client cannot produce two vocabularies
// for the same window depending on which route answered.
const (
	ClaudeWindowFiveHour = "five_hour"
	ClaudeWindowSevenDay = "seven_day"
)

// ClaudeWindowMinutes maps a Claude window key to its fixed local length
// (C3). The length is a local constant, not vendor data, because neither
// Claude route reports one. A window key outside the two known names yields
// ReasonNotReported for the length; the surface then labels it by name alone.
func ClaudeWindowMinutes(windowKey string) (minutes int, reason Reason) {
	switch windowKey {
	case ClaudeWindowFiveHour:
		return 300, ""
	case ClaudeWindowSevenDay:
		return 10080, ""
	default:
		return 0, ReasonNotReported
	}
}

// AllowedAge is C5's staleness threshold: a tenth of the window, floored at
// twice the probe interval so the flag does not fire on the product's own
// cadence.
func AllowedAge(windowMinutes int, probeInterval time.Duration) time.Duration {
	tenth := time.Duration(windowMinutes) * time.Minute / 10
	floor := 2 * probeInterval
	if tenth > floor {
		return tenth
	}
	return floor
}

// ShortestWindowMinutes returns the smallest WindowMinutes among windows
// whose length is present (WindowMinutesReason == ""). ok is false when no
// window has a present length.
func ShortestWindowMinutes(windows []Window) (minutes int, ok bool) {
	for _, w := range windows {
		if w.WindowMinutesReason != "" {
			continue
		}
		if !ok || w.WindowMinutes < minutes {
			minutes = w.WindowMinutes
			ok = true
		}
	}
	return minutes, ok
}

// Stale is C5: a client's card is stale when the age of its shortest-window
// observation exceeds that window's AllowedAge. With no window carrying a
// present length, staleness cannot be computed on this basis and is false.
func Stale(observedAt, now time.Time, windows []Window, probeInterval time.Duration) bool {
	shortest, ok := ShortestWindowMinutes(windows)
	if !ok {
		return false
	}
	return now.Sub(observedAt) > AllowedAge(shortest, probeInterval)
}

// Window is one rate-limit window's current state, keyed by WindowKey (C6).
// WindowKey is opaque to every surface: a join key for storage, alert
// dedup, and reset detection, never a label.
type Window struct {
	WindowKey string
	// Label is the vendor's optional display label; empty when absent (C2 —
	// absent on the main limit). Unlike the closed-reason fields, an absent
	// Label carries no reason: it is ordinary optional vendor metadata.
	Label string

	// Source and ObservedAt are the winning observation's provenance (C5): the
	// chosen source travels unchanged from selection through storage to the
	// payload, so a surface can name where a figure came from.
	Source     Source
	ObservedAt time.Time

	// VendorOrder is this window's position in the vendor's own response
	// order (C11 — windows[] keeps vendor order, never sorted). It is
	// persisted per window because the store is the only durable copy of
	// that order.
	VendorOrder int

	WindowMinutes       int
	WindowMinutesReason Reason // set only when WindowMinutes is absent

	UsedPercent float64

	// ResetsAt is when this window reopens, per the vendor. It is one of the
	// three reset semantics (C6) and is never conflated with the other two.
	ResetsAt time.Time

	// ObservedResetAt is AgentDeck's own locally-derived reset instant (C6):
	// set only when a decrease in UsedPercent was observed, never inferred
	// from ResetsAt passing. It is sticky — it persists until superseded by
	// the next locally-observed reset for this key.
	ObservedResetAt    time.Time
	HasObservedResetAt bool
}

// ResetCredit is one disclosed Codex reset-allowance credit (C2).
type ResetCredit struct {
	Title     string
	Status    string
	GrantedAt time.Time
	ExpiresAt time.Time
}

// ResetAllowance is C6's second reset semantic — official resets the account
// may still spend — and is Codex-only. It is never conflated with
// Window.ResetsAt or Window.ObservedResetAt.
type ResetAllowance struct {
	// Total is currently always absent (architecture.md open question 1): no
	// observation has proven a total derivable from availableCount.
	Total       int
	TotalReason Reason

	Remaining    int
	HasRemaining bool

	Credits []ResetCredit
}

// Billing carries the vendor's top-level billing/credits block. Its Balance
// is domain-internal only; C11 excludes it from the desktop wire.
type Billing struct {
	Balance    float64
	HasBalance bool
}

// Envelope is one client's current quota state, assembled by later tasks
// from Windows selected via SelectObservation and persisted via Store. Its
// three reset semantics live on separate fields (Window.ResetsAt,
// ResetAllowance.Remaining, Window.ObservedResetAt) and cannot be confused by
// construction.
type Envelope struct {
	Client Client

	Applicable       bool
	ApplicableReason Reason // set when Applicable is false (C1: probe_disabled or not_official)

	Source     Source
	ObservedAt time.Time
	Stale      bool

	Plan       string
	PlanReason Reason // set when Plan is absent

	Windows []Window

	ResetAllowance       ResetAllowance
	ResetAllowanceReason Reason // set when ResetAllowance is wholly absent (Claude has none)

	Billing Billing

	Failure Reason // set when the last probe failed rather than produced a reading
}

// AttributionConfirmed is C8, derived from Client rather than stored as an
// independently settable field: false for Claude, true for Codex, always by
// construction. A caller cannot forget to set it or set it incorrectly.
func (e Envelope) AttributionConfirmed() bool {
	return AttributionConfirmed(e.Client)
}

// EnvelopeRecord is the persisted, client-level half of an Envelope — every
// field C7 requires alongside the latest per-client envelope that is not
// carried by an individual Window row. Stale and Windows are not part of it:
// Stale is derived at read time against the current instant, and Windows
// come from the per-window rows Store already retains.
type EnvelopeRecord struct {
	Client Client

	// AccountID is an isolation key only (C8), the same as Observation's: it
	// scopes when Store discards a Codex client's stored state on an account
	// change, and is always empty for Claude. It is never a display field —
	// task 6's wire builder must not copy it into a client-facing payload.
	// This is the caller's raw account ID; Store digests it before it ever
	// reaches SQL and returns that digest on read, never the original value
	// (QD-R3-F2).
	AccountID string

	Applicable       bool
	ApplicableReason Reason // set when Applicable is false (C1: probe_disabled or not_official)

	Source     Source
	ObservedAt time.Time

	Plan       string
	PlanReason Reason // set when Plan is absent

	ResetAllowance       ResetAllowance
	ResetAllowanceReason Reason // set when ResetAllowance is wholly absent (Claude has none)

	Billing Billing

	// Failure and FailureAt describe the last probe attempt that did not
	// produce a reading. They are independent of ObservedAt, which stays the
	// last *successful* observation's instant (C9 — a backed-off client still
	// shows its last figure and real age): recording a failure must never
	// overwrite ObservedAt or any other last-known-good field (QD-R2-F2).
	Failure   Reason
	FailureAt time.Time
}
