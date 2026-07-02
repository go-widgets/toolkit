// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Calendar renders a month grid (Mon..Sun columns, up to 6 rows) for
// a given (Year, Month). The currently-selected day is highlighted;
// click on a day-cell selects it + fires OnSelect with the absolute
// (Y, M, D) triple.
//
// Calendar takes no time-source dep; the host must pass it the
// current year/month/day. A "today" pill can be drawn by setting
// Today (year/month/day); set to (0, 0, 0) to disable it.
type Calendar struct {
	Base
	Year     int
	Month    int // 1..12
	Day      int // selected day in [1, daysInMonth]
	TodayY   int
	TodayM   int
	TodayD   int
	OnSelect func(y, m, d int)
}

// Sizing.
const (
	CalendarHeaderH = 22
	CalendarCellW   = 24
	CalendarCellH   = 18
)

// NewCalendar builds a Calendar for the given (year, month, day).
func NewCalendar(year, month, day int) *Calendar {
	c := &Calendar{Year: year, Month: month, Day: day}
	c.clamp()
	return c
}

// SetDate moves the calendar to (year, month, day).
func (c *Calendar) SetDate(year, month, day int) {
	c.Year = year
	c.Month = month
	c.Day = day
	c.clamp()
}

// SetToday records the "today" pill the calendar should highlight
// regardless of which (Y/M) is being viewed.
func (c *Calendar) SetToday(y, m, d int) {
	c.TodayY = y
	c.TodayM = m
	c.TodayD = d
}

// clamp keeps the month + day in legal ranges so a malformed payload
// can't break the layout.
func (c *Calendar) clamp() {
	if c.Month < 1 {
		c.Month = 1
	} else if c.Month > 12 {
		c.Month = 12
	}
	dim := DaysInMonth(c.Year, c.Month)
	if c.Day < 1 {
		c.Day = 1
	}
	if c.Day > dim {
		c.Day = dim
	}
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
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	// Header: month / year.
	hdr := monthName(c.Month) + " " + itoa(c.Year)
	hx := r.X + (r.W-TextWidth(hdr))/2
	hy := r.Y + (CalendarHeaderH-GlyphHeight)/2
	DrawText(p, hx, hy, hdr, theme.OnSurface)
	// Weekday row.
	weekdayY := r.Y + CalendarHeaderH
	for i, label := range weekdayLabels {
		cx := r.X + i*CalendarCellW + (CalendarCellW-TextWidth(label))/2
		DrawText(p, cx, weekdayY+2, label, theme.OnSurface)
	}
	// Day grid.
	first := WeekdayOfFirst(c.Year, c.Month)
	dim := DaysInMonth(c.Year, c.Month)
	gridY := weekdayY + GlyphHeight + 4
	for d := 1; d <= dim; d++ {
		idx := first + d - 1
		col := idx % 7
		row := idx / 7
		cx := r.X + col*CalendarCellW
		cy := gridY + row*CalendarCellH
		bg := theme.Surface
		ink := theme.OnSurface
		isToday := (c.TodayY == c.Year && c.TodayM == c.Month && c.TodayD == d)
		if d == c.Day {
			bg = theme.Accent
			ink = theme.Background
		} else if isToday {
			bg = theme.SurfaceAlt
		}
		fillRect(p, cx, cy, CalendarCellW, CalendarCellH, bg)
		txt := itoa(d)
		DrawText(p, cx+(CalendarCellW-TextWidth(txt))/2, cy+(CalendarCellH-GlyphHeight)/2, txt, ink)
	}
}

// OnEvent dispatches a click on a day cell to OnSelect.
func (c *Calendar) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	gridY := CalendarHeaderH + GlyphHeight + 4
	if ev.Y < gridY {
		return
	}
	col := ev.X / CalendarCellW
	if col < 0 || col > 6 {
		return
	}
	row := (ev.Y - gridY) / CalendarCellH
	first := WeekdayOfFirst(c.Year, c.Month)
	idx := row*7 + col
	if idx < first {
		return
	}
	d := idx - first + 1
	dim := DaysInMonth(c.Year, c.Month)
	if d < 1 || d > dim {
		return
	}
	c.Day = d
	if c.OnSelect != nil {
		c.OnSelect(c.Year, c.Month, d)
	}
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
