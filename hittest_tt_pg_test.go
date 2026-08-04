// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestTreeTableNodeAtAndRemove(t *testing.T) {
	root := []*TreeTableNode{
		{Cells: []string{"a", "1"}, Expanded: true, Children: []*TreeTableNode{{Cells: []string{"a1", "2"}}}},
		{Cells: []string{"b", "3"}},
	}
	tt := NewTreeTable([]TreeTableColumn{{Title: "N"}, {Title: "V"}}, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 400})

	// Visible flattened order: a (row0), a1 (row1), b (row2).
	if tt.NodeAt(5, 2) != nil { // header band
		t.Fatal("NodeAt(header) != nil")
	}
	if tt.NodeAt(5, TreeTableHeaderHeight+2) != root[0] {
		t.Fatal("NodeAt row0 != a")
	}
	if tt.NodeAt(5, TreeTableHeaderHeight+2*TreeTableRowHeight+2) != root[1] {
		t.Fatal("NodeAt row2 != b")
	}
	if tt.NodeAt(5, 100000) != nil {
		t.Fatal("NodeAt(past end) != nil")
	}
	// Windowed branch: one visible body row, more rows below.
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: TreeTableHeaderHeight + TreeTableRowHeight + 3})
	if tt.NodeAt(5, TreeTableHeaderHeight+2*TreeTableRowHeight) != nil {
		t.Fatal("NodeAt below the painted window should be nil")
	}
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 400})

	// Remove guards + nested + top-level.
	if tt.Remove(nil) || tt.Remove(&TreeTableNode{Cells: []string{"ghost"}}) {
		t.Fatal("Remove(nil/absent) should be false")
	}
	// Grandchild removal exercises removeTreeTableChild's deep-recursion branch.
	a1 := root[0].Children[0]
	gc := &TreeTableNode{Cells: []string{"a1a", "x"}}
	a1.Children = append(a1.Children, gc)
	if !tt.Remove(gc) || len(a1.Children) != 0 {
		t.Fatal("grandchild Remove failed")
	}
	if !tt.Remove(a1) || len(tt.Root[0].Children) != 0 {
		t.Fatal("nested Remove failed")
	}
	if !tt.Remove(tt.Root[1]) || len(tt.Root) != 1 {
		t.Fatalf("top-level Remove failed: %d roots", len(tt.Root))
	}
}

func TestPropertyGridRemoveAt(t *testing.T) {
	pg := NewPropertyGrid()
	pg.Add("W", "1024")
	pg.Add("H", "768")
	pg.Add("D", "depth")

	pg.RemoveAt(1) // remove "H"
	if pg.Value("H") != "" {
		t.Fatal("H not removed")
	}
	if len(pg.Table().Rows) != 2 || pg.Table().Rows[1][0] != "D" {
		t.Fatalf("rows after remove: %+v", pg.Table().Rows)
	}
	if pg.Value("D") != "depth" || pg.Value("W") != "1024" {
		t.Fatal("surviving properties corrupted")
	}
	pg.RemoveAt(99) // out of range no-ops
	pg.RemoveAt(-1)
	if len(pg.Table().Rows) != 2 {
		t.Fatal("out-of-range RemoveAt mutated the grid")
	}
}
