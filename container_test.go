// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestItemBoxSpec covers the three Flex/Size resolutions.
func TestItemBoxSpec(t *testing.T) {
	if f, s := (Item{Flex: 3}).boxSpec(); f != 3 || s != 0 {
		t.Fatalf("flex item: %d,%d", f, s)
	}
	if f, s := (Item{Size: 40}).boxSpec(); f != 0 || s != 40 {
		t.Fatalf("size item: %d,%d", f, s)
	}
	if f, s := (Item{}).boxSpec(); f != 1 || s != 0 {
		t.Fatalf("default item: %d,%d", f, s)
	}
}

// TestFitLayout fills every item to the container.
func TestFitLayout(t *testing.T) {
	a, b := &dockProbe{}, &dockProbe{}
	c := NewContainer(FitLayout{})
	c.AddWidget(a).AddWidget(b)
	c.SetBounds(Rect{X: 2, Y: 3, W: 20, H: 10})
	want := Rect{X: 2, Y: 3, W: 20, H: 10}
	if a.Bounds() != want || b.Bounds() != want {
		t.Fatalf("fit: a=%+v b=%+v want %+v", a.Bounds(), b.Bounds(), want)
	}
}

// TestBoxLayoutHorizontal exercises fixed+flex sizing, pack and cross-align on the
// horizontal axis.
func TestBoxLayoutHorizontal(t *testing.T) {
	fixed, flex := &dockProbe{}, &dockProbe{}
	c := NewContainer(&BoxLayout{Spacing: 10})
	c.Add(Item{Widget: fixed, Size: 20}).Add(Item{Widget: flex, Flex: 1})
	c.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 30})
	// fixed 20 at x0, gap 10, flex fills the rest = 100-20-10 = 70.
	if b := fixed.Bounds(); b != (Rect{X: 0, Y: 0, W: 20, H: 30}) {
		t.Fatalf("fixed = %+v", b)
	}
	if b := flex.Bounds(); b != (Rect{X: 30, Y: 0, W: 70, H: 30}) {
		t.Fatalf("flex = %+v", b)
	}
}

// TestBoxLayoutVerticalAlignPack exercises the vertical axis with a centred cross
// alignment and end packing.
func TestBoxLayoutVerticalAlignPack(t *testing.T) {
	m := &measProbe{mw: 20, mh: 10}
	c := NewContainer(&BoxLayout{Vertical: true, Spacing: 4, Align: BoxAlignCenter, Pack: PackEnd})
	c.Add(Item{Widget: m, Size: 10})
	c.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 60})
	// One fixed 10-tall item, slack 50 → PackEnd lead 50 → Y=50. Cross width 20
	// centred in 50 → X=15.
	if b := m.Bounds(); b != (Rect{X: 15, Y: 50, W: 20, H: 10}) {
		t.Fatalf("vbox align/pack = %+v", b)
	}
}

// TestBoxLayoutEmpty covers the no-items early return.
func TestBoxLayoutEmpty(t *testing.T) {
	c := NewContainer(&BoxLayout{})
	c.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10}) // must not panic with no items
}

// TestBorderLayout arranges items by region and fills the centre.
func TestContainerBorderLayout(t *testing.T) {
	n, s, w, e, ctr := &dockProbe{}, &dockProbe{}, &dockProbe{}, &dockProbe{}, &dockProbe{}
	c := NewContainer(BorderLayout{})
	c.Add(Item{Widget: n, Region: RegionNorth, Size: 10})
	c.Add(Item{Widget: s, Region: RegionSouth, Size: 8})
	c.Add(Item{Widget: w, Region: RegionWest, Size: 12})
	c.Add(Item{Widget: e, Region: RegionEast, Size: 6})
	c.Add(Item{Widget: ctr, Region: RegionCenter})
	c.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 50})

	if b := n.Bounds(); b != (Rect{X: 0, Y: 0, W: 100, H: 10}) {
		t.Fatalf("north = %+v", b)
	}
	if b := s.Bounds(); b != (Rect{X: 0, Y: 42, W: 100, H: 8}) {
		t.Fatalf("south = %+v", b)
	}
	if b := w.Bounds(); b != (Rect{X: 0, Y: 10, W: 12, H: 32}) {
		t.Fatalf("west = %+v", b)
	}
	if b := e.Bounds(); b != (Rect{X: 94, Y: 10, W: 6, H: 32}) {
		t.Fatalf("east = %+v", b)
	}
	if b := ctr.Bounds(); b != (Rect{X: 12, Y: 10, W: 82, H: 32}) {
		t.Fatalf("center = %+v", b)
	}
}

// TestBorderLayoutNegativeSize covers the negative-size clamp and a region with no
// matching item (the continue branch).
func TestContainerBorderNegativeSize(t *testing.T) {
	n := &dockProbe{}
	c := NewContainer(BorderLayout{})
	c.Add(Item{Widget: n, Region: RegionNorth, Size: -5}) // clamps to 0; no other regions
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	if n.Bounds().H != 0 {
		t.Fatalf("negative size not clamped: %+v", n.Bounds())
	}
}

// TestRegionSide covers every Region→DockSide mapping.
func TestRegionSide(t *testing.T) {
	cases := map[Region]DockSide{
		RegionNorth: DockTop, RegionSouth: DockBottom,
		RegionWest: DockLeft, RegionEast: DockRight, RegionCenter: DockTop,
	}
	for r, want := range cases {
		if got := regionSide(r); got != want {
			t.Fatalf("regionSide(%d) = %v, want %v", r, got, want)
		}
	}
}

// TestCardLayout shows the active card and empties the rest; switching Active and
// re-laying out swaps which is visible.
func TestCardLayout(t *testing.T) {
	a, b := &dockProbe{}, &dockProbe{}
	card := &CardLayout{Active: 0}
	c := NewContainer(card)
	c.AddWidget(a).AddWidget(b)
	c.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 20})
	if a.Bounds() != (Rect{X: 0, Y: 0, W: 30, H: 20}) || b.Bounds() != (Rect{}) {
		t.Fatalf("card active 0: a=%+v b=%+v", a.Bounds(), b.Bounds())
	}
	card.Active = 1
	c.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 20})
	if b.Bounds() != (Rect{X: 0, Y: 0, W: 30, H: 20}) || a.Bounds() != (Rect{}) {
		t.Fatalf("card active 1: a=%+v b=%+v", a.Bounds(), b.Bounds())
	}
}

// TestContainerDrawEventsAndNilLayout covers Draw skipping empty items, event
// routing to a visible item / skipping empty ones / missing all, Items(), and a
// nil-layout container.
func TestContainerDrawEventsAndNilLayout(t *testing.T) {
	// nil layout: relayout is a no-op, items keep whatever bounds they are given.
	nilC := NewContainer(nil)
	nilC.AddWidget(&dockProbe{})
	nilC.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10}) // must not panic

	shown, hidden := &dockProbe{}, &dockProbe{}
	c := NewContainer(&CardLayout{Active: 0})
	c.AddWidget(shown).AddWidget(hidden)
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})

	if len(c.Items()) != 2 {
		t.Fatalf("Items() len = %d", len(c.Items()))
	}

	c.Draw(newP(makeSurface(40, 20), 40), DefaultLight()) // hidden (empty) is skipped

	// Click on the shown card routes to it; the hidden card is skipped.
	c.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if !shown.got || hidden.got {
		t.Fatalf("routing: shown=%v hidden=%v", shown.got, hidden.got)
	}
	// Click outside every visible item is a no-op.
	shown.got = false
	c.OnEvent(Event{Kind: EventClick, X: 500, Y: 500})
	if shown.got {
		t.Fatal("out-of-bounds click should route nowhere")
	}
}
