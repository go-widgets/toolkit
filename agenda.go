// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// AgendaEvent is one appointment drawn on the week grid as a coloured block.
// Title labels the block and is clipped inside it. Day is the weekday column,
// an index in [0, len(DayNames)); events whose Day falls outside that range are
// skipped (no column to place them in). StartMin and EndMin are minutes-from-
// midnight (0..1440) bounding the block vertically as the half-open range
// [StartMin, EndMin), so EndMin must be greater than StartMin. Fill is the
// block colour — its zero value falls back to the theme's Accent so an event
// added without an explicit colour still paints in the app's palette.
type AgendaEvent struct {
	Title            string
	Day              int
	StartMin, EndMin int
	Fill             RGBA
}

// Agenda is a week view of events: a top header row of day names, a left gutter
// of hour labels, and a day-column × hour-row grid on which each AgendaEvent
// paints as a rounded block positioned at its Day column and spanning its
// [StartMin, EndMin) range clamped to the visible hours. StartHour and EndHour
// bound that visible range; when they are unset (or EndHour <= StartHour) they
// fall back to a 08:00..18:00 working day so a caller can leave them zero.
// Selected (when it indexes an event) tints that block and draws an accent
// border, and a click inside a block fires OnSelect with its index.
//
// Agenda renders through painter.Painter, so the same week draws as pixels
// (WUI/GUI) or promoted cells (TUI). It is distinct from Calendar, which is a
// month date-picker; Agenda plots events on a day/time grid. An empty event
// slice draws just the header, gutter and grid.
type Agenda struct {
	Base
	Events             []AgendaEvent
	DayNames           []string
	StartHour, EndHour int
	OnSelect           func(i int)
	Selected           int
}

// Agenda sizing constants, exported like TableRowHeight / GanttHeaderH so a host
// can measure the widget before it has a surface: AgendaHeaderH + hours*
// AgendaHourH gives the natural height and AgendaGutterW is the fixed hour-label
// gutter width.
const (
	// AgendaHeaderH is the pixel height of the day-name header row.
	AgendaHeaderH = 24
	// AgendaHourH is the pixel height of one hour row in the grid.
	AgendaHourH = 32
	// AgendaGutterW is the pixel width of the left hour-label gutter.
	AgendaGutterW = 48
)

// agendaBlockPadX is the horizontal inset between a day column's edges and the
// event block inside it, so adjacent-column blocks never touch the separator.
const agendaBlockPadX = 2

// agendaBlockRadius is the corner radius of an event block.
const agendaBlockRadius = 4

// NewAgenda builds an Agenda over the given events, seeded with a Monday-first
// week (Mon..Sun), an 08:00..18:00 visible day and no selection (Selected =
// -1). A nil slice is normalised to a non-nil empty slice so range loops and
// len() checks never special-case nil.
func NewAgenda(events []AgendaEvent) *Agenda {
	if events == nil {
		events = []AgendaEvent{}
	}
	return &Agenda{
		Events:    events,
		DayNames:  []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
		StartHour: 8,
		EndHour:   18,
		Selected:  -1,
	}
}

// hourRange returns the effective visible hour span [start, end): the explicit
// StartHour/EndHour when they form a positive span, else the 8..18 working-day
// fallback so a zero-value Agenda still has a non-degenerate scale that never
// divides by zero.
func (a *Agenda) hourRange() (int, int) {
	s, e := a.StartHour, a.EndHour
	if e <= s {
		return 8, 18
	}
	return s, e
}

// agendaHourLabel formats an hour (0..24) as a zero-padded "HH:00" gutter label.
func agendaHourLabel(h int) string {
	hh := itoa(h)
	if h < 10 {
		hh = "0" + hh
	}
	return hh + ":00"
}

// agendaSelectInk is the tint blended over a Selected event's own Fill: the
// theme's Accent mixed lightly in so the block reads as highlighted while
// keeping a hint of its original colour.
func agendaSelectInk(fill RGBA, theme *Theme) RGBA {
	return blendRGBA(fill, theme.Accent, 0.4)
}

// blockRect computes the pixel rectangle of event ev with the grid's top-left
// origin at (ox, oy). ok is false when ev.Day is outside [0, len(DayNames)) —
// there is no column to place the block in. The block's width floors to 1 (so a
// narrow column still shows it) and its height floors to 1 after both ends are
// clamped to the visible hour range (so an event outside the range still leaves
// a visible sliver rather than vanishing). Draw calls it with the surface
// origin; OnEvent calls it with (0, 0) since events arrive in widget-local
// coordinates.
func (a *Agenda) blockRect(ox, oy int, ev AgendaEvent) (Rect, bool) {
	nDays := len(a.DayNames)
	if ev.Day < 0 || ev.Day >= nDays {
		return Rect{}, false
	}
	s, e := a.hourRange()
	gridX := ox + AgendaGutterW
	gridY := oy + AgendaHeaderH
	gridW := a.Bounds().W - AgendaGutterW
	nHours := e - s
	gridH := nHours * AgendaHourH
	startMin := s * 60
	endMin := e * 60
	visMin := nHours * 60

	colX := func(d int) int { return gridX + d*gridW/nDays }
	clampY := func(m int) int {
		if m < startMin {
			m = startMin
		}
		if m > endMin {
			m = endMin
		}
		return gridY + (m-startMin)*gridH/visMin
	}

	x0 := colX(ev.Day) + agendaBlockPadX
	w := colX(ev.Day+1) - colX(ev.Day) - 2*agendaBlockPadX
	if w < 1 {
		w = 1
	}
	y0 := clampY(ev.StartMin)
	h := clampY(ev.EndMin) - y0
	if h < 1 {
		h = 1
	}
	return Rect{X: x0, Y: y0, W: w, H: h}, true
}

// Draw paints the surface, the day-name header band, the hour-label gutter, the
// day-column × hour-row grid, and one rounded block per event positioned at its
// Day column and spanning its [StartMin, EndMin) range clamped to the visible
// hours. The Selected event is tinted and gets an accent border. The header,
// gutter and grid are each clipped so a long day name, hour label, title or an
// over-long block never bleeds across their boundaries.
func (a *Agenda) Draw(p painter.Painter, theme *Theme) {
	r := a.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)

	s, e := a.hourRange()
	nDays := len(a.DayNames)
	gridX := r.X + AgendaGutterW
	gridY := r.Y + AgendaHeaderH
	gridW := r.W - AgendaGutterW
	gridBottom := gridY + (e-s)*AgendaHourH
	colX := func(d int) int { return gridX + d*gridW/nDays }

	// Header band + gutter/grid separators.
	fillRect(p, r.X, r.Y, r.W, AgendaHeaderH, theme.SurfaceAlt)
	fillRect(p, r.X, r.Y+AgendaHeaderH-1, r.W, 1, theme.Border)
	fillRect(p, gridX, r.Y, 1, r.H, theme.Border)

	gridRect := Rect{X: gridX, Y: gridY, W: r.X + r.W - gridX, H: r.Y + r.H - gridY}

	// Hour rules across the grid + vertical day-column separators.
	withClip(p, gridRect, func() {
		for hr := s; hr <= e; hr++ {
			y := gridY + (hr-s)*AgendaHourH
			fillRect(p, gridX, y, gridW, 1, theme.Border)
		}
		// colX divides by nDays, so the column rules only run when there is at
		// least one day column to divide the grid into.
		if nDays > 0 {
			for d := 0; d <= nDays; d++ {
				fillRect(p, colX(d), gridY, 1, gridBottom-gridY, theme.Border)
			}
		}
	})

	// Day-name header labels, clipped to the header band.
	withClip(p, Rect{X: gridX, Y: r.Y, W: r.X + r.W - gridX, H: AgendaHeaderH}, func() {
		hy := r.Y + (AgendaHeaderH-a.glyphHeight())/2
		for d, name := range a.DayNames {
			cw := colX(d+1) - colX(d)
			a.drawText(p, colX(d)+(cw-a.textWidth(name))/2, hy, name, theme.OnSurface)
		}
	})

	// Hour labels down the gutter, clipped to the gutter area.
	withClip(p, Rect{X: r.X, Y: gridY, W: AgendaGutterW, H: r.Y + r.H - gridY}, func() {
		for hr := s; hr < e; hr++ {
			y := gridY + (hr-s)*AgendaHourH
			lbl := agendaHourLabel(hr)
			a.drawText(p, r.X+AgendaGutterW-4-a.textWidth(lbl), y+(AgendaHourH-a.glyphHeight())/2, lbl, dimInk(theme))
		}
	})

	// Event blocks, clipped to the grid so none bleed into header/gutter.
	withClip(p, gridRect, func() {
		for i, ev := range a.Events {
			br, ok := a.blockRect(r.X, r.Y, ev)
			if !ok {
				continue
			}
			fill := ev.Fill
			if fill == (RGBA{}) {
				fill = theme.Accent
			}
			if a.Selected == i {
				fill = agendaSelectInk(fill, theme)
			}
			fillRoundRect(p, br.X, br.Y, br.W, br.H, agendaBlockRadius, fill)
			if a.Selected == i {
				strokeRoundRect(p, br.X, br.Y, br.W, br.H, agendaBlockRadius, theme.Accent)
			}
			withClip(p, br, func() {
				a.drawText(p, br.X+agendaBlockPadX+2, br.Y+2, ev.Title, theme.Background)
			})
		}
	})
}

// OnEvent selects the event block under an EventClick and fires OnSelect
// (nil-safe) with its index. Clicks in the header, gutter, grid dead-space or
// anywhere off a block are no-ops, as is any non-click event. Overlapping
// blocks resolve to the visually-topmost (last-drawn) one.
func (a *Agenda) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	for i := len(a.Events) - 1; i >= 0; i-- {
		br, ok := a.blockRect(0, 0, a.Events[i])
		if ok && br.Contains(ev.X, ev.Y) {
			a.Selected = i
			if a.OnSelect != nil {
				a.OnSelect(i)
			}
			return
		}
	}
}
