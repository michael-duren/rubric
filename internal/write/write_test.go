package write

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/plan"
	"github.com/michael-duren/go-skills/internal/render"
	"github.com/michael-duren/go-skills/internal/testproject"
)

func file(path, data string) render.File {
	return render.File{Path: path, Data: []byte(data), Mode: 0o644, Kind: render.KindManaged}
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

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func prepare(t *testing.T, root, mode string, files ...render.File) plan.Plan {
	t.Helper()
	p, err := plan.Prepare(root, mode, testproject.Config(), files)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func listing(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCancelledApplyCreatesNothing(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new project")
	files := []render.File{{Path: "go.mod", Data: []byte("module example.com/app\n"), Mode: 0o644, Kind: "scaffold"}}
	p, err := plan.Prepare(root, "new", testproject.Config(), files)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Apply(ctx, root, p); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("target was created: %v", err)
	}
}

func TestApplyCreatesMissingTargetWithLiteralName(t *testing.T) {
	root := filepath.Join(t.TempDir(), "odd $(name) `q` \"x\" 'y'", "nested")
	p := prepare(t, root, "new", file("a/b.txt", "b"), file("run.sh", "#!/bin/sh\n"))
	p.Actions[slices.IndexFunc(p.Actions, func(a plan.Action) bool { return a.File.Path == "run.sh" })].File.Mode = 0o755
	res, err := Apply(context.Background(), root, p)
	if err != nil {
		t.Fatal(err)
	}
	if read(t, root, "a/b.txt") != "b" || !slices.Contains(res.Applied, plan.ManifestPath) {
		t.Fatalf("result = %+v", res)
	}
	if res.Applied[len(res.Applied)-1] != plan.ManifestPath {
		t.Fatalf("manifest not written last: %v", res.Applied)
	}
	info, err := os.Stat(filepath.Join(root, "run.sh"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, %v", info, err)
	}
	for _, name := range listing(t, root) {
		if strings.Contains(name, "rubric-tmp") {
			t.Fatalf("staged file left behind: %s", name)
		}
	}
}

func TestApplyUpdatesAndSkipsUnchanged(t *testing.T) {
	root := t.TempDir()
	if _, err := Apply(context.Background(), root, prepare(t, root, "existing", file("m.txt", "v1"), file("same.txt", "s"))); err != nil {
		t.Fatal(err)
	}
	p := prepare(t, root, "existing", file("m.txt", "v2"), file("same.txt", "s"))
	res, err := Apply(context.Background(), root, p)
	if err != nil {
		t.Fatal(err)
	}
	if read(t, root, "m.txt") != "v2" || slices.Contains(res.Applied, "same.txt") {
		t.Fatalf("result = %+v", res)
	}
	again, err := Apply(context.Background(), root, prepare(t, root, "existing", file("m.txt", "v2"), file("same.txt", "s")))
	if err != nil || len(again.Applied) != 0 {
		t.Fatalf("rerun applied %v, %v", again.Applied, err)
	}
}

func TestUnresolvedConflictsFailBeforeWrites(t *testing.T) {
	root := t.TempDir()
	put(t, root, "Makefile", "user\n")
	p := prepare(t, root, "existing", file("Makefile", "gen\n"), file("new.txt", "n"))
	_, err := Apply(context.Background(), root, p)
	var conflict *ConflictError
	if !errors.As(err, &conflict) || !slices.Equal(conflict.Paths, []string{"Makefile"}) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "new.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("wrote despite conflicts")
	}
}

func TestConcurrentChangeBeforeApplyIsConflict(t *testing.T) {
	root := t.TempDir()
	put(t, root, "rubric.yaml", "schema: 1\n")
	cfg := render.File{Path: "rubric.yaml", Data: []byte("schema: 1\nstyle: x\n"), Mode: 0o644, Kind: render.KindConfig}
	p := prepare(t, root, "existing", cfg, file("new.txt", "n"))
	put(t, root, "rubric.yaml", "schema: 1\n# user edit during review\n")
	_, err := Apply(context.Background(), root, p)
	var conflict *ConflictError
	if !errors.As(err, &conflict) || !slices.Contains(conflict.Paths, "rubric.yaml") {
		t.Fatalf("err = %v", err)
	}
	if read(t, root, "rubric.yaml") != "schema: 1\n# user edit during review\n" {
		t.Fatal("user edit overwritten")
	}
	if _, err := os.Stat(filepath.Join(root, "new.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("wrote other files despite stale plan")
	}
}

func TestStaleManifestIsConflict(t *testing.T) {
	root := t.TempDir()
	p := prepare(t, root, "existing", file("a.txt", "a"))
	put(t, root, plan.ManifestPath, `{"version":1,"files":{}}`)
	_, err := Apply(context.Background(), root, p)
	var conflict *ConflictError
	if !errors.As(err, &conflict) || !slices.Contains(conflict.Paths, plan.ManifestPath) {
		t.Fatalf("err = %v", err)
	}
}

func TestSymlinkTargetsAndParentsRefused(t *testing.T) {
	outside := t.TempDir()
	t.Run("parent", func(t *testing.T) {
		root := t.TempDir()
		p := prepare(t, root, "existing", file("dir/x.txt", "x"))
		if err := os.Symlink(outside, filepath.Join(root, "dir")); err != nil {
			t.Skip(err)
		}
		if _, err := Apply(context.Background(), root, p); err == nil {
			t.Fatal("wrote through symlinked parent")
		}
		if _, err := os.Stat(filepath.Join(outside, "x.txt")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("escaped through symlink")
		}
	})
	t.Run("target", func(t *testing.T) {
		root := t.TempDir()
		p := prepare(t, root, "existing", file("x.txt", "x"))
		if err := os.Symlink(filepath.Join(outside, "y.txt"), filepath.Join(root, "x.txt")); err != nil {
			t.Skip(err)
		}
		if _, err := Apply(context.Background(), root, p); err == nil {
			t.Fatal("wrote through symlink target")
		}
		if _, err := os.Stat(filepath.Join(outside, "y.txt")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("escaped through symlink")
		}
	})
}

func TestInvalidPlanPathsRejected(t *testing.T) {
	for _, rel := range []string{"../escape.txt", "/abs/file.txt", "a/../../b", "a\\b"} {
		t.Run(rel, func(t *testing.T) {
			root := t.TempDir()
			p := plan.Plan{Mode: "existing", Actions: []plan.Action{{File: file(rel, "x"), State: plan.StateCreate}}}
			if _, err := Apply(context.Background(), root, p); err == nil {
				t.Fatal("invalid path accepted")
			}
			if got := listing(t, filepath.Dir(root)); len(got) != 2 {
				t.Fatalf("something written: %v", got)
			}
		})
	}
}

func TestTargetMustBeDirectory(t *testing.T) {
	parent := t.TempDir()
	put(t, parent, "file", "x")
	root := filepath.Join(parent, "file")
	p := plan.Plan{Mode: "new", Actions: []plan.Action{{File: file("a.txt", "a"), State: plan.StateCreate}}}
	if _, err := Apply(context.Background(), root, p); err == nil {
		t.Fatal("wrote beneath a regular file")
	}
}
