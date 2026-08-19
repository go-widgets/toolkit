// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Entry ---------------------------------------------------------------

func TestEntryClickFocuses(t *testing.T) {
	e := NewEntry("hi")
	e.OnEvent(Event{Kind: EventClick})
	if !e.Focused() {
		t.Fatal("click should set Focused = true")
	}
}

func TestEntryConstructorParksCursorAtEnd(t *testing.T) {
	e := NewEntry("hello")
	if e.cursor != 5 {
		t.Fatalf("Cursor = %d, want 5", e.cursor)
	}
}

func TestEntryValueReturnsText(t *testing.T) {
	e := NewEntry("hello")
	if v := e.Value(); v != "hello" {
		t.Fatalf("Value() = %q, want %q", v, "hello")
	}
	e.Text().Set("changed")
	if v := e.Value(); v != "changed" {
		t.Fatalf("Value() = %q, want %q", v, "changed")
	}
}

func TestEntryBackspaceDeletesAndFiresOnChange(t *testing.T) {
	changes := 0
	e := NewEntry("abc")
	e.Text().Subscribe(func(t string) { changes++ })
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if e.Text().Get() != "ab" || e.cursor != 2 || changes != 1 {
		t.Fatalf("after Backspace: Text=%q Cursor=%d changes=%d", e.Text().Get(), e.cursor, changes)
	}
}

func TestEntryBackspaceAtStartNoOp(t *testing.T) {
	e := NewEntry("ab")
	e.cursor = 0
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if e.Text().Get() != "ab" || e.cursor != 0 {
		t.Fatalf("backspace at start should be no-op")
	}
}

func TestEntryArrowKeysMoveCursor(t *testing.T) {
	e := NewEntry("ab")
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if e.cursor != 1 {
		t.Fatalf("ArrowLeft: Cursor = %d, want 1", e.cursor)
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if e.cursor != 2 {
		t.Fatalf("ArrowRight: Cursor = %d, want 2", e.cursor)
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // can't go past 0
	if e.cursor != 0 {
		t.Fatalf("ArrowLeft clamp: Cursor = %d, want 0", e.cursor)
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // can't go past end
	if e.cursor != 2 {
		t.Fatalf("ArrowRight clamp: Cursor = %d, want 2", e.cursor)
	}
}

func TestEntryHomeEnd(t *testing.T) {
	e := NewEntry("abc")
	e.cursor = 1
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if e.cursor != 0 {
		t.Fatalf("Home: Cursor = %d", e.cursor)
	}
	e.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if e.cursor != 3 {
		t.Fatalf("End: Cursor = %d", e.cursor)
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
	e.cursor = 1
	e.Text().Subscribe(func(t string) { changes++ })
	e.OnEvent(Event{Kind: EventChar, Code: "X"})
	if e.Text().Get() != "aXb" || e.cursor != 2 || changes != 1 {
		t.Fatalf("after Char: Text=%q Cursor=%d changes=%d", e.Text().Get(), e.cursor, changes)
	}
}

func TestEntryEmptyCharIsNoOp(t *testing.T) {
	e := NewEntry("ab")
	e.OnEvent(Event{Kind: EventChar, Code: ""})
	if e.Text().Get() != "ab" {
		t.Fatal("empty Char should not mutate")
	}
}

func TestEntryUnknownKeyIsNoOp(t *testing.T) {
	e := NewEntry("ab")
	e.OnEvent(Event{Kind: EventKeyDown, Code: "F1"})
	if e.Text().Get() != "ab" {
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
	if e.Text().Get() != "ab" {
		t.Fatal("KeyUp should not mutate")
	}
}

func TestEntryDrawFocusedShowsCursor(t *testing.T) {
	const w, h = 64, 24
	theme := DefaultLight()
	e := NewEntry("ab")
	e.SetFocused(true)
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

func TestEntryCompositionStartUpdateEnd(t *testing.T) {
	e := NewEntry("abc")
	// Start: preview becomes visible; Text untouched.
	e.OnEvent(Event{Kind: EventCompositionStart, Code: "^"})
	if e.composition != "^" {
		t.Fatalf("start: Composition=%q", e.composition)
	}
	if e.Text().Get() != "abc" {
		t.Fatalf("start must not touch Text, got %q", e.Text().Get())
	}
	// Update: preview refreshed.
	e.OnEvent(Event{Kind: EventCompositionUpdate, Code: "ê"})
	if e.composition != "ê" {
		t.Fatalf("update: Composition=%q", e.composition)
	}
	if e.Text().Get() != "abc" {
		t.Fatalf("update must not touch Text, got %q", e.Text().Get())
	}
	// End (cancel path): preview cleared, Text unchanged.
	e.OnEvent(Event{Kind: EventCompositionEnd, Code: ""})
	if e.composition != "" {
		t.Fatalf("end: Composition should clear, got %q", e.composition)
	}
	if e.Text().Get() != "abc" {
		t.Fatal("end (cancel) must not touch Text")
	}
}

func TestEntryCompositionCommitViaEventChar(t *testing.T) {
	changes := 0
	e := NewEntry("abc")
	e.Text().Subscribe(func(t string) { changes++ })
	// Preview.
	e.OnEvent(Event{Kind: EventCompositionStart, Code: "^"})
	// Host now commits by delivering EventChar with the composed rune.
	e.OnEvent(Event{Kind: EventChar, Code: "ê"})
	if e.composition != "" {
		t.Fatal("EventChar must clear the composition preview")
	}
	if e.Text().Get() != "abcê" || changes != 1 {
		t.Fatalf("commit: Text=%q changes=%d", e.Text().Get(), changes)
	}
}

func TestEntryCompositionCancelledDiscardsPreviewNoChar(t *testing.T) {
	e := NewEntry("abc")
	e.OnEvent(Event{Kind: EventCompositionStart, Code: "^"})
	e.OnEvent(Event{Kind: EventCompositionUpdate, Code: "^a"})
	// Cancelled: host does NOT send EventChar.
	e.OnEvent(Event{Kind: EventCompositionEnd})
	if e.composition != "" {
		t.Fatalf("cancelled composition should discard preview, got %q", e.composition)
	}
	if e.Text().Get() != "abc" {
		t.Fatalf("cancelled composition must not mutate Text, got %q", e.Text().Get())
	}
}

func TestEntryCompositionEmptyCharAfterStartStillClearsPreview(t *testing.T) {
	e := NewEntry("ab")
	e.OnEvent(Event{Kind: EventCompositionStart, Code: "^"})
	e.OnEvent(Event{Kind: EventChar, Code: ""})
	if e.composition != "" {
		t.Fatal("empty EventChar should still clear the composition preview")
	}
	if e.Text().Get() != "ab" {
		t.Fatal("empty EventChar should not mutate Text")
	}
}

func TestEntryCompositionInteractsWithCursorPosition(t *testing.T) {
	e := NewEntry("ab")
	e.cursor = 1
	e.OnEvent(Event{Kind: EventCompositionStart, Code: "^"})
	if e.cursor != 1 {
		t.Fatalf("composition start should not move the caret, Cursor=%d", e.cursor)
	}
	e.OnEvent(Event{Kind: EventChar, Code: "x"})
	if e.Text().Get() != "axb" || e.cursor != 2 {
		t.Fatalf("commit at mid-string cursor: Text=%q Cursor=%d", e.Text().Get(), e.cursor)
	}
}

func TestEntryDrawCompositionPreviewDistinctFromCommittedText(t *testing.T) {
	const w, h = 64, 24
	theme := DefaultLight()
	e := NewEntry("ab")
	e.SetFocused(true)
	e.composition = "^"
	e.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	buf := makeSurface(w, h)
	e.Draw(newP(buf, w), theme)
	// No pixel-precise assertion beyond exercising the preview branch;
	// the buffer must at least differ from the no-composition render.
	buf2 := makeSurface(w, h)
	e2 := NewEntry("ab")
	e2.SetFocused(true)
	e2.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	e2.Draw(newP(buf2, w), theme)
	if string(buf) == string(buf2) {
		t.Fatal("composition preview should render visibly distinct from the no-composition frame")
	}
}

func TestEntryDrawNoCompositionUnchanged(t *testing.T) {
	// Regression: with Composition == "" (the zero value), Draw must be
	// byte-identical to the pre-IME rendering path.
	const w, h = 64, 24
	theme := DefaultLight()
	e := NewEntry("ab")
	e.SetFocused(true)
	e.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	buf := makeSurface(w, h)
	e.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 30, 0) != theme.Accent {
		t.Fatalf("focused top-edge border = %+v, want Accent", pixelAt(buf, w, 30, 0))
	}
}

func TestEntryCtrlCCopiesWholeValueToClipboard(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	e := NewEntry("hello")
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+C"})
	if got := ClipboardText(); got != "hello" {
		t.Fatalf("Ctrl+C clipboard = %q, want hello", got)
	}
	if e.Text().Get() != "hello" {
		t.Fatal("Ctrl+C must not mutate Text")
	}
}

func TestEntryCtrlCEmptyTextDoesNotTouchClipboard(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	SetClipboardText("previous")
	e := NewEntry("")
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+C"})
	if got := ClipboardText(); got != "previous" {
		t.Fatalf("empty-value Ctrl+C must not clobber clipboard, got %q", got)
	}
}

func TestEntryCtrlXCutsWholeValueToClipboardAndClears(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	changes := 0
	e := NewEntry("hello")
	e.Text().Subscribe(func(t string) { changes++ })
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+X"})
	if got := ClipboardText(); got != "hello" {
		t.Fatalf("Ctrl+X clipboard = %q, want hello", got)
	}
	if e.Text().Get() != "" || e.cursor != 0 || changes != 1 {
		t.Fatalf("after Ctrl+X: Text=%q Cursor=%d changes=%d", e.Text().Get(), e.cursor, changes)
	}
}

func TestEntryCtrlXEmptyTextIsNoOp(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	SetClipboardText("previous")
	changes := 0
	e := NewEntry("")
	e.Text().Subscribe(func(t string) { changes++ })
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+X"})
	if got := ClipboardText(); got != "previous" {
		t.Fatalf("empty-value Ctrl+X must not clobber clipboard, got %q", got)
	}
	if changes != 0 {
		t.Fatalf("empty-value Ctrl+X must not fire OnChange, changes=%d", changes)
	}
}

func TestEntryCtrlVPastesClipboardAtCursor(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	SetClipboardText("XY")
	changes := 0
	e := NewEntry("ab")
	e.cursor = 1
	e.Text().Subscribe(func(t string) { changes++ })
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+V"})
	if e.Text().Get() != "aXYb" || e.cursor != 3 || changes != 1 {
		t.Fatalf("after Ctrl+V: Text=%q Cursor=%d changes=%d", e.Text().Get(), e.cursor, changes)
	}
}

func TestEntryCtrlVEmptyClipboardIsNoOp(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	changes := 0
	e := NewEntry("ab")
	e.Text().Subscribe(func(t string) { changes++ })
	e.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+V"})
	if e.Text().Get() != "ab" || changes != 0 {
		t.Fatalf("empty-clipboard Ctrl+V: Text=%q changes=%d", e.Text().Get(), changes)
	}
}

func TestEntryCopyThenPasteAcrossEntriesRoundTrip(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	src := NewEntry("copied")
	src.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+C"})

	dst := NewEntry("")
	dst.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+V"})
	if dst.Text().Get() != "copied" {
		t.Fatalf("cross-Entry paste = %q, want copied", dst.Text().Get())
	}
}

// --- CheckButton ---------------------------------------------------------

func TestCheckButtonClickToggles(t *testing.T) {
	got := false
	c := NewCheckButton("OK", false)
	c.Checked().Subscribe(func(v bool) { got = v })
	c.OnEvent(Event{Kind: EventClick})
	if !c.Checked().Get() || !got {
		t.Fatalf("after click: Checked=%v got=%v", c.Checked().Get(), got)
	}
	c.OnEvent(Event{Kind: EventClick})
	if c.Checked().Get() || got {
		t.Fatalf("after second click: Checked=%v got=%v", c.Checked().Get(), got)
	}
}

func TestCheckButtonIgnoresOtherEvents(t *testing.T) {
	c := NewCheckButton("OK", false)
	// Space/Enter toggle as of Wave 3; an unrelated key (Tab) must not.
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if c.Checked().Get() {
		t.Fatal("KeyDown should not toggle")
	}
}

func TestCheckButtonNoSubscriberNoPanic(t *testing.T) {
	c := NewCheckButton("OK", false)
	c.OnEvent(Event{Kind: EventClick})
}

// A bare &CheckButton{} has a nil observable; the Checked() accessor must
// lazy-init it (to false) so a host can bind it and a toggle works, matching
// the constructor path.
func TestCheckButtonZeroValueAccessor(t *testing.T) {
	c := &CheckButton{}
	if c.Checked().Get() {
		t.Fatal("zero-value CheckButton should be unchecked")
	}
	got := false
	c.Checked().Subscribe(func(v bool) { got = v })
	c.OnEvent(Event{Kind: EventClick})
	if !c.Checked().Get() || !got {
		t.Fatalf("after click on zero value: Checked=%v got=%v", c.Checked().Get(), got)
	}
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
	c.Checked().Set(false)
	buf2 := makeSurface(w, h)
	c.Draw(newP(buf2, w), theme)
	if pixelAt(buf2, w, 5, 10) != theme.Surface {
		t.Fatalf("unchecked box fill = %+v, want Surface", pixelAt(buf2, w, 5, 10))
	}
}

// A Sized checkbox scales its box and tick; checking it must change the render,
// exercising boxSize(Size>0) and the scaled drawCheckmark.
func TestCheckButtonSizedCheckmark(t *testing.T) {
	const w, h = 60, 40
	theme := DefaultLight()
	draw := func(checked bool) []byte {
		c := NewCheckButton("", checked)
		c.Size = 24 // larger than the 12px default
		c.SetBounds(Rect{X: 2, Y: 2, W: 40, H: 36})
		buf := makeSurface(w, h)
		c.Draw(newP(buf, w), theme)
		return buf
	}
	on, off := draw(true), draw(false)
	if string(on) == string(off) {
		t.Fatal("a checked sized box should render its scaled tick, differing from unchecked")
	}
	// The 24px box's centre is Accent-filled when checked.
	if pixelAt(on, w, 2+12, 2+18) != theme.Accent {
		t.Fatalf("sized checked box centre = %+v, want Accent", pixelAt(on, w, 14, 20))
	}
	// A tiny sized box (b/12 < 1) clamps the stroke width to 1 and must not panic.
	small := NewCheckButton("", true)
	small.Size = 6
	small.SetBounds(Rect{X: 1, Y: 1, W: 20, H: 10})
	small.Draw(newP(makeSurface(w, h), w), theme)
}

// --- RadioButton + RadioGroup --------------------------------------------

func TestRadioButtonStandaloneToggles(t *testing.T) {
	got := false
	r := NewRadioButton("A")
	r.Checked().Subscribe(func(v bool) { got = v })
	r.OnEvent(Event{Kind: EventClick})
	if !r.Checked().Get() || !got {
		t.Fatalf("standalone toggle: Checked=%v got=%v", r.Checked().Get(), got)
	}
}

func TestRadioButtonIgnoresNonClick(t *testing.T) {
	r := NewRadioButton("A")
	// A standalone radio toggles on Space/Enter as of Wave 3; an unrelated key
	// (Tab) must not.
	r.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if r.Checked().Get() {
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
	if g.Active().Get() != -1 {
		t.Fatalf("initial Active = %d, want -1", g.Active().Get())
	}
	b.OnEvent(Event{Kind: EventClick})
	if !b.Checked().Get() || a.Checked().Get() || c.Checked().Get() || g.Active().Get() != 1 {
		t.Fatalf("after click B: a=%v b=%v c=%v active=%d", a.Checked().Get(), b.Checked().Get(), c.Checked().Get(), g.Active().Get())
	}
	a.OnEvent(Event{Kind: EventClick})
	if !a.Checked().Get() || b.Checked().Get() || c.Checked().Get() || g.Active().Get() != 0 {
		t.Fatalf("after click A: a=%v b=%v c=%v active=%d", a.Checked().Get(), b.Checked().Get(), c.Checked().Get(), g.Active().Get())
	}
}

// TestRadioGroupBindingsFireOnActivation proves a host observes a group
// selection purely through the members' Checked() Observables and the group's
// Active() Observable — the callbacks (OnToggle/OnChange) are gone.
func TestRadioGroupBindingsFireOnActivation(t *testing.T) {
	g := NewRadioGroup()
	r := NewRadioButton("X")
	checked := false
	r.Checked().Subscribe(func(v bool) { checked = v })
	active := -1
	g.Active().Subscribe(func(v int) { active = v })
	g.Add(r)
	r.OnEvent(Event{Kind: EventClick})
	if !checked || active != 0 {
		t.Fatalf("group activation bindings: checked=%v active=%d, want true/0", checked, active)
	}
}

func TestRadioGroupNilBindingNoPanic(t *testing.T) {
	g := NewRadioGroup()
	r := NewRadioButton("X")
	g.Add(r)
	r.OnEvent(Event{Kind: EventClick}) // no subscribers bound
}

// TestRadioButtonBareAccessorInits proves the Checked accessor lazily
// initialises to false on a bare &RadioButton{} (nil observable), and a host can
// bind it before any interaction.
func TestRadioButtonBareAccessorInits(t *testing.T) {
	r := &RadioButton{}
	if r.Checked().Get() {
		t.Fatal("bare RadioButton Checked() = true, want false")
	}
	got := false
	r.Checked().Subscribe(func(v bool) { got = v })
	r.Checked().Set(true)
	if !got || !r.Checked().Get() {
		t.Fatalf("host bind: got=%v Checked=%v", got, r.Checked().Get())
	}
}

// TestRadioGroupBareAccessorInits proves the Active accessor lazily initialises
// to 0 on a bare &RadioGroup{} (nil observable), and a host can bind it.
func TestRadioGroupBareAccessorInits(t *testing.T) {
	g := &RadioGroup{}
	if g.Active().Get() != 0 {
		t.Fatalf("bare RadioGroup Active() = %d, want 0", g.Active().Get())
	}
	got := -99
	g.Active().Subscribe(func(v int) { got = v })
	g.Active().Set(2)
	if got != 2 || g.Active().Get() != 2 {
		t.Fatalf("host bind: got=%d Active=%d", got, g.Active().Get())
	}
}

func TestRadioButtonDrawCheckedAndUnchecked(t *testing.T) {
	const w, h = 80, 24
	theme := DefaultLight()
	r := NewRadioButton("X")
	r.Checked().Set(true)
	r.SetBounds(Rect{X: 2, Y: 4, W: 70, H: 16})
	buf := makeSurface(w, h)
	r.Draw(newP(buf, w), theme)
	// Inner Accent dot at the centre.
	if pixelAt(buf, w, 8, 10) != theme.Accent {
		t.Fatalf("checked radio dot = %+v, want Accent", pixelAt(buf, w, 8, 10))
	}
	r.Checked().Set(false)
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
	tb.Pressed().Subscribe(func(v bool) { got = v })
	tb.OnEvent(Event{Kind: EventClick})
	if !tb.Pressed().Get() || !got {
		t.Fatalf("after click: Pressed=%v got=%v", tb.Pressed().Get(), got)
	}
	tb.OnEvent(Event{Kind: EventClick})
	if tb.Pressed().Get() || got {
		t.Fatalf("after second click: Pressed=%v got=%v", tb.Pressed().Get(), got)
	}
}

// TestToggleButtonPressedObservable covers the zero-value lazy-init of the
// Pressed accessor and the host binding path: a ToggleButton built as a bare
// struct (no NewToggleButton) still yields a usable Observable, and Setting it
// from outside is reflected by the widget (there is no imperative Pressed field).
func TestToggleButtonPressedObservable(t *testing.T) {
	tb := &ToggleButton{} // no NewToggleButton → pressed Observable is nil until accessed
	if tb.Pressed().Get() {
		t.Fatalf("zero-value ToggleButton Pressed = true, want false")
	}
	seen := false
	sawTrue := false
	tb.Pressed().Subscribe(func(v bool) { seen = v; sawTrue = sawTrue || v })
	tb.Pressed().Set(true) // a host drives the toggle through the Observable
	if !tb.Pressed().Get() || !seen || !sawTrue {
		t.Fatalf("host Set: value=%v subscriber=%v, want true/true", tb.Pressed().Get(), seen)
	}
}

func TestToggleButtonIgnoresNonClick(t *testing.T) {
	tb := NewToggleButton("X", false)
	// Space/Enter toggle as of Wave 3; an unrelated key (Tab) must not.
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if tb.Pressed().Get() {
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
	tb.Pressed().Set(false)
	buf2 := makeSurface(w, h)
	tb.Draw(newP(buf2, w), theme)
	if pixelAt(buf2, w, 5, 10) != theme.Surface {
		t.Fatalf("unpressed face = %+v, want Surface", pixelAt(buf2, w, 5, 10))
	}
}

func TestEntrySetText(t *testing.T) {
	e := NewEntry("ab")
	var got string
	e.Text().Subscribe(func(s string) { got = s })
	e.SetText("wörld")
	if e.Text().Get() != "wörld" || got != "wörld" {
		t.Fatalf("SetText: text=%q sub=%q", e.Text().Get(), got)
	}
	if e.cursor != 5 {
		t.Fatalf("SetText must park caret at end: cursor=%d, want 5", e.cursor)
	}
}
