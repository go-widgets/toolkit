// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"unicode/utf8"

	"github.com/go-widgets/painter"
)

// Input-method composition for painted text fields.
//
// A painted field draws its own text, so the operating system's input method
// cannot see it: without help, dead keys, kana and hangul composition, an emoji
// picker or an accent popup have nothing to talk to. The window hosts (Cocoa's
// NSTextInputClient, GTK's GtkIMContext, Win32's IMM32) translate their native
// protocol into the four calls of [Composer], and read back the one thing an
// input method needs to place its candidate window: where the composition is on
// screen ([FocusedComposition]).
//
// References, opened for this design:
//
//   - Apple, NSTextInputClient: setMarkedText:selectedRange:replacementRange:
//     (the preedit and the cursor inside it), insertText:replacementRange: (the
//     commit), unmarkText, firstRectForCharacterRange:actualRange: (the candidate
//     window anchor) and selectedRange.
//   - GTK 4, GtkIMContext: "preedit-changed" with gtk_im_context_get_preedit_string
//     (text, PangoAttrList, cursor_pos), "commit", and
//     gtk_im_context_set_cursor_location (the candidate window anchor).
//   - Microsoft, IMM32: WM_IME_COMPOSITION with GCS_COMPSTR, GCS_CURSORPOS and
//     GCS_COMPATTR (ATTR_INPUT, ATTR_TARGET_CONVERTED, ...) for the preedit,
//     GCS_RESULTSTR for the commit, ImmSetCompositionWindow and
//     ImmSetCandidateWindow for placement.

// MaxPreeditRunes bounds a preedit string, in runes. An input method composes a
// word or a clause, never a document; a host that hands over more (a buggy or
// hostile input method) has the excess cut off rather than laid out and drawn.
const MaxPreeditRunes = 256

// MaxPreeditSegments bounds the number of styled segments of one preedit.
const MaxPreeditSegments = 64

// MaxCommitRunes bounds one committed string, in runes. A commit can be larger
// than a preedit (an emoji picker, a dictation result), but it is not unbounded.
const MaxCommitRunes = 4096

// PreeditStyle is how a segment of the preedit is underlined.
type PreeditStyle uint8

const (
	// PreeditInput is text still being typed: a thin underline.
	PreeditInput PreeditStyle = iota
	// PreeditConverted is text an input method has already converted (a kanji
	// candidate not yet accepted): a thin underline, as GTK's PANGO_UNDERLINE_SINGLE
	// and IMM32's ATTR_CONVERTED.
	PreeditConverted
	// PreeditTarget is the clause the input method is converting right now (the
	// one whose candidate list is open): a thick underline in the text colour, as
	// IMM32's ATTR_TARGET_CONVERTED and Cocoa's NSUnderlineStyleThick.
	PreeditTarget
)

// PreeditSegment styles the runes [Start, End) of a preedit.
type PreeditSegment struct {
	Start, End int // rune offsets into Preedit.Text
	Style      PreeditStyle
}

// Preedit is the uncommitted text of an input-method composition. It is drawn at
// the caret and is not part of the field's text until [Composer.CommitText].
type Preedit struct {
	// Text is the composition. Empty means no composition.
	Text string
	// Cursor is the input method's caret inside Text, as a rune offset in
	// [0, runes]; -1 means the input method shows none.
	Cursor int
	// Segments style parts of Text. Runes outside every segment are
	// [PreeditInput].
	Segments []PreeditSegment
}

// Sanitize returns p made safe to store and draw: valid UTF-8 (each invalid byte
// becomes U+FFFD), no control characters, at most [MaxPreeditRunes] runes, the
// cursor in [-1, runes], and at most [MaxPreeditSegments] segments, each clamped
// into the text with empty or inverted ones dropped. It never panics, whatever
// the input.
func (p Preedit) Sanitize() Preedit {
	text, n := boundedText(p.Text, MaxPreeditRunes, false)
	out := Preedit{Text: text, Cursor: p.Cursor}
	if out.Cursor < -1 {
		out.Cursor = -1
	}
	if out.Cursor > n {
		out.Cursor = n
	}
	for _, s := range p.Segments {
		if len(out.Segments) == MaxPreeditSegments {
			break
		}
		a, b := clampInt(s.Start, 0, n), clampInt(s.End, 0, n)
		if a >= b {
			continue
		}
		st := s.Style
		if st > PreeditTarget {
			st = PreeditInput
		}
		out.Segments = append(out.Segments, PreeditSegment{Start: a, End: b, Style: st})
	}
	return out
}

// styleAt is the style of rune i of the preedit.
func (p Preedit) styleAt(i int) PreeditStyle {
	st := PreeditInput
	for _, s := range p.Segments {
		if i >= s.Start && i < s.End {
			st = s.Style // a later segment overrides an earlier one
		}
	}
	return st
}

// boundedText returns s with invalid UTF-8 replaced by U+FFFD, control
// characters dropped (newlines and tabs are kept when multiline), cut at max
// runes, and the number of runes kept.
func boundedText(s string, max int, multiline bool) (string, int) {
	if len(s) > 4*max { // no rune is longer than 4 bytes: never scan the excess
		s = s[:4*max]
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(s) && n < max; {
		r, w := utf8.DecodeRuneInString(s[i:])
		i += w
		if r == utf8.RuneError && w == 1 {
			r = '�'
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			if !multiline || (r != '\n' && r != '\t') {
				continue
			}
		}
		b.WriteRune(r)
		n++
	}
	return b.String(), n
}

// Composer is a painted field an input method can drive. A field that is focused
// and implements it receives the preedit, the commit and the cancel from its
// window host.
type Composer interface {
	// SetPreedit shows p at the caret without touching the text. The field
	// sanitises p. An empty p.Text ends the composition. A disabled field, or a
	// masked one (the preedit would be the secret in clear), keeps no preedit.
	SetPreedit(p Preedit)
	// Preedit returns the preedit being shown, with an empty Text when none.
	Preedit() Preedit
	// CommitText ends the composition and inserts text at the caret. The field
	// sanitises text.
	CommitText(text string)
	// CancelPreedit ends the composition and inserts nothing.
	CancelPreedit()
	// CompositionRect is where the composition is on screen: the rectangle of
	// the preedit (or, with none, of the caret, one pixel wide), in the same
	// space as Bounds. An input method anchors its candidate window under it.
	CompositionRect() Rect
}

// preeditMeta remembers the cursor and segments that went with a preedit text.
// Entry and TextView keep the text in a plain string (a host may set it from a
// composition event, with only the text to go on); this holds the rest, and is
// used only while it still describes that text.
type preeditMeta struct {
	text string
	cur  int
	segs []PreeditSegment
}

// view is the Preedit to draw for text.
func (m preeditMeta) view(text string) Preedit {
	if text == "" {
		return Preedit{Cursor: -1}
	}
	if m.text == text {
		return Preedit{Text: text, Cursor: m.cur, Segments: m.segs}
	}
	return Preedit{Text: text, Cursor: utf8.RuneCountInString(text)}
}

// remember stores a sanitised preedit's metadata.
func (m *preeditMeta) remember(p Preedit) {
	*m = preeditMeta{text: p.Text, cur: p.Cursor, segs: p.Segments}
}

// drawPreedit paints p with its top-left at (x, y) in the ghost tone, underlines
// each rune by its style, draws the input method's cursor, and returns the width
// of the whole preedit in pixels and the x of the input method's cursor (the
// preedit's right edge when it shows none).
func (b *Base) drawPreedit(p painter.Painter, theme *Theme, x, y int, pre Preedit) (w, cursorX int) {
	runes := []rune(pre.Text)
	h := b.glyphHeight()
	w = b.textWidth(pre.Text)
	cursorX = x + w
	b.drawText(p, x, y, pre.Text, theme.SurfaceAlt)
	// One underline per run of equal style, so a clause is one stroke.
	for i := 0; i < len(runes); {
		st := pre.styleAt(i)
		j := i + 1
		for j < len(runes) && pre.styleAt(j) == st {
			j++
		}
		x0 := x + b.textWidth(string(runes[:i]))
		x1 := x + b.textWidth(string(runes[:j]))
		if st == PreeditTarget {
			fillRect(p, x0, y+h, x1-x0, scaled(2), theme.OnSurface)
		} else {
			fillRect(p, x0, y+h, x1-x0, 1, theme.SurfaceAlt)
		}
		i = j
	}
	if pre.Cursor >= 0 && pre.Cursor <= len(runes) {
		cursorX = x + b.textWidth(string(runes[:pre.Cursor]))
	}
	return w, cursorX
}

// FocusedComposition finds the focused field under root that can compose and
// returns it with the rectangle of its composition in root coordinates. ok is
// false when no focused field composes: the host then lets the input method
// fall back to plain key input. A field that is hidden (empty bounds) is never
// returned.
func FocusedComposition(root Widget) (c Composer, rect Rect, ok bool) {
	var walk func(w Widget, dx, dy int) bool
	walk = func(w Widget, dx, dy int) bool {
		if w == nil {
			return false
		}
		if b := w.Bounds(); b.W <= 0 || b.H <= 0 {
			return false
		}
		if cc, isC := w.(Composer); isC {
			if f, isF := w.(Focusable); isF && f.Focused() {
				r := cc.CompositionRect()
				c, rect, ok = cc, Rect{X: r.X + dx, Y: r.Y + dy, W: r.W, H: r.H}, true
				return true
			}
		}
		if o, isO := w.(childOffsetter); isO {
			ox, oy := o.ChildOffset()
			dx, dy = dx+ox, dy+oy
		}
		if cc, isC := w.(childContainer); isC {
			for _, child := range cc.Children() {
				if walk(child, dx, dy) {
					return true
				}
			}
		}
		return false
	}
	walk(root, 0, 0)
	return c, rect, ok
}

// SetPreedit implements [Composer].
func (e *Entry) SetPreedit(p Preedit) {
	if e.Disabled().Get() || e.Mask != 0 {
		e.composition, e.pre = "", preeditMeta{}
		return
	}
	p = p.Sanitize() // line breaks and tabs are dropped: an entry has one line
	e.composition = p.Text
	e.pre.remember(p)
}

// Preedit implements [Composer].
func (e *Entry) Preedit() Preedit { return e.pre.view(e.composition) }

// CommitText implements [Composer]: the preedit is dropped and text typed at the
// caret, as if the host had sent it as one EventChar.
func (e *Entry) CommitText(text string) {
	text, _ = boundedText(text, MaxCommitRunes, false)
	e.composition, e.pre = "", preeditMeta{}
	if text != "" {
		e.OnEvent(Event{Kind: EventChar, Code: text})
	}
}

// CancelPreedit implements [Composer].
func (e *Entry) CancelPreedit() { e.composition, e.pre = "", preeditMeta{} }

// CompositionRect implements [Composer].
func (e *Entry) CompositionRect() Rect {
	r := e.Bounds()
	pad := scaled(entryPadX)
	runes := []rune(e.display())
	cur := clampInt(e.cursor, 0, len(runes))
	x := r.X + pad + e.textWidth(string(runes[:cur])) - e.scrollX
	w := 1
	if e.composition != "" && e.Mask == 0 {
		w = e.textWidth(e.composition)
	}
	h := e.glyphHeight()
	return Rect{X: x, Y: r.Y + (r.H-h)/2, W: w, H: h}
}

var _ Composer = (*Entry)(nil)

// SetPreedit implements [Composer].
func (t *TextView) SetPreedit(p Preedit) {
	if t.Disabled().Get() || t.ann.open {
		t.composition, t.pre = "", preeditMeta{}
		return
	}
	p = p.Sanitize()
	t.composition = p.Text
	t.pre.remember(p)
}

// Preedit implements [Composer].
func (t *TextView) Preedit() Preedit { return t.pre.view(t.composition) }

// CommitText implements [Composer]: the preedit is dropped and text typed at the
// caret, as if the host had sent it as one EventChar.
func (t *TextView) CommitText(text string) {
	text, _ = boundedText(text, MaxCommitRunes, true)
	t.composition, t.pre = "", preeditMeta{}
	if text != "" {
		t.OnEvent(Event{Kind: EventChar, Code: text})
	}
}

// CancelPreedit implements [Composer].
func (t *TextView) CancelPreedit() { t.composition, t.pre = "", preeditMeta{} }

// CompositionRect implements [Composer].
func (t *TextView) CompositionRect() Rect {
	x, y := t.CaretPixel(t.CursorLine().Get(), t.CursorCol().Get())
	w := 1
	if t.composition != "" {
		w = t.textWidth(t.composition)
	}
	return Rect{X: x, Y: y, W: w, H: t.glyphHeight() + lineGap()}
}

var _ Composer = (*TextView)(nil)
