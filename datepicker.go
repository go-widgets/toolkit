// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// DatePicker is a form input for a single calendar date: a field showing the
// selected date as ISO YYYY-MM-DD text with a small grid icon, and a drop-down
// Calendar that opens beneath it when the field is clicked. Picking a day in
// the calendar updates the field, closes the popup, and fires OnChange.
//
// Where the display-only Calendar just renders a month, DatePicker is the
// composite entry control built around it — the pixel sibling of a native
// date field. It owns its Calendar (exposed as Cal) and renders the popup
// itself when Open, so it works standalone; a host that composites overlays on
// a separate surface can instead read Open + PopoverBounds and draw Cal there.
type DatePicker struct {
	Base
	Cal      *Calendar
	Open     bool
	OnChange func(y, m, d int)
}

// DatePickerFieldH is the pixel height of the closed field.
const DatePickerFieldH = GlyphHeight + 10

// NewDatePicker builds a DatePicker initialised to (year, month, day).
func NewDatePicker(year, month, day int) *DatePicker {
	dp := &DatePicker{Cal: NewCalendar(year, month, day)}
	dp.Cal.OnSelect = func(y, m, d int) {
		dp.Open = false
		if dp.OnChange != nil {
			dp.OnChange(y, m, d)
		}
	}
	return dp
}

// Date returns the currently-selected (year, month, day).
func (dp *DatePicker) Date() (y, m, d int) {
	return dp.Cal.Year, dp.Cal.Month, dp.Cal.Day
}

// SetDate moves the selection to (year, month, day) without opening the popup.
func (dp *DatePicker) SetDate(year, month, day int) { dp.Cal.SetDate(year, month, day) }

// Text is the field's displayed value: ISO 8601 YYYY-MM-DD.
func (dp *DatePicker) Text() string {
	return pad(dp.Cal.Year, 4) + "-" + pad(dp.Cal.Month, 2) + "-" + pad(dp.Cal.Day, 2)
}

// PopoverBounds is the Rect the Calendar occupies when Open: same X and full
// calendar width below the field. Six week-rows is the worst case.
func (dp *DatePicker) PopoverBounds() Rect {
	r := dp.Bounds()
	h := CalendarHeaderH + GlyphHeight + 4 + 6*CalendarCellH + 4
	return Rect{X: r.X, Y: r.Y + r.H, W: 7 * CalendarCellW, H: h}
}

// Draw paints the field (border + date text + a grid icon) and, when Open, the
// Calendar popup positioned by PopoverBounds.
func (dp *DatePicker) Draw(p painter.Painter, theme *Theme) {
	r := dp.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	textY := r.Y + (r.H-GlyphHeight)/2
	DrawText(p, r.X+6, textY, dp.Text(), theme.OnSurface)
	dp.drawIcon(p, r, theme)
	if dp.Open {
		dp.Cal.SetBounds(dp.PopoverBounds())
		dp.Cal.Draw(p, theme)
	}
}

// drawIcon paints a tiny 2x2 calendar-grid glyph at the field's right edge.
func (dp *DatePicker) drawIcon(p painter.Painter, r Rect, theme *Theme) {
	ix := r.X + r.W - 14
	iy := r.Y + (r.H-8)/2
	strokeRect(p, ix, iy, 10, 8, theme.OnSurface)
	fillRect(p, ix, iy, 10, 2, theme.Accent) // header band
	// two-column grid of day dots below the band
	for row := 0; row < 2; row++ {
		for col := 0; col < 2; col++ {
			fillRect(p, ix+2+col*4, iy+4+row*2, 2, 1, theme.OnSurface)
		}
	}
}

// OnEvent: a click on the field toggles the popup; while open, a click inside
// the popup is forwarded (translated to Calendar-local coordinates) to the
// Calendar, whose OnSelect closes the popup and fires OnChange.
func (dp *DatePicker) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := dp.Bounds()
	if dp.Open {
		pb := dp.PopoverBounds()
		// The event arrives widget-local (relative to the field's top-left);
		// re-base it into the Calendar's own coordinate frame.
		lx := ev.X - (pb.X - r.X)
		ly := ev.Y - (pb.Y - r.Y)
		if lx >= 0 && lx < pb.W && ly >= 0 && ly < pb.H {
			dp.Cal.SetBounds(pb)
			dp.Cal.OnEvent(Event{Kind: EventClick, X: lx, Y: ly})
			return
		}
	}
	// A click on the field itself toggles the popup.
	if ev.X >= 0 && ev.X < r.W && ev.Y >= 0 && ev.Y < r.H {
		dp.Open = !dp.Open
	}
}

// pad renders n as a zero-padded decimal at least width digits wide.
func pad(n, width int) string {
	s := itoa(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}
