// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"sort"
	"strconv"
	"strings"

	"github.com/go-widgets/painter"
)

// Table renders a structured data grid: a fixed header row of column
// titles above a body of text rows. The widget is the missing piece
// vs GTK's ColumnView + DaisyUI's Table -- the toolkit's ListBox +
// TreeView give a single column of items, whereas Table lays cells
// out horizontally under labelled columns.
//
// Visual (per row):
//
//	+----------------+--------+-------------+
//	| Header A       | Hdr B  | Header C    |  <- TableHeaderHeight, SurfaceAlt
//	+----------------+--------+-------------+
//	| row 0 cell 0   | 0.1    | 0.2         |  <- TableRowHeight, Surface
//	| row 1 cell 0   | 1.1    | 1.2         |  <- TableRowHeight, Background
//	| ...
//	+----------------+--------+-------------+
//
// Selected row (if 0 <= Selected < len(Rows)) paints in Theme.Accent
// with the accent-inverted ink -- theme.Extra["OnAccent"] when the
// GTK loader supplied one, otherwise theme.Background (the same
// fallback the Button + ListBox + TreeView selected states already
// use, so the visual reads consistent across widgets). When
// MultiSelect is true every row in the multi-row selection set paints
// the same way, not just Selected (which keeps acting as the anchor
// for Shift-range clicks) -- see MultiSelect + SelectedRows.
//
// The widget is content-only: it never reorders Rows itself. Header
// clicks + separator drags are surfaced through OnSort/OnColumnResize
// so the host (which owns the data model) can re-sort Rows or persist
// a new column width, then hand the Table back its updated state.
type Table struct {
	Base
	// Columns are the header cells (title + optional pixel width).
	// A zero Width means "auto" -- the column claims an equal share of
	// whatever pixel budget is left after the fixed-Width columns.
	Columns []TableColumn
	// Rows is the body content. Each inner slice SHOULD have
	// len == len(Columns); rows shorter than that render only the
	// cells they carry (missing trailing cells are drawn as blank
	// space, the row background still paints edge-to-edge).
	Rows [][]string
	// Selected is the 0-indexed row highlighted with Theme.Accent;
	// -1 (or any out-of-range value) means "no selection" and the
	// zebra stripe pattern paints unmodified. While MultiSelect is
	// true, Selected doubles as the anchor a Shift-click ranges from;
	// it is still the ONLY row painted while MultiSelect is false.
	Selected int

	// MultiSelect switches body-row clicks (handled by OnEvent) from
	// inert to selection-driving: a plain click selects only that row
	// (clearing any other selection, moving the Selected anchor to
	// it); a Ctrl-click toggles that row's membership without
	// disturbing the anchor; a Shift-click selects the inclusive
	// range between the anchor (Selected) and the clicked row,
	// likewise leaving the anchor in place so repeated Shift-clicks
	// keep ranging from the same origin. Header-row clicks (sort) and
	// separator drags (resize) are unaffected either way.
	//
	// The zero value (false) is the original passive-viewer
	// behaviour: OnEvent never touches Selected or any selection
	// state for a body-row click, and Draw highlights only Selected --
	// byte-for-byte the same as before this field existed.
	MultiSelect bool

	// selectedRows is the multi-row selection set. A nil map means
	// "nothing selected", mirroring how Selected == -1 means no
	// single-row anchor. It is consulted by Draw/IsRowSelected only
	// while MultiSelect is true, but the SetRowSelection /
	// ToggleRowSelect / SelectRowRange / ClearRowSelection API works
	// regardless -- a host may pre-seed a selection before switching
	// MultiSelect on.
	selectedRows map[int]bool

	// ScrollRow is the 0-indexed body row currently painted at the top
	// of the body (the header itself never scrolls). Draw + rowAt both
	// read it through clampScrollRow, so an out-of-range value set
	// directly (or left stale after Rows shrinks) never windows past
	// [0, maxScrollRow()] -- the same defensive-collapse idiom Selected
	// and SortColumn already use. The zero value (0) is the original,
	// pre-feature behaviour: the body starts at row 0, and if every row
	// fits within Bounds().H, Draw renders byte-identically to before
	// this field existed (no scrollbar, no windowing). Use ScrollTo /
	// ScrollBy / scrollToSelected to move it -- they keep the field
	// itself clamped, unlike a raw assignment.
	ScrollRow int

	// SortColumn is the 0-indexed column currently sorted, or -1 (or
	// any out-of-range value) for "no sort" -- Draw skips the ▲/▼
	// indicator and OnEvent treats every header click as a fresh sort.
	// The Table never reorders Rows itself; SortColumn/SortAsc only
	// drive the indicator glyph, matching how Selected only drives the
	// accent highlight.
	SortColumn int
	// SortAsc is the direction of SortColumn: true draws ▲ (ascending),
	// false draws ▼ (descending). Meaningless while SortColumn is out
	// of range.
	SortAsc bool
	// OnSort fires when a Sortable header cell is clicked. col is the
	// clicked column; ascending is the NEW direction after the click
	// (clicking the already-active column toggles it, clicking a new
	// column resets to ascending). The Table updates SortColumn/SortAsc
	// itself before firing so the very next Draw shows the indicator;
	// the host is responsible for re-sorting Rows and handing them back.
	OnSort func(col int, ascending bool)

	// OnColumnResize fires whenever a separator drag (or a direct
	// SetColumnWidth call) changes a column's width. newWidth is the
	// clamped pixel width now in effect.
	OnColumnResize func(col, newWidth int)

	// Reorderable opts the Table into drag-to-reorder BODY rows: it makes
	// the Table both a DragSource (a press on a body row becomes a
	// draggable "tablerow:<index>" payload -- see DragData) and a
	// DropTarget for that same payload (see AcceptsDrop). The zero value
	// (false) is the original, pre-feature behaviour: DragData always
	// returns "", AcceptsDrop always returns false, and every drag event
	// (EventDragMove / EventDragLeave / EventDrop) is a no-op -- Draw and
	// OnEvent render/behave byte-identically to before this field
	// existed. Header-cell sort clicks and separator-drag resizes never
	// start a row drag regardless of this flag -- only a press that lands
	// on a BODY row does.
	Reorderable bool

	// OnReorder fires after a successful drop reorders Rows in place:
	// from is the row's index BEFORE the move, to is where it now sits.
	// Nil-guarded -- a host that doesn't care to be notified simply
	// leaves it unset.
	OnReorder func(from, to int)

	// dragRow is the body row pressed via the most recent body-row
	// EventClick, recorded only while Reorderable is true (a header or
	// separator press never touches it). It is what DragData reports as
	// the drag payload. -1 ("nothing pressed yet") mirrors how Selected
	// == -1 means "no selection" -- NewTable seeds it, and DragData
	// defensively collapses any out-of-range value to "" the same way
	// Draw collapses an out-of-range Selected to "no highlight".
	dragRow int

	// dropIndicator is the insertion index (0..len(Rows), inclusive --
	// len(Rows) means "after the last row") the in-progress drag would
	// land on, or -1 for "no drag in progress" -- driven by
	// EventDragMove / EventDragLeave and painted by Draw as a thin line
	// between body rows. NewTable seeds it to -1; only consulted while
	// Reorderable is true.
	dropIndicator int

	// resizing + resizingCol track an in-progress separator drag started
	// by a header-row EventClick on a separator hit (see
	// ColumnSeparatorAt) and cleared on EventMouseUp. resizing is a
	// separate bool (rather than resizingCol == -1) so the zero value of
	// a directly-constructed Table{} -- resizingCol == 0 -- can never be
	// mistaken for "dragging separator 0".
	resizing    bool
	resizingCol int
}

// TableColumn is one column definition: a header title + an optional
// fixed pixel Width. A Width of 0 marks the column as "auto" -- its
// width is computed at Draw time by evenly dividing the remaining
// pixel budget among all auto columns.
type TableColumn struct {
	Title string
	Width int // pixels; 0 = auto (equal share of remaining space)
	// Align controls horizontal placement of BOTH the header title and
	// every body cell in this column. The zero value (AlignLeft) keeps
	// the original left-justified behaviour; AlignRight is the natural
	// choice for numeric columns, AlignCenter for short status flags.
	Align Align
	// Sortable opts this column into header-click sorting. The zero
	// value (false) makes a header click a no-op, so existing callers
	// that never set it keep the original passive-viewer behaviour.
	Sortable bool
}

// TableHeaderHeight is the pixel height of the header row.
const TableHeaderHeight = 24

// TableRowHeight is the pixel height of one body row.
const TableRowHeight = 22

// TableCellPadX is the left/right pixel padding applied inside every
// header + body cell before its text lands.
const TableCellPadX = 4

// tableEmptyPlaceholder is the label rendered under the header when
// Rows is empty. Split into a constant so tests can assert width
// without hard-coding the string in two places.
const tableEmptyPlaceholder = "(no data)"

// NewTable builds a Table with the given columns + rows. Selected
// starts at -1 (no row selected) so a freshly constructed Table
// renders with plain zebra striping.
func NewTable(cols []TableColumn, rows [][]string) *Table {
	return &Table{
		Columns:       cols,
		Rows:          rows,
		Selected:      -1,
		SortColumn:    -1,
		dragRow:       -1,
		dropIndicator: -1,
	}
}

// Table is both a DragSource and a DropTarget for its own body rows --
// see Reorderable, DragData + AcceptsDrop.
var (
	_ DragSource = (*Table)(nil)
	_ DropTarget = (*Table)(nil)
)

// tableRowDragPrefix scopes a Table's drag-to-reorder payload to a
// private scheme so a Table never mistakes a foreign drag (e.g. a
// DropZone file path) for one of its own rows, and vice versa.
const tableRowDragPrefix = "tablerow:"

// DragData reports the drag payload for the body row currently pressed
// (see dragRow) -- "tablerow:<index>" -- while Reorderable is true and
// a body row was actually the most recent press; otherwise "". A stale
// dragRow left over after Rows shrinks collapses to "" the same
// defensive way Draw collapses an out-of-range Selected.
func (t *Table) DragData() string {
	if !t.Reorderable || t.dragRow < 0 || t.dragRow >= len(t.Rows) {
		return ""
	}
	return tableRowDragPrefix + strconv.Itoa(t.dragRow)
}

// AcceptsDrop reports whether payload is one of this Table's own
// "tablerow:" drags -- true only while Reorderable is true AND the
// payload parses as a well-formed tablerow payload. A foreign payload
// (a different scheme, or garbage) is always rejected, including while
// Reorderable is false.
func (t *Table) AcceptsDrop(payload string) bool {
	if !t.Reorderable {
		return false
	}
	_, ok := parseTableRowDragPayload(payload)
	return ok
}

// parseTableRowDragPayload extracts the source row index from a
// "tablerow:<index>" payload. ok is false for any payload that isn't
// that exact scheme or whose index isn't a non-negative integer -- the
// single choke point EventDrop + AcceptsDrop both use so a garbage or
// foreign payload is rejected identically by both.
func parseTableRowDragPayload(payload string) (int, bool) {
	rest, ok := strings.CutPrefix(payload, tableRowDragPrefix)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// Draw paints the header + body + column separators through p using
// theme's palette. Widths for auto columns are computed here, so
// resizing the widget's Bounds() between frames re-flows the columns
// automatically.
func (t *Table) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	overflow := t.bodyOverflows()
	widths := t.columnWidths(t.contentWidth())

	// --- Header row ------------------------------------------------
	fillRect(p, r.X, r.Y, r.W, TableHeaderHeight, theme.SurfaceAlt)
	// 1-px bottom-edge stroke separates the header from the body.
	fillRect(p, r.X, r.Y+TableHeaderHeight-1, r.W, 1, theme.Border)
	// Header cell titles. sortCol collapses an out-of-range SortColumn
	// to -1, mirroring how Draw resolves Selected below -- an
	// unconstructed or stale Table never crashes drawing an indicator
	// on a column that no longer exists.
	sortCol := -1
	if t.SortColumn >= 0 && t.SortColumn < len(t.Columns) {
		sortCol = t.SortColumn
	}
	hx := r.X
	hty := r.Y + (TableHeaderHeight-t.glyphHeight())/2
	for i, col := range t.Columns {
		t.drawText(p, cellTextX(&t.Base, hx, widths[i], col.Title, col.Align), hty, col.Title, theme.OnBackground)
		if i == sortCol {
			drawSortIndicator(p, hx+widths[i]-8, r.Y+TableHeaderHeight/2, t.SortAsc, theme.OnBackground)
		}
		hx += widths[i]
	}

	// --- Body ------------------------------------------------------
	bodyY := r.Y + TableHeaderHeight
	if len(t.Rows) == 0 {
		// "(no data)" centred horizontally within the widget, sitting
		// one TableRowHeight below the header.
		tw := t.textWidth(tableEmptyPlaceholder)
		tx := r.X + (r.W-tw)/2
		ty := bodyY + (TableRowHeight-t.glyphHeight())/2
		t.drawText(p, tx, ty, tableEmptyPlaceholder, theme.OnSurface)
		return
	}
	// Resolve which body row is highlighted -- Selected out of range
	// collapses to -1 so the loop below never enters the accent branch
	// for a bogus index.
	selRow := -1
	if t.Selected >= 0 && t.Selected < len(t.Rows) {
		selRow = t.Selected
	}
	onAccent := accentInk(theme)
	// scroll is the top visible row, defensively re-collapsed from
	// ScrollRow exactly like selRow/sortCol above. end is one past the
	// last row the current body height can show; when every row fits
	// (the common, pre-feature case) end == len(t.Rows) and scroll == 0,
	// so this loop paints identically to the un-windowed original.
	scroll := t.clampScrollRow()
	end := scroll + t.bodyVisibleRows()
	if end > len(t.Rows) {
		end = len(t.Rows)
	}
	// Clip the body to its own rect before painting rows. bodyVisibleRows
	// rounds UP, so when Bounds().H isn't an exact multiple of
	// TableRowHeight the last windowed row is only partially inside the
	// widget -- without a clip its background/text would spill past
	// r.Y+r.H onto whatever the host draws below this widget, and (once
	// the scrollbar is painted afterwards) would already have overdrawn
	// where the thumb belongs. Back-ends that can't clip just render
	// unclipped, the same graceful degradation ScrollView already relies
	// on for its child.
	clr, canClip := p.(painter.Clipper)
	if canClip {
		clr.PushClip(Rect{X: r.X, Y: bodyY, W: r.W, H: r.Y + r.H - bodyY})
	}
	for i := scroll; i < end; i++ {
		row := t.Rows[i]
		y := bodyY + (i-scroll)*TableRowHeight
		bg := theme.Surface
		ink := theme.OnSurface
		// Highlighted covers both the single-row anchor (always) and,
		// while MultiSelect is on, any row in the multi-row selection
		// set. With MultiSelect false the second half short-circuits,
		// so this collapses to exactly the pre-MultiSelect condition.
		highlighted := i == selRow || (t.MultiSelect && t.IsRowSelected(i))
		switch {
		case highlighted:
			bg = theme.Accent
			ink = onAccent
		case i%2 == 1:
			// Zebra keyed on the row's ABSOLUTE index i (not its
			// on-screen position) so scrolling never shifts which rows
			// read as odd/even.
			bg = theme.Background
		}
		fillRect(p, r.X, y, r.W, TableRowHeight, bg)
		cx := r.X
		cty := y + (TableRowHeight-t.glyphHeight())/2
		for j, col := range t.Columns {
			if j < len(row) {
				t.drawText(p, cellTextX(&t.Base, cx, widths[j], row[j], col.Align), cty, row[j], ink)
			}
			cx += widths[j]
		}
	}
	// Drag-to-reorder insertion indicator: a thin Accent-coloured line
	// between body rows at dropIndicator, painted only while a
	// Reorderable drag is actually in progress (dropIndicator >= 0 --
	// EventDragLeave / a fresh drop reset it to -1, and NewTable seeds
	// it there too). Drawn inside the same clip as the body rows so it
	// never spills past Bounds() any more than a row itself would.
	if t.Reorderable && t.dropIndicator >= 0 && t.dropIndicator <= len(t.Rows) {
		iy := bodyY + (t.dropIndicator-scroll)*TableRowHeight
		fillRect(p, r.X, iy-1, r.W, 2, theme.Accent)
	}
	if canClip {
		clr.PopClip()
	}

	// --- Column separators ----------------------------------------
	// One 1-px vertical stroke between adjacent columns, spanning the
	// full widget height (header + body). No stroke on the outer left
	// or right edge -- the widget's parent frame owns those.
	sepX := r.X
	for i := 0; i < len(t.Columns)-1; i++ {
		sepX += widths[i]
		fillRect(p, sepX, r.Y, 1, r.H, theme.Border)
	}

	// --- Vertical scrollbar (right edge, body only) -----------------
	// Only drawn while the body actually overflows -- with all rows
	// fitting (the byte-identical case) this is skipped entirely, same
	// as the pre-feature Table never painting one.
	if overflow {
		t.drawScrollbar(p, theme, r, bodyY, scroll)
	}
}

// tableScrollbarThumbMin is the pixel floor a scrollbar thumb is
// clamped to, mirroring ScrollView's own floor so a huge row count
// never shrinks the thumb into an unclickable sliver.
const tableScrollbarThumbMin = 8

// drawScrollbar paints the right-edge scrollbar track + thumb over the
// body rows -- the header sits above it and never scrolls, so the
// track spans only [bodyY, r.Y+r.H). It reuses ScrollView.Draw's exact
// pixel-proportion formula for the vertical thumb, substituting "rows
// converted to pixels" (len(t.Rows)*TableRowHeight, scroll*TableRowHeight)
// for ScrollView's arbitrary child content height/offset. Only called
// by Draw while bodyOverflows() is true, which guarantees trackH > 0
// and contentH > trackH (see bodyOverflows), so no defensive
// zero/negative-denominator guard is needed here.
func (t *Table) drawScrollbar(p painter.Painter, theme *Theme, r Rect, bodyY, scroll int) {
	trackX := r.X + r.W - scrollbarWidth
	trackH := r.Y + r.H - bodyY
	fillRect(p, trackX, bodyY, scrollbarWidth, trackH, theme.SurfaceAlt)
	contentH := len(t.Rows) * TableRowHeight
	thumbH := trackH * trackH / contentH
	if thumbH < tableScrollbarThumbMin {
		thumbH = tableScrollbarThumbMin
	}
	// maxScrollRow() is guaranteed > 0 here (bodyOverflows() already
	// established len(t.Rows) > bodyVisibleRows(), so the raw
	// len(t.Rows)-bodyVisibleRows() difference maxScrollRow clamps is
	// already positive) -- no divide-by-zero guard needed on that front,
	// and contentH > trackH (established above) keeps the denominator
	// below positive too.
	thumbY := bodyY + scroll*TableRowHeight*(trackH-thumbH)/(contentH-trackH)
	fillRect(p, trackX, thumbY, scrollbarWidth, thumbH, theme.Accent)
}

// columnWidths distributes the total pixel budget across every column.
// Fixed-Width columns take exactly their declared width; auto
// (Width == 0) columns split the remainder equally, with any integer
// remainder pushed onto the last auto column so all widths sum to
// total. If there are no auto columns the widths returned are simply
// the declared Widths -- they may exceed or fall short of total, but
// the painter's clipping keeps that safe.
func (t *Table) columnWidths(total int) []int {
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
	// Push integer-division leftover onto the last auto column so
	// the sum of widths equals total (only reachable when there is
	// budget left over after the fixed columns).
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

// cellTextX returns the x at which to start drawing text of the given
// string inside a cell whose left edge is cellX and whose width is
// cellW, honouring align. TableCellPadX is reserved on both the left
// (AlignLeft) and right (AlignRight) edges; AlignCenter ignores the
// padding and centres within the full cell. The result is clamped to
// the left padding so text never starts before the cell's inner edge.
//
// b supplies the effective font so right/centre alignment measures the
// text in the Table's own font (b.textWidth) rather than the global one.
func cellTextX(b *Base, cellX, cellW int, text string, align Align) int {
	switch align {
	case AlignRight:
		x := cellX + cellW - TableCellPadX - b.textWidth(text)
		if min := cellX + TableCellPadX; x < min {
			x = min
		}
		return x
	case AlignCenter:
		x := cellX + (cellW-b.textWidth(text))/2
		if min := cellX + TableCellPadX; x < min {
			x = min
		}
		return x
	default: // AlignLeft
		return cellX + TableCellPadX
	}
}

// accentInk returns the ink colour to draw ON a Theme.Accent field.
// The GTK loader may populate theme.Extra["OnAccent"] with the
// theme's canonical accent-inverted colour; if absent we fall back
// to theme.Background, matching what Button + ListBox + TreeView
// already do for their selected/pressed accent branches.
func accentInk(theme *Theme) RGBA {
	if theme.Extra != nil {
		if c, ok := theme.Extra["OnAccent"]; ok {
			return c
		}
	}
	return theme.Background
}

// drawSortIndicator paints a small 5-px-tall ▲ (ascending) / ▼
// (descending) triangle centred on (cx, cy), using the same
// row-by-row fillRect technique as Expander/Accordion's disclosure
// chevron so the glyph reads consistently across the toolkit.
func drawSortIndicator(p painter.Painter, cx, cy int, ascending bool, ink RGBA) {
	if ascending {
		// ▲ : narrow tip at the top, widening to the flat bottom row.
		for t := 0; t < 5; t++ {
			fillRect(p, cx-t, cy-2+t, 1+2*t, 1, ink)
		}
		return
	}
	// ▼ : flat top (widest row), point at bottom (narrow tip) --
	// identical shape to drawDisclosureChevron's expanded state.
	for t := 0; t < 5; t++ {
		fillRect(p, cx-t, cy+2-t, 1+2*t, 1, ink)
	}
}

// tableMinColumnWidth is the floor SetColumnWidth clamps to -- small
// enough to still show a sliver of a cell, large enough that a column
// can never be dragged into (or past) zero width.
const tableMinColumnWidth = 20

// tableSeparatorHitTolerance is the +/- pixel band around a column
// separator's exact x that still counts as a hit, mirroring the
// forgiving hit-test every pointer-driven drag handle needs (an exact
// 1-px target is unusable with a mouse).
const tableSeparatorHitTolerance = 3

// columnAt returns the index of the column whose cell spans localX (a
// Table-local x coordinate), or -1 if localX falls outside every
// column -- e.g. an empty Columns slice or a localX past the last
// column's right edge when the fixed-Width columns overflow the
// widget's Bounds().
func (t *Table) columnAt(localX int) int {
	widths := t.columnWidths(t.contentWidth())
	x := 0
	for i, w := range widths {
		if localX >= x && localX < x+w {
			return i
		}
		x += w
	}
	return -1
}

// ColumnSeparatorAt returns the 0-based index of the separator under
// localX (a Table-local x coordinate) -- the separator between column
// i and column i+1 -- within tableSeparatorHitTolerance pixels, or -1
// if localX is not near any separator. A single-column (or empty)
// Table has no separators and always returns -1.
func (t *Table) ColumnSeparatorAt(localX int) int {
	if len(t.Columns) < 2 {
		return -1
	}
	widths := t.columnWidths(t.contentWidth())
	x := 0
	for i := 0; i < len(widths)-1; i++ {
		x += widths[i]
		if localX >= x-tableSeparatorHitTolerance && localX <= x+tableSeparatorHitTolerance {
			return i
		}
	}
	return -1
}

// SetColumnWidth pins column col to a fixed pixel width w (clamped to
// tableMinColumnWidth), then fires OnColumnResize with the clamped
// value. Like a Paned's MoveHandle, this is the direct, host-callable
// entry point a drag handler (internal or external) drives; an
// out-of-range col is a no-op. Setting a width converts an "auto"
// column into a fixed one, exactly as dragging a Paned's handle turns
// its 50/50 default into an explicit Position.
func (t *Table) SetColumnWidth(col, w int) {
	if col < 0 || col >= len(t.Columns) {
		return
	}
	if w < tableMinColumnWidth {
		w = tableMinColumnWidth
	}
	t.Columns[col].Width = w
	if t.OnColumnResize != nil {
		t.OnColumnResize(col, w)
	}
}

// bodyVisibleRows is how many body-row slots the widget's current
// Bounds().H offers below the fixed header, rounded UP -- the last
// slot may show only a partial row rather than leave a gap when the
// height isn't an exact multiple of TableRowHeight. Draw clips the
// body to Bounds() so that partial row never paints past the widget's
// own edge (see the PushClip call in Draw).
func (t *Table) bodyVisibleRows() int {
	h := t.Bounds().H - TableHeaderHeight
	if h <= 0 {
		return 0
	}
	return (h + TableRowHeight - 1) / TableRowHeight
}

// bodyOverflows reports whether Rows holds more entries than fit in
// the body at the widget's current height -- the single condition
// that gates the right-edge scrollbar's presence, and the column
// width reservation contentWidth carves out for it, in Draw. A body
// with no visible row capacity at all (Bounds().H shorter than the
// header) never "overflows" -- there is no body pixel row for a
// scrollbar track to occupy.
func (t *Table) bodyOverflows() bool {
	vis := t.bodyVisibleRows()
	return vis > 0 && len(t.Rows) > vis
}

// maxScrollRow is the highest legal ScrollRow: enough rows short of
// the end that the body still shows a full window of bodyVisibleRows
// rows, or 0 once Rows no longer overflows (including the empty-Rows
// case) -- exactly the "[0, max(0, len(Rows)-visibleRows)]" range
// ScrollRow's doc promises.
func (t *Table) maxScrollRow() int {
	max := len(t.Rows) - t.bodyVisibleRows()
	if max < 0 {
		max = 0
	}
	return max
}

// clampScrollRow returns ScrollRow collapsed into [0, maxScrollRow()]
// -- the same "an out-of-range field never crashes Draw" idiom
// Selected + SortColumn already use, applied read-only here so a
// stale or directly-assigned ScrollRow (e.g. left over after Rows
// shrinks) never windows past the valid row range. It does NOT mutate
// t.ScrollRow; ScrollTo/ScrollBy/scrollToSelected are the API that
// keeps the field itself clamped going forward.
func (t *Table) clampScrollRow() int {
	s := t.ScrollRow
	if s < 0 {
		s = 0
	}
	if max := t.maxScrollRow(); s > max {
		s = max
	}
	return s
}

// contentWidth is the pixel budget columnWidths distributes for both
// the header + body: the widget's full width, minus a scrollbarWidth
// reservation on the right edge while the body overflows vertically.
// Draw, columnAt, ColumnSeparatorAt and the resize-drag branch of
// OnEvent all resolve column geometry through this one spot so the
// header, body, hit-testing and the scrollbar always agree on where
// every column + separator sits.
func (t *Table) contentWidth() int {
	w := t.Bounds().W
	if t.bodyOverflows() {
		w -= scrollbarWidth
		if w < 0 {
			w = 0
		}
	}
	return w
}

// ScrollTo sets ScrollRow to row, clamped into [0, maxScrollRow()] --
// the direct, host-callable entry point a scrollbar drag or a
// PageUp/PageDown key handler drives, mirroring how SetColumnWidth is
// the direct entry point a separator drag drives.
func (t *Table) ScrollTo(row int) {
	t.ScrollRow = row
	t.ScrollRow = t.clampScrollRow()
}

// ScrollBy adjusts ScrollRow by delta rows (positive scrolls down,
// negative scrolls up), clamped the same way as ScrollTo. A mouse
// wheel or arrow-key handler calls this directly.
func (t *Table) ScrollBy(delta int) {
	t.ScrollTo(t.ScrollRow + delta)
}

// scrollToSelected nudges ScrollRow just far enough to bring Selected
// back into the visible window: up if Selected sits above ScrollRow,
// down if it sits at or past the bottom of the window, otherwise
// ScrollRow is left untouched. Selected < 0 ("no selection") is a
// guarded no-op -- without it the arithmetic below would compute a
// bogus target from a -1 row index, the same off-by-one that panicked
// the tui Table's equivalent helper with "index out of range [-1]".
func (t *Table) scrollToSelected() {
	if t.Selected < 0 {
		return
	}
	vis := t.bodyVisibleRows()
	if vis <= 0 {
		return
	}
	switch {
	case t.Selected < t.ScrollRow:
		t.ScrollTo(t.Selected)
	case t.Selected >= t.ScrollRow+vis:
		t.ScrollTo(t.Selected - vis + 1)
	}
}

// IsRowSelected reports whether row i is a member of the multi-row
// selection set. A negative i is always false -- mirrors how every
// other row/column index in this file collapses an invalid value
// instead of indexing into (or panicking on) the underlying map/slice.
// It answers from the raw set regardless of MultiSelect; only Draw and
// OnEvent gate their use of it on MultiSelect being true.
func (t *Table) IsRowSelected(i int) bool {
	if i < 0 {
		return false
	}
	return t.selectedRows[i]
}

// SelectedRows returns every selected row index in ascending order, or
// nil if nothing is selected. The slice is a fresh copy -- mutating it
// has no effect on the Table's selection state.
func (t *Table) SelectedRows() []int {
	if len(t.selectedRows) == 0 {
		return nil
	}
	out := make([]int, 0, len(t.selectedRows))
	for i := range t.selectedRows {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// SetRowSelection replaces the current selection with exactly rows.
// Negative entries are dropped; calling it with no arguments (or with
// only negative ones) clears the selection, same end state as
// ClearRowSelection.
func (t *Table) SetRowSelection(rows ...int) {
	sel := make(map[int]bool, len(rows))
	for _, r := range rows {
		if r >= 0 {
			sel[r] = true
		}
	}
	t.selectedRows = sel
}

// ClearRowSelection empties the multi-row selection set.
func (t *Table) ClearRowSelection() {
	t.selectedRows = nil
}

// ToggleRowSelect flips row i's membership in the selection set --
// selecting it if absent, deselecting it if present. A negative i is a
// no-op.
func (t *Table) ToggleRowSelect(i int) {
	if i < 0 {
		return
	}
	if t.selectedRows == nil {
		t.selectedRows = make(map[int]bool)
	}
	if t.selectedRows[i] {
		delete(t.selectedRows, i)
	} else {
		t.selectedRows[i] = true
	}
}

// SelectRowRange replaces the selection with the inclusive range
// between a and b -- callers may pass either endpoint first, matching
// how a Shift-click can land above OR below the anchor. A negative
// endpoint clamps to 0 (so an anchor of -1, "nothing selected yet",
// still yields a sane from-the-top range instead of an empty one); if
// both endpoints are negative the resulting selection is empty.
func (t *Table) SelectRowRange(a, b int) {
	if a > b {
		a, b = b, a
	}
	if a < 0 {
		a = 0
	}
	sel := make(map[int]bool)
	for i := a; i <= b; i++ {
		sel[i] = true
	}
	t.selectedRows = sel
}

// rowAt returns the body row index whose vertical band contains
// localY (a Table-local y coordinate, i.e. relative to the widget's
// own top edge and therefore still including the header offset), or
// -1 if localY lands in/above the header or at/past the last row --
// the same "collapse to -1 outside the valid range" idiom columnAt
// and ColumnSeparatorAt already use for x coordinates. The offset
// within the body is added to clampScrollRow() (not raw ScrollRow) so
// a click always resolves to whatever row Draw actually painted at
// that y, even with an out-of-range ScrollRow.
func (t *Table) rowAt(localY int) int {
	if localY < TableHeaderHeight {
		return -1
	}
	row := t.clampScrollRow() + (localY-TableHeaderHeight)/TableRowHeight
	if row < 0 || row >= len(t.Rows) {
		return -1
	}
	return row
}

// rowInsertIndexAt returns the drag-to-reorder insertion index for a
// Table-local y coordinate: 0..len(Rows) inclusive, where len(Rows)
// means "after the last row". Unlike rowAt (which answers "which row's
// band contains y" and returns -1 outside the body), this rounds to
// the NEAREST row boundary so the drop indicator can land either above
// or below the row the pointer happens to be over, and clamps into
// range instead of returning a sentinel -- a drag hovering above the
// header or past the last row always resolves to a valid (if
// degenerate) insertion point, exactly like a real drag-and-drop
// indicator would.
func (t *Table) rowInsertIndexAt(localY int) int {
	scroll := t.clampScrollRow()
	if localY < TableHeaderHeight {
		return scroll
	}
	idx := scroll + (localY-TableHeaderHeight+TableRowHeight/2)/TableRowHeight
	if idx > len(t.Rows) {
		idx = len(t.Rows)
	}
	return idx
}

// remapRowIndex answers "row i (in the array BEFORE a single-element
// move) now lives at what index (in the array AFTER the move)", given
// the moved row's original index `from` and its resting index `final`
// (both in the same before/after sense as reorderRow's own from/final).
// It is the shared arithmetic reorderRow uses to keep Selected +
// selectedRows following the moved row instead of silently pointing at
// whatever row slid into their old slot.
func remapRowIndex(i, from, final int) int {
	switch {
	case i == from:
		return final
	case from < final:
		if i > from && i <= final {
			return i - 1
		}
	case from > final:
		if i >= final && i < from {
			return i + 1
		}
	}
	return i
}

// reorderRow moves the row at index from to insertion index to (0..
// len(Rows), pre-move coordinates -- see rowInsertIndexAt), updates
// Selected + selectedRows so the selection follows the moved row, then
// fires OnReorder with from and the row's actual resting index (which
// differs from to by one when the row moved downward past its own
// origin -- removing it first shifts every later index down). An
// out-of-range from is a no-op; to is clamped into [0, len(Rows)]
// first, so a caller can pass a raw rowInsertIndexAt result unchecked.
func (t *Table) reorderRow(from, to int) {
	n := len(t.Rows)
	if from < 0 || from >= n {
		return
	}
	if to < 0 {
		to = 0
	} else if to > n {
		to = n
	}
	final := to
	if to > from {
		final--
	}
	// final is provably within [0, n-1] here: the `to` clamp above keeps
	// to in [0, n], and from is in [0, n-1] (checked above), so both the
	// to<=from branch (final=to<=from<=n-1) and the to>from branch
	// (final=to-1, with from<to<=n so final in [from, n-1]) land inside
	// the post-removal slice's valid insertion range -- no further clamp
	// needed (and none would ever be exercised: n-1 == len(t.Rows) right
	// after the removal below).
	row := t.Rows[from]
	t.Rows = append(t.Rows[:from], t.Rows[from+1:]...)
	t.Rows = append(t.Rows, nil)
	copy(t.Rows[final+1:], t.Rows[final:])
	t.Rows[final] = row

	if t.Selected >= 0 {
		t.Selected = remapRowIndex(t.Selected, from, final)
	}
	if len(t.selectedRows) > 0 {
		remapped := make(map[int]bool, len(t.selectedRows))
		for i := range t.selectedRows {
			remapped[remapRowIndex(i, from, final)] = true
		}
		t.selectedRows = remapped
	}

	if t.OnReorder != nil {
		t.OnReorder(from, final)
	}
}

// toggleSort updates SortColumn/SortAsc for a header click on col --
// re-clicking the active column flips SortAsc, clicking a new column
// resets to ascending -- then fires OnSort with the resulting
// direction. The Table never touches Rows itself; the host re-sorts
// and hands the Table its updated data.
func (t *Table) toggleSort(col int) {
	if t.SortColumn == col {
		t.SortAsc = !t.SortAsc
	} else {
		t.SortColumn = col
		t.SortAsc = true
	}
	if t.OnSort != nil {
		t.OnSort(col, t.SortAsc)
	}
}

// OnEvent implements header-click sorting, separator drag-resize, and
// (while MultiSelect is true) body-row multi-selection. The toolkit's
// event model is click-only (see Paned): a resize drag begins on an
// EventClick that lands on a separator (ColumnSeparatorAt), is driven
// tick-by-tick by EventMouseDrag while the button stays down, and ends
// on EventMouseUp -- the same grab/move/release state machine
// RangeSlider uses for its thumbs. A click that lands on a header cell
// instead of a separator sorts that column (if Sortable).
//
// A click below the header row is a header/sort/resize no-op -- it
// falls through to the body-row branch instead. With MultiSelect
// false that branch is itself a no-op (the original, selection-free
// behaviour); with MultiSelect true a plain click selects only that
// row and moves the Selected anchor to it, Ctrl toggles the row
// without moving the anchor, and Shift selects the inclusive range
// between the anchor and the clicked row (also without moving the
// anchor, so repeated Shift-clicks keep ranging from the same
// origin). A click past the last row (rowAt returns -1) is ignored.
func (t *Table) OnEvent(ev Event) {
	switch ev.Kind {
	case EventClick:
		if ev.Y < 0 {
			return
		}
		if ev.Y < TableHeaderHeight {
			if sep := t.ColumnSeparatorAt(ev.X); sep >= 0 {
				t.resizing = true
				t.resizingCol = sep
				return
			}
			col := t.columnAt(ev.X)
			if col >= 0 && col < len(t.Columns) && t.Columns[col].Sortable {
				t.toggleSort(col)
			}
			return
		}
		row := t.rowAt(ev.Y)
		// A body-row press starts a potential row drag whenever
		// Reorderable is on -- independent of MultiSelect, so
		// drag-to-reorder works on a passive-viewer Table too.
		if t.Reorderable && row >= 0 {
			t.dragRow = row
		}
		if !t.MultiSelect {
			return
		}
		if row < 0 {
			return
		}
		switch {
		case ev.Shift:
			t.SelectRowRange(t.Selected, row)
		case ev.Ctrl:
			t.ToggleRowSelect(row)
		default:
			t.SetRowSelection(row)
			t.Selected = row
		}
	case EventMouseDrag:
		if !t.resizing {
			return
		}
		widths := t.columnWidths(t.contentWidth())
		left := 0
		for i := 0; i < t.resizingCol; i++ {
			left += widths[i]
		}
		t.SetColumnWidth(t.resizingCol, ev.X-left)
	case EventMouseUp:
		t.resizing = false
	case EventDragMove:
		if !t.Reorderable {
			return
		}
		t.dropIndicator = t.rowInsertIndexAt(ev.Y)
	case EventDragLeave:
		t.dropIndicator = -1
	case EventDrop:
		t.dropIndicator = -1
		if !t.Reorderable {
			return
		}
		from, ok := parseTableRowDragPayload(ev.Code)
		if !ok {
			return
		}
		t.reorderRow(from, t.rowInsertIndexAt(ev.Y))
	}
}
