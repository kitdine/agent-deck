package usagehook

import (
	"bytes"
	"testing"
)

func TestValidateEventAcceptsSupportedClientEvents(t *testing.T) {
	tests := []struct {
		name   string
		client Client
		input  string
	}{
		{
			name:   "codex session start",
			client: ClientCodex,
			input:  `{"session_id":"session-1","hook_event_name":"SessionStart","source":"startup"}`,
		},
		{
			name:   "claude session start",
			client: ClientClaude,
			input:  `{"session_id":"session-1","hook_event_name":"SessionStart","source":"fork"}`,
		},
		{
			name:   "claude config change",
			client: ClientClaude,
			input:  `{"session_id":"session-1","hook_event_name":"ConfigChange","source":"user_settings","file_path":"/tmp/settings.json"}`,
		},
		{
			name:   "claude session end",
			client: ClientClaude,
			input:  `{"session_id":"session-1","hook_event_name":"SessionEnd","reason":"resume"}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateEvent(test.client, []byte(test.input)); err != nil {
				t.Fatalf("ValidateEvent: %v", err)
			}
		})
	}
}

func TestValidateEventRejectsInvalidOrUnboundedInput(t *testing.T) {
	tests := []struct {
		name   string
		client Client
		input  []byte
	}{
		{name: "unknown client", client: Client("other"), input: []byte(`{"session_id":"s","hook_event_name":"SessionStart","source":"startup"}`)},
		{name: "malformed JSON", client: ClientCodex, input: []byte(`{"session_id":`)},
		{name: "trailing JSON", client: ClientCodex, input: []byte(`{"session_id":"s","hook_event_name":"SessionStart","source":"startup"}{}`)},
		{name: "missing session id", client: ClientCodex, input: []byte(`{"hook_event_name":"SessionStart","source":"startup"}`)},
		{name: "unsupported Codex event", client: ClientCodex, input: []byte(`{"session_id":"s","hook_event_name":"SessionEnd","source":"startup"}`)},
		{name: "unsupported Claude source", client: ClientClaude, input: []byte(`{"session_id":"s","hook_event_name":"SessionStart","source":"unknown"}`)},
		{name: "unsupported Claude config source", client: ClientClaude, input: []byte(`{"session_id":"s","hook_event_name":"ConfigChange","source":"unknown"}`)},
		{name: "empty Claude config path", client: ClientClaude, input: []byte(`{"session_id":"s","hook_event_name":"ConfigChange","source":"user_settings","file_path":""}`)},
		{name: "oversized", client: ClientClaude, input: bytes.Repeat([]byte("x"), MaxEventBytes+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateEvent(test.client, test.input); err == nil {
				t.Fatal("ValidateEvent unexpectedly accepted invalid input")
			}
		})
	}
}

func TestValidateEventDoesNotPersistOrMutateInput(t *testing.T) {
	input := []byte(`{"session_id":"session-1","hook_event_name":"ConfigChange","source":"user_settings","file_path":"/tmp/settings.json"}`)
	original := append([]byte(nil), input...)
	if err := ValidateEvent(ClientClaude, input); err != nil {
		t.Fatalf("ValidateEvent: %v", err)
	}
	if !bytes.Equal(input, original) {
		t.Fatalf("ValidateEvent mutated input: got %q, want %q", input, original)
	}
}
