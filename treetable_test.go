// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"fmt"
	"testing"
)

// --- fixtures --------------------------------------------------------------

// newTreeTableFixture builds a small forest exercising every shape the
// tests below need: a leaf (a), an expanded branch (b) whose own child
// (b1) is itself a COLLAPSED branch (so its chevron sits at depth 1 —
// chevronX = 1*TreeIndentW+4 = 20), a collapsed branch (d) at depth 0,
// and a trailing leaf (c).
//
// Flattened (expand-aware) order: a(0), b(0), b1(1), d(0), c(0) — 5 rows.
// b1a and d1 are hidden (their parents are collapsed).
func newTreeTableFixture() (cols []TreeTableColumn, root []*TreeTableNode, a, b, b1, b1a, d, d1, c *TreeTableNode) {
	b1a = &TreeTableNode{Cells: []string{"b1a", "21"}}
	b1 = &TreeTableNode{Cells: []string{"b1", "20"}, Children: []*TreeTableNode{b1a}}
	b = &TreeTableNode{Cells: []string{"b", "10"}, Expanded: true, Children: []*TreeTableNode{b1}}
	a = &TreeTableNode{Cells: []string{"a", "1"}}
	d1 = &TreeTableNode{Cells: []string{"d1", "41"}}
	d = &TreeTableNode{Cells: []string{"d", "40"}, Children: []*TreeTableNode{d1}}
	c = &TreeTableNode{Cells: []string{"c", "30"}}
	root = []*TreeTableNode{a, b, d, c}
	cols = []TreeTableColumn{{Title: "Name"}, {Title: "Value", Align: AlignRight}}
	return
}

// manyTreeTableLeaves builds n flat (childless) top-level nodes labeled
// n0..n(n-1), for tests that need a tree taller than any reasonable
// viewport. Unlike TreeView, TreeTable's Root is already a forest, so no
// synthetic wrapper node is needed: flattened row i == nodes[i].
func manyTreeTableLeaves(n int) []*TreeTableNode {
	nodes := make([]*TreeTableNode, n)
	for i := range nodes {
		nodes[i] = &TreeTableNode{Cells: []string{fmt.Sprintf("n%d", i)}}
	}
	return nodes
}

// --- construction ------------------------------------------------------

func TestNewTreeTable(t *testing.T) {
	cols, root, _, _, _, _, _, _, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	if len(tt.Columns) != 2 || len(tt.Root) != 4 {
		t.Fatalf("shape = %+v", tt)
	}
	if tt.Selected().Get() != nil || tt.ScrollRow().Get() != 0 {
		t.Fatalf("fresh TreeTable should have no selection + ScrollRow=0, got sel=%v scroll=%d",
			tt.Selected().Get(), tt.ScrollRow().Get())
	}
}

// TestTreeTableZeroValueObservables proves a bare &TreeTable{} (no constructor)
// lazily materialises both Observables on first accessor call: Selected reads
// the nil zero node, ScrollRow the 0 zero row, and a host can bind either.
func TestTreeTableZeroValueObservables(t *testing.T) {
	tt := &TreeTable{}
	if got := tt.Selected().Get(); got != nil {
		t.Fatalf("zero-value Selected().Get() = %v, want nil", got)
	}
	if got := tt.ScrollRow().Get(); got != 0 {
		t.Fatalf("zero-value ScrollRow().Get() = %d, want 0", got)
	}

	// A host binds both Observables and must see every change.
	var gotSel *TreeTableNode
	gotScroll := -1
	tt.Selected().Subscribe(func(n *TreeTableNode) { gotSel = n })
	tt.ScrollRow().Subscribe(func(n int) { gotScroll = n })

	node := &TreeTableNode{Cells: []string{"x"}}
	tt.Selected().Set(node)
	tt.ScrollRow().Set(7)
	if gotSel != node {
		t.Fatalf("Selected subscriber saw %v, want the bound node", gotSel)
	}
	if gotScroll != 7 {
		t.Fatalf("ScrollRow subscriber saw %d, want 7", gotScroll)
	}
}

// --- flatten respects Expanded ------------------------------------------

func TestTreeTableFlattenRespectsExpanded(t *testing.T) {
	cols, root, a, b, b1, _, d, _, c := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.flatten()
	want := []*TreeTableNode{a, b, b1, d, c}
	if len(tt.rows) != len(want) {
		t.Fatalf("flattened len = %d, want %d (%v)", len(tt.rows), len(want), tt.rows)
	}
	for i, row := range tt.rows {
		if row.node != want[i] {
			t.Fatalf("row %d = %q, want %q", i, row.node.Cells[0], want[i].Cells[0])
		}
	}
	if tt.rows[2].depth != 1 {
		t.Fatalf("b1 depth = %d, want 1", tt.rows[2].depth)
	}
}

// --- header + column alignment ------------------------------------------

func TestTreeTableHeaderPainted(t *testing.T) {
	cols, root, _, _, _, _, _, _, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150})
	theme := DefaultLight()
	buf := makeSurface(200, 150)
	tt.Draw(newP(buf, 200), theme)

	if pixelAt(buf, 200, 5, 0) != theme.SurfaceAlt {
		t.Fatal("header background should be SurfaceAlt")
	}
	if pixelAt(buf, 200, 5, TreeTableHeaderHeight-1) != theme.Border {
		t.Fatal("header bottom edge should be Border")
	}
}

func TestTreeTableColumnAlignmentAffectsRender(t *testing.T) {
	root := []*TreeTableNode{{Cells: []string{"name", "value"}}}
	left := NewTreeTable([]TreeTableColumn{{Title: "A"}, {Title: "B", Width: 100}}, root)
	right := NewTreeTable([]TreeTableColumn{{Title: "A"}, {Title: "B", Width: 100, Align: AlignRight}}, root)
	left.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 60})
	right.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 60})
	theme := DefaultLight()

	bufL := makeSurface(200, 60)
	left.Draw(newP(bufL, 200), theme)
	bufR := makeSurface(200, 60)
	right.Draw(newP(bufR, 200), theme)

	if bytes.Equal(bufL, bufR) {
		t.Fatal("AlignRight column should render differently from AlignLeft")
	}
}

// --- indentation by depth (proven via chevron hit-testing) --------------

func TestTreeTableIndentationByDepthChevronOffsets(t *testing.T) {
	cols, root, _, b, b1, _, _, _, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150}) // fits all 5 rows, unwindowed

	// b is at depth 0 (row 1): chevronX = 4.
	tt.OnEvent(Event{Kind: EventClick, X: 4, Y: TreeTableHeaderHeight + TreeTableRowHeight + 1})
	if b.Expanded {
		t.Fatal("depth-0 chevron click should have collapsed b")
	}
	tt.OnEvent(Event{Kind: EventClick, X: 4, Y: TreeTableHeaderHeight + TreeTableRowHeight + 1})
	if !b.Expanded {
		t.Fatal("second depth-0 chevron click should re-expand b")
	}

	// b1 is at depth 1 (row 2, still visible since b is re-expanded):
	// chevronX = 1*TreeIndentW+4 = 20.
	tt.OnEvent(Event{Kind: EventClick, X: 20, Y: TreeTableHeaderHeight + 2*TreeTableRowHeight + 1})
	if !b1.Expanded {
		t.Fatal("depth-1 chevron click at X=20 should have expanded b1")
	}
}

// --- disclosure toggle expands/collapses + reflows -----------------------

func TestTreeTableDisclosureToggleReflows(t *testing.T) {
	cols, root, _, _, _, _, d, _, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150})

	tt.flatten()
	before := len(tt.rows) // 5

	// d is row index 3 (a,b,b1,d,c): local y = header + 3*rowHeight.
	tt.OnEvent(Event{Kind: EventClick, X: 4, Y: TreeTableHeaderHeight + 3*TreeTableRowHeight + 1})
	if !d.Expanded {
		t.Fatal("chevron click should have expanded d")
	}
	tt.flatten()
	if len(tt.rows) != before+1 {
		t.Fatalf("after expanding d: rows = %d, want %d (d1 now visible)", len(tt.rows), before+1)
	}

	tt.OnEvent(Event{Kind: EventClick, X: 4, Y: TreeTableHeaderHeight + 3*TreeTableRowHeight + 1})
	if d.Expanded {
		t.Fatal("second click should have collapsed d")
	}
	tt.flatten()
	if len(tt.rows) != before {
		t.Fatalf("after collapsing d: rows = %d, want %d", len(tt.rows), before)
	}
}

// --- selection ------------------------------------------------------------

func TestTreeTableLeafClickWithinChevronRangeSelects(t *testing.T) {
	// a is a leaf (no children): a click inside what would be the chevron
	// hit-box must fall through to selection, not attempt a toggle.
	cols, root, a, _, _, _, _, _, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150})

	tt.OnEvent(Event{Kind: EventClick, X: 4, Y: TreeTableHeaderHeight + 1}) // row 0 = a
	if tt.Selected().Get() != a {
		t.Fatalf("Selected = %+v, want a", tt.Selected().Get())
	}
}

func TestTreeTableSelectionOnOtherColumn(t *testing.T) {
	cols, root, _, _, _, _, _, _, c := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150})

	// c is row index 4: local y = header + 4*rowHeight. Click well past
	// the first column, in column 2's territory.
	tt.OnEvent(Event{Kind: EventClick, X: 150, Y: TreeTableHeaderHeight + 4*TreeTableRowHeight + 1})
	if tt.Selected().Get() != c {
		t.Fatalf("Selected = %+v, want c", tt.Selected().Get())
	}
}

func TestTreeTableSelectedRowHighlightedInAccent(t *testing.T) {
	cols, root, _, _, _, _, _, _, c := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.Selected().Set(c)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150})
	theme := DefaultLight()
	buf := makeSurface(200, 150)
	tt.Draw(newP(buf, 200), theme)

	rowY := TreeTableHeaderHeight + 4*TreeTableRowHeight
	if pixelAt(buf, 200, 0, rowY+1) != theme.Accent {
		t.Fatal("selected row background should be Accent")
	}
	// An unselected row (a, row 0) must not be painted Accent.
	if pixelAt(buf, 200, 0, TreeTableHeaderHeight+1) == theme.Accent {
		t.Fatal("unselected row should not be Accent")
	}
}

// --- empty tree / empty columns guards ------------------------------------

func TestTreeTableEmptyRootNoPanic(t *testing.T) {
	cols, _, _, _, _, _, _, _, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, nil)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150})
	tt.Draw(newP(makeSurface(200, 150), 200), DefaultLight())
	tt.OnEvent(Event{Kind: EventClick, X: 50, Y: TreeTableHeaderHeight + 5})
	if tt.Selected().Get() != nil {
		t.Fatal("click on an empty tree should not select anything")
	}
}

func TestTreeTableEmptyColumnsNoPanic(t *testing.T) {
	_, root, _, _, _, _, d, _, _ := newTreeTableFixture()
	tt := NewTreeTable(nil, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150})
	tt.Draw(newP(makeSurface(200, 150), 200), DefaultLight())

	// OnEvent's chevron hit-test doesn't depend on Columns at all: d
	// (row index 3) should still toggle.
	tt.OnEvent(Event{Kind: EventClick, X: 4, Y: TreeTableHeaderHeight + 3*TreeTableRowHeight + 1})
	if !d.Expanded {
		t.Fatal("chevron toggle should work even with zero Columns")
	}
}

func TestTreeTableCellTextShortRow(t *testing.T) {
	cols := []TreeTableColumn{{Title: "A"}, {Title: "B"}, {Title: "C"}}
	short := &TreeTableNode{Cells: []string{"only-one"}}
	tt := NewTreeTable(cols, []*TreeTableNode{short})
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 60})
	// Must not panic despite the row carrying fewer cells than columns.
	tt.Draw(newP(makeSurface(200, 60), 200), DefaultLight())
}

func TestTreeTableDrawZeroBoundsNoOp(t *testing.T) {
	cols, root, _, _, _, _, _, _, _ := newTreeTableFixture()
	for _, b := range []Rect{{W: 0, H: 100}, {W: 100, H: 0}} {
		tt := NewTreeTable(cols, root)
		tt.SetBounds(b)
		surf := makeSurface(100, 100)
		before := make([]byte, len(surf))
		copy(before, surf)
		tt.Draw(newP(surf, 100), DefaultLight())
		if !bytes.Equal(surf, before) {
			t.Fatalf("Draw with degenerate bounds %+v touched the buffer", b)
		}
	}
}

// --- event plumbing --------------------------------------------------------

func TestTreeTableOnEventBeforeSetBoundsSkipsVirtualization(t *testing.T) {
	// Bounds().H defaults to 0, so bodyVisibleRows() short-circuits to 0
	// (h<=0) and windowed is forced false — OnEvent must still resolve a
	// click against the unwindowed, fully-flattened row list.
	cols, root, a, _, _, _, _, _, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.OnEvent(Event{Kind: EventClick, X: 4, Y: TreeTableHeaderHeight + 1})
	if tt.Selected().Get() != a {
		t.Fatalf("Selected = %+v, want a despite unset Bounds", tt.Selected().Get())
	}
}

func TestTreeTableIgnoresNonClick(t *testing.T) {
	cols, root, _, _, _, _, _, _, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150})
	tt.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if tt.Selected().Get() != nil {
		t.Fatal("KeyDown should not select")
	}
}

func TestTreeTableClickOnHeaderRowIgnored(t *testing.T) {
	cols, root, _, _, _, _, _, _, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150})
	tt.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if tt.Selected().Get() != nil {
		t.Fatal("a click on the header row must not select a body row")
	}
}

func TestTreeTableClickPastLastRowIgnored(t *testing.T) {
	cols, root, _, _, _, _, _, _, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150}) // unwindowed, total=5
	tt.OnEvent(Event{Kind: EventClick, X: 50, Y: TreeTableHeaderHeight + 10*TreeTableRowHeight})
	if tt.Selected().Get() != nil {
		t.Fatal("click past the last row should not select anything")
	}
}

// --- windowed rendering + scrolling ---------------------------------------

func TestTreeTableWindowedRenderingOffWindowNodeNotPainted(t *testing.T) {
	nodes := manyTreeTableLeaves(20)
	tt := NewTreeTable([]TreeTableColumn{{Title: "Name"}}, nodes)
	tt.Selected().Set(nodes[15])
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // wr=3

	theme := DefaultLight()
	contentW := 200 - scrollbarWidth
	hasAccent := func(buf []byte) bool {
		for y := 0; y < 90; y++ {
			for x := 0; x < contentW; x++ {
				if pixelAt(buf, 200, x, y) == theme.Accent {
					return true
				}
			}
		}
		return false
	}

	buf1 := makeSurface(200, 90)
	tt.Draw(newP(buf1, 200), theme)
	if hasAccent(buf1) {
		t.Fatal("selected node outside the scroll window was painted")
	}

	tt.ScrollTo(15)
	if tt.ScrollRow().Get() != 15 {
		t.Fatalf("ScrollRow = %d, want 15", tt.ScrollRow().Get())
	}
	buf2 := makeSurface(200, 90)
	tt.Draw(newP(buf2, 200), theme)
	if !hasAccent(buf2) {
		t.Fatal("selected node inside the scroll window was not painted")
	}
}

func TestTreeTableClickBelowWindowIgnored(t *testing.T) {
	nodes := manyTreeTableLeaves(20)
	tt := NewTreeTable([]TreeTableColumn{{Title: "Name"}}, nodes)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100}) // wr=3, dead zone below
	tt.ScrollTo(2)

	tt.OnEvent(Event{Kind: EventClick, X: 80, Y: TreeTableHeaderHeight + 3*TreeTableRowHeight})
	if tt.Selected().Get() != nil {
		t.Fatal("click below the painted window should not select anything")
	}
}

func TestTreeTableClickWithNonZeroScrollRowSelectsCorrectNode(t *testing.T) {
	nodes := manyTreeTableLeaves(20)
	tt := NewTreeTable([]TreeTableColumn{{Title: "Name"}}, nodes)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // wr=3

	tt.ScrollTo(8) // window [8,11)
	tt.OnEvent(Event{Kind: EventClick, X: 80, Y: TreeTableHeaderHeight + 2*TreeTableRowHeight})
	if tt.Selected().Get() != nodes[10] {
		t.Fatalf("Selected = %v, want nodes[10]", tt.Selected().Get())
	}
}

func TestTreeTableScrollClickCollapseReClampsScrollRow(t *testing.T) {
	bLeaves := make([]*TreeTableNode, 20)
	for i := range bLeaves {
		bLeaves[i] = &TreeTableNode{Cells: []string{fmt.Sprintf("b%d", i)}}
	}
	A := &TreeTableNode{Cells: []string{"A"}}
	B := &TreeTableNode{Cells: []string{"B"}, Expanded: true, Children: bLeaves}
	C := &TreeTableNode{Cells: []string{"C"}}
	tt := NewTreeTable([]TreeTableColumn{{Title: "Name"}}, []*TreeTableNode{A, B, C})
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // wr=3, total=23

	tt.ScrollTo(1) // window [1,4): B,b0,b1 — B at local row 0
	if tt.ScrollRow().Get() != 1 {
		t.Fatalf("ScrollRow = %d, want 1", tt.ScrollRow().Get())
	}

	tt.OnEvent(Event{Kind: EventClick, X: 4, Y: TreeTableHeaderHeight})
	if B.Expanded {
		t.Fatal("chevron click should have collapsed B")
	}
	// Post-collapse total = A,B,C = 3 <= window(3): re-clamp to 0.
	if tt.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollRow after click-collapse = %d, want re-clamped to 0", tt.ScrollRow().Get())
	}
}

func TestTreeTableScrollToAndScrollByClamp(t *testing.T) {
	nodes := manyTreeTableLeaves(20)
	tt := NewTreeTable([]TreeTableColumn{{Title: "Name"}}, nodes)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // wr=3, total=20, max=17

	tt.ScrollTo(-5)
	if tt.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollTo(-5) = %d, want 0", tt.ScrollRow().Get())
	}
	tt.ScrollTo(999)
	if tt.ScrollRow().Get() != 17 {
		t.Fatalf("ScrollTo(999) = %d, want 17", tt.ScrollRow().Get())
	}
	tt.ScrollTo(10)
	tt.ScrollBy(3)
	if tt.ScrollRow().Get() != 13 {
		t.Fatalf("ScrollBy(3) from 10 = %d, want 13", tt.ScrollRow().Get())
	}
	tt.ScrollBy(-100)
	if tt.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollBy(-100) = %d, want 0", tt.ScrollRow().Get())
	}
}

// --- scrollToSelected -------------------------------------------------

func TestTreeTableScrollToSelectedNilIsNoop(t *testing.T) {
	cols, root, _, _, _, _, _, _, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 50}) // wr=1
	tt.ScrollRow().Set(3)

	tt.scrollToSelected()
	if tt.ScrollRow().Get() != 3 {
		t.Fatalf("scrollToSelected with nil Selected changed ScrollRow to %d, want unchanged 3", tt.ScrollRow().Get())
	}
}

func TestTreeTableScrollToSelectedNotVisibleIsNoop(t *testing.T) {
	// d1 is hidden: its parent d is collapsed.
	cols, root, _, _, _, _, _, d1, _ := newTreeTableFixture()
	tt := NewTreeTable(cols, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150})
	tt.Selected().Set(d1)
	tt.ScrollRow().Set(0)

	tt.scrollToSelected()
	if tt.ScrollRow().Get() != 0 {
		t.Fatalf("scrollToSelected with a hidden Selected changed ScrollRow to %d, want unchanged 0", tt.ScrollRow().Get())
	}
}

func TestTreeTableScrollToSelectedAdjustsMinimally(t *testing.T) {
	nodes := manyTreeTableLeaves(20)
	tt := NewTreeTable([]TreeTableColumn{{Title: "Name"}}, nodes)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // wr=3, total=20

	tt.Selected().Set(nodes[15])
	tt.scrollToSelected()
	if want := 15 - 3 + 1; tt.ScrollRow().Get() != want {
		t.Fatalf("ScrollRow = %d, want %d", tt.ScrollRow().Get(), want)
	}

	tt.Selected().Set(nodes[0])
	tt.scrollToSelected()
	if tt.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollRow = %d, want 0", tt.ScrollRow().Get())
	}
}

// --- scrollbar --------------------------------------------------------

func TestTreeTableScrollbarPaintedOnOverflowAndThumbMoves(t *testing.T) {
	nodes := manyTreeTableLeaves(20)
	tt := NewTreeTable([]TreeTableColumn{{Title: "Name"}}, nodes)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // wr=3, total=20, max=17

	theme := DefaultLight()
	trackX := 200 - scrollbarWidth

	scan := func(scrollRow int) (foundAlt, foundAccent bool) {
		tt.ScrollTo(scrollRow)
		buf := makeSurface(200, 90)
		tt.Draw(newP(buf, 200), theme)
		for y := 0; y < 90; y++ {
			for x := trackX; x < 200; x++ {
				switch pixelAt(buf, 200, x, y) {
				case theme.SurfaceAlt:
					foundAlt = true
				case scrollbarThumbColor(theme):
					foundAccent = true
				}
			}
		}
		return
	}

	alt0, accent0 := scan(0)
	if !alt0 {
		t.Fatal("scrollbar track (SurfaceAlt) not painted")
	}
	if !accent0 {
		t.Fatal("scrollbar thumb (Accent) not painted")
	}

	tt.ScrollTo(17) // max
	buf := makeSurface(200, 90)
	tt.Draw(newP(buf, 200), theme)
	// Sample the track's centre column: the rounded thumb's cap tapers at the
	// track edge, so the leftmost column can be bare even where the thumb sits.
	if pixelAt(buf, 200, trackX+scrollbarWidth/2, 87) != scrollbarThumbColor(theme) {
		t.Fatal("thumb did not move to the bottom of the track at max ScrollRow")
	}
}

func TestTreeTableScrollbarThumbMinimumSize(t *testing.T) {
	nodes := manyTreeTableLeaves(500)
	tt := NewTreeTable([]TreeTableColumn{{Title: "Name"}}, nodes)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90})

	theme := DefaultLight()
	buf := makeSurface(200, 90)
	tt.Draw(newP(buf, 200), theme)

	trackX := 200 - scrollbarWidth
	thumbRows := 0
	for y := 0; y < 90; y++ {
		// Centre column: the rounded thumb is full-length there, so the count
		// reflects the true (floor-clamped) thumb height.
		if pixelAt(buf, 200, trackX+scrollbarWidth/2, y) == scrollbarThumbColor(theme) {
			thumbRows++
		}
	}
	if thumbRows < 8 {
		t.Fatalf("thumb height = %d px, want >= 8 (clamped minimum)", thumbRows)
	}
}

func TestTreeTableNoScrollbarWhenTreeFits(t *testing.T) {
	cols, root, _, _, _, _, _, _, _ := newTreeTableFixture() // 5 rows
	tt := NewTreeTable(cols, root)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150}) // wr=5: fits (not >)

	theme := DefaultLight()
	buf := makeSurface(200, 150)
	tt.Draw(newP(buf, 200), theme)

	// Scan the BODY only (y >= TreeTableHeaderHeight): the header itself
	// legitimately paints SurfaceAlt edge-to-edge (including under where
	// a scrollbar track would sit), so scanning from y=0 would mistake
	// the header for a scrollbar track.
	trackX := 200 - scrollbarWidth
	for y := TreeTableHeaderHeight; y < 150; y++ {
		for x := trackX; x < 200; x++ {
			if got := pixelAt(buf, 200, x, y); got == theme.SurfaceAlt {
				t.Fatalf("scrollbar track painted at (%d,%d) though the tree fits", x, y)
			}
		}
	}
}

// --- columnWidths -------------------------------------------------------

func TestTreeTableColumnWidthsNoColumns(t *testing.T) {
	tt := NewTreeTable(nil, nil)
	if got := tt.columnWidths(200); got != nil {
		t.Fatalf("columnWidths with no columns = %v, want nil", got)
	}
}

func TestTreeTableColumnWidthsAllFixed(t *testing.T) {
	tt := NewTreeTable([]TreeTableColumn{{Title: "a", Width: 50}, {Title: "b", Width: 70}}, nil)
	got := tt.columnWidths(200)
	if got[0] != 50 || got[1] != 70 {
		t.Fatalf("all-fixed widths = %v, want [50 70]", got)
	}
}

func TestTreeTableColumnWidthsAllAutoLeftoverGoesToLast(t *testing.T) {
	tt := NewTreeTable([]TreeTableColumn{{Title: "a"}, {Title: "b"}, {Title: "c"}}, nil)
	got := tt.columnWidths(200)
	if got[0] != 66 || got[1] != 66 || got[2] != 68 {
		t.Fatalf("all-auto+leftover widths = %v, want [66 66 68]", got)
	}
	if got[0]+got[1]+got[2] != 200 {
		t.Fatalf("all-auto+leftover sum = %d, want 200", got[0]+got[1]+got[2])
	}
}

func TestTreeTableColumnWidthsMixSumsToTotal(t *testing.T) {
	tt := NewTreeTable([]TreeTableColumn{
		{Title: "a", Width: 30},
		{Title: "b"}, // auto
		{Title: "c", Width: 20},
		{Title: "d"}, // auto
	}, nil)
	got := tt.columnWidths(200)
	sum := 0
	for _, w := range got {
		sum += w
	}
	if sum != 200 {
		t.Fatalf("mix widths %v sum = %d, want 200", got, sum)
	}
	if got[0] != 30 || got[2] != 20 {
		t.Fatalf("fixed columns unchanged: %v", got)
	}
}

func TestTreeTableColumnWidthsFixedOverflowsTotal(t *testing.T) {
	tt := NewTreeTable([]TreeTableColumn{
		{Title: "a", Width: 300},
		{Title: "b"}, // auto -- gets clamped to 0
	}, nil)
	got := tt.columnWidths(200)
	if got[0] != 300 {
		t.Fatalf("fixed overflow: col0 width = %d, want 300", got[0])
	}
	if got[1] != 0 {
		t.Fatalf("fixed overflow: auto col width = %d, want 0", got[1])
	}
}
