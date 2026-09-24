package testproject

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

// Snapshot returns every regular file under root keyed by slash-separated relative path.
func Snapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		out[filepath.ToSlash(rel)] = data
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// AssertBehaviorCoverage fails for non-exempt source with no exercised statements and logs uncovered blocks for review.
func AssertBehaviorCoverage(t *testing.T, root, profile string) {
	t.Helper()
	failures, notes, err := behaviorGaps(root, profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notes {
		t.Log(n)
	}
	for _, f := range failures {
		t.Error(f)
	}
}

type fileCoverage struct {
	statements, covered int
	uncovered           []int
}

func behaviorGaps(root, profile string) ([]string, []string, error) {
	modData, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, nil, err
	}
	module := modfile.ModulePath(modData)
	coverage, err := readProfile(profile, module)
	if err != nil {
		return nil, nil, err
	}
	sources, err := behaviorSources(root)
	if err != nil {
		return nil, nil, err
	}
	var failures, notes []string
	for _, rel := range sources {
		c, ok := coverage[rel]
		switch {
		case !ok:
			failures = append(failures, rel+": not in the coverage profile; its package has no tests or was not built")
			continue
		case c.statements > 0 && c.covered == 0:
			failures = append(failures, rel+": no statements are exercised by the shipped tests")
		}
		slices.Sort(c.uncovered)
		for _, line := range slices.Compact(c.uncovered) {
			notes = append(notes, fmt.Sprintf("%s:%d: block not exercised by tests", rel, line))
		}
	}
	return failures, notes, nil
}

func readProfile(path, module string) (map[string]*fileCoverage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "mode: ") {
		return nil, fmt.Errorf("%s: not a coverage profile", path)
	}
	out := map[string]*fileCoverage{}
	for scanner.Scan() {
		line := scanner.Text()
		name, rest, ok := strings.Cut(line, ":")
		fields := strings.Fields(rest)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("%s: malformed line %q", path, line)
		}
		start, _, _ := strings.Cut(fields[0], ".")
		startLine, err1 := strconv.Atoi(start)
		stmts, err2 := strconv.Atoi(fields[1])
		count, err3 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return nil, fmt.Errorf("%s: malformed line %q", path, line)
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(name, module), "/")
		c := out[rel]
		if c == nil {
			c = &fileCoverage{}
			out[rel] = c
		}
		c.statements += stmts
		if count > 0 {
			c.covered += stmts
		} else if stmts > 0 {
			c.uncovered = append(c.uncovered, startLine)
		}
	}
	return out, scanner.Err()
}

func behaviorSources(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (name == "cmd" || name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "main.go" {
			return nil
		}
		executable, err := hasStatements(path)
		if err != nil || !executable {
			return err
		}
		rel, err := filepath.Rel(root, path)
		out = append(out, filepath.ToSlash(rel))
		return err
	})
	return out, err
}

func hasStatements(path string) (bool, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return false, err
	}
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil && len(fn.Body.List) > 0 {
			return true, nil
		}
	}
	return false, nil
}
