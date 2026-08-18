// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// editableTable builds a 2x2 Table whose second column is Editable, bounds set
// so the header + both body rows are visible. Column 0 is 50px wide, column 1
// auto-fills the rest.
func editableTable() *Table {
	tb := NewTable([]TableColumn{
		{Title: "Name", Width: 50},
		{Title: "Value", Width: 50, Editable: true},
	}, [][]string{{"a", "1"}, {"b", "2"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	return tb
}

// bodyClick returns an EventClick landing in body row r, column-1 (the editable
// column, x=60 with the 50px first column).
func editClickCol1(r int) Event {
	return Event{Kind: EventClick, X: 60, Y: TableHeaderHeight + r*TableRowHeight + 2}
}

// TestTableCellEditBeginTypeCommit covers the happy path: a click on an editable
// cell opens an editor seeded with the value, typed characters route to it, and
// Enter commits -- writing Rows and firing OnCellEdit.
func TestTableCellEditBeginTypeCommit(t *testing.T) {
	tb := editableTable()
	var gotRow, gotCol int
	var gotVal string
	calls := 0
	tb.OnCellEdit = func(row, col int, value string) {
		gotRow, gotCol, gotVal, calls = row, col, value, calls+1
	}

	tb.OnEvent(editClickCol1(0))
	if tb.editRow != 0 || tb.editCol != 1 || tb.editor == nil {
		t.Fatalf("after click: editRow=%d editCol=%d editor=%v, want 0,1,non-nil", tb.editRow, tb.editCol, tb.editor)
	}
	if tb.editor.CellValue() != "1" {
		t.Fatalf("editor seeded with %q, want %q", tb.editor.CellValue(), "1")
	}

	// Type a character (routes to the editor) then commit with Enter.
	tb.OnEvent(Event{Kind: EventChar, Code: "9"})
	if tb.editor.CellValue() != "19" {
		t.Fatalf("after typing, editor=%q, want %q", tb.editor.CellValue(), "19")
	}
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})

	if tb.Rows[0][1] != "19" {
		t.Fatalf("Rows[0][1]=%q after commit, want %q", tb.Rows[0][1], "19")
	}
	if calls != 1 || gotRow != 0 || gotCol != 1 || gotVal != "19" {
		t.Fatalf("OnCellEdit=(row %d,col %d,val %q,calls %d), want (0,1,\"19\",1)", gotRow, gotCol, gotVal, calls)
	}
	if tb.editRow != -1 || tb.editor != nil {
		t.Fatalf("editor must close after commit, got editRow=%d editor=%v", tb.editRow, tb.editor)
	}
}

// TestTableCellEditEscapeCancels: Escape closes the editor without touching Rows
// or firing OnCellEdit.
func TestTableCellEditEscapeCancels(t *testing.T) {
	tb := editableTable()
	called := false
	tb.OnCellEdit = func(int, int, string) { called = true }

	tb.OnEvent(editClickCol1(0))
	tb.OnEvent(Event{Kind: EventChar, Code: "Z"})
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})

	if tb.Rows[0][1] != "1" {
		t.Fatalf("Rows[0][1]=%q after cancel, want unchanged %q", tb.Rows[0][1], "1")
	}
	if called {
		t.Fatal("OnCellEdit fired on cancel")
	}
	if tb.editRow != -1 || tb.editor != nil {
		t.Fatal("editor must close after cancel")
	}
}

// TestTableCellEditNonEditableColumnSelects: a click on a non-editable column
// does not open an editor and still drives row selection.
func TestTableCellEditNonEditableColumnSelects(t *testing.T) {
	tb := editableTable()
	tb.MultiSelect = true
	// Column 0 (x=10) is not Editable.
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 2})
	if tb.editRow != -1 || tb.editor != nil {
		t.Fatal("a non-editable cell must not open an editor")
	}
	if tb.Selected().Get() != 0 {
		t.Fatalf("Selected=%d, want row 0 selected", tb.Selected().Get())
	}
}

// TestTableCellEditClickOtherCellCommits: with an editor open, clicking a
// different editable cell commits the current edit and opens a new one there.
func TestTableCellEditClickOtherCellCommits(t *testing.T) {
	tb := editableTable()
	commits := 0
	tb.OnCellEdit = func(int, int, string) { commits++ }

	tb.OnEvent(editClickCol1(0))
	tb.OnEvent(Event{Kind: EventChar, Code: "X"}) // editor now "1X"
	tb.OnEvent(editClickCol1(1))                  // click row 1's editable cell

	if commits != 1 {
		t.Fatalf("commits=%d, want 1 (first edit committed on outside click)", commits)
	}
	if tb.Rows[0][1] != "1X" {
		t.Fatalf("Rows[0][1]=%q, want committed %q", tb.Rows[0][1], "1X")
	}
	if tb.editRow != 1 || tb.editCol != 1 {
		t.Fatalf("new editor at (%d,%d), want (1,1)", tb.editRow, tb.editCol)
	}
	if tb.editor.CellValue() != "2" {
		t.Fatalf("new editor seeded %q, want %q", tb.editor.CellValue(), "2")
	}
}

// TestTableCellEditClickSameCellKeepsEditing: clicking inside the cell already
// being edited leaves the editor open (no commit, no reopen).
func TestTableCellEditClickSameCellKeepsEditing(t *testing.T) {
	tb := editableTable()
	commits := 0
	tb.OnCellEdit = func(int, int, string) { commits++ }

	tb.OnEvent(editClickCol1(0))
	first := tb.editor
	tb.OnEvent(editClickCol1(0)) // same cell

	if commits != 0 {
		t.Fatalf("commits=%d, want 0 (same-cell click keeps editing)", commits)
	}
	if tb.editor != first {
		t.Fatal("same-cell click must not replace the editor")
	}
}

// TestTableCellEditKeyRoutingBackspace: a non-Enter/Escape edit key routes to
// the editor (Backspace deletes) while an edit is open.
func TestTableCellEditKeyRoutingBackspace(t *testing.T) {
	tb := editableTable()
	tb.OnEvent(editClickCol1(0)) // editor "1"
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if tb.editor.CellValue() != "" {
		t.Fatalf("after Backspace editor=%q, want empty", tb.editor.CellValue())
	}
}

// TestTableCellEditDrawOverlay: an open editor paints over its cell (some
// non-sentinel pixels appear in the editable column's band).
func TestTableCellEditDrawOverlay(t *testing.T) {
	tb := editableTable()
	tb.OnEvent(editClickCol1(0))

	const w, h = 100, 100
	buf := makeSurface(w, h)
	tb.Draw(newP(buf, w), DefaultLight())

	// The editor sits over row 0 of column 1 (x in [50,100), y just below the
	// header). Assert at least one painted pixel there.
	painted := false
	for y := TableHeaderHeight; y < TableHeaderHeight+TableRowHeight && !painted; y++ {
		for x := 55; x < w; x++ {
			px := pixelAt(buf, w, x, y)
			if px.R != 0xC8 || px.G != 0xC8 || px.B != 0xC8 {
				painted = true
				break
			}
		}
	}
	if !painted {
		t.Fatal("open editor painted nothing over its cell")
	}
}

// TestTableBeginEditNoOps exercises beginEdit's guard branches directly: an
// out-of-range row, an out-of-range column, and a non-editable column all leave
// the Table in the not-editing state.
func TestTableBeginEditNoOps(t *testing.T) {
	tb := editableTable()

	tb.beginEdit(-1, 1) // bad row
	tb.beginEdit(5, 1)  // row past end
	tb.beginEdit(0, -1) // bad col
	tb.beginEdit(0, 9)  // col past end
	tb.beginEdit(0, 0)  // column 0 is not Editable
	if tb.editRow != -1 || tb.editor != nil {
		t.Fatalf("no-op beginEdit opened an editor: editRow=%d editor=%v", tb.editRow, tb.editor)
	}
}

// TestTableBeginEditRaggedRow: an editable cell whose Row has no slot for the
// column seeds the editor with "" (the ragged-row default) and commit leaves
// Rows untouched while still firing OnCellEdit.
func TestTableBeginEditRaggedRow(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "Name", Width: 50},
		{Title: "Value", Width: 50, Editable: true},
	}, [][]string{{"only-one-cell"}}) // Row 0 has no column-1 slot
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	var gotVal string
	calls := 0
	tb.OnCellEdit = func(_, _ int, v string) { gotVal, calls = v, calls+1 }

	tb.beginEdit(0, 1)
	if tb.editor == nil || tb.editor.CellValue() != "" {
		t.Fatalf("ragged-row editor seed = %v, want empty", tb.editor)
	}
	tb.editor.SetCellValue("new")
	tb.commitEdit()
	if len(tb.Rows[0]) != 1 {
		t.Fatalf("ragged Row was widened to %d cells, want left at 1", len(tb.Rows[0]))
	}
	if calls != 1 || gotVal != "new" {
		t.Fatalf("OnCellEdit=(val %q,calls %d), want (\"new\",1)", gotVal, calls)
	}
}

// TestTableCommitEditNotEditingNoOp: commitEdit with no edit in progress does
// nothing and never fires OnCellEdit.
func TestTableCommitEditNotEditingNoOp(t *testing.T) {
	tb := editableTable()
	called := false
	tb.OnCellEdit = func(int, int, string) { called = true }
	tb.commitEdit() // editRow == -1
	if called {
		t.Fatal("commitEdit fired OnCellEdit with no edit in progress")
	}
}

// TestTableCommitEditNilCallbackNoPanic: committing with OnCellEdit unset writes
// Rows and must not panic.
func TestTableCommitEditNilCallbackNoPanic(t *testing.T) {
	tb := editableTable()
	tb.beginEdit(0, 1)
	tb.editor.SetCellValue("z")
	tb.commitEdit() // OnCellEdit nil
	if tb.Rows[0][1] != "z" {
		t.Fatalf("Rows[0][1]=%q, want %q", tb.Rows[0][1], "z")
	}
}

// TestTableCellRectGutter: cellRect carves the icon gutter off column 0 and
// tracks scroll/column offsets for later columns.
func TestTableCellRectGutter(t *testing.T) {
	tb := editableTable()

	// Without a RowIcon, column 0 starts flush at the left edge.
	plain := tb.cellRect(0, 0)
	if plain.X != 0 {
		t.Fatalf("no-icon col-0 X=%d, want 0", plain.X)
	}

	// With a RowIcon, column 0's rect is inset by the gutter.
	tb.RowIcon = func(int) (TableIconFunc, bool) { return nil, false }
	g := tb.iconGutter()
	if g == 0 {
		t.Fatal("iconGutter should be non-zero with a RowIcon set")
	}
	iconed := tb.cellRect(0, 0)
	if iconed.X != g || iconed.W != plain.W-g {
		t.Fatalf("iconed col-0 = X %d W %d, want X %d W %d", iconed.X, iconed.W, g, plain.W-g)
	}

	// Column 1 starts after column 0's width and is not gutter-adjusted; row 1
	// sits one row lower than row 0.
	c1r0 := tb.cellRect(0, 1)
	c1r1 := tb.cellRect(1, 1)
	if c1r0.X != 50 {
		t.Fatalf("col-1 X=%d, want 50 (after 50px col 0)", c1r0.X)
	}
	if c1r1.Y-c1r0.Y != TableRowHeight {
		t.Fatalf("row-1 minus row-0 Y = %d, want %d", c1r1.Y-c1r0.Y, TableRowHeight)
	}
}
