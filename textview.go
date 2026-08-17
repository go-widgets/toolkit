// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

import (
	"strconv"
	"strings"
)

// TextSpan is a coloured run within a single line, in half-open rune
// coordinates [Start, End). A TextView.Highlighter returns a slice of
// these to paint syntax-highlighted source: any rune not covered by a
// span keeps the default ink. Spans may overlap — a later span in the
// slice wins for the runes it covers. Start/End are clamped to the
// line's rune length at paint time, so a highlighter need not worry
// about off-by-one bounds. Color reuses the toolkit's RGBA (a painter
// colour), so a highlighter composes theme colours directly.
type TextSpan struct {
	Start, End int
	Color      RGBA
}

// TextView is the multi-line cousin of Entry. Lines are stored as a
// []string (one element per visible line); Cursor is a (line, col)
// position in rune coordinates. Wraps Entry's keyboard model with an
// added vertical axis (ArrowUp / ArrowDown / PageUp / PageDown).
//
// This is the foundation a native wasmdesk editor builds on top of:
// syntax highlighting, search/replace and find can live above
// TextView without it growing those concerns. v0.3 ships the raw
// buffer; v0.4 will add a SelectionStart/End pair for range ops.
type TextView struct {
	Base
	Lines      []string
	CursorLine int
	CursorCol  int
	Focused    bool
	OnChange   func()

	// ScrollLine is the index of the buffer line painted at the top of the
	// viewport — the vertical scroll offset that makes a buffer taller than the
	// bounds reachable. Draw windows from here, the wheel (EventScroll) shifts
	// it, and every cursor move scrolls it to keep the caret visible. Reads
	// clamp on the fly (clampedScrollLine), so a stale value after the buffer
	// shrank is harmless; at ScrollLine == 0 rendering is byte-identical to
	// before scrolling existed.
	ScrollLine int

	// Selection is the (start, end) range the host paints highlighted
	// + range-deletes via DeleteSelection / cut+paste via
	// CopySelection / CutSelection / Paste. An empty selection (Start
	// == End) means "no selection"; HasSelection() is the convenience
	// predicate.
	Selection Selection

	// Composition holds the in-progress IME preview string (dead-key
	// output, CJK candidate, …). Non-empty while an IME composition
	// is active; cleared on EventCompositionEnd. The Draw method
	// paints it in a muted colour at the cursor position — the
	// preview is NOT part of the buffer until the host commits via
	// EventChar. Widgets that read Lines/Text() see only committed
	// text, so downstream logic (search, syntax, autosave) never
	// operates on half-formed input.
	Composition string

	// Highlighter, when non-nil, turns each line into coloured runs at
	// paint time: Draw calls Highlighter(lineIndex, line) and paints
	// the returned TextSpans in their colours, with any uncovered runes
	// falling back to the default ink (theme.OnSurface). When nil (the
	// zero value) Draw paints every line in a single ink exactly as it
	// always has — a host adds syntax highlighting by setting this hook
	// without the widget growing a lexer of its own.
	Highlighter func(lineIndex int, line string) []TextSpan

	// ShowLineNumbers, when true, reserves a left gutter sized to the
	// widest line number and paints right-aligned 1-based line numbers
	// there; the text, caret and selection all shift right by the
	// gutter width. When false (the zero value) there is no gutter and
	// layout is byte-identical to before this field existed.
	ShowLineNumbers bool

	// GutterColor is the ink for the line-number gutter. The zero value
	// (a fully-transparent RGBA, A==0) means "unset": Draw then falls
	// back to a muted tone (dimInk) that reads on any theme.
	GutterColor RGBA

	// RowBackground, when non-nil, is consulted once per visible buffer
	// line to paint a full-width background band behind that line — over
	// the Surface fill, under the gutter number, the ink and the caret.
	// It returns (colour, true) to paint the band in colour, or
	// (_, false) to leave the row on the plain Surface. This is the seam
	// a CodeEditor uses for its current-line highlight, a search UI for
	// match rows, or a diff view for added / removed rows, without
	// TextView growing any of those concerns. When nil (the zero value)
	// no band is painted and rendering is byte-identical to before this
	// field existed.
	RowBackground func(lineIndex int) (RGBA, bool)

	// undo/redo hold point-in-time snapshots taken before each
	// mutating edit (see pushUndo). Ports the go-widgets/tui
	// TextEditor's undo model: one snapshot per mutation (no
	// coalescing of consecutive keystrokes -- simplest correct
	// behaviour, matching the sibling's documented choice).
	undo, redo []tvSnapshot

	// selAnchorLine/selAnchorCol remember where a mouse selection began: an
	// EventClick records the caret it placed as the anchor, and each
	// EventMouseDrag extends Selection from that anchor to the dragged-to
	// caret. Kept private -- callers drive programmatic selection through
	// SetSelection/SelectAll instead.
	selAnchorLine, selAnchorCol int
}

// maxUndo caps the undo history so a long editing session can't grow
// the snapshot stack without bound.
const maxUndo = 200

// tvSnapshot is a point-in-time copy of the buffer + cursor +
// selection, used to restore a prior state on Undo/Redo.
type tvSnapshot struct {
	lines []string
	line  int
	col   int
	sel   Selection
}

// snapshot captures the current buffer + cursor + selection.
func (t *TextView) snapshot() tvSnapshot {
	cp := make([]string, len(t.Lines))
	copy(cp, t.Lines)
	return tvSnapshot{lines: cp, line: t.CursorLine, col: t.CursorCol, sel: t.Selection}
}

// restore replaces the buffer + cursor + selection with s.
func (t *TextView) restore(s tvSnapshot) {
	t.Lines = s.lines
	t.CursorLine, t.CursorCol = s.line, s.col
	t.Selection = s.sel
}

// pushUndo records the current state before a mutation and drops any
// redo history -- a fresh edit invalidates the redo branch. Callers
// invoke this immediately before the mutation it should undo, and
// only when the operation will actually mutate the buffer (so purely
// no-op edits -- e.g. Backspace at the buffer start, an empty
// EventChar -- don't waste a stack slot).
func (t *TextView) pushUndo() {
	t.undo = append(t.undo, t.snapshot())
	if len(t.undo) > maxUndo {
		t.undo = t.undo[len(t.undo)-maxUndo:]
	}
	t.redo = nil
}

// Undo restores the buffer + cursor + selection to the state before
// the most recent mutation, pushing the current state onto the redo
// stack. No-op when there is nothing to undo.
func (t *TextView) Undo() {
	if len(t.undo) == 0 {
		return
	}
	t.redo = append(t.redo, t.snapshot())
	last := t.undo[len(t.undo)-1]
	t.undo = t.undo[:len(t.undo)-1]
	t.restore(last)
	if t.OnChange != nil {
		t.OnChange()
	}
}

// Redo re-applies the most recently undone mutation, pushing the
// current state back onto the undo stack. No-op when there is
// nothing to redo.
func (t *TextView) Redo() {
	if len(t.redo) == 0 {
		return
	}
	t.undo = append(t.undo, t.snapshot())
	last := t.redo[len(t.redo)-1]
	t.redo = t.redo[:len(t.redo)-1]
	t.restore(last)
	if t.OnChange != nil {
		t.OnChange()
	}
}

// NewTextView builds a TextView pre-loaded with initial text (split
// on "\n"). Empty initial text creates a single empty line so the
// cursor always has a row to live on.
func NewTextView(initial string) *TextView {
	if initial == "" {
		return &TextView{Lines: []string{""}}
	}
	return &TextView{Lines: strings.Split(initial, "\n")}
}

// Text returns the buffer's concatenated content with "\n" line
// terminators. Mirrors strings.Join(Lines, "\n").
func (t *TextView) Text() string { return strings.Join(t.Lines, "\n") }

// SetText replaces the entire buffer + parks the cursor at (0,0).
func (t *TextView) SetText(s string) {
	if s == "" {
		t.Lines = []string{""}
	} else {
		t.Lines = strings.Split(s, "\n")
	}
	t.CursorLine = 0
	t.CursorCol = 0
}

// Draw paints border + fill + every visible line + (when Focused) a
// 1-px vertical cursor stroke at the cursor's screen position.
//
// Lines that would render past the bottom of the bounds are
// painted-but-clipped by the raster helpers; wrap in a ScrollView
// for proper scrollable behaviour.
func (t *TextView) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	border := theme.Border
	if t.Focused {
		border = theme.Accent
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, border)
	lineH := t.glyphHeight() + 4 // 1-pixel-line font + 4 px line spacing
	gutterW := t.gutterWidth()
	textX := r.X + 4 + gutterW
	gutterInk := t.GutterColor
	if gutterInk.A == 0 {
		gutterInk = dimInk(theme)
	}
	// Window from ScrollLine so a buffer taller than the bounds is reachable;
	// clip to the bounds so a partially-visible trailing line never bleeds past
	// the bottom edge into a neighbour. At ScrollLine == 0 with a buffer that
	// fits, start is 0 and every line is drawn exactly where it was before.
	start := t.clampedScrollLine()
	withClip(p, r, func() {
		for i := start; i < len(t.Lines); i++ {
			y := r.Y + 4 + (i-start)*lineH
			if y >= r.Y+r.H {
				break // fully below the viewport
			}
			line := t.Lines[i]
			// Full-width row band (current-line highlight, search match, diff
			// row): painted first so the gutter number, ink and caret land on
			// top. The band brackets the glyph box (2 px above) and spans one
			// full line pitch, so consecutive bands tile without a seam.
			if t.RowBackground != nil {
				if bg, ok := t.RowBackground(i); ok {
					fillRect(p, r.X+1, y-2, r.W-2, lineH, bg)
				}
			}
			if t.ShowLineNumbers {
				num := strconv.Itoa(i + 1)
				// Right-align the number against the text's left margin.
				nx := textX - 4 - t.textWidth(num)
				t.drawText(p, nx, y, num, gutterInk)
			}
			if t.Highlighter == nil {
				t.drawText(p, textX, y, line, theme.OnSurface)
				continue
			}
			t.drawSpans(p, textX, y, line, t.Highlighter(i, line), theme.OnSurface)
		}
	})
	if t.Focused {
		cx := textX + t.CursorCol*t.glyphAdvance()
		cy := r.Y + 4 + (t.CursorLine-start)*lineH
		fillRect(p, cx, cy-1, 1, t.glyphHeight()+2, theme.OnSurface)
		// IME composition preview: render the pending string in the
		// muted SurfaceAlt tone starting at the cursor, so the user
		// sees dead-key / CJK candidates without them entering the
		// buffer. Underlined by a 1-px SurfaceAlt strip beneath.
		if t.Composition != "" {
			cw := t.textWidth(t.Composition)
			t.drawText(p, cx, cy, t.Composition, theme.SurfaceAlt)
			fillRect(p, cx, cy+t.glyphHeight(), cw, 1, theme.SurfaceAlt)
		}
	}
}

// OnEvent dispatches the editing operations.
func (t *TextView) OnEvent(ev Event) {
	switch ev.Kind {
	case EventClick:
		t.Focused = true
		if len(t.Lines) == 0 {
			return
		}
		// Place the caret under the click (mapping ev.X/ev.Y back through
		// Draw's line/col layout) and start a fresh, collapsed selection
		// anchored there so a following drag can extend it.
		t.CursorLine, t.CursorCol = t.caretAt(ev.X, ev.Y)
		t.selAnchorLine, t.selAnchorCol = t.CursorLine, t.CursorCol
		t.ClearSelection()
		t.scrollCaretIntoView()
	case EventScroll:
		// Native wheel scroll: shift the viewport by Delta lines (clamped at
		// both ends). Independent of the caret, exactly like every sibling
		// content widget that gained native scroll in v0.98.0.
		t.scrollBy(ev.Delta)
	case EventMouseDrag:
		if len(t.Lines) == 0 {
			return
		}
		// Extend the selection from the click anchor to the caret under the
		// dragged pointer, moving the cursor with it.
		t.CursorLine, t.CursorCol = t.caretAt(ev.X, ev.Y)
		t.Selection = SelectionRange(t.selAnchorLine, t.selAnchorCol, t.CursorLine, t.CursorCol)
		t.scrollCaretIntoView()
	case EventKeyDown:
		t.handleKey(ev.Code)
		t.scrollCaretIntoView()
	case EventChar:
		// If an IME composition was in flight, the incoming char is
		// the commit result — clear the preview BEFORE inserting so
		// the buffer + display stay consistent.
		t.Composition = ""
		if ev.Code != "" {
			t.pushUndo()
		}
		t.insertText(ev.Code)
		t.scrollCaretIntoView()
	case EventCompositionStart, EventCompositionUpdate:
		// Preview only — do NOT touch Lines. Repaint responsibility
		// lies with the host, who typically calls the widget's Draw
		// method after each composition event.
		t.Composition = ev.Code
	case EventCompositionEnd:
		// Cancel / commit-without-follow-up: drop the preview. When
		// the host follows up with EventChar (commit path), the
		// EventChar arm above will re-clear + insert.
		t.Composition = ""
	}
}

// handleKey runs the per-key navigation + delete operations.
func (t *TextView) handleKey(code string) {
	switch code {
	case "Backspace":
		if t.CursorCol > 0 || t.CursorLine > 0 {
			t.pushUndo()
		}
		t.backspace()
	case "Enter":
		t.pushUndo()
		t.splitLine()
	case "Ctrl+Z":
		t.Undo()
	case "Ctrl+Y", "Ctrl+Shift+Z":
		t.Redo()
	case "Ctrl+C":
		t.CopySelection()
	case "Ctrl+X":
		t.CutSelection()
	case "Ctrl+V":
		t.Paste(ClipboardText())
	case "ArrowLeft":
		t.cursorLeft()
	case "ArrowRight":
		t.cursorRight()
	case "ArrowUp":
		if t.CursorLine > 0 {
			t.CursorLine--
			t.clampCol()
		}
	case "ArrowDown":
		if t.CursorLine < len(t.Lines)-1 {
			t.CursorLine++
			t.clampCol()
		}
	case "Home":
		t.CursorCol = 0
	case "End":
		t.CursorCol = len([]rune(t.Lines[t.CursorLine]))
	}
}

// insertText inserts s at the cursor; "\n" inside s splits lines.
func (t *TextView) insertText(s string) {
	if s == "" {
		return
	}
	for _, ch := range s {
		if ch == '\n' {
			t.splitLine()
			continue
		}
		runes := []rune(t.Lines[t.CursorLine])
		runes = append(runes[:t.CursorCol], append([]rune{ch}, runes[t.CursorCol:]...)...)
		t.Lines[t.CursorLine] = string(runes)
		t.CursorCol++
	}
	if t.OnChange != nil {
		t.OnChange()
	}
}

// splitLine breaks the current line at the cursor + moves the
// cursor to col 0 of the new line.
func (t *TextView) splitLine() {
	cur := []rune(t.Lines[t.CursorLine])
	left := string(cur[:t.CursorCol])
	right := string(cur[t.CursorCol:])
	t.Lines[t.CursorLine] = left
	t.Lines = append(t.Lines[:t.CursorLine+1], append([]string{right}, t.Lines[t.CursorLine+1:]...)...)
	t.CursorLine++
	t.CursorCol = 0
	if t.OnChange != nil {
		t.OnChange()
	}
}

// backspace removes the char before the cursor (or merges lines).
func (t *TextView) backspace() {
	if t.CursorCol > 0 {
		runes := []rune(t.Lines[t.CursorLine])
		t.Lines[t.CursorLine] = string(append(runes[:t.CursorCol-1], runes[t.CursorCol:]...))
		t.CursorCol--
		if t.OnChange != nil {
			t.OnChange()
		}
		return
	}
	if t.CursorLine == 0 {
		return // at buffer start; nothing to delete
	}
	prev := []rune(t.Lines[t.CursorLine-1])
	t.CursorCol = len(prev)
	t.Lines[t.CursorLine-1] = string(prev) + t.Lines[t.CursorLine]
	t.Lines = append(t.Lines[:t.CursorLine], t.Lines[t.CursorLine+1:]...)
	t.CursorLine--
	if t.OnChange != nil {
		t.OnChange()
	}
}

// cursorLeft handles the wrap-to-previous-line case.
func (t *TextView) cursorLeft() {
	if t.CursorCol > 0 {
		t.CursorCol--
		return
	}
	if t.CursorLine > 0 {
		t.CursorLine--
		t.CursorCol = len([]rune(t.Lines[t.CursorLine]))
	}
}

// cursorRight handles the wrap-to-next-line case.
func (t *TextView) cursorRight() {
	line := []rune(t.Lines[t.CursorLine])
	if t.CursorCol < len(line) {
		t.CursorCol++
		return
	}
	if t.CursorLine < len(t.Lines)-1 {
		t.CursorLine++
		t.CursorCol = 0
	}
}

// visibleLines is how many whole buffer lines fit vertically within the bounds
// (minus the 4px top inset) at the current line height. Floored at 1 so a very
// short widget still shows the caret's line and scrolling stays well-defined. A
// non-positive height or line height both collapse to 0 — no lines fit.
func (t *TextView) visibleLines() int {
	lineH := t.glyphHeight() + 4 // > 0: glyphHeight is always positive
	h := t.Bounds().H - 4
	if h <= 0 {
		return 0
	}
	n := h / lineH
	if n < 1 {
		n = 1
	}
	return n
}

// maxScrollLine is the highest ScrollLine that still leaves a full window of
// lines on screen: len(Lines) - visibleLines(), floored at 0 so a buffer that
// already fits never scrolls.
func (t *TextView) maxScrollLine() int {
	m := len(t.Lines) - t.visibleLines()
	if m < 0 {
		m = 0
	}
	return m
}

// clampedScrollLine returns ScrollLine clamped to [0, maxScrollLine()] WITHOUT
// mutating the field, so an out-of-range value (set directly, or left stale
// after Lines shrank) never paints or hit-tests outside the valid window.
func (t *TextView) clampedScrollLine() int {
	s := t.ScrollLine
	if s < 0 {
		s = 0
	}
	if m := t.maxScrollLine(); s > m {
		s = m
	}
	return s
}

// scrollBy shifts ScrollLine by delta lines (negative scrolls up), clamped to
// [0, maxScrollLine()] and written back immediately.
func (t *TextView) scrollBy(delta int) {
	t.ScrollLine += delta
	t.ScrollLine = t.clampedScrollLine()
}

// scrollCaretIntoView nudges ScrollLine so the caret's line (CursorLine) stays
// within the visible window: up if it sits above the top line, down if at or
// past the last visible line. Called after every cursor move so typing or
// arrowing off the visible region follows the caret (Wave 4 keep-caret-visible).
func (t *TextView) scrollCaretIntoView() {
	vis := t.visibleLines()
	if vis <= 0 {
		return
	}
	if t.CursorLine < t.ScrollLine {
		t.ScrollLine = t.CursorLine
	} else if t.CursorLine >= t.ScrollLine+vis {
		t.ScrollLine = t.CursorLine - vis + 1
	}
	t.ScrollLine = t.clampedScrollLine()
}

// gutterWidth is the pixel width reserved for the line-number gutter,
// or 0 when ShowLineNumbers is false. It sizes to the widest number
// (the last line's) plus 8 px of padding, so the number column never
// jitters as the cursor moves between rows of different index widths.
func (t *TextView) gutterWidth() int {
	if !t.ShowLineNumbers {
		return 0
	}
	widest := strconv.Itoa(len(t.Lines))
	return t.textWidth(widest) + 8
}

// drawSpans paints line at (x, y) as coloured runs: every rune starts
// at the base ink, spans overwrite the runes they cover (later spans
// win), and consecutive same-colour runes are coalesced into a single
// drawText so proportional fonts stay correctly kerned within a run.
// Out-of-range span bounds are clamped, so a highlighter can be sloppy
// about edges without corrupting the paint.
func (t *TextView) drawSpans(p painter.Painter, x, y int, line string, spans []TextSpan, base RGBA) {
	runes := []rune(line)
	if len(runes) == 0 {
		return
	}
	colors := make([]RGBA, len(runes))
	for i := range colors {
		colors[i] = base
	}
	for _, sp := range spans {
		s, e := sp.Start, sp.End
		if s < 0 {
			s = 0
		}
		if e > len(runes) {
			e = len(runes)
		}
		for j := s; j < e; j++ {
			colors[j] = sp.Color
		}
	}
	cx := x
	runStart := 0
	for i := 1; i <= len(runes); i++ {
		if i == len(runes) || colors[i] != colors[runStart] {
			seg := string(runes[runStart:i])
			t.drawText(p, cx, y, seg, colors[runStart])
			cx += t.textWidth(seg)
			runStart = i
		}
	}
}

// caretAt maps a widget-local (x, y) to a (line, col) caret position -- the
// inverse of Draw's placement, where line i sits at local y == 4+i*lineH and
// column c at local x == 4+gutterW+c*glyphAdvance. The line is clamped to the
// buffer and the column to that line's rune length; the column rounds to the
// nearest gap so a click on a glyph's right half lands after it. Callers
// guarantee len(Lines) > 0 before calling.
func (t *TextView) caretAt(x, y int) (line, col int) {
	lineH := t.glyphHeight() + 4 // > 0: glyphHeight is always positive
	if y >= 4 {
		line = (y - 4) / lineH
	}
	// Map the viewport row back to an absolute buffer line through the scroll
	// offset, so a click after scrolling lands on the line the user sees.
	line += t.clampedScrollLine()
	if line > len(t.Lines)-1 {
		line = len(t.Lines) - 1
	}
	adv := t.glyphAdvance() // > 0 for every real font (Draw assumes the same)
	col = (x - 4 - t.gutterWidth() + adv/2) / adv
	if col < 0 {
		col = 0
	}
	if maxCol := len([]rune(t.Lines[line])); col > maxCol {
		col = maxCol
	}
	return line, col
}

// clampCol clamps CursorCol to the current line's rune length, used
// after ArrowUp / ArrowDown lands on a shorter line.
func (t *TextView) clampCol() {
	maxCol := len([]rune(t.Lines[t.CursorLine]))
	if t.CursorCol > maxCol {
		t.CursorCol = maxCol
	}
}
