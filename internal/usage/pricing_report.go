package usage

import (
	"errors"
	"math/big"
	"slices"
	"strings"
	"time"
)

const ActivityCoverageUnavailable = reasonCoverage

// PricingReport is the additive reporting projection. Its amounts are
// independent of every legacy cost and coverage field.
type PricingReport struct {
	Version            int                  `json:"version"`
	RuleSet            string               `json:"rule_set"`
	APIEquivalent      PricingAmount        `json:"api_equivalent"`
	ProviderEstimate   *PricingAmount       `json:"provider_estimate"`
	Reference          *PricingReference    `json:"reference"`
	Reasons            []string             `json:"reasons"`
	ReferenceDetails   []PricingReference   `json:"reference_details,omitempty"`
	Sources            []PricingRuleSource  `json:"sources,omitempty"`
	ObservedModel      string               `json:"observed_model,omitempty"`
	PricedModel        string               `json:"priced_model,omitempty"`
	RequestedTier      string               `json:"requested_tier,omitempty"`
	ExecutedTier       string               `json:"executed_tier,omitempty"`
	ContextTier        string               `json:"context_tier,omitempty"`
	Components         []PricingComponent   `json:"components,omitempty"`
	ProviderComponents []PricingComponent   `json:"provider_components,omitempty"`
	Subscription       *PricingSubscription `json:"subscription,omitempty"`
	Attribution        string               `json:"attribution,omitempty"`
}

type PricingRuleSource struct {
	RuleID         string             `json:"rule_id"`
	SourceURL      string             `json:"source_url"`
	EffectiveFrom  *PriceRuleBoundary `json:"effective_from,omitempty"`
	EffectiveUntil *PriceRuleBoundary `json:"effective_until,omitempty"`
}

func invocationReport(p InvocationPricing, attribution eventAttribution) *PricingReport {
	var sources []PricingRuleSource
	if p.Reference != nil {
		sources = p.Reference.sources
	}
	return &PricingReport{
		Version: p.Version, RuleSet: p.RuleSet, APIEquivalent: p.APIEquivalent,
		ProviderEstimate: p.ProviderEstimate, Reference: p.Reference, Reasons: append([]string{}, p.Reasons...),
		ObservedModel: p.ObservedModel, PricedModel: p.PricedModel, RequestedTier: p.RequestedTier,
		ExecutedTier: p.ExecutedTier, ContextTier: p.ContextTier, Components: p.Components,
		ProviderComponents: p.ProviderComponents, Subscription: &p.Subscription, Attribution: attribution.quality, Sources: sources,
	}
}

// In a group, compare only complete amounts sharing the same currency.
func (p *PricingReport) CompleteAPI() (currency, amount string, ok bool) {
	if p == nil || p.APIEquivalent.Status != pricingComplete || p.APIEquivalent.Amount == nil {
		return "", "", false
	}
	return p.APIEquivalent.Currency, *p.APIEquivalent.Amount, true
}

func (p *PricingReport) DisplayAPI() (basis, currency, amount string) {
	if currency, amount, ok := p.CompleteAPI(); ok {
		return "complete", currency, amount
	}
	if p == nil {
		return "unavailable", "", ""
	}
	if p.APIEquivalent.KnownAmount != nil {
		return "known", p.APIEquivalent.Currency, *p.APIEquivalent.KnownAmount
	}
	if p.Reference != nil && p.Reference.Amount != nil {
		switch p.Reference.Basis {
		case "standard", "as_of_date", "legacy_catalog", "assumed_5m":
			return "reference", p.Reference.Currency, *p.Reference.Amount
		}
	}
	return "unavailable", "", ""
}

type pricingAmountFold struct {
	currency        string
	conflict        bool
	known, complete int
	sum             big.Rat
}

func (f *pricingAmountFold) add(value *PricingAmount) {
	if value == nil {
		return
	}
	raw := value.KnownAmount
	if value.Status == pricingComplete && value.Amount != nil {
		raw = value.Amount
		f.complete++
	}
	if raw == nil {
		return
	}
	if f.currency == "" {
		f.currency = value.Currency
	}
	if value.Currency != f.currency {
		f.conflict = true
		return
	}
	if amount, err := decimal(*raw); err == nil {
		f.known++
		f.sum.Add(&f.sum, amount)
	}
}

func (f *pricingAmountFold) result(events int) PricingAmount {
	if events == 0 {
		zero := "0.000000000"
		return PricingAmount{Currency: pricingCurrency, Status: pricingComplete, Amount: &zero, KnownAmount: &zero}
	}
	value := PricingAmount{Currency: pricingCurrency, Status: pricingUnavailable}
	if f.known == 0 || f.conflict {
		return value
	}
	value.Currency = f.currency
	known := exactPrice(&f.sum, 9)
	value.KnownAmount = &known
	value.Status = pricingPartial
	if f.known == events && f.complete == events {
		value.Status = pricingComplete
		value.Amount = &known
	}
	return value
}

type pricingReferenceFold struct {
	template       PricingReference
	count          int
	sum, low, high big.Rat
}

func referenceKey(r PricingReference) string {
	return strings.Join([]string{r.Basis, r.Currency, r.AsOf, r.Tier, r.ContextUnit,
		strings.Join(r.TTLCandidates, ","), boolString(r.Amount != nil)}, "\x00")
}

func boolString(value bool) string {
	if value {
		return "amount"
	}
	return "interval"
}

func (f *pricingReferenceFold) add(r *PricingReference) {
	if r.Amount != nil {
		n, err := decimal(*r.Amount)
		if err != nil {
			return
		}
		f.sum.Add(&f.sum, n)
	} else if r.Lower != nil && r.Upper != nil {
		low, lowErr := decimal(*r.Lower)
		high, highErr := decimal(*r.Upper)
		if lowErr != nil || highErr != nil {
			return
		}
		f.low.Add(&f.low, low)
		f.high.Add(&f.high, high)
	} else {
		return
	}
	f.count++
}

func (f *pricingReferenceFold) result() PricingReference {
	value := f.template
	if value.Amount != nil {
		amount := exactPrice(&f.sum, 9)
		value.Amount = &amount
	} else {
		lower, upper := exactPrice(&f.low, 9), exactPrice(&f.high, 9)
		value.Lower, value.Upper = &lower, &upper
	}
	return value
}

type pricingReportFold struct {
	events        int
	codex         bool
	api, provider pricingAmountFold
	reasons       map[string]struct{}
	references    map[string]*pricingReferenceFold
	sources       map[string]PricingRuleSource
}

func (f *pricingReportFold) add(client string, priced *PricingReport) {
	f.events++
	f.codex = f.codex || client == "codex"
	f.api.add(&priced.APIEquivalent)
	f.provider.add(priced.ProviderEstimate)
	if len(priced.Reasons) > 0 && f.reasons == nil {
		f.reasons = make(map[string]struct{}, len(priced.Reasons))
	}
	for _, reason := range priced.Reasons {
		f.reasons[reason] = struct{}{}
	}
	if r := priced.Reference; r != nil {
		if f.references == nil {
			f.references = map[string]*pricingReferenceFold{}
		}
		key := referenceKey(*r)
		if f.references[key] == nil {
			f.references[key] = &pricingReferenceFold{template: *r}
		}
		f.references[key].add(r)
	}
	for _, source := range priced.Sources {
		if f.sources == nil {
			f.sources = map[string]PricingRuleSource{}
		}
		f.sources[pricingSourceKey(source)] = source
	}
}

func (f *pricingReportFold) result(ruleSet string, period bool) *PricingReport {
	out := &PricingReport{Version: RequestPricingVersion, RuleSet: ruleSet, APIEquivalent: f.api.result(f.events), Reasons: []string{}}
	if f.events == 0 {
		return out
	}
	if provider := f.provider.result(f.events); provider.Status != pricingUnavailable {
		out.ProviderEstimate = &provider
	}
	for reason := range f.reasons {
		out.Reasons = append(out.Reasons, reason)
	}
	if period && f.codex {
		out.Reasons = append(out.Reasons, reasonCoverage)
	}
	slices.Sort(out.Reasons)
	out.Reasons = slices.Compact(out.Reasons)
	for _, reference := range f.references {
		if reference.count == 0 {
			continue
		}
		value := reference.result()
		if reference.count == f.events {
			out.Reference = &value
		} else {
			out.ReferenceDetails = append(out.ReferenceDetails, value)
		}
	}
	slices.SortFunc(out.ReferenceDetails, func(a, b PricingReference) int {
		return strings.Compare(referenceKey(a), referenceKey(b))
	})
	for _, source := range f.sources {
		out.Sources = append(out.Sources, source)
	}
	slices.SortFunc(out.Sources, func(a, b PricingRuleSource) int {
		return strings.Compare(pricingSourceKey(a), pricingSourceKey(b))
	})
	return out
}

func pricingContribution(p InvocationPricing) *PricingReport {
	out := &PricingReport{
		Version: p.Version, RuleSet: p.RuleSet, APIEquivalent: p.APIEquivalent,
		ProviderEstimate: p.ProviderEstimate, Reference: p.Reference, Reasons: append([]string{}, p.Reasons...), ExecutedTier: p.ExecutedTier,
	}
	out.Sources = append(pricingSources(p.Components), pricingSources(p.ProviderComponents)...)
	if p.Reference != nil {
		out.Sources = append(out.Sources, p.Reference.sources...)
		ref := *p.Reference
		ref.sources = nil // provenance travels in Sources through the partition cache
		out.Reference = &ref
	}
	return out
}

func pricingSourceKey(source PricingRuleSource) string {
	boundary := func(value *PriceRuleBoundary) string {
		if value == nil {
			return ""
		}
		return strings.Join([]string{value.Value, value.Precision, value.TimeZone}, "\x00")
	}
	return strings.Join([]string{source.RuleID, source.SourceURL, boundary(source.EffectiveFrom), boundary(source.EffectiveUntil)}, "\x00")
}

func pricingSources(components []PricingComponent) []PricingRuleSource {
	var sources []PricingRuleSource
	for _, c := range components {
		if c.RuleID != "" && c.SourceURL != "" {
			sources = append(sources, PricingRuleSource{
				RuleID: c.RuleID, SourceURL: c.SourceURL, EffectiveFrom: c.EffectiveFrom, EffectiveUntil: c.EffectiveUntil,
			})
		}
	}
	return sources
}

func legacyPricingError(p InvocationPricing) error {
	if p.LegacyError != "" {
		return errors.New(p.LegacyError)
	}
	return nil
}

func completeComparable(total pricingAmountFold, events int, part pricingAmountFold, partEvents int) bool {
	return events > 0 && total.complete == events && total.known == events && !total.conflict && total.sum.Sign() > 0 &&
		partEvents > 0 && part.complete == partEvents && part.known == partEvents && !part.conflict && total.currency == part.currency
}

func statsReportOrder(left, right StatsDimension, metric string, total *PricingReport) (*big.Rat, *big.Rat) {
	if metric == "cost" {
		if _, cost, ok := total.CompleteAPI(); ok && statsKnownMetricRat(cost).Sign() > 0 {
			_, leftCost, _ := left.Pricing.CompleteAPI()
			_, rightCost, _ := right.Pricing.CompleteAPI()
			return statsKnownMetricRat(leftCost), statsKnownMetricRat(rightCost)
		}
		return big.NewRat(left.Tokens, 1), big.NewRat(right.Tokens, 1)
	}
	return statsKnownMetricRat(left.KnownMetricValue), statsKnownMetricRat(right.KnownMetricValue)
}

// As-of references can change during a UTC day at any warranted tariff
// boundary. A partition or publication is reusable only within this window.
func (r invocationRules) presentationWindow() presentationPricingWindow {
	day := r.cutoff.UTC().Truncate(24 * time.Hour)
	out := presentationPricingWindow{RuleSet: r.digest, Start: day, End: day.AddDate(0, 0, 1)}
	for _, rules := range r.models {
		for _, rule := range rules {
			for _, boundary := range []time.Time{rule.earliest, rule.start, rule.end, rule.latestEnd} {
				if boundary.After(r.cutoff) && boundary.Before(out.End) {
					out.End = boundary
				} else if !boundary.After(r.cutoff) && boundary.After(out.Start) {
					out.Start = boundary
				}
			}
		}
	}
	return out
}
