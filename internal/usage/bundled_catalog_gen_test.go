package usage

import (
	"encoding/json"
	"strings"
	"testing"
)

const genTestCommit = "f2479cc704f6e63d5510929d30ce8e11ffe43467"

// syntheticLiteLLM mirrors the shape and the traps of the real upstream
// document: both direct vendors, an Azure/Vertex channel row that must not be
// imported, a cost-less chatgpt/ row (the exact reason Spark is unpriceable
// upstream), and an openai row missing cached-input cost that the importer
// must drop rather than half-price.
func syntheticLiteLLM() []byte {
	return []byte(`{
  "gpt-test-openai": {
    "litellm_provider": "openai",
    "input_cost_per_token": 0.0000025,
    "output_cost_per_token": 0.000015,
    "cache_read_input_token_cost": 0.00000025
  },
  "claude-test-anthropic": {
    "litellm_provider": "anthropic",
    "input_cost_per_token": 0.000003,
    "output_cost_per_token": 0.000015,
    "cache_read_input_token_cost": 0.0000003,
    "cache_creation_input_token_cost": 0.00000375,
    "cache_creation_input_token_cost_above_1hr": 0.000006
  },
  "azure/gpt-test-openai": {
    "litellm_provider": "azure",
    "input_cost_per_token": 0.0000025,
    "output_cost_per_token": 0.000015,
    "cache_read_input_token_cost": 0.00000025
  },
  "chatgpt/gpt-test-spark": {
    "litellm_provider": "chatgpt"
  },
  "gpt-test-no-cached": {
    "litellm_provider": "openai",
    "input_cost_per_token": 0.0000025,
    "output_cost_per_token": 0.000015
  }
}`)
}

func generateForTest(t *testing.T, gapfill Gapfill) ([]byte, bundledCatalogDocument) {
	t.Helper()
	raw, err := GenerateBundledCatalog(syntheticLiteLLM(), genTestCommit, gapfill)
	if err != nil {
		t.Fatalf("GenerateBundledCatalog: %v", err)
	}
	var document bundledCatalogDocument
	if err = json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("generated catalog is not valid JSON: %v", err)
	}
	return raw, document
}

// TestGenerateBundledCatalogFiltersToDirectVendors pins the provider filter:
// channel resellers and the cost-less chatgpt/ namespace stay out, and a row
// missing a required component is dropped rather than partially priced.
func TestGenerateBundledCatalogFiltersToDirectVendors(t *testing.T) {
	_, document := generateForTest(t, Gapfill{SchemaVersion: 1})

	for _, unwanted := range []string{"azure/gpt-test-openai", "chatgpt/gpt-test-spark", "gpt-test-no-cached"} {
		if _, present := document.Models[unwanted]; present {
			t.Errorf("generated catalog imported %q, which the direct-vendor filter must exclude", unwanted)
		}
	}
	if len(document.Models) != 2 {
		t.Fatalf("generated catalog has %d models, want exactly the 2 direct-vendor rows: %v", len(document.Models), document.Models)
	}

	openai, ok := document.Models["gpt-test-openai"]
	if !ok {
		t.Fatal("direct openai row missing")
	}
	if openai.Provider != "openai" || openai.Prices["input"] != "2.500000000" || openai.Prices["output"] != "15.000000000" || openai.Prices["cached_input"] != "0.250000000" {
		t.Fatalf("openai per-million conversion wrong: %#v", openai)
	}
	anthropic, ok := document.Models["claude-test-anthropic"]
	if !ok {
		t.Fatal("direct anthropic row missing")
	}
	for component, want := range map[string]string{
		"input": "3.000000000", "output": "15.000000000", "cache_read": "0.300000000",
		"cache_write_5m": "3.750000000", "cache_write_1h": "6.000000000",
	} {
		if got := anthropic.Prices[component]; got != want {
			t.Errorf("anthropic %s = %q, want %q", component, got, want)
		}
	}
}

// TestGenerateBundledCatalogUsesStableEffectiveDate is the regression for the
// defect that made every historical event unpriced on a fresh install: the
// bundled layer must not be dated at generation time.
func TestGenerateBundledCatalogUsesStableEffectiveDate(t *testing.T) {
	_, document := generateForTest(t, Gapfill{SchemaVersion: 1})
	for name, model := range document.Models {
		if model.EffectiveFrom != BundledCatalogEffectiveFrom {
			t.Errorf("model %s effective_from = %q, want the stable %q; a generation-time date leaves pre-build usage unpriced", name, model.EffectiveFrom, BundledCatalogEffectiveFrom)
		}
	}
}

// TestGenerateBundledCatalogSeparatesOwnProvenanceFromUpstream pins the
// contract an existing test enforces from the other side: `sources` is this
// artifact's own identity (a single bundled:// sentinel, the value
// ImportBundledCatalog records), while upstream identity lives separately.
func TestGenerateBundledCatalogSeparatesOwnProvenanceFromUpstream(t *testing.T) {
	_, document := generateForTest(t, Gapfill{SchemaVersion: 1})

	if len(document.Sources) != 1 || document.Sources[0].URL != bundledCatalogSourceURL {
		t.Fatalf("sources = %#v, want exactly one %q sentinel", document.Sources, bundledCatalogSourceURL)
	}
	if len(document.GeneratedFrom) != 1 {
		t.Fatalf("generated_from = %#v, want one litellm origin", document.GeneratedFrom)
	}
	origin := document.GeneratedFrom[0]
	if origin.Kind != "litellm" || origin.CommitSHA != genTestCommit || !strings.Contains(origin.URL, genTestCommit) {
		t.Fatalf("litellm origin does not pin the commit: %#v", origin)
	}
	// No wall-clock anywhere in the artifact, or it could not be reproducible.
	if strings.Contains(string(mustMarshal(t, origin)), "retrieved_at") {
		t.Fatalf("generated_from carries a wall-clock timestamp, defeating reproducibility: %#v", origin)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestGenerateBundledCatalogMergesCuratedEntries proves root cause 3 is closed
// at the generator, not merely in the merge helper: a curated model upstream
// never prices survives generation, and a curated row overrides upstream.
func TestGenerateBundledCatalogMergesCuratedEntries(t *testing.T) {
	gapfill := Gapfill{SchemaVersion: 1, Entries: []GapfillEntry{
		{
			Model: "gpt-test-spark", Provider: "openai", EffectiveFrom: "2026-02-12T00:00:00Z",
			SourceURL: "https://openai.com/api/pricing/", VerifiedBy: "a-human", VerifiedOn: "2026-07-23",
			Prices: map[string]string{"input": "1.75", "cached_input": "0.175", "output": "14"},
		},
		{
			Model: "gpt-test-openai", Provider: "openai", EffectiveFrom: "2026-02-12T00:00:00Z",
			SourceURL: "https://openai.com/api/pricing/", VerifiedBy: "a-human", VerifiedOn: "2026-07-23",
			Prices: map[string]string{"input": "99", "output": "99", "cached_input": "9"},
		},
	}}
	if _, err := ParseGapfill(mustMarshal(t, gapfill)); err != nil {
		t.Fatalf("test gap-fill does not meet the curation bar: %v", err)
	}
	_, document := generateForTest(t, gapfill)

	spark, ok := document.Models["gpt-test-spark"]
	if !ok {
		t.Fatal("curated model absent from upstream did not survive generation")
	}
	// Curated values are rendered exactly like upstream's, so the merged
	// catalog carries one unit format rather than "1.75" beside "1.750000000".
	if spark.Prices["input"] != "1.750000000" || spark.EffectiveFrom != "2026-02-12T00:00:00Z" {
		t.Fatalf("curated entry lost its own price or effective date: %#v", spark)
	}
	if got := document.Models["gpt-test-openai"].Prices["input"]; got != "99.000000000" {
		t.Fatalf("curated entry did not override the upstream row: input = %q, want 99.000000000", got)
	}
	if len(document.GeneratedFrom) != 2 || document.GeneratedFrom[1].Kind != "curated" {
		t.Fatalf("curated provenance not recorded: %#v", document.GeneratedFrom)
	}
}

// TestGenerateBundledCatalogIsDeterministicAndSelfVerifying pins byte
// reproducibility (what `genprices -check` relies on) and the content-derived
// version stamp.
func TestGenerateBundledCatalogIsDeterministicAndSelfVerifying(t *testing.T) {
	first, document := generateForTest(t, Gapfill{SchemaVersion: 1})
	second, err := GenerateBundledCatalog(syntheticLiteLLM(), genTestCommit, Gapfill{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("two generations from identical inputs produced different bytes; genprices -check could never pass")
	}
	if !strings.HasPrefix(document.Version, BundledCatalogVersionPrefix+"-g") {
		t.Fatalf("version %q lacks the generated prefix and content digest", document.Version)
	}
	if err = VerifyBundledCatalogVersion(first); err != nil {
		t.Fatalf("generated catalog fails the version guard it should satisfy: %v", err)
	}

	// A different curated input must move the digest, or the guard is inert.
	changed, err := GenerateBundledCatalog(syntheticLiteLLM(), genTestCommit, Gapfill{SchemaVersion: 1, Entries: []GapfillEntry{{
		Model: "gpt-test-extra", Provider: "openai", EffectiveFrom: "2026-02-12T00:00:00Z",
		SourceURL: "https://openai.com/api/pricing/", VerifiedBy: "a-human", VerifiedOn: "2026-07-23",
		Prices: map[string]string{"input": "1", "output": "2"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	var changedDocument bundledCatalogDocument
	if err = json.Unmarshal(changed, &changedDocument); err != nil {
		t.Fatal(err)
	}
	if changedDocument.Version == document.Version {
		t.Fatal("adding a curated model did not change the content-derived version")
	}
}

func TestGenerateBundledCatalogRequiresAPinnedCommit(t *testing.T) {
	for _, commit := range []string{"", "main", "HEAD", "f2479cc", strings.Repeat("z", 40)} {
		if _, err := GenerateBundledCatalog(syntheticLiteLLM(), commit, Gapfill{SchemaVersion: 1}); err == nil {
			t.Errorf("commit %q accepted; generation must require a pinned 40-hex SHA", commit)
		}
	}
}

func TestGenerateBundledCatalogRejectsUnusableUpstream(t *testing.T) {
	if _, err := GenerateBundledCatalog([]byte("{not json"), genTestCommit, Gapfill{SchemaVersion: 1}); err == nil {
		t.Error("malformed upstream JSON accepted")
	}
	noDirect := []byte(`{"azure/x":{"litellm_provider":"azure","input_cost_per_token":1,"output_cost_per_token":1}}`)
	if _, err := GenerateBundledCatalog(noDirect, genTestCommit, Gapfill{SchemaVersion: 1}); err == nil {
		t.Error("upstream with no direct-vendor rows accepted; it would ship an empty catalog")
	}
}

// TestRecordedLiteLLMCommit covers the check-path discovery that replaced the
// Makefile's python3 subprocess, including the failure modes that must not
// silently fall back to a different pin.
func TestRecordedLiteLLMCommit(t *testing.T) {
	generated, _ := generateForTest(t, Gapfill{SchemaVersion: 1})
	got, err := RecordedLiteLLMCommit(generated)
	if err != nil || got != genTestCommit {
		t.Fatalf("RecordedLiteLLMCommit(generated) = %q, %v; want %q", got, err, genTestCommit)
	}
	// The real committed artifact must also be self-describing.
	if _, err = RecordedLiteLLMCommit(bundledCatalog); err != nil {
		t.Fatalf("shipped catalog does not record its own pinned commit: %v", err)
	}

	for _, test := range []struct{ name, data, wantErr string }{
		{"malformed json", "{not json", "read recorded LiteLLM commit"},
		{"no generated_from", `{"models":{}}`, "records no litellm entry"},
		{"no litellm kind", `{"generated_from":[{"kind":"curated"}]}`, "records no litellm entry"},
		{"ambiguous", `{"generated_from":[{"kind":"litellm","commit_sha":"` + genTestCommit + `"},{"kind":"litellm","commit_sha":"` + genTestCommit + `"}]}`, "ambiguous"},
		{"invalid sha", `{"generated_from":[{"kind":"litellm","commit_sha":"main"}]}`, "invalid pinned LiteLLM commit"},
		{"empty sha", `{"generated_from":[{"kind":"litellm","commit_sha":""}]}`, "invalid pinned LiteLLM commit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := RecordedLiteLLMCommit([]byte(test.data))
			if err == nil {
				t.Fatalf("accepted %s; check mode would compare against the wrong input", test.name)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("wrong error for %s: %v (want substring %q)", test.name, err, test.wantErr)
			}
		})
	}
}

// TestGenerateBundledCatalogGuardsEquivalentEstimates proves the estimate
// contract is enforced where the upstream catalog is actually in hand.
// ParseGapfill can only require that a basis model is named; generation is the
// one place that can require the name to refer to a real vendor-priced model
// carrying exactly the borrowed rate, which is what keeps "estimate" from
// degrading into "invented figure with a citation".
func TestGenerateBundledCatalogGuardsEquivalentEstimates(t *testing.T) {
	estimate := func(mutate func(*GapfillEntry)) Gapfill {
		entry := GapfillEntry{
			Model: "gpt-test-spark", Provider: "openai",
			Kind: GapfillKindEquivalentEstimate, BasisModel: "gpt-test-openai",
			PricingNote:   "Equivalent token-cost estimate, not a subscription invoice.",
			EffectiveFrom: "2026-02-12T00:00:00Z",
			SourceURL:     "https://openai.com/api/pricing/", VerifiedBy: "a-human", VerifiedOn: "2026-07-23",
			// Exactly the synthetic upstream rate for gpt-test-openai.
			Prices: map[string]string{"input": "2.5", "output": "15", "cached_input": "0.25"},
		}
		mutate(&entry)
		g := Gapfill{SchemaVersion: 1, Entries: []GapfillEntry{entry}}
		if _, err := ParseGapfill(mustMarshal(t, g)); err != nil {
			t.Fatalf("test gap-fill does not meet the curation bar: %v", err)
		}
		return g
	}

	_, document := generateForTest(t, estimate(func(*GapfillEntry) {}))
	spark, ok := document.Models["gpt-test-spark"]
	if !ok {
		t.Fatal("estimated model absent from upstream did not survive generation")
	}
	if spark.Prices["input"] != "2.500000000" || spark.Prices["output"] != "15.000000000" {
		t.Fatalf("estimate did not land at its basis model's rate: %#v", spark)
	}
	// The artifact says out loud that one of its curated rows is an estimate.
	if len(document.GeneratedFrom) != 2 || !strings.Contains(document.GeneratedFrom[1].Note, "equivalent estimate") {
		t.Fatalf("curated provenance does not disclose the estimate: %#v", document.GeneratedFrom)
	}

	for _, test := range []struct {
		name    string
		mutate  func(*GapfillEntry)
		wantErr string
	}{
		{
			name:    "basis model is not in the upstream catalog",
			mutate:  func(e *GapfillEntry) { e.BasisModel = "gpt-test-no-cached" },
			wantErr: "which the upstream catalog does not price",
		},
		{
			name:    "estimate drifted away from its basis rate",
			mutate:  func(e *GapfillEntry) { e.Prices["output"] = "20" },
			wantErr: "an equivalent estimate must carry the basis model's vendor rate",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := GenerateBundledCatalog(syntheticLiteLLM(), genTestCommit, estimate(test.mutate))
			if err == nil {
				t.Fatalf("generation accepted an estimate that should have been rejected for %q", test.wantErr)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("rejected for the wrong reason:\n got: %v\nwant substring: %s", err, test.wantErr)
			}
		})
	}
}
