// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// TreeTableColumn is one column definition for a TreeTable header — the
// same shape as TableColumn (title + optional fixed pixel Width + Align),
// reused here so a host that already knows Table's column model doesn't
// have to learn a second one.
type TreeTableColumn struct {
	Title string
	Width int // pixels; 0 = auto (equal share of remaining space)
	Align Align
}

// TreeTableNode is one row of a TreeTable. Cells[0] is rendered in the
// first (tree) column, indented by the node's depth and prefixed with a
// disclosure glyph when it has Children; Cells[1:] render as plain,
// column-aligned text in the remaining columns exactly like a Table row.
// A node shorter than len(Columns) renders blank trailing cells, mirroring
// Table.Rows' own "short row" tolerance.
type TreeTableNode struct {
	Cells    []string
	Children []*TreeTableNode
	Expanded bool
}

// TreeTable renders a Table-shaped grid whose body rows form a TREE: a
// fixed header row of column titles sits above body rows built from the
// visible (expand-aware) flattening of Root, exactly like TreeView flattens
// its single Root node. The first column carries the tree structure
// (indentation + a ▸/▾ disclosure glyph); the rest are plain cells.
//
// Rendering is windowed (virtualized) the same way TreeView is: only the
// rows that fit inside Bounds().H (below the header) are ever painted, no
// matter how many nodes are visible in the flattened order. See ScrollRow.
//
// Use for file managers, outline-grids, or anything that's "a Table, but
// the rows nest".
type TreeTable struct {
	Base
	focusState
	// Columns are the header cells. A zero Width means "auto" — the
	// column claims an equal share of whatever pixel budget is left
	// after the fixed-Width columns, same rule as Table.Columns.
	Columns []TreeTableColumn
	// Root holds the top-level nodes (a forest, not a single root, so a
	// host can list multiple top-level entries without a synthetic
	// invisible parent).
	Root []*TreeTableNode
	// Selected is the node highlighted with Theme.Accent, or nil for no
	// selection.
	Selected *TreeTableNode

	// ScrollRow is the index, into the current visible-flattened node
	// list, of the top row Draw paints. It's clamped on every Draw /
	// OnEvent to [0, max(0, visibleCount-windowRows)], so it's always
	// safe to set directly; prefer ScrollTo/ScrollBy for arithmetic on
	// it. When the whole tree fits in Bounds().H, ScrollRow==0 paints
	// every row.
	ScrollRow int

	// rows is a transient flat list of (node, depth) pairs computed on
	// every Draw + OnEvent so hit-tests + paint share one definition of
	// "visible", exactly like TreeView.rows.
	rows []treeTableRow

	// sbDrag tracks an in-progress drag of the vertical scrollbar thumb.
	sbDrag scrollDrag
}

type treeTableRow struct {
	node  *TreeTableNode
	depth int
}

// TreeTableHeaderHeight is the pixel height of the header row.
const TreeTableHeaderHeight = 24

// TreeTableRowHeight is the pixel height of one body row.
const TreeTableRowHeight = 22

// NewTreeTable builds a TreeTable with the given columns + forest of root
// nodes.
func NewTreeTable(cols []TreeTableColumn, root []*TreeTableNode) *TreeTable {
	return &TreeTable{Columns: cols, Root: root}
}

// flatten populates rows by walking every tree in Root in depth-first
// order + skipping the children of any collapsed node — the forest
// analogue of TreeView.flatten.
func (t *TreeTable) flatten() {
	t.rows = t.rows[:0]
	for _, n := range t.Root {
		t.walk(n, 0)
	}
}

// walk recurses into n + its children if Expanded.
func (t *TreeTable) walk(n *TreeTableNode, depth int) {
	t.rows = append(t.rows, treeTableRow{n, depth})
	if !n.Expanded {
		return
	}
	for _, c := range n.Children {
		t.walk(c, depth+1)
	}
}

// bodyVisibleRows returns how many full body rows fit below the header at
// Bounds().H, the tree-table analogue of Table.bodyVisibleRows — except,
// like TreeView.windowRows, it rounds DOWN (no partial row is ever
// painted), so no per-row clip is needed to keep a partial row from
// spilling past Bounds(). 0 means "don't virtualize" (Bounds().H not yet
// set), so a TreeTable used before SetBounds paints every row.
func (t *TreeTable) bodyVisibleRows() int {
	h := t.Bounds().H - TreeTableHeaderHeight
	if h <= 0 {
		return 0
	}
	return h / TreeTableRowHeight
}

// clampScrollRow confines row to [0, max(0, total-window)], the range
// that always leaves the window full of real rows (or, when the tree is
// shorter than the window, pinned at 0) — identical to TreeView's helper.
func (t *TreeTable) clampScrollRow(row, total, window int) int {
	return clampInt(row, 0, max(0, total-window))
}

// ScrollTo sets ScrollRow to row, clamped against the tree's current
// flattened shape + the widget's bounds.
func (t *TreeTable) ScrollTo(row int) {
	t.flatten()
	t.ScrollRow = t.clampScrollRow(row, len(t.rows), t.bodyVisibleRows())
}

// ScrollBy adjusts ScrollRow by delta, with the same clamping as ScrollTo.
// Negative delta scrolls up.
func (t *TreeTable) ScrollBy(delta int) {
	t.ScrollTo(t.ScrollRow + delta)
}

// scrollToSelected nudges ScrollRow by the minimum amount needed to bring
// Selected back inside the visible window. A nil Selected (no selection
// yet) is a deliberate no-op — mirrors TreeView.scrollToSelected exactly,
// including the "Selected isn't currently visible" no-op (idx==-1).
func (t *TreeTable) scrollToSelected() {
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
	wr := t.bodyVisibleRows()
	switch {
	case idx < t.ScrollRow:
		t.ScrollRow = idx
	case wr > 0 && idx >= t.ScrollRow+wr:
		t.ScrollRow = idx - wr + 1
	}
	t.ScrollRow = t.clampScrollRow(t.ScrollRow, len(t.rows), wr)
}

// columnWidths distributes the total pixel budget across every column —
// the TreeTableColumn analogue of Table.columnWidths, same fixed/auto
// split + same "remainder onto the last auto column" rule.
func (t *TreeTable) columnWidths(total int) []int {
	n := len(t.Columns)
	if n == 0 {
		return nil
	}
	widths := make([]int, n)
	fixedTotal := 0
	autoCount := 0
	lastAutoIdx := -1
	for i, col := range t.Columns {
		if col.Width > 0 {
			widths[i] = col.Width
			fixedTotal += col.Width
		} else {
			autoCount++
			lastAutoIdx = i
		}
	}
	if autoCount == 0 {
		return widths
	}
	remaining := total - fixedTotal
	if remaining < 0 {
		remaining = 0
	}
	share := remaining / autoCount
	for i, col := range t.Columns {
		if col.Width <= 0 {
			widths[i] = share
		}
	}
	sum := 0
	for _, w := range widths {
		sum += w
	}
	widths[lastAutoIdx] += total - sum
	if widths[lastAutoIdx] < 0 {
		widths[lastAutoIdx] = 0
	}
	return widths
}

// cellText returns row's text for column j, or "" if the row carries
// fewer cells than there are columns — the same short-row tolerance
// Table.Rows gets.
func cellText(row *TreeTableNode, j int) string {
	if j < len(row.Cells) {
		return row.Cells[j]
	}
	return ""
}

// Draw paints the header, then the rows in the current scroll window:
// flattened nodes [ScrollRow, ScrollRow+bodyVisibleRows()). The first
// column is indented by depth + prefixed with a ▸/▾ disclosure glyph when
// the node has Children (identical shape to TreeView's chevron); the rest
// are plain cells, aligned per column exactly like Table. A right-edge
// scrollbar is painted only when the flattened list overflows the window.
func (t *TreeTable) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	t.flatten()
	total := len(t.rows)
	wr := t.bodyVisibleRows()
	windowed := wr > 0 && total > wr
	t.ScrollRow = t.clampScrollRow(t.ScrollRow, total, wr)

	bodyW := r.W
	if windowed {
		bodyW = r.W - scrollbarWidth
	}
	widths := t.columnWidths(bodyW)

	// --- Header row --------------------------------------------------
	fillRect(p, r.X, r.Y, bodyW, TreeTableHeaderHeight, theme.SurfaceAlt)
	fillRect(p, r.X, r.Y+TreeTableHeaderHeight-1, bodyW, 1, theme.Border)
	hx := r.X
	hty := r.Y + (TreeTableHeaderHeight-t.glyphHeight())/2
	for i, col := range t.Columns {
		t.drawText(p, cellTextX(&t.Base, hx, widths[i], col.Title, col.Align), hty, col.Title, theme.OnBackground)
		hx += widths[i]
	}

	// --- Body ----------------------------------------------------------
	bodyY := r.Y + TreeTableHeaderHeight
	start, end := 0, total
	if windowed {
		start = t.ScrollRow
		end = start + wr
	}

	var clr painter.Clipper
	if windowed {
		if c, ok := p.(painter.Clipper); ok {
			clr = c
			clr.PushClip(Rect{X: r.X, Y: bodyY, W: bodyW, H: r.Y + r.H - bodyY})
		}
	}
	onAccent := accentInk(theme)
	for i := start; i < end; i++ {
		row := t.rows[i]
		y := bodyY + (i-start)*TreeTableRowHeight
		bg := theme.Surface
		ink := theme.OnSurface
		if row.node == t.Selected {
			bg = theme.Accent
			ink = onAccent
		}
		fillRect(p, r.X, y, bodyW, TreeTableRowHeight, bg)
		cx := r.X
		cty := y + (TreeTableRowHeight-t.glyphHeight())/2
		for j, col := range t.Columns {
			cellW := widths[j]
			if j == 0 {
				indent := r.X + row.depth*TreeIndentW
				if len(row.node.Children) > 0 {
					cxg := indent + 4
					cyg := y + TreeTableRowHeight/2
					// ▾ (expanded): flat top narrowing to a point at the
					// bottom. ▸ (collapsed): flat left narrowing to a
					// point on the right. Same 4-row fillRect technique
					// TreeView's chevron uses.
					if row.node.Expanded {
						for q := 0; q < 4; q++ {
							fillRect(p, cxg-q, cyg+2-q, 1+2*q, 1, ink)
						}
					} else {
						for q := 0; q < 4; q++ {
							fillRect(p, cxg+2-q, cyg-q, 1, 1+2*q, ink)
						}
					}
				}
				t.drawText(p, indent+TreeChevronW, cty, cellText(row.node, 0), ink)
			} else {
				text := cellText(row.node, j)
				t.drawText(p, cellTextX(&t.Base, cx, cellW, text, col.Align), cty, text, ink)
			}
			cx += cellW
		}
	}
	if clr != nil {
		clr.PopClip()
	}

	// --- Column separators ---------------------------------------------
	sepX := r.X
	for i := 0; i < len(t.Columns)-1; i++ {
		sepX += widths[i]
		fillRect(p, sepX, r.Y, 1, r.H, theme.Border)
	}

	// --- Vertical scrollbar (right edge, body only) ---------------------
	if windowed {
		t.drawScrollbar(p, theme, r)
	}
	t.drawFocusRing(p, theme, r)
}

// scrollbarGeom returns the vertical scrollbar's widget-local geometry and
// whether it is live (the flattened list overflows the window). The header sits
// above the track and never scrolls, so the track spans only [TreeTableHeaderHeight,
// r.H). It is the single definition of the track + thumb shared by drawScrollbar
// and OnEvent, using the same row-count proportion TreeView uses.
func (t *TreeTable) scrollbarGeom() (sbGeom, bool) {
	r := t.Bounds()
	t.flatten()
	total := len(t.rows)
	wr := t.bodyVisibleRows()
	if wr <= 0 || total <= wr {
		return sbGeom{}, false
	}
	trackTop := TreeTableHeaderHeight
	trackH := r.H - TreeTableHeaderHeight
	thumbH := trackH * wr / total
	if thumbH < 8 {
		thumbH = 8
	}
	maxScroll := total - wr // > 0 here (total > wr)
	scroll := t.clampScrollRow(t.ScrollRow, total, wr)
	return sbGeom{
		cross0:     r.W - scrollbarWidth,
		trackStart: trackTop,
		trackLen:   trackH,
		thumbStart: trackTop + scroll*(trackH-thumbH)/maxScroll,
		thumbLen:   thumbH,
		travelNum:  trackH - thumbH,
		travelDen:  maxScroll,
		maxScroll:  maxScroll,
	}, true
}

// drawScrollbar paints the right-edge track + thumb over the body rows from
// scrollbarGeom. Only called by Draw while windowed is true.
func (t *TreeTable) drawScrollbar(p painter.Painter, theme *Theme, r Rect) {
	// The caller only invokes this while windowed, which is exactly when
	// scrollbarGeom reports live, so the ok flag is always true here.
	g, _ := t.scrollbarGeom()
	fillRect(p, r.X+g.cross0, r.Y+g.trackStart, scrollbarWidth, g.trackLen, theme.SurfaceAlt)
	fillRect(p, r.X+g.cross0, r.Y+g.thumbStart, scrollbarWidth, g.thumbLen, theme.Accent)
}

// OnEvent: a click on the first column's disclosure glyph toggles that
// node's Expanded (re-clamping ScrollRow, since toggling can shrink or
// grow the visible row count out from under it); a click anywhere else on
// NodeAt returns the TreeTableNode at widget-local (x, y) in the current
// visible-flattened, scrolled body, or nil for the header band or empty space
// below the last row. It does not mutate ScrollRow (unlike OnEvent). Exposed so
// a host can hit-test a right-click and build a context menu for that node.
func (t *TreeTable) NodeAt(x, y int) *TreeTableNode {
	if y < TreeTableHeaderHeight {
		return nil
	}
	t.flatten()
	total := len(t.rows)
	wr := t.bodyVisibleRows()
	localIdx := (y - TreeTableHeaderHeight) / TreeTableRowHeight
	if wr > 0 && total > wr && localIdx >= wr {
		return nil
	}
	idx := localIdx + t.clampScrollRow(t.ScrollRow, total, wr)
	if idx < 0 || idx >= total {
		return nil
	}
	return t.rows[idx].node
}

// Remove detaches node n from the forest — from a top-level Root slot or from
// its parent's Children. It returns true when n was found and removed; false
// for a nil node or one not in the tree. Exposed so a host can implement a
// "delete node" menu action (TreeTableNode has no parent pointer).
func (t *TreeTable) Remove(n *TreeTableNode) bool {
	if n == nil {
		return false
	}
	for i, c := range t.Root {
		if c == n {
			t.Root = append(t.Root[:i], t.Root[i+1:]...)
			return true
		}
		if removeTreeTableChild(c, n) {
			return true
		}
	}
	return false
}

// removeTreeTableChild removes target from parent's subtree (depth-first),
// returning whether it was found.
func removeTreeTableChild(parent, target *TreeTableNode) bool {
	for i, c := range parent.Children {
		if c == target {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return true
		}
		if removeTreeTableChild(c, target) {
			return true
		}
	}
	return false
}

// a row selects the node. Y is mapped through ScrollRow back to the
// flattened index it targets, exactly like TreeView.OnEvent.
func (t *TreeTable) OnEvent(ev Event) {
	switch ev.Kind {
	case EventScroll:
		// Native wheel scroll: shift the visible row window by Delta rows
		// (ScrollBy clamps at top + bottom).
		t.ScrollBy(ev.Delta)
		return
	case EventKeyDown:
		// Arrow / Page / Home / End scroll the tree by whole rows or pages;
		// any other key is ignored.
		handleScrollKey(t, ev.Code, t.bodyVisibleRows())
		return
	case EventClick:
		// A press on the scrollbar grabs its thumb (or pages the track); it
		// must not also select a row, so consume it here.
		if g, ok := t.scrollbarGeom(); t.sbDrag.press(g, ok, ev, t.bodyVisibleRows(), t.ScrollBy) {
			return
		}
		// fall through to the click handling below.
	case EventMouseDrag:
		g, ok := t.scrollbarGeom()
		t.sbDrag.drag(g, ok, ev, t.ScrollTo)
		return
	case EventMouseUp:
		t.sbDrag.release()
		return
	default:
		return
	}
	t.flatten()
	total := len(t.rows)
	wr := t.bodyVisibleRows()
	windowed := wr > 0 && total > wr
	t.ScrollRow = t.clampScrollRow(t.ScrollRow, total, wr)

	if ev.Y < TreeTableHeaderHeight {
		// Header row: no sort/resize behaviour on a TreeTable — a click
		// there is simply a no-op.
		return
	}
	localIdx := (ev.Y - TreeTableHeaderHeight) / TreeTableRowHeight
	if windowed && localIdx >= wr {
		// Below the last painted row (only possible when the body height
		// isn't an exact multiple of TreeTableRowHeight): nothing was
		// drawn there.
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
		t.flatten()
		t.ScrollRow = t.clampScrollRow(t.ScrollRow, len(t.rows), t.bodyVisibleRows())
		return
	}
	t.Selected = row.node
}
