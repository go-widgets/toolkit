// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
	"time"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// sentinelPixel is makeSurface's own pre-fill value (see widget_test.go).
var sentinelPixel = RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}

// isPainted reports whether (x, y) differs from the surface's untouched
// sentinel — true for ANY coverage, including a partial one an
// anti-aliased stroke leaves at the edge of its own path. Exact color
// equality (countInk) is the wrong tool for a curve drawn via
// drawCurveLine: its pixels are a blend with whatever the sentinel or a
// neighbouring stroke already put there, essentially never the pure ink
// value.
func isPainted(surf []byte, w, x, y int) bool {
	return pixelAt(surf, w, x, y) != sentinelPixel
}

// countPainted counts pixels differing from the sentinel at all.
func countPainted(surf []byte, w, h int) int {
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if isPainted(surf, w, x, y) {
				n++
			}
		}
	}
	return n
}

// isCurvePainted is isPainted narrowed to exclude the plain Bresenham
// gridlines (still an exact theme.Border match, unaffected by
// drawCurveLine) — for a test that needs to find the CURVE
// specifically, not just any ink, on a column the gridlines also cross.
func isCurvePainted(surf []byte, w, x, y int, theme *Theme) bool {
	px := pixelAt(surf, w, x, y)
	return px != sentinelPixel && px != theme.Border
}

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
	noCurve := NewTimeSeriesChart([]TimePoint{{At: 1000, Value: 10}}, 0, 100)
	withCurve := NewTimeSeriesChart([]TimePoint{
		{At: 1000, Value: 10},
		{At: 2000, Value: 50},
		{At: 3000, Value: 90},
	}, 0, 100)
	noCurve.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	withCurve.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	s1, s2 := makeSurface(120, 60), makeSurface(120, 60)
	noCurve.Draw(newP(s1, 120), DefaultLight())
	withCurve.Draw(newP(s2, 120), DefaultLight())

	base, got := countPainted(s1, 120, 60), countPainted(s2, 120, 60)
	if got <= base {
		t.Errorf("a 3-point series painted %d pixels, want more than the axes-only baseline (%d)", got, base)
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
	theme := DefaultLight()
	topAt := func(x int) (y int, found bool) {
		for y := 0; y < 60; y++ {
			if isCurvePainted(surf, 120, x, y, theme) {
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
	theme := DefaultLight()
	find := func(surf []byte, x int) int {
		for y := 0; y < 60; y++ {
			if isCurvePainted(surf, 120, x, y, theme) {
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

// TestTimeSeriesChartCustomInk proves Ink is actually used by rendering
// the same series twice with two different explicit colors and checking
// the two images differ — an anti-aliased curve's own pixels are a
// blend with the surface underneath, never an exact match for either
// color, so comparing two renders is the robust way to prove the color
// choice took effect.
func TestTimeSeriesChartCustomInk(t *testing.T) {
	points := []TimePoint{{At: 1000, Value: 10}, {At: 2000, Value: 90}}
	a := NewTimeSeriesChart(points, 0, 100)
	a.Ink = RGB(0x11, 0x22, 0x33)
	b := NewTimeSeriesChart(points, 0, 100)
	b.Ink = RGB(0xEE, 0xDD, 0xCC)
	a.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	b.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	s1, s2 := makeSurface(120, 60), makeSurface(120, 60)
	a.Draw(newP(s1, 120), DefaultLight())
	b.Draw(newP(s2, 120), DefaultLight())

	for i := range s1 {
		if s1[i] != s2[i] {
			return
		}
	}
	t.Error("two different explicit Ink colors rendered byte-identical images")
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

// TestAChartCanFollowItsOwnPeak covers an axis on live data.
//
// A fixed ceiling is wrong in both directions: too low and a burst is drawn off
// the top where nobody can measure it, too high and everything else is a flat
// line along the bottom. And one that simply tracked the peak would rescale the
// whole picture every time a spike scrolled off the left — a graph whose axis
// keeps moving cannot be read at all.
func TestAChartCanFollowItsOwnPeak(t *testing.T) {
	c := NewTimeSeriesChart(nil, 0, 1)
	c.FollowPeak = true

	// Up at once, so a burst is never drawn off the top.
	c.Points = []TimePoint{{At: 1, Value: 3}}
	c.followPeak()
	if c.Max < 3 {
		t.Fatalf("a peak of 3 left the ceiling at %v", c.Max)
	}
	high := c.Max

	// Down slowly: one quiet draw must not rescale the picture.
	c.Points = []TimePoint{{At: 2, Value: 0}}
	c.followPeak()
	if c.Max >= high {
		t.Errorf("the ceiling did not come down at all: %v", c.Max)
	}
	if c.Max <= NiceCeiling(0) {
		t.Errorf("the ceiling fell all the way in one draw: %v", c.Max)
	}
	// And it arrives, given enough quiet draws.
	for i := 0; i < 200; i++ {
		c.followPeak()
	}
	if c.Max != NiceCeiling(0) {
		t.Errorf("after two hundred quiet draws the ceiling is %v", c.Max)
	}

	// A chart nobody asked to follow anything keeps the bounds it was given:
	// this is opt-in, and an existing chart is unchanged.
	fixed := NewTimeSeriesChart([]TimePoint{{At: 1, Value: 900}}, 0, 100)
	fixed.followPeak()
	if fixed.Max != 100 {
		t.Errorf("a fixed chart rescaled itself to %v", fixed.Max)
	}
}

// TestNiceCeilingSpeaksInRoundNumbers covers the gradations an axis is read in.
func TestNiceCeilingSpeaksInRoundNumbers(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{0, 1}, {-5, 1}, {1, 1}, {3, 3}, {5, 6}, {9, 12}, {13, 16},
	} {
		if got := NiceCeiling(tc.in); got != tc.want {
			t.Errorf("NiceCeiling(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
	// Never below the data: a ceiling under the peak draws a curve out of its
	// own chart.
	for _, v := range []float64{0.1, 7, 1000, 1 << 30} {
		if got := NiceCeiling(v); got < v {
			t.Errorf("NiceCeiling(%v) = %v, below the data", v, got)
		}
	}
}

// borderCount draws c and counts theme.Border pixels — the diagnostic
// used below to prove the Threshold line adds ink, since gridlines
// already use the same color and would otherwise mask the comparison.
func borderCount(t *testing.T, c *TimeSeriesChart, w, h int) int {
	t.Helper()
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	c.Draw(newP(surf, w), DefaultLight())
	return countInk(surf, w, h, DefaultLight().Border)
}

func TestTimeSeriesChartThresholdDrawnAsReferenceLine(t *testing.T) {
	points := []TimePoint{{At: 0, Value: 50}, {At: 7200, Value: 50}}
	without := NewTimeSeriesChart(points, 0, 100)
	withThreshold := NewTimeSeriesChart(points, 0, 100)
	withThreshold.Threshold = []TimePoint{{At: 0, Value: 0}, {At: 7200, Value: 100}}

	without.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	withThreshold.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	s1, s2 := makeSurface(120, 60), makeSurface(120, 60)
	without.Draw(newP(s1, 120), DefaultLight())
	withThreshold.Draw(newP(s2, 120), DefaultLight())

	base, got := countPainted(s1, 120, 60), countPainted(s2, 120, 60)
	if got <= base {
		t.Fatalf("painted pixel count with Threshold set (%d) did not exceed without (%d)", got, base)
	}
}

func TestTimeSeriesChartThresholdSinglePointDrawsNothing(t *testing.T) {
	points := []TimePoint{{At: 0, Value: 50}, {At: 7200, Value: 50}}
	without := NewTimeSeriesChart(points, 0, 100)
	withOne := NewTimeSeriesChart(points, 0, 100)
	withOne.Threshold = []TimePoint{{At: 3600, Value: 50}}

	base := borderCount(t, without, 120, 60)
	got := borderCount(t, withOne, 120, 60)
	if got != base {
		t.Fatalf("a single-point Threshold drew %d Border pixels, want exactly %d (no line, nothing to connect)", got, base)
	}
}

// TestSegmentInk exercises the exact decision drawCurveLine's color
// choice hangs on, as a plain value-in-value-out function — no painter,
// no pixels, no anti-aliasing to fight, unlike sampling drawn pixels for
// an exact color match (which an AA stroke's own blended output never
// gives).
func TestSegmentInk(t *testing.T) {
	c := &TimeSeriesChart{Threshold: []TimePoint{{At: 0, Value: 50}}}
	ink := RGB(0, 0xFF, 0)
	over := RGB(0xFF, 0, 0)

	c.OverInk = over
	if got := c.segmentInk(TimePoint{At: 10, Value: 30}, ink); got != ink {
		t.Errorf("below threshold: segmentInk = %v, want ink %v", got, ink)
	}
	if got := c.segmentInk(TimePoint{At: 10, Value: 90}, ink); got != over {
		t.Errorf("above threshold: segmentInk = %v, want OverInk %v", got, over)
	}

	c.OverInk = RGBA{}
	if got := c.segmentInk(TimePoint{At: 10, Value: 90}, ink); got != ink {
		t.Errorf("OverInk at its zero value: segmentInk = %v, want ink %v (recoloring should be disabled)", got, ink)
	}
}

// TestTimeSeriesChartOverInkChangesTheRenderedImage is the integration
// smoke test alongside TestSegmentInk's precise unit coverage: proves
// Draw actually WIRES segmentInk's choice into what gets painted, by
// checking that toggling OverInk changes the output image at all.
// Sampling for an exact OverInk pixel match is not used here since an
// anti-aliased segment's own pixels are a blend, not a pure color.
func TestTimeSeriesChartOverInkChangesTheRenderedImage(t *testing.T) {
	newChart := func() *TimeSeriesChart {
		c := NewTimeSeriesChart([]TimePoint{
			{At: 0, Value: 10},
			{At: 3600, Value: 90},
		}, 0, 100)
		c.Threshold = []TimePoint{{At: 0, Value: 50}, {At: 3600, Value: 50}}
		return c
	}

	withOverInk := newChart()
	withOverInk.OverInk = RGB(0xFF, 0x00, 0x00)
	withOverInk.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	s1 := makeSurface(120, 60)
	withOverInk.Draw(newP(s1, 120), DefaultLight())

	withoutOverInk := newChart()
	withoutOverInk.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	s2 := makeSurface(120, 60)
	withoutOverInk.Draw(newP(s2, 120), DefaultLight())

	for i := range s1 {
		if s1[i] != s2[i] {
			return
		}
	}
	t.Error("setting OverInk on a series with a stretch above Threshold rendered byte-identical to leaving it unset")
}

func TestTimeSeriesChartOverInkAllBelowThresholdStaysInk(t *testing.T) {
	c := NewTimeSeriesChart([]TimePoint{
		{At: 0, Value: 5},
		{At: 3600, Value: 10},
	}, 0, 100)
	c.Threshold = []TimePoint{{At: 0, Value: 50}, {At: 3600, Value: 50}}
	over := RGB(0xFF, 0x00, 0x00)
	c.OverInk = over
	c.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	surf := makeSurface(120, 60)
	c.Draw(newP(surf, 120), DefaultLight())

	if got := countInk(surf, 120, 60, over); got != 0 {
		t.Errorf("a series entirely below Threshold drew %d OverInk pixels, want 0", got)
	}
}

func TestThresholdAt(t *testing.T) {
	c := &TimeSeriesChart{Threshold: []TimePoint{
		{At: 100, Value: 1},
		{At: 200, Value: 2},
		{At: 300, Value: 3},
	}}
	cases := []struct {
		name string
		at   int64
		want float64
	}{
		{"before first", 50, 1},
		{"exact match", 200, 2},
		{"between samples", 250, 2},
		{"after last", 1000, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := c.thresholdAt(tc.at)
			if !ok {
				t.Fatalf("thresholdAt(%d): ok = false, want true", tc.at)
			}
			if got != tc.want {
				t.Errorf("thresholdAt(%d) = %v, want %v", tc.at, got, tc.want)
			}
		})
	}
}

func TestThresholdAtEmptyReturnsFalse(t *testing.T) {
	c := &TimeSeriesChart{}
	if _, ok := c.thresholdAt(123); ok {
		t.Fatal("thresholdAt on an empty Threshold: ok = true, want false")
	}
}

func TestVerticalGridInterval(t *testing.T) {
	cases := []struct {
		name string
		span time.Duration
		want time.Duration
	}{
		{"5 hours picks hourly", 5 * time.Hour, time.Hour},
		{"7 days picks daily", 7 * 24 * time.Hour, 24 * time.Hour},
		{"100 days falls back to the coarsest interval", 100 * 24 * time.Hour, 7 * 24 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := verticalGridInterval(tc.span); got != tc.want {
				t.Errorf("verticalGridInterval(%v) = %v, want %v", tc.span, got, tc.want)
			}
		})
	}
}

func TestVerticalGridTicksNonPositiveSpanReturnsNil(t *testing.T) {
	if got := verticalGridTicks(1000, 1000); got != nil {
		t.Errorf("verticalGridTicks(equal first/last) = %v, want nil", got)
	}
	if got := verticalGridTicks(2000, 1000); got != nil {
		t.Errorf("verticalGridTicks(last before first) = %v, want nil", got)
	}
}

// TestVerticalGridTicksHourAligned proves hour-granularity ticks land on
// the hour (Unix time divisible by 3600, true regardless of the test
// machine's local timezone since Truncate operates on the absolute
// instant) even when first itself is not hour-aligned.
func TestVerticalGridTicksHourAligned(t *testing.T) {
	first := time.Date(2026, 1, 1, 10, 17, 0, 0, time.UTC).Unix()
	last := first + int64(5*time.Hour/time.Second)
	ticks := verticalGridTicks(first, last)
	if len(ticks) == 0 {
		t.Fatal("no ticks returned for a 5-hour span")
	}
	for _, tk := range ticks {
		if tk < first || tk > last {
			t.Errorf("tick %d falls outside [first, last] = [%d, %d]", tk, first, last)
		}
		if tk%3600 != 0 {
			t.Errorf("tick %d is not hour-aligned", tk)
		}
	}
}

// TestVerticalGridTicksDayAligned proves day-granularity ticks land on
// local midnight.
func TestVerticalGridTicksDayAligned(t *testing.T) {
	first := time.Date(2026, 1, 1, 15, 0, 0, 0, time.Local).Unix()
	last := first + int64(7*24*time.Hour/time.Second)
	ticks := verticalGridTicks(first, last)
	if len(ticks) == 0 {
		t.Fatal("no ticks returned for a 7-day span")
	}
	for _, tk := range ticks {
		if tk < first || tk > last {
			t.Errorf("tick %d falls outside [first, last] = [%d, %d]", tk, first, last)
		}
		lt := time.Unix(tk, 0)
		if lt.Hour() != 0 || lt.Minute() != 0 || lt.Second() != 0 {
			t.Errorf("tick %v is not local midnight", lt)
		}
	}
}

// TestDrawCurveLineFallsBackToBresenhamOnACellPainter proves the
// non-PathPainter branch: a CellPainter has no notion of partial
// coverage (drawLine's own doc comment: "each pixel promotes to a
// filled cell"), so drawCurveLine must delegate to it unchanged rather
// than attempt an anti-aliased stroke — verified by checking the two
// paints leave an identical cell grid, not by sampling a blended color
// (there is none on this back-end).
func TestDrawCurveLineFallsBackToBresenhamOnACellPainter(t *testing.T) {
	viaCurveLine := painter.NewCellPainter(20, 10)
	drawCurveLine(viaCurveLine, 1, 1, 15, 8, RGB(0, 0xFF, 0))
	viaDrawLine := painter.NewCellPainter(20, 10)
	drawLine(viaDrawLine, 1, 1, 15, 8, RGB(0, 0xFF, 0))

	for i := range viaCurveLine.Cells {
		if viaCurveLine.Cells[i] != viaDrawLine.Cells[i] {
			t.Fatalf("cell %d differs: drawCurveLine=%+v drawLine=%+v", i, viaCurveLine.Cells[i], viaDrawLine.Cells[i])
		}
	}
}

func TestDrawDashedLineZeroLengthDrawsNothing(t *testing.T) {
	surf := makeSurface(20, 20)
	drawDashedLine(newP(surf, 20), 5, 5, 5, 5, RGB(0xFF, 0, 0))
	if got := countInk(surf, 20, 20, RGB(0xFF, 0, 0)); got != 0 {
		t.Errorf("a zero-length dashed line drew %d pixels, want 0", got)
	}
}

// countPaintedColumns counts x columns with at least one painted pixel
// — the metric TestDrawDashedLineDrawsFewerPixelsThanASolidLine needs:
// a raw painted-PIXEL count conflates "has gaps" with the fact that an
// anti-aliased 1-unit stroke straddles two pixel ROWS per unit of
// length (roughly doubling pixel count per column versus a
// single-pixel-per-column Bresenham line), which would make a dashed
// AA line look "more painted" than a solid Bresenham one despite
// covering less horizontal span.
func countPaintedColumns(surf []byte, w, h int) int {
	n := 0
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			if isPainted(surf, w, x, y) {
				n++
				break
			}
		}
	}
	return n
}

func TestDrawDashedLineDrawsFewerPixelsThanASolidLine(t *testing.T) {
	color := RGB(0xFF, 0, 0)
	dashed := makeSurface(100, 20)
	drawDashedLine(newP(dashed, 100), 0, 10, 99, 10, color)
	solid := makeSurface(100, 20)
	drawLine(newP(solid, 100), 0, 10, 99, 10, color)

	dashedCols := countPaintedColumns(dashed, 100, 20)
	solidCols := countPaintedColumns(solid, 100, 20)
	if dashedCols == 0 {
		t.Fatal("a dashed line over a 100px span painted 0 columns")
	}
	if dashedCols >= solidCols {
		t.Errorf("dashed line painted %d of 100 columns, solid painted %d — expected dashed to cover fewer (gaps)", dashedCols, solidCols)
	}
}
