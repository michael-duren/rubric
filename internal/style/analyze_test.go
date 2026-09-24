package style

import (
	"strings"
	"testing"
)

func rules(t *testing.T, name, source string) []string {
	t.Helper()
	findings, err := AnalyzeFile(name, []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range findings {
		out = append(out, f.Rule)
	}
	return out
}

func has(list []string, rule string) bool {
	for _, r := range list {
		if r == rule {
			return true
		}
	}
	return false
}

func TestDocCommentLimit(t *testing.T) {
	for _, lines := range []int{2, 3} {
		source := "package sample\n" + strings.Repeat("// Example documents behavior.\n", lines) + "func Example() {}\n"
		findings, err := AnalyzeFile("example.go", []byte(source))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range findings {
			found = found || item.Rule == "doc-lines"
		}
		if found != (lines == 3) {
			t.Fatalf("lines=%d findings=%v", lines, findings)
		}
	}
}

func TestDocLineUnicodeBoundary(t *testing.T) {
	for _, n := range []int{150, 151} {
		line := "// " + strings.Repeat("é", n-3)
		source := "package sample\n\n" + line + "\nfunc Exported() {}\n"
		if got := has(rules(t, "a.go", source), "doc-length"); got != (n == 151) {
			t.Fatalf("runes=%d doc-length=%v", n, got)
		}
	}
}

func TestDocBlockCommentCountsPhysicalLines(t *testing.T) {
	source := "package sample\n\n/*\nExported does work.\n*/\nfunc Exported() {}\n"
	if !has(rules(t, "a.go", source), "doc-lines") {
		t.Fatal("three-line block comment accepted")
	}
}

func TestDocDirectivesDoNotCount(t *testing.T) {
	source := "package sample\n\nimport _ \"embed\"\n\n// Data holds the bundled text.\n// It is read at startup.\n//\n//go:embed data.txt\nvar Data string\n"
	got := rules(t, "a.go", source)
	if has(got, "doc-lines") {
		t.Fatalf("directive counted as documentation: %v", got)
	}
}

func TestDocMissing(t *testing.T) {
	tests := []struct {
		name, source string
		missing      bool
	}{
		{"exported func", "package p\n\nfunc Run() {}\n", true},
		{"unexported func", "package p\n\nfunc run() {}\n", false},
		{"documented func", "package p\n\n// Run runs.\nfunc Run() {}\n", false},
		{"exported type", "package p\n\ntype T struct{}\n", true},
		{"method on exported type", "package p\n\n// T is a thing.\ntype T struct{}\n\nfunc (T) Do() {}\n", true},
		{"pointer method documented", "package p\n\n// T is a thing.\ntype T struct{}\n\n// Do does.\nfunc (*T) Do() {}\n", false},
		{"method on unexported type", "package p\n\ntype t struct{}\n\nfunc (t) Do() {}\n", false},
		{"generic receiver", "package p\n\n// T is generic.\ntype T[E any] struct{}\n\nfunc (T[E]) Do() {}\n", true},
		{"group doc covers specs", "package p\n\n// Limits bound inputs.\nconst (\n\tMin = 1\n\tMax = 2\n)\n", false},
		{"undocumented group", "package p\n\nconst (\n\tMin = 1\n\tmax = 2\n)\n", true},
		{"spec doc in group", "package p\n\nvar (\n\t// Err reports failure.\n\tErr error\n)\n", false},
		{"exported field needs no doc", "package p\n\n// T is a thing.\ntype T struct {\n\tName string\n}\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := has(rules(t, "a.go", tt.source), "doc-missing"); got != tt.missing {
				t.Fatalf("doc-missing = %v", got)
			}
		})
	}
}

func TestDocLimitsApplyToFieldsMethodsAndGroups(t *testing.T) {
	long := "// " + strings.Repeat("x", 150) + "\n"
	for name, source := range map[string]string{
		"field":            "package p\n\n// T is a thing.\ntype T struct {\n\t" + long + "\tName string\n}\n",
		"interface method": "package p\n\n// I is a thing.\ntype I interface {\n\t" + long + "\tDo()\n}\n",
		"group":            "package p\n\n" + long + "const (\n\tA = 1\n)\n",
		"method":           "package p\n\n// T is a thing.\ntype T struct{}\n\n" + long + "func (T) Do() {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if !has(rules(t, "a.go", source), "doc-length") {
				t.Fatal("long documentation accepted")
			}
		})
	}
}

func TestCommentInFunctionBody(t *testing.T) {
	source := "package p\n\nfunc run() {\n\t// explain the next line\n\tx := 1\n\t_ = x\n}\n"
	if !has(rules(t, "a.go", source), "comment") {
		t.Fatal("body comment accepted")
	}
}

func TestCommentTrailing(t *testing.T) {
	for name, source := range map[string]string{
		"statement": "package p\n\nfunc run() {\n\tx := 1 // one\n\t_ = x\n}\n",
		"field":     "package p\n\ntype t struct {\n\tn int // count\n}\n",
		"top level": "package p\n\nvar x = 1 // one\n",
	} {
		t.Run(name, func(t *testing.T) {
			if !has(rules(t, "a.go", source), "comment") {
				t.Fatal("trailing comment accepted")
			}
		})
	}
}

func TestCommentAllowances(t *testing.T) {
	for name, source := range map[string]string{
		"build tag":        "//go:build integration\n\npackage p\n",
		"legacy build tag": "// +build integration\n\npackage p\n",
		"package doc":      "// Package p does things.\npackage p\n",
		"header":           "// Copyright 2026 Example.\n\npackage p\n",
		"generate":         "package p\n\n//go:generate stringer -type=K\n",
		"line directive":   "package p\n\nfunc f() {\n//line a.go:1\n}\n",
		"string markers":   "package p\n\nvar s = \"// not a comment /* either */\"\n",
		"nolint":           "package p\n\nfunc f() {\n\t_ = 1 //nolint:errcheck\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got := rules(t, "a.go", source); len(got) != 0 {
				t.Fatalf("findings = %v", got)
			}
		})
	}
	if got := rules(t, "a.go", "package p\n\n// gofmt: explain something\nvar x = 1\n"); has(got, "comment") {
		t.Fatalf("doc on unexported var flagged: %v", got)
	}
	if got := rules(t, "a.go", "package p\n\n//go is great\nfunc f() {\n\t//go is great\n}\n"); !has(got, "comment") {
		t.Fatal("prose starting with //go accepted as a directive")
	}
}

func TestTestEntryPoints(t *testing.T) {
	source := "package p\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) {}\n\nfunc BenchmarkRun(b *testing.B) {}\n\n" +
		"func FuzzRun(f *testing.F) {}\n\nfunc ExampleRun() {}\n\nfunc TestMain(m *testing.M) {}\n"
	if got := rules(t, "p_test.go", source); len(got) != 0 {
		t.Fatalf("test entry points flagged: %v", got)
	}
	if !has(rules(t, "p.go", "package p\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) {}\n"), "doc-missing") {
		t.Fatal("Test-named function outside _test.go exempted")
	}
	for _, bad := range []string{"func Testrun(t *testing.T) {}\n", "func TestRun() {}\n", "func Helper() {}\n"} {
		if !has(rules(t, "p_test.go", "package p\n\nimport \"testing\"\n\nvar _ testing.T\n\n"+bad), "doc-missing") {
			t.Fatalf("%q exempted", bad)
		}
	}
}

func TestGeneratedFilesSkipped(t *testing.T) {
	source := "// Code generated by sqlc. DO NOT EDIT.\n// versions:\n//   sqlc v1.31.1\n\npackage queries\n\nfunc Exported() {\n\t// body\n}\n"
	if got := rules(t, "queries.sql.go", source); len(got) != 0 {
		t.Fatalf("generated file flagged: %v", got)
	}
}

func TestMalformedGo(t *testing.T) {
	if _, err := AnalyzeFile("bad.go", []byte("package p\n\nfunc {\n")); err == nil {
		t.Fatal("malformed Go accepted")
	}
}

func TestFindingsSortedWithLines(t *testing.T) {
	source := "package p\n\nfunc B() {}\n\nfunc A() {\n\tx := 1 // two\n\t_ = x\n}\n"
	findings, err := AnalyzeFile("a.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	want := []Finding{
		{File: "a.go", Line: 3, Rule: "doc-missing"},
		{File: "a.go", Line: 5, Rule: "doc-missing"},
		{File: "a.go", Line: 6, Rule: "comment"},
	}
	if len(findings) != len(want) {
		t.Fatalf("findings = %+v", findings)
	}
	for i, f := range findings {
		if f.File != want[i].File || f.Line != want[i].Line || f.Rule != want[i].Rule || f.Message == "" {
			t.Fatalf("finding %d = %+v, want %+v", i, f, want[i])
		}
	}
}
