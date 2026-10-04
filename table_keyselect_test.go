package toolkit

import (
	"fmt"
	"testing"
)

// newKeyTable builds a 10-row multi-select table.
func newKeyTable() *Table {
	rows := make([][]string, 10)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("row%d", i)}
	}
	tb := NewTable([]TableColumn{{Title: "name"}}, rows)
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: TableHeaderHeight + 5*TableRowHeight})
	return tb
}

func tkey(tb *Table, code string, mods ...string) {
	ev := Event{Kind: EventKeyDown, Code: code}
	for _, m := range mods {
		switch m {
		case "shift":
			ev.Shift = true
		case "ctrl":
			ev.Ctrl = true
		case "meta":
			ev.Meta = true
		}
	}
	tb.OnEvent(ev)
}

func rowsOf(tb *Table) string { return fmt.Sprint(tb.SelectedRows()) }

// TestShiftPageAndEndExtendATableInsteadOfDestroyingTheSelection.
//
// ⛔ Measured against the previous code: only ArrowUp and ArrowDown extended.
// Shift-PageDown fell through to the plain roving branch, which in MultiSelect
// mode calls SetRowSelection -- so the gesture did not merely fail to extend,
// it COLLAPSED a selection of rows 2, 3 and 4 down to the single row 5,
// destroying what the user had just built with the two keys that did work.
func TestShiftPageAndEndExtendATableInsteadOfDestroyingTheSelection(t *testing.T) {
	for _, c := range []struct{ code, want string }{
		{"PageDown", "[0 1 2 3 4 5]"},
		{"End", "[0 1 2 3 4 5 6 7 8 9]"},
		{"Home", "[0]"},
	} {
		tb := newKeyTable()
		tb.Selected().Set(0)
		tb.SetRowSelection(0)
		tkey(tb, c.code, "shift")
		if got := rowsOf(tb); got != c.want {
			t.Errorf("Shift-%s: selection %s, want %s", c.code, got, c.want)
		}
		if tb.Selected().Get() != 0 {
			t.Errorf("Shift-%s moved the anchor to %d, want 0", c.code, tb.Selected().Get())
		}
	}
}

// TestATableExtensionShrinksBackAndCrossesItsAnchor: the two halves the old
// accumulate-only code could not do.
func TestATableExtensionShrinksBackAndCrossesItsAnchor(t *testing.T) {
	tb := newKeyTable()
	tb.Selected().Set(4)
	tb.SetRowSelection(4)
	for i := 0; i < 3; i++ {
		tkey(tb, "ArrowDown", "shift")
	}
	if got := rowsOf(tb); got != "[4 5 6 7]" {
		t.Fatalf("after three Shift-ArrowDown: %s, want [4 5 6 7]", got)
	}
	for i := 0; i < 2; i++ {
		tkey(tb, "ArrowUp", "shift")
	}
	if got := rowsOf(tb); got != "[4 5]" {
		t.Fatalf("shrinking back: %s, want [4 5]", got)
	}
	// Past the anchor the range flips sides, and the anchor is still 4.
	for i := 0; i < 3; i++ {
		tkey(tb, "ArrowUp", "shift")
	}
	if got := rowsOf(tb); got != "[2 3 4]" {
		t.Fatalf("crossing the anchor: %s, want [2 3 4]", got)
	}
	if tb.Selected().Get() != 4 {
		t.Fatalf("anchor = %d after crossing, want 4", tb.Selected().Get())
	}
}

// TestATableExtensionScrollsToItsMovingEnd: the anchor is already on screen;
// the row being reached for is the one that may not be.
func TestATableExtensionScrollsToItsMovingEnd(t *testing.T) {
	tb := newKeyTable()
	tb.Selected().Set(0)
	tb.SetRowSelection(0)
	for i := 0; i < 7; i++ {
		tkey(tb, "ArrowDown", "shift")
	}
	if got := rowsOf(tb); got != "[0 1 2 3 4 5 6 7]" {
		t.Fatalf("selection %s, want rows 0..7", got)
	}
	vis := tb.bodyVisibleRows()
	if top := tb.ScrollRow().Get(); top != 7-vis+1 {
		t.Fatalf("ScrollRow = %d, want %d so that row 7 is the last visible one",
			top, 7-vis+1)
	}
}

// TestATableExtensionScrollsUpToReachBackwards: the moving end travels in both
// directions, and a Shift-Home from a scrolled position must bring the top of
// the range into view, not leave the user extending into rows above the window.
func TestATableExtensionScrollsUpToReachBackwards(t *testing.T) {
	tb := newKeyTable()
	tb.Selected().Set(8)
	tb.SetRowSelection(8)
	tb.ScrollTo(5)
	if tb.ScrollRow().Get() != 5 {
		t.Fatalf("ScrollRow = %d, want the window moved down to 5", tb.ScrollRow().Get())
	}
	tkey(tb, "Home", "shift")
	if got := rowsOf(tb); got != "[0 1 2 3 4 5 6 7 8]" {
		t.Fatalf("selection %s, want rows 0..8", got)
	}
	if top := tb.ScrollRow().Get(); top != 0 {
		t.Fatalf("ScrollRow = %d, want 0 so that row 0 is visible", top)
	}
}

// TestATableWithNoRoomToScrollStillExtends: a table laid out with no visible
// body rows -- before a first layout, or squeezed to nothing -- must still
// build the selection rather than panic or refuse.
func TestATableWithNoRoomToScrollStillExtends(t *testing.T) {
	rows := [][]string{{"0"}, {"1"}, {"2"}}
	tb := NewTable([]TableColumn{{Title: "a"}}, rows)
	tb.MultiSelect = true
	tb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 0}) // no body at all
	tb.Selected().Set(0)
	tb.SetRowSelection(0)
	tkey(tb, "ArrowDown", "shift")
	if got := rowsOf(tb); got != "[0 1]" {
		t.Fatalf("selection %s, want [0 1]", got)
	}
	if tb.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollRow = %d; there is nothing to scroll", tb.ScrollRow().Get())
	}
}

// TestATableAnswersBothCommandModifiersForSelectAll.
func TestATableAnswersBothCommandModifiersForSelectAll(t *testing.T) {
	all := "[0 1 2 3 4 5 6 7 8 9]"
	for _, mod := range []string{"ctrl", "meta"} {
		for _, code := range []string{"a", "A"} {
			tb := newKeyTable()
			tb.Selected().Set(3)
			tkey(tb, code, mod)
			if got := rowsOf(tb); got != all {
				t.Errorf("%s+%q: selection %s, want every row", mod, code, got)
			}
			if tb.Selected().Get() != 3 {
				t.Errorf("%s+%q moved the anchor to %d", mod, code, tb.Selected().Get())
			}
		}
	}
	// Plain "a" is a letter, not a command.
	tb := newKeyTable()
	tb.SetRowSelection(1)
	tkey(tb, "a")
	if got := rowsOf(tb); got != "[1]" {
		t.Fatalf("plain \"a\": selection %s, want [1] unchanged", got)
	}
	// Without MultiSelect the chord does nothing either.
	single := newKeyTable()
	single.MultiSelect = false
	tkey(single, "a", "ctrl")
	if n := len(single.SelectedRows()); n != 0 {
		t.Fatalf("Ctrl+A on a single-selection table selected %d rows", n)
	}
	// An empty table ends up with an empty selection, not a panic.
	empty := NewTable([]TableColumn{{Title: "a"}}, nil)
	empty.MultiSelect = true
	empty.SelectAllRows()
	if n := len(empty.SelectedRows()); n != 0 {
		t.Fatalf("empty SelectAllRows: %d rows", n)
	}
}

// TestCommandSpaceTogglesATableRowWithoutActivatingIt.
//
// ⛔ Activating a table row COLLAPSES the selection to the cursor row
// (activateCursor calls SetRowSelection). So a Ctrl/⌘+Space that reached the
// activation path would not merely fail to toggle: it would throw away every
// other row the user had picked -- which is the opposite of what the gesture is
// for. Enter must keep activating, and must keep opening an inline editor where
// one is configured, so only Space is diverted.
func TestCommandSpaceTogglesATableRowWithoutActivatingIt(t *testing.T) {
	for _, code := range []string{" ", "Space"} {
		for _, mod := range []string{"ctrl", "meta"} {
			tb := newKeyTable()
			tb.Selected().Set(4)
			tb.SetRowSelection(1)

			tkey(tb, code, mod)
			// Toggled: {1,4}. Had it activated, the set would be exactly {4}.
			if got := rowsOf(tb); got != "[1 4]" {
				t.Errorf("%s+%q: selection %s, want [1 4]", mod, code, got)
			}
			tkey(tb, code, mod) // again removes it
			if got := rowsOf(tb); got != "[1]" {
				t.Errorf("%s+%q twice: selection %s, want [1]", mod, code, got)
			}
		}
	}
	// Ctrl+Enter still activates: only Space is the toggle.
	tb := newKeyTable()
	tb.Selected().Set(4)
	tb.SetRowSelection(1)
	tkey(tb, "Enter", "ctrl")
	if got := rowsOf(tb); got != "[4]" {
		t.Fatalf("Ctrl+Enter: selection %s, want [4] -- it must still activate", got)
	}
	// And a plain Space activates.
	tb2 := newKeyTable()
	tb2.Selected().Set(2)
	tb2.SetRowSelection(1)
	tkey(tb2, " ")
	if got := rowsOf(tb2); got != "[2]" {
		t.Fatalf("plain Space: selection %s, want [2]", got)
	}
}

// TestACommandClickTogglesATableRow.
//
// ⛔ Measured against the previous code: a ⌘-click fell through to the default
// branch and acted as a PLAIN click, replacing the selection. On macOS, where
// Ctrl-click is the secondary click, that left no way at all to add a row with
// the mouse.
func TestACommandClickTogglesATableRow(t *testing.T) {
	for _, mod := range []string{"ctrl", "meta"} {
		tb := newKeyTable()
		tb.SetRowSelection(0)
		tb.Selected().Set(0)
		ev := Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + TableRowHeight + 2}
		if mod == "ctrl" {
			ev.Ctrl = true
		} else {
			ev.Meta = true
		}
		tb.OnEvent(ev)
		if got := rowsOf(tb); got != "[0 1]" {
			t.Errorf("%s-click: selection %s, want [0 1]", mod, got)
		}
	}
}

// TestAShiftClickHandsATableTheMovingEnd: the mouse and the keyboard drive one
// selection.
func TestAShiftClickHandsATableTheMovingEnd(t *testing.T) {
	tb := newKeyTable()
	tb.Selected().Set(1)
	tb.SetRowSelection(1)
	tb.OnEvent(Event{Kind: EventClick, X: 10,
		Y: TableHeaderHeight + 3*TableRowHeight + 2, Shift: true})
	if got := rowsOf(tb); got != "[1 2 3]" {
		t.Fatalf("Shift-click: selection %s, want [1 2 3]", got)
	}
	tkey(tb, "ArrowDown", "shift")
	if got := rowsOf(tb); got != "[1 2 3 4]" {
		t.Fatalf("Shift-arrow after Shift-click: selection %s, want [1 2 3 4]", got)
	}
}

// TestAPlainTableMoveForgetsTheExtension: the moving end must not survive a
// gesture that moves the anchor.
func TestAPlainTableMoveForgetsTheExtension(t *testing.T) {
	tb := newKeyTable()
	tb.Selected().Set(0)
	tb.SetRowSelection(0)
	for i := 0; i < 3; i++ {
		tkey(tb, "ArrowDown", "shift")
	}
	tkey(tb, "ArrowDown") // plain
	if got := rowsOf(tb); got != "[1]" {
		t.Fatalf("after a plain move: selection %s, want [1]", got)
	}
	tkey(tb, "ArrowDown", "shift")
	if got := rowsOf(tb); got != "[1 2]" {
		t.Fatalf("the forgotten end leaked: selection %s, want [1 2]", got)
	}
	// A plain click does the same.
	tkey(tb, "ArrowDown", "shift")
	tb.OnEvent(Event{Kind: EventClick, X: 10, Y: TableHeaderHeight + 2})
	tkey(tb, "ArrowDown", "shift")
	if got := rowsOf(tb); got != "[0 1]" {
		t.Fatalf("after a plain click: selection %s, want [0 1]", got)
	}
	// So does a toggle click.
	tb.Selected().Set(5)
	tb.SetRowSelection(5)
	tkey(tb, "ArrowDown", "shift") // end at 6
	tb.OnEvent(Event{Kind: EventClick, X: 10,
		Y: TableHeaderHeight + 2*TableRowHeight + 2, Meta: true})
	tb.Selected().Set(5)
	tkey(tb, "ArrowDown", "shift")
	if got := rowsOf(tb); got != "[5 6]" {
		t.Fatalf("after a toggle click: selection %s, want [5 6]", got)
	}
}

// TestADisabledTableIgnoresTheMultiSelectKeys.
func TestADisabledTableIgnoresTheMultiSelectKeys(t *testing.T) {
	tb := newKeyTable()
	tb.Disabled().Set(true)
	tb.Selected().Set(2)
	tb.SetRowSelection(2)
	for _, k := range [][]string{{"ArrowDown", "shift"}, {"a", "ctrl"}, {" ", "meta"}} {
		tkey(tb, k[0], k[1])
	}
	if got := rowsOf(tb); got != "[2]" {
		t.Fatalf("disabled table: selection %s, want [2] untouched", got)
	}
}
