package style

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourcesAreStandaloneMainPackage(t *testing.T) {
	sources, err := Sources()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range sources {
		names = append(names, s.Name)
		f, err := parser.ParseFile(token.NewFileSet(), s.Name, s.Data, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		if f.Name.Name != "main" {
			t.Fatalf("%s package = %s", s.Name, f.Name.Name)
		}
		for _, imp := range f.Imports {
			if strings.Contains(imp.Path.Value, ".") && !strings.HasPrefix(imp.Path.Value, `"go/`) {
				t.Fatalf("%s imports non-stdlib %s", s.Name, imp.Path.Value)
			}
		}
	}
	if strings.Join(names, ",") != "analyze.go,analyze_test.go,main.go,tree.go,tree_test.go" {
		t.Fatalf("sources = %v", names)
	}
	for _, s := range sources {
		if s.Name == "analyze_test.go" && !strings.Contains(string(s.Data), `"package sample\n"`) {
			t.Fatal("rewrite touched string literals")
		}
	}
}

func TestSourcesRunStandalone(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}
	sources, err := Sources()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/fixture\n\ngo 1.26.7\n")
	for _, s := range sources {
		write(".rubric/style/"+s.Name, string(s.Data))
	}
	write("app/app.go", "package app\n\n// Run runs.\nfunc Run() {}\n")
	run := func(args ...string) (string, error) {
		cmd := exec.CommandContext(t.Context(), "go", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("test", "./.rubric/style"); err != nil {
		t.Fatalf("bundled tests: %v\n%s", err, out)
	}
	if out, err := run("run", "./.rubric/style", "."); err != nil {
		t.Fatalf("clean tree: %v\n%s", err, out)
	}
	write("app/bad.go", "package app\n\nfunc Bad() {}\n")
	out, err := run("run", "./.rubric/style", ".")
	if err == nil || !strings.Contains(out, "app/bad.go:3: doc-missing") {
		t.Fatalf("violation not reported: %v\n%s", err, out)
	}
}
