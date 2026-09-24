package render

import (
	"slices"

	"github.com/michael-duren/go-skills/internal/config"
)

var cliOutputs = []output{
	{path: "internal/cli/command.go", template: "cli/flag.go.tmpl", kind: KindScaffold, when: cliIs("flag")},
	{path: "internal/cli/command.go", template: "cli/cobra.go.tmpl", kind: KindScaffold, when: cliIs("cobra")},
	{path: "internal/cli/command_test.go", template: "cli/command_test.go.tmpl", kind: KindScaffold, when: cliIs("flag", "cobra")},
	{path: "cmd/cli/main.go", template: "cli/main.go.tmpl", kind: KindScaffold, when: cliIs("flag", "cobra")},
}

func cliIs(impls ...string) func(config.Config, string) bool {
	return func(c config.Config, mode string) bool {
		return mode == "new" && slices.Contains(impls, c.Features.CLI)
	}
}
