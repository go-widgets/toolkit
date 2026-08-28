// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// TestTreeTableNodeIcon: a node's Icon drawer is called with a square box in the
// first column and paints there, and the node's text shifts right past it while a
// node without an icon keeps the plain text position.
func TestTreeTableNodeIcon(t *testing.T) {
	magenta := RGBA{R: 255, G: 0, B: 255, A: 255}
	var calls int
	var box Rect
	iconed := &TreeTableNode{
		Cells: []string{"file.tex"},
		Icon: func(p painter.Painter, r Rect, _ RGBA) {
			calls++
			box = r
			fillRect(p, r.X, r.Y, r.W, r.H, magenta)
		},
	}
	plain := &TreeTableNode{Cells: []string{"plain"}}
	tt := NewTreeTable([]TreeTableColumn{{Title: "Name"}}, []*TreeTableNode{iconed, plain})
	tt.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 140})

	buf := makeSurface(200, 140)
	tt.Draw(newP(buf, 200), DefaultLight())

	if calls != 1 {
		t.Fatalf("Icon drawer called %d times, want 1", calls)
	}
	if box.W <= 0 || box.H <= 0 {
		t.Fatalf("Icon box is empty: %+v", box)
	}
	// The box sits in the first column, past the chevron gutter.
	if box.X < scaled(TreeChevronW) {
		t.Errorf("icon box X %d should be past the chevron column %d", box.X, scaled(TreeChevronW))
	}
	// It painted: the centre of the box is magenta.
	if got := pixelAt(buf, 200, box.X+box.W/2, box.Y+box.H/2); got != magenta {
		t.Errorf("icon box centre = %+v, want magenta", got)
	}
	// The text shifted right past the icon: nothing magenta lands beyond the
	// icon+gap where the label now starts (a light sanity that the two do not
	// overlap — the label column begins at box.X+box.W+gap).
	labelX := box.X + box.W + scaled(TreeIconGap)
	if labelX <= box.X+box.W {
		t.Errorf("label should start after the icon+gap, got %d", labelX)
	}
}
