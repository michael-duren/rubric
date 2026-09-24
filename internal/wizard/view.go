package wizard

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/michael-duren/go-skills/internal/plan"
)

const (
	minWidth      = 20
	minHeight     = 6
	defaultWidth  = 100
	defaultHeight = 60
)

// View renders the current stage clipped to the terminal size.
func (m Model) View() tea.View {
	return tea.NewView(m.pageText())
}

func (m Model) size() (int, int) {
	w, h := m.width, m.height
	if w == 0 && h == 0 {
		return defaultWidth, defaultHeight
	}
	return max(w, minWidth), max(h, minHeight)
}

func (m Model) pageText() string {
	width, height := m.size()
	lines := []string{
		fmt.Sprintf("rubric init - step %d/5: %s", min(int(m.stage), int(applyStage))+1, stageNames[m.stage]),
		fmt.Sprintf("Target: %s (%s project)", target(m.base.Target), m.mode),
		"",
	}
	switch m.stage {
	case targetStage, applicationStage, toolingStage:
		lines = append(lines, m.formLines()...)
	case reviewStage:
		lines = append(lines, m.reviewLines()...)
	case applyStage:
		lines = append(lines, "Applying the reviewed plan... press ctrl+c to cancel and restore.")
	case doneStage:
		lines = append(lines, m.doneLines()...)
	}
	if m.message != "" {
		lines = append(lines, "", "! "+m.message)
	}
	lines = append(lines, "", m.help())
	var out []string
	for _, line := range lines {
		for _, part := range strings.Split(line, "\n") {
			if r := []rune(part); len(r) > width {
				part = string(r[:width])
			}
			out = append(out, part)
		}
	}
	if len(out) > height {
		out = append(out[:height-1], "...")
	}
	return strings.Join(out, "\n")
}

func target(t string) string {
	if t == "" {
		return "."
	}
	return t
}

func (m Model) formLines() []string {
	var lines []string
	if m.stage == targetStage && m.mode == "existing" {
		lines = append(lines, fmt.Sprintf("Module %s (Go %s), detected from go.mod", m.detected.Project.Module, m.detected.Project.Go))
	}
	cur := m.current()
	for _, i := range m.visibleFields() {
		f := m.fields[i]
		marker := "  "
		if cur != nil && cur.key == f.key {
			marker = "> "
		}
		switch f.kind {
		case textField:
			lines = append(lines, fmt.Sprintf("%s%s: %s", marker, f.label, f.value))
		case choiceField:
			lines = append(lines, fmt.Sprintf("%s%s: < %s >", marker, f.label, f.value))
		case toggleField:
			box := "[ ]"
			if f.value == "true" {
				box = "[x]"
			}
			lines = append(lines, fmt.Sprintf("%s%s %s - %s", marker, box, f.label, f.help))
		}
	}
	return lines
}

func (m Model) reviewLines() []string {
	if m.preparing || !m.planned {
		return []string{"Preparing the plan..."}
	}
	c := m.plan.Config
	lines := []string{
		fmt.Sprintf("Module %s, Go %s; HTTP %s, database %s/%s, CLI %s, TUI %s, config %s",
			c.Project.Module, c.Project.Go, c.Features.HTTP, c.Features.Database, c.Features.Access, c.Features.CLI, c.Features.TUI, c.Features.Config),
		fmt.Sprintf("Tooling: skills=%t lint=%t makefile=%t actions=%t", c.Tooling.Skills, c.Tooling.Lint, c.Tooling.Makefile, c.Tooling.Actions),
		"",
	}
	for i, a := range m.plan.Actions {
		marker := "  "
		if i == m.actionCursor {
			marker = "> "
		}
		line := fmt.Sprintf("%s%-9s %s", marker, a.State, a.File.Path)
		if d, ok := m.decisions[a.File.Path]; ok {
			line += " (" + d + ")"
		}
		if a.State == plan.StateConflict {
			line += " - " + a.Reason
		}
		lines = append(lines, line)
	}
	for _, rel := range m.plan.Obsolete {
		lines = append(lines, "  obsolete  "+rel+" (kept; review and delete it yourself)")
	}
	return append(lines, m.previewLines()...)
}

func (m Model) doneLines() []string {
	if m.outcome.Cancelled {
		lines := []string{"Cancelled."}
		return append(lines, m.recoveryLines()...)
	}
	if m.applyErr != nil {
		lines := []string{"Apply failed: " + m.applyErr.Error()}
		return append(lines, m.recoveryLines()...)
	}
	lines := []string{fmt.Sprintf("Wrote %d file(s):", len(m.outcome.Result.Applied))}
	for _, p := range m.outcome.Result.Applied {
		lines = append(lines, "  "+p)
	}
	lines = append(lines, "", "Rubric did not download dependencies or run your project's tests.")
	if cmds := m.plan.Config.Commands; len(cmds) > 0 {
		lines = append(lines, "Next commands:")
		for _, c := range cmds {
			lines = append(lines, fmt.Sprintf("  %s: %s", c.Name, strings.Join(c.Argv, " ")))
		}
	}
	return lines
}

func (m Model) recoveryLines() []string {
	var lines []string
	if r := m.outcome.Result.Restored; len(r) > 0 {
		lines = append(lines, "Restored: "+strings.Join(r, ", "))
	}
	if u := m.outcome.Result.Unrecovered; len(u) > 0 {
		lines = append(lines, "Not restored because they changed after Rubric wrote them; review by hand: "+strings.Join(u, ", "))
	}
	return lines
}

func (m Model) help() string {
	switch m.stage {
	case reviewStage:
		return "enter apply - up/down select - p preview - r replace - s skip - esc back - ctrl+c cancel"
	case applyStage:
		return "ctrl+c cancel"
	case doneStage:
		return "press any key to exit"
	}
	return "enter next - esc back - up/down move - left/right change - space toggle - ctrl+c cancel"
}
