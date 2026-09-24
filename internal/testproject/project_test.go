package testproject

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/render"
)

func TestConfig(t *testing.T) {
	c := Config()
	if c.Project.Module != "example.com/demo" || c.Project.Name != "demo" || c.Project.Starter != "module" {
		t.Fatalf("config = %+v", c.Project)
	}
}

func TestWriteAndFile(t *testing.T) {
	files := []render.File{
		{Path: "a/b.txt", Data: []byte("hello"), Mode: 0o644},
		{Path: "run.sh", Data: []byte("#!/bin/sh\n"), Mode: 0o755},
	}
	root := Write(t, files)
	data, err := os.ReadFile(filepath.Join(root, "a", "b.txt"))
	if err != nil || string(data) != "hello" {
		t.Fatalf("read %q %v", data, err)
	}
	info, err := os.Stat(filepath.Join(root, "run.sh"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("mode %v %v", info, err)
	}
	if string(File(t, files, "a/b.txt")) != "hello" {
		t.Fatal("File lookup failed")
	}
}

func TestWriteDirectoryIsCleanedUp(t *testing.T) {
	var root string
	t.Run("inner", func(t *testing.T) {
		root = Write(t, []render.File{{Path: "x", Data: []byte("x"), Mode: 0o644}})
	})
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("temporary project not removed: %v", err)
	}
}

func TestGoPassesArgumentsLiterally(t *testing.T) {
	root := Write(t, []render.File{
		{Path: "go.mod", Data: []byte("module example.com/args\n\ngo 1.26.7\n"), Mode: 0o644},
		{Path: "main.go", Data: []byte("package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tfor _, a := range os.Args[1:] {\n\t\tfmt.Printf(\"<%s>\\n\", a)\n\t}\n}\n"), Mode: 0o644},
	})
	args := []string{"a b", "$(echo hi)", "`id`", `"q"`, "'s'", "*"}
	out := Go(t, root, append([]string{"run", "."}, args...)...)
	for _, a := range args {
		if !strings.Contains(out, "<"+a+">") {
			t.Fatalf("argument %q not passed literally:\n%s", a, out)
		}
	}
}

func TestGoUsesLocalToolchainWithoutWorkspace(t *testing.T) {
	root := Write(t, []render.File{{Path: "go.mod", Data: []byte("module example.com/env\n\ngo 1.26.7\n"), Mode: 0o644}})
	out := Go(t, root, "env", "GOWORK", "GOTOOLCHAIN")
	if strings.TrimSpace(out) != "off\nlocal" {
		t.Fatalf("env = %q", out)
	}
}
