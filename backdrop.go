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
//
// A Backdrop is event-transparent by default. It is typically the first,
// full-cover child of a scene, over which a host composites the interactive
// widgets. Because a container routes an event to the first child whose
// HitTest covers the point (see Overlay), a full-cover Backdrop that reported
// hits would intercept every click meant for a widget drawn on top of it. So
// its HitTest returns false by default and pointer events pass THROUGH to the
// siblings/content behind it — the same "decorative, non-interactive" idiom as
// Label and Scrollbar. Set Interactive to opt back in (e.g. a modal scrim that
// deliberately swallows clicks aimed at the content beneath it).
type Backdrop struct {
	Base
	// Fill is the solid background colour. The zero value uses theme.Background.
	Fill painter.RGBA
	// Grid is the grid-line colour. The zero value uses theme.Border.
	Grid painter.RGBA
	// Step is the grid spacing in painter units. Step <= 0 draws no grid.
	Step int
	// Radius rounds the filled rectangle's corners by that many units. The zero
	// value (0) fills a plain rectangle, byte-identical to before this field
	// existed; a positive value fills a rounded rectangle — the ground of a pill /
	// chip / badge a host composites an icon or label over, so that ground is a
	// widget rather than a hand-drawn FillRoundRect. A grid (Step > 0) is drawn as
	// before, unaffected by the rounding.
	Radius int
	// Stroke, when its alpha is non-zero, outlines the (optionally rounded) fill in
	// that colour — the border of a pill / chip. The zero value (A==0) draws no
	// border, byte-identical to before this field existed.
	Stroke RGBA
	// StrokeWidth is the border thickness in units; it applies only when Stroke is
	// set, and a value < 1 is treated as 1.
	StrokeWidth int
	// NoFill suppresses the ground fill, leaving only the Stroke (and the grid, if
	// any): an outline-only decoration drawn OVER content that has to stay
	// visible — a focus ring around a pane, a drop-target highlight, a selection
	// marquee. Without it such an outline is a hand-drawn StrokeRoundRect in the
	// host, because a zero-value Fill means "the theme's Background" rather than
	// "no background", and there is no transparent colour that says otherwise.
	// The zero value (false) fills as before, byte-identical.
	NoFill bool
	// GradientTo, when its alpha is non-zero, fills the ground as a linear
	// gradient from Fill (the start edge) to GradientTo (the end edge) along
	// GradientDir, instead of a solid Fill — the toolbar/panel face a host would
	// otherwise hand-draw with a per-pixel PutPixel loop. Gradient fills a
	// rectangle (Radius is ignored while it is set). The zero value (A==0) keeps
	// the solid Fill, byte-identical to before this field existed.
	GradientTo painter.RGBA
	// GradientDir is the gradient's direction — vertical (the default), horizontal,
	// diagonal or cross-diagonal. Meaningful only when GradientTo is set.
	GradientDir GradientDir
	// Bevel draws a 1-pixel 3D bevel around the fill: none (the default), raised
	// (a bright top+left over a dark bottom+right — a pushed-out Fluxbox toolbar
	// section) or sunken (the inverse). The zero value (BevelNone) draws no bevel,
	// byte-identical to before this field existed.
	Bevel BevelKind

	// Interactive makes the Backdrop catch pointer events. The zero value
	// (false) is event-transparent: HitTest returns false so clicks pass
	// through to whatever is composited over the backdrop — the least-
	// surprising default for a decorative ground. Set it true for a backdrop
	// that should consume clicks (a modal scrim shielding the content beneath).
	Interactive bool
}

// GradientDir is the direction of a Backdrop's linear gradient fill.
type GradientDir int

const (
	// GradientVertical runs the gradient top (Fill) to bottom (GradientTo).
	GradientVertical GradientDir = iota
	// GradientHorizontal runs it left (Fill) to right (GradientTo).
	GradientHorizontal
	// GradientDiagonal runs it top-left (Fill) to bottom-right (GradientTo).
	GradientDiagonal
	// GradientCrossDiagonal runs it top-right (Fill) to bottom-left (GradientTo).
	GradientCrossDiagonal
)

// BevelKind selects a Backdrop's 1-pixel 3D edge bevel.
type BevelKind int

const (
	// BevelNone draws no bevel (the default).
	BevelNone BevelKind = iota
	// BevelRaised draws a bright top+left over a dark bottom+right, so the face
	// reads as pushed out toward the viewer.
	BevelRaised
	// BevelSunken is the inverse — dark top+left, bright bottom+right — so the
	// face reads as pressed in.
	BevelSunken
)

// fillGradient fills r with a linear gradient from `from` to `to` along dir. The
// axis-aligned directions fill a line at a time; the diagonals go per-pixel (the
// only way to vary along both axes), mirroring the classic Fluxbox toolbar
// gradients. A single-pixel extent along the axis collapses to `from`.
func fillGradient(p painter.Painter, r Rect, from, to RGBA, dir GradientDir) {
	switch dir {
	case GradientHorizontal:
		for i := 0; i < r.W; i++ {
			fillRect(p, r.X+i, r.Y, 1, r.H, lerpRGBA(from, to, i, r.W))
		}
	case GradientDiagonal, GradientCrossDiagonal:
		den := (r.W - 1) + (r.H - 1) + 1
		for j := 0; j < r.H; j++ {
			for i := 0; i < r.W; i++ {
				step := i + j
				if dir == GradientCrossDiagonal {
					step = (r.W - 1 - i) + j
				}
				p.PutPixel(r.X+i, r.Y+j, lerpRGBA(from, to, step, den))
			}
		}
	default: // GradientVertical
		for j := 0; j < r.H; j++ {
			fillRect(p, r.X, r.Y+j, r.W, 1, lerpRGBA(from, to, j, r.H))
		}
	}
}

// NewBackdrop builds a Backdrop with a solid fill and a grid every step units
// (step <= 0 = no grid). Passing the zero RGBA for either colour selects the
// theme's Background (fill) or Border (grid) at draw time.
func NewBackdrop(fill, grid painter.RGBA, step int) *Backdrop {
	return &Backdrop{Fill: fill, Grid: grid, Step: step}
}

// HitTest reports whether the Backdrop should receive a pointer event at
// (px, py). It returns false unless Interactive is set, so by default a
// full-cover backdrop lets clicks pass through to the widgets composited over
// it (the Label/Scrollbar pass-through idiom). When Interactive is set it
// behaves like any other widget, hit-testing against its Bounds.
func (b *Backdrop) HitTest(px, py int) bool {
	if !b.Interactive {
		return false
	}
	return b.Bounds().Contains(px, py)
}

// Draw fills the bounds and overlays the grid. An empty rectangle paints
// nothing; a non-positive Step paints only the fill. A positive Radius fills a
// rounded rectangle instead of a plain one; a non-zero Stroke outlines it. With
// NoFill set the fill is skipped entirely and only the outline (and grid) is
// painted, leaving whatever is already there showing through.
func (b *Backdrop) Draw(p painter.Painter, theme *Theme) {
	r := b.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if !b.NoFill {
		fill := b.Fill
		if fill == (painter.RGBA{}) {
			fill = theme.Background
		}
		switch {
		case b.GradientTo.A != 0:
			fillGradient(p, r, fill, b.GradientTo, b.GradientDir)
		case b.Radius > 0:
			fillRoundRect(p, r.X, r.Y, r.W, r.H, b.Radius, fill)
		default:
			p.FillRect(r, fill)
		}
	}
	if b.Bevel != BevelNone {
		base := b.Fill
		if base == (painter.RGBA{}) {
			base = theme.Background
		}
		// Highlight/shadow edges lifted toward white / sunk toward black from the
		// fill, so the bevel reads on any face.
		hi := blendRGBA(RGBA{R: 255, G: 255, B: 255, A: 255}, base, 0.6)
		lo := blendRGBA(RGBA{A: 255}, base, 0.55)
		if b.Bevel == BevelSunken {
			drawSunkenBevel(p, r, hi, lo)
		} else {
			drawRaisedBevel(p, r, hi, lo)
		}
	}
	if b.Stroke.A != 0 {
		w := b.StrokeWidth
		if w < 1 {
			w = 1
		}
		p.StrokeRoundRect(painter.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}, b.Radius, b.Stroke, w)
	}
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
