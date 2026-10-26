// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"strings"
	"testing"
)

// classGraph: two clusters, three nodes, a normal edge a->b and a denied b->c.
func classGraph() *GraphViewModel {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "alice", Label: "Alice"}, GraphCluster{ID: "bob", Label: "Bob"})
	m.Nodes.Append(
		GraphNode{ID: "a", Label: "web", Cluster: "alice", Category: "default"},
		GraphNode{ID: "b", Label: "db", Cluster: "alice", Category: "db"},
		GraphNode{ID: "c", Label: "curl", Cluster: "bob", Category: "default"},
	)
	m.Edges.Append(
		GraphEdge{ID: "e1", From: "c", To: "a", Label: "tcp/80", Weight: 1e6},
		GraphEdge{ID: "e2", From: "c", To: "b", Label: "tcp/5432 denied", State: GraphEdgeDenied},
	)
	return m
}

func placed(m *GraphViewModel) *GraphView {
	g := NewGraphView(m)
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 200})
	return g
}

func TestGraphViewNil(t *testing.T) {
	g := NewGraphView(nil)
	if g.Model == nil || g.Model.Nodes.Len() != 0 {
		t.Fatal("nil model")
	}
	g.SetBounds(Rect{W: 100, H: 50})
	surf := makeSurface(100, 50)
	g.Draw(newP(surf, 100), DefaultLight()) // no clusters: just the background
	if _, ok := g.NodeBox("x"); ok {
		t.Fatal("node box in an empty graph")
	}
	g.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // empty order
	if g.Model.Selected.Get() != "" {
		t.Fatal("selected in an empty graph")
	}
}

func TestGraphViewLayoutStableOnEdgeChange(t *testing.T) {
	m := classGraph()
	g := placed(m)
	a1, ok := g.NodeBox("a")
	if !ok {
		t.Fatal("no box for a")
	}
	// An edge update must not relayout.
	m.Edges.Append(GraphEdge{ID: "e3", From: "a", To: "c"})
	if !g.layoutValid {
		t.Fatal("an edge change invalidated the layout")
	}
	// A node in a new cluster does; positions are deterministic.
	m.Clusters.Append(GraphCluster{ID: "teacher", Label: "Teacher"})
	m.Nodes.Append(GraphNode{ID: "t", Label: "shared", Cluster: "teacher"})
	if g.layoutValid {
		t.Fatal("a node change kept the layout")
	}
	a2, _ := g.NodeBox("a")
	if a1 != a2 {
		t.Fatalf("a moved from %v to %v after a node was added elsewhere", a1, a2)
	}
	// Same bounds: layout kept; new bounds: recomputed.
	g.SetBounds(g.Bounds())
	if !g.layoutValid {
		t.Fatal("same bounds invalidated")
	}
	g.SetBounds(Rect{W: 120, H: 400})
	if g.layoutValid {
		t.Fatal("new bounds kept the layout")
	}
	b, _ := g.NodeBox("b")
	c, _ := g.NodeBox("c")
	if c.Y <= b.Y {
		t.Fatal("narrow bounds should stack clusters vertically")
	}
	// A node whose cluster is unknown is not placed.
	m.Nodes.Append(GraphNode{ID: "orphan", Cluster: "nowhere"})
	if _, ok := g.NodeBox("orphan"); ok {
		t.Fatal("orphan placed")
	}
}

func TestGraphViewRebind(t *testing.T) {
	g := placed(classGraph())
	g.layout()
	other := classGraph()
	g.Model = other
	g.layout() // rebinds: old subscriptions dropped
	if g.bound != other {
		t.Fatal("not rebound")
	}
	other.Nodes.Append(GraphNode{ID: "z", Cluster: "bob"})
	if g.layoutValid {
		t.Fatal("new model's changes not seen")
	}
}

func TestGraphViewDrawStates(t *testing.T) {
	for _, theme := range []*Theme{DefaultLight(), DefaultDark()} {
		m := classGraph()
		m.Edges.Append(GraphEdge{ID: "idle", From: "a", To: "b", State: GraphEdgeIdle},
			GraphEdge{ID: "dangling", From: "a", To: "gone"},
			GraphEdge{ID: "vertical", From: "a", To: "a"})
		m.Nodes.Append(GraphNode{ID: "x", Label: "x", Cluster: "nowhere"})
		g := placed(m)
		g.ShowLegend = true
		g.layoutValid = false
		g.Palette = []RGBA{RGB(0x10, 0x10, 0x60), RGB(0xF0, 0xF0, 0xA0)} // a dark and a light swatch
		m.Selected.Set("edge:e1")
		m.Filter.Set("db")
		g.SetFocused(true)
		surf := makeSurface(400, 200)
		g.Draw(newP(surf, 400), theme)
		if countInk(surf, 400, 200, graphDeniedInk(theme)) == 0 {
			t.Fatal("the denied edge is not drawn in the error colour")
		}
		if countInk(surf, 400, 200, theme.Accent) == 0 {
			t.Fatal("no selection / focus colour")
		}
		m.Selected.Set("node:a")
		m.Filter.Set("")
		g.Draw(newP(surf, 400), theme)
	}
}

// Narrow bounds stack the clusters, so c -> a runs vertically; wide bounds
// leave more columns than clusters.
func TestGraphViewDrawShapes(t *testing.T) {
	for _, r := range []Rect{{W: 120, H: 400}, {W: 2000, H: 100}} {
		g := NewGraphView(classGraph())
		g.SetBounds(r)
		surf := makeSurface(r.W, r.H)
		g.Draw(newP(surf, r.W), DefaultLight())
		if countInk(surf, r.W, r.H, DefaultLight().OnBackground) == 0 {
			t.Fatalf("%v: no edge ink", r)
		}
	}
}

func TestGraphViewDeniedInk(t *testing.T) {
	th := DefaultLight()
	th.Extra = map[string]RGBA{"error_color": RGB(1, 2, 3)}
	if graphDeniedInk(th) != RGB(1, 2, 3) {
		t.Fatal("theme error colour ignored")
	}
	if graphDeniedInk(DefaultLight()) == graphDeniedInk(DefaultDark()) {
		t.Fatal("one red for light and dark")
	}
}

func TestGraphViewEdgeWidth(t *testing.T) {
	g := NewGraphView(nil)
	if g.edgeWidth(0) != 1 || g.edgeWidth(-5) != 1 {
		t.Fatal("idle width")
	}
	if g.edgeWidth(1e3) >= g.edgeWidth(1e9) {
		t.Fatal("width does not grow with weight")
	}
	if g.edgeWidth(1e300) != 6 {
		t.Fatal("default cap")
	}
	g.MaxEdgeWidth = 2
	if g.edgeWidth(1e300) != 2 {
		t.Fatal("custom cap")
	}
}

func TestGraphViewCategories(t *testing.T) {
	g := NewGraphView(nil)
	a := g.categoryColor("default")
	if g.categoryColor("db") == a || g.categoryColor("default") != a {
		t.Fatal("category colours not stable and distinct")
	}
	g.Palette = []RGBA{RGB(9, 9, 9)}
	if g.categoryColor("x") != RGB(9, 9, 9) {
		t.Fatal("custom palette wraps")
	}
}

func TestGraphViewMatches(t *testing.T) {
	g := NewGraphView(nil)
	n := GraphNode{Label: "Web", Category: "public"}
	if !g.matches(n, "Alice") {
		t.Fatal("empty filter")
	}
	for f, want := range map[string]bool{"we": true, "ALI": true, "pub": true, "zzz": false} {
		g.Model.Filter.Set(f)
		if g.matches(n, "Alice") != want {
			t.Errorf("filter %q", f)
		}
	}
}

func TestGraphViewHitAndClick(t *testing.T) {
	m := classGraph()
	g := placed(m)
	a, _ := g.NodeBox("a")
	if got := g.Hit(a.X+1, a.Y+1); got != "node:a" {
		t.Fatalf("hit node: %q", got)
	}
	// A point on e1 (c -> a) outside every node box hits the edge.
	c, _ := g.NodeBox("c")
	ax, ay := graphCenter(a)
	cx, cy := graphCenter(c)
	found := false
	for i := 1; i < 20 && !found; i++ {
		px, py := cx+(ax-cx)*i/20, cy+(ay-cy)*i/20
		if g.Hit(px, py) == "edge:e1" {
			found = true
		}
	}
	if !found {
		t.Fatal("no point of e1 hits it")
	}
	if got := g.Hit(399, 199); got != "" {
		t.Fatalf("hit nothing: %q", got)
	}
	var activated string
	g.OnActivate = func(s string) { activated = s }
	g.OnEvent(Event{Kind: EventClick, X: a.X + 1, Y: a.Y + 1})
	if m.Selected.Get() != "node:a" || activated != "" {
		t.Fatal("first click selects")
	}
	g.OnEvent(Event{Kind: EventClick, X: a.X + 1, Y: a.Y + 1})
	if activated != "node:a" {
		t.Fatal("second click activates")
	}
	g.OnEvent(Event{Kind: EventMouseMove}) // ignored
	g.Disabled().Set(true)
	g.OnEvent(Event{Kind: EventClick, X: 399, Y: 199})
	if m.Selected.Get() != "node:a" {
		t.Fatal("disabled graph reacted")
	}
	m.Edges.Append(GraphEdge{ID: "dangling", From: "a", To: "gone"})
	g.Disabled().Set(false)
	g.OnEvent(Event{Kind: EventClick, X: 399, Y: 199})
	if m.Selected.Get() != "" {
		t.Fatal("click on nothing clears")
	}
}

func TestGraphViewKeys(t *testing.T) {
	m := classGraph()
	g := placed(m)
	var activated []string
	g.OnActivate = func(s string) { activated = append(activated, s) }
	key := func(c string) { g.OnEvent(Event{Kind: EventKeyDown, Code: c}) }
	key("Enter") // nothing selected: no activation
	key("ArrowRight")
	if m.Selected.Get() != "node:a" {
		t.Fatalf("first arrow: %q", m.Selected.Get())
	}
	key("ArrowDown")
	key("ArrowRight")
	key("ArrowRight") // clamps at the end
	if m.Selected.Get() != "node:c" {
		t.Fatalf("clamp end: %q", m.Selected.Get())
	}
	key("ArrowUp")
	key("ArrowLeft")
	key("ArrowLeft")
	if m.Selected.Get() != "node:a" {
		t.Fatalf("clamp start: %q", m.Selected.Get())
	}
	key("x") // ignored
	key("Enter")
	if len(activated) != 1 || activated[0] != "node:a" {
		t.Fatal(activated)
	}
	key("Escape")
	if m.Selected.Get() != "" {
		t.Fatal("escape")
	}
	g.OnActivate = nil
	m.Selected.Set("node:b")
	key("Enter") // no hook: nothing happens
}

func TestGraphViewA11y(t *testing.T) {
	m := classGraph()
	g := placed(m)
	a := g.A11y()
	if a.Role != RoleGroup || !strings.Contains(a.Value, "3 nodes, 2 edges, 1 denied") {
		t.Fatalf("%+v", a)
	}
	m.Selected.Set("edge:e2")
	if !strings.Contains(g.A11y().Value, "selected edge:e2") {
		t.Fatal(g.A11y().Value)
	}
}

func TestGraphViewHelpers(t *testing.T) {
	if absInt(-3) != 3 || absInt(4) != 4 {
		t.Fatal("absInt")
	}
	if segDist(1, 1, 0, 0, 0, 0) == 0 {
		t.Fatal("point segment")
	}
	if d := segDist(5, 3, 0, 0, 10, 0); d != 3 {
		t.Fatal(d)
	}
	surf := makeSurface(20, 20)
	drawDashed(newP(surf, 20), 0, 0, 0, 0, RGB(1, 2, 3)) // zero length: nothing
	if countInk(surf, 20, 20, RGB(1, 2, 3)) != 0 {
		t.Fatal("zero-length dash drew")
	}
	drawDashed(newP(surf, 20), 0, 5, 19, 5, RGB(1, 2, 3))
	if n := countInk(surf, 20, 20, RGB(1, 2, 3)); n == 0 || n >= 20 {
		t.Fatalf("dashes: %d px", n)
	}
}

// BenchmarkGraphViewDraw: about 200 nodes in 40 clusters and 1000 edges, one
// frame. The layout is computed once (outside the loop), as in steady state.
func BenchmarkGraphViewDraw(b *testing.B) {
	m := NewGraphViewModel()
	for c := range 40 {
		m.Clusters.Append(GraphCluster{ID: "s" + strconv.Itoa(c), Label: "student " + strconv.Itoa(c)})
		for k := range 5 {
			m.Nodes.Append(GraphNode{ID: strconv.Itoa(c*5 + k), Label: "c" + strconv.Itoa(k), Cluster: "s" + strconv.Itoa(c), Category: "g" + strconv.Itoa(k%3)})
		}
	}
	for i := range 1000 {
		st := GraphEdgeNormal
		if i%10 == 0 {
			st = GraphEdgeDenied
		}
		m.Edges.Append(GraphEdge{ID: strconv.Itoa(i), From: strconv.Itoa(i % 200), To: strconv.Itoa((i*7 + 3) % 200), Weight: float64(i * 1000), State: st})
	}
	g := NewGraphView(m)
	const w, h = 1920, 1080
	g.SetBounds(Rect{W: w, H: h})
	g.ShowLegend = true
	surf := makeSurface(w, h)
	p := newP(surf, w)
	th := DefaultDark()
	g.Draw(p, th)
	b.ResetTimer()
	for range b.N {
		g.Draw(p, th)
	}
}
