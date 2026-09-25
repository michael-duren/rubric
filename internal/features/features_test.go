package features

import (
	"slices"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
)

func TestSetCascadesSkillDependencies(t *testing.T) {
	tests := []struct {
		name  string
		start []string
		key   string
		on    bool
		want  []string
	}{
		{"agents pulls pstack and principles", []string{}, SkillKey(config.SkillsAgents), true, []string{"pstack", "principles", "agents"}},
		{"principles off drops dependents", config.SkillGroups, SkillKey(config.SkillsPrinciples), false, []string{"rubric"}},
		{"pstack off keeps principles", config.SkillGroups, SkillKey(config.SkillsPstack), false, []string{"rubric", "principles"}},
		{"rubric stands alone", []string{}, SkillKey(config.SkillsRubric), true, []string{"rubric"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := config.Tooling{Skills: slices.Clone(tt.start)}
			got := Set(start, tt.key, tt.on)
			if !slices.Equal(got.Skills, tt.want) {
				t.Fatalf("skills = %v, want %v", got.Skills, tt.want)
			}
			if !slices.Equal(start.Skills, tt.start) {
				t.Fatal("Set mutated its input")
			}
			cfg := config.Defaults()
			cfg.Project.Module = "example.com/a"
			cfg.Tooling = got
			if err := config.Validate(cfg, "new"); err != nil {
				t.Fatalf("cascade produced invalid config: %v", err)
			}
		})
	}
}

func TestEnabledAndSetAgreeForEveryFeature(t *testing.T) {
	for _, f := range All {
		on := Set(config.Tooling{}, f.Key, true)
		if !Enabled(on, f.Key) {
			t.Errorf("%s not enabled after Set on", f.Key)
		}
		if Enabled(Set(on, f.Key, false), f.Key) {
			t.Errorf("%s still enabled after Set off", f.Key)
		}
	}
	for _, m := range Menus {
		if len(In(m)) == 0 {
			t.Errorf("menu %s is empty", m)
		}
	}
}

func TestBuildReadsEachFeatureWithoutCascading(t *testing.T) {
	on := map[string]bool{SkillKey(config.SkillsAgents): true, "tooling.makefile": true}
	got := Build(func(key string) bool { return on[key] })
	if !slices.Equal(got.Skills, []string{config.SkillsAgents}) || !got.Makefile || got.Lint || got.Actions {
		t.Fatalf("Build = %+v", got)
	}
}
