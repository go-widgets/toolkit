// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"testing"
)

// TestAPageKeepsItsPlaceWhileItWaitsForItsBitmap is the case neither setter
// could express: lay out the whole document, hold bitmaps for a few.
//
// ⛔ The assertion is on the LAYOUT, not on the bitmap slice. A test that read
// pv.pages back would agree with any implementation that stored them, including
// one that quietly resized the page set and left the scrollbar describing a
// shorter document than the reader has.
func TestAPageKeepsItsPlaceWhileItWaitsForItsBitmap(t *testing.T) {
	pv := NewPagedView(nil)
	sizes := make([]image.Point, 500)
	for i := range sizes {
		sizes[i] = image.Point{X: 100, Y: 140}
	}
	pv.SetPageSizes(sizes)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	before := pv.lay.contentH

	pv.SetPageBitmap(250, solidPage(100, 140, pgRed))
	if got := pv.PageCount(); got != 500 {
		t.Errorf("giving one page a bitmap left %d pages, want 500", got)
	}
	pv.relayout()
	if pv.lay.contentH != before {
		t.Errorf("the stack was %d tall and is now %d: one bitmap changed the shape of "+
			"the document", before, pv.lay.contentH)
	}
	if pv.pageImage(249) == nil {
		t.Error("page 250 has no bitmap after it was given one")
	}
	if pv.pageImage(248) != nil || pv.pageImage(250) != nil {
		t.Error("a neighbour of page 250 acquired a bitmap it was never given")
	}
}

// TestABitmapCanBeTakenBackWhenThePageLeavesTheWindow: the window moves, and
// what it leaves behind has to be releasable or the window is a leak with
// extra steps.
func TestABitmapCanBeTakenBackWhenThePageLeavesTheWindow(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 100, Y: 140}, {X: 100, Y: 140}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})

	pv.SetPageBitmap(1, solidPage(100, 140, pgRed))
	if pv.pageImage(0) == nil {
		t.Fatal("precondition: page 1 should hold a bitmap")
	}
	pv.SetPageBitmap(1, nil)
	if pv.pageImage(0) != nil {
		t.Error("page 1 kept its bitmap after it was taken back")
	}
	if pv.PageCount() != 2 {
		t.Errorf("taking a bitmap back left %d pages, want 2", pv.PageCount())
	}
}

// TestABitmapForAPageThatIsNoLongerThereIsDropped: the window moves while a
// rasteriser is still working, and the document can be replaced under it. A
// bitmap arriving for a page that has gone is ordinary.
func TestABitmapForAPageThatIsNoLongerThereIsDropped(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 100, Y: 140}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})

	for _, page := range []int{0, -1, 2, 999} {
		pv.SetPageBitmap(page, solidPage(100, 140, pgRed))
	}
	if pv.PageCount() != 1 {
		t.Errorf("a bitmap for a page outside the document changed the page count to %d",
			pv.PageCount())
	}
	if pv.pageImage(0) != nil {
		t.Error("a bitmap aimed outside the document landed on page 1")
	}
}

// TestADeclaredSizeOutranksTheBitmapItIsGiven: a coarse rendering stands in
// for a page until a sharper one arrives, and the page must not change shape
// when it does.
func TestADeclaredSizeOutranksTheBitmapItIsGiven(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 400, Y: 560}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 600})
	w, h := pv.scaledSize(0)

	// A thumbnail: a tenth of the page, the right shape.
	pv.SetPageBitmap(1, solidPage(40, 56, pgRed))
	pv.relayout()
	if gw, gh := pv.scaledSize(0); gw != w || gh != h {
		t.Errorf("a coarse stand-in resized the page from %dx%d to %dx%d", w, h, gw, gh)
	}
	// And the real thing, later.
	pv.SetPageBitmap(1, solidPage(400, 560, pgRed))
	pv.relayout()
	if gw, gh := pv.scaledSize(0); gw != w || gh != h {
		t.Errorf("the sharp rendering resized the page from %dx%d to %dx%d", w, h, gw, gh)
	}
}

// TestSetPagesStillOwnsTheWholeSet: the two ways of declaring pages must not
// bleed. SetPages after SetPageBitmap replaces everything, sizes included.
func TestSetPagesStillOwnsTheWholeSet(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 100, Y: 140}, {X: 100, Y: 140}, {X: 100, Y: 140}})
	pv.SetPageBitmap(2, solidPage(100, 140, pgRed))

	pv.SetPages([]*image.RGBA{solidPage(10, 10, pgRed)})
	if pv.PageCount() != 1 {
		t.Errorf("SetPages left %d pages, want 1", pv.PageCount())
	}
	if pv.pageImage(1) != nil {
		t.Error("a bitmap from the previous page set survived SetPages")
	}
}
