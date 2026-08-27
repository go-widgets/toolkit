// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestAScrollViewSizesAMeasurableChild.
//
// Nothing gave the child bounds: NewScrollView took a widget, Draw kept the
// width and height the child already carried, and declaring the content extent
// was left to the caller. A scroll view around a freshly built column therefore
// showed nothing, and one whose content changed scrolled over a stale extent.
func TestAScrollViewSizesAMeasurableChild(t *testing.T) {
	// Height depends on width, like a card or a settings group.
	card := &widthSized{content: 12000}
	sv := NewScrollView(card)
	sv.SetBounds(Rect{X: 5, Y: 7, W: 300, H: 100})

	vp := sv.viewport()
	if got := card.Bounds().W; got != vp.W {
		t.Errorf("the child is %d wide, the viewport %d", got, vp.W)
	}
	want := card.Measure(vp.W)
	if got := card.Bounds().H; got != want {
		t.Errorf("the child is %d tall, it measures %d at that width", got, want)
	}
	if cw, ch := sv.contentW, sv.contentH; cw != vp.W || ch != want {
		t.Errorf("the content extent is %dx%d, want %dx%d", cw, ch, vp.W, want)
	}
	// Scrolling is clamped against that extent, which is what it is for.
	sv.Scroll(0, 10_000)
	if got, max := sv.OffsetY().Get(), want-vp.H; got != max {
		t.Errorf("scrolled to %d, the end of the content is %d", got, max)
	}

	// Narrower: the content gets taller and the extent follows.
	sv.SetBounds(Rect{X: 5, Y: 7, W: 150, H: 100})
	if got := card.Bounds().H; got <= want {
		t.Errorf("at half the width the child is %d tall, no more than the %d it "+
			"was: it did not re-measure", got, want)
	}
}

// TestAScrollViewFillsItselfWithAShortChild: a child that measures less than the
// viewport is stretched to it, so a short page does not leave a strip of bare
// surface below it, and there is nothing to scroll.
func TestAScrollViewFillsItselfWithAShortChild(t *testing.T) {
	card := &widthSized{content: 10} // one row, 10 tall
	sv := NewScrollView(card)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})

	vp := sv.viewport()
	if got := card.Bounds().H; got != vp.H {
		t.Errorf("a short child is %d tall in a %d viewport", got, vp.H)
	}
	sv.Scroll(0, 50)
	if got := sv.OffsetY().Get(); got != 0 {
		t.Errorf("there was something to scroll: offset %d", got)
	}
}

// TestAScrollViewLeavesAnUnmeasurableChildAlone, and a nil one is harmless.
//
// This is what keeps every existing caller working: one that lays the child out
// itself and declares the extent by hand must not have either overwritten.
func TestAScrollViewLeavesAnUnmeasurableChildAlone(t *testing.T) {
	plain := &Label{}
	plain.SetBounds(Rect{X: 1, Y: 2, W: 33, H: 44})
	sv := NewScrollView(plain)
	sv.SetContentSize(500, 600)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})

	if got := plain.Bounds(); got != (Rect{X: 1, Y: 2, W: 33, H: 44}) {
		t.Errorf("the child was re-laid-out to %+v", got)
	}
	if sv.contentW != 500 || sv.contentH != 600 {
		t.Errorf("the declared extent became %dx%d", sv.contentW, sv.contentH)
	}

	empty := NewScrollView(nil)
	empty.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10}) // must not panic
}

// TestAScrollViewTakesTheTwoAxisMeasurerToo: a child that answers on both axes
// but not "height at a width" -- an AlignBox, a Padding -- is sized the same way.
func TestAScrollViewTakesTheTwoAxisMeasurerToo(t *testing.T) {
	both := &bothSized{w: 100, h: 900}
	sv := NewScrollView(both)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	if got := both.Bounds().H; got != 900 {
		t.Errorf("the child is %d tall, want the 900 it measures", got)
	}
	if sv.contentH != 900 {
		t.Errorf("the content extent is %d tall", sv.contentH)
	}

	// A child whose measure comes back as nothing counts as unmeasurable: it is
	// left alone rather than declared to be zero pixels of content.
	none := &bothSized{w: 0, h: 0}
	none.SetBounds(Rect{W: 12, H: 13})
	sv2 := NewScrollView(none)
	sv2.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	if got := none.Bounds(); got != (Rect{W: 12, H: 13}) {
		t.Errorf("a child measuring nothing was re-laid-out to %+v", got)
	}
}

// TestAClickReachesTheContentOfAScrollView.
//
// The content was PASSIVE for clicks: a press that neither scrollbar wanted
// began a pan and nothing was forwarded, so every control inside a scroll view
// was dead to the mouse. A long form is exactly what a scroll view is for, and
// none of its drop-downs, switches or buttons could be operated -- which is how
// a settings window came to have a control nobody could use.
func TestAClickReachesTheContentOfAScrollView(t *testing.T) {
	first, second := 0, 0
	top := NewButton("Save", func() { first++ })
	lower := NewButton("Close", func() { second++ })
	page := NewVBox()
	page.Spacing = 0
	page.AddFixed(top, 30)            // content y 0..30
	page.AddFixed(NewLabel("."), 100) // 30..130
	page.AddFixed(lower, 30)          // 130..160
	page.AddFixed(NewLabel("."), 400)

	sv := NewScrollView(page)
	sv.SetBounds(Rect{X: 10, Y: 20, W: 300, H: 100})
	// A box is not measurable, so the caller sizes the content, as before.
	page.SetBounds(Rect{X: 10, Y: 20, W: 284, H: 560})
	sv.SetContentSize(284, 560)

	// View-local y=15 is content y=15, which is the first button.
	sv.OnEvent(Event{Kind: EventClick, X: 4, Y: 15})
	if first != 1 {
		t.Fatalf("the first button was pressed %d times; the click never reached "+
			"the content at all", first)
	}

	// Scrolled down by 100, the same point is content y=115 -- the filler.
	sv.Scroll(0, 100)
	sv.OnEvent(Event{Kind: EventClick, X: 4, Y: 15})
	if first != 1 {
		t.Errorf("after scrolling, the same point still pressed the first button "+
			"(%d times): the offset is not applied", first)
	}

	// And view-local y=35 is now content y=135, which is the second button.
	sv.OnEvent(Event{Kind: EventClick, X: 4, Y: 35})
	if second != 1 {
		t.Errorf("the second button was pressed %d times; a click after scrolling "+
			"does not land where the offset says", second)
	}
}

// TestAClickOnTheScrollbarDoesNotReachTheContent: the thumb owns its own press,
// as it did before.
func TestAClickOnTheScrollbarDoesNotReachTheContent(t *testing.T) {
	hit := 0
	button := NewButton("wide", func() { hit++ })
	page := NewVBox()
	page.Spacing = 0
	page.AddFixed(button, 600)

	sv := NewScrollView(page)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 100})
	page.SetBounds(Rect{X: 0, Y: 0, W: 284, H: 600})
	sv.SetContentSize(284, 600)

	// The vertical scrollbar's own geometry, so the press is on the track and
	// not a guess at where it is.
	g, ok := sv.vscrollGeom()
	if !ok {
		t.Fatal("a content taller than the view has no vertical scrollbar")
	}
	// A press near the start of the track, on the track column: paging the view
	// toward the click is the scrollbar's own behaviour.
	sv.OnEvent(Event{Kind: EventClick,
		X: g.cross0 + 1, Y: g.trackStart + g.trackLen - 2})
	if hit != 0 {
		t.Errorf("a press on the scrollbar reached the content %d times", hit)
	}
	if sv.OffsetY().Get() == 0 {
		t.Error("a press on the scrollbar track did not move the view")
	}
}
