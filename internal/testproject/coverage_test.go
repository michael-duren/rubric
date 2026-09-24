package testproject

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/render"
)

func coverageFixture(t *testing.T, extra map[string]string) (string, string) {
	t.Helper()
	files := []render.File{
		{Path: "go.mod", Data: []byte("module example.com/cov\n\ngo 1.26.7\n"), Mode: 0o644},
		{Path: "main.go", Data: []byte("package main\n\nfunc main() { println(helper()) }\n\nfunc helper() int { return 1 }\n"), Mode: 0o644},
		{Path: "cmd/tool/main.go", Data: []byte("package main\n\nfunc main() {}\n"), Mode: 0o644},
		{Path: "internal/good/good.go", Data: []byte("package good\n\n// Sign reports the sign of n.\nfunc Sign(n int) int {\n\tif n < 0 {\n\t\treturn -1\n\t}\n\treturn 1\n}\n"), Mode: 0o644},
		{Path: "internal/good/good_test.go", Data: []byte("package good\n\nimport \"testing\"\n\nfunc TestSign(t *testing.T) {\n\tif Sign(2) != 1 {\n\t\tt.Fatal()\n\t}\n}\n"), Mode: 0o644},
		{Path: "internal/good/data.go", Data: []byte("package good\n\n// Limit bounds inputs.\nconst Limit = 3\n"), Mode: 0o644},
	}
	for path, body := range extra {
		files = append(files, render.File{Path: path, Data: []byte(body), Mode: 0o644})
	}
	root := Write(t, files)
	Go(t, root, "test", "-coverprofile=coverage.out", "./...")
	return root, filepath.Join(root, "coverage.out")
}

func TestBehaviorCoverageAcceptsTestedPackages(t *testing.T) {
	root, profile := coverageFixture(t, nil)
	failures, notes, err := behaviorGaps(root, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 0 {
		t.Fatalf("failures = %v", failures)
	}
	if !slices.ContainsFunc(notes, func(n string) bool { return strings.Contains(n, "internal/good/good.go:5") }) {
		t.Fatalf("uncovered branch not reported for review: %v", notes)
	}
	AssertBehaviorCoverage(t, root, profile)
}

func TestBehaviorCoverageRejectsUntestedPackage(t *testing.T) {
	root, profile := coverageFixture(t, map[string]string{
		"internal/bad/bad.go": "package bad\n\n// Double doubles n.\nfunc Double(n int) int { return n * 2 }\n",
	})
	failures, _, err := behaviorGaps(root, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 || !strings.Contains(failures[0], "internal/bad/bad.go") {
		t.Fatalf("failures = %v", failures)
	}
}

func TestBehaviorCoverageRejectsMissingProfileEntry(t *testing.T) {
	root, profile := coverageFixture(t, nil)
	if err := os.WriteFile(filepath.Join(root, "internal", "good", "late.go"), []byte("package good\n\nfunc late() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	failures, _, err := behaviorGaps(root, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 || !strings.Contains(failures[0], "late.go") {
		t.Fatalf("failures = %v", failures)
	}
}

func TestBehaviorCoverageMalformedProfile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.out")
	if err := os.WriteFile(path, []byte("not a profile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := behaviorGaps(root, path); err == nil {
		t.Fatal("malformed profile accepted")
	}
}

func TestSnapshot(t *testing.T) {
	root := Write(t, []render.File{
		{Path: "a.txt", Data: []byte("a"), Mode: 0o644},
		{Path: "dir/b.txt", Data: []byte("b"), Mode: 0o644},
	})
	snap := Snapshot(t, root)
	if string(snap["a.txt"]) != "a" || string(snap["dir/b.txt"]) != "b" || len(snap) != 2 {
		t.Fatalf("snapshot = %v", snap)
	}
}
