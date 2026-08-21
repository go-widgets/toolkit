// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ColumnBrowser is a Miller-column ("columns") view of a tree: N side-by-side
// columns, each listing the children of a node, where picking a container opens
// the next column to its right and picking a leaf opens a compact preview column.
// The strip scrolls horizontally to keep the deepest columns visible. Each row
// carries a leading type icon and, for a container, a disclosure chevron.
//
// It is driven entirely by a caller-supplied ColumnProvider, so it navigates any
// tree (a filesystem, a settings hierarchy, an object graph) without the widget
// knowing anything about the domain. Internally each column is a toolkit ListBox
// (composed over its public API — the ColumnBrowser never modifies ListBox), so a
// column inherits vertical scrolling, keyboard roving and selection for free.
//
// Layout: a body filled with Theme.Surface; columns laid out left to right at
// ColumnWidth, anchored so the newest column stays in view, with a hairline
// Theme.Border between them; an optional preview pane (Theme.SurfaceAlt) after the
// last column showing a leaf's big icon, name and provider-supplied detail lines.
// Everything is clipped to the widget bounds.
type ColumnBrowser struct {
	Base

	// ColumnWidth is the pixel width of each directory column and the preview
	// pane. Set before SetRoot / SetBounds; defaults via NewColumnBrowser.
	ColumnWidth int

	// OnActivate fires when an already-selected leaf is picked again, with its
	// node — the "open this file" gesture. Nil-guarded.
	OnActivate func(node ColumnNode)

	provider ColumnProvider
	cols     []*browserColumn
	scrollX  int
	preview  *columnPreview
}

// ColumnNode is one entry a ColumnProvider lists for a container. Container marks
// a node that opens a further column when picked (a folder); a non-container is a
// leaf that opens the preview pane. Icon is the optional leading type icon, Name
// the displayed label, and Key the opaque identity the provider uses to list the
// node's children and describe it.
type ColumnNode struct {
	Name      string
	Key       string
	Icon      *Image
	Container bool
}

// ColumnProvider supplies the tree a ColumnBrowser navigates. Children returns
// the entries under the container identified by key (SetRoot's key for the first
// column); ok=false rejects the key — a permission error, a leaf mistaken for a
// container, an empty listing the caller wants to suppress — and no column opens.
// Preview returns the detail lines (kind, size, ...) shown under a picked leaf's
// name in the preview pane, or nil for none.
type ColumnProvider interface {
	Children(key string) (nodes []ColumnNode, ok bool)
	Preview(node ColumnNode) []string
}

// browserColumn is one open column: the provider key it lists, its nodes, the
// ListBox that renders them, and the picked row (drives the next column).
type browserColumn struct {
	key      string
	nodes    []ColumnNode
	list     *ListBox
	selected int
}

// columnPreview is the leaf shown in the preview pane plus its detail lines.
type columnPreview struct {
	node  ColumnNode
	lines []string
}

// ColumnBrowser metrics (pixels).
const (
	cbColumnWidth = 220 // default column / preview width
	cbRowHeight   = 26
	cbRowPad      = 8
	cbRowIconPx   = 16
	cbChevronRoom = 14 // width reserved at a container row's right edge
	cbPreviewIcon = 96
)

// ColumnBrowser describes itself for accessibility.
var _ Accessible = (*ColumnBrowser)(nil)

// A11y reports the ColumnBrowser as a tree. Value names the deepest picked node
// (the leaf/folder at the end of the open chain), or is empty when nothing has
// been picked yet.
func (cv *ColumnBrowser) A11y() A11yInfo {
	v := ""
	for _, col := range cv.cols {
		if col.selected >= 0 && col.selected < len(col.nodes) {
			v = col.nodes[col.selected].Name
		}
	}
	return A11yInfo{Role: RoleTree, Value: v}
}

// NewColumnBrowser builds a ColumnBrowser over provider with a default column
// width. Call SetRoot to list the first column, then SetBounds to lay it out.
func NewColumnBrowser(provider ColumnProvider) *ColumnBrowser {
	return &ColumnBrowser{
		ColumnWidth: cbColumnWidth,
		provider:    provider,
	}
}

// ColumnCount is the number of open directory columns (excluding the preview).
func (cv *ColumnBrowser) ColumnCount() int { return len(cv.cols) }

// SetRoot resets the strip to a single column listing rootKey (or to an empty
// strip when the provider rejects it).
func (cv *ColumnBrowser) SetRoot(rootKey string) {
	cv.cols = nil
	cv.preview = nil
	cv.scrollX = 0
	if col := cv.makeColumn(rootKey); col != nil {
		cv.cols = []*browserColumn{col}
	}
	cv.relayout()
}

// makeColumn lists key via the provider and builds its column, or returns nil
// when the provider rejects the key.
func (cv *ColumnBrowser) makeColumn(key string) *browserColumn {
	nodes, ok := cv.provider.Children(key)
	if !ok {
		return nil
	}
	col := &browserColumn{key: key, nodes: nodes, selected: -1}
	lb := NewListBox(nil)
	lb.RowHeight = cbRowHeight
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}
	lb.Items = names
	ci := len(cv.cols) // position this column will occupy in the strip
	lb.ItemRenderer = func(p painter.Painter, theme *Theme, rc Rect, index int, item string, selected bool, ink RGBA) {
		cv.drawColRow(p, theme, rc, col, index, ink)
	}
	lb.OnActivate = func(row int) { cv.onPick(ci, row) }
	col.list = lb
	return col
}

// onPick handles a row activation in column ci: a container truncates the strip
// after ci and opens the picked node in a fresh column to the right; a leaf drops
// any deeper columns, shows the preview, and (on a re-pick of the same leaf)
// fires OnActivate.
func (cv *ColumnBrowser) onPick(ci, row int) {
	if ci < 0 || ci >= len(cv.cols) {
		return
	}
	col := cv.cols[ci]
	if row < 0 || row >= len(col.nodes) {
		return
	}
	node := col.nodes[row]
	repick := col.selected == row
	col.selected = row
	col.list.Selected().Set(row)
	cv.cols = cv.cols[:ci+1]

	if node.Container {
		cv.preview = nil
		if next := cv.makeColumn(node.Key); next != nil {
			cv.cols = append(cv.cols, next)
		}
	} else {
		cv.preview = &columnPreview{node: node, lines: cv.provider.Preview(node)}
		if repick && cv.OnActivate != nil {
			cv.OnActivate(node)
		}
	}
	cv.relayout()
}

// drawColRow renders one column row: the type icon, the name (elided to the
// available width, leaving room for a chevron on a container), and a disclosure
// chevron on the right for a container.
func (cv *ColumnBrowser) drawColRow(p painter.Painter, theme *Theme, rc Rect, col *browserColumn, index int, ink RGBA) {
	node := col.nodes[index]
	if node.Icon != nil {
		iy := rc.Y + (rc.H-cbRowIconPx)/2
		node.Icon.SetBounds(Rect{X: rc.X + cbRowPad, Y: iy, W: cbRowIconPx, H: cbRowIconPx})
		node.Icon.Draw(p, theme)
	}
	tx := rc.X + cbRowPad + cbRowIconPx + cbRowPad
	ty := rc.Y + (rc.H-cv.glyphHeight())/2
	avail := rc.X + rc.W - cbRowPad - tx
	if node.Container {
		avail -= cbChevronRoom
	}
	name := node.Name
	if cv.textWidth(name) > avail {
		name = ellipsize(cv.EffectiveFont(), name, avail)
	}
	cv.drawText(p, tx, ty, name, ink)
	if node.Container {
		drawDisclosureChevron(p, rc.X+rc.W-cbRowPad-4, rc.Y+rc.H/2, false, ink)
	}
}

// SetBounds records bounds and lays out the columns.
func (cv *ColumnBrowser) SetBounds(r Rect) {
	cv.Base.SetBounds(r)
	cv.relayout()
}

// contentWidth is the total pixel width of every column plus the preview.
func (cv *ColumnBrowser) contentWidth() int {
	w := len(cv.cols) * cv.ColumnWidth
	if cv.preview != nil {
		w += cv.ColumnWidth
	}
	return w
}

// relayout positions each column left to right, anchoring the strip to the right
// so the deepest columns stay in view.
func (cv *ColumnBrowser) relayout() {
	b := cv.Bounds()
	if b.W <= 0 {
		return
	}
	cv.scrollX = cv.contentWidth() - b.W
	if cv.scrollX < 0 {
		cv.scrollX = 0
	}
	x := b.X - cv.scrollX
	for _, col := range cv.cols {
		col.list.SetBounds(Rect{X: x, Y: b.Y, W: cv.ColumnWidth, H: b.H})
		x += cv.ColumnWidth
	}
}

// Draw paints the columns, their separators and the preview pane, clipped to the
// widget bounds so the horizontally-scrolled strip stays within its region.
func (cv *ColumnBrowser) Draw(p painter.Painter, theme *Theme) {
	b := cv.Bounds()
	fillRect(p, b.X, b.Y, b.W, b.H, theme.Surface)
	withClip(p, b, func() {
		x := b.X - cv.scrollX
		for _, col := range cv.cols {
			col.list.Draw(p, theme)
			fillRect(p, x+cv.ColumnWidth-1, b.Y, 1, b.H, theme.Border)
			x += cv.ColumnWidth
		}
		if cv.preview != nil {
			cv.drawPreview(p, theme, Rect{X: x, Y: b.Y, W: cv.ColumnWidth, H: b.H})
		}
	})
}

// drawPreview paints the compact leaf-info pane: a big icon, the name, and every
// provider-supplied detail line, each centred and elided to the pane width.
func (cv *ColumnBrowser) drawPreview(p painter.Painter, theme *Theme, r Rect) {
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	node := cv.preview.node
	if node.Icon != nil {
		node.Icon.SetBounds(Rect{X: r.X + (r.W-cbPreviewIcon)/2, Y: r.Y + 30, W: cbPreviewIcon, H: cbPreviewIcon})
		node.Icon.Draw(p, theme)
	}
	y := r.Y + 30 + cbPreviewIcon + 16
	cv.drawCentered(p, r, y, node.Name, theme.OnSurface)
	y += 24
	for _, line := range cv.preview.lines {
		cv.drawCentered(p, r, y, line, mutedInk(theme))
		y += 20
	}
}

// drawCentered draws s horizontally centred in r at baseline y, eliding it to the
// pane width.
func (cv *ColumnBrowser) drawCentered(p painter.Painter, r Rect, y int, s string, ink RGBA) {
	if cv.textWidth(s) > r.W-20 {
		s = ellipsize(cv.EffectiveFont(), s, r.W-20)
	}
	lx := r.X + (r.W-cv.textWidth(s))/2
	cv.drawText(p, lx, y, s, ink)
}

// OnEvent routes a click/scroll to the column under the pointer, translating the
// widget-local pointer X into that column's own local space; inert while Disabled.
func (cv *ColumnBrowser) OnEvent(ev Event) {
	if cv.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventClick, EventScroll:
		if ci, ok := cv.columnAt(ev.X); ok {
			local := ev
			local.X = ev.X - (ci*cv.ColumnWidth - cv.scrollX)
			cv.cols[ci].list.OnEvent(local)
		}
	}
}

// columnAt returns the index of the column whose horizontal band contains the
// widget-local x, and ok=false when x falls past every column (e.g. on the
// preview pane or empty space).
func (cv *ColumnBrowser) columnAt(x int) (int, bool) {
	for i := range cv.cols {
		left := i*cv.ColumnWidth - cv.scrollX
		if x >= left && x < left+cv.ColumnWidth {
			return i, true
		}
	}
	return -1, false
}
