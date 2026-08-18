// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"testing"

	"github.com/go-iconoir/iconoir"
	"github.com/go-widgets/painter"
)

// pvSurface returns a fresh w×h RGBA buffer + a PixelPainter over it.
func pvSurface(w, h int) ([]byte, *painter.PixelPainter) {
	buf := make([]byte, 4*w*h)
	return buf, painter.NewPixelPainter(buf, w, h)
}

// solidPage builds a w×h opaque RGBA page filled with c, so a test can find its
// pixels in a rendered buffer.
func solidPage(w, h int, c RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i+3 < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

// bufHasRGB reports whether any pixel in buf carries exactly c's RGB.
func bufHasRGB(buf []byte, c RGBA) bool {
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] == c.R && buf[i+1] == c.G && buf[i+2] == c.B {
			return true
		}
	}
	return false
}

var (
	pgRed   = RGB(0xFF, 0x00, 0x00)
	pgGreen = RGB(0x00, 0xFF, 0x00)
	pgBlue  = RGB(0x00, 0x00, 0xFF)
)

// clickButton dispatches a widget-local EventClick at b's centre.
func clickButton(pv *PagedView, b *Button) {
	r := pv.Bounds()
	bb := b.Bounds()
	pv.OnEvent(Event{Kind: EventClick, X: bb.X + bb.W/2 - r.X, Y: bb.Y + bb.H/2 - r.Y})
}

func TestPagedViewConstructorDefaults(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(40, 40, pgRed), solidPage(40, 40, pgGreen)})
	if pv.Mode().Get() != PagedContinuous {
		t.Fatalf("mode = %v, want continuous", pv.Mode().Get())
	}
	if pv.CurrentPage().Get() != 1 {
		t.Fatalf("current = %d, want 1", pv.CurrentPage().Get())
	}
	if pv.Zoom().Get() != pagedZoomDefault {
		t.Fatalf("zoom = %d, want %d", pv.Zoom().Get(), pagedZoomDefault)
	}
	if pv.PageCount() != 2 {
		t.Fatalf("count = %d, want 2", pv.PageCount())
	}
	if got := pv.percentText(); got != "100%" {
		t.Fatalf("percentText = %q", got)
	}
}

// TestPagedViewZeroValueLazyAccessors exercises the nil-branch of every
// Observable accessor (and ensure) on a bare &PagedView{}.
func TestPagedViewZeroValueLazyAccessors(t *testing.T) {
	pv := &PagedView{}
	if pv.Mode().Get() != PagedContinuous {
		t.Fatal("zero-value mode default wrong")
	}
	if pv.CurrentPage().Get() != 1 {
		t.Fatal("zero-value current default wrong")
	}
	if pv.Zoom().Get() != pagedZoomDefault {
		t.Fatal("zero-value zoom default wrong")
	}
	// Non-nil branch on a second call.
	if pv.Mode() == nil || pv.CurrentPage() == nil || pv.Zoom() == nil {
		t.Fatal("accessor returned nil on second call")
	}
	// Drawing a zero-value view (empty, ensure lazily builds) must not panic.
	pv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	_, p := pvSurface(200, 200)
	pv.Draw(p, DefaultLight())
}

func TestPagedViewSetPagesClamps(t *testing.T) {
	pv := NewPagedView(nil)
	// Empty set → current pinned to 1.
	if pv.CurrentPage().Get() != 1 {
		t.Fatalf("empty current = %d", pv.CurrentPage().Get())
	}
	// Past-the-end current clamps down to the last page.
	pv.CurrentPage().Set(9)
	pv.SetPages([]*image.RGBA{solidPage(10, 10, pgRed), solidPage(10, 10, pgGreen)})
	if pv.CurrentPage().Get() != 2 {
		t.Fatalf("past-end clamp = %d, want 2", pv.CurrentPage().Get())
	}
	// Below-1 current clamps up to 1.
	pv.CurrentPage().Set(0)
	pv.SetPages([]*image.RGBA{solidPage(10, 10, pgRed), solidPage(10, 10, pgGreen)})
	if pv.CurrentPage().Get() != 1 {
		t.Fatalf("below-1 clamp = %d, want 1", pv.CurrentPage().Get())
	}
	// In-range current is left alone.
	pv.CurrentPage().Set(2)
	pv.SetPages([]*image.RGBA{solidPage(10, 10, pgRed), solidPage(10, 10, pgGreen)})
	if pv.CurrentPage().Get() != 2 {
		t.Fatalf("in-range = %d, want 2", pv.CurrentPage().Get())
	}
}

// TestPagedViewContinuousDrawsSeveral proves continuous mode paints every page
// card (both colours appear).
func TestPagedViewContinuousDrawsSeveral(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(60, 60, pgRed), solidPage(60, 60, pgGreen)})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	buf, p := pvSurface(300, 300)
	pv.Draw(p, DefaultLight())
	if !bufHasRGB(buf, pgRed) {
		t.Fatal("continuous: page 1 (red) not drawn")
	}
	if !bufHasRGB(buf, pgGreen) {
		t.Fatal("continuous: page 2 (green) not drawn")
	}
}

// TestPagedViewPaginatedDrawsOne proves paginated mode paints ONLY the current
// page (the others are absent), and the "n / N" indicator branch runs.
func TestPagedViewPaginatedDrawsOne(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(60, 60, pgRed), solidPage(60, 60, pgGreen)})
	pv.Mode().Set(PagedPaginated)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	buf, p := pvSurface(300, 300)
	pv.Draw(p, DefaultLight())
	if !bufHasRGB(buf, pgRed) {
		t.Fatal("paginated: current page (red) not drawn")
	}
	if bufHasRGB(buf, pgGreen) {
		t.Fatal("paginated: non-current page (green) leaked into the render")
	}
	if got := pv.pageIndicator(); got != "1 / 2" {
		t.Fatalf("indicator = %q, want '1 / 2'", got)
	}
}

// TestPagedViewNilPageSkipped covers the nil-page skip in drawPages + scaledSize.
func TestPagedViewNilPageSkipped(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(50, 50, pgRed), nil})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	buf, p := pvSurface(300, 300)
	pv.Draw(p, DefaultLight()) // must not panic on the nil page
	if !bufHasRGB(buf, pgRed) {
		t.Fatal("nil-page case: the real page did not draw")
	}
}

// TestPagedViewKeyNavigation walks every nav key + the default (ignored) key.
func TestPagedViewKeyNavigation(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{
		solidPage(40, 40, pgRed), solidPage(40, 40, pgGreen), solidPage(40, 40, pgBlue),
	})
	pv.Mode().Set(PagedPaginated)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	key := func(code string) { pv.OnEvent(Event{Kind: EventKeyDown, Code: code}) }

	key("PageDown")
	if pv.cur() != 2 {
		t.Fatalf("PageDown → %d, want 2", pv.cur())
	}
	key("ArrowUp")
	if pv.cur() != 1 {
		t.Fatalf("ArrowUp → %d, want 1", pv.cur())
	}
	key("End")
	if pv.cur() != 3 {
		t.Fatalf("End → %d, want 3", pv.cur())
	}
	key("PageDown") // clamp at end, no wrap
	if pv.cur() != 3 {
		t.Fatalf("PageDown at end → %d, want 3 (no wrap)", pv.cur())
	}
	key("Home")
	if pv.cur() != 1 {
		t.Fatalf("Home → %d, want 1", pv.cur())
	}
	key("ArrowUp") // clamp at start, no wrap
	if pv.cur() != 1 {
		t.Fatalf("ArrowUp at start → %d, want 1 (no wrap)", pv.cur())
	}
	key(" ") // Space → next
	if pv.cur() != 2 {
		t.Fatalf("Space → %d, want 2", pv.cur())
	}
	key("ArrowDown")
	if pv.cur() != 3 {
		t.Fatalf("ArrowDown → %d, want 3", pv.cur())
	}
	key("PageUp")
	if pv.cur() != 2 {
		t.Fatalf("PageUp → %d, want 2", pv.cur())
	}
	key("Space") // the literal "Space" code
	if pv.cur() != 3 {
		t.Fatalf("'Space' → %d, want 3", pv.cur())
	}
	key("Backspace") // unknown → no-op
	if pv.cur() != 3 {
		t.Fatalf("unknown key changed page to %d", pv.cur())
	}
}

// TestPagedViewKeyNavContinuous covers setPage's continuous branch
// (scrollCardIntoView).
func TestPagedViewKeyNavContinuous(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{
		solidPage(60, 200, pgRed), solidPage(60, 200, pgGreen), solidPage(60, 200, pgBlue),
	})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 150})
	pv.OnEvent(Event{Kind: EventKeyDown, Code: "PageDown"})
	if pv.cur() != 2 {
		t.Fatalf("continuous PageDown → %d, want 2", pv.cur())
	}
	if pv.scroll.OffsetY <= 0 {
		t.Fatal("continuous nav did not scroll the card into view")
	}
}

// TestPagedViewEmptyNavNoop covers setPage's cnt==0 early return.
func TestPagedViewEmptyNavNoop(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	pv.OnEvent(Event{Kind: EventKeyDown, Code: "PageDown"})
	if pv.cur() != 1 {
		t.Fatalf("empty nav changed page to %d", pv.cur())
	}
	// Drawing an empty view (empty layout branches) must not panic.
	_, p := pvSurface(200, 200)
	pv.Draw(p, DefaultLight())
}

// TestPagedViewToolbarClicks drives the mode + zoom buttons through real clicks.
func TestPagedViewToolbarClicks(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(100, 100, pgRed), solidPage(100, 100, pgGreen)})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})
	// Switch to paginated via its button.
	clickButton(pv, pv.btnPaginated)
	if pv.Mode().Get() != PagedPaginated {
		t.Fatal("paginated button did not switch mode")
	}
	// Clicking the already-active mode is a no-op (setMode unchanged branch).
	clickButton(pv, pv.btnPaginated)
	if pv.Mode().Get() != PagedPaginated {
		t.Fatal("re-click flipped mode")
	}
	// Back to continuous (setMode change → continuous branch).
	clickButton(pv, pv.btnContinuous)
	if pv.Mode().Get() != PagedContinuous {
		t.Fatal("continuous button did not switch mode")
	}
	// Zoom in / out.
	clickButton(pv, pv.btnZoomIn)
	if pv.Zoom().Get() != pagedZoomDefault+pagedZoomStep {
		t.Fatalf("zoom-in → %d", pv.Zoom().Get())
	}
	clickButton(pv, pv.btnZoomOut)
	if pv.Zoom().Get() != pagedZoomDefault {
		t.Fatalf("zoom-out → %d", pv.Zoom().Get())
	}
}

// TestPagedViewPagingButtons covers the prev/next toolbar buttons (paginated).
func TestPagedViewPagingButtons(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{
		solidPage(40, 40, pgRed), solidPage(40, 40, pgGreen), solidPage(40, 40, pgBlue),
	})
	pv.Mode().Set(PagedPaginated)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})
	clickButton(pv, pv.btnNext)
	if pv.cur() != 2 {
		t.Fatalf("next button → %d, want 2", pv.cur())
	}
	clickButton(pv, pv.btnPrev)
	if pv.cur() != 1 {
		t.Fatalf("prev button → %d, want 1", pv.cur())
	}
	// Prev at page 1 is disabled (setPage n==cur after clamp) → no change.
	clickButton(pv, pv.btnPrev)
	if pv.cur() != 1 {
		t.Fatalf("disabled prev changed page to %d", pv.cur())
	}
}

// TestPagedViewZoomClamps covers setZoom's min + max clamps.
func TestPagedViewZoomClamps(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(50, 50, pgRed)})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	for i := 0; i < 40; i++ {
		pv.zoomOut()
	}
	if pv.Zoom().Get() != pagedZoomMin {
		t.Fatalf("zoom floor = %d, want %d", pv.Zoom().Get(), pagedZoomMin)
	}
	for i := 0; i < 40; i++ {
		pv.zoomIn()
	}
	if pv.Zoom().Get() != pagedZoomMax {
		t.Fatalf("zoom ceil = %d, want %d", pv.Zoom().Get(), pagedZoomMax)
	}
}

// TestPagedViewFitWidth covers fitWidth's normal path + the zoom change it makes.
func TestPagedViewFitWidth(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(100, 100, pgRed)})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})
	before := pv.Zoom().Get()
	clickButton(pv, pv.btnFit)
	after := pv.Zoom().Get()
	if after == before {
		t.Fatalf("fit did not change zoom (still %d)", after)
	}
	// The current page's fitted width must not exceed the usable pane width.
	usableW := pv.scroll.Bounds().W - scrollGutter() - 2*scaled(pagedMargin)
	if 100*after/100 > usableW+1 {
		t.Fatalf("fitted width %d exceeds usable %d", 100*after/100, usableW)
	}
}

// TestPagedViewFitWidthGuards covers fitWidth's empty + zero-width + tiny-pane guards.
func TestPagedViewFitWidthGuards(t *testing.T) {
	// Empty: no current page.
	empty := NewPagedView(nil)
	empty.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	empty.fitWidth()
	if empty.Zoom().Get() != pagedZoomDefault {
		t.Fatal("fit on empty changed zoom")
	}
	// Zero-width page: w <= 0 guard.
	zw := NewPagedView([]*image.RGBA{solidPage(0, 30, pgRed)})
	zw.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	zw.fitWidth()
	if zw.Zoom().Get() != pagedZoomDefault {
		t.Fatal("fit on zero-width page changed zoom")
	}
	// Tiny pane: usableW < 1 floor still yields a (clamped) zoom.
	tiny := NewPagedView([]*image.RGBA{solidPage(100, 100, pgRed)})
	tiny.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 80})
	tiny.fitWidth()
	if tiny.Zoom().Get() != pagedZoomMin {
		t.Fatalf("fit in tiny pane → %d, want floor %d", tiny.Zoom().Get(), pagedZoomMin)
	}
}

// TestPagedViewWheelFlipShortPage: a page not taller than the pane flips on a
// single wheel notch at either edge (atTop && atBottom).
func TestPagedViewWheelFlipShortPage(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{
		solidPage(40, 40, pgRed), solidPage(40, 40, pgGreen), solidPage(40, 40, pgBlue),
	})
	pv.Mode().Set(PagedPaginated)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	pv.OnEvent(Event{Kind: EventScroll, Delta: 1}) // down past bottom → next
	if pv.cur() != 2 {
		t.Fatalf("wheel down → %d, want 2", pv.cur())
	}
	pv.OnEvent(Event{Kind: EventScroll, Delta: -1}) // up past top → prev
	if pv.cur() != 1 {
		t.Fatalf("wheel up → %d, want 1", pv.cur())
	}
	pv.OnEvent(Event{Kind: EventScroll, Delta: -1}) // at first, no wrap
	if pv.cur() != 1 {
		t.Fatalf("wheel up at first → %d, want 1", pv.cur())
	}
}

// TestPagedViewWheelScrollWithinThenFlip: a page taller than the pane scrolls
// WITHIN first and only flips at the bottom edge.
func TestPagedViewWheelScrollWithinThenFlip(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{
		solidPage(60, 800, pgRed), solidPage(60, 800, pgGreen),
	})
	pv.Mode().Set(PagedPaginated)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 150})
	// Not yet at the bottom: a downward notch scrolls within, page unchanged.
	pv.OnEvent(Event{Kind: EventScroll, Delta: 1})
	if pv.cur() != 1 {
		t.Fatalf("scroll-within advanced the page to %d", pv.cur())
	}
	if pv.scroll.OffsetY <= 0 {
		t.Fatal("scroll-within did not move the offset")
	}
	// Pin to the bottom, then a downward notch flips to the next page (top).
	pv.scroll.OffsetY = pv.scroll.maxOffsetY()
	pv.OnEvent(Event{Kind: EventScroll, Delta: 1})
	if pv.cur() != 2 {
		t.Fatalf("edge flip → %d, want 2", pv.cur())
	}
	if pv.scroll.OffsetY != 0 {
		t.Fatalf("next page did not land at top (offset %d)", pv.scroll.OffsetY)
	}
}

// TestPagedViewWheelUpNotAtTopScrolls covers onScroll's "Delta<0 but not atTop"
// fall-through (scroll within, no flip).
func TestPagedViewWheelUpNotAtTopScrolls(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(60, 800, pgRed), solidPage(60, 800, pgGreen)})
	pv.Mode().Set(PagedPaginated)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 150})
	pv.scroll.OffsetY = pv.scroll.maxOffsetY() // away from the top
	start := pv.scroll.OffsetY
	pv.OnEvent(Event{Kind: EventScroll, Delta: -1})
	if pv.cur() != 1 {
		t.Fatalf("up-scroll flipped page to %d", pv.cur())
	}
	if pv.scroll.OffsetY >= start {
		t.Fatal("up-scroll did not move within the page")
	}
}

// TestPagedViewWheelUpEdgeFlipToBottom covers setPage's paginated toBottom
// branch: an up-flip lands the previous page at its bottom.
func TestPagedViewWheelUpEdgeFlipToBottom(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(60, 800, pgRed), solidPage(60, 800, pgGreen)})
	pv.Mode().Set(PagedPaginated)
	pv.CurrentPage().Set(2)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 150})
	pv.scroll.OffsetY = 0 // at the top of page 2
	pv.OnEvent(Event{Kind: EventScroll, Delta: -1})
	if pv.cur() != 1 {
		t.Fatalf("up-flip → %d, want 1", pv.cur())
	}
	if pv.scroll.OffsetY != pv.scroll.maxOffsetY() {
		t.Fatalf("prev page did not land at bottom (offset %d, max %d)",
			pv.scroll.OffsetY, pv.scroll.maxOffsetY())
	}
}

// TestPagedViewContinuousWheelScrolls covers onScroll's continuous fall-through.
func TestPagedViewContinuousWheelScrolls(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(60, 400, pgRed), solidPage(60, 400, pgGreen)})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 150})
	pv.OnEvent(Event{Kind: EventScroll, Delta: 3})
	if pv.scroll.OffsetY <= 0 {
		t.Fatal("continuous wheel did not scroll the stack")
	}
	if pv.cur() != 1 {
		t.Fatalf("continuous wheel changed page to %d", pv.cur())
	}
}

// TestPagedViewClickPaneForwards covers clickToolbar miss → forwardToScroll, and
// a non-click pointer event forwarding (default branch).
func TestPagedViewClickPaneForwards(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(60, 600, pgRed)})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	// A click well inside the pane (below the toolbar) hits no button.
	pv.OnEvent(Event{Kind: EventClick, X: 150, Y: 120})
	// A drag + release forward to the ScrollView (default branch).
	pv.OnEvent(Event{Kind: EventMouseDrag, X: 150, Y: 100})
	pv.OnEvent(Event{Kind: EventMouseUp, X: 150, Y: 100})
}

// TestPagedViewDisabledIgnoresEvents covers OnEvent's Disabled early return.
func TestPagedViewDisabledIgnoresEvents(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(40, 40, pgRed), solidPage(40, 40, pgGreen)})
	pv.Mode().Set(PagedPaginated)
	pv.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	pv.Disabled = true
	pv.OnEvent(Event{Kind: EventKeyDown, Code: "PageDown"})
	if pv.cur() != 1 {
		t.Fatalf("disabled view navigated to %d", pv.cur())
	}
}

// TestPagedViewHitTest covers the HitTest override.
func TestPagedViewHitTest(t *testing.T) {
	pv := NewPagedView(nil)
	pv.SetBounds(Rect{X: 10, Y: 10, W: 100, H: 100})
	if !pv.HitTest(50, 50) {
		t.Fatal("HitTest missed a point inside bounds")
	}
	if pv.HitTest(5, 5) {
		t.Fatal("HitTest hit a point outside bounds")
	}
}

// TestPagedViewComputeLayout drives every branch of computeLayout directly.
func TestPagedViewComputeLayout(t *testing.T) {
	// Negative usable dims clamp to zero (paginated, empty).
	empty := NewPagedView(nil)
	empty.Mode().Set(PagedPaginated)
	lay := empty.computeLayout(-5, -5)
	if lay.contentW != 0 || lay.contentH != 0 || len(lay.cards) != 0 {
		t.Fatalf("empty paginated negative = %+v", lay)
	}

	// Paginated, small page: content grows to the usable size (cw/ch < usable).
	small := NewPagedView([]*image.RGBA{solidPage(20, 20, pgRed)})
	small.Mode().Set(PagedPaginated)
	lay = small.computeLayout(200, 200)
	if lay.contentW != 200 || lay.contentH != 200 || len(lay.cards) != 1 {
		t.Fatalf("small paginated = %+v", lay)
	}

	// Paginated, large page: content is the card size (cw/ch >= usable).
	large := NewPagedView([]*image.RGBA{solidPage(500, 500, pgRed)})
	large.Mode().Set(PagedPaginated)
	lay = large.computeLayout(100, 100)
	margin := scaled(pagedMargin)
	if lay.contentW != 500+2*margin || lay.contentH != 500+2*margin {
		t.Fatalf("large paginated content = %dx%d", lay.contentW, lay.contentH)
	}

	// Continuous, empty.
	ce := NewPagedView(nil)
	lay = ce.computeLayout(150, 150)
	if lay.contentW != 150 || lay.contentH != 150 || len(lay.cards) != 0 {
		t.Fatalf("continuous empty = %+v", lay)
	}

	// Continuous, two pages of differing widths → maxW update; short stack grows
	// to usable height, wide-enough content stays its own width.
	cont := NewPagedView([]*image.RGBA{solidPage(30, 20, pgRed), solidPage(80, 20, pgGreen)})
	lay = cont.computeLayout(60, 500)
	if len(lay.cards) != 2 {
		t.Fatalf("continuous cards = %d", len(lay.cards))
	}
	if lay.contentW != 80+2*margin { // widest page + margins, exceeds usable 60
		t.Fatalf("continuous contentW = %d", lay.contentW)
	}
	if lay.contentH != 500 { // short stack clamps up to usable height
		t.Fatalf("continuous contentH = %d, want 500", lay.contentH)
	}
	// Tall stack keeps its own height (ch >= usable).
	lay = cont.computeLayout(200, 10)
	if lay.contentH <= 10 {
		t.Fatalf("tall stack contentH = %d, want > 10", lay.contentH)
	}
}

// TestIconGlyphRect covers both branches of iconGlyphRect.
func TestIconGlyphRect(t *testing.T) {
	// Normal square button: inset on every edge, positive glyph.
	g := iconGlyphRect(Rect{X: 0, Y: 0, W: 24, H: 24})
	if g.W != 24-2*scaled(pagedIconInset) || g.W <= 0 {
		t.Fatalf("normal glyph = %+v", g)
	}
	// Tiny button: the s < 1 floor kicks in.
	g = iconGlyphRect(Rect{X: 0, Y: 0, W: 2, H: 2})
	if g.W != 1 {
		t.Fatalf("tiny glyph W = %d, want floor 1", g.W)
	}
}

// TestPagedViewLayoutToolbarTiny covers layoutToolbar's sz < 1 floor via a
// direct call with a tiny toolbar height.
func TestPagedViewLayoutToolbarTiny(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(10, 10, pgRed)})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	pv.layoutToolbar(pv.Bounds(), 2) // tbH=2 → sz = 2-2*pad < 1
	if pv.btnContinuous.Bounds().W != 1 {
		t.Fatalf("tiny toolbar button W = %d, want floor 1", pv.btnContinuous.Bounds().W)
	}
}

// TestPagedViewTinyBoundsDraw covers relayout's pane.H<0 clamp + computeLayout
// negative-usable clamp through the public draw path, and never panics.
func TestPagedViewTinyBoundsDraw(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(10, 10, pgRed)})
	pv.SetBounds(Rect{X: 0, Y: 0, W: 5, H: 5}) // pane.H = 5-30 < 0
	_, p := pvSurface(5, 5)
	pv.Draw(p, DefaultLight())
}

// TestPagedViewA11y covers both branches of A11y (with + without pages).
func TestPagedViewA11y(t *testing.T) {
	pv := NewPagedView([]*image.RGBA{solidPage(10, 10, pgRed), solidPage(10, 10, pgGreen)})
	pv.CurrentPage().Set(2)
	info := pv.A11y()
	if info.Role != RoleDocument {
		t.Fatalf("role = %q, want document", info.Role)
	}
	if info.Value != "2 / 2" {
		t.Fatalf("value = %q, want '2 / 2'", info.Value)
	}
	if !info.HasRange || info.Min != 1 || info.Max != 2 || info.Now != 2 {
		t.Fatalf("range = %+v", info)
	}
	// Empty view: no numeric range.
	empty := NewPagedView(nil).A11y()
	if empty.HasRange {
		t.Fatalf("empty view should not report a range: %+v", empty)
	}
}

// TestPagedViewIconsExist confirms every toolbar icon name resolves in iconoir.
func TestPagedViewIconsExist(t *testing.T) {
	for _, name := range []string{
		"zoom-in", "zoom-out", "expand",
		"nav-arrow-left", "nav-arrow-right",
		"multiple-pages", "page",
	} {
		if _, ok := iconoir.Get(name); !ok {
			t.Fatalf("iconoir has no icon %q", name)
		}
	}
}
