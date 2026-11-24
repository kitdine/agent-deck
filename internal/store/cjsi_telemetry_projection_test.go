package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
)

func TestCJSIOTelCombinedSettlementIsAtomic(t *testing.T) {
	ctx := t.Context()
	core := store.OpenCJSI37ForTelemetryTest(t)
	index, err := store.OpenSessions(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = index.Close() })
	at := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)
	received := at.Add(-telemetry.SettleAfter - time.Hour).Format(time.RFC3339Nano)
	in, out := int64(10), int64(1)
	o := store.TelemetryObservation{Key: "a", Source: "import", Producer: "codex_exec", ProducerVersion: "1", Originator: "codex_exec", Transport: "sse", EventName: "codex.sse_event", EventKind: "response.completed", ThreadID: "thread", RecordAt: received, ReceivedAt: received,
		State: "pending", Metered: true, InputTokens: &in, OutputTokens: &out, SupportEntry: "codex_exec/1/codex_exec/sse"}
	if _, err := core.ApplyTelemetryObservations(ctx, []store.TelemetryObservation{o}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := core.AcceptInventory(ctx, "inv", 1, nil, at); err != nil {
		t.Fatal(err)
	}
	if err := core.FinishInventory(ctx, "inv", "complete"); err != nil {
		t.Fatal(err)
	}

	prepared := 0
	prepare := func(context.Context) (bool, error) { prepared++; return false, nil }
	changed, err := telemetry.SettleAndProject(ctx, core, index, at, prepare)
	if changed || prepared != 0 || !errors.Is(err, store.ErrCJSIOTelAuthorityRequired) {
		t.Fatalf("combined call changed=%t prepared=%d err=%v", changed, prepared, err)
	}
	var state string
	var events, links, markers, sources int
	if err := core.DB.QueryRowContext(ctx, `SELECT state,(SELECT count(*) FROM usage_events),(SELECT count(*) FROM telemetry_contributions),(SELECT count(*) FROM telemetry_sessions) FROM telemetry_observations WHERE observation_key='a'`).Scan(&state, &events, &links, &markers); err != nil {
		t.Fatal(err)
	}
	if err := index.DB.QueryRowContext(ctx, "SELECT count(*) FROM session_sources").Scan(&sources); err != nil {
		t.Fatal(err)
	}
	if state != "pending" || events+links+markers+sources != 0 {
		t.Fatalf("refusal left state=%s events=%d links=%d markers=%d index=%d", state, events, links, markers, sources)
	}
	changed, err = telemetry.SettleAndProject(ctx, core, nil, at, prepare)
	if !changed || prepared != 1 || err != nil {
		t.Fatalf("usage-only changed=%t prepared=%d err=%v", changed, prepared, err)
	}
}
