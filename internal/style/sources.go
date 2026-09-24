package style

import (
	"embed"
	"go/parser"
	"go/token"
	"slices"
	"strings"
)

//go:embed analyze.go tree.go analyze_test.go tree_test.go main.go.tmpl
var assets embed.FS

var bundled = []string{"analyze.go", "analyze_test.go", "main.go.tmpl", "tree.go", "tree_test.go"}

// Source is one file of the standalone analyzer written to .rubric/style/.
type Source struct {
	Name string
	Data []byte
}

// Sources returns the analyzer, its tests, and a thin main, all rewritten as package main.
func Sources() ([]Source, error) {
	out := make([]Source, 0, len(bundled))
	for _, name := range bundled {
		data, err := assets.ReadFile(name)
		if err != nil {
			return nil, err
		}
		if name, ok := strings.CutSuffix(name, ".tmpl"); ok {
			out = append(out, Source{Name: name, Data: data})
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, data, parser.PackageClauseOnly)
		if err != nil {
			return nil, err
		}
		start := fset.Position(f.Name.Pos()).Offset
		end := start + len(f.Name.Name)
		out = append(out, Source{Name: name, Data: slices.Concat(data[:start], []byte("main"), data[end:])})
	}
	return out, nil
}
