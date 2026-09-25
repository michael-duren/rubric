package write

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/michael-duren/go-skills/internal/plan"
)

var errInjected = errors.New("injected failure")

func failRenameAt(k int) (operations, *int) {
	ops := defaultOperations()
	calls := 0
	rename := ops.rename
	ops.rename = func(r *os.Root, oldname, newname string) error {
		calls++
		if calls == k {
			return errInjected
		}
		return rename(r, oldname, newname)
	}
	return ops, &calls
}

func fixture(t *testing.T) (string, plan.Plan) {
	t.Helper()
	root := t.TempDir()
	if _, err := Apply(context.Background(), root, prepare(t, root, "existing", file("a.txt", "a1"), file("c.txt", "c1"))); err != nil {
		t.Fatal(err)
	}
	put(t, root, "unrelated.txt", "keep")
	p := prepare(t, root, "existing", file("a.txt", "a2"), file("b/new.txt", "b"), file("c.txt", "c2"), file("d/e/f.txt", "f"))
	return root, p
}

func TestFailureAtEveryReplacementRestores(t *testing.T) {
	_, probe := fixture(t)
	writes := 0
	for _, a := range probe.Actions {
		if a.State == plan.StateCreate || a.State == plan.StateUpdate {
			writes++
		}
	}
	for k := 1; k <= writes; k++ {
		t.Run(fmt.Sprint(k), func(t *testing.T) {
			root, p := fixture(t)
			before := snapshotTree(t, root)
			ops, _ := failRenameAt(k)
			res, err := applyWithOps(context.Background(), root, p, ops)
			var applyErr *ApplyError
			if !errors.As(err, &applyErr) || !errors.Is(err, errInjected) {
				t.Fatalf("err = %v", err)
			}
			if len(res.Unrecovered) != 0 || !slices.Equal(res.Restored, applyErr.Result.Restored) {
				t.Fatalf("result = %+v", res)
			}
			if len(res.Restored) != k-1 {
				t.Fatalf("restored %v after failing write %d", res.Restored, k)
			}
			if after := snapshotTree(t, root); !mapsEqual(before, after) {
				t.Fatalf("tree not restored:\nbefore %v\nafter  %v", before, after)
			}
		})
	}
}

func TestRollbackPreservesConcurrentEdit(t *testing.T) {
	root, p := fixture(t)
	ops := defaultOperations()
	calls := 0
	rename := ops.rename
	ops.rename = func(r *os.Root, oldname, newname string) error {
		calls++
		if calls == 3 {
			put(t, root, "a.txt", "user edit after rubric wrote it")
			return errInjected
		}
		return rename(r, oldname, newname)
	}
	res, err := applyWithOps(context.Background(), root, p, ops)
	if err == nil {
		t.Fatal("expected failure")
	}
	if !slices.Equal(res.Unrecovered, []string{"a.txt"}) {
		t.Fatalf("unrecovered = %v", res.Unrecovered)
	}
	if read(t, root, "a.txt") != "user edit after rubric wrote it" {
		t.Fatal("rollback overwrote concurrent edit")
	}
	if read(t, root, "unrelated.txt") != "keep" {
		t.Fatal("unrelated file touched")
	}
	if !strings.Contains(err.Error(), "a.txt") {
		t.Fatalf("error does not list unrecovered path: %v", err)
	}
}

func TestRollbackKeepsDirectoriesWithUserFiles(t *testing.T) {
	root, p := fixture(t)
	ops := defaultOperations()
	calls := 0
	rename := ops.rename
	ops.rename = func(r *os.Root, oldname, newname string) error {
		calls++
		if calls == 3 {
			put(t, root, "b/user.txt", "mine")
			return errInjected
		}
		return rename(r, oldname, newname)
	}
	if _, err := applyWithOps(context.Background(), root, p, ops); err == nil {
		t.Fatal("expected failure")
	}
	if read(t, root, "b/user.txt") != "mine" {
		t.Fatal("user file in created directory removed")
	}
	if _, err := os.Stat(filepath.Join(root, "b", "new.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("rubric-created file not removed")
	}
}

func TestCancellationMidApplyRollsBack(t *testing.T) {
	root, p := fixture(t)
	before := snapshotTree(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	ops := defaultOperations()
	calls := 0
	rename := ops.rename
	ops.rename = func(r *os.Root, oldname, newname string) error {
		calls++
		if calls == 2 {
			cancel()
		}
		return rename(r, oldname, newname)
	}
	res, err := applyWithOps(ctx, root, p, ops)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if len(res.Restored) != 2 || len(res.Unrecovered) != 0 {
		t.Fatalf("result = %+v", res)
	}
	if after := snapshotTree(t, root); !mapsEqual(before, after) {
		t.Fatal("cancellation left partial writes")
	}
}

func TestStagingFailureCleansUp(t *testing.T) {
	root, p := fixture(t)
	ops := defaultOperations()
	ops.chmod = func(*os.Root, string, fs.FileMode) error { return errInjected }
	if _, err := applyWithOps(context.Background(), root, p, ops); !errors.Is(err, errInjected) {
		t.Fatalf("err = %v", err)
	}
	for _, name := range listing(t, root) {
		if strings.Contains(name, "rubric-tmp") {
			t.Fatalf("staged file left: %s", name)
		}
	}
}

func TestNewTargetRemovedOnFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "fresh", "proj")
	p := prepare(t, root, "new", file("a.txt", "a"), file("x/y.txt", "y"))
	ops, _ := failRenameAt(2)
	if _, err := applyWithOps(context.Background(), root, p, ops); !errors.Is(err, errInjected) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Dir(root)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("created destination directories left behind")
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, rel := range listing(t, root) {
		path := filepath.Join(root, rel)
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			value += string(data)
		}
		out[rel] = value
	}
	return out
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func deleteFixture(t *testing.T) (string, plan.Plan) {
	t.Helper()
	root := t.TempDir()
	if _, err := Apply(context.Background(), root, prepare(t, root, "existing", file("gone/deep/x.txt", "x"), file("a.txt", "a1"))); err != nil {
		t.Fatal(err)
	}
	return root, prepare(t, root, "existing", file("a.txt", "a2"))
}

func TestApplyDeletesObsoleteFilesAndPrunesEmptyDirectories(t *testing.T) {
	root, p := deleteFixture(t)
	put(t, root, "kept/user.txt", "mine")
	res, err := Apply(context.Background(), root, p)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Deleted, []string{"gone/deep/x.txt"}) || !slices.Contains(res.Applied, "a.txt") {
		t.Fatalf("result = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(root, "gone")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("empty directory kept: %v", err)
	}
	if read(t, root, "kept/user.txt") != "mine" {
		t.Fatal("unrelated file touched")
	}
}

func TestDeleteKeepsDirectoriesWithUserFiles(t *testing.T) {
	root, p := deleteFixture(t)
	put(t, root, "gone/notes.txt", "mine")
	if _, err := Apply(context.Background(), root, p); err != nil {
		t.Fatal(err)
	}
	if read(t, root, "gone/notes.txt") != "mine" {
		t.Fatal("user file lost")
	}
	if _, err := os.Stat(filepath.Join(root, "gone/deep")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("emptied directory kept: %v", err)
	}
}

func TestFailureAfterDeleteRestoresDeletedFile(t *testing.T) {
	root, p := deleteFixture(t)
	before := snapshotTree(t, root)
	ops, _ := failRenameAt(1)
	res, err := applyWithOps(context.Background(), root, p, ops)
	if !errors.Is(err, errInjected) {
		t.Fatalf("err = %v", err)
	}
	if !slices.Contains(res.Restored, "gone/deep/x.txt") || len(res.Unrecovered) != 0 {
		t.Fatalf("result = %+v", res)
	}
	if after := snapshotTree(t, root); !mapsEqual(before, after) {
		t.Fatalf("tree not restored:\nbefore %v\nafter  %v", before, after)
	}
}

func TestEditBeforeDeleteIsConflict(t *testing.T) {
	root, p := deleteFixture(t)
	put(t, root, "gone/deep/x.txt", "edited")
	var conflict *ConflictError
	if _, err := Apply(context.Background(), root, p); !errors.As(err, &conflict) {
		t.Fatalf("err = %v", err)
	}
	if read(t, root, "gone/deep/x.txt") != "edited" {
		t.Fatal("edited file deleted")
	}
}
