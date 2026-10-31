// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// A TextView's undo history, as a value. An application that shows several
// documents in one TextView (a file panel, tabs) keeps each document's
// history while another is shown and puts it back with SetHistory, so that
// Undo never restores one file's text into another. CanUndo/CanRedo and
// OnHistory let a ribbon or a menu enable its Undo and Redo as the history
// changes.

// TextHistory is a TextView's undo and redo stacks, detached from the view.
// The zero value is an empty history.
type TextHistory struct {
	undo, redo []tvSnapshot
}

// Len is the number of steps that can be undone and redone.
func (h TextHistory) Len() (undo, redo int) { return len(h.undo), len(h.redo) }

// WithoutRedo is the history with its redo steps dropped: what a new edit
// does to it.
func (h TextHistory) WithoutRedo() TextHistory { return TextHistory{undo: h.undo} }

// History returns the view's history. The view goes on with its own copy:
// later edits do not change the value returned.
func (t *TextView) History() TextHistory {
	return TextHistory{undo: append([]tvSnapshot(nil), t.undo...), redo: append([]tvSnapshot(nil), t.redo...)}
}

// SetHistory replaces the view's history with h (the zero value clears it).
// It does not change the text: the caller shows the document h belongs to
// first, then gives it its history.
func (t *TextView) SetHistory(h TextHistory) {
	t.undo = append([]tvSnapshot(nil), h.undo...)
	t.redo = append([]tvSnapshot(nil), h.redo...)
	t.historyChanged(false)
}

// ClearHistory empties the undo and redo stacks.
func (t *TextView) ClearHistory() { t.SetHistory(TextHistory{}) }

// CanUndo reports whether Undo would change anything.
func (t *TextView) CanUndo() bool { return len(t.undo) > 0 }

// CanRedo reports whether Redo would change anything.
func (t *TextView) CanRedo() bool { return len(t.redo) > 0 }

// ReplaceText replaces the whole buffer as one undoable edit (SetText is
// not undoable: it loads a document). The caret goes to the start.
func (t *TextView) ReplaceText(s string) {
	if s == t.Text().Get() {
		return
	}
	t.pushUndo()
	t.SetText(s)
}

func (t *TextView) historyChanged(edit bool) {
	if t.OnHistory != nil {
		t.OnHistory(edit)
	}
}
