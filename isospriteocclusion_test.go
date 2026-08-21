// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	stdcolor "image/color"
	"testing"

	"github.com/go-gfx/gfx/geometry"
	"github.com/go-gfx/gfx/iso"
	"github.com/go-gfx/gfx/raster"
	"github.com/go-gfx/gfx/vector"
)

// spriteMagenta is a sprite colour far from the grey grid (Border) and the grey
// canvas (Surface/Background), so a "grid-coloured pixel" the occlusion asserts
// on can only be the floor grid, never the sprite art or the empty canvas.
var spriteMagenta = stdcolor.RGBA{R: 255, G: 0, B: 255, A: 255}

// gridColorNear reports whether a pixel within radius of the projected model
// point at is within tol per channel of the grid (Border) colour — the floor
// grid showing there. A width-1 anti-aliased grid line never reaches full Border
// colour at one pixel, so callers use a small tolerance and a few-pixel window,
// exactly like TestIsoGridVisibleOnFreeTiles.
func gridColorNear(img []byte, w, h int, d *IsoDiagram, at iso.Vec3, theme *Theme, tol, radius int) bool {
	p := d.project(at)
	cx, cy := iround(p.X), iround(p.Y)
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			x, y := cx+dx, cy+dy
			if x < 0 || y < 0 || x >= w || y >= h {
				continue
			}
			if isoChanNear(pixelAt(img, w, x, y), theme.Border, tol) {
				return true
			}
		}
	}
	return false
}

// cellEdgeMidpoints returns the four model-space midpoints of grid cell (cx, cy)'s
// bounding edges at z=0 — the points a grid line runs through on each side of the
// tile. They sit half a cell from every corner, so a small search window around
// one never reaches the perpendicular grid line at the corner (tiles are 64px
// wide): a hit there is that edge's own line, not a neighbour's.
func cellEdgeMidpoints(cx, cy int) []iso.Vec3 {
	fx, fy := float64(cx), float64(cy)
	return []iso.Vec3{
		iso.V(fx+0.5, fy, 0),   // top edge  (shared with cell above)
		iso.V(fx+0.5, fy+1, 0), // bottom edge
		iso.V(fx, fy+0.5, 0),   // left edge
		iso.V(fx+1, fy+0.5, 0), // right edge
	}
}

// freeRefCell is an interior tile far from every node whose grid must stay
// visible. View rotation is rigid about the grid centre, so its model distance
// from the sprite is preserved in every orientation — chosen far enough that the
// sprite's 64px billboard never reaches over it.
var freeRefCell = [2]int{8, 8}

// spriteOcclusionDiagram places a sprite icon on cell (4,4), a primitive box on
// (6,4) (a solid whose cell must stay occluded exactly as before) and leaves
// freeRefCell free (its grid must stay visible). The sprite is a
// per-widget-registered solid magenta square.
func spriteOcclusionDiagram() *IsoDiagram {
	reg := NewIsoIconRegistry()
	reg.Register("pic", IsoSpriteIcon{Img: solidSprite(48, 48, spriteMagenta)})
	d := NewIsoDiagram(nil)
	d.Icons = reg
	d.Doc().PutNode(IsoNode{ID: "sprite", X: 4, Y: 4, Icon: "pic"})
	d.Doc().PutNode(IsoNode{ID: "cube", X: 6, Y: 4, Shape: IsoBox, Color: RGBA{R: 210, G: 60, B: 60, A: 255}})
	return d
}

// TestIsoGridSkippedUnderSprite is the teeth of the fix: on the tile a billboard
// sprite stands on, NO grid line survives (the sprite has no solid to occlude the
// floor grid, so the four edges of its cell are dropped at the floor pass),
// while a free interior tile KEEPS every one of its four grid edges. The
// invariant holds in all four view orientations, since both the grid and the
// footprint skip live in model space and rotate together at projection.
func TestIsoGridSkippedUnderSprite(t *testing.T) {
	theme := DefaultLight()
	const w, h = 800, 600
	d := spriteOcclusionDiagram()
	for q := 0; q < 4; q++ {
		d.SetViewRotation(q)
		img, err := RenderImage(d, w, h, theme)
		if err != nil {
			t.Fatal(err)
		}
		// Sprite tile (4,4): every edge midpoint must be grid-free.
		for i, mid := range cellEdgeMidpoints(4, 4) {
			p := d.project(mid)
			if px, py := iround(p.X), iround(p.Y); px < 0 || py < 0 || px >= w || py >= h {
				t.Fatalf("rotation %d: sprite edge %d projects off-screen (%d,%d); widen canvas", q, i, px, py)
			}
			if gridColorNear(img.Pix, w, h, d, mid, theme, 20, 3) {
				t.Fatalf("rotation %d: grid line still crosses sprite tile edge %d (leak under the sprite)", q, i)
			}
		}
		// A free tile far from the sprite must still show every one of its grid
		// edges, so the skip is targeted at the sprite, not a blanket erase.
		for i, mid := range cellEdgeMidpoints(freeRefCell[0], freeRefCell[1]) {
			if !gridColorNear(img.Pix, w, h, d, mid, theme, 15, 3) {
				p := d.project(mid)
				t.Fatalf("rotation %d: free tile lost grid edge %d near (%d,%d)", q, i, iround(p.X), iround(p.Y))
			}
		}
	}
}

// TestIsoPrimitiveCellNotRegressedBySpriteFix proves the sprite fix leaves the
// primitive case exactly as the earlier floor-pass fix left it: the solid on
// (6,4) still fully occludes the grid inside its silhouette, in every
// orientation. (The dropped-edge treatment is applied ONLY to sprite tiles, so
// the cube's cell is untouched.)
func TestIsoPrimitiveCellNotRegressedBySpriteFix(t *testing.T) {
	theme := DefaultLight()
	const w, h = 800, 600
	d := spriteOcclusionDiagram()
	cube, _ := d.doc.Node("cube")
	for q := 0; q < 4; q++ {
		d.SetViewRotation(q)
		img, err := RenderImage(d, w, h, theme)
		if err != nil {
			t.Fatal(err)
		}
		polys := d.pickPolys(cube)
		minx, miny, maxx, maxy := isoPolyBBox(polys, w, h)
		for y := miny; y <= maxy; y++ {
			for x := minx; x <= maxx; x++ {
				if !isoChanNear(pixelAt(img.Pix, w, x, y), theme.Border, 20) {
					continue
				}
				for _, poly := range polys {
					if pointInPoly(float64(x)+0.5, float64(y)+0.5, poly) {
						t.Fatalf("rotation %d: grid-coloured pixel inside the primitive silhouette (%d,%d)", q, x, y)
					}
				}
			}
		}
	}
}

// TestIsoGridPrimitiveOnlyByteIdentical is the control-run: with NO sprite tiles
// the refactored drawGrid must stroke exactly the pre-fix full-line grid, byte
// for byte, in every orientation. It renders the grid through drawGrid and
// through a local copy of the original single-stroke-per-line code and asserts
// the two buffers are identical.
func TestIsoGridPrimitiveOnlyByteIdentical(t *testing.T) {
	theme := DefaultLight()
	const w, h = 640, 520
	d := occlusionDiagram() // primitive boxes only, no icons
	d.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	for q := 0; q < 4; q++ {
		d.SetViewRotation(q)
		got := raster.New(w, h)
		d.drawGrid(got, theme)
		want := raster.New(w, h)
		refDrawGridFullLines(d, want, theme)
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Fatalf("rotation %d: refactored grid differs from the pre-fix full-line grid", q)
		}
	}
}

// refDrawGridFullLines is the pre-fix drawGrid: one stroke per full grid line,
// with no footprint skipping. It is the byte-for-byte reference the control-run
// compares the refactored drawGrid against for a sprite-free diagram.
func refDrawGridFullLines(d *IsoDiagram, img *raster.Image, theme *Theme) {
	grid := stdColor(theme.Border)
	rz := &vector.Rasterizer{}
	seg := func(a, b iso.Vec3) {
		pa, pb := d.project(a), d.project(b)
		path := vector.NewPath().MoveTo(pa.X, pa.Y).LineTo(pb.X, pb.Y)
		cov, ox, oy, w, h, ok := rz.Stroke(path, 1, img.W, img.H)
		if !ok {
			return
		}
		vector.Composite(img, cov, ox, oy, w, h, vector.SolidPaint{Color: grid})
	}
	for i := 0; i <= d.Cols; i++ {
		seg(iso.V(float64(i), 0, 0), iso.V(float64(i), float64(d.Rows), 0))
	}
	for j := 0; j <= d.Rows; j++ {
		seg(iso.V(0, float64(j), 0), iso.V(float64(d.Cols), float64(j), 0))
	}
}

// TestDrawGridOffscreenStrokeSkipped drives the stroke closure's !ok early return:
// with the whole grid panned far off the buffer, every segment clips out and
// vector.Stroke reports no coverage, so drawGrid writes nothing into an otherwise
// untouched buffer (and does not panic).
func TestDrawGridOffscreenStrokeSkipped(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	d.proj.Origin = geometry.Pt(-100000, -100000) // pan the grid entirely off-buffer
	img := raster.New(8, 8)                       // freshly zeroed
	d.drawGrid(img, theme)
	for _, b := range img.Pix {
		if b != 0 {
			t.Fatal("off-screen grid wrote into an untouched buffer")
		}
	}
}

// --- gridLineRuns unit coverage ------------------------------------------------

// runsOf collects gridLineRuns' emitted [lo,hi) runs for skip set skip over n
// unit sub-segments.
func runsOf(n int, skip map[int]bool) [][2]int {
	var out [][2]int
	gridLineRuns(n, func(k int) bool { return skip[k] }, func(lo, hi int) {
		out = append(out, [2]int{lo, hi})
	})
	return out
}

func TestGridLineRuns(t *testing.T) {
	eq := func(a, b [][2]int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	cases := []struct {
		name string
		n    int
		skip map[int]bool
		want [][2]int
	}{
		{"no skip is one full run", 3, nil, [][2]int{{0, 3}}},
		{"skip in the middle splits", 3, map[int]bool{1: true}, [][2]int{{0, 1}, {2, 3}}},
		{"skip at the start", 3, map[int]bool{0: true}, [][2]int{{1, 3}}},
		{"skip at the end", 3, map[int]bool{2: true}, [][2]int{{0, 2}}},
		{"all skipped emits nothing", 3, map[int]bool{0: true, 1: true, 2: true}, nil},
		{"two adjacent skips", 4, map[int]bool{1: true, 2: true}, [][2]int{{0, 1}, {3, 4}}},
		{"empty line emits nothing", 0, nil, nil},
	}
	for _, c := range cases {
		if got := runsOf(c.n, c.skip); !eq(got, c.want) {
			t.Fatalf("%s: runs = %v, want %v", c.name, got, c.want)
		}
	}
}

// --- spriteFootprints unit coverage --------------------------------------------

// hybridIcon is an IsoIcon that contributes BOTH a solid and a sprite — the case
// a footprint skip must still cover, since the sprite blits over whatever the
// shape leaves uncovered.
type hybridIcon struct{ img *raster.Image }

func (h hybridIcon) Render(x, y int, base stdcolor.RGBA) IsoIconDrawing {
	return IsoIconDrawing{
		Shapes: []iso.Shape{iso.Cube{Pos: iso.V(float64(x), float64(y), 0), Size: 1, Color: base}},
		Sprite: h.img,
	}
}

func TestSpriteFootprints(t *testing.T) {
	theme := DefaultLight()
	reg := NewIsoIconRegistry()
	reg.Register("sprite", IsoSpriteIcon{Img: solidSprite(4, 4, spriteMagenta)})
	reg.Register("prim", IsoPrimitiveIcon{Build: isoBoxShapes})
	reg.Register("hybrid", hybridIcon{img: solidSprite(4, 4, spriteMagenta)})
	reg.Register("nilicon", nil) // a host registering nil: renders to an empty drawing

	d := NewIsoDiagram(nil)
	d.Icons = reg
	// A hidden layer whose sprite must NOT skip its tile (it never blits).
	d.Doc().PutLayer(IsoLayer{ID: "hidden", Name: "h", Visible: false, Order: 1})

	d.Doc().PutNode(IsoNode{ID: "plain", X: 0, Y: 0})                                // Icon=="" → skipped
	d.Doc().PutNode(IsoNode{ID: "sprite", X: 1, Y: 1, Icon: "sprite"})               // sprite → included
	d.Doc().PutNode(IsoNode{ID: "prim", X: 2, Y: 2, Icon: "prim"})                   // no sprite → excluded
	d.Doc().PutNode(IsoNode{ID: "hybrid", X: 3, Y: 3, Icon: "hybrid"})               // shapes+sprite → included
	d.Doc().PutNode(IsoNode{ID: "nil", X: 4, Y: 4, Icon: "nilicon"})                 // nil icon → empty drawing → excluded
	d.Doc().PutNode(IsoNode{ID: "hid", X: 5, Y: 5, Icon: "sprite", Layer: "hidden"}) // hidden → excluded

	got := d.spriteFootprints(theme)
	want := map[[2]int]bool{{1, 1}: true, {3, 3}: true}
	if len(got) != len(want) {
		t.Fatalf("spriteFootprints = %v, want %v", got, want)
	}
	for cell := range want {
		if !got[cell] {
			t.Fatalf("spriteFootprints missing cell %v; got %v", cell, got)
		}
	}
}
