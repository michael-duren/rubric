package wizard

import (
	"fmt"
	"path"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/mod/module"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/initialize"
)

type fieldKind int

const (
	textField fieldKind = iota
	choiceField
	toggleField
)

type field struct {
	stage   stage
	key     string
	label   string
	help    string
	kind    fieldKind
	options []string
	value   string
	dirty   bool
}

func (f field) patchValue() any {
	switch {
	case f.kind == toggleField:
		return f.value == "true"
	case f.key == "entry_points":
		return parseEntryPoints(f.value)
	case f.key == "commands":
		return parseCommands(f.value)
	}
	return f.value
}

func buildFields(mode string, c config.Config, req initialize.Request) []field {
	var fields []field
	if mode == "existing" {
		fields = append(fields,
			field{stage: targetStage, key: "project.description", label: "Description", kind: textField, value: c.Project.Description},
			field{stage: targetStage, key: "entry_points", label: "Entry points (comma-separated dirs)", kind: textField, value: formatEntryPoints(c.EntryPoints)},
			field{stage: targetStage, key: "commands", label: "Commands (name=args; ...)", kind: textField, value: formatCommands(c.Commands)},
		)
	} else {
		fields = append(fields,
			field{stage: targetStage, key: "project.module", label: "Module path", kind: textField, value: c.Project.Module},
			field{stage: targetStage, key: "project.name", label: "Display name (optional)", kind: textField, value: c.Project.Name},
			field{stage: targetStage, key: "project.description", label: "Description (optional)", kind: textField, value: c.Project.Description},
		)
	}
	fields = append(fields, field{stage: targetStage, key: "mode", label: "Mode", kind: choiceField, options: []string{"auto", "new", "existing"}, value: req.Mode})
	fields = append(fields, applicationFields(mode, c)...)
	fields = append(fields, toolingFields(c.Tooling)...)
	for i := range fields {
		f := &fields[i]
		value, ok := req.Overrides[f.key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case string:
			f.value = v
		case bool:
			f.value = fmt.Sprint(v)
		case []config.EntryPoint:
			f.value = formatEntryPoints(v)
		case []config.Command:
			f.value = formatCommands(v)
		}
		if f.kind == choiceField && !slices.Contains(f.options, f.value) {
			f.options = append(f.options, f.value)
		}
		f.dirty = true
	}
	return fields
}

func (m Model) index(key string) int {
	return slices.IndexFunc(m.fields, func(f field) bool { return f.key == key })
}

func (m Model) value(key string) string {
	if i := m.index(key); i >= 0 {
		return m.fields[i].value
	}
	return ""
}

func (m Model) set(key, value string, dirty bool) Model {
	i := m.index(key)
	if i < 0 {
		return m
	}
	m.fields = slices.Clone(m.fields)
	m.fields[i].value = value
	m.fields[i].dirty = m.fields[i].dirty || dirty
	return m
}

func (m Model) visible(key string) bool {
	i := m.index(key)
	return i >= 0 && m.shown(m.fields[i])
}

func (m Model) shown(f field) bool {
	switch f.key {
	case "features.access":
		return m.value("features.database") != "none"
	case "project.starter":
		return m.mode == "new" && !executableSelected(m)
	}
	return true
}

func (m Model) visibleFields() []int {
	var out []int
	for i, f := range m.fields {
		if f.stage == m.stage && m.shown(f) {
			out = append(out, i)
		}
	}
	return out
}

func (m Model) current() *field {
	if m.stage > toolingStage {
		return nil
	}
	visible := m.visibleFields()
	if len(visible) == 0 {
		return nil
	}
	f := m.fields[visible[min(m.cursor[m.stage], len(visible)-1)]]
	return &f
}

func (m Model) formKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	visible := m.visibleFields()
	pos := min(m.cursor[m.stage], max(len(visible)-1, 0))
	var f *field
	if len(visible) > 0 {
		f = &m.fields[visible[pos]]
	}
	switch msg.String() {
	case "enter":
		return m.advance()
	case "esc":
		if m.stage == targetStage {
			return m.cancel()
		}
		m.stage--
		m.message = ""
		return m, nil
	case "down", "tab":
		m.cursor[m.stage] = min(pos+1, max(len(visible)-1, 0))
		return m, nil
	case "up", "shift+tab":
		m.cursor[m.stage] = max(pos-1, 0)
		return m, nil
	}
	if f == nil {
		return m, nil
	}
	switch f.kind {
	case textField:
		switch {
		case msg.String() == "backspace":
			r := []rune(f.value)
			if len(r) > 0 {
				return m.edit(f.key, string(r[:len(r)-1]))
			}
		case msg.Text != "" && msg.Mod == 0 || msg.String() == "space":
			text := msg.Text
			if text == "" {
				text = " "
			}
			return m.edit(f.key, f.value+text)
		}
	case choiceField:
		switch msg.String() {
		case "right", "space", "l":
			return m.choose(*f, 1)
		case "left", "h":
			return m.choose(*f, -1)
		}
	case toggleField:
		if s := msg.String(); s == "space" || s == "x" || s == "right" || s == "left" {
			next := "true"
			if f.value == "true" {
				next = "false"
			}
			return m.set(f.key, next, true), nil
		}
	}
	return m, nil
}

func (m Model) edit(key, value string) (tea.Model, tea.Cmd) {
	m = m.set(key, value, true)
	m.message = ""
	return m, nil
}

func (m Model) advance() (tea.Model, tea.Cmd) {
	switch m.stage {
	case targetStage:
		if err := m.validateTarget(); err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.message = ""
		m.stage = applicationStage
		return m, nil
	case applicationStage:
		m.stage = toolingStage
		return m, nil
	case toolingStage:
		m.stage = reviewStage
		m.planned = false
		m.message = ""
		return m.startPrepare()
	}
	return m, nil
}

func (m Model) validateTarget() error {
	if m.mode != "new" {
		return nil
	}
	mod := m.value("project.module")
	if mod == "" {
		return fmt.Errorf("project.module: enter a module path such as example.com/app")
	}
	if err := module.CheckImportPath(mod); err != nil {
		return fmt.Errorf("project.module: %v", err)
	}
	return nil
}

func formatEntryPoints(eps []config.EntryPoint) string {
	dirs := make([]string, len(eps))
	for i, ep := range eps {
		dirs[i] = ep.Dir
	}
	return strings.Join(dirs, ", ")
}

func parseEntryPoints(text string) []config.EntryPoint {
	out := []config.EntryPoint{}
	for _, dir := range strings.Split(text, ",") {
		if dir = strings.TrimSpace(dir); dir != "" {
			name := path.Base(dir)
			out = append(out, config.EntryPoint{Name: name, Dir: dir})
		}
	}
	return out
}

func formatCommands(cmds []config.Command) string {
	parts := make([]string, len(cmds))
	for i, c := range cmds {
		parts[i] = c.Name + "=" + strings.Join(c.Argv, " ")
	}
	return strings.Join(parts, "; ")
}

func parseCommands(text string) []config.Command {
	out := []config.Command{}
	for _, part := range strings.Split(text, ";") {
		name, args, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		out = append(out, config.Command{Name: strings.TrimSpace(name), Dir: ".", Argv: strings.Fields(args), Env: []string{}})
	}
	return out
}
