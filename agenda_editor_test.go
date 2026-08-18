// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// editorAgenda builds a month Agenda with two calendars and one event on the
// first (Work) calendar, sized so the editor panel fits comfortably.
func editorAgenda() *Agenda {
	a := NewAgenda([]AgendaEvent{{Title: "Standup", Y: 2026, M: 7, D: 6, Calendar: 0}})
	a.Calendars = []AgendaCalendar{
		{Name: "Work", Color: RGB(0x0D, 0x94, 0x88)},
		{Name: "Home", Color: RGB(0xC0, 0x30, 0x30)},
	}
	a.Year, a.Month = 2026, 7
	a.View().Set(AgendaMonth)
	a.SetBounds(Rect{X: 0, Y: 0, W: 360, H: 360})
	return a
}

func TestAgendaEditEventAndEditing(t *testing.T) {
	a := editorAgenda()
	if a.Editing() != -1 {
		t.Fatal("fresh Agenda should report Editing()==-1")
	}
	a.EditEvent(5) // out of range → no-op
	if a.Editing() != -1 {
		t.Fatal("out-of-range EditEvent should not open an editor")
	}
	a.EditEvent(-1) // out of range → no-op
	if a.Editing() != -1 {
		t.Fatal("negative EditEvent should not open an editor")
	}
	a.EditEvent(0)
	if a.Editing() != 0 {
		t.Fatalf("Editing()=%d, want 0", a.Editing())
	}
	if a.editEntry == nil || a.editEntry.Text != "Standup" {
		t.Fatalf("editor entry not seeded with the title (%v)", a.editEntry)
	}
}

func TestAgendaDrawEditor(t *testing.T) {
	th := DefaultLight()
	const w, h = 360, 360

	// Closed: paints nothing.
	a := editorAgenda()
	buf := makeSurface(w, h)
	a.DrawEditor(newP(buf, w), th)
	if _, _, mx, _ := nbPaintedBBox(buf, w, h); mx >= 0 {
		t.Fatal("closed editor painted something")
	}

	// Open with calendars: paints inside the panel, within bounds, and shows
	// both calendar swatch colours (the picker).
	a.EditEvent(0)
	buf2 := makeSurface(w, h)
	a.DrawEditor(newP(buf2, w), th)
	minX, minY, maxX, maxY := nbPaintedBBox(buf2, w, h)
	if maxX < 0 {
		t.Fatal("open editor painted nothing")
	}
	if minX < 0 || minY < 0 || maxX >= w || maxY >= h {
		t.Errorf("editor bbox [%d,%d..%d,%d] escapes surface", minX, minY, maxX, maxY)
	}
	if countColor(buf2, w, h, RGB(0xC0, 0x30, 0x30)) == 0 {
		t.Error("editor should paint the Home calendar swatch colour")
	}

	// Editing index gone out of range (events truncated) → no-op even while open.
	a.Events = a.Events[:0]
	buf3 := makeSurface(w, h)
	a.DrawEditor(newP(buf3, w), th)
	if _, _, mx, _ := nbPaintedBBox(buf3, w, h); mx >= 0 {
		t.Fatal("editor with stale index should paint nothing")
	}
}

func TestAgendaDrawEditorNoCalendars(t *testing.T) {
	th := DefaultLight()
	const w, h = 300, 300
	a := NewAgenda([]AgendaEvent{{Title: "Solo", Y: 2026, M: 7, D: 6}})
	a.Year, a.Month = 2026, 7
	a.View().Set(AgendaMonth)
	a.SetBounds(Rect{W: w, H: h})
	a.EditEvent(0)
	buf := makeSurface(w, h)
	a.DrawEditor(newP(buf, w), th) // no calendars → no swatch row, must not panic
	if _, _, mx, _ := nbPaintedBBox(buf, w, h); mx < 0 {
		t.Fatal("editor without calendars still paints the title Entry")
	}
	lay := a.editorLayout()
	if len(lay.swatches) != 0 {
		t.Errorf("no-calendar editor should have 0 swatches, got %d", len(lay.swatches))
	}
}

func TestAgendaEditorClick(t *testing.T) {
	a := editorAgenda()
	edited := -1
	a.OnEventEdited = func(i int) { edited = i }

	// Closed → declines.
	if a.EditorClick(10, 10) {
		t.Fatal("closed editor: EditorClick should return false")
	}

	a.EditEvent(0)
	lay := a.editorLayout()

	// Click swatch 1 (Home) → reassigns calendar + fires OnEventEdited, stays open.
	sw := lay.swatches[1]
	if !a.EditorClick(sw.X+sw.W/2, sw.Y+sw.H/2) {
		t.Fatal("swatch click should consume the event")
	}
	if a.Events[0].Calendar != 1 || edited != 0 {
		t.Fatalf("swatch click: Calendar=%d edited=%d, want 1,0", a.Events[0].Calendar, edited)
	}
	if a.Editing() != 0 {
		t.Fatal("swatch click should not close the editor")
	}

	// Click inside the title Entry → focuses it, stays open.
	a.editEntry.SetFocused(false)
	if !a.EditorClick(lay.entry.X+4, lay.entry.Y+lay.entry.H/2) {
		t.Fatal("entry click should be consumed")
	}
	if !a.editEntry.Focused() {
		t.Fatal("entry click should focus the entry")
	}

	// Click inside the panel but on no control (title area) → consumed, stays open.
	if !a.EditorClick(lay.panel.X+3, lay.titleY) {
		t.Fatal("in-panel dead click should be consumed")
	}
	if a.Editing() != 0 {
		t.Fatal("in-panel dead click should not close")
	}

	// Click OUTSIDE the panel → commits + closes.
	edited = -1
	a.EditorClick(lay.panel.X-5, lay.panel.Y-5)
	if a.Editing() != -1 {
		t.Fatal("outside click should close the editor")
	}
	if edited != 0 {
		t.Fatal("outside click should commit (fire OnEventEdited)")
	}
}

func TestAgendaEditorCharAndKey(t *testing.T) {
	a := editorAgenda()
	var edits []int
	a.OnEventEdited = func(i int) { edits = append(edits, i) }

	// Char/key are no-ops while closed.
	a.EditorChar("x")
	a.EditorKey("Enter")
	if a.Editing() != -1 {
		t.Fatal("char/key while closed must not open an editor")
	}

	// Type into the title, then Enter commits the new title.
	a.EditEvent(0)
	a.EditorChar("!")        // append after "Standup"
	a.EditorKey("Backspace") // forwarded → deletes the "!"
	a.EditorKey("Home")      // forwarded → cursor to start
	a.EditorChar("A")        // "AStandup"
	a.EditorKey("Enter")     // commit
	if a.Events[0].Title != "AStandup" {
		t.Fatalf("committed title = %q, want AStandup", a.Events[0].Title)
	}
	if a.Editing() != -1 {
		t.Fatal("Enter should close the editor")
	}
	if len(edits) != 1 || edits[0] != 0 {
		t.Fatalf("commit fired OnEventEdited %v, want [0]", edits)
	}

	// Escape cancels: the title edit is discarded.
	a.EditEvent(0)
	a.EditorChar("Z") // would make "ZAStandup"
	a.EditorKey("Escape")
	if a.Editing() != -1 {
		t.Fatal("Escape should close the editor")
	}
	if a.Events[0].Title != "AStandup" {
		t.Fatalf("Escape must discard the edit; title = %q, want AStandup", a.Events[0].Title)
	}
}

// TestAgendaCommitStaleIndex covers commitEdit's guard when the edited event
// has vanished (Events truncated) before the commit lands: it must close
// cleanly without writing or firing.
func TestAgendaCommitStaleIndex(t *testing.T) {
	a := editorAgenda()
	fired := false
	a.OnEventEdited = func(int) { fired = true }
	a.EditEvent(0)
	a.editEntry.Text = "changed"
	a.Events = a.Events[:0] // the event is gone
	a.EditorKey("Enter")    // commitEdit with a stale index
	if a.Editing() != -1 {
		t.Fatal("commit should still close the editor")
	}
	if fired {
		t.Fatal("commit with a stale index must not fire OnEventEdited")
	}
}

func TestCalendarSwatchColor(t *testing.T) {
	th := DefaultLight()
	teal := RGB(0x0D, 0x94, 0x88)
	if got := calendarSwatchColor(AgendaCalendar{Color: teal}, th); got != teal {
		t.Errorf("swatch(own colour) = %v, want teal", got)
	}
	if got := calendarSwatchColor(AgendaCalendar{}, th); got != th.Accent {
		t.Errorf("swatch(no colour) = %v, want Accent", got)
	}
}

// TestAgendaEditorTinyBounds exercises the panel width clamp on a widget too
// narrow for the natural panel width.
func TestAgendaEditorTinyBounds(t *testing.T) {
	a := NewAgenda([]AgendaEvent{{Title: "T", Y: 2026, M: 7, D: 6}})
	a.SetBounds(Rect{W: 12, H: 200}) // narrower than 2*pad → width clamps to 1
	a.EditEvent(0)
	lay := a.editorLayout()
	if lay.panel.W < 1 || lay.entry.W < 1 {
		t.Fatalf("clamped panel/entry width should be >=1, got panel=%d entry=%d", lay.panel.W, lay.entry.W)
	}
	buf := makeSurface(12, 200)
	a.DrawEditor(newP(buf, 12), DefaultLight()) // must not panic
}
