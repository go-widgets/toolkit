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
type ListBox struct {
	Base
	Items       []string
	Selected    int // -1 = no selection; anchor/cursor row
	RowHeight   int // pixels per row; default 18 via NewListBox
	OnActivate  func(idx int)
	MultiSelect bool // enable Ctrl/Shift multi-row selection

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

// Draw paints every row inside the widget's bounds. Rows that fall
// outside the bounds (because the list is longer than the viewport)
// are still drawn but clipped per-pixel by the raster helpers; wrap
// a ScrollView around the ListBox for proper scrollable behaviour.
func (l *ListBox) Draw(p painter.Painter, theme *Theme) {
	r := l.Bounds()
	for i, item := range l.Items {
		y := r.Y + i*l.RowHeight
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
		fillRect(p, r.X, y, r.W, l.RowHeight, bg)
		// Vertically centre the 7-px glyph inside the row.
		textY := y + (l.RowHeight-l.glyphHeight())/2
		l.drawText(p, r.X+4, textY, item, ink)
	}
}

// OnEvent dispatches click events: a click at (X, Y) selects the
// row idx = Y / RowHeight (clamped to the list length); OnActivate
// fires with that idx.
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
	idx := ev.Y / l.RowHeight
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
