package render

import "github.com/michael-duren/go-skills/internal/config"

var configOutputs = []output{
	{path: "internal/config/settings.go", template: "config/settings.go.tmpl", kind: KindScaffold, when: configSelected("")},
	{path: "internal/config/config.go", template: "config/stdlib.go.tmpl", kind: KindScaffold, when: configSelected("stdlib")},
	{path: "internal/config/config.go", template: "config/viper.go.tmpl", kind: KindScaffold, when: configSelected("viper")},
	{path: "internal/config/config_test.go", template: "config/config_test.go.tmpl", kind: KindScaffold, when: configSelected("")},
	{path: "app.yaml", template: "config/app.yaml.tmpl", kind: KindScaffold, when: sampleSettings},
}

func configSelected(impl string) func(config.Config, string) bool {
	return func(c config.Config, mode string) bool {
		return mode == "new" && configPackage(c) && (impl == "" || c.Features.Config == impl)
	}
}

func sampleSettings(c config.Config, mode string) bool {
	return configSelected("viper")(c, mode) && (c.Features.HTTP != "none" || c.Features.Database == "sqlite")
}

func settingsGuidance(c config.Config, mode string) []string {
	if !configPackage(c) || !generated(c, mode, "internal/config") {
		return nil
	}
	var out []string
	if c.Features.Config == "viper" {
		out = append(out, "`APP_CONFIG_FILE`: optional YAML settings file read by Viper; environment values override it")
	} else {
		out = append(out, "`APP_CONFIG_FILE`: leave unset; standard-library settings come only from the environment")
	}
	if c.Features.HTTP != "none" {
		out = append(out, "`APP_HTTP_ADDR`: HTTP listen address (default `:8080`)")
	}
	switch c.Features.Database {
	case "sqlite":
		out = append(out, "`APP_DATABASE_URL`: SQLite database file (default `app.db`)")
	case "postgres":
		out = append(out, "`APP_DATABASE_URL`: PostgreSQL URL; required, never committed")
	}
	return out
}
