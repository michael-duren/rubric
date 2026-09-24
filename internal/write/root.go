package write

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type operations struct {
	read   func(r *os.Root, name string) ([]byte, error)
	stat   func(r *os.Root, name string) (fs.FileInfo, error)
	write  func(r *os.Root, name string, data []byte, perm fs.FileMode) error
	rename func(r *os.Root, oldname, newname string) error
	remove func(r *os.Root, name string) error
	mkdir  func(r *os.Root, name string, perm fs.FileMode) error
	chmod  func(r *os.Root, name string, mode fs.FileMode) error
}

func defaultOperations() operations {
	return operations{
		read: (*os.Root).ReadFile,
		stat: (*os.Root).Lstat,
		write: func(r *os.Root, name string, data []byte, perm fs.FileMode) error {
			f, err := r.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
			if err != nil {
				return err
			}
			_, werr := f.Write(data)
			return errors.Join(werr, f.Close())
		},
		rename: (*os.Root).Rename,
		remove: (*os.Root).Remove,
		mkdir:  (*os.Root).Mkdir,
		chmod:  (*os.Root).Chmod,
	}
}

type anchor struct {
	root    *os.Root
	prefix  string
	missing []string
}

func openAnchor(target string) (anchor, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return anchor{}, err
	}
	dir := abs
	var missing []string
	for {
		info, err := os.Stat(dir)
		if err == nil {
			if !info.IsDir() {
				return anchor{}, fmt.Errorf("%s is not a directory", dir)
			}
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return anchor{}, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return anchor{}, fmt.Errorf("no existing ancestor for %s", abs)
		}
		missing = append([]string{filepath.Base(dir)}, missing...)
		dir = parent
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		return anchor{}, err
	}
	return anchor{root: r, prefix: path.Join(missing...), missing: missing}, nil
}

func (a anchor) name(rel string) string {
	if a.prefix == "" {
		return rel
	}
	return a.prefix + "/" + rel
}

func checkRel(rel string) error {
	if rel == "" || !fs.ValidPath(rel) || rel == "." || path.Clean(rel) != rel || strings.Contains(rel, `\`) {
		return fmt.Errorf("invalid target path %q", rel)
	}
	return nil
}

func parents(rel string) []string {
	var out []string
	for dir := path.Dir(rel); dir != "."; dir = path.Dir(dir) {
		out = append([]string{dir}, out...)
	}
	return out
}

func noSymlinks(ops operations, a anchor, rel string) error {
	for _, dir := range append(parents(a.name(rel)), a.name(rel)) {
		info, err := ops.stat(a.root, dir)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s: refusing to write through a symlink", rel)
		}
		if dir != a.name(rel) && !info.IsDir() {
			return fmt.Errorf("%s: parent %s is not a directory", rel, dir)
		}
	}
	return nil
}

func current(ops operations, a anchor, rel string) ([]byte, bool, error) {
	info, err := ops.stat(a.root, a.name(rel))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, true, fmt.Errorf("%s is not a regular file", rel)
	}
	data, err := ops.read(a.root, a.name(rel))
	return data, true, err
}
