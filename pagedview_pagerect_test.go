// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"testing"
)

// PageRect is the forward mapping PageAt inverts: a round-trip through both
// must land where it started. This is the property that keeps a host's overlay
// on the page a click reports.
func TestPageRectRoundTripsWithPageAt(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{
		solidPage(100, 140, pgRed),
		solidPage(100, 140, pgGreen),
	})
	pv.SetBounds(Rect{X: 30, Y: 40, W: 300, H: 300})

	for page := 1; page <= 2; page++ {
		rect, clip, ok := pv.PageRect(page)
		if !ok {
			t.Fatalf("page %d: not placed", page)
		}
		if clip.W <= 0 || clip.H <= 0 {
			continue // scrolled out of the pane; nothing to probe
		}
		// A point a few pixels inside the visible part of the card, converted to
		// the widget-local space PageAt consumes.
		sx, sy := clip.X+2, clip.Y+2
		gotPage, lx, ly, hit := pv.PageAt(sx-pv.Bounds().X, sy-pv.Bounds().Y)
		if !hit || gotPage != page {
			t.Errorf("page %d: PageAt(%d,%d) = page %d, hit %v — the two seams disagree",
				page, sx, sy, gotPage, hit)
			continue
		}
		// And back: the natural point must map inside the rect PageRect reported.
		z := pv.Zoom().Get()
		backX := rect.X + lx*z/100
		backY := rect.Y + ly*z/100
		if abs(backX-sx) > 1 || abs(backY-sy) > 1 {
			t.Errorf("page %d: round-trip (%d,%d) -> (%d,%d)", page, sx, sy, backX, backY)
		}
	}
}

// The clip excludes the toolbar strip and the scrollbar gutter — a host that
// takes rect alone paints its object over the toolbar.
func TestPageRectClipsToThePane(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(100, 4000, pgRed)})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})

	rect, clip, ok := pv.PageRect(1)
	if !ok {
		t.Fatal("page 1 not placed")
	}
	if tb := pv.Bounds().Y + scaled(pagedToolbarH); clip.Y < tb {
		t.Errorf("clip %+v starts at %d, above the pane top %d — it would cover the toolbar", clip, clip.Y, tb)
	}
	if clip.H >= rect.H {
		t.Errorf("a page taller than the pane must be clipped: clip %+v rect %+v", clip, rect)
	}
	if clip.Y+clip.H > pv.Bounds().Y+pv.Bounds().H {
		t.Errorf("clip %+v escapes the widget %+v", clip, pv.Bounds())
	}
}

// A page scrolled out of the pane still exists: ok stays true and the clip goes
// empty, so a host hides its object instead of destroying it.
func TestPageRectScrolledOutIsEmptyButPresent(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{
		solidPage(80, 200, pgRed),
		solidPage(80, 200, pgGreen),
	})
	pv.SetBounds(Rect{W: 200, H: 120})
	pv.OnEvent(Event{Kind: EventScroll, X: 10, Y: 100, Delta: 400}) // to the end

	_, clip, ok := pv.PageRect(1)
	if !ok {
		t.Fatal("page 1 must still be reported: it exists, it is merely off screen")
	}
	if clip.W != 0 || clip.H != 0 {
		t.Errorf("first page scrolled away must report an empty clip, got %+v", clip)
	}
}

// Only a page the layout places is reported. Paginated mode shows one card, so
// the others are not placed; an out-of-range page is never placed.
func TestPageRectRejectsUnplacedPages(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(50, 50, pgRed), solidPage(50, 50, pgGreen)})
	pv.SetBounds(Rect{W: 200, H: 200})

	for _, page := range []int{0, -1, 3} {
		if _, _, ok := pv.PageRect(page); ok {
			t.Errorf("page %d must not be placed", page)
		}
	}
	pv.Mode().Set(PagedPaginated)
	pv.CurrentPage().Set(1)
	if _, _, ok := pv.PageRect(1); !ok {
		t.Error("the displayed page must be placed in paginated mode")
	}
	if _, _, ok := pv.PageRect(2); ok {
		t.Error("paginated mode places one card: the other page is not on screen at all")
	}
	if _, _, ok := NewPagedView(nil).PageRect(1); ok {
		t.Error("an empty view places nothing")
	}
}

// Zoom moves and resizes the card, and the reported width tracks the blit.
func TestPageRectFollowsZoom(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(100, 100, pgRed)})
	pv.SetBounds(Rect{W: 400, H: 400})

	before, _, _ := pv.PageRect(1)
	pv.Zoom().Set(200)
	after, _, ok := pv.PageRect(1)
	if !ok {
		t.Fatal("page 1 not placed after zooming")
	}
	if after.W <= before.W {
		t.Errorf("zooming in must widen the card: %d -> %d", before.W, after.W)
	}
	if want := 100 * pv.Zoom().Get() / 100; after.W != want {
		t.Errorf("card width = %d, want the natural width at zoom (%d)", after.W, want)
	}
}
