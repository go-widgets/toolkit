// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

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
// hovers over a target widget. The host app drives the reactive
// visibility + anchor (typically toggled by a mouse-enter/leave handler
// with a 500 ms delay) through the [Tooltip.Visible] and [Tooltip.Anchor]
// Observables; the toolkit's role is the rendering geometry.
//
// Auto-sized to the Text width + padding; positioned on the side of the anchor
// chosen by Placement (below by default). Text + Placement are set-once config;
// the reactive state (whether it is shown, and which rect it points at) is
// MVVM-only, unexported behind the Observable accessors.
type Tooltip struct {
	Base
	Text      string
	Placement TooltipPlacement

	visible *mvvm.Observable[bool]
	anchor  *mvvm.Observable[Rect]
}

// Visible is the tooltip's shown/hidden state as a shared [mvvm.Observable]: a
// host binds it (Set / Subscribe / two-way) — there is no settable Visible
// field. [Tooltip.Show] Sets it true, [Tooltip.Hide] Sets it false; Draw reads
// it. Lazily initialised to false so a bare &Tooltip{} is usable.
func (t *Tooltip) Visible() *mvvm.Observable[bool] {
	if t.visible == nil {
		t.visible = mvvm.NewObservable(false)
	}
	return t.visible
}

// Anchor is the rect the tooltip points at as a shared [mvvm.Observable]:
// [Tooltip.Show] Sets it to the anchored widget's rect. There is no settable
// Anchor field. Lazily initialised to the zero Rect so a bare &Tooltip{} is
// usable.
func (t *Tooltip) Anchor() *mvvm.Observable[Rect] {
	if t.anchor == nil {
		t.anchor = mvvm.NewObservable(Rect{})
	}
	return t.anchor
}

// TooltipPadX / TooltipPadY are the inner text-padding constants.
const (
	TooltipPadX = 8
	TooltipPadY = 4
)

// NewTooltip builds a hidden tooltip with the given text. Both reactive
// Observables are initialised (Visible false, Anchor the zero Rect).
func NewTooltip(text string) *Tooltip {
	return &Tooltip{
		Text:    text,
		visible: mvvm.NewObservable(false),
		anchor:  mvvm.NewObservable(Rect{}),
	}
}

// Show makes the tooltip visible, anchored to the given widget rect.
func (t *Tooltip) Show(anchor Rect) {
	t.Visible().Set(true)
	t.Anchor().Set(anchor)
	w := t.textWidth(t.Text) + 2*TooltipPadX
	h := t.glyphHeight() + 2*TooltipPadY
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
func (t *Tooltip) Hide() { t.Visible().Set(false) }

// Draw paints the bubble when Visible.
func (t *Tooltip) Draw(p painter.Painter, theme *Theme) {
	if !t.Visible().Get() {
		return
	}
	r := t.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.OnSurface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	t.drawText(p, r.X+TooltipPadX, r.Y+TooltipPadY, t.Text, theme.Background)
}
