package usage

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed price-gapfill.json
var bundledGapfill []byte

// GapfillEntry is a curated vendor price for a model the upstream catalog does
// not price. It is a separate input from the generated bundled catalog so
// regenerating that catalog cannot silently drop it (a generated-from-upstream
// file necessarily loses any model upstream has no priced row for).
//
// Entries are merged into the bundled layer, never official: bundled loses to
// a fresher litellm catalog on catalog_effective, so upstream automatically
// takes over if it ever starts pricing the model, whereas official would
// outrank upstream forever and freeze a stale price.
type GapfillEntry struct {
	Model    string   `json:"model"`
	Provider string   `json:"provider"`
	Aliases  []string `json:"aliases,omitempty"`
	// Kind is GapfillKindVendorRate when empty. GapfillKindEquivalentEstimate
	// marks a price that is explicitly not a published vendor rate for this
	// model; it then requires BasisModel and PricingNote.
	Kind          string            `json:"kind,omitempty"`
	BasisModel    string            `json:"basis_model,omitempty"`
	PricingNote   string            `json:"pricing_note,omitempty"`
	EffectiveFrom string            `json:"effective_from"`
	SourceURL     string            `json:"source_url"`
	VerifiedBy    string            `json:"verified_by"`
	VerifiedOn    string            `json:"verified_on"`
	Prices        map[string]string `json:"prices_per_million"`
}

const (
	// GapfillKindVendorRate is a rate the vendor publishes for this exact
	// model. It is the default and the only kind that may be presented as a
	// plain price.
	GapfillKindVendorRate = "vendor_rate"
	// GapfillKindEquivalentEstimate is an explicitly disclosed equivalent
	// token-cost estimate for a real, released, subscription-only model the
	// vendor prices nowhere. It borrows a named vendor-priced basis model's
	// rate rather than inventing a figure, and it is never a claim about what
	// the user's subscription actually bills.
	GapfillKindEquivalentEstimate = "equivalent_estimate"
)

// PriceKind returns the entry's kind with the default applied, so callers do
// not have to distinguish an absent field from an explicit vendor rate.
func (e GapfillEntry) PriceKind() string {
	if e.Kind == "" {
		return GapfillKindVendorRate
	}
	return e.Kind
}

// GapfillPending records a model that is known to be unpriced upstream but
// whose rate could not be confirmed to the curation bar. Keeping it in the
// file makes the gap deliberate and reviewable instead of forgotten.
type GapfillPending struct {
	Model                    string            `json:"model"`
	Provider                 string            `json:"provider"`
	ReportedPricesPerMillion map[string]string `json:"reported_prices_per_million,omitempty"`
	BlockedOn                string            `json:"blocked_on"`
	WhyNotShipped            string            `json:"why_not_shipped"`
	ToShip                   string            `json:"to_ship"`
}

type Gapfill struct {
	SchemaVersion int              `json:"schema_version"`
	Comment       string           `json:"comment"`
	CurationBar   string           `json:"curation_bar"`
	Entries       []GapfillEntry   `json:"entries"`
	Pending       []GapfillPending `json:"pending"`
}

// gapfillVendorHosts is the set of rate-card domains each provider's prices
// may be cited from. The curation bar demands a *vendor* rate card, so an
// aggregator that merely republishes a figure — models.dev, OpenRouter,
// typingmind — cannot satisfy it however trustworthy it looks; those are the
// very sources whose apparent agreement turned out to be one unconfirmed
// number copied around.
var gapfillVendorHosts = map[string][]string{
	"openai":    {"openai.com"},
	"anthropic": {"anthropic.com"},
}

// gapfillSupportedComponents is the set of price components the cost
// calculation reads *for that provider*, mirroring priceAt's two branches:
// the codex/openai branch reads input, cached_input, and output, while the
// claude/anthropic branch reads input, output, cache_read, and the two
// cache_write tiers. The split matters — a component that is valid for the
// other vendor still prices no token here, so an openai entry carrying only
// cache_read, or an anthropic entry carrying only cached_input, would look
// curated while contributing nothing.
var gapfillSupportedComponents = map[string]map[string]bool{
	"openai": {
		"input":        true,
		"output":       true,
		"cached_input": true,
	},
	"anthropic": {
		"input":          true,
		"output":         true,
		"cache_read":     true,
		"cache_write_5m": true,
		"cache_write_1h": true,
	},
}

// vendorHostMatches reports whether host is the vendor domain or a subdomain
// of it, so platform.openai.com passes for openai while an unrelated host
// that merely ends in the same text (notopenai.com) does not.
func vendorHostMatches(host, domain string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if idx := strings.LastIndex(host, ":"); idx != -1 && !strings.Contains(host[idx:], "]") {
		host = host[:idx]
	}
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// ParseGapfill validates a curated gap-fill file against the curation bar:
// every shipped entry needs a rate-card URL on its own provider's vendor
// domain, a named human verifier with a date, a parseable effective date, and
// at least one price component the cost calculation actually reads.
// An unconfirmable model belongs in Pending, not Entries.
func ParseGapfill(data []byte) (Gapfill, error) {
	var g Gapfill
	if err := json.Unmarshal(data, &g); err != nil {
		return g, err
	}
	if g.SchemaVersion != 1 {
		return g, fmt.Errorf("unsupported gap-fill schema_version %d", g.SchemaVersion)
	}
	seen := map[string]bool{}
	for _, e := range g.Entries {
		if e.Model == "" {
			return g, fmt.Errorf("gap-fill entry has no model")
		}
		if seen[e.Model] {
			return g, fmt.Errorf("gap-fill entry %s is duplicated", e.Model)
		}
		seen[e.Model] = true
		if e.Provider != "openai" && e.Provider != "anthropic" {
			return g, fmt.Errorf("gap-fill entry %s has provider %q, want openai or anthropic", e.Model, e.Provider)
		}
		switch e.PriceKind() {
		case GapfillKindVendorRate:
			// A vendor rate that carries estimate metadata would either be
			// mislabelled or would disclose an estimate it does not make.
			if strings.TrimSpace(e.BasisModel) != "" || strings.TrimSpace(e.PricingNote) != "" {
				return g, fmt.Errorf("gap-fill entry %s is a %s but carries basis_model/pricing_note; set kind to %s if it is not a published rate for this model", e.Model, GapfillKindVendorRate, GapfillKindEquivalentEstimate)
			}
		case GapfillKindEquivalentEstimate:
			if strings.TrimSpace(e.BasisModel) == "" {
				return g, fmt.Errorf("gap-fill entry %s is an %s and must name the vendor-priced basis_model its rate comes from", e.Model, GapfillKindEquivalentEstimate)
			}
			if e.BasisModel == e.Model {
				return g, fmt.Errorf("gap-fill entry %s names itself as basis_model; an estimate must borrow another model's vendor rate", e.Model)
			}
			if strings.TrimSpace(e.PricingNote) == "" {
				return g, fmt.Errorf("gap-fill entry %s is an %s and must carry a pricing_note stating that the value is an equivalent estimate, not an actual subscription invoice", e.Model, GapfillKindEquivalentEstimate)
			}
		default:
			return g, fmt.Errorf("gap-fill entry %s has kind %q, want %s or %s", e.Model, e.Kind, GapfillKindVendorRate, GapfillKindEquivalentEstimate)
		}
		if _, err := time.Parse(time.RFC3339Nano, e.EffectiveFrom); err != nil {
			return g, fmt.Errorf("gap-fill entry %s has invalid effective_from %q: %w", e.Model, e.EffectiveFrom, err)
		}
		parsed, err := url.Parse(e.SourceURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return g, fmt.Errorf("gap-fill entry %s needs a real https vendor rate-card source_url, got %q", e.Model, e.SourceURL)
		}
		domains := gapfillVendorHosts[e.Provider]
		matched := false
		for _, domain := range domains {
			if vendorHostMatches(parsed.Host, domain) {
				matched = true
				break
			}
		}
		if !matched {
			return g, fmt.Errorf("gap-fill entry %s cites %q, which is not a %s vendor rate card (want %s or a subdomain); an aggregator republishing a figure does not meet the curation bar", e.Model, parsed.Host, e.Provider, strings.Join(domains, " or "))
		}
		if strings.TrimSpace(e.VerifiedBy) == "" {
			return g, fmt.Errorf("gap-fill entry %s needs verified_by naming the human who checked the vendor rate card", e.Model)
		}
		if _, err := time.Parse("2006-01-02", e.VerifiedOn); err != nil {
			return g, fmt.Errorf("gap-fill entry %s needs verified_on as YYYY-MM-DD, got %q", e.Model, e.VerifiedOn)
		}
		if len(e.Prices) == 0 {
			return g, fmt.Errorf("gap-fill entry %s has no prices_per_million", e.Model)
		}
		allowed := gapfillSupportedComponents[e.Provider]
		supported := 0
		for component, value := range e.Prices {
			if !allowed[component] {
				return g, fmt.Errorf("gap-fill entry %s carries price component %q, which the cost calculation does not read for provider %s; it reads only %s", e.Model, component, e.Provider, strings.Join(sortedSupportedComponents(e.Provider), ", "))
			}
			if _, err := decimal(value); err != nil {
				return g, fmt.Errorf("gap-fill entry %s component %s has invalid price %q", e.Model, component, value)
			}
			supported++
		}
		if supported == 0 {
			return g, fmt.Errorf("gap-fill entry %s carries no price component supported for provider %s; it would price nothing", e.Model, e.Provider)
		}
	}
	for _, p := range g.Pending {
		if p.Model == "" {
			return g, fmt.Errorf("gap-fill pending entry has no model")
		}
		if seen[p.Model] {
			return g, fmt.Errorf("model %s is both shipped and pending in the gap-fill file", p.Model)
		}
		if strings.TrimSpace(p.BlockedOn) == "" {
			return g, fmt.Errorf("gap-fill pending entry %s must record what it is blocked_on", p.Model)
		}
	}
	return g, nil
}

// BundledGapfill returns the curated gap-fill compiled into this binary.
func BundledGapfill() (Gapfill, error) { return ParseGapfill(bundledGapfill) }

// bundledEstimateEntries indexes the compiled gap-fill's equivalent estimates
// by provider and model. Estimate disclosure is derived from this compiled
// input plus the provenance of the components that actually won, so no schema
// migration is needed to carry the marker in the database: the bundled catalog
// row and the binary that shipped it always travel together.
//
// Keyed on the model name only. Aliases are deliberately not indexed: price
// rows are keyed on model_prices.model, so an alias can never appear as the
// model of an effective price, and indexing them would be unreachable code
// implying a lookup that cannot happen.
var bundledEstimateEntries = sync.OnceValues(func() (map[string]GapfillEntry, error) {
	g, err := BundledGapfill()
	if err != nil {
		return nil, err
	}
	estimates := map[string]GapfillEntry{}
	for _, e := range g.Entries {
		if e.PriceKind() != GapfillKindEquivalentEstimate {
			continue
		}
		estimates[e.Provider+"\x00"+e.Model] = e
	}
	return estimates, nil
})

// compiledBundledCatalogVersion is the catalog_version of the catalog embedded
// in this binary. Estimate disclosure is matched against it so the marker
// attaches only to prices that came from the catalog which actually carries
// the curated estimate, not to any row that merely sits in the bundled layer:
// several bundled catalogs from different releases can coexist in one
// database, and only one of them is the compiled estimate's own.
var compiledBundledCatalogVersion = sync.OnceValues(func() (string, error) {
	c, err := parseCatalog(bundledCatalog)
	if err != nil {
		return "", err
	}
	return c.Version, nil
})

// MergeGapfill overlays curated entries onto a generated catalog document,
// returning the merged document. Curated entries win over generated ones for
// the same model: the generated catalog comes from upstream, and an entry is
// only curated because upstream's row is missing or unpriced.
func MergeGapfill(models map[string]modelPrice, g Gapfill) map[string]modelPrice {
	merged := make(map[string]modelPrice, len(models)+len(g.Entries))
	for name, p := range models {
		merged[name] = p
	}
	for _, e := range g.Entries {
		// Clone before sorting: `aliases := e.Aliases` would share the
		// caller's backing array and sort their slice in place, which this
		// function documents it does not do.
		aliases := append([]string(nil), e.Aliases...)
		sort.Strings(aliases)
		// Render curated values exactly as the upstream conversion renders
		// its own, so one merged catalog does not carry "1.75" next to
		// "1.750000000" for the same unit and the same vendor rate.
		//
		// An unparseable value is kept verbatim rather than dropped or
		// rejected: ParseGapfill already refuses such an entry, so this can
		// only be reached by a caller that built a Gapfill by hand, and
		// silently rewriting or discarding that caller's value would hide
		// their mistake instead of letting the catalog's own validation
		// surface it.
		prices := make(map[string]string, len(e.Prices))
		for component, value := range e.Prices {
			if rat, err := decimal(value); err == nil {
				value = money(rat)
			}
			prices[component] = value
		}
		merged[e.Model] = modelPrice{
			Provider:      e.Provider,
			Aliases:       aliases,
			EffectiveFrom: e.EffectiveFrom,
			Prices:        prices,
		}
	}
	return merged
}

// validateGapfillEstimates checks every equivalent-estimate entry against the
// upstream models it claims equivalence to. ParseGapfill can only see the
// curated file, so it can require that a basis model is *named*; only here,
// where the generated upstream catalog is in hand, can it be required that the
// basis model is real, vendor-priced, and carries exactly the rate the estimate
// borrows. Without this an "estimate" could quietly become an invented figure
// wearing a basis model's name.
func validateGapfillEstimates(models map[string]modelPrice, g Gapfill) error {
	for _, e := range g.Entries {
		if e.PriceKind() != GapfillKindEquivalentEstimate {
			continue
		}
		basis, ok := models[e.BasisModel]
		if !ok {
			return fmt.Errorf("gap-fill entry %s estimates from basis model %s, which the upstream catalog does not price; an estimate needs a vendor-priced basis", e.Model, e.BasisModel)
		}
		if basis.Provider != e.Provider {
			return fmt.Errorf("gap-fill entry %s (%s) estimates from basis model %s (%s); an estimate must borrow its own vendor's rate", e.Model, e.Provider, e.BasisModel, basis.Provider)
		}
		for _, component := range sortedComponents(e.Prices) {
			want, priced := basis.Prices[component]
			if !priced {
				return fmt.Errorf("gap-fill entry %s estimates component %s, which basis model %s does not price", e.Model, component, e.BasisModel)
			}
			got, err := decimal(e.Prices[component])
			if err != nil {
				return fmt.Errorf("gap-fill entry %s component %s has invalid price %q", e.Model, component, e.Prices[component])
			}
			basisValue, err := decimal(want)
			if err != nil {
				return fmt.Errorf("basis model %s component %s has invalid price %q", e.BasisModel, component, want)
			}
			if got.Cmp(basisValue) != 0 {
				return fmt.Errorf("gap-fill entry %s estimates %s at %s but basis model %s is priced at %s; an equivalent estimate must carry the basis model's vendor rate, so re-confirm the estimate rather than letting it drift", e.Model, component, e.Prices[component], e.BasisModel, want)
			}
		}
	}
	return nil
}

func sortedComponents(prices map[string]string) []string {
	names := make([]string, 0, len(prices))
	for name := range prices {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedSupportedComponents(provider string) []string {
	allowed := gapfillSupportedComponents[provider]
	names := make([]string, 0, len(allowed))
	for name := range allowed {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
