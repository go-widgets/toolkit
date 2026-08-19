// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-gfx/gfx/geometry"
	"github.com/go-gfx/gfx/iso"
	"github.com/go-widgets/mvvm"
)

// The view rotation turns the whole isometric plane about its vertical (Z) axis
// in 90° steps, giving the four isometric orientations. It is a LOCAL view state
// (like pan and zoom): it lives on the widget, never in the [IsoDocument] or its
// CRDT, so two views of one model — or two collaborating replicas — may hold
// different rotations without diverging the shared document.
//
// It is implemented as one rigid rotation of world coordinates about the grid's
// geometric centre (Cols/2, Rows/2), applied at the projection boundary: every
// world point the widget draws or hit-tests passes through [IsoDiagram.rotFwd]
// (model → view) on the way to the screen and through [IsoDiagram.rotInv]
// (view → model) on the way back. Because a 90° rotation maps an axis-aligned
// grid cell to another axis-aligned cell, the grid stays pixel-crisp and every
// node still lands exactly on a tile. At quarter 0 rotFwd is the identity, so the
// rendered buffer is byte-identical to the pre-rotation widget.

// ViewRotation returns the current view rotation in quarter-turns (0..3). 0 is
// the unrotated view; 1, 2 and 3 are successive 90° clockwise turns of the
// plane.
func (d *IsoDiagram) ViewRotation() int { return d.viewQuarter() }

// ViewRotationObservable exposes the view-rotation property so a host can bind it
// into an MVVM view model (e.g. a "rotate view" toolbar) that stays in sync with
// the diagram, rather than polling [IsoDiagram.ViewRotation] every frame. Setting
// it is equivalent to [IsoDiagram.SetViewRotation]; the stored value is always
// normalised to 0..3.
func (d *IsoDiagram) ViewRotationObservable() *mvvm.Observable[int] { return d.viewRot }

// SetViewRotation turns the view to quarter-turn q, taken modulo 4 (so any
// integer, negative included, names one of the four orientations). It re-orients
// the whole rendered plane — grid, nodes, connectors, zones and texts — and
// recomputes the depth-sort for the new orientation; it records no undo entry and
// changes nothing in the document. A redundant set (same normalised quarter)
// neither notifies nor repaints.
func (d *IsoDiagram) SetViewRotation(q int) {
	n := normQuarter(q)
	if n == d.viewQuarter() {
		return
	}
	d.viewRot.Set(n)
}

// RotateCW turns the view one quarter-turn clockwise (the next of the four
// orientations).
func (d *IsoDiagram) RotateCW() { d.SetViewRotation(d.viewQuarter() + 1) }

// RotateCCW turns the view one quarter-turn counter-clockwise (the previous of
// the four orientations).
func (d *IsoDiagram) RotateCCW() { d.SetViewRotation(d.viewQuarter() + 3) }

// viewQuarter is the current rotation normalised to 0..3, the single reader every
// rotation-aware helper goes through, so an out-of-range value a host may have
// pushed onto the observable is still interpreted as one of the four
// orientations.
func (d *IsoDiagram) viewQuarter() int { return normQuarter(d.viewRot.Get()) }

// normQuarter reduces any integer to the canonical quarter-turn 0..3.
func normQuarter(q int) int { return ((q % 4) + 4) % 4 }

// gridCenter is the world XY point the view rotates about: the grid's geometric
// centre. It is a fixed point of every quarter-turn, so the grid never drifts off
// screen as the view turns.
func (d *IsoDiagram) gridCenter() (float64, float64) {
	return float64(d.Cols) / 2, float64(d.Rows) / 2
}

// rotXY rotates world point (x, y) by q quarter-turns (0..3, counter-clockwise in
// world space) about the grid centre. Quarter 0 returns the coordinates
// unchanged, so a rotation-aware caller is exact and allocation-free in the
// common unrotated case.
func (d *IsoDiagram) rotXY(x, y float64, q int) (float64, float64) {
	cx, cy := d.gridCenter()
	dx, dy := x-cx, y-cy
	switch q {
	case 1:
		return cx - dy, cy + dx
	case 2:
		return cx - dx, cy - dy
	case 3:
		return cx + dy, cy - dx
	default:
		return x, y
	}
}

// rotFwd maps a world point from MODEL space to VIEW space (the frame the raw
// projection sees) by the current view rotation. Z is untouched — the rotation is
// about the vertical axis.
func (d *IsoDiagram) rotFwd(v iso.Vec3) iso.Vec3 {
	x, y := d.rotXY(v.X, v.Y, d.viewQuarter())
	return iso.V(x, y, v.Z)
}

// rotInv maps a world point from VIEW space back to MODEL space — the inverse of
// [IsoDiagram.rotFwd] — so a screen point unprojected into view space recovers
// its model tile. It is the transform hit-testing, placement, drag and marquee
// apply to stay exact under any orientation.
func (d *IsoDiagram) rotInv(v iso.Vec3) iso.Vec3 {
	x, y := d.rotXY(v.X, v.Y, normQuarter(4-d.viewQuarter()))
	return iso.V(x, y, v.Z)
}

// project maps a MODEL-space world point to its screen pixel through the current
// view rotation, then the isometric projection. It is the rotation-aware
// counterpart of proj.Project every overlay and hit-test polygon goes through, so
// one rotation choke point serves the whole widget.
func (d *IsoDiagram) project(v iso.Vec3) geometry.Point { return d.proj.Project(d.rotFwd(v)) }

// depth is the painter's-algorithm sort key of a MODEL-space point in the rotated
// view, so a manual depth sort (node hit-testing) orders nodes for the current
// orientation exactly as the [iso.Scene] does.
func (d *IsoDiagram) depth(v iso.Vec3) float64 { return d.proj.Depth(d.rotFwd(v)) }

// rotBox rotates an axis-aligned solid's footprint into view space: a 90° turn
// maps the box [pos, pos+dim] (in X and Y) to another axis-aligned box, swapping
// its width and height for the odd quarters, so a shaded solid re-lands exactly
// on the rotated tile it occupies. Z (pos.Z and dim.D) is untouched. At quarter 0
// it returns an unit-footprint box's corner exactly (the half-cell centre offset
// cancels), so [IsoDiagram.nodeViewCorner] is exact in the unrotated view.
func (d *IsoDiagram) rotBox(pos iso.Vec3, dim iso.Dimension) (iso.Vec3, iso.Dimension) {
	q := d.viewQuarter()
	// Rotate the footprint centre; the box stays centred on it.
	ccx, ccy := d.rotXY(pos.X+dim.W/2, pos.Y+dim.H/2, q)
	w, h := dim.W, dim.H
	if q == 1 || q == 3 {
		w, h = h, w
	}
	return iso.V(ccx-w/2, ccy-h/2, pos.Z), iso.Dimension{W: w, H: h, D: dim.D}
}

// rotShape returns shape sh with its world coordinates rotated into view space,
// so the depth-sorted [iso.Scene] — which projects through the raw, un-rotated
// projection — renders it at the current orientation. The one wrinkle it handles
// beyond a plain solid is a node icon's arbitrary primitives, which must each
// re-land on the rotated tile with their own footprint turned.
//
// The type switch is exhaustive over iso's Shape set: [iso.Shape.render] is
// unexported, so the only types satisfying the interface are the six the iso
// package defines, and every one but [iso.Line] is cased explicitly — the default
// therefore handles exactly Line. At quarter 0 the shape is returned untouched,
// so the scene is byte-identical to the unrotated widget.
func (d *IsoDiagram) rotShape(sh iso.Shape) iso.Shape {
	if d.viewQuarter() == 0 {
		return sh
	}
	switch s := sh.(type) {
	case iso.Cube:
		// A cube's footprint is square, so rotBox leaves its size; only Pos moves.
		s.Pos, _ = d.rotBox(s.Pos, iso.Dimension{W: s.Size, H: s.Size, D: s.Size})
		return s
	case iso.Brick:
		s.Pos, s.Dim = d.rotBox(s.Pos, s.Dim)
		return s
	case iso.Pyramid:
		s.Pos, s.Dim = d.rotBox(s.Pos, s.Dim)
		return s
	case iso.Slope:
		s.Pos, s.Dim = d.rotBox(s.Pos, s.Dim)
		s.Dir = rotSlopeDir(s.Dir, d.viewQuarter())
		return s
	case iso.Side:
		return d.rotSide(s)
	default: // iso.Line — the only remaining member of iso's closed Shape set.
		l := sh.(iso.Line)
		l.From, l.To = d.rotFwd(l.From), d.rotFwd(l.To)
		return l
	}
}

// rotSlopeDir turns a slope's raised-edge direction by q quarter-turns, so a
// wedge icon ramps toward the same neighbouring tile after the view rotates. The
// order E → N → W → S → E is one counter-clockwise 90° step, matching rotXY.
func rotSlopeDir(dir iso.SlopeDir, q int) iso.SlopeDir {
	// Ordered so that advancing one place is a single 90° counter-clockwise turn:
	// +X (E) rotates to -Y (N), -Y (N) to -X (W), -X (W) to +Y (S), +Y (S) to +X.
	order := []iso.SlopeDir{iso.SlopeE, iso.SlopeN, iso.SlopeW, iso.SlopeS}
	idx := map[iso.SlopeDir]int{iso.SlopeE: 0, iso.SlopeN: 1, iso.SlopeW: 2, iso.SlopeS: 3}
	return order[(idx[dir]+q)%4]
}

// rotSide rotates a flat wall into view space: its base segment (from Pos along
// its plane's ground axis by W) turns rigidly, so the wall re-lands axis-aligned
// with its plane swapped between the X and Y facing on each odd quarter and its
// anchor at the rotated segment's minimum corner.
func (d *IsoDiagram) rotSide(s iso.Side) iso.Side {
	var sx, sy float64
	if s.Plane == iso.SideYZ {
		sy = s.W // spans +Y
	} else {
		sx = s.W // spans +X
	}
	x0, y0 := d.rotXY(s.Pos.X, s.Pos.Y, d.viewQuarter())
	x1, y1 := d.rotXY(s.Pos.X+sx, s.Pos.Y+sy, d.viewQuarter())
	plane := iso.SideYZ
	if absf(x1-x0) >= absf(y1-y0) {
		plane = iso.SideXZ // rotated span lies along X
	}
	return iso.Side{
		Pos:     iso.V(minf(x0, x1), minf(y0, y1), s.Pos.Z),
		W:       s.W,
		H:       s.H,
		Plane:   plane,
		Color:   s.Color,
		Shading: s.Shading,
	}
}

// absf is the absolute value of a float64.
func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// minf is the smaller of two float64s.
func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
