// Package detect gathers read-only facts about an existing Go module without executing its code.
package detect

import (
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/michael-duren/go-skills/internal/config"
)

// Facts describes a target directory and the evidence behind each inferred value.
type Facts struct {
	Module        string              `json:"module"`
	Go            string              `json:"go"`
	Empty         bool                `json:"empty"`
	Workspace     bool                `json:"workspace"`
	NestedModules []string            `json:"nested_modules"`
	EntryPoints   []config.EntryPoint `json:"entry_points"`
	Evidence      []config.Evidence   `json:"evidence"`
}

var skippedDirs = map[string]bool{".git": true, "vendor": true, ".rubric": true, "testdata": true, "node_modules": true}

var sqlcFiles = map[string]bool{"sqlc.yaml": true, "sqlc.yml": true, "sqlc.json": true}

// Inspect reads root without modifying it or running project code.
func Inspect(root string) (Facts, error) {
	info, err := os.Stat(root)
	if err != nil {
		return Facts{}, fmt.Errorf("inspect %s: %w", root, err)
	}
	if !info.IsDir() {
		return Facts{}, fmt.Errorf("inspect %s: not a directory", root)
	}
	s := scan{root: root, facts: Facts{NestedModules: []string{}, EntryPoints: []config.EntryPoint{}, Evidence: []config.Evidence{}}}
	if err := s.module(); err != nil {
		return Facts{}, err
	}
	if err := filepath.WalkDir(root, s.visit); err != nil {
		return Facts{}, fmt.Errorf("inspect %s: %w", root, err)
	}
	s.finish()
	return s.facts, nil
}

type scan struct {
	root    string
	facts   Facts
	entries int
}

func (s *scan) add(field, value, source string) {
	s.facts.Evidence = append(s.facts.Evidence, config.Evidence{Field: field, Value: value, Source: source})
}

func (s *scan) module() error {
	if _, err := os.Stat(filepath.Join(s.root, "go.work")); err == nil {
		s.facts.Workspace = true
		s.add("file", "go.work", "go.work")
	}
	data, err := os.ReadFile(filepath.Join(s.root, "go.mod"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read go.mod: %w", err)
	}
	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return fmt.Errorf("parse go.mod: %w", err)
	}
	if mod.Module != nil {
		s.facts.Module = mod.Module.Mod.Path
		s.add("project.module", s.facts.Module, "go.mod")
	}
	if mod.Go != nil {
		s.facts.Go = mod.Go.Version
		s.add("project.go", s.facts.Go, "go.mod")
	}
	for _, req := range mod.Require {
		if !req.Indirect {
			s.add("dependency", req.Mod.Path+"@"+req.Mod.Version, "go.mod")
		}
	}
	return nil
}

func (s *scan) visit(full string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(s.root, full)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return nil
	}
	name := d.Name()
	if rel != ".git" {
		s.entries++
	}
	if d.IsDir() {
		if skippedDirs[name] || strings.HasPrefix(name, ".") && name != ".github" || strings.HasPrefix(name, "_") {
			return fs.SkipDir
		}
		if _, err := os.Lstat(filepath.Join(full, "go.mod")); err == nil {
			s.facts.NestedModules = append(s.facts.NestedModules, rel)
			return fs.SkipDir
		}
		return nil
	}
	if !d.Type().IsRegular() {
		return nil
	}
	if sqlcFiles[rel] {
		s.add("features.access", "sqlc", rel)
	}
	if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
		return nil
	}
	src, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	return s.source(rel, src)
}

func (s *scan) finish() {
	s.facts.Empty = s.entries == 0 && s.facts.Module == ""
	slices.Sort(s.facts.NestedModules)
	slices.SortFunc(s.facts.EntryPoints, func(a, b config.EntryPoint) int { return cmp.Compare(a.Dir, b.Dir) })
	s.facts.EntryPoints = slices.Compact(s.facts.EntryPoints)
	slices.SortFunc(s.facts.Evidence, compareEvidence)
	s.facts.Evidence = slices.Compact(s.facts.Evidence)
}

func compareEvidence(a, b config.Evidence) int {
	return cmp.Or(cmp.Compare(a.Field, b.Field), cmp.Compare(a.Value, b.Value), cmp.Compare(a.Source, b.Source))
}

func (s *scan) entryName(dir string) string {
	if dir != "." {
		return path.Base(dir)
	}
	if s.facts.Module != "" {
		return path.Base(s.facts.Module)
	}
	return path.Base(filepath.ToSlash(s.root))
}
