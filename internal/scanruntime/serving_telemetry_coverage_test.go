package scanruntime

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
)

func TestCompletedRoundsRetainCoverageWithTelemetrySessions(t *testing.T) {
	for _, serving := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "serving"}[serving], func(t *testing.T) {
			ctx := context.Background()
			home := t.TempDir()
			root := filepath.Join(home, ".agentdeck")
			core, err := store.Open(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now()
			input, output, cached := int64(1000), int64(10), int64(0)
			received := now.Add(-2 * telemetry.SettleAfter)
			_, err = telemetry.Import(ctx, core, []telemetry.Record{{Producer: "codex_exec", ProducerVersion: "0.165.0", Originator: "codex_exec", Transport: "sse", EventName: "codex.sse_event", EventKind: "response.completed", ThreadID: "background", Model: "gpt-5", RecordAt: received.UTC().Format(time.RFC3339Nano), InputTokens: &input, OutputTokens: &output, CachedTokens: &cached}}, received)
			if err != nil {
				t.Fatal(err)
			}
			generation, err := core.DerivedSnapshotGeneration(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := core.AcceptInventory(ctx, "ownership-proof", generation.Epoch, nil, now.Add(-time.Minute)); err != nil {
				t.Fatal(err)
			}
			if err := core.FinishInventory(ctx, "ownership-proof", "complete"); err != nil {
				t.Fatal(err)
			}
			if settled, err := core.SettleTelemetry(ctx, now, telemetry.SettleAfter); err != nil || settled.Owned != 1 {
				t.Fatalf("ownership=%+v err=%v", settled, err)
			}
			if err := core.Close(); err != nil {
				t.Fatal(err)
			}
			writeServingSource(t, home, "jsonl-session", now, 10)
			for pass := 0; pass < 2; pass++ {
				r := newRound(home)
				r.serving = serving
				r.cacheNow = func() time.Time { return now }
				r.execute(ctx, root, false)
				result, err := r.wait(ctx, ScopeBoth)
				if err != nil || result.Usage.State != "completed" || result.Session.State != "completed" {
					t.Fatalf("pass %d result=%+v err=%v", pass, result, err)
				}
				publication, err := (desktop.Service{StateRoot: root, Home: home, Now: r.cacheNow}).ReadServingSnapshot(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if publication.Header.UsageCoverage.State != "complete" || publication.Header.SessionCoverage.State != "complete" || slices.Contains(publication.Payload.Warnings, "source_update_pending") {
					t.Fatalf("pass %d final coverage invalidated: usage=%+v session=%+v warnings=%v", pass, publication.Header.UsageCoverage, publication.Header.SessionCoverage, publication.Payload.Warnings)
				}
			}
		})
	}
}
