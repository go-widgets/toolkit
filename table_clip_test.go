// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// A cell whose text is wider than its column must not paint into the next
// column: the next column's pixels are the same whether the first cell holds
// a short text or a very long one.
func TestTableCellTextIsClippedToItsColumn(t *testing.T) {
	const w, h = 160, 90
	draw := func(first string) []byte {
		tb := NewTable([]TableColumn{{Title: "A", Width: 40}, {Title: "B", Width: 100}},
			[][]string{{first, ""}, {first, ""}})
		tb.SetBounds(Rect{W: w, H: h})
		surf := makeSurface(w, h)
		tb.Draw(newP(surf, w), DefaultLight())
		return surf
	}
	short, long := draw("a"), draw("WWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWW")
	differs := false
	for y := TableHeaderHeight; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(short, w, x, y) == pixelAt(long, w, x, y) {
				continue
			}
			if x >= 40 {
				t.Fatalf("the long text painted outside its column at (%d,%d)", x, y)
			}
			differs = true
		}
	}
	if !differs {
		t.Fatal("the long text painted nothing inside its own column")
	}
}
