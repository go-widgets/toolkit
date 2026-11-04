// Copyright (c) the go-widgets authors.
// SPDX-License-Identifier: BSD-3-Clause

package toolkit

import (
	"testing"

	"github.com/go-widgets/textinput"
)

func caretOf(t *testing.T, v *TextView) (int, int) {
	t.Helper()
	a, f := v.InputSelection()
	if a != f {
		t.Fatalf("expected a caret, got selection %d..%d", a, f)
	}
	return a, f
}

func TestTextViewInputGetters(t *testing.T) {
	v := NewTextView("ab\ncd")
	var c textinput.Client = v
	var _ textinput.Client = NewCodeEditor("")
	if c.InputText() != "ab\ncd" || !c.InputMultiline() || c.InputReadOnly() {
		t.Fatal("text, multiline or read-only wrong")
	}
	if s, e := c.InputMarked(); s != -1 || e != -1 {
		t.Fatalf("no composition must give (-1,-1), got (%d,%d)", s, e)
	}
	r0 := c.InputRevision()
	v.SetText("x")
	if c.InputRevision() == r0 {
		t.Fatal("revision must change on an edit")
	}
	r1 := c.InputRevision()
	v.Text().Set("host") // a host-driven Set reloads the buffer
	if c.InputRevision() == r1 || c.InputText() != "host" {
		t.Fatal("revision must change when a host replaces the text")
	}
	if textinput.StateOf(c).Text != "host" {
		t.Fatal("StateOf must snapshot the view")
	}
}

func TestTextViewInputSelectionDirection(t *testing.T) {
	v := NewTextView("héllo\nworld")
	// forward: anchor before the caret
	v.SetInputSelection(1, 8)
	if a, f := v.InputSelection(); a != 1 || f != 8 {
		t.Fatalf("forward: got %d..%d", a, f)
	}
	if v.CursorLine().Get() != 1 || v.CursorCol().Get() != 2 || v.SelectionText() != "éllo\nwo" {
		t.Fatalf("forward: caret %d:%d sel %q", v.CursorLine().Get(), v.CursorCol().Get(), v.SelectionText())
	}
	// backward: anchor after the caret, direction kept
	v.SetInputSelection(8, 1)
	if a, f := v.InputSelection(); a != 8 || f != 1 {
		t.Fatalf("backward: got %d..%d", a, f)
	}
	// a negative offset means none: nothing changes
	v.SetInputSelection(-1, 3)
	v.SetInputSelection(3, -1)
	if a, f := v.InputSelection(); a != 8 || f != 1 {
		t.Fatalf("negative offsets must be ignored, got %d..%d", a, f)
	}
	// out-of-range offsets clamp to the document
	v.SetInputSelection(99, 99)
	if a, f := v.InputSelection(); a != 11 || f != 11 {
		t.Fatalf("clamp: got %d..%d", a, f)
	}
}

// A selection made backwards (focus before anchor) is still replaced whole by
// ReplaceInput(-1, -1, ...), and the caret ends after the new text.
func TestTextViewReplaceInputBackwardSelection(t *testing.T) {
	v := NewTextView("hello world")
	v.SetInputSelection(5, 0) // "hello", anchor after focus
	v.ReplaceInput(-1, -1, "bye")
	if v.InputText() != "bye world" {
		t.Fatalf("got %q", v.InputText())
	}
	if p, _ := caretOf(t, v); p != 3 {
		t.Fatalf("caret = %d, want 3", p)
	}
}

func TestTextViewReplaceInput(t *testing.T) {
	v := NewTextView("hello world")
	v.SetInputSelection(0, 5)
	v.ReplaceInput(-1, -1, "bye") // the selection
	if v.InputText() != "bye world" {
		t.Fatalf("got %q", v.InputText())
	}
	if p, _ := caretOf(t, v); p != 3 {
		t.Fatalf("caret after replace = %d, want 3", p)
	}
	v.ReplaceInput(9, 4, "LD\nX") // reversed range is put in order, "\n" splits
	if v.InputText() != "bye LD\nX" || len(v.lines) != 2 {
		t.Fatalf("got %q", v.InputText())
	}
	if p, _ := caretOf(t, v); p != 8 {
		t.Fatalf("caret = %d, want 8", p)
	}
	rev := v.InputRevision()
	v.ReplaceInput(2, 2, "") // empty over empty: nothing, not even an undo step
	if v.InputRevision() != rev {
		t.Fatal("a no-op must not change the revision")
	}
	v.Undo()
	if v.InputText() != "bye world" {
		t.Fatalf("undo: got %q", v.InputText())
	}
	v.Undo()
	if v.InputText() != "hello world" {
		t.Fatalf("second undo: got %q", v.InputText())
	}
}

// An IME session as AppKit drives it: setMarkedText several times, then
// insertText with no replacement range commits over the marked text.
func TestTextViewIMEMarkedSession(t *testing.T) {
	v := NewTextView("ab")
	v.SetInputSelection(1, 1) // a|b

	v.SetMarkedInput("k", 1, 1, -1, -1)
	if v.InputText() != "akb" {
		t.Fatalf("after k: %q", v.InputText())
	}
	if s, e := v.InputMarked(); s != 1 || e != 2 {
		t.Fatalf("marked = (%d,%d), want (1,2)", s, e)
	}
	v.SetMarkedInput("かん", 2, 2, -1, -1) // replaces the marked text
	if v.InputText() != "aかんb" {
		t.Fatalf("after かん: %q", v.InputText())
	}
	if s, e := v.InputMarked(); s != 1 || e != 3 {
		t.Fatalf("marked = (%d,%d), want (1,3)", s, e)
	}
	if p, _ := caretOf(t, v); p != 3 {
		t.Fatalf("caret = %d, want 3", p)
	}
	// the candidate selection inside the marked text (a clause being converted)
	v.SetMarkedInput("漢字", 0, 2, -1, -1)
	if a, f := v.InputSelection(); a != 1 || f != 3 {
		t.Fatalf("selection inside marked text = %d..%d, want 1..3", a, f)
	}

	v.ReplaceInput(-1, -1, "漢字") // commit
	if v.InputText() != "a漢字b" {
		t.Fatalf("after commit: %q", v.InputText())
	}
	if s, e := v.InputMarked(); s != -1 || e != -1 {
		t.Fatalf("composition must end, marked = (%d,%d)", s, e)
	}
	if p, _ := caretOf(t, v); p != 3 {
		t.Fatalf("caret after commit = %d, want 3", p)
	}
	if v.CursorLine().Get() != 0 || v.CursorCol().Get() != 3 {
		t.Fatalf("caret %d:%d", v.CursorLine().Get(), v.CursorCol().Get())
	}
	v.Undo() // the whole composition is one undo step
	if v.InputText() != "ab" {
		t.Fatalf("one undo must remove the composition, got %q", v.InputText())
	}
}

// Offsets are UTF-16 code units: U+1F600 counts for two, and an offset inside
// the pair rounds down to its start.
func TestTextViewInputNonBMP(t *testing.T) {
	v := NewTextView("a😀b\n😀")
	if textinput.Len(v.InputText()) != 7 {
		t.Fatal("test text should be 7 UTF-16 units")
	}
	v.CursorCol().Set(2) // a😀|b
	if p, _ := caretOf(t, v); p != 3 {
		t.Fatalf("caret after the emoji = %d UTF-16 units, want 3", p)
	}
	v.SetInputSelection(1, 3) // exactly the emoji
	if v.SelectionText() != "😀" {
		t.Fatalf("selection = %q", v.SelectionText())
	}
	v.SetInputSelection(2, 3) // starts inside the pair: rounds down to its start
	if v.SelectionText() != "😀" {
		t.Fatalf("rounded selection = %q", v.SelectionText())
	}
	v.ReplaceInput(1, 3, "x")
	if v.InputText() != "axb\n😀" {
		t.Fatalf("got %q", v.InputText())
	}
	// the second line's emoji sits at 4..6; an end inside it rounds down to 4,
	// so 4..5 is empty and inserts, and the caret lands at 4 + 4 UTF-16 units
	v.ReplaceInput(4, 5, "🙂🙂")
	if v.InputText() != "axb\n🙂🙂😀" {
		t.Fatalf("got %q", v.InputText())
	}
	if p, _ := caretOf(t, v); p != 8 {
		t.Fatalf("caret = %d, want 4+4", p)
	}
	// a marked non-BMP text spans two units per rune
	v.SetInputSelection(0, 0)
	v.SetMarkedInput("😀😀", 4, 4, -1, -1)
	if s, e := v.InputMarked(); s != 0 || e != 4 {
		t.Fatalf("marked = (%d,%d), want (0,4)", s, e)
	}
	if p, _ := caretOf(t, v); p != 4 {
		t.Fatalf("caret = %d, want 4", p)
	}
	if v.CursorCol().Get() != 2 {
		t.Fatalf("rune column = %d, want 2", v.CursorCol().Get())
	}
	v.ReplaceInput(-1, -1, "ok")
	if v.InputText() != "okaxb\n🙂🙂😀" {
		t.Fatalf("got %q", v.InputText())
	}
}

func TestTextViewMarkedReplacesAndEnds(t *testing.T) {
	v := NewTextView("hello world")
	// no selection, nothing to mark: no-op and no undo step
	rev := v.InputRevision()
	v.SetMarkedInput("", 0, 0, -1, -1)
	if v.InputRevision() != rev {
		t.Fatal("marking nothing must not edit")
	}
	if len(v.undo) != 0 {
		t.Fatal("marking nothing must not push an undo step")
	}
	// a marked text may replace earlier text (Android: the word under the caret)
	v.SetMarkedInput("HELLO", 99, -5, 0, 5) // selection clamped into the text
	if v.InputText() != "HELLO world" {
		t.Fatalf("got %q", v.InputText())
	}
	if a, f := v.InputSelection(); a != 5 || f != 0 {
		// selStart 99 clamps to 5 and selEnd -5 to 0: anchor 5, focus 0
		t.Fatalf("selection = %d..%d", a, f)
	}
	// emptying the marked text while composing is allowed
	v.SetMarkedInput("", 0, 0, -1, -1)
	if v.InputText() != " world" {
		t.Fatalf("got %q", v.InputText())
	}
	// any other edit ends the composition
	v.SetMarkedInput("x", 1, 1, 0, 0)
	if s, _ := v.InputMarked(); s != 0 {
		t.Fatal("expected a marked range")
	}
	v.OnEvent(Event{Kind: EventChar, Code: "y"})
	if s, e := v.InputMarked(); s != -1 || e != -1 {
		t.Fatalf("an EventChar must end the composition, got (%d,%d)", s, e)
	}
	// UnmarkInput commits the text as typed and keeps it
	v.SetMarkedInput("zz", 2, 2, -1, -1)
	text, r := v.InputText(), v.InputRevision()
	v.UnmarkInput()
	if v.InputText() != text || v.InputRevision() == r {
		t.Fatal("UnmarkInput keeps the text and changes the revision")
	}
	if s, e := v.InputMarked(); s != -1 || e != -1 {
		t.Fatal("UnmarkInput must end the composition")
	}
	r = v.InputRevision()
	v.UnmarkInput() // nothing marked: nothing happens
	if v.InputRevision() != r {
		t.Fatal("UnmarkInput without a composition must do nothing")
	}
	// a host replacing the text ends a composition
	v.SetMarkedInput("q", 1, 1, -1, -1)
	v.Text().Set("fresh")
	if s, _ := v.InputMarked(); s != -1 {
		t.Fatal("a host Set must end the composition")
	}
}

func TestTextViewInputGeometry(t *testing.T) {
	v := NewTextView("abcd\nef\nghijkl")
	v.SetBounds(Rect{X: 10, Y: 20, W: 400, H: 200})
	adv, lineH := v.glyphAdvance(), v.glyphHeight()+4

	// a caret: zero width at the caret cell
	x, y, w, h := v.InputRect(2, 2)
	cx, cy := v.CaretPixel(0, 2)
	if x != cx || y != cy || w != 0 || h != lineH {
		t.Fatalf("caret rect = %d,%d %dx%d", x, y, w, h)
	}
	// a range inside a line, given backwards
	x, y, w, _ = v.InputRect(3, 1)
	cx, cy = v.CaretPixel(0, 1)
	if x != cx || y != cy || w != 2*adv {
		t.Fatalf("range rect = %d,%d w=%d", x, y, w)
	}
	// a range spanning lines: the first line only, up to its end
	_, _, w, _ = v.InputRect(2, 12)
	if w != 2*adv {
		t.Fatalf("multi-line first-line width = %d, want %d", w, 2*adv)
	}

	// InputIndexAt inverts InputRect: the pixel of a cell gives its offset
	for _, off := range []int{0, 3, 5, 6, 8, 9, 14} {
		px, py, _, _ := v.InputRect(off, off)
		if got := v.InputIndexAt(px, py+1); got != off {
			t.Errorf("InputIndexAt(rect of %d) = %d", off, got)
		}
	}
	if got := v.InputIndexAt(10000, 20+4+lineH+1); got != 7 { // far right of "ef"
		t.Fatalf("far right of line 1 = %d, want 7", got)
	}
	if got := (&TextView{}).InputIndexAt(5, 5); got != 0 {
		t.Fatalf("empty view = %d, want 0", got)
	}
}
