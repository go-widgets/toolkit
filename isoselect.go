// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-gfx/gfx/geometry"
	"github.com/go-widgets/mvvm"
)

// IsoEntityKind names which family an [IsoEntityRef] points at. It replaces the
// four separate mono-selection channels' implicit "kind" with one explicit tag
// so a single selection set can hold nodes, connectors, zones and texts mixed.
type IsoEntityKind int

const (
	// IsoEntityNode references an [IsoNode].
	IsoEntityNode IsoEntityKind = iota
	// IsoEntityConnector references an [IsoConnector].
	IsoEntityConnector
	// IsoEntityZone references an [IsoZone].
	IsoEntityZone
	// IsoEntityText references an [IsoText] annotation.
	IsoEntityText
)

// IsoEntityRef identifies one selected entity: its family and its id. It is a
// comparable value, so it is a natural element of an
// mvvm.ObservableList and a map key.
type IsoEntityRef struct {
	// Kind is the entity family the id belongs to.
	Kind IsoEntityKind
	// ID is the entity's stable id within its family.
	ID string
}

// --- multi-selection accessors ------------------------------------------

// Selection returns a snapshot of the current multi-selection, in selection
// order (the order entities were added).
func (d *IsoDiagram) Selection() []IsoEntityRef { return d.selSet.Slice() }

// SelectionList exposes the authoritative multi-selection as an observable list
// so a host can bind it into an MVVM view model (e.g. a "3 items selected"
// status or a multi-object inspector) that stays in sync with clicks, the
// marquee and select-all, rather than polling [IsoDiagram.Selection] every
// frame.
func (d *IsoDiagram) SelectionList() *mvvm.ObservableList[IsoEntityRef] { return d.selSet }

// IsSelected reports whether ref is in the current multi-selection.
func (d *IsoDiagram) IsSelected(ref IsoEntityRef) bool { return d.selIndexOf(ref) >= 0 }

// selIndexOf returns the position of ref in the selection set, or -1.
func (d *IsoDiagram) selIndexOf(ref IsoEntityRef) int {
	for i := 0; i < d.selSet.Len(); i++ {
		if d.selSet.At(i) == ref {
			return i
		}
	}
	return -1
}

// --- selection mutation primitives --------------------------------------
//
// Every mutation touches selSet only when it actually changes it (so a
// redundant reselect fires nothing) and then calls refreshMono to reflect the
// new set into the four single-selection channels.

// selEqual reports whether the selection set already equals refs, in order.
func (d *IsoDiagram) selEqual(refs []IsoEntityRef) bool {
	if d.selSet.Len() != len(refs) {
		return false
	}
	for i, r := range refs {
		if d.selSet.At(i) != r {
			return false
		}
	}
	return true
}

// selReplace makes refs the whole selection (deduped is the caller's job; the
// callers pass already-distinct sets). It is a no-op when the set is unchanged.
func (d *IsoDiagram) selReplace(refs ...IsoEntityRef) {
	if d.selEqual(refs) {
		return
	}
	d.selSet.Clear()
	if len(refs) > 0 {
		d.selSet.Append(refs...)
	}
	d.refreshMono()
}

// selClear empties the selection.
func (d *IsoDiagram) selClear() { d.selReplace() }

// selAdd adds ref to the selection unless it is already present.
func (d *IsoDiagram) selAdd(ref IsoEntityRef) {
	if d.selIndexOf(ref) >= 0 {
		return
	}
	d.selSet.Append(ref)
	d.refreshMono()
}

// selToggle adds ref when absent, removes it when present.
func (d *IsoDiagram) selToggle(ref IsoEntityRef) {
	if i := d.selIndexOf(ref); i >= 0 {
		d.selSet.RemoveAt(i)
	} else {
		d.selSet.Append(ref)
	}
	d.refreshMono()
}

// selRemoveKind drops every selected entity of kind k.
func (d *IsoDiagram) selRemoveKind(k IsoEntityKind) {
	changed := false
	for i := d.selSet.Len() - 1; i >= 0; i-- {
		if d.selSet.At(i).Kind == k {
			d.selSet.RemoveAt(i)
			changed = true
		}
	}
	if changed {
		d.refreshMono()
	}
}

// pruneSelection drops every selected reference whose entity no longer exists
// (after an undo/redo or a delete). It is the multi-selection form of the old
// per-channel "clear if the selected id vanished" guard.
func (d *IsoDiagram) pruneSelection() {
	changed := false
	for i := d.selSet.Len() - 1; i >= 0; i-- {
		if !d.refExists(d.selSet.At(i)) {
			d.selSet.RemoveAt(i)
			changed = true
		}
	}
	if changed {
		d.refreshMono()
	}
}

// refExists reports whether ref still points at a live entity.
func (d *IsoDiagram) refExists(ref IsoEntityRef) bool {
	switch ref.Kind {
	case IsoEntityNode:
		_, ok := d.doc.Node(ref.ID)
		return ok
	case IsoEntityConnector:
		_, ok := d.connectorByID(ref.ID)
		return ok
	case IsoEntityZone:
		_, ok := d.doc.Zone(ref.ID)
		return ok
	default:
		_, ok := d.doc.Text(ref.ID)
		return ok
	}
}

// refreshMono reflects the multi-selection into the four single-selection
// channels: each holds the id of the LAST-selected entity of its kind, or ""
// when none of that kind is selected. Setting the connector / zone / text
// observables fires their existing OnSelect* + invalidate subscriptions; the
// node channel fires OnSelect directly, and only when its reflected id changed
// — so the Vague-B/C mono API behaves exactly as it did.
func (d *IsoDiagram) refreshMono() {
	var node, conn, zone, text string
	for _, r := range d.selSet.Slice() {
		switch r.Kind {
		case IsoEntityNode:
			node = r.ID
		case IsoEntityConnector:
			conn = r.ID
		case IsoEntityZone:
			zone = r.ID
		default:
			text = r.ID
		}
	}
	if d.selected != node {
		d.selected = node
		if d.OnSelect != nil {
			d.OnSelect(node)
		}
	}
	d.selConn.Set(conn)
	d.selZone.Set(zone)
	d.selText.Set(text)
}

// --- select-all ---------------------------------------------------------

// SelectAll selects every pickable entity (every node, connector, zone and text
// whose layer is visible and unlocked). Entities on a hidden or locked layer
// are left out, exactly as they are unclickable.
func (d *IsoDiagram) SelectAll() {
	var refs []IsoEntityRef
	for _, n := range d.doc.Nodes() {
		if d.pickable(n.Layer) {
			refs = append(refs, IsoEntityRef{Kind: IsoEntityNode, ID: n.ID})
		}
	}
	for _, c := range d.doc.Connectors() {
		if d.pickable(c.Layer) {
			refs = append(refs, IsoEntityRef{Kind: IsoEntityConnector, ID: c.ID})
		}
	}
	for _, z := range d.doc.Zones() {
		if d.pickable(z.Layer) {
			refs = append(refs, IsoEntityRef{Kind: IsoEntityZone, ID: z.ID})
		}
	}
	for _, t := range d.doc.Texts() {
		if d.pickable(t.Layer) {
			refs = append(refs, IsoEntityRef{Kind: IsoEntityText, ID: t.ID})
		}
	}
	d.selReplace(refs...)
}

// --- group delete / recolour --------------------------------------------

// DeleteSelection removes every selected entity as ONE undoable command and
// clears the selection. With nothing selected it is a no-op (no undo entry).
func (d *IsoDiagram) DeleteSelection() {
	refs := d.selSet.Slice()
	if len(refs) == 0 {
		return
	}
	d.beginEdit()
	for _, r := range refs {
		switch r.Kind {
		case IsoEntityNode:
			d.doc.RemoveNode(r.ID)
		case IsoEntityConnector:
			d.doc.RemoveConnector(r.ID)
		case IsoEntityZone:
			d.doc.RemoveZone(r.ID)
		default:
			d.doc.RemoveText(r.ID)
		}
	}
	d.selClear()
	d.invalidate()
}

// SetSelectionColor recolours every selected entity — nodes, connectors, zones
// and texts alike — to col as ONE undoable command. A zero colour (A==0) falls
// each entity back to its theme default. It returns false (no undo entry) when
// nothing is selected or no entity's colour actually changed.
func (d *IsoDiagram) SetSelectionColor(col RGBA) bool {
	if d.selSet.Len() == 0 {
		return false
	}
	if !d.selectionColorChanges(col) {
		return false
	}
	d.beginEdit()
	for _, r := range d.selSet.Slice() {
		d.applyColor(r, col)
	}
	d.invalidate()
	return true
}

// selectionColorChanges reports whether recolouring the selection to col would
// change any selected entity's stored colour (so a redundant recolour neither
// snapshots nor repaints). It scans the live document filtered by selection
// membership, so it never touches a stale reference.
func (d *IsoDiagram) selectionColorChanges(col RGBA) bool {
	changed := false
	for _, n := range d.doc.Nodes() {
		if d.IsSelected(IsoEntityRef{Kind: IsoEntityNode, ID: n.ID}) && n.Color != col {
			changed = true
		}
	}
	for _, c := range d.doc.Connectors() {
		if d.IsSelected(IsoEntityRef{Kind: IsoEntityConnector, ID: c.ID}) && c.Color != col {
			changed = true
		}
	}
	for _, z := range d.doc.Zones() {
		if d.IsSelected(IsoEntityRef{Kind: IsoEntityZone, ID: z.ID}) && z.Color != col {
			changed = true
		}
	}
	for _, t := range d.doc.Texts() {
		if d.IsSelected(IsoEntityRef{Kind: IsoEntityText, ID: t.ID}) && t.Color != col {
			changed = true
		}
	}
	return changed
}

// applyColor writes col onto the entity ref points at (a missing entity is a
// no-op). It does not snapshot; the caller wraps a batch in one edit.
func (d *IsoDiagram) applyColor(ref IsoEntityRef, col RGBA) {
	switch ref.Kind {
	case IsoEntityNode:
		if n, ok := d.doc.Node(ref.ID); ok {
			n.Color = col
			d.doc.PutNode(n)
		}
	case IsoEntityConnector:
		if c, ok := d.connectorByID(ref.ID); ok {
			c.Color = col
			d.doc.PutConnector(c)
		}
	case IsoEntityZone:
		if z, ok := d.doc.Zone(ref.ID); ok {
			z.Color = col
			d.doc.PutZone(z)
		}
	default:
		if t, ok := d.doc.Text(ref.ID); ok {
			t.Color = col
			d.doc.PutText(t)
		}
	}
}

// --- group move ---------------------------------------------------------

// isoGrabItem records one moved entity's base cell at grab time for a group
// move.
type isoGrabItem struct {
	ref  IsoEntityRef
	x, y int
}

// beginGroupMove arms a whole-selection move: it records the ground cell under
// the press and every selected node's, zone's and text's base cell, so the
// group follows the pointer by a cell delta. Connectors move implicitly with
// their endpoints, so they are not grabbed.
func (d *IsoDiagram) beginGroupMove(x, y int) {
	d.gesture = isoGestureGroupMove
	d.grabCellX, d.grabCellY = d.cellAtLocal(x, y)
	d.groupGrab = d.groupGrab[:0]
	// Selected refs are always live (the selection is pruned on every removal),
	// so each lookup succeeds; connectors carry no base cell and are skipped —
	// they move implicitly with their endpoints.
	for _, r := range d.selSet.Slice() {
		switch r.Kind {
		case IsoEntityNode:
			n, _ := d.doc.Node(r.ID)
			d.groupGrab = append(d.groupGrab, isoGrabItem{r, n.X, n.Y})
		case IsoEntityZone:
			z, _ := d.doc.Zone(r.ID)
			d.groupGrab = append(d.groupGrab, isoGrabItem{r, z.X, z.Y})
		case IsoEntityText:
			t, _ := d.doc.Text(r.ID)
			d.groupGrab = append(d.groupGrab, isoGrabItem{r, t.X, t.Y})
		}
	}
}

// moveGroupTo moves every grabbed entity so its base cell tracks the pointer by
// the cell delta from the grab. It writes each entity only when its cell
// actually changes, so a hover that stays in the same cell mutates nothing.
func (d *IsoDiagram) moveGroupTo(x, y int) {
	gx, gy := d.cellAtLocal(x, y)
	dx, dy := gx-d.grabCellX, gy-d.grabCellY
	for _, it := range d.groupGrab {
		nx, ny := it.x+dx, it.y+dy
		switch it.ref.Kind {
		case IsoEntityNode:
			if n, _ := d.doc.Node(it.ref.ID); n.X != nx || n.Y != ny {
				n.X, n.Y = nx, ny
				d.doc.PutNode(n)
			}
		case IsoEntityZone:
			if z, _ := d.doc.Zone(it.ref.ID); z.X != nx || z.Y != ny {
				z.X, z.Y = nx, ny
				d.doc.PutZone(z)
			}
		case IsoEntityText:
			if t, _ := d.doc.Text(it.ref.ID); t.X != nx || t.Y != ny {
				t.X, t.Y = nx, ny
				d.doc.PutText(t)
			}
		}
	}
}

// --- marquee ------------------------------------------------------------

// isoBox is an axis-aligned bounding box in widget-local pixels.
type isoBox struct {
	minX, minY, maxX, maxY float64
}

// boxOfPoints is the bounding box of a non-empty point set.
func boxOfPoints(pts []geometry.Point) isoBox {
	b := isoBox{pts[0].X, pts[0].Y, pts[0].X, pts[0].Y}
	for _, p := range pts[1:] {
		b.minX, b.maxX = mathMin(b.minX, p.X), mathMax(b.maxX, p.X)
		b.minY, b.maxY = mathMin(b.minY, p.Y), mathMax(b.maxY, p.Y)
	}
	return b
}

// intersects reports whether two boxes overlap (touching edges count).
func (a isoBox) intersects(b isoBox) bool {
	return a.minX <= b.maxX && b.minX <= a.maxX && a.minY <= b.maxY && b.minY <= a.maxY
}

// mathMin / mathMax are float min/max (kept local so the file needs no math
// import for two one-liners).
func mathMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func mathMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// nodeBox is a node's projected silhouette bounding box (buffer-local): the
// union of its pick polygons.
func (d *IsoDiagram) nodeBox(n IsoNode) isoBox {
	polys := d.pickPolys(n)
	b := boxOfPoints(polys[0])
	for _, poly := range polys[1:] {
		bb := boxOfPoints(poly)
		b.minX, b.minY = mathMin(b.minX, bb.minX), mathMin(b.minY, bb.minY)
		b.maxX, b.maxY = mathMax(b.maxX, bb.maxX), mathMax(b.maxY, bb.maxY)
	}
	return b
}

// marqueeRefs returns every pickable entity whose projected bounding box
// intersects the rectangle spanning (x0,y0)-(x1,y1), in node/connector/zone/text
// order.
func (d *IsoDiagram) marqueeRefs(x0, y0, x1, y1 int) []IsoEntityRef {
	m := isoBox{
		float64(min(x0, x1)), float64(min(y0, y1)),
		float64(max(x0, x1)), float64(max(y0, y1)),
	}
	var out []IsoEntityRef
	for _, n := range d.doc.Nodes() {
		if d.pickable(n.Layer) && d.nodeBox(n).intersects(m) {
			out = append(out, IsoEntityRef{Kind: IsoEntityNode, ID: n.ID})
		}
	}
	for _, c := range d.doc.Connectors() {
		if !d.pickable(c.Layer) {
			continue
		}
		pts, ok := d.connectorPath(c)
		if !ok {
			continue
		}
		if boxOfPoints(d.projPath(pts)).intersects(m) {
			out = append(out, IsoEntityRef{Kind: IsoEntityConnector, ID: c.ID})
		}
	}
	for _, z := range d.doc.Zones() {
		if d.pickable(z.Layer) && boxOfPoints(d.zoneCorners(z)).intersects(m) {
			out = append(out, IsoEntityRef{Kind: IsoEntityZone, ID: z.ID})
		}
	}
	for _, t := range d.doc.Texts() {
		if !d.pickable(t.Layer) {
			continue
		}
		box := d.textBox(t)
		tb := isoBox{float64(box.X), float64(box.Y), float64(box.X + box.W), float64(box.Y + box.H)}
		if tb.intersects(m) {
			out = append(out, IsoEntityRef{Kind: IsoEntityText, ID: t.ID})
		}
	}
	return out
}
