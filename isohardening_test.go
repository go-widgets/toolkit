// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	stdcolor "image/color"
	"math"
	"testing"

	"github.com/go-gfx/gfx/raster"
)

// A panic anywhere in these paths fails the test with its stack trace, so each
// test proves the absence of a panic simply by completing; the assertions add
// teeth by checking the defensive path also produced the RIGHT result.

// TestIsoRenderEmptyDiagram renders a diagram with no entities, in every view
// orientation. The grid floor pass must cope with an otherwise empty scene.
func TestIsoRenderEmptyDiagram(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	for q := 0; q < 4; q++ {
		d.SetViewRotation(q)
		if _, err := RenderImage(d, 300, 300, theme); err != nil {
			t.Fatalf("rotation %d: %v", q, err)
		}
	}
}

// TestIsoDegenerateGridExtent renders with a zero and a negative grid extent —
// values a host can push straight onto the exported Cols/Rows fields.
func TestIsoDegenerateGridExtent(t *testing.T) {
	theme := DefaultLight()
	for _, ex := range [][2]int{{0, 0}, {-3, -2}, {0, 5}, {5, 0}} {
		d := NewIsoDiagram(nil)
		d.Cols, d.Rows = ex[0], ex[1]
		d.Doc().PutNode(IsoNode{ID: "a", X: 0, Y: 0})
		if _, err := RenderImage(d, 300, 300, theme); err != nil {
			t.Fatalf("extent %v: %v", ex, err)
		}
	}
}

// TestIsoConnectorToMissingEndpoint drives every connector-consuming path with a
// connector whose To id names no node — decorated with arrows and a label and
// routed, so the arrow-head, label-midpoint and routing code all run.
func TestIsoConnectorToMissingEndpoint(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.Doc().PutConnector(IsoConnector{
		ID: "c", From: "a", To: "ghost",
		Arrow: IsoArrowDouble, Label: "dangling", Routed: true,
	})
	if _, ok := d.connectorPath(IsoConnector{From: "a", To: "ghost"}); ok {
		t.Fatal("connectorPath resolved a missing endpoint")
	}
	if _, err := RenderImage(d, 400, 400, theme); err != nil {
		t.Fatal(err)
	}
	// Hit-testing and marquee over the dangling connector must also be safe.
	d.connectorAtLocal(200, 200)
	d.marqueeRefs(-10, -10, 500, 500)
}

// TestIsoSelfConnectorRouted covers the routed self-loop, whose deduped path
// collapses to a single point — the arrow-head and midpoint code must not index
// past it.
func TestIsoSelfConnectorRouted(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 2})
	d.Doc().PutConnector(IsoConnector{ID: "c", From: "a", To: "a", Routed: true, Arrow: IsoArrowDouble, Label: "loop"})
	if _, err := RenderImage(d, 400, 400, theme); err != nil {
		t.Fatal(err)
	}
}

// TestIsoPlacementAtEdge places nodes at negative and far-off-board cells (a
// right-click "Add node" on the extreme edge of a panned view). Off-board tiles
// are legal — the model is a map, not a fixed array — so this must place them
// and render without panicking.
func TestIsoPlacementAtEdge(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	for _, c := range [][2]int{{-100, -100}, {100000, 100000}, {-1, 5}} {
		id := d.commitPlace(c[0], c[1])
		n, ok := d.Doc().Node(id)
		if !ok || n.X != c[0] || n.Y != c[1] {
			t.Fatalf("node not placed at %v", c)
		}
	}
	if _, err := RenderImage(d, 400, 400, theme); err != nil {
		t.Fatal(err)
	}
}

// TestIsoZoomExtremes drives the wheel-zoom to both clamps repeatedly and checks
// the tile width stays inside [IsoMinTile, IsoMaxTile] with the render intact.
func TestIsoZoomExtremes(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	for i := 0; i < 300; i++ {
		d.ZoomAt(0.5, 200, 200)
	}
	if d.Projection().TileW < IsoMinTile {
		t.Fatalf("tile width %v under the min clamp %v", d.Projection().TileW, IsoMinTile)
	}
	if _, err := RenderImage(d, 400, 400, theme); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		d.ZoomAt(2, 200, 200)
	}
	if d.Projection().TileW > IsoMaxTile {
		t.Fatalf("tile width %v over the max clamp %v", d.Projection().TileW, IsoMaxTile)
	}
	if _, err := RenderImage(d, 400, 400, theme); err != nil {
		t.Fatal(err)
	}
}

// TestIsoDegenerateAnimationPhase feeds AnimationStep a NaN and both infinities —
// a host handing a bad dt from a stalled clock. The phase must stay finite in
// [0, 1) so animated icons never emit NaN coordinates, and every frame renders.
func TestIsoDegenerateAnimationPhase(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	d.Icons = NewIsoIconRegistry()
	RegisterAnimatedIcons(d.Icons)
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1, Icon: "anim/gear"})
	d.AnimationStep(0.3) // a good step first, so a finite phase is on record
	good := d.AnimationPhase()
	for _, dt := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		d.AnimationStep(dt)
		p := d.AnimationPhase()
		if math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p >= 1 {
			t.Fatalf("dt=%v left phase %v (want finite in [0,1))", dt, p)
		}
		if p != good {
			t.Fatalf("dt=%v changed the phase from %v to %v", dt, good, p)
		}
		if _, err := RenderImage(d, 300, 300, theme); err != nil {
			t.Fatal(err)
		}
	}
	// isoWrapPhase folds any non-finite input to the rest frame directly.
	for _, ph := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := isoWrapPhase(ph); got != 0 {
			t.Fatalf("isoWrapPhase(%v) = %v, want 0", ph, got)
		}
	}
}

// spriteAnimIcon is a test-only animated icon that returns a sprite (not shapes),
// so the animated-sprite blit path is exercised.
type spriteAnimIcon struct{ img *raster.Image }

func (s spriteAnimIcon) Render(x, y int, base stdcolor.RGBA) IsoIconDrawing {
	return s.RenderAt(x, y, base, 0)
}
func (s spriteAnimIcon) RenderAt(x, y int, base stdcolor.RGBA, phase float64) IsoIconDrawing {
	return IsoIconDrawing{Sprite: s.img}
}

// TestIsoAnimatedSpriteRenders drives a node whose animated icon contributes a
// sprite through a phase step and a render.
func TestIsoAnimatedSpriteRenders(t *testing.T) {
	theme := DefaultLight()
	spr := raster.New(6, 6)
	for i := 0; i+3 < len(spr.Pix); i += 4 {
		spr.Pix[i], spr.Pix[i+1], spr.Pix[i+2], spr.Pix[i+3] = 200, 30, 30, 255
	}
	d := NewIsoDiagram(nil)
	d.Icons = NewIsoIconRegistry()
	d.Icons.Register("spr", spriteAnimIcon{img: spr})
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 2, Icon: "spr"})
	d.AnimationStep(0.5)
	if _, err := RenderImage(d, 300, 300, theme); err != nil {
		t.Fatal(err)
	}
}

// TestIsoNilIconFallsBackToSolid proves a nil resolved icon — a host registering
// nil, or nilling the exported IsoFallbackIcon an unknown id resolves to — no
// longer crashes the render path but degrades to the node's plain solid.
func TestIsoNilIconFallsBackToSolid(t *testing.T) {
	theme := DefaultLight()
	const w, h = 320, 320
	red := RGBA{R: 210, G: 60, B: 60, A: 255}

	d := NewIsoDiagram(nil)
	d.Icons = NewIsoIconRegistry()
	d.Icons.Register("broken", nil) // a nil icon under a real id
	d.Doc().PutNode(IsoNode{ID: "a", X: 3, Y: 3, Color: red, Icon: "broken"})

	img, err := RenderImage(d, w, h, theme)
	if err != nil {
		t.Fatal(err)
	}
	// The node must still be drawn (its plain solid): its top-face centre carries
	// the node colour, not the background.
	n, _ := d.Doc().Node("a")
	p := d.project(d.nodeAnchor(n))
	if got := pixelAt(img.Pix, w, iround(p.X), iround(p.Y)); !isoChanNear(got, red, 8) {
		t.Fatalf("nil-icon node not drawn as its solid: top pixel %+v, want ~%+v", got, red)
	}

	// An unknown id whose fallback has been nilled out must also be safe.
	save := IsoFallbackIcon
	IsoFallbackIcon = nil
	defer func() { IsoFallbackIcon = save }()
	d.Doc().PutNode(IsoNode{ID: "b", X: 6, Y: 6, Color: red, Icon: "no-such-icon"})
	if _, err := RenderImage(d, w, h, theme); err != nil {
		t.Fatal(err)
	}
}

// TestIsoEventStormOnDegenerateBounds fires a full press/drag/release marquee,
// context menu, undo and redo against a widget whose bounds are zero and then at
// extreme coordinates — the kind of stray input a host can deliver before layout.
func TestIsoEventStormOnDegenerateBounds(t *testing.T) {
	d := NewIsoDiagram(nil)
	// No bounds set yet.
	d.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	d.OnEvent(Event{Kind: EventMouseDrag, X: 20, Y: 20})
	d.OnEvent(Event{Kind: EventMouseUp, X: 20, Y: 20})

	d.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 0})
	for i := 0; i < 4; i++ {
		d.Doc().PutNode(IsoNode{ID: string(rune('a' + i)), X: i, Y: i})
	}
	// Marquee across the whole plane at extreme coordinates.
	d.OnEvent(Event{Kind: EventClick, X: -9999, Y: -9999})
	d.OnEvent(Event{Kind: EventMouseDrag, X: 9999, Y: 9999})
	d.OnEvent(Event{Kind: EventMouseUp, X: 9999, Y: 9999})
	d.OnEvent(Event{Kind: EventSecondaryClick, X: 5, Y: 5})
	d.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	d.Undo()
	d.Redo()
}
