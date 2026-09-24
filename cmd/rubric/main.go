// Command rubric sets up Go projects with tested templates and accurate agent guidance.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/x/term"

	"github.com/michael-duren/go-skills/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, os.Args[1:], cli.Streams{
		In: os.Stdin, Out: os.Stdout, Err: os.Stderr,
		Terminal: term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()),
	})
	stop()
	os.Exit(code)
}
