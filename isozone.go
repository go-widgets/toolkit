// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"strconv"

	"github.com/go-gfx/gfx/geometry"
	"github.com/go-gfx/gfx/iso"
	"github.com/go-gfx/gfx/raster"
	"github.com/go-gfx/gfx/vector"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// isoZoneDefaultAlpha is the fill opacity a zone left uncoloured (Color.A==0)
// takes — a light translucent tint of the theme accent, so the grid and any
// node standing on the zone still read through it.
const isoZoneDefaultAlpha = 64

// isoZoneBorder / isoZoneLabelPad / isoHandleSize / isoHandleHit size the zone
// chrome in logical pixels (each routed through the toolkit HiDPI/density scale
// via scaled): the border stroke width, the label inset from the top corner,
// the resize-handle square and how near a click must fall to a corner to grab
// its handle.
const (
	isoZoneBorder   = 2
	isoZoneLabelPad = 3
	isoHandleSize   = 6
	isoHandleHit    = 6
)

// --- selection ----------------------------------------------------------

// SelectedZone returns the selected zone's ID, or "" when no zone is selected.
func (d *IsoDiagram) SelectedZone() string { return d.selZone.Get() }

// SelectedZoneObservable exposes the zone-selection property so a host can bind
// it into an MVVM view model (e.g. a zone-properties inspector) that stays in
// sync with clicks in the diagram, rather than polling [IsoDiagram.SelectedZone]
// every frame.
func (d *IsoDiagram) SelectedZoneObservable() *mvvm.Observable[string] { return d.selZone }

// SelectZone selects the zone with the given id (or clears the zone selection
// when id is ""). Selecting a zone clears any node, connector or text selection
// so only one entity ever highlights at once.
func (d *IsoDiagram) SelectZone(id string) {
	if id == "" {
		d.selRemoveKind(IsoEntityZone)
		return
	}
	d.selReplace(IsoEntityRef{Kind: IsoEntityZone, ID: id})
}

// --- geometry -----------------------------------------------------------

// gridRectCorners returns the four projected corners (buffer-local) of the tile
// rectangle [x, x+w] x [y, y+h] on the ground plane, in the order min, +X, +X+Y,
// +Y — the polygon a zone fills and its outline strokes.
func (d *IsoDiagram) gridRectCorners(x, y, w, h int) []geometry.Point {
	x0, y0 := float64(x), float64(y)
	x1, y1 := x0+float64(w), y0+float64(h)
	pj := d.proj.Project
	return []geometry.Point{
		pj(iso.V(x0, y0, 0)),
		pj(iso.V(x1, y0, 0)),
		pj(iso.V(x1, y1, 0)),
		pj(iso.V(x0, y1, 0)),
	}
}

// zoneCorners returns the four projected ground corners of a zone (buffer-local).
func (d *IsoDiagram) zoneCorners(z IsoZone) []geometry.Point {
	return d.gridRectCorners(z.X, z.Y, z.W, z.H)
}

// zoneGridCorner returns the integer grid-vertex coordinates of corner i
// (0 = min, 1 = +X, 2 = +X+Y, 3 = +Y) of a zone.
func zoneGridCorner(z IsoZone, i int) (int, int) {
	switch i {
	case 0:
		return z.X, z.Y
	case 1:
		return z.X + z.W, z.Y
	case 2:
		return z.X + z.W, z.Y + z.H
	default:
		return z.X, z.Y + z.H
	}
}

// zoneFill is a zone's fill colour (with its alpha as the opacity): the zone's
// own Color, or a translucent theme accent when it left its colour unset
// (A==0).
func (d *IsoDiagram) zoneFill(z IsoZone, theme *Theme) RGBA {
	if z.Color.A == 0 {
		a := theme.Accent
		return RGBA{R: a.R, G: a.G, B: a.B, A: isoZoneDefaultAlpha}
	}
	return z.Color
}

// zoneBorderRGBA is the opaque form of a fill colour — the zone's border and
// label ink, so both read solidly over the translucent body.
func zoneBorderRGBA(fill RGBA) RGBA { return RGBA{R: fill.R, G: fill.G, B: fill.B, A: 255} }

// zoneBorderWidth is the border stroke width in device pixels (the logical
// literal routed through the HiDPI/density scale).
func (d *IsoDiagram) zoneBorderWidth() float64 { return float64(scaled(isoZoneBorder)) }

// handleHit is the grab radius (device pixels) around a corner handle.
func (d *IsoDiagram) handleHit() int { return scaled(isoHandleHit) }

// handleSize is the side (device pixels) of a corner handle square.
func (d *IsoDiagram) handleSize() int { return scaled(isoHandleSize) }

// --- rendering (floor pass) ---------------------------------------------

// drawZones paints every zone onto img — BEFORE the depth-sorted node/connector
// scene — as a semi-transparent filled quad with an opaque border. Rasterising
// straight into the scene buffer (through go-gfx's vector layer, the same one
// the iso solids use) keeps zones strictly behind every node and connector, so
// the scene's own depth-sort never has to account for them.
func (d *IsoDiagram) drawZones(img *raster.Image, theme *Theme) {
	d.drawZonesWhere(img, theme, func(IsoZone) bool { return true })
}

// drawZonesWhere paints every zone the predicate keep accepts (see
// [IsoDiagram.drawZones]); the layered render path passes a per-layer-order
// predicate so each layer's zones composite in their own pass.
func (d *IsoDiagram) drawZonesWhere(img *raster.Image, theme *Theme, keep func(IsoZone) bool) {
	rz := &vector.Rasterizer{}
	for _, z := range d.doc.Zones() {
		if !keep(z) {
			continue
		}
		corners := d.zoneCorners(z)
		fill := d.zoneFill(z, theme)
		zoneFillPoly(img, rz, corners, fill)
		zoneStrokePoly(img, rz, corners, zoneBorderRGBA(fill), d.zoneBorderWidth())
	}
}

// zonePath builds a closed vector path around the projected corners.
func zonePath(pts []geometry.Point) *vector.Path {
	path := vector.NewPath()
	path.MoveTo(pts[0].X, pts[0].Y)
	for _, p := range pts[1:] {
		path.LineTo(p.X, p.Y)
	}
	return path.Close()
}

// zoneFillPoly rasterises the convex polygon pts and composites it in flat colour
// col (with col's own alpha) onto img.
func zoneFillPoly(img *raster.Image, rz *vector.Rasterizer, pts []geometry.Point, col RGBA) {
	cov, ox, oy, w, h, ok := rz.Fill(zonePath(pts), vector.NonZero, img.W, img.H)
	if !ok {
		return
	}
	vector.Composite(img, cov, ox, oy, w, h, vector.SolidPaint{Color: stdColor(col)})
}

// zoneStrokePoly rasterises the outline of the closed polygon pts at the given
// width and composites it in colour col onto img.
func zoneStrokePoly(img *raster.Image, rz *vector.Rasterizer, pts []geometry.Point, col RGBA, width float64) {
	cov, ox, oy, w, h, ok := rz.Stroke(zonePath(pts), width, img.W, img.H)
	if !ok {
		return
	}
	vector.Composite(img, cov, ox, oy, w, h, vector.SolidPaint{Color: stdColor(col)})
}

// --- rendering (painter-space chrome) -----------------------------------

// drawZonePreview strokes the in-flight zone rectangle (from the press cell to
// the current cell) in the accent colour, over the blitted scene.
func (d *IsoDiagram) drawZonePreview(p painter.Painter, b Rect, theme *Theme) {
	gx0, gy0 := d.cellAtLocal(d.pressX, d.pressY)
	gx1, gy1 := d.cellAtLocal(d.curX, d.curY)
	x, y := min(gx0, gx1), min(gy0, gy1)
	w, h := iabs(gx1-gx0)+1, iabs(gy1-gy0)+1
	corners := d.gridRectCorners(x, y, w, h)
	strokeScreenPoly(p, b, corners, theme.Accent)
}

// drawZoneOverlays paints, over the blitted scene, each zone's corner label and
// — for the selected zone — its outline and corner resize handles, in painter
// space so they stay grabbable above any node drawn on the zone.
func (d *IsoDiagram) drawZoneOverlays(p painter.Painter, b Rect, theme *Theme) {
	for _, z := range d.doc.Zones() {
		if z.Label == "" || !d.layerVisible(z.Layer) {
			continue
		}
		c := d.proj.Project(iso.V(float64(z.X), float64(z.Y), 0))
		lx := b.X + iround(c.X) + scaled(isoZoneLabelPad)
		ly := b.Y + iround(c.Y)
		d.drawText(p, lx, ly, z.Label, zoneBorderRGBA(d.zoneFill(z, theme)))
	}
	// Outline every selected zone.
	for _, r := range d.selSet.Slice() {
		if r.Kind != IsoEntityZone {
			continue
		}
		z, ok := d.doc.Zone(r.ID)
		if !ok || !d.layerVisible(z.Layer) {
			continue
		}
		strokeScreenPoly(p, b, d.zoneCorners(z), theme.Accent)
	}
	// Resize handles are drawn only when a single zone is the sole selection —
	// resizing is a one-zone gesture.
	if sel, ok := d.soleSelectedZone(); ok {
		for _, c := range d.zoneCorners(sel) {
			d.drawHandle(p, b.X+iround(c.X), b.Y+iround(c.Y), theme.Accent)
		}
	}
}

// soleSelectedZone returns the selected zone when it is the ONLY selected
// entity (and its layer is visible), and whether it is.
func (d *IsoDiagram) soleSelectedZone() (IsoZone, bool) {
	if d.selSet.Len() != 1 {
		return IsoZone{}, false
	}
	r := d.selSet.At(0)
	if r.Kind != IsoEntityZone {
		return IsoZone{}, false
	}
	z, ok := d.doc.Zone(r.ID)
	if !ok || !d.layerVisible(z.Layer) {
		return IsoZone{}, false
	}
	return z, true
}

// strokeScreenPoly strokes a closed buffer-local polygon (offset into the widget
// by b) as connected accent-coloured line segments in painter space.
func strokeScreenPoly(p painter.Painter, b Rect, pts []geometry.Point, ink RGBA) {
	for i := range pts {
		a, c := pts[i], pts[(i+1)%len(pts)]
		drawLine(p, b.X+iround(a.X), b.Y+iround(a.Y), b.X+iround(c.X), b.Y+iround(c.Y), ink)
	}
}

// drawHandle fills a small square handle centred at painter-space (cx, cy).
func (d *IsoDiagram) drawHandle(p painter.Painter, cx, cy int, c RGBA) {
	s := d.handleSize()
	fillRect(p, cx-s/2, cy-s/2, s, s, c)
}

// --- hit testing --------------------------------------------------------

// zoneAtLocal returns the id of the topmost zone whose projected ground quad
// contains widget-local (x, y), and whether one was hit. Later-inserted zones
// (drawn on top) win, so the scan runs in reverse insertion order.
func (d *IsoDiagram) zoneAtLocal(x, y int) (string, bool) {
	fx, fy := float64(x), float64(y)
	zones := d.doc.Zones()
	for i := len(zones) - 1; i >= 0; i-- {
		if !d.pickable(zones[i].Layer) {
			continue
		}
		if pointInPoly(fx, fy, d.zoneCorners(zones[i])) {
			return zones[i].ID, true
		}
	}
	return "", false
}

// zoneHandleAt returns which corner (0..3) of the currently selected zone lies
// within the handle grab radius of widget-local (x, y), and whether one does.
// With no zone selected it is always a miss.
func (d *IsoDiagram) zoneHandleAt(x, y int) (int, bool) {
	z, ok := d.doc.Zone(d.selZone.Get())
	if !ok {
		return 0, false
	}
	r := float64(d.handleHit())
	for i, c := range d.zoneCorners(z) {
		if math.Hypot(c.X-float64(x), c.Y-float64(y)) <= r {
			return i, true
		}
	}
	return 0, false
}

// vertexAtLocal maps widget-local (x, y) to the nearest integer grid VERTEX
// (unproject onto z=0, then round) — the snap target when dragging a corner
// handle.
func (d *IsoDiagram) vertexAtLocal(x, y int) (int, int) {
	w := d.proj.Unproject(geometry.Pt(float64(x), float64(y)), 0)
	return int(math.Round(w.X)), int(math.Round(w.Y))
}

// --- move / resize gestures ---------------------------------------------

// beginZoneMove arms a whole-zone move: it records the zone's base cell and the
// ground cell under the press so the zone follows the pointer by a cell delta.
func (d *IsoDiagram) beginZoneMove(id string, x, y int) {
	z, _ := d.doc.Zone(id)
	d.gesture = isoGestureZoneMove
	d.dragZone = id
	d.grabEntX, d.grabEntY = z.X, z.Y
	d.grabCellX, d.grabCellY = d.cellAtLocal(x, y)
}

// moveZoneDragTo moves the zone being dragged so its base corner tracks the
// pointer by the cell delta from the grab.
func (d *IsoDiagram) moveZoneDragTo(x, y int) {
	z, ok := d.doc.Zone(d.dragZone)
	if !ok {
		return
	}
	gx, gy := d.cellAtLocal(x, y)
	nx := d.grabEntX + (gx - d.grabCellX)
	ny := d.grabEntY + (gy - d.grabCellY)
	if z.X == nx && z.Y == ny {
		return
	}
	z.X, z.Y = nx, ny
	d.doc.PutZone(z)
}

// beginZoneResize arms a corner-drag of the selected zone: the OPPOSITE corner's
// grid vertex is pinned while the dragged corner follows the pointer.
func (d *IsoDiagram) beginZoneResize(corner int) {
	z, _ := d.doc.Zone(d.selZone.Get())
	d.gesture = isoGestureZoneResize
	d.resizeFixX, d.resizeFixY = zoneGridCorner(z, (corner+2)%4)
}

// resizeZoneDragTo recomputes the selected zone's rectangle from the pinned
// opposite corner and the grid vertex under the pointer, keeping the zone at
// least one cell on each side.
func (d *IsoDiagram) resizeZoneDragTo(x, y int) {
	z, ok := d.doc.Zone(d.selZone.Get())
	if !ok {
		return
	}
	vx, vy := d.vertexAtLocal(x, y)
	nx, ny := min(vx, d.resizeFixX), min(vy, d.resizeFixY)
	nw, nh := max(1, iabs(vx-d.resizeFixX)), max(1, iabs(vy-d.resizeFixY))
	if z.X == nx && z.Y == ny && z.W == nw && z.H == nh {
		return
	}
	z.X, z.Y, z.W, z.H = nx, ny, nw, nh
	d.doc.PutZone(z)
}

// --- editing commands ---------------------------------------------------

// nextZoneID returns a zone-unique id.
func (d *IsoDiagram) nextZoneID() string {
	for {
		d.seq++
		id := "z" + strconv.Itoa(d.seq)
		if _, ok := d.doc.Zone(id); ok {
			continue
		}
		return id
	}
}

// commitPlaceZone creates a zone spanning the two ground cells (inclusive), at
// least 1x1, selects it and returns its id — as one undoable command.
func (d *IsoDiagram) commitPlaceZone(gx0, gy0, gx1, gy1 int) string {
	d.beginEdit()
	x, y := min(gx0, gx1), min(gy0, gy1)
	w, h := iabs(gx1-gx0)+1, iabs(gy1-gy0)+1
	id := d.nextZoneID()
	d.doc.PutZone(IsoZone{ID: id, X: x, Y: y, W: w, H: h})
	d.SelectZone(id)
	d.invalidate()
	return id
}

// commitDeleteZone removes a zone as an undoable command, clearing the selection
// if it was selected.
func (d *IsoDiagram) commitDeleteZone(id string) {
	if _, ok := d.doc.Zone(id); !ok {
		return
	}
	d.beginEdit()
	d.doc.RemoveZone(id)
	d.pruneSelection()
	d.invalidate()
}

// editZone applies mutate to the zone with id as one undoable edit, unless the
// zone is missing or mutate reports no change. It returns whether it changed
// anything.
func (d *IsoDiagram) editZone(id string, mutate func(*IsoZone) bool) bool {
	z, ok := d.doc.Zone(id)
	if !ok {
		return false
	}
	if !mutate(&z) {
		return false
	}
	d.beginEdit()
	d.doc.PutZone(z)
	d.invalidate()
	return true
}

// SetZoneColor sets a zone's fill colour (undoable); pass a zero colour (A==0)
// to fall back to the translucent theme accent. Returns false when the zone is
// absent or already has that colour.
func (d *IsoDiagram) SetZoneColor(id string, col RGBA) bool {
	return d.editZone(id, func(z *IsoZone) bool {
		if z.Color == col {
			return false
		}
		z.Color = col
		return true
	})
}

// SetZoneLabel sets a zone's caption (undoable). Returns false when the zone is
// absent or already carries that label.
func (d *IsoDiagram) SetZoneLabel(id, label string) bool {
	return d.editZone(id, func(z *IsoZone) bool {
		if z.Label == label {
			return false
		}
		z.Label = label
		return true
	})
}

// SetZoneRect sets a zone's rectangle (undoable), clamping the size to at least
// 1x1. Returns false when the zone is absent or already occupies that
// rectangle.
func (d *IsoDiagram) SetZoneRect(id string, x, y, w, h int) bool {
	w, h = max(1, w), max(1, h)
	return d.editZone(id, func(z *IsoZone) bool {
		if z.X == x && z.Y == y && z.W == w && z.H == h {
			return false
		}
		z.X, z.Y, z.W, z.H = x, y, w, h
		return true
	})
}

// SetSelectedZoneColor applies [IsoDiagram.SetZoneColor] to the selected zone (a
// no-op returning false when none is selected).
func (d *IsoDiagram) SetSelectedZoneColor(col RGBA) bool {
	return d.SetZoneColor(d.selZone.Get(), col)
}

// SetSelectedZoneLabel applies [IsoDiagram.SetZoneLabel] to the selected zone.
func (d *IsoDiagram) SetSelectedZoneLabel(label string) bool {
	return d.SetZoneLabel(d.selZone.Get(), label)
}
