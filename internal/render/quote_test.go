package render_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/michael-duren/go-skills/internal/render"
)

var literals = []string{
	"a path with 'quotes' $HOME `printf wrong`",
	"",
	"$(id)",
	`back\slash "double"`,
	"tab\tand*glob?",
	"'",
}

func TestShellQuotePreservesLiteralText(t *testing.T) {
	for _, value := range literals {
		command := exec.Command("sh", "-c", "printf '%s' "+render.QuotePOSIX(value))
		output, err := command.Output()
		if err != nil {
			t.Fatal(err)
		}
		if string(output) != value {
			t.Fatalf("got %q, want %q", output, value)
		}
	}
}

func TestShellQuoteMakeRecipe(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make is not installed")
	}
	for _, value := range literals {
		dir := t.TempDir()
		recipe := "show:\n\t@printf '%s' " + render.QuoteMake(render.QuotePOSIX(value)) + "\n"
		if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(recipe), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("make", "-s", "show")
		cmd.Dir = dir
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("%q: %v", value, err)
		}
		if string(output) != value {
			t.Fatalf("got %q, want %q", output, value)
		}
	}
}
