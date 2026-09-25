package wizard

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/michael-duren/go-skills/internal/features"
)

func (m Model) menuKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "s" {
		if !m.loaded {
			return m, nil
		}
		m.stage, m.planned, m.message = reviewStage, false, ""
		return m.startPrepare()
	}
	if m.open {
		switch key {
		case "esc", "left", "h", "enter":
			m.open = false
			return m, nil
		}
		return m.formKey(msg)
	}
	switch key {
	case "up", "k", "shift+tab":
		m.menu = max(m.menu-1, 0)
	case "down", "j", "tab":
		m.menu = min(m.menu+1, len(features.Menus)-1)
	case "enter", "right", "l", "space":
		if m.loaded {
			m.open, m.cursor[toolingStage] = true, 0
		}
	case "esc", "q":
		if m.edited() {
			return m.cancel()
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) edited() bool {
	for _, f := range features.All {
		if m.changed(f.Key) {
			return true
		}
	}
	return false
}

func (m Model) changed(key string) bool {
	return m.loaded && fmt.Sprint(features.Enabled(m.detected.Tooling, key)) != m.value(key)
}

func (m Model) menuLines() []string {
	if !m.loaded {
		if m.message != "" {
			return []string{"Fix rubric.yaml, then run rubric update again."}
		}
		return []string{stepNow.Render("Reading rubric.yaml...")}
	}
	if m.open {
		menu := features.Menus[m.menu]
		return append([]string{labelStyle.Render(menu), ""}, m.formLines()...)
	}
	lines := []string{subtleStyle.Render("Active features from rubric.yaml. Open a menu to toggle them, then press s to save."), ""}
	for i, menu := range features.Menus {
		var on []string
		for _, f := range features.In(menu) {
			if m.value(f.Key) == "true" {
				on = append(on, f.Label)
			}
		}
		summary := strings.Join(on, ", ")
		if summary == "" {
			summary = "none"
		}
		marker, label := "  ", fmt.Sprintf("%-8s ›", menu)
		if i == m.menu {
			marker, label = cursorStyle.Render("> "), labelStyle.Render(label)
		}
		count := subtleStyle.Render(fmt.Sprintf("(%d of %d on)", len(on), len(features.In(menu))))
		lines = append(lines, fmt.Sprintf("%s%s %s %s", marker, label, valueStyle.Render(summary), count))
	}
	if m.edited() {
		lines = append(lines, "", decisionStyle.Render("Unsaved changes: press s to review them."))
	}
	return lines
}
