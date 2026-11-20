// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"time"

	"github.com/go-widgets/painter"
)

// StackedBarChart plots a quantity over time as one bar per time bucket, each
// bar made of stacked layers (a token count split by model, a request count by
// status). It is the labelled, stacked companion to BarChart: a legend names
// every layer, the value axis shows 0, the middle and the top, and the time axis
// shows the first and last instant of TimeMin..TimeMax.
//
// Layers[l][b] is layer l's value in bucket b; buckets are equal slices of the
// time span, oldest on the left. Negative values count as zero. Display-only.
type StackedBarChart struct {
	Base
	Layers [][]float64
	// Names label the layers in the legend, in Layers order; a layer without a
	// name is left out of the legend.
	Names []string
	// Colors overrides the categorical palette, cycled by layer index.
	Colors []RGBA
	// TimeMin, TimeMax are the instants (Unix seconds) the first bucket starts
	// at and the last ends at. Zero values draw no time labels.
	TimeMin, TimeMax int64
	// Max is the top of the value axis; when <= 0 it is [NiceCeiling] of the
	// tallest stack.
	Max float64
	// FormatValue renders one value-axis label; nil uses "%.0f".
	FormatValue func(float64) string
	// FormatTime renders one time-axis label; nil uses "Jan 2 15:04".
	FormatTime func(int64) string
}

// NewStackedBarChart builds a StackedBarChart over the given layers.
func NewStackedBarChart(layers [][]float64, names []string) *StackedBarChart {
	return &StackedBarChart{Layers: layers, Names: names}
}

func (c *StackedBarChart) layerColor(i int) RGBA {
	if len(c.Colors) > 0 {
		return c.Colors[i%len(c.Colors)]
	}
	return piePalette[i%len(piePalette)]
}

// buckets is the number of bars: the longest layer.
func (c *StackedBarChart) buckets() int {
	n := 0
	for _, l := range c.Layers {
		n = max(n, len(l))
	}
	return n
}

// top is the value axis maximum.
func (c *StackedBarChart) top() float64 {
	if c.Max > 0 {
		return c.Max
	}
	most := 0.0
	for b := 0; b < c.buckets(); b++ {
		most = max(most, c.stackTotal(b))
	}
	return NiceCeiling(most)
}

// stackTotal is the height of bucket b's whole stack.
func (c *StackedBarChart) stackTotal(b int) float64 {
	sum := 0.0
	for _, l := range c.Layers {
		if b < len(l) && l[b] > 0 {
			sum += l[b]
		}
	}
	return sum
}

func (c *StackedBarChart) formatValue(v float64) string {
	if c.FormatValue != nil {
		return c.FormatValue(v)
	}
	return fmt.Sprintf("%.0f", v)
}

func (c *StackedBarChart) formatTime(at int64) string {
	if c.FormatTime != nil {
		return c.FormatTime(at)
	}
	return time.Unix(at, 0).Format("Jan 2 15:04")
}

// legendHeight is the legend strip's height: one text line plus padding.
func (c *StackedBarChart) legendHeight() int {
	return c.glyphHeight() + 2*scaled(TimeSeriesChartPad)
}

// plot is the bar area, inside the legend, the value labels and the time labels.
func (c *StackedBarChart) plot() Rect {
	r := c.Bounds()
	yAxisW := 0
	for _, v := range [3]float64{0, c.top() / 2, c.top()} {
		yAxisW = max(yAxisW, c.textWidth(c.formatValue(v)))
	}
	yAxisW += scaled(TimeSeriesChartPad)
	top := c.legendHeight()
	return Rect{X: r.X + yAxisW, Y: r.Y + top, W: r.W - yAxisW,
		H: r.H - top - c.glyphHeight() - scaled(TimeSeriesChartPad)}
}

// Draw paints the legend, the gridlines with their value labels, the stacked
// bars, and the time labels.
func (c *StackedBarChart) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	pl := c.plot()
	if r.W <= 0 || r.H <= 0 || pl.W <= 0 || pl.H <= 0 {
		return
	}
	label := dimInk(theme)
	gh := c.glyphHeight()
	pad := scaled(TimeSeriesChartPad)

	x := r.X + pad
	for i, name := range c.Names {
		if name == "" {
			continue
		}
		fillRect(p, x, r.Y+pad, gh, gh, c.layerColor(i))
		c.drawText(p, x+gh+pad, r.Y+pad, name, theme.OnSurface)
		x += gh + 2*pad + c.textWidth(name) + 2*pad
	}

	top := c.top()
	for _, frac := range [3]float64{0, 0.5, 1} {
		y := pl.Y + int((1-frac)*float64(pl.H-1))
		drawLine(p, pl.X, y, pl.X+pl.W-1, y, theme.Border)
		text := c.formatValue(frac * top)
		c.drawText(p, pl.X-pad-c.textWidth(text), y-gh/2, text, label)
	}

	n := c.buckets()
	for b := 0; b < n; b++ {
		x0 := pl.X + b*pl.W/n
		x1 := pl.X + (b+1)*pl.W/n
		w := max(1, x1-x0-1)
		base := pl.Y + pl.H - 1
		acc := 0.0
		for l, layer := range c.Layers {
			if b >= len(layer) || layer[b] <= 0 {
				continue
			}
			acc += layer[b]
			y := pl.Y + int((1-min(1, acc/top))*float64(pl.H-1))
			if base > y {
				fillRect(p, x0, y, w, base-y, c.layerColor(l))
			}
			base = min(base, y)
		}
	}

	if c.TimeMax > c.TimeMin {
		ty := pl.Y + pl.H - 1 + pad
		endText := c.formatTime(c.TimeMax)
		c.drawText(p, pl.X, ty, c.formatTime(c.TimeMin), label)
		c.drawText(p, pl.X+pl.W-c.textWidth(endText), ty, endText, label)
	}
}

// A11y describes the chart as an image with its layer and bucket counts.
func (c *StackedBarChart) A11y() A11yInfo {
	return A11yInfo{Role: RoleImg, Value: fmt.Sprintf("%d layers, %d buckets", len(c.Layers), c.buckets())}
}

var _ Accessible = (*StackedBarChart)(nil)
