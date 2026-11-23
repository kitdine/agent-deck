package quota

import (
	"context"
	"errors"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

const (
	// RefreshTimeout is the total time budget for a quota refresh.
	RefreshTimeout = 37 * time.Second

	// Each scheduled write reserves one second for the transaction and one
	// for rollback and restoring the connection before it can be reused.
	refreshWriteAndCleanup = 2 * time.Second
	refreshGuardWait       = time.Second
	refreshReconcileBudget = refreshGuardWait + 100*time.Millisecond
	refreshProcessExit     = 500 * time.Millisecond
	refreshPreparation     = refreshWriteAndCleanup + refreshReconcileBudget
	refreshProbes          = max(
		DefaultCodexTimeout+codexIdentityTimeout+refreshProcessExit+refreshWriteAndCleanup,
		DefaultClaudeAuthTimeout+DefaultClaudeProseTimeout+2*refreshProcessExit+refreshGuardWait+3*refreshWriteAndCleanup,
	)

	// RefreshReportBudget reserves alert pruning, its connection cleanup, and output.
	RefreshReportBudget = refreshWriteAndCleanup + 500*time.Millisecond
)

// RefreshLockDeadline reserves time for preparation, probes and reporting.
// It returns zero if ctx has no deadline.
func RefreshLockDeadline(ctx context.Context) time.Time {
	deadline, ok := ctx.Deadline()
	if !ok {
		return time.Time{}
	}
	cutoff := deadline.Add(-refreshPreparation - refreshProbes - RefreshReportBudget)
	waitLimit := time.Now().Add(20 * time.Second)
	if waitLimit.Before(cutoff) {
		return waitLimit
	}
	return cutoff
}

// RefreshLockWait reports the time remaining before the lock cutoff.
func RefreshLockWait(ctx context.Context) time.Duration {
	cutoff := RefreshLockDeadline(ctx)
	if cutoff.IsZero() {
		return 0
	}
	return time.Until(cutoff)
}

func hasRefreshBudget(ctx context.Context, work time.Duration) bool {
	d, ok := ctx.Deadline()
	return ok && ctx.Err() == nil && time.Until(d) >= work+RefreshReportBudget
}

// RunRefresh owns both branches until they have settled. The caller retains
// the same refresh lock and open core through subsequent alert evaluation.
func (s Scheduler) RunRefresh(ctx context.Context, root *RefreshCore, lock *store.Lock, trigger Trigger, gates map[Client]Reason) (RefreshCycle, *QuotaDeliveryGuard, error) {
	allowed := func(c Client) bool { r, ok := gates[c]; return ok && r == "" }
	if !allowed(ClientClaude) && !allowed(ClientCodex) {
		return RefreshCycle{}, nil, nil
	}
	if root == nil || root.store != s.Store || !hasRefreshBudget(ctx, refreshPreparation+refreshProbes) {
		return RefreshCycle{}, nil, store.ErrStateBusy
	}
	// The root name may have changed between opening the core and acquiring
	// the lock. Bind their directories before BeginRefreshCycle checks the entry.
	if !lock.AcquiredIn(root.dir) {
		return RefreshCycle{}, nil, store.ErrLockLost
	}
	cycle, err := s.Store.BeginRefreshCycle(ctx, lock, s.now())
	if err != nil {
		return RefreshCycle{}, nil, err
	}
	var guard *QuotaDeliveryGuard
	var guardErr error
	if allowed(ClientClaude) {
		if hasRefreshBudget(ctx, refreshReconcileBudget+refreshProbes) {
			var e error
			guard, e = s.Store.reconcileQuotaDelivery(ctx, cycle, root)
			if errors.Is(e, ErrQuotaDeliveryUnavailable) || errors.Is(e, ErrQuotaDeliveryBusy) {
				guardErr = e
			}
		} else {
			guardErr = ErrQuotaDeliveryBusy
		}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if allowed(ClientCodex) {
			s.runScheduledCodex(ctx, cycle, trigger)
		}
	}()
	if guard != nil {
		if _, busy := quotaProbeInFlight.LoadOrStore(ClientClaude, struct{}{}); !busy {
			defer quotaProbeInFlight.Delete(ClientClaude)
			var available bool
			available, guardErr = s.runGuardedClaude(ctx, cycle, guard, trigger)
			if !available {
				guard = nil
			}
		} else {
			guard = nil
		}
	}
	<-done
	// A partial guard failure travels with the completed Codex branch.
	return cycle, guard, guardErr
}

// Codex's two sides share a transport, never a cadence. Read their inputs
// together, then commit the adapter result (including its private proof)
// once, even when only one side succeeded.
func (s Scheduler) runScheduledCodex(ctx context.Context, c RefreshCycle, trigger Trigger) {
	if _, busy := quotaProbeInFlight.LoadOrStore(ClientCodex, struct{}{}); busy {
		return
	}
	defer quotaProbeInFlight.Delete(ClientCodex)
	state, err := s.scheduleState(ctx, ClientCodex)
	if err != nil {
		return
	}
	i := state.identity
	idDue := !c.at.Before(i.BackoffUntil) && (trigger == TriggerManual || i.AttemptedAt.IsZero() || !c.at.Before(i.AttemptedAt.Add(s.Interval)))
	usageDue := s.due(trigger, state.usage.Envelope, state.hasUsage, c.at)
	// A legacy injected probe simulates usage only; it cannot report an
	// identity attempt and must never fall through to an installed binary.
	if s.ProbeCodexCycle == nil && s.ProbeCodex != nil {
		idDue = false
	}
	if !idDue && !usageDue {
		return
	}
	work := DefaultCodexTimeout + refreshProcessExit + refreshWriteAndCleanup
	if idDue && usageDue {
		work += codexIdentityTimeout
	}
	if !hasRefreshBudget(ctx, work) {
		return
	}
	var result CodexCycleResult
	if s.ProbeCodexCycle != nil {
		result = s.ProbeCodexCycle(ctx, c.at, idDue, usageDue)
	} else if s.ProbeCodex == nil {
		result = ProbeCodexCycle(ctx, c.at, idDue, usageDue)
	} else {
		usage, e := s.probeCodex(ctx, c.at)
		if e != nil {
			result.Failure = &UsageFailure{Reason: codexFailureReason(e)}
		} else {
			result.Usage = &usage
		}
	}
	if !hasRefreshBudget(ctx, refreshWriteAndCleanup) {
		return
	}
	at := s.now()
	var base, until time.Time
	if result.Identity != nil && result.Identity.Failure != "" {
		base = at
		until = base.Add(s.nextBackoff(EnvelopeRecord{FailureAt: i.BackoffBaseAt, BackoffUntil: i.BackoffUntil}, true))
	}
	if result.Failure != nil {
		f := result.Failure
		f.Trigger, f.FailureObservedAt = trigger, at
		if trigger == TriggerBackground {
			f.FailureAt = at
			f.BackoffUntil = at.Add(s.nextBackoff(state.usage.Envelope, state.hasUsage))
		}
	}
	_, _ = s.Store.CommitCodexCycle(ctx, c, result, base, until)
}

type clientScheduleState struct {
	identity IdentityRecord
	usage    UsageRecord
	hasUsage bool
}

func (s Scheduler) scheduleState(ctx context.Context, client Client) (clientScheduleState, error) {
	return readQuota(s.Store.db, ctx, func(r *SnapshotReader) (v clientScheduleState, err error) {
		v.identity, _, err = r.Identity(ctx, client)
		if err == nil {
			v.usage, v.hasUsage, err = r.Usage(ctx, client)
		}
		return
	})
}

func (s Scheduler) runGuardedClaude(ctx context.Context, c RefreshCycle, g *QuotaDeliveryGuard, trigger Trigger) (bool, error) {
	state, err := s.scheduleState(ctx, ClientClaude)
	if err != nil {
		return false, nil
	}
	now := c.at
	i, u := state.identity, state.usage
	// Recover a committed tx1 followed by interruption before tx2, or a
	// confirmed gate from a previous boot, even when neither probe is due.
	gatePublished := false
	if u.Claude.Generation == u.Claude.ConfirmedGeneration && (u.Claude.Gate == CaptureBlocked || u.Claude.Gate == CaptureOpen) {
		if stamp, ok := refreshBootStamp(); ok && (u.Claude.Gate == CaptureBlocked || u.Claude.OpenedAt.Boot != stamp.Boot) {
			if !hasRefreshBudget(ctx, refreshWriteAndCleanup) {
				return false, nil
			}
			out, err := s.Store.PublishClaudeCaptureGate(ctx, c, u.Claude.Generation, stamp)
			if err != nil || out != WriteCommitted {
				return false, nil
			}
			gatePublished = true
		}
	}
	idAllowed := !now.Before(i.BackoffUntil)
	idDue := idAllowed && (trigger == TriggerManual || i.AttemptedAt.IsZero() || !now.Before(i.AttemptedAt.Add(s.Interval)))
	usageDue := s.due(trigger, u.Envelope, state.hasUsage, now)
	if !usageDue && !idDue {
		return true, nil
	}
	blocked := u.Claude.Gate == CaptureBlocked && u.Claude.Generation != u.Claude.ConfirmedGeneration
	if blocked && idAllowed {
		idDue = true
	}
	// One executable resolution supplies both subprocesses, cwd and env.
	// Injected probes are also paired; never fall through to a real binary
	// when a caller has installed an isolated usage fixture.
	var probe *ClaudeProbe
	if s.ProbeClaudeProse == nil && s.ProbeClaudeIdentity == nil {
		probe, _ = NewClaudeProbe()
	}
	var identity ClaudeIdentityResult
	if idDue {
		if !hasRefreshBudget(ctx, DefaultClaudeAuthTimeout+refreshProcessExit+refreshGuardWait+refreshWriteAndCleanup) {
			return false, nil
		}
		obs := IdentityObservation{Failure: IdentityProbeFailed}
		switch {
		case s.ProbeClaudeIdentity != nil:
			obs = s.ProbeClaudeIdentity(ctx, g.root)
		case probe != nil:
			obs = probe.ProbeIdentity(ctx, g.root)
		}
		if !hasRefreshBudget(ctx, refreshGuardWait+refreshWriteAndCleanup) {
			return false, nil
		}
		var base, until time.Time
		if obs.Failure != "" {
			base = s.now()
			until = base.Add(s.nextBackoff(EnvelopeRecord{FailureAt: i.BackoffBaseAt, BackoffUntil: i.BackoffUntil}, true))
		}
		var deliveryFailed bool
		identity, err, deliveryFailed = g.applyClaudeIdentity(ctx, obs, base, until)
		if deliveryFailed {
			return false, err
		}
		if err != nil || identity.Outcome != WriteCommitted {
			return false, nil
		}
	}
	current, err := s.Store.CommittedClaudeContext(ctx, c)
	if err != nil {
		return false, nil
	}
	if identity.Invalidation != "" {
		usageDue = true
	} else if blocked && identity.proof != nil && (trigger == TriggerManual || !now.Before(u.Envelope.BackoffUntil)) {
		usageDue = true
	}
	if !usageDue {
		return true, nil
	}
	// Recovery used this cycle's single gate write. A successful observation
	// of a changed subscription would require another after usage commits.
	// Leave the invalidated generation due for the next refresh.
	if gatePublished && current.Gate == CaptureBlocked && identity.proof != nil {
		return false, nil
	}
	canPublishGate := !gatePublished && current.Gate != CaptureOpenInitial &&
		(identity.proof != nil || current.ConfirmedGeneration == current.Generation)
	usageBudget := DefaultClaudeProseTimeout + refreshProcessExit + refreshWriteAndCleanup
	if canPublishGate {
		usageBudget += refreshWriteAndCleanup
	}
	if !hasRefreshBudget(ctx, usageBudget) {
		return false, nil
	}
	var usage ClaudeProseResult
	switch {
	case s.ProbeClaudeProse != nil:
		usage, err = s.probeClaudeProse(ctx, now)
	case probe != nil:
		usage, err = probe.ProbeUsage(ctx, now)
	default:
		err = &ClaudeFailureError{Kind: ClaudeFailureSpawn}
	}
	if !hasRefreshBudget(ctx, refreshWriteAndCleanup) {
		return false, nil
	}
	if err != nil || len(usage.Windows) == 0 {
		at := s.now()
		f := UsageFailure{Trigger: trigger, Reason: claudeFailureReason(err), FailureObservedAt: at}
		var parse *ProseParseError
		if errors.As(err, &parse) {
			f.Detail = parse.Detail
		}
		if trigger == TriggerBackground {
			prior := u.Envelope
			if identity.Invalidation != "" {
				prior = EnvelopeRecord{}
			}
			f.FailureAt = at
			f.BackoffUntil = at.Add(s.nextBackoff(prior, state.hasUsage))
		}
		out, err := s.Store.CommitClaudeUsageFailure(ctx, c, current.Generation, f)
		return err == nil && out == WriteCommitted, nil
	}
	out, err := s.Store.CommitClaudeUsage(ctx, c, current.Generation, usage, identity)
	if err != nil || out != WriteCommitted {
		return false, nil
	}
	if canPublishGate && hasRefreshBudget(ctx, refreshWriteAndCleanup) {
		if stamp, ok := refreshBootStamp(); ok && (current.Gate == CaptureBlocked || stamp.Boot != current.OpenedAt.Boot) {
			_, err = s.Store.PublishClaudeCaptureGate(ctx, c, current.Generation, stamp)
			if err != nil {
				return false, nil
			}
		}
	}
	return true, nil
}
