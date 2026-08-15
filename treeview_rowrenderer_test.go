// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// treeRowRendererCall records one RowRenderer invocation for assertions.
type treeRowRendererCall struct {
	contentRect Rect
	node        *TreeNode
	selected    bool
	ink         RGBA
}

// TestTreeViewRowRendererPerRowContentRect: with a RowRenderer set, Draw calls
// it once per visible row -- instead of the default Label text -- handing each
// row the content rect AFTER the chevron + this depth's indent, the node, the
// selection state and the resolved ink. Nodes sit at different depths (0..2) and
// mix expanded/collapsed, so the depth-aware indent is exercised precisely.
func TestTreeViewRowRendererPerRowContentRect(t *testing.T) {
	root, a, b, b1, c, d, _, e := newMultiSelectTree()
	theme := DefaultLight()
	tv := NewTreeView(root)
	tv.Selected = root // depth-0 row is the selected one
	// W=200, H large enough for all 7 visible rows (7*18=126) -> no window,
	// no scrollbar gutter. X offset non-zero to prove the rect is absolute.
	tv.SetBounds(Rect{X: 3, Y: 2, W: 200, H: 200})

	var calls []treeRowRendererCall
	tv.RowRenderer = func(p painter.Painter, th *Theme, cr Rect, node *TreeNode, selected bool, ink RGBA) {
		calls = append(calls, treeRowRendererCall{cr, node, selected, ink})
	}
	buf := makeSurface(220, 200)
	tv.Draw(newP(buf, 220), theme)

	// Visible flattened order + depths of newMultiSelectTree (d collapsed).
	type want struct {
		node  *TreeNode
		depth int
	}
	wants := []want{
		{root, 0}, {a, 1}, {b, 1}, {b1, 2}, {c, 1}, {d, 1}, {e, 1},
	}
	if len(calls) != len(wants) {
		t.Fatalf("RowRenderer called %d times, want %d (one per visible row)", len(calls), len(wants))
	}
	r := tv.Bounds()
	rh := tv.rowHeight()
	for i, w := range wants {
		got := calls[i]
		if got.node != w.node {
			t.Fatalf("call %d node = %q, want %q", i, got.node.Label, w.node.Label)
		}
		// contentRect.X = X + depth*TreeIndentW + TreeChevronW (chevron+indent).
		wantX := r.X + w.depth*scaled(TreeIndentW) + scaled(TreeChevronW)
		wantY := r.Y + i*rh
		wantW := r.X + r.W - wantX // full width (no gutter): right edge - contentX
		if got.contentRect.X != wantX || got.contentRect.Y != wantY ||
			got.contentRect.W != wantW || got.contentRect.H != rh {
			t.Fatalf("call %d contentRect = %+v, want {X:%d Y:%d W:%d H:%d}",
				i, got.contentRect, wantX, wantY, wantW, rh)
		}
		wantSel := w.node == root
		if got.selected != wantSel {
			t.Fatalf("call %d selected = %v, want %v", i, got.selected, wantSel)
		}
		wantInk := theme.OnSurface
		if wantSel {
			wantInk = theme.Background
		}
		if got.ink != wantInk {
			t.Fatalf("call %d ink = %+v, want %+v", i, got.ink, wantInk)
		}
	}
}

// TestTreeViewRowRendererMultiSelectInk: in MultiSelect mode the selected flag +
// ink follow the multi-select set, not just the single Selected anchor.
func TestTreeViewRowRendererMultiSelectInk(t *testing.T) {
	theme := DefaultLight()
	root, a, _, _, c, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetSelection(a, c) // two selected, anchor becomes c
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})

	sel := map[*TreeNode]bool{}
	tv.RowRenderer = func(p painter.Painter, th *Theme, cr Rect, node *TreeNode, selected bool, ink RGBA) {
		sel[node] = selected
		wantInk := theme.OnSurface
		if selected {
			wantInk = theme.Background
		}
		if ink != wantInk {
			t.Fatalf("node %q ink = %+v, want %+v (selected=%v)", node.Label, ink, wantInk, selected)
		}
	}
	tv.Draw(newP(makeSurface(200, 200), 200), theme)

	if !sel[a] || !sel[c] {
		t.Fatalf("a/c should be selected: a=%v c=%v", sel[a], sel[c])
	}
	if sel[root] {
		t.Fatal("root should not be selected in the multi-select set")
	}
}

// TestTreeViewRowRendererWindowedGutterInset: when the tree overflows its window
// the content rect is inset by the scrollbar gutter, and only the windowed rows
// (correct scrolled nodes) are handed to the renderer.
func TestTreeViewRowRendererWindowedGutterInset(t *testing.T) {
	root, children := manyLeaves(50)
	tv := NewTreeView(root)
	rh := tv.rowHeight()
	// 4 rows visible; scroll so the window starts partway down.
	tv.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 4 * rh})
	tv.ScrollRow = 10

	var nodes []*TreeNode
	var widths []int
	tv.RowRenderer = func(p painter.Painter, th *Theme, cr Rect, node *TreeNode, selected bool, ink RGBA) {
		nodes = append(nodes, node)
		widths = append(widths, cr.W)
	}
	tv.Draw(newP(makeSurface(120, 4*rh), 120), DefaultLight())

	if len(nodes) != 4 {
		t.Fatalf("windowed render count = %d, want 4", len(nodes))
	}
	// Window [10,14): rows are root(0)=idx0 ... so flattened idx 10 == children[9]
	// (idx0 is root, idx1==children[0]). idx 10 -> children[9], depth 1.
	wantNodes := []*TreeNode{children[9], children[10], children[11], children[12]}
	for i, n := range nodes {
		if n != wantNodes[i] {
			t.Fatalf("windowed node %d = %q, want %q", i, n.Label, wantNodes[i].Label)
		}
	}
	// All these leaves are depth 1; content width = W - gutter - indent - chevron.
	wantW := 120 - scrollGutter() - 1*scaled(TreeIndentW) - scaled(TreeChevronW)
	for i, w := range widths {
		if w != wantW {
			t.Fatalf("windowed contentRect.W[%d] = %d, want %d (gutter-inset)", i, w, wantW)
		}
	}
}

// TestTreeViewRowRendererPaintsOverSelectionBackground: a renderer that fills its
// content rect paints those pixels (proving Draw routes through it), while the
// TreeView still paints the selection background it draws underneath -- pixels
// left of the content rect (indent zone of the selected row) stay the accent.
func TestTreeViewRowRendererPaintsOverSelectionBackground(t *testing.T) {
	theme := DefaultLight()
	root, a, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.Selected = a // row 1, depth 1, no children -> no chevron in the indent zone
	const w, h = 200, 200
	tv.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})

	red := RGBA{R: 220, G: 20, B: 20, A: 255}
	tv.RowRenderer = func(p painter.Painter, _ *Theme, cr Rect, node *TreeNode, _ bool, _ RGBA) {
		if node == a {
			fillRect(p, cr.X, cr.Y, cr.W, cr.H, red)
		}
	}
	buf := makeSurface(w, h)
	tv.Draw(newP(buf, w), theme)

	rh := tv.rowHeight()
	rowY := 1*rh + rh/2 // vertical middle of row 1 (a)
	contentX := 1*scaled(TreeIndentW) + scaled(TreeChevronW)
	// A pixel inside the content rect must be the renderer's red.
	if got := pixelAt(buf, w, contentX+5, rowY); got != red {
		t.Fatalf("content pixel = %+v, want renderer red %+v", got, red)
	}
	// A pixel left of the content rect (indent zone) must still be the
	// selection background the TreeView painted under the custom content.
	if got := pixelAt(buf, w, 4, rowY); got != theme.Accent {
		t.Fatalf("indent-zone pixel = %+v, want selection accent %+v", got, theme.Accent)
	}
}

// TestTreeViewRowRendererContentWidthClampsToZero: a visible row deep enough
// that the chevron + indent run past a narrow widget's right edge gets a content
// rect width of 0 (clamped), never negative.
func TestTreeViewRowRendererContentWidthClampsToZero(t *testing.T) {
	child := &TreeNode{Label: "deep"}
	root := &TreeNode{Label: "root", Expanded: true, Children: []*TreeNode{child}}
	tv := NewTreeView(root)
	// W=20 < TreeIndentW+TreeChevronW, so the depth-1 child overruns the edge.
	tv.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 200})

	widths := map[*TreeNode]int{}
	tv.RowRenderer = func(p painter.Painter, _ *Theme, cr Rect, node *TreeNode, _ bool, _ RGBA) {
		widths[node] = cr.W
	}
	tv.Draw(newP(makeSurface(20, 200), 20), DefaultLight())

	if widths[child] != 0 {
		t.Fatalf("deep child contentRect.W = %d, want 0 (clamped, never negative)", widths[child])
	}
}

// TestTreeViewRowRendererNilUnchanged: with RowRenderer nil, Draw is
// byte-identical to the default Label render (the original behaviour), so the
// zero value is a pure no-op seam.
func TestTreeViewRowRendererNilUnchanged(t *testing.T) {
	root, _, _, _, _, _, _, _ := newMultiSelectTree()
	theme := DefaultLight()

	mk := func(withNilRenderer bool) []byte {
		tv := NewTreeView(root)
		tv.Selected = root
		tv.SetBounds(Rect{X: 2, Y: 1, W: 180, H: 200})
		if withNilRenderer {
			tv.RowRenderer = nil
		}
		buf := makeSurface(200, 200)
		tv.Draw(newP(buf, 200), theme)
		return buf
	}
	a, b := mk(false), mk(true)
	if string(a) != string(b) {
		t.Fatal("nil RowRenderer must render byte-identically to the default")
	}
}

// TestTreeViewRowContentWidth: the helper reports the same content width Draw
// hands the renderer, for both the un-windowed (no gutter) and windowed
// (gutter-inset) cases, across depths.
func TestTreeViewRowContentWidth(t *testing.T) {
	// Un-windowed: everything fits, no gutter.
	root, _, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	for depth := 0; depth < 3; depth++ {
		want := 200 - depth*scaled(TreeIndentW) - scaled(TreeChevronW)
		if got := tv.RowContentWidth(depth); got != want {
			t.Fatalf("un-windowed RowContentWidth(%d) = %d, want %d", depth, got, want)
		}
	}

	// Windowed: tall tree, small viewport -> gutter inset applies.
	big, _ := manyLeaves(50)
	tw := NewTreeView(big)
	rh := tw.rowHeight()
	tw.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 3 * rh})
	for depth := 0; depth < 3; depth++ {
		want := 120 - scrollGutter() - depth*scaled(TreeIndentW) - scaled(TreeChevronW)
		if got := tw.RowContentWidth(depth); got != want {
			t.Fatalf("windowed RowContentWidth(%d) = %d, want %d", depth, got, want)
		}
	}
}

// TestTreeViewRowContentWidthClampsToZero: a depth deep enough to run the content
// column past the right edge clamps to 0 rather than going negative (matching
// Draw's contentW clamp).
func TestTreeViewRowContentWidthClampsToZero(t *testing.T) {
	root, _, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 200}) // narrower than one deep indent
	if got := tv.RowContentWidth(10); got != 0 {
		t.Fatalf("RowContentWidth(10) on a narrow tree = %d, want 0 (clamped)", got)
	}
}
