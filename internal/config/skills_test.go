package config

import (
	"slices"
	"strings"
	"testing"
)

func TestSkillsDecodeLegacyBoolAndList(t *testing.T) {
	tests := []struct {
		name, yaml string
		want       []string
	}{
		{"legacy true", "tooling:\n  skills: true\n", SkillGroups},
		{"legacy false", "tooling:\n  skills: false\n", []string{}},
		{"list", "tooling:\n  skills: [agents, rubric, pstack, principles]\n", SkillGroups},
		{"empty list", "tooling:\n  skills: []\n", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Decode([]byte(tt.yaml))
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := Resolve(Defaults(), doc.Values)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(cfg.Tooling.Skills, tt.want) {
				t.Fatalf("skills = %v, want %v", cfg.Tooling.Skills, tt.want)
			}
		})
	}
}

func TestSkillsDecodeRejectsOtherScalars(t *testing.T) {
	if _, err := Decode([]byte("tooling:\n  skills: pstack\n")); err == nil || !strings.Contains(err.Error(), "tooling.skills") {
		t.Fatalf("err = %v", err)
	}
}

func TestSkillsEncodeUpgradesLegacyBool(t *testing.T) {
	doc, err := Decode([]byte("# keep me\ntooling:\n  skills: true # groups\n"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Resolve(Defaults(), doc.Values, Patch{"project.module": "example.com/a", "tooling.skills": []string{SkillsRubric}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := Encode(doc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "# keep me") || !strings.Contains(text, "- rubric") || strings.Contains(text, "skills: true") {
		t.Fatalf("encoded:\n%s", text)
	}
	again, err := Decode(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Values["tooling.skills"]; !slices.Equal(got.([]string), []string{SkillsRubric}) {
		t.Fatalf("round trip = %v", got)
	}
}

func TestSkillsValidateGroupsAndDependencies(t *testing.T) {
	tests := []struct {
		skills  []string
		wantErr string
	}{
		{[]string{SkillsRubric}, ""},
		{[]string{SkillsPrinciples}, ""},
		{[]string{SkillsPstack, SkillsPrinciples}, ""},
		{SkillGroups, ""},
		{[]string{SkillsPstack}, "pstack requires principles"},
		{[]string{SkillsAgents, SkillsPrinciples}, "agents requires pstack"},
		{[]string{"bogus"}, `"bogus" must be one of`},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.skills, ","), func(t *testing.T) {
			cfg, err := Resolve(Defaults(), Patch{"project.module": "example.com/a", "tooling.skills": tt.skills})
			if err != nil {
				t.Fatal(err)
			}
			err = Validate(cfg, "new")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseSkills(t *testing.T) {
	tests := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{"all", SkillGroups, false},
		{"true", SkillGroups, false},
		{"none", []string{}, false},
		{"false", []string{}, false},
		{"agents, rubric", []string{SkillsRubric, SkillsAgents}, false},
		{"rubric,nope", nil, true},
	}
	for _, tt := range tests {
		got, err := ParseSkills(tt.in)
		if (err != nil) != tt.wantErr || !slices.Equal(got, tt.want) {
			t.Fatalf("ParseSkills(%q) = %v, %v", tt.in, got, err)
		}
	}
}
