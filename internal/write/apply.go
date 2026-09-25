// Package write applies a reviewed plan inside the target directory and restores prior content on failure.
package write

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/michael-duren/go-skills/internal/plan"
)

// Result lists paths written or deleted, paths restored after a failure, and paths that could not be restored safely.
type Result struct {
	Applied     []string `json:"applied"`
	Deleted     []string `json:"deleted"`
	Restored    []string `json:"restored"`
	Unrecovered []string `json:"unrecovered"`
}

// ConflictError reports unresolved or stale paths detected before replacement.
type ConflictError struct {
	Paths []string
}

// Error lists the conflicting paths.
func (e *ConflictError) Error() string {
	return "unresolved conflicts: " + strings.Join(e.Paths, ", ")
}

// ApplyError reports a failure after writing began, with the recovery outcome.
type ApplyError struct {
	Cause  error
	Result Result
}

// Error describes the failure and which paths were or were not restored.
func (e *ApplyError) Error() string {
	msg := fmt.Sprintf("apply failed: %v", e.Cause)
	if len(e.Result.Restored) > 0 {
		msg += fmt.Sprintf("; restored %s", strings.Join(e.Result.Restored, ", "))
	}
	if len(e.Result.Unrecovered) > 0 {
		msg += fmt.Sprintf("; not restored because they changed after Rubric wrote them: %s; review them by hand",
			strings.Join(e.Result.Unrecovered, ", "))
	}
	return msg
}

// Unwrap returns the failure cause.
func (e *ApplyError) Unwrap() error {
	return e.Cause
}

// Apply deletes, then writes the plan's create and update actions beneath target, manifest last.
func Apply(ctx context.Context, target string, p plan.Plan) (Result, error) {
	return applyWithOps(ctx, target, p, defaultOperations())
}

func applyWithOps(ctx context.Context, target string, p plan.Plan, ops operations) (Result, error) {
	res := emptyResult()
	if err := ctx.Err(); err != nil {
		return res, err
	}
	if conflicts := plan.Conflicts(p); len(conflicts) > 0 {
		paths := make([]string, len(conflicts))
		for i, a := range conflicts {
			paths[i] = a.File.Path
		}
		return res, &ConflictError{Paths: paths}
	}
	actions := pending(p)
	for _, a := range actions {
		if err := checkRel(a.File.Path); err != nil {
			return res, err
		}
	}
	if len(actions) == 0 {
		return res, nil
	}
	anc, err := openAnchor(target)
	if err != nil {
		return res, err
	}
	defer func() { _ = anc.root.Close() }()
	if err := preflight(ops, anc, actions); err != nil {
		return res, err
	}
	w := writer{ops: ops, anc: anc, applied: []string{}, deleted: []string{}}
	for _, a := range actions {
		if err := ctx.Err(); err != nil {
			return w.fail(err)
		}
		apply := w.replace
		if a.State == plan.StateDelete {
			apply = w.delete
		}
		if err := apply(a); err != nil {
			return w.fail(err)
		}
	}
	w.prune()
	res.Applied, res.Deleted = w.applied, w.deleted
	return res, nil
}

func emptyResult() Result {
	return Result{Applied: []string{}, Deleted: []string{}, Restored: []string{}, Unrecovered: []string{}}
}

func pending(p plan.Plan) []plan.Action {
	var deletes, writes []plan.Action
	var manifest *plan.Action
	for _, a := range p.Actions {
		switch {
		case a.State == plan.StateDelete:
			deletes = append(deletes, a)
		case a.State != plan.StateCreate && a.State != plan.StateUpdate:
		case a.File.Path == plan.ManifestPath:
			manifest = &a
		default:
			writes = append(writes, a)
		}
	}
	byPath := func(a, b plan.Action) int { return strings.Compare(a.File.Path, b.File.Path) }
	slices.SortFunc(deletes, byPath)
	slices.SortFunc(writes, byPath)
	out := append(deletes, writes...)
	if manifest != nil {
		out = append(out, *manifest)
	}
	return out
}

func unchanged(before plan.Snapshot, current []byte, exists bool) bool {
	return before.Exists == exists && (!exists || bytes.Equal(before.Data, current))
}

func preflight(ops operations, anc anchor, actions []plan.Action) error {
	var stale []string
	for _, a := range actions {
		if err := noSymlinks(ops, anc, a.File.Path); err != nil {
			return err
		}
		data, exists, err := current(ops, anc, a.File.Path)
		if err != nil {
			return err
		}
		if !unchanged(a.Before, data, exists) {
			stale = append(stale, a.File.Path)
		}
	}
	if len(stale) > 0 {
		return &ConflictError{Paths: stale}
	}
	return nil
}

type writer struct {
	ops     operations
	anc     anchor
	journal journal
	applied []string
	deleted []string
	seq     int
}

func (w *writer) mkdirs(rel string) error {
	var dirs []string
	for i := range w.anc.missing {
		dirs = append(dirs, path.Join(w.anc.missing[:i+1]...))
	}
	for _, dir := range parents(rel) {
		dirs = append(dirs, w.anc.name(dir))
	}
	for _, dir := range dirs {
		info, err := w.ops.stat(w.anc.root, dir)
		if err == nil {
			if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("%s: not a directory", dir)
			}
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := w.ops.mkdir(w.anc.root, dir, 0o755); err != nil {
			return err
		}
		w.journal.dirs = append(w.journal.dirs, dir)
	}
	return nil
}

func (w *writer) replace(a plan.Action) error {
	rel := a.File.Path
	if err := w.mkdirs(rel); err != nil {
		return err
	}
	name := w.anc.name(rel)
	w.seq++
	tmp := path.Join(path.Dir(name), fmt.Sprintf(".%s.rubric-tmp-%d-%d", path.Base(name), os.Getpid(), w.seq))
	mode := a.File.Mode.Perm()
	if mode == 0 {
		mode = 0o644
	}
	if err := w.ops.write(w.anc.root, tmp, a.File.Data, mode); err != nil {
		return err
	}
	staged := true
	defer func() {
		if staged {
			_ = w.ops.remove(w.anc.root, tmp)
		}
	}()
	if err := w.ops.chmod(w.anc.root, tmp, mode); err != nil {
		return err
	}
	if err := noSymlinks(w.ops, w.anc, rel); err != nil {
		return err
	}
	old, exists, err := current(w.ops, w.anc, rel)
	if err != nil {
		return err
	}
	if !unchanged(a.Before, old, exists) {
		return &ConflictError{Paths: []string{rel}}
	}
	if err := w.ops.rename(w.anc.root, tmp, name); err != nil {
		return err
	}
	staged = false
	w.journal.entries = append(w.journal.entries, entry{
		path: rel, existed: exists, old: old, oldMode: a.Before.Mode.Perm(), written: digest(a.File.Data),
	})
	w.applied = append(w.applied, rel)
	return nil
}

func (w *writer) delete(a plan.Action) error {
	rel := a.File.Path
	if err := noSymlinks(w.ops, w.anc, rel); err != nil {
		return err
	}
	old, exists, err := current(w.ops, w.anc, rel)
	if err != nil {
		return err
	}
	if !exists || !unchanged(a.Before, old, exists) {
		return &ConflictError{Paths: []string{rel}}
	}
	if err := w.ops.remove(w.anc.root, w.anc.name(rel)); err != nil {
		return err
	}
	w.journal.entries = append(w.journal.entries, entry{path: rel, existed: true, old: old, oldMode: a.Before.Mode.Perm(), removed: true})
	w.deleted = append(w.deleted, rel)
	return nil
}

func (w *writer) prune() {
	seen := map[string]bool{}
	var dirs []string
	for _, rel := range w.deleted {
		for _, dir := range parents(rel) {
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
	}
	slices.SortFunc(dirs, func(a, b string) int { return strings.Count(b, "/") - strings.Count(a, "/") })
	for _, dir := range dirs {
		_ = w.ops.remove(w.anc.root, w.anc.name(dir))
	}
}
