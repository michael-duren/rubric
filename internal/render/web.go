package render

import "github.com/michael-duren/go-skills/internal/config"

var webOutputs = []output{
	{path: "internal/web/views.templ", template: "web/views.templ", raw: true, kind: KindScaffold, when: webSelected},
	{path: "internal/web/views_templ.go", template: "web/views_templ.go.raw", raw: true, kind: KindScaffold, when: webSelected},
	{path: "internal/web/web.go", template: "web/web.go.tmpl", kind: KindScaffold, when: webSelected},
	{path: "internal/web/web_test.go", template: "web/web_test.go.tmpl", kind: KindScaffold, when: webSelected},
	{path: "internal/web/static/htmx.min.js", template: "web/static/htmx.min.js", raw: true, kind: KindScaffold, when: webSelected},
	{path: "internal/web/static/alpine.min.js", template: "web/static/alpine.min.js", raw: true, kind: KindScaffold, when: webSelected},
	{path: "internal/web/static/THIRD_PARTY_LICENSES.md", template: "web/static/THIRD_PARTY_LICENSES.md", raw: true, kind: KindScaffold, when: webSelected},
	{path: "e2e/ui_test.go", template: "web/ui_test.go.tmpl", kind: KindScaffold, when: e2eSelected},
}

func webSelected(c config.Config, mode string) bool {
	return mode == "new" && c.Features.Web == "htmx" && c.Features.HTTP != "none"
}

func e2eSelected(c config.Config, mode string) bool {
	return webSelected(c, mode) && c.Features.E2E == "playwright"
}
