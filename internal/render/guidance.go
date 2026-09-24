package render

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/michael-duren/go-skills/internal/config"
)

type commandView struct {
	Name, Line, Dir, Env string
}

type guide struct {
	Name, Description, Module, Go string
	ModuleOnly                    bool
	Stack, Layout                 []string
	Commands                      []commandView
	Env                           string
	Settings                      []string
	Tooling                       []string
}

var labels = map[string]string{
	"nethttp":    "standard-library net/http",
	"chi":        "Chi router",
	"sqlite":     "SQLite (modernc.org/sqlite)",
	"postgres":   "PostgreSQL (pgx)",
	"sql":        "handwritten SQL with database/sql",
	"sqlc":       "sqlc-generated queries",
	"flag":       "standard-library flag",
	"cobra":      "Cobra",
	"bubbletea":  "Bubble Tea v2",
	"viper":      "Viper",
	"stdlib":     "standard-library configuration",
	"htmx":       "htmx and Alpine.js pages rendered with templ",
	"templ":      "templ components",
	"playwright": "Playwright browser tests (playwright-go)",
}

var plainWord = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,-]+$`)

// Instructions renders the managed AGENTS.md section, markers included, for the given mode.
func Instructions(c config.Config, mode string) ([]byte, error) {
	g := guide{
		Name:        c.Project.Name,
		Description: c.Project.Description,
		Module:      c.Project.Module,
		Go:          c.Project.Go,
		ModuleOnly:  len(c.EntryPoints) == 0 && !slices.ContainsFunc(c.Evidence, func(e config.Evidence) bool { return e.Field == "package" }),
		Stack:       stack(c, mode),
		Layout:      layout(c, mode),
		Commands:    commandViews(c.Commands),
		Env:         envNames(c.Commands),
		Settings:    settingsGuidance(c, mode),
		Tooling:     toolingGuidance(c),
	}
	out, err := execute("base/AGENTS.md.tmpl", g)
	if err != nil {
		return nil, fmt.Errorf("render AGENTS.md: %w", err)
	}
	return out, nil
}

func label(value string) string {
	if l, ok := labels[value]; ok {
		return l
	}
	return value + " (detected; Rubric does not generate integrations for it)"
}

func stack(c config.Config, mode string) []string {
	f := c.Features
	var out []string
	add := func(name, value string) {
		if value != "" && value != "none" {
			out = append(out, name+": "+label(value))
		}
	}
	add("HTTP", f.HTTP)
	add("Database", f.Database)
	if f.Database != "none" {
		add("Database access", f.Access)
	}
	add("CLI", f.CLI)
	add("TUI", f.TUI)
	if f.Config != "stdlib" || configPackage(c) && generated(c, mode, "internal/config") {
		add("Configuration", f.Config)
	}
	add("Web UI", f.Web)
	add("End-to-end tests", f.E2E)
	return out
}

func configPackage(c config.Config) bool {
	f := c.Features
	return f.HTTP != "none" || f.CLI != "none" || f.TUI != "none" || f.Database != "none" || f.Config == "viper"
}

func hasCommand(c config.Config, name string) bool {
	return slices.ContainsFunc(c.Commands, func(cmd config.Command) bool { return cmd.Name == name })
}

func hasPackage(c config.Config, dir string) bool {
	return slices.ContainsFunc(c.Evidence, func(e config.Evidence) bool { return e.Field == "package" && e.Value == dir })
}

func generated(c config.Config, mode, dir string) bool {
	return mode == "new" || hasPackage(c, dir)
}

func hasEntryPoint(c config.Config, mode, dir string) bool {
	return mode == "new" || slices.ContainsFunc(c.EntryPoints, func(ep config.EntryPoint) bool { return ep.Dir == dir })
}

func layout(c config.Config, mode string) []string {
	var out []string
	known := map[string]bool{}
	for _, e := range executables {
		if e.selected(c.Features) && hasEntryPoint(c, mode, e.dir) && generated(c, mode, e.pkg) {
			known[e.dir] = true
			out = append(out,
				fmt.Sprintf("`%s`: %s executable wiring, startup, and shutdown only", e.dir, e.name),
				fmt.Sprintf("`%s`: %s behavior and its tests", e.pkg, e.name))
		}
	}
	if c.Features.Database != "none" && generated(c, mode, "internal/store") {
		out = append(out, databaseGuidance(c)...)
	}
	if f := c.Features; f.Web == "htmx" && generated(c, mode, "internal/web") {
		line := "`internal/web`: templ views, htmx handlers, bundled htmx and Alpine.js, and their tests"
		if hasCommand(c, "generate-templ") {
			line += "; edit `views.templ`, then run generate-templ"
		}
		out = append(out, line)
	}
	if c.Features.E2E == "playwright" && hasCommand(c, "test-e2e") {
		out = append(out, "`e2e`: Playwright browser tests behind the `e2e` build tag; run setup-e2e once, then test-e2e")
	}
	if configPackage(c) && generated(c, mode, "internal/config") {
		out = append(out, "`internal/config`: runtime settings loading and its tests")
	}
	if len(known) == 0 && c.Project.Starter == "runnable" && hasEntryPoint(c, mode, ".") {
		known["."] = true
		out = append(out, "`main.go`: minimal entry point; move behavior into a tested package as it grows")
	}
	if mode != "new" {
		var extra []string
		for _, ep := range c.EntryPoints {
			if !known[ep.Dir] {
				extra = append(extra, fmt.Sprintf("`%s`: executable entry point %s", ep.Dir, ep.Name))
			}
		}
		out = append(extra, out...)
	}
	return out
}

func commandViews(cmds []config.Command) []commandView {
	out := make([]commandView, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, commandView{Name: c.Name, Line: shellLine(c.Argv), Dir: c.Dir, Env: codeList(c.Env)})
	}
	return out
}

func envNames(cmds []config.Command) string {
	var names []string
	for _, c := range cmds {
		names = append(names, c.Env...)
	}
	slices.Sort(names)
	return codeList(slices.Compact(names))
}

func codeList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "`" + n + "`"
	}
	return strings.Join(quoted, ", ")
}

func shellLine(argv []string) string {
	words := make([]string, len(argv))
	for i, a := range argv {
		if plainWord.MatchString(a) {
			words[i] = a
		} else {
			words[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(words, " ")
}
