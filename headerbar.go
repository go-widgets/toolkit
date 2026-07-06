// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// HeaderBarHeight is the default vertical extent of a HeaderBar in
// pixels. HeaderBar's Draw code assumes Bounds.H == HeaderBarHeight
// but scales cleanly for taller / shorter bars: children are inset
// by HeaderBarPad/2 top+bottom and the title / subtitle are centred
// in whatever remains.
const HeaderBarHeight = 40

// HeaderBarPad is the horizontal padding at the bar's left + right
// edges (space between the bar's edge and the first Start / End
// child). Also drives the vertical inset around child widgets:
// children get Bounds.Y = bar.Y + HeaderBarPad/2 and Bounds.H =
// bar.H - HeaderBarPad, matching GTK's typical inner spacing.
const HeaderBarPad = 8

// HeaderBarSubtitleGap is the vertical gap (in pixels) between the
// title's last row and the subtitle's first row in the two-line
// layout. Kept as a package constant so the two-line block height
// stays predictable across themes + font sizes.
const HeaderBarSubtitleGap = 2

// HeaderBar is the GTK "client-side decorations" bar: an optional
// row of Start widgets (usually navigation — back, menu), a centred
// Title (+ optional Subtitle) and an optional row of End widgets
// (usually actions — search, close). Composes cleanly above a
// Notebook + Statusbar so an app can assemble a stock GNOME window
// out of just three toolkit widgets.
//
// Start widgets paint left-to-right from the bar's left edge; End
// widgets paint right-to-left from the bar's right edge. The title
// (and subtitle, when non-empty) are centred horizontally in
// whatever space remains between the two child regions.
//
// HeaderBar does not intercept events itself; children receive
// events via the parent container's usual dispatch after HeaderBar
// has positioned them (Draw does the layout side-effect).
type HeaderBar struct {
	Base
	Title    string
	Subtitle string
	Start    []Widget // rendered left-to-right along the left edge
	End      []Widget // rendered right-to-left along the right edge
}

// NewHeaderBar constructs a HeaderBar carrying title. Subtitle,
// Start and End remain zero-valued; the caller populates them
// before the first Draw.
func NewHeaderBar(title string) *HeaderBar {
	return &HeaderBar{Title: title}
}

// Draw paints the bar body, positions + draws every Start / End
// child, then paints Title (+ Subtitle when non-empty) centred in
// whatever horizontal space is left between the two child regions.
//
// Positioning side effect: every Start / End widget's Bounds is
// updated to reflect its position inside the bar. The widget's
// original Bounds.W is preserved; Bounds.H is fitted to the bar's
// inner height (bar.H - HeaderBarPad). This mirrors GTK's
// HdyHeaderBar pattern — children carry their own preferred width
// but let the bar decide vertical placement.
func (h *HeaderBar) Draw(p painter.Painter, theme *Theme) {
	r := h.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)

	innerY := r.Y + HeaderBarPad/2
	innerH := r.H - HeaderBarPad

	// Start row: left-to-right from the bar's left edge.
	startX := r.X + HeaderBarPad
	for _, w := range h.Start {
		wb := w.Bounds()
		w.SetBounds(Rect{X: startX, Y: innerY, W: wb.W, H: innerH})
		w.Draw(p, theme)
		startX += wb.W
	}

	// End row: right-to-left from the bar's right edge.
	endX := r.X + r.W - HeaderBarPad
	for _, w := range h.End {
		wb := w.Bounds()
		endX -= wb.W
		w.SetBounds(Rect{X: endX, Y: innerY, W: wb.W, H: innerH})
		w.Draw(p, theme)
	}

	// Title / Subtitle centred in the region between Start + End.
	titleX0 := startX
	titleW := endX - startX

	if h.Subtitle == "" {
		if h.Title == "" {
			return
		}
		tw := TextWidth(h.Title)
		tx := titleX0 + (titleW-tw)/2
		ty := r.Y + (r.H-GlyphHeight)/2
		DrawText(p, tx, ty, h.Title, theme.OnSurface)
		return
	}

	// Two-line layout: title above subtitle, both centred as a
	// single block. Subtitle uses theme.Border for the "lighter"
	// muted-ink convention (matches GTK's dim-label styling).
	blockH := 2*GlyphHeight + HeaderBarSubtitleGap
	ty := r.Y + (r.H-blockH)/2
	if h.Title != "" {
		tw := TextWidth(h.Title)
		tx := titleX0 + (titleW-tw)/2
		DrawText(p, tx, ty, h.Title, theme.OnSurface)
	}
	sw := TextWidth(h.Subtitle)
	sx := titleX0 + (titleW-sw)/2
	sy := ty + GlyphHeight + HeaderBarSubtitleGap
	DrawText(p, sx, sy, h.Subtitle, theme.Border)
}
