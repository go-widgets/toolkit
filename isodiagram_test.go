// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	gfxcolor "github.com/go-gfx/gfx/color"
	"github.com/go-gfx/gfx/geometry"
	"github.com/go-gfx/gfx/iso"
	stdcolor "image/color"
	"testing"

	"github.com/go-widgets/painter"
)

type geoPoint = geometry.Point

// painterPixel builds a pixel-backed Painter over buf for the render tests.
func painterPixel(buf []byte, w, h int) painter.Painter { return painter.NewPixelPainter(buf, w, h) }

// localOf projects a world point to the widget-local pixel it lands on.
func localOf(d *IsoDiagram, v iso.Vec3) (int, int) {
	p := d.proj.Project(v)
	return iround(p.X), iround(p.Y)
}

// nodeCenterLocal is the widget-local pixel at a node's top centre.
func nodeCenterLocal(d *IsoDiagram, n IsoNode) (int, int) {
	return localOf(d, d.nodeAnchor(n))
}

// groundCenterLocal is the widget-local pixel at the ground centre of cell
// (gx,gy) — the point whose z=0 unprojection is exactly that cell, so grabbing a
// node there gives a predictable drag delta.
func groundCenterLocal(d *IsoDiagram, gx, gy int) (int, int) {
	return localOf(d, iso.V(float64(gx)+0.5, float64(gy)+0.5, 0))
}

// insideDrawn reports whether (x,y) is inside one of a node's *visible* faces
// (top + sides, i.e. every pick polygon except the undrawn base square).
func insideDrawn(d *IsoDiagram, n IsoNode, x, y int) bool {
	polys := d.pickPolys(n)
	for _, poly := range polys[1:] {
		if pointInPoly(float64(x), float64(y), poly) {
			return true
		}
	}
	return false
}

// --- projection / picking round-trips -----------------------------------

func TestIsoProjectUnprojectRoundTrip(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	for _, c := range [][2]int{{0, 0}, {3, 4}, {7, 2}} {
		g := iso.V(float64(c[0])+0.5, float64(c[1])+0.5, 0)
		s := d.proj.Project(g)
		back := d.proj.Unproject(s, 0)
		if !approx(back.X, g.X) || !approx(back.Y, g.Y) {
			t.Fatalf("round-trip %v -> %v -> %v", g, s, back)
		}
		// A click at the projected centre picks exactly that grid cell.
		gx, gy := d.cellAtLocal(iround(s.X), iround(s.Y))
		if gx != c[0] || gy != c[1] {
			t.Fatalf("cellAtLocal picked (%d,%d), want (%d,%d)", gx, gy, c[0], c[1])
		}
	}
}

func approx(a, b float64) bool {
	d := a - b
	return d < 1e-6 && d > -1e-6
}

func TestIsoNodeAnchorHeights(t *testing.T) {
	d := NewIsoDiagram(nil)
	cube := d.nodeAnchor(IsoNode{X: 2, Y: 3, Shape: IsoCube})
	if cube != iso.V(2.5, 3.5, 1) {
		t.Fatalf("cube anchor = %v", cube)
	}
	box := d.nodeAnchor(IsoNode{X: 2, Y: 3, Shape: IsoBox})
	if box != iso.V(2.5, 3.5, 2) {
		t.Fatalf("box anchor = %v", box)
	}
	py := d.nodeAnchor(IsoNode{X: 0, Y: 0, Shape: IsoPyramid})
	if py != iso.V(0.5, 0.5, 1) {
		t.Fatalf("pyramid anchor = %v", py)
	}
}

func TestIsoNodePickNearestWins(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 0, Y: 0})
	d.Doc().PutNode(IsoNode{ID: "b", X: 1, Y: 0})
	na, _ := d.Doc().Node("a")
	nb, _ := d.Doc().Node("b")
	// A point inside both silhouettes must resolve to the nearer node (b).
	for y := 0; y < 400; y++ {
		for x := 0; x < 400; x++ {
			if insideDrawn(d, na, x, y) && insideDrawn(d, nb, x, y) {
				if id, ok := d.nodeAtLocal(x, y); !ok || id != "b" {
					t.Fatalf("overlap pick = %q ok=%v, want b", id, ok)
				}
				return
			}
		}
	}
	t.Fatal("no overlap point found")
}

func TestIsoNodeAtEmptyIsMiss(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	if _, ok := d.nodeAtLocal(0, 0); ok {
		t.Fatal("empty diagram picked a node")
	}
}

func TestIsoPyramidPick(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "p", X: 4, Y: 4, Shape: IsoPyramid})
	// The base centre is unambiguously inside the pyramid's footprint square.
	x, y := groundCenterLocal(d, 4, 4)
	if id, ok := d.nodeAtLocal(x, y); !ok || id != "p" {
		t.Fatalf("pyramid pick = %q ok=%v", id, ok)
	}
}

// --- rendering / pixel probes -------------------------------------------

func TestIsoNodeTopFaceShadePixel(t *testing.T) {
	d := NewIsoDiagram(nil)
	red := RGBA{R: 220, G: 0, B: 0, A: 255}
	d.Doc().PutNode(IsoNode{ID: "a", X: 3, Y: 3, Color: red})
	img, err := RenderImage(d, 400, 400, DefaultLight())
	if err != nil {
		t.Fatal(err)
	}
	n, _ := d.Doc().Node("a")
	// Top face: DefaultShading.Top == 1, so the pixel is the node colour exactly.
	tx, ty := localOf(d, d.nodeAnchor(n))
	if got := img.RGBAAt(tx, ty); got != stdcolor.RGBA(stdColor(red)) {
		t.Fatalf("top-face pixel = %v, want %v", got, red)
	}
	// A point interior to the +X side face: shaded by DefaultShading.Right.
	sx, sy := localOf(d, iso.V(4, 3.5, 0.35))
	want := gfxcolor.Shade(stdColor(red), iso.DefaultShading.Right)
	if got := img.RGBAAt(sx, sy); got != want {
		t.Fatalf("side-face pixel = %v, want %v (shade)", got, want)
	}
}

func TestIsoDepthOrderPixel(t *testing.T) {
	d := NewIsoDiagram(nil)
	red := RGBA{R: 220, G: 0, B: 0, A: 255}
	green := RGBA{R: 0, G: 220, B: 0, A: 255}
	d.Doc().PutNode(IsoNode{ID: "a", X: 0, Y: 0, Color: red})
	d.Doc().PutNode(IsoNode{ID: "b", X: 1, Y: 0, Color: green}) // nearer, drawn on top
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	na, _ := d.Doc().Node("a")
	nb, _ := d.Doc().Node("b")
	img, err := RenderImage(d, 400, 400, DefaultLight())
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 400; y++ {
		for x := 0; x < 400; x++ {
			if insideDrawn(d, na, x, y) && insideDrawn(d, nb, x, y) {
				got := img.RGBAAt(x, y)
				if got.G <= got.R {
					t.Fatalf("overlap pixel %v not green: nearer node did not draw last", got)
				}
				return
			}
		}
	}
	t.Fatal("no overlap point found")
}

func TestIsoConnectorRendersBetweenAnchors(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 4})
	d.Doc().PutNode(IsoNode{ID: "b", X: 6, Y: 4})
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b"})
	img, err := RenderImage(d, 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	na, _ := d.Doc().Node("a")
	nb, _ := d.Doc().Node("b")
	fa := d.nodeAnchor(na)
	fb := d.nodeAnchor(nb)
	mid := iso.V((fa.X+fb.X)/2, (fa.Y+fb.Y)/2, (fa.Z+fb.Z)/2)
	mx, my := localOf(d, mid)
	if got := img.RGBAAt(mx, my); got != stdcolor.RGBA(stdColor(theme.OnSurface)) {
		t.Fatalf("connector midpoint pixel = %v, want link colour %v", got, theme.OnSurface)
	}
}

func TestIsoResolveColorDefaultsToAccent(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	if got := d.resolveColor(IsoNode{}, theme); got != stdColor(theme.Accent) {
		t.Fatalf("unset colour = %v, want accent", got)
	}
	c := RGBA{R: 1, G: 2, B: 3, A: 255}
	if got := d.resolveColor(IsoNode{Color: c}, theme); got != stdColor(c) {
		t.Fatalf("explicit colour = %v, want %v", got, c)
	}
}

func TestIsoDrawGuards(t *testing.T) {
	d := NewIsoDiagram(nil)
	buf := make([]byte, 4*10*10)
	p := painterPixel(buf, 10, 10)
	d.Base.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 10})
	d.Draw(p, DefaultLight()) // W<=0 early return
	d.Base.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 0})
	d.Draw(p, DefaultLight()) // H<=0 early return
}

func TestIsoDrawLabelsAndSelection(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 2, Label: "Server", Shape: IsoCube})
	d.Doc().PutNode(IsoNode{ID: "b", X: 5, Y: 5, Shape: IsoBox})     // no label -> label loop continues
	d.Doc().PutNode(IsoNode{ID: "c", X: 7, Y: 1, Shape: IsoPyramid}) // exercises every nodeSolid branch
	d.setSelected("a")
	if _, err := RenderImage(d, 400, 400, DefaultLight()); err != nil {
		t.Fatal(err)
	}
}

// --- interactions -------------------------------------------------------

func TestIsoTapPlacesNode(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	x, y := localOf(d, iso.V(3.5, 4.5, 0))
	d.OnEvent(Event{Kind: EventClick, X: x, Y: y})   // press on empty ground
	d.OnEvent(Event{Kind: EventMouseUp, X: x, Y: y}) // release without a drag -> place
	nodes := d.Doc().Nodes()
	if len(nodes) != 1 || nodes[0].X != 3 || nodes[0].Y != 4 {
		t.Fatalf("tap placed %+v, want one node at (3,4)", nodes)
	}
	if d.Selected() != nodes[0].ID {
		t.Fatalf("placed node not selected: %q", d.Selected())
	}
}

func TestIsoDragMovesNodeToCell(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 3})
	px, py := groundCenterLocal(d, 2, 3) // grab at the node's own ground cell
	tx, ty := groundCenterLocal(d, 6, 1) // drop on target cell (6,1)
	d.OnEvent(Event{Kind: EventClick, X: px, Y: py})
	d.OnEvent(Event{Kind: EventMouseDrag, X: tx, Y: ty})
	d.OnEvent(Event{Kind: EventMouseUp, X: tx, Y: ty})
	m, _ := d.Doc().Node("a")
	if m.X != 6 || m.Y != 1 {
		t.Fatalf("drag moved node to (%d,%d), want (6,1)", m.X, m.Y)
	}
	// Undo restores the original cell.
	d.Undo()
	u, _ := d.Doc().Node("a")
	if u.X != 2 || u.Y != 3 {
		t.Fatalf("undo left node at (%d,%d), want (2,3)", u.X, u.Y)
	}
}

func TestIsoDragSameCellIsNoop(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 3})
	px, py := groundCenterLocal(d, 2, 3)
	d.OnEvent(Event{Kind: EventClick, X: px, Y: py})
	d.OnEvent(Event{Kind: EventMouseDrag, X: px, Y: py}) // same cell
	d.OnEvent(Event{Kind: EventMouseUp, X: px, Y: py})
	m, _ := d.Doc().Node("a")
	if m.X != 2 || m.Y != 3 {
		t.Fatalf("no-op drag moved node to (%d,%d)", m.X, m.Y)
	}
}

func TestIsoMoveDragToMissingNode(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.dragNode = "ghost"
	d.moveDragTo(10, 10) // node absent -> early return, must not panic
}

func TestIsoPanOnEmptyDrag(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	ox := d.proj.Origin.X
	d.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	d.OnEvent(Event{Kind: EventMouseDrag, X: 25, Y: 35})
	d.OnEvent(Event{Kind: EventMouseUp, X: 25, Y: 35})
	if d.proj.Origin.X != ox+20 {
		t.Fatalf("pan moved origin.X to %v, want %v", d.proj.Origin.X, ox+20)
	}
	if len(d.Doc().Nodes()) != 0 {
		t.Fatal("pan should not place a node")
	}
	// Once panned, SetBounds no longer re-centres.
	before := d.proj.Origin
	d.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 800})
	if d.proj.Origin != before {
		t.Fatalf("SetBounds re-centred after user pan: %v -> %v", before, d.proj.Origin)
	}
}

func TestIsoConnectDragCreatesConnector(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Mode = IsoModeConnect
	d.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.Doc().PutNode(IsoNode{ID: "b", X: 6, Y: 1})
	na, _ := d.Doc().Node("a")
	nb, _ := d.Doc().Node("b")
	ax, ay := nodeCenterLocal(d, na)
	bx, by := nodeCenterLocal(d, nb)
	d.OnEvent(Event{Kind: EventClick, X: ax, Y: ay})
	d.OnEvent(Event{Kind: EventMouseDrag, X: (ax + bx) / 2, Y: (ay + by) / 2})
	d.Draw(painterPixel(make([]byte, 4*500*400), 500, 400), DefaultLight()) // rubber-band preview branch
	d.OnEvent(Event{Kind: EventMouseUp, X: bx, Y: by})
	cs := d.Doc().Connectors()
	if len(cs) != 1 || cs[0].From != "a" || cs[0].To != "b" {
		t.Fatalf("connect made %+v, want one a->b", cs)
	}
}

func TestIsoConnectDragToEmptyMakesNothing(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Mode = IsoModeConnect
	d.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	na, _ := d.Doc().Node("a")
	ax, ay := nodeCenterLocal(d, na)
	d.OnEvent(Event{Kind: EventClick, X: ax, Y: ay})
	d.OnEvent(Event{Kind: EventMouseDrag, X: ax + 2, Y: ay + 2})
	d.OnEvent(Event{Kind: EventMouseUp, X: 480, Y: 380}) // empty
	if len(d.Doc().Connectors()) != 0 {
		t.Fatal("connect to empty created a connector")
	}
	// Drag ending on the same node also makes nothing.
	d.OnEvent(Event{Kind: EventClick, X: ax, Y: ay})
	d.OnEvent(Event{Kind: EventMouseUp, X: ax, Y: ay})
	if len(d.Doc().Connectors()) != 0 {
		t.Fatal("self connect created a connector")
	}
}

func TestIsoSelectAndDeselect(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 2})
	var sel []string
	d.OnSelect = func(id string) { sel = append(sel, id) }
	n, _ := d.Doc().Node("a")
	px, py := nodeCenterLocal(d, n)
	d.OnEvent(Event{Kind: EventClick, X: px, Y: py})
	if d.Selected() != "a" {
		t.Fatalf("select got %q", d.Selected())
	}
	d.OnEvent(Event{Kind: EventMouseUp, X: px, Y: py})
	// Press empty -> clears selection.
	d.OnEvent(Event{Kind: EventClick, X: 380, Y: 380})
	if d.Selected() != "" {
		t.Fatalf("empty press did not clear selection: %q", d.Selected())
	}
	if len(sel) != 2 || sel[0] != "a" || sel[1] != "" {
		t.Fatalf("OnSelect calls = %v", sel)
	}
	// Re-selecting the same id fires nothing new.
	d.setSelected("")
	before := len(sel)
	d.setSelected("")
	if len(sel) != before {
		t.Fatal("setSelected fired on unchanged value")
	}
}

func TestIsoDeleteKeyRemovesSelection(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 2})
	d.Doc().PutNode(IsoNode{ID: "b", X: 3, Y: 3})
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b"})
	d.setSelected("a")
	d.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	if _, ok := d.Doc().Node("a"); ok {
		t.Fatal("Delete did not remove node a")
	}
	if len(d.Doc().Connectors()) != 0 {
		t.Fatal("Delete did not drop the attached connector")
	}
	if d.Selected() != "" {
		t.Fatal("selection not cleared after delete")
	}
	// Backspace path with nothing selected is a no-op.
	d.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
}

func TestIsoUndoRedo(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	if d.CanUndo() || d.CanRedo() {
		t.Fatal("fresh widget reports undo/redo")
	}
	d.Undo() // empty no-op
	d.Redo() // empty no-op
	id := d.commitPlace(2, 2)
	if !d.CanUndo() {
		t.Fatal("place should be undoable")
	}
	d.Undo()
	if _, ok := d.Doc().Node(id); ok {
		t.Fatal("undo did not remove placed node")
	}
	if d.Selected() != "" {
		t.Fatal("undo should have cleared the now-absent selection")
	}
	if !d.CanRedo() {
		t.Fatal("redo should be available")
	}
	d.Redo()
	if _, ok := d.Doc().Node(id); !ok {
		t.Fatal("redo did not restore the node")
	}
}

func TestIsoUndoRedoKeys(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	id := d.commitPlace(2, 2)
	d.OnEvent(Event{Kind: EventKeyDown, Code: "z"}) // no Ctrl -> ignored
	if _, ok := d.Doc().Node(id); !ok {
		t.Fatal("plain z should not undo")
	}
	d.OnEvent(Event{Kind: EventKeyDown, Code: "z", Ctrl: true}) // undo
	if _, ok := d.Doc().Node(id); ok {
		t.Fatal("Ctrl-Z did not undo")
	}
	d.OnEvent(Event{Kind: EventKeyDown, Code: "Z", Ctrl: true, Shift: true}) // redo
	if _, ok := d.Doc().Node(id); !ok {
		t.Fatal("Ctrl-Shift-Z did not redo")
	}
	d.OnEvent(Event{Kind: EventKeyDown, Code: "z", Ctrl: true}) // undo again
	d.OnEvent(Event{Kind: EventKeyDown, Code: "y", Ctrl: true}) // redo via Ctrl-Y
	if _, ok := d.Doc().Node(id); !ok {
		t.Fatal("Ctrl-Y did not redo")
	}
}

func TestIsoUndoConnectClearsConnectorsOnRestore(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.Doc().PutNode(IsoNode{ID: "b", X: 4, Y: 1})
	d.commitConnect("a", "b")
	if len(d.Doc().Connectors()) != 1 {
		t.Fatal("connector not created")
	}
	// Undo restores the pre-connect state; restore() must clear the live
	// connector (the connector-remove loop) before rebuilding.
	d.Undo()
	if len(d.Doc().Connectors()) != 0 {
		t.Fatal("undo did not remove the connector")
	}
	if len(d.Doc().Nodes()) != 2 {
		t.Fatalf("undo changed the node set: %d", len(d.Doc().Nodes()))
	}
	d.Redo()
	if len(d.Doc().Connectors()) != 1 {
		t.Fatal("redo did not restore the connector")
	}
}

func TestIsoCommitDeleteMissing(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.commitDelete("nope") // node absent -> early return, no undo pushed
	if d.CanUndo() {
		t.Fatal("deleting a missing node pushed an undo entry")
	}
}

func TestIsoNextIDSkipsCollision(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "n1", X: 0, Y: 0}) // occupies the first generated id
	id := d.commitPlace(2, 2)
	if id == "n1" {
		t.Fatalf("nextID collided with existing n1")
	}
}

func TestIsoZoom(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	w0 := d.proj.TileW
	worldBefore := d.proj.Unproject(pt(200, 150), 0)
	d.OnEvent(Event{Kind: EventScroll, X: 200, Y: 150, Delta: -1}) // zoom in
	if d.proj.TileW <= w0 {
		t.Fatalf("zoom in did not enlarge tile: %v -> %v", w0, d.proj.TileW)
	}
	worldAfter := d.proj.Unproject(pt(200, 150), 0)
	if !approx(worldAfter.X, worldBefore.X) || !approx(worldAfter.Y, worldBefore.Y) {
		t.Fatalf("zoom did not keep the anchored world point: %v vs %v", worldBefore, worldAfter)
	}
	w1 := d.proj.TileW
	d.OnEvent(Event{Kind: EventScroll, X: 200, Y: 150, Delta: 1}) // zoom out
	if d.proj.TileW >= w1 {
		t.Fatalf("zoom out did not shrink tile: %v -> %v", w1, d.proj.TileW)
	}
}

func TestIsoZoomClamps(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	for i := 0; i < 200; i++ {
		d.ZoomAt(IsoZoomStep, 200, 200)
	}
	if d.proj.TileW > IsoMaxTile {
		t.Fatalf("tile exceeded max: %v", d.proj.TileW)
	}
	// Already at max: a further zoom-in is a no-op (early return).
	d.proj.TileW = IsoMaxTile
	d.ZoomAt(2, 200, 200)
	if d.proj.TileW != IsoMaxTile {
		t.Fatalf("clamped zoom changed tile to %v", d.proj.TileW)
	}
	for i := 0; i < 200; i++ {
		d.ZoomAt(1/IsoZoomStep, 200, 200)
	}
	if d.proj.TileW < IsoMinTile {
		t.Fatalf("tile went below min: %v", d.proj.TileW)
	}
}

func pt(x, y float64) geoPoint { return geometry.Pt(x, y) }

// --- context menu -------------------------------------------------------

func TestIsoContextMenuOnNode(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 10, Y: 20, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 2})
	n, _ := d.Doc().Node("a")
	px, py := nodeCenterLocal(d, n)
	d.OnEvent(Event{Kind: EventSecondaryClick, X: px, Y: py})
	if !d.menu.Open {
		t.Fatal("secondary click did not open the menu")
	}
	if d.Selected() != "a" {
		t.Fatal("secondary click did not select the node")
	}
	if len(d.menu.Menu.Items) != 1 || d.menu.Menu.Items[0].Label != "Delete" {
		t.Fatalf("menu items = %+v, want [Delete]", d.menu.Menu.Items)
	}
	d.menu.Menu.Items[0].Action() // Delete
	if _, ok := d.Doc().Node("a"); ok {
		t.Fatal("Delete action did not remove the node")
	}
}

func TestIsoContextMenuOnEmpty(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	gx, gy := 3, 5
	sx, sy := localOf(d, iso.V(float64(gx)+0.5, float64(gy)+0.5, 0))
	d.OnEvent(Event{Kind: EventSecondaryClick, X: sx, Y: sy})
	if len(d.menu.Menu.Items) != 1 || d.menu.Menu.Items[0].Label != "Add node" {
		t.Fatalf("menu items = %+v, want [Add node]", d.menu.Menu.Items)
	}
	d.menu.Menu.Items[0].Action()
	nodes := d.Doc().Nodes()
	if len(nodes) != 1 || nodes[0].X != gx || nodes[0].Y != gy {
		t.Fatalf("Add node placed %+v, want one at (%d,%d)", nodes, gx, gy)
	}
}

func TestIsoOpenMenuConsumesInput(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 2})
	n, _ := d.Doc().Node("a")
	px, py := nodeCenterLocal(d, n)
	d.OnEvent(Event{Kind: EventSecondaryClick, X: px, Y: py})
	if !d.menu.Open {
		t.Fatal("menu not open")
	}
	// A click far outside the menu is routed to the menu and dismisses it,
	// rather than reaching the canvas (no new node is placed).
	before := len(d.Doc().Nodes())
	d.OnEvent(Event{Kind: EventClick, X: 399, Y: 399})
	if d.menu.Open {
		t.Fatal("outside click did not close the menu")
	}
	if len(d.Doc().Nodes()) != before {
		t.Fatal("event leaked to the canvas while the menu was open")
	}
}

func TestIsoDrawWithOpenMenu(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.OnEvent(Event{Kind: EventSecondaryClick, X: 100, Y: 100})
	if _, err := RenderImage(d, 400, 400, DefaultLight()); err != nil {
		t.Fatal(err)
	}
}

// --- misc / lifecycle ---------------------------------------------------

func TestIsoDisabledIgnoresEvents(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Disabled = true
	d.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	d.OnEvent(Event{Kind: EventMouseUp, X: 10, Y: 10})
	if len(d.Doc().Nodes()) != 0 {
		t.Fatal("disabled widget still placed a node")
	}
}

func TestIsoInvalidateAndClose(t *testing.T) {
	d := NewIsoDiagram(nil)
	n := 0
	d.OnInvalidate = func() { n++ }
	d.Doc().PutNode(IsoNode{ID: "a"}) // fires via the document subscription
	if n == 0 {
		t.Fatal("document edit did not invalidate")
	}
	d.Close()
	got := n
	d.Doc().PutNode(IsoNode{ID: "b"}) // no longer subscribed
	if n != got {
		t.Fatal("invalidate fired after Close")
	}
	d.Close() // second Close: unsub already nil
}

func TestIsoAccessors(t *testing.T) {
	doc := NewIsoDoc()
	d := NewIsoDiagram(doc)
	if d.Doc() != doc {
		t.Fatal("Doc() mismatch")
	}
	if d.Projection() != d.proj {
		t.Fatal("Projection() mismatch")
	}
	if d.ContextMenu() != d.menu {
		t.Fatal("ContextMenu() mismatch")
	}
}

// --- accessibility ------------------------------------------------------

func TestIsoA11yTree(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 2, Label: "Web"})
	d.Doc().PutNode(IsoNode{ID: "b", X: 4, Y: 4}) // unlabeled -> name falls back to id
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Label: "http"})
	d.Doc().PutConnector(IsoConnector{ID: "ba", From: "b", To: "a"}) // unlabeled connector
	d.setSelected("a")

	if info := d.A11y(); info.Role != RoleGroup || info.Name != "isometric diagram" {
		t.Fatalf("widget A11y = %+v", info)
	}
	nodes := WalkA11y(d)
	// group + 2 node proxies + 2 connector proxies
	if len(nodes) != 5 {
		t.Fatalf("WalkA11y returned %d elements, want 5", len(nodes))
	}
	byName := map[string]A11yInfo{}
	for _, e := range nodes {
		byName[e.Name] = e.A11yInfo
	}
	if byName["Web"].Value != "selected" {
		t.Fatalf("selected node value = %q, want selected", byName["Web"].Value)
	}
	if byName["b"].Value != "" {
		t.Fatalf("unselected node value = %q, want empty", byName["b"].Value)
	}
	if _, ok := byName["http"]; !ok {
		t.Fatal("labeled connector missing from a11y tree")
	}
	if _, ok := byName["b to a"]; !ok {
		t.Fatal("default connector name missing from a11y tree")
	}
}
