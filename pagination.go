// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Pagination is a page-navigator strip: a "<" prev button, a series
// of page-number buttons, and a ">" next button. Clicking a page
// number Sets the Current Observable to that page; clicking prev
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
	focusState
	// Total is the page count (config). The reactive current page is MVVM-only:
	// it lives in an unexported Observable exposed via [Pagination.Current].
	Total int

	current *mvvm.Observable[int]
}

// Current is the active page as a shared [mvvm.Observable]: a host binds it
// (Set / Subscribe / two-way) — there is no settable Current field. A click on
// prev / next / a page number, or an arrow / Home / End key, Sets it (clamped to
// [1, Total]); subscribers are notified on change. A bare &Pagination{} lazily
// initialises the Observable to 0 on first access.
func (pg *Pagination) Current() *mvvm.Observable[int] {
	if pg.current == nil {
		pg.current = mvvm.NewObservable(0)
	}
	return pg.current
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

// btnW / btnH are the effective per-button pixel size used for both the drawn
// cell and the click hit-test: the scaled [PaginationBtnW]/[PaginationBtnH]
// clamped UP to the density minimum hit target via [TouchTarget]. Each button
// is a tap target, so at [DensityTouch] it grows to the finger floor (>=44
// device px) on both axes; under the default [DensityCompact] the clamp is a
// pass-through, so at MetricScale 1 the button is exactly the historical raw
// size. gap is [PaginationGap] in device pixels (no floor -- it is a spacer,
// not a target).
func (pg *Pagination) btnW() int { return TouchTarget(scaled(PaginationBtnW)) }
func (pg *Pagination) btnH() int { return TouchTarget(scaled(PaginationBtnH)) }
func (pg *Pagination) gap() int  { return scaled(PaginationGap) }

// paginationEllipsis is the label used in the collapsed window slots.
const paginationEllipsis = "..."

// NewPagination builds a Pagination with the given current and total
// page counts. Current is clamped to [1, Total] when Total > 0, and
// to 1 when Total <= 0 (the widget then renders empty and swallows
// events).
func NewPagination(current, total int) *Pagination {
	if total <= 0 {
		return &Pagination{Total: total, current: mvvm.NewObservable(1)}
	}
	if current < 1 {
		current = 1
	}
	if current > total {
		current = total
	}
	return &Pagination{Total: total, current: mvvm.NewObservable(current)}
}

// Draw paints the widget body, each button in its correct tint, and
// the button labels. Total <= 0 paints only the body — no buttons.
// Bounds that cannot accommodate a single button are treated the same
// as Total <= 0 so a mis-sized Pagination degrades gracefully.
func (pg *Pagination) Draw(p painter.Painter, theme *Theme) {
	r := pg.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	if pg.Total <= 0 || r.W < pg.btnW() || r.H < pg.btnH() {
		return
	}
	// The prev + slots + next strip has a natural width (set by the slot
	// count) that a narrow box can't hold; clip it to Bounds() so it truncates
	// on the right instead of spilling past the edge.
	withClip(p, r, func() {
		cur := pg.Current().Get()
		slots := pg.slots()
		x := r.X
		// Prev button.
		pg.drawStep(p, theme, x, r.Y, "<", cur > 1)
		x += pg.btnW() + pg.gap()
		// Numeric / ellipsis buttons.
		for _, slot := range slots {
			pg.drawSlot(p, theme, x, r.Y, slot)
			x += pg.btnW() + pg.gap()
		}
		// Next button.
		pg.drawStep(p, theme, x, r.Y, ">", cur < pg.Total)
	})
	pg.drawFocusRing(p, theme, r)
}

// drawStep paints one of the "<" / ">" step buttons. enabled=false
// renders the label in Border (disabled tone).
func (pg *Pagination) drawStep(p painter.Painter, theme *Theme, x, y int, label string, enabled bool) {
	bw, bh := pg.btnW(), pg.btnH()
	fillRect(p, x, y, bw, bh, theme.SurfaceAlt)
	strokeRect(p, x, y, bw, bh, theme.Border)
	ink := theme.OnSurface
	if !enabled {
		ink = theme.Border
	}
	tx := x + (bw-pg.textWidth(label))/2
	ty := y + (bh-pg.glyphHeight())/2
	pg.drawText(p, tx, ty, label, ink)
}

// drawSlot paints one numeric-or-ellipsis slot. The Current slot
// renders on Accent + accentInk; other numeric slots on Surface +
// OnSurface; the ellipsis on Surface + Border.
func (pg *Pagination) drawSlot(p painter.Painter, theme *Theme, x, y int, slot paginationSlot) {
	label := slot.label
	fill := theme.Surface
	ink := theme.OnSurface
	if slot.page > 0 && slot.page == pg.Current().Get() {
		fill = theme.Accent
		ink = accentInk(theme)
	} else if slot.page == 0 {
		ink = theme.Border
	}
	bw, bh := pg.btnW(), pg.btnH()
	fillRect(p, x, y, bw, bh, fill)
	strokeRect(p, x, y, bw, bh, theme.Border)
	tx := x + (bw-pg.textWidth(label))/2
	ty := y + (bh-pg.glyphHeight())/2
	pg.drawText(p, tx, ty, label, ink)
}

// OnEvent routes an EventClick to whichever button contains (X, Y).
// Prev/next step Current by one when enabled; a numeric slot sets
// Current to its page. Ellipsis slots and out-of-band clicks are
// no-ops.
func (pg *Pagination) OnEvent(ev Event) {
	if ev.Kind == EventKeyDown {
		if pg.Disabled || pg.Total <= 0 {
			return
		}
		// Left/Right step one page (like the prev/next buttons); Home/End jump to
		// the first/last page. Each reuses the same clamp+fireChange path.
		cur := pg.Current().Get()
		switch ev.Code {
		case "ArrowLeft", "ArrowUp":
			pg.goTo(cur - 1)
		case "ArrowRight", "ArrowDown":
			pg.goTo(cur + 1)
		case "Home":
			pg.goTo(1)
		case "End":
			pg.goTo(pg.Total)
		}
		return
	}
	if ev.Kind != EventClick {
		return
	}
	if pg.Total <= 0 {
		return
	}
	r := pg.Bounds()
	if ev.Y < 0 || ev.Y >= pg.btnH() || ev.Y >= r.H {
		return
	}
	stride := pg.btnW() + pg.gap()
	idx := ev.X / stride
	xOff := ev.X - idx*stride
	if xOff >= pg.btnW() {
		return // gap between buttons
	}
	cur := pg.Current().Get()
	slots := pg.slots()
	// Slot 0 is prev, slots [1..len(slots)] are numeric/ellipsis,
	// slot len(slots)+1 is next.
	switch {
	case idx == 0:
		if cur > 1 {
			pg.Current().Set(cur - 1)
		}
	case idx == len(slots)+1:
		if cur < pg.Total {
			pg.Current().Set(cur + 1)
		}
	case idx >= 1 && idx <= len(slots):
		slot := slots[idx-1]
		if slot.page > 0 && slot.page != cur {
			pg.Current().Set(slot.page)
		}
	}
}

// goTo clamps page to [1, Total] and Sets the Current Observable -- the shared
// mutate path the arrow / Home / End keys reuse, matching a prev/next/number
// click. Subscribers are notified on change; an unchanged page is a no-op (per
// mvvm.Observable), so a clamped or repeat key does not re-fire.
func (pg *Pagination) goTo(page int) {
	if page < 1 {
		page = 1
	}
	if page > pg.Total {
		page = pg.Total
	}
	pg.Current().Set(page)
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
	cur := pg.Current().Get()
	out := make([]paginationSlot, 0, paginationMaxButtons)
	switch {
	case cur <= 4:
		// Near-start: show 1..5, then "...", then Total.
		for i := 1; i <= 5; i++ {
			out = append(out, paginationSlot{label: strconv.Itoa(i), page: i})
		}
		out = append(out, paginationSlot{label: paginationEllipsis, page: 0})
		out = append(out, paginationSlot{label: strconv.Itoa(pg.Total), page: pg.Total})
	case cur >= pg.Total-3:
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
		for i := cur - 1; i <= cur+1; i++ {
			out = append(out, paginationSlot{label: strconv.Itoa(i), page: i})
		}
		out = append(out, paginationSlot{label: paginationEllipsis, page: 0})
		out = append(out, paginationSlot{label: strconv.Itoa(pg.Total), page: pg.Total})
	}
	return out
}
