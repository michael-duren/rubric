package config

import (
	"strings"
	"testing"
)

func TestExplicitDisabledValuesWin(t *testing.T) {
	base := Defaults()
	base.Tooling.Lint = true
	base.Features.HTTP = "chi"
	base.Commands = []Command{{Name: "run", Dir: ".", Argv: []string{"go", "run", "."}}}
	got, err := Resolve(base, Patch{
		"tooling.lint": false, "features.http": "none", "commands": []Command{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Tooling.Lint || got.Features.HTTP != "none" || len(got.Commands) != 0 {
		t.Fatalf("explicit disabled settings lost: %+v", got)
	}
}

func TestExplicitDecodedValuesOverrideSaved(t *testing.T) {
	saved, err := Decode([]byte("tooling:\n  lint: true\n  makefile: true\ncommands:\n  - name: run\n    dir: .\n    argv: [go, run, .]\n"))
	if err != nil {
		t.Fatal(err)
	}
	input, err := Decode([]byte("tooling:\n  lint: false\ncommands: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(Defaults(), saved.Values, input.Values)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tooling.Lint || !got.Tooling.Makefile || len(got.Commands) != 0 {
		t.Fatalf("precedence lost: %+v", got)
	}
}

func TestExplicitLaterLayerWinsAndAbsentKeysInherit(t *testing.T) {
	got, err := Resolve(Defaults(),
		Patch{"project.module": "example.com/a", "project.description": "first"},
		Patch{"project.description": "second"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.Project.Module != "example.com/a" || got.Project.Description != "second" {
		t.Fatalf("got %+v", got.Project)
	}
	if got.Project.Name != "a" {
		t.Fatalf("name = %q, want derived last segment", got.Project.Name)
	}
}

func TestExplicitDatabaseAccessRules(t *testing.T) {
	tests := []struct {
		name    string
		base    func(*Config)
		patches []Patch
		access  string
		wantErr string
	}{
		{name: "database defaults access to sql", patches: []Patch{{"features.database": "sqlite"}}, access: "sql"},
		{name: "explicit sqlc kept", patches: []Patch{{"features.database": "postgres", "features.access": "sqlc"}}, access: "sqlc"},
		{name: "explicit none with database", patches: []Patch{{"features.database": "sqlite", "features.access": "none"}}, wantErr: "features.access"},
		{name: "access without database", patches: []Patch{{"features.access": "sql"}}, wantErr: "features.access"},
		{
			name:    "disabling database clears inherited access",
			base:    func(c *Config) { c.Features.Database, c.Features.Access = "sqlite", "sqlc" },
			patches: []Patch{{"features.database": "none"}},
			access:  "none",
		},
		{
			name:    "disabling database clears lower-layer access",
			patches: []Patch{{"features.database": "sqlite", "features.access": "sqlc"}, {"features.database": "none"}},
			access:  "none",
		},
		{
			name:    "explicit incompatible access in higher layer",
			patches: []Patch{{"features.database": "none"}, {"features.access": "sqlc"}},
			wantErr: "features.access",
		},
		{
			name:    "saved none access upgraded when database enabled later",
			patches: []Patch{{"features.database": "none", "features.access": "none"}, {"features.database": "sqlite"}},
			access:  "sql",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := Defaults()
			if tt.base != nil {
				tt.base(&base)
			}
			got, err := Resolve(base, tt.patches...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Features.Access != tt.access {
				t.Fatalf("access = %q, want %q", got.Features.Access, tt.access)
			}
		})
	}
}

func TestExplicitStarterNormalizedByExecutables(t *testing.T) {
	got, err := Resolve(Defaults(), Patch{"project.starter": "runnable", "features.cli": "flag"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Project.Starter != "module" {
		t.Fatalf("starter = %q", got.Project.Starter)
	}
	got, err = Resolve(Defaults(), Patch{"project.starter": "runnable", "features.database": "sqlite", "features.config": "viper"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Project.Starter != "runnable" {
		t.Fatalf("library-only starter = %q", got.Project.Starter)
	}
}

func TestExplicitPatchRejectsWrongTypesAndUnknownKeys(t *testing.T) {
	for _, p := range []Patch{
		{"tooling.lint": "yes"},
		{"features.http": true},
		{"commands": []string{"go test"}},
		{"nope": 1},
		{"tooling": map[string]any{}},
	} {
		if _, err := Resolve(Defaults(), p); err == nil {
			t.Fatalf("Resolve(%v) succeeded", p)
		}
	}
}

func TestYAMLDecodeRejectsMalformedDocuments(t *testing.T) {
	tests := []struct {
		name, doc, want string
	}{
		{"wrong bool type", "tooling:\n  lint: \"yes\"\n", "tooling.lint"},
		{"wrong int type", "schema: one\n", "schema"},
		{"wrong list type", "commands: run\n", "commands"},
		{"duplicate key", "tooling:\n  lint: true\n  lint: false\n", "duplicate"},
		{"duplicate top-level key", "schema: 1\nschema: 1\n", "duplicate"},
		{"second document", "schema: 1\n---\nschema: 1\n", "document"},
		{"alias cycle", "commands: &a\n  - name: x\n    argv: *a\n", "cycle"},
		{"unknown key", "project:\n  modul: example.com/x\n", "project.modul"},
		{"unknown item key", "entry_points:\n  - name: x\n    path: y\n", "entry_points"},
		{"top-level not mapping", "- a\n", "mapping"},
		{"null value", "features:\n  http: ~\n", "features.http"},
		{"syntax", "tooling: [\n", "yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode([]byte(tt.doc))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want mention of %q", err, tt.want)
			}
		})
	}
}

func TestYAMLDecodeAcceptsNonCyclicAliases(t *testing.T) {
	doc, err := Decode([]byte("commands:\n  - name: test\n    dir: .\n    argv: &argv [go, test, ./...]\n  - name: vet\n    dir: .\n    argv: *argv\n"))
	if err != nil {
		t.Fatal(err)
	}
	cmds := doc.Values["commands"].([]Command)
	if len(cmds) != 2 || strings.Join(cmds[1].Argv, " ") != "go test ./..." {
		t.Fatalf("commands = %+v", cmds)
	}
}

func TestYAMLDecodeEmptyDocument(t *testing.T) {
	for _, in := range []string{"", "# only a comment\n"} {
		doc, err := Decode([]byte(in))
		if err != nil {
			t.Fatalf("Decode(%q): %v", in, err)
		}
		if len(doc.Values) != 0 {
			t.Fatalf("values = %v", doc.Values)
		}
	}
}

func TestYAMLRoundTripPreservesComments(t *testing.T) {
	in := "# Rubric project settings\nschema: 1\nproject:\n  module: example.com/app # module path\n  name: app\n  description: \"Façade ✓ service\"\n  go: 1.26.7\n  starter: module\nfeatures:\n  # choose a router\n  http: none # none for now\n  database: none\n  access: none\n  cli: none\n  tui: none\n  config: stdlib\n# trailing notes\n"
	doc, err := Decode([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Resolve(Defaults(), doc.Values, Patch{"features.http": "chi"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Description != "Façade ✓ service" {
		t.Fatalf("description = %q", cfg.Project.Description)
	}
	out, err := Encode(doc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		"# Rubric project settings", "# module path", "# choose a router", "# none for now", "# trailing notes",
		"http: chi", "Façade ✓ service", "tooling:", "lint: false", "entry_points: []",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(in, "http: chi") || !strings.Contains(in, "http: none") {
		t.Fatal("input mutated")
	}
	again, err := Decode(out)
	if err != nil {
		t.Fatalf("re-decode: %v\n%s", err, out)
	}
	back, err := Resolve(Defaults(), again.Values)
	if err != nil {
		t.Fatal(err)
	}
	if back.Features.HTTP != "chi" || back.Project.Module != "example.com/app" {
		t.Fatalf("round trip lost values: %+v", back)
	}
}

func TestYAMLEncodeDoesNotMutateDocument(t *testing.T) {
	doc, err := Decode([]byte("features:\n  http: none\n"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Defaults()
	cfg.Features.HTTP = "chi"
	if _, err := Encode(doc, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := Encode(doc, Defaults())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "http: none") {
		t.Fatalf("document mutated by earlier Encode:\n%s", out)
	}
}

func TestYAMLEncodeFreshDocumentIsPortable(t *testing.T) {
	dir := t.TempDir()
	cfg := Defaults()
	cfg.Project.Module = "example.com/app"
	cfg.Features.Database = "sqlite"
	cfg.Features.Access = "sql"
	out, err := Encode(Document{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), dir) || strings.Contains(string(out), "/tmp") {
		t.Fatalf("absolute path serialized:\n%s", out)
	}
	for _, want := range []string{"schema: 1", "database: sqlite", "access: sql", "style: rubric-go-v1", "template: 1", "version: dev"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	doc, err := Decode(out)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Resolve(Config{}, doc.Values)
	if err != nil {
		t.Fatal(err)
	}
	if back.Schema != 1 || back.Generator.Go != GoBaseline || back.Project.Module != "example.com/app" {
		t.Fatalf("fresh round trip: %+v", back)
	}
}

func TestYAMLEncodeIsDeterministic(t *testing.T) {
	cfg := Defaults()
	cfg.Generator.Dependencies = map[string]string{"b": "v2", "a": "v1", "c": "v3"}
	first, err := Encode(Document{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		next, err := Encode(Document{}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if string(next) != string(first) {
			t.Fatal("encoding not deterministic")
		}
	}
}

func validNew() Config {
	cfg := Defaults()
	cfg.Project.Module = "example.com/app"
	cfg.Project.Name = "app"
	return cfg
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		edit    func(*Config)
		wantErr string
	}{
		{name: "valid new", mode: "new"},
		{name: "local module name", mode: "new", edit: func(c *Config) { c.Project.Module = "app" }},
		{name: "local nested module", mode: "new", edit: func(c *Config) { c.Project.Module = "tools/app" }},
		{name: "web with http", mode: "new", edit: func(c *Config) { c.Features.HTTP, c.Features.Web, c.Features.E2E = "chi", "htmx", "playwright" }},
		{name: "web without http", mode: "new", edit: func(c *Config) { c.Features.Web = "htmx" }, wantErr: "features.web"},
		{name: "e2e without web", mode: "new", edit: func(c *Config) { c.Features.HTTP, c.Features.E2E = "chi", "playwright" }, wantErr: "features.e2e"},
		{name: "unknown web", mode: "new", edit: func(c *Config) { c.Features.HTTP, c.Features.Web = "chi", "react" }, wantErr: "features.web"},
		{name: "long name", mode: "new", edit: func(c *Config) { c.Project.Name = strings.Repeat("n", 65) }, wantErr: "project.name"},
		{name: "unicode description", mode: "new", edit: func(c *Config) { c.Project.Description = "Überprüfung — 検証 ✓" }},
		{name: "schema 2", mode: "new", edit: func(c *Config) { c.Schema = 2 }, wantErr: "schema"},
		{name: "empty module new", mode: "new", edit: func(c *Config) { c.Project.Module = "" }, wantErr: "project.module"},
		{name: "empty module existing", mode: "existing", edit: func(c *Config) { c.Project.Module = "" }, wantErr: "project.module"},
		{name: "module with space", mode: "new", edit: func(c *Config) { c.Project.Module = "example.com/my app" }, wantErr: "project.module"},
		{name: "module invalid char", mode: "new", edit: func(c *Config) { c.Project.Module = "example.com/a$b" }, wantErr: "project.module"},
		{name: "module newline", mode: "new", edit: func(c *Config) { c.Project.Module = "example.com/a\nb" }, wantErr: "project.module"},
		{name: "unknown http new", mode: "new", edit: func(c *Config) { c.Features.HTTP = "gin" }, wantErr: "features.http"},
		{name: "unknown http existing allowed", mode: "existing", edit: func(c *Config) { c.Features.HTTP = "gin" }},
		{name: "unknown starter", mode: "new", edit: func(c *Config) { c.Project.Starter = "full" }, wantErr: "project.starter"},
		{name: "access without database", mode: "new", edit: func(c *Config) { c.Features.Access = "sql" }, wantErr: "features.access"},
		{name: "database without access", mode: "new", edit: func(c *Config) { c.Features.Database = "sqlite" }, wantErr: "features.access"},
		{name: "runnable with executable", mode: "new", edit: func(c *Config) { c.Project.Starter = "runnable"; c.Features.CLI = "cobra" }},
		{name: "go mismatch new", mode: "new", edit: func(c *Config) { c.Project.Go = "1.25.0" }, wantErr: "project.go"},
		{name: "older go existing preserved", mode: "existing", edit: func(c *Config) { c.Project.Go = "1.22" }},
		{name: "malformed go existing", mode: "existing", edit: func(c *Config) { c.Project.Go = "banana" }, wantErr: "project.go"},
		{name: "bad mode", mode: "auto", wantErr: "mode"},
		{name: "wrong style", mode: "new", edit: func(c *Config) { c.Style = "other" }, wantErr: "style"},
		{name: "entry point absolute", mode: "new", edit: func(c *Config) { c.EntryPoints = []EntryPoint{{Name: "x", Dir: "/abs"}} }, wantErr: "entry_points"},
		{name: "entry point traversal", mode: "new", edit: func(c *Config) { c.EntryPoints = []EntryPoint{{Name: "x", Dir: "cmd/../../x"}} }, wantErr: "entry_points"},
		{name: "entry point backslash", mode: "new", edit: func(c *Config) { c.EntryPoints = []EntryPoint{{Name: "x", Dir: `cmd\x`}} }, wantErr: "entry_points"},
		{name: "entry point with spaces ok", mode: "new", edit: func(c *Config) { c.EntryPoints = []EntryPoint{{Name: "x", Dir: "cmd/my $tool `q`"}} }},
		{name: "command no argv", mode: "new", edit: func(c *Config) { c.Commands = []Command{{Name: "t", Dir: "."}} }, wantErr: "commands"},
		{name: "command nul argv", mode: "new", edit: func(c *Config) { c.Commands = []Command{{Name: "t", Dir: ".", Argv: []string{"go\x00"}}} }, wantErr: "commands"},
		{name: "command bad env", mode: "new", edit: func(c *Config) {
			c.Commands = []Command{{Name: "t", Dir: ".", Argv: []string{"go"}, Env: []string{"A=b"}}}
		}, wantErr: "commands"},
		{name: "command ok", mode: "new", edit: func(c *Config) {
			c.Commands = []Command{{Name: "t", Dir: ".", Argv: []string{"go", "test", "./..."}, Env: []string{"DATABASE_URL"}}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validNew()
			if tt.edit != nil {
				tt.edit(&cfg)
			}
			err := Validate(cfg, tt.mode)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want mention of %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDefaultsNeedOnlyModule(t *testing.T) {
	cfg, err := Resolve(Defaults(), Patch{"project.module": "example.com/svc"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(cfg, "new"); err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Starter != "runnable" || cfg.Tooling != (Tooling{}) || cfg.Features.Config != "stdlib" {
		t.Fatalf("defaults: %+v", cfg)
	}
}
