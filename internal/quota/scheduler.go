package quota

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Trigger is architecture.md C9's distinction between why a probe is being
// considered: it changes only whether the plain interval gates the attempt,
// never whether C1's gate or C9's backoff do.
type Trigger int

const (
	// TriggerBackground is a periodic snapshot refresh: quota is not probed
	// unless its own interval has elapsed since the last successful probe.
	TriggerBackground Trigger = iota
	// TriggerManual is a user-initiated refresh: it bypasses the interval,
	// but not the reading switch, the provider gate, or an active backoff —
	// C9 names only the interval as something a manual refresh overrides.
	TriggerManual
)

// Scheduler runs the two poll-based probes — Codex's JSON-RPC route and
// Claude's prose route — subject to C1's gate and C9's interval, backoff, and
// single-flight rules. It never touches Claude's status-line route, which is
// captured independently of any schedule by quota_capture (C3), driven by
// Claude Code's own refresh rather than ours.
//
// A Scheduler value carries no state of its own beyond configuration: every
// decision is read fresh from Store on each call, so it is safe to construct
// a new one per request (as desktop.Build does, since each `agentdeck
// desktop snapshot` invocation is a distinct process) rather than needing one
// long-lived instance.
type Scheduler struct {
	Store *Store
	// Interval is C9's configured background cadence (5m/15m/30m) and also
	// the starting step for geometric backoff.
	Interval time.Duration
	// MaxBackoff bounds how far backoff can grow. Zero means unbounded, which
	// no caller should actually configure; desktop.go always supplies one.
	MaxBackoff time.Duration
	// Now defaults to time.Now when nil; tests inject a controlled clock.
	Now func() time.Time
	// ProbeCodex and ProbeClaudeProse default to this package's own
	// ProbeCodex and ProbeClaudeProse. They exist as fields — rather than
	// requiring a caller to override this package's unexported
	// codexCommandArgs/claudeProseCommandArgs — so a caller outside this
	// package (internal/desktop's own tests, which drive Scheduler through
	// desktop.Service.RefreshQuota) can inject a fake without needing access
	// to those internal seams.
	ProbeCodex       func(ctx context.Context, observedAt time.Time, timeout time.Duration) (CodexResult, error)
	ProbeClaudeProse func(ctx context.Context, observedAt time.Time, timeout time.Duration) (ClaudeProseResult, error)
}

func (s Scheduler) probeCodex(ctx context.Context, observedAt time.Time) (CodexResult, error) {
	if s.ProbeCodex != nil {
		return s.ProbeCodex(ctx, observedAt, DefaultCodexTimeout)
	}
	return ProbeCodex(ctx, observedAt, DefaultCodexTimeout)
}

func (s Scheduler) probeClaudeProse(ctx context.Context, observedAt time.Time) (ClaudeProseResult, error) {
	if s.ProbeClaudeProse != nil {
		return s.ProbeClaudeProse(ctx, observedAt, DefaultClaudeProseTimeout)
	}
	return ProbeClaudeProse(ctx, observedAt, DefaultClaudeProseTimeout)
}

func (s Scheduler) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// quotaProbeInFlight is single-flight's guard, and it is deliberately only
// in-process best-effort, not the full "a refresh arriving while a probe is
// running joins the running probe" guarantee C9's prose describes
// (GS-R1-F3). A call that finds its client already in flight does not start
// a second subprocess, but it also does not wait for the in-flight one and
// does not join its result — it returns immediately, having read whatever
// was stored before either call started; its own caller sees that
// unrefreshed state rather than the fresher figure the in-flight call will
// produce moments later. This is a real gap against the prose, accepted
// rather than closed, because closing it needs a cross-process lock: each
// `agentdeck desktop snapshot` invocation is its own short-lived OS process
// (RefreshQuota's own doc), so a package-level Go value can only ever
// protect concurrent calls inside one process. In this task's one production
// caller, RefreshQuota (internal/desktop/desktop.go), the two clients are
// visited sequentially and nothing else calls Run concurrently, so this
// guard does not currently trigger in production at all — it exists as a
// defensive backstop for a future concurrent caller, and is exercised only
// by this package's own tests. It is a package-level set rather than
// per-Scheduler state because Scheduler values are deliberately cheap and
// stateless (see the type doc) — two Scheduler values in the same process
// must still agree on what is in flight.
var quotaProbeInFlight sync.Map

// Run executes C9's schedule for one client. allowed is C1's already-computed
// gate result (see Allowed) — Run does not compute it itself, so its own
// tests never need to fake provider or usage services. With allowed false,
// Run does nothing at all: no store read beyond nothing, no subprocess, and
// no change to any stored state, matching C9's first table row exactly.
//
// Run is fail-open throughout: a Store error reading the current envelope,
// or from persisting a probe's result, is swallowed rather than returned,
// because a snapshot refresh must never fail on quota's account.
func (s Scheduler) Run(ctx context.Context, client Client, trigger Trigger, allowed bool) {
	if !allowed {
		return
	}
	rec, hasRecord, err := s.Store.Envelope(ctx, client)
	if err != nil {
		return
	}
	now := s.now()
	if !s.due(trigger, rec, hasRecord, now) {
		return
	}
	if _, alreadyRunning := quotaProbeInFlight.LoadOrStore(client, struct{}{}); alreadyRunning {
		return
	}
	defer quotaProbeInFlight.Delete(client)

	switch client {
	case ClientCodex:
		s.runCodex(ctx, now, rec, hasRecord, trigger)
	case ClientClaude:
		s.runClaudeProse(ctx, now, rec, hasRecord, trigger)
	}
}

// due decides whether trigger may run right now, given the client's current
// envelope record. A manual trigger bypasses both the plain interval and
// backoff — architecture.md C9 names only the reading switch and the
// provider gate as still applying to a user-initiated refresh (GS-R1-F2: an
// earlier version also applied backoff to manual, an unrecorded narrowing of
// that contract that could lock a user-requested retry out for up to an
// hour after one transient failure; single-flight still applies regardless,
// via Run's own check). The plain interval otherwise applies only to a
// background trigger.
func (s Scheduler) due(trigger Trigger, rec EnvelopeRecord, hasRecord bool, now time.Time) bool {
	if trigger == TriggerManual {
		return true
	}
	if hasRecord && !rec.BackoffUntil.IsZero() && now.Before(rec.BackoffUntil) {
		return false
	}
	if !hasRecord || rec.ObservedAt.IsZero() {
		return true
	}
	return !now.Before(rec.ObservedAt.Add(s.Interval))
}

func (s Scheduler) runCodex(ctx context.Context, observedAt time.Time, rec EnvelopeRecord, hasRecord bool, trigger Trigger) {
	result, err := s.probeCodex(ctx, observedAt)
	if err != nil {
		s.recordFailure(ctx, ClientCodex, codexFailureReason(err), observedAt, rec, hasRecord, trigger)
		return
	}
	for _, obs := range result.Windows {
		_, _, _, _ = s.Store.Record(ctx, obs)
	}
	_ = s.Store.PutEnvelope(ctx, EnvelopeRecord{
		Client: ClientCodex, AccountID: result.AccountID, Applicable: true,
		Source: SourceCodex, ObservedAt: observedAt,
		Plan: result.Plan, PlanReason: result.PlanReason,
		ResetAllowance: result.ResetAllowance,
		Billing:        result.Billing,
	})
}

func (s Scheduler) runClaudeProse(ctx context.Context, observedAt time.Time, rec EnvelopeRecord, hasRecord bool, trigger Trigger) {
	result, err := s.probeClaudeProse(ctx, observedAt)
	if err != nil {
		s.recordFailure(ctx, ClientClaude, claudeFailureReason(err), observedAt, rec, hasRecord, trigger)
		return
	}
	for _, obs := range result.Windows {
		_, _, _, _ = s.Store.Record(ctx, obs)
	}
	// Claude has neither Plan nor Billing nor ResetAllowance (C6: the reset
	// allowance is Codex-only) — the envelope carries only source and instant.
	_ = s.Store.PutEnvelope(ctx, EnvelopeRecord{
		Client: ClientClaude, Applicable: true,
		Source: SourceClaudeProse, ObservedAt: observedAt,
	})
}

// recordFailure persists a failed probe attempt. attemptedAt is the real
// instant of this attempt, background or manual alike, and is always written
// to FailureObservedAt (WC-R2-F1) so a manual failure has an attempt instant
// to show even though it leaves the background backoff chain alone. A manual
// failure records the failure itself (the Reason) but leaves that chain —
// both BackoffUntil and FailureAt — exactly as it already was, whether zero
// or already advanced by an earlier background failure: manual is neither
// consumer nor producer of that chain (GS-R2-F1, GS-R3-F1). Manual already
// bypasses backoff as a consumer (due, above); as a producer, both halves of
// the pair nextBackoff derives the prior step's length from
// (BackoffUntil.Sub(FailureAt)) must move together or not at all — moving
// FailureAt alone, as an earlier version of this fix did, shrinks or
// inverts that derived step and corrupts every later background failure's
// doubling. Only a background failure advances the chain.
func (s Scheduler) recordFailure(ctx context.Context, client Client, reason Reason, attemptedAt time.Time, rec EnvelopeRecord, hasRecord bool, trigger Trigger) {
	failureAt := attemptedAt
	backoffUntil := rec.BackoffUntil
	if trigger != TriggerManual {
		backoffUntil = attemptedAt.Add(s.nextBackoff(rec, hasRecord))
	} else {
		// GS-R3-F1: nextBackoff derives the prior step's length from
		// BackoffUntil and FailureAt together (BackoffUntil.Sub(FailureAt)).
		// A manual failure already leaves BackoffUntil untouched (above); it
		// must leave FailureAt untouched too, or that pairing is broken —
		// moving FailureAt forward while BackoffUntil stays put shrinks, and
		// can even invert, the derived step, corrupting every subsequent
		// background failure's doubling. Only Failure (the reason) and
		// FailureObservedAt (below) reflect this manual attempt; the pair
		// that anchors the background chain is exclusively a background
		// failure's to move.
		failureAt = rec.FailureAt
	}
	_ = s.Store.PutEnvelopeFailure(ctx, client, reason, failureAt, backoffUntil, attemptedAt)
}

// nextBackoff computes C9's next backoff step: the interval on the first
// failure of a streak, doubling on each consecutive failure thereafter, and
// bounded by MaxBackoff. A streak is "consecutive" exactly when the prior
// write was itself a failure with a backoff already in effect — recorded as
// rec.FailureAt and rec.BackoffUntil both being non-zero; a success clears
// both together (see PutEnvelope), so their presence here means nothing
// succeeded in between. The prior step's own duration is derived from those
// two fields (BackoffUntil.Sub(FailureAt)) rather than from a separate
// consecutive-failure counter, which the schema does not carry.
func (s Scheduler) nextBackoff(rec EnvelopeRecord, hasRecord bool) time.Duration {
	base := s.Interval
	if !hasRecord || rec.FailureAt.IsZero() || rec.BackoffUntil.IsZero() {
		return base
	}
	prev := rec.BackoffUntil.Sub(rec.FailureAt)
	if prev <= 0 {
		return base
	}
	next := prev * 2
	if s.MaxBackoff > 0 && next > s.MaxBackoff {
		next = s.MaxBackoff
	}
	if next < base {
		next = base
	}
	return next
}

func codexFailureReason(err error) Reason {
	var codexErr *CodexFailureError
	if errors.As(err, &codexErr) {
		return codexErr.Kind.Reason()
	}
	return ReasonProbeFailed
}

func claudeFailureReason(err error) Reason {
	var claudeErr *ClaudeFailureError
	if errors.As(err, &claudeErr) {
		return claudeErr.Kind.Reason()
	}
	return ReasonProbeFailed
}
