package quota

import (
	"context"
	"testing"
	"time"
)

func TestScheduledCodexIndependentCadence(t *testing.T) {
	ctx, r, lock := refreshFixture(t)
	t0 := time.Now().UTC()
	now := t0
	s := Scheduler{Store: r.store, Interval: time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return now }}
	identified := IdentityObservation{State: IdentityIdentified, Display: IdentityDisplay{EmailMasked: "f***e@example.test"}}
	identityOK := &identified
	identityBad := &IdentityObservation{Failure: IdentityIncomparable}
	usage := &CodexResult{AccountID: "synthetic-account", Plan: "prolite"}
	failedUsage := func() *UsageFailure { return &UsageFailure{Reason: ReasonProbeFailed} }
	step := func(offset time.Duration, trigger Trigger, id, us bool, res CodexCycleResult) clientScheduleState {
		t.Helper()
		now = t0.Add(offset)
		n := 0
		s.ProbeCodexCycle = func(_ context.Context, at time.Time, i, u bool) CodexCycleResult {
			n++
			if at != now || i != id || u != us {
				t.Errorf("at %s: due=(%v,%v), want (%v,%v)", offset, i, u, id, us)
			}
			return res
		}
		if _, _, err := s.RunRefresh(ctx, r, lock, trigger, map[Client]Reason{ClientCodex: ""}); err != nil {
			t.Fatal(err)
		}
		if (n == 1) != (id || us) || n > 1 {
			t.Fatal("wrong probe count", n)
		}
		state, err := s.scheduleState(ctx, ClientCodex)
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	minute := time.Minute
	a := step(0, TriggerBackground, true, true, CodexCycleResult{Identity: identityBad, Usage: usage})
	if a.identity.Failure != IdentityIncomparable || !a.identity.BackoffUntil.Equal(t0.Add(minute)) || !a.usage.Envelope.ObservedAt.Equal(t0) {
		t.Fatal("independent initial outcome")
	}
	step(time.Second, TriggerBackground, false, false, CodexCycleResult{})
	b := step(10*time.Second, TriggerManual, false, true, CodexCycleResult{Failure: failedUsage()})
	if b.identity.AttemptSeq != a.identity.AttemptSeq || !b.usage.Envelope.FailureAt.IsZero() || !b.usage.Envelope.BackoffUntil.IsZero() {
		t.Fatal("manual changed a protected backoff")
	}
	c := step(minute, TriggerManual, true, true, CodexCycleResult{Identity: identityBad, Usage: usage})
	if !c.identity.BackoffUntil.Equal(t0.Add(3 * minute)) {
		t.Fatal("manual identity failure did not double interval")
	}
	d := step(2*minute, TriggerBackground, false, true, CodexCycleResult{Failure: failedUsage()})
	if !d.usage.Envelope.BackoffUntil.Equal(t0.Add(3 * minute)) {
		t.Fatal("background usage failure did not start independent chain")
	}
	e := step(150*time.Second, TriggerManual, false, true, CodexCycleResult{Failure: failedUsage()})
	if e.usage.Envelope.FailureAt != d.usage.Envelope.FailureAt || e.usage.Envelope.BackoffUntil != d.usage.Envelope.BackoffUntil {
		t.Fatal("manual advanced background usage chain")
	}
	f := step(3*minute, TriggerBackground, true, true, CodexCycleResult{Identity: identityOK, Failure: failedUsage()})
	if !f.identity.BackoffUntil.IsZero() || !f.identity.BackoffBaseAt.IsZero() || f.identity.SuccessState != IdentityIdentified || !f.usage.Envelope.BackoffUntil.Equal(t0.Add(5*minute)) {
		t.Fatal("usage timeout was not independent of accepted identity")
	}
	step(4*minute, TriggerBackground, true, false, CodexCycleResult{Identity: &IdentityObservation{State: IdentitySignedOut}})
	joint := step(5*minute, TriggerBackground, true, true, CodexCycleResult{Identity: identityOK, Usage: usage, proof: codexAssociationProof{matched: true}})
	if joint.identity.AssociationSeq == 0 || joint.identity.AssociationSeq != joint.usage.Seq || !joint.usage.Envelope.BackoffUntil.IsZero() {
		t.Fatal("joint proof did not survive paired commit")
	}
	switched := step(5*minute+time.Second, TriggerManual, true, true, CodexCycleResult{Identity: &IdentityObservation{State: IdentitySignedOut}, Usage: &CodexResult{AccountID: "new-account"}})
	if switched.identity.AssociationSeq != 0 || switched.identity.SuccessState != IdentitySignedOut || switched.usage.Envelope.AccountID == joint.usage.Envelope.AccountID {
		t.Fatal("old association survived account switch/sign-out")
	}
}

func TestScheduledCodexLegacyProbeSkipsIdentityOnly(t *testing.T) {
	ctx, root, lock := refreshFixture(t)
	firstAt := time.Now().UTC()
	now := firstAt
	calls := 0
	scheduler := Scheduler{
		Store:    root.store,
		Interval: time.Minute,
		Now:      func() time.Time { return now },
		ProbeCodex: func(context.Context, time.Time, time.Duration) (CodexResult, error) {
			calls++
			return CodexResult{AccountID: "synthetic-legacy-account"}, nil
		},
	}
	gates := map[Client]Reason{ClientCodex: ""}
	if _, _, err := scheduler.RunRefresh(ctx, root, lock, TriggerBackground, gates); err != nil {
		t.Fatal(err)
	}
	before, err := scheduler.scheduleState(ctx, ClientCodex)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !before.hasUsage || !before.usage.Envelope.ObservedAt.Equal(firstAt) || before.identity.AttemptSeq != 0 {
		t.Fatal("initial legacy usage probe did not establish an independent usage cadence")
	}

	// Usage is fresh and no identity has ever been probed.
	now = firstAt.Add(time.Second)
	if _, _, err := scheduler.RunRefresh(ctx, root, lock, TriggerBackground, gates); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("identity-only due state ran the legacy usage probe: calls=%d", calls)
	}
	after, err := scheduler.scheduleState(ctx, ClientCodex)
	if err != nil {
		t.Fatal(err)
	}
	if after.identity.AttemptSeq != 0 || after.usage.Seq != before.usage.Seq {
		t.Fatal("a skipped legacy cycle advanced an attempt")
	}
}

func TestScheduledCodexWriteAndBudgetRetirement(t *testing.T) {
	ctx, r, lock := refreshFixture(t)
	now := time.Now().UTC()
	s := Scheduler{Store: r.store, Now: func() time.Time { return now }, Interval: time.Minute}
	next := func() RefreshCycle {
		c, err := r.store.BeginRefreshCycle(ctx, lock, now)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	c := next()
	calls := 0
	stale := false
	s.ProbeCodexCycle = func(context.Context, time.Time, bool, bool) CodexCycleResult {
		calls++
		if stale {
			next()
		}
		return CodexCycleResult{Identity: &IdentityObservation{State: IdentitySignedOut}, Usage: &CodexResult{AccountID: "test-account"}}
	}
	// A single side needs 10s including cleanup and reporting. A pair needs 12s.
	short, cancel := context.WithTimeout(ctx, 11*time.Second)
	defer cancel()
	s.runScheduledCodex(short, c, TriggerManual)
	if calls != 0 {
		t.Fatal("paired process launched without joint budget")
	}
	checkEmpty := func() {
		t.Helper()
		var n int
		if err := r.core.DB.QueryRow(`SELECT (SELECT count(*) FROM quota_identities)+(SELECT count(*) FROM quota_envelopes_v2)`).Scan(&n); err != nil || n != 0 {
			t.Fatal("partial/stale/retired attempt persisted", n, err)
		}
	}
	checkEmpty()
	stale = true
	s.runScheduledCodex(ctx, c, TriggerManual)
	checkEmpty()
	stale = false
	if _, err := r.core.DB.Exec(`CREATE TRIGGER reject_usage BEFORE INSERT ON quota_envelopes_v2 BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	s.runScheduledCodex(ctx, next(), TriggerManual)
	checkEmpty()
	if calls != 2 {
		t.Fatal(calls)
	}
	// With exactly the same remaining budget, a single due identity fits.
	if _, err := r.core.DB.Exec(`DROP TRIGGER reject_usage`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.core.DB.Exec(`INSERT INTO quota_envelopes_v2(client,observed_at) VALUES('codex',?)`, timeValue(now)); err != nil {
		t.Fatal(err)
	}
	c = next()
	s.ProbeCodexCycle = func(ctx context.Context, _ time.Time, id, usage bool) CodexCycleResult {
		calls++
		if !id || usage || calls != 3 {
			t.Fatal("unexpected extra probe or single-side mask")
		}
		// Re-entry for this client must not start a second process.
		s.runScheduledCodex(ctx, c, TriggerBackground)
		return CodexCycleResult{Identity: &IdentityObservation{State: IdentitySignedOut}}
	}
	s.runScheduledCodex(short, c, TriggerBackground)
	if calls != 3 {
		t.Fatal("single-sided budget or single-flight regression", calls)
	}
}
