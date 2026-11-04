// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"testing"
)

// TestPagedViewFitWidthFillsPane: sticky fit-to-width on a page narrower than
// the pane zooms in so the page nearly fills the usable width.
func TestPagedViewFitWidthFillsPane(t *testing.T) {
	pv := NewPagedView(nil)
	// A page whose fit zoom lands inside [min,max]: 300 wide in a ~760 usable
	// pane fits at ~253%, well under the 400% cap.
	pv.SetPageSizes([]image.Point{{X: 300, Y: 600}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 500})
	if pv.Zoom().Get() != pagedZoomDefault {
		t.Fatalf("precondition: fresh view should be at the default zoom, got %d", pv.Zoom().Get())
	}

	pv.SetFitWidth(true)
	if !pv.FitWidth() {
		t.Fatal("SetFitWidth(true) should report on")
	}
	if z := pv.Zoom().Get(); z <= pagedZoomDefault {
		t.Fatalf("a page narrower than the pane should zoom past %d, got %d", pagedZoomDefault, z)
	}
	w, _ := pv.scaledSize(0)
	usableW := pv.scroll.Bounds().W - scrollGutter() - 2*scaled(pagedMargin)
	if w > usableW {
		t.Errorf("scaled width %d overflows the usable width %d", w, usableW)
	}
	if usableW-w > 6 { // integer zoom rounds down by at most one page-natural-width/100 px
		t.Errorf("scaled width %d does not fill the usable width %d", w, usableW)
	}
}

// TestPagedViewFitWidthRefitsOnResize: with the mode on, a wider pane re-derives
// a larger zoom without any further call — the whole point of "sticky".
func TestPagedViewFitWidthRefitsOnResize(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 100, Y: 200}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	pv.SetFitWidth(true)
	narrow := pv.Zoom().Get()

	pv.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 400}) // twice as wide
	wide := pv.Zoom().Get()
	if wide <= narrow {
		t.Fatalf("a wider pane should re-fit to a larger zoom: %d -> %d", narrow, wide)
	}
}

// TestPagedViewManualZoomClearsFit: a manual zoom takes control back, so a later
// resize no longer overrides the reader's chosen zoom.
func TestPagedViewManualZoomClearsFit(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 100, Y: 200}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	pv.SetFitWidth(true)

	pv.zoomIn()
	if pv.FitWidth() {
		t.Fatal("a manual zoom-in should clear sticky fit-to-width")
	}
	chosen := pv.Zoom().Get()
	pv.SetBounds(Rect{X: 0, Y: 0, W: 900, H: 400}) // a resize must NOT re-fit now
	if pv.Zoom().Get() != chosen {
		t.Errorf("resize overrode a manual zoom: %d -> %d", chosen, pv.Zoom().Get())
	}

	pv.zoomOut() // exercise the other manual control's fit-clear too
	if pv.FitWidth() {
		t.Fatal("zoom-out should also clear fit")
	}
}

// TestPagedViewFitPageShowsTheWholePage: fit-to-page is the smaller of the two
// fits, so BOTH edges land inside the pane.
//
// ⛔ The page here is TALL for the pane (300x900 in a 800x500 pane), which is
// the case fit-to-width gets wrong and the reason this mode exists: fitting the
// width of that page puts two thirds of it below the fold. A square page, or a
// pane the same shape as the page, would pass this test under either rule and
// prove nothing.
func TestPagedViewFitPageShowsTheWholePage(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 300, Y: 900}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 500})

	pv.SetFit(PagedFitWidth)
	_, tallH := pv.scaledSize(0)
	usableH := pv.scroll.Bounds().H - scrollGutter() - 2*scaled(pagedMargin)
	if tallH <= usableH {
		t.Fatalf("precondition: fit-to-width should overflow the pane height for this page, "+
			"%d fits in %d", tallH, usableH)
	}

	pv.SetFit(PagedFitPage)
	if pv.Fit() != PagedFitPage {
		t.Fatal("SetFit(PagedFitPage) should report PagedFitPage")
	}
	w, h := pv.scaledSize(0)
	usableW := pv.scroll.Bounds().W - scrollGutter() - 2*scaled(pagedMargin)
	if w > usableW || h > usableH {
		t.Errorf("the page is %dx%d in a usable %dx%d: fit-to-page spilled", w, h, usableW, usableH)
	}
	// And it is the LARGEST such zoom, not merely a small one: the binding axis
	// must nearly touch. Without this, SetFit could satisfy the test above by
	// zooming to the minimum.
	if usableH-h > 9 { // integer percent rounds down by at most natural-height/100
		t.Errorf("the page is %d tall in a usable %d: fit-to-page left the pane half empty", h, usableH)
	}
}

// TestPagedViewFitPageRefitsOnResize: sticky, like fit-to-width — and this one
// must follow the HEIGHT, which fit-to-width never reads.
func TestPagedViewFitPageRefitsOnResize(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 300, Y: 900}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 500})
	pv.SetFit(PagedFitPage)
	short := pv.Zoom().Get()

	pv.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 1000}) // twice as tall, same width
	if tall := pv.Zoom().Get(); tall <= short {
		t.Fatalf("a taller pane should re-fit a height-bound page larger: %d -> %d", short, tall)
	}
}

// TestPagedViewFitPageFallsBackWhenHeightIsUnknown: a page that declares no
// height is laid out by width alone rather than refusing to fit. SetPageSizes
// takes whatever a host measured, and a zero is what an unmeasurable page
// reports.
func TestPagedViewFitPageFallsBackWhenHeightIsUnknown(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 300, Y: 0}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 500})

	pv.SetFit(PagedFitPage)
	byPage := pv.Zoom().Get()
	pv.SetFit(PagedFitWidth)
	if byWidth := pv.Zoom().Get(); byPage != byWidth {
		t.Errorf("a page with no height fitted to %d by page and %d by width: the two should "+
			"agree, there being no height to be bound by", byPage, byWidth)
	}
}

// TestPagedViewManualZoomClearsFitPage: the fit-clear is the mode's, not
// fit-to-width's alone.
func TestPagedViewManualZoomClearsFitPage(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 300, Y: 900}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 500})
	pv.SetFit(PagedFitPage)

	pv.zoomIn()
	if pv.Fit() != PagedFitNone {
		t.Fatalf("a manual zoom should clear fit-to-page, Fit() = %d", pv.Fit())
	}
}

// TestPagedViewFitButtonsShowWhichIsOn: the two fit buttons are sticky state,
// so exactly the active one reads Selected — a reader who cannot see which fit
// is on cannot tell a fitted page from a page that happens to be that size.
func TestPagedViewFitButtonsShowWhichIsOn(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 300, Y: 900}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 500})

	check := func(what string, wantW, wantP bool) {
		t.Helper()
		if got := pv.btnFit.Selected().Get(); got != wantW {
			t.Errorf("%s: fit-width button Selected = %v, want %v", what, got, wantW)
		}
		if got := pv.btnFitPage.Selected().Get(); got != wantP {
			t.Errorf("%s: fit-page button Selected = %v, want %v", what, got, wantP)
		}
	}
	check("fresh", false, false)
	pv.SetFit(PagedFitWidth)
	check("after fit-width", true, false)
	pv.SetFit(PagedFitPage)
	check("after fit-page", false, true)
	pv.zoomIn()
	check("after a manual zoom", false, false)
}

// TestPagedViewFitPageDoesNotOscillate: the fit subtracts the scrollbar gutter
// even when the fitted page will not need one. Giving it back would be a few
// pixels larger and could never settle — fitting the full width removes the
// scrollbar, which widens the pane, which re-fits larger, which brings it back.
//
// Laying out twice at unchanged bounds must therefore reach the same zoom.
func TestPagedViewFitPageDoesNotOscillate(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 300, Y: 900}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 500})
	pv.SetFit(PagedFitPage)
	first := pv.Zoom().Get()
	pv.relayout()
	pv.relayout()
	if again := pv.Zoom().Get(); again != first {
		t.Errorf("relaying out at unchanged bounds moved the fit: %d -> %d", first, again)
	}
}

// TestPagedViewFitWidthNoPagesNoop: turning the mode on with no pages (or a
// zero-width pane) is safe and leaves the zoom at its default.
func TestPagedViewFitWidthNoPagesNoop(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	pv.SetFitWidth(true) // no pages yet
	if pv.Zoom().Get() != pagedZoomDefault {
		t.Errorf("fit with no pages should leave the default zoom, got %d", pv.Zoom().Get())
	}
	if _, ok := pv.fitZoom(PagedFitWidth); ok {
		t.Error("fitZoom should report not-computable with no pages")
	}

	// A page but a zero-width pane clamps the usable width to 1 rather than
	// dividing by zero.
	pv.SetPageSizes([]image.Point{{X: 100, Y: 200}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 400})
	if _, ok := pv.fitZoom(PagedFitWidth); !ok {
		t.Error("fitZoom should compute (clamped) even for a zero-width pane")
	}
}

// TestClampZoom covers the min/max/pass-through branches directly.
func TestClampZoom(t *testing.T) {
	if got := clampZoom(pagedZoomMin - 50); got != pagedZoomMin {
		t.Errorf("below min: %d, want %d", got, pagedZoomMin)
	}
	if got := clampZoom(pagedZoomMax + 50); got != pagedZoomMax {
		t.Errorf("above max: %d, want %d", got, pagedZoomMax)
	}
	if got := clampZoom(150); got != 150 {
		t.Errorf("in range: %d, want 150", got)
	}
}
