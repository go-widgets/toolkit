// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// A disabled text widget must be inert for EDITING, not just announced as
// disabled: typing, IME composition, paste, cut, delete, newline/submit and the
// click that would focus it are all dropped. Caret keys and copy still work.

// editEvents is every kind of event that edits, or submits, a text widget.
func editEvents() []Event {
	return []Event{
		{Kind: EventChar, Code: "x"},
		{Kind: EventCompositionStart, Code: "k"},
		{Kind: EventCompositionUpdate, Code: "ka"},
		{Kind: EventCompositionEnd},
		{Kind: EventKeyDown, Code: "Backspace"},
		{Kind: EventKeyDown, Code: "Enter"},
		{Kind: EventKeyDown, Code: "Ctrl+X"},
		{Kind: EventKeyDown, Code: "Ctrl+V"},
		{Kind: EventKeyDown, Code: "Ctrl+Z"},
		{Kind: EventKeyDown, Code: "Ctrl+Y"},
		{Kind: EventKeyDown, Code: "Ctrl+Shift+Z"},
	}
}

func TestEditsTextClassifiesEvents(t *testing.T) {
	for _, ev := range editEvents() {
		if !editsText(ev) {
			t.Errorf("%+v should count as editing", ev)
		}
	}
	for _, ev := range []Event{
		{Kind: EventKeyDown, Code: "ArrowLeft"},
		{Kind: EventKeyDown, Code: "Ctrl+C"},
		{Kind: EventClick},
		{Kind: EventScroll},
	} {
		if editsText(ev) {
			t.Errorf("%+v should not count as editing", ev)
		}
	}
}

func TestDisabledEntryIgnoresEditing(t *testing.T) {
	e := NewEntry("ab")
	submitted := 0
	e.OnSubmit = func(string) { submitted++ }
	SetClipboardText("clip")
	e.Disabled().Set(true)
	for _, ev := range editEvents() {
		e.OnEvent(ev)
		if got := e.Text().Get(); got != "ab" {
			t.Fatalf("disabled Entry changed to %q on %+v", got, ev)
		}
	}
	if submitted != 0 {
		t.Errorf("disabled Entry submitted %d times", submitted)
	}
	if e.composition != "" {
		t.Errorf("disabled Entry kept an IME preview %q", e.composition)
	}
	e.OnEvent(Event{Kind: EventClick, X: 0})
	if e.focused {
		t.Error("a click focused a disabled Entry")
	}
	// Navigation and copy still work.
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if e.cursor != 1 {
		t.Errorf("caret = %d after ArrowLeft, want 1", e.cursor)
	}
	SetClipboardText("")
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+C"})
	if got := ClipboardText(); got != "ab" {
		t.Errorf("Ctrl+C copied %q, want ab", got)
	}
	// Enabled again, it edits.
	e.Disabled().Set(false)
	e.OnEvent(Event{Kind: EventChar, Code: "z"})
	if got := e.Text().Get(); got != "azb" {
		t.Errorf("re-enabled Entry = %q, want azb", got)
	}
}

func TestDisabledTextViewIgnoresEditing(t *testing.T) {
	v := NewTextView("ab")
	SetClipboardText("clip")
	v.SetSelection(SelectionRange(0, 0, 0, 1))
	v.Disabled().Set(true)
	for _, ev := range editEvents() {
		v.OnEvent(ev)
		if got := v.Text().Get(); got != "ab" {
			t.Fatalf("disabled TextView changed to %q on %+v", got, ev)
		}
	}
	if v.composition != "" {
		t.Errorf("disabled TextView kept an IME preview %q", v.composition)
	}
	v.OnEvent(Event{Kind: EventClick, X: 0, Y: 0})
	if v.Focused() {
		t.Error("a click focused a disabled TextView")
	}
	// The programmatic edit paths are inert too.
	v.Paste("zz")
	v.DeleteSelection()
	if got := v.CutSelection(); got != "" {
		t.Errorf("CutSelection returned %q on a disabled view", got)
	}
	v.ReplaceInput(0, 1, "q")
	v.SetMarkedInput("m", 0, 0, 0, 0)
	if got := v.Text().Get(); got != "ab" {
		t.Fatalf("disabled TextView changed to %q through its edit API", got)
	}
	if !v.InputReadOnly() {
		t.Error("a disabled TextView offers its input method a writable field")
	}
	// Copy and selection still work.
	SetClipboardText("")
	if got := v.CopySelection(); got != "a" || ClipboardText() != "a" {
		t.Errorf("copy = %q, clipboard %q, want a", got, ClipboardText())
	}
	// Enabled again, it edits.
	v.Disabled().Set(false)
	if v.InputReadOnly() {
		t.Error("an enabled TextView is read-only to its input method")
	}
	v.OnEvent(Event{Kind: EventChar, Code: "z"})
	if got := v.Text().Get(); got == "ab" {
		t.Error("re-enabled TextView ignored typing")
	}
}

func TestDisabledCodeEditorIgnoresTypingAndCompletion(t *testing.T) {
	c := NewCodeEditor("ab")
	c.Disabled().Set(true)
	for _, ev := range editEvents() {
		c.OnEvent(ev)
	}
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Space"})
	if got := c.Text().Get(); got != "ab" {
		t.Fatalf("disabled CodeEditor changed to %q", got)
	}
	if c.CompletionActive() {
		t.Error("a disabled CodeEditor opened its completion popup")
	}
	c.Disabled().Set(false)
	c.OnEvent(Event{Kind: EventChar, Code: "z"})
	if got := c.Text().Get(); got == "ab" {
		t.Error("re-enabled CodeEditor ignored typing")
	}
}
