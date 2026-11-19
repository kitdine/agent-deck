package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"maps"
	"regexp"

	"github.com/kitdine/agent-deck/internal/store"
)

// RequestFacts holds only fields observed on an admitted request. A nil count
// means the producer did not establish it. In particular, zero in the legacy
// token columns does not establish that the corresponding field was present.
type RequestFacts struct {
	Source          string `json:"source"`
	ProducerVersion string `json:"producer_version,omitempty"`
	ObservedAt      string `json:"observed_at"`
	Association     string `json:"association"`

	InputTokens      *int64 `json:"input_tokens,omitempty"`
	CachedReadTokens *int64 `json:"cached_read_tokens,omitempty"`
	CacheWriteTokens *int64 `json:"cache_write_tokens,omitempty"`
	OutputTokens     *int64 `json:"output_tokens,omitempty"`
	CacheWrite5m     *int64 `json:"cache_write_5m_tokens,omitempty"`
	CacheWrite1h     *int64 `json:"cache_write_1h_tokens,omitempty"`
	CacheWrite30m    *int64 `json:"cache_write_30m_tokens,omitempty"`
	CacheWrite12h    *int64 `json:"cache_write_12h_tokens,omitempty"`
	ExecutedTier     string `json:"executed_tier,omitempty"`
}

var producerVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+](?:alpha|beta|rc|dev)(?:[.-]?[0-9]+)?)?$`)

func requestProducerVersion(raw string) string {
	if len(raw) <= 64 && producerVersionPattern.MatchString(raw) {
		return raw
	}
	return ""
}

func requestCount(values map[string]any, key string) *int64 {
	value, ok := values[key].(float64)
	if !ok || value < 0 || value >= 1<<53 || value != float64(int64(value)) {
		return nil
	}
	count := int64(value)
	return &count
}

func codexRequestFacts(last map[string]any, admitted map[string]int64, at, version string) *RequestFacts {
	counts, valid := tokenUsage(last)
	// A cumulative delta may span multiple requests, or be smaller than
	// last_token_usage. In either case the last request cannot label the delta.
	if !valid || !maps.Equal(counts, admitted) {
		return nil
	}
	inputDetails, _ := last["input_tokens_details"].(map[string]any)
	writeDetails, _ := inputDetails["cache_write_tokens_details"].(map[string]any)
	return &RequestFacts{
		Source: "codex_jsonl", ProducerVersion: version, ObservedAt: at, Association: "last_usage_matches_admitted_counts",
		InputTokens: requestCount(last, "input_tokens"), CachedReadTokens: requestCount(last, "cached_input_tokens"),
		CacheWriteTokens: requestCount(last, "cache_write_input_tokens"), OutputTokens: requestCount(last, "output_tokens"),
		CacheWrite30m: requestCount(writeDetails, "ttl_30m_tokens"), CacheWrite12h: requestCount(writeDetails, "ttl_12h_tokens"),
	}
}

func claudeRequestFacts(usage, creation map[string]any, at, version string) *RequestFacts {
	facts := &RequestFacts{
		Source: "claude_jsonl", ProducerVersion: requestProducerVersion(version), ObservedAt: at, Association: "assistant_message_usage",
		InputTokens: requestCount(usage, "input_tokens"), CachedReadTokens: requestCount(usage, "cache_read_input_tokens"),
		CacheWriteTokens: requestCount(usage, "cache_creation_input_tokens"), OutputTokens: requestCount(usage, "output_tokens"),
		CacheWrite5m: requestCount(creation, "ephemeral_5m_input_tokens"), CacheWrite1h: requestCount(creation, "ephemeral_1h_input_tokens"),
	}
	// Anthropic's response usage reports processing speed independently of
	// priority capacity. Only the speed field proves Fast or Standard.
	switch stringValue(usage, "speed") {
	case "fast", "standard":
		facts.ExecutedTier = stringValue(usage, "speed")
	case "":
		if _, present := usage["speed"]; !present && stringValue(usage, "service_tier") == "batch" {
			facts.ExecutedTier = "batch"
		}
	}
	return facts
}

func encodeRequestFacts(event Event) (sql.NullString, error) {
	if event.Facts == nil {
		return sql.NullString{}, nil
	}
	facts := *event.Facts
	facts.ObservedAt = event.EventAt
	raw, err := json.Marshal(facts)
	return sql.NullString{String: string(raw), Valid: err == nil}, err
}

func (f *RequestFacts) Scan(value any) error {
	if value == nil {
		*f = RequestFacts{}
		return nil
	}
	var raw sql.NullString
	if err := raw.Scan(value); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(raw.String), f); err != nil {
		return err
	}
	f.ProducerVersion = requestProducerVersion(f.ProducerVersion)
	return nil
}

func (s *Service) hasPricingFactsStorage(ctx context.Context) (bool, error) {
	if !s.Store.ReadOnly() {
		return true, nil
	}
	version, err := s.Store.SchemaVersion(ctx)
	return version >= store.PricingFactsSchemaVersion, err
}

// OTel already durably stores field presence and the one-to-one contribution
// link. Read evidence from that record only while it owns this exact event.
// JSONL facts are stored independently and follow the existing event winner.
const requestFactsSelection = `COALESCE(f.facts_json, CASE WHEN o.state='otel_owned' AND o.metered=1
AND o.event_kind='response.completed' AND o.input_tokens IS NOT NULL AND o.output_tokens IS NOT NULL
AND e.event_key='otel:'||o.observation_key AND e.source_path=e.event_key
AND e.client='codex' AND e.session_id=o.thread_id AND e.model=o.model
AND e.event_at=COALESCE(NULLIF(o.record_at,''),o.received_at)
AND o.input_tokens=e.input_tokens AND COALESCE(o.cached_tokens,0)=e.cached_input_tokens
AND o.output_tokens=e.output_tokens AND COALESCE(o.cache_write_tokens,0)=e.cache_write_tokens
THEN json_object('source','codex_otel','producer_version',o.producer_version,
'observed_at',COALESCE(NULLIF(o.observed_at,''),o.received_at),'association','owned_telemetry_contribution',
'input_tokens',o.input_tokens,'cached_read_tokens',o.cached_tokens,
'output_tokens',o.output_tokens,'cache_write_tokens',o.cache_write_tokens) END)`

const requestFactsJoins = ` LEFT JOIN usage_request_facts f ON f.event_key=e.event_key
LEFT JOIN telemetry_contributions c ON c.event_key=e.event_key AND substr(e.source_path,1,5)='otel:'
LEFT JOIN telemetry_observations o ON o.observation_key=c.observation_key`
