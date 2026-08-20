// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// ScatterPoint is one (X, Y) sample plotted by a ScatterChart.
type ScatterPoint struct{ X, Y float64 }

// ScatterChart plots one or more series of (X, Y) points as small filled dots
// over a left+bottom axis frame -- the two-dimensional companion to LineChart.
// Both axes auto-scale to the combined data range (a flat range on either axis
// pads by ±1 so the points sit mid-plot rather than on an edge). Colours cycle
// through the shared categorical palette unless Colors is set. Display-only.
//
// It renders through painter.Painter, so the same chart draws as pixels
// (WUI/GUI) or promoted cells (TUI). An empty Series draws just the axes.
type ScatterChart struct {
	Base
	Series [][]ScatterPoint
	Colors []RGBA // optional per-series palette override; cycles by index

	// hover + hoverSeries/hoverPoint ring the hovered point; see
	// [ScatterChart.Hover] / [ScatterChart.HoverSeries] / [ScatterChart.HoverPoint].
	hover       *mvvm.Observable[bool]
	hoverSeries *mvvm.Observable[int]
	hoverPoint  *mvvm.Observable[int]
}

// NearestPoint returns the point closest (in pixels) to widget-local (x, y) —
// its series index, point index, the point, and ok=false when the chart has no
// data. Exposed so a host can show the value on hover.
func (c *ScatterChart) NearestPoint(localX, localY int) (series, point int, pt ScatterPoint, ok bool) {
	xr, yr, seen := c.ranges()
	if !seen {
		return 0, 0, ScatterPoint{}, false
	}
	r := c.Bounds()
	tx, ty := localX+r.X, localY+r.Y
	best := math.MaxInt // sentinel larger than any achievable squared pixel distance
	for si, s := range c.Series {
		for pi, q := range s {
			x, y := c.project(q, xr, yr)
			x = clampInt(x, r.X, r.X+r.W-scaled(ScatterDot))
			y = clampInt(y, r.Y, r.Y+r.H-scaled(ScatterDot))
			if d := (x-tx)*(x-tx) + (y-ty)*(y-ty); d < best {
				best, series, point, pt, ok = d, si, pi, q, true
			}
		}
	}
	return
}

// ScatterDot is the side (painter units) of the square marker drawn per point.
const ScatterDot = 2

// NewScatterChart builds a ScatterChart over the given series.
func NewScatterChart(series [][]ScatterPoint) *ScatterChart {
	return &ScatterChart{Series: series}
}

// Hover is the reactive point-ring toggle as a shared [mvvm.Observable]; false
// draws no ring. Lazily created, defaulting to off.
func (c *ScatterChart) Hover() *mvvm.Observable[bool] {
	if c.hover == nil {
		c.hover = mvvm.NewObservable(false)
	}
	return c.hover
}

// HoverSeries is the reactive hovered series index as a shared [mvvm.Observable].
// Lazily created, defaulting to 0.
func (c *ScatterChart) HoverSeries() *mvvm.Observable[int] {
	if c.hoverSeries == nil {
		c.hoverSeries = mvvm.NewObservable(0)
	}
	return c.hoverSeries
}

// HoverPoint is the reactive hovered point index as a shared [mvvm.Observable].
// Lazily created, defaulting to 0.
func (c *ScatterChart) HoverPoint() *mvvm.Observable[int] {
	if c.hoverPoint == nil {
		c.hoverPoint = mvvm.NewObservable(0)
	}
	return c.hoverPoint
}

// plot is the drawable rectangle inside the axes (shared scaled(ChartPad) margin).
func (c *ScatterChart) plot() Rect {
	r := c.Bounds()
	return Rect{X: r.X + scaled(ChartPad), Y: r.Y, W: r.W - scaled(ChartPad), H: r.H - scaled(ChartPad)}
}

// ranges returns the (minX,maxX) and (minY,maxY) spans over every point and
// whether any point was seen. A flat span pads by ±1; an empty chart reports
// seen == false so Draw can skip the point pass.
func (c *ScatterChart) ranges() (xr, yr [2]float64, seen bool) {
	for _, s := range c.Series {
		for _, pt := range s {
			if !seen {
				xr, yr, seen = [2]float64{pt.X, pt.X}, [2]float64{pt.Y, pt.Y}, true
				continue
			}
			if pt.X < xr[0] {
				xr[0] = pt.X
			}
			if pt.X > xr[1] {
				xr[1] = pt.X
			}
			if pt.Y < yr[0] {
				yr[0] = pt.Y
			}
			if pt.Y > yr[1] {
				yr[1] = pt.Y
			}
		}
	}
	if !seen {
		return xr, yr, false
	}
	if xr[0] == xr[1] {
		xr[0], xr[1] = xr[0]-1, xr[1]+1
	}
	if yr[0] == yr[1] {
		yr[0], yr[1] = yr[0]-1, yr[1]+1
	}
	return xr, yr, true
}

// seriesColor returns series i's colour: the Colors override (cycled) when set,
// else the shared categorical palette (cycled).
func (c *ScatterChart) seriesColor(i int) RGBA {
	if len(c.Colors) > 0 {
		return c.Colors[i%len(c.Colors)]
	}
	return piePalette[i%len(piePalette)]
}

// project maps a data point to its pixel in the plot area for the given spans.
func (c *ScatterChart) project(pt ScatterPoint, xr, yr [2]float64) (int, int) {
	pl := c.plot()
	fx := (pt.X - xr[0]) / (xr[1] - xr[0])
	fy := (pt.Y - yr[0]) / (yr[1] - yr[0])
	x := pl.X + int(fx*float64(pl.W-1))
	y := pl.Y + int((1-fy)*float64(pl.H-1))
	return x, y
}

// Draw paints the axis frame then one dot per point, coloured by series.
func (c *ScatterChart) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	pl := c.plot()
	baseY := pl.Y + pl.H - 1
	drawLine(p, pl.X, r.Y, pl.X, baseY, theme.Border)
	drawLine(p, pl.X, baseY, r.X+r.W-1, baseY, theme.Border)
	xr, yr, seen := c.ranges()
	if !seen {
		return
	}
	for si, s := range c.Series {
		col := c.seriesColor(si)
		for _, pt := range s {
			x, y := c.project(pt, xr, yr)
			// Keep the whole scaled(ScatterDot)×scaled(ScatterDot) marker inside Bounds(): a
			// point projected onto the right/bottom plot edge would otherwise
			// spill one dot-width past r.
			x = clampInt(x, r.X, r.X+r.W-scaled(ScatterDot))
			y = clampInt(y, r.Y, r.Y+r.H-scaled(ScatterDot))
			fillRect(p, x, y, scaled(ScatterDot), scaled(ScatterDot), col)
		}
	}
	if c.Hover().Get() && c.HoverSeries().Get() >= 0 && c.HoverSeries().Get() < len(c.Series) &&
		c.HoverPoint().Get() >= 0 && c.HoverPoint().Get() < len(c.Series[c.HoverSeries().Get()]) {
		x, y := c.project(c.Series[c.HoverSeries().Get()][c.HoverPoint().Get()], xr, yr)
		x = clampInt(x, r.X, r.X+r.W-scaled(ScatterDot))
		y = clampInt(y, r.Y, r.Y+r.H-scaled(ScatterDot))
		strokeRect(p, clampInt(x-2, r.X, r.X+r.W-6), clampInt(y-2, r.Y, r.Y+r.H-6), 6, 6, theme.OnSurface)
	}
}
