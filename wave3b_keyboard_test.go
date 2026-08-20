// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"testing"
)

// Wave 3b gives the data widgets (ListBox, Table, TreeView, TreeTable) a
// keyboard ROVING CURSOR -- Arrow/Page/Home/End move the selection cursor and
// auto-scroll to keep it visible, Enter/Space activate it like a click, and
// the trees add Left/Right expand/collapse -- and the Menu/MenuBar their own
// keyboard navigation. Every test asserts the cursor index AND that it sits
// inside the visible window, that activation fires the same callback a click
// would, that bounds clamp, and that a disabled widget ignores keys.

// kd3b builds an EventKeyDown carrying code (kd already exists in
// wave3_keyboard_test.go; a distinct name avoids a redeclaration).
func kd3b(code string) Event { return Event{Kind: EventKeyDown, Code: code} }

// --- ListBox ---------------------------------------------------------------

func newCursorListBox() *ListBox {
	items := make([]string, 20)
	for i := range items {
		items[i] = fmt.Sprintf("row %d", i)
	}
	lb := NewListBox(items)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 54}) // 3 of 20 rows visible
	return lb
}

// lbVisible asserts the cursor (Selected) sits inside the current scroll
// window [ScrollRow, ScrollRow+visibleRows).
func lbVisible(t *testing.T, lb *ListBox) {
	t.Helper()
	vr := lb.visibleRows()
	if lb.Selected().Get() < lb.ScrollRow().Get() || lb.Selected().Get() >= lb.ScrollRow().Get()+vr {
		t.Fatalf("cursor %d outside window [%d,%d)", lb.Selected().Get(), lb.ScrollRow().Get(), lb.ScrollRow().Get()+vr)
	}
}

func TestListBoxKeyCursorAndActivate(t *testing.T) {
	lb := newCursorListBox()
	activated := -1
	lb.OnActivate = func(i int) { activated = i }

	// First ArrowDown with no selection lands on row 0 (stays visible).
	lb.OnEvent(kd3b("ArrowDown"))
	if lb.Selected().Get() != 0 || lb.ScrollRow().Get() != 0 {
		t.Fatalf("ArrowDown from none: Selected=%d ScrollRow=%d", lb.Selected().Get(), lb.ScrollRow().Get())
	}
	lbVisible(t, lb)

	// End jumps to the last row and auto-scrolls so it is visible.
	lb.OnEvent(kd3b("End"))
	if lb.Selected().Get() != 19 || lb.ScrollRow().Get() != 17 {
		t.Fatalf("End: Selected=%d ScrollRow=%d, want 19/17", lb.Selected().Get(), lb.ScrollRow().Get())
	}
	lbVisible(t, lb)

	// ArrowDown at the last row clamps (no wrap).
	lb.OnEvent(kd3b("ArrowDown"))
	if lb.Selected().Get() != 19 {
		t.Fatalf("ArrowDown clamp: Selected=%d", lb.Selected().Get())
	}

	// Home jumps back to the first row and scrolls up to it.
	lb.OnEvent(kd3b("Home"))
	if lb.Selected().Get() != 0 || lb.ScrollRow().Get() != 0 {
		t.Fatalf("Home: Selected=%d ScrollRow=%d", lb.Selected().Get(), lb.ScrollRow().Get())
	}
	lbVisible(t, lb)

	// PageDown moves one page and keeps the cursor visible.
	lb.OnEvent(kd3b("PageDown")) // 0 -> 3
	if lb.Selected().Get() != 3 || lb.ScrollRow().Get() != 1 {
		t.Fatalf("PageDown: Selected=%d ScrollRow=%d, want 3/1", lb.Selected().Get(), lb.ScrollRow().Get())
	}
	lbVisible(t, lb)

	// PageUp moves one page back.
	lb.OnEvent(kd3b("PageUp")) // 3 -> 0
	if lb.Selected().Get() != 0 {
		t.Fatalf("PageUp: Selected=%d", lb.Selected().Get())
	}
	lbVisible(t, lb)

	// ArrowUp at the top clamps.
	lb.OnEvent(kd3b("ArrowUp"))
	if lb.Selected().Get() != 0 {
		t.Fatalf("ArrowUp clamp: Selected=%d", lb.Selected().Get())
	}
	// A non-navigation key (ArrowLeft) is ignored (rovingIndex default).
	lb.OnEvent(kd3b("ArrowLeft"))
	if lb.Selected().Get() != 0 {
		t.Fatalf("ArrowLeft moved cursor: Selected=%d", lb.Selected().Get())
	}

	// Enter / Space / " " each activate the cursor row like a click.
	lb.OnEvent(kd3b("Enter"))
	if activated != 0 {
		t.Fatalf("Enter activate: %d, want 0", activated)
	}
	activated = -1
	lb.OnEvent(kd3b(" "))
	lb.OnEvent(kd3b("Space"))
	if activated != 0 {
		t.Fatalf("Space activate: %d, want 0", activated)
	}
}

func TestListBoxKeyArrowUpFromNoSelection(t *testing.T) {
	lb := newCursorListBox()
	// First key is ArrowUp with Selected == -1: lands on row 0.
	lb.OnEvent(kd3b("ArrowUp"))
	if lb.Selected().Get() != 0 {
		t.Fatalf("ArrowUp from none: Selected=%d, want 0", lb.Selected().Get())
	}
}

func TestListBoxKeyMultiSelectAndActivate(t *testing.T) {
	lb := newCursorListBox()
	lb.MultiSelect = true
	activated := -1
	lb.OnActivate = func(i int) { activated = i }
	// A plain move sets the sole selection to the cursor row (like a click).
	lb.OnEvent(kd3b("ArrowDown")) // 0
	lb.OnEvent(kd3b("ArrowDown")) // 1
	if !lb.IsSelected(1) || lb.IsSelected(0) {
		t.Fatalf("multiselect plain move: sel(0)=%v sel(1)=%v", lb.IsSelected(0), lb.IsSelected(1))
	}
	// Enter collapses selection to the cursor + fires OnActivate.
	lb.OnEvent(kd3b("Enter"))
	if activated != 1 || !lb.IsSelected(1) {
		t.Fatalf("multiselect Enter: activated=%d sel(1)=%v", activated, lb.IsSelected(1))
	}
}

func TestListBoxKeyDisabledAndEmptyAndNil(t *testing.T) {
	// Disabled ignores keys.
	lb := newCursorListBox()
	lb.Selected().Set(5)
	lb.Disabled = true
	lb.OnEvent(kd3b("ArrowDown"))
	lb.OnEvent(kd3b("Enter"))
	if lb.Selected().Get() != 5 {
		t.Fatalf("disabled list moved (Selected=%d)", lb.Selected().Get())
	}
	// Empty list: navigation is a no-op (rovingIndex n<=0), Enter is safe.
	empty := NewListBox(nil)
	empty.OnEvent(kd3b("ArrowDown"))
	empty.OnEvent(kd3b("Enter"))
	if empty.Selected().Get() != -1 {
		t.Fatalf("empty list moved (Selected=%d)", empty.Selected().Get())
	}
	// Non-empty list with no selection: Enter (activateCursor out of range) is
	// a no-op, and a nil OnActivate is safe.
	nl := newCursorListBox()
	nl.OnEvent(kd3b("Enter"))
	if nl.Selected().Get() != -1 {
		t.Fatalf("Enter with no cursor changed Selected=%d", nl.Selected().Get())
	}
}

// --- Table -----------------------------------------------------------------

func newCursorTable() *Table {
	rows := make([][]string, 20)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("r%d", i)}
	}
	tb := NewTable([]TableColumn{{Title: "A"}}, rows)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: TableHeaderHeight + 3*TableRowHeight})
	return tb
}

func tbVisible(t *testing.T, tb *Table) {
	t.Helper()
	vr := tb.bodyVisibleRows()
	if tb.Selected().Get() < tb.ScrollRow().Get() || tb.Selected().Get() >= tb.ScrollRow().Get()+vr {
		t.Fatalf("cursor %d outside window [%d,%d)", tb.Selected().Get(), tb.ScrollRow().Get(), tb.ScrollRow().Get()+vr)
	}
}

func TestTableKeyCursor(t *testing.T) {
	tb := newCursorTable()
	tb.OnEvent(kd3b("ArrowDown")) // -1 -> 0
	if tb.Selected().Get() != 0 {
		t.Fatalf("ArrowDown: Selected=%d", tb.Selected().Get())
	}
	tbVisible(t, tb)
	tb.OnEvent(kd3b("End")) // -> 19, scrolls
	if tb.Selected().Get() != 19 || tb.ScrollRow().Get() != 17 {
		t.Fatalf("End: Selected=%d ScrollRow=%d, want 19/17", tb.Selected().Get(), tb.ScrollRow().Get())
	}
	tbVisible(t, tb)
	tb.OnEvent(kd3b("PageUp")) // 19 -> 16
	if tb.Selected().Get() != 16 {
		t.Fatalf("PageUp: Selected=%d, want 16", tb.Selected().Get())
	}
	tbVisible(t, tb)
	tb.OnEvent(kd3b("Home"))
	if tb.Selected().Get() != 0 || tb.ScrollRow().Get() != 0 {
		t.Fatalf("Home: Selected=%d ScrollRow=%d", tb.Selected().Get(), tb.ScrollRow().Get())
	}
	tbVisible(t, tb)
	// Single-select activation is a no-op (Table has no click callback).
	tb.OnEvent(kd3b("Enter"))
	if tb.Selected().Get() != 0 {
		t.Fatalf("Enter single-select changed Selected=%d", tb.Selected().Get())
	}
}

func TestTableKeyMultiSelectPlainAndActivate(t *testing.T) {
	tb := newCursorTable()
	tb.MultiSelect = true
	tb.OnEvent(kd3b("ArrowDown")) // 0, SetRowSelection(0)
	tb.OnEvent(kd3b("ArrowDown")) // 1
	if !tb.IsRowSelected(1) || tb.IsRowSelected(0) {
		t.Fatalf("multiselect plain arrow: sel(0)=%v sel(1)=%v", tb.IsRowSelected(0), tb.IsRowSelected(1))
	}
	// Enter re-selects the cursor row exclusively.
	tb.OnEvent(kd3b("Enter"))
	if !tb.IsRowSelected(1) {
		t.Fatalf("multiselect Enter: sel(1)=%v", tb.IsRowSelected(1))
	}
}

func TestTableKeyShiftExtendsSelection(t *testing.T) {
	tb := newCursorTable()
	tb.MultiSelect = true
	// Seed Selected directly (selectedRows still nil -> exercises the seed
	// branch of extendRowSelection).
	tb.Selected().Set(5)
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown", Shift: true}) // 5 -> 6, {5,6}
	if tb.Selected().Get() != 6 || !tb.IsRowSelected(5) || !tb.IsRowSelected(6) {
		t.Fatalf("shift-down: Selected=%d sel5=%v sel6=%v", tb.Selected().Get(), tb.IsRowSelected(5), tb.IsRowSelected(6))
	}
	tbVisible(t, tb)
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown", Shift: true}) // 6 -> 7, {5,6,7}
	if tb.Selected().Get() != 7 || !tb.IsRowSelected(7) {
		t.Fatalf("shift-down 2: Selected=%d sel7=%v", tb.Selected().Get(), tb.IsRowSelected(7))
	}
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp", Shift: true}) // 7 -> 6
	if tb.Selected().Get() != 6 {
		t.Fatalf("shift-up: Selected=%d, want 6", tb.Selected().Get())
	}

	// Clamp at the edges: Shift+ArrowUp at row 0 stays at 0.
	tb.Selected().Set(0)
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp", Shift: true})
	if tb.Selected().Get() != 0 || !tb.IsRowSelected(0) {
		t.Fatalf("shift-up clamp: Selected=%d", tb.Selected().Get())
	}
	// Shift+ArrowDown at the last row stays.
	tb.Selected().Set(19)
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown", Shift: true})
	if tb.Selected().Get() != 19 {
		t.Fatalf("shift-down clamp: Selected=%d", tb.Selected().Get())
	}
	// Shift extend from no selection (Selected == -1 -> prev 0).
	fresh := newCursorTable()
	fresh.MultiSelect = true
	fresh.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown", Shift: true})
	if fresh.Selected().Get() != 1 || !fresh.IsRowSelected(0) || !fresh.IsRowSelected(1) {
		t.Fatalf("shift from none: Selected=%d sel0=%v sel1=%v", fresh.Selected().Get(), fresh.IsRowSelected(0), fresh.IsRowSelected(1))
	}
	// Shift+Arrow without MultiSelect falls through to a plain cursor move.
	single := newCursorTable()
	single.Selected().Set(3)
	single.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown", Shift: true})
	if single.Selected().Get() != 4 {
		t.Fatalf("shift w/o multiselect: Selected=%d, want 4", single.Selected().Get())
	}
}

func TestTableKeyDisabledEmptyAndNoCursorActivate(t *testing.T) {
	tb := newCursorTable()
	tb.Selected().Set(4)
	tb.Disabled = true
	tb.OnEvent(kd3b("ArrowDown"))
	tb.OnEvent(kd3b("Enter"))
	if tb.Selected().Get() != 4 {
		t.Fatalf("disabled table moved (Selected=%d)", tb.Selected().Get())
	}
	// Empty table: keys are a no-op.
	empty := NewTable([]TableColumn{{Title: "A"}}, nil)
	empty.SetBounds(Rect{X: 0, Y: 0, W: 120, H: TableHeaderHeight + 3*TableRowHeight})
	empty.OnEvent(kd3b("ArrowDown"))
	empty.OnEvent(kd3b("Enter"))
	if empty.Selected().Get() != -1 {
		t.Fatalf("empty table moved (Selected=%d)", empty.Selected().Get())
	}
	// Rows present but no cursor: Enter (activateCursor out of range) is a no-op.
	nc := newCursorTable()
	nc.MultiSelect = true
	nc.OnEvent(kd3b("Enter"))
	if nc.Selected().Get() != -1 || len(nc.SelectedRows()) != 0 {
		t.Fatalf("Enter with no cursor selected something: Selected=%d rows=%v", nc.Selected().Get(), nc.SelectedRows())
	}
	// A non-navigation key is ignored.
	nav := newCursorTable()
	nav.Selected().Set(2)
	nav.OnEvent(kd3b("Tab"))
	if nav.Selected().Get() != 2 {
		t.Fatalf("Tab moved cursor (Selected=%d)", nav.Selected().Get())
	}
}

// --- TreeView --------------------------------------------------------------

func newCursorTreeView() *TreeView {
	root, _ := manyLeaves(20) // root + 20 leaves = 21 flattened rows
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 3 * 18}) // 3 rows visible
	return tv
}

func tvVisible(t *testing.T, tv *TreeView) {
	t.Helper()
	tv.flatten()
	idx := tv.cursorRow()
	wr := tv.windowRows()
	if idx < tv.ScrollRow().Get() || idx >= tv.ScrollRow().Get()+wr {
		t.Fatalf("cursor row %d outside window [%d,%d)", idx, tv.ScrollRow().Get(), tv.ScrollRow().Get()+wr)
	}
}

func TestTreeViewKeyCursorAndActivate(t *testing.T) {
	tv := newCursorTreeView()
	var activated *TreeNode
	tv.OnActivate = func(n *TreeNode) { activated = n }

	tv.OnEvent(kd3b("ArrowDown")) // none -> row 0 (root)
	if tv.Selected().Get() != tv.Root {
		t.Fatalf("ArrowDown from none: Selected=%v", tv.Selected().Get())
	}
	tvVisible(t, tv)

	tv.OnEvent(kd3b("End")) // last row, scrolls
	tv.flatten()
	if tv.Selected().Get() != tv.rows[len(tv.rows)-1].node || tv.ScrollRow().Get() != len(tv.rows)-tv.windowRows() {
		t.Fatalf("End: ScrollRow=%d", tv.ScrollRow().Get())
	}
	tvVisible(t, tv)

	tv.OnEvent(kd3b("Home"))
	if tv.Selected().Get() != tv.Root || tv.ScrollRow().Get() != 0 {
		t.Fatalf("Home: Selected=%v ScrollRow=%d", tv.Selected().Get(), tv.ScrollRow().Get())
	}
	tv.OnEvent(kd3b("PageDown"))
	tvVisible(t, tv)
	tv.OnEvent(kd3b("ArrowUp"))
	tvVisible(t, tv)

	// A non-navigation key is ignored.
	before := tv.Selected().Get()
	tv.OnEvent(kd3b("Tab"))
	if tv.Selected().Get() != before {
		t.Fatalf("Tab moved cursor")
	}

	// Enter activates the cursor node.
	tv.OnEvent(kd3b("Enter"))
	if activated != tv.Selected().Get() {
		t.Fatalf("Enter activate: activated=%v want=%v", activated, tv.Selected().Get())
	}
	// Space too.
	activated = nil
	tv.OnEvent(kd3b(" "))
	if activated != tv.Selected().Get() {
		t.Fatalf("Space activate: activated=%v", activated)
	}
}

// structuredTree builds:
//
//	root (expanded)
//	  a (children a1,a2; collapsed)
//	  b (leaf)
//	  c (children c1; collapsed)
func structuredTree() (root, a, a1, b, c *TreeNode) {
	a1 = &TreeNode{Label: "a1"}
	a2 := &TreeNode{Label: "a2"}
	a = &TreeNode{Label: "a", Children: []*TreeNode{a1, a2}}
	b = &TreeNode{Label: "b"}
	c1 := &TreeNode{Label: "c1"}
	c = &TreeNode{Label: "c", Children: []*TreeNode{c1}}
	root = &TreeNode{Label: "root", Expanded: true, Children: []*TreeNode{a, b, c}}
	return
}

func TestTreeViewKeyExpandCollapse(t *testing.T) {
	root, a, a1, b, _ := structuredTree()
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 20 * 18}) // tall: everything visible

	// ArrowRight on a collapsed parent expands it.
	tv.Selected().Set(a)
	tv.OnEvent(kd3b("ArrowRight"))
	if !a.Expanded {
		t.Fatal("ArrowRight did not expand a")
	}
	// ArrowRight again (now expanded) descends to the first child.
	tv.OnEvent(kd3b("ArrowRight"))
	if tv.Selected().Get() != a1 {
		t.Fatalf("ArrowRight descend: Selected=%v want a1", tv.Selected().Get())
	}
	// ArrowLeft on a leaf moves to the parent.
	tv.OnEvent(kd3b("ArrowLeft"))
	if tv.Selected().Get() != a {
		t.Fatalf("ArrowLeft to parent: Selected=%v want a", tv.Selected().Get())
	}
	// ArrowLeft on the expanded parent collapses it (cursor stays on a).
	tv.OnEvent(kd3b("ArrowLeft"))
	if a.Expanded || tv.Selected().Get() != a {
		t.Fatalf("ArrowLeft collapse: Expanded=%v Selected=%v", a.Expanded, tv.Selected().Get())
	}
	// ArrowLeft again (a now collapsed) moves to the parent root.
	tv.OnEvent(kd3b("ArrowLeft"))
	if tv.Selected().Get() != root {
		t.Fatalf("ArrowLeft to root: Selected=%v", tv.Selected().Get())
	}
	// ArrowRight on the expanded root descends to its first child (a).
	tv.OnEvent(kd3b("ArrowRight"))
	if tv.Selected().Get() != a {
		t.Fatalf("ArrowRight root descend: Selected=%v want a", tv.Selected().Get())
	}
	// ArrowRight on a leaf (b) does nothing.
	tv.Selected().Set(b)
	tv.OnEvent(kd3b("ArrowRight"))
	if tv.Selected().Get() != b {
		t.Fatalf("ArrowRight on leaf moved: Selected=%v", tv.Selected().Get())
	}
	// ArrowLeft at the top level with nothing to collapse stays put.
	tv.Selected().Set(root)
	tv.Root.Expanded = false // collapse via field so ArrowLeft hits parent-search
	tv.OnEvent(kd3b("ArrowLeft"))
	if tv.Selected().Get() != root {
		t.Fatalf("ArrowLeft at top stayed? Selected=%v", tv.Selected().Get())
	}
	tv.Root.Expanded = true
}

func TestTreeViewKeyEdgeCases(t *testing.T) {
	root, a, a1, _, _ := structuredTree()
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 20 * 18})

	// ArrowRight with no cursor selects the first row.
	tv.Selected().Set(nil)
	tv.OnEvent(kd3b("ArrowRight"))
	if tv.Selected().Get() != root {
		t.Fatalf("ArrowRight from none: Selected=%v", tv.Selected().Get())
	}
	// ArrowLeft with no cursor is a no-op.
	tv.Selected().Set(nil)
	tv.OnEvent(kd3b("ArrowLeft"))
	if tv.Selected().Get() != nil {
		t.Fatalf("ArrowLeft from none moved: Selected=%v", tv.Selected().Get())
	}
	// Selected points at a not-currently-visible node (a1 under collapsed a):
	// cursorRow returns -1, so ArrowDown restarts at row 0.
	a.Expanded = false
	tv.Selected().Set(a1)
	tv.OnEvent(kd3b("ArrowDown"))
	if tv.Selected().Get() != root {
		t.Fatalf("ArrowDown from invisible cursor: Selected=%v want root", tv.Selected().Get())
	}

	// MultiSelect: a plain move also updates the selection set.
	tv.MultiSelect = true
	tv.Selected().Set(root)
	tv.OnEvent(kd3b("ArrowDown"))
	if !tv.IsSelected(tv.Selected().Get()) {
		t.Fatal("multiselect arrow did not select cursor node")
	}
	// Enter while MultiSelect collapses the selection to the cursor node and
	// fires OnActivate (activateCursor's MultiSelect branch).
	activated := (*TreeNode)(nil)
	tv.OnActivate = func(n *TreeNode) { activated = n }
	cursorNode := tv.Selected().Get()
	tv.OnEvent(kd3b("Enter"))
	if activated != cursorNode || !tv.IsSelected(cursorNode) {
		t.Fatalf("multiselect Enter: activated=%v", activated)
	}
	// Enter with no cursor on a non-empty tree is a no-op (nil-return branch).
	tv.OnActivate = func(n *TreeNode) { t.Fatalf("activated with nil cursor: %v", n) }
	tv.Selected().Set(nil)
	tv.OnEvent(kd3b("Enter"))

	// Disabled ignores keys.
	tv.Disabled = true
	keep := tv.Selected().Get()
	tv.OnEvent(kd3b("ArrowDown"))
	if tv.Selected().Get() != keep {
		t.Fatalf("disabled tree moved (Selected=%v)", tv.Selected().Get())
	}

	// Empty tree + nil OnActivate are safe.
	empty := NewTreeView(nil)
	empty.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 54})
	empty.OnEvent(kd3b("ArrowDown"))
	empty.OnEvent(kd3b("Enter"))
	if empty.Selected().Get() != nil {
		t.Fatalf("empty tree moved (Selected=%v)", empty.Selected().Get())
	}
	// Enter on a live tree with no OnActivate is safe.
	nl := NewTreeView(root)
	nl.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 54})
	nl.Selected().Set(root)
	nl.OnEvent(kd3b("Enter"))
}

// --- TreeTable -------------------------------------------------------------

func newCursorTreeTable() *TreeTable {
	tt := NewTreeTable([]TreeTableColumn{{Title: "A"}}, manyTreeTableLeaves(20))
	tt.SetBounds(Rect{X: 0, Y: 0, W: 120, H: TreeTableHeaderHeight + 3*TreeTableRowHeight})
	return tt
}

func ttVisible(t *testing.T, tt *TreeTable) {
	t.Helper()
	tt.flatten()
	idx := tt.cursorRow()
	wr := tt.bodyVisibleRows()
	if sr := tt.ScrollRow().Get(); idx < sr || idx >= sr+wr {
		t.Fatalf("cursor row %d outside window [%d,%d)", idx, sr, sr+wr)
	}
}

func TestTreeTableKeyCursorAndActivate(t *testing.T) {
	tt := newCursorTreeTable()
	tt.OnEvent(kd3b("ArrowDown")) // none -> row 0
	tt.flatten()
	if tt.Selected().Get() != tt.rows[0].node {
		t.Fatalf("ArrowDown from none: Selected=%v", tt.Selected().Get())
	}
	ttVisible(t, tt)
	tt.OnEvent(kd3b("End"))
	tt.flatten()
	if tt.Selected().Get() != tt.rows[len(tt.rows)-1].node || tt.ScrollRow().Get() != len(tt.rows)-tt.bodyVisibleRows() {
		t.Fatalf("End: ScrollRow=%d", tt.ScrollRow().Get())
	}
	ttVisible(t, tt)
	tt.OnEvent(kd3b("PageUp"))
	ttVisible(t, tt)
	tt.OnEvent(kd3b("Home"))
	if tt.ScrollRow().Get() != 0 {
		t.Fatalf("Home ScrollRow=%d", tt.ScrollRow().Get())
	}
	ttVisible(t, tt)
	// Enter/Space keep the cursor node as Selected (click-equivalent).
	keep := tt.Selected().Get()
	tt.OnEvent(kd3b("Enter"))
	tt.OnEvent(kd3b(" "))
	if tt.Selected().Get() != keep {
		t.Fatalf("Enter changed Selected: %v want %v", tt.Selected().Get(), keep)
	}
	// A non-navigation key is ignored.
	tt.OnEvent(kd3b("Tab"))
	if tt.Selected().Get() != keep {
		t.Fatalf("Tab moved cursor")
	}
}

// structuredTreeTable builds a small forest:
//
//	root0 (expanded) children [a (children a1; collapsed), b (leaf)]
//	root1 (leaf)
func structuredTreeTable() (root0, a, a1, b, root1 *TreeTableNode) {
	a1 = &TreeTableNode{Cells: []string{"a1"}}
	a = &TreeTableNode{Cells: []string{"a"}, Children: []*TreeTableNode{a1}}
	b = &TreeTableNode{Cells: []string{"b"}}
	root0 = &TreeTableNode{Cells: []string{"root0"}, Expanded: true, Children: []*TreeTableNode{a, b}}
	root1 = &TreeTableNode{Cells: []string{"root1"}}
	return
}

func TestTreeTableKeyExpandCollapse(t *testing.T) {
	root0, a, a1, b, root1 := structuredTreeTable()
	tt := NewTreeTable([]TreeTableColumn{{Title: "A"}}, []*TreeTableNode{root0, root1})
	tt.SetBounds(Rect{X: 0, Y: 0, W: 120, H: TreeTableHeaderHeight + 20*TreeTableRowHeight})

	// ArrowRight expands a collapsed parent.
	tt.Selected().Set(a)
	tt.OnEvent(kd3b("ArrowRight"))
	if !a.Expanded {
		t.Fatal("ArrowRight did not expand a")
	}
	// ArrowRight (expanded) descends to first child.
	tt.OnEvent(kd3b("ArrowRight"))
	if tt.Selected().Get() != a1 {
		t.Fatalf("descend: Selected=%v want a1", tt.Selected().Get())
	}
	// ArrowLeft on the leaf child goes to parent a.
	tt.OnEvent(kd3b("ArrowLeft"))
	if tt.Selected().Get() != a {
		t.Fatalf("to parent: Selected=%v want a", tt.Selected().Get())
	}
	// ArrowLeft collapses expanded a.
	tt.OnEvent(kd3b("ArrowLeft"))
	if a.Expanded {
		t.Fatal("ArrowLeft did not collapse a")
	}
	// ArrowLeft again -> parent root0.
	tt.OnEvent(kd3b("ArrowLeft"))
	if tt.Selected().Get() != root0 {
		t.Fatalf("to root0: Selected=%v", tt.Selected().Get())
	}
	// ArrowRight on a leaf (b) does nothing.
	tt.Selected().Set(b)
	tt.OnEvent(kd3b("ArrowRight"))
	if tt.Selected().Get() != b {
		t.Fatalf("ArrowRight on leaf moved: Selected=%v", tt.Selected().Get())
	}
	// ArrowLeft on a top-level leaf (root1) with no parent stays put.
	tt.Selected().Set(root1)
	tt.OnEvent(kd3b("ArrowLeft"))
	if tt.Selected().Get() != root1 {
		t.Fatalf("ArrowLeft top-level leaf moved: Selected=%v", tt.Selected().Get())
	}
}

func TestTreeTableKeyEdgeCases(t *testing.T) {
	root0, _, a1, _, _ := structuredTreeTable()
	tt := NewTreeTable([]TreeTableColumn{{Title: "A"}}, []*TreeTableNode{root0})
	tt.SetBounds(Rect{X: 0, Y: 0, W: 120, H: TreeTableHeaderHeight + 20*TreeTableRowHeight})

	// ArrowRight with no cursor selects the first row.
	tt.Selected().Set(nil)
	tt.OnEvent(kd3b("ArrowRight"))
	if tt.Selected().Get() != root0 {
		t.Fatalf("ArrowRight from none: Selected=%v", tt.Selected().Get())
	}
	// ArrowLeft with no cursor is a no-op.
	tt.Selected().Set(nil)
	tt.OnEvent(kd3b("ArrowLeft"))
	if tt.Selected().Get() != nil {
		t.Fatalf("ArrowLeft from none moved: Selected=%v", tt.Selected().Get())
	}
	// Enter with no cursor (cursorRow == -1) is a no-op.
	tt.Selected().Set(nil)
	tt.OnEvent(kd3b("Enter"))
	if tt.Selected().Get() != nil {
		t.Fatalf("Enter from none moved: Selected=%v", tt.Selected().Get())
	}
	// Selected on a not-currently-visible node -> cursorRow -1 -> ArrowDown row 0.
	tt.Selected().Set(a1) // a1 hidden (a collapsed)
	tt.OnEvent(kd3b("ArrowDown"))
	if tt.Selected().Get() != root0 {
		t.Fatalf("ArrowDown from invisible cursor: Selected=%v", tt.Selected().Get())
	}
	// Disabled ignores keys.
	tt.Disabled = true
	keep := tt.Selected().Get()
	tt.OnEvent(kd3b("ArrowDown"))
	if tt.Selected().Get() != keep {
		t.Fatalf("disabled tree table moved (Selected=%v)", tt.Selected().Get())
	}
	// Empty forest is safe.
	empty := NewTreeTable([]TreeTableColumn{{Title: "A"}}, nil)
	empty.SetBounds(Rect{X: 0, Y: 0, W: 120, H: TreeTableHeaderHeight + 3*TreeTableRowHeight})
	empty.OnEvent(kd3b("ArrowDown"))
	empty.OnEvent(kd3b("Enter"))
	if empty.Selected().Get() != nil {
		t.Fatalf("empty forest moved (Selected=%v)", empty.Selected().Get())
	}
}

// --- Menu ------------------------------------------------------------------

// menuFixture builds a Menu whose enabled rows are index 0 and 3 (a separator
// and a disabled/nil-Action row sit between), plus a fire counter and a close
// counter.
func menuFixture() (m *Menu, fired *[3]int, closes *int) {
	f := &[3]int{}
	c := 0
	m = NewMenu([]MenuItem{
		{Label: "A", Action: func() { f[0]++ }},
		{Separator: true},
		{Label: "B (disabled)"}, // nil Action
		{Label: "C", Action: func() { f[2]++ }},
	})
	m.OnClose = func() { c++ }
	// Return pointer to the counter via closure indirection.
	closes = &c
	return m, f, closes
}

func TestMenuKeyboardNavigation(t *testing.T) {
	m, fired, closes := menuFixture()

	// ArrowDown from no hover -> first enabled (0); again skips sep+disabled -> 3.
	m.OnEvent(kd3b("ArrowDown"))
	if m.Hover().Get() != 0 {
		t.Fatalf("ArrowDown: Hover=%d, want 0", m.Hover().Get())
	}
	m.OnEvent(kd3b("ArrowDown"))
	if m.Hover().Get() != 3 {
		t.Fatalf("ArrowDown skip: Hover=%d, want 3", m.Hover().Get())
	}
	// ArrowDown wraps back to 0.
	m.OnEvent(kd3b("ArrowDown"))
	if m.Hover().Get() != 0 {
		t.Fatalf("ArrowDown wrap: Hover=%d, want 0", m.Hover().Get())
	}
	// ArrowUp wraps to the last enabled (3).
	m.OnEvent(kd3b("ArrowUp"))
	if m.Hover().Get() != 3 {
		t.Fatalf("ArrowUp wrap: Hover=%d, want 3", m.Hover().Get())
	}
	// Enter fires the hovered Action and closes.
	m.OnEvent(kd3b("Enter"))
	if fired[2] != 1 || *closes != 1 {
		t.Fatalf("Enter: fired[2]=%d closes=%d", fired[2], *closes)
	}
	// Space fires too.
	m.Hover().Set(0)
	m.OnEvent(kd3b(" "))
	if fired[0] != 1 {
		t.Fatalf("Space: fired[0]=%d", fired[0])
	}
	// Escape closes.
	before := *closes
	m.OnEvent(kd3b("Escape"))
	if *closes != before+1 {
		t.Fatalf("Escape did not close: closes=%d", *closes)
	}
}

func TestMenuKeyboardArrowUpFromNoHover(t *testing.T) {
	m, _, _ := menuFixture()
	// First key ArrowUp with Hover == -1 lands on the last enabled row.
	m.OnEvent(kd3b("ArrowUp"))
	if m.Hover().Get() != 3 {
		t.Fatalf("ArrowUp from none: Hover=%d, want 3", m.Hover().Get())
	}
}

func TestMenuKeyboardEnterOnNonEnabledIsNoop(t *testing.T) {
	m, fired, closes := menuFixture()
	// Hover on a separator/disabled row: Enter does nothing.
	m.Hover().Set(1) // separator
	m.OnEvent(kd3b("Enter"))
	m.Hover().Set(2) // disabled (nil Action)
	m.OnEvent(kd3b("Enter"))
	m.Hover().Set(-1) // no hover
	m.OnEvent(kd3b("Enter"))
	if fired[0]+fired[2] != 0 || *closes != 0 {
		t.Fatalf("Enter on non-enabled fired something: fired=%v closes=%d", *fired, *closes)
	}
}

func TestMenuKeyboardNoEnabledAndEmptyAndDisabled(t *testing.T) {
	// A menu with no enabled row: moveHover leaves Hover untouched.
	m := NewMenu([]MenuItem{{Separator: true}, {Label: "x"}}) // no Action anywhere
	m.OnEvent(kd3b("ArrowDown"))
	if m.Hover().Get() != -1 {
		t.Fatalf("no-enabled ArrowDown moved Hover=%d", m.Hover().Get())
	}
	// Empty menu: moveHover n==0 guard.
	e := NewMenu(nil)
	e.OnEvent(kd3b("ArrowDown"))
	if e.Hover().Get() != -1 {
		t.Fatalf("empty menu moved Hover=%d", e.Hover().Get())
	}
	// A nil OnClose Escape is safe.
	nc := NewMenu([]MenuItem{{Label: "A", Action: func() {}}})
	nc.OnEvent(kd3b("Escape"))
	// Disabled menu ignores keys.
	d, fired, _ := menuFixture()
	d.Disabled = true
	d.OnEvent(kd3b("ArrowDown"))
	d.OnEvent(kd3b("Enter"))
	if d.Hover().Get() != -1 || fired[0]+fired[2] != 0 {
		t.Fatalf("disabled menu responded: Hover=%d fired=%v", d.Hover().Get(), *fired)
	}
	// An unhandled key (some letter) is ignored.
	u, _, _ := menuFixture()
	u.OnEvent(kd3b("x"))
	if u.Hover().Get() != -1 {
		t.Fatalf("unhandled key moved Hover=%d", u.Hover().Get())
	}
}

// --- ContextMenu -----------------------------------------------------------

func TestContextMenuKeyboard(t *testing.T) {
	fired := 0
	menu := NewMenu([]MenuItem{{Label: "A", Action: func() { fired++ }}})
	cm := NewContextMenu(menu)
	cm.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	cm.Popup(20, 20)
	// ArrowDown highlights the first item; Enter fires it and closes the overlay.
	cm.OnEvent(kd3b("ArrowDown"))
	if menu.Hover().Get() != 0 {
		t.Fatalf("context ArrowDown: Hover=%d", menu.Hover().Get())
	}
	cm.OnEvent(kd3b("Enter"))
	if fired != 1 || cm.Open().Get() {
		t.Fatalf("context Enter: fired=%d open=%v", fired, cm.Open().Get())
	}
	// Escape on an open context menu closes it (via the Menu's OnClose).
	cm.Popup(20, 20)
	cm.OnEvent(kd3b("Escape"))
	if cm.Open().Get() {
		t.Fatal("Escape did not close the context menu")
	}
	// A key on a closed context menu is a no-op.
	cm.OnEvent(kd3b("ArrowDown"))
	if cm.Open().Get() {
		t.Fatal("key opened a closed context menu")
	}
}

// --- MenuBar ---------------------------------------------------------------

func newKeyMenuBar() *MenuBar {
	b := NewMenuBar()
	b.AddMenu("File", NewMenu([]MenuItem{{Label: "New", Action: func() {}}, {Label: "Open", Action: func() {}}}))
	b.AddMenu("Edit", NewMenu([]MenuItem{{Label: "Cut", Action: func() {}}}))
	b.AddMenu("View", NewMenu([]MenuItem{{Label: "Zoom", Action: func() {}}}))
	b.SetBounds(Rect{X: 0, Y: 0, W: 240, H: MenuBarH})
	return b
}

func TestMenuBarKeyboardArrows(t *testing.T) {
	b := newKeyMenuBar()
	// Arrows do nothing while nothing is open.
	b.OnEvent(kd3b("ArrowRight"))
	if b.Active().Get() != -1 {
		t.Fatalf("ArrowRight with nothing open: Active=%d", b.Active().Get())
	}
	b.OnEvent(kd3b("ArrowDown"))
	if b.Active().Get() != -1 {
		t.Fatalf("ArrowDown with nothing open: Active=%d", b.Active().Get())
	}
	// Open File (Alt+F), then ArrowRight/Left move Active with wrapping.
	b.OnEvent(kd3b("Alt+F")) // Active 0
	b.OnEvent(kd3b("ArrowRight"))
	if b.Active().Get() != 1 {
		t.Fatalf("ArrowRight: Active=%d, want 1", b.Active().Get())
	}
	b.OnEvent(kd3b("ArrowRight")) // 2
	b.OnEvent(kd3b("ArrowRight")) // wrap -> 0
	if b.Active().Get() != 0 {
		t.Fatalf("ArrowRight wrap: Active=%d", b.Active().Get())
	}
	b.OnEvent(kd3b("ArrowLeft")) // wrap -> 2
	if b.Active().Get() != 2 {
		t.Fatalf("ArrowLeft wrap: Active=%d", b.Active().Get())
	}
	// ArrowDown enters the open menu: its first item is highlighted.
	b.OnEvent(kd3b("ArrowDown"))
	if b.Menus[2].Hover().Get() != 0 {
		t.Fatalf("ArrowDown into menu: Hover=%d", b.Menus[2].Hover().Get())
	}
	// Escape still closes.
	b.OnEvent(kd3b("Escape"))
	if b.Active().Get() != -1 {
		t.Fatalf("Escape: Active=%d", b.Active().Get())
	}
}

func TestMenuBarKeyboardDisabledAndNilMenu(t *testing.T) {
	// Disabled ignores keys (including Alt/Escape).
	b := newKeyMenuBar()
	b.Active().Set(0)
	b.Disabled = true
	b.OnEvent(kd3b("ArrowRight"))
	if b.Active().Get() != 0 {
		t.Fatalf("disabled bar moved Active=%d", b.Active().Get())
	}
	// ArrowDown with a nil menu at Active is a safe no-op.
	nb := NewMenuBar()
	nb.Names = []string{"File"}
	nb.Menus = []*Menu{nil}
	nb.SetBounds(Rect{X: 0, Y: 0, W: 80, H: MenuBarH})
	nb.Active().Set(0)
	nb.OnEvent(kd3b("ArrowDown")) // must not panic
	// ArrowDown when Active is out of range (mismatched Names/Menus) is a no-op.
	mb := NewMenuBar()
	mb.Names = []string{"A", "B", "C"}
	mb.Menus = []*Menu{NewMenu(nil), NewMenu(nil)} // shorter than Names
	mb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: MenuBarH})
	mb.Active().Set(2)            // >= len(Menus)
	mb.OnEvent(kd3b("ArrowDown")) // must not panic, no highlight
}
