// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Entry ---------------------------------------------------------------

func TestEntryClickFocuses(t *testing.T) {
	e := NewEntry("hi")
	e.OnEvent(Event{Kind: EventClick})
	if !e.Focused {
		t.Fatal("click should set Focused = true")
	}
}

func TestEntryConstructorParksCursorAtEnd(t *testing.T) {
	e := NewEntry("hello")
	if e.Cursor != 5 {
		t.Fatalf("Cursor = %d, want 5", e.Cursor)
	}
}

func TestEntryBackspaceDeletesAndFiresOnChange(t *testing.T) {
	changes := 0
	e := NewEntry("abc")
	e.OnChange = func(t string) { changes++ }
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if e.Text != "ab" || e.Cursor != 2 || changes != 1 {
		t.Fatalf("after Backspace: Text=%q Cursor=%d changes=%d", e.Text, e.Cursor, changes)
	}
}

func TestEntryBackspaceAtStartNoOp(t *testing.T) {
	e := NewEntry("ab")
	e.Cursor = 0
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if e.Text != "ab" || e.Cursor != 0 {
		t.Fatalf("backspace at start should be no-op")
	}
}

func TestEntryArrowKeysMoveCursor(t *testing.T) {
	e := NewEntry("ab")
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if e.Cursor != 1 {
		t.Fatalf("ArrowLeft: Cursor = %d, want 1", e.Cursor)
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if e.Cursor != 2 {
		t.Fatalf("ArrowRight: Cursor = %d, want 2", e.Cursor)
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // can't go past 0
	if e.Cursor != 0 {
		t.Fatalf("ArrowLeft clamp: Cursor = %d, want 0", e.Cursor)
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // can't go past end
	if e.Cursor != 2 {
		t.Fatalf("ArrowRight clamp: Cursor = %d, want 2", e.Cursor)
	}
}

func TestEntryHomeEnd(t *testing.T) {
	e := NewEntry("abc")
	e.Cursor = 1
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if e.Cursor != 0 {
		t.Fatalf("Home: Cursor = %d", e.Cursor)
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if e.Cursor != 3 {
		t.Fatalf("End: Cursor = %d", e.Cursor)
	}
}

func TestEntryEnterFiresOnSubmit(t *testing.T) {
	got := ""
	e := NewEntry("payload")
	e.OnSubmit = func(t string) { got = t }
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if got != "payload" {
		t.Fatalf("OnSubmit got %q", got)
	}
}

func TestEntryCharInsertsAndFiresOnChange(t *testing.T) {
	changes := 0
	e := NewEntry("ab")
	e.Cursor = 1
	e.OnChange = func(t string) { changes++ }
	e.OnEvent(Event{Kind: EventChar, Code: "X"})
	if e.Text != "aXb" || e.Cursor != 2 || changes != 1 {
		t.Fatalf("after Char: Text=%q Cursor=%d changes=%d", e.Text, e.Cursor, changes)
	}
}

func TestEntryEmptyCharIsNoOp(t *testing.T) {
	e := NewEntry("ab")
	e.OnEvent(Event{Kind: EventChar, Code: ""})
	if e.Text != "ab" {
		t.Fatal("empty Char should not mutate")
	}
}

func TestEntryUnknownKeyIsNoOp(t *testing.T) {
	e := NewEntry("ab")
	e.OnEvent(Event{Kind: EventKeyDown, Code: "F1"})
	if e.Text != "ab" {
		t.Fatal("F1 should not mutate")
	}
}

func TestEntryNilCallbacksNoPanic(t *testing.T) {
	e := NewEntry("ab")
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	e.OnEvent(Event{Kind: EventChar, Code: "Z"})
}

func TestEntryIgnoredEventKind(t *testing.T) {
	e := NewEntry("ab")
	e.OnEvent(Event{Kind: EventKeyUp, Code: "x"})
	if e.Text != "ab" {
		t.Fatal("KeyUp should not mutate")
	}
}

func TestEntryDrawFocusedShowsCursor(t *testing.T) {
	const w, h = 64, 24
	theme := DefaultLight()
	e := NewEntry("ab")
	e.Focused = true
	e.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	buf := makeSurface(w, h)
	e.Draw(newP(buf, w), theme)
	// Border in Accent (focused).
	if pixelAt(buf, w, 30, 0) != theme.Accent {
		t.Fatalf("focused top-edge border = %+v, want Accent", pixelAt(buf, w, 30, 0))
	}
}

func TestEntryDrawUnfocused(t *testing.T) {
	const w, h = 64, 24
	theme := DefaultLight()
	e := NewEntry("ab")
	e.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	buf := makeSurface(w, h)
	e.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 30, 0) != theme.Border {
		t.Fatalf("unfocused top-edge border = %+v, want Border", pixelAt(buf, w, 30, 0))
	}
}

// --- CheckButton ---------------------------------------------------------

func TestCheckButtonClickToggles(t *testing.T) {
	got := false
	c := NewCheckButton("OK", false)
	c.OnToggle = func(v bool) { got = v }
	c.OnEvent(Event{Kind: EventClick})
	if !c.Checked || !got {
		t.Fatalf("after click: Checked=%v got=%v", c.Checked, got)
	}
	c.OnEvent(Event{Kind: EventClick})
	if c.Checked || got {
		t.Fatalf("after second click: Checked=%v got=%v", c.Checked, got)
	}
}

func TestCheckButtonIgnoresOtherEvents(t *testing.T) {
	c := NewCheckButton("OK", false)
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if c.Checked {
		t.Fatal("KeyDown should not toggle")
	}
}

func TestCheckButtonNilCallbackNoPanic(t *testing.T) {
	c := NewCheckButton("OK", false)
	c.OnEvent(Event{Kind: EventClick})
}

func TestCheckButtonDrawCheckedAndUnchecked(t *testing.T) {
	const w, h = 80, 24
	theme := DefaultLight()
	c := NewCheckButton("OK", true)
	c.SetBounds(Rect{X: 2, Y: 4, W: 70, H: 16})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), theme)
	// Centre of the box (which is at x=2..14, y=6..18 with box centred
	// vertically inside H=16). Checked → Accent fill at (5, 10).
	if pixelAt(buf, w, 5, 10) != theme.Accent {
		t.Fatalf("checked box fill = %+v, want Accent", pixelAt(buf, w, 5, 10))
	}
	c.Checked = false
	buf2 := makeSurface(w, h)
	c.Draw(newP(buf2, w), theme)
	if pixelAt(buf2, w, 5, 10) != theme.Surface {
		t.Fatalf("unchecked box fill = %+v, want Surface", pixelAt(buf2, w, 5, 10))
	}
}

// --- RadioButton + RadioGroup --------------------------------------------

func TestRadioButtonStandaloneToggles(t *testing.T) {
	got := false
	r := NewRadioButton("A")
	r.OnToggle = func(v bool) { got = v }
	r.OnEvent(Event{Kind: EventClick})
	if !r.Checked || !got {
		t.Fatalf("standalone toggle: Checked=%v got=%v", r.Checked, got)
	}
}

func TestRadioButtonIgnoresNonClick(t *testing.T) {
	r := NewRadioButton("A")
	r.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if r.Checked {
		t.Fatal("KeyDown should not toggle a radio")
	}
}

func TestRadioGroupMutualExclusion(t *testing.T) {
	g := NewRadioGroup()
	a := NewRadioButton("A")
	b := NewRadioButton("B")
	c := NewRadioButton("C")
	g.Add(a)
	g.Add(b)
	g.Add(c)
	if g.Active != -1 {
		t.Fatalf("initial Active = %d, want -1", g.Active)
	}
	b.OnEvent(Event{Kind: EventClick})
	if !b.Checked || a.Checked || c.Checked || g.Active != 1 {
		t.Fatalf("after click B: a=%v b=%v c=%v active=%d", a.Checked, b.Checked, c.Checked, g.Active)
	}
	a.OnEvent(Event{Kind: EventClick})
	if !a.Checked || b.Checked || c.Checked || g.Active != 0 {
		t.Fatalf("after click A: a=%v b=%v c=%v active=%d", a.Checked, b.Checked, c.Checked, g.Active)
	}
}

func TestRadioGroupOnToggleFires(t *testing.T) {
	g := NewRadioGroup()
	r := NewRadioButton("X")
	got := false
	r.OnToggle = func(v bool) { got = v }
	g.Add(r)
	r.OnEvent(Event{Kind: EventClick})
	if !got {
		t.Fatal("OnToggle didn't fire on group activation")
	}
}

func TestRadioButtonStandaloneNilCallbackNoPanic(t *testing.T) {
	r := NewRadioButton("X")
	r.OnEvent(Event{Kind: EventClick}) // OnToggle == nil
}

func TestRadioGroupNilOnToggleNoPanic(t *testing.T) {
	g := NewRadioGroup()
	r := NewRadioButton("X")
	g.Add(r)
	r.OnEvent(Event{Kind: EventClick})
}

func TestRadioButtonDrawCheckedAndUnchecked(t *testing.T) {
	const w, h = 80, 24
	theme := DefaultLight()
	r := NewRadioButton("X")
	r.Checked = true
	r.SetBounds(Rect{X: 2, Y: 4, W: 70, H: 16})
	buf := makeSurface(w, h)
	r.Draw(newP(buf, w), theme)
	// Inner Accent dot at the centre.
	if pixelAt(buf, w, 8, 10) != theme.Accent {
		t.Fatalf("checked radio dot = %+v, want Accent", pixelAt(buf, w, 8, 10))
	}
	r.Checked = false
	buf2 := makeSurface(w, h)
	r.Draw(newP(buf2, w), theme)
	if pixelAt(buf2, w, 8, 10) != theme.Surface {
		t.Fatalf("unchecked radio interior = %+v, want Surface", pixelAt(buf2, w, 8, 10))
	}
}

// --- ToggleButton --------------------------------------------------------

func TestToggleButtonClickFlips(t *testing.T) {
	got := false
	tb := NewToggleButton("X", false)
	tb.OnToggle = func(v bool) { got = v }
	tb.OnEvent(Event{Kind: EventClick})
	if !tb.Pressed || !got {
		t.Fatalf("after click: Pressed=%v got=%v", tb.Pressed, got)
	}
	tb.OnEvent(Event{Kind: EventClick})
	if tb.Pressed || got {
		t.Fatalf("after second click: Pressed=%v got=%v", tb.Pressed, got)
	}
}

func TestToggleButtonNilCallbackNoPanic(t *testing.T) {
	tb := NewToggleButton("X", false)
	tb.OnEvent(Event{Kind: EventClick})
}

func TestToggleButtonIgnoresNonClick(t *testing.T) {
	tb := NewToggleButton("X", false)
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if tb.Pressed {
		t.Fatal("KeyDown should not toggle")
	}
}

func TestToggleButtonDrawPressedAndUnpressed(t *testing.T) {
	const w, h = 80, 24
	theme := DefaultLight()
	tb := NewToggleButton("X", true)
	tb.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	buf := makeSurface(w, h)
	tb.Draw(newP(buf, w), theme)
	// Pick a pixel far from the centred label glyph (which is around
	// x=27..32 for the 5-px "X") so the face fill is the only thing
	// reaching the sample.
	if pixelAt(buf, w, 5, 10) != theme.Accent {
		t.Fatalf("pressed face = %+v, want Accent", pixelAt(buf, w, 5, 10))
	}
	tb.Pressed = false
	buf2 := makeSurface(w, h)
	tb.Draw(newP(buf2, w), theme)
	if pixelAt(buf2, w, 5, 10) != theme.Surface {
		t.Fatalf("unpressed face = %+v, want Surface", pixelAt(buf2, w, 5, 10))
	}
}
