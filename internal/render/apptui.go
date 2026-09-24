package render

import "github.com/michael-duren/go-skills/internal/config"

var tuiOutputs = []output{
	{path: "internal/tui/model.go", template: "tui/model.go.tmpl", kind: KindScaffold, when: tuiSelected},
	{path: "internal/tui/model_test.go", template: "tui/model_test.go.tmpl", kind: KindScaffold, when: tuiSelected},
	{path: "cmd/tui/main.go", template: "tui/main.go.tmpl", kind: KindScaffold, when: tuiSelected},
}

func tuiSelected(c config.Config, mode string) bool {
	return mode == "new" && c.Features.TUI == "bubbletea"
}
