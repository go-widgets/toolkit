// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Date is a plain (Year, Month, Day) triple — the same time-source-free
// representation Calendar uses (see calendar.go, which takes year/month/day
// ints and never touches time.Time). The zero Date{} means "unset": a real
// selection always has Month in 1..12, so a Month of 0 is a reliable sentinel.
type Date struct {
	Y, M, D int
}

// cmp orders two dates lexicographically by (Y, M, D): -1 if a < b, 0 if
// equal, +1 if a > b.
func (a Date) cmp(b Date) int {
	if a.Y != b.Y {
		if a.Y < b.Y {
			return -1
		}
		return 1
	}
	if a.M != b.M {
		if a.M < b.M {
			return -1
		}
		return 1
	}
	if a.D != b.D {
		if a.D < b.D {
			return -1
		}
		return 1
	}
	return 0
}

// before reports whether a falls strictly before b.
func (a Date) before(b Date) bool { return a.cmp(b) < 0 }

// isZero reports whether the date is the unset sentinel (Month == 0).
func (a Date) isZero() bool { return a.M == 0 }

// DateRangePicker is a month grid on which the user clicks a start day then an
// end day; the inclusive range between the two is highlighted. Clicking again
// once a complete range exists begins a fresh selection.
//
// It composes Calendar for all of the month-grid layout + day hit-testing math
// (via the embedded Cal, whose OnSelect this widget wires to its own selection
// logic) and adds a header with prev/next-month arrows that Calendar lacks. The
// range fill uses the theme's SurfaceAlt tone; the two endpoints use Accent.
type DateRangePicker struct {
	Base
	Cal        *Calendar
	Start, End Date
	OnChange   func(start, end Date)
}

// NewDateRangePicker builds a picker displaying (year, month) with no initial
// selection. The caller positions it with SetBounds; the grid is 7 cells wide.
func NewDateRangePicker(year, month int) *DateRangePicker {
	rp := &DateRangePicker{Cal: NewCalendar(year, month, 1)}
	rp.Cal.OnSelect = func(y, m, d int) { rp.selectDay(y, m, d) }
	return rp
}

// selectDay advances the selection state machine for a clicked day. It is the
// Calendar's OnSelect callback, so the day arrives already hit-tested.
func (rp *DateRangePicker) selectDay(y, m, d int) {
	clicked := Date{Y: y, M: m, D: d}
	if rp.Start.isZero() || !rp.End.isZero() {
		// No start yet, or a complete range already exists: begin fresh.
		rp.Start = clicked
		rp.End = Date{}
		return
	}
	// Start set, end not yet chosen.
	if clicked.before(rp.Start) {
		// An earlier day resets the start; the end stays cleared.
		rp.Start = clicked
		return
	}
	// clicked >= start: complete the range and fire OnChange.
	rp.End = clicked
	if rp.OnChange != nil {
		rp.OnChange(rp.Start, rp.End)
	}
}

// prevMonth moves the displayed grid back one month (wrapping the year).
func (rp *DateRangePicker) prevMonth() {
	rp.Cal.Month--
	if rp.Cal.Month < 1 {
		rp.Cal.Month = 12
		rp.Cal.Year--
	}
}

// nextMonth moves the displayed grid forward one month (wrapping the year).
func (rp *DateRangePicker) nextMonth() {
	rp.Cal.Month++
	if rp.Cal.Month > 12 {
		rp.Cal.Month = 1
		rp.Cal.Year++
	}
}

// isEndpoint reports whether day is the selected start or end.
func (rp *DateRangePicker) isEndpoint(day Date) bool {
	if !rp.Start.isZero() && day.cmp(rp.Start) == 0 {
		return true
	}
	return !rp.End.isZero() && day.cmp(rp.End) == 0
}

// inRange reports whether day lies strictly between start and end (exclusive of
// both endpoints), which requires a complete selection.
func (rp *DateRangePicker) inRange(day Date) bool {
	if rp.Start.isZero() || rp.End.isZero() {
		return false
	}
	return rp.Start.before(day) && day.before(rp.End)
}

// Draw paints the header (prev arrow, month/year, next arrow), the weekday row,
// and the day grid with range highlighting.
func (rp *DateRangePicker) Draw(p painter.Painter, theme *Theme) {
	r := rp.Bounds()
	y, m := rp.Cal.Year, rp.Cal.Month
	// Calendar cell/header sizes belong to Calendar (calendar.go); routed through
	// scaled so this month grid grows with HiDPI and touch Density. Identity at
	// compact/1x, so the grid is byte-identical to before.
	hdrH, cellW, cellH := scaled(CalendarHeaderH), scaled(CalendarCellW), scaled(CalendarCellH)
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	// Header: prev arrow, centred month/year, next arrow.
	hy := r.Y + (hdrH-rp.glyphHeight())/2
	hdr := monthName(m) + " " + itoa(y)
	rp.drawText(p, r.X+(r.W-rp.textWidth(hdr))/2, hy, hdr, theme.OnSurface)
	rp.drawText(p, r.X+(cellW-rp.textWidth("<"))/2, hy, "<", theme.OnSurface)
	rp.drawText(p, r.X+r.W-cellW+(cellW-rp.textWidth(">"))/2, hy, ">", theme.OnSurface)
	// Weekday row.
	weekdayY := r.Y + hdrH
	for i, label := range weekdayLabels {
		cx := r.X + i*cellW + (cellW-rp.textWidth(label))/2
		rp.drawText(p, cx, weekdayY+scaled(drpWeekdayGap), label, theme.OnSurface)
	}
	// Day grid.
	first := WeekdayOfFirst(y, m)
	dim := DaysInMonth(y, m)
	gridY := weekdayY + rp.glyphHeight() + scaled(drpGridGap)
	for d := 1; d <= dim; d++ {
		idx := first + d - 1
		col := idx % 7
		row := idx / 7
		cx := r.X + col*cellW
		cy := gridY + row*cellH
		day := Date{Y: y, M: m, D: d}
		bg := theme.Surface
		ink := theme.OnSurface
		if rp.isEndpoint(day) {
			bg = theme.Accent
			ink = theme.Background
		} else if rp.inRange(day) {
			bg = theme.SurfaceAlt
		}
		fillRect(p, cx, cy, cellW, cellH, bg)
		txt := itoa(d)
		rp.drawText(p, cx+(cellW-rp.textWidth(txt))/2, cy+(cellH-rp.glyphHeight())/2, txt, ink)
	}
}

// drpWeekdayGap and drpGridGap are the LOGICAL vertical gaps below the weekday
// row and above the day grid, scaled at use; identity at compact/1x.
const (
	drpWeekdayGap = 2
	drpGridGap    = 4
)

// prevArrowRect and nextArrowRect are the drawn header arrow cells (one
// CalendarCellW wide at each end of the header row), the two discrete buttons a
// finger presses to page the month.
func (rp *DateRangePicker) prevArrowRect() Rect {
	r := rp.Bounds()
	return Rect{X: r.X, Y: r.Y, W: scaled(CalendarCellW), H: scaled(CalendarHeaderH)}
}

func (rp *DateRangePicker) nextArrowRect() Rect {
	r := rp.Bounds()
	cellW := scaled(CalendarCellW)
	return Rect{X: r.X + r.W - cellW, Y: r.Y, W: cellW, H: scaled(CalendarHeaderH)}
}

// PrevArrowHitRect and NextArrowHitRect are the finger targets for the two
// month-paging arrows: each drawn arrow cell clamped up to the touch minimum on
// both axes and centred over it, so the 24-logical-pixel cells reach the 44px
// floor under [DensityTouch]. They sit at opposite header ends, so the enlarged
// grabs never overlap. At [DensityCompact] each equals its drawn cell
// byte-for-byte.
func (rp *DateRangePicker) PrevArrowHitRect() Rect { return hitRectFor(rp.prevArrowRect()) }

// NextArrowHitRect is the finger target for the next-month arrow; see
// PrevArrowHitRect.
func (rp *DateRangePicker) NextArrowHitRect() Rect { return hitRectFor(rp.nextArrowRect()) }

// OnEvent handles clicks (widget-local coordinates): the header arrows page the
// month; a day-cell click is forwarded to the embedded Calendar, whose OnSelect
// drives selectDay.
func (rp *DateRangePicker) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := rp.Bounds()
	cellW := scaled(CalendarCellW)
	if ev.Y >= 0 && ev.Y < scaled(CalendarHeaderH) {
		if ev.X >= 0 && ev.X < cellW {
			rp.prevMonth()
			return
		}
		if ev.X >= r.W-cellW && ev.X < r.W {
			rp.nextMonth()
			return
		}
		return
	}
	rp.Cal.OnEvent(ev)
}
