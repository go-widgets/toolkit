// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"testing"
)

// deliverPage opens a single tab and delivers a render imgW×imgH so the caller
// can force vertical and/or (under zoom) horizontal overflow. The pixels are a
// per-column gradient (red = column & 0xFF) so a horizontal-scroll blit can be
// checked positionally; links is passed through.
func deliverPage(b *Browser, imgW, imgH int, links []BrowserLink) {
	b.Open("http://a", "A")
	px := make([]byte, imgW*imgH*4)
	for y := 0; y < imgH; y++ {
		for x := 0; x < imgW; x++ {
			o := (y*imgW + x) * 4
			px[o] = byte(x)   // R encodes the page column
			px[o+1] = byte(y) // G encodes the page row
			px[o+2] = 0x40    // constant B so a page pixel never equals the Accent/SurfaceAlt chrome
			px[o+3] = 0xFF
		}
	}
	b.Deliver("http://a", px, imgW, imgH, imgW, links, "A")
}

// TestBrowserVerticalScrollbarDrawTracksScroll delivers a page 3× the content
// height and asserts the vertical bar draws on the content's right edge with an
// Accent thumb over a SurfaceAlt track, the thumb sitting at the track top at
// scroll=0 and at the track bottom at scroll=max.
func TestBrowserVerticalScrollbarDrawTracksScroll(t *testing.T) {
	theme := DefaultLight()
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*3, nil) // vertical overflow, no horizontal (zoom 1)

	tab := b.activeTab()
	g, ok := b.vscrollGeom(tab, cr)
	if !ok {
		t.Fatal("vertical scrollbar should be live for a 3×-tall page")
	}
	if g.horizontal {
		t.Error("vertical geometry marked horizontal")
	}
	if g.cross0 != cr.W-scrollbarWidth {
		t.Errorf("vbar cross0 = %d, want cr.W-scrollbarWidth = %d", g.cross0, cr.W-scrollbarWidth)
	}
	if g.thumbLen >= g.trackLen || g.thumbLen < scrollbarWidth {
		t.Errorf("vbar thumbLen = %d, want in [scrollbarWidth, trackLen=%d)", g.thumbLen, g.trackLen)
	}

	colX := cr.X + g.cross0 + scrollbarWidth/2
	draw := func() []byte {
		buf := make([]byte, browserBounds.W*browserBounds.H*4)
		b.Draw(newP(buf, browserBounds.W), theme)
		return buf
	}

	// scroll=0: thumb hugs the track top; the track's far end is bare SurfaceAlt.
	// The thumb is the shared muted grey; sample interior points (the rounded caps
	// taper at the very ends).
	muted := scrollbarThumbColor(theme)
	buf := draw()
	if got := pixelAt(buf, browserBounds.W, colX, cr.Y+g.thumbStart+2); got != muted {
		t.Errorf("scroll=0: thumb-top pixel = %+v, want muted thumb %+v", got, muted)
	}
	if got := pixelAt(buf, browserBounds.W, colX, cr.Y+g.trackLen-scrollbarWidth); got != theme.SurfaceAlt {
		t.Errorf("scroll=0: track-bottom pixel = %+v, want SurfaceAlt %+v", got, theme.SurfaceAlt)
	}

	// scroll=max: thumb hugs the track bottom; the track top is now bare.
	tab.scroll = b.maxScroll(tab, cr)
	g2, _ := b.vscrollGeom(tab, cr)
	if g2.thumbStart+g2.thumbLen != g2.trackLen {
		t.Errorf("scroll=max: thumb end %d != trackLen %d (thumb not pinned to bottom)", g2.thumbStart+g2.thumbLen, g2.trackLen)
	}
	buf = draw()
	if got := pixelAt(buf, browserBounds.W, colX, cr.Y+g2.trackStart+2); got != theme.SurfaceAlt {
		t.Errorf("scroll=max: track-top pixel = %+v, want SurfaceAlt %+v", got, theme.SurfaceAlt)
	}
	if got := pixelAt(buf, browserBounds.W, colX, cr.Y+g2.thumbStart+2); got != muted {
		t.Errorf("scroll=max: thumb-bottom pixel = %+v, want muted thumb %+v", got, muted)
	}
}

// TestBrowserHideScrollbarSuppressesPaint checks HideScrollbar stops the Browser
// painting its own bar, so a host can overlay its own matching one: the muted
// thumb a visible bar draws is gone once the flag is set.
func TestBrowserHideScrollbarSuppressesPaint(t *testing.T) {
	theme := DefaultLight()
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*3, nil) // vertical overflow
	tab := b.activeTab()
	g, ok := b.vscrollGeom(tab, cr)
	if !ok {
		t.Fatal("vertical scrollbar should be live for a 3×-tall page")
	}
	colX := cr.X + g.cross0 + scrollbarWidth/2
	thumbY := cr.Y + g.thumbStart + 2
	muted := scrollbarThumbColor(theme)
	draw := func() []byte {
		buf := make([]byte, browserBounds.W*browserBounds.H*4)
		b.Draw(newP(buf, browserBounds.W), theme)
		return buf
	}

	// Visible: the thumb paints in the shared muted grey.
	if got := pixelAt(draw(), browserBounds.W, colX, thumbY); got != muted {
		t.Fatalf("visible bar: thumb pixel = %+v, want muted thumb %+v", got, muted)
	}
	// Hidden: the same pixel is now the page, never the thumb chrome.
	b.HideScrollbar = true
	if got := pixelAt(draw(), browserBounds.W, colX, thumbY); got == muted {
		t.Fatalf("hidden bar: thumb pixel still painted %+v (bar not suppressed)", got)
	}
}

// TestBrowserScrollExtent checks the vertical extent a HideScrollbar host reads
// to size its own bar: not-shown with no tab and when the page fits, and the live
// offset/viewport/total once the page overflows.
func TestBrowserScrollExtent(t *testing.T) {
	// No tab yet: nothing to report.
	nb, _, _, _ := newTestBrowser()
	if off, vp, tot, shown := nb.ScrollExtent(); shown || off != 0 || vp != 0 || tot != 0 {
		t.Fatalf("no-tab ScrollExtent = (%d,%d,%d,%v), want (0,0,0,false)", off, vp, tot, shown)
	}

	// A page that fits reports its viewport/total but is not shown.
	fb, _, _, _ := newTestBrowser()
	crf := fb.contentRect()
	deliverPage(fb, crf.W, crf.H/2, nil)
	if off, vp, tot, shown := fb.ScrollExtent(); shown || vp != crf.H || tot != crf.H/2 {
		t.Fatalf("fitting ScrollExtent = (%d,%d,%d,%v), want (_,%d,%d,false)", off, vp, tot, shown, crf.H, crf.H/2)
	}

	// A 3×-tall page overflows; the offset tracks the tab's scroll.
	ob, _, _, _ := newTestBrowser()
	cro := ob.contentRect()
	deliverPage(ob, cro.W, cro.H*3, nil)
	ob.activeTab().scroll = 40
	off, vp, tot, shown := ob.ScrollExtent()
	if !shown || off != 40 || vp != cro.H || tot != cro.H*3 {
		t.Fatalf("overflow ScrollExtent = (%d,%d,%d,%v), want (40,%d,%d,true)", off, vp, tot, shown, cro.H, cro.H*3)
	}
}

// TestBrowserVerticalThumbDrag grabs the thumb and drags it, asserting the tab
// scroll follows to the inverse-mapped offset, and that a drag after release is
// ignored.
func TestBrowserVerticalThumbDrag(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*3, nil)
	tab := b.activeTab()
	g, _ := b.vscrollGeom(tab, cr)

	px := cr.X + g.cross0 + scrollbarWidth/2
	pyThumb := cr.Y + g.thumbStart + g.thumbLen/2
	b.OnEvent(Event{Kind: EventClick, X: px, Y: pyThumb}) // grab
	if tab.scroll != 0 {
		t.Fatalf("grabbing the thumb must not move scroll; got %d", tab.scroll)
	}
	dy := 40
	b.OnEvent(Event{Kind: EventMouseDrag, X: px, Y: pyThumb + dy})
	want := g.scrollForGrabStart(g.thumbStart + dy)
	if tab.scroll != want {
		t.Errorf("after drag: scroll = %d, want %d", tab.scroll, want)
	}
	if tab.scroll <= 0 {
		t.Error("dragging the thumb down should scroll the page down")
	}

	// Release, then a further drag must not move anything.
	b.OnEvent(Event{Kind: EventMouseUp, X: px, Y: pyThumb + dy})
	frozen := tab.scroll
	b.OnEvent(Event{Kind: EventMouseDrag, X: px, Y: pyThumb + dy + 40})
	if tab.scroll != frozen {
		t.Errorf("drag after release moved scroll %d → %d", frozen, tab.scroll)
	}
}

// TestBrowserVerticalTrackPaging presses the track off the thumb and asserts a
// one-page step toward the click (down below the thumb, up above it).
func TestBrowserVerticalTrackPaging(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*3, nil)
	tab := b.activeTab()
	g, _ := b.vscrollGeom(tab, cr)
	px := cr.X + g.cross0 + scrollbarWidth/2

	// Press the track BELOW the thumb → page down by cr.H (clamped).
	belowY := cr.Y + g.thumbStart + g.thumbLen + 1
	b.OnEvent(Event{Kind: EventClick, X: px, Y: belowY})
	wantDown := cr.H
	if m := b.maxScroll(tab, cr); wantDown > m {
		wantDown = m
	}
	if tab.scroll != wantDown {
		t.Errorf("track page-down: scroll = %d, want %d", tab.scroll, wantDown)
	}

	// Press the track ABOVE the thumb → page up.
	tab.scroll = b.maxScroll(tab, cr)
	g2, _ := b.vscrollGeom(tab, cr)
	aboveY := cr.Y + g2.thumbStart - 1
	b.OnEvent(Event{Kind: EventClick, X: px, Y: aboveY})
	if tab.scroll >= b.maxScroll(tab, cr) {
		t.Errorf("track page-up did not reduce scroll; still %d", tab.scroll)
	}
}

// TestBrowserHorizontalScrollbarUnderZoom asserts the horizontal bar appears
// only when the page overflows horizontally (zoom > 1), draws on the bottom edge,
// and that its thumb drag moves scrollX.
func TestBrowserHorizontalScrollbarUnderZoom(t *testing.T) {
	theme := DefaultLight()
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*2, nil)
	tab := b.activeTab()

	// At zoom 1 the page is fit-to-width, so there is no horizontal overflow.
	if _, ok := b.hscrollGeom(tab, cr); ok {
		t.Fatal("horizontal scrollbar should be inert at zoom 1 (fit-to-width)")
	}

	b.SetZoom(2.0) // widen the page past the content column
	g, ok := b.hscrollGeom(tab, cr)
	if !ok {
		t.Fatal("horizontal scrollbar should be live at zoom 2")
	}
	if !g.horizontal || g.cross0 != cr.H-scrollbarWidth {
		t.Errorf("hbar cross0 = %d (horizontal=%v), want cr.H-scrollbarWidth = %d", g.cross0, g.horizontal, cr.H-scrollbarWidth)
	}
	// The bar draws along the bottom edge.
	buf := make([]byte, browserBounds.W*browserBounds.H*4)
	b.Draw(newP(buf, browserBounds.W), theme)
	rowY := cr.Y + g.cross0 + scrollbarWidth/2
	if got := pixelAt(buf, browserBounds.W, cr.X+g.thumbStart+2, rowY); got != scrollbarThumbColor(theme) {
		t.Errorf("hbar thumb pixel = %+v, want muted thumb %+v", got, scrollbarThumbColor(theme))
	}

	// Drag the horizontal thumb → scrollX advances.
	pyBar := cr.Y + g.cross0 + scrollbarWidth/2
	pxThumb := cr.X + g.thumbStart + g.thumbLen/2
	b.OnEvent(Event{Kind: EventClick, X: pxThumb, Y: pyBar})
	b.OnEvent(Event{Kind: EventMouseDrag, X: pxThumb + 30, Y: pyBar})
	if tab.scrollX != g.scrollForGrabStart(g.thumbStart+30) {
		t.Errorf("hthumb drag: scrollX = %d, want %d", tab.scrollX, g.scrollForGrabStart(g.thumbStart+30))
	}
	if tab.scrollX <= 0 {
		t.Error("dragging the horizontal thumb right should scroll the page right")
	}
}

// TestBrowserHorizontalTrackPaging presses the horizontal track off the thumb
// and asserts a one-page horizontal step toward the click.
func TestBrowserHorizontalTrackPaging(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*2, nil)
	b.SetZoom(2.0)
	tab := b.activeTab()
	g, _ := b.hscrollGeom(tab, cr)
	rowY := cr.Y + g.cross0 + scrollbarWidth/2

	// Press the track to the RIGHT of the thumb → page right by cr.W (clamped).
	rightX := cr.X + g.thumbStart + g.thumbLen + 1
	b.OnEvent(Event{Kind: EventClick, X: rightX, Y: rowY})
	want := cr.W
	if m := b.maxScrollX(tab, cr); want > m {
		want = m
	}
	if tab.scrollX != want {
		t.Errorf("h-track page-right: scrollX = %d, want %d", tab.scrollX, want)
	}
}

// TestBrowserBothScrollbarsClearCorner asserts that when both axes overflow each
// track is shortened by scrollbarWidth so the bottom-right corner stays clear.
func TestBrowserBothScrollbarsClearCorner(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*3, nil)
	b.SetZoom(2.0) // add horizontal overflow on top of the vertical
	tab := b.activeTab()

	gv, okv := b.vscrollGeom(tab, cr)
	gh, okh := b.hscrollGeom(tab, cr)
	if !okv || !okh {
		t.Fatalf("both bars should be live; vertical=%v horizontal=%v", okv, okh)
	}
	if gv.trackLen != cr.H-scrollbarWidth {
		t.Errorf("vbar track %d, want cr.H-scrollbarWidth = %d (corner not reserved)", gv.trackLen, cr.H-scrollbarWidth)
	}
	if gh.trackLen != cr.W-scrollbarWidth {
		t.Errorf("hbar track %d, want cr.W-scrollbarWidth = %d (corner not reserved)", gh.trackLen, cr.W-scrollbarWidth)
	}
}

// TestBrowserLinkHitTestWithScrollOffsets delivers a link occupying the whole
// page and asserts that after scrolling both axes a content click still resolves
// to it (linkAt accounts for scroll + scrollX), while a click on a scrollbar is
// consumed and never navigates.
func TestBrowserLinkHitTestWithScrollOffsets(t *testing.T) {
	b, nav, _, _ := newTestBrowser()
	cr := b.contentRect()
	// A page 2× the content on each axis (under zoom 2) with one full-page link.
	link := BrowserLink{Rect: image.Rect(0, 0, cr.W*2, cr.H*2), Href: "http://a/dest"}
	deliverPage(b, cr.W, cr.H*2, []BrowserLink{link})
	b.SetZoom(2.0)
	tab := b.activeTab()
	tab.scroll = 20
	tab.scrollX = 15

	// A click in the page body (away from either bar) navigates to the link.
	b.OnEvent(Event{Kind: EventClick, X: cr.X + 5, Y: cr.Y + 5})
	if len(*nav) == 0 || (*nav)[len(*nav)-1] != "http://a/dest" {
		t.Fatalf("content click did not navigate the offset link; nav=%v", *nav)
	}

	// A click on the vertical scrollbar is consumed (paging/grab), not a link nav.
	before := len(*nav)
	gv, _ := b.vscrollGeom(tab, cr)
	b.OnEvent(Event{Kind: EventClick, X: cr.X + gv.cross0 + scrollbarWidth/2, Y: cr.Y + gv.trackLen - 2})
	if len(*nav) != before {
		t.Errorf("a scrollbar press must not navigate; nav grew %d → %d", before, len(*nav))
	}
}

// TestBrowserDeliverResetsScrollOffsets scrolls both axes then delivers a fresh
// render and asserts both offsets reset to zero.
func TestBrowserDeliverResetsScrollOffsets(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*3, nil)
	b.SetZoom(2.0)
	tab := b.activeTab()
	tab.scroll, tab.scrollX = 30, 25

	b.Navigate("http://a") // set the active URL for the next Deliver
	b.Deliver("http://a", make([]byte, (cr.W)*(cr.H*3)*4), cr.W, cr.H*3, cr.W, nil, "A")
	if tab.scroll != 0 || tab.scrollX != 0 {
		t.Errorf("Deliver reset: scroll=%d scrollX=%d, want 0,0", tab.scroll, tab.scrollX)
	}
}

// TestBrowserZoomReclampsBothOffsets scrolls to the extents at high zoom then
// zooms back out, asserting both offsets are re-clamped into the smaller ranges.
func TestBrowserZoomReclampsBothOffsets(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*3, nil)
	b.SetZoom(3.0)
	tab := b.activeTab()
	tab.scroll = b.maxScroll(tab, cr)
	tab.scrollX = b.maxScrollX(tab, cr)

	b.SetZoom(0.5) // shrink: horizontal overflow disappears, vertical shrinks
	if m := b.maxScroll(tab, cr); tab.scroll > m {
		t.Errorf("scroll %d exceeds new max %d after zoom-out", tab.scroll, m)
	}
	if m := b.maxScrollX(tab, cr); tab.scrollX > m {
		t.Errorf("scrollX %d exceeds new max %d after zoom-out", tab.scrollX, m)
	}
}

// TestBrowserShiftWheelScrollsHorizontally asserts a plain wheel scrolls
// vertically while Shift+wheel scrolls horizontally, each clamped.
func TestBrowserShiftWheelScrollsHorizontally(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*3, nil)
	b.SetZoom(2.0)
	tab := b.activeTab()

	b.OnEvent(Event{Kind: EventScroll, Delta: 2})
	if tab.scroll == 0 || tab.scrollX != 0 {
		t.Errorf("plain wheel: scroll=%d scrollX=%d, want vertical only", tab.scroll, tab.scrollX)
	}
	b.OnEvent(Event{Kind: EventScroll, Delta: 2, Shift: true})
	if tab.scrollX == 0 {
		t.Error("shift+wheel did not scroll horizontally")
	}
}

// TestBrowserScrollbarInertWithoutOverflow covers the not-live paths: a page that
// fits needs neither bar, and OnEvent drag/press with no live bar is a no-op.
func TestBrowserScrollbarInertWithoutOverflow(t *testing.T) {
	b, nav, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H/2, nil) // shorter than the viewport: no overflow
	tab := b.activeTab()
	if _, ok := b.vscrollGeom(tab, cr); ok {
		t.Error("vertical bar should be inert for a short page")
	}
	if _, ok := b.hscrollGeom(tab, cr); ok {
		t.Error("horizontal bar should be inert at zoom 1")
	}
	// A drag with no thumb grabbed does nothing.
	b.OnEvent(Event{Kind: EventMouseDrag, X: cr.X + 5, Y: cr.Y + 5})
	if tab.scroll != 0 || tab.scrollX != 0 {
		t.Errorf("stray drag moved offsets to %d,%d", tab.scroll, tab.scrollX)
	}
	// A content click with no link and no bar adds no navigation.
	before := len(*nav)
	b.OnEvent(Event{Kind: EventClick, X: cr.X + 5, Y: cr.Y + 5})
	if len(*nav) != before {
		t.Errorf("stray content click navigated: %v", (*nav)[before:])
	}
}

// TestBrowserScrollbarDegenerateGeometry covers the thumb floor/clamp on a track
// shorter than the thumb minimum, and the not-live path when the reserved corner
// consumes the whole track.
func TestBrowserScrollbarDegenerateGeometry(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	cr0 := b.contentRect()
	deliverPage(b, cr0.W, cr0.H*50, nil) // a very tall page → large display height
	tab := b.activeTab()

	// A track shorter than scrollbarWidth: the thumb floors up to scrollbarWidth
	// then clamps back down to the track length (never longer than its track).
	tiny := Rect{X: 0, Y: 0, W: cr0.W, H: 5}
	g, ok := b.vscrollGeom(tab, tiny)
	if !ok {
		t.Fatal("expected a live vertical bar for a tall page in a tiny track")
	}
	if g.thumbLen != tiny.H {
		t.Errorf("degenerate thumbLen = %d, want it clamped to the track length %d", g.thumbLen, tiny.H)
	}

	// With horizontal overflow the bars reserve the corner; a track exactly
	// scrollbarWidth tall/wide is fully consumed → not live.
	b.SetZoom(2.0)
	if _, ok := b.vscrollGeom(tab, Rect{X: 0, Y: 0, W: cr0.W, H: scrollbarWidth}); ok {
		t.Error("vertical bar should be inert when the reserved corner consumes its track")
	}
	if _, ok := b.hscrollGeom(tab, Rect{X: 0, Y: 0, W: scrollbarWidth, H: cr0.H}); ok {
		t.Error("horizontal bar should be inert when the reserved corner consumes its track")
	}

	// A horizontal track shorter than scrollbarWidth (no vertical overflow, so no
	// corner reserve) exercises the horizontal thumb floor + clamp.
	b2, _, _, _ := newTestBrowser()
	cr2 := b2.contentRect()
	deliverPage(b2, cr2.W, 20, nil) // short page → no vertical overflow
	b2.SetZoom(2.0)
	gh, ok := b2.hscrollGeom(b2.activeTab(), Rect{X: 0, Y: 0, W: 5, H: cr2.H})
	if !ok {
		t.Fatal("expected a live horizontal bar in a tiny track")
	}
	if gh.thumbLen != 5 {
		t.Errorf("degenerate hbar thumbLen = %d, want clamped to track length 5", gh.thumbLen)
	}
}

// TestBrowserWheelClampsNegative covers the negative-offset clamp: a wheel-up (or
// shift-wheel-left) from zero pins both offsets at zero.
func TestBrowserWheelClampsNegative(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*3, nil)
	tab := b.activeTab()
	b.OnEvent(Event{Kind: EventScroll, Delta: -3}) // up past the top
	if tab.scroll != 0 {
		t.Errorf("wheel-up from 0: scroll = %d, want 0", tab.scroll)
	}
	b.OnEvent(Event{Kind: EventScroll, Delta: -3, Shift: true}) // left past the start
	if tab.scrollX != 0 {
		t.Errorf("shift-wheel-left from 0: scrollX = %d, want 0", tab.scrollX)
	}
}

// TestBrowserHorizontalBlitWindow draws with a nonzero scrollX and asserts the
// content origin shows a page pixel (the blit is windowed by scrollX, skipping
// the columns scrolled off the left).
func TestBrowserHorizontalBlitWindow(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	cr := b.contentRect()
	deliverPage(b, cr.W, cr.H*2, nil) // page pixels carry B=0x40
	b.SetZoom(2.0)
	tab := b.activeTab()
	tab.scrollX = 20
	buf := make([]byte, browserBounds.W*browserBounds.H*4)
	b.Draw(newP(buf, browserBounds.W), DefaultLight())
	if got := pixelAt(buf, browserBounds.W, cr.X+1, cr.Y+1); got.B != 0x40 {
		t.Errorf("content origin after hscroll = %+v, want a page pixel (B=0x40)", got)
	}
}

// TestBrowserEventsWithoutActiveTab covers the no-tab guards: a drag or a content
// click on a browser with no open tab is a safe no-op.
func TestBrowserEventsWithoutActiveTab(t *testing.T) {
	b := NewBrowser()
	b.SetBounds(browserBounds)
	cr := b.contentRect()
	b.OnEvent(Event{Kind: EventMouseDrag, X: cr.X + 5, Y: cr.Y + 5})
	b.OnEvent(Event{Kind: EventClick, X: cr.X + 5, Y: cr.Y + 5})
}
