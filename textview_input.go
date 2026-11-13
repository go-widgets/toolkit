// Copyright (c) the go-widgets authors.
// SPDX-License-Identifier: BSD-3-Clause

package toolkit

import (
	"strings"

	"github.com/go-widgets/textinput"
)

// TextView is a textinput.Client: an input method (an IME, a screen reader, a
// mobile keyboard) drives it through the Input* methods, which count UTF-16
// code units over the whole buffer, "\n" ending a line. CodeEditor inherits
// them through its embedded view.
var _ textinput.Client = (*TextView)(nil)

// InputText is the whole buffer, lines joined with "\n".
func (t *TextView) InputText() string { return strings.Join(t.lines, "\n") }

// InputRevision changes on every edit (and when a host replaces the text), so
// a host can skip rewriting its editing element when nothing changed.
func (t *TextView) InputRevision() uint64 { return t.rev }

// InputSelection is the selection with its direction: the focus is the caret,
// the anchor is the other end of the selection (anchor == focus is a caret).
func (t *TextView) InputSelection() (anchor, focus int) {
	text := t.InputText()
	cl, cc := t.CursorLine().Get(), t.CursorCol().Get()
	focus = textinput.Offset(text, cl, cc)
	sel := t.Selection().Get()
	if sel.IsEmpty() {
		return focus, focus
	}
	if sel.StartLine == cl && sel.StartCol == cc {
		return textinput.Offset(text, sel.EndLine, sel.EndCol), focus
	}
	return textinput.Offset(text, sel.StartLine, sel.StartCol), focus
}

// InputMarked is the marked (composition) range, or (-1, -1) without one.
func (t *TextView) InputMarked() (start, end int) {
	if !t.marked {
		return -1, -1
	}
	return t.markStart, t.markEnd
}

// InputMultiline is true: Enter splits the line.
func (t *TextView) InputMultiline() bool { return true }

// InputReadOnly is true while the view is Disabled (an input method then offers
// no composition) and false otherwise: a TextView has no read-only mode.
func (t *TextView) InputReadOnly() bool { return t.Disabled().Get() }

// SetInputSelection selects from anchor to focus and puts the caret at focus.
// A negative offset means "none" and changes nothing.
func (t *TextView) SetInputSelection(anchor, focus int) {
	if anchor < 0 || focus < 0 {
		return
	}
	t.selectOffsets(anchor, focus)
	t.scrollCaretIntoView()
}

// selectOffsets selects anchor..focus (UTF-16 offsets in the current buffer)
// with the caret at focus, and remembers the anchor for a following drag.
func (t *TextView) selectOffsets(anchor, focus int) {
	text := t.InputText()
	al, ac := textinput.LineCol(text, anchor)
	fl, fc := textinput.LineCol(text, focus)
	t.CursorLine().Set(fl)
	t.CursorCol().Set(fc)
	t.selAnchorLine, t.selAnchorCol = al, ac
	t.Selection().Set(SelectionRange(al, ac, fl, fc))
}

// resolve turns an input range into byte offsets of the current text (start
// <= end, a surrogate pair never split) and the UTF-16 offset of the start.
// (-1, -1) is def: the range an input method means by "here".
func resolve(text string, start, end int, def func() (int, int)) (b0, b1, s0 int) {
	if start < 0 && end < 0 {
		start, end = def()
	}
	b0, b1 = textinput.ToByte(text, start), textinput.ToByte(text, end)
	if b0 > b1 {
		b0, b1 = b1, b0
	}
	return b0, b1, textinput.FromByte(text, b0)
}

// selectionOrMark is the range of "here" for ReplaceInput: the marked text
// when there is one (AppKit's insertText: with no replacement range commits
// over it), else the selection.
func (t *TextView) selectionOrMark() (int, int) {
	if t.marked {
		return t.markStart, t.markEnd
	}
	return t.selectionRange()
}

// selectionRange is the selection as ordered UTF-16 offsets.
func (t *TextView) selectionRange() (int, int) {
	a, f := t.InputSelection()
	if a > f {
		return f, a
	}
	return a, f
}

// splice replaces the bytes [b0, b1) of the buffer with ins, puts the caret at
// UTF-16 offset caret of the new buffer, and publishes the edit.
func (t *TextView) splice(b0, b1 int, ins string, caret int) {
	text := t.InputText()
	t.lines = strings.Split(text[:b0]+ins+text[b1:], "\n")
	t.colsOK = false
	l, c := textinput.LineCol(t.InputText(), caret)
	t.CursorLine().Set(l)
	t.CursorCol().Set(c)
	t.ClearSelection()
	t.composition = ""
	t.sync()
}

// ReplaceInput replaces [start, end) with text as one undoable edit and puts
// the caret after it. (-1, -1) means the marked text if there is some, else
// the selection. Committing over marked text ends the composition and does not
// add an undo step of its own: the composition already owns one.
func (t *TextView) ReplaceInput(start, end int, text string) {
	if t.Disabled().Get() {
		return
	}
	b0, b1, s0 := resolve(t.InputText(), start, end, t.selectionOrMark)
	if b0 == b1 && text == "" {
		return
	}
	if !t.marked {
		t.pushUndo()
	}
	t.splice(b0, b1, text, s0+textinput.Len(text))
	t.scrollCaretIntoView()
}

// SetMarkedInput replaces [replStart, replEnd) with text and marks it as the
// composition. (-1, -1) means the marked text, or the selection when there is
// none. selStart and selEnd are the selection inside text, relative to its
// start (clamped to it). The whole composition, however many updates, is one
// undo step.
func (t *TextView) SetMarkedInput(text string, selStart, selEnd, replStart, replEnd int) {
	if t.Disabled().Get() {
		return
	}
	b0, b1, s0 := resolve(t.InputText(), replStart, replEnd, t.selectionOrMark)
	if !t.marked {
		if b0 == b1 && text == "" {
			return // nothing to mark, nothing to undo
		}
		t.pushUndo()
	}
	n := textinput.Len(text)
	t.splice(b0, b1, text, s0+n)
	t.marked, t.markStart, t.markEnd = true, s0, s0+n
	t.selectOffsets(s0+clampInt(selStart, 0, n), s0+clampInt(selEnd, 0, n))
	t.scrollCaretIntoView()
}

// UnmarkInput commits the marked text as typed. It does nothing without
// marked text.
func (t *TextView) UnmarkInput() {
	if !t.marked {
		return
	}
	t.marked = false
	t.rev++
}

// InputRect returns the pixels, in the same space as Bounds, of the first line
// of [start, end): the caret cell's origin, the width of the range on that
// line (0 for a caret) and the line height.
func (t *TextView) InputRect(start, end int) (x, y, w, h int) {
	if start > end {
		start, end = end, start
	}
	text := t.InputText()
	sl, sc := textinput.LineCol(text, start)
	el, ec := textinput.LineCol(text, end)
	if el > sl {
		ec = len([]rune(t.lines[sl]))
	}
	x, y = t.CaretPixel(sl, sc)
	return x, y, (ec - sc) * t.glyphAdvance(), t.glyphHeight() + lineGap()
}

// InputIndexAt returns the offset nearest to the point (x, y), given in the
// same space as Bounds.
func (t *TextView) InputIndexAt(x, y int) int {
	if len(t.lines) == 0 {
		return 0
	}
	r := t.Bounds()
	l, c := t.caretAt(x-r.X, y-r.Y)
	return textinput.Offset(t.InputText(), l, c)
}
