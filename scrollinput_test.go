// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"testing"
)

// Native scroll input: EventScroll (wheel) + Arrow/Page/Home/End keys drive
// the scroll offset of every row-scrolling widget (ListBox, Table, TreeTable,
// TreeView) and the pixel-scrolling ScrollView, and containers forward an
// EventScroll to whichever child sits under the pointer. Every branch added
// for the feature is exercised here.

// --- ListBox ---------------------------------------------------------------

func newScrollListBox() *ListBox {
	items := make([]string, 20)
	for i := range items {
		items[i] = fmt.Sprintf("row %d", i)
	}
	lb := NewListBox(items)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 54}) // 3 rows visible of 20
	return lb
}

func TestListBoxWheelScroll(t *testing.T) {
	lb := newScrollListBox()
	max := lb.maxScrollRow()
	if max < 3 {
		t.Fatalf("test needs a scrollable list, maxScrollRow=%d", max)
	}

	lb.OnEvent(Event{Kind: EventScroll, Delta: 2})
	if lb.ScrollRow != 2 {
		t.Fatalf("wheel down 2: ScrollRow=%d, want 2", lb.ScrollRow)
	}
	// Clamp at the bottom: an over-large delta pins to maxScrollRow.
	lb.OnEvent(Event{Kind: EventScroll, Delta: 1000})
	if lb.ScrollRow != max {
		t.Fatalf("wheel to end: ScrollRow=%d, want %d", lb.ScrollRow, max)
	}
	// Clamp at the top: can't go negative.
	lb.OnEvent(Event{Kind: EventScroll, Delta: -1000})
	if lb.ScrollRow != 0 {
		t.Fatalf("wheel to top: ScrollRow=%d, want 0", lb.ScrollRow)
	}
}

// Arrow/Page/Home/End keys now drive a roving SELECTION cursor (not the scroll
// offset) — that behaviour and its auto-scroll-to-keep-visible are covered in
// wave3b_data_keyboard_test.go. The tests below keep only the wheel
// (EventScroll) path, which still just scrolls.

// --- Table -----------------------------------------------------------------

func newScrollTable() *Table {
	rows := make([][]string, 20)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("r%d", i)}
	}
	tb := NewTable([]TableColumn{{Title: "A"}}, rows)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: TableHeaderHeight + 3*TableRowHeight})
	return tb
}

func TestTableWheelAndKeyScroll(t *testing.T) {
	tb := newScrollTable()
	max := tb.maxScrollRow()
	if max < 3 {
		t.Fatalf("test needs a scrollable table, max=%d", max)
	}

	tb.OnEvent(Event{Kind: EventScroll, Delta: 2})
	if tb.ScrollRow != 2 {
		t.Fatalf("wheel: %d, want 2", tb.ScrollRow)
	}
	tb.OnEvent(Event{Kind: EventScroll, Delta: 1000})
	if tb.ScrollRow != max {
		t.Fatalf("wheel clamp bottom: %d, want %d", tb.ScrollRow, max)
	}
	tb.OnEvent(Event{Kind: EventScroll, Delta: -1000})
	if tb.ScrollRow != 0 {
		t.Fatalf("wheel clamp top: %d, want 0", tb.ScrollRow)
	}
}

// --- TreeTable -------------------------------------------------------------

func newScrollTreeTable() *TreeTable {
	tt := NewTreeTable([]TreeTableColumn{{Title: "A"}}, manyTreeTableLeaves(20))
	tt.SetBounds(Rect{X: 0, Y: 0, W: 120, H: TreeTableHeaderHeight + 3*TreeTableRowHeight})
	return tt
}

func TestTreeTableWheelAndKeyScroll(t *testing.T) {
	tt := newScrollTreeTable()
	tt.flatten()
	page := tt.bodyVisibleRows()
	max := len(tt.rows) - page
	if max < 3 {
		t.Fatalf("test needs a scrollable tree table, max=%d", max)
	}

	tt.OnEvent(Event{Kind: EventScroll, Delta: 2})
	if tt.ScrollRow().Get() != 2 {
		t.Fatalf("wheel: %d, want 2", tt.ScrollRow().Get())
	}
	tt.OnEvent(Event{Kind: EventScroll, Delta: 1000})
	if tt.ScrollRow().Get() != max {
		t.Fatalf("wheel clamp bottom: %d, want %d", tt.ScrollRow().Get(), max)
	}
	tt.OnEvent(Event{Kind: EventScroll, Delta: -1000})
	if tt.ScrollRow().Get() != 0 {
		t.Fatalf("wheel clamp top: %d, want 0", tt.ScrollRow().Get())
	}
	// A non-handled event kind is a no-op (covers the switch default branch).
	tt.OnEvent(Event{Kind: EventMouseUp})
	if tt.ScrollRow().Get() != 0 {
		t.Fatalf("ignored event changed ScrollRow: %d, want 0", tt.ScrollRow().Get())
	}
}

// --- TreeView --------------------------------------------------------------

func newScrollTreeView() *TreeView {
	root, _ := manyLeaves(20) // expanded root + 20 leaves = 21 flattened rows
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 3 * 18})
	return tv
}

func TestTreeViewWheelAndKeyScroll(t *testing.T) {
	tv := newScrollTreeView()
	tv.flatten()
	page := tv.windowRows()
	max := len(tv.rows) - page
	if max < 3 {
		t.Fatalf("test needs a scrollable tree view, max=%d", max)
	}

	tv.OnEvent(Event{Kind: EventScroll, Delta: 2})
	if tv.ScrollRow != 2 {
		t.Fatalf("wheel: %d, want 2", tv.ScrollRow)
	}
	tv.OnEvent(Event{Kind: EventScroll, Delta: 1000})
	if tv.ScrollRow != max {
		t.Fatalf("wheel clamp bottom: %d, want %d", tv.ScrollRow, max)
	}
	tv.OnEvent(Event{Kind: EventScroll, Delta: -1000})
	if tv.ScrollRow != 0 {
		t.Fatalf("wheel clamp top: %d, want 0", tv.ScrollRow)
	}
	// A non-handled event kind is a no-op (covers the switch default branch).
	tv.OnEvent(Event{Kind: EventMouseUp})
	if tv.ScrollRow != 0 {
		t.Fatalf("ignored event changed ScrollRow: %d, want 0", tv.ScrollRow)
	}
}

// --- ScrollView (pixel scroll) ---------------------------------------------

func newScrollView() *ScrollView {
	child := NewLabel("x")
	child.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 400})
	sv := NewScrollView(child)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	sv.SetContentSize(40, 400)
	return sv
}

// A horizontal two-finger swipe (EventScroll with DeltaX, from the browser
// wheel's deltaX) moves the ScrollView's OffsetX — i.e. drives the horizontal
// scrollbar — instead of being dropped; a pure vertical notch leaves OffsetX
// untouched, and a diagonal swipe moves both axes at once. Each axis clamps
// independently.
func TestScrollViewHorizontalWheel(t *testing.T) {
	child := NewLabel("x")
	child.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	sv := NewScrollView(child)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	sv.SetContentSize(400, 400) // overflows both axes
	line := GlyphHeight()

	// Horizontal swipe moves OffsetX and leaves OffsetY alone.
	sv.OnEvent(Event{Kind: EventScroll, DeltaX: 2})
	if sv.OffsetX != 2*line {
		t.Fatalf("swipe right 2: OffsetX=%d, want %d", sv.OffsetX, 2*line)
	}
	if sv.OffsetY != 0 {
		t.Fatalf("horizontal swipe moved OffsetY=%d, want 0", sv.OffsetY)
	}
	// Clamp at the right edge.
	sv.OnEvent(Event{Kind: EventScroll, DeltaX: 1000})
	if sv.OffsetX == 0 || sv.OffsetX != sv.maxOffsetX() {
		t.Fatalf("swipe to right edge: OffsetX=%d, want %d", sv.OffsetX, sv.maxOffsetX())
	}
	// Clamp at the left edge.
	sv.OnEvent(Event{Kind: EventScroll, DeltaX: -1000})
	if sv.OffsetX != 0 {
		t.Fatalf("swipe to left edge: OffsetX=%d, want 0", sv.OffsetX)
	}
	// A pure vertical notch does not move OffsetX.
	sv.OnEvent(Event{Kind: EventScroll, Delta: 1})
	if sv.OffsetX != 0 {
		t.Fatalf("vertical notch moved OffsetX=%d, want 0", sv.OffsetX)
	}
	// A diagonal swipe moves both axes together.
	sv.OnEvent(Event{Kind: EventScroll, DeltaX: 1, Delta: 1})
	if sv.OffsetX != line || sv.OffsetY != 2*line {
		t.Fatalf("diagonal: OffsetX=%d OffsetY=%d, want %d,%d", sv.OffsetX, sv.OffsetY, line, 2*line)
	}
}

func TestScrollViewWheelAndKeyScroll(t *testing.T) {
	sv := newScrollView()
	line := GlyphHeight()

	sv.OnEvent(Event{Kind: EventScroll, Delta: 2})
	if sv.OffsetY != 2*line {
		t.Fatalf("wheel down 2 lines: OffsetY=%d, want %d", sv.OffsetY, 2*line)
	}
	// Clamp at the bottom (content 400, viewport ~60 => maxY = 400-vp.H).
	sv.OnEvent(Event{Kind: EventScroll, Delta: 1000})
	if sv.OffsetY == 0 || sv.OffsetY != sv.contentH-sv.viewport().H {
		t.Fatalf("wheel to bottom: OffsetY=%d, want %d", sv.OffsetY, sv.contentH-sv.viewport().H)
	}
	sv.OnEvent(Event{Kind: EventScroll, Delta: -1000})
	if sv.OffsetY != 0 {
		t.Fatalf("wheel to top: OffsetY=%d, want 0", sv.OffsetY)
	}

	sv.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if sv.OffsetY != line {
		t.Fatalf("ArrowDown: %d, want %d", sv.OffsetY, line)
	}
	sv.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if sv.OffsetY != 0 {
		t.Fatalf("ArrowUp: %d, want 0", sv.OffsetY)
	}
	sv.OnEvent(Event{Kind: EventKeyDown, Code: "PageDown"})
	if sv.OffsetY != sv.viewport().H {
		t.Fatalf("PageDown: %d, want %d", sv.OffsetY, sv.viewport().H)
	}
	sv.OnEvent(Event{Kind: EventKeyDown, Code: "PageUp"})
	if sv.OffsetY != 0 {
		t.Fatalf("PageUp: %d, want 0", sv.OffsetY)
	}
	sv.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if sv.OffsetY != sv.contentH-sv.viewport().H {
		t.Fatalf("End: %d, want %d", sv.OffsetY, sv.contentH-sv.viewport().H)
	}
	sv.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if sv.OffsetY != 0 {
		t.Fatalf("Home: %d, want 0", sv.OffsetY)
	}
	// Non-scroll key + unhandled kind are ignored.
	sv.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	sv.OnEvent(Event{Kind: EventClick})
	if sv.OffsetY != 0 {
		t.Fatalf("ignored events changed OffsetY: %d", sv.OffsetY)
	}
}

// --- Container forwarding --------------------------------------------------

func TestContainerForwardsScrollToChildUnderPointer(t *testing.T) {
	lb := newScrollListBox()
	c := NewContainer(FitLayout{})
	c.AddWidget(lb)
	c.SetBounds(Rect{X: 10, Y: 20, W: 120, H: 54}) // FitLayout stretches lb to fill

	// A wheel event at a point inside the container (container-local coords)
	// must reach the ListBox and scroll it.
	c.OnEvent(Event{Kind: EventScroll, X: 30, Y: 30, Delta: 3})
	if lb.ScrollRow != 3 {
		t.Fatalf("container did not forward EventScroll: ListBox ScrollRow=%d, want 3", lb.ScrollRow)
	}
}

func TestVBoxForwardsScrollToChildUnderPointer(t *testing.T) {
	top := newScrollListBox()
	bottom := newScrollTreeTable()
	v := NewVBox()
	v.AddFlex(top, 1)
	v.AddFlex(bottom, 1)
	v.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 220}) // ~110px each child

	// Point in the lower half hits the TreeTable, not the ListBox.
	v.OnEvent(Event{Kind: EventScroll, X: 20, Y: 170, Delta: 2})
	if bottom.ScrollRow().Get() != 2 {
		t.Fatalf("VBox forwarded to wrong child: TreeTable ScrollRow=%d, want 2", bottom.ScrollRow().Get())
	}
	if top.ScrollRow != 0 {
		t.Fatalf("ListBox should not have scrolled: %d", top.ScrollRow)
	}
}
