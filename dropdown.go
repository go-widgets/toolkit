// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// DropDown is a one-of-N selector that shows the current choice in a
// button-like rectangle. Clicking opens a popover ListBox of all
// Options just below the widget; selecting one closes the popover and
// Sets the Selected Observable.
//
// Like Dialog, the popover's rendering surface is owned by the host
// app; the toolkit exposes Open + Selected — both MVVM-only, as shared
// [mvvm.Observable] handles behind accessors — so the host knows what to
// draw and binds/Subscribes to the state. This keeps DropDown independent
// of how the compositor handles overlay surfaces (some apps use a separate
// canvas, some draw the popover directly into the same buffer).
type DropDown struct {
	Base
	focusState
	// Options is the set-once list of choices (config). OpenUp is a layout
	// config flag. The reactive state is MVVM-only: the current selection and
	// the open/closed flag live in unexported Observables exposed via
	// [DropDown.Selected] and [DropDown.Open].
	Options []string
	// OpenUp makes the popover appear ABOVE the control instead of below it —
	// set it when the control sits near the bottom edge so the list has room.
	OpenUp bool

	selected *mvvm.Observable[int]
	open     *mvvm.Observable[bool]

	// savedSelected remembers the selection at the moment the popover opened via
	// the keyboard, so Escape can restore it after the arrow keys previewed other
	// options without committing.
	savedSelected int

	// popScroll is the index of the option painted at the top of the open
	// popover — the persistent scroll offset that makes options beyond
	// PopoverMaxRows reachable. The popover ListBox DrawPopover builds already
	// scrolls natively; this offset feeds its ScrollRow, the wheel
	// (EventScroll) shifts it, arrow-key selection scrolls it to keep the
	// highlighted option visible, and PopoverClick maps a click's y through it.
	// Reads clamp on the fly (see clampedPopScroll), so a stale value after
	// Options shrank is harmless.
	popScroll int
}

// Selected is the chosen option index as a shared [mvvm.Observable]: a host
// binds it (Set / Subscribe / two-way) — there is no settable Selected field. A
// click, an Enter commit or an arrow-key preview Sets it; subscribers are
// notified on change.
func (d *DropDown) Selected() *mvvm.Observable[int] {
	if d.selected == nil {
		d.selected = mvvm.NewObservable(0)
	}
	return d.selected
}

// Open is the popover open/closed flag as a shared [mvvm.Observable]: a host
// binds it to know whether to render the popover — there is no settable Open
// field. A click toggles it; selecting an option or pressing Escape Sets it
// false. Subscribers are notified on change.
func (d *DropDown) Open() *mvvm.Observable[bool] {
	if d.open == nil {
		d.open = mvvm.NewObservable(false)
	}
	return d.open
}

// NewDropDown builds a DropDown with the given options + an initial
// selection (clamped to a valid index, or 0 when options is empty).
func NewDropDown(options []string, selected int) *DropDown {
	if selected < 0 || selected >= len(options) {
		selected = 0
	}
	d := &DropDown{Options: options}
	d.selected = mvvm.NewObservable(selected)
	d.open = mvvm.NewObservable(false)
	return d
}

// Current returns the currently-selected option's string, or "" when
// Options is empty.
func (d *DropDown) Current() string {
	sel := d.Selected().Get()
	if sel < 0 || sel >= len(d.Options) {
		return ""
	}
	return d.Options[sel]
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
	d.drawText(p, r.X+scaled(dropdownPadX), textY, d.Current(), ink)
	// ▼ chevron on the right edge to signal a drop-down. The wide base
	// sits on the top row and rows narrow moving down to the point,
	// so at t=0 the 1-pixel-wide tip lands at cy+2 and at t=3 the 7-
	// pixel-wide base lands at cy-1.
	cx := r.X + r.W - scaled(dropdownChevronInset)
	cy := r.Y + r.H/2
	for t := 0; t < 4; t++ {
		fillRect(p, cx-t, cy+2-t, 1+2*t, 1, ink)
	}
	d.drawFocusRing(p, theme, r)
}

// dropdownPadX is the field-text left inset and dropdownChevronInset the
// chevron's distance from the right edge — LOGICAL bases routed through scaled,
// identity at compact/1x.
const (
	dropdownPadX         = 6
	dropdownChevronInset = 10
)

// rowH is the device-pixel height of one popover option row: PopoverRowH routed
// through scaled, so PopoverBounds and PopoverClick agree on one scaled cell
// height. Identity at compact/1x.
func (d *DropDown) rowH() int { return scaled(PopoverRowH) }

// HitRect is the DropDown's tap/toggle target: Bounds clamped up to the touch
// minimum on each axis and centred. Byte-identical to Bounds at
// [DensityCompact].
func (d *DropDown) HitRect() Rect { return touchHitRect(d.Bounds()) }

// OnEvent toggles Open on click. Selection happens via Select() which
// the host wires to its popover ListBox's OnActivate. A Disabled dropdown
// ignores every kind (it cannot be opened).
func (d *DropDown) OnEvent(ev Event) {
	if d.Disabled {
		return
	}
	switch ev.Kind {
	case EventClick:
		open := !d.Open().Get()
		d.Open().Set(open)
		if open {
			d.scrollSelectedIntoView()
		}
	case EventScroll:
		// Wheel over the open popover shifts its scroll window so options
		// beyond PopoverMaxRows are reachable; ignored while closed.
		if d.Open().Get() {
			d.scrollPopover(ev.Delta)
		}
	case EventKeyDown:
		d.onKey(ev.Code)
	}
}

// onKey drives the dropdown from the keyboard while focused:
//   - closed: Space or ArrowDown opens the popover, remembering the selection so
//     Escape can restore it.
//   - open: ArrowDown/ArrowUp move the selection through the options (clamped, no
//     commit yet); Enter commits the highlighted option (reusing Select, which
//     closes + Sets Selected); Escape restores the pre-open selection + closes.
func (d *DropDown) onKey(code string) {
	if !d.Open().Get() {
		switch code {
		case " ", "Space", "ArrowDown":
			d.savedSelected = d.Selected().Get()
			d.Open().Set(true)
			d.scrollSelectedIntoView()
		}
		return
	}
	switch code {
	case "ArrowDown":
		if sel := d.Selected().Get(); sel < len(d.Options)-1 {
			d.Selected().Set(sel + 1)
		}
		d.scrollSelectedIntoView()
	case "ArrowUp":
		if sel := d.Selected().Get(); sel > 0 {
			d.Selected().Set(sel - 1)
		}
		d.scrollSelectedIntoView()
	case "Enter":
		d.Select(d.Selected().Get())
	case "Escape":
		d.Selected().Set(d.savedSelected)
		d.Open().Set(false)
	}
}

// Select picks idx, Sets the Selected Observable and closes the popover.
func (d *DropDown) Select(idx int) {
	if idx < 0 || idx >= len(d.Options) {
		return
	}
	d.Selected().Set(idx)
	d.Open().Set(false)
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
	h := rows * d.rowH()
	y := r.Y + r.H
	if d.OpenUp {
		y = r.Y - h
	}
	return Rect{X: r.X, Y: y, W: r.W, H: h}
}

// PopoverMaxRows caps the dropdown popover height; longer option
// lists are reachable by scrolling the popover (see popScroll).
const PopoverMaxRows = 12

// maxPopScroll is the highest popScroll that still fills the popover window:
// len(Options) - PopoverMaxRows, floored at 0 so a list that already fits never
// scrolls.
func (d *DropDown) maxPopScroll() int {
	m := len(d.Options) - PopoverMaxRows
	if m < 0 {
		m = 0
	}
	return m
}

// clampedPopScroll returns popScroll clamped to [0, maxPopScroll] WITHOUT
// mutating the field, so a value left stale after Options shrank never paints or
// hit-tests outside the valid window.
func (d *DropDown) clampedPopScroll() int {
	s := d.popScroll
	if s < 0 {
		s = 0
	}
	if m := d.maxPopScroll(); s > m {
		s = m
	}
	return s
}

// scrollPopover shifts popScroll by delta rows (negative scrolls up), clamped to
// [0, maxPopScroll] and written back immediately.
func (d *DropDown) scrollPopover(delta int) {
	d.popScroll += delta
	d.popScroll = d.clampedPopScroll()
}

// scrollSelectedIntoView nudges popScroll so the highlighted option (Selected)
// stays within the PopoverMaxRows-tall window: up if it sits above the window,
// down if at or past the last visible row. Keeps arrow-key selection following
// the highlight past the fold.
func (d *DropDown) scrollSelectedIntoView() {
	sel := d.Selected().Get()
	if sel < d.popScroll {
		d.popScroll = sel
	} else if sel >= d.popScroll+PopoverMaxRows {
		d.popScroll = sel - PopoverMaxRows + 1
	}
	d.popScroll = d.clampedPopScroll()
}

// PopoverRowH is the pixel height of one option row in the popover.
const PopoverRowH = 18

// DrawPopover paints the open options list at PopoverBounds, with the current
// selection highlighted. A no-op when the DropDown is closed. The host calls it
// in its overlay pass (after the rest of the scene) so the popover — which
// extends past the control's Bounds — sits on top; that z-ordering is the one
// thing the widget can't decide for itself.
func (d *DropDown) DrawPopover(p painter.Painter, theme *Theme) {
	if !d.Open().Get() {
		return
	}
	lb := NewListBox(d.Options)
	lb.Selected = d.Selected().Get()
	// Feed the persistent scroll offset into the ListBox's own native virtual
	// scrolling so options beyond PopoverMaxRows are painted (and a scrollbar
	// appears) when the popover is scrolled. At popScroll == 0 this is
	// byte-identical to an unscrolled list.
	lb.ScrollRow = d.clampedPopScroll()
	lb.SetBounds(d.PopoverBounds())
	lb.Draw(p, theme)
}

// PopoverClick routes a click at (x, y) — in the DropDown's own coordinate
// frame, the same one Bounds/PopoverBounds use — while the popover is open: a
// click inside it selects that option (Setting Selected and closing), a click
// anywhere else just closes it. Returns true when the open popover consumed the
// click, false when the DropDown is closed (so the host falls through to its
// normal hit-testing, where a click on the control reopens it).
func (d *DropDown) PopoverClick(x, y int) bool {
	if !d.Open().Get() {
		return false
	}
	if pb := d.PopoverBounds(); x >= pb.X && x < pb.X+pb.W && y >= pb.Y && y < pb.Y+pb.H {
		// Map the click's row within the visible window back to an absolute
		// option index through the scroll offset, so a click after scrolling
		// selects the right option. Select clamps an out-of-range index.
		d.Select(d.clampedPopScroll() + (y-pb.Y)/d.rowH())
	} else {
		d.Open().Set(false)
	}
	return true
}
