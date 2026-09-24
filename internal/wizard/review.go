package wizard

import (
	"maps"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/michael-duren/go-skills/internal/plan"
)

const maxDiffLines = 400

func (m Model) reviewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.preview {
		switch msg.String() {
		case "p", "esc":
			m.preview = false
		case "up", "k":
			m.previewTop = max(m.previewTop-1, 0)
		case "down", "j":
			m.previewTop = min(m.previewTop+1, max(len(m.previewLines())-1, 0))
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.stage = toolingStage
		m.planned = false
		m.preview = false
		m.message = ""
		m.reqID++
		return m, nil
	case "up", "k":
		m.actionCursor = max(m.actionCursor-1, 0)
		return m, nil
	case "down", "j":
		m.actionCursor = min(m.actionCursor+1, max(len(m.plan.Actions)-1, 0))
		return m, nil
	case "p":
		m.preview = m.planned && len(m.plan.Actions) > 0
		m.previewTop = 0
		return m, nil
	}
	if m.preparing || !m.planned {
		return m, nil
	}
	switch msg.String() {
	case "enter":
		if n := len(plan.Conflicts(m.plan)); n > 0 {
			m.message = "resolve each conflict first: select it, then press r to replace or s to skip"
			return m, nil
		}
		return m.startApply()
	case "r", "s":
		return m.decide(msg.String())
	}
	return m, nil
}

func (m Model) decide(choice string) (tea.Model, tea.Cmd) {
	if m.actionCursor >= len(m.plan.Actions) {
		return m, nil
	}
	a := m.plan.Actions[m.actionCursor]
	if a.State != plan.StateConflict {
		m.message = a.File.Path + " is not in conflict"
		return m, nil
	}
	m.decisions = maps.Clone(m.decisions)
	if keys := toolingFor(a.File.Path); choice == "s" && len(keys) > 0 {
		for _, key := range keys {
			m = m.set(key, "false", true)
		}
		for path := range m.decisions {
			if len(toolingFor(path)) > 0 {
				delete(m.decisions, path)
			}
		}
		m.lastDecision = ""
	} else {
		m.decisions[a.File.Path] = map[string]string{"r": plan.DecisionReplace, "s": plan.DecisionSkip}[choice]
		m.lastDecision = a.File.Path
	}
	m.message = ""
	return m.startPrepare()
}

func (m Model) previewLines() []string {
	if !m.preview || m.actionCursor >= len(m.plan.Actions) {
		return nil
	}
	a := m.plan.Actions[m.actionCursor]
	var lines []string
	if !a.Before.Exists {
		for _, l := range splitLines(string(a.File.Data)) {
			lines = append(lines, "+"+l)
		}
		return lines
	}
	return append(lines, diff(splitLines(string(a.Before.Data)), splitLines(string(a.File.Data)))...)
}

func splitLines(s string) []string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) > maxDiffLines {
		lines = append(lines[:maxDiffLines], "... (truncated)")
	}
	return lines
}

func diff(a, b []string) []string {
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []string
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			out = append(out, " "+a[i])
			i, j = i+1, j+1
		case j < len(b) && (i == len(a) || lcs[i][j+1] >= lcs[i+1][j]):
			out = append(out, "+"+b[j])
			j++
		default:
			out = append(out, "-"+a[i])
			i++
		}
	}
	return out
}
