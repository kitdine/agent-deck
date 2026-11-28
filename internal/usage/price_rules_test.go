package usage

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fixturePriceRule(id, input string) PriceRule {
	return PriceRule{
		ID: id,
		Selector: PriceRuleSelector{
			Provider: "openai", Channel: "public_api", Models: []string{"gpt-example"},
			Endpoints:   []string{"v1/responses", "v1/chat/completions"},
			ServiceTier: "standard", Regions: []string{"global"},
		},
		Validity: PriceRuleValidity{
			Basis: PriceRuleEffective,
			From:  PriceRuleBoundary{Value: "2026-10-01T00:00:00Z", Precision: "instant"},
		},
		Sources: []PriceRuleSource{{
			URL: "https://developers.openai.com/api/docs/pricing", Revision: "test-price-card",
			VerifiedOn: "2026-11-18", Kind: "vendor_dated",
		}},
		Currency: "USD", Unit: PriceRuleUSDPerMillion,
		Rates: map[string]string{"input": input},
	}
}

func marshalPriceRules(t *testing.T, rules ...PriceRule) []byte {
	t.Helper()
	return marshalPriceRuleValue(t, PriceRuleSet{SchemaVersion: 2, Rules: rules})
}

func marshalPriceRuleValue(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPriceRuleNormalizationAndBundledVersion(t *testing.T) {
	a := fixturePriceRule("a", "1.2500")
	b := fixturePriceRule("b", "1.25")
	b.Selector.Models = []string{"gpt-example-b"}
	first := marshalPriceRules(t, b, a)
	a.Rates["input"] = "1.25"
	a.Selector.Endpoints[0], a.Selector.Endpoints[1] = a.Selector.Endpoints[1], a.Selector.Endpoints[0]
	second := marshalPriceRules(t, a, b)
	generate := func(data []byte) ([]byte, catalog) {
		t.Helper()
		raw, err := GenerateBundledCatalogWithRules(syntheticLiteLLM(), genTestCommit, Gapfill{}, data)
		if err != nil {
			t.Fatal(err)
		}
		c, err := parseCatalog(raw)
		if err != nil {
			t.Fatal(err)
		}
		return raw, c
	}
	firstJSON, firstCatalog := generate(first)
	secondJSON, secondCatalog := generate(second)
	if !bytes.Equal(firstJSON, secondJSON) || firstCatalog.PricingRules.Digest != secondCatalog.PricingRules.Digest {
		t.Fatal("equivalent v2 rules generated different bytes or digests")
	}
	if firstCatalog.PricingRules.Rules[0].ID != "a" || firstCatalog.PricingRules.Rules[0].Rates["input"] != "1.25" ||
		!validSHA256(firstCatalog.PricingRules.Digest) {
		t.Fatalf("bad normalized rules: %+v", firstCatalog.PricingRules)
	}
	a.Rates["input"] = "1.26"
	_, changed := generate(marshalPriceRules(t, a, b))
	if changed.Version == firstCatalog.Version || changed.PricingRules.Digest == firstCatalog.PricingRules.Digest {
		t.Fatal("a changed rule left the rule digest or catalog version unchanged")
	}

	v1Data, err := GenerateBundledCatalog(syntheticLiteLLM(), genTestCommit, Gapfill{})
	if err != nil {
		t.Fatal(err)
	}
	v1Catalog, err := parseCatalog(v1Data)
	if err != nil {
		t.Fatal(err)
	}
	// Keep this reader ignorant of pricing_rules, as existing v1 readers are.
	var v1Reader struct {
		SchemaVersion int                   `json:"schema_version"`
		Models        map[string]modelPrice `json:"models"`
	}
	if err := json.Unmarshal(firstJSON, &v1Reader); err != nil {
		t.Fatal(err)
	}
	if v1Catalog.PricingRules != nil || v1Reader.SchemaVersion != 1 || !reflect.DeepEqual(v1Reader.Models, v1Catalog.Models) {
		t.Fatal("v2 packaging changed the legacy model envelope")
	}

	tampered := bytes.Replace(firstJSON, []byte(`"input": "1.25"`), []byte(`"input": "1.26"`), 1)
	if _, err := parseCatalog(tampered); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("catalog accepted a tampered rule with its old rule digest: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(firstJSON, &document); err != nil {
		t.Fatal(err)
	}
	duplicate := string(bytes.TrimSuffix(bytes.TrimSpace(firstJSON), []byte("}"))) +
		`,"Pricing_Rules":` + string(document["pricing_rules"]) + "}"
	if _, err := parseCatalog([]byte(duplicate)); err == nil || !strings.Contains(err.Error(), "duplicate catalog pricing_rules") {
		t.Fatalf("catalog accepted duplicate pricing_rules fields: %v", err)
	}
	document["pricing_rules"] = json.RawMessage(first)
	if _, err := parseCatalog(marshalPriceRuleValue(t, document)); err == nil || !strings.Contains(err.Error(), "missing rule-set SHA-256") {
		t.Fatalf("catalog accepted a rule set without its digest: %v", err)
	}
}

func TestDecodePriceRulesRejectsInvalidSelectorsAndEvidence(t *testing.T) {
	for _, tt := range []struct {
		name string
		edit func(*PriceRule)
		want string
	}{
		{"missing provenance", func(r *PriceRule) { r.Sources = nil }, "missing sources"},
		{"undated override", func(r *PriceRule) { r.Sources[0].Kind = "vendor_current" }, "no source supports effective"},
		{"backdated observation", func(r *PriceRule) {
			r.Validity.Basis = PriceRuleObserved
			r.Sources[0].Kind = "vendor_current"
		}, "no source supports observed"},
		{"aggregator as vendor", func(r *PriceRule) { r.Sources[0].URL = "https://models.dev/api.json" }, "vendor's domains"},
		{"mutable catalog pin", func(r *PriceRule) { r.Sources[0].Kind = "pinned_catalog" }, "requires a content digest or a commit"},
		{"invalid source verification date", func(r *PriceRule) { r.Sources[0].VerifiedOn = "2026-02-30" }, "verification date"},
		{"model pattern", func(r *PriceRule) { r.Selector.Models = []string{"gpt-6*"} }, "invalid or duplicate"},
		{"wrong endpoint", func(r *PriceRule) { r.Selector.Endpoints = []string{"codex"} }, "outside openai/public_api"},
		{"tier preference", func(r *PriceRule) { r.Selector.ServiceTier = "requested_fast" }, "unknown service tier"},
		{"unknown activity", func(r *PriceRule) { r.Selector.Activity = "codex-auto-review" }, "unknown activity selector"},
		{"unknown market", func(r *PriceRule) { r.Selector.Channel = "all" }, "unsupported provider/channel"},
		{"wrong TTL", func(r *PriceRule) { r.Rates["cache_write_5m"] = "1.25" }, "unsupported cache TTL"},
		{"unspecified TTL", func(r *PriceRule) { r.Rates["cache_write"] = "1.25" }, "unknown rate component"},
		{"negative amount", func(r *PriceRule) { r.Rates["input"] = "-1" }, "invalid non-negative decimal"},
		{"local timezone", func(r *PriceRule) { r.Validity.From.TimeZone = "Local" }, "must be UTC or an IANA"},
		{"wrong offset", func(r *PriceRule) { r.Validity.From.TimeZone = "America/Los_Angeles" }, "invalid"},
		{"initial instant", func(r *PriceRule) { r.Validity.InitialRate = true }, "initial_rate requires a dated effective boundary"},
		{"announcement price", func(r *PriceRule) { r.Validity.Basis = PriceRuleAnnouncement }, "records a policy"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rule := fixturePriceRule("example", "1")
			tt.edit(&rule)
			if _, err := DecodePriceRules(marshalPriceRules(t, rule)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q rejection, got %v", tt.want, err)
			}
		})
	}
	valid := marshalPriceRules(t, fixturePriceRule("a", "1"))
	for _, tt := range []struct{ name, from, to, want string }{
		{"duplicate field", `"channel":"public_api"`, `"channel":"enterprise_usd","Channel":"public_api"`, "duplicate price rule field"},
		{"long s field alias", `"service_tier":"standard"`, `"service_tier":"batch","ſervice_tier":"standard"`, "must be ASCII"},
		{"kelvin sign field alias", `"kind":"vendor_dated"`, `"Kind":"vendor_dated"`, "must be ASCII"},
		{"unknown field", `"channel":"public_api"`, `"channel":"public_api","chanel":"public_api"`, "unknown field"},
		{"unquoted amount", `"input":"1"`, `"input":1`, "cannot unmarshal number"},
		{"unsupported version", `"schema_version":2`, `"schema_version":3`, "non-empty v2 price rule set"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := bytes.Replace(valid, []byte(tt.from), []byte(tt.to), 1)
			if _, err := DecodePriceRules(raw); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q rejection, got %v", tt.want, err)
			}
		})
	}
}

func TestDecodePriceRulesValidatesOverlapsPerComponent(t *testing.T) {
	a := fixturePriceRule("a", "1")
	b := fixturePriceRule("b", "2")
	b.Selector.Endpoints = []string{"v1/responses"}
	reject := func(want string, rules ...PriceRule) {
		t.Helper()
		if _, err := DecodePriceRules(marshalPriceRules(t, rules...)); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("want %q rejection, got %v", want, err)
		}
	}
	accept := func(rules ...PriceRule) {
		t.Helper()
		if _, err := DecodePriceRules(marshalPriceRules(t, rules...)); err != nil {
			t.Fatalf("rules should coexist: %v", err)
		}
	}
	reject("conflicting price rules", a, b)
	reject("duplicate price rule ID", a, a)
	differentComponent := b
	differentComponent.Rates = map[string]string{"cache_write_30m": "2"}
	accept(a, differentComponent)
	differentRegion := b
	differentRegion.Selector.Regions = []string{"us"}
	accept(a, differentRegion)
	disjointTime := b
	end := a.Validity.From
	disjointTime.Validity.From.Value = "2026-09-01T00:00:00Z"
	disjointTime.Validity.To = &end
	accept(a, disjointTime)

	short, long := a, b
	short.Selector.Context = &PriceRuleContext{Axis: "openai_input_tokens", Comparison: "lte", Tokens: 272000, Mode: "whole_request"}
	long.Selector.Context = &PriceRuleContext{Axis: "openai_input_tokens", Comparison: "gt", Tokens: 272000, Mode: "whole_request"}
	accept(short, long)
	long.Selector.Context.Comparison = "gte"
	reject("conflicting price rules", short, long)

	b.Supersedes = []string{"a"}
	accept(a, b)
	a.Supersedes = []string{"b"}
	reject("unambiguous supersedes", a, b)
	a.Supersedes = []string{"missing"}
	reject("without a conflicting target", a, b)
}

func TestBundledPriceRulesPreserveHistoricalCoverage(t *testing.T) {
	source, err := os.ReadFile("price-rules-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := DecodePriceRules(source)
	if err != nil {
		t.Fatal(err)
	}
	bundled, err := parseCatalog(bundledCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if bundled.PricingRules == nil || bundled.PricingRules.Digest != rules.Digest {
		t.Fatal("embedded tariff rules do not match their canonical source")
	}
	byID := make(map[string]PriceRule, len(rules.Rules))
	for _, rule := range rules.Rules {
		byID[rule.ID] = rule
	}
	at := func(raw string) time.Time {
		t.Helper()
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	for _, tt := range []struct {
		id, time, component, rate string
		covered                   bool
	}{
		{"gpt-5.6-luna.api.standard.short.launch", "2026-07-29T09:59:59Z", "input", "1", true},
		{"gpt-5.6-luna.api.standard.short.launch", "2026-07-29T10:00:00Z", "input", "1", false},
		{"gpt-5.6-luna.api.standard.short.reduced", "2026-07-31T11:59:59Z", "input", "0.2", false},
		{"gpt-5.6-luna.api.standard.short.reduced", "2026-07-31T12:00:00Z", "input", "0.2", true},
		{"gpt-5.6-luna.api.standard.long.launch", "2026-07-28T12:00:00Z", "output", "9", true},
		{"gpt-5.6-luna.api.standard.long.reduced", "2026-08-02T00:00:00Z", "output", "1.8", true},
		{"gpt-5.6-terra.api.standard.short.launch", "2026-07-25T00:00:00Z", "output", "15", true},
		{"gpt-5.6-terra.api.standard.short.reduced", "2026-08-02T00:00:00Z", "output", "12", true},
		{"gpt-5.6-terra.api.standard.long.launch", "2026-07-25T00:00:00Z", "input", "5", true},
		{"gpt-5.6-terra.api.standard.long.reduced", "2026-08-02T00:00:00Z", "input", "4", true},
		{"gpt-5.6-sol.api.standard.short.launch", "2026-08-19T00:00:00Z", "output", "30", true},
		{"gpt-5.6-sol.api.standard.short.promo", "2026-08-24T00:00:00Z", "output", "20", true},
		{"gpt-5.6-sol.api.standard.short.promo", "2026-12-01T00:00:00Z", "output", "20", false},
		{"gpt-5.6-sol.api.standard.long.launch", "2026-08-19T00:00:00Z", "output", "45", true},
		{"gpt-5.6-sol.api.standard.long.promo", "2026-08-24T00:00:00Z", "output", "30", true},
		{"gpt-5.6-sol.api.standard.long.cache-write-12h", "2026-11-18T00:00:00Z", "cache_write_12h", "20", true},
		{"gpt-6.2-astra.api.standard.global.short.cache-write-12h", "2026-11-18T12:00:00Z", "cache_write_12h", "20", true},
		{"gpt-6.2-astra.api.standard.global.long.cache-write-12h", "2026-11-18T12:00:00Z", "cache_write_12h", "40", true},
		{"gpt-6.1-sol.api.standard.long", "2026-11-18T12:00:00Z", "output", "15", true},
		{"gpt-6.1-sol.api.standard.long.cache-write-12h", "2026-10-19T09:59:59Z", "cache_write_12h", "10", false},
		{"gpt-6.1-sol.api.standard.long.cache-write-12h", "2026-10-21T12:00:00Z", "cache_write_12h", "10", true},
		{"claude-opus-4-6.api.standard.long.beta", "2026-03-11T00:00:00Z", "output", "37.5", true},
		{"claude-opus-4-6.api.standard.long.beta", "2026-03-14T12:00:00Z", "output", "37.5", false},
		{"claude-opus-4-6.api.standard.long.ga", "2026-03-14T12:00:00Z", "output", "25", true},
		{"claude-sonnet-4-6.api.standard.long.beta", "2026-03-11T00:00:00Z", "cache_write_1h", "12", true},
		{"claude-sonnet-4-6.api.standard.short", "2026-03-13T00:00:00Z", "output", "15", true},
		{"claude-sonnet-4-6.api.standard.long.ga", "2026-03-14T12:00:00Z", "cache_write_1h", "6", true},
		{"claude-sonnet-5-5.api.standard.cache-read.launch", "2026-10-05T00:00:00Z", "cache_read", "0.2", true},
		{"claude-sonnet-5-5.api.standard.cache-read.reduced", "2026-10-09T00:00:00Z", "cache_read", "0.1", true},
		{"claude-sonnet-5-5.api.standard.unchanged", "2026-10-07T00:00:00Z", "input", "2", true},
	} {
		rule, ok := byID[tt.id]
		if !ok || rule.Validity.Covers(at(tt.time)) != tt.covered || rule.Rates[tt.component] != tt.rate {
			t.Errorf("%s at %s: want covered=%v, %s=%s, got %+v", tt.id, tt.time, tt.covered, tt.component, tt.rate, rule)
		}
	}

	for _, tt := range []struct {
		short, long, axis     string
		threshold             int64
		shortRates, longRates map[string]string
	}{
		{"gpt-6.2-astra.api.standard.global.short", "gpt-6.2-astra.api.standard.global.long", "openai_input_tokens", 272000,
			map[string]string{"input": "8", "cache_read": "0.4", "cache_write_30m": "10", "output": "40"},
			map[string]string{"input": "16", "cache_read": "0.8", "cache_write_30m": "20", "output": "60"}},
		{"claude-haiku-5-5.api.standard.short", "claude-haiku-5-5.api.standard.long", "anthropic_input_with_cache", 100000,
			map[string]string{"input": "0.1", "cache_read": "0.01", "cache_write_5m": "0.125", "cache_write_1h": "0.2", "output": "0.5"},
			map[string]string{"input": "0.5", "cache_read": "0.05", "cache_write_5m": "0.625", "cache_write_1h": "1", "output": "2.5"}},
	} {
		for _, variant := range []struct {
			id, comparison string
			rates          map[string]string
		}{{tt.short, "lte", tt.shortRates}, {tt.long, "gt", tt.longRates}} {
			rule := byID[variant.id]
			want := PriceRuleContext{Axis: tt.axis, Comparison: variant.comparison, Tokens: tt.threshold, Mode: "whole_request"}
			if rule.Selector.Context == nil || *rule.Selector.Context != want || !reflect.DeepEqual(rule.Rates, variant.rates) {
				t.Errorf("%s: want context %+v and rates %+v, got %+v", variant.id, want, variant.rates, rule)
			}
		}
	}
	for _, tt := range []struct {
		id, channel, eligibility, activity, basis, kind, multiplier, value string
	}{
		{"codex.subscription.guardian-free", "chatgpt_subscription", "chatgpt_sign_in", "approve_for_me_guardian", "announcement", "free_activity", "", "2026-10-06T07:13:00Z"},
		{"codex.enterprise-usd.guardian-free", "enterprise_usd", "enterprise_usd_codex_chatgpt_sign_in", "approve_for_me_guardian", "announcement", "free_activity", "", "2026-10-06T07:13:00Z"},
		{"gpt-6.1-sol.subscription.no-context-surcharge", "chatgpt_subscription", "chatgpt_sign_in_included_usage", "", "announcement", "no_context_surcharge", "", "2026-10-11T23:48:00Z"},
		{"gpt-6.2.subscription.ultrafast.included", "chatgpt_subscription", "chatgpt_sign_in_included_usage_ultrafast_eligible", "", "effective", "speed_multiplier", "6", "2026-11-17"},
		{"gpt-6.2.api.ultrafast.promo", "public_api", "actual_ultrafast", "", "effective", "speed_multiplier", "3", "2026-11-17"},
		{"gpt-6.2.api.ultrafast.list", "public_api", "actual_ultrafast", "", "effective", "speed_multiplier", "6", "2027-03-01"},
		{"codex.enterprise-usd.cache-write-as-input", "enterprise_usd", "enterprise_usd_codex_chatgpt_sign_in", "", "observed", "cache_write_as_input", "", "2026-11-18"},
		{"codex.enterprise-usd.no-context-surcharge", "enterprise_usd", "enterprise_usd_codex_chatgpt_sign_in", "", "observed", "no_context_surcharge", "", "2026-11-18"},
	} {
		rule := byID[tt.id]
		s := rule.Selector
		if s.Channel != tt.channel || s.Eligibility != tt.eligibility || s.Activity != tt.activity ||
			rule.Validity.Basis != tt.basis || rule.Validity.From.Value != tt.value ||
			rule.Policy == nil || rule.Policy.ID == "" || rule.Policy.Kind != tt.kind || rule.Policy.Multiplier != tt.multiplier {
			t.Fatalf("%s: unexpected policy or applicability %+v", tt.id, rule)
		}
		if tt.basis == PriceRuleAnnouncement {
			confirmed := at(tt.value).Add(time.Minute)
			if rule.Validity.From.Precision != "minute" ||
				rule.Validity.Covers(confirmed.Add(-time.Nanosecond)) || !rule.Validity.Covers(confirmed) {
				t.Errorf("%s: announcement instant was inferred from the displayed minute", tt.id)
			}
		}
	}
	for _, r := range rules.Rules {
		if r.Policy != nil && r.Policy.Kind == "free_activity" && r.Selector.Activity != "approve_for_me_guardian" {
			t.Errorf("%s also makes unknown or untyped activity free", r.ID)
		}
	}
	enterprise := byID["gpt-6.2-astra.enterprise-usd.codex.standard"]
	if enterprise.Selector.Context != nil || !reflect.DeepEqual(enterprise.Selector.Endpoints, []string{"codex"}) ||
		enterprise.Validity.Covers(at("2026-10-08T00:00:00Z")) ||
		!reflect.DeepEqual(enterprise.Rates, map[string]string{"input": "8", "cache_read": "0.4", "output": "40"}) {
		t.Fatalf("Enterprise USD rate escaped the observed Codex scope: %+v", enterprise)
	}
	promo := byID["gpt-6.2.api.ultrafast.promo"]
	list := byID["gpt-6.2.api.ultrafast.list"]
	cutover := at("2027-03-01T08:00:00Z")
	if promo.Validity.To == nil || promo.Validity.To.TimeZone != "America/Los_Angeles" ||
		!promo.Validity.Covers(cutover.Add(-time.Nanosecond)) || promo.Validity.Covers(cutover) ||
		list.Validity.Covers(cutover.Add(-time.Nanosecond)) || !list.Validity.Covers(cutover) ||
		!reflect.DeepEqual(promo.Selector.Regions, []string{"global", "us"}) {
		t.Fatalf("Ultrafast API promotion or list rates have an incorrect regional window: %+v, %+v", promo, list)
	}
}

func TestPriceRulesRejectOverlappingUncertainDates(t *testing.T) {
	previous, replacement := fixturePriceRule("previous", "1"), fixturePriceRule("replacement", "2")
	previous.Validity.From = PriceRuleBoundary{Value: "2026-07-09", Precision: "date"}
	cutover := PriceRuleBoundary{Value: "2026-07-30", Precision: "date"}
	previous.Validity.To = &cutover
	replacement.Validity.From = PriceRuleBoundary{Value: "2026-07-29", Precision: "date"}
	if _, err := DecodePriceRules(marshalPriceRules(t, previous, replacement)); err == nil || !strings.Contains(err.Error(), "conflicting price rules") {
		t.Fatalf("expected potential overlap rejection, got %v", err)
	}

	replacement.Validity.From = cutover
	if _, err := DecodePriceRules(marshalPriceRules(t, previous, replacement)); err != nil {
		t.Fatalf("shared cutover should be valid: %v", err)
	}
	duringCutover := time.Date(2026, time.July, 30, 20, 0, 0, 0, time.UTC)
	if previous.Validity.Covers(duringCutover) || replacement.Validity.Covers(duringCutover) {
		t.Fatal("neither competing price should cover an uncertain cutover")
	}

	replacement.Validity.InitialRate = true
	if _, err := DecodePriceRules(marshalPriceRules(t, previous, replacement)); err == nil || !strings.Contains(err.Error(), "initial_rate declaration") {
		t.Fatalf("expected rejection of initial_rate after prior conflicting terms, got %v", err)
	}
}

func TestBundledPriceRulesCoverInitialRateDay(t *testing.T) {
	catalog, err := parseCatalog(bundledCatalog)
	if err != nil || catalog.PricingRules == nil {
		t.Fatalf("load bundled price rules: %v", err)
	}
	rules := make(map[string]PriceRule)
	for _, rule := range catalog.PricingRules.Rules {
		rules[rule.ID] = rule
	}

	for _, tt := range []struct{ id, stamp string }{
		{"gpt-6.2-astra.api.standard.global.short", "2026-11-17T20:00:00Z"},
		{"claude-haiku-5-5.api.standard.short", "2026-10-07T20:00:00Z"},
		{"gpt-6.1-sol.api.standard.short", "2026-09-29T20:00:00Z"},
		{"gpt-5.6-luna.api.standard.short.cache-write-12h", "2026-10-20T20:00:00Z"},
	} {
		t.Run(tt.id, func(t *testing.T) {
			at, err := time.Parse(time.RFC3339, tt.stamp)
			if err != nil {
				t.Fatal(err)
			}
			validity := rules[tt.id].Validity
			if !validity.InitialRate || !validity.Covers(at) || validity.Covers(at.AddDate(0, 0, -2)) {
				t.Fatalf("unexpected coverage of initial rate: %+v", validity)
			}
		})
	}
}
