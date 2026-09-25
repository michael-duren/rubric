package wizard

import (
	"fmt"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/features"
)

func toolingFields(t config.Tooling) []field {
	out := make([]field, len(features.All))
	for i, f := range features.All {
		out[i] = field{stage: toolingStage, key: f.Key, menu: f.Menu, label: f.Label, kind: toggleField,
			value: fmt.Sprint(features.Enabled(t, f.Key)), help: "adds " + f.Help}
	}
	return out
}

func (m Model) tooling() config.Tooling {
	return features.Build(func(key string) bool { return m.value(key) == "true" })
}

// setFeature switches key and cascades skill dependencies, marking every field it changes as edited.
func (m Model) setFeature(key string, on bool) Model {
	next := features.Set(m.tooling(), key, on)
	for _, f := range features.All {
		value := fmt.Sprint(features.Enabled(next, f.Key))
		if f.Key == key || value != m.value(f.Key) {
			m = m.set(f.Key, value, true)
		}
	}
	return m
}

func (m Model) skillsDirty() bool {
	for _, f := range m.fields {
		if f.dirty && features.SkillGroup(f.key) != "" {
			return true
		}
	}
	return false
}
