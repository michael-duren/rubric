// Package wizard is the Bubble Tea front end that collects answers and drives the shared initialization service.
package wizard

import (
	"context"
	"errors"
	"io"
	"maps"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/plan"
	"github.com/michael-duren/go-skills/internal/write"
)

type stage int

const (
	targetStage stage = iota
	applicationStage
	toolingStage
	reviewStage
	applyStage
	doneStage
)

var stageNames = []string{"Target", "Application", "Tooling", "Review", "Apply", "Done"}

// Backend prepares and applies plans; the wizard never renders or writes files itself.
type Backend struct {
	Prepare func(context.Context, initialize.Request) (plan.Plan, error)
	Apply   func(context.Context, initialize.Request, plan.Plan) (write.Result, error)
}

// Outcome is the final request, reviewed plan, write result, and whether the user cancelled.
type Outcome struct {
	Request   initialize.Request
	Plan      plan.Plan
	Result    write.Result
	Cancelled bool
}

type planMsg struct {
	id       int
	baseline bool
	plan     plan.Plan
	err      error
}

type appliedMsg struct {
	result write.Result
	err    error
}

// Model is the wizard's complete state; Update returns a new Model for every message.
type Model struct {
	ctx          context.Context
	base         initialize.Request
	backend      Backend
	stage        stage
	width        int
	height       int
	mode         string
	fields       []field
	cursor       [3]int
	decisions    map[string]string
	lastDecision string
	plan         plan.Plan
	detected     config.Config
	planned      bool
	actionCursor int
	preview      bool
	reqID        int
	preparing    bool
	cancelApply  context.CancelFunc
	message      string
	applyErr     error
	outcome      Outcome
}

// New returns a wizard for req whose preparation and writes go through backend.
func New(ctx context.Context, req initialize.Request, backend Backend) Model {
	m := Model{ctx: ctx, base: req, backend: backend, mode: "new", decisions: map[string]string{}, reqID: 1}
	if m.base.Mode == "" {
		m.base.Mode = "auto"
	}
	m.fields = buildFields("new", config.Defaults(), req)
	return m
}

// Init runs a baseline preparation to detect the mode and prefill existing-project facts.
func (m Model) Init() tea.Cmd {
	return m.prepareCmd(m.reqID, true, m.base)
}

func (m Model) prepareCmd(id int, baseline bool, req initialize.Request) tea.Cmd {
	ctx, prepare := m.ctx, m.backend.Prepare
	return func() tea.Msg {
		p, err := prepare(ctx, req)
		return planMsg{id: id, baseline: baseline, plan: p, err: err}
	}
}

// Update applies msg and returns the next model and command.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case planMsg:
		return m.onPlan(msg)
	case appliedMsg:
		return m.onApplied(msg)
	case tea.KeyPressMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m Model) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m.cancel()
	}
	switch m.stage {
	case targetStage, applicationStage, toolingStage:
		return m.formKey(msg)
	case reviewStage:
		return m.reviewKey(msg)
	case doneStage:
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) cancel() (tea.Model, tea.Cmd) {
	switch m.stage {
	case applyStage:
		if m.cancelApply != nil {
			m.cancelApply()
		}
		m.message = "Cancelling; restoring files written so far..."
		return m, nil
	case doneStage:
		return m, tea.Quit
	}
	m.outcome.Cancelled = true
	m.outcome.Request = m.request()
	return m, tea.Quit
}

func (m Model) onPlan(msg planMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.reqID {
		return m, nil
	}
	m.preparing = false
	if msg.baseline {
		return m.onBaseline(msg), nil
	}
	if msg.err != nil {
		m.message = msg.err.Error()
		if m.lastDecision != "" {
			delete(m.decisions, m.lastDecision)
		}
		m.lastDecision = ""
		return m, nil
	}
	m.lastDecision = ""
	m.message = ""
	m.plan, m.planned = msg.plan, true
	m.actionCursor = min(m.actionCursor, max(len(m.plan.Actions)-1, 0))
	return m, nil
}

func (m Model) onBaseline(msg planMsg) Model {
	mode := "new"
	cfg := config.Defaults()
	switch {
	case msg.err == nil:
		mode, cfg = msg.plan.Mode, msg.plan.Config
		m.message = ""
	case strings.Contains(msg.err.Error(), "project.module: required"):
		m.message = ""
	default:
		m.message = msg.err.Error()
	}
	requested := m.value("mode")
	m.mode, m.detected = mode, cfg
	m.fields = buildFields(mode, cfg, m.base)
	if requested != "" {
		m = m.set("mode", requested, requested != m.base.Mode)
	}
	return m
}

func (m Model) onApplied(msg appliedMsg) (tea.Model, tea.Cmd) {
	m.stage = doneStage
	m.cancelApply = nil
	m.applyErr = msg.err
	m.outcome.Result = msg.result
	m.outcome.Plan = m.plan
	m.outcome.Request = m.request()
	m.outcome.Cancelled = errors.Is(msg.err, context.Canceled)
	m.message = ""
	return m, tea.Quit
}

func (m Model) request() initialize.Request {
	req := m.base
	req.Overrides = maps.Clone(m.base.Overrides)
	if req.Overrides == nil {
		req.Overrides = config.Patch{}
	}
	for _, f := range m.fields {
		if f.key == "mode" {
			req.Mode = f.value
			continue
		}
		if !f.dirty {
			continue
		}
		req.Overrides[f.key] = f.patchValue()
	}
	if m.value("features.database") == "none" {
		delete(req.Overrides, "features.access")
	}
	req.Decisions = maps.Clone(m.decisions)
	return req
}

func (m Model) startPrepare() (Model, tea.Cmd) {
	m.reqID++
	m.preparing = true
	return m, m.prepareCmd(m.reqID, false, m.request())
}

// Run shows the wizard on in and out until it finishes, returning the outcome and any apply error.
func Run(ctx context.Context, req initialize.Request, in io.Reader, out io.Writer) (Outcome, error) {
	m := New(ctx, req, Backend{Prepare: initialize.Prepare, Apply: initialize.Apply})
	final, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	if fm, ok := final.(Model); ok {
		m = fm
	}
	if errors.Is(err, tea.ErrProgramKilled) || errors.Is(err, tea.ErrInterrupted) || ctx.Err() != nil {
		m.outcome.Cancelled = true
		return m.outcome, context.Canceled
	}
	if err != nil {
		return m.outcome, err
	}
	if m.outcome.Cancelled && m.applyErr == nil {
		return m.outcome, context.Canceled
	}
	return m.outcome, m.applyErr
}

func toolingFor(path string) string {
	switch {
	case path == "Makefile":
		return "tooling.makefile"
	case path == ".golangci.yml" || strings.HasPrefix(path, ".rubric/style/"):
		return "tooling.lint"
	case path == ".github/workflows/ci.yml":
		return "tooling.actions"
	case strings.HasPrefix(path, ".agents/skills/"):
		return "tooling.skills"
	}
	return ""
}
