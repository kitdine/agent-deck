package usage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestScanRereadsPreservedMtimeRewriteBeforeSuffixAnchor(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "sessions", "usage.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data := `{"type":"session_meta","payload":{"id":"s"}}
{"type":"turn_context","payload":{"turn_id":"t","model":"gpt-5"}}
{"type":"event_msg","timestamp":"2026-08-13T01:00:00Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10},"total_token_usage":{"input_tokens":10}}}}
` + strings.Repeat(" ", 5000) + "{}\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := New(db, home)
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	data = strings.ReplaceAll(data, `"input_tokens":10`, `"input_tokens":20`)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	var count, tokens int
	if err := db.DB.QueryRow("SELECT count(*),sum(input_tokens) FROM usage_events").Scan(&count, &tokens); err != nil {
		t.Fatal(err)
	}
	if count != 1 || tokens != 20 {
		t.Fatalf("rewrite left stale events: count=%d tokens=%d", count, tokens)
	}
}
