// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Chip is a small labelled pill with an optional "x" close affordance.
// Unlike Badge -- which is a passive counter / status indicator with
// no interaction surface -- Chip is a removable tag: when Closable is
// true the widget renders a click target at the right edge that fires
// OnClose when tapped. Two constructors keep the two personalities
// distinct at the callsite: NewChip for a passive tag, NewClosableChip
// for the removable variant.
//
// Auto-sizing follows Badge's convention: if Bounds().W is zero the
// first Draw() sets W to the text width plus ChipPadX on each side
// (plus the close slot's ChipCloseGap + ChipCloseW when Closable is
// true), and H to GlyphHeight() + 2*ChipPadY when it is also zero. A
// pre-sized Bounds is honoured verbatim so a fixed-width layout row
// does not shift when the chip label changes.
type Chip struct {
	Base
	Text     string
	Closable bool
	OnClose  func()
}

// Chip sizing constants. PadX / PadY are the inner insets from the
// pill edge to the Text glyphs (kept small so a row of chips reads as
// compact tags); CloseW is the pixel width of the "x" click slot at
// the right edge; CloseGap is the pixel gap between the Text and the
// close slot when Closable is true.
const (
	ChipPadX     = 8
	ChipPadY     = 2
	ChipCloseW   = 12
	ChipCloseGap = 4
)

// NewChip constructs a passive (non-closable) Chip carrying the given
// Text. OnClose stays nil; the widget ignores clicks. Bounds default
// to zero so the first Draw() auto-sizes the pill.
func NewChip(text string) *Chip {
	return &Chip{Text: text}
}

// NewClosableChip constructs a Chip whose right edge exposes an "x"
// close affordance. onClose may be nil (clicks on the affordance become
// a no-op rather than a panic) so callers can wire the callback after
// construction without ordering constraints.
func NewClosableChip(text string, onClose func()) *Chip {
	return &Chip{Text: text, Closable: true, OnClose: onClose}
}

// Draw paints the pill body + text + optional close affordance. Auto-
// sizes Bounds when W is zero. The pill body is a filled SurfaceAlt
// rectangle stroked with a Border outline; the Text is drawn left-
// aligned inside the pad, and (when Closable) an "x" glyph in Border
// colour marks the close slot at the right edge.
func (c *Chip) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	if r.W == 0 {
		r.W = TextWidth(c.Text) + 2*ChipPadX
		if c.Closable {
			r.W += ChipCloseGap + ChipCloseW
		}
		if r.H == 0 {
			r.H = GlyphHeight() + 2*ChipPadY
		}
		c.SetBounds(r)
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	tx := r.X + ChipPadX
	ty := r.Y + (r.H-GlyphHeight())/2
	DrawText(p, tx, ty, c.Text, theme.OnSurface)
	if c.Closable {
		cx := r.X + r.W - ChipPadX - ChipCloseW + (ChipCloseW-TextWidth("x"))/2
		DrawText(p, cx, ty, "x", theme.Border)
	}
}

// OnEvent fires OnClose when an EventClick lands in the right-hand
// close slot and Closable is true. Non-click events, non-closable
// chips, and clicks outside the slot are ignored. A nil OnClose is
// treated as a no-op so callers can toggle Closable without wiring
// a callback in the same statement.
//
// Event coordinates are widget-local (as documented on Event), so the
// slot's horizontal extent is measured against r.W rather than r.X;
// no localisation is required at the callsite.
func (c *Chip) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	if !c.Closable {
		return
	}
	r := c.Bounds()
	left := r.W - ChipPadX - ChipCloseW
	right := r.W - ChipPadX
	if ev.X < left || ev.X >= right {
		return
	}
	if c.OnClose != nil {
		c.OnClose()
	}
}
