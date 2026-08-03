// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// AreaChart plots one or more series of Y values as polylines whose region down
// to the baseline is filled with a semi-transparent tint of the series colour --
// the shaded sibling of LineChart. Values in each series spread evenly across
// the plot width and share a single Y scale (auto-derived from every series when
// Min == Max). Series paint back-to-front so an earlier (larger) band sits
// behind a later one. Colours cycle through the shared categorical palette
// unless Colors is set. Display-only.
//
// It renders through painter.Painter, so the same chart draws as anti-aliased
// pixels (WUI/GUI) or promoted cells (TUI). An empty Series draws just the axes;
// a lone point per series marks a dot over a single filled column.
type AreaChart struct {
	Base
	Series   [][]float64
	Min, Max float64 // shared Y bounds; when equal, taken from the data
	Colors   []RGBA  // optional per-series palette override; cycles by index
}

// AreaFillAlpha is the opacity (0..255) of the shaded band under each series,
// so overlapping bands stay legible against one another and the ground.
const AreaFillAlpha = 90

// NewAreaChart builds an AreaChart over the given series with auto Y bounds.
func NewAreaChart(series [][]float64) *AreaChart { return &AreaChart{Series: series} }

// yRange returns the effective (min, max) Y range: the explicit Min/Max when
// they differ, else the combined data range across every series (falling back to
// [v-1, v+1] for a flat range, or [0,1] when there is no data at all).
func (c *AreaChart) yRange() (float64, float64) {
	if c.Max > c.Min {
		return c.Min, c.Max
	}
	var mn, mx float64
	seen := false
	for _, s := range c.Series {
		for _, v := range s {
			if !seen {
				mn, mx, seen = v, v, true
				continue
			}
			if v < mn {
				mn = v
			}
			if v > mx {
				mx = v
			}
		}
	}
	if !seen {
		return 0, 1
	}
	if mn == mx {
		return mn - 1, mx + 1
	}
	return mn, mx
}

// plot is the drawable rectangle inside the axes (shared ChartPad margin).
func (c *AreaChart) plot() Rect {
	r := c.Bounds()
	return Rect{X: r.X + ChartPad, Y: r.Y, W: r.W - ChartPad, H: r.H - ChartPad}
}

// pointAt maps index i of series s to a pixel in the plot area.
func (c *AreaChart) pointAt(s []float64, i int, mn, mx float64) (int, int) {
	pl := c.plot()
	n := len(s)
	x := pl.X
	if n > 1 {
		x = pl.X + i*(pl.W-1)/(n-1)
	}
	frac := (s[i] - mn) / (mx - mn)
	y := pl.Y + int((1-frac)*float64(pl.H-1))
	return x, y
}

// seriesColor returns series i's colour: the Colors override (cycled) when set,
// else the shared categorical palette (cycled).
func (c *AreaChart) seriesColor(i int) RGBA {
	if len(c.Colors) > 0 {
		return c.Colors[i%len(c.Colors)]
	}
	return piePalette[i%len(piePalette)]
}

// tint returns col at the band opacity used for the filled area.
func tint(col RGBA) RGBA { return RGBA{R: col.R, G: col.G, B: col.B, A: AreaFillAlpha} }

// Draw paints the axis frame, then each series back-to-front as a filled band
// under its polyline plus the polyline stroke on top.
func (c *AreaChart) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	pl := c.plot()
	baseY := pl.Y + pl.H - 1
	drawLine(p, pl.X, r.Y, pl.X, baseY, theme.Border)
	drawLine(p, pl.X, baseY, r.X+r.W-1, baseY, theme.Border)
	if len(c.Series) == 0 {
		return
	}
	mn, mx := c.yRange()
	for si, s := range c.Series {
		if len(s) == 0 {
			continue
		}
		col := c.seriesColor(si)
		if len(s) == 1 {
			x, y := c.pointAt(s, 0, mn, mx)
			fillRect(p, x, y, 2, baseY-y+1, tint(col))
			fillRect(p, x, y, 2, 2, col)
			continue
		}
		// Fill the band segment by segment, interpolating y across each column.
		for i := 0; i < len(s)-1; i++ {
			x0, y0 := c.pointAt(s, i, mn, mx)
			x1, y1 := c.pointAt(s, i+1, mn, mx)
			for x := x0; x <= x1; x++ {
				y := y0
				if x1 != x0 {
					y = y0 + (y1-y0)*(x-x0)/(x1-x0)
				}
				fillRect(p, x, y, 1, baseY-y+1, tint(col))
			}
		}
		// Stroke the polyline on top.
		px, py := c.pointAt(s, 0, mn, mx)
		for i := 1; i < len(s); i++ {
			x, y := c.pointAt(s, i, mn, mx)
			drawLine(p, px, py, x, y, col)
			px, py = x, y
		}
	}
}
