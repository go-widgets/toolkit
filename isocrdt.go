// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"

	"github.com/go-crdt/crdt"
	"github.com/go-crdt/crdt/structured"
	"github.com/go-widgets/mvvm"
)

// IsoCRDTDocument is a collaborative [IsoDocument] backed by the shared
// structured-document CRDT core (structured.Document): every entity of all five
// families — nodes, connectors, zones, text annotations and layers — lives in
// one crdt.Composite, so any number of replicas may edit the diagram at once,
// offline and in any order, and every replica converges to the same diagram. It
// is a drop-in for the in-memory [IsoDoc]: an [IsoDiagram] built on it renders
// and edits through the exact same interface, with no widget change.
//
// It adds NO merge logic. The convergence, commutativity, idempotence and
// associativity are the structured/crdt package's, inherited whole. This type is
// only the mapping between the widget's value entities and the CRDT's records:
//
//   - An entity's stable string ID ([IsoNode.ID] and the rest) is the CRDT
//     record key directly — there is no local id table. Two replicas that place
//     "server-a" both name the same record and converge.
//   - Each entity field is one independent LWW register under the caller's own
//     field name, so one replica moving a node while another recolours it keeps
//     both edits. Only a field whose value differs from the entity's default is
//     stored, so a snapshot stays compact and re-reads to the same value.
//   - Integer fields (positions, sizes, order, width) travel through
//     [structured.EncodeInt] and boolean fields through [structured.EncodeBool],
//     so a document compiled to js/wasm stores byte-for-byte what a 64-bit server
//     does. Colours and enums reuse this package's own JSON codec words, so there
//     is no third encoding of a shape or a colour.
//
// # Fan-out through MVVM
//
// State crossing into a view crosses through go-widgets/mvvm: a local edit and an
// applied remote batch both tick an [mvvm.Observable] revision, and
// [IsoCRDTDocument.Subscribe] — the one method [IsoDiagram] binds to — registers
// a change observer over it, so every bound view repaints on a merge exactly as
// it does on a local edit.
//
// # Transport
//
// [IsoCRDTDocument.OpsSince] and [IsoCRDTDocument.Apply] exchange operations with
// peers; [IsoCRDTDocument.Snapshot] and [LoadIsoCRDTDocument] carry a whole
// document to a late joiner or to disk; [IsoCRDTDocument.Version] drives a delta
// sync. All of it is the structured.Document's own transport, re-exposed.
//
// An IsoCRDTDocument is not safe for concurrent use; drive it from one goroutine
// and exchange operations, not the value, between replicas.
type IsoCRDTDocument struct {
	doc *structured.Document
	rev *mvvm.Observable[uint64]
}

// Compile-time proof that the collaborative document is a drop-in for the
// in-memory one: the widget compiles unchanged against either.
var _ IsoDocument = (*IsoCRDTDocument)(nil)

// Entity field names. They are THIS adapter's schema — the structured core knows
// none of them — and are reused across families where the meaning is the same
// (a zone and a node both have an "x"), which is safe because each family is a
// separate record map.
const (
	fldX       = "x"
	fldY       = "y"
	fldW       = "w"
	fldH       = "h"
	fldShape   = "shape"
	fldIcon    = "icon"
	fldLabel   = "label"
	fldColor   = "color"
	fldLayer   = "layer"
	fldFrom    = "from"
	fldTo      = "to"
	fldStyle   = "style"
	fldArrow   = "arrow"
	fldWidth   = "width"
	fldRouted  = "routed"
	fldText    = "text"
	fldSize    = "size"
	fldName    = "name"
	fldVisible = "visible"
	fldLocked  = "locked"
	fldOrder   = "order"
)

// NewIsoCRDTDocument returns an empty collaborative document that issues
// operations as site. Every replica editing one diagram concurrently must pass a
// distinct [crdt.SiteID].
func NewIsoCRDTDocument(site crdt.SiteID) *IsoCRDTDocument {
	return &IsoCRDTDocument{doc: structured.NewDocument(site), rev: mvvm.NewObservable[uint64](0)}
}

// LoadIsoCRDTDocument rebuilds a collaborative document from a
// [IsoCRDTDocument.Snapshot], to be edited as site. A malformed snapshot is
// returned as an error, never a panic.
func LoadIsoCRDTDocument(site crdt.SiteID, snapshot []byte) (*IsoCRDTDocument, error) {
	doc, err := structured.LoadDocument(site, snapshot)
	if err != nil {
		return nil, err
	}
	return &IsoCRDTDocument{doc: doc, rev: mvvm.NewObservable[uint64](0)}, nil
}

// notify ticks the revision so every change observer runs. The value only
// increases, so the observable never suppresses the notice.
func (d *IsoCRDTDocument) notify() { d.rev.Set(d.rev.Get() + 1) }

// Subscribe registers fn to run after every change — a local edit or an applied
// remote batch — and returns a function that unsubscribes it. It is the seam the
// widget binds its repaint to.
func (d *IsoCRDTDocument) Subscribe(fn func()) (unsubscribe func()) {
	return d.rev.SubscribeChanged(fn)
}

// Rev returns the current revision counter, which ticks on every change.
func (d *IsoCRDTDocument) Rev() uint64 { return d.rev.Get() }

// Site returns the replica identity this document issues operations as.
func (d *IsoCRDTDocument) Site() crdt.SiteID { return d.doc.Site() }

// Snapshot encodes the whole document — every family — for a joining peer or for
// persistence. Two replicas holding the same operations produce identical bytes.
func (d *IsoCRDTDocument) Snapshot() []byte { return d.doc.Snapshot() }

// Version returns what this replica holds, to hand a peer that sends back what it
// is missing; see [IsoCRDTDocument.OpsSince].
func (d *IsoCRDTDocument) Version() crdt.CompositeVersion { return d.doc.Version() }

// OpsSince returns the operations this replica holds that v does not, across all
// families, ready to send to the peer that produced v. Pass a nil version for
// everything.
//
// It refuses with [crdt.ErrCollected] below what this replica has collected:
// what collection gave back is not in a difference any more, so a peer that far
// behind has to be sent a snapshot instead.
func (d *IsoCRDTDocument) OpsSince(v crdt.CompositeVersion) ([]crdt.PartOps, error) {
	return d.doc.OpsSince(v)
}

// Pending reports how many received operations are still waiting for the
// operations they depend on. It is zero once a replica has caught up.
func (d *IsoCRDTDocument) Pending() int { return d.doc.Pending() }

// Apply integrates batches of operations from peers, tolerating duplicates and
// reordering, and ticks the revision so every bound view repaints. An invalid
// batch is reported as an error and changes nothing.
func (d *IsoCRDTDocument) Apply(batches ...crdt.PartOps) error {
	if err := d.doc.Apply(batches...); err != nil {
		return err
	}
	d.notify()
	return nil
}

// --- field plumbing ------------------------------------------------------------

// ensure makes an entity present before its fields are written, so an entity
// carrying only default fields is still a member of its family. Add is idempotent
// by id, so re-ensuring an existing entity changes nothing.
func (d *IsoCRDTDocument) ensure(fam structured.Family, id string) {
	if !d.doc.Has(fam, id) {
		// An invalid id (empty or non-UTF-8) is refused by the core; the entity
		// simply does not become present, which is the right no-op for a store whose
		// keys must be valid.
		d.doc.Add(fam, id)
	}
}

// setField brings one field to a desired state: written when present and changed,
// removed when it should carry its default (so the record does not keep a stale
// override), and left untouched when it already agrees — the minimal operation,
// which is what keeps a field an independent LWW register rather than dragging
// the whole entity along on every edit.
func (d *IsoCRDTDocument) setField(fam structured.Family, id, field string, want []byte, present bool) {
	cur, has := d.doc.Field(fam, id, field)
	switch {
	case present && (!has || !bytes.Equal(cur, want)):
		d.doc.Set(fam, id, field, want)
	case !present && has:
		d.doc.DeleteField(fam, id, field)
	}
}

// strField reads a string field, "" when unset.
func (d *IsoCRDTDocument) strField(fam structured.Family, id, field string) string {
	if b, ok := d.doc.Field(fam, id, field); ok {
		return string(b)
	}
	return ""
}

// intField reads an integer field, 0 when unset or when a peer stored bytes this
// adapter did not write.
func (d *IsoCRDTDocument) intField(fam structured.Family, id, field string) int {
	if b, ok := d.doc.Field(fam, id, field); ok {
		v, _ := structured.DecodeInt(b)
		return int(v)
	}
	return 0
}

// boolField reads a boolean field, false when unset or when a peer stored bytes
// this adapter did not write.
func (d *IsoCRDTDocument) boolField(fam structured.Family, id, field string) bool {
	if b, ok := d.doc.Field(fam, id, field); ok {
		v, _ := structured.DecodeBool(b)
		return v
	}
	return false
}

// --- nodes ---------------------------------------------------------------------

// PutNode inserts a node or, when one already has the same ID, replaces its
// fields (an upsert with last-writer-wins fields).
func (d *IsoCRDTDocument) PutNode(n IsoNode) {
	fam := structured.Nodes
	d.ensure(fam, n.ID)
	shape := isoShapeToStr(n.Shape)
	color := isoColorToHex(n.Color)
	d.setField(fam, n.ID, fldX, structured.EncodeInt(int32(n.X)), n.X != 0)
	d.setField(fam, n.ID, fldY, structured.EncodeInt(int32(n.Y)), n.Y != 0)
	d.setField(fam, n.ID, fldShape, []byte(shape), shape != "")
	d.setField(fam, n.ID, fldIcon, []byte(n.Icon), n.Icon != "")
	d.setField(fam, n.ID, fldLabel, []byte(n.Label), n.Label != "")
	d.setField(fam, n.ID, fldColor, []byte(color), color != "")
	d.setField(fam, n.ID, fldLayer, []byte(n.Layer), n.Layer != "")
	d.notify()
}

// readNode reconstructs a node from its record. An enum or colour a peer stored
// malformed reads as its default rather than an error.
func (d *IsoCRDTDocument) readNode(id string) IsoNode {
	fam := structured.Nodes
	shape, _ := isoShapeFromStr(d.strField(fam, id, fldShape))
	color, _ := isoColorFromHex(d.strField(fam, id, fldColor))
	return IsoNode{
		ID:    id,
		X:     d.intField(fam, id, fldX),
		Y:     d.intField(fam, id, fldY),
		Shape: shape,
		Icon:  d.strField(fam, id, fldIcon),
		Label: d.strField(fam, id, fldLabel),
		Color: color,
		Layer: d.strField(fam, id, fldLayer),
	}
}

// Nodes returns every node present, ascending by ID.
func (d *IsoCRDTDocument) Nodes() []IsoNode {
	ids := d.doc.IDs(structured.Nodes)
	out := make([]IsoNode, 0, len(ids))
	for _, id := range ids {
		out = append(out, d.readNode(id))
	}
	return out
}

// Node returns the node with the given ID and whether it exists.
func (d *IsoCRDTDocument) Node(id string) (IsoNode, bool) {
	if !d.doc.Has(structured.Nodes, id) {
		return IsoNode{}, false
	}
	return d.readNode(id), true
}

// RemoveNode deletes the node with id and every connector attached to it. A
// missing ID is a complete no-op — no connector is cascaded — exactly as the
// in-memory document behaves.
func (d *IsoCRDTDocument) RemoveNode(id string) {
	if !d.doc.Has(structured.Nodes, id) {
		return
	}
	d.doc.Remove(structured.Nodes, id)
	for _, c := range d.Connectors() {
		if c.From == id || c.To == id {
			d.doc.Remove(structured.Connectors, c.ID)
		}
	}
	d.notify()
}

// --- connectors ----------------------------------------------------------------

// PutConnector inserts or replaces a connector by ID.
func (d *IsoCRDTDocument) PutConnector(c IsoConnector) {
	fam := structured.Connectors
	d.ensure(fam, c.ID)
	style := isoStyleToStr(c.Style)
	arrow := isoArrowToStr(c.Arrow)
	color := isoColorToHex(c.Color)
	d.setField(fam, c.ID, fldFrom, []byte(c.From), c.From != "")
	d.setField(fam, c.ID, fldTo, []byte(c.To), c.To != "")
	d.setField(fam, c.ID, fldLabel, []byte(c.Label), c.Label != "")
	d.setField(fam, c.ID, fldStyle, []byte(style), style != "")
	d.setField(fam, c.ID, fldArrow, []byte(arrow), arrow != "")
	d.setField(fam, c.ID, fldColor, []byte(color), color != "")
	d.setField(fam, c.ID, fldWidth, structured.EncodeInt(int32(c.Width)), c.Width != 0)
	d.setField(fam, c.ID, fldRouted, structured.EncodeBool(c.Routed), c.Routed)
	d.setField(fam, c.ID, fldLayer, []byte(c.Layer), c.Layer != "")
	d.notify()
}

// readConn reconstructs a connector from its record.
func (d *IsoCRDTDocument) readConn(id string) IsoConnector {
	fam := structured.Connectors
	style, _ := isoStyleFromStr(d.strField(fam, id, fldStyle))
	arrow, _ := isoArrowFromStr(d.strField(fam, id, fldArrow))
	color, _ := isoColorFromHex(d.strField(fam, id, fldColor))
	return IsoConnector{
		ID:     id,
		From:   d.strField(fam, id, fldFrom),
		To:     d.strField(fam, id, fldTo),
		Label:  d.strField(fam, id, fldLabel),
		Style:  style,
		Arrow:  arrow,
		Color:  color,
		Width:  d.intField(fam, id, fldWidth),
		Routed: d.boolField(fam, id, fldRouted),
		Layer:  d.strField(fam, id, fldLayer),
	}
}

// Connectors returns every connector present, ascending by ID.
func (d *IsoCRDTDocument) Connectors() []IsoConnector {
	ids := d.doc.IDs(structured.Connectors)
	out := make([]IsoConnector, 0, len(ids))
	for _, id := range ids {
		out = append(out, d.readConn(id))
	}
	return out
}

// RemoveConnector deletes a connector by ID; a missing ID is a no-op.
func (d *IsoCRDTDocument) RemoveConnector(id string) {
	d.doc.Remove(structured.Connectors, id)
	d.notify()
}

// --- zones ---------------------------------------------------------------------

// PutZone inserts or replaces a zone by ID.
func (d *IsoCRDTDocument) PutZone(z IsoZone) {
	fam := structured.Zones
	d.ensure(fam, z.ID)
	color := isoColorToHex(z.Color)
	d.setField(fam, z.ID, fldX, structured.EncodeInt(int32(z.X)), z.X != 0)
	d.setField(fam, z.ID, fldY, structured.EncodeInt(int32(z.Y)), z.Y != 0)
	d.setField(fam, z.ID, fldW, structured.EncodeInt(int32(z.W)), z.W != 0)
	d.setField(fam, z.ID, fldH, structured.EncodeInt(int32(z.H)), z.H != 0)
	d.setField(fam, z.ID, fldColor, []byte(color), color != "")
	d.setField(fam, z.ID, fldLabel, []byte(z.Label), z.Label != "")
	d.setField(fam, z.ID, fldLayer, []byte(z.Layer), z.Layer != "")
	d.notify()
}

// readZone reconstructs a zone from its record.
func (d *IsoCRDTDocument) readZone(id string) IsoZone {
	fam := structured.Zones
	color, _ := isoColorFromHex(d.strField(fam, id, fldColor))
	return IsoZone{
		ID:    id,
		X:     d.intField(fam, id, fldX),
		Y:     d.intField(fam, id, fldY),
		W:     d.intField(fam, id, fldW),
		H:     d.intField(fam, id, fldH),
		Color: color,
		Label: d.strField(fam, id, fldLabel),
		Layer: d.strField(fam, id, fldLayer),
	}
}

// Zones returns every zone present, ascending by ID.
func (d *IsoCRDTDocument) Zones() []IsoZone {
	ids := d.doc.IDs(structured.Zones)
	out := make([]IsoZone, 0, len(ids))
	for _, id := range ids {
		out = append(out, d.readZone(id))
	}
	return out
}

// Zone returns the zone with the given ID and whether it exists.
func (d *IsoCRDTDocument) Zone(id string) (IsoZone, bool) {
	if !d.doc.Has(structured.Zones, id) {
		return IsoZone{}, false
	}
	return d.readZone(id), true
}

// RemoveZone deletes a zone by ID; a missing ID is a no-op.
func (d *IsoCRDTDocument) RemoveZone(id string) {
	d.doc.Remove(structured.Zones, id)
	d.notify()
}

// --- text annotations ----------------------------------------------------------

// PutText inserts or replaces a text annotation by ID.
func (d *IsoCRDTDocument) PutText(t IsoText) {
	fam := structured.TextBoxes
	d.ensure(fam, t.ID)
	color := isoColorToHex(t.Color)
	d.setField(fam, t.ID, fldX, structured.EncodeInt(int32(t.X)), t.X != 0)
	d.setField(fam, t.ID, fldY, structured.EncodeInt(int32(t.Y)), t.Y != 0)
	d.setField(fam, t.ID, fldText, []byte(t.Text), t.Text != "")
	d.setField(fam, t.ID, fldColor, []byte(color), color != "")
	d.setField(fam, t.ID, fldSize, structured.EncodeInt(int32(t.Size)), t.Size != 0)
	d.setField(fam, t.ID, fldLayer, []byte(t.Layer), t.Layer != "")
	d.notify()
}

// readText reconstructs a text annotation from its record.
func (d *IsoCRDTDocument) readText(id string) IsoText {
	fam := structured.TextBoxes
	color, _ := isoColorFromHex(d.strField(fam, id, fldColor))
	return IsoText{
		ID:    id,
		X:     d.intField(fam, id, fldX),
		Y:     d.intField(fam, id, fldY),
		Text:  d.strField(fam, id, fldText),
		Color: color,
		Size:  d.intField(fam, id, fldSize),
		Layer: d.strField(fam, id, fldLayer),
	}
}

// Texts returns every text annotation present, ascending by ID.
func (d *IsoCRDTDocument) Texts() []IsoText {
	ids := d.doc.IDs(structured.TextBoxes)
	out := make([]IsoText, 0, len(ids))
	for _, id := range ids {
		out = append(out, d.readText(id))
	}
	return out
}

// Text returns the text annotation with the given ID and whether it exists.
func (d *IsoCRDTDocument) Text(id string) (IsoText, bool) {
	if !d.doc.Has(structured.TextBoxes, id) {
		return IsoText{}, false
	}
	return d.readText(id), true
}

// RemoveText deletes a text annotation by ID; a missing ID is a no-op.
func (d *IsoCRDTDocument) RemoveText(id string) {
	d.doc.Remove(structured.TextBoxes, id)
	d.notify()
}

// --- layers --------------------------------------------------------------------

// PutLayer inserts or replaces a layer by ID.
func (d *IsoCRDTDocument) PutLayer(l IsoLayer) {
	fam := structured.Layers
	d.ensure(fam, l.ID)
	d.setField(fam, l.ID, fldName, []byte(l.Name), l.Name != "")
	d.setField(fam, l.ID, fldVisible, structured.EncodeBool(l.Visible), l.Visible)
	d.setField(fam, l.ID, fldLocked, structured.EncodeBool(l.Locked), l.Locked)
	d.setField(fam, l.ID, fldOrder, structured.EncodeInt(int32(l.Order)), l.Order != 0)
	d.notify()
}

// readLayer reconstructs a layer from its record.
func (d *IsoCRDTDocument) readLayer(id string) IsoLayer {
	fam := structured.Layers
	return IsoLayer{
		ID:      id,
		Name:    d.strField(fam, id, fldName),
		Visible: d.boolField(fam, id, fldVisible),
		Locked:  d.boolField(fam, id, fldLocked),
		Order:   d.intField(fam, id, fldOrder),
	}
}

// Layers returns every layer present, ascending by ID.
func (d *IsoCRDTDocument) Layers() []IsoLayer {
	ids := d.doc.IDs(structured.Layers)
	out := make([]IsoLayer, 0, len(ids))
	for _, id := range ids {
		out = append(out, d.readLayer(id))
	}
	return out
}

// Layer returns the layer with the given ID and whether it exists.
func (d *IsoCRDTDocument) Layer(id string) (IsoLayer, bool) {
	if !d.doc.Has(structured.Layers, id) {
		return IsoLayer{}, false
	}
	return d.readLayer(id), true
}

// RemoveLayer deletes a layer by ID; a missing ID is a no-op. Entities still
// naming a removed layer fall back to the implicit default layer at draw time,
// exactly as with the in-memory document.
func (d *IsoCRDTDocument) RemoveLayer(id string) {
	d.doc.Remove(structured.Layers, id)
	d.notify()
}
