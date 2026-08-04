package usagehook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ValidateEvent validates the bounded JSON envelope delivered by a client
// lifecycle hook. Persistence is intentionally owned by the next task, so the
// current lifecycle task only defines and validates the installed wire shape.
func ValidateEvent(client Client, contents []byte) error {
	if len(contents) > MaxEventBytes {
		return fmt.Errorf("hook event exceeds %d bytes", MaxEventBytes)
	}
	if client != ClientCodex && client != ClientClaude {
		return fmt.Errorf("unsupported hook event client %q", client)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	var event map[string]json.RawMessage
	if err := decoder.Decode(&event); err != nil {
		return fmt.Errorf("invalid hook event JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("invalid hook event JSON: trailing data")
		}
		return fmt.Errorf("invalid hook event JSON: %w", err)
	}
	if event == nil {
		return errors.New("hook event must be a JSON object")
	}
	var sessionID, eventName, source string
	if err := decodeRequiredString(event, "session_id", &sessionID); err != nil {
		return err
	}
	if err := decodeRequiredString(event, "hook_event_name", &eventName); err != nil {
		return err
	}
	if raw, ok := event["source"]; ok {
		if err := json.Unmarshal(raw, &source); err != nil {
			return errors.New("hook event source must be a string")
		}
	}
	if client == ClientCodex {
		if eventName != "SessionStart" {
			return fmt.Errorf("unsupported Codex hook event %q", eventName)
		}
		switch source {
		case "startup", "resume", "clear", "compact":
		default:
			return fmt.Errorf("unsupported Codex hook source %q", source)
		}
		return nil
	}
	switch eventName {
	case "SessionStart":
		switch source {
		case "startup", "resume", "clear", "compact", "fork":
		default:
			return fmt.Errorf("unsupported Claude hook source %q", source)
		}
	case "ConfigChange", "SessionEnd":
		// These events do not have a required source in Claude's contract.
	default:
		return fmt.Errorf("unsupported Claude hook event %q", eventName)
	}
	return nil
}

func decodeRequiredString(event map[string]json.RawMessage, name string, destination *string) error {
	raw, ok := event[name]
	if !ok {
		return fmt.Errorf("hook event missing %s", name)
	}
	if err := json.Unmarshal(raw, destination); err != nil || *destination == "" {
		return fmt.Errorf("hook event %s must be a non-empty string", name)
	}
	return nil
}
