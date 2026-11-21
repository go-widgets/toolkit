// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/mvvm"

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

// CanUndo reports whether Undo would change anything. A Disabled view cannot
// undo: the history is kept, but a menu or ribbon greys its command.
func (t *TextView) CanUndo() bool {
	if t.hist != nil {
		return t.hist.CanUndo() && !t.Disabled().Get()
	}
	return len(t.undo) > 0 && !t.Disabled().Get()
}

// CanRedo reports whether Redo would change anything (never while Disabled).
func (t *TextView) CanRedo() bool {
	if t.hist != nil {
		return t.hist.CanRedo() && !t.Disabled().Get()
	}
	return len(t.redo) > 0 && !t.Disabled().Get()
}

// A HistoryDelegate answers undo and redo for a TextView whose text is not the
// view's alone. All four are required.
type HistoryDelegate struct {
	// Undo and Redo do the work; they are called only when the matching Can
	// function reports true. Whatever they change in the text, they change by
	// themselves (the view does not restore a buffer).
	Undo, Redo       func()
	CanUndo, CanRedo func() bool
}

// SetHistoryDelegate hands undo and redo to d, or takes them back when d is nil.
//
// Ctrl+Z, Ctrl+Y, Ctrl+Shift+Z, [TextView.Undo], [TextView.Redo], CanUndo and
// CanRedo then answer from d, and the view keeps no snapshot of its own. It
// exists for text that other people edit as well. A snapshot restores a whole
// buffer, so undoing in such text restores it to a state that predates what the
// others typed since — and a binding that publishes what the buffer became
// publishes that as the local user deleting their work. A delegate undoes the
// local user's own edits, and only those.
//
// The snapshot history is kept as it was and comes back when d is nil.
func (t *TextView) SetHistoryDelegate(d *HistoryDelegate) {
	t.hist = d
	t.historyChanged(false)
}

// ReplaceText replaces the whole buffer as one undoable edit (SetText is
// not undoable: it loads a document). The caret goes to the start.
func (t *TextView) ReplaceText(s string) {
	if s == t.Text().Get() {
		return
	}
	t.pushUndo()
	t.SetText(s)
}

// Disabled is [Base.Disabled] with one addition: CanUndo and CanRedo answer
// false while the view is disabled, so a toggle changes what a menu bound to
// them should show. The first call subscribes the flag to OnHistory, so the
// host re-reads CanUndo/CanRedo at the toggle instead of keeping the old
// greying until the next history change. Every access goes through here, so a
// bare &TextView{} is covered as well as NewTextView.
func (t *TextView) Disabled() *mvvm.Observable[bool] {
	d := t.Base.Disabled()
	if !t.histWired {
		t.histWired = true
		d.Subscribe(func(bool) { t.historyChanged(false) })
	}
	return d
}

func (t *TextView) historyChanged(edit bool) {
	if t.OnHistory != nil {
		t.OnHistory(edit)
	}
}
