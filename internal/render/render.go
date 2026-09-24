// Package render composes generated files from a normalized configuration without touching the filesystem.
package render

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"io/fs"
	"maps"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/michael-duren/go-skills/internal/catalog"
	"github.com/michael-duren/go-skills/internal/config"
)

//go:embed templates
var templates embed.FS

type require struct {
	Path, Version string
}

type data struct {
	Config   config.Config
	Mode     string
	Requires []require
	Commands []commandView
}

type output struct {
	path     string
	template string
	kind     string
	mode     fs.FileMode
	when     func(config.Config, string) bool
	build    func(config.Config, string) ([]byte, error)
}

var outputs = []output{
	{path: "go.mod", template: "base/go.mod.tmpl", kind: KindScaffold, when: isNew},
	{path: "main.go", template: "base/main.go.tmpl", kind: KindScaffold, when: rootMain},
	{path: "README.md", template: "base/README.md.tmpl", kind: KindScaffold, when: isNew},
	{path: "AGENTS.md", kind: KindGuidance, when: always, build: Instructions},
	{path: ".rubric/style.md", template: "base/style.md.tmpl", kind: KindManaged, when: always},
	{path: "rubric.yaml", kind: KindConfig, when: always, build: encodeConfig},
}

var funcs = template.FuncMap{"quote": strconv.Quote}

func always(config.Config, string) bool {
	return true
}

func encodeConfig(c config.Config, _ string) ([]byte, error) {
	return config.Encode(config.Document{}, c)
}

func isNew(_ config.Config, mode string) bool {
	return mode == "new"
}

func rootMain(c config.Config, mode string) bool {
	return mode == "new" && c.Project.Starter == "runnable"
}

// Files validates cfg for mode and renders every applicable file, sorted by path.
func Files(cfg config.Config, mode string) ([]File, error) {
	if err := config.Validate(cfg, mode); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	cfg, err := Normalize(cfg, mode)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	d := data{Config: cfg, Mode: mode, Commands: commandViews(cfg.Commands)}
	if mode == "new" {
		for _, p := range slices.Sorted(maps.Keys(cfg.Generator.Dependencies)) {
			d.Requires = append(d.Requires, require{Path: p, Version: cfg.Generator.Dependencies[p]})
		}
	}
	var files []File
	for _, out := range slices.Concat(outputs, httpOutputs) {
		if !out.when(cfg, mode) {
			continue
		}
		var body []byte
		var err error
		if out.build != nil {
			body, err = out.build(cfg, mode)
		} else {
			body, err = execute(out.template, d)
		}
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", out.path, err)
		}
		mode := out.mode
		if mode == 0 {
			mode = 0o644
		}
		files = append(files, File{Path: out.path, Data: body, Mode: mode, Kind: out.kind})
	}
	return finish(files)
}

// Normalize derives entry points, absent (nil) commands, and pinned dependencies so every artifact shares them.
func Normalize(cfg config.Config, mode string) (config.Config, error) {
	cfg.EntryPoints = EntryPoints(cfg, mode)
	if cfg.Commands == nil {
		cfg.Commands = Commands(cfg, mode)
	}
	if mode == "new" {
		deps, err := catalog.Dependencies(cfg.Features)
		if err != nil {
			return cfg, err
		}
		cfg.Generator.Dependencies = deps
	}
	return cfg, nil
}

func execute(name string, value any) ([]byte, error) {
	tmpl, err := template.New(name).Funcs(funcs).Option("missingkey=error").ParseFS(templates, "templates/"+name)
	if err != nil {
		return nil, err
	}
	tmpl = tmpl.Lookup(name[strings.LastIndex(name, "/")+1:])
	if strings.HasSuffix(name, ".go.tmpl") {
		return renderSource(tmpl, value)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderSource(tmpl *template.Template, value any) ([]byte, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, value); err != nil {
		return nil, err
	}
	return format.Source(buf.Bytes())
}
