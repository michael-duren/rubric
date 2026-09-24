package wizard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/plan"
	"github.com/michael-duren/go-skills/internal/write"
)

var (
	enter = tea.KeyPressMsg{Code: tea.KeyEnter}
	esc   = tea.KeyPressMsg{Code: tea.KeyEscape}
	down  = tea.KeyPressMsg{Code: tea.KeyDown}
	up    = tea.KeyPressMsg{Code: tea.KeyUp}
	right = tea.KeyPressMsg{Code: tea.KeyRight}
	left  = tea.KeyPressMsg{Code: tea.KeyLeft}
	space = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	bksp  = tea.KeyPressMsg{Code: tea.KeyBackspace}
	ctrlC = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
)

func key(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func typeText(s string) []tea.Msg {
	var out []tea.Msg
	for _, r := range s {
		out = append(out, key(r))
	}
	return out
}

func drive(t *testing.T, m Model, msgs ...tea.Msg) Model {
	t.Helper()
	for _, msg := range msgs {
		m = step(t, m, msg)
	}
	return m
}

func step(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, cmd := m.Update(msg)
	model, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T", next)
	}
	for cmd != nil {
		out := cmd()
		cmd = nil
		switch out.(type) {
		case nil, tea.QuitMsg:
		default:
			next, cmd = model.Update(out)
			model = next.(Model)
		}
	}
	return model
}

func start(t *testing.T, req initialize.Request, backend Backend) Model {
	t.Helper()
	m := New(t.Context(), req, backend)
	if cmd := m.Init(); cmd != nil {
		next, _ := m.Update(cmd())
		m = next.(Model)
	}
	return m
}

func realBackend() Backend {
	return Backend{Prepare: initialize.Prepare, Apply: initialize.Apply}
}

func TestCancelNeverApplies(t *testing.T) {
	applied := false
	backend := Backend{
		Prepare: initialize.Prepare,
		Apply: func(context.Context, initialize.Request, plan.Plan) (write.Result, error) {
			applied = true
			return write.Result{}, nil
		},
	}
	model := New(t.Context(), initialize.Request{Target: t.TempDir(), Mode: "auto"}, backend)
	next, command := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if command != nil {
		command()
	}
	if applied || !next.(Model).outcome.Cancelled {
		t.Fatal("cancellation applied changes")
	}
}

func TestCancelAtReviewNeverApplies(t *testing.T) {
	applied := false
	backend := realBackend()
	backend.Apply = func(context.Context, initialize.Request, plan.Plan) (write.Result, error) {
		applied = true
		return write.Result{}, nil
	}
	root := t.TempDir()
	m := start(t, initialize.Request{Target: root, Mode: "auto"}, backend)
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter, enter, enter)
	if m.stage != reviewStage || !m.planned {
		t.Fatalf("stage=%v planned=%v message=%q", m.stage, m.planned, m.message)
	}
	m = drive(t, m, ctrlC)
	if applied || !m.outcome.Cancelled {
		t.Fatal("cancel at review applied")
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatal("cancelled wizard wrote files")
	}
}

func TestInvalidModuleStaysOnTarget(t *testing.T) {
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, realBackend())
	m = drive(t, m, enter)
	if m.stage != targetStage || !strings.Contains(m.message, "module") {
		t.Fatalf("empty module accepted: stage=%v message=%q", m.stage, m.message)
	}
	m = drive(t, m, typeText("bad module")...)
	m = drive(t, m, enter)
	if m.stage != targetStage || m.message == "" {
		t.Fatalf("invalid module accepted: %q", m.message)
	}
	if !strings.Contains(m.View().Content, m.message) {
		t.Fatal("validation error not displayed")
	}
}

func TestPartialFieldEdits(t *testing.T) {
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, realBackend())
	m = drive(t, m, typeText("example.com/demox")...)
	m = drive(t, m, bksp)
	if got := m.value("project.module"); got != "example.com/demo" {
		t.Fatalf("module = %q", got)
	}
	m = drive(t, m, down, down)
	m = drive(t, m, typeText("q is not quit")...)
	if m.value("project.description") != "q is not quit" || m.outcome.Cancelled {
		t.Fatalf("description = %q", m.value("project.description"))
	}
}

func TestBackNavigationRetainsAnswers(t *testing.T) {
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, realBackend())
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter)
	m = m.setChoice(t, "features.http", "chi")
	m = drive(t, m, enter)
	m = m.toggle(t, "tooling.lint")
	m = drive(t, m, esc, esc)
	if m.stage != targetStage || m.value("project.module") != "example.com/demo" {
		t.Fatalf("stage=%v module=%q", m.stage, m.value("project.module"))
	}
	m = drive(t, m, enter, enter)
	if m.value("features.http") != "chi" || m.value("tooling.lint") != "true" {
		t.Fatalf("answers lost: http=%q lint=%q", m.value("features.http"), m.value("tooling.lint"))
	}
}

func TestDisablingDatabaseClearsAccess(t *testing.T) {
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, realBackend())
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter)
	m = m.setChoice(t, "features.database", "postgres")
	if !m.visible("features.access") {
		t.Fatal("access hidden while a database is selected")
	}
	m = m.setChoice(t, "features.access", "sqlc")
	m = m.setChoice(t, "features.database", "none")
	if m.visible("features.access") {
		t.Fatal("access shown without a database")
	}
	if _, ok := m.request().Overrides["features.access"]; ok {
		t.Fatal("stale access choice kept after disabling the database")
	}
	m = drive(t, m, enter, enter)
	if m.stage != reviewStage || m.plan.Config.Features.Access != "none" || m.message != "" {
		t.Fatalf("stage=%v access=%q message=%q", m.stage, m.plan.Config.Features.Access, m.message)
	}
}

func TestWebChoicesFollowHTTP(t *testing.T) {
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, realBackend())
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter)
	if m.visible("features.web") || m.visible("features.e2e") {
		t.Fatal("web choices shown without an HTTP server")
	}
	m = m.setChoice(t, "features.http", "chi")
	m = m.setChoice(t, "features.web", "htmx")
	m = m.setChoice(t, "features.e2e", "playwright")
	m = drive(t, m, enter, enter)
	if m.message != "" || m.plan.Config.Features.Web != "htmx" || m.plan.Config.Features.E2E != "playwright" {
		t.Fatalf("message=%q features=%+v", m.message, m.plan.Config.Features)
	}
	m = drive(t, m, esc, esc)
	m = m.setChoice(t, "features.http", "none")
	if m.visible("features.web") || m.visible("features.e2e") {
		t.Fatal("web choices still shown")
	}
	req := m.request()
	if _, ok := req.Overrides["features.web"]; ok {
		t.Fatal("stale web choice kept")
	}
	if _, ok := req.Overrides["features.e2e"]; ok {
		t.Fatal("stale e2e choice kept")
	}
	m = drive(t, m, enter, enter)
	if m.message != "" || m.plan.Config.Features.Web != "none" {
		t.Fatalf("message=%q features=%+v", m.message, m.plan.Config.Features)
	}
}

func TestStarterHiddenWithExecutables(t *testing.T) {
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, realBackend())
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter)
	if !m.visible("project.starter") {
		t.Fatal("starter hidden without executables")
	}
	m = m.setChoice(t, "features.cli", "flag")
	if m.visible("project.starter") {
		t.Fatal("starter shown with an executable")
	}
}

func TestZeroSizeTerminal(t *testing.T) {
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, realBackend())
	m = drive(t, m, tea.WindowSizeMsg{Width: 0, Height: 0})
	if m.View().Content == "" {
		t.Fatal("empty view")
	}
	m = drive(t, m, tea.WindowSizeMsg{Width: 12, Height: 3})
	for _, line := range strings.Split(m.View().Content, "\n") {
		if len([]rune(line)) > minWidth {
			t.Fatalf("line %q wider than the clamped width", line)
		}
	}
}

func TestStaleAndDuplicatePrepareIgnored(t *testing.T) {
	calls := 0
	backend := realBackend()
	backend.Prepare = func(ctx context.Context, req initialize.Request) (plan.Plan, error) {
		calls++
		return initialize.Prepare(ctx, req)
	}
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, backend)
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter, enter)
	next, cmd := m.Update(enter)
	m = next.(Model)
	if !m.preparing || cmd == nil {
		t.Fatal("review did not start preparing")
	}
	next, dup := m.Update(enter)
	m = next.(Model)
	if dup != nil {
		t.Fatal("duplicate key while preparing started another command")
	}
	result := cmd()
	stale := planMsg{id: m.reqID - 1, plan: plan.Plan{Mode: "stale"}}
	m = step(t, m, stale)
	if m.plan.Mode == "stale" || !m.preparing {
		t.Fatal("stale response accepted")
	}
	m = step(t, m, result)
	if m.preparing || m.plan.Mode != "new" {
		t.Fatalf("current response not accepted: %+v", m.plan.Mode)
	}
	if calls != 2 {
		t.Fatalf("prepare calls = %d, want baseline + review", calls)
	}
}

func TestApplyError(t *testing.T) {
	backend := realBackend()
	want := errors.New("disk full")
	backend.Apply = func(context.Context, initialize.Request, plan.Plan) (write.Result, error) {
		return write.Result{Restored: []string{"go.mod"}}, want
	}
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, backend)
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter, enter, enter, enter)
	if !errors.Is(m.applyErr, want) || m.stage != doneStage || m.outcome.Cancelled {
		t.Fatalf("stage=%v err=%v", m.stage, m.applyErr)
	}
	if !strings.Contains(m.View().Content, "disk full") {
		t.Fatalf("apply error not shown:\n%s", m.View().Content)
	}
}

func TestCancelDuringApply(t *testing.T) {
	started := make(chan struct{})
	backend := realBackend()
	backend.Apply = func(ctx context.Context, _ initialize.Request, _ plan.Plan) (write.Result, error) {
		close(started)
		<-ctx.Done()
		return write.Result{}, ctx.Err()
	}
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, backend)
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter, enter, enter)
	next, cmd := m.Update(enter)
	m = next.(Model)
	if m.stage != applyStage || cmd == nil {
		t.Fatalf("stage = %v", m.stage)
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	<-started
	m = step(t, m, ctrlC)
	m = step(t, m, <-done)
	if !m.outcome.Cancelled || !errors.Is(m.applyErr, context.Canceled) {
		t.Fatalf("cancelled=%v err=%v", m.outcome.Cancelled, m.applyErr)
	}
}

func TestReviewDecisions(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("all:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := start(t, initialize.Request{Target: root, Mode: "new"}, realBackend())
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter, enter)
	m = m.toggle(t, "tooling.makefile")
	m = drive(t, m, enter)
	if len(plan.Conflicts(m.plan)) != 2 {
		t.Fatalf("conflicts = %+v", plan.Conflicts(m.plan))
	}
	m = drive(t, m, enter)
	if m.stage != reviewStage || !strings.Contains(m.message, "conflict") {
		t.Fatalf("applied with conflicts: stage=%v", m.stage)
	}
	m = m.selectAction(t, "Makefile")
	m = drive(t, m, key('p'))
	if !strings.Contains(m.View().Content, "-all:") {
		t.Fatalf("diff preview missing:\n%s", m.View().Content)
	}
	m = drive(t, m, key('p'), key('s'))
	if m.value("tooling.makefile") != "false" {
		t.Fatal("skipping the Makefile did not disable it")
	}
	m = m.selectAction(t, "README.md")
	m = drive(t, m, key('r'))
	if len(plan.Conflicts(m.plan)) != 0 {
		t.Fatalf("conflicts remain: %+v", plan.Conflicts(m.plan))
	}
	m = drive(t, m, enter)
	if m.applyErr != nil || m.stage != doneStage {
		t.Fatalf("apply: %v", m.applyErr)
	}
	readme, _ := os.ReadFile(filepath.Join(root, "README.md"))
	mk, _ := os.ReadFile(filepath.Join(root, "Makefile"))
	if string(readme) == "mine\n" || string(mk) != "all:\n" {
		t.Fatalf("decisions not honored: README=%q Makefile=%q", readme, mk)
	}
	if strings.Contains(m.View().Content, "tests passed") || !strings.Contains(m.View().Content, "did not download dependencies") {
		t.Fatalf("final view:\n%s", m.View().Content)
	}
}

func TestSkipMandatoryRejected(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "rubric.yaml"), []byte("schema: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("<!-- rubric:begin -->\nmine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := start(t, initialize.Request{Target: root, Mode: "new"}, realBackend())
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter, enter, enter)
	m = m.selectAction(t, "AGENTS.md")
	m = drive(t, m, key('s'))
	if m.message == "" || len(plan.Conflicts(m.plan)) == 0 {
		t.Fatalf("mandatory skip accepted: %q", m.message)
	}
}

func (m Model) setChoice(t *testing.T, key, value string) Model {
	t.Helper()
	m = m.focus(t, key)
	for range 8 {
		if m.value(key) == value {
			return m
		}
		m = drive(t, m, right)
	}
	t.Fatalf("%s never reached %q", key, value)
	return m
}

func (m Model) toggle(t *testing.T, key string) Model {
	t.Helper()
	return drive(t, m.focus(t, key), space)
}

func (m Model) focus(t *testing.T, key string) Model {
	t.Helper()
	for range 20 {
		if f := m.current(); f != nil && f.key == key {
			return m
		}
		m = drive(t, m, down)
	}
	for range 20 {
		if f := m.current(); f != nil && f.key == key {
			return m
		}
		m = drive(t, m, up)
	}
	t.Fatalf("field %s not reachable on stage %v", key, m.stage)
	return m
}

func (m Model) selectAction(t *testing.T, path string) Model {
	t.Helper()
	for range len(m.plan.Actions) + 1 {
		if m.actionCursor < len(m.plan.Actions) && m.plan.Actions[m.actionCursor].File.Path == path {
			return m
		}
		m = drive(t, m, down)
	}
	m = drive(t, m, up)
	for range len(m.plan.Actions) {
		if m.plan.Actions[m.actionCursor].File.Path == path {
			return m
		}
		m = drive(t, m, up)
	}
	t.Fatalf("action %s not found", path)
	return m
}

var _ = left
