// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"image/png"
	"os"
	"testing"
)

func TestSparklineHitTestFalse(t *testing.T) {
	if NewSparkline([]float64{1, 2}).HitTest(3, 3) {
		t.Error("Sparkline is decorative; HitTest should return false")
	}
}

func TestSparklineInk(t *testing.T) {
	th := DefaultLight()
	// Zero Fill (A==0) inherits the theme accent.
	if got := NewSparkline(nil).ink(th); got != th.Accent {
		t.Errorf("default ink = %v, want accent %v", got, th.Accent)
	}
	// A set Fill wins.
	custom := RGB(0xC0, 0x30, 0x30)
	s := &Sparkline{Fill: custom}
	if got := s.ink(th); got != custom {
		t.Errorf("custom ink = %v, want %v", got, custom)
	}
}

func TestSparklineValueRange(t *testing.T) {
	// The min sits after the max so both the v<mn and v>mx branches fire.
	s := NewSparkline([]float64{3, 9, 1, 4})
	if mn, mx := s.valueRange(); mn != 1 || mx != 9 {
		t.Errorf("valueRange = (%v,%v), want (1,9)", mn, mx)
	}
}

func TestSparkFrac(t *testing.T) {
	if got := sparkFrac(5, 0, 10); got != 0.5 {
		t.Errorf("sparkFrac(5,0,10) = %v, want 0.5", got)
	}
	// Flat range (span<=0) centres at 0.5 rather than dividing by zero.
	if got := sparkFrac(7, 7, 7); got != 0.5 {
		t.Errorf("flat sparkFrac = %v, want 0.5", got)
	}
}

func TestSparklineEmptyDrawsNothing(t *testing.T) {
	s := NewSparkline(nil)
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	surf := makeSurface(40, 20)
	s.Draw(newP(surf, 40), DefaultLight())
	if got := countInk(surf, 40, 20, DefaultLight().Accent); got != 0 {
		t.Errorf("empty spark painted %d accent px, want 0", got)
	}
}

func TestSparklineSubPixelBounds(t *testing.T) {
	th := DefaultLight()
	// Width collapses to zero plot after the SparkPad inset.
	sw := &Sparkline{Values: []float64{1, 2, 3}}
	sw.SetBounds(Rect{X: 0, Y: 0, W: 2 * SparkPad, H: 20})
	surf := makeSurface(2*SparkPad, 20)
	sw.Draw(newP(surf, 2*SparkPad), th)
	if got := countInk(surf, 2*SparkPad, 20, th.Accent); got != 0 {
		t.Errorf("zero-width spark painted %d px, want 0", got)
	}
	// Height collapses to zero plot after the inset.
	sh := &Sparkline{Values: []float64{1, 2, 3}}
	sh.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 2 * SparkPad})
	surf2 := makeSurface(20, 2*SparkPad)
	sh.Draw(newP(surf2, 20), th)
	if got := countInk(surf2, 20, 2*SparkPad, th.Accent); got != 0 {
		t.Errorf("zero-height spark painted %d px, want 0", got)
	}
}

func TestSparklineSingleDot(t *testing.T) {
	s := NewSparkline([]float64{42})
	s.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 16})
	surf := makeSurface(30, 16)
	s.Draw(newP(surf, 30), DefaultLight())
	if got := countInk(surf, 30, 16, DefaultLight().Accent); got == 0 {
		t.Error("single-value spark should paint a dot")
	}
}

func TestSparklineLinePolyline(t *testing.T) {
	s := NewSparkline([]float64{1, 5, 2, 8, 3})
	s.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 24})
	surf := makeSurface(60, 24)
	s.Draw(newP(surf, 60), DefaultLight())
	if got := countInk(surf, 60, 24, DefaultLight().Accent); got == 0 {
		t.Error("polyline spark should paint accent pixels")
	}
}

func TestSparklineLineShowLastDot(t *testing.T) {
	th := DefaultLight()
	vals := []float64{1, 5, 2, 8, 3}
	mk := func(showLast bool) int {
		s := &Sparkline{Values: vals, ShowLast: showLast}
		s.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 24})
		surf := makeSurface(60, 24)
		s.Draw(newP(surf, 60), th)
		return countInk(surf, 60, 24, th.Accent)
	}
	if mk(true) <= mk(false) {
		t.Error("ShowLast should add dot pixels to the polyline")
	}
}

func TestSparklineFlatLineCentred(t *testing.T) {
	th := DefaultLight()
	s := NewSparkline([]float64{4, 4, 4, 4})
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 21})
	surf := makeSurface(40, 21)
	s.Draw(newP(surf, 40), th)
	// A flat series draws a horizontal line through the vertical centre.
	pl := s.plot()
	cy := pl.Y + int(0.5*float64(pl.H-1))
	if pixelAt(surf, 40, pl.X+2, cy) != th.Accent {
		t.Error("flat line should sit on the centre row")
	}
}

func TestSparklineBars(t *testing.T) {
	th := DefaultLight()
	// A leading value equal to the min gives frac 0 -> the bh<1 floor fires.
	s := &Sparkline{Values: []float64{0, 5, 2, 8}, Kind: SparkBar}
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 24})
	surf := makeSurface(40, 24)
	s.Draw(newP(surf, 40), th)
	if got := countInk(surf, 40, 24, th.Accent); got == 0 {
		t.Error("bar spark should paint accent pixels")
	}
}

func TestSparklineBarShowLastBrighter(t *testing.T) {
	th := DefaultLight()
	s := &Sparkline{Values: []float64{1, 4, 2, 8}, Kind: SparkBar, ShowLast: true}
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 24})
	surf := makeSurface(40, 24)
	s.Draw(newP(surf, 40), th)
	if got := countInk(surf, 40, 24, brighter(th.Accent)); got == 0 {
		t.Error("ShowLast bar should paint the brighter final bar")
	}
}

func TestSparklineBarFlat(t *testing.T) {
	th := DefaultLight()
	s := &Sparkline{Values: []float64{3, 3, 3}, Kind: SparkBar}
	s.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 20})
	surf := makeSurface(30, 20)
	s.Draw(newP(surf, 30), th)
	if got := countInk(surf, 30, 20, th.Accent); got == 0 {
		t.Error("flat bar series should still paint centred bars")
	}
}

func TestSparklineBarNarrowSlotFloors(t *testing.T) {
	th := DefaultLight()
	// More bars than plot pixels: slot and bw both floor to 1, no panic.
	s := &Sparkline{Values: []float64{1, 2, 3, 4, 5, 6, 7, 8}, Kind: SparkBar}
	s.SetBounds(Rect{X: 0, Y: 0, W: 2*SparkPad + 4, H: 16})
	w := 2*SparkPad + 4
	surf := makeSurface(w, 16)
	s.Draw(newP(surf, w), th)
	if got := countInk(surf, w, 16, th.Accent); got == 0 {
		t.Error("narrow bar spark should still paint at least one bar")
	}
}

// TestSparklineRenderPNGDemo drives both kinds through the public RenderPNG path,
// asserts accent pixels are present (and the ShowLast dot adds pixels), and writes
// a demo PNG to /tmp for eyeballing.
func TestSparklineRenderPNGDemo(t *testing.T) {
	th := DefaultLight()
	vals := []float64{3, 7, 4, 9, 5, 11, 6, 12, 8}

	// Line spark, no last dot vs. with last dot.
	line := &Sparkline{Values: vals}
	line.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 32})
	base := countPNGInk(t, renderPNG(t, line, 120, 32, th), th.Accent)
	if base == 0 {
		t.Fatal("line spark PNG has no accent pixels")
	}
	line.ShowLast = true
	withDot := countPNGInk(t, renderPNG(t, line, 120, 32, th), th.Accent)
	if withDot <= base {
		t.Errorf("ShowLast line PNG accent px = %d, want > %d", withDot, base)
	}

	// Bar spark, brighter last bar.
	bar := &Sparkline{Values: vals, Kind: SparkBar, ShowLast: true}
	bar.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 32})
	barPNG := renderPNG(t, bar, 120, 32, th)
	if countPNGInk(t, barPNG, th.Accent) == 0 {
		t.Error("bar spark PNG has no accent pixels")
	}
	if countPNGInk(t, barPNG, brighter(th.Accent)) == 0 {
		t.Error("ShowLast bar PNG missing brighter final bar")
	}

	if err := os.WriteFile("/tmp/tk-sparkline-demo.png", barPNG, 0o644); err != nil {
		t.Fatalf("write demo PNG: %v", err)
	}
}

func renderPNG(t *testing.T, w Widget, width, height int, theme *Theme) []byte {
	t.Helper()
	data, err := RenderPNG(w, width, height, theme)
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	return data
}

// countPNGInk decodes a PNG and counts pixels matching col's RGB.
func countPNGInk(t *testing.T, data []byte, col RGBA) int {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == col.R && uint8(g>>8) == col.G && uint8(bl>>8) == col.B {
				n++
			}
		}
	}
	return n
}
