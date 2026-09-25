package wizard

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/plan"
)

func initialized(t *testing.T, overrides config.Patch) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n\ngo 1.26.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := initialize.Request{Target: root, Mode: "existing", Overrides: overrides}
	p, err := initialize.Prepare(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := initialize.Apply(context.Background(), req, p); err != nil {
		t.Fatal(err)
	}
	return root
}

func startUpdate(t *testing.T, root string) Model {
	t.Helper()
	m := NewUpdate(t.Context(), initialize.Request{Target: root}, realBackend())
	if cmd := m.Init(); cmd != nil {
		next, _ := m.Update(cmd())
		m = next.(Model)
	}
	return m
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return !errors.Is(err, fs.ErrNotExist)
}

func TestUpdateMenuShowsActiveFeatures(t *testing.T) {
	root := initialized(t, config.Patch{"tooling.skills": []string{"rubric"}, "tooling.makefile": true})
	view := startUpdate(t, root).plainView()
	for _, want := range []string{"rubric update", "Features", "Skills", "Rubric skills", "(1 of 4 on)", "Tooling", "Makefile", "(1 of 3 on)"} {
		if !strings.Contains(view, want) {
			t.Errorf("menu missing %q:\n%s", want, view)
		}
	}
}

func TestUpdateTogglesAndReconcilesFiles(t *testing.T) {
	root := initialized(t, config.Patch{"tooling.skills": config.SkillGroups, "tooling.makefile": true})
	m := startUpdate(t, root)
	m = drive(t, m, enter)
	m = m.toggle(t, "skills.principles")
	if m.value("skills.pstack") != "false" || m.value("skills.agents") != "false" || m.value("skills.rubric") != "true" {
		t.Fatalf("cascade: pstack=%s agents=%s rubric=%s", m.value("skills.pstack"), m.value("skills.agents"), m.value("skills.rubric"))
	}
	if !strings.Contains(m.plainView(), "pstack principles *") {
		t.Fatalf("changed toggle not marked:\n%s", m.plainView())
	}
	m = drive(t, m, esc, down, enter)
	m = m.toggle(t, "tooling.lint")
	m = drive(t, m, esc)
	if !strings.Contains(m.plainView(), "Unsaved changes") {
		t.Fatalf("unsaved changes not shown:\n%s", m.plainView())
	}
	m = drive(t, m, key('s'))
	if m.stage != reviewStage || !m.planned {
		t.Fatalf("stage=%v planned=%v message=%q", m.stage, m.planned, m.message)
	}
	states := map[string]string{}
	for _, a := range m.plan.Actions {
		states[a.File.Path] = a.State
	}
	for path, want := range map[string]string{
		".agents/agents/poteto-agent.md":          plan.StateDelete,
		".agents/skills/poteto-mode/SKILL.md":     plan.StateDelete,
		".agents/skills/THIRD_PARTY_NOTICES.md":   plan.StateDelete,
		".agents/skills/rubric-workflow/SKILL.md": plan.StateUnchanged,
		".golangci.yml":                           plan.StateCreate,
		"Makefile":                                plan.StateUpdate,
	} {
		if states[path] != want {
			t.Errorf("%s = %q, want %q", path, states[path], want)
		}
	}
	m = drive(t, m, enter)
	if m.stage != doneStage || m.applyErr != nil {
		t.Fatalf("stage=%v err=%v", m.stage, m.applyErr)
	}
	if exists(root, ".agents/agents") || exists(root, ".agents/skills/poteto-mode") || !exists(root, ".agents/skills/rubric-workflow/SKILL.md") {
		t.Fatal("skill files not reconciled")
	}
	if !strings.Contains(m.plainView(), "Deleted") {
		t.Fatalf("done view omits deletions:\n%s", m.plainView())
	}
	saved, err := os.ReadFile(filepath.Join(root, "rubric.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := config.Decode(saved)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Values["tooling.skills"].([]string); !slices.Equal(got, []string{"rubric"}) || doc.Values["tooling.lint"] != true {
		t.Fatalf("rubric.yaml = %v", doc.Values)
	}
	again := startUpdate(t, root)
	again = drive(t, again, key('s'))
	for _, a := range again.plan.Actions {
		if a.State != plan.StateUnchanged {
			t.Errorf("rerun not a no-op: %s %s", a.State, a.File.Path)
		}
	}
}

func TestUpdateEditedObsoleteFileCanBeKept(t *testing.T) {
	root := initialized(t, config.Patch{"tooling.makefile": true})
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("mine:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := startUpdate(t, root)
	m = drive(t, m, down, enter)
	m = m.toggle(t, "tooling.makefile")
	m = drive(t, m, key('s'))
	i := slices.IndexFunc(m.plan.Actions, func(a plan.Action) bool { return a.File.Path == "Makefile" })
	if i < 0 || m.plan.Actions[i].State != plan.StateConflict {
		t.Fatalf("Makefile action = %+v", m.plan.Actions)
	}
	m.actionCursor = i
	m = drive(t, m, key('s'))
	if got := m.plan.Actions[i]; got.State != plan.StateSkip || m.value("tooling.makefile") != "false" {
		t.Fatalf("keep decision = %+v makefile=%s", got, m.value("tooling.makefile"))
	}
	m = drive(t, m, enter)
	if m.applyErr != nil || !exists(root, "Makefile") {
		t.Fatalf("err=%v kept=%v", m.applyErr, exists(root, "Makefile"))
	}
	manifest, err := os.ReadFile(filepath.Join(root, plan.ManifestPath))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifest), `"Makefile"`) {
		t.Fatal("kept file still owned by Rubric")
	}
}

func TestUpdateQuitWithoutChangesIsNotCancelled(t *testing.T) {
	m := startUpdate(t, initialized(t, nil))
	next, cmd := m.Update(esc)
	if cmd == nil || next.(Model).outcome.Cancelled {
		t.Fatal("quit without changes should exit cleanly")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("esc did not quit")
	}
	m = drive(t, m, enter, space, esc)
	next, _ = m.Update(esc)
	if !next.(Model).outcome.Cancelled {
		t.Fatal("quitting with unsaved changes should cancel")
	}
}

func TestUpdateFlagOverridesShowAsUnsavedChanges(t *testing.T) {
	root := initialized(t, nil)
	m := NewUpdate(t.Context(), initialize.Request{Target: root, Overrides: config.Patch{"tooling.lint": true}}, realBackend())
	next, _ := m.Update(m.Init()())
	m = next.(Model)
	if m.value("tooling.lint") != "true" || !m.changed("tooling.lint") || !strings.Contains(m.plainView(), "Unsaved changes") {
		t.Fatalf("lint=%s changed=%v\n%s", m.value("tooling.lint"), m.changed("tooling.lint"), m.plainView())
	}
}
