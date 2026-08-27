package usage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestSessionUsageSummaryAndInvocationsUseStoredEventDeltasWithoutPrivateMetadata(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := New(database, "")
	service.Now = func() time.Time { return time.Date(2026, 7, 13, 2, 0, 0, 0, time.UTC) }
	if err := service.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES
  ('codex','session','2026-07-13T00:00:00Z','2026-07-13T00:01:00Z'),
  ('claude','claude-session','2026-07-13T00:02:00Z','2026-07-13T00:02:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,source_path,source_offset) VALUES
  ('later','codex','session','private-event-id','2026-07-13T00:01:00Z','gpt-5.4',2000000,0,0,'/private/source.jsonl',20),
  ('earlier','codex','session','private-event-id','2026-07-13T00:00:00Z','gpt-5.4',1000000,0,0,'/private/source.jsonl',10),
  ('claude-event','claude','claude-session','private-claude-event','2026-07-13T00:02:00Z','unknown-claude-model',0,0,0,'/private/claude.jsonl',30);
UPDATE usage_events SET cache_read_tokens=7,cache_creation_tokens=11,cache_write_5m_tokens=13,cache_write_1h_tokens=17 WHERE event_key='claude-event';`)
	if err != nil {
		t.Fatal(err)
	}

	summary, err := service.SessionUsageSummary(ctx, "codex", "session")
	if err != nil {
		t.Fatal(err)
	}
	if summary.FirstAt != "2026-07-13T00:00:00Z" || summary.LastAt != "2026-07-13T00:01:00Z" || summary.Tokens["input_tokens"] != 3000000 || summary.CatalogBaseCost == nil || *summary.CatalogBaseCost != "7.500000000" {
		t.Fatalf("summary=%#v", summary)
	}

	first, pagination, err := service.SessionInvocations(ctx, "codex", "session", 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Sequence != 1 || first[0].EventAt != "2026-07-13T00:00:00Z" || first[0].Tokens["input_tokens"] != 1000000 || pagination != (InvocationPagination{Page: 1, Limit: 1, Total: 2, Shown: 1, HasMore: true, NextPage: 2}) {
		t.Fatalf("first=%#v pagination=%#v", first, pagination)
	}
	if first[0].CatalogBaseCost == nil || first[0].ProviderCost != nil || first[0].KnownProviderCost != "" {
		t.Fatalf("unattributed invocation cost = %#v", first[0])
	}
	second, pagination, err := service.SessionInvocations(ctx, "codex", "session", 2, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Sequence != 2 || second[0].EventAt != "2026-07-13T00:01:00Z" || pagination.HasMore {
		t.Fatalf("second=%#v pagination=%#v", second, pagination)
	}
	encoded, err := json.Marshal(second[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private-event-id", "/private/source.jsonl", "\"event_key\"", "\"source_offset\""} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("invocation leaked %q: %s", private, encoded)
		}
	}

	claude, _, err := service.SessionInvocations(ctx, "claude", "claude-session", 1, 20, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(claude) != 1 || claude[0].Tokens["cache_read_tokens"] != 7 || claude[0].Tokens["cache_creation_tokens"] != 11 || claude[0].Tokens["cache_write_5m_tokens"] != 13 || claude[0].Tokens["cache_write_1h_tokens"] != 17 || claude[0].CatalogBaseCost != nil || len(claude[0].Unpriced) == 0 {
		t.Fatalf("claude invocation=%#v", claude)
	}
}

func TestSessionInvocationsDoesNotDecodeOffPageRows(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = database.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at) VALUES('codex','bounded','2026-07-13T00:00:00Z','2026-07-13T00:01:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES
 ('first','codex','bounded','first','2026-07-13T00:00:00Z','gpt-5.4','first-source',1),
 ('off-page','codex','bounded','off-page','2026-07-13T00:01:00Z','gpt-5.4','off-page-source','not-an-offset');`)
	if err != nil {
		t.Fatal(err)
	}
	service := New(database, "")
	page, pagination, err := service.SessionInvocations(ctx, "codex", "bounded", 1, 1, false)
	if err != nil {
		t.Fatalf("first page decoded an off-page row: %v", err)
	}
	if len(page) != 1 || page[0].Sequence != 1 || pagination.Total != 2 || !pagination.HasMore {
		t.Fatalf("page=%#v pagination=%#v", page, pagination)
	}
	if _, _, err := service.SessionInvocations(ctx, "codex", "bounded", 2, 1, false); err == nil {
		t.Fatal("requested malformed second page unexpectedly succeeded")
	}
}
