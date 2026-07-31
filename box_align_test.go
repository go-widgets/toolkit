// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// alignProbe is a minimal widget with no Measurer, so box alignment falls back to
// its current cross Bounds.
type alignProbe struct{ Base }

// measProbe advertises a natural size via Measurer.
type measProbe struct {
	Base
	mw, mh int
}

func (m *measProbe) Measure(_, _ int) (int, int) { return m.mw, m.mh }

// TestBoxAlignHBox checks cross-axis (vertical) alignment for HBox: the default
// stretch fills the height, and start/center/end pin a child at its natural
// height (from a Measurer), while a zero or oversized natural height falls back
// to stretch.
func TestBoxAlignHBox(t *testing.T) {
	r := Rect{X: 5, Y: 7, W: 100, H: 40}

	// Default (BoxStretch): child fills the full height.
	h := NewHBox()
	stretch := &measProbe{mw: 30, mh: 10}
	h.AddFixed(stretch, 30)
	h.SetBounds(r)
	if b := stretch.Bounds(); b.Y != 7 || b.H != 40 {
		t.Fatalf("stretch: Y=%d H=%d, want Y=7 H=40", b.Y, b.H)
	}

	// Center: natural height 10 centred in 40 → Y = 7 + (40-10)/2 = 22.
	h = NewHBox()
	h.Align = BoxAlignCenter
	c := &measProbe{mw: 30, mh: 10}
	h.AddFixed(c, 30)
	h.SetBounds(r)
	if b := c.Bounds(); b.Y != 22 || b.H != 10 {
		t.Fatalf("center: Y=%d H=%d, want Y=22 H=10", b.Y, b.H)
	}

	// End: pinned to the bottom → Y = 7 + (40-10) = 37.
	h = NewHBox()
	h.Align = BoxAlignEnd
	e := &measProbe{mw: 30, mh: 10}
	h.AddFixed(e, 30)
	h.SetBounds(r)
	if b := e.Bounds(); b.Y != 37 || b.H != 10 {
		t.Fatalf("end: Y=%d H=%d, want Y=37 H=10", b.Y, b.H)
	}

	// Start via a non-Measurer child: natural height comes from its Bounds, set
	// after AddFixed (which zeroes it) but before the final layout.
	h = NewHBox()
	h.Align = BoxAlignStart
	s := &alignProbe{}
	h.AddFixed(s, 30)
	s.SetBounds(Rect{H: 12})
	h.SetBounds(r)
	if b := s.Bounds(); b.Y != 7 || b.H != 12 {
		t.Fatalf("start(non-measurer): Y=%d H=%d, want Y=7 H=12", b.Y, b.H)
	}

	// A zero natural height falls back to stretch.
	h = NewHBox()
	h.Align = BoxAlignCenter
	z := &measProbe{mw: 30, mh: 0}
	h.AddFixed(z, 30)
	h.SetBounds(r)
	if b := z.Bounds(); b.H != 40 {
		t.Fatalf("zero natural: H=%d, want stretch 40", b.H)
	}

	// A natural height >= the band also stretches.
	h = NewHBox()
	h.Align = BoxAlignEnd
	big := &measProbe{mw: 30, mh: 99}
	h.AddFixed(big, 30)
	h.SetBounds(r)
	if b := big.Bounds(); b.Y != 7 || b.H != 40 {
		t.Fatalf("oversized natural: Y=%d H=%d, want stretch Y=7 H=40", b.Y, b.H)
	}
}

// TestBoxAlignVBox checks cross-axis (horizontal) alignment for VBox, exercising
// the vertical branch of naturalCross for both a Measurer and a plain widget.
func TestBoxAlignVBox(t *testing.T) {
	r := Rect{X: 4, Y: 6, W: 50, H: 80}

	// Center via Measurer natural width 20 → X = 4 + (50-20)/2 = 19.
	v := NewVBox()
	v.Align = BoxAlignCenter
	c := &measProbe{mw: 20, mh: 30}
	v.AddFixed(c, 30)
	v.SetBounds(r)
	if b := c.Bounds(); b.X != 19 || b.W != 20 {
		t.Fatalf("vbox center: X=%d W=%d, want X=19 W=20", b.X, b.W)
	}

	// Start via a non-Measurer child (natural width from Bounds).
	v = NewVBox()
	v.Align = BoxAlignStart
	s := &alignProbe{}
	v.AddFixed(s, 30)
	s.SetBounds(Rect{W: 15})
	v.SetBounds(r)
	if b := s.Bounds(); b.X != 4 || b.W != 15 {
		t.Fatalf("vbox start(non-measurer): X=%d W=%d, want X=4 W=15", b.X, b.W)
	}
}

// TestBoxPackHBox checks main-axis packing of slack when no flex child absorbs it.
func TestBoxPackHBox(t *testing.T) {
	// Two fixed 20px children, spacing 10 → used 50, box 100 → slack 50.
	build := func(pack BoxPack) (a, b *alignProbe) {
		h := NewHBox()
		h.Spacing = 10
		h.Pack = pack
		a, b = &alignProbe{}, &alignProbe{}
		h.AddFixed(a, 20)
		h.AddFixed(b, 20)
		h.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 10})
		return a, b
	}

	if a, _ := build(PackStart); a.Bounds().X != 0 {
		t.Fatalf("pack start: first X=%d, want 0", a.Bounds().X)
	}
	if a, _ := build(PackCenter); a.Bounds().X != 25 { // slack/2
		t.Fatalf("pack center: first X=%d, want 25", a.Bounds().X)
	}
	if a, b := build(PackEnd); a.Bounds().X != 50 || b.Bounds().X != 80 {
		t.Fatalf("pack end: X=%d,%d, want 50,80", a.Bounds().X, b.Bounds().X)
	}

	// Overflow: fixed children wider than the box → no slack, flush start.
	h := NewHBox()
	h.Spacing = 10
	h.Pack = PackEnd
	a := &alignProbe{}
	h.AddFixed(a, 200)
	h.SetBounds(Rect{X: 3, Y: 0, W: 100, H: 10})
	if a.Bounds().X != 3 {
		t.Fatalf("overflow pack: X=%d, want flush 3", a.Bounds().X)
	}
}

// TestBoxPackVBox exercises the vertical pack path.
func TestBoxPackVBox(t *testing.T) {
	v := NewVBox()
	v.Spacing = 10
	v.Pack = PackCenter
	a := &alignProbe{}
	v.AddFixed(a, 20)
	v.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 100}) // slack 80 → lead 40
	if a.Bounds().Y != 40 {
		t.Fatalf("vbox pack center: Y=%d, want 40", a.Bounds().Y)
	}
}
