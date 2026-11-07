// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-richdoc/richdoc"
)

var (
	_ Focusable = (*TextView)(nil)
	_ Focusable = (*CodeEditor)(nil) // inherited through the embedded TextView
	_ Focusable = (*RichEditor)(nil)
	_ Focusable = (*AddressBar)(nil)
)

// Each editor's Focusable methods read and write the SAME Observable that
// HasFocus() returns: one source of truth.
func TestEditorsFocusableSharesHasFocusObservable(t *testing.T) {
	check := func(name string, f Focusable, obs func() bool, set func(bool)) {
		t.Helper()
		f.SetFocused(true)
		if !obs() || !f.Focused() {
			t.Fatalf("%s: SetFocused(true) not visible through HasFocus()", name)
		}
		f.SetFocused(false)
		if obs() || f.Focused() {
			t.Fatalf("%s: SetFocused(false) not visible through HasFocus()", name)
		}
		set(true)
		if !f.Focused() {
			t.Fatalf("%s: HasFocus().Set(true) not visible through Focused()", name)
		}
		set(false)
		if f.Focused() {
			t.Fatalf("%s: HasFocus().Set(false) not visible through Focused()", name)
		}
	}
	tv := NewTextView("x")
	check("TextView", tv, tv.HasFocus().Get, tv.HasFocus().Set)
	ce := NewCodeEditor("x")
	check("CodeEditor", ce, ce.HasFocus().Get, ce.HasFocus().Set)
	re := NewRichEditor(richdoc.New().P(richdoc.Txt("x")).Doc())
	check("RichEditor", re, re.HasFocus().Get, re.HasFocus().Set)
	ab := &AddressBar{}
	check("AddressBar", ab, ab.HasFocus().Get, ab.HasFocus().Set)
}

// Tab traversal now visits the editors.
func TestTabTraversalVisitsEditors(t *testing.T) {
	tv := NewTextView("a")
	ce := NewCodeEditor("b")
	re := NewRichEditor(richdoc.New().P(richdoc.Txt("c")).Doc())
	ab := &AddressBar{}
	ws := []Focusable{tv, ce, re, ab}
	v := NewVBox()
	for i, w := range []Widget{tv, ce, re, ab} {
		w.SetBounds(Rect{X: 0, Y: i * 30, W: 100, H: 30})
		v.Append(w)
	}
	v.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 120})
	for i := range ws {
		v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		for j, w := range ws {
			if w.Focused() != (j == i) {
				t.Fatalf("Tab %d: widget %d focused=%v", i+1, j, w.Focused())
			}
		}
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if !ws[0].Focused() || ws[3].Focused() {
		t.Fatal("Tab from the last editor should wrap to the first")
	}
}
