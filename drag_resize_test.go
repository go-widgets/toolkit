// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestPanedDragResizes: grabbing the handle and dragging moves the split;
// a drag with no grab is a no-op; the vertical orientation resizes on Y.
func TestPanedDragResizes(t *testing.T) {
	p := NewHPaned(NewLabel("L"), NewLabel("R"))
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	p.OnEvent(Event{Kind: EventClick, X: p.Position + 1, Y: 50}) // grab handle
	if !p.resizing {
		t.Fatal("handle press did not start a resize")
	}
	p.OnEvent(Event{Kind: EventMouseDrag, X: 60, Y: 50})
	if p.Position != 60 {
		t.Fatalf("Position=%d, want 60", p.Position)
	}
	p.OnEvent(Event{Kind: EventMouseUp, X: 60, Y: 50})
	if p.resizing {
		t.Fatal("still resizing after mouse up")
	}
	before := p.Position
	p.OnEvent(Event{Kind: EventMouseDrag, X: 20, Y: 50}) // no grab → no-op
	if p.Position != before {
		t.Fatal("drag without a grab moved the split")
	}

	v := NewVPaned(NewLabel("T"), NewLabel("B"))
	v.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	v.OnEvent(Event{Kind: EventClick, X: 50, Y: v.Position + 1})
	v.OnEvent(Event{Kind: EventMouseDrag, X: 50, Y: 70})
	if v.Position != 70 {
		t.Fatalf("vertical Position=%d, want 70", v.Position)
	}
}

// TestBorderSplitterDragResizes: grabbing a West splitter and dragging resizes
// the West region; sizeFromPointer maps all four sides; non-resize drag/up
// fall through to region routing.
func TestBorderSplitterDragResizes(t *testing.T) {
	b := NewBorder()
	b.West = NewLabel("W")
	b.WestSize = 40
	b.WestSplit = true
	b.Center = NewLabel("C")
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})

	hx := -1
	for x := 0; x < 200; x++ {
		if side, ok := b.SplitHandleAt(x, 50); ok && side == DockLeft {
			hx = x
			break
		}
	}
	if hx < 0 {
		t.Fatal("no west splitter handle found")
	}
	b.OnEvent(Event{Kind: EventClick, X: hx, Y: 50})
	if !b.resizing || b.resizeSide != DockLeft {
		t.Fatalf("west handle press: resizing=%v side=%v", b.resizing, b.resizeSide)
	}
	b.OnEvent(Event{Kind: EventMouseDrag, X: 80, Y: 50})
	if b.WestSize != 80 {
		t.Fatalf("WestSize=%d, want 80", b.WestSize)
	}
	b.OnEvent(Event{Kind: EventMouseUp, X: 80, Y: 50})
	if b.resizing {
		t.Fatal("still resizing after mouse up")
	}
	// Non-resize drag / up now fall through to region routing (no panic).
	b.OnEvent(Event{Kind: EventMouseDrag, X: 120, Y: 50})
	b.OnEvent(Event{Kind: EventMouseUp, X: 120, Y: 50})

	if b.sizeFromPointer(DockTop, 10, 30) != 30 ||
		b.sizeFromPointer(DockBottom, 10, 30) != 70 ||
		b.sizeFromPointer(DockLeft, 50, 10) != 50 ||
		b.sizeFromPointer(DockRight, 50, 10) != 150 {
		t.Fatal("sizeFromPointer mapping wrong")
	}
}

// TestChartValueAt covers LineChart/BarChart ValueAt hover mapping.
func TestChartValueAt(t *testing.T) {
	lc := NewLineChart([]float64{3, 7, 2, 8, 5})
	lc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	for i := range lc.Series {
		x, _ := lc.pointAt(i, 0, 10) // surface x of point i
		gi, gv, ok := lc.ValueAt(x - lc.Bounds().X)
		if !ok || gi != i || gv != lc.Series[i] {
			t.Fatalf("LineChart.ValueAt point %d = (%d,%v,%v)", i, gi, gv, ok)
		}
	}
	if _, _, ok := NewLineChart(nil).ValueAt(10); ok {
		t.Fatal("empty LineChart.ValueAt should be ok=false")
	}
	if i, v, ok := NewLineChart([]float64{9}).ValueAt(999); !ok || i != 0 || v != 9 {
		t.Fatalf("single-point LineChart.ValueAt = (%d,%v,%v)", i, v, ok)
	}

	bc := NewBarChart([]float64{4, 7, 2})
	bc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	slot := bc.plot().W / len(bc.Values)
	for i := range bc.Values {
		gi, gv, ok := bc.ValueAt(ChartPad + i*slot + slot/2)
		if !ok || gi != i || gv != bc.Values[i] {
			t.Fatalf("BarChart.ValueAt bar %d = (%d,%v,%v)", i, gi, gv, ok)
		}
	}
	if _, _, ok := bc.ValueAt(-5); ok {
		t.Fatal("BarChart.ValueAt left of plot should be ok=false")
	}
	if _, _, ok := bc.ValueAt(100000); ok {
		t.Fatal("BarChart.ValueAt past last bar should be ok=false")
	}
	if _, _, ok := NewBarChart(nil).ValueAt(10); ok {
		t.Fatal("empty BarChart.ValueAt should be ok=false")
	}
	// Narrow boxes hit the span<1 / slot<1 guards.
	tiny := NewLineChart([]float64{1, 2, 3})
	tiny.SetBounds(Rect{X: 0, Y: 0, W: ChartPad + 1, H: 10})
	if i, v, ok := tiny.ValueAt(0); !ok || i != 0 || v != 1 {
		t.Fatalf("narrow LineChart.ValueAt = (%d,%v,%v)", i, v, ok)
	}
	many := make([]float64, 300)
	for i := range many {
		many[i] = float64(i)
	}
	nb := NewBarChart(many)
	nb.SetBounds(Rect{X: 0, Y: 0, W: ChartPad + 10, H: 10}) // slot < 1 -> clamped to 1
	if _, _, ok := nb.ValueAt(ChartPad + 1); !ok {
		t.Fatal("narrow BarChart.ValueAt should still resolve a bar")
	}
}
