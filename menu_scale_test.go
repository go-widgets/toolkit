// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// TestMenuScaleDefaultsToOne checks the unset/negative Scale path is 1 (unscaled).
func TestMenuScaleDefaultsToOne(t *testing.T) {
	m := NewMenu(nil)
	if m.scale() != 1 {
		t.Errorf("default scale = %v, want 1", m.scale())
	}
	m.Scale = -3
	if m.scale() != 1 {
		t.Errorf("negative scale = %v, want 1", m.scale())
	}
	m.Scale = 2.5
	if m.scale() != 2.5 {
		t.Errorf("scale = %v, want 2.5", m.scale())
	}
}

// TestMenuScaleAndIcon covers the scaled metrics, the reserved icon gutter, the
// per-row icon callback, and — crucially — that a scaled menu's hit-test lands on
// the same rows its Draw laid out.
func TestMenuScaleAndIcon(t *testing.T) {
	var iconCells []Rect
	m := NewMenu([]MenuItem{
		{Label: "One", Action: func() {}, Icon: func(p painter.Painter, cell Rect, ink RGBA) {
			iconCells = append(iconCells, cell)
		}},
		{Separator: true},
		{Label: "Two", Action: func() {}},
	})
	m.Scale = 2

	if m.sc(MenuRowH) != 2*MenuRowH {
		t.Fatalf("sc(MenuRowH) = %d, want %d", m.sc(MenuRowH), 2*MenuRowH)
	}
	if !m.hasIconGutter() {
		t.Fatal("hasIconGutter should be true when an item carries an Icon")
	}
	if m.iconGutterW() != m.sc(MenuRowH) {
		t.Fatalf("iconGutterW = %d, want %d", m.iconGutterW(), m.sc(MenuRowH))
	}

	w, h := m.preferredSize()
	wantH := m.sc(4) + 2*m.sc(MenuRowH) + m.sc(MenuSeparatorH)
	if h != wantH {
		t.Fatalf("preferredSize h = %d, want %d (scaled)", h, wantH)
	}

	// Draw fires the Icon callback once (only the first row has one), in a scaled
	// square cell that sits inside the first row.
	m.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	m.Draw(newP(makeSurface(w, h), w), DefaultLight())
	if len(iconCells) != 1 {
		t.Fatalf("Icon callback fired %d times, want 1", len(iconCells))
	}
	cell := iconCells[0]
	if cell.W != m.sc(16) || cell.H != m.sc(16) {
		t.Errorf("icon cell = %+v, want %dx%d", cell, m.sc(16), m.sc(16))
	}
	if cell.Y < m.sc(2) || cell.Y+cell.H > m.sc(2)+m.sc(MenuRowH) {
		t.Errorf("icon cell %+v is not inside the first row", cell)
	}

	// Hit-test at scale: the middle of the first content row resolves to row 0,
	// and the third row (index 2, after the separator) resolves to 2.
	if got := m.rowAt(m.sc(2) + m.sc(MenuRowH)/2); got != 0 {
		t.Errorf("rowAt(first row) = %d, want 0", got)
	}
	y2 := m.sc(2) + m.sc(MenuRowH) + m.sc(MenuSeparatorH) + m.sc(MenuRowH)/2
	if got := m.rowAt(y2); got != 2 {
		t.Errorf("rowAt(third row) = %d, want 2", got)
	}
}
