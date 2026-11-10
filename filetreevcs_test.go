package toolkit

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/go-widgets/vcs"
	"github.com/go-widgets/vcs/vcstest"
)

// treeStatus is the status a node carries once the tree has caught up with its
// entries and marks (the rebuild is lazy: it runs on draw and on input).
func treeStatus(t *testing.T, f *FileTree, p string) FileStatus {
	t.Helper()
	f.ensure()
	n := f.nodes[p]
	if n == nil {
		t.Fatalf("no node for %q", p)
	}
	return n.Data.(*fileNode).status
}

func vcsFixture() (*FileTree, *vcstest.Repo) {
	f := NewFileTree([]FileEntry{
		{Path: "README.md"},
		{Path: "src/main.go"},
		{Path: "src/util/helper.go"},
		{Path: "docs/guide.md"},
		{Path: "docs/old.md"},
		{Path: "vendor", Dir: true},
	})
	f.SetBounds(Rect{W: 240, H: 400})
	return f, vcstest.NewRepo("/work")
}

// Every vcs.State draws as the FileStatus the toolkit already had for it; the two
// that have no mark of their own say why.
func TestFileMarkOfCoversEveryState(t *testing.T) {
	for s, want := range map[vcs.State]FileStatus{
		vcs.Unchanged:  FileUnchanged,
		vcs.Modified:   FileModified,
		vcs.Added:      FileAdded,
		vcs.Deleted:    FileDeleted,
		vcs.Renamed:    FileAdded, // the status map's key is the NEW path
		vcs.Untracked:  FileUntracked,
		vcs.Ignored:    FileUnchanged, // FileIgnored does not exist: it would draw as nothing
		vcs.Conflicted: FileConflicted,
		vcs.State(99):  FileUnchanged, // from a later vcs: no mark, no crash
	} {
		if got := fileMarkOf(s); got != want {
			t.Errorf("%v -> %v, want %v", s, got, want)
		}
	}
}

func TestSetRepositoryMarksEntries(t *testing.T) {
	f, repo := vcsFixture()
	repo.Set(map[string]vcs.State{
		"README.md":   vcs.Modified,
		"src/main.go": vcs.Added,
		"docs/old.md": vcs.Deleted,
		"docs/new.md": vcs.Renamed, // not listed by the host: marked in Marks, no row
		"scratch.txt": vcs.Ignored,
		"clean.txt":   vcs.Unchanged,
	})
	if f.Repository() != nil {
		t.Fatal("a new tree has no repository")
	}
	f.SetRepository(repo)
	if f.Repository() != vcs.Repository(repo) {
		t.Error("Repository() is not the attached one")
	}
	for p, want := range map[string]FileStatus{
		"README.md":          FileModified,
		"src/main.go":        FileAdded,
		"docs/old.md":        FileDeleted,
		"src/util/helper.go": FileUnchanged,
		"docs/guide.md":      FileUnchanged,
	} {
		if got := treeStatus(t, f, p); got != want {
			t.Errorf("%s = %v, want %v", p, got, want)
		}
	}
	if f.nodes["scratch.txt"] != nil || f.nodes["docs/new.md"] != nil {
		t.Error("the tree invented a row for a path its host did not list")
	}
	if got := f.Marks().Get()["docs/new.md"]; got != FileAdded {
		t.Errorf("a renamed path's mark = %v, want FileAdded", got)
	}
	if _, ok := f.Marks().Get()["scratch.txt"]; ok {
		t.Error("an ignored path got a mark")
	}
	// It draws, and a screen reader hears the folder's state too.
	f.Draw(newP(makeSurface(240, 400), 240), DefaultLight())
	f.Draw(newP(makeSurface(240, 400), 240), DefaultDark())
	f.Selected().Set("src")
	if got := f.A11y().Value; got != "src, modified" {
		t.Errorf("A11y of a folder holding an added file = %q", got)
	}
}

func TestFoldersTakeTheirDescendantsMark(t *testing.T) {
	f, repo := vcsFixture()
	repo.Set(map[string]vcs.State{
		"src/util/helper.go": vcs.Untracked,
		"docs/guide.md":      vcs.Deleted,
	})
	f.SetRepository(repo)
	for p, want := range map[string]FileStatus{
		"src":      FileModified, // "any descendant has a status", untracked included
		"src/util": FileModified,
		"docs":     FileModified,
		"vendor":   FileUnchanged,
	} {
		if got := treeStatus(t, f, p); got != want {
			t.Errorf("folder %s = %v, want %v", p, got, want)
		}
	}

	// Conflicted takes precedence over every other mark beneath it, however deep
	// and whatever order the map is read in (run many times: Go randomises it).
	repo.Set(map[string]vcs.State{
		"src/main.go":        vcs.Modified,
		"src/util/helper.go": vcs.Conflicted,
		"docs/guide.md":      vcs.Added,
		"docs/old.md":        vcs.Deleted,
	})
	for range 50 {
		if err := f.RefreshStatus(context.Background()); err != nil {
			t.Fatal(err)
		}
		for p, want := range map[string]FileStatus{
			"src": FileConflicted, "src/util": FileConflicted, "docs": FileModified,
		} {
			if got := treeStatus(t, f, p); got != want {
				t.Fatalf("folder %s = %v, want %v", p, got, want)
			}
		}
	}

	// A folder the repository reports itself (a submodule) keeps its own state,
	// unless a conflict lies beneath it.
	repo.Set(map[string]vcs.State{"vendor": vcs.Untracked, "vendor/lib.go": vcs.Modified})
	f.RefreshStatus(context.Background())
	if got := treeStatus(t, f, "vendor"); got != FileUntracked {
		t.Errorf("a folder with its own state = %v, want FileUntracked", got)
	}
	repo.Set(map[string]vcs.State{"vendor": vcs.Untracked, "vendor/lib.go": vcs.Conflicted})
	f.RefreshStatus(context.Background())
	if got := treeStatus(t, f, "vendor"); got != FileConflicted {
		t.Errorf("a conflict beneath a folder with its own state = %v, want FileConflicted", got)
	}
}

// A host's own status on an entry is not overridden by the repository.
func TestEntryStatusWinsOverTheRepository(t *testing.T) {
	f := NewFileTree([]FileEntry{{Path: "a.txt", Status: FileUntracked}, {Path: "b.txt"}})
	repo := vcstest.NewRepo("/w")
	repo.Set(map[string]vcs.State{"a.txt": vcs.Modified, "b.txt": vcs.Modified})
	f.SetRepository(repo)
	if got := treeStatus(t, f, "a.txt"); got != FileUntracked {
		t.Errorf("a.txt = %v, the host's FileUntracked must win", got)
	}
	if got := treeStatus(t, f, "b.txt"); got != FileModified {
		t.Errorf("b.txt = %v", got)
	}
}

func TestDetachClearsTheMarks(t *testing.T) {
	f, repo := vcsFixture()
	repo.Set(map[string]vcs.State{"README.md": vcs.Modified})
	f.SetRepository(repo)
	if treeStatus(t, f, "README.md") != FileModified {
		t.Fatal("not marked")
	}
	f.SetRepository(nil)
	if f.Repository() != nil || treeStatus(t, f, "README.md") != FileUnchanged || len(f.Marks().Get()) != 0 {
		t.Error("detaching must remove every repository mark")
	}
	if err := f.RefreshStatus(context.Background()); err != nil {
		t.Errorf("RefreshStatus with no repository = %v, want nil", err)
	}
}

// ⛔ The marks must follow the tree's own operations. The tree only ASKS (a
// FileRequest); the host applies it to its store, sets Entries, and that Set is
// what refreshes — so this test plays the host through every verb.
func TestMarksFollowTheTreesOperations(t *testing.T) {
	f, repo := vcsFixture()
	repo.Set(map[string]vcs.State{"README.md": vcs.Modified})
	f.SetRepository(repo)
	calls := repo.Calls()

	// The host's store and what its repository would say after each request.
	entries := f.Entries().Get()
	var last FileRequest
	f.OnRequest = func(r FileRequest) { last = r }

	// Rename README.md -> GUIDE.md: old path gone, new path added.
	f.OnRequest(FileRequest{Action: FileRename, Path: "README.md", To: "GUIDE.md"})
	renamed := append([]FileEntry{{Path: "GUIDE.md"}}, entries[1:]...)
	repo.Set(map[string]vcs.State{"GUIDE.md": vcs.Renamed})
	f.Entries().Set(renamed)
	if repo.Calls() != calls+1 {
		t.Errorf("setting Entries asked the repository %d times, want 1", repo.Calls()-calls)
	}
	if treeStatus(t, f, "GUIDE.md") != FileAdded {
		t.Error("renamed file is not marked added at its new path")
	}
	if f.nodes["README.md"] != nil {
		t.Error("the old path is still listed")
	}

	// Create, delete, move, import: each is an Entries.Set after the host acts.
	for _, step := range []struct {
		name    string
		entries []FileEntry
		states  map[string]vcs.State
		path    string
		want    FileStatus
	}{
		{"create", append(slicesClone(renamed), FileEntry{Path: "docs/new.md"}), map[string]vcs.State{"docs/new.md": vcs.Untracked}, "docs", FileModified},
		{"delete", renamed[:len(renamed)-1], map[string]vcs.State{}, "docs", FileUnchanged},
		{"move", append(slicesClone(renamed[1:]), FileEntry{Path: "docs/moved.go"}), map[string]vcs.State{"docs/moved.go": vcs.Added}, "docs/moved.go", FileAdded},
		{"import", append(slicesClone(renamed[1:]), FileEntry{Path: "docs/dropped.png"}), map[string]vcs.State{"docs/dropped.png": vcs.Untracked}, "docs/dropped.png", FileUntracked},
	} {
		repo.Set(step.states)
		f.Entries().Set(step.entries)
		if got := treeStatus(t, f, step.path); got != step.want {
			t.Errorf("after %s: %s = %v, want %v", step.name, step.path, got, step.want)
		}
	}
	_ = last
}

func slicesClone(e []FileEntry) []FileEntry { return append([]FileEntry(nil), e...) }

// An error keeps the last good marks (a transient failure must not blank the
// tree), is published, and clears on the next success.
func TestRefreshStatusErrors(t *testing.T) {
	f, repo := vcsFixture()
	repo.Set(map[string]vcs.State{"README.md": vcs.Modified})
	f.SetRepository(repo)

	boom := errors.New("repository on fire")
	repo.Fail(boom)
	if err := f.RefreshStatus(context.Background()); err != boom {
		t.Fatalf("RefreshStatus = %v, want the repository's error", err)
	}
	if f.StatusError().Get() != boom {
		t.Error("StatusError was not published")
	}
	if treeStatus(t, f, "README.md") != FileModified {
		t.Error("an error blanked the previous marks")
	}
	f.Entries().Set(append(slicesClone(f.Entries().Get()), FileEntry{Path: "z.txt"})) // auto-refresh fails too, quietly
	if f.StatusError().Get() == nil {
		t.Error("the error vanished")
	}

	repo.Fail(nil)
	if err := f.RefreshStatus(context.Background()); err != nil || f.StatusError().Get() != nil {
		t.Errorf("after recovery: %v, %v", err, f.StatusError().Get())
	}

	// SetRepository reports a failing repository through StatusError, not a panic.
	bad := vcstest.NewRepo("/bad")
	bad.Fail(errors.New("nope"))
	f.SetRepository(bad)
	if f.StatusError().Get() == nil || f.Repository() != vcs.Repository(bad) {
		t.Error("attaching a failing repository must publish its error")
	}
	f.SetRepository(nil)
	if f.StatusError().Get() != nil {
		t.Error("detaching must clear the error")
	}
}

func TestRefreshStatusPassesTheContext(t *testing.T) {
	f, repo := vcsFixture()
	f.SetRepository(repo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.RefreshStatus(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled context reached the repository as %v", err)
	}
}

func TestMarksObservableNotifiesOnChangeOnly(t *testing.T) {
	f, repo := vcsFixture()
	f.SetRepository(repo)
	n := 0
	f.Marks().Subscribe(func(map[string]FileStatus) { n++ })
	repo.Set(map[string]vcs.State{"README.md": vcs.Modified})
	f.RefreshStatus(context.Background())
	f.RefreshStatus(context.Background()) // same marks: nothing to tell
	if n != 1 {
		t.Errorf("subscriber told %d times, want 1", n)
	}
	got := maps.Clone(f.Marks().Get())
	if got["README.md"] != FileModified || len(got) != 1 {
		t.Errorf("Marks = %v", got)
	}
}

func TestSameError(t *testing.T) {
	a, b := errors.New("x"), errors.New("x")
	if !sameError(nil, nil) || sameError(nil, a) || sameError(a, nil) {
		t.Error("nil handling")
	}
	if !sameError(a, b) || sameError(a, errors.New("y")) {
		t.Error("errors compare by message")
	}
	// An uncomparable error type must not panic the observable.
	if !sameError(sliceError{1}, sliceError{1}) {
		t.Error("an uncomparable error type")
	}
}

type sliceError []int

func (e sliceError) Error() string { return "slice" }

func TestFileMarksIgnoresUnusablePaths(t *testing.T) {
	got := fileMarks(map[string]vcs.State{"": vcs.Modified, "/": vcs.Modified, "ok.txt": vcs.Modified})
	if len(got) != 1 || got["ok.txt"] != FileModified {
		t.Errorf("fileMarks = %v", got)
	}
	if fileMarks(nil) != nil || fileMarks(map[string]vcs.State{"a": vcs.Ignored}) != nil {
		t.Error("nothing marked must be nil, so equal to the empty marks")
	}
}
