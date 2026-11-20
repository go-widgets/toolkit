// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"strings"
	"testing"
)

// crowded is one cluster of n nodes with a long title, in k categories.
func crowded(n, k int) *GraphViewModel {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "c", Label: "a cluster whose title is much longer than the view is wide"},
		GraphCluster{ID: "d", Label: "d"})
	for i := 0; i < n; i++ {
		m.Nodes.Append(GraphNode{ID: "n" + strconv.Itoa(i), Label: "a long node label", Cluster: "c", Category: "cat" + strconv.Itoa(i%k)})
	}
	m.Nodes.Append(GraphNode{ID: "x", Cluster: "d", Category: "cat0"})
	return m
}

// A cluster is never wider than the view: its nodes wrap into more rows (the box
// grows taller) and the graph still scrolls, however narrow the view.
func TestGraphViewClusterNeverWiderThanTheView(t *testing.T) {
	for _, w := range []int{400, 220, 120, 40} {
		g := NewGraphView(crowded(30, 3))
		g.ShowLegend = true
		g.SetBounds(Rect{X: 5, Y: 7, W: w, H: 300})
		if got := LayoutProblems(g); len(got) != 0 {
			t.Fatalf("width %d: %v", w, got)
		}
		cb := g.clusterBox["c"]
		if cb.W > w {
			t.Fatalf("width %d: cluster %v is wider than the view", w, cb)
		}
		if w == 120 {
			// 30 nodes one per row (a node is 72 wide, the room is 120-12): tall, so it scrolls.
			if cb.H < 30*(gNodeH()+gGap()) || !g.hasBar {
				t.Fatalf("a narrow view wraps the nodes into rows and scrolls: box %v bar %v", cb, g.hasBar)
			}
		}
		// Painting it (titles and nodes clipped) must not fail either.
		g.Draw(newP(makeSurface(w+10, 320), w+10), DefaultLight())
	}
	// Wide enough, the old layout is kept: sqrt(n) columns.
	g := NewGraphView(crowded(9, 1))
	g.SetBounds(Rect{W: 900, H: 600})
	g.layout()
	if cb := g.clusterBox["c"]; cb.W != 3*(g.nodeW()+gGap())+gGap() && cb.W < g.textWidth("a cluster whose title is much longer than the view is wide") {
		t.Fatalf("a wide view changed the cluster: %v", cb)
	}
}

// The legend wraps to as many rows as it needs, and every entry stays inside.
func TestGraphViewLegendWrapsToTheWidth(t *testing.T) {
	g := NewGraphView(crowded(8, 8))
	g.ShowLegend = true
	g.SetBounds(Rect{W: 500, H: 400})
	g.layout() // the nodes' categories are the legend's entries
	one := g.legendHeight()
	wide := len(g.legendRows())
	g.SetBounds(Rect{W: 150, H: 400})
	rows := g.legendRows()
	if len(rows) <= wide || g.legendHeight() <= one {
		t.Fatalf("a narrow legend did not wrap: %d rows (was %d), height %d (was %d)", len(rows), wide, g.legendHeight(), one)
	}
	for _, row := range rows {
		w := 0
		for _, it := range row {
			w += it.w
		}
		if len(row) > 1 && w > 150-2*gGap() {
			t.Fatalf("a legend row of %d entries is %d wide in a 150 view", len(row), w)
		}
	}
	if !rows[len(rows)-1][len(rows[len(rows)-1])-1].denied {
		t.Fatal("the denied entry is last")
	}
	g.Draw(newP(makeSurface(150, 400), 150), DefaultDark())
}

// The check itself, both ways: a graph laid out by the widget passes, and boxes
// that reach out of their cluster, or a cluster out of the view, are reported.
func TestGraphProblemsFlagsBoxesOutsideTheirFrame(t *testing.T) {
	g := NewGraphView(crowded(4, 2))
	g.SetBounds(Rect{W: 600, H: 400})
	if got := LayoutProblems(g); len(got) != 0 {
		t.Fatalf("a graph the widget laid out was reported: %v", got)
	}
	nb := g.nodeBox["n0"]
	nb.W += 500
	g.nodeBox["n0"] = nb
	cb := g.clusterBox["d"]
	cb.X = 580
	g.clusterBox["d"] = cb
	delete(g.clusterBox, "ghost") // a cluster with no box is skipped, not reported
	got := LayoutProblems(g)
	var node, cluster bool
	for _, p := range got {
		node = node || strings.Contains(p.Path, "node n0")
		cluster = cluster || strings.Contains(p.Path, "cluster d")
		if p.Kind != LayoutOutside {
			t.Fatalf("kind %v", p.Kind)
		}
	}
	if !node || !cluster {
		t.Fatalf("node out of its cluster: %v, cluster out of the view: %v (%v)", node, cluster, got)
	}
	// A cluster or node that was not laid out has no box and is skipped.
	delete(g.clusterBox, "c")
	delete(g.nodeBox, "n1")
	_ = LayoutProblems(g)
}

// scroller is a container that scrolls its child: the child may reach past it.
type scroller struct{ slack }

func (*scroller) ScrollsContent() bool { return true }

func TestLayoutProblemsIgnoresWhatAScrollViewportScrolls(t *testing.T) {
	tall := Rect{X: 0, Y: 0, W: 100, H: 900}
	sc := &scroller{slack{child: NewLabel("long"), give: tall}}
	sc.SetBounds(Rect{W: 100, H: 100})
	if got := LayoutProblems(sc); len(got) != 0 {
		t.Fatalf("a scroll viewport's content was reported: %v", got)
	}
	// The same overflow in a container that does not scroll is a defect.
	bad := &slack{child: NewLabel("long"), give: tall}
	bad.SetBounds(Rect{W: 100, H: 100})
	if got := LayoutProblems(bad); len(got) != 1 {
		t.Fatalf("an overflowing child went unreported: %v", got)
	}
	if !NewScrollView(nil).ScrollsContent() {
		t.Fatal("a ScrollView is a scroll viewport")
	}
	// A scroll viewport still reports a child nobody placed.
	lazy := &scroller{slack{child: NewLabel("x")}}
	lazy.SetBounds(Rect{W: 100, H: 100})
	if got := LayoutProblems(lazy); len(got) != 1 || got[0].Kind != LayoutUnplaced {
		t.Fatalf("%v", got)
	}
}
