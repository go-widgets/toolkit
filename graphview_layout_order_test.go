// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Clusters are ordered by the mean x of their containers' hubs, whatever
// their ids say: "ca" sorts first but sits on the right-hand hub.
func TestBandsOrdersClustersByTheirHubs(t *testing.T) {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "ca", Label: "a"})
	m.Clusters.Append(GraphCluster{ID: "cb", Label: "b"})
	m.Nodes.Append(GraphNode{ID: "h1", Shape: GraphShapeHub})
	m.Nodes.Append(GraphNode{ID: "h2", Shape: GraphShapeHub})
	m.Nodes.Append(GraphNode{ID: "a", Cluster: "ca"})
	m.Nodes.Append(GraphNode{ID: "b", Cluster: "cb"})
	m.Edges.Append(GraphEdge{ID: "1", From: "a", To: "h2"})
	m.Edges.Append(GraphEdge{ID: "2", From: "b", To: "h1"})
	pl := GraphLayoutBands.Place(m, Size{W: 600, H: 400}, testMetrics())
	if !(pl.Clusters["cb"].X < pl.Clusters["ca"].X) {
		t.Fatalf("cluster cb at %v is not left of ca at %v", pl.Clusters["cb"], pl.Clusters["ca"])
	}
}

// The mean is exact, and "none" sorts after every real position.
func TestMeanOfNothingSortsLast(t *testing.T) {
	if got := mean(nil); got != noBary {
		t.Errorf("mean(nil) = %d, want noBary", got)
	}
	if got := mean([]int{2, 4}); got != 3 {
		t.Errorf("mean(2,4) = %d, want 3", got)
	}
	if got := mean([]int{7}); got != 7 {
		t.Errorf("mean(7) = %d, want 7", got)
	}
}

// A view with no Layout uses the flow grid; SetLayout replaces it.
func TestGraphViewPlacerDefaultsToTheFlowGrid(t *testing.T) {
	g := &GraphView{}
	if _, ok := g.placer().(flowGridLayout); !ok {
		t.Fatalf("default placer is %T, want the flow grid", g.placer())
	}
	g.SetLayout(GraphLayoutBands)
	if g.placer() != GraphLayoutBands {
		t.Fatalf("placer after SetLayout is %T, want the bands layout", g.placer())
	}
}
