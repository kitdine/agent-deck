package usage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	PriceRuleEffective    = "effective"
	PriceRuleObserved     = "observed"
	PriceRuleAnnouncement = "announcement"

	PriceRuleUSDPerMillion = "usd_per_million_tokens"
	PriceRuleMultiplier    = "multiplier"
	PriceRulePolicy        = "policy"
	PriceRuleModelID       = "model_id"

	priceRuleDay = "2006-01-02"
)

// PriceRuleSet is the v2 section of the legacy-compatible price catalog.
// Digest is SHA-256 of the normalized section, omitting Digest itself.
type PriceRuleSet struct {
	SchemaVersion int         `json:"schema_version"`
	Digest        string      `json:"sha256,omitempty"`
	Rules         []PriceRule `json:"rules"`
}

type PriceRule struct {
	ID         string            `json:"id"`
	Selector   PriceRuleSelector `json:"selector"`
	Validity   PriceRuleValidity `json:"validity"`
	Sources    []PriceRuleSource `json:"sources"`
	Currency   string            `json:"currency"`
	Unit       string            `json:"unit"`
	Rates      map[string]string `json:"rates,omitempty"`
	AliasTo    string            `json:"alias_to,omitempty"`
	Policy     *PriceRuleEffect  `json:"policy,omitempty"`
	Supersedes []string          `json:"supersedes,omitempty"`
}

// Selector values are exact observed or proven facts. An omitted optional
// dimension carries no restriction. "*" is limited to explicit all-model
// policies. In particular, an omitted activity never implies Guardian.
type PriceRuleSelector struct {
	Provider    string            `json:"provider"`
	Channel     string            `json:"channel"`
	Models      []string          `json:"models"`
	Endpoints   []string          `json:"endpoints"`
	ServiceTier string            `json:"service_tier,omitempty"`
	Regions     []string          `json:"regions,omitempty"`
	Context     *PriceRuleContext `json:"context,omitempty"`
	Activity    string            `json:"activity,omitempty"`
	// Eligibility names an independently proven condition, which can be a
	// conjunction such as signed-in, included usage and Ultrafast-eligible.
	Eligibility string `json:"eligibility,omitempty"`
}

// Context bounds name the vendor's token count axis. The upper short-context
// threshold is inclusive when Comparison is lte; gt excludes that threshold.
type PriceRuleContext struct {
	Axis       string `json:"axis"`
	Comparison string `json:"comparison"`
	Tokens     int64  `json:"tokens"`
	Mode       string `json:"mode"`
}

type PriceRuleValidity struct {
	Basis       string             `json:"basis"`
	From        PriceRuleBoundary  `json:"from"`
	To          *PriceRuleBoundary `json:"to,omitempty"`
	InitialRate bool               `json:"initial_rate,omitempty"`
}

// An instant retains its stated offset. A minute records the stated minute
// without claiming a second within it. A calendar_day begins at local midnight.
// A date leaves the cutover somewhere during the stated day, whose timezone may
// be unknown.
type PriceRuleBoundary struct {
	Value     string `json:"value"`
	Precision string `json:"precision"`
	TimeZone  string `json:"timezone,omitempty"`
}

type PriceRuleSource struct {
	URL        string `json:"url"`
	Revision   string `json:"revision"`
	VerifiedOn string `json:"verified_on"`
	Kind       string `json:"kind"`
}

// free_activity declares zero additional USD and zero quota usage.
// no_context_surcharge declares a multiplier of one for the context surcharge
// only. cache_write_as_input bills writes as input without a separate charge.
// speed_multiplier scales the documented token rates or included usage for
// the rule's channel relative to Standard.
type PriceRuleEffect struct {
	ID         string `json:"policy_id"`
	Kind       string `json:"kind"`
	Multiplier string `json:"multiplier,omitempty"`
}

// Covers checks the [from,to) interval warranted for observed usage. Dated
// introductions use the earliest possible start for the first published rate
// in that domain. A rate change uses the latest possible start.
// An observed interval only supports reference pricing. An announcement
// interval only establishes public confirmation and still needs independent
// eligibility evidence for the account or activity.
func (v PriceRuleValidity) Covers(at time.Time) bool {
	start, err := v.supportedStart()
	if err != nil || at.Before(start) {
		return false
	}
	if v.To == nil {
		return true
	}
	end, _, err := v.To.bounds()
	return err == nil && at.Before(end)
}

func (v PriceRuleValidity) supportedStart() (time.Time, error) {
	earliest, latest, err := v.From.bounds()
	if v.InitialRate {
		return earliest, err
	}
	return latest, err
}

func (b PriceRuleBoundary) bounds() (earliest, latest time.Time, err error) {
	if b.TimeZone != "" && b.TimeZone != "UTC" && !strings.Contains(b.TimeZone, "/") {
		return time.Time{}, time.Time{}, fmt.Errorf("timezone %q must be UTC or an IANA area/location", b.TimeZone)
	}
	switch b.Precision {
	case "instant", "minute":
		at, parseErr := time.Parse(time.RFC3339Nano, b.Value)
		if parseErr != nil || b.Precision == "minute" && (at.Second() != 0 || at.Nanosecond() != 0) {
			break
		}
		if b.TimeZone != "" {
			zone, zoneErr := time.LoadLocation(b.TimeZone)
			if zoneErr != nil {
				return time.Time{}, time.Time{}, zoneErr
			}
			_, stated := at.Zone()
			_, actual := at.In(zone).Zone()
			if actual != stated {
				break
			}
		}
		if b.Precision == "minute" {
			return at, at.Add(time.Minute), nil
		}
		return at, at, nil
	case "date", "calendar_day":
		if b.TimeZone == "" {
			if b.Precision != "date" {
				break
			}
			day, parseErr := time.Parse(priceRuleDay, b.Value)
			if parseErr != nil {
				break
			}
			// Without the publisher's zone, allow local offsets UTC+14 to
			// UTC-12. Competing rates cannot claim this uncertainty window.
			return day.Add(-14 * time.Hour), day.Add(36 * time.Hour), nil
		}
		zone, zoneErr := time.LoadLocation(b.TimeZone)
		if zoneErr != nil {
			return time.Time{}, time.Time{}, zoneErr
		}
		day, parseErr := time.ParseInLocation(priceRuleDay, b.Value, zone)
		if parseErr != nil || day.Format(priceRuleDay) != b.Value {
			break
		}
		if b.Precision == "calendar_day" {
			return day, day, nil
		}
		return day, day.AddDate(0, 0, 1), nil
	}
	return time.Time{}, time.Time{}, fmt.Errorf("invalid %q boundary %q (timezone %q)", b.Precision, b.Value, b.TimeZone)
}

// DecodePriceRules validates and normalizes a complete v2 rules document.
// A source file may leave the digest unstamped. If supplied, it must match
// the semantic contents, which are returned in canonical order.
func DecodePriceRules(data []byte) (PriceRuleSet, error) {
	return decodePriceRules(data, false)
}

type packagedPriceRules struct {
	set *PriceRuleSet
}

func (p *packagedPriceRules) UnmarshalJSON(data []byte) error {
	if p.set != nil {
		return errors.New("duplicate catalog pricing_rules field")
	}
	rules, err := decodePriceRules(data, true)
	if err != nil {
		return fmt.Errorf("catalog pricing_rules: %w", err)
	}
	p.set = &rules
	return nil
}

func decodePriceRules(data []byte, requireDigest bool) (PriceRuleSet, error) {
	var rules PriceRuleSet
	if err := checkPriceRuleJSON(data); err != nil {
		return rules, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rules); err != nil {
		return rules, err
	}
	if requireDigest && rules.Digest == "" {
		return rules, errors.New("missing rule-set SHA-256 digest")
	}
	if rules.SchemaVersion != 2 || len(rules.Rules) == 0 {
		return rules, errors.New("expected a non-empty v2 price rule set")
	}
	sort.Slice(rules.Rules, func(i, j int) bool { return rules.Rules[i].ID < rules.Rules[j].ID })
	byID := make(map[string]PriceRule, len(rules.Rules))
	for i := range rules.Rules {
		rule := &rules.Rules[i]
		if _, exists := byID[rule.ID]; exists {
			return rules, fmt.Errorf("duplicate price rule ID %q", rule.ID)
		}
		if err := rule.normalize(); err != nil {
			return rules, fmt.Errorf("price rule %q: %w", rule.ID, err)
		}
		byID[rule.ID] = *rule
	}
	if err := validatePriceRuleConflicts(rules.Rules, byID); err != nil {
		return rules, err
	}
	declared := rules.Digest
	rules.Digest = ""
	canonical, err := json.Marshal(rules)
	if err != nil {
		return rules, err
	}
	rules.Digest = hash(canonical)
	if declared != "" && declared != rules.Digest {
		return rules, fmt.Errorf("price rules SHA-256 %q does not match %q", declared, rules.Digest)
	}
	return rules, nil
}

var (
	priceRuleID      = regexp.MustCompile(`^[a-z0-9]+([._:/-][a-z0-9]+)*$`)
	priceRuleDecimal = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)
)

func normalizePriceRuleDecimal(value string) (string, error) {
	if !priceRuleDecimal.MatchString(value) {
		return "", fmt.Errorf("invalid non-negative decimal %q", value)
	}
	if strings.Contains(value, ".") {
		value = strings.TrimSuffix(strings.TrimRight(value, "0"), ".")
	}
	return value, nil
}

func (r *PriceRule) normalize() error {
	if !priceRuleID.MatchString(r.ID) || r.Currency != "USD" {
		return errors.New("expected a stable ID and USD currency")
	}
	if err := r.Selector.normalize(); err != nil {
		return err
	}
	if err := r.Validity.validate(); err != nil {
		return err
	}
	if r.Validity.InitialRate && len(r.Rates) == 0 {
		return errors.New("initial_rate requires token rates")
	}
	if r.Validity.Basis == PriceRuleAnnouncement && r.Policy == nil {
		return errors.New("announcement validity records a policy, not a price or alias")
	}
	if err := normalizePriceRuleSources(r.Sources, r.Selector.Provider, r.Validity); err != nil {
		return err
	}
	if err := normalizePriceRuleIDs(r.Supersedes); err != nil {
		return err
	}

	if r.AliasTo != "" {
		if r.Unit != PriceRuleModelID || r.Policy != nil || len(r.Rates) != 0 ||
			!priceRuleID.MatchString(r.AliasTo) || len(r.Selector.Models) != 1 ||
			r.Selector.Models[0] == "*" || r.Selector.Models[0] == r.AliasTo {
			return errors.New("alias requires one exact source model and a distinct target model")
		}
		return nil
	}
	if r.Policy != nil {
		if len(r.Rates) != 0 {
			return errors.New("a policy cannot also set token rates")
		}
		return r.normalizePolicy()
	}
	if r.Unit != PriceRuleUSDPerMillion || len(r.Rates) == 0 || r.Selector.Models[0] == "*" ||
		r.Selector.ServiceTier == "" || len(r.Selector.Regions) == 0 ||
		(r.Selector.Channel != "public_api" && r.Selector.Channel != "enterprise_usd") {
		return errors.New("token rates require an exact model, monetary channel, service tier and region")
	}
	components := []string{"input", "cache_read", "cache_write_5m", "cache_write_30m", "cache_write_1h", "cache_write_12h", "output"}
	for name, amount := range r.Rates {
		if !containsPriceRuleID(components, name) {
			return fmt.Errorf("unknown rate component %q", name)
		}
		if (name == "cache_write_5m" || name == "cache_write_1h") && r.Selector.Provider != "anthropic" ||
			(name == "cache_write_30m" || name == "cache_write_12h") && r.Selector.Provider != "openai" {
			return fmt.Errorf("unsupported cache TTL component %q for %s", name, r.Selector.Provider)
		}
		normalized, err := normalizePriceRuleDecimal(amount)
		if err != nil {
			return fmt.Errorf("rate %s: %w", name, err)
		}
		r.Rates[name] = normalized
	}
	return nil
}

func (s *PriceRuleSelector) normalize() error {
	if len(s.Models) == 0 || len(s.Endpoints) == 0 {
		return errors.New("selector requires models and endpoints")
	}
	if len(s.Models) != 1 || s.Models[0] != "*" {
		if err := normalizePriceRuleIDs(s.Models); err != nil {
			return fmt.Errorf("models: %w", err)
		}
	}
	if err := normalizePriceRuleIDs(s.Endpoints); err != nil {
		return fmt.Errorf("endpoints: %w", err)
	}
	if err := normalizePriceRuleIDs(s.Regions); err != nil {
		return fmt.Errorf("regions: %w", err)
	}
	var endpoints []string
	switch {
	case s.Channel == "public_api" && s.Provider == "openai":
		endpoints = []string{"v1/responses", "v1/chat/completions", "v1/batch", "v1/agents"}
	case s.Channel == "public_api" && s.Provider == "anthropic":
		endpoints = []string{"v1/messages", "v1/messages/batches"}
	case (s.Channel == "chatgpt_subscription" || s.Channel == "enterprise_usd") && s.Provider == "openai":
		if s.Eligibility == "" {
			return errors.New("ChatGPT billing selector requires explicit eligibility")
		}
		endpoints = []string{"codex", "chatgpt_work"}
	default:
		return fmt.Errorf("unsupported provider/channel %q/%q", s.Provider, s.Channel)
	}
	for _, endpoint := range s.Endpoints {
		if !containsPriceRuleID(endpoints, endpoint) {
			return fmt.Errorf("endpoint %q is outside %s/%s", endpoint, s.Provider, s.Channel)
		}
	}
	if s.ServiceTier != "" && !containsPriceRuleID([]string{"standard", "fast", "priority", "ultrafast", "batch", "flex", "background_2h", "background_8h"}, s.ServiceTier) {
		return fmt.Errorf("unknown service tier %q", s.ServiceTier)
	}
	if s.Eligibility != "" && !priceRuleID.MatchString(s.Eligibility) {
		return fmt.Errorf("invalid eligibility %q", s.Eligibility)
	}
	if s.Activity != "" && !containsPriceRuleID([]string{"approve_for_me_guardian", "github_pr_review", "local_review", "codex_security"}, s.Activity) {
		return fmt.Errorf("unknown activity selector %q", s.Activity)
	}
	if c := s.Context; c != nil {
		if (c.Axis != "openai_input_tokens" || s.Provider != "openai") &&
			(c.Axis != "anthropic_input_with_cache" || s.Provider != "anthropic") {
			return fmt.Errorf("unsupported context count axis %q", c.Axis)
		}
		if !containsPriceRuleID([]string{"lt", "lte", "gt", "gte"}, c.Comparison) ||
			!containsPriceRuleID([]string{"whole_request", "segmented"}, c.Mode) || c.Tokens <= 0 ||
			(c.Comparison == "gt" && c.Tokens == math.MaxInt64) {
			return errors.New("context requires a positive threshold, comparison and application mode")
		}
	}
	return nil
}

func (v PriceRuleValidity) validate() error {
	switch v.Basis {
	case PriceRuleEffective, PriceRuleObserved:
	case PriceRuleAnnouncement:
		if v.From.Precision != "instant" && v.From.Precision != "minute" {
			return errors.New("announcement requires a timestamped confirmation")
		}
	default:
		return fmt.Errorf("unknown validity basis %q", v.Basis)
	}
	if v.InitialRate && (v.Basis != PriceRuleEffective || v.From.Precision != "date") {
		return errors.New("initial_rate requires a dated effective boundary")
	}
	start, err := v.supportedStart()
	if err != nil {
		return err
	}
	if v.To != nil {
		end, _, err := v.To.bounds()
		if err != nil {
			return err
		}
		if !start.Before(end) {
			return errors.New("validity has no supported interval")
		}
	}
	return nil
}

func normalizePriceRuleSources(sources []PriceRuleSource, provider string, validity PriceRuleValidity) error {
	if len(sources) == 0 {
		return errors.New("missing sources")
	}
	supported := false
	_, start, _ := validity.From.bounds()
	for _, source := range sources {
		parsed, err := url.Parse(source.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil ||
			strings.TrimSpace(source.Revision) != source.Revision || source.Revision == "" {
			return errors.New("source requires an HTTPS URL and a fixed revision")
		}
		verified, err := time.Parse(priceRuleDay, source.VerifiedOn)
		if err != nil {
			return fmt.Errorf("source %q requires a verification date: %w", source.URL, err)
		}
		host := parsed.Hostname()
		vendor := provider == "openai" && (vendorHostMatches(host, "openai.com") || vendorHostMatches(host, "chatgpt.com")) ||
			provider == "anthropic" && (vendorHostMatches(host, "anthropic.com") || vendorHostMatches(host, "claude.com"))
		switch source.Kind {
		case "vendor_dated", "official_override":
			if !vendor {
				return fmt.Errorf("source %q is outside the vendor's domains", source.URL)
			}
			supported = supported || validity.Basis == PriceRuleEffective
		case "pinned_catalog":
			if !validSHA256(source.Revision) && !(validLiteLLMCommit(source.Revision) && strings.Contains(parsed.Path, "/"+source.Revision+"/")) {
				return fmt.Errorf("pinned catalog %q requires a content digest or a commit in the URL", source.URL)
			}
			supported = supported || validity.Basis == PriceRuleEffective
		case "vendor_announcement":
			if !vendor && !(provider == "openai" && host == "x.com" && strings.HasPrefix(parsed.Path, "/thsottiaux/status/")) {
				return fmt.Errorf("source %q is outside the vendor's announcement channels", source.URL)
			}
			supported = supported || validity.Basis == PriceRuleAnnouncement
		case "vendor_current":
			if !vendor {
				return fmt.Errorf("source %q is outside the vendor's domains", source.URL)
			}
			supported = supported || validity.Basis == PriceRuleObserved && !start.Before(verified)
		case "announcement_archive":
		default:
			return fmt.Errorf("unknown source kind %q", source.Kind)
		}
	}
	if !supported {
		return fmt.Errorf("no source supports %s validity", validity.Basis)
	}
	sort.Slice(sources, func(i, j int) bool {
		a, b := sources[i], sources[j]
		if a.URL != b.URL {
			return a.URL < b.URL
		}
		if a.Revision != b.Revision {
			return a.Revision < b.Revision
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.VerifiedOn < b.VerifiedOn
	})
	for i := 1; i < len(sources); i++ {
		if sources[i] == sources[i-1] {
			return fmt.Errorf("duplicate source %q", sources[i].URL)
		}
	}
	return nil
}

func (r *PriceRule) normalizePolicy() error {
	p, s := r.Policy, r.Selector
	if !priceRuleID.MatchString(p.ID) {
		return errors.New("policy requires a stable policy_id")
	}
	if p.Kind == "speed_multiplier" {
		if r.Unit != PriceRuleMultiplier || s.ServiceTier == "" || s.ServiceTier == "standard" || s.Models[0] == "*" ||
			(s.Channel == "public_api" && len(s.Regions) == 0) {
			return errors.New("speed multiplier requires an exact model, non-Standard tier and API region when applicable")
		}
		value, err := normalizePriceRuleDecimal(p.Multiplier)
		if err != nil || value == "0" {
			return fmt.Errorf("invalid speed multiplier %q", p.Multiplier)
		}
		p.Multiplier = value
		return nil
	}
	if r.Unit != PriceRulePolicy || p.Multiplier != "" {
		return errors.New("policy must use policy unit without a numeric rate")
	}
	switch p.Kind {
	case "free_activity":
		if !((s.Channel == "chatgpt_subscription" && s.Eligibility == "chatgpt_sign_in") ||
			(s.Channel == "enterprise_usd" && s.Eligibility == "enterprise_usd_codex_chatgpt_sign_in")) ||
			s.Activity != "approve_for_me_guardian" || len(s.Endpoints) != 1 || s.Endpoints[0] != "codex" {
			return errors.New("free Guardian policy requires ChatGPT sign-in and typed Codex approval activity")
		}
	case "no_context_surcharge":
		if s.Channel != "enterprise_usd" &&
			(s.Channel != "chatgpt_subscription" || s.Eligibility != "chatgpt_sign_in_included_usage" || s.Models[0] == "*") {
			return errors.New("context surcharge relief requires an eligible ChatGPT channel")
		}
	case "cache_write_as_input":
		if s.Channel != "enterprise_usd" {
			return errors.New("cache write treatment requires Enterprise USD")
		}
	default:
		return fmt.Errorf("unknown policy kind %q", p.Kind)
	}
	return nil
}

func normalizePriceRuleIDs(ids []string) error {
	sort.Strings(ids)
	for i, id := range ids {
		if !priceRuleID.MatchString(id) || i > 0 && id == ids[i-1] {
			return fmt.Errorf("invalid or duplicate selector/ID %q", id)
		}
	}
	return nil
}

func containsPriceRuleID(ids []string, value string) bool {
	for _, id := range ids {
		if id == value {
			return true
		}
	}
	return false
}

// Reject repeated JSON properties before decoding so an override cannot hide
// part of its selector, rates or provenance behind last-key-wins decoding.
func checkPriceRuleJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := checkPriceRuleJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("unexpected data after price rule document")
	}
	return nil
}

func checkPriceRuleJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		seen := make(map[string]struct{})
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name := key.(string)
			for _, ch := range name {
				if ch > 0x7f {
					return fmt.Errorf("price rule field %q must be ASCII", name)
				}
			}
			// The decoder matches ASCII struct properties case-insensitively.
			folded := strings.ToLower(name)
			if _, exists := seen[folded]; exists {
				return fmt.Errorf("duplicate price rule field %q", key)
			}
			seen[folded] = struct{}{}
			if err := checkPriceRuleJSONValue(decoder); err != nil {
				return err
			}
		}
	case json.Delim('['):
		for decoder.More() {
			if err := checkPriceRuleJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return nil
	}
	_, err = decoder.Token()
	return err
}

func validatePriceRuleConflicts(rules []PriceRule, byID map[string]PriceRule) error {
	for i, rule := range rules {
		for _, id := range rule.Supersedes {
			target, exists := byID[id]
			if !exists || id == rule.ID || !rule.conflictsWith(target) {
				return fmt.Errorf("price rule %q supersedes %q without a conflicting target in this set", rule.ID, id)
			}
		}
		for _, other := range rules[:i] {
			if !rule.hasDifferentTerms(other) {
				continue
			}
			if rule.Validity.initialRateConflictsWith(other.Validity) || other.Validity.initialRateConflictsWith(rule.Validity) {
				return fmt.Errorf("price rules %q and %q disagree with an initial_rate declaration", other.ID, rule.ID)
			}
			if !rule.Validity.overlaps(other.Validity) {
				continue
			}
			if hasPriceRulePredecessor(rule, other.ID, byID) == hasPriceRulePredecessor(other, rule.ID, byID) {
				return fmt.Errorf("conflicting price rules %q and %q require an unambiguous supersedes relationship", other.ID, rule.ID)
			}
		}
	}
	return nil
}

func hasPriceRulePredecessor(rule PriceRule, id string, byID map[string]PriceRule) bool {
	pending := append([]string(nil), rule.Supersedes...)
	seen := make(map[string]bool)
	for len(pending) > 0 {
		next := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if next == id {
			return true
		}
		if !seen[next] {
			seen[next] = true
			pending = append(pending, byID[next].Supersedes...)
		}
	}
	return false
}

func (r PriceRule) conflictsWith(other PriceRule) bool {
	return r.Validity.overlaps(other.Validity) && r.hasDifferentTerms(other)
}

func (r PriceRule) hasDifferentTerms(other PriceRule) bool {
	if !r.Selector.overlaps(other.Selector) {
		return false
	}
	if r.AliasTo != "" && other.AliasTo != "" {
		return r.AliasTo != other.AliasTo
	}
	if r.Policy != nil && other.Policy != nil {
		return r.Policy.Kind == other.Policy.Kind && r.Policy.Multiplier != other.Policy.Multiplier
	}
	for component, amount := range r.Rates {
		if otherAmount, ok := other.Rates[component]; ok && (otherAmount != amount ||
			r.Selector.Context != nil && other.Selector.Context != nil && r.Selector.Context.Mode != other.Selector.Context.Mode) {
			return true
		}
	}
	return false
}

func (s PriceRuleSelector) overlaps(other PriceRuleSelector) bool {
	return s.Provider == other.Provider && s.Channel == other.Channel &&
		overlappingPriceRuleValues(s.Models, other.Models) &&
		overlappingPriceRuleValues(s.Endpoints, other.Endpoints) &&
		overlappingPriceRuleValues(s.Regions, other.Regions) &&
		compatiblePriceRuleValue(s.ServiceTier, other.ServiceTier) &&
		compatiblePriceRuleValue(s.Activity, other.Activity) &&
		s.Context.overlaps(other.Context)
}

func overlappingPriceRuleValues(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 || left[0] == "*" || right[0] == "*" {
		return true
	}
	for _, value := range left {
		if containsPriceRuleID(right, value) {
			return true
		}
	}
	return false
}

func compatiblePriceRuleValue(left, right string) bool {
	return left == "" || right == "" || left == right
}

func (c *PriceRuleContext) overlaps(other *PriceRuleContext) bool {
	if c == nil || other == nil || c.Axis != other.Axis {
		return true
	}
	low, high := c.limits()
	otherLow, otherHigh := other.limits()
	return (high == nil || otherLow <= *high) && (otherHigh == nil || low <= *otherHigh)
}

func (c PriceRuleContext) limits() (int64, *int64) {
	threshold := c.Tokens
	switch c.Comparison {
	case "lt":
		threshold--
		return 0, &threshold
	case "lte":
		return 0, &threshold
	case "gt":
		return threshold + 1, nil
	default: // gte
		return threshold, nil
	}
}

func (v PriceRuleValidity) overlaps(other PriceRuleValidity) bool {
	if v.Basis != other.Basis {
		return false
	}
	// A shared boundary refers to one cutover, even when its instant is unknown.
	if v.To != nil && *v.To == other.From || other.To != nil && *other.To == v.From {
		return false
	}
	start, _, _ := v.From.bounds()
	otherStart, _, _ := other.From.bounds()
	return priceRuleStartsBeforeEnd(start, other.To) && priceRuleStartsBeforeEnd(otherStart, v.To)
}

func (v PriceRuleValidity) initialRateConflictsWith(other PriceRuleValidity) bool {
	if !v.InitialRate || v.Basis != other.Basis {
		return false
	}
	start, _, _ := v.From.bounds()
	otherStart, _, _ := other.From.bounds()
	return !otherStart.After(start)
}

func priceRuleStartsBeforeEnd(start time.Time, end *PriceRuleBoundary) bool {
	if end == nil {
		return true
	}
	_, latest, _ := end.bounds()
	return start.Before(latest)
}
