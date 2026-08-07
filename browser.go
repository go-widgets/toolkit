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

	// BackIcon / ForwardIcon / ReloadIcon / ZoomOutIcon / ZoomInIcon are the
	// host-supplied vector-icon painters for the toolbar buttons — the same seam
	// as SearchEntry.Icon. Each is invoked with its button's rect and the button
	// face ink (which already carries the enabled / disabled tint), so the host
	// draws a real arrow / refresh / minus / plus glyph centred in the button.
	// The toolkit ships no icon set of its own (keeping its zero-dependency
	// contract); a host wires these to, e.g., an Iconoir binding. Each is
	// nil-safe: a nil hook falls back to the plain text label, so headless
	// renders and existing callers keep working unchanged.
	BackIcon    func(p painter.Painter, r Rect, ink RGBA)
	ForwardIcon func(p painter.Painter, r Rect, ink RGBA)
	ReloadIcon  func(p painter.Painter, r Rect, ink RGBA)
	ZoomOutIcon func(p painter.Painter, r Rect, ink RGBA)
	ZoomInIcon  func(p painter.Painter, r Rect, ink RGBA)

	// LeadingIcon, when set, paints a status glyph at the LEFT of the address
	// field — e.g. an SSL padlock whose look the host varies by certificate state
	// (secure / insecure / none). Same painter seam as the toolbar icons; the
	// address text indents to its right. Nil → no leading slot (text starts at
	// the normal inset).
	LeadingIcon func(p painter.Painter, r Rect, ink RGBA)

	// BookmarkIcon, when set, paints a toggle glyph at the RIGHT of the address
	// field — e.g. a star, filled when on. It takes the current Bookmarked state
	// so the host can draw the on/off variant. Clicking the slot flips Bookmarked
	// and fires OnBookmarkToggle. Nil → no bookmark slot.
	BookmarkIcon     func(p painter.Painter, r Rect, ink RGBA, on bool)
	Bookmarked       bool
	OnBookmarkToggle func(on bool)

	// OnChange fires once whenever any observable-relevant state mutates
	// (navigation, tab add/close/switch, loading/progress change, address edit,
	// delivered page). A mvvm binder subscribes to push state into Observables.
	// It is additive to OnNavigate and never replaces it. Nil is safe.
	OnChange func()

	// Phase drives the indeterminate loading bar animation (0..1); advance it
	// from the host frame loop via Tick, exactly like Spinner.Phase.
	Phase float64

	// Scale multiplies every chrome metric — the tab-strip and toolbar heights,
	// the pads, the button squares, the address slot, the loading-bar thickness
	// and the tab-pill sizing — for HiDPI / device-pixel hosts. A host that lays
	// the widget out in DEVICE pixels (e.g. a Retina surface at devicePixelRatio
	// 2, optionally times a UI zoom) sets Scale = devicePixelRatio*zoom so the
	// chrome stays physically the right size instead of shrinking to half. The
	// zero value (and 1) mean "no scaling": layout is byte-identical to a build
	// without the field, so existing callers are unaffected. Values <= 0 are
	// treated as 1. Metrics are rounded (not truncated) at every use so the
	// scaled buttons/tabs stay pixel-aligned and do not drift. The host-supplied
	// icon hooks fill the now-larger button rects, so the glyphs grow with the
	// buttons automatically.
	Scale float64

	// HideChrome, when true, hides BOTH the toolbar and the tab strip: neither is
	// drawn and neither takes any vertical space, so the page content area fills
	// the entire widget bounds. Toolbar clicks and address-field editing are then
	// inert (there are no hit targets), but navigation still works
	// programmatically (Open / Navigate / Back / Forward / Reload / SetZoom / …),
	// so a host can drive a chromeless page view. The loading bar still shows over
	// the content while a load is in flight. The zero value is false → the chrome
	// is shown exactly as before, so existing callers are unaffected.
	HideChrome bool

	tabs   []*browserTab
	active int
	mode   TabMode

	addrFocused bool
	addrBuf     string
	// addrCopied marks that the address was just copied (CopyAddress), so the
	// field paints a select-all highlight as visual feedback of what went to the
	// clipboard. Cleared on any edit, navigation or focus change.
	addrCopied bool

	// pressActive + pressKind give a toolbar button its click feedback: a click
	// (EventClick) arms pressActive on the hit button's kind so the next Draw
	// paints that button's pressed face, and the release (EventMouseUp) clears it.
	// The button instances are rebuilt each frame, so the pressed state lives here
	// on the persistent Browser rather than on the ephemeral Button.
	pressActive bool
	pressKind   browserBtnKind

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
	// BrowserToolbarH is the Back/Forward/Reload/address toolbar row height. It
	// is deliberately tall enough that a button box (BrowserToolbarH - 2*PadY)
	// is a comfortable ~34px square at scale 1 — a real, clickable icon button
	// rather than a cramped text chip. Multiply by Browser.Scale on HiDPI hosts.
	BrowserToolbarH = 44
	// BrowserProgressH is the loading bar height across the content top.
	BrowserProgressH = 3
	// BrowserPadX / BrowserPadY are the toolbar's inner insets. PadY is generous
	// so the buttons sit centred in the taller row with breathing room above and
	// below.
	BrowserPadX = 4
	BrowserPadY = 5
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

// browserBtnKind identifies one toolbar button so layout, drawing and
// hit-testing all share a single definition of each button's label, icon hook
// and enabled state.
type browserBtnKind int

const (
	browserBtnBack browserBtnKind = iota
	browserBtnFwd
	browserBtnReload
	browserBtnZoomOut
	browserBtnZoomIn
)

// btnLabel is a toolbar button's text-fallback label (drawn when its icon hook
// is nil).
func (b *Browser) btnLabel(k browserBtnKind) string {
	switch k {
	case browserBtnBack:
		return browserBackLabel
	case browserBtnFwd:
		return browserFwdLabel
	case browserBtnReload:
		return browserReloadLabel
	case browserBtnZoomOut:
		return browserZoomOutLabel
	default: // browserBtnZoomIn
		return browserZoomInLabel
	}
}

// btnIcon returns the host-supplied icon painter for a toolbar button, or nil
// when the host did not wire one (the text label is then used).
func (b *Browser) btnIcon(k browserBtnKind) func(p painter.Painter, r Rect, ink RGBA) {
	switch k {
	case browserBtnBack:
		return b.BackIcon
	case browserBtnFwd:
		return b.ForwardIcon
	case browserBtnReload:
		return b.ReloadIcon
	case browserBtnZoomOut:
		return b.ZoomOutIcon
	default: // browserBtnZoomIn
		return b.ZoomInIcon
	}
}

// btnEnabled reports whether a toolbar button is currently actionable: Back /
// Forward follow the history ends, Reload needs an active tab, and the zoom
// buttons follow the zoom clamps.
func (b *Browser) btnEnabled(k browserBtnKind) bool {
	switch k {
	case browserBtnBack:
		return b.CanBack()
	case browserBtnFwd:
		return b.CanForward()
	case browserBtnReload:
		return b.activeTab() != nil
	case browserBtnZoomOut:
		return b.CanZoomOut()
	default: // browserBtnZoomIn
		return b.CanZoomIn()
	}
}

// NewBrowser builds an empty Browser in the default MultiTab mode at 1.0 zoom.
func NewBrowser() *Browser { return &Browser{zoom: 1.0} }

// scale is the effective chrome scale factor: Scale when positive, else 1 (the
// zero value and any non-positive value mean "no scaling").
func (b *Browser) scale() float64 {
	if b.Scale <= 0 {
		return 1
	}
	return b.Scale
}

// sc scales a base chrome metric by the effective scale, rounding to the nearest
// pixel so the scaled buttons/tabs stay pixel-aligned and never drift. At scale 1
// it returns v unchanged (math.Round(v*1) == v), keeping the default layout
// byte-identical.
func (b *Browser) sc(v int) int { return int(math.Round(float64(v) * b.scale())) }

// Scaled chrome metrics: every layout/draw/hit-test site reads geometry through
// these so a single scale factor moves them all together.
func (b *Browser) tabStripH() int { return b.sc(BrowserTabStripH) }
func (b *Browser) toolbarH() int  { return b.sc(BrowserToolbarH) }
func (b *Browser) padX() int      { return b.sc(BrowserPadX) }
func (b *Browser) padY() int      { return b.sc(BrowserPadY) }
func (b *Browser) btnGap() int    { return b.sc(BrowserBtnGap) }
func (b *Browser) btnPad() int    { return b.sc(BrowserBtnPad) }
func (b *Browser) progressH() int { return b.sc(BrowserProgressH) }
func (b *Browser) tabGap() int    { return b.sc(BrowserTabGap) }
func (b *Browser) tabCloseW() int { return b.sc(BrowserTabCloseW) }

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

// showTabStrip reports whether the tab strip is drawn: chrome visible, MultiTab
// mode and at least two tabs.
func (b *Browser) showTabStrip() bool {
	return !b.HideChrome && b.mode == MultiTab && len(b.tabs) >= 2
}

// stripH is the tab strip's height (0 when it is not shown).
func (b *Browser) stripH() int {
	if b.showTabStrip() {
		return b.tabStripH()
	}
	return 0
}

// toolbarSpace is the vertical space the toolbar occupies: the scaled toolbar
// height when the chrome is shown, 0 when HideChrome hides it.
func (b *Browser) toolbarSpace() int {
	if b.HideChrome {
		return 0
	}
	return b.toolbarH()
}

// tabStripRect is the tab-strip row (zero height when not shown).
func (b *Browser) tabStripRect() Rect {
	r := b.Bounds()
	return Rect{X: r.X, Y: r.Y, W: r.W, H: b.stripH()}
}

// toolbarRect is the Back/Forward/Reload/address row (zero height when the chrome
// is hidden).
func (b *Browser) toolbarRect() Rect {
	r := b.Bounds()
	return Rect{X: r.X, Y: r.Y + b.stripH(), W: r.W, H: b.toolbarSpace()}
}

// contentRect is the page area below the chrome (height clamped at 0). With the
// chrome hidden it spans the whole widget bounds.
func (b *Browser) contentRect() Rect {
	r := b.Bounds()
	top := b.stripH() + b.toolbarSpace()
	h := r.H - top
	if h < 0 {
		h = 0
	}
	return Rect{X: r.X, Y: r.Y + top, W: r.W, H: h}
}

// renderWidth is the pixel width a page is rendered for — the content width.
func (b *Browser) renderWidth() int { return b.contentRect().W }

// toolbarLayout returns the Back, Forward, Reload, zoom-out and zoom-in button
// rects and the address field rect for the current toolbar. A button whose icon
// hook is set is laid out as a square (side = the toolbar's inner height) so the
// icons form an even row of real buttons; a button with no icon is sized to its
// text label so the fallback stays legible. The address field fills the
// remainder to the right pad.
func (b *Browser) toolbarLayout() (back, fwd, reload, zoomOut, zoomIn, addr Rect) {
	tr := b.toolbarRect()
	padY, padX, gap := b.padY(), b.padX(), b.btnGap()
	btnY := tr.Y + padY
	btnH := tr.H - 2*padY
	x := tr.X + padX
	// Every toolbar button is a btnH×btnH square so the two ButtonGroups (nav,
	// zoom) divide their bounds into equal members. Members are adjacent within a
	// group; a gap separates the nav group, the zoom group and the address field.
	sq := func() Rect {
		rc := Rect{X: x, Y: btnY, W: btnH, H: btnH}
		x += btnH
		return rc
	}
	back, fwd, reload = sq(), sq(), sq()
	x += gap
	zoomOut, zoomIn = sq(), sq()
	x += gap
	addrW := tr.X + tr.W - padX - x
	if addrW < 0 {
		addrW = 0
	}
	addr = Rect{X: x, Y: btnY, W: addrW, H: btnH}
	return
}

// toolbarBtnAt maps an absolute-coordinate point to the toolbar button kind it
// lands on (for press feedback). It returns false when the point is not over any
// button (e.g. the inter-group gap or the address field).
func (b *Browser) toolbarBtnAt(ax, ay int) (browserBtnKind, bool) {
	back, fwd, reload, zoomOut, zoomIn, _ := b.toolbarLayout()
	for _, m := range []struct {
		r Rect
		k browserBtnKind
	}{
		{back, browserBtnBack}, {fwd, browserBtnFwd}, {reload, browserBtnReload},
		{zoomOut, browserBtnZoomOut}, {zoomIn, browserBtnZoomIn},
	} {
		if m.r.Contains(ax, ay) {
			return m.k, true
		}
	}
	return 0, false
}

// toolbarGroups builds the two segmented ButtonGroups — nav (Back / Forward /
// Reload) and zoom (out / in) — with each member wired to its action, icon,
// label fallback, font and disabled state, and positioned over its cluster.
// Draw and click-routing both go through it so the visual and the hit-testing
// stay in lock-step. addr is the address-field rect.
func (b *Browser) toolbarGroups() (nav, zoom *ButtonGroup, addr Rect) {
	back, _, reload, zoomOut, zoomIn, ad := b.toolbarLayout()
	addr = ad
	mk := func(k browserBtnKind, onClick func()) *Button {
		btn := &Button{Label: b.btnLabel(k), Icon: b.btnIcon(k), OnClick: onClick, PressFeedback: true}
		btn.SetFont(b.EffectiveFont())
		btn.Disabled = !b.btnEnabled(k)
		// Reflect the persistent press state so the pressed button keeps its
		// pressed face across the per-frame rebuild until the release clears it.
		if b.pressActive && b.pressKind == k {
			btn.SetPressed(true)
		}
		return btn
	}
	nav = NewButtonGroup(
		mk(browserBtnBack, func() {
			if b.CanBack() {
				b.Back()
			}
		}),
		mk(browserBtnFwd, func() {
			if b.CanForward() {
				b.Forward()
			}
		}),
		mk(browserBtnReload, func() {
			if b.activeTab() != nil {
				b.Reload()
			}
		}),
	)
	nav.SetBounds(unionRect(back, reload))
	zoom = NewButtonGroup(
		mk(browserBtnZoomOut, func() {
			if b.CanZoomOut() {
				b.ZoomOut()
			}
		}),
		mk(browserBtnZoomIn, func() {
			if b.CanZoomIn() {
				b.ZoomIn()
			}
		}),
	)
	zoom.SetBounds(unionRect(zoomOut, zoomIn))
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
	gap, inTop, inBot := b.tabGap(), b.sc(2), b.sc(4)
	rects := make([]Rect, n)
	for i := 0; i < n; i++ {
		w := pillW - gap
		if w < 1 {
			w = 1
		}
		rects[i] = Rect{X: sr.X + i*pillW, Y: sr.Y + inTop, W: w, H: sr.H - inBot}
	}
	return rects
}

// tabCloseRect is the × close box within a tab pill.
func (b *Browser) tabCloseRect(pill Rect) Rect {
	cw := b.tabCloseW()
	return Rect{X: pill.X + pill.W - cw - b.padX(), Y: pill.Y, W: cw, H: pill.H}
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
	if !b.HideChrome {
		if b.showTabStrip() {
			b.drawTabStrip(p, theme)
		}
		b.drawToolbar(p, theme)
	}
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
		rad := b.sc(6)
		fillRoundRect(p, pill.X, pill.Y, pill.W, pill.H, rad, fill)
		strokeRoundRect(p, pill.X, pill.Y, pill.W, pill.H, rad, ring)
		ty := pill.Y + (pill.H-b.glyphHeight())/2
		title := tabDisplayTitle(t)
		avail := pill.W - 2*b.btnPad() - b.tabCloseW()
		if b.textWidth(title) > avail {
			title = ellipsize(b.EffectiveFont(), title, avail)
		}
		b.drawText(p, pill.X+b.btnPad(), ty, title, theme.OnSurface)
		cxr := b.tabCloseRect(pill)
		b.drawText(p, cxr.X+(cxr.W-b.textWidth("x"))/2, ty, "x", theme.Border)
	}
}

// drawToolbar paints the toolbar background, the five icon buttons and the
// address field.
func (b *Browser) drawToolbar(p painter.Painter, theme *Theme) {
	tr := b.toolbarRect()
	fillRect(p, tr.X, tr.Y, tr.W, tr.H, theme.SurfaceAlt)
	nav, zoom, addr := b.toolbarGroups()
	nav.Draw(p, theme)
	zoom.Draw(p, theme)
	b.drawAddress(p, theme, addr)
}

// unionRect is the smallest rect covering a and b, both on the same toolbar row.
func unionRect(a, b Rect) Rect {
	return Rect{X: a.X, Y: a.Y, W: b.X + b.W - a.X, H: a.H}
}

// drawAddress paints the editable address field: the edit buffer when focused
// (with a caret + Accent focus ring) or the current URL otherwise, head-clipped
// so a long URL keeps its tail (the path) visible.
// addrZones splits the address-field rect into an optional leading status-icon
// square (when LeadingIcon is set), an optional trailing bookmark square (when
// BookmarkIcon is set), and the text zone between them.
func (b *Browser) addrZones(r Rect) (lead, text, star Rect) {
	textX, textR := r.X, r.X+r.W
	if b.LeadingIcon != nil {
		lead = Rect{X: r.X, Y: r.Y, W: r.H, H: r.H}
		textX = r.X + r.H
	}
	if b.BookmarkIcon != nil {
		star = Rect{X: r.X + r.W - r.H, Y: r.Y, W: r.H, H: r.H}
		textR = r.X + r.W - r.H
	}
	if textR < textX {
		textR = textX
	}
	text = Rect{X: textX, Y: r.Y, W: textR - textX, H: r.H}
	return
}

func (b *Browser) drawAddress(p painter.Painter, theme *Theme, r Rect) {
	rad := b.sc(4)
	fillRoundRect(p, r.X, r.Y, r.W, r.H, rad, theme.Surface)
	ring := theme.Border
	if b.addrFocused {
		ring = theme.Accent
	}
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, rad, ring)
	lead, tz, star := b.addrZones(r)
	if b.LeadingIcon != nil {
		b.LeadingIcon(p, lead, theme.OnSurface)
	}
	if b.BookmarkIcon != nil {
		b.BookmarkIcon(p, star, theme.OnSurface, b.Bookmarked)
	}
	text := b.CurrentURL()
	if b.addrFocused {
		text = b.addrBuf
	}
	innerX := tz.X + b.btnPad()
	avail := tz.W - 2*b.btnPad()
	shown := text
	if b.textWidth(shown) > avail {
		shown = clipHeadToWidth(b.EffectiveFont(), shown, avail)
	}
	ty := r.Y + (r.H-b.glyphHeight())/2
	// After a copy, paint a select-all highlight behind the text so the user sees
	// exactly what went to the clipboard.
	if b.addrCopied && shown != "" {
		hl := blendRGBA(theme.Accent, theme.Surface, 0.62)
		fillRect(p, innerX, ty, b.textWidth(shown), b.glyphHeight(), hl)
	}
	b.drawText(p, innerX, ty, shown, theme.OnSurface)
	if b.addrFocused {
		caretW := b.glyphHeight() / 12
		if caretW < 1 {
			caretW = 1
		}
		fillRect(p, innerX+b.textWidth(shown), ty, caretW, b.glyphHeight(), theme.OnSurface)
	}
}

// AddressFocused reports whether the address field currently holds keyboard
// focus (a prior click landed in it), so a host can route a copy chord to
// [Browser.CopyAddress] instead of its own copy action.
func (b *Browser) AddressFocused() bool { return b.addrFocused }

// AddressText returns the text the address field shows: the editable buffer
// while focused, else the current page URL.
func (b *Browser) AddressText() string {
	if b.addrFocused {
		return b.addrBuf
	}
	return b.CurrentURL()
}

// CopyAddress copies the address field's text to the toolkit-wide clipboard and
// flags a select-all highlight (visual feedback of what was copied), reporting
// the text and whether anything was copied. It is a no-op returning ("", false)
// when the field is not focused or is empty — so a host can try it first and
// fall back to another copy action. Mirrors [Entry]'s "no selection → copy the
// whole value" model.
func (b *Browser) CopyAddress() (string, bool) {
	if !b.addrFocused {
		return "", false
	}
	txt := b.AddressText()
	if txt == "" {
		return "", false
	}
	SetClipboardText(txt)
	b.addrCopied = true
	b.changed()
	return txt, true
}

// toggleBookmark flips the bookmark state + fires OnBookmarkToggle. Called when
// the bookmark slot in the address field is clicked.
func (b *Browser) toggleBookmark() {
	b.Bookmarked = !b.Bookmarked
	if b.OnBookmarkToggle != nil {
		b.OnBookmarkToggle(b.Bookmarked)
	}
	b.changed()
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
	pb.SetBounds(Rect{X: cr.X, Y: cr.Y, W: cr.W, H: b.progressH()})
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
	case EventMouseUp:
		// Release clears the toolbar button's pressed face armed by EventClick.
		if b.pressActive {
			b.pressActive = false
			b.changed()
		}
	case EventChar:
		if !b.addrFocused || ev.Code == "" {
			return
		}
		b.addrCopied = false // an edit dismisses the copied-highlight
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
			b.addrCopied = false // an edit dismisses the copied-highlight
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
	b.addrCopied = false
	if raw == "" {
		b.changed()
		return
	}
	b.Navigate(normalizeURL(raw))
}

// handleClick routes an absolute-coordinate click to the tab strip, a toolbar
// button, the address field or a page link.
func (b *Browser) handleClick(ax, ay int) {
	b.addrCopied = false // any click dismisses the copied-highlight
	// With the chrome hidden there are no tab-strip / toolbar / address hit
	// targets: a click falls straight through to the content area.
	if !b.HideChrome {
		if b.showTabStrip() && b.tabStripRect().Contains(ax, ay) {
			for i, pill := range b.tabRects() {
				if pill.Contains(ax, ay) {
					if b.tabCloseRect(pill).Contains(ax, ay) {
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
		nav, zoom, addr := b.toolbarGroups()
		for _, g := range []*ButtonGroup{nav, zoom} {
			if gb := g.Bounds(); gb.Contains(ax, ay) {
				b.addrFocused = false
				// Arm the pressed face on the hit button (when enabled) so the
				// ensuing redraw shows the click; EventMouseUp clears it.
				if k, ok := b.toolbarBtnAt(ax, ay); ok && b.btnEnabled(k) {
					b.pressActive, b.pressKind = true, k
				}
				g.OnEvent(Event{Kind: EventClick, X: ax - gb.X, Y: ay - gb.Y})
				return
			}
		}
		if addr.Contains(ax, ay) {
			// A click on the trailing bookmark slot toggles it rather than focusing.
			if _, _, star := b.addrZones(addr); b.BookmarkIcon != nil && star.Contains(ax, ay) {
				b.addrFocused = false
				b.toggleBookmark()
				return
			}
			b.addrFocused = true
			b.addrBuf = b.CurrentURL()
			b.changed()
			return
		}
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
