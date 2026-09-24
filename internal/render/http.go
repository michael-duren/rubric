package render

import (
	"slices"

	"github.com/michael-duren/go-skills/internal/config"
)

var httpOutputs = []output{
	{path: "internal/httpserver/routes.go", template: "http/nethttp.go.tmpl", kind: KindScaffold, when: httpIs("nethttp")},
	{path: "internal/httpserver/routes.go", template: "http/chi.go.tmpl", kind: KindScaffold, when: httpIs("chi")},
	{path: "internal/httpserver/server.go", template: "http/server.go.tmpl", kind: KindScaffold, when: httpIs("nethttp", "chi")},
	{path: "internal/httpserver/server_test.go", template: "http/server_test.go.tmpl", kind: KindScaffold, when: httpIs("nethttp", "chi")},
	{path: "cmd/server/main.go", template: "http/main.go.tmpl", kind: KindScaffold, when: httpIs("nethttp", "chi")},
}

func httpIs(routers ...string) func(config.Config, string) bool {
	return func(c config.Config, mode string) bool {
		return mode == "new" && slices.Contains(routers, c.Features.HTTP)
	}
}
