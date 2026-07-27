// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "strings"

// Selection is a (start, end) range of TextView positions. Positions
// are (line, col) pairs in rune coordinates -- same model the TextView
// cursor uses. Start <= End in canonical order; SelectionRange
// normalises any (anchor, cursor) pair the caller hands it.
//
// Selection is pure data; the TextView holds one + uses it to drive
// painting + range-delete + clipboard ops.
type Selection struct {
	StartLine, StartCol int
	EndLine, EndCol     int
}

// IsEmpty reports whether the selection covers zero characters.
func (s Selection) IsEmpty() bool {
	return s.StartLine == s.EndLine && s.StartCol == s.EndCol
}

// SelectionRange returns a canonical Selection from an anchor + a
// cursor: whichever pair is "earlier" in document order becomes the
// start.
func SelectionRange(anchorLine, anchorCol, cursorLine, cursorCol int) Selection {
	if anchorLine < cursorLine || (anchorLine == cursorLine && anchorCol <= cursorCol) {
		return Selection{anchorLine, anchorCol, cursorLine, cursorCol}
	}
	return Selection{cursorLine, cursorCol, anchorLine, anchorCol}
}

// SelectionText returns the substring covered by sel in lines (a
// TextView's Lines slice). Empty selection returns "".
func SelectionText(lines []string, sel Selection) string {
	if sel.IsEmpty() {
		return ""
	}
	if sel.StartLine == sel.EndLine {
		runes := []rune(lines[sel.StartLine])
		if sel.EndCol > len(runes) {
			sel.EndCol = len(runes)
		}
		return string(runes[sel.StartCol:sel.EndCol])
	}
	var b strings.Builder
	first := []rune(lines[sel.StartLine])
	b.WriteString(string(first[sel.StartCol:]))
	b.WriteByte('\n')
	for li := sel.StartLine + 1; li < sel.EndLine; li++ {
		b.WriteString(lines[li])
		b.WriteByte('\n')
	}
	last := []rune(lines[sel.EndLine])
	if sel.EndCol > len(last) {
		sel.EndCol = len(last)
	}
	b.WriteString(string(last[:sel.EndCol]))
	return b.String()
}

// DeleteSelection removes the selected range from lines + returns
// the new lines slice. The result always has at least one line (an
// empty line at minimum).
func DeleteSelection(lines []string, sel Selection) []string {
	if sel.IsEmpty() {
		return lines
	}
	first := []rune(lines[sel.StartLine])
	last := []rune(lines[sel.EndLine])
	if sel.EndCol > len(last) {
		sel.EndCol = len(last)
	}
	merged := string(first[:sel.StartCol]) + string(last[sel.EndCol:])
	out := make([]string, 0, len(lines)-(sel.EndLine-sel.StartLine))
	out = append(out, lines[:sel.StartLine]...)
	out = append(out, merged)
	out = append(out, lines[sel.EndLine+1:]...)
	return out
}

// --- TextView selection API ----------------------------------------------

// HasSelection reports whether the TextView's selection covers > 0
// characters.
func (t *TextView) HasSelection() bool { return !t.Selection.IsEmpty() }

// SelectionText returns the selected substring, or "".
func (t *TextView) SelectionText() string {
	return SelectionText(t.Lines, t.Selection)
}

// ClearSelection collapses the selection to (CursorLine, CursorCol).
func (t *TextView) ClearSelection() {
	t.Selection = Selection{t.CursorLine, t.CursorCol, t.CursorLine, t.CursorCol}
}

// SetSelection records a new (start, end) selection without moving
// the cursor.
func (t *TextView) SetSelection(sel Selection) { t.Selection = sel }

// SelectAll selects the entire buffer + parks the cursor at its end.
func (t *TextView) SelectAll() {
	n := len(t.Lines)
	if n == 0 {
		return
	}
	last := []rune(t.Lines[n-1])
	t.Selection = Selection{0, 0, n - 1, len(last)}
	t.CursorLine = n - 1
	t.CursorCol = len(last)
}

// DeleteSelection removes the selected text + parks the cursor at
// the deletion point. No-op when the selection is empty.
func (t *TextView) DeleteSelection() {
	if t.Selection.IsEmpty() {
		return
	}
	t.pushUndo()
	sel := t.Selection
	t.Lines = DeleteSelection(t.Lines, sel)
	t.CursorLine = sel.StartLine
	t.CursorCol = sel.StartCol
	t.ClearSelection()
	if t.OnChange != nil {
		t.OnChange()
	}
}

// CopySelection returns the selected text and, when non-empty, writes
// it to the toolkit's global Clipboard (see clipboard.go) so it can
// be pasted into any other text widget. Leaves the buffer untouched.
// An empty selection is a no-op on the clipboard (mirrors a
// Ctrl+C-with-nothing-selected not clobbering whatever was copied
// before).
func (t *TextView) CopySelection() string {
	s := t.SelectionText()
	if s != "" {
		SetClipboardText(s)
	}
	return s
}

// CutSelection returns the selected text, writes it to the global
// Clipboard (when non-empty) + removes it from the buffer.
func (t *TextView) CutSelection() string {
	s := t.SelectionText()
	if s != "" {
		SetClipboardText(s)
	}
	t.DeleteSelection()
	return s
}

// Paste inserts text at the cursor (after first deleting the
// selection if any). "\n" splits lines. The whole operation --
// selection removal + insertion -- is a single undo step. Callers
// that want to paste the toolkit's global Clipboard contents pass
// ClipboardText() (this is what the Ctrl+V key path does); Paste
// itself stays a plain "insert this text" primitive so callers can
// also use it to insert arbitrary programmatic text.
func (t *TextView) Paste(text string) {
	if t.HasSelection() {
		// DeleteSelection pushes undo; insertText below joins it into
		// the same step (no separate push).
		t.DeleteSelection()
	} else if text != "" {
		t.pushUndo()
	}
	t.insertText(text)
}
