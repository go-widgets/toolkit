// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestGraphViewNodeWidth(t *testing.T) {
	m := manyClusters(1, 4)
	g := NewGraphView(m)
	g.SetBounds(Rect{W: 800, H: 300})
	b0, _ := g.NodeBox("n0_0")
	if b0.W != graphNodeW {
		t.Fatalf("default node width %d, want %d", b0.W, graphNodeW)
	}
	box72 := g.clusterBox["c0"]
	// Changing the width after the first layout re-lays out.
	g.NodeWidth = 150
	b1, _ := g.NodeBox("n0_0")
	if b1.W != 150 {
		t.Fatalf("node width %d after NodeWidth=150", b1.W)
	}
	if g.clusterBox["c0"].W <= box72.W {
		t.Fatalf("the cluster did not widen: %v -> %v", box72, g.clusterBox["c0"])
	}
	// Nodes sit side by side without overlapping.
	b2, _ := g.NodeBox("n0_1")
	if b2.X < b1.X+b1.W {
		t.Fatalf("nodes overlap: %v %v", b1, b2)
	}
}

// A label wider than its node must not paint outside the node box.
func TestGraphViewLabelIsClippedToItsNode(t *testing.T) {
	const w, h = 420, 120
	draw := func(label string) []byte {
		m := NewGraphViewModel()
		m.Clusters.Append(GraphCluster{ID: "c", Label: "c"})
		m.Nodes.Append(GraphNode{ID: "a", Label: label, Cluster: "c", Category: "k"}, GraphNode{ID: "b", Label: "", Cluster: "c", Category: "k"})
		g := NewGraphView(m)
		g.SetBounds(Rect{W: w, H: h})
		surf := makeSurface(w, h)
		g.Draw(newP(surf, w), DefaultLight())
		return surf
	}
	short, long := draw("a"), draw("WWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWW")
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "c", Label: "c"})
	m.Nodes.Append(GraphNode{ID: "a", Cluster: "c"}, GraphNode{ID: "b", Cluster: "c"})
	g := NewGraphView(m)
	g.SetBounds(Rect{W: w, H: h})
	a, _ := g.NodeBox("a")
	differs := false
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(short, w, x, y) == pixelAt(long, w, x, y) {
				continue
			}
			if x < a.X || x >= a.X+a.W || y < a.Y || y >= a.Y+a.H {
				t.Fatalf("the long label painted outside its node %v at (%d,%d)", a, x, y)
			}
			differs = true
		}
	}
	if !differs {
		t.Fatal("the long label painted nothing")
	}
}
