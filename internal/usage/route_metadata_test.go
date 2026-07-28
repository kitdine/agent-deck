package usage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// newRouteMetadataFixture records two selections for one provider — the first
// direct, the second through its wrapper — plus one session and event under
// each, so a single provider row spans both routes.
func newRouteMetadataFixture(t *testing.T, providerName string) (context.Context, *store.Store) {
	t.Helper()
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	for _, selection := range []store.Selection{
		{
			Client:             "codex",
			ProviderName:       providerName,
			EndpointSnapshot:   "https://provider.example",
			MultiplierSnapshot: "1",
			SelectedAt:         time.Date(2026, 7, 20, 1, 0, 0, 0, time.UTC),
		},
		{
			Client:             "codex",
			ProviderName:       providerName,
			EndpointSnapshot:   "https://wrapper.example",
			MultiplierSnapshot: "1",
			ViaWrapper:         true,
			SelectedAt:         time.Date(2026, 7, 20, 3, 0, 0, 0, time.UTC),
		},
	} {
		if err = database.RecordSelection(ctx, selection); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = database.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES
 ('codex','direct-session','2026-07-20T02:00:00Z','2026-07-20T02:00:00Z'),
 ('codex','wrapped-session','2026-07-20T04:00:00Z','2026-07-20T04:00:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,source_path,source_offset) VALUES
 ('direct','codex','direct-session','direct','2026-07-20T02:00:00Z','gpt-test',10,0,'fixture',0),
 ('wrapped','codex','wrapped-session','wrapped','2026-07-20T04:00:00Z','gpt-test',20,0,'fixture',1)`); err != nil {
		t.Fatal(err)
	}
	return ctx, database
}

func routeMetadataStats(t *testing.T, ctx context.Context, database *store.Store, options StatsOptions) StatsReport {
	t.Helper()
	options.From = time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	options.To = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	options.GroupBy, options.Metric, options.Location, options.Timezone = "day", "tokens", time.UTC, "UTC"
	report, err := New(database, "").Stats(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// TestStatsReportsWrapperRouteWithoutSplittingTheProviderDimension is the
// task's core contract: the route is reported metadata, so both routes stay in
// one row keyed by provider name and only the count distinguishes them.
func TestStatsReportsWrapperRouteWithoutSplittingTheProviderDimension(t *testing.T) {
	ctx, database := newRouteMetadataFixture(t, "relay")
	report := routeMetadataStats(t, ctx, database, StatsOptions{})

	if len(report.Providers) != 1 {
		t.Fatalf("providers = %#v, want one row spanning both routes", report.Providers)
	}
	row := report.Providers[0]
	if row.Name != "relay" || row.Client != "codex" {
		t.Fatalf("provider row = %#v", row)
	}
	if row.Events != 2 || row.Tokens != 30 {
		t.Fatalf("provider row lost an event to the route: %#v", row)
	}
	if row.WrapperEvents != 1 {
		t.Fatalf("wrapper_events = %d, want only the wrapped event", row.WrapperEvents)
	}
}

// TestStatsProviderFilterSelectsBothRoutes pins that the route never becomes
// part of the filter key: --provider names an account, not a path to it.
func TestStatsProviderFilterSelectsBothRoutes(t *testing.T) {
	ctx, database := newRouteMetadataFixture(t, "relay")
	report := routeMetadataStats(t, ctx, database, StatsOptions{Provider: "relay"})

	if len(report.Providers) != 1 || report.Providers[0].Events != 2 || report.Providers[0].WrapperEvents != 1 {
		t.Fatalf("filtered providers = %#v", report.Providers)
	}
	if report.Totals.Events != 2 || report.Totals.Tokens != 30 {
		t.Fatalf("filtered totals = %#v, want both routes", report.Totals)
	}
}

// TestStatsKeepsWrappedOfficialUnderOfficialAtMultiplierOne covers the
// subscription case the plan calls out: a proxy in front of the built-in
// provider must not appear as a second provider or change the multiplier.
func TestStatsKeepsWrappedOfficialUnderOfficialAtMultiplierOne(t *testing.T) {
	ctx, database := newRouteMetadataFixture(t, "official")
	report := routeMetadataStats(t, ctx, database, StatsOptions{})

	if len(report.Providers) != 1 || report.Providers[0].Name != "official" {
		t.Fatalf("providers = %#v, want one official row", report.Providers)
	}
	if report.Providers[0].WrapperEvents != 1 || report.Providers[0].Events != 2 {
		t.Fatalf("official row = %#v", report.Providers[0])
	}
	snapshot, err := database.CurrentProviderSnapshot(ctx, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.ViaWrapper || snapshot.Multiplier != "1" {
		t.Fatalf("wrapped official selection = %#v, want multiplier 1", snapshot)
	}
}

// TestStatsWithoutAWrapperOmitsTheRouteFieldEntirely is the acceptance
// criterion: with no wrapper anywhere, every existing contract is untouched,
// which for JSON means the field is absent rather than zero.
func TestStatsWithoutAWrapperOmitsTheRouteFieldEntirely(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.RecordSelection(ctx, store.Selection{
		Client:             "codex",
		ProviderName:       "relay",
		EndpointSnapshot:   "https://provider.example",
		MultiplierSnapshot: "1",
		SelectedAt:         time.Date(2026, 7, 20, 1, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES
 ('codex','direct-session','2026-07-20T02:00:00Z','2026-07-20T02:00:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,source_path,source_offset) VALUES
 ('direct','codex','direct-session','direct','2026-07-20T02:00:00Z','gpt-test',10,0,'fixture',0)`); err != nil {
		t.Fatal(err)
	}

	report := routeMetadataStats(t, ctx, database, StatsOptions{})
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Providers []map[string]any `json:"providers"`
		Clients   []map[string]any `json:"clients"`
		Models    []map[string]any `json:"models"`
	}
	if err = json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	for name, rows := range map[string][]map[string]any{"providers": payload.Providers, "clients": payload.Clients, "models": payload.Models} {
		if len(rows) == 0 {
			t.Fatalf("%s dimension is empty: %s", name, encoded)
		}
		for _, row := range rows {
			if _, present := row["wrapper_events"]; present {
				t.Fatalf("%s row carries wrapper_events with no wrapper in play: %s", name, encoded)
			}
		}
	}
}

// TestStatsReadsAnExactEventsRouteFromItsRunStartNotItsSessionStart covers a
// session that spans a route change on one provider. The run pins the
// provider, so the route must come from the same instant; reading it from the
// session start would report the route the session opened with, which is
// wrong in both directions.
func TestStatsReadsAnExactEventsRouteFromItsRunStartNotItsSessionStart(t *testing.T) {
	for _, test := range []struct {
		name                string
		sessionRouteWrapped bool
		runRouteWrapped     bool
		wantWrapperEvents   int64
	}{
		{name: "session direct, run wrapped", sessionRouteWrapped: false, runRouteWrapped: true, wantWrapperEvents: 1},
		{name: "session wrapped, run direct", sessionRouteWrapped: true, runRouteWrapped: false, wantWrapperEvents: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			for _, selection := range []store.Selection{
				{
					Client: "codex", ProviderName: "relay", MultiplierSnapshot: "1",
					ViaWrapper: test.sessionRouteWrapped,
					SelectedAt: time.Date(2026, 7, 20, 1, 0, 0, 0, time.UTC),
				},
				{
					Client: "codex", ProviderName: "relay", MultiplierSnapshot: "1",
					ViaWrapper: test.runRouteWrapped,
					SelectedAt: time.Date(2026, 7, 20, 2, 30, 0, 0, time.UTC),
				},
			} {
				if err = database.RecordSelection(ctx, selection); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = database.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES
 ('codex','spanning-session','2026-07-20T02:00:00Z','2026-07-20T03:00:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,source_path,source_offset) VALUES
 ('spanning','codex','spanning-session','spanning','2026-07-20T03:00:00Z','gpt-test',10,0,'fixture',0);
INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact) VALUES
 (1,'codex','relay','1','2026-07-20T02:59:00Z','2026-07-20T03:01:00Z',1);
INSERT INTO usage_run_bindings(event_key,run_id) VALUES ('spanning',1)`); err != nil {
				t.Fatal(err)
			}

			report := routeMetadataStats(t, ctx, database, StatsOptions{})
			if len(report.Providers) != 1 || report.Providers[0].Name != "relay" {
				t.Fatalf("providers = %#v", report.Providers)
			}
			if report.Providers[0].WrapperEvents != test.wantWrapperEvents {
				t.Fatalf("wrapper_events = %d, want %d (the route in effect when the run pinned its provider)", report.Providers[0].WrapperEvents, test.wantWrapperEvents)
			}
		})
	}
}

// TestStatsLeavesTheRouteUnreportedWhenTheRunAndSnapshotDisagree pins the
// deliberate under-report: an exact run records its provider but not its
// route, so a session-start snapshot naming a different provider must not
// lend its route to the run's provider.
func TestStatsLeavesTheRouteUnreportedWhenTheRunAndSnapshotDisagree(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.RecordSelection(ctx, store.Selection{
		Client:             "codex",
		ProviderName:       "wrapped-relay",
		EndpointSnapshot:   "https://wrapper.example",
		MultiplierSnapshot: "1",
		ViaWrapper:         true,
		SelectedAt:         time.Date(2026, 7, 20, 1, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES
 ('codex','exact-session','2026-07-20T02:00:00Z','2026-07-20T02:00:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,source_path,source_offset) VALUES
 ('exact','codex','exact-session','exact','2026-07-20T02:00:00Z','gpt-test',10,0,'fixture',0);
INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact) VALUES
 (1,'codex','other-relay','1','2026-07-20T01:59:00Z','2026-07-20T02:01:00Z',1);
INSERT INTO usage_run_bindings(event_key,run_id) VALUES ('exact',1)`); err != nil {
		t.Fatal(err)
	}

	report := routeMetadataStats(t, ctx, database, StatsOptions{})
	if len(report.Providers) != 1 || report.Providers[0].Name != "other-relay" {
		t.Fatalf("providers = %#v, want the run's own provider", report.Providers)
	}
	if report.Providers[0].WrapperEvents != 0 {
		t.Fatalf("route borrowed from a snapshot naming a different provider: %#v", report.Providers[0])
	}
}
