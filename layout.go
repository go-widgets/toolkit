// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// DefaultBoxSpacing is the inter-child gap (in pixels) the box constructors
// NewHBox/NewVBox/NewBoxLayout seed into their Spacing field. Picked to match the
// 4-pixel rhythm the rest of the toolkit uses (Frame.Padding, the Button border
// inset, ...). Because the default lives in the constructor rather than the layout
// math, Spacing is honoured LITERALLY: a caller who wants a flush, zero-gap box
// sets Spacing = 0 explicitly; only negative values are clamped (to 0). Containers
// expose Spacing as a public field so apps can override it before the first
// SetBounds call.
const DefaultBoxSpacing = 4

// defaultPadding is Frame's interior inset between its border and the
// child widget. Same 4-pixel rationale as DefaultBoxSpacing.
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
// AddFlex sets an explicit weight; AddFixed a fixed size — a flex + fixed
// box model.
type boxChild struct {
	w    Widget
	flex int // >0 => proportional; 0 => fixed
	size int // used when flex == 0
}

// clampSpacing takes a box's Spacing literally, clamping only negative values to
// zero. The default gap lives in the constructors (NewHBox/NewVBox/NewBoxLayout,
// which seed DefaultBoxSpacing), so a literal Spacing = 0 means a flush, zero-gap
// box; a Spacing = -1 (the pre-DefaultBoxSpacing idiom for "no gap") still yields
// 0, so old call sites keep working.
func clampSpacing(s int) int {
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

// --- box alignment (cross-axis align + main-axis pack) -----------------------

// BoxAlign controls how HBox/VBox position each child on the CROSS axis (the axis
// perpendicular to the flow: vertical for HBox, horizontal for VBox). The zero
// value BoxStretch fills the cross axis — the historical behaviour — so existing
// layouts are unchanged. The others place the child at its natural cross size
// (reported via the optional Measurer, else its current cross Bounds) against the
// start, centre, or end of the box. The box `align` model:
// stretch | start | center | end.
type BoxAlign int

const (
	BoxStretch     BoxAlign = iota // fill the cross axis (default)
	BoxAlignStart                  // pin to the top (HBox) / left (VBox)
	BoxAlignCenter                 // centre on the cross axis
	BoxAlignEnd                    // pin to the bottom (HBox) / right (VBox)
)

// BoxPack controls how HBox/VBox distribute SLACK on the MAIN axis (the flow axis)
// when the children do not fill it — i.e. when no flex child absorbs the space.
// The zero value PackStart leaves the slack after the last child (historical).
// PackCenter splits it either side; PackEnd puts it all before the first child.
// The box `pack` model (start|center|end). With any flex child the slack is
// zero, so Pack has no visible effect then.
type BoxPack int

const (
	PackStart  BoxPack = iota // flush to the start (default)
	PackCenter                // centre the group in the box
	PackEnd                   // flush to the end
)

// Measurer is an optional interface a widget may implement to advertise its
// natural size — an optional natural-size query. Box layouts
// consult it for cross-axis alignment (Align != BoxStretch); widgets that do not
// implement it fall back to their current cross Bounds, and failing that stretch
// to fill. availW/availH is the space the box can offer the child on each axis.
type Measurer interface {
	Measure(availW, availH int) (w, h int)
}

// boxSlack is the leftover main-axis space: the total minus the children's cell
// sizes and the inter-child gaps, floored at 0 (a flex child consumes the
// remainder, so this is ~0 whenever one is present).
func boxSlack(total, spacing int, sizes []int) int {
	used := spacing * (len(sizes) - 1)
	for _, s := range sizes {
		used += s
	}
	if slack := total - used; slack > 0 {
		return slack
	}
	return 0
}

// boxLead is the main-axis offset of the first child for the given pack and slack.
func boxLead(pack BoxPack, slack int) int {
	if slack <= 0 {
		return 0
	}
	switch pack {
	case PackCenter:
		return slack / 2
	case PackEnd:
		return slack
	default: // PackStart
		return 0
	}
}

// naturalCross reports a child's natural cross-axis extent: from Measurer if it
// implements one, else its current Bounds cross dimension. For HBox (horizontal)
// the cross axis is height; for VBox it is width.
func naturalCross(w Widget, mainSize, crossAvail int, horizontal bool) int {
	if m, ok := w.(Measurer); ok {
		if horizontal { // HBox: main = width, cross = height
			_, mh := m.Measure(mainSize, crossAvail)
			return mh
		}
		mw, _ := m.Measure(crossAvail, mainSize)
		return mw
	}
	if horizontal {
		return w.Bounds().H
	}
	return w.Bounds().W
}

// boxCross returns a child's cross-axis offset + size within a band of extent
// cross starting at base, honouring align. BoxStretch (or a child with no usable
// natural size) fills the band; otherwise the child keeps its natural cross size
// and is pinned start/centre/end. mainSize is the child's main-axis size (offered
// to Measurer).
func boxCross(w Widget, align BoxAlign, base, cross, mainSize int, horizontal bool) (off, size int) {
	if align == BoxStretch {
		return base, cross
	}
	nat := naturalCross(w, mainSize, cross, horizontal)
	if nat <= 0 || nat >= cross {
		return base, cross // nothing sensible to align against → fill
	}
	switch align {
	case BoxAlignCenter:
		return base + (cross-nat)/2, nat
	case BoxAlignEnd:
		return base + (cross - nat), nat
	default: // BoxAlignStart
		return base, nat
	}
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
	// Spacing is the gap in pixels between adjacent children. NewHBox seeds it to
	// DefaultBoxSpacing (4); it is then honoured literally, so setting it to 0
	// yields a flush box and negative values are clamped to zero at layout time.
	Spacing int
	// Align positions each child on the cross (vertical) axis; the zero value
	// BoxStretch fills the height (the historical behaviour). Pack distributes
	// leftover width when the children do not fill the box (no flex child).
	Align    BoxAlign
	Pack     BoxPack
	children []boxChild
}

// NewHBox constructs an empty HBox with Spacing seeded to DefaultBoxSpacing. Add
// children via Append/AddFlex/AddFixed.
func NewHBox() *HBox { return &HBox{Spacing: DefaultBoxSpacing} }

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

// SetBounds positions the HBox + lays out its children across the width. An empty
// incoming rect (W<=0 or H<=0) collapses every child to Rect{} so a hidden box
// (e.g. an inactive CardLayout item) leaves no leaf with stale non-empty bounds.
func (h *HBox) SetBounds(r Rect) {
	h.Base.SetBounds(r)
	if len(h.children) == 0 {
		return
	}
	if r.W <= 0 || r.H <= 0 {
		for _, c := range h.children {
			c.w.SetBounds(Rect{})
		}
		return
	}
	sp := clampSpacing(h.Spacing)
	sizes := boxCells(r.W, sp, h.children)
	x := r.X + boxLead(h.Pack, boxSlack(r.W, sp, sizes))
	for i, c := range h.children {
		y, ch := boxCross(c.w, h.Align, r.Y, r.H, sizes[i], true)
		c.w.SetBounds(Rect{X: x, Y: y, W: sizes[i], H: ch})
		x += sizes[i] + sp
	}
}

// Draw paints every child in append order (the box itself draws nothing).
func (h *HBox) Draw(p painter.Painter, theme *Theme) {
	for _, c := range h.children {
		c.w.Draw(p, theme)
	}
}

// focusableChildren yields the box's children left-to-right so the focus walker
// can enumerate focusable descendants in visual order.
func (h *HBox) focusableChildren() []Widget {
	out := make([]Widget, len(h.children))
	for i, c := range h.children {
		out[i] = c.w
	}
	return out
}

// Children yields the box's child widgets in insertion order, so generic tree
// walkers (e.g. [CollectRuns]) can descend without knowing the box type.
func (h *HBox) Children() []Widget { return h.focusableChildren() }

// OnEvent forwards to the first child whose Bounds contains the event point,
// translated into that child's local space. EventMouseMove is forwarded to
// EVERY child (translated) instead, so the child under the pointer raises its
// hover face while the ones it left clear theirs. Keyboard events go through the
// focus system (routeFocusKey): Tab/Shift+Tab move focus, other keys route to
// the focused descendant; a click also moves focus to the focusable it hits.
func (h *HBox) OnEvent(ev Event) {
	if routeFocusKey(h, ev) {
		return
	}
	pr := h.Bounds()
	if ev.Kind == EventMouseMove {
		for _, c := range h.children {
			c.w.OnEvent(translateEvent(ev, pr, c.w.Bounds()))
		}
		return
	}
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	if ev.Kind == EventClick {
		focusClick(h, sx, sy)
	}
	for _, c := range h.children {
		if c.w.HitTest(sx, sy) {
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
	// HBox.Spacing (NewVBox seeds DefaultBoxSpacing, honoured literally, negatives
	// clamped to 0).
	Spacing int
	// Align positions each child on the cross (horizontal) axis; the zero value
	// BoxStretch fills the width. Pack distributes leftover height when the
	// children do not fill the box (no flex child). Same semantics as HBox.
	Align    BoxAlign
	Pack     BoxPack
	children []boxChild
}

// NewVBox constructs an empty VBox with Spacing seeded to DefaultBoxSpacing.
func NewVBox() *VBox { return &VBox{Spacing: DefaultBoxSpacing} }

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

// SetBounds positions the VBox + stacks its children down the height. An empty
// incoming rect (W<=0 or H<=0) collapses every child to Rect{} — see HBox.SetBounds.
func (v *VBox) SetBounds(r Rect) {
	v.Base.SetBounds(r)
	if len(v.children) == 0 {
		return
	}
	if r.W <= 0 || r.H <= 0 {
		for _, c := range v.children {
			c.w.SetBounds(Rect{})
		}
		return
	}
	sp := clampSpacing(v.Spacing)
	sizes := boxCells(r.H, sp, v.children)
	y := r.Y + boxLead(v.Pack, boxSlack(r.H, sp, sizes))
	for i, c := range v.children {
		x, cw := boxCross(c.w, v.Align, r.X, r.W, sizes[i], false)
		c.w.SetBounds(Rect{X: x, Y: y, W: cw, H: sizes[i]})
		y += sizes[i] + sp
	}
}

// Draw paints every child in append order.
func (v *VBox) Draw(p painter.Painter, theme *Theme) {
	for _, c := range v.children {
		c.w.Draw(p, theme)
	}
}

// focusableChildren yields the box's children top-to-bottom so the focus walker
// can enumerate focusable descendants in visual order.
func (v *VBox) focusableChildren() []Widget {
	out := make([]Widget, len(v.children))
	for i, c := range v.children {
		out[i] = c.w
	}
	return out
}

// Children yields the box's child widgets in insertion order, so generic tree
// walkers (e.g. [CollectRuns]) can descend without knowing the box type.
func (v *VBox) Children() []Widget { return v.focusableChildren() }

// OnEvent forwards to the first child containing the event point. EventMouseMove
// is forwarded to every child instead, so hover-enter and hover-leave both
// propagate (see HBox.OnEvent). Keyboard events go through the focus system and
// a click also moves focus to the focusable it hits (see HBox.OnEvent).
func (v *VBox) OnEvent(ev Event) {
	if routeFocusKey(v, ev) {
		return
	}
	pr := v.Bounds()
	if ev.Kind == EventMouseMove {
		for _, c := range v.children {
			c.w.OnEvent(translateEvent(ev, pr, c.w.Bounds()))
		}
		return
	}
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	if ev.Kind == EventClick {
		focusClick(v, sx, sy)
	}
	for _, c := range v.children {
		if c.w.HitTest(sx, sy) {
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

// Grid lays children out in a fixed cols x rows table. Children are placed via
// Attach(child, col, row); a cell with no attached child stays empty.
//
// By default every cell is the same size (container W/cols, H/rows) with no gutter
// — the historical, zero-config behaviour. Two additive fields refine that:
//
//   - Spacing adds an inter-cell gutter (in pixels) on BOTH axes. The gutters are
//     subtracted from the extent before the cells are sized. Default 0 = flush.
//   - ColWidths/RowHeights pin individual tracks to a fixed pixel size. An entry
//     of 0 (or a track index past the slice) is a FLEXIBLE track: after the fixed
//     tracks and gutters are removed, the remaining space is split equally among
//     the flexible tracks. An absent/empty slice makes every track flexible, i.e.
//     the all-equal historical layout.
//
// Grid is a Widget: Draw fans out to every attached child + OnEvent hit-tests then
// forwards.
type Grid struct {
	Base
	// Spacing is the inter-cell gutter in pixels applied on both axes (negatives
	// clamped to 0 at layout time). Default 0 keeps the historical flush grid.
	Spacing int
	// ColWidths/RowHeights pin individual tracks to a fixed pixel size; a 0 entry
	// (or a missing index) is a flexible track sharing the remaining space equally.
	// Absent/empty = all-flexible (the historical equal-cell layout).
	ColWidths  []int
	RowHeights []int
	cols, rows int
	children   []gridChild
}

// trackFixed returns the fixed size of track i from a ColWidths/RowHeights slice,
// or 0 (flexible) when the index is past the slice.
func trackFixed(fixed []int, i int) int {
	if i < len(fixed) {
		return fixed[i]
	}
	return 0
}

// gridTracks computes each track's pixel size and start offset along one axis:
// total is the axis extent, n the track count (>=1), spacing the inter-track
// gutter, and fixed the per-track fixed sizes (a >0 entry pins the track; 0 or a
// missing index is flexible). Fixed tracks keep their size; the space left after
// the gutters and fixed tracks is divided equally among the flexible tracks. Both
// returned slices have length n. With spacing 0 and no fixed tracks this yields
// exactly the historical total/n cells at col*cell offsets.
func gridTracks(total, n, spacing int, fixed []int) (sizes, offsets []int) {
	sizes = make([]int, n)
	offsets = make([]int, n)
	fixedSum, flexCount := 0, 0
	for i := 0; i < n; i++ {
		if f := trackFixed(fixed, i); f > 0 {
			sizes[i] = f
			fixedSum += f
		} else {
			flexCount++
		}
	}
	avail := total - spacing*(n-1) - fixedSum
	if avail < 0 {
		avail = 0
	}
	flexSize := 0
	if flexCount > 0 {
		flexSize = avail / flexCount
	}
	off := 0
	for i := 0; i < n; i++ {
		if trackFixed(fixed, i) <= 0 {
			sizes[i] = flexSize
		}
		offsets[i] = off
		off += sizes[i] + spacing
	}
	return sizes, offsets
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

// SetBounds positions the Grid + sizes every attached child to its (col, row)
// cell, honouring Spacing gutters and any fixed ColWidths/RowHeights tracks. An
// empty incoming rect (W<=0 or H<=0) collapses every child to Rect{} so a hidden
// grid leaves no leaf with stale bounds.
func (g *Grid) SetBounds(r Rect) {
	g.Base.SetBounds(r)
	if len(g.children) == 0 {
		return
	}
	if r.W <= 0 || r.H <= 0 {
		for _, c := range g.children {
			c.w.SetBounds(Rect{})
		}
		return
	}
	sp := clampSpacing(g.Spacing)
	colSizes, colOff := gridTracks(r.W, g.cols, sp, g.ColWidths)
	rowSizes, rowOff := gridTracks(r.H, g.rows, sp, g.RowHeights)
	for _, c := range g.children {
		c.w.SetBounds(Rect{
			X: r.X + colOff[c.col],
			Y: r.Y + rowOff[c.row],
			W: colSizes[c.col],
			H: rowSizes[c.row],
		})
	}
}

// Draw paints every attached child in attach order.
func (g *Grid) Draw(p painter.Painter, theme *Theme) {
	for _, c := range g.children {
		c.w.Draw(p, theme)
	}
}

// focusableChildren yields the grid's attached children in attach order so the
// focus walker can enumerate focusable descendants.
func (g *Grid) focusableChildren() []Widget {
	out := make([]Widget, len(g.children))
	for i, c := range g.children {
		out[i] = c.w
	}
	return out
}

// OnEvent hit-tests attached children + forwards with translated
// coordinates. EventMouseMove is forwarded to every attached child instead, so
// hover-enter and hover-leave both propagate (see HBox.OnEvent). Keyboard events
// go through the focus system and a click also moves focus to the focusable it
// hits (see HBox.OnEvent).
func (g *Grid) OnEvent(ev Event) {
	if routeFocusKey(g, ev) {
		return
	}
	pr := g.Bounds()
	if ev.Kind == EventMouseMove {
		for _, c := range g.children {
			c.w.OnEvent(translateEvent(ev, pr, c.w.Bounds()))
		}
		return
	}
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	if ev.Kind == EventClick {
		focusClick(g, sx, sy)
	}
	for _, c := range g.children {
		if c.w.HitTest(sx, sy) {
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
	// Title, when non-empty, draws a title bar across the top of the frame
	// (inside the border), turning the plain group-box into a titled panel.
	// The zero value "" keeps the original border-only box.
	Title string
	// Collapsible shows a ▼/▶ disclosure chevron in the title bar; a click
	// on the bar toggles Collapsed (and fires OnCollapse). It forces a title
	// bar even when Title is "".
	Collapsible bool
	// Collapsed hides the child, drawing only the title bar (Ext panel
	// collapse). Meaningful only when a title bar is present.
	Collapsed bool
	// OnCollapse fires when a title-bar click toggles Collapsed; the new
	// state is passed. Nil is safe.
	OnCollapse func(collapsed bool)
	child      Widget
}

// FrameTitleH is the pixel height of a Frame's optional title bar.
const FrameTitleH = 22

// NewFrame wraps child in a Frame. child may be nil (the Frame then
// just draws its border + accepts no events).
func NewFrame(child Widget) *Frame { return &Frame{child: child} }

// headerH is the title-bar height: FrameTitleH when the frame is titled or
// collapsible, else 0 (the plain border-only box, unchanged from before).
func (f *Frame) headerH() int {
	if f.Title != "" || f.Collapsible {
		return FrameTitleH
	}
	return 0
}

// SetBounds positions the Frame + resizes its child to fit inside the
// border, title bar (if any) + padding. A collapsed frame hides the child.
func (f *Frame) SetBounds(r Rect) {
	f.Base.SetBounds(r)
	if f.child == nil {
		return
	}
	hh := f.headerH()
	if f.Collapsed && hh > 0 {
		f.child.SetBounds(Rect{}) // hidden while collapsed
		return
	}
	pad := f.Padding
	if pad == 0 {
		pad = defaultPadding
	}
	if pad < 0 {
		pad = 0
	}
	// 1px border on each side plus pad on each side; the title bar (hh)
	// eats into the top only. With hh == 0 this is the original inset box.
	inset := 1 + pad
	top := r.Y + 1 + hh + pad
	f.child.SetBounds(Rect{
		X: r.X + inset,
		Y: top,
		W: r.W - 2*inset,
		H: r.Y + r.H - inset - top,
	})
}

// drawTitleBar paints the title-bar fill, the optional collapse chevron and
// the title text.
func (f *Frame) drawTitleBar(p painter.Painter, theme *Theme, r Rect, hh int) {
	fillRect(p, r.X+1, r.Y+1, r.W-2, hh, theme.SurfaceAlt)
	tx := r.X + 6
	if f.Collapsible {
		drawDisclosureChevron(p, r.X+6, r.Y+1+hh/2, !f.Collapsed, theme.OnSurface)
		tx = r.X + 16
	}
	ty := r.Y + 1 + (hh-f.glyphHeight())/2
	f.drawText(p, tx, ty, f.Title, theme.OnSurface)
}

// Draw paints the 1-pixel border, the title bar (if any) then the child. A
// collapsed frame draws only the title bar + a border around it.
func (f *Frame) Draw(p painter.Painter, theme *Theme) {
	r := f.Bounds()
	hh := f.headerH()
	if f.Collapsed && hh > 0 {
		strokeRect(p, r.X, r.Y, r.W, hh+2, theme.Border)
		f.drawTitleBar(p, theme, r, hh)
		return
	}
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	if hh > 0 {
		f.drawTitleBar(p, theme, r, hh)
	}
	if f.child != nil {
		f.child.Draw(p, theme)
	}
}

// focusableChildren yields the frame's single child (if any) so the focus
// walker can descend into it.
func (f *Frame) focusableChildren() []Widget {
	if f.child == nil {
		return nil
	}
	return []Widget{f.child}
}

// OnEvent toggles Collapsed on a title-bar click (when Collapsible), else
// forwards to the child if the event lands inside its Bounds. Keyboard events go
// through the focus system first (Tab/Shift+Tab traversal + routing to the
// focused descendant); a click inside the child also moves focus to the
// focusable it hits.
func (f *Frame) OnEvent(ev Event) {
	if routeFocusKey(f, ev) {
		return
	}
	hh := f.headerH()
	// The title-bar band drawn by drawTitleBar spans widget-local Y in
	// [1, 1+hh) (the 1-px top border sits at Y == 0); match it exactly so a
	// click on the border above the bar, or the first content row below it,
	// does not toggle.
	if f.Collapsible && ev.Kind == EventClick && ev.Y >= 1 && ev.Y < 1+hh {
		f.Collapsed = !f.Collapsed
		if f.OnCollapse != nil {
			f.OnCollapse(f.Collapsed)
		}
		return
	}
	if f.child == nil || (f.Collapsed && hh > 0) {
		return
	}
	pr := f.Bounds()
	// A move is forwarded to the child unconditionally (translated) so it can
	// clear its hover face once the pointer leaves it; every other kind lands
	// only when the point is inside the child.
	if ev.Kind == EventMouseMove {
		f.child.OnEvent(translateEvent(ev, pr, f.child.Bounds()))
		return
	}
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	if ev.Kind == EventClick {
		focusClick(f, sx, sy)
	}
	if f.child.HitTest(sx, sy) {
		f.child.OnEvent(translateEvent(ev, pr, f.child.Bounds()))
	}
}
