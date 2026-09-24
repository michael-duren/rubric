package config

import (
	"strings"
	"testing"
)

func TestWebAndE2EDefaultsAndDecode(t *testing.T) {
	c := Defaults()
	if c.Features.Web != "none" || c.Features.E2E != "none" {
		t.Fatalf("defaults = %+v", c.Features)
	}
	doc, err := Decode([]byte("features:\n  http: chi\n  web: htmx\n  e2e: playwright\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(Defaults(), doc.Values)
	if err != nil {
		t.Fatal(err)
	}
	if got.Features.Web != "htmx" || got.Features.E2E != "playwright" {
		t.Fatalf("features = %+v", got.Features)
	}
	out, err := Encode(Document{}, Defaults())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "web: none") || !strings.Contains(string(out), "e2e: none") {
		t.Fatalf("encoded:\n%s", out)
	}
}

func TestYAMLEncodeRemovesDroppedMapKeys(t *testing.T) {
	doc, err := Decode([]byte("generator:\n  tools:\n    golangci-lint: v2.13.2 # pinned\n    sqlc: v1.31.1\n"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Defaults()
	cfg.Generator.Tools = map[string]string{"sqlc": "v1.31.1"}
	out, err := Encode(doc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "golangci-lint") || !strings.Contains(string(out), "sqlc: v1.31.1") {
		t.Fatalf("stale map key kept:\n%s", out)
	}
}

func TestExplicitListPresenceSurvivesResolve(t *testing.T) {
	got, err := Resolve(Defaults(), Patch{"project.module": "example.com/a"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Commands != nil || got.EntryPoints != nil {
		t.Fatalf("absent lists became present: %#v %#v", got.Commands, got.EntryPoints)
	}
	got, err = Resolve(Defaults(), Patch{"commands": []Command{}, "entry_points": []EntryPoint{}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Commands == nil || got.EntryPoints == nil {
		t.Fatal("explicit empty lists lost their presence")
	}
	out, err := Encode(Document{}, Defaults())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "commands: []") || !strings.Contains(string(out), "entry_points: []") {
		t.Fatalf("absent lists not serialized as empty:\n%s", out)
	}
}
