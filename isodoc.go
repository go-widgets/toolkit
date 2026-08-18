// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/mvvm"

// IsoShape names the solid an [IsoNode] renders as on the isometric grid.
type IsoShape int

const (
	// IsoCube is a unit cube one grid cell on every side — the default node.
	IsoCube IsoShape = iota
	// IsoBox is a 1x1 footprint raised twice as tall as a cube, for a node that
	// should read as a stack / server / tower rather than a single block.
	IsoBox
	// IsoPyramid is a 1x1 footprint tapering to an apex one unit up.
	IsoPyramid
)

// IsoNode is one placed element of an isometric diagram: an identity, a grid
// cell it occupies, the solid it draws as, a label and a base colour the
// isometric renderer shades per face. It is a plain value so the whole document
// is cheap to snapshot (undo) and to mirror into a CRDT OR-map keyed by ID.
type IsoNode struct {
	// ID is the node's stable identity — the key an OR-map / LWW store uses. A
	// document must not hold two nodes with the same ID; [IsoDoc.PutNode]
	// upserts on it.
	ID string
	// X, Y is the node's grid cell (integer world coordinates); the node's
	// footprint is the unit square [X,X+1] x [Y,Y+1].
	X, Y int
	// Shape selects the rendered solid (cube / box / pyramid). It is the fallback
	// geometry: it draws only when Icon is empty.
	Shape IsoShape
	// Icon, when non-empty, names an [IsoIcon] in the diagram's registry
	// ([IsoDiagram.Icons], else [IsoDefaultIcons]) that renders this node instead
	// of the bare Shape. An unknown id falls back to a plain cube. The field is a
	// plain string on the value node, so it snapshots for undo and flows through
	// the observable document like every other field — no per-frame copy.
	Icon string
	// Label is the caption drawn above the node and announced to a screen
	// reader.
	Label string
	// Color is the node's base colour; a zero value (A==0) inherits the theme
	// accent at draw time so a node placed without a colour is still visible.
	Color RGBA
}

// IsoConnectorStyle selects the stroke pattern of an [IsoConnector].
type IsoConnectorStyle int

const (
	// IsoSolid is a continuous line — the zero value and the legacy default, so a
	// connector that never sets a style draws exactly as it did before styles
	// existed.
	IsoSolid IsoConnectorStyle = iota
	// IsoDashed is a line of evenly spaced dashes.
	IsoDashed
	// IsoDotted is a line of short round dots.
	IsoDotted
)

// IsoArrow selects the end decorations of an [IsoConnector].
type IsoArrow int

const (
	// IsoArrowNone draws no head — the zero value and the legacy default, so a
	// connector that never sets an arrow renders as the bare line it always was.
	IsoArrowNone IsoArrow = iota
	// IsoArrowSingle draws a filled head at the target (To) end.
	IsoArrowSingle
	// IsoArrowDouble draws a head at both the source (From) and target (To) ends.
	IsoArrowDouble
)

// IsoConnector is a directed link between two [IsoNode]s. From and To are node
// IDs; a connector whose endpoints are not both present in the document is
// skipped when drawing.
//
// Every field beyond ID/From/To is an enrichment whose zero value reproduces the
// original bare connector: an unset connector routes as a single straight
// segment between the two node top anchors, solid, headless, in the theme's link
// colour, at the default width — byte-for-byte the pre-enrichment rendering.
type IsoConnector struct {
	// ID is the connector's stable identity (the OR-map key).
	ID string
	// From and To are the IDs of the source and destination nodes.
	From, To string
	// Label is an optional caption drawn along the routed path (at its midpoint,
	// offset off the line so it does not sit on the stroke).
	Label string
	// Style selects the stroke pattern. The zero value [IsoSolid] is the legacy
	// continuous line.
	Style IsoConnectorStyle
	// Arrow selects the end heads. The zero value [IsoArrowNone] draws none.
	Arrow IsoArrow
	// Color is the stroke colour; a zero value (A==0) inherits the theme's link
	// colour (OnSurface) at draw time, so a connector left uncoloured is still
	// visible and follows the light/dark theme like the rest of the widget.
	Color RGBA
	// Width is the stroke thickness in logical pixels; zero uses the default and
	// any positive value is routed through the toolkit's HiDPI/density scale, so
	// connectors thicken in lockstep with the rest of the chrome.
	Width int
	// Routed, when true, routes the connector along the isometric grid — a chain
	// of grid-orthogonal segments anchored on the source and target node faces
	// (the face nearest the neighbour) — instead of the single straight
	// anchor-to-anchor segment. The zero value (false) is the legacy straight
	// line. The route is COMPUTED from the endpoints at draw time, never stored.
	Routed bool
}

// IsoZone is a rectangular coloured region on the ground plane (z=0) that groups
// nodes visually. It occupies the tile rectangle [X, X+W] x [Y, Y+H]: (X, Y) is
// its minimum corner in grid cells and (W, H) is its size in cells. Like every
// other entity it is a plain value so the whole document snapshots cheaply for
// undo and mirrors into a CRDT OR-map keyed by ID.
type IsoZone struct {
	// ID is the zone's stable identity — the OR-map key. A document must not hold
	// two zones with the same ID; [IsoDoc.PutZone] upserts on it.
	ID string
	// X, Y is the zone's minimum corner in grid cells (integer world
	// coordinates).
	X, Y int
	// W, H is the zone's size in grid cells. The widget's editing commands keep
	// both at least 1, so a zone always covers a non-empty rectangle.
	W, H int
	// Color is the zone's fill colour; its alpha is the fill opacity, so a
	// semi-transparent value tints the ground without hiding the grid. A zero
	// value (A==0) inherits a translucent theme accent at draw time, so a zone
	// created without a colour is still visible and follows the theme.
	Color RGBA
	// Label is an optional caption drawn in a corner of the zone and announced to
	// a screen reader.
	Label string
}

// IsoText is a standalone text annotation anchored at a tile position on the
// ground plane. It is not attached to any node; it floats on the topmost layer,
// above every node, connector and zone. Like the other entities it is a plain
// value for cheap snapshotting and CRDT mirroring.
type IsoText struct {
	// ID is the annotation's stable identity (the OR-map key).
	ID string
	// X, Y is the annotation's anchor tile (integer world coordinates); the text
	// is centred on that cell's projected ground centre.
	X, Y int
	// Text is the string drawn. An empty string still selects and moves as an
	// (invisible) annotation, so an editor can place one and fill it in later.
	Text string
	// Color is the ink; a zero value (A==0) inherits the theme's OnSurface colour
	// at draw time so an uncoloured annotation is still legible and theme-aware.
	Color RGBA
	// Size is the integer type scale routed through the toolkit HiDPI/density
	// scale (1 is the base bitmap font, 2 double-height, ...). Zero uses the
	// widget's effective font, so an annotation placed without a size matches the
	// rest of the chrome.
	Size int
}

// IsoDocument is the backing store an [IsoDiagram] edits. It is deliberately a
// small interface over "sets of nodes, connectors, zones and text annotations,
// each keyed by ID, with change notification" so the widget never reaches into a
// concrete store. The bundled [IsoDoc] backs it with go-widgets/mvvm observable
// lists; a structured-collab CRDT (each entity set an OR-map with LWW fields)
// that implements this same interface is a drop-in replacement — the widget
// compiles unchanged against either.
type IsoDocument interface {
	// Nodes returns a snapshot copy of every node, in insertion order.
	Nodes() []IsoNode
	// Connectors returns a snapshot copy of every connector, in insertion order.
	Connectors() []IsoConnector
	// Zones returns a snapshot copy of every zone, in insertion order.
	Zones() []IsoZone
	// Texts returns a snapshot copy of every text annotation, in insertion order.
	Texts() []IsoText
	// Node returns the node with the given ID and whether it was found.
	Node(id string) (IsoNode, bool)
	// Zone returns the zone with the given ID and whether it was found.
	Zone(id string) (IsoZone, bool)
	// Text returns the text annotation with the given ID and whether it was found.
	Text(id string) (IsoText, bool)
	// PutNode inserts a node or, when one already has the same ID, replaces it
	// (an OR-map upsert with last-writer-wins fields).
	PutNode(n IsoNode)
	// RemoveNode deletes the node with the given ID and every connector that
	// touches it. A missing ID is a no-op.
	RemoveNode(id string)
	// PutConnector inserts or replaces a connector by ID.
	PutConnector(c IsoConnector)
	// RemoveConnector deletes a connector by ID; a missing ID is a no-op.
	RemoveConnector(id string)
	// PutZone inserts or replaces a zone by ID.
	PutZone(z IsoZone)
	// RemoveZone deletes a zone by ID; a missing ID is a no-op.
	RemoveZone(id string)
	// PutText inserts or replaces a text annotation by ID.
	PutText(t IsoText)
	// RemoveText deletes a text annotation by ID; a missing ID is a no-op.
	RemoveText(id string)
	// Subscribe registers fn to run after every mutation and returns a function
	// that unsubscribes it.
	Subscribe(fn func()) (unsubscribe func())
}

// IsoDoc is the default in-memory [IsoDocument], backed by two
// go-widgets/mvvm ObservableLists (one for nodes, one for connectors). Because
// every edit flows through an Observable, the widget binds to the document once
// and repaints on change instead of copying node fields every frame — the MVVM
// discipline the toolkit follows — and a host can bind the same lists into its
// own view models. Swapping this for a CRDT-backed store means implementing
// [IsoDocument] elsewhere; nothing else changes.
type IsoDoc struct {
	nodes *mvvm.ObservableList[IsoNode]
	conns *mvvm.ObservableList[IsoConnector]
	zones *mvvm.ObservableList[IsoZone]
	texts *mvvm.ObservableList[IsoText]
}

// NewIsoDoc returns an empty document.
func NewIsoDoc() *IsoDoc {
	return &IsoDoc{
		nodes: mvvm.NewObservableList[IsoNode](),
		conns: mvvm.NewObservableList[IsoConnector](),
		zones: mvvm.NewObservableList[IsoZone](),
		texts: mvvm.NewObservableList[IsoText](),
	}
}

// NodeList exposes the underlying observable node collection so a host can bind
// it into an MVVM view (e.g. a palette or an outline list) that stays in sync
// with the diagram. Mutating it directly is equivalent to the Put/Remove
// methods.
func (d *IsoDoc) NodeList() *mvvm.ObservableList[IsoNode] { return d.nodes }

// ConnectorList exposes the underlying observable connector collection (see
// [IsoDoc.NodeList]).
func (d *IsoDoc) ConnectorList() *mvvm.ObservableList[IsoConnector] { return d.conns }

// ZoneList exposes the underlying observable zone collection (see
// [IsoDoc.NodeList]).
func (d *IsoDoc) ZoneList() *mvvm.ObservableList[IsoZone] { return d.zones }

// TextList exposes the underlying observable text-annotation collection (see
// [IsoDoc.NodeList]).
func (d *IsoDoc) TextList() *mvvm.ObservableList[IsoText] { return d.texts }

// Nodes returns a snapshot copy of every node.
func (d *IsoDoc) Nodes() []IsoNode { return d.nodes.Slice() }

// Connectors returns a snapshot copy of every connector.
func (d *IsoDoc) Connectors() []IsoConnector { return d.conns.Slice() }

// Zones returns a snapshot copy of every zone.
func (d *IsoDoc) Zones() []IsoZone { return d.zones.Slice() }

// Texts returns a snapshot copy of every text annotation.
func (d *IsoDoc) Texts() []IsoText { return d.texts.Slice() }

// nodeIndex returns the position of the node with the given ID, or -1.
func (d *IsoDoc) nodeIndex(id string) int {
	for i := 0; i < d.nodes.Len(); i++ {
		if d.nodes.At(i).ID == id {
			return i
		}
	}
	return -1
}

// connIndex returns the position of the connector with the given ID, or -1.
func (d *IsoDoc) connIndex(id string) int {
	for i := 0; i < d.conns.Len(); i++ {
		if d.conns.At(i).ID == id {
			return i
		}
	}
	return -1
}

// zoneIndex returns the position of the zone with the given ID, or -1.
func (d *IsoDoc) zoneIndex(id string) int {
	for i := 0; i < d.zones.Len(); i++ {
		if d.zones.At(i).ID == id {
			return i
		}
	}
	return -1
}

// textIndex returns the position of the text annotation with the given ID, or
// -1.
func (d *IsoDoc) textIndex(id string) int {
	for i := 0; i < d.texts.Len(); i++ {
		if d.texts.At(i).ID == id {
			return i
		}
	}
	return -1
}

// Node returns the node with the given ID and whether it exists.
func (d *IsoDoc) Node(id string) (IsoNode, bool) {
	if i := d.nodeIndex(id); i >= 0 {
		return d.nodes.At(i), true
	}
	return IsoNode{}, false
}

// PutNode upserts n by its ID.
func (d *IsoDoc) PutNode(n IsoNode) {
	if i := d.nodeIndex(n.ID); i >= 0 {
		d.nodes.Set(i, n)
		return
	}
	d.nodes.Append(n)
}

// RemoveNode deletes the node with id and every connector attached to it.
func (d *IsoDoc) RemoveNode(id string) {
	i := d.nodeIndex(id)
	if i < 0 {
		return
	}
	d.nodes.RemoveAt(i)
	for j := d.conns.Len() - 1; j >= 0; j-- {
		if c := d.conns.At(j); c.From == id || c.To == id {
			d.conns.RemoveAt(j)
		}
	}
}

// PutConnector upserts c by its ID.
func (d *IsoDoc) PutConnector(c IsoConnector) {
	if i := d.connIndex(c.ID); i >= 0 {
		d.conns.Set(i, c)
		return
	}
	d.conns.Append(c)
}

// RemoveConnector deletes the connector with id.
func (d *IsoDoc) RemoveConnector(id string) {
	if i := d.connIndex(id); i >= 0 {
		d.conns.RemoveAt(i)
	}
}

// Zone returns the zone with the given ID and whether it exists.
func (d *IsoDoc) Zone(id string) (IsoZone, bool) {
	if i := d.zoneIndex(id); i >= 0 {
		return d.zones.At(i), true
	}
	return IsoZone{}, false
}

// PutZone upserts z by its ID.
func (d *IsoDoc) PutZone(z IsoZone) {
	if i := d.zoneIndex(z.ID); i >= 0 {
		d.zones.Set(i, z)
		return
	}
	d.zones.Append(z)
}

// RemoveZone deletes the zone with id.
func (d *IsoDoc) RemoveZone(id string) {
	if i := d.zoneIndex(id); i >= 0 {
		d.zones.RemoveAt(i)
	}
}

// Text returns the text annotation with the given ID and whether it exists.
func (d *IsoDoc) Text(id string) (IsoText, bool) {
	if i := d.textIndex(id); i >= 0 {
		return d.texts.At(i), true
	}
	return IsoText{}, false
}

// PutText upserts t by its ID.
func (d *IsoDoc) PutText(t IsoText) {
	if i := d.textIndex(t.ID); i >= 0 {
		d.texts.Set(i, t)
		return
	}
	d.texts.Append(t)
}

// RemoveText deletes the text annotation with id.
func (d *IsoDoc) RemoveText(id string) {
	if i := d.textIndex(id); i >= 0 {
		d.texts.RemoveAt(i)
	}
}

// Subscribe runs fn after any node, connector, zone or text edit and returns an
// unsubscribe function. It fans out to every observable list so a single
// subscription covers the whole document.
func (d *IsoDoc) Subscribe(fn func()) (unsubscribe func()) {
	un := d.nodes.SubscribeChanged(fn)
	uc := d.conns.SubscribeChanged(fn)
	uz := d.zones.SubscribeChanged(fn)
	ut := d.texts.SubscribeChanged(fn)
	return func() {
		un()
		uc()
		uz()
		ut()
	}
}
