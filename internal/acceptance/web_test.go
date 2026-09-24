package acceptance

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPlaywrightBrowserTests(t *testing.T) {
	acceptance(t)
	root := filepath.Join(t.TempDir(), "app")
	mustRubric(t, "init", root, "--module", "example.com/webapp", "--http", "chi", "--web", "htmx", "--e2e", "playwright",
		"--database", "sqlite", "--makefile", "--lint", "--actions")
	assertGuidancePaths(t, root)
	mustCommand(t, root, nil, "go", "mod", "tidy")
	mustCommand(t, root, nil, "sh", ".rubric/check.sh", "test")
	mustCommand(t, root, nil, "sh", ".rubric/check.sh", "lint")
	mustCommand(t, root, nil, "sh", ".rubric/check.sh", "generate-templ")
	mustCommand(t, root, nil, "sh", ".rubric/check.sh", "setup-e2e")
	out := mustCommand(t, root, nil, "go", "test", "-count=1", "-tags", "e2e", "-v", "./e2e/...")
	if !strings.Contains(out, "--- PASS: TestCounterAndToggle") {
		t.Fatalf("browser test did not pass:\n%s", out)
	}
	mustCommand(t, root, nil, "sh", ".rubric/check.sh", "test-e2e")
	r := mustRubric(t, "init", root, "--format", "json")
	if strings.Contains(r.out, `"state": "update"`) || strings.Contains(r.out, `"state": "create"`) {
		t.Fatalf("rerun is not a no-op:\n%s", r.out)
	}
}
