package render_test

import (
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

func TestGeneratedTUI(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated projects")
	}
	tuiTests := []string{"TestQuit", "TestInitialContent", "TestWindowResize", "TestReadiness",
		"TestUnrelatedMessage", "TestCancelledContext"}
	t.Run("tui only", func(t *testing.T) {
		buildAndTest(t, config.Features{HTTP: "none", Database: "none", Access: "none", CLI: "none", TUI: "bubbletea", Config: "stdlib"}, tuiTests...)
	})
	t.Run("http cli tui", func(t *testing.T) {
		want := append([]string{"TestHealth", "TestStatus"}, tuiTests...)
		buildAndTest(t, config.Features{HTTP: "chi", Database: "none", Access: "none", CLI: "cobra", TUI: "bubbletea", Config: "stdlib"}, want...)
	})
}

func TestGeneratedTUIFiles(t *testing.T) {
	c := testproject.Config()
	c.Features.TUI = "bubbletea"
	files, err := render.Files(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"internal/tui/model.go", "internal/tui/model_test.go", "cmd/tui/main.go"} {
		data := string(testproject.File(t, files, path))
		if strings.Contains(data, "github.com/charmbracelet/bubbletea") || !strings.Contains(data, "charm.land/bubbletea/v2") {
			t.Fatalf("%s must import charm.land/bubbletea/v2 only", path)
		}
	}
	if !strings.Contains(string(testproject.File(t, files, "cmd/tui/main.go")), "tea.WithContext(ctx)") {
		t.Fatal("main does not bind the program to its context")
	}
}
