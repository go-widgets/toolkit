// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestTreeViewBareValueObservables proves the zero-value &TreeView{} (built
// without NewTreeView) still exposes working Selected / ScrollRow observables:
// the accessors lazily initialise them, Get returns the zero value (nil / 0),
// and a host can bind (Subscribe + Set) both without a panic.
func TestTreeViewBareValueObservables(t *testing.T) {
	tv := &TreeView{}

	if got := tv.Selected().Get(); got != nil {
		t.Fatalf("bare &TreeView{} Selected().Get() = %v, want nil", got)
	}
	if got := tv.ScrollRow().Get(); got != 0 {
		t.Fatalf("bare &TreeView{} ScrollRow().Get() = %d, want 0", got)
	}

	// The accessor returns the SAME observable on a second call (it does not
	// re-init a fresh one over the lazily created handle).
	if tv.Selected() != tv.Selected() {
		t.Fatal("Selected() returned a different observable on the second call")
	}
	if tv.ScrollRow() != tv.ScrollRow() {
		t.Fatal("ScrollRow() returned a different observable on the second call")
	}

	// Host binds Selected: a Subscribe fires on every Set.
	node := &TreeNode{Label: "n"}
	var gotSel *TreeNode
	tv.Selected().Subscribe(func(n *TreeNode) { gotSel = n })
	tv.Selected().Set(node)
	if gotSel != node || tv.Selected().Get() != node {
		t.Fatalf("Selected bind: subscriber=%v get=%v, want %v", gotSel, tv.Selected().Get(), node)
	}

	// Host binds ScrollRow the same way.
	gotScroll := -1
	tv.ScrollRow().Subscribe(func(n int) { gotScroll = n })
	tv.ScrollRow().Set(7)
	if gotScroll != 7 || tv.ScrollRow().Get() != 7 {
		t.Fatalf("ScrollRow bind: subscriber=%d get=%d, want 7", gotScroll, tv.ScrollRow().Get())
	}
}

// TestTreeViewConstructorObservables proves NewTreeView pre-initialises both
// observables (a host may Subscribe immediately, before any Draw/OnEvent).
func TestTreeViewConstructorObservables(t *testing.T) {
	root := &TreeNode{Label: "root"}
	tv := NewTreeView(root)
	if tv.Selected().Get() != nil {
		t.Fatalf("NewTreeView Selected().Get() = %v, want nil", tv.Selected().Get())
	}
	if tv.ScrollRow().Get() != 0 {
		t.Fatalf("NewTreeView ScrollRow().Get() = %d, want 0", tv.ScrollRow().Get())
	}
}
