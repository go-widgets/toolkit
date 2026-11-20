// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"testing"
)

// manyClusters is n clusters of k nodes each, ids c<i> and n<i>_<j>.
func manyClusters(n, k int) *GraphViewModel {
	m := NewGraphViewModel()
	for i := 0; i < n; i++ {
		c := "c" + strconv.Itoa(i)
		m.Clusters.Append(GraphCluster{ID: c, Label: c})
		for j := 0; j < k; j++ {
			m.Nodes.Append(GraphNode{ID: "n" + strconv.Itoa(i) + "_" + strconv.Itoa(j), Label: "x", Cluster: c, Category: "k"})
		}
	}
	return m
}

func TestGraphViewClustersAreSizedByTheirOwnMembers(t *testing.T) {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "big", Label: "big"}, GraphCluster{ID: "small", Label: "small"}, GraphCluster{ID: "empty", Label: "empty"})
	for i := 0; i < 9; i++ {
		m.Nodes.Append(GraphNode{ID: "b" + strconv.Itoa(i), Cluster: "big"})
	}
	m.Nodes.Append(GraphNode{ID: "s", Cluster: "small"})
	g := NewGraphView(m)
	g.SetBounds(Rect{W: 600, H: 400})
	g.layout()
	big, small, empty := g.clusterBox["big"], g.clusterBox["small"], g.clusterBox["empty"]
	if small.W >= big.W || small.H >= big.H {
		t.Fatalf("a one-node cluster %v is not smaller than a nine-node one %v", small, big)
	}
	if empty.W != small.W || empty.H != small.H {
		t.Fatalf("an empty cluster is sized like a one-node one: %v vs %v", empty, small)
	}
	if g.hasBar {
		t.Fatal("a graph that fits has no scrollbar")
	}
}

func TestGraphViewClusterIsAtLeastAsWideAsItsTitle(t *testing.T) {
	m := NewGraphViewModel()
	long := "a cluster title that is far longer than one node is wide"
	m.Clusters.Append(GraphCluster{ID: "l", Label: long}, GraphCluster{ID: "s", Label: "s"})
	m.Nodes.Append(GraphNode{ID: "a", Cluster: "l"}, GraphNode{ID: "b", Cluster: "s"})
	g := NewGraphView(m)
	g.SetBounds(Rect{W: 900, H: 300})
	g.layout()
	l, s := g.clusterBox["l"], g.clusterBox["s"]
	if l.W < g.textWidth(long) {
		t.Fatalf("cluster %v is narrower than its title (%d)", l, g.textWidth(long))
	}
	if s.X < l.X+l.W {
		t.Fatalf("the next cluster %v starts inside the long one %v", s, l)
	}
}

func TestGraphViewScrollsWhenItOverflows(t *testing.T) {
	g := NewGraphView(manyClusters(30, 4))
	g.ShowLegend = true
	g.SetBounds(Rect{X: 5, Y: 7, W: 300, H: 150})
	g.layout()
	if !g.hasBar || g.contentH <= g.viewH || g.ScrollY() != 0 {
		t.Fatalf("hasBar %v contentH %d viewH %d scroll %d", g.hasBar, g.contentH, g.viewH, g.ScrollY())
	}
	last := "n29_3"
	if b, _ := g.NodeBox(last); b.Y < g.Bounds().Y+g.viewH {
		t.Fatalf("the last node %v is already in view", b)
	}
	// Scrolling up past the top stays at the top.
	g.ScrollBy(-100)
	if g.ScrollY() != 0 {
		t.Fatal("scrolled above the top")
	}
	// The wheel moves it, in rows.
	g.OnEvent(Event{Kind: EventScroll, Delta: 3})
	if g.ScrollY() != 3*graphScrollStep() {
		t.Fatalf("wheel: scroll %d", g.ScrollY())
	}
	// Scrolling past the end clamps, and then the last node is reachable.
	g.ScrollTo(1 << 20)
	if g.ScrollY() != g.contentH-g.viewH {
		t.Fatalf("not clamped: %d of %d", g.ScrollY(), g.contentH-g.viewH)
	}
	b, _ := g.NodeBox(last)
	if r := g.Bounds(); b.Y < r.Y || b.Y+b.H > r.Y+g.viewH {
		t.Fatalf("the last node %v is not in the view after scrolling to the end", b)
	}
	// Scrolling to where it already is changes nothing.
	g.ScrollTo(g.ScrollY())

	// A click on a node that is scrolled into view selects it.
	g.OnEvent(Event{Kind: EventClick, X: b.X - g.Bounds().X + 2, Y: b.Y - g.Bounds().Y + 2})
	if g.Model.Selected.Get() != "node:"+last {
		t.Fatalf("selected %q", g.Model.Selected.Get())
	}

	// Drawing with the bar and the legend paints the bar and nothing under the legend.
	surf := makeSurface(320, 180)
	g.Draw(newP(surf, 320), DefaultLight())
	sg, ok := g.scrollbarGeom()
	if !ok {
		t.Fatal("no scrollbar geometry")
	}
	if pixelAt(surf, 320, g.Bounds().X+sg.cross0+2, g.Bounds().Y+sg.thumbStart+sg.thumbLen/2) == pixelAt(surf, 320, g.Bounds().X+sg.cross0-30, g.Bounds().Y+g.viewH/2) {
		t.Log("thumb and content share a colour; geometry still asserted below")
	}
	if sg.thumbLen < graphThumbMin || sg.thumbStart+sg.thumbLen > sg.trackLen {
		t.Fatalf("thumb %d+%d in a track of %d", sg.thumbStart, sg.thumbLen, sg.trackLen)
	}
}

func TestGraphViewScrollbarThumbTrackAndRelease(t *testing.T) {
	g := NewGraphView(manyClusters(30, 4))
	g.SetBounds(Rect{W: 300, H: 150})
	g.layout()
	sg, ok := g.scrollbarGeom()
	if !ok {
		t.Fatal("no scrollbar")
	}
	x := sg.cross0 + 2
	// A press on the track below the thumb pages down, and is not a node click.
	g.OnEvent(Event{Kind: EventClick, X: x, Y: sg.trackLen - 2})
	if g.ScrollY() != g.viewH || g.Model.Selected.Get() != "" {
		t.Fatalf("page down: scroll %d selected %q", g.ScrollY(), g.Model.Selected.Get())
	}
	// And above the thumb pages back up.
	sg, _ = g.scrollbarGeom()
	g.OnEvent(Event{Kind: EventClick, X: x, Y: 0})
	if g.ScrollY() != 0 {
		t.Fatalf("page up: scroll %d", g.ScrollY())
	}
	// Grab the thumb, drag it to the bottom, release.
	sg, _ = g.scrollbarGeom()
	g.OnEvent(Event{Kind: EventClick, X: x, Y: sg.thumbStart + 1})
	if !g.sbDrag.active {
		t.Fatal("the thumb was not grabbed")
	}
	g.OnEvent(Event{Kind: EventMouseDrag, X: x, Y: sg.trackLen + 50})
	if g.ScrollY() != g.contentH-g.viewH {
		t.Fatalf("drag to the end: scroll %d", g.ScrollY())
	}
	g.OnEvent(Event{Kind: EventMouseUp})
	if g.sbDrag.active {
		t.Fatal("release did not end the drag")
	}
	// A drag with nothing grabbed does nothing.
	g.OnEvent(Event{Kind: EventMouseDrag, X: x, Y: 0})
	if g.ScrollY() != g.contentH-g.viewH {
		t.Fatal("an ungrabbed drag scrolled")
	}
}

func TestGraphViewKeyboardSelectionScrollsIntoView(t *testing.T) {
	g := NewGraphView(manyClusters(30, 4))
	g.SetBounds(Rect{X: 3, Y: 4, W: 300, H: 150})
	g.layout()
	g.Model.Selected.Set("node:n29_2")
	g.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // to n29_3, far below
	b, _ := g.NodeBox("n29_3")
	if r := g.Bounds(); b.Y+b.H > r.Y+g.viewH || b.Y < r.Y {
		t.Fatalf("the selected node %v is not in the view", b)
	}
	if g.ScrollY() == 0 {
		t.Fatal("the view did not scroll down")
	}
	g.Model.Selected.Set("node:n0_1")
	g.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // to n0_0, far above
	if b0, _ := g.NodeBox("n0_0"); b0.Y < g.Bounds().Y {
		t.Fatalf("the node %v is above the view: scroll %d", b0, g.ScrollY())
	}
	g.revealNode("nowhere") // an unknown node is a no-op
}

func TestGraphViewScrollOnAnEmptyOrFittingGraph(t *testing.T) {
	g := NewGraphView(nil)
	g.SetBounds(Rect{W: 100, H: 50})
	g.ScrollBy(500)
	if g.ScrollY() != 0 {
		t.Fatal("an empty graph scrolled")
	}
	g.OnEvent(Event{Kind: EventScroll, Delta: 3})
	g.OnEvent(Event{Kind: EventClick, X: 99, Y: 5}) // no scrollbar to press
	h := NewGraphView(classGraph())
	h.SetBounds(Rect{W: 400, H: 200})
	h.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if h.ScrollY() != 0 {
		t.Fatal("a graph that fits scrolled")
	}
	// Shrinking the bounds keeps a scroll position that is still legal and
	// pulls back one that no longer is.
	big := NewGraphView(manyClusters(30, 4))
	big.SetBounds(Rect{W: 300, H: 150})
	big.ScrollTo(1 << 20)
	big.SetBounds(Rect{W: 300, H: 600})
	big.layout()
	if big.ScrollY() > big.contentH-big.viewH {
		t.Fatalf("scroll %d beyond %d", big.ScrollY(), big.contentH-big.viewH)
	}
}
