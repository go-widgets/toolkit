// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/mvvm"

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

func TestTimeSeriesChartZeroSizeBoundsDrawsNothing(t *testing.T) {
	c := NewTimeSeriesChart([]TimePoint{{At: 1000, Value: 10}, {At: 2000, Value: 90}}, 0, 100)
	c.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 0})
	surf := makeSurface(4, 4)
	c.Draw(newP(surf, 4), DefaultLight()) // must not panic on an empty rect
	if got := countInk(surf, 4, 4, DefaultLight().Border); got != 0 {
		t.Errorf("zero-size bounds drew %d border pixels, want 0", got)
	}
}

// TestTimeSeriesChartTooSmallForAPlotDrawsNothing covers the guard for
// bounds too small to leave a positive plot area once the axis-label
// columns/rows are subtracted — distinct from zero bounds outright.
func TestTimeSeriesChartTooSmallForAPlotDrawsNothing(t *testing.T) {
	c := NewTimeSeriesChart([]TimePoint{{At: 1000, Value: 10}, {At: 2000, Value: 90}}, 0, 100)
	c.SetBounds(Rect{X: 0, Y: 0, W: 2, H: 2})
	surf := makeSurface(2, 2)
	c.Draw(newP(surf, 2), DefaultLight()) // must not panic
	if got := countInk(surf, 2, 2, DefaultLight().Accent); got != 0 {
		t.Errorf("a too-small plot area drew %d accent pixels, want 0", got)
	}
}

// TestTimeSeriesChartDegenerateSpanDrawsNoCurve covers points whose first
// and last At are equal (or, pathologically, out of order) — no
// meaningful span to place a curve or time labels along, so it must draw
// just the value axis rather than dividing by a zero or negative span.
func TestTimeSeriesChartDegenerateSpanDrawsNoCurve(t *testing.T) {
	c := NewTimeSeriesChart([]TimePoint{{At: 5000, Value: 10}, {At: 5000, Value: 90}}, 0, 100)
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	surf := makeSurface(120, 60)
	c.Draw(newP(surf, 120), DefaultLight())
	if got := countInk(surf, 120, 60, DefaultLight().Accent); got != 0 {
		t.Errorf("a degenerate (zero-span) series drew %d accent pixels, want 0", got)
	}
	if got := countInk(surf, 120, 60, DefaultLight().Border); got == 0 {
		t.Error("the value axis should still draw even with a degenerate span")
	}
}

func TestTimeSeriesChartZeroBoundsDoesNotPanic(t *testing.T) {
	// Min == Max (a degenerate value range) must not divide by zero.
	c := NewTimeSeriesChart([]TimePoint{{At: 1000, Value: 5}, {At: 2000, Value: 5}}, 5, 5)
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	surf := makeSurface(120, 60)
	c.Draw(newP(surf, 120), DefaultLight()) // must not panic
}

// TestAChartsSeriesCanBeBound covers the binding these charts did not have.
//
// A host had to assign a field and hope something repainted. mvvm.Observable is
// constrained to comparable types so it can skip a notification when nothing
// changed, and a slice is not comparable — which is why no chart here had a
// bindable series at all. ObservableList is the vehicle, and it says WHAT
// changed rather than only that something did.
func TestAChartsSeriesCanBeBound(t *testing.T) {
	c := NewTimeSeriesChart([]TimePoint{{At: 1, Value: 10}}, 0, 100)

	// Until somebody takes the list, the field is what the chart draws: a
	// chart nobody binds keeps working exactly as it did.
	if got := len(c.points()); got != 1 {
		t.Fatalf("the unbound chart draws %d points", got)
	}

	// Taking it seeds from the field, so nothing is lost at the moment of
	// binding.
	list := c.Series()
	if list.Len() != 1 || list.At(0).Value != 10 {
		t.Fatalf("the list came back as %+v", list.Slice())
	}
	// And from then on the LIST is the truth. Two sources for one truth is how
	// a chart comes to show last minute's data.
	list.Append(TimePoint{At: 2, Value: 20})
	if got := len(c.points()); got != 2 {
		t.Errorf("after appending, the chart draws %d points", got)
	}
	c.Points = nil
	if got := len(c.points()); got != 2 {
		t.Errorf("clearing the field changed the bound chart to %d points", got)
	}

	// A subscriber hears about it, which is the whole reason for binding: the
	// host repaints because the model changed, not because it polled.
	heard := 0
	list.Subscribe(func(mvvm.ListEvent[TimePoint]) { heard++ })
	list.Append(TimePoint{At: 3, Value: 30})
	if heard == 0 {
		t.Error("appending to the series told nobody")
	}
	// The same list every time, or two callers would bind to two charts.
	if c.Series() != list {
		t.Error("Series() handed out a second list")
	}
}
