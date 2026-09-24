package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

type run struct {
	code        int
	out, stderr string
}

func invoke(t *testing.T, ctx context.Context, args ...string) run {
	t.Helper()
	var out, stderr bytes.Buffer
	code := Run(ctx, args, Streams{In: strings.NewReader(""), Out: &out, Err: &stderr})
	return run{code: code, out: out.String(), stderr: stderr.String()}
}

type jsonReport struct {
	Status string `json:"status"`
	Mode   string `json:"mode"`
	DryRun bool   `json:"dry_run"`
	Config struct {
		Project  struct{ Module, Description, Go string }
		Features struct{ HTTP string }
		Tooling  struct{ Lint, Makefile bool }
		Commands []struct {
			Name string
			Argv []string
		}
		EntryPoints []struct{ Name, Dir string } `json:"entry_points"`
	} `json:"config"`
	Actions []struct {
		Path, State, Kind, Reason string
	} `json:"actions"`
	Conflicts []struct {
		Path, Reason string
	} `json:"conflicts"`
	Diagnostics []struct {
		Severity, Message string
	} `json:"diagnostics"`
	Next []struct {
		Name string
		Argv []string
	} `json:"next_commands"`
}

func decode(t *testing.T, r run) jsonReport {
	t.Helper()
	var rep jsonReport
	dec := json.NewDecoder(strings.NewReader(r.out))
	if err := dec.Decode(&rep); err != nil {
		t.Fatalf("invalid JSON (%v): %q stderr=%q", err, r.out, r.stderr)
	}
	if dec.More() {
		t.Fatalf("more than one JSON document: %q", r.out)
	}
	return rep
}

func put(t *testing.T, root, rel, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestJSONDryRunDoesNotWrite(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new project")
	var out, stderr bytes.Buffer
	args := []string{"init", root, "--module", "example.com/demo", "--format", "json", "--dry-run"}
	code := Run(t.Context(), args, Streams{In: strings.NewReader(""), Out: &out, Err: &stderr})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("invalid JSON: %s", out.String())
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("dry run wrote files: %v", err)
	}
	rep := decode(t, run{out: out.String()})
	if rep.Status != "dry-run" || rep.Mode != "new" || !rep.DryRun || len(rep.Actions) == 0 || stderr.Len() != 0 {
		t.Fatalf("report = %+v stderr=%q", rep, stderr.String())
	}
	if strings.Contains(out.String(), "module example.com/demo\n") {
		t.Fatal("raw file contents emitted")
	}
}

func TestHelp(t *testing.T) {
	r := invoke(t, t.Context(), "--help")
	if r.code != 0 || !strings.Contains(r.out, "init") {
		t.Fatalf("root help: %+v", r)
	}
	r = invoke(t, t.Context(), "init", "--help")
	for _, want := range []string{"--module", "--non-interactive", "--dry-run", "--format", "--entry-point", "--clear-commands", "--app-config"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("init help missing %s", want)
		}
	}
	if r.code != 0 {
		t.Fatalf("exit %d", r.code)
	}
}

func TestUsageErrorsExit2(t *testing.T) {
	for _, args := range [][]string{
		{"bogus"},
		{"init", "--bogus"},
		{"init", "--format", "yaml"},
		{"init", "a", "b"},
		{"init", "--lint=maybe"},
	} {
		r := invoke(t, t.Context(), args...)
		if r.code != 2 || r.stderr == "" {
			t.Errorf("%v: exit %d stderr %q", args, r.code, r.stderr)
		}
	}
}

func TestMissingModuleFailsBeforeWrites(t *testing.T) {
	root := t.TempDir()
	r := invoke(t, t.Context(), "init", root, "--format", "json")
	rep := decode(t, r)
	if r.code != 2 || rep.Status != "invalid" || len(rep.Diagnostics) == 0 || !strings.Contains(rep.Diagnostics[0].Message, "project.module") {
		t.Fatalf("exit %d report %+v", r.code, rep)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatal("wrote files")
	}
	r = invoke(t, t.Context(), "init", root)
	if r.code != 2 || !strings.Contains(r.stderr, "project.module") {
		t.Fatalf("text: %+v", r)
	}
}

func TestConfigFlagPrecedenceAndExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "input.yaml")
	put(t, dir, "input.yaml", "project:\n  module: example.com/fromfile\n  description: from file\ntooling:\n  lint: true\n  makefile: true\n")
	root := filepath.Join(dir, "proj")
	r := invoke(t, t.Context(), "init", root, "--config", cfg, "--description", "from flag", "--lint=false", "--format", "json", "--dry-run")
	rep := decode(t, r)
	c := rep.Config
	if r.code != 0 || c.Project.Module != "example.com/fromfile" || c.Project.Description != "from flag" || c.Tooling.Lint || !c.Tooling.Makefile {
		t.Fatalf("exit %d config %+v", r.code, c)
	}
}

func TestBadConfigInputs(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "schema.yaml", "schema: 2\nproject:\n  module: example.com/x\n")
	for _, args := range [][]string{
		{"--config", filepath.Join(dir, "schema.yaml")},
		{"--config", filepath.Join(dir, "missing.yaml")},
		{"--module", "example.com/x", "--http", "gin"},
		{"--module", "bad module"},
		{"--module", "example.com/x", "--entry-point", "{not json"},
		{"--module", "example.com/x", "--command", `{"name":"t","argv":["go"],"shell":"sh -c"}`},
		{"--module", "example.com/x", "--entry-point", `{"name":"a","dir":"cmd/a"}`, "--clear-entry-points"},
	} {
		r := invoke(t, t.Context(), append([]string{"init", filepath.Join(dir, "p"), "--format", "json"}, args...)...)
		if r.code != 2 {
			t.Errorf("%v: exit %d out %s", args, r.code, r.out)
			continue
		}
		decode(t, r)
	}
}

func TestDryRunConflictsExit2(t *testing.T) {
	root := t.TempDir()
	put(t, root, "README.md", "mine\n")
	r := invoke(t, t.Context(), "init", root, "--mode", "new", "--module", "example.com/demo", "--format", "json", "--dry-run")
	rep := decode(t, r)
	if r.code != 2 || rep.Status != "conflict" || len(rep.Conflicts) != 1 || rep.Conflicts[0].Path != "README.md" {
		t.Fatalf("exit %d report %+v", r.code, rep)
	}
	r = invoke(t, t.Context(), "init", root, "--mode", "new", "--module", "example.com/demo")
	if r.code != 2 || !strings.Contains(r.stderr+r.out, "README.md") {
		t.Fatalf("apply with conflict: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("wrote despite conflict")
	}
}

func TestNonemptyNonModuleNeedsExplicitNew(t *testing.T) {
	root := t.TempDir()
	put(t, root, "notes.txt", "x")
	if r := invoke(t, t.Context(), "init", root, "--module", "example.com/demo"); r.code != 2 || !strings.Contains(r.stderr, "--mode new") {
		t.Fatalf("auto: %+v", r)
	}
	if r := invoke(t, t.Context(), "init", root, "--module", "example.com/demo", "--mode", "new"); r.code != 0 {
		t.Fatalf("explicit new: %+v", r)
	}
}

func TestCancellationExit130(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := invoke(t, ctx, "init", root, "--module", "example.com/demo", "--format", "json")
	rep := decode(t, r)
	if r.code != 130 || rep.Status != "cancelled" {
		t.Fatalf("exit %d report %+v", r.code, rep)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatal("wrote files")
	}
}

func TestOperationalFailureExit1(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission test needs a non-root POSIX user")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
	r := invoke(t, t.Context(), "init", filepath.Join(parent, "proj"), "--module", "example.com/demo", "--format", "json")
	rep := decode(t, r)
	if r.code != 1 || rep.Status != "error" || len(rep.Diagnostics) == 0 {
		t.Fatalf("exit %d report %+v", r.code, rep)
	}
}

func TestExistingProjectWithUnknownLibraries(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	main := "package main\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc main() { _ = gin.Default().Run() }\n"
	put(t, root, "main.go", main)
	r := invoke(t, t.Context(), "init", root, "--format", "json")
	rep := decode(t, r)
	if r.code != 0 || rep.Mode != "existing" || rep.Config.Features.HTTP != "gin" || rep.Config.Project.Go != "1.22" {
		t.Fatalf("exit %d report %+v", r.code, rep)
	}
	if !slices.ContainsFunc(rep.Diagnostics, func(d struct{ Severity, Message string }) bool {
		return strings.Contains(d.Message, "1.22")
	}) {
		t.Fatalf("older Go version not reported: %+v", rep.Diagnostics)
	}
	data, _ := os.ReadFile(filepath.Join(root, "main.go"))
	if string(data) != main {
		t.Fatal("application code rewritten")
	}
	mod, _ := os.ReadFile(filepath.Join(root, "go.mod"))
	if string(mod) != "module example.com/app\n\ngo 1.22\n" {
		t.Fatal("go.mod rewritten")
	}
}

func TestStructuredEntryPointAndCommandFlags(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	put(t, root, "tools/my tool/main.go", "package main\n\nfunc main() {}\n")
	r := invoke(t, t.Context(), "init", root, "--format", "json", "--dry-run",
		"--entry-point", `{"name":"tool","dir":"tools/my tool"}`,
		"--command", `{"name":"test","argv":["go","test","./...","-run","$(Test)"],"env":["DATABASE_URL"]}`)
	rep := decode(t, r)
	if r.code != 0 {
		t.Fatalf("exit %d %+v", r.code, rep)
	}
	var test []string
	for _, c := range rep.Config.Commands {
		if c.Name == "test" {
			test = c.Argv
		}
	}
	if !slices.Equal(test, []string{"go", "test", "./...", "-run", "$(Test)"}) {
		t.Fatalf("command argv = %v", test)
	}
	if len(rep.Config.EntryPoints) != 1 || rep.Config.EntryPoints[0].Dir != "tools/my tool" {
		t.Fatalf("entry points = %+v", rep.Config.EntryPoints)
	}
	r = invoke(t, t.Context(), "init", root, "--format", "json", "--dry-run", "--clear-commands", "--clear-entry-points")
	rep = decode(t, r)
	if r.code != 0 || len(rep.Config.Commands) != 0 || len(rep.Next) != 0 {
		t.Fatalf("clear flags: exit %d %+v", r.code, rep.Config.Commands)
	}
}

func TestTextReportAndRerun(t *testing.T) {
	root := t.TempDir()
	r := invoke(t, t.Context(), "init", root, "--module", "example.com/demo", "--starter", "runnable")
	if r.code != 0 {
		t.Fatalf("first: %+v", r)
	}
	for _, want := range []string{"create", "main.go", "go run .", "go mod tidy"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("text report missing %q:\n%s", want, r.out)
		}
	}
	put(t, root, "main.go", "package main\n\nfunc main() { println(1) }\n")
	if r := invoke(t, t.Context(), "init", root); r.code != 0 {
		t.Fatalf("second: %+v", r)
	}
	r = invoke(t, t.Context(), "init", root, "--format", "json")
	rep := decode(t, r)
	for _, a := range rep.Actions {
		if a.State != "unchanged" {
			t.Fatalf("third run not a no-op: %+v", rep.Actions)
		}
	}
	data, _ := os.ReadFile(filepath.Join(root, "main.go"))
	if !strings.Contains(string(data), "println(1)") {
		t.Fatal("rerun rewrote main.go")
	}
}

var (
	buildOnce sync.Once
	binary    string
	buildErr  error
)

func rubricBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "rubric-bin")
		if err != nil {
			buildErr = err
			return
		}
		binary = filepath.Join(dir, "rubric")
		cmd := exec.Command("go", "build", "-o", binary, "../../cmd/rubric")
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = errors.New(string(out))
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return binary
}

func TestProcessHelpUsageAndStreams(t *testing.T) {
	bin := rubricBinary(t)
	out, err := exec.CommandContext(t.Context(), bin, "init", "--help").Output()
	if err != nil || !strings.Contains(string(out), "--non-interactive") {
		t.Fatalf("help: %v %s", err, out)
	}
	cmd := exec.CommandContext(t.Context(), bin, "init", "--nope")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(stderr.String(), "nope") {
		t.Fatalf("unknown flag: %v %q", err, stderr.String())
	}
	root := filepath.Join(t.TempDir(), "p")
	cmd = exec.CommandContext(t.Context(), bin, "init", root, "--module", "example.com/demo", "--format", "json")
	var stdout bytes.Buffer
	stderr.Reset()
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("json run: %v %s", err, stderr.String())
	}
	if !json.Valid(stdout.Bytes()) || stderr.Len() != 0 {
		t.Fatalf("stream separation: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "rubric.yaml")); err != nil {
		t.Fatal(err)
	}
}
