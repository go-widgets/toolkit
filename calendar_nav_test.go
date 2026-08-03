// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestCalendarNextMonth covers NextMonth: a normal step and the December→
// January year wrap, with OnMonthChange reporting the new (year, month).
func TestCalendarNextMonth(t *testing.T) {
	c := NewCalendar(2026, 6, 10)
	var gotY, gotM int
	c.OnMonthChange = func(y, m int) { gotY, gotM = y, m }
	c.NextMonth()
	if c.Year != 2026 || c.Month != 7 {
		t.Fatalf("NextMonth: y=%d m=%d, want 2026/7", c.Year, c.Month)
	}
	if gotY != 2026 || gotM != 7 {
		t.Fatalf("OnMonthChange got %d/%d, want 2026/7", gotY, gotM)
	}
	// Wrap: December -> next January.
	c.SetDate(2026, 12, 5)
	c.NextMonth()
	if c.Year != 2027 || c.Month != 1 {
		t.Fatalf("Dec NextMonth wrap: y=%d m=%d, want 2027/1", c.Year, c.Month)
	}
}

// TestCalendarPrevMonth covers PrevMonth: a normal step and the January→
// December year wrap.
func TestCalendarPrevMonth(t *testing.T) {
	c := NewCalendar(2026, 6, 10)
	var gotY, gotM int
	c.OnMonthChange = func(y, m int) { gotY, gotM = y, m }
	c.PrevMonth()
	if c.Year != 2026 || c.Month != 5 {
		t.Fatalf("PrevMonth: y=%d m=%d, want 2026/5", c.Year, c.Month)
	}
	if gotY != 2026 || gotM != 5 {
		t.Fatalf("OnMonthChange got %d/%d, want 2026/5", gotY, gotM)
	}
	// Wrap: January -> previous December.
	c.SetDate(2026, 1, 5)
	c.PrevMonth()
	if c.Year != 2025 || c.Month != 12 {
		t.Fatalf("Jan PrevMonth wrap: y=%d m=%d, want 2025/12", c.Year, c.Month)
	}
}

// TestCalendarMonthChangeClampsDay proves the selected day is re-clamped into
// the new month: Jan 31 -> Feb (28 in 2026) pins Day to 28.
func TestCalendarMonthChangeClampsDay(t *testing.T) {
	c := NewCalendar(2026, 1, 31)
	c.NextMonth()
	if c.Month != 2 || c.Day != 28 {
		t.Fatalf("Jan31->Feb: m=%d d=%d, want 2/28", c.Month, c.Day)
	}
}

// TestCalendarNavNilCallbackSafe proves Prev/NextMonth are safe with no
// OnMonthChange wired (nil branch).
func TestCalendarNavNilCallbackSafe(t *testing.T) {
	c := NewCalendar(2026, 6, 1) // OnMonthChange nil
	c.NextMonth()
	c.PrevMonth()
	if c.Month != 6 {
		t.Fatalf("round trip landed on month %d, want 6", c.Month)
	}
}

// TestCalendarHeaderNavClicks drives the header-arrow hit-zones: a click in the
// left zone pages back, the right zone pages forward, and a click in the middle
// of the header (over the month title) changes nothing.
func TestCalendarHeaderNavClicks(t *testing.T) {
	c := NewCalendar(2026, 6, 15)
	c.SetBounds(Rect{X: 0, Y: 0, W: 7 * CalendarCellW, H: 200})
	w := c.Bounds().W

	c.OnEvent(Event{Kind: EventClick, X: CalendarNavW - 1, Y: CalendarHeaderH / 2})
	if c.Month != 5 {
		t.Fatalf("left-arrow click: month=%d, want 5", c.Month)
	}
	c.OnEvent(Event{Kind: EventClick, X: w - 1, Y: CalendarHeaderH / 2})
	if c.Month != 6 {
		t.Fatalf("right-arrow click: month=%d, want 6", c.Month)
	}
	// Middle of the header (over the title) is inert.
	c.OnEvent(Event{Kind: EventClick, X: w / 2, Y: CalendarHeaderH / 2})
	if c.Month != 6 {
		t.Fatalf("header-title click changed month to %d, want 6", c.Month)
	}
}

// TestCalendarDrawsNavArrows renders the header and asserts both the "<" and
// ">" arrow glyphs paint in their respective nav zones.
func TestCalendarDrawsNavArrows(t *testing.T) {
	c := NewCalendar(2026, 6, 1)
	const w, h = 7 * CalendarCellW, 160
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	theme := DefaultLight()
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	inZone := func(x0, x1 int) bool {
		for y := 0; y < CalendarHeaderH; y++ {
			for x := x0; x < x1; x++ {
				if pixelAt(buf, w, x, y) == theme.OnSurface {
					return true
				}
			}
		}
		return false
	}
	if !inZone(0, CalendarNavW) {
		t.Fatal("no left-arrow glyph painted in the left nav zone")
	}
	if !inZone(w-CalendarNavW, w) {
		t.Fatal("no right-arrow glyph painted in the right nav zone")
	}
}

// TestCalendarClickWeekdayRowNoOp clicks in the weekday-header band (between the
// month header and the day grid): it selects nothing and pages nothing.
func TestCalendarClickWeekdayRowNoOp(t *testing.T) {
	c := NewCalendar(2026, 6, 15)
	c.SetBounds(Rect{X: 0, Y: 0, W: 7 * CalendarCellW, H: 200})
	fired := false
	c.OnSelect = func(int, int, int) { fired = true }
	gridY := CalendarHeaderH + GlyphHeight() + 4
	// Y in [CalendarHeaderH, gridY): past the arrows, above the day cells.
	c.OnEvent(Event{Kind: EventClick, X: 3 * CalendarCellW, Y: (CalendarHeaderH + gridY) / 2})
	if fired || c.Month != 6 {
		t.Fatalf("weekday-row click had an effect: fired=%v month=%d", fired, c.Month)
	}
}
