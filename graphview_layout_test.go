// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math/rand"
	"reflect"
	"strconv"
	"testing"
)

// networkModel is a compose-like graph: two networks (hubs), the host (a
// gateway), and containers of two projects, one of them on both networks.
func networkModel() (clusters []GraphCluster, nodes []GraphNode, edges []GraphEdge) {
	clusters = []GraphCluster{{ID: "p2", Label: "shop"}, {ID: "p1", Label: "blog"}, {ID: "p3", Label: "empty"}}
	nodes = []GraphNode{
		{ID: "net-front", Label: "frontend", Shape: GraphShapeHub},
		{ID: "net-back", Label: "backend", Shape: GraphShapeHub},
		{ID: "host", Label: "this tab", Shape: GraphShapeGateway},
		{ID: "web", Label: "web", Cluster: "p1"},
		{ID: "db", Label: "db", Cluster: "p1"},
		{ID: "api", Label: "api", Cluster: "p2"},
		{ID: "cache", Label: "cache", Cluster: "p2"},
		{ID: "lost", Label: "lost", Cluster: "nowhere"},
	}
	edges = []GraphEdge{
		{ID: "e1", From: "web", To: "net-front"},
		{ID: "e2", From: "web", To: "net-back"},
		{ID: "e3", From: "net-back", To: "db"},
		{ID: "e4", From: "api", To: "net-back"},
		{ID: "e5", From: "host", To: "web"},
	}
	return
}

func fillNetworkModel(m *GraphViewModel, clusters []GraphCluster, nodes []GraphNode, edges []GraphEdge) {
	for _, c := range clusters {
		m.Clusters.Append(c)
	}
	for _, n := range nodes {
		m.Nodes.Append(n)
	}
	for _, e := range edges {
		m.Edges.Append(e)
	}
}

func testMetrics() GraphMetrics {
	return GraphMetrics{NodeW: 72, NodeH: 18, Gap: 6, TitleH: 14, TitleW: func(c GraphCluster) int { return 6 * len(c.Label) }}
}

func TestBandsPlacesGatewayThenHubsThenClusters(t *testing.T) {
	m := NewGraphViewModel()
	c, n, e := networkModel()
	fillNetworkModel(m, c, n, e)
	pl := GraphLayoutBands.Place(m, Size{W: 600, H: 400}, testMetrics())
	host, front, back := pl.Nodes["host"], pl.Nodes["net-front"], pl.Nodes["net-back"]
	if !(host.Y+host.H < front.Y) {
		t.Fatalf("gateway %v is not above the hubs %v", host, front)
	}
	if front.Y != back.Y {
		t.Fatalf("hubs on different rows: %v %v", front, back)
	}
	for _, id := range []string{"web", "db", "api", "cache"} {
		b := pl.Nodes[id]
		if b.Y < front.Y+front.H {
			t.Errorf("container %s at %v is not below the hubs", id, b)
		}
	}
	if _, ok := pl.Nodes["lost"]; ok {
		t.Error("a container in an unknown cluster was placed")
	}
	if len(pl.Order) != 7 || pl.Order[0] != "host" {
		t.Errorf("reading order = %v, want the gateway first and 7 nodes", pl.Order)
	}
	if pl.Height <= 0 || pl.Width <= 0 || pl.HasBar {
		t.Errorf("content %dx%d bar %v", pl.Width, pl.Height, pl.HasBar)
	}
	if _, ok := pl.Clusters["p3"]; !ok {
		t.Error("an empty cluster is still boxed")
	}
}

// Containers are ordered by the mean x of their hubs, whatever their ids say.
func TestBandsOrdersContainersByTheirHubs(t *testing.T) {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "c", Label: "c"})
	m.Nodes.Append(GraphNode{ID: "h1", Shape: GraphShapeHub})
	m.Nodes.Append(GraphNode{ID: "h2", Shape: GraphShapeHub})
	m.Nodes.Append(GraphNode{ID: "a", Cluster: "c"}) // sorts first, sits on h2
	m.Nodes.Append(GraphNode{ID: "b", Cluster: "c"}) // sorts second, sits on h1
	m.Nodes.Append(GraphNode{ID: "z", Cluster: "c"}) // on no hub: last
	m.Edges.Append(GraphEdge{ID: "1", From: "a", To: "h2"})
	m.Edges.Append(GraphEdge{ID: "2", From: "h1", To: "b"})
	pl := GraphLayoutBands.Place(m, Size{W: 600, H: 400}, testMetrics())
	var inCluster []string
	for _, id := range pl.Order {
		if id != "h1" && id != "h2" {
			inCluster = append(inCluster, id)
		}
	}
	if !reflect.DeepEqual(inCluster, []string{"b", "a", "z"}) {
		t.Fatalf("containers in reading order = %v, want [b a z]", inCluster)
	}
}

// Hubs follow their containers on the second pass: a hub whose containers sit
// to the right of another's is placed to its right.
func TestBandsSecondPassReordersHubs(t *testing.T) {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "ca", Label: "a"})
	m.Clusters.Append(GraphCluster{ID: "cb", Label: "b"})
	for _, h := range []string{"h1", "h2", "h3"} {
		m.Nodes.Append(GraphNode{ID: h, Shape: GraphShapeHub})
	}
	m.Nodes.Append(GraphNode{ID: "a", Cluster: "ca"}) // on h1 and h3
	m.Nodes.Append(GraphNode{ID: "b", Cluster: "cb"}) // on h2 only
	for i, e := range [][2]string{{"a", "h1"}, {"a", "h3"}, {"b", "h2"}} {
		m.Edges.Append(GraphEdge{ID: strconv.Itoa(i), From: e[0], To: e[1]})
	}
	pl := GraphLayoutBands.Place(m, Size{W: 600, H: 400}, testMetrics())
	// First pass: hubs by id (h1 h2 h3). Container a sits left of b, so the hubs
	// it joins (h1, h3) follow it to the left of h2.
	if !(pl.Nodes["h1"].X < pl.Nodes["h3"].X && pl.Nodes["h3"].X < pl.Nodes["h2"].X) {
		t.Fatalf("hub x h1 %d h3 %d h2 %d: hubs did not follow their containers",
			pl.Nodes["h1"].X, pl.Nodes["h3"].X, pl.Nodes["h2"].X)
	}
}

func TestBandsIsShuffleProof(t *testing.T) {
	c, n, e := networkModel()
	ref := NewGraphViewModel()
	fillNetworkModel(ref, c, n, e)
	want := GraphLayoutBands.Place(ref, Size{W: 400, H: 300}, testMetrics())
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 100; i++ {
		c2 := append([]GraphCluster(nil), c...)
		n2 := append([]GraphNode(nil), n...)
		e2 := append([]GraphEdge(nil), e...)
		rng.Shuffle(len(c2), func(a, b int) { c2[a], c2[b] = c2[b], c2[a] })
		rng.Shuffle(len(n2), func(a, b int) { n2[a], n2[b] = n2[b], n2[a] })
		rng.Shuffle(len(e2), func(a, b int) { e2[a], e2[b] = e2[b], e2[a] })
		m := NewGraphViewModel()
		fillNetworkModel(m, c2, n2, e2)
		if got := GraphLayoutBands.Place(m, Size{W: 400, H: 300}, testMetrics()); !reflect.DeepEqual(got, want) {
			t.Fatalf("shuffle %d changed the placement:\n got %+v\nwant %+v", i, got, want)
		}
	}
}

func TestBandsScrollbarReflow(t *testing.T) {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "c", Label: "c"})
	for i := 0; i < 40; i++ {
		m.Nodes.Append(GraphNode{ID: "n" + strconv.Itoa(1000+i), Cluster: "c"})
	}
	pl := GraphLayoutBands.Place(m, Size{W: 300, H: 60}, testMetrics())
	if !pl.HasBar {
		t.Fatal("tall content did not ask for a scrollbar")
	}
	if pl.Width > 300-scrollbarTrack() {
		t.Errorf("content width %d overlaps the scrollbar column", pl.Width)
	}
}

func TestBandsHubsAndGatewaysWrapInNarrowViews(t *testing.T) {
	m := NewGraphViewModel()
	for i := 0; i < 5; i++ {
		m.Nodes.Append(GraphNode{ID: "h" + strconv.Itoa(i), Shape: GraphShapeHub})
		m.Nodes.Append(GraphNode{ID: "g" + strconv.Itoa(i), Shape: GraphShapeGateway})
	}
	pl := GraphLayoutBands.Place(m, Size{W: 170, H: 2000}, testMetrics()) // two per row
	if pl.Nodes["h0"].Y == pl.Nodes["h4"].Y || pl.Nodes["g0"].Y == pl.Nodes["g4"].Y {
		t.Fatalf("bands did not wrap: %v %v", pl.Nodes["h0"], pl.Nodes["h4"])
	}
	if len(pl.Clusters) != 0 {
		t.Error("a graph of hubs only grew a cluster box")
	}
	if pl.Height <= pl.Nodes["h4"].Y {
		t.Errorf("height %d does not reach the last hub at %v", pl.Height, pl.Nodes["h4"])
	}
}

func TestGraphViewBandsDrawsAndSelectsHubs(t *testing.T) {
	m := NewGraphViewModel()
	c, n, e := networkModel()
	fillNetworkModel(m, c, n, e)
	g := NewGraphView(m)
	g.CornerRadius = 3
	g.ShowLegend = true
	g.SetBounds(Rect{W: 600, H: 400})
	g.SetLayout(GraphLayoutBands)
	surf := makeSurface(600, 400)
	g.Draw(newP(surf, 600), DefaultLight())
	hb, ok := g.NodeBox("net-back")
	if !ok {
		t.Fatal("hub has no box")
	}
	if got := g.Hit(hb.X+hb.W/2, hb.Y+hb.H/2); got != "node:net-back" {
		t.Errorf("Hit on the hub = %q", got)
	}
	if _, ok := g.NodeBox("lost"); ok {
		t.Error("node of an unknown cluster has a box")
	}
	// A layout change takes effect without a resize.
	g.SetLayout(nil)
	g.Draw(newP(surf, 600), DefaultLight())
	if _, ok := g.NodeBox("net-back"); ok {
		t.Error("the flow grid placed a hub that belongs to no cluster")
	}
}

// With no Layout, and with the flow grid named, the picture is the same.
func TestGraphViewDefaultLayoutIsTheFlowGrid(t *testing.T) {
	c, n, e := networkModel()
	boxes := func(l GraphLayout) map[string]Rect {
		m := NewGraphViewModel()
		fillNetworkModel(m, c, n, e)
		g := NewGraphView(m)
		g.Layout = l
		g.SetBounds(Rect{W: 500, H: 300})
		g.Draw(newP(makeSurface(500, 300), 500), DefaultLight())
		out := map[string]Rect{}
		for _, x := range n {
			if b, ok := g.NodeBox(x.ID); ok {
				out[x.ID] = b
			}
		}
		return out
	}
	if a, b := boxes(nil), boxes(GraphLayoutFlowGrid); !reflect.DeepEqual(a, b) || len(a) != 4 {
		t.Fatalf("nil layout %v, flow grid %v", a, b)
	}
}

type fixedLayout struct{ calls *int }

func (f fixedLayout) Place(m *GraphViewModel, v Size, mt GraphMetrics) GraphPlacement {
	*f.calls++
	return GraphPlacement{Nodes: map[string]Rect{"n": {X: 5, Y: 5, W: mt.NodeW, H: mt.NodeH}},
		Clusters: map[string]Rect{}, Order: []string{"n"}, Width: 100, Height: 50}
}

func TestGraphViewUsesACustomLayoutOnce(t *testing.T) {
	m := NewGraphViewModel()
	m.Nodes.Append(GraphNode{ID: "n", Label: "n", Shape: GraphShapeHub})
	calls := 0
	g := NewGraphView(m)
	g.Layout = fixedLayout{&calls}
	g.SetBounds(Rect{W: 300, H: 200})
	for i := 0; i < 3; i++ {
		g.Draw(newP(makeSurface(300, 200), 300), DefaultLight())
	}
	if calls != 1 {
		t.Errorf("the layout ran %d times for 3 draws, want 1", calls)
	}
	if b, ok := g.NodeBox("n"); !ok || b.X != 5 {
		t.Errorf("custom placement not used: %v %v", b, ok)
	}
}
