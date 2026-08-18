// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- TreeView multi-select model ------------------------------------------
//
// Layout shared by several tests below (all children of an Expanded
// root, so all rows are visible at row height 18):
//
//	idx  node   depth
//	 0   root    0
//	 1   a       1
//	 2   b       1   (Expanded, has child b1)
//	 3   b1      2
//	 4   c       1
//	 5   d       1   (collapsed, has hidden child d1)
//	 6   e       1
//
// Label clicks use X=80, which clears every chevron hit-box at every
// depth used here (see TestTreeViewSelectFiresOnActivate for the same
// convention).

func newMultiSelectTree() (root, a, b, b1, c, d, d1, e *TreeNode) {
	b1 = &TreeNode{Label: "b1"}
	b = &TreeNode{Label: "b", Expanded: true, Children: []*TreeNode{b1}}
	a = &TreeNode{Label: "a"}
	c = &TreeNode{Label: "c"}
	d1 = &TreeNode{Label: "d1"}
	d = &TreeNode{Label: "d", Expanded: false, Children: []*TreeNode{d1}}
	e = &TreeNode{Label: "e"}
	root = &TreeNode{Label: "root", Expanded: true,
		Children: []*TreeNode{a, b, c, d, e}}
	return
}

func clickLabel(tv *TreeView, rowIdx int, ctrl, shift bool) {
	tv.OnEvent(Event{Kind: EventClick, X: 80, Y: rowIdx * 18, Ctrl: ctrl, Shift: shift})
}

// --- regression: MultiSelect defaults false, behavior byte-identical -----

func TestTreeViewMultiSelectDefaultsFalse(t *testing.T) {
	root, _, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	if tv.MultiSelect {
		t.Fatal("MultiSelect should default to false")
	}
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	clickLabel(tv, 1, false, false) // a
	if tv.Selected().Get() == nil || tv.Selected().Get().Label != "a" {
		t.Fatalf("Selected = %+v, want a", tv.Selected().Get())
	}
	if len(tv.SelectedNodes()) != 0 {
		t.Fatal("SelectedNodes should be empty when MultiSelect is false")
	}
	if tv.IsSelected(tv.Selected().Get()) {
		t.Fatal("IsSelected should be false when MultiSelect is false")
	}
}

func TestTreeViewSingleSelectIgnoresModifiers(t *testing.T) {
	// Ctrl/Shift on Event must not change single-select behavior:
	// each click still replaces Selected with exactly the clicked
	// node, regardless of modifier flags.
	root, _, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})

	clickLabel(tv, 1, false, false) // a
	if tv.Selected().Get().Label != "a" {
		t.Fatalf("Selected = %s, want a", tv.Selected().Get().Label)
	}
	clickLabel(tv, 4, true, false) // Ctrl-click c
	if tv.Selected().Get().Label != "c" {
		t.Fatalf("after Ctrl-click: Selected = %s, want c", tv.Selected().Get().Label)
	}
	clickLabel(tv, 2, false, true) // Shift-click b
	if tv.Selected().Get().Label != "b" {
		t.Fatalf("after Shift-click: Selected = %s, want b", tv.Selected().Get().Label)
	}
}

func TestTreeViewSingleSelectExpandCollapseUnaffected(t *testing.T) {
	// Regression from the pre-multiselect suite: chevron clicks still
	// toggle Expanded and never touch Selected, with MultiSelect off.
	leaf := &TreeNode{Label: "leaf"}
	root := &TreeNode{Label: "root", Children: []*TreeNode{leaf}}
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	tv.OnEvent(Event{Kind: EventClick, X: 4, Y: 5})
	if !root.Expanded {
		t.Fatal("chevron click should expand")
	}
	if tv.Selected().Get() != nil {
		t.Fatal("chevron click should not select")
	}
}

// --- MultiSelect: plain / Ctrl / Shift click semantics --------------------

func TestTreeViewMultiSelectPlainClickSelectsOnlyOne(t *testing.T) {
	root, a, _, _, c, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})

	clickLabel(tv, 1, false, false) // a
	if !tv.IsSelected(a) || tv.Selected().Get() != a {
		t.Fatal("plain click should select + anchor a")
	}
	clickLabel(tv, 4, false, false) // c
	if tv.IsSelected(a) {
		t.Fatal("plain click should clear prior selection (a still selected)")
	}
	if !tv.IsSelected(c) || tv.Selected().Get() != c {
		t.Fatal("plain click should select + anchor c")
	}
	if got := tv.SelectedNodes(); len(got) != 1 || got[0] != c {
		t.Fatalf("SelectedNodes = %+v, want [c]", got)
	}
}

func TestTreeViewMultiSelectCtrlToggle(t *testing.T) {
	root, a, _, _, c, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})

	clickLabel(tv, 1, true, false) // Ctrl-click a
	if !tv.IsSelected(a) {
		t.Fatal("Ctrl-click should add a to the selection")
	}
	clickLabel(tv, 4, true, false) // Ctrl-click c
	if !tv.IsSelected(a) || !tv.IsSelected(c) {
		t.Fatal("Ctrl-click should add without clearing prior selection")
	}
	if tv.Selected().Get() != c {
		t.Fatal("Ctrl-click should move the anchor to the clicked node")
	}
	clickLabel(tv, 1, true, false) // Ctrl-click a again: toggle off
	if tv.IsSelected(a) {
		t.Fatal("second Ctrl-click on a should remove it")
	}
	if !tv.IsSelected(c) {
		t.Fatal("c should remain selected")
	}
}

func TestTreeViewMultiSelectShiftRangeOverVisibleOrder(t *testing.T) {
	root, a, b, b1, c, d, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})

	clickLabel(tv, 1, false, false) // plain click a: anchor
	clickLabel(tv, 5, false, true)  // Shift-click d

	want := []*TreeNode{a, b, b1, c, d}
	got := tv.SelectedNodes()
	if len(got) != len(want) {
		t.Fatalf("SelectedNodes = %+v, want %+v", got, want)
	}
	for i, n := range want {
		if got[i] != n {
			t.Fatalf("SelectedNodes[%d] = %s, want %s", i, got[i].Label, n.Label)
		}
	}
	if tv.IsSelected(root) {
		t.Fatal("root (before anchor) should not be in range")
	}
}

func TestTreeViewMultiSelectShiftRangeSkipsCollapsedChildren(t *testing.T) {
	// d is collapsed; d1 must never enter the range even though it
	// sits between d and e in the underlying tree, because it isn't
	// part of the visible flattened order.
	root, _, _, _, _, d, d1, e := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})

	clickLabel(tv, 5, false, false) // plain click d: anchor (visible idx 5)
	clickLabel(tv, 6, false, true)  // Shift-click e (visible idx 6)

	if !tv.IsSelected(d) || !tv.IsSelected(e) {
		t.Fatal("range endpoints should be selected")
	}
	if tv.IsSelected(d1) {
		t.Fatal("collapsed child d1 must not be selected by a visible-order range")
	}
}

func TestTreeViewMultiSelectShiftRangeIsInclusiveBothDirections(t *testing.T) {
	// Clicking backwards (anchor after the shift target) must still
	// select the inclusive range, in visible order.
	root, a, b, b1, c, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})

	clickLabel(tv, 4, false, false) // plain click c: anchor
	clickLabel(tv, 1, false, true)  // Shift-click a (before anchor)

	for _, n := range []*TreeNode{a, b, b1, c} {
		if !tv.IsSelected(n) {
			t.Fatalf("%s should be selected in backwards range", n.Label)
		}
	}
}

func TestTreeViewMultiSelectShiftWithNoAnchorFallsBackToPlainSelect(t *testing.T) {
	root, _, _, _, c, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})

	// No prior click at all: Selected is nil, so Shift-click has no
	// anchor to range from and must behave like a plain click.
	clickLabel(tv, 4, false, true) // Shift-click c
	if !tv.IsSelected(c) || tv.Selected().Get() != c {
		t.Fatal("Shift-click with no anchor should select just the clicked node")
	}
	if got := tv.SelectedNodes(); len(got) != 1 {
		t.Fatalf("SelectedNodes = %+v, want exactly [c]", got)
	}
}

func TestTreeViewMultiSelectDisclosureClickDoesNotAlterSelection(t *testing.T) {
	root, a, b, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})

	clickLabel(tv, 1, true, false) // Ctrl-select a
	clickLabel(tv, 2, true, false) // Ctrl-select b
	before := tv.SelectedNodes()

	// Click b's chevron (depth 1: chevronX=20, sensitive [17,28)).
	tv.OnEvent(Event{Kind: EventClick, X: 20, Y: 2 * 18})
	if b.Expanded {
		t.Fatal("chevron click on expanded b should collapse it")
	}
	after := tv.SelectedNodes()
	if len(before) != len(after) {
		t.Fatalf("disclosure click changed selection: before=%+v after=%+v", before, after)
	}
	if !tv.IsSelected(a) || !tv.IsSelected(b) {
		t.Fatal("disclosure click must not alter the selection set")
	}
}

// --- selection-model methods, called directly ------------------------------

func TestTreeViewSetSelectionAndClear(t *testing.T) {
	root, a, _, _, c, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true

	tv.SetSelection(a, c)
	if !tv.IsSelected(a) || !tv.IsSelected(c) {
		t.Fatal("SetSelection should select both nodes")
	}
	if tv.Selected().Get() != c {
		t.Fatal("SetSelection should anchor the last node")
	}

	tv.ClearSelection()
	if tv.IsSelected(a) || tv.IsSelected(c) {
		t.Fatal("ClearSelection should empty the set")
	}
	if tv.Selected().Get() != c {
		t.Fatal("ClearSelection should not touch the anchor")
	}
	if got := tv.SelectedNodes(); got != nil {
		t.Fatalf("SelectedNodes after ClearSelection = %+v, want nil", got)
	}
}

func TestTreeViewSetSelectionEmptyLeavesAnchorAlone(t *testing.T) {
	root, a, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.Selected().Set(a)

	tv.SetSelection() // no nodes
	if tv.Selected().Get() != a {
		t.Fatal("SetSelection with no nodes should not move the anchor")
	}
	if got := tv.SelectedNodes(); got != nil {
		t.Fatalf("SelectedNodes = %+v, want nil", got)
	}
}

func TestTreeViewToggleSelectNilNoop(t *testing.T) {
	root, _, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.ToggleSelect(nil) // must not panic or create a bogus entry
	if got := tv.SelectedNodes(); got != nil {
		t.Fatalf("SelectedNodes = %+v, want nil", got)
	}
}

func TestTreeViewIsSelectedNilMapSafe(t *testing.T) {
	root, a, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	if tv.IsSelected(a) {
		t.Fatal("IsSelected should be false before any selection call")
	}
	if tv.IsSelected(nil) {
		t.Fatal("IsSelected(nil) should be false")
	}
}

func TestTreeViewSelectRangeNoopWhenNodeNotVisible(t *testing.T) {
	root, a, _, _, _, d, d1, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true

	// b is a valid endpoint, d1 is hidden (d is collapsed): the whole
	// call must no-op, and it must not panic.
	tv.SelectRange(a, d1)
	if got := tv.SelectedNodes(); got != nil {
		t.Fatalf("SelectRange with a hidden endpoint should no-op, got %+v", got)
	}

	// Neither endpoint is even in the tree.
	stray := &TreeNode{Label: "stray"}
	tv.SelectRange(stray, d)
	if got := tv.SelectedNodes(); got != nil {
		t.Fatalf("SelectRange with an unrelated node should no-op, got %+v", got)
	}
}

func TestTreeViewSelectRangeAccumulatesAcrossCalls(t *testing.T) {
	root, a, b, b1, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true

	tv.SelectRange(a, b) // selects root..? no: a..b -> a,b
	tv.SelectRange(b1, b1)
	if !tv.IsSelected(a) || !tv.IsSelected(b) || !tv.IsSelected(b1) {
		t.Fatal("SelectRange calls should accumulate, not replace")
	}
}

func TestTreeViewSelectedNodesVisibleOrderIndependentOfSelectionOrder(t *testing.T) {
	root, a, _, _, c, _, _, e := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true

	// Select in reverse-of-visible order; SelectedNodes must still
	// come back in visible (pre-order) order.
	tv.SetSelection(e, c, a)
	got := tv.SelectedNodes()
	want := []*TreeNode{a, c, e}
	if len(got) != len(want) {
		t.Fatalf("SelectedNodes = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SelectedNodes[%d] = %s, want %s", i, got[i].Label, want[i].Label)
		}
	}
}

// --- Draw: multi-highlight vs single-highlight -----------------------------

func TestTreeViewMultiSelectDrawHighlightsEverySelectedNode(t *testing.T) {
	root, a, _, _, c, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetSelection(a, c)
	// Non-zero bounds offset to catch any hard-coded-origin bugs.
	tv.SetBounds(Rect{X: 10, Y: 5, W: 150, H: 200})

	w := 200
	buf := makeSurface(w, 220)
	theme := DefaultLight()
	tv.Draw(newP(buf, w), theme)

	// Rows, in surface Y: root=5, a=23, b=41, c=59, d=77, e=95
	// (r.Y=5, rh=18). Sample at x=100 (inside the row, right of
	// x=10..160 bounds) for each row's background.
	rowY := func(i int) int { return 5 + i*18 }
	x := 100

	if got := pixelAt(buf, w, x, rowY(1)); got != theme.Accent {
		t.Fatalf("a (selected) bg = %+v, want Accent", got)
	}
	if got := pixelAt(buf, w, x, rowY(4)); got != theme.Accent {
		t.Fatalf("c (selected) bg = %+v, want Accent", got)
	}
	if got := pixelAt(buf, w, x, rowY(0)); got != theme.Surface {
		t.Fatalf("root (unselected) bg = %+v, want Surface", got)
	}
	if got := pixelAt(buf, w, x, rowY(2)); got != theme.Surface {
		t.Fatalf("b (unselected) bg = %+v, want Surface", got)
	}
}

func TestTreeViewSingleSelectDrawOnlyHighlightsSelected(t *testing.T) {
	root, a, _, _, c, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	// MultiSelect left false: even if the (unexported) selection set
	// were somehow populated, Draw must ignore it and paint solely
	// based on Selected.
	tv.Selected().Set(c)

	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	w := 200
	buf := makeSurface(w, 200)
	theme := DefaultLight()
	tv.Draw(newP(buf, w), theme)

	rowY := func(i int) int { return i * 18 }
	x := 100
	if got := pixelAt(buf, w, x, rowY(4)); got != theme.Accent {
		t.Fatalf("c (Selected) bg = %+v, want Accent", got)
	}
	if got := pixelAt(buf, w, x, rowY(1)); got != theme.Surface {
		t.Fatalf("a (not Selected) bg = %+v, want Surface", got)
	}
	_ = a
}

// --- nil-root edge guards with MultiSelect --------------------------------

func TestTreeViewMultiSelectNilRootNoPanic(t *testing.T) {
	tv := NewTreeView(nil)
	tv.MultiSelect = true
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	tv.Draw(newP(makeSurface(200, 100), 200), DefaultLight())
	tv.OnEvent(Event{Kind: EventClick, X: 50, Y: 5, Ctrl: true})
	tv.OnEvent(Event{Kind: EventClick, X: 50, Y: 5, Shift: true})
	if got := tv.SelectedNodes(); got != nil {
		t.Fatalf("SelectedNodes on empty tree = %+v, want nil", got)
	}
}
