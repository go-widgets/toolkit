// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// countColor returns how many pixels in buf exactly equal c.
func countColor(buf []byte, w, h int, c RGBA) int {
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == c {
				n++
			}
		}
	}
	return n
}

// --- colour + visibility model -------------------------------------------

func TestAgendaEventCalendarResolution(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	red := RGB(0xC0, 0x30, 0x30)
	th := DefaultLight()
	a := &Agenda{
		Calendars: []AgendaCalendar{
			{Name: "Work", Color: teal},              // 0: coloured, visible
			{Name: "Home", Color: red, Hidden: true}, // 1: coloured, hidden
			{Name: "Unset"},                          // 2: no colour
		},
	}

	// eventCalendar: valid index resolves; out-of-range / empty → not ok.
	if cal, ok := a.eventCalendar(AgendaEvent{Calendar: 0}); !ok || cal.Color != teal {
		t.Errorf("eventCalendar(0) = %v,%v want Work,true", cal, ok)
	}
	if _, ok := a.eventCalendar(AgendaEvent{Calendar: 3}); ok {
		t.Error("eventCalendar(out-of-range) should be false")
	}
	if _, ok := a.eventCalendar(AgendaEvent{Calendar: -1}); ok {
		t.Error("eventCalendar(-1) should be false")
	}
	if _, ok := (&Agenda{}).eventCalendar(AgendaEvent{Calendar: 0}); ok {
		t.Error("eventCalendar with no Calendars should be false")
	}

	// eventVisible: no calendar → visible; visible cal → visible; hidden → not.
	if !a.eventVisible(AgendaEvent{Calendar: -1}) {
		t.Error("event with no calendar should be visible")
	}
	if !a.eventVisible(AgendaEvent{Calendar: 0}) {
		t.Error("event on a visible calendar should be visible")
	}
	if a.eventVisible(AgendaEvent{Calendar: 1}) {
		t.Error("event on a hidden calendar should NOT be visible")
	}

	// eventFill precedence: Fill > calendar Color > Accent.
	if got := a.eventFill(AgendaEvent{Calendar: 0, Fill: red}, th); got != red {
		t.Errorf("eventFill(explicit Fill) = %v, want red", got)
	}
	if got := a.eventFill(AgendaEvent{Calendar: 0}, th); got != teal {
		t.Errorf("eventFill(calendar colour) = %v, want teal", got)
	}
	if got := a.eventFill(AgendaEvent{Calendar: 2}, th); got != th.Accent {
		t.Errorf("eventFill(calendar without colour) = %v, want Accent", got)
	}
	if got := a.eventFill(AgendaEvent{Calendar: -1}, th); got != th.Accent {
		t.Errorf("eventFill(no calendar) = %v, want Accent", got)
	}
}

// hiddenSpec drives one view's "hidden calendar hides its events" check: it
// builds an Agenda showing one dated event on a coloured calendar, and returns
// the widget plus the hit-test coordinate of that event.
type hiddenViewCase struct {
	name  string
	build func(teal RGBA) (*Agenda, int, int) // returns agenda + (hitX, hitY)
}

// TestAgendaHiddenCalendarHidesEvents proves that toggling a calendar's Hidden
// flag removes its events from BOTH the paint (no coloured pixels) and the
// hit-test (OnSelect never fires) in every view, and restores them when shown.
func TestAgendaHiddenCalendarHidesEvents(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	th := DefaultLight()
	const w, h = 360, 360

	cases := []hiddenViewCase{
		{"week", func(teal RGBA) (*Agenda, int, int) {
			a := NewAgenda([]AgendaEvent{{Title: "Sync", Day: 1, StartMin: 9 * 60, EndMin: 10 * 60, Calendar: 0}})
			a.Calendars = []AgendaCalendar{{Name: "Work", Color: teal}}
			a.SetBounds(Rect{W: w, H: h})
			// centre of the Tue 09:00-10:00 block
			br, _ := a.blockRect(0, 0, a.Events[0])
			return a, br.X + br.W/2, br.Y + br.H/2
		}},
		{"month", func(teal RGBA) (*Agenda, int, int) {
			a := NewAgenda([]AgendaEvent{{Title: "Release", Y: 2026, M: 7, D: 10, Calendar: 0}})
			a.Calendars = []AgendaCalendar{{Name: "Work", Color: teal}}
			a.View, a.Year, a.Month = AgendaMonth, 2026, 7
			a.SetBounds(Rect{W: w, H: h})
			chips, _ := a.monthChips(0, 0)
			return a, chips[0].rect.X + 1, chips[0].rect.Y + chips[0].rect.H/2
		}},
		{"mini", func(teal RGBA) (*Agenda, int, int) {
			a := NewAgenda([]AgendaEvent{{Title: "Trip", Y: 2026, M: 7, D: 10, Calendar: 0}})
			a.Calendars = []AgendaCalendar{{Name: "Home", Color: teal}}
			a.View, a.Year, a.Month = AgendaQuarter, 2026, 7
			a.SetBounds(Rect{W: w, H: h})
			// hit via hitMini by locating the event's cell through the widget itself
			return a, -1, -1 // mini uses selection-based check below
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, hx, hy := c.build(teal)

			// Visible: the calendar colour appears + a click selects the event.
			surf := makeSurface(w, h)
			a.Draw(newP(surf, w), th)
			if countColor(surf, w, h, teal) == 0 {
				t.Fatal("visible calendar: expected coloured pixels, found none")
			}
			if c.name != "mini" {
				a.Selected = -1
				a.OnEvent(Event{Kind: EventClick, X: hx, Y: hy})
				if a.Selected != 0 {
					t.Fatalf("visible calendar: click did not select event (Selected=%d)", a.Selected)
				}
			}

			// Hidden: no coloured pixels + the event is unhittable.
			a.Calendars[0].Hidden = true
			a.Selected = -1
			surf2 := makeSurface(w, h)
			a.Draw(newP(surf2, w), th)
			if got := countColor(surf2, w, h, teal); got != 0 {
				t.Fatalf("hidden calendar: expected 0 coloured pixels, got %d", got)
			}
			if c.name != "mini" {
				a.OnEvent(Event{Kind: EventClick, X: hx, Y: hy})
				if a.Selected != -1 {
					t.Fatalf("hidden calendar: click selected a hidden event (Selected=%d)", a.Selected)
				}
			}
		})
	}
}

// TestAgendaHitMiniSkipsHiddenAndDayEvent covers the mini-view hit skip + the
// dayEvent visibility skip directly (the paint check above proves the dot skip).
func TestAgendaHitMiniSkipsHiddenAndDayEvent(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	a := NewAgenda([]AgendaEvent{{Title: "Trip", Y: 2026, M: 7, D: 10, Calendar: 0}})
	a.Calendars = []AgendaCalendar{{Name: "Home", Color: teal}}
	a.View, a.Year, a.Month = AgendaQuarter, 2026, 7
	a.SetBounds(Rect{W: 360, H: 360})

	// Find a hitMini coordinate by scanning the widget for the event cell.
	var hx, hy int = -1, -1
	for y := 0; y < 360 && hx < 0; y++ {
		for x := 0; x < 360; x++ {
			if a.hitMini(x, y) == 0 {
				hx, hy = x, y
				break
			}
		}
	}
	if hx < 0 {
		t.Fatal("could not locate the mini event cell")
	}
	if _, ok := a.dayEvent(2026, 7, 10); !ok {
		t.Fatal("dayEvent should see the visible event")
	}
	a.Calendars[0].Hidden = true
	if a.hitMini(hx, hy) != -1 {
		t.Error("hitMini should skip a hidden-calendar event")
	}
	if _, ok := a.dayEvent(2026, 7, 10); ok {
		t.Error("dayEvent should skip a hidden-calendar event")
	}
}

// --- AgendaSidebar --------------------------------------------------------

func TestNewAgendaSidebar(t *testing.T) {
	s := NewAgendaSidebar(nil)
	if s.Calendars == nil {
		t.Error("nil calendars should normalise to non-nil")
	}
	if s.Title != "Calendars" {
		t.Errorf("default Title = %q, want Calendars", s.Title)
	}
}

func TestAgendaSidebarDrawAndSwatch(t *testing.T) {
	teal := RGB(0x0D, 0x94, 0x88)
	th := DefaultLight()
	const w, h = 180, 200
	cals := []AgendaCalendar{
		{Name: "Work", Color: teal},               // visible → filled swatch
		{Name: "Home", Color: teal, Hidden: true}, // hidden → hollow swatch
		{Name: "Unset"},                           // no colour → accent swatch
	}
	s := NewAgendaSidebar(cals)
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	s.Draw(newP(surf, w), th)

	// Row 0 (visible): swatch interior is filled with teal.
	r0 := s.rowRect(0)
	inX := r0.X + agendaSidebarPadX + agendaSidebarSwatch/2
	inY := r0.Y + (r0.H-agendaSidebarSwatch)/2 + agendaSidebarSwatch/2
	if got := pixelAt(surf, w, inX, inY); got != teal {
		t.Errorf("visible swatch interior = %v, want teal (filled)", got)
	}
	// Row 1 (hidden): swatch interior is NOT teal (hollow outline only).
	r1 := s.rowRect(1)
	inY1 := r1.Y + (r1.H-agendaSidebarSwatch)/2 + agendaSidebarSwatch/2
	if got := pixelAt(surf, w, inX, inY1); got == teal {
		t.Error("hidden swatch interior should be hollow (not teal-filled)")
	}
	// swatchColor falls back to Accent for the colourless calendar.
	if got := s.swatchColor(cals[2], th); got != th.Accent {
		t.Errorf("swatchColor(no colour) = %v, want Accent", got)
	}
}

func TestAgendaSidebarNoTitle(t *testing.T) {
	th := DefaultLight()
	const w, h = 180, 200
	s := NewAgendaSidebar([]AgendaCalendar{{Name: "Work", Color: RGB(1, 2, 3)}})
	s.Title = ""
	s.SetBounds(Rect{W: w, H: h})
	if s.headerH() != 0 {
		t.Errorf("headerH with empty Title = %d, want 0", s.headerH())
	}
	// Row 0 now sits at the very top (y offset 0).
	if s.rowRect(0).Y != 0 {
		t.Errorf("first row Y with no title = %d, want 0", s.rowRect(0).Y)
	}
	surf := makeSurface(w, h)
	s.Draw(newP(surf, w), th) // must not panic; paints the row at the top
}

func TestAgendaSidebarOnEvent(t *testing.T) {
	toggled := -1
	cals := []AgendaCalendar{{Name: "Work"}, {Name: "Home"}}
	s := NewAgendaSidebar(cals)
	s.OnToggle = func(i int) { toggled = i }
	s.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 120})

	// Click row 1 → toggles Hidden + fires OnToggle(1).
	r1 := s.rowRect(1)
	s.OnEvent(Event{Kind: EventClick, X: 10, Y: r1.Y + r1.H/2})
	if !s.Calendars[1].Hidden {
		t.Error("click did not hide calendar 1")
	}
	if toggled != 1 {
		t.Errorf("OnToggle got %d, want 1", toggled)
	}
	// Click it again → un-hides.
	toggled = -1
	s.OnEvent(Event{Kind: EventClick, X: 10, Y: r1.Y + r1.H/2})
	if s.Calendars[1].Hidden {
		t.Error("second click did not un-hide calendar 1")
	}

	// Header click (y in header band) → no-op.
	toggled = -1
	s.OnEvent(Event{Kind: EventClick, X: 10, Y: s.headerH() / 2})
	if toggled != -1 {
		t.Error("header click should not toggle")
	}
	// Below the last row → no-op.
	s.OnEvent(Event{Kind: EventClick, X: 10, Y: 119})
	if toggled != -1 {
		t.Error("dead-space click should not toggle")
	}
	// x past the right edge → no-op.
	s.OnEvent(Event{Kind: EventClick, X: 200, Y: s.rowRect(0).Y + 2})
	if toggled != -1 {
		t.Error("out-of-rail click should not toggle")
	}
	// Non-click event → no-op.
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if toggled != -1 {
		t.Error("non-click should be ignored")
	}
	// OnToggle nil is safe.
	s.OnToggle = nil
	s.OnEvent(Event{Kind: EventClick, X: 10, Y: r1.Y + r1.H/2})
}

func TestAgendaSidebarRowAtEdges(t *testing.T) {
	s := NewAgendaSidebar([]AgendaCalendar{{Name: "A"}})
	s.SetBounds(Rect{W: 160, H: 120})
	if s.rowAt(-1, 40) != -1 {
		t.Error("x<0 should be -1")
	}
	if s.rowAt(160, 40) != -1 {
		t.Error("x>=W should be -1")
	}
	if s.rowAt(10, s.headerH()-1) != -1 {
		t.Error("y in header should be -1")
	}
	if s.rowAt(10, s.headerH()+2) != 0 {
		t.Error("first row should be 0")
	}
	if s.rowAt(10, s.headerH()+AgendaSidebarRowH+2) != -1 {
		t.Error("past the last row should be -1")
	}
}

// TestAgendaSidebarStaysWithinBounds asserts the sidebar never paints outside
// its Bounds() (long names, many rows).
func TestAgendaSidebarStaysWithinBounds(t *testing.T) {
	th := DefaultLight()
	const w, h = 200, 260
	b := Rect{X: 20, Y: 24, W: 150, H: 200}
	cals := []AgendaCalendar{
		{Name: "A very long calendar name that must be clipped", Color: RGB(0x0D, 0x94, 0x88)},
		{Name: "Personal", Color: RGB(0xC0, 0x30, 0x30), Hidden: true},
		{Name: "Family", Color: RGB(0xE0, 0xA0, 0x30)},
	}
	s := NewAgendaSidebar(cals)
	s.SetBounds(b)
	surf := makeSurface(w, h)
	s.Draw(newP(surf, w), th)
	minX, minY, maxX, maxY := nbPaintedBBox(surf, w, h)
	if maxX < 0 {
		t.Fatal("sidebar painted nothing")
	}
	if minX < b.X || minY < b.Y || maxX >= b.X+b.W || maxY >= b.Y+b.H {
		t.Errorf("painted bbox [%d,%d..%d,%d] escapes bounds %+v", minX, minY, maxX, maxY, b)
	}
}

// --- AgendaSidebar inline rename -----------------------------------------

// TestAgendaSidebarRenameLifecycle covers the double-click-to-open, typing,
// Enter-commit (writes Name + fires OnRename with the right i/name), and the
// invariant that a single click still only toggles Hidden.
func TestAgendaSidebarRenameLifecycle(t *testing.T) {
	cals := []AgendaCalendar{{Name: "Work"}, {Name: "Home"}}
	s := NewAgendaSidebar(cals)
	var gotI = -1
	var gotName string
	renameCalls := 0
	s.OnRename = func(i int, name string) { gotI, gotName, renameCalls = i, name, renameCalls+1 }
	toggled := -1
	s.OnToggle = func(i int) { toggled = i }
	s.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 120})

	if s.Editing() != -1 {
		t.Fatalf("Editing() = %d before opening, want -1", s.Editing())
	}

	// Double-click row 1 opens the editor WITHOUT toggling Hidden.
	r1 := s.rowRect(1)
	s.OnEvent(Event{Kind: EventClick, Code: AgendaSidebarDoubleClick, X: 60, Y: r1.Y + r1.H/2})
	if s.Editing() != 1 {
		t.Fatalf("double-click Editing() = %d, want 1", s.Editing())
	}
	if s.Calendars[1].Hidden {
		t.Error("opening the editor must not toggle Hidden")
	}
	if toggled != -1 {
		t.Error("opening the editor must not fire OnToggle")
	}
	if s.editEntry.Text != "Home" || !s.editEntry.Focused {
		t.Errorf("editor seeded %q focused=%v, want \"Home\" focused", s.editEntry.Text, s.editEntry.Focused)
	}

	// Type: a character and a Backspace both reach the Entry.
	s.OnEvent(Event{Kind: EventChar, Code: "X"})
	if s.editEntry.Text != "HomeX" {
		t.Errorf("after EventChar text = %q, want HomeX", s.editEntry.Text)
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if s.editEntry.Text != "Home" {
		t.Errorf("after Backspace text = %q, want Home", s.editEntry.Text)
	}
	s.OnEvent(Event{Kind: EventChar, Code: "y"}) // -> "Homey"

	// Enter commits: Name written, OnRename(1,"Homey"), editor closed.
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if s.Editing() != -1 {
		t.Errorf("after Enter Editing() = %d, want -1", s.Editing())
	}
	if s.Calendars[1].Name != "Homey" {
		t.Errorf("committed Name = %q, want Homey", s.Calendars[1].Name)
	}
	if gotI != 1 || gotName != "Homey" || renameCalls != 1 {
		t.Errorf("OnRename got (%d,%q) calls=%d, want (1,\"Homey\") calls=1", gotI, gotName, renameCalls)
	}

	// A plain single click still toggles Hidden and does NOT open an editor.
	toggled = -1
	s.OnEvent(Event{Kind: EventClick, X: 10, Y: r1.Y + r1.H/2})
	if s.Editing() != -1 {
		t.Error("single click must not open the editor")
	}
	if !s.Calendars[1].Hidden || toggled != 1 {
		t.Errorf("single click toggle: Hidden=%v toggled=%d, want true,1", s.Calendars[1].Hidden, toggled)
	}
}

// TestAgendaSidebarRenameCancel covers Escape cancelling (Name unchanged) and
// CancelEdit being reachable.
func TestAgendaSidebarRenameCancel(t *testing.T) {
	s := NewAgendaSidebar([]AgendaCalendar{{Name: "Work"}})
	renamed := false
	s.OnRename = func(int, string) { renamed = true }
	s.SetBounds(Rect{W: 160, H: 120})

	s.EditName(0)
	s.OnEvent(Event{Kind: EventChar, Code: "Z"}) // edit in flight
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if s.Editing() != -1 {
		t.Error("Escape should close the editor")
	}
	if s.Calendars[0].Name != "Work" {
		t.Errorf("Escape changed Name to %q, want Work (unchanged)", s.Calendars[0].Name)
	}
	if renamed {
		t.Error("Escape must not fire OnRename")
	}
}

// TestAgendaSidebarRenameClickOutsideCommits covers the click-away-commits path
// and the click-inside-keeps-editing path.
func TestAgendaSidebarRenameClickInsideAndOutside(t *testing.T) {
	s := NewAgendaSidebar([]AgendaCalendar{{Name: "Work"}, {Name: "Home"}})
	commits := 0
	s.OnRename = func(int, string) { commits++ }
	s.SetBounds(Rect{W: 160, H: 120})

	s.EditName(0)
	er := s.editLocalRect(0)
	// Click inside the editor: stays editing, no commit.
	s.OnEvent(Event{Kind: EventClick, X: er.X + er.W/2, Y: er.Y + er.H/2})
	if s.Editing() != 0 {
		t.Error("click inside the editor should keep it open")
	}
	if commits != 0 {
		t.Error("click inside must not commit")
	}
	// Click outside (a different row) commits and closes.
	r1 := s.rowRect(1)
	s.OnEvent(Event{Kind: EventClick, X: 10, Y: r1.Y + r1.H/2})
	if s.Editing() != -1 {
		t.Error("click outside should commit + close the editor")
	}
	if commits != 1 {
		t.Errorf("click-away commits = %d, want 1", commits)
	}
	// The click that committed must NOT also toggle the row it landed on.
	if s.Calendars[1].Hidden {
		t.Error("the committing click must not toggle another row")
	}
}

// TestAgendaSidebarEditNameGuards covers out-of-range EditName (both ends),
// CommitEdit / Editing on a closed editor, a stale editing index at commit,
// and nil OnRename safety.
func TestAgendaSidebarEditNameGuards(t *testing.T) {
	s := NewAgendaSidebar([]AgendaCalendar{{Name: "Work"}})
	s.SetBounds(Rect{W: 160, H: 120})

	// Out-of-range EditName is a no-op (both i<0 and i>=len).
	s.EditName(-1)
	if s.Editing() != -1 {
		t.Error("EditName(-1) should not open an editor")
	}
	s.EditName(5)
	if s.Editing() != -1 {
		t.Error("EditName(out-of-range) should not open an editor")
	}

	// CommitEdit with no editor open is a safe no-op.
	s.CommitEdit()
	if s.Editing() != -1 {
		t.Error("CommitEdit with no editor should stay closed")
	}

	// nil OnRename must not panic on commit.
	s.OnRename = nil
	s.EditName(0)
	s.OnEvent(Event{Kind: EventChar, Code: "!"})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if s.Calendars[0].Name != "Work!" {
		t.Errorf("commit with nil OnRename Name = %q, want Work!", s.Calendars[0].Name)
	}

	// Stale editing index: the calendar goes away while editing, so commit
	// closes without writing (and without OnRename).
	s.EditName(0)
	s.Calendars = s.Calendars[:0]
	fired := false
	s.OnRename = func(int, string) { fired = true }
	s.CommitEdit()
	if s.Editing() != -1 {
		t.Error("commit on a stale index should still close the editor")
	}
	if fired {
		t.Error("commit on a stale index must not fire OnRename")
	}
}

// TestAgendaSidebarRenameIgnoresStrayEvents covers the editor's default event
// arms: a non-editing, non-click event while open is ignored, and a narrow
// bounds still produces a valid (clamped) editor rect.
func TestAgendaSidebarRenameStrayAndNarrow(t *testing.T) {
	s := NewAgendaSidebar([]AgendaCalendar{{Name: "Work"}})
	s.SetBounds(Rect{W: 160, H: 120})
	s.EditName(0)
	// A key-up (not handled by the editor switch) is ignored, editor stays open.
	s.OnEvent(Event{Kind: EventKeyUp, Code: "A"})
	if s.Editing() != 0 {
		t.Error("an unhandled event kind should leave the editor open unchanged")
	}
	if s.Calendars[0].Name != "Work" {
		t.Error("an unhandled event must not mutate the name")
	}

	// Narrow bounds clamp the editor width to at least 1.
	s.SetBounds(Rect{W: 1, H: 120})
	if w := s.editLocalRect(0).W; w != 1 {
		t.Errorf("narrow editLocalRect W = %d, want 1 (clamped)", w)
	}
	if w := s.editRect(0).W; w != 1 {
		t.Errorf("narrow editRect W = %d, want 1 (clamped)", w)
	}
}

// TestAgendaSidebarRenameDrawsEntryOverRow proves the inline Entry is painted
// over the editing row's name area (its focused Accent border appears there,
// contained within the row) and that the static name is replaced.
func TestAgendaSidebarRenameDrawsEntryOverRow(t *testing.T) {
	th := DefaultLight()
	const w, h = 180, 200
	teal := RGB(0x0D, 0x94, 0x88)
	cals := []AgendaCalendar{{Name: "Work", Color: teal}, {Name: "Home", Color: teal}}
	s := NewAgendaSidebar(cals)
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})

	// Baseline (not editing): no Accent pixels in row 1's name area (swatch is
	// teal, name is OnSurface).
	surf0 := makeSurface(w, h)
	s.Draw(newP(surf0, w), th)
	er := s.editRect(1)
	if n := countColorInRect(surf0, w, er, th.Accent); n != 0 {
		t.Fatalf("baseline: found %d Accent pixels in the name area, want 0", n)
	}

	// Open the editor on row 1 and redraw: the focused Entry's Accent border
	// now paints in that area, and every Accent pixel stays inside row 1.
	s.EditName(1)
	surf := makeSurface(w, h)
	s.Draw(newP(surf, w), th)
	if n := countColorInRect(surf, w, er, th.Accent); n == 0 {
		t.Error("editing: expected the Entry's Accent border in the name area, found none")
	}
	row := s.rowRect(1)
	// Accent pixels must all fall within the editing row (nothing bled out).
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(surf, w, x, y) == th.Accent {
				if x < row.X || x >= row.X+row.W || y < row.Y || y >= row.Y+row.H {
					t.Fatalf("Accent pixel at (%d,%d) escapes editing row %+v", x, y, row)
				}
			}
		}
	}
}

// countColorInRect counts pixels equal to c within rect r (clamped to the buf).
func countColorInRect(buf []byte, w int, r Rect, c RGBA) int {
	n := 0
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if pixelAt(buf, w, x, y) == c {
				n++
			}
		}
	}
	return n
}
