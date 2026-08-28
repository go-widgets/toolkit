// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"strconv"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// PagedView is a reusable document / page viewer: it shows a sequence of
// pre-rendered page bitmaps with two layout modes, uniform zoom, and full
// keyboard + mouse navigation. It OWNS an inner [ScrollView] (the scrollable
// pane) and its own toolbar strip, so a host wires only the pages in and reads
// the navigation state out — it never hand-rolls paging, zoom or a toolbar.
//
// Content is a []*image.RGBA of page bitmaps at natural size (see [PagedView.SetPages]);
// PagedView never knows how they were rasterised (SVG, LaTeX, PDF — all the
// caller's concern). It scales every page's blit uniformly by [PagedView.Zoom].
//
// The two modes ([PagedView.Mode]):
//
//   - PagedContinuous stacks every page vertically as a card (a subtle drop
//     shadow + 1px border, a gap between cards), the whole stack scrollable.
//   - PagedPaginated shows ONE page centred in the pane; the toolbar grows a
//     prev / "n / N" / next group, and the current page still scrolls within the
//     pane when it is taller than the pane (a PDF single-page-continuous feel:
//     the wheel scrolls WITHIN the page first and only flips at the edge).
//
// All reactive state is MVVM-only, exposed through shared [mvvm.Observable]
// accessors (there are no settable state fields, mirroring DropDown / Switch):
// [PagedView.Mode], [PagedView.CurrentPage] (1-based), and [PagedView.Zoom]
// (percent). CurrentPage updates identically whether the change came from a
// toolbar button, a key, or the wheel.
type PagedView struct {
	Base
	focusState

	// pages is the set of page bitmaps (config-ish content set via SetPages, not
	// reactive Observable state); it is unexported so the MVVM gate stays green.
	// A nil entry is a page the view lays out but does not blit.
	pages []*image.RGBA

	// sizes is each page's NATURAL size, in the bitmap's own pixels. SetPages
	// derives it from the bitmaps; SetPageSizes declares it for pages that have
	// no bitmap at all, because a host renders them itself. It is what every
	// layout decision reads, so a hosted page pages, zooms, scrolls and reports
	// its rectangle exactly like a blitted one.
	sizes []image.Point

	// Reactive state, MVVM-only behind accessors.
	mode    *mvvm.Observable[PagedMode]
	current *mvvm.Observable[int]
	zoom    *mvvm.Observable[int]

	// fitToWidth, when set, re-derives the zoom from the pane width on every
	// relayout (a resize, a page change, a new document) so the current page
	// keeps filling the width — until a manual zoom-in / zoom-out turns it off.
	// The fit toolbar button turns it on. Off by default: a bare PagedView opens
	// at the fixed default zoom.
	fitToWidth bool

	// Owned sub-widgets: the scrollable pane over the page content, and the
	// toolbar icon buttons.
	scroll  *ScrollView
	content *pagedContent

	btnContinuous, btnPaginated   *Button
	btnZoomOut, btnZoomIn, btnFit *Button
	btnPrev, btnNext              *Button

	// lay is the current page layout (card rects + content size), recomputed each
	// relayout so Draw and the scroll-into-view helpers share one geometry.
	lay pagedLayout
	// percentRect and pageNRect are the toolbar text slots resolved each relayout.
	percentRect, pageNRect Rect
}

// PagedMode selects PagedView's layout: a continuous vertical stack of page
// cards, or a single centred page with prev/next paging.
type PagedMode int

const (
	// PagedContinuous stacks every page vertically as a scrollable card list.
	PagedContinuous PagedMode = iota
	// PagedPaginated shows one page at a time, centred, with prev/next paging.
	PagedPaginated
)

// Toolbar + page-card layout metrics, in LOGICAL pixels (routed through scaled
// so they grow under HiDPI / touch density).
const (
	pagedToolbarH   = 30 // toolbar strip height
	pagedToolbarPad = 4  // inset from the toolbar edge to the buttons
	pagedToolbarGap = 4  // gap between toolbar elements
	pagedTextPad    = 6  // horizontal padding around a toolbar text slot
	pagedIconInset  = 5  // inset from a button edge to its icon glyph
	pagedMargin     = 12 // margin around the page stack / single page
	pagedGap        = 12 // gap between stacked cards in continuous mode
	pagedShadow     = 3  // drop-shadow offset of a page card

	pagedZoomMin     = 25  // smallest zoom, percent
	pagedZoomMax     = 400 // largest zoom, percent
	pagedZoomStep    = 25  // zoom-in / zoom-out step, percent
	pagedZoomDefault = 100 // initial zoom, percent
)

// pagedShadowColor is the translucent black of a page card's drop shadow,
// src-over-blended so it darkens whatever sits behind the card.
var pagedShadowColor = RGBA{R: 0, G: 0, B: 0, A: 0x30}

// pagedLayout is the resolved geometry of the page content: one card rect per
// drawn page (in CONTENT-space, origin 0,0), the page index each card shows, and
// the total content size the ScrollView scrolls over.
type pagedLayout struct {
	cards    []Rect
	indices  []int
	contentW int
	contentH int
}

// NewPagedView builds a PagedView over the given pages (any of which may be
// nil, which is skipped when drawn). It starts in PagedContinuous mode at 100%
// zoom on page 1.
func NewPagedView(pages []*image.RGBA) *PagedView {
	pv := &PagedView{
		mode:    mvvm.NewObservable(PagedContinuous),
		current: mvvm.NewObservable(1),
		zoom:    mvvm.NewObservable(pagedZoomDefault),
	}
	pv.ensure()
	pv.SetPages(pages)
	return pv
}

// Mode is the layout mode as a shared [mvvm.Observable]: a host binds it (Set /
// Subscribe / two-way) — there is no settable Mode field. The two toolbar mode
// buttons Set it; subscribers are notified on change.
func (pv *PagedView) Mode() *mvvm.Observable[PagedMode] {
	if pv.mode == nil {
		pv.mode = mvvm.NewObservable(PagedContinuous)
	}
	return pv.mode
}

// CurrentPage is the 1-based current page as a shared [mvvm.Observable]: 1 for
// the first page, [PagedView.PageCount] for the last, and 1 when there are no
// pages. A toolbar button, a nav key, or a wheel edge-flip all Set it through
// the same clamp; subscribers are notified on change.
func (pv *PagedView) CurrentPage() *mvvm.Observable[int] {
	if pv.current == nil {
		pv.current = mvvm.NewObservable(1)
	}
	return pv.current
}

// Zoom is the zoom level in PERCENT as a shared [mvvm.Observable] (100 = natural
// size). The zoom-in / zoom-out / fit buttons Set it (clamped to
// [pagedZoomMin, pagedZoomMax]); subscribers are notified on change.
func (pv *PagedView) Zoom() *mvvm.Observable[int] {
	if pv.zoom == nil {
		pv.zoom = mvvm.NewObservable(pagedZoomDefault)
	}
	return pv.zoom
}

// PageCount is the number of pages currently set.
func (pv *PagedView) PageCount() int { return len(pv.sizes) }

// SetPages replaces the page bitmaps (each a natural-size RGBA the caller
// rasterised; nil entries are skipped when drawn). CurrentPage is clamped back
// into range: to the last page if it now points past the end, to 1 if the set
// is empty or the page was below 1.
func (pv *PagedView) SetPages(pages []*image.RGBA) {
	pv.pages = pages
	pv.sizes = make([]image.Point, len(pages))
	for i, img := range pages {
		if img != nil {
			pv.sizes[i] = image.Point{X: img.Rect.Dx(), Y: img.Rect.Dy()}
		}
	}
	pv.clampCurrent(len(pages))
}

// SetPageSizes declares pages the view LAYS OUT but does not draw the content
// of: it paints each one's paper, shadow and border, and a host puts the real
// thing on top — an <svg> in the DOM, a video, a native subview — reading where
// from [PagedView.PageRect].
//
// This is the same widget doing the same paging, zoom, scrolling and page
// reporting; only the blit is missing, because for some content the host draws
// it better than a pixel buffer can. Rasterising a typeset page into a bitmap
// costs 2,6x to 5,6x what handing the same SVG to a browser costs, measured in
// wasm against the browser it runs in — and the bitmap is fixed at one
// resolution while the host's own rendering is not.
//
// sizes are natural (un-zoomed) page sizes, in the same pixels a bitmap would
// have used, so a hosted set and a blitted set lay out identically.
func (pv *PagedView) SetPageSizes(sizes []image.Point) {
	pv.pages = nil
	pv.sizes = sizes
	pv.clampCurrent(len(sizes))
}

// clampCurrent brings CurrentPage back into range after the page set changed:
// to the last page if it now points past the end, to 1 if the set is empty or
// the page was below 1.
func (pv *PagedView) clampCurrent(n int) {
	c := pv.CurrentPage().Get()
	switch {
	case n == 0, c < 1:
		pv.CurrentPage().Set(1)
	case c > n:
		pv.CurrentPage().Set(n)
	}
}

// cur is the 1-based current page as a plain int.
func (pv *PagedView) cur() int { return pv.CurrentPage().Get() }

// ensure lazily builds the owned sub-widgets (scroll pane + toolbar buttons) on
// first use, so a bare &PagedView{} works exactly like one from NewPagedView.
func (pv *PagedView) ensure() {
	if pv.scroll != nil {
		return
	}
	pv.content = &pagedContent{pv: pv}
	pv.scroll = NewScrollView(pv.content)
	pv.btnContinuous = pv.iconButton("multiple-pages", func() { pv.setMode(PagedContinuous) })
	pv.btnPaginated = pv.iconButton("page", func() { pv.setMode(PagedPaginated) })
	pv.btnZoomOut = pv.iconButton("zoom-out", pv.zoomOut)
	pv.btnZoomIn = pv.iconButton("zoom-in", pv.zoomIn)
	pv.btnFit = pv.iconButton("expand", pv.fitWidth)
	pv.btnPrev = pv.iconButton("nav-arrow-left", func() { pv.setPage(pv.cur()-1, false) })
	pv.btnNext = pv.iconButton("nav-arrow-right", func() { pv.setPage(pv.cur()+1, false) })
}

// iconButton builds one toolbar button whose face is an iconoir glyph. Press
// feedback is off (there is no routed mouse-up on these) so a click just fires
// onClick.
func (pv *PagedView) iconButton(name string, onClick func()) *Button {
	b := NewButton("", onClick)
	b.PressFeedback = false
	b.Icon = func(p painter.Painter, r Rect, ink RGBA) {
		drawIconoir(p, iconGlyphRect(r), name, ink)
	}
	return b
}

// iconGlyphRect is the centred square an icon glyph is drawn into inside a
// (square) toolbar button, inset on every edge.
func iconGlyphRect(r Rect) Rect {
	in := scaled(pagedIconInset)
	s := r.W - 2*in
	if s < 1 {
		s = 1
	}
	return Rect{X: r.X + (r.W-s)/2, Y: r.Y + (r.H-s)/2, W: s, H: s}
}

// SetBounds records the placement and relays out the pane + toolbar so events
// arriving before the first Draw already hit-test against the right geometry.
func (pv *PagedView) SetBounds(r Rect) {
	pv.Base.SetBounds(r)
	pv.relayout()
}

// relayout resolves the toolbar + pane geometry and the page content layout from
// the current bounds, mode, zoom and pages. Idempotent, so it runs at the top of
// Draw and OnEvent as well as from SetBounds.
func (pv *PagedView) relayout() {
	pv.ensure()
	r := pv.Bounds()
	tbH := scaled(pagedToolbarH)
	pane := Rect{X: r.X, Y: r.Y + tbH, W: r.W, H: r.H - tbH}
	if pane.H < 0 {
		pane.H = 0
	}
	pv.scroll.SetBounds(pane)
	// Sticky fit-to-width: re-derive the zoom from the just-set pane width before
	// laying the page out, so a resize or a new document keeps the page filling
	// the width. Set the observable directly (not setZoom) — this IS the relayout.
	if pv.fitToWidth {
		if z, ok := pv.fitWidthZoom(); ok {
			pv.zoom.Set(clampZoom(z))
		}
	}
	usableW := pane.W - scrollGutter()
	pv.lay = pv.computeLayout(usableW, pane.H)
	pv.content.SetBounds(Rect{X: pane.X, Y: pane.Y, W: pv.lay.contentW, H: pv.lay.contentH})
	pv.scroll.SetContentSize(pv.lay.contentW, pv.lay.contentH)
	pv.layoutToolbar(r, tbH)
	pv.syncButtonStates()
}

// scaledSize is page i's blit size at the current zoom, in device pixels. A nil
// page (or one scaled away to nothing) reports zero and is skipped when drawn.
func (pv *PagedView) scaledSize(i int) (w, h int) {
	sz := pv.sizes[i]
	if sz.X <= 0 || sz.Y <= 0 {
		return 0, 0
	}
	z := pv.Zoom().Get()
	return sz.X * z / 100, sz.Y * z / 100
}

// computeLayout resolves the card rects + content size for the current mode. The
// content is at least the usable pane size on each axis, so a page narrower or
// shorter than the pane is centred within it rather than pinned to a corner.
func (pv *PagedView) computeLayout(usableW, usableH int) pagedLayout {
	if usableW < 0 {
		usableW = 0
	}
	if usableH < 0 {
		usableH = 0
	}
	margin := scaled(pagedMargin)
	gap := scaled(pagedGap)
	var lay pagedLayout
	if pv.Mode().Get() == PagedPaginated {
		idx := pv.cur() - 1
		if idx < 0 || idx >= len(pv.sizes) {
			lay.contentW, lay.contentH = usableW, usableH
			return lay
		}
		pw, ph := pv.scaledSize(idx)
		cw := pw + 2*margin
		ch := ph + 2*margin
		if cw < usableW {
			cw = usableW
		}
		if ch < usableH {
			ch = usableH
		}
		lay.cards = []Rect{{X: (cw - pw) / 2, Y: (ch - ph) / 2, W: pw, H: ph}}
		lay.indices = []int{idx}
		lay.contentW, lay.contentH = cw, ch
		return lay
	}
	// Continuous: a centred vertical stack of every page.
	if len(pv.sizes) == 0 {
		lay.contentW, lay.contentH = usableW, usableH
		return lay
	}
	maxW := 0
	for i := range pv.sizes {
		if pw, _ := pv.scaledSize(i); pw > maxW {
			maxW = pw
		}
	}
	cw := maxW + 2*margin
	if cw < usableW {
		cw = usableW
	}
	y := margin
	for i := range pv.sizes {
		pw, ph := pv.scaledSize(i)
		lay.cards = append(lay.cards, Rect{X: (cw - pw) / 2, Y: y, W: pw, H: ph})
		lay.indices = append(lay.indices, i)
		y += ph + gap
	}
	ch := y - gap + margin // trailing gap becomes the bottom margin
	if ch < usableH {
		ch = usableH
	}
	lay.contentW, lay.contentH = cw, ch
	return lay
}

// layoutToolbar positions every toolbar button (left to right) and resolves the
// percent + "n / N" text slots. The prev/next group is placed only in paginated
// mode.
func (pv *PagedView) layoutToolbar(r Rect, tbH int) {
	pad := scaled(pagedToolbarPad)
	gap := scaled(pagedToolbarGap)
	sz := tbH - 2*pad
	if sz < 1 {
		sz = 1
	}
	x := r.X + pad
	y := r.Y + pad
	place := func(b *Button) {
		b.SetBounds(Rect{X: x, Y: y, W: sz, H: sz})
		x += sz + gap
	}
	place(pv.btnContinuous)
	place(pv.btnPaginated)
	x += gap // separate the mode toggle from the zoom group
	place(pv.btnZoomOut)
	pw := pv.textWidth(pv.percentText()) + 2*scaled(pagedTextPad)
	pv.percentRect = Rect{X: x, Y: y, W: pw, H: sz}
	x += pw + gap
	place(pv.btnZoomIn)
	place(pv.btnFit)
	if pv.Mode().Get() == PagedPaginated {
		x += gap
		place(pv.btnPrev)
		nw := pv.textWidth(pv.pageIndicator()) + 2*scaled(pagedTextPad)
		pv.pageNRect = Rect{X: x, Y: y, W: nw, H: sz}
		x += nw + gap
		place(pv.btnNext)
	}
}

// syncButtonStates reflects the reactive state onto the toolbar buttons: the
// active mode button reads Selected, and the zoom / paging buttons disable at
// their range ends.
func (pv *PagedView) syncButtonStates() {
	mode := pv.Mode().Get()
	pv.btnContinuous.Selected().Set(mode == PagedContinuous)
	pv.btnPaginated.Selected().Set(mode == PagedPaginated)
	z := pv.Zoom().Get()
	pv.btnZoomOut.Disabled().Set(z <= pagedZoomMin)
	pv.btnZoomIn.Disabled().Set(z >= pagedZoomMax)
	cur, cnt := pv.cur(), pv.PageCount()
	pv.btnPrev.Disabled().Set(cur <= 1)
	pv.btnNext.Disabled().Set(cur >= cnt)
}

// toolbarButtons is the drawn / hit-tested button set for the current mode: the
// prev/next pair only exists in paginated mode.
func (pv *PagedView) toolbarButtons() []*Button {
	bs := []*Button{pv.btnContinuous, pv.btnPaginated, pv.btnZoomOut, pv.btnZoomIn, pv.btnFit}
	if pv.Mode().Get() == PagedPaginated {
		bs = append(bs, pv.btnPrev, pv.btnNext)
	}
	return bs
}

// percentText is the zoom read-out, "NNN%".
func (pv *PagedView) percentText() string { return strconv.Itoa(pv.Zoom().Get()) + "%" }

// pageIndicator is the paginated "n / N" read-out.
func (pv *PagedView) pageIndicator() string {
	return strconv.Itoa(pv.cur()) + " / " + strconv.Itoa(pv.PageCount())
}

// Draw paints the toolbar strip (buttons + read-outs) then the scrollable pane
// of page cards, and finally the focus ring.
func (pv *PagedView) Draw(p painter.Painter, theme *Theme) {
	pv.relayout()
	r := pv.Bounds()
	tbH := scaled(pagedToolbarH)
	fillRect(p, r.X, r.Y, r.W, tbH, theme.SurfaceAlt)
	for _, b := range pv.toolbarButtons() {
		b.Draw(p, theme)
	}
	pv.drawCenteredText(p, theme, pv.percentRect, pv.percentText())
	if pv.Mode().Get() == PagedPaginated {
		pv.drawCenteredText(p, theme, pv.pageNRect, pv.pageIndicator())
	}
	fillRect(p, r.X, r.Y+tbH-strokeWidth(), r.W, strokeWidth(), theme.Border)
	pv.scroll.Draw(p, theme)
	pv.drawFocusRing(p, theme, r)
}

// drawCenteredText paints s centred in slot, in the theme's on-surface ink.
func (pv *PagedView) drawCenteredText(p painter.Painter, theme *Theme, slot Rect, s string) {
	tx := slot.X + (slot.W-pv.textWidth(s))/2
	ty := slot.Y + (slot.H-pv.glyphHeight())/2
	pv.drawText(p, tx, ty, s, theme.OnSurface)
}

// drawPages paints every card in the current layout, relative to base (the
// content origin the ScrollView placed the child at). Each card is a drop
// shadow, a surface matte, the page blit, and a 1px border.
func (pv *PagedView) drawPages(p painter.Painter, theme *Theme, base Rect) {
	for k, card := range pv.lay.cards {
		dst := Rect{X: base.X + card.X, Y: base.Y + card.Y, W: card.W, H: card.H}
		if dst.W <= 0 || dst.H <= 0 {
			continue
		}
		sh := scaled(pagedShadow)
		fillRect(p, dst.X+sh, dst.Y+sh, dst.W, dst.H, pagedShadowColor)
		fillRect(p, dst.X, dst.Y, dst.W, dst.H, theme.Surface)
		// A hosted page has no bitmap: its paper, shadow and border are painted
		// here and the host draws the content over them (see SetPageSizes).
		if img := pv.pageImage(pv.lay.indices[k]); img != nil {
			blitImage(p, dst, dst, img.Pix, img.Rect.Dx(), img.Rect.Dy())
		}
		strokeRect(p, dst.X, dst.Y, dst.W, dst.H, theme.Border)
	}
}

// pageImage is page i's bitmap, or nil when the host renders that page.
func (pv *PagedView) pageImage(i int) *image.RGBA {
	if i < 0 || i >= len(pv.pages) {
		return nil
	}
	return pv.pages[i]
}

// HitTest covers the full bounds (the toolbar + pane are both interactive).
func (pv *PagedView) HitTest(px, py int) bool { return pv.Bounds().Contains(px, py) }

// A11y describes the widget as a document, with the current page position as its
// value ("n / N") and — when there is at least one page — a machine-readable
// aria-valuemin/max/now triple over [1, PageCount].
func (pv *PagedView) A11y() A11yInfo {
	info := A11yInfo{Role: RoleDocument, Value: pv.pageIndicator()}
	if n := pv.PageCount(); n > 0 {
		info.HasRange = true
		info.Min, info.Max, info.Now = 1, float64(n), float64(pv.cur())
	}
	return info
}

// ScrollOffset reports the inner [ScrollView]'s current content offset, in
// DEVICE pixels: how far the page content has scrolled right (x) and down (y)
// inside the pane. A host persists it to restore the scroll position across a
// reload, or pairs it with [PagedView.PageAt] / [PagedView.ScrollToPage] to
// reason about what is currently visible. It is a thin passthrough of the owned
// ScrollView's OffsetX / OffsetY.
func (pv *PagedView) ScrollOffset() (x, y int) {
	pv.ensure()
	return pv.scroll.OffsetX().Get(), pv.scroll.OffsetY().Get()
}

// naturalToContentY and contentYToNatural are the ONE page-y↔content-y mapping,
// as an inverse pair over a single card. contentY is the ScrollView content-space
// Y (origin at the top of the page stack, device pixels); naturalY is a Y in the
// page bitmap's own un-zoomed pixels. [PagedView.PageAt] inverts this with
// contentYToNatural and [PagedView.ScrollToPage] applies it with naturalToContentY,
// so the two seams read the same card offset (lay.cards[k].Y) and the same zoom
// and can never drift — the discipline PagedView already keeps between Draw and
// its hit-tests. cardIdx indexes pv.lay.cards (NOT a page number).
func (pv *PagedView) naturalToContentY(cardIdx, naturalY int) int {
	return pv.lay.cards[cardIdx].Y + naturalY*pv.Zoom().Get()/100
}

func (pv *PagedView) contentYToNatural(cardIdx, contentY int) int {
	return (contentY - pv.lay.cards[cardIdx].Y) * 100 / pv.Zoom().Get()
}

// PageAt maps a WIDGET-LOCAL pointer position — (x, y) as delivered to
// [PagedView.OnEvent], i.e. relative to the widget's own bounds — to the page
// under it and the point WITHIN that page's NATURAL (un-zoomed) image pixels. It
// accounts for the toolbar strip height, the layout mode (a continuous stack vs a
// single centred page), the zoom factor (divided back out so localX / localY are
// natural pixels), the scroll offset, and each card's own position and the gaps
// between them. The returned page is 1-based, matching [PagedView.CurrentPage].
//
// ok is false when the point falls in the toolbar strip, in a gap between cards,
// over the scrollbar gutter, or outside every page (including every case where
// there are no pages) — the localX / localY are then zero. This is the seam that
// turns a click on a rendered page into a (page, natural-point) a host can look
// up against its own source map. It is the exact inverse of [PagedView.ScrollToPage]'s
// vertical mapping (they share naturalToContentY / contentYToNatural).
func (pv *PagedView) PageAt(x, y int) (page, localX, localY int, ok bool) {
	pv.relayout()
	tbH := scaled(pagedToolbarH)
	if y < tbH {
		return 0, 0, 0, false // in the toolbar strip, not over any page
	}
	// Widget-local → content-space: the pane starts at widget-local (0, tbH), and
	// the ScrollView shows the content shifted up/left by its offset.
	contentX := x + pv.scroll.OffsetX().Get()
	contentY := (y - tbH) + pv.scroll.OffsetY().Get()
	for k, card := range pv.lay.cards {
		if contentX < card.X || contentX >= card.X+card.W ||
			contentY < card.Y || contentY >= card.Y+card.H {
			continue // outside this card (or it is a zero-area / nil page)
		}
		localX = (contentX - card.X) * 100 / pv.Zoom().Get()
		localY = pv.contentYToNatural(k, contentY)
		return pv.lay.indices[k] + 1, localX, localY, true
	}
	return 0, 0, 0, false // in a gap, the gutter, or past the last card
}

// ScrollToPage scrolls the pane so that natural-coordinate localY of the given
// 1-based page is brought to the TOP of the viewport (clamped to the legal scroll
// range at both ends). It is the vertical inverse of [PagedView.PageAt]: a
// round-trip — PageAt to read a point, then ScrollToPage with the page + localY it
// returned — lands that point at the top of the pane to within a pixel.
//
// In PagedPaginated mode it also makes page the current page (so the single
// displayed card is that page). A page outside [1, PageCount] or an empty view is
// a no-op. This is the seam that turns an editor caret move into a scroll: a host
// maps the caret's source line to (page, natural-Y) via its own source map and
// calls this to bring that line into view.
func (pv *PagedView) ScrollToPage(page, localY int) {
	cnt := pv.PageCount()
	if cnt == 0 || page < 1 || page > cnt {
		return // empty view or page out of range
	}
	var k int
	if pv.Mode().Get() == PagedPaginated {
		if page != pv.cur() {
			pv.setPage(page, false) // switch the displayed page (relays out)
		} else {
			pv.relayout()
		}
		k = 0 // paginated lays out exactly the current page as the sole card
	} else {
		pv.relayout()
		k = page - 1 // continuous cards are 1:1 with pages, in order
	}
	target := pv.naturalToContentY(k, localY)
	pv.scroll.Scroll(0, target-pv.scroll.OffsetY().Get()) // Scroll clamps to [0, max]
}

// OnEvent drives the widget: a toolbar click routes to the button under it; a
// wheel scroll pages-or-scrolls (see onScroll); a nav key steps the page; every
// other pointer event (drag / release for the pane's pan + scrollbars) forwards
// to the inner ScrollView. A Disabled PagedView ignores everything.
func (pv *PagedView) OnEvent(ev Event) {
	if pv.Disabled().Get() {
		return
	}
	pv.relayout()
	switch ev.Kind {
	case EventClick:
		if pv.clickToolbar(ev) {
			return
		}
		pv.forwardToScroll(ev)
	case EventScroll:
		pv.onScroll(ev)
	case EventKeyDown:
		pv.onKey(ev.Code)
	default:
		pv.forwardToScroll(ev)
	}
}

// clickToolbar routes a click (widget-local coords) to the toolbar button that
// contains it, returning whether one did. Buttons respect their own Disabled
// state, so a click on a greyed prev/next/zoom button is a no-op.
func (pv *PagedView) clickToolbar(ev Event) bool {
	r := pv.Bounds()
	sx, sy := ev.X+r.X, ev.Y+r.Y
	for _, b := range pv.toolbarButtons() {
		if b.Bounds().Contains(sx, sy) {
			b.OnEvent(Event{Kind: EventClick})
			return true
		}
	}
	return false
}

// forwardToScroll hands an event to the inner ScrollView, translating the
// widget-local Y down past the toolbar strip into the pane's own frame.
func (pv *PagedView) forwardToScroll(ev Event) {
	ev.Y -= scaled(pagedToolbarH)
	pv.scroll.OnEvent(ev)
}

// onScroll implements the paginated single-page-continuous feel: a wheel notch
// past the BOTTOM edge of the current page flips to the next page (landing at
// its top), past the TOP edge flips to the previous (landing at its bottom); a
// page taller than the pane scrolls WITHIN first and only flips at the edge. In
// continuous mode (and for horizontal swipes) the wheel just scrolls the pane.
func (pv *PagedView) onScroll(ev Event) {
	if pv.Mode().Get() == PagedPaginated {
		atTop := pv.scroll.OffsetY().Get() <= 0
		atBottom := pv.scroll.OffsetY().Get() >= pv.scroll.maxOffsetY()
		if ev.Delta > 0 && atBottom {
			pv.setPage(pv.cur()+1, false)
			return
		}
		if ev.Delta < 0 && atTop {
			pv.setPage(pv.cur()-1, true)
			return
		}
	}
	pv.forwardToScroll(ev)
}

// onKey steps the page from the keyboard: PageDown / ArrowDown / Space → next,
// PageUp / ArrowUp → previous, Home → first, End → last. All clamp with no wrap.
func (pv *PagedView) onKey(code string) {
	switch code {
	case "PageDown", "ArrowDown", " ", "Space":
		pv.setPage(pv.cur()+1, false)
	case "PageUp", "ArrowUp":
		pv.setPage(pv.cur()-1, false)
	case "Home":
		pv.setPage(1, false)
	case "End":
		pv.setPage(pv.PageCount(), false)
	}
}

// setMode switches layout mode (no-op if unchanged), then rescrolls so the
// current page is shown: continuous scrolls its card into view, paginated shows
// the page from the top.
func (pv *PagedView) setMode(m PagedMode) {
	if m == pv.Mode().Get() {
		return
	}
	pv.Mode().Set(m)
	pv.relayout()
	pv.scroll.Scroll(0, -scrollExtreme)
	if pv.Mode().Get() == PagedContinuous {
		pv.scrollCardIntoView(pv.cur())
	}
}

// setPage moves to page n (1-based, clamped to [1, PageCount], no wrap). A real
// change relays out and repositions the pane: paginated lands at the new page's
// top (or bottom, when toBottom — an upward wheel-flip so continued up-scroll is
// seamless), continuous scrolls the card into view. An empty view or an
// unchanged page is a no-op.
func (pv *PagedView) setPage(n int, toBottom bool) {
	cnt := pv.PageCount()
	if cnt == 0 {
		return
	}
	if n < 1 {
		n = 1
	}
	if n > cnt {
		n = cnt
	}
	if n == pv.cur() {
		return
	}
	pv.CurrentPage().Set(n)
	pv.relayout()
	switch {
	case pv.Mode().Get() != PagedPaginated:
		pv.scrollCardIntoView(n)
	case toBottom:
		pv.scroll.Scroll(0, scrollExtreme)
	default:
		pv.scroll.Scroll(0, -scrollExtreme)
	}
}

// scrollCardIntoView (continuous mode) scrolls so page n's card sits at the top
// of the pane, less the top margin. Scroll clamps at both ends.
func (pv *PagedView) scrollCardIntoView(n int) {
	card := pv.lay.cards[n-1]
	pv.scroll.Scroll(0, card.Y-scaled(pagedMargin)-pv.scroll.OffsetY().Get())
}

// zoomIn / zoomOut step the zoom by one increment; setZoom clamps + relays out.
// zoomIn / zoomOut are the manual zoom controls: they step the zoom and turn
// sticky fit-to-width OFF, because the reader has taken the zoom into their own
// hands and a later resize should no longer override it.
func (pv *PagedView) zoomIn()  { pv.fitToWidth = false; pv.setZoom(pv.Zoom().Get() + pagedZoomStep) }
func (pv *PagedView) zoomOut() { pv.fitToWidth = false; pv.setZoom(pv.Zoom().Get() - pagedZoomStep) }

// clampZoom confines a zoom percent to [pagedZoomMin, pagedZoomMax].
func clampZoom(z int) int {
	if z < pagedZoomMin {
		return pagedZoomMin
	}
	if z > pagedZoomMax {
		return pagedZoomMax
	}
	return z
}

// setZoom Sets the zoom percent clamped to [pagedZoomMin, pagedZoomMax] and
// relays out so the new blit size takes effect.
func (pv *PagedView) setZoom(z int) {
	pv.Zoom().Set(clampZoom(z))
	pv.relayout()
}

// SetFitWidth turns sticky fit-to-width on or off. When on, the current page's
// natural width is scaled to fill the pane and RE-fits on every relayout (a
// resize, a page change, a new document), so the preview keeps using the whole
// width instead of leaving the fixed-zoom page adrift in empty margins — until
// the reader zooms manually, which turns it back off. The fit toolbar button
// turns it on. Safe before any page or bounds exist: the fit is applied once
// there is a current page with a positive width.
func (pv *PagedView) SetFitWidth(on bool) {
	pv.ensure()
	pv.fitToWidth = on
	pv.relayout()
}

// FitWidth reports whether sticky fit-to-width is currently on.
func (pv *PagedView) FitWidth() bool { return pv.fitToWidth }

// fitWidthZoom is the zoom percent at which the current page's natural width
// fills the pane (less the page margins), and whether it could be computed (a
// current page with a positive width and a laid-out pane).
func (pv *PagedView) fitWidthZoom() (int, bool) {
	idx := pv.cur() - 1
	if idx < 0 || idx >= len(pv.sizes) {
		return 0, false
	}
	w := pv.sizes[idx].X
	if w <= 0 {
		return 0, false
	}
	usableW := pv.scroll.Bounds().W - scrollGutter() - 2*scaled(pagedMargin)
	if usableW < 1 {
		usableW = 1
	}
	return usableW * 100 / w, true
}

// fitWidth is the fit toolbar button's action: turn sticky fit-to-width on.
func (pv *PagedView) fitWidth() { pv.SetFitWidth(true) }

// pagedContent is the ScrollView's child: a thin adapter that draws the page
// cards its owning PagedView laid out. It has no state of its own — the layout
// lives on the PagedView — so it only overrides Draw.
type pagedContent struct {
	Base
	pv *PagedView
}

// Draw paints the page cards at this child's placed origin (the ScrollView has
// already applied the scroll translation + viewport clip around this call).
func (c *pagedContent) Draw(p painter.Painter, theme *Theme) {
	c.pv.drawPages(p, theme, c.Bounds())
}

// PageRect reports where a 1-based page is currently DRAWN and how much of it
// the pane still shows, both in SURFACE coordinates — the forward mapping
// [PagedView.PageAt] inverts.
//
// ⚠ The coordinate spaces differ on purpose. PageAt consumes a WIDGET-LOCAL
// point because that is what [PagedView.OnEvent] delivers. PageRect produces
// SURFACE coordinates because its caller is placing something over the pane —
// a [Foreign] region's host object, a magnifier, an annotation — and every
// other placement seam in this toolkit ([WalkA11y], [WalkForeign]) is
// surface-absolute. Adding the widget's origin twice is the mistake this note
// exists to prevent.
//
// rect is the whole card at the current zoom and scroll, so it can start above
// or left of the pane, or extend past it. clip is the part the pane actually
// shows: rect intersected with the ScrollView's viewport, which excludes the
// toolbar strip and the scrollbar gutters. clip is empty when the page is
// scrolled out of view or the mode shows a different page — ok stays true,
// because the page exists and simply is not on screen, which a host answers by
// hiding its object rather than destroying it.
//
// ok is false only when there is no such page: page outside [1, PageCount], or
// a page the current layout does not place (paginated mode shows one).
//
// The zoom the card is drawn at is rect.W / the page bitmap's natural width, or
// equivalently [PagedView.Zoom] as a percentage — a host scaling its own
// content to match the blit reads it from either.
func (pv *PagedView) PageRect(page int) (rect, clip Rect, ok bool) {
	pv.relayout()
	if page < 1 || page > pv.PageCount() {
		return Rect{}, Rect{}, false
	}
	for k, card := range pv.lay.cards {
		if pv.lay.indices[k] != page-1 {
			continue
		}
		// The content widget holds the card origin unscrolled; the ScrollView
		// reports how far it PAINTS that content from there. Reading both is what
		// keeps this in step with drawPages, which is handed the same base.
		base := pv.content.Bounds()
		dx, dy := pv.scroll.ChildOffset()
		rect = Rect{X: base.X + card.X + dx, Y: base.Y + card.Y + dy, W: card.W, H: card.H}
		return rect, intersectRect(rect, pv.scroll.ChildClip()), true
	}
	return Rect{}, Rect{}, false
}
