package wizard

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/plan"
	"github.com/michael-duren/go-skills/internal/write"
)

func TestEditingCommandsPreservesUntouchedRows(t *testing.T) {
	root := t.TempDir()
	saved := "schema: 1\nentry_points:\n  - name: api-server\n    dir: cmd/api\ncommands:\n" +
		"  - name: it\n    dir: .\n    argv: [go, test, -tags, integration, ./...]\n    env: [TEST_DATABASE_URL]\n" +
		"  - name: say\n    dir: tools\n    argv: [echo, \"a b; c\"]\n    env: []\n"
	for name, body := range map[string]string{
		"go.mod":          "module example.com/app\n\ngo 1.26.7\n",
		"cmd/api/main.go": "package main\n\nfunc main() {}\n",
		"tools/x.txt":     "x",
		"rubric.yaml":     saved,
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := start(t, initialize.Request{Target: root, Mode: "auto"}, realBackend())
	m = m.focus(t, "commands")
	m = drive(t, m, typeText("; hi=echo hi")...)
	m = m.focus(t, "entry_points")
	m = drive(t, m, bksp)
	m = drive(t, m, key('i'))
	req := m.request()
	cmds := req.Overrides["commands"].([]config.Command)
	want := []config.Command{
		{Name: "it", Dir: ".", Argv: []string{"go", "test", "-tags", "integration", "./..."}, Env: []string{"TEST_DATABASE_URL"}},
		{Name: "say", Dir: "tools", Argv: []string{"echo", "a b; c"}, Env: []string{}},
	}
	for i, w := range want {
		if !reflect.DeepEqual(cmds[i], w) {
			t.Fatalf("row %d = %+v, want %+v", i, cmds[i], w)
		}
	}
	if cmds[len(cmds)-1].Name != "hi" {
		t.Fatalf("new row missing: %+v", cmds)
	}
	eps := req.Overrides["entry_points"].([]config.EntryPoint)
	if len(eps) != 1 || eps[0].Name != "api-server" {
		t.Fatalf("entry point renamed: %+v", eps)
	}
}

func manyActions(t *testing.T) Model {
	t.Helper()
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, realBackend())
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter)
	m = m.setChoice(t, "features.http", "chi")
	m = drive(t, m, enter)
	for _, k := range []string{"tooling.skills", "tooling.lint", "tooling.makefile", "tooling.actions"} {
		m = m.toggle(t, k)
	}
	m = drive(t, m, enter)
	if len(m.plan.Actions) < 15 {
		t.Fatalf("fixture has only %d actions", len(m.plan.Actions))
	}
	return m
}

func TestReviewScrollsToSelection(t *testing.T) {
	m := drive(t, manyActions(t), tea.WindowSizeMsg{Width: 80, Height: 12})
	for range len(m.plan.Actions) {
		m = drive(t, m, down)
	}
	view := m.View().Content
	last := m.plan.Actions[len(m.plan.Actions)-1].File.Path
	if !strings.Contains(view, "> ") || !strings.Contains(view, last) {
		t.Fatalf("selected row not visible:\n%s", view)
	}
	if !strings.Contains(view, "enter apply") || len(strings.Split(view, "\n")) > 12 {
		t.Fatalf("help line missing or view too tall:\n%s", view)
	}
}

func TestReviewPreviewScrolls(t *testing.T) {
	m := drive(t, manyActions(t), tea.WindowSizeMsg{Width: 80, Height: 10})
	m = m.selectAction(t, ".rubric/style/analyze.go")
	m = drive(t, m, key('p'))
	first := m.View().Content
	if !strings.Contains(first, "Preview of .rubric/style/analyze.go") || !strings.Contains(first, "p close") {
		t.Fatalf("preview screen:\n%s", first)
	}
	m = drive(t, m, down, down, down)
	if m.View().Content == first {
		t.Fatal("preview did not scroll")
	}
	m = drive(t, m, key('p'))
	if strings.Contains(m.View().Content, "Preview of") {
		t.Fatal("preview did not close")
	}
}

func TestSkippingCheckScriptDisablesDependentTooling(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rubric"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rubric", "check.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := start(t, initialize.Request{Target: root, Mode: "new"}, realBackend())
	m = drive(t, m, typeText("example.com/demo")...)
	m = drive(t, m, enter, enter)
	m = m.toggle(t, "tooling.makefile")
	m = m.toggle(t, "tooling.actions")
	m = drive(t, m, enter)
	m = m.selectAction(t, ".rubric/check.sh")
	m = drive(t, m, key('s'))
	if m.value("tooling.makefile") != "false" || m.value("tooling.actions") != "false" || len(plan.Conflicts(m.plan)) != 0 {
		t.Fatalf("makefile=%s actions=%s conflicts=%v", m.value("tooling.makefile"), m.value("tooling.actions"), plan.Conflicts(m.plan))
	}
}

func TestRunWaitsForApplyAfterExternalCancel(t *testing.T) {
	var prepares atomic.Int32
	applying := make(chan struct{})
	backend := Backend{
		Prepare: func(ctx context.Context, req initialize.Request) (plan.Plan, error) {
			defer prepares.Add(1)
			return initialize.Prepare(ctx, req)
		},
		Apply: func(ctx context.Context, _ initialize.Request, _ plan.Plan) (write.Result, error) {
			close(applying)
			<-ctx.Done()
			time.Sleep(50 * time.Millisecond)
			return write.Result{Restored: []string{"go.mod"}}, ctx.Err()
		},
	}
	inR, inW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type ran struct {
		outcome Outcome
		err     error
	}
	done := make(chan ran, 1)
	go func() {
		o, err := runWith(ctx, initialize.Request{Target: t.TempDir(), Mode: "auto"}, backend, inR, io.Discard)
		done <- ran{o, err}
	}()
	waitFor := func(cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatal("timed out")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitFor(func() bool { return prepares.Load() >= 1 })
	if _, err := io.WriteString(inW, "example.com/demo\r\r\r"); err != nil {
		t.Fatal(err)
	}
	waitFor(func() bool { return prepares.Load() >= 2 })
	time.Sleep(100 * time.Millisecond)
	if _, err := io.WriteString(inW, "\r"); err != nil {
		t.Fatal(err)
	}
	<-applying
	cancel()
	r := <-done
	if !r.outcome.Cancelled || len(r.outcome.Result.Restored) != 1 {
		t.Fatalf("outcome = %+v err = %v", r.outcome, r.err)
	}
	_ = inW.Close()
}
