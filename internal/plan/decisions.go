package plan

import (
	"fmt"
	"maps"
	"slices"

	"github.com/michael-duren/go-skills/internal/render"
)

// Decide resolves conflicts with replace or skip decisions keyed by path and returns a new plan.
func Decide(p Plan, decisions map[string]string) (Plan, error) {
	out := p
	out.Actions = slices.Clone(p.Actions)
	for _, rel := range slices.Sorted(maps.Keys(decisions)) {
		decision := decisions[rel]
		i := slices.IndexFunc(out.Actions, func(a Action) bool { return a.File.Path == rel })
		if i < 0 {
			return Plan{}, fmt.Errorf("decision for %s: path is not in the plan", rel)
		}
		a := &out.Actions[i]
		if a.State != StateConflict {
			return Plan{}, fmt.Errorf("decision for %s: path is not in conflict", rel)
		}
		switch decision {
		case DecisionReplace:
			if a.fixed {
				return Plan{}, fmt.Errorf("decision for %s: cannot replace: %s", rel, a.Reason)
			}
			a.State, a.Reason = StateUpdate, "replace existing content by explicit decision"
			if a.Obsolete {
				a.State, a.Reason = StateDelete, "delete edited file by explicit decision"
			}
		case DecisionSkip:
			if a.Obsolete {
				a.State, a.Reason = StateSkip, "keep edited file by explicit decision; Rubric stops managing it"
				continue
			}
			if mandatory(a.File) {
				return Plan{}, fmt.Errorf("decision for %s: required Rubric file cannot be skipped", rel)
			}
			a.State, a.Reason = StateSkip, "keep existing content by explicit decision"
		default:
			return Plan{}, fmt.Errorf("decision for %s: %q must be %s or %s", rel, decision, DecisionReplace, DecisionSkip)
		}
	}
	return refreshManifest(out)
}

func mandatory(f render.File) bool {
	return f.Kind == render.KindConfig || f.Kind == render.KindGuidance || f.Path == ManifestPath || f.Path == ".rubric/style.md"
}

// Mandatory reports whether a file must be written for initialization to complete.
func Mandatory(a Action) bool {
	return mandatory(a.File)
}
