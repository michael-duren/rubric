package render

import (
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/michael-duren/go-skills/internal/catalog"
	"github.com/michael-duren/go-skills/internal/config"
)

const checkScript = ".rubric/check.sh"

var opName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type checkOp struct {
	Name, Line, Dir string
	Env             []string
	Run             bool
}

type toolingData struct {
	Config                  config.Config
	GoVersion, LintLauncher string
	Ops                     []checkOp
	Build, Test             string
	Lint, Makefile          bool
	Postgres, Generate      bool
	Integration, E2E, Templ bool
	PlaywrightLauncher      string
}

// Run returns the shell text that performs op through Make when enabled, otherwise through the check script.
func (d toolingData) Run(op string) string {
	if d.Makefile {
		return "make " + op
	}
	return "sh " + checkScript + " " + op
}

func usesCheckScript(t config.Tooling) bool {
	return t.Makefile || t.Actions || t.Lint
}

// Tooling renders the enabled optional tooling from the same normalized commands as the guidance.
func Tooling(c config.Config) ([]File, error) {
	d := newToolingData(c)
	var files []File
	steps := []struct {
		on         bool
		path, tmpl string
		mode       fs.FileMode
	}{
		{usesCheckScript(c.Tooling), checkScript, "tooling/check.sh.tmpl", 0o755},
		{c.Tooling.Makefile, "Makefile", "tooling/Makefile.tmpl", 0o644},
		{c.Tooling.Lint, ".golangci.yml", "tooling/golangci.yml.tmpl", 0o644},
		{c.Tooling.Actions, ".github/workflows/ci.yml", "tooling/workflow.yml.tmpl", 0o644},
		{c.Tooling.Skills, ".agents/skills/rubric-workflow/SKILL.md", "tooling/workflow-skill.md.tmpl", 0o644},
		{c.Tooling.Skills, ".agents/skills/rubric-testing/SKILL.md", "tooling/testing-skill.md.tmpl", 0o644},
		{c.Tooling.Skills, ".agents/skills/rubric-style/SKILL.md", "tooling/style-skill.md.tmpl", 0o644},
	}
	for _, s := range steps {
		if !s.on {
			continue
		}
		body, err := execute(s.tmpl, d)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: s.path, Data: body, Mode: s.mode, Kind: KindManaged})
	}
	analyzer, err := styleFiles(c)
	if err != nil {
		return nil, err
	}
	return append(files, analyzer...), nil
}

func newToolingData(c config.Config) toolingData {
	d := toolingData{
		Config:             c,
		GoVersion:          newestGo(c.Project.Go, c.Generator.Go, config.GoBaseline),
		LintLauncher:       catalog.Launcher("golangci-lint"),
		PlaywrightLauncher: catalog.Launcher("playwright"),
		Build:              "go build ./...",
		Test:               "go test ./...",
		Lint:               c.Tooling.Lint,
		Makefile:           c.Tooling.Makefile,
		Postgres:           c.Features.Database == "postgres",
	}
	for _, cmd := range c.Commands {
		line := commandLine(cmd)
		switch {
		case cmd.Name == "build":
			d.Build = line
		case cmd.Name == "test":
			d.Test = line
		case cmd.Name == "lint" || !opName.MatchString(cmd.Name):
		default:
			d.Ops = append(d.Ops, checkOp{
				Name: cmd.Name, Line: line, Dir: QuotePOSIX(cmd.Dir), Env: cmd.Env,
				Run: len(cmd.Argv) > 1 && cmd.Argv[0] == "go" && cmd.Argv[1] == "run" && cmd.Name != "generate",
			})
			d.Generate = d.Generate || cmd.Name == "generate"
			d.Integration = d.Integration || cmd.Name == "test-integration"
			d.E2E = d.E2E || cmd.Name == "test-e2e"
			d.Templ = d.Templ || cmd.Name == "generate-templ"
		}
	}
	return d
}

func commandLine(cmd config.Command) string {
	words := make([]string, len(cmd.Argv))
	for i, a := range cmd.Argv {
		words[i] = QuotePOSIX(a)
	}
	line := strings.Join(words, " ")
	if cmd.Dir != "." {
		line = "cd " + QuotePOSIX(cmd.Dir) + " && " + line
	}
	return line
}

func newestGo(versions ...string) string {
	best := ""
	for _, v := range versions {
		if v != "" && (best == "" || semver.Compare("v"+v, "v"+best) > 0) {
			best = v
		}
	}
	return best
}

func toolingGuidance(c config.Config) []string {
	d := newToolingData(c)
	var out []string
	if c.Tooling.Makefile {
		targets := []string{"build", "test"}
		if c.Tooling.Lint {
			targets = append(targets, "lint")
		}
		for _, op := range d.Ops {
			targets = append(targets, op.Name)
		}
		out = append(out, "`Makefile`: `make test` and the other targets ("+strings.Join(slices.Compact(targets), ", ")+
			") run `sh .rubric/check.sh <target>`")
	} else if usesCheckScript(c.Tooling) {
		out = append(out, "`.rubric/check.sh`: run `sh .rubric/check.sh build`, `test`, and the other configured operations")
	}
	if c.Tooling.Lint {
		out = append(out, "`.golangci.yml` and `.rubric/style`: `"+d.Run("lint")+"` checks formatting, vet, static analysis, and the comment policy")
	}
	if c.Tooling.Actions {
		out = append(out, "`.github/workflows/ci.yml`: runs the same checks in GitHub Actions")
	}
	if c.Tooling.Skills {
		out = append(out, "Agent skills: `.agents/skills/rubric-workflow/SKILL.md`, `.agents/skills/rubric-testing/SKILL.md`, "+
			"`.agents/skills/rubric-style/SKILL.md`")
	}
	return out
}
