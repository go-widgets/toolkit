// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"strings"
	"testing"

	"github.com/go-widgets/painter"
)

// drawPlot renders c into a fresh w×h surface and returns it.
func drawPlot(c *XYPlot, w, h int, th *Theme) []byte {
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	c.Draw(newP(surf, w), th)
	return surf
}

func TestNiceStep(t *testing.T) {
	cases := []struct {
		span float64
		n    int
		want float64
	}{
		{10, 10, 1},
		{10, 5, 2},
		{10, 2, 5},
		{0.95, 1, 1},   // past 5x: the next power of ten
		{1000, 4, 500}, // 250 -> 500
		{0, 4, 1},      // degenerate span
		{math.NaN(), 4, 1},
		{math.Inf(1), 4, 1},
		{10, 0, 1}, // no intervals asked for
	}
	for _, tc := range cases {
		if got := NiceStep(tc.span, tc.n); math.Abs(got-tc.want) > 1e-12 {
			t.Errorf("NiceStep(%v, %d) = %v, want %v", tc.span, tc.n, got, tc.want)
		}
	}
}

func TestPlotTicks(t *testing.T) {
	got := plotTicks(-0.3, 0.3, 0.1)
	if len(got) != 7 || got[3] != 0 {
		t.Fatalf("plotTicks(-0.3, 0.3, 0.1) = %v; want 7 ticks with an exact 0 in the middle", got)
	}
	if FormatPlotNumber(got[3]) != "0" || FormatPlotNumber(-0.1) != "-0.1" || FormatPlotNumber(12345) != "1.234e+04" {
		t.Errorf("FormatPlotNumber: %q %q %q", FormatPlotNumber(got[3]), FormatPlotNumber(-0.1), FormatPlotNumber(12345))
	}
	// The cap: a step far too fine for the range stops at maxPlotTicks.
	if n := len(plotTicks(0, 1000, 1)); n != maxPlotTicks {
		t.Errorf("uncapped tick list: %d ticks", n)
	}
}

func TestPlotSpan(t *testing.T) {
	for _, tc := range []struct{ lo, hi, wlo, whi float64 }{
		{1, 2, 1, 2},
		{0, 0, -1, 1},
		{4, 4, 2, 6},
		{-4, -4, -6, -2},
	} {
		if lo, hi := span(tc.lo, tc.hi); lo != tc.wlo || hi != tc.whi {
			t.Errorf("span(%v,%v) = (%v,%v), want (%v,%v)", tc.lo, tc.hi, lo, hi, tc.wlo, tc.whi)
		}
	}
}

func TestPlotSeriesIndexAndPairing(t *testing.T) {
	s := PlotSeries{Y: []float64{5, 6, 7}}
	if s.len() != 3 || s.x(2) != 2 {
		t.Errorf("nil X: len %d, x(2) %v", s.len(), s.x(2))
	}
	s.X = []float64{10, 20}
	if s.len() != 2 || s.x(1) != 20 {
		t.Errorf("paired: len %d, x(1) %v", s.len(), s.x(1))
	}
}

func TestXYPlotDataRange(t *testing.T) {
	c := NewXYPlot(
		PlotSeries{X: []float64{1, math.NaN(), 3}, Y: []float64{2, 9, 5}},
		PlotSeries{Y: []float64{math.Inf(1), 4}, Style: PlotStem},
	)
	xlo, xhi, ylo, yhi, ok := c.dataRange()
	if !ok || xlo != 1 || xhi != 3 || ylo != 0 || yhi != 5 {
		t.Errorf("dataRange = %v %v %v %v %v; want 1 3 0 5 true (NaN/Inf skipped, stems reach 0)", xlo, xhi, ylo, yhi, ok)
	}
	if _, _, _, _, ok := NewXYPlot().dataRange(); ok {
		t.Error("an empty plot reported a data range")
	}
}

func TestXYPlotSeriesBinding(t *testing.T) {
	c := &XYPlot{}
	changes := 0
	c.Series().SubscribeChanged(func() { changes++ })
	c.SetSeries(PlotSeries{Y: []float64{1}}, PlotSeries{Y: []float64{2}})
	if c.Series().Len() != 2 || changes == 0 {
		t.Fatalf("SetSeries: len %d, %d notifications", c.Series().Len(), changes)
	}
	c.SetSeries(PlotSeries{Y: []float64{3}})
	if c.Series().Len() != 1 || c.Series().At(0).Y[0] != 3 {
		t.Error("SetSeries did not replace the curves")
	}
	if c.Hover().Get() || c.HoverIndex().Get() != 0 {
		t.Error("hover defaults are not off / 0")
	}
}

// TestXYPlotDrawsEachStyle checks every style puts its own ink on the
// surface, the grid and labels are drawn, and nothing leaves Bounds.
func TestXYPlotDrawsEachStyle(t *testing.T) {
	th := DefaultLight()
	red, green, blue := RGB(255, 0, 0), RGB(0, 200, 0), RGB(0, 0, 255)
	c := NewXYPlot(
		PlotSeries{Label: "line", Y: []float64{0, 4, math.NaN(), 2, 3}, Ink: red},
		PlotSeries{Label: "stem", Y: []float64{-1, 2, math.NaN(), 1}, Style: PlotStem, Ink: green},
		PlotSeries{Y: []float64{1, math.Inf(-1), 2}, Style: PlotMarkers, Ink: blue},
	)
	c.XLabel, c.YLabel = "Hz", "amp"
	surf := drawPlot(c, 320, 200, th)
	for name, ink := range map[string]RGBA{"line": red, "stem": green, "markers": blue, "grid": th.Border} {
		if countInk(surf, 320, 200, ink) == 0 {
			t.Errorf("no %s pixels", name)
		}
	}
	// The legend panel sits on Surface.
	if countInk(surf, 320, 200, th.Surface) == 0 {
		t.Error("no legend panel")
	}
}

// TestXYPlotStemsStandOnZero: a stem series' stems all meet the y = 0 row.
func TestXYPlotStemsStandOnZero(t *testing.T) {
	th := DefaultLight()
	ink := RGB(200, 0, 0)
	c := NewXYPlot(PlotSeries{Y: []float64{3, -2, 5}, Style: PlotStem, Ink: ink})
	surf := drawPlot(c, 300, 200, th)
	s := c.scale()
	y0 := s.py(0)
	for i := range 3 {
		if pixelAt(surf, 300, s.px(float64(i)), y0) != ink {
			t.Errorf("stem %d does not reach the zero row", i)
		}
	}
}

// TestXYPlotDenseEnvelope: a series far longer than the plot is wide is drawn
// as a per-column envelope that still shows a single-sample spike.
func TestXYPlotDenseEnvelope(t *testing.T) {
	th := DefaultLight()
	ink := RGB(10, 20, 200)
	n := 20000
	y := make([]float64, n)
	y[n/2] = 1 // one spike in a flat signal
	y[n/2+1] = math.NaN()
	x := make([]float64, n)
	for i := range x {
		x[i] = float64(i)
		if i > n/4 && i < n/3 {
			x[i] = math.NaN() // a gap: whole columns with no point
		}
	}
	for _, style := range []PlotStyle{PlotLine, PlotStem} {
		c := NewXYPlot(PlotSeries{X: x, Y: y, Style: style, Ink: ink})
		surf := drawPlot(c, 300, 160, th)
		s := c.scale()
		col := s.px(float64(n / 2))
		if pixelAt(surf, 300, col, s.py(1)) != ink {
			t.Errorf("style %d: the spike was lost in the envelope", style)
		}
	}
}

func TestXYPlotPaletteAndAccent(t *testing.T) {
	th := DefaultDark()
	if inkFor(PlotSeries{}, 0, th) != th.Accent {
		t.Error("first series is not the accent")
	}
	if inkFor(PlotSeries{}, 1, th) != plotPalette[0] || inkFor(PlotSeries{}, 1+len(plotPalette), th) != plotPalette[0] {
		t.Error("palette does not cycle")
	}
	own := RGB(1, 2, 3)
	if inkFor(PlotSeries{Ink: own}, 0, th) != own {
		t.Error("explicit ink ignored")
	}
}

func TestXYPlotTooSmallAndEmpty(t *testing.T) {
	th := DefaultLight()
	c := NewXYPlot(PlotSeries{Y: []float64{1, 2}})
	surf := drawPlot(c, 10, 10, th) // no room for a plot area at all
	if countInk(surf, 10, 10, th.Accent) != 0 {
		t.Error("drew a curve with no plot area")
	}
	// An empty plot still draws its frame.
	e := &XYPlot{}
	surf = drawPlot(e, 200, 120, th)
	if countInk(surf, 200, 120, th.Border) == 0 {
		t.Error("empty plot drew no grid")
	}
}

func TestXYPlotLegendSkippedWhenNoRoom(t *testing.T) {
	th := DefaultLight()
	c := NewXYPlot(PlotSeries{Label: strings.Repeat("a very long legend label ", 4), Y: []float64{1, 2}})
	surf := drawPlot(c, 160, 100, th)
	if countInk(surf, 160, 100, th.Surface) != 0 {
		t.Error("a legend wider than the plot was drawn over the axes")
	}
}

func TestXYPlotFormatters(t *testing.T) {
	c := NewXYPlot(PlotSeries{Y: []float64{1, 2}})
	c.FormatX = func(v float64) string { return "X" }
	c.FormatY = func(v float64) string { return "Y" }
	if c.formatX(1) != "X" || c.formatY(1) != "Y" {
		t.Error("custom formatters ignored")
	}
	drawPlot(c, 200, 120, DefaultLight())
}

func TestXYPlotHover(t *testing.T) {
	th := DefaultLight()
	c := NewXYPlot(PlotSeries{X: []float64{0, math.NaN(), 10, 20}, Y: []float64{0, 1, 5, 2}})
	c.SetBounds(Rect{X: 40, Y: 30, W: 300, H: 200})
	s := c.scale()

	// Not a move: ignored.
	c.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if c.Hover().Get() {
		t.Fatal("a click raised the hover")
	}
	// A move over x = 10's column selects index 2.
	lx := s.px(10) - 40
	c.OnEvent(Event{Kind: EventMouseMove, X: lx + 1, Y: 50})
	if !c.Hover().Get() || c.HoverIndex().Get() != 2 {
		t.Fatalf("hover = %v index %d; want true 2", c.Hover().Get(), c.HoverIndex().Get())
	}
	surf := makeSurface(400, 300)
	c.Draw(newP(surf, 400), th)
	if pixelAt(surf, 400, s.px(10), s.pl.Y+s.pl.H/2) == th.Background {
		t.Error("no hover rule at the hovered point")
	}
	// From the left of the first point too.
	c.OnEvent(Event{Kind: EventMouseMove, X: 0, Y: 50})
	if c.HoverIndex().Get() != 0 {
		t.Errorf("leftmost move picked %d", c.HoverIndex().Get())
	}
	// A move off the plot clears it.
	c.OnEvent(Event{Kind: EventMouseMove, X: -5, Y: 50})
	if c.Hover().Get() {
		t.Error("leaving did not clear the hover")
	}

	// Out-of-range and non-finite hovered points draw no readout.
	c.Hover().Set(true)
	for _, i := range []int{-1, 9, 1} {
		c.HoverIndex().Set(i)
		c.Draw(newP(makeSurface(400, 300), 400), th)
	}
	// No series at all: no value, no hover.
	e := &XYPlot{}
	e.SetBounds(Rect{W: 100, H: 100})
	if _, _, _, ok := e.ValueAt(10); ok {
		t.Error("ValueAt on an empty plot")
	}
	e.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: 10})
	e.Hover().Set(true)
	e.Draw(newP(makeSurface(100, 100), 100), th)
}

func TestXYPlotA11y(t *testing.T) {
	c := NewXYPlot(PlotSeries{Y: []float64{1, 2, 3}}, PlotSeries{X: []float64{1}, Y: []float64{1, 2}})
	c.YLabel = "magnitude"
	a := c.A11y()
	if a.Role != RoleImg || a.Name != "magnitude" || a.Value != "2 series, 4 points" {
		t.Errorf("A11y = %+v", a)
	}
}

// TestXYPlotOnACellGrid: on a terminal back-end (no path stroking) the curve
// falls back to Bresenham and still lands in the grid.
func TestXYPlotOnACellGrid(t *testing.T) {
	cp := painter.NewCellPainter(120, 60)
	c := NewXYPlot(PlotSeries{Y: []float64{0, 5, 1, 4}})
	c.SetBounds(Rect{W: 120, H: 60})
	c.Draw(cp, DefaultLight())
	plotStroke(cp, 1, 1, 10, 5, RGB(1, 2, 3))
}
