// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	stdcolor "image/color"
	"testing"

	"github.com/go-gfx/gfx/iso"
)

// zoneDiagram builds a 400x400 diagram with a single zone "z" at the given
// rectangle.
func zoneDiagram(x, y, w, h int) *IsoDiagram {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutZone(IsoZone{ID: "z", X: x, Y: y, W: w, H: h})
	return d
}

// --- geometry -----------------------------------------------------------

func TestIsoZoneCornersExact(t *testing.T) {
	d := zoneDiagram(2, 3, 4, 5)
	z, _ := d.Doc().Zone("z")
	got := d.zoneCorners(z)
	want := []geoPoint{
		d.proj.Project(iso.V(2, 3, 0)),
		d.proj.Project(iso.V(6, 3, 0)),
		d.proj.Project(iso.V(6, 8, 0)),
		d.proj.Project(iso.V(2, 8, 0)),
	}
	if len(got) != 4 {
		t.Fatalf("zoneCorners returned %d points", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("corner[%d] = %v, want exactly %v", i, got[i], want[i])
		}
	}
}

func TestIsoZoneGridCornerAllFour(t *testing.T) {
	z := IsoZone{X: 2, Y: 3, W: 4, H: 5}
	cases := [][3]int{{0, 2, 3}, {1, 6, 3}, {2, 6, 8}, {3, 2, 8}}
	for _, c := range cases {
		gx, gy := zoneGridCorner(z, c[0])
		if gx != c[1] || gy != c[2] {
			t.Fatalf("zoneGridCorner(%d) = (%d,%d), want (%d,%d)", c[0], gx, gy, c[1], c[2])
		}
	}
}

func TestIsoZoneFillAndBorder(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	// Unset colour: translucent theme accent.
	def := d.zoneFill(IsoZone{}, theme)
	a := theme.Accent
	if def != (RGBA{R: a.R, G: a.G, B: a.B, A: isoZoneDefaultAlpha}) {
		t.Fatalf("default zone fill = %v", def)
	}
	// Explicit colour passes through.
	c := RGBA{R: 10, G: 20, B: 30, A: 100}
	if got := d.zoneFill(IsoZone{Color: c}, theme); got != c {
		t.Fatalf("explicit zone fill = %v, want %v", got, c)
	}
	// Border is the opaque form of the fill.
	if got := zoneBorderRGBA(c); got != (RGBA{R: 10, G: 20, B: 30, A: 255}) {
		t.Fatalf("zone border = %v", got)
	}
}

// --- rendering order: a zone never masks a node standing on it -----------

func TestIsoZoneRendersBehindNode(t *testing.T) {
	theme := DefaultLight()
	red := RGBA{R: 220, G: 0, B: 0, A: 255}
	// Node without a zone.
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "n", X: 3, Y: 3, Color: red})
	imgA, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := d.Doc().Node("n")
	tx, ty := localOf(d, d.nodeAnchor(n))

	// Same node, now covered by a zone spanning the node's cell.
	d.Doc().PutZone(IsoZone{ID: "z", X: 2, Y: 2, W: 4, H: 4})
	imgB, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	// The node's top pixel is unchanged and still the exact node colour: the
	// floor zone did not paint over the node.
	if got := imgB.RGBAAt(tx, ty); got != stdcolor.RGBA(stdColor(red)) {
		t.Fatalf("node top under a zone = %v, want %v (zone masked the node)", got, red)
	}
	if imgA.RGBAAt(tx, ty) != imgB.RGBAAt(tx, ty) {
		t.Fatal("node top pixel changed when a zone was added over it")
	}
	// Yet the zone DID render: adding it changed the ground somewhere.
	diff := 0
	for i := range imgA.Pix {
		if imgA.Pix[i] != imgB.Pix[i] {
			diff++
		}
	}
	if diff == 0 {
		t.Fatal("adding a zone changed nothing in the render")
	}
}

func TestIsoZoneOffSurfaceRastersNothing(t *testing.T) {
	// A zone whose projected quad falls entirely outside a small buffer makes the
	// vector fill/stroke return ok=false — the guarded no-coverage branch.
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	d.Doc().PutZone(IsoZone{ID: "far", X: 5000, Y: 5000, W: 1, H: 1})
	if _, err := RenderImage(d, 40, 40, DefaultLight()); err != nil {
		t.Fatal(err)
	}
}

// --- hit testing --------------------------------------------------------

func TestIsoZoneAtLocalTopmost(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	if _, ok := d.zoneAtLocal(10, 10); ok {
		t.Fatal("empty diagram hit a zone")
	}
	d.Doc().PutZone(IsoZone{ID: "low", X: 1, Y: 1, W: 5, H: 5})
	d.Doc().PutZone(IsoZone{ID: "high", X: 1, Y: 1, W: 5, H: 5}) // drawn later -> on top
	cx, cy := localOf(d, iso.V(3, 3, 0))
	if id, ok := d.zoneAtLocal(cx, cy); !ok || id != "high" {
		t.Fatalf("overlap zone pick = %q ok=%v, want high", id, ok)
	}
	// A point far outside both is a miss.
	if _, ok := d.zoneAtLocal(399, 1); ok {
		t.Fatal("far point hit a zone")
	}
}

func TestIsoZoneHandleAt(t *testing.T) {
	d := zoneDiagram(2, 2, 3, 3)
	// No selection -> never a handle.
	if _, ok := d.zoneHandleAt(0, 0); ok {
		t.Fatal("handle hit with no zone selected")
	}
	d.SelectZone("z")
	// A press exactly on corner 2 (+X+Y) grabs that handle.
	cx, cy := localOf(d, iso.V(5, 5, 0))
	if corner, ok := d.zoneHandleAt(cx, cy); !ok || corner != 2 {
		t.Fatalf("corner-2 handle = %d ok=%v", corner, ok)
	}
	// The zone centre is far from every corner: a miss.
	mx, my := localOf(d, iso.V(3.5, 3.5, 0))
	if _, ok := d.zoneHandleAt(mx, my); ok {
		t.Fatal("zone centre grabbed a handle")
	}
}

// --- creation gesture ---------------------------------------------------

func TestIsoZoneDragCreates(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Mode = IsoModeZone
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	sx, sy := localOf(d, iso.V(2.5, 3.5, 0))
	ex, ey := localOf(d, iso.V(5.5, 6.5, 0))
	d.OnEvent(Event{Kind: EventClick, X: sx, Y: sy})
	d.OnEvent(Event{Kind: EventMouseDrag, X: ex, Y: ey})
	// The in-flight rubber-band preview draws without panicking.
	d.Draw(painterPixel(make([]byte, 4*400*400), 400, 400), DefaultLight())
	d.OnEvent(Event{Kind: EventMouseUp, X: ex, Y: ey})
	zs := d.Doc().Zones()
	if len(zs) != 1 || zs[0].X != 2 || zs[0].Y != 3 || zs[0].W != 4 || zs[0].H != 4 {
		t.Fatalf("drag created %+v, want one zone at (2,3) 4x4", zs)
	}
	if d.SelectedZone() != zs[0].ID {
		t.Fatalf("created zone not selected: %q", d.SelectedZone())
	}
	// Undo removes it.
	d.Undo()
	if len(d.Doc().Zones()) != 0 {
		t.Fatal("undo did not remove the created zone")
	}
}

func TestIsoZoneTapCreatesUnitZone(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Mode = IsoModeZone
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	sx, sy := localOf(d, iso.V(4.5, 4.5, 0))
	d.OnEvent(Event{Kind: EventClick, X: sx, Y: sy})
	d.OnEvent(Event{Kind: EventMouseUp, X: sx, Y: sy}) // no drag -> 1x1
	zs := d.Doc().Zones()
	if len(zs) != 1 || zs[0].W != 1 || zs[0].H != 1 || zs[0].X != 4 || zs[0].Y != 4 {
		t.Fatalf("tap created %+v, want one 1x1 zone at (4,4)", zs)
	}
}

func TestIsoZoneNextIDSkipsCollision(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutZone(IsoZone{ID: "z1", X: 0, Y: 0, W: 1, H: 1}) // occupies the first id
	id := d.commitPlaceZone(2, 2, 3, 3)
	if id == "z1" {
		t.Fatal("nextZoneID collided with existing z1")
	}
}

// --- move / resize gestures ---------------------------------------------

func TestIsoZoneMoveDrag(t *testing.T) {
	d := zoneDiagram(2, 2, 2, 2)
	tx, ty := groundCenterLocal(d, 6, 6)
	sx, sy := groundCenterLocal(d, 3, 3) // a cell inside the zone body
	d.OnEvent(Event{Kind: EventClick, X: sx, Y: sy})
	if d.SelectedZone() != "z" {
		t.Fatalf("press on zone body did not select it: %q", d.SelectedZone())
	}
	d.OnEvent(Event{Kind: EventMouseDrag, X: tx, Y: ty})
	d.OnEvent(Event{Kind: EventMouseUp, X: tx, Y: ty})
	z, _ := d.Doc().Zone("z")
	if z.X != 5 || z.Y != 5 { // moved by (+3,+3)
		t.Fatalf("zone moved to (%d,%d), want (5,5)", z.X, z.Y)
	}
	d.Undo()
	if z, _ := d.Doc().Zone("z"); z.X != 2 || z.Y != 2 {
		t.Fatalf("undo left zone at (%d,%d), want (2,2)", z.X, z.Y)
	}
}

func TestIsoZoneMoveSameCellNoop(t *testing.T) {
	d := zoneDiagram(2, 2, 2, 2)
	sx, sy := groundCenterLocal(d, 3, 3)
	d.OnEvent(Event{Kind: EventClick, X: sx, Y: sy})
	d.OnEvent(Event{Kind: EventMouseDrag, X: sx, Y: sy}) // same cell
	d.OnEvent(Event{Kind: EventMouseUp, X: sx, Y: sy})
	if z, _ := d.Doc().Zone("z"); z.X != 2 || z.Y != 2 {
		t.Fatalf("no-op move shifted zone to (%d,%d)", z.X, z.Y)
	}
}

func TestIsoZoneMoveMissingNoPanic(t *testing.T) {
	d := zoneDiagram(2, 2, 2, 2)
	d.dragZone = "ghost"
	d.moveZoneDragTo(10, 10) // absent -> early return
}

func TestIsoZoneResizeCorner(t *testing.T) {
	d := zoneDiagram(2, 2, 2, 2) // corners 0=(2,2) 1=(4,2) 2=(4,4) 3=(2,4)
	d.SelectZone("z")
	// Grab corner 2 and drag it to grid vertex (6,6); corner 0 stays fixed.
	cx, cy := localOf(d, iso.V(4, 4, 0))
	tx, ty := localOf(d, iso.V(6, 6, 0))
	d.OnEvent(Event{Kind: EventClick, X: cx, Y: cy})
	d.OnEvent(Event{Kind: EventMouseDrag, X: tx, Y: ty})
	d.OnEvent(Event{Kind: EventMouseUp, X: tx, Y: ty})
	z, _ := d.Doc().Zone("z")
	if z.X != 2 || z.Y != 2 || z.W != 4 || z.H != 4 {
		t.Fatalf("resize gave %+v, want (2,2) 4x4", z)
	}
	// Undo restores the original 2x2.
	d.Undo()
	if z, _ := d.Doc().Zone("z"); z.W != 2 || z.H != 2 {
		t.Fatalf("undo left size %dx%d, want 2x2", z.W, z.H)
	}
}

func TestIsoZoneResizeClampsToOneCell(t *testing.T) {
	d := zoneDiagram(2, 2, 3, 3) // corner 2 = (5,5), opposite corner 0 = (2,2)
	d.SelectZone("z")
	cx, cy := localOf(d, iso.V(5, 5, 0))
	// Drag corner 2 onto the fixed corner (2,2): width/height clamp to 1.
	tx, ty := localOf(d, iso.V(2, 2, 0))
	d.OnEvent(Event{Kind: EventClick, X: cx, Y: cy})
	d.OnEvent(Event{Kind: EventMouseDrag, X: tx, Y: ty})
	d.OnEvent(Event{Kind: EventMouseUp, X: tx, Y: ty})
	z, _ := d.Doc().Zone("z")
	if z.W != 1 || z.H != 1 || z.X != 2 || z.Y != 2 {
		t.Fatalf("clamped resize = %+v, want (2,2) 1x1", z)
	}
}

func TestIsoZoneResizeSameRectNoop(t *testing.T) {
	d := zoneDiagram(2, 2, 2, 2)
	d.SelectZone("z")
	d.beginZoneResize(2) // opposite corner 0 = (2,2) pinned
	cx, cy := localOf(d, iso.V(4, 4, 0))
	d.moved = true
	d.resizeZoneDragTo(cx, cy) // corner 2 back onto itself -> no change
	if z, _ := d.Doc().Zone("z"); z.W != 2 || z.H != 2 {
		t.Fatalf("no-op resize changed size to %dx%d", z.W, z.H)
	}
}

func TestIsoZoneResizeMissingNoPanic(t *testing.T) {
	d := zoneDiagram(2, 2, 2, 2)
	d.SelectZone("z")
	d.beginZoneResize(2)
	d.Doc().RemoveZone("z") // vanish mid-gesture
	d.moved = true
	d.resizeZoneDragTo(50, 50) // absent -> early return
}

// --- selection ----------------------------------------------------------

func TestIsoZoneSelectionObservable(t *testing.T) {
	d := zoneDiagram(1, 1, 2, 2)
	d.Doc().PutNode(IsoNode{ID: "n", X: 6, Y: 6})
	d.Doc().PutConnector(IsoConnector{ID: "c", From: "n", To: "n"})
	d.Doc().PutText(IsoText{ID: "tx", X: 8, Y: 8})
	var seen []string
	d.OnSelectZone = func(id string) { seen = append(seen, id) }
	if d.SelectedZone() != "" {
		t.Fatal("fresh widget has a selected zone")
	}
	if d.SelectedZoneObservable() != d.selZone {
		t.Fatal("observable accessor mismatch")
	}
	d.SelectZone("z")
	if d.SelectedZone() != "z" {
		t.Fatalf("SelectedZone = %q", d.SelectedZone())
	}
	// Selecting each of the other three clears the zone selection.
	d.setSelected("n")
	if d.SelectedZone() != "" {
		t.Fatal("node selection did not clear the zone selection")
	}
	d.SelectZone("z")
	d.SelectConnector("c")
	if d.SelectedZone() != "" {
		t.Fatal("connector selection did not clear the zone selection")
	}
	d.SelectZone("z")
	d.SelectText("tx")
	if d.SelectedZone() != "" {
		t.Fatal("text selection did not clear the zone selection")
	}
	if len(seen) < 2 || seen[0] != "z" {
		t.Fatalf("OnSelectZone calls = %v", seen)
	}
}

// --- setters ------------------------------------------------------------

func TestIsoZoneSetters(t *testing.T) {
	d := zoneDiagram(1, 1, 2, 2)
	col := RGBA{R: 9, G: 9, B: 9, A: 200}
	if !d.SetZoneColor("z", col) {
		t.Fatal("colour change reported no-op")
	}
	if d.SetZoneColor("z", col) {
		t.Fatal("redundant colour set reported a change")
	}
	if d.SetZoneColor("nope", col) {
		t.Fatal("colouring a missing zone reported a change")
	}
	if !d.SetZoneLabel("z", "Cluster") {
		t.Fatal("label change no-op")
	}
	if d.SetZoneLabel("z", "Cluster") {
		t.Fatal("redundant label set reported a change")
	}
	if !d.SetZoneRect("z", 3, 4, 5, 6) {
		t.Fatal("rect change no-op")
	}
	if d.SetZoneRect("z", 3, 4, 5, 6) {
		t.Fatal("redundant rect set reported a change")
	}
	// Size clamps to at least 1x1.
	if !d.SetZoneRect("z", 0, 0, -3, 0) {
		t.Fatal("clamped rect reported no change")
	}
	z, _ := d.Doc().Zone("z")
	if z.Color != col || z.Label != "Cluster" || z.W != 1 || z.H != 1 {
		t.Fatalf("zone after edits = %+v", z)
	}
	// Undo the clamp edit.
	d.Undo()
	if z, _ := d.Doc().Zone("z"); z.W != 5 || z.H != 6 {
		t.Fatalf("undo left size %dx%d, want 5x6", z.W, z.H)
	}
}

func TestIsoSetSelectedZone(t *testing.T) {
	d := zoneDiagram(1, 1, 2, 2)
	// Nothing selected -> no-ops.
	if d.SetSelectedZoneColor(RGBA{A: 255}) {
		t.Fatal("coloured with no zone selected")
	}
	if d.SetSelectedZoneLabel("x") {
		t.Fatal("labelled with no zone selected")
	}
	d.SelectZone("z")
	if !d.SetSelectedZoneColor(RGBA{R: 1, G: 2, B: 3, A: 255}) {
		t.Fatal("colour-selected no-op")
	}
	if !d.SetSelectedZoneLabel("grp") {
		t.Fatal("label-selected no-op")
	}
	z, _ := d.Doc().Zone("z")
	if z.Label != "grp" || z.Color.A != 255 {
		t.Fatalf("selected zone edits not applied: %+v", z)
	}
}

// --- delete: key + context menu -----------------------------------------

func TestIsoZoneDeleteKey(t *testing.T) {
	d := zoneDiagram(1, 1, 2, 2)
	d.SelectZone("z")
	d.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	if _, ok := d.Doc().Zone("z"); ok {
		t.Fatal("Delete did not remove the selected zone")
	}
	if d.SelectedZone() != "" {
		t.Fatal("zone selection not cleared after delete")
	}
}

func TestIsoZoneCommitDeleteMissing(t *testing.T) {
	d := zoneDiagram(1, 1, 2, 2)
	before := d.CanUndo()
	d.commitDeleteZone("nope") // absent -> no undo pushed
	if d.CanUndo() != before {
		t.Fatal("deleting a missing zone pushed an undo entry")
	}
}

func TestIsoZoneContextMenu(t *testing.T) {
	d := zoneDiagram(2, 2, 3, 3)
	cx, cy := localOf(d, iso.V(3.5, 3.5, 0))
	d.OnEvent(Event{Kind: EventSecondaryClick, X: cx, Y: cy})
	if d.SelectedZone() != "z" {
		t.Fatal("secondary click did not select the zone")
	}
	if len(d.menu.Menu.Items) != 1 || d.menu.Menu.Items[0].Label != "Delete" {
		t.Fatalf("zone menu = %+v, want [Delete]", d.menu.Menu.Items)
	}
	d.menu.Menu.Items[0].Action()
	if _, ok := d.Doc().Zone("z"); ok {
		t.Fatal("Delete action did not remove the zone")
	}
}

// --- overlay + label rendering ------------------------------------------

func TestIsoZoneOverlayAndLabelRender(t *testing.T) {
	d := zoneDiagram(2, 2, 4, 4)
	d.Doc().PutZone(IsoZone{ID: "z", X: 2, Y: 2, W: 4, H: 4, Label: "Zone", Color: RGBA{R: 12, G: 200, B: 34, A: 120}})
	d.SelectZone("z") // draws outline + handles
	img, err := RenderImage(d, 400, 400, DefaultLight())
	if err != nil {
		t.Fatal(err)
	}
	// The label ink is the opaque form of the fill colour.
	if !hasInk(img.Pix, RGBA{R: 12, G: 200, B: 34, A: 255}) {
		t.Fatal("zone label ink not painted")
	}
	// The selection outline/handles are drawn in the accent colour.
	if !hasInk(img.Pix, DefaultLight().Accent) {
		t.Fatal("selected-zone accent chrome not painted")
	}
}
