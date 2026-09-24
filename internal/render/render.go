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
}

type output struct {
	path     string
	template string
	kind     string
	mode     fs.FileMode
	when     func(config.Config, string) bool
}

var outputs = []output{
	{path: "go.mod", template: "base/go.mod.tmpl", kind: KindScaffold, when: isNew},
	{path: "main.go", template: "base/main.go.tmpl", kind: KindScaffold, when: rootMain},
}

var funcs = template.FuncMap{"quote": strconv.Quote}

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
	d := data{Config: cfg, Mode: mode}
	if mode == "new" {
		deps, err := catalog.Dependencies(cfg.Features)
		if err != nil {
			return nil, fmt.Errorf("render: %w", err)
		}
		for _, p := range slices.Sorted(maps.Keys(deps)) {
			d.Requires = append(d.Requires, require{Path: p, Version: deps[p]})
		}
	}
	var files []File
	for _, out := range outputs {
		if !out.when(cfg, mode) {
			continue
		}
		body, err := execute(out.template, d)
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
