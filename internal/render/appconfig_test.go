package render_test

import (
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

func features(mut func(*config.Features)) config.Features {
	f := config.Defaults().Features
	mut(&f)
	return f
}

func TestGeneratedConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated projects")
	}
	common := []string{"TestEnvironmentOverridesDefaults", "TestInvalidAddress"}
	viper := []string{"TestMissingFile", "TestMalformedYAML", "TestTypeError", "TestFileOverridesDefault",
		"TestEnvironmentOverridesFile", "TestIsolatedInstances"}
	tests := []struct {
		name string
		f    config.Features
		want []string
	}{
		{"stdlib http", features(func(f *config.Features) { f.HTTP = "nethttp" }), append([]string{"TestDefaults", "TestConfigFileUnsupported", "TestHealth"}, common...)},
		{"viper library only", features(func(f *config.Features) { f.Config = "viper" }), append([]string{"TestDefaults"}, append(common, viper...)...)},
		{"viper chi flag", features(func(f *config.Features) { f.Config, f.HTTP, f.CLI = "viper", "chi", "flag" }), append([]string{"TestStatus"}, append(common, viper...)...)},
		{"stdlib sqlite", features(func(f *config.Features) { f.Database, f.Access = "sqlite", "sql" }), append([]string{"TestDefaults", "TestDatabaseDefault"}, common...)},
		{"viper postgres", features(func(f *config.Features) { f.Database, f.Access, f.Config = "postgres", "sql", "viper" }),
			append([]string{"TestMissingDatabaseURL", "TestInvalidDatabaseURL"}, append(common, viper...)...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildAndTest(t, tt.f, tt.want...)
		})
	}
}

func TestGeneratedConfigPlacement(t *testing.T) {
	render1 := func(starter string, f config.Features) []render.File {
		c := testproject.Config()
		c.Project.Starter = starter
		c.Features = f
		files, err := render.Files(c, "new")
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	for _, starter := range []string{"module", "runnable"} {
		if _, ok := find(render1(starter, config.Defaults().Features), "internal/config/config.go"); ok {
			t.Fatalf("%s starter got a config package", starter)
		}
	}
	files := render1("runnable", features(func(f *config.Features) { f.Config = "viper" }))
	if !strings.Contains(string(testproject.File(t, files, "cmd/demo/main.go")), "config.Load(") {
		t.Fatal("runnable main does not load Viper settings")
	}
	mod := string(testproject.File(t, files, "go.mod"))
	if !strings.Contains(mod, "github.com/spf13/viper") || strings.Contains(mod, "cobra") {
		t.Fatalf("go.mod = %s", mod)
	}
	server := render1("module", features(func(f *config.Features) { f.HTTP = "nethttp" }))
	main := string(testproject.File(t, server, "cmd/server/main.go"))
	if !strings.Contains(main, "settings.HTTPAddr") || strings.Contains(main, "HTTP_ADDR\"") {
		t.Fatalf("server main does not use centralized settings:\n%s", main)
	}
	agents := string(testproject.File(t, server, "AGENTS.md"))
	if !strings.Contains(agents, "APP_HTTP_ADDR") || strings.Contains(agents, "APP_DATABASE_URL") {
		t.Fatalf("settings guidance:\n%s", agents)
	}
	cli := render1("module", features(func(f *config.Features) { f.CLI = "flag" }))
	if strings.Contains(string(testproject.File(t, cli, "AGENTS.md")), "APP_HTTP_ADDR") {
		t.Fatal("CLI-only guidance documents an HTTP setting")
	}
	viperFiles := render1("module", features(func(f *config.Features) { f.Config, f.Database, f.Access = "viper", "postgres", "sql" }))
	sample, ok := find(viperFiles, "app.yaml")
	if ok && strings.Contains(string(sample.Data), "postgres://") {
		t.Fatal("sample settings contain credentials")
	}
	if src := string(testproject.File(t, viperFiles, "internal/config/config.go")); strings.Contains(src, "viper.SetDefault") ||
		strings.Contains(src, "viper.ReadInConfig") || strings.Contains(src, "rubric.yaml") {
		t.Fatalf("uses Viper's global instance or reads rubric.yaml:\n%s", src)
	}
}
