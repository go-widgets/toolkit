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
// The widget is content-only: no per-cell events, no header sorting,
// no column drag-resize. A future revision layered on top may add
// them; the MVP is a passive spreadsheet-shaped viewer.
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
}

// TableColumn is one column definition: a header title + an optional
// fixed pixel Width. A Width of 0 marks the column as "auto" -- its
// width is computed at Draw time by evenly dividing the remaining
// pixel budget among all auto columns.
type TableColumn struct {
	Title string
	Width int // pixels; 0 = auto (equal share of remaining space)
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
		Columns:  cols,
		Rows:     rows,
		Selected: -1,
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
	// Header cell titles.
	hx := r.X
	hty := r.Y + (TableHeaderHeight-GlyphHeight())/2
	for i, col := range t.Columns {
		DrawText(p, hx+TableCellPadX, hty, col.Title, theme.OnBackground)
		hx += widths[i]
	}

	// --- Body ------------------------------------------------------
	bodyY := r.Y + TableHeaderHeight
	if len(t.Rows) == 0 {
		// "(no data)" centred horizontally within the widget, sitting
		// one TableRowHeight below the header.
		tw := TextWidth(tableEmptyPlaceholder)
		tx := r.X + (r.W-tw)/2
		ty := bodyY + (TableRowHeight-GlyphHeight())/2
		DrawText(p, tx, ty, tableEmptyPlaceholder, theme.OnSurface)
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
		cty := y + (TableRowHeight-GlyphHeight())/2
		for j := range t.Columns {
			if j < len(row) {
				DrawText(p, cx+TableCellPadX, cty, row[j], ink)
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
