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

// IsoDocument is the backing store an [IsoDiagram] edits. It is deliberately a
// small interface over "a set of nodes and a set of connectors, keyed by ID,
// with change notification" so the widget never reaches into a concrete store.
// The bundled [IsoDoc] backs it with go-widgets/mvvm observable lists; a
// structured-collab CRDT (nodes/connectors as OR-maps with LWW fields) that
// implements this same interface is a drop-in replacement — the widget compiles
// unchanged against either.
type IsoDocument interface {
	// Nodes returns a snapshot copy of every node, in insertion order.
	Nodes() []IsoNode
	// Connectors returns a snapshot copy of every connector, in insertion order.
	Connectors() []IsoConnector
	// Node returns the node with the given ID and whether it was found.
	Node(id string) (IsoNode, bool)
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
}

// NewIsoDoc returns an empty document.
func NewIsoDoc() *IsoDoc {
	return &IsoDoc{
		nodes: mvvm.NewObservableList[IsoNode](),
		conns: mvvm.NewObservableList[IsoConnector](),
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

// Nodes returns a snapshot copy of every node.
func (d *IsoDoc) Nodes() []IsoNode { return d.nodes.Slice() }

// Connectors returns a snapshot copy of every connector.
func (d *IsoDoc) Connectors() []IsoConnector { return d.conns.Slice() }

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

// Subscribe runs fn after any node or connector edit and returns an
// unsubscribe function. It fans out to both observable lists so a single
// subscription covers the whole document.
func (d *IsoDoc) Subscribe(fn func()) (unsubscribe func()) {
	un := d.nodes.SubscribeChanged(fn)
	uc := d.conns.SubscribeChanged(fn)
	return func() {
		un()
		uc()
	}
}
