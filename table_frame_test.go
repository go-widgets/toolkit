// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestTableFrameAndRowLines: Framed strokes the outer edge and RowLines a
// line under each body row, both in theme.Border; neither is drawn by
// default.
func TestTableFrameAndRowLines(t *testing.T) {
	const w, h = 200, 120
	theme := DefaultLight()
	theme.Border = RGB(0x12, 0x34, 0x56)
	draw := func(framed, lines bool) []byte {
		tb := NewTable([]TableColumn{{Title: "A"}, {Title: "B"}}, [][]string{{"1", "2"}, {"3", "4"}})
		tb.Framed, tb.RowLines = framed, lines
		tb.SetBounds(Rect{W: w, H: h})
		buf := makeSurface(w, h)
		tb.Draw(newP(buf, w), theme)
		return buf
	}
	rowLine := scaled(TableHeaderHeight) + tb0RowH() - 1
	plain := draw(false, false)
	if pixelAt(plain, w, 0, h/2) == theme.Border || pixelAt(plain, w, w/4, rowLine) == theme.Border {
		t.Error("an unframed table drew a frame or row lines")
	}
	framed := draw(true, true)
	for _, pt := range [][2]int{{0, h / 2}, {w - 1, h / 2}, {w / 4, 0}, {w / 4, h - 1}, {w / 4, rowLine}} {
		if got := pixelAt(framed, w, pt[0], pt[1]); got != theme.Border {
			t.Errorf("(%d,%d) = %v, want the border", pt[0], pt[1], got)
		}
	}
	// An empty table is framed too.
	empty := NewTable([]TableColumn{{Title: "A"}}, nil)
	empty.Framed = true
	empty.SetBounds(Rect{W: w, H: h})
	buf := makeSurface(w, h)
	empty.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 0, h/2) != theme.Border || pixelAt(buf, w, w/2, h-1) != theme.Border {
		t.Error("an empty framed table has no frame")
	}
}

func tb0RowH() int {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"1"}})
	return tb.rowH()
}
