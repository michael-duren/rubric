package wizard

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

type applyTracker struct {
	started chan struct{}
	done    chan appliedMsg
}

func newApplyTracker() *applyTracker {
	return &applyTracker{started: make(chan struct{}), done: make(chan appliedMsg, 1)}
}

func (a *applyTracker) running() bool {
	select {
	case <-a.started:
		return true
	default:
		return false
	}
}

func (m Model) startApply() (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(m.ctx)
	m.stage = applyStage
	m.cancelApply = cancel
	m.message = ""
	apply, req, p, tracker := m.backend.Apply, m.request(), m.plan, m.tracker
	close(tracker.started)
	return m, func() tea.Msg {
		defer cancel()
		result, err := apply(ctx, req, p)
		msg := appliedMsg{result: result, err: err}
		tracker.done <- msg
		return msg
	}
}
