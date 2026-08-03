// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// expanderTable builds an ungrouped Table opted into row detail via RowDetail,
// tall enough that every line fits.
func expanderTable() *Table {
	tb := NewTable([]TableColumn{
		{Title: "Name", Width: 80},
		{Title: "Val", Width: 60},
	}, [][]string{
		{"a", "1"},
		{"b", "2"},
		{"c", "3"},
	})
	tb.RowDetail = func(row int) string { return "detail-" + tb.Rows[row][0] }
	tb.SetBounds(Rect{X: 0, Y: 0, W: 140, H: 300})
	return tb
}

// --- gutters + predicates ------------------------------------------------

func TestTableDisclosureGutter(t *testing.T) {
	tb := expanderTable()
	if tb.disclosureGutter() != TableCellPadX+TableIconSize {
		t.Fatalf("disclosureGutter = %d, want %d", tb.disclosureGutter(), TableCellPadX+TableIconSize)
	}
	if tb.leadGutter() != TableCellPadX+TableIconSize {
		t.Fatalf("leadGutter (detail only) = %d", tb.leadGutter())
	}
	// With RowIcon also set the leading gutter stacks both.
	tb.RowIcon = func(int) (TableIconFunc, bool) { return DrawIconOpen, true }
	if tb.leadGutter() != 2*(TableCellPadX+TableIconSize) {
		t.Fatalf("leadGutter (detail+icon) = %d", tb.leadGutter())
	}
	// Nil RowDetail -> no disclosure gutter (byte-identical path).
	plain := NewTable([]TableColumn{{Title: "A"}}, nil)
	if plain.disclosureGutter() != 0 || plain.leadGutter() != 0 {
		t.Fatal("nil RowDetail must reserve no gutter")
	}
}

func TestTableExpandPredicates(t *testing.T) {
	tb := expanderTable()
	if tb.useLineModel() || tb.hasExpanded() {
		t.Fatal("no expansion yet -> line model off")
	}
	tb.toggleExpand(1)
	if !tb.expanded[1] || !tb.rowExpanded(1) {
		t.Fatal("row 1 should be expanded")
	}
	if !tb.hasExpanded() || !tb.useLineModel() {
		t.Fatal("an expanded row must activate the line model")
	}
	// Folding it shut deletes the entry so hasExpanded collapses cleanly.
	tb.toggleExpand(1)
	if tb.rowExpanded(1) || tb.hasExpanded() || len(tb.expanded) != 0 {
		t.Fatalf("fold should empty the set: %v", tb.expanded)
	}
	// Out-of-range toggle is a no-op.
	tb.toggleExpand(-1)
	tb.toggleExpand(99)
	if len(tb.expanded) != 0 {
		t.Fatalf("out-of-range toggle changed state: %v", tb.expanded)
	}
	// rowExpanded is false when RowDetail is nil even if the map says true.
	tb.expanded[0] = true
	tb.RowDetail = nil
	if tb.rowExpanded(0) || tb.hasExpanded() {
		t.Fatal("nil RowDetail must read as not expanded")
	}
}

// --- line model with detail lines ---------------------------------------

func TestTableExpandLinesUngrouped(t *testing.T) {
	tb := expanderTable()
	tb.toggleExpand(1)
	lines := tb.lines()
	// [r0][r1][detail r1][r2] = 4
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4: %+v", len(lines), lines)
	}
	if lines[1].dataRow != 1 || lines[1].detail {
		t.Fatalf("line1 = %+v, want data row 1", lines[1])
	}
	if !lines[2].detail || lines[2].dataRow != 1 {
		t.Fatalf("line2 = %+v, want detail of row 1", lines[2])
	}
	if lines[3].detail || lines[3].dataRow != 2 {
		t.Fatalf("line3 = %+v, want data row 2", lines[3])
	}
	if tb.lineCount() != 4 {
		t.Fatalf("lineCount = %d, want 4", tb.lineCount())
	}
}

func TestTableExpandLinesGrouped(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "Grp", Width: 50},
		{Title: "Val", Width: 50},
	}, [][]string{{"g1", "a"}, {"g1", "b"}, {"g2", "c"}})
	tb.GroupBy = 0
	tb.RowDetail = func(row int) string { return "d" }
	tb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})
	tb.toggleExpand(0) // expand first row of g1
	lines := tb.lines()
	// [hdr g1][r0][detail r0][r1][hdr g2][r2] = 6
	if len(lines) != 6 {
		t.Fatalf("grouped+detail lines = %d, want 6: %+v", len(lines), lines)
	}
	if !lines[2].detail || lines[2].dataRow != 0 {
		t.Fatalf("line2 = %+v, want detail of row 0", lines[2])
	}
}

// TestTableExpandRowAtVisualIndex: a detail-line click resolves to no row,
// data rows still map, and visualIndex skips detail lines.
func TestTableExpandRowAtVisualIndex(t *testing.T) {
	tb := expanderTable()
	tb.toggleExpand(0)
	yOf := func(vi int) int { return TableHeaderHeight + vi*TableRowHeight + 2 }
	if got := tb.rowAt(yOf(0)); got != 0 {
		t.Fatalf("rowAt(data line 0) = %d, want 0", got)
	}
	if got := tb.rowAt(yOf(1)); got != -1 { // detail line for row 0
		t.Fatalf("rowAt(detail line) = %d, want -1", got)
	}
	if got := tb.rowAt(yOf(2)); got != 1 { // row 1, shifted down by the detail line
		t.Fatalf("rowAt(line 2) = %d, want 1", got)
	}
	if got := tb.visualIndex(1); got != 2 {
		t.Fatalf("visualIndex(1) = %d, want 2 (detail line pushes it down)", got)
	}
}

// --- OnEvent: disclosure toggles vs cell selects -------------------------

func TestTableExpandDisclosureClick(t *testing.T) {
	tb := expanderTable()
	tb.MultiSelect = true
	yRow0 := TableHeaderHeight + TableRowHeight/2
	// A click inside the disclosure gutter toggles expansion, no selection.
	tb.OnEvent(Event{Kind: EventClick, X: TableCellPadX + 2, Y: yRow0})
	if !tb.rowExpanded(0) {
		t.Fatal("disclosure click should expand row 0")
	}
	if len(tb.SelectedRows()) != 0 {
		t.Fatalf("disclosure click must not select: %v", tb.SelectedRows())
	}
	// A click on the cell area (past the gutter) selects instead of toggling.
	tb.OnEvent(Event{Kind: EventClick, X: tb.leadGutter() + 20, Y: yRow0})
	if !tb.IsRowSelected(0) {
		t.Fatal("cell click should select row 0")
	}
	if !tb.rowExpanded(0) {
		t.Fatal("cell click must not collapse an expanded row")
	}
	// Clicking the disclosure again folds it back.
	tb.OnEvent(Event{Kind: EventClick, X: TableCellPadX + 2, Y: yRow0})
	if tb.rowExpanded(0) {
		t.Fatal("second disclosure click should collapse row 0")
	}
}

// TestTableExpandDisclosureInEditableColumn: the chevron toggles even when
// column 0 is Editable (disclosure wins over the editor).
func TestTableExpandDisclosureInEditableColumn(t *testing.T) {
	tb := expanderTable()
	tb.Columns[0].Editable = true
	tb.OnEvent(Event{Kind: EventClick, X: TableCellPadX + 2, Y: TableHeaderHeight + TableRowHeight/2})
	if !tb.rowExpanded(0) {
		t.Fatal("disclosure click in an Editable column should still expand")
	}
	if tb.editRow != -1 {
		t.Fatalf("disclosure click must not open the editor: editRow=%d", tb.editRow)
	}
	// A click past the gutter on the Editable cell opens the editor.
	tb.OnEvent(Event{Kind: EventClick, X: tb.leadGutter() + 10, Y: TableHeaderHeight + TableRowHeight/2})
	if tb.editRow != 0 {
		t.Fatalf("cell click should open editor: editRow=%d", tb.editRow)
	}
}

// TestTableExpandDetailClickInert: a click on the detail line neither selects
// nor toggles.
func TestTableExpandDetailClickInert(t *testing.T) {
	tb := expanderTable()
	tb.MultiSelect = true
	tb.toggleExpand(0)
	// Detail line is visual line 1; click well past the gutter.
	tb.OnEvent(Event{Kind: EventClick, X: 60, Y: TableHeaderHeight + TableRowHeight + 2})
	if len(tb.SelectedRows()) != 0 || tb.Selected != -1 {
		t.Fatalf("detail click changed selection: rows=%v sel=%d", tb.SelectedRows(), tb.Selected)
	}
	if !tb.rowExpanded(0) {
		t.Fatal("detail click must not collapse the row")
	}
}

// TestTableExpandReorderSuppressed: an expanded row suppresses drag-to-reorder
// (the flat insertion math can't place a drop against detail lines).
func TestTableExpandReorderSuppressed(t *testing.T) {
	tb := expanderTable()
	tb.Reorderable = true
	if !tb.reorderActive() {
		t.Fatal("reorder should be active before any expansion")
	}
	tb.toggleExpand(0)
	if tb.reorderActive() {
		t.Fatal("reorder must be suppressed while a row is expanded")
	}
}

// --- Draw: chevron + detail band paint ----------------------------------

func TestTableExpandDrawChevronAndDetail(t *testing.T) {
	tb := expanderTable()
	tb.toggleExpand(0)
	theme := DefaultLight()
	buf := makeTableSurface(140, 300)
	tb.Draw(newP(buf, 140), theme)
	// A disclosure chevron paints OnSurface ink inside row 0's leading gutter.
	chevFound := false
	for y := TableHeaderHeight; y < TableHeaderHeight+TableRowHeight && !chevFound; y++ {
		for x := 0; x < TableCellPadX+TableIconSize; x++ {
			if pixelAt(buf, 140, x, y) == theme.OnSurface {
				chevFound = true
				break
			}
		}
	}
	if !chevFound {
		t.Fatal("no disclosure chevron ink found in row 0 gutter")
	}
	// The detail line (visual line 1) paints a SurfaceAlt band.
	dy := TableHeaderHeight + TableRowHeight + TableRowHeight/2
	if got := pixelAt(buf, 140, 5, dy); got != theme.SurfaceAlt {
		t.Fatalf("detail band pixel = %+v, want SurfaceAlt %+v", got, theme.SurfaceAlt)
	}
	// The detail text paints OnSurface ink somewhere in that band, past the
	// indent gutter.
	textFound := false
	for y := TableHeaderHeight + TableRowHeight; y < TableHeaderHeight+2*TableRowHeight && !textFound; y++ {
		for x := tb.leadGutter(); x < 140; x++ {
			if pixelAt(buf, 140, x, y) == theme.OnSurface {
				textFound = true
				break
			}
		}
	}
	if !textFound {
		t.Fatal("no detail text ink found in detail band")
	}
	// Row 1 (visual line 2) still renders as a normal data row -- the detail
	// line pushed it down but its zebra keys on the absolute index 1 (odd), so
	// it paints Background, distinct from the detail band's SurfaceAlt.
	ry := TableHeaderHeight + 2*TableRowHeight + TableRowHeight/2
	if got := pixelAt(buf, 140, 60, ry); got != theme.Background {
		t.Fatalf("row-1 pixel = %+v, want Background", got)
	}
}

// TestTableExpandDrawWithIcon exercises paintCell's stacked disclosure+icon
// gutter (both leading features on at once).
func TestTableExpandDrawWithIcon(t *testing.T) {
	tb := expanderTable()
	tb.RowIcon = func(int) (TableIconFunc, bool) { return DrawIconOpen, true }
	tb.toggleExpand(0)
	buf := makeTableSurface(140, 300)
	tb.Draw(newP(buf, 140), DefaultLight())
	// Chevron ink in the FIRST gutter slice, icon ink in the SECOND slice.
	theme := DefaultLight()
	slice := TableCellPadX + TableIconSize
	inSlice := func(x0, x1 int) bool {
		for y := TableHeaderHeight; y < TableHeaderHeight+TableRowHeight; y++ {
			for x := x0; x < x1; x++ {
				if pixelAt(buf, 140, x, y) == theme.OnSurface {
					return true
				}
			}
		}
		return false
	}
	if !inSlice(0, slice) {
		t.Fatal("no chevron ink in the disclosure gutter slice")
	}
	if !inSlice(slice, 2*slice) {
		t.Fatal("no icon ink in the icon gutter slice")
	}
}

// TestTableExpandEditorOverlay: cellRect honours the leading gutter so the
// inline editor sits over the text area of an expander+editable table.
func TestTableExpandEditorOverlay(t *testing.T) {
	tb := expanderTable()
	tb.Columns[1].Editable = true
	tb.toggleExpand(0)
	tb.beginEdit(1, 1) // edit row 1, which sits at visual line 2
	buf := makeTableSurface(140, 300)
	tb.Draw(newP(buf, 140), DefaultLight())
	rect := tb.cellRect(1, 1)
	// Row 1's cell is at visual line 2 (after row 0 + its detail line).
	wantY := TableHeaderHeight + 2*TableRowHeight
	if rect.Y != wantY {
		t.Fatalf("cellRect Y = %d, want %d", rect.Y, wantY)
	}
	if tb.editor == nil {
		t.Fatal("editor should be open")
	}
}
