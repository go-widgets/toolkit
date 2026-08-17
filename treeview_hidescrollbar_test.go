// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestTreeViewScrollExtentAndHideScrollbar(t *testing.T) {
	root, _ := manyLeaves(20) // root + 20 children = 21 visible rows
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 90}) // rowHeight 18 → windowRows 5
	theme := DefaultLight()

	off, win, total, shown := tv.ScrollExtent()
	if !shown || off != 0 || win != 5 || total != 21 {
		t.Fatalf("ScrollExtent = (%d,%d,%d,%v), want (0,5,21,true)", off, win, total, shown)
	}
	tv.ScrollTo(3)
	if off, _, _, _ := tv.ScrollExtent(); off != 3 {
		t.Fatalf("offset after ScrollTo(3) = %d, want 3", off)
	}

	// A tree that fits its window reports shown=false.
	small := NewTreeView(&TreeNode{Label: "r", Expanded: true, Children: []*TreeNode{{Label: "a"}}})
	small.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 90})
	if _, _, _, shown := small.ScrollExtent(); shown {
		t.Fatal("a fitting tree must report shown=false")
	}
	// A zero-height window yields windowRows 0 and shown=false.
	small.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 0})
	if _, _, _, shown := small.ScrollExtent(); shown {
		t.Fatal("a zero-height tree must report shown=false")
	}

	// The built-in muted-grey thumb paints when shown, and HideScrollbar suppresses it.
	tv.ScrollTo(0)
	tv.HideScrollbar = false
	buf := makeSurface(120, 90)
	tv.Draw(newP(buf, 120), theme)
	if !hasColor(buf, 120, scrollbarThumbColor(theme)) {
		t.Fatal("the built-in scrollbar thumb should paint when shown")
	}
	tv.HideScrollbar = true
	buf2 := makeSurface(120, 90)
	tv.Draw(newP(buf2, 120), theme)
	if hasColor(buf2, 120, scrollbarThumbColor(theme)) {
		t.Fatal("HideScrollbar must suppress the scrollbar thumb")
	}
}
