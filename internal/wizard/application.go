package wizard

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/michael-duren/go-skills/internal/config"
)

var applicationChoices = []struct {
	key, label string
	options    []string
	value      func(config.Config) string
}{
	{"project.starter", "Starter without executables", []string{"module", "runnable"}, func(c config.Config) string { return c.Project.Starter }},
	{"features.http", "HTTP server", []string{"none", "nethttp", "chi"}, func(c config.Config) string { return c.Features.HTTP }},
	{"features.database", "Database", []string{"none", "sqlite", "postgres"}, func(c config.Config) string { return c.Features.Database }},
	{"features.access", "Database access", []string{"sql", "sqlc"}, func(c config.Config) string { return c.Features.Access }},
	{"features.cli", "CLI", []string{"none", "flag", "cobra"}, func(c config.Config) string { return c.Features.CLI }},
	{"features.tui", "Terminal UI", []string{"none", "bubbletea"}, func(c config.Config) string { return c.Features.TUI }},
	{"features.config", "Runtime configuration", []string{"stdlib", "viper"}, func(c config.Config) string { return c.Features.Config }},
}

func applicationFields(mode string, c config.Config) []field {
	var out []field
	for _, choice := range applicationChoices {
		if mode != "new" && choice.key == "project.starter" {
			continue
		}
		value := choice.value(c)
		options := slices.Clone(choice.options)
		if choice.key == "features.access" && value == "none" {
			value = "sql"
		}
		if !slices.Contains(options, value) {
			options = append([]string{value}, options...)
		}
		out = append(out, field{stage: applicationStage, key: choice.key, label: choice.label, kind: choiceField, options: options, value: value})
	}
	return out
}

func executableSelected(m Model) bool {
	for _, key := range []string{"features.http", "features.cli", "features.tui"} {
		if v := m.value(key); v != "none" && v != "" {
			return true
		}
	}
	return false
}

func (m Model) keepCursor(key string) Model {
	for pos, i := range m.visibleFields() {
		if m.fields[i].key == key {
			m.cursor[m.stage] = pos
		}
	}
	return m
}

func (m Model) choose(f field, delta int) (tea.Model, tea.Cmd) {
	i := slices.Index(f.options, f.value)
	next := f.options[(i+delta+len(f.options))%len(f.options)]
	m = m.set(f.key, next, true).keepCursor(f.key)
	switch {
	case f.key == "mode":
		m.base.Mode = next
		m.reqID++
		return m, m.prepareCmd(m.reqID, true, m.request())
	case f.key == "features.database" && next == "none":
		if i := m.index("features.access"); i >= 0 {
			m.fields[i].value, m.fields[i].dirty = "sql", false
		}
	}
	return m, nil
}
