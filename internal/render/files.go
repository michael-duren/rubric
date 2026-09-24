package render

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

const (
	// KindScaffold marks initial application files that are never regenerated over user edits.
	KindScaffold = "scaffold"
	// KindManaged marks files Rubric owns completely and may refresh while unedited.
	KindManaged = "managed"
	// KindGuidance marks a managed section inside a user-owned document such as AGENTS.md.
	KindGuidance = "guidance"
	// KindConfig marks the user-editable rubric.yaml.
	KindConfig = "config"
)

// File is one rendered output relative to the target directory.
type File struct {
	Path string      `json:"path"`
	Data []byte      `json:"-"`
	Mode fs.FileMode `json:"mode"`
	Kind string      `json:"kind"`
}

func finish(files []File) ([]File, error) {
	slices.SortFunc(files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	for i, f := range files {
		if !fs.ValidPath(f.Path) || f.Path == "." || path.Clean(f.Path) != f.Path || strings.Contains(f.Path, `\`) {
			return nil, fmt.Errorf("render: invalid output path %q", f.Path)
		}
		if i > 0 && files[i-1].Path == f.Path {
			return nil, fmt.Errorf("render: duplicate output path %q", f.Path)
		}
	}
	return files, nil
}
