package toolkit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-widgets/vcs"
	"github.com/go-widgets/vcs/vcstest"
)

// countRepo counts the Status calls that reach the repository.
type countRepo struct {
	*vcstest.Repo
	n int
}

func (r *countRepo) Status(ctx context.Context) (map[string]vcs.State, error) {
	r.n++
	return r.Repo.Status(ctx)
}

// flipSyncStatusBlocks sets the platform seam for one test (toolkit tests do not
// compile for js/wasm, issue #408).
func flipSyncStatusBlocks(t *testing.T, v bool) {
	t.Helper()
	old := syncStatusBlocks
	syncStatusBlocks = v
	t.Cleanup(func() { syncStatusBlocks = old })
}

// ⛔ In js/wasm a synchronous Status hangs the only thread: with no PostUI the
// tree refuses, calls no Status, and says how to fix it.
func TestWasmRefusesSynchronousStatus(t *testing.T) {
	flipSyncStatusBlocks(t, true)
	f, repo := vcsFixture()
	cr := &countRepo{Repo: repo}
	repo.Set(map[string]vcs.State{"README.md": vcs.Modified})

	f.SetRepository(cr) // would refresh synchronously
	if cr.n != 0 {
		t.Fatalf("SetRepository called Status %d times", cr.n)
	}
	if f.Repository() != vcs.Repository(cr) {
		t.Error("the repository must still be attached")
	}
	if !errors.Is(f.StatusError().Get(), ErrSyncStatusBlocks) {
		t.Errorf("StatusError = %v, want ErrSyncStatusBlocks", f.StatusError().Get())
	}

	err := f.RefreshStatus(context.Background())
	if !errors.Is(err, ErrSyncStatusBlocks) || cr.n != 0 {
		t.Errorf("RefreshStatus = %v after %d Status calls", err, cr.n)
	}
	f.RefreshStatusAsync()
	if cr.n != 0 {
		t.Error("RefreshStatusAsync without PostUI called Status")
	}
	for _, want := range []string{"PostUI", "RefreshStatusAsync"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %q: %v", want, err)
		}
	}

	f.SetRepository(nil)
	if f.StatusError().Get() != nil {
		t.Error("detaching must clear the refusal")
	}
	// With no repository there is nothing to refuse.
	if err := f.RefreshStatus(context.Background()); err != nil {
		t.Errorf("no repository: %v", err)
	}
}

// With a PostUI the refresh is asynchronous and allowed in js/wasm too.
func TestWasmAllowsStatusWithPostUI(t *testing.T) {
	flipSyncStatusBlocks(t, true)
	repo := newSlowRepo()
	f, posted := asyncTree(repo)
	call := <-repo.calls
	call.reply <- slowReply{states: modified("README.md")}
	(<-posted)()
	if got := treeStatus(t, f, "README.md"); got != FileModified {
		t.Errorf("README.md = %v", got)
	}
	if f.StatusError().Get() != nil {
		t.Errorf("unexpected %v", f.StatusError().Get())
	}
}

// Native builds are unchanged: the synchronous refresh runs.
func TestNativeStatusStaysSynchronous(t *testing.T) {
	flipSyncStatusBlocks(t, false)
	f, repo := vcsFixture()
	cr := &countRepo{Repo: repo}
	repo.Set(map[string]vcs.State{"README.md": vcs.Modified})
	f.SetRepository(cr)
	if cr.n != 1 || f.StatusError().Get() != nil {
		t.Errorf("Status calls = %d, error = %v", cr.n, f.StatusError().Get())
	}
	if got := treeStatus(t, f, "README.md"); got != FileModified {
		t.Errorf("README.md = %v", got)
	}
}
