package cli

import (
	"context"
	"errors"

	"github.com/michael-duren/go-skills/internal/initialize"
	"github.com/michael-duren/go-skills/internal/write"
)

const (
	exitOK        = 0
	exitFailure   = 1
	exitInvalid   = 2
	exitCancelled = 130
)

type usageError struct {
	err error
}

func (e *usageError) Error() string {
	return e.err.Error()
}

func (e *usageError) Unwrap() error {
	return e.err
}

func exitCode(err error) int {
	var input *initialize.InputError
	var conflict *initialize.ConflictError
	var stale *write.ConflictError
	var usage *usageError
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, context.Canceled):
		return exitCancelled
	case errors.As(err, &input), errors.As(err, &conflict), errors.As(err, &stale), errors.As(err, &usage):
		return exitInvalid
	}
	return exitFailure
}

func status(err error, dryRun bool) string {
	switch exitCode(err) {
	case exitOK:
		if dryRun {
			return "dry-run"
		}
		return "ok"
	case exitCancelled:
		return "cancelled"
	case exitInvalid:
		var conflict *initialize.ConflictError
		var stale *write.ConflictError
		if errors.As(err, &conflict) || errors.As(err, &stale) {
			return "conflict"
		}
		return "invalid"
	}
	return "error"
}
