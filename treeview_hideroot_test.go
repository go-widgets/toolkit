// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestTreeViewHideRoot(t *testing.T) {
	c1 := &TreeNode{Label: "a", Expanded: true, Children: []*TreeNode{{Label: "a1"}}}
	c2 := &TreeNode{Label: "b"}
	root := &TreeNode{Label: "root", Expanded: true, Children: []*TreeNode{c1, c2}}
	tv := NewTreeView(root)
	tv.HideRoot = true
	tv.flatten()

	// The root is not a visible row; its children are the top-level rows at depth
	// 0, with their own descendants nested beneath: a(0), a1(1), b(0).
	if len(tv.rows) != 3 {
		t.Fatalf("visible rows = %d, want 3", len(tv.rows))
	}
	if tv.rows[0].node != c1 || tv.rows[0].depth != 0 {
		t.Fatalf("row0 = %+v, want c1 at depth 0", tv.rows[0])
	}
	if tv.rows[1].node.Label != "a1" || tv.rows[1].depth != 1 {
		t.Fatalf("row1 = %+v, want a1 at depth 1", tv.rows[1])
	}
	if tv.rows[2].node != c2 || tv.rows[2].depth != 0 {
		t.Fatalf("row2 = %+v, want c2 at depth 0", tv.rows[2])
	}
	for _, r := range tv.rows {
		if r.node == root {
			t.Fatal("HideRoot must never include the root as a visible row")
		}
	}

	// The root's own Expanded flag is ignored under HideRoot: its children are
	// always shown even when the root is collapsed.
	root.Expanded = false
	tv.flatten()
	if len(tv.rows) != 3 {
		t.Fatalf("collapsed-root rows under HideRoot = %d, want 3 (children always shown)", len(tv.rows))
	}

	// With HideRoot off, the same tree shows the root as the single top row (its
	// now-collapsed children hidden).
	tv.HideRoot = false
	tv.flatten()
	if len(tv.rows) != 1 || tv.rows[0].node != root {
		t.Fatalf("root-visible collapsed tree = %d rows, want just the root", len(tv.rows))
	}
}
