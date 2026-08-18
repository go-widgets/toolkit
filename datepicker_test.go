// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
)

// newTestDatePicker returns a picker on 2026-07-04, its field bounded at the
// top-left so popover coordinate math is exercised.
func newTestDatePicker() *DatePicker {
	dp := NewDatePicker(2026, 7, 4)
	dp.SetBounds(Rect{X: 0, Y: 0, W: 160, H: DatePickerFieldH()})
	return dp
}

func TestDatePickerText(t *testing.T) {
	dp := NewDatePicker(2026, 7, 4)
	if got := dp.Text(); got != "2026-07-04" {
		t.Errorf("Text() = %q, want 2026-07-04", got)
	}
	// A one-digit year still pads to four columns.
	dp.SetDate(9, 12, 25)
	if got := dp.Text(); got != "0009-12-25" {
		t.Errorf("Text() = %q, want 0009-12-25", got)
	}
	y, m, d := dp.Date()
	if y != 9 || m != 12 || d != 25 {
		t.Errorf("Date() = (%d,%d,%d), want (9,12,25)", y, m, d)
	}
}

func TestDatePickerDrawClosedAndOpen(t *testing.T) {
	dp := newTestDatePicker()
	surf := makeSurface(200, 300)
	// Closed: the field border paints the top-left corner.
	dp.Draw(newP(surf, 200), DefaultLight())
	if got := pixelAt(surf, 200, 0, 0); got != DefaultLight().Border {
		t.Errorf("closed field corner = %+v, want Border", got)
	}
	// Open: the calendar popup paints below the field.
	dp.Open().Set(true)
	dp.Draw(newP(surf, 200), DefaultLight())
	pb := dp.PopoverBounds()
	if got := pixelAt(surf, 200, pb.X, pb.Y); got != DefaultLight().Border {
		t.Errorf("open popup corner = %+v, want Calendar Border", got)
	}
}

func TestDatePickerClickTogglesPopup(t *testing.T) {
	dp := newTestDatePicker()
	// Click on the field opens it.
	dp.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if !dp.Open().Get() {
		t.Fatal("field click did not open popup")
	}
	// Click on the field again closes it.
	dp.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if dp.Open().Get() {
		t.Fatal("second field click did not close popup")
	}
}

func TestDatePickerPickDayClosesAndBindsDate(t *testing.T) {
	dp := newTestDatePicker()
	// A host binds the selected date through the embedded Calendar's own
	// accessors and the open flag through Open(); picking a day notifies both.
	var gotY, gotM, gotD int
	var gotOpen bool
	dp.Cal.Day().Subscribe(func(d int) {
		gotY, gotM, gotD = dp.Cal.Year().Get(), dp.Cal.Month().Get(), d
	})
	dp.Open().Subscribe(func(o bool) { gotOpen = o })
	dp.Open().Set(true)

	// Compute the popover-local pixel of day 15 and translate it into the
	// widget-local frame OnEvent expects (field-relative).
	pb := dp.PopoverBounds()
	first := WeekdayOfFirst(2026, 7)
	idx := first + 15 - 1
	col, row := idx%7, idx/7
	gridY := CalendarHeaderH + GlyphHeight() + 4
	calX := col*CalendarCellW + CalendarCellW/2
	calY := gridY + row*CalendarCellH + CalendarCellH/2
	// widget-local = calendar-local + (popover origin - field origin)
	r := dp.Bounds()
	dp.OnEvent(Event{Kind: EventClick, X: calX + (pb.X - r.X), Y: calY + (pb.Y - r.Y)})

	if dp.Open().Get() || gotOpen {
		t.Error("picking a day should close the popup")
	}
	if gotY != 2026 || gotM != 7 || gotD != 15 {
		t.Errorf("bound date = (%d,%d,%d), want (2026,7,15)", gotY, gotM, gotD)
	}
}

func TestDatePickerClickOutsidePopupIgnored(t *testing.T) {
	dp := newTestDatePicker()
	dp.Open().Set(true)
	// A click far to the right of both the field and the popup: neither the
	// forward-to-calendar branch nor the field-toggle branch fires.
	dp.OnEvent(Event{Kind: EventClick, X: 10_000, Y: 5})
	if !dp.Open().Get() {
		t.Error("click outside field+popup should leave popup open, unchanged")
	}
}

func TestDatePickerNonClickIgnored(t *testing.T) {
	dp := newTestDatePicker()
	dp.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if dp.Open().Get() {
		t.Error("non-click event should be ignored")
	}
}

// TestDatePickerBareAccessors covers the zero-value &DatePicker{} path: Open()
// lazily initialises to false, and a host can bind it (Subscribe / Set).
func TestDatePickerBareAccessors(t *testing.T) {
	dp := &DatePicker{}
	if dp.Open().Get() {
		t.Fatal("bare DatePicker Open() should default to false")
	}
	var gotOpen bool
	dp.Open().Subscribe(func(o bool) { gotOpen = o })
	dp.Open().Set(true)
	if !dp.Open().Get() || !gotOpen {
		t.Fatalf("host bind Open: Get=%v host=%v, want true", dp.Open().Get(), gotOpen)
	}
}

// TestCalendarBareAccessors covers the zero-value &Calendar{} path: Year() /
// Month() / Day() lazily initialise to 0, and a host can bind each (Subscribe /
// Set).
func TestCalendarBareAccessors(t *testing.T) {
	c := &Calendar{}
	if c.Year().Get() != 0 || c.Month().Get() != 0 || c.Day().Get() != 0 {
		t.Fatalf("bare Calendar accessors = %d/%d/%d, want 0/0/0",
			c.Year().Get(), c.Month().Get(), c.Day().Get())
	}
	var gotY, gotM, gotD int
	c.Year().Subscribe(func(y int) { gotY = y })
	c.Month().Subscribe(func(m int) { gotM = m })
	c.Day().Subscribe(func(d int) { gotD = d })
	c.Year().Set(2026)
	c.Month().Set(7)
	c.Day().Set(4)
	if gotY != 2026 || gotM != 7 || gotD != 4 {
		t.Fatalf("host bind date = %d/%d/%d, want 2026/7/4", gotY, gotM, gotD)
	}
}
