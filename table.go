// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

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
// use, so the visual reads consistent across widgets).
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
	// zebra stripe pattern paints unmodified.
	Selected int

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
		Columns:    cols,
		Rows:       rows,
		Selected:   -1,
		SortColumn: -1,
	}
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
	widths := t.columnWidths(r.W)

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
	for i, row := range t.Rows {
		y := bodyY + i*TableRowHeight
		bg := theme.Surface
		ink := theme.OnSurface
		switch {
		case i == selRow:
			bg = theme.Accent
			ink = onAccent
		case i%2 == 1:
			// Zebra: row 0 -> Surface, row 1 -> Background, ...
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

	// --- Column separators ----------------------------------------
	// One 1-px vertical stroke between adjacent columns, spanning the
	// full widget height (header + body). No stroke on the outer left
	// or right edge -- the widget's parent frame owns those.
	sepX := r.X
	for i := 0; i < len(t.Columns)-1; i++ {
		sepX += widths[i]
		fillRect(p, sepX, r.Y, 1, r.H, theme.Border)
	}
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
	widths := t.columnWidths(t.Bounds().W)
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
	widths := t.columnWidths(t.Bounds().W)
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

// OnEvent implements header-click sorting + separator drag-resize.
// The toolkit's event model is click-only (see Paned): a resize drag
// begins on an EventClick that lands on a separator (ColumnSeparatorAt),
// is driven tick-by-tick by EventMouseDrag while the button stays down,
// and ends on EventMouseUp -- the same grab/move/release state machine
// RangeSlider uses for its thumbs. A click that lands on a header cell
// instead of a separator sorts that column (if Sortable); a click
// below the header row is ignored (the Table has no other interactive
// surface).
func (t *Table) OnEvent(ev Event) {
	switch ev.Kind {
	case EventClick:
		if ev.Y < 0 || ev.Y >= TableHeaderHeight {
			return
		}
		if sep := t.ColumnSeparatorAt(ev.X); sep >= 0 {
			t.resizing = true
			t.resizingCol = sep
			return
		}
		col := t.columnAt(ev.X)
		if col >= 0 && col < len(t.Columns) && t.Columns[col].Sortable {
			t.toggleSort(col)
		}
	case EventMouseDrag:
		if !t.resizing {
			return
		}
		widths := t.columnWidths(t.Bounds().W)
		left := 0
		for i := 0; i < t.resizingCol; i++ {
			left += widths[i]
		}
		t.SetColumnWidth(t.resizingCol, ev.X-left)
	case EventMouseUp:
		t.resizing = false
	}
}
