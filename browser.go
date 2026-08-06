// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"math"
	"strings"

	"github.com/go-widgets/painter"
)

// Browser is a reusable mini web-browser chrome: a tab strip, a Back / Forward /
// Reload toolbar, an editable address field, a loading progress bar and a
// scrollable content area that shows a page render. It is deliberately
// renderer-agnostic and fully synchronous — the widget NEVER fetches or renders
// a page itself and imports no networking or HTML engine. Instead it exposes a
// seam: the host sets OnNavigate, and whenever the widget needs a page rendered
// it invokes OnNavigate(target, width). The host runs the actual fetch/render
// asynchronously elsewhere and calls back into the widget's synchronous Deliver
// and SetProgress methods on its own UI thread. This mirrors the proven
// callback seam used by other host-driven widgets and keeps the toolkit's
// zero-dependency, no-network-in-tests contract intact.
//
// Browser is a plain MVVM View: it holds view state, exposes it through
// exported getters (CurrentURL, CanBack, CanForward, TabCount, Loading,
// Progress, ActiveTitle, TabTitle), and offers command-style methods
// (Open, Navigate, Back, Forward, Reload, CloseTab) each with a matching Can…
// guard where relevant. A single OnChange hook fires whenever any
// observable-relevant state mutates, so a binder in the mvvm layer can push
// state into Observables WITHOUT the toolkit ever importing mvvm (which would
// invert the toolkit↔mvvm layering).
type Browser struct {
	Base

	// OnNavigate is the host's async fetch/render trigger. The widget calls it
	// with the target to render and the pixel width the content area currently
	// offers; the host renders off-thread and calls Deliver / SetProgress back.
	// Nil is safe (navigation still updates history + loading state).
	OnNavigate func(target string, width int)

	// OnOpenExternal, when set, is the seam for an "open in the system browser"
	// affordance: OpenExternal() invokes it with the current URL. Optional; nil
	// is safe.
	OnOpenExternal func(url string)

	// OnChange fires once whenever any observable-relevant state mutates
	// (navigation, tab add/close/switch, loading/progress change, address edit,
	// delivered page). A mvvm binder subscribes to push state into Observables.
	// It is additive to OnNavigate and never replaces it. Nil is safe.
	OnChange func()

	// Phase drives the indeterminate loading bar animation (0..1); advance it
	// from the host frame loop via Tick, exactly like Spinner.Phase.
	Phase float64

	tabs   []*browserTab
	active int
	mode   TabMode

	addrFocused bool
	addrBuf     string

	zoom float64 // page display scale; clamped to [BrowserMinZoom, BrowserMaxZoom]
}

// TabMode selects how Open allocates tabs.
type TabMode int

const (
	// MultiTab (the zero value / default) makes Open add a new tab and activate
	// it, evicting the oldest once BrowserMaxTabs is exceeded.
	MultiTab TabMode = iota
	// SingleTab makes Open reuse the single tab instead of adding one.
	SingleTab
)

// BrowserLink is one clickable region of a delivered page render. Rect is in
// RENDER-pixel coordinates (the coordinate space of the pixels the host handed
// to Deliver, at the render width the host was told to use); the widget maps a
// content-area click back into that space to hit-test it.
type BrowserLink struct {
	Rect image.Rectangle
	Href string
}

// browserTab is one tab's model: a visited-URL history with a cursor, the last
// delivered render (pixels + dimensions + the width it was rendered for), the
// link map, the title, the scroll offset and the per-tab loading state.
type browserTab struct {
	history []string // visited URLs, oldest first
	cursor  int      // index into history of the current URL (always valid: len>=1)
	title   string

	pixels     []byte // last delivered render, RGBA (imgW*imgH*4 bytes)
	imgW, imgH int    // render dimensions
	renderW    int    // width the render was produced for

	links  []BrowserLink
	scroll int // vertical scroll offset, in content pixels

	loading     bool
	progress    float64 // determinate download fraction, 0..1
	hasProgress bool    // SetProgress was called during this load
}

// Chrome sizing constants.
const (
	// BrowserTabStripH is the tab-strip row height (shown only in MultiTab with
	// at least two tabs).
	BrowserTabStripH = 24
	// BrowserToolbarH is the Back/Forward/Reload/address toolbar row height.
	BrowserToolbarH = 26
	// BrowserProgressH is the loading bar height across the content top.
	BrowserProgressH = 3
	// BrowserPadX / BrowserPadY are the toolbar's inner insets.
	BrowserPadX = 4
	BrowserPadY = 3
	// BrowserBtnGap is the gap between toolbar buttons.
	BrowserBtnGap = 4
	// BrowserBtnPad is the horizontal text inset inside a toolbar button / the
	// address field.
	BrowserBtnPad = 6
	// BrowserMaxTabs caps how many tabs MultiTab keeps; opening past it evicts
	// the oldest.
	BrowserMaxTabs = 12
	// BrowserScrollStep is the content pixels scrolled per wheel row.
	BrowserScrollStep = 24
	// BrowserTabGap is the gap between tab pills.
	BrowserTabGap = 2
	// BrowserTabCloseW is the width of a tab pill's × close box.
	BrowserTabCloseW = 14
)

// Page-zoom bounds. Zoom scales the already-delivered page bitmap for display
// only (it never re-fetches or re-renders): the content is drawn magnified or
// shrunk by the zoom factor, the scroll extent grows/shrinks with it, and link
// hit-testing maps back through the zoom.
const (
	// BrowserMinZoom / BrowserMaxZoom clamp the page-zoom factor.
	BrowserMinZoom = 0.5
	BrowserMaxZoom = 3.0
	// browserZoomStep is the increment ZoomIn / ZoomOut apply. Every zoom stop
	// (0.5 … 3.0) is an exact multiple of the step, so the clamp comparisons are
	// exact in binary floating point.
	browserZoomStep = 0.25
)

// Toolbar button labels. Plain ASCII so they render legibly on both the pixel
// back-end (5×7 bitmap font, which carries no guillemet or refresh glyph) and
// the terminal back-end.
const (
	browserBackLabel   = "< Back"
	browserFwdLabel    = "Fwd >"
	browserReloadLabel = "Reload"
	// browserZoomOutLabel / browserZoomInLabel are the page-zoom buttons. Plain
	// ASCII "-" / "+" render on both the pixel and terminal back-ends.
	browserZoomOutLabel = "-"
	browserZoomInLabel  = "+"
)

// NewBrowser builds an empty Browser in the default MultiTab mode at 1.0 zoom.
func NewBrowser() *Browser { return &Browser{zoom: 1.0} }

// changed fires OnChange when set. Every mutating path routes through it so a
// mvvm binder sees exactly one notification per state change.
func (b *Browser) changed() {
	if b.OnChange != nil {
		b.OnChange()
	}
}

// activeTab returns the active tab, or nil when there are no tabs.
func (b *Browser) activeTab() *browserTab {
	if b.active < 0 || b.active >= len(b.tabs) {
		return nil
	}
	return b.tabs[b.active]
}

// Tick advances Phase by deltaSeconds, wrapping modulo 1 (like Spinner.Tick), so
// the indeterminate loading bar animates in step with the host frame loop.
func (b *Browser) Tick(deltaSeconds float64) {
	b.Phase += deltaSeconds
	b.Phase -= math.Floor(b.Phase)
}

// SetTabMode selects MultiTab or SingleTab for subsequent Open calls.
func (b *Browser) SetTabMode(m TabMode) { b.mode = m }

// Mode reports the current TabMode.
func (b *Browser) Mode() TabMode { return b.mode }

// TabCount reports how many tabs are open.
func (b *Browser) TabCount() int { return len(b.tabs) }

// ActiveIndex reports the active tab index (0 when there are no tabs).
func (b *Browser) ActiveIndex() int { return b.active }

// CurrentURL returns the active tab's current URL, or "" when there are no tabs.
func (b *Browser) CurrentURL() string {
	t := b.activeTab()
	if t == nil {
		return ""
	}
	return t.history[t.cursor]
}

// CanBack reports whether the active tab can go back (history behind the cursor).
func (b *Browser) CanBack() bool {
	t := b.activeTab()
	return t != nil && t.cursor > 0
}

// CanForward reports whether the active tab can go forward (history ahead of the
// cursor).
func (b *Browser) CanForward() bool {
	t := b.activeTab()
	return t != nil && t.cursor < len(t.history)-1
}

// Loading reports whether the active tab has an in-flight load.
func (b *Browser) Loading() bool {
	t := b.activeTab()
	return t != nil && t.loading
}

// Progress reports the active tab's determinate download fraction (0 when there
// is no tab or SetProgress was never called this load).
func (b *Browser) Progress() float64 {
	t := b.activeTab()
	if t == nil {
		return 0
	}
	return t.progress
}

// ActiveTitle returns the active tab's title, falling back to its URL when the
// title is empty; "" when there are no tabs.
func (b *Browser) ActiveTitle() string {
	t := b.activeTab()
	if t == nil {
		return ""
	}
	return tabDisplayTitle(t)
}

// TabTitle returns tab i's display title (title, or its URL when the title is
// empty); "" for an out-of-range index.
func (b *Browser) TabTitle(i int) string {
	if i < 0 || i >= len(b.tabs) {
		return ""
	}
	return tabDisplayTitle(b.tabs[i])
}

// tabDisplayTitle is a tab's title, or its current URL when the title is empty.
func tabDisplayTitle(t *browserTab) string {
	if t.title != "" {
		return t.title
	}
	return t.history[t.cursor]
}

// Open opens target in a tab. In MultiTab it adds a new active tab (evicting the
// oldest past BrowserMaxTabs); in SingleTab it replaces the one tab. It seeds
// history, marks the tab loading, sets a pending render width and invokes
// OnNavigate.
func (b *Browser) Open(target, title string) {
	nt := &browserTab{history: []string{target}, cursor: 0, title: title}
	if b.mode == SingleTab {
		if len(b.tabs) > 0 {
			b.tabs[0] = nt
		} else {
			b.tabs = append(b.tabs, nt)
		}
		b.active = 0
	} else {
		b.tabs = append(b.tabs, nt)
		if len(b.tabs) > BrowserMaxTabs {
			b.tabs = b.tabs[1:]
		}
		b.active = len(b.tabs) - 1
	}
	b.startLoad(nt, target)
}

// Navigate performs in-tab navigation to href (a link click or a typed
// address): it truncates any forward history, appends href, marks the tab
// loading and invokes OnNavigate. With no active tab it falls back to Open.
func (b *Browser) Navigate(href string) {
	t := b.activeTab()
	if t == nil {
		b.Open(href, "")
		return
	}
	t.history = append(t.history[:t.cursor+1], href)
	t.cursor = len(t.history) - 1
	b.startLoad(t, href)
}

// Back moves the active tab's cursor one step back and re-fetches that URL. It
// is a no-op with no active tab or at the start of history.
func (b *Browser) Back() {
	t := b.activeTab()
	if t == nil || t.cursor <= 0 {
		return
	}
	t.cursor--
	b.startLoad(t, t.history[t.cursor])
}

// Forward moves the active tab's cursor one step forward and re-fetches that
// URL. It is a no-op with no active tab or at the end of history.
func (b *Browser) Forward() {
	t := b.activeTab()
	if t == nil || t.cursor >= len(t.history)-1 {
		return
	}
	t.cursor++
	b.startLoad(t, t.history[t.cursor])
}

// Reload re-fetches the active tab's current URL. It is a no-op with no active
// tab.
func (b *Browser) Reload() {
	t := b.activeTab()
	if t == nil {
		return
	}
	b.startLoad(t, t.history[t.cursor])
}

// CloseTab drops tab i and its state; if it was the active tab a neighbour is
// activated. Out-of-range indices are ignored.
func (b *Browser) CloseTab(i int) {
	if i < 0 || i >= len(b.tabs) {
		return
	}
	b.tabs = append(b.tabs[:i], b.tabs[i+1:]...)
	if len(b.tabs) == 0 {
		b.active = 0
	} else if i < b.active {
		b.active--
	} else if i == b.active && b.active >= len(b.tabs) {
		b.active = len(b.tabs) - 1
	}
	b.changed()
}

// Deliver hands the widget a finished render for target. When target matches the
// active tab's current URL the render (pixels + dimensions + width), links and
// title are stored, loading is cleared and scroll is reset; otherwise it is
// ignored (a stale or non-active delivery).
func (b *Browser) Deliver(target string, pixels []byte, imgW, imgH, width int, links []BrowserLink, title string) {
	t := b.activeTab()
	if t == nil || t.history[t.cursor] != target {
		return
	}
	t.pixels = pixels
	t.imgW, t.imgH = imgW, imgH
	t.renderW = width
	t.links = links
	t.title = title
	t.loading = false
	t.scroll = 0
	b.changed()
}

// SetProgress sets the active tab's determinate download progress (clamped to
// 0..1) for the in-flight load. If it is never called during a load the bar
// renders indeterminate (driven by Phase). No-op with no active tab.
func (b *Browser) SetProgress(frac float64) {
	t := b.activeTab()
	if t == nil {
		return
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	t.hasProgress = true
	t.progress = frac
	b.changed()
}

// OpenExternal invokes OnOpenExternal with the current URL, the seam for an
// "open in the system browser" affordance. No-op when the hook is unset or there
// is no current URL.
func (b *Browser) OpenExternal() {
	if b.OnOpenExternal == nil {
		return
	}
	if u := b.CurrentURL(); u != "" {
		b.OnOpenExternal(u)
	}
}

// --- zoom ----------------------------------------------------------------

// Zoom reports the current page-display zoom factor (1.0 is 1:1 fit-to-width).
func (b *Browser) Zoom() float64 { return b.zoom }

// CanZoomIn reports whether the zoom can still increase (below BrowserMaxZoom).
func (b *Browser) CanZoomIn() bool { return b.zoom < BrowserMaxZoom }

// CanZoomOut reports whether the zoom can still decrease (above BrowserMinZoom).
func (b *Browser) CanZoomOut() bool { return b.zoom > BrowserMinZoom }

// ZoomIn increases the zoom by one step (no-op at BrowserMaxZoom).
func (b *Browser) ZoomIn() { b.SetZoom(b.zoom + browserZoomStep) }

// ZoomOut decreases the zoom by one step (no-op at BrowserMinZoom).
func (b *Browser) ZoomOut() { b.SetZoom(b.zoom - browserZoomStep) }

// ResetZoom returns the zoom to 1.0 (no-op when already there).
func (b *Browser) ResetZoom() { b.SetZoom(1.0) }

// SetZoom sets the page-display zoom, clamped to [BrowserMinZoom, BrowserMaxZoom].
// A real change re-clamps the active tab's scroll to the new (smaller) extent and
// fires OnChange; setting the current value is a no-op (no notification).
func (b *Browser) SetZoom(f float64) {
	if f < BrowserMinZoom {
		f = BrowserMinZoom
	}
	if f > BrowserMaxZoom {
		f = BrowserMaxZoom
	}
	if f == b.zoom {
		return
	}
	b.zoom = f
	if t := b.activeTab(); t != nil {
		if m := b.maxScroll(t, b.contentRect()); t.scroll > m {
			t.scroll = m
		}
	}
	b.changed()
}

// startLoad marks t loading for target, resets its progress, records the pending
// render width, notifies OnChange and triggers the host render via OnNavigate.
func (b *Browser) startLoad(t *browserTab, target string) {
	t.loading = true
	t.hasProgress = false
	t.progress = 0
	t.renderW = b.renderWidth()
	b.changed()
	if b.OnNavigate != nil {
		b.OnNavigate(target, t.renderW)
	}
}

// --- geometry ------------------------------------------------------------

// showTabStrip reports whether the tab strip is drawn: MultiTab with at least
// two tabs.
func (b *Browser) showTabStrip() bool {
	return b.mode == MultiTab && len(b.tabs) >= 2
}

// stripH is the tab strip's height (0 when it is not shown).
func (b *Browser) stripH() int {
	if b.showTabStrip() {
		return BrowserTabStripH
	}
	return 0
}

// tabStripRect is the tab-strip row (zero height when not shown).
func (b *Browser) tabStripRect() Rect {
	r := b.Bounds()
	return Rect{X: r.X, Y: r.Y, W: r.W, H: b.stripH()}
}

// toolbarRect is the Back/Forward/Reload/address row.
func (b *Browser) toolbarRect() Rect {
	r := b.Bounds()
	return Rect{X: r.X, Y: r.Y + b.stripH(), W: r.W, H: BrowserToolbarH}
}

// contentRect is the page area below the toolbar (height clamped at 0).
func (b *Browser) contentRect() Rect {
	r := b.Bounds()
	top := b.stripH() + BrowserToolbarH
	h := r.H - top
	if h < 0 {
		h = 0
	}
	return Rect{X: r.X, Y: r.Y + top, W: r.W, H: h}
}

// renderWidth is the pixel width a page is rendered for — the content width.
func (b *Browser) renderWidth() int { return b.contentRect().W }

// toolbarLayout returns the Back, Forward, Reload, zoom-out and zoom-in button
// rects and the address field rect for the current toolbar. Button widths follow
// their label; the address field fills the remainder.
func (b *Browser) toolbarLayout() (back, fwd, reload, zoomOut, zoomIn, addr Rect) {
	tr := b.toolbarRect()
	btnY := tr.Y + BrowserPadY
	btnH := tr.H - 2*BrowserPadY
	x := tr.X + BrowserPadX
	place := func(label string) Rect {
		w := b.textWidth(label) + 2*BrowserBtnPad
		rc := Rect{X: x, Y: btnY, W: w, H: btnH}
		x += w + BrowserBtnGap
		return rc
	}
	back = place(browserBackLabel)
	fwd = place(browserFwdLabel)
	reload = place(browserReloadLabel)
	zoomOut = place(browserZoomOutLabel)
	zoomIn = place(browserZoomInLabel)
	addrW := tr.X + tr.W - BrowserPadX - x
	if addrW < 0 {
		addrW = 0
	}
	addr = Rect{X: x, Y: btnY, W: addrW, H: btnH}
	return
}

// tabRects returns the pill rect of every tab, dividing the strip evenly.
func (b *Browser) tabRects() []Rect {
	sr := b.tabStripRect()
	n := len(b.tabs)
	if n == 0 {
		return nil
	}
	pillW := sr.W / n
	rects := make([]Rect, n)
	for i := 0; i < n; i++ {
		w := pillW - BrowserTabGap
		if w < 1 {
			w = 1
		}
		rects[i] = Rect{X: sr.X + i*pillW, Y: sr.Y + 2, W: w, H: sr.H - 4}
	}
	return rects
}

// tabCloseRect is the × close box within a tab pill.
func tabCloseRect(pill Rect) Rect {
	return Rect{X: pill.X + pill.W - BrowserTabCloseW - BrowserPadX, Y: pill.Y, W: BrowserTabCloseW, H: pill.H}
}

// pageDisplaySize is t's on-screen render size in content cr: the fit-to-width
// base (dispW = cr.W) scaled by the current zoom, with the height following the
// render's aspect ratio. It returns 0, 0 when there is no render yet.
func (b *Browser) pageDisplaySize(t *browserTab, cr Rect) (dispW, dispH int) {
	if t.imgW <= 0 {
		return 0, 0
	}
	dispW = int(float64(cr.W) * b.zoom)
	dispH = t.imgH * dispW / t.imgW
	return dispW, dispH
}

// maxScroll is the largest vertical scroll offset for t within content cr (0
// when the page fits or has no render). It follows the zoomed display height.
func (b *Browser) maxScroll(t *browserTab, cr Rect) int {
	_, dispH := b.pageDisplaySize(t, cr)
	m := dispH - cr.H
	if m < 0 {
		m = 0
	}
	return m
}

// --- drawing -------------------------------------------------------------

// Draw paints the chrome (tab strip when shown, toolbar) and the content area
// (page render + loading bar), strictly within Bounds.
func (b *Browser) Draw(p painter.Painter, theme *Theme) {
	r := b.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Background)
	if b.showTabStrip() {
		b.drawTabStrip(p, theme)
	}
	b.drawToolbar(p, theme)
	b.drawContent(p, theme)
}

// drawTabStrip paints the strip background and every tab pill.
func (b *Browser) drawTabStrip(p painter.Painter, theme *Theme) {
	sr := b.tabStripRect()
	fillRect(p, sr.X, sr.Y, sr.W, sr.H, theme.SurfaceAlt)
	for i, pill := range b.tabRects() {
		t := b.tabs[i]
		fill, ring := theme.SurfaceAlt, theme.Border
		if i == b.active {
			fill, ring = theme.Surface, theme.Accent
		}
		fillRoundRect(p, pill.X, pill.Y, pill.W, pill.H, 6, fill)
		strokeRoundRect(p, pill.X, pill.Y, pill.W, pill.H, 6, ring)
		ty := pill.Y + (pill.H-b.glyphHeight())/2
		title := tabDisplayTitle(t)
		avail := pill.W - 2*BrowserBtnPad - BrowserTabCloseW
		if b.textWidth(title) > avail {
			title = ellipsize(b.EffectiveFont(), title, avail)
		}
		b.drawText(p, pill.X+BrowserBtnPad, ty, title, theme.OnSurface)
		cxr := tabCloseRect(pill)
		b.drawText(p, cxr.X+(cxr.W-b.textWidth("x"))/2, ty, "x", theme.Border)
	}
}

// drawToolbar paints the toolbar background, the three buttons and the address
// field.
func (b *Browser) drawToolbar(p painter.Painter, theme *Theme) {
	tr := b.toolbarRect()
	fillRect(p, tr.X, tr.Y, tr.W, tr.H, theme.SurfaceAlt)
	back, fwd, reload, zoomOut, zoomIn, addr := b.toolbarLayout()
	b.drawButton(p, theme, back, browserBackLabel, b.CanBack())
	b.drawButton(p, theme, fwd, browserFwdLabel, b.CanForward())
	b.drawButton(p, theme, reload, browserReloadLabel, b.activeTab() != nil)
	b.drawButton(p, theme, zoomOut, browserZoomOutLabel, b.CanZoomOut())
	b.drawButton(p, theme, zoomIn, browserZoomInLabel, b.CanZoomIn())
	b.drawAddress(p, theme, addr)
}

// drawButton paints one rounded toolbar button; a disabled button reads muted.
func (b *Browser) drawButton(p painter.Painter, theme *Theme, r Rect, label string, enabled bool) {
	fill, ink := theme.Surface, theme.OnSurface
	if !enabled {
		fill, ink = mutedFace(theme), mutedInk(theme)
	}
	fillRoundRect(p, r.X, r.Y, r.W, r.H, 4, fill)
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, 4, theme.Border)
	tx := r.X + (r.W-b.textWidth(label))/2
	ty := r.Y + (r.H-b.glyphHeight())/2
	b.drawText(p, tx, ty, label, ink)
}

// drawAddress paints the editable address field: the edit buffer when focused
// (with a caret + Accent focus ring) or the current URL otherwise, head-clipped
// so a long URL keeps its tail (the path) visible.
func (b *Browser) drawAddress(p painter.Painter, theme *Theme, r Rect) {
	fillRoundRect(p, r.X, r.Y, r.W, r.H, 4, theme.Surface)
	ring := theme.Border
	if b.addrFocused {
		ring = theme.Accent
	}
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, 4, ring)
	text := b.CurrentURL()
	if b.addrFocused {
		text = b.addrBuf
	}
	innerX := r.X + BrowserBtnPad
	avail := r.W - 2*BrowserBtnPad
	shown := text
	if b.textWidth(shown) > avail {
		shown = clipHeadToWidth(b.EffectiveFont(), shown, avail)
	}
	ty := r.Y + (r.H-b.glyphHeight())/2
	b.drawText(p, innerX, ty, shown, theme.OnSurface)
	if b.addrFocused {
		caretW := b.glyphHeight() / 12
		if caretW < 1 {
			caretW = 1
		}
		fillRect(p, innerX+b.textWidth(shown), ty, caretW, b.glyphHeight(), theme.OnSurface)
	}
}

// drawContent paints the content background, the page render and the loading
// bar.
func (b *Browser) drawContent(p painter.Painter, theme *Theme) {
	cr := b.contentRect()
	if cr.W <= 0 || cr.H <= 0 {
		return
	}
	fillRect(p, cr.X, cr.Y, cr.W, cr.H, theme.Background)
	t := b.activeTab()
	if t == nil {
		return
	}
	b.drawPage(p, cr, t)
	if t.loading {
		b.drawProgress(p, theme, cr, t)
	}
}

// drawPage blits t's render scaled to the zoomed display size, top-aligned and
// offset by the tab's scroll; rows outside the content rect are skipped and
// columns past the content width are clipped (zoom > 1 makes the page wider than
// cr) so the blit stays within cr.
func (b *Browser) drawPage(p painter.Painter, cr Rect, t *browserTab) {
	if t.imgW <= 0 || t.imgH <= 0 || len(t.pixels) < t.imgW*t.imgH*4 {
		return
	}
	dispW, dispH := b.pageDisplaySize(t, cr)
	if dispH < 1 {
		return
	}
	for vy := 0; vy < dispH; vy++ {
		sy := cr.Y + vy - t.scroll
		if sy < cr.Y || sy >= cr.Y+cr.H {
			continue
		}
		base := (vy * t.imgH / dispH) * t.imgW
		for vx := 0; vx < dispW; vx++ {
			if vx >= cr.W {
				break
			}
			off := (base + vx*t.imgW/dispW) * 4
			p.PutPixel(cr.X+vx, sy, RGBA{R: t.pixels[off], G: t.pixels[off+1], B: t.pixels[off+2], A: t.pixels[off+3]})
		}
	}
}

// drawProgress paints the loading bar across the content top: determinate when
// SetProgress was called this load, else indeterminate driven by Phase.
func (b *Browser) drawProgress(p painter.Painter, theme *Theme, cr Rect, t *browserTab) {
	pb := &ProgressBar{}
	pb.SetBounds(Rect{X: cr.X, Y: cr.Y, W: cr.W, H: BrowserProgressH})
	if t.hasProgress {
		pb.Fraction = t.progress
	} else {
		pb.Indeterminate = true
		pb.Phase = b.Phase
	}
	pb.Draw(p, theme)
}

// clipHeadToWidth trims s from the FRONT until it fits width in font f, prefixing
// a leading ellipsis — so a long URL keeps its tail (the path) visible.
func clipHeadToWidth(f Font, s string, width int) string {
	const ell = "…"
	runes := []rune(s)
	for len(runes) > 0 {
		if f.Measure(ell+string(runes)) <= width {
			return ell + string(runes)
		}
		runes = runes[1:]
	}
	return ell
}

// normalizeURL prefixes https:// to a bare address that carries no scheme, so a
// typed "example.com" becomes a fetchable URL.
func normalizeURL(s string) string {
	if strings.Contains(s, "://") {
		return s
	}
	return "https://" + s
}

// --- input ---------------------------------------------------------------

// OnEvent routes widget-local input: clicks to the tab strip / toolbar / address
// field / page links, character + Backspace + Enter to the focused address
// field, and wheel scroll to the content. It early-returns when Disabled.
func (b *Browser) OnEvent(ev Event) {
	if b.Disabled {
		return
	}
	r := b.Bounds()
	ax, ay := ev.X+r.X, ev.Y+r.Y
	switch ev.Kind {
	case EventClick:
		b.handleClick(ax, ay)
	case EventChar:
		if !b.addrFocused || ev.Code == "" {
			return
		}
		b.addrBuf += ev.Code
		b.changed()
	case EventKeyDown:
		if !b.addrFocused {
			return
		}
		switch ev.Code {
		case "Backspace":
			runes := []rune(b.addrBuf)
			if len(runes) == 0 {
				return
			}
			b.addrBuf = string(runes[:len(runes)-1])
			b.changed()
		case "Enter":
			b.commitAddress()
		}
	case EventScroll:
		t := b.activeTab()
		if t == nil {
			return
		}
		t.scroll += ev.Delta * BrowserScrollStep
		if t.scroll < 0 {
			t.scroll = 0
		}
		if m := b.maxScroll(t, b.contentRect()); t.scroll > m {
			t.scroll = m
		}
		b.changed()
	}
}

// commitAddress normalises + navigates to the address buffer on Enter, then
// defocuses the field. An empty buffer is a no-op (it just defocuses).
func (b *Browser) commitAddress() {
	raw := strings.TrimSpace(b.addrBuf)
	b.addrFocused = false
	if raw == "" {
		b.changed()
		return
	}
	b.Navigate(normalizeURL(raw))
}

// handleClick routes an absolute-coordinate click to the tab strip, a toolbar
// button, the address field or a page link.
func (b *Browser) handleClick(ax, ay int) {
	if b.showTabStrip() && b.tabStripRect().Contains(ax, ay) {
		for i, pill := range b.tabRects() {
			if pill.Contains(ax, ay) {
				if tabCloseRect(pill).Contains(ax, ay) {
					b.CloseTab(i)
				} else {
					b.active = i
					b.addrFocused = false
					b.changed()
				}
				return
			}
		}
		return
	}
	back, fwd, reload, zoomOut, zoomIn, addr := b.toolbarLayout()
	switch {
	case back.Contains(ax, ay):
		b.addrFocused = false
		if b.CanBack() {
			b.Back()
		}
		return
	case fwd.Contains(ax, ay):
		b.addrFocused = false
		if b.CanForward() {
			b.Forward()
		}
		return
	case reload.Contains(ax, ay):
		b.addrFocused = false
		if b.activeTab() != nil {
			b.Reload()
		}
		return
	case zoomOut.Contains(ax, ay):
		b.addrFocused = false
		if b.CanZoomOut() {
			b.ZoomOut()
		}
		return
	case zoomIn.Contains(ax, ay):
		b.addrFocused = false
		if b.CanZoomIn() {
			b.ZoomIn()
		}
		return
	case addr.Contains(ax, ay):
		b.addrFocused = true
		b.addrBuf = b.CurrentURL()
		b.changed()
		return
	}
	cr := b.contentRect()
	if cr.Contains(ax, ay) {
		b.addrFocused = false
		if t := b.activeTab(); t != nil {
			if href, ok := b.linkAt(t, cr, ax, ay); ok {
				b.Navigate(href)
			}
		}
		return
	}
	b.addrFocused = false
}

// linkAt maps an absolute content-area click back into render-pixel space and
// returns the Href of the first link whose Rect contains it.
func (b *Browser) linkAt(t *browserTab, cr Rect, ax, ay int) (string, bool) {
	if t.imgW <= 0 || t.imgH <= 0 {
		return "", false
	}
	dispW, dispH := b.pageDisplaySize(t, cr)
	if dispH <= 0 {
		return "", false
	}
	rx := (ax - cr.X) * t.imgW / dispW
	ry := (ay - cr.Y + t.scroll) * t.imgH / dispH
	pt := image.Pt(rx, ry)
	for _, ln := range t.links {
		if pt.In(ln.Rect) {
			return ln.Href, true
		}
	}
	return "", false
}
