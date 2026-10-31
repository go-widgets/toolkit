// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"sort"
	"strconv"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// PageStrip is a strip of page thumbnails — the slide panel of PowerPoint,
// Keynote and Google Slides, the thumbnail sidebar of pdf.js and every PDF viewer
// — vertical (the default) or horizontal. It is generic: a page is whatever the
// host says, shown from a bitmap handed in with [PageStrip.SetImage] or painted by
// the [PageStrip.DrawPage] callback.
//
// The strip is virtualised the way pdf.js' thumbnail sidebar is: only the pages
// in (and one either side of) the window are laid out, requested and painted, so
// a 500-page document costs what a 10-page one does. A page that scrolls into
// view without an image is asked for ONCE through [PageStrip.NeedImage]; the host
// renders it (synchronously or later, on its own schedule) and answers with
// SetImage; pdf.js likewise keeps a per-view rendering state (initial / running /
// finished) and renders what is visible first. A bounded number of images stays
// resident ([PageStrip.MaxResident]); the ones farthest from the window are
// dropped and asked for again if they come back. [PageStrip.Invalidate] marks one
// page stale — the old bitmap stays on screen until its replacement arrives — so
// when a document is edited only the changed pages are re-rendered.
//
// The current page is the [PageStrip.Current] Observable (0-based, -1 for none):
// a click or a key sets it, a host sets it to follow its own view, and the strip
// scrolls to keep it visible either way. Pages are laid out at a common aspect
// (PageW:PageH, 16:9 by default) unless [PageStrip.SetAspects] says otherwise.
// A scrollbar in the toolkit's shared style shows while the pages overflow.
type PageStrip struct {
	Base
	focusState

	// Horizontal lays the pages out left to right instead of top to bottom. Set
	// once, before the first Draw.
	Horizontal bool

	// ThumbSize is the logical width of a thumbnail in a vertical strip (its
	// height in a horizontal one); 0 means [PageStripThumbSize]. The card shrinks
	// to fit a narrower strip.
	ThumbSize int

	// Fluid makes the cards follow the strip: a card is as wide (as tall in a
	// horizontal strip) as the strip's cross size less the padding, the number and
	// the scrollbar, between MinThumbSize and MaxThumbSize, keeping each page's
	// aspect ratio. Resizing the strip then resizes the cards: the first visible
	// page stays put, only the pages in the window are laid out, and the host is
	// asked for new bitmaps (only for those pages) once the card has changed by a
	// quarter since it last asked, not on every pixel of a drag; the bitmap it has
	// is drawn scaled meanwhile. Zero value false: cards are ThumbSize, shrunk to fit.
	Fluid bool

	// MinThumbSize and MaxThumbSize bound a Fluid card, in logical pixels; 0 means
	// 48 and 480.
	MinThumbSize, MaxThumbSize int

	// PageW and PageH give the default page aspect (0 means 16:9).
	PageW, PageH int

	// MaxResident bounds the bitmaps kept in memory; 0 means
	// [PageStripMaxResident].
	MaxResident int

	// NeedImage is called once when page i first comes within a page of the window
	// with no bitmap (and again after Invalidate). The host answers, now or later,
	// with SetImage(i, …). Nil: the strip never asks.
	NeedImage func(i int)

	// DrawPage, when set, paints page i into r — the card's inner rectangle, the
	// painter clipped to it — for pages that have no bitmap. Nil: a placeholder
	// with the page number.
	DrawPage func(p painter.Painter, theme *Theme, r Rect, i int)

	count   int
	aspects []pageAspect // optional per-page aspects, len == count when set
	imgs    []*stripImage
	asked   []bool
	nres    int // resident bitmaps

	cur    *mvvm.Observable[int]
	reveal bool                  // scroll Current into view at the next Draw
	scroll *mvvm.Observable[int] // scroll offset along the strip, device pixels
	drag   scrollDrag

	// offs[i] is the start of page i along the strip's axis, offs[count] the end,
	// for the cross size crossUsed; rebuilt when either changes.
	offs      []int
	crossUsed int
	// askSize[i] is the card size the host was last asked for page i's bitmap at
	// (Fluid): a page in the window is asked again once the card is a quarter
	// larger or smaller, so a drag through every size does not re-render at each.
	askSize []int
}

type pageAspect struct{ w, h int }

type stripImage struct {
	pix  []byte
	w, h int
}

const (
	// PageStripThumbSize is the default logical thumbnail width — pdf.js'
	// THUMBNAIL_WIDTH is 126, Keynote's navigator about 130.
	PageStripThumbSize = 120
	// PageStripMaxResident is the default bound on resident bitmaps.
	PageStripMaxResident = 96

	stripGap      = 10 // logical px between cards
	stripPad      = 8  // logical px around the strip's content
	stripPrefetch = 1  // pages requested beyond each end of the window
)

// NewPageStrip builds a strip of n pages with no current page.
func NewPageStrip(n int) *PageStrip {
	s := &PageStrip{}
	s.SetCount(n)
	return s
}

// Current is the selected page, 0-based, -1 for none, as a bindable Observable.
func (s *PageStrip) Current() *mvvm.Observable[int] {
	if s.cur == nil {
		s.cur = mvvm.NewObservable(-1)
		s.cur.Subscribe(func(int) { s.reveal = true })
	}
	return s.cur
}

// Scroll is the scroll offset along the strip, in device pixels, as a bindable
// Observable: a host that overlays something on the cards (an HTML thumbnail
// layer the canvas cannot paint) subscribes to keep it aligned, and may set it.
// Reads clamp on the fly.
func (s *PageStrip) Scroll() *mvvm.Observable[int] {
	if s.scroll == nil {
		s.scroll = mvvm.NewObservable(0)
	}
	return s.scroll
}

// CardRect is page i's card — the picture, not its number — in the SAME
// coordinates as Bounds (the surface's), at the current scroll offset; the zero
// Rect for an out-of-range page. A host overlaying its own content on the cards
// reads it (with Scroll) to place them.
func (s *PageStrip) CardRect(i int) Rect {
	if i < 0 || i >= s.count {
		return Rect{}
	}
	cr, r := s.cardRect(i), s.Bounds()
	return Rect{X: r.X + cr.X, Y: r.Y + cr.Y, W: cr.W, H: cr.H}
}

// Count is the number of pages.
func (s *PageStrip) Count() int { return s.count }

// SetCount sets the number of pages, keeping the bitmaps of the pages that stay
// and clamping Current. Per-page aspects set by SetAspects are dropped when the
// count no longer matches.
func (s *PageStrip) SetCount(n int) {
	if n < 0 {
		n = 0
	}
	if n < s.count {
		for i := n; i < s.count; i++ {
			if s.imgs[i] != nil {
				s.nres--
			}
		}
	}
	for len(s.imgs) < n {
		s.imgs = append(s.imgs, nil)
		s.asked = append(s.asked, false)
	}
	s.imgs, s.asked = s.imgs[:n], s.asked[:n]
	if len(s.aspects) != n {
		s.aspects = nil
	}
	s.count = n
	s.offs = nil
	if c := s.Current(); c.Get() >= n {
		c.Set(n - 1)
	}
}

// SetAspects gives every page its own width and height (their ratio is what
// matters); its length must equal Count or it is ignored.
func (s *PageStrip) SetAspects(sizes [][2]int) {
	if len(sizes) != s.count {
		return
	}
	s.aspects = make([]pageAspect, len(sizes))
	for i, z := range sizes {
		s.aspects[i] = pageAspect{z[0], z[1]}
	}
	s.offs = nil
}

// SetImage supplies page i's bitmap: w*h RGBA pixels (len >= w*h*4), nearest-
// neighbour scaled to the card — render it at the card's pixel size for an exact
// picture. The bytes are used as is, not copied. Out-of-range i or a short
// buffer is ignored. It evicts the farthest resident bitmaps when over budget.
func (s *PageStrip) SetImage(i int, pix []byte, w, h int) {
	if i < 0 || i >= s.count || w <= 0 || h <= 0 || len(pix) < w*h*4 {
		return
	}
	if s.imgs[i] == nil {
		s.nres++
	}
	s.imgs[i] = &stripImage{pix: pix, w: w, h: h}
	s.asked[i] = true
	s.evict(i)
}

// Invalidate marks page i stale: its bitmap (if any) stays shown until SetImage
// replaces it, and NeedImage is called again as soon as the page is in view.
func (s *PageStrip) Invalidate(i int) {
	if i >= 0 && i < s.count {
		s.asked[i] = false
	}
}

// InvalidateAll marks every page stale (a new theme, a new document).
func (s *PageStrip) InvalidateAll() {
	for i := range s.asked {
		s.asked[i] = false
	}
}

// Resident is the number of bitmaps currently held.
func (s *PageStrip) Resident() int { return s.nres }

func (s *PageStrip) maxResident() int {
	if s.MaxResident > 0 {
		return s.MaxResident
	}
	return PageStripMaxResident
}

// evict drops the resident bitmaps farthest from keep until under budget; a
// dropped page is asked for again when it returns to view.
func (s *PageStrip) evict(keep int) {
	for s.nres > s.maxResident() {
		far, dist := -1, -1
		for i, im := range s.imgs {
			if im == nil || i == keep {
				continue
			}
			d := i - keep
			if d < 0 {
				d = -d
			}
			if d > dist {
				far, dist = i, d
			}
		}
		if far < 0 {
			return
		}
		s.imgs[far], s.asked[far] = nil, false
		s.nres--
	}
}

// --- layout -----------------------------------------------------------------

func (s *PageStrip) thumb() int {
	t := s.ThumbSize
	if t <= 0 {
		t = PageStripThumbSize
	}
	return scaled(t)
}

// numW is the room the page number takes beside (vertical) or under
// (horizontal) a card.
func (s *PageStrip) numW() int {
	if s.Horizontal {
		return s.glyphHeight() + scaled(6)
	}
	return s.textWidth(strconv.Itoa(s.count)) + scaled(8)
}

// cross is the card's size across the strip: ThumbSize, shrunk to fit.
func (s *PageStrip) cross() int {
	r := s.Bounds()
	avail := r.W
	if s.Horizontal {
		avail = r.H
	}
	avail -= 2*scaled(stripPad) + scrollbarTrack() + s.numW()
	c := s.thumb()
	if s.Fluid {
		lo, hi := s.MinThumbSize, s.MaxThumbSize
		if lo <= 0 {
			lo = 48
		}
		if hi <= 0 {
			hi = 480
		}
		c = avail
		if c > scaled(hi) {
			c = scaled(hi)
		}
		if c < scaled(lo) {
			c = scaled(lo)
		}
		if avail < c && avail >= 1 { // a strip narrower than the minimum still shows a whole card
			c = avail
		}
	} else if avail < c {
		c = avail
	}
	if c < 1 {
		c = 1
	}
	return c
}

func (s *PageStrip) aspect(i int) (w, h int) {
	if s.aspects != nil && s.aspects[i].w > 0 && s.aspects[i].h > 0 {
		return s.aspects[i].w, s.aspects[i].h
	}
	w, h = s.PageW, s.PageH
	if w <= 0 || h <= 0 {
		return 16, 9
	}
	return w, h
}

// layout (re)builds the offsets for the current cross size.
func (s *PageStrip) layout() {
	c := s.cross()
	if s.offs != nil && s.crossUsed == c && len(s.offs) == s.count+1 {
		return
	}
	// A strip resized under a scrolled window keeps its first visible page: the
	// scroll moves in proportion to the content.
	oldLen, oldCross := 0, s.crossUsed
	if s.offs != nil && len(s.offs) == s.count+1 && s.count > 0 {
		oldLen = s.offs[s.count] - scaled(stripGap) + scaled(stripPad)
	}
	s.crossUsed = c
	s.offs = make([]int, s.count+1)
	defer func() {
		if oldLen > 0 && oldCross != c {
			if newLen := s.contentLen(); newLen > 0 {
				s.Scroll().Set(int(int64(s.Scroll().Get()) * int64(newLen) / int64(oldLen)))
			}
		}
	}()
	pos := scaled(stripPad)
	gap := scaled(stripGap)
	for i := 0; i < s.count; i++ {
		s.offs[i] = pos
		w, h := s.aspect(i)
		main := c * h / w
		if s.Horizontal { // the card is as tall as the cross size says: main is its width
			main = c * w / h
		}
		if main < 1 {
			main = 1
		}
		pos += main + gap
	}
	s.offs[s.count] = pos // end of the last card plus the gap after it
}

// contentLen is the strip's whole length: every card, the gaps between them and
// the padding at both ends.
func (s *PageStrip) contentLen() int {
	s.layout()
	if s.count == 0 {
		return 0
	}
	return s.offs[s.count] - scaled(stripGap) + scaled(stripPad)
}

// cardMain is page i's size along the strip.
func (s *PageStrip) cardMain(i int) int {
	return s.offs[i+1] - s.offs[i] - scaled(stripGap)
}

func (s *PageStrip) viewLen() int {
	r := s.Bounds()
	if s.Horizontal {
		return r.W
	}
	return r.H
}

func (s *PageStrip) maxScroll() int {
	m := s.contentLen() - s.viewLen()
	if m < 0 {
		m = 0
	}
	return m
}

func (s *PageStrip) clampedScroll() int {
	sc := s.Scroll().Get()
	if sc < 0 {
		sc = 0
	}
	if m := s.maxScroll(); sc > m {
		sc = m
	}
	return sc
}

// window is the range [first, last] of pages intersecting the viewport, and
// ok false when there are none. O(log n).
func (s *PageStrip) window() (first, last int, ok bool) {
	s.layout()
	if s.count == 0 || s.viewLen() <= 0 {
		return 0, 0, false
	}
	sc := s.clampedScroll()
	first = sort.Search(s.count, func(i int) bool { return s.offs[i+1] > sc })
	last = sort.Search(s.count, func(i int) bool { return s.offs[i] >= sc+s.viewLen() }) - 1
	// The scroll is clamped, so the first page ends after it and starts before the
	// window's end: first <= last < count always holds here.
	return first, last, true
}

// Visible returns the first and last page intersecting the window (0, -1 when
// none) — what a host can use to prioritise its own rendering.
func (s *PageStrip) Visible() (first, last int) {
	f, l, ok := s.window()
	if !ok {
		return 0, -1
	}
	return f, l
}

// ScrollTo scrolls so that page i is fully in the window (as little as needed).
func (s *PageStrip) ScrollTo(i int) {
	if i < 0 || i >= s.count {
		return
	}
	s.layout()
	start, end := s.offs[i]-scaled(stripPad), s.offs[i+1]-scaled(stripGap)+scaled(stripPad)
	sc := s.clampedScroll()
	if start < sc {
		sc = start
	} else if end > sc+s.viewLen() {
		sc = end - s.viewLen()
	}
	s.Scroll().Set(sc)
	s.Scroll().Set(s.clampedScroll())
}

// cardRect is page i's card, widget-local (the picture, not its number).
func (s *PageStrip) cardRect(i int) Rect {
	s.layout()
	sc := s.clampedScroll()
	main := s.cardMain(i)
	c := s.crossUsed
	if s.Horizontal {
		return Rect{X: s.offs[i] - sc, Y: scaled(stripPad), W: main, H: c}
	}
	return Rect{X: scaled(stripPad) + s.numW(), Y: s.offs[i] - sc, W: c, H: main}
}

// PageAt returns the page under widget-local (x, y), -1 for none.
func (s *PageStrip) PageAt(x, y int) int {
	first, last, ok := s.window()
	if !ok {
		return -1
	}
	for i := first; i <= last; i++ {
		r := s.cardRect(i)
		if s.Horizontal {
			if x >= r.X && x < r.X+r.W+scaled(stripGap)/2 && y >= 0 && y < s.Bounds().H {
				return i
			}
		} else if y >= r.Y && y < r.Y+r.H+scaled(stripGap)/2 && x >= 0 && x < s.Bounds().W-scrollbarTrack() {
			return i
		}
	}
	return -1
}

// --- scrollbar --------------------------------------------------------------

func (s *PageStrip) barGeom() (sbGeom, bool) {
	s.layout()
	r := s.Bounds()
	if s.maxScroll() == 0 {
		return sbGeom{}, false
	}
	tr := scrollbarTrack()
	if s.Horizontal {
		return scrollGeom(true, r.H-tr, tr, 0, r.W, s.contentLen(), r.W, s.clampedScroll()), true
	}
	return scrollGeom(false, r.W-tr, tr, 0, r.H, s.contentLen(), r.H, s.clampedScroll()), true
}

// --- draw -------------------------------------------------------------------

// Draw paints the visible cards (and their numbers), the current page outlined,
// and the scrollbar. Requests the images the window and its neighbours lack.
func (s *PageStrip) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	if s.reveal {
		s.reveal = false
		s.ScrollTo(s.Current().Get())
	}
	first, last, ok := s.window()
	if ok {
		s.request(first, last)
		cur := s.Current().Get()
		sel := ensureContrast(theme.Accent, theme.Surface, theme.OnSurface, 3)
		withClip(p, r, func() {
			for i := first; i <= last; i++ {
				s.drawCard(p, theme, r, i, i == cur, sel)
			}
		})
	}
	if g, shown := s.barGeom(); shown {
		if s.Horizontal {
			paintScrollTrack(p, theme, r.X+g.trackStart, r.Y+g.cross0, g.trackLen, g.crossW)
			paintScrollThumb(p, theme, r.X+g.thumbStart, r.Y+g.cross0, g.thumbLen, g.crossW)
		} else {
			paintScrollTrack(p, theme, r.X+g.cross0, r.Y+g.trackStart, g.crossW, g.trackLen)
			paintScrollThumb(p, theme, r.X+g.cross0, r.Y+g.thumbStart, g.crossW, g.thumbLen)
		}
	}
	s.drawFocusRing(p, theme, r)
}

// request asks the host, once, for each bitmap-less page in the window and one
// page beyond each end of it.
func (s *PageStrip) request(first, last int) {
	if s.NeedImage == nil {
		return
	}
	lo, hi := first-stripPrefetch, last+stripPrefetch
	if lo < 0 {
		lo = 0
	}
	if hi >= s.count {
		hi = s.count - 1
	}
	for i := first; i <= last; i++ { // the window first, then the neighbours
		s.ask(i)
	}
	for i := lo; i < first; i++ {
		s.ask(i)
	}
	for i := last + 1; i <= hi; i++ {
		s.ask(i)
	}
}

func (s *PageStrip) ask(i int) {
	for len(s.askSize) < len(s.asked) {
		s.askSize = append(s.askSize, 0)
	}
	if s.asked[i] && s.Fluid && s.askSize[i] > 0 {
		if d := s.crossUsed - s.askSize[i]; d*4 >= s.askSize[i] || -d*4 >= s.askSize[i] {
			s.asked[i] = false
		}
	}
	if !s.asked[i] {
		s.asked[i] = true
		s.askSize[i] = s.crossUsed
		s.NeedImage(i)
	}
}

func (s *PageStrip) drawCard(p painter.Painter, theme *Theme, r Rect, i int, selected bool, sel RGBA) {
	cr := s.cardRect(i)
	cr.X += r.X
	cr.Y += r.Y
	// number
	num := strconv.Itoa(i + 1)
	ink := dimInk(theme)
	if selected {
		ink = theme.OnSurface
	}
	if s.Horizontal {
		s.drawText(p, cr.X+(cr.W-s.textWidth(num))/2, cr.Y+cr.H+scaled(3), num, ink)
	} else {
		s.drawText(p, r.X+scaled(stripPad), cr.Y+(cr.H-s.glyphHeight())/2, num, ink)
	}
	if im := s.imgs[i]; im != nil {
		fillRect(p, cr.X, cr.Y, cr.W, cr.H, theme.Background)
		blitImage(p, cr, cr, im.pix, im.w, im.h)
	} else if s.DrawPage != nil {
		fillRect(p, cr.X, cr.Y, cr.W, cr.H, theme.Background)
		withClip(p, cr, func() { s.DrawPage(p, theme, cr, i) })
	} else {
		fillRect(p, cr.X, cr.Y, cr.W, cr.H, theme.SurfaceAlt)
		s.drawText(p, cr.X+(cr.W-s.textWidth(num))/2, cr.Y+(cr.H-s.glyphHeight())/2, num, dimInk(theme))
	}
	if selected {
		o := atLeast1(scaled(2))
		for k := 0; k < o; k++ {
			strokeRect(p, cr.X-o+k, cr.Y-o+k, cr.W+2*(o-k), cr.H+2*(o-k), sel)
		}
	} else {
		strokeRect(p, cr.X, cr.Y, cr.W, cr.H, theme.Border)
	}
}

// --- events -----------------------------------------------------------------

// OnEvent: a press on the scrollbar grabs or pages it, a click on a card selects
// that page, the wheel scrolls, and (focused) the arrow, Page, Home and End keys
// move the selection.
func (s *PageStrip) OnEvent(ev Event) {
	if s.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventClick:
		g, ok := s.barGeom()
		if s.drag.press(g, ok, ev, s.viewLen(), s.scrollBy) {
			return
		}
		if i := s.PageAt(ev.X, ev.Y); i >= 0 {
			s.Current().Set(i)
		}
	case EventMouseDrag:
		g, ok := s.barGeom()
		s.drag.drag(g, ok, ev, func(v int) { s.Scroll().Set(v) })
	case EventMouseUp:
		s.drag.release()
	case EventScroll:
		d := ev.Delta
		if s.Horizontal && d == 0 {
			d = ev.DeltaX
		}
		s.scrollBy(d * s.step())
	case EventKeyDown:
		if s.focused {
			s.onKey(ev.Code)
		}
	}
}

// step is one wheel row: a third of a card.
func (s *PageStrip) step() int {
	s.layout()
	if s.count == 0 {
		return 1
	}
	return atLeast1((s.cardMain(0) + scaled(stripGap)) / 3)
}

func (s *PageStrip) scrollBy(d int) {
	s.Scroll().Set(s.clampedScroll() + d)
	s.Scroll().Set(s.clampedScroll())
}

func (s *PageStrip) onKey(code string) {
	cur, n := s.Current().Get(), s.count
	if n == 0 {
		return
	}
	first, last, _ := s.window()
	page := last - first
	if page < 1 {
		page = 1
	}
	switch code {
	case "ArrowDown", "ArrowRight":
		cur++
	case "ArrowUp", "ArrowLeft":
		cur--
	case "PageDown":
		cur += page
	case "PageUp":
		cur -= page
	case "Home":
		cur = 0
	case "End":
		cur = n - 1
	default:
		return
	}
	if cur < 0 {
		cur = 0
	}
	if cur >= n {
		cur = n - 1
	}
	s.Current().Set(cur)
}

// A11y reports the strip as a listbox whose value is "page N of M".
func (s *PageStrip) A11y() A11yInfo {
	v := ""
	if c := s.Current().Get(); c >= 0 && c < s.count {
		v = "page " + strconv.Itoa(c+1) + " of " + strconv.Itoa(s.count)
	}
	return A11yInfo{Role: RoleListbox, Value: v}
}
