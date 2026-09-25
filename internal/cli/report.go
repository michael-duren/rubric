package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/plan"
	"github.com/michael-duren/go-skills/internal/write"
)

type actionReport struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	State  string `json:"state"`
	Reason string `json:"reason"`
}

type diagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type report struct {
	command     string
	Status      string           `json:"status"`
	Mode        string           `json:"mode,omitempty"`
	Target      string           `json:"target"`
	DryRun      bool             `json:"dry_run"`
	Config      *config.Config   `json:"config,omitempty"`
	Actions     []actionReport   `json:"actions"`
	Conflicts   []actionReport   `json:"conflicts"`
	Diagnostics []diagnostic     `json:"diagnostics"`
	Next        []config.Command `json:"next_commands"`
	Result      *write.Result    `json:"result,omitempty"`
}

func newReport(command, target string, dryRun bool, p *plan.Plan, res *write.Result, err error) report {
	r := report{
		command: command, Status: status(err, dryRun), Target: target, DryRun: dryRun,
		Actions: []actionReport{}, Conflicts: []actionReport{},
		Diagnostics: []diagnostic{}, Next: []config.Command{}, Result: res,
	}
	var conflict *initialize.ConflictError
	if p == nil && errors.As(err, &conflict) {
		p = &conflict.Plan
	}
	if p != nil && p.Mode != "" {
		r.Mode = p.Mode
		r.Config = &p.Config
		for _, a := range p.Actions {
			item := actionReport{Path: a.File.Path, Kind: a.File.Kind, State: a.State, Reason: a.Reason}
			r.Actions = append(r.Actions, item)
			if a.State == plan.StateConflict {
				r.Conflicts = append(r.Conflicts, item)
			}
		}
		if err == nil {
			r.Next = append(r.Next, p.Config.Commands...)
		}
		r.Diagnostics = append(r.Diagnostics, notes(*p)...)
	}
	if err != nil {
		r.Diagnostics = append([]diagnostic{{Severity: "error", Message: err.Error()}}, r.Diagnostics...)
	}
	return r
}

func notes(p plan.Plan) []diagnostic {
	var out []diagnostic
	if p.Mode == "existing" && semver.Compare("v"+p.Config.Project.Go, "v"+config.GoBaseline) < 0 {
		out = append(out, diagnostic{Severity: "warning", Message: fmt.Sprintf(
			"go.mod declares Go %s; Rubric tooling is tested with Go %s. go.mod was not changed.",
			p.Config.Project.Go, config.GoBaseline)})
	}
	conflicts := plan.Conflicts(p)
	if len(conflicts) > 0 {
		out = append(out, diagnostic{Severity: "error", Message: "resolve each conflict outside rubric (edit, move, or delete the file), then rerun"})
	}
	if slices.ContainsFunc(conflicts, func(a plan.Action) bool { return a.Obsolete }) {
		out = append(out, diagnostic{Severity: "error", Message: "files Rubric no longer generates but you edited: " +
			"delete them, or run rubric update in a terminal to keep them"})
	}
	return out
}

func writeJSON(w io.Writer, r any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func writeText(out, errw io.Writer, r report) {
	if r.Mode != "" {
		printf(out, "rubric %s: %s project at %s\n", r.command, r.Mode, r.Target)
		for _, a := range r.Actions {
			printf(out, "  %-9s %s\n", a.State, a.Path)
		}
	}
	if len(r.Conflicts) > 0 {
		printLine(out, "\nConflicts:")
		for _, c := range r.Conflicts {
			printf(out, "  %s: %s\n", c.Path, c.Reason)
		}
	}
	for _, d := range r.Diagnostics {
		w := out
		if d.Severity == "error" {
			w = errw
		}
		printf(w, "%s: %s\n", d.Severity, d.Message)
	}
	if r.Result != nil && len(r.Result.Unrecovered) > 0 {
		printf(errw, "not restored: %s\n", strings.Join(r.Result.Unrecovered, ", "))
	}
	switch r.Status {
	case "dry-run":
		printLine(out, "\nDry run: no files were written.")
	case "ok":
		if r.Result != nil {
			deleted := ""
			if n := len(r.Result.Deleted); n > 0 {
				deleted = fmt.Sprintf(", deleted %d", n)
			}
			printf(out, "\nApplied %d file(s)%s. Dependencies were not downloaded and no project tests were run.\n",
				len(r.Result.Applied), deleted)
		}
	}
	if len(r.Next) > 0 {
		printLine(out, "\nCommands:")
		for _, c := range r.Next {
			printf(out, "  %s: %s\n", c.Name, strings.Join(quoteArgs(c.Argv), " "))
		}
	}
}

func quoteArgs(argv []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		if a != "" && strings.IndexFunc(a, unsafeShellRune) < 0 {
			out[i] = a
		} else {
			out[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return out
}

func printf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func printLine(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

func unsafeShellRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	}
	return !strings.ContainsRune("_./:=@%+,-", r)
}
