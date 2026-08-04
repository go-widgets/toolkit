// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"os"
	"testing"
)

// --- pure helpers ----------------------------------------------------------

func TestAgendaFocusYM(t *testing.T) {
	// Explicit Year+Month win.
	a := &Agenda{Year: 2026, Month: 2}
	if y, m, ok := a.focusYM(); !ok || y != 2026 || m != 2 {
		t.Errorf("explicit focusYM = %d/%d ok=%v, want 2026/2 true", y, m, ok)
	}
	// Month out of range (0) falls through to the first dated event.
	a = &Agenda{Year: 2026, Events: []AgendaEvent{{Y: 2030, M: 5, D: 1}}}
	if y, m, ok := a.focusYM(); !ok || y != 2030 || m != 5 {
		t.Errorf("derived focusYM = %d/%d ok=%v, want 2030/5 true", y, m, ok)
	}
	// An event lacking a usable date is skipped; a later valid one is used.
	a = &Agenda{Events: []AgendaEvent{{M: 13}, {Y: 2026, M: 7, D: 3}}}
	if y, m, ok := a.focusYM(); !ok || y != 2026 || m != 7 {
		t.Errorf("skip-bad focusYM = %d/%d ok=%v, want 2026/7 true", y, m, ok)
	}
	// Nothing usable -> not ok.
	a = &Agenda{}
	if _, _, ok := a.focusYM(); ok {
		t.Error("empty Agenda focusYM should be !ok")
	}
}

func TestAgendaFocusYear(t *testing.T) {
	if y, ok := (&Agenda{Year: 2027}).focusYear(); !ok || y != 2027 {
		t.Errorf("explicit focusYear = %d ok=%v, want 2027 true", y, ok)
	}
	a := &Agenda{Events: []AgendaEvent{{D: 1}, {Y: 2029, M: 1, D: 2}}}
	if y, ok := a.focusYear(); !ok || y != 2029 {
		t.Errorf("derived focusYear = %d ok=%v, want 2029 true", y, ok)
	}
	if _, ok := (&Agenda{}).focusYear(); ok {
		t.Error("empty Agenda focusYear should be !ok")
	}
}

func TestAgendaMonthArith(t *testing.T) {
	// addMonth rolls the year over past December.
	if y, m := addMonth(2026, 11, 2); y != 2027 || m != 1 {
		t.Errorf("addMonth(2026,11,2) = %d/%d, want 2027/1", y, m)
	}
	if y, m := addMonth(2026, 3, 0); y != 2026 || m != 3 {
		t.Errorf("addMonth(2026,3,0) = %d/%d, want 2026/3", y, m)
	}
	// prevMonth wraps January back to the previous December.
	if y, m := prevMonth(2026, 1); y != 2025 || m != 12 {
		t.Errorf("prevMonth(2026,1) = %d/%d, want 2025/12", y, m)
	}
	if y, m := prevMonth(2026, 8); y != 2026 || m != 7 {
		t.Errorf("prevMonth(2026,8) = %d/%d, want 2026/7", y, m)
	}
}

func TestAgendaDayEventAndEventFill(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	a := &Agenda{Events: []AgendaEvent{{Y: 2026, M: 2, D: 5, Fill: teal}}}
	if ev, has := a.dayEvent(2026, 2, 5); !has || ev.Fill != teal {
		t.Errorf("dayEvent hit = %v,%v want teal,true", ev, has)
	}
	if _, has := a.dayEvent(2026, 2, 6); has {
		t.Error("dayEvent on an empty day should be false")
	}
	th := DefaultLight()
	// Explicit Fill wins.
	if got := a.eventFill(AgendaEvent{Fill: teal}, th); got != teal {
		t.Errorf("eventFill(explicit Fill) = %v, want teal", got)
	}
	// No Fill, no calendar → theme Accent.
	if got := a.eventFill(AgendaEvent{}, th); got != th.Accent {
		t.Errorf("eventFill(bare) = %v, want Accent", got)
	}
}

// --- month view ------------------------------------------------------------

// monthEvents seeds a February 2026 month (first weekday = Sat, so there are
// leading days from January and a trailing day into March) with a zero-Fill
// event (Accent), a coloured event, and four events piled on one day to force
// the "+N" overflow marker.
func monthEvents(teal, gold RGBA) []AgendaEvent {
	evs := []AgendaEvent{
		{Title: "Kickoff", Y: 2026, M: 2, D: 3},             // zero Fill -> Accent
		{Title: "Review", Y: 2026, M: 2, D: 10, Fill: teal}, // own colour
		{Title: "Offsite", Y: 2026, M: 3, D: 2, Fill: gold}, // wrong month -> ignored
		{Title: "Ping", Y: 2026, M: 2, D: 42, Fill: gold},   // impossible day -> ignored
	}
	for k := 0; k < 4; k++ {
		evs = append(evs, AgendaEvent{Title: "Slot", Y: 2026, M: 2, D: 20, Fill: gold})
	}
	return evs
}

func TestAgendaMonthDraw(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	gold := RGB(0xE0, 0xA0, 0x30)
	a := NewAgenda(monthEvents(teal, gold))
	a.View = AgendaMonth
	a.Year, a.Month = 2026, 2
	a.DayNames = []string{"Mon", "Tue"} // short list -> empty header labels for cols 2..6
	a.Selected = 1                      // the teal event gets a selection tint
	w, h := 560, AgendaHeaderH+6*AgendaDayCellH
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	th := DefaultLight()
	a.Draw(newP(surf, w), th)

	if got := countInk(surf, w, h, th.Border); got == 0 {
		t.Error("month grid drew no border pixels")
	}
	if got := countInk(surf, w, h, th.Accent); got == 0 {
		t.Error("zero-Fill event should paint an Accent chip")
	}
	if got := countInk(surf, w, h, gold); got == 0 {
		t.Error("overflow-day events should paint gold chips")
	}
	if got := countInk(surf, w, h, agendaSelectInk(teal, th)); got == 0 {
		t.Error("Selected chip should paint the accent tint")
	}
	// The overflow marker exists: the day-20 cell has 4 events but only room
	// for 2 chips + one "+2" marker.
	chips, overflows := a.monthChips(0, 0)
	if len(overflows) != 1 || overflows[0].n != 2 {
		t.Fatalf("overflow = %+v, want one marker of n=2", overflows)
	}
	// Two chips on the overflow day + one each for the two valid single-day
	// events = 4 chips total.
	if len(chips) != 4 {
		t.Errorf("chip count = %d, want 4", len(chips))
	}
}

func TestAgendaMonthEmpty(t *testing.T) {
	// No dated events and no explicit period -> the grid stays empty but the
	// weekday header band still paints.
	a := NewAgenda(nil)
	a.View = AgendaMonth
	w, h := 400, AgendaHeaderH+6*AgendaDayCellH
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	th := DefaultLight()
	a.Draw(newP(surf, w), th) // must not panic
	if got := countInk(surf, w, h, th.SurfaceAlt); got == 0 {
		t.Error("empty month view should still paint the weekday header band")
	}
	if _, _, ok := a.focusYM(); ok {
		t.Error("empty Agenda should have no focus month")
	}
	// hitMonth with no chips returns -1.
	if got := a.hitMonth(10, 100); got != -1 {
		t.Errorf("hitMonth on empty grid = %d, want -1", got)
	}
}

func TestAgendaMonthJanuaryLeadingFromPrevYear(t *testing.T) {
	// January focus exercises prevMonth's month==1 wrap for the leading cells.
	a := NewAgenda([]AgendaEvent{{Title: "NY", Y: 2026, M: 1, D: 1}})
	a.View = AgendaMonth
	a.Year, a.Month = 2026, 1
	w, h := 480, AgendaHeaderH+6*AgendaDayCellH
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	a.Draw(newP(surf, w), DefaultLight()) // must not panic
	if got := countInk(surf, w, h, DefaultLight().Border); got == 0 {
		t.Error("January month view should still draw cell borders")
	}
}

func TestAgendaMonthHit(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	gold := RGB(0xE0, 0xA0, 0x30)
	fired := -1
	a := NewAgenda(monthEvents(teal, gold))
	a.View = AgendaMonth
	a.Year, a.Month = 2026, 2
	a.OnSelect = func(i int) { fired = i }
	w, h := 560, AgendaHeaderH+6*AgendaDayCellH
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})

	chips, overflows := a.monthChips(0, 0)
	// Click the centre of the first chip -> selects its event.
	c0 := chips[0]
	a.OnEvent(Event{Kind: EventClick, X: c0.rect.X + c0.rect.W/2, Y: c0.rect.Y + c0.rect.H/2})
	if a.Selected != c0.idx || fired != c0.idx {
		t.Errorf("chip click: Selected=%d fired=%d, want %d", a.Selected, fired, c0.idx)
	}
	// A non-click event is ignored.
	a.Selected = 99
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if a.Selected != 99 {
		t.Errorf("non-click changed Selected to %d, want 99", a.Selected)
	}
	// A click on the "+N" overflow marker selects nothing.
	a.Selected = 99
	ov := overflows[0]
	a.OnEvent(Event{Kind: EventClick, X: ov.rect.X + ov.rect.W/2, Y: ov.rect.Y + ov.rect.H/2})
	if a.Selected != 99 {
		t.Errorf("overflow-marker click changed Selected to %d, want 99", a.Selected)
	}
	// A click on an empty day cell selects nothing.
	a.OnEvent(Event{Kind: EventClick, X: 5, Y: AgendaHeaderH + 5})
	if a.Selected != 99 {
		t.Errorf("empty-cell click changed Selected to %d, want 99", a.Selected)
	}
}

func TestAgendaMonthRenderPNG(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	gold := RGB(0xE0, 0xA0, 0x30)
	a := NewAgenda(monthEvents(teal, gold))
	a.View = AgendaMonth
	a.Year, a.Month = 2026, 2
	a.Selected = 1
	w, h := 640, AgendaHeaderH+6*AgendaDayCellH
	png, err := RenderPNG(a, w, h, DefaultLight())
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if len(png) == 0 {
		t.Fatal("month RenderPNG produced no bytes")
	}
	if err := os.WriteFile("/tmp/tk-agenda-month.png", png, 0o644); err != nil {
		t.Fatalf("write month PNG: %v", err)
	}
}

// --- quarter view ----------------------------------------------------------

func TestAgendaQuarterDrawAndHit(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	gold := RGB(0xE0, 0xA0, 0x30)
	fired := -1
	// Focus Nov 2026 so the three mini months are Nov, Dec and Jan-2027
	// (year rollover). Each carries a dated event.
	a := NewAgenda([]AgendaEvent{
		{Title: "Nov", Y: 2026, M: 11, D: 12, Fill: teal},
		{Title: "Dec", Y: 2026, M: 12, D: 24}, // zero Fill -> Accent dot
		{Title: "Jan", Y: 2027, M: 1, D: 3, Fill: gold},
	})
	a.View = AgendaQuarter
	a.Year, a.Month = 2026, 11
	a.OnSelect = func(i int) { fired = i }
	w, h := 600, 220
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	th := DefaultLight()
	a.Draw(newP(surf, w), th)

	if got := countInk(surf, w, h, teal); got == 0 {
		t.Error("Nov event should paint a teal dot")
	}
	if got := countInk(surf, w, h, gold); got == 0 {
		t.Error("Jan (rolled-over) event should paint a gold dot")
	}
	if got := countInk(surf, w, h, th.Accent); got == 0 {
		t.Error("zero-Fill Dec event should paint an Accent dot")
	}

	// Click the Jan-2027 event's day cell in the third mini month.
	specs, cols, rows := a.miniLayout()
	if len(specs) != 3 {
		t.Fatalf("quarter has %d mini months, want 3", len(specs))
	}
	box := a.miniMonthBox(0, 0, 2, cols, rows) // third mini month = Jan 2027
	titleH := a.glyphHeight() + 6
	gridTop := box.Y + titleH
	cellW := box.W / 7
	cellH := (box.H - titleH) / 6
	first := WeekdayOfFirst(2027, 1)
	idx := first + 3 - 1 // day 3
	col, row := idx%7, idx/7
	cx := box.X + col*cellW + cellW/2
	cy := gridTop + row*cellH + cellH/2
	a.OnEvent(Event{Kind: EventClick, X: cx, Y: cy})
	if a.Selected != 2 || fired != 2 {
		t.Errorf("quarter day click: Selected=%d fired=%d, want 2", a.Selected, fired)
	}

	// A click on a title row (y < gridTop) selects nothing.
	a.Selected = -1
	a.OnEvent(Event{Kind: EventClick, X: box.X + box.W/2, Y: box.Y + 1})
	if a.Selected != -1 {
		t.Errorf("title-row click changed Selected to %d, want -1", a.Selected)
	}
	// A click on a dateless cell (day 3 exists, but pick an empty day) selects
	// nothing: click day 1 of Jan which has no event.
	idx1 := first // day 1
	c1, r1 := idx1%7, idx1/7
	a.OnEvent(Event{Kind: EventClick, X: box.X + c1*cellW + cellW/2, Y: gridTop + r1*cellH + cellH/2})
	if a.Selected != -1 {
		t.Errorf("empty-day click changed Selected to %d, want -1", a.Selected)
	}
}

func TestAgendaQuarterEmpty(t *testing.T) {
	a := NewAgenda(nil)
	a.View = AgendaQuarter
	w, h := 500, 200
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	a.Draw(newP(surf, w), DefaultLight()) // must not panic, draws only surface
	specs, cols, rows := a.miniLayout()
	if specs != nil || cols != 3 || rows != 1 {
		t.Errorf("empty quarter layout = %v %d×%d, want nil 3×1", specs, cols, rows)
	}
	if got := a.hitMini(10, 100); got != -1 {
		t.Errorf("hitMini on empty quarter = %d, want -1", got)
	}
}

func TestAgendaQuarterHitEdges(t *testing.T) {
	a := NewAgenda([]AgendaEvent{{Y: 2026, M: 5, D: 10}})
	a.View = AgendaQuarter
	a.Year, a.Month = 2026, 5
	// Tiny height so each mini month's cell height floors to 0 -> the
	// cellH<=0 guard fires when a click lands inside a box below its title.
	w, h := 300, 16
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	box := a.miniMonthBox(0, 0, 0, 3, 1)
	if got := a.hitMini(box.X+box.W/2, box.Y+box.H-1); got != -1 {
		t.Errorf("degenerate-cell hitMini = %d, want -1", got)
	}

	// Roomy height, but click the far right column so the column index
	// overshoots the 7-wide grid on a width that doesn't divide by 7.
	a.SetBounds(Rect{X: 0, Y: 0, W: 305, H: 240})
	box = a.miniMonthBox(0, 0, 0, 3, 1)
	titleH := a.glyphHeight() + 6
	if got := a.hitMini(box.X+box.W-1, box.Y+titleH+1); got != -1 {
		t.Errorf("right-edge overshoot hitMini = %d, want -1", got)
	}
	// Click in the inter-month gap: no box contains it -> -1.
	if got := a.hitMini(box.X+box.W+2, box.Y+titleH+1); got != -1 {
		t.Errorf("gap-click hitMini = %d, want -1", got)
	}
	// Click the top-left cell: May 2026 starts mid-week, so that cell is a
	// leading day from April (day index < 1) and resolves to no date.
	cellW := box.W / 7
	cellH := (box.H - titleH) / 6
	if got := a.hitMini(box.X+cellW/2, box.Y+titleH+cellH/2); got != -1 {
		t.Errorf("leading-cell hitMini = %d, want -1", got)
	}
}

func TestAgendaQuarterRenderPNG(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	gold := RGB(0xE0, 0xA0, 0x30)
	a := NewAgenda([]AgendaEvent{
		{Y: 2026, M: 11, D: 12, Fill: teal},
		{Y: 2026, M: 12, D: 24},
		{Y: 2027, M: 1, D: 3, Fill: gold},
	})
	a.View = AgendaQuarter
	a.Year, a.Month = 2026, 11
	png, err := RenderPNG(a, 720, 220, DefaultLight())
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if len(png) == 0 {
		t.Fatal("quarter RenderPNG produced no bytes")
	}
	if err := os.WriteFile("/tmp/tk-agenda-quarter.png", png, 0o644); err != nil {
		t.Fatalf("write quarter PNG: %v", err)
	}
}

// --- year view -------------------------------------------------------------

func TestAgendaYearDrawAndHit(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	gold := RGB(0xE0, 0xA0, 0x30)
	fired := -1
	a := NewAgenda([]AgendaEvent{
		{Title: "Feb", Y: 2026, M: 2, D: 14, Fill: teal}, // month index 1
		{Title: "May", Y: 2026, M: 5, D: 10, Fill: gold}, // month index 4
	})
	a.View = AgendaYear
	a.Year = 2026
	a.OnSelect = func(i int) { fired = i }
	w, h := 720, 480
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	a.Draw(newP(surf, w), DefaultLight())

	if got := countInk(surf, w, h, teal); got == 0 {
		t.Error("Feb event should paint a teal dot")
	}
	if got := countInk(surf, w, h, gold); got == 0 {
		t.Error("May event should paint a gold dot")
	}

	specs, cols, rows := a.miniLayout()
	if len(specs) != 12 || cols != 4 || rows != 3 {
		t.Fatalf("year layout = %d months %d×%d, want 12 4×3", len(specs), cols, rows)
	}
	// Click the May-10 cell in month index 4 (this iterates past boxes 0..3,
	// exercising the not-contained continue path).
	box := a.miniMonthBox(0, 0, 4, cols, rows)
	titleH := a.glyphHeight() + 6
	gridTop := box.Y + titleH
	cellW := box.W / 7
	cellH := (box.H - titleH) / 6
	first := WeekdayOfFirst(2026, 5)
	idx := first + 10 - 1
	col, row := idx%7, idx/7
	a.OnEvent(Event{Kind: EventClick, X: box.X + col*cellW + cellW/2, Y: gridTop + row*cellH + cellH/2})
	if a.Selected != 1 || fired != 1 {
		t.Errorf("year day click: Selected=%d fired=%d, want 1", a.Selected, fired)
	}
}

func TestAgendaYearEmpty(t *testing.T) {
	a := NewAgenda(nil)
	a.View = AgendaYear
	w, h := 600, 400
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	a.Draw(newP(surf, w), DefaultLight()) // must not panic
	specs, cols, rows := a.miniLayout()
	if specs != nil || cols != 4 || rows != 3 {
		t.Errorf("empty year layout = %v %d×%d, want nil 4×3", specs, cols, rows)
	}
}

func TestAgendaYearRenderPNG(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	gold := RGB(0xE0, 0xA0, 0x30)
	a := NewAgenda([]AgendaEvent{
		{Y: 2026, M: 2, D: 14, Fill: teal},
		{Y: 2026, M: 5, D: 10, Fill: gold},
		{Y: 2026, M: 9, D: 1},
		{Y: 2026, M: 12, D: 25, Fill: RGB(0xC0, 0x30, 0x30)},
	})
	a.View = AgendaYear
	a.Year = 2026
	png, err := RenderPNG(a, 760, 520, DefaultLight())
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if len(png) == 0 {
		t.Fatal("year RenderPNG produced no bytes")
	}
	if err := os.WriteFile("/tmp/tk-agenda-year.png", png, 0o644); err != nil {
		t.Fatalf("write year PNG: %v", err)
	}
}
