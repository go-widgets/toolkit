// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestFrameTitledInsetsChildBelowBar: a Title turns the frame into a titled
// panel, insetting the child below the FrameTitleH-tall bar (untitled frames
// are unaffected -- covered by the existing Frame tests).
func TestFrameTitledInsetsChildBelowBar(t *testing.T) {
	child := &spyWidget{}
	f := NewFrame(child)
	f.Title = "Panel"
	f.Padding = 4 // inset = 5
	f.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	// top = Y + 1 + FrameTitleH + pad = 0 + 1 + 22 + 4 = 27; bottom = 100-5 = 95.
	want := Rect{X: 5, Y: 1 + FrameTitleH + 4, W: 90, H: 95 - (1 + FrameTitleH + 4)}
	if child.Bounds() != want {
		t.Fatalf("titled child Bounds = %+v, want %+v", child.Bounds(), want)
	}
	if f.headerH() != FrameTitleH {
		t.Fatalf("headerH=%d, want %d for a titled frame", f.headerH(), FrameTitleH)
	}
}

// TestFrameCollapsibleTogglesOnHeaderClick: a header click flips the Collapsed
// Observable and notifies its subscribers; while collapsed the child is hidden
// (empty bounds) and not drawn, and only the title bar paints.
func TestFrameCollapsibleTogglesOnHeaderClick(t *testing.T) {
	const w, h = 120, 120
	child := &spyWidget{}
	f := NewFrame(child)
	f.Title = "Advanced"
	f.Collapsible = true
	var got []bool
	f.Collapsed().Subscribe(func(c bool) { got = append(got, c) })
	f.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})

	// Click the title bar → collapse.
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if !f.Collapsed().Get() {
		t.Fatal("header click should collapse the frame")
	}
	f.SetBounds(Rect{X: 0, Y: 0, W: w, H: h}) // relayout while collapsed
	if child.Bounds() != (Rect{}) {
		t.Fatalf("collapsed child should be hidden, bounds = %+v", child.Bounds())
	}
	buf := makeSurface(w, h)
	f.Draw(newP(buf, w), DefaultLight())
	if child.drawCount != 0 {
		t.Fatalf("collapsed child must not draw, count = %d", child.drawCount)
	}
	// Nothing painted well below the collapsed header (only the bar shows).
	if anyPainted(buf, w, 1, FrameTitleH+8, w-1, h-1) {
		t.Fatal("collapsed frame painted below its title bar")
	}

	// Click again → expand; child draws and lays out again.
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if f.Collapsed().Get() {
		t.Fatal("second header click should expand the frame")
	}
	f.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf2 := makeSurface(w, h)
	f.Draw(newP(buf2, w), DefaultLight())
	if child.drawCount != 1 {
		t.Fatalf("expanded child should draw once, count = %d", child.drawCount)
	}
	if len(got) != 2 || got[0] != true || got[1] != false {
		t.Fatalf("Collapsed subscriber calls = %v, want [true false]", got)
	}
}

// TestFrameCollapsedAccessorLazyInit: a bare &Frame{} (no constructor) still
// yields a usable Collapsed Observable — the accessor lazily initialises it to
// false rather than panicking on a nil handle.
func TestFrameCollapsedAccessorLazyInit(t *testing.T) {
	f := &Frame{}
	if f.Collapsed().Get() {
		t.Fatalf("zero-value Frame Collapsed = true, want false")
	}
}

// TestFrameCollapsedHostBinding: a host binds the Collapsed Observable
// (Subscribe + Set) and drives the collapse state through it, with no imperative
// state field in sight.
func TestFrameCollapsedHostBinding(t *testing.T) {
	f := NewFrame(&spyWidget{})
	f.Collapsible = true
	f.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 80})
	var seen bool
	f.Collapsed().Subscribe(func(c bool) { seen = c })
	f.Collapsed().Set(true)
	if !seen {
		t.Fatal("host Subscribe not notified on Set(true)")
	}
	if !f.Collapsed().Get() {
		t.Fatal("Collapsed().Get() should reflect the host Set(true)")
	}
}

// TestFrameCollapsibleForcesBarWithoutTitle: Collapsible alone (empty Title)
// still produces a title bar and a working toggle.
func TestFrameCollapsibleForcesBarWithoutTitle(t *testing.T) {
	f := NewFrame(&spyWidget{})
	f.Collapsible = true // no Title
	if f.headerH() != FrameTitleH {
		t.Fatalf("headerH=%d, want %d (Collapsible forces a bar)", f.headerH(), FrameTitleH)
	}
	f.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 80})
	buf := makeSurface(80, 80)
	f.Draw(newP(buf, 80), DefaultLight()) // draws chevron + (empty) title
	f.OnEvent(Event{Kind: EventClick, X: 5, Y: 3})
	if !f.Collapsed().Get() {
		t.Fatal("Collapsible frame without a title must still toggle")
	}
}

// TestFrameTitleBarClickNotCollapsibleForwardsNothing: a title-bar area click
// on a titled-but-not-collapsible frame is not a toggle; a body click still
// reaches the child.
func TestFrameTitleBarClickNotCollapsibleForwardsNothing(t *testing.T) {
	child := &spyWidget{}
	f := NewFrame(child)
	f.Title = "Static" // not Collapsible
	f.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 5}) // in the title bar
	if f.Collapsed().Get() {
		t.Fatal("a non-collapsible titled frame must not collapse")
	}
	// A click in the body (below the bar, inside the child) reaches the child.
	cb := child.Bounds()
	f.OnEvent(Event{Kind: EventClick, X: cb.X + 2, Y: cb.Y + 2})
	if child.evCount != 1 {
		t.Fatalf("body click should reach child once, count = %d", child.evCount)
	}
}
