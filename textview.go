// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "strings"

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
func (t *TextView) Draw(surface []byte, surfaceW int, theme *Theme) {
	r := t.Bounds()
	border := theme.Border
	if t.Focused {
		border = theme.Accent
	}
	fillRect(surface, surfaceW, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(surface, surfaceW, r.X, r.Y, r.W, r.H, border)
	lineH := GlyphHeight + 4 // 1-pixel-line font + 4 px line spacing
	for i, line := range t.Lines {
		y := r.Y + 4 + i*lineH
		DrawText(surface, surfaceW, r.X+4, y, line, theme.OnSurface)
	}
	if t.Focused {
		cx := r.X + 4 + t.CursorCol*GlyphAdvance
		cy := r.Y + 4 + t.CursorLine*lineH
		fillRect(surface, surfaceW, cx, cy-1, 1, GlyphHeight+2, theme.OnSurface)
		// IME composition preview: render the pending string in the
		// muted SurfaceAlt tone starting at the cursor, so the user
		// sees dead-key / CJK candidates without them entering the
		// buffer. Underlined by a 1-px SurfaceAlt strip beneath.
		if t.Composition != "" {
			cw := TextWidth(t.Composition)
			DrawText(surface, surfaceW, cx, cy, t.Composition, theme.SurfaceAlt)
			fillRect(surface, surfaceW, cx, cy+GlyphHeight, cw, 1, theme.SurfaceAlt)
		}
	}
}

// OnEvent dispatches the editing operations.
func (t *TextView) OnEvent(ev Event) {
	switch ev.Kind {
	case EventClick:
		t.Focused = true
	case EventKeyDown:
		t.handleKey(ev.Code)
	case EventChar:
		// If an IME composition was in flight, the incoming char is
		// the commit result — clear the preview BEFORE inserting so
		// the buffer + display stay consistent.
		t.Composition = ""
		t.insertText(ev.Code)
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
		t.backspace()
	case "Enter":
		t.splitLine()
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

// clampCol clamps CursorCol to the current line's rune length, used
// after ArrowUp / ArrowDown lands on a shorter line.
func (t *TextView) clampCol() {
	maxCol := len([]rune(t.Lines[t.CursorLine]))
	if t.CursorCol > maxCol {
		t.CursorCol = maxCol
	}
}
