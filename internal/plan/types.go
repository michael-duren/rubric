// Package plan classifies rendered files against the target directory without writing anything.
package plan

import (
	"io/fs"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
)

const (
	// StateCreate writes a file that does not exist yet.
	StateCreate = "create"
	// StateUpdate replaces reviewed existing content.
	StateUpdate = "update"
	// StateUnchanged leaves identical content alone.
	StateUnchanged = "unchanged"
	// StateConflict blocks application until the user decides.
	StateConflict = "conflict"
	// StateSkip leaves the existing file untouched by explicit decision.
	StateSkip = "skip"

	// DecisionReplace resolves a conflict by writing the proposed content.
	DecisionReplace = "replace"
	// DecisionSkip resolves a conflict by keeping the existing file.
	DecisionSkip = "skip"
)

// Plan is the reviewed set of file actions for one invocation.
type Plan struct {
	Mode     string        `json:"mode"`
	Config   config.Config `json:"config"`
	Actions  []Action      `json:"actions"`
	Obsolete []string      `json:"obsolete"`

	previous Manifest
}

// Action is the proposed change for one path and the snapshot it was reviewed against.
type Action struct {
	File   render.File `json:"file"`
	Before Snapshot    `json:"-"`
	State  string      `json:"state"`
	Reason string      `json:"reason"`

	fixed bool
}

// Snapshot captures a path's content at preview time so later writes can detect concurrent edits.
type Snapshot struct {
	Exists bool        `json:"exists"`
	Data   []byte      `json:"-"`
	Mode   fs.FileMode `json:"mode"`
	Hash   string      `json:"hash"`
}

// Conflicts returns the actions still awaiting a decision.
func Conflicts(p Plan) []Action {
	var out []Action
	for _, a := range p.Actions {
		if a.State == StateConflict {
			out = append(out, a)
		}
	}
	return out
}
