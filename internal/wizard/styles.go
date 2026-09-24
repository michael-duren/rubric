package wizard

import (
	"charm.land/lipgloss/v2"

	"github.com/michael-duren/go-skills/internal/plan"
)

var (
	accent = lipgloss.Color("#7D56F4")
	green  = lipgloss.Color("#04B575")
	yellow = lipgloss.Color("#E5C07B")
	red    = lipgloss.Color("#FF5F87")
	muted  = lipgloss.Color("#8A8A8A")
	white  = lipgloss.Color("#FAFAFA")

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(white).Background(accent).Padding(0, 1)
	stepNow       = lipgloss.NewStyle().Bold(true).Foreground(accent)
	stepDone      = lipgloss.NewStyle().Foreground(green)
	stepLater     = lipgloss.NewStyle().Foreground(muted)
	subtleStyle   = lipgloss.NewStyle().Foreground(muted)
	cursorStyle   = lipgloss.NewStyle().Bold(true).Foreground(accent)
	labelStyle    = lipgloss.NewStyle().Bold(true)
	valueStyle    = lipgloss.NewStyle().Foreground(accent)
	onStyle       = lipgloss.NewStyle().Bold(true).Foreground(green)
	errorStyle    = lipgloss.NewStyle().Bold(true).Foreground(red)
	successStyle  = lipgloss.NewStyle().Bold(true).Foreground(green)
	addedStyle    = lipgloss.NewStyle().Foreground(green)
	removedStyle  = lipgloss.NewStyle().Foreground(red)
	decisionStyle = lipgloss.NewStyle().Foreground(yellow)

	stateStyles = map[string]lipgloss.Style{
		plan.StateCreate:    lipgloss.NewStyle().Foreground(green),
		plan.StateUpdate:    lipgloss.NewStyle().Foreground(yellow),
		plan.StateUnchanged: lipgloss.NewStyle().Foreground(muted),
		plan.StateConflict:  lipgloss.NewStyle().Bold(true).Foreground(red),
		plan.StateSkip:      lipgloss.NewStyle().Foreground(muted),
	}
)
