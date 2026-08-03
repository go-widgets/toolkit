// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// nbPaintedBBox returns the bounding box of all non-sentinel pixels.
func nbPaintedBBox(buf []byte, w, h int) (minX, minY, maxX, maxY int) {
	minX, minY, maxX, maxY = w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px := pixelAt(buf, w, x, y)
			if px.R != 0xC8 || px.G != 0xC8 || px.B != 0xC8 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	return
}

// TestNotebookNeverPaintsOutsideBounds is a PRECISE bounds-containment test: a
// notebook with more tabs than fit at the nominal 80px width (4 tabs in a 296px
// box — the gallery case) and an oversized chart page must not paint a single
// pixel outside its Bounds(), for every active tab. Regression for the
// fixed-80px-tab horizontal overflow.
func TestNotebookNeverPaintsOutsideBounds(t *testing.T) {
	r := Rect{X: 40, Y: 40, W: 296, H: 80}
	const w, h = 400, 200
	for _, active := range []int{0, 1, 2, 3} {
		nb := NewNotebook()
		nb.AddTab("Line", NewLineChart([]float64{3, 7, 2, 8, 5, 9, 4, 6}))
		nb.AddTab("Bar", NewBarChart([]float64{4, 7, 2, 8, 5, 3}))
		nb.AddTab("Pie", NewPieChart([]float64{3, 5, 2, 4, 1}))
		nb.AddTab("Docs", NewMarkdownView("# Charts\n\n- line\n- bar\n- pie"))
		nb.Active = active
		nb.SetBounds(r)
		buf := makeSurface(w, h)
		nb.Draw(newP(buf, w), DefaultLight())
		minX, minY, maxX, maxY := nbPaintedBBox(buf, w, h)
		if minX < r.X || minY < r.Y || maxX >= r.X+r.W || maxY >= r.Y+r.H {
			t.Fatalf("tab %d painted outside bounds %+v: X[%d..%d] Y[%d..%d]", active, r, minX, maxX, minY, maxY)
		}
	}
}

// TestNotebookTabsFitWidth checks tabW shrinks tabs so they never exceed the
// notebook width, and that with few tabs they keep the nominal width.
func TestNotebookTabsFitWidth(t *testing.T) {
	nb := NewNotebook()
	for i := 0; i < 4; i++ {
		nb.AddTab("T", NewLabel("x"))
	}
	nb.SetBounds(Rect{X: 0, Y: 0, W: 296, H: 80})
	// 4 tabs must fit: last tab's right edge <= width.
	last := nb.tabRect(3)
	if last.X+last.W > 296 {
		t.Fatalf("4 tabs overflow: last right edge %d > 296", last.X+last.W)
	}
	// Two tabs in a wide box keep the nominal width.
	nb2 := NewNotebook()
	nb2.AddTab("A", NewLabel("a"))
	nb2.AddTab("B", NewLabel("b"))
	nb2.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 80})
	if nb2.tabRect(0).W != NotebookTabWidth {
		t.Fatalf("wide box tab width = %d, want nominal %d", nb2.tabRect(0).W, NotebookTabWidth)
	}
	// An empty notebook's tabW returns the nominal width (guards div-by-zero).
	if NewNotebook().tabW() != NotebookTabWidth {
		t.Fatal("empty notebook tabW must return the nominal width")
	}
}
