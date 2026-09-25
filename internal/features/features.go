// Package features catalogs the Rubric features a repository can toggle and reads or changes them on a configuration.
package features

import (
	"slices"
	"strings"

	"github.com/michael-duren/go-skills/internal/config"
)

const (
	// MenuSkills groups the agent skill toggles.
	MenuSkills = "Skills"
	// MenuTooling groups the development tooling toggles.
	MenuTooling = "Tooling"

	skillPrefix = "skills."
)

// Feature is one toggle; Key is skills.<group> or the tooling.<name> configuration key.
type Feature struct {
	Key   string
	Menu  string
	Label string
	Help  string
}

// Menus lists the submenus in display order.
var Menus = []string{MenuSkills, MenuTooling}

// All lists every feature in display order.
var All = []Feature{
	{skillPrefix + config.SkillsRubric, MenuSkills, "Rubric skills", ".agents/skills/rubric-{workflow,testing,style}"},
	{skillPrefix + config.SkillsPstack, MenuSkills, "pstack skills", "poteto-mode, tdd, and other pstack workflows in .agents/skills (needs principles)"},
	{skillPrefix + config.SkillsPrinciples, MenuSkills, "pstack principles", ".agents/skills/principle-* and the pstack license notice"},
	{skillPrefix + config.SkillsAgents, MenuSkills, "pstack agents", "subagents in .agents/agents (needs pstack skills)"},
	{"tooling.lint", MenuTooling, "Linting", ".golangci.yml, .rubric/style, and .rubric/check.sh"},
	{"tooling.makefile", MenuTooling, "Makefile", "Makefile and .rubric/check.sh"},
	{"tooling.actions", MenuTooling, "GitHub Actions", ".github/workflows/ci.yml and .rubric/check.sh"},
}

// In returns the features shown in menu.
func In(menu string) []Feature {
	var out []Feature
	for _, f := range All {
		if f.Menu == menu {
			out = append(out, f)
		}
	}
	return out
}

// SkillGroup returns the skill group a key toggles, or "" for tooling keys.
func SkillGroup(key string) string {
	group, ok := strings.CutPrefix(key, skillPrefix)
	if !ok {
		return ""
	}
	return group
}

// SkillKey returns the feature key that toggles group.
func SkillKey(group string) string {
	return skillPrefix + group
}

// Enabled reports whether key is on in t.
func Enabled(t config.Tooling, key string) bool {
	if group := SkillGroup(key); group != "" {
		return t.Has(group)
	}
	switch key {
	case "tooling.lint":
		return t.Lint
	case "tooling.makefile":
		return t.Makefile
	case "tooling.actions":
		return t.Actions
	}
	return false
}

// Build returns the tooling whose features on reports as enabled, without cascading dependencies.
func Build(on func(key string) bool) config.Tooling {
	t := config.Tooling{Skills: []string{}}
	for _, g := range config.SkillGroups {
		if on(SkillKey(g)) {
			t.Skills = append(t.Skills, g)
		}
	}
	t.Lint, t.Makefile, t.Actions = on("tooling.lint"), on("tooling.makefile"), on("tooling.actions")
	return t
}

// Set returns t with key switched on or off, enabling required skill groups and disabling groups that depend on it.
func Set(t config.Tooling, key string, on bool) config.Tooling {
	t.Skills = slices.Clone(t.Skills)
	group := SkillGroup(key)
	switch {
	case group != "" && on:
		for g := group; g != "" && !t.Has(g); g = config.Requires(g) {
			t.Skills = append(t.Skills, g)
		}
	case group != "":
		off := map[string]bool{group: true}
		for changed := true; changed; {
			changed = false
			for _, g := range config.SkillGroups {
				if !off[g] && off[config.Requires(g)] {
					off[g], changed = true, true
				}
			}
		}
		t.Skills = slices.DeleteFunc(t.Skills, func(g string) bool { return off[g] })
	case key == "tooling.lint":
		t.Lint = on
	case key == "tooling.makefile":
		t.Makefile = on
	case key == "tooling.actions":
		t.Actions = on
	}
	ordered := []string{}
	for _, g := range config.SkillGroups {
		if t.Has(g) {
			ordered = append(ordered, g)
		}
	}
	t.Skills = ordered
	return t
}
