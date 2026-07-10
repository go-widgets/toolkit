// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
)

func TestPieChartTotalIgnoresNegatives(t *testing.T) {
	c := NewPieChart([]float64{3, -5, 2})
	if got := c.total(); got != 5 {
		t.Errorf("total = %v, want 5 (negatives dropped)", got)
	}
}

func TestPieChartCumFractions(t *testing.T) {
	c := NewPieChart([]float64{1, 3}) // total 4 → 0.25, 1.0
	cum := c.cumFractions(c.total())
	if len(cum) != 2 || cum[0] != 0.25 || cum[1] != 1 {
		t.Errorf("cumFractions = %v, want [0.25 1]", cum)
	}
	// A negative value contributes nothing to the running share.
	c = NewPieChart([]float64{2, -9, 2}) // total 4 → 0.5, 0.5, 1.0
	cum = c.cumFractions(c.total())
	if cum[0] != 0.5 || cum[1] != 0.5 || cum[2] != 1 {
		t.Errorf("cumFractions w/ negative = %v, want [0.5 0.5 1]", cum)
	}
}

func TestSliceOf(t *testing.T) {
	cum := []float64{0.25, 0.75, 1.0}
	cases := []struct {
		f    float64
		want int
	}{
		{0.0, 0}, {0.24, 0}, {0.25, 1}, {0.5, 1}, {0.75, 2}, {0.99, 2},
		{1.0, 2}, // pinned at the boundary → last slice (fallthrough branch)
	}
	for _, tc := range cases {
		if got := sliceOf(cum, tc.f); got != tc.want {
			t.Errorf("sliceOf(%v) = %d, want %d", tc.f, got, tc.want)
		}
	}
}

func TestPieChartSliceColor(t *testing.T) {
	c := NewPieChart([]float64{1, 1})
	// Default palette cycles by index.
	if c.sliceColor(0) != piePalette[0] || c.sliceColor(len(piePalette)) != piePalette[0] {
		t.Error("default palette should cycle")
	}
	// Colors override cycles independently.
	red := RGBA{R: 255, A: 255}
	blue := RGBA{B: 255, A: 255}
	c.Colors = []RGBA{red, blue}
	if c.sliceColor(0) != red || c.sliceColor(1) != blue || c.sliceColor(2) != red {
		t.Error("Colors override should cycle")
	}
}

func TestPieChartDrawFillsWedges(t *testing.T) {
	// A two-equal-slice pie: the top-right quadrant is slice 0's colour, the
	// top-left is slice 1's — a clean clockwise-from-12 split at 6 o'clock.
	c := NewPieChart([]float64{1, 1})
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	surf := makeSurface(40, 40)
	c.Draw(newP(surf, 40), DefaultLight())

	// Slice 0 spans 12→6 o'clock on the RIGHT half; sample a right-of-centre px.
	if got := pixelAt(surf, 40, 28, 20); got != piePalette[0] {
		t.Errorf("right half = %+v, want slice-0 %+v", got, piePalette[0])
	}
	// Slice 1 spans 6→12 o'clock on the LEFT half.
	if got := pixelAt(surf, 40, 11, 20); got != piePalette[1] {
		t.Errorf("left half = %+v, want slice-1 %+v", got, piePalette[1])
	}
	// A corner pixel lies outside the disc → untouched sentinel (0xC8).
	if got := pixelAt(surf, 40, 0, 0); got != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 255}) {
		t.Errorf("corner outside disc = %+v, want sentinel", got)
	}
}

func TestPieChartEmptyAndZeroDrawNothing(t *testing.T) {
	th := DefaultLight()
	for _, vals := range [][]float64{nil, {0, 0}} {
		c := NewPieChart(vals)
		c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
		surf := makeSurface(40, 40)
		c.Draw(newP(surf, 40), th)
		if got := countInk(surf, 40, 40, piePalette[0]); got != 0 {
			t.Errorf("total<=0 drew %d px, want 0", got)
		}
	}
}

func TestPieChartTinyRadiusNoop(t *testing.T) {
	// A zero-size (sub-pixel radius) box bails before the fill loop.
	c := NewPieChart([]float64{1, 1})
	c.SetBounds(Rect{X: 0, Y: 0, W: 1, H: 1}) // radius = 0
	surf := makeSurface(4, 4)
	c.Draw(newP(surf, 4), DefaultLight())
	if got := countInk(surf, 4, 4, piePalette[0]); got != 0 {
		t.Errorf("radius<1 drew %d px, want 0", got)
	}
}
