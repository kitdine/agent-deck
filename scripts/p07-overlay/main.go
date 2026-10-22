// p07-overlay produces instrumented copies and a Go -overlay manifest. It never
// edits product files or starts the product. Type information distinguishes
// sql.Tx.Commit from unrelated Commit methods, including SQLite backup.Commit.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const measureImport = "github.com/kitdine/agent-deck/scripts/p07measure"

type pkg struct {
	ImportPath, Dir, Export string
	GoFiles, Imports        []string
}
type edit struct {
	start, end int
	text       string
}
type site struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Operation string `json:"operation"`
}
type identity struct{ Original, Instrumented string }

func sqlType(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "database/sql" {
		return n.Obj().Name()
	}
	if n, ok := t.(*types.Named); ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "os/exec" && n.Obj().Name() == "Cmd" {
		return "Cmd"
	}
	return ""
}

func operationFor(kind, name string) string {
	switch kind {
	case "Cmd":
		switch name {
		case "Start", "Wait", "Run", "Output", "CombinedOutput":
			return "Command" + name
		}
	case "DB":
		switch name {
		case "Begin", "BeginTx", "Conn":
			return name
		case "Close":
			return "CloseDB"
		case "ExecContext":
			return "ExecDB"
		case "Exec":
			return "ExecDBNoContext"
		case "QueryRowContext":
			return "QueryRowSQL"
		case "QueryRow":
			return "QueryRowDBNoContext"
		case "QueryContext":
			return "QuerySQL"
		case "Query":
			return "QueryDBNoContext"
		case "PrepareContext":
			return "PrepareSQL"
		case "Prepare":
			return "PrepareDBNoContext"
		}
	case "Conn":
		switch name {
		case "BeginTx":
			return name
		case "ExecContext":
			return "ExecConn"
		case "QueryRowContext":
			return "QueryRowSQL"
		case "QueryContext":
			return "QuerySQL"
		case "PrepareContext":
			return "PrepareSQL"
		case "Close":
			return "CloseConn"
		}
	case "Tx":
		if name == "Commit" || name == "Rollback" {
			return name
		}
	case "Row":
		if name == "Scan" {
			return "ScanRow"
		}
	}
	return ""
}

func siteArgument(operation string) bool {
	switch operation {
	case "Open", "Conn", "CloseDB", "CloseConn", "Commit", "Rollback", "ScanRow":
		return false
	}
	return true
}

func rewrite(fset *token.FileSet, file *ast.File, info *types.Info, input []byte, relative string) ([]byte, []site, error) {
	var edits []edit
	var sites []site
	called := map[*ast.SelectorExpr]bool{}
	var failure error
	insertions := map[int][]string{}
	offset := func(pos token.Pos) int { return fset.Position(pos).Offset }
	if file.Name.Name == "main" {
		for _, declaration := range file.Decls {
			if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Name.Name == "main" && fn.Recv == nil && fn.Body != nil {
				pos := offset(fn.Body.Lbrace) + 1
				edits = append(edits, edit{pos, pos, "\np07measure.ProcessStart(); defer p07measure.ProcessEnd();\n"})
				sites = append(sites, site{relative, fset.Position(fn.Pos()).Line, "ProcessLifecycle"})
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		kind := ""
		if selection := info.Selections[sel]; selection != nil {
			kind = sqlType(selection.Recv())
			if selection.Kind() == types.MethodExpr && kind != "" {
				failure = fmt.Errorf("unsupported SQL method expression at %s", fset.Position(sel.Pos()))
				return true
			}
		}
		name := sel.Sel.Name
		operation := operationFor(kind, name)
		if obj, ok := info.Uses[sel.Sel].(*types.Func); ok && obj.Pkg() != nil && obj.Pkg().Path() == "database/sql" && obj.Name() == "Open" {
			operation = "Open"
		}
		if obj, ok := info.Uses[sel.Sel].(*types.Func); ok && obj.Pkg() != nil && obj.Pkg().Path() == "os" && obj.Name() == "Exit" {
			operation = "Exit"
		}
		if operation == "" {
			return true
		}
		called[sel] = true
		line := fset.Position(sel.Pos()).Line
		prefix := "p07measure." + operation + "("
		if operation == "Open" || operation == "Exit" {
			edits = append(edits, edit{offset(call.Pos()), offset(call.Lparen) + 1, prefix})
		} else {
			if siteArgument(operation) {
				prefix += strconv.Quote(fmt.Sprintf("%s:%d", relative, line)) + ", "
			}
			// Preserve the receiver expression in place. Prefixes at a shared
			// position follow outer-to-inner AST order, so chained QueryRow.Scan
			// rewrites both boundaries without duplicating/eagerly executing it.
			pos := offset(call.Pos())
			insertions[pos] = append(insertions[pos], prefix)
			separator := ""
			if len(call.Args) != 0 {
				separator = ", "
			}
			edits = append(edits, edit{offset(sel.X.End()), offset(call.Lparen) + 1, separator})
		}
		sites = append(sites, site{relative, line, operation})
		return true
	})
	// A method captured as a value could escape instrumentation. Refuse rather
	// than silently claim exhaustive coverage when the source shape changes.
	for sel, selection := range info.Selections {
		if sel.Pos() < file.Pos() || sel.End() > file.End() {
			continue
		}
		kind, name := sqlType(selection.Recv()), sel.Sel.Name
		if operationFor(kind, name) != "" && !called[sel] {
			failure = fmt.Errorf("unhandled SQL method value at %s", fset.Position(sel.Pos()))
		}
	}
	if failure != nil {
		return nil, nil, failure
	}
	if len(edits) == 0 {
		return input, nil, nil
	}
	for pos, values := range insertions {
		edits = append(edits, edit{pos, pos, strings.Join(values, "")})
	}
	for _, imp := range file.Imports {
		if imp.Name != nil && imp.Name.Name == "p07measure" {
			return nil, nil, fmt.Errorf("instrumentation import collision: %s", relative)
		}
		if imp.Path.Value == `"database/sql"` && imp.Name != nil && imp.Name.Name == "." {
			return nil, nil, fmt.Errorf("SQL dot import unsupported: %s", relative)
		}
	}
	edits = append(edits, edit{offset(file.Name.End()), offset(file.Name.End()), "\nimport p07measure " + strconv.Quote(measureImport) + "\n"})
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].start == edits[j].start {
			return edits[i].end > edits[j].end
		}
		return edits[i].start > edits[j].start
	})
	output := append([]byte(nil), input...)
	last := len(output) + 1
	for _, e := range edits {
		if e.end > last {
			return nil, nil, fmt.Errorf("nested instrumentation expressions unsupported: %s", relative)
		}
		output = append(append(append([]byte(nil), output[:e.start]...), []byte(e.text)...), output[e.end:]...)
		last = e.start
	}
	// Replacing the only sql.Open call can leave its import unused. Preserve
	// initialization with a blank import; retain SQL types/constants if used.
	parsedSet := token.NewFileSet()
	parsed, err := parser.ParseFile(parsedSet, relative, output, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	for _, imp := range parsed.Imports {
		if imp.Path.Value != `"database/sql"` {
			continue
		}
		name := "sql"
		if imp.Name != nil {
			name = imp.Name.Name
		}
		used := false
		ast.Inspect(parsed, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == name {
					used = true
				}
			}
			return true
		})
		if !used {
			imp.Name = ast.NewIdent("_")
		}
	}
	var formatted bytes.Buffer
	err = format.Node(&formatted, parsedSet, parsed)
	return formatted.Bytes(), sites, err
}

func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func writeJSON(name string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(name, append(b, '\n'), 0600)
}

func run() error {
	root := flag.String("repo", ".", "verified workspace root")
	out := flag.String("out", "", "new private output directory (must not exist)")
	flag.Parse()
	if *out == "" {
		return fmt.Errorf("-out is required")
	}
	return generate(*root, *out)
}

func generate(root, out string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	output, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	if err = os.Mkdir(output, 0700); err != nil {
		return err
	}
	cmd := exec.Command("go", "list", "-mod=vendor", "-deps", "-export", "-json", "./internal/...", "./cmd/agentdeck")
	cmd.Dir = abs
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("go list failed: %w: %s", err, stderr.String())
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	packages := []pkg{}
	exports := map[string]string{}
	for {
		var p pkg
		if err = decoder.Decode(&p); err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		packages = append(packages, p)
		exports[p.ImportPath] = p.Export
	}
	fset := token.NewFileSet()
	lookup := func(path string) (io.ReadCloser, error) {
		name := exports[path]
		if name == "" {
			return nil, fmt.Errorf("missing export for %s", path)
		}
		return os.Open(name)
	}
	imp := importer.ForCompiler(fset, "gc", lookup)
	replacements := map[string]string{}
	identities := map[string]identity{}
	var sites []site
	for _, p := range packages {
		if !strings.HasPrefix(p.Dir, abs+string(filepath.Separator)) {
			continue
		}
		hasObservedAPI := false
		for _, i := range p.Imports {
			if i == "database/sql" || i == "os/exec" {
				hasObservedAPI = true
			}
		}
		if !hasObservedAPI {
			continue
		}
		files := []*ast.File{}
		inputs := map[*ast.File][]byte{}
		names := map[*ast.File]string{}
		for _, name := range p.GoFiles {
			path := filepath.Join(p.Dir, name)
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			f, e := parser.ParseFile(fset, path, b, 0)
			if e != nil {
				return e
			}
			files = append(files, f)
			inputs[f] = b
			names[f] = path
		}
		info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}, Uses: map[*ast.Ident]types.Object{}}
		conf := types.Config{Importer: imp}
		if _, err = conf.Check(p.ImportPath, fset, files, info); err != nil {
			return err
		}
		for _, f := range files {
			rel, e := filepath.Rel(abs, names[f])
			if e != nil {
				return e
			}
			instrumented, found, e := rewrite(fset, f, info, inputs[f], filepath.ToSlash(rel))
			if e != nil {
				return e
			}
			if len(found) == 0 {
				continue
			}
			dest := filepath.Join(output, rel)
			if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
				return e
			}
			if e = os.WriteFile(dest, instrumented, 0600); e != nil {
				return e
			}
			replacements[names[f]] = dest
			identities[filepath.ToSlash(rel)] = identity{digest(inputs[f]), digest(instrumented)}
			sites = append(sites, found...)
		}
	}
	if len(sites) == 0 {
		return fmt.Errorf("no SQL sites found")
	}
	sort.Slice(sites, func(i, j int) bool {
		if sites[i].File == sites[j].File {
			return sites[i].Line < sites[j].Line
		}
		return sites[i].File < sites[j].File
	})
	if err = writeJSON(filepath.Join(output, "overlay.json"), map[string]any{"Replace": replacements}); err != nil {
		return err
	}
	return writeJSON(filepath.Join(output, "manifest.json"), map[string]any{"schema": "p07-overlay/v2", "files": identities, "sites": sites, "scope": "explicit DB/Conn transactions and autocommit boundaries; typed os/exec lifecycle in SQL or exec-importing workspace packages, including checked-in dependencies and packages without SQL; unknown SQL and unobserved endpoints invalidate coverage; test sources excluded; original product and vendor files unchanged"})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
