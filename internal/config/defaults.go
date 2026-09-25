package config

const (
	// SchemaVersion is the only rubric.yaml schema this build understands.
	SchemaVersion = 1
	// StyleID identifies the style policy applied to generated code.
	StyleID = "rubric-go-v1"
	// TemplateRevision identifies the bundled template set.
	TemplateRevision = 1
	// GoBaseline is the Go version Rubric generates and tests against.
	GoBaseline = "1.26.7"
)

// GeneratorVersion is the Rubric version recorded in generated configuration; releases override it.
var GeneratorVersion = "dev"

// Defaults returns the documented defaults; nil entry points and commands mean "derive them".
func Defaults() Config {
	return Config{
		Schema: SchemaVersion,
		Project: Project{
			Go:      GoBaseline,
			Starter: "runnable",
		},
		Features: Features{
			HTTP:     "none",
			Database: "none",
			Access:   "none",
			CLI:      "none",
			TUI:      "none",
			Config:   "stdlib",
			Web:      "none",
			E2E:      "none",
		},
		Tooling:  Tooling{Skills: []string{}},
		Evidence: []Evidence{},
		Generator: Generator{
			Version:      GeneratorVersion,
			Template:     TemplateRevision,
			Go:           GoBaseline,
			Dependencies: map[string]string{},
			Tools:        map[string]string{},
		},
		Style: StyleID,
	}
}
