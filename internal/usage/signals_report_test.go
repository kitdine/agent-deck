package usage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSignalsReportCoversAllFamiliesAndActivityFiltering(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	seedSignalCostPrices(t, database)
	debugFile := filepath.Join(root, "debug.go")

	writeSource(t, filepath.Join(home, ".claude", "projects", "signals.jsonl"),
		userLine("debug", "2026-08-27T00:00:00Z", "fix the crash"),
		assistantLine("debug", "2026-08-27T00:00:01Z", "m-debug", pricedFixtureModel, 200000, "Edit", debugFile),
		userLine("coding", "2026-08-27T00:01:00Z", "implement the cache"),
		assistantLine("coding", "2026-08-27T00:01:01Z", "m-coding", pricedFixtureModel, 100000, "Bash", ""),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	from, to := signalCostWindow()
	report, err := service.Signals(ctx, SignalOptions{
		Period:     "7d",
		From:       from,
		To:         to,
		Client:     "claude",
		IncludeSub: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Period != "7d" || report.Client != "claude" {
		t.Fatalf("scope = %q/%q, want 7d/claude", report.Period, report.Client)
	}
	if report.Activity == nil || !report.Activity.Available || len(report.Activity.Kinds) != 4 {
		t.Fatalf("activity = %+v, want four available categories", report.Activity)
	}
	if report.Workflow == nil || !report.Workflow.Available || report.Workflow.EditsPerSession == nil || *report.Workflow.EditsPerSession != 0.5 {
		t.Fatalf("workflow = %+v, want one edit over two sessions", report.Workflow)
	}
	if report.Tooling == nil || !report.Tooling.Available || report.Tooling.Calls != 2 || report.Tooling.Groups != 2 {
		t.Fatalf("tooling = %+v, want two calls across edit and bash", report.Tooling)
	}

	filtered, err := service.Signals(ctx, SignalOptions{
		Period:     "7d",
		From:       from,
		To:         to,
		Client:     "claude",
		Activity:   "debugging",
		IncludeSub: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Activity == nil || len(filtered.Activity.Kinds) != 1 || filtered.Activity.Kinds[0].Kind != "debugging" || filtered.Activity.Kinds[0].Share != 100 {
		t.Fatalf("filtered activity = %+v, want debugging renormalized to 100%%", filtered.Activity)
	}
	if len(filtered.Activity.Kinds[0].Sub) != 2 || filtered.Activity.Kinds[0].Sub[1].Kind != "repair" || filtered.Activity.Kinds[0].Sub[1].Share != 100 {
		t.Fatalf("filtered subcategories = %+v, want repair at 100%%", filtered.Activity.Kinds[0].Sub)
	}
	if filtered.Tooling == nil || filtered.Tooling.Calls != 1 || len(filtered.Tooling.Rows) != 1 || filtered.Tooling.Rows[0].Kind != "edit" {
		t.Fatalf("filtered tooling = %+v, want only the debugging turn's edit", filtered.Tooling)
	}
	if filtered.Workflow == nil || filtered.Workflow.EditsPerSession == nil || *filtered.Workflow.EditsPerSession != 1 {
		t.Fatalf("filtered workflow = %+v, want one edit in one selected session", filtered.Workflow)
	}
}

func TestSignalsReportDistinguishesEmptyScopeAndSessionSummary(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	service, database, home := newSignalService(t, root)
	seedSignalCostPrices(t, database)
	target := filepath.Join(root, "cache.go")
	writeSource(t, filepath.Join(home, ".claude", "projects", "session.jsonl"),
		userLine("s", "2026-08-27T00:00:00Z", "implement the cache"),
		assistantLine("s", "2026-08-27T00:00:04Z", "m1", pricedFixtureModel, 100000, "Edit", target),
	)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	empty, err := service.Signals(ctx, SignalOptions{
		Period: "today",
		From:   time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC),
		To:     time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Activity == nil || empty.Activity.Available || empty.Workflow == nil || empty.Workflow.Available || empty.Tooling == nil || empty.Tooling.Available {
		t.Fatalf("empty report = %+v, want all three families explicitly unavailable", empty)
	}

	summary, found, err := service.SessionSignals(ctx, "claude", "s")
	if err != nil {
		t.Fatal(err)
	}
	if !found || summary.Kind != "coding" || summary.ToolCalls != 1 || summary.FilesTouched == nil || *summary.FilesTouched != 1 || summary.FirstEditSeconds == nil || *summary.FirstEditSeconds != 4 {
		t.Fatalf("session signals = %+v found=%v, want coding/1 call/1 file/4s", summary, found)
	}
	if _, found, err = service.SessionSignals(ctx, "claude", "missing"); err != nil || found {
		t.Fatalf("missing session found=%v err=%v, want clean omission", found, err)
	}
}
