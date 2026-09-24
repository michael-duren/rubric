// Package testproject provides helpers for tests that render, write, and build generated projects.
package testproject

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
)

// Config returns the defaults for a new project named demo at example.com/demo.
func Config() config.Config {
	c := config.Defaults()
	c.Project.Module = "example.com/demo"
	c.Project.Name = "demo"
	return c
}

// File returns the rendered bytes at path, failing the test when it is absent.
func File(t *testing.T, files []render.File, path string) []byte {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return f.Data
		}
	}
	t.Fatalf("rendered output has no %s", path)
	return nil
}

// Write materializes files in a fresh temporary directory removed when the test ends.
func Write(t *testing.T, files []render.File) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		if !filepath.IsLocal(filepath.FromSlash(f.Path)) {
			t.Fatalf("refusing non-local path %q", f.Path)
		}
		path := filepath.Join(root, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, f.Data, f.Mode.Perm()); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, f.Mode.Perm()); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// Go runs the go tool in dir with the local toolchain and no workspace, returning combined output.
func Go(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %q in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}
