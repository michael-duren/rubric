package style

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// AnalyzeTree checks Go files under root, skipping vendor, testdata, hidden dirs, and nested modules.
func AnalyzeTree(root string) ([]Finding, error) {
	var findings []Finding
	var failures []error
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			failures = append(failures, err)
			return nil
		}
		if d.IsDir() {
			return skipDir(root, path, d.Name())
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			failures = append(failures, err)
			return nil
		}
		found, err := AnalyzeFile(filepath.ToSlash(rel), src)
		if err != nil {
			failures = append(failures, fmt.Errorf("parse %s: %w", filepath.ToSlash(rel), err))
			return nil
		}
		findings = append(findings, found...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(findings, compareFindings)
	return findings, errors.Join(failures...)
}

func skipDir(root, path, name string) error {
	if path == root {
		return nil
	}
	if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return filepath.SkipDir
	}
	if _, err := os.Lstat(filepath.Join(path, "go.mod")); err == nil {
		return filepath.SkipDir
	}
	return nil
}
