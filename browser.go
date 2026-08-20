// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"math"
	"strings"

	"github.com/go-widgets/mvvm"
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

	// HideScrollbar suppresses the Browser's OWN content scrollbars (both axes),
	// for a host that overlays its own — e.g. a reader that draws one shared
	// Scrollbar style down every panel and wants the preview's web view to match
	// the feed and sidebar exactly rather than show the embedded house style.
	// Only the paint is suppressed; wheel scrolling still works. The host reads
	// ScrollExtent to size and place its replacement bar, exactly as it does with
	// TreeView.HideScrollbar + TreeView.ScrollExtent.
	HideScrollbar bool

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
	// FitIcon is the host-supplied painter for the best-fit zoom button (the
	// third member of the zoom group, next to zoom-out / zoom-in). Same nil-safe
	// seam as the other toolbar icons: a nil hook falls back to the text label.
	FitIcon func(p painter.Painter, r Rect, ink RGBA)

	// LeadingIcon, when set, paints a status glyph at the LEFT of the address
	// field — e.g. an SSL padlock whose look the host varies by certificate state
	// (secure / insecure / none). Same painter seam as the toolbar icons; the
	// address text indents to its right. Nil → no leading slot (text starts at
	// the normal inset).
	LeadingIcon func(p painter.Painter, r Rect, ink RGBA)

	// BookmarkIcon, when set, paints a toggle glyph at the RIGHT of the address
	// field — e.g. a star, filled when on. It takes the current bookmark state so
	// the host can draw the on/off variant. Clicking the slot flips the shared
	// [Browser.Bookmarked] Observable. Nil → no bookmark slot.
	BookmarkIcon func(p painter.Painter, r Rect, ink RGBA, on bool)

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

	// addr is the composed editable address field ([AddressBar]). Its appearance
	// config + bounds are set each frame by layoutAddr, and the Browser publishes
	// the current URL into its shared URL Observable; all of the field's mutable
	// STATE (focus, edit buffer, bookmark, copy flag) lives in the field's own
	// Observables, which the Browser subscribes to for repaint — never copied.
	addr AddressBar

	// pressActive + pressKind give a toolbar button its click feedback: a click
	// (EventClick) arms pressActive on the hit button's kind so the next Draw
	// paints that button's pressed face, and the release (EventMouseUp) clears it.
	// The button instances are rebuilt each frame, so the pressed state lives here
	// on the persistent Browser rather than on the ephemeral Button.
	pressActive bool
	pressKind   browserBtnKind

	zoom float64 // page display scale; clamped to [BrowserMinZoom, BrowserMaxZoom]

	// sbV and sbH carry an in-progress drag of the vertical / horizontal content
	// scrollbar thumb. They share the same press/drag/release policy (scrollDrag)
	// as ScrollView and Table, so a thumb drag on the page behaves identically to
	// every other scrollable widget in the toolkit.
	sbV, sbH scrollDrag
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

	links   []BrowserLink
	scroll  int // vertical scroll offset, in content pixels
	scrollX int // horizontal scroll offset, in zoomed-display pixels

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
	// browserFitLabel is the best-fit zoom button's text fallback. Plain ASCII so
	// it renders on both the pixel and terminal back-ends.
	browserFitLabel = "Fit"
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
	browserBtnFit
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
	case browserBtnZoomIn:
		return browserZoomInLabel
	default: // browserBtnFit
		return browserFitLabel
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
	case browserBtnZoomIn:
		return b.ZoomInIcon
	default: // browserBtnFit
		return b.FitIcon
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
	case browserBtnZoomIn:
		return b.CanZoomIn()
	default: // browserBtnFit
		return b.CanFit()
	}
}

// NewBrowser builds an empty Browser in the default MultiTab mode at 1.0 zoom.
func NewBrowser() *Browser {
	b := &Browser{zoom: 1.0}
	// The address field is MVVM: Enter runs a Command that normalises + navigates
	// to the field's edit buffer, and the Browser repaints by SUBSCRIBING to the
	// field's state Observables — no per-frame state copy, no read-back. The URL
	// Observable is published by the Browser (the source of truth) in layoutAddr.
	b.addr.Commit = mvvm.NewCommand(func() { b.Navigate(normalizeURL(b.addr.Editing().Get())) }, nil)
	for _, o := range []mvvm.Changeable{b.addr.Editing(), b.addr.Focused(), b.addr.Copied(), b.addr.Bookmarked()} {
		o.SubscribeChanged(b.changed)
	}
	return b
}

// scale is the effective chrome scale factor: Scale when positive, else the
// toolkit's own [MetricScale]. See Menu.scale on why it defers rather than
// multiplies.
func (b *Browser) scale() float64 {
	if b.Scale <= 0 {
		return MetricScale()
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

// Deliver hands the widget a finished render for target: it delivers a final
// stage (loading clears). See DeliverStage. The scroll position was reset when
// the navigation to target began (startLoad), so a delivered render is shown
// from wherever the user has scrolled to — deliveries do not yank the page.
func (b *Browser) Deliver(target string, pixels []byte, imgW, imgH, width int, links []BrowserLink, title string) {
	b.DeliverStage(target, pixels, imgW, imgH, width, links, title, true)
}

// DeliverStage delivers one render for target, distinguishing a final render
// from an intermediate progressive frame. When target matches the active tab's
// current URL the render (pixels + dimensions + width), links and title are
// stored; a stale or non-active delivery is ignored.
//
// With final=true the load is complete and loading clears. With final=false it
// is one staged frame of a still-running progressive render (a fast first paint,
// then refinements): the content updates but loading stays on, so the progress
// indicator keeps animating and the page does not read as "done" until the
// final frame lands. Neither form resets the scroll position — that happens once
// when the navigation begins (startLoad) — so a staged render refines in place
// instead of snapping to the top on every frame.
func (b *Browser) DeliverStage(target string, pixels []byte, imgW, imgH, width int, links []BrowserLink, title string, final bool) {
	t := b.activeTab()
	if t == nil || t.history[t.cursor] != target {
		return
	}
	t.pixels = pixels
	t.imgW, t.imgH = imgW, imgH
	t.renderW = width
	t.links = links
	t.title = title
	if final {
		t.loading = false
	}
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

// CanFit reports whether a best-fit zoom is possible: there is an active tab
// with a delivered render and a non-empty content rect to fit it into.
func (b *Browser) CanFit() bool {
	t := b.activeTab()
	if t == nil || t.imgW <= 0 || t.imgH <= 0 {
		return false
	}
	cr := b.contentRect()
	return cr.W > 0 && cr.H > 0
}

// FitZoom sets the zoom so the WHOLE current page/image fits within the content
// rect on both axes. The natural display size at zoom 1 is dispW0 = cr.W (pages
// render fit-to-width) and dispH0 = imgH*cr.W/imgW; the fit factor is
// min(1, cr.W/dispW0, cr.H/dispH0) — capped at 1 so a page already smaller than
// the pane is not blown up — then clamped to [BrowserMinZoom, BrowserMaxZoom]
// by SetZoom (which also re-clamps scroll). It is a no-op when there is no
// render or the content rect is empty.
func (b *Browser) FitZoom() {
	t := b.activeTab()
	if t == nil || t.imgW <= 0 || t.imgH <= 0 {
		return
	}
	cr := b.contentRect()
	if cr.W <= 0 || cr.H <= 0 {
		return
	}
	dispW0 := float64(cr.W)
	dispH0 := float64(t.imgH) * float64(cr.W) / float64(t.imgW)
	// min(1, width-factor, height-factor). The width factor is cr.W/dispW0, which
	// is always 1 (pages render fit-to-width, so dispW0 == cr.W); the height factor
	// cr.H/dispH0 is what actually shrinks a tall page. The 1.0 cap keeps a page
	// already smaller than the pane from being blown up.
	fit := math.Min(1.0, math.Min(float64(cr.W)/dispW0, float64(cr.H)/dispH0))
	b.SetZoom(fit)
}

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
		// A zoom change resizes BOTH extents (a smaller zoom shrinks the page, a
		// larger one grows its width past the column), so re-clamp both offsets.
		b.clampScroll(t)
	}
	b.changed()
}

// startLoad marks t loading for target, resets its progress, records the pending
// render width, notifies OnChange and triggers the host render via OnNavigate.
func (b *Browser) startLoad(t *browserTab, target string) {
	t.loading = true
	t.hasProgress = false
	t.progress = 0
	// A new navigation starts at the top; deliveries afterward preserve the
	// scroll, so a progressive render refines in place.
	t.scroll = 0
	t.scrollX = 0
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

// toolbarLayout returns the Back, Forward, Reload, zoom-out, zoom-in and best-fit
// button rects and the address field rect for the current toolbar. A button whose
// icon hook is set is laid out as a square (side = the toolbar's inner height) so
// the icons form an even row of real buttons; a button with no icon is sized to
// its text label so the fallback stays legible. The address field fills the
// remainder to the right pad.
func (b *Browser) toolbarLayout() (back, fwd, reload, zoomOut, zoomIn, fit, addr Rect) {
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
	zoomOut, zoomIn, fit = sq(), sq(), sq()
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
	back, fwd, reload, zoomOut, zoomIn, fit, _ := b.toolbarLayout()
	for _, m := range []struct {
		r Rect
		k browserBtnKind
	}{
		{back, browserBtnBack}, {fwd, browserBtnFwd}, {reload, browserBtnReload},
		{zoomOut, browserBtnZoomOut}, {zoomIn, browserBtnZoomIn}, {fit, browserBtnFit},
	} {
		if m.r.Contains(ax, ay) {
			return m.k, true
		}
	}
	return 0, false
}

// toolbarGroups builds the two segmented ButtonGroups — nav (Back / Forward /
// Reload) and zoom (out / in / best-fit) — with each member wired to its action,
// icon, label fallback, font and disabled state, and positioned over its cluster.
// Draw and click-routing both go through it so the visual and the hit-testing
// stay in lock-step. addr is the address-field rect.
func (b *Browser) toolbarGroups() (nav, zoom *ButtonGroup, addr Rect) {
	back, _, reload, zoomOut, _, fit, ad := b.toolbarLayout()
	addr = ad
	mk := func(k browserBtnKind, onClick func()) *Button {
		btn := &Button{Icon: b.btnIcon(k), OnClick: onClick, PressFeedback: true}
		btn.Label().Set(b.btnLabel(k))
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
		mk(browserBtnFit, func() {
			if b.CanFit() {
				b.FitZoom()
			}
		}),
	)
	zoom.SetBounds(unionRect(zoomOut, fit))
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

// maxScrollX is the largest horizontal scroll offset for t within content cr, in
// zoomed-display pixels (0 when the page is not wider than cr). At the 1.0 zoom
// the page is fit-to-width (dispW == cr.W) so this is 0; only zoom > 1 widens the
// page past the content column and makes horizontal scrolling meaningful.
func (b *Browser) maxScrollX(t *browserTab, cr Rect) int {
	dispW, _ := b.pageDisplaySize(t, cr)
	m := dispW - cr.W
	if m < 0 {
		m = 0
	}
	return m
}

// clampScroll pins t's vertical and horizontal offsets back into their legal
// ranges for the current content rect + zoom. Every mutation of scroll / scrollX
// (wheel, thumb drag, track page, zoom change) routes through it so an offset can
// never fall off either extent.
func (b *Browser) clampScroll(t *browserTab) {
	cr := b.contentRect()
	if t.scroll < 0 {
		t.scroll = 0
	}
	if m := b.maxScroll(t, cr); t.scroll > m {
		t.scroll = m
	}
	if t.scrollX < 0 {
		t.scrollX = 0
	}
	if m := b.maxScrollX(t, cr); t.scrollX > m {
		t.scrollX = m
	}
}

// ScrollExtent reports the active page's VERTICAL scroll position in content
// pixels — the offset, the viewport height and the total (zoomed) page height —
// and whether the page overflows. A host that sets HideScrollbar and paints its
// own bar reads this to size and place a matching one, exactly as
// TreeView.ScrollExtent serves the same purpose for a windowed tree. It reports
// not-shown when there is no active tab, no render yet, or the page fits.
func (b *Browser) ScrollExtent() (offset, viewport, total int, shown bool) {
	t := b.activeTab()
	if t == nil {
		return 0, 0, 0, false
	}
	cr := b.contentRect()
	_, dispH := b.pageDisplaySize(t, cr)
	if cr.H <= 0 || dispH <= cr.H {
		return 0, cr.H, dispH, false
	}
	return t.scroll, cr.H, dispH, true
}

// vscrollGeom returns the vertical content scrollbar's geometry (in coordinates
// RELATIVE to the content rect's top-left) and whether it is live (the page
// overflows vertically). It is the single definition shared by drawScrollbars
// (paint) and OnEvent (hit-test + drag), so the drawn thumb and the drag target
// can never drift apart. The track sits on the content's right edge and is
// shortened by scrollbarWidth at the bottom when the horizontal bar also shows,
// leaving the corner clear. The thumb is sized to the visible fraction cr.H/dispH
// (floored at scrollbarWidth) and positioned from scroll/maxScroll.
func (b *Browser) vscrollGeom(t *browserTab, cr Rect) (sbGeom, bool) {
	maxOff := b.maxScroll(t, cr)
	if maxOff <= 0 || cr.W <= 0 || cr.H <= 0 {
		return sbGeom{}, false
	}
	_, dispH := b.pageDisplaySize(t, cr)
	trackLen := cr.H
	if b.maxScrollX(t, cr) > 0 {
		trackLen -= scrollbarWidth // reserve the corner for the horizontal bar
	}
	if trackLen <= 0 {
		return sbGeom{}, false
	}
	thumb := trackLen * cr.H / dispH
	if thumb < scrollbarWidth {
		thumb = scrollbarWidth
	}
	if thumb > trackLen {
		thumb = trackLen
	}
	travelNum := trackLen - thumb
	return sbGeom{
		cross0:     cr.W - scrollbarWidth,
		trackStart: 0,
		trackLen:   trackLen,
		thumbStart: travelNum * t.scroll / maxOff,
		thumbLen:   thumb,
		travelNum:  travelNum,
		travelDen:  maxOff,
		maxScroll:  maxOff,
	}, true
}

// hscrollGeom returns the horizontal content scrollbar's geometry (in coordinates
// RELATIVE to the content rect's top-left) and whether it is live (the page
// overflows horizontally, i.e. under zoom > 1). The track sits on the content's
// bottom edge and is shortened by scrollbarWidth at the right when the vertical
// bar also shows, leaving the corner clear. The thumb is sized to the visible
// fraction cr.W/dispW (floored at scrollbarWidth) and positioned from
// scrollX/maxScrollX.
func (b *Browser) hscrollGeom(t *browserTab, cr Rect) (sbGeom, bool) {
	maxOff := b.maxScrollX(t, cr)
	if maxOff <= 0 || cr.W <= 0 || cr.H <= 0 {
		return sbGeom{}, false
	}
	dispW, _ := b.pageDisplaySize(t, cr)
	trackLen := cr.W
	if b.maxScroll(t, cr) > 0 {
		trackLen -= scrollbarWidth // reserve the corner for the vertical bar
	}
	if trackLen <= 0 {
		return sbGeom{}, false
	}
	thumb := trackLen * cr.W / dispW
	if thumb < scrollbarWidth {
		thumb = scrollbarWidth
	}
	if thumb > trackLen {
		thumb = trackLen
	}
	travelNum := trackLen - thumb
	return sbGeom{
		horizontal: true,
		cross0:     cr.H - scrollbarWidth,
		trackStart: 0,
		trackLen:   trackLen,
		thumbStart: travelNum * t.scrollX / maxOff,
		thumbLen:   thumb,
		travelNum:  travelNum,
		travelDen:  maxOff,
		maxScroll:  maxOff,
	}, true
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
		// Each tab is a composed rounded Backdrop (fill + border), byte-identical to
		// the former hand-drawn fillRoundRect/strokeRoundRect pair.
		pillBg := Backdrop{Fill: fill, Radius: b.sc(6), Stroke: ring, StrokeWidth: strokeWidth()}
		pillBg.SetBounds(pill)
		pillBg.Draw(p, theme)
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
	b.layoutAddr(addr)
	b.addr.Draw(p, theme)
}

// layoutAddr positions + configures the composed address field before it is drawn
// or handed an event: its bounds, the appearance config (icon hooks, corner
// radius, text inset, font) and the current URL published into its shared
// Observable (the Browser is the URL source of truth). This is layout + config
// only — the field's mutable STATE (focus, edit buffer, bookmark, copy flag)
// lives in its own Observables and is never copied here or read back.
func (b *Browser) layoutAddr(r Rect) {
	b.addr.SetBounds(r)
	b.addr.LeadingIcon = b.LeadingIcon
	b.addr.BookmarkIcon = b.BookmarkIcon
	b.addr.Radius = b.sc(4)
	b.addr.TextPad = b.btnPad()
	b.addr.SetFont(b.EffectiveFont())
	b.addr.URL().Set(b.CurrentURL())
}

// Bookmarked is the shared bookmark-toggle Observable of the address field: the
// host binds it to its bookmark store (Set it, or Subscribe to it), and clicking
// the bookmark slot flips it. It replaces the former Bookmarked field +
// OnBookmarkToggle callback — bookmark state now flows only through MVVM.
func (b *Browser) Bookmarked() *mvvm.Observable[bool] { return b.addr.Bookmarked() }

// unionRect is the smallest rect covering a and b, both on the same toolbar row.
func unionRect(a, b Rect) Rect {
	return Rect{X: a.X, Y: a.Y, W: b.X + b.W - a.X, H: a.H}
}

// AddressFocused reports whether the address field currently holds keyboard
// focus (a prior click landed in it), so a host can route a copy chord to
// [Browser.CopyAddress] instead of its own copy action.
func (b *Browser) AddressFocused() bool { return b.addr.Focused().Get() }

// AddressText returns the text the address field shows: the editable buffer
// while focused, else the current page URL.
func (b *Browser) AddressText() string {
	if b.addr.Focused().Get() {
		return b.addr.Value()
	}
	return b.CurrentURL()
}

// CopyAddress copies the address field's text to the toolkit-wide clipboard and
// flags a select-all highlight (visual feedback of what was copied), reporting
// the text and whether anything was copied. It is a no-op returning ("", false)
// when the field is not focused or is empty — so a host can try it first and
// fall back to another copy action. Mirrors [Entry]'s "no selection → copy the
// whole value" model.
func (b *Browser) CopyAddress() (string, bool) { return b.addr.CopySelectAll() }

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
	b.drawScrollbars(p, theme, cr, t)
	if t.loading {
		b.drawProgress(p, theme, cr, t)
	}
}

// drawPage blits t's render scaled to the zoomed display size, top-aligned and
// offset by the tab's scroll (vertical) and scrollX (horizontal): it paints the
// display window [scrollX, scrollX+cr.W) × [scroll, scroll+cr.H) of the zoomed
// page. Rows and columns outside the content rect are skipped, so the blit always
// stays within cr while any overflowing side is reachable by scrolling.
func (b *Browser) drawPage(p painter.Painter, cr Rect, t *browserTab) {
	if t.imgW <= 0 || t.imgH <= 0 || len(t.pixels) < t.imgW*t.imgH*4 {
		return
	}
	dispW, dispH := b.pageDisplaySize(t, cr)
	if dispH < 1 {
		return
	}
	// The whole zoomed page is placed at the scrolled origin and the content
	// rect crops it. Walking the destination and skipping what falls outside --
	// which is what this did -- computes the same pixels, but a pixel at a time
	// and with the crop decided per pixel; the painter decides it once and
	// copies rows.
	blitImage(p,
		Rect{X: cr.X - t.scrollX, Y: cr.Y - t.scroll, W: dispW, H: dispH},
		cr, t.pixels, t.imgW, t.imgH)
}

// drawScrollbars paints the vertical (right-edge) and horizontal (bottom-edge)
// content scrollbars whenever the page overflows the corresponding axis, in the
// house style shared with ScrollView and Table: a SurfaceAlt track with an Accent
// thumb sized to the visible fraction. Both read the same vscrollGeom/hscrollGeom
// the input path hit-tests against, so the drawn thumb and the drag target are one
// geometry. When both show, each track is shortened by scrollbarWidth so the
// bottom-right corner stays clear.
func (b *Browser) drawScrollbars(p painter.Painter, theme *Theme, cr Rect, t *browserTab) {
	if b.HideScrollbar {
		return // the host overlays its own bar (see HideScrollbar / ScrollExtent)
	}
	if g, ok := b.vscrollGeom(t, cr); ok {
		paintScrollTrack(p, theme, cr.X+g.cross0, cr.Y+g.trackStart, scrollbarWidth, g.trackLen)
		paintScrollThumb(p, theme, cr.X+g.cross0, cr.Y+g.thumbStart, scrollbarWidth, g.thumbLen)
	}
	if g, ok := b.hscrollGeom(t, cr); ok {
		paintScrollTrack(p, theme, cr.X+g.trackStart, cr.Y+g.cross0, g.trackLen, scrollbarWidth)
		paintScrollThumb(p, theme, cr.X+g.thumbStart, cr.Y+g.cross0, g.thumbLen, scrollbarWidth)
	}
}

// drawProgress paints the loading bar across the content top: determinate when
// SetProgress was called this load, else indeterminate driven by Phase.
func (b *Browser) drawProgress(p painter.Painter, theme *Theme, cr Rect, t *browserTab) {
	pb := &ProgressBar{}
	pb.SetBounds(Rect{X: cr.X, Y: cr.Y, W: cr.W, H: b.progressH()})
	if t.hasProgress {
		pb.Fraction().Set(t.progress)
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
		// Release ends any in-progress scrollbar thumb drag ...
		b.sbV.release()
		b.sbH.release()
		// ... and clears the toolbar button's pressed face armed by EventClick.
		if b.pressActive {
			b.pressActive = false
			b.changed()
		}
	case EventMouseDrag:
		// A drag with a scrollbar thumb grabbed maps the pointer to a clamped
		// offset; with no grab active both drags are no-ops (the content itself is
		// not draggable).
		t := b.activeTab()
		if t == nil {
			return
		}
		cr := b.contentRect()
		lev := Event{Kind: EventMouseDrag, X: ax - cr.X, Y: ay - cr.Y}
		gv, okv := b.vscrollGeom(t, cr)
		b.sbV.drag(gv, okv, lev, func(target int) { t.scroll = target; b.clampScroll(t); b.changed() })
		gh, okh := b.hscrollGeom(t, cr)
		b.sbH.drag(gh, okh, lev, func(target int) { t.scrollX = target; b.clampScroll(t); b.changed() })
	case EventChar, EventKeyDown:
		// The composed address field owns the edit buffer + focus; it ignores the
		// event when unfocused, appends a rune, deletes on Backspace and commits
		// (OnCommit → normalise + navigate) on Enter, firing changed() through its
		// OnChange hook.
		b.addr.OnEvent(ev)
	case EventScroll:
		t := b.activeTab()
		if t == nil {
			return
		}
		// Shift-wheel scrolls horizontally (the Event model carries a single Delta
		// plus a Shift modifier rather than a separate horizontal axis); a plain
		// wheel scrolls vertically as before. Both clamp to their extents.
		if ev.Shift {
			t.scrollX += ev.Delta * BrowserScrollStep
		} else {
			t.scroll += ev.Delta * BrowserScrollStep
		}
		b.clampScroll(t)
		b.changed()
	}
}

// handleClick routes an absolute-coordinate click to the tab strip, a toolbar
// button, the address field or a page link.
func (b *Browser) handleClick(ax, ay int) {
	b.addr.dismissCopied() // any click dismisses the copied-highlight
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
						b.addr.Blur()
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
				b.addr.Blur()
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
			// The composed field decides: a click on the bookmark slot flips the
			// shared Bookmarked Observable, any other click focuses + seeds the edit
			// buffer from the current URL. No state is read back — the toggle and
			// focus live in the field's own Observables that the Browser subscribes to.
			b.layoutAddr(addr)
			b.addr.OnEvent(Event{Kind: EventClick, X: ax - addr.X, Y: ay - addr.Y})
			return
		}
	}
	cr := b.contentRect()
	if cr.Contains(ax, ay) {
		b.addr.Blur()
		t := b.activeTab()
		if t == nil {
			return
		}
		// A press on either scrollbar grabs its thumb (or pages the track toward the
		// click) and consumes the event, so it never falls through to a page link.
		lev := Event{Kind: EventClick, X: ax - cr.X, Y: ay - cr.Y}
		if gv, ok := b.vscrollGeom(t, cr); b.sbV.press(gv, ok, lev, cr.H, func(d int) { t.scroll += d; b.clampScroll(t); b.changed() }) {
			return
		}
		if gh, ok := b.hscrollGeom(t, cr); b.sbH.press(gh, ok, lev, cr.W, func(d int) { t.scrollX += d; b.clampScroll(t); b.changed() }) {
			return
		}
		if href, ok := b.linkAt(t, cr, ax, ay); ok {
			b.Navigate(href)
		}
		return
	}
	b.addr.Blur()
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
	rx := (ax - cr.X + t.scrollX) * t.imgW / dispW
	ry := (ay - cr.Y + t.scroll) * t.imgH / dispH
	pt := image.Pt(rx, ry)
	for _, ln := range t.links {
		if pt.In(ln.Rect) {
			return ln.Href, true
		}
	}
	return "", false
}
