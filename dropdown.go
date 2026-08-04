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
	focusState
	Options  []string
	Selected int
	Open     bool
	// OpenUp makes the popover appear ABOVE the control instead of below it —
	// set it when the control sits near the bottom edge so the list has room.
	OpenUp   bool
	OnSelect func(idx int)

	// savedSelected remembers Selected at the moment the popover opened via the
	// keyboard, so Escape can restore it after the arrow keys previewed other
	// options without committing.
	savedSelected int
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
	// A disabled dropdown mutes its face, border, text and chevron; only taken
	// when Disabled so the enabled draw is byte-identical.
	face, border, ink := theme.Surface, theme.Border, theme.OnSurface
	if d.Disabled {
		face, border, ink = mutedFace(theme), mutedInk(theme), mutedInk(theme)
	}
	fillRect(p, r.X, r.Y, r.W, r.H, face)
	strokeRect(p, r.X, r.Y, r.W, r.H, border)
	textY := r.Y + (r.H-d.glyphHeight())/2
	d.drawText(p, r.X+6, textY, d.Current(), ink)
	// ▼ chevron on the right edge to signal a drop-down. The wide base
	// sits on the top row and rows narrow moving down to the point,
	// so at t=0 the 1-pixel-wide tip lands at cy+2 and at t=3 the 7-
	// pixel-wide base lands at cy-1.
	cx := r.X + r.W - 10
	cy := r.Y + r.H/2
	for t := 0; t < 4; t++ {
		fillRect(p, cx-t, cy+2-t, 1+2*t, 1, ink)
	}
	d.drawFocusRing(p, theme, r)
}

// OnEvent toggles Open on click. Selection happens via Select() which
// the host wires to its popover ListBox's OnActivate. A Disabled dropdown
// ignores every kind (it cannot be opened).
func (d *DropDown) OnEvent(ev Event) {
	if d.Disabled {
		return
	}
	switch ev.Kind {
	case EventClick:
		d.Open = !d.Open
	case EventKeyDown:
		d.onKey(ev.Code)
	}
}

// onKey drives the dropdown from the keyboard while focused:
//   - closed: Space or ArrowDown opens the popover, remembering Selected so
//     Escape can restore it.
//   - open: ArrowDown/ArrowUp move Selected through the options (clamped, no
//     commit yet); Enter commits the highlighted option (reusing Select, which
//     closes + fires OnSelect); Escape restores the pre-open Selected + closes.
func (d *DropDown) onKey(code string) {
	if !d.Open {
		switch code {
		case " ", "Space", "ArrowDown":
			d.savedSelected = d.Selected
			d.Open = true
		}
		return
	}
	switch code {
	case "ArrowDown":
		if d.Selected < len(d.Options)-1 {
			d.Selected++
		}
	case "ArrowUp":
		if d.Selected > 0 {
			d.Selected--
		}
	case "Enter":
		d.Select(d.Selected)
	case "Escape":
		d.Selected = d.savedSelected
		d.Open = false
	}
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
// ListBox: same X+W as the widget, height proportional to the option
// count (clamped to PopoverMaxRows rows). Positioned just below the
// widget, or — when OpenUp is set — just above it so a control near the
// bottom edge still has room for its list.
func (d *DropDown) PopoverBounds() Rect {
	rows := len(d.Options)
	if rows > PopoverMaxRows {
		rows = PopoverMaxRows
	}
	r := d.Bounds()
	h := rows * PopoverRowH
	y := r.Y + r.H
	if d.OpenUp {
		y = r.Y - h
	}
	return Rect{X: r.X, Y: y, W: r.W, H: h}
}

// PopoverMaxRows caps the dropdown popover height; longer option
// lists can wrap in a ScrollView the caller supplies.
const PopoverMaxRows = 12

// PopoverRowH is the pixel height of one option row in the popover.
const PopoverRowH = 18

// DrawPopover paints the open options list at PopoverBounds, with the current
// selection highlighted. A no-op when the DropDown is closed. The host calls it
// in its overlay pass (after the rest of the scene) so the popover — which
// extends past the control's Bounds — sits on top; that z-ordering is the one
// thing the widget can't decide for itself.
func (d *DropDown) DrawPopover(p painter.Painter, theme *Theme) {
	if !d.Open {
		return
	}
	lb := NewListBox(d.Options)
	lb.Selected = d.Selected
	lb.SetBounds(d.PopoverBounds())
	lb.Draw(p, theme)
}

// PopoverClick routes a click at (x, y) — in the DropDown's own coordinate
// frame, the same one Bounds/PopoverBounds use — while the popover is open: a
// click inside it selects that option (firing OnSelect and closing), a click
// anywhere else just closes it. Returns true when the open popover consumed the
// click, false when the DropDown is closed (so the host falls through to its
// normal hit-testing, where a click on the control reopens it).
func (d *DropDown) PopoverClick(x, y int) bool {
	if !d.Open {
		return false
	}
	if pb := d.PopoverBounds(); x >= pb.X && x < pb.X+pb.W && y >= pb.Y && y < pb.Y+pb.H {
		d.Select((y - pb.Y) / PopoverRowH)
	} else {
		d.Open = false
	}
	return true
}
