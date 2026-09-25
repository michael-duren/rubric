// Package cli parses rubric arguments, runs the initialization service, and reports results.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/plan"
	"github.com/michael-duren/go-skills/internal/wizard"
	"github.com/michael-duren/go-skills/internal/write"
)

// Streams are the process streams; Terminal reports whether stdin and stdout are interactive.
type Streams struct {
	In       io.Reader
	Out      io.Writer
	Err      io.Writer
	Terminal bool
}

// Run executes rubric with args and returns the process exit status.
func Run(ctx context.Context, args []string, s Streams) int {
	root := &cobra.Command{
		Use:           "rubric",
		Short:         "Rubric sets up Go projects with tested templates and accurate agent guidance",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetArgs(args)
	root.SetIn(s.In)
	root.SetOut(s.Out)
	root.SetErr(s.Err)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &usageError{err: err} })
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(initCommand(s), updateCommand(s))
	cmd, err := root.ExecuteContextC(ctx)
	if err == nil {
		return exitOK
	}
	code := exitCode(err)
	if code == exitFailure && !ranInit(cmd) {
		code = exitInvalid
		err = &usageError{err: err}
	}
	if !reported(cmd) {
		printf(s.Err, "rubric: %v\n", err)
	}
	return code
}

func ranInit(cmd *cobra.Command) bool {
	return cmd != nil && cmd.Annotations["ran"] == "true"
}

func reported(cmd *cobra.Command) bool {
	return cmd != nil && cmd.Annotations["reported"] == "true"
}

type wizardFunc func(context.Context, initialize.Request, io.Reader, io.Writer) (wizard.Outcome, error)

func initCommand(s Streams) *cobra.Command {
	var o options
	cmd := &cobra.Command{
		Use:   "init [directory]",
		Short: "Create a Go project or add Rubric configuration and guidance to an existing module",
		Long: "Create a Go project or add Rubric configuration, guidance, and tooling to an existing module.\n\n" +
			"Explicit flags override --config input, which overrides rubric.yaml in the target; detection fills\n" +
			"missing facts for existing projects. Exit status: 0 success, 2 invalid input or conflicts,\n" +
			"1 operational failure, 130 cancelled.",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{},
	}
	register(cmd.Flags(), &o)
	cmd.RunE = func(c *cobra.Command, args []string) error {
		c.Annotations["ran"] = "true"
		if err := checkFormat(o); err != nil {
			return err
		}
		return pipeline(c, o, targetArg(args), s, "init", runWizard)
	}
	return cmd
}

func updateCommand(s Streams) *cobra.Command {
	var o options
	var list bool
	cmd := &cobra.Command{
		Use:   "update [directory]",
		Short: "Show and change which Rubric features are active in an initialized repository",
		Long: "Show the Rubric features recorded in rubric.yaml and change them. In a terminal, update opens a menu of\n" +
			"skills and tooling to toggle; saving reviews and applies the changes, creating files for features turned on\n" +
			"and deleting unedited files for features turned off. Files you edited are conflicts you delete or keep.\n\n" +
			"--list prints the active features and pending file changes without writing. Exit status: 0 success,\n" +
			"2 invalid input, conflicts, or no rubric.yaml, 1 operational failure, 130 cancelled.",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{},
	}
	registerUpdate(cmd.Flags(), &o, &list)
	cmd.RunE = func(c *cobra.Command, args []string) error {
		c.Annotations["ran"] = "true"
		if err := checkFormat(o); err != nil {
			return err
		}
		target := targetArg(args)
		if err := requireConfig(target); err != nil {
			return err
		}
		o.mode = "existing"
		if list {
			if toggled(c) || o.dryRun {
				return &usageError{err: fmt.Errorf("--list cannot be combined with feature flags or --dry-run")}
			}
			return listFeatures(c, o, target, s)
		}
		return pipeline(c, o, target, s, "update", runUpdateWizard)
	}
	return cmd
}

func checkFormat(o options) error {
	if o.format != "text" && o.format != "json" {
		return &usageError{err: fmt.Errorf("--format: %q must be text or json", o.format)}
	}
	return nil
}

func targetArg(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return "."
}

func requireConfig(target string) error {
	info, err := os.Stat(filepath.Join(target, "rubric.yaml"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &initialize.InputError{Cause: fmt.Errorf("%s has no rubric.yaml; run rubric init first", target)}
	case err != nil:
		return err
	case !info.Mode().IsRegular():
		return &initialize.InputError{Cause: errors.New("rubric.yaml is not a regular file")}
	}
	return nil
}

func pipeline(c *cobra.Command, o options, target string, s Streams, name string, wiz wizardFunc) error {
	c.Annotations["reported"] = "true"
	if s.Terminal && o.format == "text" && !o.nonInteractive && !o.dryRun {
		p, res, err := interactive(c.Context(), c, o, target, s, wiz)
		if name == "update" && p == nil && err == nil {
			printLine(s.Out, "rubric update: no changes")
			return nil
		}
		writeText(s.Out, s.Err, newReport(name, target, false, p, res, err))
		return err
	}
	p, res, err := execute(c.Context(), c, o, target)
	r := newReport(name, target, o.dryRun, p, res, err)
	if o.format == "json" {
		if werr := writeJSON(s.Out, r); werr != nil {
			return werr
		}
	} else {
		writeText(s.Out, s.Err, r)
	}
	return err
}

var (
	runWizard       wizardFunc = wizard.Run
	runUpdateWizard wizardFunc = wizard.RunUpdate
)

func request(c *cobra.Command, o options, target string) (initialize.Request, error) {
	patch, err := overrides(c.Flags(), o)
	if err != nil {
		return initialize.Request{}, &initialize.InputError{Cause: err}
	}
	req := initialize.Request{Target: target, Mode: o.mode, Overrides: patch}
	if o.configPath != "" {
		data, err := os.ReadFile(o.configPath)
		if err != nil {
			return initialize.Request{}, &initialize.InputError{Cause: fmt.Errorf("--config: %w", err)}
		}
		req.Input = data
	}
	return req, nil
}

func interactive(ctx context.Context, c *cobra.Command, o options, target string, s Streams, wiz wizardFunc) (*plan.Plan, *write.Result, error) {
	req, err := request(c, o, target)
	if err != nil {
		return nil, nil, err
	}
	outcome, err := wiz(ctx, req, s.In, s.Out)
	if outcome.Cancelled && err == nil {
		err = context.Canceled
	}
	var p *plan.Plan
	if outcome.Plan.Mode != "" {
		p = &outcome.Plan
	}
	return p, &outcome.Result, err
}

func execute(ctx context.Context, c *cobra.Command, o options, target string) (*plan.Plan, *write.Result, error) {
	req, err := request(c, o, target)
	if err != nil {
		return nil, nil, err
	}
	p, err := initialize.Prepare(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	if o.dryRun {
		if len(plan.Conflicts(p)) > 0 {
			return &p, nil, &initialize.ConflictError{Plan: p}
		}
		return &p, nil, nil
	}
	res, err := initialize.Apply(ctx, req, p)
	return &p, &res, err
}
