package telemetry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// Decode budget. A request that exceeds it is malformed, not truncated.
const (
	MaxBodyBytes         = 4 << 20
	maxRecordsPerRequest = 10000
	maxAttributes        = 512
	maxFieldBytes        = 256
)

// ErrMalformed reports a request body that is not a decodable OTLP/HTTP JSON
// logs export within the decode budget.
var ErrMalformed = errors.New("malformed OTLP JSON logs request")

// Record is one allowlisted log record. It is the journal line format and
// holds nothing outside the allowlist. Token counts are nil when absent.
type Record struct {
	Producer         string `json:"producer"`
	ProducerVersion  string `json:"producer_version"`
	Originator       string `json:"originator"`
	EventName        string `json:"event_name"`
	EventKind        string `json:"event_kind"`
	Transport        string `json:"transport"`
	RecordAt         string `json:"record_at"`
	ObservedAt       string `json:"observed_at"`
	Model            string `json:"model"`
	ResponseID       string `json:"response_id"`
	RequestID        string `json:"request_id"`
	AttemptID        string `json:"attempt_id"`
	ThreadID         string `json:"thread_id"`
	SourceClass      string `json:"source_class"`
	InputTokens      *int64 `json:"input_tokens"`
	OutputTokens     *int64 `json:"output_tokens"`
	CachedTokens     *int64 `json:"cached_tokens"`
	CacheWriteTokens *int64 `json:"cache_write_tokens"`
	ReasoningTokens  *int64 `json:"reasoning_tokens"`
	TotalTokens      *int64 `json:"total_tokens"`
	// Invalid names the first reason the record cannot be metered, such as
	// a negative or overflowing count. Invalid records are kept as evidence
	// of delivery but never metered.
	Invalid string `json:"invalid,omitempty"`
}

// Key is the delivery identity: identical allowlisted content replays to the
// same key, so a repeated HTTP delivery or import is a no-op.
func (r Record) Key() string {
	data, _ := json.Marshal(r)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type otlpRequest struct {
	ResourceLogs []struct {
		Resource struct {
			Attributes []otlpAttribute `json:"attributes"`
		} `json:"resource"`
		ScopeLogs []struct {
			LogRecords []struct {
				TimeUnixNano         json.RawMessage `json:"timeUnixNano"`
				ObservedTimeUnixNano json.RawMessage `json:"observedTimeUnixNano"`
				Attributes           []otlpAttribute `json:"attributes"`
				// body and every other field are deliberately not decoded.
			} `json:"logRecords"`
		} `json:"scopeLogs"`
	} `json:"resourceLogs"`
}

type otlpAttribute struct {
	Key   string    `json:"key"`
	Value otlpValue `json:"value"`
}

// otlpValue decodes only scalar values. Arrays, maps and bytes are skipped
// without being materialized, so no free-form content is retained.
type otlpValue struct {
	String *string
	Int    json.RawMessage
	Double json.RawMessage
	Bool   *bool
}

func (v *otlpValue) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if raw, ok := fields["stringValue"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		v.String = &s
	}
	if raw, ok := fields["intValue"]; ok {
		v.Int = raw
	}
	if raw, ok := fields["doubleValue"]; ok {
		v.Double = raw
	}
	if raw, ok := fields["boolValue"]; ok {
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return err
		}
		v.Bool = &b
	}
	return nil
}

// Decode parses an OTLP/HTTP JSON logs export and returns the allowlisted
// records it carries. Records without an event name are ignored: they are not
// producer usage events.
func Decode(body []byte) ([]Record, error) {
	if len(body) > MaxBodyBytes {
		return nil, ErrMalformed
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	var request otlpRequest
	if err := decoder.Decode(&request); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, errorClass(err))
	}
	if decoder.More() {
		return nil, ErrMalformed
	}
	var records []Record
	for _, resourceLogs := range request.ResourceLogs {
		if len(resourceLogs.Resource.Attributes) > maxAttributes {
			return nil, ErrMalformed
		}
		resource := attributeMap(resourceLogs.Resource.Attributes)
		for _, scope := range resourceLogs.ScopeLogs {
			for _, log := range scope.LogRecords {
				if len(records) >= maxRecordsPerRequest || len(log.Attributes) > maxAttributes {
					return nil, ErrMalformed
				}
				attributes := attributeMap(log.Attributes)
				record := Record{}
				// Only the event.name attribute names the event: Codex sets the
				// OTLP eventName field to a tracing target, which is not one.
				record.EventName = attributes.text("event.name")
				if record.EventName == "" {
					continue
				}
				record.Producer = resource.text("service.name")
				record.ProducerVersion = firstNonEmpty(resource.text("service.version"), attributes.text("app.version"))
				record.Originator = attributes.text("originator")
				record.EventKind = attributes.text("event.kind")
				record.Transport = transportOf(record.EventName, attributes.text("transport"))
				record.Model = attributes.text("model")
				record.ResponseID = firstNonEmpty(attributes.text("response.id"), attributes.text("response_id"))
				record.RequestID = firstNonEmpty(attributes.text("request.id"), attributes.text("request_id"))
				record.AttemptID = firstNonEmpty(attributes.text("attempt"), attributes.text("attempt.id"))
				record.ThreadID = firstNonEmpty(attributes.text("conversation.id"), attributes.text("thread.id"))
				record.SourceClass = attributes.text("source.kind")
				record.RecordAt = unixNanoTime(log.TimeUnixNano)
				record.ObservedAt = unixNanoTime(log.ObservedTimeUnixNano)
				for _, field := range []struct {
					key    string
					target **int64
				}{
					{"input_token_count", &record.InputTokens},
					{"output_token_count", &record.OutputTokens},
					{"cached_token_count", &record.CachedTokens},
					{"cache_write_token_count", &record.CacheWriteTokens},
					{"reasoning_token_count", &record.ReasoningTokens},
					{"total_token_count", &record.TotalTokens},
				} {
					value, present, reason := attributes.count(field.key)
					if reason != "" && record.Invalid == "" {
						record.Invalid = reason
					}
					if present {
						v := value
						*field.target = &v
					}
				}
				if reason := attributes.overlong; reason != "" && record.Invalid == "" {
					record.Invalid = reason
				}
				if record.Invalid == "" {
					record.Invalid = componentConflict(record)
				}
				records = append(records, record)
			}
		}
	}
	return records, nil
}

// errorClass keeps decoder errors free of request content.
func errorClass(err error) string {
	var syntax *json.SyntaxError
	var typeError *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return "syntax"
	case errors.As(err, &typeError):
		return "type"
	default:
		return "decode"
	}
}

// Metered reports whether the record is a completed response that carries an
// actual token count. Stream deltas, failures and interruptions are not.
func (r Record) Metered() bool {
	return r.EventKind == "response.completed" && (r.InputTokens != nil || r.OutputTokens != nil)
}

func componentConflict(r Record) string {
	if r.InputTokens != nil && r.CachedTokens != nil && *r.CachedTokens > *r.InputTokens {
		return "cached_exceeds_input"
	}
	if r.OutputTokens != nil && r.ReasoningTokens != nil && *r.ReasoningTokens > *r.OutputTokens {
		return "reasoning_exceeds_output"
	}
	if r.TotalTokens != nil && r.InputTokens != nil && r.OutputTokens != nil {
		if sum := new(big.Int).Add(big.NewInt(*r.InputTokens), big.NewInt(*r.OutputTokens)); sum.Cmp(big.NewInt(*r.TotalTokens)) != 0 {
			return "total_disagrees_with_components"
		}
	}
	return ""
}

func transportOf(eventName, declared string) string {
	switch eventName {
	case "codex.sse_event":
		return "sse"
	case "codex.websocket_event", "codex.websocket_request":
		return "websocket"
	}
	switch declared {
	case "sse", "websocket", "http":
		return declared
	}
	return "unknown"
}

type attributes struct {
	values   map[string]otlpValue
	overlong string
}

// allowedKeys is the complete attribute allowlist. Every other attribute,
// including prompts, tool output, authorization and e-mail, is discarded
// before any value is copied.
var allowedKeys = map[string]bool{
	"service.name": true, "service.version": true, "originator": true,
	"event.name": true, "event.kind": true, "app.version": true, "transport": true,
	"model": true, "response.id": true, "response_id": true, "request.id": true, "request_id": true,
	"attempt": true, "attempt.id": true, "conversation.id": true, "thread.id": true, "source.kind": true,
	"input_token_count": true, "output_token_count": true, "cached_token_count": true,
	"cache_write_token_count": true, "reasoning_token_count": true, "total_token_count": true,
}

func attributeMap(list []otlpAttribute) attributes {
	result := attributes{values: map[string]otlpValue{}}
	for _, attribute := range list {
		if !allowedKeys[attribute.Key] {
			continue
		}
		if attribute.Value.String != nil && len(*attribute.Value.String) > maxFieldBytes {
			result.overlong = "field_too_long"
			continue
		}
		result.values[attribute.Key] = attribute.Value
	}
	return result
}

func (a attributes) text(key string) string {
	value, ok := a.values[key]
	if !ok {
		return ""
	}
	switch {
	case value.String != nil:
		return strings.ToValidUTF8(*value.String, "")
	case value.Int != nil:
		if n, ok := integerText(value.Int); ok {
			return n
		}
	}
	return ""
}

// count decodes an integer token count without passing through float64.
// OTLP JSON encodes int64 as a string or a number; a double that is not an
// exact integer, a negative value or an int64 overflow is invalid.
func (a attributes) count(key string) (int64, bool, string) {
	value, ok := a.values[key]
	if !ok {
		return 0, false, ""
	}
	var text string
	switch {
	case value.Int != nil:
		n, ok := integerText(value.Int)
		if !ok {
			return 0, false, "count_not_integer"
		}
		text = n
	case value.String != nil:
		text = *value.String
	case value.Double != nil:
		n, ok := integerText(value.Double)
		if !ok {
			return 0, false, "count_not_integer"
		}
		text = n
	default:
		return 0, false, "count_not_integer"
	}
	parsed, ok := new(big.Int).SetString(strings.TrimSpace(text), 10)
	if !ok {
		return 0, false, "count_not_integer"
	}
	if parsed.Sign() < 0 {
		return 0, false, "count_negative"
	}
	if !parsed.IsInt64() {
		return 0, false, "count_overflow"
	}
	return parsed.Int64(), true, ""
}

// integerText returns the decimal text of a JSON number or numeric string
// when it denotes an exact integer.
func integerText(raw json.RawMessage) (string, bool) {
	text := strings.TrimSpace(string(raw))
	if strings.HasPrefix(text, `"`) {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", false
		}
		text = s
	}
	if text == "" {
		return "", false
	}
	if _, ok := new(big.Int).SetString(text, 10); ok {
		return text, true
	}
	rat, ok := new(big.Rat).SetString(text)
	if !ok || !rat.IsInt() {
		return "", false
	}
	return rat.Num().String(), true
}

func unixNanoTime(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	text, ok := integerText(raw)
	if !ok {
		return ""
	}
	nanos, err := strconv.ParseInt(text, 10, 64)
	if err != nil || nanos <= 0 {
		return ""
	}
	return time.Unix(0, nanos).UTC().Format(time.RFC3339Nano)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
