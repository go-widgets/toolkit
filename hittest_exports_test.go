// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestTableRowAt covers the exported RowAt hit helper.
func TestTableRowAt(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}, {Title: "B"}},
		[][]string{{"r0", "x"}, {"r1", "y"}, {"r2", "z"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})
	if got := tb.RowAt(5, TableHeaderHeight+2); got != 0 {
		t.Fatalf("RowAt row0 = %d, want 0", got)
	}
	if got := tb.RowAt(5, TableHeaderHeight+2*TableRowHeight+2); got != 2 {
		t.Fatalf("RowAt row2 = %d, want 2", got)
	}
	if got := tb.RowAt(5, 2); got != -1 {
		t.Fatalf("RowAt(header) = %d, want -1", got)
	}
	if got := tb.RowAt(5, TableHeaderHeight+100*TableRowHeight); got != -1 {
		t.Fatalf("RowAt(past end) = %d, want -1", got)
	}
}

// TestListBoxIndexAt covers the exported IndexAt hit helper.
func TestListBoxIndexAt(t *testing.T) {
	lb := NewListBox([]string{"a", "b", "c"})
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 200})
	if got := lb.IndexAt(5, 2); got != 0 {
		t.Fatalf("IndexAt row0 = %d, want 0", got)
	}
	if got := lb.IndexAt(5, lb.RowHeight+2); got != 1 {
		t.Fatalf("IndexAt row1 = %d, want 1", got)
	}
	if got := lb.IndexAt(5, -1); got != -1 {
		t.Fatalf("IndexAt(y<0) = %d, want -1", got)
	}
	if got := lb.IndexAt(5, 100*lb.RowHeight); got != -1 {
		t.Fatalf("IndexAt(past end) = %d, want -1", got)
	}
	lb.RowHeight = 0
	if got := lb.IndexAt(5, 5); got != -1 {
		t.Fatalf("IndexAt(RowHeight=0) = %d, want -1", got)
	}
}

// TestTreeViewNodeAtAndRemove covers NodeAt (incl. the windowed branch) and the
// Remove / removeTreeChild mutation helpers.
func TestTreeViewNodeAtAndRemove(t *testing.T) {
	root := &TreeNode{Label: "/", Expanded: true, Children: []*TreeNode{
		{Label: "a"},
		{Label: "b", Expanded: true, Children: []*TreeNode{{Label: "b1"}}},
	}}
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 400})

	if tv.NodeAt(5, 2) != root { // row 0
		t.Fatal("NodeAt row0 != root")
	}
	if got := tv.NodeAt(5, tv.RowHeight+2); got == nil || got.Label != "a" {
		t.Fatalf("NodeAt row1 = %v, want a", got)
	}
	if tv.NodeAt(5, -1) != nil {
		t.Fatal("NodeAt(y<0) != nil")
	}
	if tv.NodeAt(5, 10000) != nil {
		t.Fatal("NodeAt(past end) != nil")
	}
	// Windowed branch: a 1-row-tall window with more rows below it.
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: tv.RowHeight + 3})
	if tv.NodeAt(5, 2*tv.RowHeight) != nil {
		t.Fatal("NodeAt below the painted window should be nil")
	}
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 400})

	// Remove guards + a nested removal.
	if tv.Remove(nil) || tv.Remove(root) || tv.Remove(&TreeNode{Label: "ghost"}) {
		t.Fatal("Remove(nil/root/absent) should all be false")
	}
	b1 := root.Children[1].Children[0]
	if !tv.Remove(b1) {
		t.Fatal("Remove(b1) should be true")
	}
	if len(root.Children[1].Children) != 0 {
		t.Fatalf("b1 not detached: %+v", root.Children[1].Children)
	}
	a := root.Children[0]
	if !tv.Remove(a) || len(root.Children) != 1 || root.Children[0].Label != "b" {
		t.Fatalf("Remove(a) failed: %+v", root.Children)
	}
}
