package wizard

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

func (m Model) startApply() (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(m.ctx)
	m.stage = applyStage
	m.cancelApply = cancel
	m.message = ""
	apply, req, p := m.backend.Apply, m.request(), m.plan
	return m, func() tea.Msg {
		defer cancel()
		result, err := apply(ctx, req, p)
		return appliedMsg{result: result, err: err}
	}
}
