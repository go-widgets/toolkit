// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// TooltipPlacement selects which side of the anchor the bubble sits on. Below
// is the zero value (the original behaviour).
type TooltipPlacement int

const (
	// PlaceBelow puts the bubble under the anchor (the default).
	PlaceBelow TooltipPlacement = iota
	// PlaceAbove puts the bubble over the anchor.
	PlaceAbove
	// PlaceLeft puts the bubble to the anchor's left.
	PlaceLeft
	// PlaceRight puts the bubble to the anchor's right.
	PlaceRight
)

// Tooltip is a small text bubble shown near the cursor when the user
// hovers over a target widget. The host app drives Visible + Anchor
// (typically toggled by a mouse-enter/leave handler with a 500 ms
// delay); the toolkit's role is the rendering geometry.
//
// Auto-sized to the Text width + padding; positioned on the side of the anchor
// chosen by Placement (below by default).
type Tooltip struct {
	Base
	Text      string
	Visible   bool
	Placement TooltipPlacement
	Anchor    Rect // widget the tooltip belongs to
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
	h := GlyphHeight() + 2*TooltipPadY
	var x, y int
	switch t.Placement {
	case PlaceAbove:
		x, y = anchor.X, anchor.Y-h-2
	case PlaceLeft:
		x, y = anchor.X-w-2, anchor.Y
	case PlaceRight:
		x, y = anchor.X+anchor.W+2, anchor.Y
	default: // PlaceBelow
		x, y = anchor.X, anchor.Y+anchor.H+2
	}
	t.SetBounds(Rect{X: x, Y: y, W: w, H: h})
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
