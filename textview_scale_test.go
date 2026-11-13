// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// A text view at MetricScale 2 is the view at scale 1 with every length doubled:
// the glyph, the row spacing, the inset and the padding. Before, only the glyph
// scaled, and a cell drawn on a dense display had tighter rows than the same
// cell on a plain one.
func TestTextViewLayoutScalesWithTheMetricScale(t *testing.T) {
	defer SetMetricScale(MetricScale())
	src := "alpha\nbeta\ngamma\ndelta"
	measure := func(scale float64) (pref, pitch, glyph, caretY1, caretY2 int, line, col int) {
		SetMetricScale(scale)
		c := newFlowEditor(src, 600)
		glyph = c.glyphHeight()
		_, y0 := c.CaretPixel(0, 0)
		_, y1 := c.CaretPixel(1, 0)
		_, y2 := c.CaretPixel(2, 0)
		// a click in the middle of the third row lands on it
		line, col = c.caretAt(c.textLeftInset()+2, y2+glyph/2)
		return c.PreferredHeight().Get(), y1 - y0, glyph, y0, y2, line, col
	}
	p1, pitch1, g1, top1, _, line1, _ := measure(1)
	p2, pitch2, g2, top2, _, line2, _ := measure(2)
	if g2 != 2*g1 {
		t.Skipf("the active font does not scale exactly (glyph %d at 1, %d at 2)", g1, g2)
	}
	if pitch2 != 2*pitch1 {
		t.Errorf("row pitch %d at scale 2, %d at scale 1: want double", pitch2, pitch1)
	}
	if top2 != 2*top1 {
		t.Errorf("inset %d at scale 2, %d at scale 1: want double", top2, top1)
	}
	if p2 != 2*p1 {
		t.Errorf("preferred height %d at scale 2, %d at scale 1: want double", p2, p1)
	}
	if line1 != 2 || line2 != 2 {
		t.Errorf("a click in the third row hit row %d at scale 1 and %d at scale 2", line1, line2)
	}
	// and the historical layout is what scale 1 gives
	if pitch1 != g1+4 || top1 != 4 {
		t.Errorf("scale 1: pitch %d (want %d), inset %d (want 4)", pitch1, g1+4, top1)
	}
}
