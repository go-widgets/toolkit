// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Entry is a single-line text input. Receives focus on click, edits
// Text via EventKeyDown (Backspace, ArrowLeft/Right, Home, End,
// Enter) + EventChar (printable runes). A 1-pixel vertical cursor
// renders at the cursor offset when Focused.
//
// The widget treats Text as a rune index space so multi-byte UTF-8
// characters move the cursor by one position even when they take
// several bytes on the wire.
type Entry struct {
	Base
	Text     string
	Cursor   int // rune index in [0, len(runes)]
	Focused  bool
	OnChange func(text string)
	OnSubmit func(text string)
}

// NewEntry builds an Entry with initial text + cursor parked at end.
func NewEntry(initial string) *Entry {
	r := []rune(initial)
	return &Entry{Text: initial, Cursor: len(r)}
}

// Draw paints the border, fill, text + (when Focused) a 1-px cursor
// stroke at the cursor's pixel position.
func (e *Entry) Draw(p painter.Painter, theme *Theme) {
	r := e.Bounds()
	border := theme.Border
	if e.Focused {
		border = theme.Accent
	}
	fillRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, theme.Surface)
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, border)
	textY := r.Y + (r.H-GlyphHeight())/2
	DrawText(p, r.X+4, textY, e.Text, theme.OnSurface)
	if e.Focused {
		cx := r.X + 4 + e.Cursor*GlyphAdvance()
		fillRect(p, cx, textY-1, 1, GlyphHeight()+2, theme.OnSurface)
	}
}

// OnEvent handles focus, keyboard navigation, character insertion +
// delete.
func (e *Entry) OnEvent(ev Event) {
	runes := []rune(e.Text)
	switch ev.Kind {
	case EventClick:
		e.Focused = true
	case EventKeyDown:
		switch ev.Code {
		case "Backspace":
			if e.Cursor > 0 {
				runes = append(runes[:e.Cursor-1], runes[e.Cursor:]...)
				e.Cursor--
				e.Text = string(runes)
				if e.OnChange != nil {
					e.OnChange(e.Text)
				}
			}
		case "ArrowLeft":
			if e.Cursor > 0 {
				e.Cursor--
			}
		case "ArrowRight":
			if e.Cursor < len(runes) {
				e.Cursor++
			}
		case "Home":
			e.Cursor = 0
		case "End":
			e.Cursor = len(runes)
		case "Enter":
			if e.OnSubmit != nil {
				e.OnSubmit(e.Text)
			}
		}
	case EventChar:
		ch := []rune(ev.Code)
		if len(ch) == 0 {
			return
		}
		runes = append(runes[:e.Cursor], append(ch, runes[e.Cursor:]...)...)
		e.Cursor += len(ch)
		e.Text = string(runes)
		if e.OnChange != nil {
			e.OnChange(e.Text)
		}
	}
}
