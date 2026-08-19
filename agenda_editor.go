// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Event editing. An Agenda can open a small in-widget editor for one event —
// its title (an Entry) plus, when the Agenda has Calendars, a row of colour
// swatches to reassign the event to another calendar. Like DropDown's popover,
// the editor is a host-owned overlay: the host calls DrawEditor last (so it
// paints above the grid) and routes clicks/keys to EditorClick / EditorChar /
// EditorKey while Editing() reports an open editor. Committing (Enter or a
// click outside the panel) writes the title back and fires OnEventEdited;
// Escape cancels without touching the title. Reassigning the calendar applies
// immediately (and also fires OnEventEdited).
//
// The editor state lives in unexported fields on Agenda; a zero-value Agenda
// has no editor open (editorOpen is false), so this is fully opt-in and every
// existing Agenda renders exactly as before until EditEvent is called.

// agendaEditorW is the editor panel's target width (clamped to the widget).
const agendaEditorW = 220

// agendaEditorPad is the panel's inner padding.
const agendaEditorPad = 10

// agendaEditorEntryH is the title Entry's pixel height.
const agendaEditorEntryH = 22

// agendaEditorSwatch is the side length of a calendar-picker swatch.
const agendaEditorSwatch = 16

// agendaEditorSwatchGap is the horizontal gap between calendar swatches.
const agendaEditorSwatchGap = 6

// calendarSwatchColor resolves a calendar's swatch colour: its own Color, or
// the theme Accent when Color is unset — the one rule the sidebar rail and the
// editor's calendar picker share so a calendar reads as one colour everywhere.
func calendarSwatchColor(cal AgendaCalendar, theme *Theme) RGBA {
	if cal.Color != (RGBA{}) {
		return cal.Color
	}
	return theme.Accent
}

// EditEvent opens the inline editor for event i, seeding the title Entry with
// its current Title. Out-of-range i is a no-op (no editor opens). Opening a new
// editor replaces any editor already open.
func (a *Agenda) EditEvent(i int) {
	if i < 0 || i >= len(a.Events) {
		return
	}
	a.editing = i
	a.editorOpen = true
	a.editEntry = NewEntry(a.Events[i].Title)
	a.editEntry.SetFocused(true)
}

// Editing returns the index of the event whose editor is open, or -1 when no
// editor is open. A host checks this to decide whether to route keystrokes to
// EditorChar/EditorKey and to draw the overlay.
func (a *Agenda) Editing() int {
	if !a.editorOpen {
		return -1
	}
	return a.editing
}

// agendaEditorLayout is the resolved geometry of the open editor, computed once
// and shared by DrawEditor + EditorClick so paint and hit-testing never drift.
type agendaEditorLayout struct {
	panel    Rect
	entry    Rect
	swatches []Rect // one per calendar (empty when there are no calendars)
	titleY   int
	calLblY  int
	hintY    int
}

// editorLayout lays the panel out, centred in the widget's bounds and clamped
// to fit: a title line, the title Entry, an optional "Calendar" label + swatch
// row (only when the Agenda has calendars), and a hint line.
func (a *Agenda) editorLayout() agendaEditorLayout {
	b := a.Bounds()
	gh := a.glyphHeight()
	hasCal := len(a.Calendars) > 0

	w := agendaEditorW
	if max := b.W - 2*agendaEditorPad; w > max {
		w = max
	}
	if w < 1 {
		w = 1
	}

	h := agendaEditorPad
	h += gh + 6                 // title
	h += agendaEditorEntryH + 8 // entry
	if hasCal {
		h += gh + 4                 // "Calendar" label
		h += agendaEditorSwatch + 8 // swatch row
	}
	h += gh // hint
	h += agendaEditorPad
	// h is a sum of positive padding + glyph terms, so it is always >= 1.

	panel := Rect{X: b.X + (b.W-w)/2, Y: b.Y + (b.H-h)/2, W: w, H: h}

	x := panel.X + agendaEditorPad
	iw := panel.W - 2*agendaEditorPad
	if iw < 1 {
		iw = 1
	}
	y := panel.Y + agendaEditorPad

	lay := agendaEditorLayout{panel: panel}
	lay.titleY = y
	y += gh + 6
	lay.entry = Rect{X: x, Y: y, W: iw, H: agendaEditorEntryH}
	y += agendaEditorEntryH + 8
	if hasCal {
		lay.calLblY = y
		y += gh + 4
		sx := x
		for range a.Calendars {
			lay.swatches = append(lay.swatches, Rect{X: sx, Y: y, W: agendaEditorSwatch, H: agendaEditorSwatch})
			sx += agendaEditorSwatch + agendaEditorSwatchGap
		}
		y += agendaEditorSwatch + 8
	}
	lay.hintY = y
	return lay
}

// DrawEditor paints the open editor overlay (a no-op when none is open or the
// editing index has fallen out of range). The panel is clipped to itself so no
// control ever bleeds past its rounded border. The swatch of the event's
// current calendar carries an accent ring so the active calendar is obvious.
func (a *Agenda) DrawEditor(p painter.Painter, theme *Theme) {
	if !a.editorOpen || a.editing < 0 || a.editing >= len(a.Events) {
		return
	}
	lay := a.editorLayout()
	withClip(p, lay.panel, func() {
		fillRoundRect(p, lay.panel.X, lay.panel.Y, lay.panel.W, lay.panel.H, buttonRadius, theme.Surface)
		strokeRoundRect(p, lay.panel.X, lay.panel.Y, lay.panel.W, lay.panel.H, buttonRadius, theme.Accent)

		tx := lay.panel.X + agendaEditorPad
		a.drawText(p, tx, lay.titleY, "Edit event", theme.OnSurface)

		a.editEntry.SetBounds(lay.entry)
		a.editEntry.Draw(p, theme)

		if len(lay.swatches) > 0 {
			a.drawText(p, tx, lay.calLblY, "Calendar", dimInk(theme))
			cur := a.Events[a.editing].Calendar
			for k, sw := range lay.swatches {
				fillRoundRect(p, sw.X, sw.Y, sw.W, sw.H, 3, calendarSwatchColor(a.Calendars[k], theme))
				if k == cur {
					strokeRoundRect(p, sw.X-2, sw.Y-2, sw.W+4, sw.H+4, 4, theme.Accent)
				}
			}
		}

		a.drawText(p, tx, lay.hintY, "Enter save · Esc cancel", dimInk(theme))
	})
}

// EditorClick routes a click at (x, y) — in the SAME (absolute) coordinate
// space as the widget's Bounds, like DropDown.PopoverClick — while the editor
// is open, and reports whether it consumed the event (always true while open,
// so the host doesn't also treat the click as a grid selection). A click on a
// calendar swatch reassigns the event (fires OnEventEdited); a click in the
// title Entry focuses it; a click anywhere outside the panel commits the edit
// and closes. When no editor is open it returns false and does nothing.
func (a *Agenda) EditorClick(x, y int) bool {
	if !a.editorOpen {
		return false
	}
	lay := a.editorLayout()
	// Calendar reassignment.
	for k, sw := range lay.swatches {
		if sw.Contains(x, y) {
			a.Events[a.editing].Calendar = k
			if a.OnEventEdited != nil {
				a.OnEventEdited(a.editing)
			}
			return true
		}
	}
	if lay.entry.Contains(x, y) {
		a.editEntry.SetFocused(true)
		return true
	}
	if !lay.panel.Contains(x, y) {
		a.commitEdit()
		return true
	}
	return true
}

// EditorChar feeds a printable character (post-IME) to the open title Entry. A
// no-op when no editor is open.
func (a *Agenda) EditorChar(code string) {
	if !a.editorOpen {
		return
	}
	a.editEntry.OnEvent(Event{Kind: EventChar, Code: code})
}

// EditorKey feeds a key press to the open editor: "Enter" commits + closes,
// "Escape" cancels + closes (discarding the title edit), and every other key
// (Backspace, arrows, Home/End, clipboard shortcuts) is forwarded to the title
// Entry. A no-op when no editor is open.
func (a *Agenda) EditorKey(code string) {
	if !a.editorOpen {
		return
	}
	switch code {
	case "Enter":
		a.commitEdit()
	case "Escape":
		a.editorOpen = false
	default:
		a.editEntry.OnEvent(Event{Kind: EventKeyDown, Code: code})
	}
}

// commitEdit writes the title Entry back onto the edited event, fires
// OnEventEdited, and closes the editor. Guarded against a stale editing index.
func (a *Agenda) commitEdit() {
	if a.editing >= 0 && a.editing < len(a.Events) {
		a.Events[a.editing].Title = a.editEntry.Text().Get()
		if a.OnEventEdited != nil {
			a.OnEventEdited(a.editing)
		}
	}
	a.editorOpen = false
}
