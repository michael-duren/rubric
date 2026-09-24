package render_test

import (
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

func buildAndTest(t *testing.T, f config.Features, wantTests ...string) {
	t.Helper()
	c := testproject.Config()
	c.Features = f
	files, err := render.Files(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	root := testproject.Write(t, files)
	testproject.Go(t, root, "mod", "tidy")
	testproject.Go(t, root, "build", "./...")
	testproject.Go(t, root, "vet", "./...")
	out := testproject.Go(t, root, "test", "-count=1", "-v", "./...")
	for _, name := range wantTests {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("generated test %s did not pass:\n%s", name, out)
		}
	}
}

func TestGeneratedCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated projects")
	}
	tests := []string{"TestStatus", "TestStatusCheckFailure", "TestCancelledContext", "TestHelp",
		"TestInvalidFlag", "TestUnknownCommand", "TestWriterFailure", "TestConcurrentRuns"}
	for _, f := range []config.Features{
		{HTTP: "none", Database: "none", Access: "none", CLI: "flag", TUI: "none", Config: "stdlib"},
		{HTTP: "none", Database: "none", Access: "none", CLI: "cobra", TUI: "none", Config: "stdlib"},
		{HTTP: "nethttp", Database: "none", Access: "none", CLI: "flag", TUI: "none", Config: "stdlib"},
		{HTTP: "chi", Database: "none", Access: "none", CLI: "cobra", TUI: "none", Config: "stdlib"},
	} {
		t.Run(f.HTTP+"-"+f.CLI, func(t *testing.T) {
			want := tests
			if f.HTTP != "none" {
				want = append(append([]string{}, tests...), "TestHealth")
			}
			buildAndTest(t, f, want...)
		})
	}
}

func TestGeneratedCLIFiles(t *testing.T) {
	for _, impl := range []string{"flag", "cobra"} {
		c := testproject.Config()
		c.Features.CLI = impl
		files, err := render.Files(c, "new")
		if err != nil {
			t.Fatal(err)
		}
		command := string(testproject.File(t, files, "internal/cli/command.go"))
		if (impl == "cobra") != strings.Contains(command, "github.com/spf13/cobra") {
			t.Fatalf("%s: wrong implementation:\n%s", impl, command)
		}
		testproject.File(t, files, "internal/cli/command_test.go")
		main := string(testproject.File(t, files, "cmd/cli/main.go"))
		if !strings.Contains(main, `"example.com/demo/internal/cli"`) {
			t.Fatalf("main does not import the tested package:\n%s", main)
		}
		for _, f := range files {
			if strings.HasPrefix(f.Path, "internal/") && strings.Contains(string(f.Data), "/cmd/") {
				t.Fatalf("%s imports a main package", f.Path)
			}
		}
	}
}
