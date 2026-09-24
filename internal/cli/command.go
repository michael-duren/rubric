// Package cli parses rubric arguments, runs the initialization service, and reports results.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"

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
	root.AddCommand(initCommand(s))
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
		fmt.Fprintf(s.Err, "rubric: %v\n", err)
	}
	return code
}

func ranInit(cmd *cobra.Command) bool {
	return cmd != nil && cmd.Annotations["ran"] == "true"
}

func reported(cmd *cobra.Command) bool {
	return cmd != nil && cmd.Annotations["reported"] == "true"
}

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
		if o.format != "text" && o.format != "json" {
			return &usageError{err: fmt.Errorf("--format: %q must be text or json", o.format)}
		}
		target := "."
		if len(args) == 1 {
			target = args[0]
		}
		c.Annotations["reported"] = "true"
		if s.Terminal && o.format == "text" && !o.nonInteractive && !o.dryRun {
			p, res, err := interactive(c.Context(), c, o, target, s)
			writeText(s.Out, s.Err, newReport(target, false, p, res, err))
			return err
		}
		p, res, err := execute(c.Context(), c, o, target)
		r := newReport(target, o.dryRun, p, res, err)
		if o.format == "json" {
			if werr := writeJSON(s.Out, r); werr != nil {
				return werr
			}
		} else {
			writeText(s.Out, s.Err, r)
		}
		return err
	}
	return cmd
}

var runWizard = wizard.Run

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

func interactive(ctx context.Context, c *cobra.Command, o options, target string, s Streams) (*plan.Plan, *write.Result, error) {
	req, err := request(c, o, target)
	if err != nil {
		return nil, nil, err
	}
	outcome, err := runWizard(ctx, req, s.In, s.Out)
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
