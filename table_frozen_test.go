// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// frozenTable builds a 4-column all-fixed (60px each = 240 natural) table in a
// 150px-wide viewport, so the columns overflow and horizontal scrolling is
// active; the first column is frozen. Two rows keep the body from overflowing
// vertically (contentWidth stays the full 150).
func frozenTable() *Table {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 60},
		{Title: "B", Width: 60},
		{Title: "C", Width: 60},
		{Title: "D", Width: 60},
	}, [][]string{{"a0", "b0", "c0", "d0"}, {"a1", "b1", "c1", "d1"}})
	tb.FrozenColumns = 1
	tb.SetBounds(Rect{X: 0, Y: 0, W: 150, H: 200})
	return tb
}

// TestTableHScrollableModel checks the activation predicate and derived widths.
func TestTableHScrollableModel(t *testing.T) {
	tb := frozenTable()
	if !tb.hScrollable() {
		t.Fatal("240px of columns in a 150px viewport must be horizontally scrollable")
	}
	if tb.naturalWidth() != 240 {
		t.Fatalf("naturalWidth=%d, want 240", tb.naturalWidth())
	}
	if tb.maxScrollX() != 90 {
		t.Fatalf("maxScrollX=%d, want 90 (240-150)", tb.maxScrollX())
	}
	if tb.frozenCount() != 1 || tb.frozenWidth(tb.columnWidths(tb.contentWidth())) != 60 {
		t.Fatalf("frozenCount=%d frozenWidth=%d, want 1, 60", tb.frozenCount(), tb.frozenWidth(tb.columnWidths(tb.contentWidth())))
	}

	// An auto-width column disables horizontal scroll (table fits to width),
	// which also forces frozenCount to 0.
	fit := NewTable([]TableColumn{{Title: "A", Width: 60}, {Title: "B"}}, [][]string{{"x", "y"}})
	fit.FrozenColumns = 1
	fit.SetBounds(Rect{X: 0, Y: 0, W: 150, H: 200})
	if fit.hasAutoColumn() != true || fit.hScrollable() {
		t.Fatal("a table with an auto column must not be hScrollable")
	}
	if fit.frozenCount() != 0 || fit.clampScrollX() != 0 {
		t.Fatal("non-scrollable table must report frozenCount 0 and clampScrollX 0")
	}
}

// TestTableFrozenCountClamp covers the low/high clamps of frozenCount.
func TestTableFrozenCountClamp(t *testing.T) {
	tb := frozenTable()
	tb.FrozenColumns = -3
	if tb.frozenCount() != 0 {
		t.Fatalf("frozenCount(neg)=%d, want 0", tb.frozenCount())
	}
	tb.FrozenColumns = 99
	if tb.frozenCount() != 4 {
		t.Fatalf("frozenCount(huge)=%d, want 4 (len Columns)", tb.frozenCount())
	}
}

// TestTableScrollXClampAndSetters covers ScrollXTo/ScrollXBy and clampScrollX
// (negative, over-max, and the inactive-table short-circuit).
func TestTableScrollXClampAndSetters(t *testing.T) {
	tb := frozenTable()
	tb.ScrollXTo(1000)
	if tb.ScrollX != 90 || tb.clampScrollX() != 90 {
		t.Fatalf("ScrollXTo(1000) → %d (clamp %d), want 90", tb.ScrollX, tb.clampScrollX())
	}
	tb.ScrollXBy(-30)
	if tb.ScrollX != 60 {
		t.Fatalf("ScrollXBy(-30) → %d, want 60", tb.ScrollX)
	}
	tb.ScrollXTo(-5)
	if tb.ScrollX != 0 {
		t.Fatalf("ScrollXTo(-5) → %d, want 0", tb.ScrollX)
	}
	// The raw clamp of an over-max value without going through the setter.
	tb.ScrollX = 500
	if tb.clampScrollX() != 90 {
		t.Fatalf("clampScrollX(raw 500)=%d, want 90", tb.clampScrollX())
	}
}

// TestTableColumnAtFrozenScrolled maps clicks through the frozen/scrolled split.
func TestTableColumnAtFrozenScrolled(t *testing.T) {
	tb := frozenTable() // ScrollX 0: [A|B C ...], A frozen at [0,60)

	if got := tb.columnAt(30); got != 0 { // frozen column A
		t.Fatalf("columnAt(30)=%d, want 0 (frozen A)", got)
	}
	if got := tb.columnAt(70); got != 1 { // B natural [60,120), screen [60,120)
		t.Fatalf("columnAt(70)=%d, want 1 (B)", got)
	}
	if got := tb.columnAt(130); got != 2 { // C natural [120,180), screen [120,180)
		t.Fatalf("columnAt(130)=%d, want 2 (C)", got)
	}
	if got := tb.columnAt(300); got != -1 { // past all content
		t.Fatalf("columnAt(300)=%d, want -1", got)
	}

	// Scroll so D (natural [180,240)) comes into the scrollable viewport.
	tb.ScrollXTo(90) // screen X of a scrolled col = natural - 90
	if got := tb.columnAt(100); got != 3 { // nat = 100+90 = 190 ∈ D
		t.Fatalf("after scroll, columnAt(100)=%d, want 3 (D)", got)
	}
	if got := tb.columnAt(30); got != 0 { // frozen A still pinned
		t.Fatalf("after scroll, columnAt(30)=%d, want 0 (A pinned)", got)
	}
}

// TestTableColumnSeparatorAtScrolled covers separator hit-testing across the
// split, including a scrolled separator that is out of view until scrolled in.
func TestTableColumnSeparatorAtScrolled(t *testing.T) {
	tb := frozenTable() // seps at screen x: after A=60, after B=120, after C=180(off)

	if got := tb.ColumnSeparatorAt(120); got != 1 { // after B, in view
		t.Fatalf("ColumnSeparatorAt(120)=%d, want 1", got)
	}
	if got := tb.ColumnSeparatorAt(180); got != -1 { // after C, off-screen (>150)
		t.Fatalf("ColumnSeparatorAt(180)=%d, want -1 (scrolled out)", got)
	}
	tb.ScrollXTo(90) // after C now at 180-90=90, in view
	if got := tb.ColumnSeparatorAt(90); got != 2 {
		t.Fatalf("after scroll, ColumnSeparatorAt(90)=%d, want 2", got)
	}
}

// TestTableFrozenTwoColumns covers the frozen-frozen separator path (f-1 loop)
// in both ColumnSeparatorAt and Draw, plus columnAt within a 2-wide frozen block.
func TestTableFrozenTwoColumns(t *testing.T) {
	tb := frozenTable()
	tb.FrozenColumns = 2 // A,B frozen (120 wide); C,D scroll in [120,150)+scroll

	if got := tb.ColumnSeparatorAt(60); got != 0 { // between the two frozen cols
		t.Fatalf("ColumnSeparatorAt(60)=%d, want 0 (frozen/frozen sep)", got)
	}
	if got := tb.columnAt(90); got != 1 { // frozen B [60,120)
		t.Fatalf("columnAt(90)=%d, want 1 (frozen B)", got)
	}
	buf := makeSurface(150, 200)
	tb.Draw(newP(buf, 150), DefaultLight()) // exercises the f-1 frozen-sep draw loop
}

// TestTableHScrollDrawAndScrollbar renders a frozen table and asserts the
// bottom horizontal scrollbar paints, moving with ScrollX.
func TestTableHScrollDrawAndScrollbar(t *testing.T) {
	tb := frozenTable()
	const w, h = 150, 200

	buf := makeSurface(w, h)
	tb.Draw(newP(buf, w), DefaultLight())
	// The h-scrollbar occupies the bottom scrollbarWidth rows across the
	// scrollable region; assert something painted there.
	if !anyPainted(buf, w, 60, h-scrollbarWidth, w, h) {
		t.Fatal("horizontal scrollbar painted nothing")
	}

	// Frozen column A's header text ("A") stays pinned at the left even when
	// scrolled; assert the top-left header area is painted after scrolling.
	tb.ScrollXTo(90)
	buf2 := makeSurface(w, h)
	tb.Draw(newP(buf2, w), DefaultLight())
	if !anyPainted(buf2, w, 0, 0, 60, TableHeaderHeight) {
		t.Fatal("frozen header column painted nothing after scroll")
	}
}

// TestTableHScrollEditorPosition: editing a scrolled cell positions the editor
// via the scrolled screen X (cellRect through columnScreenX).
func TestTableHScrollEditorPosition(t *testing.T) {
	tb := frozenTable()
	tb.Columns[2].Editable = true // column C (scrolled)
	tb.ScrollXTo(30)

	tb.beginEdit(0, 2)
	got := tb.cellRect(0, 2)
	// C natural left = 120; screen left = 120 - 30 = 90.
	if got.X != 90 {
		t.Fatalf("scrolled editable cell X=%d, want 90 (120 natural - 30 scroll)", got.X)
	}
	// A frozen editable cell is unaffected by scroll.
	tb.Columns[0].Editable = true
	if fx := tb.cellRect(0, 0).X; fx != 0 {
		t.Fatalf("frozen cell X=%d, want 0", fx)
	}
}

// TestTableHScrollResizeDrag: dragging a scrolled column's separator resizes it
// using the scrolled screen-space left edge.
func TestTableHScrollResizeDrag(t *testing.T) {
	tb := frozenTable()
	tb.ScrollXTo(0)
	// Grab the separator after B (screen x 120) and drag it to x 140: column B
	// spans natural [60,120), screen left 60, so new width = 140-60 = 80.
	tb.OnEvent(Event{Kind: EventClick, X: 120, Y: 5})
	tb.OnEvent(Event{Kind: EventMouseDrag, X: 140, Y: 5})
	tb.OnEvent(Event{Kind: EventMouseUp, X: 140, Y: 5})
	if tb.Columns[1].Width != 80 {
		t.Fatalf("resized B width=%d, want 80", tb.Columns[1].Width)
	}
}

// TestTableHScrollWithoutClipperFallsBack drives Draw with a Painter that hides
// its Clipper capability: the frozen/scrolled passes run unclipped (withClip's
// fallback) without panic.
func TestTableHScrollWithoutClipperFallsBack(t *testing.T) {
	tb := frozenTable()
	buf := makeSurface(150, 200)
	tb.Draw(noClipPainter{newP(buf, 150)}, DefaultLight())
}
