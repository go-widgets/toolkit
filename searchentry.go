// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// SearchEntry is a single-line text input decorated with a leading
// search-prefix glyph and, when Text is non-empty, a trailing "clear"
// affordance on the right. Think GTK's SearchEntry: an Entry whose
// visual chrome hints at its role and offers a one-click reset. The
// widget appends printable characters, deletes on Backspace, and clears
// on a click in the right-side X slot. It draws a simple end-of-text
// caret when Focused (set by the host), measured with its own font so it
// always aligns; it has no cursor navigation or IME — callers needing
// those should reach for Entry / TextView instead.
//
// An optional leading Icon lets the host paint a real magnifier (or any
// glyph) in the left prefix slot instead of the "?" text stand-in. When
// set, Draw invokes Icon with the prefix slot's rect + the OnSurface ink
// and skips the "?" text; when nil, the classic "?" stand-in is drawn, so
// existing callers are unaffected. This mirrors Banner.Icon.
type SearchEntry struct {
	Base
	focusState // when focused, a text caret is drawn at the end of Text
	Text       string
	OnChange   func(s string)
	Icon       func(p painter.Painter, r Rect, ink RGBA)
}

// SearchEntryPadX is the horizontal padding between the widget's outer
// border and the inner content (the search prefix, the text field, the
// clear affordance).
const SearchEntryPadX = 4

// SearchEntryIconW is the pixel width reserved for the leading prefix
// glyph and the trailing clear affordance. Both slots share the same
// width so hit-testing stays symmetric.
const SearchEntryIconW = 16

// searchEntryPrefix is the glyph rendered in the left icon slot. We
// pick "?" from the 5x7 bitmap font (font.go) as a stand-in for a
// magnifier — the toolkit's bitmap font does not carry a magnifier
// glyph and adding one just for this widget would be out of scale.
const searchEntryPrefix = "?"

// searchEntryClear is the glyph rendered in the right icon slot when
// Text is non-empty. Lower-case "x" reads as a subtle close/reset
// affordance next to the ink text.
const searchEntryClear = "x"

// NewSearchEntry builds a SearchEntry pre-loaded with initial text.
// The constructor does not run OnChange for the initial value so
// callers can wire the callback after construction without a spurious
// notification.
func NewSearchEntry(text string) *SearchEntry {
	return &SearchEntry{Text: text}
}

// Draw paints the entry body, the leading prefix glyph, the current
// Text, and (when Text is non-empty) the trailing clear affordance.
func (s *SearchEntry) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	textY := r.Y + (r.H-s.glyphHeight())/2
	// Left prefix slot: a host-supplied Icon (real magnifier) when set,
	// otherwise the "?" bitmap-font stand-in.
	if s.Icon != nil {
		iconR := Rect{X: r.X + SearchEntryPadX, Y: r.Y, W: SearchEntryIconW, H: r.H}
		s.Icon(p, iconR, theme.OnSurface)
	} else {
		prefixX := r.X + SearchEntryPadX + (SearchEntryIconW-s.glyphAdvance())/2
		s.drawText(p, prefixX, textY, searchEntryPrefix, theme.OnSurface)
	}
	// Middle text.
	textX := r.X + SearchEntryPadX + SearchEntryIconW
	s.drawText(p, textX, textY, s.Text, theme.OnSurface)
	// Caret at the end of the text when focused. Measured with the widget's own
	// font so it always aligns with the text the widget just drew — a host must not
	// overlay its own caret with a different font engine.
	if s.focused {
		caretW := s.glyphHeight() / 12
		if caretW < 1 {
			caretW = 1
		}
		fillRect(p, textX+s.textWidth(s.Text), textY, caretW, s.glyphHeight(), theme.OnSurface)
	}
	// Right clear slot only when there is text to clear.
	if s.Text != "" {
		clearX := r.X + r.W - SearchEntryPadX - SearchEntryIconW + (SearchEntryIconW-s.glyphAdvance())/2
		s.drawText(p, clearX, textY, searchEntryClear, theme.Border)
	}
	s.drawFocusRing(p, theme, r)
}

// OnEvent handles character insertion (EventChar), Backspace deletion
// (EventKeyDown / "Backspace"), and click-to-clear in the right icon
// slot (EventClick, when Text is non-empty). Other events are ignored.
func (s *SearchEntry) OnEvent(ev Event) {
	switch ev.Kind {
	case EventChar:
		if ev.Code == "" {
			return
		}
		s.Text += ev.Code
		s.fireChange()
	case EventKeyDown:
		if ev.Code != "Backspace" {
			return
		}
		runes := []rune(s.Text)
		if len(runes) == 0 {
			return
		}
		s.Text = string(runes[:len(runes)-1])
		s.fireChange()
	case EventClick:
		if s.Text == "" {
			return
		}
		r := s.Bounds()
		clearLeft := r.W - SearchEntryPadX - SearchEntryIconW
		clearRight := r.W - SearchEntryPadX
		if ev.X >= clearLeft && ev.X < clearRight {
			s.Text = ""
			s.fireChange()
		}
	}
}

// fireChange invokes OnChange when set. Kept as a helper so every
// mutation path routes through one guard.
func (s *SearchEntry) fireChange() {
	if s.OnChange != nil {
		s.OnChange(s.Text)
	}
}
