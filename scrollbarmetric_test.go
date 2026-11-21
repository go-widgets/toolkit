// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"strings"
	"testing"
)

// Every scrollable widget draws and hit-tests the same scrollbar: one thickness,
// scrollbarTrack(), at every metric scale. A widget that read the unscaled
// constant instead drifted from the others as soon as the scale was not 1.
func TestEveryScrollableWidgetHasTheSameScrollbar(t *testing.T) {
	const w, h = 300, 200
	rows := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("row %d", i)
		}
		return out
	}
	cells := func(n int) [][]string {
		out := make([][]string, n)
		for i := range out {
			out[i] = []string{fmt.Sprintf("row %d", i), "x"}
		}
		return out
	}
	treeRoot := &TreeNode{Label: "root", Expanded: true}
	ttRoot := []*TreeTableNode{{Cells: []string{"root", "x"}, Expanded: true}}
	for i := 0; i < 60; i++ {
		treeRoot.Children = append(treeRoot.Children, &TreeNode{Label: fmt.Sprintf("n%d", i)})
		ttRoot[0].Children = append(ttRoot[0].Children, &TreeTableNode{Cells: []string{fmt.Sprintf("n%d", i), "x"}})
	}
	widgets := map[string]func() (Widget, func() (sbGeom, bool)){
		"ListBox": func() (Widget, func() (sbGeom, bool)) {
			l := NewListBox(rows(60))
			return l, l.scrollbarGeom
		},
		"Table": func() (Widget, func() (sbGeom, bool)) {
			tb := NewTable([]TableColumn{{Title: "a"}, {Title: "b"}}, cells(60))
			return tb, tb.scrollbarGeom
		},
		"TreeTable": func() (Widget, func() (sbGeom, bool)) {
			tt := NewTreeTable([]TreeTableColumn{{Title: "a"}, {Title: "b"}}, ttRoot)
			return tt, tt.scrollbarGeom
		},
		"TreeView": func() (Widget, func() (sbGeom, bool)) {
			tv := NewTreeView(treeRoot)
			return tv, tv.scrollbarGeom
		},
		"WrapList": func() (Widget, func() (sbGeom, bool)) {
			l := NewWrapList(rows(60))
			return l, l.scrollbarGeom
		},
		"TextView": func() (Widget, func() (sbGeom, bool)) {
			tv := NewTextView(strings.Join(rows(60), "\n"))
			return tv, tv.vbarGeom
		},
	}
	defer SetMetricScale(MetricScale())
	for _, scale := range []float64{1, 1.5, 2, 3} {
		SetMetricScale(scale)
		want := scrollbarTrack()
		for name, mk := range widgets {
			wd, geom := mk()
			wd.SetBounds(Rect{W: w, H: h})
			if tv, isText := wd.(*TextView); isText {
				tv.ShowScrollbars = true
				tv.SetText(strings.Join(rows(60), "\n"))
			}
			g, ok := geom()
			if !ok {
				t.Errorf("scale %v, %s: no scrollbar for 60 rows in %dpx", scale, name, h)
				continue
			}
			if got := g.crossWidth(); got != want {
				t.Errorf("scale %v, %s: hit area %d px wide, want %d", scale, name, got, want)
			}
			inset := 0
			if name == "TextView" {
				inset = 1 // the editor's own 1px border
			}
			if got := w - g.cross0 - inset; got != want {
				t.Errorf("scale %v, %s: track starts %d px from the right edge, want %d", scale, name, got, want)
			}
		}
	}
}
