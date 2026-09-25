package wizard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/plan"
)

func equivalent(t *testing.T, got, want plan.Plan) {
	t.Helper()
	if !reflect.DeepEqual(got.Config, want.Config) {
		t.Fatalf("config differs:\nwizard     %+v\nunattended %+v", got.Config, want.Config)
	}
	if len(got.Actions) != len(want.Actions) {
		t.Fatalf("actions differ: %d vs %d", len(got.Actions), len(want.Actions))
	}
	for i := range got.Actions {
		g, w := got.Actions[i], want.Actions[i]
		if g.File.Path != w.File.Path || g.State != w.State || !bytes.Equal(g.File.Data, w.File.Data) {
			t.Fatalf("action %s differs from unattended %s", g.File.Path, w.File.Path)
		}
	}
}

func TestFlowMatchesUnattendedNewProject(t *testing.T) {
	root := t.TempDir()
	m := start(t, initialize.Request{Target: root, Mode: "auto"}, realBackend())
	if m.mode != "new" {
		t.Fatalf("mode = %q", m.mode)
	}
	m = drive(t, m, typeText("example.com/shop")...)
	m = drive(t, m, down, down)
	m = drive(t, m, typeText("Shop service")...)
	m = drive(t, m, enter)
	m = m.setChoice(t, "features.http", "chi")
	m = m.setChoice(t, "features.database", "sqlite")
	m = m.setChoice(t, "features.access", "sqlc")
	m = m.setChoice(t, "features.config", "viper")
	m = drive(t, m, enter)
	m = m.toggle(t, "tooling.lint")
	m = m.toggle(t, "tooling.actions")
	m = drive(t, m, enter)
	if m.stage != reviewStage || m.message != "" {
		t.Fatalf("stage=%v message=%q", m.stage, m.message)
	}
	unattended, err := initialize.Prepare(context.Background(), initialize.Request{Target: root, Mode: "auto", Overrides: config.Patch{
		"project.module": "example.com/shop", "project.description": "Shop service", "features.http": "chi",
		"features.database": "sqlite", "features.access": "sqlc", "features.config": "viper",
		"tooling.lint": true, "tooling.actions": true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	equivalent(t, m.plan, unattended)
	if m.plan.Config.Features.CLI != "none" || m.plan.Config.Tooling.Makefile {
		t.Fatal("wizard enabled unselected components")
	}
	m = drive(t, m, enter)
	if m.stage != doneStage || m.applyErr != nil {
		t.Fatalf("apply: %v", m.applyErr)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "httpserver", "routes.go")); err != nil {
		t.Fatal(err)
	}
	if !m.outcome.Plan.Config.Tooling.Lint || len(m.outcome.Result.Applied) == 0 {
		t.Fatalf("outcome = %+v", m.outcome.Result)
	}
}

func TestFlowViperWithoutCobraAndStarters(t *testing.T) {
	for _, starter := range []string{"module", "runnable"} {
		t.Run(starter, func(t *testing.T) {
			root := t.TempDir()
			m := start(t, initialize.Request{Target: root, Mode: "auto"}, realBackend())
			m = drive(t, m, typeText("example.com/demo")...)
			m = drive(t, m, enter)
			m = m.setChoice(t, "project.starter", starter)
			if starter == "runnable" {
				m = m.setChoice(t, "features.config", "viper")
			}
			m = drive(t, m, enter, enter)
			patch := config.Patch{"project.module": "example.com/demo", "project.starter": starter}
			if starter == "runnable" {
				patch["features.config"] = "viper"
			}
			want, err := initialize.Prepare(context.Background(), initialize.Request{Target: root, Mode: "auto", Overrides: patch})
			if err != nil {
				t.Fatal(err)
			}
			equivalent(t, m.plan, want)
			if m.plan.Config.Features.CLI != "none" {
				t.Fatal("Viper selected Cobra")
			}
		})
	}
}

func TestFlowExistingProjectFacts(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":          "module example.com/app\n\ngo 1.22\n",
		"cmd/api/main.go": "package main\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc main() { _ = gin.Default().Run() }\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := start(t, initialize.Request{Target: root, Mode: "auto"}, realBackend())
	if m.mode != "existing" || !bytes.Contains([]byte(m.plainView()), []byte("example.com/app")) {
		t.Fatalf("mode=%q view:\n%s", m.mode, m.plainView())
	}
	if m.value("features.http") != "gin" || m.value("entry_points") != "cmd/api" {
		t.Fatalf("facts not shown: http=%q entries=%q", m.value("features.http"), m.value("entry_points"))
	}
	m = drive(t, m, enter, enter)
	m = m.toggle(t, "skills.agents")
	m = drive(t, m, enter)
	want, err := initialize.Prepare(context.Background(), initialize.Request{Target: root, Mode: "auto", Overrides: config.Patch{"tooling.skills": []string{"pstack", "principles", "agents"}}})
	if err != nil {
		t.Fatal(err)
	}
	equivalent(t, m.plan, want)
	for _, a := range m.plan.Actions {
		if a.File.Path == "cmd/api/main.go" || a.File.Path == "go.mod" {
			t.Fatal("existing application file in plan")
		}
	}
	m = drive(t, m, esc, esc, esc)
	m = m.focus(t, "entry_points")
	for range len("cmd/api") {
		m = drive(t, m, bksp)
	}
	m = drive(t, m, enter, enter, enter)
	if len(m.plan.Config.EntryPoints) != 0 {
		t.Fatalf("edited entry points ignored: %+v", m.plan.Config.EntryPoints)
	}
}

func TestRunOverPipesCancels(t *testing.T) {
	in := bytes.NewBufferString("\x03")
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	outcome, err := Run(ctx, initialize.Request{Target: t.TempDir(), Mode: "auto"}, in, &out)
	if !outcome.Cancelled {
		t.Fatalf("outcome = %+v err = %v", outcome, err)
	}
	var _ tea.Msg = nil
}
