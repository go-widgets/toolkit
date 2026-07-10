// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// BarChart plots one series of non-negative Values as vertical bars over a
// left+bottom axis frame -- the categorical companion to LineChart. Bars share
// the plot width evenly with a 1-unit gutter between them and scale to the
// tallest value (or an explicit Max). Display-only.
//
// It renders through painter.Painter, so the same chart draws as pixels
// (WUI/GUI) or promoted cells (TUI). An empty series draws just the axes.
type BarChart struct {
	Base
	Values []float64
	Max    float64 // top of the Y axis; when <= 0, taken from the data
}

// BarGutter is the horizontal gap (painter units) between adjacent bars.
const BarGutter = 1

// NewBarChart builds a BarChart over the given values with an auto Y max.
func NewBarChart(values []float64) *BarChart { return &BarChart{Values: values} }

// top returns the effective Y-axis maximum: the explicit Max when positive,
// else the largest value (min 1 so a zero/empty series still has a scale).
func (c *BarChart) top() float64 {
	if c.Max > 0 {
		return c.Max
	}
	mx := 0.0
	for _, v := range c.Values {
		if v > mx {
			mx = v
		}
	}
	if mx <= 0 {
		return 1
	}
	return mx
}

// plot is the drawable rectangle inside the axes (shared geometry with
// LineChart's ChartPad margin).
func (c *BarChart) plot() Rect {
	r := c.Bounds()
	return Rect{X: r.X + ChartPad, Y: r.Y, W: r.W - ChartPad, H: r.H - ChartPad}
}

// Draw paints the axis frame then one Accent bar per value.
func (c *BarChart) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	pl := c.plot()
	baseY := pl.Y + pl.H - 1
	drawLine(p, pl.X, r.Y, pl.X, baseY, theme.Border)
	drawLine(p, pl.X, baseY, r.X+r.W-1, baseY, theme.Border)
	n := len(c.Values)
	if n == 0 {
		return
	}
	top := c.top()
	slot := pl.W / n
	if slot < 1 {
		slot = 1
	}
	bw := slot - BarGutter
	if bw < 1 {
		bw = 1
	}
	for i, v := range c.Values {
		if v <= 0 {
			continue
		}
		frac := v / top
		if frac > 1 {
			frac = 1
		}
		bh := int(frac * float64(pl.H-1))
		if bh < 1 {
			bh = 1
		}
		bx := pl.X + 1 + i*slot
		fillRect(p, bx, baseY-bh, bw, bh, theme.Accent)
	}
}
