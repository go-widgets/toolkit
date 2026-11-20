// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// A graph drawn on a HiDPI surface grows with the metric scale: nodes, gaps and
// the scroll step double at scale 2, and more clusters fit per row as the width
// grows (the layout is a flow).
func TestGraphViewHonoursTheMetricScale(t *testing.T) {
	defer SetMetricScale(MetricScale())
	laid := func(scale float64, w int) (*GraphView, Rect) {
		SetMetricScale(scale)
		g := NewGraphView(manyClusters(6, 4))
		g.SetBounds(Rect{W: w, H: 2000})
		g.layout()
		return g, g.nodeBox["n0_0"]
	}
	_, one := laid(1, 800)
	g2, two := laid(2, 1600)
	if two.W != 2*one.W || two.H != 2*one.H {
		t.Fatalf("node %v at scale 2 is not twice %v", two, one)
	}
	if got, want := graphScrollStep(), 2*(graphNodeH+graphGap); got != want {
		t.Fatalf("scroll step %d, want %d", got, want)
	}
	if g2.clusterBox["c1"].Y != g2.clusterBox["c0"].Y {
		t.Fatal("two clusters should share a row at this width")
	}
	narrow, _ := laid(2, 400)
	if narrow.clusterBox["c1"].Y == narrow.clusterBox["c0"].Y {
		t.Fatal("a narrow surface should wrap clusters onto more rows")
	}
	wide, _ := laid(2, 3000)
	if wide.clusterBox["c5"].Y != wide.clusterBox["c0"].Y {
		t.Fatal("a wide surface should hold more cluster columns")
	}
}
