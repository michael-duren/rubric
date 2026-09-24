package render

import "github.com/michael-duren/go-skills/internal/config"

var sqlcOutputs = []output{
	{path: "sqlc.yaml", template: "sqlc/sqlc.yaml.tmpl", kind: KindScaffold, when: sqlcFor("sqlite", "postgres")},
	{path: "sql/queries.sql", template: "sqlc/queries.sqlite.sql.tmpl", kind: KindScaffold, when: sqlcFor("sqlite")},
	{path: "sql/queries.sql", template: "sqlc/queries.postgres.sql.tmpl", kind: KindScaffold, when: sqlcFor("postgres")},
	{path: "internal/store/queries/queries_test.go", template: "sqlc/queries_test.go.tmpl", kind: KindScaffold, when: sqlcFor("sqlite", "postgres")},
	{path: "internal/store/queries/db.go", template: "sqlc/sqlite/db.go.tmpl", raw: true, kind: KindScaffold, when: sqlcFor("sqlite")},
	{path: "internal/store/queries/models.go", template: "sqlc/sqlite/models.go.tmpl", raw: true, kind: KindScaffold, when: sqlcFor("sqlite")},
	{path: "internal/store/queries/queries.sql.go", template: "sqlc/sqlite/queries.sql.go.tmpl", raw: true, kind: KindScaffold, when: sqlcFor("sqlite")},
	{path: "internal/store/queries/db.go", template: "sqlc/postgres/db.go.tmpl", raw: true, kind: KindScaffold, when: sqlcFor("postgres")},
	{path: "internal/store/queries/models.go", template: "sqlc/postgres/models.go.tmpl", raw: true, kind: KindScaffold, when: sqlcFor("postgres")},
	{path: "internal/store/queries/queries.sql.go", template: "sqlc/postgres/queries.sql.go.tmpl", raw: true, kind: KindScaffold, when: sqlcFor("postgres")},
}

func sqlcFor(databases ...string) func(config.Config, string) bool {
	return func(c config.Config, mode string) bool {
		return c.Features.Access == "sqlc" && databaseIs(databases...)(c, mode)
	}
}
