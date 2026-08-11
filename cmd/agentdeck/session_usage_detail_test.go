package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func TestSessionShowTokensUsesEventTimeUsageAndInvocationPagination(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	home := t.TempDir()
	sessions, err := store.OpenSessions(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.ReplaceDocuments(ctx, sessions.DB, "codex", "token-session", []session.Document{{Client: "codex", SessionID: "token-session", Kind: "user_prompt", Text: "visible"}}); err != nil {
		sessions.Close()
		t.Fatal(err)
	}
	if err := sessions.Close(); err != nil {
		t.Fatal(err)
	}
	core, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	service := usage.New(core, home)
	service.Now = func() time.Time { return time.Date(2026, 7, 13, 2, 0, 0, 0, time.UTC) }
	if err := service.ImportBundledCatalog(ctx); err != nil {
		core.Close()
		t.Fatal(err)
	}
	_, err = core.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES('codex','token-session','2026-07-13T00:00:00Z','2026-07-13T00:01:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,source_path,source_offset) VALUES
 ('early','codex','token-session','private-event','2026-07-13T00:00:00Z','gpt-5.4',1000000,'/private/source.jsonl',1),
 ('late','codex','token-session','private-event','2026-07-13T00:01:00Z','gpt-5.4',2000000,'/private/source.jsonl',2);`)
	if err != nil {
		core.Close()
		t.Fatal(err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = oldHome })

	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"--state-dir", state, "--format", "json", "session", "show", "token-session", "--client", "codex", "--tokens", "--limit", "1"}, bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	var envelope struct {
		Command string `json:"command"`
		Data    struct {
			Usage       usage.SessionSummary          `json:"usage"`
			Invocations []usage.SessionInvocation     `json:"invocations"`
			Pagination  map[string]session.Pagination `json:"pagination"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Command != "session.show" || envelope.Data.Usage.Tokens["input_tokens"] != 3000000 || len(envelope.Data.Invocations) != 1 || envelope.Data.Invocations[0].Sequence != 1 || envelope.Data.Pagination["invocations"] != (session.Pagination{Page: 1, Limit: 1, Total: 2, Shown: 1, HasMore: true, NextPage: 2}) {
		t.Fatalf("envelope=%#v", envelope)
	}
	if strings.Contains(stdout.String(), "private-event") || strings.Contains(stdout.String(), "/private/source.jsonl") || stderr.Len() != 0 {
		t.Fatalf("session token output leaked private usage metadata stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	stdout.Reset()
	if exit := execute([]string{"--state-dir", state, "--format", "json", "session", "show", "token-session", "--client", "codex", "--tokens"}, bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
		t.Fatalf("complete JSON exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	var complete struct {
		Data struct {
			Invocations []usage.SessionInvocation     `json:"invocations"`
			Pagination  map[string]session.Pagination `json:"pagination"`
		} `json:"data"`
	}
	var rawEnvelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &complete); err != nil || json.Unmarshal(stdout.Bytes(), &rawEnvelope) != nil || len(complete.Data.Invocations) != 2 || complete.Data.Pagination != nil || rawEnvelope.Data["pagination"] != nil {
		t.Fatalf("complete JSON=%q decoded=%#v err=%v", stdout.String(), complete, err)
	}
	var failedOut, failedErr bytes.Buffer
	if exit := execute([]string{"--state-dir", state, "session", "show", "token-session", "--client", "codex", "--tokens", "--limit", "0"}, bytes.NewReader(nil), &failedOut, &failedErr); exit == 0 {
		t.Fatalf("invalid token page unexpectedly succeeded stdout=%q stderr=%q", failedOut.String(), failedErr.String())
	}

	stdout.Reset()
	if exit := execute([]string{"--state-dir", state, "session", "show", "token-session", "--client", "codex", "--tokens", "--page", "1", "--limit", "1"}, bytes.NewReader(nil), &stdout, &stderr); exit != 0 {
		t.Fatalf("text exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "TOKENS") || !strings.Contains(stdout.String(), "INVOCATIONS") || !strings.Contains(stdout.String(), "SHOWING") || !strings.Contains(stdout.String(), "1-1 of 2") || !strings.Contains(stdout.String(), "--tokens") {
		t.Fatalf("text token page=%q", stdout.String())
	}
}
