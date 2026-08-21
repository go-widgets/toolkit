// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	stdcolor "image/color"
	"math"
	"sort"
	"strconv"

	"github.com/go-gfx/gfx/geometry"
	"github.com/go-gfx/gfx/iso"
	"github.com/go-gfx/gfx/raster"
	"github.com/go-gfx/gfx/vector"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// IsoMode selects what a left-drag on a node does.
type IsoMode int

const (
	// IsoModeSelect is the default: click selects a node / connector / zone / text,
	// drag moves the pressed node, zone or text (or resizes the selected zone by a
	// corner handle), and drag on empty ground pans the view.
	IsoModeSelect IsoMode = iota
	// IsoModeConnect turns a node drag into a connector gesture: press one node,
	// release on another, and a connector is created between them.
	IsoModeConnect
	// IsoModeZone turns a ground drag into a zone-creation gesture: press one
	// ground cell, drag to another and release to create a rectangular zone
	// spanning the two cells.
	IsoModeZone
	// IsoModeText turns a ground tap into a text-annotation gesture: tap a ground
	// cell to drop a new (empty-captioned) text annotation there, selected and
	// ready for [IsoDiagram.SetSelectedTextContent].
	IsoModeText
)

// isoGesture is the in-flight pointer gesture between an EventClick (press) and
// the matching EventMouseUp (release).
type isoGesture int

const (
	isoGestureNone isoGesture = iota
	isoGestureMove
	isoGestureConnect
	isoGesturePan
	// isoGestureZoneCreate is a rubber-band rectangle on the ground that becomes a
	// zone on release.
	isoGestureZoneCreate
	// isoGestureZoneMove drags a whole zone by a cell delta.
	isoGestureZoneMove
	// isoGestureZoneResize drags one corner handle of the selected zone, the
	// opposite corner staying fixed.
	isoGestureZoneResize
	// isoGestureTextMove drags a text annotation by a cell delta.
	isoGestureTextMove
	// isoGestureTextAdd drops a new text annotation on release (a ground tap in
	// [IsoModeText]).
	isoGestureTextAdd
	// isoGestureMarquee is a rubber-band selection rectangle on empty ground
	// (a Shift-drag): every entity whose projected box intersects the rectangle
	// is selected on release.
	isoGestureMarquee
	// isoGestureGroupMove drags the whole multi-selection (its nodes, zones and
	// texts) by one cell delta, as a single undoable move.
	isoGestureGroupMove
)

// isoSnapshot is a whole-document copy for the undo/redo stacks. The document is
// small and value-typed, so a snapshot is a cheap slice pair and undo is a plain
// replace — a scheme that works against any [IsoDocument], the bundled one or a
// future CRDT store.
type isoSnapshot struct {
	nodes  []IsoNode
	conns  []IsoConnector
	zones  []IsoZone
	texts  []IsoText
	layers []IsoLayer
}

// IsoZoomStep is the multiplicative zoom applied per wheel notch.
const IsoZoomStep = 1.1

// IsoMinTile / IsoMaxTile clamp the tile width so zoom cannot collapse the grid
// to a point or blow it up unboundedly.
const (
	IsoMinTile = 8.0
	IsoMaxTile = 512.0
)

// IsoDiagram is an editable isometric diagram widget — a FossFLOW-style node /
// connector editor. It owns an [IsoDocument] (nodes + connectors) and an
// isometric [iso.Projection]; it renders the ground grid, each node as a shaded
// isometric solid and each connector as a depth-sorted line by compositing an
// [iso.Scene] onto a pixel buffer, then blitting that buffer through the
// Painter. All projection and primitive drawing is delegated to
// github.com/go-gfx/gfx/iso — the widget adds only the document model, the
// interactions (place / drag / connect / select / delete / pan / zoom /
// context-menu / undo) and the accessibility tree.
type IsoDiagram struct {
	Base

	// Cols and Rows are the ground grid's extent in cells.
	Cols, Rows int
	// DefaultShape is the solid a newly placed node takes.
	DefaultShape IsoShape
	// Icons, when non-nil, is the per-widget icon registry an [IsoNode]'s Icon id
	// resolves through; nil uses the package-global [IsoDefaultIcons]. A host sets
	// this to give one diagram its own component library without touching the
	// shared default.
	Icons *IsoIconRegistry
	// Mode selects the left-drag behaviour (move vs connect).
	Mode IsoMode
	// AnimationPeriod is the wall-clock length of one full animation cycle, in the
	// same unit AnimationStep's dt is given in (seconds by convention). The global
	// animation phase advances by dt/AnimationPeriod each step and wraps at 1. A
	// non-positive value is treated as the default (isoDefaultAnimPeriod), so the
	// zero value animates at the default rate. It is plain view state (like pan or
	// zoom) and never enters the document.
	AnimationPeriod float64

	// OnSelect fires when the selected node changes; id is "" when the selection
	// is cleared.
	OnSelect func(id string)
	// OnSelectConnector fires when the selected connector changes; id is "" when
	// the connector selection is cleared. It mirrors OnSelect for the connector
	// half of the selection, so a host can drive a "connector properties" panel.
	OnSelectConnector func(id string)
	// OnSelectZone fires when the selected zone changes; id is "" when the zone
	// selection is cleared. It mirrors OnSelect for the zone third of the
	// selection, so a host can drive a "zone properties" panel.
	OnSelectZone func(id string)
	// OnSelectText fires when the selected text annotation changes; id is "" when
	// the text selection is cleared. It mirrors OnSelect for the text quarter of
	// the selection, so a host can drive a "text properties" panel.
	OnSelectText func(id string)
	// OnInvalidate, when set, is called whenever the widget's appearance changed
	// and it should be redrawn. A document edit (including one from another
	// collaborator, via the store's Subscribe) also triggers it.
	OnInvalidate func()

	doc  IsoDocument
	proj *iso.Projection
	menu *ContextMenu

	selected string
	// selConn is the selected connector's id, held in an mvvm.Observable so the
	// selection crosses the widget/host boundary the MVVM way: a host binds
	// [IsoDiagram.SelectedConnectorObservable] into a view model instead of the
	// widget copying the id out every frame. "" means no connector selected.
	selConn *mvvm.Observable[string]
	// selZone / selText are the selected zone / text-annotation ids, each held in
	// its own mvvm.Observable on the same model as selConn so all four selection
	// channels (node, connector, zone, text) are mutually exclusive and each
	// crosses the widget/host boundary the MVVM way. "" means nothing selected.
	selZone *mvvm.Observable[string]
	selText *mvvm.Observable[string]
	// selSet is the authoritative multi-selection: an observable ordered set of
	// entity references (nodes, connectors, zones and texts, mixed). The four
	// single-selection channels above are DERIVED reflections of it — each holds
	// the id of the last-selected entity of its kind — kept in sync by
	// [IsoDiagram.refreshMono], so the Vague-B/C mono-selection API keeps working
	// unchanged while the set drives multi-selection, group edits and the
	// clipboard. A host binds [IsoDiagram.SelectionList] to observe it the MVVM
	// way instead of polling.
	selSet *mvvm.ObservableList[IsoEntityRef]
	// clip is the widget-internal clipboard populated by Copy/Cut and consumed by
	// Paste/Duplicate — a private model, not the system clipboard.
	clip isoClip
	// placeIcon is the "armed" palette icon id: while non-empty, a plain tap on
	// empty ground drops a node carrying this icon instead of a bare default node
	// (the click-to-place companion of the drag-and-drop path). It is an
	// mvvm.Observable so the selection crosses the palette/diagram boundary the
	// MVVM way — a host binds an [IsoIconPalette]'s selected-icon observable into
	// it — instead of the widget copying an id out every frame. "" (the zero
	// value) disarms placement, so a diagram no host ever arms behaves exactly as
	// it did before the palette existed.
	placeIcon *mvvm.Observable[string]
	// viewRot is the view-plane rotation in 90° quarter-turns (0..3), a LOCAL
	// view state exactly like pan/zoom: it turns the whole rendered plane — grid,
	// nodes, connectors, zones, texts — about the grid's vertical centre axis, and
	// never enters the document or its CRDT, so two views (or two replicas) may
	// hold different rotations without diverging the model. It is an
	// mvvm.Observable so the rotation crosses the widget/host boundary the MVVM
	// way — a host binds [IsoDiagram.ViewRotationObservable] into a toolbar view
	// model — instead of the widget copying an int out every frame. The zero value
	// is the unrotated view, whose rendering is byte-identical to the pre-rotation
	// widget.
	viewRot *mvvm.Observable[int]
	seq     int // monotonic id source for placed nodes/connectors/zones/texts
	// animPhase is the global animation phase in [0, 1) shared by every animated
	// icon in the diagram. It is advanced only by [IsoDiagram.AnimationStep] from
	// the host's clock — never by an internal timer — so the rendering is a pure,
	// deterministic function of the accumulated dt. It is LOCAL view state: it
	// never enters the document or its CRDT, so two views of the same document may
	// hold different phases without diverging the model. The zero value renders the
	// rest frame, byte-identical to a diagram that has no animation at all.
	animPhase float64

	// interaction state
	gesture     isoGesture
	dragNode    string
	connectFrom string
	moved       bool
	pressX      int
	pressY      int
	lastX       int
	lastY       int
	curX        int
	curY        int
	// grab reference for a move gesture: the node's cell and the ground cell
	// under the press, so the node follows the pointer by a cell DELTA — grabbing
	// it anywhere (even on its raised top) and dropping without moving leaves it
	// put.
	grabNodeX int
	grabNodeY int
	grabCellX int
	grabCellY int

	// zone / text move + resize state. dragZone / dragText name the entity a
	// move gesture is dragging; grabEntX/grabEntY is that entity's own base cell
	// at grab time (the cell-DELTA anchor, like grabNodeX/Y for a node).
	dragZone string
	dragText string
	grabEntX int
	grabEntY int
	// resizeFixX/resizeFixY is the OPPOSITE grid vertex of the corner a zone
	// resize gesture drags, held fixed while the dragged corner follows the
	// pointer.
	resizeFixX int
	resizeFixY int

	// groupGrab records, for a group move, each moved entity's base cell at grab
	// time, so the whole selection follows the pointer by one cell delta from
	// grabCellX/grabCellY.
	groupGrab []isoGrabItem

	userMoved bool // set once the user pans/zooms so auto-centre stops

	undo []isoSnapshot
	redo []isoSnapshot

	unsub func()
}

// NewIsoDiagram returns an isometric diagram editing doc. A nil doc gets a fresh
// empty [IsoDoc]. The widget subscribes to the document so any edit — local or
// from a collaborating store — repaints via OnInvalidate.
func NewIsoDiagram(doc IsoDocument) *IsoDiagram {
	if doc == nil {
		doc = NewIsoDoc()
	}
	d := &IsoDiagram{
		Cols:      10,
		Rows:      10,
		doc:       doc,
		proj:      iso.NewDefault(geometry.Pt(0, 0)),
		menu:      NewContextMenu(NewMenu(nil)),
		selConn:   mvvm.NewObservable(""),
		selZone:   mvvm.NewObservable(""),
		selText:   mvvm.NewObservable(""),
		selSet:    mvvm.NewObservableList[IsoEntityRef](),
		placeIcon: mvvm.NewObservable(""),
		viewRot:   mvvm.NewObservable(0),
	}
	// A view-rotation change only re-orients the local view (like a pan or zoom);
	// it repaints and touches nothing in the document, so no edit is recorded.
	d.viewRot.Subscribe(func(int) { d.invalidate() })
	// Arming (or disarming) the click-to-place icon only changes the placement
	// cursor's intent; a repaint keeps any host-drawn affordance in step.
	d.placeIcon.Subscribe(func(string) { d.invalidate() })
	// Any change to the multi-selection repaints, so a purely additive selection
	// (one that does not move the last-of-kind the mono channels reflect) still
	// invalidates.
	d.selSet.SubscribeChanged(func() { d.invalidate() })
	// A connector-selection change repaints (highlight) and notifies the host,
	// exactly as a node selection does.
	d.selConn.Subscribe(func(id string) {
		if d.OnSelectConnector != nil {
			d.OnSelectConnector(id)
		}
		d.invalidate()
	})
	// The zone and text selections notify + repaint on the same model.
	d.selZone.Subscribe(func(id string) {
		if d.OnSelectZone != nil {
			d.OnSelectZone(id)
		}
		d.invalidate()
	})
	d.selText.Subscribe(func(id string) {
		if d.OnSelectText != nil {
			d.OnSelectText(id)
		}
		d.invalidate()
	})
	d.unsub = doc.Subscribe(func() { d.invalidate() })
	return d
}

// Doc returns the document the widget edits.
func (d *IsoDiagram) Doc() IsoDocument { return d.doc }

// Projection returns the live isometric projection (tile size + origin). Panning
// and zooming mutate it in place.
func (d *IsoDiagram) Projection() *iso.Projection { return d.proj }

// ContextMenu returns the widget's right-click menu overlay, so a host can style
// it or add items.
func (d *IsoDiagram) ContextMenu() *ContextMenu { return d.menu }

// Selected returns the selected node's ID, or "" when nothing is selected.
func (d *IsoDiagram) Selected() string { return d.selected }

// SelectedConnector returns the selected connector's ID, or "" when no connector
// is selected.
func (d *IsoDiagram) SelectedConnector() string { return d.selConn.Get() }

// SelectedConnectorObservable exposes the connector-selection property so a host
// can bind it into an MVVM view model (e.g. a connector-style inspector) that
// stays in sync with clicks in the diagram, rather than polling
// [IsoDiagram.SelectedConnector] every frame.
func (d *IsoDiagram) SelectedConnectorObservable() *mvvm.Observable[string] { return d.selConn }

// SelectConnector selects the connector with the given id (or clears the
// connector selection when id is ""). Selecting a connector clears any node,
// zone or text selection so only one entity ever highlights at once.
func (d *IsoDiagram) SelectConnector(id string) {
	if id == "" {
		d.selRemoveKind(IsoEntityConnector)
		return
	}
	d.selReplace(IsoEntityRef{Kind: IsoEntityConnector, ID: id})
}

// Close unsubscribes the widget from its document. It is optional: a diagram
// that outlives its use leaks only one closure, but a host churning through
// documents should call it.
func (d *IsoDiagram) Close() {
	if d.unsub != nil {
		d.unsub()
		d.unsub = nil
	}
}

// invalidate calls OnInvalidate when one is set.
func (d *IsoDiagram) invalidate() {
	if d.OnInvalidate != nil {
		d.OnInvalidate()
	}
}

// isoDefaultAnimPeriod is the fallback [IsoDiagram.AnimationPeriod]: the
// wall-clock length of one animation cycle when the field is left non-positive.
const isoDefaultAnimPeriod = 2.0

// AnimationStep advances the diagram's global animation phase by dt (elapsed
// wall-clock time, in AnimationPeriod's unit — seconds by convention) and, when
// the document holds at least one node carrying an animated icon, requests a
// repaint via OnInvalidate. The host owns the clock and calls this each frame;
// the widget keeps no timer, so the rendering stays a deterministic function of
// the accumulated dt — two diagrams stepped by the same dt sequence render
// identically.
//
// The phase wraps into [0, 1) after each step, so a long dt (or many steps) folds
// through the cycle correctly. A diagram whose nodes carry only still icons (or no
// icon) still advances its phase but never invalidates — nothing on screen moves —
// and a diagram that is never stepped renders the rest frame, byte-identical to
// the pre-animation widget.
func (d *IsoDiagram) AnimationStep(dt float64) {
	period := d.AnimationPeriod
	if period <= 0 {
		period = isoDefaultAnimPeriod
	}
	p := d.animPhase + dt/period
	// A non-finite step (a host handing a NaN or ±Inf dt) would fold to a NaN
	// phase, which flows into every animated icon's trig and out as NaN shape
	// coordinates — a blank or garbage frame. Leave the phase (and the frame)
	// untouched in that case; a finite dt is unaffected.
	if math.IsNaN(p) || math.IsInf(p, 0) {
		return
	}
	d.animPhase = p - math.Floor(p)
	if d.hasAnimatedNode() {
		d.invalidate()
	}
}

// AnimationPhase is the diagram's current global animation phase in [0, 1). A
// host may read it to drive a synchronised affordance, or to snapshot/restore an
// animation for a deterministic test.
func (d *IsoDiagram) AnimationPhase() float64 { return d.animPhase }

// hasAnimatedNode reports whether any node's resolved icon is an
// [IsoAnimatedIcon], i.e. whether a phase change would change the rendering.
func (d *IsoDiagram) hasAnimatedNode() bool {
	reg := d.iconRegistry()
	for _, n := range d.doc.Nodes() {
		if n.Icon == "" {
			continue
		}
		icon, _ := reg.Resolve(n.Icon)
		if _, ok := icon.(IsoAnimatedIcon); ok {
			return true
		}
	}
	return false
}

// renderIconAtPhase evaluates icon at the diagram's current animation phase: an
// [IsoAnimatedIcon] is rendered at that phase, any other icon by its static
// [IsoIcon.Render] (the phase has no effect). This is the single seam the phase
// enters the draw path, so a still icon — and any diagram whose phase is 0 —
// renders exactly as before.
func (d *IsoDiagram) renderIconAtPhase(icon IsoIcon, x, y int, base stdcolor.RGBA) IsoIconDrawing {
	// A nil icon can reach here when a host registers a nil under an id (or nils
	// out the exported IsoFallbackIcon that an unknown id resolves to). Rendering
	// it would dereference a nil interface and take the whole widget down; treat it
	// as an icon that contributes nothing instead. addNodeShapes turns that empty
	// drawing back into the node's plain solid, so the node still shows.
	if icon == nil {
		return IsoIconDrawing{}
	}
	if a, ok := icon.(IsoAnimatedIcon); ok {
		return a.RenderAt(x, y, base, d.animPhase)
	}
	return icon.Render(x, y, base)
}

// SetBounds positions the widget and, until the user first pans or zooms,
// re-centres the grid within the new bounds so the diagram is visible without
// any host setup.
func (d *IsoDiagram) SetBounds(r Rect) {
	d.Base.SetBounds(r)
	if !d.userMoved {
		d.center(r)
	}
}

// center places the grid's midpoint at the middle of r.
func (d *IsoDiagram) center(r Rect) {
	mid := float64(d.Cols+d.Rows) / 2
	d.proj.Origin = geometry.Pt(
		float64(r.W)/2,
		float64(r.H)/2-mid*(d.proj.TileH/2),
	)
}

// --- colour / geometry helpers ------------------------------------------

// stdColor converts a toolkit RGBA to the image/color.RGBA the iso primitives
// take.
func stdColor(c RGBA) stdcolor.RGBA { return stdcolor.RGBA{R: c.R, G: c.G, B: c.B, A: c.A} }

// resolveColor is the node's base colour, defaulting to the theme accent when
// the node left its colour unset (A==0).
func (d *IsoDiagram) resolveColor(n IsoNode, theme *Theme) stdcolor.RGBA {
	if n.Color.A == 0 {
		return stdColor(theme.Accent)
	}
	return stdColor(n.Color)
}

// nodeHeight is a node's extent along +Z in grid units.
func (d *IsoDiagram) nodeHeight(n IsoNode) float64 {
	if n.Shape == IsoBox {
		return 2
	}
	return 1
}

// iconRegistry is the registry a node's Icon id resolves through: the widget's
// own override when set, otherwise the package-global default.
func (d *IsoDiagram) iconRegistry() *IsoIconRegistry {
	if d.Icons != nil {
		return d.Icons
	}
	return IsoDefaultIcons()
}

// nodeSolid builds the iso solid for a node in world space, coloured c.
func (d *IsoDiagram) nodeSolid(n IsoNode, c stdcolor.RGBA) iso.Shape {
	pos := iso.V(float64(n.X), float64(n.Y), 0)
	switch n.Shape {
	case IsoBox:
		return iso.Brick{Pos: pos, Dim: iso.Dimension{W: 1, H: 1, D: 2}, Color: c}
	case IsoPyramid:
		return iso.Pyramid{Pos: pos, Dim: iso.Dimension{W: 1, H: 1, D: 1}, Color: c}
	default:
		return iso.Cube{Pos: pos, Size: 1, Color: c}
	}
}

// nodeAnchor is the world point a connector attaches to and a label sits above:
// the centre of the node's top.
func (d *IsoDiagram) nodeAnchor(n IsoNode) iso.Vec3 {
	return iso.V(float64(n.X)+0.5, float64(n.Y)+0.5, d.nodeHeight(n))
}

// topFacePoly returns the four projected corners of a node's top face, in
// buffer-local coordinates — the outline the selection highlight strokes.
func (d *IsoDiagram) topFacePoly(n IsoNode) []geometry.Point {
	z := d.nodeHeight(n)
	x0, y0 := float64(n.X), float64(n.Y)
	x1, y1 := x0+1, y0+1
	return []geometry.Point{
		d.project(iso.V(x0, y0, z)),
		d.project(iso.V(x1, y0, z)),
		d.project(iso.V(x1, y1, z)),
		d.project(iso.V(x0, y1, z)),
	}
}

// nodeViewCorner is a node footprint's minimum corner in VIEW space (after the
// current view rotation). The silhouette [IsoDiagram.pickPolys] tests must be
// built there, not in model space, so their two visible sides are the ones the
// projection actually shows front — the world +X and +Y faces are always the
// screen-facing pair whatever the rotation.
func (d *IsoDiagram) nodeViewCorner(n IsoNode) (float64, float64) {
	pos, _ := d.rotBox(iso.V(float64(n.X), float64(n.Y), 0), iso.Dimension{W: 1, H: 1, D: d.nodeHeight(n)})
	return pos.X, pos.Y
}

// pickPolys returns the projected silhouette polygons of a node (buffer-local),
// whose union is the whole area a pointer can hit it on. For a box that is the
// top, the two visible sides and the bottom (which together tile the hexagon
// outline); for a pyramid the base square and the two visible triangles.
func (d *IsoDiagram) pickPolys(n IsoNode) [][]geometry.Point {
	z := d.nodeHeight(n)
	// Build the silhouette in VIEW space (the rotated footprint corner), projected
	// by the raw projection, so the two visible sides are the world +X / +Y faces
	// the projection always shows front — for any view rotation.
	x0, y0 := d.nodeViewCorner(n)
	x1, y1 := x0+1, y0+1
	pj := d.proj.Project
	// base square, shared by every shape
	base := []geometry.Point{
		pj(iso.V(x0, y0, 0)), pj(iso.V(x1, y0, 0)),
		pj(iso.V(x1, y1, 0)), pj(iso.V(x0, y1, 0)),
	}
	if n.Shape == IsoPyramid {
		apex := pj(iso.V(x0+0.5, y0+0.5, z))
		return [][]geometry.Point{
			base,
			{pj(iso.V(x1, y0, 0)), pj(iso.V(x1, y1, 0)), apex}, // +X triangle
			{pj(iso.V(x0, y1, 0)), pj(iso.V(x1, y1, 0)), apex}, // +Y triangle
		}
	}
	return [][]geometry.Point{
		base,
		{pj(iso.V(x0, y0, z)), pj(iso.V(x1, y0, z)), pj(iso.V(x1, y1, z)), pj(iso.V(x0, y1, z))}, // top
		{pj(iso.V(x1, y0, 0)), pj(iso.V(x1, y1, 0)), pj(iso.V(x1, y1, z)), pj(iso.V(x1, y0, z))}, // +X side
		{pj(iso.V(x0, y1, 0)), pj(iso.V(x1, y1, 0)), pj(iso.V(x1, y1, z)), pj(iso.V(x0, y1, z))}, // +Y side
	}
}

// pointInPoly reports whether (px, py) lies inside the polygon by the
// even-odd crossing-number rule.
func pointInPoly(px, py float64, poly []geometry.Point) bool {
	in := false
	n := len(poly)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		yi, yj := poly[i].Y, poly[j].Y
		if (yi > py) != (yj > py) {
			xcross := poly[i].X + (py-yi)/(yj-yi)*(poly[j].X-poly[i].X)
			if px < xcross {
				in = !in
			}
		}
	}
	return in
}

// nodeAtLocal returns the ID of the node whose silhouette contains widget-local
// (x, y), testing nearest to the viewer first so the front-most of overlapping
// nodes wins.
func (d *IsoDiagram) nodeAtLocal(x, y int) (string, bool) {
	nodes := d.doc.Nodes()
	sort.SliceStable(nodes, func(i, j int) bool {
		return d.depth(iso.V(float64(nodes[i].X), float64(nodes[i].Y), 0)) >
			d.depth(iso.V(float64(nodes[j].X), float64(nodes[j].Y), 0))
	})
	fx, fy := float64(x), float64(y)
	for _, n := range nodes {
		if !d.pickable(n.Layer) {
			continue
		}
		for _, poly := range d.pickPolys(n) {
			if pointInPoly(fx, fy, poly) {
				return n.ID, true
			}
		}
	}
	return "", false
}

// cellAtLocal maps widget-local (x, y) to the ground grid cell under it, by
// unprojecting onto the z=0 plane and flooring.
func (d *IsoDiagram) cellAtLocal(x, y int) (gx, gy int) {
	w := d.rotInv(d.proj.Unproject(geometry.Pt(float64(x), float64(y)), 0))
	return int(math.Floor(w.X)), int(math.Floor(w.Y))
}

// --- connector geometry -------------------------------------------------

// isoConnectorWidth is the default connector stroke width in logical pixels —
// the historical literal, so an unset connector at metric scale 1 strokes
// exactly 2 device pixels, unchanged from before styles existed.
const isoConnectorWidth = 2

// isoConnectorHit is how near (logical pixels) a click must fall to a connector's
// projected path to pick it.
const isoConnectorHit = 6

// resolveConnColor is a connector's stroke colour as the iso primitives take it,
// defaulting to the theme link colour (OnSurface) when the connector left its
// colour unset (A==0) — the exact colour and defaulting rule the bare connector
// always used.
func (d *IsoDiagram) resolveConnColor(c IsoConnector, theme *Theme) stdcolor.RGBA {
	return stdColor(d.connInk(c, theme))
}

// connInk is a connector's stroke colour as a toolkit RGBA (for the painter-space
// decorations: arrow heads, label, highlight), defaulting to the theme link
// colour when unset.
func (d *IsoDiagram) connInk(c IsoConnector, theme *Theme) RGBA {
	if c.Color.A == 0 {
		return theme.OnSurface
	}
	return c.Color
}

// connWidth is a connector's stroke width in device pixels: its own Width (or the
// default) routed through the toolkit HiDPI/density scale. At the default scale
// and default width this is 2, matching the legacy literal exactly.
func (d *IsoDiagram) connWidth(c IsoConnector) float64 {
	w := c.Width
	if w <= 0 {
		w = isoConnectorWidth
	}
	return float64(scaled(w))
}

// faceAnchor returns the CENTRE of node n's visible lateral face toward the
// neighbour direction (dx, dy) (the vector from n toward its neighbour, in grid
// cells): the midpoint of n's +X, -X, +Y or -Y footprint edge, raised to half the
// solid's height so the point lands on the vertical middle of that face rather
// than on the ground. For the vertical-sided solids (cube and box) that is the
// exact geometric centre of the rectangular side face; a routed connector so
// leaves and enters each node at the middle of the face it presents to its
// neighbour, symmetric at both ends. It is a MODEL-space point — the caller
// projects it through the view rotation.
func (d *IsoDiagram) faceAnchor(n IsoNode, dx, dy float64) iso.Vec3 {
	x0, y0 := float64(n.X), float64(n.Y)
	zc := d.nodeHeight(n) / 2 // vertical centre of the lateral face
	if math.Abs(dx) >= math.Abs(dy) {
		if dx >= 0 {
			return iso.V(x0+1, y0+0.5, zc) // +X face centre
		}
		return iso.V(x0, y0+0.5, zc) // -X face centre
	}
	if dy >= 0 {
		return iso.V(x0+0.5, y0+1, zc) // +Y face centre
	}
	return iso.V(x0+0.5, y0, zc) // -Y face centre
}

// dedupPath drops consecutive duplicate points so a returned polyline has no
// zero-length segment (an already-aligned route collapses its elbow).
func dedupPath(pts []iso.Vec3) []iso.Vec3 {
	out := make([]iso.Vec3, 0, len(pts))
	for _, p := range pts {
		if len(out) == 0 || p != out[len(out)-1] {
			out = append(out, p)
		}
	}
	return out
}

// routeTiles computes a routed connector's grid-orthogonal path from a's face
// centre to b's face centre with one elbow, each face chosen by the direction to
// the other node. The connector leaves each ±X face horizontally and each ±Y face
// vertically, so the elbow lies on the grid axes; the elbow sits at the leaving
// anchor's height, so when both nodes are the same height the whole L runs level
// through their face centres (and the terminal segment still meets each face
// centre exactly whatever the heights).
func (d *IsoDiagram) routeTiles(a, b IsoNode) []iso.Vec3 {
	acx, acy := float64(a.X)+0.5, float64(a.Y)+0.5
	bcx, bcy := float64(b.X)+0.5, float64(b.Y)+0.5
	pa := d.faceAnchor(a, bcx-acx, bcy-acy)
	pb := d.faceAnchor(b, acx-bcx, acy-bcy)
	var corner iso.Vec3
	if pa.X != acx { // a left on a ±X face → move along X first
		corner = iso.V(pb.X, pa.Y, pa.Z)
	} else { // a left on a ±Y face → move along Y first
		corner = iso.V(pa.X, pb.Y, pa.Z)
	}
	return dedupPath([]iso.Vec3{pa, corner, pb})
}

// connectorPath returns the world-space polyline a connector draws along and
// whether both its endpoints exist. An unrouted connector is the single straight
// segment between the two node top anchors (the legacy path); a routed one is the
// grid-orthogonal ground route.
func (d *IsoDiagram) connectorPath(c IsoConnector) ([]iso.Vec3, bool) {
	a, ok1 := d.doc.Node(c.From)
	b, ok2 := d.doc.Node(c.To)
	if !ok1 || !ok2 {
		return nil, false
	}
	if c.Routed {
		return d.routeTiles(a, b), true
	}
	return []iso.Vec3{d.nodeAnchor(a), d.nodeAnchor(b)}, true
}

// projPath projects a world polyline to widget-buffer-local screen points.
func (d *IsoDiagram) projPath(pts []iso.Vec3) []geometry.Point {
	out := make([]geometry.Point, len(pts))
	for i, p := range pts {
		out[i] = d.project(p)
	}
	return out
}

// lerp3 linearly interpolates between two world points.
func lerp3(a, b iso.Vec3, t float64) iso.Vec3 {
	return iso.V(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t, a.Z+(b.Z-a.Z)*t)
}

// dashSpec is the on/off screen-pixel pattern for a stroke style: dash length and
// gap length, and whether the style is dashed at all. A solid style is never
// subdivided.
func dashSpec(style IsoConnectorStyle) (dash, gap float64, dashed bool) {
	switch style {
	case IsoDashed:
		return float64(scaled(9)), float64(scaled(6)), true
	case IsoDotted:
		return float64(scaled(2)), float64(scaled(5)), true
	default:
		return 0, 0, false
	}
}

// addConnectorSegment adds the iso.Line(s) drawing one world segment a->b of a
// connector in colour col at width w and the given style: a single line for
// solid, or a run of dash lines spaced along the segment's SCREEN length for
// dashed/dotted (so the dash cadence is uniform on screen whatever the segment's
// world direction).
func (d *IsoDiagram) addConnectorSegment(sc *iso.Scene, a, b iso.Vec3, col stdcolor.RGBA, w float64, style IsoConnectorStyle) {
	// Rotate the segment's world endpoints into view space up front, so both the
	// solid line and the dash sub-segments (and their screen-length projection)
	// are computed in the frame the scene's raw projection draws through.
	a, b = d.rotFwd(a), d.rotFwd(b)
	dash, gap, dashed := dashSpec(style)
	if !dashed {
		sc.Add(iso.Line{From: a, To: b, Color: col, Width: w})
		return
	}
	pa, pb := d.proj.Project(a), d.proj.Project(b)
	length := math.Hypot(pb.X-pa.X, pb.Y-pa.Y)
	if length == 0 {
		return
	}
	period := dash + gap
	for s := 0.0; s < length; s += period {
		e := s + dash
		if e > length {
			e = length
		}
		sc.Add(iso.Line{From: lerp3(a, b, s/length), To: lerp3(a, b, e/length), Color: col, Width: w})
	}
}

// distToSegment returns the SQUARED distance from point (px,py) to the segment
// a-b in screen space, clamping the projection to the segment's ends.
func distToSegment(px, py float64, a, b geometry.Point) float64 {
	vx, vy := b.X-a.X, b.Y-a.Y
	wx, wy := px-a.X, py-a.Y
	t := 0.0
	if denom := vx*vx + vy*vy; denom > 0 {
		t = (wx*vx + wy*vy) / denom
		if t < 0 {
			t = 0
		} else if t > 1 {
			t = 1
		}
	}
	dx, dy := px-(a.X+t*vx), py-(a.Y+t*vy)
	return dx*dx + dy*dy
}

// polylineMidpoint returns the "middle" of a projected path: for an even number
// of points the midpoint of the two central vertices, for an odd number the
// central vertex itself (the elbow of an L-route). It is a cheap stand-in for the
// arc-length centre that lands on or beside the short (2- or 3-point) routes this
// widget produces.
func polylineMidpoint(pts []geometry.Point) geometry.Point {
	m := len(pts) / 2
	if len(pts)%2 == 1 {
		return pts[m]
	}
	a, b := pts[m-1], pts[m]
	return geometry.Pt((a.X+b.X)/2, (a.Y+b.Y)/2)
}

// arrowBarbs returns the two barb endpoints of an arrow head whose tip is at
// (tx,ty) and whose incoming segment comes from (fx,fy): the barbs splay back
// from the tip by isoArrowSpread on each side of the segment. A zero-length
// segment (tip==from) has no direction and returns the tip for both barbs, so the
// caller draws nothing visible.
func arrowBarbs(tx, ty, fx, fy int) (lx, ly, rx, ry int) {
	dx, dy := float64(tx-fx), float64(ty-fy)
	n := math.Hypot(dx, dy)
	if n == 0 {
		return tx, ty, tx, ty
	}
	dx, dy = dx/n, dy/n
	l := float64(scaled(isoArrowLen))
	cs, sn := math.Cos(isoArrowSpread), math.Sin(isoArrowSpread)
	// back = (-dx,-dy), rotated +spread (left) and -spread (right).
	lvx, lvy := -dx*cs+dy*sn, -dx*sn-dy*cs
	rvx, rvy := -dx*cs-dy*sn, dx*sn-dy*cs
	return tx + iround(l*lvx), ty + iround(l*lvy), tx + iround(l*rvx), ty + iround(l*rvy)
}

// isoArrowLen / isoArrowSpread size the arrow heads: barb length in logical
// pixels and the half-angle each barb opens from the segment.
const isoArrowLen = 10

var isoArrowSpread = 30.0 * math.Pi / 180

// --- rendering ----------------------------------------------------------

// scene assembles the depth-sorted isometric scene: a solid per node, then a
// line per connector whose endpoints both exist. The ground grid is NOT part of
// the scene — it is painted as a floor pass before the scene renders (see
// [IsoDiagram.drawGrid]) so opaque solids occlude the cells they cover.
func (d *IsoDiagram) scene(theme *Theme) *iso.Scene {
	sc := iso.NewScene(d.proj)
	for _, n := range d.doc.Nodes() {
		d.addNodeShapes(sc, theme, n)
	}
	for _, c := range d.doc.Connectors() {
		d.addConnectorShapes(sc, theme, c)
	}
	return sc
}

// drawGrid strokes the ground grid straight into img in the theme's border
// colour — the FLOOR pass, painted BEFORE the depth-sorted node/connector scene,
// exactly where the zone floor is painted.
//
// The grid used to be added to the depth-sorted scene as one iso.Line per line.
// The painter's algorithm sorts a shape by a single key (a line by its midpoint),
// so a long grid line spanning the board — midpoint at the board centre — could
// sort in FRONT of a nearer solid and paint over its face, letting the grid show
// through solids that should occlude it. Painting the grid as an opaque floor,
// then compositing the solids back-to-front over it, occludes every cell a solid
// covers.
//
// This is geometrically exact for a ground-plane (z=0) grid under solids that
// stand on the ground: a grid pixel a solid's screen footprint covers is
// genuinely under or behind that solid, and a grid line on a free tile — even one
// in front of a solid — keeps every pixel the solid does not cover, because such
// a segment never projects onto the solid's body. Connectors STAY in the scene,
// so one routed behind a solid is still occluded and one in front still shows.
//
// SPRITE nodes need one extra step. A sprite icon adds NO solid to the
// depth-sorted scene (see addNodeShapes): it is a straight-alpha billboard
// blitted flat over the rendered scene (drawSprites). Because it contributes no
// body, nothing composites over the cell it stands on, so the floor-pass grid
// keeps showing through the sprite's transparent pixels — grid lines run across
// the icon's own tile. To fix that leak WITHOUT touching the depth sort (so
// connectors and primitive solids order exactly as before), the four grid
// sub-segments that bound a sprite node's footprint cell are simply not stroked:
// the tile the sprite plants on then carries no grid line at all, matching the
// clean look a solid gives its own cell. This is done in MODEL space — the grid
// lines and the node footprints are both model-space, and the current view
// rotation is applied uniformly to both at projection (d.project → rotFwd) — so a
// skipped cell stays exactly under its sprite in every orientation, no view-space
// footprint maths required.
//
// PRIMITIVE (non-sprite) nodes are left untouched: their opaque solid already
// occludes the cell, and skipping their edges would erase the outer half of the
// near-edge grid lines a primitive currently shows, so a primitive-only diagram
// stays byte-identical to the pre-fix render (spriteFootprints is empty → every
// line strokes as one full run, exactly as before).
//
// Each run is stroked exactly as iso.Scene strokes an iso.Line (round cap,
// width 1, flat composite through the same vector layer), so an unoccluded grid
// is pixel-identical to the pre-fix scene grid, and the grid still turns with the
// view (rotFwd) and rides pan/zoom through d.project.
func (d *IsoDiagram) drawGrid(img *raster.Image, theme *Theme) {
	grid := stdColor(theme.Border)
	rz := &vector.Rasterizer{}
	stroke := func(a, b iso.Vec3) {
		pa, pb := d.project(a), d.project(b)
		path := vector.NewPath().MoveTo(pa.X, pa.Y).LineTo(pb.X, pb.Y)
		cov, ox, oy, w, h, ok := rz.Stroke(path, 1, img.W, img.H)
		if !ok {
			return
		}
		vector.Composite(img, cov, ox, oy, w, h, vector.SolidPaint{Color: grid})
	}
	occ := d.spriteFootprints(theme)
	// Vertical lines x=i: the unit sub-segment [j, j+1] is the shared edge of cells
	// (i-1, j) and (i, j); drop it when either is a sprite footprint so no grid line
	// touches a sprite's tile.
	for i := 0; i <= d.Cols; i++ {
		gridLineRuns(d.Rows, func(j int) bool {
			return occ[[2]int{i - 1, j}] || occ[[2]int{i, j}]
		}, func(lo, hi int) {
			stroke(iso.V(float64(i), float64(lo), 0), iso.V(float64(i), float64(hi), 0))
		})
	}
	// Horizontal lines y=j: the unit sub-segment [i, i+1] is the shared edge of
	// cells (i, j-1) and (i, j).
	for j := 0; j <= d.Rows; j++ {
		gridLineRuns(d.Cols, func(i int) bool {
			return occ[[2]int{i, j - 1}] || occ[[2]int{i, j}]
		}, func(lo, hi int) {
			stroke(iso.V(float64(lo), float64(j), 0), iso.V(float64(hi), float64(j), 0))
		})
	}
}

// gridLineRuns walks a grid line's n unit sub-segments (indexed 0..n-1) and calls
// emit(lo, hi) once per maximal run of consecutive segments that skip reports
// false for, so each run is stroked as a single polyline. When skip is never true
// the whole line emits as one run emit(0, n) — identical to stroking it end to
// end — which is why a diagram with no sprite footprints renders the grid exactly
// as the pre-fix single-stroke code did. A run is never emitted for a line with
// no unit segments (n == 0).
func gridLineRuns(n int, skip func(int) bool, emit func(lo, hi int)) {
	for lo := 0; lo < n; {
		if skip(lo) {
			lo++
			continue
		}
		hi := lo + 1
		for hi < n && !skip(hi) {
			hi++
		}
		emit(lo, hi)
		lo = hi
	}
}

// spriteFootprints is the set of model grid cells occupied by a node that blits a
// sprite this frame — an icon node, on a visible layer, whose resolved icon's
// drawing carries a Sprite (a pure sprite icon or a hybrid that also adds
// shapes). These are exactly the cells drawGrid must leave gridless, since a
// billboard sprite adds no solid to occlude the floor grid under it. A
// primitive-only node (no Sprite) is never included, so the common case yields an
// empty set and the grid strokes unchanged. Cell membership is evaluated with the
// same registry, phase and layer-visibility rules as drawSpritesWhere, so the
// skipped cells match the sprites actually blitted.
func (d *IsoDiagram) spriteFootprints(theme *Theme) map[[2]int]bool {
	cells := map[[2]int]bool{}
	for _, n := range d.doc.Nodes() {
		if n.Icon == "" || !d.layerVisible(n.Layer) {
			continue
		}
		icon, _ := d.iconRegistry().Resolve(n.Icon)
		if d.renderIconAtPhase(icon, n.X, n.Y, d.resolveColor(n, theme)).Sprite != nil {
			cells[[2]int{n.X, n.Y}] = true
		}
	}
	return cells
}

// addNodeShapes adds a node's depth-sortable solids to sc: the bare shape solid
// when the node has no icon (the icon system is purely additive), otherwise the
// resolved icon's primitives. A sprite icon adds no shapes here and is blitted
// after the scene renders (see drawSprites).
func (d *IsoDiagram) addNodeShapes(sc *iso.Scene, theme *Theme, n IsoNode) {
	base := d.resolveColor(n, theme)
	if n.Icon == "" {
		sc.Add(d.rotShape(d.nodeSolid(n, base)))
		return
	}
	icon, _ := d.iconRegistry().Resolve(n.Icon)
	// A resolved-nil icon (a host registered nil, or nilled the fallback an unknown
	// id resolves to) would dereference a nil interface downstream: draw the bare
	// solid instead, so the node is still visible and nothing crashes.
	if icon == nil {
		sc.Add(d.rotShape(d.nodeSolid(n, base)))
		return
	}
	for _, sh := range d.renderIconAtPhase(icon, n.X, n.Y, base).Shapes {
		sc.Add(d.rotShape(sh))
	}
}

// addConnectorShapes adds a connector's stroked path segments to sc, or nothing
// when an endpoint is missing.
func (d *IsoDiagram) addConnectorShapes(sc *iso.Scene, theme *Theme, c IsoConnector) {
	pts, ok := d.connectorPath(c)
	if !ok {
		return
	}
	col := d.resolveConnColor(c, theme)
	w := d.connWidth(c)
	for i := 1; i < len(pts); i++ {
		d.addConnectorSegment(sc, pts[i-1], pts[i], col, w, c.Style)
	}
}

// spriteRect is the widget-local pixel rectangle a billboarded sprite icon fills
// for node n: a TileW-sized square standing on the projected ground centre of
// the node's cell, i.e. its bottom-centre sits on that ground point so the
// sprite reads as planted on the tile.
func (d *IsoDiagram) spriteRect(n IsoNode) Rect {
	p := d.project(iso.V(float64(n.X)+0.5, float64(n.Y)+0.5, 0))
	s := iround(d.proj.TileW)
	return Rect{X: iround(p.X) - s/2, Y: iround(p.Y) - s, W: s, H: s}
}

// drawSprites blits the sprite of every icon node whose icon contributes one,
// over the already-rendered scene (into the same buffer, so it is captured by
// the single blit). Primitive-only icons contribute no sprite and are skipped.
func (d *IsoDiagram) drawSprites(img *raster.Image, theme *Theme) {
	d.drawSpritesWhere(img, theme, func(IsoNode) bool { return true })
}

// drawSpritesWhere blits the sprite of every icon node the predicate keep
// accepts (see [IsoDiagram.drawSprites]); the layered render path passes a
// per-layer-order predicate.
func (d *IsoDiagram) drawSpritesWhere(img *raster.Image, theme *Theme, keep func(IsoNode) bool) {
	for _, n := range d.doc.Nodes() {
		if !keep(n) || n.Icon == "" {
			continue
		}
		icon, _ := d.iconRegistry().Resolve(n.Icon)
		if sprite := d.renderIconAtPhase(icon, n.X, n.Y, d.resolveColor(n, theme)).Sprite; sprite != nil {
			blitSprite(img, d.spriteRect(n), sprite)
		}
	}
}

// fillRaster paints every pixel of img the flat colour c.
func fillRaster(img *raster.Image, c RGBA) {
	for i := 0; i+3 < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
}

// Draw renders the diagram: the isometric scene is composited into an opaque
// buffer the size of the widget, blitted into place, then labels, the
// rubber-band connector preview, the selection outline and the context menu are
// drawn over it in painter space.
func (d *IsoDiagram) Draw(p painter.Painter, theme *Theme) {
	b := d.Bounds()
	if b.W <= 0 || b.H <= 0 {
		return
	}
	img := raster.New(b.W, b.H)
	fillRaster(img, theme.Surface)
	// The blit pipeline (zone floor, ground-grid floor, depth-sorted
	// node/connector scene, sprite overlay) is composited by layer order: see
	// drawContent. With a single visible layer — every pre-layers document — this
	// is one legacy pass.
	d.drawContent(img, theme)
	blitImage(p, b, b, img.Pix, b.W, b.H)

	// rubber-band preview while connecting
	if d.gesture == isoGestureConnect && d.moved {
		if from, ok := d.doc.Node(d.connectFrom); ok {
			a := d.project(d.nodeAnchor(from))
			drawLine(p, b.X+iround(a.X), b.Y+iround(a.Y), b.X+d.curX, b.Y+d.curY, theme.Accent)
		}
	}

	// rubber-band preview while drawing a new zone rectangle
	if d.gesture == isoGestureZoneCreate {
		d.drawZonePreview(p, b, theme)
	}

	// rubber-band preview while sweeping a marquee selection rectangle
	if d.gesture == isoGestureMarquee && d.moved {
		strokeRect(p, b.X+min(d.pressX, d.curX), b.Y+min(d.pressY, d.curY),
			iabs(d.curX-d.pressX), iabs(d.curY-d.pressY), theme.Accent)
	}

	// labels
	for _, n := range d.doc.Nodes() {
		if n.Label == "" || !d.layerVisible(n.Layer) {
			continue
		}
		pt := d.project(d.nodeAnchor(n))
		lx := b.X + iround(pt.X) - d.textWidth(n.Label)/2
		ly := b.Y + iround(pt.Y) - d.glyphHeight()
		d.drawText(p, lx, ly, n.Label, theme.OnSurface)
	}

	// connector decorations (arrow heads, path-following labels and the
	// connector-selection highlight) in painter space over the blitted scene —
	// the same overlay layer node labels and the node outline use.
	d.drawConnectors(p, b, theme)

	// zone chrome (corner labels, the selection outline and its resize handles),
	// in painter space so it stays grabbable over any node drawn on the zone.
	d.drawZoneOverlays(p, b, theme)

	// selection outline (top face of every selected node)
	for _, r := range d.selSet.Slice() {
		if r.Kind != IsoEntityNode {
			continue
		}
		n, ok := d.doc.Node(r.ID)
		if !ok || !d.layerVisible(n.Layer) {
			continue
		}
		poly := d.topFacePoly(n)
		for i := 0; i < len(poly); i++ {
			a, c := poly[i], poly[(i+1)%len(poly)]
			drawLine(p, b.X+iround(a.X), b.Y+iround(a.Y), b.X+iround(c.X), b.Y+iround(c.Y), theme.Accent)
		}
	}

	// Top layer: text annotations float above every node, connector and zone.
	d.drawTexts(p, b, theme)

	if d.menu.Open().Get() {
		d.menu.SetBounds(b)
		d.menu.Draw(p, theme)
	}
}

// drawConnectors paints, over the blitted scene, each connector's arrow heads,
// path-following label and (for the selected connector) its highlight, in
// painter space. The stroked path itself is part of the depth-sorted scene; only
// these overlays live here, alongside the node labels and node outline.
func (d *IsoDiagram) drawConnectors(p painter.Painter, b Rect, theme *Theme) {
	for _, c := range d.doc.Connectors() {
		if !d.layerVisible(c.Layer) {
			continue
		}
		pts, ok := d.connectorPath(c)
		if !ok {
			continue
		}
		scr := d.projPath(pts)
		ink := d.connInk(c, theme)
		if d.IsSelected(IsoEntityRef{Kind: IsoEntityConnector, ID: c.ID}) {
			d.strokePath(p, b, scr, theme.Accent)
		}
		d.drawArrowHeads(p, b, scr, c.Arrow, ink)
		if c.Label != "" {
			mid := polylineMidpoint(scr)
			lx := b.X + iround(mid.X) - d.textWidth(c.Label)/2
			ly := b.Y + iround(mid.Y) - d.glyphHeight()
			d.drawText(p, lx, ly, c.Label, ink)
		}
	}
}

// strokePath draws a projected polyline as connected line segments in colour ink
// (the connector-selection highlight).
func (d *IsoDiagram) strokePath(p painter.Painter, b Rect, scr []geometry.Point, ink RGBA) {
	for i := 1; i < len(scr); i++ {
		drawLine(p, b.X+iround(scr[i-1].X), b.Y+iround(scr[i-1].Y),
			b.X+iround(scr[i].X), b.Y+iround(scr[i].Y), ink)
	}
}

// drawArrowHeads draws the connector's end heads: one at the target (To) end,
// and — for [IsoArrowDouble] — one at the source (From) end too. A headless
// connector or a path too short to have a direction draws nothing.
func (d *IsoDiagram) drawArrowHeads(p painter.Painter, b Rect, scr []geometry.Point, arrow IsoArrow, ink RGBA) {
	if arrow == IsoArrowNone || len(scr) < 2 {
		return
	}
	d.drawArrowHead(p, b, scr[len(scr)-1], scr[len(scr)-2], ink)
	if arrow == IsoArrowDouble {
		d.drawArrowHead(p, b, scr[0], scr[1], ink)
	}
}

// drawArrowHead draws a single arrow head with its tip at buffer-local point tip
// and its incoming segment coming from point from, oriented along that segment.
func (d *IsoDiagram) drawArrowHead(p painter.Painter, b Rect, tip, from geometry.Point, ink RGBA) {
	tx, ty := b.X+iround(tip.X), b.Y+iround(tip.Y)
	fx, fy := b.X+iround(from.X), b.Y+iround(from.Y)
	lx, ly, rx, ry := arrowBarbs(tx, ty, fx, fy)
	drawLine(p, tx, ty, lx, ly, ink)
	drawLine(p, tx, ty, rx, ry, ink)
}

// iround rounds a float to the nearest int (half away from zero).
func iround(v float64) int { return int(math.Round(v)) }

// iabs is the absolute value of an int.
func iabs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// --- editing commands ---------------------------------------------------

// snapshot copies the whole document.
func (d *IsoDiagram) snapshot() isoSnapshot {
	return isoSnapshot{
		nodes:  d.doc.Nodes(),
		conns:  d.doc.Connectors(),
		zones:  d.doc.Zones(),
		texts:  d.doc.Texts(),
		layers: d.doc.Layers(),
	}
}

// restore replaces the document's contents with s, without touching the undo
// stacks.
func (d *IsoDiagram) restore(s isoSnapshot) {
	// Clear connectors before nodes so this works against a store that does NOT
	// cascade a node removal to its connectors (a CRDT backing store need not).
	for _, c := range d.doc.Connectors() {
		d.doc.RemoveConnector(c.ID)
	}
	for _, n := range d.doc.Nodes() {
		d.doc.RemoveNode(n.ID)
	}
	for _, z := range d.doc.Zones() {
		d.doc.RemoveZone(z.ID)
	}
	for _, t := range d.doc.Texts() {
		d.doc.RemoveText(t.ID)
	}
	for _, l := range d.doc.Layers() {
		d.doc.RemoveLayer(l.ID)
	}
	putSnapshot(d.doc, s)
	d.pruneSelection()
}

// putSnapshot inserts every entity of s into doc, in
// node/connector/zone/text/layer order. It is the shared "load a whole-document
// value copy into a store" step behind both undo/redo ([IsoDiagram.restore],
// which clears the store first) and native JSON import ([UnmarshalIsoDocument]
// and [IsoDiagram.ImportJSON], which load into a store the caller already
// cleared or freshly made) — one code path, so a snapshot round-trips
// identically whether it came from the undo stack or a file.
func putSnapshot(doc IsoDocument, s isoSnapshot) {
	for _, n := range s.nodes {
		doc.PutNode(n)
	}
	for _, c := range s.conns {
		doc.PutConnector(c)
	}
	for _, z := range s.zones {
		doc.PutZone(z)
	}
	for _, t := range s.texts {
		doc.PutText(t)
	}
	for _, l := range s.layers {
		doc.PutLayer(l)
	}
}

// beginEdit records the pre-edit state for undo and drops the redo stack. Every
// mutating command calls it exactly once before it changes the document.
func (d *IsoDiagram) beginEdit() {
	d.undo = append(d.undo, d.snapshot())
	d.redo = nil
}

// CanUndo reports whether there is an edit to undo.
func (d *IsoDiagram) CanUndo() bool { return len(d.undo) > 0 }

// CanRedo reports whether there is an undone edit to redo.
func (d *IsoDiagram) CanRedo() bool { return len(d.redo) > 0 }

// Undo reverts the last edit.
func (d *IsoDiagram) Undo() {
	if len(d.undo) == 0 {
		return
	}
	cur := d.snapshot()
	s := d.undo[len(d.undo)-1]
	d.undo = d.undo[:len(d.undo)-1]
	d.redo = append(d.redo, cur)
	d.restore(s)
	d.invalidate()
}

// Redo re-applies the last undone edit.
func (d *IsoDiagram) Redo() {
	if len(d.redo) == 0 {
		return
	}
	cur := d.snapshot()
	s := d.redo[len(d.redo)-1]
	d.redo = d.redo[:len(d.redo)-1]
	d.undo = append(d.undo, cur)
	d.restore(s)
	d.invalidate()
}

// clearAllSelections drops every selection (a press on empty ground).
func (d *IsoDiagram) clearAllSelections() { d.selClear() }

// setSelected makes the node with id the sole selection, or — when id is "" —
// clears the node selection channel (leaving any other-kind selection alone,
// exactly as the pre-multi-selection channel did). It is the node analogue of
// [IsoDiagram.SelectConnector]; the OnSelect callback fires (through
// [IsoDiagram.refreshMono]) only when the reflected node id actually changes.
func (d *IsoDiagram) setSelected(id string) {
	if id == "" {
		d.selRemoveKind(IsoEntityNode)
		return
	}
	d.selReplace(IsoEntityRef{Kind: IsoEntityNode, ID: id})
}

// nextID returns a document-unique id with the given prefix.
func (d *IsoDiagram) nextID(prefix string) string {
	for {
		d.seq++
		id := prefix + strconv.Itoa(d.seq)
		if _, ok := d.doc.Node(id); ok {
			continue
		}
		return id
	}
}

// placeIconAt places a new node of the default shape carrying icon id icon at
// grid cell (gx, gy), selects it and returns its id. An empty icon leaves the
// node drawing as its bare shape — so [IsoDiagram.placeAt] is exactly this with
// no icon. It does not snapshot — the caller wraps it in an edit.
func (d *IsoDiagram) placeIconAt(gx, gy int, icon string) string {
	id := d.nextID("n")
	d.doc.PutNode(IsoNode{ID: id, X: gx, Y: gy, Shape: d.DefaultShape, Icon: icon})
	d.setSelected(id)
	return id
}

// placeAt places a new bare (icon-less) node at grid cell (gx, gy); see
// [IsoDiagram.placeIconAt].
func (d *IsoDiagram) placeAt(gx, gy int) string { return d.placeIconAt(gx, gy, "") }

// commitPlace is placeAt as a standalone undoable command.
func (d *IsoDiagram) commitPlace(gx, gy int) string {
	d.beginEdit()
	id := d.placeAt(gx, gy)
	d.invalidate()
	return id
}

// commitPlaceIcon is placeIconAt as a standalone undoable command: it drops a
// node carrying icon icon at cell (gx, gy) in one snapshot, so a single Undo
// removes it. It is the shared placement step behind both the drag-and-drop drop
// ([IsoDiagram.onDrop]) and the click-to-place tap ([IsoDiagram.onRelease]).
func (d *IsoDiagram) commitPlaceIcon(gx, gy int, icon string) string {
	d.beginEdit()
	id := d.placeIconAt(gx, gy, icon)
	d.invalidate()
	return id
}

// PlacementIconObservable exposes the armed click-to-place icon id so a host can
// bind an [IsoIconPalette]'s selected-icon observable straight into it: once
// bound, picking an icon in the palette arms the diagram, and the next tap on
// empty ground drops a node carrying that icon. Binding is the MVVM alternative
// to the drag-and-drop path for hosts whose back-end has no inter-widget drag.
func (d *IsoDiagram) PlacementIconObservable() *mvvm.Observable[string] { return d.placeIcon }

// PlacementIcon returns the armed click-to-place icon id, or "" when placement is
// disarmed.
func (d *IsoDiagram) PlacementIcon() string { return d.placeIcon.Get() }

// SetPlacementIcon arms (or, with "", disarms) the click-to-place icon — the
// setter counterpart of [IsoDiagram.PlacementIconObservable].
func (d *IsoDiagram) SetPlacementIcon(id string) { d.placeIcon.Set(id) }

// commitDelete removes a node (and its connectors) as an undoable command,
// clearing the selection if it was selected.
func (d *IsoDiagram) commitDelete(id string) {
	if _, ok := d.doc.Node(id); !ok {
		return
	}
	d.beginEdit()
	d.doc.RemoveNode(id)
	d.pruneSelection()
	d.invalidate()
}

// commitConnect adds a connector between two distinct nodes as an undoable
// command.
func (d *IsoDiagram) commitConnect(from, to string) {
	d.beginEdit()
	d.doc.PutConnector(IsoConnector{ID: d.nextID("c"), From: from, To: to})
	d.invalidate()
}

// connectorByID returns the connector with the given id and whether it exists.
// The [IsoDocument] interface exposes connectors only as a snapshot, so this
// scans it — the connector set a diagram carries is small.
func (d *IsoDiagram) connectorByID(id string) (IsoConnector, bool) {
	for _, c := range d.doc.Connectors() {
		if c.ID == id {
			return c, true
		}
	}
	return IsoConnector{}, false
}

// editConnector applies mutate to the connector with id as one undoable edit,
// unless the connector is missing or mutate reports no change (so a redundant set
// neither snapshots nor repaints). It returns whether it changed anything.
func (d *IsoDiagram) editConnector(id string, mutate func(*IsoConnector) bool) bool {
	c, ok := d.connectorByID(id)
	if !ok {
		return false
	}
	if !mutate(&c) {
		return false
	}
	d.beginEdit()
	d.doc.PutConnector(c)
	d.invalidate()
	return true
}

// SetConnectorStyle sets the stroke style of connector id (undoable). It returns
// false when the connector is absent or already has that style.
func (d *IsoDiagram) SetConnectorStyle(id string, s IsoConnectorStyle) bool {
	return d.editConnector(id, func(c *IsoConnector) bool {
		if c.Style == s {
			return false
		}
		c.Style = s
		return true
	})
}

// SetConnectorArrow sets the end heads of connector id (undoable). It returns
// false when the connector is absent or already has that arrow.
func (d *IsoDiagram) SetConnectorArrow(id string, a IsoArrow) bool {
	return d.editConnector(id, func(c *IsoConnector) bool {
		if c.Arrow == a {
			return false
		}
		c.Arrow = a
		return true
	})
}

// SetConnectorColor sets the stroke colour of connector id (undoable); pass a
// zero colour (A==0) to fall back to the theme link colour. It returns false when
// the connector is absent or already has that colour.
func (d *IsoDiagram) SetConnectorColor(id string, col RGBA) bool {
	return d.editConnector(id, func(c *IsoConnector) bool {
		if c.Color == col {
			return false
		}
		c.Color = col
		return true
	})
}

// SetConnectorRouted toggles grid-aware routing on connector id (undoable). It
// returns false when the connector is absent or already in that mode.
func (d *IsoDiagram) SetConnectorRouted(id string, routed bool) bool {
	return d.editConnector(id, func(c *IsoConnector) bool {
		if c.Routed == routed {
			return false
		}
		c.Routed = routed
		return true
	})
}

// SetSelectedConnectorStyle applies [IsoDiagram.SetConnectorStyle] to the
// currently selected connector (a no-op returning false when none is selected).
func (d *IsoDiagram) SetSelectedConnectorStyle(s IsoConnectorStyle) bool {
	return d.SetConnectorStyle(d.selConn.Get(), s)
}

// SetSelectedConnectorArrow applies [IsoDiagram.SetConnectorArrow] to the
// currently selected connector.
func (d *IsoDiagram) SetSelectedConnectorArrow(a IsoArrow) bool {
	return d.SetConnectorArrow(d.selConn.Get(), a)
}

// SetSelectedConnectorColor applies [IsoDiagram.SetConnectorColor] to the
// currently selected connector.
func (d *IsoDiagram) SetSelectedConnectorColor(col RGBA) bool {
	return d.SetConnectorColor(d.selConn.Get(), col)
}

// SetSelectedConnectorRouted applies [IsoDiagram.SetConnectorRouted] to the
// currently selected connector.
func (d *IsoDiagram) SetSelectedConnectorRouted(routed bool) bool {
	return d.SetConnectorRouted(d.selConn.Get(), routed)
}

// connectorAtLocal returns the id of the connector whose projected path passes
// within isoConnectorHit pixels of widget-local (x, y), nearest first, and
// whether one was hit. Only drawable connectors (both endpoints present) can be
// picked.
func (d *IsoDiagram) connectorAtLocal(x, y int) (string, bool) {
	px, py := float64(x), float64(y)
	best, bestID := math.MaxFloat64, ""
	for _, c := range d.doc.Connectors() {
		if !d.pickable(c.Layer) {
			continue
		}
		pts, ok := d.connectorPath(c)
		if !ok {
			continue
		}
		scr := d.projPath(pts)
		for i := 1; i < len(scr); i++ {
			if dsq := distToSegment(px, py, scr[i-1], scr[i]); dsq < best {
				best, bestID = dsq, c.ID
			}
		}
	}
	thr := float64(scaled(isoConnectorHit))
	if bestID != "" && best <= thr*thr {
		return bestID, true
	}
	return "", false
}

// moveDragTo moves the node being dragged to the cell under widget-local
// (x, y).
func (d *IsoDiagram) moveDragTo(x, y int) {
	n, ok := d.doc.Node(d.dragNode)
	if !ok {
		return
	}
	gx, gy := d.cellAtLocal(x, y)
	nx := d.grabNodeX + (gx - d.grabCellX)
	ny := d.grabNodeY + (gy - d.grabCellY)
	if n.X == nx && n.Y == ny {
		return
	}
	n.X, n.Y = nx, ny
	d.doc.PutNode(n)
}

// --- pan / zoom ---------------------------------------------------------

// Pan shifts the view by (dx, dy) pixels.
func (d *IsoDiagram) Pan(dx, dy int) {
	d.userMoved = true
	d.proj.Origin = geometry.Pt(d.proj.Origin.X+float64(dx), d.proj.Origin.Y+float64(dy))
	d.invalidate()
}

// ZoomAt multiplies the tile size by factor, keeping the world point currently
// under widget-local (cx, cy) fixed on screen, and clamps the tile width to
// [IsoMinTile, IsoMaxTile].
func (d *IsoDiagram) ZoomAt(factor float64, cx, cy int) {
	newW := d.proj.TileW * factor
	if newW < IsoMinTile {
		newW = IsoMinTile
	}
	if newW > IsoMaxTile {
		newW = IsoMaxTile
	}
	if newW == d.proj.TileW {
		return
	}
	d.userMoved = true
	s := newW / d.proj.TileW
	anchor := geometry.Pt(float64(cx), float64(cy))
	world := d.proj.Unproject(anchor, 0)
	d.proj.TileW *= s
	d.proj.TileH *= s
	d.proj.ZScale *= s
	// re-anchor the origin so `world` still projects onto `anchor` at z=0
	moved := d.proj.Project(iso.V(world.X, world.Y, 0))
	d.proj.Origin = geometry.Pt(
		d.proj.Origin.X+(anchor.X-moved.X),
		d.proj.Origin.Y+(anchor.Y-moved.Y),
	)
	d.invalidate()
}

// --- context menu -------------------------------------------------------

// openContextMenu builds and pops up the right-click menu for widget-local
// (x, y): Delete on the topmost entity under the pointer (text, then node, then
// zone), or Add node / Add text on empty ground.
func (d *IsoDiagram) openContextMenu(x, y int) {
	b := d.Bounds()
	if id, ok := d.textAtLocal(x, y); ok {
		d.SelectText(id)
		d.menu.Menu.Items = []MenuItem{{Label: "Delete", Action: func() { d.commitDeleteText(id) }}}
	} else if id, ok := d.nodeAtLocal(x, y); ok {
		d.setSelected(id)
		d.menu.Menu.Items = []MenuItem{{Label: "Delete", Action: func() { d.commitDelete(id) }}}
	} else if id, ok := d.zoneAtLocal(x, y); ok {
		d.SelectZone(id)
		d.menu.Menu.Items = []MenuItem{{Label: "Delete", Action: func() { d.commitDeleteZone(id) }}}
	} else {
		gx, gy := d.cellAtLocal(x, y)
		d.menu.Menu.Items = []MenuItem{
			{Label: "Add node", Action: func() { d.commitPlace(gx, gy) }},
			{Label: "Add text", Action: func() { d.commitPlaceText(gx, gy) }},
		}
	}
	d.menu.SetBounds(b)
	d.menu.Popup(b.X+x, b.Y+y)
	d.invalidate()
}

// --- events -------------------------------------------------------------

// OnEvent drives every interaction. While the context menu is open it consumes
// all input; otherwise a press starts a gesture (move / connect / pan / place),
// drags update it, release commits it, the wheel zooms and Delete/undo/redo
// keys act on the selection.
func (d *IsoDiagram) OnEvent(ev Event) {
	if d.Disabled().Get() {
		return
	}
	if d.menu.Open().Get() {
		b := d.Bounds()
		d.menu.SetBounds(b)
		d.menu.OnEvent(Event{Kind: ev.Kind, X: ev.X + b.X, Y: ev.Y + b.Y, Code: ev.Code, Delta: ev.Delta})
		d.invalidate()
		return
	}
	switch ev.Kind {
	case EventSecondaryClick:
		d.openContextMenu(ev.X, ev.Y)
	case EventClick:
		d.onPress(ev)
	case EventMouseDrag:
		d.onDrag(ev)
	case EventMouseUp:
		d.onRelease(ev)
	case EventScroll:
		f := IsoZoomStep
		if ev.Delta > 0 {
			f = 1 / IsoZoomStep
		}
		d.ZoomAt(f, ev.X, ev.Y)
	case EventKeyDown:
		d.onKey(ev)
	case EventDrop:
		d.onDrop(ev)
	}
}

// AcceptsDrop makes the diagram a [DropTarget] for icon drops from an
// [IsoIconPalette]: it accepts exactly the payloads [DecodeIsoIconPayload]
// recognises (an "iso-icon:<id>" item, possibly among several newline-separated
// ones), so a host shows the accept cursor over the canvas only for a real icon
// drag and never for an unrelated payload.
func (d *IsoDiagram) AcceptsDrop(payload string) bool {
	_, ok := DecodeIsoIconPayload(payload)
	return ok
}

// onDrop places a node carrying the dropped icon at the ground cell under the
// drop point, as one undoable edit. A payload that carries no icon id (a foreign
// drag that reached the canvas anyway) drops nothing.
func (d *IsoDiagram) onDrop(ev Event) {
	icon, ok := DecodeIsoIconPayload(ev.Code)
	if !ok {
		return
	}
	gx, gy := d.cellAtLocal(ev.X, ev.Y)
	d.commitPlaceIcon(gx, gy, icon)
}

// onPress starts a gesture from a button press.
func (d *IsoDiagram) onPress(ev Event) {
	d.moved = false
	d.curX, d.curY = ev.X, ev.Y
	d.pressX, d.pressY = ev.X, ev.Y
	d.lastX, d.lastY = ev.X, ev.Y

	// The creation modes are modal: a press begins a zone rectangle or arms a
	// text drop, regardless of what lies under the pointer.
	switch d.Mode {
	case IsoModeZone:
		d.gesture = isoGestureZoneCreate
		d.clearAllSelections()
		d.invalidate()
		return
	case IsoModeText:
		d.gesture = isoGestureTextAdd
		d.invalidate()
		return
	}

	// A resize handle of the already-selected zone wins over every other target,
	// so a corner handle sitting under a node is still grabbable.
	if corner, ok := d.zoneHandleAt(ev.X, ev.Y); ok {
		d.beginZoneResize(corner)
		d.invalidate()
		return
	}
	// A Ctrl/Shift-click extends the multi-selection instead of replacing it.
	additive := ev.Ctrl || ev.Shift

	// Text annotations are the topmost content layer, so they pick first.
	if id, ok := d.textAtLocal(ev.X, ev.Y); ok {
		ref := IsoEntityRef{Kind: IsoEntityText, ID: id}
		if additive {
			d.selToggle(ref)
			d.gesture = isoGestureNone
			d.invalidate()
			return
		}
		if d.groupPress(ref, ev.X, ev.Y) {
			d.invalidate()
			return
		}
		d.SelectText(id)
		d.beginTextMove(id, ev.X, ev.Y)
		d.invalidate()
		return
	}
	if id, ok := d.nodeAtLocal(ev.X, ev.Y); ok {
		ref := IsoEntityRef{Kind: IsoEntityNode, ID: id}
		if additive {
			d.selToggle(ref)
			d.gesture = isoGestureNone
			d.invalidate()
			return
		}
		if d.Mode != IsoModeConnect && d.groupPress(ref, ev.X, ev.Y) {
			d.invalidate()
			return
		}
		d.setSelected(id)
		if d.Mode == IsoModeConnect {
			d.gesture = isoGestureConnect
			d.connectFrom = id
		} else {
			d.gesture = isoGestureMove
			d.dragNode = id
			n, _ := d.doc.Node(id)
			d.grabNodeX, d.grabNodeY = n.X, n.Y
			d.grabCellX, d.grabCellY = d.cellAtLocal(ev.X, ev.Y)
		}
		d.invalidate()
		return
	}
	// no node under the pointer: a connector there selects it, without starting a
	// drag or a pan.
	if id, ok := d.connectorAtLocal(ev.X, ev.Y); ok {
		ref := IsoEntityRef{Kind: IsoEntityConnector, ID: id}
		if additive {
			d.selToggle(ref)
		} else {
			d.SelectConnector(id)
		}
		d.gesture = isoGestureNone
		d.invalidate()
		return
	}
	// a zone body (the floor layer) selects it and starts a move.
	if id, ok := d.zoneAtLocal(ev.X, ev.Y); ok {
		ref := IsoEntityRef{Kind: IsoEntityZone, ID: id}
		if additive {
			d.selToggle(ref)
			d.gesture = isoGestureNone
			d.invalidate()
			return
		}
		if d.groupPress(ref, ev.X, ev.Y) {
			d.invalidate()
			return
		}
		d.SelectZone(id)
		d.beginZoneMove(id, ev.X, ev.Y)
		d.invalidate()
		return
	}
	// empty ground: a Shift/Ctrl-drag sweeps a marquee selection; a plain drag
	// pans (and a plain tap places a node / clears the selection on release).
	if additive {
		d.gesture = isoGestureMarquee
		d.invalidate()
		return
	}
	d.gesture = isoGesturePan
	d.clearAllSelections()
	d.invalidate()
}

// groupPress starts a whole-selection move when the plain-pressed ref is part of
// a multi-selection, so dragging any member drags the group. It reports whether
// it took over the gesture.
func (d *IsoDiagram) groupPress(ref IsoEntityRef, x, y int) bool {
	if d.selSet.Len() > 1 && d.selIndexOf(ref) >= 0 {
		d.beginGroupMove(x, y)
		return true
	}
	return false
}

// onDrag advances the in-flight gesture.
func (d *IsoDiagram) onDrag(ev Event) {
	d.curX, d.curY = ev.X, ev.Y
	switch d.gesture {
	case isoGestureMove:
		if !d.moved {
			d.beginEdit()
			d.moved = true
		}
		d.moveDragTo(ev.X, ev.Y)
	case isoGesturePan:
		d.moved = true
		d.Pan(ev.X-d.lastX, ev.Y-d.lastY)
		d.lastX, d.lastY = ev.X, ev.Y
	case isoGestureConnect:
		d.moved = true
	case isoGestureZoneCreate, isoGestureTextAdd, isoGestureMarquee:
		d.moved = true
	case isoGestureGroupMove:
		if !d.moved {
			d.beginEdit()
			d.moved = true
		}
		d.moveGroupTo(ev.X, ev.Y)
	case isoGestureZoneMove:
		if !d.moved {
			d.beginEdit()
			d.moved = true
		}
		d.moveZoneDragTo(ev.X, ev.Y)
	case isoGestureZoneResize:
		if !d.moved {
			d.beginEdit()
			d.moved = true
		}
		d.resizeZoneDragTo(ev.X, ev.Y)
	case isoGestureTextMove:
		if !d.moved {
			d.beginEdit()
			d.moved = true
		}
		d.moveTextDragTo(ev.X, ev.Y)
	}
	d.invalidate()
}

// onRelease commits the in-flight gesture.
func (d *IsoDiagram) onRelease(ev Event) {
	switch d.gesture {
	case isoGestureMove:
		if d.moved {
			d.moveDragTo(ev.X, ev.Y)
		}
	case isoGestureConnect:
		if id, ok := d.nodeAtLocal(ev.X, ev.Y); ok && id != d.connectFrom {
			d.commitConnect(d.connectFrom, id)
		}
	case isoGesturePan:
		if !d.moved {
			gx, gy := d.cellAtLocal(d.pressX, d.pressY)
			if icon := d.placeIcon.Get(); icon != "" {
				d.commitPlaceIcon(gx, gy, icon)
			} else {
				d.commitPlace(gx, gy)
			}
		}
	case isoGestureZoneCreate:
		gx0, gy0 := d.cellAtLocal(d.pressX, d.pressY)
		gx1, gy1 := d.cellAtLocal(ev.X, ev.Y)
		d.commitPlaceZone(gx0, gy0, gx1, gy1)
	case isoGestureZoneMove:
		if d.moved {
			d.moveZoneDragTo(ev.X, ev.Y)
		}
	case isoGestureZoneResize:
		if d.moved {
			d.resizeZoneDragTo(ev.X, ev.Y)
		}
	case isoGestureTextMove:
		if d.moved {
			d.moveTextDragTo(ev.X, ev.Y)
		}
	case isoGestureTextAdd:
		if !d.moved {
			gx, gy := d.cellAtLocal(d.pressX, d.pressY)
			d.commitPlaceText(gx, gy)
		}
	case isoGestureMarquee:
		d.selReplace(d.marqueeRefs(d.pressX, d.pressY, ev.X, ev.Y)...)
	case isoGestureGroupMove:
		if d.moved {
			d.moveGroupTo(ev.X, ev.Y)
		}
	}
	d.gesture = isoGestureNone
	d.dragNode = ""
	d.connectFrom = ""
	d.dragZone = ""
	d.dragText = ""
	d.groupGrab = d.groupGrab[:0]
	d.moved = false
	d.invalidate()
}

// onKey handles Delete (remove selection), Ctrl-Z (undo) and Ctrl-Y /
// Ctrl-Shift-Z (redo).
func (d *IsoDiagram) onKey(ev Event) {
	switch ev.Code {
	case "Delete", "Backspace":
		d.DeleteSelection()
	case "a", "A":
		if ev.Ctrl {
			d.SelectAll()
		}
	case "c", "C":
		if ev.Ctrl {
			d.Copy()
		}
	case "x", "X":
		if ev.Ctrl {
			d.Cut()
		}
	case "v", "V":
		if ev.Ctrl {
			d.Paste()
		}
	case "d", "D":
		if ev.Ctrl {
			d.Duplicate()
		}
	case "z", "Z":
		if ev.Ctrl {
			if ev.Shift {
				d.Redo()
			} else {
				d.Undo()
			}
		}
	case "y", "Y":
		if ev.Ctrl {
			d.Redo()
		}
	}
}

// --- accessibility ------------------------------------------------------

// A11y describes the widget itself as a group holding the diagram's elements.
func (d *IsoDiagram) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: "isometric diagram"}
}

// Children exposes each node and connector as an accessibility proxy so a screen
// reader can enumerate the diagram's contents, with the node's label and its
// selected state. The proxies are synthetic — they are not laid out or drawn;
// only the a11y walk (and text-run collection, which finds nothing on them)
// consults them.
func (d *IsoDiagram) Children() []Widget {
	b := d.Bounds()
	var out []Widget
	for _, n := range d.doc.Nodes() {
		name := n.Label
		if name == "" {
			name = n.ID
		}
		pt := d.project(d.nodeAnchor(n))
		sx := b.X + iround(pt.X)
		sy := b.Y + iround(pt.Y)
		tile := iround(d.proj.TileW)
		pr := &isoProxy{info: A11yInfo{Role: RoleImg, Name: name, Value: stateValue(d.IsSelected(IsoEntityRef{Kind: IsoEntityNode, ID: n.ID}), "selected")}}
		pr.SetBounds(Rect{X: sx - tile/2, Y: sy - tile/2, W: tile, H: tile})
		out = append(out, pr)
	}
	for _, c := range d.doc.Connectors() {
		name := c.Label
		if name == "" {
			name = c.From + " to " + c.To
		}
		out = append(out, &isoProxy{info: A11yInfo{Role: RoleImg, Name: name}})
	}
	for _, z := range d.doc.Zones() {
		name := z.Label
		if name == "" {
			name = z.ID
		}
		pr := &isoProxy{info: A11yInfo{Role: RoleGroup, Name: name, Value: stateValue(d.IsSelected(IsoEntityRef{Kind: IsoEntityZone, ID: z.ID}), "selected")}}
		corners := d.zoneCorners(z)
		pr.SetBounds(zoneBoundsRect(b, corners))
		out = append(out, pr)
	}
	for _, t := range d.doc.Texts() {
		name := t.Text
		if name == "" {
			name = t.ID
		}
		box := d.textBox(t)
		pr := &isoProxy{info: A11yInfo{Role: RoleImg, Name: name, Value: stateValue(d.IsSelected(IsoEntityRef{Kind: IsoEntityText, ID: t.ID}), "selected")}}
		pr.SetBounds(Rect{X: b.X + box.X, Y: b.Y + box.Y, W: box.W, H: box.H})
		out = append(out, pr)
	}
	return out
}

// zoneBoundsRect is the widget-absolute bounding rectangle of a zone's projected
// corners (buffer-local), offset into the widget by b's origin — the a11y box a
// screen reader reports for the zone.
func zoneBoundsRect(b Rect, corners []geometry.Point) Rect {
	minX, minY := corners[0].X, corners[0].Y
	maxX, maxY := corners[0].X, corners[0].Y
	for _, c := range corners[1:] {
		minX, maxX = math.Min(minX, c.X), math.Max(maxX, c.X)
		minY, maxY = math.Min(minY, c.Y), math.Max(maxY, c.Y)
	}
	return Rect{X: b.X + iround(minX), Y: b.Y + iround(minY), W: iround(maxX - minX), H: iround(maxY - minY)}
}

// isoProxy is a non-visual widget that carries one accessibility description for
// an [IsoDiagram] node or connector.
type isoProxy struct {
	Base
	info A11yInfo
}

// A11y returns the proxy's fixed description.
func (p *isoProxy) A11y() A11yInfo { return p.info }
