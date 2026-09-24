package wizard

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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
	header := []string{
		titleStyle.Render("rubric init") + " " + m.steps(),
		subtleStyle.Render(fmt.Sprintf("%s · %s project", target(m.base.Target), m.mode)),
		"",
	}
	var body []string
	focus := -1
	switch m.stage {
	case targetStage, applicationStage, toolingStage:
		body = m.formLines()
	case reviewStage:
		body, focus = m.reviewLines()
	case applyStage:
		body = []string{stepNow.Render("Applying the reviewed plan...") + subtleStyle.Render(" press ctrl+c to cancel and restore.")}
	case doneStage:
		body = m.doneLines()
	}
	var footer []string
	if m.message != "" {
		footer = append(footer, "", errorStyle.Render("✗ "+m.message))
	}
	footer = append(footer, "", subtleStyle.Render(m.help()))
	header, body, footer = clip(header, width), clip(body, width), clip(footer, width)
	available := max(height-len(header)-len(footer), 1)
	if len(body) > available {
		start := 0
		switch {
		case m.stage == reviewStage && m.preview:
			start = min(m.previewTop, len(body)-available)
		case focus >= 0:
			start = min(max(focus-available/2, 0), len(body)-available)
		}
		body = body[start : start+available]
	}
	return strings.Join(slices.Concat(header, body, footer), "\n")
}

func (m Model) steps() string {
	names := stageNames[:applyStage+1]
	parts := make([]string, len(names))
	for i, name := range names {
		switch {
		case stage(i) == m.stage || m.stage == doneStage && stage(i) == applyStage:
			parts[i] = stepNow.Render(name)
		case stage(i) < m.stage:
			parts[i] = stepDone.Render("✓ " + name)
		default:
			parts[i] = stepLater.Render(name)
		}
	}
	return strings.Join(parts, subtleStyle.Render(" › "))
}

func clip(lines []string, width int) []string {
	var out []string
	for _, line := range lines {
		for _, part := range strings.Split(line, "\n") {
			out = append(out, ansi.Truncate(part, width, ""))
		}
	}
	return out
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
		lines = append(lines, subtleStyle.Render(fmt.Sprintf("Module %s (Go %s), detected from go.mod", m.detected.Project.Module, m.detected.Project.Go)), "")
	}
	cur := m.current()
	pad := 0
	for _, i := range m.visibleFields() {
		if m.fields[i].kind != toggleField {
			pad = max(pad, len([]rune(m.fields[i].label)))
		}
	}
	for _, i := range m.visibleFields() {
		f := m.fields[i]
		focused := cur != nil && cur.key == f.key
		text := f.label
		if f.kind != toggleField {
			text += ":" + strings.Repeat(" ", pad-len([]rune(f.label)))
		}
		marker, label := "  ", text
		if focused {
			marker, label = cursorStyle.Render("> "), labelStyle.Render(text)
		}
		switch f.kind {
		case textField:
			value := f.value
			if focused {
				value = valueStyle.Render(f.value) + cursorStyle.Render("▏")
			}
			lines = append(lines, fmt.Sprintf("%s%s %s", marker, label, value))
		case choiceField:
			var value string
			if focused {
				value = valueStyle.Render("‹ " + f.value + " ›")
			} else {
				value = subtleStyle.Render("‹ ") + f.value + subtleStyle.Render(" ›")
			}
			lines = append(lines, fmt.Sprintf("%s%s %s", marker, label, value))
		case toggleField:
			box := subtleStyle.Render("[ ]")
			if f.value == "true" {
				box = onStyle.Render("[x]")
			}
			lines = append(lines, fmt.Sprintf("%s%s %s %s", marker, box, label, subtleStyle.Render("- "+f.help)))
		}
	}
	return lines
}

func stateLabel(state string) string {
	style, ok := stateStyles[state]
	text := fmt.Sprintf("%-9s", state)
	if !ok {
		return text
	}
	return style.Render(text)
}

func (m Model) reviewLines() ([]string, int) {
	if m.preparing || !m.planned {
		return []string{stepNow.Render("Preparing the plan...")}, -1
	}
	if m.preview {
		a := m.plan.Actions[m.actionCursor]
		lines := []string{labelStyle.Render("Preview of "+a.File.Path) + " " + stateLabel(a.State) + subtleStyle.Render(a.Reason)}
		for _, l := range m.previewLines() {
			switch {
			case strings.HasPrefix(l, "+"):
				l = addedStyle.Render(l)
			case strings.HasPrefix(l, "-"):
				l = removedStyle.Render(l)
			default:
				l = subtleStyle.Render(l)
			}
			lines = append(lines, l)
		}
		return lines, -1
	}
	c := m.plan.Config
	lines := []string{
		subtleStyle.Render(fmt.Sprintf("Module %s, Go %s; HTTP %s, database %s/%s, CLI %s, TUI %s, config %s, web %s, e2e %s",
			c.Project.Module, c.Project.Go, c.Features.HTTP, c.Features.Database, c.Features.Access, c.Features.CLI, c.Features.TUI,
			c.Features.Config, orNone(c.Features.Web), orNone(c.Features.E2E))),
		subtleStyle.Render(fmt.Sprintf("Tooling: skills=%t lint=%t makefile=%t actions=%t", c.Tooling.Skills, c.Tooling.Lint, c.Tooling.Makefile, c.Tooling.Actions)),
		"",
	}
	focus := -1
	for i, a := range m.plan.Actions {
		marker, path := "  ", a.File.Path
		if i == m.actionCursor {
			marker, path = cursorStyle.Render("> "), labelStyle.Render(a.File.Path)
			focus = len(lines)
		}
		line := marker + stateLabel(a.State) + " " + path
		if d, ok := m.decisions[a.File.Path]; ok {
			line += " " + decisionStyle.Render("("+d+")")
		}
		if a.State == plan.StateConflict {
			line += " " + errorStyle.Render("- "+a.Reason)
		}
		lines = append(lines, line)
	}
	for _, rel := range m.plan.Obsolete {
		lines = append(lines, "  "+decisionStyle.Render("obsolete ")+" "+rel+subtleStyle.Render(" (kept; review and delete it yourself)"))
	}
	return lines, focus
}

func (m Model) doneLines() []string {
	if m.outcome.Cancelled {
		lines := []string{errorStyle.Render("Cancelled.")}
		return append(lines, m.recoveryLines()...)
	}
	if m.applyErr != nil {
		lines := []string{errorStyle.Render("Apply failed: " + m.applyErr.Error())}
		return append(lines, m.recoveryLines()...)
	}
	lines := []string{successStyle.Render(fmt.Sprintf("✓ Wrote %d file(s):", len(m.outcome.Result.Applied)))}
	for _, p := range m.outcome.Result.Applied {
		lines = append(lines, "  "+addedStyle.Render(p))
	}
	lines = append(lines, "", subtleStyle.Render("Rubric did not download dependencies or run your project's tests."))
	if cmds := m.plan.Config.Commands; len(cmds) > 0 {
		lines = append(lines, labelStyle.Render("Next commands:"))
		for _, c := range cmds {
			lines = append(lines, fmt.Sprintf("  %s %s", valueStyle.Render(c.Name+":"), strings.Join(c.Argv, " ")))
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
		lines = append(lines, errorStyle.Render("Not restored because they changed after Rubric wrote them; review by hand: "+strings.Join(u, ", ")))
	}
	return lines
}

func (m Model) help() string {
	switch m.stage {
	case reviewStage:
		if m.preview {
			return "up/down scroll - p close - ctrl+c cancel"
		}
		return "enter apply - up/down select - p preview - r replace - s skip - esc back - ctrl+c cancel"
	case applyStage:
		return "ctrl+c cancel"
	case doneStage:
		return "press any key to exit"
	}
	return "enter next - esc back - up/down move - left/right change - space toggle - ctrl+c cancel"
}
