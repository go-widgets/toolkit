// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-gfx/gfx/geometry"
	"github.com/go-gfx/gfx/iso"
)

// isoChanNear reports whether every channel of a is within tol of b's.
func isoChanNear(a, b RGBA, tol int) bool {
	d := func(u, v uint8) int {
		if u > v {
			return int(u - v)
		}
		return int(v - u)
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

// isoPolyBBox is the pixel bounding box (clamped to [0,w)x[0,h)) of a set of
// projected polygons.
func isoPolyBBox(polys [][]geometry.Point, w, h int) (minx, miny, maxx, maxy int) {
	minx, miny, maxx, maxy = w, h, -1, -1
	for _, poly := range polys {
		for _, p := range poly {
			x, y := iround(p.X), iround(p.Y)
			if x < minx {
				minx = x
			}
			if y < miny {
				miny = y
			}
			if x > maxx {
				maxx = x
			}
			if y > maxy {
				maxy = y
			}
		}
	}
	if minx < 0 {
		minx = 0
	}
	if miny < 0 {
		miny = 0
	}
	if maxx >= w {
		maxx = w - 1
	}
	if maxy >= h {
		maxy = h - 1
	}
	return
}

// occlusionDiagram is a deterministic scene of tall boxes in strongly saturated
// colours (far from the grey grid) clustered so the pre-fix depth sort drew grid
// lines over their faces. Node colours are chosen so no shaded face lands near
// the theme border colour, giving the occlusion assertions a wide margin.
func occlusionDiagram() *IsoDiagram {
	d := NewIsoDiagram(nil)
	d.Cols, d.Rows = 8, 8
	red := RGBA{R: 210, G: 60, B: 60, A: 255}
	green := RGBA{R: 60, G: 170, B: 90, A: 255}
	place := []struct {
		x, y int
		col  RGBA
	}{
		{1, 1, red}, {2, 1, red}, {1, 2, green},
		{3, 3, green}, {4, 3, red}, {3, 4, red}, {5, 5, green},
	}
	for i, p := range place {
		d.Doc().PutNode(IsoNode{ID: string(rune('a' + i)), X: p.x, Y: p.y, Shape: IsoBox, Color: p.col})
	}
	return d
}

// gridUnderSolids counts pixels close to the grid (Border) colour that fall
// strictly inside a node's projected silhouette — the grid showing through a
// solid that should occlude it. A correct render leaves none: the floor-pass
// grid is painted, then every opaque solid composites over the cells it covers.
func gridUnderSolids(t *testing.T, d *IsoDiagram, w, h int, theme *Theme) int {
	t.Helper()
	img, err := RenderImage(d, w, h, theme)
	if err != nil {
		t.Fatal(err)
	}
	const tol = 20 // the closest legitimate interior pixel sits >70 away per channel
	bad := 0
	for _, n := range d.doc.Nodes() {
		polys := d.pickPolys(n)
		minx, miny, maxx, maxy := isoPolyBBox(polys, w, h)
		for y := miny; y <= maxy; y++ {
			for x := minx; x <= maxx; x++ {
				if !isoChanNear(pixelAt(img.Pix, w, x, y), theme.Border, tol) {
					continue
				}
				for _, poly := range polys {
					if pointInPoly(float64(x)+0.5, float64(y)+0.5, poly) {
						bad++
						break
					}
				}
			}
		}
	}
	return bad
}

// TestIsoGridOccludedBySolids proves the ground grid is fully occluded by the
// solids standing on it — the floor-pass fix. Before it, long grid lines added
// to the depth-sorted scene could sort in front of a nearer solid and paint over
// its face; now the grid is a floor pass under every solid. The invariant must
// hold in all four view orientations, since the grid turns with the view.
func TestIsoGridOccludedBySolids(t *testing.T) {
	theme := DefaultLight()
	const w, h = 640, 520
	d := occlusionDiagram()
	// Sanity: node colours are nowhere near the grid colour, so a "grid-coloured
	// pixel inside a solid" can only be the grid bleeding through.
	for _, n := range d.doc.Nodes() {
		if isoChanNear(RGBA{R: n.Color.R, G: n.Color.G, B: n.Color.B, A: 255}, theme.Border, 40) {
			t.Fatalf("test node colour %+v too close to grid %+v", n.Color, theme.Border)
		}
	}
	for q := 0; q < 4; q++ {
		d.SetViewRotation(q)
		if bad := gridUnderSolids(t, d, w, h, theme); bad != 0 {
			t.Fatalf("rotation %d: %d grid-coloured pixels under a solid silhouette (grid shows through faces)", q, bad)
		}
	}
}

// TestIsoGridVisibleOnFreeTiles proves the fix occludes ONLY the covered cells:
// a grid line on a free tile — beside a solid, and one in front of a solid
// (nearer the camera) — stays visible. Floor-pass occlusion is a screen-space
// cover, so a grid segment a solid does not project over is untouched.
func TestIsoGridVisibleOnFreeTiles(t *testing.T) {
	theme := DefaultLight()
	const w, h = 640, 520
	d := occlusionDiagram()
	img, err := RenderImage(d, w, h, theme)
	if err != nil {
		t.Fatal(err)
	}
	// A width-1 anti-aliased diagonal line never reaches full grid colour at a
	// single pixel; its core is within ~12 per channel. Search a small window
	// around the projected point for the line core.
	gridNearby := func(at iso.Vec3) (int, int, bool) {
		p := d.project(at)
		cx, cy := iround(p.X), iround(p.Y)
		for dy := -3; dy <= 3; dy++ {
			for dx := -3; dx <= 3; dx++ {
				x, y := cx+dx, cy+dy
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				if isoChanNear(pixelAt(img.Pix, w, x, y), theme.Border, 15) {
					return x, y, true
				}
			}
		}
		return cx, cy, false
	}
	cases := []struct {
		name string
		at   iso.Vec3
	}{
		{"free corner beside the cluster", iso.V(7, 7, 0)}, // grid crossing, no node near
		{"in front of node (5,5)", iso.V(6, 6.5, 0)},       // grid line x=6, one tile in front (nearer)
	}
	for _, c := range cases {
		if _, _, ok := gridNearby(c.at); !ok {
			p := d.project(c.at)
			t.Fatalf("%s: grid line not visible near %v (projected ~%d,%d)",
				c.name, c.at, iround(p.X), iround(p.Y))
		}
	}
}
