// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// --- constants -----------------------------------------------------------

func TestGlyphMetricsAreSane(t *testing.T) {
	if GlyphHeight != 7 {
		t.Fatalf("GlyphHeight = %d, want 7", GlyphHeight)
	}
	if GlyphAdvance != 6 {
		t.Fatalf("GlyphAdvance = %d, want 6 (5px glyph + 1px spacing)", GlyphAdvance)
	}
}

// --- TextWidth -----------------------------------------------------------

func TestTextWidthScalesWithLen(t *testing.T) {
	if got := TextWidth(""); got != 0 {
		t.Fatalf("TextWidth(\"\") = %d, want 0", got)
	}
	if got := TextWidth("a"); got != GlyphAdvance {
		t.Fatalf("TextWidth(\"a\") = %d, want %d", got, GlyphAdvance)
	}
	if got := TextWidth("Hello"); got != 5*GlyphAdvance {
		t.Fatalf("TextWidth(\"Hello\") = %d, want %d", got, 5*GlyphAdvance)
	}
}

// --- DrawText ------------------------------------------------------------

// glyphTouchesAnyPixel renders ch on a fresh surface and reports
// whether any pixel was inked by the glyph (i.e. differs from the
// sentinel fill). Used by the every-glyph-paints test.
func glyphTouchesAnyPixel(t *testing.T, ch byte) bool {
	t.Helper()
	const w, h = 16, GlyphHeight + 2
	buf := makeSurface(w, h)
	ink := RGB(0x10, 0x20, 0x30)
	DrawText(newP(buf, w), 1, 1, string([]byte{ch}), ink)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == ink {
				return true
			}
		}
	}
	return false
}

// Every printable glyph in the spec'd alphabet must light up at least
// one pixel — otherwise the table entry is broken (all-zero columns or
// missing from the map).
func TestEveryGlyphPaintsAtLeastOnePixel(t *testing.T) {
	groups := []string{
		"0123456789",
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"abcdefghijklmnopqrstuvwxyz",
		".,:-_/?!()<>+*=#%",
	}
	for _, g := range groups {
		for i := 0; i < len(g); i++ {
			ch := g[i]
			if !glyphTouchesAnyPixel(t, ch) {
				t.Errorf("glyph %q paints no pixels", ch)
			}
		}
	}
	// Space is explicitly blank: it must NOT paint anything (and must
	// still be in the table so it consumes an advance slot rather than
	// being treated as unknown — which is observably identical for
	// blanks, but the spec calls it out).
	const w, h = GlyphAdvance + 2, GlyphHeight + 2
	buf := makeSurface(w, h)
	DrawText(newP(buf, w), 0, 0, " ", RGB(1, 2, 3))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == (RGBA{R: 1, G: 2, B: 3, A: 0xFF}) {
				t.Fatalf("space painted pixel at (%d,%d)", x, y)
			}
		}
	}
}

// Two adjacent glyphs land at the expected x offsets — i.e. DrawText
// honours GlyphAdvance for layout, not just for TextWidth.
func TestDrawTextAdvancesByGlyphAdvance(t *testing.T) {
	const w, h = 64, GlyphHeight + 4
	buf := makeSurface(w, h)
	ink := RGB(0xFF, 0x00, 0xAA)
	// "II" — 'I' lights up only its column 2 (bits[2] = 0x7F), so we
	// can pinpoint exactly which columns must be inked.
	DrawText(newP(buf, w), 0, 0, "II", ink)
	if pixelAt(buf, w, 2, 3) != ink {
		t.Fatalf("first I middle column not inked: %+v", pixelAt(buf, w, 2, 3))
	}
	if pixelAt(buf, w, GlyphAdvance+2, 3) != ink {
		t.Fatalf("second I middle column not inked at x=%d: %+v",
			GlyphAdvance+2, pixelAt(buf, w, GlyphAdvance+2, 3))
	}
	// The single-pixel gap between the two glyphs must remain sentinel.
	if pixelAt(buf, w, 5, 3) != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatalf("inter-glyph gap was inked: %+v", pixelAt(buf, w, 5, 3))
	}
}

// Unknown characters render as a blank (no pixels) but still consume
// the advance slot so subsequent glyphs land at the expected x.
func TestDrawTextUnknownCharRendersBlankButAdvances(t *testing.T) {
	const w, h = 32, GlyphHeight + 2
	buf := makeSurface(w, h)
	ink := RGB(0xAB, 0xCD, 0xEF)
	// '~' is not in the table; 'I' is. After "~I" the I should still
	// land at x = GlyphAdvance + 2 (column 2 of the second slot).
	DrawText(newP(buf, w), 0, 0, "~I", ink)
	// Nothing inked in the first slot.
	for x := 0; x < GlyphAdvance; x++ {
		for y := 0; y < GlyphHeight; y++ {
			if pixelAt(buf, w, x, y) == ink {
				t.Fatalf("unknown char painted pixel at (%d,%d)", x, y)
			}
		}
	}
	if pixelAt(buf, w, GlyphAdvance+2, 3) != ink {
		t.Fatalf("I after unknown char not painted at expected x: %+v",
			pixelAt(buf, w, GlyphAdvance+2, 3))
	}
}

// Per-pixel clipping: DrawText must not panic + must not write past
// the buffer when the glyph lands partly off-surface (negative x/y,
// past right edge, past bottom edge).
func TestDrawTextClipsOOB(t *testing.T) {
	const w, h = 8, 8
	ink := RGB(0x77, 0x88, 0x99)

	// Negative x: leftmost columns fall off the left.
	buf := makeSurface(w, h)
	DrawText(newP(buf, w), -3, 0, "A", ink)

	// Negative y: top rows fall off the top.
	buf = makeSurface(w, h)
	DrawText(newP(buf, w), 0, -3, "A", ink)

	// Past right edge: rightmost columns fall off the right.
	buf = makeSurface(w, h)
	DrawText(newP(buf, w), w-1, 0, "A", ink)

	// Past bottom edge: bottom rows fall off the bottom (triggers
	// the off+3 >= len(surface) guard in putPixel).
	buf = makeSurface(w, h)
	DrawText(newP(buf, w), 0, h-1, "A", ink)

	// Fully off-surface to the right + below.
	buf = makeSurface(w, h)
	DrawText(newP(buf, w), w+5, h+5, "A", ink)
}

// Defensive guards: painters wrapping unusable surfaces must short-
// circuit without panicking.
func TestDrawTextGuardsAgainstEmptySurface(t *testing.T) {
	// Nil buffer.
	DrawText(newP(nil, 16), 0, 0, "A", RGB(1, 2, 3))
	// Buffer too small for one pixel.
	DrawText(newP(make([]byte, 3), 16), 0, 0, "A", RGB(1, 2, 3))
	// Width <= 0.
	DrawText(newP(make([]byte, 64), 0), 0, 0, "A", RGB(1, 2, 3))
	DrawText(newP(make([]byte, 64), -4), 0, 0, "A", RGB(1, 2, 3))
}

// On a non-Pixel painter (a CellPainter for a TUI, an SvgPainter for
// vector output) DrawText delegates to the painter's own Text
// primitive instead of writing bitmap pixels. Prove it by rendering
// into a CellPainter and reading the runes back.
func TestDrawTextDelegatesOnNonPixelPainter(t *testing.T) {
	cp := painter.NewCellPainter(10, 2)
	DrawText(cp, 2, 0, "OK", RGB(0xFF, 0xFF, 0xFF))
	if cp.Cells[0*10+2].Rune != 'O' {
		t.Fatalf("first cell rune = %q, want 'O'", cp.Cells[0*10+2].Rune)
	}
	if cp.Cells[0*10+3].Rune != 'K' {
		t.Fatalf("second cell rune = %q, want 'K'", cp.Cells[0*10+3].Rune)
	}
}

// Empty string is a no-op: must not panic + must not paint anything.
func TestDrawTextEmptyString(t *testing.T) {
	const w, h = 16, 8
	buf := makeSurface(w, h)
	DrawText(newP(buf, w), 4, 4, "", RGB(1, 2, 3))
	if pixelAt(buf, w, 4, 4) != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatal("empty string should paint nothing")
	}
}

// Ink alpha is honoured: a semi-transparent ink is composited toward the
// destination (the painter alpha-blends), so it lands lighter than the opaque
// ink over the 0xC8 sentinel background.
func TestDrawTextHonoursInkAlpha(t *testing.T) {
	const w, h = 16, 8
	// Opaque ink writes verbatim.
	full := makeSurface(w, h)
	DrawText(newP(full, w), 0, 0, "I", RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF})
	if got := pixelAt(full, w, 2, 3); got != (RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF}) {
		t.Fatalf("opaque ink pixel = %+v, want the ink verbatim", got)
	}
	// Half-alpha ink blends toward the 0xC8 background: strictly between the
	// ink (0x10) and the background (0xC8).
	half := makeSurface(w, h)
	DrawText(newP(half, w), 0, 0, "I", RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0x80})
	if got := pixelAt(half, w, 2, 3); !(got.R > 0x10 && got.R < 0xC8) {
		t.Fatalf("half-alpha ink R = %d, want strictly between 0x10 and 0xC8 (blended)", got.R)
	}
}
