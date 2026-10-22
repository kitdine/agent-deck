package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type sqlImporter struct{ p *types.Package }

func TestGeneratedOverlayIncludesExecOnlyPackage(t *testing.T) {
	root := t.TempDir()
	for relative, content := range map[string]string{
		"go.mod":                  "module fixture\n\ngo 1.24\n",
		"internal/child/child.go": "package child\nimport \"os/exec\"\nfunc Output()([]byte,error){return exec.Command(\"unused-synthetic-command\").Output()}\n",
		"cmd/agentdeck/main.go":   "package main\nimport \"fixture/internal/child\"\nfunc main(){_,_=child.Output()}\n",
	} {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(root, "overlay")
	if err := generate(root, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Sites []site `json:"sites"`
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, s := range manifest.Sites {
		if s.File == "internal/child/child.go" && s.Operation == "CommandOutput" {
			return
		}
	}
	t.Fatal("exec-only product package omitted from generated overlay")
}

func (i sqlImporter) Import(string) (*types.Package, error) { return i.p, nil }

func testRewrite(t *testing.T, body string) (string, error) {
	t.Helper()
	p := types.NewPackage("database/sql", "sql")
	tx := types.NewNamed(types.NewTypeName(token.NoPos, p, "Tx", nil), types.NewStruct(nil, nil), nil)
	p.Scope().Insert(tx.Obj())
	db := types.NewNamed(types.NewTypeName(token.NoPos, p, "DB", nil), types.NewStruct(nil, nil), nil)
	p.Scope().Insert(db.Obj())
	row := types.NewNamed(types.NewTypeName(token.NoPos, p, "Row", nil), types.NewStruct(nil, nil), nil)
	p.Scope().Insert(row.Obj())
	ptr := func(n *types.Named) *types.Pointer { return types.NewPointer(n) }
	errType := types.Universe.Lookup("error").Type()
	row.AddMethod(types.NewFunc(token.NoPos, p, "Scan", types.NewSignatureType(types.NewVar(token.NoPos, p, "r", ptr(row)), nil, nil, types.NewTuple(types.NewVar(token.NoPos, p, "dest", types.NewSlice(types.NewInterfaceType(nil, nil).Complete()))), types.NewTuple(types.NewVar(token.NoPos, p, "", errType)), true)))
	db.AddMethod(types.NewFunc(token.NoPos, p, "QueryRow", types.NewSignatureType(types.NewVar(token.NoPos, p, "d", ptr(db)), nil, nil, types.NewTuple(types.NewVar(token.NoPos, p, "query", types.Typ[types.String])), types.NewTuple(types.NewVar(token.NoPos, p, "", ptr(row))), false)))
	for _, name := range []string{"Commit", "Rollback"} {
		tx.AddMethod(types.NewFunc(token.NoPos, p, name, types.NewSignatureType(types.NewVar(token.NoPos, p, "t", ptr(tx)), nil, nil, nil, types.NewTuple(types.NewVar(token.NoPos, p, "", errType)), false)))
	}
	db.AddMethod(types.NewFunc(token.NoPos, p, "Begin", types.NewSignatureType(types.NewVar(token.NoPos, p, "d", ptr(db)), nil, nil, nil, types.NewTuple(types.NewVar(token.NoPos, p, "", ptr(tx)), types.NewVar(token.NoPos, p, "", errType)), false)))
	p.Scope().Insert(types.NewFunc(token.NoPos, p, "Open", types.NewSignatureType(nil, nil, nil, nil, types.NewTuple(types.NewVar(token.NoPos, p, "", ptr(db)), types.NewVar(token.NoPos, p, "", errType)), false)))
	p.MarkComplete()
	source := []byte("package sample\nimport \"database/sql\"\n" + body)
	fset := token.NewFileSet()
	f, e := parser.ParseFile(fset, "sample.go", source, 0)
	if e != nil {
		t.Fatal(e)
	}
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}, Uses: map[*ast.Ident]types.Object{}}
	c := types.Config{Importer: sqlImporter{p}}
	if _, e = c.Check("sample", fset, []*ast.File{f}, info); e != nil {
		t.Fatal(e)
	}
	out, _, e := rewrite(fset, f, info, source, "sample.go")
	return string(out), e
}

func TestTypeAwareTerminalRewrite(t *testing.T) {
	out, e := testRewrite(t, `type backup struct{};func(*backup)Commit()int{return 1};func f(db *sql.DB,b *backup){tx,_:=db.Begin();defer tx.Rollback();_ = tx.Commit();_ = b.Commit()}`)
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{`p07measure.Begin("sample.go:3", db)`, `p07measure.Rollback(tx)`, `p07measure.Commit(tx)`, `b.Commit()`} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %s in %s", s, out)
		}
	}
}

func TestCommandLifecycleRewriteAndEscapes(t *testing.T) {
	p := types.NewPackage("os/exec", "exec")
	cmd := types.NewNamed(types.NewTypeName(token.NoPos, p, "Cmd", nil), types.NewStruct(nil, nil), nil)
	p.Scope().Insert(cmd.Obj())
	errType := types.Universe.Lookup("error").Type()
	for _, name := range []string{"Start", "Wait", "Run", "Output", "CombinedOutput"} {
		results := types.NewTuple(types.NewVar(token.NoPos, p, "", errType))
		if name == "Output" || name == "CombinedOutput" {
			results = types.NewTuple(types.NewVar(token.NoPos, p, "", types.NewSlice(types.Typ[types.Byte])), types.NewVar(token.NoPos, p, "", errType))
		}
		cmd.AddMethod(types.NewFunc(token.NoPos, p, name, types.NewSignatureType(types.NewVar(token.NoPos, p, "c", types.NewPointer(cmd)), nil, nil, nil, results, false)))
	}
	p.MarkComplete()
	for _, escaped := range []bool{false, true} {
		body := `func f(c *exec.Cmd){_ = c.Start();_ = c.Wait();_ = c.Run();_,_=c.Output();_,_=c.CombinedOutput()}`
		if escaped {
			body = `func f(c *exec.Cmd){run:=c.Run;_ = run()}`
		}
		source := []byte("package sample\nimport \"os/exec\"\n" + body)
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "sample.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}, Uses: map[*ast.Ident]types.Object{}}
		config := types.Config{Importer: sqlImporter{p}}
		if _, err = config.Check("sample", fset, []*ast.File{f}, info); err != nil {
			t.Fatal(err)
		}
		out, _, err := rewrite(fset, f, info, source, "sample.go")
		if escaped {
			if err == nil {
				t.Fatal("escaped lifecycle command was accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"Start", "Wait", "Run", "Output", "CombinedOutput"} {
			if !strings.Contains(string(out), "p07measure.Command"+name+`("sample.go:3", c)`) {
				t.Fatalf("missing command %s in %s", name, out)
			}
		}
	}
}

func TestRejectEscapingTransactionMethod(t *testing.T) {
	_, e := testRewrite(t, `func f(tx *sql.Tx){end:=tx.Commit;_ = end()}`)
	if e == nil || !strings.Contains(e.Error(), "unhandled SQL method value") {
		t.Fatal(e)
	}
}

func TestOpenOnlyRetainsBlankSQLImport(t *testing.T) {
	out, e := testRewrite(t, `func f(){db,_:=sql.Open();_ = db}`)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out, `_ "database/sql"`) || !strings.Contains(out, `p07measure.Open()`) {
		t.Fatal(out)
	}
}

func TestChainedReturningScanIsNotEagerOrDuplicated(t *testing.T) {
	out, e := testRewrite(t, `func f(db *sql.DB){var n int;_ = db.QueryRow("UPDATE t SET n=n+1 RETURNING n").Scan(&n)}`)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out, `p07measure.ScanRow(p07measure.QueryRowDBNoContext("sample.go:3", db, "UPDATE t SET n=n+1 RETURNING n"), &n)`) {
		t.Fatal(out)
	}
	if strings.Count(out, "QueryRowDBNoContext(") != 1 || strings.Count(out, "ScanRow(") != 1 {
		t.Fatal(out)
	}
}

func TestRejectEscapingScan(t *testing.T) {
	_, e := testRewrite(t, `func f(row *sql.Row){scan:=row.Scan;_ = scan()}`)
	if e == nil || !strings.Contains(e.Error(), "unhandled SQL method value") {
		t.Fatal(e)
	}
}
