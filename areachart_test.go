// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"os"
	"testing"
)

// blendOver returns src at alpha a composited over dst, matching the pixel
// back-end's src-over rounding so tests can predict a tinted fill colour.
func blendOver(src, dst RGBA, a uint8) RGBA {
	af, ia := uint32(a), uint32(255-a)
	mix := func(s, d uint8) uint8 { return uint8((uint32(s)*af + uint32(d)*ia + 127) / 255) }
	return RGBA{R: mix(src.R, dst.R), G: mix(src.G, dst.G), B: mix(src.B, dst.B), A: 255}
}

func TestAreaChartYRange(t *testing.T) {
	// Explicit bounds win when Max > Min.
	c := &AreaChart{Series: [][]float64{{2, 8}}, Min: 0, Max: 10}
	if mn, mx := c.yRange(); mn != 0 || mx != 10 {
		t.Errorf("explicit yRange = (%v,%v), want (0,10)", mn, mx)
	}
	// Auto range spans the combined min..max across every series (covers both
	// v<mn and v>mx updates).
	c = NewAreaChart([][]float64{{3, 1}, {9, 4}})
	if mn, mx := c.yRange(); mn != 1 || mx != 9 {
		t.Errorf("auto yRange = (%v,%v), want (1,9)", mn, mx)
	}
	// A flat combined range pads by ±1.
	c = NewAreaChart([][]float64{{5}, {5}})
	if mn, mx := c.yRange(); mn != 4 || mx != 6 {
		t.Errorf("flat yRange = (%v,%v), want (4,6)", mn, mx)
	}
	// No data at all falls back to [0,1] (empty + all-empty series).
	c = NewAreaChart(nil)
	if mn, mx := c.yRange(); mn != 0 || mx != 1 {
		t.Errorf("empty yRange = (%v,%v), want (0,1)", mn, mx)
	}
	c = NewAreaChart([][]float64{{}, {}})
	if mn, mx := c.yRange(); mn != 0 || mx != 1 {
		t.Errorf("all-empty yRange = (%v,%v), want (0,1)", mn, mx)
	}
}

func TestAreaChartSeriesColor(t *testing.T) {
	c := NewAreaChart(nil)
	// Default palette cycles.
	if c.seriesColor(0) != piePalette[0] || c.seriesColor(len(piePalette)) != piePalette[0] {
		t.Error("default series colour should cycle the shared palette")
	}
	// Colors override cycles.
	c.Colors = []RGBA{{R: 1, A: 255}, {G: 1, A: 255}}
	if c.seriesColor(1) != (RGBA{G: 1, A: 255}) || c.seriesColor(2) != (RGBA{R: 1, A: 255}) {
		t.Error("override series colour should cycle Colors")
	}
}

func TestAreaChartDrawFilledBand(t *testing.T) {
	th := DefaultLight()
	c := NewAreaChart([][]float64{{1, 9, 3, 7}, {4, 2, 8, 5}})
	c.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 50})
	img, err := RenderImage(c, 80, 50, th)
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
	// Axis frame present.
	if got := countInk(img.Pix, 80, 50, th.Border); got == 0 {
		t.Error("no axis pixels drawn")
	}
	// The polyline stroke (opaque palette colour) is present for series 0.
	if got := countInk(img.Pix, 80, 50, piePalette[0]); got == 0 {
		t.Error("no series-0 stroke drawn")
	}
	// The shaded band under series 0 (palette[0] at AreaFillAlpha over the
	// theme Background) is present.
	fill0 := blendOver(piePalette[0], th.Background, AreaFillAlpha)
	if got := countInk(img.Pix, 80, 50, fill0); got == 0 {
		t.Error("no filled band pixels for series 0")
	}
}

func TestAreaChartSinglePoint(t *testing.T) {
	th := DefaultLight()
	c := NewAreaChart([][]float64{{5}})
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	img, err := RenderImage(c, 40, 30, th)
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
	// A lone point marks an opaque dot over a tinted column.
	if got := countInk(img.Pix, 40, 30, piePalette[0]); got == 0 {
		t.Error("single-point marker not drawn")
	}
	fill0 := blendOver(piePalette[0], th.Background, AreaFillAlpha)
	if got := countInk(img.Pix, 40, 30, fill0); got == 0 {
		t.Error("single-point column not filled")
	}
}

func TestAreaChartEmptyAndEmptySeriesSkipped(t *testing.T) {
	th := DefaultLight()
	// Fully empty: axes only.
	c := NewAreaChart(nil)
	c.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	surf := makeSurface(40, 30)
	c.Draw(newP(surf, 40), th)
	if got := countInk(surf, 40, 30, th.Border); got == 0 {
		t.Error("empty chart should still draw axes")
	}
	// A mix of empty and non-empty series: the empty one is skipped (continue),
	// the other still paints.
	c = NewAreaChart([][]float64{{}, {1, 5, 2}})
	c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 40})
	img, err := RenderImage(c, 60, 40, th)
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
	if got := countInk(img.Pix, 60, 40, piePalette[1]); got == 0 {
		t.Error("second series should paint despite a leading empty series")
	}
}

func TestAreaChartNarrowCollapsesColumns(t *testing.T) {
	// More points than plot columns forces adjacent points to share an x
	// (x1 == x0), exercising the flat-segment interpolation branch.
	th := DefaultLight()
	c := NewAreaChart([][]float64{{1, 2, 3, 4, 5, 6, 7, 8}})
	c.SetBounds(Rect{X: 0, Y: 0, W: ChartPad + 3, H: 20})
	surf := makeSurface(ChartPad+3, 20)
	c.Draw(newP(surf, ChartPad+3), th)
	// Just assert it painted without panicking.
	if got := countInk(surf, ChartPad+3, 20, th.Border); got == 0 {
		t.Error("narrow area chart should still draw axes")
	}
}

func TestAreaChartDemoPNG(t *testing.T) {
	c := NewAreaChart([][]float64{
		{3, 6, 4, 8, 5, 9, 7},
		{1, 4, 2, 5, 3, 6, 4},
	})
	png, err := RenderPNG(c, 240, 140, DefaultLight())
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if err := os.WriteFile("/tmp/tk-areachart-demo.png", png, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
