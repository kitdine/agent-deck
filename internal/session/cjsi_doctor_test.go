package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestCJSIIndexHealth(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	missing, err := CheckHealth(ctx, root, true)
	if err != nil || missing.Present || missing.Integrity != "not_requested" {
		t.Fatalf("missing: %+v, err=%v", missing, err)
	}
	index, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	paths := t.TempDir()
	readable := filepath.Join(paths, "readable")
	if err := os.WriteFile(readable, []byte("private-content-sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{readable, filepath.Join(paths, "private-missing-sentinel"), paths, "otel:synthetic", replaceDocumentsSourcePath} {
		if _, err := index.DB.ExecContext(ctx, `INSERT INTO session_sources
			(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at)
			VALUES (?,'fixture',0,0,0,'',1,1,'')`, path); err != nil {
			t.Fatal(err)
		}
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	health, err := CheckHealth(ctx, root, true)
	if err != nil || !health.Present || !health.FTSAvailable || health.Integrity != "ok" || health.UnreadableSources != 2 {
		t.Fatalf("health: %+v, err=%v", health, err)
	}
	text, err := json.Marshal(health)
	if err != nil || strings.Contains(string(text), "sentinel") {
		t.Fatalf("health detail leaked: %s, err=%v", text, err)
	}
}
