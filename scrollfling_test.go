// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Touch scrolling: a drag moves the content, a release coasts, an overscroll
// springs home. The tests assert BEHAVIOUR — offsets, overscroll, whether the
// view still wants frames — never the engine's internals, so retuning the
// physics does not rewrite them.

// flick drags the view upward by dy pixels over one frame and releases.
func flick(dy int, dt float64) *ScrollView {
	sv := newPanScrollView()
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 60 - dy})
	sv.Tick(dt) // measures the speed of that sample
	sv.OnEvent(Event{Kind: EventMouseUp, X: 40, Y: 60 - dy})
	return sv
}

// settle ticks until the view stops asking for frames, or gives up.
func settleScroll(t *testing.T, sv *ScrollView) {
	t.Helper()
	for i := 0; i < 2000 && sv.Animating(); i++ {
		sv.Tick(1.0 / 60)
	}
	if sv.Animating() {
		t.Fatal("the view never came to rest")
	}
}

func TestTouchScrollCoastsAndDecelerates(t *testing.T) {
	sv := flick(60, 1.0/60)
	if !sv.Animating() {
		t.Fatal("a fast release should leave the view coasting")
	}
	at := sv.OffsetY
	sv.Tick(1.0 / 60)
	if sv.OffsetY <= at {
		t.Fatalf("the coast did not carry the view: OffsetY=%d, was %d", sv.OffsetY, at)
	}
	last := 0
	for i := 0; i < 4; i++ {
		before := sv.OffsetY
		sv.Tick(1.0 / 60)
		step := sv.OffsetY - before
		if last != 0 && step > last {
			t.Fatalf("step %d grew to %d from %d: a coast must decelerate", i, step, last)
		}
		last = step
	}
}

func TestTouchScrollComesToRest(t *testing.T) {
	sv := flick(60, 1.0/60)
	settleScroll(t, sv)
	if x, y := sv.Overscroll(); x != 0 || y != 0 {
		t.Fatalf("a view at rest still shows overscroll (%d,%d)", x, y)
	}
}

func TestPressCatchesACoastingView(t *testing.T) {
	sv := flick(60, 1.0/60)
	if !sv.Animating() {
		t.Fatal("setup: expected a coast")
	}
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	at := sv.OffsetY
	sv.OnEvent(Event{Kind: EventMouseUp, X: 40, Y: 60})
	sv.Tick(1.0 / 60)
	if sv.OffsetY != at {
		t.Fatalf("a caught view moved on: OffsetY=%d, want %d", sv.OffsetY, at)
	}
}

func TestOffsetNeverLeavesItsBounds(t *testing.T) {
	// The contract this design exists to keep: OffsetX/OffsetY are exported,
	// callers convert screen points with them and the scrollbar computes the
	// thumb from them, so they stay clamped even while the rubber band is
	// stretched. The excursion lives in Overscroll instead.
	sv := newPanScrollView()
	max := sv.contentH - sv.viewport().H
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	// Drag far past the start, then far past the end.
	for _, y := range []int{4000, -8000} {
		sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: y})
		if sv.OffsetY < 0 || sv.OffsetY > max {
			t.Fatalf("OffsetY=%d escaped [0,%d]", sv.OffsetY, max)
		}
	}
	if _, over := sv.Overscroll(); over == 0 {
		t.Fatal("dragging well past the end should show a rubber band")
	}
	sv.OnEvent(Event{Kind: EventMouseUp, X: 40, Y: -8000})
	settleScroll(t, sv)
	if sv.OffsetY != max {
		t.Fatalf("after springing home OffsetY=%d, want the bound %d", sv.OffsetY, max)
	}
	if _, over := sv.Overscroll(); over != 0 {
		t.Fatalf("overscroll=%d after settling, want 0", over)
	}
}

func TestTickDuringADragOnlyMeasures(t *testing.T) {
	sv := newPanScrollView()
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 30})
	at := sv.OffsetY
	sv.Tick(1.0 / 60)
	if sv.OffsetY != at {
		t.Fatalf("Tick during a drag moved the view: OffsetY=%d, want %d", sv.OffsetY, at)
	}
	if !sv.Animating() {
		t.Fatal("a view under the finger needs frames, to keep measuring")
	}
}

func TestTickIgnoresANonPositiveDelta(t *testing.T) {
	sv := flick(60, 1.0/60)
	at := sv.OffsetY
	sv.Tick(0)
	sv.Tick(-1)
	if sv.OffsetY != at {
		t.Fatalf("a non-positive dt moved the view: OffsetY=%d, want %d", sv.OffsetY, at)
	}
}

func TestIdleViewAsksForNothing(t *testing.T) {
	sv := newPanScrollView()
	if sv.Animating() {
		t.Fatal("an untouched view should not ask for frames")
	}
	sv.Tick(1.0 / 60)
	if sv.OffsetY != 0 || sv.OffsetX != 0 {
		t.Fatal("ticking an idle view should not move it")
	}
	if x, y := sv.Overscroll(); x != 0 || y != 0 {
		t.Fatalf("an untouched view shows overscroll (%d,%d)", x, y)
	}
}
