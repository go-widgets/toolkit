// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"sort"

	"github.com/go-widgets/painter"
)

// ListBox is a vertical list of selectable string rows. Click on a
// row selects it + fires OnActivate.
//
// Visual: each row is RowHeight pixels tall. The selected row uses
// Theme.Accent as background + Theme.Background as ink; unselected
// rows use Theme.Surface + Theme.OnSurface. Rows are rendered via
// font.DrawText with a 4 px left margin.
//
// Multi-selection: setting MultiSelect enables Ctrl/Shift-modified
// clicks that build a set of selected rows (see IsSelected /
// SelectedIndices / SetSelection / ClearSelection / ToggleSelect /
// SelectRange). Selected remains the anchor/cursor row -- the point a
// Shift-click range is measured from, and the row most recently
// clicked (plain or Ctrl). When MultiSelect is false (the default)
// none of this is reachable: Ctrl/Shift are ignored and only Selected
// is ever highlighted, exactly as before this feature existed.
//
// Virtual scrolling: ListBox is self-contained -- it never relies on
// an outer ScrollView. ScrollRow is the index of the top visible row;
// Draw paints only the rows that fit in Bounds().H (windowed
// rendering, so a list with thousands of rows costs the same per
// frame as one with a handful), and OnEvent maps click coordinates
// back through ScrollRow so hit-testing stays correct while scrolled.
// See ScrollTo / ScrollBy. When every row already fits in the
// viewport (len(Items) <= the number of visible rows) rendering is
// byte-identical to a ListBox with no scrolling at all -- no
// scrollbar is drawn and the windowing has no visible effect.
type ListBox struct {
	Base
	Items       []string
	Selected    int // -1 = no selection; anchor/cursor row
	RowHeight   int // pixels per row; default 18 via NewListBox
	OnActivate  func(idx int)
	MultiSelect bool // enable Ctrl/Shift multi-row selection

	// ScrollRow is the index of the row painted at the very top of the
	// widget's bounds. Reads through Draw/OnEvent are clamped to
	// [0, maxScrollRow()] on the fly (see clampedScrollRow), so setting
	// this directly to an out-of-range value is safe -- it just behaves
	// as whichever in-range value it clamps to. Prefer ScrollTo/ScrollBy,
	// which clamp + write back immediately.
	ScrollRow int

	// selected holds the multi-selection set. Only consulted for
	// rendering/queries when MultiSelect is true, but the mutator
	// methods (SetSelection, ToggleSelect, SelectRange, ...) work
	// regardless of MultiSelect so callers can drive selection
	// programmatically before switching the widget into multi mode.
	selected map[int]bool
}

// NewListBox builds a ListBox containing items. Selected starts at
// -1 (no row selected) and RowHeight defaults to 18 (a comfortable
// 7-px font + 11 px vertical padding).
func NewListBox(items []string) *ListBox {
	return &ListBox{
		Items:     items,
		Selected:  -1,
		RowHeight: 18,
	}
}

// Draw paints only the rows currently within the scroll window --
// [ScrollRow, ScrollRow+visibleRows) -- positioning row i at
// top + (i-ScrollRow)*RowHeight. When every row already fits
// (len(Items) <= visibleRows() and ScrollRow clamps to 0), that
// window covers the whole list and rendering is byte-identical to a
// non-scrolling ListBox: no scrollbar, no clipping, full-width rows.
//
// When the list overflows the viewport, rows are clipped to the
// content area (via painter.Clipper, if the backend supports it) so
// a partially-visible trailing row never bleeds past Bounds().H, and
// a thin scrollbar track+thumb is painted on the right edge.
func (l *ListBox) Draw(p painter.Painter, theme *Theme) {
	r := l.Bounds()
	vr := l.visibleRows()
	overflow := len(l.Items) > vr

	cr := r // content rect: full bounds, minus the scrollbar column if any
	if overflow {
		cr.W -= scrollbarWidth
	}

	var clr painter.Clipper
	canClip := false
	if overflow {
		clr, canClip = p.(painter.Clipper)
		if canClip {
			clr.PushClip(cr)
		}
	}

	start := l.clampedScrollRow()
	end := start + vr
	if end > len(l.Items) {
		end = len(l.Items)
	}
	for i := start; i < end; i++ {
		item := l.Items[i]
		y := r.Y + (i-start)*l.RowHeight
		bg := theme.Surface
		ink := theme.OnSurface
		hi := i == l.Selected
		if l.MultiSelect {
			hi = l.IsSelected(i)
		}
		if hi {
			bg = theme.Accent
			ink = theme.Background
		}
		fillRect(p, cr.X, y, cr.W, l.RowHeight, bg)
		// Vertically centre the 7-px glyph inside the row.
		textY := y + (l.RowHeight-l.glyphHeight())/2
		l.drawText(p, cr.X+4, textY, item, ink)
	}

	if overflow && canClip {
		clr.PopClip()
	}
	if overflow {
		l.drawScrollbar(p, theme, r)
	}
}

// visibleRows is how many rows fit vertically within Bounds().H at
// RowHeight, rounded UP so a partially-visible trailing row still
// counts (Draw then clips it to the exact pixel boundary). A
// non-positive RowHeight or Bounds().H both collapse to 0 -- no rows
// fit, and callers must not divide by RowHeight in that case.
func (l *ListBox) visibleRows() int {
	if l.RowHeight <= 0 {
		return 0
	}
	h := l.Bounds().H
	if h <= 0 {
		return 0
	}
	n := h / l.RowHeight
	if h%l.RowHeight != 0 {
		n++
	}
	return n
}

// maxScrollRow is the highest ScrollRow that still leaves a full
// window of content on screen: len(Items) - visibleRows(), floored
// at 0 so a list that already fits the viewport never scrolls.
func (l *ListBox) maxScrollRow() int {
	m := len(l.Items) - l.visibleRows()
	if m < 0 {
		return 0
	}
	return m
}

// clampedScrollRow returns ScrollRow clamped to [0, maxScrollRow()]
// WITHOUT mutating the field. Draw + OnEvent read through this
// instead of ScrollRow directly, so an out-of-range value (set
// directly, or left stale after Items shrank) never paints or
// hit-tests outside the valid window.
func (l *ListBox) clampedScrollRow() int {
	s := l.ScrollRow
	if s < 0 {
		s = 0
	}
	if m := l.maxScrollRow(); s > m {
		s = m
	}
	return s
}

// ScrollTo moves the top visible row to row, clamped to
// [0, maxScrollRow()], and writes the clamped value back to
// ScrollRow.
func (l *ListBox) ScrollTo(row int) {
	l.ScrollRow = row
	l.ScrollRow = l.clampedScrollRow()
}

// ScrollBy shifts ScrollRow by delta rows (negative scrolls up),
// clamped exactly like ScrollTo.
func (l *ListBox) ScrollBy(delta int) {
	l.ScrollTo(l.ScrollRow + delta)
}

// scrollToSelected nudges ScrollRow so Selected stays within the
// visible window: scrolling up if Selected sits above ScrollRow,
// down if it sits at or past the last visible row. It is a no-op
// when nothing is selected (Selected < 0) so a fresh or
// selection-cleared list is never pulled to a bogus ScrollRow.
//
// ListBox has no built-in keyboard navigation today; this is exposed
// for a host (or a future arrow-key handler) that drives Selected
// externally and wants the list to keep it in view.
func (l *ListBox) scrollToSelected() {
	if l.Selected < 0 {
		return
	}
	if l.Selected < l.ScrollRow {
		l.ScrollTo(l.Selected)
		return
	}
	vr := l.visibleRows()
	if vr <= 0 {
		return
	}
	if l.Selected >= l.ScrollRow+vr {
		l.ScrollTo(l.Selected - vr + 1)
	}
}

// drawScrollbar paints the vertical scrollbar track (always, while
// overflowing) + a proportionally-sized thumb (while there's
// something to scroll) on the right edge of r -- the full widget
// bounds, not the shrunk content rect. Modelled on ScrollView's
// track+thumb proportion math, but driven by ScrollRow (whole rows)
// rather than a pixel offset, since ListBox only ever scrolls by
// full rows.
func (l *ListBox) drawScrollbar(p painter.Painter, theme *Theme, r Rect) {
	trackX := r.X + r.W - scrollbarWidth
	fillRect(p, trackX, r.Y, scrollbarWidth, r.H, theme.SurfaceAlt)

	contentH := len(l.Items) * l.RowHeight
	if r.H <= 0 || contentH <= r.H {
		return
	}
	thumbH := r.H * r.H / contentH
	if thumbH < 8 {
		thumbH = 8
	}
	thumbY := r.Y
	if max := l.maxScrollRow(); max > 0 {
		thumbY += l.clampedScrollRow() * (r.H - thumbH) / max
	}
	fillRect(p, trackX, thumbY, scrollbarWidth, thumbH, theme.Accent)
}

// OnEvent dispatches click events: a click at (X, Y) selects the row
// idx = ScrollRow + Y/RowHeight -- Y/RowHeight locates the row within
// the visible window, ScrollRow maps that back to an absolute Items
// index (clamped to the list length); OnActivate fires with that idx.
//
// When MultiSelect is false, a click simply moves Selected to idx --
// unchanged from the widget's original single-selection behaviour,
// and Ctrl/Shift are ignored entirely.
//
// When MultiSelect is true:
//   - a plain click selects ONLY idx (clearing any other selected
//     rows) and moves the anchor (Selected) to idx;
//   - a Ctrl-click toggles idx's membership in the selection set and
//     moves the anchor to idx;
//   - a Shift-click selects the inclusive range between the current
//     anchor (Selected) and idx, replacing the selection set, and
//     leaves the anchor itself unchanged so successive Shift-clicks
//     keep extending/shrinking from the same origin.
func (l *ListBox) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	if l.RowHeight <= 0 {
		return
	}
	if ev.Y < 0 { // Go truncates toward zero -- guard early.
		return
	}
	idx := l.clampedScrollRow() + ev.Y/l.RowHeight
	if idx >= len(l.Items) {
		return
	}

	if l.MultiSelect {
		switch {
		case ev.Shift:
			l.SelectRange(l.Selected, idx)
		case ev.Ctrl:
			l.ToggleSelect(idx)
			l.Selected = idx
		default:
			l.SetSelection(idx)
			l.Selected = idx
		}
	} else {
		l.Selected = idx
	}

	if l.OnActivate != nil {
		l.OnActivate(idx)
	}
}

// IsSelected reports whether row i is a member of the multi-selection
// set. It is independent of MultiSelect + Selected, so it can be
// queried (and pre-seeded via SetSelection/ToggleSelect/SelectRange)
// even before multi-selection is switched on.
func (l *ListBox) IsSelected(i int) bool {
	return l.selected[i]
}

// SelectedIndices returns the selected rows in ascending order. The
// returned slice is a fresh copy the caller may mutate freely.
func (l *ListBox) SelectedIndices() []int {
	out := make([]int, 0, len(l.selected))
	for i := range l.selected {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// SetSelection replaces the selection set with exactly the given
// indices. Indices outside [0, len(Items)) are silently dropped.
func (l *ListBox) SetSelection(indices ...int) {
	set := make(map[int]bool, len(indices))
	for _, i := range indices {
		if i < 0 || i >= len(l.Items) {
			continue
		}
		set[i] = true
	}
	l.selected = set
}

// ClearSelection empties the selection set. Selected (the
// anchor/cursor row) is left untouched.
func (l *ListBox) ClearSelection() {
	l.selected = nil
}

// ToggleSelect flips row i's membership in the selection set.
// Out-of-range indices are a no-op.
func (l *ListBox) ToggleSelect(i int) {
	if i < 0 || i >= len(l.Items) {
		return
	}
	if l.selected == nil {
		l.selected = make(map[int]bool)
	}
	if l.selected[i] {
		delete(l.selected, i)
	} else {
		l.selected[i] = true
	}
}

// SelectRange selects the inclusive range of rows between a and b
// (either order accepted), replacing the current selection set. The
// range is clamped to [0, len(Items)); if the list is empty, or the
// clamped range is inverted, the resulting selection is empty.
func (l *ListBox) SelectRange(a, b int) {
	if a > b {
		a, b = b, a
	}
	if a < 0 {
		a = 0
	}
	if b >= len(l.Items) {
		b = len(l.Items) - 1
	}
	set := make(map[int]bool)
	for i := a; i <= b; i++ {
		set[i] = true
	}
	l.selected = set
}
