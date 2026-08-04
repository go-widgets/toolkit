// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// AgendaSidebar is the calendar list that sits beside an Agenda (Google/Apple
// Calendar's left rail): an optional title row above one row per
// AgendaCalendar, each showing a colour swatch, the calendar name, and its
// visibility state. A single click on a row flips that calendar's Hidden flag
// and fires OnToggle; a double-click opens an inline editor over the row's name
// to rename the calendar (the remote agenda), firing OnRename on commit.
//
// It shares the SAME AgendaCalendar slice as its Agenda — set both from one
// slice value (agenda.Calendars = cals; sidebar := NewAgendaSidebar(cals)) —
// so a visibility toggle here is reflected in the Agenda's rendering with no
// extra wiring: they mutate the one backing array. (Because the toggle mutates
// an element in place it never reallocates, so the shared view holds; only
// appending/replacing the slice would break the link.)
//
// A host lays it out to the left of the Agenda with an HBox (a fixed-width
// sidebar column + a flexible Agenda column), exactly as it composes any other
// two widgets — the sidebar is a plain Widget, not a mode of the Agenda.
type AgendaSidebar struct {
	Base
	// Calendars is the shared list (see the type doc). Rows render + hit-test
	// in this order.
	Calendars []AgendaCalendar
	// Title is the header label above the rows; "" hides the header row
	// entirely (the first calendar then sits at the very top).
	Title string
	// OnToggle fires after a single click flips Calendars[i].Hidden, with that
	// row's index. Nil is safe. The flip has already been applied when it runs,
	// so a host can persist the new state or re-sync.
	OnToggle func(i int)
	// OnRename fires after CommitEdit writes a new Name onto Calendars[i] (open
	// the inline editor by double-clicking a row, then Enter to commit). It
	// carries the row index and the new name, already applied to Calendars[i]
	// when it runs, so the host/VM persists it — the MVVM seam, analogous to
	// OnToggle. Nil is safe.
	OnRename func(i int, name string)

	// editIndex is the calendar row whose name is being renamed inline. It is
	// only meaningful while editEntry != nil (which is the "editor open" flag);
	// a zero-value sidebar has editEntry == nil, so no editor is open.
	editIndex int
	// editEntry is the inline text field painted over the editing row's name; a
	// nil editEntry means no rename editor is open.
	editEntry *Entry
}

// AgendaSidebarRowH is the pixel height of one calendar row.
const AgendaSidebarRowH = 24

// agendaSidebarSwatch is the side length of a row's colour swatch square.
const agendaSidebarSwatch = 12

// agendaSidebarPadX is the left inset before the swatch + the gap after it.
const agendaSidebarPadX = 8

// agendaSidebarEditInset is the vertical inset of the inline rename editor
// inside a row, so the Entry's border sits a touch above/below the row edges.
const agendaSidebarEditInset = 3

// AgendaSidebarDoubleClick is the Event.Code a host tags a double-click
// EventClick with, mirroring StatusIconSecondary for a right-click: a click
// carrying this Code opens the inline rename editor on the row under it, while
// an ordinary click (empty Code) toggles the row's visibility. A host that does
// not distinguish double-clicks simply never sets it, and rows only ever
// toggle — the rename editor stays fully opt-in.
const AgendaSidebarDoubleClick = "double"

// NewAgendaSidebar builds a sidebar over cals (the same slice the Agenda uses),
// titled "Calendars". A nil slice is normalised to a non-nil empty slice so
// range loops never special-case nil.
func NewAgendaSidebar(cals []AgendaCalendar) *AgendaSidebar {
	if cals == nil {
		cals = []AgendaCalendar{}
	}
	return &AgendaSidebar{Calendars: cals, Title: "Calendars"}
}

// headerH is the pixel height of the title row, or 0 when Title is empty (no
// header drawn, rows start at the top).
func (s *AgendaSidebar) headerH() int {
	if s.Title == "" {
		return 0
	}
	return AgendaHeaderH
}

// rowRect returns calendar row i's absolute pixel rectangle (below the header),
// the geometry Draw paints; rowAt is its widget-local inverse for hit-testing.
func (s *AgendaSidebar) rowRect(i int) Rect {
	r := s.Bounds()
	return Rect{X: r.X, Y: r.Y + s.headerH() + i*AgendaSidebarRowH, W: r.W, H: AgendaSidebarRowH}
}

// rowAt maps a widget-local (x, y) to the calendar row under it, or -1 for the
// header band, a point left/right of the rail, or below the last row. It is the
// local-coordinate inverse of rowRect (which is absolute), matching how the
// host feeds OnEvent widget-local coordinates.
func (s *AgendaSidebar) rowAt(x, y int) int {
	if x < 0 || x >= s.Bounds().W {
		return -1
	}
	yy := y - s.headerH()
	if yy < 0 {
		return -1
	}
	i := yy / AgendaSidebarRowH
	if i < 0 || i >= len(s.Calendars) {
		return -1
	}
	return i
}

// swatchColor resolves a calendar's swatch colour via the shared
// calendarSwatchColor rule (own Color, else theme Accent), so the rail and the
// editor's calendar picker paint a calendar identically.
func (s *AgendaSidebar) swatchColor(cal AgendaCalendar, theme *Theme) RGBA {
	return calendarSwatchColor(cal, theme)
}

// editLocalRect is the inline rename editor's rectangle for row i in
// widget-local coordinates (origin at the widget's top-left) — the space
// OnEvent hit-tests in, exactly like rowAt. It spans the row's name area (right
// of the swatch to the right inset), vertically inset by agendaSidebarEditInset.
func (s *AgendaSidebar) editLocalRect(i int) Rect {
	x := agendaSidebarPadX + agendaSidebarSwatch + agendaSidebarPadX
	h := AgendaSidebarRowH - 2*agendaSidebarEditInset
	y := s.headerH() + i*AgendaSidebarRowH + (AgendaSidebarRowH-h)/2
	w := s.Bounds().W - agendaSidebarPadX - x
	if w < 1 {
		w = 1
	}
	return Rect{X: x, Y: y, W: w, H: h}
}

// editRect is editLocalRect shifted into absolute coordinates, the geometry
// Draw hands the Entry (whose Draw paints in absolute space, like every row).
func (s *AgendaSidebar) editRect(i int) Rect {
	r := s.editLocalRect(i)
	b := s.Bounds()
	return Rect{X: b.X + r.X, Y: b.Y + r.Y, W: r.W, H: r.H}
}

// EditName opens the inline rename editor on calendar row i, seeding the Entry
// with its current Name and focusing it. Out-of-range i is a safe no-op (no
// editor opens). Opening a new editor replaces any editor already open, without
// committing it. It never touches Hidden.
func (s *AgendaSidebar) EditName(i int) {
	if i < 0 || i >= len(s.Calendars) {
		return
	}
	s.editIndex = i
	s.editEntry = NewEntry(s.Calendars[i].Name)
	s.editEntry.SetFocused(true)
}

// Editing returns the index of the calendar whose rename editor is open, or -1
// when none is open. A host checks this to route keystrokes into OnEvent while
// editing (Enter/Esc commit/cancel) and to know the editor overlay is live.
func (s *AgendaSidebar) Editing() int {
	if s.editEntry == nil {
		return -1
	}
	return s.editIndex
}

// CommitEdit writes the editor's text back onto Calendars[i].Name, fires
// OnRename with (i, name), and closes the editor. A no-op when no editor is
// open; guarded against an editing index gone stale (e.g. the calendar was
// removed while editing).
func (s *AgendaSidebar) CommitEdit() {
	if s.editEntry == nil {
		return
	}
	i := s.editIndex
	if i >= 0 && i < len(s.Calendars) {
		s.Calendars[i].Name = s.editEntry.Text
		if s.OnRename != nil {
			s.OnRename(i, s.Calendars[i].Name)
		}
	}
	s.editEntry = nil
}

// CancelEdit closes the rename editor without changing any Name. Safe (a no-op)
// when no editor is open.
func (s *AgendaSidebar) CancelEdit() {
	s.editEntry = nil
}

// Draw paints the sidebar: a SurfaceAlt background with a right divider, the
// optional title row, then one row per calendar. A visible calendar shows a
// filled swatch and full-strength name; a Hidden one shows a hollow (outline)
// swatch and a dimmed name, so visibility reads at a glance. Every row is
// clipped to its rectangle so a long name never bleeds into the next row or
// past the rail. While a row is being renamed (see EditName) its inline Entry
// is painted over the name area in place of the static name; the swatch is
// unchanged.
func (s *AgendaSidebar) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	// Right divider between the rail and whatever sits to its right.
	fillRect(p, r.X+r.W-1, r.Y, 1, r.H, theme.Border)

	if s.Title != "" {
		hh := s.headerH()
		withClip(p, Rect{X: r.X, Y: r.Y, W: r.W, H: hh}, func() {
			s.drawText(p, r.X+agendaSidebarPadX, r.Y+(hh-s.glyphHeight())/2, s.Title, theme.OnSurface)
		})
		fillRect(p, r.X, r.Y+hh-1, r.W, 1, theme.Border)
	}

	sw := agendaSidebarSwatch
	for i, cal := range s.Calendars {
		row := s.rowRect(i)
		withClip(p, row, func() {
			sx := row.X + agendaSidebarPadX
			sy := row.Y + (row.H-sw)/2
			col := s.swatchColor(cal, theme)
			if cal.Hidden {
				strokeRoundRect(p, sx, sy, sw, sw, 2, col)
			} else {
				fillRoundRect(p, sx, sy, sw, sw, 2, col)
			}
			if s.editEntry != nil && i == s.editIndex {
				// Rename in progress: the Entry replaces the static name (the
				// swatch above still paints, so visibility stays visible).
				s.editEntry.SetBounds(s.editRect(i))
				s.editEntry.Draw(p, theme)
				return
			}
			ink := theme.OnSurface
			if cal.Hidden {
				ink = dimInk(theme)
			}
			s.drawText(p, sx+sw+agendaSidebarPadX, row.Y+(row.H-s.glyphHeight())/2, cal.Name, ink)
		})
	}
}

// OnEvent drives the sidebar. When no rename editor is open: a double-click on
// a row (an EventClick tagged with AgendaSidebarDoubleClick) opens the inline
// rename editor over that row; an ordinary EventClick toggles the row's
// Hidden flag and fires OnToggle; clicks on the header or dead space, and any
// non-click event, are no-ops.
//
// While a rename editor is open (Editing() >= 0), events route to it instead:
// EventChar and text-editing EventKeyDown feed the Entry (caret + text);
// "Enter" commits (CommitEdit), "Escape" cancels (CancelEdit); a click inside
// the editor keeps it focused, and a click anywhere else commits — the
// Google/Apple Calendar convention that clicking away saves the rename (the
// same rule the Agenda event editor uses). No visibility toggle happens while
// editing.
func (s *AgendaSidebar) OnEvent(ev Event) {
	if s.editEntry != nil {
		s.onEventEditing(ev)
		return
	}
	if ev.Kind != EventClick {
		return
	}
	i := s.rowAt(ev.X, ev.Y)
	if i < 0 {
		return
	}
	if ev.Code == AgendaSidebarDoubleClick {
		s.EditName(i)
		return
	}
	s.Calendars[i].Hidden = !s.Calendars[i].Hidden
	if s.OnToggle != nil {
		s.OnToggle(i)
	}
}

// onEventEditing routes an event while the rename editor is open: Enter commits,
// Escape cancels, other keys + characters feed the Entry, a click inside the
// editor keeps focus, and a click outside commits (click-away saves).
func (s *AgendaSidebar) onEventEditing(ev Event) {
	switch ev.Kind {
	case EventKeyDown:
		switch ev.Code {
		case "Enter":
			s.CommitEdit()
		case "Escape":
			s.CancelEdit()
		default:
			s.editEntry.OnEvent(ev)
		}
	case EventChar:
		s.editEntry.OnEvent(ev)
	case EventClick:
		if s.editLocalRect(s.editIndex).Contains(ev.X, ev.Y) {
			s.editEntry.OnEvent(ev)
		} else {
			s.CommitEdit()
		}
	}
}
