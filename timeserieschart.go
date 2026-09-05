// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
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
}

// NewTimeSeriesChart builds a TimeSeriesChart over points (already in
// ascending At order) with fixed Y bounds min..max.
func NewTimeSeriesChart(points []TimePoint, min, max float64) *TimeSeriesChart {
	return &TimeSeriesChart{Points: points, Min: min, Max: max}
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
func (c *TimeSeriesChart) Draw(p painter.Painter, theme *Theme) {
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

	if len(c.Points) < 2 {
		return
	}
	first, last := c.Points[0], c.Points[len(c.Points)-1]
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
	px, py := pointAt(c.Points[0])
	for _, pt := range c.Points[1:] {
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
	return A11yInfo{Role: RoleImg, Value: fmt.Sprintf("%d points", len(c.Points))}
}

var _ Accessible = (*TimeSeriesChart)(nil)
