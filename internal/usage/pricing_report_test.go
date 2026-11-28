package usage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func seedReportEvent(t *testing.T, db *store.Store, session string, event storedEvent) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.Exec(ctx, `INSERT OR IGNORE INTO usage_sessions(client,session_id,first_at,last_at) VALUES(?,?,?,?)`,
		event.Client, session, event.EventAt, event.EventAt); err != nil {
		t.Fatal(err)
	}
	key := session + event.EventAt
	if _, err := db.Exec(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,cache_write_5m_tokens,cache_write_1h_tokens,cache_write_tokens,source_path,source_offset) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,0)`,
		key, event.Client, session, key, event.EventAt, event.Model, event.Tokens["input_tokens"], event.Tokens["cached_input_tokens"], event.Tokens["output_tokens"], event.Tokens["cache_read_tokens"], event.Tokens["cache_creation_tokens"], event.Tokens["cache_write_5m_tokens"], event.Tokens["cache_write_1h_tokens"], event.Tokens["cache_write_tokens"], "private-source"); err != nil {
		t.Fatal(err)
	}
	payload, err := encodeRequestFacts(event.Event)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Valid {
		if _, err := db.Exec(ctx, `INSERT INTO usage_request_facts(event_key,facts_json) VALUES(?,?)`, key, payload.String); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPricingReportsUseSameAdmittedScopeAndLegacyAmounts(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := New(db, "")
	now := pricingTime("2026-11-23T20:00:00Z")
	service.Now = func() time.Time { return now }
	if err := service.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	installLegacyAstraFixture(t, service)
	complete := codexPricingEvent("gpt-6.2-astra", "2026-11-23T14:00:00Z", 272_001, 100_000, 20_000, 10_000)
	complete.Facts.CacheWrite30m = complete.Facts.CacheWriteTokens
	complete.Facts.Pricing.Channel, complete.Facts.Pricing.ChannelEndpoint = "personal_subscription", "codex"
	complete.Facts.Pricing.Eligibility = []string{"chatgpt_sign_in_included_usage"}
	partial := codexPricingEvent(complete.Model, "2026-11-22T14:00:00Z", 272_000, 100_000, 20_000, 10_000)
	other := codexPricingEvent("gpt-6.1-sol", "2026-11-23T15:00:00Z", 300_000, 0, 0, 10_000)
	claude := claudePricingEvent("claude-haiku-5-5", "2026-11-23T16:00:00Z", 49_000, 40_000, 11_000, 10_000, 1_000, 5_000)
	seedReportEvent(t, db, "combined", complete)
	seedReportEvent(t, db, "combined", partial)
	seedReportEvent(t, db, "other-model", other)
	seedReportEvent(t, db, "combined", claude)
	execPricingSQL(t, db, `INSERT INTO usage_session_routes(client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,source,quality,semantic_key)
VALUES('codex','combined','2026-11-22T13:00:00Z','official','1',0,'SessionStart','fixture','exact','report-fixture')`)

	summary, rows, page, err := service.SessionUsagePage(ctx, "codex", "combined", 2, 1, false)
	if err != nil || page.Total != 2 || page.Shown != 1 || rows[0].Sequence != 2 {
		t.Fatalf("session page = %v %v, %v", rows, page, err)
	}
	if summary.ProviderCost == nil || rows[0].ProviderCost == nil || rows[0].Pricing.ProviderEstimate != nil {
		t.Fatalf("legacy estimate and new channel estimate lost their independent semantics: %+v %+v", summary, rows[0])
	}
	checkPriceAmount(t, rows[0].Pricing.APIEquivalent, pricingComplete, "3.512016000", "3.512016000")
	if rows[0].Pricing.Subscription.Status != "included" || len(rows[0].Pricing.Components) != 5 {
		t.Fatalf("request detail = %+v", rows[0].Pricing)
	}
	exported, err := json.Marshal(rows[0])
	if err != nil || strings.Contains(string(exported), "private-source") || !strings.Contains(string(exported), "effective_from") {
		t.Fatalf("invalid detail projection: %s (%v)", exported, err)
	}
	checkPriceAmount(t, summary.Pricing.APIEquivalent, pricingPartial, "", "5.168016000")
	if summary.Pricing.Reference != nil || len(summary.Pricing.ReferenceDetails) != 1 ||
		valueString(summary.Pricing.ReferenceDetails[0].Upper) != "2.056000000" || slices.Contains(summary.Pricing.Reasons, reasonCoverage) {
		t.Fatalf("reference or coverage escaped its scope: %+v", summary.Pricing)
	}

	options := StatsOptions{From: pricingTime("2026-11-22T00:00:00Z"), To: pricingTime("2026-11-24T00:00:00Z"),
		Client: "codex", Model: complete.Model, GroupBy: "day", Metric: "cost", Location: time.UTC}
	stats, err := service.Stats(ctx, options)
	if err != nil || stats.Totals.Events != 2 || stats.Totals.Pricing.RuleSet != summary.Pricing.RuleSet {
		t.Fatalf("filtered stats = %+v (%v)", stats, err)
	}
	checkPriceAmount(t, stats.Totals.Pricing.APIEquivalent, pricingPartial, "", "5.168016000")
	if stats.APIPeak != nil || stats.Models[0].APIShare != nil ||
		strings.Count(strings.Join(stats.Totals.Pricing.Reasons, ","), reasonCoverage) != 1 ||
		slices.Contains(stats.Buckets[0].Pricing.Reasons, reasonCoverage) {
		t.Fatalf("incomparable filtered report got a peak, share, or stray coverage: %+v", stats)
	}
	options.From = pricingTime("2026-11-23T00:00:00Z")
	stats, err = service.Stats(ctx, options)
	if err != nil || stats.APIPeak == nil || valueString(stats.APIPeak.Value) != "3.512016000" || valueString(stats.Models[0].APIShare) != "100.00" {
		t.Fatalf("comparable stats got no peak or share: %+v (%v)", stats, err)
	}
	options.Client, options.Model = "claude", ""
	stats, err = service.Stats(ctx, options)
	if err != nil || stats.Totals.Events != 1 || slices.Contains(stats.Totals.Pricing.Reasons, reasonCoverage) {
		t.Fatalf("Claude-only stats acquired Codex coverage: %+v (%v)", stats.Totals, err)
	}
	projected, err := service.ProjectedPresentation(ctx, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	oracle, err := service.Presentation(ctx, now, time.UTC)
	if err != nil || !reflect.DeepEqual(projected, oracle) {
		t.Fatalf("projection differs from transaction oracle: %v", err)
	}
	rhythm := projected.RequestPricing.Scopes[0].Rhythm
	// Sunday at 14:00 belongs to the 30-day all-client rhythm with its own TTL interval.
	sunday := slices.IndexFunc(rhythm, func(bucket PricingTimeBucket) bool { return bucket.Key == "158" })
	if sunday < 0 || valueString(rhythm[sunday].Pricing.APIEquivalent.KnownAmount) != "1.656000000" {
		t.Fatalf("hour-of-week pricing is missing or incorrectly grouped: %+v", rhythm)
	}
	codex := projected.RequestPricing.Scopes[1].Periods
	checkPriceAmount(t, codex[0].Pricing.APIEquivalent, pricingComplete, "4.862016000", "4.862016000")
	if codex[0].Peak == nil || codex[0].Models[0].Key != complete.Model || codex[1].Peak != nil || codex[1].Models[0].Share != nil {
		t.Fatalf("period filter/ranking mismatch: %+v", codex)
	}
	if !slices.Contains(codex[0].Pricing.Reasons, reasonCoverage) || len(codex[0].Pricing.Sources) == 0 {
		t.Fatalf("period omitted coverage or provenance: %+v", codex[0].Pricing)
	}
}

func TestProjectedPricingIncludesWritesBeforeEveryPartitionBegins(t *testing.T) {
	ctx := context.Background()
	db := openPricingTestStore(t, filepath.Join(t.TempDir(), "state"))
	service := New(db, "")
	const model = "gpt-6.2-astra"
	seedReportEvent(t, db, "selected", codexPricingEvent(model, "2026-11-20T12:00:00Z", 10, 0, 0, 0))
	seedReportEvent(t, db, "selected", codexPricingEvent(model, "2026-11-21T12:00:00Z", 20, 0, 0, 0))
	// The first Begin clears its dirty flag. Insert an event in the second
	// partition while that partition is still dirty, coalescing its revision.
	execPricingSQL(t, db, `CREATE TRIGGER report_write_before_later_partition
		AFTER UPDATE OF dirty ON usage_derived_partitions
		WHEN OLD.day='2026-11-20' AND OLD.dirty=1 AND NEW.dirty=0
		BEGIN
			INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset)
			VALUES('late','codex','selected','late','2026-11-21T12:01:00Z','gpt-6.2-astra',30,'fixture',0);
		END;`)
	from := pricingTime("2026-11-20T00:00:00Z")
	to := pricingTime("2026-11-22T00:00:00Z")
	now := pricingTime("2026-11-22T15:00:00Z")
	check := func(label string, incremental bool) {
		t.Helper()
		events, _, _, err := service.presentationInputs(ctx, from, to, now, incremental)
		if err != nil {
			t.Fatalf("%s presentation failed: %v", label, err)
		}
		var input int64
		for _, event := range events {
			input += event.Tokens["input_tokens"]
		}
		if len(events) != 3 || input != 60 || events[len(events)-1].Key != "late" {
			t.Errorf("%s presentation read %d events with %d input tokens, want 3 / 60 including late", label, len(events), input)
		}
	}
	check("rebuilt", true)
	reads := service.eventsRangeReads.Load()
	check("cached", true)
	if service.eventsRangeReads.Load() != reads {
		t.Error("second incremental read did not use the completed cache")
	}
	check("full", false)
}

func TestEmptyPricingReportsUseCompleteZero(t *testing.T) {
	ctx := context.Background()
	db := openPricingTestStore(t, filepath.Join(t.TempDir(), "state"))
	service := New(db, "")
	from := pricingTime("2026-11-21T00:00:00Z")
	to := pricingTime("2026-11-22T00:00:00Z")
	now := pricingTime("2026-11-21T17:00:00Z")
	service.Now = func() time.Time { return now }
	check := func(label string, pricing *PricingReport) {
		t.Helper()
		if pricing == nil {
			t.Fatalf("%s omitted pricing", label)
		}
		amount := pricing.APIEquivalent
		if amount.Currency != pricingCurrency || amount.Status != pricingComplete ||
			valueString(amount.Amount) != "0.000000000" || valueString(amount.KnownAmount) != "0.000000000" {
			t.Errorf("%s amount = %+v, want complete USD zero", label, amount)
		}
		if pricing.ProviderEstimate != nil || pricing.Reference != nil || len(pricing.Reasons) != 0 {
			t.Errorf("%s empty usage reported an estimate, reference, or reason: %+v", label, pricing)
		}
	}
	summary, err := service.SummaryRange(ctx, from, to)
	if err != nil {
		t.Fatal(err)
	}
	check("summary", summary.Pricing)
	stats, err := service.Stats(ctx, StatsOptions{From: from, To: to, GroupBy: "day", Metric: "cost", Location: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	check("stats", stats.Totals.Pricing)
	if stats.APIPeak != nil {
		t.Fatalf("empty stats acquired an API peak: %+v", stats.APIPeak)
	}
	presentation, err := service.ProjectedPresentation(ctx, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range presentation.RequestPricing.Scopes {
		if len(scope.Periods) != 3 {
			t.Errorf("empty %s scope has %d periods, want 3", scope.Client, len(scope.Periods))
		}
		for _, period := range scope.Periods {
			check(scope.Client+" "+period.Period, period.Pricing)
			if period.Peak != nil {
				t.Errorf("empty %s %s acquired a peak", scope.Client, period.Period)
			}
		}
	}
}

func TestPricingDisplayDistinguishesZeroScopesAndDiagnosticReferences(t *testing.T) {
	zero, upper, fallback := "0", "4", "2"
	full := PricingAmount{Status: pricingComplete, Amount: &zero}
	part := PricingAmount{Status: pricingPartial, KnownAmount: &zero}
	for _, tc := range []struct {
		want  string
		price PricingReport
	}{
		{"complete", PricingReport{APIEquivalent: full}},
		{"known", PricingReport{APIEquivalent: part, Reference: &PricingReference{Basis: "as_of_date", Amount: &fallback}}},
		{"reference", PricingReport{Reference: &PricingReference{Basis: "as_of_date", Amount: &fallback}}},
		{"unavailable", PricingReport{Reference: &PricingReference{Basis: "cache_write_ttl_range", Lower: &fallback, Upper: &upper}}},
		{"unavailable", PricingReport{Reference: &PricingReference{Basis: "per_request_context", Amount: &fallback}}},
		{"unavailable", PricingReport{Reference: &PricingReference{Basis: "requested_tier", Amount: &fallback}}},
	} {
		if got, _, _ := tc.price.DisplayAPI(); got != tc.want {
			t.Fatalf("display basis %q, want %q for %+v", got, tc.want, tc.price)
		}
	}
}
