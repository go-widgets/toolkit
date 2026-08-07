// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package virtual adds live-data list virtualization on top of the
// go-widgets/toolkit widget set. Where the core ListBox / Table / TreeView
// window a static []string, the widgets here bind to a live
// mvvm.ObservableList and realise ONLY the rows (or cells) that intersect the
// viewport, so a model with a million items costs the same per frame as one
// with a handful.
//
// Two widgets are provided, both toolkit.Widget (Draw / OnEvent / Bounds /
// SetBounds via the embedded toolkit.Base):
//
//   - VirtualList — a scrollable vertical list with a per-row draw callback and
//     optional VARIABLE row heights. The scroll-offset → first-visible-row
//     lookup is O(1) when every row is the same height and O(log n) via a
//     Fenwick (binary-indexed) prefix-sum tree when heights vary.
//   - VirtualGrid — the gengrid analogue: it reflows N uniform cells into as
//     many columns as the width allows and realises only the visible cells.
//
// Both subscribe to their model's ListEvent stream and keep the scroll anchor
// stable across mutations: an insert (or remove) ABOVE the viewport shifts the
// offset so the rows on screen do not jump, while a change below the top item
// leaves the offset untouched. Rendering reuses painter.Clipper (when the
// back-end supports it) to clip the partially-visible trailing item to the
// exact viewport edge.
package virtual

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// DefaultRowHeight is the row height a VirtualList uses when its RowHeight
// callback is nil — a comfortable uniform default that mirrors the core
// ListBox's proportions.
const DefaultRowHeight = 20

// ---------------------------------------------------------------------------
// height index — the O(1)/O(log n) offset↔row model
// ---------------------------------------------------------------------------

// heightIndex answers two queries over a list of row heights: prefix(row) (the
// pixel Y at which row starts) and locate(offset) (the first row whose span
// contains pixel offset). When every row is the same height it stores a single
// height and both queries are plain arithmetic (O(1)); when heights vary it
// builds a Fenwick / binary-indexed prefix-sum tree so both queries stay
// O(log n) instead of O(n).
type heightIndex struct {
	n       int
	uniform bool
	rowH    int   // uniform heights only
	heights []int // per-row heights (variable case) for O(1) heightAt in Draw
	fen     []int // 1-indexed Fenwick tree of heights; nil when uniform
	total   int   // total content height in pixels
	hi      int   // highest power of two ≤ n, for the Fenwick binary lift
}

// norm clamps a caller-supplied height to a non-negative value, so a callback
// that returns a bogus negative height degrades to a zero-height row rather
// than corrupting the prefix sums.
func norm(h int) int {
	if h < 0 {
		return 0
	}
	return h
}

// buildIndex scans the n row heights once. If they are all equal it keeps the
// uniform O(1) fast path; otherwise it builds a Fenwick tree for O(log n)
// lookups. The scan is the only O(n) work and happens just at (re)build time,
// never per scroll tick.
func buildIndex(n int, h func(i int) int) *heightIndex {
	idx := &heightIndex{n: n}
	if n <= 0 {
		idx.uniform = true
		return idx
	}
	h0 := norm(h(0))
	uniform := true
	total := h0
	for i := 1; i < n; i++ {
		hi := norm(h(i))
		total += hi
		if hi != h0 {
			uniform = false
		}
	}
	idx.total = total
	if uniform {
		idx.uniform = true
		idx.rowH = h0
		return idx
	}
	heights := make([]int, n)
	fen := make([]int, n+1)
	for i := 0; i < n; i++ {
		hv := norm(h(i))
		heights[i] = hv
		fen[i+1] = hv
	}
	for i := 1; i <= n; i++ {
		if j := i + (i & -i); j <= n {
			fen[j] += fen[i]
		}
	}
	idx.heights = heights
	idx.fen = fen
	p := 1
	for p<<1 <= n {
		p <<= 1
	}
	idx.hi = p
	return idx
}

// prefix returns the cumulative height of rows [0, row) — i.e. the pixel Y at
// which row would start. row is clamped to [0, n].
func (idx *heightIndex) prefix(row int) int {
	if row <= 0 {
		return 0
	}
	if row > idx.n {
		row = idx.n
	}
	if idx.uniform {
		return row * idx.rowH
	}
	sum := 0
	for i := row; i > 0; i -= i & -i {
		sum += idx.fen[i]
	}
	return sum
}

// heightAt returns row i's height, or 0 for an out-of-range index.
func (idx *heightIndex) heightAt(i int) int {
	if i < 0 || i >= idx.n {
		return 0
	}
	if idx.uniform {
		return idx.rowH
	}
	return idx.heights[i]
}

// locate returns the first row whose vertical span contains pixel offset — the
// largest row with prefix(row) ≤ offset. O(1) for uniform heights, O(log n)
// via a Fenwick binary lift otherwise.
func (idx *heightIndex) locate(offset int) int {
	if idx.n == 0 || offset <= 0 {
		return 0
	}
	if offset >= idx.total {
		return idx.n - 1
	}
	if idx.uniform {
		return offset / idx.rowH
	}
	pos, rem := 0, offset
	for k := idx.hi; k > 0; k >>= 1 {
		next := pos + k
		if next <= idx.n && idx.fen[next] <= rem {
			pos = next
			rem -= idx.fen[next]
		}
	}
	return pos
}

// remapMoveTop maps a tracked "top" item index across an mvvm ListMove of the
// item at from to the destination index to (both in mvvm's final-index space:
// remove at from, then insert at to). It keeps a virtualized view's scroll
// anchor pointing at the same logical item after a move.
func remapMoveTop(top, from, to int) int {
	if from == to {
		return top
	}
	if top == from {
		return to
	}
	t := top
	if top > from {
		t--
	}
	if t >= to {
		t++
	}
	return t
}

// ---------------------------------------------------------------------------
// VirtualList
// ---------------------------------------------------------------------------

// VirtualList binds a scrollable vertical list to a live mvvm.ObservableList
// and a per-row draw callback, realising only the rows that intersect the
// viewport. Row heights may be uniform (a constant RowHeight, or nil for
// DefaultRowHeight) or variable (an arbitrary RowHeight function); the
// scroll-offset → first-visible-row lookup is O(1) in the uniform case and
// O(log n) via a Fenwick prefix-sum tree in the variable case. It subscribes
// to the model and holds the scroll anchor stable across inserts / removes
// above the viewport.
type VirtualList[T any] struct {
	toolkit.Base

	// Model is the live backing collection. Setting it (via the field or
	// NewVirtualList) rewires the subscription on the next operation.
	Model *mvvm.ObservableList[T]
	// RowHeight returns row i's pixel height. A constant function yields the
	// uniform O(1) fast path; a varying one drives the Fenwick index. nil means
	// a uniform DefaultRowHeight.
	RowHeight func(i int) int
	// Render draws one row into rectangle r (its exact on-screen span). It is
	// invoked only for rows currently in the viewport.
	Render func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item T)

	// ScrollOffset is the pixel offset of the viewport top from the top of the
	// content. Reads clamp it to [0, maxOffset]; prefer ScrollTo / ScrollBy.
	ScrollOffset int

	idx        *heightIndex
	unsub      func()
	subscribed *mvvm.ObservableList[T]
}

var _ toolkit.Widget = (*VirtualList[int])(nil)

// NewVirtualList builds a VirtualList over model with the given per-row height
// and draw callbacks, wiring the model subscription immediately.
func NewVirtualList[T any](
	model *mvvm.ObservableList[T],
	rowHeight func(i int) int,
	render func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item T),
) *VirtualList[T] {
	v := &VirtualList[T]{Model: model, RowHeight: rowHeight, Render: render}
	v.ensure()
	return v
}

// modelLen is the backing list length (0 for a nil model).
func (v *VirtualList[T]) modelLen() int {
	if v.Model == nil {
		return 0
	}
	return v.Model.Len()
}

// rowHeightFn is the effective height callback (RowHeight, or a uniform
// DefaultRowHeight when unset).
func (v *VirtualList[T]) rowHeightFn() func(i int) int {
	if v.RowHeight != nil {
		return v.RowHeight
	}
	return func(int) int { return DefaultRowHeight }
}

// ensure lazily (re)subscribes when Model changes and (re)builds the height
// index when it has been invalidated. Called at the head of every read; a
// no-op — and allocation-free — once the model is subscribed and the index is
// current, which is the steady state a scroll tick runs in.
func (v *VirtualList[T]) ensure() {
	if v.Model != v.subscribed {
		if v.unsub != nil {
			v.unsub()
			v.unsub = nil
		}
		v.subscribed = v.Model
		v.idx = nil
		if v.Model != nil {
			v.unsub = v.Model.Subscribe(v.onChange)
		}
	}
	if v.idx == nil {
		v.idx = buildIndex(v.modelLen(), v.rowHeightFn())
	}
}

// Close unsubscribes from the model. Safe to call more than once.
func (v *VirtualList[T]) Close() {
	if v.unsub != nil {
		v.unsub()
		v.unsub = nil
	}
	v.subscribed = nil
}

// clampOffset clamps off to [0, total-viewportHeight].
func (v *VirtualList[T]) clampOffset(off int) int {
	if off < 0 {
		return 0
	}
	m := v.idx.total - v.Bounds().H
	if m < 0 {
		m = 0
	}
	if off > m {
		return m
	}
	return off
}

// ScrollTo sets the pixel scroll offset, clamped to the valid range.
func (v *VirtualList[T]) ScrollTo(offset int) {
	v.ensure()
	v.ScrollOffset = v.clampOffset(offset)
}

// ScrollBy shifts the pixel scroll offset by delta, clamped.
func (v *VirtualList[T]) ScrollBy(delta int) {
	v.ensure()
	v.ScrollOffset = v.clampOffset(v.ScrollOffset + delta)
}

// ScrollByRows shifts the viewport by whole rows (negative scrolls up),
// snapping the offset to the resulting row's top so wheel scrolling advances a
// row at a time regardless of variable heights.
func (v *VirtualList[T]) ScrollByRows(delta int) {
	v.ensure()
	if v.idx.n == 0 {
		return
	}
	first := v.idx.locate(v.clampOffset(v.ScrollOffset))
	nf := first + delta
	if nf < 0 {
		nf = 0
	}
	if nf > v.idx.n {
		nf = v.idx.n
	}
	v.ScrollOffset = v.clampOffset(v.idx.prefix(nf))
}

// VisibleRange returns the index of the first visible row and the number of
// rows that intersect the viewport (including the partially-visible top and
// bottom rows). It is O(1) for uniform heights and O(log n) otherwise, and
// allocation-free, so it is safe to call every scroll tick.
func (v *VirtualList[T]) VisibleRange() (first, count int) {
	v.ensure()
	if v.idx.n == 0 || v.idx.total <= 0 {
		return 0, 0
	}
	h := v.Bounds().H
	if h <= 0 {
		return 0, 0
	}
	off := v.clampOffset(v.ScrollOffset)
	first = v.idx.locate(off)
	last := v.idx.locate(off + h - 1)
	count = last - first + 1
	return first, count
}

// onChange keeps the scroll anchor stable across a model mutation: it locates
// the top visible row + the sub-row remainder BEFORE the index is rebuilt,
// shifts that anchor by the change when it lands at or above the top, rebuilds
// the index against the mutated model, and restores the offset so the same
// content stays on screen (an insert/remove above the viewport does not make
// the visible rows jump). A Reset scrolls back to the top.
func (v *VirtualList[T]) onChange(ev mvvm.ListEvent[T]) {
	off := v.clampOffset(v.ScrollOffset)
	top := v.idx.locate(off)
	rem := off - v.idx.prefix(top)
	switch ev.Kind {
	case mvvm.ListInsert:
		if ev.Index <= top {
			top += ev.Count
		}
	case mvvm.ListRemove:
		switch {
		case ev.Index+ev.Count <= top:
			top -= ev.Count
		case ev.Index <= top:
			top = ev.Index
			rem = 0
		}
	case mvvm.ListMove:
		top = remapMoveTop(top, ev.Index, ev.To)
	case mvvm.ListReset:
		top = 0
		rem = 0
	}
	v.idx = buildIndex(v.modelLen(), v.rowHeightFn())
	v.ScrollOffset = v.clampOffset(v.idx.prefix(top) + rem)
}

// Draw paints only the rows intersecting the viewport, positioning row i at its
// exact content Y minus the scroll offset. When the content overflows the
// bounds it pushes a clip rect (if the painter supports Clipper) so the
// partially-visible trailing (and leading) row is clipped to the viewport edge.
func (v *VirtualList[T]) Draw(p painter.Painter, th *toolkit.Theme) {
	v.ensure()
	r := v.Bounds()
	if r.W <= 0 || r.H <= 0 || v.idx.n == 0 || v.Render == nil {
		return
	}
	off := v.clampOffset(v.ScrollOffset)
	overflow := v.idx.total > r.H

	var clr painter.Clipper
	canClip := false
	if overflow {
		clr, canClip = p.(painter.Clipper)
		if canClip {
			clr.PushClip(r)
		}
	}

	first, count := v.VisibleRange()
	y := r.Y + v.idx.prefix(first) - off
	for k := 0; k < count; k++ {
		i := first + k
		hgt := v.idx.heightAt(i)
		v.Render(p, th, toolkit.Rect{X: r.X, Y: y, W: r.W, H: hgt}, i, v.Model.At(i))
		y += hgt
	}

	if overflow && canClip {
		clr.PopClip()
	}
}

// OnEvent handles a wheel EventScroll by scrolling whole rows; every other
// event kind is ignored (the row-content widgets the Render callback draws
// handle their own input).
func (v *VirtualList[T]) OnEvent(ev toolkit.Event) {
	if ev.Kind == toolkit.EventScroll {
		v.ScrollByRows(ev.Delta)
	}
}

// ---------------------------------------------------------------------------
// VirtualGrid — the gengrid analogue
// ---------------------------------------------------------------------------

// VirtualGrid reflows N uniform-sized cells (CellSize) into as many columns as
// the widget width allows and realises only the cells that intersect the
// viewport — the recycled 2-D card / thumbnail grid. It binds to a live
// mvvm.ObservableList and keeps its scroll anchor stable across mutations above
// the viewport, exactly like VirtualList.
type VirtualGrid[T any] struct {
	toolkit.Base

	// Model is the live backing collection.
	Model *mvvm.ObservableList[T]
	// CellSize is every cell's fixed footprint in painter units.
	CellSize toolkit.Size
	// Render draws one cell into rectangle r (its exact on-screen span),
	// invoked only for the cells currently in the viewport.
	Render func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item T)

	// ScrollOffset is the pixel offset of the viewport top from the top of the
	// content. Reads clamp it; prefer ScrollTo / ScrollBy.
	ScrollOffset int

	// anchor is the item index the grid keeps pinned to the top-left of the
	// viewport across model mutations. It is resynced from the offset whenever
	// the user scrolls (or the bounds change) and shifted — not recomputed — on
	// each model change, so a run of small inserts above the viewport
	// accumulates correctly instead of quantising away sub-row shifts.
	anchor int

	unsub      func()
	subscribed *mvvm.ObservableList[T]
}

var _ toolkit.Widget = (*VirtualGrid[int])(nil)

// NewVirtualGrid builds a VirtualGrid over model with the given cell size and
// draw callback, wiring the model subscription immediately.
func NewVirtualGrid[T any](
	model *mvvm.ObservableList[T],
	cell toolkit.Size,
	render func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item T),
) *VirtualGrid[T] {
	g := &VirtualGrid[T]{Model: model, CellSize: cell, Render: render}
	g.ensure()
	return g
}

// modelLen is the backing list length (0 for a nil model).
func (g *VirtualGrid[T]) modelLen() int {
	if g.Model == nil {
		return 0
	}
	return g.Model.Len()
}

// ensure lazily (re)subscribes when Model changes. The grid needs no prefix-sum
// index (cells are uniform), so this is a no-op — and allocation-free — once
// the model is subscribed.
func (g *VirtualGrid[T]) ensure() {
	if g.Model != g.subscribed {
		if g.unsub != nil {
			g.unsub()
			g.unsub = nil
		}
		g.subscribed = g.Model
		if g.Model != nil {
			g.unsub = g.Model.Subscribe(g.onChange)
		}
	}
}

// Close unsubscribes from the model. Safe to call more than once.
func (g *VirtualGrid[T]) Close() {
	if g.unsub != nil {
		g.unsub()
		g.unsub = nil
	}
	g.subscribed = nil
}

// cols is how many columns fit across the current width (at least 1 whenever
// there is any width and cell width to work with, 0 when the grid cannot lay
// out at all).
func (g *VirtualGrid[T]) cols() int {
	w := g.Bounds().W
	if g.CellSize.W <= 0 || w <= 0 {
		return 0
	}
	c := w / g.CellSize.W
	if c < 1 {
		c = 1
	}
	return c
}

// contentHeight is the total pixel height of all cell rows at the current
// column count.
func (g *VirtualGrid[T]) contentHeight() int {
	n := g.modelLen()
	c := g.cols()
	if c <= 0 || g.CellSize.H <= 0 || n == 0 {
		return 0
	}
	rows := (n + c - 1) / c
	return rows * g.CellSize.H
}

// clampOffset clamps off to [0, contentHeight-viewportHeight].
func (g *VirtualGrid[T]) clampOffset(off int) int {
	if off < 0 {
		return 0
	}
	m := g.contentHeight() - g.Bounds().H
	if m < 0 {
		m = 0
	}
	if off > m {
		return m
	}
	return off
}

// SetBounds positions the grid and resyncs the anchor, since a width change
// reflows the columns (and so changes which item sits at the top-left).
func (g *VirtualGrid[T]) SetBounds(r toolkit.Rect) {
	g.Base.SetBounds(r)
	g.syncAnchor()
}

// syncAnchor recomputes the anchored top-left item from the current offset. It
// runs on user-driven scrolls (and bounds changes) — never on a model change,
// where the anchor is shifted instead, so small mutations accumulate exactly.
func (g *VirtualGrid[T]) syncAnchor() {
	c := g.cols()
	ch := g.CellSize.H
	if c <= 0 || ch <= 0 {
		g.anchor = 0
		return
	}
	g.anchor = (g.clampOffset(g.ScrollOffset) / ch) * c
}

// ScrollTo sets the pixel scroll offset, clamped to the valid range, and
// resyncs the anchor.
func (g *VirtualGrid[T]) ScrollTo(offset int) {
	g.ScrollOffset = g.clampOffset(offset)
	g.syncAnchor()
}

// ScrollBy shifts the pixel scroll offset by delta, clamped.
func (g *VirtualGrid[T]) ScrollBy(delta int) {
	g.ScrollTo(g.clampOffset(g.ScrollOffset) + delta)
}

// ScrollByRows shifts the viewport by whole cell-rows (negative scrolls up).
func (g *VirtualGrid[T]) ScrollByRows(delta int) {
	if g.CellSize.H <= 0 {
		return
	}
	g.ScrollTo(g.clampOffset(g.ScrollOffset) + delta*g.CellSize.H)
}

// VisibleRange returns the index of the first visible cell and the number of
// cells intersecting the viewport (a whole number of rows' worth, clamped to
// the model length). Allocation-free.
func (g *VirtualGrid[T]) VisibleRange() (first, count int) {
	n := g.modelLen()
	c := g.cols()
	ch := g.CellSize.H
	h := g.Bounds().H
	if n == 0 || c <= 0 || ch <= 0 || h <= 0 {
		return 0, 0
	}
	off := g.clampOffset(g.ScrollOffset)
	firstRow := off / ch
	lastRow := (off + h - 1) / ch
	first = firstRow * c
	last := (lastRow+1)*c - 1
	if last >= n {
		last = n - 1
	}
	count = last - first + 1
	return first, count
}

// onChange keeps the scroll anchor stable across a model mutation by tracking
// the top-left visible cell's item index: it shifts that index by an
// insert/remove at or above it, then restores the offset to the row the same
// item now lands on (preserving the sub-row remainder). A Reset scrolls to the
// top. When the grid has no usable geometry yet the offset is simply left to be
// clamped on the next read.
func (g *VirtualGrid[T]) onChange(ev mvvm.ListEvent[T]) {
	c := g.cols()
	ch := g.CellSize.H
	if c <= 0 || ch <= 0 {
		return
	}
	rem := g.clampOffset(g.ScrollOffset) % ch
	a := g.anchor
	switch ev.Kind {
	case mvvm.ListInsert:
		if ev.Index <= a {
			a += ev.Count
		}
	case mvvm.ListRemove:
		switch {
		case ev.Index+ev.Count <= a:
			a -= ev.Count
		case ev.Index <= a:
			a = ev.Index
			rem = 0
		}
	case mvvm.ListMove:
		a = remapMoveTop(a, ev.Index, ev.To)
	case mvvm.ListReset:
		a = 0
		rem = 0
	}
	g.anchor = a
	g.ScrollOffset = g.clampOffset((a/c)*ch + rem)
}

// Draw paints only the cells intersecting the viewport, each at its reflowed
// column/row position minus the scroll offset. When the content overflows it
// clips (if the painter supports Clipper) so partially-visible edge rows are
// trimmed to the viewport.
func (g *VirtualGrid[T]) Draw(p painter.Painter, th *toolkit.Theme) {
	g.ensure()
	r := g.Bounds()
	n := g.modelLen()
	c := g.cols()
	ch := g.CellSize.H
	cw := g.CellSize.W
	if r.W <= 0 || r.H <= 0 || n == 0 || c <= 0 || ch <= 0 || cw <= 0 || g.Render == nil {
		return
	}
	off := g.clampOffset(g.ScrollOffset)
	overflow := g.contentHeight() > r.H

	var clr painter.Clipper
	canClip := false
	if overflow {
		clr, canClip = p.(painter.Clipper)
		if canClip {
			clr.PushClip(r)
		}
	}

	first, count := g.VisibleRange()
	for k := 0; k < count; k++ {
		i := first + k
		row := i / c
		col := i % c
		rc := toolkit.Rect{X: r.X + col*cw, Y: r.Y + row*ch - off, W: cw, H: ch}
		g.Render(p, th, rc, i, g.Model.At(i))
	}

	if overflow && canClip {
		clr.PopClip()
	}
}

// OnEvent handles a wheel EventScroll by scrolling whole cell-rows; every other
// event kind is ignored.
func (g *VirtualGrid[T]) OnEvent(ev toolkit.Event) {
	if ev.Kind == toolkit.EventScroll {
		g.ScrollByRows(ev.Delta)
	}
}
