// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
)

func TestBarChartTop(t *testing.T) {
	// Explicit positive Max wins.
	c := &BarChart{Values: []float64{2, 8}, Max: 20}
	if got := c.top(); got != 20 {
		t.Errorf("explicit top = %v, want 20", got)
	}
	// Auto: the tallest value.
	c = NewBarChart([]float64{3, 9, 4})
	if got := c.top(); got != 9 {
		t.Errorf("auto top = %v, want 9", got)
	}
	// All-zero (or empty) series falls back to a scale of 1.
	c = NewBarChart([]float64{0, 0})
	if got := c.top(); got != 1 {
		t.Errorf("zero-series top = %v, want 1", got)
	}
}

func TestBarChartDrawBars(t *testing.T) {
	c := NewBarChart([]float64{1, 4, 2, 8})
	c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 40})
	surf := makeSurface(60, 40)
	c.Draw(newP(surf, 60), DefaultLight())
	th := DefaultLight()
	if got := countInk(surf, 60, 40, th.Border); got == 0 {
		t.Error("no axis pixels drawn")
	}
	if got := countInk(surf, 60, 40, th.Accent); got == 0 {
		t.Error("no bar pixels drawn")
	}
}

func TestBarChartZeroValueSkipped(t *testing.T) {
	// A zero-height bar in the middle is skipped (continue branch); the two
	// non-zero bars still paint.
	c := NewBarChart([]float64{5, 0, 5})
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	surf := makeSurface(40, 30)
	c.Draw(newP(surf, 40), DefaultLight())
	if got := countInk(surf, 40, 30, DefaultLight().Accent); got == 0 {
		t.Error("non-zero bars should paint")
	}
}

func TestBarChartClampsAndMinHeights(t *testing.T) {
	// A value above Max clamps to full height; a tiny value floors to 1px.
	c := &BarChart{Values: []float64{100, 0.001}, Max: 1}
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	surf := makeSurface(40, 30)
	c.Draw(newP(surf, 40), DefaultLight())
	if got := countInk(surf, 40, 30, DefaultLight().Accent); got == 0 {
		t.Error("clamped + floored bars should paint")
	}
}

func TestBarChartEmptyDrawsAxesOnly(t *testing.T) {
	c := NewBarChart(nil)
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

func TestBarChartNarrowSlotFloors(t *testing.T) {
	// More bars than pixels: slot and bar width both floor to 1 (no divide-by-
	// zero, no zero-width fillRect).
	c := NewBarChart([]float64{1, 2, 3, 4, 5, 6, 7, 8})
	c.SetBounds(Rect{X: 0, Y: 0, W: ChartPad + 4, H: 20}) // ~4px plot for 8 bars
	surf := makeSurface(ChartPad+4, 20)
	c.Draw(newP(surf, ChartPad+4), DefaultLight())
	// Just assert it painted something without panicking.
	if got := countInk(surf, ChartPad+4, 20, DefaultLight().Accent); got == 0 {
		t.Error("narrow chart should still paint at least one bar")
	}
}
