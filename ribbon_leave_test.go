// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// OnLeave tells the host the keyboard use of the ribbon is over, so it can give
// the keyboard back to the editor: a command run from the keyboard, Escape on
// the tabs with nothing open, the keytips ended. A click does not call it (the
// pointer already told the host where the focus went).
func TestRibbonOnLeave(t *testing.T) {
	r, run := rtRibbon(2000)
	left := 0
	r.OnLeave = func() { left++ }
	r.SetFocused(true)

	// Escape on the tabs with nothing open leaves.
	rtKey(r, "Escape")
	if left != 1 {
		t.Errorf("Escape on the tabs: left %d times", left)
	}
	// A peeking body is put away first, and that is not leaving.
	r.Collapsed().Set(true)
	r.peek = true
	rtKey(r, "Escape")
	if left != 1 || r.peek {
		t.Errorf("Escape over a peeking body should only close it: left %d", left)
	}
	r.Collapsed().Set(false)
	// A command run from the keyboard leaves.
	rtKey(r, "ArrowDown")
	rtKey(r, "ArrowRight")
	rtKey(r, "Enter")
	if left != 2 || len(run.ids) != 1 {
		t.Errorf("Enter on a control: left %d, ran %v", left, run.ids)
	}
	// A command run by the pointer does not.
	rtClickCell(t, r, "Undo")
	if left != 2 {
		t.Errorf("a click must not call OnLeave, left %d", left)
	}
	// The keytips: running a control leaves, Escape out of them leaves, Alt again leaves.
	rtKey(r, "Alt")
	rtKey(r, "H")
	rtKey(r, "U")
	if left != 3 {
		t.Errorf("a keytip that ran a command: left %d", left)
	}
	rtKey(r, "Alt")
	rtKey(r, "Escape")
	if left != 4 {
		t.Errorf("Escape out of the keytips: left %d", left)
	}
	rtKey(r, "Alt")
	rtKey(r, "Alt")
	if left != 5 {
		t.Errorf("Alt toggling the keytips off: left %d", left)
	}
	// Going back from the control tips to the tab tips is not leaving.
	rtKey(r, "Alt")
	rtKey(r, "H")
	rtKey(r, "Escape")
	if left != 5 || r.keytips != 1 {
		t.Errorf("Escape from the control tips to the tab tips: left %d keytips %d", left, r.keytips)
	}
	// A ribbon with no hook just carries on.
	r.OnLeave = nil
	rtKey(r, "Escape")
	rtKey(r, "Escape")
}

func TestRibbonMeasure(t *testing.T) {
	r, _ := rtRibbon(2000)
	if w, h := r.Measure(777, 5); w != 777 || h != r.Height() {
		t.Errorf("Measure = %d,%d", w, h)
	}
	r.Collapsed().Set(true)
	if _, h := r.Measure(777, 5); h != r.metrics().tabH+1 {
		t.Errorf("collapsed Measure height = %d", h)
	}
	// In a VBox it is what AddNatural reads: the box gives it exactly that height.
	box := NewVBox()
	box.AddNatural(r)
	box.AddFlex(NewLabel("content"), 1)
	box.SetBounds(Rect{W: 800, H: 400})
	if got := r.Bounds().H; got != r.Height() {
		t.Errorf("VBox gave the ribbon %d, want %d", got, r.Height())
	}
	r.Collapsed().Set(false)
	box.SetBounds(Rect{W: 800, H: 400})
	if got := r.Bounds().H; got != r.Height() {
		t.Errorf("after expanding, VBox gave the ribbon %d, want %d", got, r.Height())
	}
}
