// Package activity extracts only allowlisted session and tool-call metadata.
package activity

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxMetadataLength = 256

// Record is an internal source-owned tool-call transition. It deliberately has
// no fields for arguments, results, command text, environment, or reasoning.
type Record struct {
	Key, Client, SessionID, Model, Tool, StartedAt, CompletedAt, Status, SourcePath string
	SourceOffset                                                                    int64
}

// Detail is the safe, user-visible form of a merged tool call.
type Detail struct {
	Client      string `json:"client"`
	SessionID   string `json:"session_id"`
	Model       string `json:"model"`
	Tool        string `json:"tool"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	Status      string `json:"status"`
	DurationMS  *int64 `json:"duration_ms"`
}

// Parser retains only the client context required to identify safe metadata.
type Parser struct {
	client, sourcePath       string
	sessionID, turnID, model string
}

func NewParser(client, sourcePath string) *Parser {
	return &Parser{client: client, sourcePath: sourcePath}
}

func (p *Parser) SetContext(sessionID, turnID, model string) {
	p.sessionID, p.turnID, p.model = sessionID, turnID, model
}

func (p *Parser) Context() (sessionID, turnID, model string) {
	return p.sessionID, p.turnID, p.model
}

func (p *Parser) Parse(value map[string]any, offset int64) []Record {
	if p.client == "codex" {
		return p.parseCodex(value, offset)
	}
	if p.client == "claude" {
		return p.parseClaude(value, offset)
	}
	return nil
}

func (p *Parser) parseCodex(value map[string]any, offset int64) []Record {
	payload, _ := value["payload"].(map[string]any)
	switch safeString(value["type"]) {
	case "session_meta":
		p.sessionID = firstSafe(payload["session_id"], payload["id"])
		return nil
	case "turn_context":
		p.turnID = safeString(payload["turn_id"])
		p.model = safeString(payload["model"])
		return nil
	}
	if p.sessionID == "" {
		return nil
	}
	item := payload
	if nested, ok := payload["item"].(map[string]any); ok {
		item = nested
	}
	if safeString(value["type"]) == "event_msg" {
		if nested, ok := payload["item"].(map[string]any); ok {
			item = nested
		}
	}
	timestamp := safeTimestamp(value["timestamp"])
	kind := safeString(item["type"])
	switch kind {
	case "function_call", "custom_tool_call", "mcp_tool_call", "web_search_call", "computer_call":
		tool := firstSafe(item["name"], item["tool_name"], item["tool"])
		callID := firstSafe(item["call_id"], item["id"])
		return p.started(callID, tool, timestamp, offset)
	case "function_call_output", "custom_tool_call_output", "mcp_tool_call_output", "web_search_call_output", "computer_call_output":
		callID := firstSafe(item["call_id"], item["id"])
		return p.completed(callID, timestamp, "completed", offset)
	}
	return nil
}

func (p *Parser) parseClaude(value map[string]any, offset int64) []Record {
	p.sessionID = firstSafe(value["sessionId"], value["session_id"], p.sessionID)
	if p.sessionID == "" {
		return nil
	}
	message, _ := value["message"].(map[string]any)
	if model := safeString(message["model"]); model != "" && model != "<synthetic>" {
		p.model = model
	}
	timestamp := safeTimestamp(value["timestamp"])
	var records []Record
	for _, item := range contentItems(message["content"]) {
		switch safeString(item["type"]) {
		case "tool_use":
			records = append(records, p.started(safeString(item["id"]), safeString(item["name"]), timestamp, offset)...)
		case "tool_result":
			status := "completed"
			if failed, _ := item["is_error"].(bool); failed {
				status = "failed"
			}
			records = append(records, p.completed(firstSafe(item["tool_use_id"], item["id"]), timestamp, status, offset)...)
		}
	}
	return records
}

func (p *Parser) started(callID, tool, at string, offset int64) []Record {
	if tool == "" || at == "" {
		return nil
	}
	key := p.key(callID, tool, at)
	return []Record{{Key: key, Client: p.client, SessionID: p.sessionID, Model: p.model, Tool: tool, StartedAt: at, Status: "started", SourcePath: p.sourcePath, SourceOffset: offset}}
}

func (p *Parser) completed(callID, at, status string, offset int64) []Record {
	if callID == "" || at == "" {
		return nil
	}
	return []Record{{Key: p.key(callID, "", ""), Client: p.client, SessionID: p.sessionID, CompletedAt: at, Status: status, SourcePath: p.sourcePath, SourceOffset: offset}}
}

func (p *Parser) key(callID, tool, at string) string {
	if callID != "" {
		return p.client + ":" + p.sessionID + ":" + callID
	}
	digest := sha256.Sum256([]byte(strings.Join([]string{p.sessionID, p.turnID, p.model, tool, at}, "\x00")))
	return p.client + ":" + p.sessionID + ":anonymous:" + hex.EncodeToString(digest[:])
}

func contentItems(value any) []map[string]any {
	items, _ := value.([]any)
	result := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		if item, ok := raw.(map[string]any); ok {
			result = append(result, item)
		}
	}
	return result
}

func safeTimestamp(value any) string {
	raw := safeString(value)
	at, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}

func firstSafe(values ...any) string {
	for _, value := range values {
		if text := safeString(value); text != "" {
			return text
		}
	}
	return ""
}

func safeString(value any) string {
	text, _ := value.(string)
	text = strings.TrimSpace(text)
	if text == "" || len(text) > maxMetadataLength || !utf8.ValidString(text) {
		return ""
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return text
}

// Merge combines start and completion transitions without exposing call IDs.
func Merge(records []Record) []Detail {
	byKey := make(map[string]*Detail, len(records))
	for _, record := range records {
		detail := byKey[record.Key]
		if record.StartedAt != "" {
			if detail == nil {
				detail = &Detail{}
				byKey[record.Key] = detail
			}
			detail.Client = record.Client
			detail.SessionID = record.SessionID
			detail.Model = record.Model
			detail.Tool = record.Tool
			detail.StartedAt = record.StartedAt
			detail.Status = "started"
		}
		if detail == nil || record.CompletedAt == "" {
			continue
		}
		detail.CompletedAt = record.CompletedAt
		detail.Status = record.Status
		started, startErr := time.Parse(time.RFC3339Nano, detail.StartedAt)
		completed, completeErr := time.Parse(time.RFC3339Nano, detail.CompletedAt)
		if startErr == nil && completeErr == nil && !completed.Before(started) {
			duration := completed.Sub(started).Milliseconds()
			detail.DurationMS = &duration
		}
	}
	result := make([]Detail, 0, len(byKey))
	for _, detail := range byKey {
		if detail.Tool != "" && detail.StartedAt != "" {
			result = append(result, *detail)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].StartedAt != result[j].StartedAt {
			return result[i].StartedAt < result[j].StartedAt
		}
		return result[i].Tool < result[j].Tool
	})
	return result
}

// ReadDetails parses a source on demand and returns only safe metadata for one
// logical session. Raw tool input and output are never retained.
func ReadDetails(path, client, sessionID string) ([]Detail, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	parser := NewParser(client, path)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	var records []Record
	var offset int64
	for scanner.Scan() {
		line := scanner.Bytes()
		var value map[string]any
		if json.Unmarshal(line, &value) == nil {
			records = append(records, parser.Parse(value, offset)...)
		}
		offset += int64(len(line) + 1)
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("read session activity: %w", err)
	}
	details := Merge(records)
	filtered := details[:0]
	for _, detail := range details {
		if detail.Client == client && detail.SessionID == sessionID {
			filtered = append(filtered, detail)
		}
	}
	return filtered, nil
}
