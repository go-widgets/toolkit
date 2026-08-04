// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"sort"
	"strconv"
	"strings"

	"github.com/go-widgets/painter"
)

// ListBox is a vertical list of selectable string rows. Click on a
// row selects it + fires OnActivate.
//
// Visual: each row is RowHeight pixels tall. The selected row uses
// Theme.Accent as background + Theme.Background as ink; unselected
// rows use Theme.Surface + Theme.OnSurface. Rows are rendered via
// font.DrawText with a 4 px left margin.
//
// Multi-selection: setting MultiSelect enables Ctrl/Shift-modified
// clicks that build a set of selected rows (see IsSelected /
// SelectedIndices / SetSelection / ClearSelection / ToggleSelect /
// SelectRange). Selected remains the anchor/cursor row -- the point a
// Shift-click range is measured from, and the row most recently
// clicked (plain or Ctrl). When MultiSelect is false (the default)
// none of this is reachable: Ctrl/Shift are ignored and only Selected
// is ever highlighted, exactly as before this feature existed.
//
// Virtual scrolling: ListBox is self-contained -- it never relies on
// an outer ScrollView. ScrollRow is the index of the top visible row;
// Draw paints only the rows that fit in Bounds().H (windowed
// rendering, so a list with thousands of rows costs the same per
// frame as one with a handful), and OnEvent maps click coordinates
// back through ScrollRow so hit-testing stays correct while scrolled.
// See ScrollTo / ScrollBy. When every row already fits in the
// viewport (len(Items) <= the number of visible rows) rendering is
// byte-identical to a ListBox with no scrolling at all -- no
// scrollbar is drawn and the windowing has no visible effect.
//
// Drag-to-reorder: setting Reorderable turns the ListBox into both a
// DragSource and a DropTarget for its own private "listrow:" payload
// scheme (see ListRowDragPrefix / DragData / AcceptsDrop) -- a host wires
// its native drag gestures to the widget exactly as it would for any other
// DragSource/DropTarget pair (see dnd.go), and the ListBox handles
// tracking the pressed row, painting an insertion-line indicator on
// EventDragMove, and reordering Items in place on EventDrop, firing
// OnReorder. When Reorderable is false (the default) none of this is
// reachable: DragData always returns "", AcceptsDrop always returns
// false, EventDragMove/EventDragLeave/EventDrop are no-ops, and Draw never
// paints an indicator -- rendering + behavior are byte-identical to a
// ListBox with no drag-to-reorder support at all.
type ListBox struct {
	Base
	Items       []string
	Selected    int // -1 = no selection; anchor/cursor row
	RowHeight   int // pixels per row; default 18 via NewListBox
	OnActivate  func(idx int)
	MultiSelect bool // enable Ctrl/Shift multi-row selection

	// ItemRenderer, when non-nil, draws each row's CONTENT instead of the
	// default single line of text. It is handed the row's content rectangle
	// rc (full row height, minus the scrollbar gutter), the row index, the
	// item string, whether the row is selected, and the resolved text ink
	// (theme.OnSurface, or theme.Background when selected). The ListBox still
	// paints the row background (selection highlight) and owns scrolling,
	// selection and drag-reorder -- the renderer only fills in the content, so
	// a host can draw an icon + multi-line text, badges, a progress bar, etc.
	// This is the DataView seam. The zero value (nil) keeps the original
	// one-line text render, byte-identical to before this field existed.
	ItemRenderer func(p painter.Painter, theme *Theme, rc Rect, index int, item string, selected bool, ink RGBA)

	// Reorderable enables drag-to-reorder (see the type doc). Default
	// false leaves the ListBox exactly as it behaved before this feature
	// existed.
	Reorderable bool

	// OnReorder fires after a successful drag-reorder with the row's
	// original index (from) and its final index after the move (to).
	// Nil-guarded; never called while Reorderable is false.
	OnReorder func(from, to int)

	// ScrollRow is the index of the row painted at the very top of the
	// widget's bounds. Reads through Draw/OnEvent are clamped to
	// [0, maxScrollRow()] on the fly (see clampedScrollRow), so setting
	// this directly to an out-of-range value is safe -- it just behaves
	// as whichever in-range value it clamps to. Prefer ScrollTo/ScrollBy,
	// which clamp + write back immediately.
	ScrollRow int

	// selected holds the multi-selection set. Only consulted for
	// rendering/queries when MultiSelect is true, but the mutator
	// methods (SetSelection, ToggleSelect, SelectRange, ...) work
	// regardless of MultiSelect so callers can drive selection
	// programmatically before switching the widget into multi mode.
	selected map[int]bool

	// pressedRow is the row hit by the most recent valid EventClick, or
	// -1 if none has landed yet. DragData reads it to know which row a
	// drag beginning "now" should carry -- a host starts a drag right
	// after the mousedown that already reached us as EventClick, so the
	// pressed row IS the row being dragged.
	pressedRow int

	// dropIndicator is the insertion boundary (an index in
	// [0, len(Items)]) Draw paints a line at while a Reorderable drag
	// hovers over this ListBox, or -1 when there is none to show. Driven
	// by EventDragMove/EventDragLeave/EventDrop.
	dropIndicator int

	// sbDrag tracks an in-progress drag of the vertical scrollbar thumb.
	sbDrag scrollDrag
}

// NewListBox builds a ListBox containing items. Selected starts at
// -1 (no row selected) and RowHeight defaults to 18 (a comfortable
// 7-px font + 11 px vertical padding).
func NewListBox(items []string) *ListBox {
	return &ListBox{
		Items:         items,
		Selected:      -1,
		RowHeight:     18,
		pressedRow:    -1,
		dropIndicator: -1,
	}
}

// ListBox is a DragSource + DropTarget when Reorderable (see the type doc).
var (
	_ DragSource = (*ListBox)(nil)
	_ DropTarget = (*ListBox)(nil)
)

// ListRowDragPrefix is the payload scheme ListBox's drag-to-reorder gesture
// uses: DragData returns ListRowDragPrefix followed by the pressed row's
// decimal index, and AcceptsDrop only recognizes payloads carrying this
// prefix -- so a foreign payload (say, a file path offered to a DropZone)
// is never mistaken for a reorder drag.
const ListRowDragPrefix = "listrow:"

// DragData returns the drag-to-reorder payload for the row hit by the most
// recent EventClick (see pressedRow), or "" when Reorderable is false or no
// row has been pressed yet.
func (l *ListBox) DragData() string {
	if !l.Reorderable || l.pressedRow < 0 {
		return ""
	}
	return ListRowDragPrefix + strconv.Itoa(l.pressedRow)
}

// AcceptsDrop reports whether payload is a reorder drag carrying ListBox's
// own "listrow:" scheme. It is always false when Reorderable is false, and
// false for any payload that doesn't carry the scheme (e.g. a different
// DragSource's payload).
func (l *ListBox) AcceptsDrop(payload string) bool {
	if !l.Reorderable {
		return false
	}
	return strings.HasPrefix(payload, ListRowDragPrefix)
}

// Draw paints only the rows currently within the scroll window --
// [ScrollRow, ScrollRow+visibleRows) -- positioning row i at
// top + (i-ScrollRow)*RowHeight. When every row already fits
// (len(Items) <= visibleRows() and ScrollRow clamps to 0), that
// window covers the whole list and rendering is byte-identical to a
// non-scrolling ListBox: no scrollbar, no clipping, full-width rows.
//
// When the list overflows the viewport, rows are clipped to the
// content area (via painter.Clipper, if the backend supports it) so
// a partially-visible trailing row never bleeds past Bounds().H, and
// a thin scrollbar track+thumb is painted on the right edge.
func (l *ListBox) Draw(p painter.Painter, theme *Theme) {
	r := l.Bounds()
	vr := l.visibleRows()
	overflow := len(l.Items) > vr

	cr := r // content rect: full bounds, minus the scrollbar column if any
	if overflow {
		cr.W -= scrollbarWidth
	}

	var clr painter.Clipper
	canClip := false
	if overflow {
		clr, canClip = p.(painter.Clipper)
		if canClip {
			clr.PushClip(cr)
		}
	}

	start := l.clampedScrollRow()
	end := start + vr
	if end > len(l.Items) {
		end = len(l.Items)
	}
	for i := start; i < end; i++ {
		item := l.Items[i]
		y := r.Y + (i-start)*l.RowHeight
		bg := theme.Surface
		ink := theme.OnSurface
		hi := i == l.Selected
		if l.MultiSelect {
			hi = l.IsSelected(i)
		}
		if hi {
			bg = theme.Accent
			ink = theme.Background
		}
		fillRect(p, cr.X, y, cr.W, l.RowHeight, bg)
		if l.ItemRenderer != nil {
			// DataView seam: hand the whole row content rect to the host.
			l.ItemRenderer(p, theme, Rect{X: cr.X, Y: y, W: cr.W, H: l.RowHeight}, i, item, hi, ink)
		} else {
			// Vertically centre the 7-px glyph inside the row.
			textY := y + (l.RowHeight-l.glyphHeight())/2
			l.drawText(p, cr.X+4, textY, item, ink)
		}
	}

	if l.Reorderable && l.dropIndicator >= 0 {
		l.drawDropIndicator(p, theme, cr, start, end)
	}

	if overflow && canClip {
		clr.PopClip()
	}
	if overflow {
		l.drawScrollbar(p, theme, r)
	}
}

// dropIndicatorHeight is the pixel thickness of the insertion line Draw
// paints between rows while a Reorderable drag hovers over this ListBox.
const dropIndicatorHeight = 2

// drawDropIndicator paints the reorder insertion line at dropIndicator's
// row boundary, straddling the gap between the row above and the row
// below it. cr is the content rect (full width minus the scrollbar column,
// if any); start/end are the currently-visible row window as computed by
// Draw. A boundary outside [start, end] -- the drag is hovering a row
// that's currently scrolled off-screen -- paints nothing, rather than an
// indicator jammed against the top/bottom edge.
func (l *ListBox) drawDropIndicator(p painter.Painter, theme *Theme, cr Rect, start, end int) {
	rel := l.dropIndicator - start
	if rel < 0 || rel > end-start {
		return
	}
	y := cr.Y + rel*l.RowHeight - dropIndicatorHeight/2
	if y < cr.Y {
		y = cr.Y
	}
	fillRect(p, cr.X, y, cr.W, dropIndicatorHeight, theme.Accent)
}

// visibleRows is how many rows fit vertically within Bounds().H at
// RowHeight, rounded UP so a partially-visible trailing row still
// counts (Draw then clips it to the exact pixel boundary). A
// non-positive RowHeight or Bounds().H both collapse to 0 -- no rows
// fit, and callers must not divide by RowHeight in that case.
func (l *ListBox) visibleRows() int {
	if l.RowHeight <= 0 {
		return 0
	}
	h := l.Bounds().H
	if h <= 0 {
		return 0
	}
	n := h / l.RowHeight
	if h%l.RowHeight != 0 {
		n++
	}
	return n
}

// maxScrollRow is the highest ScrollRow that still leaves a full
// window of content on screen: len(Items) - visibleRows(), floored
// at 0 so a list that already fits the viewport never scrolls.
func (l *ListBox) maxScrollRow() int {
	m := len(l.Items) - l.visibleRows()
	if m < 0 {
		return 0
	}
	return m
}

// clampedScrollRow returns ScrollRow clamped to [0, maxScrollRow()]
// WITHOUT mutating the field. Draw + OnEvent read through this
// instead of ScrollRow directly, so an out-of-range value (set
// directly, or left stale after Items shrank) never paints or
// hit-tests outside the valid window.
func (l *ListBox) clampedScrollRow() int {
	s := l.ScrollRow
	if s < 0 {
		s = 0
	}
	if m := l.maxScrollRow(); s > m {
		s = m
	}
	return s
}

// ScrollTo moves the top visible row to row, clamped to
// [0, maxScrollRow()], and writes the clamped value back to
// ScrollRow.
func (l *ListBox) ScrollTo(row int) {
	l.ScrollRow = row
	l.ScrollRow = l.clampedScrollRow()
}

// ScrollBy shifts ScrollRow by delta rows (negative scrolls up),
// clamped exactly like ScrollTo.
func (l *ListBox) ScrollBy(delta int) {
	l.ScrollTo(l.ScrollRow + delta)
}

// scrollToSelected nudges ScrollRow so Selected stays within the
// visible window: scrolling up if Selected sits above ScrollRow,
// down if it sits at or past the last visible row. It is a no-op
// when nothing is selected (Selected < 0) so a fresh or
// selection-cleared list is never pulled to a bogus ScrollRow.
//
// ListBox has no built-in keyboard navigation today; this is exposed
// for a host (or a future arrow-key handler) that drives Selected
// externally and wants the list to keep it in view.
func (l *ListBox) scrollToSelected() {
	if l.Selected < 0 {
		return
	}
	if l.Selected < l.ScrollRow {
		l.ScrollTo(l.Selected)
		return
	}
	vr := l.visibleRows()
	if vr <= 0 {
		return
	}
	if l.Selected >= l.ScrollRow+vr {
		l.ScrollTo(l.Selected - vr + 1)
	}
}

// drawScrollbar paints the vertical scrollbar track (always, while
// overflowing) + a proportionally-sized thumb (while there's
// something to scroll) on the right edge of r -- the full widget
// bounds, not the shrunk content rect. Modelled on ScrollView's
// track+thumb proportion math, but driven by ScrollRow (whole rows)
// rather than a pixel offset, since ListBox only ever scrolls by
// full rows.
func (l *ListBox) drawScrollbar(p painter.Painter, theme *Theme, r Rect) {
	trackX := r.X + r.W - scrollbarWidth
	fillRect(p, trackX, r.Y, scrollbarWidth, r.H, theme.SurfaceAlt)

	g, ok := l.scrollbarGeom()
	if !ok {
		return
	}
	fillRect(p, r.X+g.cross0, r.Y+g.thumbStart, scrollbarWidth, g.thumbLen, theme.Accent)
}

// scrollbarGeom returns the vertical scrollbar's widget-local geometry and
// whether the thumb is live (the content overflows the bounds). It is the single
// definition of the thumb shared by drawScrollbar and OnEvent; the track column
// itself is always painted by drawScrollbar while the list overflows. Modelled on
// ScrollView's proportion math but driven by ScrollRow (whole rows).
func (l *ListBox) scrollbarGeom() (sbGeom, bool) {
	r := l.Bounds()
	contentH := len(l.Items) * l.RowHeight
	if r.H <= 0 || contentH <= r.H {
		return sbGeom{}, false
	}
	thumbH := r.H * r.H / contentH
	if thumbH < 8 {
		thumbH = 8
	}
	max := l.maxScrollRow()
	thumbY := 0
	if max > 0 {
		thumbY = l.clampedScrollRow() * (r.H - thumbH) / max
	}
	return sbGeom{
		cross0:     r.W - scrollbarWidth,
		trackStart: 0,
		trackLen:   r.H,
		thumbStart: thumbY,
		thumbLen:   thumbH,
		travelNum:  r.H - thumbH,
		travelDen:  max,
		maxScroll:  max,
	}, true
}

// OnEvent dispatches: EventClick to onClick (selection, unchanged from
// before this feature); EventDragMove/EventDragLeave/EventDrop to the
// drag-to-reorder handlers below, which are all no-ops while Reorderable
// is false, so behavior is byte-identical to a ListBox with no
// drag-to-reorder support when the feature isn't opted into. EventScroll
// (wheel) and the Arrow/Page/Home/End keys scroll the visible window
// natively (see handleScrollKey). Every other event kind is ignored.
func (l *ListBox) OnEvent(ev Event) {
	switch ev.Kind {
	case EventScroll:
		// Native wheel scroll: shift the visible window by Delta rows
		// (ScrollBy clamps at top + bottom).
		l.ScrollBy(ev.Delta)
	case EventKeyDown:
		// Arrow / Page / Home / End scroll the visible window by whole
		// rows or pages; any other key is ignored.
		handleScrollKey(l, ev.Code, l.visibleRows())
	case EventClick:
		// A press on the scrollbar grabs its thumb (or pages the track); it
		// must not also select a row, so consume it before onClick.
		if g, ok := l.scrollbarGeom(); l.sbDrag.press(g, ok, ev, l.visibleRows(), l.ScrollBy) {
			return
		}
		l.onClick(ev)
	case EventMouseDrag:
		g, ok := l.scrollbarGeom()
		l.sbDrag.drag(g, ok, ev, l.ScrollTo)
	case EventMouseUp:
		l.sbDrag.release()
	case EventDragMove:
		l.onDragMove(ev)
	case EventDragLeave:
		l.onDragLeave()
	case EventDrop:
		l.onDrop(ev)
	}
}

// onClick handles a click at (X, Y): it selects the row
// idx = ScrollRow + Y/RowHeight -- Y/RowHeight locates the row within
// the visible window, ScrollRow maps that back to an absolute Items
// index (clamped to the list length); OnActivate fires with that idx.
//
// When MultiSelect is false, a click simply moves Selected to idx --
// unchanged from the widget's original single-selection behaviour,
// and Ctrl/Shift are ignored entirely.
//
// When MultiSelect is true:
//   - a plain click selects ONLY idx (clearing any other selected
//     rows) and moves the anchor (Selected) to idx;
//   - a Ctrl-click toggles idx's membership in the selection set and
//     moves the anchor to idx;
//   - a Shift-click selects the inclusive range between the current
//     anchor (Selected) and idx, replacing the selection set, and
//     leaves the anchor itself unchanged so successive Shift-clicks
//     keep extending/shrinking from the same origin.
//
// Every valid click also records idx as pressedRow (see DragData) --
// unconditionally, regardless of Reorderable, since it costs nothing and
// DragData itself already gates on Reorderable.
// IndexAt returns the Items index under widget-local (x, y), accounting for the
// scroll offset, or -1 for empty space past the last row (or a zero RowHeight).
// Exposed so a host can hit-test a right-click and build a context menu for the
// item under the cursor (x is accepted for signature symmetry; the row is
// determined by y).
func (l *ListBox) IndexAt(x, y int) int {
	if l.RowHeight <= 0 || y < 0 {
		return -1
	}
	idx := l.clampedScrollRow() + y/l.RowHeight
	if idx < 0 || idx >= len(l.Items) {
		return -1
	}
	return idx
}

func (l *ListBox) onClick(ev Event) {
	if l.RowHeight <= 0 {
		return
	}
	if ev.Y < 0 { // Go truncates toward zero -- guard early.
		return
	}
	idx := l.clampedScrollRow() + ev.Y/l.RowHeight
	if idx >= len(l.Items) {
		return
	}
	l.pressedRow = idx

	if l.MultiSelect {
		switch {
		case ev.Shift:
			l.SelectRange(l.Selected, idx)
		case ev.Ctrl:
			l.ToggleSelect(idx)
			l.Selected = idx
		default:
			l.SetSelection(idx)
			l.Selected = idx
		}
	} else {
		l.Selected = idx
	}

	if l.OnActivate != nil {
		l.OnActivate(idx)
	}
}

// rowInsertionIndex maps a widget-local Y to a drag insertion boundary --
// the index in [0, len(Items)] a dropped row would land at if inserted
// there. It locates the row under the pointer via the same
// ScrollRow-relative math as onClick's hit-test, then picks the near edge
// of that row: the boundary above it if the pointer is in its top half,
// the boundary below (row+1) if in its bottom half. That way dragging to
// the middle of the list targets the nearest gap between rows rather than
// always "before".
func (l *ListBox) rowInsertionIndex(y int) int {
	if l.RowHeight <= 0 {
		return 0
	}
	if y < 0 {
		y = 0
	}
	row := y / l.RowHeight
	within := y % l.RowHeight
	idx := l.clampedScrollRow() + row
	if within >= l.RowHeight/2 {
		idx++
	}
	// idx can never be negative here: y and clampedScrollRow() are both
	// >= 0 at this point, so row and idx are too -- only the upper bound
	// needs clamping.
	if idx > len(l.Items) {
		idx = len(l.Items)
	}
	return idx
}

// onDragMove updates dropIndicator to the insertion boundary under ev.Y
// while Reorderable; a no-op otherwise, so dropIndicator (and therefore
// Draw) never changes when the feature isn't opted into.
func (l *ListBox) onDragMove(ev Event) {
	if !l.Reorderable {
		return
	}
	l.dropIndicator = l.rowInsertionIndex(ev.Y)
}

// onDragLeave clears dropIndicator while Reorderable; a no-op otherwise.
func (l *ListBox) onDragLeave() {
	if !l.Reorderable {
		return
	}
	l.dropIndicator = -1
}

// onDrop implements the reorder-drop half of the DnD lifecycle: while
// Reorderable, it parses ev.Code as one of this ListBox's own "listrow:"
// payloads (see DragData/AcceptsDrop), moves that row to the insertion
// boundary under ev.Y, keeps Selected + the multi-select set pointing at
// the same logical rows they pointed at before the move, clears
// dropIndicator, and fires OnReorder with the row's original + final
// index. A non-Reorderable ListBox, a payload AcceptsDrop rejects (a
// foreign scheme, or garbage), or an unparsable/out-of-range source row
// are all silently ignored -- Items is left untouched and OnReorder does
// not fire.
func (l *ListBox) onDrop(ev Event) {
	if !l.Reorderable {
		return
	}
	l.dropIndicator = -1
	if !l.AcceptsDrop(ev.Code) {
		return
	}
	from, err := strconv.Atoi(strings.TrimPrefix(ev.Code, ListRowDragPrefix))
	if err != nil || from < 0 || from >= len(l.Items) {
		return
	}
	to := l.rowInsertionIndex(ev.Y)
	newIdx := l.moveItem(from, to)

	l.Selected = remapReorderedIndex(l.Selected, from, to)
	if l.selected != nil {
		remapped := make(map[int]bool, len(l.selected))
		for i := range l.selected {
			remapped[remapReorderedIndex(i, from, to)] = true
		}
		l.selected = remapped
	}

	if l.OnReorder != nil {
		l.OnReorder(from, newIdx)
	}
}

// moveTargetIndex is the final resting index of a row moved from "from" to
// insertion boundary "to" (both in the pre-move index space -- "to" in
// [0, len(Items)], as produced by rowInsertionIndex). Removing "from"
// shifts every later boundary left by one, so a boundary past "from" lands
// one lower than it reads in the original space.
func moveTargetIndex(from, to int) int {
	if to > from {
		return to - 1
	}
	return to
}

// remapReorderedIndex maps idx -- any row's index BEFORE a move -- to its
// index after moving the row at "from" to boundary "to" (see
// moveTargetIndex). It mirrors the same remove-then-insert shift moveItem
// applies to Items, so callers can keep selection state (Selected, the
// multi-select set) pointing at the same logical rows across a reorder.
func remapReorderedIndex(idx, from, to int) int {
	target := moveTargetIndex(from, to)
	if idx == from {
		return target
	}
	j := idx
	if idx > from {
		j--
	}
	if j >= target {
		j++
	}
	return j
}

// moveItem relocates Items[from] to insertion boundary "to" (see
// rowInsertionIndex), preserving every other row's relative order, and
// returns the moved row's final index (see moveTargetIndex). The caller
// (onDrop) only ever passes a "from" already validated against
// len(Items).
func (l *ListBox) moveItem(from, to int) int {
	if from < 0 || from >= len(l.Items) {
		return from
	}
	target := moveTargetIndex(from, to)

	item := l.Items[from]
	rest := make([]string, 0, len(l.Items)-1)
	rest = append(rest, l.Items[:from]...)
	rest = append(rest, l.Items[from+1:]...)
	if target < 0 {
		target = 0
	}
	if target > len(rest) {
		target = len(rest)
	}

	out := make([]string, 0, len(l.Items))
	out = append(out, rest[:target]...)
	out = append(out, item)
	out = append(out, rest[target:]...)
	l.Items = out
	return target
}

// IsSelected reports whether row i is a member of the multi-selection
// set. It is independent of MultiSelect + Selected, so it can be
// queried (and pre-seeded via SetSelection/ToggleSelect/SelectRange)
// even before multi-selection is switched on.
func (l *ListBox) IsSelected(i int) bool {
	return l.selected[i]
}

// SelectedIndices returns the selected rows in ascending order. The
// returned slice is a fresh copy the caller may mutate freely.
func (l *ListBox) SelectedIndices() []int {
	out := make([]int, 0, len(l.selected))
	for i := range l.selected {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// SetSelection replaces the selection set with exactly the given
// indices. Indices outside [0, len(Items)) are silently dropped.
func (l *ListBox) SetSelection(indices ...int) {
	set := make(map[int]bool, len(indices))
	for _, i := range indices {
		if i < 0 || i >= len(l.Items) {
			continue
		}
		set[i] = true
	}
	l.selected = set
}

// ClearSelection empties the selection set. Selected (the
// anchor/cursor row) is left untouched.
func (l *ListBox) ClearSelection() {
	l.selected = nil
}

// ToggleSelect flips row i's membership in the selection set.
// Out-of-range indices are a no-op.
func (l *ListBox) ToggleSelect(i int) {
	if i < 0 || i >= len(l.Items) {
		return
	}
	if l.selected == nil {
		l.selected = make(map[int]bool)
	}
	if l.selected[i] {
		delete(l.selected, i)
	} else {
		l.selected[i] = true
	}
}

// SelectRange selects the inclusive range of rows between a and b
// (either order accepted), replacing the current selection set. The
// range is clamped to [0, len(Items)); if the list is empty, or the
// clamped range is inverted, the resulting selection is empty.
func (l *ListBox) SelectRange(a, b int) {
	if a > b {
		a, b = b, a
	}
	if a < 0 {
		a = 0
	}
	if b >= len(l.Items) {
		b = len(l.Items) - 1
	}
	set := make(map[int]bool)
	for i := a; i <= b; i++ {
		set[i] = true
	}
	l.selected = set
}
