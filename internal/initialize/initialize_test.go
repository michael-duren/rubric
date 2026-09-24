package initialize

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/plan"
	"github.com/michael-duren/go-skills/internal/write"
)

func put(t *testing.T, root, rel, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func newRequest(root string, overrides config.Patch) Request {
	if overrides == nil {
		overrides = config.Patch{}
	}
	if _, ok := overrides["project.module"]; !ok {
		overrides["project.module"] = "example.com/demo"
	}
	return Request{Target: root, Mode: "auto", Overrides: overrides}
}

func mustPrepare(t *testing.T, req Request) plan.Plan {
	t.Helper()
	p, err := Prepare(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func paths(p plan.Plan, states ...string) []string {
	var out []string
	for _, a := range p.Actions {
		if len(states) == 0 || slices.Contains(states, a.State) {
			out = append(out, a.File.Path)
		}
	}
	return out
}

func isInputError(err error) bool {
	var in *InputError
	return errors.As(err, &in)
}

func TestNewProjectIntoMissingDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new project")
	p := mustPrepare(t, newRequest(root, nil))
	if p.Mode != "new" {
		t.Fatalf("mode = %s", p.Mode)
	}
	want := []string{".rubric/manifest.json", ".rubric/style.md", "AGENTS.md", "README.md", "go.mod", "rubric.yaml"}
	if !slices.Equal(paths(p, plan.StateCreate), want) {
		t.Fatalf("created = %v", paths(p, plan.StateCreate))
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("prepare created the target")
	}
	res, err := Apply(context.Background(), newRequest(root, nil), p)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != len(want) || !strings.HasPrefix(read(t, root, "go.mod"), "module example.com/demo\n") {
		t.Fatalf("applied = %v", res.Applied)
	}
}

func TestModeResolution(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string)
		mode  string
		want  string
	}{
		{"empty auto", func(*testing.T, string) {}, "auto", "new"},
		{"git only auto", func(t *testing.T, r string) { put(t, r, ".git/HEAD", "x") }, "auto", "new"},
		{"module auto", func(t *testing.T, r string) { put(t, r, "go.mod", "module example.com/demo\n\ngo 1.22\n") }, "auto", "existing"},
		{"module new", func(t *testing.T, r string) { put(t, r, "go.mod", "module example.com/x\n") }, "new", "error"},
		{"nonempty auto", func(t *testing.T, r string) { put(t, r, "notes.txt", "x") }, "auto", "error"},
		{"nonempty new", func(t *testing.T, r string) { put(t, r, "notes.txt", "x") }, "new", "new"},
		{"nonempty existing", func(t *testing.T, r string) { put(t, r, "notes.txt", "x") }, "existing", "error"},
		{"workspace", func(t *testing.T, r string) { put(t, r, "go.work", "go 1.26.7\n"); put(t, r, "a/go.mod", "module a\n") }, "auto", "error"},
		{"nested modules only", func(t *testing.T, r string) { put(t, r, "a/go.mod", "module a\n"); put(t, r, "b/go.mod", "module b\n") }, "new", "error"},
		{"bad mode", func(*testing.T, string) {}, "fresh", "error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)
			req := newRequest(root, nil)
			req.Mode = tt.mode
			p, err := Prepare(context.Background(), req)
			if tt.want == "error" {
				if !isInputError(err) {
					t.Fatalf("err = %v, want InputError", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.Mode != tt.want {
				t.Fatalf("mode = %s, want %s", p.Mode, tt.want)
			}
		})
	}
	req := newRequest(filepath.Join(t.TempDir(), "missing"), nil)
	req.Mode = "existing"
	if _, err := Prepare(context.Background(), req); !isInputError(err) {
		t.Fatalf("existing on absent target: %v", err)
	}
}

func TestMissingModuleAndBadInput(t *testing.T) {
	root := t.TempDir()
	if _, err := Prepare(context.Background(), Request{Target: root, Mode: "auto"}); !isInputError(err) || !strings.Contains(err.Error(), "project.module") {
		t.Fatalf("missing module: %v", err)
	}
	for _, input := range []string{"schema: 2\nproject:\n  module: example.com/x\n", "bogus: 1\n", "tooling: [\n"} {
		req := newRequest(root, nil)
		req.Input = []byte(input)
		if _, err := Prepare(context.Background(), req); !isInputError(err) {
			t.Fatalf("input %q: %v", input, err)
		}
	}
	put(t, root, "rubric.yaml", "schema: 7\n")
	if _, err := Prepare(context.Background(), newRequest(root, nil)); !isInputError(err) {
		t.Fatalf("bad saved config accepted")
	}
}

func TestPrecedence(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	put(t, root, "rubric.yaml", "# saved notes\nschema: 1\ntooling:\n  lint: true\n  makefile: true\n  skills: true\nproject:\n  description: saved\n")
	req := Request{
		Target:    root,
		Mode:      "auto",
		Input:     []byte("project:\n  description: from input\ntooling:\n  skills: false\n"),
		Overrides: config.Patch{"tooling.lint": false},
	}
	p := mustPrepare(t, req)
	c := p.Config
	if c.Tooling.Lint || !c.Tooling.Makefile || c.Tooling.Skills || c.Project.Description != "from input" {
		t.Fatalf("precedence wrong: %+v %+v", c.Tooling, c.Project)
	}
	if c.Project.Module != "example.com/app" || c.Project.Go != "1.22" {
		t.Fatalf("go.mod facts not preserved: %+v", c.Project)
	}
	var yaml string
	for _, a := range p.Actions {
		if a.File.Path == "rubric.yaml" {
			yaml = string(a.File.Data)
		}
	}
	if !strings.Contains(yaml, "# saved notes") || !strings.Contains(yaml, "lint: false") {
		t.Fatalf("rubric.yaml edit lost comments or values:\n%s", yaml)
	}
}

func TestExistingModuleMismatchRejected(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	if _, err := Prepare(context.Background(), newRequest(root, config.Patch{"project.module": "example.com/other"})); !isInputError(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestExistingUnknownLibrariesAndNoScaffolding(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n\nrequire github.com/gin-gonic/gin v1.10.0\n")
	put(t, root, "main.go", "package main\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc main() { _ = gin.Default().Run() }\n")
	p := mustPrepare(t, Request{Target: root, Mode: "auto"})
	if p.Mode != "existing" || p.Config.Features.HTTP != "gin" {
		t.Fatalf("mode %s features %+v", p.Mode, p.Config.Features)
	}
	for _, a := range p.Actions {
		if strings.HasSuffix(a.File.Path, ".go") || a.File.Path == "go.mod" || a.File.Path == "README.md" {
			t.Fatalf("existing project scaffolded %s", a.File.Path)
		}
	}
	if !slices.ContainsFunc(p.Config.Evidence, func(e config.Evidence) bool { return e.Value == "github.com/gin-gonic/gin" }) {
		t.Fatalf("evidence not retained: %+v", p.Config.Evidence)
	}
	if !slices.ContainsFunc(p.Config.Commands, func(c config.Command) bool { return c.Name == "run" }) {
		t.Fatalf("verified entry point command missing: %+v", p.Config.Commands)
	}
}

func TestExplicitEmptyListsPreserved(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	put(t, root, "cmd/a/main.go", "package main\n\nfunc main() {}\n")
	p := mustPrepare(t, Request{Target: root, Mode: "auto", Overrides: config.Patch{
		"commands": []config.Command{}, "entry_points": []config.EntryPoint{},
	}})
	if len(p.Config.Commands) != 0 || len(p.Config.EntryPoints) != 0 {
		t.Fatalf("explicit empty lists repopulated: %+v %+v", p.Config.Commands, p.Config.EntryPoints)
	}
}

func TestStaleSavedEntryPointFails(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	put(t, root, "cmd/a/main.go", "package main\n\nfunc main() {}\n")
	put(t, root, "rubric.yaml", "schema: 1\nentry_points:\n  - name: gone\n    dir: cmd/gone\n")
	_, err := Prepare(context.Background(), Request{Target: root, Mode: "auto"})
	if !isInputError(err) || !strings.Contains(err.Error(), "cmd/gone") {
		t.Fatalf("err = %v", err)
	}
	p := mustPrepare(t, Request{Target: root, Mode: "auto", Overrides: config.Patch{
		"entry_points": []config.EntryPoint{{Name: "a", Dir: "cmd/a"}},
	}})
	if len(p.Config.EntryPoints) != 1 {
		t.Fatalf("correction not applied: %+v", p.Config.EntryPoints)
	}
}

func savedYAML(t *testing.T, root string) config.Config {
	t.Helper()
	doc, err := config.Decode([]byte(read(t, root, "rubric.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	c, err := config.Resolve(config.Defaults(), doc.Values)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func commandNames(cmds []config.Command) []string {
	var out []string
	for _, c := range cmds {
		out = append(out, c.Name)
	}
	return out
}

func TestRerunKeepsUserRemovedCommandsAndEntryPoints(t *testing.T) {
	root := t.TempDir()
	req := newRequest(root, config.Patch{"features.cli": "flag", "features.http": "nethttp"})
	if _, err := Apply(context.Background(), req, mustPrepare(t, req)); err != nil {
		t.Fatal(err)
	}
	c := savedYAML(t, root)
	c.Commands = slices.DeleteFunc(c.Commands, func(cmd config.Command) bool { return cmd.Name == "run-cli" })
	c.EntryPoints = slices.DeleteFunc(c.EntryPoints, func(ep config.EntryPoint) bool { return ep.Dir == "cmd/cli" })
	data, err := config.Encode(config.Document{}, c)
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "rubric.yaml", string(data))
	p := mustPrepare(t, Request{Target: root, Mode: "auto"})
	if slices.Contains(commandNames(p.Config.Commands), "run-cli") {
		t.Fatalf("removed command restored: %v", commandNames(p.Config.Commands))
	}
	if slices.ContainsFunc(p.Config.EntryPoints, func(ep config.EntryPoint) bool { return ep.Dir == "cmd/cli" }) {
		t.Fatalf("removed entry point restored: %+v", p.Config.EntryPoints)
	}
}

func TestRerunDropsCommandsOfDisabledTooling(t *testing.T) {
	root := t.TempDir()
	req := newRequest(root, config.Patch{"project.starter": "runnable", "tooling.lint": true})
	if _, err := Apply(context.Background(), req, mustPrepare(t, req)); err != nil {
		t.Fatal(err)
	}
	p := mustPrepare(t, Request{Target: root, Mode: "auto", Overrides: config.Patch{"tooling.lint": false}})
	if slices.Contains(commandNames(p.Config.Commands), "lint") {
		t.Fatalf("stale lint command kept: %v", commandNames(p.Config.Commands))
	}
	p = mustPrepare(t, Request{Target: root, Mode: "auto", Overrides: config.Patch{"tooling.makefile": true}})
	if !slices.Contains(commandNames(p.Config.Commands), "lint") || !slices.Contains(commandNames(p.Config.Commands), "run") {
		t.Fatalf("commands lost: %v", commandNames(p.Config.Commands))
	}
}

func TestRerunKeepsGeneratedGuidance(t *testing.T) {
	for _, patch := range []config.Patch{
		{"features.http": "chi", "features.database": "postgres", "features.config": "viper", "features.cli": "flag"},
		{"project.starter": "runnable", "features.database": "sqlite"},
		{"features.tui": "bubbletea", "tooling.lint": true, "tooling.makefile": true},
	} {
		root := t.TempDir()
		req := newRequest(root, patch)
		if _, err := Apply(context.Background(), req, mustPrepare(t, req)); err != nil {
			t.Fatal(err)
		}
		first := read(t, root, "AGENTS.md")
		p := mustPrepare(t, Request{Target: root, Mode: "auto"})
		for _, a := range p.Actions {
			if a.File.Path == "AGENTS.md" && a.State != plan.StateUnchanged {
				t.Fatalf("%v: rerun rewrote guidance:\n--- before\n%s\n--- after\n%s", patch, first, a.File.Data)
			}
		}
	}
}

func TestRerunKeepsGeneratorRecord(t *testing.T) {
	root := t.TempDir()
	req := newRequest(root, config.Patch{"features.http": "chi", "features.database": "sqlite", "features.access": "sqlc"})
	if _, err := Apply(context.Background(), req, mustPrepare(t, req)); err != nil {
		t.Fatal(err)
	}
	saved := savedYAML(t, root)
	p := mustPrepare(t, Request{Target: root, Mode: "auto"})
	if !maps.Equal(p.Config.Generator.Dependencies, saved.Generator.Dependencies) || p.Config.Generator.Tools["sqlc"] != "v1.31.1" {
		t.Fatalf("generator record changed: %+v vs saved %+v", p.Config.Generator, saved.Generator)
	}
}

func TestRemoteGoRunCommandsAreNotEntryPoints(t *testing.T) {
	root := t.TempDir()
	req := newRequest(root, config.Patch{"features.database": "sqlite", "features.access": "sqlc"})
	if _, err := Apply(context.Background(), req, mustPrepare(t, req)); err != nil {
		t.Fatal(err)
	}
	p := mustPrepare(t, Request{Target: root, Mode: "auto"})
	if !slices.ContainsFunc(p.Config.Commands, func(c config.Command) bool { return c.Name == "generate" }) {
		t.Fatalf("generate command lost: %+v", p.Config.Commands)
	}
}

func TestConflictsBlockApplyAndDecisionsResolve(t *testing.T) {
	root := t.TempDir()
	put(t, root, "README.md", "mine\n")
	req := newRequest(root, nil)
	req.Mode = "new"
	p := mustPrepare(t, req)
	_, err := Apply(context.Background(), req, p)
	var conflict *ConflictError
	if !errors.As(err, &conflict) || len(plan.Conflicts(conflict.Plan)) != 1 {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("wrote despite conflict")
	}
	req.Decisions = map[string]string{"README.md": plan.DecisionSkip}
	p = mustPrepare(t, req)
	if _, err := Apply(context.Background(), req, p); err != nil {
		t.Fatal(err)
	}
	if read(t, root, "README.md") != "mine\n" {
		t.Fatal("skipped file replaced")
	}
	req.Decisions = map[string]string{"go.mod": "bogus"}
	if _, err := Prepare(context.Background(), req); !isInputError(err) {
		t.Fatalf("bad decision: %v", err)
	}
}

func TestRerunPreservesApplicationCodeAndBecomesNoOp(t *testing.T) {
	root := t.TempDir()
	req := newRequest(root, config.Patch{"project.starter": "runnable", "project.description": "demo app"})
	if _, err := Apply(context.Background(), req, mustPrepare(t, req)); err != nil {
		t.Fatal(err)
	}
	put(t, root, "main.go", "package main\n\nfunc main() { println(\"edited\") }\n")
	second := mustPrepare(t, Request{Target: root, Mode: "auto"})
	if second.Mode != "existing" {
		t.Fatalf("rerun mode = %s", second.Mode)
	}
	if len(plan.Conflicts(second)) != 0 {
		t.Fatalf("rerun conflicts: %+v", plan.Conflicts(second))
	}
	for _, a := range second.Actions {
		if a.File.Path == "main.go" || a.File.Path == "go.mod" || a.File.Path == "README.md" {
			t.Fatalf("rerun touches application file %s", a.File.Path)
		}
	}
	if second.Config.Project.Description != "demo app" || second.Config.Project.Starter != "runnable" {
		t.Fatalf("saved choices lost: %+v", second.Config.Project)
	}
	if _, err := Apply(context.Background(), Request{Target: root, Mode: "auto"}, second); err != nil {
		t.Fatal(err)
	}
	third := mustPrepare(t, Request{Target: root, Mode: "auto"})
	if changed := paths(third, plan.StateCreate, plan.StateUpdate, plan.StateConflict); len(changed) != 0 {
		t.Fatalf("unchanged rerun not a no-op: %v", changed)
	}
	if !strings.Contains(read(t, root, "main.go"), "edited") {
		t.Fatal("application code rewritten")
	}
}

func TestCancelledPrepareAndApply(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Prepare(ctx, newRequest(root, nil)); !errors.Is(err, context.Canceled) {
		t.Fatalf("prepare: %v", err)
	}
	p := mustPrepare(t, newRequest(root, nil))
	if _, err := Apply(ctx, newRequest(root, nil), p); !errors.Is(err, context.Canceled) {
		t.Fatalf("apply: %v", err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("cancelled apply wrote files")
	}
}

func TestStaleApplyReportsWriteConflict(t *testing.T) {
	root := t.TempDir()
	req := newRequest(root, nil)
	p := mustPrepare(t, req)
	put(t, root, "go.mod", "module sneaky\n")
	_, err := Apply(context.Background(), req, p)
	var conflict *write.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v", err)
	}
}
