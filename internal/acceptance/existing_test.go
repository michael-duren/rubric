package acceptance

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/testproject"
	"github.com/michael-duren/go-skills/internal/write"
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

var existingFixture = map[string]string{
	"go.mod":                   "module example.com/legacy\n\ngo 1.22\n",
	"main.go":                  "package main\n\nimport \"example.com/legacy/internal/greet\"\n\nfunc main() { println(greet.Hello()) }\n",
	"internal/greet/g.go":      "package greet\n\n// Hello greets.\nfunc Hello() string { return \"hi\" } // legacy comment\n",
	"internal/greet/g_test.go": "package greet\n\nimport \"testing\"\n\nfunc TestHello(t *testing.T) {\n\tif Hello() != \"hi\" {\n\t\tt.Fatal()\n\t}\n}\n",
	"AGENTS.md":                "# Team rules\n\nAlways run the linter before pushing.\n",
	"Makefile":                 "test:\n\tgo test ./...\n",
}

func TestExistingProjectPreservation(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, existingFixture)
	before := testproject.Snapshot(t, root)

	r := rubric(t, "init", root, "--makefile", "--format", "json")
	if r.code != 2 || !strings.Contains(r.out, `"path": "Makefile"`) {
		t.Fatalf("user Makefile collision not reported: exit %d\n%s", r.code, r.out)
	}
	if !maps.EqualFunc(before, testproject.Snapshot(t, root), func(a, b []byte) bool { return string(a) == string(b) }) {
		t.Fatal("conflicting run wrote files")
	}

	mustRubric(t, "init", root, "--lint", "--skills", "--actions")
	after := testproject.Snapshot(t, root)
	for name, data := range before {
		switch name {
		case "AGENTS.md":
			if !strings.HasPrefix(string(after[name]), string(data)) || !strings.Contains(string(after[name]), "<!-- rubric:begin -->") {
				t.Fatalf("user guidance not preserved:\n%s", after[name])
			}
		default:
			if string(after[name]) != string(data) {
				t.Fatalf("%s changed", name)
			}
		}
	}
	for _, want := range []string{"rubric.yaml", ".golangci.yml", ".github/workflows/ci.yml", ".agents/skills/rubric-style/SKILL.md", ".rubric/manifest.json"} {
		if _, ok := after[want]; !ok {
			t.Errorf("%s not created", want)
		}
	}
	if _, ok := after["README.md"]; ok {
		t.Error("existing project received a README")
	}
	assertGuidancePaths(t, root)

	r = mustRubric(t, "init", root, "--format", "json")
	var report struct {
		Actions []struct{ Path, State string }
	}
	if err := json.Unmarshal([]byte(r.out), &report); err != nil {
		t.Fatal(err)
	}
	for _, a := range report.Actions {
		if a.State != "unchanged" {
			t.Fatalf("rerun changed %s (%s)", a.Path, a.State)
		}
	}
	out, err := command(t, root, nil, "sh", ".rubric/check.sh", "lint")
	if err == nil || !strings.Contains(out, "internal/greet/g.go:4: comment") {
		t.Fatalf("existing violations not reported by lint (and must not be fixed):\n%s", out)
	}
	if string(testproject.Snapshot(t, root)["internal/greet/g.go"]) != existingFixture["internal/greet/g.go"] {
		t.Fatal("lint rewrote application code")
	}
}

func TestEditedManagedContentConflicts(t *testing.T) {
	root := t.TempDir()
	mustRubric(t, "init", root, "--module", "example.com/demo", "--lint")
	path := filepath.Join(root, ".golangci.yml")
	if err := os.WriteFile(path, []byte("version: \"2\"\n# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := rubric(t, "init", root, "--format", "json")
	if r.code != 2 || !strings.Contains(r.out, "edited") {
		t.Fatalf("edited managed file not a conflict: exit %d\n%s", r.code, r.out)
	}
}

func TestStalePreviewRefusesWrite(t *testing.T) {
	root := t.TempDir()
	req := initialize.Request{Target: root, Mode: "auto", Overrides: config.Patch{"project.module": "example.com/demo"}}
	p, err := initialize.Prepare(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, root, map[string]string{"README.md": "written during review\n"})
	_, err = initialize.Apply(context.Background(), req, p)
	var conflict *write.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v", err)
	}
	if got := testproject.Snapshot(t, root); len(got) != 1 {
		t.Fatalf("stale apply wrote files: %v", got)
	}
}

func TestLiteralPathCharacters(t *testing.T) {
	root := filepath.Join(t.TempDir(), "odd $HOME `id` \"q\" 'single' dir")
	mustRubric(t, "init", root, "--module", "example.com/odd", "--starter", "runnable", "--makefile", "--actions", "--lint")
	if _, err := os.Stat(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("RUBRIC_ACCEPTANCE") == "1" {
		mustCommand(t, root, nil, "sh", ".rubric/check.sh", "build")
		mustCommand(t, root, nil, "sh", ".rubric/check.sh", "test")
	}
}

func TestDocumentedExamples(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "cli-init.md"))
	if err != nil {
		t.Fatal(err)
	}
	var examples []string
	inShell := false
	scanner := bufio.NewScanner(strings.NewReader(string(doc)))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "```sh"):
			inShell = true
		case strings.HasPrefix(line, "```"):
			inShell = false
		case inShell && strings.HasPrefix(line, "rubric init "):
			examples = append(examples, line)
		}
	}
	if len(examples) < 6 {
		t.Fatalf("found %d documented examples", len(examples))
	}
	for _, example := range examples {
		t.Run(example, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, filepath.Join(dir, "legacy-app"), map[string]string{
				"go.mod":  "module example.com/legacy-app\n\ngo 1.26.7\n",
				"main.go": "package main\n\nfunc main() {}\n",
			})
			writeFiles(t, dir, map[string]string{"input.yaml": "project:\n  module: example.com/from-input\ntooling:\n  makefile: true\n"})
			var args []string
			for _, word := range strings.Fields(example)[1:] {
				switch word {
				case "my-app", "legacy-app", "input.yaml":
					word = filepath.Join(dir, word)
				}
				args = append(args, strings.Trim(word, "'"))
			}
			if r := rubric(t, args...); r.code != 0 {
				t.Fatalf("exit %d\n%s\n%s", r.code, r.out, r.stderr)
			}
		})
	}
}
