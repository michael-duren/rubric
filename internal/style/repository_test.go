package style

import (
	"fmt"
	"strings"
	"testing"
)

func TestRepositoryStyle(t *testing.T) {
	findings, err := AnalyzeTree("../..")
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for _, f := range findings {
		if !strings.HasSuffix(f.File, "_test.go") {
			bad = append(bad, fmt.Sprintf("%s:%d: %s: %s", f.File, f.Line, f.Rule, f.Message))
		}
	}
	if len(bad) > 0 {
		t.Fatalf("Rubric source violates its own style policy:\n%s", strings.Join(bad, "\n"))
	}
}
