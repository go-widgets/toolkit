// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestUndoStackOrderAndRedo(t *testing.T) {
	s := NewUndoStack(0)
	v := 0
	set := func(label string, to int) UndoCommand {
		from := v
		return UndoCommand{Label: label, Undo: func() { v = from }, Redo: func() { v = to }}
	}
	s.Do(set("one", 1))
	s.Do(set("two", 2))
	if v != 2 || s.UndoLabel() != "two" || s.CanRedo() {
		t.Fatalf("v=%d label=%q", v, s.UndoLabel())
	}
	if !s.Undo() || v != 1 || s.RedoLabel() != "two" || !s.CanRedo() {
		t.Fatalf("undo: v=%d", v)
	}
	if !s.Undo() || v != 0 || s.CanUndo() || s.Undo() {
		t.Fatalf("undo twice: v=%d", v)
	}
	if !s.Redo() || v != 1 || !s.Redo() || v != 2 || s.Redo() {
		t.Fatalf("redo: v=%d", v)
	}
	s.Undo()
	s.Do(set("three", 3)) // a new step drops the redo
	if s.CanRedo() || v != 3 {
		t.Fatal("a new step keeps the redo")
	}
	if u, r := s.Len(); u != 2 || r != 0 {
		t.Fatalf("len %d %d", u, r)
	}
}

func TestUndoStackLimitAndReentrancy(t *testing.T) {
	s := NewUndoStack(3)
	for i := 0; i < 10; i++ {
		s.Push(UndoCommand{Undo: func() {}, Redo: func() {}})
	}
	if u, _ := s.Len(); u != 3 {
		t.Fatalf("kept %d, want 3", u)
	}
	// a step whose undo records a step does not record it
	s.Clear()
	s.Push(UndoCommand{Label: "outer", Undo: func() {
		if !s.Busy() {
			t.Error("not busy inside Undo")
		}
		s.Push(UndoCommand{Label: "inner", Undo: func() {}, Redo: func() {}})
	}, Redo: func() {}})
	s.Undo()
	if u, r := s.Len(); u != 0 || r != 1 || s.RedoLabel() != "outer" {
		t.Fatalf("len %d %d %q", u, r, s.RedoLabel())
	}
	// a step missing a function is refused
	s.Push(UndoCommand{Undo: func() {}})
	if s.CanUndo() {
		t.Fatal("a half step was recorded")
	}
	s.DropRedo()
	if s.CanRedo() {
		t.Fatal("DropRedo")
	}
}

func TestUndoStackEmptyAndRefused(t *testing.T) {
	s := NewUndoStack(0)
	if s.UndoLabel() != "" || s.RedoLabel() != "" {
		t.Fatal("labels of an empty stack")
	}
	v0 := s.Version().Get()
	s.DropRedo() // nothing to drop: no change
	ran := false
	s.Do(UndoCommand{Redo: func() { ran = true }}) // no Undo: refused, not run
	if ran || s.CanUndo() || s.Version().Get() != v0 {
		t.Fatal("a half step ran or was recorded")
	}
}

func TestUndoStackBindsActions(t *testing.T) {
	s := NewUndoStack(0)
	undo, redo := NewAction("undo", "Undo", nil), NewAction("redo", "Redo", nil)
	unbind := s.Bind(undo, redo)
	if undo.IsEnabled() || redo.IsEnabled() {
		t.Fatal("enabled on an empty stack")
	}
	n := 0
	s.Push(UndoCommand{Undo: func() { n-- }, Redo: func() { n++ }})
	if !undo.IsEnabled() || redo.IsEnabled() {
		t.Fatal("after a push")
	}
	if !undo.Execute() || n != -1 || undo.IsEnabled() || !redo.IsEnabled() {
		t.Fatalf("undo through the action: n=%d", n)
	}
	redo.Execute()
	if n != 0 {
		t.Fatalf("n=%d", n)
	}
	unbind()
	s.Clear()
	if !undo.IsEnabled() {
		t.Fatal("followed after unbind")
	}
	v0 := s.Version().Get()
	s.Clear() // nothing to clear: no change
	if s.Version().Get() != v0 {
		t.Fatal("an empty Clear bumped the version")
	}
}

func TestTextHistoryPerDocument(t *testing.T) {
	tv := NewTextView("alpha")
	var edits, others int
	tv.OnHistory = func(edit bool) {
		if edit {
			edits++
		} else {
			others++
		}
	}
	tv.ReplaceText("alpha one")
	tv.ReplaceText("alpha two")
	if !tv.CanUndo() || tv.CanRedo() || edits != 2 {
		t.Fatalf("edits=%d", edits)
	}
	a := tv.History()
	// show another document with its own (empty) history
	tv.SetText("beta")
	tv.SetHistory(TextHistory{})
	if tv.CanUndo() {
		t.Fatal("beta inherited alpha's history")
	}
	tv.ReplaceText("beta one")
	b := tv.History()
	// back to alpha: its history comes back, and Undo restores alpha's text
	tv.SetText("alpha two")
	tv.SetHistory(a)
	tv.Undo()
	if got := tv.Text().Get(); got != "alpha one" {
		t.Fatalf("undo in alpha gave %q", got)
	}
	if u, r := tv.History().Len(); u != 1 || r != 1 {
		t.Fatalf("alpha history %d/%d", u, r)
	}
	if u, r := b.Len(); u != 1 || r != 0 {
		t.Fatalf("beta history %d/%d", u, r)
	}
	if _, r := tv.History().WithoutRedo().Len(); r != 0 {
		t.Fatal("WithoutRedo")
	}
	// the value handed out does not follow later edits
	h := tv.History()
	tv.ReplaceText("alpha three")
	if u, _ := h.Len(); u != 1 {
		t.Fatal("History aliased the view's stack")
	}
	tv.ReplaceText("alpha three") // the same text: no step
	if u, _ := tv.History().Len(); u != 2 {
		t.Fatal("a no-op replace was recorded")
	}
	tv.ClearHistory()
	if tv.CanUndo() || tv.CanRedo() || others == 0 {
		t.Fatal("ClearHistory")
	}
}
