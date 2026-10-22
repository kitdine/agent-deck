package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestAffectedSessionBatchRenameCountsAgainstFullView(t *testing.T) {
	for _, batch := range []bool{false, true} {
		name := "full-inventory"
		if batch {
			name = "priority-batch"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			home := t.TempDir()
			db, err := store.OpenSessions(ctx, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			active := filepath.Join(home, ".codex", "sessions", "a.jsonl")
			alternate := filepath.Join(home, ".codex", "archived_sessions", "b.jsonl")
			moved := filepath.Join(home, ".codex", "archived_sessions", "zz.jsonl")
			write := func(path, text string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				b, err := json.Marshal(map[string]any{"type": "visible_user_prompt", "session_id": "shared", "timestamp": "2026-10-20T01:00:00Z", "payload": map[string]string{"text": text}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(b, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write(active, "active winner")
			write(alternate, "archived alternative")
			if _, err := Scan(ctx, db.DB, home); err != nil {
				t.Fatal(err)
			}
			before, err := visibleDocuments(ctx, db.DB)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(active, moved); err != nil {
				t.Fatal(err)
			}
			sources, err := ingest.Discover(home)
			if err != nil {
				t.Fatal(err)
			}
			if batch {
				only := sources[:0]
				for _, s := range sources {
					if s.Path == moved {
						only = append(only, s)
					}
				}
				sources = only
				if len(sources) != 1 {
					t.Fatalf("expected one renamed source, got %d", len(sources))
				}
			}
			result, err := ScanWithOptions(ctx, db.DB, home, ScanOptions{PreparedSources: sources, Batch: batch})
			if err != nil {
				t.Fatal(err)
			}
			after, err := visibleDocuments(ctx, db.DB)
			if err != nil {
				t.Fatal(err)
			}
			changed, removed := visibleDocumentChanges(before, after)
			if changed != 1 || removed != 0 {
				t.Fatalf("expected owner replacement: changed=%d removed=%d", changed, removed)
			}
			t.Logf("batch=%v before=%+v after=%+v reported documents=%d removed=%d full-oracle documents=%d removed=%d", batch, before, after, result.Documents, result.Removed, changed, removed)
			if result.Documents != changed || result.Removed != removed {
				t.Fatalf("scoped session delta differs from full oracle")
			}
		})
	}
}
