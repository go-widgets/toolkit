// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// NotebookTab is one entry in a Notebook. Label is the human title
// painted on the tab; Page is the widget shown when the tab is
// active.
type NotebookTab struct {
	Label string
	Page  Widget
}

// TabSide selects which edge of the Notebook hosts the tab strip.
type TabSide int

const (
	// TabTop places the strip along the top edge (the default).
	TabTop TabSide = iota
	// TabBottom places the strip along the bottom edge.
	TabBottom
	// TabLeft places the strip down the left edge (tabs stacked vertically).
	TabLeft
	// TabRight places the strip down the right edge (tabs stacked vertically).
	TabRight
)

// Notebook is a tabbed container. A n.stripH()-thick strip on the side
// chosen by TabSide (Top by default) hosts the tabs; the rest is the active
// page's body. For Top/Bottom the tabs run horizontally (each scaled(NotebookTabWidth)
// wide, shrunk to fit); for Left/Right they stack vertically (each
// NotebookTabStripH tall) and the strip SCROLLS: the mouse wheel over a
// vertical strip shifts the stacked tabs (clamped at both ends), and arrow-key
// tab switching scrolls the strip to keep the active tab in view, so a strip
// with more tabs than fit stays fully reachable. Clicking a tab Sets the Active
// Observable (notifying its subscribers).
//
// The reactive active-tab index is MVVM-only: it lives in an unexported
// Observable exposed via [Notebook.Active]. Tabs and TabSide are set-once
// layout config and stay plain fields.
type Notebook struct {
	Base
	focusState
	Tabs    []NotebookTab
	TabSide TabSide

	active *mvvm.Observable[int]

	// tabScroll is the index of the first tab shown at the top of a vertical
	// (Left/Right) strip -- the scroll offset that makes an over-long vertical
	// tab strip reachable. Ignored for horizontal (Top/Bottom) strips, which
	// shrink-to-fit instead. Reads clamp on the fly (clampedTabScroll), so a
	// stale value is harmless; at tabScroll == 0 the strip renders + hit-tests
	// byte-identically to before scrolling existed.
	tabScroll int
}

// Active is the active tab index as a shared [mvvm.Observable]: a host binds it
// (Set / Subscribe / two-way) — there is no settable Active field. A tab click
// or a keyboard tab move Sets it; subscribers are notified. The accessor
// lazy-inits to 0 so a bare &Notebook{} is usable without a constructor.
func (n *Notebook) Active() *mvvm.Observable[int] {
	if n.active == nil {
		n.active = mvvm.NewObservable(0)
	}
	return n.active
}

// Geometry constants for the tab strip: the strip's thickness (its height for a
// Top/Bottom strip, its width for a Left/Right strip) and each tab's extent
// along the strip.
const (
	NotebookTabStripH = 24
	NotebookTabWidth  = 80
)

// stripH is the effective tab-strip thickness -- the height of a Top/Bottom
// strip and the per-tab stacked height of a Left/Right strip -- used for both
// layout and hit-testing: the scaled [NotebookTabStripH] clamped UP to the
// density minimum hit target via [TouchTarget]. Under [DensityCompact] the
// clamp is a pass-through (byte-identical to scaled(NotebookTabStripH)); under
// [DensityTouch] a tab grows to the finger floor (>=44 device px) along its
// short axis so it is a large-enough tap target. The strip's other extent
// ([NotebookTabWidth], 80px) already clears the floor, so it keeps its plain
// scaled width.
func (n *Notebook) stripH() int { return TouchTarget(scaled(NotebookTabStripH)) }

// stripRect is the tab-strip band on the chosen side.
func (n *Notebook) stripRect() Rect {
	r := n.Bounds()
	switch n.TabSide {
	case TabBottom:
		return Rect{X: r.X, Y: r.Y + r.H - n.stripH(), W: r.W, H: n.stripH()}
	case TabLeft:
		return Rect{X: r.X, Y: r.Y, W: scaled(NotebookTabWidth), H: r.H}
	case TabRight:
		return Rect{X: r.X + r.W - scaled(NotebookTabWidth), Y: r.Y, W: scaled(NotebookTabWidth), H: r.H}
	default: // TabTop
		return Rect{X: r.X, Y: r.Y, W: r.W, H: n.stripH()}
	}
}

// tabW is the per-tab width for the horizontal (Top/Bottom) strips: the
// nominal scaled(NotebookTabWidth), shrunk so that ALL tabs fit within the notebook's
// width when there are too many to fit at the nominal width. Without this the
// strip laid tabs at a fixed 80px each and overflowed the widget's box once
// nTabs*80 exceeded Bounds().W (e.g. 4 tabs in a ~296px notebook).
func (n *Notebook) tabW() int {
	nt := len(n.Tabs)
	if nt == 0 {
		return scaled(NotebookTabWidth)
	}
	tw := scaled(NotebookTabWidth)
	if fit := n.Bounds().W / nt; fit < tw {
		tw = fit
	}
	return tw
}

// tabRect is the i-th tab's rect (in surface coordinates).
func (n *Notebook) tabRect(i int) Rect {
	r := n.Bounds()
	switch n.TabSide {
	case TabBottom:
		tw := n.tabW()
		return Rect{X: r.X + i*tw, Y: r.Y + r.H - n.stripH(), W: tw, H: n.stripH()}
	case TabLeft:
		ty := r.Y + (i-n.clampedTabScroll())*n.stripH()
		return Rect{X: r.X, Y: ty, W: scaled(NotebookTabWidth), H: n.stripH()}
	case TabRight:
		ty := r.Y + (i-n.clampedTabScroll())*n.stripH()
		return Rect{X: r.X + r.W - scaled(NotebookTabWidth), Y: ty, W: scaled(NotebookTabWidth), H: n.stripH()}
	default: // TabTop
		tw := n.tabW()
		return Rect{X: r.X + i*tw, Y: r.Y, W: tw, H: n.stripH()}
	}
}

// verticalStrip reports whether the tab strip stacks its tabs vertically
// (Left/Right) rather than laying them out horizontally (Top/Bottom).
func (n *Notebook) verticalStrip() bool {
	return n.TabSide == TabLeft || n.TabSide == TabRight
}

// visibleTabs is how many vertically-stacked tabs the strip can show at once,
// floored at 0. Meaningful only for a Left/Right strip.
func (n *Notebook) visibleTabs() int {
	h := n.Bounds().H
	if h < 0 {
		h = 0
	}
	return h / n.stripH()
}

// maxTabScroll is the highest tabScroll that still fills a vertical strip:
// len(Tabs) - visibleTabs(), floored at 0. Always 0 for a horizontal strip,
// which does not scroll.
func (n *Notebook) maxTabScroll() int {
	if !n.verticalStrip() {
		return 0
	}
	m := len(n.Tabs) - n.visibleTabs()
	if m < 0 {
		m = 0
	}
	return m
}

// clampedTabScroll returns tabScroll clamped to [0, maxTabScroll()] without
// mutating the field. Always 0 for a horizontal strip, so a Top/Bottom
// notebook lays its tabs out exactly as before.
func (n *Notebook) clampedTabScroll() int {
	if !n.verticalStrip() {
		return 0
	}
	v := n.tabScroll
	if v < 0 {
		v = 0
	}
	if m := n.maxTabScroll(); v > m {
		v = m
	}
	return v
}

// ScrollTabsBy shifts a vertical strip's scroll offset by delta tabs (negative
// scrolls up), clamped to [0, maxTabScroll()] and written back. A no-op for a
// horizontal strip.
func (n *Notebook) ScrollTabsBy(delta int) {
	if !n.verticalStrip() {
		return
	}
	n.tabScroll += delta
	n.tabScroll = n.clampedTabScroll()
}

// scrollActiveIntoView nudges a vertical strip's scroll offset so the active
// tab is within the visible window -- called after a keyboard tab switch so
// arrowing onto an off-screen tab scrolls it into view. A no-op for a
// horizontal strip or an empty window.
func (n *Notebook) scrollActiveIntoView() {
	if !n.verticalStrip() {
		return
	}
	vis := n.visibleTabs()
	if vis <= 0 {
		return
	}
	active := n.Active().Get()
	if active < n.tabScroll {
		n.tabScroll = active
	} else if active >= n.tabScroll+vis {
		n.tabScroll = active - vis + 1
	}
	n.tabScroll = n.clampedTabScroll()
}

// bodyRect is the page area — the bounds minus the strip band.
func (n *Notebook) bodyRect() Rect {
	r := n.Bounds()
	switch n.TabSide {
	case TabBottom:
		return Rect{X: r.X, Y: r.Y, W: r.W, H: r.H - n.stripH()}
	case TabLeft:
		return Rect{X: r.X + scaled(NotebookTabWidth), Y: r.Y, W: r.W - scaled(NotebookTabWidth), H: r.H}
	case TabRight:
		return Rect{X: r.X, Y: r.Y, W: r.W - scaled(NotebookTabWidth), H: r.H}
	default: // TabTop
		return Rect{X: r.X, Y: r.Y + n.stripH(), W: r.W, H: r.H - n.stripH()}
	}
}

// tabAt returns the tab index at a surface point, or -1.
func (n *Notebook) tabAt(px, py int) int {
	for i := range n.Tabs {
		if n.tabRect(i).Contains(px, py) {
			return i
		}
	}
	return -1
}

// drawActiveEdge paints the accent indicator on the active tab's body-facing
// edge — an underline for Top, an overline for Bottom, a side bar for Left/Right.
func (n *Notebook) drawActiveEdge(p painter.Painter, tr Rect, ink RGBA) {
	e := max(1, scaled(2))
	switch n.TabSide {
	case TabBottom:
		fillRect(p, tr.X, tr.Y, tr.W, e, ink)
	case TabLeft:
		fillRect(p, tr.X+tr.W-e, tr.Y, e, tr.H, ink)
	case TabRight:
		fillRect(p, tr.X, tr.Y, e, tr.H, ink)
	default: // TabTop
		fillRect(p, tr.X, tr.Y+tr.H-e, tr.W, e, ink)
	}
}

// NewNotebook returns an empty Notebook with no tabs + the Active Observable
// initialised to 0.
func NewNotebook() *Notebook {
	return &Notebook{active: mvvm.NewObservable(0)}
}

// AddTab appends a tab to the strip with label + the page widget
// shown when that tab is active.
func (n *Notebook) AddTab(label string, page Widget) {
	n.Tabs = append(n.Tabs, NotebookTab{Label: label, Page: page})
}

// Draw paints the strip (on the chosen side) + the active page. The whole
// render is clipped to Bounds() so nothing ever escapes the widget's box (a
// defence-in-depth over tabW's fit-to-width), and the active page is clipped to
// its body rect so an oversized page cannot paint over the tab strip.
func (n *Notebook) Draw(p painter.Painter, theme *Theme) {
	withClip(p, n.Bounds(), func() {
		strip := n.stripRect()
		active := n.Active().Get()
		fillRect(p, strip.X, strip.Y, strip.W, strip.H, theme.SurfaceAlt)
		for i, tab := range n.Tabs {
			tr := n.tabRect(i)
			fill := theme.SurfaceAlt
			if i == active {
				fill = theme.Surface
			}
			fillRect(p, tr.X, tr.Y, tr.W, tr.H, fill)
			// Label centred in the tab.
			tw := n.textWidth(tab.Label)
			textX := tr.X + (tr.W-tw)/2
			textY := tr.Y + (tr.H-n.glyphHeight())/2
			n.drawText(p, textX, textY, tab.Label, theme.OnSurface)
			if i == active {
				n.drawActiveEdge(p, tr, theme.Accent)
			}
		}
		// Active page in the body area, clipped to it.
		if active >= 0 && active < len(n.Tabs) {
			page := n.Tabs[active].Page
			if page != nil {
				body := n.bodyRect()
				page.SetBounds(body)
				withClip(p, body, func() { page.Draw(p, theme) })
			}
		}
	})
	n.drawFocusRing(p, theme, n.Bounds())
}

// OnEvent: a click on a tab (any side) selects it; a click in the body — or any
// non-click event — routes to the active page, translated into its local frame.
func (n *Notebook) OnEvent(ev Event) {
	r := n.Bounds()
	if ev.Kind == EventScroll && n.verticalStrip() {
		// Wheel over a vertical tab strip scrolls the stacked tabs (clamped at
		// both ends). ev is widget-local; hit-test the strip in surface coords.
		ax, ay := ev.X+r.X, ev.Y+r.Y
		if n.stripRect().Contains(ax, ay) {
			n.ScrollTabsBy(ev.Delta)
			return
		}
	}
	if ev.Kind == EventKeyDown && !n.Disabled().Get() {
		// Arrow keys move the active tab along the strip, wrapping, Setting the
		// Active Observable -- the tablist keyboard convention. Both axes are accepted
		// so it works for a horizontal (Top/Bottom) or vertical (Left/Right) strip.
		switch ev.Code {
		case "ArrowLeft", "ArrowUp":
			n.stepTab(-1)
			return
		case "ArrowRight", "ArrowDown":
			n.stepTab(+1)
			return
		}
	}
	if ev.Kind == EventClick {
		// ev is widget-local; hit-test the tabs in surface coordinates.
		ax, ay := ev.X+r.X, ev.Y+r.Y
		if idx := n.tabAt(ax, ay); idx >= 0 {
			n.setActive(idx)
			return
		}
		// A click that lands neither on a tab nor in the body (e.g. empty strip
		// space) is ignored.
		if !n.bodyRect().Contains(ax, ay) {
			return
		}
	}
	if active := n.Active().Get(); active >= 0 && active < len(n.Tabs) {
		page := n.Tabs[active].Page
		if page != nil {
			body := n.bodyRect()
			page.SetBounds(body)
			page.OnEvent(translateEvent(ev, r, body))
		}
	}
}

// setActive selects tab idx and Sets the Active Observable (notifying its
// subscribers) -- the shared mutate path for a tab click and a keyboard tab
// move.
func (n *Notebook) setActive(idx int) {
	n.Active().Set(idx)
	// Keep the newly-active tab visible on a vertical strip (a no-op for a
	// horizontal strip or an already-visible tab), so keyboard tab switching
	// onto an off-screen tab scrolls it into view.
	n.scrollActiveIntoView()
}

// stepTab moves the active tab delta tabs along the strip, wrapping at both
// ends, and Sets the Active Observable. A no-op when there are no tabs.
func (n *Notebook) stepTab(delta int) {
	count := len(n.Tabs)
	if count == 0 {
		return
	}
	cur := n.Active().Get()
	if cur < 0 || cur >= count {
		cur = 0
	}
	n.setActive(((cur+delta)%count + count) % count)
}
