// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"os"
	"testing"
)

func TestRadarChartTop(t *testing.T) {
	// Explicit positive Max wins.
	c := &RadarChart{Series: [][]float64{{2, 8}}, Max: 20}
	if got := c.top(); got != 20 {
		t.Errorf("explicit top = %v, want 20", got)
	}
	// Auto: the largest value across series.
	c = NewRadarChart([]string{"a", "b"}, [][]float64{{3, 9}, {4, 7}})
	if got := c.top(); got != 9 {
		t.Errorf("auto top = %v, want 9", got)
	}
	// All-zero (or empty) data falls back to a scale of 1.
	c = NewRadarChart([]string{"a"}, [][]float64{{0, 0}})
	if got := c.top(); got != 1 {
		t.Errorf("zero-data top = %v, want 1", got)
	}
}

func TestRadarChartSeriesColor(t *testing.T) {
	c := NewRadarChart(nil, nil)
	if c.seriesColor(0) != piePalette[0] || c.seriesColor(len(piePalette)) != piePalette[0] {
		t.Error("default series colour should cycle the shared palette")
	}
	c.Colors = []RGBA{{R: 1, A: 255}, {G: 1, A: 255}}
	if c.seriesColor(1) != (RGBA{G: 1, A: 255}) || c.seriesColor(2) != (RGBA{R: 1, A: 255}) {
		t.Error("override series colour should cycle Colors")
	}
}

func TestRadarChartDraw(t *testing.T) {
	th := DefaultLight()
	c := NewRadarChart(
		[]string{"Speed", "Power", "Range", "Armor", "Agility", "Luck"},
		[][]float64{
			{8, 6, 9, 4, 7, 5},
			{5, 9, 3, 8, 4, 6},
		},
	)
	c.Max = 10
	c.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 160})
	img, err := RenderImage(c, 160, 160, th)
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
	// Grid rings + spokes drew Border pixels.
	if got := countInk(img.Pix, 160, 160, th.Border); got == 0 {
		t.Error("no grid pixels drawn")
	}
	// Each series drew a solid outline in its palette colour.
	if got := countInk(img.Pix, 160, 160, piePalette[0]); got == 0 {
		t.Error("no series-0 outline drawn")
	}
	if got := countInk(img.Pix, 160, 160, piePalette[1]); got == 0 {
		t.Error("no series-1 outline drawn")
	}
	// Each series filled a translucent tint of its colour over the background.
	fill0 := blendOver(piePalette[0], th.Background, AreaFillAlpha)
	if got := countInk(img.Pix, 160, 160, fill0); got == 0 {
		t.Error("no series-0 fill drawn")
	}
	// Axis labels drew ink pixels.
	if got := countInk(img.Pix, 160, 160, dimInk(th)); got == 0 {
		t.Error("no axis-label pixels drawn")
	}
}

func TestRadarChartClampsHighAndLow(t *testing.T) {
	// A value above Max clamps to the outer ring (frac>1) and a negative value
	// clamps to the centre (frac<0), exercising both guards.
	th := DefaultLight()
	c := NewRadarChart([]string{"a", "b", "c"}, [][]float64{{50, -5, 3}})
	c.Max = 10
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 120})
	img, err := RenderImage(c, 120, 120, th)
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
	if got := countInk(img.Pix, 120, 120, piePalette[0]); got == 0 {
		t.Error("clamped polygon should still outline")
	}
}

func TestRadarChartShortSeries(t *testing.T) {
	// A series shorter than the axis count treats missing axes as 0 (the
	// k >= len(s) branch).
	th := DefaultLight()
	c := NewRadarChart([]string{"a", "b", "c", "d"}, [][]float64{{5, 8}})
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 120})
	if _, err := RenderImage(c, 120, 120, th); err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
}

func TestRadarChartFillSpansMinY(t *testing.T) {
	// A low value on axis 0 (top) and a high value on an upper-half axis puts
	// the polygon's topmost vertex after index 0, so fillPolygon's minY-update
	// branch runs.
	th := DefaultLight()
	c := NewRadarChart([]string{"a", "b", "c", "d", "e", "f"},
		[][]float64{{2, 10, 5, 3, 5, 8}})
	c.Max = 10
	c.SetBounds(Rect{X: 0, Y: 0, W: 140, H: 140})
	img, err := RenderImage(c, 140, 140, th)
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
	fill0 := blendOver(piePalette[0], th.Background, AreaFillAlpha)
	if got := countInk(img.Pix, 140, 140, fill0); got == 0 {
		t.Error("polygon should still fill")
	}
}

func TestRadarChartTwoAxesSkipsFill(t *testing.T) {
	// Fewer than three vertices bound no area, so fillPolygon returns without
	// filling; the outline (a line) still draws.
	th := DefaultLight()
	c := NewRadarChart([]string{"a", "b"}, [][]float64{{5, 8}})
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 120})
	img, err := RenderImage(c, 120, 120, th)
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
	if got := countInk(img.Pix, 120, 120, piePalette[0]); got == 0 {
		t.Error("two-axis outline should still draw")
	}
}

func TestRadarChartNoAxesDrawsNothing(t *testing.T) {
	th := DefaultLight()
	c := NewRadarChart(nil, [][]float64{{1, 2}})
	c.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 60})
	surf := makeSurface(60, 60)
	c.Draw(newP(surf, 60), th)
	if got := countInk(surf, 60, 60, th.Border); got != 0 {
		t.Errorf("no-axes chart drew %d border px, want 0", got)
	}
}

func TestRadarChartTinyBoundsDrawsNothing(t *testing.T) {
	// A radius below 1 (after the ChartPad inset) bails before any drawing.
	th := DefaultLight()
	c := NewRadarChart([]string{"a", "b", "c"}, [][]float64{{1, 2, 3}})
	c.SetBounds(Rect{X: 0, Y: 0, W: 2*ChartPad + 1, H: 2*ChartPad + 1})
	surf := makeSurface(2*ChartPad+1, 2*ChartPad+1)
	c.Draw(newP(surf, 2*ChartPad+1), th)
	if got := countInk(surf, 2*ChartPad+1, 2*ChartPad+1, th.Border); got != 0 {
		t.Errorf("tiny chart drew %d border px, want 0", got)
	}
}

func TestRadarChartDemoPNG(t *testing.T) {
	c := NewRadarChart(
		[]string{"Speed", "Power", "Range", "Armor", "Agility", "Luck"},
		[][]float64{
			{8, 6, 9, 4, 7, 5},
			{5, 9, 3, 8, 4, 6},
		},
	)
	c.Max = 10
	png, err := RenderPNG(c, 240, 200, DefaultLight())
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if err := os.WriteFile("/tmp/tk-radar-demo.png", png, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
