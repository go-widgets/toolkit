// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"github.com/go-widgets/mvvm"
	"time"

	"github.com/go-widgets/painter"
)

// TimeSeriesChartPad separates a TimeSeriesChart's plot area from its own
// axis-label text, on whichever side that text sits.
const TimeSeriesChartPad = 4

// TimePoint is one (time, value) sample a TimeSeriesChart plots.
type TimePoint struct {
	// At is a Unix timestamp (seconds).
	At int64
	// Value is the sample itself, in whatever unit Min/Max are given in
	// (a raw count, a percentage 0-100, ...).
	Value float64
}

// TimeSeriesChart draws one time-ordered series as a polyline against a
// LABELED value axis (left: Min/mid/Max) and a LABELED time axis
// (bottom: Points[0] and the last point) — the labeled counterpart to
// LineChart, whose x-axis is index-order and carries no axis text at
// all. Points are placed by ACTUAL elapsed time (At), not by
// evenly-spaced index the way LineChart spaces Series — so irregular
// sampling (a missed poll, say) draws an honest gap in pace rather than
// pretending every interval was the same length.
//
// Points must already be in ascending At order (the same order
// LineChart's Series is assumed to already be in display order) —
// TimeSeriesChart does not sort. A point with fewer than two entries
// draws just the labeled axes, no curve: nothing to connect, and no
// meaningful span to put a time label at either end of.
//
// Unlike LineChart's Min==Max-means-auto-derive convenience, Min and Max
// here are always taken as given: a percentage chart's whole point is a
// fixed 0-100 scale, not one that rescales to whatever the data happens
// to reach today.
//
// Renders through painter.Painter like every other chart in this
// package — GUI pixels or promoted TUI cells — via drawLine/putPixel and
// the package's own installed font (Base.drawText), never a vector-path
// stroke (which has no TUI equivalent) and never painter's own built-in
// font directly (a caller drawing text via painter.PixelPainter.Text
// gets ITS prototype-scope font — uppercase/digits only, see
// go-widgets/painter/font.go's own doc comment — not this package's
// complete one).
type TimeSeriesChart struct {
	Base
	Points   []TimePoint
	Min, Max float64
	// Ink is the curve's color. The zero value uses theme.Accent.
	Ink RGBA
	// FormatValue renders one y-axis label, at Min, the midpoint, and
	// Max. A nil FormatValue uses a plain "%.0f".
	FormatValue func(float64) string
	// FormatTime renders one x-axis label, at Points[0] and the last
	// point. A nil FormatTime uses time.Unix(t, 0).Format("Jan 2 15:04")
	// — always a real date, never elided for "today": a chart is read
	// whenever someone happens to open it, not only the day a point was
	// recorded, so a label missing its date is just a clock.
	FormatTime func(int64) string

	// FollowPeak makes the chart choose its own Max from the data instead of
	// being told one: it rises to a new peak AT ONCE and comes back down
	// slowly. Min is left alone.
	//
	// A fixed ceiling is wrong in both directions on live data. Too low and a
	// burst is drawn off the top, where nobody can measure it; too high and
	// everything else is a flat line along the bottom. And a ceiling that
	// simply tracked the peak would rescale the whole picture every time a
	// spike scrolled off the left -- a graph whose axis keeps moving cannot be
	// read at all.
	FollowPeak bool

	// series is the bindable form of Points, created by Series().
	series *mvvm.ObservableList[TimePoint]
	// ceiling is where FollowPeak has the scale now.
	ceiling float64
}

// NewTimeSeriesChart builds a TimeSeriesChart over points (already in
// ascending At order) with fixed Y bounds min..max.
func NewTimeSeriesChart(points []TimePoint, min, max float64) *TimeSeriesChart {
	return &TimeSeriesChart{Points: points, Min: min, Max: max}
}

// Series is the chart's points as a shared [mvvm.ObservableList], so a host
// binds its model to the chart instead of assigning a field and hoping
// something repaints.
//
// A LIST, not an Observable: mvvm.Observable is constrained to comparable
// types so it can skip a notification when nothing changed, and a slice is not
// comparable. That constraint is why no chart here had a bindable series at
// all -- the vehicle for one is ObservableList, which also says WHAT changed
// rather than only that something did.
//
// It is created on first use, seeded from [TimeSeriesChart.Points] -- and from
// then on IT is what the chart draws. Two sources for one truth is how a chart
// comes to show last minute's data; the field stays as the way to give initial
// points to a chart nobody binds, and taking the list settles which one wins.
func (c *TimeSeriesChart) Series() *mvvm.ObservableList[TimePoint] {
	if c.series == nil {
		c.series = mvvm.NewObservableList(c.Points...)
	}
	return c.series
}

// points is what the chart draws: the observable once somebody has taken it,
// the field until then.
func (c *TimeSeriesChart) points() []TimePoint {
	if c.series != nil {
		return c.series.Slice()
	}
	return c.Points
}

func (c *TimeSeriesChart) formatValue(v float64) string {
	if c.FormatValue != nil {
		return c.FormatValue(v)
	}
	return fmt.Sprintf("%.0f", v)
}

func (c *TimeSeriesChart) formatTime(at int64) string {
	if c.FormatTime != nil {
		return c.FormatTime(at)
	}
	return time.Unix(at, 0).Format("Jan 2 15:04")
}

// plot is the drawable rectangle inside both axes' label space.
func (c *TimeSeriesChart) plot() Rect {
	r := c.Bounds()
	yAxisW := c.valueAxisWidth()
	xAxisH := c.glyphHeight() + scaled(TimeSeriesChartPad)
	return Rect{X: r.X + yAxisW, Y: r.Y, W: r.W - yAxisW, H: r.H - xAxisH}
}

// valueAxisWidth is how wide the left label column needs to be: the
// widest of the three value labels actually drawn, plus padding.
func (c *TimeSeriesChart) valueAxisWidth() int {
	widest := 0
	for _, v := range [3]float64{c.Min, (c.Min + c.Max) / 2, c.Max} {
		if w := c.textWidth(c.formatValue(v)); w > widest {
			widest = w
		}
	}
	return widest + scaled(TimeSeriesChartPad)
}

// Draw paints the value axis (gridlines + labels), the time axis (start
// + end labels, only when there's a span to label), and the polyline.
// followPeak moves the scale towards what the data now needs, and reports the
// ceiling to draw against.
func (c *TimeSeriesChart) followPeak() {
	if !c.FollowPeak {
		return
	}
	high := 0.0
	for _, pt := range c.points() {
		if pt.Value > high {
			high = pt.Value
		}
	}
	want := NiceCeiling(high)
	switch {
	case c.ceiling <= 0 || want >= c.ceiling:
		c.ceiling = want
	default:
		// A fifth of the distance each draw, so a quiet minute lowers the
		// scale and a single quiet sample does not.
		next := c.ceiling - (c.ceiling-want)/5
		// Approaching by fifths never arrives: the gap halves for ever and the
		// axis would read 1.0000000000000004 rather than 1. Close enough is
		// there.
		if next <= want*1.001 {
			next = want
		}
		c.ceiling = next
	}
	c.Max = c.ceiling
}

// NiceCeiling rounds a value up to a power of two, or to a half or three
// quarters of one: 1, 1.5, 2, 3, 4, 6, 8 and so on.
//
// Those are the gradations a quantity is spoken in -- half a mebibyte, three
// quarters of a gibibyte -- so an axis reads as a number rather than as
// whatever the peak happened to be, and two charts sharing a scale land on the
// same marks.
func NiceCeiling(v float64) float64 {
	if v <= 0 {
		return 1
	}
	step := 1.0
	for step < v {
		step *= 2
	}
	for _, f := range []float64{0.5, 0.75} {
		if c := step * f; c >= v {
			return c
		}
	}
	return step
}

func (c *TimeSeriesChart) Draw(p painter.Painter, theme *Theme) {
	c.followPeak()
	r := c.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	pl := c.plot()
	if pl.W <= 0 || pl.H <= 0 {
		return
	}
	label := dimInk(theme)
	gh := c.glyphHeight()

	for _, frac := range [3]float64{0, 0.5, 1} {
		y := pl.Y + int((1-frac)*float64(pl.H-1))
		drawLine(p, pl.X, y, pl.X+pl.W-1, y, theme.Border)
		text := c.formatValue(c.Min + frac*(c.Max-c.Min))
		tx := pl.X - scaled(TimeSeriesChartPad) - c.textWidth(text)
		c.drawText(p, tx, y-gh/2, text, label)
	}

	if len(c.points()) < 2 {
		return
	}
	pts := c.points()
	first, last := pts[0], pts[len(pts)-1]
	span := last.At - first.At
	if span <= 0 {
		return
	}

	ink := c.Ink
	if ink == (RGBA{}) {
		ink = theme.Accent
	}
	valueSpan := c.Max - c.Min
	pointAt := func(t TimePoint) (int, int) {
		frac := float64(t.At-first.At) / float64(span)
		x := pl.X + int(frac*float64(pl.W-1))
		vf := 0.0
		if valueSpan != 0 {
			vf = (t.Value - c.Min) / valueSpan
		}
		vf = min(1, max(0, vf))
		y := pl.Y + int((1-vf)*float64(pl.H-1))
		return x, y
	}
	px, py := pointAt(pts[0])
	for _, pt := range pts[1:] {
		x, y := pointAt(pt)
		drawLine(p, px, py, x, y, ink)
		px, py = x, y
	}

	ty := pl.Y + pl.H - 1 + scaled(TimeSeriesChartPad)
	startText := c.formatTime(first.At)
	endText := c.formatTime(last.At)
	c.drawText(p, pl.X, ty, startText, label)
	c.drawText(p, pl.X+pl.W-c.textWidth(endText), ty, endText, label)
}

// A11y reports the TimeSeriesChart as an img carrying its point count —
// the same convention Sparkline (its inline-sized sibling) and LineChart
// use, since a full time-value series has no single "current" reading to
// report the way a Gauge's RoleMeter does.
func (c *TimeSeriesChart) A11y() A11yInfo {
	return A11yInfo{Role: RoleImg, Value: fmt.Sprintf("%d points", len(c.points()))}
}

var _ Accessible = (*TimeSeriesChart)(nil)
