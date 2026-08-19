// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- TextView undo/redo ---------------------------------------------------

func TestTextViewUndoRestoresTextCursorSelection(t *testing.T) {
	tv := NewTextView("hello")
	tv.SetSelection(Selection{0, 1, 0, 3})
	tv.CursorLine().Set(0)
	tv.CursorCol().Set(3)
	tv.OnEvent(Event{Kind: EventChar, Code: "X"})
	if tv.lines[0] != "helXlo" {
		t.Fatalf("setup: %v", tv.lines)
	}
	tv.Undo()
	if tv.lines[0] != "hello" {
		t.Fatalf("Undo text: %v", tv.lines)
	}
	if tv.CursorLine().Get() != 0 || tv.CursorCol().Get() != 3 {
		t.Fatalf("Undo cursor: line=%d col=%d", tv.CursorLine().Get(), tv.CursorCol().Get())
	}
	if tv.Selection().Get() != (Selection{0, 1, 0, 3}) {
		t.Fatalf("Undo selection: %+v", tv.Selection().Get())
	}
}

func TestTextViewRedoReappliesEdit(t *testing.T) {
	tv := NewTextView("hello")
	tv.CursorCol().Set(5)
	tv.OnEvent(Event{Kind: EventChar, Code: "!"})
	if tv.lines[0] != "hello!" {
		t.Fatalf("setup: %v", tv.lines)
	}
	tv.Undo()
	if tv.lines[0] != "hello" {
		t.Fatalf("after undo: %v", tv.lines)
	}
	tv.Redo()
	if tv.lines[0] != "hello!" {
		t.Fatalf("after redo: %v", tv.lines)
	}
	if tv.CursorCol().Get() != 6 {
		t.Fatalf("redo cursor: col=%d", tv.CursorCol().Get())
	}
}

func TestTextViewUndoNoOpOnEmptyStack(t *testing.T) {
	tv := NewTextView("abc")
	tv.Undo()
	if tv.lines[0] != "abc" {
		t.Fatal("Undo on empty stack must not mutate")
	}
}

func TestTextViewRedoNoOpOnEmptyStack(t *testing.T) {
	tv := NewTextView("abc")
	tv.Redo()
	if tv.lines[0] != "abc" {
		t.Fatal("Redo on empty stack must not mutate")
	}
}

func TestTextViewUndoFiresOnChange(t *testing.T) {
	changes := 0
	tv := NewTextView("ab")
	tv.CursorCol().Set(2)
	tv.OnEvent(Event{Kind: EventChar, Code: "c"})
	tv.Text().Subscribe(func(string) { changes++ })
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
	tv.CursorCol().Set(2)
	tv.OnEvent(Event{Kind: EventChar, Code: "c"})
	tv.Undo()
	tv.Redo()
}

func TestTextViewRedoClearedAfterFreshEdit(t *testing.T) {
	tv := NewTextView("ab")
	tv.CursorCol().Set(2)
	tv.OnEvent(Event{Kind: EventChar, Code: "c"}) // "abc"
	tv.Undo()                                     // back to "ab", redo has "abc"
	tv.OnEvent(Event{Kind: EventChar, Code: "z"}) // fresh edit -> "abz"
	if tv.lines[0] != "abz" {
		t.Fatalf("setup: %v", tv.lines)
	}
	tv.Redo() // redo stack must be empty now: no-op
	if tv.lines[0] != "abz" {
		t.Fatalf("Redo after fresh edit should be a no-op, got %v", tv.lines)
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
	if len(tv.lines[0]) != 10 {
		t.Fatalf("after draining capped stack, len(Lines[0]) = %d, want 10 (10 dropped edits still applied)", len(tv.lines[0]))
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
	tv.CursorCol().Set(3)
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	tv.Undo()
	if len(tv.lines) != 1 || tv.lines[0] != "abcdef" {
		t.Fatalf("Undo after Enter: %v", tv.lines)
	}
}

func TestTextViewBackspacePushesUndo(t *testing.T) {
	tv := NewTextView("abc")
	tv.CursorCol().Set(2)
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	tv.Undo()
	if tv.lines[0] != "abc" || tv.CursorCol().Get() != 2 {
		t.Fatalf("Undo after Backspace: %v col=%d", tv.lines, tv.CursorCol().Get())
	}
}

func TestTextViewDeleteSelectionPushesUndo(t *testing.T) {
	tv := NewTextView("hello world")
	tv.SetSelection(Selection{0, 0, 0, 5})
	tv.DeleteSelection()
	if tv.Text().Get() != " world" {
		t.Fatalf("setup: %q", tv.Text().Get())
	}
	tv.Undo()
	if tv.Text().Get() != "hello world" {
		t.Fatalf("Undo after DeleteSelection: %q", tv.Text().Get())
	}
	if tv.Selection().Get() != (Selection{0, 0, 0, 5}) {
		t.Fatalf("Undo restored selection: %+v", tv.Selection().Get())
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
	if tv.Text().Get() != "hello world" {
		t.Fatalf("Undo after CutSelection: %q", tv.Text().Get())
	}
}

func TestTextViewPasteOverSelectionIsOneUndoStep(t *testing.T) {
	tv := NewTextView("hello world")
	tv.SetSelection(Selection{0, 0, 0, 5})
	tv.Paste("HEY")
	if tv.Text().Get() != "HEY world" {
		t.Fatalf("setup: %q", tv.Text().Get())
	}
	tv.Undo() // must restore the pre-paste state in ONE undo, not two.
	if tv.Text().Get() != "hello world" {
		t.Fatalf("Undo after Paste-over-selection: %q", tv.Text().Get())
	}
	tv.Redo()
	if tv.Text().Get() != "HEY world" {
		t.Fatalf("Redo after Paste-over-selection: %q", tv.Text().Get())
	}
}

func TestTextViewPasteNoSelectionPushesUndo(t *testing.T) {
	tv := NewTextView("world")
	tv.Paste("hello ")
	if tv.Text().Get() != "hello world" {
		t.Fatalf("setup: %q", tv.Text().Get())
	}
	tv.Undo()
	if tv.Text().Get() != "world" {
		t.Fatalf("Undo after Paste (no selection): %q", tv.Text().Get())
	}
}

func TestTextViewPasteEmptyTextNoSelectionDoesNotPushUndo(t *testing.T) {
	tv := NewTextView("world")
	tv.Paste("")
	if len(tv.undo) != 0 {
		t.Fatalf("no-op paste must not push undo, len=%d", len(tv.undo))
	}
	if tv.Text().Get() != "world" {
		t.Fatalf("empty paste must not mutate: %q", tv.Text().Get())
	}
}

func TestTextViewUndoKeyBinding(t *testing.T) {
	tv := NewTextView("ab")
	tv.CursorCol().Set(2)
	tv.OnEvent(Event{Kind: EventChar, Code: "c"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Z"})
	if tv.lines[0] != "ab" {
		t.Fatalf("Ctrl+Z should undo: %v", tv.lines)
	}
}

func TestTextViewRedoKeyBindingCtrlY(t *testing.T) {
	tv := NewTextView("ab")
	tv.CursorCol().Set(2)
	tv.OnEvent(Event{Kind: EventChar, Code: "c"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Z"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Y"})
	if tv.lines[0] != "abc" {
		t.Fatalf("Ctrl+Y should redo: %v", tv.lines)
	}
}

func TestTextViewRedoKeyBindingCtrlShiftZ(t *testing.T) {
	tv := NewTextView("ab")
	tv.CursorCol().Set(2)
	tv.OnEvent(Event{Kind: EventChar, Code: "c"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Z"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Shift+Z"})
	if tv.lines[0] != "abc" {
		t.Fatalf("Ctrl+Shift+Z should redo: %v", tv.lines)
	}
}

// A host that Sets the Text() Observable directly (a VM→widget push in a
// two-way binding) reloads the line buffer and clamps the caret into the new
// bounds — without a feedback loop (sync()'s own echo is ignored).
func TestTextViewTextObservableReloadsBuffer(t *testing.T) {
	tv := NewTextView("one\ntwo\nthree")
	tv.CursorLine().Set(2)
	tv.CursorCol().Set(4)
	// Host push: shorter buffer than the caret's line/col.
	tv.Text().Set("hi")
	if len(tv.lines) != 1 || tv.lines[0] != "hi" {
		t.Fatalf("adopt did not reload buffer: %v", tv.lines)
	}
	if tv.CursorLine().Get() != 0 || tv.CursorCol().Get() != 2 {
		t.Fatalf("caret not clamped into new bounds: line=%d col=%d", tv.CursorLine().Get(), tv.CursorCol().Get())
	}
	// Empty host push collapses to a single empty line.
	tv.Text().Set("")
	if len(tv.lines) != 1 || tv.lines[0] != "" {
		t.Fatalf("empty host push: %v", tv.lines)
	}
	// An internal edit still publishes through Text() (no loop / no double-apply).
	tv.CursorCol().Set(0)
	tv.OnEvent(Event{Kind: EventChar, Code: "z"})
	if tv.Text().Get() != "z" {
		t.Fatalf("edit after host push: %q", tv.Text().Get())
	}
	// A host that parked the caret line below 0 is clamped up to the first line
	// on the next push (the defensive lower bound).
	tv.CursorLine().Set(-3)
	tv.Text().Set("a\nb\nc")
	if tv.CursorLine().Get() != 0 {
		t.Fatalf("negative caret line not clamped up: %d", tv.CursorLine().Get())
	}
}
