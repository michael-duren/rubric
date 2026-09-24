package render_test

import (
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

func TestNormalizeRespectsExplicitCommandLists(t *testing.T) {
	c := testproject.Config()
	c.Project.Starter = "runnable"
	derived, err := render.Normalize(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	if len(derived.Commands) == 0 {
		t.Fatal("absent commands not derived")
	}
	c.Commands = []config.Command{}
	explicit, err := render.Normalize(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	if len(explicit.Commands) != 0 {
		t.Fatalf("explicit empty commands repopulated: %+v", explicit.Commands)
	}
}
