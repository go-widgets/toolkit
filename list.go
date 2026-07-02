// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ListBox is a vertical list of selectable string rows. Click on a
// row selects it + fires OnActivate.
//
// Visual: each row is RowHeight pixels tall. The selected row uses
// Theme.Accent as background + Theme.Background as ink; unselected
// rows use Theme.Surface + Theme.OnSurface. Rows are rendered via
// font.DrawText with a 4 px left margin.
type ListBox struct {
	Base
	Items       []string
	Selected    int // -1 = no selection
	RowHeight   int // pixels per row; default 18 via NewListBox
	OnActivate  func(idx int)
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
		if i == l.Selected {
			bg = theme.Accent
			ink = theme.Background
		}
		fillRect(p, r.X, y, r.W, l.RowHeight, bg)
		// Vertically centre the 7-px glyph inside the row.
		textY := y + (l.RowHeight-GlyphHeight)/2
		DrawText(p, r.X+4, textY, item, ink)
	}
}

// OnEvent dispatches click events: a click at (X, Y) selects the
// row idx = Y / RowHeight (clamped to the list length); OnActivate
// fires with that idx.
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
	l.Selected = idx
	if l.OnActivate != nil {
		l.OnActivate(idx)
	}
}
