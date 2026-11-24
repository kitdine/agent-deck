package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestCJSIViewerOrder(t *testing.T) {
	m := session.Metadata{Client: "codex", SessionID: "synthetic", SourcePath: t.TempDir() + "/synthetic.jsonl"}
	want := [][]string{{"A", "X"}, {"B", "C"}, {}}
	for i, labels := range want {
		page, err := loadViewerDocuments(t.Context(), m, i+1, 2, func(_ context.Context, selected session.Metadata, page, limit int) ([]session.Document, session.Pagination, error) {
			if selected != m || page != i+1 || limit != 2 {
				t.Fatalf("viewer requested wrong session or page: %v %d %d", selected, page, limit)
			}
			var docs []session.Document
			for _, label := range labels {
				docs = append(docs, session.Document{Text: label})
			}
			return docs, session.Pagination{Page: page, Limit: limit, Total: 4, Shown: len(docs), HasMore: page == 1}, nil
		})
		if err != nil || page.Total != 4 || page.Page != i+1 || !page.Stale {
			t.Fatalf("page %d: %#v, %v", i+1, page, err)
		}
		var got []string
		for _, row := range page.Rows {
			got = append(got, row.Value)
		}
		if !slices.Equal(got, labels) {
			t.Fatalf("rendered order %v, want %v", got, labels)
		}
	}
	t.Run("projected source and failed read", func(t *testing.T) {
		m := session.Metadata{Client: "codex", SessionID: "projected", SourcePath: session.TelemetrySourcePath("codex", "projected")}
		load := func(problem error) (sessionViewerPage, error) {
			return loadViewerDocuments(t.Context(), m, 1, 10, func(context.Context, session.Metadata, int, int) ([]session.Document, session.Pagination, error) {
				return []session.Document{{Text: "provisional"}}, session.Pagination{Page: 1, Total: 1, Limit: 10}, problem
			})
		}
		p, err := load(nil)
		if err != nil || p.Stale || p.Total != 1 {
			t.Fatalf("virtual source page=%+v err=%v", p, err)
		}
		failure := errors.New("selection expired")
		p, err = load(failure)
		if !errors.Is(err, failure) || !reflect.DeepEqual(p, sessionViewerPage{}) {
			t.Fatalf("failed read returned page=%+v err=%v", p, err)
		}
	})
	t.Run("legacy continuation", testLegacySessionContinuation)
}

func testLegacySessionContinuation(t *testing.T) {
	for _, operation := range []string{"unrelated write", "zero source scan"} {
		t.Run(operation, func(t *testing.T) {
			ctx := t.Context()
			state := t.TempDir()
			db, err := store.OpenSessions(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			const id = "selected"
			var want []session.Document
			for _, text := range []string{"A", "B", "C"} {
				want = append(want, session.Document{Client: "codex", SessionID: id, Kind: "user_prompt", Text: text})
			}
			if err := session.ReplaceDocuments(ctx, db.DB, "codex", id, want); err != nil {
				t.Fatal(err)
			}
			if operation == "zero source scan" {
				// A scan of an empty inventory will remove this unrelated source.
				home := t.TempDir()
				path := filepath.Join(home, ".codex", "sessions", "unrelated.jsonl")
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("{\"type\":\"visible_user_prompt\",\"session_id\":\"other\",\"payload\":{\"text\":\"other\"}}\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if r, err := session.Scan(ctx, db.DB, home); err != nil || r.Documents != 1 {
					t.Fatalf("seed unrelated file = %+v, %v", r, err)
				}
			}
			old, err := session.ShowMetadata(ctx, db.DB, "codex", id)
			if err != nil {
				t.Fatal(err)
			}
			load := newSessionViewerLoad(ctx, db, old, nil)
			first, err := load(ctx, viewerDocuments, 1, 2)
			if err != nil || first.Total != 3 || len(first.Rows) != 2 || first.Rows[0].Value != "A" || first.Rows[1].Value != "B" {
				t.Fatalf("first page = %+v, %v", first, err)
			}
			reader := openSessionSelectionReader(ctx, state, db)
			defer reader.Close()
			snap, err := reader.selectSessions(ctx, sessionorigin.Selection{Scope: sessionorigin.ScopeAll}, nil)
			if err != nil {
				t.Fatal(err)
			}
			browser := newSessionBrowserPage(sessionorigin.ScopeAll, snap)
			i := slices.IndexFunc(browser.items, func(m session.Metadata) bool { return m.SessionID == id })
			if i < 0 {
				t.Fatal("selected session missing from browser")
			}
			browserLoad := newSessionViewerLoad(ctx, db, browser.items[i], nil)
			switch operation {
			case "unrelated write":
				if err := session.ReplaceDocuments(ctx, db.DB, "claude", "elsewhere", []session.Document{{Client: "claude", SessionID: "elsewhere", Kind: "user_prompt", Text: "unrelated"}}); err != nil {
					t.Fatal(err)
				}
			case "zero source scan":
				if r, err := session.Scan(ctx, db.DB, t.TempDir()); err != nil || r.Sources != 0 || r.Skipped != 0 || r.Removed != 1 {
					t.Fatalf("empty inventory scan = %+v, %v", r, err)
				}
			}
			for name, read := range map[string]sessionViewerLoad{"viewer": load, "browser": browserLoad} {
				p, err := read(ctx, viewerDocuments, 2, 2)
				if err != nil || p.Total != 3 || len(p.Rows) != 1 || p.Rows[0].Value != "C" {
					t.Errorf("%s page 2 using original selection = %+v, %v", name, p, err)
				}
			}
			// The non-interactive show --all reader uses its already selected metadata.
			docs, p, err := session.DocumentsPage(ctx, db.DB, old, 1, 2, true)
			if err != nil || p.Total != 3 || !slices.Equal(docs, want) {
				t.Errorf("whole read using original selection = %v, %+v, %v", docs, p, err)
			}
			cli := runOriginCLI(t, state, "--format", "json", "session", "show", id, "--client", "codex", "--all")
			var result struct{ Data session.Result }
			if err := json.Unmarshal([]byte(cli.stdout), &result); cli.exit != 0 || err != nil || !slices.Equal(result.Data.Documents, want) {
				t.Errorf("show --all = %+v, %v", cli, err)
			}
		})
	}
}
