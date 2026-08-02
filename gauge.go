// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"

	"github.com/go-widgets/painter"
)

// GaugeThickness is the default arc stroke width in pixels, used when
// Gauge.Thickness is left at its zero value. It leaves room inside the
// ring for the centred Caption on the toolkit's 5x7 font while keeping
// the track visually prominent.
const GaugeThickness = 6

// gaugeStart is the arc's start angle and gaugeSweep its total angular
// extent, both measured clockwise from 12 o'clock (the PieChart angle
// convention: theta = atan2(dx, -dy)). The arc starts at the lower-left
// (7:30, 225°), sweeps 270° clockwise over the top, and ends at the
// lower-right (4:30, 135°), leaving a 90° gap centred on 6 o'clock. A
// pixel's position along the arc is phi = theta - gaugeStart wrapped
// into [0, 2π): phi in [0, gaugeSweep] is on the track, and the rest is
// the bottom gap.
const (
	gaugeStart = 5 * math.Pi / 4
	gaugeSweep = 3 * math.Pi / 2
)

// GaugeBand is a coloured segment of a Gauge's track up to the value
// Upto: the band whose Upto first reaches (>=) the current Value gives
// the value arc its colour. Bands are consulted in slice order, so
// callers list them by ascending Upto (e.g. a green "ok" band, then a
// yellow "warn" band, then a red "critical" band).
type GaugeBand struct {
	Upto  float64
	Color RGBA
}

// Gauge is a radial arc gauge: a 270° track from the lower-left to the
// lower-right representing the range Min..Max, filled up to Value, with
// optional coloured threshold Bands and a centred Caption. Unlike
// ProgressCircle (a full ring that "fills up") it carries a value scale
// and colour zones, the display-only counterpart to a dashboard dial.
//
// It rasterises the arc per-pixel over painter.Painter's putPixel (the
// same approach as PieChart — no arc primitive is added): the background
// track paints in theme.SurfaceAlt across the whole sweep, and the value
// arc paints from the start up to frac() in theme.Accent (or, when Bands
// is set, the colour of the band matching Value). The Caption is drawn
// centred in Base.Font. Value is clamped to [Min, Max] via frac.
type Gauge struct {
	Base
	Min, Max  float64
	Value     float64
	Bands     []GaugeBand
	Caption   string
	Thickness int
}

// NewGauge constructs a Gauge over the range min..max at the given value.
func NewGauge(min, max, value float64) *Gauge {
	return &Gauge{Min: min, Max: max, Value: value}
}

// frac is the clamped fill fraction (Value-Min)/(Max-Min) in [0, 1]. A
// non-positive span (the degenerate Max==Min, or Max<Min) yields 0 so
// there is no divide-by-zero and an unscaled gauge simply reads empty.
func (g *Gauge) frac() float64 {
	span := g.Max - g.Min
	if span <= 0 {
		return 0
	}
	f := (g.Value - g.Min) / span
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// valueColor is the colour the value arc is painted in: theme.Accent
// when no Bands are set, else the Color of the first band whose Upto
// reaches Value (the zone Value falls in), or the last band's Color when
// Value exceeds every threshold.
func (g *Gauge) valueColor(theme *Theme) RGBA {
	if len(g.Bands) == 0 {
		return theme.Accent
	}
	for _, b := range g.Bands {
		if g.Value <= b.Upto {
			return b.Color
		}
	}
	return g.Bands[len(g.Bands)-1].Color
}

// Draw paints the background track arc, the value arc up to frac(), and
// the centred Caption. It is a no-op for an empty or sub-pixel bounds so
// a hidden or collapsed gauge draws nothing and never panics.
func (g *Gauge) Draw(p painter.Painter, theme *Theme) {
	r := g.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	side := min(r.W, r.H)
	outerR := side / 2
	if outerR < 1 {
		return
	}
	thick := g.Thickness
	if thick <= 0 {
		thick = GaugeThickness
	}
	innerR := outerR - thick
	if innerR < 0 {
		innerR = 0
	}
	cx := r.X + r.W/2
	cy := r.Y + r.H/2
	outerF := float64(outerR)
	innerF := float64(innerR)
	frac := g.frac()
	valCol := g.valueColor(theme)
	for y := cy - outerR; y <= cy+outerR; y++ {
		for x := cx - outerR; x <= cx+outerR; x++ {
			dx := float64(x) + 0.5 - float64(cx)
			dy := float64(y) + 0.5 - float64(cy)
			dist := math.Hypot(dx, dy)
			if dist > outerF {
				continue
			}
			if dist < innerF {
				continue
			}
			// Angle clockwise from 12 o'clock, normalised to [0, 2π).
			theta := math.Atan2(dx, -dy)
			if theta < 0 {
				theta += 2 * math.Pi
			}
			// Position along the arc from the lower-left start.
			phi := theta - gaugeStart
			if phi < 0 {
				phi += 2 * math.Pi
			}
			if phi > gaugeSweep {
				continue // bottom gap
			}
			pf := phi / gaugeSweep
			if frac > 0 && pf <= frac {
				putPixel(p, x, y, valCol)
			} else {
				putPixel(p, x, y, theme.SurfaceAlt)
			}
		}
	}
	if g.Caption != "" {
		tw := g.textWidth(g.Caption)
		tx := cx - tw/2
		ty := cy - g.glyphHeight()/2
		g.drawText(p, tx, ty, g.Caption, theme.OnSurface)
	}
}
