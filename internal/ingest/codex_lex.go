package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

// CodexSpan identifies a complete line in one captured source generation.
// End is exclusive and includes the terminating newline.
type CodexSpan struct{ Start, End int64 }

type CodexRecordKind uint8

const (
	CodexIgnored CodexRecordKind = iota
	CodexMalformed
	CodexSessionMeta
	CodexTurnContext
	CodexTurnStart
	CodexTurnEnd
	CodexMessage
	CodexToolStart
	CodexToolEnd
	CodexRequestUsage
	CodexLegacyUsage
	CodexBoundary
	CodexOversize
)

func (kind CodexRecordKind) isGap() bool {
	return kind == CodexMalformed || kind == CodexBoundary || kind == CodexOversize
}

type CodexFieldState uint8

const (
	CodexAbsent CodexFieldState = iota
	CodexValid
	CodexInvalid
)

type CodexCount struct {
	State CodexFieldState
	Value int64
}

type CodexCounts struct {
	State                             CodexFieldState
	Input, Output, Cached, CacheWrite CodexCount
	Reasoning, ReportedTotal          CodexCount
}

// Total returns a checked primary sum. Auxiliary counts never add to it.
func (c CodexCounts) Total() (int64, bool) {
	if c.State != CodexValid || c.Input.State != CodexValid || c.Output.State != CodexValid ||
		c.Input.Value > math.MaxInt64-c.Output.Value {
		return 0, false
	}
	return c.Input.Value + c.Output.Value, true
}

// CodexRecord contains normalized facts, never document text or tool payloads.
// Single records are lexical observations and establish no publication authority.
type CodexRecord struct {
	Span                          CodexSpan
	Kind                          CodexRecordKind
	At, HeadAt                    time.Time
	HeadID, LegacyID              string
	ThreadID, TurnID              string
	RootTurnID, ResponseID        string
	ParentID, ForkedFromID        string
	Model, CallID                 string
	Tool                          string
	HasHeadID, BadIdentity        bool
	BadThread, BadLegacyID        bool
	MixedSource, UserMessage      bool
	VisibleKind                   CodexVisibleKind
	Usage, TurnTotal, ThreadTotal CodexCounts
	LegacyLast, LegacyTotal       CodexCounts
}

type CodexVisibleKind uint8

const (
	CodexNoDocument CodexVisibleKind = iota
	CodexUserPrompt
	CodexAssistantFinal
)

// CodexVisible is transient index-only material. It is not in the proof or
// checkpoint. Consumers must join by Span and require a local resolved record.
type CodexVisible struct {
	Span CodexSpan
	Kind CodexVisibleKind
	Text string
}

type CodexReadProgress struct {
	ReadEnd     int64
	CompleteEnd int64
	Partial     bool
}

type CodexCaptureBounds struct {
	Start, End   int64
	MaxLineBytes int
}

var errCodexRange = errors.New("invalid Codex capture range")

// ReadCodexJSONL reads from a verified line boundary (Start) up to End,
// exclusively. MaxLineBytes includes the newline. ReadEnd counts consumed
// bytes, including a rejected callback's line but excluding unconsumed prefetch.
// Only complete lines accepted by visit advance CompleteEnd. Resume there
// with a verified prefix. Use includeVisible only for a document consumer.
func ReadCodexJSONL(ctx context.Context, r io.Reader, bounds CodexCaptureBounds, includeVisible bool, visit func(CodexRecord, *CodexVisible) error) (CodexReadProgress, error) {
	progress := CodexReadProgress{ReadEnd: bounds.Start, CompleteEnd: bounds.Start}
	if bounds.Start < 0 || bounds.End < bounds.Start || bounds.MaxLineBytes <= 0 {
		return progress, errCodexRange
	}
	if err := ctx.Err(); err != nil {
		return progress, err
	}
	var buffer [4096]byte
	var line []byte
	oversize, emptyReads := false, 0
	for progress.ReadEnd < bounds.End {
		if err := ctx.Err(); err != nil {
			return progress, err
		}
		size := min(int64(len(buffer)), int64(bounds.MaxLineBytes), bounds.End-progress.ReadEnd)
		n, readErr := r.Read(buffer[:size])
		if n == 0 && readErr == nil {
			emptyReads++
			if emptyReads == 100 {
				return progress, io.ErrNoProgress
			}
			continue
		}
		emptyReads = 0
		for remaining := buffer[:n]; len(remaining) > 0; {
			if err := ctx.Err(); err != nil {
				return progress, err
			}
			newline := bytes.IndexByte(remaining, '\n')
			size := len(remaining)
			if newline >= 0 {
				size = newline + 1
			}
			fragment := remaining[:size]
			remaining = remaining[size:]
			progress.ReadEnd += int64(size)
			progress.Partial = newline < 0
			if !oversize && size > bounds.MaxLineBytes-len(line) {
				oversize, line = true, nil
			}
			if newline < 0 {
				if !oversize {
					line = appendCodexFragment(line, fragment, bounds.MaxLineBytes)
				}
				continue
			}

			span := CodexSpan{Start: progress.CompleteEnd, End: progress.ReadEnd}
			record := CodexRecord{Kind: CodexOversize, Span: span}
			var visible *CodexVisible
			if !oversize {
				if len(line) != 0 {
					fragment = appendCodexFragment(line, fragment, bounds.MaxLineBytes)
				}
				record, visible = lexCodex(fragment, span, includeVisible)
			}
			if err := visit(record, visible); err != nil {
				return progress, err
			}
			progress.CompleteEnd = span.End
			line, oversize = nil, false
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return progress, readErr
		}
	}
	return progress, ctx.Err()
}

// Grow the retained line without append allocating beyond the caller's cap.
func appendCodexFragment(line, fragment []byte, limit int) []byte {
	if need := len(line) + len(fragment); need > cap(line) {
		capacity := max(need, cap(line)+min(cap(line), limit-cap(line)))
		grown := make([]byte, len(line), capacity)
		copy(grown, line)
		line = grown
	}
	return append(line, fragment...)
}

type codexObject map[string]json.RawMessage

func (o codexObject) field(key string) string {
	var value string
	_ = json.Unmarshal(o[key], &value)
	return value
}

func (o codexObject) object(key string) codexObject {
	var value codexObject
	_ = json.Unmarshal(o[key], &value)
	return value
}

func codexID(value string) string {
	if len(value) == 0 || len(value) > 128 {
		return ""
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '-' || c == '_' || c == '.' || c == ':' || c == '/') {
			return ""
		}
	}
	// Reject path-like data; this is an identifier, never a tool target.
	if value[0] == '/' || strings.Contains(value, "../") || strings.Contains(value, "//") {
		return ""
	}
	return value
}

func codexEventID(value string) string {
	if strings.ContainsRune(value, '/') {
		return ""
	}
	return codexID(value)
}

// CodexModelLabel accepts bare names and provider-qualified names.
func CodexModelLabel(value string) string {
	label := codexID(value)
	provider, model, qualified := strings.Cut(label, "/")
	if qualified && (model == "" || strings.HasPrefix(provider, ".") || strings.HasPrefix(model, ".") ||
		strings.ContainsRune(provider, ':') || strings.ContainsRune(model, '/')) {
		return ""
	}
	return label
}

func codexUUID(s string) string {
	if len(s) != 36 {
		return ""
	}
	for i, c := range []byte(s) {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return ""
			}
		default:
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return ""
			}
		}
	}
	return strings.ToLower(s)
}

// CodexLegacySessionID normalizes the session_id and sessionId aliases.
func CodexLegacySessionID(s string) string {
	if id := codexUUID(s); id != "" {
		return id
	}
	return codexID(s)
}

func codexTime(values ...string) time.Time {
	for _, value := range values {
		if at, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return at.UTC()
		}
	}
	return time.Time{}
}

// codexAliases preserves absence and rejects contradictory or malformed
// identifiers across the approved positions.
func codexAliases(key string, normalize func(string) string, objects ...codexObject) (string, bool) {
	var found string
	for _, object := range objects {
		if _, present := object[key]; !present {
			continue
		}
		id := normalize(object.field(key))
		if id == "" || found != "" && found != id {
			return "", true
		}
		found = id
	}
	return found, false
}

func lexCodex(line []byte, span CodexSpan, includeVisible bool) (CodexRecord, *CodexVisible) {
	var envelope codexObject
	record := CodexRecord{Span: span}
	if len(bytes.TrimSpace(line)) == 0 {
		return record, nil
	}
	if json.Unmarshal(line, &envelope) != nil || envelope == nil || envelope.field("type") == "" {
		record.Kind = CodexMalformed
		return record, nil
	}
	payload := envelope.object("payload")
	item := payload.object("item")
	if item == nil {
		item = payload
	}
	record.At = codexTime(envelope.field("timestamp"))
	typ, event := envelope.field("type"), payload.field("type")
	if _, supplied := envelope["payload"]; payload == nil && (supplied || typ == "session_meta") {
		record.Kind = CodexMalformed
		return record, nil
	}
	switch typ {
	case "session_meta":
		record.Kind = CodexSessionMeta
		readCodexHead(&record, payload)
	case "turn_context":
		record.Kind = CodexTurnContext
		record.Model = CodexModelLabel(payload.field("model"))
	case "token_usage_record":
		record.Kind = CodexRequestUsage
		record.ResponseID = codexEventID(payload.field("response_id"))
		record.Usage = readCodexCounts(payload["usage"])
		record.TurnTotal = readCodexCounts(payload["turn_token_usage"])
		record.ThreadTotal = readCodexCounts(payload["thread_token_usage"])
	case "compacted":
		record.Kind = CodexBoundary
	case "event_msg":
		switch event {
		case "token_count":
			record.Kind = CodexLegacyUsage
			info := payload.object("info")
			record.LegacyLast = readCodexCounts(info["last_token_usage"])
			record.LegacyTotal = readCodexCounts(info["total_token_usage"])
		case "task_started", "turn_started":
			record.Kind = CodexTurnStart
		case "task_complete":
			record.Kind = CodexTurnEnd
		case "turn_aborted":
			record.Kind = CodexBoundary
		case "user_message", "agent_message":
			record.Kind = CodexMessage
			record.UserMessage = event == "user_message"
		default:
			readCodexTool(&record, item)
		}
	case "response_item":
		if item.field("type") == "message" {
			record.Kind = CodexMessage
		} else {
			readCodexTool(&record, item)
		}
	}
	record.ThreadID, record.BadThread = codexAliases("thread_id", codexUUID, envelope, payload, item)
	var badTurn, badLegacy, badCamel bool
	record.TurnID, badTurn = codexAliases("turn_id", codexEventID, envelope, payload, item)
	record.RootTurnID = codexEventID(payload.field("root_turn_id"))
	record.LegacyID, badLegacy = codexAliases("session_id", CodexLegacySessionID, envelope, payload)
	camel, badCamel := codexAliases("sessionId", CodexLegacySessionID, envelope)
	record.BadIdentity = badTurn
	record.BadLegacyID = badLegacy || badCamel ||
		record.LegacyID != "" && camel != "" && record.LegacyID != camel
	if record.LegacyID == "" {
		record.LegacyID = camel
	}
	if typ != "response_item" || record.Kind != CodexMessage {
		return record, nil
	}
	kind, text := readCodexVisible(payload, item, includeVisible)
	record.VisibleKind = kind
	record.UserMessage = kind == CodexUserPrompt
	if !includeVisible || kind == CodexNoDocument {
		return record, nil
	}
	return record, &CodexVisible{Span: span, Kind: kind, Text: text}
}

func readCodexHead(record *CodexRecord, payload codexObject) {
	_, record.HasHeadID = payload["id"]
	record.HeadID = codexUUID(payload.field("id"))
	record.HeadAt = codexTime(payload.field("timestamp"))
	record.ForkedFromID = codexUUID(payload.field("forked_from_id"))
	source := payload.object("source")
	subagent := source.object("subagent")
	spawn := subagent.object("thread_spawn")
	_, nativeSubagent := source["subagent"]
	fork := len(payload["forked_from_id"]) != 0 && string(payload["forked_from_id"]) != "null"
	parent, badParent := codexAliases("parent_thread_id", codexUUID, payload, spawn)
	record.ParentID = parent
	record.MixedSource = nativeSubagent || fork || badParent || parent != ""
}

func readCodexTool(record *CodexRecord, item codexObject) {
	kind := item.field("type")
	switch kind {
	case "function_call", "custom_tool_call", "mcp_tool_call", "web_search_call", "computer_call":
		record.Kind = CodexToolStart
		for _, key := range []string{"name", "tool_name", "tool"} {
			if name := item.field(key); name != "" {
				record.Tool = CodexToolLabel(name)
				break
			}
		}
		if record.Tool == "" {
			switch kind {
			case "mcp_tool_call":
				record.Tool = "mcp"
			case "web_search_call":
				record.Tool = "web_search"
			case "computer_call":
				record.Tool = "computer"
			default:
				record.Tool = "other"
			}
		}
	case "function_call_output", "custom_tool_call_output", "mcp_tool_call_output", "web_search_call_output", "computer_call_output":
		record.Kind = CodexToolEnd
	default:
		return
	}
	if _, present := item["call_id"]; present {
		record.CallID = codexEventID(item.field("call_id"))
	} else {
		record.CallID = codexEventID(item.field("id"))
	}
}

// CodexToolLabel retains only known display labels, never arbitrary tool names.
func CodexToolLabel(name string) string {
	name = strings.TrimSpace(name)
	switch name {
	case "apply_patch", "exec_command", "exec", "js", "write_stdin",
		"wait", "wait_agent", "spawn_agent", "list_agents", "interrupt_agent",
		"send_message", "followup_task", "update_plan", "view_image",
		"mcp", "web_search", "computer":
		return name
	default:
		return "other"
	}
}

func readCodexVisible(payload, item codexObject, includeText bool) (CodexVisibleKind, string) {
	kind, contentType := CodexUserPrompt, "input_text"
	switch item.field("role") {
	case "user":
	case "assistant":
		kind, contentType = CodexAssistantFinal, "output_text"
		for _, object := range []codexObject{payload, item} {
			if _, present := object["phase"]; present && object.field("phase") != "final_answer" {
				return CodexNoDocument, ""
			}
		}
	default:
		return CodexNoDocument, ""
	}
	var single string
	if kind == CodexUserPrompt && json.Unmarshal(item["content"], &single) == nil && strings.TrimSpace(single) != "" {
		if !includeText {
			single = ""
		}
		return kind, strings.TrimSpace(single)
	}
	var parts []codexObject
	if json.Unmarshal(item["content"], &parts) != nil || len(parts) == 0 {
		return CodexNoDocument, ""
	}
	var joined strings.Builder
	nonblank := false
	for i, part := range parts {
		text := part.field("text")
		if part.field("type") != contentType || text == "" {
			return CodexNoDocument, ""
		}
		nonblank = nonblank || strings.TrimSpace(text) != ""
		if includeText {
			if i != 0 {
				joined.WriteByte('\n')
			}
			joined.WriteString(text)
		}
	}
	if !nonblank {
		return CodexNoDocument, ""
	}
	return kind, strings.TrimSpace(joined.String())
}

func readCodexCounts(raw json.RawMessage) CodexCounts {
	if len(raw) == 0 {
		return CodexCounts{}
	}
	var object codexObject
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return CodexCounts{State: CodexInvalid}
	}
	counts := CodexCounts{
		State: CodexValid,
		Input: readCodexCount(object["input_tokens"]), Output: readCodexCount(object["output_tokens"]),
		Cached: readCodexCount(object["cached_input_tokens"]), CacheWrite: readCodexCount(object["cache_write_input_tokens"]),
		Reasoning: readCodexCount(object["reasoning_output_tokens"]), ReportedTotal: readCodexCount(object["total_tokens"]),
	}
	for _, count := range []CodexCount{counts.Input, counts.Output, counts.Cached, counts.CacheWrite, counts.Reasoning, counts.ReportedTotal} {
		if count.State == CodexInvalid {
			counts.State = CodexInvalid
		}
	}
	if counts.Input.State == CodexValid && (counts.Cached.Value > counts.Input.Value ||
		counts.Output.State == CodexValid && counts.Input.Value > math.MaxInt64-counts.Output.Value) ||
		counts.Output.State == CodexValid && counts.Reasoning.Value > counts.Output.Value {
		counts.State = CodexInvalid
	}
	return counts
}

func readCodexCount(raw json.RawMessage) CodexCount {
	if len(raw) == 0 {
		return CodexCount{}
	}
	invalid := CodexCount{State: CodexInvalid}
	s := string(bytes.TrimSpace(raw))
	if s == "" || s[0] < '0' || s[0] > '9' {
		return invalid
	}
	// JSON already validated number syntax. Work from decimal digits rather
	// than float64, and bound scaling before allocating an exponent's zeros.
	mantissa, exponent, _ := strings.Cut(strings.ToLower(s), "e")
	whole, frac, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(whole+frac, "0")
	if digits == "" {
		return CodexCount{State: CodexValid}
	}
	var shift int64
	if exponent != "" {
		var err error
		if shift, err = strconv.ParseInt(exponent, 10, 32); err != nil {
			return invalid
		}
	}
	shift -= int64(len(frac))
	if shift < 0 {
		trimmed := strings.TrimRight(digits, "0")
		if -shift > int64(len(digits)-len(trimmed)) {
			return invalid
		}
		digits = digits[:int64(len(digits))+shift]
		shift = 0
	}
	if int64(len(digits))+shift > 19 {
		return invalid
	}
	n, err := strconv.ParseInt(digits+strings.Repeat("0", int(shift)), 10, 64)
	if err != nil {
		return invalid
	}
	return CodexCount{State: CodexValid, Value: n}
}
