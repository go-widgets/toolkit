// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Calendar renders a month grid (Mon..Sun columns, up to 6 rows) for
// a given (Year, Month). The currently-selected day is highlighted;
// clicking a day-cell selects it by Setting the [Calendar.Day]
// Observable (and [Calendar.Year] / [Calendar.Month] when the click lands
// in an adjacent month), notifying subscribers.
//
// Calendar takes no time-source dep; the host must pass it the
// current year/month/day. A "today" pill can be drawn by setting
// Today (year/month/day); set to (0, 0, 0) to disable it.
//
// The reactive state is MVVM-only: the viewed year / month and the selected
// day live in unexported Observables exposed via [Calendar.Year],
// [Calendar.Month] and [Calendar.Day]. A host binds them (Set / Subscribe /
// two-way); there are no settable Year/Month/Day fields.
//
// The header carries prev/next arrows ("<" / ">"): clicking them steps
// the viewed month (wrapping the year at the Dec/Jan boundary) by Setting
// the Year / Month Observables. PrevMonth / NextMonth expose the same
// navigation programmatically.
type Calendar struct {
	Base
	focusState
	// TodayY / TodayM / TodayD are the set-once "today" pill the calendar
	// highlights regardless of the viewed (Y/M); (0, 0, 0) disables it. They are
	// appearance config, not reactive state.
	TodayY int
	TodayM int
	TodayD int

	year  *mvvm.Observable[int]
	month *mvvm.Observable[int]
	day   *mvvm.Observable[int]
}

// Year is the viewed year as a shared [mvvm.Observable]: a host binds it
// (Set / Subscribe / two-way). There is no settable Year field; prev/next
// navigation and clicks in an adjacent month Set it.
func (c *Calendar) Year() *mvvm.Observable[int] {
	if c.year == nil {
		c.year = mvvm.NewObservable(0)
	}
	return c.year
}

// Month is the viewed month (1..12) as a shared [mvvm.Observable]: a host binds
// it. There is no settable Month field; prev/next navigation Sets it.
func (c *Calendar) Month() *mvvm.Observable[int] {
	if c.month == nil {
		c.month = mvvm.NewObservable(0)
	}
	return c.month
}

// Day is the selected day (in [1, daysInMonth]) as a shared [mvvm.Observable]:
// a host binds it (Set / Subscribe / two-way). There is no settable Day field;
// a day-click or a keyboard move Sets it, notifying subscribers.
func (c *Calendar) Day() *mvvm.Observable[int] {
	if c.day == nil {
		c.day = mvvm.NewObservable(0)
	}
	return c.day
}

// Sizing.
const (
	CalendarHeaderH = 22
	CalendarCellW   = 24
	CalendarCellH   = 18
	// CalendarNavW is the width of each header prev/next arrow hit-zone, at the
	// left ("<") and right (">") ends of the header row.
	CalendarNavW = 20
)

// NewCalendar builds a Calendar for the given (year, month, day).
func NewCalendar(year, month, day int) *Calendar {
	c := &Calendar{}
	c.year = mvvm.NewObservable(year)
	c.month = mvvm.NewObservable(month)
	c.day = mvvm.NewObservable(day)
	c.clamp()
	return c
}

// SetDate moves the calendar to (year, month, day), Setting the Year / Month /
// Day Observables (notifying subscribers) and re-clamping into legal ranges.
func (c *Calendar) SetDate(year, month, day int) {
	c.Year().Set(year)
	c.Month().Set(month)
	c.Day().Set(day)
	c.clamp()
}

// SetToday records the "today" pill the calendar should highlight
// regardless of which (Y/M) is being viewed.
func (c *Calendar) SetToday(y, m, d int) {
	c.TodayY = y
	c.TodayM = m
	c.TodayD = d
}

// NextMonth advances the view one month, wrapping December to the next
// January (Setting the Year / Month Observables) and re-clamps the selected day
// into the new month. Subscribers are notified through the Observables.
func (c *Calendar) NextMonth() {
	y, m := c.Year().Get(), c.Month().Get()+1
	if m > 12 {
		m = 1
		y++
	}
	c.Year().Set(y)
	c.Month().Set(m)
	c.clamp()
}

// PrevMonth steps the view one month back, wrapping January to the previous
// December (Setting the Year / Month Observables) and re-clamps the selected
// day into the new month. Subscribers are notified through the Observables.
func (c *Calendar) PrevMonth() {
	y, m := c.Year().Get(), c.Month().Get()-1
	if m < 1 {
		m = 12
		y--
	}
	c.Year().Set(y)
	c.Month().Set(m)
	c.clamp()
}

// clamp keeps the month + day in legal ranges so a malformed payload
// can't break the layout; it Sets the Month / Day Observables when a value
// falls outside its bounds (an unchanged value is a no-op).
func (c *Calendar) clamp() {
	m := c.Month().Get()
	if m < 1 {
		m = 1
	} else if m > 12 {
		m = 12
	}
	c.Month().Set(m)
	dim := DaysInMonth(c.Year().Get(), m)
	d := c.Day().Get()
	if d < 1 {
		d = 1
	}
	if d > dim {
		d = dim
	}
	c.Day().Set(d)
}

// DaysInMonth returns the day count for (year, month).
func DaysInMonth(year, month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if isLeap(year) {
			return 29
		}
		return 28
	default:
		return 30
	}
}

func isLeap(y int) bool {
	if y%400 == 0 {
		return true
	}
	if y%100 == 0 {
		return false
	}
	return y%4 == 0
}

// WeekdayOfFirst returns the weekday-index (0=Mon..6=Sun) of the
// first day of (year, month). Uses Zeller-ish congruence so we don't
// depend on time.Time.
func WeekdayOfFirst(year, month int) int {
	y := year
	m := month
	if m < 3 {
		m += 12
		y--
	}
	K := y % 100
	J := y / 100
	h := (1 + (13*(m+1))/5 + K + K/4 + J/4 + 5*J) % 7
	// Zeller: 0=Sat..6=Fri; remap to 0=Mon..6=Sun.
	switch h {
	case 0:
		return 5 // Sat
	case 1:
		return 6 // Sun
	default:
		return h - 2
	}
}

// Draw paints header (Y M) + weekday row + day grid.
func (c *Calendar) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	// Header: month / year, centred, flanked by prev/next arrows.
	year, month, selDay := c.Year().Get(), c.Month().Get(), c.Day().Get()
	hdr := monthName(month) + " " + itoa(year)
	hx := r.X + (r.W-c.textWidth(hdr))/2
	hy := r.Y + (scaled(CalendarHeaderH)-c.glyphHeight())/2
	c.drawText(p, hx, hy, hdr, theme.OnSurface)
	c.drawText(p, r.X+(scaled(CalendarNavW)-c.textWidth("<"))/2, hy, "<", theme.OnSurface)
	c.drawText(p, r.X+r.W-scaled(CalendarNavW)+(scaled(CalendarNavW)-c.textWidth(">"))/2, hy, ">", theme.OnSurface)
	// Weekday row.
	weekdayY := r.Y + scaled(CalendarHeaderH)
	for i, label := range weekdayLabels {
		cx := r.X + i*scaled(CalendarCellW) + (scaled(CalendarCellW)-c.textWidth(label))/2
		c.drawText(p, cx, weekdayY+2, label, theme.OnSurface)
	}
	// Day grid.
	first := WeekdayOfFirst(year, month)
	dim := DaysInMonth(year, month)
	gridY := weekdayY + c.glyphHeight() + 4
	for d := 1; d <= dim; d++ {
		idx := first + d - 1
		col := idx % 7
		row := idx / 7
		cx := r.X + col*scaled(CalendarCellW)
		cy := gridY + row*scaled(CalendarCellH)
		bg := theme.Surface
		ink := theme.OnSurface
		isToday := (c.TodayY == year && c.TodayM == month && c.TodayD == d)
		if d == selDay {
			bg = theme.Accent
			ink = theme.Background
		} else if isToday {
			bg = theme.SurfaceAlt
		}
		fillRect(p, cx, cy, scaled(CalendarCellW), scaled(CalendarCellH), bg)
		txt := itoa(d)
		c.drawText(p, cx+(scaled(CalendarCellW)-c.textWidth(txt))/2, cy+(scaled(CalendarCellH)-c.glyphHeight())/2, txt, ink)
	}
	// Border LAST, so day-cell fills (which start at r.X for the first column
	// and can reach the right/bottom edges) never erase the frame — the
	// previous order stroked the border first and the col-0 cells painted over
	// the left border in the grid rows.
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	c.drawFocusRing(p, theme, r)
}

// OnEvent dispatches a header-arrow click to Prev/NextMonth and a day-cell
// click to the Day Observable (Setting the selected day).
func (c *Calendar) OnEvent(ev Event) {
	if ev.Kind == EventKeyDown {
		c.onKey(ev.Code)
		return
	}
	if ev.Kind != EventClick {
		return
	}
	// Header row: prev/next month arrows.
	if ev.Y < scaled(CalendarHeaderH) {
		w := c.Bounds().W
		if ev.X < scaled(CalendarNavW) {
			c.PrevMonth()
		} else if ev.X >= w-scaled(CalendarNavW) {
			c.NextMonth()
		}
		return
	}
	gridY := scaled(CalendarHeaderH) + c.glyphHeight() + 4
	if ev.Y < gridY {
		return
	}
	col := ev.X / scaled(CalendarCellW)
	if col < 0 || col > 6 {
		return
	}
	row := (ev.Y - gridY) / scaled(CalendarCellH)
	first := WeekdayOfFirst(c.Year().Get(), c.Month().Get())
	idx := row*7 + col
	if idx < first {
		return
	}
	d := idx - first + 1
	dim := DaysInMonth(c.Year().Get(), c.Month().Get())
	if d < 1 || d > dim {
		return
	}
	c.Day().Set(d)
}

// onKey drives the calendar from the keyboard while focused. Left/Right move the
// selected day by one, Up/Down by a week; Home/End jump to the first/last day of
// the month; PageUp/PageDown step the viewed month (reusing Prev/NextMonth). In
// the MVVM model the selection IS the [Calendar.Day] Observable, so every move
// Sets it and notifies subscribers -- there is no separate Enter/Space commit.
// A disabled calendar ignores every key.
func (c *Calendar) onKey(code string) {
	if c.Disabled {
		return
	}
	switch code {
	case "ArrowLeft":
		c.moveDay(-1)
	case "ArrowRight":
		c.moveDay(+1)
	case "ArrowUp":
		c.moveDay(-7)
	case "ArrowDown":
		c.moveDay(+7)
	case "Home":
		c.moveDay(-c.Day().Get() + 1) // first of month
	case "End":
		c.moveDay(DaysInMonth(c.Year().Get(), c.Month().Get()) - c.Day().Get()) // last of month
	case "PageUp":
		c.PrevMonth()
	case "PageDown":
		c.NextMonth()
	}
}

// moveDay shifts the selected day by delta, clamped to [1, days-in-month], and
// Sets the Day Observable (notifying subscribers on change).
func (c *Calendar) moveDay(delta int) {
	d := c.Day().Get() + delta
	if d < 1 {
		d = 1
	}
	if dim := DaysInMonth(c.Year().Get(), c.Month().Get()); d > dim {
		d = dim
	}
	c.Day().Set(d)
}

var weekdayLabels = [7]string{"M", "T", "W", "T", "F", "S", "S"}

func monthName(m int) string {
	switch m {
	case 1:
		return "Jan"
	case 2:
		return "Feb"
	case 3:
		return "Mar"
	case 4:
		return "Apr"
	case 5:
		return "May"
	case 6:
		return "Jun"
	case 7:
		return "Jul"
	case 8:
		return "Aug"
	case 9:
		return "Sep"
	case 10:
		return "Oct"
	case 11:
		return "Nov"
	case 12:
		return "Dec"
	}
	return "???"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
