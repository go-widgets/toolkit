// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestColorChooserNarrowHexClamp covers the hex left-clamp branch: in a box too
// narrow to right-align the 7-char hex, it clamps to the left edge and still
// paints inside Bounds.
func TestColorChooserNarrowHexClamp(t *testing.T) {
	c := NewColorChooser(RGB(0x0d, 0x94, 0x88))
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 90}) // narrower than the hex
	const w, h = 40, 90
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), DefaultLight())
	minX, _, maxX, _ := nbPaintedBBox(buf, w, h)
	if minX < 0 || maxX >= w {
		t.Fatalf("ColorChooser hex painted outside narrow box: X[%d..%d] w=%d", minX, maxX, w)
	}
}
