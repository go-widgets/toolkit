// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package isodemo

import (
	"bytes"
	"image"
	stdcolor "image/color"
	_ "image/png" // registers the PNG decoder for image.DecodeConfig
	"math"
	"os"
	"path/filepath"
	"testing"

	gfxcolor "github.com/go-gfx/gfx/color"
	"github.com/go-gfx/gfx/iso"

	"github.com/go-widgets/toolkit"
)

// --- pixel helpers -------------------------------------------------------

// std converts a toolkit colour to the image/color.RGBA the rendered buffer
// reports, so a rendered pixel can be compared to an expected colour exactly.
func std(c toolkit.RGBA) stdcolor.RGBA { return stdcolor.RGBA{R: c.R, G: c.G, B: c.B, A: c.A} }

// screenOf projects a world point through d's live projection to the exact
// buffer pixel it lands on — the same rounding the widget's own overlay drawing
// uses — so a test can name a pixel by the world coordinate that produced it.
func screenOf(d *toolkit.IsoDiagram, x, y, z float64) (int, int) {
	p := d.Projection().Project(iso.V(x, y, z))
	return int(math.Round(p.X)), int(math.Round(p.Y))
}

// faceColors are the three visible-face colours an isometric solid of base
// colour c is shaded into: the top at factor 1 (so exactly c), the +Y (left)
// and +X (right) sides progressively darker.
func faceColors(c toolkit.RGBA) []stdcolor.RGBA {
	s := std(c)
	return []stdcolor.RGBA{
		s,
		gfxcolor.Shade(s, iso.DefaultShading.Left),
		gfxcolor.Shade(s, iso.DefaultShading.Right),
	}
}

// isFace reports whether pixel c is one of set's colours.
func isFace(c stdcolor.RGBA, set []stdcolor.RGBA) bool {
	for _, w := range set {
		if c == w {
			return true
		}
	}
	return false
}

// render sizes d to the canvas and captures it to an in-memory image.
func render(t *testing.T, d *toolkit.IsoDiagram) *image.RGBA {
	t.Helper()
	d.SetBounds(toolkit.Rect{X: 0, Y: 0, W: CanvasWidth, H: CanvasHeight})
	img, err := toolkit.RenderImage(d, CanvasWidth, CanvasHeight, Theme())
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
	return img
}

// cubeCorners returns the eight world vertices of a unit cube (height 1) placed
// at grid cell (x,y): the projected extreme of these bounds the drawn
// silhouette.
func cubeCorners(x, y int) []iso.Vec3 {
	fx, fy := float64(x), float64(y)
	var out []iso.Vec3
	for _, z := range []float64{0, 1} {
		out = append(out,
			iso.V(fx, fy, z), iso.V(fx+1, fy, z),
			iso.V(fx+1, fy+1, z), iso.V(fx, fy+1, z))
	}
	return out
}

// --- exact-position pixel assertions ------------------------------------

// TestNodeTopAndSideFacesExact pins a bare cube node to its projected screen
// position: the pixel at the node's top-face centre is EXACTLY its base colour
// (the isometric top-shade factor is 1), and interior pixels of the two visible
// side faces are exactly the base colour shaded by the fixed left / right
// factors. Only a node painted precisely where iso.Project places it can satisfy
// all three at once.
func TestNodeTopAndSideFacesExact(t *testing.T) {
	col := toolkit.RGB(220, 40, 40)
	doc := toolkit.NewIsoDoc()
	doc.PutNode(toolkit.IsoNode{ID: "n", X: 4, Y: 4, Color: col})
	d := toolkit.NewIsoDiagram(doc)
	img := render(t, d)

	// Top face centre — world (4.5, 4.5, 1) — is exactly the base colour.
	tx, ty := screenOf(d, 4.5, 4.5, 1)
	if got := img.RGBAAt(tx, ty); got != std(col) {
		t.Fatalf("top-face centre pixel = %v, want exact base %v", got, std(col))
	}
	// +X (right) side interior — world (5, 4.5, 0.35).
	rx, ry := screenOf(d, 5, 4.5, 0.35)
	if got, want := img.RGBAAt(rx, ry), gfxcolor.Shade(std(col), iso.DefaultShading.Right); got != want {
		t.Fatalf("+X side pixel = %v, want right-shade %v", got, want)
	}
	// +Y (left) side interior — world (4.5, 5, 0.35).
	lx, ly := screenOf(d, 4.5, 5, 0.35)
	if got, want := img.RGBAAt(lx, ly), gfxcolor.Shade(std(col), iso.DefaultShading.Left); got != want {
		t.Fatalf("+Y side pixel = %v, want left-shade %v", got, want)
	}
}

// TestNodeSilhouetteBoundsExact pins the node's EXTENT: sampled a few pixels
// outside the projected silhouette bounding box the buffer is bare theme
// surface (nothing bleeds past the solid), while just inside the top diamond it
// is the node colour. Sampling several pixels clear of the ~1px anti-aliased
// seam keeps the check exact and robust.
func TestNodeSilhouetteBoundsExact(t *testing.T) {
	col := toolkit.RGB(30, 90, 210)
	doc := toolkit.NewIsoDoc()
	doc.PutNode(toolkit.IsoNode{ID: "n", X: 4, Y: 4, Color: col})
	d := toolkit.NewIsoDiagram(doc)
	surface := std(Theme().Surface)
	img := render(t, d)

	// Bounding box of the eight projected cube corners.
	minX, minY := math.MaxInt, math.MaxInt
	maxX, maxY := math.MinInt, math.MinInt
	for _, v := range cubeCorners(4, 4) {
		p := d.Projection().Project(v)
		px, py := int(math.Round(p.X)), int(math.Round(p.Y))
		minX, maxX = min(minX, px), max(maxX, px)
		minY, maxY = min(minY, py), max(maxY, py)
	}
	midY := (minY + maxY) / 2
	midX := (minX + maxX) / 2
	// Just outside each edge: bare surface (the node does not extend there).
	for _, pt := range []struct{ x, y int }{
		{minX - 4, midY}, {maxX + 4, midY}, {midX, minY - 4}, {midX, maxY + 4},
	} {
		if got := img.RGBAAt(pt.x, pt.y); got != surface {
			t.Fatalf("outside-silhouette pixel (%d,%d) = %v, want bare surface %v", pt.x, pt.y, got, surface)
		}
	}
	// The top-face centre sits inside the box and is the node colour.
	tx, ty := screenOf(d, 4.5, 4.5, 1)
	if !(tx > minX && tx < maxX && ty > minY && ty < maxY) {
		t.Fatalf("top centre (%d,%d) not inside silhouette box [%d,%d]-[%d,%d]", tx, ty, minX, minY, maxX, maxY)
	}
	if got := img.RGBAAt(tx, ty); got != std(col) {
		t.Fatalf("top centre pixel = %v, want %v", got, std(col))
	}
}

// TestConnectorPathPixelsExact asserts a coloured connector is stroked exactly
// along the straight line between its two node anchors: the midpoint and a
// quarter-point of that world segment both land on pixels of the connector's
// own colour.
func TestConnectorPathPixelsExact(t *testing.T) {
	link := toolkit.RGB(200, 40, 160)
	doc := toolkit.NewIsoDoc()
	doc.PutNode(toolkit.IsoNode{ID: "a", X: 1, Y: 4})
	doc.PutNode(toolkit.IsoNode{ID: "b", X: 7, Y: 4})
	doc.PutConnector(toolkit.IsoConnector{ID: "ab", From: "a", To: "b", Color: link})
	d := toolkit.NewIsoDiagram(doc)
	img := render(t, d)

	// Anchors are the node top centres (height 1).
	ax, ay, az := 1.5, 4.5, 1.0
	bx, by, bz := 7.5, 4.5, 1.0
	for _, frac := range []float64{0.5, 0.25} {
		wx := ax + (bx-ax)*frac
		wy := ay + (by-ay)*frac
		wz := az + (bz-az)*frac
		px, py := screenOf(d, wx, wy, wz)
		if got := img.RGBAAt(px, py); got != std(link) {
			t.Fatalf("connector pixel at frac %.2f = %v, want link colour %v", frac, got, std(link))
		}
	}
}

// TestDepthOrderNearerDrawsOver proves the painter's-algorithm depth sort by
// rendering the far node alone and then both nodes: a pixel that the far node
// painted its own colour becomes the near node's colour once the near node is
// added, i.e. the nearer solid draws over the farther one.
func TestDepthOrderNearerDrawsOver(t *testing.T) {
	far := toolkit.RGB(220, 30, 30)  // (0,0), depth 0
	near := toolkit.RGB(30, 200, 30) // (1,0), depth 1 — nearer, drawn last
	farFaces := faceColors(far)
	nearFaces := faceColors(near)

	docFar := toolkit.NewIsoDoc()
	docFar.PutNode(toolkit.IsoNode{ID: "far", X: 0, Y: 0, Color: far})
	imgFar := render(t, toolkit.NewIsoDiagram(docFar))

	docBoth := toolkit.NewIsoDoc()
	docBoth.PutNode(toolkit.IsoNode{ID: "far", X: 0, Y: 0, Color: far})
	docBoth.PutNode(toolkit.IsoNode{ID: "near", X: 1, Y: 0, Color: near})
	imgBoth := render(t, toolkit.NewIsoDiagram(docBoth))

	for y := 0; y < CanvasHeight; y++ {
		for x := 0; x < CanvasWidth; x++ {
			if isFace(imgFar.RGBAAt(x, y), farFaces) && isFace(imgBoth.RGBAAt(x, y), nearFaces) {
				return // a far-node pixel is now a near-node pixel: correct overlap.
			}
		}
	}
	t.Fatal("no pixel where the nearer node overdrew the farther one")
}

// TestZoneUnderNodeAndCoverage checks the ground-plane zone's render order and
// its exact projected coverage: (1) a node standing on the zone keeps its own
// top-face colour (the zone floor never masks a node), and (2) rendering with
// and without the zone changes the ground pixel at the zone's interior centre
// while leaving a pixel well outside the zone untouched.
func TestZoneUnderNodeAndCoverage(t *testing.T) {
	nodeCol := toolkit.RGB(240, 200, 40)

	withZone := toolkit.NewIsoDoc()
	withZone.PutZone(toolkit.IsoZone{ID: "z", X: 1, Y: 1, W: 5, H: 5, Color: toolkit.RGBA{R: 40, G: 120, B: 220, A: 90}})
	withZone.PutNode(toolkit.IsoNode{ID: "n", X: 3, Y: 3, Color: nodeCol})
	dz := toolkit.NewIsoDiagram(withZone)
	imgZone := render(t, dz)

	// (1) The node on the zone keeps its exact top colour.
	tx, ty := screenOf(dz, 3.5, 3.5, 1)
	if got := imgZone.RGBAAt(tx, ty); got != std(nodeCol) {
		t.Fatalf("node-on-zone top pixel = %v, want node colour %v (zone masked the node)", got, std(nodeCol))
	}

	// Same scene without the zone, for the coverage diff.
	noZone := toolkit.NewIsoDoc()
	noZone.PutNode(toolkit.IsoNode{ID: "n", X: 3, Y: 3, Color: nodeCol})
	dn := toolkit.NewIsoDiagram(noZone)
	imgBare := render(t, dn)

	// (2a) A ground pixel inside the zone but on an empty cell changes when the
	// zone is present.
	inX, inY := screenOf(dz, 4.5, 1.5, 0)
	if imgZone.RGBAAt(inX, inY) == imgBare.RGBAAt(inX, inY) {
		t.Fatalf("zone-interior ground pixel (%d,%d) unchanged by the zone", inX, inY)
	}
	// (2b) A pixel far outside the zone footprint is identical in both renders.
	outX, outY := screenOf(dz, 8.5, 8.5, 0)
	if imgZone.RGBAAt(outX, outY) != imgBare.RGBAAt(outX, outY) {
		t.Fatalf("pixel (%d,%d) outside the zone changed — the fill bled past its polygon", outX, outY)
	}
}

// TestMultiSelectionOutlineExact asserts a multi-node selection draws the accent
// top-face outline on exactly the selected nodes: a projected top-face corner of
// each selected node is the accent colour (an outline vertex is a drawn line
// endpoint), the selected node's top-face centre is still its own colour (only
// the perimeter is stroked), and an unselected node's corner is not accent.
func TestMultiSelectionOutlineExact(t *testing.T) {
	accent := std(Theme().Accent)
	red := toolkit.RGB(220, 40, 40)
	blue := toolkit.RGB(40, 80, 220)
	green := toolkit.RGB(30, 190, 60)

	doc := toolkit.NewIsoDoc()
	doc.PutNode(toolkit.IsoNode{ID: "s0", X: 2, Y: 2, Color: red})
	doc.PutNode(toolkit.IsoNode{ID: "s1", X: 5, Y: 2, Color: blue})
	doc.PutNode(toolkit.IsoNode{ID: "u", X: 2, Y: 6, Color: green})
	d := toolkit.NewIsoDiagram(doc)
	SelectNodes(d, "s0", "s1")
	img := render(t, d)

	for _, n := range []struct {
		id      string
		x, y    int
		col     toolkit.RGBA
		wantSel bool
	}{
		{"s0", 2, 2, red, true},
		{"s1", 5, 2, blue, true},
		{"u", 2, 6, green, false},
	} {
		cx, cy := screenOf(d, float64(n.x), float64(n.y), 1) // a top-face corner
		got := img.RGBAAt(cx, cy)
		if n.wantSel && got != accent {
			t.Fatalf("selected node %s corner = %v, want accent %v", n.id, got, accent)
		}
		if !n.wantSel && got == accent {
			t.Fatalf("unselected node %s corner unexpectedly accent %v", n.id, got)
		}
		if n.wantSel {
			mx, my := screenOf(d, float64(n.x)+0.5, float64(n.y)+0.5, 1)
			if c := img.RGBAAt(mx, my); c != std(n.col) {
				t.Fatalf("selected node %s top centre = %v, want its colour %v", n.id, c, std(n.col))
			}
		}
	}
}

// TestHiddenLayerRestoresSurface renders the full showcase scene with the
// monitor layer visible and then hidden, and checks the probe node (a bare cube
// on that layer): with the layer visible its ground-cell centre is covered by
// the cube (not bare surface); once the layer is hidden that same pixel is
// exactly the theme surface again — the layer's paint is fully gone.
func TestHiddenLayerRestoresSurface(t *testing.T) {
	surface := std(Theme().Surface)

	shown := BuildScene()
	imgShown := render(t, shown)
	gx, gy := screenOf(shown, 3.5, 7.5, 0) // probe node cell (3,7) ground centre

	if got := imgShown.RGBAAt(gx, gy); got == surface {
		t.Fatalf("probe ground pixel is bare surface while its layer is visible")
	}

	hidden := BuildScene()
	HideMonitoring(hidden)
	imgHidden := render(t, hidden)
	if got := imgHidden.RGBAAt(gx, gy); got != surface {
		t.Fatalf("hidden-layer ground pixel = %v, want bare surface %v", got, surface)
	}
}

// --- CRDT convergence proof ---------------------------------------------

// TestConvergedReplicasIdentical proves the collaborative store: two replicas
// making concurrent edits converge to a byte-identical snapshot AND render to a
// byte-identical PNG, the concurrent move and recolour of the same node both
// survive (a per-field merge), and replica B's concurrent additions reach A.
func TestConvergedReplicasIdentical(t *testing.T) {
	a, b := ConvergedReplicas()

	// (a) Identical snapshots after the bidirectional exchange.
	if !bytes.Equal(a.Snapshot(), b.Snapshot()) {
		t.Fatal("replica snapshots differ after exchange")
	}

	// (c) The move (replica A) and the recolour (replica B) of the same node
	// both survived — a field-wise LWW merge, not a whole-record overwrite.
	for _, rep := range []struct {
		name string
		doc  *toolkit.IsoCRDTDocument
	}{{"A", a}, {"B", b}} {
		n, ok := rep.doc.Node(CollabMovedNode)
		if !ok {
			t.Fatalf("replica %s lost the shared node", rep.name)
		}
		if n.X != CollabMovedX || n.Y != CollabMovedY {
			t.Fatalf("replica %s: move lost, node at (%d,%d) want (%d,%d)", rep.name, n.X, n.Y, CollabMovedX, CollabMovedY)
		}
		if n.Color != CollabRecolor {
			t.Fatalf("replica %s: recolour lost, colour %v want %v", rep.name, n.Color, CollabRecolor)
		}
		if _, ok := rep.doc.Node(CollabAddedNode); !ok {
			t.Fatalf("replica %s missing B's concurrently added node", rep.name)
		}
		if _, ok := rep.doc.Zone(CollabAddedZone); !ok {
			t.Fatalf("replica %s missing B's concurrently added zone", rep.name)
		}
	}

	// (b) Convergence at the pixel level: the two replicas render byte-identical.
	da := replicaDiagram(a)
	db := replicaDiagram(b)
	pngA, err := toolkit.RenderPNG(da, CanvasWidth, CanvasHeight, Theme())
	if err != nil {
		t.Fatal(err)
	}
	pngB, err := toolkit.RenderPNG(db, CanvasWidth, CanvasHeight, Theme())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pngA, pngB) {
		t.Fatal("converged replicas rendered different images")
	}
}

// --- generation / output plumbing ---------------------------------------

// No committed golden PNGs.
//
// Every visual claim in this file is proven by COMPUTED assertions — a pixel's
// expected colour is derived from the same iso.Project / color.Shade maths the
// widget uses, and the CRDT convergence images are compared to EACH OTHER in the
// same run — so the proof never depends on a stored reference image. A golden
// PNG would instead be a byte-for-byte comparison of the anti-aliased vector /
// bitmap-font rasterisation, which is not guaranteed identical across the OS /
// arch matrix this module builds on (the coverage rasteriser and font renderer
// round in float32), and would risk a spurious CI failure. The inspectable
// images are produced on demand by the cmd/isodemo binary (or by pointing
// ISODEMO_OUT at this test) rather than checked in.

// TestGenerateWritesAllCaptures runs the full generator into a directory (the
// ISODEMO_OUT env var when set, so a human can point it at an inspectable path,
// else a temp dir) and checks every expected PNG file was written and is a
// decodable image of the canvas size. When the two replica images are present it
// re-confirms they are byte-identical on disk.
func TestGenerateWritesAllCaptures(t *testing.T) {
	dir := os.Getenv("ISODEMO_OUT")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := Generate(dir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	want := []string{"scene.png", "layer-hidden.png", "selection.png", "crdt-replica-a.png", "crdt-replica-b.png"}
	if len(paths) != len(want) {
		t.Fatalf("wrote %d files, want %d", len(paths), len(want))
	}
	for i, p := range paths {
		if filepath.Base(p) != want[i] {
			t.Fatalf("path[%d] = %s, want base %s", i, p, want[i])
		}
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		cfg, _, err := image.DecodeConfig(f)
		f.Close()
		if err != nil {
			t.Fatalf("decode %s: %v", p, err)
		}
		if cfg.Width != CanvasWidth || cfg.Height != CanvasHeight {
			t.Fatalf("%s is %dx%d, want %dx%d", p, cfg.Width, cfg.Height, CanvasWidth, CanvasHeight)
		}
	}
	// The two replica PNGs must be identical bytes on disk.
	repA, err := os.ReadFile(paths[3])
	if err != nil {
		t.Fatal(err)
	}
	repB, err := os.ReadFile(paths[4])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(repA, repB) {
		t.Fatal("replica PNG files differ on disk")
	}
	t.Logf("wrote %d showcase PNGs to %s", len(paths), dir)
}

// TestWriteWidgetErrors covers writeWidget's two failure paths: a render error
// (non-positive dimensions) and a filesystem error (a directory that does not
// exist).
func TestWriteWidgetErrors(t *testing.T) {
	theme := Theme()
	if _, err := writeWidget(t.TempDir(), "x.png", BuildScene(), 0, CanvasHeight, theme); err == nil {
		t.Fatal("writeWidget with zero width: want error")
	}
	missing := filepath.Join(t.TempDir(), "no-such-dir")
	if _, err := writeWidget(missing, "x.png", BuildScene(), CanvasWidth, CanvasHeight, theme); err == nil {
		t.Fatal("writeWidget into a missing dir: want error")
	}
}

// TestGenerateError checks Generate surfaces the first write failure — here an
// output directory that does not exist.
func TestGenerateError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-dir")
	if _, err := Generate(missing); err == nil {
		t.Fatal("Generate into a missing dir: want error")
	}
}
