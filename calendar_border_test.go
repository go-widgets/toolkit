// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestCalendarBorderIntact is a PRECISE test: after Draw, every pixel along all
// four edges of the Calendar's bounds must be the Border colour. Regression for
// the day-cell fills (which start at r.X for column 0 and can reach the right/
// bottom edges) erasing the frame when the border was stroked first.
func TestCalendarBorderIntact(t *testing.T) {
	theme := DefaultLight()
	r := Rect{X: 20, Y: 20, W: 220, H: 180}
	const w, h = 280, 240
	c := NewCalendar(2026, 7, 2) // a month whose 1st + selected day land in col 0 somewhere
	c.SetToday(2026, 7, 2)
	c.SetBounds(r)
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)

	bad := func(x, y int) bool { return pixelAt(buf, w, x, y) != theme.Border }
	// Left + right edges (every row).
	for y := r.Y; y < r.Y+r.H; y++ {
		if bad(r.X, y) {
			t.Fatalf("left border broken at y=%d: %+v", y, pixelAt(buf, w, r.X, y))
		}
		if bad(r.X+r.W-1, y) {
			t.Fatalf("right border broken at y=%d", y)
		}
	}
	// Top + bottom edges (every column).
	for x := r.X; x < r.X+r.W; x++ {
		if bad(x, r.Y) {
			t.Fatalf("top border broken at x=%d", x)
		}
		if bad(x, r.Y+r.H-1) {
			t.Fatalf("bottom border broken at x=%d", x)
		}
	}
}
