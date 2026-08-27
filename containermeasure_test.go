// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestAColumnContainerMeasuresItsItems.
//
// This is the piece that lets a page of cards be scrollable content: a
// ScrollView sizes a child that can say how tall it is, an Item marked Natural
// asks the same question, and until now a Container could answer neither -- so
// the answer had to be a constant the caller worked out by hand, which is what
// went wrong every time the window changed size.
func TestAColumnContainerMeasuresItsItems(t *testing.T) {
	first := &widthSized{content: 900}  // 3 rows of 10 at width 300
	second := &widthSized{content: 300} // 1 row of 10
	spacer := &Label{}

	l := NewBoxLayout()
	l.Vertical = true
	l.Spacing = 6
	page := NewContainer(l)
	page.Add(Item{Widget: first, Natural: true})
	page.Add(Item{Widget: second, Natural: true})
	page.Add(Item{Widget: spacer, Flex: 1})

	w, h := page.Measure(300, 0)
	// 30 + 10 for the two cards, nothing for the flex spacer, two gaps of 6.
	if want := 30 + 10 + 2*6; h != want {
		t.Errorf("the page measures %d tall, want %d", h, want)
	}
	if w != 300 {
		t.Errorf("the page measures %d wide, want the width it was offered", w)
	}

	// Narrower: the first card wraps to more rows and the page grows with it.
	_, tall := page.Measure(150, 0)
	if tall <= h {
		t.Errorf("at half the width the page measures %d, no more than %d", tall, h)
	}

	// And the measurement moved nothing: a parent asks several sizes before it
	// commits to one.
	if first.Bounds() != (Rect{}) || second.Bounds() != (Rect{}) {
		t.Errorf("measuring laid the items out: %+v, %+v",
			first.Bounds(), second.Bounds())
	}

	// An empty container needs nothing.
	if w, h := NewContainer(l).Measure(300, 300); w != 0 || h != 0 {
		t.Errorf("an empty container measures %dx%d", w, h)
	}
}

// TestARowContainerMeasuresItsItems: the horizontal case, where an explicit Size
// is the item's own answer and is used as it stands.
func TestARowContainerMeasuresItsItems(t *testing.T) {
	l := NewBoxLayout()
	l.Spacing = 4
	row := NewContainer(l)
	row.Add(Item{Widget: NewButton("Save", nil), Size: 96})
	row.Add(Item{Widget: NewButton("Close", nil), Size: 96})

	w, h := row.Measure(0, 40)
	if want := 96 + 96 + 4; w != want {
		t.Errorf("the button bar measures %d wide, want %d", w, want)
	}
	if h <= 0 {
		t.Errorf("the button bar measures %d tall", h)
	}
}

// TestAFitContainerMeasuresItsChild, in all three ways a child can answer.
func TestAFitContainerMeasuresItsChild(t *testing.T) {
	both := &bothSized{w: 120, h: 40}
	fit := NewContainer(FitLayout{})
	fit.Add(Item{Widget: both})
	if w, h := fit.Measure(500, 500); w != 120 || h != 40 {
		t.Errorf("a two-axis child measured %dx%d, want 120x40", w, h)
	}

	card := &widthSized{content: 600}
	fit2 := NewContainer(FitLayout{})
	fit2.Add(Item{Widget: card})
	if w, h := fit2.Measure(300, 500); w != 300 || h != 20 {
		t.Errorf("a card child measured %dx%d, want 300x20", w, h)
	}

	plain := &Label{}
	fit3 := NewContainer(FitLayout{})
	fit3.Add(Item{Widget: plain})
	plain.SetBounds(Rect{W: 44, H: 22})
	if w, h := fit3.Measure(300, 500); w != 44 || h != 22 {
		t.Errorf("a plain child measured %dx%d, want the 44x22 it carries", w, h)
	}

}

// TestAContainerWithAnUnmeasurableLayoutReportsItsBounds: a layout that cannot
// measure leaves the container answering what every other unmeasurable widget
// answers, so nothing regresses for a caller using one.
func TestAContainerWithAnUnmeasurableLayoutReportsItsBounds(t *testing.T) {
	c := NewContainer(BorderLayout{})
	c.Add(Item{Widget: NewLabel("north"), Region: RegionNorth, Size: 20})
	c.SetBounds(Rect{X: 0, Y: 0, W: 321, H: 123})
	if w, h := c.Measure(999, 999); w != 321 || h != 123 {
		t.Errorf("measured %dx%d, want the bounds 321x123", w, h)
	}
}

// TestAMeasuredContainerScrollsAndNests is the composition this was for: a page
// of cards inside a ScrollView, with a button bar pinned below it by a border
// layout. It is the arrangement a settings window needs, and the one that
// previously drew the bar over the text when the window shrank.
func TestAMeasuredContainerScrollsAndNests(t *testing.T) {
	col := NewBoxLayout()
	col.Vertical = true
	col.Spacing = 8
	page := NewContainer(col)
	page.Add(Item{Widget: &widthSized{content: 60000}, Natural: true})

	scroll := NewScrollView(page)
	bar := NewLabel("Save   Close")
	frame := NewContainer(BorderLayout{})
	frame.Add(Item{Widget: bar, Region: RegionSouth, Size: 40})
	frame.Add(Item{Widget: scroll, Region: RegionCenter})

	frame.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})

	// The bar keeps its band at the bottom and the scroll view takes the rest.
	if got := bar.Bounds(); got.Y+got.H != 300 || got.H != 40 {
		t.Errorf("the button bar is at %+v, not pinned to the bottom", got)
	}
	if got := scroll.Bounds(); got.Y+got.H > bar.Bounds().Y {
		t.Errorf("the scroll view %+v runs under the button bar %+v",
			got, bar.Bounds())
	}
	// The page is as tall as its content, which is taller than the view: that is
	// what there is to scroll, and it is why nothing has to overlap.
	if page.Bounds().H <= scroll.Bounds().H {
		t.Errorf("the page is %d tall in a %d view: nothing to scroll",
			page.Bounds().H, scroll.Bounds().H)
	}

	// Shrink the window: the page re-measures, the bar stays put, and the page
	// still does not reach into the bar's band.
	frame.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 150})
	if got := bar.Bounds(); got.Y+got.H != 150 || got.H != 40 {
		t.Errorf("after the resize the bar is at %+v", got)
	}
	if got := scroll.Bounds(); got.Y+got.H > bar.Bounds().Y {
		t.Errorf("after the resize the scroll view %+v runs under the bar %+v",
			got, bar.Bounds())
	}
}
