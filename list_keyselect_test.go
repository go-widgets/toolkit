package toolkit

import (
	"fmt"
	"testing"
)

// newKeyList builds a 10-row multi-select list showing 5 rows at a time, so a
// test can tell a selection change from a scroll.
func newKeyList() *ListBox {
	items := make([]string, 10)
	for i := range items {
		items[i] = fmt.Sprintf("row%d", i)
	}
	l := NewListBox(items)
	l.MultiSelect = true
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 100}) // 5 visible rows
	return l
}

// key sends one key press with the given modifiers.
func key(l *ListBox, code string, mods ...string) {
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
	l.OnEvent(ev)
}

func selectionOf(l *ListBox) string { return fmt.Sprint(l.SelectedIndices()) }

// TestShiftArrowExtendsTheSelectionFromTheAnchor.
//
// ⛔ The extension's moving end must be computed from itself, not from the
// anchor. Computing it from Selected -- which a Shift gesture deliberately
// leaves in place -- makes every Shift-ArrowDown extend exactly one row from
// the anchor, so the selection reaches two rows and never grows again.
func TestShiftArrowExtendsTheSelectionFromTheAnchor(t *testing.T) {
	l := newKeyList()
	l.Selected().Set(2)
	l.SetSelection(2)

	for i, want := range []string{"[2 3]", "[2 3 4]", "[2 3 4 5]"} {
		key(l, "ArrowDown", "shift")
		if got := selectionOf(l); got != want {
			t.Fatalf("after %d Shift-ArrowDown: selection %s, want %s", i+1, got, want)
		}
		if l.Selected().Get() != 2 {
			t.Fatalf("after %d Shift-ArrowDown: anchor moved to %d, want 2",
				i+1, l.Selected().Get())
		}
	}
	// Shrinking back up walks the same end in reverse.
	key(l, "ArrowUp", "shift")
	if got := selectionOf(l); got != "[2 3 4]" {
		t.Fatalf("Shift-ArrowUp: selection %s, want [2 3 4]", got)
	}
	// And past the anchor the range flips sides without losing the anchor.
	for i := 0; i < 3; i++ {
		key(l, "ArrowUp", "shift")
	}
	if got := selectionOf(l); got != "[1 2]" {
		t.Fatalf("crossing the anchor: selection %s, want [1 2]", got)
	}
	if l.Selected().Get() != 2 {
		t.Fatalf("anchor = %d after crossing, want 2", l.Selected().Get())
	}
}

// TestShiftPageAndEndExtendTheSelectionToo: every key rovingIndex understands
// extends, not only the arrows -- a list of a thousand rows is unusable one row
// at a time.
func TestShiftPageAndEndExtendTheSelectionToo(t *testing.T) {
	for _, c := range []struct{ code, want string }{
		{"PageDown", "[0 1 2 3 4 5]"},
		{"End", "[0 1 2 3 4 5 6 7 8 9]"},
		{"Home", "[0]"},
	} {
		l := newKeyList()
		l.Selected().Set(0)
		l.SetSelection(0)
		key(l, c.code, "shift")
		if got := selectionOf(l); got != c.want {
			t.Errorf("Shift-%s: selection %s, want %s", c.code, got, c.want)
		}
	}
}

// TestAnExtensionScrollsToItsMovingEndNotToTheAnchor.
//
// ⛔ The anchor is already on screen; the row being reached for is the one that
// may not be. Scrolling to the anchor leaves the user extending into rows they
// cannot see.
func TestAnExtensionScrollsToItsMovingEndNotToTheAnchor(t *testing.T) {
	l := newKeyList()
	l.Selected().Set(0)
	l.SetSelection(0)
	for i := 0; i < 7; i++ {
		key(l, "ArrowDown", "shift")
	}
	if got := selectionOf(l); got != "[0 1 2 3 4 5 6 7]" {
		t.Fatalf("selection %s, want rows 0..7", got)
	}
	// Row 7 is the 8th row of a 5-row window: the window must have moved to
	// show it, and the anchor (row 0) must have scrolled off the top.
	if top := l.ScrollRow().Get(); top != 3 {
		t.Fatalf("ScrollRow = %d, want 3 so that row 7 is the last visible one", top)
	}
}

// TestShiftArrowWithNoAnchorYetTakesTheFirstRow: a list nothing has touched has
// Selected at -1, and a first Shift-ArrowDown must still select something.
func TestShiftArrowWithNoAnchorYetTakesTheFirstRow(t *testing.T) {
	l := newKeyList()
	key(l, "ArrowDown", "shift")
	if got := selectionOf(l); got != "[0 1]" {
		t.Fatalf("selection %s, want [0 1]", got)
	}
	if l.Selected().Get() != 0 {
		t.Fatalf("anchor = %d, want 0", l.Selected().Get())
	}

	// An empty list has nothing to anchor to and must not move Selected.
	e := NewListBox(nil)
	e.MultiSelect = true
	e.RowHeight = 20
	e.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 100})
	key(e, "ArrowDown", "shift")
	if e.Selected().Get() != -1 || len(e.SelectedIndices()) != 0 {
		t.Fatalf("empty list: anchor %d, selection %v; want -1 and none",
			e.Selected().Get(), e.SelectedIndices())
	}
}

// TestShiftWithANonMovementKeyChangesNothing covers the branch where Shift is
// held but the key means nothing to rovingIndex.
func TestShiftWithANonMovementKeyChangesNothing(t *testing.T) {
	l := newKeyList()
	l.Selected().Set(2)
	l.SetSelection(2)
	key(l, "F5", "shift")
	if got := selectionOf(l); got != "[2]" {
		t.Fatalf("selection %s, want [2] unchanged", got)
	}
}

// TestSelectAllAnswersBothCommandModifiers.
//
// ⛔ A widget cannot tell which platform it is painting on, so it answers to
// Ctrl AND to ⌘: on macOS the command modifier is ⌘ and a Ctrl chord means
// something else entirely.
func TestSelectAllAnswersBothCommandModifiers(t *testing.T) {
	all := "[0 1 2 3 4 5 6 7 8 9]"
	for _, mod := range []string{"ctrl", "meta"} {
		for _, code := range []string{"a", "A"} {
			l := newKeyList()
			l.Selected().Set(3)
			key(l, code, mod)
			if got := selectionOf(l); got != all {
				t.Errorf("%s+%q: selection %s, want every row", mod, code, got)
			}
			if l.Selected().Get() != 3 {
				t.Errorf("%s+%q moved the anchor to %d, want 3 left alone",
					mod, code, l.Selected().Get())
			}
		}
	}
	// Without the modifier, "a" is just a letter and must not select anything.
	l := newKeyList()
	l.SetSelection(1)
	key(l, "a")
	if got := selectionOf(l); got != "[1]" {
		t.Fatalf("plain \"a\": selection %s, want [1] unchanged", got)
	}
	// And an empty list ends up with an empty selection, not a panic.
	e := NewListBox(nil)
	e.MultiSelect = true
	e.SelectAll()
	if len(e.SelectedIndices()) != 0 {
		t.Fatalf("empty list SelectAll: %v, want none", e.SelectedIndices())
	}
}

// TestCommandSpaceTogglesTheCursorRowWithoutActivatingIt.
//
// ⛔ Space activates. Ctrl/⌘+Space must NOT: it is the only keyboard gesture
// that adds one far-apart row to a selection, and activating the row (opening
// it, running it) while merely picking it is the kind of surprise that loses
// work.
func TestCommandSpaceTogglesTheCursorRowWithoutActivatingIt(t *testing.T) {
	for _, code := range []string{" ", "Space"} {
		for _, mod := range []string{"ctrl", "meta"} {
			l := newKeyList()
			activated := -1
			l.OnActivate = func(i int) { activated = i }
			l.Selected().Set(4)
			l.SetSelection(1)

			key(l, code, mod)
			if got := selectionOf(l); got != "[1 4]" {
				t.Errorf("%s+%q: selection %s, want [1 4]", mod, code, got)
			}
			if activated != -1 {
				t.Errorf("%s+%q activated row %d; it must only toggle", mod, code, activated)
			}
			// Again on the same row removes it.
			key(l, code, mod)
			if got := selectionOf(l); got != "[1]" {
				t.Errorf("%s+%q twice: selection %s, want [1]", mod, code, got)
			}
		}
	}
	// A plain Space still activates.
	l := newKeyList()
	activated := -1
	l.OnActivate = func(i int) { activated = i }
	l.Selected().Set(4)
	key(l, " ")
	if activated != 4 {
		t.Fatalf("plain Space activated %d, want 4", activated)
	}
}

// TestAPlainMoveStillCollapsesTheSelectionAndForgetsTheExtension: the
// extension's end must not survive a gesture that moves the anchor, or the next
// Shift-arrow would extend from a row the user has left.
func TestAPlainMoveStillCollapsesTheSelectionAndForgetsTheExtension(t *testing.T) {
	l := newKeyList()
	l.Selected().Set(0)
	l.SetSelection(0)
	for i := 0; i < 3; i++ {
		key(l, "ArrowDown", "shift") // selection 0..3, moving end at 3
	}
	key(l, "ArrowDown") // plain: collapses to row 1 and forgets the end
	if got := selectionOf(l); got != "[1]" {
		t.Fatalf("after a plain move: selection %s, want [1]", got)
	}
	key(l, "ArrowDown", "shift")
	if got := selectionOf(l); got != "[1 2]" {
		t.Fatalf("the forgotten end leaked: selection %s, want [1 2]", got)
	}
}

// TestACommandClickTogglesTheSameWayACtrlClickDoes.
//
// ⛔ On macOS Ctrl-click IS the secondary click -- it opens a context menu --
// so it cannot also mean "add this row to the selection". ⌘-click is that
// gesture there, and a widget that consults only ev.Ctrl is unreachable with a
// Mac mouse.
func TestACommandClickTogglesTheSameWayACtrlClickDoes(t *testing.T) {
	for _, mod := range []string{"ctrl", "meta"} {
		l := newKeyList()
		l.SetSelection(0)
		l.Selected().Set(0)
		ev := Event{Kind: EventClick, Y: 50} // row 2
		if mod == "ctrl" {
			ev.Ctrl = true
		} else {
			ev.Meta = true
		}
		l.OnEvent(ev)
		if got := selectionOf(l); got != "[0 2]" {
			t.Errorf("%s-click: selection %s, want [0 2]", mod, got)
		}
		if l.Selected().Get() != 2 {
			t.Errorf("%s-click: anchor %d, want 2", mod, l.Selected().Get())
		}
	}
}

// TestAShiftClickHandsTheMovingEndToTheKeyboard: the two devices drive one
// selection, so a Shift-arrow after a Shift-click continues from the row the
// mouse reached rather than from the anchor.
func TestAShiftClickHandsTheMovingEndToTheKeyboard(t *testing.T) {
	l := newKeyList()
	l.Selected().Set(1)
	l.SetSelection(1)
	l.OnEvent(Event{Kind: EventClick, Y: 70, Shift: true}) // row 3
	if got := selectionOf(l); got != "[1 2 3]" {
		t.Fatalf("Shift-click: selection %s, want [1 2 3]", got)
	}
	key(l, "ArrowDown", "shift")
	if got := selectionOf(l); got != "[1 2 3 4]" {
		t.Fatalf("Shift-arrow after Shift-click: selection %s, want [1 2 3 4]", got)
	}
}

// TestASectionedListIgnoresTheMultiSelectKeys: sectioned mode is
// single-selection, and these keys must fall through to the plain roving cursor
// rather than build a range across a caption.
func TestASectionedListIgnoresTheMultiSelectKeys(t *testing.T) {
	l := NewSectionedListBox(
		ListSection{Title: "One", Items: []string{"a", "b"}},
		ListSection{Title: "Two", Items: []string{"c", "d"}},
	)
	l.MultiSelect = true
	l.RowHeight = 20
	l.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 200})
	l.Selected().Set(0)

	key(l, "ArrowDown", "shift")
	if l.Selected().Get() != 1 {
		t.Fatalf("Shift-ArrowDown in sectioned mode: cursor %d, want it to rove to 1",
			l.Selected().Get())
	}
	key(l, "a", "ctrl")
	if n := len(l.SelectedIndices()); n > 1 {
		t.Fatalf("Ctrl+A in sectioned mode selected %d rows, want single selection", n)
	}
}

// TestADisabledListIgnoresEveryMultiSelectKey too.
func TestADisabledListIgnoresEveryMultiSelectKey(t *testing.T) {
	l := newKeyList()
	l.Disabled().Set(true)
	l.Selected().Set(2)
	l.SetSelection(2)
	for _, k := range [][]string{{"ArrowDown", "shift"}, {"a", "ctrl"}, {" ", "meta"}} {
		key(l, k[0], k[1])
	}
	if got := selectionOf(l); got != "[2]" {
		t.Fatalf("disabled list: selection %s, want [2] untouched", got)
	}
}
