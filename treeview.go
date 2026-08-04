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
// Rendering is windowed (virtualized): only the rows that fit inside
// Bounds().H are ever painted, no matter how many nodes are visible in
// the flattened (expand-aware) order. See ScrollRow.
//
// Use for file browsers, settings hierarchies, JSON inspectors,
// outline views.
type TreeView struct {
	Base
	Root       *TreeNode
	Selected   *TreeNode
	OnActivate func(node *TreeNode)
	RowHeight  int // default 18

	// ScrollRow is the index, into the current visible-flattened node
	// list, of the top row Draw paints. It's clamped on every Draw /
	// OnEvent to [0, max(0, visibleCount-windowRows)], so it's always
	// safe to set directly; prefer ScrollTo/ScrollBy for arithmetic on
	// it. When the whole tree fits in Bounds().H, ScrollRow==0 paints
	// byte-identically to a TreeView with no virtualization.
	ScrollRow int

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

// rowHeight returns the effective per-row pixel height, applying the
// same "0 means default" fallback everywhere it's needed.
func (t *TreeView) rowHeight() int {
	if t.RowHeight <= 0 {
		return 18
	}
	return t.RowHeight
}

// windowRows returns how many full rows fit inside Bounds().H at the
// effective row height. 0 when Bounds().H hasn't been set (or is
// non-positive), which callers treat as "don't virtualize" so a
// TreeView used before SetBounds keeps painting every row.
func (t *TreeView) windowRows() int {
	h := t.Bounds().H
	if h <= 0 {
		return 0
	}
	return h / t.rowHeight()
}

// clampScrollRow confines row to [0, max(0, total-window)], the range
// that always leaves the window full of real rows (or, when the tree
// is shorter than the window, pinned at 0).
func (t *TreeView) clampScrollRow(row, total, window int) int {
	return clampInt(row, 0, max(0, total-window))
}

// ScrollTo sets ScrollRow to row, clamped against the tree's current
// flattened shape + the widget's bounds.
func (t *TreeView) ScrollTo(row int) {
	t.flatten()
	t.ScrollRow = t.clampScrollRow(row, len(t.rows), t.windowRows())
}

// ScrollBy adjusts ScrollRow by delta, with the same clamping as
// ScrollTo. Negative delta scrolls up.
func (t *TreeView) ScrollBy(delta int) {
	t.ScrollTo(t.ScrollRow + delta)
}

// scrollToSelected nudges ScrollRow by the minimum amount needed to
// bring Selected back inside the visible window. A nil Selected (no
// selection yet) is a deliberate no-op: without a node to locate
// there's no valid target row, and computing one anyway is exactly
// how a stray -1 "not found" index turns into a negative ScrollRow.
func (t *TreeView) scrollToSelected() {
	if t.Selected == nil {
		return
	}
	t.flatten()
	idx := -1
	for i, row := range t.rows {
		if row.node == t.Selected {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}
	wr := t.windowRows()
	switch {
	case idx < t.ScrollRow:
		t.ScrollRow = idx
	case wr > 0 && idx >= t.ScrollRow+wr:
		t.ScrollRow = idx - wr + 1
	}
	t.ScrollRow = t.clampScrollRow(t.ScrollRow, len(t.rows), wr)
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

// Draw paints the rows in the current scroll window: flattened nodes
// [ScrollRow, ScrollRow+windowRows). When the whole tree fits inside
// Bounds().H, that window covers every row + ScrollRow clamps to 0, so
// painting is byte-identical to an unvirtualized TreeView. When it
// doesn't fit, a right-edge scrollbar track+thumb is painted too.
func (t *TreeView) Draw(p painter.Painter, theme *Theme) {
	t.flatten()
	r := t.Bounds()
	rh := t.rowHeight()
	total := len(t.rows)
	wr := t.windowRows()
	windowed := wr > 0 && total > wr
	t.ScrollRow = t.clampScrollRow(t.ScrollRow, total, wr)

	rowW := r.W
	if windowed {
		rowW = r.W - scrollbarWidth
	}
	start, end := 0, total
	if windowed {
		start = t.ScrollRow
		end = start + wr
	}

	// Only clip when actually windowing: an unclipped Draw (the whole
	// tree fits) must stay byte-identical to a pre-virtualization
	// TreeView, including for rows whose label overflows Bounds().W —
	// today that's drawn, not clipped, and this must not change it.
	var clr painter.Clipper
	if windowed {
		if c, ok := p.(painter.Clipper); ok {
			clr = c
			clr.PushClip(Rect{X: r.X, Y: r.Y, W: rowW, H: r.H})
		}
	}
	for i := start; i < end; i++ {
		row := t.rows[i]
		y := r.Y + (i-start)*rh
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
		fillRect(p, r.X, y, rowW, rh, bg)
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
	if clr != nil {
		clr.PopClip()
	}
	if windowed {
		t.drawScrollbar(p, theme, r, wr, total)
	}
}

// drawScrollbar paints the right-edge track + thumb, sized + positioned
// by the same viewport/content proportion math ScrollView uses. Only
// called when the flattened list overflows the window.
func (t *TreeView) drawScrollbar(p painter.Painter, theme *Theme, r Rect, wr, total int) {
	trackX := r.X + r.W - scrollbarWidth
	fillRect(p, trackX, r.Y, scrollbarWidth, r.H, theme.SurfaceAlt)
	thumbH := r.H * wr / total
	if thumbH < 8 {
		thumbH = 8
	}
	maxScroll := total - wr // > 0: drawScrollbar is only called when windowed
	thumbY := r.Y + t.ScrollRow*(r.H-thumbH)/maxScroll
	fillRect(p, trackX, thumbY, scrollbarWidth, thumbH, theme.Accent)
}

// NodeAt returns the TreeNode at widget-local (x, y) in the current
// visible-flattened, scrolled layout, or nil for empty space below the last
// row. It does not mutate ScrollRow (unlike OnEvent). Exposed so a host can
// hit-test a right-click and build a context menu for that node.
func (t *TreeView) NodeAt(x, y int) *TreeNode {
	if y < 0 {
		return nil
	}
	t.flatten()
	rh := t.rowHeight()
	total := len(t.rows)
	wr := t.windowRows()
	localIdx := y / rh
	if wr > 0 && total > wr && localIdx >= wr {
		return nil
	}
	idx := localIdx + t.clampScrollRow(t.ScrollRow, total, wr)
	if idx < 0 || idx >= total {
		return nil
	}
	return t.rows[idx].node
}

// Remove detaches node n from the tree, removing it from its parent's Children.
// It returns true when n was found and removed; false for a nil node, an empty
// tree, or an attempt to remove the Root (which has no parent). Exposed so a
// host can implement a "delete node" menu action without threading parent
// pointers (TreeNode has none).
func (t *TreeView) Remove(n *TreeNode) bool {
	if n == nil || t.Root == nil || n == t.Root {
		return false
	}
	return removeTreeChild(t.Root, n)
}

// removeTreeChild removes target from parent's subtree (depth-first), returning
// whether it was found.
func removeTreeChild(parent, target *TreeNode) bool {
	for i, c := range parent.Children {
		if c == target {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return true
		}
		if removeTreeChild(c, target) {
			return true
		}
	}
	return false
}

// OnEvent: a click on the chevron toggles Expanded; a click anywhere
// else on the row selects the node + fires OnActivate. Y is mapped
// through ScrollRow back to the flattened index it targets.
func (t *TreeView) OnEvent(ev Event) {
	switch ev.Kind {
	case EventScroll:
		// Native wheel scroll: shift the visible row window by Delta rows
		// (ScrollBy clamps at top + bottom).
		t.ScrollBy(ev.Delta)
		return
	case EventKeyDown:
		// Arrow / Page / Home / End scroll the tree by whole rows or pages;
		// any other key is ignored.
		handleScrollKey(t, ev.Code, t.windowRows())
		return
	case EventClick:
		// fall through to the click handling below.
	default:
		return
	}
	t.flatten()
	rh := t.rowHeight()
	total := len(t.rows)
	wr := t.windowRows()
	windowed := wr > 0 && total > wr
	t.ScrollRow = t.clampScrollRow(t.ScrollRow, total, wr)
	if ev.Y < 0 {
		return
	}
	localIdx := ev.Y / rh
	if windowed && localIdx >= wr {
		// Below the last painted row (only possible when Bounds().H
		// isn't an exact multiple of rh): nothing was drawn there.
		return
	}
	idx := localIdx + t.ScrollRow
	if idx >= total {
		return
	}
	row := t.rows[idx]
	chevronX := row.depth*TreeIndentW + 4
	if ev.X >= chevronX-3 && ev.X < chevronX+8 && len(row.node.Children) > 0 {
		row.node.Expanded = !row.node.Expanded
		// Toggling a subtree can shrink (collapse) or grow (expand) the
		// visible row count out from under ScrollRow: re-flatten +
		// re-clamp so it never points past the new end of the list.
		t.flatten()
		t.ScrollRow = t.clampScrollRow(t.ScrollRow, len(t.rows), t.windowRows())
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
