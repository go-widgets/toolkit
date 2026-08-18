// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// groupedTable builds a Table grouped by column 0 with two groups ("g1" has
// two rows, "g2" one) and a tall enough viewport that all visual lines fit
// (no scrollbar). Visual lines: [hdr g1][a][b][hdr g2][c] = 5.
func groupedTable() *Table {
	tb := NewTable([]TableColumn{
		{Title: "Grp", Width: 50},
		{Title: "Val", Width: 50},
	}, [][]string{{"g1", "a"}, {"g1", "b"}, {"g2", "c"}})
	tb.GroupBy = 0
	tb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	return tb
}

// yOfLine is the Table-local y that lands inside visual line vi's band.
func yOfLine(vi int) int { return TableHeaderHeight + vi*TableRowHeight + 2 }

// TestTableGroupLines checks the visual-line model: headers carry the group
// value + member count, data lines point at the right rows, and lineCount
// counts headers + expanded rows.
func TestTableGroupLines(t *testing.T) {
	tb := groupedTable()
	lines := tb.lines()
	if len(lines) != 5 {
		t.Fatalf("lines=%d, want 5 (2 headers + 3 rows)", len(lines))
	}
	if !lines[0].header || lines[0].group != "g1" || lines[0].count != 2 {
		t.Fatalf("line0=%+v, want header g1 count 2", lines[0])
	}
	if lines[1].header || lines[1].dataRow != 0 || lines[2].dataRow != 1 {
		t.Fatalf("data lines wrong: %+v %+v", lines[1], lines[2])
	}
	if !lines[3].header || lines[3].group != "g2" || lines[3].count != 1 {
		t.Fatalf("line3=%+v, want header g2 count 1", lines[3])
	}
	if lines[4].dataRow != 2 {
		t.Fatalf("line4 dataRow=%d, want 2", lines[4].dataRow)
	}
	if tb.lineCount() != 5 {
		t.Fatalf("lineCount=%d, want 5", tb.lineCount())
	}
	if !tb.grouped() {
		t.Fatal("grouped() should be true")
	}
	// An out-of-range GroupBy is treated as ungrouped.
	tb.GroupBy = 9
	if tb.grouped() || tb.lines() != nil || tb.lineCount() != 3 {
		t.Fatal("out-of-range GroupBy must read as ungrouped")
	}
}

// TestTableGroupCollapseToggle: clicking a header folds its members away
// (lineCount shrinks, the group's rows vanish from the line list); clicking
// again restores them.
func TestTableGroupCollapseToggle(t *testing.T) {
	tb := groupedTable()

	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: yOfLine(0)}) // toggle g1
	if !tb.collapsed["g1"] {
		t.Fatal("g1 should be collapsed after header click")
	}
	// Now lines: [hdr g1 collapsed][hdr g2][c] = 3.
	if tb.lineCount() != 3 {
		t.Fatalf("collapsed lineCount=%d, want 3", tb.lineCount())
	}
	lines := tb.lines()
	if !lines[0].collapsed {
		t.Fatal("g1 header line should report collapsed")
	}
	for _, ln := range lines {
		if !ln.header && (ln.dataRow == 0 || ln.dataRow == 1) {
			t.Fatal("collapsed g1's member rows must not appear as data lines")
		}
	}

	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: yOfLine(0)}) // toggle g1 back
	if tb.collapsed["g1"] || tb.lineCount() != 5 {
		t.Fatalf("re-expand failed: collapsed=%v lineCount=%d", tb.collapsed["g1"], tb.lineCount())
	}
}

// TestTableGroupRowAt: rowAt maps a data-line click to its row index and a
// header-line click to -1 (no row).
func TestTableGroupRowAt(t *testing.T) {
	tb := groupedTable()
	if got := tb.rowAt(yOfLine(1)); got != 0 { // first data line = row 0
		t.Fatalf("rowAt(data line 1)=%d, want 0", got)
	}
	if got := tb.rowAt(yOfLine(4)); got != 2 { // last data line = row 2
		t.Fatalf("rowAt(data line 4)=%d, want 2", got)
	}
	if got := tb.rowAt(yOfLine(0)); got != -1 { // header line
		t.Fatalf("rowAt(header)=%d, want -1", got)
	}
	if got := tb.rowAt(yOfLine(99)); got != -1 { // past the body
		t.Fatalf("rowAt(out of range)=%d, want -1", got)
	}
}

// TestTableGroupSelectDataRow: with grouping on, clicking a data row still
// selects it (by data-row index), while a header click never selects.
func TestTableGroupSelectDataRow(t *testing.T) {
	tb := groupedTable()
	tb.MultiSelect = true
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: yOfLine(4)}) // row 2
	if tb.Selected().Get() != 2 {
		t.Fatalf("Selected=%d, want 2", tb.Selected().Get())
	}
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: yOfLine(3)}) // g2 header
	if tb.Selected().Get() != 2 {
		t.Fatalf("Selected changed to %d on a header click, want still 2", tb.Selected().Get())
	}
}

// TestTableLineAtEdges covers lineAt's guard branches: header strip, ungrouped
// table, and a y past the last line all return ok=false.
func TestTableLineAtEdges(t *testing.T) {
	tb := groupedTable()
	if _, ok := tb.lineAt(TableHeaderHeight - 1); ok {
		t.Fatal("lineAt in the header strip must be false")
	}
	if _, ok := tb.lineAt(yOfLine(99)); ok {
		t.Fatal("lineAt past the body must be false")
	}
	if ln, ok := tb.lineAt(yOfLine(1)); !ok || ln.header {
		t.Fatalf("lineAt on a data line = (%+v,%v), want data line, true", ln, ok)
	}
	ub := NewTable([]TableColumn{{Title: "A", Width: 50}}, [][]string{{"x"}})
	ub.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	if _, ok := ub.lineAt(TableHeaderHeight + 2); ok {
		t.Fatal("lineAt on an ungrouped table must be false")
	}
}

// TestTableGroupVisualIndex: a data row maps to its on-screen line; a row in a
// collapsed group has no visible line (-1).
func TestTableGroupVisualIndex(t *testing.T) {
	tb := groupedTable()
	if got := tb.visualIndex(2); got != 4 {
		t.Fatalf("visualIndex(row 2)=%d, want 4", got)
	}
	tb.toggleGroup("g1") // collapse g1
	if got := tb.visualIndex(0); got != -1 {
		t.Fatalf("visualIndex of a row in a collapsed group=%d, want -1", got)
	}
}

// TestTableGroupRaggedValue: a row too short to have the GroupBy cell forms its
// own "" group.
func TestTableGroupRaggedValue(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "A", Width: 50},
		{Title: "B", Width: 50},
	}, [][]string{{"a", "x"}, {"b"}}) // row 1 has no column-1 cell
	tb.GroupBy = 1
	tb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	lines := tb.lines()
	// [hdr "x"][row0][hdr ""][row1]
	if len(lines) != 4 || lines[0].group != "x" || lines[2].group != "" {
		t.Fatalf("ragged grouping lines=%+v", lines)
	}
}

// TestTableGroupDraw renders a grouped table (both expanded and with one group
// collapsed) and asserts the header bands paint.
func TestTableGroupDraw(t *testing.T) {
	tb := groupedTable()
	const w, h = 200, 200

	buf := makeSurface(w, h)
	tb.Draw(newP(buf, w), DefaultLight())
	// The g1 header band sits at the first body line; assert its chevron/text
	// painted something in the top-left of the body.
	if !anyPainted(buf, w, 0, TableHeaderHeight, 60, TableHeaderHeight+TableRowHeight) {
		t.Fatal("expanded group header painted nothing")
	}

	tb.toggleGroup("g1")
	buf2 := makeSurface(w, h)
	tb.Draw(newP(buf2, w), DefaultLight())
	if !anyPainted(buf2, w, 0, TableHeaderHeight, 60, TableHeaderHeight+TableRowHeight) {
		t.Fatal("collapsed group header painted nothing")
	}
}

// TestTableGroupScrollbar: with more visual lines than fit, the body overflows
// and a scrollbar is drawn (contentH measured over lines, not rows).
func TestTableGroupScrollbar(t *testing.T) {
	rows := make([][]string, 0, 12)
	for i := 0; i < 6; i++ {
		rows = append(rows, []string{"g1", "a"})
	}
	for i := 0; i < 6; i++ {
		rows = append(rows, []string{"g2", "b"})
	}
	tb := NewTable([]TableColumn{{Title: "G", Width: 50}, {Title: "V", Width: 50}}, rows)
	tb.GroupBy = 0
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 100}) // 14 lines >> visible
	if !tb.bodyOverflows() {
		t.Fatal("a 14-line grouped body must overflow a 100px viewport")
	}
	buf := makeSurface(120, 100)
	tb.Draw(newP(buf, 120), DefaultLight()) // exercises the grouped scrollbar path
}

// TestTableGroupEditorHiddenWhenCollapsed: an open editor whose row is folded
// into a collapsed group is not drawn (visualIndex == -1) and Draw must not
// panic.
func TestTableGroupEditorHiddenWhenCollapsed(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "Grp", Width: 50},
		{Title: "Val", Width: 50, Editable: true},
	}, [][]string{{"g1", "a"}, {"g2", "b"}})
	tb.GroupBy = 0
	tb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})

	tb.beginEdit(0, 1) // edit row 0 (in group g1)
	tb.toggleGroup("g1")
	buf := makeSurface(200, 200)
	tb.Draw(newP(buf, 200), DefaultLight()) // editor for the hidden row must be skipped
	if tb.visualIndex(0) != -1 {
		t.Fatal("row 0 should be hidden after collapsing g1")
	}
}

// TestTableGroupSuppressesReorder: grouping turns drag-to-reorder off --
// DragData is empty, AcceptsDrop is false, a body press sets no drag row, and
// drag events are inert.
func TestTableGroupSuppressesReorder(t *testing.T) {
	tb := groupedTable()
	tb.Reorderable = true // ...but grouping wins

	if tb.reorderActive() {
		t.Fatal("reorderActive must be false while grouped")
	}
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: yOfLine(1)}) // press a data row
	if tb.dragRow != -1 {
		t.Fatalf("dragRow=%d, want -1 (no drag while grouped)", tb.dragRow)
	}
	if tb.DragData() != "" {
		t.Fatalf("DragData=%q, want empty while grouped", tb.DragData())
	}
	if tb.AcceptsDrop(tableRowDragPrefix + "0") {
		t.Fatal("AcceptsDrop must be false while grouped")
	}
	tb.OnEvent(Event{Kind: EventDragMove, X: 10, Y: yOfLine(1)})
	if tb.dropIndicator != -1 {
		t.Fatalf("dropIndicator=%d, want -1 (drag inert while grouped)", tb.dropIndicator)
	}
	tb.OnEvent(Event{Kind: EventDrop, X: 10, Y: yOfLine(1), Code: tableRowDragPrefix + "0"})
	if tb.Rows[0][0] != "g1" {
		t.Fatal("a drop while grouped must not reorder rows")
	}
}

// anyPainted reports whether any pixel in [x0,x1)x[y0,y1) differs from the
// makeSurface sentinel (0xC8).
func anyPainted(buf []byte, w, x0, y0, x1, y1 int) bool {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			px := pixelAt(buf, w, x, y)
			if px.R != 0xC8 || px.G != 0xC8 || px.B != 0xC8 {
				return true
			}
		}
	}
	return false
}
