// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// A Frame's chrome -- title bar, border and padding -- is in logical pixels
// and follows MetricScale: at 2x each is exactly twice its 1x value, for the
// default padding and for an explicit one, and the child's box shrinks by
// exactly the doubled chrome.
func TestFrameChromeFollowsMetricScale(t *testing.T) {
	defer SetMetricScale(1)
	type chrome struct{ header, inset, measure int }
	at := func(scale float64, pad int) (chrome, Rect) {
		SetMetricScale(scale)
		f := NewFrame(nil)
		f.Title = "t"
		f.Padding = pad
		child := NewLabel("x")
		f.child = child
		f.SetBounds(Rect{X: 0, Y: 0, W: 400 * int(scale), H: 300 * int(scale)})
		return chrome{f.headerH(), f.inset(), f.Measure(400)}, child.Bounds()
	}
	for _, pad := range []int{0, 7} { // 0 = default padding
		c1, b1 := at(1, pad)
		c2, b2 := at(2, pad)
		if c2.header != 2*c1.header {
			t.Errorf("pad %d: header %d at 2x, want %d", pad, c2.header, 2*c1.header)
		}
		if c2.inset != 2*c1.inset { // border + padding together
			t.Errorf("pad %d: inset %d at 2x, want %d", pad, c2.inset, 2*c1.inset)
		}
		if b2.X != 2*b1.X || b2.Y != 2*b1.Y {
			t.Errorf("pad %d: child origin (%d,%d) at 2x, want (%d,%d)", pad, b2.X, b2.Y, 2*b1.X, 2*b1.Y)
		}
		if b2.W != 800-2*c2.inset || b1.W != 400-2*c1.inset {
			t.Errorf("pad %d: child width %d/%d does not match 2*inset", pad, b1.W, b2.W)
		}
	}
	c1, _ := at(1, 0)
	if c1.header != FrameTitleH || c1.inset != 1+defaultPadding {
		t.Errorf("1x chrome %+v, want header %d inset %d", c1, FrameTitleH, 1+defaultPadding)
	}
}
