package acceptance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

const (
	checkoutRef = "actions/checkout@08c6903cd8c0fde910a37f88322edcfb5dd907a8"
	setupGoRef  = "actions/setup-go@44694675825211faa026b3c33043df3e48a5fa00"
)

func validateWorkflow(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var w struct {
		Jobs map[string]struct {
			Steps []struct {
				Uses string
				Run  string
			}
		}
	}
	if err := yaml.Unmarshal(data, &w); err != nil {
		t.Fatalf("workflow YAML: %v", err)
	}
	var runs []string
	for _, job := range w.Jobs {
		if len(job.Steps) < 2 || job.Steps[0].Uses != checkoutRef || job.Steps[1].Uses != setupGoRef {
			t.Fatalf("unpinned or missing setup actions: %+v", job.Steps)
		}
		for _, s := range job.Steps {
			if s.Run != "" {
				runs = append(runs, s.Run)
			}
		}
	}
	return runs
}

func TestToolingVariants(t *testing.T) {
	acceptance(t)
	_, makeErr := exec.LookPath("make")
	projects := map[string][]string{
		"module-only": {"--starter", "module"},
		"service":     {"--http", "nethttp", "--database", "sqlite", "--cli", "flag"},
	}
	for name, features := range projects {
		for mask := range 16 {
			skills, lint, makefile, actions := mask&1 != 0, mask&2 != 0, mask&4 != 0, mask&8 != 0
			t.Run(fmt.Sprintf("%s-skills=%t-lint=%t-make=%t-actions=%t", name, skills, lint, makefile, actions), func(t *testing.T) {
				t.Parallel()
				root := filepath.Join(t.TempDir(), "app")
				args := append([]string{"init", root, "--module", "example.com/tool", "--non-interactive",
					fmt.Sprintf("--skills=%t", skills), fmt.Sprintf("--lint=%t", lint),
					fmt.Sprintf("--makefile=%t", makefile), fmt.Sprintf("--actions=%t", actions)}, features...)
				mustRubric(t, args...)
				assertGuidancePaths(t, root)
				if name != "module-only" {
					mustCommand(t, root, nil, "go", "mod", "tidy")
				}
				if !makefile && !actions && !lint {
					if _, err := os.Stat(filepath.Join(root, ".rubric", "check.sh")); err == nil {
						t.Fatal("check script generated without tooling")
					}
					return
				}
				ops := []string{"build", "test"}
				if lint {
					ops = append(ops, "lint")
				}
				for _, op := range ops {
					out := mustCommand(t, root, nil, "sh", ".rubric/check.sh", op)
					if name == "module-only" && op != "lint" && !strings.Contains(out, "no application packages") {
						t.Fatalf("module-only %s did not report missing packages:\n%s", op, out)
					}
				}
				if makefile && makeErr == nil {
					for _, op := range ops {
						mustCommand(t, root, nil, "make", op)
					}
				}
				if actions {
					for _, run := range validateWorkflow(t, filepath.Join(root, ".github", "workflows", "ci.yml")) {
						if strings.HasPrefix(run, "make ") && !makefile {
							t.Fatalf("workflow uses make without a Makefile: %s", run)
						}
						if strings.Contains(run, "lint") && !lint {
							t.Fatalf("workflow lints without lint enabled: %s", run)
						}
					}
				}
			})
		}
	}
}

func TestSQLCRegenerationThroughTooling(t *testing.T) {
	acceptance(t)
	for _, db := range []string{"sqlite", "postgres"} {
		t.Run(db, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "app")
			mustRubric(t, "init", root, "--module", "example.com/gen", "--database", db, "--access", "sqlc", "--makefile")
			mustCommand(t, root, nil, "go", "mod", "tidy")
			before := snapshotDir(t, filepath.Join(root, "internal", "store", "queries"))
			mustCommand(t, root, nil, "sh", ".rubric/check.sh", "generate")
			after := snapshotDir(t, filepath.Join(root, "internal", "store", "queries"))
			if fmt.Sprint(before) != fmt.Sprint(after) {
				t.Fatal("sqlc regeneration changed bundled output")
			}
		})
	}
}

func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = string(data)
	}
	return out
}
