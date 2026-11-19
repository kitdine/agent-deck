package usage

import (
	"math"
	"math/big"
	"slices"
	"strings"
	"time"
)

const pricingCurrency = "USD"

type invocationCounts struct {
	input, read, write, output *int64
	shortWrite, longWrite      *int64
	conflict                   bool
}

func pricingCounts(event storedEvent) invocationCounts {
	facts := RequestFacts{}
	if event.Facts != nil {
		facts = *event.Facts
	}
	counts := invocationCounts{}
	fromEvent := func(key string, observed *int64) *int64 {
		count := event.Tokens[key]
		if count < 0 || observed != nil && *observed != count {
			counts.conflict = true
		}
		if observed != nil || event.Facts == nil && count != 0 {
			return &count
		}
		return nil
	}
	counts.input = fromEvent("input_tokens", facts.InputTokens)
	counts.output = fromEvent("output_tokens", facts.OutputTokens)
	if event.Client == "codex" {
		counts.read = fromEvent("cached_input_tokens", facts.CachedReadTokens)
		counts.write = fromEvent("cache_write_tokens", facts.CacheWriteTokens)
		counts.shortWrite, counts.longWrite = facts.CacheWrite30m, facts.CacheWrite12h
	} else {
		counts.read = fromEvent("cache_read_tokens", facts.CachedReadTokens)
		counts.write = fromEvent("cache_creation_tokens", facts.CacheWriteTokens)
		counts.shortWrite = fromEvent("cache_write_5m_tokens", facts.CacheWrite5m)
		counts.longWrite = fromEvent("cache_write_1h_tokens", facts.CacheWrite1h)
	}
	buckets, valid := countSum(valueOf(counts.shortWrite), valueOf(counts.longWrite))
	if counts.write == nil && counts.shortWrite != nil && counts.longWrite != nil && valid {
		counts.write = &buckets
		// Codex buckets can supply a missing total only for the admitted delta.
		if event.Client == "codex" && buckets != event.Tokens["cache_write_tokens"] {
			counts.conflict = true
		}
	}
	if !valid || counts.write != nil && buckets > *counts.write {
		counts.conflict = true
	}
	input, valid := countSum(valueOf(counts.read), max(valueOf(counts.write), buckets))
	if !valid || event.Client == "codex" && counts.input != nil && input > *counts.input {
		counts.conflict = true
	}
	if event.Client == "claude" {
		if _, valid := countSum(valueOf(counts.input), input); !valid {
			counts.conflict = true
		}
	}
	return counts
}

func valueOf(count *int64) int64 {
	if count == nil {
		return 0
	}
	return *count
}

func countSum(a, b int64) (int64, bool) {
	if a < 0 || b < 0 || a > math.MaxInt64-b {
		return 0, false
	}
	return a + b, true
}

func (c invocationCounts) context(provider string) *int64 {
	if provider == "openai" || c.input == nil || c.read == nil || c.write == nil {
		if provider == "openai" {
			return c.input
		}
		return nil
	}
	total := *c.input + *c.read + *c.write // validated in pricingCounts
	return &total
}

type tokenComponent struct {
	kind  string
	count *int64
}

type invocationComponents struct {
	tokens     []tokenComponent
	unassigned int64
	assume5m   bool
}

func (c invocationCounts) components(provider string, writeAsInput, onlyShortWrite bool) invocationComponents {
	input := c.input
	short, long := c.shortWrite, c.longWrite
	shortKind, longKind := "cache_write_5m", "cache_write_1h"
	if provider == "openai" {
		shortKind, longKind = "cache_write_30m", "cache_write_12h"
		input = nil
		if c.input != nil && c.read != nil && (c.write != nil || writeAsInput) {
			ordinary := *c.input - *c.read
			if !writeAsInput {
				ordinary -= *c.write
			}
			input = &ordinary
		}
	}

	out := invocationComponents{tokens: []tokenComponent{{"input", input}, {"cache_read", c.read}, {"output", c.output}}}
	if writeAsInput {
		return out
	}
	out.assume5m = provider == "anthropic" && short == nil && long == nil
	if c.write != nil {
		out.unassigned = *c.write - valueOf(short) - valueOf(long)
		zero := int64(0)
		if onlyShortWrite && valueOf(long) == 0 {
			short, long = c.write, &zero
			out.unassigned = 0
		}
		// A known total proves that the remaining buckets are zero once
		// the counted buckets exhaust it, even if zero was omitted.
		if out.unassigned == 0 {
			if short == nil {
				short = &zero
			}
			if long == nil {
				long = &zero
			}
		}
	}
	return invocationComponents{tokens: append(out.tokens, tokenComponent{shortKind, short}, tokenComponent{longKind, long}),
		unassigned: out.unassigned, assume5m: out.assume5m}
}

type tariffAmount struct {
	amount     PricingAmount
	components []PricingComponent
	reasons    pricingReasons
	reference  *PricingReference
	context    string
}

func (r invocationRules) amount(s tariffSelection, at time.Time, counts invocationCounts) tariffAmount {
	out := tariffAmount{amount: PricingAmount{Currency: pricingCurrency, Status: pricingUnavailable}, context: "unknown"}
	if counts.conflict {
		out.reasons.add(reasonCounts)
		return out
	}
	if s.model == "" {
		return out
	}
	if s.tier == "" {
		out.reasons.add(reasonTier)
		return out
	}
	if !s.singleInvocation {
		out.reasons.add(reasonInvocation)
	}
	writePolicy := r.selectTerm(s, at, tariffField{policy: "cache_write_as_input"})
	writeAsInput := writePolicy.rule != nil
	terms := counts.components(s.provider, writeAsInput, r.onlyShortOpenAIWrite(s, at))
	if writePolicy.reason != "" && (counts.write == nil || *counts.write > 0) {
		// Uncertain cache treatment also makes the ordinary-input quantity
		// uncertain. Cache read and output keep their independent counts.
		out.reasons.add(writePolicy.reason)
		terms.tokens[0].count = nil
	}
	if terms.unassigned != 0 {
		out.reasons.add(reasonTTL)
	}
	standard := s
	standard.tier = "standard"
	speed := r.selectTerm(s, at, tariffField{policy: "speed_multiplier"})
	sum := new(big.Rat)
	rated, missing, unmetered := false, false, false
	for _, token := range terms.tokens {
		if token.count == nil {
			missing = true
			if terms.unassigned == 0 || !strings.HasPrefix(token.kind, "cache_write_") {
				unmetered = true
			}
			continue
		}
		term := r.selectTerm(s, at, tariffField{rate: token.kind})
		var modifier *PricingPolicy
		if term.rule == nil && term.reason == "" && speed.rule != nil {
			term = r.selectTerm(standard, at, tariffField{rate: token.kind})
			modifier = tariffPolicy(speed)
		}
		component := PricingComponent{Kind: token.kind, Quantity: *token.count, Unit: "token"}
		if token.kind == "input" && writeAsInput {
			component.Treatment = tariffPolicy(writePolicy)
		}
		if term.rule == nil {
			if *token.count > 0 {
				reason := term.reason
				if reason == "" {
					reason = speed.reason
				}
				if reason == "" {
					reason = reasonHistorical
				}
				out.reasons.add(reason)
				out.components = append(out.components, component)
				missing = true
			}
			continue
		}
		out.context = mergeContextTier(out.context, term.rule.Selector.Context)
		component, value := priceComponent(component, term, modifier)
		sum.Add(sum, value)
		out.components = append(out.components, component)
		rated = true
	}
	if unmetered {
		out.reasons.add(reasonUnmetered)
	}
	if !rated {
		if len(out.reasons) == 0 {
			out.reasons.add(reasonHistorical)
		}
		return out
	}
	value := exactPrice(sum, 9)
	out.amount = PricingAmount{Currency: pricingCurrency, Status: pricingPartial, KnownAmount: &value}
	if !missing && terms.unassigned == 0 && s.singleInvocation {
		out.amount.Status, out.amount.Amount = pricingComplete, &value
	} else if terms.unassigned > 0 && !unmetered && s.singleInvocation && len(out.reasons) == 1 {
		out.reference = r.ttlReference(s, at, terms, speed, sum)
	}
	return out
}

func (r invocationRules) ttlReference(s tariffSelection, at time.Time, counts invocationComponents, speed tariffTerm, known *big.Rat) *PricingReference {
	var values []*big.Rat
	var candidates []string
	for _, token := range counts.tokens {
		// A measured bucket cannot absorb an unallocated remainder.
		if !strings.HasPrefix(token.kind, "cache_write_") || token.count != nil {
			continue
		}
		term := r.selectTerm(s, at, tariffField{rate: token.kind})
		var modifier *PricingPolicy
		if term.rule == nil && term.reason == "" && speed.rule != nil {
			standard := s
			standard.tier = "standard"
			term = r.selectTerm(standard, at, tariffField{rate: token.kind})
			modifier = tariffPolicy(speed)
		}
		if term.rule == nil {
			if counts.assume5m && len(values) == 1 {
				break
			}
			return nil
		}
		_, extra := priceComponent(PricingComponent{Quantity: counts.unassigned}, term, modifier)
		values = append(values, new(big.Rat).Add(known, extra))
		candidates = append(candidates, strings.TrimPrefix(token.kind, "cache_write_"))
	}
	ref := &PricingReference{Currency: pricingCurrency, Tier: s.tier}
	if counts.assume5m && len(values) > 0 {
		amount := exactPrice(values[0], 9)
		ref.Basis, ref.Amount = "assumed_5m", &amount
		return ref
	}
	if len(values) == 0 {
		return nil
	}
	lower, upper := values[0], values[0]
	for _, value := range values[1:] {
		if value.Cmp(lower) < 0 {
			lower = value
		}
		if value.Cmp(upper) > 0 {
			upper = value
		}
	}
	lo, hi := exactPrice(lower, 9), exactPrice(upper, 9)
	ref.Basis, ref.Lower, ref.Upper, ref.TTLCandidates = "cache_write_ttl_range", &lo, &hi, candidates
	return ref
}

func mergeContextTier(current string, c *PriceRuleContext) string {
	if c != nil {
		if c.Comparison == "gt" || c.Comparison == "gte" {
			return "long"
		}
		return "short"
	}
	if current == "unknown" {
		return "flat"
	}
	return current
}

func priceComponent(component PricingComponent, term tariffTerm, modifier *PricingPolicy) (PricingComponent, *big.Rat) {
	rate, _ := decimal(term.value) // decoded catalog terms have been validated
	if modifier != nil {
		scale, _ := decimal(modifier.Multiplier)
		rate.Mul(rate, scale)
	}
	formattedRate := exactPrice(rate, 0)
	value := new(big.Rat).Mul(rate, big.NewRat(component.Quantity, 1_000_000))
	amount := exactPrice(value, 9)
	component.Currency, component.Rate, component.Amount = term.rule.Currency, &formattedRate, &amount
	component.RuleID, component.SourceURL = term.rule.ID, term.rule.source.URL
	component.EffectiveFrom, component.EffectiveUntil = &term.rule.Validity.From, term.rule.Validity.To
	component.Modifier = modifier
	return component, value
}

func exactPrice(value *big.Rat, minScale int) string {
	precision, _ := value.FloatPrec()
	return value.FloatString(max(minScale, precision))
}

func scaleEstimate(amount PricingAmount, components []PricingComponent, raw string) (PricingAmount, []PricingComponent) {
	scale, err := multiplier(raw)
	if err != nil {
		return PricingAmount{Currency: pricingCurrency, Status: pricingUnavailable}, nil
	}
	scaled := func(value *string, minScale int) *string {
		if value == nil {
			return nil
		}
		parsed, _ := decimal(*value)
		result := exactPrice(parsed.Mul(parsed, scale), minScale)
		return &result
	}
	amount.Amount, amount.KnownAmount = scaled(amount.Amount, 9), scaled(amount.KnownAmount, 9)
	components = slices.Clone(components)
	for i := range components {
		components[i].Rate = scaled(components[i].Rate, 0)
		components[i].Amount = scaled(components[i].Amount, 9)
	}
	return amount, components
}
