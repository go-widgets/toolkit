// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestAlignBoxFillsBothAxes checks the zero value (AnchorFill on both axes)
// stretches the child to fill the box.
func TestAlignBoxFillsBothAxes(t *testing.T) {
	child := &measProbe{mw: 10, mh: 10}
	a := NewAlignBox(child)
	a.SetBounds(Rect{X: 3, Y: 4, W: 60, H: 40})
	if b := child.Bounds(); b != (Rect{X: 3, Y: 4, W: 60, H: 40}) {
		t.Fatalf("fill: child = %+v, want the full box {3 4 60 40}", b)
	}
}

// TestAlignBoxCenter checks NewCenter centres an intrinsically-sized child on
// both axes at exactly ((W-w)/2, (H-h)/2).
func TestAlignBoxCenter(t *testing.T) {
	child := &measProbe{mw: 20, mh: 10}
	a := NewCenter(child)
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 50})
	// X = (100-20)/2 = 40, Y = (50-10)/2 = 20.
	if b := child.Bounds(); b != (Rect{X: 40, Y: 20, W: 20, H: 10}) {
		t.Fatalf("center: child = %+v, want {40 20 20 10}", b)
	}
}

// TestAlignBoxVCenterFixedHeight is the desktop vcenterWidget case: a fixed
// content height centred vertically at (H-childH)/2 while the width fills.
func TestAlignBoxVCenterFixedHeight(t *testing.T) {
	child := &measProbe{mw: 30, mh: 99} // intrinsic height ignored; FixedH wins
	a := NewVCenter(child, 20)
	a.SetBounds(Rect{X: 5, Y: 0, W: 200, H: 60})
	// Width fills; height pinned to 20, centred: Y = (60-20)/2 = 20.
	if b := child.Bounds(); b != (Rect{X: 5, Y: 20, W: 200, H: 20}) {
		t.Fatalf("vcenter: child = %+v, want {5 20 200 20}", b)
	}
}

// TestAlignBoxVCenterIntrinsicHeight checks NewVCenter with a zero fixed height
// centres the child at its own intrinsic height.
func TestAlignBoxVCenterIntrinsicHeight(t *testing.T) {
	child := &measProbe{mw: 30, mh: 12}
	a := NewVCenter(child, 0)
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 40})
	// Y = (40-12)/2 = 14, width fills.
	if b := child.Bounds(); b != (Rect{X: 0, Y: 14, W: 100, H: 12}) {
		t.Fatalf("vcenter intrinsic: child = %+v, want {0 14 100 12}", b)
	}
}

// TestAlignBoxStartEnd covers the AnchorStart and AnchorEnd branches on both
// axes, with the intrinsic size coming from a Measurer.
func TestAlignBoxStartEnd(t *testing.T) {
	// Start on X, End on Y.
	child := &measProbe{mw: 20, mh: 10}
	a := &AlignBox{Horizontal: AnchorStart, Vertical: AnchorEnd, child: child}
	a.SetBounds(Rect{X: 2, Y: 3, W: 100, H: 50})
	// X pinned leading: 2, W 20. Y pinned trailing: 3 + (50-10) = 43, H 10.
	if b := child.Bounds(); b != (Rect{X: 2, Y: 43, W: 20, H: 10}) {
		t.Fatalf("start/end: child = %+v, want {2 43 20 10}", b)
	}
}

// TestAlignBoxFixedWidth covers the FixedW override (scaled) on an anchored axis.
func TestAlignBoxFixedWidth(t *testing.T) {
	child := &measProbe{mw: 5, mh: 5} // intrinsic width ignored; FixedW wins
	a := &AlignBox{Horizontal: AnchorMiddle, Vertical: AnchorFill, FixedW: 40, child: child}
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 30})
	// Width pinned 40, centred: X = (100-40)/2 = 30. Height fills.
	if b := child.Bounds(); b != (Rect{X: 30, Y: 0, W: 40, H: 30}) {
		t.Fatalf("fixed width: child = %+v, want {30 0 40 30}", b)
	}
}

// TestAlignBoxNonMeasurerUsesBounds covers childNatural's fallback to the child's
// current Bounds when it is not a Measurer.
func TestAlignBoxNonMeasurerUsesBounds(t *testing.T) {
	child := &alignProbe{}
	child.SetBounds(Rect{W: 24, H: 16})
	a := NewCenter(child)
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 40})
	// X = (100-24)/2 = 38, Y = (40-16)/2 = 12.
	if b := child.Bounds(); b != (Rect{X: 38, Y: 12, W: 24, H: 16}) {
		t.Fatalf("non-measurer center: child = %+v, want {38 12 24 16}", b)
	}
}

// TestAlignBoxZeroAndOversizedNaturalFill covers the two axisLayout fallbacks on
// an anchored axis: a zero intrinsic size, and one at least as large as the
// extent, both stretch to fill.
func TestAlignBoxZeroAndOversizedNaturalFill(t *testing.T) {
	// Zero natural width on a centred X axis → fill.
	zero := &measProbe{mw: 0, mh: 10}
	a := &AlignBox{Horizontal: AnchorMiddle, Vertical: AnchorFill, child: zero}
	a.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 20})
	if b := zero.Bounds(); b.X != 0 || b.W != 50 {
		t.Fatalf("zero natural width must fill: X=%d W=%d, want 0,50", b.X, b.W)
	}

	// Natural height >= the extent on a centred Y axis → fill.
	big := &measProbe{mw: 10, mh: 99}
	a = &AlignBox{Horizontal: AnchorFill, Vertical: AnchorMiddle, child: big}
	a.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 30})
	if b := big.Bounds(); b.Y != 0 || b.H != 30 {
		t.Fatalf("oversized natural height must fill: Y=%d H=%d, want 0,30", b.Y, b.H)
	}
}

// TestAlignBoxMeasure checks Measure reports the intrinsic size, and the fixed
// overrides on each axis, and 0,0 for a nil child.
func TestAlignBoxMeasure(t *testing.T) {
	if w, h := NewAlignBox(nil).Measure(50, 50); w != 0 || h != 0 {
		t.Fatalf("nil child Measure = %d,%d, want 0,0", w, h)
	}
	child := &measProbe{mw: 12, mh: 8}
	if w, h := NewAlignBox(child).Measure(100, 100); w != 12 || h != 8 {
		t.Fatalf("intrinsic Measure = %d,%d, want 12,8", w, h)
	}
	fixed := &AlignBox{FixedW: 30, FixedH: 14, child: child}
	if w, h := fixed.Measure(100, 100); w != 30 || h != 14 {
		t.Fatalf("fixed Measure = %d,%d, want 30,14", w, h)
	}
}

// TestAlignBoxScaledFixed checks a fixed size is expressed in scaled pixels.
func TestAlignBoxScaledFixed(t *testing.T) {
	defer SetMetricScale(1)
	SetMetricScale(2)
	child := &measProbe{mw: 5, mh: 5}
	a := NewVCenter(child, 10) // 10 logical → 20 device
	a.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	// Height 20, centred: Y = (40-20)/2 = 10.
	if b := child.Bounds(); b.H != 20 || b.Y != 10 {
		t.Fatalf("scaled fixed height: Y=%d H=%d, want 10,20", b.Y, b.H)
	}
}

// TestAlignBoxNilChild exercises the nil-child paths of every method.
func TestAlignBoxNilChild(t *testing.T) {
	a := NewCenter(nil)
	a.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10}) // child==nil early return
	a.Draw(newP(makeSurface(10, 10), 10), DefaultLight())
	a.OnEvent(Event{Kind: EventClick})
	if len(a.Children()) != 0 || len(a.focusableChildren()) != 0 {
		t.Fatal("nil child must yield no children")
	}
}

// TestAlignBoxExposesChildAndRole checks the accessibility surface.
func TestAlignBoxExposesChildAndRole(t *testing.T) {
	child := &measProbe{mw: 5, mh: 5}
	a := NewCenter(child)
	if got := a.Children(); len(got) != 1 || got[0] != child {
		t.Fatalf("Children = %v, want [child]", got)
	}
	if got := a.focusableChildren(); len(got) != 1 || got[0] != child {
		t.Fatalf("focusableChildren = %v, want [child]", got)
	}
	if a.A11y().Role != RolePresentation {
		t.Fatalf("A11y role = %q, want presentation", a.A11y().Role)
	}
}

// TestAlignBoxDrawForwardsToChild checks Draw delegates to the child.
func TestAlignBoxDrawForwardsToChild(t *testing.T) {
	const w, h = 60, 30
	lbl := NewLabel("Hi")
	a := NewCenter(lbl)
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	a.Draw(newP(buf, w), DefaultLight())
	if labelTopRow(buf, w, h) < 0 {
		t.Fatal("AlignBox.Draw must forward to the child")
	}
}

// TestAlignBoxRoutesEvents covers every OnEvent branch, mirroring the Padding
// routing: keyboard consumed, move forwarded, click translated, non-click
// forwarded, and an outside click dropped.
func TestAlignBoxRoutesEvents(t *testing.T) {
	child := &evProbe{mw: 20, mh: 10}
	a := NewCenter(child)
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 50}) // child centred at {40,20,20,10}

	a.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if len(child.got) != 0 {
		t.Fatalf("keyboard event must not be positionally routed, saw %d", len(child.got))
	}

	// Click at the child's top-left (surface 40,20 → widget-local 40,20).
	a.OnEvent(Event{Kind: EventClick, X: 40, Y: 20})
	if len(child.got) != 1 || child.got[0].X != 0 || child.got[0].Y != 0 {
		t.Fatalf("click must arrive child-local (0,0), got %v", child.got)
	}

	child.got = nil
	a.OnEvent(Event{Kind: EventMouseMove, X: 0, Y: 0})
	if len(child.got) != 1 || child.got[0].Kind != EventMouseMove {
		t.Fatalf("move must be forwarded unconditionally, got %v", child.got)
	}

	child.got = nil
	a.OnEvent(Event{Kind: EventMouseUp, X: 45, Y: 22})
	if len(child.got) != 1 || child.got[0].Kind != EventMouseUp {
		t.Fatalf("mouseup inside child must be forwarded, got %v", child.got)
	}

	child.got = nil
	a.OnEvent(Event{Kind: EventClick, X: 0, Y: 0}) // corner, outside centred child
	if len(child.got) != 0 {
		t.Fatalf("click outside the child must be dropped, saw %d", len(child.got))
	}
}
