// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// An empty model places nothing, in either layout, and does not panic.
func TestLayoutsPlaceAnEmptyModel(t *testing.T) {
	for name, l := range map[string]GraphLayout{"flow": GraphLayoutFlowGrid, "bands": GraphLayoutBands} {
		pl := l.Place(NewGraphViewModel(), Size{W: 300, H: 200}, testMetrics())
		if len(pl.Nodes) != 0 || len(pl.Clusters) != 0 || len(pl.Order) != 0 || pl.HasBar {
			t.Errorf("%s: empty model placed %+v", name, pl)
		}
	}
}

// A container in an unknown cluster is not placed, even when a hub names it:
// the hub still is, and nothing else moves.
func TestBandsSkipsAnUnplacedContainerOnAHub(t *testing.T) {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "c", Label: "c"})
	m.Nodes.Append(GraphNode{ID: "h", Shape: GraphShapeHub})
	m.Nodes.Append(GraphNode{ID: "a", Cluster: "c"})
	m.Nodes.Append(GraphNode{ID: "lost", Cluster: "nowhere"})
	m.Edges.Append(GraphEdge{ID: "1", From: "a", To: "h"})
	m.Edges.Append(GraphEdge{ID: "2", From: "lost", To: "h"})
	pl := GraphLayoutBands.Place(m, Size{W: 600, H: 400}, testMetrics())
	if _, ok := pl.Nodes["lost"]; ok {
		t.Error("a container in an unknown cluster was placed")
	}
	if _, ok := pl.Nodes["h"]; !ok {
		t.Error("the hub was not placed")
	}
	if len(pl.Order) != 2 {
		t.Errorf("reading order = %v, want the hub and its container", pl.Order)
	}
}

// A view whose model empties drops its scroll, pan and content size.
func TestGraphViewResetsScrollWhenThePlacementIsEmpty(t *testing.T) {
	m := NewGraphViewModel()
	c, n, e := networkModel()
	fillNetworkModel(m, c, n, e)
	g := NewGraphView(m)
	g.SetBounds(Rect{W: 600, H: 400})
	g.SetLayout(GraphLayoutBands)
	surf := makeSurface(600, 400)
	g.Draw(newP(surf, 600), DefaultLight())
	if g.contentW == 0 {
		t.Fatal("the populated view has no content width")
	}
	g.scrollY, g.panX = 5, 5
	g.Model = NewGraphViewModel()
	g.layoutValid = false
	g.Draw(newP(surf, 600), DefaultLight())
	if g.scrollY != 0 || g.panX != 0 || g.contentW != 0 || g.viewH != 0 {
		t.Errorf("scrollY %d panX %d contentW %d viewH %d after the model emptied, want all 0", g.scrollY, g.panX, g.contentW, g.viewH)
	}
}
