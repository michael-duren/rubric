package acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostgresIntegration(t *testing.T) {
	acceptance(t)
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("RUBRIC_ACCEPTANCE=1 requires TEST_DATABASE_URL for an isolated PostgreSQL test database")
	}
	for _, access := range []string{"sql", "sqlc"} {
		t.Run(access, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "app")
			mustRubric(t, "init", root, "--module", "example.com/pg", "--database", "postgres", "--access", access,
				"--http", "nethttp", "--makefile")
			mustCommand(t, root, nil, "go", "mod", "tidy")
			unit := mustCommand(t, root, []string{"TEST_DATABASE_URL="}, "go", "test", "-count=1", "./...")
			if strings.Contains(unit, "FAIL") {
				t.Fatalf("default tests depend on the database:\n%s", unit)
			}
			if out, err := command(t, root, []string{"TEST_DATABASE_URL="}, "sh", ".rubric/check.sh", "test-integration"); err == nil {
				t.Fatalf("integration check ran without TEST_DATABASE_URL:\n%s", out)
			}
			out := mustCommand(t, root, []string{"TEST_DATABASE_URL=" + url}, "go", "test", "-count=1", "-tags", "integration", "-v", "./internal/store/...")
			for _, name := range []string{"TestIntegrationRoundTrip", "TestIntegrationUpsert", "TestIntegrationTransaction", "TestIntegrationNotFound"} {
				if !strings.Contains(out, "--- PASS: "+name) {
					t.Fatalf("%s did not pass:\n%s", name, out)
				}
			}
			mustCommand(t, root, []string{"TEST_DATABASE_URL=" + url}, "sh", ".rubric/check.sh", "test-integration")
		})
	}
}
