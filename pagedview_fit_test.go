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

// TestPagedViewFitWidthNoPagesNoop: turning the mode on with no pages (or a
// zero-width pane) is safe and leaves the zoom at its default.
func TestPagedViewFitWidthNoPagesNoop(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	pv.SetFitWidth(true) // no pages yet
	if pv.Zoom().Get() != pagedZoomDefault {
		t.Errorf("fit with no pages should leave the default zoom, got %d", pv.Zoom().Get())
	}
	if _, ok := pv.fitWidthZoom(); ok {
		t.Error("fitWidthZoom should report not-computable with no pages")
	}

	// A page but a zero-width pane clamps the usable width to 1 rather than
	// dividing by zero.
	pv.SetPageSizes([]image.Point{{X: 100, Y: 200}})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 400})
	if _, ok := pv.fitWidthZoom(); !ok {
		t.Error("fitWidthZoom should compute (clamped) even for a zero-width pane")
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
