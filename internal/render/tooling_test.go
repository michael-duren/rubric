package render_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

const (
	checkoutRef = "actions/checkout@08c6903cd8c0fde910a37f88322edcfb5dd907a8"
	setupGoRef  = "actions/setup-go@44694675825211faa026b3c33043df3e48a5fa00"
	lintLaunch  = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2"
)

var skillPaths = []string{
	".agents/skills/rubric-style/SKILL.md", ".agents/skills/rubric-testing/SKILL.md", ".agents/skills/rubric-workflow/SKILL.md",
}

func renderTooling(t *testing.T, mode string, f config.Features, tooling config.Tooling) (config.Config, []render.File) {
	t.Helper()
	c := testproject.Config()
	c.Features = f
	c.Tooling = tooling
	if mode == "existing" {
		c.Project.Go = "1.22"
	}
	files, err := render.Files(c, mode)
	if err != nil {
		t.Fatal(err)
	}
	return c, files
}

func checkOps(t *testing.T, script string) []string {
	t.Helper()
	var ops []string
	for _, m := range regexp.MustCompile(`(?m)^\t([a-z0-9][a-z0-9-]*)\)`).FindAllStringSubmatch(script, -1) {
		ops = append(ops, m[1])
	}
	return ops
}

func shellSyntax(t *testing.T, script []byte) {
	t.Helper()
	cmd := exec.Command("sh", "-n")
	cmd.Stdin = bytes.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("check.sh syntax: %v\n%s", err, out)
	}
}

type workflow struct {
	Jobs map[string]struct {
		Services map[string]struct {
			Image string
			Env   map[string]string
		}
		Steps []struct {
			Name string
			Uses string
			Run  string
			With map[string]string
			Env  map[string]string
		}
	}
}

func parseWorkflow(t *testing.T, data []byte) workflow {
	t.Helper()
	var w workflow
	if err := yaml.Unmarshal(data, &w); err != nil {
		t.Fatalf("workflow YAML: %v\n%s", err, data)
	}
	if len(w.Jobs) != 1 {
		t.Fatalf("jobs = %v", w.Jobs)
	}
	return w
}

func TestToolingCombinations(t *testing.T) {
	t.Setenv("RUBRIC_SECRET_TOKEN", "s3cr3t-value")
	f := features(func(f *config.Features) { f.HTTP, f.Database, f.Access = "nethttp", "sqlite", "sql" })
	for mask := range 16 {
		tooling := config.Tooling{Skills: mask&1 != 0, Lint: mask&2 != 0, Makefile: mask&4 != 0, Actions: mask&8 != 0}
		for _, mode := range []string{"new", "existing"} {
			t.Run(fmt.Sprintf("%s-%+v", mode, tooling), func(t *testing.T) {
				_, files := renderTooling(t, mode, f, tooling)
				present := func(path string) bool { _, ok := find(files, path); return ok }
				expect := map[string]bool{
					"Makefile":                 tooling.Makefile,
					".golangci.yml":            tooling.Lint,
					".rubric/style/analyze.go": tooling.Lint,
					".github/workflows/ci.yml": tooling.Actions,
					".rubric/check.sh":         tooling.Makefile || tooling.Actions || tooling.Lint,
				}
				for _, p := range skillPaths {
					expect[p] = tooling.Skills
				}
				for path, want := range expect {
					if present(path) != want {
						t.Errorf("%s present=%v, want %v", path, present(path), want)
					}
				}
				doc, err := config.Decode(testproject.File(t, files, "rubric.yaml"))
				if err != nil {
					t.Fatal(err)
				}
				saved, err := config.Resolve(config.Defaults(), doc.Values)
				if err != nil {
					t.Fatal(err)
				}
				if saved.Tooling != tooling {
					t.Fatalf("saved tooling %+v, want %+v", saved.Tooling, tooling)
				}
				var ops []string
				if script, ok := find(files, ".rubric/check.sh"); ok {
					shellSyntax(t, script.Data)
					if script.Mode != 0o755 {
						t.Errorf("check.sh mode %v", script.Mode)
					}
					ops = checkOps(t, string(script.Data))
					if tooling.Lint && !strings.Contains(string(script.Data), lintLaunch+" run --allow-parallel-runners") {
						t.Error("lint does not allow concurrent golangci-lint runs")
					}
					if tooling.Lint != slices.Contains(ops, "lint") {
						t.Errorf("lint operation present=%v", slices.Contains(ops, "lint"))
					}
				}
				if mk, ok := find(files, "Makefile"); ok {
					for _, m := range regexp.MustCompile(`sh \.rubric/check\.sh '?([a-z0-9-]+)'?`).FindAllStringSubmatch(string(mk.Data), -1) {
						if !slices.Contains(ops, m[1]) {
							t.Errorf("Makefile calls absent operation %s", m[1])
						}
					}
				}
				if wf, ok := find(files, ".github/workflows/ci.yml"); ok {
					w := parseWorkflow(t, wf.Data)
					for _, job := range w.Jobs {
						var runs []string
						for _, s := range job.Steps {
							runs = append(runs, s.Run)
						}
						all := strings.Join(runs, "\n")
						if strings.Contains(all, "make ") != tooling.Makefile {
							t.Errorf("workflow uses make=%v with makefile=%v:\n%s", strings.Contains(all, "make "), tooling.Makefile, all)
						}
						if strings.Contains(all, "lint") != tooling.Lint {
							t.Errorf("workflow lint step with lint=%v", tooling.Lint)
						}
						if job.Steps[0].Uses != checkoutRef || job.Steps[1].Uses != setupGoRef || job.Steps[1].With["go-version"] != "1.26.7" {
							t.Errorf("workflow setup steps = %+v", job.Steps[:2])
						}
					}
				}
				agents := string(testproject.File(t, files, "AGENTS.md"))
				for _, p := range skillPaths {
					if strings.Contains(agents, p) != tooling.Skills {
						t.Errorf("AGENTS mentions %s = %v", p, strings.Contains(agents, p))
					}
				}
				if strings.Contains(agents, "make test") != tooling.Makefile {
					t.Errorf("AGENTS mentions make test = %v", strings.Contains(agents, "make test"))
				}
				for _, file := range files {
					for _, bad := range []string{"rubric validate", "rubric make", "rubric paths", "rubric perf", "s3cr3t-value"} {
						if strings.Contains(string(file.Data), bad) {
							t.Errorf("%s contains %q", file.Path, bad)
						}
					}
				}
			})
		}
	}
}

func TestToolingPostgresWorkflow(t *testing.T) {
	f := features(func(f *config.Features) { f.Database, f.Access = "postgres", "sqlc" })
	_, files := renderTooling(t, "new", f, config.Tooling{Actions: true})
	w := parseWorkflow(t, testproject.File(t, files, ".github/workflows/ci.yml"))
	for _, job := range w.Jobs {
		pg := job.Services["postgres"]
		if pg.Image != "postgres:17.6" || pg.Env["POSTGRES_DB"] != "rubric_test" || pg.Env["POSTGRES_HOST_AUTH_METHOD"] != "trust" {
			t.Fatalf("service = %+v", pg)
		}
		var integration, generate bool
		for _, s := range job.Steps {
			if strings.Contains(s.Run, "test-integration") {
				integration = s.Env["TEST_DATABASE_URL"] == "postgres://postgres@127.0.0.1:5432/rubric_test?sslmode=disable"
			}
			generate = generate || strings.Contains(s.Run, "generate")
		}
		if !integration || !generate {
			t.Fatalf("steps = %+v", job.Steps)
		}
	}
}

func TestToolingSkills(t *testing.T) {
	for _, lint := range []bool{false, true} {
		_, files := renderTooling(t, "new", features(func(f *config.Features) { f.CLI = "flag" }), config.Tooling{Skills: true, Lint: lint})
		for _, p := range skillPaths {
			body := string(testproject.File(t, files, p))
			parts := strings.SplitN(body, "---\n", 3)
			if len(parts) != 3 || parts[0] != "" {
				t.Fatalf("%s frontmatter missing", p)
			}
			var meta struct{ Name, Description string }
			if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
				t.Fatal(err)
			}
			if meta.Name != filepath.Base(filepath.Dir(p)) || !strings.HasPrefix(meta.Description, "Use when") {
				t.Fatalf("%s meta = %+v", p, meta)
			}
			if strings.Contains(body, "rubric validate") {
				t.Fatalf("%s calls a deferred command", p)
			}
		}
		workflowSkill := string(testproject.File(t, files, ".agents/skills/rubric-workflow/SKILL.md"))
		for _, want := range []string{"go test ./...", "go run ./cmd/cli"} {
			if !strings.Contains(workflowSkill, want) {
				t.Errorf("workflow skill missing %q", want)
			}
		}
		styleSkill := string(testproject.File(t, files, ".agents/skills/rubric-style/SKILL.md"))
		for _, link := range []string{"https://go.dev/wiki/CodeReviewComments", "https://peter.bourgon.org/go-in-production/#formatting-and-style", "https://github.com/uber-go/guide/blob/master/style.md"} {
			if !strings.Contains(styleSkill, link) {
				t.Errorf("style skill missing %s", link)
			}
		}
		if strings.Contains(styleSkill, ".rubric/check.sh lint") != lint {
			t.Errorf("style skill lint claim with lint=%v", lint)
		}
	}
}

func TestSkillFrontmatterQuotesProjectName(t *testing.T) {
	c := testproject.Config()
	c.Project.Name = `my: app # "x" 'y'`
	c.Tooling.Skills = true
	files, err := render.Files(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range skillPaths {
		parts := strings.SplitN(string(testproject.File(t, files, p)), "---\n", 3)
		var meta struct{ Name, Description string }
		if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
			t.Fatalf("%s: %v\n%s", p, err, parts[1])
		}
		if !strings.Contains(meta.Description, c.Project.Name) {
			t.Fatalf("%s description = %q", p, meta.Description)
		}
	}
}

func runCheck(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), args[0], args[1:]...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestEmptyChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("runs generated checks")
	}
	_, files := renderTooling(t, "new", config.Defaults().Features, config.Tooling{Makefile: true, Actions: true, Lint: true})
	root := testproject.Write(t, files)
	for _, op := range []string{"build", "test"} {
		out, err := runCheck(t, root, "sh", ".rubric/check.sh", op)
		if err != nil || !strings.Contains(out, "no application packages") {
			t.Fatalf("%s on module-only project: %v\n%s", op, err, out)
		}
		if op == "test" && !strings.Contains(out, ".rubric/style") {
			t.Fatalf("analyzer tests skipped:\n%s", out)
		}
	}
	if _, err := exec.LookPath("make"); err == nil {
		if out, err := runCheck(t, root, "make", "test"); err != nil {
			t.Fatalf("make test: %v\n%s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "app_test.go"), []byte("package demo\n\nimport \"testing\"\n\nfunc TestFail(t *testing.T) { t.Fatal(\"boom\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runCheck(t, root, "sh", ".rubric/check.sh", "test"); err == nil || !strings.Contains(out, "boom") {
		t.Fatalf("failing test passed the check: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runCheck(t, root, "sh", ".rubric/check.sh", "build"); err == nil || strings.Contains(out, "no application packages") {
		t.Fatalf("broken go list treated as empty: %v\n%s", err, out)
	}
	if out, err := runCheck(t, root, "sh", ".rubric/check.sh", "bogus"); err == nil {
		t.Fatalf("unknown operation accepted:\n%s", out)
	}
}

func TestToolingChecksGeneratedProject(t *testing.T) {
	if testing.Short() {
		t.Skip("runs golangci-lint and sqlc")
	}
	pg := features(func(f *config.Features) {
		f.HTTP, f.CLI, f.Database, f.Access = "nethttp", "flag", "postgres", "sqlc"
	})
	_, pgFiles := renderTooling(t, "new", pg, config.Tooling{Lint: true})
	pgRoot := testproject.Write(t, pgFiles)
	testproject.Go(t, pgRoot, "mod", "tidy")
	if out, err := runCheck(t, pgRoot, "sh", ".rubric/check.sh", "lint"); err != nil {
		t.Fatalf("postgres lint: %v\n%s", err, out)
	}
	f := features(func(f *config.Features) {
		f.HTTP, f.CLI, f.TUI, f.Config, f.Database, f.Access = "chi", "cobra", "bubbletea", "viper", "sqlite", "sqlc"
	})
	_, files := renderTooling(t, "new", f, config.Tooling{Makefile: true, Lint: true})
	root := testproject.Write(t, files)
	testproject.Go(t, root, "mod", "tidy")
	for _, op := range []string{"build", "test", "lint", "generate"} {
		if out, err := runCheck(t, root, "sh", ".rubric/check.sh", op); err != nil {
			t.Fatalf("%s: %v\n%s", op, err, out)
		}
	}
	if out, err := runCheck(t, root, "git", "diff", "--no-index", "--quiet", "internal/store/queries", "internal/store/queries"); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if _, err := exec.LookPath("make"); err == nil {
		if out, err := runCheck(t, root, "make", "generate"); err != nil {
			t.Fatalf("make generate: %v\n%s", err, out)
		}
	}
	bad := filepath.Join(root, "internal", "cli", "unformatted.go")
	if err := os.WriteFile(bad, []byte("package cli\n\nvar  unformatted = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runCheck(t, root, "sh", ".rubric/check.sh", "lint"); err == nil {
		t.Fatalf("unformatted code passed lint:\n%s", out)
	}
}

func TestToolingToolchainDiagnosis(t *testing.T) {
	c, files := renderTooling(t, "existing", config.Defaults().Features, config.Tooling{Makefile: true})
	script := string(testproject.File(t, files, ".rubric/check.sh"))
	if !strings.Contains(script, `required="1.26.7"`) || c.Project.Go != "1.22" {
		t.Fatalf("toolchain requirement not rendered:\n%s", script)
	}
	root := testproject.Write(t, files)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(script, `required="1.26.7"`, `required="9.0.0"`, 1)
	if err := os.WriteFile(filepath.Join(root, ".rubric", "check.sh"), []byte(patched), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := runCheck(t, root, "sh", ".rubric/check.sh", "build")
	if err == nil || !strings.Contains(out, "Go 9.0.0 or newer") {
		t.Fatalf("old toolchain not diagnosed: %v\n%s", err, out)
	}
	mod, _ := os.ReadFile(filepath.Join(root, "go.mod"))
	if string(mod) != "module example.com/demo\n\ngo 1.22\n" {
		t.Fatal("go.mod rewritten")
	}
}
