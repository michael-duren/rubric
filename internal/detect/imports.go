package detect

import (
	"go/ast"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type capability struct {
	field, value string
}

var importCapabilities = map[string]capability{
	"modernc.org/sqlite":                 {"features.database", "sqlite"},
	"github.com/mattn/go-sqlite3":        {"features.database", "sqlite"},
	"github.com/jackc/pgx/v5":            {"features.database", "postgres"},
	"github.com/jackc/pgx/v5/stdlib":     {"features.database", "postgres"},
	"github.com/jackc/pgx/v5/pgxpool":    {"features.database", "postgres"},
	"github.com/lib/pq":                  {"features.database", "postgres"},
	"github.com/spf13/cobra":             {"features.cli", "cobra"},
	"github.com/urfave/cli/v2":           {"features.cli", "urfave-cli"},
	"charm.land/bubbletea/v2":            {"features.tui", "bubbletea"},
	"github.com/charmbracelet/bubbletea": {"features.tui", "bubbletea"},
	"github.com/spf13/viper":             {"features.config", "viper"},
}

var serverCalls = map[string]map[string]string{
	"net/http":                    {"ListenAndServe": "nethttp", "ListenAndServeTLS": "nethttp", "Serve": "nethttp", "ServeTLS": "nethttp", "NewServeMux": "nethttp", "Server": "nethttp"},
	"github.com/go-chi/chi/v5":    {"NewRouter": "chi", "NewMux": "chi"},
	"github.com/go-chi/chi":       {"NewRouter": "chi", "NewMux": "chi"},
	"github.com/gin-gonic/gin":    {"Default": "gin", "New": "gin"},
	"github.com/labstack/echo/v4": {"New": "echo"},
	"github.com/gofiber/fiber/v2": {"New": "fiber"},
}

var majorSuffix = regexp.MustCompile(`^v[0-9]+$`)

func importPaths(f *ast.File) ([]string, error) {
	paths := make([]string, 0, len(f.Imports))
	for _, item := range f.Imports {
		value, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			return nil, err
		}
		paths = append(paths, value)
	}
	slices.Sort(paths)
	return slices.Compact(paths), nil
}

func localNames(f *ast.File) map[string]string {
	names := map[string]string{}
	for _, item := range f.Imports {
		value, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			continue
		}
		if item.Name != nil {
			names[item.Name.Name] = value
			continue
		}
		base := path.Base(value)
		if majorSuffix.MatchString(base) {
			base = path.Base(path.Dir(value))
		}
		base = strings.TrimPrefix(base, "go-")
		names[base] = value
	}
	return names
}

func (s *scan) classify(rel string, f *ast.File) error {
	paths, err := importPaths(f)
	if err != nil {
		return err
	}
	for _, p := range paths {
		s.add("library", p, rel)
		if c, ok := importCapabilities[p]; ok {
			s.add(c.field, c.value, rel)
		}
		if p == "database/sql" {
			s.add("features.access", "sql", rel)
		}
	}
	names := localNames(f)
	ast.Inspect(f, func(n ast.Node) bool {
		var sel *ast.SelectorExpr
		switch n := n.(type) {
		case *ast.CallExpr:
			sel, _ = n.Fun.(*ast.SelectorExpr)
		case *ast.CompositeLit:
			sel, _ = n.Type.(*ast.SelectorExpr)
		}
		if sel == nil {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if value, ok := serverCalls[names[pkg.Name]][sel.Sel.Name]; ok {
			s.add("features.http", value, rel)
		}
		return true
	})
	return nil
}
