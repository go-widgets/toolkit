// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Dragging the CONTENT of a ScrollView scrolls it. Before this a scrollable
// view could only be moved by a wheel, an arrow key, or a scrollbar thumb a
// few pixels wide — none of which a touch screen has, so a ScrollView on a
// phone could not be scrolled by any means at all.

// newPanScrollView returns a ScrollView with content taller and wider than its
// viewport, so both axes have somewhere to scroll to.
func newPanScrollView() *ScrollView {
	inner := NewLabel("content")
	sv := NewScrollView(inner)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	sv.contentW, sv.contentH = 400, 400
	return sv
}

func TestScrollViewContentPan(t *testing.T) {
	sv := newPanScrollView()

	// Press on the content, then drag UP: the content follows the finger, so
	// the view scrolls DOWN — a rising offset.
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 40})
	if sv.OffsetY().Get() != 20 {
		t.Fatalf("drag up 20: OffsetY=%d, want 20", sv.OffsetY().Get())
	}
	// Each sample is relative to the previous one, not to where the press was.
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 25})
	if sv.OffsetY().Get() != 35 {
		t.Fatalf("drag up 15 more: OffsetY=%d, want 35", sv.OffsetY().Get())
	}
	// Horizontal drags pan too.
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 10, Y: 25})
	if sv.OffsetX().Get() != 30 {
		t.Fatalf("drag left 30: OffsetX=%d, want 30", sv.OffsetX().Get())
	}
	// Dragging back down returns the view.
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 10, Y: 60})
	if sv.OffsetY().Get() != 0 {
		t.Fatalf("drag back down: OffsetY=%d, want 0", sv.OffsetY().Get())
	}
}

func TestScrollViewPanClampsAtTheEnds(t *testing.T) {
	sv := newPanScrollView()
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	// A drag far past the end pins to the end rather than running off.
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: -10000})
	vp := sv.viewport()
	if want := 400 - vp.H; sv.OffsetY().Get() != want {
		t.Fatalf("drag past the end: OffsetY=%d, want %d", sv.OffsetY().Get(), want)
	}
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 10000})
	if sv.OffsetY().Get() != 0 {
		t.Fatalf("drag past the start: OffsetY=%d, want 0", sv.OffsetY().Get())
	}
}

func TestScrollViewPanEndsOnRelease(t *testing.T) {
	sv := newPanScrollView()
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 40})
	sv.OnEvent(Event{Kind: EventMouseUp, X: 40, Y: 40})
	before := sv.OffsetY().Get()
	// A drag with no press behind it is a stray move, not a pan: a pointer
	// crossing the widget with no button down must not scroll it.
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 5})
	if sv.OffsetY().Get() != before {
		t.Fatalf("drag after release: OffsetY=%d, want it unchanged at %d", sv.OffsetY().Get(), before)
	}
}

func TestScrollViewPanIgnoresAPressOnTheScrollbar(t *testing.T) {
	// A press that a scrollbar took must not ALSO arm a content pan, or the
	// one gesture would move the view twice — once by the thumb and once by
	// the finger.
	sv := newPanScrollView()
	g, ok := sv.vscrollGeom()
	if !ok {
		t.Fatal("test needs a visible vertical scrollbar")
	}
	// Grab the thumb, then drag: the thumb moves the view, the pan does not.
	sv.OnEvent(Event{Kind: EventClick, X: g.cross0 + 1, Y: g.thumbStart + 1})
	if !sv.sbV.active {
		t.Fatal("the press should have grabbed the thumb")
	}
	if sv.pan.active {
		t.Fatal("a press on the scrollbar should not arm a content pan")
	}
	sv.OnEvent(Event{Kind: EventMouseDrag, X: g.cross0 + 1, Y: g.thumbStart + 21})
	moved := sv.OffsetY().Get()
	if moved == 0 {
		t.Fatal("dragging the thumb should have scrolled the view")
	}

	// And the reverse, which is the property that matters: with a pan ALSO
	// armed, a grabbed thumb still moves the view by exactly as much as it
	// does alone -- the pan adds nothing on top.
	sv2 := newPanScrollView()
	sv2.OnEvent(Event{Kind: EventClick, X: 40, Y: 60}) // arms a content pan
	if !sv2.pan.active {
		t.Fatal("a press on the content should arm a pan")
	}
	sv2.sbV = sv.sbV // same grabbed thumb, same grab offset
	sv2.OnEvent(Event{Kind: EventMouseDrag, X: g.cross0 + 1, Y: g.thumbStart + 21})
	if sv2.OffsetY().Get() != moved {
		t.Fatalf("thumb drag with a pan armed: OffsetY=%d, want %d -- the pan added to it", sv2.OffsetY().Get(), moved)
	}
}

func TestContentPanDelta(t *testing.T) {
	var p contentPan
	p.start(Event{X: 10, Y: 20})
	if !p.active {
		t.Fatal("start should arm the pan")
	}
	// The delta is the movement of the content inverted: what to ADD to the
	// scroll offset.
	if dx, dy := p.delta(Event{X: 4, Y: 5}); dx != 6 || dy != 15 {
		t.Fatalf("delta = (%d,%d), want (6,15)", dx, dy)
	}
	// It is relative to the previous sample, so a still finger moves nothing.
	if dx, dy := p.delta(Event{X: 4, Y: 5}); dx != 0 || dy != 0 {
		t.Fatalf("delta of a still pointer = (%d,%d), want (0,0)", dx, dy)
	}
	p.release()
	if p.active {
		t.Fatal("release should disarm the pan")
	}
}
