package render_test

import (
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

func TestGeneratedHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated projects")
	}
	for _, router := range []string{"nethttp", "chi"} {
		t.Run(router, func(t *testing.T) {
			c := testproject.Config()
			c.Features.HTTP = router
			files, err := render.Files(c, "new")
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{
				"internal/httpserver/routes.go", "internal/httpserver/server.go",
				"internal/httpserver/server_test.go", "cmd/server/main.go",
			} {
				data := testproject.File(t, files, path)
				if strings.Contains(string(data), "\t// ") || strings.Contains(string(data), " // ") {
					t.Errorf("%s has explanatory comments", path)
				}
			}
			if _, has := find(files, "main.go"); has {
				t.Fatal("root main.go generated alongside cmd/server")
			}
			routes := string(testproject.File(t, files, "internal/httpserver/routes.go"))
			if router == "chi" != strings.Contains(routes, "github.com/go-chi/chi/v5") {
				t.Fatalf("router import mismatch:\n%s", routes)
			}
			root := testproject.Write(t, files)
			testproject.Go(t, root, "mod", "tidy")
			testproject.Go(t, root, "build", "./...")
			out := testproject.Go(t, root, "test", "-count=1", "-v", "./...")
			for _, name := range []string{"TestHealth", "TestReadiness", "TestUnknownRouteAndMethod", "TestServeListenerFailure", "TestServeGracefulShutdown"} {
				if !strings.Contains(out, "--- PASS: "+name) {
					t.Errorf("generated test %s did not pass:\n%s", name, out)
				}
			}
		})
	}
}

func find(files []render.File, path string) (render.File, bool) {
	for _, f := range files {
		if f.Path == path {
			return f, true
		}
	}
	return render.File{}, false
}
