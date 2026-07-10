// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"

	"github.com/go-widgets/painter"
)

// PieChart plots proportional Values as wedges of a filled disc -- the part-of-
// whole complement to LineChart/BarChart. Wedges start at 12 o'clock and run
// clockwise, sized by each value's share of the total. Colours cycle through a
// built-in categorical palette unless Colors is set. Display-only.
//
// It fills each wedge per-pixel over painter.Painter's putPixel (no arc
// primitive needed), so it renders as pixels (WUI/GUI) or promoted cells (TUI).
// A zero or empty total draws nothing.
type PieChart struct {
	Base
	Values []float64
	Colors []RGBA // optional per-slice palette override; cycles by index
}

// piePalette is a categorical colour set (Tableau-10-derived) chosen to stay
// distinct on both light and dark surfaces.
var piePalette = []RGBA{
	{R: 0x4E, G: 0x79, B: 0xA7, A: 255}, // blue
	{R: 0xF2, G: 0x8E, B: 0x2B, A: 255}, // orange
	{R: 0xE1, G: 0x57, B: 0x59, A: 255}, // red
	{R: 0x76, G: 0xB7, B: 0xB2, A: 255}, // teal
	{R: 0x59, G: 0xA1, B: 0x4F, A: 255}, // green
	{R: 0xED, G: 0xC9, B: 0x48, A: 255}, // yellow
}

// NewPieChart builds a PieChart over the given values with the default palette.
func NewPieChart(values []float64) *PieChart { return &PieChart{Values: values} }

// total sums the positive values (negatives are treated as zero so a stray
// sign can't invert a wedge).
func (c *PieChart) total() float64 {
	t := 0.0
	for _, v := range c.Values {
		if v > 0 {
			t += v
		}
	}
	return t
}

// cumFractions returns the running share [0..1] through each slice, given the
// total. Slice i covers [cum[i-1], cum[i]) of the circle.
func (c *PieChart) cumFractions(total float64) []float64 {
	cum := make([]float64, len(c.Values))
	run := 0.0
	for i, v := range c.Values {
		if v > 0 {
			run += v
		}
		cum[i] = run / total
	}
	return cum
}

// sliceColor returns the colour for slice i: the Colors override (cycled) when
// set, else the built-in palette (cycled).
func (c *PieChart) sliceColor(i int) RGBA {
	if len(c.Colors) > 0 {
		return c.Colors[i%len(c.Colors)]
	}
	return piePalette[i%len(piePalette)]
}

// sliceOf returns the index of the slice whose cumulative range contains the
// fraction f in [0,1): the first slice whose running share exceeds f, or the
// last slice for f pinned at the boundary.
func sliceOf(cum []float64, f float64) int {
	for i, c := range cum {
		if f < c {
			return i
		}
	}
	return len(cum) - 1
}

// Draw fills the disc, colouring each pixel by the wedge its angle falls in.
func (c *PieChart) Draw(p painter.Painter, theme *Theme) {
	_ = theme
	total := c.total()
	if total <= 0 {
		return
	}
	r := c.Bounds()
	radius := min(r.W, r.H) / 2
	if radius < 1 {
		return
	}
	cx := r.X + r.W/2
	cy := r.Y + r.H/2
	cum := c.cumFractions(total)
	rf := float64(radius)
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			dx := float64(x) + 0.5 - float64(cx)
			dy := float64(y) + 0.5 - float64(cy)
			if math.Hypot(dx, dy) > rf {
				continue
			}
			// Angle measured clockwise from 12 o'clock, normalised to [0,2π).
			theta := math.Atan2(dx, -dy)
			if theta < 0 {
				theta += 2 * math.Pi
			}
			putPixel(p, x, y, c.sliceColor(sliceOf(cum, theta/(2*math.Pi))))
		}
	}
}
