package write

import (
	"fmt"
	"os"
	"path"
	"slices"
)

func (w *writer) fail(cause error) (Result, error) {
	res := w.rollback()
	return res, &ApplyError{Cause: cause, Result: res}
}

func (w *writer) rollback() Result {
	res := emptyResult()
	for _, e := range slices.Backward(w.journal.entries) {
		if w.restore(e) {
			res.Restored = append(res.Restored, e.path)
		} else {
			res.Unrecovered = append(res.Unrecovered, e.path)
		}
	}
	for _, dir := range slices.Backward(w.journal.dirs) {
		_ = w.ops.remove(w.anc.root, dir)
	}
	slices.Sort(res.Restored)
	slices.Sort(res.Unrecovered)
	return res
}

func (w *writer) restore(e entry) bool {
	name := w.anc.name(e.path)
	if noSymlinks(w.ops, w.anc, e.path) != nil {
		return false
	}
	data, exists, err := current(w.ops, w.anc, e.path)
	switch {
	case err != nil, e.removed && exists:
		return false
	case !e.removed && (!exists || digest(data) != e.written):
		return false
	}
	if !e.existed {
		return w.ops.remove(w.anc.root, name) == nil
	}
	w.seq++
	tmp := path.Join(path.Dir(name), fmt.Sprintf(".%s.rubric-tmp-%d-%d", path.Base(name), os.Getpid(), w.seq))
	mode := e.oldMode
	if mode == 0 {
		mode = 0o644
	}
	if err := w.ops.write(w.anc.root, tmp, e.old, mode); err != nil {
		return false
	}
	if w.ops.chmod(w.anc.root, tmp, mode) != nil || w.ops.rename(w.anc.root, tmp, name) != nil {
		_ = w.ops.remove(w.anc.root, tmp)
		return false
	}
	return true
}
