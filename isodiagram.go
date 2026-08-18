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
	"github.com/go-widgets/painter"
)

// IsoMode selects what a left-drag on a node does.
type IsoMode int

const (
	// IsoModeSelect is the default: click selects a node, drag moves it to
	// another cell, drag on empty ground pans the view.
	IsoModeSelect IsoMode = iota
	// IsoModeConnect turns a node drag into a connector gesture: press one node,
	// release on another, and a connector is created between them.
	IsoModeConnect
)

// isoGesture is the in-flight pointer gesture between an EventClick (press) and
// the matching EventMouseUp (release).
type isoGesture int

const (
	isoGestureNone isoGesture = iota
	isoGestureMove
	isoGestureConnect
	isoGesturePan
)

// isoSnapshot is a whole-document copy for the undo/redo stacks. The document is
// small and value-typed, so a snapshot is a cheap slice pair and undo is a plain
// replace — a scheme that works against any [IsoDocument], the bundled one or a
// future CRDT store.
type isoSnapshot struct {
	nodes []IsoNode
	conns []IsoConnector
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

	// OnSelect fires when the selected node changes; id is "" when the selection
	// is cleared.
	OnSelect func(id string)
	// OnInvalidate, when set, is called whenever the widget's appearance changed
	// and it should be redrawn. A document edit (including one from another
	// collaborator, via the store's Subscribe) also triggers it.
	OnInvalidate func()

	doc  IsoDocument
	proj *iso.Projection
	menu *ContextMenu

	selected string
	seq      int // monotonic id source for placed nodes/connectors

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
		Cols: 10,
		Rows: 10,
		doc:  doc,
		proj: iso.NewDefault(geometry.Pt(0, 0)),
		menu: NewContextMenu(NewMenu(nil)),
	}
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
		d.proj.Project(iso.V(x0, y0, z)),
		d.proj.Project(iso.V(x1, y0, z)),
		d.proj.Project(iso.V(x1, y1, z)),
		d.proj.Project(iso.V(x0, y1, z)),
	}
}

// pickPolys returns the projected silhouette polygons of a node (buffer-local),
// whose union is the whole area a pointer can hit it on. For a box that is the
// top, the two visible sides and the bottom (which together tile the hexagon
// outline); for a pyramid the base square and the two visible triangles.
func (d *IsoDiagram) pickPolys(n IsoNode) [][]geometry.Point {
	z := d.nodeHeight(n)
	x0, y0 := float64(n.X), float64(n.Y)
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
		return d.proj.Depth(iso.V(float64(nodes[i].X), float64(nodes[i].Y), 0)) >
			d.proj.Depth(iso.V(float64(nodes[j].X), float64(nodes[j].Y), 0))
	})
	fx, fy := float64(x), float64(y)
	for _, n := range nodes {
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
	w := d.proj.Unproject(geometry.Pt(float64(x), float64(y)), 0)
	return int(math.Floor(w.X)), int(math.Floor(w.Y))
}

// --- rendering ----------------------------------------------------------

// scene assembles the depth-sorted isometric scene: ground grid first, then a
// solid per node, then a line per connector whose endpoints both exist.
func (d *IsoDiagram) scene(theme *Theme) *iso.Scene {
	sc := iso.NewScene(d.proj)
	grid := stdColor(theme.Border)
	for i := 0; i <= d.Cols; i++ {
		sc.Add(iso.Line{From: iso.V(float64(i), 0, 0), To: iso.V(float64(i), float64(d.Rows), 0), Color: grid, Width: 1})
	}
	for j := 0; j <= d.Rows; j++ {
		sc.Add(iso.Line{From: iso.V(0, float64(j), 0), To: iso.V(float64(d.Cols), float64(j), 0), Color: grid, Width: 1})
	}
	for _, n := range d.doc.Nodes() {
		base := d.resolveColor(n, theme)
		if n.Icon == "" {
			// No icon: draw the bare shape solid (cube / box / pyramid), exactly as
			// before — the icon system is purely additive.
			sc.Add(d.nodeSolid(n, base))
			continue
		}
		// An icon id (known, or an unknown one that resolves to the cube fallback)
		// contributes its depth-sortable solids to the same scene. A sprite icon
		// adds no shapes here and is blitted after the scene renders (drawSprites).
		icon, _ := d.iconRegistry().Resolve(n.Icon)
		sc.Add(icon.Render(n.X, n.Y, base).Shapes...)
	}
	link := stdColor(theme.OnSurface)
	for _, c := range d.doc.Connectors() {
		a, ok1 := d.doc.Node(c.From)
		b, ok2 := d.doc.Node(c.To)
		if ok1 && ok2 {
			sc.Add(iso.Line{From: d.nodeAnchor(a), To: d.nodeAnchor(b), Color: link, Width: 2})
		}
	}
	return sc
}

// spriteRect is the widget-local pixel rectangle a billboarded sprite icon fills
// for node n: a TileW-sized square standing on the projected ground centre of
// the node's cell, i.e. its bottom-centre sits on that ground point so the
// sprite reads as planted on the tile.
func (d *IsoDiagram) spriteRect(n IsoNode) Rect {
	p := d.proj.Project(iso.V(float64(n.X)+0.5, float64(n.Y)+0.5, 0))
	s := iround(d.proj.TileW)
	return Rect{X: iround(p.X) - s/2, Y: iround(p.Y) - s, W: s, H: s}
}

// drawSprites blits the sprite of every icon node whose icon contributes one,
// over the already-rendered scene (into the same buffer, so it is captured by
// the single blit). Primitive-only icons contribute no sprite and are skipped.
func (d *IsoDiagram) drawSprites(img *raster.Image, theme *Theme) {
	for _, n := range d.doc.Nodes() {
		if n.Icon == "" {
			continue
		}
		icon, _ := d.iconRegistry().Resolve(n.Icon)
		if sprite := icon.Render(n.X, n.Y, d.resolveColor(n, theme)).Sprite; sprite != nil {
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
	d.scene(theme).Render(img)
	d.drawSprites(img, theme)
	blitImage(p, b, b, img.Pix, b.W, b.H)

	// rubber-band preview while connecting
	if d.gesture == isoGestureConnect && d.moved {
		if from, ok := d.doc.Node(d.connectFrom); ok {
			a := d.proj.Project(d.nodeAnchor(from))
			drawLine(p, b.X+iround(a.X), b.Y+iround(a.Y), b.X+d.curX, b.Y+d.curY, theme.Accent)
		}
	}

	// labels
	for _, n := range d.doc.Nodes() {
		if n.Label == "" {
			continue
		}
		pt := d.proj.Project(d.nodeAnchor(n))
		lx := b.X + iround(pt.X) - d.textWidth(n.Label)/2
		ly := b.Y + iround(pt.Y) - d.glyphHeight()
		d.drawText(p, lx, ly, n.Label, theme.OnSurface)
	}

	// selection outline (top face of the selected node)
	if n, ok := d.doc.Node(d.selected); ok {
		poly := d.topFacePoly(n)
		for i := 0; i < len(poly); i++ {
			a, c := poly[i], poly[(i+1)%len(poly)]
			drawLine(p, b.X+iround(a.X), b.Y+iround(a.Y), b.X+iround(c.X), b.Y+iround(c.Y), theme.Accent)
		}
	}

	if d.menu.Open {
		d.menu.SetBounds(b)
		d.menu.Draw(p, theme)
	}
}

// iround rounds a float to the nearest int (half away from zero).
func iround(v float64) int { return int(math.Round(v)) }

// --- editing commands ---------------------------------------------------

// snapshot copies the whole document.
func (d *IsoDiagram) snapshot() isoSnapshot {
	return isoSnapshot{nodes: d.doc.Nodes(), conns: d.doc.Connectors()}
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
	for _, n := range s.nodes {
		d.doc.PutNode(n)
	}
	for _, c := range s.conns {
		d.doc.PutConnector(c)
	}
	if _, ok := d.doc.Node(d.selected); !ok {
		d.setSelected("")
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

// setSelected updates the selection and fires OnSelect when it changed.
func (d *IsoDiagram) setSelected(id string) {
	if d.selected == id {
		return
	}
	d.selected = id
	if d.OnSelect != nil {
		d.OnSelect(id)
	}
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

// placeAt places a new node of the default shape at grid cell (gx, gy), selects
// it and returns its id. It does not snapshot — the caller wraps it in an edit.
func (d *IsoDiagram) placeAt(gx, gy int) string {
	id := d.nextID("n")
	d.doc.PutNode(IsoNode{ID: id, X: gx, Y: gy, Shape: d.DefaultShape})
	d.setSelected(id)
	return id
}

// commitPlace is placeAt as a standalone undoable command.
func (d *IsoDiagram) commitPlace(gx, gy int) string {
	d.beginEdit()
	id := d.placeAt(gx, gy)
	d.invalidate()
	return id
}

// commitDelete removes a node (and its connectors) as an undoable command,
// clearing the selection if it was selected.
func (d *IsoDiagram) commitDelete(id string) {
	if _, ok := d.doc.Node(id); !ok {
		return
	}
	d.beginEdit()
	d.doc.RemoveNode(id)
	if d.selected == id {
		d.setSelected("")
	}
	d.invalidate()
}

// commitConnect adds a connector between two distinct nodes as an undoable
// command.
func (d *IsoDiagram) commitConnect(from, to string) {
	d.beginEdit()
	d.doc.PutConnector(IsoConnector{ID: d.nextID("c"), From: from, To: to})
	d.invalidate()
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
// (x, y): Delete on a node, Add node on empty ground.
func (d *IsoDiagram) openContextMenu(x, y int) {
	b := d.Bounds()
	if id, ok := d.nodeAtLocal(x, y); ok {
		d.setSelected(id)
		d.menu.Menu.Items = []MenuItem{{Label: "Delete", Action: func() { d.commitDelete(id) }}}
	} else {
		gx, gy := d.cellAtLocal(x, y)
		d.menu.Menu.Items = []MenuItem{{Label: "Add node", Action: func() { d.commitPlace(gx, gy) }}}
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
	if d.Disabled {
		return
	}
	if d.menu.Open {
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
	}
}

// onPress starts a gesture from a button press.
func (d *IsoDiagram) onPress(ev Event) {
	d.moved = false
	d.curX, d.curY = ev.X, ev.Y
	d.pressX, d.pressY = ev.X, ev.Y
	d.lastX, d.lastY = ev.X, ev.Y
	if id, ok := d.nodeAtLocal(ev.X, ev.Y); ok {
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
	// empty ground: could become a pan (drag) or a place (tap on release)
	d.gesture = isoGesturePan
	d.setSelected("")
	d.invalidate()
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
			d.commitPlace(gx, gy)
		}
	}
	d.gesture = isoGestureNone
	d.dragNode = ""
	d.connectFrom = ""
	d.moved = false
	d.invalidate()
}

// onKey handles Delete (remove selection), Ctrl-Z (undo) and Ctrl-Y /
// Ctrl-Shift-Z (redo).
func (d *IsoDiagram) onKey(ev Event) {
	switch ev.Code {
	case "Delete", "Backspace":
		if d.selected != "" {
			d.commitDelete(d.selected)
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
		pt := d.proj.Project(d.nodeAnchor(n))
		sx := b.X + iround(pt.X)
		sy := b.Y + iround(pt.Y)
		tile := iround(d.proj.TileW)
		pr := &isoProxy{info: A11yInfo{Role: RoleImg, Name: name, Value: stateValue(n.ID == d.selected, "selected")}}
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
	return out
}

// isoProxy is a non-visual widget that carries one accessibility description for
// an [IsoDiagram] node or connector.
type isoProxy struct {
	Base
	info A11yInfo
}

// A11y returns the proxy's fixed description.
func (p *isoProxy) A11y() A11yInfo { return p.info }
