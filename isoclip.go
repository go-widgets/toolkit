// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// isoPasteOffset is how far, in grid cells, a paste shifts the copied entities
// from their originals on each axis, so a paste (or duplicate) never lands
// exactly on top of the source.
const isoPasteOffset = 1

// isoClip is the widget-internal clipboard: value copies of the entities a Copy
// or Cut captured, plus the connectors internal to the copied node set. It is a
// private model (not the system clipboard) so a paste is fully deterministic and
// needs no platform integration at this stage.
type isoClip struct {
	nodes []IsoNode
	conns []IsoConnector
	zones []IsoZone
	texts []IsoText
}

// empty reports whether the clipboard holds nothing to paste.
func (c isoClip) empty() bool {
	return len(c.nodes) == 0 && len(c.conns) == 0 && len(c.zones) == 0 && len(c.texts) == 0
}

// Copy captures the current multi-selection into the widget's clipboard: every
// selected node, zone and text, plus every connector INTERNAL to the selected
// nodes (both endpoints selected) so a pasted subgraph keeps its wiring. A
// selected connector with an unselected endpoint is not captured — it could not
// be reconnected on paste. Copy does not mutate the document and, with nothing
// selected, leaves the clipboard untouched.
func (d *IsoDiagram) Copy() {
	if d.selSet.Len() == 0 {
		return
	}
	var clip isoClip
	nodeSel := map[string]bool{}
	for _, n := range d.doc.Nodes() {
		if d.IsSelected(IsoEntityRef{Kind: IsoEntityNode, ID: n.ID}) {
			clip.nodes = append(clip.nodes, n)
			nodeSel[n.ID] = true
		}
	}
	for _, c := range d.doc.Connectors() {
		if nodeSel[c.From] && nodeSel[c.To] {
			clip.conns = append(clip.conns, c)
		}
	}
	for _, z := range d.doc.Zones() {
		if d.IsSelected(IsoEntityRef{Kind: IsoEntityZone, ID: z.ID}) {
			clip.zones = append(clip.zones, z)
		}
	}
	for _, t := range d.doc.Texts() {
		if d.IsSelected(IsoEntityRef{Kind: IsoEntityText, ID: t.ID}) {
			clip.texts = append(clip.texts, t)
		}
	}
	d.clip = clip
}

// Paste inserts the clipboard's entities as ONE undoable command, each shifted
// by [isoPasteOffset] cells and given a fresh document-unique id, and selects
// the newly pasted entities. Internal connectors are rewired onto the pasted
// nodes' new ids (the copied node set guarantees both endpoints were remapped).
// An empty clipboard is a no-op.
func (d *IsoDiagram) Paste() {
	if d.clip.empty() {
		return
	}
	d.beginEdit()
	idmap := map[string]string{}
	var refs []IsoEntityRef
	for _, n := range d.clip.nodes {
		nid := d.nextID("n")
		idmap[n.ID] = nid
		n.ID = nid
		n.X += isoPasteOffset
		n.Y += isoPasteOffset
		d.doc.PutNode(n)
		refs = append(refs, IsoEntityRef{Kind: IsoEntityNode, ID: nid})
	}
	for _, c := range d.clip.conns {
		c.ID = d.nextID("c")
		c.From = idmap[c.From]
		c.To = idmap[c.To]
		d.doc.PutConnector(c)
		refs = append(refs, IsoEntityRef{Kind: IsoEntityConnector, ID: c.ID})
	}
	for _, z := range d.clip.zones {
		z.ID = d.nextZoneID()
		z.X += isoPasteOffset
		z.Y += isoPasteOffset
		d.doc.PutZone(z)
		refs = append(refs, IsoEntityRef{Kind: IsoEntityZone, ID: z.ID})
	}
	for _, t := range d.clip.texts {
		t.ID = d.nextTextID()
		t.X += isoPasteOffset
		t.Y += isoPasteOffset
		d.doc.PutText(t)
		refs = append(refs, IsoEntityRef{Kind: IsoEntityText, ID: t.ID})
	}
	d.selReplace(refs...)
	d.invalidate()
}

// Cut copies the current selection and then deletes it — the copy and the delete
// being independent, so a following Paste re-inserts what was cut. The delete is
// one undoable command (see [IsoDiagram.DeleteSelection]).
func (d *IsoDiagram) Cut() {
	d.Copy()
	d.DeleteSelection()
}

// Duplicate copies the current selection and immediately pastes it (Ctrl-D): a
// one-gesture clone offset by [isoPasteOffset] cells, leaving the copy selected.
// With nothing selected it is a no-op and does not disturb the clipboard.
func (d *IsoDiagram) Duplicate() {
	if d.selSet.Len() == 0 {
		return
	}
	d.Copy()
	d.Paste()
}
