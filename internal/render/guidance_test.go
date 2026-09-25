package render_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

func instructions(t *testing.T, c config.Config, mode string) string {
	t.Helper()
	c.EntryPoints = render.EntryPoints(c, mode)
	c.Commands = render.Commands(c, mode)
	text, err := render.Instructions(c, mode)
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

func TestModuleOnlyGuidanceHasNoRunCommand(t *testing.T) {
	c := testproject.Config()
	c.Project.Starter = "module"
	c.Commands = render.Commands(c, "new")
	text, err := render.Instructions(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(text), "go run .") || strings.Contains(string(text), "cmd/server") {
		t.Fatalf("guidance invents an executable:\n%s", text)
	}
	if !strings.Contains(string(text), "main.go") || !strings.Contains(string(text), "cmd/") {
		t.Fatal("generated testing exemptions missing")
	}
}

func TestModuleOnlyGuidanceHasNoBuildOrTestClaims(t *testing.T) {
	c := testproject.Config()
	c.Project.Starter = "module"
	if cmds := render.Commands(c, "new"); len(cmds) != 0 {
		t.Fatalf("module-only commands = %+v", cmds)
	}
	text := instructions(t, c, "new")
	for _, bad := range []string{"go test ./...", "go build ./...", "database", "server"} {
		if strings.Contains(strings.ToLower(text), bad) {
			t.Errorf("module-only guidance mentions %q:\n%s", bad, text)
		}
	}
	if !strings.Contains(text, "no application packages") {
		t.Errorf("module-only state not explained:\n%s", text)
	}
}

func TestGuidanceMarkersWrapSectionOnly(t *testing.T) {
	text := instructions(t, testproject.Config(), "new")
	if !strings.HasPrefix(text, "<!-- rubric:begin -->\n") || !strings.HasSuffix(text, "<!-- rubric:end -->\n") {
		t.Fatalf("markers wrong:\n%s", text)
	}
	if strings.Count(text, "<!-- rubric:") != 2 {
		t.Fatalf("extra markers:\n%s", text)
	}
}

func TestGuidanceRootRunnable(t *testing.T) {
	c := testproject.Config()
	c.Project.Starter = "runnable"
	cmds := render.Commands(c, "new")
	names := commandNames(cmds)
	if !slices.Equal(names, []string{"setup", "build", "test", "run-demo"}) {
		t.Fatalf("commands = %v", names)
	}
	text := instructions(t, c, "new")
	if !strings.Contains(text, "`go run ./cmd/demo`") || !strings.Contains(text, "`cmd/demo`") || strings.Contains(text, "cmd/server") {
		t.Fatalf("runnable guidance:\n%s", text)
	}
}

func commandNames(cmds []config.Command) []string {
	var out []string
	for _, c := range cmds {
		out = append(out, c.Name)
	}
	return out
}

func TestGuidanceForEachExecutable(t *testing.T) {
	tests := []struct {
		name    string
		edit    func(*config.Features)
		dir     string
		command string
		pkg     string
	}{
		{"server", func(f *config.Features) { f.HTTP = "nethttp" }, "cmd/server", "run-server", "internal/httpserver"},
		{"cli", func(f *config.Features) { f.CLI = "cobra" }, "cmd/cli", "run-cli", "internal/cli"},
		{"tui", func(f *config.Features) { f.TUI = "bubbletea" }, "cmd/tui", "run-tui", "internal/tui"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testproject.Config()
			tt.edit(&c.Features)
			cmds := render.Commands(c, "new")
			if !slices.Contains(commandNames(cmds), tt.command) {
				t.Fatalf("commands = %v", commandNames(cmds))
			}
			text := instructions(t, c, "new")
			for _, want := range []string{tt.dir, "go run ./" + tt.dir, tt.pkg, "go test ./..."} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q:\n%s", want, text)
				}
			}
			for _, other := range tests {
				if other.name != tt.name && strings.Contains(text, other.dir) {
					t.Errorf("unselected %s advertised:\n%s", other.dir, text)
				}
			}
		})
	}
}

func TestGuidanceLibraryOnlyComponents(t *testing.T) {
	c := testproject.Config()
	c.Project.Starter = "module"
	c.Features.Database, c.Features.Access, c.Features.Config = "sqlite", "sql", "viper"
	text := instructions(t, c, "new")
	for _, want := range []string{"internal/store", "internal/config", "SQLite", "Viper", "go test ./..."} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "go run") {
		t.Errorf("library-only project advertises a run command:\n%s", text)
	}
}

func TestGuidanceExistingCustomCommands(t *testing.T) {
	c := testproject.Config()
	c.Project.Go = "1.22"
	c.Features.HTTP = "gin"
	c.EntryPoints = []config.EntryPoint{{Name: "api", Dir: "cmd/api"}}
	c.Commands = []config.Command{{Name: "test", Dir: ".", Argv: []string{"make", "test"}, Env: []string{"DATABASE_URL"}}}
	cmds := render.Commands(c, "existing")
	if !slices.Equal(commandNames(cmds), []string{"build", "test", "run-api"}) {
		t.Fatalf("commands = %+v", cmds)
	}
	text := instructions(t, c, "existing")
	for _, want := range []string{"`make test`", "`go run ./cmd/api`", "DATABASE_URL", "gin", "Go 1.22"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "go test ./...") {
		t.Errorf("explicit test command replaced:\n%s", text)
	}
}

func TestGuidanceExistingMissingCommands(t *testing.T) {
	c := testproject.Config()
	c.Project.Go = "1.22"
	if cmds := render.Commands(c, "existing"); len(cmds) != 0 {
		t.Fatalf("invented commands %+v", cmds)
	}
	text := instructions(t, c, "existing")
	if !strings.Contains(text, "No commands are configured") || strings.Contains(text, "go run") {
		t.Fatalf("missing-command guidance:\n%s", text)
	}
}

func TestGuidanceOmitsUnselectedTooling(t *testing.T) {
	c := testproject.Config()
	c.Project.Starter = "runnable"
	text := instructions(t, c, "new")
	for _, bad := range []string{"golangci", "make ", "Makefile", ".github/workflows", ".agents/skills", "sqlc"} {
		if strings.Contains(text, bad) {
			t.Errorf("unselected tooling %q mentioned:\n%s", bad, text)
		}
	}
}

func TestGuidanceStylePolicy(t *testing.T) {
	text := instructions(t, testproject.Config(), "new")
	for _, want := range []string{
		"https://go.dev/wiki/CodeReviewComments",
		"https://peter.bourgon.org/go-in-production/#formatting-and-style",
		"https://github.com/uber-go/guide/blob/master/style.md",
		"two physical lines", "150 Unicode characters", ".rubric/style.md",
		"inline or trailing comments", "measured",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q:\n%s", want, text)
		}
	}
}

func TestGuidanceNeverCallsDeferredCommands(t *testing.T) {
	for _, mode := range []string{"new", "existing"} {
		c := testproject.Config()
		c.Features.HTTP = "chi"
		c.Tooling = config.Tooling{Skills: config.SkillGroups, Lint: true, Makefile: true, Actions: true}
		authored := instructions(t, c, mode)
		all := authored
		files, err := render.Files(c, mode)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			all += string(f.Data)
			vendored := strings.HasPrefix(f.Path, ".agents/agents/") ||
				strings.HasPrefix(f.Path, ".agents/skills/") && !strings.HasPrefix(f.Path, ".agents/skills/rubric-")
			if !vendored {
				authored += string(f.Data)
			}
		}
		for _, bad := range []string{"rubric validate", "rubric make", "rubric paths", "rubric perf"} {
			if strings.Contains(all, bad) {
				t.Errorf("%s output mentions %q", mode, bad)
			}
		}
		if strings.Contains(authored, "performance") {
			t.Errorf("%s Rubric-authored output advertises performance tooling", mode)
		}
	}
}

func TestFilesIncludeConfigGuidanceAndStyle(t *testing.T) {
	c := testproject.Config()
	c.Project.Starter = "runnable"
	c.Project.Description = "Runs $(rm -rf /) `whoami` \"quoted\" — café ✓"
	files, err := render.Files(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, f := range files {
		kinds[f.Path] = f.Kind
	}
	want := map[string]string{
		"rubric.yaml": "config", "README.md": "scaffold", "AGENTS.md": "guidance",
		".rubric/style.md": "managed", "go.mod": "scaffold", "cmd/demo/main.go": "scaffold",
	}
	for path, kind := range want {
		if kinds[path] != kind {
			t.Errorf("%s kind = %q, want %q", path, kinds[path], kind)
		}
	}
	readme := string(testproject.File(t, files, "README.md"))
	if !strings.Contains(readme, c.Project.Description) || !strings.Contains(readme, "go run ./cmd/demo") {
		t.Fatalf("README:\n%s", readme)
	}
	doc, err := config.Decode(testproject.File(t, files, "rubric.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := config.Resolve(config.Defaults(), doc.Values)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Project.Description != c.Project.Description || !slices.Contains(commandNames(saved.Commands), "run-demo") {
		t.Fatalf("saved config = %+v", saved)
	}
	agents := string(testproject.File(t, files, "AGENTS.md"))
	if agents != instructions(t, c, "new") {
		t.Fatal("AGENTS.md differs from Instructions for the same inventory")
	}
}

func TestExistingFilesSkipReadme(t *testing.T) {
	c := testproject.Config()
	c.Project.Go = "1.22"
	files, err := render.Files(c, "existing")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	if !slices.Equal(paths, []string{".rubric/style.md", "AGENTS.md", "rubric.yaml"}) {
		t.Fatalf("paths = %v", paths)
	}
}
