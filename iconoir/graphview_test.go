// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package iconoir

import (
	"slices"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

func newPainter(buf []byte, w, h int) painter.Painter { return painter.NewPixelPainter(buf, w, h) }

// TestGraphViewTakesDrawerAsItsIconSource: `g.Icons = iconoir.Drawer` is the
// whole wiring; a node, a toggle and a badge each paint real Iconoir ink.
func TestGraphViewTakesDrawerAsItsIconSource(t *testing.T) {
	const w, h = 400, 200
	view := func(icons bool) []byte {
		m := toolkit.NewGraphViewModel()
		m.Clusters.Append(toolkit.GraphCluster{ID: "c", Label: ""})
		m.Nodes.Append(toolkit.GraphNode{ID: "a", Cluster: "c", Category: "k", Icon: "airplane",
			Toggles: []toolkit.GraphToggle{{ID: "t", Icon: "airplane", On: true}},
			Badges:  []toolkit.GraphBadge{{Icon: "airplane"}}})
		g := toolkit.NewGraphView(m)
		if icons {
			g.Icons = Drawer
		}
		g.SetBounds(toolkit.Rect{W: w, H: h})
		buf := make([]byte, w*h*4)
		g.Draw(newPainter(buf, w, h), toolkit.DefaultLight())
		return buf
	}
	with, without := view(true), view(false)
	if slices.Equal(with, without) {
		t.Fatal("setting Icons = Drawer changed no pixel")
	}
}
