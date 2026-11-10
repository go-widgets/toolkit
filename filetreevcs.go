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
// once, with context.Background; a host that needs a deadline calls
// [FileTree.RefreshStatus] itself.
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
	f.repo = r
	f.statusErr.Set(nil)
	if r == nil {
		f.marks.Set(nil)
		return
	}
	_ = f.RefreshStatus(context.Background()) // the failure is in StatusError
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
	states, err := f.repo.Status(ctx)
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
	if f.repo != nil {
		_ = f.RefreshStatus(context.Background())
	}
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
