package usage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestPresentationBuildsAllScopesAndQualityTiersFromOneBoundedRange(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err = database.RecordSelection(ctx, store.Selection{
		Client: "codex", ProviderName: "relay", MultiplierSnapshot: "1",
		SelectedAt: time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES
 ('codex','exact-session','2026-08-13T08:00:00Z','2026-08-13T08:00:00Z'),
 ('codex','estimated-session','2026-08-13T09:00:00Z','2026-08-13T09:00:00Z'),
 ('claude','unknown-session','2026-08-13T09:30:00Z','2026-08-13T09:30:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES
 ('exact','codex','exact-session','exact','2026-08-13T08:00:00Z','gpt-exact',10,1,'fixture',0),
 ('estimated','codex','estimated-session','estimated','2026-08-13T09:00:00Z','gpt-estimated',20,2,'fixture',1),
 ('unknown','claude','unknown-session','unknown','2026-08-13T09:30:00Z','claude-unknown',30,3,'fixture',2);
INSERT INTO usage_runs(id,client,provider,multiplier,started_at,ended_at,exact) VALUES
 (1,'codex','relay','1','2026-08-13T07:59:00Z','2026-08-13T08:01:00Z',1);
INSERT INTO usage_run_bindings(event_key,run_id) VALUES ('exact',1)`); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	service := New(database, "")
	service.Now = func() time.Time { return now }
	report, err := service.Presentation(ctx, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Available || len(report.Scopes) != 3 || len(report.ClientSubtotals.Items) != 6 {
		t.Fatalf("presentation shape = %#v", report)
	}
	all := report.Scopes[0]
	if all.Client != "all" || len(all.Periods.Items) != 3 || len(all.Daily.Items) != 90 ||
		all.Hourly.ThroughHour != 10 || len(all.Hourly.Items) != 11 || len(all.Rhythm.Intensities) != 168 {
		t.Fatalf("all scope shape = %#v", all)
	}
	if all.Hourly.Items[8].Hour != 8 || all.Hourly.Items[8].Value.Tokens != 11 || all.Hourly.Items[8].Value.Events != 1 {
		t.Fatalf("hour 8 = %#v, want the exact event totals", all.Hourly.Items[8])
	}
	if all.Hourly.Items[9].Hour != 9 || all.Hourly.Items[9].Value.Tokens != 55 || all.Hourly.Items[9].Value.Events != 2 {
		t.Fatalf("hour 9 = %#v, want both events in the same local-hour bucket", all.Hourly.Items[9])
	}
	rhythmIndex := 3*24 + 8
	if all.Rhythm.Tokens[rhythmIndex] != 11 || all.Rhythm.ProviderCosts[rhythmIndex] == "" {
		t.Fatalf("rhythm Thursday hour 8 = tokens %d, cost %q; want same-pass tokens and provider price", all.Rhythm.Tokens[rhythmIndex], all.Rhythm.ProviderCosts[rhythmIndex])
	}
	today := all.Periods.Items[0]
	if today.Period != "today" || today.Totals.Tokens != 66 || today.Totals.Events != 3 || today.Totals.Sessions != 3 || len(today.Models) != 3 {
		t.Fatalf("today period = %#v", today)
	}
	if got := qualityEventCounts(all.Quality.Items[0]); got != [3]int64{1, 1, 1} {
		t.Fatalf("quality event counts = %v, want [1 1 1]", got)
	}
	// quality and pricing are the Client x Period product, not current-period
	// only: every supported period carries its own record set, in periods order,
	// so a panel under a period filter reads a record rather than inventing one.
	if got := periodsOf(all.Quality.Items); !slicesEqual(got, []string{"today", "7d", "30d"}) {
		t.Fatalf("quality periods = %v, want each supported period in order", got)
	}
	if all.Quality.Items[0].Period != "today" || all.Quality.Items[0].Provider != nil {
		t.Fatalf("first quality record = %#v, want the today client-scope aggregate", all.Quality.Items[0])
	}
	if report.Summary.Counts["exact"] != 1 || report.Summary.Counts["estimated"] != 1 || report.Summary.Counts["historical"] != 1 {
		t.Fatalf("legacy summary quality counts = %#v", report.Summary.Counts)
	}
	if !all.Pricing.Available || len(all.Pricing.Items) != 3 {
		t.Fatalf("pricing = %#v, want one record per supported period", all.Pricing)
	}
	pricingToday := all.Pricing.Items[0]
	if pricingToday.Period != "today" || len(pricingToday.UnpricedIdentifiers) != 3 || pricingToday.Coverage != "0.00" {
		t.Fatalf("today pricing = %#v", pricingToday)
	}
	if got := []string{all.Pricing.Items[1].Period, all.Pricing.Items[2].Period}; !slicesEqual(got, []string{"7d", "30d"}) {
		t.Fatalf("pricing periods = %v", got)
	}
}

func periodsOf(items []PresentationQualityItem) []string {
	seen := map[string]struct{}{}
	order := make([]string, 0, 3)
	for _, item := range items {
		if _, ok := seen[item.Period]; ok {
			continue
		}
		seen[item.Period] = struct{}{}
		order = append(order, item.Period)
	}
	return order
}

func slicesEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func qualityEventCounts(item PresentationQualityItem) [3]int64 {
	var counts [3]int64
	for index, tier := range item.Tiers {
		counts[index] = tier.Value.Events
	}
	return counts
}

func TestEmptyPresentationReportUsesNonNilCollections(t *testing.T) {
	report := EmptyPresentationReport()
	if report.Available || report.Scopes == nil || report.ClientSubtotals.Items == nil {
		t.Fatalf("empty report = %#v", report)
	}
}

// PPS-F6. A concrete client with no data keeps its record, but its families
// report that nothing was supplied rather than presenting synthetic zeros as a
// measurement. `all` is an explicit scope, not a missing client, so it keeps
// reporting the measured total.
func TestEmptyConcreteClientScopeReportsUnavailableFamilies(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if _, err = database.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES
 ('codex','codex-session','2026-08-13T08:00:00Z','2026-08-13T08:00:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES
 ('only','codex','codex-session','only','2026-08-13T08:00:00Z','gpt-5',10,1,'fixture',0)`); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	service := New(database, "")
	service.Now = func() time.Time { return now }
	report, err := service.Presentation(ctx, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Scopes) != 3 {
		t.Fatalf("scopes = %d, want the three contract records", len(report.Scopes))
	}

	claude := report.Scopes[2]
	if claude.Client != "claude" {
		t.Fatalf("third scope = %q, want claude", claude.Client)
	}
	if claude.Periods.Available || claude.Daily.Available || claude.Hourly.Available || claude.Quality.Available ||
		claude.Pricing.Available || claude.Rhythm.Available {
		t.Fatalf("empty claude scope = %#v, want every family unavailable", claude)
	}
	if claude.Hourly.ThroughHour != 10 {
		t.Fatalf("empty claude through hour = %d, want the snapshot's current local hour", claude.Hourly.ThroughHour)
	}
	if claude.Periods.Items == nil || claude.Daily.Items == nil || claude.Hourly.Items == nil || claude.Quality.Items == nil ||
		claude.Pricing.Items == nil || claude.Rhythm.Intensities == nil || claude.Rhythm.Tokens == nil ||
		claude.Rhythm.ProviderCosts == nil || claude.Rhythm.CostIncomplete == nil {
		t.Fatalf("empty claude scope has a nil collection: %#v", claude)
	}
	if len(claude.Periods.Items) != 0 || len(claude.Daily.Items) != 0 || len(claude.Hourly.Items) != 0 || len(claude.Quality.Items) != 0 ||
		len(claude.Pricing.Items) != 0 || len(claude.Rhythm.Intensities) != 0 || len(claude.Rhythm.Tokens) != 0 ||
		len(claude.Rhythm.ProviderCosts) != 0 || len(claude.Rhythm.CostIncomplete) != 0 {
		t.Fatalf("empty claude scope carries synthetic rows: %#v", claude)
	}

	// The nil check above is what the wire cares about: a nil slice encodes as
	// `null`, which the Swift decoder rejects even where it accepts absence.
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"items":null`) || strings.Contains(string(encoded), `"intensities":null`) ||
		strings.Contains(string(encoded), `"provider_costs":null`) {
		t.Fatalf("presentation encodes a null collection: %s", encoded)
	}

	codex := report.Scopes[1]
	if !codex.Periods.Available || len(codex.Daily.Items) != 90 || codex.Hourly.ThroughHour != 10 ||
		len(codex.Hourly.Items) != 11 || len(codex.Rhythm.Intensities) != 168 {
		t.Fatalf("codex scope = %#v, want a populated client to keep its families", codex)
	}
	all := report.Scopes[0]
	if !all.Periods.Available || all.Periods.Items[0].Totals.Events != 1 {
		t.Fatalf("all scope = %#v, want the measured total", all)
	}
}
