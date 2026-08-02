// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Backdrop is a decorative full-bounds ground: it fills its rectangle with a
// solid colour and, when Step > 0, overlays a regular grid of 1-unit lines
// every Step units. It draws no children and handles no events — the plain
// backing a host composites the rest of a scene on top of (a desktop wallpaper,
// a canvas backing sheet, a chart plotting area).
//
// Both colours are optional: a zero-value Fill falls back to the theme's
// Background and a zero-value Grid to the theme's Border, so a Backdrop dropped
// in with no configuration reads sensibly under any theme. A host that wants an
// exact palette (a compositor matching its own desktop colours) sets Fill and
// Grid explicitly.
//
// The grid is painted as 1-unit FillRects rather than StrokeRect hairlines so
// it renders identically on both the pixel and cell back-ends (a CellPainter
// has no sub-cell stroke); the lines start at the top-left of Bounds and repeat
// every Step, matching a host that draws a world-aligned grid from the origin.
type Backdrop struct {
	Base
	// Fill is the solid background colour. The zero value uses theme.Background.
	Fill painter.RGBA
	// Grid is the grid-line colour. The zero value uses theme.Border.
	Grid painter.RGBA
	// Step is the grid spacing in painter units. Step <= 0 draws no grid.
	Step int
}

// NewBackdrop builds a Backdrop with a solid fill and a grid every step units
// (step <= 0 = no grid). Passing the zero RGBA for either colour selects the
// theme's Background (fill) or Border (grid) at draw time.
func NewBackdrop(fill, grid painter.RGBA, step int) *Backdrop {
	return &Backdrop{Fill: fill, Grid: grid, Step: step}
}

// Draw fills the bounds and overlays the grid. An empty rectangle paints
// nothing; a non-positive Step paints only the fill.
func (b *Backdrop) Draw(p painter.Painter, theme *Theme) {
	r := b.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	fill := b.Fill
	if fill == (painter.RGBA{}) {
		fill = theme.Background
	}
	p.FillRect(r, fill)
	if b.Step <= 0 {
		return
	}
	grid := b.Grid
	if grid == (painter.RGBA{}) {
		grid = theme.Border
	}
	for gx := r.X; gx < r.X+r.W; gx += b.Step {
		p.FillRect(Rect{X: gx, Y: r.Y, W: 1, H: r.H}, grid)
	}
	for gy := r.Y; gy < r.Y+r.H; gy += b.Step {
		p.FillRect(Rect{X: r.X, Y: gy, W: r.W, H: 1}, grid)
	}
}
