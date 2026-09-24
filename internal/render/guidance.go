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
	Existing                      bool
	Stack, Layout                 []string
	Commands                      []commandView
	Env                           string
}

var labels = map[string]string{
	"nethttp":   "standard-library net/http",
	"chi":       "Chi router",
	"sqlite":    "SQLite (modernc.org/sqlite)",
	"postgres":  "PostgreSQL (pgx)",
	"sql":       "handwritten SQL with database/sql",
	"sqlc":      "sqlc-generated queries",
	"flag":      "standard-library flag",
	"cobra":     "Cobra",
	"bubbletea": "Bubble Tea v2",
	"viper":     "Viper",
	"stdlib":    "standard-library configuration",
}

var plainWord = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,-]+$`)

// Instructions renders the managed AGENTS.md section, markers included, for the given mode.
func Instructions(c config.Config, mode string) ([]byte, error) {
	g := guide{
		Name:        c.Project.Name,
		Description: c.Project.Description,
		Module:      c.Project.Module,
		Go:          c.Project.Go,
		Existing:    mode != "new",
		Stack:       stack(c, mode),
		Layout:      layout(c, mode),
		Commands:    commandViews(c.Commands),
		Env:         envNames(c.Commands),
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
	if f.Config != "stdlib" || mode == "new" && configPackage(c) {
		add("Configuration", f.Config)
	}
	return out
}

func configPackage(c config.Config) bool {
	f := c.Features
	return f.HTTP != "none" || f.CLI != "none" || f.TUI != "none" || f.Database != "none" || f.Config == "viper"
}

func layout(c config.Config, mode string) []string {
	var out []string
	if mode != "new" {
		for _, ep := range c.EntryPoints {
			out = append(out, fmt.Sprintf("`%s`: executable entry point %s", ep.Dir, ep.Name))
		}
		return out
	}
	for _, e := range executables {
		if e.selected(c.Features) {
			out = append(out,
				fmt.Sprintf("`%s`: %s executable wiring, startup, and shutdown only", e.dir, e.name),
				fmt.Sprintf("`%s`: %s behavior and its tests", e.pkg, e.name))
		}
	}
	if c.Features.Database != "none" {
		out = append(out, "`internal/store`: database access and its tests")
	}
	if configPackage(c) {
		out = append(out, "`internal/config`: runtime settings loading and its tests")
	}
	if len(out) == 0 && c.Project.Starter == "runnable" {
		out = append(out, "`main.go`: minimal entry point; move behavior into a tested package as it grows")
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
