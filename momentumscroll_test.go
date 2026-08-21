// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// newMomentumScrollView returns a ScrollView with content larger than its
// viewport on both axes, so both momentum axes have somewhere to travel.
func newMomentumScrollView() *ScrollView {
	sv := NewScrollView(NewLabel("content"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	sv.contentW, sv.contentH = 400, 400
	return sv
}

// A drag-then-release flings the ScrollView: the offset keeps growing past
// where the finger let go, then decelerates to a clean stop within bounds.
func TestMomentumScrollerFlingDrivesViewOffset(t *testing.T) {
	sv := newMomentumScrollView()
	sc := NewMomentumScroller(sv)
	// Deterministic tuning: half the velocity survives each second, hard edges.
	sc.AxisX = &Momentum{Friction: 0.5, StopVelocity: 1, Bounce: false}
	sc.AxisY = &Momentum{Friction: 0.5, StopVelocity: 1, Bounce: false}

	sc.TouchDown(Event{X: 40, Y: 60})
	sc.TouchMove(Event{X: 40, Y: 40}, 1) // finger up 20 -> offset +20, v sample 20
	sc.TouchMove(Event{X: 40, Y: 20}, 1) // finger up 20 more -> offset 40, v 20
	if sv.OffsetY().Get() != 40 {
		t.Fatalf("after drag: OffsetY=%d, want 40", sv.OffsetY().Get())
	}
	sc.TouchUp() // fling AxisY at +20 px/s

	// Coast: v=10 off=50, v=5 off=55, v=2.5 off=57.5, v=1.25 off=58.75,
	// v=0.625 <= 1 -> stop. Rounded offset 59.
	settled := false
	for i := 0; i < 100; i++ {
		if !sc.Tick(1) {
			settled = true
			break
		}
	}
	if !settled {
		t.Fatalf("scroller never settled")
	}
	if sc.Settling() {
		t.Fatalf("Settling() true after settle")
	}
	if sv.OffsetY().Get() != 59 {
		t.Fatalf("flung OffsetY=%d, want 59 (drag 40 + coast ~19)", sv.OffsetY().Get())
	}
	if sv.OffsetX().Get() != 0 {
		t.Fatalf("OffsetX=%d, want 0 (no horizontal flick)", sv.OffsetX().Get())
	}
}

// TouchMove before TouchDown is inert — a stray move with no active drag must
// not scroll anything.
func TestMomentumScrollerMoveBeforeDownIsNoOp(t *testing.T) {
	sv := newMomentumScrollView()
	sc := NewMomentumScroller(sv)
	sc.TouchMove(Event{X: 10, Y: 10}, 1)
	if sv.OffsetX().Get() != 0 || sv.OffsetY().Get() != 0 {
		t.Fatalf("move before down scrolled: (%d,%d)", sv.OffsetX().Get(), sv.OffsetY().Get())
	}
	if sc.Settling() {
		t.Fatalf("Settling() true with no interaction")
	}
}

// When the content fits inside the viewport there is nowhere to scroll: both
// axis bounds floor at zero.
func TestMomentumScrollerSyncBoundsFloorsAtZero(t *testing.T) {
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	sv.contentW, sv.contentH = 1, 1 // smaller than the viewport
	sc := NewMomentumScroller(sv)
	sc.syncBounds()
	if _, maxX := sc.AxisX.Bounds(); maxX != 0 {
		t.Fatalf("maxX=%v, want 0 for non-overflowing content", maxX)
	}
	if _, maxY := sc.AxisY.Bounds(); maxY != 0 {
		t.Fatalf("maxY=%v, want 0 for non-overflowing content", maxY)
	}
}

// The adapter is additive: a ScrollView behaves identically on its default
// (non-touch) event path whether or not a MomentumScroller is wrapped around
// it. Driving the SAME wheel + pan events through a wrapped and an unwrapped
// view must land on the same offsets.
func TestMomentumScrollerLeavesDefaultPathUnchanged(t *testing.T) {
	plain := newMomentumScrollView()
	wrapped := newMomentumScrollView()
	_ = NewMomentumScroller(wrapped) // wrap, but never route touch to it

	events := []Event{
		{Kind: EventScroll, Delta: 3},
		{Kind: EventClick, X: 40, Y: 60},
		{Kind: EventMouseDrag, X: 40, Y: 30},
		{Kind: EventMouseDrag, X: 10, Y: 30},
		{Kind: EventMouseUp},
		{Kind: EventKeyDown, Code: "PageDown"},
	}
	for _, ev := range events {
		plain.OnEvent(ev)
		wrapped.OnEvent(ev)
	}
	if plain.OffsetX().Get() != wrapped.OffsetX().Get() || plain.OffsetY().Get() != wrapped.OffsetY().Get() {
		t.Fatalf("wrapping changed default behaviour: plain=(%d,%d) wrapped=(%d,%d)",
			plain.OffsetX().Get(), plain.OffsetY().Get(), wrapped.OffsetX().Get(), wrapped.OffsetY().Get())
	}
}

// A release while overscrolled springs the view back to the exact bound (proves
// the rubber-band path reaches the real ScrollView offset). Uses the default
// bouncy tuning.
func TestMomentumScrollerReleaseOverscrolledSpringsHome(t *testing.T) {
	sv := newMomentumScrollView()
	sc := NewMomentumScroller(sv)
	sc.TouchDown(Event{X: 40, Y: 40})
	// Drag the content DOWN hard (finger moving down) -> offset below 0, rubber.
	sc.TouchMove(Event{X: 40, Y: 240}, 1)
	if sv.OffsetY().Get() >= 0 {
		// Overscrolled at the top: engine offset is negative; rounded view
		// offset should be <= 0. (Exactly 0 only if resistance rounded there.)
		if sc.AxisY.Offset() >= 0 {
			t.Fatalf("expected overscroll past top, AxisY offset=%v", sc.AxisY.Offset())
		}
	}
	sc.TouchUp()
	settled := false
	for i := 0; i < 1000; i++ {
		if !sc.Tick(0.1) {
			settled = true
			break
		}
	}
	if !settled {
		t.Fatalf("spring never settled")
	}
	if sv.OffsetY().Get() != 0 {
		t.Fatalf("sprung OffsetY=%d, want exactly 0 (home)", sv.OffsetY().Get())
	}
}
