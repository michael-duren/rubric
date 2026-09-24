package render_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/catalog"
	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/style"
	"github.com/michael-duren/go-skills/internal/testproject"
)

func TestStyleEveryGeneratedTemplate(t *testing.T) {
	for _, starter := range []string{"module", "runnable"} {
		for _, f := range catalog.Cases() {
			if starter == "runnable" && (f.HTTP != "none" || f.CLI != "none" || f.TUI != "none") {
				continue
			}
			c := testproject.Config()
			c.Project.Starter = starter
			c.Features = f
			c.Tooling.Lint = true
			files, err := render.Files(c, "new")
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range files {
				if !strings.HasSuffix(file.Path, ".go") {
					continue
				}
				findings, err := style.AnalyzeFile(file.Path, file.Data)
				if err != nil {
					t.Fatalf("%+v %s: %v", f, file.Path, err)
				}
				if len(findings) > 0 {
					t.Fatalf("%+v: %s violates the style policy: %+v", f, file.Path, findings)
				}
			}
		}
	}
}

func TestStyleAnalyzerOnlyWithLint(t *testing.T) {
	for _, lint := range []bool{false, true} {
		for _, mode := range []string{"new", "existing"} {
			c := testproject.Config()
			c.Tooling.Lint = lint
			if mode == "existing" {
				c.Project.Go = "1.22"
			}
			files, err := render.Files(c, mode)
			if err != nil {
				t.Fatal(err)
			}
			_, has := find(files, ".rubric/style/analyze.go")
			if has != lint {
				t.Fatalf("lint=%v mode=%s analyzer present=%v", lint, mode, has)
			}
			if lint {
				f, _ := find(files, ".rubric/style/analyze_test.go")
				if f.Kind != render.KindManaged {
					t.Fatalf("analyzer tests kind = %q", f.Kind)
				}
			}
		}
	}
}

func TestGeneratedAnalyzer(t *testing.T) {
	if testing.Short() {
		t.Skip("runs generated tooling")
	}
	c := testproject.Config()
	c.Features.HTTP, c.Features.CLI, c.Features.Database, c.Features.Access = "chi", "cobra", "sqlite", "sqlc"
	c.Tooling.Lint = true
	files, err := render.Files(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	root := testproject.Write(t, files)
	testproject.Go(t, root, "mod", "tidy")
	testproject.Go(t, root, "test", "./.rubric/style")
	testproject.Go(t, root, "run", "./.rubric/style", ".")
	bad := filepath.Join(root, "internal", "httpserver", "extra.go")
	if err := os.WriteFile(bad, []byte("package httpserver\n\nfunc Extra() {\n\t// why\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "go", "run", "./.rubric/style", ".")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("violations not reported:\n%s", out)
	}
	for _, want := range []string{"internal/httpserver/extra.go:3: doc-missing", "internal/httpserver/extra.go:4: comment"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in:\n%s", want, fmt.Sprint(string(out)))
		}
	}
}
