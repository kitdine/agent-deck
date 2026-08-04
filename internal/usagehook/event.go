package usagehook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Event is the bounded, non-sensitive portion of a client Hook payload.
type Event struct {
	ConfigPath                string
	SessionID, TranscriptPath string
	Name, Source              string
}

// ValidateEvent validates the bounded JSON envelope delivered by a client
// lifecycle hook. Persistence is intentionally owned by the next task, so the
// current lifecycle task only defines and validates the installed wire shape.
func ValidateEvent(client Client, contents []byte) error {
	_, err := ParseEvent(client, contents)
	return err
}

// ParseEvent rejects unsupported client lifecycle input without retaining its
// raw payload. The caller decides whether a valid event forms a route boundary.
func ParseEvent(client Client, contents []byte) (Event, error) {
	if len(contents) > MaxEventBytes {
		return Event{}, fmt.Errorf("hook event exceeds %d bytes", MaxEventBytes)
	}
	if client != ClientCodex && client != ClientClaude {
		return Event{}, fmt.Errorf("unsupported hook event client %q", client)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	var event map[string]json.RawMessage
	if err := decoder.Decode(&event); err != nil {
		return Event{}, fmt.Errorf("invalid hook event JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Event{}, errors.New("invalid hook event JSON: trailing data")
		}
		return Event{}, fmt.Errorf("invalid hook event JSON: %w", err)
	}
	if event == nil {
		return Event{}, errors.New("hook event must be a JSON object")
	}
	var sessionID, eventName, source, transcriptPath string
	if err := decodeRequiredString(event, "session_id", &sessionID); err != nil {
		return Event{}, err
	}
	if err := decodeRequiredString(event, "hook_event_name", &eventName); err != nil {
		return Event{}, err
	}
	if raw, ok := event["transcript_path"]; ok {
		if err := json.Unmarshal(raw, &transcriptPath); err != nil || transcriptPath == "" {
			return Event{}, errors.New("hook event transcript_path must be a non-empty string")
		}
	}
	if raw, ok := event["source"]; ok {
		if err := json.Unmarshal(raw, &source); err != nil {
			return Event{}, errors.New("hook event source must be a string")
		}
	}
	var configPath string
	if raw, ok := event["file_path"]; ok {
		if err := json.Unmarshal(raw, &configPath); err != nil || configPath == "" {
			return Event{}, errors.New("hook event file_path must be a non-empty string")
		}
	}
	if client == ClientCodex {
		if eventName != "SessionStart" {
			return Event{}, fmt.Errorf("unsupported Codex hook event %q", eventName)
		}
		switch source {
		case "startup", "resume", "clear", "compact":
		default:
			return Event{}, fmt.Errorf("unsupported Codex hook source %q", source)
		}
		return Event{SessionID: sessionID, TranscriptPath: transcriptPath, Name: eventName, Source: source, ConfigPath: configPath}, nil
	}
	switch eventName {
	case "SessionStart":
		switch source {
		case "startup", "resume", "clear", "compact", "fork":
		default:
			return Event{}, fmt.Errorf("unsupported Claude hook source %q", source)
		}
	case "ConfigChange":
		switch source {
		case "user_settings", "project_settings", "local_settings", "policy_settings", "skills":
		default:
			return Event{}, fmt.Errorf("unsupported Claude config source %q", source)
		}
	case "SessionEnd":
		// This event does not have a required source in Claude's contract.
	default:
		return Event{}, fmt.Errorf("unsupported Claude hook event %q", eventName)
	}
	return Event{SessionID: sessionID, TranscriptPath: transcriptPath, Name: eventName, Source: source, ConfigPath: configPath}, nil
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
