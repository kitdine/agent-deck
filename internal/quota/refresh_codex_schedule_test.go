package quota

import (
	"context"
	"testing"
	"time"
)

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
