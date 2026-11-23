package store

import (
	"encoding/json"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
)

// These are the lexical fields approved for the private workset. Thread,
// turn, call, kind, and byte evidence live in SQL columns for indexed reads.
type cjsiCheckpointMetadata struct {
	At           string                `json:"at,omitempty"`
	HeadAt       string                `json:"head_at,omitempty"`
	HeadID       string                `json:"head_id,omitempty"`
	LegacyID     string                `json:"legacy_id,omitempty"`
	RootTurnID   string                `json:"root_turn_id,omitempty"`
	ResponseID   string                `json:"response_id,omitempty"`
	ParentID     string                `json:"parent_id,omitempty"`
	ForkedFromID string                `json:"forked_from_id,omitempty"`
	Model        string                `json:"model,omitempty"`
	Tool         string                `json:"tool,omitempty"`
	HasHeadID    bool                  `json:"has_head_id,omitempty"`
	BadIdentity  bool                  `json:"bad_identity,omitempty"`
	BadThread    bool                  `json:"bad_thread,omitempty"`
	BadLegacyID  bool                  `json:"bad_legacy_id,omitempty"`
	MixedSource  bool                  `json:"mixed_source,omitempty"`
	UserMessage  bool                  `json:"user_message,omitempty"`
	VisibleKind  string                `json:"visible_kind,omitempty"`
	Usage        *cjsiCheckpointCounts `json:"usage,omitempty"`
	TurnTotal    *cjsiCheckpointCounts `json:"turn_total,omitempty"`
	ThreadTotal  *cjsiCheckpointCounts `json:"thread_total,omitempty"`
	LegacyLast   *cjsiCheckpointCounts `json:"legacy_last,omitempty"`
	LegacyTotal  *cjsiCheckpointCounts `json:"legacy_total,omitempty"`
}

type cjsiCheckpointCounts struct {
	State         string               `json:"state"`
	Input         *cjsiCheckpointCount `json:"input,omitempty"`
	Output        *cjsiCheckpointCount `json:"output,omitempty"`
	Cached        *cjsiCheckpointCount `json:"cached,omitempty"`
	CacheWrite    *cjsiCheckpointCount `json:"cache_write,omitempty"`
	Reasoning     *cjsiCheckpointCount `json:"reasoning,omitempty"`
	ReportedTotal *cjsiCheckpointCount `json:"reported_total,omitempty"`
}

type cjsiCheckpointCount struct {
	State string `json:"state"`
	Value int64  `json:"value"`
}

var cjsiCheckpointKinds = map[ingest.CodexRecordKind]string{
	ingest.CodexIgnored: "ignored", ingest.CodexMalformed: "malformed",
	ingest.CodexSessionMeta: "session_meta", ingest.CodexTurnContext: "turn_context",
	ingest.CodexTurnStart: "turn_start", ingest.CodexTurnEnd: "turn_end",
	ingest.CodexMessage: "message", ingest.CodexToolStart: "tool_start", ingest.CodexToolEnd: "tool_end",
	ingest.CodexRequestUsage: "request_usage", ingest.CodexLegacyUsage: "legacy_usage",
	ingest.CodexBoundary: "boundary", ingest.CodexOversize: "oversize",
}

func encodeCJSICheckpointRecord(r ingest.CodexRecord) (string, string, error) {
	kind, ok := cjsiCheckpointKinds[r.Kind]
	if !ok {
		return "", "", errCJSICheckpointWorkset
	}
	if r.Kind == ingest.CodexOversize && r != (ingest.CodexRecord{Span: r.Span, Kind: ingest.CodexOversize}) {
		return "", "", errCJSICheckpointWorkset
	}
	if ingest.CodexLegacySessionID(r.LegacyID) != r.LegacyID {
		return "", "", errCJSICheckpointWorkset
	}
	for _, id := range []string{r.HeadID, r.ThreadID, r.TurnID, r.RootTurnID, r.ResponseID, r.ParentID, r.ForkedFromID, r.CallID} {
		if !validCJSICheckpointID(id) {
			return "", "", errCJSICheckpointWorkset
		}
	}
	if ingest.CodexModelLabel(r.Model) != r.Model || r.Tool != "" && ingest.CodexToolLabel(r.Tool) != r.Tool {
		return "", "", errCJSICheckpointWorkset
	}
	meta := cjsiCheckpointMetadata{
		At: cjsiCheckpointTime(r.At), HeadAt: cjsiCheckpointTime(r.HeadAt),
		HeadID: r.HeadID, LegacyID: r.LegacyID, RootTurnID: r.RootTurnID, ResponseID: r.ResponseID,
		ParentID: r.ParentID, ForkedFromID: r.ForkedFromID, Model: r.Model, Tool: r.Tool,
		HasHeadID: r.HasHeadID, BadIdentity: r.BadIdentity, BadThread: r.BadThread, BadLegacyID: r.BadLegacyID,
		MixedSource: r.MixedSource, UserMessage: r.UserMessage,
	}
	switch r.VisibleKind {
	case ingest.CodexNoDocument:
	case ingest.CodexUserPrompt:
		meta.VisibleKind = "user_prompt"
	case ingest.CodexAssistantFinal:
		meta.VisibleKind = "assistant_final"
	default:
		return "", "", errCJSICheckpointWorkset
	}
	fields := []struct {
		input ingest.CodexCounts
		out   **cjsiCheckpointCounts
	}{
		{r.Usage, &meta.Usage}, {r.TurnTotal, &meta.TurnTotal}, {r.ThreadTotal, &meta.ThreadTotal},
		{r.LegacyLast, &meta.LegacyLast}, {r.LegacyTotal, &meta.LegacyTotal},
	}
	for _, field := range fields {
		value, ok := encodeCJSICheckpointCounts(field.input)
		if !ok {
			return "", "", errCJSICheckpointWorkset
		}
		*field.out = value
	}
	encoded, err := json.Marshal(meta)
	return kind, string(encoded), err
}

func validCJSICheckpointID(id string) bool {
	if len(id) > 128 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '-' || c == '_' || c == '.' || c == ':') {
			return false
		}
	}
	return true
}

func cjsiCheckpointTime(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}

func encodeCJSICheckpointCounts(input ingest.CodexCounts) (*cjsiCheckpointCounts, bool) {
	if input.State == ingest.CodexAbsent {
		return nil, input == (ingest.CodexCounts{})
	}
	out := &cjsiCheckpointCounts{State: cjsiCheckpointCountState(input.State)}
	if out.State == "" {
		return nil, false
	}
	fields := []struct {
		input ingest.CodexCount
		out   **cjsiCheckpointCount
	}{
		{input.Input, &out.Input}, {input.Output, &out.Output}, {input.Cached, &out.Cached},
		{input.CacheWrite, &out.CacheWrite}, {input.Reasoning, &out.Reasoning}, {input.ReportedTotal, &out.ReportedTotal},
	}
	for _, field := range fields {
		state := cjsiCheckpointCountState(field.input.State)
		if field.input.State == ingest.CodexAbsent || field.input.State == ingest.CodexInvalid {
			if field.input.Value != 0 {
				return nil, false
			}
		}
		if field.input.State == ingest.CodexAbsent {
			continue
		}
		if state == "" || field.input.Value < 0 {
			return nil, false
		}
		*field.out = &cjsiCheckpointCount{State: state, Value: field.input.Value}
	}
	return out, true
}

func cjsiCheckpointCountState(state ingest.CodexFieldState) string {
	switch state {
	case ingest.CodexValid:
		return "valid"
	case ingest.CodexInvalid:
		return "invalid"
	default:
		return ""
	}
}

// decodeCJSICheckpointRecord accepts only the canonical representation emitted
// by the writer. Round-tripping rejects unknown or repeated JSON fields,
// noncanonical time/count encodings, and unapproved indexed values.
func decodeCJSICheckpointRecord(span ingest.CodexSpan, kind, threadID, turnID, callID, encoded string) (ingest.CodexRecord, error) {
	var r ingest.CodexRecord
	var meta cjsiCheckpointMetadata
	if err := json.Unmarshal([]byte(encoded), &meta); err != nil {
		return r, errCJSICheckpointWorkset
	}
	r = ingest.CodexRecord{
		Span: span, ThreadID: threadID, TurnID: turnID, CallID: callID,
		HeadID: meta.HeadID, LegacyID: meta.LegacyID, RootTurnID: meta.RootTurnID, ResponseID: meta.ResponseID,
		ParentID: meta.ParentID, ForkedFromID: meta.ForkedFromID, Model: meta.Model, Tool: meta.Tool,
		HasHeadID: meta.HasHeadID, BadIdentity: meta.BadIdentity, BadThread: meta.BadThread,
		BadLegacyID: meta.BadLegacyID, MixedSource: meta.MixedSource, UserMessage: meta.UserMessage,
	}
	found := false
	for value, label := range cjsiCheckpointKinds {
		if kind == label {
			r.Kind, found = value, true
			break
		}
	}
	if !found {
		return ingest.CodexRecord{}, errCJSICheckpointWorkset
	}
	switch meta.VisibleKind {
	case "":
	case "user_prompt":
		r.VisibleKind = ingest.CodexUserPrompt
	case "assistant_final":
		r.VisibleKind = ingest.CodexAssistantFinal
	default:
		return ingest.CodexRecord{}, errCJSICheckpointWorkset
	}
	for _, field := range []struct {
		value string
		out   *time.Time
	}{{meta.At, &r.At}, {meta.HeadAt, &r.HeadAt}} {
		if field.value == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, field.value)
		if err != nil {
			return ingest.CodexRecord{}, errCJSICheckpointWorkset
		}
		*field.out = at.UTC()
	}
	for _, field := range []struct {
		value *cjsiCheckpointCounts
		out   *ingest.CodexCounts
	}{
		{meta.Usage, &r.Usage}, {meta.TurnTotal, &r.TurnTotal}, {meta.ThreadTotal, &r.ThreadTotal},
		{meta.LegacyLast, &r.LegacyLast}, {meta.LegacyTotal, &r.LegacyTotal},
	} {
		if field.value != nil {
			*field.out = decodeCJSICheckpointCounts(*field.value)
		}
	}
	if label, canonical, err := encodeCJSICheckpointRecord(r); err != nil || label != kind || canonical != encoded {
		return ingest.CodexRecord{}, errCJSICheckpointWorkset
	}
	return r, nil
}

func decodeCJSICheckpointCounts(input cjsiCheckpointCounts) ingest.CodexCounts {
	out := ingest.CodexCounts{State: decodeCJSICheckpointCountState(input.State)}
	for _, field := range []struct {
		value *cjsiCheckpointCount
		out   *ingest.CodexCount
	}{
		{input.Input, &out.Input}, {input.Output, &out.Output}, {input.Cached, &out.Cached},
		{input.CacheWrite, &out.CacheWrite}, {input.Reasoning, &out.Reasoning}, {input.ReportedTotal, &out.ReportedTotal},
	} {
		if field.value != nil {
			*field.out = ingest.CodexCount{State: decodeCJSICheckpointCountState(field.value.State), Value: field.value.Value}
		}
	}
	return out
}

func decodeCJSICheckpointCountState(state string) ingest.CodexFieldState {
	switch state {
	case "valid":
		return ingest.CodexValid
	case "invalid":
		return ingest.CodexInvalid
	default:
		// Unknown states cannot reproduce the original encoding.
		return ingest.CodexAbsent
	}
}
