// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// newTestRangePicker returns a July-2026 picker placed at a non-zero origin so
// the coordinate math is exercised with a real offset.
func newTestRangePicker() *DateRangePicker {
	rp := NewDateRangePicker(2026, 7)
	rp.SetBounds(Rect{X: 5, Y: 7, W: 7 * CalendarCellW, H: 220})
	return rp
}

// clickDay dispatches a widget-local click at the centre of the cell for day d
// in the currently-displayed (y, m).
func clickDay(rp *DateRangePicker, y, m, d int) {
	first := WeekdayOfFirst(y, m)
	idx := first + d - 1
	col, row := idx%7, idx/7
	gridY := CalendarHeaderH + GlyphHeight() + 4
	x := col*CalendarCellW + CalendarCellW/2
	yy := gridY + row*CalendarCellH + CalendarCellH/2
	rp.OnEvent(Event{Kind: EventClick, X: x, Y: yy})
}

// cellFillAbs returns the absolute (surface-space) pixel near the top-left
// corner of the day-d cell — away from the centred day-number glyph — so tests
// sample the cell's fill colour rather than its text ink.
func cellFillAbs(rp *DateRangePicker, y, m, d int) (int, int) {
	r := rp.Bounds()
	first := WeekdayOfFirst(y, m)
	idx := first + d - 1
	col, row := idx%7, idx/7
	gridY := CalendarHeaderH + GlyphHeight() + 4
	x := r.X + col*CalendarCellW + 2
	yy := r.Y + gridY + row*CalendarCellH + 2
	return x, yy
}

func TestDateRangePickerSelectStartEnd(t *testing.T) {
	rp := newTestRangePicker()
	var gotEnd Date
	fired := 0
	// A host subscribes to End(): completing the range Sets End and notifies.
	rp.End().Subscribe(func(e Date) { gotEnd, fired = e, fired+1 })

	// First click sets Start, clears End (already unset -> no End notify).
	clickDay(rp, 2026, 7, 5)
	if got := rp.Start().Get(); (got != Date{Y: 2026, M: 7, D: 5}) {
		t.Fatalf("Start = %+v, want 2026-07-05", got)
	}
	if !rp.End().Get().isZero() {
		t.Fatalf("End = %+v, want unset after first click", rp.End().Get())
	}
	if fired != 0 {
		t.Fatalf("End notified %d times after first click, want 0", fired)
	}

	// Second click (>= Start) sets End and notifies once.
	clickDay(rp, 2026, 7, 20)
	if got := rp.End().Get(); (got != Date{Y: 2026, M: 7, D: 20}) {
		t.Fatalf("End = %+v, want 2026-07-20", got)
	}
	if fired != 1 {
		t.Fatalf("End notified %d times, want 1", fired)
	}
	if (rp.Start().Get() != Date{Y: 2026, M: 7, D: 5}) || (gotEnd != Date{Y: 2026, M: 7, D: 20}) {
		t.Fatalf("range = (%+v, %+v), want (07-05, 07-20)", rp.Start().Get(), gotEnd)
	}
}

func TestDateRangePickerEndBeforeStartResets(t *testing.T) {
	rp := newTestRangePicker()
	fired := 0
	// End() is Set to a real date only on range completion, so a subscription
	// to it is the MVVM equivalent of the old OnChange fire count.
	rp.End().Subscribe(func(Date) { fired++ })

	clickDay(rp, 2026, 7, 20) // Start = 20
	clickDay(rp, 2026, 7, 5)  // earlier than Start -> resets Start, no fire
	if got := rp.Start().Get(); (got != Date{Y: 2026, M: 7, D: 5}) {
		t.Fatalf("Start = %+v, want reset to 2026-07-05", got)
	}
	if !rp.End().Get().isZero() {
		t.Fatalf("End = %+v, want still unset", rp.End().Get())
	}
	if fired != 0 {
		t.Fatalf("End notified %d times, want 0", fired)
	}
}

func TestDateRangePickerRangeHighlight(t *testing.T) {
	rp := newTestRangePicker()
	clickDay(rp, 2026, 7, 5)
	clickDay(rp, 2026, 7, 20)

	th := DefaultLight()
	surf := makeSurface(300, 320)
	rp.Draw(newP(surf, 300), th)

	// A day strictly between the endpoints gets the range fill (SurfaceAlt).
	bx, by := cellFillAbs(rp, 2026, 7, 10)
	if got := pixelAt(surf, 300, bx, by); got != th.SurfaceAlt {
		t.Errorf("between-day fill = %+v, want SurfaceAlt %+v", got, th.SurfaceAlt)
	}
	// Both endpoints get the Accent fill.
	sx, sy := cellFillAbs(rp, 2026, 7, 5)
	if got := pixelAt(surf, 300, sx, sy); got != th.Accent {
		t.Errorf("start-day fill = %+v, want Accent %+v", got, th.Accent)
	}
	ex, ey := cellFillAbs(rp, 2026, 7, 20)
	if got := pixelAt(surf, 300, ex, ey); got != th.Accent {
		t.Errorf("end-day fill = %+v, want Accent %+v", got, th.Accent)
	}
	// A day outside the range keeps the plain Surface fill.
	ox, oy := cellFillAbs(rp, 2026, 7, 25)
	if got := pixelAt(surf, 300, ox, oy); got != th.Surface {
		t.Errorf("outside-day fill = %+v, want Surface %+v", got, th.Surface)
	}
}

func TestDateRangePickerDrawNoSelection(t *testing.T) {
	rp := newTestRangePicker()
	th := DefaultLight()
	surf := makeSurface(300, 320)
	rp.Draw(newP(surf, 300), th)
	// With no selection every day cell paints the plain Surface fill.
	x, y := cellFillAbs(rp, 2026, 7, 10)
	if got := pixelAt(surf, 300, x, y); got != th.Surface {
		t.Errorf("no-selection day fill = %+v, want Surface %+v", got, th.Surface)
	}
}

func TestDateRangePickerReselectAfterComplete(t *testing.T) {
	rp := newTestRangePicker()
	clickDay(rp, 2026, 7, 5)
	clickDay(rp, 2026, 7, 20) // complete range

	// A click after a complete range begins a fresh selection.
	clickDay(rp, 2026, 7, 12)
	if got := rp.Start().Get(); (got != Date{Y: 2026, M: 7, D: 12}) {
		t.Fatalf("Start = %+v, want fresh 2026-07-12", got)
	}
	if !rp.End().Get().isZero() {
		t.Fatalf("End = %+v, want cleared after re-selection", rp.End().Get())
	}
}

func TestDateRangePickerMonthNav(t *testing.T) {
	rp := newTestRangePicker()
	// Preserve an in-progress selection to prove nav doesn't disturb it.
	clickDay(rp, 2026, 7, 5)
	start := rp.Start().Get()

	// Prev arrow: July -> June (no wrap).
	rp.OnEvent(Event{Kind: EventClick, X: CalendarCellW / 2, Y: CalendarHeaderH / 2})
	if rp.Cal.Month().Get() != 6 || rp.Cal.Year().Get() != 2026 {
		t.Fatalf("after prev: %d-%d, want 2026-6", rp.Cal.Year().Get(), rp.Cal.Month().Get())
	}
	if rp.Start().Get() != start || !rp.End().Get().isZero() {
		t.Fatalf("nav disturbed selection: Start=%+v End=%+v", rp.Start().Get(), rp.End().Get())
	}

	// Next arrow twice: June -> July -> August (no wrap).
	r := rp.Bounds()
	rp.OnEvent(Event{Kind: EventClick, X: r.W - 1, Y: CalendarHeaderH / 2})
	rp.OnEvent(Event{Kind: EventClick, X: r.W - 1, Y: CalendarHeaderH / 2})
	if rp.Cal.Month().Get() != 8 || rp.Cal.Year().Get() != 2026 {
		t.Fatalf("after two next: %d-%d, want 2026-8", rp.Cal.Year().Get(), rp.Cal.Month().Get())
	}

	// Middle-of-header click is a no-op.
	rp.OnEvent(Event{Kind: EventClick, X: r.W / 2, Y: CalendarHeaderH / 2})
	if rp.Cal.Month().Get() != 8 {
		t.Fatalf("header middle click changed month to %d", rp.Cal.Month().Get())
	}

	// Prev wrap: January -> December of the previous year.
	rp.Cal.Month().Set(1)
	rp.OnEvent(Event{Kind: EventClick, X: CalendarCellW / 2, Y: CalendarHeaderH / 2})
	if rp.Cal.Month().Get() != 12 || rp.Cal.Year().Get() != 2025 {
		t.Fatalf("after prev wrap: %d-%d, want 2025-12", rp.Cal.Year().Get(), rp.Cal.Month().Get())
	}

	// Next wrap: December -> January of the next year.
	rp.OnEvent(Event{Kind: EventClick, X: r.W - 1, Y: CalendarHeaderH / 2})
	if rp.Cal.Month().Get() != 1 || rp.Cal.Year().Get() != 2026 {
		t.Fatalf("after next wrap: %d-%d, want 2026-1", rp.Cal.Year().Get(), rp.Cal.Month().Get())
	}
}

func TestDateRangePickerCompleteNoSubscriber(t *testing.T) {
	rp := newTestRangePicker() // no Start()/End() subscribers
	clickDay(rp, 2026, 7, 5)
	clickDay(rp, 2026, 7, 20) // completing the range must not panic
	if got := rp.End().Get(); (got != Date{Y: 2026, M: 7, D: 20}) {
		t.Fatalf("End = %+v, want 2026-07-20 with no subscriber", got)
	}
}

func TestDateRangePickerNonClickIgnored(t *testing.T) {
	rp := newTestRangePicker()
	rp.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if !rp.Start().Get().isZero() || !rp.End().Get().isZero() {
		t.Fatalf("non-click event mutated state: Start=%+v End=%+v", rp.Start().Get(), rp.End().Get())
	}
}

// TestDateRangePickerBareAccessors proves the Start()/End() accessors
// lazy-init their Observable on a bare &DateRangePicker{} (nil fields).
func TestDateRangePickerBareAccessors(t *testing.T) {
	d := &DateRangePicker{}
	if !d.Start().Get().isZero() {
		t.Fatalf("bare Start() = %+v, want zero Date", d.Start().Get())
	}
	if !d.End().Get().isZero() {
		t.Fatalf("bare End() = %+v, want zero Date", d.End().Get())
	}
	d.Start().Set(Date{Y: 2026, M: 3, D: 4})
	if got := d.Start().Get(); (got != Date{Y: 2026, M: 3, D: 4}) {
		t.Fatalf("Start() after Set = %+v", got)
	}
}

func TestDateCmp(t *testing.T) {
	base := Date{Y: 2026, M: 7, D: 15}
	cases := []struct {
		a, b Date
		want int
	}{
		{base, Date{Y: 2027, M: 7, D: 15}, -1}, // year less
		{base, Date{Y: 2025, M: 7, D: 15}, 1},  // year greater
		{base, Date{Y: 2026, M: 8, D: 15}, -1}, // month less
		{base, Date{Y: 2026, M: 6, D: 15}, 1},  // month greater
		{base, Date{Y: 2026, M: 7, D: 16}, -1}, // day less
		{base, Date{Y: 2026, M: 7, D: 14}, 1},  // day greater
		{base, Date{Y: 2026, M: 7, D: 15}, 0},  // equal
	}
	for _, c := range cases {
		if got := c.a.cmp(c.b); got != c.want {
			t.Errorf("%+v.cmp(%+v) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
