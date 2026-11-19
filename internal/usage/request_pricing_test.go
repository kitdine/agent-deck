package usage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func pricingTime(raw string) time.Time {
	at, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		panic(err)
	}
	return at
}

func pricingInt(n int64) *int64 { return &n }

func fixtureInvocationRules(t *testing.T, now string) invocationRules {
	t.Helper()
	catalog, err := parseCatalog(bundledCatalog)
	if err != nil {
		t.Fatal(err)
	}
	rules := invocationRules{cutoff: pricingTime(now), models: map[ruleModel][]*invocationRule{}, digest: catalog.PricingRules.Digest}
	rules.add(*catalog.PricingRules)
	return rules
}

func codexPricingEvent(model, at string, input, read, write, output int64) storedEvent {
	return storedEvent{Event: Event{Key: "codex:event", Client: "codex", Model: model, EventAt: at, SessionID: "session",
		Tokens: map[string]int64{"input_tokens": input, "cached_input_tokens": read, "cache_write_tokens": write, "output_tokens": output},
		Facts: &RequestFacts{
			Source: "codex_test", ObservedAt: at, Association: "fixture_invocation",
			InputTokens: &input, CachedReadTokens: &read, CacheWriteTokens: &write, OutputTokens: &output,
			ExecutedTier: "standard",
			Pricing:      &RequestPricingEvidence{APIEndpoint: "v1/responses", Region: "global", Channel: "api_key", Activity: "ordinary"},
		}}}
}

func claudePricingEvent(model, at string, input, read, write, short, long, output int64) storedEvent {
	return storedEvent{Event: Event{Key: "claude:event", Client: "claude", Model: model, EventAt: at, SessionID: "session",
		Tokens: map[string]int64{"input_tokens": input, "cache_read_tokens": read, "cache_creation_tokens": write,
			"cache_write_5m_tokens": short, "cache_write_1h_tokens": long, "output_tokens": output},
		Facts: &RequestFacts{
			Source: "claude_jsonl", ObservedAt: at, Association: "assistant_message_usage",
			InputTokens: &input, CachedReadTokens: &read, CacheWriteTokens: &write, OutputTokens: &output,
			CacheWrite5m: &short, CacheWrite1h: &long, ExecutedTier: "standard",
			Pricing: &RequestPricingEvidence{Region: "global", Channel: "api_key", Activity: "ordinary"},
		}}}
}

func priceFixture(r invocationRules, event storedEvent) InvocationPricing {
	return r.evaluate(event, pricingTime(event.EventAt), eventAttribution{}, Result{})
}

func checkPriceAmount(t *testing.T, result PricingAmount, status, amount, known string) {
	t.Helper()
	if result.Status != status || valueString(result.Amount) != amount || valueString(result.KnownAmount) != known ||
		result.Currency != "USD" {
		t.Fatalf("amount = %s %s amount=%q known=%q, want %s %q %q", result.Currency, result.Status,
			valueString(result.Amount), valueString(result.KnownAmount), status, amount, known)
	}
}

func valueString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func checkPricingReason(t *testing.T, got InvocationPricing, reason string) {
	t.Helper()
	if !slices.Contains(got.Reasons, reason) {
		t.Fatalf("reasons %v do not contain %q", got.Reasons, reason)
	}
}

func TestRequestPricingApprovedArithmetic(t *testing.T) {
	rules := fixtureInvocationRules(t, "2026-11-25T00:00:00Z")
	const at = "2026-11-21T00:00:00Z"
	for _, tc := range []struct {
		name       string
		event      storedEvent
		want, tier string
	}{
		{"Astra short", codexPricingEvent("gpt-6.2-astra", at, 272_000, 100_000, 20_000, 10_000), "1.856000000", "short"},
		{"Astra long", codexPricingEvent("gpt-6.2-astra", at, 272_001, 100_000, 20_000, 10_000), "3.512016000", "long"},
		{"Haiku short", claudePricingEvent("claude-haiku-5-5", at, 49_000, 40_000, 11_000, 10_000, 1_000, 5_000), "0.009250000", "short"},
		{"Haiku long", claudePricingEvent("claude-haiku-5-5", at, 49_001, 40_000, 11_000, 10_000, 1_000, 5_000), "0.046250500", "long"},
		{"Sol long", codexPricingEvent("gpt-6.1-sol", at, 300_000, 0, 0, 10_000), "1.350000000", "long"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.event.Client == "codex" && *tc.event.Facts.CacheWriteTokens > 0 {
				tc.event.Facts.CacheWrite30m = tc.event.Facts.CacheWriteTokens
			}
			got := priceFixture(rules, tc.event)
			checkPriceAmount(t, got.APIEquivalent, pricingComplete, tc.want, tc.want)
			if got.ProviderEstimate == nil {
				t.Fatal("verified API Key has no estimate")
			}
			checkPriceAmount(t, *got.ProviderEstimate, pricingComplete, tc.want, tc.want)
			if got.ExecutedTier != "standard" || got.ContextTier != tc.tier || len(got.Reasons) != 0 {
				t.Fatalf("tiers/reasons = %s / %s / %v", got.ExecutedTier, got.ContextTier, got.Reasons)
			}
			for _, part := range got.Components {
				if part.RuleID == "" || part.SourceURL == "" || part.EffectiveFrom == nil || part.Rate == nil || part.Amount == nil {
					t.Fatalf("component omitted evidence: %+v", part)
				}
			}
		})
	}
}

func TestRequestPricingDerivesMatchingCodexCacheWriteTotal(t *testing.T) {
	rules := fixtureInvocationRules(t, "2026-11-25T00:00:00Z")
	event := codexPricingEvent("gpt-6.2-astra", "2026-11-21T00:00:00Z", 272_000, 100_000, 20_000, 10_000)
	noLongWrite := int64(0)
	event.Facts.CacheWrite30m = event.Facts.CacheWriteTokens
	event.Facts.CacheWrite12h = &noLongWrite
	event.Facts.CacheWriteTokens = nil

	got := priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "1.856000000", "1.856000000")
	if len(got.Reasons) != 0 {
		t.Fatalf("matching cache-write counts produced reasons %v", got.Reasons)
	}
}

func TestRequestPricingPolicyDomains(t *testing.T) {
	rules := fixtureInvocationRules(t, "2027-03-04T00:00:00Z")
	const now = "2026-11-21T00:00:00Z"
	enterprise := codexPricingEvent("gpt-6.2-astra", now, 272_001, 100_000, 20_000, 10_000)
	enterprise.Facts.CacheWrite30m = enterprise.Facts.CacheWriteTokens
	enterprise.Facts.Pricing.Channel, enterprise.Facts.Pricing.ChannelEndpoint = "enterprise_usd", "codex"
	enterprise.Facts.Pricing.Eligibility = []string{"enterprise_usd_codex_chatgpt_sign_in"}
	got := priceFixture(rules, enterprise)
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "3.512016000", "3.512016000")
	if got.ProviderEstimate == nil || got.Subscription.SubscriptionLongContext != "exempt" || got.Subscription.QuotaReason != "paid_channel" {
		t.Fatalf("enterprise policy = %+v, provider=%+v", got.Subscription, got.ProviderEstimate)
	}
	checkPriceAmount(t, *got.ProviderEstimate, pricingComplete, "1.816008000", "1.816008000")
	input := got.ProviderComponents[0]
	if input.Kind != "input" || input.Quantity != 172_001 || input.Treatment == nil ||
		input.Treatment.ID != "openai.codex.enterprise-usd.cache-write-as-input" {
		t.Fatalf("Enterprise write treatment = %+v", input)
	}
	enterprise.Facts.Pricing.Eligibility = []string{"chatgpt_sign_in_included_usage"}
	got = priceFixture(rules, enterprise)
	if got.ProviderEstimate != nil || got.Subscription.Status != "unknown" {
		t.Fatalf("generic sign-in crossed into enterprise: %+v", got)
	}

	for _, tc := range []struct {
		name, at, channel, activity, status, eligibility, reason string
		signIn                                                   bool
	}{
		{"before announcement", "2026-10-06T07:12:59Z", "personal_subscription", "guardian_approval", "unknown", "unknown", "policy_unverified", true},
		{"minute precision", "2026-10-06T07:13:59Z", "personal_subscription", "guardian_approval", "unknown", "unknown", "policy_unverified", true},
		{"after announcement", "2026-10-06T07:14:00Z", "personal_subscription", "guardian_approval", "free_activity", "excluded", "guardian_free", true},
		{"unknown identity", now, "personal_subscription", "guardian_approval", "unknown", "unknown", "policy_unverified", false},
		{"API Key", now, "api_key", "guardian_approval", "metered", "excluded", "paid_channel", false},
		{"Enterprise USD", now, "enterprise_usd", "guardian_approval", "free_activity", "excluded", "guardian_free", true},
		{"only model name", now, "personal_subscription", "", "unknown", "unknown", "activity_unverified", true},
		{"GitHub PR review", now, "personal_subscription", "github_review", "unknown", "unknown", "policy_unverified", true},
		{"local review", now, "personal_subscription", "local_review", "unknown", "unknown", "policy_unverified", true},
		{"Codex Security", now, "personal_subscription", "codex_security", "unknown", "unknown", "policy_unverified", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := codexPricingEvent("codex-auto-review", tc.at, 15, 0, 0, 1)
			e := event.Facts.Pricing
			e.Channel, e.Activity, e.ChannelEndpoint = tc.channel, tc.activity, "codex"
			if tc.signIn {
				e.Eligibility = []string{"chatgpt_sign_in"}
				if e.Channel == "enterprise_usd" {
					e.Eligibility = []string{"enterprise_usd_codex_chatgpt_sign_in"}
				}
			}
			got := priceFixture(rules, event)
			if got.Subscription.Status != tc.status || got.Subscription.QuotaEligibility != tc.eligibility || got.Subscription.QuotaReason != tc.reason {
				t.Fatalf("subscription = %+v", got.Subscription)
			}
			if got.APIEquivalent.Amount != nil || got.PricedModel != "" {
				t.Fatalf("activity supplied a model or zero API price: %+v", got)
			}
			if tc.status == "free_activity" && (got.ProviderEstimate != nil || got.Subscription.PolicyID != "openai.codex.guardian.free.2026-10-06") {
				t.Fatalf("free Guardian = %+v", got)
			}
		})
	}

	for _, tc := range []struct{ at, channel, endpoint, qualification, status, long, quota string }{
		{"2026-10-11T23:47:59Z", "personal_subscription", "codex", "chatgpt_sign_in_included_usage", "included", "unknown", "eligible"},
		{"2026-10-11T23:48:30Z", "personal_subscription", "codex", "chatgpt_sign_in_included_usage", "included", "unknown", "eligible"},
		{"2026-10-11T23:49:00Z", "personal_subscription", "chatgpt_work", "chatgpt_sign_in_included_usage", "included", "exempt", "eligible"},
		{now, "personal_subscription", "codex", "chatgpt_sign_in", "unknown", "unknown", "unknown"},
		{now, "api_key", "codex", "chatgpt_sign_in_included_usage", "metered", "not_applicable", "excluded"},
	} {
		event := codexPricingEvent("gpt-6.1-sol", tc.at, 300_000, 0, 0, 10_000)
		e := event.Facts.Pricing
		e.Channel, e.ChannelEndpoint, e.Eligibility = tc.channel, tc.endpoint, []string{tc.qualification}
		result := priceFixture(rules, event)
		checkPriceAmount(t, result.APIEquivalent, pricingComplete, "1.350000000", "1.350000000")
		if result.Subscription.Status != tc.status || result.Subscription.SubscriptionLongContext != tc.long || result.Subscription.QuotaEligibility != tc.quota ||
			(result.ProviderEstimate == nil) != (tc.channel != "api_key") || (result.Subscription.PolicyID != "") != (tc.long == "exempt") {
			t.Fatalf("%+v: %+v", tc, result)
		}
	}

	// A typed paid Guardian visits the API tariff just like another paid call.
	paid := codexPricingEvent("gpt-6.1-sol", now, 300_000, 0, 0, 10_000)
	paid.Facts.Pricing.Activity = "guardian_approval"
	got = priceFixture(rules, paid)
	checkPriceAmount(t, *got.ProviderEstimate, pricingComplete, "1.350000000", "1.350000000")
}

func TestRequestPricingExecutionAndSpeedEvidence(t *testing.T) {
	rules := fixtureInvocationRules(t, "2027-03-04T00:00:00Z")
	const now = "2026-11-21T00:00:00Z"
	event := codexPricingEvent("gpt-6.2-astra", now, 100_000, 0, 0, 0)
	e := event.Facts.Pricing
	e.RequestedTier = "priority"
	event.Facts.ExecutedTier = "default"
	got := priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "0.800000000", "0.800000000")
	if got.RequestedTier != "fast" || got.ExecutedTier != "standard" || got.Reference != nil {
		t.Fatalf("response downgrade result = %+v", got)
	}

	event.Facts.ExecutedTier = ""
	got = priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingUnavailable, "", "")
	checkPricingReason(t, got, reasonTier)
	if got.Reference == nil || got.Reference.Basis != "requested_tier" || valueString(got.Reference.Amount) != "1.600000000" ||
		got.Standard == nil || got.Standard.Basis != "standard" || valueString(got.Standard.Amount) != "0.800000000" {
		t.Fatalf("requested vs standard = %+v / %+v", got.Reference, got.Standard)
	}

	e.RequestedTier = ""
	got = priceFixture(rules, event)
	if got.Reference == nil || got.Reference.Basis != "standard" || got.Reference.Tier != "standard" {
		t.Fatalf("reference = %+v", got.Reference)
	}
	e.Region = ""
	got = priceFixture(rules, event)
	if got.Reference != nil || got.Standard != nil {
		t.Fatalf("unverified region admitted a standard reference: %+v", got)
	}

	e.Region, e.Channel, e.ChannelEndpoint = "global", "personal_subscription", "codex"
	e.Eligibility = []string{"chatgpt_sign_in_included_usage_ultrafast_eligible"}
	event.Facts.ExecutedTier = "ultrafast"
	got = priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "2.400000000", "2.400000000")
	if got.ProviderEstimate != nil || got.Subscription.SpeedPolicy == nil || got.Subscription.SpeedPolicy.Multiplier != "6" ||
		got.Components[0].Modifier == nil || got.Components[0].Modifier.Multiplier != "3" || got.Subscription.QuotaEligibility != "eligible" {
		t.Fatalf("API and included speed domains = %+v", got)
	}
	event.EventAt = "2027-03-01T08:00:00Z" // midnight in the published zone
	got = priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "4.800000000", "4.800000000")
	event.EventAt = "2027-03-01T07:59:59.999999999Z"
	got = priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "2.400000000", "2.400000000")
	e.Region = "eu"
	got = priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingUnavailable, "", "")
	if got.Subscription.SpeedPolicy == nil || got.Subscription.SpeedPolicy.Multiplier != "6" {
		t.Fatalf("API region changed subscription policy: %+v", got.Subscription)
	}
}

func TestRequestPricingHistoricalAndPartial(t *testing.T) {
	rules := fixtureInvocationRules(t, "2026-11-25T00:00:00Z")
	for _, tc := range []struct{ at, amount string }{
		{"2026-07-08T09:59:59Z", ""},
		{"2026-07-08T10:00:00Z", "0.100000000"}, // earliest possible launch day
		{"2026-07-29T09:59:59.999Z", "0.100000000"},
		{"2026-07-29T10:00:00Z", ""},
		{"2026-07-31T11:59:59.999Z", ""},
		{"2026-07-31T12:00:00Z", "0.020000000"},
	} {
		t.Run(tc.at, func(t *testing.T) {
			event := codexPricingEvent("gpt-5.6-luna", tc.at, 100_000, 0, 0, 0)
			got := priceFixture(rules, event)
			if tc.amount == "" {
				checkPriceAmount(t, got.APIEquivalent, pricingUnavailable, "", "")
				if got.Reference == nil || got.Reference.Basis != "as_of_date" || got.Reference.AsOf != "2026-11-25" {
					t.Fatalf("dated missing price reference = %+v", got.Reference)
				}
			} else {
				checkPriceAmount(t, got.APIEquivalent, pricingComplete, tc.amount, tc.amount)
			}
		})
	}

	event := claudePricingEvent("claude-sonnet-5-5", "2026-10-07T00:00:00Z", 500_000, 20_000, 0, 0, 0, 10_000)
	got := priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingPartial, "", "1.100000000")
	checkPricingReason(t, got, reasonBoundary)

	const now = "2026-11-21T00:00:00Z"
	astra := codexPricingEvent("gpt-6.2-astra", now, 272_000, 100_000, 20_000, 10_000)
	got = priceFixture(rules, astra)
	checkPriceAmount(t, got.APIEquivalent, pricingPartial, "", "1.656000000")
	checkPricingReason(t, got, reasonTTL)
	if got.Reference == nil || got.Reference.Basis != "cache_write_ttl_range" || valueString(got.Reference.Lower) != "1.856000000" ||
		valueString(got.Reference.Upper) != "2.056000000" || !slices.Equal(got.Reference.TTLCandidates, []string{"30m", "12h"}) {
		t.Fatalf("TTL range = %+v", got.Reference)
	}
	astra.Facts.CacheWrite30m = pricingInt(5000)
	got = priceFixture(rules, astra)
	checkPriceAmount(t, got.APIEquivalent, pricingPartial, "", "1.706000000")
	checkPricingReason(t, got, reasonTTL)

	// A known short-only cache domain and a complete reported total price
	// the total. The same inference no longer applies after 12h is introduced.
	old := codexPricingEvent("gpt-5.6-luna", "2026-09-01T00:00:00Z", 1000, 0, 200, 0)
	got = priceFixture(rules, old)
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "0.000210000", "0.000210000")

	haiku := claudePricingEvent("claude-haiku-5-5", now, 49_000, 40_000, 11_000, 0, 0, 5_000)
	haiku.Facts.CacheWrite5m, haiku.Facts.CacheWrite1h = nil, nil
	got = priceFixture(rules, haiku)
	checkPriceAmount(t, got.APIEquivalent, pricingPartial, "", "0.007800000")
	if got.Reference == nil || got.Reference.Basis != "assumed_5m" || valueString(got.Reference.Amount) != "0.009175000" {
		t.Fatalf("5m reference = %+v", got.Reference)
	}

	astra.Facts.CacheWriteTokens, astra.Facts.CacheWrite30m = nil, nil
	got = priceFixture(rules, astra)
	checkPriceAmount(t, got.APIEquivalent, pricingPartial, "", "0.440000000")
	checkPricingReason(t, got, reasonUnmetered)
	astra.Facts = nil
	got = priceFixture(rules, astra)
	checkPriceAmount(t, got.APIEquivalent, pricingUnavailable, "", "")
	checkPricingReason(t, got, reasonInvocation)

	bad := codexPricingEvent("gpt-6.2-astra", now, 100, 90, 20, 1)
	got = priceFixture(rules, bad)
	checkPriceAmount(t, got.APIEquivalent, pricingUnavailable, "", "")
	checkPricingReason(t, got, reasonCounts)
	zero := codexPricingEvent("gpt-6.2-astra", now, 0, 0, 0, 0)
	got = priceFixture(rules, zero)
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "0.000000000", "0.000000000")
	zero.Model = "gpt-missing"
	checkPriceAmount(t, priceFixture(rules, zero).APIEquivalent, pricingUnavailable, "", "")
}

func TestRequestPricingReadsAdmittedInvocationsAndLegacyAmounts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	path := filepath.Join(home, ".codex", "sessions", "2026", "pricing.jsonl")
	const transcript = `{"type":"session_meta","payload":{"session_id":"s","cli_version":"0.169.0"}}
{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-6.2-astra","service_tier":"priority"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","service_tier":"default","activity":"guardian_approval"}}
{"type":"event_msg","timestamp":"2026-11-21T00:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":272000,"cached_input_tokens":100000,"cache_write_input_tokens":20000,"output_tokens":10000,"input_tokens_details":{"cache_write_tokens_details":{"ttl_30m_tokens":20000}}},"total_token_usage":{"input_tokens":272000,"cached_input_tokens":100000,"cache_write_input_tokens":20000,"output_tokens":10000}}}}
{"type":"event_msg","timestamp":"2026-11-21T00:00:00.1Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":272001,"cached_input_tokens":100000,"cache_write_input_tokens":20000,"output_tokens":10000,"input_tokens_details":{"cache_write_tokens_details":{"ttl_30m_tokens":20000}}},"total_token_usage":{"input_tokens":544001,"cached_input_tokens":200000,"cache_write_input_tokens":40000,"output_tokens":20000}}}}
{"type":"event_msg","timestamp":"2026-11-21T00:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"output_tokens":2},"total_token_usage":{"input_tokens":544101,"cached_input_tokens":200000,"cache_write_input_tokens":40005,"output_tokens":20002}}}}
{"type":"event_msg","timestamp":"2026-11-21T00:00:02Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1,"output_tokens":1},"total_token_usage":{"input_tokens":544121,"cached_input_tokens":200000,"cache_write_input_tokens":40005,"output_tokens":20003}}}}
{"type":"event_msg","timestamp":"2026-11-21T00:00:03Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":0,"output_tokens":2,"input_tokens_details":{"cache_write_tokens_details":{"ttl_30m_tokens":0,"ttl_12h_tokens":0}}},"total_token_usage":{"input_tokens":544221,"cached_input_tokens":200000,"cache_write_input_tokens":40010,"output_tokens":20005}}}}
`
	writeFactsFixture(t, path, transcript)
	writeFactsFixture(t, filepath.Join(filepath.Dir(path), "pricing-copy.jsonl"), transcript)
	db := openPricingTestStore(t, filepath.Join(root, "state"))
	s := New(db, home)
	s.Now = func() time.Time { return pricingTime("2026-11-25T00:00:00Z") }
	if _, err := s.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	events, err := s.events(ctx, "codex", "s")
	if err != nil || len(events) != 5 {
		t.Fatalf("existing admission/deduplication read %d, %v", len(events), err)
	}
	installLegacyAstraFixture(t, s)
	execPricingSQL(t, db, `INSERT INTO usage_session_routes(client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,source,quality,semantic_key)
VALUES('codex','s','2026-11-20T23:59:00Z','official','1',0,'SessionStart','fixture','exact','pricing-fixture')`)
	legacyBefore, err := s.SessionUsageSummary(ctx, "codex", "s")
	if err != nil || legacyBefore.ProviderCost == nil {
		t.Fatalf("legacy summary = %+v (%v)", legacyBefore, err)
	}
	byTime := make(map[string]storedEvent)
	for _, e := range events {
		byTime[e.EventAt] = e
	}
	if byTime["2026-11-21T00:00:02Z"].Facts != nil {
		t.Fatal("count mismatch established an invocation boundary")
	}
	third := byTime["2026-11-21T00:00:01Z"]
	if third.Facts == nil || third.Facts.CacheWriteTokens != nil || third.Tokens["cache_write_tokens"] != 5 {
		t.Fatalf("missing last field became a mismatch or a confirmed count: %+v", third)
	}
	conflicting := byTime["2026-11-21T00:00:03Z"]
	if conflicting.Facts == nil || conflicting.Facts.CacheWriteTokens != nil || conflicting.Tokens["cache_write_tokens"] != 5 ||
		conflicting.Facts.CacheWrite30m == nil || *conflicting.Facts.CacheWrite30m != 0 ||
		conflicting.Facts.CacheWrite12h == nil || *conflicting.Facts.CacheWrite12h != 0 {
		t.Fatalf("fixture did not retain conflicting TTL evidence: %+v", conflicting)
	}

	ungated, err := s.PriceInvocations(ctx, InvocationPricingFilter{SessionID: "s"})
	if err != nil || len(ungated.Invocations) != 5 || !slices.Equal(ungated.Reasons, []string{reasonCoverage}) {
		t.Fatalf("admitted pricing read: %+v (%v)", ungated, err)
	}
	for _, call := range ungated.Invocations {
		if call.ExecutedTier != "unknown" || call.Subscription.Activity != "unknown" || call.ProviderEstimate != nil {
			t.Fatalf("transcript context/response item proved execution or billing: %+v", call)
		}
	}
	checkPricingReason(t, ungated.Invocations[3], reasonInvocation)

	// Add test-only bound evidence to the existing fact rows. The producer
	// under test above supplied identity and counts; there is no live account.
	for _, at := range []string{"2026-11-21T00:00:00Z", "2026-11-21T00:00:00.1Z", "2026-11-21T00:00:01Z", "2026-11-21T00:00:03Z"} {
		e := byTime[at]
		e.Facts.ExecutedTier = "default"
		e.Facts.Pricing = &RequestPricingEvidence{
			APIEndpoint: "v1/responses", Region: "global", RequestedTier: "priority", Channel: "personal_subscription",
			ChannelEndpoint: "codex", Activity: "ordinary", Eligibility: []string{"chatgpt_sign_in_included_usage"},
		}
		payload, err := encodeRequestFacts(e.Event)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.ExecContext(ctx, `UPDATE usage_request_facts SET facts_json=? WHERE event_key=?`, payload.String, e.Key); err != nil {
			t.Fatal(err)
		}
	}
	filter := InvocationPricingFilter{Client: "codex", SessionID: "s", Model: "gpt-6.2-astra"}
	priced, err := s.PriceInvocations(ctx, filter)
	if err != nil || len(priced.Invocations) != 5 || priced.RuleSet == "" {
		t.Fatalf("priced read: %+v (%v)", priced, err)
	}
	for i, want := range []string{"1.856000000", "3.512016000"} {
		got := priced.Invocations[i]
		checkPriceAmount(t, got.APIEquivalent, pricingComplete, want, want)
		if got.EventKey != byTime[got.EventAt].Key || got.Client != "codex" || got.SessionID != "s" || got.RuleSet != priced.RuleSet ||
			got.Version != RequestPricingVersion || got.ProviderEstimate != nil || got.Subscription.QuotaEligibility != "eligible" ||
			got.Subscription.QuotaReason != "included_observed" || got.ExecutedTier != "standard" {
			t.Fatalf("quota handoff = %+v", got)
		}
	}
	checkPriceAmount(t, priced.Invocations[2].APIEquivalent, pricingPartial, "", "0.000080000")
	checkPricingReason(t, priced.Invocations[2], reasonUnmetered)
	checkPriceAmount(t, priced.Invocations[4].APIEquivalent, pricingUnavailable, "", "")
	checkPricingReason(t, priced.Invocations[4], reasonCounts)
	for i, after := range priced.Invocations {
		before := ungated.Invocations[i]
		if !reflect.DeepEqual(after.Legacy, before.Legacy) || before.Legacy.ProviderCost == nil ||
			!reflect.DeepEqual(after.Legacy.Tokens, byTime[after.EventAt].Tokens) {
			t.Fatalf("legacy output changed for %s: %+v -> %+v", after.EventKey, before.Legacy, after.Legacy)
		}
	}
	if legacyAfter, err := s.SessionUsageSummary(ctx, "codex", "s"); err != nil || !reflect.DeepEqual(legacyAfter, legacyBefore) {
		t.Fatalf("v1 session output changed: %+v (%v)", legacyAfter, err)
	}
	if again, err := s.PriceInvocations(ctx, filter); err != nil || !reflect.DeepEqual(again, priced) {
		t.Fatalf("repeat differs: err=%v", err)
	}
	filter.From, filter.To = pricingTime("2026-11-21T00:00:00.01Z"), pricingTime("2026-11-21T00:00:01Z")
	window, err := s.PriceInvocations(ctx, filter)
	if err != nil || len(window.Invocations) != 1 || window.Invocations[0].EventKey != priced.Invocations[1].EventKey {
		t.Fatalf("half-open subsecond window = %+v, %v", window, err)
	}
	if window, err = s.PriceInvocations(ctx, InvocationPricingFilter{Client: "claude"}); err != nil || len(window.Invocations) != 0 || len(window.Reasons) != 0 {
		t.Fatalf("empty Claude window = %+v, %v", window, err)
	}
}

func TestRequestPricingLegacyReadOnlyCatalog(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db := openPricingTestStore(t, dir)
	s := New(db, "")
	s.Now = func() time.Time { return pricingTime("2026-11-25T00:00:00Z") }
	if err := s.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	installLegacyAstraFixture(t, s)
	event := codexPricingEvent("gpt-6.2-astra", "2026-11-21T00:00:00Z", 1000, 0, 0, 2)
	if _, _, err := upsertTx(ctx, db.DB, event.Event); err != nil {
		t.Fatal(err)
	}
	execPricingSQL(t, db, `DROP TABLE usage_request_facts; DROP TABLE price_rule_sets; UPDATE schema_metadata SET version=35`)
	readOnly, err := store.OpenReadOnly(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	legacyService := New(readOnly, "")
	legacyService.Now = s.Now
	priced, err := legacyService.PriceInvocations(ctx, InvocationPricingFilter{Client: "codex"})
	if err != nil || len(priced.Invocations) != 1 {
		t.Fatalf("legacy read = %+v (%v)", priced, err)
	}
	result := priced.Invocations[0]
	checkPriceAmount(t, result.APIEquivalent, pricingUnavailable, "", "")
	checkPricingReason(t, result, reasonInvocation)
	if result.Reference == nil || result.Reference.Basis != "legacy_catalog" ||
		valueString(result.Reference.Amount) != valueString(result.Legacy.CatalogBaseCost) || result.ProviderEstimate != nil {
		t.Fatalf("legacy reference = %+v", result)
	}
}

func TestRequestPricingAcrossCatalogsRequiresEvidenceToChoose(t *testing.T) {
	rules := invocationRules{cutoff: pricingTime("2026-11-25T00:00:00Z"), models: map[ruleModel][]*invocationRule{}}
	old := fixturePriceRule("published", "2")
	conflict := fixturePriceRule("other", "3")
	add := func(rows ...PriceRule) {
		raw, err := json.Marshal(PriceRuleSet{SchemaVersion: 2, Rules: rows})
		if err != nil {
			t.Fatal(err)
		}
		set, err := DecodePriceRules(raw)
		if err != nil {
			t.Fatal(err)
		}
		rules.add(set)
	}
	add(old)
	add(conflict)
	selection := tariffSelection{provider: old.Selector.Provider, model: old.Selector.Models[0], channel: old.Selector.Channel,
		tier: old.Selector.ServiceTier, endpoint: old.Selector.Endpoints[0], region: old.Selector.Regions[0], singleInvocation: true}
	at := pricingTime("2026-11-22T00:00:00Z")
	if term := rules.selectTerm(selection, at, tariffField{rate: "input"}); term.reason != reasonConflict {
		t.Fatalf("equal authority conflict = %+v", term)
	}
	replacement := fixturePriceRule("correction", "4")
	replacement.Supersedes = []string{old.ID, conflict.ID}
	// The copy included in this set has a precedence edge for validation.
	conflict.Supersedes = []string{old.ID}
	add(old, conflict, replacement)
	if term := rules.selectTerm(selection, at, tariffField{rate: "input"}); term.rule == nil || term.value != "4" {
		t.Fatalf("documented replacement = %+v", term)
	}
}

func TestRequestPricingThirdPartyMultiplierRequiresAttributableRoute(t *testing.T) {
	rules := fixtureInvocationRules(t, "2026-11-25T00:00:00Z")
	event := codexPricingEvent("gpt-6.1-sol", "2026-11-21T00:00:00Z", 300_000, 0, 0, 10_000)
	event.Facts.Pricing.Channel = ""
	attribution := eventAttribution{quality: "exact", provider: "local-fixture-proxy", spendEligible: true, multiplier: "1.125"}
	got := rules.evaluate(event, pricingTime(event.EventAt), attribution, Result{})
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "1.350000000", "1.350000000")
	if got.ProviderEstimate == nil || got.Subscription.Channel != "third_party" || got.Subscription.QuotaEligibility != "excluded" {
		t.Fatalf("third-party estimate = %+v", got)
	}
	checkPriceAmount(t, *got.ProviderEstimate, pricingComplete, "1.518750000", "1.518750000")
	attribution.quality, attribution.reason = "estimated", attributionReasonAmbiguousRoute
	got = rules.evaluate(event, pricingTime(event.EventAt), attribution, Result{})
	if got.ProviderEstimate != nil || got.Subscription.Channel != "unknown" {
		t.Fatalf("ambiguous route supplied a paid channel: %+v", got)
	}
	checkPricingReason(t, got, reasonRoute)
	attribution.quality, attribution.provider = "exact", "official"
	got = rules.evaluate(event, pricingTime(event.EventAt), attribution, Result{})
	if got.ProviderEstimate != nil || got.Subscription.Channel != "unknown" {
		t.Fatalf("official route supplied an authentication channel: %+v", got)
	}
}

func installLegacyAstraFixture(t *testing.T, s *Service) {
	t.Helper()
	err := s.ImportOfficialOverrides(context.Background(), []OfficialOverride{{
		Model: "gpt-6.2-astra", Provider: "openai", SourceURL: "https://developers.openai.com/api/docs/pricing",
		EffectiveFrom: pricingTime("2026-11-01T00:00:00Z"),
		Prices:        map[string]string{"input": "5", "cached_input": "0.5", "output": "30"},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequestPricingHistoricalTariffChanges(t *testing.T) {
	rules := fixtureInvocationRules(t, "2026-11-25T00:00:00Z")
	for _, tc := range []struct {
		model, before, boundary, after, old, new string
	}{
		{"gpt-5.6-terra", "2026-07-29T09:59:59Z", "2026-07-30T00:00:00Z", "2026-07-31T12:00:00Z", "0.250000000", "0.200000000"},
		{"gpt-5.6-sol", "2026-08-20T09:59:59Z", "2026-08-21T00:00:00Z", "2026-08-22T12:00:00Z", "0.500000000", "0.400000000"},
		{"claude-opus-4-6", "2026-03-12T09:59:59Z", "2026-03-13T00:00:00Z", "2026-03-14T12:00:00Z", "3.000000000", "1.500000000"},
		{"claude-sonnet-4-6", "2026-03-12T09:59:59Z", "2026-03-13T00:00:00Z", "2026-03-14T12:00:00Z", "1.800000000", "0.900000000"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			event := codexPricingEvent(tc.model, tc.before, 100_000, 0, 0, 0)
			if strings.HasPrefix(tc.model, "claude-") {
				event = claudePricingEvent(tc.model, tc.before, 300_000, 0, 0, 0, 0, 0)
				event.Facts.Pricing.Eligibility = []string{"claude_context_1m_beta"}
			}
			checkPriceAmount(t, priceFixture(rules, event).APIEquivalent, pricingComplete, tc.old, tc.old)
			event.EventAt = tc.boundary
			got := priceFixture(rules, event)
			checkPriceAmount(t, got.APIEquivalent, pricingUnavailable, "", "")
			checkPricingReason(t, got, reasonBoundary)
			event.EventAt = tc.after
			checkPriceAmount(t, priceFixture(rules, event).APIEquivalent, pricingComplete, tc.new, tc.new)
			if event.Client == "claude" {
				event.EventAt, event.Facts.Pricing.Eligibility = tc.before, nil
				got := priceFixture(rules, event)
				checkPriceAmount(t, got.APIEquivalent, pricingUnavailable, "", "")
				checkPricingReason(t, got, reasonSelector)
			}
		})
	}
}

func TestRequestPricingAliasAndPrecisionRequireIndependentEvidence(t *testing.T) {
	rules := fixtureInvocationRules(t, "2026-11-25T00:00:00Z")
	event := codexPricingEvent("gpt-5.6", "2026-08-01T00:00:00Z", 100_000, 0, 0, 0)
	got := priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "0.500000000", "0.500000000")
	if got.ObservedModel != "gpt-5.6" || got.PricedModel != "gpt-5.6-sol" {
		t.Fatalf("alias = %q / %q", got.ObservedModel, got.PricedModel)
	}
	event.Facts.Pricing.Channel = "personal_subscription"
	got = priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingUnavailable, "", "")
	checkPricingReason(t, got, reasonModel)
	event.Facts.Pricing.ActualModel = "gpt-5.6-luna"
	got = priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "0.020000000", "0.020000000")
	if got.ObservedModel != "gpt-5.6" || got.PricedModel != "gpt-5.6-luna" {
		t.Fatalf("actual model = %q / %q", got.ObservedModel, got.PricedModel)
	}

	precisionRule := fixturePriceRule("exact-decimal", "0.00012")
	validated, err := DecodePriceRules(marshalPriceRules(t, precisionRule))
	if err != nil {
		t.Fatal(err)
	}
	rules.add(validated)
	event = codexPricingEvent("gpt-example", "2026-11-21T00:00:00Z", 1, 0, 0, 0)
	got = priceFixture(rules, event)
	checkPriceAmount(t, got.APIEquivalent, pricingComplete, "0.00000000012", "0.00000000012")
}
