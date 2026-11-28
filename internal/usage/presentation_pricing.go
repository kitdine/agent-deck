package usage

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// PricingPresentation serves the application. The Widget continues to receive
// PresentationReport's legacy payment estimates and coverage.
type PricingPresentation struct {
	Version    int            `json:"version"`
	ValidUntil string         `json:"valid_until"`
	Scopes     []PricingScope `json:"scopes"`
}

type PricingScope struct {
	Client  string              `json:"client"`
	Periods []PricingPeriod     `json:"periods"`
	Daily   []PricingTimeBucket `json:"daily"`
	Hourly  []PricingTimeBucket `json:"hourly"`
	Rhythm  []PricingTimeBucket `json:"rhythm,omitempty"`
}

type PricingPeriod struct {
	Period  string             `json:"period"`
	Pricing *PricingReport     `json:"pricing"`
	Models  []PricingRow       `json:"models"`
	Tiers   []PricingRow       `json:"tiers"`
	Quality []PricingRow       `json:"quality"`
	Peak    *PricingTimeBucket `json:"peak"`
}

type PricingRow struct {
	Key      string         `json:"key"`
	Client   string         `json:"client,omitempty"`
	Provider string         `json:"provider,omitempty"`
	Tokens   int64          `json:"tokens"`
	Share    *string        `json:"share"`
	Pricing  *PricingReport `json:"pricing"`
}

type PricingTimeBucket struct {
	Key     string         `json:"key"`
	Pricing *PricingReport `json:"pricing"`
}

func buildPricingPresentation(scopes map[string]*presentationScopeAccumulator, periods []presentationPeriodDefinition, window presentationPricingWindow) *PricingPresentation {
	out := &PricingPresentation{Version: RequestPricingVersion, ValidUntil: window.End.Format(time.RFC3339Nano), Scopes: []PricingScope{}}
	for _, client := range []string{"all", "codex", "claude"} {
		value := scopes[client]
		scope := PricingScope{Client: client, Periods: []PricingPeriod{}, Daily: []PricingTimeBucket{}, Hourly: []PricingTimeBucket{}}
		for date, day := range value.daily {
			scope.Daily = append(scope.Daily, PricingTimeBucket{Key: date, Pricing: compactPricing(&day.stats.pricing, window.RuleSet)})
		}
		for hour, value := range value.hourly {
			scope.Hourly = append(scope.Hourly, PricingTimeBucket{Key: strconv.Itoa(hour), Pricing: compactPricing(&value.stats.pricing, window.RuleSet)})
		}
		byKey := func(a, b PricingTimeBucket) int { return strings.Compare(a.Key, b.Key) }
		slices.SortFunc(scope.Daily, byKey)
		slices.SortFunc(scope.Hourly, func(a, b PricingTimeBucket) int {
			left, _ := strconv.Atoi(a.Key)
			right, _ := strconv.Atoi(b.Key)
			return left - right
		})
		if client == "all" {
			for cell := range 7 * 24 {
				if bucket := value.rhythm[[2]int{cell / 24, cell % 24}]; bucket != nil {
					scope.Rhythm = append(scope.Rhythm, PricingTimeBucket{Key: strconv.Itoa(cell), Pricing: compactPricing(&bucket.stats.pricing, window.RuleSet)})
				}
			}
		}
		for _, period := range periods {
			scope.Periods = append(scope.Periods, buildPricingPeriod(value, scope, period, window.RuleSet))
		}
		out.Scopes = append(out.Scopes, scope)
	}
	return out
}

func buildPricingPeriod(value *presentationScopeAccumulator, scope PricingScope, period presentationPeriodDefinition, ruleSet string) PricingPeriod {
	p := value.periods[period.name]
	total := &p.total.stats.pricing
	out := PricingPeriod{Period: period.name, Pricing: total.result(ruleSet, true), Models: []PricingRow{}, Tiers: []PricingRow{}, Quality: []PricingRow{}}
	for key, model := range p.models {
		client, name, _ := strings.Cut(key, "\x00")
		out.Models = append(out.Models, pricingRow(name, client, "", model.stats, total, ruleSet))
	}
	for key, tier := range p.tiers {
		out.Tiers = append(out.Tiers, pricingRow(key, "", "", tier, total, ruleSet))
	}
	byProvider := value.quality[period.name]
	providers := make([]string, 0, len(byProvider)+1)
	providers = append(providers, "")
	for provider := range byProvider {
		if provider != "" {
			providers = append(providers, provider)
		}
	}
	for _, provider := range providers {
		for _, key := range []string{"determinable", "inferred", "unattributed"} {
			tier := byProvider[provider][key]
			if tier == nil {
				out.Quality = append(out.Quality, PricingRow{Key: key, Provider: provider, Pricing: (&pricingReportFold{}).result(ruleSet, false)})
				continue
			}
			row := pricingRow(key, "", provider, tier.stats, total, ruleSet)
			if key == "unattributed" {
				row.Pricing.ProviderEstimate = nil
			}
			out.Quality = append(out.Quality, row)
		}
	}
	order := func(a, b PricingRow) int {
		if completeComparable(total.api, total.events, total.api, total.events) {
			_, aa, _ := a.Pricing.CompleteAPI()
			_, bb, _ := b.Pricing.CompleteAPI()
			if cmp := statsKnownMetricRat(bb).Cmp(statsKnownMetricRat(aa)); cmp != 0 {
				return cmp
			}
		}
		if a.Tokens > b.Tokens {
			return -1
		}
		if a.Tokens < b.Tokens {
			return 1
		}
		return strings.Compare(a.Client+"\x00"+a.Key, b.Client+"\x00"+b.Key)
	}
	slices.SortFunc(out.Models, order)
	slices.SortFunc(out.Tiers, order)
	slices.SortFunc(out.Quality, func(a, b PricingRow) int {
		if a.Provider != b.Provider {
			return strings.Compare(a.Provider, b.Provider)
		}
		return order(a, b)
	})
	out.Models = out.Models[:min(len(out.Models), presentationModelLimit)]
	if _, amount, ok := out.Pricing.CompleteAPI(); !ok || statsKnownMetricRat(amount).Sign() == 0 {
		return out
	}
	buckets := scope.Daily
	start := period.start.Format("2006-01-02")
	if period.name == "today" {
		buckets, start = scope.Hourly, ""
	}
	for _, candidate := range buckets {
		if candidate.Key < start {
			continue
		}
		currency, amount, ok := candidate.Pricing.CompleteAPI()
		if !ok || currency != out.Pricing.APIEquivalent.Currency {
			continue
		}
		if out.Peak != nil {
			_, previous, _ := out.Peak.Pricing.CompleteAPI()
			if statsKnownMetricRat(amount).Cmp(statsKnownMetricRat(previous)) <= 0 {
				continue
			}
		}
		out.Peak = &candidate
	}
	return out
}

func pricingRow(key, client, provider string, part *statsAccumulator, total *pricingReportFold, ruleSet string) PricingRow {
	row := PricingRow{Key: key, Client: client, Provider: provider, Tokens: part.tokens, Pricing: compactPricing(&part.pricing, ruleSet)}
	if completeComparable(total.api, total.events, part.pricing.api, part.pricing.events) {
		row.Share = percentRat(&part.pricing.api.sum, &total.api.sum)
	}
	return row
}

func compactPricing(fold *pricingReportFold, ruleSet string) *PricingReport {
	pricing := fold.result(ruleSet, false)
	pricing.Sources = nil
	return pricing
}

// ValidAt is used by both desktop publication layers for cutoff-dependent
// references. The optional projection is absent from older snapshots.
func (p *PricingPresentation) ValidAt(at time.Time) (bool, error) {
	if p == nil {
		return true, nil
	}
	end, err := time.Parse(time.RFC3339Nano, p.ValidUntil)
	if err != nil {
		return false, fmt.Errorf("pricing valid_until: %w", err)
	}
	return at.Before(end), nil
}
