package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/michael-duren/go-skills/internal/catalog"
	"github.com/michael-duren/go-skills/internal/cli"
	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

func acceptance(t *testing.T) {
	t.Helper()
	if os.Getenv("RUBRIC_ACCEPTANCE") != "1" {
		t.Skip("set RUBRIC_ACCEPTANCE=1 to run acceptance tests")
	}
}

type result struct {
	code        int
	out, stderr string
}

func rubric(t *testing.T, args ...string) result {
	t.Helper()
	var out, stderr bytes.Buffer
	code := cli.Run(t.Context(), args, cli.Streams{In: strings.NewReader(""), Out: &out, Err: &stderr})
	return result{code: code, out: out.String(), stderr: stderr.String()}
}

func mustRubric(t *testing.T, args ...string) result {
	t.Helper()
	r := rubric(t, args...)
	if r.code != 0 {
		t.Fatalf("rubric %q: exit %d\nstdout:\n%s\nstderr:\n%s", args, r.code, r.out, r.stderr)
	}
	return r
}

func command(t *testing.T, dir string, env []string, name string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off"), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func mustCommand(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()
	out, err := command(t, dir, env, name, args...)
	if err != nil {
		t.Fatalf("%s %q in %s: %v\n%s", name, args, dir, err, out)
	}
	return out
}

type appCase struct {
	index    int
	starter  string
	features config.Features
}

func (c appCase) name() string {
	f := c.features
	return fmt.Sprintf("%03d-%s-%s-%s-%s-%s-%s-%s", c.index, c.starter, f.HTTP, f.Database, f.Access, f.CLI, f.TUI, f.Config)
}

func executables(f config.Features) bool {
	return f.HTTP != "none" || f.CLI != "none" || f.TUI != "none"
}

func appCases() []appCase {
	var out []appCase
	for _, f := range catalog.Cases() {
		starters := []string{"module"}
		if !executables(f) {
			starters = append(starters, "runnable")
		}
		for _, s := range starters {
			out = append(out, appCase{index: len(out), starter: s, features: f})
		}
	}
	return out
}

func (c appCase) flags(target string) []string {
	f := c.features
	return []string{
		"init", target, "--non-interactive", "--module", "example.com/case" + strconv.Itoa(c.index),
		"--starter", c.starter, "--http", f.HTTP, "--database", f.Database, "--access", accessFlag(f),
		"--cli", f.CLI, "--tui", f.TUI, "--app-config", f.Config,
	}
}

func accessFlag(f config.Features) string {
	if f.Database == "none" {
		return "none"
	}
	return f.Access
}

func (c appCase) hasPackages() bool {
	f := c.features
	return executables(f) || f.Database != "none" || f.Config != "stdlib"
}

func inShard(index int) bool {
	spec := os.Getenv("RUBRIC_SHARD")
	if spec == "" {
		return true
	}
	a, b, ok := strings.Cut(spec, "/")
	i, err1 := strconv.Atoi(a)
	n, err2 := strconv.Atoi(b)
	return ok && err1 == nil && err2 == nil && n > 0 && index%n == i
}

func TestCatalogMatrixSize(t *testing.T) {
	if got := len(catalog.Cases()); got != 180 {
		t.Fatalf("catalog cases=%d, want 180", got)
	}
	if got := len(appCases()); got != 190 {
		t.Fatalf("application cases=%d, want 190", got)
	}
}

func missingTests(files []render.File) []string {
	tests := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f.Path, "_test.go") {
			tests[path.Dir(f.Path)] = true
		}
	}
	var missing []string
	for _, f := range files {
		dir := path.Dir(f.Path)
		exempt := path.Base(f.Path) == "main.go" || contains(strings.Split(dir, "/"), "cmd") || strings.HasPrefix(f.Path, ".")
		if strings.HasSuffix(f.Path, ".go") && !strings.HasSuffix(f.Path, "_test.go") && !exempt && !tests[dir] {
			missing = append(missing, f.Path)
		}
	}
	return missing
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestGeneratedTestInventory(t *testing.T) {
	for _, c := range appCases() {
		cfg := testproject.Config()
		cfg.Project.Starter, cfg.Features = c.starter, c.features
		files, err := render.Files(cfg, "new")
		if err != nil {
			t.Fatal(err)
		}
		if missing := missingTests(files); len(missing) > 0 {
			t.Fatalf("%s: non-exempt source without package tests: %v", c.name(), missing)
		}
	}
}

func TestInventoryGateRejectsMissingTests(t *testing.T) {
	cfg := testproject.Config()
	cfg.Features.HTTP = "chi"
	files, err := render.Files(cfg, "new")
	if err != nil {
		t.Fatal(err)
	}
	var without []render.File
	for _, f := range files {
		if f.Path != "internal/httpserver/server_test.go" {
			without = append(without, f)
		}
	}
	missing := missingTests(without)
	if len(missing) != 2 {
		t.Fatalf("gate did not reject the missing test: %v", missing)
	}
}

var (
	guidanceDir    = regexp.MustCompile("`((?:internal|cmd|sql|\\.rubric|\\.agents|\\.github)/[A-Za-z0-9_.-][A-Za-z0-9_./-]*)`")
	guidanceLayout = regexp.MustCompile("(?m)^- `([A-Za-z0-9_-]*[./][A-Za-z0-9_./-]*)`:")
)

func assertGuidancePaths(t *testing.T, root string) {
	t.Helper()
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	matches := append(guidanceDir.FindAllStringSubmatch(string(agents), -1), guidanceLayout.FindAllStringSubmatch(string(agents), -1)...)
	for _, m := range matches {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(m[1]))); err != nil {
			t.Errorf("AGENTS.md mentions %s, which does not exist", m[1])
		}
	}
}

func runProjectChecks(t *testing.T, root string) {
	t.Helper()
	mustCommand(t, root, nil, "go", "mod", "tidy")
	mustCommand(t, root, nil, "go", "build", "./...")
	mustCommand(t, root, nil, "go", "vet", "./...")
	mustCommand(t, root, nil, "go", "test", "-count=1", "-coverprofile=coverage.out", "./...")
	testproject.AssertBehaviorCoverage(t, root, filepath.Join(root, "coverage.out"))
	t.Log(mustCommand(t, root, nil, "go", "tool", "cover", "-func=coverage.out"))
}

func TestAcceptanceMatrix(t *testing.T) {
	acceptance(t)
	for _, c := range appCases() {
		if !inShard(c.index) {
			continue
		}
		t.Run(c.name(), func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "app")
			mustRubric(t, c.flags(root)...)
			assertGuidancePaths(t, root)
			switch {
			case c.hasPackages():
				runProjectChecks(t, root)
			case c.starter == "runnable":
				mustCommand(t, root, nil, "go", "build", "./...")
			default:
				out := strings.TrimSpace(mustCommand(t, root, nil, "go", "list", "./..."))
				if out != `go: warning: "./..." matched no packages` {
					t.Fatalf("module-only project has packages: %s", out)
				}
			}
			var report struct {
				Actions []struct{ State string }
			}
			r := mustRubric(t, "init", root, "--format", "json")
			if err := json.Unmarshal([]byte(r.out), &report); err != nil {
				t.Fatal(err)
			}
			for _, a := range report.Actions {
				if a.State != "unchanged" {
					t.Fatalf("first rerun is not a no-op:\n%s", r.out)
				}
			}
		})
	}
}
