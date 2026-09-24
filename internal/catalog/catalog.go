// Package catalog lists the supported generation choices and the dependency versions pinned for them.
package catalog

import (
	"fmt"

	"github.com/michael-duren/go-skills/internal/config"
)

const (
	chi       = "github.com/go-chi/chi/v5"
	sqlite    = "modernc.org/sqlite"
	pgx       = "github.com/jackc/pgx/v5"
	cobra     = "github.com/spf13/cobra"
	bubbletea = "charm.land/bubbletea/v2"
	viper     = "github.com/spf13/viper"
	sqlmock   = "github.com/DATA-DOG/go-sqlmock"
)

var versions = map[string]string{
	chi:       "v5.3.2",
	sqlite:    "v1.59.0",
	pgx:       "v5.11.0",
	cobra:     "v1.10.2",
	bubbletea: "v2.0.9",
	viper:     "v1.21.0",
	sqlmock:   "v1.5.2",
}

var choices = map[string]map[string]string{
	"http":     {"none": "", "nethttp": "", "chi": chi},
	"database": {"none": "", "sqlite": sqlite, "postgres": pgx},
	"access":   {"none": "", "sql": "", "sqlc": ""},
	"cli":      {"none": "", "flag": "", "cobra": cobra},
	"tui":      {"none": "", "bubbletea": bubbletea},
	"config":   {"stdlib": "", "viper": viper},
}

// Dependencies returns the pinned module versions required by the selected features.
func Dependencies(f config.Features) (map[string]string, error) {
	deps := map[string]string{}
	for _, sel := range []struct{ name, value string }{
		{"http", f.HTTP}, {"database", f.Database}, {"access", f.Access},
		{"cli", f.CLI}, {"tui", f.TUI}, {"config", f.Config},
	} {
		module, ok := choices[sel.name][sel.value]
		if !ok {
			return nil, fmt.Errorf("features.%s: unsupported choice %q", sel.name, sel.value)
		}
		if module != "" {
			deps[module] = versions[module]
		}
	}
	if f.Database == "postgres" {
		deps[sqlmock] = versions[sqlmock]
	}
	return deps, nil
}

// Cases enumerates every supported feature combination in a stable order.
func Cases() []config.Features {
	stores := [][2]string{{"none", "none"}, {"sqlite", "sql"}, {"sqlite", "sqlc"}, {"postgres", "sql"}, {"postgres", "sqlc"}}
	var out []config.Features
	for _, http := range []string{"none", "nethttp", "chi"} {
		for _, store := range stores {
			for _, cli := range []string{"none", "flag", "cobra"} {
				for _, tui := range []string{"none", "bubbletea"} {
					for _, cfg := range []string{"stdlib", "viper"} {
						out = append(out, config.Features{
							HTTP: http, Database: store[0], Access: store[1], CLI: cli, TUI: tui, Config: cfg,
						})
					}
				}
			}
		}
	}
	return out
}

var tools = map[string]struct{ path, version string }{
	"sqlc":          {"github.com/sqlc-dev/sqlc/cmd/sqlc", "v1.31.1"},
	"golangci-lint": {"github.com/golangci/golangci-lint/v2/cmd/golangci-lint", "v2.13.2"},
}

// Tools returns pinned versions of the tools the selected features and tooling run through the Go toolchain.
func Tools(f config.Features, t config.Tooling) map[string]string {
	out := map[string]string{}
	if f.Access == "sqlc" {
		out["sqlc"] = tools["sqlc"].version
	}
	if t.Lint {
		out["golangci-lint"] = tools["golangci-lint"].version
	}
	return out
}

// Launcher returns the go run argument that executes the pinned tool, such as path@version.
func Launcher(name string) string {
	t := tools[name]
	return t.path + "@" + t.version
}
