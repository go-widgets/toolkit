// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constructor ---------------------------------------------------------

func TestNewBreadcrumbsStoresSegments(t *testing.T) {
	segs := []string{"Home", "Docs"}
	b := NewBreadcrumbs(segs)
	if len(b.Segments) != 2 || b.Segments[0] != "Home" {
		t.Fatalf("NewBreadcrumbs round-trip broken: %+v", b.Segments)
	}
}

// --- Draw branches -------------------------------------------------------

// Empty segments: the loop body never runs; only the bounding rect is
// touched (nothing painted). Verifies the no-op early-exit path.
func TestBreadcrumbsDrawEmptyNoPaint(t *testing.T) {
	const w, h = 40, 16
	theme := DefaultLight()
	b := NewBreadcrumbs(nil)
	b.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 16})
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)
	// Every pixel should remain the sentinel 0xC8 — nothing painted.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y).R != 0xC8 {
				t.Fatalf("empty Breadcrumbs painted (%d,%d) = %+v",
					x, y, pixelAt(buf, w, x, y))
			}
		}
	}
}

// Single segment: text is drawn, no chevron appears (the `i < n-1`
// branch stays false).
func TestBreadcrumbsDrawSingleSegmentNoChevron(t *testing.T) {
	const w, h = 60, 16
	theme := DefaultLight()
	b := NewBreadcrumbs([]string{"Home"})
	b.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 16})
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)
	// Some ink pixels in OnBackground for the "Home" text.
	inkFound := false
	for y := 0; y < h && !inkFound; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.OnBackground {
				inkFound = true
				break
			}
		}
	}
	if !inkFound {
		t.Fatal("single-segment breadcrumbs painted 0 ink pixels")
	}
	// No Border-coloured chevron ink anywhere.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.Border {
				t.Fatalf("chevron drawn at (%d,%d) with a single segment", x, y)
			}
		}
	}
}

// Multi-segment: chevrons are painted in Theme.Border between each
// pair. Exercises both the OnBackground text branch and the Border
// chevron branch.
func TestBreadcrumbsDrawMultiSegmentPaintsChevron(t *testing.T) {
	const w, h = 160, 16
	theme := DefaultLight()
	b := NewBreadcrumbs([]string{"Home", "Docs", "Reference"})
	b.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 16})
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)
	borderInk := 0
	onBgInk := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch pixelAt(buf, w, x, y) {
			case theme.Border:
				borderInk++
			case theme.OnBackground:
				onBgInk++
			}
		}
	}
	if onBgInk == 0 {
		t.Fatal("segment text should paint OnBackground pixels")
	}
	if borderInk == 0 {
		t.Fatal("multi-segment breadcrumbs should paint chevron pixels in Border")
	}
}

// Tight bounds (Bounds.H == GlyphHeight()): the centring branch is
// skipped and text anchors at Bounds.Y.
func TestBreadcrumbsDrawTightBoundsSkipsCentring(t *testing.T) {
	var w, h = 40, GlyphHeight() + 2
	theme := DefaultLight()
	b := NewBreadcrumbs([]string{"Hi"})
	b.SetBounds(Rect{X: 0, Y: 0, W: 40, H: GlyphHeight()})
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)
	// The centring branch skipped -> ink starts at y = Bounds.Y = 0.
	// 'H' column 0 is 0x7F so row 0 IS lit at (0,0).
	if pixelAt(buf, w, 0, 0) != theme.OnBackground {
		t.Fatalf("tight-bounds ink at (0,0) = %+v, want OnBackground",
			pixelAt(buf, w, 0, 0))
	}
}

// Tall bounds: the centring branch fires. Verify the ink lands below
// r.Y instead of at r.Y.
func TestBreadcrumbsDrawTallBoundsCentresText(t *testing.T) {
	const w, h = 60, 30
	theme := DefaultLight()
	b := NewBreadcrumbs([]string{"H"})
	b.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 30})
	buf := makeSurface(w, h)
	b.Draw(newP(buf, w), theme)
	// Row 0 must be unpainted (text drops below the top of the box).
	for x := 0; x < w; x++ {
		if pixelAt(buf, w, x, 0) == theme.OnBackground {
			t.Fatalf("tall bounds should NOT paint row 0; got ink at (%d,0)", x)
		}
	}
	// Some ink lands in the middle band.
	found := false
	for y := h/2 - 3; y < h/2+3 && !found; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.OnBackground {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("tall-bounds Breadcrumbs painted no ink in the vertical middle band")
	}
}
