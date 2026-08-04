// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"sort"

	"github.com/go-widgets/painter"
)

// RadarChart plots one or more series over a set of shared axes as closed
// polygons on a polygonal (spider) grid -- the multivariate complement to
// LineChart. Each of the N Axes gets a spoke from the centre (the first at 12
// o'clock, the rest clockwise); a series' value on each axis, normalised to Max,
// sets how far out along that spoke its polygon vertex sits. Faint concentric
// N-gon rings and the spokes form the grid, each axis labelled just past its
// outer tip. Every series polygon is filled with a translucent tint of its
// colour and outlined in the solid colour. Colours cycle through the shared
// categorical palette unless Colors is set. Display-only.
//
// It renders through painter.Painter, so the same chart draws as pixels
// (WUI/GUI) or promoted cells (TUI). No axes (or a degenerate size) draws
// nothing; a series shorter than the axis count treats the missing values as 0.
type RadarChart struct {
	Base
	Axes   []string
	Series [][]float64
	Max    float64 // normalisation max; when <= 0, taken from the data
	Colors []RGBA  // optional per-series palette override; cycles by index

	// Hover + HoverAxis highlight the hovered axis spoke. Opt-in; the zero
	// value draws none.
	Hover     bool
	HoverAxis int
}

// AxisAt returns the axis whose spoke is nearest (in angle) to widget-local
// (x, y), and ok=false when the chart has no axes. Exposed so a host can show
// that axis's values on hover.
func (c *RadarChart) AxisAt(localX, localY int) (axis int, ok bool) {
	n := len(c.Axes)
	if n == 0 {
		return 0, false
	}
	dx := float64(localX) - float64(c.Bounds().W)/2
	dy := float64(localY) - float64(c.Bounds().H)/2
	if dx == 0 && dy == 0 {
		return 0, true
	}
	pa := math.Atan2(dy, dx)
	best, bestDiff := 0, 10.0
	for k := 0; k < n; k++ {
		if d := math.Abs(angleNorm(pa - axisAngle(k, n))); d < bestDiff {
			bestDiff, best = d, k
		}
	}
	return best, true
}

// angleNorm wraps a radian angle to (-π, π].
func angleNorm(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a <= -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

// drawHover highlights the hovered axis's spoke + tip when Hover is set.
func (c *RadarChart) drawHover(p painter.Painter, theme *Theme) {
	n := len(c.Axes)
	if !c.Hover || n == 0 || c.HoverAxis < 0 || c.HoverAxis >= n {
		return
	}
	r := c.Bounds()
	radius := min(r.W, r.H)/2 - ChartPad - 2*c.glyphHeight()
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	x, y := vertex(cx, cy, radius, c.HoverAxis, n, 1)
	drawLine(p, cx, cy, x, y, theme.Accent)
	fillRect(p, clampInt(x-2, r.X, r.X+r.W-4), clampInt(y-2, r.Y, r.Y+r.H-4), 4, 4, theme.Accent)
}

// RadarRings is the number of concentric grid rings drawn between the centre
// and the outer edge.
const RadarRings = 4

// NewRadarChart builds a RadarChart over the given axis labels and series.
func NewRadarChart(axes []string, series [][]float64) *RadarChart {
	return &RadarChart{Axes: axes, Series: series}
}

// top returns the effective normalisation maximum: the explicit Max when
// positive, else the largest value across every series (min 1 so an all-zero or
// empty data set still has a scale).
func (c *RadarChart) top() float64 {
	if c.Max > 0 {
		return c.Max
	}
	mx := 0.0
	for _, s := range c.Series {
		for _, v := range s {
			if v > mx {
				mx = v
			}
		}
	}
	if mx <= 0 {
		return 1
	}
	return mx
}

// seriesColor returns series i's colour: the Colors override (cycled) when set,
// else the shared categorical palette (cycled).
func (c *RadarChart) seriesColor(i int) RGBA {
	if len(c.Colors) > 0 {
		return c.Colors[i%len(c.Colors)]
	}
	return piePalette[i%len(piePalette)]
}

// axisAngle is the polar angle of axis k of n, starting at -90° (straight up)
// and running clockwise.
func axisAngle(k, n int) float64 { return -math.Pi/2 + 2*math.Pi*float64(k)/float64(n) }

// vertex returns the pixel at fraction frac (0..1) of the radius along axis k's
// spoke, given the centre and radius.
func vertex(cx, cy, radius, k, n int, frac float64) (int, int) {
	theta := axisAngle(k, n)
	x := cx + int(frac*float64(radius)*math.Cos(theta))
	y := cy + int(frac*float64(radius)*math.Sin(theta))
	return x, y
}

// ringVertices returns the N vertices of the grid/series polygon at fraction
// frac of the radius.
func ringVertices(cx, cy, radius, n int, frac float64) [][2]int {
	pts := make([][2]int, n)
	for k := 0; k < n; k++ {
		x, y := vertex(cx, cy, radius, k, n, frac)
		pts[k] = [2]int{x, y}
	}
	return pts
}

// drawPolygon strokes the closed polygon through pts in colour col.
func drawPolygon(p painter.Painter, pts [][2]int, col RGBA) {
	for i := range pts {
		j := (i + 1) % len(pts)
		drawLine(p, pts[i][0], pts[i][1], pts[j][0], pts[j][1], col)
	}
}

// fillPolygon fills the closed polygon through pts with col using an even-odd
// scanline sweep (handles the concave "star" shapes a radar series can form).
// Fewer than three vertices bound no area, so nothing is filled.
func fillPolygon(p painter.Painter, pts [][2]int, col RGBA) {
	if len(pts) < 3 {
		return
	}
	minY, maxY := pts[0][1], pts[0][1]
	for _, pt := range pts {
		if pt[1] < minY {
			minY = pt[1]
		}
		if pt[1] > maxY {
			maxY = pt[1]
		}
	}
	for y := minY; y <= maxY; y++ {
		var xs []int
		for i := range pts {
			j := (i + 1) % len(pts)
			y0, y1 := pts[i][1], pts[j][1]
			if (y0 <= y) != (y1 <= y) {
				x0, x1 := pts[i][0], pts[j][0]
				xs = append(xs, x0+(x1-x0)*(y-y0)/(y1-y0))
			}
		}
		sort.Ints(xs)
		for k := 0; k+1 < len(xs); k += 2 {
			fillRect(p, xs[k], y, xs[k+1]-xs[k]+1, 1, col)
		}
	}
}

// Draw paints the grid rings, spokes and axis labels, then each series as a
// translucent fill under a solid outline.
func (c *RadarChart) Draw(p painter.Painter, theme *Theme) {
	n := len(c.Axes)
	if n == 0 {
		return
	}
	r := c.Bounds()
	// Reserve a band around the grid for the axis labels so they sit inside
	// the widget rather than clipping at the edges.
	labelPad := 2 * c.glyphHeight()
	radius := min(r.W, r.H)/2 - ChartPad - labelPad
	if radius < 1 {
		return
	}
	cx := r.X + r.W/2
	cy := r.Y + r.H/2
	// Concentric polygonal grid rings.
	for ring := 1; ring <= RadarRings; ring++ {
		drawPolygon(p, ringVertices(cx, cy, radius, n, float64(ring)/RadarRings), theme.Border)
	}
	// Spokes centre -> outer vertex.
	for k := 0; k < n; k++ {
		x, y := vertex(cx, cy, radius, k, n, 1)
		drawLine(p, cx, cy, x, y, theme.Border)
	}
	// Axis labels centred on an anchor just beyond each tip.
	ink := dimInk(theme)
	reach := float64(radius + c.glyphHeight())
	for k := 0; k < n; k++ {
		theta := axisAngle(k, n)
		lx := cx + int(reach*math.Cos(theta))
		ly := cy + int(reach*math.Sin(theta))
		c.drawText(p, lx-c.textWidth(c.Axes[k])/2, ly-c.glyphHeight()/2, c.Axes[k], ink)
	}
	// Series polygons: translucent fill first, solid outline on top.
	mx := c.top()
	for si, s := range c.Series {
		col := c.seriesColor(si)
		pts := make([][2]int, n)
		for k := 0; k < n; k++ {
			v := 0.0
			if k < len(s) {
				v = s[k]
			}
			frac := v / mx
			if frac > 1 {
				frac = 1
			}
			if frac < 0 {
				frac = 0
			}
			x, y := vertex(cx, cy, radius, k, n, frac)
			pts[k] = [2]int{x, y}
		}
		fillPolygon(p, pts, tint(col))
		drawPolygon(p, pts, col)
	}
	c.drawHover(p, theme)
}
