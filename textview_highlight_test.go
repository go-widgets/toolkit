// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"strings"
	"testing"
)

// scanHasColor reports whether any pixel in the inclusive box
// [x0,x1]×[y0,y1] of a w-wide RGBA buffer equals c.
func scanHasColor(buf []byte, w, x0, y0, x1, y1 int, c RGBA) bool {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if pixelAt(buf, w, x, y) == c {
				return true
			}
		}
	}
	return false
}

// A nil Highlighter with no gutter is byte-identical to a Highlighter
// that colours every rune with the default ink: both must produce the
// same pixels, proving the coloured-run path collapses to the legacy
// single-ink path.
func TestTextViewHighlighterBaseIsByteIdentical(t *testing.T) {
	const w, h = 100, 60
	theme := DefaultLight()

	v1 := NewTextView("hello\nworld")
	v1.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf1 := makeSurface(w, h)
	v1.Draw(newP(buf1, w), theme)

	v2 := NewTextView("hello\nworld")
	v2.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	v2.Highlighter = func(_ int, line string) []TextSpan {
		return []TextSpan{{Start: 0, End: len([]rune(line)), Color: theme.OnSurface}}
	}
	buf2 := makeSurface(w, h)
	v2.Draw(newP(buf2, w), theme)

	if !bytes.Equal(buf1, buf2) {
		t.Fatal("full-line OnSurface highlighter is not byte-identical to nil highlighter")
	}
}

// A highlighter that colours only the first rune must paint that
// rune's glyph in the span colour and leave the next rune in the
// default ink — proving per-run colouring and the uncovered-gap
// fallback both work, with the second run drawn at the correct x.
func TestTextViewHighlighterColoursRunsAndGaps(t *testing.T) {
	const w, h = 60, 20
	theme := DefaultLight()
	red := RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}

	v := NewTextView("AB")
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	v.Highlighter = func(_ int, _ string) []TextSpan {
		return []TextSpan{{Start: 0, End: 1, Color: red}}
	}
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)

	adv := GlyphAdvance()
	gh := GlyphHeight()
	// 'A' occupies the box at textX=4, y=4.
	if !scanHasColor(buf, w, 4, 4, 4+adv, 4+gh, red) {
		t.Fatal("first rune 'A' not painted in span colour")
	}
	// 'B' is the next run at x=4+adv, painted in default ink, never red.
	if scanHasColor(buf, w, 4+adv, 4, 4+2*adv, 4+gh, red) {
		t.Fatal("second rune 'B' should not be red")
	}
	if !scanHasColor(buf, w, 4+adv, 4, 4+2*adv, 4+gh, theme.OnSurface) {
		t.Fatal("second rune 'B' not painted in default ink")
	}
}

// Out-of-range span bounds (negative Start, End past the line, and a
// span entirely beyond the line) must be clamped without panicking and
// still colour the in-range runes.
func TestTextViewHighlighterClampsBounds(t *testing.T) {
	const w, h = 60, 20
	theme := DefaultLight()
	red := RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}

	v := NewTextView("AB")
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	v.Highlighter = func(_ int, _ string) []TextSpan {
		return []TextSpan{
			{Start: -5, End: 99, Color: red}, // clamps to whole line
			{Start: 50, End: 60, Color: red}, // wholly out of range: no-op
		}
	}
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)

	adv := GlyphAdvance()
	gh := GlyphHeight()
	if !scanHasColor(buf, w, 4, 4, 4+adv, 4+gh, red) {
		t.Fatal("clamped span did not colour the first rune")
	}
}

// An empty line under a highlighter must be a no-op (drawSpans returns
// early) and not panic.
func TestTextViewHighlighterEmptyLine(t *testing.T) {
	const w, h = 60, 40
	v := NewTextView("\nx")
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	v.Highlighter = func(_ int, _ string) []TextSpan { return nil }
	v.Draw(newP(makeSurface(w, h), w), DefaultLight())
}

// The gutter reserves a left column, paints right-aligned numbers, and
// shifts the text right by the gutter width; with the gutter off the
// same text lands at the legacy x. The caret shifts by the same amount.
func TestTextViewLineNumberGutter(t *testing.T) {
	const w, h = 120, 60
	theme := DefaultLight()

	// Gutter OFF: 'h' of "hello" paints at textX=4.
	off := NewTextView("hello\nworld")
	off.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	if off.gutterWidth() != 0 {
		t.Fatalf("gutter-off: gutterWidth = %d, want 0", off.gutterWidth())
	}
	bufOff := makeSurface(w, h)
	off.Draw(newP(bufOff, w), theme)
	if !scanHasColor(bufOff, w, 4, 4, 4+GlyphAdvance(), 4+GlyphHeight(), theme.OnSurface) {
		t.Fatal("gutter-off: first glyph not at legacy x=4")
	}

	// Gutter ON: numbers appear on the left, text shifts right.
	on := NewTextView("hello\nworld")
	on.ShowLineNumbers = true
	on.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	bufOn := makeSurface(w, h)
	on.Draw(newP(bufOn, w), theme)

	gutterW := on.gutterWidth() // padded, metric-scaled: padL + numbersWidth + padR
	textX := gutterW            // text begins right after the gutter's right padding
	// The number ink (dimInk) is painted somewhere in the gutter column.
	if !scanHasColor(bufOn, w, 0, 4, gutterW, 4+GlyphHeight(), dimInk(theme)) {
		t.Fatal("gutter-on: line number not painted in the gutter")
	}
	// The text now starts at textX, not at the legacy x=4.
	if !scanHasColor(bufOn, w, textX, 4, textX+GlyphAdvance(), 4+GlyphHeight(), theme.OnSurface) {
		t.Fatal("gutter-on: text not shifted right by the gutter width")
	}
	// Breathing room: no number ink flush at the very left edge, and clear
	// Surface between the numbers column and the text (left + right padding).
	if scanHasColor(bufOn, w, 0, 4, 1, 4+GlyphHeight(), dimInk(theme)) {
		t.Fatal("gutter-on: number ink flush against the left edge (no left padding)")
	}
	numsRight := on.gutterPadL() + on.numbersWidth()
	if !scanHasColor(bufOn, w, numsRight, 4, textX, 4+GlyphHeight(), theme.Surface) {
		t.Fatal("gutter-on: no right padding between the numbers and the text")
	}

	// Caret math accounts for the gutter: a focused, cursored view
	// paints the caret stroke at textX + col*advance.
	cur := NewTextView("hello")
	cur.ShowLineNumbers = true
	cur.Focused = true
	cur.CursorCol = 2
	cur.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	bufCur := makeSurface(w, h)
	cur.Draw(newP(bufCur, w), theme)
	caretX := cur.gutterWidth() + 2*GlyphAdvance()
	if pixelAt(bufCur, w, caretX, 10) != theme.OnSurface {
		t.Fatalf("caret not at gutter-offset x=%d", caretX)
	}
}

// Line numbers are RIGHT-justified within the gutter: a 1-digit number sits
// further right than the 2-digit numbers it shares the column with — the
// left part of the column (where a 2-digit number's first glyph lands) is
// blank on the 1-digit row.
func TestTextViewLineNumbersRightJustified(t *testing.T) {
	const w, h = 160, 240
	theme := DefaultLight()
	lines := make([]string, 12) // last line "12" is 2 digits, so numbersWidth = width("12")
	for i := range lines {
		lines[i] = "x"
	}
	v := NewTextView(strings.Join(lines, "\n"))
	v.ShowLineNumbers = true
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)

	lineH := GlyphHeight() + 4
	rowY := func(i int) int { return 4 + i*lineH } // line i's glyph top
	// Column split: [padL, indentEnd) is where a 1-digit number is NOT drawn
	// (it's pushed right); [indentEnd, padL+numbersWidth) holds the single digit.
	indentEnd := v.gutterPadL() + v.numbersWidth() - TextWidth("1")

	// Row 0 ("1", 1 digit): the indent region is blank, the digit sits to its right.
	if scanHasColor(buf, w, v.gutterPadL(), rowY(0), indentEnd, rowY(0)+GlyphHeight(), dimInk(theme)) {
		t.Fatal(`"1" is not right-justified: ink found in the left indent region`)
	}
	if !scanHasColor(buf, w, indentEnd, rowY(0), v.gutterPadL()+v.numbersWidth(), rowY(0)+GlyphHeight(), dimInk(theme)) {
		t.Fatal(`"1" digit not painted in the right part of the numbers column`)
	}
	// Row 11 ("12", 2 digits): fills the whole numbers column, incl. the indent region.
	if !scanHasColor(buf, w, v.gutterPadL(), rowY(11), indentEnd, rowY(11)+GlyphHeight(), dimInk(theme)) {
		t.Fatal(`"12" should occupy the left of the numbers column (2-digit width)`)
	}
}

// CaretPixel is the exact inverse of the click→caret mapping: placing a click at
// CaretPixel(line, col) lands back on (line, col) — at any metric scale and with
// the padded line-number gutter — so a host never has to duplicate the gutter /
// advance geometry.
func TestTextViewCaretPixelRoundTrips(t *testing.T) {
	defer SetMetricScale(1)
	for _, sc := range []float64{1, 2} {
		SetMetricScale(sc)
		v := NewTextView("hello\nbrave new\nworld here")
		v.ShowLineNumbers = true
		v.SetBounds(Rect{X: 7, Y: 3, W: 400, H: 300})
		for _, tc := range [][2]int{{0, 0}, {0, 5}, {1, 3}, {2, 4}, {1, 0}} {
			x, y := v.CaretPixel(tc[0], tc[1])
			gl, gc := v.caretAt(x-v.Bounds().X, y-v.Bounds().Y)
			if gl != tc[0] || gc != tc[1] {
				t.Errorf("scale %v: CaretPixel(%d,%d) round-tripped to (%d,%d)", sc, tc[0], tc[1], gl, gc)
			}
		}
	}
}

// A custom GutterColor overrides the muted default.
func TestTextViewGutterColorOverride(t *testing.T) {
	const w, h = 120, 40
	theme := DefaultLight()
	green := RGBA{R: 0x00, G: 0x80, B: 0x00, A: 0xFF}

	v := NewTextView("x\ny")
	v.ShowLineNumbers = true
	v.GutterColor = green
	v.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	v.Draw(newP(buf, w), theme)

	gutterW := TextWidth("2") + 8
	if !scanHasColor(buf, w, 0, 4, gutterW, 4+GlyphHeight(), green) {
		t.Fatal("custom GutterColor not used for line numbers")
	}
	if scanHasColor(buf, w, 0, 4, gutterW, 4+GlyphHeight(), dimInk(theme)) {
		t.Fatal("dimInk fallback painted despite explicit GutterColor")
	}
}
