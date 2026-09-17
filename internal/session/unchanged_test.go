package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestUnchangedSourcesRequireCompleteCurrentCheckpoints(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "sessions", "s.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	contents := []byte(`{"type":"visible_user_prompt","session_id":"s","payload":{"text":"original"}}` + "\n")
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenSessions(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := Scan(ctx, db.DB, home); err != nil {
		t.Fatal(err)
	}
	paths, err := sessionSources(home, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || !paths[0].stable {
		t.Skip("stable file generations unavailable")
	}
	if ok, err := unchangedSources(ctx, db.DB, paths); err != nil || !ok {
		t.Fatalf("unchanged=%v, %v", ok, err)
	}
	for _, column := range []string{"changed_at", "parser_version", "cursor"} {
		var original int64
		if err := db.DB.QueryRow("SELECT " + column + " FROM session_sources").Scan(&original); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.Exec("UPDATE session_sources SET " + column + "=0"); err != nil {
			t.Fatal(err)
		}
		if ok, err := unchangedSources(ctx, db.DB, paths); err != nil || ok {
			t.Fatalf("missing %s checkpoint accepted: %v, %v", column, ok, err)
		}
		if _, err := db.DB.Exec("UPDATE session_sources SET "+column+"=?", original); err != nil {
			t.Fatal(err)
		}
	}
	// Pure growth after discovery does not invalidate the already committed
	// captured range; a fresh inventory will schedule the suffix next time.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(contents, contents...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if ok, err := unchangedSources(ctx, db.DB, paths); err != nil || !ok {
		t.Fatalf("captured range rejected append growth: %v, %v", ok, err)
	}
	refreshed, err := sessionSources(home, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := unchangedSources(ctx, db.DB, refreshed); err != nil || ok {
		t.Fatalf("fresh inventory skipped appended suffix: %v, %v", ok, err)
	}
}

func TestUnchangedSourcesRejectRewriteAfterDiscovery(t *testing.T) {
	for _, test := range []struct {
		name    string
		rewrite func([]byte) []byte
	}{
		{name: "equal size", rewrite: func(contents []byte) []byte {
			return []byte(strings.Replace(string(contents), "original", "modified", 1))
		}},
		{name: "rewrite plus growth", rewrite: func(contents []byte) []byte {
			changed := strings.Replace(string(contents), "original", "modified", 1)
			return []byte(changed + `{"type":"visible_user_prompt","session_id":"s","payload":{"text":"later"}}` + "\n")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			home := t.TempDir()
			path := filepath.Join(home, ".codex", "sessions", "s.jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			contents := []byte(`{"type":"visible_user_prompt","session_id":"s","payload":{"text":"original"}}` + "\n")
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			db, err := store.OpenSessions(ctx, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err = Scan(ctx, db.DB, home); err != nil {
				t.Fatal(err)
			}
			paths, err := sessionSources(home, nil)
			if err != nil || len(paths) != 1 || !paths[0].stable {
				t.Skipf("stable file generation unavailable: paths=%#v err=%v", paths, err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, test.rewrite(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if err = os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
			if unchanged, validateErr := unchangedSources(ctx, db.DB, paths); unchanged || !errors.Is(validateErr, ingest.ErrSourceChanged) {
				t.Fatalf("rewrite accepted: unchanged=%t err=%v", unchanged, validateErr)
			}
		})
	}
}

func TestUnchangedScanRereadsPreservedMtimeRewriteBeyondAnchor(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "sessions", "s.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	original := strings.Repeat(" ", 5000) + `{"type":"visible_user_prompt","session_id":"s","payload":{"text":"original"}}` + "\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenSessions(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := Scan(ctx, db.DB, home); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(original, "original", "modified", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(ctx, db.DB, home); err != nil {
		t.Fatal(err)
	}
	got, err := Search(ctx, db.DB, "modified")
	if err != nil || len(got) != 1 {
		t.Fatalf("same-size rewrite not indexed: %v, %v", got, err)
	}
	stale, err := Search(ctx, db.DB, "original")
	if err != nil || len(stale) != 0 {
		t.Fatalf("old text retained: %v, %v", stale, err)
	}
}

func TestBatchedSessionDocumentsPreserveOrderAndRollback(t *testing.T) {
	ctx := context.Background()
	db, err := store.OpenSessions(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	docs := make([]Document, 137)
	for i := range docs {
		docs[i] = Document{Client: "codex", SessionID: "batch", Kind: "user", Text: string(rune('a' + i))}
	}
	if err := ReplaceDocuments(ctx, db.DB, "codex", "batch", docs); err != nil {
		t.Fatal(err)
	}
	before := captureSessionIndex(t, db.DB)
	rows, err := db.DB.Query("SELECT text FROM session_documents ORDER BY rowid")
	if err != nil {
		t.Fatal(err)
	}
	i := 0
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			t.Fatal(err)
		}
		if i >= len(docs) || text != docs[i].Text {
			t.Fatalf("wrong document at %d: %q", i, text)
		}
		i++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if i != len(docs) {
		t.Fatalf("documents=%d", i)
	}
	// Fail after the batches have inserted their FTS rows. The transaction
	// must restore all preexisting FTS and metadata, not retain a partial batch.
	if _, err := db.DB.Exec(`CREATE TRIGGER reject_batch BEFORE INSERT ON session_metadata BEGIN SELECT RAISE(ABORT,'reject batch'); END`); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceDocuments(ctx, db.DB, "codex", "batch", docs[:70]); err == nil {
		t.Fatal("injected metadata failure ignored")
	}
	after := captureSessionIndex(t, db.DB)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed batch changed committed session index")
	}
}
