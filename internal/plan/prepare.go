package plan

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/render"
)

// Prepare compares rendered files for target with what exists there and returns a write-free preview.
func Prepare(target, mode string, cfg config.Config, files []render.File) (Plan, error) {
	seen := map[string]bool{}
	for _, f := range files {
		if !fs.ValidPath(f.Path) || f.Path == "." || path.Clean(f.Path) != f.Path || strings.Contains(f.Path, `\`) {
			return Plan{}, fmt.Errorf("plan: invalid path %q", f.Path)
		}
		if f.Path == ManifestPath || seen[f.Path] {
			return Plan{}, fmt.Errorf("plan: duplicate or reserved path %q", f.Path)
		}
		seen[f.Path] = true
	}
	manifestSnap, err := Capture(target, ManifestPath)
	if err != nil {
		return Plan{}, fmt.Errorf("plan: %s: %w", ManifestPath, err)
	}
	previous, err := parseManifest(manifestSnap)
	if err != nil {
		return Plan{}, fmt.Errorf("plan: %w", err)
	}
	p := Plan{Mode: mode, Config: cfg, Actions: []Action{}, previous: previous}
	for _, f := range files {
		a, err := classify(target, f, previous)
		if err != nil {
			return Plan{}, fmt.Errorf("plan: %s: %w", f.Path, err)
		}
		p.Actions = append(p.Actions, a)
	}
	for _, rel := range slices.Sorted(maps.Keys(previous.Files)) {
		if seen[rel] {
			continue
		}
		a, err := retire(target, rel, previous.Files[rel])
		if err != nil {
			return Plan{}, fmt.Errorf("plan: %s: %w", rel, err)
		}
		if a.Before.Exists {
			p.Actions = append(p.Actions, a)
		}
	}
	manifestAction := Action{
		File:   render.File{Path: ManifestPath, Mode: 0o644, Kind: render.KindManaged},
		Before: manifestSnap,
	}
	p.Actions = append(p.Actions, manifestAction)
	slices.SortFunc(p.Actions, func(a, b Action) int { return strings.Compare(a.File.Path, b.File.Path) })
	return refreshManifest(p)
}

func classify(target string, f render.File, previous Manifest) (Action, error) {
	a := Action{File: f}
	snap, err := Capture(target, f.Path)
	if err != nil {
		if errors.Is(err, errSymlink) {
			a.State, a.Reason, a.fixed = StateConflict, "refusing to write through a symlink", true
			a.Before = snap
			return a, nil
		}
		if snap.Exists || strings.Contains(err.Error(), "not a directory") {
			a.State, a.Reason, a.fixed = StateConflict, err.Error(), true
			a.Before = snap
			return a, nil
		}
		return Action{}, err
	}
	a.Before = snap
	if !snap.Exists {
		a.State, a.Reason = StateCreate, "new file"
		return a, nil
	}
	stamp, owned := previous.Files[f.Path]
	switch f.Kind {
	case render.KindConfig:
		if bytes.Equal(snap.Data, f.Data) {
			a.State, a.Reason = StateUnchanged, "settings unchanged"
		} else {
			a.State, a.Reason = StateUpdate, "update settings from current rubric.yaml and inputs"
		}
	case render.KindGuidance:
		parts, err := splitGuidance(snap.Data)
		if err != nil {
			a.State, a.Reason, a.fixed = StateConflict, err.Error(), true
			return a, nil
		}
		proposed := parts.with(f.Data)
		a.File.Data = proposed
		switch {
		case !parts.found:
			a.State, a.Reason = StateUpdate, "append managed section; content outside markers is preserved"
		case bytes.Equal(parts.managed, f.Data):
			a.State, a.Reason = StateUnchanged, "managed section unchanged"
		case owned && stamp.Hash == digest(parts.managed):
			a.State, a.Reason = StateUpdate, "refresh managed section; content outside markers is preserved"
		default:
			a.State, a.Reason = StateConflict, "managed section was edited since Rubric wrote it"
		}
	default:
		switch {
		case bytes.Equal(snap.Data, f.Data):
			a.State, a.Reason = StateUnchanged, "identical content"
		case f.Kind == render.KindManaged && owned && stamp.Hash == snap.Hash:
			a.State, a.Reason = StateUpdate, "refresh Rubric-managed file"
		case owned:
			a.State, a.Reason = StateConflict, "file was edited since Rubric wrote it"
		default:
			a.State, a.Reason = StateConflict, "existing file differs and is not managed by Rubric"
		}
	}
	return a, nil
}

func retire(target, rel string, stamp Stamp) (Action, error) {
	a := Action{File: render.File{Path: rel, Kind: stamp.Kind}, Obsolete: true}
	snap, err := Capture(target, rel)
	a.Before = snap
	switch {
	case err != nil && (snap.Exists || errors.Is(err, errSymlink)):
		a.Before.Exists = true
		a.State, a.Reason, a.fixed = StateConflict, "no longer generated: "+err.Error(), true
	case err != nil && strings.Contains(err.Error(), "not a directory"):
	case err != nil:
		return Action{}, err
	case !snap.Exists:
	case snap.Hash == stamp.Hash:
		a.State, a.Reason = StateDelete, "no longer generated; unedited since Rubric wrote it"
	default:
		a.State, a.Reason = StateConflict, "no longer generated, but edited since Rubric wrote it"
	}
	return a, nil
}

func ownedBytes(f render.File) []byte {
	if f.Kind != render.KindGuidance {
		return f.Data
	}
	parts, err := splitGuidance(f.Data)
	if err != nil || !parts.found {
		return f.Data
	}
	return parts.managed
}

func refreshManifest(p Plan) (Plan, error) {
	next := Manifest{Version: manifestVersion, Files: map[string]Stamp{}}
	idx := -1
	for i, a := range p.Actions {
		if a.File.Path == ManifestPath {
			idx = i
			continue
		}
		if a.Obsolete {
			if a.State == StateConflict {
				next.Files[a.File.Path] = p.previous.Files[a.File.Path]
			}
			continue
		}
		if a.File.Kind != render.KindManaged && a.File.Kind != render.KindGuidance {
			continue
		}
		old, owned := p.previous.Files[a.File.Path]
		switch {
		case a.State == StateCreate || a.State == StateUpdate:
			next.Files[a.File.Path] = Stamp{Hash: digest(ownedBytes(a.File)), Kind: a.File.Kind}
		case a.State == StateUnchanged && owned:
			next.Files[a.File.Path] = Stamp{Hash: digest(ownedBytes(a.File)), Kind: a.File.Kind}
		case owned:
			next.Files[a.File.Path] = old
		}
	}
	data, err := next.encode()
	if err != nil {
		return Plan{}, fmt.Errorf("plan: encode manifest: %w", err)
	}
	m := &p.Actions[idx]
	m.File.Data = data
	switch {
	case !m.Before.Exists:
		m.State, m.Reason = StateCreate, "record ownership of Rubric-written content"
	case bytes.Equal(m.Before.Data, data):
		m.State, m.Reason = StateUnchanged, "ownership unchanged"
	default:
		m.State, m.Reason = StateUpdate, "record ownership of Rubric-written content"
	}
	return p, nil
}
