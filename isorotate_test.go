// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"image/color"
	"testing"

	"github.com/go-crdt/crdt"
	"github.com/go-gfx/gfx/iso"
)

// handRotXY is an independent re-derivation of the 90°-step rotation about the
// grid centre, used to check the widget's own rotation without calling it.
func handRotXY(cx, cy, x, y float64, q int) (float64, float64) {
	dx, dy := x-cx, y-cy
	switch ((q % 4) + 4) % 4 {
	case 1:
		return cx - dy, cy + dx
	case 2:
		return cx - dx, cy - dy
	case 3:
		return cx + dy, cy - dx
	default:
		return x, y
	}
}

// projLocalRot projects a MODEL point through the widget's current view rotation
// — the rotation-aware pixel a rotated overlay lands on.
func projLocalRot(d *IsoDiagram, v iso.Vec3) (int, int) {
	p := d.project(v)
	return iround(p.X), iround(p.Y)
}

// --- API --------------------------------------------------------------------

func TestIsoViewRotationAPI(t *testing.T) {
	d := NewIsoDiagram(nil)
	if d.ViewRotation() != 0 {
		t.Fatalf("fresh rotation = %d, want 0", d.ViewRotation())
	}
	// Setter, with modulo folding of any integer onto 0..3.
	for _, c := range []struct{ set, want int }{
		{1, 1}, {2, 2}, {3, 3}, {4, 0}, {5, 1}, {-1, 3}, {-4, 0}, {7, 3},
	} {
		d.SetViewRotation(c.set)
		if d.ViewRotation() != c.want {
			t.Fatalf("SetViewRotation(%d) -> %d, want %d", c.set, d.ViewRotation(), c.want)
		}
	}
	// The observable is the live channel and mirrors the getter.
	if d.ViewRotationObservable() != d.viewRot {
		t.Fatal("ViewRotationObservable is not the backing observable")
	}
	d.SetViewRotation(0)
	if d.ViewRotationObservable().Get() != 0 {
		t.Fatalf("observable = %d after reset, want 0", d.ViewRotationObservable().Get())
	}
}

func TestIsoRotateCWCCWCycle(t *testing.T) {
	d := NewIsoDiagram(nil)
	// Four CW turns return to the start, stepping through every orientation.
	for i := 1; i <= 4; i++ {
		d.RotateCW()
		if want := i % 4; d.ViewRotation() != want {
			t.Fatalf("after %d CW turns: %d, want %d", i, d.ViewRotation(), want)
		}
	}
	// CCW is the exact inverse of CW.
	d.RotateCW() // -> 1
	d.RotateCCW()
	if d.ViewRotation() != 0 {
		t.Fatalf("CW then CCW = %d, want 0", d.ViewRotation())
	}
	d.RotateCCW() // 0 -> 3
	if d.ViewRotation() != 3 {
		t.Fatalf("CCW from 0 = %d, want 3", d.ViewRotation())
	}
}

func TestIsoViewRotationRedundantSetAndInvalidate(t *testing.T) {
	d := NewIsoDiagram(nil)
	n := 0
	d.OnInvalidate = func() { n++ }
	d.SetViewRotation(0) // redundant: already 0 -> no notify
	if n != 0 {
		t.Fatalf("redundant SetViewRotation invalidated %d times, want 0", n)
	}
	d.SetViewRotation(1) // real change -> one invalidate
	if n != 1 {
		t.Fatalf("SetViewRotation(1) invalidated %d times, want 1", n)
	}
	d.SetViewRotation(5) // folds to 1, same as current -> no notify
	if n != 1 {
		t.Fatalf("folded redundant set invalidated %d times, want 1", n)
	}
}

// --- transform correctness --------------------------------------------------

func TestIsoRotForwardInverseRoundTrip(t *testing.T) {
	d := NewIsoDiagram(nil) // 10x10 -> centre (5,5)
	pts := []iso.Vec3{iso.V(0, 0, 0), iso.V(1.5, 2.5, 1), iso.V(9, 3, 2), iso.V(5, 5, 0.5)}
	for q := 0; q < 4; q++ {
		d.SetViewRotation(q)
		for _, v := range pts {
			got := d.rotInv(d.rotFwd(v))
			if !vecClose(got, v) {
				t.Fatalf("q=%d rotInv(rotFwd(%v)) = %v", q, v, got)
			}
		}
		// The grid centre is a fixed point of every quarter-turn.
		if c := d.rotFwd(iso.V(5, 5, 0)); !vecClose(c, iso.V(5, 5, 0)) {
			t.Fatalf("q=%d rotated centre = %v, want (5,5,0)", q, c)
		}
	}
}

func TestIsoRotXYConcrete90(t *testing.T) {
	d := NewIsoDiagram(nil) // centre (5,5)
	// A node at (1,2): its anchor XY (1.5,2.5) rotates one quarter to (7.5,1.5).
	x, y := d.rotXY(1.5, 2.5, 1)
	if x != 7.5 || y != 1.5 {
		t.Fatalf("rotXY(1.5,2.5,1) = (%v,%v), want (7.5,1.5)", x, y)
	}
	// 180°: point reflects through the centre.
	x, y = d.rotXY(1.5, 2.5, 2)
	if x != 8.5 || y != 7.5 {
		t.Fatalf("rotXY(1.5,2.5,2) = (%v,%v), want (8.5,7.5)", x, y)
	}
	x, y = d.rotXY(1.5, 2.5, 3)
	if x != 2.5 || y != 8.5 {
		t.Fatalf("rotXY(1.5,2.5,3) = (%v,%v), want (2.5,8.5)", x, y)
	}
}

// TestIsoNodeProjectsExactlyAfterRotation is the toothed assertion that a tile
// (X,Y) node projects to the EXACT pixel the rotation predicts, at every
// orientation, and that its solid re-lands on the matching view tile.
func TestIsoNodeProjectsExactlyAfterRotation(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	cx, cy := d.gridCenter()
	n := IsoNode{ID: "n", X: 1, Y: 2}
	d.Doc().PutNode(n)
	for q := 1; q <= 3; q++ {
		d.SetViewRotation(q)
		// Independent reference: hand-rotate the anchor, then project with the raw
		// projection (no rotation helper involved).
		ax, ay := float64(n.X)+0.5, float64(n.Y)+0.5
		rx, ry := handRotXY(cx, cy, ax, ay, q)
		want := d.proj.Project(iso.V(rx, ry, d.nodeHeight(n)))
		got := d.project(d.nodeAnchor(n))
		if got != want {
			t.Fatalf("q=%d node anchor projects to %v, want %v", q, got, want)
		}
		// The rendered solid's view footprint corner is the rotated cell.
		wantX, wantY := handRotXY(cx, cy, ax, ay, q)
		vx, vy := d.nodeViewCorner(n)
		if vx != wantX-0.5 || vy != wantY-0.5 {
			t.Fatalf("q=%d view corner = (%v,%v), want (%v,%v)", q, vx, vy, wantX-0.5, wantY-0.5)
		}
	}
}

// TestIsoRotationInverseHitTest proves a click at a rotated node's own projected
// position re-selects it, and that the ground cell under the cursor is recovered
// exactly — the inverse transform hit-testing applies.
func TestIsoRotationInverseHitTest(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	n := IsoNode{ID: "n", X: 3, Y: 1}
	d.Doc().PutNode(n)
	for q := 0; q < 4; q++ {
		d.SetViewRotation(q)
		// The node's projected top centre must hit the node.
		hx, hy := projLocalRot(d, d.nodeAnchor(n))
		if id, ok := d.nodeAtLocal(hx, hy); !ok || id != "n" {
			t.Fatalf("q=%d nodeAtLocal at node = %q,%v", q, id, ok)
		}
		// The projected ground centre of its cell unprojects back to that cell.
		gx, gy := projLocalRot(d, iso.V(float64(n.X)+0.5, float64(n.Y)+0.5, 0))
		if cxr, cyr := d.cellAtLocal(gx, gy); cxr != n.X || cyr != n.Y {
			t.Fatalf("q=%d cellAtLocal = (%d,%d), want (%d,%d)", q, cxr, cyr, n.X, n.Y)
		}
	}
}

// TestIsoPlacementExactUnderCursorAfterRotation proves a tap on empty ground
// drops the node on the tile actually under the cursor, whatever the orientation.
func TestIsoPlacementExactUnderCursorAfterRotation(t *testing.T) {
	for q := 0; q < 4; q++ {
		d := NewIsoDiagram(nil)
		d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
		d.SetViewRotation(q)
		tx, ty := 4, 6
		px, py := projLocalRot(d, iso.V(float64(tx)+0.5, float64(ty)+0.5, 0))
		d.OnEvent(Event{Kind: EventClick, X: px, Y: py})
		d.OnEvent(Event{Kind: EventMouseUp, X: px, Y: py})
		nodes := d.Doc().Nodes()
		if len(nodes) != 1 {
			t.Fatalf("q=%d placed %d nodes, want 1", q, len(nodes))
		}
		if nodes[0].X != tx || nodes[0].Y != ty {
			t.Fatalf("q=%d placed at (%d,%d), want (%d,%d)", q, nodes[0].X, nodes[0].Y, tx, ty)
		}
	}
}

// TestIsoDragExactAfterRotation proves a node dragged by one tile's screen step
// moves exactly one model cell in the direction of the cursor, under rotation.
func TestIsoDragExactAfterRotation(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.SetViewRotation(1)
	n := IsoNode{ID: "n", X: 3, Y: 3}
	d.Doc().PutNode(n)
	// Grab at the node's cell ground centre, drag to the neighbouring model tile
	// (4,3)'s ground centre.
	gx, gy := projLocalRot(d, iso.V(3.5, 3.5, 0))
	tx, ty := projLocalRot(d, iso.V(4.5, 3.5, 0))
	d.OnEvent(Event{Kind: EventClick, X: gx, Y: gy})
	d.OnEvent(Event{Kind: EventMouseDrag, X: tx, Y: ty})
	d.OnEvent(Event{Kind: EventMouseUp, X: tx, Y: ty})
	moved, _ := d.Doc().Node("n")
	if moved.X != 4 || moved.Y != 3 {
		t.Fatalf("dragged node at (%d,%d), want (4,3)", moved.X, moved.Y)
	}
}

// TestIsoDepthSortRecomputedAfterRotation proves the painter's-algorithm key that
// orders nodes flips when the plane turns 180°, so front/back is recomputed.
func TestIsoDepthSortRecomputedAfterRotation(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	a := iso.V(0, 0, 0)
	b := iso.V(9, 9, 0)
	d.SetViewRotation(0)
	if !(d.depth(a) < d.depth(b)) {
		t.Fatalf("q=0 expected a behind b: depth(a)=%v depth(b)=%v", d.depth(a), d.depth(b))
	}
	d.SetViewRotation(2)
	if !(d.depth(a) > d.depth(b)) {
		t.Fatalf("q=2 expected a in front of b: depth(a)=%v depth(b)=%v", d.depth(a), d.depth(b))
	}
}

// --- CRDT / model isolation -------------------------------------------------

// TestIsoRotationDoesNotTouchDocument proves the view rotation records no undo
// and leaves a CRDT snapshot byte-identical — it is pure view state.
func TestIsoRotationDoesNotTouchDocument(t *testing.T) {
	cdoc := NewIsoCRDTDocument(crdt.SiteID(1))
	d := NewIsoDiagram(cdoc)
	d.Doc().PutNode(IsoNode{ID: "n", X: 2, Y: 3, Label: "x"})
	d.Doc().PutConnector(IsoConnector{ID: "c", From: "n", To: "n"})
	before := cdoc.Snapshot()

	d.RotateCW()
	d.RotateCW()
	d.SetViewRotation(3)

	after := cdoc.Snapshot()
	if !bytes.Equal(before, after) {
		t.Fatal("view rotation changed the CRDT snapshot")
	}
	if d.CanUndo() {
		t.Fatal("view rotation pushed an undo entry")
	}
}

// TestIsoTwoViewsIndependentRotation proves two views over ONE shared document
// keep independent rotations without diverging the model.
func TestIsoTwoViewsIndependentRotation(t *testing.T) {
	doc := NewIsoDoc()
	v1 := NewIsoDiagram(doc)
	v2 := NewIsoDiagramView(doc)
	v1.Doc().PutNode(IsoNode{ID: "n", X: 1, Y: 1})

	v1.SetViewRotation(1)
	v2.SetViewRotation(3)
	if v1.ViewRotation() != 1 || v2.ViewRotation() != 3 {
		t.Fatalf("rotations = %d,%d, want 1,3", v1.ViewRotation(), v2.ViewRotation())
	}
	// Both views still see the one node — the model never forked.
	if len(v1.Doc().Nodes()) != 1 || len(v2.Doc().Nodes()) != 1 {
		t.Fatal("shared document diverged under independent rotation")
	}
}

// --- control run: rotation 0 is byte-identical ------------------------------

// TestIsoRotationZeroIsIdentity is the control-run assertion: at quarter 0 every
// rotation hook is an exact identity, so the whole render path is byte-for-byte
// the pre-rotation widget.
func TestIsoRotationZeroIsIdentity(t *testing.T) {
	d := NewIsoDiagram(nil) // quarter 0
	pts := []iso.Vec3{iso.V(0, 0, 0), iso.V(1.5, 2.5, 1), iso.V(9, 3, 2)}
	for _, v := range pts {
		if d.rotFwd(v) != v {
			t.Fatalf("rotFwd(%v) = %v at q=0", v, d.rotFwd(v))
		}
		if d.rotInv(v) != v {
			t.Fatalf("rotInv(%v) = %v at q=0", v, d.rotInv(v))
		}
		if d.project(v) != d.proj.Project(v) {
			t.Fatalf("project(%v) diverged from proj.Project at q=0", v)
		}
		if d.depth(v) != d.proj.Depth(v) {
			t.Fatalf("depth(%v) diverged from proj.Depth at q=0", v)
		}
	}
	// Every shape kind passes through rotShape untouched at q=0.
	shapes := []iso.Shape{
		iso.Cube{Pos: iso.V(1, 2, 0), Size: 1, Color: color.RGBA{A: 255}},
		iso.Brick{Pos: iso.V(1, 2, 0), Dim: iso.Dimension{W: 1, H: 0.6, D: 2}},
		iso.Pyramid{Pos: iso.V(3, 4, 0), Dim: iso.Dimension{W: 1, H: 1, D: 1}},
		iso.Slope{Pos: iso.V(0, 0, 0), Dim: iso.Dimension{W: 1, H: 1, D: 1}, Dir: iso.SlopeE},
		iso.Side{Pos: iso.V(2, 2, 0), W: 1, H: 1, Plane: iso.SideYZ},
		iso.Line{From: iso.V(0, 0, 0), To: iso.V(1, 1, 1)},
	}
	for _, sh := range shapes {
		if d.rotShape(sh) != sh {
			t.Fatalf("rotShape(%T) mutated a shape at q=0", sh)
		}
	}
}

// TestIsoRotationZeroRenderStable renders the full widget at quarter 0, turns the
// view through every orientation and back, and proves the quarter-0 buffer is
// unchanged — while the rotated buffers actually differ (rotation is visible).
func TestIsoRotationZeroRenderStable(t *testing.T) {
	theme := DefaultLight()
	build := func() *IsoDiagram {
		d := NewIsoDiagram(nil)
		d.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
		d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1, Label: "A"})
		d.Doc().PutNode(IsoNode{ID: "b", X: 5, Y: 2, Shape: IsoBox, Icon: "server"})
		d.Doc().PutConnector(IsoConnector{ID: "c", From: "a", To: "b", Routed: true, Arrow: IsoArrowSingle})
		d.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 0, W: 3, H: 3, Label: "grp"})
		d.Doc().PutText(IsoText{ID: "t", X: 7, Y: 7, Text: "note"})
		return d
	}
	render := func(d *IsoDiagram) []byte {
		img, err := RenderImage(d, 300, 300, theme)
		if err != nil {
			t.Fatal(err)
		}
		return append([]byte(nil), img.Pix...)
	}

	d := build()
	base := render(d)

	rotated := make(map[int][]byte)
	for q := 1; q <= 3; q++ {
		d.SetViewRotation(q)
		rotated[q] = render(d)
		if bytes.Equal(rotated[q], base) {
			t.Fatalf("q=%d render identical to q=0 (rotation not visible)", q)
		}
	}
	// Back to 0: byte-identical to the first quarter-0 render.
	d.SetViewRotation(0)
	if !bytes.Equal(render(d), base) {
		t.Fatal("quarter-0 render changed after rotating away and back")
	}
	// A freshly built, never-rotated widget renders the same quarter-0 bytes.
	if !bytes.Equal(render(build()), base) {
		t.Fatal("quarter-0 render depends on rotation history")
	}
}

// --- rotShape per primitive -------------------------------------------------

// TestIsoRotShapePrimitives covers every arm of rotShape at a non-zero quarter,
// asserting the exact rotated primitive.
func TestIsoRotShapePrimitives(t *testing.T) {
	d := NewIsoDiagram(nil) // centre (5,5)
	d.SetViewRotation(1)

	// Cube: square footprint, only the position turns.
	cube := d.rotShape(iso.Cube{Pos: iso.V(1, 2, 0), Size: 1}).(iso.Cube)
	if cube.Pos != iso.V(7, 1, 0) || cube.Size != 1 {
		t.Fatalf("rotated cube = %+v", cube)
	}
	// Brick with W!=H: the footprint swaps W and H and re-centres.
	brick := d.rotShape(iso.Brick{Pos: iso.V(1, 2, 0), Dim: iso.Dimension{W: 1, H: 0.6, D: 2}}).(iso.Brick)
	if brick.Dim != (iso.Dimension{W: 0.6, H: 1, D: 2}) {
		t.Fatalf("rotated brick dim = %+v", brick.Dim)
	}
	// centre (1.5,2.3) -> (7.7,1.5); min corner = centre - (0.3,0.5) = (7.4,1.0)
	if !vecClose(brick.Pos, iso.V(7.4, 1.0, 0)) {
		t.Fatalf("rotated brick pos = %+v", brick.Pos)
	}
	// Pyramid rotates like a brick footprint.
	pyr := d.rotShape(iso.Pyramid{Pos: iso.V(3, 4, 0), Dim: iso.Dimension{W: 1, H: 1, D: 1}}).(iso.Pyramid)
	if !vecClose(pyr.Pos, iso.V(5, 3, 0)) {
		t.Fatalf("rotated pyramid pos = %+v", pyr.Pos)
	}
	// Line endpoints each rotate.
	line := d.rotShape(iso.Line{From: iso.V(1.5, 2.5, 1), To: iso.V(5, 5, 0)}).(iso.Line)
	if !vecClose(line.From, iso.V(7.5, 1.5, 1)) || !vecClose(line.To, iso.V(5, 5, 0)) {
		t.Fatalf("rotated line = %+v", line)
	}
	// Slope: footprint turns and the raised edge steps E -> N.
	slope := d.rotShape(iso.Slope{Pos: iso.V(0, 0, 0), Dim: iso.Dimension{W: 1, H: 1, D: 1}, Dir: iso.SlopeE}).(iso.Slope)
	if slope.Dir != iso.SlopeN {
		t.Fatalf("rotated slope dir = %v, want SlopeN", slope.Dir)
	}
	// Side: a +X-facing (YZ) wall spanning +Y turns to an XZ wall.
	side := d.rotShape(iso.Side{Pos: iso.V(2, 2, 0), W: 1, H: 1, Plane: iso.SideYZ}).(iso.Side)
	if side.Plane != iso.SideXZ {
		t.Fatalf("rotated side plane = %v, want SideXZ", side.Plane)
	}
}

// TestIsoRotBoxNoSwapAt180 covers the even-quarter (no W/H swap) arm of rotBox.
func TestIsoRotBoxNoSwapAt180(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetViewRotation(2)
	brick := d.rotShape(iso.Brick{Pos: iso.V(1, 2, 0), Dim: iso.Dimension{W: 1, H: 0.6, D: 2}}).(iso.Brick)
	if brick.Dim != (iso.Dimension{W: 1, H: 0.6, D: 2}) {
		t.Fatalf("q=2 brick dim swapped: %+v", brick.Dim)
	}
	// centre (1.5,2.3) reflects through (5,5) to (8.5,7.7); min corner (8,7.4).
	if !vecClose(brick.Pos, iso.V(8, 7.4, 0)) {
		t.Fatalf("q=2 brick pos = %+v", brick.Pos)
	}
}

// TestIsoRotSlopeDirAllQuarters covers every (direction, quarter) of the slope
// direction rotation.
func TestIsoRotSlopeDirAllQuarters(t *testing.T) {
	seq := []iso.SlopeDir{iso.SlopeE, iso.SlopeN, iso.SlopeW, iso.SlopeS}
	for start := 0; start < 4; start++ {
		for q := 0; q < 4; q++ {
			got := rotSlopeDir(seq[start], q)
			want := seq[(start+q)%4]
			if got != want {
				t.Fatalf("rotSlopeDir(%v,%d) = %v, want %v", seq[start], q, got, want)
			}
		}
	}
}

// TestIsoRotSideSpanAxes covers rotSide for both source planes across quarters,
// checking the rotated span axis (plane) and anchor.
func TestIsoRotSideSpanAxes(t *testing.T) {
	d := NewIsoDiagram(nil)
	// An XZ wall (spans +X) turns to a YZ wall on odd quarters, back to XZ on even.
	d.SetViewRotation(1)
	xz := d.rotSide(iso.Side{Pos: iso.V(1, 1, 0), W: 2, H: 1, Plane: iso.SideXZ})
	if xz.Plane != iso.SideYZ {
		t.Fatalf("q=1 XZ wall -> %v, want SideYZ", xz.Plane)
	}
	d.SetViewRotation(2)
	xz2 := d.rotSide(iso.Side{Pos: iso.V(1, 1, 0), W: 2, H: 1, Plane: iso.SideXZ})
	if xz2.Plane != iso.SideXZ {
		t.Fatalf("q=2 XZ wall -> %v, want SideXZ", xz2.Plane)
	}
	// A YZ wall (spans +Y) turns to XZ on odd quarters.
	d.SetViewRotation(3)
	yz := d.rotSide(iso.Side{Pos: iso.V(1, 1, 0), W: 2, H: 1, Plane: iso.SideYZ})
	if yz.Plane != iso.SideXZ {
		t.Fatalf("q=3 YZ wall -> %v, want SideXZ", yz.Plane)
	}
}

// TestIsoRotationIconNodeRenders drives the full layered + non-layered render
// with icon nodes (primitive shapes AND a sprite) under rotation, exercising
// rotShape through the scene for every built-in icon kind.
func TestIsoRotationIconNodeRenders(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	icons := []string{"server", "cloud", "database", "router", "switch", "storage", "box", "user"}
	for i, ic := range icons {
		d.Doc().PutNode(IsoNode{ID: ic, X: i, Y: 1, Icon: ic})
	}
	// A sprite icon too, so drawSprites' rotated cell rectangle runs.
	sprite := solidSprite(4, 4, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	d.iconRegistry().Register("spr", IsoSpriteIcon{Img: sprite})
	d.Doc().PutNode(IsoNode{ID: "s", X: 2, Y: 5, Icon: "spr"})
	for q := 0; q < 4; q++ {
		d.SetViewRotation(q)
		if _, err := RenderImage(d, 400, 400, theme); err != nil {
			t.Fatalf("q=%d render: %v", q, err)
		}
	}
}

// vecClose reports whether two world points agree to a tight tolerance (the
// rotation is exact for these cases, but float re-centring can leave a last-bit
// residue).
func vecClose(a, b iso.Vec3) bool {
	const eps = 1e-9
	return absf(a.X-b.X) < eps && absf(a.Y-b.Y) < eps && absf(a.Z-b.Z) < eps
}
