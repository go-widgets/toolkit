// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// defaultSpacing is the inter-child gap (in pixels) HBox + VBox apply
// when their Spacing field is left at its zero value. Picked to match
// the 4-pixel rhythm the rest of the toolkit uses (Frame.Padding, the
// Button border inset, ...). Containers expose Spacing as a public
// field so apps that want a tighter or looser layout can override it
// before the first SetBounds call.
const defaultSpacing = 4

// defaultPadding is Frame's interior inset between its border and the
// child widget. Same 4-pixel rationale as defaultSpacing.
const defaultPadding = 4

// translateEvent rewrites a parent-local event into the child's
// widget-local coordinate space. parentRect is the container's
// Bounds() (in surface coords); childRect is the child's Bounds()
// (also surface coords). The container's OnEvent input is in parent-
// local coords per the package convention, so the surface position
// of the event is (ev.X+parentRect.X, ev.Y+parentRect.Y); subtracting
// childRect.X/Y yields child-local coords.
func translateEvent(ev Event, parentRect, childRect Rect) Event {
	out := ev
	out.X = ev.X + parentRect.X - childRect.X
	out.Y = ev.Y + parentRect.Y - childRect.Y
	return out
}

// --- box sizing ----------------------------------------------------------

// boxChild pairs a widget with its main-axis sizing spec: a flex weight (a
// proportional share of the space left after fixed children + gaps) or a fixed
// pixel size. Append gives flex 1 (all-equal, the historical behaviour);
// AddFlex sets an explicit weight; AddFixed a fixed size — the Sencha
// vbox/hbox model (flex + fixed items).
type boxChild struct {
	w    Widget
	flex int // >0 => proportional; 0 => fixed
	size int // used when flex == 0
}

// boxSpacing normalises a container's Spacing (0 → default, negative → 0).
func boxSpacing(s int) int {
	if s == 0 {
		return defaultSpacing
	}
	if s < 0 {
		return 0
	}
	return s
}

// boxCells returns each child's main-axis size for a container of the given
// total extent and spacing: fixed children keep their size, the remainder is
// split among flex children by weight (integer division, no remainder
// redistribution — so an all-flex-1 box matches the historical equal split
// exactly).
func boxCells(total, spacing int, children []boxChild) []int {
	n := len(children)
	sizes := make([]int, n)
	fixed, flexTotal := 0, 0
	for _, c := range children {
		if c.flex > 0 {
			flexTotal += c.flex
		} else {
			fixed += c.size
		}
	}
	avail := total - spacing*(n-1) - fixed
	if avail < 0 {
		avail = 0
	}
	for i, c := range children {
		if c.flex > 0 && flexTotal > 0 {
			sizes[i] = avail * c.flex / flexTotal
		} else {
			sizes[i] = c.size
		}
	}
	return sizes
}

// --- HBox ----------------------------------------------------------------

// HBox is a horizontal flow container. Children are laid out left-to-right;
// each takes a flex share of the width or a fixed width (see boxChild), with
// Spacing gaps between them. Children's Y + height fill the box's vertical
// extent.
//
// HBox is a Widget itself: Draw fans out to every child + OnEvent hit-tests by
// child Bounds, translating coordinates into the matched child's local space.
type HBox struct {
	Base
	// Spacing is the gap in pixels between adjacent children. Defaults to 4 when
	// left at zero (negative values are clamped to zero at layout time).
	Spacing  int
	children []boxChild
}

// NewHBox constructs an empty HBox. Add children via Append/AddFlex/AddFixed.
func NewHBox() *HBox { return &HBox{} }

// Append adds w with flex weight 1 (an equal share of the width).
func (h *HBox) Append(w Widget) { h.add(w, 1, 0) }

// AddFlex adds w with an explicit flex weight (clamped to ≥1).
func (h *HBox) AddFlex(w Widget, flex int) {
	if flex < 1 {
		flex = 1
	}
	h.add(w, flex, 0)
}

// AddFixed adds w with a fixed width in pixels (clamped to ≥0).
func (h *HBox) AddFixed(w Widget, size int) {
	if size < 0 {
		size = 0
	}
	h.add(w, 0, size)
}

func (h *HBox) add(w Widget, flex, size int) {
	h.children = append(h.children, boxChild{w: w, flex: flex, size: size})
	h.SetBounds(h.Bounds())
}

// SetBounds positions the HBox + lays out its children across the width.
func (h *HBox) SetBounds(r Rect) {
	h.Base.SetBounds(r)
	if len(h.children) == 0 {
		return
	}
	sp := boxSpacing(h.Spacing)
	sizes := boxCells(r.W, sp, h.children)
	x := r.X
	for i, c := range h.children {
		c.w.SetBounds(Rect{X: x, Y: r.Y, W: sizes[i], H: r.H})
		x += sizes[i] + sp
	}
}

// Draw paints every child in append order (the box itself draws nothing).
func (h *HBox) Draw(p painter.Painter, theme *Theme) {
	for _, c := range h.children {
		c.w.Draw(p, theme)
	}
}

// OnEvent forwards to the first child whose Bounds contains the event point,
// translated into that child's local space.
func (h *HBox) OnEvent(ev Event) {
	pr := h.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, c := range h.children {
		if c.w.Bounds().Contains(sx, sy) {
			c.w.OnEvent(translateEvent(ev, pr, c.w.Bounds()))
			return
		}
	}
}

// --- VBox ----------------------------------------------------------------

// VBox is the vertical analogue of HBox: children stack top-to-bottom, each a
// flex share of the height or a fixed height, filling the box's width.
type VBox struct {
	Base
	// Spacing is the gap in pixels between adjacent children; same semantics as
	// HBox.Spacing.
	Spacing  int
	children []boxChild
}

// NewVBox constructs an empty VBox.
func NewVBox() *VBox { return &VBox{} }

// Append adds w with flex weight 1 (an equal share of the height).
func (v *VBox) Append(w Widget) { v.add(w, 1, 0) }

// AddFlex adds w with an explicit flex weight (clamped to ≥1).
func (v *VBox) AddFlex(w Widget, flex int) {
	if flex < 1 {
		flex = 1
	}
	v.add(w, flex, 0)
}

// AddFixed adds w with a fixed height in pixels (clamped to ≥0).
func (v *VBox) AddFixed(w Widget, size int) {
	if size < 0 {
		size = 0
	}
	v.add(w, 0, size)
}

func (v *VBox) add(w Widget, flex, size int) {
	v.children = append(v.children, boxChild{w: w, flex: flex, size: size})
	v.SetBounds(v.Bounds())
}

// SetBounds positions the VBox + stacks its children down the height.
func (v *VBox) SetBounds(r Rect) {
	v.Base.SetBounds(r)
	if len(v.children) == 0 {
		return
	}
	sp := boxSpacing(v.Spacing)
	sizes := boxCells(r.H, sp, v.children)
	y := r.Y
	for i, c := range v.children {
		c.w.SetBounds(Rect{X: r.X, Y: y, W: r.W, H: sizes[i]})
		y += sizes[i] + sp
	}
}

// Draw paints every child in append order.
func (v *VBox) Draw(p painter.Painter, theme *Theme) {
	for _, c := range v.children {
		c.w.Draw(p, theme)
	}
}

// OnEvent forwards to the first child containing the event point.
func (v *VBox) OnEvent(ev Event) {
	pr := v.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, c := range v.children {
		if c.w.Bounds().Contains(sx, sy) {
			c.w.OnEvent(translateEvent(ev, pr, c.w.Bounds()))
			return
		}
	}
}

// --- Grid ----------------------------------------------------------------

// gridChild pairs a widget with its (col, row) placement so Grid can
// re-position it whenever SetBounds runs.
type gridChild struct {
	w        Widget
	col, row int
}

// Grid lays children out in a fixed cols x rows table. Each cell is
// the same size (container W/cols, H/rows). Children are placed via
// Attach(child, col, row); a cell with no attached child stays empty.
//
// Grid is a Widget: Draw fans out to every attached child + OnEvent
// hit-tests then forwards.
type Grid struct {
	Base
	cols, rows int
	children   []gridChild
}

// NewGrid constructs an empty cols x rows grid. cols + rows must be
// positive; the constructor clamps non-positive inputs to 1 to keep
// the divide-by-zero out of SetBounds.
func NewGrid(cols, rows int) *Grid {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	return &Grid{cols: cols, rows: rows}
}

// Attach places w at (col, row). Out-of-range coordinates are clamped
// into the grid so a typo doesn't silently vanish + the child still
// ends up somewhere visible. Re-runs layout immediately.
func (g *Grid) Attach(w Widget, col, row int) {
	if col < 0 {
		col = 0
	}
	if col >= g.cols {
		col = g.cols - 1
	}
	if row < 0 {
		row = 0
	}
	if row >= g.rows {
		row = g.rows - 1
	}
	g.children = append(g.children, gridChild{w: w, col: col, row: row})
	g.SetBounds(g.Bounds())
}

// SetBounds positions the Grid + sizes every attached child to its
// (col, row) cell.
func (g *Grid) SetBounds(r Rect) {
	g.Base.SetBounds(r)
	if len(g.children) == 0 {
		return
	}
	cellW := r.W / g.cols
	cellH := r.H / g.rows
	for _, c := range g.children {
		c.w.SetBounds(Rect{
			X: r.X + c.col*cellW,
			Y: r.Y + c.row*cellH,
			W: cellW,
			H: cellH,
		})
	}
}

// Draw paints every attached child in attach order.
func (g *Grid) Draw(p painter.Painter, theme *Theme) {
	for _, c := range g.children {
		c.w.Draw(p, theme)
	}
}

// OnEvent hit-tests attached children + forwards with translated
// coordinates.
func (g *Grid) OnEvent(ev Event) {
	pr := g.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, c := range g.children {
		if c.w.Bounds().Contains(sx, sy) {
			c.w.OnEvent(translateEvent(ev, pr, c.w.Bounds()))
			return
		}
	}
}

// --- Frame ---------------------------------------------------------------

// Frame draws a 1-pixel border around a single child widget + inset
// the child by Padding pixels inside that border. Useful as a group-
// box / panel separator when an app wants to visually fence off a
// region of widgets.
//
// Frame is a Widget: Draw paints the border + delegates to the child;
// OnEvent forwards to the child with translated coordinates.
type Frame struct {
	Base
	// Padding is the inset (in pixels) between Frame's border + its
	// child. Defaults to 4 when left at zero; negative values are
	// clamped to zero at layout time.
	Padding int
	child   Widget
}

// NewFrame wraps child in a Frame. child may be nil (the Frame then
// just draws its border + accepts no events).
func NewFrame(child Widget) *Frame { return &Frame{child: child} }

// SetBounds positions the Frame + resizes its child to fit inside the
// border + padding.
func (f *Frame) SetBounds(r Rect) {
	f.Base.SetBounds(r)
	if f.child == nil {
		return
	}
	pad := f.Padding
	if pad == 0 {
		pad = defaultPadding
	}
	if pad < 0 {
		pad = 0
	}
	// 1px border on each side plus pad on each side.
	inset := 1 + pad
	f.child.SetBounds(Rect{
		X: r.X + inset,
		Y: r.Y + inset,
		W: r.W - 2*inset,
		H: r.H - 2*inset,
	})
}

// Draw paints the 1-pixel border then the child (if any).
func (f *Frame) Draw(p painter.Painter, theme *Theme) {
	r := f.Bounds()
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	if f.child != nil {
		f.child.Draw(p, theme)
	}
}

// OnEvent forwards to the child if the event lands inside its Bounds.
func (f *Frame) OnEvent(ev Event) {
	if f.child == nil {
		return
	}
	pr := f.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	if f.child.Bounds().Contains(sx, sy) {
		f.child.OnEvent(translateEvent(ev, pr, f.child.Bounds()))
	}
}
