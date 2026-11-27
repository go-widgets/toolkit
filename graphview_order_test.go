// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"testing"

	"github.com/go-widgets/painter"
)

// textLog is a painter that remembers the order of the strings drawn through it.
type textLog struct {
	painter.Painter
	texts []string
}

func (l *textLog) Text(x, y int, s string, ink painter.RGBA) {
	l.texts = append(l.texts, s)
	l.Painter.Text(x, y, s, ink)
}

// The cluster boxes are drawn in the model's order, every time: a recording of the
// same model must be the same sequence of operations, or a captured demo of the
// screen differs from one run to the next.
func TestGraphViewDrawsClustersInTheModelsOrder(t *testing.T) {
	m := NewGraphViewModel()
	var want []string
	for i := 0; i < 8; i++ {
		id := "c" + strconv.Itoa(i)
		label := "cluster-" + strconv.Itoa(i)
		m.Clusters.Append(GraphCluster{ID: id, Label: label})
		m.Nodes.Append(GraphNode{ID: "n" + strconv.Itoa(i), Label: "", Cluster: id})
		want = append(want, label)
	}
	g := NewGraphView(m)
	g.SetBounds(Rect{W: 1200, H: 800})
	surf := makeSurface(1200, 800)
	for round := 0; round < 20; round++ {
		l := &textLog{Painter: newP(surf, 1200)}
		g.Draw(l, DefaultLight())
		if len(l.texts) < len(want) {
			t.Fatalf("round %d: %d strings drawn, want at least %d", round, len(l.texts), len(want))
		}
		for i, w := range want {
			if l.texts[i] != w {
				t.Fatalf("round %d: title %d is %q, want %q (all: %q)", round, i, l.texts[i], w, l.texts[:len(want)])
			}
		}
	}
}
