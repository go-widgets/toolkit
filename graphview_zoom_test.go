// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func zoomGraph() *GraphView {
	g := NewGraphView(manyClusters(30, 4))
	g.SetBounds(Rect{X: 5, Y: 7, W: 300, H: 150})
	g.layout()
	return g
}

func TestGraphViewZoomDefaultsAndClamps(t *testing.T) {
	g := zoomGraph()
	if g.Zoom().Get() != 100 {
		t.Fatalf("default zoom %d", g.Zoom().Get())
	}
	for i := 0; i < 40; i++ {
		g.ZoomIn()
	}
	if g.zoomPct() != GraphZoomMax {
		t.Fatalf("zoom in clamps to %d, got %d", GraphZoomMax, g.zoomPct())
	}
	for i := 0; i < 40; i++ {
		g.ZoomOut()
	}
	if g.zoomPct() != GraphZoomMin {
		t.Fatalf("zoom out clamps to %d, got %d", GraphZoomMin, g.zoomPct())
	}
	g.Zoom().Set(100)
	g.ZoomIn()
	if g.Zoom().Get() != 125 {
		t.Fatalf("one step is 25, got %d", g.Zoom().Get())
	}
	g.Zoom().Set(9999)
	if g.zoomPct() != GraphZoomMax {
		t.Fatalf("a value set past the limit reads clamped, got %d", g.zoomPct())
	}
}

func TestGraphViewZoomKeepsThePointUnderTheCursor(t *testing.T) {
	g := zoomGraph()
	g.ScrollTo(40)
	nb, _ := g.NodeBox("n3_1")
	// A point inside that node, widget-local.
	ax, ay := nb.X+nb.W/2-g.Bounds().X, nb.Y+nb.H/2-g.Bounds().Y
	g.Model.Selected.Set("")
	before := g.Hit(g.Bounds().X+ax, g.Bounds().Y+ay)
	g.OnEvent(Event{Kind: EventScroll, X: ax, Y: ay, Delta: -1, Ctrl: true})
	if g.zoomPct() != 125 {
		t.Fatalf("ctrl+wheel up zooms in by one step, got %d", g.zoomPct())
	}
	if after := g.Hit(g.Bounds().X+ax, g.Bounds().Y+ay); after != before || before == "" {
		t.Fatalf("the thing under the cursor changed: %q -> %q", before, after)
	}
	// The anchor's content point moves by at most a pixel on screen.
	g2 := zoomGraph()
	g2.ScrollTo(40)
	b0, _ := g2.NodeBox("n3_1")
	g2.SetZoom(200, ax, ay)
	b1, _ := g2.NodeBox("n3_1")
	cy0 := b0.Y - g2.Bounds().Y
	// the node's top-left, mapped from 100 % to 200 % around (ax, ay), is
	// ax + (cx0-ax)*2.
	wantY := ay + (cy0-ay)*2
	gotY := b1.Y - g2.Bounds().Y
	if absInt(gotY-wantY) > 1 {
		t.Fatalf("anchor drifted vertically: want %d got %d", wantY, gotY)
	}
	// Horizontally, in a view narrower than the content, so the pan has room.
	g3 := NewGraphView(manyClusters(30, 4))
	g3.SetBounds(Rect{W: 100, H: 150})
	g3.layout()
	nb3, _ := g3.NodeBox("n0_0")
	px := nb3.X + nb3.W/2
	g3.SetZoom(200, px, 0)
	nb3b, _ := g3.NodeBox("n0_0")
	if want := px + (nb3.X-px)*2; absInt(nb3b.X-want) > 1 {
		t.Fatalf("anchor drifted horizontally: want %d got %d (pan %d, max %d)", want, nb3b.X, g3.panX, g3.maxPanX())
	}
	// Wheel down zooms out.
	g.OnEvent(Event{Kind: EventScroll, X: ax, Y: ay, Delta: 1, Meta: true})
	if g.zoomPct() != 100 {
		t.Fatalf("cmd+wheel down zooms out, got %d", g.zoomPct())
	}
}

func TestGraphViewZoomScalesTheBoxesAndHits(t *testing.T) {
	g := NewGraphView(manyClusters(30, 4))
	g.SetBounds(Rect{W: 300, H: 800})
	g.layout()
	b100, _ := g.NodeBox("n0_0")
	g.SetZoom(200, 0, 0)
	b200, _ := g.NodeBox("n0_0")
	if b200.W != 2*b100.W || b200.H != 2*b100.H {
		t.Fatalf("a node is twice the size at 200 %%: %v vs %v", b100, b200)
	}
	if got := g.Hit(b200.X+1, b200.Y+1); got != "node:n0_0" {
		t.Fatalf("a point in the zoomed box hits it, got %q", got)
	}
	// The scrollbar's thumb shrinks: more content is below.
	g.Zoom().Set(100)
	g1, _ := g.scrollbarGeom()
	g.Zoom().Set(200)
	g2, _ := g.scrollbarGeom()
	if g2.thumbLen >= g1.thumbLen {
		t.Fatalf("thumb at 200 %% (%d) must be shorter than at 100 %% (%d)", g2.thumbLen, g1.thumbLen)
	}
}

func TestGraphViewPanClampsToContent(t *testing.T) {
	g := zoomGraph()
	g.PanTo(500)
	if g.Pan().Get().X != 0 {
		t.Fatalf("at 100 %% the content is no wider than the view; pan x = %d", g.Pan().Get().X)
	}
	g.SetZoom(400, 0, 0)
	g.PanTo(1 << 20)
	if got, max := g.Pan().Get().X, g.maxPanX(); got != max || max <= 0 {
		t.Fatalf("pan x %d, max %d", got, max)
	}
	g.PanTo(-50)
	if g.Pan().Get().X != 0 {
		t.Fatalf("pan x below 0 clamps, got %d", g.Pan().Get().X)
	}
	g.ScrollTo(1 << 20)
	if got := g.Pan().Get().Y; got != g.maxScrollY() || got != g.ScrollY() {
		t.Fatalf("pan y %d, scrollY %d, max %d", got, g.ScrollY(), g.maxScrollY())
	}
	// A host may set the pan through the observable.
	g.Pan().Set(GraphPoint{X: 10, Y: 20})
	g.layout()
	if g.ScrollY() != 20 || g.panX != 10 {
		t.Fatalf("a set pan is adopted: scrollY %d panX %d", g.ScrollY(), g.panX)
	}
}

func TestGraphViewBackgroundDragPans(t *testing.T) {
	g := zoomGraph()
	g.SetZoom(200, 0, 0)
	// A background point: not on a node or edge, and clear of the scrollbar.
	x, y, found := 0, 0, false
	for yy := 5; yy < g.Bounds().H-5 && !found; yy += 3 {
		for xx := 5; xx < g.Bounds().W-scrollbarTrack()-5 && !found; xx += 3 {
			if g.Hit(g.Bounds().X+xx, g.Bounds().Y+yy) == "" {
				x, y, found = xx, yy, true
			}
		}
	}
	if !found {
		t.Fatal("no background point in the view")
	}
	g.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	g.OnEvent(Event{Kind: EventMouseDrag, X: x - 40, Y: y - 20})
	g.OnEvent(Event{Kind: EventMouseUp, X: x - 40, Y: y - 20})
	if g.panX != 20 || g.ScrollY() != 10 {
		t.Fatalf("a 40,20 drag at 200 %% pans 20,10 content px, got %d,%d", g.panX, g.ScrollY())
	}
	// Dragging from a node pans nothing.
	nb, _ := g.NodeBox("n0_0")
	if nb.X >= g.Bounds().X && nb.Y >= g.Bounds().Y {
		before := g.panX
		g.OnEvent(Event{Kind: EventClick, X: nb.X - g.Bounds().X + 1, Y: nb.Y - g.Bounds().Y + 1})
		g.OnEvent(Event{Kind: EventMouseDrag, X: 0, Y: 0})
		if g.panX != before {
			t.Fatalf("a drag that starts on a node must not pan")
		}
	}
}

func TestGraphViewZoomKeys(t *testing.T) {
	g := zoomGraph()
	g.OnEvent(Event{Kind: EventKeyDown, Code: "+"})
	g.OnEvent(Event{Kind: EventKeyDown, Code: "+"})
	if g.zoomPct() != 150 {
		t.Fatalf("two + give 150, got %d", g.zoomPct())
	}
	g.OnEvent(Event{Kind: EventKeyDown, Code: "-"})
	if g.zoomPct() != 125 {
		t.Fatalf("- gives 125, got %d", g.zoomPct())
	}
	g.OnEvent(Event{Kind: EventKeyDown, Code: "0"})
	if z := g.zoomPct(); z != GraphZoomMin || g.ScrollY() != 0 {
		t.Fatalf("0 on a very tall graph fits as far as the minimum: zoom %d scroll %d", z, g.ScrollY())
	}
	// A graph that fits at some zoom above the minimum shows every cluster.
	f := NewGraphView(manyClusters(8, 2))
	f.SetBounds(Rect{W: 300, H: 300})
	f.ZoomIn()
	f.OnEvent(Event{Kind: EventKeyDown, Code: "0"})
	if z := f.zoomPct(); z >= 100 || z <= GraphZoomMin || z%GraphZoomStep != 0 ||
		f.contentH*z > f.viewH*100 || f.contentW*z > f.Bounds().W*100 {
		t.Fatalf("fit leaves content outside: %dx%d at %d%% in %dx%d", f.contentW, f.contentH, f.zoomPct(), f.Bounds().W, f.viewH)
	}
	// Arrows still move the selection.
	g.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if g.Model.Selected.Get() == "" {
		t.Fatal("arrow keys still select")
	}
}

func TestGraphViewPinchZooms(t *testing.T) {
	g := zoomGraph()
	g.OnEvent(Event{Kind: EventTouchStart, Code: "a", X: 100, Y: 70})
	g.OnEvent(Event{Kind: EventTouchStart, Code: "b", X: 140, Y: 70})
	g.OnEvent(Event{Kind: EventTouchMove, Code: "b", X: 180, Y: 70}) // span 40 -> 80: x2
	if g.zoomPct() != 200 {
		t.Fatalf("a 2x pinch zooms to 200, got %d", g.zoomPct())
	}
}

// The zoomed picture is drawn where the zoomed boxes are: the node's colour
// fills the middle of its box at 200 % and is absent at its 100 % place.
func TestGraphViewDrawsAtTheZoom(t *testing.T) {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "c", Label: "c"})
	m.Nodes.Append(GraphNode{ID: "a", Label: "", Cluster: "c", Category: "k"})
	g := NewGraphView(m)
	g.Palette = []RGBA{RGB(0xE6, 0x9F, 0x00)}
	g.SetBounds(Rect{W: 300, H: 200})
	g.SetZoom(200, 0, 0)
	b, _ := g.NodeBox("a")
	surf := makeSurface(300, 200)
	g.Draw(newP(surf, 300), DefaultLight())
	if countInk(surf, 300, 200, g.Palette[0]) < b.W*b.H/2 {
		t.Fatalf("the node is not painted over its zoomed box %v", b)
	}
	if b.W != 2*g.nodeBox["a"].W {
		t.Fatalf("the box is not doubled: %v vs %v", b, g.nodeBox["a"])
	}
}

// A graph nobody zooms, pans or scrolls allocates neither observable.
func TestGraphViewZoomStateIsLazy(t *testing.T) {
	g := zoomGraph()
	g.Draw(newP(makeSurface(320, 170), 320), DefaultLight())
	if g.zoom != nil && g.zoom.Get() != 100 {
		t.Fatal("zoom moved by itself")
	}
	if g.pan != nil {
		t.Fatal("pan observable allocated though nothing moved")
	}
}

// A graph used as before (no zoom, no pan) is drawn and hit exactly as it was.
func TestGraphViewIdentityTransformIsExact(t *testing.T) {
	g := zoomGraph()
	r := Rect{X: 11, Y: 22, W: 33, H: 44}
	if g.screen(r) != r || g.sx(9) != 9 || g.cy(9) != 9 {
		t.Fatal("at 100 % and no pan the transform is the identity")
	}
}
