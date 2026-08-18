// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// A ScrollView pans and flings on its own. A MomentumScroller wrapping one
// consumes the SAME drag, and a host has to deliver those events to the widget
// tree anyway — buttons and lists need them — so the two would apply the
// gesture twice. A driven view stands down.

func TestMomentumScrollerClaimsTheView(t *testing.T) {
	sv := newPanScrollView()
	if sv.ScrollDriven() {
		t.Fatal("an unwrapped view drives itself")
	}
	ms := NewMomentumScroller(sv)
	if sv.ScrollDriven() {
		t.Fatal("merely wrapping a view must not change it: a wrapper that is\n\t\t\tnever used leaves the view driving itself")
	}

	// The gesture, delivered to BOTH as a host would: the wrapper moves the
	// view, the view itself does not add to it.
	ms.TouchDown(Event{Kind: EventClick, X: 40, Y: 60})
	if !sv.ScrollDriven() {
		t.Fatal("the first gesture should claim the view")
	}
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	if sv.pan.active {
		t.Fatal("a driven view must not arm its own pan")
	}
	ms.TouchMove(Event{Kind: EventMouseDrag, X: 40, Y: 30}, 1.0/60)
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 30})
	if sv.OffsetY != 30 {
		t.Fatalf("OffsetY=%d after one 30px sample, want 30 — the drag applied twice", sv.OffsetY)
	}

	// And the view's own fling stays out of it: only the engine coasts.
	ms.TouchUp()
	sv.OnEvent(Event{Kind: EventMouseUp, X: 40, Y: 30})
	if sv.Animating() {
		t.Fatal("a driven view must not fling on its own")
	}
	at := sv.OffsetY
	sv.Tick(1.0 / 60)
	if sv.OffsetY != at {
		t.Fatalf("Tick moved a driven view: OffsetY=%d, want %d", sv.OffsetY, at)
	}
}

func TestScrollDriverCanBeHandedBack(t *testing.T) {
	sv := newPanScrollView()
	NewMomentumScroller(sv).TouchDown(Event{X: 40, Y: 60})
	sv.SetScrollDriver(nil)
	if sv.ScrollDriven() {
		t.Fatal("a nil driver hands the view back to itself")
	}
	// It pans on its own again.
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 40})
	if sv.OffsetY != 20 {
		t.Fatalf("OffsetY=%d, want 20: an undriven view pans itself", sv.OffsetY)
	}
}

func TestSetScrollDriverStopsWhatIsInFlight(t *testing.T) {
	// Handing a coasting, mid-drag view to a driver must leave nothing of the
	// old motion behind for the driver to fight.
	sv := flick(60, 1.0/60)
	if !sv.Animating() {
		t.Fatal("setup: expected a fling")
	}
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60}) // re-arm a pan
	sv.SetScrollDriver(NewMomentum())
	if sv.Animating() || sv.pan.active {
		t.Fatal("claiming a view should stop its coast and release its pan")
	}
}
