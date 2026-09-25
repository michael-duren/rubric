package cli

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/features"
	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/plan"
)

type featureList struct {
	Target   string          `json:"target"`
	Skills   map[string]bool `json:"skills"`
	Tooling  map[string]bool `json:"tooling"`
	Features config.Features `json:"features"`
	Pending  []actionReport  `json:"pending"`
}

func listFeatures(c *cobra.Command, o options, target string, s Streams) error {
	req, err := request(c, o, target)
	if err != nil {
		return err
	}
	p, err := initialize.Prepare(c.Context(), req)
	if err != nil {
		return err
	}
	l := featureList{Target: target, Skills: map[string]bool{}, Tooling: map[string]bool{}, Features: p.Config.Features, Pending: []actionReport{}}
	for _, f := range features.All {
		on := features.Enabled(p.Config.Tooling, f.Key)
		if group := features.SkillGroup(f.Key); group != "" {
			l.Skills[group] = on
		} else {
			l.Tooling[strings.TrimPrefix(f.Key, "tooling.")] = on
		}
	}
	for _, a := range p.Actions {
		if a.State != plan.StateUnchanged {
			l.Pending = append(l.Pending, actionReport{Path: a.File.Path, Kind: a.File.Kind, State: a.State, Reason: a.Reason})
		}
	}
	c.Annotations["reported"] = "true"
	if o.format == "json" {
		return writeJSON(s.Out, l)
	}
	writeList(s.Out, l, p.Config.Tooling)
	return nil
}

func writeList(out io.Writer, l featureList, t config.Tooling) {
	printf(out, "rubric update: features at %s\n", l.Target)
	for _, menu := range features.Menus {
		printf(out, "\n%s\n", menu)
		for _, f := range features.In(menu) {
			box := "[ ]"
			if features.Enabled(t, f.Key) {
				box = "[x]"
			}
			printf(out, "  %s %-18s %s\n", box, f.Label, f.Help)
		}
	}
	f := l.Features
	printf(out, "\nApplication (detected; edit rubric.yaml to change): http %s, database %s/%s, cli %s, tui %s, config %s, web %s, e2e %s\n",
		f.HTTP, f.Database, f.Access, f.CLI, f.TUI, f.Config, none(f.Web), none(f.E2E))
	if len(l.Pending) == 0 {
		printLine(out, "\nFiles match rubric.yaml.")
		return
	}
	printf(out, "\n%d file change(s) pending; run rubric update to review and apply them:\n", len(l.Pending))
	for _, a := range l.Pending {
		printf(out, "  %-9s %s\n", a.State, a.Path)
	}
}

func none(v string) string {
	if v == "" {
		return "none"
	}
	return v
}
