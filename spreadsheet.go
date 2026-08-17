// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/toolkit/internal/formula"

// Spreadsheet is an A1-addressed formula grid: columns A, B, ..., Z, AA, ...
// across 1-based rows, each cell holding a literal (number or text) or a
// leading-"=" formula. It composes the toolkit's shared grid primitives — the
// same fillRect / strokeRect / cellTextX painting, the stock Entry as the
// inline editor, and the scrollDrag / sbGeom scrollbar machinery ScrollView and
// Table already use — rather than duplicating Table's data-grid rendering.
//
// It is a DISTINCT widget from Table, not a "formula mode" bolted onto it,
// because the two contracts are orthogonal. Table is a data grid: named,
// individually-sized, sortable/groupable columns over Rows [][]string. A
// spreadsheet is uniform A1 cells over a formula engine with a dependency graph
// and recomputation. Forcing Table's Columns/Rows model to also carry cell
// references, formulas and recalculation would bloat exactly the data-grid
// contract that makes Table simple; a separate widget keeps both clean while
// still sharing the low-level painting and scrolling helpers.
//
// The formula engine (parse, evaluate, dependency-ordered recalc, cycle
// detection) lives in internal/formula; the widget is a thin view over a
// formula.Model.
type Spreadsheet struct {
	Base
	focusState

	model                *formula.Model
	cur                  formula.Ref // the active (selected) cell
	scrollCol, scrollRow int         // top-left visible cell, in cell units

	// editor is the inline Entry while a cell edit is open (nil otherwise).
	editor *Entry

	// OnCellChange, when set, fires after a committed edit with the cell and the
	// raw text that was stored — the seam a view model observes.
	OnCellChange func(ref formula.Ref, raw string)

	// sbV and sbH track an in-progress drag of the vertical / horizontal
	// scrollbar thumb respectively.
	sbV, sbH scrollDrag
}

// Spreadsheet cell + band metrics, in LOGICAL pixels (scaled to device pixels
// through scaled() at use, exactly like Table's TableRowHeight etc.).
const (
	// SpreadsheetColWidth is the uniform width of a data column.
	SpreadsheetColWidth = 64
	// SpreadsheetRowHeight is the uniform height of a data row.
	SpreadsheetRowHeight = 20
	// SpreadsheetHeaderHeight is the height of the column-letter header band.
	SpreadsheetHeaderHeight = 20
	// SpreadsheetRowHeaderWidth is the width of the row-number header band.
	SpreadsheetRowHeaderWidth = 36
)

// spreadsheetThumbMin is the smallest a scrollbar thumb shrinks to, so it stays
// grabbable on a very tall or wide sheet.
const spreadsheetThumbMin = 8

func spColW() int    { return scaled(SpreadsheetColWidth) }
func spRowH() int    { return scaled(SpreadsheetRowHeight) }
func spHeaderH() int { return scaled(SpreadsheetHeaderHeight) }
func spRowHdrW() int { return scaled(SpreadsheetRowHeaderWidth) }

// NewSpreadsheet builds an empty cols x rows sheet with cell A1 active. Negative
// dimensions clamp to 0 (the underlying model's contract).
func NewSpreadsheet(cols, rows int) *Spreadsheet {
	return &Spreadsheet{model: formula.NewModel(cols, rows)}
}

// Cols reports the sheet's column count.
func (s *Spreadsheet) Cols() int { return s.model.Cols() }

// Rows reports the sheet's row count.
func (s *Spreadsheet) Rows() int { return s.model.Rows() }

// SetCell stores raw (a literal or a leading-"=" formula) in cell (col,row) and
// recomputes every dependent cell. An out-of-bounds cell is a no-op.
func (s *Spreadsheet) SetCell(col, row int, raw string) {
	s.model.SetCell(formula.Ref{Col: col, Row: row}, raw)
}

// CellDisplay is the computed text shown in cell (col,row).
func (s *Spreadsheet) CellDisplay(col, row int) string {
	return s.model.Display(formula.Ref{Col: col, Row: row})
}

// CellRaw is the raw text stored in cell (col,row) — the formula or literal a
// user typed, which the editor re-opens.
func (s *Spreadsheet) CellRaw(col, row int) string {
	return s.model.Raw(formula.Ref{Col: col, Row: row})
}

// Active reports the currently selected cell.
func (s *Spreadsheet) Active() (col, row int) { return s.cur.Col, s.cur.Row }

// ScrollOffset reports the top-left visible cell (the current scroll position),
// in cell units — the observable a view model (or a test) reads.
func (s *Spreadsheet) ScrollOffset() (col, row int) { return s.scrollCol, s.scrollRow }

// Editing reports whether an inline cell edit is open.
func (s *Spreadsheet) Editing() bool { return s.editor != nil }

// A11y reports the Spreadsheet as a grid named by its active cell's A1 address,
// with the cell's displayed value.
func (s *Spreadsheet) A11y() A11yInfo {
	return A11yInfo{Role: RoleGrid, Name: s.cur.A1(), Value: s.model.Display(s.cur)}
}

// --- Geometry ----------------------------------------------------------------

// gridRect is the on-surface rectangle the data cells occupy: the widget minus
// the header bands and minus a scrollbar gutter on whichever edge overflows.
func (s *Spreadsheet) gridRect() Rect {
	r := s.Bounds()
	w := r.W - spRowHdrW()
	h := r.H - spHeaderH()
	if s.vOverflow() {
		w -= scrollbarWidth
	}
	if s.hOverflow() {
		h -= scrollbarWidth
	}
	return Rect{X: r.X + spRowHdrW(), Y: r.Y + spHeaderH(), W: w, H: h}
}

// vOverflow reports whether the rows are taller than the space below the header
// band (so a vertical scrollbar is needed).
func (s *Spreadsheet) vOverflow() bool {
	return s.model.Rows()*spRowH() > s.Bounds().H-spHeaderH()
}

// hOverflow reports whether the columns are wider than the space right of the
// row-header band (so a horizontal scrollbar is needed).
func (s *Spreadsheet) hOverflow() bool {
	return s.model.Cols()*spColW() > s.Bounds().W-spRowHdrW()
}

// cellRect is the on-surface rectangle of cell (col,row) at the current scroll,
// whether or not it is currently in view.
func (s *Spreadsheet) cellRect(col, row int) Rect {
	r := s.Bounds()
	return Rect{
		X: r.X + spRowHdrW() + (col-s.scrollCol)*spColW(),
		Y: r.Y + spHeaderH() + (row-s.scrollRow)*spRowH(),
		W: spColW(),
		H: spRowH(),
	}
}

// fullCols is the number of columns that fit whole in the grid viewport.
func (s *Spreadsheet) fullCols() int {
	if g := s.gridRect(); g.W > 0 {
		return g.W / spColW()
	}
	return 0
}

// fullRows is the number of rows that fit whole in the grid viewport.
func (s *Spreadsheet) fullRows() int {
	if g := s.gridRect(); g.H > 0 {
		return g.H / spRowH()
	}
	return 0
}

// visibleCols is the number of columns to paint from scrollCol (rounding up so a
// partial trailing column still draws), clamped to what remains.
func (s *Spreadsheet) visibleCols() int {
	g := s.gridRect()
	if g.W <= 0 {
		return 0
	}
	n := (g.W + spColW() - 1) / spColW()
	if s.scrollCol+n > s.model.Cols() {
		n = s.model.Cols() - s.scrollCol
	}
	return n
}

// visibleRows is the row counterpart of visibleCols.
func (s *Spreadsheet) visibleRows() int {
	g := s.gridRect()
	if g.H <= 0 {
		return 0
	}
	n := (g.H + spRowH() - 1) / spRowH()
	if s.scrollRow+n > s.model.Rows() {
		n = s.model.Rows() - s.scrollRow
	}
	return n
}

// maxScrollCol / maxScrollRow are the largest legal scroll offsets: enough to
// bring the last cell to the viewport's leading edge, never negative.
func (s *Spreadsheet) maxScrollCol() int {
	if m := s.model.Cols() - s.fullCols(); m > 0 {
		return m
	}
	return 0
}

func (s *Spreadsheet) maxScrollRow() int {
	if m := s.model.Rows() - s.fullRows(); m > 0 {
		return m
	}
	return 0
}

// clampScroll pins both scroll offsets into their legal range.
func (s *Spreadsheet) clampScroll() {
	if s.scrollCol < 0 {
		s.scrollCol = 0
	}
	if m := s.maxScrollCol(); s.scrollCol > m {
		s.scrollCol = m
	}
	if s.scrollRow < 0 {
		s.scrollRow = 0
	}
	if m := s.maxScrollRow(); s.scrollRow > m {
		s.scrollRow = m
	}
}

// ScrollBy shifts the visible window by (dCol, dRow) cells, clamped.
func (s *Spreadsheet) ScrollBy(dCol, dRow int) {
	s.scrollCol += dCol
	s.scrollRow += dRow
	s.clampScroll()
}

func (s *Spreadsheet) scrollColTo(col int) { s.scrollCol = col; s.clampScroll() }
func (s *Spreadsheet) scrollRowTo(row int) { s.scrollRow = row; s.clampScroll() }

// ensureVisible scrolls the minimum needed to bring the active cell into view.
func (s *Spreadsheet) ensureVisible() {
	if s.cur.Col < s.scrollCol {
		s.scrollCol = s.cur.Col
	}
	if s.cur.Col >= s.scrollCol+s.fullCols() {
		s.scrollCol = s.cur.Col - s.fullCols() + 1
	}
	if s.cur.Row < s.scrollRow {
		s.scrollRow = s.cur.Row
	}
	if s.cur.Row >= s.scrollRow+s.fullRows() {
		s.scrollRow = s.cur.Row - s.fullRows() + 1
	}
	s.clampScroll()
}
