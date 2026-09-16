package session

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

type deleteCountingExecutor struct {
	*sql.DB
	deletes      int
	orphanChecks int
}

func (e *deleteCountingExecutor) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	if strings.HasPrefix(q, "SELECT DISTINCT d.source_path") {
		e.orphanChecks++
	}
	return e.DB.QueryContext(ctx, q, args...)
}

func (e *deleteCountingExecutor) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	if strings.HasPrefix(q, "DELETE FROM session_documents") {
		e.deletes++
	}
	return e.DB.ExecContext(ctx, q, args...)
}

func TestNewSourceAvoidsFTSDeleteButRewriteAndOrphanAreCleaned(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "sessions", "s.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(`{"type":"visible_user_prompt","session_id":"s","payload":{"text":"`+text+`"}}`+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	db, err := store.OpenSessions(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	e := &deleteCountingExecutor{DB: db.DB}
	run := func() {
		t.Helper()
		paths, err := sessionSources(home, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = scan(ctx, e, paths, nil, func(src source) (bool, int, error) { return scanSourceExec(ctx, e, src) }, func(seen map[string]bool) error { return removeMissingSourcesExec(ctx, e, seen) })
		if err != nil {
			t.Fatal(err)
		}
	}
	write("first")
	run()
	if e.deletes != 0 {
		t.Fatalf("new source deleted FTS %d times", e.deletes)
	}
	run()
	if e.orphanChecks != 1 || e.deletes != 0 {
		t.Fatalf("unchanged scan: orphan checks=%d deletes=%d", e.orphanChecks, e.deletes)
	}
	write("next")
	run()
	if e.deletes != 1 {
		t.Fatalf("rewrite deletes=%d", e.deletes)
	}
	// FTS is not foreign-key constrained; losing its registry must not duplicate documents.
	if _, err = db.DB.ExecContext(ctx, "DELETE FROM session_metadata; DELETE FROM session_sources"); err != nil {
		t.Fatal(err)
	}
	run()
	if e.deletes != 2 {
		t.Fatalf("orphan recovery deletes=%d", e.deletes)
	}
	var count int
	if err = db.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM session_documents").Scan(&count); err != nil || count != 1 {
		t.Fatalf("documents=%d err=%v", count, err)
	}
}
