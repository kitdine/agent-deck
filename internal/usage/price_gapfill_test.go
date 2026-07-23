package usage

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// validGapfillEntry is the shape a promoted pending model must reach. Tests
// below vary one field at a time from it so each rejection is attributable.
func validGapfillEntry() GapfillEntry {
	return GapfillEntry{
		Model:         "gpt-5.3-codex-spark",
		Provider:      "openai",
		EffectiveFrom: "2026-02-12T00:00:00Z",
		SourceURL:     "https://openai.com/api/pricing/",
		VerifiedBy:    "a-human",
		VerifiedOn:    "2026-07-23",
		Prices:        map[string]string{"input": "1.75", "cached_input": "0.175", "output": "14"},
	}
}

// asEstimate turns the valid vendor entry into the valid equivalent-estimate
// form, so estimate cases below vary one disclosure field at a time from a
// shape that is otherwise known to pass.
func asEstimate(e *GapfillEntry) {
	e.Kind = GapfillKindEquivalentEstimate
	e.BasisModel = "gpt-5.3-codex"
	e.PricingNote = "Equivalent token-cost estimate, not what the subscription bills."
}

func gapfillWith(entry GapfillEntry) []byte {
	data, err := json.Marshal(Gapfill{SchemaVersion: 1, Entries: []GapfillEntry{entry}})
	if err != nil {
		panic(err)
	}
	return data
}

// TestParseGapfillEnforcesCurationBar is the table the review asks for: the
// bar has to reject a plausible-looking entry, not just an empty one, so the
// first promoted pending model cannot slip through on an aggregator URL or a
// component that prices nothing.
func TestParseGapfillEnforcesCurationBar(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*GapfillEntry)
		wantErr string // substring; empty means the entry must be accepted
	}{
		{name: "valid vendor entry", mutate: func(*GapfillEntry) {}},
		{
			name:   "vendor subdomain is a real rate card",
			mutate: func(e *GapfillEntry) { e.SourceURL = "https://platform.openai.com/docs/pricing" },
		},
		{
			name: "anthropic entry on anthropic domain",
			mutate: func(e *GapfillEntry) {
				e.Provider, e.SourceURL = "anthropic", "https://www.anthropic.com/pricing"
				e.Prices = map[string]string{"input": "3", "output": "15", "cache_read": "0.3"}
			},
		},
		{
			name:    "aggregator url is not a vendor rate card",
			mutate:  func(e *GapfillEntry) { e.SourceURL = "https://models.dev/openai/gpt-5.3-codex-spark" },
			wantErr: "not a openai vendor rate card",
		},
		{
			name:    "openrouter aggregator rejected",
			mutate:  func(e *GapfillEntry) { e.SourceURL = "https://openrouter.ai/models/openai/gpt-5.3-codex" },
			wantErr: "not a openai vendor rate card",
		},
		{
			name:    "placeholder example host rejected",
			mutate:  func(e *GapfillEntry) { e.SourceURL = "https://vendor.example/rates" },
			wantErr: "not a openai vendor rate card",
		},
		{
			name:    "provider and vendor domain must agree",
			mutate:  func(e *GapfillEntry) { e.Provider = "anthropic" },
			wantErr: "not a anthropic vendor rate card",
		},
		{
			name:    "lookalike host is not a subdomain",
			mutate:  func(e *GapfillEntry) { e.SourceURL = "https://notopenai.com/api/pricing/" },
			wantErr: "not a openai vendor rate card",
		},
		{
			name:    "http is not https",
			mutate:  func(e *GapfillEntry) { e.SourceURL = "http://openai.com/api/pricing/" },
			wantErr: "needs a real https vendor rate-card source_url",
		},
		{
			name:    "arbitrary component prices nothing",
			mutate:  func(e *GapfillEntry) { e.Prices = map[string]string{"not_a_price": "1.75"} },
			wantErr: `price component "not_a_price"`,
		},
		{
			name:    "unsupported component alongside a real one still rejected",
			mutate:  func(e *GapfillEntry) { e.Prices = map[string]string{"input": "1.75", "made_up": "2"} },
			wantErr: `price component "made_up"`,
		},
		// The allowlist is per provider, not global: priceAt's codex branch
		// never reads cache_read and its claude branch never reads
		// cached_input, so a component borrowed from the other vendor prices
		// nothing even though the name exists somewhere in priceAt.
		{
			name:    "openai entry cannot use an anthropic-only component",
			mutate:  func(e *GapfillEntry) { e.Prices = map[string]string{"cache_read": "0.175"} },
			wantErr: `price component "cache_read", which the cost calculation does not read for provider openai`,
		},
		{
			name: "openai entry cannot mix in an anthropic-only component",
			mutate: func(e *GapfillEntry) {
				e.Prices = map[string]string{"input": "1.75", "output": "14", "cache_write_5m": "2"}
			},
			wantErr: `does not read for provider openai`,
		},
		{
			name: "anthropic entry cannot use an openai-only component",
			mutate: func(e *GapfillEntry) {
				e.Provider, e.SourceURL = "anthropic", "https://www.anthropic.com/pricing"
				e.Prices = map[string]string{"cached_input": "0.3"}
			},
			wantErr: `price component "cached_input", which the cost calculation does not read for provider anthropic`,
		},
		{
			name: "anthropic entry cannot mix in an openai-only component",
			mutate: func(e *GapfillEntry) {
				e.Provider, e.SourceURL = "anthropic", "https://www.anthropic.com/pricing"
				e.Prices = map[string]string{"input": "3", "output": "15", "cached_input": "0.3"}
			},
			wantErr: `does not read for provider anthropic`,
		},
		{
			name: "openai entry with its full supported set",
			mutate: func(e *GapfillEntry) {
				e.Prices = map[string]string{"input": "1.75", "cached_input": "0.175", "output": "14"}
			},
		},
		{
			name: "anthropic entry with its full supported set",
			mutate: func(e *GapfillEntry) {
				e.Provider, e.SourceURL = "anthropic", "https://www.anthropic.com/pricing"
				e.Prices = map[string]string{
					"input": "3", "output": "15", "cache_read": "0.3",
					"cache_write_5m": "3.75", "cache_write_1h": "6",
				}
			},
		},
		// The equivalent-estimate exception is the only way a price that the
		// vendor does not publish for this exact model may ship, so its
		// disclosure fields are mandatory rather than decorative.
		{name: "valid equivalent estimate", mutate: asEstimate},
		{
			name: "estimate must name its basis model",
			mutate: func(e *GapfillEntry) {
				asEstimate(e)
				e.BasisModel = "  "
			},
			wantErr: "must name the vendor-priced basis_model",
		},
		{
			name: "estimate cannot cite itself as its own basis",
			mutate: func(e *GapfillEntry) {
				asEstimate(e)
				e.BasisModel = e.Model
			},
			wantErr: "names itself as basis_model",
		},
		{
			name: "estimate must disclose what it is",
			mutate: func(e *GapfillEntry) {
				asEstimate(e)
				e.PricingNote = ""
			},
			wantErr: "must carry a pricing_note",
		},
		{
			name:    "a vendor rate may not wear estimate metadata",
			mutate:  func(e *GapfillEntry) { e.BasisModel = "gpt-5.3-codex" },
			wantErr: "carries basis_model/pricing_note",
		},
		{
			name:    "unknown kind rejected",
			mutate:  func(e *GapfillEntry) { e.Kind = "vibes" },
			wantErr: `has kind "vibes"`,
		},
		{
			name:    "no prices at all",
			mutate:  func(e *GapfillEntry) { e.Prices = map[string]string{} },
			wantErr: "has no prices_per_million",
		},
		{
			name:    "non-decimal price",
			mutate:  func(e *GapfillEntry) { e.Prices = map[string]string{"input": "free"} },
			wantErr: "invalid price",
		},
		{
			name:    "missing verifier",
			mutate:  func(e *GapfillEntry) { e.VerifiedBy = "   " },
			wantErr: "needs verified_by",
		},
		{
			name:    "missing verification date",
			mutate:  func(e *GapfillEntry) { e.VerifiedOn = "" },
			wantErr: "needs verified_on",
		},
		{
			name:    "malformed verification date",
			mutate:  func(e *GapfillEntry) { e.VerifiedOn = "23/07/2026" },
			wantErr: "needs verified_on",
		},
		{
			name:    "unparseable effective_from",
			mutate:  func(e *GapfillEntry) { e.EffectiveFrom = "2026-02-12" },
			wantErr: "invalid effective_from",
		},
		{
			name:    "unknown provider",
			mutate:  func(e *GapfillEntry) { e.Provider = "google" },
			wantErr: "want openai or anthropic",
		},
		{
			name:    "missing model",
			mutate:  func(e *GapfillEntry) { e.Model = "" },
			wantErr: "has no model",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := validGapfillEntry()
			test.mutate(&entry)
			_, err := ParseGapfill(gapfillWith(entry))
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("valid entry rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("entry accepted but should have been rejected for %q", test.wantErr)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("rejected for the wrong reason:\n got: %v\nwant substring: %s", err, test.wantErr)
			}
		})
	}
}

func TestParseGapfillRejectsDuplicateAndContradictoryModels(t *testing.T) {
	entry := validGapfillEntry()
	dup, err := json.Marshal(Gapfill{SchemaVersion: 1, Entries: []GapfillEntry{entry, entry}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseGapfill(dup); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("duplicate entries accepted: %v", err)
	}

	both, err := json.Marshal(Gapfill{
		SchemaVersion: 1,
		Entries:       []GapfillEntry{entry},
		Pending:       []GapfillPending{{Model: entry.Model, BlockedOn: "something"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseGapfill(both); err == nil || !strings.Contains(err.Error(), "both shipped and pending") {
		t.Fatalf("model shipped and pending simultaneously accepted: %v", err)
	}

	silent, err := json.Marshal(Gapfill{SchemaVersion: 1, Pending: []GapfillPending{{Model: "x"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseGapfill(silent); err == nil || !strings.Contains(err.Error(), "blocked_on") {
		t.Fatalf("pending entry without a reason accepted: %v", err)
	}
}

// TestMergeGapfillDoesNotMutateItsInputs covers the aliases aliasing bug: the
// previous implementation sorted the caller's backing array in place while
// documenting that it copies.
func TestMergeGapfillDoesNotMutateItsInputs(t *testing.T) {
	entry := validGapfillEntry()
	entry.Aliases = []string{"zzz-alias", "aaa-alias"}
	originalAliases := append([]string(nil), entry.Aliases...)
	g := Gapfill{SchemaVersion: 1, Entries: []GapfillEntry{entry}}
	originalEntry := g.Entries[0]

	generated := map[string]modelPrice{
		"upstream-model": {Provider: "openai", EffectiveFrom: BundledCatalogEffectiveFrom, Prices: map[string]string{"input": "1", "output": "2"}},
	}
	generatedCopy := map[string]modelPrice{}
	for k, v := range generated {
		generatedCopy[k] = v
	}

	merged := MergeGapfill(generated, g)

	if !reflect.DeepEqual(g.Entries[0].Aliases, originalAliases) {
		t.Fatalf("MergeGapfill reordered the caller's aliases in place: got %v, want %v", g.Entries[0].Aliases, originalAliases)
	}
	if !reflect.DeepEqual(g.Entries[0], originalEntry) {
		t.Fatalf("MergeGapfill mutated the caller's gap-fill entry: got %#v, want %#v", g.Entries[0], originalEntry)
	}
	if !reflect.DeepEqual(generated, generatedCopy) {
		t.Fatalf("MergeGapfill mutated the generated models map: got %#v, want %#v", generated, generatedCopy)
	}
	if got := merged[entry.Model].Aliases; !reflect.DeepEqual(got, []string{"aaa-alias", "zzz-alias"}) {
		t.Fatalf("merged aliases are not sorted: %v", got)
	}
}

// TestValidateGapfillEstimatesRequiresARealVendorPricedBasis closes the gap
// ParseGapfill structurally cannot: it sees only the curated file, so it can
// require that a basis model is named but not that the name refers to a real
// vendor-priced model carrying exactly the rate the estimate borrows. Without
// this check an "estimate" could be an invented figure wearing a basis model's
// name, which is precisely what the curation bar exists to prevent.
func TestValidateGapfillEstimatesRequiresARealVendorPricedBasis(t *testing.T) {
	upstream := func() map[string]modelPrice {
		return map[string]modelPrice{
			"gpt-5.3-codex": {Provider: "openai", EffectiveFrom: BundledCatalogEffectiveFrom, Prices: map[string]string{
				"input": "1.750000000", "cached_input": "0.175000000", "output": "14.000000000",
			}},
			"claude-basis": {Provider: "anthropic", EffectiveFrom: BundledCatalogEffectiveFrom, Prices: map[string]string{
				"input": "3.000000000", "output": "15.000000000",
			}},
		}
	}
	tests := []struct {
		name    string
		mutate  func(*GapfillEntry)
		wantErr string
	}{
		{
			name:   "estimate carrying its basis model's rate in a different decimal form",
			mutate: func(*GapfillEntry) {},
		},
		{
			name:    "basis model upstream does not price",
			mutate:  func(e *GapfillEntry) { e.BasisModel = "gpt-does-not-exist" },
			wantErr: "which the upstream catalog does not price",
		},
		{
			name:    "basis model belongs to another vendor",
			mutate:  func(e *GapfillEntry) { e.BasisModel = "claude-basis" },
			wantErr: "must borrow its own vendor's rate",
		},
		{
			name:    "estimate drifted from its basis rate",
			mutate:  func(e *GapfillEntry) { e.Prices["input"] = "2.00" },
			wantErr: "an equivalent estimate must carry the basis model's vendor rate",
		},
		{
			name:    "estimate prices a component its basis does not",
			mutate:  func(e *GapfillEntry) { e.BasisModel = "claude-basis"; e.Provider = "anthropic" },
			wantErr: "which basis model claude-basis does not price",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := validGapfillEntry()
			asEstimate(&entry)
			entry.Prices = map[string]string{"input": "1.75", "cached_input": "0.175", "output": "14"}
			test.mutate(&entry)
			err := validateGapfillEstimates(upstream(), Gapfill{SchemaVersion: 1, Entries: []GapfillEntry{entry}})
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("valid estimate rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("estimate accepted but should have been rejected for %q", test.wantErr)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("rejected for the wrong reason:\n got: %v\nwant substring: %s", err, test.wantErr)
			}
		})
	}

	// A plain vendor rate is not held to the basis contract; only estimates
	// borrow a rate, so only estimates can drift from one.
	vendor := validGapfillEntry()
	vendor.Prices = map[string]string{"input": "999"}
	if err := validateGapfillEstimates(upstream(), Gapfill{SchemaVersion: 1, Entries: []GapfillEntry{vendor}}); err != nil {
		t.Fatalf("vendor-rate entry held to the estimate basis contract: %v", err)
	}
}
