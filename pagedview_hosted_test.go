// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"testing"

	"github.com/go-widgets/painter"
)

// hostedRender draws pv onto a fresh w×h buffer and returns it.
func hostedRender(pv *PagedView, w, h int) []byte {
	buf := make([]byte, 4*w*h)
	pv.Draw(painter.NewPixelPainter(buf, w, h), DefaultLight())
	return buf
}

// A hosted page lays out exactly like a blitted one: same cards, same
// rectangles, same paging. Only the blit is missing.
func TestHostedPagesLayOutLikeBitmaps(t *testing.T) {
	const w, h = 120, 160
	blit := NewPagedView([]*image.RGBA{solidPage(w, h, pgRed), solidPage(w, h, pgGreen)})
	blit.SetBounds(Rect{X: 7, Y: 9, W: 300, H: 300})

	hosted := NewPagedView(nil)
	hosted.SetPageSizes([]image.Point{{X: w, Y: h}, {X: w, Y: h}})
	hosted.SetBounds(Rect{X: 7, Y: 9, W: 300, H: 300})

	if blit.PageCount() != hosted.PageCount() {
		t.Fatalf("page counts differ: %d vs %d", blit.PageCount(), hosted.PageCount())
	}
	for page := 1; page <= 2; page++ {
		br, bc, bok := blit.PageRect(page)
		hr, hc, hok := hosted.PageRect(page)
		if bok != hok || br != hr || bc != hc {
			t.Errorf("page %d: blitted %+v/%+v ok=%v, hosted %+v/%+v ok=%v",
				page, br, bc, bok, hr, hc, hok)
		}
	}
}

// The card is still painted — paper, shadow and border — so a hosted page looks
// like a page while the host puts the real thing on top. What is absent is the
// content.
func TestHostedPagePaintsPaperButNoContent(t *testing.T) {
	blit := NewPagedView([]*image.RGBA{solidPage(80, 100, pgRed)})
	blit.SetBounds(Rect{W: 240, H: 260})
	if !bufHasRGB(hostedRender(blit, 240, 260), pgRed) {
		t.Fatal("control: a blitted page must put its own pixels on screen")
	}

	hosted := NewPagedView(nil)
	hosted.SetPageSizes([]image.Point{{X: 80, Y: 100}})
	hosted.SetBounds(Rect{W: 240, H: 260})
	buf := hostedRender(hosted, 240, 260)
	if bufHasRGB(buf, pgRed) {
		t.Error("a hosted page must not paint content it does not have")
	}
	if !bufHasRGB(buf, DefaultLight().Surface) {
		t.Error("a hosted page must still paint its paper")
	}
	if !bufHasRGB(buf, DefaultLight().Border) {
		t.Error("a hosted page must still paint its border")
	}
}

// Zoom, fit-width and the page rectangle all read the declared size, so a hosted
// page behaves under the toolbar exactly like a blitted one.
func TestHostedPageZoomAndFit(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 100, Y: 200}})
	pv.SetBounds(Rect{W: 400, H: 400})

	before, _, _ := pv.PageRect(1)
	pv.Zoom().Set(200)
	after, _, ok := pv.PageRect(1)
	if !ok || after.W != 200 {
		t.Errorf("card width at 200%% = %d, want 200 (%+v)", after.W, after)
	}
	if after.W <= before.W {
		t.Errorf("zooming in must widen a hosted card: %d -> %d", before.W, after.W)
	}

	pv.Zoom().Set(100)
	pv.fitWidth()
	if pv.Zoom().Get() <= 100 {
		t.Errorf("fit-width must zoom a 100px page into a 400px pane, got %d%%", pv.Zoom().Get())
	}
}

// Both setters clamp the current page the same way, and each clears the other's
// notion of what a page is.
func TestSetPageSizesAndSetPagesAreExclusive(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(10, 10, pgRed), solidPage(10, 10, pgGreen)})
	pv.SetBounds(Rect{W: 100, H: 100})
	pv.CurrentPage().Set(2)

	pv.SetPageSizes([]image.Point{{X: 10, Y: 10}})
	if got := pv.CurrentPage().Get(); got != 1 {
		t.Errorf("current page = %d, want it clamped to 1", got)
	}
	if pv.pageImage(0) != nil {
		t.Error("declaring hosted pages must drop the bitmaps")
	}

	pv.SetPageSizes(nil)
	if pv.PageCount() != 0 || pv.CurrentPage().Get() != 1 {
		t.Errorf("an empty hosted set must read as empty at page 1: %d pages, page %d",
			pv.PageCount(), pv.CurrentPage().Get())
	}

	pv.SetPages([]*image.RGBA{solidPage(10, 10, pgBlue)})
	if pv.pageImage(0) == nil {
		t.Error("SetPages must restore a blitted page")
	}
	if pv.pageImage(-1) != nil || pv.pageImage(9) != nil {
		t.Error("an out-of-range page has no bitmap")
	}
}

// A page declared with no size is laid out as nothing rather than crashing, the
// way a nil bitmap always was.
func TestHostedPageWithNoSize(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetPageSizes([]image.Point{{X: 0, Y: 0}, {X: 40, Y: 40}})
	pv.SetBounds(Rect{W: 200, H: 200})
	if pv.PageCount() != 2 {
		t.Fatalf("page count = %d, want 2", pv.PageCount())
	}
	hostedRender(pv, 200, 200) // must not panic
	if _, clip, ok := pv.PageRect(1); ok && (clip.W > 0 || clip.H > 0) {
		t.Errorf("a zero-size page shows nothing: %+v", clip)
	}
}
