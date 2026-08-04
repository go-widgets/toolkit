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
// HeaderBar positions its Start/End children in SetBounds (so their
// Bounds are correct before the first paint) and forwards pointer
// events to them in OnEvent, hit-testing each child and translating the
// event into its local frame -- the same dispatch HBox does. A child
// whose Bounds contains the event handles it; a click elsewhere on the
// bar is ignored.
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

// SetBounds positions the bar + lays out its Start/End children so their
// Bounds are correct before the first paint (and before any OnEvent dispatch).
func (h *HeaderBar) SetBounds(r Rect) {
	h.Base.SetBounds(r)
	h.layout()
}

// layout positions every Start / End child inside the bar and returns the
// central strip (X origin + width) left for the Title / Subtitle. Each child's
// original Bounds.W is preserved; its Bounds.H is fitted to the bar's inner
// height (bar.H - HeaderBarPad) and Y to the inner top -- GTK's HdyHeaderBar
// pattern, where children carry their own width but the bar owns vertical
// placement. Shared by SetBounds (positions ahead of paint/events), Draw and
// OnEvent, so the drawn child, its event target and its stored Bounds can never
// drift apart.
func (h *HeaderBar) layout() (titleX0, titleW int) {
	r := h.Bounds()
	innerY := r.Y + HeaderBarPad/2
	innerH := r.H - HeaderBarPad

	// Start row: left-to-right from the bar's left edge.
	startX := r.X + HeaderBarPad
	for _, w := range h.Start {
		wb := w.Bounds()
		w.SetBounds(Rect{X: startX, Y: innerY, W: wb.W, H: innerH})
		startX += wb.W
	}

	// End row: right-to-left from the bar's right edge.
	endX := r.X + r.W - HeaderBarPad
	for _, w := range h.End {
		wb := w.Bounds()
		endX -= wb.W
		w.SetBounds(Rect{X: endX, Y: innerY, W: wb.W, H: innerH})
	}
	return startX, endX - startX
}

// Draw paints the bar body, (re)positions + draws every Start / End child, then
// paints Title (+ Subtitle when non-empty) centred in whatever horizontal space
// is left between the two child regions.
func (h *HeaderBar) Draw(p painter.Painter, theme *Theme) {
	r := h.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)

	// Re-run layout so children added after SetBounds still position, then
	// paint each in its Bounds.
	titleX0, titleW := h.layout()
	for _, w := range h.Start {
		w.Draw(p, theme)
	}
	for _, w := range h.End {
		w.Draw(p, theme)
	}

	if h.Subtitle == "" {
		if h.Title == "" {
			return
		}
		tw := h.textWidth(h.Title)
		tx := titleX0 + (titleW-tw)/2
		ty := r.Y + (r.H-h.glyphHeight())/2
		h.drawText(p, tx, ty, h.Title, theme.OnSurface)
		return
	}

	// Two-line layout: title above subtitle, both centred as a
	// single block. Subtitle uses theme.Border for the "lighter"
	// muted-ink convention (matches GTK's dim-label styling).
	blockH := 2*h.glyphHeight() + HeaderBarSubtitleGap
	ty := r.Y + (r.H-blockH)/2
	if h.Title != "" {
		tw := h.textWidth(h.Title)
		tx := titleX0 + (titleW-tw)/2
		h.drawText(p, tx, ty, h.Title, theme.OnSurface)
	}
	sw := h.textWidth(h.Subtitle)
	sx := titleX0 + (titleW-sw)/2
	sy := ty + h.glyphHeight() + HeaderBarSubtitleGap
	h.drawText(p, sx, sy, h.Subtitle, dimInk(theme))
}

// OnEvent forwards a pointer event to the first Start / End child whose Bounds
// contains it, translated into that child's local frame -- mirroring HBox's
// dispatch. EventMouseMove goes to every child (so each raises/clears its hover
// face); every other kind lands only on the child under the pointer. Event
// coordinates are widget-local. layout() runs first so a child added after the
// last SetBounds still has current Bounds to hit-test against.
func (h *HeaderBar) OnEvent(ev Event) {
	h.layout()
	pr := h.Bounds()
	if ev.Kind == EventMouseMove {
		for _, w := range h.Start {
			w.OnEvent(translateEvent(ev, pr, w.Bounds()))
		}
		for _, w := range h.End {
			w.OnEvent(translateEvent(ev, pr, w.Bounds()))
		}
		return
	}
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, w := range h.Start {
		if w.Bounds().Contains(sx, sy) {
			w.OnEvent(translateEvent(ev, pr, w.Bounds()))
			return
		}
	}
	for _, w := range h.End {
		if w.Bounds().Contains(sx, sy) {
			w.OnEvent(translateEvent(ev, pr, w.Bounds()))
			return
		}
	}
}
