// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// DropDown is a one-of-N selector that shows the current choice in a
// button-like rectangle. Clicking opens a popover ListBox of all
// Options just below the widget; selecting one closes the popover +
// fires OnSelect.
//
// Like Dialog, the popover's rendering surface is owned by the host
// app; the toolkit exposes Open + Selected so the host knows what to
// draw. This keeps DropDown independent of how the compositor handles
// overlay surfaces (some apps use a separate canvas, some draw the
// popover directly into the same buffer).
type DropDown struct {
	Base
	Options  []string
	Selected int
	Open     bool
	OnSelect func(idx int)
}

// NewDropDown builds a DropDown with the given options + an initial
// selection (clamped to a valid index, or 0 when options is empty).
func NewDropDown(options []string, selected int) *DropDown {
	if selected < 0 || selected >= len(options) {
		selected = 0
	}
	return &DropDown{Options: options, Selected: selected}
}

// Current returns the currently-selected option's string, or "" when
// Options is empty.
func (d *DropDown) Current() string {
	if d.Selected < 0 || d.Selected >= len(d.Options) {
		return ""
	}
	return d.Options[d.Selected]
}

// Draw paints the closed widget. The popover, when Open, is the
// host app's responsibility (host can render a ListBox on top using
// PopoverBounds).
func (d *DropDown) Draw(p painter.Painter, theme *Theme) {
	r := d.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	textY := r.Y + (r.H-GlyphHeight)/2
	DrawText(p, r.X+6, textY, d.Current(), theme.OnSurface)
	// ▼ chevron on the right edge to signal a drop-down.
	cx := r.X + r.W - 10
	cy := r.Y + r.H/2
	for t := 0; t < 4; t++ {
		fillRect(p, cx-t, cy-1+t, 1+2*t, 1, theme.OnSurface)
	}
}

// OnEvent toggles Open on click. Selection happens via Select() which
// the host wires to its popover ListBox's OnActivate.
func (d *DropDown) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	d.Open = !d.Open
}

// Select picks idx, closes the popover + fires OnSelect.
func (d *DropDown) Select(idx int) {
	if idx < 0 || idx >= len(d.Options) {
		return
	}
	d.Selected = idx
	d.Open = false
	if d.OnSelect != nil {
		d.OnSelect(idx)
	}
}

// PopoverBounds returns the Rect the host should give to its popover
// ListBox: same X+W as the widget, positioned just below, height
// proportional to the option count (clamped to PopoverMaxRows rows).
func (d *DropDown) PopoverBounds() Rect {
	rows := len(d.Options)
	if rows > PopoverMaxRows {
		rows = PopoverMaxRows
	}
	r := d.Bounds()
	return Rect{X: r.X, Y: r.Y + r.H, W: r.W, H: rows * 18}
}

// PopoverMaxRows caps the dropdown popover height; longer option
// lists can wrap in a ScrollView the caller supplies.
const PopoverMaxRows = 12
