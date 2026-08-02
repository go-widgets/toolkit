// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// CycleButton is a button that steps through a fixed set of Options, showing the
// active one and advancing to the next on each click (wrapping past the end). It
// is the compact alternative to a radio group or dropdown when the choice set is
// small and cycling is natural (e.g. a view mode: List → Grid → Compact).
type CycleButton struct {
	Base
	Options  []string
	Index    int // index of the shown option; advanced on click
	OnChange func(index int, value string)
}

// NewCycleButton builds a CycleButton over options (the first shown).
func NewCycleButton(options ...string) *CycleButton { return &CycleButton{Options: options} }

// Value returns the currently shown option, or "" when there are none (or Index
// is out of range).
func (c *CycleButton) Value() string {
	if c.Index < 0 || c.Index >= len(c.Options) {
		return ""
	}
	return c.Options[c.Index]
}

// Draw paints the button body + the active option's label, centred, using the
// widget's font.
func (c *CycleButton) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	fillRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, theme.Surface)
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, theme.Border)
	if v := c.Value(); v != "" {
		tx := r.X + (r.W-c.textWidth(v))/2
		ty := r.Y + (r.H-c.glyphHeight())/2
		c.drawText(p, tx, ty, v, theme.OnSurface)
	}
}

// OnEvent advances to the next option on a click (wrapping), firing OnChange.
func (c *CycleButton) OnEvent(ev Event) {
	if ev.Kind == EventClick && len(c.Options) > 0 {
		c.Index = (c.Index + 1) % len(c.Options)
		if c.OnChange != nil {
			c.OnChange(c.Index, c.Options[c.Index])
		}
	}
}
