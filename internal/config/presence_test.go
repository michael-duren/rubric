package config

import (
	"strings"
	"testing"
)

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
