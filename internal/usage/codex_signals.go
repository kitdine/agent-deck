package usage

import (
	"encoding/json"
	"fmt"
	"strings"
)

const codexSignalCursorPrefix = "codex-signal-cursor:"

func usageParserOutdated(client string, version int) bool {
	// This compatibility exception belongs only to the Codex-only version 8
	// change; a later parser update must not silently reuse Claude version 7.
	if ParserVersion == 8 && client == "claude" && version == 7 {
		return false
	}
	return version != ParserVersion
}

// Like the Claude pending marker, this is private source-cursor metadata, not
// an event turn ID. It preserves an already observed context's index even when
// its message is still pending and therefore absent from the committed turns.
type codexSignalCursor struct {
	Turn            string `json:"turn"`
	Index           int    `json:"index"`
	Output          bool   `json:"output"`
	AwaitingContext bool   `json:"awaiting_context"`
}

func restoreCodexSignalCursor(state *parseState) (bool, error) {
	if !strings.HasPrefix(state.turn, codexSignalCursorPrefix) {
		return false, nil
	}
	var cursor codexSignalCursor
	if err := json.Unmarshal([]byte(strings.TrimPrefix(state.turn, codexSignalCursorPrefix)), &cursor); err != nil {
		return false, fmt.Errorf("invalid Codex signal cursor: %w", err)
	}
	if cursor.Index < 0 {
		return false, fmt.Errorf("invalid Codex signal cursor index")
	}
	state.turn, state.turnIndex = cursor.Turn, cursor.Index
	state.codexOutput, state.codexAwaitingContext, state.codexHasSignal = cursor.Output, cursor.AwaitingContext, true
	return true, nil
}

func storedCodexTurn(state parseState) string {
	// A context may be captured in a scan before its first user message. Its
	// index must survive even when no pending/classified signal exists yet.
	if !state.codexHasSignal && state.turn == "" {
		return state.turn
	}
	// All fields are supported JSON primitives; the message reduction remains
	// solely in usage_work_signals and no prompt text enters this cursor.
	data, _ := json.Marshal(codexSignalCursor{state.turn, state.turnIndex, state.codexOutput, state.codexAwaitingContext})
	return codexSignalCursorPrefix + string(data)
}

func codexHasOutput(recordType string, payload map[string]any) bool {
	if recordType == "event_msg" {
		switch payload["type"] {
		case "agent_message", "agent_reasoning", "task_complete":
			return true
		}
	}
	if recordType != "response_item" {
		return false
	}
	item := payload
	if nested, ok := payload["item"].(map[string]any); ok {
		item = nested
	}
	if item["type"] == "message" {
		return item["role"] == "assistant"
	}
	switch item["type"] {
	case "function_call", "custom_tool_call", "mcp_tool_call", "web_search_call", "computer_call":
		return true
	}
	return false
}

// codexSignalMessage reads only user message envelopes. Only the reduction in
// turnSignal leaves the parse call; message text never enters the usage store.
func codexSignalMessage(recordType string, payload map[string]any) (string, bool) {
	var text string
	switch recordType {
	case "event_msg":
		if payload["type"] != "user_message" {
			return "", false
		}
		text, _ = payload["message"].(string)
	case "response_item":
		item := payload
		if nested, ok := payload["item"].(map[string]any); ok {
			item = nested
		}
		if item["type"] != "message" || item["role"] != "user" {
			return "", false
		}
		content, _ := item["content"].([]any)
		var parts []string
		for _, raw := range content {
			block, _ := raw.(map[string]any)
			if block["type"] == "input_text" {
				if value, ok := block["text"].(string); ok && !codexInjectedContext(value) {
					parts = append(parts, value)
				}
			}
		}
		if len(parts) == 0 {
			return "", false
		}
		text = strings.Join(parts, "\n")
	default:
		return "", false
	}
	return text, !codexInjectedContext(text)
}

func codexInjectedContext(text string) bool {
	text = strings.TrimSpace(text)
	for _, prefix := range []string{
		"# AGENTS.md instructions for ",
		"<INSTRUCTIONS>",
		"<environment_context>",
		"<turn_aborted>",
		"<subagent_notification>",
	} {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}
