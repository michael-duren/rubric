// Package style checks Go source against Rubric's mechanical comment and documentation rules.
package style

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxDocLines = 2
	maxDocRunes = 150
)

var directive = regexp.MustCompile(`^//(go:[a-z][a-z0-9_]*\b|line \S|export \S|extern \S|nolint\b)|^// \+build `)

// Finding is one policy violation at a source line.
type Finding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

type analyzer struct {
	fset     *token.FileSet
	name     string
	test     bool
	docs     map[*ast.CommentGroup]bool
	findings []Finding
}

// AnalyzeFile checks one file's source; parse failures are returned as errors, not findings.
func AnalyzeFile(name string, src []byte) ([]Finding, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	if ast.IsGenerated(f) {
		return nil, nil
	}
	a := &analyzer{fset: fset, name: name, test: strings.HasSuffix(name, "_test.go"), docs: map[*ast.CommentGroup]bool{}}
	if f.Doc != nil {
		a.docs[f.Doc] = true
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			a.doc(d.Doc)
			if d.Doc == nil && a.exportedFunc(d) {
				a.add(d.Pos(), "doc-missing", fmt.Sprintf("exported %s needs a documentation comment", d.Name.Name))
			}
		case *ast.GenDecl:
			a.genDecl(d)
		}
	}
	for _, group := range f.Comments {
		if a.docs[group] || group.End() < f.Package {
			continue
		}
		for _, c := range group.List {
			if !directive.MatchString(c.Text) {
				a.add(c.Pos(), "comment", "explanatory comments are not allowed; remove it or move the explanation into declaration documentation")
				break
			}
		}
	}
	slices.SortFunc(a.findings, compareFindings)
	return a.findings, nil
}

func compareFindings(x, y Finding) int {
	return cmp.Or(cmp.Compare(x.File, y.File), cmp.Compare(x.Line, y.Line), cmp.Compare(x.Rule, y.Rule))
}

func (a *analyzer) add(pos token.Pos, rule, message string) {
	a.findings = append(a.findings, Finding{File: a.name, Line: a.fset.Position(pos).Line, Rule: rule, Message: message})
}

func (a *analyzer) doc(group *ast.CommentGroup) {
	if group == nil {
		return
	}
	a.docs[group] = true
	var text []string
	for _, c := range group.List {
		if directive.MatchString(c.Text) {
			continue
		}
		for i, line := range strings.Split(c.Text, "\n") {
			text = append(text, line)
			if utf8.RuneCountInString(line) > maxDocRunes {
				a.add(c.Pos()+token.Pos(lineOffset(c.Text, i)), "doc-length",
					fmt.Sprintf("documentation line has %d characters; the limit is %d", utf8.RuneCountInString(line), maxDocRunes))
			}
		}
	}
	lines := len(text)
	if hasDirective(group) {
		for lines > 0 && strings.TrimSpace(text[lines-1]) == "//" {
			lines--
		}
	}
	if lines > maxDocLines {
		a.add(group.Pos(), "doc-lines", fmt.Sprintf("documentation has %d lines; the limit is %d", lines, maxDocLines))
	}
}

func hasDirective(group *ast.CommentGroup) bool {
	return slices.ContainsFunc(group.List, func(c *ast.Comment) bool { return directive.MatchString(c.Text) })
}

func lineOffset(text string, line int) int {
	offset := 0
	for range line {
		offset += strings.IndexByte(text[offset:], '\n') + 1
	}
	return offset
}

func (a *analyzer) genDecl(d *ast.GenDecl) {
	a.doc(d.Doc)
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			a.doc(s.Doc)
			a.fields(s.Type)
			if s.Name.IsExported() && d.Doc == nil && s.Doc == nil {
				a.add(s.Pos(), "doc-missing", fmt.Sprintf("exported type %s needs a documentation comment", s.Name.Name))
			}
		case *ast.ValueSpec:
			a.doc(s.Doc)
			for _, name := range s.Names {
				if name.IsExported() && d.Doc == nil && s.Doc == nil {
					a.add(name.Pos(), "doc-missing", fmt.Sprintf("exported %s needs a documentation comment", name.Name))
					break
				}
			}
		}
	}
}

func (a *analyzer) fields(expr ast.Expr) {
	var list *ast.FieldList
	switch t := expr.(type) {
	case *ast.StructType:
		list = t.Fields
	case *ast.InterfaceType:
		list = t.Methods
	}
	if list == nil {
		return
	}
	for _, field := range list.List {
		a.doc(field.Doc)
		a.fields(field.Type)
	}
}

func (a *analyzer) exportedFunc(d *ast.FuncDecl) bool {
	if !d.Name.IsExported() {
		return false
	}
	if d.Recv != nil {
		return len(d.Recv.List) == 1 && receiverName(d.Recv.List[0].Type).IsExported()
	}
	return !a.test || !testEntryPoint(d)
}

func receiverName(expr ast.Expr) *ast.Ident {
	for {
		switch t := expr.(type) {
		case *ast.StarExpr:
			expr = t.X
		case *ast.IndexExpr:
			expr = t.X
		case *ast.IndexListExpr:
			expr = t.X
		case *ast.ParenExpr:
			expr = t.X
		case *ast.Ident:
			return t
		default:
			return ast.NewIdent("_")
		}
	}
}

func testEntryPoint(d *ast.FuncDecl) bool {
	params := d.Type.Params.NumFields()
	if d.Type.Results != nil || d.Type.TypeParams != nil {
		return false
	}
	name := d.Name.Name
	if name == "TestMain" {
		return params == 1
	}
	for prefix, want := range map[string]int{"Test": 1, "Benchmark": 1, "Fuzz": 1, "Example": 0} {
		if rest, ok := strings.CutPrefix(name, prefix); ok && params == want {
			r, _ := utf8.DecodeRuneInString(rest)
			return rest == "" && prefix == "Example" || rest != "" && !unicode.IsLower(r) || rest == "" && prefix != "Example"
		}
	}
	return false
}
