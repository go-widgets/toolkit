// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// TreeNode is one entry in a TreeView. Children are nested arbitrarily
// deep; Expanded controls whether the children are rendered.
type TreeNode struct {
	Label    string
	Expanded bool
	Children []*TreeNode

	// Anything the host wants to associate with this node (typically a
	// path, an id, or the model object). The toolkit doesn't read it.
	Data any
}

// TreeView renders a hierarchical TreeNode set as indented rows.
// Click on a row's ▶/▼ chevron toggles Expanded; click anywhere
// else on the row selects it + fires OnActivate with the clicked
// node.
//
// Use for file browsers, settings hierarchies, JSON inspectors,
// outline views.
type TreeView struct {
	Base
	Root       *TreeNode
	Selected   *TreeNode
	OnActivate func(node *TreeNode)
	RowHeight  int // default 18

	// MultiSelect enables a multi-node selection set on top of the
	// single-node Selected anchor. When false (the default), TreeView
	// behaves exactly as before: only Selected is tracked/painted.
	MultiSelect bool

	// selected is the multi-select set. Only consulted when
	// MultiSelect is true. Selected remains the "anchor" node used as
	// the start of a Shift range: it follows plain + Ctrl clicks, but
	// a Shift click (a range extension) never itself becomes the new
	// anchor, so repeated Shift clicks keep extending from the same
	// origin.
	selected map[*TreeNode]bool

	// rows is a transient flat list of (node, depth) pairs computed on
	// every Draw + OnEvent so hit-tests + paint share one definition
	// of "visible".
	rows []treeRow
}

type treeRow struct {
	node  *TreeNode
	depth int
}

// TreeChevronW is the pixel column the chevron lives in.
const TreeChevronW = 14

// TreeIndentW is the per-depth pixel indent.
const TreeIndentW = 16

// NewTreeView builds a TreeView rooted at root (which may be nil for
// an empty initial view).
func NewTreeView(root *TreeNode) *TreeView {
	return &TreeView{Root: root, RowHeight: 18}
}

// flatten populates rows by walking Root in depth-first order +
// skipping the children of any collapsed node.
func (t *TreeView) flatten() {
	t.rows = t.rows[:0]
	if t.Root == nil {
		return
	}
	t.walkTree(t.Root, 0)
}

// walkTree recurses into n + its children if Expanded.
func (t *TreeView) walkTree(n *TreeNode, depth int) {
	t.rows = append(t.rows, treeRow{n, depth})
	if !n.Expanded {
		return
	}
	for _, c := range n.Children {
		t.walkTree(c, depth+1)
	}
}

// IsSelected reports whether n is part of the multi-select set. It
// only reflects MultiSelect state; when MultiSelect is false it
// always returns false (single-select uses Selected directly).
func (t *TreeView) IsSelected(n *TreeNode) bool {
	return t.selected != nil && t.selected[n]
}

// SelectedNodes returns the multi-selected nodes in visible
// (pre-order, expanded-aware) traversal order. Empty when
// MultiSelect is false or nothing is selected.
func (t *TreeView) SelectedNodes() []*TreeNode {
	if len(t.selected) == 0 {
		return nil
	}
	t.flatten()
	out := make([]*TreeNode, 0, len(t.selected))
	for _, row := range t.rows {
		if t.selected[row.node] {
			out = append(out, row.node)
		}
	}
	return out
}

// SetSelection replaces the multi-select set with nodes. The last
// node (if any) becomes the anchor (Selected).
func (t *TreeView) SetSelection(nodes ...*TreeNode) {
	t.selected = make(map[*TreeNode]bool, len(nodes))
	for _, n := range nodes {
		t.selected[n] = true
	}
	if len(nodes) > 0 {
		t.Selected = nodes[len(nodes)-1]
	}
}

// ClearSelection empties the multi-select set. Selected (the anchor)
// is left untouched.
func (t *TreeView) ClearSelection() {
	t.selected = nil
}

// ToggleSelect flips n's membership in the multi-select set.
func (t *TreeView) ToggleSelect(n *TreeNode) {
	if n == nil {
		return
	}
	if t.selected == nil {
		t.selected = make(map[*TreeNode]bool)
	}
	if t.selected[n] {
		delete(t.selected, n)
	} else {
		t.selected[n] = true
	}
}

// SelectRange selects every node between a + b (inclusive) over the
// currently-visible flattened node order (collapsed subtrees are
// excluded, matching what the user can actually see). If either node
// isn't currently visible, SelectRange is a no-op.
func (t *TreeView) SelectRange(a, b *TreeNode) {
	t.flatten()
	ai, bi := -1, -1
	for i, row := range t.rows {
		if row.node == a {
			ai = i
		}
		if row.node == b {
			bi = i
		}
	}
	if ai == -1 || bi == -1 {
		return
	}
	if ai > bi {
		ai, bi = bi, ai
	}
	if t.selected == nil {
		t.selected = make(map[*TreeNode]bool)
	}
	for i := ai; i <= bi; i++ {
		t.selected[t.rows[i].node] = true
	}
}

// Draw paints every visible row.
func (t *TreeView) Draw(p painter.Painter, theme *Theme) {
	t.flatten()
	r := t.Bounds()
	rh := t.RowHeight
	if rh <= 0 {
		rh = 18
	}
	for i, row := range t.rows {
		y := r.Y + i*rh
		bg := theme.Surface
		ink := theme.OnSurface
		isSel := row.node == t.Selected
		if t.MultiSelect {
			isSel = t.IsSelected(row.node)
		}
		if isSel {
			bg = theme.Accent
			ink = theme.Background
		}
		fillRect(p, r.X, y, r.W, rh, bg)
		indent := r.X + row.depth*TreeIndentW
		// Chevron if the node has children: ▶ collapsed, ▼ expanded.
		// The wide base sits away from the pointing direction: for ▼
		// the widest row is at the top (y = cy-1) narrowing to the tip
		// at the bottom (y = cy+2); for ▶ the tallest column is on
		// the left (x = cx-1) narrowing to the tip on the right (x =
		// cx+2).
		if len(row.node.Children) > 0 {
			cx := indent + 4
			cy := y + rh/2
			if row.node.Expanded {
				for q := 0; q < 4; q++ {
					fillRect(p, cx-q, cy+2-q, 1+2*q, 1, ink)
				}
			} else {
				for q := 0; q < 4; q++ {
					fillRect(p, cx+2-q, cy-q, 1, 1+2*q, ink)
				}
			}
		}
		textY := y + (rh-t.glyphHeight())/2
		t.drawText(p, indent+TreeChevronW, textY, row.node.Label, ink)
	}
}

// OnEvent: a click on the chevron toggles Expanded; a click anywhere
// else on the row selects the node + fires OnActivate.
func (t *TreeView) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	t.flatten()
	rh := t.RowHeight
	if rh <= 0 {
		rh = 18
	}
	if ev.Y < 0 {
		return
	}
	idx := ev.Y / rh
	if idx >= len(t.rows) {
		return
	}
	row := t.rows[idx]
	chevronX := row.depth*TreeIndentW + 4
	if ev.X >= chevronX-3 && ev.X < chevronX+8 && len(row.node.Children) > 0 {
		row.node.Expanded = !row.node.Expanded
		return
	}
	if t.MultiSelect {
		switch {
		case ev.Shift && t.Selected != nil:
			t.SelectRange(t.Selected, row.node)
		case ev.Ctrl:
			t.ToggleSelect(row.node)
			t.Selected = row.node
		default:
			t.SetSelection(row.node)
		}
	} else {
		t.Selected = row.node
	}
	if t.OnActivate != nil {
		t.OnActivate(row.node)
	}
}
