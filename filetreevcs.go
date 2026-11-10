// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"context"
	"path"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/vcs"
)

// SetRepository attaches a version-control repository: the tree marks each entry
// with the state the repository reports for it (see [FileTree.RefreshStatus] for
// the rules) and keeps the marks current. nil detaches it and clears every mark
// the repository gave (a status a host set on a [FileEntry] stays).
//
// The tree imports only github.com/go-widgets/vcs, never a driver: the host picks
// one (vcs.Detect with `import _ "github.com/go-widgets/vcs/git"`, or a fake from
// vcstest in a test) and hands over the [vcs.Repository]. Attaching refreshes at
// once, with context.Background.
//
// # Threading
//
// ⛔ A repository's Status can take seconds (go-git builds the whole status in
// memory), so a host with a large repository MUST set [FileTree.PostUI]. The tree
// then never calls Status on the UI goroutine: it runs it on a goroutine of its
// own and hands the result to PostUI as a closure that the host runs on its UI
// goroutine — the contract of collabui.CollabText.Updates and [Browser], where
// widget state is only ever touched from the one UI goroutine. A newer refresh
// (another SetRepository, a change of Entries, [FileTree.RefreshStatus],
// [FileTree.RefreshStatusAsync]) cancels the one in flight through its context and
// makes its late closure a no-op, so a stale answer never overwrites a newer one.
// Without PostUI every refresh is synchronous, which is correct for a fake, a
// small tree or a host with no UI thread, and freezes the UI otherwise.
//
// [FileTree.RefreshStatus] stays the explicit, waitable call: it returns when the
// marks are applied, so call it on the UI goroutine, or give it a context with a
// deadline.
//
// ⛔ Attaching a repository is the opt-in to a policy the tree otherwise refuses
// to guess: a FOLDER is marked [FileModified] when any entry under it has a mark,
// and [FileConflicted] when any entry under it is conflicted (conflicts win). A
// folder whose own path the repository reports (a submodule) keeps that state,
// except that a conflict beneath still outranks it.
//
// Marks never go stale through the tree's own operations. The tree does not apply
// a create, rename, delete, move or import: it asks its host (OnRequest), the
// host changes its store and sets [FileTree.Entries], and that Set refreshes the
// marks before the next draw. A change the tree is not told of (a file saved
// behind its back) needs a [FileTree.RefreshStatus] call.
//
// ⛔ A deleted file is marked only if the host still lists it: the tree shows the
// entries it is given and does not invent rows. List the repository's deleted
// paths beside the working tree's files (that is what [FileDeleted] documents).
func (f *FileTree) SetRepository(r vcs.Repository) {
	f.supersede()
	f.repo = r
	f.statusErr.Set(nil)
	if r == nil {
		f.marks.Set(nil)
		return
	}
	f.refresh() // the failure is in StatusError
}

// supersede cancels the asynchronous refresh in flight and makes its result,
// should it arrive anyway, stale: it carries the generation it started under.
func (f *FileTree) supersede() {
	if f.cancelStatus != nil {
		f.cancelStatus()
		f.cancelStatus = nil
	}
	f.statusGen++
}

// refresh is the refresh the tree starts itself (SetRepository, a change of
// Entries, [FileTree.RefreshStatusAsync]): off the UI goroutine when the host gave
// a PostUI, otherwise inline.
func (f *FileTree) refresh() {
	if f.PostUI == nil {
		_ = f.RefreshStatus(context.Background())
		return
	}
	f.supersede()
	ctx, cancel := context.WithCancel(context.Background())
	f.cancelStatus = cancel
	repo, gen, post := f.repo, f.statusGen, f.PostUI
	go func() {
		states, err := repo.Status(ctx)
		post(func() { f.finishRefresh(gen, states, err) })
	}()
}

// finishRefresh applies an asynchronous result on the UI goroutine, unless a
// newer refresh or a detach has superseded it.
func (f *FileTree) finishRefresh(gen uint64, states map[string]vcs.State, err error) {
	if gen != f.statusGen {
		return
	}
	f.cancelStatus() // releases the context; it is only ever set for the current generation
	f.cancelStatus = nil
	_ = f.applyStatus(states, err)
}

// RefreshStatusAsync starts a refresh and returns at once: with a PostUI the
// repository is asked on another goroutine and the marks change when the host
// runs the closure it was handed; without one it is [FileTree.RefreshStatus]
// with context.Background. A refresh already in flight is cancelled and its result
// discarded, so a slow answer never overwrites a newer one. It does nothing
// without a repository. Call it for a change the tree is not told of.
func (f *FileTree) RefreshStatusAsync() {
	if f.repo != nil {
		f.refresh()
	}
}

// Repository is the attached repository, nil when none.
func (f *FileTree) Repository() vcs.Repository { return f.repo }

// RefreshStatus asks the attached repository for its status and re-marks the
// tree. With no repository it does nothing and returns nil.
//
// Each reported path takes the mark of its state: Modified, Added, Deleted,
// Untracked and Conflicted map to the FileStatus of the same name; Renamed is
// [FileAdded] (the path in the map is the new one); Ignored and Unchanged draw no
// mark. A status set on a [FileEntry] by the host wins over the repository's.
//
// On failure the previous marks are kept (a transient error must not blank the
// tree), the error is returned and published on [FileTree.StatusError], and a
// later success clears it.
func (f *FileTree) RefreshStatus(ctx context.Context) error {
	if f.repo == nil {
		return nil
	}
	f.supersede() // an answer still in flight would be older than this one
	states, err := f.repo.Status(ctx)
	return f.applyStatus(states, err)
}

// applyStatus publishes a Status answer: the marks, or the error with the old
// marks kept.
func (f *FileTree) applyStatus(states map[string]vcs.State, err error) error {
	if err != nil {
		f.statusErr.Set(err)
		return err
	}
	f.statusErr.Set(nil)
	f.marks.Set(fileMarks(states))
	return nil
}

// Marks is what the repository put on the tree, folders included, as a shared
// [mvvm.Observable]: path to status, absent for no mark. A host binds a legend,
// a "3 changes" badge or a commit button's enabled state to it. It is read-only:
// the tree owns it, and setting it moves the marks until the next refresh.
func (f *FileTree) Marks() *mvvm.Observable[map[string]FileStatus] { return f.marks }

// StatusError is the error of the last [FileTree.RefreshStatus], nil after a
// success, as a shared [mvvm.Observable].
func (f *FileTree) StatusError() *mvvm.Observable[error] { return f.statusErr }

// refreshAfterEntries is the Entries subscriber's refresh.
func (f *FileTree) refreshAfterEntries() {
	f.RefreshStatusAsync()
}

// applyMarks gives every node without a status of its own the repository's mark.
func (f *FileTree) applyMarks() {
	marks := f.marks.Get()
	if len(marks) == 0 {
		return
	}
	for p, n := range f.nodes {
		if fn := n.Data.(*fileNode); p != "" && fn.status == FileUnchanged {
			fn.status = marks[p]
		}
	}
}

// sameError is the equality of the StatusError observable: errors are compared
// by message, because == on an interface panics for an uncomparable error type.
func sameError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Error() == b.Error()
}

// fileMarkOf is the FileStatus a vcs.State draws as.
func fileMarkOf(s vcs.State) FileStatus {
	switch s {
	case vcs.Modified:
		return FileModified
	case vcs.Added, vcs.Renamed:
		return FileAdded
	case vcs.Deleted:
		return FileDeleted
	case vcs.Untracked:
		return FileUntracked
	case vcs.Conflicted:
		return FileConflicted
	}
	return FileUnchanged // Unchanged, Ignored, and a state from a later vcs
}

// fileMarks turns a repository's status into the tree's marks: each path's own,
// then each folder above a marked path.
func fileMarks(states map[string]vcs.State) map[string]FileStatus {
	own := map[string]FileStatus{}
	for p, s := range states {
		if p = cleanFilePath(p); p != "" {
			if m := fileMarkOf(s); m != FileUnchanged {
				own[p] = m
			}
		}
	}
	out := map[string]FileStatus{}
	conflicted := map[string]bool{}
	for p, m := range own {
		out[p] = m
		for d := path.Dir(p); d != "."; d = path.Dir(d) {
			if m == FileConflicted {
				conflicted[d] = true
			}
			if _, has := own[d]; !has {
				out[d] = FileModified
			}
		}
	}
	for d := range conflicted {
		out[d] = FileConflicted
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
