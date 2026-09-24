package acceptance

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCopiedAnalyzerCoverage(t *testing.T) {
	acceptance(t)
	root := filepath.Join(t.TempDir(), "app")
	mustRubric(t, "init", root, "--module", "example.com/lint", "--lint")
	mustCommand(t, root, nil, "go", "test", "-count=1", "-coverprofile=style.out", "./.rubric/style")
	covered := map[string]int{}
	f, err := os.Open(filepath.Join(root, "style.out"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		name, rest, ok := strings.Cut(scanner.Text(), ":")
		fields := strings.Fields(rest)
		if !ok || len(fields) != 3 {
			continue
		}
		stmts, _ := strconv.Atoi(fields[1])
		if count, _ := strconv.Atoi(fields[2]); count > 0 {
			covered[filepath.Base(name)] += stmts
		}
	}
	for _, file := range []string{"analyze.go", "tree.go"} {
		if covered[file] == 0 {
			t.Errorf("copied analyzer %s has no exercised statements", file)
		}
	}
	t.Log(mustCommand(t, root, nil, "go", "tool", "cover", "-func=style.out"))
}

func TestRepresentativeCoverageReports(t *testing.T) {
	acceptance(t)
	for _, flags := range [][]string{
		{"--http", "chi", "--cli", "cobra", "--tui", "bubbletea", "--app-config", "viper"},
		{"--database", "postgres", "--access", "sqlc"},
		{"--database", "sqlite", "--access", "sql", "--starter", "runnable"},
	} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "app")
			mustRubric(t, append([]string{"init", root, "--module", "example.com/cov"}, flags...)...)
			runProjectChecks(t, root)
		})
	}
}
