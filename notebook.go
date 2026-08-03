// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

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

// Notebook is a tabbed container. A NotebookTabStripH-thick strip on the side
// chosen by TabSide (Top by default) hosts the tabs; the rest is the active
// page's body. For Top/Bottom the tabs run horizontally (each NotebookTabWidth
// wide); for Left/Right they stack vertically (each NotebookTabStripH tall).
// Clicking a tab swaps Active + fires OnTabChanged.
type Notebook struct {
	Base
	Tabs         []NotebookTab
	Active       int
	TabSide      TabSide
	OnTabChanged func(idx int)
}

// Geometry constants for the tab strip: the strip's thickness (its height for a
// Top/Bottom strip, its width for a Left/Right strip) and each tab's extent
// along the strip.
const (
	NotebookTabStripH = 24
	NotebookTabWidth  = 80
)

// stripRect is the tab-strip band on the chosen side.
func (n *Notebook) stripRect() Rect {
	r := n.Bounds()
	switch n.TabSide {
	case TabBottom:
		return Rect{X: r.X, Y: r.Y + r.H - NotebookTabStripH, W: r.W, H: NotebookTabStripH}
	case TabLeft:
		return Rect{X: r.X, Y: r.Y, W: NotebookTabWidth, H: r.H}
	case TabRight:
		return Rect{X: r.X + r.W - NotebookTabWidth, Y: r.Y, W: NotebookTabWidth, H: r.H}
	default: // TabTop
		return Rect{X: r.X, Y: r.Y, W: r.W, H: NotebookTabStripH}
	}
}

// tabW is the per-tab width for the horizontal (Top/Bottom) strips: the
// nominal NotebookTabWidth, shrunk so that ALL tabs fit within the notebook's
// width when there are too many to fit at the nominal width. Without this the
// strip laid tabs at a fixed 80px each and overflowed the widget's box once
// nTabs*80 exceeded Bounds().W (e.g. 4 tabs in a ~296px notebook).
func (n *Notebook) tabW() int {
	nt := len(n.Tabs)
	if nt == 0 {
		return NotebookTabWidth
	}
	tw := NotebookTabWidth
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
		return Rect{X: r.X + i*tw, Y: r.Y + r.H - NotebookTabStripH, W: tw, H: NotebookTabStripH}
	case TabLeft:
		return Rect{X: r.X, Y: r.Y + i*NotebookTabStripH, W: NotebookTabWidth, H: NotebookTabStripH}
	case TabRight:
		return Rect{X: r.X + r.W - NotebookTabWidth, Y: r.Y + i*NotebookTabStripH, W: NotebookTabWidth, H: NotebookTabStripH}
	default: // TabTop
		tw := n.tabW()
		return Rect{X: r.X + i*tw, Y: r.Y, W: tw, H: NotebookTabStripH}
	}
}

// bodyRect is the page area — the bounds minus the strip band.
func (n *Notebook) bodyRect() Rect {
	r := n.Bounds()
	switch n.TabSide {
	case TabBottom:
		return Rect{X: r.X, Y: r.Y, W: r.W, H: r.H - NotebookTabStripH}
	case TabLeft:
		return Rect{X: r.X + NotebookTabWidth, Y: r.Y, W: r.W - NotebookTabWidth, H: r.H}
	case TabRight:
		return Rect{X: r.X, Y: r.Y, W: r.W - NotebookTabWidth, H: r.H}
	default: // TabTop
		return Rect{X: r.X, Y: r.Y + NotebookTabStripH, W: r.W, H: r.H - NotebookTabStripH}
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
	switch n.TabSide {
	case TabBottom:
		fillRect(p, tr.X, tr.Y, tr.W, 2, ink)
	case TabLeft:
		fillRect(p, tr.X+tr.W-2, tr.Y, 2, tr.H, ink)
	case TabRight:
		fillRect(p, tr.X, tr.Y, 2, tr.H, ink)
	default: // TabTop
		fillRect(p, tr.X, tr.Y+tr.H-2, tr.W, 2, ink)
	}
}

// NewNotebook returns an empty Notebook with no tabs + Active = 0.
func NewNotebook() *Notebook { return &Notebook{} }

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
		fillRect(p, strip.X, strip.Y, strip.W, strip.H, theme.SurfaceAlt)
		for i, tab := range n.Tabs {
			tr := n.tabRect(i)
			fill := theme.SurfaceAlt
			if i == n.Active {
				fill = theme.Surface
			}
			fillRect(p, tr.X, tr.Y, tr.W, tr.H, fill)
			// Label centred in the tab.
			tw := n.textWidth(tab.Label)
			textX := tr.X + (tr.W-tw)/2
			textY := tr.Y + (tr.H-n.glyphHeight())/2
			n.drawText(p, textX, textY, tab.Label, theme.OnSurface)
			if i == n.Active {
				n.drawActiveEdge(p, tr, theme.Accent)
			}
		}
		// Active page in the body area, clipped to it.
		if n.Active >= 0 && n.Active < len(n.Tabs) {
			page := n.Tabs[n.Active].Page
			if page != nil {
				body := n.bodyRect()
				page.SetBounds(body)
				withClip(p, body, func() { page.Draw(p, theme) })
			}
		}
	})
}

// OnEvent: a click on a tab (any side) selects it; a click in the body — or any
// non-click event — routes to the active page, translated into its local frame.
func (n *Notebook) OnEvent(ev Event) {
	r := n.Bounds()
	if ev.Kind == EventClick {
		// ev is widget-local; hit-test the tabs in surface coordinates.
		ax, ay := ev.X+r.X, ev.Y+r.Y
		if idx := n.tabAt(ax, ay); idx >= 0 {
			n.Active = idx
			if n.OnTabChanged != nil {
				n.OnTabChanged(idx)
			}
			return
		}
		// A click that lands neither on a tab nor in the body (e.g. empty strip
		// space) is ignored.
		if !n.bodyRect().Contains(ax, ay) {
			return
		}
	}
	if n.Active >= 0 && n.Active < len(n.Tabs) {
		page := n.Tabs[n.Active].Page
		if page != nil {
			body := n.bodyRect()
			page.SetBounds(body)
			page.OnEvent(translateEvent(ev, r, body))
		}
	}
}
