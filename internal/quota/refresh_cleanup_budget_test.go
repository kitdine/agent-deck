package quota

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func TestRefreshLockDeadlineReservesWriteCleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		ctx, cancel := context.WithTimeout(t.Context(), 37*time.Second)
		defer cancel()
		if got := RefreshLockDeadline(ctx).Sub(now); got != 9400*time.Millisecond {
			t.Fatalf("open and lock budget = %s, want 9.4s", got)
		}
		if refreshPreparation != 3100*time.Millisecond || refreshProbes != 22*time.Second ||
			RefreshReportBudget != 2500*time.Millisecond {
			t.Fatalf("phase budgets: preparation=%s probes=%s report=%s", refreshPreparation, refreshProbes, RefreshReportBudget)
		}
		time.Sleep(9400 * time.Millisecond)
		if !hasRefreshBudget(ctx, refreshPreparation+refreshProbes) {
			t.Fatal("full period has room at the lock cutoff")
		}
		time.Sleep(time.Nanosecond)
		if hasRefreshBudget(ctx, refreshPreparation+refreshProbes) {
			t.Fatal("admitted work after consuming some of its cleanup reserve")
		}
	})
}
