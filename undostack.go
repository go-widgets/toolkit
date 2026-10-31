// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/mvvm"

// UndoStack is an application's command history (the command pattern): each
// step knows how to undo and redo itself, and the stack runs them in order.
// It suits what a TextView's own history cannot hold: an operation on many
// documents at once (an import, a restore), a change to a diagram, a
// setting. A ribbon's or a menu's Undo and Redo bind to it with Bind, which
// keeps them enabled exactly when there is something to undo or redo, and
// labels them with the step ("Undo Import").
//
// A new step drops the redo steps, as everywhere. Undo and Redo run the
// step's function with the stack marked busy: a Push from inside them is
// ignored, so a step whose undo goes through code that records steps does
// not record itself again.
type UndoStack struct {
	// Limit bounds the steps kept; the oldest go first (0: DefaultUndoLimit).
	Limit int

	undo, redo []UndoCommand
	busy       bool
	version    *mvvm.Observable[int]
}

// UndoCommand is one step of an UndoStack. Undo puts things back as they
// were before the step, Redo does the step again; Label names it for a
// menu or a tooltip (optional).
type UndoCommand struct {
	Label      string
	Undo, Redo func()
}

// DefaultUndoLimit is the steps an UndoStack keeps when Limit is 0.
const DefaultUndoLimit = 100

// NewUndoStack returns an empty stack keeping at most limit steps
// (0: DefaultUndoLimit).
func NewUndoStack(limit int) *UndoStack { return &UndoStack{Limit: limit} }

// Version counts the changes to the stack: it is bumped by every Push, Undo,
// Redo, Clear and DropRedo, so a view can subscribe and refresh.
func (s *UndoStack) Version() *mvvm.Observable[int] {
	if s.version == nil {
		s.version = mvvm.NewObservable(0)
	}
	return s.version
}

func (s *UndoStack) bump() { s.Version().Set(s.Version().Get() + 1) }

// Push records a step that has just been done. It drops the redo steps.
// Ignored while an Undo or Redo is running, and for a step without both
// functions.
func (s *UndoStack) Push(c UndoCommand) {
	if s.busy || c.Undo == nil || c.Redo == nil {
		return
	}
	s.undo = append(s.undo, c)
	limit := s.Limit
	if limit <= 0 {
		limit = DefaultUndoLimit
	}
	if len(s.undo) > limit {
		s.undo = append([]UndoCommand(nil), s.undo[len(s.undo)-limit:]...)
	}
	s.redo = nil
	s.bump()
}

// Do runs a step (its Redo) and records it.
func (s *UndoStack) Do(c UndoCommand) {
	if s.busy || c.Undo == nil || c.Redo == nil {
		return
	}
	s.run(c.Redo)
	s.Push(c)
}

// Undo undoes the last step, reporting whether there was one.
func (s *UndoStack) Undo() bool {
	if s.busy || len(s.undo) == 0 {
		return false
	}
	c := s.undo[len(s.undo)-1]
	s.undo = s.undo[:len(s.undo)-1]
	s.run(c.Undo)
	s.redo = append(s.redo, c)
	s.bump()
	return true
}

// Redo does again the last step undone, reporting whether there was one.
func (s *UndoStack) Redo() bool {
	if s.busy || len(s.redo) == 0 {
		return false
	}
	c := s.redo[len(s.redo)-1]
	s.redo = s.redo[:len(s.redo)-1]
	s.run(c.Redo)
	s.undo = append(s.undo, c)
	s.bump()
	return true
}

func (s *UndoStack) run(f func()) {
	s.busy = true
	defer func() { s.busy = false }()
	f()
}

// Busy reports whether an Undo, Redo or Do is running.
func (s *UndoStack) Busy() bool { return s.busy }

// CanUndo reports whether Undo would do something.
func (s *UndoStack) CanUndo() bool { return len(s.undo) > 0 }

// CanRedo reports whether Redo would do something.
func (s *UndoStack) CanRedo() bool { return len(s.redo) > 0 }

// UndoLabel is the label of the step Undo would undo ("" for none).
func (s *UndoStack) UndoLabel() string {
	if len(s.undo) == 0 {
		return ""
	}
	return s.undo[len(s.undo)-1].Label
}

// RedoLabel is the label of the step Redo would redo ("" for none).
func (s *UndoStack) RedoLabel() string {
	if len(s.redo) == 0 {
		return ""
	}
	return s.redo[len(s.redo)-1].Label
}

// Len is the number of steps that can be undone and redone.
func (s *UndoStack) Len() (undo, redo int) { return len(s.undo), len(s.redo) }

// Clear forgets every step.
func (s *UndoStack) Clear() {
	if len(s.undo) == 0 && len(s.redo) == 0 {
		return
	}
	s.undo, s.redo = nil, nil
	s.bump()
}

// DropRedo forgets the steps that could be redone: what a change made
// outside the stack (typing in an editor with a history of its own) does
// to it.
func (s *UndoStack) DropRedo() {
	if len(s.redo) == 0 {
		return
	}
	s.redo = nil
	s.bump()
}

// Bind keeps undo and redo enabled exactly when the stack can undo and redo
// (nil skips one), now and on every change, and sets their Run to the
// stack's Undo and Redo when they have none. It returns the function that
// stops following.
func (s *UndoStack) Bind(undo, redo *Action) (unbind func()) {
	if undo != nil && undo.Run == nil {
		undo.Run = func() { s.Undo() }
	}
	if redo != nil && redo.Run == nil {
		redo.Run = func() { s.Redo() }
	}
	sync := func(int) {
		if undo != nil {
			undo.SetEnabled(s.CanUndo())
		}
		if redo != nil {
			redo.SetEnabled(s.CanRedo())
		}
	}
	sync(0)
	return s.Version().Subscribe(sync)
}
