package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestSessionSearchCommandPaginationContracts(t *testing.T) {
	stateDir := t.TempDir()
	database, err := store.OpenSessions(context.Background(), stateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for i := 0; i < 25; i++ {
		id := fmt.Sprintf("codex-%02d", i)
		document := session.Document{Client: "codex", SessionID: id, EventAt: "2026-08-06T00:00:00Z", Kind: "user_prompt", Text: "needle " + id}
		if err := session.ReplaceDocuments(context.Background(), database.DB, "codex", id, []session.Document{document}); err != nil {
			t.Fatal(err)
		}
	}
	if err := session.ReplaceDocuments(context.Background(), database.DB, "claude", "claude-01", []session.Document{{Client: "claude", SessionID: "claude-01", EventAt: "2026-08-06T00:00:00Z", Kind: "user_prompt", Text: "needle claude"}}); err != nil {
		t.Fatal(err)
	}

	legacy, err := runSessionCommand(stateDir, "--format", "json", "session", "search", "needle", "--client", "codex")
	if err != nil {
		t.Fatal(err)
	}
	var legacyEnvelope struct {
		Data []session.Document `json:"data"`
	}
	if err := json.Unmarshal([]byte(legacy), &legacyEnvelope); err != nil {
		t.Fatal(err)
	}
	if len(legacyEnvelope.Data) != 25 {
		t.Fatalf("legacy documents = %d, want 25", len(legacyEnvelope.Data))
	}
	text, err := runSessionCommand(stateDir, "session", "search", "needle", "--client", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(text, "needle codex-") != 20 || !strings.Contains(text, "Showing 1-20 of 25\n") || !strings.Contains(text, "Next page: agentdeck") {
		t.Fatalf("default text page = %q", text)
	}

	paged, err := runSessionCommand(stateDir, "--format", "json", "session", "search", "needle", "--client", "codex", "--page", "2", "--limit", "20")
	if err != nil {
		t.Fatal(err)
	}
	var pagedEnvelope struct {
		Data struct {
			Documents  []session.Document            `json:"documents"`
			Pagination map[string]session.Pagination `json:"pagination"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(paged), &pagedEnvelope); err != nil {
		t.Fatal(err)
	}
	page, found := pagedEnvelope.Data.Pagination["search"]
	if !found || page.Total != 25 || page.Shown != 5 || page.Page != 2 || page.HasMore {
		t.Fatalf("search pagination = %#v", pagedEnvelope.Data.Pagination)
	}
	if _, found := pagedEnvelope.Data.Pagination["documents"]; found {
		t.Fatalf("legacy pagination namespace remained: %#v", pagedEnvelope.Data.Pagination)
	}
	if len(pagedEnvelope.Data.Documents) != 5 {
		t.Fatalf("paged documents = %d, want 5", len(pagedEnvelope.Data.Documents))
	}
	for _, document := range pagedEnvelope.Data.Documents {
		if document.Client != "codex" {
			t.Fatalf("client filter was applied after pagination: %#v", document)
		}
	}

	if _, err := runSessionCommand(stateDir, "session", "search", "needle", "--all", "--page", "2"); err == nil {
		t.Fatal("--all with --page succeeded")
	}
	if _, err := runSessionCommand(stateDir, "session", "search", "needle", "--limit", "0"); err == nil {
		t.Fatal("zero search limit succeeded")
	}
}

func TestRenderSessionDocumentsRendersInvalidTimesAsDash(t *testing.T) {
	var output bytes.Buffer
	if err := renderSessionDocuments(&output, []session.Document{
		{Client: "codex", SessionID: "empty", EventAt: "", Kind: "user_prompt", Text: "empty"},
		{Client: "codex", SessionID: "invalid", EventAt: "not-a-time", Kind: "assistant_final", Text: "invalid"},
	}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "EVENT AT (") || strings.Contains(text, "not-a-time") || strings.Count(text, "—") != 2 {
		t.Fatalf("rendered documents = %q", text)
	}
}

func TestRenderSessionDocumentsRendersEventTimeInDisplayZone(t *testing.T) {
	usePinnedDisplayZone(t, time.FixedZone("UTC+8", 8*60*60))
	var output bytes.Buffer
	if err := renderSessionDocuments(&output, []session.Document{{Client: "codex", SessionID: "s", EventAt: "2026-08-06T16:00:00Z", Kind: "user_prompt", Text: "visible"}}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "EVENT AT (UTC+8)") || !strings.Contains(text, "2026-08-07 00:00:00") {
		t.Fatalf("rendered document time = %q", text)
	}
}

func TestSessionSearchNextCommandQuotesQuery(t *testing.T) {
	command := sessionNextCommand("/tmp/state dir", "search", "codex", "needle's", false, false, session.Pagination{HasMore: true, NextPage: 2, Limit: 20})
	if !strings.Contains(command, "search 'needle'\"'\"'s' --client 'codex' --page 2 --limit 20") {
		t.Fatalf("next command = %q", command)
	}
}

func runSessionCommand(stateDir string, args ...string) (string, error) {
	var output bytes.Buffer
	command := newRootCommand(strings.NewReader(""), &output)
	command.SetArgs(append([]string{"--state-dir", stateDir}, args...))
	err := command.Execute()
	return output.String(), err
}
