package initialize

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/detect"
	"github.com/michael-duren/go-skills/internal/plan"
	"github.com/michael-duren/go-skills/internal/render"
)

func inputErr(format string, args ...any) error {
	return &InputError{Cause: fmt.Errorf(format, args...)}
}

// Prepare merges defaults, detection, saved settings, input, and overrides, then previews the result.
func Prepare(ctx context.Context, req Request) (plan.Plan, error) {
	if err := ctx.Err(); err != nil {
		return plan.Plan{}, err
	}
	root := target(req)
	facts, present, err := inspect(root)
	if err != nil {
		return plan.Plan{}, err
	}
	mode, err := resolveMode(req.Mode, facts, present)
	if err != nil {
		return plan.Plan{}, err
	}
	saved, err := readSaved(root)
	if err != nil {
		return plan.Plan{}, err
	}
	input, err := config.Decode(req.Input)
	if err != nil {
		return plan.Plan{}, inputErr("--config: %w", err)
	}
	overrides := req.Overrides
	if overrides == nil {
		overrides = config.Patch{}
	}
	var layers []config.Patch
	if mode == "existing" {
		layers = append(layers, detectionPatch(facts))
	}
	layers = append(layers, saved.Values, input.Values, overrides)
	cfg, err := config.Resolve(config.Defaults(), layers...)
	if err != nil {
		return plan.Plan{}, &InputError{Cause: err}
	}
	explicit := func(key string) bool {
		_, inInput := input.Values[key]
		_, inOverride := overrides[key]
		return inInput || inOverride
	}
	_, savedCommands := saved.Values["commands"]
	if mode == "existing" {
		previous, err := config.Resolve(config.Defaults(), detectionPatch(facts), saved.Values)
		if err != nil {
			return plan.Plan{}, &InputError{Cause: err}
		}
		if err := reconcileExisting(&cfg, previous, facts, explicit, savedCommands); err != nil {
			return plan.Plan{}, err
		}
		if _, saved := saved.Values["project.starter"]; !saved && !explicit("project.starter") {
			cfg.Project.Starter = "module"
		}
	} else {
		cfg.Evidence = []config.Evidence{}
		if !explicit("commands") && !savedCommands {
			cfg.Commands = nil
		}
	}
	cfg.Generator.Version = config.GeneratorVersion
	cfg.Generator.Template = config.TemplateRevision
	cfg.Generator.Go = config.GoBaseline
	if err := config.Validate(cfg, mode); err != nil {
		return plan.Plan{}, &InputError{Cause: err}
	}
	cfg, err = render.Normalize(cfg, mode)
	if err != nil {
		return plan.Plan{}, &InputError{Cause: err}
	}
	if mode == "new" {
		if cfg.Evidence, err = projectedEvidence(root, cfg, req.Decisions); err != nil {
			return plan.Plan{}, err
		}
	}
	files, err := render.Files(cfg, mode)
	if err != nil {
		return plan.Plan{}, err
	}
	yaml, err := config.Encode(saved, cfg)
	if err != nil {
		return plan.Plan{}, err
	}
	for i := range files {
		switch files[i].Path {
		case "rubric.yaml":
			files[i].Data = yaml
		case "AGENTS.md":
			text := string(files[i].Data)
			if strings.Count(text, "<!-- rubric:begin -->") != 1 || strings.Count(text, "<!-- rubric:end -->") != 1 {
				return plan.Plan{}, inputErr("rendered guidance contains Rubric markers from configuration values; remove them")
			}
		}
	}
	p, err := plan.Prepare(root, mode, cfg, files)
	if err != nil {
		return plan.Plan{}, &InputError{Cause: err}
	}
	if len(req.Decisions) > 0 {
		if p, err = plan.Decide(p, req.Decisions); err != nil {
			return plan.Plan{}, &InputError{Cause: err}
		}
	}
	return p, nil
}

func inspect(root string) (detect.Facts, bool, error) {
	facts, err := detect.Inspect(root)
	if errors.Is(err, fs.ErrNotExist) {
		return detect.Facts{Empty: true}, false, nil
	}
	if err != nil {
		if info, statErr := os.Stat(root); statErr == nil && !info.IsDir() {
			return detect.Facts{}, false, inputErr("target %s is not a directory", root)
		}
		return detect.Facts{}, false, inputErr("inspect target: %w", err)
	}
	return facts, true, nil
}

func resolveMode(requested string, facts detect.Facts, present bool) (string, error) {
	hasModule := facts.Module != ""
	switch {
	case requested != "" && requested != "auto" && requested != "new" && requested != "existing":
		return "", inputErr("--mode: %q must be auto, new, or existing", requested)
	case !hasModule && facts.Workspace:
		return "", inputErr("target is a go.work workspace; run rubric init in one module directory")
	case !hasModule && len(facts.NestedModules) > 0:
		return "", inputErr("target contains modules in %s; run rubric init in one module directory",
			strings.Join(facts.NestedModules, ", "))
	case hasModule && requested == "new":
		return "", inputErr("target already contains module %s; use --mode existing or auto", facts.Module)
	case hasModule:
		return "existing", nil
	case requested == "existing":
		if !present {
			return "", inputErr("target does not exist; use --mode new to create a project")
		}
		return "", inputErr("target has no go.mod; use --mode new to create a project here")
	case requested == "new" || facts.Empty:
		return "new", nil
	}
	return "", inputErr("target is not empty and has no go.mod; pass --mode new to create a project here")
}

func readSaved(root string) (config.Document, error) {
	path := filepath.Join(root, "rubric.yaml")
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
		return config.Document{Values: config.Patch{}}, nil
	}
	if err != nil {
		return config.Document{}, err
	}
	if !info.Mode().IsRegular() {
		return config.Document{}, inputErr("rubric.yaml is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config.Document{}, err
	}
	doc, err := config.Decode(data)
	if err != nil {
		return config.Document{}, inputErr("rubric.yaml: %w", err)
	}
	return doc, nil
}

func detectionPatch(f detect.Facts) config.Patch {
	p := config.Patch{"project.module": f.Module, "entry_points": slices.Clone(f.EntryPoints)}
	if f.Go != "" {
		p["project.go"] = f.Go
	}
	values := map[string][]string{}
	for _, e := range f.Evidence {
		if strings.HasPrefix(e.Field, "features.") && !slices.Contains(values[e.Field], e.Value) {
			values[e.Field] = append(values[e.Field], e.Value)
		}
	}
	pick := func(field string, weaker string) (string, bool) {
		v := slices.DeleteFunc(slices.Clone(values[field]), func(s string) bool { return len(values[field]) > 1 && s == weaker })
		if len(v) != 1 {
			return "", false
		}
		return v[0], true
	}
	for _, field := range []string{"features.http", "features.cli", "features.tui", "features.config"} {
		if v, ok := pick(field, "nethttp"); ok {
			p[field] = v
		}
	}
	if db, ok := pick("features.database", ""); ok {
		p["features.database"] = db
		if access, ok := pick("features.access", "sql"); ok {
			p["features.access"] = access
		}
	}
	return p
}

func reconcileExisting(cfg *config.Config, previous config.Config, facts detect.Facts, explicit func(string) bool, savedCommands bool) error {
	if explicit("project.module") && cfg.Project.Module != facts.Module {
		return inputErr("project.module: %q does not match go.mod module %q", cfg.Project.Module, facts.Module)
	}
	cfg.Project.Module = facts.Module
	if facts.Go != "" {
		cfg.Project.Go = facts.Go
	}
	if cfg.Project.Name == "" {
		cfg.Project.Name = cfg.Project.Module[strings.LastIndex(cfg.Project.Module, "/")+1:]
	}
	cfg.Evidence = slices.Clone(facts.Evidence)
	detected := map[string]bool{}
	for _, ep := range facts.EntryPoints {
		detected[ep.Dir] = true
	}
	for _, ep := range cfg.EntryPoints {
		if !detected[ep.Dir] {
			return inputErr("entry point %s (%s) has no main package; correct entry_points in rubric.yaml "+
				"or pass --entry-point / --clear-entry-points", ep.Name, ep.Dir)
		}
	}
	switch {
	case explicit("commands"):
	case savedCommands && len(cfg.Commands) > 0:
		cfg.Commands = reconcileCommands(cfg.Commands, derivedCommands(previous), derivedCommands(*cfg))
	case savedCommands:
	default:
		cfg.Commands = nil
	}
	for _, cmd := range cfg.Commands {
		local := len(cmd.Argv) >= 3 && (cmd.Argv[2] == "." || strings.HasPrefix(cmd.Argv[2], "./"))
		if local && cmd.Argv[0] == "go" && cmd.Argv[1] == "run" {
			dir := strings.TrimPrefix(cmd.Argv[2], "./")
			if dir == "" {
				dir = "."
			}
			if !detected[dir] {
				return inputErr("command %s runs %s, which has no main package; correct commands in rubric.yaml "+
					"or pass --command / --clear-commands", cmd.Name, cmd.Argv[2])
			}
		}
	}
	return nil
}

func derivedCommands(c config.Config) []config.Command {
	c.Commands = nil
	return append(render.Commands(c, "new"), render.Commands(c, "existing")...)
}

func reconcileCommands(saved, before, now []config.Command) []config.Command {
	same := func(a config.Command) func(config.Command) bool {
		return func(b config.Command) bool { return reflect.DeepEqual(a, b) }
	}
	named := func(name string) func(config.Command) bool {
		return func(c config.Command) bool { return c.Name == name }
	}
	out := []config.Command{}
	for _, cmd := range saved {
		if slices.ContainsFunc(before, same(cmd)) && !slices.ContainsFunc(now, same(cmd)) {
			continue
		}
		out = append(out, cmd)
	}
	for _, cmd := range now {
		if !slices.ContainsFunc(out, named(cmd.Name)) && !slices.ContainsFunc(before, named(cmd.Name)) {
			out = append(out, cmd)
		}
	}
	return out
}

func projectedEvidence(root string, cfg config.Config, decisions map[string]string) ([]config.Evidence, error) {
	files, err := render.Files(cfg, "new")
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "rubric-detect-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	existing, err := detectionInputs(root)
	if err != nil {
		return nil, err
	}
	for rel, data := range existing {
		if err := writeProjected(dir, rel, data); err != nil {
			return nil, err
		}
	}
	for _, f := range files {
		if _, onDisk := existing[f.Path]; onDisk && decisions[f.Path] != plan.DecisionReplace {
			continue
		}
		if err := writeProjected(dir, f.Path, f.Data); err != nil {
			return nil, err
		}
	}
	facts, err := detect.Inspect(dir)
	if err != nil {
		return nil, err
	}
	return facts.Evidence, nil
}

func writeProjected(dir, rel string, data []byte) error {
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func detectionInputs(root string) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) && path == root {
			return filepath.SkipAll
		}
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (name == ".git" || name == "vendor" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !detectionInput(name) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		out[filepath.ToSlash(rel)] = data
		return err
	})
	return out, err
}

func detectionInput(name string) bool {
	return strings.HasSuffix(name, ".go") || name == "go.mod" || name == "go.work" || strings.HasPrefix(name, "sqlc.")
}
