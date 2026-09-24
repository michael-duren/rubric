package render

import (
	"fmt"
	"slices"

	"github.com/michael-duren/go-skills/internal/config"
)

var drivers = map[string]struct{ name, pkg string }{
	"sqlite":   {"sqlite", "modernc.org/sqlite"},
	"postgres": {"pgx", "github.com/jackc/pgx/v5/stdlib"},
}

var databaseOutputs = []output{
	{path: "internal/store/store.go", template: "store/store.go.tmpl", kind: KindScaffold, when: handwritten},
	{path: "internal/store/store.go", template: "sqlc/store.go.tmpl", kind: KindScaffold, when: sqlcFor("sqlite", "postgres")},
	{path: "internal/store/sql.go", template: "store/sql.go.tmpl", kind: KindScaffold, when: databaseIs("sqlite", "postgres")},
	{path: "internal/store/sqlite_test.go", template: "store/sqlite_test.go.tmpl", kind: KindScaffold, when: databaseIs("sqlite")},
	{path: "internal/store/store_test.go", template: "store/store_test.go.tmpl", kind: KindScaffold, when: databaseIs("postgres")},
	{path: "internal/store/postgres_integration_test.go", template: "store/postgres_integration_test.go.tmpl", kind: KindScaffold, when: databaseIs("postgres")},
	{path: "sql/schema.sql", template: "store/schema.sqlite.sql.tmpl", kind: KindScaffold, when: databaseIs("sqlite")},
	{path: "sql/schema.sql", template: "store/schema.postgres.sql.tmpl", kind: KindScaffold, when: databaseIs("postgres")},
}

func databaseIs(names ...string) func(config.Config, string) bool {
	return func(c config.Config, mode string) bool {
		return mode == "new" && slices.Contains(names, c.Features.Database)
	}
}

func handwritten(c config.Config, mode string) bool {
	return c.Features.Access != "sqlc" && databaseIs("sqlite", "postgres")(c, mode)
}

func databaseGuidance(c config.Config) []string {
	d, ok := drivers[c.Features.Database]
	if !ok {
		return nil
	}
	return []string{
		fmt.Sprintf("`internal/store`: database access and its tests; importers register the `%s` driver with `import _ %q` in their main package", d.name, d.pkg),
		"`sql/schema.sql`: schema to apply to your database yourself; Rubric never creates tables or changes a database",
	}
}
