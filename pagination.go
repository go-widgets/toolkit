// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

	"github.com/go-widgets/painter"
)

// Pagination is a page-navigator strip: a "<" prev button, a series
// of page-number buttons, and a ">" next button. Clicking a page
// number jumps Current to that page and fires OnChange; clicking prev
// or next steps by one (clamped). When Current is at either extreme
// the corresponding step button renders in a disabled tone and
// swallows clicks.
//
// When Total exceeds paginationMaxButtons the middle of the range
// collapses into a "1 ... k-1 k k+1 ... Total" window so the widget's
// footprint stays bounded. Non-numeric window slots ("...") are drawn
// but not clickable — the hit-test skips them.
type Pagination struct {
	Base
	Current  int
	Total    int
	OnChange func(page int)
}

// PaginationBtnW is the pixel width of each button (prev, next, and
// every page number).
const PaginationBtnW = 28

// PaginationBtnH is the pixel height of each button.
const PaginationBtnH = 24

// PaginationGap is the horizontal pixel gap between successive
// buttons.
const PaginationGap = 2

// paginationMaxButtons is the largest count of numeric page buttons
// rendered inline before the window heuristic kicks in.
const paginationMaxButtons = 7

// paginationEllipsis is the label used in the collapsed window slots.
const paginationEllipsis = "..."

// NewPagination builds a Pagination with the given current and total
// page counts. Current is clamped to [1, Total] when Total > 0, and
// to 1 when Total <= 0 (the widget then renders empty and swallows
// events).
func NewPagination(current, total int) *Pagination {
	if total <= 0 {
		return &Pagination{Current: 1, Total: total}
	}
	if current < 1 {
		current = 1
	}
	if current > total {
		current = total
	}
	return &Pagination{Current: current, Total: total}
}

// Draw paints the widget body, each button in its correct tint, and
// the button labels. Total <= 0 paints only the body — no buttons.
// Bounds that cannot accommodate a single button are treated the same
// as Total <= 0 so a mis-sized Pagination degrades gracefully.
func (pg *Pagination) Draw(p painter.Painter, theme *Theme) {
	r := pg.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	if pg.Total <= 0 || r.W < PaginationBtnW || r.H < PaginationBtnH {
		return
	}
	slots := pg.slots()
	x := r.X
	// Prev button.
	pg.drawStep(p, theme, x, r.Y, "<", pg.Current > 1)
	x += PaginationBtnW + PaginationGap
	// Numeric / ellipsis buttons.
	for _, slot := range slots {
		pg.drawSlot(p, theme, x, r.Y, slot)
		x += PaginationBtnW + PaginationGap
	}
	// Next button.
	pg.drawStep(p, theme, x, r.Y, ">", pg.Current < pg.Total)
}

// drawStep paints one of the "<" / ">" step buttons. enabled=false
// renders the label in Border (disabled tone).
func (pg *Pagination) drawStep(p painter.Painter, theme *Theme, x, y int, label string, enabled bool) {
	fillRect(p, x, y, PaginationBtnW, PaginationBtnH, theme.SurfaceAlt)
	strokeRect(p, x, y, PaginationBtnW, PaginationBtnH, theme.Border)
	ink := theme.OnSurface
	if !enabled {
		ink = theme.Border
	}
	tx := x + (PaginationBtnW-TextWidth(label))/2
	ty := y + (PaginationBtnH-GlyphHeight())/2
	DrawText(p, tx, ty, label, ink)
}

// drawSlot paints one numeric-or-ellipsis slot. The Current slot
// renders on Accent + accentInk; other numeric slots on Surface +
// OnSurface; the ellipsis on Surface + Border.
func (pg *Pagination) drawSlot(p painter.Painter, theme *Theme, x, y int, slot paginationSlot) {
	label := slot.label
	fill := theme.Surface
	ink := theme.OnSurface
	if slot.page > 0 && slot.page == pg.Current {
		fill = theme.Accent
		ink = accentInk(theme)
	} else if slot.page == 0 {
		ink = theme.Border
	}
	fillRect(p, x, y, PaginationBtnW, PaginationBtnH, fill)
	strokeRect(p, x, y, PaginationBtnW, PaginationBtnH, theme.Border)
	tx := x + (PaginationBtnW-TextWidth(label))/2
	ty := y + (PaginationBtnH-GlyphHeight())/2
	DrawText(p, tx, ty, label, ink)
}

// OnEvent routes an EventClick to whichever button contains (X, Y).
// Prev/next step Current by one when enabled; a numeric slot sets
// Current to its page. Ellipsis slots and out-of-band clicks are
// no-ops.
func (pg *Pagination) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	if pg.Total <= 0 {
		return
	}
	r := pg.Bounds()
	if ev.Y < 0 || ev.Y >= PaginationBtnH || ev.Y >= r.H {
		return
	}
	stride := PaginationBtnW + PaginationGap
	idx := ev.X / stride
	xOff := ev.X - idx*stride
	if xOff >= PaginationBtnW {
		return // gap between buttons
	}
	slots := pg.slots()
	// Slot 0 is prev, slots [1..len(slots)] are numeric/ellipsis,
	// slot len(slots)+1 is next.
	switch {
	case idx == 0:
		if pg.Current > 1 {
			pg.Current--
			pg.fireChange()
		}
	case idx == len(slots)+1:
		if pg.Current < pg.Total {
			pg.Current++
			pg.fireChange()
		}
	case idx >= 1 && idx <= len(slots):
		slot := slots[idx-1]
		if slot.page > 0 && slot.page != pg.Current {
			pg.Current = slot.page
			pg.fireChange()
		}
	}
}

// fireChange invokes OnChange when set.
func (pg *Pagination) fireChange() {
	if pg.OnChange != nil {
		pg.OnChange(pg.Current)
	}
}

// paginationSlot is one drawable/hit-testable slot in the numeric
// strip. page == 0 marks an ellipsis (not clickable); page > 0 marks
// a numeric page button.
type paginationSlot struct {
	label string
	page  int
}

// slots computes the numeric-strip layout. For small Total the strip
// is simply 1..Total. For Total > paginationMaxButtons the strip
// collapses to exactly paginationMaxButtons entries using one of
// three shapes:
//   - Current near the start: [1 2 3 4 5 ... Total]
//   - Current near the end:   [1 ... T-4 T-3 T-2 T-1 T]
//   - Current in the middle:  [1 ... k-1 k k+1 ... Total]
func (pg *Pagination) slots() []paginationSlot {
	if pg.Total <= paginationMaxButtons {
		out := make([]paginationSlot, 0, pg.Total)
		for i := 1; i <= pg.Total; i++ {
			out = append(out, paginationSlot{label: strconv.Itoa(i), page: i})
		}
		return out
	}
	out := make([]paginationSlot, 0, paginationMaxButtons)
	switch {
	case pg.Current <= 4:
		// Near-start: show 1..5, then "...", then Total.
		for i := 1; i <= 5; i++ {
			out = append(out, paginationSlot{label: strconv.Itoa(i), page: i})
		}
		out = append(out, paginationSlot{label: paginationEllipsis, page: 0})
		out = append(out, paginationSlot{label: strconv.Itoa(pg.Total), page: pg.Total})
	case pg.Current >= pg.Total-3:
		// Near-end: show 1, "...", then Total-4..Total.
		out = append(out, paginationSlot{label: "1", page: 1})
		out = append(out, paginationSlot{label: paginationEllipsis, page: 0})
		for i := pg.Total - 4; i <= pg.Total; i++ {
			out = append(out, paginationSlot{label: strconv.Itoa(i), page: i})
		}
	default:
		// Middle: 1, "...", k-1, k, k+1, "...", Total.
		out = append(out, paginationSlot{label: "1", page: 1})
		out = append(out, paginationSlot{label: paginationEllipsis, page: 0})
		for i := pg.Current - 1; i <= pg.Current+1; i++ {
			out = append(out, paginationSlot{label: strconv.Itoa(i), page: i})
		}
		out = append(out, paginationSlot{label: paginationEllipsis, page: 0})
		out = append(out, paginationSlot{label: strconv.Itoa(pg.Total), page: pg.Total})
	}
	return out
}
