// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// With a history delegate set (text other people edit too, toolkit#591) the key
// path and the methods answer from it, and the view keeps no snapshot of its own.
func TestHistoryDelegateOwnsUndoAndRedo(t *testing.T) {
	tv := NewTextView("")
	var undone, redone int
	canU, canR := true, true
	var edits []bool
	tv.OnHistory = func(edit bool) { edits = append(edits, edit) }
	tv.SetHistoryDelegate(&HistoryDelegate{
		Undo:    func() { undone++ },
		Redo:    func() { redone++ },
		CanUndo: func() bool { return canU },
		CanRedo: func() bool { return canR },
	})

	tv.OnEvent(Event{Kind: EventChar, Code: "x"})
	if got := tv.Text().Get(); got != "x" {
		t.Fatalf("typing still edits the view: %q", got)
	}
	if u, r := tv.History().Len(); u != 0 || r != 0 {
		t.Fatalf("the view kept a snapshot (%d, %d) although the delegate owns history", u, r)
	}
	if len(edits) == 0 || !edits[len(edits)-1] {
		t.Fatalf("an edit is still announced to OnHistory: %v", edits)
	}

	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Z"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Y"})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Shift+Z"})
	if undone != 1 || redone != 2 {
		t.Fatalf("the keys reached the delegate %d undo, %d redo, want 1 and 2", undone, redone)
	}
	if got := tv.Text().Get(); got != "x" {
		t.Fatalf("the view restored a buffer of its own: %q", got)
	}
	tv.Undo()
	tv.Redo()
	if undone != 2 || redone != 3 {
		t.Fatalf("the methods reached the delegate %d undo, %d redo", undone, redone)
	}

	canU, canR = false, false
	if tv.CanUndo() || tv.CanRedo() {
		t.Fatal("CanUndo/CanRedo do not answer from the delegate")
	}
	tv.Undo()
	tv.Redo()
	if undone != 2 || redone != 3 {
		t.Fatal("the delegate was asked to do what it said it cannot")
	}
	canU, canR = true, true
	tv.Disabled().Set(true)
	if tv.CanUndo() || tv.CanRedo() {
		t.Fatal("a disabled view can undo through its delegate")
	}
	tv.Disabled().Set(false)
}

// Control: without a delegate the editor behaves as before, and taking the
// delegate back returns to the snapshot history it left untouched.
func TestHistoryDelegateRemovedRestoresSnapshots(t *testing.T) {
	tv := NewTextView("")
	tv.OnEvent(Event{Kind: EventChar, Code: "a"})
	tv.SetHistoryDelegate(&HistoryDelegate{
		Undo: func() { t.Fatal("delegate used") }, Redo: func() { t.Fatal("delegate used") },
		CanUndo: func() bool { return false }, CanRedo: func() bool { return false },
	})
	tv.SetHistoryDelegate(nil)
	if !tv.CanUndo() {
		t.Fatal("the snapshot taken before the delegate is gone")
	}
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Z"})
	if got := tv.Text().Get(); got != "" {
		t.Fatalf("snapshot undo broken: %q", got)
	}
	if !tv.CanRedo() {
		t.Fatal("no redo after a snapshot undo")
	}
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Y"})
	if got := tv.Text().Get(); got != "a" {
		t.Fatalf("snapshot redo broken: %q", got)
	}
}
