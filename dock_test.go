// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// dockProbe records whether it received an event, so routing can be asserted.
type dockProbe struct {
	Base
	got bool
	ev  Event
}

func (d *dockProbe) OnEvent(ev Event) { d.got = true; d.ev = ev }

// TestDockLayout docks one bar on each edge (in order top, bottom, left, right)
// and checks each bar's rect and the body filling the remainder. The top/bottom
// bars are added first, so they span the full width and the left/right bars only
// get the height between them.
func TestDockLayout(t *testing.T) {
	body := &dockProbe{}
	top, bot, left, right := &dockProbe{}, &dockProbe{}, &dockProbe{}, &dockProbe{}
	d := NewDock(body)
	d.Dock(top, DockTop, 10)
	d.Dock(bot, DockBottom, 8)
	d.Dock(left, DockLeft, 12)
	d.Dock(right, DockRight, 6)
	d.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 50})

	if b := top.Bounds(); b != (Rect{X: 0, Y: 0, W: 100, H: 10}) {
		t.Fatalf("top = %+v", b)
	}
	if b := bot.Bounds(); b != (Rect{X: 0, Y: 42, W: 100, H: 8}) {
		t.Fatalf("bottom = %+v", b)
	}
	// Left/right sit in the band between the top (10) and bottom (8): Y 10..42.
	if b := left.Bounds(); b != (Rect{X: 0, Y: 10, W: 12, H: 32}) {
		t.Fatalf("left = %+v", b)
	}
	if b := right.Bounds(); b != (Rect{X: 94, Y: 10, W: 6, H: 32}) {
		t.Fatalf("right = %+v", b)
	}
	// Body is what's left: X 12..94, Y 10..42.
	if b := body.Bounds(); b != (Rect{X: 12, Y: 10, W: 82, H: 32}) {
		t.Fatalf("body = %+v", b)
	}
}

// TestDockClampAndNilBody checks an oversized bar is clamped to the available
// extent (per side) and that a nil body is a no-op.
func TestDockClampAndNilBody(t *testing.T) {
	sides := []DockSide{DockTop, DockBottom, DockLeft, DockRight}
	for _, side := range sides {
		bar := &dockProbe{}
		d := NewDock(nil) // nil body: SetBounds must not panic
		d.Dock(bar, side, 1000)
		d.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
		b := bar.Bounds()
		switch side {
		case DockTop, DockBottom:
			if b.H != 20 || b.W != 40 {
				t.Fatalf("%v clamp: %+v, want H=20 W=40", side, b)
			}
		default:
			if b.W != 40 || b.H != 20 {
				t.Fatalf("%v clamp: %+v, want W=40 H=20", side, b)
			}
		}
	}
}

// TestDockSizeClamp checks a negative dock size is clamped to zero.
func TestDockSizeClamp(t *testing.T) {
	bar := &dockProbe{}
	d := NewDock(nil)
	d.Dock(bar, DockTop, -5)
	d.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 30})
	if bar.Bounds().H != 0 {
		t.Fatalf("negative size not clamped: H=%d", bar.Bounds().H)
	}
}

// TestDockDrawAndEvents checks Draw paints without panicking and that events
// route to a bar, to the body, and to nothing when outside every child.
func TestDockDrawAndEvents(t *testing.T) {
	body := &dockProbe{}
	top := &dockProbe{}
	d := NewDock(body)
	d.Dock(top, DockTop, 10)
	d.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 50})

	d.Draw(newP(makeSurface(100, 50), 100), DefaultLight()) // no panic

	// Click in the top bar (y=5) routes to it.
	d.OnEvent(Event{Kind: EventClick, X: 20, Y: 5})
	if !top.got || body.got {
		t.Fatalf("top-bar click routing: top=%v body=%v", top.got, body.got)
	}
	// Click in the body (y=30) routes to the body.
	top.got = false
	d.OnEvent(Event{Kind: EventClick, X: 20, Y: 30})
	if !body.got || top.got {
		t.Fatalf("body click routing: top=%v body=%v", top.got, body.got)
	}
	// Click outside every child is a no-op.
	body.got, top.got = false, false
	d.OnEvent(Event{Kind: EventClick, X: 500, Y: 500})
	if body.got || top.got {
		t.Fatal("out-of-bounds click should route nowhere")
	}
}

// TestDockNilBodyEvent checks OnEvent is safe (and a no-op) with a nil body when
// the point misses every bar.
func TestDockNilBodyEvent(t *testing.T) {
	bar := &dockProbe{}
	d := NewDock(nil)
	d.Dock(bar, DockLeft, 10)
	d.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	d.OnEvent(Event{Kind: EventClick, X: 30, Y: 20}) // right of the bar, no body
	if bar.got {
		t.Fatal("click outside the only bar should not route to it")
	}
}
