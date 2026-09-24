package render_test

import (
	"os"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

var (
	sqliteTests   = []string{"TestRoundTrip", "TestNotFound", "TestUpsert", "TestNewRejectsNil", "TestCancelled", "TestClose"}
	postgresTests = []string{"TestGetBindsParameter", "TestGetNotFound", "TestGetScanError", "TestPutBindsParameters",
		"TestPutError", "TestPingFailure", "TestCancelled", "TestCloseError", "TestNewRejectsNil", "TestOpenIsLazy"}
)

func TestGeneratedDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated projects")
	}
	tests := []struct {
		name string
		f    config.Features
		want []string
	}{
		{"sqlite library", features(func(f *config.Features) { f.Database, f.Access = "sqlite", "sql" }), sqliteTests},
		{"postgres library", features(func(f *config.Features) { f.Database, f.Access = "postgres", "sql" }), postgresTests},
		{"sqlite all executables", features(func(f *config.Features) {
			f.Database, f.Access, f.HTTP, f.CLI, f.TUI = "sqlite", "sql", "nethttp", "flag", "bubbletea"
		}), append([]string{"TestHealth", "TestStatus", "TestQuit"}, sqliteTests...)},
		{"postgres chi cobra viper", features(func(f *config.Features) {
			f.Database, f.Access, f.HTTP, f.CLI, f.Config = "postgres", "sql", "chi", "cobra", "viper"
		}), append([]string{"TestHealth", "TestStatus", "TestFileOverridesDefault"}, postgresTests...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildAndTest(t, tt.f, tt.want...)
		})
	}
}

func TestGeneratedDatabaseFiles(t *testing.T) {
	for _, db := range []string{"sqlite", "postgres"} {
		c := testproject.Config()
		c.Features.Database, c.Features.Access, c.Features.HTTP = db, "sql", "nethttp"
		files, err := render.Files(c, "new")
		if err != nil {
			t.Fatal(err)
		}
		driver := map[string]string{"sqlite": `_ "modernc.org/sqlite"`, "postgres": `_ "github.com/jackc/pgx/v5/stdlib"`}[db]
		for _, path := range []string{"internal/store/store.go", "internal/store/sql.go"} {
			if strings.Contains(string(testproject.File(t, files, path)), driver) {
				t.Errorf("%s: library code registers the driver", path)
			}
		}
		main := string(testproject.File(t, files, "cmd/server/main.go"))
		if !strings.Contains(main, driver) || !strings.Contains(main, "st.Ping") || !strings.Contains(main, "st.Close()") {
			t.Errorf("%s main does not register driver, wire readiness, and close:\n%s", db, main)
		}
		query := string(testproject.File(t, files, "internal/store/sql.go"))
		placeholder := map[string]string{"sqlite": "key = ?", "postgres": "key = $1"}[db]
		if !strings.Contains(query, placeholder) || !strings.Contains(query, "ON CONFLICT (key) DO UPDATE SET value = excluded.value") {
			t.Errorf("%s queries:\n%s", db, query)
		}
		if !strings.Contains(string(testproject.File(t, files, "sql/schema.sql")), "CREATE TABLE") {
			t.Errorf("%s schema missing", db)
		}
		agents := string(testproject.File(t, files, "AGENTS.md"))
		if !strings.Contains(agents, "sql/schema.sql") {
			t.Errorf("%s guidance does not explain schema setup", db)
		}
		if db == "postgres" {
			for _, want := range []string{"TEST_DATABASE_URL", "-tags integration", "APP_DATABASE_URL"} {
				if !strings.Contains(agents, want) {
					t.Errorf("postgres guidance missing %s:\n%s", want, agents)
				}
			}
		}
	}
	c := testproject.Config()
	c.Features.Database, c.Features.Access = "sqlite", "sql"
	files, err := render.Files(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(testproject.File(t, files, "AGENTS.md")), "modernc.org/sqlite") {
		t.Error("library-only guidance omits driver registration")
	}
	c.Project.Starter = "runnable"
	files, err = render.Files(c, "new")
	if err != nil {
		t.Fatal(err)
	}
	if main := string(testproject.File(t, files, "main.go")); !strings.Contains(main, "store.Open") || !strings.Contains(main, `_ "modernc.org/sqlite"`) {
		t.Errorf("runnable main does not use the store:\n%s", main)
	}
}

func TestGeneratedPostgresIntegration(t *testing.T) {
	url := os.Getenv("RUBRIC_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set RUBRIC_TEST_DATABASE_URL to run generated PostgreSQL integration tests")
	}
	for _, access := range []string{"sql", "sqlc"} {
		t.Run(access, func(t *testing.T) {
			c := testproject.Config()
			c.Features.Database, c.Features.Access = "postgres", access
			files, err := render.Files(c, "new")
			if err != nil {
				t.Fatal(err)
			}
			root := testproject.Write(t, files)
			testproject.Go(t, root, "mod", "tidy")
			t.Setenv("TEST_DATABASE_URL", url)
			out := testproject.Go(t, root, "test", "-count=1", "-tags", "integration", "-v", "./internal/store/...")
			for _, name := range []string{"TestIntegrationRoundTrip", "TestIntegrationUpsert", "TestIntegrationNotFound"} {
				if !strings.Contains(out, "--- PASS: "+name) {
					t.Errorf("%s did not pass:\n%s", name, out)
				}
			}
		})
	}
}
