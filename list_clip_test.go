// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"testing"

	"github.com/go-widgets/painter"
)

// paintedIn reports the first pixel in columns [x0, x1) of a w-wide, h-tall
// buffer that the draw touched (alpha != 0) on a freshly zeroed buffer, and
// whether one exists.
func paintedIn(buf []byte, w, h, x0, x1 int) (int, int, bool) {
	for py := 0; py < h; py++ {
		for px := x0; px < x1; px++ {
			if buf[(py*w+px)*4+3] != 0 {
				return px, py, true
			}
		}
	}
	return 0, 0, false
}

// A row wider than the list is clipped to the list's content rect, whether or
// not the list overflows vertically. Clipping used to be pushed only while the
// list overflowed (to stop a partial trailing row bleeding past the bottom), so
// a short list — a connection log with its first few lines, say — drew a long
// line straight past its right edge (#481).
func TestListBoxClipsRowsWiderThanTheList(t *testing.T) {
	long := strings.Repeat("a long log line ", 20)
	const bw, bh, listW = 400, 60, 80
	cases := []struct {
		name string
		lb   func() *ListBox
	}{
		{"flat, fits", func() *ListBox { return NewListBox([]string{long}) }},
		{"flat, overflows", func() *ListBox {
			return NewListBox([]string{long, long, long, long, long, long, long, long})
		}},
		{"sectioned, fits", func() *ListBox {
			return NewSectionedListBox(ListSection{Title: long, Items: []string{long}})
		}},
		{"item renderer, fits", func() *ListBox {
			lb := NewListBox([]string{long})
			// A host renderer that ignores its rect is held to it as well.
			lb.ItemRenderer = func(p painter.Painter, _ *Theme, r Rect, _ int, _ string, _ bool, ink RGBA) {
				fillRect(p, r.X, r.Y, r.W+200, r.H, ink)
			}
			return lb
		}},
	}
	for _, c := range cases {
		buf := make([]byte, 4*bw*bh)
		p := painter.NewPixelPainter(buf, bw, bh)
		lb := c.lb()
		lb.SetBounds(Rect{X: 0, Y: 0, W: listW, H: 40})
		lb.Draw(p, DefaultLight())
		if x, y, ok := paintedIn(buf, bw, bh, listW, bw); ok {
			t.Errorf("%s: pixel (%d,%d) painted right of the list's edge at x=%d", c.name, x, y, listW)
		}
		if _, _, ok := paintedIn(buf, bw, bh, 0, listW); !ok {
			t.Errorf("%s: nothing painted inside the list at all", c.name)
		}
	}
}
