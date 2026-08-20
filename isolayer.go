// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"sort"
	"strconv"

	"github.com/go-gfx/gfx/iso"
	"github.com/go-gfx/gfx/raster"
)

// --- layer property resolution ------------------------------------------
//
// Every resolver falls the empty (or unknown) layer id back to the implicit
// default — order 0, visible, unlocked — so a document that never creates a
// layer behaves exactly as it did before layers existed.

// layerOrder is the back-to-front rank of the layer with id (0 for the default
// / unknown layer).
func (d *IsoDiagram) layerOrder(id string) int {
	if l, ok := d.doc.Layer(id); ok {
		return l.Order
	}
	return 0
}

// layerVisible reports whether the layer with id draws (true for the default /
// unknown layer).
func (d *IsoDiagram) layerVisible(id string) bool {
	if l, ok := d.doc.Layer(id); ok {
		return l.Visible
	}
	return true
}

// layerLocked reports whether the layer with id is frozen (false for the
// default / unknown layer).
func (d *IsoDiagram) layerLocked(id string) bool {
	if l, ok := d.doc.Layer(id); ok {
		return l.Locked
	}
	return false
}

// pickable reports whether entities on the layer with id can be selected or
// edited: its layer must be visible and unlocked.
func (d *IsoDiagram) pickable(id string) bool {
	return d.layerVisible(id) && !d.layerLocked(id)
}

// --- layered rendering pipeline -----------------------------------------

// contentOrders is the sorted set of distinct layer orders among the entities
// that render into the blit pipeline (nodes, connectors, zones). An empty
// document reports the single default order 0, so the grid always draws.
func (d *IsoDiagram) contentOrders() []int {
	seen := map[int]bool{}
	for _, n := range d.doc.Nodes() {
		seen[d.layerOrder(n.Layer)] = true
	}
	for _, c := range d.doc.Connectors() {
		seen[d.layerOrder(c.Layer)] = true
	}
	for _, z := range d.doc.Zones() {
		seen[d.layerOrder(z.Layer)] = true
	}
	if len(seen) == 0 {
		return []int{0}
	}
	out := make([]int, 0, len(seen))
	for o := range seen {
		out = append(out, o)
	}
	sort.Ints(out)
	return out
}

// blitHidden reports whether any blit-pipeline entity (node, connector or zone)
// sits on a hidden layer. When none does and every such entity shares one order,
// the diagram takes the single-pass legacy render, which is byte-for-byte the
// pre-layers output.
func (d *IsoDiagram) blitHidden() bool {
	for _, n := range d.doc.Nodes() {
		if !d.layerVisible(n.Layer) {
			return true
		}
	}
	for _, c := range d.doc.Connectors() {
		if !d.layerVisible(c.Layer) {
			return true
		}
	}
	for _, z := range d.doc.Zones() {
		if !d.layerVisible(z.Layer) {
			return true
		}
	}
	return false
}

// drawContent composites the blit pipeline (zone floor, ground-grid floor,
// depth-sorted node / connector scene, sprite overlay) into img, honouring
// layers.
//
// The common case — a single layer order and nothing hidden, which is EVERY
// pre-layers document — renders in one pass through the unchanged legacy
// helpers. When entities span several layer orders (or some are hidden) each
// order is composited back-to-front in its own pass, so a higher-order layer
// draws over a lower one whatever the isometric depth; the grid floor draws
// once, in the backmost pass, under every solid.
func (d *IsoDiagram) drawContent(img *raster.Image, theme *Theme) {
	orders := d.contentOrders()
	if len(orders) == 1 && !d.blitHidden() {
		d.drawZones(img, theme)
		d.drawGrid(img, theme)
		d.scene(theme).Render(img)
		d.drawSprites(img, theme)
		return
	}
	for i, ord := range orders {
		d.drawZonesWhere(img, theme, func(z IsoZone) bool {
			return d.layerVisible(z.Layer) && d.layerOrder(z.Layer) == ord
		})
		if i == 0 {
			d.drawGrid(img, theme)
		}
		sc := iso.NewScene(d.proj)
		for _, n := range d.doc.Nodes() {
			if d.layerVisible(n.Layer) && d.layerOrder(n.Layer) == ord {
				d.addNodeShapes(sc, theme, n)
			}
		}
		for _, c := range d.doc.Connectors() {
			if d.layerVisible(c.Layer) && d.layerOrder(c.Layer) == ord {
				d.addConnectorShapes(sc, theme, c)
			}
		}
		sc.Render(img)
		d.drawSpritesWhere(img, theme, func(n IsoNode) bool {
			return d.layerVisible(n.Layer) && d.layerOrder(n.Layer) == ord
		})
	}
}

// --- layer editing commands ---------------------------------------------

// Layers returns a snapshot of the document's layer set, in document order.
func (d *IsoDiagram) Layers() []IsoLayer { return d.doc.Layers() }

// nextLayerID returns a document-unique layer id.
func (d *IsoDiagram) nextLayerID() string {
	for {
		d.seq++
		id := "L" + strconv.Itoa(d.seq)
		if _, ok := d.doc.Layer(id); ok {
			continue
		}
		return id
	}
}

// topLayerOrder is the highest Order among existing layers, or 0 when there are
// none — the baseline a freshly added layer stacks one above.
func (d *IsoDiagram) topLayerOrder() int {
	top := 0
	for _, l := range d.doc.Layers() {
		if l.Order > top {
			top = l.Order
		}
	}
	return top
}

// AddLayer creates a new visible, unlocked layer named name, stacked one order
// above the current top layer, and returns its id — as one undoable command.
func (d *IsoDiagram) AddLayer(name string) string {
	d.beginEdit()
	id := d.nextLayerID()
	d.doc.PutLayer(IsoLayer{ID: id, Name: name, Visible: true, Order: d.topLayerOrder() + 1})
	d.invalidate()
	return id
}

// DeleteLayer removes the layer with id as one undoable command, first
// reassigning every entity on it to the default layer so nothing is orphaned.
// It returns false for the empty (default) id or an unknown layer.
func (d *IsoDiagram) DeleteLayer(id string) bool {
	if id == "" {
		return false
	}
	if _, ok := d.doc.Layer(id); !ok {
		return false
	}
	d.beginEdit()
	for _, n := range d.doc.Nodes() {
		if n.Layer == id {
			n.Layer = ""
			d.doc.PutNode(n)
		}
	}
	for _, c := range d.doc.Connectors() {
		if c.Layer == id {
			c.Layer = ""
			d.doc.PutConnector(c)
		}
	}
	for _, z := range d.doc.Zones() {
		if z.Layer == id {
			z.Layer = ""
			d.doc.PutZone(z)
		}
	}
	for _, t := range d.doc.Texts() {
		if t.Layer == id {
			t.Layer = ""
			d.doc.PutText(t)
		}
	}
	d.doc.RemoveLayer(id)
	d.invalidate()
	return true
}

// editLayer applies mutate to the layer with id as one undoable edit, unless it
// is missing or mutate reports no change. It returns whether it changed
// anything.
func (d *IsoDiagram) editLayer(id string, mutate func(*IsoLayer) bool) bool {
	l, ok := d.doc.Layer(id)
	if !ok {
		return false
	}
	if !mutate(&l) {
		return false
	}
	d.beginEdit()
	d.doc.PutLayer(l)
	d.invalidate()
	return true
}

// RenameLayer sets a layer's display name (undoable). Returns false when the
// layer is absent or already carries that name.
func (d *IsoDiagram) RenameLayer(id, name string) bool {
	return d.editLayer(id, func(l *IsoLayer) bool {
		if l.Name == name {
			return false
		}
		l.Name = name
		return true
	})
}

// SetLayerVisible toggles a layer's visibility (undoable). Hiding a layer stops
// its entities from drawing and from being picked. Returns false when the layer
// is absent or already in that state.
func (d *IsoDiagram) SetLayerVisible(id string, visible bool) bool {
	return d.editLayer(id, func(l *IsoLayer) bool {
		if l.Visible == visible {
			return false
		}
		l.Visible = visible
		return true
	})
}

// SetLayerLocked toggles a layer's lock (undoable). A locked layer still draws
// but its entities cannot be picked or edited. Returns false when the layer is
// absent or already in that state.
func (d *IsoDiagram) SetLayerLocked(id string, locked bool) bool {
	return d.editLayer(id, func(l *IsoLayer) bool {
		if l.Locked == locked {
			return false
		}
		l.Locked = locked
		return true
	})
}

// SetLayerOrder sets a layer's back-to-front rank (undoable). Returns false when
// the layer is absent or already at that order.
func (d *IsoDiagram) SetLayerOrder(id string, order int) bool {
	return d.editLayer(id, func(l *IsoLayer) bool {
		if l.Order == order {
			return false
		}
		l.Order = order
		return true
	})
}

// refLayer returns the layer id the entity ref points at, and whether the
// entity was found.
func (d *IsoDiagram) refLayer(ref IsoEntityRef) (string, bool) {
	switch ref.Kind {
	case IsoEntityNode:
		if n, ok := d.doc.Node(ref.ID); ok {
			return n.Layer, true
		}
	case IsoEntityConnector:
		if c, ok := d.connectorByID(ref.ID); ok {
			return c.Layer, true
		}
	case IsoEntityZone:
		if z, ok := d.doc.Zone(ref.ID); ok {
			return z.Layer, true
		}
	default:
		if t, ok := d.doc.Text(ref.ID); ok {
			return t.Layer, true
		}
	}
	return "", false
}

// applyLayer writes layerID onto the entity ref points at (a missing entity is a
// no-op). It does not snapshot; the caller wraps a batch in one edit.
func (d *IsoDiagram) applyLayer(ref IsoEntityRef, layerID string) {
	switch ref.Kind {
	case IsoEntityNode:
		if n, ok := d.doc.Node(ref.ID); ok {
			n.Layer = layerID
			d.doc.PutNode(n)
		}
	case IsoEntityConnector:
		if c, ok := d.connectorByID(ref.ID); ok {
			c.Layer = layerID
			d.doc.PutConnector(c)
		}
	case IsoEntityZone:
		if z, ok := d.doc.Zone(ref.ID); ok {
			z.Layer = layerID
			d.doc.PutZone(z)
		}
	default:
		if t, ok := d.doc.Text(ref.ID); ok {
			t.Layer = layerID
			d.doc.PutText(t)
		}
	}
}

// AssignLayer moves one entity to the layer with layerID (undoable). Returns
// false when the entity is absent or already on that layer.
func (d *IsoDiagram) AssignLayer(ref IsoEntityRef, layerID string) bool {
	cur, ok := d.refLayer(ref)
	if !ok || cur == layerID {
		return false
	}
	d.beginEdit()
	d.applyLayer(ref, layerID)
	d.invalidate()
	return true
}

// AssignSelectionToLayer moves every selected entity to the layer with layerID
// as ONE undoable command. Returns false when nothing is selected or no
// entity's layer actually changes.
func (d *IsoDiagram) AssignSelectionToLayer(layerID string) bool {
	refs := d.selSet.Slice()
	if len(refs) == 0 {
		return false
	}
	changed := false
	for _, r := range refs {
		if cur, ok := d.refLayer(r); ok && cur != layerID {
			changed = true
			break
		}
	}
	if !changed {
		return false
	}
	d.beginEdit()
	for _, r := range refs {
		d.applyLayer(r, layerID)
	}
	d.invalidate()
	return true
}
