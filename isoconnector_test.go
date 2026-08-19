// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	stdcolor "image/color"
	"math"
	"testing"

	"github.com/go-gfx/gfx/geometry"
	"github.com/go-gfx/gfx/iso"
)

// twoNodeConn builds a diagram with nodes a,b at the given cells plus one
// connector "ab" (fields left to the caller to set on the returned connector).
func twoNodeConn(ax, ay, bx, by int) (*IsoDiagram, IsoConnector) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: ax, Y: ay})
	d.Doc().PutNode(IsoNode{ID: "b", X: bx, Y: by})
	c := IsoConnector{ID: "ab", From: "a", To: "b"}
	d.Doc().PutConnector(c)
	return d, c
}

// --- control run: the bare connector is byte-identical to the legacy line ---

// TestIsoConnectorDefaultIsLegacyLine proves that a connector with every
// enrichment field left zero produces EXACTLY the primitive the pre-enrichment
// code produced: a single iso.Line between the two node top anchors, width 2, in
// the theme link colour. This is the control run for byte-for-byte retro-compat.
func TestIsoConnectorDefaultIsLegacyLine(t *testing.T) {
	theme := DefaultLight()
	d, _ := twoNodeConn(1, 4, 6, 4)
	a, _ := d.Doc().Node("a")
	b, _ := d.Doc().Node("b")

	// Legacy formula, reconstructed independently.
	want := iso.Line{
		From:  d.nodeAnchor(a),
		To:    d.nodeAnchor(b),
		Color: stdColor(theme.OnSurface),
		Width: 2,
	}

	shapes := d.scene(theme).Shapes()
	var lines []iso.Line
	for _, s := range shapes {
		if l, ok := s.(iso.Line); ok {
			lines = append(lines, l)
		}
	}
	// The grid contributes many iso.Line shapes; the connector is the only line
	// whose endpoints are at a non-zero Z (the node tops). Find it.
	var got iso.Line
	found := false
	for _, l := range lines {
		if l.From.Z != 0 || l.To.Z != 0 {
			got = l
			found = true
		}
	}
	if !found {
		t.Fatal("no connector line in the scene")
	}
	if got != want {
		t.Fatalf("default connector line = %+v, want legacy %+v", got, want)
	}
}

// TestIsoConnectorZeroEqualsExplicitDefaults renders a diagram with a zero
// connector and the same diagram with the connector's fields set to their
// explicit legacy defaults, and asserts the two images are byte-identical.
func TestIsoConnectorZeroEqualsExplicitDefaults(t *testing.T) {
	theme := DefaultLight()

	d1, _ := twoNodeConn(1, 4, 6, 4)
	img1, err := RenderImage(d1, 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}

	d2, _ := twoNodeConn(1, 4, 6, 4)
	d2.Doc().PutConnector(IsoConnector{
		ID: "ab", From: "a", To: "b",
		Style: IsoSolid, Arrow: IsoArrowNone, Width: 0, Routed: false,
	})
	img2, err := RenderImage(d2, 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}

	for i := range img1.Pix {
		if img1.Pix[i] != img2.Pix[i] {
			t.Fatalf("byte %d differs: zero=%d explicit-default=%d", i, img1.Pix[i], img2.Pix[i])
		}
	}
}

// TestIsoConnectorColorAndWidth checks the per-connector colour and the scaled
// width reach the scene primitive exactly.
func TestIsoConnectorColorAndWidth(t *testing.T) {
	theme := DefaultLight()
	d, _ := twoNodeConn(1, 4, 6, 4)
	red := RGBA{R: 200, G: 10, B: 10, A: 255}
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Color: red, Width: 5})

	for _, s := range d.scene(theme).Shapes() {
		l, ok := s.(iso.Line)
		if !ok || (l.From.Z == 0 && l.To.Z == 0) {
			continue
		}
		if l.Color != stdColor(red) {
			t.Fatalf("connector colour = %v, want %v", l.Color, red)
		}
		if l.Width != 5 {
			t.Fatalf("connector width = %v, want 5", l.Width)
		}
		return
	}
	t.Fatal("connector line not found")
}

func TestIsoConnInkDefaultsToLink(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	if got := d.connInk(IsoConnector{}, theme); got != theme.OnSurface {
		t.Fatalf("unset ink = %v, want link %v", got, theme.OnSurface)
	}
	c := RGBA{R: 9, G: 8, B: 7, A: 255}
	if got := d.connInk(IsoConnector{Color: c}, theme); got != c {
		t.Fatalf("explicit ink = %v, want %v", got, c)
	}
}

func TestIsoConnWidthDefault(t *testing.T) {
	d := NewIsoDiagram(nil)
	if got := d.connWidth(IsoConnector{}); got != 2 {
		t.Fatalf("default width = %v, want 2", got)
	}
	if got := d.connWidth(IsoConnector{Width: 7}); got != 7 {
		t.Fatalf("explicit width = %v, want 7", got)
	}
}

// --- routing ------------------------------------------------------------

func TestIsoFaceAnchorSides(t *testing.T) {
	d := NewIsoDiagram(nil)
	n := IsoNode{X: 2, Y: 3} // default cube: height 1, so face centre at z = 0.5
	cases := []struct {
		dx, dy float64
		want   iso.Vec3
	}{
		{+1, 0, iso.V(3, 3.5, 0.5)},   // +X face centre
		{-1, 0, iso.V(2, 3.5, 0.5)},   // -X face centre
		{0, +1, iso.V(2.5, 4, 0.5)},   // +Y face centre
		{0, -1, iso.V(2.5, 3, 0.5)},   // -Y face centre
		{0.4, -1, iso.V(2.5, 3, 0.5)}, // |dy| dominates -> -Y
	}
	for _, c := range cases {
		if got := d.faceAnchor(n, c.dx, c.dy); got != c.want {
			t.Fatalf("faceAnchor(%v,%v) = %v, want %v", c.dx, c.dy, got, c.want)
		}
	}
}

func TestIsoRouteTilesElbow(t *testing.T) {
	d := NewIsoDiagram(nil)
	a := IsoNode{X: 0, Y: 0}
	b := IsoNode{X: 4, Y: 3} // dx>dy -> a leaves +X, b entered from -X
	path := d.routeTiles(a, b)
	// a's +X face centre (1,0.5,0.5), elbow at (b.X face, a.Y) at the leaving
	// height, b's -X face centre (4,3.5,0.5) — both cubes, so the L runs level.
	want := []iso.Vec3{iso.V(1, 0.5, 0.5), iso.V(4, 0.5, 0.5), iso.V(4, 3.5, 0.5)}
	if len(path) != len(want) {
		t.Fatalf("route = %v, want %v", path, want)
	}
	for i := range want {
		if path[i] != want[i] {
			t.Fatalf("route[%d] = %v, want %v", i, path[i], want[i])
		}
	}
}

func TestIsoRouteTilesVerticalFirst(t *testing.T) {
	d := NewIsoDiagram(nil)
	a := IsoNode{X: 0, Y: 0}
	b := IsoNode{X: 1, Y: 5} // dy>dx -> a leaves +Y first
	path := d.routeTiles(a, b)
	// a's +Y face centre (0.5,1,0.5), elbow at (a.X, b.Y face) = (0.5,5) at the
	// leaving height, b's -Y face centre (1.5,5,0.5).
	want := []iso.Vec3{iso.V(0.5, 1, 0.5), iso.V(0.5, 5, 0.5), iso.V(1.5, 5, 0.5)}
	for i := range want {
		if path[i] != want[i] {
			t.Fatalf("route[%d] = %v, want %v", i, path[i], want[i])
		}
	}
}

func TestIsoRouteTilesAlignedCollapsesElbow(t *testing.T) {
	d := NewIsoDiagram(nil)
	// Same row: a at (0,0), b at (3,0). a leaves +X at (1,0.5,0.5), b entered from
	// -X at (3,0.5,0.5); the elbow (3,0.5,0.5) coincides with b -> deduped to two
	// points, both at the cube face-centre height.
	a := IsoNode{X: 0, Y: 0}
	b := IsoNode{X: 3, Y: 0}
	path := d.routeTiles(a, b)
	if len(path) != 2 {
		t.Fatalf("aligned route = %v, want 2 points", path)
	}
	if path[0] != iso.V(1, 0.5, 0.5) || path[1] != iso.V(3, 0.5, 0.5) {
		t.Fatalf("aligned route endpoints = %v", path)
	}
}

func TestIsoDedupPath(t *testing.T) {
	in := []iso.Vec3{iso.V(0, 0, 0), iso.V(0, 0, 0), iso.V(1, 0, 0), iso.V(1, 0, 0)}
	got := dedupPath(in)
	if len(got) != 2 || got[0] != iso.V(0, 0, 0) || got[1] != iso.V(1, 0, 0) {
		t.Fatalf("dedup = %v", got)
	}
}

func TestIsoConnectorPathModes(t *testing.T) {
	d, _ := twoNodeConn(1, 1, 5, 1)
	a, _ := d.Doc().Node("a")
	b, _ := d.Doc().Node("b")

	// Unrouted: two top anchors.
	straight, ok := d.connectorPath(IsoConnector{From: "a", To: "b"})
	if !ok || len(straight) != 2 ||
		straight[0] != d.nodeAnchor(a) || straight[1] != d.nodeAnchor(b) {
		t.Fatalf("unrouted path = %v ok=%v", straight, ok)
	}
	// Routed: ground route.
	routed, ok := d.connectorPath(IsoConnector{From: "a", To: "b", Routed: true})
	if !ok || len(routed) != 2 { // same row -> collapsed elbow
		t.Fatalf("routed path = %v ok=%v", routed, ok)
	}
	// The routed path now attaches at the face CENTRE, so both endpoints sit at
	// half the (cube) node height, not on the ground.
	if routed[0].Z != 0.5 {
		t.Fatalf("routed path not at the face centre height: %v", routed)
	}
	// Dangling: endpoint missing.
	if _, ok := d.connectorPath(IsoConnector{From: "a", To: "ghost"}); ok {
		t.Fatal("dangling connector returned a path")
	}
	if _, ok := d.connectorPath(IsoConnector{From: "ghost", To: "b"}); ok {
		t.Fatal("dangling connector (source) returned a path")
	}
}

// A routed connector between diagonal nodes has a real elbow (3 points) — draw
// it and confirm the mid vertex is where the label anchors.
func TestIsoRoutedConnectorRendersAndPicks(t *testing.T) {
	d, _ := twoNodeConn(0, 0, 4, 3)
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Routed: true, Style: IsoDashed})
	if _, err := RenderImage(d, 500, 400, DefaultLight()); err != nil {
		t.Fatal(err)
	}
	// The route has an elbow; picking near its midpoint selects it.
	c, _ := d.connectorByID("ab")
	pts, _ := d.connectorPath(c)
	scr := d.projPath(pts)
	mid := polylineMidpoint(scr)
	if id, ok := d.connectorAtLocal(iround(mid.X), iround(mid.Y)); !ok || id != "ab" {
		t.Fatalf("pick at routed midpoint = %q ok=%v", id, ok)
	}
}

// A dangling connector (an endpoint missing) is skipped by both the scene
// assembly and the painter-space overlay pass — rendering one must not panic and
// must exercise the skip branch in each.
func TestIsoDanglingConnectorSkippedOnRender(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.Doc().PutConnector(IsoConnector{ID: "dangling", From: "a", To: "gone", Label: "x", Arrow: IsoArrowSingle})
	if _, err := RenderImage(d, 400, 400, DefaultLight()); err != nil {
		t.Fatal(err)
	}
}

// --- stroke styles ------------------------------------------------------

func TestIsoDashSpec(t *testing.T) {
	if _, _, dashed := dashSpec(IsoSolid); dashed {
		t.Fatal("solid reported dashed")
	}
	if dash, gap, dashed := dashSpec(IsoDashed); !dashed || dash <= 0 || gap <= 0 {
		t.Fatalf("dashed spec = %v,%v,%v", dash, gap, dashed)
	}
	if dash, gap, dashed := dashSpec(IsoDotted); !dashed || dash <= 0 || gap <= 0 {
		t.Fatalf("dotted spec = %v,%v,%v", dash, gap, dashed)
	}
}

func TestIsoDashedProducesMultipleGappedSegments(t *testing.T) {
	theme := DefaultLight()
	d, _ := twoNodeConn(1, 4, 8, 4)
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Style: IsoDashed})

	var dashLines int
	for _, s := range d.scene(theme).Shapes() {
		l, ok := s.(iso.Line)
		if !ok || (l.From.Z == 0 && l.To.Z == 0) {
			continue // skip grid lines (z==0)
		}
		dashLines++
	}
	if dashLines < 2 {
		t.Fatalf("dashed connector made %d segments, want several", dashLines)
	}

	// A solid connector between the same nodes contributes exactly one segment.
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Style: IsoSolid})
	var solidLines int
	for _, s := range d.scene(theme).Shapes() {
		if l, ok := s.(iso.Line); ok && (l.From.Z != 0 || l.To.Z != 0) {
			solidLines++
		}
	}
	if solidLines != 1 {
		t.Fatalf("solid connector made %d segments, want 1", solidLines)
	}
}

func TestIsoAddConnectorSegmentDegenerate(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	sc := iso.NewScene(d.proj)
	// A dashed segment of zero screen length adds nothing (guards div-by-zero).
	v := iso.V(2, 2, 1)
	d.addConnectorSegment(sc, v, v, stdcolor.RGBA{A: 255}, 2, IsoDashed)
	if len(sc.Shapes()) != 0 {
		t.Fatalf("degenerate dashed segment added %d shapes", len(sc.Shapes()))
	}
}

func TestIsoLerp3(t *testing.T) {
	got := lerp3(iso.V(0, 0, 0), iso.V(2, 4, 6), 0.5)
	if got != iso.V(1, 2, 3) {
		t.Fatalf("lerp3 = %v", got)
	}
}

// --- geometry helpers ---------------------------------------------------

func TestIsoDistToSegment(t *testing.T) {
	a, b := geometry.Pt(0, 0), geometry.Pt(10, 0)
	// On the segment (t in [0,1]): perpendicular distance squared.
	if got := distToSegment(5, 3, a, b); math.Abs(got-9) > 1e-9 {
		t.Fatalf("mid dist^2 = %v, want 9", got)
	}
	// Before a (t<0): clamps to a.
	if got := distToSegment(-4, 0, a, b); math.Abs(got-16) > 1e-9 {
		t.Fatalf("before-start dist^2 = %v, want 16", got)
	}
	// After b (t>1): clamps to b.
	if got := distToSegment(13, 0, a, b); math.Abs(got-9) > 1e-9 {
		t.Fatalf("after-end dist^2 = %v, want 9", got)
	}
	// Degenerate segment (a==b): distance to the point.
	if got := distToSegment(3, 4, a, a); math.Abs(got-25) > 1e-9 {
		t.Fatalf("degenerate dist^2 = %v, want 25", got)
	}
}

func TestIsoPolylineMidpoint(t *testing.T) {
	// Even: midpoint of the two central points.
	two := []geometry.Point{geometry.Pt(0, 0), geometry.Pt(10, 4)}
	if got := polylineMidpoint(two); got != geometry.Pt(5, 2) {
		t.Fatalf("2-pt midpoint = %v, want (5,2)", got)
	}
	// Odd: the central vertex (the elbow).
	three := []geometry.Point{geometry.Pt(0, 0), geometry.Pt(6, 1), geometry.Pt(6, 9)}
	if got := polylineMidpoint(three); got != geometry.Pt(6, 1) {
		t.Fatalf("3-pt midpoint = %v, want the elbow (6,1)", got)
	}
}

func TestIsoArrowBarbs(t *testing.T) {
	// Rightward arrow: tip at (10,0), from (0,0). Barbs splay back-left,
	// symmetric about the x axis.
	lx, ly, rx, ry := arrowBarbs(10, 0, 0, 0)
	if lx != rx {
		t.Fatalf("barbs not symmetric in x: %d vs %d", lx, rx)
	}
	if lx >= 10 {
		t.Fatalf("barb x %d not behind the tip (10)", lx)
	}
	if ly != -ry {
		t.Fatalf("barbs not mirrored in y: %d vs %d", ly, ry)
	}
	if ly == 0 {
		t.Fatal("barbs collapsed onto the axis")
	}
	// Zero-length direction: both barbs sit on the tip.
	zlx, zly, zrx, zry := arrowBarbs(4, 4, 4, 4)
	if zlx != 4 || zly != 4 || zrx != 4 || zry != 4 {
		t.Fatalf("degenerate barbs = (%d,%d)(%d,%d), want the tip", zlx, zly, zrx, zry)
	}
}

// --- arrow heads / labels / highlight drawing ---------------------------

// pixelPresent reports whether any pixel in the image is (close to) ink.
func hasInk(pix []byte, ink RGBA) bool {
	for i := 0; i+3 < len(pix); i += 4 {
		if pix[i] == ink.R && pix[i+1] == ink.G && pix[i+2] == ink.B && pix[i+3] == ink.A {
			return true
		}
	}
	return false
}

func TestIsoArrowHeadDrawsAtTarget(t *testing.T) {
	theme := DefaultLight()
	d, _ := twoNodeConn(1, 4, 6, 4)
	red := RGBA{R: 220, G: 0, B: 0, A: 255}
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Arrow: IsoArrowSingle, Color: red})

	img, err := RenderImage(d, 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	// The exact barb endpoints must be painted the connector colour.
	b, _ := d.Doc().Node("b")
	a, _ := d.Doc().Node("a")
	tip := d.proj.Project(d.nodeAnchor(b))
	from := d.proj.Project(d.nodeAnchor(a))
	lx, ly, rx, ry := arrowBarbs(iround(tip.X), iround(tip.Y), iround(from.X), iround(from.Y))
	for _, p := range [][2]int{{lx, ly}, {rx, ry}} {
		if got := img.RGBAAt(p[0], p[1]); got != stdcolor.RGBA(stdColor(red)) {
			t.Fatalf("barb pixel at %v = %v, want %v", p, got, red)
		}
	}
}

func TestIsoDoubleArrowDrawsBothEnds(t *testing.T) {
	theme := DefaultLight()
	d, _ := twoNodeConn(1, 4, 6, 4)
	red := RGBA{R: 220, G: 0, B: 0, A: 255}
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Arrow: IsoArrowDouble, Color: red})
	img, err := RenderImage(d, 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := d.Doc().Node("a")
	b, _ := d.Doc().Node("b")
	// Head at the source end too: barb near a's anchor, oriented from b.
	tip := d.proj.Project(d.nodeAnchor(a))
	from := d.proj.Project(d.nodeAnchor(b))
	lx, ly, _, _ := arrowBarbs(iround(tip.X), iround(tip.Y), iround(from.X), iround(from.Y))
	if got := img.RGBAAt(lx, ly); got != stdcolor.RGBA(stdColor(red)) {
		t.Fatalf("source-end barb = %v, want %v", got, red)
	}
}

func TestIsoArrowNoneDrawsNoHead(t *testing.T) {
	// A short path (single point) draws no head; and IsoArrowNone draws none.
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	buf := make([]byte, 4*400*400)
	p := painterPixel(buf, 400, 400)
	// len(scr)<2 branch:
	d.drawArrowHeads(p, d.Bounds(), []geometry.Point{geometry.Pt(1, 1)}, IsoArrowSingle, RGBA{A: 255})
	// arrow None branch:
	d.drawArrowHeads(p, d.Bounds(), []geometry.Point{geometry.Pt(1, 1), geometry.Pt(2, 2)}, IsoArrowNone, RGBA{A: 255})
}

func TestIsoConnectorLabelDrawnAlongPath(t *testing.T) {
	theme := DefaultLight()
	d, _ := twoNodeConn(1, 4, 6, 4)
	ink := RGBA{R: 3, G: 200, B: 3, A: 255}
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Label: "http", Color: ink})
	img, err := RenderImage(d, 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	// The label glyphs are drawn in the connector ink; some pixel must carry it.
	if !hasInk(img.Pix, ink) {
		t.Fatal("connector label ink not present on the surface")
	}
}

func TestIsoSelectedConnectorHighlight(t *testing.T) {
	theme := DefaultLight()
	d, _ := twoNodeConn(1, 4, 6, 4)
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b"})
	d.SelectConnector("ab")
	img, err := RenderImage(d, 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	// The highlight strokes the path in the accent colour.
	if !hasInk(img.Pix, theme.Accent) {
		t.Fatal("selected-connector highlight (accent) not painted")
	}
}

// --- selection + interactions -------------------------------------------

func TestIsoConnectorSelectionObservable(t *testing.T) {
	d, _ := twoNodeConn(1, 4, 6, 4)
	var seen []string
	d.OnSelectConnector = func(id string) { seen = append(seen, id) }
	if d.SelectedConnector() != "" {
		t.Fatal("fresh widget has a selected connector")
	}
	if d.SelectedConnectorObservable() != d.selConn {
		t.Fatal("observable accessor mismatch")
	}
	d.SelectConnector("ab")
	if d.SelectedConnector() != "ab" {
		t.Fatalf("SelectedConnector = %q", d.SelectedConnector())
	}
	// Selecting a node clears the connector selection.
	d.setSelected("a")
	if d.SelectedConnector() != "" {
		t.Fatal("node selection did not clear the connector selection")
	}
	if len(seen) != 2 || seen[0] != "ab" || seen[1] != "" {
		t.Fatalf("OnSelectConnector calls = %v", seen)
	}
}

func TestIsoClickSelectsConnector(t *testing.T) {
	d, _ := twoNodeConn(1, 4, 6, 4)
	c, _ := d.connectorByID("ab")
	pts, _ := d.connectorPath(c)
	scr := d.projPath(pts)
	mid := polylineMidpoint(scr)
	d.OnEvent(Event{Kind: EventClick, X: iround(mid.X), Y: iround(mid.Y)})
	if d.SelectedConnector() != "ab" {
		t.Fatalf("click on the connector selected %q", d.SelectedConnector())
	}
	if d.Selected() != "" {
		t.Fatal("selecting a connector left a node selected")
	}
	// Release does nothing harmful (gesture is None).
	d.OnEvent(Event{Kind: EventMouseUp, X: iround(mid.X), Y: iround(mid.Y)})

	// A press on truly empty ground clears the connector selection.
	d.OnEvent(Event{Kind: EventClick, X: 480, Y: 20})
	if d.SelectedConnector() != "" {
		t.Fatal("empty press did not clear the connector selection")
	}
}

func TestIsoConnectorAtLocalMiss(t *testing.T) {
	d, _ := twoNodeConn(1, 4, 6, 4)
	// A point far from any connector path is a miss.
	if id, ok := d.connectorAtLocal(490, 10); ok {
		t.Fatalf("far click hit connector %q", id)
	}
	// No connectors at all: also a miss (bestID stays empty).
	empty := NewIsoDiagram(nil)
	empty.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	if _, ok := empty.connectorAtLocal(10, 10); ok {
		t.Fatal("empty diagram hit a connector")
	}
	// A dangling connector (missing endpoint) is not drawn, so not pickable.
	empty.Doc().PutNode(IsoNode{ID: "x", X: 1, Y: 1})
	empty.Doc().PutConnector(IsoConnector{ID: "dangling", From: "x", To: "gone"})
	if _, ok := empty.connectorAtLocal(10, 10); ok {
		t.Fatal("dangling connector was pickable")
	}
}

// --- setters ------------------------------------------------------------

func TestIsoConnectorByID(t *testing.T) {
	d, _ := twoNodeConn(1, 1, 2, 2)
	if _, ok := d.connectorByID("ab"); !ok {
		t.Fatal("existing connector not found")
	}
	if _, ok := d.connectorByID("nope"); ok {
		t.Fatal("missing connector reported found")
	}
}

func TestIsoSetConnectorStyle(t *testing.T) {
	d, _ := twoNodeConn(1, 1, 2, 2)
	if !d.SetConnectorStyle("ab", IsoDashed) {
		t.Fatal("style change reported no-op")
	}
	c, _ := d.connectorByID("ab")
	if c.Style != IsoDashed {
		t.Fatalf("style = %v, want dashed", c.Style)
	}
	// Setting the same style again is a no-op (no undo pushed).
	before := len(d.undo)
	if d.SetConnectorStyle("ab", IsoDashed) {
		t.Fatal("redundant style set reported a change")
	}
	if len(d.undo) != before {
		t.Fatal("redundant set pushed an undo entry")
	}
	// A missing connector is a no-op.
	if d.SetConnectorStyle("nope", IsoDashed) {
		t.Fatal("styling a missing connector reported a change")
	}
	// Undo restores the solid style.
	d.Undo()
	c, _ = d.connectorByID("ab")
	if c.Style != IsoSolid {
		t.Fatalf("undo left style %v, want solid", c.Style)
	}
}

func TestIsoSetConnectorArrowColorRouted(t *testing.T) {
	d, _ := twoNodeConn(1, 1, 4, 3)
	if !d.SetConnectorArrow("ab", IsoArrowSingle) {
		t.Fatal("arrow change no-op")
	}
	if d.SetConnectorArrow("ab", IsoArrowSingle) {
		t.Fatal("redundant arrow set reported a change")
	}
	red := RGBA{R: 1, G: 2, B: 3, A: 255}
	if !d.SetConnectorColor("ab", red) {
		t.Fatal("colour change no-op")
	}
	if d.SetConnectorColor("ab", red) {
		t.Fatal("redundant colour set reported a change")
	}
	if !d.SetConnectorRouted("ab", true) {
		t.Fatal("routed change no-op")
	}
	if d.SetConnectorRouted("ab", true) {
		t.Fatal("redundant routed set reported a change")
	}
	c, _ := d.connectorByID("ab")
	if c.Arrow != IsoArrowSingle || c.Color != red || !c.Routed {
		t.Fatalf("connector after edits = %+v", c)
	}
}

func TestIsoSetSelectedConnector(t *testing.T) {
	d, _ := twoNodeConn(1, 1, 4, 3)
	// Nothing selected -> the selected-wrappers are no-ops.
	if d.SetSelectedConnectorStyle(IsoDotted) {
		t.Fatal("styled with no selection")
	}
	d.SelectConnector("ab")
	if !d.SetSelectedConnectorStyle(IsoDotted) {
		t.Fatal("style-selected no-op")
	}
	if !d.SetSelectedConnectorArrow(IsoArrowDouble) {
		t.Fatal("arrow-selected no-op")
	}
	if !d.SetSelectedConnectorColor(RGBA{R: 5, G: 5, B: 5, A: 255}) {
		t.Fatal("colour-selected no-op")
	}
	if !d.SetSelectedConnectorRouted(true) {
		t.Fatal("routed-selected no-op")
	}
	c, _ := d.connectorByID("ab")
	if c.Style != IsoDotted || c.Arrow != IsoArrowDouble || !c.Routed {
		t.Fatalf("selected edits not applied: %+v", c)
	}
}

// Undo of a connector-create clears a selection pointing at the vanished
// connector; undo of a mere field edit keeps the (still-present) selection.
func TestIsoUndoClearsVanishedConnectorSelection(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.Doc().PutNode(IsoNode{ID: "b", X: 4, Y: 1})
	d.commitConnect("a", "b")
	c := d.Doc().Connectors()[0]
	d.SelectConnector(c.ID)

	// A field edit keeps the selection (the connector still exists after undo).
	d.SetConnectorStyle(c.ID, IsoDashed)
	d.Undo()
	if d.SelectedConnector() != c.ID {
		t.Fatalf("undo of a field edit dropped the selection: %q", d.SelectedConnector())
	}

	// Undo of the create removes the connector -> selection clears.
	d.Undo()
	if len(d.Doc().Connectors()) != 0 {
		t.Fatal("connector not removed by undo")
	}
	if d.SelectedConnector() != "" {
		t.Fatalf("selection kept a vanished connector: %q", d.SelectedConnector())
	}
}
