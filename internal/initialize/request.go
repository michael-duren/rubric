// Package initialize is the prepare and apply service shared by the CLI and the wizard.
package initialize

import (
	"strings"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/plan"
)

// Request is one invocation's inputs; Target, Mode, and Decisions are never persisted.
type Request struct {
	Target    string            `json:"target"`
	Mode      string            `json:"mode"`
	Input     []byte            `json:"-"`
	Overrides config.Patch      `json:"overrides"`
	Decisions map[string]string `json:"decisions"`
}

// InputError reports invalid choices, configuration, or targets that the user must correct.
type InputError struct {
	Cause error
}

// Error returns the underlying cause's message.
func (e *InputError) Error() string {
	return e.Cause.Error()
}

// Unwrap returns the underlying cause.
func (e *InputError) Unwrap() error {
	return e.Cause
}

// ConflictError reports a plan that still has undecided conflicts.
type ConflictError struct {
	Plan plan.Plan
}

// Error lists each conflicting path and why it conflicts.
func (e *ConflictError) Error() string {
	conflicts := plan.Conflicts(e.Plan)
	paths := make([]string, len(conflicts))
	for i, a := range conflicts {
		paths[i] = a.File.Path + " (" + a.Reason + ")"
	}
	return "unresolved conflicts: " + strings.Join(paths, "; ")
}
