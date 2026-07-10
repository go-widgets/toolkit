// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Badge is a small pill-shaped counter or indicator — the "12" that
// hangs off an inbox icon, the "NEW" beside a menu item. Renders Text
// inside a rounded-pill body filled in Theme.Accent with the ink in
// Theme.Background for contrast.
//
// A Badge is passive: it displays a value + does not respond to input.
// The parent widget (button, menu item, ...) is responsible for
// positioning it in the top-right corner or wherever the design puts it.
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
// The pill shape is approximated by clipping the four corner pixels:
// the body fills the full rectangle minus a one-pixel bite off each
// corner, which reads as "rounded" against the low-resolution 5x7
// glyph aesthetic without touching the painter's curve primitives.
func (b *Badge) Draw(p painter.Painter, theme *Theme) {
	r := b.Bounds()
	if r.W == 0 {
		r.W = TextWidth(b.Text) + 2*BadgePadX
		if r.H == 0 {
			r.H = GlyphHeight() + 2*BadgePadY
		}
		b.SetBounds(r)
	}
	// Three fills approximate a pill: centre strip full-height, then
	// two 1-px side columns that skip the top + bottom pixels so the
	// corners read as rounded.
	fillRect(p, r.X+1, r.Y, r.W-2, r.H, theme.Accent)
	fillRect(p, r.X, r.Y+1, 1, r.H-2, theme.Accent)
	fillRect(p, r.X+r.W-1, r.Y+1, 1, r.H-2, theme.Accent)
	tw := TextWidth(b.Text)
	tx := r.X + (r.W-tw)/2
	ty := r.Y + (r.H-GlyphHeight())/2
	DrawText(p, tx, ty, b.Text, theme.Background)
}
