// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// LineChart plots one series of Y values as a polyline over a left+bottom axis
// frame -- the full-size sibling of the inline Sparkline. Values are spread
// evenly across the plot width and scaled vertically between Min and Max (auto-
// derived from the data when Min == Max). Display-only.
//
// It renders through painter.Painter, so the same chart draws as anti-aliased
// pixels (WUI/GUI) or promoted cells (TUI). A single point renders as a dot;
// an empty series draws just the axes.
type LineChart struct {
	Base
	Series   []float64
	Min, Max float64 // Y bounds; when equal, taken from the data
}

// ChartPad is the margin (painter units) reserved for the axes on the left and
// bottom edges of a chart's plot area.
const ChartPad = 6

// NewLineChart builds a LineChart over the given series with auto Y bounds.
func NewLineChart(series []float64) *LineChart { return &LineChart{Series: series} }

// bounds returns the effective (min, max) Y range: the explicit Min/Max when
// they differ, else the data's own range (falling back to [v-1, v+1] for a
// flat series so the polyline sits mid-plot rather than on the axis).
func (c *LineChart) yRange() (float64, float64) {
	if c.Max > c.Min {
		return c.Min, c.Max
	}
	if len(c.Series) == 0 {
		return 0, 1
	}
	mn, mx := c.Series[0], c.Series[0]
	for _, v := range c.Series[1:] {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	if mn == mx {
		return mn - 1, mx + 1
	}
	return mn, mx
}

// plot is the drawable rectangle inside the axes.
func (c *LineChart) plot() Rect {
	r := c.Bounds()
	return Rect{X: r.X + ChartPad, Y: r.Y, W: r.W - ChartPad, H: r.H - ChartPad}
}

// pointAt maps series index i to a pixel in the plot area.
func (c *LineChart) pointAt(i int, mn, mx float64) (int, int) {
	pl := c.plot()
	n := len(c.Series)
	x := pl.X
	if n > 1 {
		x = pl.X + i*(pl.W-1)/(n-1)
	}
	frac := (c.Series[i] - mn) / (mx - mn)
	y := pl.Y + int((1-frac)*float64(pl.H-1))
	return x, y
}

// Draw paints the axis frame then the polyline (or a dot for a lone point).
func (c *LineChart) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	pl := c.plot()
	// L-shaped axes: left rule + bottom rule.
	drawLine(p, pl.X, r.Y, pl.X, pl.Y+pl.H-1, theme.Border)
	drawLine(p, pl.X, pl.Y+pl.H-1, r.X+r.W-1, pl.Y+pl.H-1, theme.Border)
	if len(c.Series) == 0 {
		return
	}
	mn, mx := c.yRange()
	if len(c.Series) == 1 {
		x, y := c.pointAt(0, mn, mx)
		fillRect(p, x, y, 2, 2, theme.Accent)
		return
	}
	px, py := c.pointAt(0, mn, mx)
	for i := 1; i < len(c.Series); i++ {
		x, y := c.pointAt(i, mn, mx)
		drawLine(p, px, py, x, y, theme.Accent)
		px, py = x, y
	}
}
