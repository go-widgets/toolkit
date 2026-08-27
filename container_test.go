// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestItemBoxSpec covers the three Flex/Size/natural resolutions.
//
// The third one changed: an item with no configuration used to mean an equal
// flex share and now means the widget's natural size. That default matters more
// than it looks -- it is what the tempting way to add a widget does, and while
// it was "share the space equally" a caller who wanted a sensible column had to
// compute one in pixels.
func TestItemBoxSpec(t *testing.T) {
	if f, s, n := (Item{Flex: 3}).boxSpec(); f != 3 || s != 0 || n {
		t.Fatalf("flex item: %d,%d,%v", f, s, n)
	}
	if f, s, n := (Item{Size: 40}).boxSpec(); f != 0 || s != 40 || n {
		t.Fatalf("size item: %d,%d,%v", f, s, n)
	}
	if f, s, n := (Item{}).boxSpec(); f != 0 || s != 0 || !n {
		t.Fatalf("default item: %d,%d,%v", f, s, n)
	}
	// Natural spelled out is the same thing, and an explicit Size or Flex still
	// wins over it: they are answers to the same question.
	if f, s, n := (Item{Natural: true}).boxSpec(); f != 0 || s != 0 || !n {
		t.Fatalf("natural item: %d,%d,%v", f, s, n)
	}
	if f, s, n := (Item{Natural: true, Size: 12}).boxSpec(); f != 0 || s != 12 || n {
		t.Fatalf("natural with a size: %d,%d,%v", f, s, n)
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

// TestNewBoxLayoutAndZeroValueSpacing covers the new spacing semantics for the
// container BoxLayout: NewBoxLayout seeds DefaultBoxSpacing while the zero-value
// BoxLayout{} keeps a literal flush (0-gap) axis.
func TestNewBoxLayoutAndZeroValueSpacing(t *testing.T) {
	if NewBoxLayout().Spacing != DefaultBoxSpacing {
		t.Fatalf("NewBoxLayout Spacing = %d, want %d", NewBoxLayout().Spacing, DefaultBoxSpacing)
	}
	// Zero-value BoxLayout{} → flush: two equal children each 50 of 100, no gap.
	a, b := &dockProbe{}, &dockProbe{}
	c := NewContainer(&BoxLayout{})
	c.AddWidget(a).AddWidget(b)
	c.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 10})
	if a.Bounds() != (Rect{X: 0, Y: 0, W: 50, H: 10}) || b.Bounds() != (Rect{X: 50, Y: 0, W: 50, H: 10}) {
		t.Fatalf("zero-value BoxLayout flush: a=%+v b=%+v", a.Bounds(), b.Bounds())
	}
}

// TestBoxLayoutEmptyRectCollapses asserts a box layout handed an empty rect
// collapses every item to Rect{}.
func TestBoxLayoutEmptyRectCollapses(t *testing.T) {
	a, b := &dockProbe{}, &dockProbe{}
	a.SetBounds(Rect{X: 1, Y: 1, W: 9, H: 9})
	b.SetBounds(Rect{X: 2, Y: 2, W: 9, H: 9})
	(&BoxLayout{}).Arrange(Rect{X: 3, Y: 3, W: 0, H: 5}, []Item{{Widget: a}, {Widget: b}})
	if a.Bounds() != (Rect{}) || b.Bounds() != (Rect{}) {
		t.Fatalf("box layout empty collapse: a=%+v b=%+v", a.Bounds(), b.Bounds())
	}
}

// TestCardLayoutCollapsesLeafDescendants is the leaf-collapse invariant: after a
// CardLayout activates card B, card A (a VBox) and its leaf children must all
// report empty Bounds — no stale non-empty leaf survives the collapse.
func TestCardLayoutCollapsesLeafDescendants(t *testing.T) {
	leafA, leafB := &dockProbe{}, &dockProbe{}
	cardA, cardB := NewVBox(), NewVBox()
	cardA.Append(leafA)
	cardB.Append(leafB)
	card := &CardLayout{Active: 0}
	c := NewContainer(card)
	c.AddWidget(cardA).AddWidget(cardB)
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	// Card A active: its leaf fills; B's leaf is collapsed.
	if leafA.Bounds() != (Rect{X: 0, Y: 0, W: 40, H: 40}) {
		t.Fatalf("card A leaf should fill: %+v", leafA.Bounds())
	}
	// Switch to card B and re-lay out.
	card.Active = 1
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	if leafA.Bounds() != (Rect{}) {
		t.Fatalf("card A leaf must collapse to empty, got %+v", leafA.Bounds())
	}
	if leafB.Bounds() != (Rect{X: 0, Y: 0, W: 40, H: 40}) {
		t.Fatalf("card B leaf should fill, got %+v", leafB.Bounds())
	}
}

// TestContainerSetItems covers replacing and clearing the item list.
func TestContainerSetItems(t *testing.T) {
	a, b, c2 := &dockProbe{}, &dockProbe{}, &dockProbe{}
	c := NewContainer(FitLayout{})
	c.AddWidget(a)
	c.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})

	// Replace the single item with two new ones — both get laid out (filled).
	c.SetItems(Item{Widget: b}, Item{Widget: c2})
	if len(c.Items()) != 2 || b.Bounds() != (Rect{X: 0, Y: 0, W: 10, H: 10}) {
		t.Fatalf("SetItems replace: items=%d b=%+v", len(c.Items()), b.Bounds())
	}
	// Clear.
	c.SetItems()
	if len(c.Items()) != 0 {
		t.Fatalf("SetItems() should clear: %d", len(c.Items()))
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
