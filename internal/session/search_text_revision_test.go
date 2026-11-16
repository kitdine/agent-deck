package session

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

// Searchable text is a selection input, and the FTS5 content table carries no
// trigger (one there makes every content write flush and merge the index). So
// each product path that inserts, replaces or deletes session_documents rows
// advances the session serving revision itself, in the same transaction.
func TestEveryProductWriteOfSearchTextAdvancesTheServingRevision(t *testing.T) {
	ctx := context.Background()
	index, err := store.OpenSessions(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	var triggers int
	if err := index.DB.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='trigger' AND tbl_name='session_documents_content'`).Scan(&triggers); err != nil || triggers != 0 {
		t.Fatalf("content table triggers = %d %v, want none", triggers, err)
	}
	// Drop every other revision trigger, so only the explicit advance can
	// move the revision in the steps below.
	rows, err := index.DB.Query(`SELECT name FROM sqlite_master WHERE type='trigger' AND sql LIKE '%serving_generation SET revision%'`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	rows.Close()
	for _, name := range names {
		if _, err := index.DB.Exec(`DROP TRIGGER "` + name + `"`); err != nil {
			t.Fatal(err)
		}
	}
	revision := func() int64 {
		t.Helper()
		var value int64
		if err := index.DB.QueryRow(`SELECT revision FROM serving_generation WHERE singleton=1`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	documents := func() int {
		t.Helper()
		var count int
		if err := index.DB.QueryRow(`SELECT count(*) FROM session_documents`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	step := func(name string, wantDocuments int, write func() error) {
		t.Helper()
		before := revision()
		if err := write(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := documents(); got != wantDocuments {
			t.Fatalf("%s: %d documents, want %d", name, got, wantDocuments)
		}
		if after := revision(); after <= before {
			t.Fatalf("%s wrote searchable text without advancing the serving revision (%d -> %d)", name, before, after)
		}
	}
	doc := func(text string) []Document {
		return []Document{{Client: "claude", SessionID: "s-1", EventAt: "2026-11-01T08:00:00Z", Kind: "user_prompt", Text: text}}
	}
	step("insert", 1, func() error { return ReplaceDocuments(ctx, index.DB, "claude", "s-1", doc("first needle")) })
	step("replace", 1, func() error { return ReplaceDocuments(ctx, index.DB, "claude", "s-1", doc("second needle")) })
	step("move source", 1, func() error {
		tx, err := index.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err := moveSource(ctx, tx, replaceDocumentsSourcePath, "/moved.jsonl"); err != nil {
			return err
		}
		return tx.Commit()
	})
	step("delete source", 0, func() error {
		tx, err := index.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err := deleteSource(ctx, tx, "/moved.jsonl"); err != nil {
			return err
		}
		return tx.Commit()
	})
	step("insert again", 1, func() error { return ReplaceDocuments(ctx, index.DB, "claude", "s-1", doc("third needle")) })
	step("exclude session", 0, func() error { return Exclude(ctx, index.DB, "session", "s-1") })

	// The remaining write points. Each step leaves the table empty, so the
	// next one starts from its own insert.
	insert := func(name string, result Result) {
		t.Helper()
		step(name, 1, func() error {
			tx, err := index.DB.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()
			source := sourceState{path: result.SourcePath, identity: result.SourcePath, priority: 1, parserVersion: ParserVersion, partial: []byte{}}
			if err := saveSource(ctx, tx, source); err != nil {
				return err
			}
			if err := replaceExec(ctx, tx, result); err != nil {
				return err
			}
			return tx.Commit()
		})
	}
	result := func(client, sessionID, project, path string) Result {
		return Result{
			Metadata:  Metadata{Client: client, SessionID: sessionID, Project: project, SourcePath: path},
			Documents: []Document{{Client: client, SessionID: sessionID, EventAt: "2026-11-01T08:00:00Z", Kind: "user_prompt", Text: "needle"}},
		}
	}
	// A replacement with no documents reaches only replaceExec's own delete:
	// insertDocuments writes nothing and so advances nothing.
	insert("insert for empty replace", result("claude", "s-2", "", replaceDocumentsSourcePath))
	step("replace with no documents", 0, func() error { return ReplaceDocuments(ctx, index.DB, "claude", "s-2", nil) })
	insert("insert for project exclusion", result("claude", "s-3", "/work/project", "/project.jsonl"))
	step("exclude project", 0, func() error { return Exclude(ctx, index.DB, "project", "/work/project") })
	insert("insert for path exclusion", result("claude", "s-4", "", "/path.jsonl"))
	step("exclude path", 0, func() error { return Exclude(ctx, index.DB, "path", "/path.jsonl") })
	insert("insert for client exclusion", result("codex", "s-5", "", "/client.jsonl"))
	step("exclude client", 0, func() error { return Exclude(ctx, index.DB, "client", "codex") })
	// An appended record whose session is now excluded removes that session's
	// earlier projection of the same source.
	insert("insert for late exclusion", result("claude", "s-1", "", "/late.jsonl"))
	step("append to an excluded session", 0, func() error {
		tx, err := index.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		update := sourceUpdate{path: "/late.jsonl", appendOnly: true, writeState: true,
			state:   sourceState{path: "/late.jsonl", identity: "late", priority: 1, parserVersion: ParserVersion, partial: []byte{}},
			results: []Result{result("claude", "s-1", "", "/late.jsonl")}}
		if _, err := applySourceUpdate(ctx, tx, update); err != nil {
			return err
		}
		return tx.Commit()
	})
	// An append inserts without a preceding delete, so only insertDocuments'
	// own advance covers it.
	step("append documents", 1, func() error {
		tx, err := index.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		update := sourceUpdate{path: "/append.jsonl", appendOnly: true, writeState: true,
			state:   sourceState{path: "/append.jsonl", identity: "append", priority: 1, parserVersion: ParserVersion, partial: []byte{}},
			results: []Result{result("claude", "s-7", "", "/append.jsonl")}}
		if _, err := applySourceUpdate(ctx, tx, update); err != nil {
			return err
		}
		return tx.Commit()
	})
	step("exclude appended path", 0, func() error { return Exclude(ctx, index.DB, "path", "/append.jsonl") })
	insert("insert for rebuild", result("claude", "s-6", "", "/rebuild.jsonl"))
	step("rebuild", 0, func() error {
		_, err := Rebuild(ctx, index.DB, t.TempDir())
		return err
	})
}

// The steps above exercise the write points that exist today. This check
// covers the ones that do not exist yet: in this package every statement that
// inserts into, updates or deletes from session_documents either carries the
// searchTextChanged prefix or sits in insertDocuments, which advances the
// revision before its inserts. No other product package writes the table.
func TestEverySearchTextStatementCarriesTheRevisionAdvance(t *testing.T) {
	write := regexp.MustCompile(`(?i)\b(INSERT\s+(OR\s+\w+\s+)?INTO|UPDATE|DELETE\s+FROM|REPLACE\s+INTO)\s+session_documents\b`)
	literal := func(expr ast.Expr) (string, bool) {
		basic, ok := expr.(*ast.BasicLit)
		if !ok || basic.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(basic.Value)
		return value, err == nil
	}
	callsMark := func(function *ast.FuncDecl) bool {
		found := false
		ast.Inspect(function, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "MarkSearchTextChanged" {
					found = true
				}
			}
			return true
		})
		return found
	}
	files := token.NewFileSet()
	checked := 0
	check := func(dir string, own bool) {
		t.Helper()
		packages, err := parser.ParseDir(files, dir, func(info fs.FileInfo) bool { return !strings.HasSuffix(info.Name(), "_test.go") }, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, pkg := range packages {
			for _, file := range pkg.Files {
				prefixed := map[ast.Expr]bool{}
				ast.Inspect(file, func(node ast.Node) bool {
					if binary, ok := node.(*ast.BinaryExpr); ok && binary.Op == token.ADD {
						if name, ok := binary.X.(*ast.Ident); ok && name.Name == "searchTextChanged" {
							prefixed[binary.Y] = true
						}
					}
					return true
				})
				for _, declaration := range file.Decls {
					function, _ := declaration.(*ast.FuncDecl)
					ast.Inspect(declaration, func(node ast.Node) bool {
						expr, ok := node.(ast.Expr)
						if !ok {
							return true
						}
						text, ok := literal(expr)
						if !ok || !write.MatchString(text) {
							return true
						}
						checked++
						position := files.Position(expr.Pos())
						switch {
						case !own:
							t.Errorf("%s writes session_documents outside the session package", position)
						case prefixed[expr]:
						case function != nil && function.Name.Name == "insertDocuments" && callsMark(function):
						default:
							t.Errorf("%s writes session_documents without the searchTextChanged prefix", position)
						}
						return true
					})
				}
			}
		}
	}
	check(".", true)
	if checked < 10 {
		t.Fatalf("found %d session_documents write statements in this package, want at least the 10 known ones", checked)
	}
	root := filepath.Join("..", "..")
	for _, tree := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || !entry.IsDir() {
				return err
			}
			if absolute, _ := filepath.Abs(path); absolute == mustAbs(t, ".") {
				return nil
			}
			check(path, false)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}
