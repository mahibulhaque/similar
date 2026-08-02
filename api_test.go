package similar_test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestPublicAPI snapshots the exported surface of package similar: every
// exported type, function, method, constant, and variable, rendered as a
// signature. A diff here means the public API changed — intended or not.
//
// It guards two failure modes the rest of the suite cannot see. A symbol
// silently dropped from the surface still compiles everywhere it is not used,
// and a symbol accidentally exported (one capital letter in a root file) becomes
// supported API the moment it ships. Doc comments are deliberately excluded so
// that rewording a comment does not fail the test: a guard that cries wolf gets
// -update'd without being read.
//
// Run with -update to accept a change.
func TestPublicAPI(t *testing.T) {
	got := strings.Join(exportedAPI(t, "."), "\n") + "\n"
	path := filepath.Join("testdata", "api.golden")

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update): %v", err)
	}
	if got != string(want) {
		t.Fatalf("public API changed.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// exportedAPI parses the non-test Go files in dir and returns one sorted entry
// per exported declaration.
func exportedAPI(t *testing.T, dir string) []string {
	t.Helper()

	fset := token.NewFileSet()
	names, err := goFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	var entries []string
	for _, name := range names {
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		// Trim the file to its exported declarations. This also drops unexported
		// struct fields and interface methods, so only the parts of a type a
		// caller can reach are recorded.
		ast.FileExports(file)
		entries = append(entries, declEntries(fset, file)...)
	}

	slices.Sort(entries)
	return entries
}

// goFiles returns the non-test .go file names in dir, sorted.
func goFiles(dir string) ([]string, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range dirEntries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// declEntries renders one entry per exported declaration in file.
func declEntries(fset *token.FileSet, file *ast.File) []string {
	var entries []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if entry, ok := funcEntry(fset, d); ok {
				entries = append(entries, entry)
			}
		case *ast.GenDecl:
			// A const block states its type and value only on the first spec; the
			// rest inherit both. Carry them along so every constant records its
			// own type and ordinal — otherwise reordering a block, which changes
			// the values callers see, would leave this snapshot untouched.
			var inherited constContext
			for i, spec := range d.Specs {
				if d.Tok == token.CONST {
					inherited.observe(fset, spec, i)
				}
				if entry, ok := specEntry(fset, d.Tok, spec, inherited); ok {
					entries = append(entries, entry)
				}
			}
		}
	}
	return entries
}

// funcEntry renders a function or method signature, dropping the body. A method
// is included only when its receiver type is also exported: ast.FileExports
// keeps exported methods on unexported types, and those are not public API.
func funcEntry(fset *token.FileSet, fn *ast.FuncDecl) (string, bool) {
	if !fn.Name.IsExported() {
		return "", false
	}
	if fn.Recv != nil && !receiverIsExported(fn.Recv) {
		return "", false
	}
	sig := *fn
	sig.Doc = nil
	sig.Body = nil
	return render(fset, &sig), true
}

// receiverIsExported reports whether a method's receiver base type is exported.
func receiverIsExported(recv *ast.FieldList) bool {
	if len(recv.List) == 0 {
		return false
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	// A generic receiver is written T[P]; the base type is the indexed operand.
	switch e := expr.(type) {
	case *ast.IndexExpr:
		expr = e.X
	case *ast.IndexListExpr:
		expr = e.X
	}
	ident, ok := expr.(*ast.Ident)
	return ok && ident.IsExported()
}

// constContext carries what a const spec inherits from earlier specs in its
// block: the last type written down, and the last value expression together with
// the position it appeared at.
type constContext struct {
	typ       string
	value     string
	valueAt   int
	position  int
	inherited bool
}

// observe folds the spec at index i into the context.
func (c *constContext) observe(fset *token.FileSet, spec ast.Spec, i int) {
	c.position = i
	value, ok := spec.(*ast.ValueSpec)
	if !ok {
		return
	}
	if value.Type != nil {
		c.typ = render(fset, value.Type)
	}
	if len(value.Values) > 0 {
		c.value = render(fset, value.Values[0])
		c.valueAt = i
		c.inherited = false
		return
	}
	c.inherited = true
}

// implicitValue renders what a valueless const spec actually equals: an iota
// block counts up from where the iota appeared, and any other repeated
// expression is reported as a repeat with its position.
func (c constContext) implicitValue() string {
	if c.value == "iota" {
		if offset := c.position - c.valueAt; offset > 0 {
			return fmt.Sprintf("iota + %d", offset)
		}
		return "iota"
	}
	return fmt.Sprintf("%s [implicit, position %d]", c.value, c.position)
}

// specEntry renders a type, const, or var spec. Import specs are skipped.
func specEntry(fset *token.FileSet, tok token.Token, spec ast.Spec, inherited constContext) (string, bool) {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		if !s.Name.IsExported() {
			return "", false
		}
		clean := *s
		clean.Doc = nil
		clean.Comment = nil
		return "type " + render(fset, &clean), true
	case *ast.ValueSpec:
		if !slices.ContainsFunc(s.Names, (*ast.Ident).IsExported) {
			return "", false
		}
		clean := *s
		clean.Doc = nil
		clean.Comment = nil
		entry := tok.String() + " " + render(fset, &clean)
		if tok == token.CONST && inherited.inherited {
			entry += " " + inherited.typ + " = " + inherited.implicitValue()
		}
		return entry, true
	default:
		return "", false
	}
}

// render prints an AST node on one line, collapsing the whitespace the printer
// uses for alignment so that gofumpt-driven reformatting cannot move the golden.
func render(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := (&printer.Config{Mode: printer.RawFormat}).Fprint(&buf, fset, node); err != nil {
		return "!render error: " + err.Error()
	}
	one := strings.Join(strings.Fields(buf.String()), " ")
	// A signature broken across lines in the source collapses to "( ctx ..., )";
	// tighten the punctuation so the entry reads like the declaration would.
	for _, r := range [...][2]string{{"( ", "("}, {" )", ")"}, {" ,", ","}, {",)", ")"}, {"[ ", "["}, {" ]", "]"}} {
		one = strings.ReplaceAll(one, r[0], r[1])
	}
	return one
}
