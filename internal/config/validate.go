package config

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

const maxNameRunes = 64

var (
	starters  = []string{"module", "runnable"}
	httpOpts  = []string{"none", "nethttp", "chi"}
	databases = []string{"none", "sqlite", "postgres"}
	accesses  = []string{"none", "sql", "sqlc"}
	clis      = []string{"none", "flag", "cobra"}
	tuis      = []string{"none", "bubbletea"}
	configs   = []string{"stdlib", "viper"}
	envName   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Validate checks a resolved configuration for the resolved mode, new or existing.
func Validate(cfg Config, mode string) error {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if mode != "new" && mode != "existing" {
		fail("mode: %q must be new or existing", mode)
	}
	if cfg.Schema != SchemaVersion {
		fail("schema: unsupported version %d (supported: %d)", cfg.Schema, SchemaVersion)
	}
	if cfg.Style != StyleID {
		fail("style: unsupported policy %q (supported: %s)", cfg.Style, StyleID)
	}
	if cfg.Project.Module == "" {
		fail("project.module: required")
	} else if err := module.CheckImportPath(cfg.Project.Module); err != nil {
		fail("project.module: %v", err)
	}
	if hasControl(cfg.Project.Name) {
		fail("project.name: must not contain control characters")
	}
	if n := utf8.RuneCountInString(cfg.Project.Name); n > maxNameRunes {
		fail("project.name: %d characters; the limit is %d", n, maxNameRunes)
	}
	if hasControl(cfg.Project.Description) {
		fail("project.description: must be a single line without control characters")
	}
	for key, value := range map[string]string{
		"project.name": cfg.Project.Name, "project.description": cfg.Project.Description,
		"features.http": cfg.Features.HTTP, "features.database": cfg.Features.Database, "features.access": cfg.Features.Access,
		"features.cli": cfg.Features.CLI, "features.tui": cfg.Features.TUI, "features.config": cfg.Features.Config,
	} {
		if hasMarker(value) {
			fail("%s: must not contain Rubric guidance markers", key)
		}
		if strings.HasPrefix(key, "features.") && hasControl(value) {
			fail("%s: must not contain control characters", key)
		}
	}
	switch {
	case mode == "new" && cfg.Project.Go != GoBaseline:
		fail("project.go: new projects use Go %s, got %q", GoBaseline, cfg.Project.Go)
	case !modfile.GoVersionRE.MatchString(cfg.Project.Go):
		fail("project.go: %q is not a Go version", cfg.Project.Go)
	}
	if !slices.Contains(starters, cfg.Project.Starter) {
		fail("project.starter: %q must be one of %s", cfg.Project.Starter, strings.Join(starters, ", "))
	} else if cfg.Project.Starter == "runnable" && hasExecutable(cfg.Features) {
		fail("project.starter: runnable starter cannot be combined with HTTP, CLI, or TUI executables")
	}
	if mode == "new" {
		f := cfg.Features
		enum(fail, "features.http", f.HTTP, httpOpts)
		enum(fail, "features.database", f.Database, databases)
		enum(fail, "features.access", f.Access, accesses)
		enum(fail, "features.cli", f.CLI, clis)
		enum(fail, "features.tui", f.TUI, tuis)
		enum(fail, "features.config", f.Config, configs)
		if f.Database == "none" && f.Access != "none" {
			fail("features.access: %q requires a database", f.Access)
		}
		if f.Database != "none" && f.Access == "none" {
			fail("features.access: must be sql or sqlc when database is %s", f.Database)
		}
	}
	for i, ep := range cfg.EntryPoints {
		if hasMarker(ep.Name) || hasMarker(ep.Dir) {
			fail("entry_points[%d]: must not contain Rubric guidance markers", i)
		}
		if ep.Name == "" || hasControl(ep.Name) {
			fail("entry_points[%d].name: required without control characters", i)
		}
		if err := checkDir(ep.Dir); err != nil {
			fail("entry_points[%d].dir: %v", i, err)
		}
	}
	for i, cmd := range cfg.Commands {
		if hasMarker(cmd.Name) || hasMarker(cmd.Dir) || slices.ContainsFunc(cmd.Argv, hasMarker) {
			fail("commands[%d]: must not contain Rubric guidance markers", i)
		}
		if cmd.Name == "" || hasControl(cmd.Name) {
			fail("commands[%d].name: required without control characters", i)
		}
		if err := checkDir(cmd.Dir); err != nil {
			fail("commands[%d].dir: %v", i, err)
		}
		if len(cmd.Argv) == 0 || cmd.Argv[0] == "" {
			fail("commands[%d].argv: program required", i)
		}
		if slices.ContainsFunc(cmd.Argv, hasControl) {
			fail("commands[%d].argv: arguments must not contain NUL or newlines", i)
		}
		for _, name := range cmd.Env {
			if !envName.MatchString(name) {
				fail("commands[%d].env: %q is not an environment variable name", i, name)
			}
		}
	}
	return errors.Join(errs...)
}

func enum(fail func(string, ...any), key, value string, allowed []string) {
	if !slices.Contains(allowed, value) {
		fail("%s: %q must be one of %s", key, value, strings.Join(allowed, ", "))
	}
}

func hasControl(s string) bool {
	return strings.ContainsAny(s, "\x00\n\r")
}

func checkDir(dir string) error {
	switch {
	case dir == "":
		return errors.New("required")
	case hasControl(dir):
		return errors.New("must not contain NUL or newlines")
	case strings.Contains(dir, `\`):
		return errors.New("must use forward slashes")
	case path.IsAbs(dir) || (len(dir) > 1 && dir[1] == ':'):
		return errors.New("must be relative to the module root")
	case path.Clean(dir) != dir:
		return fmt.Errorf("must be clean (%q)", path.Clean(dir))
	case dir == ".." || strings.HasPrefix(dir, "../"):
		return errors.New("must stay inside the module")
	}
	return nil
}

func hasMarker(s string) bool {
	return strings.Contains(s, "rubric:begin") || strings.Contains(s, "rubric:end")
}
