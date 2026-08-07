package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func TestSessionViewerTokensUseNormalizedSummaryTotals(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	service := usage.New(database, t.TempDir())
	service.Now = func() time.Time { return time.Date(2026, 7, 13, 2, 0, 0, 0, time.UTC) }
	if err := service.ImportBundledCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO usage_sessions(client,session_id,first_at,last_at)
VALUES('codex','token-session','2026-07-13T00:00:00Z','2026-07-13T00:00:00Z');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset)
VALUES('token-event','codex','token-session','event','2026-07-13T00:00:00Z','gpt-5.4',120,30,'fixture',0);`); err != nil {
		t.Fatal(err)
	}

	load := newSessionViewerLoad(ctx, database, session.Metadata{Client: "codex", SessionID: "token-session"}, service)
	page, err := load(ctx, viewerTokens, 1, sessionViewerPageLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Lines) == 0 || page.Lines[0] != "input: 120 · output: 30" {
		t.Fatalf("token summary = %#v", page.Lines)
	}
}
