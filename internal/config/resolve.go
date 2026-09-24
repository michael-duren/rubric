package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

type kind int

const (
	kindSection kind = iota
	kindString
	kindBool
	kindInt
	kindStringMap
	kindEntryPoints
	kindCommands
	kindEvidence
)

type field struct {
	kind kind
	set  func(*Config, any) error
}

var fields = map[string]field{
	"schema":                 intField(func(c *Config) *int { return &c.Schema }),
	"project":                {kind: kindSection},
	"project.module":         stringField(func(c *Config) *string { return &c.Project.Module }),
	"project.name":           stringField(func(c *Config) *string { return &c.Project.Name }),
	"project.description":    stringField(func(c *Config) *string { return &c.Project.Description }),
	"project.go":             stringField(func(c *Config) *string { return &c.Project.Go }),
	"project.starter":        stringField(func(c *Config) *string { return &c.Project.Starter }),
	"features":               {kind: kindSection},
	"features.http":          stringField(func(c *Config) *string { return &c.Features.HTTP }),
	"features.database":      stringField(func(c *Config) *string { return &c.Features.Database }),
	"features.access":        stringField(func(c *Config) *string { return &c.Features.Access }),
	"features.cli":           stringField(func(c *Config) *string { return &c.Features.CLI }),
	"features.tui":           stringField(func(c *Config) *string { return &c.Features.TUI }),
	"features.config":        stringField(func(c *Config) *string { return &c.Features.Config }),
	"tooling":                {kind: kindSection},
	"tooling.skills":         boolField(func(c *Config) *bool { return &c.Tooling.Skills }),
	"tooling.lint":           boolField(func(c *Config) *bool { return &c.Tooling.Lint }),
	"tooling.makefile":       boolField(func(c *Config) *bool { return &c.Tooling.Makefile }),
	"tooling.actions":        boolField(func(c *Config) *bool { return &c.Tooling.Actions }),
	"entry_points":           listField(kindEntryPoints, func(c *Config) *[]EntryPoint { return &c.EntryPoints }),
	"commands":               listField(kindCommands, func(c *Config) *[]Command { return &c.Commands }),
	"evidence":               listField(kindEvidence, func(c *Config) *[]Evidence { return &c.Evidence }),
	"generator":              {kind: kindSection},
	"generator.version":      stringField(func(c *Config) *string { return &c.Generator.Version }),
	"generator.template":     intField(func(c *Config) *int { return &c.Generator.Template }),
	"generator.go":           stringField(func(c *Config) *string { return &c.Generator.Go }),
	"generator.dependencies": mapField(func(c *Config) *map[string]string { return &c.Generator.Dependencies }),
	"generator.tools":        mapField(func(c *Config) *map[string]string { return &c.Generator.Tools }),
	"style":                  stringField(func(c *Config) *string { return &c.Style }),
}

func stringField(at func(*Config) *string) field {
	return field{kind: kindString, set: func(c *Config, v any) error { return assign(at(c), v, "string") }}
}

func boolField(at func(*Config) *bool) field {
	return field{kind: kindBool, set: func(c *Config, v any) error { return assign(at(c), v, "boolean") }}
}

func intField(at func(*Config) *int) field {
	return field{kind: kindInt, set: func(c *Config, v any) error { return assign(at(c), v, "integer") }}
}

func mapField(at func(*Config) *map[string]string) field {
	return field{kind: kindStringMap, set: func(c *Config, v any) error {
		m, ok := v.(map[string]string)
		if !ok {
			return fmt.Errorf("expected string map")
		}
		*at(c) = maps.Clone(m)
		if *at(c) == nil {
			*at(c) = map[string]string{}
		}
		return nil
	}}
}

func listField[T any](k kind, at func(*Config) *[]T) field {
	return field{kind: k, set: func(c *Config, v any) error {
		list, ok := v.([]T)
		if !ok {
			return fmt.Errorf("expected list of %T", *new(T))
		}
		*at(c) = cloneList(list)
		return nil
	}}
}

func assign[T any](dst *T, v any, name string) error {
	value, ok := v.(T)
	if !ok {
		return fmt.Errorf("expected %s", name)
	}
	*dst = value
	return nil
}

func cloneList[T any](list []T) []T {
	out := make([]T, len(list))
	copy(out, list)
	for i := range out {
		if cmd, ok := any(&out[i]).(*Command); ok {
			cmd.Argv = slices.Clone(cmd.Argv)
			cmd.Env = slices.Clone(cmd.Env)
		}
	}
	return out
}

// Resolve applies patches in increasing precedence to base; present keys win, including false, none, and empty lists.
func Resolve(base Config, patches ...Patch) (Config, error) {
	cfg := clone(base)
	accessLayer, databaseLayer := -1, -1
	for i, patch := range patches {
		for _, key := range slices.Sorted(maps.Keys(patch)) {
			f, ok := fields[key]
			if !ok || f.kind == kindSection {
				return Config{}, fmt.Errorf("%s: unknown setting", key)
			}
			if err := f.set(&cfg, patch[key]); err != nil {
				return Config{}, fmt.Errorf("%s: %w", key, err)
			}
		}
		if _, ok := patch["features.access"]; ok {
			accessLayer = i
		}
		if _, ok := patch["features.database"]; ok {
			databaseLayer = i
		}
	}
	if err := normalizeAccess(&cfg, accessLayer, databaseLayer); err != nil {
		return Config{}, err
	}
	if hasExecutable(cfg.Features) {
		cfg.Project.Starter = "module"
	}
	if cfg.Project.Name == "" && cfg.Project.Module != "" {
		cfg.Project.Name = cfg.Project.Module[strings.LastIndex(cfg.Project.Module, "/")+1:]
	}
	return cfg, nil
}

func normalizeAccess(cfg *Config, accessLayer, databaseLayer int) error {
	explicitOverDatabase := accessLayer >= 0 && accessLayer >= databaseLayer
	switch {
	case cfg.Features.Database == "none" && cfg.Features.Access != "none":
		if explicitOverDatabase {
			return fmt.Errorf("features.access: %q requires a database", cfg.Features.Access)
		}
		cfg.Features.Access = "none"
	case knownDatabase(cfg.Features.Database) && cfg.Features.Access == "none":
		if explicitOverDatabase {
			return fmt.Errorf("features.access: must be sql or sqlc when database is %s", cfg.Features.Database)
		}
		cfg.Features.Access = "sql"
	}
	return nil
}

func knownDatabase(name string) bool {
	return name == "sqlite" || name == "postgres"
}

func hasExecutable(f Features) bool {
	return enabled(f.HTTP) || enabled(f.CLI) || enabled(f.TUI)
}

func enabled(choice string) bool {
	return choice != "" && choice != "none"
}

func clone(c Config) Config {
	out := c
	out.EntryPoints = cloneList(c.EntryPoints)
	out.Commands = cloneList(c.Commands)
	out.Evidence = cloneList(c.Evidence)
	out.Generator.Dependencies = maps.Clone(c.Generator.Dependencies)
	out.Generator.Tools = maps.Clone(c.Generator.Tools)
	if out.Generator.Dependencies == nil {
		out.Generator.Dependencies = map[string]string{}
	}
	if out.Generator.Tools == nil {
		out.Generator.Tools = map[string]string{}
	}
	return out
}
