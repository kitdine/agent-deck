package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kitdine/agent-deck/internal/usage"
)

const guardianCoverageCopy = "Codex approval review usage may be missing"

func apiReportText(p *usage.PricingReport) (string, string) {
	basis, currency, amount := p.DisplayAPI()
	switch basis {
	case "complete":
		return currency + " " + amount, basis
	case "known":
		return currency + " " + amount + " known", "partial"
	case "reference":
		return "Reference " + currency + " " + amount + " · " + pricingReferenceBasis(*p.Reference), basis
	default:
		return "Unavailable", basis
	}
}

func pricingReferenceBasis(r usage.PricingReference) string {
	switch r.Basis {
	case "as_of_date":
		return "prices as of " + r.AsOf
	case "legacy_catalog":
		return "legacy catalog"
	case "standard":
		return "Standard tier assumed"
	case "requested_tier":
		return titlePricing(r.Tier) + " requested tier assumed"
	case "per_request_context":
		return "request-level context assumed"
	case "assumed_5m":
		return "5m cache writes assumed"
	case "cache_write_ttl_range":
		return strings.Join(r.TTLCandidates, "/") + " cache writes · " + titlePricing(r.Tier)
	default:
		return titlePricing(r.Basis)
	}
}

func pricingReferenceText(r usage.PricingReference) string {
	if r.Amount != nil {
		return r.Currency + " " + *r.Amount + " · " + pricingReferenceBasis(r)
	}
	if r.Lower != nil && r.Upper != nil {
		return r.Currency + " " + *r.Lower + "–" + *r.Upper + " · " + pricingReferenceBasis(r)
	}
	return ""
}

func titlePricing(raw string) string {
	if raw == "" {
		return "Unknown"
	}
	return statsTitle(strings.ReplaceAll(raw, "_", " "))
}

func pricingReasonText(reason string) string {
	switch reason {
	case "cache_write_ttl_unverified":
		return "Cache write TTL unverified"
	case "model_unresolved":
		return "Model unresolved"
	case "historical_rule_missing":
		return "Historical tariff unavailable"
	case "service_tier_unverified":
		return "Executed tier unverified"
	case "context_policy_unverified":
		return "Context pricing scope unverified"
	case "provider_route_unverified":
		return "Billing path unverified"
	default:
		return titlePricing(reason)
	}
}

func providerEstimateText(amount *usage.PricingAmount) string {
	if amount == nil {
		return "Unavailable"
	}
	if amount.Amount != nil {
		return amount.Currency + " " + *amount.Amount
	}
	if amount.KnownAmount != nil {
		return amount.Currency + " " + *amount.KnownAmount + " known"
	}
	return "Unavailable"
}

type priceTextField struct{ label, value string }

func requestPricingFields(p *usage.PricingReport, details bool) []priceTextField {
	api, basis := apiReportText(p)
	fields := []priceTextField{{"API EQUIVALENT", api}}
	if details || p.ProviderEstimate != nil {
		fields = append(fields, priceTextField{"PROVIDER ESTIMATE", providerEstimateText(p.ProviderEstimate)})
	}
	var reasons []string
	for _, reason := range p.Reasons {
		if reason == usage.ActivityCoverageUnavailable {
			fields = append(fields, priceTextField{"COVERAGE", guardianCoverageCopy})
		} else if details {
			reasons = append(reasons, pricingReasonText(reason))
		}
	}
	if len(reasons) > 0 {
		fields = append(fields, priceTextField{"PRICING EVIDENCE", strings.Join(reasons, " · ")})
	}
	if !details {
		return fields
	}
	if p.Reference != nil && basis != "reference" {
		fields = append(fields, priceTextField{"REFERENCE", pricingReferenceText(*p.Reference)})
	}
	for _, reference := range p.ReferenceDetails {
		fields = append(fields, priceTextField{"REFERENCE SUBSET", pricingReferenceText(reference)})
	}
	if p.ObservedModel != "" {
		fields = append(fields, priceTextField{"OBSERVED MODEL", p.ObservedModel})
	}
	if p.PricedModel != "" {
		fields = append(fields, priceTextField{"PRICED MODEL", p.PricedModel})
	}
	if p.ExecutedTier != "" {
		fields = append(fields, priceTextField{"EXECUTED TIER", titlePricing(p.ExecutedTier)}, priceTextField{"CONTEXT TIER", titlePricing(p.ContextTier)})
	}
	if p.RequestedTier != "" {
		fields = append(fields, priceTextField{"REQUESTED TIER", titlePricing(p.RequestedTier)})
	}
	if p.Subscription != nil {
		billing := map[string]string{"api_key": "Official API key", "third_party": "Third-party route", "enterprise_usd": "Enterprise USD", "personal_subscription": "Personal subscription"}[p.Subscription.Channel]
		if billing == "" {
			billing = "Unverified"
		}
		if p.Attribution != "" {
			billing += " · " + titlePricing(p.Attribution) + " attribution"
		}
		fields = append(fields, priceTextField{"BILLING PATH", billing})
		status := titlePricing(p.Subscription.Status)
		if p.Subscription.Status == "free_activity" {
			status = "Auto-review · No extra charge · No quota used"
		} else if p.Subscription.SubscriptionLongContext == "exempt" {
			status += " · No subscription long-context surcharge"
		}
		if p.Subscription.PolicyID != "" {
			status += " · " + p.Subscription.PolicyID
		}
		fields = append(fields, priceTextField{"PLAN STATUS", status})
	}
	for _, component := range p.Components {
		fields = append(fields, priceTextField{strings.ToUpper(titlePricing(component.Kind)), pricingComponentText(component)})
	}
	for _, component := range p.ProviderComponents {
		fields = append(fields, priceTextField{"CHANNEL " + strings.ToUpper(titlePricing(component.Kind)), pricingComponentText(component)})
	}
	for _, source := range p.Sources {
		fields = append(fields, priceTextField{"PRICE RULE", pricingSourceText(source.RuleID, source.SourceURL, source.EffectiveFrom, source.EffectiveUntil)})
	}
	return fields
}

func pricingComponentText(c usage.PricingComponent) string {
	text := fmt.Sprint(c.Quantity)
	if c.Rate != nil {
		text += " × " + *c.Rate
		if c.Unit == "token" || c.Unit == usage.PriceRuleUSDPerMillion {
			text += " / 1M"
		} else {
			text += " " + c.Unit
		}
	}
	if c.Amount != nil {
		text += " = " + c.Currency + " " + *c.Amount
	}
	if c.Modifier != nil {
		text += " · multiplier " + c.Modifier.Multiplier + " · " + c.Modifier.RuleID + " · " + c.Modifier.SourceURL
	}
	if c.Treatment != nil {
		text += " · " + c.Treatment.ID + " · " + c.Treatment.RuleID + " · " + c.Treatment.SourceURL
	}
	return text + " · " + pricingSourceText(c.RuleID, c.SourceURL, c.EffectiveFrom, c.EffectiveUntil)
}

func pricingSourceText(id, source string, from, to *usage.PriceRuleBoundary) string {
	parts := []string{id}
	if from != nil {
		parts = append(parts, "from "+pricingBoundaryText(*from))
	}
	if to != nil {
		parts = append(parts, "until "+pricingBoundaryText(*to)+" (exclusive)")
	}
	if source != "" {
		parts = append(parts, source)
	}
	return strings.Join(parts, " · ")
}

func pricingBoundaryText(b usage.PriceRuleBoundary) string {
	out := b.Value + " (" + b.Precision
	if b.TimeZone != "" {
		out += ", " + b.TimeZone
	}
	return out + ")"
}

func priceTariffText(t usage.PriceTariff) string {
	s := t.Selector
	conditions := []string{s.Channel, strings.Join(s.Endpoints, "/"), s.ServiceTier}
	if s.Context != nil {
		conditions = append(conditions, fmt.Sprintf("%s %s %d · %s", s.Context.Axis, s.Context.Comparison, s.Context.Tokens, s.Context.Mode))
	}
	conditions = append(conditions, strings.Join(s.Regions, "/"), s.Activity, s.Eligibility)
	var values []string
	for kind, rate := range t.Rates {
		values = append(values, kind+" "+rate)
	}
	sort.Strings(values)
	if len(values) > 0 {
		conditions = append(conditions, strings.Join(values, ", ")+" "+t.Currency+" / 1M tokens")
	}
	if t.Policy != nil {
		conditions = append(conditions, t.Policy.ID, t.Policy.Multiplier)
	}
	if t.AliasTo != "" {
		conditions = append(conditions, "alias to "+t.AliasTo)
	}
	conditions = append(conditions, t.Applicability, pricingSourceText(t.ID, t.Source.URL, &t.Validity.From, t.Validity.To))
	text := conditions[:0]
	for _, field := range conditions {
		if field != "" {
			text = append(text, field)
		}
	}
	return strings.Join(text, " · ")
}

func compactAPICost(p *usage.PricingReport) string {
	basis, _, amount := p.DisplayAPI()
	switch basis {
	case "complete":
		return compactCost(&amount, "", true)
	case "known":
		return compactCost(nil, amount, true)
	case "reference":
		return compactCost(&amount, "", true) + " REF"
	default:
		return "unavailable"
	}
}

func statsPricingLabel(p *usage.PricingReport) string {
	if p != nil {
		return "API EQUIV."
	}
	return "COST"
}

func statsAPIComparable(report usage.StatsReport) bool {
	return report.APIPeak != nil
}

func statsDimensionShare(item usage.StatsDimension, metric string) (float64, bool) {
	if item.Pricing != nil && strings.EqualFold(metric, "cost") {
		if item.APIShare == nil {
			return 0, false
		}
		share, err := strconv.ParseFloat(*item.APIShare, 64)
		return share, err == nil
	}
	share, err := strconv.ParseFloat(item.KnownShare, 64)
	return share, err == nil && (!strings.EqualFold(metric, "cost") || knownCostAvailable(item.MetricValue, item.KnownMetricValue, item.Coverage))
}

func statsDimensionCost(item usage.StatsDimension) string {
	if item.Pricing != nil {
		return compactAPICost(item.Pricing)
	}
	return compactCost(item.ProviderCost, item.KnownProviderCost, knownCostAvailable(item.ProviderCost, item.KnownProviderCost, item.Coverage))
}

func statsBucketCost(item usage.StatsBucket) string {
	if item.Pricing != nil {
		return compactAPICost(item.Pricing)
	}
	return compactCost(item.MetricValue, item.KnownMetricValue, knownCostAvailable(item.MetricValue, item.KnownMetricValue, item.Coverage))
}

func statsBucketMagnitude(item usage.StatsBucket, report usage.StatsReport) float64 {
	if report.Metric == "cost" && item.Pricing != nil {
		if !statsAPIComparable(report) {
			return float64(item.Tokens)
		}
		_, amount, _ := item.Pricing.CompleteAPI()
		value, _ := strconv.ParseFloat(amount, 64)
		return value
	}
	return statsBucketMetric(item, report.Metric)
}

func statsActivityMagnitude(item usage.StatsActivity, report usage.StatsReport) float64 {
	raw := item.KnownMetricValue
	if report.Metric == "cost" && report.Totals.Pricing != nil {
		if !statsAPIComparable(report) {
			return float64(item.Tokens)
		}
		_, raw, _ = item.Pricing.CompleteAPI()
	}
	value, _ := strconv.ParseFloat(raw, 64)
	return value
}
