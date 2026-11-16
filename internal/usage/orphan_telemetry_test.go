package usage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestOrphanCleanupKeepsTelemetryOwnedFacts(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for _, row := range [][2]string{{"otel:abc", "bg-1"}, {"/gone/rollout.jsonl", "main-1"}} {
		if _, err := database.DB.Exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES(?, 'codex', ?, ?, '2026-10-29T12:00:00Z', 'm', 10, 1, ?, 0)`, row[0], row[1], row[0], row[0]); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := New(database, t.TempDir()).cleanupOrphanedEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	rows, _ := database.DB.Query(`SELECT source_path FROM usage_events`)
	for rows.Next() {
		var path string
		_ = rows.Scan(&path)
		kept = append(kept, path)
	}
	rows.Close()
	if removed != 1 || len(kept) != 1 || kept[0] != "otel:abc" {
		t.Fatalf("removed %d, kept %v", removed, kept)
	}
}
