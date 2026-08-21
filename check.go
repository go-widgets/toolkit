// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// CheckButton is a square checkbox + a label. A click toggles the reactive
// Checked state, notifying its Observable's subscribers. Visual: 12 x 12 px box
// (left-aligned), Theme.Border outline, Theme.Surface fill, Theme.Accent fill +
// two diagonal "checkmark" strokes in Theme.Background when Checked. Label
// rendered in Theme.OnBackground to the right of the box.
type CheckButton struct {
	Base
	focusState
	// Label + Size are config. The reactive checked state is MVVM-only: it lives
	// in an unexported Observable exposed via [CheckButton.Checked].
	Label string
	Size  int // box side length in px; 0 uses the 12px default

	checked *mvvm.Observable[bool]
}

// Checked is the current checked state as a shared [mvvm.Observable]: a host
// binds it (Set / Subscribe / two-way) — there is no settable Checked field. A
// click or a Space/Enter key press Sets it (flipping the value); subscribers are
// notified on change.
func (c *CheckButton) Checked() *mvvm.Observable[bool] {
	if c.checked == nil {
		c.checked = mvvm.NewObservable(false)
	}
	return c.checked
}

// checkBoxSize is the default LOGICAL side length of the box (used when
// Size == 0); the effective size is this at the current metric scale.
const checkBoxSize = 12

// checkLabelGap is the logical gap between the box and its label.
const checkLabelGap = 4

// boxSize returns the effective box side length.
func (c *CheckButton) boxSize() int {
	if c.Size > 0 {
		return c.Size // a caller-chosen size is already in the units it chose
	}
	return scaled(checkBoxSize)
}

// NewCheckButton constructs a CheckButton with the given label +
// initial Checked state.
func NewCheckButton(label string, checked bool) *CheckButton {
	return &CheckButton{Label: label, checked: mvvm.NewObservable(checked)}
}

// Draw paints the box + checkmark + label.
func (c *CheckButton) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	box := c.boxSize()
	boxY := r.Y + (r.H-box)/2
	fill := theme.Surface
	if c.Checked().Get() {
		fill = theme.Accent
	}
	// A disabled checkbox mutes the box, border, checkmark and label so it
	// reads as inert. Only taken when Disabled — the enabled draw is unchanged.
	border, tick, labelInk := theme.Border, theme.Background, theme.OnBackground
	if c.Disabled().Get() {
		fill, border, tick, labelInk = mutedFace(theme), mutedInk(theme), mutedInk(theme), mutedInk(theme)
	}
	fillRect(p, r.X, boxY, box, box, fill)
	strokeRect(p, r.X, boxY, box, box, border)
	if c.Checked().Get() {
		if box != checkBoxSize {
			// Any box that is not the classic 12 -- a caller-chosen Size, or the
			// default one at a metric scale above 1 -- gets the proportional
			// tick, so the mark grows with the box instead of sitting in its
			// top-left corner.
			drawCheckmark(p, r.X, boxY, box, tick)
		} else {
			// The classic fixed 12px checkmark (byte-identical to prior releases).
			for t := 0; t < 4; t++ {
				fillRect(p, r.X+3+t, boxY+6+t, 1, 1, tick)
			}
			for t := 0; t < 6; t++ {
				fillRect(p, r.X+6+t, boxY+9-t, 1, 1, tick)
			}
		}
	}
	// Label to the right of the box, vertically centred on glyph row.
	textY := r.Y + (r.H-c.glyphHeight())/2
	c.drawText(p, r.X+box+scaled(checkLabelGap), textY, c.Label, labelInk)
	c.drawFocusRing(p, theme, r)
}

// drawCheckmark strokes a two-segment "✓" scaled to a box of side b at (x, y),
// so a Sized checkbox's tick scales with it. Strokes are square blocks (the
// painter has no line primitive), thickness proportional to b.
func drawCheckmark(p painter.Painter, x, y, b int, ink RGBA) {
	sw := b / 12
	if sw < 1 {
		sw = 1
	}
	for t := 0; t <= b/3; t++ { // down-stroke: (0.25b,0.5b) → (~0.58b,0.83b)
		fillRect(p, x+b/4+t, y+b/2+t, sw, sw, ink)
	}
	for t := 0; t <= b/2; t++ { // up-stroke: (0.58b,0.83b) → (~0.92b,0.33b)
		fillRect(p, x+b/4+b/3+t, y+b/2+b/3-t, sw, sw, ink)
	}
}

// OnEvent flips the Checked Observable on click. A Disabled checkbox ignores
// every kind.
func (c *CheckButton) OnEvent(ev Event) {
	if c.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventClick:
		c.toggle()
	case EventKeyDown:
		// Space or Enter toggles the box while focused, reusing the click path.
		switch ev.Code {
		case " ", "Space", "Enter":
			c.toggle()
		}
	}
}

// toggle flips the Checked Observable, notifying its subscribers -- the shared
// mutate path for a click and a Space/Enter key press.
func (c *CheckButton) toggle() {
	c.Checked().Set(!c.Checked().Get())
}

// HitRect is the checkbox's interactive rectangle: its drawn Bounds clamped up
// to the density hit-target and centred over them (see [touchHitRect]). A
// checkbox row is only a dozen logical pixels tall, so under DensityTouch its
// hit height grows to the >=44px finger floor while the drawn 12px box is
// untouched; byte-identical to Bounds under DensityCompact.
func (c *CheckButton) HitRect() Rect { return touchHitRect(c.Bounds()) }

// HitTest reports whether a surface point falls on the checkbox's (touch-clamped)
// hit rect.
func (c *CheckButton) HitTest(px, py int) bool { return c.HitRect().Contains(px, py) }
