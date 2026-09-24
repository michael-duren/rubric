package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/pflag"

	"github.com/michael-duren/go-skills/internal/config"
)

type options struct {
	mode, format, configPath   string
	nonInteractive, dryRun     bool
	entryPoints, commands      []string
	clearEntries, clearCommand bool
}

var stringFlags = []struct{ name, key, usage string }{
	{"module", "project.module", "module path for a new project"},
	{"name", "project.name", "display name (defaults to the last module path element)"},
	{"description", "project.description", "project description"},
	{"starter", "project.starter", "starter without executables: module or runnable"},
	{"http", "features.http", "HTTP server: none, nethttp, or chi"},
	{"database", "features.database", "database: none, sqlite, or postgres"},
	{"access", "features.access", "database access: sql or sqlc"},
	{"cli", "features.cli", "CLI executable: none, flag, or cobra"},
	{"tui", "features.tui", "terminal UI: none or bubbletea"},
	{"app-config", "features.config", "runtime configuration: stdlib or viper"},
	{"web", "features.web", "web UI: none or htmx (htmx + Alpine.js + templ; requires --http)"},
	{"e2e", "features.e2e", "browser tests: none or playwright (requires --web htmx)"},
}

var boolFlags = []struct{ name, key, usage string }{
	{"skills", "tooling.skills", "install repo-local agent skills"},
	{"lint", "tooling.lint", "add golangci-lint configuration and the style analyzer"},
	{"makefile", "tooling.makefile", "add a Makefile for the configured commands"},
	{"actions", "tooling.actions", "add a GitHub Actions workflow"},
}

func register(fs *pflag.FlagSet, o *options) {
	fs.StringVar(&o.mode, "mode", "auto", "target mode: auto, new, or existing")
	for _, f := range stringFlags {
		fs.String(f.name, "", f.usage)
	}
	for _, f := range boolFlags {
		fs.Bool(f.name, false, f.usage)
	}
	fs.StringArrayVar(&o.entryPoints, "entry-point", nil, `existing entry point as JSON, e.g. {"name":"api","dir":"cmd/api"} (repeatable)`)
	fs.StringArrayVar(&o.commands, "command", nil,
		`command as JSON, e.g. {"name":"test","dir":".","argv":["go","test","./..."],"env":["NAME"]} (repeatable)`)
	fs.BoolVar(&o.clearEntries, "clear-entry-points", false, "record an explicitly empty entry point list")
	fs.BoolVar(&o.clearCommand, "clear-commands", false, "record an explicitly empty command list")
	fs.StringVar(&o.configPath, "config", "", "read a Rubric configuration document as input")
	fs.BoolVar(&o.nonInteractive, "non-interactive", false, "never prompt or start the wizard")
	fs.BoolVar(&o.dryRun, "dry-run", false, "validate and preview without writing files")
	fs.StringVar(&o.format, "format", "text", "report format: text or json (json implies --non-interactive)")
}

func overrides(fs *pflag.FlagSet, o options) (config.Patch, error) {
	p := config.Patch{}
	for _, f := range stringFlags {
		if fs.Changed(f.name) {
			v, _ := fs.GetString(f.name)
			p[f.key] = v
		}
	}
	for _, f := range boolFlags {
		if fs.Changed(f.name) {
			v, _ := fs.GetBool(f.name)
			p[f.key] = v
		}
	}
	if o.clearEntries && len(o.entryPoints) > 0 {
		return nil, errors.New("--clear-entry-points cannot be combined with --entry-point")
	}
	if o.clearCommand && len(o.commands) > 0 {
		return nil, errors.New("--clear-commands cannot be combined with --command")
	}
	if o.clearEntries || len(o.entryPoints) > 0 {
		list := []config.EntryPoint{}
		for _, raw := range o.entryPoints {
			var ep config.EntryPoint
			if err := strictJSON(raw, &ep); err != nil {
				return nil, fmt.Errorf("--entry-point %s: %w", raw, err)
			}
			list = append(list, ep)
		}
		p["entry_points"] = list
	}
	if o.clearCommand || len(o.commands) > 0 {
		list := []config.Command{}
		for _, raw := range o.commands {
			var cmd config.Command
			if err := strictJSON(raw, &cmd); err != nil {
				return nil, fmt.Errorf("--command %s: %w", raw, err)
			}
			if cmd.Dir == "" {
				cmd.Dir = "."
			}
			if cmd.Env == nil {
				cmd.Env = []string{}
			}
			list = append(list, cmd)
		}
		p["commands"] = list
	}
	return p, nil
}

func strictJSON(raw string, v any) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing data after JSON value")
	}
	return nil
}
