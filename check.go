// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// CheckButton is a square checkbox + a label. Click toggles Checked
// + fires OnToggle. Visual: 12 x 12 px box (left-aligned), Theme.Border
// outline, Theme.Surface fill, Theme.Accent fill + two diagonal
// "checkmark" strokes in Theme.Background when Checked. Label
// rendered in Theme.OnBackground to the right of the box.
type CheckButton struct {
	Base
	Label    string
	Checked  bool
	Size     int // box side length in px; 0 uses the 12px default
	OnToggle func(checked bool)
}

// checkBoxSize is the default pixel side length of the box (used when Size == 0).
const checkBoxSize = 12

// boxSize returns the effective box side length.
func (c *CheckButton) boxSize() int {
	if c.Size > 0 {
		return c.Size
	}
	return checkBoxSize
}

// NewCheckButton constructs a CheckButton with the given label +
// initial Checked state.
func NewCheckButton(label string, checked bool) *CheckButton {
	return &CheckButton{Label: label, Checked: checked}
}

// Draw paints the box + checkmark + label.
func (c *CheckButton) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	box := c.boxSize()
	boxY := r.Y + (r.H-box)/2
	fill := theme.Surface
	if c.Checked {
		fill = theme.Accent
	}
	fillRect(p, r.X, boxY, box, box, fill)
	strokeRect(p, r.X, boxY, box, box, theme.Border)
	if c.Checked {
		if c.Size > 0 {
			drawCheckmark(p, r.X, boxY, box, theme.Background)
		} else {
			// The classic fixed 12px checkmark (byte-identical to prior releases).
			for t := 0; t < 4; t++ {
				fillRect(p, r.X+3+t, boxY+6+t, 1, 1, theme.Background)
			}
			for t := 0; t < 6; t++ {
				fillRect(p, r.X+6+t, boxY+9-t, 1, 1, theme.Background)
			}
		}
	}
	// Label to the right of the box, vertically centred on glyph row.
	textY := r.Y + (r.H-c.glyphHeight())/2
	c.drawText(p, r.X+box+4, textY, c.Label, theme.OnBackground)
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

// OnEvent flips Checked + fires OnToggle on click.
func (c *CheckButton) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	c.Checked = !c.Checked
	if c.OnToggle != nil {
		c.OnToggle(c.Checked)
	}
}
