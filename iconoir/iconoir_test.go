// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package iconoir

import (
	"slices"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

func inked(draw func(p painter.Painter, r toolkit.Rect, ink toolkit.RGBA)) bool {
	const sz = 24
	buf := make([]byte, sz*sz*4)
	draw(painter.NewPixelPainter(buf, sz, sz), toolkit.Rect{W: sz, H: sz}, toolkit.RGB(0x20, 0x20, 0x20))
	return slices.ContainsFunc(buf, func(b byte) bool { return b != 0 })
}

// TestDraw: every name of the full set — including ones the toolkit's own
// subset does not carry, such as "airplane" — paints ink, through Draw and
// through Drawer; an unknown name paints nothing.
func TestDraw(t *testing.T) {
	names := Names()
	if len(names) < 1000 {
		t.Fatalf("Names() has %d entries; the full Iconoir set has well over 1000", len(names))
	}
	if !slices.IsSorted(names) {
		t.Error("Names() is not sorted")
	}
	for _, n := range []string{names[0], "airplane", names[len(names)-1]} {
		if !inked(func(p painter.Painter, r toolkit.Rect, ink toolkit.RGBA) { Draw(p, r, n, ink) }) {
			t.Errorf("Draw(%q) painted no ink", n)
		}
		if !inked(Drawer(n)) {
			t.Errorf("Drawer(%q) painted no ink", n)
		}
	}
	if inked(Drawer("not-a-real-iconoir-stem")) {
		t.Error("an unknown name painted something")
	}
}
