// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"runtime"
	"sync"

	"github.com/go-widgets/vcs/vcstest"
)

// vcsHarness lets the FileTree VCS tests assert the same marks on every platform
// through the path each one really takes: synchronously where Repository.Status
// may run on the UI goroutine, and through PostUI + RefreshStatusAsync on js/wasm,
// where the tree refuses a synchronous Status (ErrSyncStatusBlocks). The two
// platform halves are filetreevcs_harness_native_test.go and
// filetreevcs_harness_js_test.go.
type vcsHarness struct {
	f     *FileTree
	repos []*vcstest.Repo

	mu     sync.Mutex
	cond   *sync.Cond
	posted int
	queue  []func()
}

func newVCSHarness(f *FileTree, repos ...*vcstest.Repo) *vcsHarness {
	h := &vcsHarness{f: f, repos: repos}
	h.cond = sync.NewCond(&h.mu)
	h.install()
	return h
}

// track adds a repository whose Status calls settle must wait for.
func (h *vcsHarness) track(r *vcstest.Repo) { h.repos = append(h.repos, r) }

// post is the tree's PostUI: it queues the closure for the test goroutine, the
// stand-in for the host's UI goroutine.
func (h *vcsHarness) post(apply func()) {
	h.mu.Lock()
	h.posted++
	h.queue = append(h.queue, apply)
	h.mu.Unlock()
	h.cond.Broadcast()
}

// drain waits until every Status call made so far has posted its answer (a refresh
// the tree just started has not called Status yet: on js/wasm its goroutine runs
// only when this one yields), then runs the queued closures on the calling
// goroutine, in order.
func (h *vcsHarness) drain() { h.collect(false) }

// drainNewestFirst is drain with the answers applied in reverse: the newer
// refresh's answer lands BEFORE the older one's, the order in which a stale answer
// overwrites a fresh one unless the tree discards it.
func (h *vcsHarness) drainNewestFirst() { h.collect(true) }

// wait lets every refresh started so far call Status and post its answer, and
// applies none of them.
func (h *vcsHarness) wait() {
	h.mu.Lock()
	h.waitLocked()
	h.mu.Unlock()
}

func (h *vcsHarness) waitLocked() {
	h.mu.Unlock()
	for range 3 {
		runtime.Gosched()
	}
	h.mu.Lock()
	calls := 0
	for _, r := range h.repos {
		calls += r.Calls()
	}
	for h.posted < calls {
		h.cond.Wait()
	}
}

func (h *vcsHarness) collect(reverse bool) {
	h.mu.Lock()
	h.waitLocked()
	q := h.queue
	h.queue = nil
	h.mu.Unlock()
	if reverse {
		for i := len(q) - 1; i >= 0; i-- {
			q[i]()
		}
		return
	}
	for _, apply := range q {
		apply()
	}
}
