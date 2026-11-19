package usage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

const RequestPricingVersion = 1

const (
	pricingComplete    = "complete"
	pricingPartial     = "partial"
	pricingUnavailable = "unavailable"
)

const (
	reasonModel      = "model_unresolved"
	reasonHistorical = "historical_rule_missing"
	reasonBoundary   = "effective_boundary_unverified"
	reasonTier       = "service_tier_unverified"
	reasonContext    = "context_policy_unverified"
	reasonInvocation = "invocation_boundary_unverified"
	reasonTTL        = "cache_write_ttl_unverified"
	reasonUnmetered  = "component_unmetered"
	reasonCounts     = "conflicting_counts"
	reasonRoute      = "provider_route_unverified"
	reasonActivity   = "activity_unverified"
	reasonCoverage   = "activity_coverage_unavailable"
	reasonConflict   = "pricing_rule_conflict"
	reasonSelector   = "pricing_selector_unverified"
)

// RequestPricingEvidence contains only pricing selectors independently tied
// to this admitted invocation by its fact producer. APIEndpoint identifies
// the corresponding public API operation; ChannelEndpoint is where it ran.
// Eligibility holds allowlisted tariff qualifications, never account IDs.
type RequestPricingEvidence struct {
	ActualModel     string   `json:"actual_model,omitempty"`
	RequestedTier   string   `json:"requested_tier,omitempty"`
	APIEndpoint     string   `json:"api_endpoint,omitempty"`
	Region          string   `json:"region,omitempty"`
	Channel         string   `json:"channel,omitempty"`
	ChannelEndpoint string   `json:"channel_endpoint,omitempty"`
	Activity        string   `json:"activity,omitempty"`
	Eligibility     []string `json:"eligibility,omitempty"`
}

type PricingAmount struct {
	Currency    string
	Status      string
	Amount      *string
	KnownAmount *string
}

type PricingReference struct {
	Basis         string
	Currency      string
	AsOf          string
	Amount        *string
	Lower         *string
	Upper         *string
	Tier          string
	ContextUnit   string
	TTLCandidates []string
}

type PricingComponent struct {
	Kind           string
	Quantity       int64
	Unit           string
	Currency       string
	Rate           *string
	Amount         *string
	RuleID         string
	SourceURL      string
	EffectiveFrom  *PriceRuleBoundary
	EffectiveUntil *PriceRuleBoundary
	Modifier       *PricingPolicy
	Treatment      *PricingPolicy
}

type PricingPolicy struct {
	ID         string
	RuleID     string
	SourceURL  string
	Multiplier string
}

type PricingSubscription struct {
	Channel                 string
	Status                  string
	Activity                string
	QuotaEligibility        string
	QuotaReason             string
	SubscriptionLongContext string
	PolicyID                string
	SpeedPolicy             *PricingPolicy
}

// InvocationPricing is the shared calculation result, including the identity
// and evidence needed by quota consumers. Reporting owns its wire projection.
type InvocationPricing struct {
	Client             string
	EventKey           string
	SessionID          string
	EventAt            string
	Version            int
	RuleSet            string
	ObservedModel      string
	PricedModel        string
	RequestedTier      string
	ExecutedTier       string
	ContextTier        string
	APIEquivalent      PricingAmount
	ProviderEstimate   *PricingAmount
	Reference          *PricingReference
	Standard           *PricingReference
	Components         []PricingComponent
	ProviderComponents []PricingComponent
	Subscription       PricingSubscription
	Reasons            []string
	Legacy             Result
	LegacyError        string
}

type InvocationPricingFilter struct {
	Client    string
	SessionID string
	Model     string
	From      time.Time
	To        time.Time
}

type InvocationPricingRead struct {
	KnowledgeCutoff time.Time
	RuleSet         string
	Invocations     []InvocationPricing
	Reasons         []string
}

// PriceInvocations evaluates precisely the admitted events. The route timeline
// is loaded using the existing attribution reader and its revision is checked
// against the read transaction before any results are returned. That same
// transaction supplies the routes, v1 catalog, dated rules and event facts.
func (s *Service) PriceInvocations(ctx context.Context, filter InvocationPricingFilter) (InvocationPricingRead, error) {
	cutoff := s.now()
	revision, err := s.Store.UsageProjectionMetadataRevision(ctx)
	if err != nil {
		return InvocationPricingRead{}, err
	}
	timeline, err := s.Store.LoadProviderTimeline(ctx)
	if err != nil {
		return InvocationPricingRead{}, err
	}
	tx, err := s.Store.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return InvocationPricingRead{}, err
	}
	defer tx.Rollback()
	var snapshotRevision int64
	err = tx.QueryRowContext(ctx, "SELECT revision FROM usage_derived_metadata WHERE singleton=1").Scan(&snapshotRevision)
	if err != nil {
		return InvocationPricingRead{}, err
	}
	if revision != snapshotRevision {
		return InvocationPricingRead{}, store.ErrUsagePartitionChanged
	}
	var schema int
	if err = tx.QueryRowContext(ctx, "SELECT version FROM schema_metadata").Scan(&schema); err != nil {
		return InvocationPricingRead{}, err
	}
	supported := schema >= store.PricingFactsSchemaVersion
	resolver, err := loadReadPriceRows(ctx, tx, cutoff, timeline)
	if err != nil {
		return InvocationPricingRead{}, err
	}
	rules, err := loadInvocationRules(ctx, tx, cutoff, supported)
	if err != nil {
		return InvocationPricingRead{}, err
	}
	query := requestEventSelect(supported)
	var clauses []string
	var args []any
	add := func(clause string, value any) {
		clauses, args = append(clauses, clause), append(args, value)
	}
	if filter.Client != "" {
		add("e.client=?", filter.Client)
	}
	if filter.SessionID != "" {
		add("e.session_id=?", filter.SessionID)
	}
	if filter.Model != "" {
		add("e.model=?", filter.Model)
	}
	if !filter.From.IsZero() {
		add("e.event_at>=?", filter.From.UTC().Truncate(time.Second).Format("2006-01-02T15:04:05"))
	}
	if !filter.To.IsZero() {
		add("e.event_at<?", filter.To.UTC().Truncate(time.Second).Add(time.Second).Format("2006-01-02T15:04:05"))
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	rows, err := tx.QueryContext(ctx, query+" ORDER BY substr(e.event_at,1,19),substr(e.event_at,20,length(e.event_at)-20),e.event_key", args...)
	if err != nil {
		return InvocationPricingRead{}, err
	}
	defer rows.Close()
	out := InvocationPricingRead{KnowledgeCutoff: cutoff, RuleSet: rules.digest, Invocations: []InvocationPricing{}}
	err = visitRequestEvents(rows, func(event storedEvent) error {
		at, err := time.Parse(time.RFC3339Nano, event.EventAt)
		if err != nil {
			return err
		}
		if (!filter.From.IsZero() && at.Before(filter.From)) || (!filter.To.IsZero() && !at.Before(filter.To)) {
			return nil
		}
		attribution, err := resolver.priceForEvent(event)
		if err != nil {
			return err
		}
		legacy, err := calculateAttributedEvent(event, attribution)
		priced := rules.evaluate(event, at, attribution, legacy)
		if err != nil {
			priced.LegacyError = err.Error()
		}
		out.Invocations = append(out.Invocations, priced)
		if event.Client == "codex" && len(out.Reasons) == 0 {
			// Neither admitted Codex producer currently proves period-wide
			// Guardian collection coverage.
			out.Reasons = []string{reasonCoverage}
		}
		return nil
	})
	if err != nil {
		return InvocationPricingRead{}, err
	}
	if err = tx.Commit(); err != nil {
		return InvocationPricingRead{}, err
	}
	return out, nil
}

func loadInvocationRules(ctx context.Context, tx *sql.Tx, cutoff time.Time, supported bool) (invocationRules, error) {
	out := invocationRules{cutoff: cutoff, models: map[ruleModel][]*invocationRule{}}
	query := `SELECT c.version,c.content_sha256,NULL FROM price_catalogs c ORDER BY c.version`
	if supported {
		query = `SELECT c.version,c.content_sha256,p.rules_json FROM price_catalogs c
LEFT JOIN price_rule_sets p ON p.catalog_version=c.version ORDER BY c.version`
	}
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	digest := sha256.New()
	for rows.Next() {
		var version, content string
		var raw sql.NullString
		if err = rows.Scan(&version, &content, &raw); err != nil {
			return out, err
		}
		var set PriceRuleSet
		if raw.Valid {
			set, err = decodePriceRules([]byte(raw.String), true)
			if err != nil {
				return out, err
			}
			out.add(set)
		}
		// Quoted keys keep the encoding unambiguous for imported version IDs.
		fmt.Fprintf(digest, "%q\t%s\t%s\n", version, content, set.Digest)
	}
	out.digest = hex.EncodeToString(digest.Sum(nil))
	return out, rows.Err()
}

type pricingReasons []string

func (r *pricingReasons) add(reason string) {
	if reason != "" && !slices.Contains(*r, reason) {
		*r = append(*r, reason)
	}
}

func (r invocationRules) evaluate(event storedEvent, at time.Time, attribution eventAttribution, legacy Result) InvocationPricing {
	out := InvocationPricing{
		Client: event.Client, EventKey: event.Key, SessionID: event.SessionID, EventAt: event.EventAt,
		Version: RequestPricingVersion, RuleSet: r.digest, ObservedModel: event.Model,
		ExecutedTier: "unknown", ContextTier: "unknown", Legacy: legacy,
		APIEquivalent: PricingAmount{Currency: pricingCurrency, Status: pricingUnavailable},
	}
	var evidence RequestPricingEvidence
	var tier string
	if event.Facts != nil {
		tier = pricingTier(event.Facts.ExecutedTier)
		if event.Facts.Pricing != nil {
			evidence = *event.Facts.Pricing
		}
	}
	if tier != "" {
		out.ExecutedTier = tier
	}
	out.RequestedTier = pricingTier(evidence.RequestedTier)
	model := event.Model
	if evidence.ActualModel != "" {
		model = evidence.ActualModel
	}
	model = readPriceModelKey(event.Client, model)
	provider := map[string]string{"codex": "openai", "claude": "anthropic"}[event.Client]
	counts := pricingCounts(event)
	s := tariffSelection{
		provider: provider, channel: "public_api", model: model, endpoint: evidence.APIEndpoint,
		tier: tier, region: evidence.Region, activity: selectorActivity(evidence.Activity),
		eligibility: evidencedQualifications(evidence.Eligibility, tier, false),
		context:     counts.context(provider), singleInvocation: event.Facts != nil,
		allowAlias: evidence.Channel == "api_key",
	}
	if s.endpoint == "" && event.Facts != nil {
		switch event.Facts.Source {
		case "claude_jsonl":
			s.endpoint = "v1/messages"
			if tier == "batch" {
				s.endpoint = "v1/messages/batches"
			}
		case "codex_otel":
			s.endpoint = "v1/responses"
		}
	}
	var reasons pricingReasons
	if !s.singleInvocation {
		reasons.add(reasonInvocation)
	}
	if counts.conflict {
		reasons.add(reasonCounts)
	}
	var modelReason string
	s.model, modelReason = r.resolveModel(s, at, model)
	out.PricedModel = s.model
	reasons.add(modelReason)
	if tier == "" {
		reasons.add(reasonTier)
	}
	if s.model == "" && event.Client == "codex" && ambiguousCodexContext(model, s.context) {
		reasons.add(reasonContext)
	}
	api := r.amount(s, at, counts)
	out.APIEquivalent, out.Components, out.ContextTier, out.Reference = api.amount, api.components, api.context, api.reference
	for _, reason := range api.reasons {
		reasons.add(reason)
	}

	if !counts.conflict && s.singleInvocation {
		if tier != "standard" {
			standard := s
			standard.tier = "standard"
			out.Standard = wholeReference("standard", "standard", "", r.amount(standard, at, counts).amount)
		}
		if out.APIEquivalent.Status != pricingComplete && out.Reference == nil {
			out.Reference = r.invocationReference(s, at, counts, out.RequestedTier, out.Standard, model, reasons)
		}
	}
	if out.APIEquivalent.Status != pricingComplete && out.Reference == nil && !counts.conflict && legacy.CatalogBaseCost != nil {
		out.Reference = &PricingReference{Basis: "legacy_catalog", Currency: pricingCurrency, Amount: legacy.CatalogBaseCost}
	}

	out.Subscription = r.subscription(s, at, evidence, model, attribution, &reasons)
	switch out.Subscription.Channel {
	case "api_key":
		amount := out.APIEquivalent
		out.ProviderEstimate, out.ProviderComponents = &amount, out.Components
	case "third_party":
		if validThirdPartyRoute(attribution) {
			amount, components := scaleEstimate(out.APIEquivalent, out.Components, attribution.multiplier)
			out.ProviderEstimate, out.ProviderComponents = &amount, components
		} else {
			reasons.add(reasonRoute)
		}
	case "enterprise_usd":
		if out.Subscription.Status == "metered" {
			enterprise := subscriptionSelection(s, evidence, model, "enterprise_usd")
			enterprise.model, _ = r.resolveModel(enterprise, at, model)
			result := r.amount(enterprise, at, counts)
			out.ProviderEstimate, out.ProviderComponents = &result.amount, result.components
			for _, reason := range result.reasons {
				reasons.add(reason)
			}
		}
	default:
		if out.Subscription.Channel == "unknown" {
			reasons.add(reasonRoute)
		}
	}
	out.Reasons = []string(reasons)
	return out
}

func ambiguousCodexContext(model string, count *int64) bool {
	return (model == "gpt-5.4" || model == "gpt-5.5") && (count == nil || *count > 272_000)
}

func wholeReference(basis, tier, asOf string, amount PricingAmount) *PricingReference {
	if amount.Status != pricingComplete {
		return nil
	}
	return &PricingReference{Basis: basis, Currency: amount.Currency, Amount: amount.Amount, Tier: tier, AsOf: asOf}
}

func (r invocationRules) invocationReference(s tariffSelection, at time.Time, counts invocationCounts, requested string, standard *PricingReference, model string, reasons pricingReasons) *PricingReference {
	if s.tier == "" {
		if requested != "" {
			assumed := s
			assumed.tier = requested
			if ref := wholeReference("requested_tier", requested, "", r.amount(assumed, at, counts).amount); ref != nil {
				return ref
			}
		}
		return standard
	}
	if r.cutoff.Equal(at) || !slices.Contains(reasons, reasonHistorical) && !slices.Contains(reasons, reasonBoundary) {
		return nil
	}
	s.asOf = true
	s.model, _ = r.resolveModel(s, r.cutoff, model)
	return wholeReference("as_of_date", "", r.cutoff.Format(priceRuleDay), r.amount(s, r.cutoff, counts).amount)
}

func validThirdPartyRoute(a eventAttribution) bool {
	return a.spendEligible && a.quality == "exact" && a.provider != "" && a.provider != "unknown" && a.provider != "official"
}

func evidencedQualifications(raw []string, tier string, subscription bool) []string {
	var result []string
	for _, value := range raw {
		switch value {
		case "claude_context_1m_beta", "background_2h_eligible", "background_8h_eligible":
			if !subscription {
				result = append(result, value)
			}
		case "chatgpt_sign_in", "enterprise_usd_codex_chatgpt_sign_in":
			if subscription {
				result = append(result, value)
			}
		case "chatgpt_sign_in_included_usage_ultrafast_eligible":
			if subscription {
				result = append(result, value, "chatgpt_sign_in_included_usage", "chatgpt_sign_in")
			}
		case "chatgpt_sign_in_included_usage":
			if subscription {
				result = append(result, value, "chatgpt_sign_in")
			}
		}
	}
	if tier == "ultrafast" && !subscription {
		result = append(result, "actual_ultrafast")
	}
	return result
}

func subscriptionSelection(api tariffSelection, evidence RequestPricingEvidence, model, channel string) tariffSelection {
	return tariffSelection{
		provider: api.provider, channel: channel, model: model, endpoint: evidence.ChannelEndpoint,
		tier: api.tier, region: api.region, activity: api.activity,
		eligibility: evidencedQualifications(evidence.Eligibility, api.tier, true),
		context:     api.context, singleInvocation: api.singleInvocation,
	}
}

func (r invocationRules) subscription(api tariffSelection, at time.Time, evidence RequestPricingEvidence, model string, attribution eventAttribution, reasons *pricingReasons) PricingSubscription {
	out := PricingSubscription{
		Channel: "unknown", Status: "unknown", Activity: "unknown", QuotaEligibility: "unknown",
		QuotaReason: "policy_unverified", SubscriptionLongContext: "unknown",
	}
	if api.activity != "" {
		out.Activity = evidence.Activity
	} else if autoReviewModel(model) {
		out.QuotaReason = "activity_unverified"
		reasons.add(reasonActivity)
	}

	switch evidence.Channel {
	case "api_key", "third_party", "personal_subscription", "enterprise_usd":
		out.Channel = evidence.Channel
	case "":
		if validThirdPartyRoute(attribution) {
			out.Channel = "third_party"
		}
	}
	if out.Channel == "api_key" || out.Channel == "third_party" {
		out.Status, out.QuotaEligibility, out.QuotaReason = "metered", "excluded", "paid_channel"
		out.SubscriptionLongContext = "not_applicable"
		return out
	}
	if out.Channel == "unknown" {
		return out
	}

	channel := "chatgpt_subscription"
	if out.Channel == "enterprise_usd" {
		channel = "enterprise_usd"
	}
	s := subscriptionSelection(api, evidence, model, channel)
	if !s.singleInvocation {
		return out
	}
	if out.Activity == "guardian_approval" {
		free := r.selectTerm(s, at, tariffField{policy: "free_activity"})
		if free.rule != nil {
			out.Status, out.QuotaEligibility, out.QuotaReason = "free_activity", "excluded", "guardian_free"
			out.SubscriptionLongContext, out.PolicyID = "not_applicable", free.rule.Policy.ID
		} else {
			reasons.add(free.reason)
		}
		return out
	}

	context := r.selectTerm(s, at, tariffField{policy: "no_context_surcharge"})
	if context.rule != nil {
		out.SubscriptionLongContext, out.PolicyID = "exempt", context.rule.Policy.ID
	} else {
		reasons.add(context.reason)
	}

	if out.Channel == "enterprise_usd" {
		if slices.Contains(s.eligibility, "enterprise_usd_codex_chatgpt_sign_in") && s.endpoint == "codex" && out.Activity != "unknown" {
			out.Status, out.QuotaEligibility, out.QuotaReason = "metered", "excluded", "paid_channel"
		}
		return out
	}
	if s.endpoint != "codex" && s.endpoint != "chatgpt_work" {
		return out
	}
	if slices.Contains(s.eligibility, "chatgpt_sign_in_included_usage") && out.Activity != "unknown" {
		out.Status, out.QuotaEligibility, out.QuotaReason = "included", "eligible", "included_observed"
		out.SpeedPolicy = tariffPolicy(r.selectTerm(s, at, tariffField{policy: "speed_multiplier"}))
	}
	return out
}
