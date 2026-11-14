package session

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

// All files and databases below are isolated fixtures under t.TempDir().

func sessionRevision(t *testing.T, db *store.Store) int64 {
	t.Helper()
	snapshot, err := ReadIndexSnapshot(context.Background(), db.DB, "")
	if err != nil {
		t.Fatal(err)
	}
	return snapshot.Revision
}

// session_documents is FTS5 and cannot carry a trigger. Every path that
// changes searchable text must still advance the session serving revision,
// or a scope selection could pair a search with a different index state.
func TestEveryDocumentWritePathAdvancesTheSessionRevision(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	active := filepath.Join(home, ".codex", "sessions", "a.jsonl")
	archive := filepath.Join(home, ".codex", "archived_sessions", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(active), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(archive), 0o700); err != nil {
		t.Fatal(err)
	}
	line := func(text string) string {
		return "{\"type\":\"visible_user_prompt\",\"session_id\":\"s\",\"payload\":{\"text\":\"" + text + "\"}}\n"
	}
	db, err := store.OpenSessions(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	step := func(name, query string, matches int, change func()) {
		t.Helper()
		before := sessionRevision(t, db)
		change()
		if after := sessionRevision(t, db); after <= before {
			t.Fatalf("%s left the revision at %d", name, after)
		}
		if query == "" {
			return
		}
		snapshot, err := ReadIndexSnapshot(ctx, db.DB, query)
		if err != nil || len(snapshot.Documents) != matches {
			t.Fatalf("%s: %d matches, %v", name, len(snapshot.Documents), err)
		}
	}
	scan := func() {
		t.Helper()
		if _, err := Scan(ctx, db.DB, home); err != nil {
			t.Fatal(err)
		}
	}
	step("new source", "before", 1, func() {
		if err := os.WriteFile(active, []byte(line("before")), 0o600); err != nil {
			t.Fatal(err)
		}
		scan()
	})
	step("append", "appended", 1, func() {
		f, err := os.OpenFile(active, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.WriteString(line("appended"))
		_ = f.Close()
		scan()
	})
	step("rewrite", "rewritten", 1, func() {
		if err := os.WriteFile(active, []byte(line("rewritten")), 0o600); err != nil {
			t.Fatal(err)
		}
		scan()
	})
	step("move", "rewritten", 1, func() {
		if err := os.Rename(active, archive); err != nil {
			t.Fatal(err)
		}
		scan()
	})
	step("replace documents", "replaced", 1, func() {
		if err := ReplaceDocuments(ctx, db.DB, "claude", "r", []Document{{Client: "claude", SessionID: "r", Kind: "user", Text: "replaced"}}); err != nil {
			t.Fatal(err)
		}
	})
	step("exclude session", "replaced", 0, func() {
		if err := Exclude(ctx, db.DB, "session", "r"); err != nil {
			t.Fatal(err)
		}
	})
	step("rebuild", "rewritten", 1, func() {
		if _, err := Rebuild(ctx, db.DB, home); err != nil {
			t.Fatal(err)
		}
	})
	step("delete", "rewritten", 0, func() {
		if err := os.Remove(archive); err != nil {
			t.Fatal(err)
		}
		scan()
	})
}

func TestIndexSnapshotMatchesListAndSearch(t *testing.T) {
	ctx := context.Background()
	db, err := store.OpenSessions(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, row := range []struct {
		path, identity, prefix, client, id, project, first, last string
		priority                                                 int
	}{
		{"/a/1.jsonl", "i1", "p1", "claude", "s1", "/w/a", "2026-10-29T08:00:00Z", "2026-10-29T09:00:00Z", 1},
		{"/b/1.jsonl", "i2", "p2", "claude", "s1", "/w/b", "2026-10-29T07:00:00Z", "2026-10-29T10:00:00Z", 0},
		{"/a/2.jsonl", "i3", "p3", "codex", "s1", "", "2026-10-29T06:00:00Z", "2026-10-29T06:30:00Z", 1},
	} {
		if _, err := db.DB.Exec(`INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,0,0,0,?,?,7,'t')`, row.path, row.identity, row.prefix, row.priority); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.Exec(`INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at) VALUES(?,?,?,?,'m',7,?,?)`, row.path, row.client, row.id, row.project, row.first, row.last); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.Exec(`INSERT INTO session_documents(source_path,client,session_id,event_at,kind,text) VALUES(?,?,?,?,'user','shared text')`, row.path, row.client, row.id, row.first); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := ReadIndexSnapshot(ctx, db.DB, "shared")
	if err != nil {
		t.Fatal(err)
	}
	listed, err := List(ctx, db.DB)
	if err != nil {
		t.Fatal(err)
	}
	var metadata []Metadata
	for _, item := range snapshot.Sessions {
		metadata = append(metadata, item.Metadata)
	}
	if !reflect.DeepEqual(metadata, listed) {
		t.Fatalf("snapshot %+v, List %+v", metadata, listed)
	}
	// Every copy of a canonical session is carried for its input token.
	if got := snapshot.Sessions[0].Copies; snapshot.Sessions[0].SourcePath != "/a/1.jsonl" || len(got) != 2 ||
		got[0] != (SourceCopy{Identity: "i1", PrefixHash: "p1", ParserVersion: 7, FirstAt: "2026-10-29T08:00:00Z"}) || got[1].Identity != "i2" {
		t.Fatalf("copies %+v", snapshot.Sessions[0])
	}
	searched, err := Search(ctx, db.DB, "shared")
	if err != nil || !reflect.DeepEqual(snapshot.Documents, searched) || len(searched) != 2 {
		t.Fatalf("snapshot search %+v, Search %+v %v", snapshot.Documents, searched, err)
	}
	if _, err := ReadIndexSnapshot(ctx, db.DB, "  "); err == nil {
		t.Fatal("blank query accepted")
	}
}

func TestProjectGroupKeepsIdentityAndLabelSeparate(t *testing.T) {
	for value, want := range map[string][2]string{
		"":                                       {"", ""},
		"/":                                      {"", ""},
		" /w/alpha/ ":                            {"/w/alpha", "alpha"},
		`C:\w\beta`:                              {"C:/w/beta", "beta"},
		".":                                      {".", ""},
		"/tmp/Referenced-ChatGPT-Conversation-1": {"chatgpt-work://referenced-conversation", "ChatGPT Work"},
	} {
		identity, label := ProjectGroup(value)
		if identity != want[0] || label != want[1] {
			t.Fatalf("%q = %q/%q, want %q/%q", value, identity, label, want[0], want[1])
		}
	}
}
