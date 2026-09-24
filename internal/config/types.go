package config

import "go.yaml.in/yaml/v3"

// Config is the versioned project model persisted in rubric.yaml.
type Config struct {
	Schema      int          `yaml:"schema" json:"schema"`
	Project     Project      `yaml:"project" json:"project"`
	Features    Features     `yaml:"features" json:"features"`
	Tooling     Tooling      `yaml:"tooling" json:"tooling"`
	EntryPoints []EntryPoint `yaml:"entry_points" json:"entry_points"`
	Commands    []Command    `yaml:"commands" json:"commands"`
	Evidence    []Evidence   `yaml:"evidence" json:"evidence"`
	Generator   Generator    `yaml:"generator" json:"generator"`
	Style       string       `yaml:"style" json:"style"`
}

// Project identifies the module and how a new project starts.
type Project struct {
	Module      string `yaml:"module" json:"module"`
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Go          string `yaml:"go" json:"go"`
	Starter     string `yaml:"starter" json:"starter"`
}

// Features records the selected or detected application capabilities.
type Features struct {
	HTTP     string `yaml:"http" json:"http"`
	Database string `yaml:"database" json:"database"`
	Access   string `yaml:"access" json:"access"`
	CLI      string `yaml:"cli" json:"cli"`
	TUI      string `yaml:"tui" json:"tui"`
	Config   string `yaml:"config" json:"config"`
}

// Tooling records which optional development tooling is enabled.
type Tooling struct {
	Skills   bool `yaml:"skills" json:"skills"`
	Lint     bool `yaml:"lint" json:"lint"`
	Makefile bool `yaml:"makefile" json:"makefile"`
	Actions  bool `yaml:"actions" json:"actions"`
}

// EntryPoint names an executable package directory relative to the module root.
type EntryPoint struct {
	Name string `yaml:"name" json:"name"`
	Dir  string `yaml:"dir" json:"dir"`
}

// Command is an argument vector run from Dir; Env lists required variable names, never values.
type Command struct {
	Name string   `yaml:"name" json:"name"`
	Dir  string   `yaml:"dir" json:"dir"`
	Argv []string `yaml:"argv" json:"argv"`
	Env  []string `yaml:"env" json:"env"`
}

// Evidence explains where a detected value came from.
type Evidence struct {
	Field  string `yaml:"field" json:"field"`
	Value  string `yaml:"value" json:"value"`
	Source string `yaml:"source" json:"source"`
}

// Generator records the Rubric version, template revision, and pinned versions used.
type Generator struct {
	Version      string            `yaml:"version" json:"version"`
	Template     int               `yaml:"template" json:"template"`
	Go           string            `yaml:"go" json:"go"`
	Dependencies map[string]string `yaml:"dependencies" json:"dependencies"`
	Tools        map[string]string `yaml:"tools" json:"tools"`
}

// Patch holds explicitly present values keyed by dot-separated schema paths.
type Patch map[string]any

// Document is a decoded rubric.yaml syntax tree and the values it explicitly sets.
type Document struct {
	Node   *yaml.Node
	Values Patch
}
