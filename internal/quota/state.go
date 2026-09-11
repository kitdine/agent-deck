package quota

import "time"

// Observation is one probe's reading for a single window, before persistence
// bookkeeping. AccountID scopes Codex observations (C8); it is always empty
// for Claude, which reports no account identifier. This is the caller's raw
// account ID — Store digests it before it ever reaches SQL (QD-R3-F2); this
// type itself carries the original value.
type Observation struct {
	Client    Client
	AccountID string
	WindowKey string
	Source    Source

	ObservedAt time.Time

	// VendorOrder is this window's position in the vendor's own response
	// (C11 — windows[] keeps vendor order). The caller assigns it from the
	// response it just parsed; Store persists and orders reads by it.
	VendorOrder int

	WindowMinutes       int
	WindowMinutesReason Reason

	Label       string
	UsedPercent float64
	ResetsAt    time.Time
}

// Stored is what Store retains per (client, account_id, window_key): the
// latest observation plus at most one prior. C7 refuses a time series; one
// prior is exactly what ApplyObservation needs to derive ObservedResetAt.
type Stored struct {
	Latest Observation
	Prior  *Observation
}

// Accept reports whether next should replace existing at the persistence
// boundary, per C5's source precedence: a newer observation always wins; at
// equal age, only the status-line route may replace a non-status-line one.
// An older observation, or an equal-age non-status-line observation arriving
// after a status-line one, is rejected — arrival order at the store must not
// override C5 (QD-R1-F1).
func Accept(existing, next Observation) bool {
	switch {
	case next.ObservedAt.After(existing.ObservedAt):
		return true
	case next.ObservedAt.Equal(existing.ObservedAt):
		return next.Source == SourceClaudeStatusLine && existing.Source != SourceClaudeStatusLine
	default:
		return false
	}
}

// crossRouteResetThreshold is the minimum decrease, in percentage points,
// required to treat a change of Source as a reset (QD-R1-F2). Claude's two
// routes report different precision — the prose route is integer-only, the
// status-line payload carries a JSON number — so a bare route switch can
// look like a small decrease that is really the same figure read more
// coarsely. A same-source decrease of any size still counts: within one
// route there is no precision seam to guard against.
const crossRouteResetThreshold = 1.0

// ApplyObservation folds a new Observation onto the existing Stored state for
// the same key. A reset is observed only when UsedPercent decreases (C6) —
// never when ResetsAt merely passes, since a passed instant only proves the
// window *should* have reset. The decrease must clear crossRouteResetThreshold
// when prior and next came from different sources.
//
// existing is nil for a key with no prior stored state. Callers must gate
// next through Accept first when existing is non-nil; ApplyObservation does
// not itself reject an older or losing-tiebreak observation.
//
// resetObserved reports whether this call detected a fresh local reset; when
// true, observedResetAt is next.ObservedAt.
func ApplyObservation(existing *Stored, next Observation) (updated Stored, observedResetAt time.Time, resetObserved bool) {
	if existing == nil {
		return Stored{Latest: next}, time.Time{}, false
	}
	prior := existing.Latest
	decrease := prior.UsedPercent - next.UsedPercent
	if prior.Source == next.Source {
		resetObserved = decrease > 0
	} else {
		resetObserved = decrease >= crossRouteResetThreshold
	}
	if resetObserved {
		observedResetAt = next.ObservedAt
	}
	return Stored{Latest: next, Prior: &prior}, observedResetAt, resetObserved
}

// SelectObservation implements C5's per-client source precedence: the newest
// usable observation wins, with a tiebreak toward the status-line route at
// equal age. The chosen Source travels unchanged into the resulting
// Observation so a surface can name where the figure came from.
func SelectObservation(candidates []Observation) (Observation, bool) {
	var best Observation
	found := false
	for _, c := range candidates {
		switch {
		case !found:
			best, found = c, true
		case c.ObservedAt.After(best.ObservedAt):
			best = c
		case c.ObservedAt.Equal(best.ObservedAt) &&
			c.Source == SourceClaudeStatusLine && best.Source != SourceClaudeStatusLine:
			best = c
		}
	}
	return best, found
}
