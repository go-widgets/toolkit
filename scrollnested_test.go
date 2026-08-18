// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// A ScrollView is almost never the root. Every other scroll test builds one at
// the surface origin, where widget-local and absolute coordinates coincide and
// a confusion between them cannot show — so these put one INSIDE a box, offset
// from the origin, which is where a real one lives.

// nestedScrollView returns a scroll view placed low inside a tall box, and the
// box, so a test can drive the gesture the way a host does: at the root.
func nestedScrollView(t *testing.T) (*VBox, *ScrollView) {
	t.Helper()
	sv := NewScrollView(NewLabel("content"))
	box := NewVBox()
	box.Append(NewLabel("above"))
	box.Append(sv)
	box.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 800})
	sv.contentW, sv.contentH = 1000, 4000

	if b := sv.Bounds(); b.Y == 0 {
		t.Fatalf("the scroll view is at the origin (%+v): this test needs it offset", b)
	}
	return box, sv
}

func TestNestedScrollViewPans(t *testing.T) {
	box, sv := nestedScrollView(t)
	b := sv.Bounds()
	// A press in the middle of the scroll view, in SURFACE coordinates, which
	// is what a host delivers to the root.
	midY := b.Y + b.H/2
	box.OnEvent(Event{Kind: EventClick, X: 100, Y: midY})
	box.OnEvent(Event{Kind: EventMouseDrag, X: 100, Y: midY - 40})
	if sv.OffsetY != 40 {
		t.Fatalf("OffsetY=%d after a 40 px drag, want 40 — a nested view must pan too", sv.OffsetY)
	}
	// And it still coasts after the release.
	sv.Tick(1.0 / 60)
	box.OnEvent(Event{Kind: EventMouseUp, X: 100, Y: midY - 40})
	if !sv.Animating() {
		t.Fatal("a nested view should coast after a released drag")
	}
}

func TestNestedScrollViewIgnoresAPressOutsideIt(t *testing.T) {
	// The press must be tested against the view's own area, not merely
	// accepted because the coordinates happen to be small.
	box, sv := nestedScrollView(t)
	box.OnEvent(Event{Kind: EventClick, X: 100, Y: 10}) // in the label above
	box.OnEvent(Event{Kind: EventMouseDrag, X: 100, Y: 0})
	if sv.OffsetY != 0 {
		t.Fatalf("OffsetY=%d: a press outside the scroll view must not pan it", sv.OffsetY)
	}
}
