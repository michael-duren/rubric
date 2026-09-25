package config

import (
	"fmt"
	"slices"
	"strings"
)

const (
	// SkillsRubric installs the rubric-workflow, rubric-testing, and rubric-style skills.
	SkillsRubric = "rubric"
	// SkillsPstack installs the vendored pstack workflow skills.
	SkillsPstack = "pstack"
	// SkillsPrinciples installs the vendored pstack principle-* skills.
	SkillsPrinciples = "principles"
	// SkillsAgents installs the vendored pstack subagents.
	SkillsAgents = "agents"
)

// SkillGroups lists every skill group in canonical order.
var SkillGroups = []string{SkillsRubric, SkillsPstack, SkillsPrinciples, SkillsAgents}

var skillRequires = map[string]string{SkillsPstack: SkillsPrinciples, SkillsAgents: SkillsPstack}

// Requires returns the group that group depends on, or "" when it stands alone.
func Requires(group string) string {
	return skillRequires[group]
}

// Has reports whether the skill group is enabled.
func (t Tooling) Has(group string) bool {
	return slices.Contains(t.Skills, group)
}

// ParseSkills reads a comma-separated group list; all or true selects every group, none or false selects none.
func ParseSkills(text string) ([]string, error) {
	switch strings.TrimSpace(text) {
	case "all", "true":
		return slices.Clone(SkillGroups), nil
	case "", "none", "false":
		return []string{}, nil
	}
	var out []string
	for _, part := range strings.Split(text, ",") {
		group := strings.TrimSpace(part)
		if !slices.Contains(SkillGroups, group) {
			return nil, fmt.Errorf("%q must be one of %s, all, or none", group, strings.Join(SkillGroups, ", "))
		}
		out = append(out, group)
	}
	return canonicalSkills(out), nil
}

func canonicalSkills(groups []string) []string {
	out := []string{}
	for _, g := range SkillGroups {
		if slices.Contains(groups, g) {
			out = append(out, g)
		}
	}
	for _, g := range groups {
		if !slices.Contains(out, g) {
			out = append(out, g)
		}
	}
	return out
}

func skillsValue(v any) ([]string, error) {
	switch v := v.(type) {
	case bool:
		if v {
			return slices.Clone(SkillGroups), nil
		}
		return []string{}, nil
	case []string:
		return canonicalSkills(v), nil
	}
	return nil, fmt.Errorf("expected a list of skill groups")
}
