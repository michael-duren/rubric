package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/wizard"
)

func stubWizard(t *testing.T, outcome wizard.Outcome, err error) *[]initialize.Request {
	t.Helper()
	var calls []initialize.Request
	previous := runWizard
	runWizard = func(_ context.Context, req initialize.Request, _ io.Reader, _ io.Writer) (wizard.Outcome, error) {
		calls = append(calls, req)
		return outcome, err
	}
	t.Cleanup(func() { runWizard = previous })
	return &calls
}

func terminal(t *testing.T, args ...string) run {
	t.Helper()
	var out, stderr bytes.Buffer
	code := Run(t.Context(), args, Streams{In: strings.NewReader(""), Out: &out, Err: &stderr, Terminal: true})
	return run{code: code, out: out.String(), stderr: stderr.String()}
}

func TestTerminalUsesWizardWithFlagAnswers(t *testing.T) {
	root := t.TempDir()
	calls := stubWizard(t, wizard.Outcome{}, nil)
	r := terminal(t, "init", root, "--module", "example.com/demo", "--lint")
	if r.code != 0 || len(*calls) != 1 {
		t.Fatalf("exit %d calls %d: %s", r.code, len(*calls), r.stderr)
	}
	req := (*calls)[0]
	if req.Target != root || req.Overrides["project.module"] != "example.com/demo" || req.Overrides["tooling.lint"] != true {
		t.Fatalf("request = %+v", req)
	}
}

func TestUnattendedPathsNeverStartWizard(t *testing.T) {
	calls := stubWizard(t, wizard.Outcome{}, nil)
	for _, args := range [][]string{
		{"--format", "json"},
		{"--non-interactive"},
		{"--dry-run"},
	} {
		r := terminal(t, append([]string{"init", t.TempDir(), "--module", "example.com/demo"}, args...)...)
		if r.code != 0 {
			t.Fatalf("%v: exit %d %s", args, r.code, r.stderr)
		}
	}
	if r := invoke(t, t.Context(), "init", t.TempDir(), "--module", "example.com/demo"); r.code != 0 {
		t.Fatalf("redirected: %+v", r)
	}
	if len(*calls) != 0 {
		t.Fatalf("wizard started %d times", len(*calls))
	}
}

func TestWizardCancellationExit130(t *testing.T) {
	stubWizard(t, wizard.Outcome{Cancelled: true}, context.Canceled)
	if r := terminal(t, "init", t.TempDir()); r.code != 130 {
		t.Fatalf("exit %d", r.code)
	}
}

func TestWizardApplyFailureExit1(t *testing.T) {
	stubWizard(t, wizard.Outcome{}, io.ErrShortWrite)
	r := terminal(t, "init", t.TempDir())
	if r.code != 1 || !strings.Contains(r.stderr, io.ErrShortWrite.Error()) {
		t.Fatalf("exit %d stderr %q", r.code, r.stderr)
	}
}
