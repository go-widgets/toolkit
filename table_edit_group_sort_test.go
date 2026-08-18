// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"testing"

	"github.com/go-widgets/painter"
)

// fakeCellEditor is a minimal CellEditor for exercising the TableColumn.Editor
// seam: it records the value seeded/read, the focus flag, whether Draw ran, and
// the submit callback the Table registers.
type fakeCellEditor struct {
	Base
	val     string
	focused bool
	drawn   bool
	submit  func()
}

func (f *fakeCellEditor) Draw(painter.Painter, *Theme) { f.drawn = true }
func (f *fakeCellEditor) CellValue() string            { return f.val }
func (f *fakeCellEditor) SetCellValue(s string)        { f.val = s }
func (f *fakeCellEditor) OnCellSubmit(fn func())       { f.submit = fn }
func (f *fakeCellEditor) Focus(b bool)                 { f.focused = b }

// --- EditActivation ---------------------------------------------------------

// TestTableEditOnDoubleClick: in EditOnDoubleClick mode a single click on an
// Editable cell selects its row (no editor); a double-click (Code
// TableDoubleClick) opens the editor at that exact cell.
func TestTableEditOnDoubleClick(t *testing.T) {
	tb := editableTable()
	tb.MultiSelect = true
	tb.EditActivation = EditOnDoubleClick

	// A plain single click selects row 0 and opens NO editor.
	tb.OnEvent(editClickCol1(0))
	if _, _, editing := tb.Editing(); editing {
		t.Fatal("single click in double-click mode must not open an editor")
	}
	if tb.Selected().Get() != 0 {
		t.Fatalf("single click Selected=%d, want row 0 selected", tb.Selected().Get())
	}

	// A double-click on row 1's editable cell opens the editor there.
	dbl := editClickCol1(1)
	dbl.Code = TableDoubleClick
	tb.OnEvent(dbl)
	row, col, editing := tb.Editing()
	if !editing || row != 1 || col != 1 {
		t.Fatalf("double click Editing()=(%d,%d,%v), want (1,1,true)", row, col, editing)
	}
}

// TestTableEditManual: EditManual disables click activation; only BeginEdit
// opens an editor.
func TestTableEditManual(t *testing.T) {
	tb := editableTable()
	tb.EditActivation = EditManual

	tb.OnEvent(editClickCol1(0))
	if _, _, editing := tb.Editing(); editing {
		t.Fatal("a click in EditManual mode must not open an editor")
	}
	tb.BeginEdit(0, 1)
	if row, col, editing := tb.Editing(); !editing || row != 0 || col != 1 {
		t.Fatalf("BeginEdit Editing()=(%d,%d,%v), want (0,1,true)", row, col, editing)
	}
}

// TestTableEnterBeginsEditInDoubleClickMode: Enter on the cursor row opens the
// first Editable column's editor when EditOnDoubleClick, and does NOT when the
// mode leaves Enter as a plain row activation.
func TestTableEnterBeginsEditInDoubleClickMode(t *testing.T) {
	tb := editableTable()
	tb.EditActivation = EditOnDoubleClick
	tb.Selected().Set(1)
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if row, col, editing := tb.Editing(); !editing || row != 1 || col != 1 {
		t.Fatalf("Enter Editing()=(%d,%d,%v), want (1,1,true)", row, col, editing)
	}

	// EditOnSingleClick (default): Enter does NOT edit even with an editable col.
	tb2 := editableTable()
	tb2.Selected().Set(0)
	tb2.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if _, _, editing := tb2.Editing(); editing {
		t.Fatal("Enter must not edit outside EditOnDoubleClick mode")
	}
}

// TestTableEnterEditGuards: Enter-to-edit is a no-op (falls back to activation)
// when the cursor is out of range or the row has no Editable column.
func TestTableEnterEditGuards(t *testing.T) {
	// Out-of-range cursor.
	tb := editableTable()
	tb.EditActivation = EditOnDoubleClick
	tb.Selected().Set(-1)
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if _, _, editing := tb.Editing(); editing {
		t.Fatal("Enter with no cursor must not open an editor")
	}

	// No editable column at all.
	ro := NewTable([]TableColumn{{Title: "A"}, {Title: "B"}}, [][]string{{"x", "y"}})
	ro.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	ro.EditActivation = EditOnDoubleClick
	ro.Selected().Set(0)
	ro.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if _, _, editing := ro.Editing(); editing {
		t.Fatal("Enter with no editable column must not open an editor")
	}
}

// --- Editor seam ------------------------------------------------------------

// TestTableCustomCellEditor: a column's Editor factory supplies the editing
// control; the Table seeds it, focuses it, wires its submit to commit, draws
// it, and commit reads its CellValue back into Rows.
func TestTableCustomCellEditor(t *testing.T) {
	var built *fakeCellEditor
	tb := NewTable([]TableColumn{
		{Title: "Name", Width: 50},
		{Title: "Value", Width: 50, Editable: true, Editor: func() CellEditor {
			built = &fakeCellEditor{}
			return built
		}},
	}, [][]string{{"a", "1"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	got := ""
	tb.OnCellEdit = func(_, _ int, v string) { got = v }

	tb.BeginEdit(0, 1)
	if built == nil {
		t.Fatal("Editor factory was not called")
	}
	if built.val != "1" {
		t.Fatalf("custom editor seeded %q, want %q", built.val, "1")
	}
	if !built.focused {
		t.Fatal("custom editor was not focused")
	}
	if built.submit == nil {
		t.Fatal("custom editor submit callback not registered")
	}

	// Draw routes to the custom editor.
	buf := makeSurface(100, 100)
	tb.Draw(newP(buf, 100), DefaultLight())
	if !built.drawn {
		t.Fatal("custom editor Draw was not called")
	}

	// Change the value and fire the editor's submit -> commit.
	built.val = "custom"
	built.submit()
	if tb.Rows[0][1] != "custom" {
		t.Fatalf("Rows[0][1]=%q after custom commit, want %q", tb.Rows[0][1], "custom")
	}
	if got != "custom" {
		t.Fatalf("OnCellEdit value=%q, want %q", got, "custom")
	}
}

// --- Validation -------------------------------------------------------------

// TestTableCellEditValidationReject: a value failing the column's Validate rule
// is rejected -- Rows untouched, editor stays open, EditError set,
// OnCellEditRejected fired -- and a subsequent valid value commits and clears
// the error.
func TestTableCellEditValidationReject(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "Name", Width: 50},
		{Title: "Value", Width: 50, Editable: true, Validate: Required("required")},
	}, [][]string{{"a", "1"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	var rejRow, rejCol int
	var rejVal string
	var rejErr error
	rejects, commits := 0, 0
	tb.OnCellEditRejected = func(row, col int, value string, err error) {
		rejRow, rejCol, rejVal, rejErr, rejects = row, col, value, err, rejects+1
	}
	tb.OnCellEdit = func(int, int, string) { commits++ }

	tb.BeginEdit(0, 1)
	tb.editor.SetCellValue("") // empty -> Required fails
	tb.CommitEdit()

	if commits != 0 {
		t.Fatalf("commits=%d after rejected edit, want 0", commits)
	}
	if tb.Rows[0][1] != "1" {
		t.Fatalf("Rows[0][1]=%q after reject, want unchanged %q", tb.Rows[0][1], "1")
	}
	if _, _, editing := tb.Editing(); !editing {
		t.Fatal("editor must stay open after a rejected commit")
	}
	if tb.EditError() == nil || tb.EditError().Error() != "required" {
		t.Fatalf("EditError()=%v, want %q", tb.EditError(), "required")
	}
	if rejects != 1 || rejRow != 0 || rejCol != 1 || rejVal != "" || rejErr == nil {
		t.Fatalf("OnCellEditRejected=(%d,%d,%q,%v,calls %d), want (0,1,\"\",err,1)", rejRow, rejCol, rejVal, rejErr, rejects)
	}

	// Now supply a valid value: it commits and clears the error.
	tb.editor.SetCellValue("9")
	tb.CommitEdit()
	if commits != 1 || tb.Rows[0][1] != "9" {
		t.Fatalf("after fix: commits=%d Rows[0][1]=%q, want 1 and %q", commits, tb.Rows[0][1], "9")
	}
	if tb.EditError() != nil {
		t.Fatalf("EditError()=%v after successful commit, want nil", tb.EditError())
	}
	if _, _, editing := tb.Editing(); editing {
		t.Fatal("editor must close after a successful commit")
	}
}

// TestTableCellEditErrorBorder: a rejected edit rings the editing cell in the
// error colour. Assert the exact border pixels on the cell rect.
func TestTableCellEditErrorBorder(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "Name", Width: 50},
		{Title: "Value", Width: 50, Editable: true, Validate: Required("required")},
	}, [][]string{{"a", "1"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	tb.BeginEdit(0, 1)
	tb.editor.SetCellValue("")
	tb.CommitEdit() // rejected -> editErr set

	const w, h = 100, 100
	buf := makeSurface(w, h)
	tb.Draw(newP(buf, w), DefaultLight())

	// The editor cell is col 1 (X in [50,100)), row 0 (Y == TableHeaderHeight).
	// strokeRect paints the top edge at y == TableHeaderHeight and the left edge
	// at x == 50. Assert the exact error colour on both.
	rc := tb.cellRect(0, 1)
	top := pixelAt(buf, w, rc.X+5, rc.Y)
	if top != tableErrorBorder {
		t.Fatalf("top border pixel=%v, want error colour %v", top, tableErrorBorder)
	}
	left := pixelAt(buf, w, rc.X, rc.Y+5)
	if left != tableErrorBorder {
		t.Fatalf("left border pixel=%v, want error colour %v", left, tableErrorBorder)
	}
}

// TestTableCancelEditClearsError: CancelEdit discards a rejected edit and its
// error.
func TestTableCancelEditClearsError(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "V", Width: 50, Editable: true, Validate: Required("required")},
	}, [][]string{{"1"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	tb.BeginEdit(0, 0)
	tb.editor.SetCellValue("")
	tb.CommitEdit()
	if tb.EditError() == nil {
		t.Fatal("precondition: expected an edit error")
	}
	tb.CancelEdit()
	if tb.EditError() != nil {
		t.Fatalf("EditError()=%v after CancelEdit, want nil", tb.EditError())
	}
	if _, _, editing := tb.Editing(); editing {
		t.Fatal("editor must be closed after CancelEdit")
	}
}

// --- Sorting ----------------------------------------------------------------

// TestDefaultCellCompare covers every ordering branch: numeric less/greater/
// equal and the lexicographic fallback for non-numeric cells.
func TestDefaultCellCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int // sign
	}{
		{"1", "2", -1},
		{"2", "1", 1},
		{"1", "1", 0},
		{"9", "10", -1}, // numeric, not lexicographic
		{"apple", "banana", -1},
		{"b", "a", 1},
		{"x", "x", 0},
	}
	for _, c := range cases {
		got := defaultCellCompare(c.a, c.b)
		if (got < 0) != (c.want < 0) || (got > 0) != (c.want > 0) || (got == 0) != (c.want == 0) {
			t.Fatalf("defaultCellCompare(%q,%q)=%d, want sign %d", c.a, c.b, got, c.want)
		}
	}
}

// TestTableSortByColumnNumeric: a numeric column sorts by value (not string
// order) ascending, and re-sorting descending reverses it.
func TestTableSortByColumnNumeric(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "Name"}, {Title: "Value"}}, [][]string{
		{"x", "10"}, {"y", "9"}, {"z", "2"},
	})
	tb.SortByColumn(1, true)
	if got := colValues(tb, 0); !strSliceEq(got, []string{"z", "y", "x"}) {
		t.Fatalf("asc numeric order=%v, want [z y x]", got)
	}
	if tb.SortColumn().Get() != 1 || !tb.SortAsc().Get() {
		t.Fatalf("SortColumn=%d SortAsc=%v, want 1,true", tb.SortColumn().Get(), tb.SortAsc().Get())
	}
	tb.SortByColumn(1, false)
	if got := colValues(tb, 0); !strSliceEq(got, []string{"x", "y", "z"}) {
		t.Fatalf("desc numeric order=%v, want [x y z]", got)
	}
	if tb.SortAsc().Get() {
		t.Fatal("SortAsc must be false after descending sort")
	}
}

// TestTableSortByColumnCustomComparator: a column's Comparator overrides the
// default ordering (here: by rune length).
func TestTableSortByColumnCustomComparator(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "W", Comparator: func(a, b string) int { return len([]rune(a)) - len([]rune(b)) }},
	}, [][]string{{"aaa"}, {"a"}, {"aa"}})
	tb.SortByColumn(0, true)
	if got := colValues(tb, 0); !strSliceEq(got, []string{"a", "aa", "aaa"}) {
		t.Fatalf("custom comparator order=%v, want [a aa aaa]", got)
	}
}

// TestTableSortByColumnOutOfRange: an out-of-range column is a no-op.
func TestTableSortByColumnOutOfRange(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "A"}}, [][]string{{"2"}, {"1"}})
	tb.SortByColumn(5, true)
	if got := colValues(tb, 0); !strSliceEq(got, []string{"2", "1"}) {
		t.Fatalf("out-of-range sort changed rows to %v", got)
	}
	if tb.SortColumn().Get() != -1 {
		t.Fatalf("SortColumn=%d after no-op sort, want -1", tb.SortColumn().Get())
	}
}

// TestTableSortRemapsState: sorting follows Selected, the multi-row selection
// set and the expanded-row set to their rows' new positions, and copes with
// ragged rows (cellAt returns "").
func TestTableSortRemapsState(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "K"}}, [][]string{
		{"c"}, {"a"}, {}, // row 2 ragged -> key ""
	})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	tb.RowDetail = func(int) string { return "detail" }
	tb.Selected().Set(0) // row "c"
	tb.selectedRows = map[int]bool{0: true, 1: true}
	tb.expanded = map[int]bool{1: true} // row "a"

	tb.SortByColumn(0, true) // "" < "a" < "c" -> order rows 2,1,0
	if got := colValues(tb, 0); !strSliceEq(got, []string{"", "a", "c"}) {
		t.Fatalf("ragged sort order=%v, want ['' a c]", got)
	}
	// Old row 0 ("c") is now at index 2; old row 1 ("a") at index 1.
	if tb.Selected().Get() != 2 {
		t.Fatalf("Selected=%d after sort, want 2 (followed row 'c')", tb.Selected().Get())
	}
	if !tb.selectedRows[2] || !tb.selectedRows[1] || len(tb.selectedRows) != 2 {
		t.Fatalf("selectedRows=%v, want {1,2}", tb.selectedRows)
	}
	if !tb.expanded[1] || len(tb.expanded) != 1 {
		t.Fatalf("expanded=%v, want {1}", tb.expanded)
	}
}

// TestTableSelfSortHeaderClick: with SelfSort set a Sortable header click sorts
// Rows in place and fires OnSort; re-clicking the same column reverses it.
// Without SelfSort the same click only sets the indicator and fires OnSort,
// never touching Rows.
func TestTableSelfSortHeaderClick(t *testing.T) {
	newTbl := func() *Table {
		tb := NewTable([]TableColumn{{Title: "N", Width: 50, Sortable: true}}, [][]string{
			{"3"}, {"1"}, {"2"},
		})
		tb.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 100})
		return tb
	}
	headerClick := Event{Kind: EventClick, X: 10, Y: 2}

	// SelfSort on: rows reorder ascending, then descending.
	tb := newTbl()
	tb.SelfSort = true
	sorts := 0
	var lastAsc bool
	tb.OnSort = func(_ int, asc bool) { sorts++; lastAsc = asc }
	tb.OnEvent(headerClick)
	if got := colValues(tb, 0); !strSliceEq(got, []string{"1", "2", "3"}) {
		t.Fatalf("SelfSort asc order=%v, want [1 2 3]", got)
	}
	if sorts != 1 || !lastAsc {
		t.Fatalf("after 1st click sorts=%d lastAsc=%v, want 1,true", sorts, lastAsc)
	}
	tb.OnEvent(headerClick)
	if got := colValues(tb, 0); !strSliceEq(got, []string{"3", "2", "1"}) {
		t.Fatalf("SelfSort desc order=%v, want [3 2 1]", got)
	}
	if sorts != 2 || lastAsc {
		t.Fatalf("after 2nd click sorts=%d lastAsc=%v, want 2,false", sorts, lastAsc)
	}

	// SelfSort off: rows unchanged, indicator + callback still fire.
	off := newTbl()
	fired := 0
	off.OnSort = func(int, bool) { fired++ }
	off.OnEvent(headerClick)
	if got := colValues(off, 0); !strSliceEq(got, []string{"3", "1", "2"}) {
		t.Fatalf("non-SelfSort reordered rows to %v", got)
	}
	if off.SortColumn().Get() != 0 || !off.SortAsc().Get() || fired != 1 {
		t.Fatalf("non-SelfSort SortColumn=%d SortAsc=%v fired=%d, want 0,true,1", off.SortColumn().Get(), off.SortAsc().Get(), fired)
	}
}

// --- Grouping ---------------------------------------------------------------

// TestTableGroupKeyHeaders: a GroupKey buckets rows by a derived key; the group
// headers carry the exact keys and member counts, in order.
func TestTableGroupKeyHeaders(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "Fruit", GroupKey: firstLetter}}, [][]string{
		{"Apple"}, {"Avocado"}, {"Banana"}, {"Blueberry"}, {"Cherry"},
	})
	tb.GroupBy = 0

	headers := groupHeaders(tb)
	wantKeys := []string{"A", "B", "C"}
	wantCounts := []int{2, 2, 1}
	if len(headers) != 3 {
		t.Fatalf("got %d group headers, want 3: %+v", len(headers), headers)
	}
	for i, h := range headers {
		if h.group != wantKeys[i] || h.count != wantCounts[i] {
			t.Fatalf("header %d = (%q,%d), want (%q,%d)", i, h.group, h.count, wantKeys[i], wantCounts[i])
		}
	}
}

// TestTableArrangeGroups: ArrangeGroups reorders out-of-order rows so a
// GroupKey's groups become contiguous and key-ordered, stably within a group.
func TestTableArrangeGroups(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "Fruit", GroupKey: firstLetter}}, [][]string{
		{"Banana"}, {"Apple"}, {"Cherry"}, {"Avocado"},
	})
	tb.GroupBy = 0
	tb.ArrangeGroups()
	if got := colValues(tb, 0); !strSliceEq(got, []string{"Apple", "Avocado", "Banana", "Cherry"}) {
		t.Fatalf("ArrangeGroups order=%v, want [Apple Avocado Banana Cherry]", got)
	}
	// Groups are now contiguous: A(2), B(1), C(1).
	headers := groupHeaders(tb)
	if len(headers) != 3 || headers[0].count != 2 || headers[1].count != 1 || headers[2].count != 1 {
		t.Fatalf("post-arrange headers=%+v, want counts 2,1,1", headers)
	}
}

// TestTableArrangeGroupsNotGrouped: ArrangeGroups is a no-op when grouping is
// off.
func TestTableArrangeGroupsNotGrouped(t *testing.T) {
	tb := NewTable([]TableColumn{{Title: "F"}}, [][]string{{"b"}, {"a"}})
	// GroupBy defaults to -1 (ungrouped).
	tb.ArrangeGroups()
	if got := colValues(tb, 0); !strSliceEq(got, []string{"b", "a"}) {
		t.Fatalf("ArrangeGroups reordered an ungrouped table to %v", got)
	}
}

// --- Custom aggregate -------------------------------------------------------

// TestTableAggregateFunc: a column's AggregateFunc overrides the built-in
// reduction for both a per-group range and the grand total, receiving one entry
// per row (ragged rows contribute "").
func TestTableAggregateFunc(t *testing.T) {
	tb := NewTable([]TableColumn{
		{Title: "Name"},
		{Title: "Qty", Aggregate: AggregateSum, AggregateFunc: func(cells []string) string {
			// A custom reducer: "<count>/<blanks>" (rows seen / ragged blanks).
			blanks := 0
			for _, c := range cells {
				if c == "" {
					blanks++
				}
			}
			return strconv.Itoa(len(cells)) + "/" + strconv.Itoa(blanks)
		}},
	}, [][]string{{"a", "1"}, {"a", "2"}, {"b"}}) // row 2 ragged in col 1

	// Group "a" range [0,2): 2 cells, 0 blanks.
	if got := tb.aggregate(AggregateSum, 1, 0, 2); got != "2/0" {
		t.Fatalf("group aggregate=%q, want %q", got, "2/0")
	}
	// Grand total [0,3): 3 cells, 1 ragged blank.
	if got := tb.aggregate(AggregateSum, 1, 0, 3); got != "3/1" {
		t.Fatalf("grand-total aggregate=%q, want %q", got, "3/1")
	}
}

// --- test helpers -----------------------------------------------------------

func firstLetter(cell string) string {
	if cell == "" {
		return ""
	}
	return cell[:1]
}

// colValues collects column c across every row in current order.
func colValues(t *Table, c int) []string {
	out := make([]string, len(t.Rows))
	for i := range t.Rows {
		if c < len(t.Rows[i]) {
			out[i] = t.Rows[i][c]
		}
	}
	return out
}

func strSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// groupHeaders returns just the header lines of the current line model.
func groupHeaders(t *Table) []tableLine {
	var out []tableLine
	for _, ln := range t.lines() {
		if ln.header {
			out = append(out, ln)
		}
	}
	return out
}
