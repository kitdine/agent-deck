package usage

import (
	"context"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestSessionRebuildPreservesUnchangedRowsAndRemovesEmptySessions(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash,session_id,session_started_at) VALUES('/source','identity',0,0,'','s','2026-10-20T00:00:00Z')`,
		`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES('first','codex','s','first','2026-10-21T00:00:00Z','m','/source',0)`,
	} {
		if _, err = db.DB.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	affected := map[string][2]string{"codex\x00s": {"codex", "s"}}
	rebuild := func() {
		t.Helper()
		tx, err := db.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err = rebuildSessions(ctx, tx, affected); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	rebuild()
	if _, err = db.DB.ExecContext(ctx, `CREATE TRIGGER no_unchanged_delete BEFORE DELETE ON usage_sessions BEGIN SELECT RAISE(ABORT,'unchanged session was deleted');END; CREATE TRIGGER no_unchanged_update BEFORE UPDATE ON usage_sessions BEGIN SELECT RAISE(ABORT,'unchanged session was updated');END;`); err != nil {
		t.Fatal(err)
	}
	rebuild()
	if _, err = db.DB.ExecContext(ctx, `DROP TRIGGER no_unchanged_delete; DROP TRIGGER no_unchanged_update; INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,source_path,source_offset) VALUES('late','codex','s','late','2026-10-22T00:00:00Z','m','/source',10)`); err != nil {
		t.Fatal(err)
	}
	rebuild()
	var first, last, started string
	if err = db.DB.QueryRowContext(ctx, `SELECT first_at,last_at,started_at FROM usage_sessions WHERE client='codex' AND session_id='s'`).Scan(&first, &last, &started); err != nil {
		t.Fatal(err)
	}
	if first != "2026-10-21T00:00:00Z" || last != "2026-10-22T00:00:00Z" || started != "2026-10-20T00:00:00Z" {
		t.Fatalf("incorrect aggregate %s %s %s", first, last, started)
	}
	if _, err = db.DB.ExecContext(ctx, `DELETE FROM usage_events`); err != nil {
		t.Fatal(err)
	}
	rebuild()
	var count int
	if err = db.DB.QueryRowContext(ctx, `SELECT count(*) FROM usage_sessions`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("empty session remains: %d %v", count, err)
	}
}
