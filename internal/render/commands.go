package render

import (
	"slices"

	"github.com/michael-duren/go-skills/internal/catalog"

	"github.com/michael-duren/go-skills/internal/config"
)

type executable struct {
	name, dir, pkg string
	selected       func(config.Features) bool
}

var executables = []executable{
	{"server", "cmd/server", "internal/httpserver", func(f config.Features) bool { return f.HTTP != "none" }},
	{"cli", "cmd/cli", "internal/cli", func(f config.Features) bool { return f.CLI != "none" }},
	{"tui", "cmd/tui", "internal/tui", func(f config.Features) bool { return f.TUI != "none" }},
}

// EntryPoints returns generated executables for new projects and the recorded ones for existing projects.
func EntryPoints(c config.Config, mode string) []config.EntryPoint {
	if mode != "new" {
		return slices.Clone(c.EntryPoints)
	}
	out := []config.EntryPoint{}
	for _, e := range executables {
		if e.selected(c.Features) {
			out = append(out, config.EntryPoint{Name: e.name, Dir: e.dir})
		}
	}
	if len(out) == 0 && c.Project.Starter == "runnable" {
		out = append(out, config.EntryPoint{Name: c.Project.Name, Dir: "."})
	}
	return out
}

// Commands merges explicit commands with the inventory derived from the project; explicit names win.
func Commands(c config.Config, mode string) []config.Command {
	var derived []config.Command
	if hasPackages(c, mode) {
		if mode == "new" {
			derived = append(derived, goCommand("setup", "mod", "tidy"))
		}
		derived = append(derived, goCommand("build", "build", "./..."), goCommand("test", "test", "./..."))
	}
	if mode == "new" && c.Features.Access == "sqlc" {
		derived = append(derived, config.Command{
			Name: "generate", Dir: ".", Argv: []string{"go", "run", catalog.Launcher("sqlc"), "generate"}, Env: []string{},
		})
	}
	if c.Tooling.Lint {
		derived = append(derived, config.Command{Name: "lint", Dir: ".", Argv: []string{"sh", checkScript, "lint"}, Env: []string{}})
	}
	postgres := mode == "new" && c.Features.Database == "postgres"
	if postgres {
		derived = append(derived, config.Command{
			Name: "test-integration", Dir: ".", Argv: []string{"go", "test", "-tags", "integration", "./internal/store/..."},
			Env: []string{"TEST_DATABASE_URL"},
		})
	}
	for _, ep := range EntryPoints(c, mode) {
		run := runCommand(runName(ep), runTarget(ep.Dir))
		if postgres {
			run.Env = []string{"APP_DATABASE_URL"}
		}
		derived = append(derived, run)
	}
	out := []config.Command{}
	for _, cmd := range derived {
		if i := slices.IndexFunc(c.Commands, func(e config.Command) bool { return e.Name == cmd.Name }); i >= 0 {
			cmd = c.Commands[i]
		}
		out = append(out, cmd)
	}
	for _, cmd := range c.Commands {
		if !slices.ContainsFunc(out, func(e config.Command) bool { return e.Name == cmd.Name }) {
			out = append(out, cmd)
		}
	}
	return out
}

func hasPackages(c config.Config, mode string) bool {
	f := c.Features
	if mode == "new" {
		return c.Project.Starter == "runnable" || f.HTTP != "none" || f.Database != "none" ||
			f.CLI != "none" || f.TUI != "none" || f.Config != "stdlib"
	}
	return len(c.EntryPoints) > 0 || f.HTTP != "none" || f.Database != "none" || f.CLI != "none" || f.TUI != "none"
}

func goCommand(name string, args ...string) config.Command {
	return config.Command{Name: name, Dir: ".", Argv: append([]string{"go"}, args...), Env: []string{}}
}

func runCommand(name, dir string) config.Command {
	return config.Command{Name: name, Dir: ".", Argv: []string{"go", "run", dir}, Env: []string{}}
}

func runName(ep config.EntryPoint) string {
	if ep.Dir == "." {
		return "run"
	}
	return "run-" + ep.Name
}

func runTarget(dir string) string {
	if dir == "." {
		return "."
	}
	return "./" + dir
}
