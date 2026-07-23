package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// coldStartModels is the set of models observed in real use when this plan was
// measured. It is the fixture that keeps the bundled catalog from silently
// decaying back toward the two-model stub it started as: a regeneration that
// drops one of these fails here rather than showing up as UNPRICED on a fresh
// install.
//
// codex-auto-review is deliberately expected to be unpriced — see the
// wantPriced field and the assertions below.
var coldStartModels = []struct {
	client     string
	model      string
	wantPriced bool
	why        string
}{
	{client: "codex", model: "gpt-5.6-sol", wantPriced: true},
	{client: "codex", model: "gpt-5.6-terra", wantPriced: true},
	{client: "codex", model: "gpt-5.6-luna", wantPriced: true},
	{client: "codex", model: "gpt-5.5", wantPriced: true},
	{client: "codex", model: "gpt-5.4", wantPriced: true},
	{client: "codex", model: "gpt-5.4-mini", wantPriced: true},
	{client: "claude", model: "claude-opus-4-8", wantPriced: true},
	{client: "claude", model: "claude-sonnet-5", wantPriced: true},
	{client: "claude", model: "claude-sonnet-4-6", wantPriced: true},
	{client: "claude", model: "claude-fable-5", wantPriced: true},
	{client: "claude", model: "claude-haiku-4-5-20251001", wantPriced: true},
	// Dotted spellings the claude- punctuation rule normalizes onto the
	// hyphenated catalog rows; they must price without their own entries.
	{client: "claude", model: "claude-haiku-4.5", wantPriced: true},
	{client: "claude", model: "claude-opus-4.8", wantPriced: true},
	{
		client: "codex", model: "gpt-5.3-codex-spark", wantPriced: true,
		why: "upstream prices it only under the cost-less chatgpt/ namespace, so it is priced from price-gapfill.json as an explicitly marked equivalent estimate borrowing its predecessor gpt-5.3-codex's vendor rate",
	},
	{
		client: "codex", model: "codex-auto-review", wantPriced: false,
		why: "absent from every pricing source; out of scope as a probable Codex-internal pseudo-model needing classification, not a price",
	},
}

// TestBundledCatalogPricesRealModelsWithoutNetwork is the cold-start guarantee:
// a fresh install that has never reached the network must price the models
// people actually run, using only the catalog compiled into the binary.
func TestBundledCatalogPricesRealModelsWithoutNetwork(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// No UpdateLiteLLM, no ImportOfficialOverrides: bundled only.
	service := New(s, "")
	at := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	service.Now = func() time.Time { return at }
	if err = service.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}

	for index, want := range coldStartModels {
		insert := `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_write_5m_tokens,cache_write_1h_tokens,source_path,source_offset) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,'fixture',?)`
		key := fmt.Sprintf("e%d", index)
		// Only exercise components the client actually reports, so an
		// unpriced-component gap cannot masquerade as an unpriced model.
		var cachedInput, cacheRead, write5m, write1h int64
		if want.client == "codex" {
			cachedInput = 1000
		} else {
			cacheRead, write5m, write1h = 1000, 1000, 1000
		}
		if _, err = s.Exec(ctx, insert, key, want.client, key, key,
			at.Format(time.RFC3339Nano), want.model,
			10000, cachedInput, 5000, cacheRead, write5m, write1h, index); err != nil {
			t.Fatal(err)
		}
		if _, err = s.Exec(ctx, `INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES(?,?,?,?)`,
			want.client, key, at.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}

	summary, err := service.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	priced := map[string]bool{}
	for _, m := range summary.Models {
		priced[m.Client+"/"+m.Model] = m.PricedEvents > 0 && m.UnpricedEvents == 0
	}
	for _, want := range coldStartModels {
		key := want.client + "/" + want.model
		got, seen := priced[key]
		if !seen {
			t.Fatalf("model %s missing from summary entirely: %+v", key, summary.Models)
		}
		if got != want.wantPriced {
			if want.wantPriced {
				t.Errorf("cold start does not price %s from the bundled catalog; a fresh install would show it UNPRICED", key)
			} else {
				t.Errorf("cold start now prices %s, which this plan deliberately leaves unpriced (%s); if that changed on purpose, update coldStartModels and the plan", key, want.why)
			}
		}
	}
}

// TestBundledCatalogIsNotAStub guards the other direction: the catalog must
// stay broad. The stub it replaced carried two models and covered 7.5% of real
// tokens.
func TestBundledCatalogIsNotAStub(t *testing.T) {
	parsed, err := parseCatalog(bundledCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Models) < 50 {
		t.Fatalf("bundled catalog carries only %d models; it has decayed back toward the two-model stub this plan replaced", len(parsed.Models))
	}
	var openai, anthropic int
	for _, p := range parsed.Models {
		switch p.Provider {
		case "openai":
			openai++
		case "anthropic":
			anthropic++
		}
	}
	if openai == 0 || anthropic == 0 {
		t.Fatalf("bundled catalog must cover both vendors, got openai=%d anthropic=%d", openai, anthropic)
	}
	if !strings.HasPrefix(parsed.Version, BundledCatalogVersionPrefix) {
		t.Fatalf("bundled catalog version %q lost its generated prefix %q", parsed.Version, BundledCatalogVersionPrefix)
	}
}

// TestBundledGapfillMeetsCurationBar keeps the curated input honest: it must
// parse, and anything shipped must carry real verified provenance. It also
// pins that Spark is either shipped or tracked, never silently dropped.
func TestBundledGapfillMeetsCurationBar(t *testing.T) {
	g, err := BundledGapfill()
	if err != nil {
		t.Fatalf("bundled gap-fill does not meet the curation bar: %v", err)
	}
	for _, e := range g.Entries {
		if !strings.HasPrefix(e.SourceURL, "https://") {
			t.Errorf("gap-fill entry %s must cite a real vendor rate card, got %q", e.Model, e.SourceURL)
		}
	}
	var sparkTracked bool
	for _, e := range g.Entries {
		if e.Model != "gpt-5.3-codex-spark" {
			continue
		}
		sparkTracked = true
		if e.PriceKind() != GapfillKindEquivalentEstimate {
			t.Errorf("gpt-5.3-codex-spark ships as %s; OpenAI publishes no rate for it, so it may only ship as an %s", e.PriceKind(), GapfillKindEquivalentEstimate)
		}
		if e.BasisModel != "gpt-5.3-codex" {
			t.Errorf("gpt-5.3-codex-spark estimates from %q, want its priced predecessor gpt-5.3-codex", e.BasisModel)
		}
		if !strings.Contains(strings.ToLower(e.PricingNote), "subscription") {
			t.Errorf("gpt-5.3-codex-spark pricing_note must say the value is not what the subscription bills, got %q", e.PricingNote)
		}
	}
	for _, p := range g.Pending {
		if p.Model == "gpt-5.3-codex-spark" {
			sparkTracked = true
		}
	}
	if !sparkTracked {
		t.Fatal("gpt-5.3-codex-spark is neither shipped nor tracked as pending; the reported gap would be silently forgotten")
	}
}

// TestBundledCatalogEstimateBorrowsItsBasisModelRate is the shipping guard on
// the estimate itself: the value must still be its basis model's vendor rate.
// If upstream reprices gpt-5.3-codex, the estimate no longer estimates
// anything and has to be re-confirmed rather than quietly drifting.
func TestBundledCatalogEstimateBorrowsItsBasisModelRate(t *testing.T) {
	g, err := BundledGapfill()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseCatalog(bundledCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if err = validateGapfillEstimates(parsed.Models, g); err != nil {
		t.Fatalf("shipped catalog and gap-fill disagree: %v", err)
	}
	for _, e := range g.Entries {
		if e.PriceKind() != GapfillKindEquivalentEstimate {
			continue
		}
		if _, ok := parsed.Models[e.BasisModel]; !ok {
			t.Fatalf("basis model %s is missing from the shipped catalog", e.BasisModel)
		}
	}
}

// TestGapfillEntriesMergeIntoBundledLayer proves the contract task 3 defines:
// a curated entry lands in the generated catalog, and a curated entry for a
// model upstream also prices wins (that is the only reason to curate one).
func TestGapfillEntriesMergeIntoBundledLayer(t *testing.T) {
	generated := map[string]modelPrice{
		"already-upstream": {Provider: "openai", EffectiveFrom: BundledCatalogEffectiveFrom, Prices: map[string]string{"input": "1", "output": "2"}},
	}
	g := Gapfill{SchemaVersion: 1, Entries: []GapfillEntry{
		{Model: "never-upstream", Provider: "openai", EffectiveFrom: BundledCatalogEffectiveFrom, SourceURL: "https://vendor.example/rates", VerifiedBy: "someone", VerifiedOn: "2026-07-23", Prices: map[string]string{"input": "9", "output": "9"}},
		{Model: "already-upstream", Provider: "openai", EffectiveFrom: BundledCatalogEffectiveFrom, SourceURL: "https://vendor.example/rates", VerifiedBy: "someone", VerifiedOn: "2026-07-23", Prices: map[string]string{"input": "7", "output": "7"}},
	}}
	merged := MergeGapfill(generated, g)
	// Curated values are rendered like upstream's, so one catalog cannot mix
	// "9" with "9.000000000" for the same unit.
	if merged["never-upstream"].Prices["input"] != "9.000000000" {
		t.Fatalf("curated-only entry did not survive the merge: %#v", merged["never-upstream"])
	}
	if merged["already-upstream"].Prices["input"] != "7.000000000" {
		t.Fatalf("curated entry did not win over the generated row: %#v", merged["already-upstream"])
	}
	if len(generated) != 1 {
		t.Fatal("MergeGapfill mutated its input")
	}
}

// sparkEstimate returns the shipped curated estimate, so the tests below assert
// against the real disclosure text rather than a copy of it that could drift.
func sparkEstimate(t *testing.T) GapfillEntry {
	t.Helper()
	g, err := BundledGapfill()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Entries {
		if e.Model == "gpt-5.3-codex-spark" {
			return e
		}
	}
	t.Fatal("gpt-5.3-codex-spark is not a shipped gap-fill entry")
	return GapfillEntry{}
}

func freshInstall(t *testing.T, at time.Time) (*Service, *store.Store) {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	service := New(s, "")
	service.Now = func() time.Time { return at }
	if err = service.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	return service, s
}

// TestPriceListDisclosesTheEstimateAndItsBasis is the disclosure contract: the
// one place a curated equivalent estimate is allowed to surface is `price
// list`, and there it must say what it is and what it is derived from. A
// published vendor rate in the same listing keeps exactly its old shape, so the
// disclosure cannot be mistaken for a property of every price.
func TestPriceListDisclosesTheEstimateAndItsBasis(t *testing.T) {
	ctx := context.Background()
	entry := sparkEstimate(t)
	service, _ := freshInstall(t, time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC))

	prices, err := service.PriceList(ctx, "openai", "gpt-5.3-codex-spark")
	if err != nil || len(prices) != 1 {
		t.Fatalf("price list for the estimated model = %#v, %v", prices, err)
	}
	spark := prices[0]
	if spark.PriceKind != GapfillKindEquivalentEstimate {
		t.Fatalf("price_kind = %q, want %q; the estimate would read as a published rate", spark.PriceKind, GapfillKindEquivalentEstimate)
	}
	if spark.BasisModel != entry.BasisModel || spark.PricingNote != entry.PricingNote {
		t.Fatalf("disclosure does not match the curated entry: basis=%q note=%q", spark.BasisModel, spark.PricingNote)
	}
	if spark.Prices["input"] != "1.750000000" || spark.Prices["output"] != "14.000000000" || spark.Prices["cached_input"] != "0.175000000" {
		t.Fatalf("estimated prices = %#v", spark.Prices)
	}

	// JSON carries the machine-readable kind, basis, and note.
	encoded, err := json.Marshal(spark)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"price_kind":"equivalent_estimate"`, `"basis_model":"gpt-5.3-codex"`, `"pricing_note":`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("price-list JSON is missing %s:\n%s", want, encoded)
		}
	}

	// The basis model itself is a published vendor rate and stays unmarked.
	basis, err := service.PriceList(ctx, "openai", entry.BasisModel)
	if err != nil || len(basis) != 1 {
		t.Fatalf("price list for the basis model = %#v, %v", basis, err)
	}
	if basis[0].PriceKind != "" || basis[0].BasisModel != "" || basis[0].PricingNote != "" {
		t.Fatalf("a published vendor rate was marked as an estimate: %#v", basis[0])
	}
	encodedBasis, err := json.Marshal(basis[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"price_kind", "basis_model", "pricing_note"} {
		if strings.Contains(string(encodedBasis), unwanted) {
			t.Fatalf("vendor-rate JSON grew a %s field:\n%s", unwanted, encodedBasis)
		}
	}
}

// TestFresherUpstreamPricingRetiresTheEstimateMarker pins the automatic exit
// this design depends on: the estimate is a stand-in for a rate upstream does
// not publish, so the moment upstream supplies every effective component the
// user is no longer looking at an estimate and must not be told they are.
// While upstream covers only part of the model, the estimate is still doing
// work and the disclosure has to stay.
func TestFresherUpstreamPricingRetiresTheEstimateMarker(t *testing.T) {
	ctx := context.Background()
	const commit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const digest = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	for _, test := range []struct {
		name         string
		upstream     string
		wantEstimate bool
	}{
		{
			name:         "upstream prices every component",
			upstream:     `{"input":"2","cached_input":"0.2","output":"16"}`,
			wantEstimate: false,
		},
		{
			name:         "upstream prices only part of the model",
			upstream:     `{"input":"2"}`,
			wantEstimate: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, database, at := newFreshInstallAt(t)
			// Effective the same day as the fresh install, so it outranks the
			// bundled layer on catalog_effective the way a real update would.
			stamp := at.Format(time.RFC3339Nano)
			_, err := database.Exec(ctx, `
INSERT INTO price_catalogs(version,source_kind,source_url,commit_sha,content_sha256,imported_at,effective_from,currency,schema_version) VALUES
 ('litellm-fresher','litellm','https://raw.githubusercontent.com/BerriAI/litellm/`+commit+`/model_prices_and_context_window.json','`+commit+`','`+digest+`','`+stamp+`','`+stamp+`','USD',1);
INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES
 ('litellm-fresher','gpt-5.3-codex-spark','openai','`+stamp+`','`+test.upstream+`','[]')`)
			if err != nil {
				t.Fatal(err)
			}
			prices, err := service.PriceList(ctx, "openai", "gpt-5.3-codex-spark")
			if err != nil || len(prices) != 1 {
				t.Fatalf("price list = %#v, %v", prices, err)
			}
			gotEstimate := prices[0].PriceKind == GapfillKindEquivalentEstimate
			if gotEstimate != test.wantEstimate {
				t.Fatalf("estimate marker = %v, want %v (prices=%#v provenance=%#v)", gotEstimate, test.wantEstimate, prices[0].Prices, prices[0].Provenance)
			}
			if !test.wantEstimate && prices[0].Prices["input"] != "2" {
				t.Fatalf("fresher upstream did not win the component: %#v", prices[0].Prices)
			}
		})
	}
}

// TestAnotherBundledCatalogsPriceIsNotMarkedAsThisEstimate pins that the
// marker follows the estimate, not the layer. Bundled catalogs from several
// releases coexist in one database, so "some component came from the bundled
// layer" is not evidence that the user is looking at *this* binary's curated
// estimate — only the compiled catalog's own version is.
func TestAnotherBundledCatalogsPriceIsNotMarkedAsThisEstimate(t *testing.T) {
	ctx := context.Background()
	service, database, at := newFreshInstallAt(t)
	// A different bundled catalog, effective later, pricing the same model in
	// full. Whatever it is, it is not the estimate this binary compiled in.
	if _, err := database.Exec(ctx, `
INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES
 ('bundled-other-release','bundled','bundled://agentdeck/model-prices.json','dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd',?,?,'USD',1);
INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES
 ('bundled-other-release','gpt-5.3-codex-spark','openai','2026-02-12T00:00:00Z','{"input":"5","cached_input":"0.5","output":"20"}','[]')`,
		at.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	prices, err := service.PriceList(ctx, "openai", "gpt-5.3-codex-spark")
	if err != nil || len(prices) != 1 {
		t.Fatalf("price list = %#v, %v", prices, err)
	}
	if prices[0].Prices["input"] != "5" {
		t.Fatalf("the later bundled catalog did not win: %#v", prices[0].Prices)
	}
	if prices[0].PriceKind != "" {
		t.Fatalf("a price from catalog %q was disclosed as this binary's estimate: %#v",
			prices[0].Provenance["input"].CatalogVersion, prices[0])
	}
}

func newFreshInstallAt(t *testing.T) (*Service, *store.Store, time.Time) {
	t.Helper()
	at := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	service, database := freshInstall(t, at)
	return service, database, at
}

// TestUsageOutputNeverCarriesTheEstimateDisclosure holds the boundary the
// product decision draws: the estimate is disclosed in the price catalog, not
// smeared across every cost report. Stats and summary keep their contracts
// exactly as they were while still pricing the model.
func TestUsageOutputNeverCarriesTheEstimateDisclosure(t *testing.T) {
	ctx := context.Background()
	service, database, at := newFreshInstallAt(t)
	_, err := database.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES('codex','s',?,?);
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,source_path,source_offset)
 VALUES('e','codex','s','e',?,'gpt-5.3-codex-spark',1000000,0,1000000,'fixture',0)`,
		at.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}

	summary, err := service.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 1M input at 1.75 + 1M output at 14.
	if summary.CatalogBaseCost == nil || *summary.CatalogBaseCost != "15.750000000" {
		t.Fatalf("estimated model did not price in the summary: %#v", summary)
	}
	report, err := service.Stats(ctx, StatsOptions{
		From: at.Add(-24 * time.Hour), To: at.Add(24 * time.Hour),
		GroupBy: "day", Metric: "cost", Location: time.UTC, Timezone: "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{"summary": summary, "stats": report} {
		encoded, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		// "ESTIMATED" is matched case-sensitively on purpose: it is the
		// price-list text marker. Summary output legitimately contains the
		// unrelated lowercase counts.estimated, which counts events priced
		// with an estimated provider multiplier — do not relax this to a
		// case-insensitive match or it will fail on that pre-existing field.
		for _, leaked := range []string{"price_kind", "basis_model", "pricing_note", "equivalent_estimate", "ESTIMATED"} {
			if strings.Contains(string(encoded), leaked) {
				t.Fatalf("%s output leaked the price-list-only disclosure %q:\n%s", name, leaked, encoded)
			}
		}
	}
}

// TestBundledCatalogDateIsIndependentOfItsCuratedModelDates pins the fix for
// the interaction the plan flagged and the review caught: the catalog's own
// effective time is a *precedence key* among same-layer catalogs, so deriving
// it from the earliest model date let one curated 2026-02-12 entry drag the
// whole catalog's date back. Each model keeps its own honest date; the catalog
// keeps the stable one.
func TestBundledCatalogDateIsIndependentOfItsCuratedModelDates(t *testing.T) {
	ctx := context.Background()
	service, database := freshInstall(t, time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC))

	history, err := service.PriceHistory(ctx)
	if err != nil || len(history) != 1 {
		t.Fatalf("price history = %#v, %v", history, err)
	}
	if history[0].EffectiveFrom != BundledCatalogEffectiveFrom {
		t.Fatalf("bundled catalog effective_from = %q, want the stable %q; a curated model date must not move it", history[0].EffectiveFrom, BundledCatalogEffectiveFrom)
	}

	// The model row still carries its real release date, which is the part
	// that describes history rather than precedence.
	var modelEffective string
	if err = database.DB.QueryRowContext(ctx,
		`SELECT effective_from FROM model_prices WHERE model='gpt-5.3-codex-spark'`).Scan(&modelEffective); err != nil {
		t.Fatal(err)
	}
	if modelEffective != "2026-02-12T00:00:00Z" {
		t.Fatalf("curated model effective_from = %q, want its own 2026-02-12T00:00:00Z", modelEffective)
	}
}

// TestNewerBundledCatalogOutranksAPreviouslyInstalledOne is the upgrade
// regression. An install that already imported an earlier release's bundled
// catalog keeps those rows forever — they are never deleted, only outranked —
// so the catalog compiled into the running binary must win. It did not: with
// the catalog date derived from the earliest model, this build's catalog
// recorded 2026-02-12 while the shipped v0.1.0 stub recorded 2026-07-13, and
// priceLayerBefore handed every shared model back to the retired stub.
func TestNewerBundledCatalogOutranksAPreviouslyInstalledOne(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	service := New(s, "")
	service.Now = func() time.Time { return time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC) }

	// The catalog a previous release installed: the real retired version
	// string, its real 2026-07-13 date, imported before this upgrade, and a
	// price this release corrects.
	if _, err = s.Exec(ctx, `
INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES
 ('2026-07-13-openai-standard-v1','bundled','bundled://agentdeck/model-prices.json','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','2026-07-15T00:00:00Z','2026-07-13T00:00:00Z','USD',1);
INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES
 ('2026-07-13-openai-standard-v1','gpt-5.4','openai','2026-07-13T00:00:00Z','{"input":"999","cached_input":"999","output":"999"}','[]')`); err != nil {
		t.Fatal(err)
	}
	if err = service.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}

	compiled, err := parseCatalog(bundledCatalog)
	if err != nil {
		t.Fatal(err)
	}
	prices, err := service.PriceList(ctx, "openai", "gpt-5.4")
	if err != nil || len(prices) != 1 {
		t.Fatalf("price list = %#v, %v", prices, err)
	}
	for _, component := range []string{"input", "cached_input", "output"} {
		got := prices[0].Provenance[component]
		if got.CatalogVersion != compiled.Version {
			t.Errorf("%s is served by catalog %q, want this binary's %q; an upgraded install is keeping a retired catalog's price (%s)",
				component, got.CatalogVersion, compiled.Version, prices[0].Prices[component])
		}
	}
}
