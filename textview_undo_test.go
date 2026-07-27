// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- TextView undo/redo ---------------------------------------------------

func TestTextViewUndoRestoresTextCursorSelection(t *testing.T) {
	tv := NewTextView("hello")
	tv.SetSelection(Selection{0, 1, 0, 3})
	tv.CursorLine, tv.CursorCol = 0, 3
	tv.OnEvent(Event{Kind: EventChar, Code: "X"})
	if tv.Lines[0] != "helXlo" {
		t.Fatalf("setup: %v", tv.Lines)
	}
	tv.Undo()
	if tv.Lines[0] != "hello" {
		t.Fatalf("Undo text: %v", tv.Lines)
	}
	if tv.CursorLine != 0 || tv.CursorCol != 3 {
		t.Fatalf("Undo cursor: line=%d col=%d", tv.CursorLine, tv.CursorCol)
	}
	if tv.Selection != (Selection{0, 1, 0, 3}) {
		t.Fatalf("Undo selection: %+v", tv.Selection)
	}
}

func TestTextViewRedoReappliesEdit(t *testing.T) {
	tv := NewTextView("hello")
	tv.CursorCol = 5
	tv.OnEvent(Event{Kind: EventChar, Code: "!"})
	if tv.Lines[0] != "hello!" {
		t.Fatalf("setup: %v", tv.Lines)
	}
	tv.Undo()
	if tv.Lines[0] != "hello" {
		t.Fatalf("after undo: %v", tv.Lines)
	}
	tv.Redo()
	if tv.Lines[0] != "hello!" {
		t.Fatalf("after redo: %v", tv.Lines)
	}
	if tv.CursorCol != 6 {
		t.Fatalf("redo cursor: col=%d", tv.CursorCol)
	}
}

func TestTextViewUndoNoOpOnEmptyStack(t *testing.T) {
	tv := NewTextView("abc")
	tv.Undo()
	if tv.Lines[0] != "abc" {
		t.Fatal("Undo on empty stack must not mutate")
	}
}

func TestTextViewRedoNoOpOnEmptyStack(t *testing.T) {
	tv := NewTextView("abc")
	tv.Redo()
	if tv.Lines[0] != "abc" {
		t.Fatal("Redo on empty stack must not mutate")
	}
}

func TestTextViewUndoFiresOnChange(t *testing.T) {
	changes := 0
	tv := NewTextView("ab")
	tv.CursorCol = 2
	tv.OnEvent(Event{Kind: EventChar, Code: "c"})
	tv.OnChange = func() { changes++ }
	tv.Undo()
	if changes != 1 {
		t.Fatalf("Undo should fire OnChange once, got %d", changes)
	}
	tv.Redo()
	if changes != 2 {
		t.Fatalf("Redo should fire OnChange once, got %d", changes)
	}
}

func TestTextViewUndoRedoNilOnChangeNoPanic(t *testing.T) {
	tv := NewTextView("ab")
	tv.CursorCol = 2
	tv.OnEvent(Event{Kind: EventChar, Code: "c"})
	tv.Undo()
	tv.Redo()
}

func TestTextViewRedoClearedAfterFreshEdit(t *testing.T) {
	tv := NewTextView("ab")
	tv.CursorCol = 2
	tv.OnEvent(Event{Kind: EventChar, Code: "c"}) // "abc"
	tv.Undo()                                     // back to "ab", redo has "abc"
	tv.OnEvent(Event{Kind: EventChar, Code: "z"}) // fresh edit -> "abz"
	if tv.Lines[0] != "abz" {
		t.Fatalf("setup: %v", tv.Lines)
	}
	tv.Redo() // redo stack must be empty now: no-op
	if tv.Lines[0] != "abz" {
		t.Fatalf("Redo after fresh edit should be a no-op, got %v", tv.Lines)
	}
}

func TestTextViewUndoStackCapEnforced(t *testing.T) {
	tv := NewTextView("")
	// Push more than maxUndo edits; each EventChar with a single rune
	// pushes exactly one undo snapshot.
	for i := 0; i < maxUndo+10; i++ {
		tv.OnEvent(Event{Kind: EventChar, Code: "x"})
	}
	if len(tv.undo) != maxUndo {
		t.Fatalf("undo stack len = %d, want cap %d", len(tv.undo), maxUndo)
	}
	// Undo everything the stack still holds; the buffer must land on
	// the oldest RETAINED snapshot, not the very first edit (which was
	// evicted).
	for i := 0; i < maxUndo; i++ {
		tv.Undo()
	}
	if len(tv.Lines[0]) != 10 {
		t.Fatalf("after draining capped stack, len(Lines[0]) = %d, want 10 (10 dropped edits still applied)", len(tv.Lines[0]))
	}
}

func TestTextViewEmptyCharDoesNotPushUndo(t *testing.T) {
	tv := NewTextView("ab")
	tv.OnEvent(Event{Kind: EventChar, Code: ""})
	if len(tv.undo) != 0 {
		t.Fatalf("empty char must not push undo, len=%d", len(tv.undo))
	}
}

func TestTextViewBackspaceAtBufferStartDoesNotPushUndo(t *testing.T) {
	tv := NewTextView("ab")
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if len(tv.undo) != 0 {
		t.Fatalf("no-op backspace must not push undo, len=%d", len(tv.undo))
	}
}

func TestTextViewEnterPushesUndo(t *testing.T) {
	tv := NewTextView("abcdef")
	tv.CursorCol = 3
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	tv.Undo()
	if len(tv.Lines) != 1 || tv.Lines[0] != "abcdef" {
		t.Fatalf("Undo after Enter: %v", tv.Lines)
	}
}

func TestTextViewBackspacePushesUndo(t *testing.T) {
	tv := NewTextView("abc")
	tv.CursorCol = 2
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	tv.Undo()
	if tv.Lines[0] != "abc" || tv.CursorCol != 2 {
		t.Fatalf("Undo after Backspace: %v col=%d", tv.Lines, tv.CursorCol)
	}
}

func TestTextViewDeleteSelectionPushesUndo(t *testing.T) {
	tv := NewTextView("hello world")
	tv.SetSelection(Selection{0, 0, 0, 5})
	tv.DeleteSelection()
	if tv.Text() != " world" {
		t.Fatalf("setup: %q", tv.Text())
	}
	tv.Undo()
	if tv.Text() != "hello world" {
		t.Fatalf("Undo after DeleteSelection: %q", tv.Text())
	}
	if tv.Selection != (Selection{0, 0, 0, 5}) {
		t.Fatalf("Undo restored selection: %+v", tv.Selection)
	}
}

func TestTextViewDeleteSelectionEmptyDoesNotPushUndo(t *testing.T) {
	tv := NewTextView("hello")
	tv.DeleteSelection() // no active selection: no-op
	if len(tv.undo) != 0 {
		t.Fatalf("empty-selection delete must not push undo, len=%d", len(tv.undo))
	}
}

func TestTextViewCutSelectionPushesUndo(t *testing.T) {
	tv := NewTextView("hello world")
	tv.SetSelection(Selection{0, 0, 0, 5})
	tv.CutSelection()
	tv.Undo()
	if tv.Text() != "hello world" {
		t.Fatalf("Undo after CutSelection: %q", tv.Text())
	}
}

func TestTextViewPasteOverSelectionIsOneUndoStep(t *testing.T) {
	tv := NewTextView("hello world")
	tv.SetSelection(Selection{0, 0, 0, 5})
	tv.Paste("HEY")
	if tv.Text() != "HEY world" {
		t.Fatalf("setup: %q", tv.Text())
	}
	tv.Undo() // must restore the pre-paste state in ONE undo, not two.
	if tv.Text() != "hello world" {
		t.Fatalf("Undo after Paste-over-selection: %q", tv.Text())
	}
	tv.Redo()
	if tv.Text() != "HEY world" {
		t.Fatalf("Redo after Paste-over-selection: %q", tv.Text())
	}
}

func TestTextViewPasteNoSelectionPushesUndo(t *testing.T) {
	tv := NewTextView("world")
	tv.Paste("hello ")
	if tv.Text() != "hello world" {
		t.Fatalf("setup: %q", tv.Text())
	}
	tv.Undo()
	if tv.Text() != "world" {
		t.Fatalf("Undo after Paste (no selection): %q", tv.Text())
	}
}

func TestTextViewPasteEmptyTextNoSelectionDoesNotPushUndo(t *testing.T) {
	tv := NewTextView("world")
	tv.Paste("")
	if len(tv.undo) != 0 {
		t.Fatalf("no-op paste must not push undo, len=%d", len(tv.undo))
	}
	if tv.Text() != "world" {
		t.Fatalf("empty paste must not mutate: %q", tv.Text())
	}
}

func TestTextViewUndoKeyBinding(t *testing.T) {
	tv := NewTextView("ab")
	tv.CursorCol = 2
	tv.OnEvent(Event{Kind: EventChar, Code: "c"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Z"})
	if tv.Lines[0] != "ab" {
		t.Fatalf("Ctrl+Z should undo: %v", tv.Lines)
	}
}

func TestTextViewRedoKeyBindingCtrlY(t *testing.T) {
	tv := NewTextView("ab")
	tv.CursorCol = 2
	tv.OnEvent(Event{Kind: EventChar, Code: "c"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Z"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Y"})
	if tv.Lines[0] != "abc" {
		t.Fatalf("Ctrl+Y should redo: %v", tv.Lines)
	}
}

func TestTextViewRedoKeyBindingCtrlShiftZ(t *testing.T) {
	tv := NewTextView("ab")
	tv.CursorCol = 2
	tv.OnEvent(Event{Kind: EventChar, Code: "c"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Z"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Shift+Z"})
	if tv.Lines[0] != "abc" {
		t.Fatalf("Ctrl+Shift+Z should redo: %v", tv.Lines)
	}
}
