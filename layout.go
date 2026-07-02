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

// --- HBox ----------------------------------------------------------------

// HBox is a horizontal flow container. Children are laid out left-to-
// right + share the container's width equally (minus Spacing gaps
// between adjacent children). Children's Y + height fill the box's
// vertical extent.
//
// HBox is a Widget itself: Draw fans out to every child + OnEvent
// hit-tests by child Bounds, translating coordinates into the
// matched child's local space before forwarding.
type HBox struct {
	Base
	// Spacing is the gap in pixels between adjacent children. Defaults
	// to 4 when left at zero (set to a negative value to truly disable
	// the gap; negative values are clamped to zero at layout time).
	Spacing  int
	children []Widget
}

// NewHBox constructs an empty HBox. Callers add children via Append
// + then call SetBounds to trigger layout.
func NewHBox() *HBox { return &HBox{} }

// Append adds w to the right of any existing children. Re-runs layout
// so the new child is positioned immediately + the caller doesn't
// have to remember to re-call SetBounds.
func (h *HBox) Append(w Widget) {
	h.children = append(h.children, w)
	h.SetBounds(h.Bounds())
}

// SetBounds positions the HBox + lays out its children. Width is
// divided equally among children; Spacing pixels separate adjacent
// cells. Children's Y matches the box's Y; height matches H.
func (h *HBox) SetBounds(r Rect) {
	h.Base.SetBounds(r)
	n := len(h.children)
	if n == 0 {
		return
	}
	spacing := h.Spacing
	if spacing == 0 {
		spacing = defaultSpacing
	}
	if spacing < 0 {
		spacing = 0
	}
	totalGap := spacing * (n - 1)
	cellW := (r.W - totalGap) / n
	for i, c := range h.children {
		x := r.X + i*(cellW+spacing)
		c.SetBounds(Rect{X: x, Y: r.Y, W: cellW, H: r.H})
	}
}

// Draw paints every child in append order. Children render directly
// into the surface using their own Bounds; the HBox itself draws no
// background or border (it's a pure layout container).
func (h *HBox) Draw(p painter.Painter, theme *Theme) {
	for _, c := range h.children {
		c.Draw(p, theme)
	}
}

// OnEvent hit-tests by child Bounds + forwards the event with
// coordinates translated into the child's local space. The first
// child whose Bounds contains the event point wins (children should
// not overlap inside an HBox, but a stable order is still useful).
func (h *HBox) OnEvent(ev Event) {
	pr := h.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, c := range h.children {
		if c.Bounds().Contains(sx, sy) {
			c.OnEvent(translateEvent(ev, pr, c.Bounds()))
			return
		}
	}
}

// --- VBox ----------------------------------------------------------------

// VBox is the vertical analogue of HBox: children stack top-to-bottom,
// sharing the container's height + filling its width.
type VBox struct {
	Base
	// Spacing is the gap in pixels between adjacent children; same
	// semantics as HBox.Spacing.
	Spacing  int
	children []Widget
}

// NewVBox constructs an empty VBox.
func NewVBox() *VBox { return &VBox{} }

// Append adds w below any existing children + re-runs layout.
func (v *VBox) Append(w Widget) {
	v.children = append(v.children, w)
	v.SetBounds(v.Bounds())
}

// SetBounds positions the VBox + stacks its children vertically.
func (v *VBox) SetBounds(r Rect) {
	v.Base.SetBounds(r)
	n := len(v.children)
	if n == 0 {
		return
	}
	spacing := v.Spacing
	if spacing == 0 {
		spacing = defaultSpacing
	}
	if spacing < 0 {
		spacing = 0
	}
	totalGap := spacing * (n - 1)
	cellH := (r.H - totalGap) / n
	for i, c := range v.children {
		y := r.Y + i*(cellH+spacing)
		c.SetBounds(Rect{X: r.X, Y: y, W: r.W, H: cellH})
	}
}

// Draw paints every child in append order.
func (v *VBox) Draw(p painter.Painter, theme *Theme) {
	for _, c := range v.children {
		c.Draw(p, theme)
	}
}

// OnEvent hit-tests + forwards just like HBox.
func (v *VBox) OnEvent(ev Event) {
	pr := v.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, c := range v.children {
		if c.Bounds().Contains(sx, sy) {
			c.OnEvent(translateEvent(ev, pr, c.Bounds()))
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
