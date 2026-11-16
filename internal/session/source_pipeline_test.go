package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func writeSessionSource(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// sessionPipelineCorpus covers appends, a rewrite, a rename that preserves
// ownership by identity, multi-session sources and an incomplete tail.
func sessionPipelineCorpus(t *testing.T, home string, round int) {
	t.Helper()
	for i := range 20 {
		path := filepath.Join(home, ".codex", "sessions", fmt.Sprintf("r%02d.jsonl", i))
		lines := []string{fmt.Sprintf(`{"type":"session_meta","payload":{"session_id":"s%02d","cwd":"/work/p%d"}}`, i, i%4)}
		for turn := range round + i%3 {
			at := fmt.Sprintf("2026-10-08T12:%02d:%02dZ", turn, i)
			lines = append(lines,
				fmt.Sprintf(`{"type":"visible_user_prompt","timestamp":"%s","payload":{"text":"prompt %d %d needle"}}`, at, i, turn),
				fmt.Sprintf(`{"type":"visible_assistant_final","timestamp":"%s","payload":{"text":"reply %d %d"}}`, at, i, turn))
		}
		if i%7 == 3 {
			lines = append(lines, `{"type":"session_meta","payload":{"session_id":"second-`+fmt.Sprint(i)+`"}}`,
				`{"type":"visible_user_prompt","timestamp":"2026-10-08T13:00:00Z","payload":{"text":"other session"}}`)
		}
		body := strings.Join(lines, "\n") + "\n"
		if i == 6 {
			body += `{"type":"visible_user_prompt","payload":{"text":"unfinished`
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if round > 1 {
		writeSessionSource(t, filepath.Join(home, ".codex", "sessions", "r01.jsonl"),
			`{"type":"session_meta","payload":{"session_id":"rewritten"}}`,
			`{"type":"visible_user_prompt","payload":{"text":"after rewrite"}}`)
		archived := filepath.Join(home, ".codex", "archived_sessions", "r02.jsonl")
		if err := os.MkdirAll(filepath.Dir(archived), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(filepath.Join(home, ".codex", "sessions", "r02.jsonl"), archived); err != nil {
			t.Fatal(err)
		}
	}
}

func sessionPipelineRows(t *testing.T, database *store.Store) sessionIndexSnapshot {
	t.Helper()
	return sessionIndexSnapshot{
		sources:  snapshotSessionRows(t, database.DB, `SELECT source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,parser_context FROM session_sources ORDER BY source_path`),
		metadata: snapshotSessionRows(t, database.DB, `SELECT source_path,client,session_id,project,model,parser_version,first_at,last_at FROM session_metadata ORDER BY source_path,client,session_id`),
		// Results of a multi-session source are emitted from a map, so their
		// relative insertion order varies even between serial runs; order
		// within each session is compared exactly.
		documents:  snapshotSessionRows(t, database.DB, `SELECT source_path,client,session_id,event_at,kind,text FROM session_documents ORDER BY source_path,client,session_id,rowid`),
		exclusions: snapshotSessionRows(t, database.DB, `SELECT kind,value FROM session_exclusions ORDER BY kind,value`),
	}
}

// Parallel reduce with one ordered writer publishes exactly the serial index,
// including FTS row order, across a cold import and an incremental round.
func TestParallelReduceMatchesSerialSessionIndex(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	databases := map[int]*store.Store{}
	for _, workers := range []int{1, 4, 8} {
		database, err := store.OpenSessions(ctx, filepath.Join(root, fmt.Sprintf("state-%d", workers)))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { database.Close() })
		databases[workers] = database
	}
	for round := 1; round <= 2; round++ {
		sessionPipelineCorpus(t, home, round)
		results := map[int]ScanResult{}
		for workers, database := range databases {
			result, err := ScanWithOptions(ctx, database.DB, home, ScanOptions{ReduceWorkers: workers})
			if err != nil {
				t.Fatalf("round %d workers %d: %v", round, workers, err)
			}
			results[workers] = result
		}
		serial := sessionPipelineRows(t, databases[1])
		if len(serial.documents) == 0 {
			t.Fatalf("round %d indexed no documents", round)
		}
		for _, workers := range []int{4, 8} {
			if results[workers] != results[1] {
				t.Fatalf("round %d workers %d result = %+v, serial %+v", round, workers, results[workers], results[1])
			}
			got := sessionPipelineRows(t, databases[workers])
			for name, pair := range map[string][2][]string{"sources": {got.sources, serial.sources}, "metadata": {got.metadata, serial.metadata}, "documents": {got.documents, serial.documents}, "exclusions": {got.exclusions, serial.exclusions}} {
				if !reflect.DeepEqual(pair[0], pair[1]) {
					t.Fatalf("round %d workers %d %s differ: %s", round, workers, name, firstDifference(pair[0], pair[1]))
				}
			}
		}
	}
}

func firstDifference(got, want []string) string {
	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			return fmt.Sprintf("row %d:\nparallel %s\nserial   %s", i, got[i], want[i])
		}
	}
	return fmt.Sprintf("parallel %d rows, serial %d rows", len(got), len(want))
}

// A reduce result whose committed source state advanced before its turn in
// the writer is recomputed from the committed state, never published stale.
func TestWriterRecomputesStaleReduceResult(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "home", ".codex", "sessions", "a.jsonl")
	lines := []string{`{"type":"session_meta","payload":{"session_id":"s"}}`, `{"type":"visible_user_prompt","payload":{"text":"one"}}`}
	writeSessionSource(t, path, lines...)
	stale, fresh := filepath.Join(root, "stale"), filepath.Join(root, "fresh")
	databases := map[string]*store.Store{}
	for _, dir := range []string{stale, fresh} {
		database, err := store.OpenSessions(ctx, dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { database.Close() })
		databases[dir] = database
		if _, err = Scan(ctx, database.DB, filepath.Join(root, "home")); err != nil {
			t.Fatal(err)
		}
	}
	writeSessionSource(t, path, append(lines, `{"type":"visible_user_prompt","payload":{"text":"two"}}`)...)
	sources, err := sessionSources(filepath.Join(root, "home"), nil)
	if err != nil {
		t.Fatal(err)
	}
	update, changed, err := prepareSourceUpdate(ctx, databases[stale].DB, sources[0], nil)
	if err != nil || !changed {
		t.Fatalf("prepare changed=%v err=%v", changed, err)
	}
	// Another publication advances the checkpoint the reduce started from.
	writeSessionSource(t, path, append(lines, `{"type":"visible_user_prompt","payload":{"text":"two"}}`, `{"type":"visible_user_prompt","payload":{"text":"three"}}`)...)
	if _, err = Scan(ctx, databases[stale].DB, filepath.Join(root, "home")); err != nil {
		t.Fatal(err)
	}
	writeSessionSource(t, path, append(lines, `{"type":"visible_user_prompt","payload":{"text":"two"}}`, `{"type":"visible_user_prompt","payload":{"text":"three"}}`, `{"type":"visible_user_prompt","payload":{"text":"four"}}`)...)
	sources, err = sessionSources(filepath.Join(root, "home"), nil)
	if err != nil {
		t.Fatal(err)
	}
	writer := &sessionWriter{ctx: ctx, db: databases[stale].DB}
	if _, err = writer.write(&preparedUpdate{src: sources[0], update: update, changed: changed}, false); err != nil {
		t.Fatal(err)
	}
	if err = writer.commit(); err != nil {
		t.Fatal(err)
	}
	if _, err = Scan(ctx, databases[fresh].DB, filepath.Join(root, "home")); err != nil {
		t.Fatal(err)
	}
	got, want := sessionPipelineRows(t, databases[stale]), sessionPipelineRows(t, databases[fresh])
	if !reflect.DeepEqual(got.documents, want.documents) || !reflect.DeepEqual(got.sources, want.sources) {
		t.Fatalf("stale result was published:\ngot  %v\nwant %v", got.documents, want.documents)
	}
}

// A new source with more documents than one write transaction commits them in
// blocks that stay unsearchable until the last block; the result equals the
// serial single-transaction import, also after a crash before the last block.
func TestLargeNewSourceBlocksMatchSingleTransaction(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	lines := []string{`{"type":"session_meta","payload":{"session_id":"large","cwd":"/work/large"}}`}
	for i := range 1300 {
		at := fmt.Sprintf("2026-10-08T%02d:%02d:%02dZ", 12+i/3600, i/60%60, i%60)
		lines = append(lines, fmt.Sprintf(`{"type":"visible_user_prompt","timestamp":"%s","payload":{"text":"prompt %d needle"}}`, at, i))
		if i == 400 {
			lines = append(lines, `{"type":"session_meta","payload":{"session_id":"second"}}`)
		}
	}
	writeSessionSource(t, filepath.Join(home, ".codex", "sessions", "large.jsonl"), lines...)
	open := func(name string) *store.Store {
		database, err := store.OpenSessions(ctx, filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { database.Close() })
		return database
	}
	blocked, single := open("blocked"), open("single")
	if _, err := blocked.DB.Exec(`CREATE TRIGGER reject_source BEFORE INSERT ON session_sources BEGIN SELECT RAISE(ABORT,'synthetic crash'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(ctx, blocked.DB, home); err == nil {
		t.Fatal("scan succeeded without its source row")
	}
	var prefix int
	if err := blocked.DB.QueryRow(`SELECT count(*) FROM session_documents`).Scan(&prefix); err != nil || prefix <= writeTxRows || prefix >= 1300 {
		t.Fatalf("prefix documents=%d err=%v", prefix, err)
	}
	if found, err := Search(ctx, blocked.DB, "needle"); err != nil || len(found) != 0 {
		t.Fatalf("prefix searchable before its source row: %d %v", len(found), err)
	}
	if _, err := blocked.DB.Exec(`DROP TRIGGER reject_source`); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(ctx, blocked.DB, home); err != nil {
		t.Fatal(err)
	}
	sources, err := sessionSources(home, nil)
	if err != nil {
		t.Fatal(err)
	}
	sources[0].noOrphanDocuments = true
	tx, err := single.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = scanSourceExec(ctx, tx, sources[0]); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	want := sessionPipelineRows(t, single)
	if len(want.documents) <= 2*writeTxRows {
		t.Fatalf("fixture fits in blocks: %d", len(want.documents))
	}
	got := sessionPipelineRows(t, blocked)
	if !reflect.DeepEqual(got.documents, want.documents) || !reflect.DeepEqual(got.metadata, want.metadata) || !reflect.DeepEqual(got.sources, want.sources) {
		t.Fatalf("recovered blocked import differs: %s", firstDifference(got.documents, want.documents))
	}
}

// A multi-session source appends within its tail session and falls back to a
// full parse when an appended record continues an earlier session; either way
// the index equals a full scan of the final file.
func TestMultiSessionAppendMatchesFullScan(t *testing.T) {
	ctx := context.Background()
	base := []string{
		`{"type":"session_meta","payload":{"session_id":"first","cwd":"/work/excluded-first"}}`,
		`{"type":"visible_user_prompt","timestamp":"2026-10-08T12:00:00Z","payload":{"text":"first prompt"}}`,
		`{"type":"session_meta","payload":{"session_id":"tail","cwd":"/work/tail"}}`,
		`{"type":"visible_user_prompt","timestamp":"2026-10-08T12:01:00Z","payload":{"text":"tail prompt"}}`,
	}
	for name, appended := range map[string][]string{
		"tail":    {`{"type":"visible_assistant_final","timestamp":"2026-10-08T12:02:00Z","payload":{"text":"tail reply"}}`},
		"earlier": {`{"type":"session_meta","payload":{"session_id":"first"}}`, `{"type":"visible_assistant_final","timestamp":"2026-10-08T12:03:00Z","payload":{"text":"first reply"}}`},
		"new":     {`{"type":"session_meta","payload":{"session_id":"third"}}`, `{"type":"visible_user_prompt","timestamp":"2026-10-08T12:04:00Z","payload":{"text":"third prompt"}}`},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home")
			path := filepath.Join(home, ".codex", "sessions", "multi.jsonl")
			incremental, err := store.OpenSessions(ctx, filepath.Join(root, "incremental"))
			if err != nil {
				t.Fatal(err)
			}
			defer incremental.Close()
			if _, err = incremental.DB.Exec(`INSERT INTO session_exclusions(kind,value) VALUES('project','/work/excluded-first')`); err != nil {
				t.Fatal(err)
			}
			writeSessionSource(t, path, base...)
			if _, err = Scan(ctx, incremental.DB, home); err != nil {
				t.Fatal(err)
			}
			state, _, err := loadSource(ctx, incremental.DB, path)
			if err != nil || !parserContextMulti(state.parserContext) || decodeParserContext(state.parserContext).SessionID != "tail" {
				t.Fatalf("multi-session context = %q %v", state.parserContext, err)
			}
			writeSessionSource(t, path, append(append([]string{}, base...), appended...)...)
			update, changed, err := prepareSourceUpdate(ctx, incremental.DB, source{client: "codex", path: path}, nil)
			if err != nil || !changed {
				t.Fatalf("prepare changed=%v err=%v", changed, err)
			}
			if update.appendOnly != (name == "tail") {
				t.Fatalf("appendOnly=%v for a %s append", update.appendOnly, name)
			}
			if _, err = Scan(ctx, incremental.DB, home); err != nil {
				t.Fatal(err)
			}
			full, err := store.OpenSessions(ctx, filepath.Join(root, "full"))
			if err != nil {
				t.Fatal(err)
			}
			defer full.Close()
			if _, err = full.DB.Exec(`INSERT INTO session_exclusions(kind,value) VALUES('project','/work/excluded-first')`); err != nil {
				t.Fatal(err)
			}
			if _, err = Scan(ctx, full.DB, home); err != nil {
				t.Fatal(err)
			}
			got, want := sessionPipelineRows(t, incremental), sessionPipelineRows(t, full)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s append differs from a full scan:\nmetadata %v\nwant     %v\ndocuments %v\nwant      %v", name, got.metadata, want.metadata, got.documents, want.documents)
			}
		})
	}
}
