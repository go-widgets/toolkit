// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
)

// countInk returns how many pixels in the w*h surface carry colour c.
func countInk(surf []byte, w, h int, c RGBA) int {
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(surf, w, x, y) == c {
				n++
			}
		}
	}
	return n
}

func TestLineChartYRange(t *testing.T) {
	// Explicit bounds win when Max > Min.
	c := &LineChart{Series: []float64{2, 8}, Min: 0, Max: 10}
	if mn, mx := c.yRange(); mn != 0 || mx != 10 {
		t.Errorf("explicit yRange = (%v,%v), want (0,10)", mn, mx)
	}
	// Auto range from the data spans its own min..max.
	c = NewLineChart([]float64{3, 1, 9, 4})
	if mn, mx := c.yRange(); mn != 1 || mx != 9 {
		t.Errorf("auto yRange = (%v,%v), want (1,9)", mn, mx)
	}
	// A flat series pads to ±1 so the line sits mid-plot.
	c = NewLineChart([]float64{5, 5, 5})
	if mn, mx := c.yRange(); mn != 4 || mx != 6 {
		t.Errorf("flat yRange = (%v,%v), want (4,6)", mn, mx)
	}
	// An empty series falls back to [0,1].
	c = NewLineChart(nil)
	if mn, mx := c.yRange(); mn != 0 || mx != 1 {
		t.Errorf("empty yRange = (%v,%v), want (0,1)", mn, mx)
	}
}

func TestLineChartDrawPolyline(t *testing.T) {
	c := NewLineChart([]float64{0, 10, 2, 8})
	c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 40})
	surf := makeSurface(60, 40)
	c.Draw(newP(surf, 60), DefaultLight())

	th := DefaultLight()
	// The axis frame drew Border pixels, and the polyline drew Accent pixels.
	if got := countInk(surf, 60, 40, th.Border); got == 0 {
		t.Error("no axis pixels drawn")
	}
	if got := countInk(surf, 60, 40, th.Accent); got == 0 {
		t.Error("no polyline pixels drawn")
	}
}

func TestLineChartSinglePointIsDot(t *testing.T) {
	c := NewLineChart([]float64{5})
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	surf := makeSurface(40, 30)
	c.Draw(newP(surf, 40), DefaultLight())
	// A lone point draws a 2x2 Accent dot (4 pixels), no connecting line.
	if got := countInk(surf, 40, 30, DefaultLight().Accent); got != 4 {
		t.Errorf("single-point dot = %d accent px, want 4", got)
	}
}

func TestLineChartEmptyDrawsAxesOnly(t *testing.T) {
	c := NewLineChart(nil)
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	surf := makeSurface(40, 30)
	c.Draw(newP(surf, 40), DefaultLight())
	th := DefaultLight()
	if got := countInk(surf, 40, 30, th.Border); got == 0 {
		t.Error("empty chart should still draw axes")
	}
	if got := countInk(surf, 40, 30, th.Accent); got != 0 {
		t.Errorf("empty chart drew %d accent px, want 0", got)
	}
}

func TestLineChartPointAtSingle(t *testing.T) {
	// With one value, pointAt must not divide by (n-1)==0; x pins to plot left.
	c := NewLineChart([]float64{7})
	c.SetBounds(Rect{X: 10, Y: 0, W: 40, H: 30})
	mn, mx := c.yRange()
	x, _ := c.pointAt(0, mn, mx)
	if x != c.plot().X {
		t.Errorf("single-point x = %d, want plot left %d", x, c.plot().X)
	}
}

func TestDrawLineDiagonalAndSteep(t *testing.T) {
	// Exercise both Bresenham branches (shallow dx>dy and steep dy>dx) plus a
	// reversed direction (sx/sy negative).
	surf := makeSurface(20, 20)
	p := newP(surf, 20)
	drawLine(p, 0, 0, 19, 5, RGBA{R: 1, A: 255})  // shallow, →
	drawLine(p, 19, 19, 5, 0, RGBA{G: 1, A: 255}) // steep, ← and ↑
	// A degenerate zero-length line paints exactly one pixel then returns.
	drawLine(p, 2, 2, 2, 2, RGBA{B: 1, A: 255})
	if got := pixelAt(surf, 20, 2, 2); got != (RGBA{B: 1, A: 255}) {
		t.Errorf("zero-length line pixel = %+v", got)
	}
}
