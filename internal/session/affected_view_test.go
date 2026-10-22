package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestAffectedSessionViewsMatchFullOracleAcrossOwnershipChanges(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	db, err := store.OpenSessions(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	path := func(name string) string { return filepath.Join(home, ".codex", "sessions", name+".jsonl") }
	line := func(id, text string) []byte {
		b, err := json.Marshal(map[string]any{"type": "visible_user_prompt", "session_id": id, "timestamp": "2026-10-20T01:00:00Z", "payload": map[string]string{"text": text}})
		if err != nil {
			t.Fatal(err)
		}
		return append(b, '\n')
	}
	write := func(name, id, text string) {
		t.Helper()
		p := path(name)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, line(id, text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	check := func(name string, mutate func()) {
		t.Helper()
		before, err := visibleDocuments(ctx, db.DB)
		if err != nil {
			t.Fatal(err)
		}
		mutate()
		result, err := Scan(ctx, db.DB, home)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		after, err := visibleDocuments(ctx, db.DB)
		if err != nil {
			t.Fatal(err)
		}
		changed, removed := visibleDocumentChanges(before, after)
		if result.Documents != changed || result.Removed != removed {
			t.Fatalf("%s: scoped delta %+v, full oracle changed=%d removed=%d", name, result, changed, removed)
		}
	}
	check("initial", func() { write("a", "shared", "original"); write("z", "other", "unaffected") })
	check("append", func() {
		f, err := os.OpenFile(path("a"), os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write(line("shared", "append")); err != nil {
			t.Fatal(err)
		}
		f.Close()
	})
	check("new higher-priority duplicate", func() { write("0", "shared", "replacement owner") })
	check("rewrite introduces an existing other ID", func() { write("0", "other", "replacement of other") })
	check("remove reveals previous owner", func() {
		if err := os.Remove(path("0")); err != nil {
			t.Fatal(err)
		}
	})
	check("multi-session rewrite", func() {
		if err := os.WriteFile(path("a"), append(line("shared", "one"), line("other", "two")...), 0600); err != nil {
			t.Fatal(err)
		}
	})
	check("move", func() {
		destination := filepath.Join(home, ".codex", "archived_sessions", "moved.jsonl")
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path("a"), destination); err != nil {
			t.Fatal(err)
		}
	})
	selected, err := visibleDocumentsFiltered(ctx, db.DB, map[string]affectedSession{"codex\x00other": {client: "codex", id: "other"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected["codex\x00other"] == nil {
		t.Fatal("scoped view materialized unrelated sessions")
	}
}
