// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

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

	// Hover + HoverIndex outline the hovered bar's column. Opt-in; the zero
	// value draws none.
	hover      *mvvm.Observable[bool]
	hoverIndex *mvvm.Observable[int]
}

// BarGutter is the horizontal gap (painter units) between adjacent bars.
const BarGutter = 1

// NewBarChart builds a BarChart over the given values with an auto Y max.
func NewBarChart(values []float64) *BarChart { return &BarChart{Values: values} }

// Hover is the reactive hover-highlight toggle as a shared [mvvm.Observable];
// false draws no hover affordance. Lazily created, defaulting to off.
func (c *BarChart) Hover() *mvvm.Observable[bool] {
	if c.hover == nil {
		c.hover = mvvm.NewObservable(false)
	}
	return c.hover
}

// HoverIndex is the reactive hovered index as a shared [mvvm.Observable].
// Lazily created, defaulting to 0.
func (c *BarChart) HoverIndex() *mvvm.Observable[int] {
	if c.hoverIndex == nil {
		c.hoverIndex = mvvm.NewObservable(0)
	}
	return c.hoverIndex
}

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
// LineChart's scaled(ChartPad) margin).
func (c *BarChart) plot() Rect {
	r := c.Bounds()
	return Rect{X: r.X + scaled(ChartPad), Y: r.Y, W: r.W - scaled(ChartPad), H: r.H - scaled(ChartPad)}
}

// ValueAt maps a widget-local x to the bar under it, returning its index and
// value; ok is false for an empty chart or an x outside every bar slot. Exposed
// so a host can show the underlying value on hover.
func (c *BarChart) ValueAt(localX int) (index int, value float64, ok bool) {
	n := len(c.Values)
	if n == 0 {
		return 0, 0, false
	}
	slot := c.plot().W / n
	if slot < 1 {
		slot = 1
	}
	rel := localX - scaled(ChartPad)
	if rel < 0 {
		return 0, 0, false
	}
	if idx := rel / slot; idx < n {
		return idx, c.Values[idx], true
	}
	return 0, 0, false
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
	bw := slot - scaled(BarGutter)
	if bw < 1 {
		bw = 1
	}
	// Clip the bars to the plot so a series with more bars than the box is wide
	// (slot floored at 1) never paints past the right edge.
	withClip(p, Rect{X: pl.X, Y: r.Y, W: r.X + r.W - pl.X, H: r.H}, func() {
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
		if c.Hover().Get() && c.HoverIndex().Get() >= 0 && c.HoverIndex().Get() < n {
			hw := bw + 2
			hx := clampInt(pl.X+c.HoverIndex().Get()*slot, r.X, r.X+r.W-hw)
			strokeRect(p, hx, pl.Y, hw, baseY-pl.Y+1, theme.OnSurface)
		}
	})
}
