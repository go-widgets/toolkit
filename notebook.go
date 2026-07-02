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

// Notebook is a tabbed container. The top NotebookTabStripH pixels
// host the tab strip; the rest is the active page's body. Clicking
// a tab swaps Active + fires OnTabChanged.
type Notebook struct {
	Base
	Tabs         []NotebookTab
	Active       int
	OnTabChanged func(idx int)
}

// Geometry constants for the tab strip.
const (
	NotebookTabStripH = 24
	NotebookTabWidth  = 80
)

// NewNotebook returns an empty Notebook with no tabs + Active = 0.
func NewNotebook() *Notebook { return &Notebook{} }

// AddTab appends a tab to the strip with label + the page widget
// shown when that tab is active.
func (n *Notebook) AddTab(label string, page Widget) {
	n.Tabs = append(n.Tabs, NotebookTab{Label: label, Page: page})
}

// Draw paints the strip + the active page.
func (n *Notebook) Draw(p painter.Painter, theme *Theme) {
	r := n.Bounds()
	// Strip background.
	fillRect(p, r.X, r.Y, r.W, NotebookTabStripH, theme.SurfaceAlt)
	for i, tab := range n.Tabs {
		tx := r.X + i*NotebookTabWidth
		fill := theme.SurfaceAlt
		if i == n.Active {
			fill = theme.Surface
		}
		fillRect(p, tx, r.Y, NotebookTabWidth, NotebookTabStripH, fill)
		// Label centred in the tab.
		tw := TextWidth(tab.Label)
		textX := tx + (NotebookTabWidth-tw)/2
		textY := r.Y + (NotebookTabStripH-GlyphHeight)/2
		DrawText(p, textX, textY, tab.Label, theme.OnSurface)
		if i == n.Active {
			// Accent underline so the active tab reads as selected.
			fillRect(p, tx, r.Y+NotebookTabStripH-2, NotebookTabWidth, 2, theme.Accent)
		}
	}
	// Active page in the body area.
	if n.Active >= 0 && n.Active < len(n.Tabs) {
		page := n.Tabs[n.Active].Page
		if page != nil {
			body := Rect{X: r.X, Y: r.Y + NotebookTabStripH, W: r.W, H: r.H - NotebookTabStripH}
			page.SetBounds(body)
			page.Draw(p, theme)
		}
	}
}

// OnEvent: click on the strip selects a tab; click in the body
// routes to the active page.
func (n *Notebook) OnEvent(ev Event) {
	if ev.Kind == EventClick && ev.Y < NotebookTabStripH {
		idx := ev.X / NotebookTabWidth
		if idx >= 0 && idx < len(n.Tabs) {
			n.Active = idx
			if n.OnTabChanged != nil {
				n.OnTabChanged(idx)
			}
		}
		return
	}
	if n.Active >= 0 && n.Active < len(n.Tabs) {
		page := n.Tabs[n.Active].Page
		if page != nil {
			page.OnEvent(ev)
		}
	}
}
