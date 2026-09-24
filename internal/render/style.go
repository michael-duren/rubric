package render

import (
	"github.com/michael-duren/go-skills/internal/config"
	"github.com/michael-duren/go-skills/internal/style"
)

const styleDir = ".rubric/style/"

func styleFiles(c config.Config) ([]File, error) {
	if !c.Tooling.Lint {
		return nil, nil
	}
	sources, err := style.Sources()
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(sources))
	for _, s := range sources {
		files = append(files, File{Path: styleDir + s.Name, Data: s.Data, Mode: 0o644, Kind: KindManaged})
	}
	return files, nil
}
