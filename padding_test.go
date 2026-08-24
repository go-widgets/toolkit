// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// evProbe is a measurable widget that records every event it receives, so a
// wrapper's coordinate routing can be asserted exactly.
type evProbe struct {
	Base
	mw, mh int
	got    []Event
}

func (e *evProbe) Measure(_, _ int) (int, int) { return e.mw, e.mh }
func (e *evProbe) OnEvent(ev Event)            { e.got = append(e.got, ev) }

// TestPaddingInsetsChildByScaledPadding pins the core geometry: the child is
// seated inset by exactly the (scaled) per-side padding, and the wrapper's own
// bounds are unchanged.
func TestPaddingInsetsChildByScaledPadding(t *testing.T) {
	child := &evProbe{}
	pd := NewPadding(child, 5) // uniform 5 logical px, scale 1.0
	pd.SetBounds(Rect{X: 10, Y: 20, W: 100, H: 50})

	if b := pd.Bounds(); b != (Rect{X: 10, Y: 20, W: 100, H: 50}) {
		t.Fatalf("wrapper bounds = %+v, want unchanged", b)
	}
	// Inset by 5 on every side: X+5, Y+5, W-10, H-10.
	if b := child.Bounds(); b != (Rect{X: 15, Y: 25, W: 90, H: 40}) {
		t.Fatalf("child bounds = %+v, want {15 25 90 40}", b)
	}
}

// TestPaddingPerSideInsets checks the four sides are applied independently.
func TestPaddingPerSideInsets(t *testing.T) {
	child := &evProbe{}
	pd := &Padding{Left: 1, Top: 2, Right: 3, Bottom: 4, child: child}
	pd.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	// X+1, Y+2, W-(1+3), H-(2+4).
	if b := child.Bounds(); b != (Rect{X: 1, Y: 2, W: 36, H: 24}) {
		t.Fatalf("child bounds = %+v, want {1 2 36 24}", b)
	}
}

// TestPaddingScalesWithMetricScale is the red-without-scaling assertion: at
// MetricScale 2, an 8-logical-px pad insets the child by 16 device px.
func TestPaddingScalesWithMetricScale(t *testing.T) {
	defer SetMetricScale(1)
	SetMetricScale(2)

	child := &evProbe{}
	pd := NewPadding(child, 8)
	pd.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	if b := child.Bounds(); b != (Rect{X: 16, Y: 16, W: 68, H: 68}) {
		t.Fatalf("child bounds at scale 2 = %+v, want {16 16 68 68}", b)
	}
}

// TestPaddingNegativeClampsToZero checks a negative side is treated as zero (no
// inset, no overflow), covering the clamp branch of insets.
func TestPaddingNegativeClampsToZero(t *testing.T) {
	child := &evProbe{}
	pd := &Padding{Left: -7, Top: 0, Right: 4, Bottom: 0, child: child}
	pd.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 20})
	// Left clamps to 0, Right is 4: X unchanged, W-4.
	if b := child.Bounds(); b != (Rect{X: 0, Y: 0, W: 46, H: 20}) {
		t.Fatalf("child bounds = %+v, want {0 0 46 20}", b)
	}
}

// TestPaddingMeasureAddsBothPads checks Measure reports the child's measured
// size grown by the two paddings per axis (child+2*pad), for a Measurer child.
func TestPaddingMeasureAddsBothPads(t *testing.T) {
	child := &evProbe{mw: 30, mh: 12}
	pd := NewPadding(child, 5)
	w, h := pd.Measure(200, 200)
	if w != 40 || h != 22 { // 30+2*5, 12+2*5
		t.Fatalf("Measure = %d,%d, want 40,22", w, h)
	}
}

// TestPaddingMeasureNonMeasurerUsesBounds covers the fallback where the child
// does not implement Measurer: its current Bounds size is grown by the pads.
func TestPaddingMeasureNonMeasurerUsesBounds(t *testing.T) {
	child := &alignProbe{}
	child.SetBounds(Rect{W: 20, H: 8})
	pd := NewPadding(child, 3)
	w, h := pd.Measure(100, 100)
	if w != 26 || h != 14 { // 20+6, 8+6
		t.Fatalf("Measure = %d,%d, want 26,14", w, h)
	}
}

// TestPaddingNilChild exercises every method with no child: nothing panics and
// Measure reports just the paddings.
func TestPaddingNilChild(t *testing.T) {
	pd := NewPadding(nil, 4)
	pd.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10}) // child==nil early return
	if w, h := pd.Measure(50, 50); w != 8 || h != 8 {
		t.Fatalf("Measure with nil child = %d,%d, want 8,8", w, h)
	}
	pd.Draw(newP(makeSurface(10, 10), 10), DefaultLight()) // child==nil: paints nothing
	pd.OnEvent(Event{Kind: EventClick})                    // child==nil early return
	if got := pd.Children(); len(got) != 0 {
		t.Fatalf("nil child must yield no children, got %d", len(got))
	}
	if got := pd.focusableChildren(); len(got) != 0 {
		t.Fatalf("nil child must yield no focusable children, got %d", len(got))
	}
}

// TestPaddingExposesChildAndRole checks the accessibility surface: the child is
// exposed for generic walks and the wrapper is presentational.
func TestPaddingExposesChildAndRole(t *testing.T) {
	child := &evProbe{}
	pd := NewPadding(child, 2)
	if got := pd.Children(); len(got) != 1 || got[0] != child {
		t.Fatalf("Children = %v, want [child]", got)
	}
	if got := pd.focusableChildren(); len(got) != 1 || got[0] != child {
		t.Fatalf("focusableChildren = %v, want [child]", got)
	}
	if pd.A11y().Role != RolePresentation {
		t.Fatalf("A11y role = %q, want presentation", pd.A11y().Role)
	}
}

// TestPaddingDrawForwardsToChild checks Draw delegates to the child (the drawn
// pixels come from the child, not the wrapper).
func TestPaddingDrawForwardsToChild(t *testing.T) {
	const w, h = 40, 20
	lbl := NewLabel("Hi")
	pd := NewPadding(lbl, 2)
	pd.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	pd.Draw(newP(buf, w), DefaultLight())
	if labelTopRow(buf, w, h) < 0 {
		t.Fatal("Padding.Draw must forward to the child (nothing painted)")
	}
}

// TestPaddingRoutesEvents covers every branch of OnEvent: a keyboard event taken
// by the focus system, a move forwarded unconditionally, a click translated to
// child-local coords, a non-click pointer event forwarded, and a click outside
// the child dropped.
func TestPaddingRoutesEvents(t *testing.T) {
	child := &evProbe{}
	pd := NewPadding(child, 5)
	pd.SetBounds(Rect{X: 10, Y: 20, W: 100, H: 50}) // child at {15,25,90,40}

	// (a) A keyboard event is consumed by routeFocusKey and never reaches the
	// child's positional routing.
	pd.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if len(child.got) != 0 {
		t.Fatalf("keyboard event must not be positionally routed, child saw %d", len(child.got))
	}

	// (b) A click at the child's top-left (surface 15,25 → widget-local 5,5)
	// arrives at the child in child-local coords (0,0).
	pd.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if len(child.got) != 1 {
		t.Fatalf("click inside child must be forwarded once, got %d", len(child.got))
	}
	if e := child.got[0]; e.X != 0 || e.Y != 0 {
		t.Fatalf("forwarded click = (%d,%d), want child-local (0,0)", e.X, e.Y)
	}

	// (c) A move is forwarded unconditionally (translated).
	child.got = nil
	pd.OnEvent(Event{Kind: EventMouseMove, X: 0, Y: 0})
	if len(child.got) != 1 || child.got[0].Kind != EventMouseMove {
		t.Fatalf("move must be forwarded unconditionally, got %v", child.got)
	}

	// (d) A non-click pointer event inside the child is forwarded (the branch
	// that does NOT call focusClick).
	child.got = nil
	pd.OnEvent(Event{Kind: EventMouseUp, X: 5, Y: 5})
	if len(child.got) != 1 || child.got[0].Kind != EventMouseUp {
		t.Fatalf("mouseup inside child must be forwarded, got %v", child.got)
	}

	// (e) A click in the padding margin (outside the child) is dropped.
	child.got = nil
	pd.OnEvent(Event{Kind: EventClick, X: 0, Y: 0}) // surface 10,20 → outside child
	if len(child.got) != 0 {
		t.Fatalf("click outside the child must be dropped, child saw %d", len(child.got))
	}
}
