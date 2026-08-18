// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// TimePicker is a stepper-based time-of-day picker — the pixel sibling of the
// calendar-oriented DatePicker. It shows two spinners laid left-to-right, one
// for the hour and one for the minute, each with a ▲ (up) and ▼ (down)
// affordance the user clicks to increment / decrement. A ":" separates them.
// When Use12h is set an extra AM/PM toggle cell is drawn on the right and the
// hour reads as a 12-hour clock, while the stored Hour stays 0..23.
//
// The widget never reads the wall clock: the initial hour+minute are supplied
// by the caller (NewTimePicker), so it stays deterministic and host-agnostic.
//
// Minute wrap policy: StepMinute wraps the minute within 0..59 WITHOUT
// carrying into the hour — stepping past :59 rolls back to the low end of the
// same hour. This keeps each spinner independent (the hour only ever changes
// via the hour spinner or the AM/PM toggle), matching how a native two-field
// time stepper behaves.
type TimePicker struct {
	Base
	Hour       int  // 0..23, always stored as 24-hour
	Minute     int  // 0..59
	MinuteStep int  // increment per minute step (default 1; e.g. 5 or 15)
	Use12h     bool // display as 12-hour with an AM/PM segment
	OnChange   func(hour, minute int)
}

// TimePicker layout constants. Cells are laid out left-to-right from the
// widget's top-left; scaled(tpBtnW) is the width of the up/down affordance column
// stacked on the right of each spinner cell.
const (
	tpCellW    = 34 // width of an hour / minute spinner cell
	tpColonW   = 8  // width of the ":" separator cell
	tpAmPmW    = 34 // width of the AM/PM toggle cell (only when Use12h)
	tpBtnW     = 14 // width of the ▲/▼ button column inside a spinner cell
	tpTextPadX = 4  // left inset for a spinner cell's value text
)

// HitRect is the TimePicker's field-level tap target: Bounds clamped up to the
// touch minimum on each axis and centred, byte-identical to Bounds at
// [DensityCompact]. The two ▲/▼ steppers are stacked within each spinner cell,
// so they cannot each grow to the 44px floor without overlapping; the field
// clamp guarantees the control as a whole meets the touch height, while the
// cells (scaled(tpCellW) wide) grow with density so the steppers stay legible.
func (tp *TimePicker) HitRect() Rect { return hitRectFor(tp.Bounds()) }

// NewTimePicker builds a TimePicker initialised to (hour, minute). The inputs
// are normalised into range (hour into 0..23, minute into 0..59) so an
// out-of-range caller value can never desync the display, and MinuteStep
// defaults to 1.
func NewTimePicker(hour, minute int) *TimePicker {
	tp := &TimePicker{MinuteStep: 1}
	tp.Hour = ((hour % 24) + 24) % 24
	tp.Minute = ((minute % 60) + 60) % 60
	return tp
}

// String is the formatted time: "15:04" (24-hour, zero-padded) by default, or
// "3:04 PM" when Use12h. Midnight (0) and noon (12) both render as 12 in
// 12-hour form, as AM and PM respectively.
func (tp *TimePicker) String() string {
	if tp.Use12h {
		return itoa(tp.hour12()) + ":" + pad(tp.Minute, 2) + " " + tp.meridiem()
	}
	return pad(tp.Hour, 2) + ":" + pad(tp.Minute, 2)
}

// hour12 maps the stored 24-hour Hour onto a 1..12 clock face.
func (tp *TimePicker) hour12() int {
	h := tp.Hour % 12
	if h == 0 {
		h = 12
	}
	return h
}

// meridiem is "AM" for hours [0,12) and "PM" for [12,24).
func (tp *TimePicker) meridiem() string {
	if tp.Hour >= 12 {
		return "PM"
	}
	return "AM"
}

// StepHour adjusts the hour by delta (typically +1 / -1), wrapping 0..23 in
// both directions, then fires OnChange.
func (tp *TimePicker) StepHour(delta int) {
	tp.Hour = ((tp.Hour+delta)%24 + 24) % 24
	tp.fire()
}

// StepMinute adjusts the minute by delta*MinuteStep (delta is a direction,
// +1 / -1), wrapping within 0..59 without carrying into the hour, then fires
// OnChange. A non-positive MinuteStep is treated as 1 so a click is never a
// silent no-op.
func (tp *TimePicker) StepMinute(delta int) {
	step := tp.MinuteStep
	if step <= 0 {
		step = 1
	}
	tp.Minute = ((tp.Minute+delta*step)%60 + 60) % 60
	tp.fire()
}

// ToggleAmPm flips between AM and PM by shifting Hour ±12, keeping the stored
// value in 0..23, then fires OnChange.
func (tp *TimePicker) ToggleAmPm() {
	if tp.Hour < 12 {
		tp.Hour += 12
	} else {
		tp.Hour -= 12
	}
	tp.fire()
}

// fire notifies OnChange if one is set.
func (tp *TimePicker) fire() {
	if tp.OnChange != nil {
		tp.OnChange(tp.Hour, tp.Minute)
	}
}

// hourText is the string shown in the hour cell: the 12-hour face when Use12h,
// else the zero-padded 24-hour value.
func (tp *TimePicker) hourText() string {
	if tp.Use12h {
		return itoa(tp.hour12())
	}
	return pad(tp.Hour, 2)
}

// layout resolves the absolute Rects of the cells, laid out left-to-right from
// the widget's bounds. ampmCell is the zero Rect (which Contains() never
// matches) unless Use12h is set, so hit-testing needs no separate guard.
func (tp *TimePicker) layout() (hourCell, colon, minCell, ampmCell Rect) {
	r := tp.Bounds()
	x := r.X
	hourCell = Rect{X: x, Y: r.Y, W: scaled(tpCellW), H: r.H}
	x += scaled(tpCellW)
	colon = Rect{X: x, Y: r.Y, W: scaled(tpColonW), H: r.H}
	x += scaled(tpColonW)
	minCell = Rect{X: x, Y: r.Y, W: scaled(tpCellW), H: r.H}
	x += scaled(tpCellW)
	if tp.Use12h {
		ampmCell = Rect{X: x, Y: r.Y, W: scaled(tpAmPmW), H: r.H}
	}
	return
}

// spinButtons splits a spinner cell into its ▲ (up, top) and ▼ (down, bottom)
// button rectangles — the column of width scaled(tpBtnW) on the cell's right edge.
func spinButtons(cell Rect) (up, down Rect) {
	bx := cell.X + cell.W - scaled(tpBtnW)
	half := cell.H / 2
	up = Rect{X: bx, Y: cell.Y, W: scaled(tpBtnW), H: half}
	down = Rect{X: bx, Y: cell.Y + half, W: scaled(tpBtnW), H: cell.H - half}
	return
}

// Draw paints the frame, the two spinner cells (value text + ▲/▼ buttons),
// the ":" separator and, when Use12h, the AM/PM toggle cell.
func (tp *TimePicker) Draw(p painter.Painter, theme *Theme) {
	r := tp.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	hourCell, colon, minCell, ampmCell := tp.layout()
	tp.drawSpinner(p, theme, hourCell, tp.hourText())
	tp.drawCentredText(p, colon, ":", theme.OnSurface)
	tp.drawSpinner(p, theme, minCell, pad(tp.Minute, 2))
	if tp.Use12h {
		fillRect(p, ampmCell.X, ampmCell.Y, ampmCell.W, ampmCell.H, theme.SurfaceAlt)
		strokeRect(p, ampmCell.X, ampmCell.Y, ampmCell.W, ampmCell.H, theme.Border)
		tp.drawCentredText(p, ampmCell, tp.meridiem(), theme.OnSurface)
	}
}

// drawSpinner paints one spinner cell: its body + value text on the left and a
// stacked ▲/▼ button column on the right.
func (tp *TimePicker) drawSpinner(p painter.Painter, theme *Theme, cell Rect, text string) {
	fillRect(p, cell.X, cell.Y, cell.W, cell.H, theme.Surface)
	strokeRect(p, cell.X, cell.Y, cell.W, cell.H, theme.Border)
	ty := cell.Y + (cell.H-tp.glyphHeight())/2
	tp.drawText(p, cell.X+scaled(tpTextPadX), ty, text, theme.OnSurface)
	up, down := spinButtons(cell)
	fillRect(p, up.X, up.Y, up.W, up.H, theme.SurfaceAlt)
	fillRect(p, down.X, down.Y, down.W, down.H, theme.SurfaceAlt)
	strokeRect(p, up.X, up.Y, up.W, up.H, theme.Border)
	strokeRect(p, down.X, down.Y, down.W, down.H, theme.Border)
	drawUpTriangle(p, up.X+up.W/2, up.Y+up.H/2, theme.OnSurface)
	drawDownTriangle(p, down.X+down.W/2, down.Y+down.H/2, theme.OnSurface)
}

// drawCentredText draws s horizontally + vertically centred within cell.
func (tp *TimePicker) drawCentredText(p painter.Painter, cell Rect, s string, ink RGBA) {
	tx := cell.X + (cell.W-tp.textWidth(s))/2
	ty := cell.Y + (cell.H-tp.glyphHeight())/2
	tp.drawText(p, tx, ty, s, ink)
}

// drawUpTriangle paints a solid ▲ centred on (cx, cy): a 1-px tip on top
// widening to a 9-px base — the same stacked-fillRect technique the spinner /
// chevron helpers use, so it needs no trig or anti-aliasing.
func drawUpTriangle(p painter.Painter, cx, cy int, ink RGBA) {
	for t := 0; t < 5; t++ {
		fillRect(p, cx-t, cy-2+t, 1+2*t, 1, ink)
	}
}

// drawDownTriangle paints a solid ▼ centred on (cx, cy): a 9-px base on top
// narrowing to a 1-px tip at the bottom.
func drawDownTriangle(p painter.Painter, cx, cy int, ink RGBA) {
	for t := 0; t < 5; t++ {
		fillRect(p, cx-t, cy+2-t, 1+2*t, 1, ink)
	}
}

// OnEvent handles EventClick: a click on an ▲/▼ button steps the matching
// field, a click on the AM/PM cell toggles the meridiem, and a click anywhere
// else (the value text, the ":" separator, outside a button) does nothing. The
// hit regions are computed from the same layout() Draw uses, so clicks always
// line up with what is painted.
func (tp *TimePicker) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := tp.Bounds()
	sx, sy := ev.X+r.X, ev.Y+r.Y // event is widget-local; layout is absolute
	hourCell, _, minCell, ampmCell := tp.layout()
	hUp, hDown := spinButtons(hourCell)
	mUp, mDown := spinButtons(minCell)
	switch {
	case hUp.Contains(sx, sy):
		tp.StepHour(1)
	case hDown.Contains(sx, sy):
		tp.StepHour(-1)
	case mUp.Contains(sx, sy):
		tp.StepMinute(1)
	case mDown.Contains(sx, sy):
		tp.StepMinute(-1)
	case ampmCell.Contains(sx, sy):
		tp.ToggleAmPm()
	}
}
