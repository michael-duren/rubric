package render_test

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

func TestMinimalStarters(t *testing.T) {
	for _, starter := range []string{"module", "runnable"} {
		t.Run(starter, func(t *testing.T) {
			c := testproject.Config()
			c.Project.Starter = starter
			files, err := render.Files(c, "new")
			if err != nil {
				t.Fatal(err)
			}
			root := testproject.Write(t, files)
			if starter == "runnable" {
				testproject.Go(t, root, "build", "./...")
			}
			if _, err := os.Stat(filepath.Join(root, "main_test.go")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("unnecessary main test: %v", err)
			}
			for _, f := range files {
				if strings.HasSuffix(f.Path, ".go") && starter == "module" {
					t.Fatalf("module-only starter has Go source %s", f.Path)
				}
				if f.Path == "main.go" {
					t.Fatal("runnable starter wrote a root main.go instead of cmd/<name>/main.go")
				}
			}
			if _, err := os.Stat(filepath.Join(root, "cmd", "demo", "main.go")); (err == nil) != (starter == "runnable") {
				t.Fatalf("cmd/demo/main.go presence wrong for %s: %v", starter, err)
			}
			mod := string(testproject.File(t, files, "go.mod"))
			if !strings.HasPrefix(mod, "module example.com/demo\n\ngo 1.26.7\n") {
				t.Fatalf("go.mod = %q", mod)
			}
		})
	}
}

func TestRunnableMainIsMinimal(t *testing.T) {
	c := testproject.Config()
	c.Project.Starter = "runnable"
	files, err := render.Files(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(testproject.File(t, files, "cmd/demo/main.go")); got != "// Command demo is the module entry point.\npackage main\n\nfunc main() {}\n" {
		t.Fatalf("main.go = %q", got)
	}
}

func TestFilesDeterministicSortedUnique(t *testing.T) {
	c := testproject.Config()
	c.Project.Starter = "runnable"
	first, err := render.Files(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		next, err := render.Files(c, "new")
		if err != nil {
			t.Fatal(err)
		}
		if len(next) != len(first) {
			t.Fatal("file count changed")
		}
		for i := range next {
			if next[i].Path != first[i].Path || !bytes.Equal(next[i].Data, first[i].Data) || next[i].Mode != first[i].Mode {
				t.Fatalf("nondeterministic output at %s", next[i].Path)
			}
		}
	}
	for i := 1; i < len(first); i++ {
		if first[i-1].Path >= first[i].Path {
			t.Fatalf("paths not sorted/unique: %s, %s", first[i-1].Path, first[i].Path)
		}
	}
	for _, f := range first {
		if f.Mode != 0o644 {
			t.Fatalf("%s mode %v", f.Path, f.Mode)
		}
		switch f.Kind {
		case "scaffold", "managed", "guidance", "config":
		default:
			t.Fatalf("%s kind %q", f.Path, f.Kind)
		}
	}
}

func TestExistingModeEmitsNoApplicationSource(t *testing.T) {
	c := testproject.Config()
	c.Project.Go = "1.22"
	files, err := render.Files(c, "existing")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "go.mod" || strings.HasSuffix(f.Path, ".go") || f.Kind == "scaffold" {
			t.Fatalf("existing mode rendered %s", f.Path)
		}
	}
}

func TestInvalidModuleRejectedBeforeRendering(t *testing.T) {
	for _, module := range []string{"example.com/a\"b", "example.com/a\nb", "", "example.com/a b"} {
		c := testproject.Config()
		c.Project.Module = module
		if _, err := render.Files(c, "new"); err == nil {
			t.Fatalf("module %q accepted", module)
		}
	}
}

func TestDescriptionIsLiteralData(t *testing.T) {
	c := testproject.Config()
	c.Project.Starter = "runnable"
	c.Project.Description = "Runs $(rm -rf /) `whoami` \"quoted\" 'single' — café ✓"
	files, err := render.Files(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".go") {
			if _, err := parser.ParseFile(token.NewFileSet(), f.Path, f.Data, parser.ParseComments); err != nil {
				t.Fatalf("%s does not parse: %v", f.Path, err)
			}
		}
		if strings.Contains(string(f.Data), "$(rm") && !strings.Contains(string(f.Data), c.Project.Description) {
			t.Fatalf("%s altered description", f.Path)
		}
	}
}

func TestDefaultStarterAlwaysHasAnExecutable(t *testing.T) {
	for _, tt := range []struct {
		module, dir string
	}{
		{"example.com/demo", "cmd/demo/main.go"},
		{"example.com/Shop.API/v2", "cmd/shop.api/main.go"},
		{"example.com/@@@", "cmd/app/main.go"},
		{"tool", "cmd/tool/main.go"},
	} {
		c := config.Defaults()
		c.Project.Module = tt.module
		c.Features.Database, c.Features.Access = "sqlite", "sql"
		files, err := render.Files(c, "new")
		if err != nil {
			if strings.Contains(tt.module, "@") {
				continue
			}
			t.Fatal(err)
		}
		main := string(testproject.File(t, files, tt.dir))
		if !strings.Contains(main, "store.Open") {
			t.Fatalf("%s does not wire the store:\n%s", tt.dir, main)
		}
	}
}
