// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Badge is a small pill-shaped counter or indicator — the "12" that
// hangs off an inbox icon, the "NEW" beside a menu item. Renders Text
// inside a rounded-pill body filled in Fill (Theme.Accent by default)
// with the ink in Ink (Theme.Background by default) for contrast.
//
// A Badge is passive: it displays a value + does not respond to input.
// The parent widget (button, menu item, ...) is responsible for
// positioning it in the top-right corner or wherever the design puts it.
//
// Per-badge colour: Fill overrides the pill body colour and Ink the
// text colour. Both default to the zero RGBA, in which case Draw falls
// back to Theme.Accent / Theme.Background — so a plain NewBadge keeps
// the theme look, while a caller that needs a categorical colour (a
// per-source tag, a severity chip, ...) sets Fill/Ink without having to
// hand-draw its own pill. A fully-transparent colour (A==0) is treated
// as "unset"; callers wanting a see-through badge is not a use case the
// widget serves.
//
// Auto-sizing: if the caller sets Bounds().W to 0, the first Draw()
// resizes the Bounds to the text width plus BadgePadX on each side
// (plus GlyphHeight() + BadgePadY on each side vertically if H is also 0).
// This spares the caller from having to compute glyph widths just to
// paint a two-digit counter. A pre-sized Bounds is honoured verbatim
// so a fixed-width layout column doesn't shift when the digit count
// changes.
type Badge struct {
	Base
	Text string
	Fill RGBA // pill body colour; zero (A==0) => Theme.Accent
	Ink  RGBA // text colour; zero (A==0) => Theme.Background
}

// BadgePadX / BadgePadY are the horizontal and vertical insets between
// the pill body and the text glyphs. Small: a badge should read as a
// compact tag, not a button. Vertical padding is intentionally 1 so
// the pill stays short next to same-line body text.
const (
	BadgePadX = 4
	BadgePadY = 1
)

// NewBadge constructs a Badge with the given text. Bounds default to
// zero so the first Draw() auto-sizes the pill to the text.
func NewBadge(text string) *Badge { return &Badge{Text: text} }

// Draw paints the pill body + centred text. If Bounds().W is zero the
// widget resizes itself to fit its Text (and Bounds().H is filled in
// too if it was zero) before painting; a pre-sized Bounds is preserved.
//
// The pill body is a full rounded-rect painted through the painter's
// FillRoundRect (radius = half the shorter side, so short pills read as
// a stadium and tall ones as a circle). Back-ends that cannot round (a
// cell grid) degrade to a square fill. Fill/Ink override the body/text
// colours; an unset (transparent) colour falls back to the theme.
func (b *Badge) Draw(p painter.Painter, theme *Theme) {
	r := b.Bounds()
	if r.W == 0 {
		r.W = b.textWidth(b.Text) + 2*BadgePadX
		if r.H == 0 {
			r.H = b.glyphHeight() + 2*BadgePadY
		}
		b.SetBounds(r)
	}
	body := theme.Accent
	if b.Fill.A != 0 {
		body = b.Fill
	}
	ink := theme.Background
	if b.Ink.A != 0 {
		ink = b.Ink
	}
	radius := r.H
	if r.W < radius {
		radius = r.W
	}
	p.FillRoundRect(painter.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}, radius/2, body)
	tw := b.textWidth(b.Text)
	tx := r.X + (r.W-tw)/2
	ty := r.Y + (r.H-b.glyphHeight())/2
	b.drawText(p, tx, ty, b.Text, ink)
}
