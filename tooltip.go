// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Tooltip is a small text bubble shown near the cursor when the user
// hovers over a target widget. The host app drives Visible + Anchor
// (typically toggled by a mouse-enter/leave handler with a 500 ms
// delay); the toolkit's role is the rendering geometry.
//
// Auto-sized to the Text width + 8 px horizontal padding + 6 px
// vertical padding; appears just below/right of (Anchor.X, Anchor.Y).
type Tooltip struct {
	Base
	Text    string
	Visible bool
	Anchor  Rect // widget the tooltip belongs to; positions below it
}

// TooltipPadX / TooltipPadY are the inner text-padding constants.
const (
	TooltipPadX = 8
	TooltipPadY = 4
)

// NewTooltip builds a hidden tooltip with the given text.
func NewTooltip(text string) *Tooltip { return &Tooltip{Text: text} }

// Show makes the tooltip visible, anchored to the given widget rect.
func (t *Tooltip) Show(anchor Rect) {
	t.Visible = true
	t.Anchor = anchor
	w := TextWidth(t.Text) + 2*TooltipPadX
	h := GlyphHeight + 2*TooltipPadY
	t.SetBounds(Rect{
		X: anchor.X,
		Y: anchor.Y + anchor.H + 2,
		W: w,
		H: h,
	})
}

// Hide removes the tooltip from view.
func (t *Tooltip) Hide() { t.Visible = false }

// Draw paints the bubble when Visible.
func (t *Tooltip) Draw(p painter.Painter, theme *Theme) {
	if !t.Visible {
		return
	}
	r := t.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.OnSurface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	DrawText(p, r.X+TooltipPadX, r.Y+TooltipPadY, t.Text, theme.Background)
}
