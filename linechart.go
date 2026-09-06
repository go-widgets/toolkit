// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

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

	// Hover + HoverIndex drive a hover crosshair: when Hover is set, Draw paints
	// a vertical rule at data point HoverIndex and a marker where it meets the
	// curve. A host sets these from ValueAt on pointer motion; the zero value
	// (Hover == false) draws no crosshair, so existing renders are unchanged.
	hover *mvvm.Observable[bool]
	// values is the bindable form of Series, created by Values().
	values     *mvvm.ObservableList[float64]
	hoverIndex *mvvm.Observable[int]
}

// ChartPad is the margin (painter units) reserved for the axes on the left and
// bottom edges of a chart's plot area.
const ChartPad = 6

// NewLineChart builds a LineChart over the given series with auto Y bounds.
func NewLineChart(series []float64) *LineChart { return &LineChart{Series: series} }

// Values is the curve as a shared [mvvm.ObservableList], so a host binds its
// model to the chart instead of assigning a field and hoping something
// repaints.
//
// A LIST, not an Observable: mvvm.Observable is constrained to comparable
// types so it can skip a notification when nothing changed, and a slice is not
// comparable. That is why no chart here had a bindable series at all.
//
// Created on first use, seeded from [LineChart.Series] -- and from then on IT
// is what the chart draws, because two sources for one truth is how a chart
// comes to show last minute's data.
func (c *LineChart) Values() *mvvm.ObservableList[float64] {
	if c.values == nil {
		c.values = mvvm.NewObservableList(c.Series...)
	}
	return c.values
}

// series is what the chart draws: the list once somebody has taken it, the
// field until then.
func (c *LineChart) series() []float64 {
	if c.values != nil {
		return c.values.Slice()
	}
	return c.Series
}

// Hover is the reactive hover-highlight toggle as a shared [mvvm.Observable];
// false draws no hover affordance. Lazily created, defaulting to off.
func (c *LineChart) Hover() *mvvm.Observable[bool] {
	if c.hover == nil {
		c.hover = mvvm.NewObservable(false)
	}
	return c.hover
}

// HoverIndex is the reactive hovered index as a shared [mvvm.Observable].
// Lazily created, defaulting to 0.
func (c *LineChart) HoverIndex() *mvvm.Observable[int] {
	if c.hoverIndex == nil {
		c.hoverIndex = mvvm.NewObservable(0)
	}
	return c.hoverIndex
}

// bounds returns the effective (min, max) Y range: the explicit Min/Max when
// they differ, else the data's own range (falling back to [v-1, v+1] for a
// flat series so the polyline sits mid-plot rather than on the axis).
func (c *LineChart) yRange() (float64, float64) {
	if c.Max > c.Min {
		return c.Min, c.Max
	}
	if len(c.series()) == 0 {
		return 0, 1
	}
	mn, mx := c.series()[0], c.series()[0]
	for _, v := range c.series()[1:] {
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
	return Rect{X: r.X + scaled(ChartPad), Y: r.Y, W: r.W - scaled(ChartPad), H: r.H - scaled(ChartPad)}
}

// ValueAt maps a widget-local x to the nearest plotted point, returning its
// index and value (ok=false only for an empty series). Exposed so a host can
// show the underlying value on hover.
func (c *LineChart) ValueAt(localX int) (index int, value float64, ok bool) {
	n := len(c.series())
	if n == 0 {
		return 0, 0, false
	}
	if n == 1 {
		return 0, c.series()[0], true
	}
	span := c.plot().W - 1
	if span < 1 {
		return 0, c.series()[0], true
	}
	rel := localX - scaled(ChartPad)
	idx := clampInt((2*rel*(n-1)+span)/(2*span), 0, n-1) // nearest index
	return idx, c.series()[idx], true
}

// pointAt maps series index i to a pixel in the plot area.
func (c *LineChart) pointAt(i int, mn, mx float64) (int, int) {
	pl := c.plot()
	n := len(c.series())
	x := pl.X
	if n > 1 {
		x = pl.X + i*(pl.W-1)/(n-1)
	}
	frac := (c.series()[i] - mn) / (mx - mn)
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
	if len(c.series()) == 0 {
		return
	}
	mn, mx := c.yRange()
	if len(c.series()) == 1 {
		x, y := c.pointAt(0, mn, mx)
		fillRect(p, x, y, 2, 2, theme.Accent)
	} else {
		px, py := c.pointAt(0, mn, mx)
		for i := 1; i < len(c.series()); i++ {
			x, y := c.pointAt(i, mn, mx)
			drawLine(p, px, py, x, y, theme.Accent)
			px, py = x, y
		}
	}
	c.drawHover(p, theme, mn, mx)
}

// drawHover paints the hover crosshair — a vertical rule at data point
// HoverIndex plus a marker where it meets the curve — when Hover is set and
// HoverIndex is in range. The marker is clamped inside Bounds.
func (c *LineChart) drawHover(p painter.Painter, theme *Theme, mn, mx float64) {
	if !c.Hover().Get() || c.HoverIndex().Get() < 0 || c.HoverIndex().Get() >= len(c.series()) {
		return
	}
	r, pl := c.Bounds(), c.plot()
	hx, hy := c.pointAt(c.HoverIndex().Get(), mn, mx)
	drawLine(p, hx, r.Y, hx, pl.Y+pl.H-1, dimInk(theme))
	fillRect(p, clampInt(hx-2, r.X, r.X+r.W-4), clampInt(hy-2, r.Y, r.Y+r.H-4), 4, 4, theme.Accent)
}
