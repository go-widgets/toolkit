// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package isodemo is a runnable, self-checking showcase of the toolkit's
// isometric diagram widget ([toolkit.IsoDiagram]) and its collaborative CRDT
// backing store ([toolkit.IsoCRDTDocument]).
//
// It is built entirely on the widget's PUBLIC API — the same calls a host
// application makes — and renders through the toolkit's headless capture path
// ([toolkit.RenderPNG]), so it needs no window, display server or GPU. Every
// scene here is assembled by a reusable builder (never hidden inside a main),
// so the accompanying test can render each one to an in-memory buffer and
// assert on EXACT projected pixel positions and colours (see isodemo_test.go),
// and the cmd/isodemo binary can write the same scenes to PNG files for a human
// to inspect.
//
// The five families of the diagram model are all exercised:
//
//   - [toolkit.IsoNode]     — placed solids, some drawn as their bare shape and
//     some as a registry icon (server / database / cloud / router / switch /
//     storage), each with its own base colour the isometric renderer shades
//     per face.
//   - [toolkit.IsoConnector] — directed links in every stroke style (solid /
//     dashed / dotted), arrow decoration (none / single / double), custom
//     colour and width, straight or grid-routed, with path labels.
//   - [toolkit.IsoZone]     — translucent grouping regions on the ground plane.
//   - [toolkit.IsoText]     — free-floating text annotations.
//   - [toolkit.IsoLayer]    — two ordered drawing planes ("infra" behind,
//     "monitor" in front) whose visibility the demo toggles.
//
// [ConvergedReplicas] additionally proves the collaborative store: two replicas
// on distinct sites make CONCURRENT edits (one moves and the other recolours the
// same node, a third-field merge; the second replica also adds a node and a
// zone), exchange operations both ways and converge to a byte-identical
// snapshot — and therefore to a byte-identical rendering.
package isodemo

import (
	"os"
	"path/filepath"

	"github.com/go-crdt/crdt"
	"github.com/go-widgets/toolkit"
)

// Canvas is the pixel size every scene renders at. It is wide and tall enough
// for the default 10x10 grid, centred by the widget, to sit inside the frame
// with room for the raised solids and their labels.
const (
	CanvasWidth  = 760
	CanvasHeight = 560
)

// Layer ids used across the showcase scene. "infra" is the back plane (lower
// order) and "monitor" the front plane (higher order), so a monitor-layer
// entity always draws over an infra-layer one whatever their isometric depth.
const (
	LayerInfra   = "infra"
	LayerMonitor = "monitor"
)

// Node ids in the showcase scene, exported so the test can assert on the exact
// entities it renders.
const (
	NodeWeb    = "web-01"
	NodeDB     = "db-01"
	NodeCloud  = "cloud-01"
	NodeRouter = "rtr-01"
	NodeSwitch = "sw-01"
	NodeStore  = "store-01"
	NodeProbe  = "mon-probe" // a bare cube (no icon) on the monitor layer
)

// Base colours of the showcase nodes. Each is fully opaque so a node's top face
// — which the isometric renderer shades at factor 1.0 — is drawn as exactly this
// colour, which is what the pixel-exact assertions in the test check.
var (
	ColWeb    = toolkit.RGB(60, 120, 220)
	ColDB     = toolkit.RGB(40, 170, 90)
	ColCloud  = toolkit.RGB(150, 80, 210)
	ColRouter = toolkit.RGB(230, 140, 30)
	ColSwitch = toolkit.RGB(30, 180, 180)
	ColStore  = toolkit.RGB(210, 60, 60)
	ColProbe  = toolkit.RGB(240, 200, 40)
)

// Connector colours used in the showcase (each fully opaque so a stroked pixel
// is exactly this colour).
var (
	ColSync  = toolkit.RGB(200, 40, 160)
	ColRoute = toolkit.RGB(40, 40, 60)
)

// Zone fill colours: the alpha is the fill opacity, so the ground grid still
// shows through the tint.
var (
	ColZoneCore = toolkit.RGBA{R: 60, G: 120, B: 220, A: 70}
	ColZoneEdge = toolkit.RGBA{R: 40, G: 170, B: 90, A: 70}
)

// Theme is the palette every scene renders with. Using one shared theme keeps
// the demo's rendering deterministic and lets the test predict theme-derived
// colours (surface, accent) exactly.
func Theme() *toolkit.Theme { return toolkit.DefaultLight() }

// BuildScene assembles the full showcase diagram from the public API and
// returns the widget, laid out on the default 10x10 grid. It is the single
// reusable construction path shared by the renderer, the assertions and the
// cmd binary — nothing about the scene lives in a main.
//
// The two named layers are created explicitly and made visible (a named layer
// is hidden until its Visible flag is set); the implicit default layer is not
// used, so [HideMonitoring] can drop a whole plane.
func BuildScene() *toolkit.IsoDiagram {
	doc := toolkit.NewIsoDoc()

	doc.PutLayer(toolkit.IsoLayer{ID: LayerInfra, Name: "Infrastructure", Visible: true, Order: 0})
	doc.PutLayer(toolkit.IsoLayer{ID: LayerMonitor, Name: "Monitoring", Visible: true, Order: 1})

	// Grouping zones on the infra plane's ground.
	doc.PutZone(toolkit.IsoZone{ID: "z-core", X: 1, Y: 1, W: 4, H: 3, Color: ColZoneCore, Label: "Core", Layer: LayerInfra})
	doc.PutZone(toolkit.IsoZone{ID: "z-edge", X: 6, Y: 5, W: 3, H: 3, Color: ColZoneEdge, Label: "Edge", Layer: LayerInfra})

	// Icon nodes: each resolves a different built-in architecture icon.
	doc.PutNode(toolkit.IsoNode{ID: NodeWeb, X: 2, Y: 2, Icon: "server", Color: ColWeb, Label: "web-01", Layer: LayerInfra})
	doc.PutNode(toolkit.IsoNode{ID: NodeDB, X: 4, Y: 1, Icon: "database", Color: ColDB, Label: "db", Layer: LayerInfra})
	doc.PutNode(toolkit.IsoNode{ID: NodeCloud, X: 1, Y: 6, Icon: "cloud", Color: ColCloud, Label: "cloud", Layer: LayerInfra})
	doc.PutNode(toolkit.IsoNode{ID: NodeRouter, X: 6, Y: 6, Icon: "router", Color: ColRouter, Label: "rtr", Layer: LayerInfra})
	doc.PutNode(toolkit.IsoNode{ID: NodeSwitch, X: 8, Y: 4, Icon: "switch", Color: ColSwitch, Label: "sw", Layer: LayerInfra})

	// Front-plane (monitor layer) entities: a storage icon and a bare cube probe.
	doc.PutNode(toolkit.IsoNode{ID: NodeStore, X: 7, Y: 7, Icon: "storage", Color: ColStore, Label: "store", Layer: LayerMonitor})
	doc.PutNode(toolkit.IsoNode{ID: NodeProbe, X: 3, Y: 7, Color: ColProbe, Label: "probe", Layer: LayerMonitor})

	// Rich connectors: every style / arrow / colour / width / routing combination.
	doc.PutConnector(toolkit.IsoConnector{ID: "c-sql", From: NodeWeb, To: NodeDB, Style: toolkit.IsoSolid, Arrow: toolkit.IsoArrowSingle, Label: "sql", Layer: LayerInfra})
	doc.PutConnector(toolkit.IsoConnector{ID: "c-sync", From: NodeWeb, To: NodeCloud, Style: toolkit.IsoDashed, Arrow: toolkit.IsoArrowDouble, Color: ColSync, Width: 3, Label: "sync", Layer: LayerInfra})
	doc.PutConnector(toolkit.IsoConnector{ID: "c-route", From: NodeDB, To: NodeRouter, Style: toolkit.IsoDotted, Arrow: toolkit.IsoArrowSingle, Color: ColRoute, Routed: true, Label: "route", Layer: LayerInfra})
	doc.PutConnector(toolkit.IsoConnector{ID: "c-uplink", From: NodeRouter, To: NodeSwitch, Style: toolkit.IsoSolid, Arrow: toolkit.IsoArrowSingle, Color: ColSwitch, Routed: true, Layer: LayerInfra})

	// Floating text annotations.
	doc.PutText(toolkit.IsoText{ID: "t-dc", X: 1, Y: 9, Text: "Datacenter A", Size: 2, Layer: LayerMonitor})
	doc.PutText(toolkit.IsoText{ID: "t-region", X: 7, Y: 2, Text: "region: eu-west", Size: 1, Layer: LayerInfra})

	d := toolkit.NewIsoDiagram(doc)
	d.SetBounds(toolkit.Rect{X: 0, Y: 0, W: CanvasWidth, H: CanvasHeight})
	return d
}

// HideMonitoring flips the monitor layer of d's document to invisible, so its
// storage node, probe cube and "Datacenter A" annotation stop rendering while
// the infra plane is untouched. It is the toggle the layer-visibility capture
// and its assertion use.
func HideMonitoring(d *toolkit.IsoDiagram) {
	l, _ := d.Doc().Layer(LayerMonitor)
	l.Visible = false
	d.Doc().PutLayer(l)
}

// SelectNodes makes ids the widget's multi-selection by appending a node
// reference for each onto the authoritative selection list, so every one of
// them draws its accent top-face outline. Appending to the observable list is
// the documented public equivalent of the widget's internal select — it is what
// a host binds a "N items selected" panel to.
func SelectNodes(d *toolkit.IsoDiagram, ids ...string) {
	list := d.SelectionList()
	list.Clear()
	for _, id := range ids {
		list.Append(toolkit.IsoEntityRef{Kind: toolkit.IsoEntityNode, ID: id})
	}
}

// Collaboration scenario identities.
const (
	collabSiteA crdt.SiteID = 1
	collabSiteB crdt.SiteID = 2

	// CollabMovedNode is the node both replicas edit concurrently: replica A
	// moves it, replica B recolours it. Different fields, so the field-wise LWW
	// merge keeps both edits.
	CollabMovedNode = "srv"
	// CollabAddedNode and CollabAddedZone are replica B's concurrent additions.
	CollabAddedNode = "cache"
	CollabAddedZone = "z-edge"
)

// The concurrent edits' target values, exported so the test asserts the exact
// merged state.
var (
	CollabMovedX, CollabMovedY = 4, 6
	CollabRecolor              = toolkit.RGB(255, 90, 180)
)

// seedCollab populates a fresh collaborative replica with the shared base
// diagram every replica starts from: the node the two replicas will edit
// concurrently, a second node, a connecting link and a grouping zone.
func seedCollab(d *toolkit.IsoCRDTDocument) {
	d.PutNode(toolkit.IsoNode{ID: CollabMovedNode, X: 2, Y: 2, Icon: "server", Color: ColWeb, Label: "srv", Layer: LayerInfra})
	d.PutNode(toolkit.IsoNode{ID: "db", X: 5, Y: 2, Icon: "database", Color: ColDB, Label: "db", Layer: LayerInfra})
	d.PutConnector(toolkit.IsoConnector{ID: "c0", From: CollabMovedNode, To: "db", Arrow: toolkit.IsoArrowSingle, Layer: LayerInfra})
	d.PutZone(toolkit.IsoZone{ID: "z-core", X: 1, Y: 1, W: 5, H: 3, Color: ColZoneCore, Label: "Core", Layer: LayerInfra})
	d.PutLayer(toolkit.IsoLayer{ID: LayerInfra, Name: "Infrastructure", Visible: true, Order: 0})
}

// apply integrates a peer's operation batches into d. The batches are produced
// by a sibling replica of the same document and are therefore always
// well-formed, so [toolkit.IsoCRDTDocument.Apply] cannot report an error here
// (it only rejects malformed input from an untrusted peer); the demo discards
// the impossible error deliberately. The test proves convergence independently
// by comparing snapshots.
func apply(d *toolkit.IsoCRDTDocument, batches []crdt.PartOps) {
	_ = d.Apply(batches...)
}

// ConvergedReplicas builds two collaborating replicas of one diagram, applies a
// pair of CONCURRENT edits — replica A moves the shared node while replica B
// recolours that same node AND adds a node and a zone — then exchanges
// operations both ways so each converges. It returns the two replicas after
// convergence; they hold byte-identical snapshots and therefore render
// identically.
//
// The edits are captured against the shared base version BEFORE either side
// integrates the other's operations, which is exactly the offline-then-sync
// pattern a real collaborative session produces.
func ConvergedReplicas() (a, b *toolkit.IsoCRDTDocument) {
	a = toolkit.NewIsoCRDTDocument(collabSiteA)
	seedCollab(a)

	// Replica B joins from A's snapshot, so both share the same base history.
	// The snapshot is self-produced and valid, so the load cannot fail.
	b, _ = toolkit.LoadIsoCRDTDocument(collabSiteB, a.Snapshot())

	baseA := a.Version()
	baseB := b.Version()

	// Replica A: move the shared node (writes only its x/y registers).
	srv, _ := a.Node(CollabMovedNode)
	srv.X, srv.Y = CollabMovedX, CollabMovedY
	a.PutNode(srv)

	// Replica B: recolour the SAME node (writes only its colour register), and
	// add a fresh node and zone.
	rc, _ := b.Node(CollabMovedNode)
	rc.Color = CollabRecolor
	b.PutNode(rc)
	b.PutNode(toolkit.IsoNode{ID: CollabAddedNode, X: 7, Y: 6, Icon: "storage", Color: ColStore, Label: "cache", Layer: LayerInfra})
	b.PutZone(toolkit.IsoZone{ID: CollabAddedZone, X: 6, Y: 5, W: 3, H: 3, Color: ColZoneEdge, Label: "Edge", Layer: LayerInfra})

	// Exchange the operations each side is missing, both directions.
	//
	// OpsSince refuses below what a replica has collected — a peer that far
	// behind has to be sent a snapshot rather than a difference. Nothing in
	// this demo collects, so neither call can refuse here.
	fromA, err := a.OpsSince(baseB)
	if err != nil {
		panic(err)
	}
	fromB, err := b.OpsSince(baseA)
	if err != nil {
		panic(err)
	}
	apply(b, fromA)
	apply(a, fromB)
	return a, b
}

// replicaDiagram wraps a collaborative document in a diagram sized like every
// other scene, ready to render.
func replicaDiagram(doc toolkit.IsoDocument) *toolkit.IsoDiagram {
	d := toolkit.NewIsoDiagram(doc)
	d.SetBounds(toolkit.Rect{X: 0, Y: 0, W: CanvasWidth, H: CanvasHeight})
	return d
}

// writeWidget renders w to a PNG and writes it into dir under name, returning
// the file's absolute-or-relative path. It surfaces both a render failure
// (non-positive dimensions) and a filesystem failure (an unwritable dir) to its
// caller.
func writeWidget(dir, name string, w toolkit.Widget, width, height int, theme *toolkit.Theme) (string, error) {
	data, err := toolkit.RenderPNG(w, width, height, theme)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Capture names one showcase image and the widget that renders it.
type Capture struct {
	// Name is the PNG file name written into the output directory.
	Name string
	// Widget is the scene rendered into that file.
	Widget toolkit.Widget
}

// Captures builds every showcase scene and returns them paired with the file
// name each is written to, in a stable order:
//
//   - scene.png           — the full diagram, both layers visible.
//   - layer-hidden.png    — the same diagram with the monitor layer hidden.
//   - selection.png       — the full diagram with a multi-node selection active.
//   - crdt-replica-a.png  — replica A after CRDT convergence.
//   - crdt-replica-b.png  — replica B after CRDT convergence (identical to A).
//
// It is the shared source of truth for both [Generate] (which writes the files)
// and the test (which renders and asserts on the same widgets).
func Captures() []Capture {
	full := BuildScene()

	hidden := BuildScene()
	HideMonitoring(hidden)

	selected := BuildScene()
	SelectNodes(selected, NodeWeb, NodeDB)

	a, b := ConvergedReplicas()

	return []Capture{
		{Name: "scene.png", Widget: full},
		{Name: "layer-hidden.png", Widget: hidden},
		{Name: "selection.png", Widget: selected},
		{Name: "crdt-replica-a.png", Widget: replicaDiagram(a)},
		{Name: "crdt-replica-b.png", Widget: replicaDiagram(b)},
	}
}

// Generate renders every showcase scene and writes it as a PNG into dir,
// returning the paths written in capture order. The first write to fail aborts
// and returns its error.
func Generate(dir string) ([]string, error) {
	theme := Theme()
	var paths []string
	for _, c := range Captures() {
		path, err := writeWidget(dir, c.Name, c.Widget, CanvasWidth, CanvasHeight, theme)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}
