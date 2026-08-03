// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"os"
	"testing"
)

func TestScatterChartRanges(t *testing.T) {
	// Multi-point, multi-series: covers min/max updates on both axes.
	c := NewScatterChart([][]ScatterPoint{
		{{X: 2, Y: 5}, {X: 8, Y: 1}},
		{{X: 1, Y: 3}, {X: 6, Y: 9}},
	})
	xr, yr, seen := c.ranges()
	if !seen || xr != [2]float64{1, 8} || yr != [2]float64{1, 9} {
		t.Errorf("ranges = x%v y%v seen=%v, want x[1 8] y[1 9] true", xr, yr, seen)
	}
	// Empty: seen is false, spans untouched.
	c = NewScatterChart(nil)
	if _, _, seen := c.ranges(); seen {
		t.Error("empty ranges should report seen=false")
	}
	// A single point makes both spans flat -> each pads by ±1.
	c = NewScatterChart([][]ScatterPoint{{{X: 4, Y: 7}}})
	xr, yr, seen = c.ranges()
	if !seen || xr != [2]float64{3, 5} || yr != [2]float64{6, 8} {
		t.Errorf("single-point ranges = x%v y%v, want x[3 5] y[6 8]", xr, yr)
	}
}

func TestScatterChartSeriesColor(t *testing.T) {
	c := NewScatterChart(nil)
	if c.seriesColor(0) != piePalette[0] || c.seriesColor(len(piePalette)) != piePalette[0] {
		t.Error("default series colour should cycle the shared palette")
	}
	c.Colors = []RGBA{{R: 1, A: 255}, {G: 1, A: 255}}
	if c.seriesColor(1) != (RGBA{G: 1, A: 255}) || c.seriesColor(2) != (RGBA{R: 1, A: 255}) {
		t.Error("override series colour should cycle Colors")
	}
}

func TestScatterChartDrawDots(t *testing.T) {
	th := DefaultLight()
	c := NewScatterChart([][]ScatterPoint{
		{{X: 1, Y: 1}, {X: 5, Y: 9}, {X: 9, Y: 3}},
		{{X: 2, Y: 8}, {X: 7, Y: 2}},
	})
	c.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 50})
	img, err := RenderImage(c, 80, 50, th)
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
	if got := countInk(img.Pix, 80, 50, th.Border); got == 0 {
		t.Error("no axis pixels drawn")
	}
	if got := countInk(img.Pix, 80, 50, piePalette[0]); got == 0 {
		t.Error("no series-0 dots drawn")
	}
	if got := countInk(img.Pix, 80, 50, piePalette[1]); got == 0 {
		t.Error("no series-1 dots drawn")
	}
}

func TestScatterChartSinglePointDot(t *testing.T) {
	// A flat range on both axes still projects a dot mid-plot.
	th := DefaultLight()
	c := NewScatterChart([][]ScatterPoint{{{X: 4, Y: 7}}})
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	img, err := RenderImage(c, 40, 30, th)
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
	if got := countInk(img.Pix, 40, 30, piePalette[0]); got == 0 {
		t.Error("single-point dot not drawn")
	}
}

func TestScatterChartEmptyDrawsAxesOnly(t *testing.T) {
	th := DefaultLight()
	c := NewScatterChart(nil)
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	surf := makeSurface(40, 30)
	c.Draw(newP(surf, 40), th)
	if got := countInk(surf, 40, 30, th.Border); got == 0 {
		t.Error("empty chart should still draw axes")
	}
	if got := countInk(surf, 40, 30, piePalette[0]); got != 0 {
		t.Errorf("empty chart drew %d dot px, want 0", got)
	}
}

func TestScatterChartDemoPNG(t *testing.T) {
	c := NewScatterChart([][]ScatterPoint{
		{{X: 1, Y: 2}, {X: 3, Y: 6}, {X: 5, Y: 4}, {X: 7, Y: 9}, {X: 9, Y: 5}},
		{{X: 2, Y: 8}, {X: 4, Y: 3}, {X: 6, Y: 7}, {X: 8, Y: 2}},
	})
	png, err := RenderPNG(c, 240, 140, DefaultLight())
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if err := os.WriteFile("/tmp/tk-scatter-demo.png", png, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
