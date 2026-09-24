package wizard

import (
	"fmt"

	"github.com/michael-duren/go-skills/internal/config"
)

func toolingFields(t config.Tooling) []field {
	return []field{
		{stage: toolingStage, key: "tooling.skills", label: "Agent skills", kind: toggleField, value: fmt.Sprint(t.Skills),
			help: "adds .agents/skills/rubric-{workflow,testing,style} plus pstack skills and agents"},
		{stage: toolingStage, key: "tooling.lint", label: "Linting", kind: toggleField, value: fmt.Sprint(t.Lint),
			help: "adds .golangci.yml, .rubric/style, and .rubric/check.sh"},
		{stage: toolingStage, key: "tooling.makefile", label: "Makefile", kind: toggleField, value: fmt.Sprint(t.Makefile),
			help: "adds Makefile and .rubric/check.sh"},
		{stage: toolingStage, key: "tooling.actions", label: "GitHub Actions", kind: toggleField, value: fmt.Sprint(t.Actions),
			help: "adds .github/workflows/ci.yml and .rubric/check.sh"},
	}
}
