package catalog

import (
	"maps"
	"slices"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
)

func TestCasesEnumerateAllUniqueCombinations(t *testing.T) {
	cases := Cases()
	if len(cases) != 180 {
		t.Fatalf("len = %d, want 180", len(cases))
	}
	seen := map[config.Features]bool{}
	for _, c := range cases {
		if seen[c] {
			t.Fatalf("duplicate %+v", c)
		}
		seen[c] = true
		cfg := config.Defaults()
		cfg.Project.Module = "example.com/demo"
		cfg.Features = c
		if err := config.Validate(cfg, "new"); err != nil {
			t.Fatalf("invalid case %+v: %v", c, err)
		}
		if _, err := Dependencies(c); err != nil {
			t.Fatalf("Dependencies(%+v): %v", c, err)
		}
	}
	if !slices.Equal(Cases(), cases) {
		t.Fatal("Cases not deterministic")
	}
}

func TestDependencies(t *testing.T) {
	none := config.Defaults().Features
	got, err := Dependencies(none)
	if err != nil || len(got) != 0 {
		t.Fatalf("none: %v %v", got, err)
	}
	all := config.Features{HTTP: "chi", Database: "postgres", Access: "sqlc", CLI: "cobra", TUI: "bubbletea", Config: "viper"}
	got, err = Dependencies(all)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"github.com/go-chi/chi/v5":       "v5.3.2",
		"github.com/jackc/pgx/v5":        "v5.11.0",
		"github.com/spf13/cobra":         "v1.10.2",
		"charm.land/bubbletea/v2":        "v2.0.9",
		"github.com/spf13/viper":         "v1.21.0",
		"github.com/DATA-DOG/go-sqlmock": "v1.5.2",
	}
	if !maps.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
	got, _ = Dependencies(config.Features{HTTP: "nethttp", Database: "sqlite", Access: "sql", CLI: "flag", TUI: "none", Config: "stdlib"})
	if !maps.Equal(got, map[string]string{"modernc.org/sqlite": "v1.59.0"}) {
		t.Fatalf("stdlib selections: %v", got)
	}
}

func TestTools(t *testing.T) {
	sqlc := config.Features{Database: "sqlite", Access: "sqlc"}
	if got := Tools(sqlc, config.Tooling{}); !maps.Equal(got, map[string]string{"sqlc": "v1.31.1"}) {
		t.Fatalf("sqlc tools = %v", got)
	}
	if got := Tools(sqlc, config.Tooling{Lint: true}); !maps.Equal(got, map[string]string{"sqlc": "v1.31.1", "golangci-lint": "v2.13.2"}) {
		t.Fatalf("existing lint tools = %v", got)
	}
	if got := Tools(config.Features{}, config.Tooling{}); len(got) != 0 {
		t.Fatalf("no tools expected: %v", got)
	}
	if got := Launcher("golangci-lint"); got != "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2" {
		t.Fatalf("launcher = %s", got)
	}
}

func TestDependenciesRejectsUnknownChoices(t *testing.T) {
	for _, f := range []config.Features{
		{HTTP: "gin", Database: "none", Access: "none", CLI: "none", TUI: "none", Config: "stdlib"},
		{HTTP: "none", Database: "mysql", Access: "sql", CLI: "none", TUI: "none", Config: "stdlib"},
		{HTTP: "none", Database: "none", Access: "none", CLI: "none", TUI: "none", Config: "koanf"},
	} {
		if _, err := Dependencies(f); err == nil {
			t.Fatalf("accepted %+v", f)
		}
	}
}
