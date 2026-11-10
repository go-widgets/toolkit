package toolkit

import (
	"context"
	"errors"
	"testing"

	"github.com/go-widgets/vcs"
)

// slowRepo is a repository whose Status answers only when the test says so: each
// call is handed to the test, which replies (or does not) at the moment it picks.
type slowRepo struct {
	calls     chan *slowCall
	ignoreCtx bool // a driver that does not honour cancellation, like go-git
}

type slowCall struct {
	ctx   context.Context
	reply chan slowReply
}

type slowReply struct {
	states map[string]vcs.State
	err    error
}

func newSlowRepo() *slowRepo { return &slowRepo{calls: make(chan *slowCall, 16)} }

func (r *slowRepo) Root() string { return "/slow" }

func (r *slowRepo) Status(ctx context.Context) (map[string]vcs.State, error) {
	c := &slowCall{ctx: ctx, reply: make(chan slowReply, 1)}
	r.calls <- c
	if r.ignoreCtx {
		v := <-c.reply
		return v.states, v.err
	}
	select {
	case v := <-c.reply:
		return v.states, v.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// asyncTree is a tree with a PostUI that queues the closures the test then runs,
// standing in for the host's UI goroutine.
func asyncTree(repo vcs.Repository) (*FileTree, chan func()) {
	f, _ := vcsFixture()
	posted := make(chan func(), 16)
	f.PostUI = func(apply func()) { posted <- apply }
	if repo != nil {
		f.SetRepository(repo)
	}
	return f, posted
}

func modified(p string) map[string]vcs.State { return map[string]vcs.State{p: vcs.Modified} }

// ⛔ The UI call returns at once: nothing here answers Status until the end, so a
// SetRepository that waited for it would never return and this test would hang.
func TestAsyncRefreshReturnsAtOnce(t *testing.T) {
	repo := newSlowRepo()
	f, posted := asyncTree(repo)
	call := <-repo.calls // Status is running, on another goroutine
	if len(f.Marks().Get()) != 0 {
		t.Fatal("marks appeared before the repository answered")
	}
	call.reply <- slowReply{states: modified("README.md")}
	(<-posted)() // the host runs the closure on its UI goroutine
	if got := treeStatus(t, f, "README.md"); got != FileModified {
		t.Errorf("README.md = %v after the answer", got)
	}
}

func TestSupersededRefreshIsDiscarded(t *testing.T) {
	repo := newSlowRepo()
	repo.ignoreCtx = true
	f, posted := asyncTree(repo)
	first := <-repo.calls
	f.RefreshStatusAsync()
	second := <-repo.calls
	if first.ctx.Err() == nil {
		t.Error("the superseded refresh was not cancelled")
	}
	if second.ctx.Err() != nil {
		t.Error("the current refresh was cancelled")
	}
	second.reply <- slowReply{states: modified("src/main.go")}
	(<-posted)()
	// The older answer arrives late, success or failure: neither may land.
	first.reply <- slowReply{states: modified("README.md")}
	(<-posted)()
	if treeStatus(t, f, "README.md") != FileUnchanged || treeStatus(t, f, "src/main.go") != FileModified {
		t.Errorf("a stale answer overwrote a newer one: %v", f.Marks().Get())
	}

	// A stale failure does not publish an error either.
	f.RefreshStatusAsync()
	third := <-repo.calls
	f.RefreshStatusAsync()
	fourth := <-repo.calls
	fourth.reply <- slowReply{states: modified("src/main.go")}
	(<-posted)()
	third.reply <- slowReply{err: errors.New("stale failure")}
	(<-posted)()
	if f.StatusError().Get() != nil {
		t.Errorf("a stale error was published: %v", f.StatusError().Get())
	}
}

func TestDetachDiscardsAnAnswerInFlight(t *testing.T) {
	repo := newSlowRepo()
	repo.ignoreCtx = true
	f, posted := asyncTree(repo)
	call := <-repo.calls
	f.SetRepository(nil)
	call.reply <- slowReply{states: modified("README.md")}
	(<-posted)()
	if f.Repository() != nil || len(f.Marks().Get()) != 0 {
		t.Errorf("a late answer re-marked a detached tree: %v", f.Marks().Get())
	}
}

func TestExplicitRefreshSupersedesTheAsyncOne(t *testing.T) {
	repo := newSlowRepo()
	repo.ignoreCtx = true
	f, posted := asyncTree(repo)
	old := <-repo.calls
	done := make(chan error, 1)
	go func() { done <- f.RefreshStatus(context.Background()) }()
	explicit := <-repo.calls
	explicit.reply <- slowReply{states: modified("docs/guide.md")}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	old.reply <- slowReply{states: modified("README.md")}
	(<-posted)()
	if treeStatus(t, f, "docs/guide.md") != FileModified || treeStatus(t, f, "README.md") != FileUnchanged {
		t.Errorf("the explicit refresh did not win: %v", f.Marks().Get())
	}
}

func TestEntriesChangeRefreshesAsynchronously(t *testing.T) {
	repo := newSlowRepo()
	f, posted := asyncTree(repo)
	(<-repo.calls).reply <- slowReply{}
	(<-posted)()
	f.Entries().Set(append(slicesClone(f.Entries().Get()), FileEntry{Path: "new.txt"}))
	next := <-repo.calls // asked, and Set has already returned
	next.reply <- slowReply{states: map[string]vcs.State{"new.txt": vcs.Untracked}}
	(<-posted)()
	if treeStatus(t, f, "new.txt") != FileUntracked {
		t.Error("the new entry was not marked after the host changed Entries")
	}
}

func TestAsyncErrorKeepsTheMarks(t *testing.T) {
	repo := newSlowRepo()
	f, posted := asyncTree(repo)
	(<-repo.calls).reply <- slowReply{states: modified("README.md")}
	(<-posted)()
	f.RefreshStatusAsync()
	boom := errors.New("boom")
	(<-repo.calls).reply <- slowReply{err: boom}
	(<-posted)()
	if f.StatusError().Get() != boom || treeStatus(t, f, "README.md") != FileModified {
		t.Errorf("error %v, README.md %v", f.StatusError().Get(), treeStatus(t, f, "README.md"))
	}
}

func TestRefreshStatusAsyncWithoutRepositoryDoesNothing(t *testing.T) {
	f, posted := asyncTree(nil)
	f.RefreshStatusAsync()
	select {
	case <-posted:
		t.Error("a refresh was posted with no repository")
	default:
	}
}
