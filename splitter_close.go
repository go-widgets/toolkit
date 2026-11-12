// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/mvvm"

// Closing and reopening a pane.
//
// A pane is OPEN while its [SplitPane.Visible] Observable is true (NewSplitter
// gives every pane one). Closing a pane is writing false into it: the pane takes
// no room, its share goes to the panes that stay (the flexible ones by weight),
// and its divider goes with it. Nothing else is touched, so a pane that comes
// back comes back at the width it had: its Size or Weight were never written.
//
// The last pane refuses. A splitter with every pane closed is a blank window the
// person cannot get out of, so a close that would leave no [SplitPane.Essential]
// pane (no pane at all, when none is marked) is put back at once, whoever wrote
// it: the button, a menu toggle bound to the same Observable, a stored state.
// [Splitter.CanClose] answers beforehand, for a button to disable itself.

// Panes is the number of panes, open or not.
func (s *Splitter) Panes() int { return len(s.panes) }

// PaneName is pane i's accessible name.
func (s *Splitter) PaneName(i int) string { return s.panes[i].Name }

// Open is pane i's visibility: true while it is open. It is the pane's own
// Visible Observable, so a view-model binds it, persists it, and two-way binds a
// menu toggle to it.
func (s *Splitter) Open(i int) *mvvm.Observable[bool] { return s.panes[i].Visible }

// IsOpen reports whether pane i is open.
func (s *Splitter) IsOpen(i int) bool { return s.on(i) }

// mayClose reports whether the splitter keeps what it must with pane i off.
func (s *Splitter) mayClose(i int) bool {
	if s.anyEssential && !s.panes[i].Essential {
		return true
	}
	for j := range s.panes {
		if j != i && s.on(j) && (!s.anyEssential || s.panes[j].Essential) {
			return true
		}
	}
	return false
}

// CanClose reports whether pane i is open and may be closed: closing it would
// not take the last Essential pane (the last pane, when none is marked).
func (s *Splitter) CanClose(i int) bool { return s.on(i) && s.mayClose(i) }

// Close closes pane i and reports whether it did: false when it was already
// closed, or when it is the last one and refuses. Whatever held the keyboard in
// the pane, and the divider beside it, lets go of it.
func (s *Splitter) Close(i int) bool {
	if !s.CanClose(i) {
		return false
	}
	s.panes[i].Visible.Set(false)
	return true
}

// Reopen opens pane i again, at the size it had.
func (s *Splitter) Reopen(i int) { s.panes[i].Visible.Set(true) }

// release takes the keyboard off a pane that is closing: its widgets, and every
// divider (the dividers are numbered by the panes that are on, so one that
// is not beside the pane would still change its neighbours, and a ring left on
// one that goes off would be back, stale, when it returns).
func (s *Splitter) release(i int) {
	blurTree(s.panes[i].Widget)
	s.Blur()
}

// blurTree takes the keyboard off w and every focusable under it.
func blurTree(w Widget) {
	if f, ok := w.(Focusable); ok {
		f.SetFocused(false)
	}
	if ce, ok := w.(focusEnumerator); ok {
		for _, c := range ce.focusableChildren() {
			if c != nil {
				blurTree(c)
			}
		}
	}
}
