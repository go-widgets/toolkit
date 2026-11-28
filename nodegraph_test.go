// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/go-widgets/painter"
)

// ngView places a NodeGraph over m at (10, 20), 600x400. With ngPair() node a's
// output "o" is at local (160, 51), its output "p" at (160, 69); node b's input
// "i" is at local (300, 71), "j" at (300, 89). Surface = local + (10, 20).
func ngView(m *NodeGraphModel) *NodeGraph {
	g := NewNodeGraph(m)
	g.SetBounds(Rect{X: 10, Y: 20, W: 600, H: 400})
	return g
}

func ngSend(g *NodeGraph, k EventKind, x, y int) { g.OnEvent(Event{Kind: k, X: x, Y: y}) }

func ngKey(g *NodeGraph, code string) { g.OnEvent(Event{Kind: EventKeyDown, Code: code}) }

func TestNodeGraphNilModel(t *testing.T) {
	g := NewNodeGraph(nil)
	if g.Model == nil || len(g.Model.Nodes()) != 0 {
		t.Fatal("nil model")
	}
	g.SetBounds(Rect{W: 100, H: 50})
	g.Draw(newP(makeSurface(100, 50), 100), DefaultLight())
	if g.Hit(5, 5).Kind != NodeGraphHitNone {
		t.Fatal("hit in an empty graph")
	}
	ngKey(g, "ArrowRight") // no nodes: nothing to select
	ngKey(g, "Delete")
	if g.Model.Selected.Get() != "" || len(g.Children()) != 0 {
		t.Fatal("empty graph")
	}
}

func TestNodeGraphHit(t *testing.T) {
	m := ngPair()
	_ = m.Connect(NodeGraphEdge{ID: "e", FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"})
	g := ngView(m)
	cases := []struct {
		name string
		x, y int
		want NodeGraphHit
	}{
		{"output port", 170, 71, NodeGraphHit{Kind: NodeGraphHitPort, Node: "a", Port: "o", Output: true}},
		{"second output", 172, 90, NodeGraphHit{Kind: NodeGraphHitPort, Node: "a", Port: "p", Output: true}},
		{"input port", 310, 91, NodeGraphHit{Kind: NodeGraphHitPort, Node: "b", Port: "i"}},
		{"node header", 80, 45, NodeGraphHit{Kind: NodeGraphHitNode, Node: "a"}},
		{"node body", 60, 98, NodeGraphHit{Kind: NodeGraphHitNode, Node: "a"}},
		{"edge middle", 240, 81, NodeGraphHit{Kind: NodeGraphHitEdge, Edge: "e"}},
		{"edge slack", 240, 84, NodeGraphHit{Kind: NodeGraphHitEdge, Edge: "e"}},
		{"edge missed", 240, 100, NodeGraphHit{}},
		{"background", 500, 300, NodeGraphHit{}},
		{"just outside a node", 29, 70, NodeGraphHit{}},
	}
	for _, c := range cases {
		if got := g.Hit(c.x, c.y); got != c.want {
			t.Errorf("%s: Hit(%d,%d) = %+v, want %+v", c.name, c.x, c.y, got, c.want)
		}
	}
}

func TestNodeGraphHitTopmostNodeWins(t *testing.T) {
	m := NewNodeGraphModel()
	_ = m.AddNode(NodeGraphNode{ID: "under", X: 0, Y: 0})
	_ = m.AddNode(NodeGraphNode{ID: "over", X: 10, Y: 10})
	g := ngView(m)
	if h := g.Hit(10+20, 20+20); h.Node != "over" {
		t.Fatalf("overlap: %+v", h)
	}
	if h := g.Hit(10+2, 20+2); h.Node != "under" {
		t.Fatalf("only the lower node there: %+v", h)
	}
}

func TestNodeGraphHitFollowsZoomAndPan(t *testing.T) {
	m := ngPair()
	g := ngView(m)
	m.View.Set(NodeGraphViewport{Zoom: 2, PanX: -40, PanY: 10})
	// Output "o" of a: graph (160, 51) -> local (160*2-40, 51*2+10) = (280, 112).
	if h := g.Hit(10+280, 20+112); h.Kind != NodeGraphHitPort || h.Port != "o" {
		t.Fatalf("zoomed port: %+v", h)
	}
	if h := g.Hit(10+170, 20+71); h.Kind != NodeGraphHitNone && h.Port == "o" {
		t.Fatalf("the unzoomed spot still hits the port: %+v", h)
	}
}

func TestNodeGraphViewportIsSanitised(t *testing.T) {
	g := ngView(ngPair())
	for _, v := range []NodeGraphViewport{
		{Zoom: math.NaN(), PanX: math.NaN(), PanY: math.Inf(1)},
		{Zoom: 0, PanX: 1e300, PanY: -1e300},
		{Zoom: -3},
		{Zoom: math.Inf(1)},
		{Zoom: 1e9},
	} {
		g.Model.View.Set(v)
		got := g.viewport()
		if !(got.Zoom >= NodeGraphZoomMin && got.Zoom <= NodeGraphZoomMax) || math.IsNaN(got.PanX+got.PanY) || math.Abs(got.PanX) > NodeGraphMaxCoord || math.Abs(got.PanY) > NodeGraphMaxCoord {
			t.Errorf("viewport(%+v) = %+v", v, got)
		}
		g.Draw(newP(makeSurface(700, 500), 700), DefaultLight()) // and drawing it must not fail
		g.Hit(50, 50)
	}
}

func TestNodeGraphZoomKeepsThePointUnderTheAnchor(t *testing.T) {
	g := ngView(ngPair())
	g.Model.View.Set(NodeGraphViewport{Zoom: 1, PanX: 30, PanY: -12})
	before := func(ax, ay int) (float64, float64) { return g.toGraph(g.viewport(), 10+ax, 20+ay) }
	x0, y0 := before(200, 150)
	g.ZoomBy(1.5, 200, 150)
	x1, y1 := before(200, 150)
	if math.Abs(x0-x1) > 1e-9 || math.Abs(y0-y1) > 1e-9 {
		t.Fatalf("anchor moved: (%v,%v) -> (%v,%v)", x0, y0, x1, y1)
	}
	g.ZoomBy(1000, 0, 0)
	if g.viewport().Zoom != NodeGraphZoomMax {
		t.Fatal("zoom not capped")
	}
	g.ZoomBy(1e-9, 0, 0)
	if g.viewport().Zoom != NodeGraphZoomMin {
		t.Fatal("zoom not floored")
	}
	g.Reset()
	if g.Model.View.Get() != (NodeGraphViewport{Zoom: 1}) {
		t.Fatal("Reset")
	}
}

func TestNodeGraphDragMovesANodeAndReportsOnce(t *testing.T) {
	m := ngPair()
	g := ngView(m)
	var ev []NodeGraphChange
	m.Subscribe(func(c NodeGraphChange) { ev = append(ev, c) })
	ngSend(g, EventClick, 60, 30) // header of a, 20 px right of and 10 below its corner
	if m.Selected.Get() != "node:a" {
		t.Fatalf("selection %q", m.Selected.Get())
	}
	ngSend(g, EventMouseDrag, 80, 40)
	ngSend(g, EventMouseDrag, 100, 80)
	if n, _ := m.Node("a"); n.X != 60 || n.Y != 70 {
		t.Fatalf("a at (%v,%v), want (60,70): the grab offset is kept", n.X, n.Y)
	}
	if len(ev) != 0 {
		t.Fatalf("events during the drag: %v", ev)
	}
	ngSend(g, EventMouseUp, 100, 80)
	if len(ev) != 1 || ev[0] != (NodeGraphChange{NodeMoved, "a"}) {
		t.Fatalf("events after the drag: %v", ev)
	}
	ngSend(g, EventMouseUp, 100, 80) // a stray release is nothing
	if len(ev) != 1 {
		t.Fatal("second release reported")
	}
}

func TestNodeGraphClickWithoutDragDoesNotMove(t *testing.T) {
	m := ngPair()
	g := ngView(m)
	n := 0
	m.Subscribe(func(NodeGraphChange) { n++ })
	ngSend(g, EventClick, 60, 30)
	ngSend(g, EventMouseUp, 60, 30)
	ngSend(g, EventMouseDrag, 90, 90) // no press is held: ignored
	if n != 0 {
		t.Fatalf("%d events for a click", n)
	}
	// A click on an input port grabs its node.
	ngSend(g, EventClick, 300, 71)
	if m.Selected.Get() != "node:b" {
		t.Fatalf("input click selects its node: %q", m.Selected.Get())
	}
}

func TestNodeGraphDragFromOutputToInputConnects(t *testing.T) {
	m := ngPair()
	g := ngView(m)
	var ev []NodeGraphChange
	m.Subscribe(func(c NodeGraphChange) { ev = append(ev, c) })
	ngSend(g, EventClick, 160, 51)
	ngSend(g, EventMouseDrag, 230, 60)
	if g.curX != 240 || g.curY != 80 {
		t.Fatalf("pointer %d,%d", g.curX, g.curY)
	}
	g.Draw(newP(makeSurface(700, 500), 700), DefaultLight()) // the preview and the rings
	ngSend(g, EventMouseUp, 300, 71)
	if len(m.Edges()) != 1 || len(ev) != 1 || ev[0].Kind != EdgeAdded {
		t.Fatalf("edges %v events %v", m.Edges(), ev)
	}
	if e := m.Edges()[0]; e.FromNode != "a" || e.FromPort != "o" || e.ToNode != "b" || e.ToPort != "i" {
		t.Fatalf("edge %+v", e)
	}
	if g.mode != ngIdle {
		t.Fatal("still connecting")
	}
}

func TestNodeGraphIncompatibleDropIsRefusedAndReported(t *testing.T) {
	m := ngPair()
	g := ngView(m)
	ngSend(g, EventClick, 160, 51)   // o is pdf
	ngSend(g, EventMouseUp, 300, 89) // j is img
	if len(m.Edges()) != 0 {
		t.Fatal("an incompatible edge was added")
	}
	var got error
	g.OnReject = func(err error) { got = err }
	ngSend(g, EventClick, 160, 51)
	ngSend(g, EventMouseUp, 300, 89)
	if !errors.Is(got, ErrNodeGraphIncompat) {
		t.Fatalf("OnReject got %v", got)
	}
	got = nil
	ngSend(g, EventClick, 160, 51)
	ngSend(g, EventMouseUp, 300, 71)
	ngSend(g, EventClick, 160, 51)
	ngSend(g, EventMouseUp, 300, 71) // the same edge again
	if !errors.Is(got, ErrNodeGraphDuplicate) || len(m.Edges()) != 1 {
		t.Fatalf("duplicate: %v, %d edges", got, len(m.Edges()))
	}
}

func TestNodeGraphDropOnNothingOrAnOutputCancels(t *testing.T) {
	m := ngPair()
	g := ngView(m)
	called := false
	g.OnReject = func(error) { called = true }
	ngSend(g, EventClick, 160, 51)
	ngSend(g, EventMouseUp, 450, 350) // background
	ngSend(g, EventClick, 160, 51)
	ngSend(g, EventMouseUp, 160, 69) // another output
	ngSend(g, EventClick, 160, 51)
	ngSend(g, EventMouseUp, 80, 30) // a node's body, not a port
	if len(m.Edges()) != 0 || called {
		t.Fatalf("edges %d, rejected %v", len(m.Edges()), called)
	}
}

func TestNodeGraphSelectEdgeAndDeleteByKeyboard(t *testing.T) {
	m := ngPair()
	_ = m.Connect(NodeGraphEdge{ID: "e", FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"})
	g := ngView(m)
	ngSend(g, EventClick, 230, 61)
	if m.Selected.Get() != "edge:e" || g.mode != ngIdle {
		t.Fatalf("selected %q", m.Selected.Get())
	}
	ngSend(g, EventMouseDrag, 100, 100) // dragging an edge moves nothing
	ngSend(g, EventMouseUp, 100, 100)
	ngKey(g, "Delete")
	if len(m.Edges()) != 0 || m.Selected.Get() != "" {
		t.Fatal("Delete did not remove the edge")
	}
	_ = m.Connect(NodeGraphEdge{ID: "e", FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"})
	ngSend(g, EventClick, 80, 30)
	ngKey(g, "Backspace")
	if len(m.Nodes()) != 1 || len(m.Edges()) != 0 {
		t.Fatalf("Backspace: %d nodes %d edges", len(m.Nodes()), len(m.Edges()))
	}
}

func TestNodeGraphEscapeCancelsAConnectionAndClearsTheSelection(t *testing.T) {
	m := ngPair()
	g := ngView(m)
	ngSend(g, EventClick, 160, 51)
	ngKey(g, "Escape")
	ngSend(g, EventMouseUp, 300, 71) // released on a compatible input after Escape
	if len(m.Edges()) != 0 {
		t.Fatal("connected after Escape")
	}
	ngSend(g, EventClick, 80, 30)
	ngKey(g, "Escape")
	if m.Selected.Get() != "" {
		t.Fatal("Escape kept the selection")
	}
}

func TestNodeGraphBackgroundDragPansAndClearsSelection(t *testing.T) {
	m := ngPair()
	g := ngView(m)
	m.Selected.Set("node:a")
	ngSend(g, EventClick, 400, 300)
	if m.Selected.Get() != "" {
		t.Fatal("background click kept the selection")
	}
	ngSend(g, EventMouseDrag, 430, 290)
	ngSend(g, EventMouseDrag, 450, 280)
	ngSend(g, EventMouseUp, 450, 280)
	if v := m.View.Get(); v.PanX != 50 || v.PanY != -20 || v.Zoom != 1 {
		t.Fatalf("view %+v", v)
	}
}

func TestNodeGraphWheel(t *testing.T) {
	m := ngPair()
	g := ngView(m)
	g.OnEvent(Event{Kind: EventScroll, X: 100, Y: 100, Delta: -2, Ctrl: true})
	if z := m.View.Get().Zoom; math.Abs(z-1.21) > 1e-9 {
		t.Fatalf("zoom after two notches in: %v", z)
	}
	g.Reset()
	g.OnEvent(Event{Kind: EventScroll, X: 100, Y: 100, Delta: 1, Meta: true})
	if m.View.Get().Zoom >= 1 {
		t.Fatal("wheel down must zoom out")
	}
	g.Reset()
	g.OnEvent(Event{Kind: EventScroll, Delta: 2, DeltaX: -1})
	if v := m.View.Get(); v.PanY != -2*ngPanStep || v.PanX != ngPanStep {
		t.Fatalf("pan %+v", v)
	}
}

func TestNodeGraphKeyboardSelectionAndZoom(t *testing.T) {
	m := ngPair()
	_ = m.AddNode(NodeGraphNode{ID: "c"})
	g := ngView(m)
	want := func(s string) {
		t.Helper()
		if m.Selected.Get() != s {
			t.Fatalf("selected %q, want %q", m.Selected.Get(), s)
		}
	}
	ngKey(g, "ArrowRight")
	want("node:a")
	ngKey(g, "ArrowDown")
	want("node:b")
	ngKey(g, "ArrowRight")
	ngKey(g, "ArrowRight")
	want("node:c")
	ngKey(g, "ArrowLeft")
	want("node:b")
	ngKey(g, "ArrowUp")
	ngKey(g, "ArrowUp")
	want("node:a")
	m.Selected.Set("edge:e")
	ngKey(g, "ArrowLeft") // nothing is selected as a node: the first one
	want("node:a")
	ngKey(g, "Tab") // ignored

	ngKey(g, "+")
	if m.View.Get().Zoom <= 1 {
		t.Fatal("+ did not zoom in")
	}
	ngKey(g, "-")
	ngKey(g, "-")
	if m.View.Get().Zoom >= 1 {
		t.Fatal("- did not zoom out")
	}
	ngKey(g, "0")
	if m.View.Get() != (NodeGraphViewport{Zoom: 1}) {
		t.Fatal("0 did not reset")
	}
	for _, c := range []string{"=", "Equal", "NumpadAdd", "_", "Minus", "NumpadSubtract", "Digit0", "Numpad0"} {
		ngKey(g, c)
	}
}

func TestNodeGraphDisabledIgnoresInput(t *testing.T) {
	m := ngPair()
	g := ngView(m)
	g.Disabled().Set(true)
	ngSend(g, EventClick, 80, 30)
	ngKey(g, "ArrowRight")
	if m.Selected.Get() != "" || !g.A11y().Disabled {
		t.Fatal("a disabled graph reacted")
	}
}

func TestNodeGraphDrawPixels(t *testing.T) {
	m := ngPair()
	_ = m.Connect(NodeGraphEdge{ID: "e", FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"})
	g := ngView(m)
	th := DefaultLight()
	draw := func() []byte {
		buf := makeSurface(700, 500)
		g.Draw(newP(buf, 700), th)
		return buf
	}
	buf := draw()
	if c := pixelAt(buf, 700, 500, 300); c != th.Background {
		t.Fatalf("background %v want %v", c, th.Background)
	}
	if c := pixelAt(buf, 700, 60, 98); c != th.Surface {
		t.Fatalf("node body %v want %v", c, th.Surface)
	}
	if c := pixelAt(buf, 700, 100, 40); c != th.Border {
		t.Fatalf("node border %v want %v", c, th.Border)
	}
	if c := pixelAt(buf, 700, 170, 71); c != ngColor("pdf") {
		t.Fatalf("pdf port %v want %v", c, ngColor("pdf"))
	}
	if c := pixelAt(buf, 700, 170, 89); c != ngColor("img") {
		t.Fatalf("img port %v want %v", c, ngColor("img"))
	}
	if pixelAt(buf, 700, 240, 81) == th.Background {
		t.Fatal("the edge was not drawn between the nodes")
	}
	if !bytes.Equal(buf, draw()) {
		t.Fatal("the same graph does not draw the same pixels")
	}
	m.Selected.Set("node:a")
	if c := pixelAt(draw(), 700, 100, 40); c != th.Accent {
		t.Fatalf("selected node border %v want %v", c, th.Accent)
	}
	m.Selected.Set("edge:e")
	if bytes.Equal(buf, draw()) {
		t.Fatal("a selected edge looks like an unselected one")
	}
}

func TestNodeGraphConnectingRingsCompatibleInputsOnly(t *testing.T) {
	m := ngPair()
	g := ngView(m)
	th := DefaultLight()
	buf := makeSurface(700, 500)
	ngSend(g, EventClick, 160, 51) // pdf output
	g.Draw(newP(buf, 700), th)
	// The ring is 2 px wider than the glyph (radius 5): pixel at distance 6 from i's centre.
	near := func(c RGBA) bool { // the ring's rim is anti-aliased: allow a few levels
		d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
		return d(c.R, th.Accent.R) < 8 && d(c.G, th.Accent.G) < 8 && d(c.B, th.Accent.B) < 8
	}
	if c := pixelAt(buf, 700, 310+6, 91); !near(c) {
		t.Fatalf("compatible input has no ring: %v (accent %v)", c, th.Accent)
	}
	if c := pixelAt(buf, 700, 310+6, 111); near(c) {
		t.Fatalf("the img input is ringed: %v", c)
	}
}

func TestNodeGraphDrawStaysInsideItsBounds(t *testing.T) {
	m := ngPair()
	_ = m.Connect(NodeGraphEdge{ID: "e", FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"})
	g := NewNodeGraph(m)
	g.SetBounds(Rect{X: 40, Y: 40, W: 300, H: 200}) // cuts both nodes and the edge
	g.SetFocused(true)
	m.View.Set(NodeGraphViewport{Zoom: 1.5, PanX: -30, PanY: 10})
	buf := makeSurface(400, 300)
	g.Draw(newP(buf, 400), DefaultLight())
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			in := x >= 40 && x < 340 && y >= 40 && y < 240
			if !in && pixelAt(buf, 400, x, y) != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
				t.Fatalf("pixel (%d,%d) outside the bounds was painted", x, y)
			}
		}
	}
}

func TestNodeGraphDrawZoomedOutOmitsTextAndCullsOffscreenNodes(t *testing.T) {
	m := ngPair()
	_ = m.AddNode(NodeGraphNode{ID: "far", X: 5000, Y: 5000})
	g := ngView(m)
	m.View.Set(NodeGraphViewport{Zoom: 0.4})
	th := DefaultLight()
	buf := makeSurface(700, 500)
	g.Draw(newP(buf, 700), th)
	// At 0.4 the header text (clipped to the header) is not drawn: the header is a flat fill.
	hx, hy := 10+int(20*0.4)+30, 20+int(20*0.4)+2
	want := pixelAt(buf, 700, hx, hy)
	for x := hx - 8; x <= hx+8; x++ {
		if pixelAt(buf, 700, x, hy) != want {
			t.Fatal("text drawn at zoom 0.4")
		}
	}
	g.Children() // the culled node has no proxy
	for _, c := range g.Children() {
		if c.(*isoProxy).id == "node:far" {
			t.Fatal("an off-screen node has a proxy")
		}
	}
}

func TestNodeGraphDrawOnACellPainter(t *testing.T) {
	m := ngPair()
	_ = m.Connect(NodeGraphEdge{ID: "e", FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"})
	g := NewNodeGraph(m)
	g.SetBounds(Rect{W: 80, H: 25})
	m.View.Set(NodeGraphViewport{Zoom: 0.25})
	ngSend(g, EventClick, 5, 5)
	g.Draw(painter.NewCellPainter(80, 25), DefaultLight()) // no PathPainter: line segments
	m.Selected.Set("edge:e")
	g.Draw(painter.NewCellPainter(80, 25), DefaultLight())
}

func TestNodeGraphA11y(t *testing.T) {
	m := ngPair()
	_ = m.AddNode(NodeGraphNode{ID: "bare", X: 400, Y: 300, Inputs: []NodeGraphPort{{ID: "only"}}})
	_ = m.Connect(NodeGraphEdge{ID: "e", FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"})
	g := ngView(m)
	info := g.A11y()
	if info.Role != RoleGroup || info.Name != "node graph editor" || info.Value != "3 nodes, 1 edges" {
		t.Fatalf("%+v", info)
	}
	m.Selected.Set("node:a")
	if g.A11y().Value != "3 nodes, 1 edges; selected node:a" {
		t.Fatalf("%q", g.A11y().Value)
	}
	if !g.a11yDisablesSubtree() {
		t.Fatal("the proxies stand in for the children")
	}
	kids := g.Children()
	if len(kids) != 4 {
		t.Fatalf("%d proxies, want 3 nodes + 1 edge", len(kids))
	}
	byID := map[string]A11yInfo{}
	for _, k := range kids {
		p := k.(*isoProxy)
		byID[p.id] = p.A11y()
		if p.Bounds().W <= 0 {
			t.Fatalf("%s has no bounds", p.id)
		}
	}
	if a := byID["node:a"]; a.Role != RoleTreeItem || a.Value != "selected" || a.Name != "Alpha, inputs: ; outputs: out, pic" {
		t.Fatalf("node a: %+v", a)
	}
	if a := byID["node:b"]; a.Name != "Beta, inputs: in, img; outputs: " || a.Value != "" {
		t.Fatalf("node b: %+v", a)
	}
	if a := byID["node:bare"]; a.Name != "bare, inputs: only; outputs: " {
		t.Fatalf("an unlabelled node and port are named by their ids: %+v", a)
	}
	m.Selected.Set("edge:e")
	for _, k := range g.Children() {
		if p := k.(*isoProxy); p.id == "edge:e" {
			if a := p.A11y(); a.Role != RoleListItem || a.Name != "edge from Alpha to Beta" || a.Value != "selected" {
				t.Fatalf("edge: %+v", a)
			}
		}
	}
	m.View.Set(NodeGraphViewport{Zoom: 1, PanX: -5000}) // everything scrolled out
	if len(g.Children()) != 0 {
		t.Fatal("proxies for what is not on screen")
	}
}

func TestNodeGraphEdgeProxyForAnEdgeThatIsHalfVisible(t *testing.T) {
	m := ngPair()
	_ = m.Connect(NodeGraphEdge{ID: "e", FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"})
	g := ngView(m)
	m.View.Set(NodeGraphViewport{Zoom: 1, PanX: -200}) // a is gone, b and the edge's end remain
	var edge bool
	for _, c := range g.Children() {
		edge = edge || c.(*isoProxy).id == "edge:e"
	}
	if !edge {
		t.Fatal("no proxy for a visible edge end")
	}
}

func TestNodeGraphPortIndex(t *testing.T) {
	ports := []NodeGraphPort{{ID: "x"}, {ID: "y"}}
	if portIndex(ports, "y") != 1 || portIndex(ports, "z") != -1 || portIndex(nil, "x") != -1 {
		t.Fatal("portIndex")
	}
}

func TestNodeGraphColourOfATypeIsStable(t *testing.T) {
	if ngColor("pdf") != ngColor("pdf") || ngColor("") == (RGBA{}) {
		t.Fatal("ngColor")
	}
	seen := map[RGBA]bool{}
	for i := 0; i < 64; i++ {
		seen[ngColor("type"+strconv.Itoa(i))] = true
	}
	if len(seen) < 4 {
		t.Fatalf("only %d colours over 64 types", len(seen))
	}
}

// 200 nodes with 3 ports a side and ~300 edges: the size the playground's
// workflow editor never reaches, so the numbers are a ceiling.
func ngBig() (*NodeGraph, []byte) {
	m := NewNodeGraphModel()
	ports := []NodeGraphPort{{ID: "a", Label: "a", Type: "t"}, {ID: "b", Label: "b", Type: "t"}, {ID: "c", Label: "c", Type: "t"}}
	for i := 0; i < 200; i++ {
		_ = m.AddNode(NodeGraphNode{ID: strconv.Itoa(i), Label: "node " + strconv.Itoa(i), X: float64(i%20) * 170, Y: float64(i/20) * 120, Inputs: ports, Outputs: ports})
	}
	for i := 0; i < 190; i++ {
		_ = m.Connect(NodeGraphEdge{FromNode: strconv.Itoa(i), FromPort: "a", ToNode: strconv.Itoa(i + 10), ToPort: "b"})
		_ = m.Connect(NodeGraphEdge{FromNode: strconv.Itoa(i), FromPort: "c", ToNode: strconv.Itoa(i + 1), ToPort: "c"})
	}
	g := NewNodeGraph(m)
	g.SetBounds(Rect{W: 1280, H: 800})
	m.View.Set(NodeGraphViewport{Zoom: 0.5})
	return g, makeSurface(1280, 800)
}

func BenchmarkNodeGraphDraw200(b *testing.B) {
	g, buf := ngBig()
	p, th := newP(buf, 1280), DefaultLight()
	g.Draw(p, th)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Draw(p, th)
	}
}

func BenchmarkNodeGraphHit200(b *testing.B) {
	g, _ := ngBig()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Hit(640+i%7, 400) // mostly the worst case: the background, so every edge is tested
	}
}

func TestNodeGraph200Nodes(t *testing.T) {
	g, buf := ngBig()
	if len(g.Model.Edges()) != 380 {
		t.Fatalf("%d edges", len(g.Model.Edges()))
	}
	g.Draw(newP(buf, 1280), DefaultLight())
	if h := g.Hit(5, 5); h.Kind != NodeGraphHitNode || h.Node != "0" {
		t.Fatalf("%+v", h)
	}
	if n := testing.AllocsPerRun(3, func() { g.Hit(640, 400) }); n != 0 {
		t.Fatalf("Hit allocates %v times", n)
	}
}
