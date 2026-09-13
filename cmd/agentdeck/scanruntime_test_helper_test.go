package main

import (
	"context"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/scanruntime"
)

// waitForBackgroundScan runs before t.TempDir cleanup. Scoped command tests
// intentionally return before the detached round's unrelated work, so their
// isolated state must be handed through the documented maintenance barrier.
func waitForBackgroundScan(t *testing.T, stateRoot string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := scanruntime.WithMaintenance(ctx, stateRoot, 5*time.Second, func(context.Context) error { return nil }); err != nil {
			t.Errorf("wait for background scan: %v", err)
		}
	})
}
