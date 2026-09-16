package usage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestPreparedIngestionSkipsUnchangedWritesAndPreservesOwnership(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	p, err := prepareIngestion(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	e := Event{Key: "one", Client: "codex", SessionID: "s", EventID: "one", EventAt: "2026-08-13T01:00:00Z", Model: "gpt-5", SourcePath: "z", Tokens: map[string]int64{"input_tokens": 10}}
	if added, changed, err := upsertTx(ctx, p, e); err != nil || !added || !changed {
		t.Fatalf("initial=%v/%v %v", added, changed, err)
	}
	if _, err = tx.ExecContext(ctx, `CREATE TEMP TABLE writes(n INTEGER); CREATE TEMP TRIGGER count_event_update AFTER UPDATE ON usage_events BEGIN INSERT INTO writes VALUES(1); END`); err != nil {
		t.Fatal(err)
	}
	if added, changed, err := upsertTx(ctx, p, e); err != nil || added || changed {
		t.Fatalf("repeat=%v/%v %v", added, changed, err)
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM writes").Scan(&count); err != nil || count != 0 {
		t.Fatalf("no-op writes=%d %v", count, err)
	}
	e.SourceOffset = 5
	if _, changed, err := upsertTx(ctx, p, e); err != nil || changed {
		t.Fatalf("offset-only logical change=%v %v", changed, err)
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM writes").Scan(&count); err != nil || count != 1 {
		t.Fatalf("offset write=%d %v", count, err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('z','id',10,10,'')`); err != nil {
		t.Fatal(err)
	}
	e.SourcePath = "a"
	e.Tokens["input_tokens"] = 99
	if _, changed, err := upsertTx(ctx, p, e); err != nil || changed {
		t.Fatalf("lower owner=%v %v", changed, err)
	}
	var tokens int
	if err = tx.QueryRowContext(ctx, "SELECT input_tokens FROM usage_events WHERE event_key='one'").Scan(&tokens); err != nil || tokens != 10 {
		t.Fatalf("tokens=%d %v", tokens, err)
	}
}
