package plan

import (
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return out
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		value := info.Mode().String()
		if d.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += string(data)
		}
		out[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func prepare(t *testing.T, root, mode string, files []render.File) Plan {
	t.Helper()
	before := tree(t, root)
	p, err := Prepare(root, mode, testproject.Config(), files)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(before, tree(t, root)) {
		t.Fatal("preview modified the target")
	}
	return p
}

func put(t *testing.T, root, rel, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func apply(t *testing.T, root string, p Plan) {
	t.Helper()
	for _, a := range p.Actions {
		if a.State == StateCreate || a.State == StateUpdate {
			put(t, root, a.File.Path, string(a.File.Data))
		}
	}
}

func action(t *testing.T, p Plan, path string) Action {
	t.Helper()
	for _, a := range p.Actions {
		if a.File.Path == path {
			return a
		}
	}
	t.Fatalf("no action for %s in %+v", path, p.Actions)
	return Action{}
}

func manifestOf(t *testing.T, p Plan) Manifest {
	t.Helper()
	var m Manifest
	if err := json.Unmarshal(action(t, p, ManifestPath).File.Data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func managed(path, data string) render.File {
	return render.File{Path: path, Data: []byte(data), Mode: 0o644, Kind: render.KindManaged}
}

func section(body string) string {
	return "<!-- rubric:begin -->\n" + body + "<!-- rubric:end -->\n"
}

func guidance(body string) render.File {
	return render.File{Path: "AGENTS.md", Data: []byte(section(body)), Mode: 0o644, Kind: render.KindGuidance}
}

func TestExistingUserFileIsAConflict(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Makefile")
	if err := os.WriteFile(path, []byte("user-target:\n\ttrue\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := []render.File{{Path: "Makefile", Data: []byte("new-target:\n"), Mode: 0o644, Kind: "managed"}}
	p, err := Prepare(root, "existing", testproject.Config(), files)
	if err != nil {
		t.Fatal(err)
	}
	if len(Conflicts(p)) != 1 {
		t.Fatalf("want one conflict: %+v", p.Actions)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "user-target:\n\ttrue\n" {
		t.Fatal("preview wrote a file")
	}
}

func TestCreateIntoMissingTarget(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new project")
	p := prepare(t, root, "new", []render.File{managed("a.txt", "a")})
	if got := action(t, p, "a.txt"); got.State != StateCreate || got.Before.Exists {
		t.Fatalf("action = %+v", got)
	}
	if got := action(t, p, ManifestPath); got.State != StateCreate {
		t.Fatalf("manifest = %+v", got)
	}
	if m := manifestOf(t, p); m.Version != 1 || m.Files["a.txt"].Hash != digest([]byte("a")) {
		t.Fatalf("manifest = %+v", m)
	}
	if p.Mode != "new" || p.Config.Project.Module != "example.com/demo" {
		t.Fatalf("plan header = %q %+v", p.Mode, p.Config.Project)
	}
}

func TestIdenticalUnmanagedFileIsUnchangedButNotOwned(t *testing.T) {
	root := t.TempDir()
	put(t, root, "Makefile", "same\n")
	p := prepare(t, root, "existing", []render.File{managed("Makefile", "same\n")})
	if got := action(t, p, "Makefile"); got.State != StateUnchanged {
		t.Fatalf("action = %+v", got)
	}
	if _, owned := manifestOf(t, p).Files["Makefile"]; owned {
		t.Fatal("identical user file adopted into manifest")
	}
}

func TestOwnedManagedFileUpdatesWhenUnedited(t *testing.T) {
	root := t.TempDir()
	first := prepare(t, root, "existing", []render.File{managed("Makefile", "v1\n")})
	apply(t, root, first)
	second := prepare(t, root, "existing", []render.File{managed("Makefile", "v2\n")})
	if got := action(t, second, "Makefile"); got.State != StateUpdate {
		t.Fatalf("action = %+v", got)
	}
	if manifestOf(t, second).Files["Makefile"].Hash != digest([]byte("v2\n")) {
		t.Fatal("manifest not refreshed")
	}
}

func TestEditedManagedFileIsConflict(t *testing.T) {
	root := t.TempDir()
	apply(t, root, prepare(t, root, "existing", []render.File{managed("Makefile", "v1\n")}))
	put(t, root, "Makefile", "v1 edited\n")
	p := prepare(t, root, "existing", []render.File{managed("Makefile", "v2\n")})
	if got := action(t, p, "Makefile"); got.State != StateConflict || !strings.Contains(got.Reason, "edited") {
		t.Fatalf("action = %+v", got)
	}
	if manifestOf(t, p).Files["Makefile"].Hash != digest([]byte("v1\n")) {
		t.Fatal("conflicted file lost its previous stamp")
	}
}

func TestUnchangedRerunIsNoOp(t *testing.T) {
	root := t.TempDir()
	files := []render.File{
		managed("Makefile", "v1\n"),
		guidance("rules\n"),
		{Path: "rubric.yaml", Data: []byte("schema: 1\n"), Mode: 0o644, Kind: render.KindConfig},
		{Path: "go.mod", Data: []byte("module x\n"), Mode: 0o644, Kind: render.KindScaffold},
	}
	apply(t, root, prepare(t, root, "new", files))
	p := prepare(t, root, "existing", files)
	for _, a := range p.Actions {
		if a.State != StateUnchanged {
			t.Errorf("%s = %s (%s)", a.File.Path, a.State, a.Reason)
		}
	}
}

func TestAbsentManifestTreatsExistingDifferingFilesAsConflicts(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".rubric/style.md", "old\n")
	p := prepare(t, root, "existing", []render.File{managed(".rubric/style.md", "new\n")})
	if got := action(t, p, ".rubric/style.md"); got.State != StateConflict {
		t.Fatalf("action = %+v", got)
	}
}

func TestEditedManifestDoesNotGrantOwnership(t *testing.T) {
	root := t.TempDir()
	put(t, root, "Makefile", "user\n")
	m := Manifest{Version: 1, Files: map[string]Stamp{"Makefile": {Hash: digest([]byte("something else\n")), Kind: "managed"}}}
	data, _ := json.Marshal(m)
	put(t, root, ManifestPath, string(data))
	p := prepare(t, root, "existing", []render.File{managed("Makefile", "gen\n")})
	if got := action(t, p, "Makefile"); got.State != StateConflict {
		t.Fatalf("action = %+v", got)
	}
}

func TestCorruptManifestRejected(t *testing.T) {
	for name, body := range map[string]string{
		"not json":      "{",
		"wrong version": `{"version": 9, "files": {}}`,
		"unknown field": `{"version": 1, "files": {}, "extra": true}`,
		"bad path":      `{"version": 1, "files": {"../x": {"hash": "` + digest(nil) + `", "kind": "managed"}}}`,
		"bad hash":      `{"version": 1, "files": {"x": {"hash": "zz", "kind": "managed"}}}`,
		"self":          `{"version": 1, "files": {".rubric/manifest.json": {"hash": "` + digest(nil) + `", "kind": "managed"}}}`,
		"trailing":      `{"version": 1, "files": {}} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			put(t, root, ManifestPath, body)
			if _, err := Prepare(root, "existing", testproject.Config(), nil); err == nil {
				t.Fatal("corrupt manifest accepted")
			}
		})
	}
}

func TestRubricYAMLEditsAreUpdatesNotConflicts(t *testing.T) {
	root := t.TempDir()
	put(t, root, "rubric.yaml", "# my notes\nschema: 1\n")
	file := render.File{Path: "rubric.yaml", Data: []byte("# my notes\nschema: 1\ntooling: {}\n"), Mode: 0o644, Kind: render.KindConfig}
	p := prepare(t, root, "existing", []render.File{file})
	got := action(t, p, "rubric.yaml")
	if got.State != StateUpdate || string(got.Before.Data) != "# my notes\nschema: 1\n" {
		t.Fatalf("action = %+v", got)
	}
	if _, owned := manifestOf(t, p).Files["rubric.yaml"]; owned {
		t.Fatal("rubric.yaml hashed into manifest")
	}
}

func TestScaffoldCollisionIsConflict(t *testing.T) {
	root := t.TempDir()
	put(t, root, "README.md", "mine\n")
	p := prepare(t, root, "new", []render.File{{Path: "README.md", Data: []byte("gen\n"), Mode: 0o644, Kind: render.KindScaffold}})
	if got := action(t, p, "README.md"); got.State != StateConflict {
		t.Fatalf("action = %+v", got)
	}
	if _, owned := manifestOf(t, p).Files["README.md"]; owned {
		t.Fatal("scaffold stamped")
	}
}

func TestAgentsAppendsSectionPreservingUserContent(t *testing.T) {
	root := t.TempDir()
	put(t, root, "AGENTS.md", "# Team rules\nBe kind.")
	p := prepare(t, root, "existing", []render.File{guidance("generated\n")})
	got := action(t, p, "AGENTS.md")
	want := "# Team rules\nBe kind.\n\n" + section("generated\n")
	if got.State != StateUpdate || string(got.File.Data) != want {
		t.Fatalf("action = %s %q", got.State, got.File.Data)
	}
	if manifestOf(t, p).Files["AGENTS.md"].Hash != digest([]byte(section("generated\n"))) {
		t.Fatal("guidance stamp must hash only the managed section")
	}
}

func TestAgentsReplacesOwnedSectionKeepingSurroundings(t *testing.T) {
	root := t.TempDir()
	apply(t, root, prepare(t, root, "existing", []render.File{guidance("v1\n")}))
	put(t, root, "AGENTS.md", "intro\n\n"+section("v1\n")+"\noutro\n")
	p := prepare(t, root, "existing", []render.File{guidance("v2\n")})
	got := action(t, p, "AGENTS.md")
	if got.State != StateUpdate || string(got.File.Data) != "intro\n\n"+section("v2\n")+"\noutro\n" {
		t.Fatalf("action = %s %q", got.State, got.File.Data)
	}
	if string(got.Before.Data) != "intro\n\n"+section("v1\n")+"\noutro\n" {
		t.Fatal("whole-file snapshot missing")
	}
}

func TestAgentsUserEditOutsideMarkersIsNotConflict(t *testing.T) {
	root := t.TempDir()
	apply(t, root, prepare(t, root, "existing", []render.File{guidance("v1\n")}))
	put(t, root, "AGENTS.md", section("v1\n")+"my extra rule\n")
	p := prepare(t, root, "existing", []render.File{guidance("v1\n")})
	if got := action(t, p, "AGENTS.md"); got.State != StateUnchanged {
		t.Fatalf("action = %+v", got)
	}
}

func TestAgentsEditedSectionIsConflict(t *testing.T) {
	root := t.TempDir()
	apply(t, root, prepare(t, root, "existing", []render.File{guidance("v1\n")}))
	put(t, root, "AGENTS.md", section("v1 hand-edited\n"))
	p := prepare(t, root, "existing", []render.File{guidance("v2\n")})
	got := action(t, p, "AGENTS.md")
	if got.State != StateConflict || string(got.File.Data) != section("v2\n") {
		t.Fatalf("action = %s %q", got.State, got.File.Data)
	}
}

func TestAgentsMalformedMarkers(t *testing.T) {
	for name, body := range map[string]string{
		"begin only": "<!-- rubric:begin -->\nx\n",
		"end first":  "<!-- rubric:end -->\n<!-- rubric:begin -->\n",
		"duplicated": section("a\n") + section("b\n"),
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			put(t, root, "AGENTS.md", body)
			p := prepare(t, root, "existing", []render.File{guidance("v\n")})
			got := action(t, p, "AGENTS.md")
			if got.State != StateConflict || !strings.Contains(got.Reason, "marker") {
				t.Fatalf("action = %+v", got)
			}
			if _, err := Decide(p, map[string]string{"AGENTS.md": DecisionReplace}); err == nil {
				t.Fatal("replace accepted for malformed markers")
			}
		})
	}
}

func TestObsoleteManagedFilesPreservedAndListed(t *testing.T) {
	root := t.TempDir()
	apply(t, root, prepare(t, root, "existing", []render.File{managed("Makefile", "v1\n"), managed("keep.txt", "k\n")}))
	p := prepare(t, root, "existing", []render.File{managed("keep.txt", "k\n")})
	if !slices.Equal(p.Obsolete, []string{"Makefile"}) {
		t.Fatalf("obsolete = %v", p.Obsolete)
	}
	for _, a := range p.Actions {
		if a.File.Path == "Makefile" {
			t.Fatal("obsolete file scheduled for change")
		}
	}
	if _, kept := manifestOf(t, p).Files["Makefile"]; !kept {
		t.Fatal("obsolete stamp dropped")
	}
}

func TestSymlinkTargetsAreConflicts(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "x"), filepath.Join(root, "Makefile")); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".github")); err != nil {
		t.Fatal(err)
	}
	p := prepare(t, root, "existing", []render.File{managed("Makefile", "m\n"), managed(".github/workflows/ci.yml", "c\n")})
	for _, path := range []string{"Makefile", ".github/workflows/ci.yml"} {
		if got := action(t, p, path); got.State != StateConflict || !strings.Contains(got.Reason, "symlink") {
			t.Fatalf("%s = %+v", path, got)
		}
		if _, err := Decide(p, map[string]string{path: DecisionReplace}); err == nil {
			t.Fatalf("replace through symlink accepted for %s", path)
		}
	}
}

func TestInvalidRenderedPathRejected(t *testing.T) {
	for _, path := range []string{"../x", "/abs", "a/../../b", ""} {
		if _, err := Prepare(t.TempDir(), "new", testproject.Config(), []render.File{managed(path, "x")}); err == nil {
			t.Fatalf("path %q accepted", path)
		}
	}
}

func TestSnapshotDetectsConcurrentChange(t *testing.T) {
	root := t.TempDir()
	put(t, root, "Makefile", "v1\n")
	p := prepare(t, root, "existing", []render.File{managed("Makefile", "v2\n")})
	before := action(t, p, "Makefile").Before
	put(t, root, "Makefile", "v1 changed\n")
	now, err := Capture(root, "Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if now.Hash == before.Hash || before.Hash != digest([]byte("v1\n")) {
		t.Fatal("snapshot hash cannot detect change")
	}
}

func TestDecide(t *testing.T) {
	root := t.TempDir()
	put(t, root, "Makefile", "user\n")
	put(t, root, "README.md", "mine\n")
	files := []render.File{
		managed("Makefile", "gen\n"),
		{Path: "README.md", Data: []byte("gen\n"), Mode: 0o644, Kind: render.KindScaffold},
		managed("new.txt", "n\n"),
	}
	p := prepare(t, root, "existing", files)
	decided, err := Decide(p, map[string]string{"Makefile": DecisionReplace, "README.md": DecisionSkip})
	if err != nil {
		t.Fatal(err)
	}
	if len(Conflicts(decided)) != 0 {
		t.Fatalf("conflicts remain: %+v", Conflicts(decided))
	}
	if got := action(t, decided, "Makefile"); got.State != StateUpdate {
		t.Fatalf("Makefile = %+v", got)
	}
	if got := action(t, decided, "README.md"); got.State != StateSkip {
		t.Fatalf("README = %+v", got)
	}
	m := manifestOf(t, decided)
	if m.Files["Makefile"].Hash != digest([]byte("gen\n")) {
		t.Fatal("replaced file not stamped")
	}
	if len(Conflicts(p)) != 2 {
		t.Fatal("Decide mutated the input plan")
	}
	for name, d := range map[string]map[string]string{
		"unknown value":  {"Makefile": "overwrite"},
		"not a conflict": {"new.txt": DecisionSkip},
		"unknown path":   {"missing": DecisionSkip},
		"skip mandatory": {"AGENTS.md": DecisionSkip},
	} {
		t.Run(name, func(t *testing.T) {
			q := p
			if name == "skip mandatory" {
				put(t, root, "AGENTS.md", section("edited\n"))
				q = prepare(t, root, "existing", append(slices.Clone(files), guidance("gen\n")))
			}
			if _, err := Decide(q, d); err == nil {
				t.Fatal("invalid decision accepted")
			}
		})
	}
}
