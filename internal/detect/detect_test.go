package detect

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
)

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[rel] = info.Mode().String() + string(data)
		} else {
			out[rel] = info.Mode().String()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func inspect(t *testing.T, root string) Facts {
	t.Helper()
	before := snapshot(t, root)
	facts, err := Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if after := snapshot(t, root); !mapsEqual(before, after) {
		t.Fatal("inspection modified the target")
	}
	return facts
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func has(facts Facts, field, value string) bool {
	return slices.ContainsFunc(facts.Evidence, func(e config.Evidence) bool {
		return e.Field == field && e.Value == value
	})
}

func evidence(facts Facts, field, value string) config.Evidence {
	for _, e := range facts.Evidence {
		if e.Field == field && e.Value == value {
			return e
		}
	}
	return config.Evidence{}
}

const gomod = "module example.com/app\n\ngo 1.26.7\n"

func TestDependencyIsNotAnActiveComponent(t *testing.T) {
	root := t.TempDir()
	mod := "module example.com/app\n\ngo 1.26.7\n\nrequire github.com/go-chi/chi/v5 v5.3.2\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	facts := inspect(t, root)
	for _, item := range facts.Evidence {
		if item.Field == "features.http" && item.Value == "chi" {
			t.Fatal("unused dependency classified as active HTTP component")
		}
	}
	if got := evidence(facts, "dependency", "github.com/go-chi/chi/v5@v5.3.2"); got.Source != "go.mod" {
		t.Fatalf("dependency evidence = %+v", facts.Evidence)
	}
	if facts.Module != "example.com/app" || facts.Go != "1.26.7" || facts.Empty || facts.Workspace {
		t.Fatalf("facts = %+v", facts)
	}
}

func TestStdlibServerDetected(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod": gomod,
		"cmd/api/main.go": `package main

import "net/http"

func main() {
	mux := http.NewServeMux()
	_ = http.ListenAndServe(":8080", mux)
}
`,
	})
	facts := inspect(t, root)
	if got := evidence(facts, "features.http", "nethttp"); got.Source != "cmd/api/main.go" {
		t.Fatalf("evidence = %+v", facts.Evidence)
	}
	want := []config.EntryPoint{{Name: "api", Dir: "cmd/api"}}
	if !slices.Equal(facts.EntryPoints, want) {
		t.Fatalf("entry points = %+v", facts.EntryPoints)
	}
}

func TestChiRouterDetectedFromConstruction(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod": gomod,
		"internal/web/routes.go": `package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func Routes() http.Handler {
	r := chi.NewRouter()
	return r
}
`,
	})
	facts := inspect(t, root)
	if !has(facts, "features.http", "chi") || has(facts, "features.http", "nethttp") {
		t.Fatalf("evidence = %+v", facts.Evidence)
	}
	if got := evidence(facts, "library", "github.com/go-chi/chi/v5"); got.Source != "internal/web/routes.go" {
		t.Fatalf("library evidence = %+v", facts.Evidence)
	}
}

func TestHTTPClientOnlyIsNotAServer(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod": gomod,
		"client.go": `package app

import "net/http"

func Fetch(url string) (*http.Response, error) {
	c := &http.Client{}
	return c.Get(url)
}
`,
	})
	facts := inspect(t, root)
	if has(facts, "features.http", "nethttp") {
		t.Fatalf("client classified as server: %+v", facts.Evidence)
	}
	if !has(facts, "library", "net/http") {
		t.Fatalf("stdlib import not recorded: %+v", facts.Evidence)
	}
}

func TestRouterImportedOnlyByTestsIsIgnored(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod": gomod,
		"app.go": "package app\n",
		"app_test.go": `package app

import (
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestX(t *testing.T) { _ = chi.NewRouter() }
`,
	})
	facts := inspect(t, root)
	if has(facts, "features.http", "chi") || has(facts, "library", "github.com/go-chi/chi/v5") {
		t.Fatalf("test-only import classified: %+v", facts.Evidence)
	}
}

func TestUnknownLibrariesDescribedWithoutCatalogClaims(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod": gomod,
		"main.go": `package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	_ = r.Run()
}
`,
	})
	facts := inspect(t, root)
	if !has(facts, "library", "github.com/gin-gonic/gin") || !has(facts, "features.http", "gin") {
		t.Fatalf("evidence = %+v", facts.Evidence)
	}
	if has(facts, "features.http", "chi") || has(facts, "features.http", "nethttp") {
		t.Fatalf("catalog value invented: %+v", facts.Evidence)
	}
	want := []config.EntryPoint{{Name: "app", Dir: "."}}
	if !slices.Equal(facts.EntryPoints, want) {
		t.Fatalf("entry points = %+v", facts.EntryPoints)
	}
}

func TestOtherCapabilitiesDetected(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod":    gomod,
		"sqlc.yaml": "version: \"2\"\n",
		"Makefile":  "test:\n\tgo test ./...\n",
		"internal/store/db.go": `package store

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func Open() (*sql.DB, error) { return sql.Open("sqlite", ":memory:") }
`,
		"cmd/tool/main.go": `package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	tea "charm.land/bubbletea/v2"
)

func main() {
	_ = &cobra.Command{}
	_ = viper.New()
	var _ tea.Model
}
`,
	})
	facts := inspect(t, root)
	for _, want := range [][2]string{
		{"features.database", "sqlite"}, {"features.access", "sqlc"}, {"features.cli", "cobra"},
		{"features.tui", "bubbletea"}, {"features.config", "viper"}, {"file", "Makefile"},
	} {
		if !has(facts, want[0], want[1]) {
			t.Errorf("missing %s=%s in %+v", want[0], want[1], facts.Evidence)
		}
	}
}

func TestMainPackageWithoutMainFunc(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod":             gomod,
		"cmd/helper/util.go": "package main\n\nfunc helper() {}\n",
		"cmd/real/main.go":   "package main\n\nfunc main() {}\n",
		"cmd/method/m.go":    "package main\n\ntype T struct{}\n\nfunc (T) main() {}\n",
	})
	facts := inspect(t, root)
	want := []config.EntryPoint{{Name: "real", Dir: "cmd/real"}}
	if !slices.Equal(facts.EntryPoints, want) {
		t.Fatalf("entry points = %+v", facts.EntryPoints)
	}
}

func TestNestedModulesAreBoundaries(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod":               gomod,
		"main.go":              "package main\n\nfunc main() {}\n",
		"tools/go.mod":         "module example.com/tools\n\ngo 1.26.7\n",
		"tools/cmd/x/main.go":  "package main\n\nimport \"github.com/spf13/cobra\"\n\nfunc main() { _ = cobra.Command{} }\n",
		"nested/deep/go.mod":   "module example.com/deep\n",
		"nested/deep/main.go":  "package main\n\nfunc main() {}\n",
		"nested/other/main.go": "package main\n\nfunc main() {}\n",
	})
	facts := inspect(t, root)
	if !slices.Equal(facts.NestedModules, []string{"nested/deep", "tools"}) {
		t.Fatalf("nested = %v", facts.NestedModules)
	}
	want := []config.EntryPoint{{Name: "app", Dir: "."}, {Name: "other", Dir: "nested/other"}}
	if !slices.Equal(facts.EntryPoints, want) {
		t.Fatalf("entry points = %+v", facts.EntryPoints)
	}
	if has(facts, "features.cli", "cobra") {
		t.Fatal("scanned nested module source")
	}
}

func TestWorkspaceWithoutModule(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.work":    "go 1.26.7\n\nuse ./svc\n",
		"svc/go.mod": "module example.com/svc\n\ngo 1.26.7\n",
	})
	facts := inspect(t, root)
	if !facts.Workspace || facts.Module != "" || !slices.Equal(facts.NestedModules, []string{"svc"}) {
		t.Fatalf("facts = %+v", facts)
	}
}

func TestExplicitModuleBeneathWorkspace(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.work":           "go 1.26.7\n\nuse ./svc\n",
		"svc/go.mod":        "module example.com/svc\n\ngo 1.25.0\n",
		"svc/cmd/a/main.go": "package main\n\nfunc main() {}\n",
	})
	facts := inspect(t, filepath.Join(root, "svc"))
	if facts.Workspace || facts.Module != "example.com/svc" || facts.Go != "1.25.0" {
		t.Fatalf("facts = %+v", facts)
	}
	if len(facts.EntryPoints) != 1 || facts.EntryPoints[0].Dir != "cmd/a" {
		t.Fatalf("entry points = %+v", facts.EntryPoints)
	}
}

func TestMalformedSourceIsReportedNotFatal(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod":  gomod,
		"bad.go":  "package app\n\nfunc {\n",
		"good.go": "package main\n\nfunc main() {}\n",
	})
	facts := inspect(t, root)
	if got := evidence(facts, "warning", "unparsable Go source"); got.Source != "bad.go" {
		t.Fatalf("evidence = %+v", facts.Evidence)
	}
	if len(facts.EntryPoints) != 1 {
		t.Fatalf("entry points = %+v", facts.EntryPoints)
	}
}

func TestMalformedGoModIsAnError(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{"go.mod": "module\n\nbogus directive here\n"})
	if _, err := Inspect(root); err == nil {
		t.Fatal("malformed go.mod accepted")
	}
}

func TestIgnoredDirectories(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod":                     gomod,
		".git/hooks/main.go":         "package main\n\nfunc main() {}\n",
		"vendor/x/main.go":           "package main\n\nfunc main() {}\n",
		".rubric/style/main.go":      "package main\n\nfunc main() {}\n",
		"testdata/fixture/main.go":   "package main\n\nfunc main() {}\n",
		"_scratch/main.go":           "package main\n\nfunc main() {}\n",
		"vendor/github.com/x/go.mod": "module github.com/x\n",
	})
	facts := inspect(t, root)
	if len(facts.EntryPoints) != 0 || len(facts.NestedModules) != 0 {
		t.Fatalf("ignored paths scanned: %+v", facts)
	}
}

func TestSymlinksAreNotFollowed(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	write(t, outside, map[string]string{"main.go": "package main\n\nimport \"github.com/spf13/cobra\"\n\nfunc main() { _ = cobra.Command{} }\n"})
	write(t, root, map[string]string{"go.mod": gomod})
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	if err := os.Symlink(filepath.Join(outside, "main.go"), filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	facts := inspect(t, root)
	if len(facts.EntryPoints) != 0 || has(facts, "features.cli", "cobra") {
		t.Fatalf("symlink followed: %+v", facts)
	}
}

func TestDeletedEntryPointOnRerun(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod":        gomod,
		"cmd/a/main.go": "package main\n\nfunc main() {}\n",
		"cmd/b/main.go": "package main\n\nfunc main() {}\n",
	})
	if got := inspect(t, root).EntryPoints; len(got) != 2 {
		t.Fatalf("first = %+v", got)
	}
	if err := os.RemoveAll(filepath.Join(root, "cmd", "b")); err != nil {
		t.Fatal(err)
	}
	want := []config.EntryPoint{{Name: "a", Dir: "cmd/a"}}
	if got := inspect(t, root).EntryPoints; !slices.Equal(got, want) {
		t.Fatalf("second = %+v", got)
	}
}

func TestEmptyAndAbsentTargets(t *testing.T) {
	root := t.TempDir()
	if facts := inspect(t, root); !facts.Empty || facts.Module != "" {
		t.Fatalf("empty = %+v", facts)
	}
	write(t, root, map[string]string{".git/HEAD": "ref: refs/heads/main\n"})
	if facts := inspect(t, root); !facts.Empty {
		t.Fatalf("git-only dir not empty: %+v", facts)
	}
	write(t, root, map[string]string{"notes.txt": "hi"})
	if facts := inspect(t, root); facts.Empty || facts.Module != "" {
		t.Fatalf("nonempty = %+v", facts)
	}
	if _, err := Inspect(filepath.Join(root, "missing")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("absent err = %v", err)
	}
}

func TestEvidenceSortedAndDeduplicated(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod": gomod,
		"a.go":   "package app\n\nimport \"net/http\"\n\nfunc A() { _ = http.ListenAndServe(\"\", nil); _ = http.ListenAndServe(\"\", nil) }\n",
		"b.go":   "package app\n\nimport \"net/http\"\n\nvar _ = http.NewServeMux()\n",
	})
	facts := inspect(t, root)
	if !slices.IsSortedFunc(facts.Evidence, compareEvidence) {
		t.Fatalf("unsorted: %+v", facts.Evidence)
	}
	if len(slices.CompactFunc(slices.Clone(facts.Evidence), func(a, b config.Evidence) bool { return a == b })) != len(facts.Evidence) {
		t.Fatalf("duplicates: %+v", facts.Evidence)
	}
}
