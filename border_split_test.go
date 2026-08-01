// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestBorderSplitLayout checks a splittable region reserves a BorderSplitW handle
// strip between it and the centre, on every edge, and that the handle sits at the
// region's inner boundary.
func TestBorderSplitLayout(t *testing.T) {
	n, w, c := &dockProbe{}, &dockProbe{}, &dockProbe{}
	b := NewBorder()
	b.North, b.NorthSize, b.NorthSplit = n, 10, true
	b.West, b.WestSize, b.WestSplit = w, 20, true
	b.Center = c
	b.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 50})

	// North 10 tall at top; its handle is the next 6px strip (Y 10..16), full width
	// at that moment (before West carves), so centre starts at Y=16.
	if got := n.Bounds(); got != (Rect{X: 0, Y: 0, W: 100, H: 10}) {
		t.Fatalf("north = %+v", got)
	}
	// West 20 wide, in the band below the north handle (Y 16..50); its handle is the
	// next 6px column (X 20..26).
	if got := w.Bounds(); got != (Rect{X: 0, Y: 16, W: 20, H: 34}) {
		t.Fatalf("west = %+v", got)
	}
	if got := c.Bounds(); got != (Rect{X: 26, Y: 16, W: 74, H: 34}) {
		t.Fatalf("center = %+v", got)
	}

	// Two handles recorded, at the right places.
	ns, ok := b.SplitHandleAt(50, 12) // inside the north handle strip (Y 10..16)
	if !ok || ns != DockTop {
		t.Fatalf("north handle hit = %v,%v", ns, ok)
	}
	ws, ok := b.SplitHandleAt(22, 30) // inside the west handle column (X 20..26)
	if !ok || ws != DockLeft {
		t.Fatalf("west handle hit = %v,%v", ws, ok)
	}
	if _, ok := b.SplitHandleAt(60, 30); ok { // in the centre, no handle
		t.Fatal("centre point should not hit a handle")
	}
}

// TestBorderResizeSplit covers ResizeSplit for every edge, the negative and
// over-extent clamps, and the OnResize callback (fired and nil).
func TestBorderResizeSplit(t *testing.T) {
	b := NewBorder()
	b.North, b.NorthSplit = &dockProbe{}, true
	b.South, b.SouthSplit = &dockProbe{}, true
	b.West, b.WestSplit = &dockProbe{}, true
	b.East, b.EastSplit = &dockProbe{}, true
	var gotSide DockSide
	var gotSize int
	calls := 0
	b.OnResize = func(side DockSide, size int) { gotSide, gotSize, calls = side, size, calls+1 }
	b.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})

	b.ResizeSplit(DockTop, 25)
	if b.NorthSize != 25 || gotSide != DockTop || gotSize != 25 || calls != 1 {
		t.Fatalf("resize north: size=%d cb=%v/%d calls=%d", b.NorthSize, gotSide, gotSize, calls)
	}
	b.ResizeSplit(DockBottom, 12)
	if b.SouthSize != 12 {
		t.Fatalf("resize south = %d", b.SouthSize)
	}
	b.ResizeSplit(DockLeft, 30)
	if b.WestSize != 30 {
		t.Fatalf("resize west = %d", b.WestSize)
	}
	b.ResizeSplit(DockRight, 15)
	if b.EastSize != 15 {
		t.Fatalf("resize east = %d", b.EastSize)
	}

	// Negative clamps to 0.
	b.ResizeSplit(DockTop, -5)
	if b.NorthSize != 0 {
		t.Fatalf("negative clamp = %d", b.NorthSize)
	}
	// Over-extent clamps to the axis size: height for N/S, width for E/W.
	b.ResizeSplit(DockBottom, 9999)
	if b.SouthSize != 60 {
		t.Fatalf("vertical over-clamp = %d, want 60", b.SouthSize)
	}
	b.ResizeSplit(DockRight, 9999)
	if b.EastSize != 100 {
		t.Fatalf("horizontal over-clamp = %d, want 100", b.EastSize)
	}

	// nil OnResize must not panic.
	b.OnResize = nil
	b.ResizeSplit(DockLeft, 5)
}

// TestBorderSplitDraw paints a split border (exercises the handle-fill loop).
func TestBorderSplitDraw(t *testing.T) {
	b := NewBorder()
	b.West, b.WestSize, b.WestSplit = &dockProbe{}, 10, true
	b.Center = &dockProbe{}
	b.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	buf := makeSurface(40, 20)
	b.Draw(newP(buf, 40), DefaultLight())
	// The handle column (X 10..16) should have painted the SurfaceAlt seam.
	th := DefaultLight()
	if px := pixelAt(buf, 40, 12, 10); px != th.SurfaceAlt {
		t.Fatalf("handle not painted: %+v want %+v", px, th.SurfaceAlt)
	}
}
