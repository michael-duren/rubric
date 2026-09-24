package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var errSymlink = errors.New("path passes through a symlink")

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Capture snapshots rel beneath root without following symlinks in the target or its parents.
func Capture(root, rel string) (Snapshot, error) {
	if err := checkParents(root, rel); err != nil {
		return Snapshot{}, err
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return Snapshot{Hash: digest(nil)}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	snap := Snapshot{Exists: true, Mode: info.Mode()}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return snap, errSymlink
	case !info.Mode().IsRegular():
		return snap, fmt.Errorf("%s is not a regular file", rel)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	snap.Data = data
	snap.Hash = digest(data)
	return snap, nil
}

func checkParents(root, rel string) error {
	parts := strings.Split(rel, "/")
	dir := root
	for _, part := range parts[:len(parts)-1] {
		dir = filepath.Join(dir, part)
		info, err := os.Lstat(dir)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return errSymlink
		}
		if !info.IsDir() {
			return fmt.Errorf("parent %s is not a directory", part)
		}
	}
	return nil
}
