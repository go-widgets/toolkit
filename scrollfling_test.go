// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Releasing a fast drag lets the view coast to a stop. Velocity is measured by
// Tick because a toolkit.Event carries no timestamp, so these tests drive the
// gesture the way a host does: drag, tick, release, tick.

// flick drags the view upward by dy pixels and ticks once, so Tick sees the
// movement and turns it into a velocity. It returns the ScrollView.
func flick(dy int, dt float64) *ScrollView {
	sv := newPanScrollView()
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 60 - dy})
	sv.Tick(dt) // measures dy/dt px/s
	sv.OnEvent(Event{Kind: EventMouseUp, X: 40, Y: 60 - dy})
	return sv
}

func TestScrollViewFlingCoasts(t *testing.T) {
	// 60 px in a 60th of a second is 3600 px/s: comfortably a flick.
	sv := flick(60, 1.0/60)
	if !sv.flinging {
		t.Fatal("a fast release should start a fling")
	}
	if !sv.Animating() {
		t.Fatal("a flinging view still needs frames")
	}
	settled := sv.OffsetY
	// The view keeps moving after the finger is gone.
	sv.Tick(1.0 / 60)
	if sv.OffsetY <= settled {
		t.Fatalf("the fling did not carry the view: OffsetY=%d, was %d", sv.OffsetY, settled)
	}
	// And it slows down: each tick moves the view less than the one before.
	prev, last := sv.OffsetY, 0
	for i := 0; i < 3; i++ {
		before := sv.OffsetY
		sv.Tick(1.0 / 60)
		step := sv.OffsetY - before
		if last != 0 && step > last {
			t.Fatalf("step %d grew to %d from %d: a fling must decelerate", i, step, last)
		}
		last, prev = step, sv.OffsetY
	}
	_ = prev
}

func TestScrollViewFlingComesToRest(t *testing.T) {
	sv := flick(60, 1.0/60)
	// A second of ticking is far longer than the decay needs.
	for i := 0; i < 120 && sv.flinging; i++ {
		sv.Tick(1.0 / 60)
	}
	if sv.flinging {
		t.Fatal("the fling never ended")
	}
	if sv.Animating() {
		t.Fatal("a view at rest should stop asking for frames")
	}
	if sv.velX != 0 || sv.velY != 0 {
		t.Fatalf("a stopped fling should keep no velocity, got (%v,%v)", sv.velX, sv.velY)
	}
}

func TestScrollViewSlowReleaseDoesNotFling(t *testing.T) {
	// A deliberate placement is not a flick: carrying it on would fight the
	// user, who put the content exactly where they wanted it.
	sv := flick(1, 1.0/60) // 60 px/s, below flingStartVelocity
	if sv.flinging {
		t.Fatal("a slow release should not fling")
	}
	at := sv.OffsetY
	sv.Tick(1.0 / 60)
	if sv.OffsetY != at {
		t.Fatalf("a view that is not flinging must not move: OffsetY=%d, want %d", sv.OffsetY, at)
	}
}

func TestScrollViewPressStopsAFling(t *testing.T) {
	sv := flick(60, 1.0/60)
	if !sv.flinging {
		t.Fatal("setup: expected a fling")
	}
	// Catching a coasting view stops it dead, the way a finger on a spinning
	// record does.
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	if sv.flinging {
		t.Fatal("a press should stop the fling")
	}
	at := sv.OffsetY
	sv.Tick(1.0 / 60)
	if sv.OffsetY != at {
		t.Fatalf("a caught view must not coast on: OffsetY=%d, want %d", sv.OffsetY, at)
	}
}

func TestScrollViewFlingStopsAtTheEnd(t *testing.T) {
	// Reaching an end ends the fling on that axis: the content cannot move,
	// so there is nothing left to carry.
	sv := flick(60, 1.0/60)
	for i := 0; i < 600 && sv.flinging; i++ {
		sv.Tick(1.0 / 60)
	}
	if want := sv.maxOffsetY(); sv.OffsetY > want {
		t.Fatalf("OffsetY=%d ran past the end %d", sv.OffsetY, want)
	}
	if sv.flinging {
		t.Fatal("the fling should have ended at the end of the content")
	}
}

func TestScrollViewTickWhileDraggingOnlyMeasures(t *testing.T) {
	sv := newPanScrollView()
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 30})
	at := sv.OffsetY
	sv.Tick(1.0 / 60)
	// The drag itself scrolls while the contact lasts; the tick only reads
	// the speed, it must not move the view a second time.
	if sv.OffsetY != at {
		t.Fatalf("Tick during a drag moved the view: OffsetY=%d, want %d", sv.OffsetY, at)
	}
	if sv.velY != 30*60 {
		t.Fatalf("velY=%v, want %v px/s", sv.velY, 30*60.0)
	}
	if !sv.Animating() {
		t.Fatal("a view under the finger needs frames, to keep measuring")
	}
	// The accumulator resets, so a still finger reads as no speed at all.
	sv.Tick(1.0 / 60)
	if sv.velY != 0 {
		t.Fatalf("a still finger should measure 0, got %v", sv.velY)
	}
}

func TestScrollViewTickIgnoresANonPositiveDelta(t *testing.T) {
	// A host that hands out a zero or negative dt (a clock that did not move,
	// or went backwards) must not divide by it.
	sv := flick(60, 1.0/60)
	at, vel := sv.OffsetY, sv.velY
	sv.Tick(0)
	sv.Tick(-1)
	if sv.OffsetY != at || sv.velY != vel {
		t.Fatalf("a non-positive dt changed the view: OffsetY=%d velY=%v", sv.OffsetY, sv.velY)
	}
}

func TestScrollViewIdleTickDoesNothing(t *testing.T) {
	sv := newPanScrollView()
	if sv.Animating() {
		t.Fatal("an untouched view should not ask for frames")
	}
	sv.Tick(1.0 / 60)
	if sv.OffsetY != 0 || sv.OffsetX != 0 {
		t.Fatal("ticking an idle view should not move it")
	}
}

func TestScrollViewMaxOffsets(t *testing.T) {
	sv := newPanScrollView()
	vp := sv.viewport()
	if got, want := sv.maxOffsetY(), 400-vp.H; got != want {
		t.Fatalf("maxOffsetY = %d, want %d", got, want)
	}
	// Content smaller than the viewport has nowhere to scroll, and a negative
	// maximum would let a fling drag the content off screen.
	sv.contentW, sv.contentH = 1, 1
	if got := sv.maxOffsetX(); got != 0 {
		t.Fatalf("maxOffsetX with tiny content = %d, want 0", got)
	}
	if got := sv.maxOffsetY(); got != 0 {
		t.Fatalf("maxOffsetY with tiny content = %d, want 0", got)
	}
}

func TestScrollViewFlingDecaysToRestAwayFromTheEnds(t *testing.T) {
	// A gentle flick — fast enough to fling, slow enough that friction alone
	// stops it well before the end of the content. It is the other way a fling
	// can end, and the only one that exercises the velocity threshold.
	sv := flick(3, 1.0/60) // 180 px/s, above the 120 px/s start threshold
	if !sv.flinging {
		t.Fatal("180 px/s should be a fling")
	}
	for i := 0; i < 600 && sv.flinging; i++ {
		sv.Tick(1.0 / 60)
	}
	if sv.flinging {
		t.Fatal("friction alone should have stopped the fling")
	}
	if sv.OffsetY <= 0 || sv.OffsetY >= sv.maxOffsetY() {
		t.Fatalf("OffsetY=%d: this test only means something away from the ends", sv.OffsetY)
	}
}
