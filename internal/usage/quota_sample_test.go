package usage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestQuotaSampleUsagePendingTelemetry(t *testing.T) {
	ctx := context.Background()
	core, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	count := int64(1)
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	seed := func(key, thread, state string, metered bool) {
		t.Helper()
		_, err := core.ApplyTelemetryObservations(ctx, []store.TelemetryObservation{{
			Key: key, Source: "receiver", ThreadID: thread, RecordAt: now.Format(time.RFC3339Nano),
			InputTokens: &count, OutputTokens: &count, Metered: metered, State: state,
		}}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	assertPending := func(client string, want bool) {
		t.Helper()
		tx, err := core.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		pending, err := QuotaSampleUsagePending(ctx, tx, client, "selected", now)
		if err != nil || pending != want {
			t.Fatalf("%s: pending=%t, want %t: %v", client, pending, want, err)
		}
	}
	seed("other-pending", "other", "pending", true)
	seed("selected-unmetered", "selected", "capture_only", false)
	assertPending("codex", false)
	seed("selected-pending", "selected", "pending", true)
	assertPending("codex", true)
	assertPending("claude", false)
	if _, err := core.DB.ExecContext(ctx, `UPDATE telemetry_observations SET state='jsonl_owned' WHERE observation_key='selected-pending'`); err != nil {
		t.Fatal(err)
	}
	assertPending("codex", false)
}
