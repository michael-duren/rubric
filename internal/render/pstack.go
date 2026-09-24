package render

import (
	"bytes"
	"io/fs"
	"path"

	"github.com/michael-duren/go-skills/internal/config"
)

const pstackRoot = "templates/pstack"

func pstackFiles(c config.Config) ([]File, error) {
	if !c.Tooling.Skills {
		return nil, nil
	}
	var files []File
	err := fs.WalkDir(templates, pstackRoot, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := templates.ReadFile(name)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if bytes.HasPrefix(data, []byte("#!")) {
			mode = 0o755
		}
		rel := name[len(pstackRoot)+1:]
		files = append(files, File{Path: path.Join(".agents", rel), Data: data, Mode: mode, Kind: KindManaged})
		return nil
	})
	return files, err
}
