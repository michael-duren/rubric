package render

import (
	"bytes"
	"io/fs"
	"path"
	"strings"

	"github.com/michael-duren/go-skills/internal/config"
)

const (
	pstackRoot    = "templates/pstack"
	pstackNotices = ".agents/skills/THIRD_PARTY_NOTICES.md"
)

// SkillGroup returns the skill group that installs the Rubric-generated path rel, or "" for other paths.
func SkillGroup(rel string) string {
	switch {
	case strings.HasPrefix(rel, ".agents/skills/rubric-"):
		return config.SkillsRubric
	case strings.HasPrefix(rel, ".agents/skills/principle-"), rel == pstackNotices:
		return config.SkillsPrinciples
	case strings.HasPrefix(rel, ".agents/agents/"):
		return config.SkillsAgents
	case strings.HasPrefix(rel, ".agents/skills/"):
		return config.SkillsPstack
	}
	return ""
}

func pstackFiles(c config.Config) ([]File, error) {
	var files []File
	err := fs.WalkDir(templates, pstackRoot, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := path.Join(".agents", name[len(pstackRoot)+1:])
		if !c.Tooling.Has(SkillGroup(rel)) {
			return nil
		}
		data, err := templates.ReadFile(name)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if bytes.HasPrefix(data, []byte("#!")) {
			mode = 0o755
		}
		files = append(files, File{Path: rel, Data: data, Mode: mode, Kind: KindManaged})
		return nil
	})
	return files, err
}
