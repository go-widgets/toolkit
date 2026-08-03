// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"os"
	"testing"
)

func TestNewAgendaSeedsDefaults(t *testing.T) {
	a := NewAgenda(nil)
	if a.Events == nil {
		t.Error("nil event slice should normalise to non-nil empty slice")
	}
	if len(a.Events) != 0 {
		t.Errorf("empty Agenda has %d events, want 0", len(a.Events))
	}
	if len(a.DayNames) != 7 {
		t.Errorf("NewAgenda seeded %d day names, want 7", len(a.DayNames))
	}
	if a.DayNames[0] != "Mon" || a.DayNames[6] != "Sun" {
		t.Errorf("day names = %v, want Mon..Sun", a.DayNames)
	}
	if a.StartHour != 8 || a.EndHour != 18 {
		t.Errorf("hour range = %d..%d, want 8..18", a.StartHour, a.EndHour)
	}
	if a.Selected != -1 {
		t.Errorf("new Agenda Selected = %d, want -1", a.Selected)
	}
	// A non-nil slice is retained verbatim.
	evs := []AgendaEvent{{Title: "A", Day: 0, StartMin: 540, EndMin: 600}}
	if a := NewAgenda(evs); len(a.Events) != 1 {
		t.Errorf("NewAgenda kept %d events, want 1", len(a.Events))
	}
}

func TestAgendaHourRange(t *testing.T) {
	// Explicit positive span wins.
	a := &Agenda{StartHour: 6, EndHour: 20}
	if s, e := a.hourRange(); s != 6 || e != 20 {
		t.Errorf("explicit hourRange = %d..%d, want 6..20", s, e)
	}
	// Zero-value / inverted span falls back to the working day.
	a = &Agenda{}
	if s, e := a.hourRange(); s != 8 || e != 18 {
		t.Errorf("fallback hourRange = %d..%d, want 8..18", s, e)
	}
	a = &Agenda{StartHour: 12, EndHour: 12}
	if s, e := a.hourRange(); s != 8 || e != 18 {
		t.Errorf("equal-bounds hourRange = %d..%d, want 8..18", s, e)
	}
}

func TestAgendaHourLabel(t *testing.T) {
	if got := agendaHourLabel(8); got != "08:00" {
		t.Errorf("agendaHourLabel(8) = %q, want 08:00", got)
	}
	if got := agendaHourLabel(14); got != "14:00" {
		t.Errorf("agendaHourLabel(14) = %q, want 14:00", got)
	}
}

func TestAgendaBlockRect(t *testing.T) {
	a := NewAgenda(nil)
	a.SetBounds(Rect{X: 0, Y: 0, W: 700, H: AgendaHeaderH + 10*AgendaHourH})

	// Day out of range (below and above) yields ok=false.
	if _, ok := a.blockRect(0, 0, AgendaEvent{Day: -1, StartMin: 540, EndMin: 600}); ok {
		t.Error("Day -1 should be out of range")
	}
	if _, ok := a.blockRect(0, 0, AgendaEvent{Day: 7, StartMin: 540, EndMin: 600}); ok {
		t.Error("Day 7 (== len) should be out of range")
	}

	// A valid mid-grid event: check it lands right of the gutter and below the
	// header with a positive size.
	br, ok := a.blockRect(0, 0, AgendaEvent{Day: 1, StartMin: 540, EndMin: 600})
	if !ok {
		t.Fatal("valid event should be placeable")
	}
	if br.X < AgendaGutterW {
		t.Errorf("block X %d leaked into the gutter (<%d)", br.X, AgendaGutterW)
	}
	if br.Y < AgendaHeaderH {
		t.Errorf("block Y %d leaked into the header (<%d)", br.Y, AgendaHeaderH)
	}
	if br.W < 1 || br.H < 1 {
		t.Errorf("block size = %dx%d, want positive", br.W, br.H)
	}

	// An event fully above the visible range clamps to zero span and floors to
	// height 1 (m<startMin branch + h<1 floor).
	br, _ = a.blockRect(0, 0, AgendaEvent{Day: 0, StartMin: 0, EndMin: 60})
	if br.H != 1 {
		t.Errorf("above-range block H = %d, want floored to 1", br.H)
	}

	// An event running past the visible range clamps its end (m>endMin branch).
	full, _ := a.blockRect(0, 0, AgendaEvent{Day: 0, StartMin: 8 * 60, EndMin: 23 * 60})
	gridH := 10 * AgendaHourH
	if full.H != gridH {
		t.Errorf("over-range block H = %d, want clamped to grid height %d", full.H, gridH)
	}
}

func TestAgendaBlockWidthFloors(t *testing.T) {
	// A 7-day week squeezed into a 3px grid gives a sub-pixel column; the block
	// width floors to 1 instead of going negative.
	a := NewAgenda([]AgendaEvent{{Day: 3, StartMin: 540, EndMin: 600}})
	a.SetBounds(Rect{X: 0, Y: 0, W: AgendaGutterW + 3, H: AgendaHeaderH + 10*AgendaHourH})
	br, ok := a.blockRect(0, 0, a.Events[0])
	if !ok {
		t.Fatal("event should be placeable")
	}
	if br.W != 1 {
		t.Errorf("floored block W = %d, want 1", br.W)
	}
}

func TestAgendaDrawEventsAndGrid(t *testing.T) {
	// A zero-Fill event falls back to Accent; an explicit-Fill event paints its
	// own colour. Grid borders and both blocks render.
	teal := RGB(0x0D, 0x94, 0x88)
	a := NewAgenda([]AgendaEvent{
		{Title: "Standup", Day: 0, StartMin: 9 * 60, EndMin: 10 * 60}, // zero Fill -> Accent
		{Title: "Review", Day: 2, StartMin: 13 * 60, EndMin: 15 * 60, Fill: teal},
		{Title: "OOB", Day: 42, StartMin: 9 * 60, EndMin: 10 * 60}, // out-of-range -> skipped
	})
	w, h := 700, AgendaHeaderH+10*AgendaHourH
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	th := DefaultLight()
	a.Draw(newP(surf, w), th)

	if got := countInk(surf, w, h, th.Border); got == 0 {
		t.Error("no grid/gutter border pixels drawn")
	}
	if got := countInk(surf, w, h, th.Accent); got == 0 {
		t.Error("zero-Fill event should paint an Accent block")
	}
	if got := countInk(surf, w, h, teal); got == 0 {
		t.Error("explicit-Fill event should paint its own colour")
	}
	// Blocks live right of the gutter, never inside it.
	accentInGutter := 0
	for y := 0; y < h; y++ {
		for x := 0; x < AgendaGutterW; x++ {
			if pixelAt(surf, w, x, y) == th.Accent {
				accentInGutter++
			}
		}
	}
	if accentInGutter != 0 {
		t.Errorf("%d block pixels leaked into the hour gutter, want 0", accentInGutter)
	}
}

func TestAgendaDrawSelectionTintAndBorder(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	a := NewAgenda([]AgendaEvent{
		{Title: "A", Day: 0, StartMin: 9 * 60, EndMin: 10 * 60, Fill: teal},
		{Title: "B", Day: 1, StartMin: 11 * 60, EndMin: 12 * 60, Fill: teal},
	})
	a.Selected = 1
	w, h := 500, AgendaHeaderH+10*AgendaHourH
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	th := DefaultLight()
	a.Draw(newP(surf, w), th)
	if got := countInk(surf, w, h, agendaSelectInk(teal, th)); got == 0 {
		t.Error("Selected event should paint the accent tint")
	}
}

func TestAgendaDrawEmptyDayNames(t *testing.T) {
	// An Agenda with no day columns still draws the header/gutter/hour rules
	// without dividing by zero.
	a := NewAgenda([]AgendaEvent{{Title: "x", Day: 0, StartMin: 540, EndMin: 600}})
	a.DayNames = nil
	w, h := 300, AgendaHeaderH+10*AgendaHourH
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	th := DefaultLight()
	a.Draw(newP(surf, w), th) // must not panic
	if got := countInk(surf, w, h, th.Border); got == 0 {
		t.Error("empty-week Agenda should still draw the hour rules")
	}
}

func TestAgendaOnEventSelectsBlock(t *testing.T) {
	fired := -1
	a := NewAgenda([]AgendaEvent{
		{Title: "A", Day: 1, StartMin: 9 * 60, EndMin: 10 * 60},
		{Title: "B", Day: 3, StartMin: 13 * 60, EndMin: 14 * 60},
	})
	a.OnSelect = func(i int) { fired = i }
	a.SetBounds(Rect{X: 0, Y: 0, W: 700, H: AgendaHeaderH + 10*AgendaHourH})

	// Click inside the first block (day 1, 09:00-10:00).
	br0, _ := a.blockRect(0, 0, a.Events[0])
	a.OnEvent(Event{Kind: EventClick, X: br0.X + br0.W/2, Y: br0.Y + br0.H/2})
	if a.Selected != 0 {
		t.Errorf("after block click Selected = %d, want 0", a.Selected)
	}
	if fired != 0 {
		t.Errorf("OnSelect fired with %d, want 0", fired)
	}

	// A non-click event is ignored.
	a.Selected = 1
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if a.Selected != 1 {
		t.Errorf("non-click event changed Selected to %d, want 1", a.Selected)
	}

	// A click on grid dead-space (no block there) is a no-op.
	a.OnEvent(Event{Kind: EventClick, X: AgendaGutterW + 5, Y: AgendaHeaderH + 5})
	if a.Selected != 1 {
		t.Errorf("dead-space click changed Selected to %d, want 1", a.Selected)
	}

	// A click in the header band hits no block.
	a.OnEvent(Event{Kind: EventClick, X: 200, Y: 2})
	if a.Selected != 1 {
		t.Errorf("header click changed Selected to %d, want 1", a.Selected)
	}
}

func TestAgendaOnEventTopmostAndNilSafe(t *testing.T) {
	// Two overlapping blocks in the same day/time: the later-drawn (index 1)
	// wins the hit. OnSelect is nil, so the click must not panic.
	a := NewAgenda([]AgendaEvent{
		{Title: "under", Day: 2, StartMin: 9 * 60, EndMin: 11 * 60},
		{Title: "over", Day: 2, StartMin: 9 * 60, EndMin: 11 * 60},
	})
	a.SetBounds(Rect{X: 0, Y: 0, W: 700, H: AgendaHeaderH + 10*AgendaHourH})
	br, _ := a.blockRect(0, 0, a.Events[1])
	a.OnEvent(Event{Kind: EventClick, X: br.X + br.W/2, Y: br.Y + br.H/2})
	if a.Selected != 1 {
		t.Errorf("overlap click selected %d, want topmost 1", a.Selected)
	}
}

// TestAgendaRenderPNGDemo renders a week of events through the public RenderPNG
// path and writes it to /tmp so the result can be eyeballed. It asserts the
// encode succeeds and produces a non-empty PNG.
func TestAgendaRenderPNGDemo(t *testing.T) {
	a := NewAgenda([]AgendaEvent{
		{Title: "Standup", Day: 0, StartMin: 9 * 60, EndMin: 9*60 + 30},
		{Title: "Design review", Day: 0, StartMin: 11 * 60, EndMin: 12*60 + 30, Fill: RGB(0x2E, 0x8B, 0x57)},
		{Title: "1:1", Day: 1, StartMin: 10 * 60, EndMin: 11 * 60, Fill: RGB(0xE0, 0xA0, 0x30)},
		{Title: "Lunch", Day: 1, StartMin: 12 * 60, EndMin: 13 * 60},
		{Title: "Sprint plan", Day: 2, StartMin: 13*60 + 30, EndMin: 15 * 60, Fill: RGB(0x0D, 0x94, 0x88)},
		{Title: "Interview", Day: 3, StartMin: 9 * 60, EndMin: 10*60 + 30, Fill: RGB(0xC0, 0x30, 0x30)},
		{Title: "Focus", Day: 4, StartMin: 14 * 60, EndMin: 17 * 60, Fill: RGB(0x63, 0x4A, 0xB0)},
	})
	a.Selected = 4
	w, h := 720, AgendaHeaderH+10*AgendaHourH
	png, err := RenderPNG(a, w, h, DefaultLight())
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if len(png) == 0 {
		t.Fatal("RenderPNG produced no bytes")
	}
	if err := os.WriteFile("/tmp/tk-agenda-demo.png", png, 0o644); err != nil {
		t.Fatalf("write demo PNG: %v", err)
	}
}

// agendaMonthFixture builds a month-view agenda focused on July 2026 with a
// single event on the 3rd, sized so all six week rows fit.
func agendaMonthFixture() *Agenda {
	a := NewAgenda([]AgendaEvent{{Title: "E", Y: 2026, M: 7, D: 3}})
	a.View, a.Year, a.Month = AgendaMonth, 2026, 7
	a.DayNames = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	a.SetBounds(Rect{X: 0, Y: 0, W: 700, H: 500})
	return a
}

// dayCellCenter returns a widget-local point at the centre of the given day's
// month cell, mirroring drawMonth/monthDayAt geometry.
func dayCellCenter(a *Agenda, day int) (int, int) {
	first := WeekdayOfFirst(2026, 7)
	idx := first + day - 1
	row, col := idx/7, idx%7
	W := a.Bounds().W
	cx := monthColX(0, W, col)
	cw := monthColX(0, W, col+1) - cx
	return cx + cw/2, AgendaHeaderH + row*AgendaDayCellH + AgendaDayCellH/2
}

// TestAgendaMonthDayActivate: clicking an empty in-month day cell fires
// OnDayActivate with that day; clicking the event chip selects it instead.
func TestAgendaMonthDayActivate(t *testing.T) {
	a := agendaMonthFixture()
	got := [3]int{-9, -9, -9}
	a.OnDayActivate = func(y, m, d int) { got = [3]int{y, m, d} }

	x, y := dayCellCenter(a, 10) // day 10 has no event
	a.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	if got != [3]int{2026, 7, 10} {
		t.Fatalf("OnDayActivate = %v, want [2026 7 10]", got)
	}
}

// TestAgendaMonthDayActivateNilAndNonMonth covers the two skip branches: a
// month-view miss with no OnDayActivate set, and a non-month view (where a miss
// never activates a day) even with the callback set.
func TestAgendaMonthDayActivateNilAndNonMonth(t *testing.T) {
	a := agendaMonthFixture()
	x, y := dayCellCenter(a, 10)
	a.OnEvent(Event{Kind: EventClick, X: x, Y: y}) // nil callback: no panic

	fired := false
	a.OnDayActivate = func(int, int, int) { fired = true }
	a.View = AgendaWeek // a week-view miss must not activate a day
	a.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if fired {
		t.Fatalf("OnDayActivate fired outside month view")
	}
}

// TestAgendaMonthDayAtBranches drives monthDayAt's guards directly: no focus,
// header band, below the grid, a column miss, a spill cell, and a valid cell.
func TestAgendaMonthDayAtBranches(t *testing.T) {
	a := agendaMonthFixture()

	// No focus period -> ok=false.
	noFocus := NewAgenda(nil)
	noFocus.View = AgendaMonth
	noFocus.SetBounds(Rect{X: 0, Y: 0, W: 700, H: 500})
	if _, _, _, ok := noFocus.monthDayAt(100, 100); ok {
		t.Fatalf("monthDayAt with no focus returned ok")
	}

	if _, _, _, ok := a.monthDayAt(100, AgendaHeaderH-1); ok {
		t.Fatalf("header band returned ok")
	}
	if _, _, _, ok := a.monthDayAt(100, 100000); ok {
		t.Fatalf("below grid returned ok")
	}
	if _, _, _, ok := a.monthDayAt(a.Bounds().W, AgendaHeaderH+10); ok {
		t.Fatalf("column miss returned ok")
	}
	// Cell (row 0, col 0) is a leading spill cell when the month starts mid-week.
	if first := WeekdayOfFirst(2026, 7); first > 0 {
		cx := monthColX(0, a.Bounds().W, 0)
		if _, _, _, ok := a.monthDayAt(cx+2, AgendaHeaderH+2); ok {
			t.Fatalf("leading spill cell returned ok")
		}
	}
	// A valid in-month cell.
	x, y := dayCellCenter(a, 15)
	yy, m, d, ok := a.monthDayAt(x, y)
	if !ok || yy != 2026 || m != 7 || d != 15 {
		t.Fatalf("monthDayAt(day 15) = (%d,%d,%d,%v), want (2026,7,15,true)", yy, m, d, ok)
	}
}
