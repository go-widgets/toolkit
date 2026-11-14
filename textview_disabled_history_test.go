// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// fakeHistoryItem is a menu item bound to CanUndo/CanRedo the way a host binds
// one: it re-reads both whenever OnHistory fires.
type fakeHistoryItem struct{ undoGreyed, redoGreyed bool }

func bindFakeHistoryItem(tv *TextView) *fakeHistoryItem {
	it := &fakeHistoryItem{undoGreyed: !tv.CanUndo(), redoGreyed: !tv.CanRedo()}
	tv.OnHistory = func(bool) {
		it.undoGreyed, it.redoGreyed = !tv.CanUndo(), !tv.CanRedo()
	}
	return it
}

// Toggling Disabled changes CanUndo/CanRedo, so it must fire OnHistory: a menu
// item bound to them greys the moment the view is disabled and ungreys when it
// is enabled again, not at the next history change.
func TestTextViewDisabledToggleFiresOnHistory(t *testing.T) {
	for name, tv := range map[string]*TextView{
		"NewTextView": NewTextView("a"),
		"bare":        {lines: []string{"a"}},
	} {
		t.Run(name, func(t *testing.T) {
			tv.ReplaceText("ab") // one undo step
			tv.Undo()            // one redo step, none left to undo
			tv.ReplaceText("ac")
			tv.ReplaceText("ad")
			tv.Undo() // undo and redo both available
			it := bindFakeHistoryItem(tv)
			if it.undoGreyed || it.redoGreyed {
				t.Fatalf("setup: item greyed (undo %v, redo %v) with both available", it.undoGreyed, it.redoGreyed)
			}

			tv.Disabled().Set(true)
			if !it.undoGreyed || !it.redoGreyed {
				t.Errorf("disabled: item undo-greyed=%v redo-greyed=%v, want both greyed", it.undoGreyed, it.redoGreyed)
			}
			tv.Disabled().Set(false)
			if it.undoGreyed || it.redoGreyed {
				t.Errorf("re-enabled: item undo-greyed=%v redo-greyed=%v, want neither", it.undoGreyed, it.redoGreyed)
			}
		})
	}
}

// The subscription is made once: setting the same value again, or toggling
// repeatedly, fires exactly once per change, never twice.
func TestTextViewDisabledSubscribesOnce(t *testing.T) {
	tv := NewTextView("a")
	n := 0
	tv.OnHistory = func(edit bool) {
		if edit {
			t.Error("a Disabled toggle reported an edit")
		}
		n++
	}
	tv.Disabled().Set(true)
	tv.Disabled().Set(true)
	if n != 1 {
		t.Fatalf("OnHistory fired %d times for one change, want 1", n)
	}
	tv.Disabled().Set(false)
	if n != 2 {
		t.Fatalf("OnHistory fired %d times after two changes, want 2", n)
	}
}
