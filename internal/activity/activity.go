// Package activity reads session tool arguments transiently and retains only
// allowlisted session, turn, tool-kind, MCP, and opaque file metadata.
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

// Record is an internal source-owned tool-call transition. It deliberately
// retains no arguments, results, command text, user message, environment,
// reasoning, path, or directory; file identity is reduced before construction.
type Record struct {
	Key, Client, SessionID, Model, Tool, StartedAt, CompletedAt, Status, SourcePath string
	SourceOffset                                                                    int64
	TurnIndex                                                                       int
	ToolKind, MCPServer                                                             string
	Files                                                                           []File
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

// Page is a bounded, safe view of activity from one source scan.
type Page struct {
	Details  []Detail
	Page     int
	Limit    int
	Total    int
	Shown    int
	HasMore  bool
	NextPage int
	Summary  Summary
}

// Summary retains aggregate safe activity metadata without retaining details.
type Summary struct {
	Total             int
	Completed         int
	Failed            int
	Incomplete        int
	TotalDurationMS   int64
	AverageDurationMS *int64
	ByTool            map[string]int
}

// maxPageCandidates bounds the in-memory sorted selection used to serve a
// single source scan. Deeper activity pages are rejected rather than growing
// memory with the caller-controlled page number.
const maxPageCandidates = 1000

// Parser retains only the client context required to identify safe metadata.
type Parser struct {
	client, sourcePath       string
	sessionID, turnID, model string
	machineIdentity          string
	turnIndex                int
	claudePendingUserMessage bool
}

func NewParser(client, sourcePath string) *Parser {
	return &Parser{client: client, sourcePath: sourcePath}
}

func (p *Parser) SetContext(sessionID, turnID, model string) {
	p.sessionID, p.turnID, p.model = sessionID, turnID, model
}

func (p *Parser) SetTurnIndex(turnIndex int) { p.turnIndex = max(turnIndex, 0) }

func (p *Parser) SetClaudePending(pending bool) { p.claudePendingUserMessage = pending }

func (p *Parser) SetMachineIdentity(machineIdentity string) {
	p.machineIdentity = machineIdentity
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
		sessionID := firstSafe(payload["session_id"], payload["id"])
		if sessionID != "" && sessionID != p.sessionID {
			p.turnID, p.turnIndex = "", 0
		}
		p.sessionID = sessionID
		return nil
	case "turn_context":
		turnID := safeString(payload["turn_id"])
		if turnID != "" && turnID != p.turnID {
			p.turnIndex++
		}
		p.turnID = turnID
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
		if tool == "" && kind == "mcp_tool_call" {
			tool = "mcp"
		}
		callID := firstSafe(item["call_id"], item["id"])
		toolKind, mcpServer, files := classifyCodexTool(item, kind, tool, p.machineIdentity)
		return p.started(callID, tool, timestamp, offset, toolKind, mcpServer, files)
	case "function_call_output", "custom_tool_call_output", "mcp_tool_call_output", "web_search_call_output", "computer_call_output":
		callID := firstSafe(item["call_id"], item["id"])
		return p.completed(callID, timestamp, "completed", offset)
	}
	return nil
}

func (p *Parser) parseClaude(value map[string]any, offset int64) []Record {
	sessionID := firstSafe(value["sessionId"], value["session_id"], p.sessionID)
	if sessionID != "" && sessionID != p.sessionID {
		p.turnID, p.turnIndex, p.claudePendingUserMessage = "", 0, false
	}
	p.sessionID = sessionID
	if p.sessionID == "" {
		return nil
	}
	message, _ := value["message"].(map[string]any)
	if ClaudeUserTurnBoundary(value, message) {
		p.claudePendingUserMessage = true
		return nil
	}
	if safeString(value["type"]) == "assistant" && p.claudePendingUserMessage {
		p.turnIndex++
		p.claudePendingUserMessage = false
	}
	if model := safeString(message["model"]); model != "" && model != "<synthetic>" {
		p.model = model
	}
	timestamp := safeTimestamp(value["timestamp"])
	var records []Record
	for _, item := range contentItems(message["content"]) {
		switch safeString(item["type"]) {
		case "tool_use":
			tool := safeString(item["name"])
			toolKind, mcpServer, files := classifyClaudeTool(item, tool, p.machineIdentity)
			records = append(records, p.started(safeString(item["id"]), tool, timestamp, offset, toolKind, mcpServer, files)...)
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

func (p *Parser) started(callID, tool, at string, offset int64, toolKind, mcpServer string, files []File) []Record {
	if tool == "" || at == "" {
		return nil
	}
	key := p.key(callID, tool, at)
	return []Record{{Key: key, Client: p.client, SessionID: p.sessionID, Model: p.model, Tool: tool, StartedAt: at, Status: "started", SourcePath: p.sourcePath, SourceOffset: offset, TurnIndex: p.turnIndex, ToolKind: toolKind, MCPServer: mcpServer, Files: files}}
}

func (p *Parser) completed(callID, at, status string, offset int64) []Record {
	if callID == "" || at == "" {
		return nil
	}
	return []Record{{Key: p.key(callID, "", ""), Client: p.client, SessionID: p.sessionID, CompletedAt: at, Status: status, SourcePath: p.sourcePath, SourceOffset: offset, TurnIndex: p.turnIndex}}
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

// ReadDetailsPage scans a source once, retaining only the requested earliest
// details while accumulating a complete safe summary for the selected session.
func ReadDetailsPage(path, client, sessionID string, page, limit int, all bool) (Page, error) {
	if page < 1 || limit < 1 || limit > 100 {
		return Page{}, fmt.Errorf("page must be positive and limit must be between 1 and 100")
	}
	keep := limit
	if !all {
		if page > maxPageCandidates/limit {
			return Page{}, fmt.Errorf("activity page exceeds bounded window of %d rows", maxPageCandidates)
		}
		keep = page * limit
	}
	file, err := os.Open(path)
	if err != nil {
		return Page{}, err
	}
	defer file.Close()
	parser := NewParser(client, path)
	started := map[string]Detail{}
	selected := make([]Detail, 0, keep)
	summary, durations := Summary{ByTool: map[string]int{}}, 0
	appendDetail := func(detail Detail) {
		if detail.DurationMS != nil {
			durations++
		}
		summarize(&summary, detail)
		if all {
			selected = append(selected, detail)
			return
		}
		at := sort.Search(len(selected), func(i int) bool { return detailBefore(detail, selected[i]) })
		if at >= keep {
			return
		}
		selected = append(selected, Detail{})
		copy(selected[at+1:], selected[at:])
		selected[at] = detail
		if len(selected) > keep {
			selected = selected[:keep]
		}
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	var offset int64
	for scanner.Scan() {
		line := scanner.Bytes()
		var value map[string]any
		if json.Unmarshal(line, &value) == nil {
			for _, record := range parser.Parse(value, offset) {
				if record.Client != client || record.SessionID != sessionID {
					continue
				}
				if record.StartedAt != "" {
					started[record.Key] = Detail{Client: record.Client, SessionID: record.SessionID, Model: record.Model, Tool: record.Tool, StartedAt: record.StartedAt, Status: "started"}
				}
				if record.CompletedAt != "" {
					detail, found := started[record.Key]
					if !found {
						continue
					}
					detail.CompletedAt, detail.Status = record.CompletedAt, record.Status
					if start, startErr := time.Parse(time.RFC3339Nano, detail.StartedAt); startErr == nil {
						if completed, completeErr := time.Parse(time.RFC3339Nano, detail.CompletedAt); completeErr == nil && !completed.Before(start) {
							duration := completed.Sub(start).Milliseconds()
							detail.DurationMS = &duration
						}
					}
					delete(started, record.Key)
					appendDetail(detail)
				}
			}
		}
		offset += int64(len(line) + 1)
	}
	if err := scanner.Err(); err != nil {
		return Page{}, fmt.Errorf("read session activity: %w", err)
	}
	for _, detail := range started {
		appendDetail(detail)
	}
	sort.Slice(selected, func(i, j int) bool { return detailBefore(selected[i], selected[j]) })
	if durations > 0 {
		average := summary.TotalDurationMS / int64(durations)
		summary.AverageDurationMS = &average
	}
	result := Page{Details: selected, Page: page, Limit: limit, Total: summary.Total, Shown: len(selected), Summary: summary}
	if all {
		result.Page, result.Limit = 1, len(selected)
		return result, nil
	}
	start := (page - 1) * limit
	if start >= summary.Total {
		result.Details, result.Shown = []Detail{}, 0
		return result, nil
	}
	end := min(start+limit, len(selected))
	result.Details = selected[start:end]
	result.Shown = len(result.Details)
	result.HasMore = start+result.Shown < summary.Total
	if result.HasMore {
		result.NextPage = page + 1
	}
	return result, nil
}

func detailBefore(left, right Detail) bool {
	if left.StartedAt != right.StartedAt {
		return left.StartedAt < right.StartedAt
	}
	if left.Tool != right.Tool {
		return left.Tool < right.Tool
	}
	return left.Status < right.Status
}

func summarize(summary *Summary, detail Detail) {
	summary.Total++
	summary.ByTool[detail.Tool]++
	switch detail.Status {
	case "completed":
		summary.Completed++
	case "failed":
		summary.Failed++
	default:
		summary.Incomplete++
	}
	if detail.DurationMS != nil {
		summary.TotalDurationMS += *detail.DurationMS
	}
}
