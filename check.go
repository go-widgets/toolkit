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
	OnToggle func(checked bool)
}

// checkBoxSize is the pixel side length of the box drawn next to the
// label.
const checkBoxSize = 12

// NewCheckButton constructs a CheckButton with the given label +
// initial Checked state.
func NewCheckButton(label string, checked bool) *CheckButton {
	return &CheckButton{Label: label, Checked: checked}
}

// Draw paints the box + checkmark + label.
func (c *CheckButton) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	boxY := r.Y + (r.H-checkBoxSize)/2
	fill := theme.Surface
	if c.Checked {
		fill = theme.Accent
	}
	fillRect(p, r.X, boxY, checkBoxSize, checkBoxSize, fill)
	strokeRect(p, r.X, boxY, checkBoxSize, checkBoxSize, theme.Border)
	if c.Checked {
		// Two-segment checkmark "✓" in Theme.Background, approximated as
		// short diagonal strokes inside the box.
		for t := 0; t < 4; t++ {
			fillRect(p, r.X+3+t, boxY+6+t, 1, 1, theme.Background)
		}
		for t := 0; t < 6; t++ {
			fillRect(p, r.X+6+t, boxY+9-t, 1, 1, theme.Background)
		}
	}
	// Label to the right of the box, vertically centred on glyph row.
	textY := r.Y + (r.H-GlyphHeight)/2
	DrawText(p, r.X+checkBoxSize+4, textY, c.Label, theme.OnBackground)
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
