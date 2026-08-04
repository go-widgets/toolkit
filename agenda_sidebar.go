// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// AgendaSidebar is the calendar list that sits beside an Agenda (Google/Apple
// Calendar's left rail): an optional title row above one row per
// AgendaCalendar, each showing a colour swatch, the calendar name, and its
// visibility state. Clicking a row flips that calendar's Hidden flag and fires
// OnToggle.
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
	// OnToggle fires after a click flips Calendars[i].Hidden, with that row's
	// index. Nil is safe. The flip has already been applied when it runs, so a
	// host can persist the new state or re-sync.
	OnToggle func(i int)
}

// AgendaSidebarRowH is the pixel height of one calendar row.
const AgendaSidebarRowH = 24

// agendaSidebarSwatch is the side length of a row's colour swatch square.
const agendaSidebarSwatch = 12

// agendaSidebarPadX is the left inset before the swatch + the gap after it.
const agendaSidebarPadX = 8

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

// swatchColor resolves a calendar's swatch colour: its own Color, or the theme
// Accent when it left Color unset — the same "zero falls back to Accent" rule
// the events use, so a calendar and its events read as one colour.
func (s *AgendaSidebar) swatchColor(cal AgendaCalendar, theme *Theme) RGBA {
	if cal.Color != (RGBA{}) {
		return cal.Color
	}
	return theme.Accent
}

// Draw paints the sidebar: a SurfaceAlt background with a right divider, the
// optional title row, then one row per calendar. A visible calendar shows a
// filled swatch and full-strength name; a Hidden one shows a hollow (outline)
// swatch and a dimmed name, so visibility reads at a glance. Every row is
// clipped to its rectangle so a long name never bleeds into the next row or
// past the rail.
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
			ink := theme.OnSurface
			if cal.Hidden {
				ink = dimInk(theme)
			}
			s.drawText(p, sx+sw+agendaSidebarPadX, row.Y+(row.H-s.glyphHeight())/2, cal.Name, ink)
		})
	}
}

// OnEvent toggles the visibility of the calendar row under an EventClick
// (flipping Calendars[i].Hidden) and fires OnToggle with its index. Clicks on
// the header or dead space, and any non-click event, are no-ops.
func (s *AgendaSidebar) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	i := s.rowAt(ev.X, ev.Y)
	if i < 0 {
		return
	}
	s.Calendars[i].Hidden = !s.Calendars[i].Hidden
	if s.OnToggle != nil {
		s.OnToggle(i)
	}
}
