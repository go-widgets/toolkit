// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"

	"github.com/go-widgets/painter"
)

// ComboBox is an editable, type-to-filter dropdown: a single-line text field
// the user can type into, backed by a popover list of Options filtered to
// those containing the typed Text. It sits between Entry (a free-text field
// with no list) and DropDown (a closed list with no typing): the field accepts
// free text AND offers the matching options for one-click / Enter selection.
//
// Like DropDown and DatePicker, the popover appears just below the field. The
// widget renders that list itself when Open (so it works standalone), while a
// host that composites overlays on a separate surface can instead read Open +
// PopoverBounds and draw the list there.
type ComboBox struct {
	Base
	Options []string
	// Text is the current field value — either free text the user typed or an
	// option they selected. Filtered() narrows Options against it.
	Text string
	// Placeholder is shown in the muted tone when Text is empty (a hint such as
	// "search…" or "pick a colour").
	Placeholder string
	// Open reports whether the filtered popover list is showing.
	Open bool
	// OnChange fires whenever Text changes (a keystroke edit or a selection).
	OnChange func(string)
	// OnSelect fires when an option is chosen (click or Enter).
	OnSelect func(string)
}

// NewComboBox builds a ComboBox with the given options and an empty field.
func NewComboBox(options []string) *ComboBox {
	return &ComboBox{Options: options}
}

// Filtered returns the Options whose lowercased text contains the lowercased
// Text. When Text is empty every option matches, so the full list is returned.
func (c *ComboBox) Filtered() []string {
	if c.Text == "" {
		return c.Options
	}
	needle := strings.ToLower(c.Text)
	var out []string
	for _, o := range c.Options {
		if strings.Contains(strings.ToLower(o), needle) {
			out = append(out, o)
		}
	}
	return out
}

// visible is the filtered list clamped to PopoverMaxRows — the rows the popover
// actually shows. Both PopoverBounds (for its height) and Draw (for the rows it
// paints) go through here so the clamp lives in exactly one place.
func (c *ComboBox) visible() []string {
	f := c.Filtered()
	if len(f) > PopoverMaxRows {
		f = f[:PopoverMaxRows]
	}
	return f
}

// comboRowH is the pixel height of one option row in the popover, matching the
// 18px row used by DropDown.PopoverBounds.
const comboRowH = 18

// PopoverBounds returns the Rect the filtered list occupies below the field:
// same X and W as the field, height proportional to the visible option count.
// Mirrors DropDown.PopoverBounds / DatePicker.PopoverBounds.
func (c *ComboBox) PopoverBounds() Rect {
	r := c.Bounds()
	return Rect{X: r.X, Y: r.Y + r.H, W: r.W, H: len(c.visible()) * comboRowH}
}

// Draw paints the field (rounded border, Text or muted Placeholder, an
// end-of-text caret, and a right-side chevron) and, when Open, the filtered
// options as a plain list in PopoverBounds.
func (c *ComboBox) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	border := theme.Border
	if c.Open {
		border = theme.Accent
	}
	fillRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, theme.Surface)
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, border)
	textY := r.Y + (r.H-c.glyphHeight())/2
	if c.Text == "" && c.Placeholder != "" {
		c.drawText(p, r.X+4, textY, c.Placeholder, theme.SurfaceAlt)
	} else {
		c.drawText(p, r.X+4, textY, c.Text, theme.OnSurface)
	}
	// Caret at the end of the typed text, measured through the effective font so
	// it lands correctly under a proportional / CJK face (mirrors Entry).
	cx := r.X + 4 + c.textWidth(c.Text)
	fillRect(p, cx, textY-1, 1, c.glyphHeight()+2, theme.OnSurface)
	// ▼ chevron on the right edge, drawn exactly like DropDown's.
	cvx := r.X + r.W - 10
	cvy := r.Y + r.H/2
	for t := 0; t < 4; t++ {
		fillRect(p, cvx-t, cvy+2-t, 1+2*t, 1, theme.OnSurface)
	}
	if c.Open {
		pb := c.PopoverBounds()
		fillRect(p, pb.X, pb.Y, pb.W, pb.H, theme.Surface)
		strokeRect(p, pb.X, pb.Y, pb.W, pb.H, theme.Border)
		for i, opt := range c.visible() {
			oy := pb.Y + i*comboRowH + (comboRowH-c.glyphHeight())/2
			c.drawText(p, pb.X+4, oy, opt, theme.OnSurface)
		}
	}
}

// OnEvent drives the type-to-filter behaviour: printable characters and
// Backspace edit Text (firing OnChange) and open the popover; a click on the
// field toggles Open; a click on a listed option selects it; Enter selects the
// first filtered option.
func (c *ComboBox) OnEvent(ev Event) {
	r := c.Bounds()
	switch ev.Kind {
	case EventClick:
		if c.Open {
			pb := c.PopoverBounds()
			lx := ev.X - (pb.X - r.X)
			ly := ev.Y - (pb.Y - r.Y)
			if lx >= 0 && lx < pb.W && ly >= 0 && ly < pb.H {
				// ly < pb.H == len(visible)*comboRowH guarantees the row index
				// is in range, so no extra bounds check is needed.
				c.selectOption(c.visible()[ly/comboRowH])
				return
			}
		}
		// A click anywhere else (the field itself) toggles the popover.
		c.Open = !c.Open
	case EventKeyDown:
		switch ev.Code {
		case "Backspace":
			runes := []rune(c.Text)
			if len(runes) > 0 {
				c.Text = string(runes[:len(runes)-1])
				c.Open = true
				if c.OnChange != nil {
					c.OnChange(c.Text)
				}
			}
		case "Enter":
			if f := c.Filtered(); len(f) > 0 {
				c.selectOption(f[0])
			}
		}
	case EventChar:
		if ev.Code == "" {
			return
		}
		c.Text += ev.Code
		c.Open = true
		if c.OnChange != nil {
			c.OnChange(c.Text)
		}
	}
}

// selectOption commits s as the field value, closes the popover, and fires
// OnSelect then OnChange (both guarded for nil).
func (c *ComboBox) selectOption(s string) {
	c.Text = s
	c.Open = false
	if c.OnSelect != nil {
		c.OnSelect(s)
	}
	if c.OnChange != nil {
		c.OnChange(s)
	}
}
