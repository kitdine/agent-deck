package quota

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openSchedulerTestStore(t *testing.T) *Store {
	t.Helper()
	store, _ := openTestStore(t)
	return store
}

// countingFakeCodex behaves like the codex_test.go happy path (returning a
// bare, window-less result so the scheduler's own persistence path is what's
// under test, not codex.go's window mapping) while appending one line to
// counterPath per invocation, so a test can assert how many times the
// scheduler actually spawned the fake process.
func countingFakeCodex(t *testing.T, counterPath string) {
	t.Helper()
	withFakeCodex(t, `echo x >> `+shellQuotePath(counterPath)+`
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  elif [ "$n" = "3" ]; then
    echo '{"id":2,"result":{"accountId":"acct_fake","rateLimits":{"planType":"pro"}}}'
  fi
done
`)
}

func countingFailingFakeCodex(t *testing.T, counterPath string) {
	t.Helper()
	withFakeCodex(t, `echo x >> `+shellQuotePath(counterPath)+`
exit 1
`)
}

func countingFakeClaude(t *testing.T, counterPath string) {
	t.Helper()
	withFakeClaude(t, `echo x >> `+shellQuotePath(counterPath)+`
printf 'Current session: 22%% used \xc2\xb7 resets Sep 9 at 1:50am (America/Los_Angeles)\n'
`)
}

func shellQuotePath(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

func invocationCount(t *testing.T, counterPath string) int {
	t.Helper()
	data, err := os.ReadFile(counterPath)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("ReadFile counter: %v", err)
	}
	return len(strings.Split(strings.TrimRight(string(data), "\n"), "\n"))
}

func TestSchedulerNotAllowedDoesNothing(t *testing.T) {
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFakeCodex(t, counter)

	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour}
	sched.Run(context.Background(), ClientCodex, TriggerBackground, false)
	sched.Run(context.Background(), ClientCodex, TriggerManual, false)

	if got := invocationCount(t, counter); got != 0 {
		t.Fatalf("subprocess invocations = %d, want 0 when the gate reports not allowed", got)
	}
	if _, ok, err := store.Envelope(context.Background(), ClientCodex); err != nil || ok {
		t.Fatalf("Envelope = (ok=%v, err=%v), want no record written", ok, err)
	}
}

func TestSchedulerNotAllowedRetainsExistingObservations(t *testing.T) {
	// C9: turning reading off must retain stored observations rather than
	// probe or discard them (requirements.md clause 2).
	store := openSchedulerTestStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct-1", WindowKey: "codex", Source: SourceCodex, ObservedAt: t1, UsedPercent: 40,
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := store.PutEnvelope(ctx, EnvelopeRecord{Client: ClientCodex, AccountID: "acct-1", Applicable: true, Source: SourceCodex, ObservedAt: t1, Plan: "pro"}); err != nil {
		t.Fatalf("PutEnvelope: %v", err)
	}

	counter := filepath.Join(t.TempDir(), "count")
	countingFakeCodex(t, counter)
	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return t1.Add(time.Hour) }}
	sched.Run(ctx, ClientCodex, TriggerBackground, false)

	if got := invocationCount(t, counter); got != 0 {
		t.Fatalf("subprocess invocations = %d, want 0", got)
	}
	windows, err := store.Windows(ctx, ClientCodex)
	if err != nil || len(windows) != 1 || windows[0].UsedPercent != 40 {
		t.Fatalf("Windows = (%+v, %v), want the one retained window untouched", windows, err)
	}
	env, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil || !ok || env.Plan != "pro" {
		t.Fatalf("Envelope = (%+v, %v, %v), want the retained envelope untouched", env, ok, err)
	}
}

func TestSchedulerBackgroundFirstProbeIsAlwaysDue(t *testing.T) {
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFakeCodex(t, counter)

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return now }}
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)

	if got := invocationCount(t, counter); got != 1 {
		t.Fatalf("subprocess invocations = %d, want 1 for a client never probed before", got)
	}
	env, ok, err := store.Envelope(context.Background(), ClientCodex)
	if err != nil || !ok || !env.Applicable || !env.ObservedAt.Equal(now) {
		t.Fatalf("Envelope = (%+v, %v, %v), want an applicable envelope observed at %v", env, ok, err, now)
	}
}

func TestSchedulerBackgroundRespectsInterval(t *testing.T) {
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFakeCodex(t, counter)

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return now }}
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)
	if got := invocationCount(t, counter); got != 1 {
		t.Fatalf("invocations after first probe = %d, want 1", got)
	}

	sched.Now = func() time.Time { return now.Add(4 * time.Minute) }
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)
	if got := invocationCount(t, counter); got != 1 {
		t.Fatalf("invocations before the interval elapses = %d, want still 1", got)
	}

	sched.Now = func() time.Time { return now.Add(5 * time.Minute) }
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)
	if got := invocationCount(t, counter); got != 2 {
		t.Fatalf("invocations once the interval has elapsed = %d, want 2", got)
	}
}

func TestSchedulerManualBypassesInterval(t *testing.T) {
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFakeCodex(t, counter)

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return now }}
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)

	sched.Now = func() time.Time { return now.Add(time.Second) }
	sched.Run(context.Background(), ClientCodex, TriggerManual, true)

	if got := invocationCount(t, counter); got != 2 {
		t.Fatalf("invocations after an immediate manual refresh = %d, want 2 (the interval must not gate it)", got)
	}
}

func TestSchedulerManualBypassesBackoff(t *testing.T) {
	// GS-R1-F2: architecture.md C9 names only the reading switch and the
	// provider gate as still applying to a manual refresh; an earlier version
	// also applied backoff to manual, which could lock a user-requested retry
	// out for up to an hour after a single transient failure.
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFailingFakeCodex(t, counter)

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return now }}
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)
	if got := invocationCount(t, counter); got != 1 {
		t.Fatalf("invocations after the first failure = %d, want 1", got)
	}

	sched.Now = func() time.Time { return now.Add(time.Second) }
	sched.Run(context.Background(), ClientCodex, TriggerManual, true)
	if got := invocationCount(t, counter); got != 2 {
		t.Fatalf("invocations for a manual refresh inside the backoff window = %d, want 2 (manual bypasses backoff)", got)
	}
}

func TestSchedulerManualFailureDoesNotAdvanceBackgroundBackoff(t *testing.T) {
	// GS-R2-F1: manual already bypasses backoff as a consumer (the test
	// above); this asserts it is also not a producer. Without the fix, each
	// manual retry's own failure doubled the same backoff chain a background
	// trigger reads, so a few quick manual clicks pushed the next background
	// probe out by as much as MaxBackoff.
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFailingFakeCodex(t, counter)

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	interval := 5 * time.Minute
	sched := Scheduler{Store: store, Interval: interval, MaxBackoff: time.Hour, Now: func() time.Time { return now }}
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true) // background failure: backoff = 5m
	env, _, err := store.Envelope(context.Background(), ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	originalBackoffUntil := env.BackoffUntil
	if got := originalBackoffUntil.Sub(env.FailureAt); got != interval {
		t.Fatalf("backoff after the background failure = %v, want %v", got, interval)
	}

	for i := 1; i <= 4; i++ {
		sched.Now = func() time.Time { return now.Add(time.Duration(i) * 30 * time.Second) }
		sched.Run(context.Background(), ClientCodex, TriggerManual, true)
	}
	if got := invocationCount(t, counter); got != 5 {
		t.Fatalf("invocations after 4 manual retries = %d, want 5 (1 background + 4 manual)", got)
	}
	env, _, err = store.Envelope(context.Background(), ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if !env.BackoffUntil.Equal(originalBackoffUntil) {
		t.Fatalf("BackoffUntil after 4 manual failures = %v, want unchanged from %v (manual failures must not advance the background backoff chain)", env.BackoffUntil, originalBackoffUntil)
	}

	// A background attempt at the ORIGINAL deadline must run, unaffected by
	// the manual retries in between.
	sched.Now = func() time.Time { return originalBackoffUntil }
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)
	if got := invocationCount(t, counter); got != 6 {
		t.Fatalf("invocations at the original background deadline = %d, want 6", got)
	}
}

func TestSchedulerManualFailureWithNoPriorFailureStoresNoBackoffChainInstant(t *testing.T) {
	// GS-R4-F1: a manual failure inherits FailureAt from the envelope
	// (GS-R3-F1). With nothing to inherit — never probed, or the last probe
	// succeeded and cleared it — that value is zero and must stay absent,
	// not be persisted as 0001-01-01 beside a real Failure reason.
	//
	// WC-R2-F1: FailureObservedAt is not part of that backoff-chain pair — a
	// manual failure always writes its own real attempt instant there, even
	// though FailureAt/BackoffUntil stay empty.
	ctx := context.Background()
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	t.Run("never probed", func(t *testing.T) {
		store, db := openTestStore(t)
		countingFailingFakeCodex(t, filepath.Join(t.TempDir(), "count"))
		sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return t1 }}
		sched.Run(ctx, ClientCodex, TriggerManual, true)

		env, ok, err := store.Envelope(ctx, ClientCodex)
		if err != nil || !ok || env.Failure != ReasonProbeFailed {
			t.Fatalf("Envelope = (%+v, %v, %v), want Failure=probe_failed", env, ok, err)
		}
		if failureAt, backoffUntil, failureObservedAt := rawEnvelopeInstants(t, db, ClientCodex); failureAt != "" || backoffUntil != "" || failureObservedAt == "" {
			t.Fatalf("stored failure_at=%q backoff_until=%q failure_observed_at=%q, want the first two empty and the third the attempt instant", failureAt, backoffUntil, failureObservedAt)
		}
		if !env.FailureObservedAt.Equal(t1) {
			t.Fatalf("FailureObservedAt = %v, want the manual attempt instant %v", env.FailureObservedAt, t1)
		}
	})

	t.Run("after a success", func(t *testing.T) {
		store, db := openTestStore(t)
		counter := filepath.Join(t.TempDir(), "count")
		countingFakeCodex(t, counter)
		sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return t1 }}
		sched.Run(ctx, ClientCodex, TriggerBackground, true)

		manualAt := t1.Add(time.Hour)
		countingFailingFakeCodex(t, counter)
		sched.Now = func() time.Time { return manualAt }
		sched.Run(ctx, ClientCodex, TriggerManual, true)

		env, ok, err := store.Envelope(ctx, ClientCodex)
		if err != nil || !ok || env.Failure != ReasonProbeFailed {
			t.Fatalf("Envelope = (%+v, %v, %v), want Failure=probe_failed", env, ok, err)
		}
		if failureAt, backoffUntil, _ := rawEnvelopeInstants(t, db, ClientCodex); failureAt != "" || backoffUntil != "" {
			t.Fatalf("stored failure_at=%q backoff_until=%q, want both empty", failureAt, backoffUntil)
		}
		if !env.FailureObservedAt.Equal(manualAt) {
			t.Fatalf("FailureObservedAt = %v, want the manual attempt instant %v", env.FailureObservedAt, manualAt)
		}
		if !env.ObservedAt.Equal(t1) {
			t.Fatalf("ObservedAt = %v, want the last success %v retained", env.ObservedAt, t1)
		}
	})
}

func TestSchedulerManualFailureDoesNotCorruptBackoffStepDerivation(t *testing.T) {
	// GS-R3-F1: an earlier fix for GS-R2-F1 left BackoffUntil untouched by a
	// manual failure but still moved FailureAt forward. nextBackoff derives
	// the prior step's length from BackoffUntil.Sub(FailureAt), so that alone
	// shrank (and could even invert) the derived step, corrupting every
	// later background failure's doubling.
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFailingFakeCodex(t, counter)

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	interval := 5 * time.Minute
	sched := Scheduler{Store: store, Interval: interval, MaxBackoff: time.Hour, Now: func() time.Time { return now }}

	sched.Run(context.Background(), ClientCodex, TriggerBackground, true) // 1st background failure: step 5m
	env, _, err := store.Envelope(context.Background(), ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	firstBackoffUntil := env.BackoffUntil

	// A manual retry lands inside that backoff window and also fails.
	sched.Now = func() time.Time { return now.Add(2 * time.Minute) }
	sched.Run(context.Background(), ClientCodex, TriggerManual, true)

	// The next background failure, at the ORIGINAL deadline, must still
	// double from the 1st step (5m -> 10m), unaffected by the manual retry.
	sched.Now = func() time.Time { return firstBackoffUntil }
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)
	env, _, err = store.Envelope(context.Background(), ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if got := env.BackoffUntil.Sub(env.FailureAt); got != 2*interval {
		t.Fatalf("2nd background failure's step = %v, want %v (doubled from the 1st, unaffected by the manual retry)", got, 2*interval)
	}
	if got := invocationCount(t, counter); got != 3 {
		t.Fatalf("invocations = %d, want 3 (2 background + 1 manual)", got)
	}
}

func TestSchedulerRepeatedManualRetriesBetweenBackgroundFailuresStillDoubleCorrectly(t *testing.T) {
	// GS-R3-F1's REPRO R3/C: with one manual retry after every background
	// failure, the background step must still climb 5m, 10m, 20m, 40m, 1h —
	// not flatten to 5m every cycle.
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFailingFakeCodex(t, counter)

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	interval := 5 * time.Minute
	sched := Scheduler{Store: store, Interval: interval, MaxBackoff: time.Hour, Now: func() time.Time { return now }}

	expectedSteps := []time.Duration{5 * time.Minute, 10 * time.Minute, 20 * time.Minute, 40 * time.Minute, time.Hour}
	for cycle, want := range expectedSteps {
		deadline := now
		sched.Now = func() time.Time { return deadline }
		sched.Run(context.Background(), ClientCodex, TriggerBackground, true)
		env, _, err := store.Envelope(context.Background(), ClientCodex)
		if err != nil {
			t.Fatalf("Envelope: %v", err)
		}
		if got := env.BackoffUntil.Sub(env.FailureAt); got != want {
			t.Fatalf("background failure %d step = %v, want %v", cycle+1, got, want)
		}

		manualAt := deadline.Add(time.Minute)
		sched.Now = func() time.Time { return manualAt }
		sched.Run(context.Background(), ClientCodex, TriggerManual, true)

		now = env.BackoffUntil
	}
	if got := invocationCount(t, counter); got != 2*len(expectedSteps) {
		t.Fatalf("invocations = %d, want %d (one background + one manual per cycle)", got, 2*len(expectedSteps))
	}
}

func TestSchedulerBackoffDoublesOnConsecutiveFailuresAndCapsAtMax(t *testing.T) {
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFailingFakeCodex(t, counter)

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	interval := 5 * time.Minute
	maxBackoff := 20 * time.Minute
	sched := Scheduler{Store: store, Interval: interval, MaxBackoff: maxBackoff, Now: func() time.Time { return now }}

	sched.Run(context.Background(), ClientCodex, TriggerBackground, true) // 1st failure: backoff = interval (5m)
	env, _, err := store.Envelope(context.Background(), ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if got := env.BackoffUntil.Sub(env.FailureAt); got != interval {
		t.Fatalf("backoff after 1st failure = %v, want %v", got, interval)
	}

	now = env.BackoffUntil
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true) // 2nd failure: backoff = 10m
	env, _, err = store.Envelope(context.Background(), ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if got := env.BackoffUntil.Sub(env.FailureAt); got != 2*interval {
		t.Fatalf("backoff after 2nd failure = %v, want %v", got, 2*interval)
	}

	now = env.BackoffUntil
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true) // 3rd failure: would be 20m, at the cap
	env, _, err = store.Envelope(context.Background(), ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if got := env.BackoffUntil.Sub(env.FailureAt); got != maxBackoff {
		t.Fatalf("backoff after 3rd failure = %v, want the cap %v", got, maxBackoff)
	}

	now = env.BackoffUntil
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true) // 4th failure: stays at the cap
	env, _, err = store.Envelope(context.Background(), ClientCodex)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if got := env.BackoffUntil.Sub(env.FailureAt); got != maxBackoff {
		t.Fatalf("backoff after 4th failure = %v, want it to stay at the cap %v", got, maxBackoff)
	}

	if got := invocationCount(t, counter); got != 4 {
		t.Fatalf("invocations = %d, want 4 (one per failure, each attempted exactly at its backoff deadline)", got)
	}
}

func TestSchedulerSuccessResetsBackoff(t *testing.T) {
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFailingFakeCodex(t, counter)

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	interval := 5 * time.Minute
	sched := Scheduler{Store: store, Interval: interval, MaxBackoff: time.Hour, Now: func() time.Time { return now }}
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true) // failure: backoff = 5m
	env, _, _ := store.Envelope(context.Background(), ClientCodex)
	if got := env.BackoffUntil.Sub(env.FailureAt); got != interval {
		t.Fatalf("backoff after 1st failure = %v, want %v", got, interval)
	}

	now = env.BackoffUntil
	countingFakeCodex(t, counter) // now succeeds
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)
	env, ok, err := store.Envelope(context.Background(), ClientCodex)
	if err != nil || !ok || !env.BackoffUntil.IsZero() {
		t.Fatalf("Envelope after success = (%+v, %v, %v), want BackoffUntil cleared", env, ok, err)
	}

	now = env.ObservedAt.Add(interval)
	countingFailingFakeCodex(t, counter) // fails again after a success in between
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)
	env, _, _ = store.Envelope(context.Background(), ClientCodex)
	if got := env.BackoffUntil.Sub(env.FailureAt); got != interval {
		t.Fatalf("backoff after a failure following a success = %v, want it to restart at %v, not keep doubling", got, interval)
	}
}

func TestSchedulerSingleFlightSkipsWhenAlreadyInFlight(t *testing.T) {
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFakeCodex(t, counter)

	quotaProbeInFlight.Store(ClientCodex, struct{}{})
	t.Cleanup(func() { quotaProbeInFlight.Delete(ClientCodex) })

	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour}
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)

	if got := invocationCount(t, counter); got != 0 {
		t.Fatalf("invocations while another probe is already in flight for this client = %d, want 0", got)
	}
}

func TestSchedulerSingleFlightIsPerClient(t *testing.T) {
	// Marking Codex in flight must not block Claude's own probe.
	store := openSchedulerTestStore(t)
	counter := filepath.Join(t.TempDir(), "count")
	countingFakeClaude(t, counter)

	quotaProbeInFlight.Store(ClientCodex, struct{}{})
	t.Cleanup(func() { quotaProbeInFlight.Delete(ClientCodex) })

	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour}
	sched.Run(context.Background(), ClientClaude, TriggerBackground, true)

	if got := invocationCount(t, counter); got != 1 {
		t.Fatalf("Claude invocations while only Codex is in flight = %d, want 1", got)
	}
}

func TestSchedulerCodexMalformedResponseRecordsParseFailed(t *testing.T) {
	store := openSchedulerTestStore(t)
	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  elif [ "$n" = "3" ]; then
    echo 'not json'
  fi
done
`)

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return now }}
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)

	env, ok, err := store.Envelope(context.Background(), ClientCodex)
	if err != nil || !ok || env.Failure != ReasonParseFailed {
		t.Fatalf("Envelope = (%+v, %v, %v), want Failure=parse_failed for a malformed response", env, ok, err)
	}
}

func TestSchedulerClaudeSpawnFailureRecordsProbeFailed(t *testing.T) {
	store := openSchedulerTestStore(t)
	previous := claudeProseCommandArgs
	claudeProseCommandArgs = []string{filepath.Join(t.TempDir(), "does-not-exist")}
	t.Cleanup(func() { claudeProseCommandArgs = previous })

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return now }}
	sched.Run(context.Background(), ClientClaude, TriggerBackground, true)

	env, ok, err := store.Envelope(context.Background(), ClientClaude)
	if err != nil || !ok || env.Failure != ReasonProbeFailed {
		t.Fatalf("Envelope = (%+v, %v, %v), want Failure=probe_failed for a spawn failure", env, ok, err)
	}
}

// Codex PR #5 P1: a successful probe's Record loop only upserts the windows
// a response actually contains. Without pruning, a bucket the vendor stops
// reporting for an otherwise-unchanged account lingers under its last
// observed values forever.
func TestSchedulerSuccessfulProbeRemovesWindowsTheResponseNoLongerReports(t *testing.T) {
	store := openSchedulerTestStore(t)
	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  elif [ "$n" = "3" ]; then
    echo '{"id":2,"result":{"accountId":"acct_fake","rateLimits":{"primary":{"usedPercent":10},"secondary":{"usedPercent":20}}}}'
  fi
done
`)
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return now }}
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)

	before, err := store.Windows(context.Background(), ClientCodex)
	if err != nil || len(before) != 2 {
		t.Fatalf("Windows after first probe = (%+v, %v), want both primary and secondary", before, err)
	}

	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  elif [ "$n" = "3" ]; then
    echo '{"id":2,"result":{"accountId":"acct_fake","rateLimits":{"primary":{"usedPercent":15}}}}'
  fi
done
`)
	now = now.Add(5 * time.Minute)
	sched.Run(context.Background(), ClientCodex, TriggerBackground, true)

	after, err := store.Windows(context.Background(), ClientCodex)
	if err != nil || len(after) != 1 || after[0].UsedPercent != 15 {
		t.Fatalf("Windows after second probe = (%+v, %v), want only the still-reported primary window at 15%%", after, err)
	}
}

// Codex PR #5 third review, P1: quotaProbeInFlight's single-flight guard is
// process-local, so two `agentdeck desktop ...` processes can probe the same
// client concurrently. An earlier-started probe that happens to finish after
// a newer one already persisted its own result must not prune the newer
// windows or move the envelope's ObservedAt, plan, or account backward.
// Simulated here by seeding the store with a "newer" envelope and window
// directly (standing in for the second process's completed write) before
// running a scheduler whose own probe reports an earlier observedAt.
func TestSchedulerDiscardsProbeCompletionOlderThanStoredEnvelope(t *testing.T) {
	store := openSchedulerTestStore(t)
	ctx := context.Background()

	newer := time.Date(2026, 9, 10, 10, 5, 0, 0, time.UTC)
	if _, _, _, err := store.Record(ctx, Observation{
		Client: ClientCodex, AccountID: "acct_new", WindowKey: "codex",
		Source: SourceCodex, ObservedAt: newer, UsedPercent: 42,
	}); err != nil {
		t.Fatalf("seed newer window: %v", err)
	}
	if err := store.PutEnvelope(ctx, EnvelopeRecord{
		Client: ClientCodex, AccountID: "acct_new", Applicable: true,
		Source: SourceCodex, ObservedAt: newer,
	}); err != nil {
		t.Fatalf("seed newer envelope: %v", err)
	}

	older := newer.Add(-5 * time.Minute)
	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  elif [ "$n" = "3" ]; then
    echo '{"id":2,"result":{"accountId":"acct_old","rateLimits":{"primary":{"usedPercent":99}}}}'
  fi
done
`)
	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return older }}
	sched.Run(ctx, ClientCodex, TriggerManual, true)

	env, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil || !ok || !env.ObservedAt.Equal(newer) {
		t.Fatalf("Envelope after a stale probe completion = (%+v, %v, %v), want the newer envelope (observedAt=%v) left untouched", env, ok, err, newer)
	}
	windows, err := store.Windows(ctx, ClientCodex)
	if err != nil || len(windows) != 1 || windows[0].UsedPercent != 42 {
		t.Fatalf("Windows after a stale probe completion = (%+v, %v), want only the newer probe's untouched window at 42%%", windows, err)
	}
}

// Codex PR #5 seventh review, P2: a Store.Record failure for one window (a
// corrupt stored observed_reset_at, here) was silently discarded, and the
// cycle still pruned and wrote a successful envelope -- the corrupt row was
// kept but never refreshed, while the envelope reported ObservedAt advanced
// and Failure/backoff cleared, so every later snapshot kept reading that
// client as healthy. The scheduler must abort the success path instead.
func TestSchedulerAbortsOnWindowPersistenceFailure(t *testing.T) {
	store := openSchedulerTestStore(t)
	ctx := context.Background()

	digest := accountDigest(ClientCodex, "acct_fake")
	if _, err := store.db.ExecContext(ctx, `INSERT INTO quota_windows(
			client, account_id, window_key, source, observed_at, vendor_order,
			window_minutes, window_minutes_reason, label, used_percent, resets_at,
			observed_reset_at, prior_used_percent, prior_observed_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		string(ClientCodex), digest, "codex", string(SourceCodex), "2026-09-10T09:00:00Z", 0,
		300, "", "", 10.0, "2026-09-10T10:00:00Z",
		"not-a-valid-timestamp", nil, nil,
	); err != nil {
		t.Fatalf("seed corrupt window: %v", err)
	}

	withFakeCodex(t, `
n=0
while IFS= read -r line; do
  n=$((n+1))
  if [ "$n" = "1" ]; then
    echo '{"id":1,"result":{}}'
  elif [ "$n" = "3" ]; then
    echo '{"id":2,"result":{"accountId":"acct_fake","rateLimits":{"primary":{"usedPercent":20}}}}'
  fi
done
`)
	now := time.Date(2026, 9, 10, 11, 0, 0, 0, time.UTC)
	sched := Scheduler{Store: store, Interval: 5 * time.Minute, MaxBackoff: time.Hour, Now: func() time.Time { return now }}
	sched.Run(ctx, ClientCodex, TriggerBackground, true)

	env, ok, err := store.Envelope(ctx, ClientCodex)
	if err != nil || !ok || env.Failure != ReasonProbeFailed {
		t.Fatalf("Envelope after a window persistence failure = (%+v, %v, %v), want Failure=probe_failed", env, ok, err)
	}
	if env.ObservedAt.Equal(now) {
		t.Fatal("Envelope.ObservedAt was advanced to the failed cycle's instant, want it left alone (no false success)")
	}
}
