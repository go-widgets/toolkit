// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"testing"
)

// --- sbGeom: pure geometry helpers ----------------------------------------

func TestSBGeomAxisAndHitTest(t *testing.T) {
	// A vertical bar: cross axis is X, along axis is Y.
	v := sbGeom{cross0: 92, trackStart: 0, trackLen: 60, thumbStart: 10, thumbLen: 20}
	if v.along(3, 7) != 7 || v.cross(3, 7) != 3 {
		t.Fatalf("vertical along/cross = %d/%d, want 7/3", v.along(3, 7), v.cross(3, 7))
	}
	if !v.onTrack(95, 5) {
		t.Fatal("(95,5) should be on the vertical track")
	}
	if v.onTrack(50, 5) {
		t.Fatal("(50,5) is left of the track, not on it")
	}
	if !v.onThumb(95, 15) {
		t.Fatal("(95,15) should be on the thumb")
	}
	if v.onThumb(95, 55) {
		t.Fatal("(95,55) is below the thumb, not on it")
	}

	// A horizontal bar: cross axis is Y, along axis is X.
	h := sbGeom{horizontal: true, cross0: 32, trackStart: 0, trackLen: 32, thumbStart: 4, thumbLen: 8}
	if h.along(11, 34) != 11 || h.cross(11, 34) != 34 {
		t.Fatalf("horizontal along/cross = %d/%d, want 11/34", h.along(11, 34), h.cross(11, 34))
	}
	if !h.onTrack(5, 34) || !h.onThumb(5, 34) {
		t.Fatal("(5,34) should be on the horizontal track + thumb")
	}
	if h.onTrack(5, 10) {
		t.Fatal("(5,10) is above the horizontal track, not on it")
	}
}

func TestSBGeomScrollForGrabStart(t *testing.T) {
	// thumbStart = trackStart + scroll*travelNum/travelDen, so with
	// travelNum=50, travelDen=16 the inverse maps a grabbed pixel back to a row.
	g := sbGeom{trackStart: 0, travelNum: 50, travelDen: 16, maxScroll: 16}
	if got := g.scrollForGrabStart(0); got != 0 {
		t.Fatalf("grab at the top = %d, want 0", got)
	}
	if got := g.scrollForGrabStart(25); got != 8 { // (25*16+25)/50 = 8.5 -> 8
		t.Fatalf("grab at mid = %d, want 8", got)
	}
	// Overshoot below the track clamps to maxScroll.
	if got := g.scrollForGrabStart(9999); got != 16 {
		t.Fatalf("overshoot = %d, want 16 (clamped)", got)
	}
	// A grab above the track (negative rel) clamps to 0.
	g2 := sbGeom{trackStart: 24, travelNum: 50, travelDen: 16, maxScroll: 16}
	if got := g2.scrollForGrabStart(0); got != 0 {
		t.Fatalf("grab above the track = %d, want 0 (clamped)", got)
	}
	// A thumb that fills (or exceeds) the track has nowhere to travel.
	full := sbGeom{travelNum: 0, travelDen: 16, maxScroll: 16}
	if got := full.scrollForGrabStart(40); got != 0 {
		t.Fatalf("zero-travel scroll = %d, want 0", got)
	}
}

// --- TreeTable: draggable scrollbar ---------------------------------------

func TestTreeTableScrollbarDragPagesAndSelects(t *testing.T) {
	tt := NewTreeTable([]TreeTableColumn{{Title: "Name"}}, manyTreeTableLeaves(20))
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // wr=3, total=20, max=17
	trackX := 200 - scrollbarWidth                // 188
	// Geometry: trackTop=24, trackH=66, thumbH=9, thumb at [24,33) when ScrollRow=0.

	// Press ON the thumb, then drag down the track: ScrollRow rises
	// proportionally and clamps at the end.
	before := tt.ScrollRow
	tt.OnEvent(Event{Kind: EventClick, X: trackX + 3, Y: 26})
	if !tt.sbDrag.active {
		t.Fatal("pressing the thumb should begin a drag")
	}
	tt.OnEvent(Event{Kind: EventMouseDrag, X: trackX + 3, Y: 50})
	mid := tt.ScrollRow
	if !(mid > before && mid < 17) {
		t.Fatalf("mid drag ScrollRow = %d, want strictly between %d and 17", mid, before)
	}
	if mid != 7 { // (48-24)*17/57 rounded = 7
		t.Fatalf("mid drag ScrollRow = %d, want 7", mid)
	}
	tt.OnEvent(Event{Kind: EventMouseDrag, X: trackX + 3, Y: 90})
	if tt.ScrollRow != 17 {
		t.Fatalf("drag past the end ScrollRow = %d, want 17 (clamped)", tt.ScrollRow)
	}
	tt.OnEvent(Event{Kind: EventMouseUp, X: trackX + 3, Y: 90})
	if tt.sbDrag.active {
		t.Fatal("EventMouseUp should end the drag")
	}

	// A press on the track BELOW the thumb pages down by one window.
	tt.ScrollTo(0)
	tt.OnEvent(Event{Kind: EventClick, X: trackX + 3, Y: 60})
	if tt.ScrollRow != 3 {
		t.Fatalf("page-down ScrollRow = %d, want 3 (one window)", tt.ScrollRow)
	}
	// A press on the track ABOVE the thumb pages up by one window.
	tt.ScrollTo(10)
	tt.OnEvent(Event{Kind: EventClick, X: trackX + 3, Y: 30})
	if tt.ScrollRow != 7 {
		t.Fatalf("page-up ScrollRow = %d, want 7", tt.ScrollRow)
	}

	// A press LEFT of the scrollbar still selects a row and never scrolls.
	tt.ScrollTo(5)
	tt.Selected = nil
	tt.OnEvent(Event{Kind: EventClick, X: 100, Y: 30})
	if tt.Selected == nil {
		t.Fatal("a press left of the scrollbar must select a row")
	}
	if tt.ScrollRow != 5 {
		t.Fatalf("selecting a row must not scroll: ScrollRow = %d, want 5", tt.ScrollRow)
	}
	if tt.sbDrag.active {
		t.Fatal("a content press must not begin a scrollbar drag")
	}
}

// --- ListBox: draggable scrollbar -----------------------------------------

func manyItems(n int) []string {
	items := make([]string, n)
	for i := range items {
		items[i] = fmt.Sprintf("item %d", i)
	}
	return items
}

func TestListBoxScrollbarDragPagesAndSelects(t *testing.T) {
	l := NewListBox(manyItems(20)) // RowHeight=18
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	// Geometry: visibleRows=4, contentH=360, max=16, thumbH=10, thumb [0,10).
	trackX := 100 - scrollbarWidth // 88

	before := l.ScrollRow().Get()
	l.OnEvent(Event{Kind: EventClick, X: trackX + 3, Y: 3})
	if !l.sbDrag.active {
		t.Fatal("pressing the thumb should begin a drag")
	}
	l.OnEvent(Event{Kind: EventMouseDrag, X: trackX + 3, Y: 30})
	if l.ScrollRow().Get() != 9 { // (27*16+25)/50 = 9.14 -> 9
		t.Fatalf("mid drag ScrollRow = %d, want 9", l.ScrollRow().Get())
	}
	if !(l.ScrollRow().Get() > before && l.ScrollRow().Get() < 16) {
		t.Fatalf("mid drag ScrollRow = %d, want strictly between %d and 16", l.ScrollRow().Get(), before)
	}
	l.OnEvent(Event{Kind: EventMouseDrag, X: trackX + 3, Y: 60})
	if l.ScrollRow().Get() != 16 {
		t.Fatalf("drag past the end ScrollRow = %d, want 16 (clamped)", l.ScrollRow().Get())
	}
	l.OnEvent(Event{Kind: EventMouseUp})
	if l.sbDrag.active {
		t.Fatal("EventMouseUp should end the drag")
	}

	// Track press below the thumb pages down.
	l.ScrollTo(0)
	l.OnEvent(Event{Kind: EventClick, X: trackX + 3, Y: 40})
	if l.ScrollRow().Get() != 4 {
		t.Fatalf("page-down ScrollRow = %d, want 4", l.ScrollRow().Get())
	}

	// A press left of the scrollbar still selects a row.
	l.ScrollTo(0)
	l.Selected().Set(-1)
	l.OnEvent(Event{Kind: EventClick, X: 50, Y: 20})
	if l.Selected().Get() != 1 {
		t.Fatalf("content press should select row 1, got %d", l.Selected().Get())
	}
	if l.ScrollRow().Get() != 0 {
		t.Fatalf("selecting must not scroll: ScrollRow = %d, want 0", l.ScrollRow().Get())
	}
}

func TestListBoxScrollbarNoTravelWhenBarelyOverflowing(t *testing.T) {
	// items == visibleRows but a partial extra pixel makes contentH > bounds:
	// the scrollbar is live yet maxScrollRow is 0, so the thumb can't travel.
	l := NewListBox(manyItems(2))
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20}) // visibleRows=2, contentH=36>20, max=0
	g, ok := l.scrollbarGeom()
	if !ok || g.thumbStart != 0 || g.maxScroll != 0 {
		t.Fatalf("geom = %+v ok=%v, want live with thumbStart=0 maxScroll=0", g, ok)
	}
	trackX := 100 - scrollbarWidth
	l.OnEvent(Event{Kind: EventClick, X: trackX + 3, Y: 2}) // grab the immovable thumb
	l.OnEvent(Event{Kind: EventMouseDrag, X: trackX + 3, Y: 18})
	if l.ScrollRow().Get() != 0 {
		t.Fatalf("an immovable thumb must keep ScrollRow at 0, got %d", l.ScrollRow().Get())
	}
	l.OnEvent(Event{Kind: EventMouseUp})
}

// --- TreeView: draggable scrollbar ----------------------------------------

func TestTreeViewScrollbarDrag(t *testing.T) {
	root, _ := manyLeaves(20) // total = 21 rows (root + 20 leaves)
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 90}) // RowHeight=18, windowRows=5, max=16
	trackX := 120 - scrollbarWidth
	// thumbH = 90*5/21 = 21, thumb [0,21) at ScrollRow 0.

	tv.OnEvent(Event{Kind: EventClick, X: trackX + 3, Y: 5})
	if !tv.sbDrag.active {
		t.Fatal("pressing the thumb should begin a drag")
	}
	tv.OnEvent(Event{Kind: EventMouseDrag, X: trackX + 3, Y: 45})
	if tv.ScrollRow().Get() != 9 { // (40*16+34)/69 = 9.7 -> 9
		t.Fatalf("mid drag ScrollRow = %d, want 9", tv.ScrollRow().Get())
	}
	tv.OnEvent(Event{Kind: EventMouseDrag, X: trackX + 3, Y: 90})
	if tv.ScrollRow().Get() != 16 {
		t.Fatalf("drag past the end ScrollRow = %d, want 16 (clamped)", tv.ScrollRow().Get())
	}
	tv.OnEvent(Event{Kind: EventMouseUp})
	if tv.sbDrag.active {
		t.Fatal("EventMouseUp should end the drag")
	}

	// Track press below the thumb pages down; a content press selects.
	tv.ScrollTo(0)
	tv.OnEvent(Event{Kind: EventClick, X: trackX + 3, Y: 60})
	if tv.ScrollRow().Get() != 5 {
		t.Fatalf("page-down ScrollRow = %d, want 5", tv.ScrollRow().Get())
	}
	tv.ScrollTo(0)
	tv.Selected().Set(nil)
	tv.OnEvent(Event{Kind: EventClick, X: 20, Y: 5}) // root row
	if tv.Selected().Get() == nil {
		t.Fatal("a content press must select a node")
	}
}

func TestTreeScrollbarWidgetsIgnoreUnrelatedEvents(t *testing.T) {
	// An event kind that is neither scroll/key/click/drag/up is a no-op, and
	// must not begin a drag or move the scroll position.
	tt := NewTreeTable([]TreeTableColumn{{Title: "N"}}, manyTreeTableLeaves(20))
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90})
	tt.ScrollTo(4)
	tt.OnEvent(Event{Kind: EventKeyUp, X: 10, Y: 10})
	if tt.ScrollRow != 4 || tt.sbDrag.active {
		t.Fatalf("TreeTable must ignore EventKeyUp: ScrollRow=%d active=%v", tt.ScrollRow, tt.sbDrag.active)
	}

	root, _ := manyLeaves(20)
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 90})
	tv.ScrollTo(4)
	tv.OnEvent(Event{Kind: EventKeyUp, X: 10, Y: 10})
	if tv.ScrollRow().Get() != 4 || tv.sbDrag.active {
		t.Fatalf("TreeView must ignore EventKeyUp: ScrollRow=%d active=%v", tv.ScrollRow().Get(), tv.sbDrag.active)
	}
}

// --- Table: draggable scrollbar coexists with column resize ---------------

func manyTableRows(n int) [][]string {
	rows := make([][]string, n)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("r%d", i)}
	}
	return rows
}

func TestTableScrollbarDragAndSelect(t *testing.T) {
	tbl := NewTable([]TableColumn{{Title: "Name"}}, manyTableRows(20))
	tbl.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // bodyVisibleRows=3, max=17
	trackX := 200 - scrollbarWidth
	// contentH=440, trackH=66, thumbH=9, thumb [24,33) at ScrollRow 0.

	tbl.OnEvent(Event{Kind: EventClick, X: trackX + 3, Y: 26})
	if !tbl.sbDrag.active {
		t.Fatal("pressing the thumb should begin a drag")
	}
	tbl.OnEvent(Event{Kind: EventMouseDrag, X: trackX + 3, Y: 50})
	if tbl.ScrollRow != 7 { // (24*374 + 627)/1254 = 7.65 -> 7
		t.Fatalf("mid drag ScrollRow = %d, want 7", tbl.ScrollRow)
	}
	tbl.OnEvent(Event{Kind: EventMouseDrag, X: trackX + 3, Y: 90})
	if tbl.ScrollRow != 17 {
		t.Fatalf("drag past the end ScrollRow = %d, want 17 (clamped)", tbl.ScrollRow)
	}
	tbl.OnEvent(Event{Kind: EventMouseUp})
	if tbl.sbDrag.active {
		t.Fatal("EventMouseUp should end the drag")
	}

	// Track press below the thumb pages down.
	tbl.ScrollTo(0)
	tbl.OnEvent(Event{Kind: EventClick, X: trackX + 3, Y: 60})
	if tbl.ScrollRow != 3 {
		t.Fatalf("page-down ScrollRow = %d, want 3", tbl.ScrollRow)
	}

	// A body press left of the scrollbar still selects and never scrolls.
	tbl.MultiSelect = true // Table only tracks Selected in multi-select mode
	tbl.ScrollTo(5)
	tbl.Selected = -1
	tbl.OnEvent(Event{Kind: EventClick, X: 100, Y: TableHeaderHeight + 2})
	if tbl.Selected < 0 {
		t.Fatal("a content press must select a row")
	}
	if tbl.ScrollRow != 5 {
		t.Fatalf("selecting must not scroll: ScrollRow = %d, want 5", tbl.ScrollRow)
	}
}

// --- ScrollView: draggable scrollbars on both axes ------------------------

func newOverflowScrollView() *ScrollView {
	child := NewLabel("x")
	child.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	sv := NewScrollView(child)
	sv.SetContentSize(200, 200)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	return sv
}

func TestScrollViewVerticalScrollbarDrag(t *testing.T) {
	sv := newOverflowScrollView()
	// The overflow reserves a scrollGutter (track + normalized gap) on each axis,
	// so the content viewport shrinks to 40-scrollGutter on both; the clamp is
	// content-viewport and a page equals the viewport. The track itself stays a
	// scrollbarWidth column, so the thumb x-band is unchanged.
	viewport := 40 - scrollGutter()
	clamp := 200 - viewport
	// vertical thumb x-band [40-scrollbarWidth,40), y-thumb [0,8) at Offset 0.
	sv.OnEvent(Event{Kind: EventClick, X: 34, Y: 3})
	if !sv.sbV.active {
		t.Fatal("pressing the vertical thumb should begin a drag")
	}
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 34, Y: 10})
	if !(sv.OffsetY > 0 && sv.OffsetY < clamp) {
		t.Fatalf("mid drag OffsetY = %d, want strictly between 0 and %d", sv.OffsetY, clamp)
	}
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 34, Y: 40})
	if sv.OffsetY != clamp {
		t.Fatalf("drag past the end OffsetY = %d, want %d (clamped)", sv.OffsetY, clamp)
	}
	sv.OnEvent(Event{Kind: EventMouseUp})
	if sv.sbV.active {
		t.Fatal("EventMouseUp should end the vertical drag")
	}

	// Track press below the thumb pages down one viewport.
	sv.OffsetY = 0
	sv.OnEvent(Event{Kind: EventClick, X: 34, Y: 20})
	if sv.OffsetY != viewport {
		t.Fatalf("vertical page-down OffsetY = %d, want %d", sv.OffsetY, viewport)
	}
}

func TestScrollViewHorizontalScrollbarDrag(t *testing.T) {
	sv := newOverflowScrollView()
	viewport := 40 - scrollGutter()
	clamp := 200 - viewport
	// horizontal thumb y-band [40-scrollbarWidth,40), x-thumb [0,8) at Offset 0.
	sv.OnEvent(Event{Kind: EventClick, X: 3, Y: 34})
	if !sv.sbH.active {
		t.Fatal("pressing the horizontal thumb should begin a drag")
	}
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 34})
	if sv.OffsetX != clamp {
		t.Fatalf("drag past the end OffsetX = %d, want %d (clamped)", sv.OffsetX, clamp)
	}
	sv.OnEvent(Event{Kind: EventMouseUp})
	if sv.sbH.active {
		t.Fatal("EventMouseUp should end the horizontal drag")
	}

	// Track press right of the thumb pages right one viewport.
	sv.OffsetX = 0
	sv.OnEvent(Event{Kind: EventClick, X: 20, Y: 34})
	if sv.OffsetX != viewport {
		t.Fatalf("horizontal page-right OffsetX = %d, want %d", sv.OffsetX, viewport)
	}
}

func TestScrollViewScrollbarPassiveAndInertCases(t *testing.T) {
	sv := newOverflowScrollView()
	// A press in the content area (on neither bar) is a no-op: ScrollView stays
	// passive for content clicks.
	sv.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if sv.sbV.active || sv.sbH.active || sv.OffsetX != 0 || sv.OffsetY != 0 {
		t.Fatalf("content press must not scroll or grab: %+v", sv)
	}
	// A drag with no active grab is a no-op.
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 34, Y: 20})
	if sv.OffsetY != 0 {
		t.Fatalf("drag without a grab must not scroll, OffsetY=%d", sv.OffsetY)
	}
	// A grab that goes stale (content shrinks to fit) drags to nothing.
	sv.OnEvent(Event{Kind: EventClick, X: 34, Y: 3}) // grab vertical thumb
	sv.SetContentSize(10, 10)                        // now everything fits
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 34, Y: 30})
	if sv.OffsetY != 0 {
		t.Fatalf("dragging a stale grab must not scroll, OffsetY=%d", sv.OffsetY)
	}
	sv.OnEvent(Event{Kind: EventMouseUp})
}
