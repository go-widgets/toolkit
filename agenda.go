// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// AgendaEvent is one appointment. In the week view (AgendaWeek) it draws as a
// coloured block: Title labels it, Day is the weekday column — an index in
// [0, len(DayNames)); events whose Day falls outside that range are skipped (no
// column to place them in) — and StartMin/EndMin are minutes-from-midnight
// (0..1440) bounding the block vertically as the half-open range
// [StartMin, EndMin), so EndMin must be greater than StartMin. In the calendar
// views (AgendaMonth/AgendaQuarter/AgendaYear) the event is placed by its
// absolute date instead: Y is the year, M the month (1..12) and D the
// day-of-month (1..31); Day/StartMin/EndMin are ignored there. Fill is the
// block/chip/dot colour — its zero value falls back to the theme's Accent so an
// event added without an explicit colour still paints in the app's palette.
type AgendaEvent struct {
	Title            string
	Day              int
	StartMin, EndMin int
	Y, M, D          int
	Fill             RGBA
}

// AgendaView selects which of the four calendar layouts an Agenda draws. The
// zero value AgendaWeek keeps the original day/time week grid (so a zero-value
// Agenda is byte-identical to before this type existed); the other three plot
// events by their absolute Y/M/D date.
type AgendaView int

const (
	// AgendaWeek is the day-column × hour-row week grid (the default).
	AgendaWeek AgendaView = iota
	// AgendaMonth is a single month grid: a weekday header, up to six week
	// rows of day cells, event chips per day and a "+N" overflow marker.
	AgendaMonth
	// AgendaQuarter is three compact month grids (Month, Month+1, Month+2)
	// side by side, each dotting the days that carry events.
	AgendaQuarter
	// AgendaYear is twelve very compact month grids for Year in a 4×3 layout,
	// each dotting the days that carry events.
	AgendaYear
)

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
	// OnDayActivate fires when a month-view click lands on an in-month day cell
	// that is not an event chip, carrying that cell's (year, month, day). A
	// host uses it to add an event on the clicked day. Nil is safe.
	OnDayActivate func(year, month, day int)

	// View selects the layout (week/month/quarter/year); zero = AgendaWeek.
	View AgendaView
	// Year and Month are the focused period for the calendar views. When a
	// needed field is zero it is derived from the first dated event (see
	// focusYM/focusYear); if none is available the calendar grid stays empty.
	Year  int
	Month int // 1..12
}

// Agenda sizing constants, exported like TableRowHeight / GanttHeaderH so a host
// can measure the widget before it has a surface: AgendaHeaderH + hours*
// AgendaHourH gives the natural height and AgendaGutterW is the fixed hour-label
// gutter width.
const (
	// AgendaHeaderH is the pixel height of the day-name header row (also the
	// weekday header in the month view).
	AgendaHeaderH = 24
	// AgendaHourH is the pixel height of one hour row in the grid.
	AgendaHourH = 32
	// AgendaGutterW is the pixel width of the left hour-label gutter.
	AgendaGutterW = 48
	// AgendaDayCellH is the pixel height of one day cell in the month view.
	AgendaDayCellH = 56
	// AgendaMiniMonthGap is the pixel gap between mini month grids in the
	// quarter and year views.
	AgendaMiniMonthGap = 12
)

// agendaChipH is the pixel height of one event chip in a month-view day cell.
const agendaChipH = 12

// agendaChipRadius is the corner radius of a month-view event chip.
const agendaChipRadius = 2

// agendaDotR is the radius of the event marker dot in the quarter/year mini
// months.
const agendaDotR = 2

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

// Draw dispatches to the active View's painter. AgendaWeek (the zero value)
// draws the original week grid unchanged; the calendar views draw a month,
// quarter or year of dated events.
func (a *Agenda) Draw(p painter.Painter, theme *Theme) {
	switch a.View {
	case AgendaMonth:
		a.drawMonth(p, theme)
	case AgendaQuarter, AgendaYear:
		a.drawMini(p, theme)
	default:
		a.drawWeek(p, theme)
	}
}

// drawWeek paints the surface, the day-name header band, the hour-label gutter,
// the day-column × hour-row grid, and one rounded block per event positioned at
// its Day column and spanning its [StartMin, EndMin) range clamped to the
// visible hours. The Selected event is tinted and gets an accent border. The
// header, gutter and grid are each clipped so a long day name, hour label,
// title or an over-long block never bleeds across their boundaries.
func (a *Agenda) drawWeek(p painter.Painter, theme *Theme) {
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

// OnEvent selects the event under an EventClick in whatever View is active and
// fires OnSelect (nil-safe) with its index. Each view has a hit-test that shares
// its paint geometry so the two can't drift: hitWeek walks the day/time blocks,
// hitMonth the day-cell chips, hitMini the mini-month day cells. Clicks that
// land on no event — headers, gutters, dead-space, "+N" markers — and any
// non-click event are no-ops. Overlapping candidates resolve to the
// visually-topmost (last-drawn) one.
func (a *Agenda) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	var i int
	switch a.View {
	case AgendaMonth:
		i = a.hitMonth(ev.X, ev.Y)
	case AgendaQuarter, AgendaYear:
		i = a.hitMini(ev.X, ev.Y)
	default:
		i = a.hitWeek(ev.X, ev.Y)
	}
	if i < 0 {
		// A month-view click that hits no chip may still land on an in-month
		// day cell: activate that day (a host adds an event there). Other
		// views have no day-activate target.
		if a.View == AgendaMonth && a.OnDayActivate != nil {
			if yy, m, d, ok := a.monthDayAt(ev.X, ev.Y); ok {
				a.OnDayActivate(yy, m, d)
			}
		}
		return
	}
	a.Selected = i
	if a.OnSelect != nil {
		a.OnSelect(i)
	}
}

// DayAt maps widget-local (x, y) to the in-month day cell under it in the month
// view, returning the focused (year, month, day) and ok=true; ok=false for the
// header, outside the grid, or a spill cell. Exposed so a host can hit-test a
// right-click and offer an "add event here" menu. Only meaningful when
// View == AgendaMonth.
func (a *Agenda) DayAt(x, y int) (year, month, day int, ok bool) { return a.monthDayAt(x, y) }

// monthDayAt maps widget-local (x, y) to the in-month day cell under it,
// returning the focused (year, month, day) and ok=true. It returns ok=false
// for the weekday header band, points outside the day grid, columns that miss
// every cell, and the leading/trailing spill cells that belong to the adjacent
// month. It walks the same geometry drawMonth paints, in local coordinates
// (origin at the widget's top-left), matching hitMonth.
func (a *Agenda) monthDayAt(x, y int) (int, int, int, bool) {
	yy, m, ok := a.focusYM()
	if !ok {
		return 0, 0, 0, false
	}
	if y < AgendaHeaderH {
		return 0, 0, 0, false
	}
	W := a.Bounds().W
	first := WeekdayOfFirst(yy, m)
	dim := DaysInMonth(yy, m)
	rows := (first + dim + 6) / 7
	row := (y - AgendaHeaderH) / AgendaDayCellH
	if row >= rows {
		return 0, 0, 0, false
	}
	col := -1
	for c := 0; c < 7; c++ {
		cx := monthColX(0, W, c)
		if x >= cx && x < monthColX(0, W, c+1) {
			col = c
			break
		}
	}
	if col < 0 {
		return 0, 0, 0, false
	}
	idx := row*7 + col
	if idx < first || idx >= first+dim {
		return 0, 0, 0, false
	}
	return yy, m, idx - first + 1, true
}

// hitWeek returns the index of the topmost week-view block under (x, y) in
// widget-local coordinates, or -1 when the point hits no block.
func (a *Agenda) hitWeek(x, y int) int {
	for i := len(a.Events) - 1; i >= 0; i-- {
		br, ok := a.blockRect(0, 0, a.Events[i])
		if ok && br.Contains(x, y) {
			return i
		}
	}
	return -1
}

// --- shared calendar helpers ---------------------------------------------

// focusYM resolves the (year, month) the month and quarter views centre on: the
// explicit Year/Month when both are set, else the date of the first event that
// carries one, else ok=false so the caller draws an empty grid.
func (a *Agenda) focusYM() (year, month int, ok bool) {
	if a.Year != 0 && a.Month >= 1 && a.Month <= 12 {
		return a.Year, a.Month, true
	}
	for _, ev := range a.Events {
		if ev.Y != 0 && ev.M >= 1 && ev.M <= 12 {
			return ev.Y, ev.M, true
		}
	}
	return 0, 0, false
}

// focusYear resolves the year the year view spans: the explicit Year, else the
// first event's year, else ok=false for an empty grid.
func (a *Agenda) focusYear() (year int, ok bool) {
	if a.Year != 0 {
		return a.Year, true
	}
	for _, ev := range a.Events {
		if ev.Y != 0 {
			return ev.Y, true
		}
	}
	return 0, false
}

// addMonth advances (year, month) by delta months (delta >= 0), rolling the
// year forward as the month passes December.
func addMonth(year, month, delta int) (int, int) {
	t := month - 1 + delta
	return year + t/12, t%12 + 1
}

// prevMonth returns the (year, month) immediately before (year, month).
func prevMonth(year, month int) (int, int) {
	if month == 1 {
		return year - 1, 12
	}
	return year, month - 1
}

// dayEvent reports whether any event falls on (y, m, d) and returns the fill of
// the first such event (its zero value left for the caller to resolve against
// the theme accent).
func (a *Agenda) dayEvent(y, m, d int) (RGBA, bool) {
	for _, ev := range a.Events {
		if ev.Y == y && ev.M == m && ev.D == d {
			return ev.Fill, true
		}
	}
	return RGBA{}, false
}

// resolveFill returns fill unless it is the zero value, in which case it falls
// back to the theme accent — the same rule the week blocks use.
func resolveFill(fill RGBA, theme *Theme) RGBA {
	if fill == (RGBA{}) {
		return theme.Accent
	}
	return fill
}

// --- month view -----------------------------------------------------------

// agendaChip is one event chip in the month view: idx indexes into Events and
// rect is its pixel rectangle. monthChips builds them so drawMonth and hitMonth
// share one geometry.
type agendaChip struct {
	idx  int
	rect Rect
}

// agendaOverflow is a "+N" marker drawn when a day holds more events than its
// cell can show as chips.
type agendaOverflow struct {
	rect Rect
	n    int
}

// monthColX returns the x of month-grid column c (0..7) for a grid whose left
// edge is ox and whose total width is w, matching the week view's integer
// column maths so cells tile without gaps.
func monthColX(ox, w, c int) int { return ox + c*w/7 }

// monthChips computes the event chips and overflow markers for the focused
// month with the grid's top-left origin at (ox, oy). It returns nothing when no
// month is in focus. Chips stack from the top of each day cell below the day
// number; when a day has more events than fit, the last slot becomes a "+N"
// overflow marker. Draw passes the surface origin; hitMonth passes (0, 0).
func (a *Agenda) monthChips(ox, oy int) (chips []agendaChip, overflows []agendaOverflow) {
	y, m, ok := a.focusYM()
	if !ok {
		return nil, nil
	}
	w := a.Bounds().W
	first := WeekdayOfFirst(y, m)
	dim := DaysInMonth(y, m)
	cellsY := oy + AgendaHeaderH
	top := a.glyphHeight() + 4
	slot := agendaChipH + 2
	maxSlots := (AgendaDayCellH - top) / slot

	byDay := make([][]int, dim+1)
	for i, ev := range a.Events {
		if ev.Y == y && ev.M == m && ev.D >= 1 && ev.D <= dim {
			byDay[ev.D] = append(byDay[ev.D], i)
		}
	}
	for day := 1; day <= dim; day++ {
		evs := byDay[day]
		if len(evs) == 0 {
			continue
		}
		idx := first + day - 1
		col, row := idx%7, idx/7
		cellX := monthColX(ox, w, col)
		cellW := monthColX(ox, w, col+1) - cellX
		cellY := cellsY + row*AgendaDayCellH
		shown, ofN := len(evs), 0
		if shown > maxSlots {
			shown = maxSlots - 1
			ofN = len(evs) - shown
		}
		for k := 0; k < shown; k++ {
			cy := cellY + top + k*slot
			chips = append(chips, agendaChip{idx: evs[k], rect: Rect{X: cellX + 2, Y: cy, W: cellW - 4, H: agendaChipH}})
		}
		if ofN > 0 {
			cy := cellY + top + shown*slot
			overflows = append(overflows, agendaOverflow{rect: Rect{X: cellX + 2, Y: cy, W: cellW - 4, H: agendaChipH}, n: ofN})
		}
	}
	return chips, overflows
}

// drawMonth paints the month view: a weekday header row, up to six week rows of
// day cells (leading/trailing days from the adjacent months dimmed), each day's
// event chips and a "+N" overflow marker. With no month in focus it draws just
// the surface and the weekday header. The grid is clipped so chips and numbers
// never bleed past it.
func (a *Agenda) drawMonth(p painter.Painter, theme *Theme) {
	r := a.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)

	// Weekday header band + labels.
	fillRect(p, r.X, r.Y, r.W, AgendaHeaderH, theme.SurfaceAlt)
	fillRect(p, r.X, r.Y+AgendaHeaderH-1, r.W, 1, theme.Border)
	hy := r.Y + (AgendaHeaderH-a.glyphHeight())/2
	withClip(p, Rect{X: r.X, Y: r.Y, W: r.W, H: AgendaHeaderH}, func() {
		for c := 0; c < 7; c++ {
			name := ""
			if c < len(a.DayNames) {
				name = a.DayNames[c]
			}
			cx := monthColX(r.X, r.W, c)
			cw := monthColX(r.X, r.W, c+1) - cx
			a.drawText(p, cx+(cw-a.textWidth(name))/2, hy, name, theme.OnSurface)
		}
	})

	y, m, ok := a.focusYM()
	if !ok {
		return
	}
	first := WeekdayOfFirst(y, m)
	dim := DaysInMonth(y, m)
	rows := (first + dim + 6) / 7
	cellsY := r.Y + AgendaHeaderH
	py, pm := prevMonth(y, m)
	pdim := DaysInMonth(py, pm)

	gridRect := Rect{X: r.X, Y: cellsY, W: r.W, H: rows * AgendaDayCellH}
	// Never let the grid clip (and thus the cells) extend past the widget's
	// own bottom edge: a box shorter than the natural six-row height must clip
	// the overflowing rows, not paint outside Bounds().
	if bottom := r.Y + r.H; gridRect.Y+gridRect.H > bottom {
		gridRect.H = bottom - gridRect.Y
	}
	withClip(p, gridRect, func() {
		for row := 0; row < rows; row++ {
			for col := 0; col < 7; col++ {
				idx := row*7 + col
				cellX := monthColX(r.X, r.W, col)
				cellW := monthColX(r.X, r.W, col+1) - cellX
				cellY := cellsY + row*AgendaDayCellH
				strokeRect(p, cellX, cellY, cellW, AgendaDayCellH, theme.Border)
				var num int
				ink := theme.OnSurface
				switch {
				case idx < first:
					num = pdim - first + 1 + idx
					ink = dimInk(theme)
				case idx >= first+dim:
					num = idx - (first + dim) + 1
					ink = dimInk(theme)
				default:
					num = idx - first + 1
				}
				a.drawText(p, cellX+3, cellY+2, itoa(num), ink)
			}
		}
		chips, overflows := a.monthChips(r.X, r.Y)
		for _, c := range chips {
			ev := a.Events[c.idx]
			fill := resolveFill(ev.Fill, theme)
			if a.Selected == c.idx {
				fill = agendaSelectInk(fill, theme)
			}
			fillRoundRect(p, c.rect.X, c.rect.Y, c.rect.W, c.rect.H, agendaChipRadius, fill)
			if a.Selected == c.idx {
				strokeRoundRect(p, c.rect.X, c.rect.Y, c.rect.W, c.rect.H, agendaChipRadius, theme.Accent)
			}
			withClip(p, c.rect, func() {
				a.drawText(p, c.rect.X+2, c.rect.Y+(agendaChipH-a.glyphHeight())/2, ev.Title, theme.Background)
			})
		}
		for _, o := range overflows {
			withClip(p, o.rect, func() {
				a.drawText(p, o.rect.X+2, o.rect.Y+(agendaChipH-a.glyphHeight())/2, "+"+itoa(o.n), dimInk(theme))
			})
		}
	})
}

// hitMonth returns the index of the topmost chip under (x, y) in widget-local
// coordinates, or -1. "+N" overflow markers are not selectable.
func (a *Agenda) hitMonth(x, y int) int {
	chips, _ := a.monthChips(0, 0)
	for i := len(chips) - 1; i >= 0; i-- {
		if chips[i].rect.Contains(x, y) {
			return chips[i].idx
		}
	}
	return -1
}

// --- quarter & year views (mini months) -----------------------------------

// miniSpec is one mini month in the quarter/year grid.
type miniSpec struct {
	y, m int
}

// miniLayout returns the mini months to draw and the (cols, rows) they tile
// into: three-across for the quarter view starting at the focused month, and a
// 4×3 grid of the twelve months for the year view. It returns no specs when no
// period is in focus.
func (a *Agenda) miniLayout() (specs []miniSpec, cols, rows int) {
	if a.View == AgendaYear {
		yy, ok := a.focusYear()
		if !ok {
			return nil, 4, 3
		}
		for m := 1; m <= 12; m++ {
			specs = append(specs, miniSpec{y: yy, m: m})
		}
		return specs, 4, 3
	}
	y, m, ok := a.focusYM()
	if !ok {
		return nil, 3, 1
	}
	for d := 0; d < 3; d++ {
		yy, mm := addMonth(y, m, d)
		specs = append(specs, miniSpec{y: yy, m: mm})
	}
	return specs, 3, 1
}

// miniMonthBox returns the pixel rectangle of the idx-th mini month, tiled left
// to right then top to bottom over cols×rows with AgendaMiniMonthGap between
// them, with the region's top-left at (ox, oy).
func (a *Agenda) miniMonthBox(ox, oy, idx, cols, rows int) Rect {
	W, H := a.Bounds().W, a.Bounds().H
	gap := AgendaMiniMonthGap
	mw := (W - (cols-1)*gap) / cols
	mh := (H - (rows-1)*gap) / rows
	c, row := idx%cols, idx/cols
	return Rect{X: ox + c*(mw+gap), Y: oy + row*(mh+gap), W: mw, H: mh}
}

// drawMini paints the quarter or year view: a grid of mini months, each with a
// centred month title, its day numbers and a coloured dot on every day that
// carries an event. Each mini month is clipped to its box.
func (a *Agenda) drawMini(p painter.Painter, theme *Theme) {
	r := a.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	specs, cols, rows := a.miniLayout()
	for i, sp := range specs {
		box := a.miniMonthBox(r.X, r.Y, i, cols, rows)
		withClip(p, box, func() {
			a.drawMiniMonth(p, theme, box, sp)
		})
	}
}

// drawMiniMonth paints one mini month inside box: title, day numbers, event
// dots.
func (a *Agenda) drawMiniMonth(p painter.Painter, theme *Theme, box Rect, sp miniSpec) {
	titleH := a.glyphHeight() + 6
	title := monthName(sp.m)
	a.drawText(p, box.X+(box.W-a.textWidth(title))/2, box.Y+3, title, theme.OnSurface)

	first := WeekdayOfFirst(sp.y, sp.m)
	dim := DaysInMonth(sp.y, sp.m)
	gridTop := box.Y + titleH
	cellW := box.W / 7
	cellH := (box.H - titleH) / 6
	for day := 1; day <= dim; day++ {
		idx := first + day - 1
		col, row := idx%7, idx/7
		cx := box.X + col*cellW
		cy := gridTop + row*cellH
		a.drawText(p, cx+(cellW-a.textWidth(itoa(day)))/2, cy, itoa(day), theme.OnSurface)
		if fill, has := a.dayEvent(sp.y, sp.m, day); has {
			// Sit the marker just under the day number so it reads as
			// belonging to this cell rather than crowding the next row.
			dx := cx + cellW/2 - agendaDotR
			dy := cy + a.glyphHeight() + 1
			fillRoundRect(p, dx, dy, 2*agendaDotR, 2*agendaDotR, agendaDotR, resolveFill(fill, theme))
		}
	}
}

// hitMini returns the index of the topmost event on the mini-month day cell
// under (x, y) in widget-local coordinates, or -1 when the point hits no dated
// day cell.
func (a *Agenda) hitMini(x, y int) int {
	specs, cols, rows := a.miniLayout()
	for i, sp := range specs {
		box := a.miniMonthBox(0, 0, i, cols, rows)
		if !box.Contains(x, y) {
			continue
		}
		titleH := a.glyphHeight() + 6
		gridTop := box.Y + titleH
		cellW := box.W / 7
		cellH := (box.H - titleH) / 6
		if cellW <= 0 || cellH <= 0 || y < gridTop {
			return -1
		}
		// box.Contains guarantees x >= box.X and the y < gridTop guard above
		// keeps y >= gridTop, so col and row are non-negative here; only the
		// far edges (remainder columns/rows past the 7×6 grid) can overshoot.
		col := (x - box.X) / cellW
		row := (y - gridTop) / cellH
		if col > 6 || row > 5 {
			return -1
		}
		idx := row*7 + col
		day := idx - WeekdayOfFirst(sp.y, sp.m) + 1
		if day < 1 || day > DaysInMonth(sp.y, sp.m) {
			return -1
		}
		for j := len(a.Events) - 1; j >= 0; j-- {
			ev := a.Events[j]
			if ev.Y == sp.y && ev.M == sp.m && ev.D == day {
				return j
			}
		}
		return -1
	}
	return -1
}
