package usage

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"time"
)

// PriceTariff reports a date-supported rule and its selectors. It does not
// assert that a request meets those selectors or resolve overlapping rules.
type PriceTariff struct {
	PriceRule
	RuleSet       string          `json:"rule_set"`
	Source        PriceRuleSource `json:"source"`
	Applicability string          `json:"applicability"`
}

func (s *Service) PriceListAt(ctx context.Context, provider, model string, at time.Time) ([]EffectivePrice, error) {
	return s.reportPriceList(ctx, provider, model, at, s.now())
}

func (s *Service) reportPriceList(ctx context.Context, provider, model string, at, cutoff time.Time) ([]EffectivePrice, error) {
	tx, err := s.Store.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	out, err := priceListAt(ctx, tx, provider, model, at)
	if err != nil {
		return nil, err
	}
	supported, err := pricingRulesSupported(ctx, tx)
	if err != nil {
		return nil, err
	}
	rules, err := loadInvocationRules(ctx, tx, cutoff, supported)
	if err != nil {
		return nil, err
	}
	byKey := make(map[ruleModel]int, len(out))
	for index, item := range out {
		byKey[ruleModel{item.Provider, item.Model}] = index
	}
	for key, entries := range rules.models {
		if provider != "" && key.provider != provider || model != "" && key.model != model {
			continue
		}
		var tariffs []PriceTariff
		for _, rule := range entries {
			field := tariffField{}
			if rule.Policy != nil {
				field.policy = rule.Policy.Kind
			}
			matches, reason := rule.temporalMatch(at, field, tariffSelection{channel: rule.Selector.Channel, asOf: true})
			if !matches {
				continue
			}
			status := rule.Validity.Basis
			if reason != "" {
				status = reason
			}
			tariffs = append(tariffs, PriceTariff{PriceRule: rule.PriceRule, RuleSet: rule.setDigest, Source: rule.source, Applicability: status})
		}
		if len(tariffs) == 0 {
			continue
		}
		slices.SortFunc(tariffs, func(a, b PriceTariff) int { return strings.Compare(a.ID+"\x00"+a.RuleSet, b.ID+"\x00"+b.RuleSet) })
		index, exists := byKey[key]
		if !exists {
			index = len(out)
			out = append(out, EffectivePrice{Provider: key.provider, Model: key.model, Unit: "USD / 1M tokens", Prices: map[string]string{}, Provenance: map[string]PriceProvenance{}})
		}
		out[index].Tariffs = tariffs
	}
	slices.SortFunc(out, func(a, b EffectivePrice) int {
		return strings.Compare(a.Provider+"\x00"+a.Model, b.Provider+"\x00"+b.Model)
	})
	return out, tx.Commit()
}
