package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/wizard"
)

func initializedRepo(t *testing.T, flags ...string) string {
	t.Helper()
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/app\n\ngo 1.26.7\n")
	if r := invoke(t.Context(), t, append([]string{"init", root, "--non-interactive"}, flags...)...); r.code != 0 {
		t.Fatalf("init: exit %d %s", r.code, r.stderr)
	}
	return root
}

func present(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return !errors.Is(err, fs.ErrNotExist)
}

func TestUpdateRequiresRubricYAML(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/app\n")
	r := invoke(t.Context(), t, "update", root, "--lint")
	if r.code != 2 || !strings.Contains(r.stderr, "run rubric init first") {
		t.Fatalf("exit %d stderr %q", r.code, r.stderr)
	}
}

func TestUpdateListShowsActiveFeaturesAndDrift(t *testing.T) {
	root := initializedRepo(t, "--skills=rubric", "--makefile")
	r := invoke(t.Context(), t, "update", root, "--list")
	if r.code != 0 {
		t.Fatalf("exit %d %s", r.code, r.stderr)
	}
	for _, want := range []string{"[x] Rubric skills", "[ ] pstack skills", "[x] Makefile", "[ ] Linting", "Files match rubric.yaml."} {
		if !strings.Contains(r.out, want) {
			t.Errorf("list missing %q:\n%s", want, r.out)
		}
	}
	if err := os.Remove(filepath.Join(root, "Makefile")); err != nil {
		t.Fatal(err)
	}
	r = invoke(t.Context(), t, "update", root, "--list", "--format", "json")
	var l struct {
		Skills  map[string]bool
		Tooling map[string]bool
		Pending []struct{ Path, State string }
	}
	if err := json.Unmarshal([]byte(r.out), &l); err != nil {
		t.Fatalf("%v: %q", err, r.out)
	}
	if !l.Skills["rubric"] || l.Skills["pstack"] || !l.Tooling["makefile"] || l.Tooling["lint"] {
		t.Fatalf("list = %+v", l)
	}
	if len(l.Pending) != 1 || l.Pending[0].Path != "Makefile" || l.Pending[0].State != "create" {
		t.Fatalf("pending = %+v", l.Pending)
	}
	if r := invoke(t.Context(), t, "update", root, "--list", "--lint"); r.code != 2 {
		t.Fatalf("--list with toggles: exit %d", r.code)
	}
}

func TestUpdateFlagsCreateAndDeleteFiles(t *testing.T) {
	root := initializedRepo(t, "--skills", "--makefile")
	r := invoke(t.Context(), t, "update", root, "--skills=rubric", "--makefile=false", "--lint")
	if r.code != 0 {
		t.Fatalf("exit %d %s\n%s", r.code, r.stderr, r.out)
	}
	if !strings.Contains(r.out, "rubric update:") || !strings.Contains(r.out, "delete ") || !strings.Contains(r.out, ", deleted ") {
		t.Fatalf("report:\n%s", r.out)
	}
	if present(root, "Makefile") || present(root, ".agents/agents") || present(root, ".agents/skills/poteto-mode") {
		t.Fatal("disabled feature files remain")
	}
	if !present(root, ".golangci.yml") || !present(root, ".agents/skills/rubric-workflow/SKILL.md") {
		t.Fatal("enabled feature files missing")
	}
	rerun := invoke(t.Context(), t, "update", root, "--format", "json")
	rep := decode(t, rerun)
	for _, a := range rep.Actions {
		if a.State != "unchanged" {
			t.Errorf("rerun not a no-op: %s %s", a.State, a.Path)
		}
	}
}

func TestUpdateEditedObsoleteFileIsConflict(t *testing.T) {
	root := initializedRepo(t, "--makefile")
	put(t, root, "Makefile", "mine:\n")
	r := invoke(t.Context(), t, "update", root, "--makefile=false")
	if r.code != 2 || !strings.Contains(r.stderr, "no longer generates") {
		t.Fatalf("exit %d stderr %q", r.code, r.stderr)
	}
	if !present(root, "Makefile") {
		t.Fatal("edited file deleted")
	}
}

func TestUpdateSkillsFlagRejectsUnknownGroups(t *testing.T) {
	root := initializedRepo(t)
	if r := invoke(t.Context(), t, "update", root, "--skills=rubric,nope"); r.code != 2 || !strings.Contains(r.stderr, "--skills") {
		t.Fatalf("exit %d stderr %q", r.code, r.stderr)
	}
	if r := invoke(t.Context(), t, "update", root, "--skills=agents"); r.code != 2 || !strings.Contains(r.stderr, "agents requires pstack") {
		t.Fatalf("exit %d stderr %q", r.code, r.stderr)
	}
}

func TestUpdateTerminalUsesMenuWizard(t *testing.T) {
	root := initializedRepo(t)
	var got []initialize.Request
	previous := runUpdateWizard
	runUpdateWizard = func(_ context.Context, req initialize.Request, _ io.Reader, _ io.Writer) (wizard.Outcome, error) {
		got = append(got, req)
		return wizard.Outcome{}, nil
	}
	t.Cleanup(func() { runUpdateWizard = previous })
	r := terminal(t, "update", root, "--lint")
	if r.code != 0 || len(got) != 1 || !strings.Contains(r.out, "no changes") {
		t.Fatalf("exit %d calls %d out %q", r.code, len(got), r.out)
	}
	if got[0].Mode != "existing" || got[0].Overrides["tooling.lint"] != true {
		t.Fatalf("request = %+v", got[0])
	}
}
