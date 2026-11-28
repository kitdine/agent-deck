package usage

import (
	"slices"
	"strings"
	"time"
)

type ruleModel struct{ provider, model string }

type invocationRules struct {
	cutoff time.Time
	digest string
	models map[ruleModel][]*invocationRule
}

type invocationRule struct {
	PriceRule
	start, earliest, end, latestEnd time.Time
	source                          PriceRuleSource
	sourceRank                      int
	setDigest                       string
	predecessors                    map[string]bool
}

func priceSourceRank(kind, basis string) int {
	switch basis {
	case PriceRuleEffective:
		switch kind {
		case "vendor_dated":
			return 4
		case "official_override":
			return 3
		case "pinned_catalog":
			return 1
		}
	case PriceRuleAnnouncement:
		if kind == "vendor_announcement" {
			return 4
		}
	case PriceRuleObserved:
		if kind == "vendor_current" {
			return 2
		}
	}
	return 0
}

func (r *invocationRules) add(set PriceRuleSet) {
	byID := make(map[string]PriceRule, len(set.Rules))
	for _, rule := range set.Rules {
		byID[rule.ID] = rule
	}
	for _, rule := range set.Rules {
		entry := &invocationRule{PriceRule: rule, setDigest: set.Digest}
		for _, source := range rule.Sources {
			rank := priceSourceRank(source.Kind, rule.Validity.Basis)
			if source.VerifiedOn <= r.cutoff.Format(priceRuleDay) && rank > entry.sourceRank {
				entry.source, entry.sourceRank = source, rank
			}
		}
		if entry.sourceRank == 0 {
			continue
		}
		entry.start, _ = rule.Validity.supportedStart()
		entry.earliest, _, _ = rule.Validity.From.bounds()
		if rule.Validity.To != nil {
			entry.end, entry.latestEnd, _ = rule.Validity.To.bounds()
		}
		if len(rule.Supersedes) > 0 {
			entry.predecessors = make(map[string]bool)
			for id := range byID {
				if hasPriceRulePredecessor(rule, id, byID) {
					entry.predecessors[id] = true
				}
			}
		}
		for _, model := range rule.Selector.Models {
			key := ruleModel{rule.Selector.Provider, model}
			r.models[key] = append(r.models[key], entry)
		}
	}
}

type tariffSelection struct {
	provider, channel, model, endpoint, tier, region, activity string
	eligibility                                                []string
	context                                                    *int64
	singleInvocation                                           bool
	asOf                                                       bool
	allowAlias                                                 bool
}

func (s tariffSelection) matching(selector PriceRuleSelector) (bool, string) {
	if s.channel != selector.Channel {
		return false, ""
	}
	reason := ""
	if s.endpoint == "" {
		reason = reasonSelector
	} else if !slices.Contains(selector.Endpoints, s.endpoint) {
		return false, ""
	}
	if len(selector.Regions) != 0 {
		if s.region == "" {
			reason = reasonSelector
		} else if !slices.Contains(selector.Regions, s.region) {
			return false, ""
		}
	}
	if selector.ServiceTier != "" && pricingTier(selector.ServiceTier) != s.tier {
		if s.tier != "" {
			return false, ""
		}
		reason = reasonTier
	}
	if selector.Activity != "" && selector.Activity != s.activity {
		if s.activity != "" {
			return false, ""
		}
		reason = reasonActivity
	}
	if selector.Eligibility != "" && !slices.Contains(s.eligibility, selector.Eligibility) {
		reason = reasonSelector
	}
	if c := selector.Context; c != nil {
		switch {
		case !s.singleInvocation:
			reason = reasonInvocation
		case s.context == nil, c.Mode == "segmented":
			reason = reasonContext
		case !contextContains(c, *s.context):
			return false, ""
		}
	}
	return true, reason
}

func contextContains(c *PriceRuleContext, count int64) bool {
	switch c.Comparison {
	case "lt":
		return count < c.Tokens
	case "lte":
		return count <= c.Tokens
	case "gt":
		return count > c.Tokens
	default: // gte, validated during import
		return count >= c.Tokens
	}
}

func pricingTier(tier string) string {
	switch tier {
	case "default":
		return "standard"
	case "priority":
		return "fast"
	case "standard", "fast", "ultrafast", "batch", "flex", "background_2h", "background_8h":
		return tier
	}
	return ""
}

func (r invocationRules) candidates(s tariffSelection) []*invocationRule {
	exact := r.models[ruleModel{s.provider, s.model}]
	wildcard := r.models[ruleModel{s.provider, "*"}]
	if len(wildcard) == 0 || s.model == "*" {
		return exact
	}
	return slices.Concat(exact, wildcard)
}

type tariffTerm struct {
	value  string
	rule   *invocationRule
	reason string
}

type tariffField struct {
	rate, policy string
	alias        bool
}

func (field tariffField) value(r *invocationRule) string {
	switch {
	case field.alias:
		return r.AliasTo
	case field.rate != "":
		return r.Rates[field.rate]
	case r.Policy != nil && r.Policy.Kind == field.policy:
		if field.policy == "speed_multiplier" {
			return r.Policy.Multiplier
		}
		return r.Policy.ID
	}
	return ""
}

func (r *invocationRule) temporalMatch(at time.Time, field tariffField, selection tariffSelection) (bool, string) {
	// An observed statement supplies a dated comparison. The approved current
	// Enterprise USD card can also support a verified channel estimate.
	if r.Validity.Basis == PriceRuleObserved && selection.channel != "enterprise_usd" && !selection.asOf {
		return false, ""
	}
	if r.Validity.Basis == PriceRuleAnnouncement && field.policy == "" {
		return false, ""
	}
	if at.Before(r.earliest) || (!r.latestEnd.IsZero() && !at.Before(r.latestEnd)) {
		return false, ""
	}
	if at.Before(r.start) || (!r.end.IsZero() && !at.Before(r.end)) {
		return true, reasonBoundary
	}
	return true, ""
}

func (r invocationRules) selectTerm(s tariffSelection, at time.Time, field tariffField) tariffTerm {
	var exact, possible []tariffTerm
	for _, rule := range r.candidates(s) {
		value := field.value(rule)
		if value == "" {
			continue
		}
		matches, reason := s.matching(rule.Selector)
		if !matches {
			continue
		}
		inTime, dateReason := rule.temporalMatch(at, field, s)
		if !inTime {
			continue
		}
		if reason == "" {
			reason = dateReason
		}
		term := tariffTerm{value: value, rule: rule, reason: reason}
		if reason == "" {
			exact = append(exact, term)
		} else {
			possible = append(possible, term)
		}
	}
	var winner tariffTerm
	for _, term := range exact {
		if slices.ContainsFunc(exact, func(other tariffTerm) bool {
			return term.rule != other.rule && compareTermPrecedence(other.rule, term.rule) > 0
		}) {
			continue
		}
		if winner.rule != nil && (winner.value != term.value || !sameContextMode(winner.rule, term.rule)) {
			return tariffTerm{reason: reasonConflict}
		}
		if winner.rule == nil || term.rule.ID < winner.rule.ID || term.rule.ID == winner.rule.ID && term.rule.setDigest < winner.rule.setDigest {
			winner = term
		}
	}
	if winner.rule == nil && len(exact) > 0 {
		return tariffTerm{reason: reasonConflict}
	}
	for _, term := range possible {
		if winner.rule == nil || (term.value != winner.value || !sameContextMode(term.rule, winner.rule)) &&
			compareTermPrecedence(winner.rule, term.rule) <= 0 {
			return tariffTerm{reason: term.reason}
		}
	}
	return winner
}

func sameContextMode(left, right *invocationRule) bool {
	l, r := left.Selector.Context, right.Selector.Context
	return l == nil || r == nil || l.Mode == r.Mode
}

func compareTermPrecedence(left, right *invocationRule) int {
	diff := boolRank(left.predecessors[right.ID]) - boolRank(right.predecessors[left.ID])
	if diff != 0 {
		return diff
	}
	if diff := left.sourceRank - right.sourceRank; diff != 0 {
		return diff
	}
	// A precise official interval can supersede a less precise official
	// account of the same selector. Import order never settles a conflict.
	if left.sourceRank >= 2 {
		return validityPrecision(left.Validity) - validityPrecision(right.Validity)
	}
	return 0
}

func boolRank(value bool) int {
	if value {
		return 1
	}
	return 0
}

func validityPrecision(v PriceRuleValidity) int {
	precision := func(b PriceRuleBoundary) int {
		switch b.Precision {
		case "instant", "calendar_day":
			return 3
		case "minute":
			return 2
		case "date":
			return boolRank(b.TimeZone != "")
		}
		return 0
	}
	result := precision(v.From)
	if v.To != nil {
		result = min(result, precision(*v.To))
	}
	return result
}

func (r invocationRules) resolveModel(s tariffSelection, at time.Time, model string) (string, string) {
	seen := make(map[string]bool)
	for {
		if seen[model] {
			return "", reasonConflict
		}
		seen[model] = true
		s.model = model
		rows := r.models[ruleModel{s.provider, model}]
		if slices.ContainsFunc(rows, func(rule *invocationRule) bool { return len(rule.Rates) > 0 && s.channel == rule.Selector.Channel }) {
			return model, ""
		}
		if !s.allowAlias {
			return "", reasonModel
		}
		alias := r.selectTerm(s, at, tariffField{alias: true})
		if alias.rule == nil {
			if alias.reason != "" {
				return "", alias.reason
			}
			return "", reasonModel
		}
		model = alias.value
	}
}

func (r invocationRules) onlyShortOpenAIWrite(s tariffSelection, at time.Time) bool {
	// An initial 12h rate establishes when that bucket first became possible
	// in its selector domain. A missing catalog entry alone proves nothing.
	s.tier = "standard"
	for _, rule := range r.candidates(s) {
		matches, reason := s.matching(rule.Selector)
		if matches && reason == "" && rule.Validity.InitialRate && rule.Rates["cache_write_12h"] != "" && at.Before(rule.earliest) {
			return true
		}
	}
	return false
}

func tariffPolicy(term tariffTerm) *PricingPolicy {
	if term.rule == nil || term.rule.Policy == nil {
		return nil
	}
	return &PricingPolicy{ID: term.rule.Policy.ID, RuleID: term.rule.ID, SourceURL: term.rule.source.URL, Multiplier: term.rule.Policy.Multiplier}
}

func selectorActivity(activity string) string {
	switch activity {
	case "guardian_approval":
		return "approve_for_me_guardian"
	case "github_review":
		return "github_pr_review"
	case "local_review", "codex_security", "ordinary":
		return activity
	}
	return ""
}

func autoReviewModel(model string) bool {
	switch strings.ToLower(model) {
	case "auto-code-review", "codex-auto-review", "slim":
		return true
	}
	return false
}
