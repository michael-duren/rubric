package wizard

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/michael-duren/go-skills/internal/initialize"
)

func (m Model) plainView() string {
	return ansi.Strip(m.View().Content)
}

func lineWith(t *testing.T, view, text string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(ansi.Strip(line), text) {
			return line
		}
	}
	t.Fatalf("no line contains %q:\n%s", text, ansi.Strip(view))
	return ""
}

func TestViewIsStyled(t *testing.T) {
	m := start(t, initialize.Request{Target: t.TempDir(), Mode: "auto"}, realBackend())
	view := m.View().Content
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("wizard view has no styling")
	}
	for _, want := range []string{"rubric init", "Target", "Application", "Tooling", "Review", "Apply", "Module path"} {
		if !strings.Contains(m.plainView(), want) {
			t.Errorf("plain view missing %q:\n%s", want, m.plainView())
		}
	}
	focused := lineWith(t, view, "Module path")
	other := lineWith(t, view, "Description")
	if ansi.Strip(focused) == focused || focused[:strings.Index(focused, "M")] == other[:strings.Index(other, "D")] {
		t.Fatalf("focused row not highlighted differently:\n%q\n%q", focused, other)
	}
}

func TestStyledLinesFitWidth(t *testing.T) {
	m := manyActions(t)
	for _, width := range []int{20, 40, 80} {
		m = drive(t, m, tea.WindowSizeMsg{Width: width, Height: 20})
		for _, line := range strings.Split(m.View().Content, "\n") {
			if w := ansi.StringWidth(line); w > max(width, minWidth) {
				t.Fatalf("width %d: line is %d cells: %q", width, w, ansi.Strip(line))
			}
		}
	}
}

func TestReviewStatesAreColoredDifferently(t *testing.T) {
	m := manyActions(t)
	view := m.View().Content
	create := lineWith(t, view, "create")
	unchanged := lineWith(t, view, "Tooling:")
	if !strings.Contains(create, "\x1b[") || ansi.Strip(create) == create || create == unchanged {
		t.Fatalf("review state not colored: %q", create)
	}
}
