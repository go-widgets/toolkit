// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestGraphViewLanes(t *testing.T) {
	lanes := edgeLanes([]GraphEdge{
		{ID: "a", From: "x", To: "y"}, {ID: "b", From: "y", To: "x"}, {ID: "c", From: "x", To: "y"}, {ID: "d", From: "x", To: "z"},
	})
	if lanes["a"] != 0 || lanes["b"] != 1 || lanes["c"] != 2 || lanes["d"] != 0 {
		t.Fatalf("%v", lanes)
	}
	for lane, want := range map[int]int{0: 0, 1: graphLaneGap, 2: -graphLaneGap, 3: 2 * graphLaneGap} {
		if got := laneOffset(lane); got != want {
			t.Errorf("lane %d: %d, want %d", lane, got, want)
		}
	}
}

// Two edges between the same nodes, one allowed and one denied, are both
// visible and both clickable.
func TestGraphViewParallelEdgesBothVisible(t *testing.T) {
	m := classGraph()
	m.Edges.Clear()
	m.Edges.Append(GraphEdge{ID: "ok", From: "c", To: "a"}, GraphEdge{ID: "no", From: "c", To: "a", State: GraphEdgeDenied})
	for _, r := range []Rect{{W: 400, H: 200}, {W: 120, H: 400}} { // horizontal, then vertical
		g := NewGraphView(m)
		g.SetBounds(r)
		th := DefaultLight()
		surf := makeSurface(r.W, r.H)
		g.Draw(newP(surf, r.W), th)
		if countInk(surf, r.W, r.H, graphDeniedInk(th)) == 0 {
			t.Fatalf("%v: the denied edge is hidden under the allowed one", r)
		}
		edges := m.Edges.Slice()
		lanes := edgeLanes(edges)
		for _, e := range edges {
			x0, y0, x1, y1, _ := g.edgeEnds(e, lanes[e.ID])
			hit := ""
			for i := 1; i < 20 && hit != "edge:"+e.ID; i++ {
				hit = g.Hit(x0+(x1-x0)*i/20, y0+(y1-y0)*i/20)
			}
			if hit != "edge:"+e.ID {
				t.Fatalf("%v: edge %s cannot be clicked", r, e.ID)
			}
		}
	}
}
