// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/toolkit/internal/formula"
)

// --- Construction, accessors, model wiring ------------------------------------

func TestSpreadsheetDefaults(t *testing.T) {
	s := NewSpreadsheet(4, 5)
	if s.Cols() != 4 || s.Rows() != 5 {
		t.Errorf("dims = %dx%d, want 4x5", s.Cols(), s.Rows())
	}
	if c, r := s.Active(); c != 0 || r != 0 {
		t.Errorf("Active = (%d,%d), want (0,0)", c, r)
	}
	if col, row := s.ScrollOffset(); col != 0 || row != 0 {
		t.Errorf("ScrollOffset = (%d,%d), want (0,0)", col, row)
	}
	if s.Editing() {
		t.Error("fresh sheet must not be editing")
	}
}

func TestSpreadsheetSetAndReadCell(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetCell(1, 1, "7")
	if got := s.CellDisplay(1, 1); got != "7" {
		t.Errorf("CellDisplay(1,1) = %q, want 7", got)
	}
	if got := s.CellRaw(1, 1); got != "7" {
		t.Errorf("CellRaw(1,1) = %q, want 7", got)
	}
}

// TestSpreadsheetFormulaRecalc drives a formula and its recomputation through
// the widget's public surface.
func TestSpreadsheetFormulaRecalc(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetCell(0, 0, "10")
	s.SetCell(0, 1, "20")
	s.SetCell(1, 0, "=A1+A2")
	if got := s.CellDisplay(1, 0); got != "30" {
		t.Fatalf("B1 = %q, want 30", got)
	}
	s.SetCell(0, 0, "100")
	if got := s.CellDisplay(1, 0); got != "120" {
		t.Fatalf("B1 after edit = %q, want 120", got)
	}
}

func TestSpreadsheetA11y(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	info := s.A11y()
	if info.Role != RoleGrid {
		t.Errorf("role = %q, want grid", info.Role)
	}
	if info.Name != "A1" {
		t.Errorf("name = %q, want A1", info.Name)
	}
	if info.Value != "" {
		t.Errorf("empty A1 value = %q, want empty", info.Value)
	}
	s.SetCell(0, 0, "42")
	if got := s.A11y().Value; got != "42" {
		t.Errorf("A1 value = %q, want 42", got)
	}
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if got := s.A11y().Name; got != "B1" {
		t.Errorf("name after ArrowRight = %q, want B1", got)
	}
}

// --- Selection + keyboard navigation ------------------------------------------

func TestSpreadsheetArrowNavigationClamps(t *testing.T) {
	s := NewSpreadsheet(2, 2)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	// Down + right to the far corner, then push past both edges.
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if c, r := s.Active(); c != 1 || r != 1 {
		t.Fatalf("Active = (%d,%d), want (1,1)", c, r)
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})  // past bottom -> clamps
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // past right -> clamps
	if c, r := s.Active(); c != 1 || r != 1 {
		t.Fatalf("Active clamped = (%d,%d), want (1,1)", c, r)
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if c, r := s.Active(); c != 0 || r != 0 {
		t.Fatalf("Active = (%d,%d), want (0,0)", c, r)
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})   // past top -> clamps
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // past left -> clamps
	if c, r := s.Active(); c != 0 || r != 0 {
		t.Fatalf("Active clamped = (%d,%d), want (0,0)", c, r)
	}
	// A key the widget does not handle is a no-op.
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if c, r := s.Active(); c != 0 || r != 0 {
		t.Fatalf("Active after unhandled key = (%d,%d), want (0,0)", c, r)
	}
}

func TestSpreadsheetClickSelectsCell(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	// Cell (1,1) spans x[100,164) y[40,60); click its centre.
	s.OnEvent(Event{Kind: EventClick, X: 132, Y: 50})
	if c, r := s.Active(); c != 1 || r != 1 {
		t.Errorf("Active after click = (%d,%d), want (1,1)", c, r)
	}
}

func TestSpreadsheetClickOnHeaderDoesNotSelect(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	// Click in the column-header band (y < headerHeight) — no selection change.
	s.OnEvent(Event{Kind: EventClick, X: 120, Y: 5})
	if c, r := s.Active(); c != 0 || r != 0 {
		t.Errorf("Active after header click = (%d,%d), want (0,0)", c, r)
	}
	// Click in the row-header band (x < rowHeaderWidth).
	s.OnEvent(Event{Kind: EventClick, X: 5, Y: 50})
	if c, r := s.Active(); c != 0 || r != 0 {
		t.Errorf("Active after row-header click = (%d,%d), want (0,0)", c, r)
	}
	// Click past the last cell (within the grid area but beyond the data).
	s.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600}) // whole 3x3 fits, lots of empty grid
	s.OnEvent(Event{Kind: EventClick, X: 700, Y: 500})
	if c, r := s.Active(); c != 0 || r != 0 {
		t.Errorf("Active after empty-area click = (%d,%d), want (0,0)", c, r)
	}
}

// --- Inline editing -----------------------------------------------------------

func TestSpreadsheetTypeToEditCommitsAndMovesDown(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	var gotRef string
	var gotRaw string
	s.OnCellChange = func(ref formula.Ref, raw string) { gotRef = ref.A1(); gotRaw = raw }

	s.OnEvent(Event{Kind: EventChar, Code: "5"}) // start editing A1 with "5"
	if !s.Editing() {
		t.Fatal("typing a character must open the editor")
	}
	s.OnEvent(Event{Kind: EventChar, Code: "0"}) // -> "50"
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if s.Editing() {
		t.Fatal("Enter must commit and close the editor")
	}
	if got := s.CellDisplay(0, 0); got != "50" {
		t.Errorf("A1 = %q, want 50", got)
	}
	if gotRef != "A1" || gotRaw != "50" {
		t.Errorf("OnCellChange = (%q,%q), want (A1,50)", gotRef, gotRaw)
	}
	if c, r := s.Active(); c != 0 || r != 1 {
		t.Errorf("cursor after Enter = (%d,%d), want (0,1)", c, r)
	}
}

func TestSpreadsheetF2EditsExistingValue(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.SetCell(0, 0, "=1+2")
	s.OnEvent(Event{Kind: EventKeyDown, Code: "F2"}) // opens seeded with the raw formula
	if !s.Editing() {
		t.Fatal("F2 must open the editor")
	}
	// Append "+3" and commit.
	s.OnEvent(Event{Kind: EventChar, Code: "+"})
	s.OnEvent(Event{Kind: EventChar, Code: "3"})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if got := s.CellDisplay(0, 0); got != "6" {
		t.Errorf("A1 = %q, want 6", got)
	}
}

func TestSpreadsheetEnterOpensEditor(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.SetCell(0, 0, "hi")
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"}) // not editing -> opens editor
	if !s.Editing() {
		t.Fatal("Enter on a selected cell must open the editor")
	}
}

func TestSpreadsheetEscapeCancels(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.SetCell(0, 0, "keep")
	s.OnEvent(Event{Kind: EventChar, Code: "x"}) // begin editing (would replace)
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if s.Editing() {
		t.Fatal("Escape must close the editor")
	}
	if got := s.CellDisplay(0, 0); got != "keep" {
		t.Errorf("A1 = %q, want keep (edit discarded)", got)
	}
}

func TestSpreadsheetTabCommitsAndMovesRight(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.OnEvent(Event{Kind: EventChar, Code: "9"})
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if got := s.CellDisplay(0, 0); got != "9" {
		t.Errorf("A1 = %q, want 9", got)
	}
	if c, r := s.Active(); c != 1 || r != 0 {
		t.Errorf("cursor after Tab = (%d,%d), want (1,0)", c, r)
	}
}

func TestSpreadsheetTypingRoutesToEditor(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.OnEvent(Event{Kind: EventChar, Code: "a"})
	// An arrow key while editing routes to the editor (moves its caret), not the
	// grid cursor.
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if c, r := s.Active(); c != 0 || r != 0 {
		t.Errorf("cursor moved during edit = (%d,%d), want (0,0)", c, r)
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if got := s.CellDisplay(0, 0); got != "a" {
		t.Errorf("A1 = %q, want a", got)
	}
}

func TestSpreadsheetClickWhileEditingCommitsThenSelects(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.OnEvent(Event{Kind: EventChar, Code: "3"}) // editing A1
	// Click cell (2,2): x[164,228) y[40,60) — centre (196,50).
	s.OnEvent(Event{Kind: EventClick, X: 196, Y: 50})
	if s.Editing() {
		t.Fatal("clicking away must commit and close the editor")
	}
	if got := s.CellDisplay(0, 0); got != "3" {
		t.Errorf("A1 = %q, want 3 (committed)", got)
	}
	if c, r := s.Active(); c != 2 || r != 1 {
		t.Errorf("Active after click-away = (%d,%d), want (2,1)", c, r)
	}
}

func TestSpreadsheetPublicEditCommands(t *testing.T) {
	s := NewSpreadsheet(3, 3)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	// CommitEdit / CancelEdit are no-ops when nothing is being edited.
	s.CommitEdit()
	s.CancelEdit()
	if s.Editing() {
		t.Fatal("no-op commands must not open an editor")
	}
	// BeginEdit then CancelEdit.
	s.SetCell(0, 0, "v")
	s.BeginEdit()
	if !s.Editing() {
		t.Fatal("BeginEdit must open the editor")
	}
	s.CancelEdit()
	if s.Editing() {
		t.Fatal("CancelEdit must close the editor")
	}
	// BeginEdit then CommitEdit with no OnCellChange handler set.
	s.OnCellChange = nil
	s.BeginEdit()
	s.CommitEdit()
	if s.Editing() {
		t.Fatal("CommitEdit must close the editor")
	}
}

// TestSpreadsheetCellAtGutter covers cellAt's viewport-gutter guard, which a
// click never reaches through OnEvent because the scrollbar consumes gutter
// clicks first.
func TestSpreadsheetCellAtGutter(t *testing.T) {
	s := NewSpreadsheet(20, 30) // overflows -> a scrollbar gutter exists
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	// gridRect is {36,20,258,174} with the slim 6px track; a point in the right
	// gutter (gx >= g.W, i.e. x >= 294).
	if _, ok := s.cellAt(296, 50); ok {
		t.Error("point in the vertical scrollbar gutter must not resolve to a cell")
	}
	// A point in the bottom gutter (gy >= g.H, i.e. y >= 194).
	if _, ok := s.cellAt(50, 196); ok {
		t.Error("point in the horizontal scrollbar gutter must not resolve to a cell")
	}
}

// TestSpreadsheetEditZeroSizedSheetNoop covers openEditor's out-of-bounds guard.
func TestSpreadsheetEditZeroSizedSheetNoop(t *testing.T) {
	s := NewSpreadsheet(0, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	s.BeginEdit()
	if s.Editing() {
		t.Fatal("cannot edit a cell that does not exist")
	}
	s.OnEvent(Event{Kind: EventChar, Code: "x"})
	if s.Editing() {
		t.Fatal("type-to-edit on an empty sheet must be a no-op")
	}
}
