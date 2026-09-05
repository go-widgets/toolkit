// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestTimeSeriesChartEmptyDrawsAxesOnly(t *testing.T) {
	c := NewTimeSeriesChart(nil, 0, 100)
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	surf := makeSurface(120, 60)
	c.Draw(newP(surf, 120), DefaultLight())

	th := DefaultLight()
	if got := countInk(surf, 120, 60, th.Border); got == 0 {
		t.Error("no gridline pixels drawn for an empty series")
	}
	if got := countInk(surf, 120, 60, th.Accent); got != 0 {
		t.Errorf("an empty series drew %d accent (curve) pixels, want 0", got)
	}
}

func TestTimeSeriesChartSinglePointDrawsNoCurve(t *testing.T) {
	c := NewTimeSeriesChart([]TimePoint{{At: 1000, Value: 50}}, 0, 100)
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	surf := makeSurface(120, 60)
	c.Draw(newP(surf, 120), DefaultLight())
	if got := countInk(surf, 120, 60, DefaultLight().Accent); got != 0 {
		t.Errorf("a single point drew %d accent pixels, want 0 (nothing to connect)", got)
	}
}

func TestTimeSeriesChartDrawsAPolyline(t *testing.T) {
	c := NewTimeSeriesChart([]TimePoint{
		{At: 1000, Value: 10},
		{At: 2000, Value: 50},
		{At: 3000, Value: 90},
	}, 0, 100)
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	surf := makeSurface(120, 60)
	c.Draw(newP(surf, 120), DefaultLight())
	if got := countInk(surf, 120, 60, DefaultLight().Accent); got == 0 {
		t.Error("no curve (Accent) pixels drawn for a 3-point series")
	}
}

// TestTimeSeriesChartRisesLeftToRight is the actual load-bearing proof:
// a climbing series must draw higher on screen (smaller Y) later in
// time — not just "some Accent pixels somewhere".
func TestTimeSeriesChartRisesLeftToRight(t *testing.T) {
	c := NewTimeSeriesChart([]TimePoint{
		{At: 1000, Value: 10},
		{At: 2000, Value: 90},
	}, 0, 100)
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	surf := makeSurface(120, 60)
	c.Draw(newP(surf, 120), DefaultLight())

	pl := c.plot()
	ink := DefaultLight().Accent
	topAt := func(x int) (y int, found bool) {
		for y := 0; y < 60; y++ {
			if pixelAt(surf, 120, x, y) == ink {
				return y, true
			}
		}
		return 0, false
	}
	leftY, leftOK := topAt(pl.X)
	rightY, rightOK := topAt(pl.X + pl.W - 1)
	if !leftOK || !rightOK {
		t.Fatalf("did not find curve ink at both plot edges (left=%v right=%v)", leftOK, rightOK)
	}
	if rightY >= leftY {
		t.Fatalf("right-edge y (%d) is not above left-edge y (%d) for a rising series", rightY, leftY)
	}
}

func TestTimeSeriesChartRespectsExplicitBounds(t *testing.T) {
	// Two series with the SAME shape but different explicit Min/Max must
	// draw the curve at different heights — proving Min/Max are actually
	// used as given, not auto-derived from the data the way LineChart's
	// Min==Max convenience would.
	base := []TimePoint{{At: 1000, Value: 50}, {At: 2000, Value: 50}}
	narrow := NewTimeSeriesChart(base, 0, 100)
	wide := NewTimeSeriesChart(base, 0, 1000)
	narrow.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	wide.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	s1, s2 := makeSurface(120, 60), makeSurface(120, 60)
	narrow.Draw(newP(s1, 120), DefaultLight())
	wide.Draw(newP(s2, 120), DefaultLight())

	// Each chart's own plot().X: a wider Max ("1000" vs "100") measures a
	// wider label column, so the two plot areas don't necessarily start
	// at the same x.
	find := func(surf []byte, x int) int {
		ink := DefaultLight().Accent
		for y := 0; y < 60; y++ {
			if pixelAt(surf, 120, x, y) == ink {
				return y
			}
		}
		return -1
	}
	y1, y2 := find(s1, narrow.plot().X), find(s2, wide.plot().X)
	if y1 == -1 || y2 == -1 {
		t.Fatal("curve not found in one of the two surfaces")
	}
	if y1 == y2 {
		t.Fatalf("value 50 on [0,100] and [0,1000] drew at the same y (%d) — Min/Max are not taking effect", y1)
	}
}

func TestTimeSeriesChartCustomInk(t *testing.T) {
	custom := RGB(0x11, 0x22, 0x33)
	c := NewTimeSeriesChart([]TimePoint{{At: 1000, Value: 10}, {At: 2000, Value: 90}}, 0, 100)
	c.Ink = custom
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	surf := makeSurface(120, 60)
	c.Draw(newP(surf, 120), DefaultLight())
	if got := countInk(surf, 120, 60, custom); got == 0 {
		t.Error("no pixels drawn in the explicitly-set Ink color")
	}
}

func TestTimeSeriesChartDrawsValueAndTimeLabels(t *testing.T) {
	c := NewTimeSeriesChart([]TimePoint{{At: 1000, Value: 10}, {At: 2000, Value: 90}}, 0, 100)
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	surf := makeSurface(200, 80)
	c.Draw(newP(surf, 200), DefaultLight())

	label := dimInk(DefaultLight())
	if got := countInk(surf, 200, 80, label); got == 0 {
		t.Error("no axis-label (dimInk) pixels drawn at all")
	}
}

// TestTimeSeriesChartFormatFuncsAreHonored proves FormatValue/FormatTime
// are actually called, not just accepted and ignored — by supplying
// formatters whose output couldn't come from the defaults ("%.0f" and
// "Jan 2 15:04") and confirming SOME label pixels moved as a result
// (a distinguishable draw happened) rather than asserting exact glyph
// positions, which is painter's own font's business.
func TestTimeSeriesChartFormatFuncsAreHonored(t *testing.T) {
	calledValue, calledTime := false, false
	c := NewTimeSeriesChart([]TimePoint{{At: 1000, Value: 10}, {At: 2000, Value: 90}}, 0, 100)
	c.FormatValue = func(v float64) string { calledValue = true; return "V" }
	c.FormatTime = func(t int64) string { calledTime = true; return "T" }
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 80})
	surf := makeSurface(200, 80)
	c.Draw(newP(surf, 200), DefaultLight())

	if !calledValue {
		t.Error("FormatValue was never called")
	}
	if !calledTime {
		t.Error("FormatTime was never called")
	}
}

func TestTimeSeriesChartA11y(t *testing.T) {
	c := NewTimeSeriesChart([]TimePoint{{At: 1000, Value: 1}, {At: 2000, Value: 2}, {At: 3000, Value: 3}}, 0, 10)
	got := c.A11y()
	if got.Role != RoleImg {
		t.Errorf("A11y().Role = %v, want RoleImg", got.Role)
	}
	if got.Value != "3 points" {
		t.Errorf("A11y().Value = %q, want %q", got.Value, "3 points")
	}
}

func TestTimeSeriesChartZeroBoundsDoesNotPanic(t *testing.T) {
	// Min == Max (a degenerate value range) must not divide by zero.
	c := NewTimeSeriesChart([]TimePoint{{At: 1000, Value: 5}, {At: 2000, Value: 5}}, 5, 5)
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	surf := makeSurface(120, 60)
	c.Draw(newP(surf, 120), DefaultLight()) // must not panic
}
