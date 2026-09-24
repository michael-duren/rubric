package plan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
)

// ManifestPath is where Rubric records hashes of content it wrote.
const ManifestPath = ".rubric/manifest.json"

const manifestVersion = 1

var hexHash = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Manifest records ownership of Rubric-written content; it never lists itself or rubric.yaml.
type Manifest struct {
	Version int              `json:"version"`
	Files   map[string]Stamp `json:"files"`
}

// Stamp is the SHA-256 of the last content Rubric wrote for a path and that content's kind.
type Stamp struct {
	Hash string `json:"hash"`
	Kind string `json:"kind"`
}

func parseManifest(snap Snapshot) (Manifest, error) {
	if !snap.Exists {
		return Manifest{Version: manifestVersion, Files: map[string]Stamp{}}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(snap.Data))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("corrupt %s: %w", ManifestPath, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("corrupt %s: trailing data", ManifestPath)
	}
	if m.Version != manifestVersion {
		return Manifest{}, fmt.Errorf("corrupt %s: unsupported version %d", ManifestPath, m.Version)
	}
	if m.Files == nil {
		m.Files = map[string]Stamp{}
	}
	for p, s := range m.Files {
		if !fs.ValidPath(p) || p == "." || path.Clean(p) != p || p == ManifestPath || p == "rubric.yaml" {
			return Manifest{}, fmt.Errorf("corrupt %s: invalid path %q", ManifestPath, p)
		}
		if !hexHash.MatchString(s.Hash) {
			return Manifest{}, fmt.Errorf("corrupt %s: invalid hash for %q", ManifestPath, p)
		}
	}
	return m, nil
}

func (m Manifest) encode() ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
