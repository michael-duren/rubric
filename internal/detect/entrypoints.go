package detect

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"

	"github.com/michael-duren/go-skills/internal/config"
)

func (s *scan) source(rel string, src []byte) error {
	f, err := parser.ParseFile(token.NewFileSet(), rel, src, parser.SkipObjectResolution)
	if err != nil {
		s.add("warning", "unparsable Go source", rel)
		return nil
	}
	if dir := path.Dir(rel); f.Name.Name != "main" && dir != "." {
		s.add("package", dir, dir)
	}
	if f.Name.Name == "main" && hasMainFunc(f) {
		dir := path.Dir(rel)
		s.facts.EntryPoints = append(s.facts.EntryPoints, config.EntryPoint{Name: s.entryName(dir), Dir: dir})
	}
	return s.classify(rel, f)
}

func hasMainFunc(f *ast.File) bool {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == "main" && fn.Type.Params.NumFields() == 0 && fn.Type.Results == nil {
			return true
		}
	}
	return false
}
