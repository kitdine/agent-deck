package usage

import (
	"context"
	"math/big"
	"sort"
	"strings"
	"time"
)

const (
	presentationDailyLimit    = 90
	presentationModelLimit    = 12
	presentationUnpricedLimit = 12
)

type PresentationReport struct {
	Available       bool                        `json:"available"`
	Scopes          []PresentationScope         `json:"scopes"`
	ClientSubtotals PresentationClientSubtotals `json:"client_subtotals"`
	Summary         Summary                     `json:"-"`
}

type PresentationScope struct {
	Client  string              `json:"client"`
	Periods PresentationPeriods `json:"periods"`
	Daily   PresentationDaily   `json:"daily"`
	Hourly  PresentationHourly  `json:"hourly"`
	Quality PresentationQuality `json:"quality"`
	Pricing PresentationPricing `json:"pricing"`
	Rhythm  PresentationRhythm  `json:"rhythm"`
}

type PresentationPeriods struct {
	Available bool                 `json:"available"`
	Items     []PresentationPeriod `json:"items"`
}

type PresentationPeriod struct {
	Period        string              `json:"period"`
	Totals        PresentationTotals  `json:"totals"`
	AveragePerDay PresentationAverage `json:"average_per_day"`
	Peak          PresentationPeak    `json:"peak"`
	CacheHitShare *string             `json:"cache_hit_share"`
	Models        []PresentationModel `json:"models"`
}

type PresentationTotals struct {
	Tokens               int64   `json:"tokens"`
	InputTokens          int64   `json:"input_tokens"`
	OutputTokens         int64   `json:"output_tokens"`
	CachedReadTokens     int64   `json:"cached_read_tokens"`
	CacheWriteTokens     int64   `json:"cache_write_tokens"`
	Events               int64   `json:"events"`
	Sessions             int64   `json:"sessions"`
	CatalogBaseCost      *string `json:"catalog_base_cost"`
	ProviderCost         *string `json:"provider_cost"`
	KnownCatalogBaseCost string  `json:"known_catalog_base_cost"`
	KnownProviderCost    string  `json:"known_provider_cost"`
	PricingComplete      bool    `json:"pricing_complete"`
	UnpricedComponents   int     `json:"unpriced_components"`
}

type PresentationAverage struct {
	Tokens            string  `json:"tokens"`
	ProviderCost      *string `json:"provider_cost"`
	KnownProviderCost string  `json:"known_provider_cost"`
}

type PresentationPeak struct {
	Date   string             `json:"date"`
	Totals PresentationTotals `json:"totals"`
}

// PresentationDisplayValue is the bounded subset used by rows and plotted
// buckets. Period totals keep the complete accounting tuple; these high-count
// collections do not repeat fields their consumers never read.
type PresentationDisplayValue struct {
	Tokens         int64  `json:"tokens"`
	Events         int64  `json:"events"`
	ProviderCost   string `json:"provider_cost"`
	CostIncomplete bool   `json:"cost_incomplete"`
}

type PresentationModel struct {
	Client string                   `json:"client,omitempty"`
	Model  string                   `json:"model"`
	Value  PresentationDisplayValue `json:"value"`
	Share  *string                  `json:"share"`
}

type PresentationDaily struct {
	Available bool                    `json:"available"`
	Items     []PresentationDailyItem `json:"items"`
}

type PresentationDailyItem struct {
	Date  string                   `json:"date"`
	Value PresentationDisplayValue `json:"value"`
}

type PresentationHourly struct {
	Available   bool                     `json:"available"`
	ThroughHour int                      `json:"through_hour"`
	Items       []PresentationHourlyItem `json:"items"`
}

type PresentationHourlyItem struct {
	Hour  int                      `json:"hour"`
	Value PresentationDisplayValue `json:"value"`
}

type PresentationQuality struct {
	Available bool                      `json:"available"`
	Items     []PresentationQualityItem `json:"items"`
}

type PresentationQualityItem struct {
	Period   string                    `json:"period"`
	Provider *string                   `json:"provider"`
	Tiers    []PresentationQualityTier `json:"tiers"`
}

type PresentationQualityTier struct {
	Quality string                   `json:"quality"`
	Value   PresentationDisplayValue `json:"value"`
	Share   *string                  `json:"share"`
}

type PresentationPricing struct {
	Available bool                      `json:"available"`
	Items     []PresentationPricingItem `json:"items"`
}

type PresentationPricingItem struct {
	Period              string   `json:"period"`
	PricedEvents        int64    `json:"priced_events"`
	UnpricedEvents      int64    `json:"unpriced_events"`
	Coverage            string   `json:"coverage"`
	UnpricedIdentifiers []string `json:"unpriced_identifiers"`
}

type PresentationRhythm struct {
	Available      bool     `json:"available"`
	Intensities    []int    `json:"intensities"`
	Tokens         []int64  `json:"tokens"`
	ProviderCosts  []string `json:"provider_costs"`
	CostIncomplete []bool   `json:"cost_incomplete"`
	ActiveDays     int      `json:"active_days"`
	BusiestDay     string   `json:"busiest_day"`
	QuietestDay    string   `json:"quietest_day"`
}

type PresentationClientSubtotals struct {
	Available bool                         `json:"available"`
	Items     []PresentationClientSubtotal `json:"items"`
}

type PresentationClientSubtotal struct {
	Period string                   `json:"period"`
	Client string                   `json:"client"`
	Value  PresentationDisplayValue `json:"value"`
}

type presentationPeriodDefinition struct {
	name  string
	days  int64
	start time.Time
}

type presentationAccumulator struct {
	stats        *statsAccumulator
	logicalInput int64
}

func newPresentationAccumulator() *presentationAccumulator {
	return &presentationAccumulator{stats: newStatsAccumulator()}
}

func (a *presentationAccumulator) add(event storedEvent, result Result) error {
	if err := a.stats.add(event, result); err != nil {
		return err
	}
	a.logicalInput += event.Tokens["input_tokens"]
	if event.Client == "claude" {
		a.logicalInput += event.Tokens["cache_read_tokens"] + event.Tokens["cache_write_5m_tokens"] + event.Tokens["cache_write_1h_tokens"]
		if event.Tokens["cache_write_5m_tokens"]+event.Tokens["cache_write_1h_tokens"] == 0 {
			a.logicalInput += event.Tokens["cache_creation_tokens"]
		}
	}
	return nil
}

type presentationPeriodAccumulator struct {
	total  *presentationAccumulator
	models map[string]*presentationAccumulator
}

func newPresentationPeriodAccumulator() *presentationPeriodAccumulator {
	return &presentationPeriodAccumulator{total: newPresentationAccumulator(), models: map[string]*presentationAccumulator{}}
}

type presentationScopeAccumulator struct {
	periods map[string]*presentationPeriodAccumulator
	daily   map[string]*presentationAccumulator
	hourly  map[int]*presentationAccumulator
	// quality, pricing and unpriced are keyed by period first: the contract
	// moves both families from client scope to the Client x Period product, so
	// a filtered panel reads one record rather than the current period only.
	quality  map[string]map[string]map[string]*presentationAccumulator
	pricing  map[string]*presentationAccumulator
	unpriced map[string]map[string]struct{}
	rhythm   map[[2]int]*presentationAccumulator
}

func newPresentationScopeAccumulator(periods []presentationPeriodDefinition) *presentationScopeAccumulator {
	value := &presentationScopeAccumulator{
		periods: map[string]*presentationPeriodAccumulator{}, daily: map[string]*presentationAccumulator{}, hourly: map[int]*presentationAccumulator{},
		quality: map[string]map[string]map[string]*presentationAccumulator{},
		pricing: map[string]*presentationAccumulator{}, unpriced: map[string]map[string]struct{}{},
		rhythm: map[[2]int]*presentationAccumulator{},
	}
	for _, period := range periods {
		value.periods[period.name] = newPresentationPeriodAccumulator()
		value.quality[period.name] = map[string]map[string]*presentationAccumulator{}
		value.pricing[period.name] = newPresentationAccumulator()
		value.unpriced[period.name] = map[string]struct{}{}
	}
	return value
}

func EmptyPresentationReport() PresentationReport {
	return PresentationReport{Scopes: []PresentationScope{}, ClientSubtotals: PresentationClientSubtotals{Items: []PresentationClientSubtotal{}}}
}

// Presentation performs one bounded event read and one in-memory aggregation
// for every menu-bar and Widget presentation dimension. Callers select already
// computed scopes and periods; they never regroup the daily series.
func (s *Service) Presentation(ctx context.Context, now time.Time, location *time.Location) (PresentationReport, error) {
	if location == nil {
		location = time.Local
	}
	today := localDateStart(now, location)
	currentHour := now.In(location).Hour()
	end := today.AddDate(0, 0, 1)
	start90 := today.AddDate(0, 0, -(presentationDailyLimit - 1))
	periods := []presentationPeriodDefinition{
		{name: "today", days: 1, start: today},
		{name: "7d", days: 7, start: today.AddDate(0, 0, -6)},
		{name: "30d", days: 30, start: today.AddDate(0, 0, -29)},
	}

	events, err := s.eventsRange(ctx, start90, end, "", "")
	if err != nil {
		return PresentationReport{}, err
	}
	resolver, err := s.loadReadPriceResolver(ctx, now.UTC())
	if err != nil {
		return PresentationReport{}, err
	}
	scopes := map[string]*presentationScopeAccumulator{
		"all": newPresentationScopeAccumulator(periods), "codex": newPresentationScopeAccumulator(periods), "claude": newPresentationScopeAccumulator(periods),
	}
	summary := newPresentationSummaryBuilder()

	for _, event := range events {
		at, parseErr := time.Parse(time.RFC3339Nano, event.EventAt)
		if parseErr != nil {
			return PresentationReport{}, parseErr
		}
		at = at.In(location)
		attribution, attributionErr := resolver.priceForEvent(event)
		if attributionErr != nil {
			return PresentationReport{}, attributionErr
		}
		calculated, calculateErr := Calculate(event.Client, event.Model, event.Tokens, attribution.price, attribution.multiplier)
		if calculateErr != nil {
			return PresentationReport{}, calculateErr
		}
		if !at.Before(today) {
			summary.add(event, attribution, calculated)
		}

		for _, scopeName := range []string{"all", event.Client} {
			scope := scopes[scopeName]
			if scope == nil {
				continue
			}
			date := at.Format("2006-01-02")
			if scope.daily[date] == nil {
				scope.daily[date] = newPresentationAccumulator()
			}
			if err = scope.daily[date].add(event, calculated); err != nil {
				return PresentationReport{}, err
			}
			if !at.Before(today) {
				if scope.hourly[at.Hour()] == nil {
					scope.hourly[at.Hour()] = newPresentationAccumulator()
				}
				if err = scope.hourly[at.Hour()].add(event, calculated); err != nil {
					return PresentationReport{}, err
				}
			}

			if !at.Before(periods[2].start) {
				key := [2]int{(int(at.Weekday()) + 6) % 7, at.Hour()}
				if scope.rhythm[key] == nil {
					scope.rhythm[key] = newPresentationAccumulator()
				}
				if err = scope.rhythm[key].add(event, calculated); err != nil {
					return PresentationReport{}, err
				}
			}

			for _, period := range periods {
				if at.Before(period.start) {
					continue
				}
				if err = addPresentationQuality(scope, period.name, attribution, event, calculated); err != nil {
					return PresentationReport{}, err
				}
				if err = scope.pricing[period.name].add(event, calculated); err != nil {
					return PresentationReport{}, err
				}
				if len(calculated.Unpriced) > 0 {
					identifier := event.Model
					if scopeName == "all" {
						identifier = event.Client + "/" + event.Model
					}
					scope.unpriced[period.name][identifier] = struct{}{}
				}
				periodValue := scope.periods[period.name]
				if err = periodValue.total.add(event, calculated); err != nil {
					return PresentationReport{}, err
				}
				modelKey := event.Client + "\x00" + event.Model
				if periodValue.models[modelKey] == nil {
					periodValue.models[modelKey] = newPresentationAccumulator()
				}
				if err = periodValue.models[modelKey].add(event, calculated); err != nil {
					return PresentationReport{}, err
				}
			}
		}
	}

	report := PresentationReport{Available: true, Scopes: make([]PresentationScope, 0, 3), ClientSubtotals: PresentationClientSubtotals{Available: true, Items: []PresentationClientSubtotal{}}, Summary: summary.finish()}
	for _, scopeName := range []string{"all", "codex", "claude"} {
		report.Scopes = append(report.Scopes, buildPresentationScope(scopeName, scopes[scopeName], periods, start90, today, currentHour))
	}
	for _, period := range periods {
		for _, client := range []string{"codex", "claude"} {
			report.ClientSubtotals.Items = append(report.ClientSubtotals.Items, PresentationClientSubtotal{
				Period: period.name, Client: client, Value: presentationDisplayValue(scopes[client].periods[period.name].total.stats),
			})
		}
	}
	return report, nil
}

func addPresentationQuality(scope *presentationScopeAccumulator, period string, attribution eventAttribution, event storedEvent, result Result) error {
	quality := "inferred"
	if attribution.provider == "unknown" {
		quality = "unattributed"
	} else if attribution.quality == "exact" {
		quality = "determinable"
	}
	byPeriod := scope.quality[period]
	if byPeriod == nil {
		return nil
	}
	for _, providerName := range []string{"", attribution.provider} {
		if byPeriod[providerName] == nil {
			byPeriod[providerName] = map[string]*presentationAccumulator{}
		}
		if byPeriod[providerName][quality] == nil {
			byPeriod[providerName][quality] = newPresentationAccumulator()
		}
		if err := byPeriod[providerName][quality].add(event, result); err != nil {
			return err
		}
	}
	return nil
}

// presentationScopeHasData reports whether this client scope observed anything
// inside the bounded read. The daily map is consulted as well as the period
// records because an event older than 30 days still lands in the 90-day series,
// and a client with such history is not an empty client.
func presentationScopeHasData(value *presentationScopeAccumulator) bool {
	if len(value.daily) > 0 {
		return true
	}
	for _, period := range value.periods {
		if period.total.stats.events > 0 {
			return true
		}
	}
	return false
}

// unavailablePresentationScope keeps the record a client scope is contractually
// required to have while reporting that no family was supplied for it. Emitting
// synthetic zeros instead would present "no data was measured" as "zero was
// measured", which are different claims about the same client. Every collection
// is a non-null empty array for the same reason `emptySessionsSnapshot` is.
func unavailablePresentationScope(name string, currentHour int) PresentationScope {
	return PresentationScope{
		Client:  name,
		Periods: PresentationPeriods{Items: []PresentationPeriod{}},
		Daily:   PresentationDaily{Items: []PresentationDailyItem{}},
		Hourly:  PresentationHourly{ThroughHour: currentHour, Items: []PresentationHourlyItem{}},
		Quality: PresentationQuality{Items: []PresentationQualityItem{}},
		Pricing: PresentationPricing{Items: []PresentationPricingItem{}},
		Rhythm: PresentationRhythm{
			Intensities: []int{}, Tokens: []int64{}, ProviderCosts: []string{}, CostIncomplete: []bool{},
		},
	}
}

func buildPresentationScope(name string, value *presentationScopeAccumulator, periods []presentationPeriodDefinition, start90, today time.Time, currentHour int) PresentationScope {
	// `all` is an explicit scope rather than a missing client, so it keeps its
	// families available and reports a measured zero. A concrete client with no
	// data reports unavailable families instead.
	if name != "all" && !presentationScopeHasData(value) {
		return unavailablePresentationScope(name, currentHour)
	}
	scope := PresentationScope{
		Client:  name,
		Periods: PresentationPeriods{Available: true, Items: []PresentationPeriod{}},
		Daily:   PresentationDaily{Available: true, Items: []PresentationDailyItem{}},
		Hourly:  PresentationHourly{Available: true, ThroughHour: currentHour, Items: []PresentationHourlyItem{}},
		Quality: PresentationQuality{Available: true, Items: []PresentationQualityItem{}},
		Pricing: presentationPricing(value, periods),
		Rhythm:  presentationRhythm(value, periods[2].start),
	}
	for _, period := range periods {
		periodValue := value.periods[period.name]
		scope.Periods.Items = append(scope.Periods.Items, PresentationPeriod{
			Period: period.name, Totals: presentationTotals(periodValue.total.stats),
			AveragePerDay: presentationAverage(periodValue.total.stats, period.days),
			Peak:          presentationPeak(value.daily, period.start, today),
			CacheHitShare: percentPointer(periodValue.total.stats.cachedRead, periodValue.total.logicalInput),
			Models:        presentationModels(name, periodValue),
		})
	}
	for offset := 0; offset < presentationDailyLimit; offset++ {
		date := start90.AddDate(0, 0, offset)
		item := value.daily[date.Format("2006-01-02")]
		if item == nil {
			item = newPresentationAccumulator()
		}
		scope.Daily.Items = append(scope.Daily.Items, PresentationDailyItem{Date: date.Format("2006-01-02"), Value: presentationDisplayValue(item.stats)})
	}
	for hour := 0; hour <= currentHour; hour++ {
		item := value.hourly[hour]
		if item == nil {
			item = newPresentationAccumulator()
		}
		scope.Hourly.Items = append(scope.Hourly.Items, PresentationHourlyItem{Hour: hour, Value: presentationDisplayValue(item.stats)})
	}
	scope.Quality.Items = presentationQuality(value, periods)
	return scope
}

func presentationTotals(value *statsAccumulator) PresentationTotals {
	providerCost, knownProviderCost := statsCost(value)
	knownCatalogBaseCost := money(value.base)
	var catalogBaseCost *string
	if value.complete {
		copy := knownCatalogBaseCost
		catalogBaseCost = &copy
	}
	return PresentationTotals{
		Tokens: value.tokens, InputTokens: value.input, OutputTokens: value.output,
		CachedReadTokens: value.cachedRead, CacheWriteTokens: value.cacheWrite,
		Events: value.events, Sessions: int64(len(value.sessions)),
		CatalogBaseCost: catalogBaseCost, ProviderCost: providerCost,
		KnownCatalogBaseCost: knownCatalogBaseCost, KnownProviderCost: knownProviderCost,
		PricingComplete: value.complete, UnpricedComponents: len(value.missing),
	}
}

func presentationDisplayValue(value *statsAccumulator) PresentationDisplayValue {
	providerCost, knownProviderCost := statsCost(value)
	displayCost := knownProviderCost
	costIncomplete := true
	if providerCost != nil {
		displayCost = *providerCost
		costIncomplete = false
	}
	return PresentationDisplayValue{
		Tokens: value.tokens, Events: value.events,
		ProviderCost: displayCost, CostIncomplete: costIncomplete,
	}
}

func presentationAverage(value *statsAccumulator, days int64) PresentationAverage {
	if days < 1 {
		days = 1
	}
	known := money(new(big.Rat).Quo(new(big.Rat).Set(value.provider), big.NewRat(days, 1)))
	var complete *string
	if value.complete {
		copy := known
		complete = &copy
	}
	return PresentationAverage{
		Tokens:       new(big.Rat).Quo(big.NewRat(value.tokens, 1), big.NewRat(days, 1)).FloatString(2),
		ProviderCost: complete, KnownProviderCost: known,
	}
}

func presentationPeak(daily map[string]*presentationAccumulator, start, today time.Time) PresentationPeak {
	selectedDate := start.Format("2006-01-02")
	selected := newPresentationAccumulator()
	for date := start; !date.After(today); date = date.AddDate(0, 0, 1) {
		value := daily[date.Format("2006-01-02")]
		if value != nil && value.stats.tokens > selected.stats.tokens {
			selectedDate, selected = date.Format("2006-01-02"), value
		}
	}
	return PresentationPeak{Date: selectedDate, Totals: presentationTotals(selected.stats)}
}

func presentationModels(scopeName string, value *presentationPeriodAccumulator) []PresentationModel {
	type modelValue struct {
		client, model string
		value         *presentationAccumulator
	}
	items := make([]modelValue, 0, len(value.models))
	for key, model := range value.models {
		parts := strings.SplitN(key, "\x00", 2)
		items = append(items, modelValue{client: parts[0], model: parts[1], value: model})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].value.stats.tokens != items[j].value.stats.tokens {
			return items[i].value.stats.tokens > items[j].value.stats.tokens
		}
		if items[i].client != items[j].client {
			return items[i].client < items[j].client
		}
		return items[i].model < items[j].model
	})
	if len(items) > presentationModelLimit {
		items = items[:presentationModelLimit]
	}
	out := make([]PresentationModel, 0, len(items))
	for _, item := range items {
		client := ""
		if scopeName == "all" {
			client = item.client
		}
		out = append(out, PresentationModel{Client: client, Model: item.model, Value: presentationDisplayValue(item.value.stats), Share: percentPointer(item.value.stats.tokens, value.total.stats.tokens)})
	}
	return out
}

// presentationQuality emits one record set per supported period, in the periods'
// own order, each set being the client-scope aggregate followed by its
// deterministic per-provider records. A period with no events still emits its
// empty set, so a filtered panel always finds the record it selects.
func presentationQuality(value *presentationScopeAccumulator, periods []presentationPeriodDefinition) []PresentationQualityItem {
	out := make([]PresentationQualityItem, 0, len(periods))
	for _, period := range periods {
		out = append(out, presentationPeriodQuality(period.name, value.quality[period.name])...)
	}
	return out
}

func presentationPeriodQuality(period string, byProvider map[string]map[string]*presentationAccumulator) []PresentationQualityItem {
	providers := make([]string, 0, len(byProvider))
	for providerName := range byProvider {
		providers = append(providers, providerName)
	}
	sort.Slice(providers, func(i, j int) bool {
		if providers[i] == providers[j] {
			return false
		}
		if providers[i] == "" {
			return true
		}
		if providers[j] == "" {
			return false
		}
		return providers[i] < providers[j]
	})
	out := make([]PresentationQualityItem, 0, len(providers))
	for _, providerName := range providers {
		byQuality := byProvider[providerName]
		totalCost := new(big.Rat)
		for _, item := range byQuality {
			totalCost.Add(totalCost, item.stats.provider)
		}
		var providerValue *string
		if providerName != "" {
			copy := providerName
			providerValue = &copy
		}
		result := PresentationQualityItem{Period: period, Provider: providerValue, Tiers: []PresentationQualityTier{}}
		for _, quality := range []string{"determinable", "inferred", "unattributed"} {
			item := byQuality[quality]
			if item == nil {
				item = newPresentationAccumulator()
			}
			result.Tiers = append(result.Tiers, PresentationQualityTier{Quality: quality, Value: presentationDisplayValue(item.stats), Share: percentRat(item.stats.provider, totalCost)})
		}
		out = append(out, result)
	}
	if len(out) == 0 {
		out = append(out, PresentationQualityItem{Period: period, Tiers: []PresentationQualityTier{
			{Quality: "determinable", Value: presentationDisplayValue(newStatsAccumulator())},
			{Quality: "inferred", Value: presentationDisplayValue(newStatsAccumulator())},
			{Quality: "unattributed", Value: presentationDisplayValue(newStatsAccumulator())},
		}})
	}
	return out
}

// presentationPricing emits one record per supported period under the same
// { available, items } family shape the other collections use.
func presentationPricing(value *presentationScopeAccumulator, periods []presentationPeriodDefinition) PresentationPricing {
	items := make([]PresentationPricingItem, 0, len(periods))
	for _, period := range periods {
		unpriced := value.unpriced[period.name]
		identifiers := make([]string, 0, len(unpriced))
		for identifier := range unpriced {
			identifiers = append(identifiers, identifier)
		}
		sort.Strings(identifiers)
		if len(identifiers) > presentationUnpricedLimit {
			identifiers = identifiers[:presentationUnpricedLimit]
		}
		stats := value.pricing[period.name]
		if stats == nil {
			stats = newPresentationAccumulator()
		}
		items = append(items, PresentationPricingItem{
			Period: period.name, PricedEvents: stats.stats.priced, UnpricedEvents: stats.stats.unpriced,
			Coverage: statsCoverage(stats.stats.priced, stats.stats.unpriced), UnpricedIdentifiers: identifiers,
		})
	}
	return PresentationPricing{Available: true, Items: items}
}

func presentationRhythm(value *presentationScopeAccumulator, start time.Time) PresentationRhythm {
	maximum := int64(0)
	weekdayTotals := [7]int64{}
	for key, item := range value.rhythm {
		if item.stats.tokens > maximum {
			maximum = item.stats.tokens
		}
		weekdayTotals[key[0]] += item.stats.tokens
	}
	intensities := make([]int, 0, 7*24)
	tokens := make([]int64, 0, 7*24)
	providerCosts := make([]string, 0, 7*24)
	costIncomplete := make([]bool, 0, 7*24)
	for weekday := 0; weekday < 7; weekday++ {
		for hour := 0; hour < 24; hour++ {
			intensity := 0
			item := value.rhythm[[2]int{weekday, hour}]
			if item == nil {
				item = newPresentationAccumulator()
			}
			if maximum > 0 {
				intensity = int((item.stats.tokens*100 + maximum/2) / maximum)
			}
			display := presentationDisplayValue(item.stats)
			intensities = append(intensities, intensity)
			tokens = append(tokens, display.Tokens)
			providerCosts = append(providerCosts, display.ProviderCost)
			costIncomplete = append(costIncomplete, display.CostIncomplete)
		}
	}
	activeDays := 0
	for date, item := range value.daily {
		if item.stats.tokens > 0 && date >= start.Format("2006-01-02") {
			activeDays++
		}
	}
	busiest, quietest := presentationDayNames(weekdayTotals)
	return PresentationRhythm{
		Available: true, Intensities: intensities, Tokens: tokens,
		ProviderCosts: providerCosts, CostIncomplete: costIncomplete,
		ActiveDays: activeDays, BusiestDay: busiest, QuietestDay: quietest,
	}
}

func presentationDayNames(totals [7]int64) (string, string) {
	if totals == [7]int64{} {
		return "", ""
	}
	names := [...]string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	busiest, quietest := 0, 0
	for index := 1; index < len(totals); index++ {
		if totals[index] > totals[busiest] {
			busiest = index
		}
		if totals[index] < totals[quietest] {
			quietest = index
		}
	}
	return names[busiest], names[quietest]
}

func percentRat(numerator, denominator *big.Rat) *string {
	if denominator.Sign() <= 0 {
		return nil
	}
	value := new(big.Rat).Mul(new(big.Rat).Quo(new(big.Rat).Set(numerator), denominator), big.NewRat(100, 1)).FloatString(2)
	return &value
}

func localDateStart(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

type presentationSummaryBuilder struct {
	summary  Summary
	base     *big.Rat
	provider *big.Rat
	complete bool
	warned   map[string]bool
	unpriced map[string]bool
	coverage map[string]*ModelCoverage
}

func newPresentationSummaryBuilder() *presentationSummaryBuilder {
	return &presentationSummaryBuilder{
		summary: Summary{Tokens: map[string]int64{}, Counts: map[string]int64{"events": 0, "exact": 0, "estimated": 0, "historical": 0, "priced": 0, "unpriced": 0}, Models: []ModelCoverage{}, Unpriced: []string{}, Warnings: []string{}},
		base:    new(big.Rat), provider: new(big.Rat), complete: true,
		warned: map[string]bool{}, unpriced: map[string]bool{}, coverage: map[string]*ModelCoverage{},
	}
}

func (b *presentationSummaryBuilder) add(event storedEvent, attribution eventAttribution, result Result) {
	b.summary.Counts["events"]++
	b.summary.Counts[attribution.quality]++
	if attribution.quality != "exact" && !b.warned[attribution.quality] {
		b.summary.Warnings = append(b.summary.Warnings, attribution.quality+" attribution")
		b.warned[attribution.quality] = true
	}
	for key, value := range event.Tokens {
		b.summary.Tokens[key] += value
	}
	for _, warning := range result.Warnings {
		if !b.warned[warning] {
			b.summary.Warnings = append(b.summary.Warnings, warning)
			b.warned[warning] = true
		}
	}
	coverageKey := event.Client + "\x00" + event.Model
	model := b.coverage[coverageKey]
	if model == nil {
		model = &ModelCoverage{Client: event.Client, Model: event.Model}
		b.coverage[coverageKey] = model
	}
	model.Events++
	knownBase, _ := decimal(result.KnownCatalogBaseCost)
	knownProvider, _ := decimal(result.KnownProviderCost)
	b.base.Add(b.base, knownBase)
	b.provider.Add(b.provider, knownProvider)
	if result.CatalogBaseCost == nil {
		b.complete = false
		b.summary.Counts["unpriced"]++
		model.UnpricedEvents++
		for _, component := range result.Unpriced {
			b.unpriced[component] = true
		}
		return
	}
	b.summary.Counts["priced"]++
	model.PricedEvents++
}

func (b *presentationSummaryBuilder) finish() Summary {
	knownBase, knownProvider := money(b.base), money(b.provider)
	b.summary.KnownCatalogBaseCost = &knownBase
	b.summary.KnownProviderCost = &knownProvider
	if b.complete {
		b.summary.CatalogBaseCost = &knownBase
		b.summary.ProviderCost = &knownProvider
	}
	for value := range b.unpriced {
		b.summary.Unpriced = append(b.summary.Unpriced, value)
	}
	sort.Strings(b.summary.Unpriced)
	for _, model := range b.coverage {
		b.summary.Models = append(b.summary.Models, *model)
	}
	sort.Slice(b.summary.Models, func(i, j int) bool {
		if b.summary.Models[i].Client == b.summary.Models[j].Client {
			return b.summary.Models[i].Model < b.summary.Models[j].Model
		}
		return b.summary.Models[i].Client < b.summary.Models[j].Client
	})
	sort.Strings(b.summary.Warnings)
	return b.summary
}
